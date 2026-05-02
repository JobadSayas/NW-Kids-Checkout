package checkin

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"kids-checkin/internal/repo"

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
	Recent             bool
}

type Checkin struct {
	ID                    int64
	PlanningCenterID      string
	LocationID            int64
	EventID               int64
	FirstName             string
	LastName              string
	SecurityCode          string
	CheckedOutAt          time.Time
	CheckedOutConfirmedAt time.Time
}

type Repo interface {
	ListCheckins(ctx context.Context, filter Filter) ([]Checkin, error)
	CreateCheckin(ctx context.Context, checkin Checkin) (Checkin, error)
	SetCheckedOutConfirmedAt(ctx context.Context, planningCenterID string, confirmed bool) (Checkin, error)
	RemoveOldCheckins(ctx context.Context, olderThan time.Time) (deletedCount int64, err error)
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
		"checkins.checked_out_confirmed_at",
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

	if !filter.CheckedOutAtBefore.IsZero() {
		builder = builder.Where(squirrel.Lt{"checkins.checked_out_at": filter.CheckedOutAtBefore.UTC()})
	}

	if !filter.CheckedOutAtAfter.IsZero() {
		builder = builder.Where(squirrel.Gt{"checkins.checked_out_at": filter.CheckedOutAtAfter.UTC()})
	}

	if filter.LocationID > 0 {
		builder = builder.Where(squirrel.Eq{"checkins.location_id": filter.LocationID})
	}

	if filter.Recent {
		builder = builder.OrderBy("checkins.checked_out_at DESC")
	}

	if filter.Limit > 0 {
		builder = builder.Limit(uint64(filter.Limit))
	}

	rows, err := builder.RunWith(s.db).QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("querying checkins: %w", err)
	}
	defer rows.Close()
	checkins := make([]Checkin, 0)
	for rows.Next() {
		var checkin Checkin
		var checkedOutAt sql.NullTime
		var checkedOutConfirmedAt sql.NullTime

		err := rows.Scan(
			&checkin.ID,
			&checkin.PlanningCenterID,
			&checkin.LocationID,
			&checkin.FirstName,
			&checkin.LastName,
			&checkin.SecurityCode,
			&checkedOutAt,
			&checkedOutConfirmedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning checkin: %w", err)
		}

		if checkedOutAt.Valid {
			checkin.CheckedOutAt = checkedOutAt.Time
		}

		if checkedOutConfirmedAt.Valid {
			checkin.CheckedOutConfirmedAt = checkedOutConfirmedAt.Time
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

	var checkedOutConfirmedAt *time.Time
	if !checkin.CheckedOutConfirmedAt.IsZero() {
		tt := checkin.CheckedOutConfirmedAt.UTC()
		checkedOutConfirmedAt = &tt
	}

	columns := []string{"planning_center_id", "location_id", "first_name", "last_name", "security_code", "checked_out_at", "checked_out_confirmed_at"}
	values := []any{checkin.PlanningCenterID, checkin.LocationID, checkin.FirstName, checkin.LastName, checkin.SecurityCode, checkedOutAt, checkedOutConfirmedAt}
	if checkin.EventID > 0 {
		columns = append(columns, "event_id")
		values = append(values, checkin.EventID)
	}

	conflictSuffix := squirrel.Expr("ON CONFLICT(planning_center_id) DO UPDATE SET checked_out_at = ?, checked_out_confirmed_at = ?", checkedOutAt, checkedOutConfirmedAt)
	if checkin.EventID > 0 {
		conflictSuffix = squirrel.Expr("ON CONFLICT(planning_center_id) DO UPDATE SET checked_out_at = ?, checked_out_confirmed_at = ?, event_id = ?", checkedOutAt, checkedOutConfirmedAt, checkin.EventID)
	}

	builder := squirrel.Insert("checkins").
		RunWith(s.db).
		Columns(columns...).
		Values(values...).
		SuffixExpr(conflictSuffix)

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

func (s *sqliteRepo) SetCheckedOutConfirmedAt(ctx context.Context, planningCenterID string, confirmed bool) (Checkin, error) {
	var checkedOutConfirmedAt *time.Time
	if confirmed {
		now := time.Now().UTC()
		checkedOutConfirmedAt = &now
	}

	res, err := squirrel.Update("checkins").
		Set("checked_out_confirmed_at", checkedOutConfirmedAt).
		Where(squirrel.Eq{"planning_center_id": planningCenterID}).
		RunWith(s.db).
		ExecContext(ctx)
	if err != nil {
		return Checkin{}, err
	}

	ra, err := res.RowsAffected()
	if err != nil {
		return Checkin{}, err
	}
	if ra == 0 {
		return Checkin{}, repo.ErrNotFound
	}

	checkins, err := s.ListCheckins(ctx, Filter{PlanningCenterID: planningCenterID, Limit: 1})
	if err != nil {
		return Checkin{}, err
	}
	if len(checkins) == 0 {
		return Checkin{}, repo.ErrNotFound
	}

	return checkins[0], nil
}

func (s *sqliteRepo) RemoveOldCheckins(ctx context.Context, olderThan time.Time) (int64, error) {
	if time.Now().Before(olderThan) {
		return 0, nil
	}

	res, err := squirrel.Delete("checkins").
		Where(squirrel.Lt{"checked_out_at": olderThan.UTC()}).
		RunWith(s.db).
		ExecContext(ctx)
	if err != nil {
		return 0, err
	}

	ra, _ := res.RowsAffected()
	return ra, err
}
