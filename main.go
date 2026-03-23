package main

import (
	"os"

	"dangernoodle.io/terratools/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
