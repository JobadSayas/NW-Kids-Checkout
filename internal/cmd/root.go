package cmd

import (
	"time"

	"kids-checkin/internal/cmd/apiserver"
	"kids-checkin/internal/cmd/checkoutsfetcher"
	"kids-checkin/internal/cmd/location"

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
				Action: apiserver.ServeCmd,
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
				Action: checkoutsfetcher.FetchCheckouts,
			},
			{
				Name:     "locations",
				Usage:    "Commands to manage locations",
				Commands: location.Commands,
			},
		},
	}
}
