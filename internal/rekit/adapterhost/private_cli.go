package adapterhost

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
)

const (
	privateAuthorizedVMPIDAFlag = "-run-authorized-vmp-ida-index-inspector"
	privateChildVMPIDAFlag      = "-child-vmp-ida-index-inspector"
)

func IsEmbeddedPrivateInvocation(args []string) bool {
	for _, arg := range args {
		name, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(arg)), "=")
		if name == privateAuthorizedVMPIDAFlag || name == privateChildVMPIDAFlag {
			return true
		}
	}
	return false
}

func RunEmbeddedPrivate(args []string, stdout, stderr io.Writer) (bool, int) {
	if !IsEmbeddedPrivateInvocation(args) {
		return false, 0
	}
	flags := flag.NewFlagSet("steamai-private-adapter", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {}
	var common Options
	var authorized AuthorizedRunOptions
	var child VMPIDAIndexChildOptions
	var authorizedMode, childMode bool
	var controlBindingJSON string
	flags.StringVar(&common.RepoRoot, "repo", "", "")
	flags.StringVar(&common.CaseRoot, "target", "", "")
	flags.StringVar(&common.Pack, "pack", "", "")
	flags.StringVar(&common.GateEventID, "gate-event-id", "", "")
	flags.StringVar(&common.ExpectedDispatchSHA256, "expected-dispatch-sha256", "", "")
	flags.BoolVar(&authorizedMode, strings.TrimPrefix(privateAuthorizedVMPIDAFlag, "-"), false, "")
	flags.StringVar(&authorized.ExecutionReportPath, "execution-report-path", "", "")
	flags.StringVar(&authorized.AdapterSession, "adapter-session", "", "")
	flags.StringVar(&authorized.Actor, "actor", "", "")
	flags.BoolVar(&authorized.DeferSuccessfulTaskBinding, "defer-successful-task-binding", false, "")
	flags.StringVar(&controlBindingJSON, "execution-control-binding-json", "", "")
	flags.BoolVar(&childMode, strings.TrimPrefix(privateChildVMPIDAFlag, "-"), false, "")
	flags.StringVar(&child.Executor, "executor", "", "")
	flags.IntVar(&child.ExpectedExecutorGeneration, "expected-executor-generation", 0, "")
	flags.StringVar(&child.RequestPath, "child-request-path", "", "")
	if err := flags.Parse(args); err != nil {
		return true, 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "private STeamAI adapter invocation does not accept positional arguments")
		return true, 2
	}
	if authorizedMode == childMode {
		fmt.Fprintln(stderr, "private STeamAI adapter invocation requires exactly one parent or child mode")
		return true, 2
	}

	var result any
	var err error
	if childMode {
		if strings.TrimSpace(authorized.Actor) != "" || strings.TrimSpace(authorized.ExecutionReportPath) != "" ||
			authorized.DeferSuccessfulTaskBinding || strings.TrimSpace(controlBindingJSON) != "" {
			fmt.Fprintln(stderr, "private VMP IDA child flags cannot be combined with parent-only flags")
			return true, 2
		}
		child.RepoRoot = common.RepoRoot
		child.CaseRoot = common.CaseRoot
		child.Pack = common.Pack
		child.GateEventID = common.GateEventID
		child.ExpectedDispatchSHA256 = common.ExpectedDispatchSHA256
		child.AdapterSession = authorized.AdapterSession
		result, err = RunVMPIDAIndexChild(child)
	} else {
		if strings.TrimSpace(common.ExpectedDispatchSHA256) != "" || strings.TrimSpace(child.RequestPath) != "" ||
			strings.TrimSpace(child.Executor) != "" || child.ExpectedExecutorGeneration != 0 {
			fmt.Fprintln(stderr, "authorized VMP IDA parent flags cannot be combined with immutable-dispatch child flags")
			return true, 2
		}
		if strings.TrimSpace(controlBindingJSON) != "" {
			var binding executioncontrol.Binding
			decoder := json.NewDecoder(bytes.NewBufferString(controlBindingJSON))
			decoder.DisallowUnknownFields()
			if decodeErr := decoder.Decode(&binding); decodeErr != nil {
				fmt.Fprintln(stderr, "decode execution control binding:", decodeErr)
				return true, 2
			}
			var trailing any
			if decodeErr := decoder.Decode(&trailing); decodeErr != io.EOF {
				fmt.Fprintln(stderr, "execution control binding must contain exactly one JSON object")
				return true, 2
			}
			if validateErr := executioncontrol.ValidateBinding(binding); validateErr != nil {
				fmt.Fprintln(stderr, validateErr)
				return true, 2
			}
			authorized.ExecutionControlBinding = &binding
		}
		authorized.RepoRoot = common.RepoRoot
		authorized.CaseRoot = common.CaseRoot
		authorized.Pack = common.Pack
		authorized.GateEventID = common.GateEventID
		result, err = RunAuthorizedGate(authorized)
	}
	if result != nil {
		data, marshalErr := json.MarshalIndent(result, "", "  ")
		if marshalErr == nil {
			fmt.Fprintln(stdout, string(data))
		} else if err == nil {
			err = marshalErr
		}
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return true, 1
	}
	return true, 0
}
