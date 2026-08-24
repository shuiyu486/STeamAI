package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// PublicInvocation reports whether args use the small user-facing STeamAI
// command surface. The deterministic command parser remains the owner of all
// maintenance and typed mutation flags.
func PublicInvocation(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "help", "status", "continue", "-h", "--help":
		return true
	default:
		return false
	}
}

// PublicInvocationTarget extracts the optional public target without parsing
// any internal runtime flags. An omitted target is resolved by the caller's
// project-local executable or current working directory.
func PublicInvocationTarget(args []string) (string, error) {
	options, err := parsePublicOptions(args)
	if err != nil {
		return "", wrapPublicUsageError(err)
	}
	return options.Target, nil
}

// RunPublic executes help, status, and the safe continue preview. It is a
// presentation layer over the existing Go-owned command handlers: it does
// not introduce a second durable state machine or a second public projection.
func RunPublic(args []string, stdout io.Writer, projectRoot string) error {
	return runPublic(args, stdout, projectRoot, false)
}

// RunPublicRecovery preserves the same public presentation while limiting a
// stale project-local executable to the existing current-sync recovery route.
func RunPublicRecovery(args []string, stdout io.Writer, projectRoot string) error {
	return runPublic(args, stdout, projectRoot, true)
}

func runPublic(args []string, stdout io.Writer, projectRoot string, recoveryOnly bool) error {
	return runPublicWithExecutor(args, stdout, projectRoot, recoveryOnly, executePublicCommand)
}

type publicOutputMode string

const (
	publicOutputInteraction publicOutputMode = "interaction"
	publicOutputDiagnostics publicOutputMode = "diagnostics"
)

type publicOptions struct {
	Command string
	Target  string
	Lane    string
	Mode    publicOutputMode
}

type publicExecutor func(publicOptions, string, bool) (json.RawMessage, error)

func runPublicWithExecutor(
	args []string,
	stdout io.Writer,
	projectRoot string,
	recoveryOnly bool,
	execute publicExecutor,
) error {
	if stdout == nil {
		stdout = io.Discard
	}
	options, err := parsePublicOptions(args)
	if err != nil {
		return wrapPublicUsageError(err)
	}
	if options.Command == "help" {
		return writePublicHelp(stdout)
	}
	if execute == nil {
		return fmt.Errorf("public executor is unavailable")
	}
	raw, err := execute(options, projectRoot, recoveryOnly)
	if err != nil {
		return err
	}
	if options.Mode == publicOutputDiagnostics {
		return writePublicDiagnostics(stdout, raw)
	}
	return writePublicInteraction(stdout, options.Command, raw)
}

func executePublicCommand(options publicOptions, projectRoot string, recoveryOnly bool) (json.RawMessage, error) {
	var raw bytes.Buffer
	commandArgs := []string{"-Command", options.Command}
	if options.Target != "" {
		commandArgs = append(commandArgs, "-Target", options.Target)
	}
	if options.Lane != "" {
		commandArgs = append(commandArgs, "-Lane", options.Lane)
	}
	if options.Command == "continue" {
		commandArgs = append(commandArgs, "-WhatIf")
	}
	format := "json"
	if options.Command == "status" && options.Mode == publicOutputInteraction {
		format = "compact-json"
	}
	commandArgs = append(commandArgs, "-Format", format)
	var err error
	switch {
	case recoveryOnly:
		err = RunProjectLocalRecovery(commandArgs, &raw, projectRoot)
	case projectRoot != "":
		err = RunProjectLocal(commandArgs, &raw, projectRoot)
	default:
		err = Run(commandArgs, &raw)
	}
	if err != nil {
		return nil, err
	}
	return append(json.RawMessage(nil), raw.Bytes()...), nil
}

func parsePublicOptions(args []string) (publicOptions, error) {
	if len(args) == 0 {
		return publicOptions{Command: "help", Mode: publicOutputInteraction}, nil
	}
	command := strings.ToLower(strings.TrimSpace(args[0]))
	if command == "-h" || command == "--help" {
		command = "help"
	}
	switch command {
	case "help", "status", "continue":
	default:
		return publicOptions{}, fmt.Errorf("unknown public STeamAI command %q; use steamai help", args[0])
	}
	options := publicOptions{Command: command, Mode: publicOutputInteraction}
	for index := 1; index < len(args); index++ {
		argument := strings.TrimSpace(args[index])
		name, value, assigned := strings.Cut(argument, "=")
		switch strings.ToLower(name) {
		case "--help", "-h":
			if command != "help" {
				return publicOptions{Command: "help", Mode: publicOutputInteraction}, nil
			}
		case "--diagnostics":
			options.Mode = publicOutputDiagnostics
		case "--target":
			if !assigned {
				index++
				if index >= len(args) {
					return publicOptions{}, fmt.Errorf("public %s requires a value for --target", command)
				}
				value = args[index]
			}
			if options.Target != "" {
				return publicOptions{}, fmt.Errorf("public %s received multiple --target values", command)
			}
			options.Target = strings.TrimSpace(value)
			if options.Target == "" {
				return publicOptions{}, fmt.Errorf("public %s requires a non-empty --target", command)
			}
		case "--lane":
			if command != "continue" {
				return publicOptions{}, fmt.Errorf("--lane is supported only by public continue")
			}
			if !assigned {
				index++
				if index >= len(args) {
					return publicOptions{}, fmt.Errorf("public continue requires a value for --lane")
				}
				value = args[index]
			}
			if options.Lane != "" {
				return publicOptions{}, fmt.Errorf("public continue received multiple --lane values")
			}
			options.Lane = strings.TrimSpace(value)
			if options.Lane == "" {
				return publicOptions{}, fmt.Errorf("public continue requires a non-empty --lane")
			}
		case "--format":
			if !assigned {
				index++
				if index >= len(args) {
					return publicOptions{}, fmt.Errorf("public %s requires a value for --format", command)
				}
				value = args[index]
			}
			if !strings.EqualFold(strings.TrimSpace(value), "json") {
				return publicOptions{}, fmt.Errorf("public %s supports only --format json for diagnostics", command)
			}
			options.Mode = publicOutputDiagnostics
		default:
			return publicOptions{}, fmt.Errorf("public %s does not accept %s; use --diagnostics for typed JSON", command, argument)
		}
	}
	return options, nil
}

func writePublicHelp(out io.Writer) error {
	_, err := fmt.Fprint(out,
		"STeamAI public commands:\n"+
			"  steamai status [--diagnostics] [--target PATH]\n"+
			"  steamai continue [--lane SELECTOR] [--diagnostics] [--target PATH]\n"+
			"  steamai help\n"+
			"\n"+
			"status is read-only. continue creates a fresh preview only; it does not Apply, launch Claude, or run heavy tools.\n"+
			"Use --diagnostics when a typed JSON response is needed. Lane selection accepts a human selector and is resolved by the runtime.\n",
	)
	return err
}

func writePublicDiagnostics(out io.Writer, data []byte) error {
	var formatted bytes.Buffer
	if err := json.Indent(&formatted, bytes.TrimSpace(data), "", "  "); err != nil {
		return err
	}
	formatted.WriteByte('\n')
	_, err := out.Write(formatted.Bytes())
	return err
}
