package main

import (
	"os"

	"github.com/steipete/sonoscli/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
