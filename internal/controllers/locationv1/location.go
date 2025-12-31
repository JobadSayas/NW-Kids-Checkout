package locationv1

import (
	"database/sql"
	"encoding/json"
	"errors"

	"kids-checkin/internal/repo/location"

	"github.com/gofiber/fiber/v2"
)

type Controller struct {
	repo location.Repo
}

func NewController(db *sql.DB) *Controller {
	return &Controller{
		repo: location.NewRepo(db),
	}
}

func (controller *Controller) RegisterRoutes(app *fiber.App) {
	locationGroup := app.Group("/v1/locations")

	locationGroup.Get("", controller.GetListLocations)
	locationGroup.Post("", controller.PostCreateLocation)
}

func (controller *Controller) GetListLocations(c *fiber.Ctx) error {
	locations, err := controller.repo.ListLocations(c.Context(), location.Filter{
		Name:             c.Query("name"),
		PlanningCenterID: c.Query("planning_center_id"),
	})
	if err != nil {
		return err
	}

	return c.JSON(repoLocationSliceToOutput(locations))
}

func (controller *Controller) PostCreateLocation(c *fiber.Ctx) error {
	a := Location{}
	err := json.Unmarshal(c.Body(), &a)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON")
	}

	if a.Name == "" {
		return fiber.NewError(fiber.StatusBadRequest, "name is required")
	}

	_, err = controller.repo.CreateLocation(c.Context(), location.Location{
		Name:             a.Name,
		PlanningCenterID: a.PublicID,
	})

	if err != nil {
		if errors.Is(err, location.ErrLocationExists) {
			return fiber.NewError(fiber.StatusBadRequest, "location already exists")
		}
		return err
	}

	return nil
}

type Location struct {
	PublicID string `json:"public_id"`
	Name     string `json:"name"`
}

func repoLocationToOutput(location location.Location) Location {
	return Location{
		PublicID: location.PlanningCenterID,
		Name:     location.Name,
	}
}

func repoLocationSliceToOutput(locations []location.Location) []Location {
	output := make([]Location, len(locations))
	for i := range locations {
		output[i] = repoLocationToOutput(locations[i])
	}
	return output
}
