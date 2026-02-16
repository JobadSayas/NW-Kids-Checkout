package checkinv1

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"kids-checkin/internal/db"
	"kids-checkin/internal/repo/checkin"
	"kids-checkin/internal/repo/manualcheckin"

	"github.com/Masterminds/squirrel"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestController_PatchCheckedOutConfirmed(t *testing.T) {
	app, store := setupAuthedApp()

	testDB, cleanup, err := db.PrepareTestDB()
	require.NoError(t, err, "Failed to prepare test DB")
	t.Cleanup(cleanup)

	controller := NewController(testDB, store)
	controller.RegisterRoutes(app)

	_, err = squirrel.Delete("checkins").RunWith(testDB).ExecContext(t.Context())
	require.NoError(t, err)
	_, err = squirrel.Delete("locations").RunWith(testDB).ExecContext(t.Context())
	require.NoError(t, err)

	res, err := squirrel.Insert("locations").
		RunWith(testDB).
		Columns("name", "planning_center_id", "event_id").
		Values("location1", "plloc_1234", 1).
		ExecContext(t.Context())
	require.NoError(t, err)
	locationID, _ := res.LastInsertId()

	checkinRepo := checkin.NewRepo(testDB)
	created, err := checkinRepo.CreateCheckin(t.Context(), checkin.Checkin{
		PlanningCenterID: "plc_1234",
		LocationID:       locationID,
		FirstName:        "sam",
		LastName:         "alpha",
		SecurityCode:     "ABC123",
		CheckedOutAt:     time.Date(2022, 1, 1, 12, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		req := httptest.NewRequest("PATCH", "/v1/checkins/plc_1234/checked_out_confirmed", bytes.NewBufferString("{\"confirmed\":true}"))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)

		var payload Checkin
		err = json.NewDecoder(resp.Body).Decode(&payload)
		require.NoError(t, err)
		require.NotNil(t, payload.CheckedOutConfirmedAt)
		assert.WithinDuration(t, time.Now().UTC(), *payload.CheckedOutConfirmedAt, 2*time.Second)

		checkins, err := checkinRepo.ListCheckins(t.Context(), checkin.Filter{PlanningCenterID: created.PlanningCenterID})
		require.NoError(t, err)
		require.Len(t, checkins, 1)
		assert.WithinDuration(t, time.Now().UTC(), checkins[0].CheckedOutConfirmedAt, 2*time.Second)
	})

	t.Run("not found", func(t *testing.T) {
		req := httptest.NewRequest("PATCH", "/v1/checkins/missing/checked_out_confirmed", bytes.NewBufferString("{\"confirmed\":true}"))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
	})

	t.Run("missing content type", func(t *testing.T) {
		req := httptest.NewRequest("PATCH", "/v1/checkins/plc_1234/checked_out_confirmed", bytes.NewBufferString("{\"confirmed\":true}"))
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusUnsupportedMediaType, resp.StatusCode)
	})

	t.Run("unsupported content type", func(t *testing.T) {
		req := httptest.NewRequest("PATCH", "/v1/checkins/plc_1234/checked_out_confirmed", bytes.NewBufferString("{\"confirmed\":true}"))
		req.Header.Set("Content-Type", "text/plain")
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusUnsupportedMediaType, resp.StatusCode)
	})

	t.Run("invalid json", func(t *testing.T) {
		req := httptest.NewRequest("PATCH", "/v1/checkins/plc_1234/checked_out_confirmed", bytes.NewBufferString("{bad"))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
	})

	t.Run("missing confirmed field", func(t *testing.T) {
		req := httptest.NewRequest("PATCH", "/v1/checkins/plc_1234/checked_out_confirmed", bytes.NewBufferString("{}"))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
	})
}

func TestController_PatchManualCheckedOutConfirmed(t *testing.T) {
	app, store := setupAuthedApp()

	testDB, cleanup, err := db.PrepareTestDB()
	require.NoError(t, err, "Failed to prepare test DB")
	t.Cleanup(cleanup)

	controller := NewController(testDB, store)
	controller.RegisterRoutes(app)

	_, err = squirrel.Delete("manual_checkins").RunWith(testDB).ExecContext(t.Context())
	require.NoError(t, err)

	manualRepo := manualcheckin.NewRepo(testDB)
	created, err := manualRepo.CreateManualCheckin(t.Context(), manualcheckin.ManualCheckin{
		PublicID:     "manual-public-1",
		FirstName:    "sam",
		LastName:     "alpha",
		CheckedOutAt: time.Date(2022, 1, 1, 12, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		req := httptest.NewRequest("PATCH", "/v1/checkins/manual-checkins/manual-public-1/checked_out_confirmed", bytes.NewBufferString("{\"confirmed\":true}"))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)

		var payload Checkin
		err = json.NewDecoder(resp.Body).Decode(&payload)
		require.NoError(t, err)
		require.NotNil(t, payload.CheckedOutConfirmedAt)
		assert.Equal(t, created.PublicID, payload.PublicID)
		assert.WithinDuration(t, time.Now().UTC(), *payload.CheckedOutConfirmedAt, 2*time.Second)

		manualCheckins, err := manualRepo.ListManualCheckins(t.Context(), manualcheckin.Filter{PublicID: created.PublicID})
		require.NoError(t, err)
		require.Len(t, manualCheckins, 1)
		assert.WithinDuration(t, time.Now().UTC(), manualCheckins[0].CheckedOutConfirmedAt, 2*time.Second)
	})

	t.Run("not found", func(t *testing.T) {
		req := httptest.NewRequest("PATCH", "/v1/checkins/manual-checkins/missing/checked_out_confirmed", bytes.NewBufferString("{\"confirmed\":true}"))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
	})

	t.Run("missing content type", func(t *testing.T) {
		req := httptest.NewRequest("PATCH", "/v1/checkins/manual-checkins/manual-public-1/checked_out_confirmed", bytes.NewBufferString("{\"confirmed\":true}"))
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusUnsupportedMediaType, resp.StatusCode)
	})

	t.Run("unsupported content type", func(t *testing.T) {
		req := httptest.NewRequest("PATCH", "/v1/checkins/manual-checkins/manual-public-1/checked_out_confirmed", bytes.NewBufferString("{\"confirmed\":true}"))
		req.Header.Set("Content-Type", "text/plain")
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusUnsupportedMediaType, resp.StatusCode)
	})

	t.Run("invalid json", func(t *testing.T) {
		req := httptest.NewRequest("PATCH", "/v1/checkins/manual-checkins/manual-public-1/checked_out_confirmed", bytes.NewBufferString("{bad"))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
	})

	t.Run("missing confirmed field", func(t *testing.T) {
		req := httptest.NewRequest("PATCH", "/v1/checkins/manual-checkins/manual-public-1/checked_out_confirmed", bytes.NewBufferString("{}"))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
	})
}

func setupAuthedApp() (*fiber.App, *session.Store) {
	app := fiber.New()
	store := session.New()
	app.Use(func(c *fiber.Ctx) error {
		sess, _ := store.Get(c)
		sess.Set("authenticated", true)
		sess.Set("role", "admin")
		if err := sess.Save(); err != nil {
			return err
		}
		return c.Next()
	})
	return app, store
}
