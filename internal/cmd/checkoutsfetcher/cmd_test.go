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
	"kids-checkin/internal/repo/eventcheckwindow"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// noCheckWindows returns a window repo with no configured windows, so all
// auto-fetch events are treated as active unless a test configures windows.
func noCheckWindows() *eventcheckwindow.MockRepo {
	return &eventcheckwindow.MockRepo{
		ListCheckWindowsFunc: func(ctx context.Context, filter eventcheckwindow.Filter) ([]eventcheckwindow.EventCheckWindow, error) {
			return nil, nil
		},
	}
}

// testService builds a Service with the given collaborators so tests can call
// its methods directly.
func testService(checkWindowRepo eventcheckwindow.Repo, eventRepo event.Repo, checkinRepo checkin.Repo, pcClient planningcenter.Client, locationIDMap *sync.Map, eventUpdateInterval time.Duration, useCheckWindows bool) *Service {
	return &Service{
		pcClient:            pcClient,
		eventRepo:           eventRepo,
		checkinRepo:         checkinRepo,
		checkWindowRepo:     checkWindowRepo,
		locationIDMap:       locationIDMap,
		eventUpdateInterval: eventUpdateInterval,
		useCheckWindows:     useCheckWindows,
	}
}

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

	err := testService(noCheckWindows(), eventRepo, checkinRepo, pcClient, &locationIDMap, 3*time.Second, false).eventCheckoutLoop(t.Context(), nil)
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

	err := testService(noCheckWindows(), eventRepo, checkinRepo, pcClient, &locationIDMap, 1*time.Hour, false).eventCheckoutLoop(t.Context(), nil)
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

	err := testService(noCheckWindows(), eventRepo, checkinRepo, pcClient, &locationIDMap, 5*time.Minute, false).eventCheckoutLoop(t.Context(), nil)
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
		svc := testService(noCheckWindows(), eventRepo, checkinRepo, pcClient, &locationIDMap, 0, false)
		svc.processEventCheckouts(t.Context(), events[0], errCh)
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
		svc := testService(noCheckWindows(), eventRepo, checkinRepo, pcClient, &locationIDMap, 0, false)
		svc.processEventCheckouts(t.Context(), event.Event{ID: 1, PlanningCenterID: "evt_1"}, errCh)
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

	err := testService(noCheckWindows(), eventRepo, checkinRepo, pcClient, &locationIDMap, 5*time.Minute, false).eventCheckoutLoop(ctx, nil)
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
		svc := testService(noCheckWindows(), eventRepo, checkinRepo, pcClient, &locationIDMap, 0, false)
		svc.processEventCheckouts(t.Context(), events[0], errCh)
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

	err := testService(noCheckWindows(), eventRepo, checkinRepo, pcClient, &locationIDMap, 5*time.Minute, false).eventCheckoutLoop(t.Context(), nil)
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
		svc := testService(noCheckWindows(), eventRepo, checkinRepo, pcClient, &locationIDMap, 0, false)
		svc.processEventCheckouts(t.Context(), ev, errCh)
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
	svc := testService(noCheckWindows(), eventRepo, checkinRepo, pcClient, &locationIDMap, 0, false)

	var wg sync.WaitGroup

	for range goroutines {
		wg.Go(func() {
			svc.processEventCheckouts(t.Context(), events[0], errCh)
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
		errCh <- testService(noCheckWindows(), eventRepo, checkinRepo, pcClient, &locationIDMap, 5*time.Minute, false).eventCheckoutLoop(ctx, nil)
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

	err := testService(noCheckWindows(), eventRepo, checkinRepo, pcClient, &locationIDMap, 5*time.Minute, false).eventCheckoutLoop(t.Context(), stopCh)
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
		errCh <- testService(noCheckWindows(), eventRepo, checkinRepo, pcClient, &locationIDMap, 5*time.Minute, false).eventCheckoutLoop(t.Context(), stopCh)
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

func TestMinutesSinceWeekStartUTC(t *testing.T) {
	tests := []struct {
		name     string
		mockTime time.Time
		want     int
	}{
		{
			name:     "monday midnight",
			mockTime: time.Date(2026, 3, 23, 0, 0, 0, 0, time.UTC),
			want:     0,
		},
		{
			name:     "monday 09:30",
			mockTime: time.Date(2026, 3, 23, 9, 30, 0, 0, time.UTC),
			want:     570,
		},
		{
			name:     "wednesday 12:00",
			mockTime: time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC),
			want:     3600,
		},
		{
			name:     "saturday midday",
			mockTime: time.Date(2026, 3, 28, 12, 0, 0, 0, time.UTC),
			want:     7920,
		},
		{
			name:     "sunday 23:59",
			mockTime: time.Date(2026, 3, 29, 23, 59, 0, 0, time.UTC),
			want:     10079,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := nowFunc
			nowFunc = func() time.Time { return tt.mockTime }
			defer func() { nowFunc = original }()

			got := minutesSinceWeekStartUTC()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLocalToUTCWeekMinutes(t *testing.T) {
	original := nowFunc
	nowFunc = func() time.Time { return time.Date(2026, 3, 29, 22, 0, 0, 0, time.UTC) }
	defer func() { nowFunc = original }()

	got, err := localToUTCWeekMinutes(1, 9, 0, "America/New_York")
	require.NoError(t, err)
	assert.Equal(t, 780, got, "Monday 09:00 NY should be 780 minutes (13:00 UTC)")
}

func TestLocalToUTCWeekMinutes_SameDay(t *testing.T) {
	original := nowFunc
	nowFunc = func() time.Time { return time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC) }
	defer func() { nowFunc = original }()

	got, err := localToUTCWeekMinutes(3, 14, 30, "America/Los_Angeles")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, got, 0)
	assert.Less(t, got, minutesPerWeek)
}

func TestLocalToUTCWeekMinutes_InvalidTimezone(t *testing.T) {
	original := nowFunc
	defer func() { nowFunc = original }()

	_, err := localToUTCWeekMinutes(1, 9, 0, "Invalid/Timezone")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid timezone")
}

func TestParseTime(t *testing.T) {
	tests := []struct {
		name      string
		timeStr   string
		wantHour  int
		wantMin   int
		wantError bool
	}{
		{"valid 24hr", "14:30", 14, 30, false},
		{"valid with leading zero", "09:05", 9, 5, false},
		{"valid midnight", "00:00", 0, 0, false},
		{"valid end of day", "23:59", 23, 59, false},
		{"invalid format", "14-30", 0, 0, true},
		{"invalid hour", "25:00", 0, 0, true},
		{"invalid minute", "12:60", 0, 0, true},
		{"empty", "", 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hour, minute, err := parseTime(tt.timeStr)
			if tt.wantError {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantHour, hour)
			assert.Equal(t, tt.wantMin, minute)
		})
	}
}

func TestMergeWindows(t *testing.T) {
	tests := []struct {
		name         string
		checkWindows []eventcheckwindow.EventCheckWindow
		wantLen      int
		wantErr      bool
	}{
		{
			name: "single window same day",
			checkWindows: []eventcheckwindow.EventCheckWindow{
				{StartDayOfWeek: 1, StartTime: "09:00", EndDayOfWeek: 1, EndTime: "12:00", Timezone: "America/New_York"},
			},
			wantLen: 1,
		},
		{
			name: "multiple non-overlapping",
			checkWindows: []eventcheckwindow.EventCheckWindow{
				{StartDayOfWeek: 1, StartTime: "09:00", EndDayOfWeek: 1, EndTime: "11:00", Timezone: "America/New_York"},
				{StartDayOfWeek: 1, StartTime: "14:00", EndDayOfWeek: 1, EndTime: "16:00", Timezone: "America/New_York"},
			},
			wantLen: 2,
		},
		{
			name: "overlapping windows merge",
			checkWindows: []eventcheckwindow.EventCheckWindow{
				{StartDayOfWeek: 1, StartTime: "09:00", EndDayOfWeek: 1, EndTime: "12:00", Timezone: "America/New_York"},
				{StartDayOfWeek: 1, StartTime: "11:00", EndDayOfWeek: 1, EndTime: "14:00", Timezone: "America/New_York"},
			},
			wantLen: 1,
		},
		{
			name: "crosses week boundary",
			checkWindows: []eventcheckwindow.EventCheckWindow{
				{StartDayOfWeek: 5, StartTime: "22:00", EndDayOfWeek: 6, EndTime: "02:00", Timezone: "America/New_York"},
			},
			wantLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mergeWindows(tt.checkWindows)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, got, tt.wantLen)
		})
	}
}

