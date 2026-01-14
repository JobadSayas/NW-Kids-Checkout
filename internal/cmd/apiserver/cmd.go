package apiserver

import (
	"context"
	"errors"

	"kids-checkin/internal/controllers"
	"kids-checkin/internal/db"

	"github.com/urfave/cli/v3"
)

func ServeCmd(ctx context.Context, cmd *cli.Command) error {
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
