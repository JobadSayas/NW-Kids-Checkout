package main

import (
	"context"
	"os"

	"kids-checkin/internal/cmd"
)

func main() {
	_ = (cmd.NewCommand()).Run(context.Background(), os.Args)
}
