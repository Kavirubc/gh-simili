package main

import (
	"os"

	"github.com/Kavirubc/gh-simili/internal/cli"
)

func main() {
	// Filter out empty arguments passed by GitHub Actions expressions
	args := []string{}
	for _, arg := range os.Args {
		if arg != "" {
			args = append(args, arg)
		}
	}
	os.Args = args

	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
