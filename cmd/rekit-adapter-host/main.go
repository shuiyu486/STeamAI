package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/shuiyu486/re-context-kits/internal/rekit/adapterhost"
)

func main() {
	var opt adapterhost.Options
	flag.StringVar(&opt.RepoRoot, "repo", "", "canonical re-context-kits repository root")
	flag.StringVar(&opt.CaseRoot, "target", "", "attached case root")
	flag.StringVar(&opt.Pack, "pack", "", "attached pack")
	flag.StringVar(&opt.GateEventID, "gate-event-id", "", "authorized gate event id")
	flag.StringVar(&opt.ExpectedDispatchSHA256, "expected-dispatch-sha256", "", "immutable pre-execution dispatch sha256")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "rekit-adapter-host does not accept positional arguments")
		os.Exit(2)
	}
	result, err := adapterhost.Run(opt)
	data, marshalErr := json.MarshalIndent(result, "", "  ")
	if marshalErr == nil {
		fmt.Println(string(data))
	} else if err == nil {
		err = marshalErr
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
