package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/adapterhost"
	"github.com/shuiyu486/re-context-kits/internal/rekit/cli"
	"github.com/shuiyu486/re-context-kits/internal/rekit/hostcmd"
	rekitruntime "github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

type invocationExecutableContext struct {
	recoveryOnly bool
	projectRoot  string
}

func run(args []string) int {
	mode, modeArgs, err := invocationMode(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	process, err := validateInvocationExecutable(mode, modeArgs)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	switch mode {
	case "adapter":
		handled, code := adapterhost.RunEmbeddedPrivate(modeArgs, os.Stdout, os.Stderr)
		if !handled {
			fmt.Fprintln(os.Stderr, "private STeamAI adapter mode was not recognized")
			return 2
		}
		return code
	case "host":
		if process.recoveryOnly {
			return hostcmd.RunProjectLocalRecovery(
				modeArgs,
				os.Stdout,
				os.Stderr,
				process.projectRoot,
			)
		}
		if process.projectRoot != "" {
			return hostcmd.RunProjectLocal(
				modeArgs,
				os.Stdout,
				os.Stderr,
				process.projectRoot,
			)
		}
		return hostcmd.Run(modeArgs, os.Stdout, os.Stderr)
	default:
		var runErr error
		switch {
		case process.recoveryOnly:
			runErr = cli.RunProjectLocalRecovery(
				modeArgs,
				os.Stdout,
				process.projectRoot,
			)
		case process.projectRoot != "":
			runErr = cli.RunProjectLocal(
				modeArgs,
				os.Stdout,
				process.projectRoot,
			)
		default:
			runErr = cli.Run(modeArgs, os.Stdout)
		}
		if runErr != nil {
			fmt.Fprintln(os.Stderr, runErr)
			return 1
		}
		return 0
	}
}

func validateInvocationExecutable(
	mode string,
	args []string,
) (invocationExecutableContext, error) {
	projectRoot, projectLocal, err :=
		rekitruntime.RunningExecutableProjectRoot()
	if err != nil {
		return invocationExecutableContext{}, err
	}
	ordinaryErr := rekitruntime.ValidateRunningExecutable()
	if !projectLocal {
		if ordinaryErr != nil {
			return invocationExecutableContext{}, ordinaryErr
		}
		return invocationExecutableContext{}, nil
	}

	target, err := invocationProjectLocalTarget(mode, args)
	if err != nil {
		return invocationExecutableContext{}, err
	}
	if _, err := rekitruntime.ResolveProjectLocalTarget(
		projectRoot,
		target,
		"",
	); err != nil {
		return invocationExecutableContext{}, err
	}
	if mode == "runtime" {
		opt, err := cli.Parse(args)
		if err != nil {
			return invocationExecutableContext{}, err
		}
		if cli.CurrentSyncMaintenanceRequested(opt) {
			return invocationExecutableContext{}, fmt.Errorf(
				"project-local STeamAI executable cannot execute current project maintenance; use the exact external maintenance executable",
			)
		}
	}
	if ordinaryErr == nil {
		return invocationExecutableContext{projectRoot: projectRoot}, nil
	}

	recoveryTarget, err := invocationRecoveryTarget(mode, args)
	if err != nil {
		return invocationExecutableContext{}, fmt.Errorf(
			"%v; recovery-aware validation is unavailable: %w",
			ordinaryErr,
			err,
		)
	}
	recoveryTarget, err = rekitruntime.ResolveProjectLocalTarget(
		projectRoot,
		recoveryTarget,
		"",
	)
	if err != nil {
		return invocationExecutableContext{}, err
	}
	if err := rekitruntime.ValidateRunningExecutableForRecovery(
		recoveryTarget,
	); err != nil {
		return invocationExecutableContext{}, err
	}
	return invocationExecutableContext{
		recoveryOnly: true,
		projectRoot:  projectRoot,
	}, nil
}

func invocationProjectLocalTarget(mode string, args []string) (string, error) {
	switch mode {
	case "runtime":
		opt, err := cli.Parse(args)
		if err != nil {
			return "", err
		}
		return opt.Target, nil
	case "host":
		if hostcmd.IsInternalInvocation(args) {
			return "", nil
		}
		return hostInvocationTarget(args)
	case "adapter":
		return hostInvocationTarget(args)
	default:
		return "", fmt.Errorf("unsupported STeamAI invocation mode: %s", mode)
	}
}

func invocationRecoveryTarget(mode string, args []string) (string, error) {
	switch mode {
	case "runtime":
		opt, err := cli.Parse(args)
		if err != nil {
			return "", err
		}
		return opt.Target, nil
	case "host":
		if !hostRecoveryDailyRequested(args) {
			return "", fmt.Errorf(
				"project-local recovery permits only the daily host front door",
			)
		}
		return hostRecoveryTarget(args)
	default:
		return "", fmt.Errorf("unsupported STeamAI invocation mode: %s", mode)
	}
}

func hostRecoveryDailyRequested(args []string) bool {
	for _, argument := range args {
		name, _, _ := strings.Cut(
			strings.ToLower(strings.TrimSpace(argument)),
			"=",
		)
		switch name {
		case "-daily", "--daily", "-goal", "--goal", "-correction", "--correction":
			return true
		}
	}
	return false
}

func hostRecoveryTarget(args []string) (string, error) {
	return hostInvocationTarget(args)
}

func hostInvocationTarget(args []string) (string, error) {
	target := ""
	seen := false
	for index := 0; index < len(args); index++ {
		argument := strings.TrimSpace(args[index])
		name, value, assigned := strings.Cut(argument, "=")
		name = strings.ToLower(name)
		if name != "-target" && name != "--target" {
			continue
		}
		if seen {
			return "", fmt.Errorf(
				"project-local host invocation received multiple -target values",
			)
		}
		seen = true
		if !assigned {
			index++
			if index >= len(args) {
				return "", fmt.Errorf("missing value for -target")
			}
			value = args[index]
		}
		target = strings.TrimSpace(value)
		if target == "" {
			return "", fmt.Errorf("missing value for -target")
		}
	}
	return target, nil
}

func invocationMode(args []string) (string, []string, error) {
	if hostcmd.IsInternalInvocation(args) {
		return "host", args, nil
	}
	if adapterhost.IsEmbeddedPrivateInvocation(args) {
		return "adapter", args, nil
	}
	if len(args) == 0 || strings.HasPrefix(strings.TrimSpace(args[0]), "-") {
		return "runtime", args, nil
	}
	mode := strings.ToLower(strings.TrimSpace(args[0]))
	switch mode {
	case "runtime", "host":
		return mode, args[1:], nil
	default:
		return "", nil, fmt.Errorf("unknown steamai mode %q; use runtime or host", args[0])
	}
}
