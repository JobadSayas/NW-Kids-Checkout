package location

import (
	"context"
	"log/slog"

	"kids-checkin/internal/client/planningcenter"
	"kids-checkin/internal/db"
	"kids-checkin/internal/repo/location"

	"github.com/urfave/cli/v3"
)

var Commands = []*cli.Command{
	{
		Name:  "upsert-location",
		Usage: "Upserts locations from Planning Center",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "location-id",
				Usage:    "Planning Center ID of the location to fetch",
				Required: true,
			},
			&cli.StringFlag{
				Name:    "db-file",
				Value:   "kids-checkin.db",
				Sources: cli.NewValueSourceChain(cli.EnvVar("DB_FILE")),
			},
		},
		Action: upsertLocation,
	},
}

func upsertLocation(ctx context.Context, cmd *cli.Command) error {
	dbFile := cmd.String("db-file")
	database, err := db.InitDB(dbFile)
	if err != nil {
		panic(err)
	}

	defer database.Close()

	pcClient := planningcenter.NewClient()
	locationRepo := location.NewRepo(database)

	locations, err := pcClient.GetLocation(ctx, cmd.String("location-id"), true)
	if err != nil {
		return err
	}

	for _, l := range locations {
		_, err = locationRepo.CreateLocation(ctx, location.Location{
			PlanningCenterID:       l.ID,
			PlanningCenterParentID: l.ParentID,
			Name:                   l.Name,
		})
		if err != nil {
			return err
		}
	}

	slog.Info("done upserting locations", slog.Int("locations_count", len(locations)))
	return nil
}
