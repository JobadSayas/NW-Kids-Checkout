package eventlocation

import (
	"testing"

	"kids-checkin/internal/db"
	"kids-checkin/internal/repo"
	"kids-checkin/internal/repo/event"
	"kids-checkin/internal/repo/location"

	"github.com/Masterminds/squirrel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleteEventWithDependents(t *testing.T) {
	testDB, cleanup, err := db.PrepareTestDB()
	require.NoError(t, err, "Failed to prepare test DB")
	t.Cleanup(cleanup)

	eventRepo := event.NewRepo(testDB)
	createdEvent, err := eventRepo.CreateEvent(t.Context(), event.Event{
		Name:             "Kids Service",
		PlanningCenterID: "pc_evt_del_dep",
	})
	require.NoError(t, err)

	locRepo := location.NewRepo(testDB)
	loc1, err := locRepo.CreateLocation(t.Context(), location.Location{
		PlanningCenterID: "pc_loc_1",
		EventID:          createdEvent.ID,
		Name:             "Room 1",
	})
	require.NoError(t, err)

	loc2, err := locRepo.CreateLocation(t.Context(), location.Location{
		PlanningCenterID: "pc_loc_2",
		EventID:          createdEvent.ID,
		Name:             "Room 2",
	})
	require.NoError(t, err)

	// Check-in tied via event_id and location_id
	_, err = squirrel.Insert("checkins").
		RunWith(testDB).
		Columns("planning_center_id", "location_id", "event_id", "first_name", "last_name", "security_code").
		Values("chk_1", loc1.ID, createdEvent.ID, "Alice", "Smith", "SEC1").
		ExecContext(t.Context())
	require.NoError(t, err)

	// Check-in tied via location_id where event_id might be null
	_, err = squirrel.Insert("checkins").
		RunWith(testDB).
		Columns("planning_center_id", "location_id", "first_name", "last_name", "security_code").
		Values("chk_2", loc2.ID, "Bob", "Smith", "SEC2").
		ExecContext(t.Context())
	require.NoError(t, err)

	// Event check window
	_, err = squirrel.Insert("event_check_windows").
		RunWith(testDB).
		Columns("event_id", "start_day_of_week", "start_time", "end_day_of_week", "end_time", "timezone").
		Values(createdEvent.ID, 1, "09:00:00", 1, "12:00:00", "America/New_York").
		ExecContext(t.Context())
	require.NoError(t, err)

	// Another event to ensure isolation
	otherEvent, err := eventRepo.CreateEvent(t.Context(), event.Event{
		Name:             "Youth Service",
		PlanningCenterID: "pc_evt_other",
	})
	require.NoError(t, err)
	_, err = locRepo.CreateLocation(t.Context(), location.Location{
		PlanningCenterID: "pc_loc_other",
		EventID:          otherEvent.ID,
		Name:             "Youth Room",
	})
	require.NoError(t, err)

	// Execute delete
	err = DeleteEventWithDependents(t.Context(), testDB, createdEvent.ID)
	require.NoError(t, err)

	// Verify event deleted
	_, err = eventRepo.GetEventByID(t.Context(), createdEvent.ID)
	require.ErrorIs(t, err, repo.ErrNotFound)

	// Verify locations deleted
	locs, err := locRepo.ListLocations(t.Context(), location.LocationFilter{EventID: createdEvent.ID})
	require.NoError(t, err)
	assert.Empty(t, locs)

	// Verify checkins deleted
	var checkinCount int64
	err = squirrel.Select("count(*)").
		From("checkins").
		Where(squirrel.Or{
			squirrel.Eq{"event_id": createdEvent.ID},
			squirrel.Eq{"location_id": loc1.ID},
			squirrel.Eq{"location_id": loc2.ID},
		}).
		RunWith(testDB).
		QueryRowContext(t.Context()).
		Scan(&checkinCount)
	require.NoError(t, err)
	assert.Equal(t, int64(0), checkinCount)

	// Verify event_check_windows deleted
	var windowCount int64
	err = squirrel.Select("count(*)").
		From("event_check_windows").
		Where(squirrel.Eq{"event_id": createdEvent.ID}).
		RunWith(testDB).
		QueryRowContext(t.Context()).
		Scan(&windowCount)
	require.NoError(t, err)
	assert.Equal(t, int64(0), windowCount)

	// Verify other event and location still exist
	loadedOther, err := eventRepo.GetEventByID(t.Context(), otherEvent.ID)
	require.NoError(t, err)
	assert.Equal(t, "Youth Service", loadedOther.Name)
	otherLocs, err := locRepo.ListLocations(t.Context(), location.LocationFilter{EventID: otherEvent.ID})
	require.NoError(t, err)
	assert.Len(t, otherLocs, 1)

	// Calling DeleteEventWithDependents again returns ErrNotFound
	err = DeleteEventWithDependents(t.Context(), testDB, createdEvent.ID)
	require.ErrorIs(t, err, repo.ErrNotFound)
}
