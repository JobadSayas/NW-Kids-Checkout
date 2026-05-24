package checkoutsfetcher

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"kids-checkin/internal/client/planningcenter"
	"kids-checkin/internal/db"
	"kids-checkin/internal/repo/checkin"
	"kids-checkin/internal/repo/event"
	"kids-checkin/internal/repo/location"
	"kids-checkin/internal/static"

	"github.com/google/uuid"
	"github.com/urfave/cli/v3"
)

func FetchCheckoutsV1(ctx context.Context, cmd *cli.Command) error {
	dbFile := cmd.String("db-file")
	database, err := db.InitDB(dbFile)
	if err != nil {
		panic(err)
	}

	defer database.Close()

	interval := cmd.Duration("interval")
	if interval <= 0 {
		return errors.New("interval must be greater than 0")
	}

	locationUpdateInterval := cmd.Duration("location-update-interval")
	if locationUpdateInterval <= 0 {
		return errors.New("location-update-interval must be greater than 0")
	}

	runtime := cmd.Duration("runtime")
	if runtime <= 0 {
		return errors.New("runtime must be greater than 0")
	}

	ctx, cancel := context.WithTimeout(ctx, runtime)
	defer cancel()

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		os.Exit(1)
	}()

	pcClient, checkinRepo, locationsRepo, _ := getClients(database)

	slog.InfoContext(ctx, "starting checkout fetcher (by location)",
		slog.Duration("interval", interval),
		slog.Duration("location_update_interval", locationUpdateInterval),
		slog.Duration("runtime", runtime),
		slog.String("db_file", dbFile))

	for {
		if ctx.Err() != nil {
			break
		}

		slog.InfoContext(ctx, "checking for locations that need updating")

		err = checkoutLoop(ctx, locationsRepo, checkinRepo, pcClient, locationUpdateInterval)
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			return fmt.Errorf("failed to checkoutLoop: %w", err)
		}

		time.Sleep(interval)
	}

	return nil
}

func FetchCheckouts(ctx context.Context, cmd *cli.Command) error {
	fetchByEvent := cmd.Bool("fetch-by-event")
	if fetchByEvent {
		return FetchCheckoutsV2(ctx, cmd)
	}
	return FetchCheckoutsV1(ctx, cmd)
}

func FetchCheckoutsV2(ctx context.Context, cmd *cli.Command) error {
	dbFile := cmd.String("db-file")
	database, err := db.InitDB(dbFile)
	if err != nil {
		panic(err)
	}

	defer database.Close()

	interval := cmd.Duration("interval")
	if interval <= 0 {
		return errors.New("interval must be greater than 0")
	}

	eventUpdateInterval := cmd.Duration("event-update-interval")
	if eventUpdateInterval <= 0 {
		return errors.New("event-update-interval must be greater than 0")
	}

	runtime := cmd.Duration("runtime")
	if runtime <= 0 {
		return errors.New("runtime must be greater than 0")
	}

	ctx, cancel := context.WithTimeout(ctx, runtime)
	defer cancel()

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		os.Exit(1)
	}()

	pcClient, checkinRepo, locationsRepo, eventRepo := getClients(database)

	slog.InfoContext(ctx, "starting checkout fetcher (by event)",
		slog.Duration("interval", interval),
		slog.Duration("event_update_interval", eventUpdateInterval),
		slog.Duration("runtime", runtime),
		slog.String("db_file", dbFile))

	locationMap := sync.Map{}

	locations, err := locationsRepo.ListLocations(ctx, location.LocationFilter{})
	if err != nil {
		return fmt.Errorf("failed to list locations: %w", err)
	}

	for _, loc := range locations {
		locationMap.Store(loc.PlanningCenterID, loc.ID)
	}

	go func() {
		// Refresh the data every 5 minutes
		for range time.Tick(5 * time.Minute) {
			locations, err := locationsRepo.ListLocations(ctx, location.LocationFilter{})
			if err != nil {
				slog.ErrorContext(ctx, "could not fetch locations", "error", err)
				continue
			}

			for _, loc := range locations {
				locationMap.Store(loc.PlanningCenterID, loc.ID)
			}
		}
	}()

	for {
		if ctx.Err() != nil {
			break
		}

		slog.InfoContext(ctx, "checking for events that need updating")

		err = eventCheckoutLoop(ctx, eventRepo, checkinRepo, pcClient, &locationMap, eventUpdateInterval)
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			return fmt.Errorf("failed to eventCheckoutLoop: %w", err)
		}

		time.Sleep(interval)
	}

	return nil
}

