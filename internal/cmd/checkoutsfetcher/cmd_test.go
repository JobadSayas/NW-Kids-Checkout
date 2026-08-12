package checkoutsfetcher

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"kids-checkin/internal/client/planningcenter"
	"kids-checkin/internal/repo/checkin"
	"kids-checkin/internal/repo/event"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type concurrentEventRepo struct {
	events      []event.Event
	onUpdate    func()
	mu          sync.Mutex
	updateCount atomic.Int64
}

func (m *concurrentEventRepo) ListEvents(ctx context.Context, filter event.EventFilter) ([]event.Event, error) {
	return m.events, nil
}

func (m *concurrentEventRepo) GetEventByID(ctx context.Context, id int64) (event.Event, error) {
	for _, e := range m.events {
		if e.ID == id {
			return e, nil
		}
	}
	return event.Event{}, nil
}

func (m *concurrentEventRepo) GetEventByPlanningCenterID(ctx context.Context, planningCenterID string) (event.Event, error) {
	for _, e := range m.events {
		if e.PlanningCenterID == planningCenterID {
			return e, nil
		}
	}
	return event.Event{}, nil
}

func (m *concurrentEventRepo) CreateEvent(ctx context.Context, ev event.Event) (event.Event, error) {
	ev.ID = int64(len(m.events) + 1)
	m.events = append(m.events, ev)
	return ev, nil
}

func (m *concurrentEventRepo) UpdateEvent(ctx context.Context, ev event.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateCount.Add(1)
	if m.onUpdate != nil {
		m.onUpdate()
	}
	for i, e := range m.events {
		if e.ID == ev.ID {
			m.events[i] = ev
			return nil
		}
	}
	return nil
}

func Test_eventCheckoutLoop_noEvents(t *testing.T) {
	var events []event.Event
	eventRepo := &event.MockRepo{
		ListEventsFunc: func(ctx context.Context, filter event.EventFilter) ([]event.Event, error) {
			return events, nil
		},
	}

	checkinRepo := &checkin.MockRepo{}
	pcClient := &planningcenter.MockClient{}
	locationIDMap := sync.Map{}

	err := eventCheckoutLoop(t.Context(), nil, eventRepo, checkinRepo, pcClient, &locationIDMap, 3*time.Second)
	require.NoError(t, err)
}

func Test_eventCheckoutLoop_noEventsNeedingUpdate(t *testing.T) {
	now := time.Now()
	events := []event.Event{
		{ID: 1, PlanningCenterID: "evt_1", AutoFetch: true, LastCheckedOutTime: now},
		{ID: 2, PlanningCenterID: "evt_2", AutoFetch: true, LastCheckedOutTime: now.Add(1 * time.Second)},
	}

	eventRepo := &event.MockRepo{
		ListEventsFunc: func(ctx context.Context, filter event.EventFilter) ([]event.Event, error) {
			return events, nil
		},
	}

	checkinRepo := &checkin.MockRepo{}
	pcClient := &planningcenter.MockClient{}
	locationIDMap := sync.Map{}

	err := eventCheckoutLoop(t.Context(), nil, eventRepo, checkinRepo, pcClient, &locationIDMap, 1*time.Hour)
	require.NoError(t, err)
}

