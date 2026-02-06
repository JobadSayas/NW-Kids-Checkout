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
