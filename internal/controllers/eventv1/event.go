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
	"kids-checkin/internal/repo/eventcheckwindow"
	"kids-checkin/internal/repo/location"

	"github.com/gofiber/fiber/v2"
)

type Controller struct {
	db              *sql.DB
	repo            event.Repo
	checkWindowRepo eventcheckwindow.Repo
	client          planningcenter.Client
	sessionStore    session.Storer
}

func NewController(db *sql.DB, sessionStore session.Storer) *Controller {
	return &Controller{
		db:              db,
		repo:            event.NewRepo(db),
		checkWindowRepo: eventcheckwindow.NewRepo(db),
		client:          planningcenter.NewClient(),
		sessionStore:    sessionStore,
	}
}

func (controller *Controller) RegisterRoutes(app *fiber.App) {
	eventGroup := app.Group("/v1/events")
	eventGroup.Use(middleware.AuthRequired(controller.sessionStore, ""))

	eventGroup.Get("/:id", controller.GetEventByID)

	adminGroup := app.Group("/v1/admin/events")
	adminGroup.Use(middleware.AuthRequired(controller.sessionStore, "admin"))
	adminGroup.Get("/lookup", controller.GetEventByPlanningCenterID)
	adminGroup.Post("", controller.PostCreateEvent)

	adminGroup.Get("/:eventId/check-windows", controller.GetEventCheckWindows)
	adminGroup.Post("/:eventId/check-windows", controller.PostCreateCheckWindow)
	adminGroup.Put("/:eventId/check-windows/:windowId", controller.PutUpdateCheckWindow)
	adminGroup.Delete("/:eventId/check-windows/:windowId", controller.DeleteCheckWindow)
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

	created, _, eventCreated, err := eventlocation.CreateEventWithLocations(c.Context(), controller.db, event.Event{
		Name:             pcEvent.Name,
		PlanningCenterID: pcEvent.ID,
	}, locations)
	if err != nil {
		slog.WarnContext(c.Context(), "failed to sync event", slog.String("planning_center_id", input.PlanningCenterID), slog.String("error", err.Error()))
		return err
	}

	status := fiber.StatusOK
	logMsg := "synced event from planning center"
	if eventCreated {
		status = fiber.StatusCreated
		logMsg = "created event from planning center"
	}

	slog.InfoContext(c.Context(), logMsg, slog.Int64("event_id", created.ID), slog.String("planning_center_id", input.PlanningCenterID), slog.Int("locations_count", len(locations)))

	return c.Status(status).JSON(repoEventToOutput(created))
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

func (controller *Controller) GetEventCheckWindows(c *fiber.Ctx) error {
	eventID, err := strconv.ParseInt(c.Params("eventId"), 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid event id")
	}

	windows, err := controller.checkWindowRepo.GetCheckWindowsForEvent(c.Context(), eventID)
	if err != nil {
		return err
	}

	output := make([]EventCheckWindowOutput, len(windows))
	for i, w := range windows {
		output[i] = repoCheckWindowToOutput(w)
	}

	return c.JSON(output)
}

func (controller *Controller) PostCreateCheckWindow(c *fiber.Ctx) error {
	eventID, err := strconv.ParseInt(c.Params("eventId"), 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid event id")
	}

	var input EventCheckWindowInput
	if err := json.Unmarshal(c.Body(), &input); err != nil {
		slog.WarnContext(c.Context(), "invalid check window creation payload", slog.String("error", err.Error()))
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON")
	}

	window := inputToCheckWindow(input, eventID)
	created, err := controller.checkWindowRepo.CreateCheckWindow(c.Context(), window)
	if err != nil {
		slog.WarnContext(c.Context(), "failed to create check window", slog.String("error", err.Error()))
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	slog.InfoContext(c.Context(), "created check window", slog.Int64("event_id", eventID), slog.Int64("window_id", created.ID))

	return c.Status(fiber.StatusCreated).JSON(repoCheckWindowToOutput(created))
}

func (controller *Controller) PutUpdateCheckWindow(c *fiber.Ctx) error {
	eventID, err := strconv.ParseInt(c.Params("eventId"), 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid event id")
	}

	windowID, err := strconv.ParseInt(c.Params("windowId"), 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid window id")
	}

	var input EventCheckWindowInput
	if err := json.Unmarshal(c.Body(), &input); err != nil {
		slog.WarnContext(c.Context(), "invalid check window update payload", slog.String("error", err.Error()))
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON")
	}

	window := inputToCheckWindow(input, eventID)
	window.ID = windowID

	err = controller.checkWindowRepo.UpdateCheckWindow(c.Context(), window)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return fiber.NewError(fiber.StatusNotFound, "check window not found")
		}
		slog.WarnContext(c.Context(), "failed to update check window", slog.String("error", err.Error()))
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	slog.InfoContext(c.Context(), "updated check window", slog.Int64("event_id", eventID), slog.Int64("window_id", windowID))

	return c.JSON(repoCheckWindowToOutput(window))
}

func (controller *Controller) DeleteCheckWindow(c *fiber.Ctx) error {
	windowID, err := strconv.ParseInt(c.Params("windowId"), 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid window id")
	}

	err = controller.checkWindowRepo.DeleteCheckWindow(c.Context(), windowID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return fiber.NewError(fiber.StatusNotFound, "check window not found")
		}
		return err
	}

	slog.InfoContext(c.Context(), "deleted check window", slog.Int64("window_id", windowID))

	return c.SendStatus(fiber.StatusNoContent)
}

type EventCheckWindowInput struct {
	StartDayOfWeek int    `json:"start_day_of_week"`
	StartTime      string `json:"start_time"`
	EndDayOfWeek   int    `json:"end_day_of_week"`
	EndTime        string `json:"end_time"`
	Timezone       string `json:"timezone"`
}

type EventCheckWindowOutput struct {
	ID             int64  `json:"id"`
	EventID        int64  `json:"event_id"`
	StartDayOfWeek int    `json:"start_day_of_week"`
	StartTime      string `json:"start_time"`
	EndDayOfWeek   int    `json:"end_day_of_week"`
	EndTime        string `json:"end_time"`
	Timezone       string `json:"timezone"`
}

func repoCheckWindowToOutput(w eventcheckwindow.EventCheckWindow) EventCheckWindowOutput {
	return EventCheckWindowOutput{
		ID:             w.ID,
		EventID:        w.EventID,
		StartDayOfWeek: w.StartDayOfWeek,
		StartTime:      w.StartTime,
		EndDayOfWeek:   w.EndDayOfWeek,
		EndTime:        w.EndTime,
		Timezone:       w.Timezone,
	}
}

func inputToCheckWindow(input EventCheckWindowInput, eventID int64) eventcheckwindow.EventCheckWindow {
	return eventcheckwindow.EventCheckWindow{
		EventID:        eventID,
		StartDayOfWeek: input.StartDayOfWeek,
		StartTime:      input.StartTime,
		EndDayOfWeek:   input.EndDayOfWeek,
		EndTime:        input.EndTime,
		Timezone:       input.Timezone,
	}
}
