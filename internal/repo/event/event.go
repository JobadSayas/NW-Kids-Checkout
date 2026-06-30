package event

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"kids-checkin/internal/repo"

	"github.com/Masterminds/squirrel"
	"github.com/mattn/go-sqlite3"
)

type EventFilter struct {
	ID               int64
	Name             string
	PlanningCenterID string
	AutoFetch        *bool
	LocationGroupID  *int64
}

type Event struct {
	ID                 int64
	Name               string
	PlanningCenterID   string
	AutoFetch          bool
	LastCheckedOutTime time.Time
	LocationGroupID    *int64
}

type Repo interface {
	GetEventByID(ctx context.Context, id int64) (Event, error)
	GetEventByPlanningCenterID(ctx context.Context, planningCenterID string) (Event, error)
	ListEvents(ctx context.Context, filter EventFilter) ([]Event, error)
	CreateEvent(ctx context.Context, event Event) (Event, error)
	UpdateEvent(ctx context.Context, event Event) error
}

type sqliteRepo struct {
	db repo.DBTX
}

var ErrEventExists = errors.New("event already exists")

func NewRepo(db repo.DBTX) Repo {
	return &sqliteRepo{
		db: db,
	}
}

func (r *sqliteRepo) GetEventByID(ctx context.Context, id int64) (Event, error) {
	builder := squirrel.
		Select("id", "name", "planning_center_id", "auto_fetch", "last_checked_out_time", "location_group_id").
		From("events").
		Where(squirrel.Eq{"id": id}).
		Limit(1).
		RunWith(r.db)

	var event Event
	var lastCheckedOutSQL sql.NullTime
	var locationGroupIDSQL sql.NullInt64
	err := builder.QueryRowContext(ctx).Scan(&event.ID, &event.Name, &event.PlanningCenterID, &event.AutoFetch, &lastCheckedOutSQL, &locationGroupIDSQL)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Event{}, repo.ErrNotFound
		}
		return Event{}, fmt.Errorf("querying event: %w", err)
	}
	if lastCheckedOutSQL.Valid {
		event.LastCheckedOutTime = lastCheckedOutSQL.Time
	}
	if locationGroupIDSQL.Valid {
		event.LocationGroupID = &locationGroupIDSQL.Int64
	}

	return event, nil
}

func (r *sqliteRepo) GetEventByPlanningCenterID(ctx context.Context, planningCenterID string) (Event, error) {
	builder := squirrel.
		Select("id", "name", "planning_center_id", "auto_fetch", "last_checked_out_time", "location_group_id").
		From("events").
		Where(squirrel.Eq{"planning_center_id": planningCenterID}).
		Limit(1).
		RunWith(r.db)

	var event Event
	var lastCheckedOutSQL sql.NullTime
	var locationGroupIDSQL sql.NullInt64
	err := builder.QueryRowContext(ctx).Scan(&event.ID, &event.Name, &event.PlanningCenterID, &event.AutoFetch, &lastCheckedOutSQL, &locationGroupIDSQL)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Event{}, repo.ErrNotFound
		}
		return Event{}, fmt.Errorf("querying event: %w", err)
	}
	if lastCheckedOutSQL.Valid {
		event.LastCheckedOutTime = lastCheckedOutSQL.Time
	}
	if locationGroupIDSQL.Valid {
		event.LocationGroupID = &locationGroupIDSQL.Int64
	}

	return event, nil
}

func (r *sqliteRepo) ListEvents(ctx context.Context, filter EventFilter) ([]Event, error) {
	builder := squirrel.
		Select("id", "name", "planning_center_id", "auto_fetch", "last_checked_out_time", "location_group_id").
		From("events").
		OrderBy("name").
		RunWith(r.db)

	if filter.ID > 0 {
		builder = builder.Where(squirrel.Eq{"id": filter.ID})
	}

	if filter.Name != "" {
		builder = builder.Where(squirrel.Eq{"name": filter.Name})
	}

	if filter.PlanningCenterID != "" {
		builder = builder.Where(squirrel.Eq{"planning_center_id": filter.PlanningCenterID})
	}

	if filter.AutoFetch != nil {
		builder = builder.Where(squirrel.Eq{"auto_fetch": *filter.AutoFetch})
	}

	if filter.LocationGroupID != nil {
		builder = builder.Where(squirrel.Eq{"location_group_id": *filter.LocationGroupID})
	}

	rows, err := builder.QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("querying events: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var event Event
		var lastCheckedOutSQL sql.NullTime
		var locationGroupIDSQL sql.NullInt64
		err := rows.Scan(&event.ID, &event.Name, &event.PlanningCenterID, &event.AutoFetch, &lastCheckedOutSQL, &locationGroupIDSQL)
		if err != nil {
			return nil, fmt.Errorf("scanning event: %w", err)
		}
		if lastCheckedOutSQL.Valid {
			event.LastCheckedOutTime = lastCheckedOutSQL.Time
		}
		if locationGroupIDSQL.Valid {
			event.LocationGroupID = &locationGroupIDSQL.Int64
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating events: %w", err)
	}

	return events, nil
}

func (r *sqliteRepo) CreateEvent(ctx context.Context, event Event) (Event, error) {
	columns := []string{"name", "planning_center_id"}
	values := []any{event.Name, event.PlanningCenterID}

	if event.AutoFetch {
		columns = append(columns, "auto_fetch")
		values = append(values, event.AutoFetch)
	}

	if event.LocationGroupID != nil {
		columns = append(columns, "location_group_id")
		values = append(values, *event.LocationGroupID)
	}

	builder := squirrel.
		Insert("events").
		RunWith(r.db).
		Columns(columns...).
		Values(values...)

	res, err := builder.ExecContext(ctx)
	if err != nil {
		var sqliteErr sqlite3.Error
		if errors.As(err, &sqliteErr) && (sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique || sqliteErr.ExtendedCode == sqlite3.ErrConstraintPrimaryKey) {
			return Event{}, ErrEventExists
		}
		return Event{}, fmt.Errorf("inserting event: %w", err)
	}

	insertedID, err := res.LastInsertId()
	if err != nil {
		return Event{}, err
	}

	event.ID = insertedID
	return event, nil
}

func (r *sqliteRepo) UpdateEvent(ctx context.Context, event Event) error {
	setMap := map[string]any{
		"name":              event.Name,
		"auto_fetch":        event.AutoFetch,
		"location_group_id": event.LocationGroupID,
	}

	if !event.LastCheckedOutTime.IsZero() {
		setMap["last_checked_out_time"] = event.LastCheckedOutTime.Format(time.RFC3339)
	}

	builder := squirrel.
		Update("events").
		RunWith(r.db).
		SetMap(setMap).
		Where(squirrel.Eq{"id": event.ID})

	res, err := builder.ExecContext(ctx)
	if err != nil {
		var sqliteErr sqlite3.Error
		if errors.As(err, &sqliteErr) && (sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique || sqliteErr.ExtendedCode == sqlite3.ErrConstraintPrimaryKey) {
			return ErrEventExists
		}
		return fmt.Errorf("updating event: %w", err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return repo.ErrNotFound
	}
	return nil
}
