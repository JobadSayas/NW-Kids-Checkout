package main

import (
	"context"
	"fmt"
	"os"

	"kids-checkin/internal/cmd"
)

func main() {
	err := (cmd.NewCommand()).Run(context.Background(), os.Args)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
