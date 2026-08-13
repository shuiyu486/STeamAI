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
	if err := Run([]string{"start", "-Name", "login", "-Apply", "-Force", "-Executor", "session-a", "-Actor", "mission-commander", "-Reason", "initial nested session claim"}, &out); err != nil {
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
	if err := Run([]string{"start", "-Name", "login", "-Apply", "-Force", "--executor", "session-b", "-Actor", "mission-commander", "-Reason", "replace nested session"}, &out); err != nil {
		t.Fatal(err)
	}
	takeover := decodeStartResult(t, out.Bytes())
	if takeover.Lane.CurrentExecutor != "session-b" || takeover.Lane.ExecutorGeneration != 2 {
		t.Fatalf("session-b takeover = %+v, want generation 2", takeover.Lane)
	}

	beforeStale := snapshotFiles(t, caseRoot)
	out.Reset()
	if err := Run([]string{"continue", "login", "-WhatIf", "-Executor", "session-a", "-ExpectedExecutorGeneration", "1", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var stalePreview struct {
		Blocked                    bool         `json:"blocked"`
		Applied                    bool         `json:"applied"`
		IsMutation                 bool         `json:"isMutation"`
		Writes                     []startWrite `json:"writes"`
		ContinueOwnerGuardRecovery struct {
			Ready                       bool                 `json:"ready"`
			Reason                      string               `json:"reason"`
			ReceivedExecutor            string               `json:"receivedExecutor"`
			ReceivedExecutorGeneration  int                  `json:"receivedExecutorGeneration"`
			CurrentExecutor             string               `json:"currentExecutor"`
			CurrentExecutorGeneration   int                  `json:"currentExecutorGeneration"`
			CurrentContinueCommand      string               `json:"currentContinueCommand"`
			HandoffPath                 string               `json:"handoffPath"`
			StartTakeoverPreviewCommand string               `json:"startTakeoverPreviewCommand"`
			StartTakeoverApplyCommand   string               `json:"startTakeoverApplyCommand"`
			LaneTakeoverPackage         *laneTakeoverPackage `json:"laneTakeoverPackage"`
			Boundary                    []string             `json:"boundary"`
		} `json:"continueOwnerGuardRecovery"`
	}
	if err := json.Unmarshal(out.Bytes(), &stalePreview); err != nil {
		t.Fatalf("stale continue preview JSON did not decode: %v\n%s", err, out.String())
	}
	if !stalePreview.Blocked || stalePreview.Applied || stalePreview.IsMutation || len(stalePreview.Writes) != 0 || !stalePreview.ContinueOwnerGuardRecovery.Ready || !strings.Contains(stalePreview.ContinueOwnerGuardRecovery.Reason, "owner guard is not current") || stalePreview.ContinueOwnerGuardRecovery.ReceivedExecutor != "session-a" || stalePreview.ContinueOwnerGuardRecovery.ReceivedExecutorGeneration != 1 || stalePreview.ContinueOwnerGuardRecovery.CurrentExecutor != "session-b" || stalePreview.ContinueOwnerGuardRecovery.CurrentExecutorGeneration != 2 || stalePreview.ContinueOwnerGuardRecovery.CurrentContinueCommand != "/rekit continue login -Executor session-b -ExpectedExecutorGeneration 2" || stalePreview.ContinueOwnerGuardRecovery.HandoffPath != ".rekit/handovers/feature-login-latest.md" || stalePreview.ContinueOwnerGuardRecovery.LaneTakeoverPackage == nil || stalePreview.ContinueOwnerGuardRecovery.LaneTakeoverPackage.CurrentExecutor != "session-b" || !containsSubstring(stalePreview.ContinueOwnerGuardRecovery.Boundary, "zero-write") {
		t.Fatalf("stale continue preview omitted owner guard recovery: %+v", stalePreview)
	}
	if !strings.Contains(stalePreview.ContinueOwnerGuardRecovery.StartTakeoverPreviewCommand, "/rekit start login -WhatIf -Executor <new-executor>") || !strings.Contains(stalePreview.ContinueOwnerGuardRecovery.StartTakeoverApplyCommand, "/rekit start login -Apply -Executor <new-executor>") {
		t.Fatalf("stale continue preview omitted takeover commands: %+v", stalePreview.ContinueOwnerGuardRecovery)
	}
	assertSnapshotEqual(t, beforeStale, snapshotFiles(t, caseRoot))

	out.Reset()
	if err := Run([]string{"continue", "login", "-Apply", "--executor", "session-a", "--expected-executor-generation", "1", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"continue owner guard recovery：blocked=true ready=true lane=feature-login label=login", "continue owner guard received：executor=session-a generation=1", "continue owner guard current：executor=session-b generation=2 continue=`/rekit continue login -Executor session-b -ExpectedExecutorGeneration 2`", "continue owner guard paths：resume=`.rekit/lanes/feature-login/prompts/RESUME.md` checkpoint=`.rekit/lanes/feature-login/checkpoints/latest.json` handoff=`.rekit/handovers/feature-login-latest.md`", "continue owner guard takeover：preview=`/rekit start login -WhatIf -Executor <new-executor>", "continue owner guard boundary：owner guard mismatch is fail-closed and zero-write"} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("stale continue text missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "已选择工作线：feature-login") || strings.Contains(out.String(), "{\n") {
		t.Fatalf("stale continue text should be recovery guidance only:\n%s", out.String())
	}
	assertSnapshotEqual(t, beforeStale, snapshotFiles(t, caseRoot))

	beforePreview := snapshotFiles(t, caseRoot)
	out.Reset()
	if err := Run([]string{"continue", "login", "-WhatIf", "--executor", "session-b", "--expected-executor-generation", "2", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "已选择工作线：feature-login") || strings.Contains(out.String(), "{\n") {
		t.Fatalf("current session-b text preview unavailable:\n%s", out.String())
	}
	assertSnapshotEqual(t, beforePreview, snapshotFiles(t, caseRoot))

	beforeTakeoverPreview := snapshotFiles(t, caseRoot)
	out.Reset()
	if err := Run([]string{"start", "login", "-WhatIf", "-Executor", "session-c", "-Actor", "mission-commander", "-Reason", "replace stale executor after owner guard mismatch", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	takeoverPreview := decodeStartResult(t, out.Bytes())
	if takeoverPreview.IsMutation || takeoverPreview.Applied || takeoverPreview.Lane.ID != "feature-login" || takeoverPreview.Lane.CurrentExecutor != "session-c" || takeoverPreview.Lane.ExecutorGeneration != 3 || takeoverPreview.LaneTakeoverPackage == nil || !takeoverPreview.LaneTakeoverPackage.ApplyRequired || takeoverPreview.LaneTakeoverPackage.CurrentCommand != `/rekit start login -Apply -Executor session-c -Actor mission-commander -Reason "replace stale executor after owner guard mismatch"` || takeoverPreview.ExecutorAction.MissionCommanderAction.PrimaryCommand != takeoverPreview.LaneTakeoverPackage.CurrentCommand {
		t.Fatalf("recovery-guided takeover preview did not produce apply-ready package: %+v", takeoverPreview)
	}
	if !containsMissionCommanderNextAction(takeoverPreview.MissionCommanderNextActions, "missionCommanderActions.followUp", "/rekit continue login -Executor session-c -ExpectedExecutorGeneration 3", true, true) {
		t.Fatalf("recovery-guided takeover preview omitted post-apply continue handoff: %+v", takeoverPreview.MissionCommanderNextActions)
	}
	assertSnapshotEqual(t, beforeTakeoverPreview, snapshotFiles(t, caseRoot))

	out.Reset()
	if err := Run([]string{"start", "login", "-Apply", "-Executor", "session-c", "-Actor", "mission-commander", "-Reason", "replace stale executor after owner guard mismatch", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	takeoverApply := decodeStartResult(t, out.Bytes())
	if !takeoverApply.IsMutation || !takeoverApply.Applied || takeoverApply.Lane.ID != "feature-login" || takeoverApply.Lane.CurrentExecutor != "session-c" || takeoverApply.Lane.ExecutorGeneration != 3 || takeoverApply.Lane.LastTakeoverBy != "mission-commander" || takeoverApply.Lane.LastTakeoverReason != "replace stale executor after owner guard mismatch" {
		t.Fatalf("recovery-guided takeover apply did not replace executor: %+v", takeoverApply)
	}
	assertStartWrite(t, takeoverApply.Writes, ".rekit/lanes/feature-login/lane.json", "update-executor-takeover")
	if takeoverApply.LaneTakeoverPackage == nil || takeoverApply.LaneTakeoverPackage.ApplyRequired || !takeoverApply.LaneTakeoverPackage.ContinueReady || takeoverApply.LaneTakeoverPackage.ContinueCommand != "/rekit continue login -Executor session-c -ExpectedExecutorGeneration 3" || takeoverApply.LaneTakeoverPackage.CurrentCommand != "/rekit continue login -Executor session-c -ExpectedExecutorGeneration 3" {
		t.Fatalf("recovery-guided takeover apply omitted current continue package: %+v", takeoverApply.LaneTakeoverPackage)
	}

	beforeSecondStale := snapshotFiles(t, caseRoot)
	out.Reset()
	if err := Run([]string{"continue", "login", "-WhatIf", "-Executor", "session-b", "-ExpectedExecutorGeneration", "2", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var staleSessionB struct {
		Blocked                    bool         `json:"blocked"`
		Applied                    bool         `json:"applied"`
		IsMutation                 bool         `json:"isMutation"`
		Writes                     []startWrite `json:"writes"`
		ContinueOwnerGuardRecovery struct {
			Ready                     bool   `json:"ready"`
			ReceivedExecutor          string `json:"receivedExecutor"`
			CurrentExecutor           string `json:"currentExecutor"`
			CurrentExecutorGeneration int    `json:"currentExecutorGeneration"`
			CurrentContinueCommand    string `json:"currentContinueCommand"`
		} `json:"continueOwnerGuardRecovery"`
	}
	if err := json.Unmarshal(out.Bytes(), &staleSessionB); err != nil {
		t.Fatalf("post-takeover stale session-b JSON did not decode: %v\n%s", err, out.String())
	}
	if !staleSessionB.Blocked || staleSessionB.Applied || staleSessionB.IsMutation || len(staleSessionB.Writes) != 0 || !staleSessionB.ContinueOwnerGuardRecovery.Ready || staleSessionB.ContinueOwnerGuardRecovery.ReceivedExecutor != "session-b" || staleSessionB.ContinueOwnerGuardRecovery.CurrentExecutor != "session-c" || staleSessionB.ContinueOwnerGuardRecovery.CurrentExecutorGeneration != 3 || staleSessionB.ContinueOwnerGuardRecovery.CurrentContinueCommand != "/rekit continue login -Executor session-c -ExpectedExecutorGeneration 3" {
		t.Fatalf("post-takeover stale session-b recovery did not point at session-c: %+v", staleSessionB)
	}
	assertSnapshotEqual(t, beforeSecondStale, snapshotFiles(t, caseRoot))

	out.Reset()
	if err := Run([]string{"continue", "login", "-Apply", "-Executor", "session-c", "-ExpectedExecutorGeneration", "3", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var currentContinue struct {
		Applied bool      `json:"applied"`
		RunID   string    `json:"runId"`
		Lane    startLane `json:"lane"`
	}
	if err := json.Unmarshal(out.Bytes(), &currentContinue); err != nil {
		t.Fatalf("session-c continue JSON did not decode: %v\n%s", err, out.String())
	}
	if !currentContinue.Applied || currentContinue.RunID == "" || currentContinue.Lane.CurrentExecutor != "session-c" || currentContinue.Lane.ExecutorGeneration != 3 {
		t.Fatalf("current session-c apply failed: %+v", currentContinue)
	}
}
