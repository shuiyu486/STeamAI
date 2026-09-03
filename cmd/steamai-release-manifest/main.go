package main

import (
	"fmt"
	"os"

	"github.com/shuiyu486/STeamAI/internal/steamai"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: steamai-release-manifest <manifest> <version>")
		os.Exit(2)
	}
	file, err := os.Open(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer file.Close()
	if _, err := steamai.ParseReleaseManifest(file, os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
