package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/currentloop"
)

type reopenProductResult struct {
	Command                     string                              `json:"command"`
	Applied                     bool                                `json:"applied"`
	RequestedLane               string                              `json:"requestedLane"`
	EffectiveTargets            []reopenProductTarget               `json:"effectiveTargets"`
	ReopenPlanSHA256            string                              `json:"reopenPlanSha256"`
	PublicationStamp            string                              `json:"publicationStamp"`
	ApplyCommand                string                              `json:"applyCommand"`
	ApplyArgs                   []string                            `json:"applyArgs"`
	OperationSequence           int                                 `json:"operationSequence"`
	OperationCommit             *reopenProductOperationCommit       `json:"operationCommit"`
	MissionCommanderActionQueue missionCommanderActionQueueSnapshot `json:"missionCommanderActionQueue"`
}

type reopenProductTarget struct {
	Lane struct {
		ID string `json:"id"`
	} `json:"lane"`
	Sequence                     int    `json:"sequence"`
	PreviousReceiptSHA256        string `json:"previousReceiptSha256"`
	SupersededCompletionSequence int    `json:"supersededCompletionSequence"`
	Reason                       string `json:"reason"`
}

type reopenProductOperationCommit struct {
	State        string `json:"state"`
	OperationID  string `json:"operationId"`
	Sequence     int    `json:"sequence"`
	NoAuthority  bool   `json:"noAuthority"`
	NoConfirmed  bool   `json:"noConfirmed"`
	NoHeavyTool  bool   `json:"noHeavyTool"`
	NoAutoResume bool   `json:"noAutoResume"`
}

type reopenProductLane struct {
	ID                 string `json:"id"`
	Status             string `json:"status"`
	CurrentExecutor    string `json:"currentExecutor"`
	ExecutorGeneration int    `json:"executorGeneration"`
}

