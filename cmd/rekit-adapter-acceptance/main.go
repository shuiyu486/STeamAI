package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/shuiyu486/re-context-kits/internal/rekit/adapterhost"
)

func main() {
	var opt adapterhost.LiveAcceptanceOptions
	flag.StringVar(&opt.RepoRoot, "repo", "", "canonical re-context-kits repository root")
	flag.StringVar(&opt.AdapterPath, "adapter", "", "built rekit-adapter-host executable")
	flag.StringVar(&opt.ReceiptPath, "receipt", "", "repository-external acceptance receipt path")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "rekit-adapter-acceptance does not accept positional arguments")
		os.Exit(2)
	}
	receipt, err := adapterhost.RunLiveAcceptance(opt)
	if writeErr := adapterhost.WriteLiveAcceptanceReceipt(opt.ReceiptPath, receipt); writeErr != nil {
		if err == nil {
			err = writeErr
		} else {
			err = fmt.Errorf("%v; write receipt: %w", err, writeErr)
		}
	}
	data, marshalErr := json.MarshalIndent(receipt, "", "  ")
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
