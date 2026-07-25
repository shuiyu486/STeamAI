package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunContinueBlockedInterventionHandoffPreservesCurrentExecutor(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	writeHandoffFixture(t, caseRoot)
	before := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))

	var out bytes.Buffer
	if err := Run([]string{"-Command", "continue", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "login", "-Executor", "session-login", "-ExpectedExecutorGeneration", "2", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Blocked           bool `json:"blocked"`
		ReconcileRequired bool `json:"reconcileRequired"`
		ReconcileHandoffs []struct {
			EventID       string `json:"eventId"`
			WhatIfCommand string `json:"whatIfCommand"`
			ApplyCommand  string `json:"applyCommand"`
		} `json:"reconcileHandoffs"`
		Writes []startWrite `json:"writes"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("blocked owner continue JSON did not decode: %v\n%s", err, out.String())
	}
	if !result.Blocked || !result.ReconcileRequired || len(result.Writes) != 0 || len(result.ReconcileHandoffs) != 1 {
		t.Fatalf("unexpected owner-bound blocked continue: %+v", result)
	}
	handoff := result.ReconcileHandoffs[0]
	if handoff.EventID != "evt-human-stop" || handoff.WhatIfCommand != "/rekit reconcile login -InterventionId evt-human-stop -WhatIf -Executor session-login" || handoff.ApplyCommand != "/rekit reconcile login -InterventionId evt-human-stop -Apply -Executor session-login" {
		t.Fatalf("blocked continue did not preserve current executor in reconcile handoff: %+v", handoff)
	}
	assertSnapshotEqual(t, before, snapshotFiles(t, filepath.Join(caseRoot, ".rekit")))
}

func TestRunContinueReplaceableSessionOwnerGuardNestedProductPath(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	nested := filepath.Join(caseRoot, "workspace", "features", "feature-login", "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(nested); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldwd); err != nil {
			t.Fatal(err)
		}
	})

	var out bytes.Buffer
	if err := Run([]string{"start", "login", "-Name", "login", "-Apply", "-Force", "-Executor", "session-a", "-Actor", "mission-commander", "-Reason", "initial nested session claim"}, &out); err != nil {
		t.Fatal(err)
	}
	claim := decodeStartResult(t, out.Bytes())
	if claim.Lane.CurrentExecutor != "session-a" || claim.Lane.ExecutorGeneration != 1 {
		t.Fatalf("session-a claim = %+v, want generation 1", claim.Lane)
	}

	out.Reset()
	if err := Run([]string{"continue", "login", "-Apply", "-Executor", "session-a", "-ExpectedExecutorGeneration", "1", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var firstContinue struct {
		Applied bool      `json:"applied"`
		Lane    startLane `json:"lane"`
	}
	if err := json.Unmarshal(out.Bytes(), &firstContinue); err != nil {
		t.Fatalf("session-a continue JSON did not decode: %v\n%s", err, out.String())
	}
	if !firstContinue.Applied || firstContinue.Lane.CurrentExecutor != "session-a" || firstContinue.Lane.ExecutorGeneration != 1 {
		t.Fatalf("session-a could not continue with current guard: %+v", firstContinue)
	}

	out.Reset()
	if err := Run([]string{"start", "login", "-Name", "login", "-Apply", "-Force", "--executor", "session-b", "-Actor", "mission-commander", "-Reason", "replace nested session"}, &out); err != nil {
		t.Fatal(err)
	}
	takeover := decodeStartResult(t, out.Bytes())
	if takeover.Lane.CurrentExecutor != "session-b" || takeover.Lane.ExecutorGeneration != 2 {
		t.Fatalf("session-b takeover = %+v, want generation 2", takeover.Lane)
	}

	beforeStale := snapshotFiles(t, caseRoot)
	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "preview", args: []string{"continue", "login", "-WhatIf", "-Executor", "session-a", "-ExpectedExecutorGeneration", "1", "-Format", "json"}},
		{name: "apply", args: []string{"continue", "login", "-Apply", "--executor", "session-a", "--expected-executor-generation", "1", "-Format", "text"}},
	} {
		t.Run("stale-"+tc.name, func(t *testing.T) {
			out.Reset()
			err := Run(tc.args, &out)
			if err == nil || !strings.Contains(err.Error(), "owner guard is not current") {
				t.Fatalf("stale %s error = %v", tc.name, err)
			}
			assertSnapshotEqual(t, beforeStale, snapshotFiles(t, caseRoot))
		})
	}

	beforePreview := snapshotFiles(t, caseRoot)
	out.Reset()
	if err := Run([]string{"continue", "login", "-WhatIf", "--executor", "session-b", "--expected-executor-generation", "2", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "已选择工作线：feature-login") || strings.Contains(out.String(), "{\n") {
		t.Fatalf("current session-b text preview unavailable:\n%s", out.String())
	}
	assertSnapshotEqual(t, beforePreview, snapshotFiles(t, caseRoot))

	out.Reset()
	if err := Run([]string{"continue", "login", "-Apply", "-Executor", "session-b", "-ExpectedExecutorGeneration", "2", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var currentContinue struct {
		Applied bool      `json:"applied"`
		RunID   string    `json:"runId"`
		Lane    startLane `json:"lane"`
	}
	if err := json.Unmarshal(out.Bytes(), &currentContinue); err != nil {
		t.Fatalf("session-b continue JSON did not decode: %v\n%s", err, out.String())
	}
	if !currentContinue.Applied || currentContinue.RunID == "" || currentContinue.Lane.CurrentExecutor != "session-b" || currentContinue.Lane.ExecutorGeneration != 2 {
		t.Fatalf("current session-b apply failed: %+v", currentContinue)
	}
}
