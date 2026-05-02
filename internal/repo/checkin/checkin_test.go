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
	_, err := squirrel.Delete("checkins").RunWith(testDB).ExecContext(t.Context())
	require.NoError(t, err)
	_, err = squirrel.Delete("locations").RunWith(testDB).ExecContext(t.Context())
	require.NoError(t, err)
	_, err = squirrel.Delete("location_groups").RunWith(testDB).ExecContext(t.Context())
	require.NoError(t, err)

	groupBuilder := squirrel.Insert("location_groups").
		RunWith(testDB).
		Columns("name")
	res, err := groupBuilder.Values("group-a").ExecContext(t.Context())
	require.NoError(t, err)
	groupAID, _ := res.LastInsertId()
	res, err = groupBuilder.Values("group-b").ExecContext(t.Context())
	require.NoError(t, err)
	groupBID, _ := res.LastInsertId()

	builder := squirrel.Insert("locations").
		RunWith(testDB).
		Columns("name", "planning_center_id", "event_id", "location_group_id")
	res, err = builder.Values("location1", "plloc_1234", 1, groupAID).ExecContext(t.Context())
	require.NoError(t, err)
	location1ID, _ := res.LastInsertId()
	res, err = builder.Values("location2", "plloc_1235", 1, groupBID).ExecContext(t.Context())
	require.NoError(t, err)
	location2ID, _ := res.LastInsertId()
	res, err = builder.Values("location3", "plloc_1236", 1, groupAID).ExecContext(t.Context())
	require.NoError(t, err)
	location3ID, _ := res.LastInsertId()

	s := NewRepo(testDB)

	time1 := time.Date(2022, 1, 1, 12, 18, 32, 0, time.UTC)
	time2 := time1.Add(time.Hour * 24)
	confirmed1 := time1.Add(30 * time.Minute)
	confirmed2 := time1.Add(45 * time.Minute)
	confirmed3 := time2.Add(15 * time.Minute)

	checkin1, err := s.CreateCheckin(t.Context(), Checkin{
		PlanningCenterID:      "plc_1234",
		LocationID:            location1ID,
		FirstName:             "sam",
		LastName:              "alpha",
		SecurityCode:          "ABC123",
		CheckedOutAt:          time1,
		CheckedOutConfirmedAt: confirmed1,
	})
	require.NoError(t, err)

	checkin2, err := s.CreateCheckin(t.Context(), Checkin{
		PlanningCenterID:      "plc_1235",
		LocationID:            location2ID,
		FirstName:             "sam",
		LastName:              "bravo",
		SecurityCode:          "ABC124",
		CheckedOutAt:          time1,
		CheckedOutConfirmedAt: confirmed2,
	})
	require.NoError(t, err)

	checkin3, err := s.CreateCheckin(t.Context(), Checkin{
		PlanningCenterID:      "plc_1236",
		LocationID:            location3ID,
		FirstName:             "alex",
		LastName:              "alpha",
		SecurityCode:          "ABC125",
		CheckedOutAt:          time2,
		CheckedOutConfirmedAt: confirmed3,
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
		require.Lenf(t, c, 1, "expected 1 checkins, got %d", len(c))
		assert.Equal(t, location2ID, c[0].LocationID)
		assert.Equal(t, "plc_1235", c[0].PlanningCenterID)
		assert.Equal(t, "sam", c[0].FirstName)
		assert.Equal(t, "bravo", c[0].LastName)
		assert.Equal(t, "ABC124", c[0].SecurityCode)
		assert.Equal(t, time1, c[0].CheckedOutAt)
		assert.Equal(t, confirmed2, c[0].CheckedOutConfirmedAt)
	})

	t.Run("filter by Planning Center ID", func(t *testing.T) {
		c, err := s.ListCheckins(t.Context(), Filter{PlanningCenterID: "plc_1236"})
		require.NoError(t, err)
		require.Lenf(t, c, 1, "expected 1 checkins, got %d", len(c))
		assert.Equal(t, "plc_1236", c[0].PlanningCenterID)
	})

	t.Run("filter by location name", func(t *testing.T) {
		c, err := s.ListCheckins(t.Context(), Filter{LocationName: "location2"})
		require.NoError(t, err)
		require.Len(t, c, 1)
		assert.Equal(t, location2ID, c[0].LocationID)
	})

	t.Run("filter by location group ID", func(t *testing.T) {
		c, err := s.ListCheckins(t.Context(), Filter{LocationGroupID: groupAID})
		require.NoError(t, err)
		require.Len(t, c, 2)
	})

	t.Run("filter by location group name", func(t *testing.T) {
		c, err := s.ListCheckins(t.Context(), Filter{LocationGroupName: "group-b"})
		require.NoError(t, err)
		require.Len(t, c, 1)
		assert.Equal(t, location2ID, c[0].LocationID)
	})

	t.Run("filter by first name", func(t *testing.T) {
		c, err := s.ListCheckins(t.Context(), Filter{FirstName: "sam"})
		require.NoError(t, err)
		require.Len(t, c, 2)
	})

	t.Run("filter by last name", func(t *testing.T) {
		c, err := s.ListCheckins(t.Context(), Filter{LastName: "alpha"})
		require.NoError(t, err)
		require.Len(t, c, 2)
	})

	t.Run("filter by checked out before", func(t *testing.T) {
		c, err := s.ListCheckins(t.Context(), Filter{CheckedOutAtBefore: time2})
		require.NoError(t, err)
		require.Len(t, c, 2)
	})

	t.Run("filter by checked out after", func(t *testing.T) {
		c, err := s.ListCheckins(t.Context(), Filter{CheckedOutAtAfter: time1})
		require.NoError(t, err)
		require.Len(t, c, 1)
		assert.Equal(t, "plc_1236", c[0].PlanningCenterID)
	})

	t.Run("filter by recent", func(t *testing.T) {
		c, err := s.ListCheckins(t.Context(), Filter{Recent: true})
		require.NoError(t, err)
		require.Len(t, c, 3)
		assert.Equal(t, "plc_1236", c[0].PlanningCenterID)
		assert.ElementsMatch(t, []string{"plc_1234", "plc_1235"}, []string{c[1].PlanningCenterID, c[2].PlanningCenterID})
	})

	t.Run("filter by limit", func(t *testing.T) {
		c, err := s.ListCheckins(t.Context(), Filter{Limit: 1, Recent: true})
		require.NoError(t, err)
		require.Len(t, c, 1)
		assert.Equal(t, "plc_1236", c[0].PlanningCenterID)
	})

	t.Run("filter by ID", func(t *testing.T) {
		c, err := s.ListCheckins(t.Context(), Filter{ID: checkin2.ID})
		require.NoError(t, err)
		require.Len(t, c, 1)
		assert.Equal(t, checkin2.ID, c[0].ID)
		assert.Equal(t, confirmed2, c[0].CheckedOutConfirmedAt)
	})

	_ = checkin1
	_ = checkin3
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
				PlanningCenterID:      "plc_1234",
				LocationID:            1,
				FirstName:             "somefirstname",
				LastName:              "somelastname",
				SecurityCode:          "ABC123",
				CheckedOutAt:          time.Date(2022, 1, 1, 12, 0, 0, 0, time.UTC),
				CheckedOutConfirmedAt: time.Date(2022, 1, 1, 12, 30, 0, 0, time.UTC),
			},
		},
		{
			name: "create checkin 2",
			arg: Checkin{
				PlanningCenterID:      "plc_1235",
				LocationID:            1,
				FirstName:             "somefirstname",
				LastName:              "somelastname",
				SecurityCode:          "ABC123",
				CheckedOutAt:          time.Time{},
				CheckedOutConfirmedAt: time.Time{},
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
			assert.Equal(t, tt.arg.CheckedOutConfirmedAt, actual.CheckedOutConfirmedAt)

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
			assert.Equal(t, actual.CheckedOutConfirmedAt, checkins[0].CheckedOutConfirmedAt)
		})
	}
}

