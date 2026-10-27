package main

import (
	"github.com/jsirianni/dayz_config/internal/deploycli"
	"os"
)

func main() { os.Exit(deploycli.Main(os.Args[1:])) }