func Test_eventCheckoutLoop_updatesEventsNeedingUpdate(t *testing.T) {
	events := []event.Event{
		{ID: 1, PlanningCenterID: "evt_1", AutoFetch: true, LastCheckedOutTime: time.Time{}},
		{ID: 2, PlanningCenterID: "evt_2", AutoFetch: true, LastCheckedOutTime: time.Now().Add(-10 * time.Minute)},
	}

	eventRepo := &event.MockRepo{
		ListEventsFunc: func(ctx context.Context, filter event.EventFilter) ([]event.Event, error) {
			return events, nil
		},
		GetEventByIDFunc: func(ctx context.Context, id int64) (event.Event, error) {
			for _, e := range events {
				if e.ID == id {
					return e, nil
				}
			}
			return event.Event{}, nil
		},
		UpdateEventFunc: func(ctx context.Context, ev event.Event) error {
			for i, e := range events {
				if e.ID == ev.ID {
					events[i] = ev
					return nil
				}
			}
			return nil
		},
	}

	var createdCheckins []checkin.Checkin
	checkinRepo := &checkin.MockRepo{
		CreateCheckinFunc: func(ctx context.Context, c checkin.Checkin) (checkin.Checkin, error) {
			c.ID = int64(len(createdCheckins) + 1)
			createdCheckins = append(createdCheckins, c)
			return c, nil
		},
	}

	checkouts := map[string][]planningcenter.Checkout{
		"evt_1": {
			{ID: "pc_checkout_1", FirstName: "John", LastName: "Doe", SecurityCode: "ABCD", PlanningCenterLocationID: "pc_loc_1"},
		},
		"evt_2": {
			{ID: "pc_checkout_2", FirstName: "Jane", LastName: "Smith", SecurityCode: "EFGH", PlanningCenterLocationID: "pc_loc_1"},
		},
	}

	pcClient := &planningcenter.MockClient{
		GetCheckoutsForEventFunc: func(ctx context.Context, eventID string, checkedOutOnOrAfter time.Time, limit int) ([]planningcenter.Checkout, error) {
			return checkouts[eventID], nil
		},
	}

	locationIDMap := sync.Map{}
	locationIDMap.Store("pc_loc_1", int64(1))

	err := eventCheckoutLoop(t.Context(), nil, eventRepo, checkinRepo, pcClient, &locationIDMap, 5*time.Minute)
	require.NoError(t, err)

	require.Len(t, createdCheckins, 2, "this test requires checkinRepo to return checkins")

	checkinIDs := map[string]bool{}
	for _, c := range createdCheckins {
		checkinIDs[c.PlanningCenterID] = true
	}
	assert.True(t, checkinIDs["pc_checkout_1"])
	assert.True(t, checkinIDs["pc_checkout_2"])
	assert.Equal(t, int64(1), createdCheckins[0].LocationID)

	assert.True(t, events[0].LastCheckedOutTime.After(time.Now().Add(-1*time.Minute)))
	assert.True(t, events[1].LastCheckedOutTime.After(time.Now().Add(-1*time.Minute)))
}

func Test_processEventCheckouts_createsCheckinsAndUpdatesEvent(t *testing.T) {
	events := []event.Event{
		{ID: 1, PlanningCenterID: "evt_1", AutoFetch: true, LastCheckedOutTime: time.Time{}},
	}

	eventRepo := &event.MockRepo{
		GetEventByIDFunc: func(ctx context.Context, id int64) (event.Event, error) {
			return events[0], nil
		},
		UpdateEventFunc: func(ctx context.Context, ev event.Event) error {
			events[0] = ev
			return nil
		},
	}

	var createdCheckins []checkin.Checkin
	checkinRepo := &checkin.MockRepo{
		CreateCheckinFunc: func(ctx context.Context, c checkin.Checkin) (checkin.Checkin, error) {
			c.ID = int64(len(createdCheckins) + 1)
			createdCheckins = append(createdCheckins, c)
			return c, nil
		},
	}

	pcClient := &planningcenter.MockClient{
		GetCheckoutsForEventFunc: func(ctx context.Context, eventID string, checkedOutOnOrAfter time.Time, limit int) ([]planningcenter.Checkout, error) {
			return []planningcenter.Checkout{
				{ID: "pc_checkout_1", FirstName: "John", LastName: "Doe", SecurityCode: "ABCD", PlanningCenterLocationID: "pc_loc_1", CheckedOutAt: time.Now().Add(-1 * time.Hour)},
				{ID: "pc_checkout_2", FirstName: "Jane", LastName: "Smith", SecurityCode: "EFGH", PlanningCenterLocationID: "pc_loc_1", CheckedOutAt: time.Now().Add(-30 * time.Minute)},
			}, nil
		},
	}

	locationIDMap := sync.Map{}
	locationIDMap.Store("pc_loc_1", int64(5))

	errCh := make(chan error, 10)
	var wg sync.WaitGroup
	wg.Go(func() {
		processEventCheckouts(t.Context(), events[0], checkinRepo, eventRepo, pcClient, &locationIDMap, &sync.Map{}, errCh)
	})
	wg.Wait()
	close(errCh)

	for err := range errCh {
		require.NoError(t, err)
	}

	assert.Len(t, createdCheckins, 2)
	assert.Equal(t, int64(5), createdCheckins[0].LocationID)
	assert.Equal(t, int64(5), createdCheckins[1].LocationID)
	assert.Equal(t, int64(1), createdCheckins[0].EventID, "checkin should have EventID set to the source event")
	assert.Equal(t, int64(1), createdCheckins[1].EventID, "checkin should have EventID set to the source event")
	assert.Equal(t, "John", createdCheckins[0].FirstName)
	assert.Equal(t, "Jane", createdCheckins[1].FirstName)

	assert.True(t, events[0].LastCheckedOutTime.After(time.Now().Add(-1*time.Minute)))
}

