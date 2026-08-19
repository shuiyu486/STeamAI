package sessionhost

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	"github.com/shuiyu486/re-context-kits/internal/rekit/laneowner"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

func TestRunPublicNotePreviewApplyRechecksStaleDuplicateControl(t *testing.T) {
	repoRoot := sessionhostTestRepoRoot(t)
	caseRoot := provisionSessionhostAttachedCase(t, repoRoot, liveAcceptancePack)
	stateRoot := filepath.Join(caseRoot, projectstate.CurrentDir)
	if err := os.WriteFile(filepath.Join(stateRoot, "board.json"), []byte(`{"lanes":[{"id":"main","status":"open","currentExecutor":"executor-a","executorGeneration":1}]}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	lanePath := filepath.Join(stateRoot, "lanes", "main", "lane.json")
	if err := os.MkdirAll(filepath.Dir(lanePath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lanePath, []byte(`{"id":"main","status":"open","currentExecutor":"executor-a","executorGeneration":1}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	owner, err := laneowner.Read(caseRoot, "main")
	if err != nil {
		t.Fatal(err)
	}
	binding, err := executioncontrol.CaptureBinding(caseRoot, owner)
	if err != nil {
		t.Fatal(err)
	}
	eventArgs := []string{
		"-Kind", "verification", "-Lane", "main",
		"-Subject", "execution evidence review accepted",
		"-Summary", "accepted recorded execution evidence for gateEventId gate-exact",
		"-Actor", "mission-commander", "-Verifier", "tool-review",
		"-Verdict", "accepted", "-Status", "resolved",
		"-Related", "observation-exact,gate-exact", "-Reason", "accepted; exact lineage",
		"-TargetRef", "tooling/ida-agent-bridge/requests/request.json",
		"-EvidenceRefs", "ida-index:functions:tooling/ida-agent-bridge/export/function_index.tsv#L2",
		"-CreatedAt", "2026-08-19T00:00:00Z",
	}
	preview, applied, err := runPublicNotePreviewApply(caseRoot, liveAcceptancePack, eventArgs, binding)
	if err != nil {
		t.Fatal(err)
	}
	if preview.IsMutation || preview.Applied || !applied.IsMutation || !applied.Applied || applied.EventID != preview.EventID {
		t.Fatalf("public controlled note preview/apply = preview=%+v applied=%+v", preview, applied)
	}

	control, err := executioncontrol.Preview(caseRoot, executioncontrol.Options{
		Lane: "main", Action: executioncontrol.ActionPause, Actor: "public-note-test",
		Reason: "make the exact acknowledgement control birth stale", PublicationStamp: "2026-08-19T00:00:01Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := executioncontrol.Apply(caseRoot, executioncontrol.Options{
		Lane: control.Lane, Action: control.Action, Actor: control.Actor, Reason: control.Reason,
		PublicationStamp: control.PublicationStamp, ExpectedPlanSHA256: control.ExpectedPlanSHA256,
	}); err != nil {
		t.Fatal(err)
	}
	_, _, err = runPublicNotePreviewApply(caseRoot, liveAcceptancePack, eventArgs, binding)
	if err == nil || !strings.Contains(err.Error(), "lane execution is paused") {
		t.Fatalf("stale duplicate public note error = %v", err)
	}
}

func TestRunPublicApplyCommandRejectsUnexpectedOrReboundRoute(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    string
	}{
		{name: "unexpected command", command: `/rekit status -Apply -Format json`, want: "expected bounded reopen route"},
		{name: "missing Apply", command: `/rekit reopen mission -Format json`, want: "expected bounded reopen route"},
		{name: "command override", command: `/rekit reopen mission -Apply -Command status -Format json`, want: "bounded command"},
		{name: "target override", command: `/rekit reopen mission -Apply -Target other -Format json`, want: "must not override"},
		{name: "pack override", command: `/rekit reopen mission -Apply --pack other -Format json`, want: "must not override"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := runPublicApplyCommand(test.command, "reopen", t.TempDir(), "_template", nil)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("runPublicApplyCommand error=%v want substring %q", err, test.want)
			}
		})
	}
}
