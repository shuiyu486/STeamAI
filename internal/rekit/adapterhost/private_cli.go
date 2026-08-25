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
	privateAuthorizedVMPIDAFlag      = "-run-authorized-vmp-ida-index-inspector"
	privateChildVMPIDAFlag           = "-child-vmp-ida-index-inspector"
	privateChildBinaryInventoryFlag  = "-child-binary-inventory"
	privateChildOpenAPIInventoryFlag = "-child-openapi-inventory"
	privateChildBoundedReplayFlag    = "-child-bounded-http-replay"
)

func IsEmbeddedPrivateInvocation(args []string) bool {
	for _, arg := range args {
		name, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(arg)), "=")
		switch name {
		case privateAuthorizedVMPIDAFlag,
			privateChildVMPIDAFlag,
			privateChildBinaryInventoryFlag,
			privateChildOpenAPIInventoryFlag,
			privateChildBoundedReplayFlag:
			return true
		}
	}
	return false
}

func decodePrivateExecutionControlBinding(value string) (*executioncontrol.Binding, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	var binding executioncontrol.Binding
	decoder := json.NewDecoder(bytes.NewBufferString(value))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&binding); err != nil {
		return nil, fmt.Errorf("decode execution control binding: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("execution control binding must contain exactly one JSON object")
	}
	if err := executioncontrol.ValidateBinding(binding); err != nil {
		return nil, err
	}
	return &binding, nil
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
	var binaryChild BinaryInventoryChildOptions
	var openAPIChild OpenAPIInventoryChildOptions
	var replayChild BoundedReplayChildOptions
	var authorizedMode, childMode, binaryChildMode, openAPIChildMode, replayChildMode bool
	var controlBindingJSON string
	var parentLaneLeaseHandle uint64
	var instructionIdentityJSON string
	flags.StringVar(&common.RepoRoot, "repo", "", "")
	flags.StringVar(&common.CaseRoot, "target", "", "")
	flags.StringVar(&common.Pack, "pack", "", "")
	flags.StringVar(&common.GateEventID, "gate-event-id", "", "")
	flags.StringVar(&common.ExpectedDispatchSHA256, "expected-dispatch-sha256", "", "")
	flags.BoolVar(&authorizedMode, strings.TrimPrefix(privateAuthorizedVMPIDAFlag, "-"), false, "")
	flags.StringVar(&authorized.ExecutionReportPath, "execution-report-path", "", "")
	flags.StringVar(&authorized.AdapterID, "adapter-id", "", "")
	flags.StringVar(&authorized.AdapterSession, "adapter-session", "", "")
	flags.StringVar(&authorized.Actor, "actor", "", "")
	flags.BoolVar(&authorized.DeferSuccessfulTaskBinding, "defer-successful-task-binding", false, "")
	flags.StringVar(&controlBindingJSON, "execution-control-binding-json", "", "")
	flags.Uint64Var(&parentLaneLeaseHandle, "parent-lane-lease-handle", 0, "")
	flags.StringVar(&instructionIdentityJSON, "instruction-identity-json", "", "")
	flags.BoolVar(&childMode, strings.TrimPrefix(privateChildVMPIDAFlag, "-"), false, "")
	flags.BoolVar(&binaryChildMode, strings.TrimPrefix(privateChildBinaryInventoryFlag, "-"), false, "")
	flags.BoolVar(&openAPIChildMode, strings.TrimPrefix(privateChildOpenAPIInventoryFlag, "-"), false, "")
	flags.BoolVar(&replayChildMode, strings.TrimPrefix(privateChildBoundedReplayFlag, "-"), false, "")
	flags.StringVar(&child.Executor, "executor", "", "")
	flags.IntVar(&child.ExpectedExecutorGeneration, "expected-executor-generation", 0, "")
	flags.StringVar(&child.RequestPath, "child-request-path", "", "")
	flags.StringVar(&binaryChild.SourcePath, "child-source-path", "", "")
	if err := flags.Parse(args); err != nil {
		return true, 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "private STeamAI adapter invocation does not accept positional arguments")
		return true, 2
	}
	parentHandle := uintptr(parentLaneLeaseHandle)
	if uint64(parentHandle) != parentLaneLeaseHandle {
		fmt.Fprintln(stderr, "private child inherited lane mutation lease handle overflows uintptr")
		return true, 2
	}
	modes := 0
	for _, enabled := range []bool{authorizedMode, childMode, binaryChildMode, openAPIChildMode, replayChildMode} {
		if enabled {
			modes++
		}
	}
	if modes != 1 {
		fmt.Fprintln(stderr, "private STeamAI adapter invocation requires exactly one parent or child mode")
		return true, 2
	}
	if (childMode || binaryChildMode || openAPIChildMode || replayChildMode) && parentHandle == 0 {
		fmt.Fprintln(stderr, "private STeamAI adapter child requires an inherited parent lane mutation lease handle")
		return true, 2
	}
	instructionIdentity, decodeErr := decodeAdapterInstructionIdentityJSON(instructionIdentityJSON)
	if decodeErr != nil {
		fmt.Fprintln(stderr, decodeErr)
		return true, 2
	}
	controlBinding, decodeErr := decodePrivateExecutionControlBinding(controlBindingJSON)
	if decodeErr != nil {
		fmt.Fprintln(stderr, decodeErr)
		return true, 2
	}

	var result any
	var err error
	switch {
	case childMode:
		if strings.TrimSpace(authorized.Actor) != "" || strings.TrimSpace(authorized.ExecutionReportPath) != "" ||
			strings.TrimSpace(authorized.AdapterID) != "" || authorized.DeferSuccessfulTaskBinding ||
			strings.TrimSpace(binaryChild.SourcePath) != "" {
			fmt.Fprintln(stderr, "private VMP IDA child flags cannot be combined with parent-only flags")
			return true, 2
		}
		child.RepoRoot = common.RepoRoot
		child.CaseRoot = common.CaseRoot
		child.Pack = common.Pack
		child.GateEventID = common.GateEventID
		child.ExpectedDispatchSHA256 = common.ExpectedDispatchSHA256
		child.AdapterSession = authorized.AdapterSession
		child.ExecutionControlBinding = executioncontrol.CloneBinding(controlBinding)
		child.ParentLaneLeaseHandle = parentHandle
		child.InstructionIdentity = cloneAdapterInstructionIdentity(instructionIdentity)
		result, err = RunVMPIDAIndexChild(child)
	case binaryChildMode:
		if strings.TrimSpace(authorized.Actor) != "" || strings.TrimSpace(authorized.ExecutionReportPath) != "" ||
			strings.TrimSpace(authorized.AdapterID) != "" || authorized.DeferSuccessfulTaskBinding ||
			strings.TrimSpace(child.RequestPath) != "" {
			fmt.Fprintln(stderr, "private binary inventory child flags cannot be combined with parent-only or VMP child flags")
			return true, 2
		}
		binaryChild.RepoRoot = common.RepoRoot
		binaryChild.CaseRoot = common.CaseRoot
		binaryChild.Pack = common.Pack
		binaryChild.GateEventID = common.GateEventID
		binaryChild.ExpectedDispatchSHA256 = common.ExpectedDispatchSHA256
		binaryChild.AdapterSession = authorized.AdapterSession
		binaryChild.Executor = child.Executor
		binaryChild.ExpectedExecutorGeneration = child.ExpectedExecutorGeneration
		binaryChild.ExecutionControlBinding = executioncontrol.CloneBinding(controlBinding)
		binaryChild.ParentLaneLeaseHandle = parentHandle
		binaryChild.InstructionIdentity = cloneAdapterInstructionIdentity(instructionIdentity)
		result, err = RunBinaryInventoryChild(binaryChild)
	case openAPIChildMode:
		if strings.TrimSpace(authorized.Actor) != "" || strings.TrimSpace(authorized.ExecutionReportPath) != "" ||
			strings.TrimSpace(authorized.AdapterID) != "" || authorized.DeferSuccessfulTaskBinding ||
			strings.TrimSpace(child.RequestPath) != "" {
			fmt.Fprintln(stderr, "private OpenAPI inventory child flags cannot be combined with parent-only or request-path child flags")
			return true, 2
		}
		openAPIChild.RepoRoot = common.RepoRoot
		openAPIChild.CaseRoot = common.CaseRoot
		openAPIChild.Pack = common.Pack
		openAPIChild.GateEventID = common.GateEventID
		openAPIChild.ExpectedDispatchSHA256 = common.ExpectedDispatchSHA256
		openAPIChild.AdapterSession = authorized.AdapterSession
		openAPIChild.Executor = child.Executor
		openAPIChild.ExpectedExecutorGeneration = child.ExpectedExecutorGeneration
		openAPIChild.SourcePath = binaryChild.SourcePath
		openAPIChild.ExecutionControlBinding = executioncontrol.CloneBinding(controlBinding)
		openAPIChild.ParentLaneLeaseHandle = parentHandle
		openAPIChild.InstructionIdentity = cloneAdapterInstructionIdentity(instructionIdentity)
		result, err = RunOpenAPIInventoryChild(openAPIChild)
	case replayChildMode:
		if strings.TrimSpace(authorized.Actor) != "" || strings.TrimSpace(authorized.ExecutionReportPath) != "" ||
			strings.TrimSpace(authorized.AdapterID) != "" || authorized.DeferSuccessfulTaskBinding ||
			strings.TrimSpace(binaryChild.SourcePath) != "" {
			fmt.Fprintln(stderr, "private bounded replay child flags cannot be combined with parent-only or source-path child flags")
			return true, 2
		}
		replayChild.RepoRoot = common.RepoRoot
		replayChild.CaseRoot = common.CaseRoot
		replayChild.Pack = common.Pack
		replayChild.GateEventID = common.GateEventID
		replayChild.ExpectedDispatchSHA256 = common.ExpectedDispatchSHA256
		replayChild.AdapterSession = authorized.AdapterSession
		replayChild.Executor = child.Executor
		replayChild.ExpectedExecutorGeneration = child.ExpectedExecutorGeneration
		replayChild.RequestPath = child.RequestPath
		replayChild.ExecutionControlBinding = executioncontrol.CloneBinding(controlBinding)
		replayChild.ParentLaneLeaseHandle = parentHandle
		replayChild.InstructionIdentity = cloneAdapterInstructionIdentity(instructionIdentity)
		result, err = RunBoundedReplayChild(replayChild)
	default:
		if parentHandle != 0 {
			fmt.Fprintln(stderr, "parent lane lease handle is valid only for a private child")
			return true, 2
		}
		if strings.TrimSpace(common.ExpectedDispatchSHA256) != "" || strings.TrimSpace(child.RequestPath) != "" ||
			strings.TrimSpace(binaryChild.SourcePath) != "" ||
			strings.TrimSpace(child.Executor) != "" || child.ExpectedExecutorGeneration != 0 {
			fmt.Fprintln(stderr, "authorized VMP IDA parent flags cannot be combined with immutable-dispatch child flags")
			return true, 2
		}
		authorized.ExecutionControlBinding = executioncontrol.CloneBinding(controlBinding)
		authorized.RepoRoot = common.RepoRoot
		authorized.CaseRoot = common.CaseRoot
		authorized.Pack = common.Pack
		authorized.GateEventID = common.GateEventID
		authorized.InstructionIdentity = cloneAdapterInstructionIdentity(instructionIdentity)
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
