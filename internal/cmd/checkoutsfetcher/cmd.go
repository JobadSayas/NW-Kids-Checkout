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
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"kids-checkin/internal/client/planningcenter"
	"kids-checkin/internal/db"
	"kids-checkin/internal/logger"
	"kids-checkin/internal/repo/checkin"
	"kids-checkin/internal/repo/event"
	"kids-checkin/internal/repo/eventcheckwindow"
	"kids-checkin/internal/repo/location"
	"kids-checkin/internal/static"

	"github.com/google/uuid"
	"github.com/urfave/cli/v3"
)

// shutdownGracePeriod is how long the fetcher waits for in-flight work to drain
// after a shutdown signal before force-exiting. It is set slightly above the
// Planning Center client's HTTP timeout so a single in-flight request can
// finish before the force-exit fires.
const shutdownGracePeriod = 11 * time.Second

// Service coordinates the by-event checkout fetcher. It owns the Planning
// Center client, the repos it reads and writes, and the in-memory location
// map (PlanningCenterID -> local ID) used to resolve checkout locations.
type Service struct {
	pcClient        planningcenter.Client
	eventRepo       event.Repo
	checkinRepo     checkin.Repo
	checkWindowRepo eventcheckwindow.Repo
	locationsRepo   location.Repo

	locationIDMap       *sync.Map
	eventMutexes        sync.Map
	eventUpdateInterval time.Duration
	useCheckWindows     bool
}

func FetchCheckouts(ctx context.Context, cmd *cli.Command) error {
	return FetchCheckoutsV2(ctx, cmd)
}

// FetchCheckoutsV2 runs the by-event checkout fetcher. It periodically polls
// Planning Center for new checkouts per event, using a background goroutine
// to keep an in-memory location map (PlanningCenterID -> local ID) refreshed
// every 5 minutes.
func FetchCheckoutsV2(ctx context.Context, cmd *cli.Command) error {
	dbFile := cmd.String("db-file")
	database, err := db.InitDB(dbFile)
	if err != nil {
		panic(err)
	}

	defer database.Close()

	interval := cmd.Duration("interval")
	if interval <= 0 {
		return fmt.Errorf("interval must be greater than 0")
	}

	eventUpdateInterval := cmd.Duration("event-update-interval")
	if eventUpdateInterval <= 0 {
		return fmt.Errorf("event-update-interval must be greater than 0")
	}

	runtime := cmd.Duration("runtime")
	useCheckWindows := cmd.Bool("use-check-windows")

	var cancel context.CancelFunc
	serviceMode := cmd.Bool("service")
	if serviceMode {
		ctx, cancel = context.WithCancel(ctx)
	} else {
		if runtime <= 0 {
			return fmt.Errorf("runtime must be greater than 0")
		}
		ctx, cancel = context.WithTimeout(ctx, runtime)
	}
	defer cancel()

	logCtx := logger.WithLogger(ctx, slog.With(slog.String("worker", "checkout-fetcher-v2")))

	if serviceMode {
		logger.FromContext(logCtx).WarnContext(logCtx, "running as service: --runtime is ignored")
	}

	stopCh := make(chan struct{})
	forceExitCh := make(chan struct{})
	var stopOnce sync.Once

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		stopOnce.Do(func() {
			logger.FromContext(logCtx).InfoContext(logCtx, "received shutdown signal, draining; no new work will be scheduled")
			close(stopCh)
			time.AfterFunc(shutdownGracePeriod, func() {
				close(forceExitCh)
			})
		})
		<-sigCh
		logger.FromContext(logCtx).ErrorContext(logCtx, "received second shutdown signal, exiting immediately")
		os.Exit(1)
	}()

	pcClient, checkinRepo, locationsRepo, eventRepo, eventCheckWindowRepo := getClients(database)

	logAttrs := []any{
		slog.Bool("service", serviceMode),
		slog.Duration("interval", interval),
		slog.Duration("event_update_interval", eventUpdateInterval),
		slog.String("db_file", dbFile),
	}
	if !serviceMode {
		logAttrs = append(logAttrs, slog.Duration("runtime", runtime))
	}
	logAttrs = append(logAttrs, slog.Bool("use_check_windows", useCheckWindows))
	logger.FromContext(logCtx).InfoContext(logCtx, "starting checkout fetcher (by event)", logAttrs...)

	locationMap := sync.Map{}
	svc := &Service{
		pcClient:            pcClient,
		eventRepo:           eventRepo,
		checkinRepo:         checkinRepo,
		checkWindowRepo:     eventCheckWindowRepo,
		locationsRepo:       locationsRepo,
		locationIDMap:       &locationMap,
		eventUpdateInterval: eventUpdateInterval,
		useCheckWindows:     useCheckWindows,
	}

	if err := svc.loadLocationMap(ctx); err != nil {
		return fmt.Errorf("failed to load location map: %w", err)
	}

	go svc.refreshLocationMapLoop(logCtx, stopCh)

	return svc.run(logCtx, stopCh, forceExitCh, interval)
}

