package metricsv1

import (
	"fmt"
	"math"
	"strconv"

	"kids-checkin/internal/controllers/middleware"
	"kids-checkin/internal/controllers/session"
	"kids-checkin/internal/repo/metrics"

	"github.com/gofiber/fiber/v2"
)

type Controller struct {
	repo         metrics.Repo
	sessionStore session.Storer
}

func NewController(repo metrics.Repo, sessionStore session.Storer) *Controller {
	return &Controller{repo: repo, sessionStore: sessionStore}
}

func (controller *Controller) RegisterRoutes(app fiber.Router) {
	group := app.Group("/v1/admin/metrics")
	group.Use(middleware.AuthRequired(controller.sessionStore, "admin"))
	group.Get("", controller.GetMetrics)
}

type DailyMetricResponse struct {
	Date              string  `json:"date"`
	EventName         string  `json:"event_name"`
	Called            int     `json:"called"`
	Confirmed         int     `json:"confirmed"`
	Unconfirmed       int     `json:"unconfirmed"`
	AvgConfirmMinutes float64 `json:"avg_confirm_minutes"`
	ManualCount       int     `json:"manual_count"`
}

type MetricsResponse struct {
	Days  int                   `json:"days"`
	Daily []DailyMetricResponse `json:"daily"`
}

func (controller *Controller) GetMetrics(c *fiber.Ctx) error {
	days := 14
	if raw := c.Query("days"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 90 {
			return fiber.NewError(fiber.StatusBadRequest, "days must be an integer between 1 and 90")
		}
		days = parsed
	}

	daily, err := controller.repo.ListDailyMetrics(c.Context(), metrics.Filter{Days: days})
	if err != nil {
		return fmt.Errorf("listing daily metrics: %w", err)
	}

	response := MetricsResponse{Days: days, Daily: make([]DailyMetricResponse, 0, len(daily))}
	for _, dm := range daily {
		response.Daily = append(response.Daily, DailyMetricResponse{
			Date:              dm.Date,
			EventName:         dm.EventName,
			Called:            dm.Called,
			Confirmed:         dm.Confirmed,
			Unconfirmed:       dm.Unconfirmed,
			AvgConfirmMinutes: math.Round(dm.AvgConfirmMinutes*100) / 100,
			ManualCount:       dm.ManualCount,
		})
	}
	return c.JSON(response)
}