func Test_processEventCheckouts_handlesFetchError(t *testing.T) {
	eventRepo := &event.MockRepo{
		GetEventByIDFunc: func(ctx context.Context, id int64) (event.Event, error) {
			return event.Event{ID: 1, PlanningCenterID: "evt_1", AutoFetch: true, LastCheckedOutTime: time.Time{}}, nil
		},
	}

	checkinRepo := &checkin.MockRepo{}
	pcClient := &planningcenter.MockClient{
		GetCheckoutsForEventFunc: func(ctx context.Context, eventID string, checkedOutOnOrAfter time.Time, limit int) ([]planningcenter.Checkout, error) {
			return nil, assert.AnError
		},
	}
	locationIDMap := sync.Map{}
	locationIDMap.Store("pc_loc_1", int64(1))

	errCh := make(chan error, 10)
	var wg sync.WaitGroup
	wg.Go(func() {
		processEventCheckouts(t.Context(), event.Event{ID: 1, PlanningCenterID: "evt_1"}, checkinRepo, eventRepo, pcClient, &locationIDMap, &sync.Map{}, errCh)
	})
	wg.Wait()
	close(errCh)

	errs := []error{}
	for e := range errCh {
		errs = append(errs, e)
	}

	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "failed to fetch checkouts for event")
}

func Test_eventCheckoutLoop_contextCancellation(t *testing.T) {
	events := []event.Event{
		{ID: 1, PlanningCenterID: "evt_1", AutoFetch: true, LastCheckedOutTime: time.Time{}},
		{ID: 2, PlanningCenterID: "evt_2", AutoFetch: true, LastCheckedOutTime: time.Time{}},
		{ID: 3, PlanningCenterID: "evt_3", AutoFetch: true, LastCheckedOutTime: time.Time{}},
	}

	eventRepo := &event.MockRepo{
		ListEventsFunc: func(ctx context.Context, filter event.EventFilter) ([]event.Event, error) {
			return events, nil
		},
	}

	checkinRepo := &checkin.MockRepo{}
	pcClient := &planningcenter.MockClient{}
	locationIDMap := sync.Map{}
	locationIDMap.Store("pc_loc_1", int64(1))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := eventCheckoutLoop(ctx, nil, eventRepo, checkinRepo, pcClient, &locationIDMap, 5*time.Minute)
	assert.ErrorIs(t, err, context.Canceled)
}

func Test_processEventCheckouts_unknownLocation(t *testing.T) {
	events := []event.Event{
		{ID: 1, PlanningCenterID: "evt_1", AutoFetch: true, LastCheckedOutTime: time.Time{}},
	}

	eventRepo := &event.MockRepo{
		GetEventByIDFunc: func(ctx context.Context, id int64) (event.Event, error) {
			return events[0], nil
		},
		UpdateEventFunc: func(ctx context.Context, ev event.Event) error {
			events[0] = ev
			return nil
		},
	}

	var createdCheckins []checkin.Checkin
	checkinRepo := &checkin.MockRepo{
		CreateCheckinFunc: func(ctx context.Context, c checkin.Checkin) (checkin.Checkin, error) {
			c.ID = int64(len(createdCheckins) + 1)
			createdCheckins = append(createdCheckins, c)
			return c, nil
		},
	}

	pcClient := &planningcenter.MockClient{
		GetCheckoutsForEventFunc: func(ctx context.Context, eventID string, checkedOutOnOrAfter time.Time, limit int) ([]planningcenter.Checkout, error) {
			return []planningcenter.Checkout{
				{ID: "pc_checkout_unknown", FirstName: "Unknown", LastName: "Person", SecurityCode: "UNKN", PlanningCenterLocationID: "unknown_loc"},
				{ID: "pc_checkout_known", FirstName: "Known", LastName: "Person", SecurityCode: "KNOW", PlanningCenterLocationID: "pc_loc_1"},
			}, nil
		},
	}

	locationIDMap := sync.Map{}
	locationIDMap.Store("pc_loc_1", int64(5))

	errCh := make(chan error, 10)
	var wg sync.WaitGroup
	wg.Go(func() {
		processEventCheckouts(t.Context(), events[0], checkinRepo, eventRepo, pcClient, &locationIDMap, &sync.Map{}, errCh)
	})
	wg.Wait()
	close(errCh)

	for err := range errCh {
		require.NoError(t, err)
	}

	assert.Len(t, createdCheckins, 1, "should only create checkin for known location")
	assert.Equal(t, "Known", createdCheckins[0].FirstName)
}

