package eventv1

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"

	"kids-checkin/internal/client/planningcenter"
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

func TestController_PostCreateEvent(t *testing.T) {
	app, store := setupAuthedApp()

	testDB, cleanup, err := db.PrepareTestDB()
	require.NoError(t, err, "Failed to prepare test DB")
	t.Cleanup(cleanup)

	controller := NewController(testDB, store)
	mockClient := &planningcenter.MockClient{}
	controller.client = mockClient
	controller.RegisterRoutes(app)

	payload := EventInput{
		PlanningCenterID: "pc_evt_77",
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	mockClient.GetEventByIDFunc = func(ctx context.Context, eventID string) (planningcenter.Event, error) {
		require.Equal(t, payload.PlanningCenterID, eventID)
		return planningcenter.Event{ID: eventID, Name: "Kids Service"}, nil
	}

	mockClient.GetLocationsForEventFunc = func(ctx context.Context, eventID string) ([]planningcenter.Location, error) {
		require.Equal(t, payload.PlanningCenterID, eventID)
		return []planningcenter.Location{
			{ID: "loc-1", Name: "Room A"},
			{ID: "loc-2", Name: "Room B"},
		}, nil
	}

	req := httptest.NewRequest("POST", "/admin/api/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusCreated, resp.StatusCode)

	var created Event
	err = json.NewDecoder(resp.Body).Decode(&created)
	require.NoError(t, err)
	assert.NotZero(t, created.ID)
	assert.Equal(t, "Kids Service", created.Name)
	assert.Equal(t, payload.PlanningCenterID, created.PlanningCenterID)
}

func TestController_PostCreateEvent_MissingPlanningCenterID(t *testing.T) {
	app, store := setupAuthedApp()

	testDB, cleanup, err := db.PrepareTestDB()
	require.NoError(t, err, "Failed to prepare test DB")
	t.Cleanup(cleanup)

	controller := NewController(testDB, store)
	controller.client = &planningcenter.MockClient{}
	controller.RegisterRoutes(app)

	req := httptest.NewRequest("POST", "/admin/api/events", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestController_PostCreateEvent_Conflict(t *testing.T) {
	app, store := setupAuthedApp()

	testDB, cleanup, err := db.PrepareTestDB()
	require.NoError(t, err, "Failed to prepare test DB")
	t.Cleanup(cleanup)

	controller := NewController(testDB, store)
	mockClient := &planningcenter.MockClient{}
	controller.client = mockClient
	controller.RegisterRoutes(app)

	payload := EventInput{
		PlanningCenterID: "pc_evt_77",
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	_, err = squirrel.Insert("events").
		RunWith(testDB).
		Columns("name", "planning_center_id").
		Values("Existing Event", payload.PlanningCenterID).
		ExecContext(t.Context())
	require.NoError(t, err)

	mockClient.GetEventByIDFunc = func(ctx context.Context, eventID string) (planningcenter.Event, error) {
		require.Equal(t, payload.PlanningCenterID, eventID)
		return planningcenter.Event{ID: eventID, Name: "Kids Service"}, nil
	}

	mockClient.GetLocationsForEventFunc = func(ctx context.Context, eventID string) ([]planningcenter.Location, error) {
		require.Equal(t, payload.PlanningCenterID, eventID)
		return []planningcenter.Location{{ID: "loc-1", Name: "Room A"}}, nil
	}

	req := httptest.NewRequest("POST", "/admin/api/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusConflict, resp.StatusCode)
}

func TestController_GetEventByPlanningCenterID(t *testing.T) {
	app, store := setupAuthedApp()

	testDB, cleanup, err := db.PrepareTestDB()
	require.NoError(t, err, "Failed to prepare test DB")
	t.Cleanup(cleanup)

	controller := NewController(testDB, store)
	controller.RegisterRoutes(app)

	res, err := squirrel.Insert("events").
		RunWith(testDB).
		Columns("name", "planning_center_id").
		Values("Family Service", "pc_evt_22").
		ExecContext(t.Context())
	require.NoError(t, err)
	insertedID, _ := res.LastInsertId()
	require.NotZero(t, insertedID)

	req := httptest.NewRequest("GET", "/admin/api/events/lookup?planning_center_id=pc_evt_22", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var payload Event
	err = json.NewDecoder(resp.Body).Decode(&payload)
	require.NoError(t, err)
	assert.Equal(t, insertedID, payload.ID)
	assert.Equal(t, "Family Service", payload.Name)
	assert.Equal(t, "pc_evt_22", payload.PlanningCenterID)

	t.Run("not found", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin/api/events/lookup?planning_center_id=missing", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusNotFound, resp.StatusCode)
	})

	t.Run("missing query", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin/api/events/lookup", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
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
