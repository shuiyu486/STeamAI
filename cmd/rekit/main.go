package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/adapterhost"
	"github.com/shuiyu486/re-context-kits/internal/rekit/bootstrapcmd"
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
		if mode == "public" {
			return cli.RenderPublicFailure(
				modeArgs,
				err,
				cli.PublicFailureSourceExecutable,
				os.Stdout,
				os.Stderr,
			)
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	switch mode {
	case "bootstrap":
		executable, executableErr := os.Executable()
		if executableErr != nil {
			fmt.Fprintln(os.Stderr, executableErr)
			return 1
		}
		repoRoot, contextErr := bootstrapSourceRepoRoot(executable)
		if contextErr != nil {
			fmt.Fprintln(os.Stderr, contextErr)
			return 1
		}
		return bootstrapcmd.Run(modeArgs, os.Stdin, os.Stdout, os.Stderr, repoRoot, executable)
	case "public":
		var runErr error
		switch {
		case process.recoveryOnly:
			runErr = cli.RunPublicRecovery(
				modeArgs,
				os.Stdout,
				process.projectRoot,
			)
		case process.projectRoot != "":
			runErr = cli.RunPublic(
				modeArgs,
				os.Stdout,
				process.projectRoot,
			)
		default:
			runErr = cli.RunPublic(modeArgs, os.Stdout, "")
		}
		if runErr != nil {
			return cli.RenderPublicFailure(
				modeArgs,
				runErr,
				cli.PublicFailureSourceRuntime,
				os.Stdout,
				os.Stderr,
			)
		}
		return 0
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
		executable, executableErr := os.Executable()
		if executableErr != nil {
			fmt.Fprintln(os.Stderr, executableErr)
			return 1
		}
		if process.projectRoot != "" {
			return hostcmd.RunProjectLocalWithUnifiedExecutable(
				modeArgs,
				os.Stdout,
				os.Stderr,
				process.projectRoot,
				executable,
			)
		}
		return hostcmd.RunWithUnifiedExecutable(modeArgs, os.Stdout, os.Stderr, executable)
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
			executable, executableErr := os.Executable()
			if executableErr != nil {
				fmt.Fprintln(os.Stderr, executableErr)
				return 1
			}
			runErr = cli.RunWithUnifiedExecutable(modeArgs, os.Stdout, executable)
		}
		if runErr != nil {
			if code, handled := cli.RenderRuntimePlanFailure(
				modeArgs,
				runErr,
				os.Stdout,
				os.Stderr,
			); handled {
				return code
			}
			fmt.Fprintln(os.Stderr, runErr)
			return 1
		}
		return 0
	}
}

func bootstrapSourceRepoRoot(executable string) (string, error) {
	dir, err := filepath.Abs(filepath.Dir(strings.TrimSpace(executable)))
	if err != nil {
		return "", err
	}
	for {
		if info, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil && !info.IsDir() {
			if packs, packsErr := os.Stat(filepath.Join(dir, "packs")); packsErr == nil && packs.IsDir() {
				if cmd, cmdErr := os.Stat(filepath.Join(dir, "cmd", "rekit")); cmdErr == nil && cmd.IsDir() {
					moduleData, readErr := os.ReadFile(filepath.Join(dir, "go.mod"))
					if readErr == nil && strings.Contains(string(moduleData), "module github.com/shuiyu486/re-context-kits") {
						return dir, nil
					}
				}
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("bootstrap executable must remain inside its canonical source clone")
		}
		dir = parent
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
	if mode == "bootstrap" {
		return invocationExecutableContext{}, fmt.Errorf("project-local STeamAI executable cannot bootstrap another directory")
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
	case "bootstrap":
		return hostInvocationTarget(args)
	case "public":
		return cli.PublicInvocationTarget(args)
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
	case "public":
		return cli.PublicInvocationTarget(args)
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
	if len(args) == 0 {
		return "runtime", args, nil
	}
	first := strings.ToLower(strings.TrimSpace(args[0]))
	if cli.PublicInvocation(args) {
		if first == "-h" || first == "--help" {
			return "public", append([]string{"help"}, args[1:]...), nil
		}
		return "public", args, nil
	}
	if strings.HasPrefix(strings.TrimSpace(args[0]), "-") {
		return "runtime", args, nil
	}
	mode := first
	switch mode {
	case "bootstrap":
		if args[0] != "bootstrap" {
			return "", nil, fmt.Errorf("bootstrap mode token must be exactly %q", "bootstrap")
		}
		return mode, args[1:], nil
	case "runtime":
		return mode, args[1:], nil
	case "host":
		if args[0] != "host" {
			return "", nil, fmt.Errorf("host mode token must be exactly %q", "host")
		}
		return mode, args[1:], nil
	default:
		return "", nil, fmt.Errorf("unknown steamai mode %q; use bootstrap, runtime, or host", args[0])
	}
}
