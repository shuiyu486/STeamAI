package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/sessionhost"
)

func main() {
	var opt sessionhost.Options
	var liveOpt sessionhost.LiveAcceptanceOptions
	liveAcceptance := flag.Bool("live-acceptance", false, "run the explicit default vmp-re real-Claude acceptance gate")
	flag.StringVar(&opt.Target, "target", "", "attached case root")
	flag.StringVar(&opt.Pack, "pack", "", "optional attached pack override")
	flag.StringVar(&opt.Actor, "actor", "rekit-claude-host", "durable host actor")
	flag.StringVar(&opt.ClaudePath, "claude", "", "Claude Code executable path")
	flag.StringVar(&opt.Model, "model", "", "optional Claude model")
	flag.DurationVar(&opt.Timeout, "timeout", 30*time.Minute, "per-session wall-clock timeout")
	flag.IntVar(&opt.MaxAttempts, "max-attempts", 3, "maximum real Claude launches")
	flag.StringVar(&liveOpt.Goal, "goal", "", "natural-language goal for explicit live acceptance")
	flag.StringVar(&liveOpt.Correction, "correction", "", "human correction applied before replacement member launch")
	flag.BoolVar(&liveOpt.KeepCase, "keep-case", false, "retain the fresh acceptance case after the gate")
	flag.StringVar(&liveOpt.ReceiptPath, "receipt", "", "optional machine-readable acceptance receipt path")
	flag.Parse()

	if *liveAcceptance {
		if opt.Pack != "" && opt.Pack != "vmp-re" {
			fmt.Fprintln(os.Stderr, "live acceptance always uses the default vmp-re pack")
			os.Exit(2)
		}
		liveOpt.CaseRoot = opt.Target
		liveOpt.ClaudePath = opt.ClaudePath
		liveOpt.Model = opt.Model
		liveOpt.Actor = opt.Actor
		liveOpt.Timeout = opt.Timeout
		liveOpt.MaxAttempts = opt.MaxAttempts
		result, err := sessionhost.RunLiveAcceptance(context.Background(), liveOpt)
		if strings.TrimSpace(liveOpt.ReceiptPath) != "" {
			if writeErr := sessionhost.WriteLiveAcceptanceReceipt(liveOpt.ReceiptPath, result); writeErr != nil {
				err = errorsJoin(err, writeErr)
			}
		}
		printResult(result, err)
		return
	}

	result, err := sessionhost.Run(context.Background(), opt)
	printResult(result, err)
}

func printResult(result any, err error) {
	data, marshalErr := json.MarshalIndent(result, "", "  ")
	if marshalErr == nil {
		fmt.Println(string(data))
	} else {
		err = errorsJoin(err, marshalErr)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func errorsJoin(a, b error) error {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	return fmt.Errorf("%v; %w", a, b)
}
