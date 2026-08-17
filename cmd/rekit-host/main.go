package main

import (
	"os"

	"github.com/shuiyu486/re-context-kits/internal/rekit/hostcmd"
)

func main() {
	os.Exit(hostcmd.Run(os.Args[1:], os.Stdout, os.Stderr))
}