// loadLocationMap populates the service's in-memory location map from the
// locations repo.
func (s *Service) loadLocationMap(ctx context.Context) error {
	locations, err := s.locationsRepo.ListLocations(ctx, location.LocationFilter{})
	if err != nil {
		return err
	}

	for _, loc := range locations {
		s.locationIDMap.Store(loc.PlanningCenterID, loc.ID)
	}
	return nil
}

// refreshLocationMapLoop keeps s.locationIDMap in sync with the locations
// repo. It runs until ctx is cancelled or stopCh is closed.
func (s *Service) refreshLocationMapLoop(ctx context.Context, stopCh <-chan struct{}) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-stopCh:
			return
		case <-ticker.C:
		}

		locations, err := s.locationsRepo.ListLocations(ctx, location.LocationFilter{})
		if err != nil {
			logger.FromContext(ctx).ErrorContext(ctx, "could not fetch locations", "error", err)
			continue
		}

		// Mark-and-sweep: add new entries, remove stale ones that no longer
		// appear in the repo.
		seen := make(map[string]bool, len(locations))
		for _, loc := range locations {
			seen[loc.PlanningCenterID] = true
			s.locationIDMap.Store(loc.PlanningCenterID, loc.ID)
		}
		s.locationIDMap.Range(func(key, _ any) bool {
			if !seen[key.(string)] {
				s.locationIDMap.Delete(key)
			}
			return true
		})
	}
}

// run executes the main fetch loop, polling Planning Center for checkouts on
// the configured interval until it stops.
func (s *Service) run(ctx context.Context, stopCh, forceExitCh <-chan struct{}, interval time.Duration) error {
	for {
		select {
		case <-forceExitCh:
			logger.FromContext(ctx).ErrorContext(ctx, "graceful shutdown did not complete within the grace period, forcing exit", slog.Duration("grace_period", shutdownGracePeriod))
			return fmt.Errorf("graceful shutdown did not complete within %s", shutdownGracePeriod)
		default:
		}

		select {
		case <-stopCh:
			return nil
		default:
		}

		if ctx.Err() != nil {
			return nil
		}

		logger.FromContext(ctx).InfoContext(ctx, "checking for events that need updating")

		err := s.eventCheckoutLoop(ctx, stopCh)
		if err != nil {
			if shouldSwallowLoopError(ctx, stopCh) {
				continue
			}
			return fmt.Errorf("failed to eventCheckoutLoop: %w", err)
		}

		select {
		case <-forceExitCh:
			logger.FromContext(ctx).ErrorContext(ctx, "graceful shutdown did not complete within the grace period, forcing exit", slog.Duration("grace_period", shutdownGracePeriod))
			return fmt.Errorf("graceful shutdown did not complete within %s", shutdownGracePeriod)
		case <-stopCh:
			return nil
		case <-ctx.Done():
			return nil
		case <-time.After(interval):
		}
	}
}

