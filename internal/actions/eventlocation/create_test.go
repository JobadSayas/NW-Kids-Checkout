package eventlocation

import (
	"testing"

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

	createdEvent, createdLocations, err := CreateEventWithLocations(t.Context(), testDB, event.Event{
		Name:             "Family Service",
		PlanningCenterID: "pc_evt_22",
	}, []location.Location{
		{PlanningCenterID: "loc-1", Name: "Room A"},
		{PlanningCenterID: "loc-2", Name: "Room B"},
	})
	require.NoError(t, err)
	require.NotZero(t, createdEvent.ID)
	require.Len(t, createdLocations, 2)

	var locationCount int64
	err = squirrel.Select("count(*)").From("locations").Where(squirrel.Eq{"event_id": createdEvent.ID}).RunWith(testDB).QueryRow().Scan(&locationCount)
	require.NoError(t, err)
	assert.Equal(t, int64(2), locationCount)
}

func TestCreateEventWithLocations_Conflict(t *testing.T) {
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

	_, _, err = CreateEventWithLocations(t.Context(), testDB, event.Event{
		Name:             "Family Service",
		PlanningCenterID: "pc_evt_22",
	}, []location.Location{
		{PlanningCenterID: "loc-1", Name: "Room A"},
	})
	require.ErrorIs(t, err, event.ErrEventExists)

	var locationCount int64
	err = squirrel.Select("count(*)").From("locations").RunWith(testDB).QueryRow().Scan(&locationCount)
	require.NoError(t, err)
	assert.Equal(t, int64(0), locationCount)
}
