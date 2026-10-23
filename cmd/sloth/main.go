package main

import (
	"os"

	"github.com/ansonnchan/sloth/internal/buildinfo"
	"github.com/ansonnchan/sloth/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, buildinfo.Current()))
}
