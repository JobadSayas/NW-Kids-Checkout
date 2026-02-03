package location

import (
	"database/sql"
	"testing"
	"time"

	"kids-checkin/internal/db"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_sqliteRepo_ListLocations(t *testing.T) {
	testDB, cleanup, err := db.PrepareTestDB()
	require.NoError(t, err, "Failed to prepare test DB")
	t.Cleanup(cleanup)
	s := &sqliteRepo{db: testDB}

	t.Run("empty table", func(t *testing.T) {
		locations, err := s.ListLocations(t.Context(), LocationFilter{})
		require.NoError(t, err)
		assert.Empty(t, locations)
	})

	t.Run("some filtered locations", func(t *testing.T) {
		group, err := s.CreateLocationGroup(t.Context(), LocationGroup{Name: "group-a"})
		require.NoError(t, err)
		autoFetch := true
		parentID := "parent-1"
		lastCheckedOut := time.Date(2022, 1, 1, 10, 0, 0, 0, time.UTC)
		location, err := s.CreateLocation(t.Context(), Location{
			PlanningCenterID:       "pcloc_1234",
			PlanningCenterParentID: &parentID,
			EventID:                2,
			LocationGroupID:        &group.ID,
			Name:                   "Cool location",
			AutoFetch:              true,
			LastCheckedOutTime:     lastCheckedOut,
		})
		require.NoError(t, err)

		locations, err := s.ListLocations(t.Context(), LocationFilter{ID: location.ID})
		require.NoError(t, err)
		require.Len(t, locations, 1)
		assert.Equal(t, location.ID, locations[0].ID)
		assert.Equal(t, "pcloc_1234", locations[0].PlanningCenterID)
		assert.NotNil(t, locations[0].PlanningCenterParentID)
		assert.Equal(t, parentID, *locations[0].PlanningCenterParentID)
		assert.NotNil(t, locations[0].LocationGroupID)
		assert.Equal(t, group.ID, *locations[0].LocationGroupID)
		assert.Equal(t, "Cool location", locations[0].Name)
		assert.True(t, locations[0].AutoFetch)
		assert.Equal(t, lastCheckedOut, locations[0].LastCheckedOutTime)

		locations, err = s.ListLocations(t.Context(), LocationFilter{PlanningCenterID: "pcloc_1234"})
		require.NoError(t, err)
		assert.Len(t, locations, 1)

		locations, err = s.ListLocations(t.Context(), LocationFilter{Name: "Cool location"})
		require.NoError(t, err)
		assert.Len(t, locations, 1)

		locations, err = s.ListLocations(t.Context(), LocationFilter{LocationGroupID: group.ID})
		require.NoError(t, err)
		assert.Len(t, locations, 1)

		locations, err = s.ListLocations(t.Context(), LocationFilter{AutoFetch: &autoFetch})
		require.NoError(t, err)
		assert.Len(t, locations, 1)
	})
}

func Test_sqliteRepo_CreateLocation(t *testing.T) {
	testDB, cleanup, err := db.PrepareTestDB()
	require.NoError(t, err, "Failed to prepare test DB")
	t.Cleanup(cleanup)
	s := &sqliteRepo{db: testDB}

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

	t.Run("optional fields set", func(t *testing.T) {
		group, err := s.CreateLocationGroup(t.Context(), LocationGroup{Name: "group-b"})
		require.NoError(t, err)
		parentID := "parent-2"
		lastCheckedOut := time.Date(2023, 2, 2, 11, 0, 0, 0, time.UTC)
		location, err := s.CreateLocation(t.Context(), Location{
			PlanningCenterID:       "pcloc_9999",
			PlanningCenterParentID: &parentID,
			EventID:                3,
			LocationGroupID:        &group.ID,
			Name:                   "Optional location",
			AutoFetch:              true,
			LastCheckedOutTime:     lastCheckedOut,
		})
		require.NoError(t, err)

		locations, err := s.ListLocations(t.Context(), LocationFilter{ID: location.ID})
		require.NoError(t, err)
		require.Len(t, locations, 1)
		assert.NotNil(t, locations[0].PlanningCenterParentID)
		assert.Equal(t, parentID, *locations[0].PlanningCenterParentID)
		assert.NotNil(t, locations[0].LocationGroupID)
		assert.Equal(t, group.ID, *locations[0].LocationGroupID)
		assert.Equal(t, lastCheckedOut, locations[0].LastCheckedOutTime)
	})
}

func Test_sqliteRepo_UpdateLocation(t *testing.T) {
	testDB, cleanup, err := db.PrepareTestDB()
	require.NoError(t, err, "Failed to prepare test DB")
	t.Cleanup(cleanup)
	s := &sqliteRepo{db: testDB}

	group, err := s.CreateLocationGroup(t.Context(), LocationGroup{Name: "group-update"})
	require.NoError(t, err)
	parentID := "parent-update"
	location, err := s.CreateLocation(t.Context(), Location{
		PlanningCenterID:       "pcloc_update",
		PlanningCenterParentID: &parentID,
		EventID:                4,
		LocationGroupID:        &group.ID,
		Name:                   "Original location",
		AutoFetch:              false,
	})
	require.NoError(t, err)

	updatedGroup, err := s.CreateLocationGroup(t.Context(), LocationGroup{Name: "group-updated"})
	require.NoError(t, err)
	updatedParent := "parent-updated"
	updatedTime := time.Date(2024, 3, 3, 12, 0, 0, 0, time.UTC)

	err = s.UpdateLocation(t.Context(), Location{
		ID:                     location.ID,
		PlanningCenterID:       "pcloc_update",
		PlanningCenterParentID: &updatedParent,
		EventID:                5,
		LocationGroupID:        &updatedGroup.ID,
		Name:                   "Updated location",
		AutoFetch:              true,
		LastCheckedOutTime:     updatedTime,
	})
	require.NoError(t, err)

	locations, err := s.ListLocations(t.Context(), LocationFilter{ID: location.ID})
	require.NoError(t, err)
	require.Len(t, locations, 1)
	assert.Equal(t, "Updated location", locations[0].Name)
	assert.True(t, locations[0].AutoFetch)
	assert.Equal(t, updatedTime, locations[0].LastCheckedOutTime)
	assert.NotNil(t, locations[0].PlanningCenterParentID)
	assert.Equal(t, updatedParent, *locations[0].PlanningCenterParentID)
	assert.NotNil(t, locations[0].LocationGroupID)
	assert.Equal(t, updatedGroup.ID, *locations[0].LocationGroupID)
}

func Test_sqliteRepo_UpdateLocation_NotFound(t *testing.T) {
	testDB, cleanup, err := db.PrepareTestDB()
	require.NoError(t, err, "Failed to prepare test DB")
	t.Cleanup(cleanup)
	s := &sqliteRepo{db: testDB}

	err = s.UpdateLocation(t.Context(), Location{
		ID:               999,
		PlanningCenterID: "missing",
		EventID:          1,
		Name:             "Missing",
		AutoFetch:        false,
	})
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

func Test_sqliteRepo_ListLocationGroups(t *testing.T) {
	testDB, cleanup, err := db.PrepareTestDB()
	require.NoError(t, err, "Failed to prepare test DB")
	t.Cleanup(cleanup)
	s := &sqliteRepo{db: testDB}

	groupA, err := s.CreateLocationGroup(t.Context(), LocationGroup{Name: "group-a"})
	require.NoError(t, err)
	groupB, err := s.CreateLocationGroup(t.Context(), LocationGroup{Name: "group-b"})
	require.NoError(t, err)

	groups, err := s.ListLocationGroups(t.Context(), LocationGroupFilter{})
	require.NoError(t, err)
	assert.Len(t, groups, 2)

	groups, err = s.ListLocationGroups(t.Context(), LocationGroupFilter{ID: groupA.ID})
	require.NoError(t, err)
	assert.Len(t, groups, 1)
	assert.Equal(t, groupA.ID, groups[0].ID)

	groups, err = s.ListLocationGroups(t.Context(), LocationGroupFilter{Name: "group-b"})
	require.NoError(t, err)
	assert.Len(t, groups, 1)
	assert.Equal(t, groupB.ID, groups[0].ID)
}

func Test_sqliteRepo_CreateLocationGroup(t *testing.T) {
	testDB, cleanup, err := db.PrepareTestDB()
	require.NoError(t, err, "Failed to prepare test DB")
	t.Cleanup(cleanup)
	s := &sqliteRepo{db: testDB}

	first, err := s.CreateLocationGroup(t.Context(), LocationGroup{Name: "group-unique"})
	require.NoError(t, err)
	_, err = s.CreateLocationGroup(t.Context(), LocationGroup{Name: "group-unique"})
	require.NoError(t, err)

	groups, err := s.ListLocationGroups(t.Context(), LocationGroupFilter{Name: "group-unique"})
	require.NoError(t, err)
	require.Len(t, groups, 1)
	assert.Equal(t, first.ID, groups[0].ID)
}