func Test_eventCheckoutLoop_concurrentEvents_sequentialProcessing(t *testing.T) {
	events := []event.Event{
		{ID: 1, PlanningCenterID: "evt_1", AutoFetch: true, LastCheckedOutTime: time.Time{}},
		{ID: 2, PlanningCenterID: "evt_2", AutoFetch: true, LastCheckedOutTime: time.Time{}},
		{ID: 3, PlanningCenterID: "evt_3", AutoFetch: true, LastCheckedOutTime: time.Time{}},
	}

	eventRepo := &event.MockRepo{
		ListEventsFunc: func(ctx context.Context, filter event.EventFilter) ([]event.Event, error) {
			return events, nil
		},
		GetEventByIDFunc: func(ctx context.Context, id int64) (event.Event, error) {
			for _, e := range events {
				if e.ID == id {
					return e, nil
				}
			}
			return event.Event{}, nil
		},
		UpdateEventFunc: func(ctx context.Context, ev event.Event) error {
			for i, e := range events {
				if e.ID == ev.ID {
					events[i] = ev
					return nil
				}
			}
			return nil
		},
	}

	var createdCheckins []checkin.Checkin
	checkinRepo := &checkin.MockRepo{
		CreateCheckinFunc: func(ctx context.Context, c checkin.Checkin) (checkin.Checkin, error) {
			c.ID = int64(len(createdCheckins) + 1)
			createdCheckins = append(createdCheckins, c)
			return c, nil
		},
	}

	pcClient := &planningcenter.MockClient{
		GetCheckoutsForEventFunc: func(ctx context.Context, eventID string, checkedOutOnOrAfter time.Time, limit int) ([]planningcenter.Checkout, error) {
			switch eventID {
			case "evt_1":
				return []planningcenter.Checkout{{ID: "c1", FirstName: "A", LastName: "A", SecurityCode: "AAA", PlanningCenterLocationID: "pc_loc_1"}}, nil
			case "evt_2":
				return []planningcenter.Checkout{{ID: "c2", FirstName: "B", LastName: "B", SecurityCode: "BBB", PlanningCenterLocationID: "pc_loc_1"}}, nil
			case "evt_3":
				return []planningcenter.Checkout{{ID: "c3", FirstName: "C", LastName: "C", SecurityCode: "CCC", PlanningCenterLocationID: "pc_loc_1"}}, nil
			}
			return nil, nil
		},
	}

	locationIDMap := sync.Map{}
	locationIDMap.Store("pc_loc_1", int64(1))

	err := eventCheckoutLoop(t.Context(), nil, eventRepo, checkinRepo, pcClient, &locationIDMap, 5*time.Minute)
	require.NoError(t, err)

	assert.Len(t, createdCheckins, 3)
	for _, ev := range events {
		assert.False(t, ev.LastCheckedOutTime.IsZero(), "event %d should have LastCheckedOutTime set", ev.ID)
	}
}