func Test_sqliteRepo_CreateCheckin_ConflictUpdatesCheckedOutAt(t *testing.T) {
	s := NewRepo(testDB)
	_, err := squirrel.Delete("checkins").RunWith(testDB).ExecContext(t.Context())
	require.NoError(t, err)

	start := time.Date(2022, 1, 1, 12, 0, 0, 0, time.UTC)
	updated := start.Add(2 * time.Hour)
	startConfirmed := start.Add(10 * time.Minute)
	updatedConfirmed := updated.Add(10 * time.Minute)

	first, err := s.CreateCheckin(t.Context(), Checkin{
		PlanningCenterID:      "plc_conflict",
		LocationID:            1,
		FirstName:             "first",
		LastName:              "last",
		SecurityCode:          "ABC123",
		CheckedOutAt:          start,
		CheckedOutConfirmedAt: startConfirmed,
	})
	require.NoError(t, err)

	_, err = s.CreateCheckin(t.Context(), Checkin{
		PlanningCenterID:      "plc_conflict",
		LocationID:            1,
		FirstName:             "first",
		LastName:              "last",
		SecurityCode:          "ABC123",
		CheckedOutAt:          updated,
		CheckedOutConfirmedAt: updatedConfirmed,
	})
	require.NoError(t, err)

	checkins, err := s.ListCheckins(t.Context(), Filter{PlanningCenterID: "plc_conflict"})
	require.NoError(t, err)
	require.Len(t, checkins, 1)
	assert.Equal(t, first.ID, checkins[0].ID)
	assert.Equal(t, updated, checkins[0].CheckedOutAt)
	assert.Equal(t, updatedConfirmed, checkins[0].CheckedOutConfirmedAt)
}

