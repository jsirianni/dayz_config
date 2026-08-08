package main

import (
	"os"

	"dzcli/internal/commanddocs"
)

var exit = os.Exit

func main() {
	exit(commanddocs.Main(os.Args[1:], os.Stdout, os.Stderr))
}
