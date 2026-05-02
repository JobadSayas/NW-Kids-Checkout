package eventlocation

import (
	"testing"
	"time"

	"kids-checkin/internal/db"
	"kids-checkin/internal/repo/event"
	"kids-checkin/internal/repo/location"

	"github.com/Masterminds/squirrel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateEventWithLocations_Success(t *testing.T) {
	testDB, cleanup, err := db.PrepareTestDB()
	require.NoError(t, err, "Failed to prepare test DB")
	t.Cleanup(cleanup)

	createdEvent, syncedLocations, eventCreated, err := CreateEventWithLocations(t.Context(), testDB, event.Event{
		Name:             "Family Service",
		PlanningCenterID: "pc_evt_22",
	}, []location.Location{
		{PlanningCenterID: "loc-1", Name: "Room A"},
		{PlanningCenterID: "loc-2", Name: "Room B"},
	})
	require.NoError(t, err)
	require.True(t, eventCreated)
	require.NotZero(t, createdEvent.ID)
	require.Len(t, syncedLocations, 2)

	var locationCount int64
	err = squirrel.Select("count(*)").From("locations").Where(squirrel.Eq{"event_id": createdEvent.ID}).RunWith(testDB).QueryRow().Scan(&locationCount)
	require.NoError(t, err)
	assert.Equal(t, int64(2), locationCount)
}

func TestCreateEventWithLocations_SyncExisting(t *testing.T) {
	testDB, cleanup, err := db.PrepareTestDB()
	require.NoError(t, err, "Failed to prepare test DB")
	t.Cleanup(cleanup)

	res, err := squirrel.Insert("events").
		RunWith(testDB).
		Columns("name", "planning_center_id").
		Values("Existing Event", "pc_evt_22").
		ExecContext(t.Context())
	require.NoError(t, err)
	insertedID, _ := res.LastInsertId()
	require.NotZero(t, insertedID)

	groupRes, err := squirrel.Insert("location_groups").
		RunWith(testDB).
		Columns("name").
		Values("group-a").
		ExecContext(t.Context())
	require.NoError(t, err)
	groupID, _ := groupRes.LastInsertId()
	require.NotZero(t, groupID)

	_, err = squirrel.Update("events").
		RunWith(testDB).
		Set("location_group_id", groupID).
		Where(squirrel.Eq{"id": insertedID}).
		ExecContext(t.Context())
	require.NoError(t, err)

	lastCheckedOut := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	_, err = squirrel.Insert("locations").
		RunWith(testDB).
		Columns("planning_center_id", "planning_center_parent_id", "event_id", "location_group_id", "name", "auto_fetch", "last_checked_out_time").
		Values("loc-1", "parent-old", insertedID, groupID, "Old Room A", true, lastCheckedOut).
		ExecContext(t.Context())
	require.NoError(t, err)

	_, err = squirrel.Insert("locations").
		RunWith(testDB).
		Columns("planning_center_id", "event_id", "name", "auto_fetch").
		Values("loc-2", insertedID, "Room B", false).
		ExecContext(t.Context())
	require.NoError(t, err)

	parentNew := "parent-new"
	createdEvent, syncedLocations, eventCreated, err := CreateEventWithLocations(t.Context(), testDB, event.Event{
		Name:             "Family Service",
		PlanningCenterID: "pc_evt_22",
	}, []location.Location{
		{PlanningCenterID: "loc-1", Name: "Room A", PlanningCenterParentID: &parentNew},
		{PlanningCenterID: "loc-3", Name: "Room C"},
	})
	require.NoError(t, err)
	require.Len(t, syncedLocations, 2)
	require.Equal(t, insertedID, createdEvent.ID)
	require.False(t, eventCreated)
	require.Equal(t, "Family Service", createdEvent.Name)

	eventRepo := event.NewRepo(testDB)
	storedEvent, err := eventRepo.GetEventByID(t.Context(), insertedID)
	require.NoError(t, err)
	assert.Equal(t, "Family Service", storedEvent.Name)
	require.NotNil(t, storedEvent.LocationGroupID)
	assert.Equal(t, groupID, *storedEvent.LocationGroupID)

	locRepo := location.NewRepo(testDB)
	allLocations, err := locRepo.ListLocations(t.Context(), location.LocationFilter{EventID: insertedID})
	require.NoError(t, err)
	require.Len(t, allLocations, 2)

	locationsByPCID := make(map[string]location.Location, len(allLocations))
	for _, loc := range allLocations {
		locationsByPCID[loc.PlanningCenterID] = loc
	}

	updated := locationsByPCID["loc-1"]
	assert.Equal(t, "Room A", updated.Name)
	if assert.NotNil(t, updated.PlanningCenterParentID) {
		assert.Equal(t, "parent-new", *updated.PlanningCenterParentID)
	}
	if assert.NotNil(t, updated.LocationGroupID) {
		assert.Equal(t, groupID, *updated.LocationGroupID)
	}
	assert.True(t, updated.AutoFetch)
	assert.Equal(t, lastCheckedOut, updated.LastCheckedOutTime)

	created := locationsByPCID["loc-3"]
	assert.Equal(t, "Room C", created.Name)
	assert.False(t, created.AutoFetch)
	assert.Nil(t, created.LocationGroupID)
	assert.Zero(t, created.LastCheckedOutTime)

	_, exists := locationsByPCID["loc-2"]
	assert.False(t, exists)
}

