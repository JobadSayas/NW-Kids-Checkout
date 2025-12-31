package checkinv1

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"sort"
	"strconv"
	"time"

	"kids-checkin/internal/repo/checkin"
	"kids-checkin/internal/repo/location"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
)

const defaultCheckedOutAfterDelta = -31 * time.Minute

type Controller struct {
	checkinRepo  checkin.Repo
	locationRepo location.Repo
	wsClients    map[*websocket.Conn]*wsClient
}

type wsClient struct {
	checkedOutAfterDelta time.Duration
	location             string
}

func NewController(db *sql.DB) *Controller {
	return &Controller{
		checkinRepo:  checkin.NewRepo(db),
		locationRepo: location.NewRepo(db),
		wsClients:    make(map[*websocket.Conn]*wsClient),
	}
}

func (controller *Controller) RegisterRoutes(app *fiber.App) {
	checkinGroup := app.Group("/v1/checkins")

	checkinGroup.Get("/checkouts/:location", controller.Checkouts)
}

func (controller *Controller) Checkouts(c *fiber.Ctx) error {
	if websocket.IsWebSocketUpgrade(c) {
		return controller.checkoutsWebsocket(c)
	}

	return controller.checkoutsWeb(c)
}

func (controller *Controller) checkoutsWeb(c *fiber.Ctx) error {
	accepts := c.Accepts("application/json", "text/html")
	if accepts == "" {
		return fiber.NewError(fiber.StatusUnsupportedMediaType, "unsupported media type")
	}

	filter, err := buildFilter(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	if filter.CheckedOutAtAfter.IsZero() {
		filter.CheckedOutAtAfter = time.Now().Add(defaultCheckedOutAfterDelta)
	}

	checkins, err := controller.checkinRepo.ListCheckins(c.Context(), filter)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	checkins = sortCheckins(checkins)
	msg, err := json.Marshal(repoCheckinSliceToOutput(checkins))
	if err != nil {
		slog.WarnContext(c.Context(), "cannot marshal checkins", slog.String("error", err.Error()))
	}

	locations, err := controller.locationRepo.ListLocations(c.Context(), location.Filter{})
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	locations = append(locations, location.Location{
		ID:               3333,
		PlanningCenterID: "ffff",
		Name:             "Another ONE",
	})

	if accepts == "text/html" {
		return c.Render("checkoutsv1/checkouts", fiber.Map{
			"Location":  filter.LocationName,
			"Locations": locations,
			"Checkouts": string(msg),
		})
	} else {
		return c.JSON(repoCheckinSliceToOutput(checkins))
	}
}

func (controller *Controller) checkoutsWebsocket(c *fiber.Ctx) error {
	return websocket.New(func(webscocketConn *websocket.Conn) {
		defer webscocketConn.Close()
		controller.wsClients[webscocketConn] = &wsClient{
			checkedOutAfterDelta: defaultCheckedOutAfterDelta,
		}
		// c.Locals is added to the *websocket.Conn
		slog.InfoContext(
			c.Context(),
			"client connected to websocket",
			slog.String("allowed", fmt.Sprintf("%v", webscocketConn.Locals("allowed"))),
			slog.String("id", webscocketConn.Params("id")),
			slog.String("v", webscocketConn.Query("v")),
			slog.String("session", webscocketConn.Cookies("session")),
		)

		// websocket.Conn bindings https://pkg.go.dev/github.com/fasthttp/websocket?tab=doc#pkg-index
		var (
			mt  int
			msg []byte
			err error
		)

		for {
			if mt, msg, err = webscocketConn.ReadMessage(); err != nil {
				slog.WarnContext(c.Context(), "error reading from websocket", slog.String("error", err.Error()))
				delete(controller.wsClients, webscocketConn)
				break
			}
			filter := CheckinFilter{}
			err = json.Unmarshal(msg, &filter)
			if err != nil {
				slog.WarnContext(c.Context(), "cannot unmarshal filter", slog.String("error", err.Error()))
				continue
			}

			slog.InfoContext(c.Context(), "recv filter", slog.String("filter", fmt.Sprintf("%+v", filter)))

			checkins, err := controller.checkinRepo.ListCheckins(context.Background(), checkin.Filter{
				LocationName:      controller.wsClients[webscocketConn].location,
				CheckedOutAtAfter: time.Now().Add(controller.wsClients[webscocketConn].checkedOutAfterDelta),
			})
			if err != nil {
				slog.WarnContext(c.Context(), "cannot list checkins", slog.String("error", err.Error()))
				continue
			}

			checkins = sortCheckins(checkins)
			msg, err = json.Marshal(repoCheckinSliceToOutput(checkins))
			if err != nil {
				slog.WarnContext(c.Context(), "cannot marshal checkins", slog.String("error", err.Error()))
				continue
			}

			if err = webscocketConn.WriteMessage(mt, msg); err != nil {
				slog.WarnContext(c.Context(), "cannot write to websocket", slog.String("error", err.Error()))
				webscocketConn.Close()

				delete(controller.wsClients, webscocketConn)

				break
			}
		}
	})(c)
}

func buildFilter(c *fiber.Ctx) (checkin.Filter, error) {
	locationName := c.Params("location", "")
	if locationName == "" {
		return checkin.Filter{}, errors.New("location is required")
	}

	locationName, err := url.QueryUnescape(locationName)
	if err != nil {
		return checkin.Filter{}, errors.New("cannot parse location name")
	}

	filter := checkin.Filter{
		LocationName:     locationName,
		PlanningCenterID: c.Query("planning_center_id"),
		FirstName:        c.Query("first_name"),
		LastName:         c.Query("last_name"),
	}

	if idStr := c.Query("id"); idStr != "" {
		filter.ID, err = strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return checkin.Filter{}, errors.New("cannot parse id")
		}
	}

	if cobStr := c.Query("checked_out_before"); cobStr != "" {
		// try time.ParseDuration
		ago, err := time.ParseDuration(cobStr)
		if err == nil {
			filter.CheckedOutAtBefore = time.Now().Add(ago)
		} else {
			filter.CheckedOutAtBefore, err = time.ParseInLocation(time.RFC3339, cobStr, time.UTC)
			if err != nil {
				return checkin.Filter{}, errors.New("cannot parse checked_out_before")
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
				return checkin.Filter{}, errors.New("cannot parse checked_out_after")
			}
		}
	}

	if !filter.CheckedOutAtAfter.IsZero() && !filter.CheckedOutAtBefore.IsZero() && filter.CheckedOutAtAfter.After(filter.CheckedOutAtBefore) {
		return checkin.Filter{}, errors.New("checked_out_after must be before checked_out_before")
	}

	return filter, nil
}