// shouldSwallowLoopError reports whether a per-loop error should be treated as
// non-fatal because the fetcher is shutting down or its runtime has expired.
// During a graceful drain the context is intentionally not cancelled so
// in-flight work can complete, so stopCh must be checked in addition to ctx.
func shouldSwallowLoopError(ctx context.Context, stopCh <-chan struct{}) bool {
	select {
	case <-stopCh:
		return true
	default:
	}
	return ctx.Err() != nil
}

func (s *Service) eventCheckoutLoop(ctx context.Context, stopCh <-chan struct{}) error {
	select {
	case <-stopCh:
		return nil
	default:
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}

	autoFetch := true
	events, err := s.eventRepo.ListEvents(ctx, event.EventFilter{
		AutoFetch: &autoFetch,
	})
	if err != nil {
		return fmt.Errorf("failed to list events: %w", err)
	}

	windowsByEvent := make(map[int64][]eventcheckwindow.EventCheckWindow)
	var active map[int64]bool
	if s.useCheckWindows {
		windows, err := s.checkWindowRepo.ListCheckWindows(ctx, eventcheckwindow.Filter{})
		if err != nil {
			return fmt.Errorf("failed to list check windows: %w", err)
		}

		for _, w := range windows {
			windowsByEvent[w.EventID] = append(windowsByEvent[w.EventID], w)
		}

		active = activeEventIDs(windowsByEvent)
	}

	var eventsToUpdate []event.Event
	skippedOutsideWindow := 0
	now := time.Now()
	for _, ev := range events {
		if s.useCheckWindows {
			if _, hasWindows := windowsByEvent[ev.ID]; hasWindows && !active[ev.ID] {
				skippedOutsideWindow++
				continue
			}
		}
		if ev.LastCheckedOutTime.IsZero() || now.Sub(ev.LastCheckedOutTime) >= s.eventUpdateInterval {
			eventsToUpdate = append(eventsToUpdate, ev)
		}
	}

	if skippedOutsideWindow > 0 {
		logger.FromContext(ctx).InfoContext(ctx, "events skipped: outside configured check window", slog.Int("skipped_count", skippedOutsideWindow))
	}

	if len(eventsToUpdate) == 0 {
		logger.FromContext(ctx).InfoContext(ctx, "no events need updating")
		return nil
	}

	logger.FromContext(ctx).InfoContext(ctx, "processing events that need updating", slog.Int("total_events", len(events)), slog.Int("events_to_update", len(eventsToUpdate)))

	var (
		wg    sync.WaitGroup
		errs  []error
		errMu sync.Mutex
		sem   = make(chan struct{}, 5)
		errCh = make(chan error, len(eventsToUpdate))
	)

loop:
	for _, ev := range eventsToUpdate {
		select {
		case <-ctx.Done():
			errMu.Lock()
			errs = append(errs, ctx.Err())
			errMu.Unlock()
			break loop
		case <-stopCh:
			break loop
		case sem <- struct{}{}:
			wg.Add(1)
			go func(ev event.Event) {
				defer wg.Done()
				defer func() { <-sem }()
				s.processEventCheckouts(ctx, ev, errCh)
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

func (s *Service) getEventMutex(eventID int64) *sync.Mutex {
	return getEventMutex(&s.eventMutexes, eventID)
}

func (s *Service) processEventCheckouts(ctx context.Context, ev event.Event, errCh chan<- error) {
	mu := s.getEventMutex(ev.ID)
	mu.Lock()
	defer mu.Unlock()

	currentEvent, err := s.eventRepo.GetEventByID(ctx, ev.ID)
	if err != nil {
		errCh <- fmt.Errorf("failed to get event %d: %w", ev.ID, err)
		return
	}

	timeToUse := currentEvent.LastCheckedOutTime
	if timeToUse.Before(time.Now().Add(getLookBackTime())) {
		timeToUse = time.Now().Add(getLookBackTime())
	}

	checkouts, err := s.pcClient.GetCheckoutsForEvent(ctx, currentEvent.PlanningCenterID, timeToUse, 0)

	if err != nil {
		var timeoutErr *planningcenter.TimeoutError
		if errors.As(err, &timeoutErr) {
			logger.FromContext(ctx).WarnContext(ctx, "timeout fetching checkouts for event", slog.String("event_id", currentEvent.PlanningCenterID), slog.String("error", err.Error()), slog.Any("err", err))
			return
		}
		errCh <- fmt.Errorf("failed to fetch checkouts for event %s: %w", currentEvent.PlanningCenterID, err)
		return
	}
	logger.FromContext(ctx).InfoContext(ctx, "fetched checkouts for event", slog.String("event_id", currentEvent.PlanningCenterID), slog.Int("checkouts_count", len(checkouts)))

	for _, checkout := range checkouts {
		locationIDA, found := s.locationIDMap.Load(checkout.PlanningCenterLocationID)
		if !found {
			logger.FromContext(ctx).ErrorContext(ctx, "could not find location by planning center id in map", "checkout_pc_id", checkout.PlanningCenterLocationID)
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

		co, err := s.checkinRepo.CreateCheckin(ctx, co)
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

	err = s.eventRepo.UpdateEvent(ctx, updatedEvent)
	if err != nil {
		errCh <- fmt.Errorf("failed to update event %d: %w", currentEvent.ID, err)
		return
	}

	currentEvent.LastCheckedOutTime = newLastCheckedOut
}

// minutesPerWeek is one week expressed in minutes (Monday 00:00 UTC to the
// following Monday 00:00 UTC).
const minutesPerWeek = 7 * 24 * 60

// Window represents a time range within a week, in minutes elapsed since Monday
// 00:00 UTC. EndMinutes is exclusive.
type Window struct {
	StartMinutes int
	EndMinutes   int
}

// nowFunc is the time source used by the check-window helpers; overridable in
// tests.
var nowFunc = time.Now

// minutesSinceWeekStartUTC returns how many minutes have elapsed since Monday
// 00:00:00 UTC of the current week.
func minutesSinceWeekStartUTC() int {
	now := nowFunc().UTC()
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	weekday--
	weekStart := now.AddDate(0, 0, -weekday)
	weekStart = weekStart.Truncate(24 * time.Hour)
	return int(now.Sub(weekStart).Minutes())
}

func parseTime(timeStr string) (hour, minute int, err error) {
	parts := strings.Split(timeStr, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid time format: %s", timeStr)
	}
	parts[0] = strings.TrimLeft(parts[0], "0")
	if parts[0] == "" {
		parts[0] = "0"
	}
	hour, err = strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return 0, 0, fmt.Errorf("invalid hour: %s", timeStr)
	}

	parts[1] = strings.TrimLeft(parts[1], "0")
	if parts[1] == "" {
		parts[1] = "0"
	}
	minute, err = strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return 0, 0, fmt.Errorf("invalid minute: %s", timeStr)
	}
	return hour, minute, nil
}

// localToUTCWeekMinutes converts a local (day of week, hour, minute, timezone)
// to minutes since Monday 00:00 UTC, anchored to the week that nowFunc() falls
// in.
func localToUTCWeekMinutes(dayOfWeek, hour, minute int, timezone string) (int, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return 0, fmt.Errorf("invalid timezone %q: %w", timezone, err)
	}

	now := nowFunc().In(loc)

	utcNow := now.UTC()
	utcWeekday := int(utcNow.Weekday())
	if utcWeekday == 0 {
		utcWeekday = 7
	}
	utcWeekday--
	utcWeekStart := utcNow.AddDate(0, 0, -utcWeekday)
	utcWeekStart = time.Date(utcWeekStart.Year(), utcWeekStart.Month(), utcWeekStart.Day(), 0, 0, 0, 0, time.UTC)

	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	weekday--

	weekStart := time.Date(now.Year(), now.Month(), now.Day()-weekday, 0, 0, 0, 0, loc)
	targetTime := time.Date(
		weekStart.Year(), weekStart.Month(), weekStart.Day()+(dayOfWeek-1),
		hour, minute, 0, 0, loc,
	)

	return int(targetTime.UTC().Sub(utcWeekStart).Minutes()), nil
}

// mergeWindows converts check windows into merged UTC week-minute ranges,
// splitting windows that cross the Sunday/Saturday boundary into two ranges
// and coalescing overlaps.
func mergeWindows(checkWindows []eventcheckwindow.EventCheckWindow) ([]Window, error) {
	var ranges []Window

	for _, ct := range checkWindows {
		startH, startM, err := parseTime(ct.StartTime)
		if err != nil {
			return nil, fmt.Errorf("parse start time: %w", err)
		}
		endH, endM, err := parseTime(ct.EndTime)
		if err != nil {
			return nil, fmt.Errorf("parse end time: %w", err)
		}

		startMinutes, err := localToUTCWeekMinutes(ct.StartDayOfWeek, startH, startM, ct.Timezone)
		if err != nil {
			return nil, fmt.Errorf("start time timezone: %w", err)
		}
		endMinutes, err := localToUTCWeekMinutes(ct.EndDayOfWeek, endH, endM, ct.Timezone)
		if err != nil {
			return nil, fmt.Errorf("end time timezone: %w", err)
		}

		if startMinutes <= endMinutes {
			ranges = append(ranges, Window{startMinutes, endMinutes})
		} else {
			ranges = append(ranges,
				Window{startMinutes, minutesPerWeek},
				Window{0, endMinutes},
			)
		}
	}

	sort.Slice(ranges, func(i, j int) bool {
		return ranges[i].StartMinutes < ranges[j].StartMinutes
	})

	merged := make([]Window, 0)
	for _, r := range ranges {
		if len(merged) == 0 || merged[len(merged)-1].EndMinutes < r.StartMinutes {
			merged = append(merged, r)
		} else {
			if r.EndMinutes > merged[len(merged)-1].EndMinutes {
				merged[len(merged)-1].EndMinutes = r.EndMinutes
			}
		}
	}

	return merged, nil
}

// activeEventIDs returns the set of event IDs whose merged check windows cover
// the current time. Events with no windows are not included.
func activeEventIDs(windowsByEvent map[int64][]eventcheckwindow.EventCheckWindow) map[int64]bool {
	active := make(map[int64]bool, len(windowsByEvent))
	nowMinutes := minutesSinceWeekStartUTC()

	for eventID, eventWindows := range windowsByEvent {
		merged, err := mergeWindows(eventWindows)
		if err != nil {
			slog.Warn("could not merge check windows for event", slog.Int64("event_id", eventID), slog.String("error", err.Error()))
			continue
		}

		for _, w := range merged {
			if nowMinutes >= w.StartMinutes && nowMinutes < w.EndMinutes {
				active[eventID] = true
				break
			}
		}
	}

	return active
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

func getClients(db *sql.DB) (planningcenter.Client, checkin.Repo, location.Repo, event.Repo, eventcheckwindow.Repo) {
	if strings.ToLower(os.Getenv("CHECKOUT_FETCHER_USE_MOCK")) != "true" {
		return planningcenter.NewClient(), checkin.NewRepo(db), location.NewRepo(db), event.NewRepo(db), eventcheckwindow.NewRepo(db)
	}

	locationsRepo := location.NewRepo(db)
	checkinRepo := checkin.NewRepo(db)
	eventRepo := event.NewRepo(db)
	eventCheckWindowRepo := eventcheckwindow.NewRepo(db)

	var pcClient planningcenter.Client = &planningcenter.MockClient{
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

	return pcClient, checkinRepo, locationsRepo, eventRepo, eventCheckWindowRepo
}
