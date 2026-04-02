package event

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"kids-checkin/internal/repo"

	"github.com/Masterminds/squirrel"
	"github.com/mattn/go-sqlite3"
)

type Event struct {
	ID               int64
	Name             string
	PlanningCenterID string
}

type Repo interface {
	GetEventByID(ctx context.Context, id int64) (Event, error)
	GetEventByPlanningCenterID(ctx context.Context, planningCenterID string) (Event, error)
	ListEvents(ctx context.Context) ([]Event, error)
	CreateEvent(ctx context.Context, event Event) (Event, error)
	UpdateEventName(ctx context.Context, id int64, name string) error
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
		Select("id", "name", "planning_center_id").
		From("events").
		Where(squirrel.Eq{"id": id}).
		Limit(1).
		RunWith(r.db)

	var event Event
	err := builder.QueryRowContext(ctx).Scan(&event.ID, &event.Name, &event.PlanningCenterID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Event{}, repo.ErrNotFound
		}
		return Event{}, fmt.Errorf("querying event: %w", err)
	}

	return event, nil
}

func (r *sqliteRepo) GetEventByPlanningCenterID(ctx context.Context, planningCenterID string) (Event, error) {
	builder := squirrel.
		Select("id", "name", "planning_center_id").
		From("events").
		Where(squirrel.Eq{"planning_center_id": planningCenterID}).
		Limit(1).
		RunWith(r.db)

	var event Event
	err := builder.QueryRowContext(ctx).Scan(&event.ID, &event.Name, &event.PlanningCenterID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Event{}, repo.ErrNotFound
		}
		return Event{}, fmt.Errorf("querying event: %w", err)
	}

	return event, nil
}

func (r *sqliteRepo) ListEvents(ctx context.Context) ([]Event, error) {
	builder := squirrel.
		Select("id", "name", "planning_center_id").
		From("events").
		OrderBy("name").
		RunWith(r.db)

	rows, err := builder.QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("querying events: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var event Event
		err := rows.Scan(&event.ID, &event.Name, &event.PlanningCenterID)
		if err != nil {
			return nil, fmt.Errorf("scanning event: %w", err)
		}
		events = append(events, event)
	}

	return events, nil
}

func (r *sqliteRepo) CreateEvent(ctx context.Context, event Event) (Event, error) {
	builder := squirrel.
		Insert("events").
		RunWith(r.db).
		Columns("name", "planning_center_id").
		Values(event.Name, event.PlanningCenterID)

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

func (r *sqliteRepo) UpdateEventName(ctx context.Context, id int64, name string) error {
	builder := squirrel.
		Update("events").
		RunWith(r.db).
		Set("name", name).
		Where(squirrel.Eq{"id": id})

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
