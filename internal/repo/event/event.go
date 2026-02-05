package event

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Masterminds/squirrel"
)

type Event struct {
	ID               int64
	Name             string
	PlanningCenterID string
}

type Repo interface {
	GetEventByID(ctx context.Context, id int64) (Event, error)
}

type sqliteRepo struct {
	db *sql.DB
}

func NewRepo(db *sql.DB) Repo {
	return &sqliteRepo{
		db: db,
	}
}

func (repo *sqliteRepo) GetEventByID(ctx context.Context, id int64) (Event, error) {
	builder := squirrel.
		Select("id", "name", "planning_center_id").
		From("events").
		Where(squirrel.Eq{"id": id}).
		Limit(1).
		RunWith(repo.db)

	var event Event
	err := builder.QueryRowContext(ctx).Scan(&event.ID, &event.Name, &event.PlanningCenterID)
	if err != nil {
		return Event{}, fmt.Errorf("querying event: %w", err)
	}

	return event, nil
}
