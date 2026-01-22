package main

import (
	"os"

	"github.com/STop211650/sonoscli/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
