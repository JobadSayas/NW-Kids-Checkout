package metrics

import (
	"context"
	"fmt"
	"sort"
	"time"

	"kids-checkin/internal/repo"

	"github.com/Masterminds/squirrel"
)

type DailyMetric struct {
	Date              string
	EventName         string
	Called            int
	Confirmed         int
	Unconfirmed       int
	AvgConfirmMinutes float64
	ManualCount       int
}

type Filter struct {
	Days int
}

type Repo interface {
	ListDailyMetrics(ctx context.Context, filter Filter) ([]DailyMetric, error)
}

type sqliteRepo struct {
	db repo.DBTX
}

func NewRepo(database repo.DBTX) Repo {
	return &sqliteRepo{db: database}
}

func (r *sqliteRepo) ListDailyMetrics(ctx context.Context, filter Filter) ([]DailyMetric, error) {
	days := filter.Days
	if days <= 0 {
		days = 14
	}
	since := time.Now().UTC().AddDate(0, 0, -days)

	pcRows, err := squirrel.Select(
		"date(checkins.checked_out_at) AS day",
		"COALESCE(events.name, 'Unknown') AS event_name",
		"COUNT(*) AS called",
		"COALESCE(SUM(CASE WHEN checkins.checked_out_confirmed_at IS NOT NULL THEN 1 ELSE 0 END), 0) AS confirmed",
		"COALESCE(AVG(CASE WHEN checkins.checked_out_confirmed_at IS NOT NULL THEN (julianday(checkins.checked_out_confirmed_at) - julianday(checkins.checked_out_at)) * 24 * 60 END), 0) AS avg_minutes",
	).
		From("checkins").
		LeftJoin("locations ON locations.id = checkins.location_id").
		LeftJoin("events ON events.id = COALESCE(checkins.event_id, locations.event_id)").
		Where(squirrel.GtOrEq{"checkins.checked_out_at": since}).
		GroupBy("day", "event_name").
		OrderBy("day DESC, event_name ASC").
		RunWith(r.db).
		QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("querying checkin metrics: %w", err)
	}
	defer pcRows.Close()

	daily := []DailyMetric{}
	for pcRows.Next() {
		var dm DailyMetric
		var day string
		if err := pcRows.Scan(&day, &dm.EventName, &dm.Called, &dm.Confirmed, &dm.AvgConfirmMinutes); err != nil {
			return nil, fmt.Errorf("scanning checkin metrics: %w", err)
		}
		dm.Date = day
		dm.Unconfirmed = dm.Called - dm.Confirmed
		daily = append(daily, dm)
	}
	if err := pcRows.Err(); err != nil {
		return nil, fmt.Errorf("iterating checkin metrics: %w", err)
	}

	manualRows, err := squirrel.Select(
		"date(created_at) AS day",
		"COUNT(*) AS count",
	).
		From("manual_checkins").
		Where(squirrel.GtOrEq{"created_at": since}).
		GroupBy("day").
		OrderBy("day DESC").
		RunWith(r.db).
		QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("querying manual metrics: %w", err)
	}
	defer manualRows.Close()

	manualByDay := map[string]int{}
	for manualRows.Next() {
		var day string
		var count int
		if err := manualRows.Scan(&day, &count); err != nil {
			return nil, fmt.Errorf("scanning manual metrics: %w", err)
		}
		manualByDay[day] = count
	}
	if err := manualRows.Err(); err != nil {
		return nil, fmt.Errorf("iterating manual metrics: %w", err)
	}

	// Merge manual counts into the matching day rows; add rows for manual-only days.
	for i := range daily {
		if count, ok := manualByDay[daily[i].Date]; ok {
			daily[i].ManualCount = count
			delete(manualByDay, daily[i].Date)
		}
	}
	for day, count := range manualByDay {
		daily = append(daily, DailyMetric{Date: day, EventName: "Manual Check-Ins", ManualCount: count})
	}

	// Deterministic sort: day DESC, event_name ASC.
	sort.SliceStable(daily, func(i, j int) bool {
		if daily[i].Date != daily[j].Date {
			return daily[i].Date > daily[j].Date
		}
		return daily[i].EventName < daily[j].EventName
	})

	return daily, nil
}
