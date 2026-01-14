package locationgroupv1

import (
	"database/sql"
	"strconv"
	"strings"

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
	locationGroup := app.Group("/v1/location_groups")

	locationGroup.Get("", controller.GetListLocationGroups)
}

func (controller *Controller) GetListLocationGroups(c *fiber.Ctx) error {
	var id int64

	cleanedStr := strings.TrimSpace(c.Params("id", ""))
	if cleanedStr != "" {
		var err error
		id, err = strconv.ParseInt(cleanedStr, 10, 64)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid id")
		}
	}

	locationGroups, err := controller.repo.ListLocationGroups(c.Context(), location.LocationGroupFilter{
		ID:   id,
		Name: c.Query("name"),
	})
	if err != nil {
		return err
	}

	return c.JSON(repoLocationGroupSliceToOutput(locationGroups))
}

type LocationGroup struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func repoLocationGroupToOutput(location location.LocationGroup) LocationGroup {
	return LocationGroup{
		ID:   location.ID,
		Name: location.Name,
	}
}

func repoLocationGroupSliceToOutput(locations []location.LocationGroup) []LocationGroup {
	output := make([]LocationGroup, len(locations))
	for i := range locations {
		output[i] = repoLocationGroupToOutput(locations[i])
	}
	return output
}
