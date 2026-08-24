package plancontract

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestNormalizeSHA256(t *testing.T) {
	upper := strings.Repeat("AB", 32)
	got, err := NormalizeSHA256(" \t" + upper + "\n")
	if err != nil || got != strings.ToLower(upper) {
		t.Fatalf("NormalizeSHA256=%q, %v", got, err)
	}
	for _, value := range []string{"", strings.Repeat("a", 63), strings.Repeat("a", 65), strings.Repeat("g", 64)} {
		if _, err := NormalizeSHA256(value); err == nil {
			t.Fatalf("NormalizeSHA256 accepted %q", value)
		}
	}
}

func TestValidatePhaseAndMatchReturnTypedFailures(t *testing.T) {
	hashA := strings.Repeat("a", 64)
	hashB := strings.Repeat("b", 64)
	tests := []struct {
		name  string
		err   error
		code  string
		stage string
	}{
		{name: "missing phase", err: phaseError("continue", false, false, ""), code: CodePhaseConflict, stage: StagePreviewValidation},
		{name: "both phases", err: phaseError("continue", true, true, ""), code: CodePhaseConflict, stage: StagePreviewValidation},
		{name: "preview binds hash", err: phaseError("continue", true, false, hashA), code: CodePhaseConflict, stage: StagePreviewValidation},
		{name: "apply misses hash", err: phaseError("continue", false, true, ""), code: CodePlanMissing, stage: StageApplyValidation},
		{name: "apply invalid hash", err: phaseError("continue", false, true, "bad"), code: CodePlanInvalid, stage: StageApplyValidation},
		{name: "plan drift", err: matchError("continue", hashA, hashB), code: CodePlanMismatch, stage: StagePlanCurrentness},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			wrapped := fmt.Errorf("wrapped: %w", test.err)
			failure, ok := FromError(wrapped)
			if !ok || failure.Code != test.code || failure.Stage != test.stage || failure.MutationApplied || failure.MutationBoundary != "none" || failure.NextAction == "" {
				t.Fatalf("FromError=%+v, %t", failure, ok)
			}
			var typed *Error
			if !errors.As(wrapped, &typed) || typed.Failure.Flag != "-ExpectedContinuePlanSha256" {
				t.Fatalf("errors.As lost typed failure: %#v", typed)
			}
		})
	}
}

func TestValidatePhaseAndMatchAcceptExactBindings(t *testing.T) {
	hash := strings.Repeat("a", 64)
	if got, err := ValidatePhase("continue", "-ExpectedContinuePlanSha256", true, false, ""); err != nil || got != "" {
		t.Fatalf("preview phase=%q, %v", got, err)
	}
	if got, err := ValidatePhase("continue", "-ExpectedContinuePlanSha256", false, true, strings.ToUpper(hash)); err != nil || got != hash {
		t.Fatalf("apply phase=%q, %v", got, err)
	}
	if got, err := Match("continue", "-ExpectedContinuePlanSha256", strings.ToUpper(hash), hash); err != nil || got != hash {
		t.Fatalf("Match=%q, %v", got, err)
	}
}

func TestInvalidBindingReturnsTypedFailureForPhase(t *testing.T) {
	for _, apply := range []bool{false, true} {
		failure, ok := FromError(InvalidBinding("continue", "-ExpectedContinuePlanSha256", apply, "duplicate"))
		if !ok || failure.Code != CodePlanInvalid || failure.MutationApplied || failure.MutationBoundary != "none" {
			t.Fatalf("InvalidBinding=%+v, %t", failure, ok)
		}
		wantStage := StagePreviewValidation
		if apply {
			wantStage = StageApplyValidation
		}
		if failure.Stage != wantStage || failure.Actual != "duplicate" || failure.NextAction == "" {
			t.Fatalf("InvalidBinding phase=%+v", failure)
		}
	}
}

func phaseError(command string, whatIf, apply bool, expected string) error {
	_, err := ValidatePhase(command, "-ExpectedContinuePlanSha256", whatIf, apply, expected)
	return err
}

func matchError(command, expected, actual string) error {
	_, err := Match(command, "-ExpectedContinuePlanSha256", expected, actual)
	return err
}
