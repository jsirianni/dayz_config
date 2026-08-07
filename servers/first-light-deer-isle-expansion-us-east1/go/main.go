package main

import (
	"os"

	"dzcli/cli"
)

func main() {
	cli.Main(os.Args[1:], os.Stdout, os.Stderr, os.Exit)
}