func Test_eventCheckoutLoop_windowFiltersEvents(t *testing.T) {
	original := nowFunc
	nowFunc = func() time.Time { return time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC) }
	defer func() { nowFunc = original }()

	events := []event.Event{
		{ID: 1, PlanningCenterID: "evt_active", AutoFetch: true, LastCheckedOutTime: time.Time{}},
		{ID: 2, PlanningCenterID: "evt_inactive", AutoFetch: true, LastCheckedOutTime: time.Time{}},
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
				{ID: "pc_" + eventID, FirstName: "John", LastName: "Doe", SecurityCode: "ABCD", PlanningCenterLocationID: "pc_loc_1"},
			}, nil
		},
	}

	locationIDMap := sync.Map{}
	locationIDMap.Store("pc_loc_1", int64(1))

	checkWindowRepo := &eventcheckwindow.MockRepo{
		ListCheckWindowsFunc: func(ctx context.Context, filter eventcheckwindow.Filter) ([]eventcheckwindow.EventCheckWindow, error) {
			return []eventcheckwindow.EventCheckWindow{
				{EventID: 1, StartDayOfWeek: 3, StartTime: "10:00", EndDayOfWeek: 3, EndTime: "14:00", Timezone: "UTC"},
				{EventID: 2, StartDayOfWeek: 3, StartTime: "08:00", EndDayOfWeek: 3, EndTime: "10:00", Timezone: "UTC"},
			}, nil
		},
	}

	err := testService(checkWindowRepo, eventRepo, checkinRepo, pcClient, &locationIDMap, 5*time.Minute, true).eventCheckoutLoop(t.Context(), nil)
	require.NoError(t, err)

	require.Len(t, createdCheckins, 1, "only the event inside its check window should be fetched")
	assert.Equal(t, "pc_evt_active", createdCheckins[0].PlanningCenterID)
}

