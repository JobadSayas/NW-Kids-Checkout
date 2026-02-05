package eventv1

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"

	"kids-checkin/internal/db"

	"github.com/Masterminds/squirrel"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestController_GetEventByID(t *testing.T) {
	app, store := setupAuthedApp()

	testDB, cleanup, err := db.PrepareTestDB()
	require.NoError(t, err, "Failed to prepare test DB")
	t.Cleanup(cleanup)

	controller := NewController(testDB, store)
	controller.RegisterRoutes(app)

	builder := squirrel.Insert("events").
		RunWith(testDB).
		Columns("name", "planning_center_id")
	res, err := builder.Values("Family Service", "pc_evt_22").ExecContext(t.Context())
	require.NoError(t, err)
	insertedID, _ := res.LastInsertId()

	t.Run("success", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/v1/events/%d", insertedID), nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)

		var payload Event
		err = json.NewDecoder(resp.Body).Decode(&payload)
		require.NoError(t, err)
		assert.Equal(t, insertedID, payload.ID)
		assert.Equal(t, "Family Service", payload.Name)
		assert.Equal(t, "pc_evt_22", payload.PlanningCenterID)
	})

	t.Run("not found", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/events/9999", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
	})

	t.Run("invalid id", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/events/not-a-number", nil)
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
