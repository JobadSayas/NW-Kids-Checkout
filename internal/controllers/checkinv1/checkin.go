package checkinv1

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"kids-checkin/internal/web/static"
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

const defaultCheckedOutAfterDelta = -150 * time.Hour

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

	checkinGroup.Get("/checkouts", controller.Checkouts)
}

func (controller *Controller) Checkouts(c *fiber.Ctx) error {
	if websocket.IsWebSocketUpgrade(c) {
		return controller.checkoutsWebsocket(c)
	}

	return controller.checkoutsWeb(c)
}

func (controller *Controller) checkoutsWeb(c *fiber.Ctx) error {
	accepts := c.Accepts(fiber.MIMEApplicationJSON, fiber.MIMETextHTML)
	if accepts == "" {
		return fiber.NewError(fiber.StatusUnsupportedMediaType, "unsupported media type")
	}

	if accepts == fiber.MIMETextHTML {
		f, err := static.EmbeddedFS.Open("pages/checkoutsv1/checkouts.html")
		if err != nil {
			return fiber.ErrInternalServerError
		}
		defer f.Close()

		c.Type("html")
		return c.SendStream(f)
	}

	filter, err := buildFilter(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	if filter.CheckedOutAtAfter.IsZero() {
		filter.CheckedOutAtAfter = time.Now().Add(defaultCheckedOutAfterDelta)
	}

	fmt.Printf("%+v\n", filter)

	checkins, err := controller.checkinRepo.ListCheckins(c.Context(), filter)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	checkins = sortCheckins(checkins)

	_, err = controller.locationRepo.ListLocations(c.Context(), location.LocationFilter{})
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(repoCheckinSliceToOutput(checkins))
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
	locationGroupName := c.Query("location_group_name", "")
	var err error

	if locationGroupName != "" {
		locationGroupName, err = url.QueryUnescape(locationGroupName)
		if err != nil {
			return checkin.Filter{}, errors.New("cannot parse location_group_id")
		}
	}

	filter := checkin.Filter{
		LocationGroupName: locationGroupName,
		PlanningCenterID:  c.Query("planning_center_id"),
		FirstName:         c.Query("first_name"),
		LastName:          c.Query("last_name"),
	}

	if idStr := c.Query("id"); idStr != "" {
		filter.ID, err = strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return checkin.Filter{}, errors.New("cannot parse id")
		}
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		limitInt, err := strconv.Atoi(limitStr)
		if err != nil {
			return checkin.Filter{}, errors.New("cannot parse limit")
		}
		if limitInt < 0 {
			return checkin.Filter{}, errors.New("limit must be positive")
		}
		filter.Limit = limitInt
	}

	if lgIDStr := c.Query("location_group_id"); lgIDStr != "" {
		lgID, err := strconv.ParseInt(lgIDStr, 10, 64)
		if err != nil {
			return checkin.Filter{}, errors.New("cannot parse location_group_id")
		}
		if lgID < 0 {
			return checkin.Filter{}, errors.New("location_group_id must be positive")
		}
		filter.LocationGroupID = lgID
	}

	if lgName := c.Query("location_group_name"); lgName != "" {
		filter.LocationGroupName = lgName
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