func eventCheckoutLoop(ctx context.Context, eventRepo event.Repo, checkinRepo checkin.Repo, pcClient planningcenter.Client, locationIDMap *sync.Map, eventUpdateInterval time.Duration) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	autoFetch := true
	events, err := eventRepo.ListEvents(ctx, event.EventFilter{
		AutoFetch: &autoFetch,
	})
	if err != nil {
		return fmt.Errorf("failed to list events: %w", err)
	}

	var eventsToUpdate []event.Event
	now := time.Now()
	for _, ev := range events {
		if ev.LastCheckedOutTime.IsZero() || now.Sub(ev.LastCheckedOutTime) >= eventUpdateInterval {
			eventsToUpdate = append(eventsToUpdate, ev)
		}
	}

	if len(eventsToUpdate) == 0 {
		slog.InfoContext(ctx, "no events need updating")
		return nil
	}

	slog.InfoContext(ctx, "processing events that need updating", slog.Int("total_events", len(events)), slog.Int("events_to_update", len(eventsToUpdate)))

	var (
		wg           sync.WaitGroup
		errs         []error
		errMu        sync.Mutex
		sem          = make(chan struct{}, 5)
		errCh        = make(chan error, len(eventsToUpdate))
		eventMutexes sync.Map
	)

	for _, ev := range eventsToUpdate {
		select {
		case <-ctx.Done():
			errMu.Lock()
			errs = append(errs, ctx.Err())
			errMu.Unlock()
			break
		case sem <- struct{}{}:
			wg.Add(1)
			go func(ev event.Event) {
				defer wg.Done()
				defer func() { <-sem }()
				processEventCheckouts(ctx, ev, checkinRepo, eventRepo, pcClient, locationIDMap, &eventMutexes, errCh)
			}(ev)
		}
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		errMu.Lock()
		errs = append(errs, err)
		errMu.Unlock()
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

func getEventMutex(m *sync.Map, eventID int64) *sync.Mutex {
	mu, _ := m.LoadOrStore(eventID, &sync.Mutex{})
	return mu.(*sync.Mutex)
}

func processEventCheckouts(ctx context.Context, ev event.Event, checkinRepo checkin.Repo, eventRepo event.Repo, pcClient planningcenter.Client, locationIDMap *sync.Map, eventMutexes *sync.Map, errCh chan<- error) {
	mu := getEventMutex(eventMutexes, ev.ID)
	mu.Lock()
	defer mu.Unlock()

	currentEvent, err := eventRepo.GetEventByID(ctx, ev.ID)
	if err != nil {
		errCh <- fmt.Errorf("failed to get event %d: %w", ev.ID, err)
		return
	}

	timeToUse := currentEvent.LastCheckedOutTime
	if timeToUse.Before(time.Now().Add(getLookBackTime())) {
		timeToUse = time.Now().Add(getLookBackTime())
	}

	checkouts, err := pcClient.GetCheckoutsForEvent(ctx, currentEvent.PlanningCenterID, timeToUse, 0)

	if err != nil {
		var timeoutErr *planningcenter.TimeoutError
		if errors.As(err, &timeoutErr) {
			slog.WarnContext(ctx, "timeout fetching checkouts for event", slog.String("event_id", currentEvent.PlanningCenterID), slog.String("error", err.Error()), slog.Any("err", err))
			return
		}
		errCh <- fmt.Errorf("failed to fetch checkouts for event %s: %w", currentEvent.PlanningCenterID, err)
		return
	}
	slog.InfoContext(ctx, "fetched checkouts for event", slog.String("event_id", currentEvent.PlanningCenterID), slog.Int("checkouts_count", len(checkouts)))

	for _, checkout := range checkouts {
		locationIDA, found := locationIDMap.Load(checkout.PlanningCenterLocationID)
		if !found {
			slog.ErrorContext(ctx, "could not find location by planning center id in map", "checkout_pc_id", checkout.PlanningCenterLocationID)
			continue
		}

		co := checkin.Checkin{
			PlanningCenterID: checkout.ID,
			LocationID:       locationIDA.(int64),
			EventID:          currentEvent.ID,
			FirstName:        checkout.FirstName,
			LastName:         checkout.LastName,
			SecurityCode:     checkout.SecurityCode,
			CheckedOutAt:     checkout.CheckedOutAt,
		}

		co, err := checkinRepo.CreateCheckin(ctx, co)
		if err != nil {
			errCh <- fmt.Errorf("failed to create checkin: %w", err)
			return
		}
	}

	newLastCheckedOut := time.Now().UTC()
	updatedEvent := event.Event{
		ID:                 currentEvent.ID,
		Name:               currentEvent.Name,
		PlanningCenterID:   currentEvent.PlanningCenterID,
		AutoFetch:          currentEvent.AutoFetch,
		LastCheckedOutTime: newLastCheckedOut,
		LocationGroupID:    currentEvent.LocationGroupID,
	}

	err = eventRepo.UpdateEvent(ctx, updatedEvent)
	if err != nil {
		errCh <- fmt.Errorf("failed to update event %d: %w", currentEvent.ID, err)
		return
	}

	currentEvent.LastCheckedOutTime = newLastCheckedOut
}

func checkoutLoop(ctx context.Context, locationRepo location.Repo, checkinRepo checkin.Repo, pcClient planningcenter.Client, locationUpdateInterval time.Duration) error {
	autoFetch := true
	locations, err := locationRepo.ListLocations(ctx, location.LocationFilter{
		AutoFetch: &autoFetch,
	})
	if err != nil {
		return fmt.Errorf("failed to list locations: %w", err)
	}

	// Filter locations that need updating (haven't been updated in locationUpdateInterval)
	var locationsToUpdate []location.Location
	now := time.Now()
	for _, loc := range locations {
		// If location has never been updated, or was updated more than locationUpdateInterval ago, include it
		if loc.LastCheckedOutTime.IsZero() || now.Sub(loc.LastCheckedOutTime) >= locationUpdateInterval {
			locationsToUpdate = append(locationsToUpdate, loc)
		}
	}

	if len(locationsToUpdate) == 0 {
		slog.InfoContext(ctx, "no locations need updating")
		return nil
	}

	slog.InfoContext(ctx, "processing locations that need updating", slog.Int("total_locations", len(locations)), slog.Int("locations_to_update", len(locationsToUpdate)))

	var (
		wg    sync.WaitGroup
		errs  []error
		errMu sync.Mutex                                 // Mutex to protect errs
		sem   = make(chan struct{}, 5)                   // Semaphore to limit concurrency to 5
		errCh = make(chan error, len(locationsToUpdate)) // Buffered channel for errors
	)

	for _, loc := range locationsToUpdate {
		select {
		case <-ctx.Done():
			// Context cancelled, stop launching new goroutines
			errs = append(errs, ctx.Err())
			break // Exit the loop when context is cancelled
		case sem <- struct{}{}: // Acquire a slot in the semaphore
			wg.Add(1)
			go func(loc location.Location) {
				defer wg.Done()
				defer func() { <-sem }() // Release the slot
				processLocationCheckouts(ctx, loc, checkinRepo, locationRepo, pcClient, errCh)
			}(loc)
		}
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		errMu.Lock()
		errs = append(errs, err)
		errMu.Unlock()
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// processLocationCheckouts fetches checkouts for a given location and updates the database.
func processLocationCheckouts(ctx context.Context, loc location.Location, checkinRepo checkin.Repo, locationRepo location.Repo, pcClient planningcenter.Client, errCh chan<- error) {
	timeToUse := loc.LastCheckedOutTime
	if timeToUse.Before(time.Now().Add(getLookBackTime())) {
		timeToUse = time.Now().Add(getLookBackTime())
	}

	checkouts, err := pcClient.GetCheckoutsForLocation(ctx, loc.PlanningCenterID, timeToUse, 0)

	if err != nil {
		var timeoutErr *planningcenter.TimeoutError
		if errors.As(err, &timeoutErr) {
			slog.WarnContext(ctx, "timeout fetching checkouts for location", slog.String("location_id", loc.PlanningCenterID), slog.String("error", err.Error()), slog.Any("err", err))
			return
		}
		errCh <- fmt.Errorf("failed to fetch checkouts for location %s: %w", loc.PlanningCenterID, err)
		return
	}
	slog.InfoContext(ctx, "fetched checkouts for location", slog.String("location_id", loc.PlanningCenterID), slog.Int("checkouts_count", len(checkouts)))

	for _, checkout := range checkouts {
		co := checkin.Checkin{
			PlanningCenterID: checkout.ID,
			LocationID:       loc.ID,
			FirstName:        checkout.FirstName,
			LastName:         checkout.LastName,
			SecurityCode:     checkout.SecurityCode,
			CheckedOutAt:     checkout.CheckedOutAt,
		}

		co, err := checkinRepo.CreateCheckin(ctx, co)
		if err != nil {
			errCh <- fmt.Errorf("failed to create checkin: %w", err)
			return
		}
	}

	// Set the last_checked_out_time to current time to track when this location was last processed
	loc.LastCheckedOutTime = time.Now().UTC()
	err = locationRepo.UpdateLocation(ctx, loc)
	if err != nil {
		errCh <- fmt.Errorf("failed to update location: %w", err)
		return
	}
}

// getLookBackTime returns the time to look back for checkouts based on the CHECKOUT_FETCHER_LOOKBACK_TIME env var
var getLookBackTime = sync.OnceValue(func() time.Duration {
	const defaultLookBackTime = -12 * time.Hour

	lbStr := os.Getenv("CHECKOUT_FETCHER_LOOKBACK_TIME")
	if lbStr == "" {
		return defaultLookBackTime
	}

	lb, err := time.ParseDuration(lbStr)
	if err != nil {
		slog.Warn("could not parse CHECKOUT_FETCHER_LOOKBACK_TIME, using default", slog.String("env_var", lbStr), slog.Duration("default", defaultLookBackTime))
		return defaultLookBackTime
	}

	if lb > 0 {
		slog.Warn("CHECKOUT_FETCHER_LOOKBACK_TIME must not be greater than 0, using default", slog.String("env_var", lbStr), slog.Duration("default", defaultLookBackTime))
		return defaultLookBackTime
	}

	return lb
})

func getClients(db *sql.DB) (planningcenter.Client, checkin.Repo, location.Repo, event.Repo) {
	if strings.ToLower(os.Getenv("CHECKOUT_FETCHER_USE_MOCK")) != "true" {
		return planningcenter.NewClient(), checkin.NewRepo(db), location.NewRepo(db), event.NewRepo(db)
	}

	locationsRepo := location.NewRepo(db)
	checkinRepo := checkin.NewRepo(db)
	eventRepo := event.NewRepo(db)

	var pcClient planningcenter.Client = &planningcenter.MockClient{
		GetCheckoutsForLocationFunc: func(ctx context.Context, locationID string, olderThan time.Time) ([]planningcenter.Checkout, error) {
			if rand.Intn(2) == 0 {
				cw, err := checkinRepo.ListCheckins(ctx, checkin.Filter{})
				if err != nil {
					return nil, err
				}

				if len(cw) != 0 {
					sort.Slice(cw, func(i, j int) bool {
						return cw[i].SecurityCode < cw[j].SecurityCode
					})
					pcco := make([]planningcenter.Checkout, 0, len(cw)/2)
					for i := 0; i < len(cw)/2; i++ {
						if !cw[i].CheckedOutAt.IsZero() {
							continue
						}
						pcco = append(pcco, planningcenter.Checkout{
							ID:           cw[i].PlanningCenterID,
							FirstName:    cw[i].FirstName,
							LastName:     cw[i].LastName,
							CheckedOutAt: time.Now().UTC(),
							SecurityCode: strings.ToUpper(cw[i].SecurityCode),
						})
					}
					return pcco, nil
				}
			}

			locations, err := locationsRepo.ListLocations(ctx, location.LocationFilter{})
			if err != nil {
				return nil, err
			}

			var locID string
			if len(locations) > 0 {
				loc := locations[rand.Intn(len(locations))]
				locID = loc.PlanningCenterID
			}

			return []planningcenter.Checkout{
				{
					ID:                       "pcloc_" + uuid.New().String(),
					FirstName:                static.RandomFirstName(),
					LastName:                 static.RandomLastName(),
					SecurityCode:             strings.ToUpper(uuid.New().String()[:4]),
					PlanningCenterLocationID: locID,
				},
			}, nil
		},
		GetCheckoutsForEventFunc: func(ctx context.Context, eventID string, checkedOutOnOrAfter time.Time, limit int) ([]planningcenter.Checkout, error) {
			locations, err := locationsRepo.ListLocations(ctx, location.LocationFilter{})
			if err != nil {
				return nil, err
			}

			var locID string
			if len(locations) > 0 {
				loc := locations[rand.Intn(len(locations))]
				locID = loc.PlanningCenterID
			}

			return []planningcenter.Checkout{
				{
					ID:                       "pccheckoutevent_" + uuid.New().String(),
					FirstName:                static.RandomFirstName(),
					LastName:                 static.RandomLastName(),
					SecurityCode:             strings.ToUpper(uuid.New().String()[:4]),
					CheckedOutAt:             time.Now().UTC(),
					PlanningCenterLocationID: locID,
				},
			}, nil
		},
	}

	return pcClient, checkinRepo, locationsRepo, eventRepo
}
