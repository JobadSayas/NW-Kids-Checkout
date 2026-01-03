package location

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
)

type LocationFilter struct {
	ID               int64
	PlanningCenterID string
	LocationGroupID  int64
	AutoFetch        *bool
	Name             string
}

type LocationGroupFilter struct {
	ID   int64
	Name string
}

type LocationGroup struct {
	ID   int64
	Name string
}

type Location struct {
	ID                     int64
	PlanningCenterID       string
	PlanningCenterParentID *string
	LocationGroupID        *int64
	Name                   string
	AutoFetch              bool
	LastCheckedOutTime     time.Time
}

type Repo interface {
	ListLocations(ctx context.Context, filter LocationFilter) ([]Location, error)
	CreateLocation(ctx context.Context, location Location) (Location, error)
	UpdateLocation(ctx context.Context, location Location) error
	ListLocationGroups(ctx context.Context, filter LocationGroupFilter) ([]LocationGroup, error)
}

type sqliteRepo struct {
	db *sql.DB
}

func NewRepo(db *sql.DB) Repo {
	return &sqliteRepo{
		db: db,
	}
}

func (repo *sqliteRepo) ListLocationGroups(ctx context.Context, filter LocationGroupFilter) ([]LocationGroup, error) {
	builder := squirrel.
		Select("id", "name").
		From("location_groups").
		RunWith(repo.db)

	if filter.ID > 0 {
		builder = builder.Where(squirrel.Eq{"id": filter.ID})
	}

	if filter.Name != "" {
		builder = builder.Where(squirrel.Eq{"name": filter.Name})
	}

	rows, err := builder.QueryContext(ctx)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	groups := make([]LocationGroup, 0)
	for rows.Next() {
		var group LocationGroup
		err := rows.Scan(&group.ID, &group.Name)
		if err != nil {
			return nil, err
		}
		groups = append(groups, group)
	}

	return groups, nil
}

func (repo *sqliteRepo) CreateLocationGroup(ctx context.Context, lg LocationGroup) (LocationGroup, error) {
	builder := squirrel.
		Insert("location_groups").
		RunWith(repo.db).
		Columns("name").
		Values(lg.Name).
		SuffixExpr(squirrel.Expr("ON CONFLICT(name) DO UPDATE SET name = ?", lg.Name))
	res, err := builder.ExecContext(ctx)
	if err != nil {
		return LocationGroup{}, fmt.Errorf("inserting location group: %w", err)
	}
	lg.ID, _ = res.LastInsertId()
	return lg, nil
}

func (repo *sqliteRepo) ListLocations(ctx context.Context, filter LocationFilter) ([]Location, error) {
	builder := squirrel.
		Select(
			"id",
			"planning_center_id",
			"planning_center_parent_id",
			"location_group_id",
			"name",
			"auto_fetch",
			"last_checked_out_time",
		).
		From("locations").
		RunWith(repo.db)

	if filter.ID > 0 {
		builder = builder.Where(squirrel.Eq{"id": filter.ID})
	}

	if filter.PlanningCenterID != "" {
		builder = builder.Where(squirrel.Eq{"planning_center_id": filter.PlanningCenterID})
	}

	if filter.Name != "" {
		builder = builder.Where(squirrel.Eq{"name": filter.Name})
	}

	if filter.LocationGroupID > 0 {
		builder = builder.Where(squirrel.Eq{"location_group_id": filter.LocationGroupID})
	}

	if filter.AutoFetch != nil {
		builder = builder.Where(squirrel.Eq{"auto_fetch": filter.AutoFetch})
	}

	rows, err := builder.QueryContext(ctx)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	locations := make([]Location, 0)
	for rows.Next() {
		var location Location
		var lgIDSQL sql.NullInt64
		var pcpIDSQL sql.NullString
		var lastFetchedAtSQL sql.NullTime
		err := rows.Scan(&location.ID, &location.PlanningCenterID, &pcpIDSQL, &lgIDSQL, &location.Name, &location.AutoFetch, &lastFetchedAtSQL)
		if err != nil {
			return nil, err
		}

		if lgIDSQL.Valid {
			location.LocationGroupID = &lgIDSQL.Int64
		}
		if pcpIDSQL.Valid {
			location.PlanningCenterParentID = &pcpIDSQL.String
		}
		if lastFetchedAtSQL.Valid {
			location.LastCheckedOutTime = lastFetchedAtSQL.Time
		}

		locations = append(locations, location)
	}
	return locations, nil
}

var ErrLocationExists = errors.New("location with Planning Center ID already exists")

func (repo *sqliteRepo) CreateLocation(ctx context.Context, location Location) (Location, error) {
	columns := []string{"planning_center_id", "name", "auto_fetch"}
	values := []any{location.PlanningCenterID, location.Name, location.AutoFetch}

	if location.PlanningCenterParentID != nil {
		columns = append(columns, "planning_center_parent_id")
		values = append(values, *location.PlanningCenterParentID)
	}

	if location.LocationGroupID != nil {
		columns = append(columns, "location_group_id")
		values = append(values, *location.LocationGroupID)
	}

	if !location.LastCheckedOutTime.IsZero() {
		columns = append(columns, "last_checked_out_time")
		values = append(values, location.LastCheckedOutTime.Format(time.RFC3339))
	}

	builder := squirrel.
		Insert("locations").
		RunWith(repo.db).
		Columns(columns...).
		Values(values...).
		SuffixExpr(squirrel.Expr("ON CONFLICT(planning_center_id) DO UPDATE SET name = excluded.name"))

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

func (repo *sqliteRepo) UpdateLocation(ctx context.Context, location Location) error {
	setMap := map[string]any{
		"planning_center_id":        location.PlanningCenterID,
		"planning_center_parent_id": location.PlanningCenterParentID,
		"location_group_id":         location.LocationGroupID,
		"name":                      location.Name,
		"auto_fetch":                location.AutoFetch,
	}

	if !location.LastCheckedOutTime.IsZero() {
		setMap["last_checked_out_time"] = location.LastCheckedOutTime
	}

	builder := squirrel.
		Update("locations").
		RunWith(repo.db).
		SetMap(setMap).
		Where(squirrel.Eq{"id": location.ID})
	res, err := builder.ExecContext(ctx)
	if err != nil {
		return err
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}
	return nil
}
