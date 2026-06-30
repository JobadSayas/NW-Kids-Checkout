package event

import (
	"testing"

	"kids-checkin/internal/db"
	"kids-checkin/internal/repo"

	"github.com/Masterminds/squirrel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_sqliteRepo_GetEventByID(t *testing.T) {
	testDB, cleanup, err := db.PrepareTestDB()
	require.NoError(t, err, "Failed to prepare test DB")
	t.Cleanup(cleanup)

	eventRepo := NewRepo(testDB)

	builder := squirrel.Insert("events").
		RunWith(testDB).
		Columns("name", "planning_center_id")
	res, err := builder.Values("Sunday Service", "pc_evt_1").ExecContext(t.Context())
	require.NoError(t, err)
	insertedID, _ := res.LastInsertId()

	got, err := eventRepo.GetEventByID(t.Context(), insertedID)
	require.NoError(t, err)
	assert.Equal(t, insertedID, got.ID)
	assert.Equal(t, "Sunday Service", got.Name)
	assert.Equal(t, "pc_evt_1", got.PlanningCenterID)

	t.Run("not found", func(t *testing.T) {
		_, err := eventRepo.GetEventByID(t.Context(), 9999)
		assert.ErrorIs(t, err, repo.ErrNotFound)
	})
}

func Test_sqliteRepo_GetEventByPlanningCenterID(t *testing.T) {
	testDB, cleanup, err := db.PrepareTestDB()
	require.NoError(t, err, "Failed to prepare test DB")
	t.Cleanup(cleanup)

	eventRepo := NewRepo(testDB)

	res, err := squirrel.Insert("events").
		RunWith(testDB).
		Columns("name", "planning_center_id").
		Values("Sunday Service", "pc_evt_1").
		ExecContext(t.Context())
	require.NoError(t, err)
	insertedID, _ := res.LastInsertId()

	got, err := eventRepo.GetEventByPlanningCenterID(t.Context(), "pc_evt_1")
	require.NoError(t, err)
	assert.Equal(t, insertedID, got.ID)
	assert.Equal(t, "Sunday Service", got.Name)
	assert.Equal(t, "pc_evt_1", got.PlanningCenterID)

	t.Run("not found", func(t *testing.T) {
		_, err := eventRepo.GetEventByPlanningCenterID(t.Context(), "missing")
		assert.ErrorIs(t, err, repo.ErrNotFound)
	})
}

func Test_sqliteRepo_CreateEvent(t *testing.T) {
	testDB, cleanup, err := db.PrepareTestDB()
	require.NoError(t, err, "Failed to prepare test DB")
	t.Cleanup(cleanup)

	eventRepo := NewRepo(testDB)

	created, err := eventRepo.CreateEvent(t.Context(), Event{
		Name:             "Family Service",
		PlanningCenterID: "pc_evt_22",
	})
	require.NoError(t, err)
	require.NotZero(t, created.ID)
	assert.Equal(t, "Family Service", created.Name)
	assert.Equal(t, "pc_evt_22", created.PlanningCenterID)

	loaded, err := eventRepo.GetEventByID(t.Context(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, loaded.ID)
	assert.Equal(t, "Family Service", loaded.Name)
	assert.Equal(t, "pc_evt_22", loaded.PlanningCenterID)

	t.Run("duplicate event should not be created", func(t *testing.T) {
		_, err = eventRepo.CreateEvent(t.Context(), Event{
			Name:             "Family Service",
			PlanningCenterID: "pc_evt_22",
		})
		require.ErrorIs(t, err, ErrEventExists)

		var count int64
		err = squirrel.Select("count(*)").From("events").Where(squirrel.Eq{"planning_center_id": "pc_evt_22"}).RunWith(testDB).QueryRow().Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, int64(1), count, "Expected exactly one event with planning center ID pc_evt_22")
	})
}

