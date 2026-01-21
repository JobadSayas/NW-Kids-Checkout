package apiserver

import (
	"context"
	"errors"

	"kids-checkin/internal/controllers"

	"github.com/urfave/cli/v3"
)

func ServeCmd(ctx context.Context, cmd *cli.Command) error {
	port := cmd.Int("port")

	if port <= 0 {
		return errors.New("port must be greater than 0")
	}

	err := controllers.StartServer(port, cmd.String("db-file"))
	if err != nil {
		panic(err)
	}
	return nil
}
