package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	metricRequestDuration = "http.server.request.duration"
	metricRequests        = "http.server.requests"
	metricServerErrors    = "http.server.errors"
)

// HTTPMetrics records per-request OTel metrics: a duration histogram, a
// request counter, and a 5xx error counter. Attributes include the route
// pattern (not the raw path), method, and response status.
func HTTPMetrics(meter metric.Meter) (fiber.Handler, error) {
	duration, err := meter.Float64Histogram(
		metricRequestDuration,
		metric.WithUnit("ms"),
		metric.WithDescription("Duration of HTTP server requests"),
	)
	if err != nil {
		return nil, err
	}
	requests, err := meter.Int64Counter(
		metricRequests,
		metric.WithDescription("Total number of HTTP requests"),
	)
	if err != nil {
		return nil, err
	}
	serverErrors, err := meter.Int64Counter(
		metricServerErrors,
		metric.WithDescription("Total number of HTTP responses with status >= 500"),
	)
	if err != nil {
		return nil, err
	}

	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		status := c.Response().StatusCode()
		if ferr, ok := err.(*fiber.Error); ok {
			status = ferr.Code
		}
		route := c.Route().Path

		attrs := metric.WithAttributes(
			attribute.String("http.request.method", c.Method()),
			attribute.String("http.route", route),
			attribute.Int("http.response.status_code", status),
		)
		duration.Record(c.Context(), float64(time.Since(start).Milliseconds()), attrs)
		requests.Add(c.Context(), 1, attrs)
		if status >= fiber.StatusInternalServerError {
			serverErrors.Add(c.Context(), 1, attrs)
		}
		return err
	}, nil
}
