package admin

import (
	"kids-checkin/internal/controllers/middleware"
	"kids-checkin/internal/controllers/session"
	"kids-checkin/internal/web/static"

	"github.com/gofiber/fiber/v2"
)

type Controller struct {
	sessionStore session.Storer
}

func NewController(sessStore session.Storer) *Controller {
	return &Controller{
		sessionStore: sessStore,
	}
}

func (controller *Controller) RegisterRoutes(app *fiber.App) {
	adminGroup := app.Group("/admin")
	adminGroup.Use(middleware.AuthRequired(controller.sessionStore, "admin"))
	adminGroup.Get("/", controller.GetLanding)
	adminGroup.Get("/locations", controller.GetLocations)
	adminGroup.Get("/planningcenter/events", controller.GetPlanningCenterEvents)
	adminGroup.Get("/planningcenter/events/:id", controller.GetPlanningCenterEvent)
}

func (controller *Controller) GetLanding(c *fiber.Ctx) error {
	f, err := static.EmbeddedFS.Open("pages/admin/index.html")
	if err != nil {
		return fiber.ErrInternalServerError
	}
	defer f.Close()

	c.Type("html")
	return c.SendStream(f)
}

func (controller *Controller) GetLocations(c *fiber.Ctx) error {
	f, err := static.EmbeddedFS.Open("pages/admin/locations.html")
	if err != nil {
		return fiber.ErrInternalServerError
	}
	defer f.Close()

	c.Type("html")
	return c.SendStream(f)
}

func (controller *Controller) GetPlanningCenterEvents(c *fiber.Ctx) error {
	f, err := static.EmbeddedFS.Open("pages/admin/planningcenter-events.html")
	if err != nil {
		return fiber.ErrInternalServerError
	}
	defer f.Close()

	c.Type("html")
	return c.SendStream(f)
}

func (controller *Controller) GetPlanningCenterEvent(c *fiber.Ctx) error {
	f, err := static.EmbeddedFS.Open("pages/admin/planningcenter-event.html")
	if err != nil {
		return fiber.ErrInternalServerError
	}
	defer f.Close()

	c.Type("html")
	return c.SendStream(f)
}
