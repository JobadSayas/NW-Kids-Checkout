package checkin

import (
	"database/sql"
	"log"
	"os"
	"testing"
	"time"

	"kids-checkin/internal/db"

	"github.com/Masterminds/squirrel"
	_ "github.com/mattn/go-sqlite3"
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

func Test_sqliteRepo_ListCheckins(t *testing.T) {
	builder := squirrel.Insert("locations").
		RunWith(testDB).
		Columns("name", "planning_center_id")
	res, err := builder.Values("location1", "plloc_1234").ExecContext(t.Context())
	require.NoError(t, err)
	location1ID, _ := res.LastInsertId()
	res, err = builder.Values("location1", "plloc_1235").ExecContext(t.Context())
	require.NoError(t, err)
	location2ID, _ := res.LastInsertId()

	s := NewRepo(testDB)

	time1 := time.Date(2022, 1, 1, 12, 18, 32, 0, time.UTC)
	time2 := time1.Add(time.Hour * 24)

	_, err = s.CreateCheckin(t.Context(), Checkin{
		PlanningCenterID: "plc_1234",
		LocationID:       location1ID,
		FirstName:        "sss",
		LastName:         "aaa",
		SecurityCode:     "ABC123",
		CheckedOutAt:     time1,
	})
	require.NoError(t, err)

	_, err = s.CreateCheckin(t.Context(), Checkin{
		PlanningCenterID: "plc_1235",
		LocationID:       location2ID,
		FirstName:        "sss",
		LastName:         "aaa",
		SecurityCode:     "ABC124",
		CheckedOutAt:     time1,
	})
	require.NoError(t, err)

	_, err = s.CreateCheckin(t.Context(), Checkin{
		PlanningCenterID: "plc_1236",
		LocationID:       location2ID,
		FirstName:        "sss",
		LastName:         "aaa",
		SecurityCode:     "ABC125",
		CheckedOutAt:     time2,
	})
	require.NoError(t, err)

	t.Run("no filter", func(t *testing.T) {
		c, err := s.ListCheckins(t.Context(), Filter{})
		require.NoError(t, err)
		assert.Lenf(t, c, 3, "expected 3 checkins, got %d", len(c))
	})

	t.Run("filter by location ID", func(t *testing.T) {
		c, err := s.ListCheckins(t.Context(), Filter{LocationID: location2ID})
		require.NoError(t, err)
		require.Lenf(t, c, 2, "expected 2 checkins, got %d", len(c))
		assert.Equal(t, location2ID, c[0].LocationID)
		assert.Equal(t, "plc_1235", c[0].PlanningCenterID)
		assert.Equal(t, "sss", c[0].FirstName)
		assert.Equal(t, "aaa", c[0].LastName)
		assert.Equal(t, "ABC124", c[0].SecurityCode)
		assert.Equal(t, time1, c[0].CheckedOutAt)
	})

	t.Run("filter by Planning Center ID", func(t *testing.T) {
		c, err := s.ListCheckins(t.Context(), Filter{PlanningCenterID: "plc_1236"})
		require.NoError(t, err)
		require.Lenf(t, c, 1, "expected  checkins, got %d", len(c))
		assert.Equal(t, "plc_1236", c[0].PlanningCenterID)
	})
}

func Test_sqliteRepo_CreateCheckin(t *testing.T) {
	s := NewRepo(testDB)
	_, err := squirrel.Delete("checkins").RunWith(testDB).ExecContext(t.Context())
	require.NoError(t, err)

	tests := []struct {
		name      string
		arg       Checkin
		expected  Checkin
		expectErr bool
	}{
		{
			name: "create checkin",
			arg: Checkin{
				PlanningCenterID: "plc_1234",
				LocationID:       1,
				FirstName:        "somefirstname",
				LastName:         "somelastname",
				SecurityCode:     "ABC123",
				CheckedOutAt:     time.Date(2022, 1, 1, 12, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "create checkin 2",
			arg: Checkin{
				PlanningCenterID: "plc_1235",
				LocationID:       1,
				FirstName:        "somefirstname",
				LastName:         "somelastname",
				SecurityCode:     "ABC123",
				CheckedOutAt:     time.Time{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := s.CreateCheckin(t.Context(), tt.arg)
			if tt.expectErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.NotZero(t, actual.ID)
			assert.Equal(t, tt.arg.PlanningCenterID, actual.PlanningCenterID)
			assert.Equal(t, tt.arg.LocationID, actual.LocationID)
			assert.Equal(t, tt.arg.FirstName, actual.FirstName)
			assert.Equal(t, tt.arg.LastName, actual.LastName)
			assert.Equal(t, tt.arg.SecurityCode, actual.SecurityCode)
			assert.Equal(t, tt.arg.CheckedOutAt, actual.CheckedOutAt)

			checkins, err := s.ListCheckins(t.Context(), Filter{
				PlanningCenterID: tt.arg.PlanningCenterID,
			})
			require.NoError(t, err)
			require.Len(t, checkins, 1)

			assert.Equal(t, actual.ID, checkins[0].ID)
			assert.Equal(t, actual.PlanningCenterID, checkins[0].PlanningCenterID)
			assert.Equal(t, actual.LocationID, checkins[0].LocationID)
			assert.Equal(t, actual.FirstName, checkins[0].FirstName)
			assert.Equal(t, actual.LastName, checkins[0].LastName)
			assert.Equal(t, actual.SecurityCode, checkins[0].SecurityCode)
			assert.Equal(t, actual.CheckedOutAt, checkins[0].CheckedOutAt)
		})
	}
}

func toPointer[T comparable](v T) *T { return &v }
