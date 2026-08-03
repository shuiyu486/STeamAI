package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type completionProductResult struct {
	Command                     string                              `json:"command"`
	Applied                     bool                                `json:"applied"`
	Blocked                     bool                                `json:"blocked"`
	Blockers                    []completionProductBlocker          `json:"blockers"`
	CompletionPlanSHA256        string                              `json:"completionPlanSha256"`
	ApplyCommand                string                              `json:"applyCommand"`
	Lane                        completionProductLane               `json:"lane"`
	CompletionReceipt           *completionProductReceipt           `json:"completionReceipt"`
	MissionCommanderActionQueue missionCommanderActionQueueSnapshot `json:"missionCommanderActionQueue"`
}

type completionProductBlocker struct {
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
}

type completionProductLane struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type completionProductReceipt struct {
	State         string                      `json:"state"`
	Lane          string                      `json:"lane"`
	NoAuthority   bool                        `json:"noAuthority"`
	NoConfirmed   bool                        `json:"noConfirmed"`
	NoHeavyTool   bool                        `json:"noHeavyTool"`
	PreviewSHA256 string                      `json:"previewSha256"`
	Evidence      []completionProductEvidence `json:"evidence"`
}

type completionProductEvidence struct {
	Ref    string `json:"ref"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

func TestRunLaneCompletionProductPathRoutesNextLaneAndMissionComplete(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	runCompletionJSON(t, &out, []string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, nil)
	runCompletionJSON(t, &out, []string{"-Command", "start", "verifier", "-Target", caseRoot, "-Pack", "_template", "-Apply", "-Format", "json"}, nil)

	featureEvidence := filepath.Join(caseRoot, ".rekit", "lanes", "feature-verifier", "workspace", "completion-evidence.md")
	writeCompletionEvidence(t, featureEvidence, "reviewed verifier output")
	featurePreview := previewCompletion(t, &out, caseRoot, "verifier", ".rekit/lanes/feature-verifier/workspace/completion-evidence.md")
	before := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	if featurePreview.Blocked || len(featurePreview.CompletionPlanSHA256) != 64 || !strings.Contains(featurePreview.ApplyCommand, "-ExpectedCompletePlanSha256 "+featurePreview.CompletionPlanSHA256) {
		t.Fatalf("feature completion preview is not exact hash-bound: %+v", featurePreview)
	}
	assertSnapshotEqual(t, before, snapshotFiles(t, filepath.Join(caseRoot, ".rekit")))
	featureApplied := applyCompletion(t, &out, caseRoot, featurePreview)
	if !featureApplied.Applied || featureApplied.Lane.Status != "closed" || featureApplied.CompletionReceipt == nil || featureApplied.CompletionReceipt.State != "committed" || len(featureApplied.CompletionReceipt.Evidence) != 1 || len(featureApplied.CompletionReceipt.Evidence[0].SHA256) != 64 || featureApplied.CompletionReceipt.Evidence[0].Bytes == 0 || !featureApplied.CompletionReceipt.NoAuthority || !featureApplied.CompletionReceipt.NoConfirmed || !featureApplied.CompletionReceipt.NoHeavyTool {
		t.Fatalf("feature completion did not publish truthful committed closure: %+v", featureApplied)
	}
	if featureApplied.MissionCommanderActionQueue.CurrentDriverRequest == nil || !strings.Contains(featureApplied.MissionCommanderActionQueue.CurrentDriverRequest.Command, "/rekit continue main") {
		t.Fatalf("feature closure did not route to the next open main lane: %+v", featureApplied.MissionCommanderActionQueue)
	}
	if err := Run([]string{"-Command", "start", "verifier", "-Target", caseRoot, "-Pack", "_template", "-Force", "-Apply", "-Format", "json"}, &out); err == nil || !strings.Contains(err.Error(), "start refuses closed lane") {
		t.Fatalf("start must not implicitly reopen committed closed lane, err=%v", err)
	}

	mainEvidence := filepath.Join(caseRoot, ".rekit", "lanes", "main", "workspace", "completion-evidence.md")
	writeCompletionEvidence(t, mainEvidence, "reviewed aggregate mission evidence")
	mainPreview := previewCompletion(t, &out, caseRoot, "main", ".rekit/lanes/main/workspace/completion-evidence.md")
	mainApplied := applyCompletion(t, &out, caseRoot, mainPreview)
	if !mainApplied.Applied || mainApplied.Lane.Status != "closed" || mainApplied.MissionCommanderActionQueue.CurrentDriverRequest != nil {
		t.Fatalf("last lane closure should have no executable continue request: %+v", mainApplied)
	}

	var status struct {
		CaseMission *struct {
			Ready                       bool                                `json:"ready"`
			MissionCommanderActionQueue missionCommanderActionQueueSnapshot `json:"missionCommanderActionQueue"`
			DailyMissionControlRunbook  *dailyMissionControlRunbookSnapshot `json:"dailyMissionControlRunbook"`
			MissionCompletion           *struct {
				Ready                 bool                       `json:"ready"`
				State                 string                     `json:"state"`
				OperationallyComplete bool                       `json:"operationallyComplete"`
				OpenLaneCount         int                        `json:"openLaneCount"`
				CompletedLaneCount    int                        `json:"completedLaneCount"`
				Receipts              []completionProductReceipt `json:"receipts"`
				Boundary              []string                   `json:"boundary"`
			} `json:"missionCompletion"`
		} `json:"caseMission"`
		MissionControlRunbook *statusMissionControlRunbookSnapshot `json:"missionControlRunbook"`
	}
	runCompletionJSON(t, &out, []string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &status)
	if status.CaseMission == nil || status.CaseMission.Ready || status.CaseMission.MissionCompletion == nil || !status.CaseMission.MissionCompletion.Ready || status.CaseMission.MissionCompletion.State != "mission-complete" || !status.CaseMission.MissionCompletion.OperationallyComplete || status.CaseMission.MissionCompletion.OpenLaneCount != 0 || status.CaseMission.MissionCompletion.CompletedLaneCount != 2 || len(status.CaseMission.MissionCompletion.Receipts) != 2 || status.CaseMission.MissionCommanderActionQueue.CurrentDriverRequest != nil {
		t.Fatalf("all-closed status omitted typed terminal handoff: %+v", status.CaseMission)
	}
	if status.CaseMission.DailyMissionControlRunbook == nil || status.CaseMission.DailyMissionControlRunbook.CurrentDriverRequest != nil || status.CaseMission.DailyMissionControlRunbook.Ready {
		t.Fatalf("mission-complete case runbook must not suggest continue or bootstrap: %+v", status.CaseMission.DailyMissionControlRunbook)
	}
	beforeTerminalStart := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	if err := Run([]string{"-Command", "start", "later", "-Target", caseRoot, "-Pack", "_template", "-Apply", "-Format", "json"}, &out); err == nil || !strings.Contains(err.Error(), "start refuses mission-complete case") {
		t.Fatalf("ordinary start must not reactivate a terminal mission, err=%v", err)
	}
	assertSnapshotEqual(t, beforeTerminalStart, snapshotFiles(t, filepath.Join(caseRoot, ".rekit")))
	runbookJSON, err := json.Marshal(status.CaseMission.DailyMissionControlRunbook)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(runbookJSON), "start <name>") || strings.Contains(string(runbookJSON), "/rekit continue") || strings.Contains(string(runbookJSON), "handoff-preview") {
		t.Fatalf("mission-complete runbook leaked non-terminal action guidance: %s", runbookJSON)
	}
	for _, args := range [][]string{
		{"-Command", "continue", "verifier", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Format", "json"},
		{"-Command", "handoff", "verifier", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Format", "json"},
		{"-Command", "note", "-Target", caseRoot, "-Pack", "_template", "-Kind", "observation", "-Lane", "feature-verifier", "-Summary", "late note", "-WhatIf", "-Format", "json"},
		{"-Command", "gate", "-Target", caseRoot, "-Pack", "_template", "-Action", "debug", "-Lane", "feature-verifier", "-WhatIf", "-Format", "json"},
		{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-Lane", "feature-verifier", "-TaskType", "feature-analysis", "-Items", "late-review", "-Format", "json"},
	} {
		out.Reset()
		if err := Run(args, &out); err == nil || !strings.Contains(err.Error(), "refuses closed lane") {
			t.Fatalf("closed lane mutation must fail closed: args=%+v err=%v", args, err)
		}
	}
}

func TestRunLaneCompletionRejectsEvidenceDrift(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	runCompletionJSON(t, &out, []string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, nil)

	evidence := filepath.Join(caseRoot, ".rekit", "lanes", "main", "workspace", "completion-evidence.md")
	writeCompletionEvidence(t, evidence, "reviewed evidence A")
	preview := previewCompletion(t, &out, caseRoot, "main", ".rekit/lanes/main/workspace/completion-evidence.md")
	if len(preview.CompletionPlanSHA256) != 64 {
		t.Fatalf("completion preview omitted exact evidence-bound hash: %+v", preview)
	}
	writeCompletionEvidence(t, evidence, "unreviewed evidence B")
	beforeDriftApply := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	if err := Run(append(rekitCommandCLIArgs(t, preview.ApplyCommand), "-Target", caseRoot, "-Pack", "_template"), &out); err == nil || !strings.Contains(err.Error(), "preview sha256 mismatch") {
		t.Fatalf("evidence drift must invalidate the reviewed completion plan, err=%v", err)
	}
	assertSnapshotEqual(t, beforeDriftApply, snapshotFiles(t, filepath.Join(caseRoot, ".rekit")))
}

func TestRunLaneCompletionRejectsBlockersAndMainBeforeFeature(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	runCompletionJSON(t, &out, []string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, nil)
	runCompletionJSON(t, &out, []string{"-Command", "start", "worker", "-Target", caseRoot, "-Pack", "_template", "-Apply", "-Format", "json"}, nil)
	mainEvidence := filepath.Join(caseRoot, ".rekit", "lanes", "main", "workspace", "completion-evidence.md")
	writeCompletionEvidence(t, mainEvidence, "main evidence")
	mainPreview := previewCompletion(t, &out, caseRoot, "main", ".rekit/lanes/main/workspace/completion-evidence.md")
	if !mainPreview.Blocked || !hasCompletionBlocker(mainPreview.Blockers, "open-feature-lane") || mainPreview.CompletionPlanSHA256 != "" || mainPreview.ApplyCommand != "" {
		t.Fatalf("main closure must wait for feature lanes: %+v", mainPreview)
	}

	workerEvidence := filepath.Join(caseRoot, ".rekit", "lanes", "feature-worker", "workspace", "completion-evidence.md")
	writeCompletionEvidence(t, workerEvidence, "worker evidence")
	interventions := filepath.Join(caseRoot, ".rekit", "facts", "interventions.jsonl")
	appendCompletionJSONLine(t, interventions, `{"kind":"intervention","eventId":"int-complete-block","lane":"feature-worker","subject":"redirect worker","action":"override","status":"open"}`)
	workerPreview := previewCompletion(t, &out, caseRoot, "worker", ".rekit/lanes/feature-worker/workspace/completion-evidence.md")
	if !workerPreview.Blocked || !hasCompletionBlocker(workerPreview.Blockers, "open-intervention") || len(workerPreview.CompletionPlanSHA256) != 0 {
		t.Fatalf("open intervention must block closure: %+v", workerPreview)
	}
}

func previewCompletion(t *testing.T, out *bytes.Buffer, caseRoot, lane, evidence string) completionProductResult {
	t.Helper()
	var result completionProductResult
	runCompletionJSON(t, out, []string{"-Command", "complete", lane, "-Target", caseRoot, "-Pack", "_template", "-Actor", "main-agent", "-Reason", "reviewed durable evidence and operational lane completion", "-EvidenceRefs", evidence, "-WhatIf", "-Format", "json"}, &result)
	return result
}

func applyCompletion(t *testing.T, out *bytes.Buffer, caseRoot string, preview completionProductResult) completionProductResult {
	t.Helper()
	var result completionProductResult
	args := rekitCommandCLIArgs(t, preview.ApplyCommand)
	args = append(args, "-Target", caseRoot, "-Pack", "_template")
	runCompletionJSON(t, out, args, &result)
	return result
}

func runCompletionJSON(t *testing.T, out *bytes.Buffer, args []string, target any) {
	t.Helper()
	out.Reset()
	if err := Run(args, out); err != nil {
		t.Fatalf("completion product command failed: args=%+v err=%v\n%s", args, err, out.String())
	}
	if target != nil {
		if err := json.Unmarshal(out.Bytes(), target); err != nil {
			t.Fatalf("completion product output is not JSON: %v\n%s", err, out.String())
		}
	}
}

func writeCompletionEvidence(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func appendCompletionJSONLine(t *testing.T, path, line string) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(line + "\n"); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func hasCompletionBlocker(items []completionProductBlocker, kind string) bool {
	for _, item := range items {
		if item.Kind == kind {
			return true
		}
	}
	return false
}