func Test_sqliteRepo_UpdateEvent(t *testing.T) {
	testDB, cleanup, err := db.PrepareTestDB()
	require.NoError(t, err, "Failed to prepare test DB")
	t.Cleanup(cleanup)

	eventRepo := NewRepo(testDB)

	res, err := squirrel.Insert("events").
		RunWith(testDB).
		Columns("name", "planning_center_id").
		Values("Original Name", "pc_evt_99").
		ExecContext(t.Context())
	require.NoError(t, err)
	insertedID, _ := res.LastInsertId()

	err = eventRepo.UpdateEvent(t.Context(), Event{
		ID:               insertedID,
		Name:             "Updated Name",
		PlanningCenterID: "pc_evt_99",
		AutoFetch:        true,
	})
	require.NoError(t, err)

	loaded, err := eventRepo.GetEventByID(t.Context(), insertedID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", loaded.Name)
	assert.True(t, loaded.AutoFetch)

	err = eventRepo.UpdateEvent(t.Context(), Event{
		ID:               9999,
		Name:             "Missing",
		PlanningCenterID: "pc_evt_999",
	})
	assert.ErrorIs(t, err, repo.ErrNotFound)
}

func Test_sqliteRepo_ListEvents_filter_by_AutoFetch(t *testing.T) {
	testDB, cleanup, err := db.PrepareTestDB()
	require.NoError(t, err, "Failed to prepare test DB")
	t.Cleanup(cleanup)

	eventRepo := NewRepo(testDB)

	_, err = eventRepo.CreateEvent(t.Context(), Event{Name: "Event 1", PlanningCenterID: "pc_evt_list_1"})
	require.NoError(t, err)
	_, err = eventRepo.CreateEvent(t.Context(), Event{Name: "Event 2", PlanningCenterID: "pc_evt_list_2", AutoFetch: true})
	require.NoError(t, err)
	_, err = eventRepo.CreateEvent(t.Context(), Event{Name: "Event 3", PlanningCenterID: "pc_evt_list_3", AutoFetch: true})
	require.NoError(t, err)

	t.Run("filter by auto_fetch true", func(t *testing.T) {
		autoFetch := true
		events, err := eventRepo.ListEvents(t.Context(), EventFilter{AutoFetch: &autoFetch})
		require.NoError(t, err)
		require.Len(t, events, 2)
		for _, e := range events {
			assert.True(t, e.AutoFetch)
		}
	})

	t.Run("filter by auto_fetch false", func(t *testing.T) {
		autoFetch := false
		events, err := eventRepo.ListEvents(t.Context(), EventFilter{AutoFetch: &autoFetch})
		require.NoError(t, err)
		require.Len(t, events, 1)
		assert.False(t, events[0].AutoFetch)
	})

	t.Run("no filter returns all", func(t *testing.T) {
		events, err := eventRepo.ListEvents(t.Context(), EventFilter{})
		require.NoError(t, err)
		require.Len(t, events, 3)
	})
}

func Test_sqliteRepo_ListEvents_filter_by_LocationGroupID(t *testing.T) {
	testDB, cleanup, err := db.PrepareTestDB()
	require.NoError(t, err, "Failed to prepare test DB")
	t.Cleanup(cleanup)

	eventRepo := NewRepo(testDB)

	groupRes, err := squirrel.Insert("location_groups").
		RunWith(testDB).
		Columns("name").
		Values("Nursery").
		ExecContext(t.Context())
	require.NoError(t, err)
	groupID, _ := groupRes.LastInsertId()

	_, err = eventRepo.CreateEvent(t.Context(), Event{
		Name:             "Event 1",
		PlanningCenterID: "pc_evt_group_1",
		LocationGroupID:  &groupID,
	})
	require.NoError(t, err)
	_, err = eventRepo.CreateEvent(t.Context(), Event{
		Name:             "Event 2",
		PlanningCenterID: "pc_evt_group_2",
		LocationGroupID:  &groupID,
	})
	require.NoError(t, err)
	_, err = eventRepo.CreateEvent(t.Context(), Event{
		Name:             "Event 3",
		PlanningCenterID: "pc_evt_group_3",
	})
	require.NoError(t, err)

	t.Run("filter by location_group_id", func(t *testing.T) {
		events, err := eventRepo.ListEvents(t.Context(), EventFilter{LocationGroupID: &groupID})
		require.NoError(t, err)
		require.Len(t, events, 2)
		for _, e := range events {
			require.NotNil(t, e.LocationGroupID)
			assert.Equal(t, groupID, *e.LocationGroupID)
		}
	})

	t.Run("filter by non_existent group returns empty", func(t *testing.T) {
		nonExistentGroupID := int64(9999)
		events, err := eventRepo.ListEvents(t.Context(), EventFilter{LocationGroupID: &nonExistentGroupID})
		require.NoError(t, err)
		assert.Len(t, events, 0)
	})
}
