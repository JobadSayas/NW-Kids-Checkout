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
	"kids-checkin/internal/repo/location"
	"kids-checkin/internal/static"

	"github.com/google/uuid"
	"github.com/urfave/cli/v3"
)

func FetchCheckouts(ctx context.Context, cmd *cli.Command) error {
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

	pcClient, checkinRepo, locationsRepo := getClients(database)

	for {
		if ctx.Err() != nil {
			break
		}

		slog.Info("checking for locations that need updating")

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
		slog.Info("no locations need updating")
		return nil
	}

	slog.Info("processing locations that need updating", slog.Int("total_locations", len(locations)), slog.Int("locations_to_update", len(locationsToUpdate)))

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
			slog.Warn("timeout fetching checkouts for location", slog.String("location_id", loc.PlanningCenterID), slog.String("error", err.Error()))
			return
		}
		errCh <- fmt.Errorf("failed to fetch checkouts for location %s: %w", loc.PlanningCenterID, err)
		return
	}
	slog.Info("fetched checkouts for location", slog.String("location_id", loc.PlanningCenterID), slog.Int("checkouts_count", len(checkouts)))

	lastCheckedOutTimeUnix := time.Time{}.Unix()

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

		lastCheckedOutTimeUnix = max(lastCheckedOutTimeUnix, co.CheckedOutAt.Unix())
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

func getClients(db *sql.DB) (planningcenter.Client, checkin.Repo, location.Repo) {
	if strings.ToLower(os.Getenv("CHECKOUT_FETCHER_USE_MOCK")) != "true" {
		return planningcenter.NewClient(), checkin.NewRepo(db), location.NewRepo(db)
	}

	locationsRepo := location.NewRepo(db)
	checkinRepo := checkin.NewRepo(db)

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

			return []planningcenter.Checkout{
				{
					ID:           "pcloc_" + uuid.New().String(),
					FirstName:    static.RandomFirstName(),
					LastName:     static.RandomLastName(),
					SecurityCode: strings.ToUpper(uuid.New().String()[:4]),
				},
			}, nil
		},
	}

	return pcClient, checkinRepo, locationsRepo
}