func Test_processEventCheckouts_updateEventFailureDoesNotCorruptState(t *testing.T) {
	originalTime := time.Now().Add(-1 * time.Hour).UTC()
	ev := event.Event{ID: 1, PlanningCenterID: "evt_1", AutoFetch: true, LastCheckedOutTime: originalTime}

	eventRepo := &event.MockRepo{
		GetEventByIDFunc: func(ctx context.Context, id int64) (event.Event, error) {
			return ev, nil
		},
		UpdateEventFunc: func(ctx context.Context, event event.Event) error {
			return assert.AnError
		},
	}

	var createdCheckins []checkin.Checkin
	checkinRepo := &checkin.MockRepo{
		CreateCheckinFunc: func(ctx context.Context, c checkin.Checkin) (checkin.Checkin, error) {
			c.ID = int64(len(createdCheckins) + 1)
			createdCheckins = append(createdCheckins, c)
			return c, nil
		},
	}

	pcClient := &planningcenter.MockClient{
		GetCheckoutsForEventFunc: func(ctx context.Context, eventID string, checkedOutOnOrAfter time.Time, limit int) ([]planningcenter.Checkout, error) {
			return []planningcenter.Checkout{
				{ID: "pc_checkout_1", FirstName: "John", LastName: "Doe", SecurityCode: "ABCD", PlanningCenterLocationID: "pc_loc_1", CheckedOutAt: time.Now().Add(-30 * time.Minute)},
			}, nil
		},
	}

	locationIDMap := sync.Map{}
	locationIDMap.Store("pc_loc_1", int64(5))

	errCh := make(chan error, 10)
	var wg sync.WaitGroup
	wg.Go(func() {
		processEventCheckouts(t.Context(), ev, checkinRepo, eventRepo, pcClient, &locationIDMap, &sync.Map{}, errCh)
	})
	wg.Wait()
	close(errCh)

	errs := []error{}
	for e := range errCh {
		errs = append(errs, e)
	}

	require.Len(t, errs, 1, "should return error when UpdateEvent fails")
	assert.Contains(t, errs[0].Error(), "failed to update event")

	assert.Equal(t, originalTime, ev.LastCheckedOutTime, "LastCheckedOutTime should not be updated when UpdateEvent fails")

	assert.Len(t, createdCheckins, 1, "checkin should still be created before UpdateEvent failure")
}

func Test_processEventCheckouts_concurrentSameEvent_mutexProtection(t *testing.T) {
	var maxConcurrent atomic.Int64
	var currentConcurrent atomic.Int64

	events := []event.Event{
		{ID: 1, PlanningCenterID: "evt_1", AutoFetch: true, LastCheckedOutTime: time.Time{}},
	}

	eventRepo := &event.MockRepo{
		GetEventByIDFunc: func(ctx context.Context, id int64) (event.Event, error) {
			return events[0], nil
		},
		UpdateEventFunc: func(ctx context.Context, ev event.Event) error {
			current := currentConcurrent.Add(1)
			prev := maxConcurrent.Load()
			for current > prev {
				if maxConcurrent.CompareAndSwap(prev, current) {
					break
				}
				prev = maxConcurrent.Load()
			}
			time.Sleep(10 * time.Millisecond)
			currentConcurrent.Add(-1)
			events[0] = ev
			return nil
		},
	}

	var createdCheckins []checkin.Checkin
	checkinRepo := &checkin.MockRepo{
		CreateCheckinFunc: func(ctx context.Context, c checkin.Checkin) (checkin.Checkin, error) {
			c.ID = int64(len(createdCheckins) + 1)
			createdCheckins = append(createdCheckins, c)
			return c, nil
		},
	}

	pcClient := &planningcenter.MockClient{
		GetCheckoutsForEventFunc: func(ctx context.Context, eventID string, checkedOutOnOrAfter time.Time, limit int) ([]planningcenter.Checkout, error) {
			return []planningcenter.Checkout{
				{ID: "c1", FirstName: "A", LastName: "A", SecurityCode: "AAA", PlanningCenterLocationID: "pc_loc_1"},
				{ID: "c2", FirstName: "B", LastName: "B", SecurityCode: "BBB", PlanningCenterLocationID: "pc_loc_1"},
			}, nil
		},
	}

	locationIDMap := sync.Map{}
	locationIDMap.Store("pc_loc_1", int64(1))

	const goroutines = 10
	errCh := make(chan error, goroutines)
	eventMutexes := sync.Map{}

	var wg sync.WaitGroup

	for range goroutines {
		wg.Go(func() {
			processEventCheckouts(t.Context(), events[0], checkinRepo, eventRepo, pcClient, &locationIDMap, &eventMutexes, errCh)
		})
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		require.NoError(t, err)
	}

	assert.Equal(t, int64(1), maxConcurrent.Load(), "at most 1 UpdateEvent should run concurrently due to mutex protection")
	assert.False(t, events[0].LastCheckedOutTime.IsZero(), "LastCheckedOutTime should be set after processing")
	assert.Len(t, createdCheckins, 2*goroutines, "each goroutine creates 2 checkins")
}

