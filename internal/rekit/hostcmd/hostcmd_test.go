package hostcmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	"github.com/shuiyu486/re-context-kits/internal/rekit/packidentity"
	"github.com/shuiyu486/re-context-kits/internal/rekit/sessionhost"
)

func TestValidateAdapterFlagPackIdentityPolicy(t *testing.T) {
	for _, retired := range []string{packidentity.RetiredGeneric, packidentity.RetiredVMP} {
		err := ValidateAdapterFlag(false, retired, "")
		if err == nil || !packidentity.IsMigrationRequired(err) {
			t.Fatalf("ValidateAdapterFlag(%q) error = %v, want typed migration requirement", retired, err)
		}
	}
	if err := ValidateAdapterFlag(false, "does-not-exist", ""); err != nil {
		t.Fatalf("unknown pack must retain ordinary downstream semantics: %v", err)
	}
}

func TestRunRecoveryAcceptsOnlyDailySafeFlagsAndNeverFallsThrough(t *testing.T) {
	caseRoot := t.TempDir()
	before, err := os.ReadDir(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	exitCode := RunRecovery([]string{
		"-daily",
		"-target", caseRoot,
		"-claude", filepath.Join(caseRoot, "missing-claude"),
		"-model", "recovery-test-model",
		"-timeout", "1s",
		"-max-attempts", "7",
	}, &stdout, &stderr)
	if exitCode != 1 || !strings.Contains(
		stderr.String(),
		"requires a pending durable current project update",
	) || strings.Contains(stderr.String(), "flag provided but not defined") {
		t.Fatalf(
			"recovery daily safe flags exit=%d stderr=%q",
			exitCode,
			stderr.String(),
		)
	}
	var result sessionhost.DailyResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("recovery daily stdout is not typed JSON: %v\n%s", err, stdout.String())
	}
	if result.SessionLaunches != 0 || len(result.HostRuns) != 0 {
		t.Fatalf("recovery daily fell through to host execution: %+v", result)
	}
	after, err := os.ReadDir(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != len(after) {
		t.Fatalf("recovery daily changed ordinary target: before=%d after=%d", len(before), len(after))
	}

	for _, args := range [][]string{
		{"-target", caseRoot},
		{"-daily", "-target", caseRoot, "-live-acceptance"},
		{"-daily", "-target", caseRoot, "-internal-supervisor", "spec.json"},
	} {
		stdout.Reset()
		stderr.Reset()
		if code := RunRecovery(args, &stdout, &stderr); code != 2 {
			t.Fatalf("recovery forbidden args=%v exit=%d stderr=%q", args, code, stderr.String())
		}
	}
}

func TestRunDailyDirectoryAdoptionEmitsTypedActionWithoutMutation(t *testing.T) {
	caseRoot := t.TempDir()
	userPath := filepath.Join(caseRoot, "user.txt")
	original := []byte("keep\n")
	if err := os.WriteFile(userPath, original, 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{
		"-daily", "-target", caseRoot, "-goal", "inspect this project",
	}, &stdout, &stderr)
	if exitCode != 0 || stderr.Len() != 0 {
		t.Fatalf("daily directory adoption exit=%d stderr=%q", exitCode, stderr.String())
	}
	var result sessionhost.DailyResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("daily directory adoption stdout is not typed JSON: %v\n%s", err, stdout.String())
	}
	if !result.Blocked || result.Action == nil ||
		result.Action.Code != sessionhost.DailyActionDirectoryAdoptionRequired ||
		result.Action.RecoveryState != sessionhost.DailyRecoveryUserDecisionRequired ||
		!result.Action.RequiresInput || len(result.Action.Choices) != 2 ||
		result.Action.Now == "" || result.Action.Reason == "" || result.Action.Next == "" {
		t.Fatalf("daily directory adoption action = %+v", result.Action)
	}
	if result.Action.Choices[0].ID != "initialize-in-place" || result.Action.Choices[1].ID != "cancel" {
		t.Fatalf("daily directory adoption choices = %+v", result.Action.Choices)
	}
	current, err := os.ReadFile(userPath)
	if err != nil || !bytes.Equal(current, original) {
		t.Fatalf("daily directory adoption changed user file: %q err=%v", current, err)
	}
	entries, err := os.ReadDir(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "user.txt" {
		t.Fatalf("daily directory adoption wrote project state: %+v", entries)
	}
}

func TestRunDailyDirectoryAdoptionPreviewAndApplyBridge(t *testing.T) {
	caseRoot := t.TempDir()
	userPath := filepath.Join(caseRoot, "user.txt")
	original := []byte("keep\n")
	if err := os.WriteFile(userPath, original, 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	previewCode := Run([]string{
		"-daily", "-target", caseRoot,
		"-directory-adoption-action", "initialize-in-place",
	}, &stdout, &stderr)
	if previewCode != 0 || stderr.Len() != 0 {
		t.Fatalf("daily adoption preview exit=%d stderr=%q", previewCode, stderr.String())
	}
	var preview sessionhost.DailyResult
	if err := json.Unmarshal(stdout.Bytes(), &preview); err != nil {
		t.Fatalf("daily adoption preview JSON: %v\n%s", err, stdout.String())
	}
	if preview.Action == nil || preview.Action.Code != sessionhost.DailyActionConfirmationRequired || preview.DirectoryAdoption == nil || preview.DirectoryAdoption.Plan == nil || !preview.DirectoryAdoption.Plan.AdoptionReady || len(preview.DirectoryAdoption.Plan.ExpectedPlanSHA256) != 64 || preview.SessionLaunches != 0 {
		t.Fatalf("daily adoption preview = %+v", preview)
	}
	if _, err := os.Lstat(filepath.Join(caseRoot, ".steamai")); !os.IsNotExist(err) {
		t.Fatalf("daily adoption preview wrote project state: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	applyCode := Run([]string{
		"-daily", "-target", caseRoot,
		"-directory-adoption-action", "confirm-exact-plan",
		"-expected-init-plan-sha256", preview.DirectoryAdoption.Plan.ExpectedPlanSHA256,
	}, &stdout, &stderr)
	if applyCode != 0 || stderr.Len() != 0 {
		t.Fatalf("daily adoption Apply exit=%d stderr=%q", applyCode, stderr.String())
	}
	var applied sessionhost.DailyResult
	if err := json.Unmarshal(stdout.Bytes(), &applied); err != nil {
		t.Fatalf("daily adoption Apply JSON: %v\n%s", err, stdout.String())
	}
	if applied.Action == nil || applied.Action.Code != sessionhost.DailyActionReadyToContinue || applied.DirectoryAdoption == nil || applied.DirectoryAdoption.Apply == nil || !applied.DirectoryAdoption.Apply.Applied || applied.OnboardingApplied || applied.SessionLaunches != 0 {
		t.Fatalf("daily adoption Apply = %+v", applied)
	}
	if content, err := os.ReadFile(userPath); err != nil || !bytes.Equal(content, original) {
		t.Fatalf("daily adoption bridge changed user file: %q err=%v", content, err)
	}
	if _, err := os.Lstat(filepath.Join(caseRoot, ".rekit")); !os.IsNotExist(err) {
		t.Fatalf("daily adoption bridge wrote legacy state: %v", err)
	}
}

func TestRunProjectLocalRejectsOrdinaryDirectoryAdoptionControls(t *testing.T) {
	projectRoot := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := RunProjectLocal([]string{
		"-daily", "-directory-adoption-action", "initialize-in-place",
	}, &stdout, &stderr, projectRoot)
	if code != 2 || !strings.Contains(stderr.String(), "cannot adopt an ordinary directory") || stdout.Len() != 0 {
		t.Fatalf("project-local adoption controls exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunDailyControlFlagBridgePreviewApplyWithoutClaude(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "daily-control")
	bootstrap, err := sessionhost.RunDaily(context.Background(), sessionhost.DailyOptions{
		Target:                            caseRoot,
		Goal:                              "provision host control bridge fixture",
		Actor:                             "host-control-test",
		ClaudePath:                        filepath.Join(caseRoot, "missing-claude.exe"),
		ExpectedClaudeExecutableSHA256:    strings.Repeat("0", 64),
		ExpectedClaudeExecutablePublisher: "Anthropic, PBC",
	})
	if err == nil || !bootstrap.OnboardingApplied || bootstrap.Lane == "" || bootstrap.SessionLaunches != 0 {
		t.Fatalf("host control fixture result=%+v err=%v", bootstrap, err)
	}

	actor := "host-control-test"
	invoke := func(action, reason string, preview *executioncontrol.Plan) sessionhost.DailyResult {
		t.Helper()
		args := []string{
			"-target", caseRoot,
			"-lane", bootstrap.Lane,
			"-actor", actor,
			"-control-action", action,
			"-control-reason", reason,
		}
		if preview == nil {
			args = append(args, "-control-what-if")
		} else {
			args = append(args,
				"-control-apply",
				"-control-publication-stamp", preview.PublicationStamp,
				"-expected-control-plan-sha256", preview.ExpectedPlanSHA256,
			)
		}
		var stdout, stderr bytes.Buffer
		code := Run(args, &stdout, &stderr)
		if code != 0 || stderr.Len() != 0 {
			t.Fatalf("host control %s exit=%d stderr=%q", action, code, stderr.String())
		}
		var result sessionhost.DailyResult
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
			t.Fatalf("host control %s JSON: %v\n%s", action, err, stdout.String())
		}
		if result.Mode != "control" || result.Lane != bootstrap.Lane || result.ExecutionControl == nil ||
			result.ExecutionControl.Action != action || result.ExecutionControl.Actor != actor ||
			result.SessionLaunches != 0 || result.SessionCompletions != 0 || result.Replacements != 0 ||
			len(result.HostRuns) != 0 || len(result.DriverSteps) != 0 {
			t.Fatalf("host control flags crossed the zero-Claude boundary: %+v", result)
		}
		return result
	}

	for generation, fixture := range []struct {
		action string
		reason string
		state  string
	}{
		{executioncontrol.ActionPause, "reviewed pause from host flags", executioncontrol.StatePaused},
		{executioncontrol.ActionResume, "reviewed resume from host flags", executioncontrol.StateRunning},
		{executioncontrol.ActionStop, "reviewed stop from host flags", executioncontrol.StateStopped},
	} {
		before, err := executioncontrol.Inspect(caseRoot, bootstrap.Lane)
		if err != nil {
			t.Fatal(err)
		}
		previewResult := invoke(fixture.action, fixture.reason, nil)
		preview := previewResult.ExecutionControl
		if preview.Applied || !preview.RequiresConfirmation || len(preview.ExpectedPlanSHA256) != 64 || preview.PublicationStamp == "" {
			t.Fatalf("host control preview = %+v", preview)
		}
		afterPreview, err := executioncontrol.Inspect(caseRoot, bootstrap.Lane)
		if err != nil {
			t.Fatal(err)
		}
		if afterPreview.Pending || afterPreview.State != before.State || afterPreview.CurrentGeneration != before.CurrentGeneration || afterPreview.CurrentReceiptSHA256 != before.CurrentReceiptSHA256 {
			t.Fatalf("host control preview wrote durable state: before=%+v after=%+v", before, afterPreview)
		}
		applied := invoke(fixture.action, fixture.reason, preview).ExecutionControl
		if !applied.Applied || applied.State != fixture.state || applied.ControlGeneration != generation+1 {
			t.Fatalf("host control Apply = %+v", applied)
		}
	}
}

func TestRunDailyControlFlagConflictsFailBeforeMutation(t *testing.T) {
	for _, fixture := range []struct {
		name string
		args []string
		want string
		code int
	}{
		{
			name: "goal",
			args: []string{"-goal", "must not combine", "-control-action", "pause", "-control-reason", "conflict", "-control-what-if"},
			want: "cannot be combined with -goal or -correction",
			code: 1,
		},
		{
			name: "acceptance",
			args: []string{"-live-acceptance", "-adapter", "unused-adapter", "-control-action", "pause", "-control-reason", "conflict", "-control-what-if"},
			want: "supported only by the external daily front door",
			code: 2,
		},
		{
			name: "directory adoption",
			args: []string{"-directory-adoption-action", "initialize-in-place", "-control-action", "pause", "-control-reason", "conflict", "-control-what-if"},
			want: "mutually exclusive",
			code: 2,
		},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			caseRoot := filepath.Join(t.TempDir(), "must-not-exist")
			args := append([]string{"-target", caseRoot}, fixture.args...)
			var stdout, stderr bytes.Buffer
			code := Run(args, &stdout, &stderr)
			if code != fixture.code || !strings.Contains(stderr.String(), fixture.want) {
				t.Fatalf("control conflict exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			if _, err := os.Lstat(caseRoot); !os.IsNotExist(err) {
				t.Fatalf("control conflict created target: %v", err)
			}
		})
	}
}

func TestRunDailyFailureEmitsTypedRecoveryBeforeNonzeroExit(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "missing")
	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{
		"-daily", "-target", caseRoot, "-goal", "goal", "-correction", "correction",
	}, &stdout, &stderr)
	if exitCode != 1 || !strings.Contains(stderr.String(), "either -goal or -correction") {
		t.Fatalf("daily conflicting input exit=%d stderr=%q", exitCode, stderr.String())
	}
	var result sessionhost.DailyResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("daily failure stdout is not typed JSON: %v\n%s", err, stdout.String())
	}
	if result.Failure == nil || result.Action == nil ||
		result.Action.Code != sessionhost.DailyActionFailed ||
		result.Action.RecoveryState != sessionhost.DailyRecoveryUserDecisionRequired ||
		result.Action.Now == "" || result.Action.Reason == "" || result.Action.Next == "" ||
		result.Action.Recovery != result.Action.Next {
		t.Fatalf("daily failure action = %+v failure=%+v", result.Action, result.Failure)
	}
	if _, err := os.Lstat(caseRoot); !os.IsNotExist(err) {
		t.Fatalf("daily conflicting input created target: %v", err)
	}
}
