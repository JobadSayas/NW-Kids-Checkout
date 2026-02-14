package manualcheckinv1

import (
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"time"

	"kids-checkin/internal/controllers/middleware"
	"kids-checkin/internal/controllers/session"
	"kids-checkin/internal/repo/manualcheckin"

	"github.com/gofiber/fiber/v2"
)

const defaultCheckedOutAfterDelta = -12 * time.Hour

type Controller struct {
	manualRepo   manualcheckin.Repo
	sessionStore session.Storer
}

func NewController(db *sql.DB, sessionStore session.Storer) *Controller {
	return &Controller{
		manualRepo:   manualcheckin.NewRepo(db),
		sessionStore: sessionStore,
	}
}

func (controller *Controller) RegisterRoutes(app *fiber.App) {
	manualGroup := app.Group("/v1/checkins")
	manualGroup.Use(middleware.AuthRequired(controller.sessionStore, ""))

	manualGroup.Get("/manual", controller.GetManualCheckins)
	manualGroup.Post("/manual", controller.PostManualCheckin)
}

func (controller *Controller) GetManualCheckins(c *fiber.Ctx) error {
	filter, err := buildManualFilter(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	if filter.CheckedOutAtAfter.IsZero() {
		filter.CheckedOutAtAfter = time.Now().Add(defaultCheckedOutAfterDelta)
	}

	filter.Recent = true

	manualCheckins, err := controller.manualRepo.ListManualCheckins(c.Context(), filter)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	manualCheckins = sortManualCheckins(manualCheckins)
	return c.JSON(repoManualCheckinSliceToOutput(manualCheckins))
}

func (controller *Controller) PostManualCheckin(c *fiber.Ctx) error {
	type manualCheckinPayload struct {
		PublicID  string `json:"public_id"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	}

	var payload manualCheckinPayload
	if err := json.Unmarshal(c.Body(), &payload); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON")
	}

	if payload.FirstName == "" {
		return fiber.NewError(fiber.StatusBadRequest, "first_name is required")
	}
	if payload.LastName == "" {
		return fiber.NewError(fiber.StatusBadRequest, "last_name is required")
	}

	created, err := controller.manualRepo.CreateManualCheckin(c.Context(), manualcheckin.ManualCheckin{
		PublicID:     payload.PublicID,
		FirstName:    payload.FirstName,
		LastName:     payload.LastName,
		CheckedOutAt: time.Now().UTC(),
	})
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(repoManualCheckinToOutput(created))
}

func repoManualCheckinToOutput(manualCheckin manualcheckin.ManualCheckin) Checkin {
	var coa *time.Time
	if !manualCheckin.CheckedOutAt.IsZero() {
		coa = &manualCheckin.CheckedOutAt
	}
	var coc *time.Time
	if !manualCheckin.CheckedOutConfirmedAt.IsZero() {
		coc = &manualCheckin.CheckedOutConfirmedAt
	}
	return Checkin{
		PublicID:              manualCheckin.PublicID,
		FirstName:             manualCheckin.FirstName,
		LastName:              manualCheckin.LastName,
		CheckedOutAt:          coa,
		CheckedOutConfirmedAt: coc,
		Source:                "manual",
	}
}

func repoManualCheckinSliceToOutput(checkins []manualcheckin.ManualCheckin) []Checkin {
	output := make([]Checkin, len(checkins))
	for i := range checkins {
		output[i] = repoManualCheckinToOutput(checkins[i])
	}
	return output
}

func sortManualCheckins(checkins []manualcheckin.ManualCheckin) []manualcheckin.ManualCheckin {
	sort.Slice(checkins, func(i, j int) bool {
		if !checkins[i].CheckedOutAt.Equal(checkins[j].CheckedOutAt) {
			return checkins[i].CheckedOutAt.After(checkins[j].CheckedOutAt)
		}

		if checkins[i].LastName != checkins[j].LastName {
			return checkins[i].LastName < checkins[j].LastName
		}

		return checkins[i].FirstName < checkins[j].FirstName
	})
	return checkins
}

func buildManualFilter(c *fiber.Ctx) (manualcheckin.Filter, error) {
	filter := manualcheckin.Filter{
		FirstName: c.Query("first_name"),
		LastName:  c.Query("last_name"),
	}

	if idStr := c.Query("id"); idStr != "" {
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return manualcheckin.Filter{}, errors.New("cannot parse id")
		}
		if id < 0 {
			return manualcheckin.Filter{}, errors.New("id must be positive")
		}
		filter.ID = id
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		limitInt, err := strconv.Atoi(limitStr)
		if err != nil {
			return manualcheckin.Filter{}, errors.New("cannot parse limit")
		}
		if limitInt < 0 {
			return manualcheckin.Filter{}, errors.New("limit must be positive")
		}
		filter.Limit = limitInt
	}

	if cobStr := c.Query("checked_out_before"); cobStr != "" {
		ago, err := time.ParseDuration(cobStr)
		if err == nil {
			filter.CheckedOutAtBefore = time.Now().Add(ago)
		} else {
			filter.CheckedOutAtBefore, err = time.ParseInLocation(time.RFC3339, cobStr, time.UTC)
			if err != nil {
				return manualcheckin.Filter{}, errors.New("cannot parse checked_out_before")
			}
		}
	}

	if coaStr := c.Query("checked_out_after"); coaStr != "" {
		ago, err := time.ParseDuration(coaStr)
		if err == nil {
			filter.CheckedOutAtAfter = time.Now().Add(ago)
		} else {
			filter.CheckedOutAtAfter, err = time.ParseInLocation(time.RFC3339, coaStr, time.UTC)
			if err != nil {
				return manualcheckin.Filter{}, errors.New("cannot parse checked_out_after")
			}
		}
	}

	if !filter.CheckedOutAtAfter.IsZero() && !filter.CheckedOutAtBefore.IsZero() && filter.CheckedOutAtAfter.After(filter.CheckedOutAtBefore) {
		return manualcheckin.Filter{}, errors.New("checked_out_after must be before checked_out_before")
	}

	return filter, nil
}

type Checkin struct {
	PlanningCenterID      string     `json:"planning_center_id"`
	LocationID            int64      `json:"location_id"`
	PublicID              string     `json:"public_id"`
	FirstName             string     `json:"first_name"`
	LastName              string     `json:"last_name"`
	SecurityCode          string     `json:"security_code"`
	CheckedOutAt          *time.Time `json:"checked_out_at"`
	CheckedOutConfirmedAt *time.Time `json:"checked_out_confirmed_at"`
	Source                string     `json:"source"`
}