func Test_sqliteRepo_CreateCheckin_ConflictPreservesExistingEventIDWhenUnset(t *testing.T) {
	s := NewRepo(testDB)
	_, err := squirrel.Delete("checkins").RunWith(testDB).ExecContext(t.Context())
	require.NoError(t, err)

	start := time.Date(2022, 1, 1, 12, 0, 0, 0, time.UTC)
	updated := start.Add(2 * time.Hour)

	_, err = s.CreateCheckin(t.Context(), Checkin{
		PlanningCenterID:      "plc_event_conflict",
		LocationID:            1,
		EventID:               7,
		FirstName:             "first",
		LastName:              "last",
		SecurityCode:          "ABC123",
		CheckedOutAt:          start,
		CheckedOutConfirmedAt: start.Add(10 * time.Minute),
	})
	require.NoError(t, err)

	_, err = s.CreateCheckin(t.Context(), Checkin{
		PlanningCenterID:      "plc_event_conflict",
		LocationID:            1,
		FirstName:             "first",
		LastName:              "last",
		SecurityCode:          "ABC123",
		CheckedOutAt:          updated,
		CheckedOutConfirmedAt: updated.Add(10 * time.Minute),
	})
	require.NoError(t, err)

	var eventID sql.NullInt64
	err = testDB.QueryRowContext(t.Context(), "SELECT event_id FROM checkins WHERE planning_center_id = ?", "plc_event_conflict").Scan(&eventID)
	require.NoError(t, err)
	require.True(t, eventID.Valid)
	assert.Equal(t, int64(7), eventID.Int64)
}

