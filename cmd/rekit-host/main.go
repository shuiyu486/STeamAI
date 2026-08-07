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
	daily := flag.Bool("daily", false, "run the natural-language daily front door; implied by -goal or -correction")
	liveAcceptance := flag.Bool("live-acceptance", false, "run the explicit default vmp-re real-Claude acceptance gate")
	flag.StringVar(&opt.Target, "target", "", "fresh or attached case root")
	flag.StringVar(&opt.Pack, "pack", "", "optional attached pack override")
	flag.StringVar(&opt.Actor, "actor", "rekit-claude-host", "durable host actor")
	flag.StringVar(&opt.ClaudePath, "claude", "", "Claude Code executable path")
	flag.StringVar(&opt.Model, "model", "", "optional Claude model")
	flag.DurationVar(&opt.Timeout, "timeout", 30*time.Minute, "per-session wall-clock timeout")
	flag.IntVar(&opt.MaxAttempts, "max-attempts", 3, "maximum real Claude launches")
	flag.StringVar(&liveOpt.Goal, "goal", "", "natural-language goal for daily or explicit live acceptance mode")
	flag.StringVar(&liveOpt.Correction, "correction", "", "human correction for daily or explicit live acceptance mode")
	flag.BoolVar(&liveOpt.KeepCase, "keep-case", false, "retain the fresh acceptance case after the gate")
	flag.StringVar(&liveOpt.ReceiptPath, "receipt", "", "optional machine-readable acceptance receipt path")
	flag.Parse()

	if *liveAcceptance {
		if *daily {
			fmt.Fprintln(os.Stderr, "-live-acceptance and -daily are mutually exclusive")
			os.Exit(2)
		}
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
		result, err = publishLiveAcceptanceReceipt(liveOpt.ReceiptPath, result, err)
		printResult(result, err)
		return
	}

	if *daily || strings.TrimSpace(liveOpt.Goal) != "" || strings.TrimSpace(liveOpt.Correction) != "" {
		if strings.TrimSpace(liveOpt.ReceiptPath) != "" || liveOpt.KeepCase {
			fmt.Fprintln(os.Stderr, "-receipt and -keep-case are supported only by -live-acceptance")
			os.Exit(2)
		}
		if strings.TrimSpace(opt.Pack) != "" {
			fmt.Fprintln(os.Stderr, "daily front door derives pack from onboarding or attached case metadata; omit -pack")
			os.Exit(2)
		}
		result, err := sessionhost.RunDaily(context.Background(), sessionhost.DailyOptions{
			Target:      opt.Target,
			Goal:        liveOpt.Goal,
			Correction:  liveOpt.Correction,
			Actor:       opt.Actor,
			ClaudePath:  opt.ClaudePath,
			Model:       opt.Model,
			Timeout:     opt.Timeout,
			MaxAttempts: opt.MaxAttempts,
		})
		printResult(result, err)
		return
	}

	if strings.TrimSpace(liveOpt.ReceiptPath) != "" || liveOpt.KeepCase {
		fmt.Fprintln(os.Stderr, "-receipt and -keep-case are supported only by -live-acceptance")
		os.Exit(2)
	}
	result, err := sessionhost.Run(context.Background(), opt)
	printResult(result, err)
}

func publishLiveAcceptanceReceipt(path string, result sessionhost.LiveAcceptanceReceipt, err error) (sessionhost.LiveAcceptanceReceipt, error) {
	if strings.TrimSpace(path) == "" {
		return result, err
	}
	result.ReceiptPublication = "published"
	if writeErr := sessionhost.WriteLiveAcceptanceReceipt(path, result); writeErr != nil {
		result.Passed = false
		result.ReceiptPublication = "failed"
		result.ReceiptError = writeErr.Error()
		err = errorsJoin(err, writeErr)
	}
	return result, err
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