func TestCreateEventWithLocations_SyncExisting_NameConflict(t *testing.T) {
	testDB, cleanup, err := db.PrepareTestDB()
	require.NoError(t, err, "Failed to prepare test DB")
	t.Cleanup(cleanup)

	res, err := squirrel.Insert("events").
		RunWith(testDB).
		Columns("name", "planning_center_id").
		Values("Existing Event", "pc_evt_88").
		ExecContext(t.Context())
	require.NoError(t, err)
	insertedID, _ := res.LastInsertId()
	require.NotZero(t, insertedID)

	_, err = squirrel.Insert("events").
		RunWith(testDB).
		Columns("name", "planning_center_id").
		Values("Kids Service", "pc_evt_other").
		ExecContext(t.Context())
	require.NoError(t, err)

	_, _, _, err = CreateEventWithLocations(t.Context(), testDB, event.Event{
		Name:             "Kids Service",
		PlanningCenterID: "pc_evt_88",
	}, []location.Location{
		{PlanningCenterID: "loc-1", Name: "Room A"},
	})
	require.ErrorIs(t, err, event.ErrEventExists)

	eventRepo := event.NewRepo(testDB)
	storedEvent, err := eventRepo.GetEventByID(t.Context(), insertedID)
	require.NoError(t, err)
	assert.Equal(t, "Existing Event", storedEvent.Name)
}

func TestCreateEventWithLocations_SyncExisting_DeleteAllLocations(t *testing.T) {
	testDB, cleanup, err := db.PrepareTestDB()
	require.NoError(t, err, "Failed to prepare test DB")
	t.Cleanup(cleanup)

	res, err := squirrel.Insert("events").
		RunWith(testDB).
		Columns("name", "planning_center_id").
		Values("Existing Event", "pc_evt_99").
		ExecContext(t.Context())
	require.NoError(t, err)
	insertedID, _ := res.LastInsertId()
	require.NotZero(t, insertedID)

	_, err = squirrel.Insert("locations").
		RunWith(testDB).
		Columns("planning_center_id", "event_id", "name", "auto_fetch").
		Values("loc-1", insertedID, "Room A", true).
		ExecContext(t.Context())
	require.NoError(t, err)

	_, err = squirrel.Insert("locations").
		RunWith(testDB).
		Columns("planning_center_id", "event_id", "name", "auto_fetch").
		Values("loc-2", insertedID, "Room B", false).
		ExecContext(t.Context())
	require.NoError(t, err)

	createdEvent, syncedLocations, eventCreated, err := CreateEventWithLocations(t.Context(), testDB, event.Event{
		Name:             "Existing Event",
		PlanningCenterID: "pc_evt_99",
	}, []location.Location{})
	require.NoError(t, err)
	require.False(t, eventCreated)
	require.Empty(t, syncedLocations)
	require.Equal(t, insertedID, createdEvent.ID)

	locRepo := location.NewRepo(testDB)
	remaining, err := locRepo.ListLocations(t.Context(), location.LocationFilter{EventID: insertedID})
	require.NoError(t, err)
	assert.Empty(t, remaining)
}

func TestCreateEventWithLocations_AllowsDuplicateNames(t *testing.T) {
	testDB, cleanup, err := db.PrepareTestDB()
	require.NoError(t, err, "Failed to prepare test DB")
	t.Cleanup(cleanup)

	createdEvent, syncedLocations, eventCreated, err := CreateEventWithLocations(t.Context(), testDB, event.Event{
		Name:             "Family Service",
		PlanningCenterID: "pc_evt_dup",
	}, []location.Location{
		{PlanningCenterID: "loc-1", Name: "Room A"},
		{PlanningCenterID: "loc-2", Name: "Room A"},
	})
	require.NoError(t, err)
	require.True(t, eventCreated)
	require.NotZero(t, createdEvent.ID)
	require.Len(t, syncedLocations, 2)

	locRepo := location.NewRepo(testDB)
	allLocations, err := locRepo.ListLocations(t.Context(), location.LocationFilter{EventID: createdEvent.ID})
	require.NoError(t, err)
	require.Len(t, allLocations, 2)
}

func TestCreateEventWithLocations_SyncExisting_DuplicateNameDeletesByPlanningCenterID(t *testing.T) {
	testDB, cleanup, err := db.PrepareTestDB()
	require.NoError(t, err, "Failed to prepare test DB")
	t.Cleanup(cleanup)

	res, err := squirrel.Insert("events").
		RunWith(testDB).
		Columns("name", "planning_center_id").
		Values("Existing Event", "pc_evt_dup_sync").
		ExecContext(t.Context())
	require.NoError(t, err)
	insertedID, _ := res.LastInsertId()
	require.NotZero(t, insertedID)

	_, err = squirrel.Insert("locations").
		RunWith(testDB).
		Columns("planning_center_id", "event_id", "name", "auto_fetch").
		Values("loc-1", insertedID, "Room A", true).
		ExecContext(t.Context())
	require.NoError(t, err)

	_, err = squirrel.Insert("locations").
		RunWith(testDB).
		Columns("planning_center_id", "event_id", "name", "auto_fetch").
		Values("loc-2", insertedID, "Room A", false).
		ExecContext(t.Context())
	require.NoError(t, err)

	createdEvent, syncedLocations, eventCreated, err := CreateEventWithLocations(t.Context(), testDB, event.Event{
		Name:             "Existing Event",
		PlanningCenterID: "pc_evt_dup_sync",
	}, []location.Location{
		{PlanningCenterID: "loc-1", Name: "Room A"},
	})
	require.NoError(t, err)
	require.False(t, eventCreated)
	require.Len(t, syncedLocations, 1)
	require.Equal(t, insertedID, createdEvent.ID)

	locRepo := location.NewRepo(testDB)
	allLocations, err := locRepo.ListLocations(t.Context(), location.LocationFilter{EventID: insertedID})
	require.NoError(t, err)
	require.Len(t, allLocations, 1)
	assert.Equal(t, "loc-1", allLocations[0].PlanningCenterID)
}
