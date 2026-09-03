package main

import (
	"os"

	"github.com/shuiyu486/STeamAI/internal/steamai"
)

var version = "dev"

func main() {
	os.Exit(steamai.Main(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, version))
}