func repoCheckinToOutput(checkin checkin.Checkin) Checkin {
	var coa *time.Time
	if !checkin.CheckedOutAt.IsZero() {
		coa = &checkin.CheckedOutAt
	}
	return Checkin{
		PlanningCenterID: checkin.PlanningCenterID,
		LocationID:       checkin.LocationID,
		FirstName:        checkin.FirstName,
		LastName:         checkin.LastName,
		SecurityCode:     checkin.SecurityCode,
		CheckedOutAt:     coa,
	}
}

func repoCheckinSliceToOutput(checkins []checkin.Checkin) []Checkin {
	output := make([]Checkin, len(checkins))
	for i := range checkins {
		output[i] = repoCheckinToOutput(checkins[i])
	}
	return output
}

func sortCheckins(checkins []checkin.Checkin) []checkin.Checkin {
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

type Checkin struct {
	PlanningCenterID string     `json:"planning_center_id"`
	LocationID       int64      `json:"location_id"`
	FirstName        string     `json:"first_name"`
	LastName         string     `json:"last_name"`
	SecurityCode     string     `json:"security_code"`
	CheckedOutAt     *time.Time `json:"checked_out_at"`
}

type CheckinFilter struct {
	Location         string    `json:"location"`
	CheckedOutBefore time.Time `json:"checked_out_before"`
	CheckedOutAfter  time.Time `json:"checked_out_after"`
}
