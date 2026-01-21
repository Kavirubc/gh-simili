package main

import (
	"os"

	"github.com/kaviruhapuarachchi/gh-simili/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
