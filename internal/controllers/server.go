package controllers

import (
	"database/sql"
	"errors"
	"fmt"
	"kids-checkin/internal/web/static"
	"net/http"
	"strconv"

	"kids-checkin/internal/controllers/checkinv1"
	"kids-checkin/internal/controllers/locationv1"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	_ "github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/template/html/v2"
)

func StartServer(port int, db *sql.DB) error {
	// Create a new engine
	templateEngine := html.New("internal/web/templates", ".tmpl")

	app := fiber.New(fiber.Config{
		Views: templateEngine,

		// Override default error handler
		ErrorHandler: func(ctx *fiber.Ctx, err error) error {
			// Status code defaults to 500
			code := fiber.StatusInternalServerError

			// Retrieve the custom status code if it's a *fiber.Error
			var e *fiber.Error

			message := ""
			if errors.As(err, &e) {
				message = e.Message
				code = e.Code
			}

			// Send custom error page
			err = ctx.Status(code).SendString(fmt.Sprintf(`{"sorry":"%s"}`, message))
			if err != nil {
				// In case the SendFile fails
				return ctx.Status(fiber.StatusInternalServerError).SendString("Internal Server Error")
			}

			// Return from handler
			return nil
		},
	})
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		TimeZone:   "UTC",
		TimeFormat: "2006-01-02T15:04:05Z",
	}))

	registerRoutes(app, db)

	// Serve static pages. Should be the last of all registered routes.
	app.Use("/static", filesystem.New(filesystem.Config{
		Root:       http.FS(static.NewFilteredFS()),
		PathPrefix: "",
		Browse:     true,
	}))

	err := app.Listen(":" + strconv.Itoa(port))
	if err != nil {
		return err
	}

	return nil
}

func registerRoutes(app *fiber.App, db *sql.DB) {
	checkinController := checkinv1.NewController(db)
	checkinController.RegisterRoutes(app)

	areaController := locationv1.NewController(db)
	areaController.RegisterRoutes(app)
}
