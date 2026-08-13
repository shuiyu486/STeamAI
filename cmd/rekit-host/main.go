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
	internalSupervisor := flag.String("internal-supervisor", "", "internal exact host-run supervision spec")
	internalSupervisorSHA256 := flag.String("internal-supervisor-sha256", "", "internal exact host-run supervision spec sha256")
	daily := flag.Bool("daily", false, "run the natural-language daily front door; implied by -goal or -correction")
	liveAcceptance := flag.Bool("live-acceptance", false, "run the explicit default or allowlisted cross-pack real-Claude acceptance gate")
	liveSupervisionAcceptance := flag.Bool("live-supervision-acceptance", false, "run the explicit RH-04 real-Claude process-start recovery gate")
	livePackMemoryAcceptance := flag.Bool("live-pack-memory-acceptance", false, "run the explicit RH-07 cross-case pack-memory real-Claude acceptance gate")
	liveSoakAcceptance := flag.Bool("live-soak-acceptance", false, "run the explicit RH-09 Windows three-task real-Claude soak and recovery gate")
	internalPackMemoryAcceptance := flag.String("internal-pack-memory-live-acceptance", "", "internal RH-07 isolated child spec")
	internalPackMemoryAcceptanceSHA256 := flag.String("internal-pack-memory-live-acceptance-sha256", "", "internal RH-07 isolated child spec sha256")
	flag.StringVar(&opt.Target, "target", "", "fresh or attached case root")
	flag.StringVar(&opt.Pack, "pack", "", "optional attached pack override")
	flag.StringVar(&opt.SelectedLane, "lane", "", "exact current lane selected for this invocation")
	flag.StringVar(&opt.ExpectedCurrentDriverRequestSHA256, "expected-current-driver-request-sha256", "", "exact fresh missionControlRunbook current driver request sha256")
	flag.StringVar(&opt.Actor, "actor", "rekit-claude-host", "durable host actor")
	flag.StringVar(&opt.ClaudePath, "claude", "", "Claude Code executable path")
	flag.StringVar(&opt.Model, "model", "", "optional Claude model")
	flag.DurationVar(&opt.Timeout, "timeout", 30*time.Minute, "per-session wall-clock timeout")
	flag.IntVar(&opt.MaxAttempts, "max-attempts", 3, "maximum real Claude launches")
	flag.StringVar(&liveOpt.Goal, "goal", "", "natural-language goal for daily or explicit live acceptance mode")
	flag.StringVar(&liveOpt.Correction, "correction", "", "human correction for daily or explicit live acceptance mode")
	flag.BoolVar(&liveOpt.KeepCase, "keep-case", false, "retain the fresh acceptance case after the gate")
	flag.StringVar(&liveOpt.ReceiptPath, "receipt", "", "machine-readable acceptance receipt path; required by -live-soak-acceptance")
	flag.StringVar(&liveOpt.AdapterPath, "adapter", "", "built rekit-adapter-host executable required by vmp-re live acceptance")
	flag.Parse()

	if err := validateAdapterFlag(*liveAcceptance, opt.Pack, liveOpt.AdapterPath); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	ordinaryHostRequested := !publicModeRequested(*daily, *liveAcceptance, *liveSupervisionAcceptance, *livePackMemoryAcceptance, *liveSoakAcceptance) &&
		strings.TrimSpace(liveOpt.Goal) == "" && strings.TrimSpace(liveOpt.Correction) == "" &&
		strings.TrimSpace(*internalSupervisor) == "" && strings.TrimSpace(*internalSupervisorSHA256) == "" &&
		strings.TrimSpace(*internalPackMemoryAcceptance) == "" && strings.TrimSpace(*internalPackMemoryAcceptanceSHA256) == ""
	if err := validateExpectedCurrentDriverRequestFlag(opt.ExpectedCurrentDriverRequestSHA256, ordinaryHostRequested); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	if strings.TrimSpace(*internalSupervisor) != "" || strings.TrimSpace(*internalSupervisorSHA256) != "" {
		if strings.TrimSpace(*internalSupervisor) == "" || strings.TrimSpace(*internalSupervisorSHA256) == "" || strings.TrimSpace(*internalPackMemoryAcceptance) != "" || strings.TrimSpace(*internalPackMemoryAcceptanceSHA256) != "" || publicModeRequested(*daily, *liveAcceptance, *liveSupervisionAcceptance, *livePackMemoryAcceptance, *liveSoakAcceptance) || flag.NArg() != 0 {
			fmt.Fprintln(os.Stderr, "internal supervisor requires an exact spec path and sha256")
			os.Exit(2)
		}
		if err := sessionhost.RunSupervisorChild(context.Background(), *internalSupervisor, *internalSupervisorSHA256); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	if strings.TrimSpace(*internalPackMemoryAcceptance) != "" || strings.TrimSpace(*internalPackMemoryAcceptanceSHA256) != "" {
		if strings.TrimSpace(*internalPackMemoryAcceptance) == "" || strings.TrimSpace(*internalPackMemoryAcceptanceSHA256) == "" || publicModeRequested(*daily, *liveAcceptance, *liveSupervisionAcceptance, *livePackMemoryAcceptance, *liveSoakAcceptance) || flag.NArg() != 0 {
			fmt.Fprintln(os.Stderr, "internal pack-memory acceptance requires an exact spec path and sha256")
			os.Exit(2)
		}
		result, err := sessionhost.RunPackMemoryLiveAcceptanceChild(context.Background(), *internalPackMemoryAcceptance, *internalPackMemoryAcceptanceSHA256)
		printResult(result, err)
		return
	}

	if *livePackMemoryAcceptance {
		if *daily || *liveAcceptance || *liveSupervisionAcceptance || *liveSoakAcceptance {
			fmt.Fprintln(os.Stderr, "-live-pack-memory-acceptance, -live-supervision-acceptance, -live-soak-acceptance, -live-acceptance, and -daily are mutually exclusive")
			os.Exit(2)
		}
		if strings.TrimSpace(opt.Pack) != "" && !strings.EqualFold(strings.TrimSpace(opt.Pack), "vmp-re") {
			fmt.Fprintln(os.Stderr, "pack-memory live acceptance always uses the default vmp-re pack")
			os.Exit(2)
		}
		result, err := sessionhost.RunPackMemoryLiveAcceptance(context.Background(), sessionhost.PackMemoryLiveAcceptanceOptions{
			Goal: liveOpt.Goal, ClaudePath: opt.ClaudePath, Model: opt.Model, Actor: opt.Actor,
			Timeout: opt.Timeout, MaxAttempts: opt.MaxAttempts, KeepCase: liveOpt.KeepCase, ReceiptPath: liveOpt.ReceiptPath,
		})
		result, err = publishPackMemoryLiveAcceptanceReceipt(liveOpt.ReceiptPath, result, err)
		printResult(result, err)
		return
	}

	if *liveSupervisionAcceptance {
		if *daily || *liveAcceptance || *liveSoakAcceptance {
			fmt.Fprintln(os.Stderr, "-live-supervision-acceptance, -live-soak-acceptance, -live-acceptance, and -daily are mutually exclusive")
			os.Exit(2)
		}
		result, err := sessionhost.RunLiveSupervisionAcceptance(context.Background(), sessionhost.LiveSupervisionAcceptanceOptions{
			CaseRoot: opt.Target, Goal: liveOpt.Goal, Model: opt.Model, Actor: opt.Actor,
			Timeout: opt.Timeout, MaxAttempts: opt.MaxAttempts, KeepCase: liveOpt.KeepCase, ReceiptPath: liveOpt.ReceiptPath,
		})
		if strings.TrimSpace(liveOpt.ReceiptPath) != "" {
			result.ReceiptPublication = "published"
			if writeErr := sessionhost.WriteLiveSupervisionAcceptanceReceipt(liveOpt.ReceiptPath, result); writeErr != nil {
				result.Passed = false
				result.ReceiptPublication = "failed"
				result.ReceiptError = writeErr.Error()
				err = errorsJoin(err, writeErr)
			}
		}
		printResult(result, err)
		return
	}

	if *liveAcceptance {
		if *daily || *liveSoakAcceptance {
			fmt.Fprintln(os.Stderr, "-live-acceptance, -live-soak-acceptance, and -daily are mutually exclusive")
			os.Exit(2)
		}
		liveOpt.CaseRoot = opt.Target
		liveOpt.Pack = opt.Pack
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

	if *liveSoakAcceptance {
		if *daily {
			fmt.Fprintln(os.Stderr, "-live-soak-acceptance and -daily are mutually exclusive")
			os.Exit(2)
		}
		if strings.TrimSpace(opt.Target) != "" || strings.TrimSpace(opt.Pack) != "" || strings.TrimSpace(opt.ClaudePath) != "" || liveOpt.KeepCase {
			fmt.Fprintln(os.Stderr, "live soak acceptance owns disposable cases and canonical Claude discovery; omit -target, -pack, -claude, and -keep-case")
			os.Exit(2)
		}
		result, err := sessionhost.RunLiveSoakAcceptance(context.Background(), sessionhost.LiveSoakAcceptanceOptions{
			Goal: liveOpt.Goal, Correction: liveOpt.Correction, Model: opt.Model, Actor: opt.Actor,
			Timeout: opt.Timeout, MaxAttempts: opt.MaxAttempts, ReceiptPath: liveOpt.ReceiptPath,
		})
		result, err = publishLiveSoakAcceptanceReceipt(liveOpt.ReceiptPath, result, err)
		printResult(result, err)
		return
	}

	if *daily || strings.TrimSpace(liveOpt.Goal) != "" || strings.TrimSpace(liveOpt.Correction) != "" {
		if strings.TrimSpace(opt.ExpectedCurrentDriverRequestSHA256) != "" {
			fmt.Fprintln(os.Stderr, "-expected-current-driver-request-sha256 is supported only by the ordinary rekit-host mode")
			os.Exit(2)
		}
		if strings.TrimSpace(liveOpt.ReceiptPath) != "" || liveOpt.KeepCase {
			fmt.Fprintln(os.Stderr, "-receipt and -keep-case are supported only by -live-acceptance")
			os.Exit(2)
		}
		if strings.TrimSpace(opt.Pack) != "" {
			fmt.Fprintln(os.Stderr, "daily front door derives pack from onboarding or attached case metadata; omit -pack")
			os.Exit(2)
		}
		result, err := sessionhost.RunDaily(context.Background(), sessionhost.DailyOptions{
			Target:       opt.Target,
			Goal:         liveOpt.Goal,
			Correction:   liveOpt.Correction,
			SelectedLane: opt.SelectedLane,
			Actor:        opt.Actor,
			ClaudePath:   opt.ClaudePath,
			Model:        opt.Model,
			Timeout:      opt.Timeout,
			MaxAttempts:  opt.MaxAttempts,
		})
		printResult(result, err)
		return
	}

	if strings.TrimSpace(liveOpt.ReceiptPath) != "" || liveOpt.KeepCase {
		fmt.Fprintln(os.Stderr, "-receipt and -keep-case are supported only by -live-acceptance")
		os.Exit(2)
	}
	opt.RequireCurrentDriverRequest()
	result, err := sessionhost.Run(context.Background(), opt)
	printResult(result, err)
}

func publishPackMemoryLiveAcceptanceReceipt(path string, result sessionhost.PackMemoryLiveAcceptanceReceipt, err error) (sessionhost.PackMemoryLiveAcceptanceReceipt, error) {
	if strings.TrimSpace(path) == "" {
		return result, err
	}
	result.ReceiptPublication = "published"
	if writeErr := sessionhost.WritePackMemoryLiveAcceptanceReceipt(path, result); writeErr != nil {
		result.Passed = false
		result.ReceiptPublication = "failed"
		result.ReceiptError = writeErr.Error()
		err = errorsJoin(err, writeErr)
	}
	return result, err
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

func publishLiveSoakAcceptanceReceipt(path string, result sessionhost.LiveSoakAcceptanceReceipt, err error) (sessionhost.LiveSoakAcceptanceReceipt, error) {
	if strings.TrimSpace(path) == "" {
		return result, err
	}
	result.ReceiptPublication = "published"
	if writeErr := sessionhost.WriteLiveSoakAcceptanceReceipt(path, result); writeErr != nil {
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

func validateAdapterFlag(liveAcceptance bool, pack, adapterPath string) error {
	adapterPath = strings.TrimSpace(adapterPath)
	if adapterPath != "" && !liveAcceptance {
		return fmt.Errorf("-adapter is supported only by -live-acceptance")
	}
	pack = strings.ToLower(strings.TrimSpace(pack))
	if liveAcceptance && (pack == "" || pack == "vmp-re") && adapterPath == "" {
		return fmt.Errorf("vmp-re live acceptance requires -adapter with the built rekit-adapter-host executable")
	}
	return nil
}

func validateExpectedCurrentDriverRequestFlag(value string, ordinaryHost bool) error {
	if strings.TrimSpace(value) != "" && !ordinaryHost {
		return fmt.Errorf("-expected-current-driver-request-sha256 is supported only by the ordinary rekit-host mode")
	}
	return nil
}

func publicModeRequested(modes ...bool) bool {
	for _, mode := range modes {
		if mode {
			return true
		}
	}
	return false
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
