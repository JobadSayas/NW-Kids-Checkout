package metrics

import (
	"context"
	"database/sql"
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

type FetchLatencyMetric struct {
	Date  string
	Count int
	AvgMs float64
	P95Ms float64
	P99Ms float64
}

type Repo interface {
	ListDailyMetrics(ctx context.Context, filter Filter) ([]DailyMetric, error)
	ListFetchLatency(ctx context.Context, filter Filter) ([]FetchLatencyMetric, error)
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

// percentile returns the nearest-rank percentile of deltas (milliseconds).
func percentile(sorted []int64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	rank := int((p / 100 * float64(len(sorted))) + 0.5)
	if rank < 1 {
		rank = 1
	}
	if rank > len(sorted) {
		rank = len(sorted)
	}
	return float64(sorted[rank-1])
}

func (r *sqliteRepo) ListFetchLatency(ctx context.Context, filter Filter) ([]FetchLatencyMetric, error) {
	days := filter.Days
	if days <= 0 {
		days = 14
	}
	since := time.Now().UTC().AddDate(0, 0, -days)

	rows, err := squirrel.Select(
		"checkins.checked_out_at",
		"checkins.fetched_at",
	).
		From("checkins").
		Where(squirrel.GtOrEq{"checkins.checked_out_at": since}).
		Where("checkins.fetched_at IS NOT NULL").
		RunWith(r.db).
		QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("querying fetch latency: %w", err)
	}
	defer rows.Close()

	// Group deltas by UTC date in code; SQLite has no percentile aggregate.
	latencyByDate := make(map[string][]int64)
	for rows.Next() {
		var checkedOutAt sql.NullTime
		var fetchedAt sql.NullTime
		if err := rows.Scan(&checkedOutAt, &fetchedAt); err != nil {
			return nil, fmt.Errorf("scanning fetch latency: %w", err)
		}
		if !checkedOutAt.Valid || !fetchedAt.Valid {
			continue
		}
		deltaMs := fetchedAt.Time.Sub(checkedOutAt.Time).Milliseconds()
		if deltaMs <= 0 {
			continue
		}
		day := checkedOutAt.Time.UTC().Format("2006-01-02")
		latencyByDate[day] = append(latencyByDate[day], deltaMs)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating fetch latency: %w", err)
	}

	metrics := make([]FetchLatencyMetric, 0, len(latencyByDate))
	for day, deltas := range latencyByDate {
		var sum int64
		for _, d := range deltas {
			sum += d
		}
		sort.Slice(deltas, func(i, j int) bool {
			return deltas[i] < deltas[j]
		})
		metrics = append(metrics, FetchLatencyMetric{
			Date:  day,
			Count: len(deltas),
			AvgMs: float64(sum) / float64(len(deltas)),
			P95Ms: percentile(deltas, 95),
			P99Ms: percentile(deltas, 99),
		})
	}

	sort.SliceStable(metrics, func(i, j int) bool {
		return metrics[i].Date > metrics[j].Date
	})

	return metrics, nil
}