func Test_eventCheckoutLoop_contextCancellationMidLoop_breaksEarly(t *testing.T) {
	events := []event.Event{
		{ID: 1, PlanningCenterID: "evt_1", AutoFetch: true, LastCheckedOutTime: time.Time{}},
		{ID: 2, PlanningCenterID: "evt_2", AutoFetch: true, LastCheckedOutTime: time.Time{}},
		{ID: 3, PlanningCenterID: "evt_3", AutoFetch: true, LastCheckedOutTime: time.Time{}},
		{ID: 4, PlanningCenterID: "evt_4", AutoFetch: true, LastCheckedOutTime: time.Time{}},
		{ID: 5, PlanningCenterID: "evt_5", AutoFetch: true, LastCheckedOutTime: time.Time{}},
		{ID: 6, PlanningCenterID: "evt_6", AutoFetch: true, LastCheckedOutTime: time.Time{}},
	}

	ctx, cancel := context.WithCancel(t.Context())

	releaseCh := make(chan struct{})
	var blockedWG sync.WaitGroup
	blockedWG.Add(5)

	eventRepo := &event.MockRepo{
		ListEventsFunc: func(ctx context.Context, filter event.EventFilter) ([]event.Event, error) {
			return events, nil
		},
		GetEventByIDFunc: func(ctx context.Context, id int64) (event.Event, error) {
			for _, e := range events {
				if e.ID == id {
					return e, nil
				}
			}
			return event.Event{}, nil
		},
		UpdateEventFunc: func(ctx context.Context, ev event.Event) error {
			return nil
		},
	}

	checkinRepo := &checkin.MockRepo{
		CreateCheckinFunc: func(ctx context.Context, c checkin.Checkin) (checkin.Checkin, error) {
			return c, nil
		},
	}

	pcClient := &planningcenter.MockClient{
		GetCheckoutsForEventFunc: func(ctx context.Context, eventID string, checkedOutOnOrAfter time.Time, limit int) ([]planningcenter.Checkout, error) {
			blockedWG.Done()
			<-releaseCh
			return []planningcenter.Checkout{
				{ID: "c_" + eventID, FirstName: "F", LastName: "L", SecurityCode: "CODE", PlanningCenterLocationID: "pc_loc_1"},
			}, nil
		},
	}

	locationIDMap := sync.Map{}
	locationIDMap.Store("pc_loc_1", int64(1))

	errCh := make(chan error, 1)
	go func() {
		errCh <- eventCheckoutLoop(ctx, nil, eventRepo, checkinRepo, pcClient, &locationIDMap, 5*time.Minute)
	}()

	blockedWG.Wait()
	// All 5 semaphore slots are held; the 6th event is blocked on send.

	cancel()

	close(releaseCh)

	err := <-errCh
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func Test_eventCheckoutLoop_stopChClosed_noDispatch(t *testing.T) {
	events := []event.Event{
		{ID: 1, PlanningCenterID: "evt_1", AutoFetch: true, LastCheckedOutTime: time.Time{}},
	}

	eventRepo := &event.MockRepo{
		ListEventsFunc: func(ctx context.Context, filter event.EventFilter) ([]event.Event, error) {
			return events, nil
		},
	}

	var createdCheckins atomic.Int64
	checkinRepo := &checkin.MockRepo{
		CreateCheckinFunc: func(ctx context.Context, c checkin.Checkin) (checkin.Checkin, error) {
			createdCheckins.Add(1)
			return c, nil
		},
	}

	pcClient := &planningcenter.MockClient{
		GetCheckoutsForEventFunc: func(ctx context.Context, eventID string, checkedOutOnOrAfter time.Time, limit int) ([]planningcenter.Checkout, error) {
			return []planningcenter.Checkout{
				{ID: "pc_checkout_1", FirstName: "John", LastName: "Doe", SecurityCode: "ABCD", PlanningCenterLocationID: "pc_loc_1"},
			}, nil
		},
	}

	locationIDMap := sync.Map{}
	locationIDMap.Store("pc_loc_1", int64(1))

	stopCh := make(chan struct{})
	close(stopCh)

	err := eventCheckoutLoop(t.Context(), stopCh, eventRepo, checkinRepo, pcClient, &locationIDMap, 5*time.Minute)
	require.NoError(t, err)
	assert.Zero(t, createdCheckins.Load(), "no checkouts should be fetched when stopCh is already closed")
}

func Test_eventCheckoutLoop_stopChMidLoop_drainsInFlight(t *testing.T) {
	events := []event.Event{
		{ID: 1, PlanningCenterID: "evt_1", AutoFetch: true, LastCheckedOutTime: time.Time{}},
		{ID: 2, PlanningCenterID: "evt_2", AutoFetch: true, LastCheckedOutTime: time.Time{}},
		{ID: 3, PlanningCenterID: "evt_3", AutoFetch: true, LastCheckedOutTime: time.Time{}},
		{ID: 4, PlanningCenterID: "evt_4", AutoFetch: true, LastCheckedOutTime: time.Time{}},
		{ID: 5, PlanningCenterID: "evt_5", AutoFetch: true, LastCheckedOutTime: time.Time{}},
		{ID: 6, PlanningCenterID: "evt_6", AutoFetch: true, LastCheckedOutTime: time.Time{}},
	}

	releaseCh := make(chan struct{})
	var blockedWG sync.WaitGroup
	blockedWG.Add(5)

	eventRepo := &event.MockRepo{
		ListEventsFunc: func(ctx context.Context, filter event.EventFilter) ([]event.Event, error) {
			return events, nil
		},
		GetEventByIDFunc: func(ctx context.Context, id int64) (event.Event, error) {
			for _, e := range events {
				if e.ID == id {
					return e, nil
				}
			}
			return event.Event{}, nil
		},
		UpdateEventFunc: func(ctx context.Context, ev event.Event) error {
			return nil
		},
	}

	var createdCheckins atomic.Int64
	checkinRepo := &checkin.MockRepo{
		CreateCheckinFunc: func(ctx context.Context, c checkin.Checkin) (checkin.Checkin, error) {
			return c, nil
		},
	}

	pcClient := &planningcenter.MockClient{
		GetCheckoutsForEventFunc: func(ctx context.Context, eventID string, checkedOutOnOrAfter time.Time, limit int) ([]planningcenter.Checkout, error) {
			blockedWG.Done()
			<-releaseCh
			createdCheckins.Add(1)
			return []planningcenter.Checkout{
				{ID: "c_" + eventID, FirstName: "F", LastName: "L", SecurityCode: "CODE", PlanningCenterLocationID: "pc_loc_1"},
			}, nil
		},
	}

	locationIDMap := sync.Map{}
	locationIDMap.Store("pc_loc_1", int64(1))

	stopCh := make(chan struct{})

	errCh := make(chan error, 1)
	go func() {
		errCh <- eventCheckoutLoop(t.Context(), stopCh, eventRepo, checkinRepo, pcClient, &locationIDMap, 5*time.Minute)
	}()

	blockedWG.Wait()
	// All 5 semaphore slots are held; the 6th event is blocked on send.
	close(stopCh)
	close(releaseCh)

	err := <-errCh
	require.NoError(t, err)
	assert.Equal(t, int64(5), createdCheckins.Load(), "only in-flight events should complete after stopCh closes")
	assert.Equal(t, 5, eventRepo.GetEventByIDFuncCallCount, "the 6th event should not have been dispatched after stopCh closes")
}

func Test_shouldSwallowLoopError(t *testing.T) {
	t.Run("running normally", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		assert.False(t, shouldSwallowLoopError(ctx, make(chan struct{})))
	})

	t.Run("stopCh closed during graceful drain", func(t *testing.T) {
		stopCh := make(chan struct{})
		close(stopCh)
		assert.True(t, shouldSwallowLoopError(t.Context(), stopCh))
	})

	t.Run("context cancelled (runtime expired)", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		assert.True(t, shouldSwallowLoopError(ctx, make(chan struct{})))
	})

	t.Run("both stopCh closed and context cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		stopCh := make(chan struct{})
		close(stopCh)
		assert.True(t, shouldSwallowLoopError(ctx, stopCh))
	})
}
