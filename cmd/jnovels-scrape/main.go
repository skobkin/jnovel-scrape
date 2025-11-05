package main

import (
	"context"
	"os"

	"git.skobk.in/skobkin/jnovel-scrape/internal/app"
)

func main() {
	logger := app.NewLogger(os.Stderr)

	cfg, err := app.ParseArgs(os.Args[1:], os.Stderr)
	if err != nil {
		logger.Errorf("%v", err)
		os.Exit(2)
	}

	if err := app.Run(context.Background(), cfg, logger); err != nil {
		logger.Errorf("%v", err)
		os.Exit(1)
	}
}
