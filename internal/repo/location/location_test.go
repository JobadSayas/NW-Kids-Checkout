package location

import (
	"database/sql"
	"log"
	"os"
	"testing"

	"kids-checkin/internal/db"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testDB *sql.DB

func TestMain(m *testing.M) {
	tDB, cleanup, err := db.PrepareTestDB()
	if err != nil {
		log.Fatalf("Failed to prepare test DB: %v", err)
	}
	testDB = tDB

	code := m.Run()
	cleanup()
	os.Exit(code)
}

func Test_sqliteRepo_CreateLocation(t *testing.T) {
	s := NewRepo(testDB)

	t.Run("location created successfully", func(t *testing.T) {
		actual, err := s.CreateLocation(t.Context(), Location{
			PlanningCenterID: "pcloc_1234",
			Name:             "Cool location",
		})
		require.NoError(t, err)
		assert.NotZero(t, actual.ID)
		assert.Equal(t, "pcloc_1234", actual.PlanningCenterID)
		assert.Equal(t, "Cool location", actual.Name)

		locations, err := s.ListLocations(t.Context(), LocationFilter{
			PlanningCenterID: "pcloc_1234",
		})
		require.NoError(t, err)
		require.Len(t, locations, 1)
		assert.Equal(t, actual.ID, locations[0].ID)
		assert.Equal(t, actual.PlanningCenterID, locations[0].PlanningCenterID)
		assert.Equal(t, actual.Name, locations[0].Name)
	})

	t.Run("duplicate location should not be created", func(t *testing.T) {
		first, err := s.CreateLocation(t.Context(), Location{
			PlanningCenterID: "pcloc_1235",
			Name:             "another location",
		})
		require.NoError(t, err)

		second, err := s.CreateLocation(t.Context(), Location{
			PlanningCenterID: "pcloc_1235",
			Name:             "new location name",
		})
		require.NoError(t, err)

		assert.Equal(t, first.ID, second.ID, "duplicate location should not be created")
		assert.NotEqual(t, first.Name, second.Name, "duplicate location should have updated name")

		locations, err := s.ListLocations(t.Context(), LocationFilter{
			PlanningCenterID: "pcloc_1235",
		})
		require.NoError(t, err)
		require.Len(t, locations, 1)
		assert.Equal(t, second.ID, locations[0].ID)
		assert.Equal(t, second.Name, locations[0].Name)
		assert.Equal(t, second.PlanningCenterID, locations[0].PlanningCenterID)
	})
}
