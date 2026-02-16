package main

import (
	"os"

	"github.com/benedict2310/htmlctl/internal/cli"
)

var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		os.Exit(cli.ExitCode(err))
	}
}

func run(args []string) error {
	cmd := cli.NewRootCmd(version)
	cmd.SetArgs(args)
	return cmd.Execute()
}
