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

func Test_sqliteRepo_UpdateEventName(t *testing.T) {
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

	err = eventRepo.UpdateEventName(t.Context(), insertedID, "Updated Name")
	require.NoError(t, err)

	loaded, err := eventRepo.GetEventByID(t.Context(), insertedID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", loaded.Name)

	err = eventRepo.UpdateEventName(t.Context(), 9999, "Missing")
	assert.ErrorIs(t, err, repo.ErrNotFound)
}
