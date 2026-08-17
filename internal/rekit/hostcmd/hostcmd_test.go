package hostcmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/sessionhost"
)

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
