package plancontract

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

const (
	CodePhaseConflict = "plan-phase-conflict"
	CodePlanMissing   = "plan-binding-missing"
	CodePlanInvalid   = "plan-binding-invalid"
	CodePlanMismatch  = "plan-binding-mismatch"

	StagePreviewValidation = "preview-validation"
	StageApplyValidation   = "apply-validation"
	StagePlanCurrentness   = "plan-currentness"
)

type Failure struct {
	Code             string `json:"code"`
	Stage            string `json:"stage"`
	Detail           string `json:"detail"`
	MutationBoundary string `json:"mutationBoundary"`
	MutationApplied  bool   `json:"mutationApplied"`
	NextAction       string `json:"nextAction"`
	Flag             string `json:"flag,omitempty"`
	Expected         string `json:"expected,omitempty"`
	Actual           string `json:"actual,omitempty"`
}

type Error struct {
	Failure Failure
}

func (err *Error) Error() string {
	if err == nil {
		return ""
	}
	return err.Failure.Detail
}

func FromError(err error) (Failure, bool) {
	var typed *Error
	if !errors.As(err, &typed) || typed == nil {
		return Failure{}, false
	}
	return typed.Failure, true
}

func NormalizeSHA256(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	decoded, err := hex.DecodeString(normalized)
	if err != nil || len(decoded) != 32 {
		return "", fmt.Errorf("sha256 must contain exactly 64 hexadecimal characters")
	}
	return normalized, nil
}

func ValidatePhase(command, flag string, whatIf, apply bool, expected string) (string, error) {
	command = strings.TrimSpace(command)
	flag = strings.TrimSpace(flag)
	switch {
	case whatIf == apply:
		return "", failure(
			CodePhaseConflict,
			StagePreviewValidation,
			flag,
			"",
			expected,
			fmt.Sprintf("%s requires exactly one of preview or Apply", command),
			fmt.Sprintf("Rerun %s in preview mode, then consume only its exact Apply action.", command),
		)
	case whatIf && strings.TrimSpace(expected) != "":
		return "", failure(
			CodePhaseConflict,
			StagePreviewValidation,
			flag,
			"",
			expected,
			fmt.Sprintf("%s preview must not carry %s", command, flag),
			fmt.Sprintf("Remove %s and rerun the preview.", flag),
		)
	case whatIf:
		return "", nil
	}
	return RequireApplyBinding(command, flag, expected)
}

func InvalidBinding(command, flag string, apply bool, actual string) error {
	command = strings.TrimSpace(command)
	flag = strings.TrimSpace(flag)
	stage := StagePreviewValidation
	if apply {
		stage = StageApplyValidation
	}
	return failure(
		CodePlanInvalid,
		stage,
		flag,
		strings.TrimSpace(actual),
		"",
		fmt.Sprintf("%s received an invalid or duplicate %s binding", command, flag),
		fmt.Sprintf("Rerun %s in preview mode and execute only the returned exact Apply action.", command),
	)
}

func RequireApplyBinding(command, flag, expected string) (string, error) {
	command = strings.TrimSpace(command)
	flag = strings.TrimSpace(flag)
	if strings.TrimSpace(expected) == "" {
		return "", failure(
			CodePlanMissing,
			StageApplyValidation,
			flag,
			"",
			"",
			fmt.Sprintf("%s Apply requires %s from the exact preview", command, flag),
			fmt.Sprintf("Rerun %s in preview mode and execute only the returned exact Apply action.", command),
		)
	}
	normalized, err := NormalizeSHA256(expected)
	if err != nil {
		return "", failure(
			CodePlanInvalid,
			StageApplyValidation,
			flag,
			"",
			strings.TrimSpace(expected),
			fmt.Sprintf("%s Apply received an invalid %s", command, flag),
			fmt.Sprintf("Rerun %s in preview mode and execute only the returned exact Apply action.", command),
		)
	}
	return normalized, nil
}

func Match(command, flag, expected, actual string) (string, error) {
	normalizedExpected, err := RequireApplyBinding(command, flag, expected)
	if err != nil {
		return "", err
	}
	normalizedActual, normalizeErr := NormalizeSHA256(actual)
	if normalizeErr != nil || normalizedExpected != normalizedActual {
		return "", failure(
			CodePlanMismatch,
			StagePlanCurrentness,
			flag,
			normalizedActual,
			normalizedExpected,
			fmt.Sprintf("%s plan changed after preview: got %s want %s", strings.TrimSpace(command), normalizedExpected, normalizedActual),
			fmt.Sprintf("Rerun %s in preview mode and review the fresh exact Apply action.", strings.TrimSpace(command)),
		)
	}
	return normalizedExpected, nil
}

func failure(code, stage, flag, actual, expected, detail, nextAction string) error {
	return &Error{Failure: Failure{
		Code:             code,
		Stage:            stage,
		Detail:           detail,
		MutationBoundary: "none",
		MutationApplied:  false,
		NextAction:       nextAction,
		Flag:             flag,
		Expected:         expected,
		Actual:           actual,
	}}
}
