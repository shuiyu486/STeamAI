package main

import (
	"os"

	"github.com/shuiyu486/re-context-kits/internal/rekit/adapterhost"
	"github.com/shuiyu486/re-context-kits/internal/rekit/hostcmd"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if handled, code := adapterhost.RunEmbeddedPrivate(args, os.Stdout, os.Stderr); handled {
		return code
	}
	return hostcmd.Run(args, os.Stdout, os.Stderr)
}
