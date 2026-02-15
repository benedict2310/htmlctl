package main

import (
	"os"

	"github.com/benedict2310/htmlctl/internal/cli"
)

var version = "dev"

func main() {
	if err := cli.NewRootCmd(version).Execute(); err != nil {
		os.Exit(1)
	}
}
