package metrics

import (
	"database/sql"
	"testing"
	"time"

	"kids-checkin/internal/db"

	"github.com/Masterminds/squirrel"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fixture struct {
	testDB  *sql.DB
	eventID int64
	locID   int64
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	testDB, cleanup, err := db.PrepareTestDB()
	require.NoError(t, err, "Failed to prepare test DB")
	t.Cleanup(cleanup)

	res, err := squirrel.Insert("events").
		RunWith(testDB).
		Columns("name", "planning_center_id").
		Values("Kids Service", "pc_evt_metrics").
		ExecContext(t.Context())
	require.NoError(t, err)
	eventID, err := res.LastInsertId()
	require.NoError(t, err)

	res, err = squirrel.Insert("locations").
		RunWith(testDB).
		Columns("name", "planning_center_id", "event_id").
		Values("Room 1", "pc_loc_metrics", eventID).
		ExecContext(t.Context())
	require.NoError(t, err)
	locID, err := res.LastInsertId()
	require.NoError(t, err)

	return fixture{testDB: testDB, eventID: eventID, locID: locID}
}

func (f fixture) insertCheckin(t *testing.T, pcID string, checkedOutAt time.Time, confirmedAt *time.Time) {
	t.Helper()
	_, err := squirrel.Insert("checkins").
		RunWith(f.testDB).
		Columns("planning_center_id", "location_id", "event_id", "first_name", "last_name", "security_code", "checked_out_at", "checked_out_confirmed_at").
		Values(pcID, f.locID, f.eventID, "Alice", "Smith", "SEC1", checkedOutAt, confirmedAt).
		ExecContext(t.Context())
	require.NoError(t, err)
}

func (f fixture) insertManualCheckin(t *testing.T) {
	t.Helper()
	_, err := squirrel.Insert("manual_checkins").
		RunWith(f.testDB).
		Columns("first_name", "last_name").
		Values("Manny", "Manual").
		ExecContext(t.Context())
	require.NoError(t, err)
}

func today(t *testing.T) string {
	t.Helper()
	return time.Now().UTC().Format("2006-01-02")
}

func findMetric(t *testing.T, daily []DailyMetric, eventName string) DailyMetric {
	t.Helper()
	for _, dm := range daily {
		if dm.EventName == eventName {
			return dm
		}
	}
	return DailyMetric{}
}

func Test_sqliteRepo_ListDailyMetrics_confirmed(t *testing.T) {
	f := newFixture(t)

	checkedOut := time.Now().UTC().Add(-10 * time.Minute)
	confirmed := checkedOut.Add(5 * time.Minute)
	f.insertCheckin(t, "chk_confirmed", checkedOut, &confirmed)

	repo := NewRepo(f.testDB)
	daily, err := repo.ListDailyMetrics(t.Context(), Filter{Days: 14})
	require.NoError(t, err)

	dm := findMetric(t, daily, "Kids Service")
	require.NotEmpty(t, dm, "expected a Kids Service metric row")
	assert.Equal(t, today(t), dm.Date)
	assert.Equal(t, 1, dm.Called)
	assert.Equal(t, 1, dm.Confirmed)
	assert.Equal(t, 0, dm.Unconfirmed)
	assert.InDelta(t, 5, dm.AvgConfirmMinutes, 0.01)
}

func Test_sqliteRepo_ListDailyMetrics_unconfirmed(t *testing.T) {
	f := newFixture(t)

	checkedOut := time.Now().UTC().Add(-10 * time.Minute)
	f.insertCheckin(t, "chk_unconfirmed", checkedOut, nil)

	repo := NewRepo(f.testDB)
	daily, err := repo.ListDailyMetrics(t.Context(), Filter{Days: 14})
	require.NoError(t, err)

	dm := findMetric(t, daily, "Kids Service")
	require.NotEmpty(t, dm, "expected a Kids Service metric row")
	assert.Equal(t, today(t), dm.Date)
	assert.Equal(t, 1, dm.Called)
	assert.Equal(t, 0, dm.Confirmed)
	assert.Equal(t, 1, dm.Unconfirmed)
	assert.Equal(t, float64(0), dm.AvgConfirmMinutes)
}

func Test_sqliteRepo_ListDailyMetrics_manual_merged(t *testing.T) {
	f := newFixture(t)

	checkedOut := time.Now().UTC().Add(-10 * time.Minute)
	f.insertCheckin(t, "chk_manual", checkedOut, nil)
	f.insertManualCheckin(t)
	f.insertManualCheckin(t)

	repo := NewRepo(f.testDB)
	daily, err := repo.ListDailyMetrics(t.Context(), Filter{Days: 14})
	require.NoError(t, err)

	dm := findMetric(t, daily, "Kids Service")
	require.NotEmpty(t, dm, "expected a Kids Service metric row")
	assert.Equal(t, 2, dm.ManualCount, "manual count should merge into the PC row for the same day")
}

func Test_sqliteRepo_ListDailyMetrics_manual_only_day(t *testing.T) {
	f := newFixture(t)

	f.insertManualCheckin(t)

	repo := NewRepo(f.testDB)
	daily, err := repo.ListDailyMetrics(t.Context(), Filter{Days: 14})
	require.NoError(t, err)

	dm := findMetric(t, daily, "Manual Check-Ins")
	require.NotEmpty(t, dm, "expected a Manual Check-Ins metric row")
	assert.Equal(t, today(t), dm.Date)
	assert.Equal(t, 1, dm.ManualCount)
}

func Test_sqliteRepo_ListDailyMetrics_days_filter(t *testing.T) {
	f := newFixture(t)

	checkedOutToday := time.Now().UTC().Add(-10 * time.Minute)
	f.insertCheckin(t, "chk_today", checkedOutToday, nil)

	old := time.Now().UTC().AddDate(0, 0, -3)
	f.insertCheckin(t, "chk_old", old, nil)

	repo := NewRepo(f.testDB)

	daily, err := repo.ListDailyMetrics(t.Context(), Filter{Days: 0})
	require.NoError(t, err)
	require.Len(t, daily, 2, "Days:0 should default to 14 and include both rows")

	daily, err = repo.ListDailyMetrics(t.Context(), Filter{Days: 1})
	require.NoError(t, err)
	require.Len(t, daily, 1, "Days:1 should exclude the 3-day-old row")
	assert.Equal(t, today(t), daily[0].Date)
	assert.Equal(t, "Kids Service", daily[0].EventName)
}
