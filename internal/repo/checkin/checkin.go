package checkin

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
)

type Filter struct {
	ID                 int64
	PlanningCenterID   string
	LocationID         int64
	LocationName       string
	LocationGroupID    int64
	LocationGroupName  string
	FirstName          string
	LastName           string
	CheckedOutAtBefore time.Time
	CheckedOutAtAfter  time.Time
	Limit              int
}

type Checkin struct {
	ID               int64
	PlanningCenterID string
	LocationID       int64
	FirstName        string
	LastName         string
	SecurityCode     string
	CheckedOutAt     time.Time
}

type Repo interface {
	ListCheckins(ctx context.Context, filter Filter) ([]Checkin, error)
	CreateCheckin(ctx context.Context, checkin Checkin) (Checkin, error)
}

type sqliteRepo struct {
	db *sql.DB
}

func NewRepo(db *sql.DB) Repo {
	return &sqliteRepo{
		db: db,
	}
}

func (s *sqliteRepo) ListCheckins(ctx context.Context, filter Filter) ([]Checkin, error) {
	joinedTables := map[string]bool{}

	builder := squirrel.Select(
		"checkins.id",
		"checkins.planning_center_id",
		"checkins.location_id",
		"checkins.first_name",
		"checkins.last_name",
		"checkins.security_code",
		"checkins.checked_out_at",
	).From("checkins")

	if filter.LocationName != "" {
		joinedTables["locations"] = true
		builder = builder.Join("locations ON locations.id = checkins.location_id")
		builder = builder.Where(squirrel.Eq{"locations.name": filter.LocationName})
	}

	if filter.ID > 0 {
		builder = builder.Where(squirrel.Eq{"checkins.id": filter.ID})
	}

	if filter.LocationGroupID > 0 {
		if !joinedTables["locations"] {
			builder = builder.Join("locations ON locations.id = checkins.location_id")
			joinedTables["locations"] = true
		}
		builder = builder.Where(squirrel.Eq{"locations.location_group_id": filter.LocationGroupID})
	}

	if filter.LocationGroupName != "" {
		if !joinedTables["locations"] {
			builder = builder.Join("locations ON locations.id = checkins.location_id")
			joinedTables["locations"] = true
		}
		if !joinedTables["location_groups"] {
			builder = builder.Join("location_groups ON location_groups.id = locations.location_group_id")
			joinedTables["location_groups"] = true
		}
		builder = builder.Where(squirrel.Eq{"location_groups.name": filter.LocationGroupName})
	}

	if filter.PlanningCenterID != "" {
		builder = builder.Where(squirrel.Eq{"checkins.planning_center_id": filter.PlanningCenterID})
	}

	if filter.FirstName != "" {
		builder = builder.Where(squirrel.Eq{"checkins.first_name": filter.FirstName})
	}

	if filter.LastName != "" {
		builder = builder.Where(squirrel.Eq{"checkins.last_name": filter.LastName})
	}

	if filter.CheckedOutAtBefore != (time.Time{}) {
		builder = builder.Where(squirrel.Lt{"checkins.checked_out_at": filter.CheckedOutAtBefore.UTC()})
	}

	if filter.CheckedOutAtAfter != (time.Time{}) {
		builder = builder.Where(squirrel.Gt{"checkins.checked_out_at": filter.CheckedOutAtAfter.UTC()})
	}

	if filter.LocationID > 0 {
		builder = builder.Where(squirrel.Eq{"checkins.location_id": filter.LocationID})
	}

	if filter.Limit > 0 {
		builder = builder.Limit(uint64(filter.Limit))
	}

	q, args := builder.MustSql()
	fmt.Println(q)
	fmt.Println(args)

	rows, err := builder.RunWith(s.db).QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("querying checkins: %w", err)
	}
	defer rows.Close()
	checkins := make([]Checkin, 0)
	for rows.Next() {
		var checkin Checkin
		var checkedOutAt sql.NullTime

		err := rows.Scan(
			&checkin.ID,
			&checkin.PlanningCenterID,
			&checkin.LocationID,
			&checkin.FirstName,
			&checkin.LastName,
			&checkin.SecurityCode,
			&checkedOutAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning checkin: %w", err)
		}

		if checkedOutAt.Valid {
			checkin.CheckedOutAt = checkedOutAt.Time
		}

		checkins = append(checkins, checkin)
	}

	return checkins, nil
}

func (s *sqliteRepo) CreateCheckin(ctx context.Context, checkin Checkin) (Checkin, error) {
	var checkedOutAt *time.Time
	if !checkin.CheckedOutAt.IsZero() {
		tt := checkin.CheckedOutAt.UTC()
		checkedOutAt = &tt
	}

	builder := squirrel.Insert("checkins").
		RunWith(s.db).
		Columns("planning_center_id", "location_id", "first_name", "last_name", "security_code", "checked_out_at").
		Values(checkin.PlanningCenterID, checkin.LocationID, checkin.FirstName, checkin.LastName, checkin.SecurityCode, checkedOutAt).
		SuffixExpr(squirrel.Expr("ON CONFLICT(planning_center_id) DO UPDATE SET checked_out_at = ?", checkedOutAt))

	res, err := builder.ExecContext(ctx)
	if err != nil {
		return Checkin{}, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return Checkin{}, err
	}

	checkin.ID = id
	return checkin, nil
}
