package eventv1

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"

	"kids-checkin/internal/actions/eventlocation"
	"kids-checkin/internal/client/planningcenter"
	"kids-checkin/internal/controllers/middleware"
	"kids-checkin/internal/controllers/session"
	"kids-checkin/internal/repo"
	"kids-checkin/internal/repo/event"
	"kids-checkin/internal/repo/location"

	"github.com/gofiber/fiber/v2"
)

type Controller struct {
	db           *sql.DB
	repo         event.Repo
	client       planningcenter.Client
	sessionStore session.Storer
}

func NewController(db *sql.DB, sessionStore session.Storer) *Controller {
	return &Controller{
		db:           db,
		repo:         event.NewRepo(db),
		client:       planningcenter.NewClient(),
		sessionStore: sessionStore,
	}
}

func (controller *Controller) RegisterRoutes(app *fiber.App) {
	eventGroup := app.Group("/v1/events")
	eventGroup.Use(middleware.AuthRequired(controller.sessionStore, ""))

	eventGroup.Get("/:id", controller.GetEventByID)

	adminGroup := app.Group("/admin/api/events")
	adminGroup.Use(middleware.AuthRequired(controller.sessionStore, "admin"))
	adminGroup.Get("/lookup", controller.GetEventByPlanningCenterID)
	adminGroup.Post("", controller.PostCreateEvent)
}

func (controller *Controller) GetEventByID(c *fiber.Ctx) error {
	cleanedStr := c.Params("id", "")
	id, err := strconv.ParseInt(cleanedStr, 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid id")
	}

	result, err := controller.repo.GetEventByID(c.Context(), id)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return fiber.NewError(fiber.StatusNotFound, "event not found")
		}
		return err
	}

	return c.JSON(repoEventToOutput(result))
}

func (controller *Controller) PostCreateEvent(c *fiber.Ctx) error {
	var input EventInput
	if err := json.Unmarshal(c.Body(), &input); err != nil {
		slog.WarnContext(c.Context(), "invalid event creation payload", slog.String("error", err.Error()))
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON")
	}

	if input.PlanningCenterID == "" {
		slog.WarnContext(c.Context(), "missing planning center id")
		return fiber.NewError(fiber.StatusBadRequest, "planning_center_id is required")
	}

	slog.InfoContext(c.Context(), "creating event from planning center", slog.String("planning_center_id", input.PlanningCenterID))

	pcEvent, err := controller.client.GetEventByID(c.Context(), input.PlanningCenterID)
	if err != nil {
		slog.WarnContext(c.Context(), "failed to fetch planning center event", slog.String("planning_center_id", input.PlanningCenterID), slog.String("error", err.Error()))
		return err
	}

	pcLocations, err := controller.client.GetLocationsForEvent(c.Context(), input.PlanningCenterID)
	if err != nil {
		slog.WarnContext(c.Context(), "failed to fetch planning center locations", slog.String("planning_center_id", input.PlanningCenterID), slog.String("error", err.Error()))
		return err
	}

	locations := make([]location.Location, 0, len(pcLocations))
	for _, pcLocation := range pcLocations {
		locations = append(locations, location.Location{
			PlanningCenterID:       pcLocation.ID,
			PlanningCenterParentID: pcLocation.ParentID,
			Name:                   pcLocation.Name,
			AutoFetch:              false,
		})
	}

	created, _, err := eventlocation.CreateEventWithLocations(c.Context(), controller.db, event.Event{
		Name:             pcEvent.Name,
		PlanningCenterID: pcEvent.ID,
	}, locations)
	if err != nil {
		if errors.Is(err, event.ErrEventExists) {
			slog.InfoContext(c.Context(), "event already exists", slog.String("planning_center_id", input.PlanningCenterID))
			return fiber.NewError(fiber.StatusConflict, "event already exists")
		}
		slog.WarnContext(c.Context(), "failed to create event", slog.String("planning_center_id", input.PlanningCenterID), slog.String("error", err.Error()))
		return err
	}

	slog.InfoContext(c.Context(), "created event from planning center", slog.Int64("event_id", created.ID), slog.String("planning_center_id", input.PlanningCenterID), slog.Int("locations_count", len(locations)))

	return c.Status(fiber.StatusCreated).JSON(repoEventToOutput(created))
}

func (controller *Controller) GetEventByPlanningCenterID(c *fiber.Ctx) error {
	planningCenterID := c.Query("planning_center_id")
	if planningCenterID == "" {
		slog.WarnContext(c.Context(), "missing planning center id")
		return fiber.NewError(fiber.StatusBadRequest, "planning_center_id is required")
	}

	slog.InfoContext(c.Context(), "looking up event by planning center id", slog.String("planning_center_id", planningCenterID))

	result, err := controller.repo.GetEventByPlanningCenterID(c.Context(), planningCenterID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			slog.InfoContext(c.Context(), "event not found for planning center id", slog.String("planning_center_id", planningCenterID))
			return fiber.NewError(fiber.StatusNotFound, "event not found")
		}
		slog.WarnContext(c.Context(), "failed to lookup event by planning center id", slog.String("planning_center_id", planningCenterID), slog.String("error", err.Error()))
		return err
	}

	return c.JSON(repoEventToOutput(result))
}

type Event struct {
	ID               int64  `json:"id"`
	Name             string `json:"name"`
	PlanningCenterID string `json:"planning_center_id"`
}

type EventInput struct {
	PlanningCenterID string `json:"planning_center_id"`
}

func repoEventToOutput(event event.Event) Event {
	return Event{
		ID:               event.ID,
		Name:             event.Name,
		PlanningCenterID: event.PlanningCenterID,
	}
}