func TestRunLaneReopenProductPathSupersedesTerminalCompletion(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	runCompletionJSON(t, &out, []string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, nil)
	loopPreview := runCurrentLoopPreview(t, caseRoot, 2)
	loopApplied := runCurrentLoopApply(t, caseRoot, loopPreview)
	if !loopApplied.Applied || loopApplied.SegmentCheckpoint == nil || loopApplied.SegmentCheckpoint.ArtifactSHA256 == "" {
		t.Fatalf("pre-reopen current-loop checkpoint was not published: %+v", loopApplied)
	}
	runCompletionJSON(t, &out, []string{"-Command", "start", "verifier", "-Target", caseRoot, "-Pack", "_template", "-Apply", "-Format", "json"}, nil)

	featureEvidence := filepath.Join(caseRoot, ".rekit", "lanes", "feature-verifier", "workspace", "completion-evidence.md")
	writeCompletionEvidence(t, featureEvidence, "reviewed verifier output")
	featureFirst := applyCompletion(t, &out, caseRoot, previewCompletion(t, &out, caseRoot, "verifier", ".rekit/lanes/feature-verifier/workspace/completion-evidence.md"))
	mainEvidence := filepath.Join(caseRoot, ".rekit", "lanes", "main", "workspace", "completion-evidence.md")
	writeCompletionEvidence(t, mainEvidence, "reviewed aggregate mission evidence")
	mainFirst := applyCompletion(t, &out, caseRoot, previewCompletion(t, &out, caseRoot, "main", ".rekit/lanes/main/workspace/completion-evidence.md"))
	if featureFirst.CompletionReceipt == nil || mainFirst.CompletionReceipt == nil {
		t.Fatal("initial mission completion receipts are missing")
	}

	featureReceiptPath := filepath.Join(caseRoot, ".rekit", "lanes", "feature-verifier", "completion.json")
	mainReceiptPath := filepath.Join(caseRoot, ".rekit", "lanes", "main", "completion.json")
	featureReceiptBefore, err := os.ReadFile(featureReceiptPath)
	if err != nil {
		t.Fatal(err)
	}
	mainReceiptBefore, err := os.ReadFile(mainReceiptPath)
	if err != nil {
		t.Fatal(err)
	}

	reopenEvidence := filepath.Join(caseRoot, ".rekit", "reopen-evidence.md")
	writeCompletionEvidence(t, reopenEvidence, "post-completion review found additional verifier work")
	beforePreview := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	var preview reopenProductResult
	runCompletionJSON(t, &out, []string{"-Command", "reopen", "verifier", "-Target", caseRoot, "-Pack", "_template", "-Actor", "main-agent", "-Reason", "post-completion review requires additional verifier evidence", "-EvidenceRefs", ".rekit/reopen-evidence.md", "-WhatIf", "-Format", "json"}, &preview)
	if preview.Command != "reopen" || preview.Applied || preview.RequestedLane != "feature-verifier" || preview.OperationSequence != 1 || len(preview.ReopenPlanSHA256) != 64 || preview.PublicationStamp == "" || !strings.Contains(preview.ApplyCommand, "-ReopenPublicationStamp "+preview.PublicationStamp) || !strings.Contains(preview.ApplyCommand, "-ExpectedReopenPlanSha256 "+preview.ReopenPlanSHA256) || len(preview.ApplyArgs) == 0 {
		t.Fatalf("compound reopen preview is not exact hash-bound: %+v", preview)
	}
	if len(preview.EffectiveTargets) != 2 || preview.EffectiveTargets[0].Lane.ID != "feature-verifier" || preview.EffectiveTargets[1].Lane.ID != "main" {
		t.Fatalf("terminal feature reopen did not explicitly include feature and authority targets: %+v", preview.EffectiveTargets)
	}
	for _, target := range preview.EffectiveTargets {
		if target.Sequence != 2 || target.SupersededCompletionSequence != 1 || len(target.PreviousReceiptSHA256) != 64 || strings.TrimSpace(target.Reason) == "" {
			t.Fatalf("reopen target omitted supersession identity: %+v", target)
		}
	}
	assertSnapshotEqual(t, beforePreview, snapshotFiles(t, filepath.Join(caseRoot, ".rekit")))

	var applied reopenProductResult
	runCompletionJSON(t, &out, preview.ApplyArgs, &applied)
	if !applied.Applied || applied.OperationCommit == nil || applied.OperationCommit.State != "committed" || applied.OperationCommit.Sequence != 1 || !applied.OperationCommit.NoAuthority || !applied.OperationCommit.NoConfirmed || !applied.OperationCommit.NoHeavyTool || !applied.OperationCommit.NoAutoResume {
		t.Fatalf("compound reopen did not publish a truthful final operation commit: %+v", applied)
	}
	if applied.MissionCommanderActionQueue.CurrentDriverRequest != nil {
		t.Fatalf("compound reopen must require an explicit lane choice: %+v", applied.MissionCommanderActionQueue)
	}
	requireMissionCommanderLaneChoices(t, applied.MissionCommanderActionQueue, "feature-verifier", "main")
	assertFileBytesEqual(t, featureReceiptPath, featureReceiptBefore)
	assertFileBytesEqual(t, mainReceiptPath, mainReceiptBefore)

	for _, laneID := range []string{"feature-verifier", "main"} {
		var lane reopenProductLane
		data, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "lanes", laneID, "lane.json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, &lane); err != nil {
			t.Fatal(err)
		}
		if lane.Status != "open" || lane.CurrentExecutor != "" || lane.ExecutorGeneration != 1 {
			t.Fatalf("reopen did not fence prior executor ownership for %s: %+v", laneID, lane)
		}
	}

	var status struct {
		CaseMission *struct {
			Ready             bool `json:"ready"`
			MissionCompletion *struct {
				State                 string `json:"state"`
				OperationallyComplete bool   `json:"operationallyComplete"`
				OpenLaneCount         int    `json:"openLaneCount"`
				CompletedLaneCount    int    `json:"completedLaneCount"`
			} `json:"missionCompletion"`
			MissionCommanderActionQueue missionCommanderActionQueueSnapshot `json:"missionCommanderActionQueue"`
		} `json:"caseMission"`
		MissionControlRunbook *struct {
			CurrentLoopSegment *currentloop.Inspection `json:"currentLoopSegment"`
		} `json:"missionControlRunbook"`
	}
	runCompletionJSON(t, &out, []string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Lane", "feature-verifier", "-Format", "json"}, &status)
	if status.CaseMission == nil || !status.CaseMission.Ready || status.CaseMission.MissionCompletion != nil || status.CaseMission.MissionCommanderActionQueue.CurrentDriverRequest == nil || status.CaseMission.MissionCommanderActionQueue.CurrentDriverRequest.Lane != "feature-verifier" {
		t.Fatalf("selected reopened lane did not return to active mission control: %+v", status.CaseMission)
	}
	checkpoint := status.MissionControlRunbook.CurrentLoopSegment
	if checkpoint == nil || checkpoint.Ready || checkpoint.State != "stale-reopen-lifecycle" || checkpoint.Continuation != nil || checkpoint.ResumeDriverRequest != nil || checkpoint.ArtifactSHA256 != loopApplied.SegmentCheckpoint.ArtifactSHA256 {
		t.Fatalf("reopen did not invalidate pre-reopen current-loop budget: %+v", checkpoint)
	}

	featureEvidence2 := filepath.Join(caseRoot, ".rekit", "lanes", "feature-verifier", "workspace", "completion-evidence-2.md")
	writeCompletionEvidence(t, featureEvidence2, "fresh verifier closure evidence after reopen")
	featureSecond := applyCompletion(t, &out, caseRoot, previewCompletion(t, &out, caseRoot, "verifier", ".rekit/lanes/feature-verifier/workspace/completion-evidence-2.md"))
	mainEvidence2 := filepath.Join(caseRoot, ".rekit", "lanes", "main", "workspace", "completion-evidence-2.md")
	writeCompletionEvidence(t, mainEvidence2, "fresh aggregate closure evidence after reopen")
	mainSecond := applyCompletion(t, &out, caseRoot, previewCompletion(t, &out, caseRoot, "main", ".rekit/lanes/main/workspace/completion-evidence-2.md"))
	if featureSecond.CompletionReceipt == nil || featureSecond.CompletionReceipt.Sequence != 3 || mainSecond.CompletionReceipt == nil || mainSecond.CompletionReceipt.Sequence != 3 {
		t.Fatalf("post-reopen closure did not append generation 3 completion receipts: feature=%+v main=%+v", featureSecond.CompletionReceipt, mainSecond.CompletionReceipt)
	}

	runCompletionJSON(t, &out, []string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &status)
	if status.CaseMission == nil || status.CaseMission.MissionCompletion == nil || status.CaseMission.MissionCompletion.State != "mission-complete" || !status.CaseMission.MissionCompletion.OperationallyComplete || status.CaseMission.MissionCompletion.CompletedLaneCount != 2 || status.CaseMission.MissionCompletion.OpenLaneCount != 0 {
		t.Fatalf("second closure did not restore mission-complete: %+v", status.CaseMission)
	}
}

func assertFileBytesEqual(t *testing.T, path string, expected []byte) {
	t.Helper()
	actual, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual, expected) {
		t.Fatalf("historical receipt bytes changed: %s", path)
	}
}
