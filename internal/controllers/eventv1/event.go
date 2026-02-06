package eventv1

import (
	"database/sql"
	"errors"
	"strconv"

	"kids-checkin/internal/controllers/middleware"
	"kids-checkin/internal/controllers/session"
	"kids-checkin/internal/repo"
	"kids-checkin/internal/repo/event"

	"github.com/gofiber/fiber/v2"
)

type Controller struct {
	repo         event.Repo
	sessionStore session.Storer
}

func NewController(db *sql.DB, sessionStore session.Storer) *Controller {
	return &Controller{
		repo:         event.NewRepo(db),
		sessionStore: sessionStore,
	}
}

func (controller *Controller) RegisterRoutes(app *fiber.App) {
	eventGroup := app.Group("/v1/events")
	eventGroup.Use(middleware.AuthRequired(controller.sessionStore, ""))

	eventGroup.Get("/:id", controller.GetEventByID)
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

type Event struct {
	ID               int64  `json:"id"`
	Name             string `json:"name"`
	PlanningCenterID string `json:"planning_center_id"`
}

func repoEventToOutput(event event.Event) Event {
	return Event{
		ID:               event.ID,
		Name:             event.Name,
		PlanningCenterID: event.PlanningCenterID,
	}
}
