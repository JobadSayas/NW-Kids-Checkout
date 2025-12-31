package location

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Masterminds/squirrel"
)

type Filter struct {
	ID               int64
	PlanningCenterID string
	Name             string
}

type Location struct {
	ID               int64
	PlanningCenterID string
	Name             string
}

type Repo interface {
	ListLocations(ctx context.Context, filter Filter) ([]Location, error)
	CreateLocation(ctx context.Context, location Location) (Location, error)
}

type sqliteRepo struct {
	db *sql.DB
}

func NewRepo(db *sql.DB) Repo {
	return &sqliteRepo{
		db: db,
	}
}

func (repo *sqliteRepo) ListLocations(ctx context.Context, filter Filter) ([]Location, error) {
	builder := squirrel.Select("id", "planning_center_id", "name").From("locations").RunWith(repo.db)

	if filter.ID > 0 {
		builder = builder.Where(squirrel.Eq{"id": filter.ID})
	}

	if filter.PlanningCenterID != "" {
		builder = builder.Where(squirrel.Eq{"planning_center_id": filter.PlanningCenterID})
	}

	if filter.Name != "" {
		builder = builder.Where(squirrel.Eq{"name": filter.Name})
	}

	rows, err := builder.QueryContext(ctx)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	locations := make([]Location, 0)
	for rows.Next() {
		var location Location
		err := rows.Scan(&location.ID, &location.PlanningCenterID, &location.Name)
		if err != nil {
			return nil, err
		}
		locations = append(locations, location)
	}
	return locations, nil
}

var ErrLocationExists = errors.New("location with Planning Center ID already exists")

func (repo *sqliteRepo) CreateLocation(ctx context.Context, location Location) (Location, error) {
	builder := squirrel.
		Insert("locations").
		RunWith(repo.db).
		Columns("planning_center_id", "name").
		Values(location.PlanningCenterID, location.Name).
		SuffixExpr(squirrel.Expr("ON CONFLICT(planning_center_id) DO UPDATE SET name = ?", location.Name))

	result, err := builder.ExecContext(ctx)
	if err != nil {
		return Location{}, fmt.Errorf("inserting location: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return Location{}, err
	}

	location.ID = id
	return location, nil
}
