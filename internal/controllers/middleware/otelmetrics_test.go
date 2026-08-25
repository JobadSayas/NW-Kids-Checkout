package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func collectMetric(t *testing.T, reader *sdkmetric.ManualReader, name string) metricdata.Metrics {
	t.Helper()
	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(t.Context(), &rm))
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == name {
				return m
			}
		}
	}
	return metricdata.Metrics{}
}

func Test_HTTPMetrics_records_duration_and_requests(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = mp.Shutdown(t.Context()) })

	mw, err := HTTPMetrics(mp.Meter("test"))
	require.NoError(t, err)

	app := fiber.New()
	app.Use(mw)
	app.Get("/hello/:name", func(c *fiber.Ctx) error {
		return c.SendString("hi")
	})

	req, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/hello/world", nil))
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, req.StatusCode)

	hist := collectMetric(t, reader, "http.server.request.duration")
	require.Equal(t, "ms", hist.Unit)
	data, ok := hist.Data.(metricdata.Histogram[float64])
	require.True(t, ok, "expected histogram data point")

	count := 0
	statusSeen := false
	routeSeen := false
	for _, dp := range data.DataPoints {
		count += int(dp.Count)
		_, hasStatus := dp.Attributes.Value(attribute.Key("http.response.status_code"))
		_, hasRoute := dp.Attributes.Value(attribute.Key("http.route"))
		statusSeen = statusSeen || hasStatus
		routeSeen = routeSeen || hasRoute
	}
	assert.Equal(t, 1, count)
	assert.True(t, statusSeen, "expected status code attr on datapoint")
	assert.True(t, routeSeen, "expected http.route attr on datapoint")

	requests := collectMetric(t, reader, "http.server.requests")
	reqData, ok := requests.Data.(metricdata.Sum[int64])
	require.True(t, ok, "expected sum data for request counter")
	total := int64(0)
	for _, dp := range reqData.DataPoints {
		total += dp.Value
	}
	assert.Equal(t, int64(1), total)
}

func Test_HTTPMetrics_counts_server_errors(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = mp.Shutdown(t.Context()) })

	mw, err := HTTPMetrics(mp.Meter("test"))
	require.NoError(t, err)

	app := fiber.New()
	app.Use(mw)
	app.Get("/boom", func(c *fiber.Ctx) error {
		return fiber.NewError(fiber.StatusInternalServerError, "boom")
	})
	app.Get("/ok", func(c *fiber.Ctx) error {
		return c.SendString("fine")
	})

	_, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/boom", nil))
	require.NoError(t, err)
	_, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/ok", nil))
	require.NoError(t, err)

	errMetric := collectMetric(t, reader, "http.server.errors")
	errSum, ok := errMetric.Data.(metricdata.Sum[int64])
	require.True(t, ok, "expected sum data for error counter")
	total := int64(0)
	for _, dp := range errSum.DataPoints {
		total += dp.Value
	}
	assert.Equal(t, int64(1), total, "only the 5xx response should be counted as an error")
}
