package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"kids-checkin/internal/client/planningcenter"
	"kids-checkin/internal/controllers"
	"kids-checkin/internal/db"
	"kids-checkin/internal/repo/checkin"
	"kids-checkin/internal/repo/location"
	"kids-checkin/internal/static"

	"github.com/google/uuid"
	"github.com/urfave/cli/v3"
)

func NewCommand() *cli.Command {
	return &cli.Command{
		Commands: []*cli.Command{
			{
				Name:  "apiserver",
				Usage: "Starts the API server",
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:    "port",
						Value:   3000,
						Sources: cli.NewValueSourceChain(cli.EnvVar("PORT")),
					},
					&cli.StringFlag{
						Name:    "db-file",
						Value:   "kids-checkin.db",
						Sources: cli.NewValueSourceChain(cli.EnvVar("DB_FILE")),
					},
				},
				Action: serveCmd,
			},
			{
				Name:  "checkout-fetcher",
				Usage: "Fetches checkouts from Planning Center",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "db-file",
						Value:   "kids-checkin.db",
						Sources: cli.NewValueSourceChain(cli.EnvVar("DB_FILE")),
					},
					&cli.DurationFlag{
						Name:    "interval",
						Value:   3 * time.Second,
						Sources: cli.NewValueSourceChain(cli.EnvVar("FETCH_CHECKOUTS_INTERVAL")),
					},
					&cli.DurationFlag{
						Name:  "runtime",
						Usage: "How long to run the fetcher for",
						Value: 5000 * time.Second,
					},
				},
				Action: fetchCheckoutsCmd,
			},
		},
	}
}

func serveCmd(ctx context.Context, cmd *cli.Command) error {
	port := cmd.Int("port")

	if port <= 0 {
		return errors.New("port must be greater than 0")
	}

	dbFile := cmd.String("db-file")
	database, err := db.InitDB(dbFile)
	if err != nil {
		panic(err)
	}

	err = controllers.StartServer(port, database)
	if err != nil {
		panic(err)
	}
	return nil
}

func fetchCheckoutsCmd(ctx context.Context, cmd *cli.Command) error {
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

	locationsRepo := location.NewRepo(database)
	_, err = locationsRepo.CreateLocation(ctx, location.Location{PlanningCenterID: "pcloc_12345", Name: "Test Location"})
	if err != nil {
		return fmt.Errorf("failed to create test location: %w", err)
	}
	checkinRepo := checkin.NewRepo(database)

	pcClient := &planningcenter.MockClient{
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
				{
					ID:           "pcloc_" + uuid.New().String(),
					FirstName:    static.RandomFirstName(),
					LastName:     static.RandomLastName(),
					SecurityCode: strings.ToUpper(uuid.New().String()[:4]),
				},
			}, nil
		},
	}

	for {
		if ctx.Err() != nil {
			break
		}

		err = funLoop(ctx, locationsRepo, checkinRepo, pcClient)
		if err != nil {
			return fmt.Errorf("failed to fetch locations: %w", err)
		}

		slog.Info("done fetching. sleeping", slog.Duration("sleep_duration", interval))
		time.Sleep(interval)
	}

	return nil
}

func funLoop(ctx context.Context, locationRepo location.Repo, checkinRepo checkin.Repo, pcClient planningcenter.Client) error {
	locations, err := locationRepo.ListLocations(ctx, location.Filter{})
	if err != nil {
		return fmt.Errorf("failed to list locations: %w", err)
	}

	fmt.Printf("found %d locations\n", len(locations))

	for _, loc := range locations {
		checkouts, err := pcClient.GetCheckoutsForLocation(ctx, loc.PlanningCenterID, time.Time{})
		if err != nil {
			return fmt.Errorf("failed to fetch checkouts for location %s: %w", loc.PlanningCenterID, err)
		}
		slog.Info("fetched checkouts for location", slog.String("location_id", loc.PlanningCenterID), slog.Int("checkouts_count", len(checkouts)))
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
				return fmt.Errorf("failed to create checkin: %w", err)
			}
		}
	}
	return nil
}