func Test_eventCheckoutLoop_noWindows_alwaysActive(t *testing.T) {
	events := []event.Event{
		{ID: 1, PlanningCenterID: "evt_1", AutoFetch: true, LastCheckedOutTime: time.Time{}},
	}

	eventRepo := &event.MockRepo{
		ListEventsFunc: func(ctx context.Context, filter event.EventFilter) ([]event.Event, error) {
			return events, nil
		},
		GetEventByIDFunc: func(ctx context.Context, id int64) (event.Event, error) {
			return events[0], nil
		},
		UpdateEventFunc: func(ctx context.Context, ev event.Event) error {
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
				{ID: "pc_checkout_1", FirstName: "John", LastName: "Doe", SecurityCode: "ABCD", PlanningCenterLocationID: "pc_loc_1"},
			}, nil
		},
	}

	locationIDMap := sync.Map{}
	locationIDMap.Store("pc_loc_1", int64(1))

	err := testService(noCheckWindows(), eventRepo, checkinRepo, pcClient, &locationIDMap, 5*time.Minute, true).eventCheckoutLoop(t.Context(), nil)
	require.NoError(t, err)
	assert.Len(t, createdCheckins, 1, "events without configured windows should still be fetched")
}

func Test_eventCheckoutLoop_windowRepoError(t *testing.T) {
	eventRepo := &event.MockRepo{
		ListEventsFunc: func(ctx context.Context, filter event.EventFilter) ([]event.Event, error) {
			return []event.Event{{ID: 1, PlanningCenterID: "evt_1", AutoFetch: true}}, nil
		},
	}

	checkWindowRepo := &eventcheckwindow.MockRepo{
		ListCheckWindowsFunc: func(ctx context.Context, filter eventcheckwindow.Filter) ([]eventcheckwindow.EventCheckWindow, error) {
			return nil, assert.AnError
		},
	}

	err := testService(checkWindowRepo, eventRepo, &checkin.MockRepo{}, &planningcenter.MockClient{}, &sync.Map{}, 5*time.Minute, true).eventCheckoutLoop(t.Context(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list check windows")
}

func Test_eventCheckoutLoop_windowsDisabled_ignoresWindows(t *testing.T) {
	events := []event.Event{
		{ID: 1, PlanningCenterID: "evt_1", AutoFetch: true, LastCheckedOutTime: time.Time{}},
	}

	eventRepo := &event.MockRepo{
		ListEventsFunc: func(ctx context.Context, filter event.EventFilter) ([]event.Event, error) {
			return events, nil
		},
		GetEventByIDFunc: func(ctx context.Context, id int64) (event.Event, error) {
			return events[0], nil
		},
		UpdateEventFunc: func(ctx context.Context, ev event.Event) error {
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
				{ID: "pc_checkout_1", FirstName: "John", LastName: "Doe", SecurityCode: "ABCD", PlanningCenterLocationID: "pc_loc_1"},
			}, nil
		},
	}

	locationIDMap := sync.Map{}
	locationIDMap.Store("pc_loc_1", int64(1))

	checkWindowRepo := &eventcheckwindow.MockRepo{
		ListCheckWindowsFunc: func(ctx context.Context, filter eventcheckwindow.Filter) ([]eventcheckwindow.EventCheckWindow, error) {
			return nil, assert.AnError
		},
	}

	err := testService(checkWindowRepo, eventRepo, checkinRepo, pcClient, &locationIDMap, 5*time.Minute, false).eventCheckoutLoop(t.Context(), nil)
	require.NoError(t, err)
	assert.Len(t, createdCheckins, 1, "events should be fetched regardless of windows when the flag is off")
	assert.Zero(t, checkWindowRepo.ListCheckWindowsFuncCallCount, "check windows should not be queried when the flag is off")
}
