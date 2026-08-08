package main

import (
	"context"
	"fmt"
	"os"

	"git.skobk.in/skobkin/jnovel-scrape/internal/app"
)

func main() {
	logger := app.NewLogger(os.Stderr)

	// --version is handled before ParseArgs so it does not interact with
	// the main flag set (it must short-circuit before --until is required).
	if hasVersionFlag(os.Args[1:]) {
		fmt.Println(app.Version)

		return
	}

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

// hasVersionFlag reports whether any positional argument equals
// "--version" or "-version". The main flag set does not register --version
// because it must short-circuit before --until is required and must work
// regardless of other argument ordering.
func hasVersionFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--version" || arg == "-version" {
			return true
		}
	}

	return false
}