func Test_sqliteRepo_RemoveOldCheckins(t *testing.T) {
	s := NewRepo(testDB)

	tests := []struct {
		name      string
		olderThan time.Time
		expected  int64
		expectErr bool
		beforeFn  func(t *testing.T)
		checkFn   func(t *testing.T)
	}{
		{
			name:      "remove older than 1 day",
			olderThan: time.Now().Add(-24 * time.Hour),
			expected:  1,
			expectErr: false,
			beforeFn: func(t *testing.T) {
				_, err := squirrel.Delete("checkins").RunWith(testDB).ExecContext(t.Context())
				require.NoError(t, err)
				_, err = s.CreateCheckin(t.Context(), Checkin{
					PlanningCenterID: "plc_1234",
					LocationID:       1,
					FirstName:        "somefirstname1",
					LastName:         "somelastname1",
					SecurityCode:     "ABC123",
					CheckedOutAt:     time.Now().Add(-24 * time.Hour).Add(-1 * time.Millisecond),
				})
				require.NoError(t, err)
				_, err = s.CreateCheckin(t.Context(), Checkin{
					PlanningCenterID: "plc_4321",
					LocationID:       2,
					FirstName:        "somefirstname2",
					LastName:         "somelastname2",
					SecurityCode:     "321CBA",
					CheckedOutAt:     time.Now().Add(-2 * time.Hour),
				})
				require.NoError(t, err)
			},
			checkFn: func(t *testing.T) {
				checkins, err := s.ListCheckins(t.Context(), Filter{})
				require.NoError(t, err)
				require.Len(t, checkins, 1, "unexpected number of checkins")
				assert.Equal(t, "plc_4321", checkins[0].PlanningCenterID, "unexpected checkin")
			},
		},
		{
			name:      "older than in the future",
			olderThan: time.Now().Add(2 * time.Hour),
			expected:  0,
			expectErr: false,
			beforeFn: func(t *testing.T) {
				_, err := squirrel.Delete("checkins").RunWith(testDB).ExecContext(t.Context())
				require.NoError(t, err)
				_, err = s.CreateCheckin(t.Context(), Checkin{
					PlanningCenterID: "plc_future",
					LocationID:       1,
					FirstName:        "future",
					LastName:         "checkin",
					SecurityCode:     "FUT123",
					CheckedOutAt:     time.Now().Add(-10 * time.Minute),
				})
				require.NoError(t, err)
			},
			checkFn: func(t *testing.T) {
				checkins, err := s.ListCheckins(t.Context(), Filter{})
				require.NoError(t, err)
				require.Len(t, checkins, 1)
				assert.Equal(t, "plc_future", checkins[0].PlanningCenterID)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.beforeFn != nil {
				tt.beforeFn(t)
			}

			actual, err := s.RemoveOldCheckins(t.Context(), tt.olderThan)
			if tt.expectErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, actual)
			if tt.checkFn != nil {
				tt.checkFn(t)
			}
		})
	}
}

func Test_sqliteRepo_SetCheckedOutConfirmedAt(t *testing.T) {
	s := NewRepo(testDB)
	_, err := squirrel.Delete("checkins").RunWith(testDB).ExecContext(t.Context())
	require.NoError(t, err)

	created, err := s.CreateCheckin(t.Context(), Checkin{
		PlanningCenterID: "plc_confirmed",
		LocationID:       1,
		FirstName:        "first",
		LastName:         "last",
		SecurityCode:     "ABC123",
		CheckedOutAt:     time.Date(2022, 1, 1, 12, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)

	before := time.Now().UTC()
	updated, err := s.SetCheckedOutConfirmedAt(t.Context(), created.PlanningCenterID, true)
	require.NoError(t, err)
	assert.Equal(t, created.ID, updated.ID)
	assert.WithinDuration(t, time.Now().UTC(), updated.CheckedOutConfirmedAt, 2*time.Second)
	assert.True(t, updated.CheckedOutConfirmedAt.After(before) || updated.CheckedOutConfirmedAt.Equal(before))

	updated, err = s.SetCheckedOutConfirmedAt(t.Context(), created.PlanningCenterID, false)
	require.NoError(t, err)
	assert.True(t, updated.CheckedOutConfirmedAt.IsZero())
}
