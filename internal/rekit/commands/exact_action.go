package commands

import (
	"fmt"
	"slices"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/plancontract"
)

// ExactAction normalizes the existing public command and CLI-argument carriers
// into one typed identity. It is intentionally not a second wire DTO: domain
// responses may keep the carrier that is natural for their boundary, while
// construction and conformance checks consume this shared representation.
type ExactAction struct {
	Invocation PublicInvocation
	CLIArgs    []string
}

func ExactActionFromCommand(command string) (ExactAction, error) {
	invocation, err := ParsePublicInvocation(command)
	if err != nil {
		return ExactAction{}, err
	}
	return exactActionForInvocation(invocation)
}

func ExactActionFromCLIArgs(args []string) (ExactAction, error) {
	if len(args) < 2 || !strings.EqualFold(strings.TrimSpace(args[0]), "-Command") || strings.TrimSpace(args[1]) == "" {
		return ExactAction{}, fmt.Errorf("exact action CLI args must begin with -Command <command>")
	}
	invocation, err := NewPublicInvocation(args[1], args[2:]...)
	if err != nil {
		return ExactAction{}, err
	}
	return exactActionForInvocation(invocation)
}

func exactActionForInvocation(invocation PublicInvocation) (ExactAction, error) {
	args, err := invocation.CLIArgs()
	if err != nil {
		return ExactAction{}, err
	}
	return ExactAction{
		Invocation: invocation,
		CLIArgs:    args,
	}, nil
}

func (action ExactAction) Validate() error {
	if err := action.Invocation.Validate(); err != nil {
		return err
	}
	args, err := action.Invocation.CLIArgs()
	if err != nil {
		return err
	}
	if !slices.Equal(args, action.CLIArgs) {
		return fmt.Errorf("exact action CLI args do not match its typed invocation")
	}
	return nil
}

func (action ExactAction) RenderForEntrypoint(entrypoint string) (string, error) {
	if err := action.Validate(); err != nil {
		return "", err
	}
	return action.Invocation.RenderForEntrypoint(entrypoint)
}

// ValidatePlanApply verifies that this carrier is the exact Apply phase for a
// stable strict-plan command and binds the reviewed plan hash.
func (action ExactAction) ValidatePlanApply(command, expectedPlanSHA256 string) error {
	if err := action.Validate(); err != nil {
		return err
	}
	command = strings.ToLower(strings.TrimSpace(command))
	if action.Invocation.Command != command {
		return fmt.Errorf("exact action command is %s, want %s", action.Invocation.Command, command)
	}
	if err := ValidateExecutablePlanInvocation(action.Invocation); err != nil {
		return err
	}
	contract, ok := MutationContractFor(command, "")
	if !ok || contract.Currentness != MutationCurrentnessStrictPlan {
		return fmt.Errorf("strict plan Apply validation is unsupported for %s", command)
	}
	actual, present, valid := action.Invocation.FlagValue(contract.ExpectedAliases...)
	if !present || !valid {
		return plancontract.InvalidBinding(command, contract.ExpectedFlag, true, actual)
	}
	if _, err := plancontract.Match(command, contract.ExpectedFlag, expectedPlanSHA256, actual); err != nil {
		return err
	}
	return nil
}

func (action ExactAction) Equivalent(other ExactAction) bool {
	return action.Invocation.Equivalent(other.Invocation) && slices.Equal(action.CLIArgs, other.CLIArgs)
}
