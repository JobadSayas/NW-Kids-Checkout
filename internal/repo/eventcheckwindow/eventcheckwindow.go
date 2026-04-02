package eventcheckwindow

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"kids-checkin/internal/repo"

	"github.com/Masterminds/squirrel"
)

type EventCheckWindow struct {
	ID             int64
	EventID        int64
	StartDayOfWeek int
	StartTime      string
	EndDayOfWeek   int
	EndTime        string
	Timezone       string
}

type Filter struct {
	ID      int64
	EventID int64
	Limit   int
}

type Repo interface {
	GetCheckWindowByID(ctx context.Context, id int64) (EventCheckWindow, error)
	GetCheckWindowsForEvent(ctx context.Context, eventID int64) ([]EventCheckWindow, error)
	ListCheckWindows(ctx context.Context, filter Filter) ([]EventCheckWindow, error)
	CreateCheckWindow(ctx context.Context, window EventCheckWindow) (EventCheckWindow, error)
	UpdateCheckWindow(ctx context.Context, window EventCheckWindow) error
	DeleteCheckWindow(ctx context.Context, id int64) error
}

type sqliteRepo struct {
	db repo.DBTX
}

func NewRepo(db repo.DBTX) Repo {
	return &sqliteRepo{
		db: db,
	}
}

func (r *sqliteRepo) GetCheckWindowByID(ctx context.Context, id int64) (EventCheckWindow, error) {
	builder := squirrel.
		Select("id", "event_id", "start_day_of_week", "start_time", "end_day_of_week", "end_time", "timezone").
		From("event_check_windows").
		Where(squirrel.Eq{"id": id}).
		Limit(1).
		RunWith(r.db)

	var w EventCheckWindow
	err := builder.QueryRowContext(ctx).Scan(&w.ID, &w.EventID, &w.StartDayOfWeek, &w.StartTime, &w.EndDayOfWeek, &w.EndTime, &w.Timezone)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return EventCheckWindow{}, repo.ErrNotFound
		}
		return EventCheckWindow{}, fmt.Errorf("querying event_check_window: %w", err)
	}

	return w, nil
}

func (r *sqliteRepo) GetCheckWindowsForEvent(ctx context.Context, eventID int64) ([]EventCheckWindow, error) {
	builder := squirrel.
		Select("id", "event_id", "start_day_of_week", "start_time", "end_day_of_week", "end_time", "timezone").
		From("event_check_windows").
		Where(squirrel.Eq{"event_id": eventID}).
		OrderBy("start_day_of_week", "start_time").
		RunWith(r.db)

	rows, err := builder.QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("querying event_check_windows: %w", err)
	}
	defer rows.Close()

	var windows []EventCheckWindow
	for rows.Next() {
		var w EventCheckWindow
		err := rows.Scan(&w.ID, &w.EventID, &w.StartDayOfWeek, &w.StartTime, &w.EndDayOfWeek, &w.EndTime, &w.Timezone)
		if err != nil {
			return nil, fmt.Errorf("scanning event_check_window: %w", err)
		}
		windows = append(windows, w)
	}

	return windows, nil
}

func (r *sqliteRepo) ListCheckWindows(ctx context.Context, filter Filter) ([]EventCheckWindow, error) {
	builder := squirrel.
		Select("id", "event_id", "start_day_of_week", "start_time", "end_day_of_week", "end_time", "timezone").
		From("event_check_windows").
		OrderBy("event_id", "start_day_of_week", "start_time").
		RunWith(r.db)

	if filter.ID > 0 {
		builder = builder.Where(squirrel.Eq{"id": filter.ID})
	}
	if filter.EventID > 0 {
		builder = builder.Where(squirrel.Eq{"event_id": filter.EventID})
	}
	if filter.Limit > 0 {
		builder = builder.Limit(uint64(filter.Limit))
	}

	rows, err := builder.QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("querying event_check_windows: %w", err)
	}
	defer rows.Close()

	var windows []EventCheckWindow
	for rows.Next() {
		var w EventCheckWindow
		err := rows.Scan(&w.ID, &w.EventID, &w.StartDayOfWeek, &w.StartTime, &w.EndDayOfWeek, &w.EndTime, &w.Timezone)
		if err != nil {
			return nil, fmt.Errorf("scanning event_check_window: %w", err)
		}
		windows = append(windows, w)
	}

	return windows, nil
}

func validateWindow(window *EventCheckWindow) error {
	if window.StartDayOfWeek < 1 || window.StartDayOfWeek > 7 {
		return fmt.Errorf("start_day_of_week must be between 1 and 7")
	}
	if window.EndDayOfWeek < 1 || window.EndDayOfWeek > 7 {
		return fmt.Errorf("end_day_of_week must be between 1 and 7")
	}

	if _, err := time.Parse("15:04", window.StartTime); err != nil {
		return fmt.Errorf("invalid start_time format, expected HH:MM")
	}
	if _, err := time.Parse("15:04", window.EndTime); err != nil {
		return fmt.Errorf("invalid end_time format, expected HH:MM")
	}

	if _, err := time.LoadLocation(window.Timezone); err != nil {
		return fmt.Errorf("invalid timezone: %w", err)
	}

	return nil
}

func (r *sqliteRepo) CreateCheckWindow(ctx context.Context, window EventCheckWindow) (EventCheckWindow, error) {
	if err := validateWindow(&window); err != nil {
		return EventCheckWindow{}, err
	}

	window.StartTime = padTime(window.StartTime)
	window.EndTime = padTime(window.EndTime)

	builder := squirrel.
		Insert("event_check_windows").
		RunWith(r.db).
		Columns("event_id", "start_day_of_week", "start_time", "end_day_of_week", "end_time", "timezone").
		Values(window.EventID, window.StartDayOfWeek, window.StartTime, window.EndDayOfWeek, window.EndTime, window.Timezone)

	res, err := builder.ExecContext(ctx)
	if err != nil {
		return EventCheckWindow{}, fmt.Errorf("inserting event_check_window: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return EventCheckWindow{}, err
	}

	window.ID = id
	return window, nil
}

func (r *sqliteRepo) UpdateCheckWindow(ctx context.Context, window EventCheckWindow) error {
	if err := validateWindow(&window); err != nil {
		return err
	}

	window.StartTime = padTime(window.StartTime)
	window.EndTime = padTime(window.EndTime)

	builder := squirrel.
		Update("event_check_windows").
		RunWith(r.db).
		Set("event_id", window.EventID).
		Set("start_day_of_week", window.StartDayOfWeek).
		Set("start_time", window.StartTime).
		Set("end_day_of_week", window.EndDayOfWeek).
		Set("end_time", window.EndTime).
		Set("timezone", window.Timezone).
		Where(squirrel.Eq{"id": window.ID})

	res, err := builder.ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("updating event_check_window: %w", err)
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return repo.ErrNotFound
	}

	return nil
}

func (r *sqliteRepo) DeleteCheckWindow(ctx context.Context, id int64) error {
	builder := squirrel.
		Delete("event_check_windows").
		Where(squirrel.Eq{"id": id}).
		RunWith(r.db)

	res, err := builder.ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("deleting event_check_window: %w", err)
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return repo.ErrNotFound
	}

	return nil
}

func padTime(t string) string {
	parts := []byte(t)
	if len(parts) == 4 && parts[1] == ':' {
		return "0" + t
	}
	if len(parts) == 5 && parts[2] == ':' {
		return t
	}
	if len(parts) == 4 && parts[0] != '0' && parts[1] == ':' {
		return "0" + t
	}
	return t
}
