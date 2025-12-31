package checkin

import (
	"context"
	"database/sql"
	"time"

	"github.com/Masterminds/squirrel"
)

type Filter struct {
	ID                 int64
	PlanningCenterID   string
	LocationID         int64
	LocationName       string
	FirstName          string
	LastName           string
	CheckedOutAtBefore time.Time
	CheckedOutAtAfter  time.Time
}

type Checkin struct {
	ID               int64  `json:"-"`
	PlanningCenterID string `json:"planning_center_id"`
	LocationID       int64  `json:"location_id"`
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
		builder = builder.Join("locations ON locations.id = checkins.location_id")
	}

	if filter.LocationName != "" {
		builder = builder.Where(squirrel.Eq{"locations.name": filter.LocationName})
	}

	if filter.ID > 0 {
		builder = builder.Where(squirrel.Eq{"checkins.id": filter.ID})
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

	rows, err := builder.RunWith(s.db).QueryContext(ctx)
	if err != nil {
		return nil, err
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
			return nil, err
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
