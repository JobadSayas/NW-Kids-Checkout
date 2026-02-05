package locationv1

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"kids-checkin/internal/db"
	"kids-checkin/internal/repo/location"

	"github.com/Masterminds/squirrel"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestController_ListLocationsIncludesEventID(t *testing.T) {
	app, store := setupAuthedApp()

	testDB, cleanup, err := db.PrepareTestDB()
	require.NoError(t, err, "Failed to prepare test DB")
	t.Cleanup(cleanup)

	controller := NewController(testDB, store)
	controller.RegisterRoutes(app)

	builder := squirrel.Insert("events").
		RunWith(testDB).
		Columns("name", "planning_center_id")
	res, err := builder.Values("Evening Service", "pc_evt_33").ExecContext(t.Context())
	require.NoError(t, err)
	insertedID, _ := res.LastInsertId()

	locRepo := location.NewRepo(testDB)
	_, err = locRepo.CreateLocation(t.Context(), location.Location{
		PlanningCenterID: "pcloc_evt",
		EventID:          insertedID,
		Name:             "Event Room",
		AutoFetch:        false,
	})
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/v1/locations", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var payload []Location
	err = json.NewDecoder(resp.Body).Decode(&payload)
	require.NoError(t, err)
	require.Len(t, payload, 1)
	assert.Equal(t, insertedID, payload[0].EventID)
	assert.Equal(t, "Event Room", payload[0].Name)
	assert.Equal(t, "pcloc_evt", payload[0].PlanningCenterID)
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
