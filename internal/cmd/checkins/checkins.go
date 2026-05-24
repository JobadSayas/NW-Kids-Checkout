package checkins

import (
	"context"
	"log/slog"
	"time"

	"kids-checkin/internal/db"
	"kids-checkin/internal/repo/checkin"
	"kids-checkin/internal/repo/manualcheckin"

	"github.com/urfave/cli/v3"
)

var Commands = []*cli.Command{
	{
		Name:  "delete-old",
		Usage: "Deletes old checkins older than the specified age",
		Flags: []cli.Flag{
			&cli.DurationFlag{
				Name:    "age",
				Value:   -7 * 24 * time.Hour, // 7 days ago
				Sources: cli.NewValueSourceChain(cli.EnvVar("CHECKINS_DELETE_OLDER_THAN_AGE")),
			},
			&cli.StringFlag{
				Name:    "db-file",
				Value:   "kids-checkin.db",
				Sources: cli.NewValueSourceChain(cli.EnvVar("DB_FILE")),
			},
		},
		Action: deleteOlderThanCmd,
	},
}

func deleteOlderThanCmd(ctx context.Context, cmd *cli.Command) error {
	olderThan := cmd.Duration("age")
	if olderThan > 0 {
		return cli.Exit("Age in the future is not allowed. Use a negative value", 1)
	}

	dbFile := cmd.String("db-file")
	database, err := db.InitDB(dbFile)
	if err != nil {
		panic(err)
	}

	defer database.Close()

	slog.InfoContext(ctx, "starting checkins delete-old", slog.Duration("age", olderThan), slog.String("db_file", dbFile))

	checkinRepo := checkin.NewRepo(database)
	deletedCount, err := checkinRepo.RemoveOldCheckins(ctx, time.Now().Add(olderThan))
	if err != nil {
		return cli.Exit(err.Error(), 1)
	}

	slog.InfoContext(ctx, "deleted old checkins", slog.Int64("deleted_count", deletedCount), slog.Duration("older_than", olderThan))

	manualCheckinRepo := manualcheckin.NewRepo(database)
	deletedCount, err = manualCheckinRepo.RemoveOldManualCheckins(ctx, time.Now().Add(olderThan))
	if err != nil {
	}

	slog.InfoContext(ctx, "deleted old manual checkins", slog.Int64("deleted_count", deletedCount), slog.Duration("older_than", olderThan))

	return nil
}
