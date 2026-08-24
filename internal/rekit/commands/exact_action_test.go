package commands

import (
	"slices"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/plancontract"
)

func TestExactActionNormalizesCommandAndCLIArgs(t *testing.T) {
	fromCommand, err := ExactActionFromCommand(`/steamai continue -Lane "feature auth" -WhatIf -Format json`)
	if err != nil {
		t.Fatal(err)
	}
	fromArgs, err := ExactActionFromCLIArgs([]string{"-Command", Continue, "-Lane", "feature auth", "-WhatIf", "-Format", "json"})
	if err != nil {
		t.Fatal(err)
	}
	if err := fromCommand.Validate(); err != nil {
		t.Fatal(err)
	}
	if !fromCommand.Equivalent(fromArgs) || !slices.Equal(fromCommand.CLIArgs, []string{"-Command", Continue, "-Lane", "feature auth", "-WhatIf", "-Format", "json"}) {
		t.Fatalf("exact action normalization drifted: command=%+v args=%+v", fromCommand, fromArgs)
	}
	rendered, err := fromArgs.RenderForEntrypoint(CurrentPublicEntrypoint)
	if err != nil || rendered != `/steamai continue -Lane "feature auth" -WhatIf -Format json` {
		t.Fatalf("rendered exact action=%q err=%v", rendered, err)
	}
}

func TestExactActionValidatesStrictPlanApplyBinding(t *testing.T) {
	hash := strings.Repeat("a", 64)
	action, err := ExactActionFromCLIArgs([]string{
		"-Command", Continue,
		"-Lane", "main",
		"-ExpectedContinuePlanSha256", hash,
		"-Apply", "-Format", "json",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := action.ValidatePlanApply(Continue, hash); err != nil {
		t.Fatal(err)
	}
	if err := action.ValidatePlanApply(Complete, hash); err == nil {
		t.Fatal("strict Apply accepted a different command identity")
	}
	failureHash := strings.Repeat("b", 64)
	err = action.ValidatePlanApply(Continue, failureHash)
	failure, typed := plancontract.FromError(err)
	if !typed || failure.Code != plancontract.CodePlanMismatch || failure.MutationApplied || failure.MutationBoundary != "none" {
		t.Fatalf("strict Apply mismatch=%+v typed=%t err=%v", failure, typed, err)
	}
}

func TestExactActionRejectsCarrierDrift(t *testing.T) {
	action, err := ExactActionFromCLIArgs([]string{"-Command", Status, "-Format", "json"})
	if err != nil {
		t.Fatal(err)
	}
	action.CLIArgs = append(action.CLIArgs, "-WhatIf")
	if err := action.Validate(); err == nil {
		t.Fatal("exact action accepted drifted CLI args")
	}
	for _, args := range [][]string{nil, {"status"}, {"-Command"}, {"-Command", "unknown"}} {
		if _, err := ExactActionFromCLIArgs(args); err == nil {
			t.Fatalf("invalid exact action args accepted: %v", args)
		}
	}
}
