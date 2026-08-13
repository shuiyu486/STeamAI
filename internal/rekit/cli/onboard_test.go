package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	"github.com/shuiyu486/re-context-kits/internal/rekit/missionintent"
	syncreview "github.com/shuiyu486/re-context-kits/internal/rekit/sync"
)

type onboardCLIPlan struct {
	Command              string   `json:"command"`
	CaseRoot             string   `json:"caseRoot"`
	PublicationStamp     string   `json:"publicationStamp"`
	OnboardingPlanSHA256 string   `json:"onboardingPlanSha256"`
	ApplyCommand         string   `json:"applyCommand"`
	ApplyArgs            []string `json:"applyArgs"`
}

func TestRunOnboardPreviewApplyStatusAndReplay(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "onboard-case")
	args := []string{"-Command", "onboard", "-Target", caseRoot, "-Pack", "_template", "-ProjectName", "demo", "-Goal", "opaque goal", "-Actor", "operator", "-Executor", "executor-a", "-InitialLane", "feature-analysis"}
	var out bytes.Buffer
	if err := Run(append(append([]string{}, args...), "-WhatIf", "-Format", "json"), &out); err != nil {
		t.Fatal(err)
	}
	var plan onboardCLIPlan
	if err := json.Unmarshal(out.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.Command != "onboard" || plan.CaseRoot != caseRoot || plan.OnboardingPlanSHA256 == "" || !strings.HasPrefix(plan.ApplyCommand, "/rekit onboard ") || !strings.Contains(plan.ApplyCommand, plan.PublicationStamp) || len(plan.ApplyArgs) == 0 {
		t.Fatalf("unexpected preview: %+v", plan)
	}
	if _, err := os.Lstat(caseRoot); !os.IsNotExist(err) {
		t.Fatalf("WhatIf wrote target: %v", err)
	}
	apply := append([]string{}, plan.ApplyArgs...)
	out.Reset()
	if err := Run(apply, &out); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(caseRoot, ".rekit", "board.json")); !os.IsNotExist(err) {
		t.Fatalf("onboard created board: %v", err)
	}
	out.Reset()
	var overviewStatus struct {
		Onboarding struct {
			State string `json:"state"`
		} `json:"onboarding"`
		MissionControlRunbook *statusMissionControlRunbookSnapshot `json:"missionControlRunbook"`
	}
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out.Bytes(), &overviewStatus); err != nil {
		t.Fatal(err)
	}
	if overviewStatus.Onboarding.State != "committed" || overviewStatus.MissionControlRunbook == nil || overviewStatus.MissionControlRunbook.Quickstart == nil || !strings.Contains(overviewStatus.MissionControlRunbook.Quickstart.Command, "/rekit overview ") {
		t.Fatalf("status did not expose the single overview onboarding route: %+v\n%s", overviewStatus, out.String())
	}
	takeover := overviewStatus.MissionControlRunbook.ReplacementExecutorTakeoverPackage
	if takeover == nil || !containsSubstring(takeover.TargetDocuments, ".rekit/mission-intent.json") {
		t.Fatalf("replacement takeover omitted durable mission identity: %+v", takeover)
	}
	overviewArgs, ok := missionCommanderDriverRequestCommandCLIArgs(t, overviewStatus.MissionControlRunbook.Quickstart.CurrentDriverRequest)
	if !ok {
		t.Fatal("overview route is not executable")
	}
	out.Reset()
	if err := Run(overviewArgs, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	var startStatus struct {
		MissionControlRunbook *statusMissionControlRunbookSnapshot `json:"missionControlRunbook"`
	}
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out.Bytes(), &startStatus); err != nil {
		t.Fatal(err)
	}
	startRequest := startStatus.MissionControlRunbook.Quickstart.CurrentDriverRequest
	if startRequest == nil || !strings.Contains(startRequest.Command, "/rekit start ") || !strings.Contains(startRequest.Command, ` "analysis" `) || !strings.Contains(startRequest.Command, `-Executor "executor-a"`) || !strings.Contains(startRequest.Command, `-Actor "operator"`) {
		t.Fatalf("fresh status omitted exact durable start route: %+v", startRequest)
	}
	startPreviewArgs, ok := missionCommanderDriverRequestCommandCLIArgs(t, startRequest)
	if !ok {
		t.Fatal("start preview route is not executable")
	}
	out.Reset()
	if err := Run(startPreviewArgs, &out); err != nil {
		t.Fatal(err)
	}
	var startPreview struct {
		MissionCommanderActionQueue missionCommanderActionQueueSnapshot `json:"missionCommanderActionQueue"`
	}
	if err := json.Unmarshal(out.Bytes(), &startPreview); err != nil {
		t.Fatal(err)
	}
	startApplyArgs, ok := missionCommanderDriverRequestCommandCLIArgs(t, startPreview.MissionCommanderActionQueue.CurrentDriverRequest)
	if !ok {
		t.Fatal("start preview omitted exact apply request")
	}
	startApplyArgs = append(startApplyArgs, "-Target", caseRoot, "-Pack", "_template", "-Format", "json")
	out.Reset()
	if err := Run(startApplyArgs, &out); err != nil {
		t.Fatal(err)
	}
	laneBytes, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "lanes", "feature-analysis", "lane.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(laneBytes), `"currentExecutor": "executor-a"`) || !strings.Contains(string(laneBytes), `"executorGeneration": 1`) {
		t.Fatalf("initial lane was not created and claimed exactly: %s", laneBytes)
	}
	before := snapshotFiles(t, caseRoot)
	out.Reset()
	if err := Run(apply, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"replay": true`) {
		t.Fatalf("committed replay not reported: %s", out.String())
	}
	assertSnapshotEqual(t, before, snapshotFiles(t, caseRoot))
}

func TestRunOnboardDefaultPackRoundTripsEmittedStartRoute(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "vmp-onboard-case")
	base := []string{"-Command", "onboard", "-Target", caseRoot, "-Pack", defaults.DefaultPack, "-ProjectName", "vmp-journey", "-Goal", "opaque default-pack goal", "-Actor", "operator", "-Executor", "executor-a", "-InitialLane", "feature-analysis-live-check"}
	var out bytes.Buffer
	var onboard onboardCLIPlan
	if err := Run(append(append([]string{}, base...), "-WhatIf", "-Format", "json"), &out); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out.Bytes(), &onboard); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run(onboard.ApplyArgs, &out); err != nil {
		t.Fatal(err)
	}

	statusArgs := []string{"-Command", "status", "-Target", caseRoot, "-Pack", defaults.DefaultPack, "-Format", "json"}
	selectedStatusArgs := append(append([]string{}, statusArgs...), "-Lane", "feature-analysis-live-check")
	var status struct {
		MissionControlRunbook *statusMissionControlRunbookSnapshot `json:"missionControlRunbook"`
	}
	out.Reset()
	if err := Run(statusArgs, &out); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.MissionControlRunbook == nil || status.MissionControlRunbook.Quickstart == nil {
		t.Fatalf("default-pack status omitted onboarding runbook: %+v", status.MissionControlRunbook)
	}
	overviewArgs, ok := missionCommanderDriverRequestCommandCLIArgs(t, status.MissionControlRunbook.Quickstart.CurrentDriverRequest)
	if !ok {
		t.Fatalf("default-pack overview route is not executable: %+v", status.MissionControlRunbook)
	}
	out.Reset()
	if err := Run(overviewArgs, &out); err != nil {
		t.Fatal(err)
	}

	out.Reset()
	if err := Run(statusArgs, &out); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.MissionControlRunbook == nil || status.MissionControlRunbook.Quickstart == nil {
		t.Fatalf("default-pack status omitted start runbook: %+v", status.MissionControlRunbook)
	}
	startRequest := status.MissionControlRunbook.Quickstart.CurrentDriverRequest
	if startRequest == nil || startRequest.Lane != "feature-analysis-live-check" || startRequest.Label != "analysis-live-check" || !strings.Contains(startRequest.Command, ` "analysis-live-check" `) {
		t.Fatalf("default-pack status did not emit the exact round-trip start route: %+v", startRequest)
	}
	startPreviewArgs, ok := missionCommanderDriverRequestCommandCLIArgs(t, startRequest)
	if !ok {
		t.Fatalf("default-pack start preview route is not executable: %+v", startRequest)
	}
	var startPreview struct {
		MissionCommanderActionQueue missionCommanderActionQueueSnapshot `json:"missionCommanderActionQueue"`
	}
	out.Reset()
	if err := Run(startPreviewArgs, &out); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out.Bytes(), &startPreview); err != nil {
		t.Fatal(err)
	}
	startApplyArgs, ok := missionCommanderDriverRequestCommandCLIArgs(t, startPreview.MissionCommanderActionQueue.CurrentDriverRequest)
	if !ok {
		t.Fatalf("default-pack start preview omitted exact apply request: %+v", startPreview.MissionCommanderActionQueue)
	}
	startApplyArgs = append(startApplyArgs, "-Target", caseRoot, "-Pack", defaults.DefaultPack, "-Format", "json")
	out.Reset()
	if err := Run(startApplyArgs, &out); err != nil {
		t.Fatal(err)
	}

	lanePath := filepath.Join(caseRoot, ".rekit", "lanes", "feature-analysis-live-check", "lane.json")
	laneBytes, err := os.ReadFile(lanePath)
	if err != nil {
		t.Fatal(err)
	}
	var lane struct {
		ID                 string `json:"id"`
		Type               string `json:"type"`
		Name               string `json:"name"`
		CurrentExecutor    string `json:"currentExecutor"`
		ExecutorGeneration int    `json:"executorGeneration"`
	}
	if err := json.Unmarshal(laneBytes, &lane); err != nil {
		t.Fatal(err)
	}
	if lane.ID != "feature-analysis-live-check" || lane.Type != "feature-analysis" || lane.Name != "analysis-live-check" || lane.CurrentExecutor != "executor-a" || lane.ExecutorGeneration != 1 {
		t.Fatalf("default-pack initial lane did not preserve exact identity and owner: %+v", lane)
	}

	out.Reset()
	if err := Run(selectedStatusArgs, &out); err != nil {
		t.Fatal(err)
	}
	status = struct {
		MissionControlRunbook *statusMissionControlRunbookSnapshot `json:"missionControlRunbook"`
	}{}
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.MissionControlRunbook == nil || status.MissionControlRunbook.Quickstart == nil {
		t.Fatalf("default-pack status omitted post-start runbook: %+v", status.MissionControlRunbook)
	}
	current := status.MissionControlRunbook.Quickstart.CurrentDriverRequest
	if current == nil || current.Lane != "feature-analysis-live-check" || !strings.Contains(current.Command, "-Lane feature-analysis-live-check") || !strings.Contains(current.Command, "-Executor executor-a -ExpectedExecutorGeneration 1") {
		t.Fatalf("default-pack status did not focus the owned initial feature lane after exact lane creation: %+v", current)
	}
	if current.Source == "committedMissionIntent" || strings.Contains(current.Command, "/rekit start ") {
		t.Fatalf("default-pack status repeated committed mission-intent bootstrap after exact lane creation: %+v", current)
	}

	loop := runCurrentLoopResult(t, []string{"-Command", "run-current-loop", "-Target", caseRoot, "-Pack", defaults.DefaultPack, "-MaxSteps", "2", "-Lane", "feature-analysis-live-check", "-WhatIf", "-Format", "json"})
	if loop.InitialCurrentStep == nil || loop.InitialCurrentStep.MemberExecution == nil || loop.ExpectedCurrentLoopPlanSHA256 == "" {
		t.Fatalf("default-pack current-loop preview omitted the owned member dispatch: %+v", loop)
	}
	memberPlan := loop.InitialCurrentStep.MemberExecution
	appliedLoop := runCurrentLoopResult(t, []string{
		"-Command", "run-current-loop", "-Target", caseRoot, "-Pack", defaults.DefaultPack, "-MaxSteps", "2",
		"-Lane", "feature-analysis-live-check",
		"-ExpectedMemberExecutionPlanSha256", memberPlan.ExpectedPlanSHA256,
		"-ExpectedCurrentLoopPlanSha256", loop.ExpectedCurrentLoopPlanSHA256,
		"-Apply", "-Format", "json",
	})
	if !appliedLoop.Applied || appliedLoop.AppliedSteps != 1 || appliedLoop.StopReason.Code != "external-member-handoff" || appliedLoop.SegmentCheckpoint == nil || !appliedLoop.SegmentCheckpoint.Ready || appliedLoop.SegmentCheckpoint.RemainingMaxSteps != 1 || appliedLoop.SegmentCheckpoint.StopCode != "external-member-handoff" {
		t.Fatalf("default-pack current-loop did not publish the member handoff checkpoint: %+v", appliedLoop)
	}

	var durableStatus statusInventory
	out.Reset()
	if err := Run(selectedStatusArgs, &out); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out.Bytes(), &durableStatus); err != nil {
		t.Fatal(err)
	}
	operator := durableStatus.MissionControlRunbook.CurrentLoopOperator
	if operator == nil || operator.ExternalMemberHandoff == nil || operator.ExternalSessionJob == nil || operator.ExternalSessionJob.State != "awaiting-submission" || operator.ExternalSessionJob.SessionKind != "member" || operator.ExternalSessionJob.MemberAttemptID != memberPlan.AttemptID {
		t.Fatalf("default-pack status omitted the deterministic external member job: %+v", operator)
	}
}

func TestRunStatusRecoversIntentFirstPendingWithoutInstanceMetadata(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "pending-status")
	args := []string{"-Command", "onboard", "-Target", caseRoot, "-Pack", "_template", "-ProjectName", "demo", "-Goal", "opaque goal", "-Actor", "operator", "-Executor", "executor-a", "-InitialLane", "feature-analysis"}
	var out bytes.Buffer
	if err := Run(append(append([]string{}, args...), "-WhatIf", "-Format", "json"), &out); err != nil {
		t.Fatal(err)
	}
	var plan onboardCLIPlan
	if err := json.Unmarshal(out.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	restore := syncreview.SetExclusiveInitLeafWriteHookForTest(func(stage, path string) error {
		if stage == "before-publish" && path != missionintent.IntentRel {
			return os.ErrClosed
		}
		return nil
	})
	if err := Run(plan.ApplyArgs, &bytes.Buffer{}); err == nil {
		t.Fatal("hooked onboard unexpectedly completed")
	}
	restore()
	if _, err := os.Lstat(filepath.Join(caseRoot, ".rekit", "instance.yml")); !os.IsNotExist(err) {
		t.Fatalf("partial status fixture unexpectedly has instance metadata: %v", err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var status struct {
		Mode       string `json:"mode"`
		Onboarding struct {
			State     string   `json:"state"`
			ApplyArgs []string `json:"applyArgs"`
		} `json:"onboarding"`
	}
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.Mode != "case-onboarding-pending" || status.Onboarding.State != "pending" || len(status.Onboarding.ApplyArgs) == 0 {
		t.Fatalf("fresh pending status omitted exact recovery: %+v\n%s", status, out.String())
	}
}

func TestRunOnboardPublicJourneyThroughReopenAndRecompletion(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "onboard-journey")
	base := []string{"-Command", "onboard", "-Target", caseRoot, "-Pack", "_template", "-ProjectName", "journey", "-Goal", "opaque end-to-end goal", "-Actor", "operator", "-Executor", "executor-a", "-InitialLane", "feature-analysis"}
	var out bytes.Buffer
	var onboard onboardCLIPlan
	runCompletionJSON(t, &out, append(append([]string{}, base...), "-WhatIf", "-Format", "json"), &onboard)
	runCompletionJSON(t, &out, onboard.ApplyArgs, nil)

	var status struct {
		CaseMission *struct {
			MissionCompletion *struct {
				State                 string `json:"state"`
				OperationallyComplete bool   `json:"operationallyComplete"`
				OpenLaneCount         int    `json:"openLaneCount"`
				CompletedLaneCount    int    `json:"completedLaneCount"`
			} `json:"missionCompletion"`
		} `json:"caseMission"`
		MissionControlRunbook *statusMissionControlRunbookSnapshot `json:"missionControlRunbook"`
	}
	runCompletionJSON(t, &out, []string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &status)
	overviewArgs, ok := missionCommanderDriverRequestCommandCLIArgs(t, status.MissionControlRunbook.Quickstart.CurrentDriverRequest)
	if !ok {
		t.Fatal("onboard journey overview route is not executable")
	}
	runCompletionJSON(t, &out, overviewArgs, nil)
	runCompletionJSON(t, &out, []string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &status)
	startArgs, ok := missionCommanderDriverRequestCommandCLIArgs(t, status.MissionControlRunbook.Quickstart.CurrentDriverRequest)
	if !ok {
		t.Fatal("onboard journey start route is not executable")
	}
	var startPreview struct {
		MissionCommanderActionQueue missionCommanderActionQueueSnapshot `json:"missionCommanderActionQueue"`
	}
	runCompletionJSON(t, &out, startArgs, &startPreview)
	startApplyArgs, ok := missionCommanderDriverRequestCommandCLIArgs(t, startPreview.MissionCommanderActionQueue.CurrentDriverRequest)
	if !ok {
		t.Fatal("onboard journey start apply route is not executable")
	}
	runCompletionJSON(t, &out, append(startApplyArgs, "-Target", caseRoot, "-Pack", "_template", "-Format", "json"), nil)

	var continuedPreview struct {
		MissionCommanderActionQueue missionCommanderActionQueueSnapshot `json:"missionCommanderActionQueue"`
	}
	runCompletionJSON(t, &out, []string{"-Command", "continue", "analysis", "-Target", caseRoot, "-Pack", "_template", "-Executor", "executor-a", "-ExpectedExecutorGeneration", "1", "-WhatIf", "-Format", "json"}, &continuedPreview)
	continueApplyArgs, ok := missionCommanderDriverRequestCommandCLIArgs(t, continuedPreview.MissionCommanderActionQueue.CurrentDriverRequest)
	if !ok {
		t.Fatal("onboard journey continue apply route is not executable")
	}
	runCompletionJSON(t, &out, append(continueApplyArgs, "-Target", caseRoot, "-Pack", "_template", "-Apply", "-Format", "json"), nil)

	noteArgs := []string{"-Command", "note", "-Target", caseRoot, "-Pack", "_template", "-Kind", "intervention", "-Lane", "feature-analysis", "-Subject", "journey correction", "-Summary", "operator redirected analysis", "-Action", "override", "-Status", "open", "-Actor", "operator", "-EventId", "int-onboard-journey", "-CreatedAt", "2026-08-03T01:02:03Z", "-WhatIf", "-Format", "json"}
	var notePreview struct {
		RecordCommand string `json:"recordCommand"`
	}
	runCompletionJSON(t, &out, noteArgs, &notePreview)
	runCompletionJSON(t, &out, rekitCommandCLIArgs(t, notePreview.RecordCommand), nil)
	var reconcilePreview struct {
		MissionCommanderActionQueue missionCommanderActionQueueSnapshot `json:"missionCommanderActionQueue"`
	}
	runCompletionJSON(t, &out, []string{"-Command", "reconcile", "analysis", "-Target", caseRoot, "-Pack", "_template", "-InterventionId", "int-onboard-journey", "-Actor", "operator", "-Reason", "reviewed operator correction", "-WhatIf", "-Format", "json"}, &reconcilePreview)
	reconcileApplyArgs, ok := missionCommanderDriverRequestCommandCLIArgs(t, reconcilePreview.MissionCommanderActionQueue.CurrentDriverRequest)
	if !ok {
		t.Fatal("onboard journey reconcile apply route is not executable")
	}
	runCompletionJSON(t, &out, append(reconcileApplyArgs, "-Target", caseRoot, "-Pack", "_template", "-Format", "json"), nil)

	var handoffPreview struct {
		ApplyCommand string `json:"applyCommand"`
	}
	runCompletionJSON(t, &out, []string{"-Command", "handoff", "analysis", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Format", "json"}, &handoffPreview)
	var handoffApplied struct {
		Applied                            bool                                        `json:"applied"`
		ReplacementExecutorTakeoverPackage *replacementExecutorTakeoverPackageSnapshot `json:"replacementExecutorTakeoverPackage"`
	}
	runCompletionJSON(t, &out, rekitCommandCLIArgs(t, handoffPreview.ApplyCommand), &handoffApplied)
	if !handoffApplied.Applied || handoffApplied.ReplacementExecutorTakeoverPackage == nil || !handoffApplied.ReplacementExecutorTakeoverPackage.Ready {
		t.Fatalf("public journey handoff omitted replacement package: %+v", handoffApplied)
	}

	featureEvidence := filepath.Join(caseRoot, ".rekit", "lanes", "feature-analysis", "workspace", "completion-evidence.md")
	writeCompletionEvidence(t, featureEvidence, "reviewed initial analysis closure")
	featureFirst := applyCompletion(t, &out, caseRoot, previewCompletion(t, &out, caseRoot, "analysis", ".rekit/lanes/feature-analysis/workspace/completion-evidence.md"))
	mainEvidence := filepath.Join(caseRoot, ".rekit", "lanes", "main", "workspace", "completion-evidence.md")
	writeCompletionEvidence(t, mainEvidence, "reviewed initial aggregate closure")
	mainFirst := applyCompletion(t, &out, caseRoot, previewCompletion(t, &out, caseRoot, "main", ".rekit/lanes/main/workspace/completion-evidence.md"))
	if featureFirst.CompletionReceipt == nil || mainFirst.CompletionReceipt == nil || featureFirst.CompletionReceipt.Sequence != 1 || mainFirst.CompletionReceipt.Sequence != 1 {
		t.Fatalf("first public journey completion receipts missing: feature=%+v main=%+v", featureFirst, mainFirst)
	}
	runCompletionJSON(t, &out, []string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &status)
	if status.CaseMission == nil || status.CaseMission.MissionCompletion == nil || status.CaseMission.MissionCompletion.State != "mission-complete" || !status.CaseMission.MissionCompletion.OperationallyComplete || status.CaseMission.MissionCompletion.CompletedLaneCount != 2 || status.CaseMission.MissionCompletion.OpenLaneCount != 0 {
		t.Fatalf("first public journey closure did not reach terminal state: %+v", status.CaseMission)
	}

	reopenEvidence := filepath.Join(caseRoot, ".rekit", "reopen-evidence.md")
	writeCompletionEvidence(t, reopenEvidence, "review found additional analysis work")
	var reopenPreview reopenProductResult
	runCompletionJSON(t, &out, []string{"-Command", "reopen", "analysis", "-Target", caseRoot, "-Pack", "_template", "-Actor", "operator", "-Reason", "review requires fresh analysis evidence", "-EvidenceRefs", ".rekit/reopen-evidence.md", "-WhatIf", "-Format", "json"}, &reopenPreview)
	var reopened reopenProductResult
	runCompletionJSON(t, &out, append(rekitCommandCLIArgs(t, reopenPreview.ApplyCommand), "-Target", caseRoot, "-Pack", "_template"), &reopened)
	if !reopened.Applied || reopened.OperationCommit == nil || !reopened.OperationCommit.NoAuthority || !reopened.OperationCommit.NoConfirmed {
		t.Fatalf("public journey reopen receipt is not truthful: %+v", reopened)
	}
	runCompletionJSON(t, &out, []string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &status)
	if status.MissionControlRunbook == nil || status.MissionControlRunbook.Quickstart == nil || status.MissionControlRunbook.Quickstart.CurrentDriverRequest == nil {
		t.Fatalf("fresh status omitted post-reopen route: %+v", status)
	}

	featureEvidence2 := filepath.Join(caseRoot, ".rekit", "lanes", "feature-analysis", "workspace", "completion-evidence-2.md")
	writeCompletionEvidence(t, featureEvidence2, "fresh analysis closure after reopen")
	featureSecond := applyCompletion(t, &out, caseRoot, previewCompletion(t, &out, caseRoot, "analysis", ".rekit/lanes/feature-analysis/workspace/completion-evidence-2.md"))
	mainEvidence2 := filepath.Join(caseRoot, ".rekit", "lanes", "main", "workspace", "completion-evidence-2.md")
	writeCompletionEvidence(t, mainEvidence2, "fresh aggregate closure after reopen")
	mainSecond := applyCompletion(t, &out, caseRoot, previewCompletion(t, &out, caseRoot, "main", ".rekit/lanes/main/workspace/completion-evidence-2.md"))
	if featureSecond.CompletionReceipt == nil || featureSecond.CompletionReceipt.Sequence != 3 || mainSecond.CompletionReceipt == nil || mainSecond.CompletionReceipt.Sequence != 3 {
		t.Fatalf("public journey recompletion generation drifted: feature=%+v main=%+v", featureSecond.CompletionReceipt, mainSecond.CompletionReceipt)
	}
	runCompletionJSON(t, &out, []string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &status)
	if status.CaseMission == nil || status.CaseMission.MissionCompletion == nil || status.CaseMission.MissionCompletion.State != "mission-complete" || !status.CaseMission.MissionCompletion.OperationallyComplete || status.CaseMission.MissionCompletion.CompletedLaneCount != 2 || status.CaseMission.MissionCompletion.OpenLaneCount != 0 {
		t.Fatalf("public journey recompletion did not restore terminal state: %+v", status.CaseMission)
	}
	for _, rel := range []string{".rekit/facts/authority.jsonl", ".rekit/facts/confirmed.jsonl"} {
		if _, err := os.Lstat(filepath.Join(caseRoot, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Fatalf("public journey wrote forbidden %s: %v", rel, err)
		}
	}
}

func TestRunStatusReportsCorruptOnboardingAndMutationFailsClosed(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "corrupt")
	args := []string{"-Command", "onboard", "-Target", caseRoot, "-Pack", "_template", "-ProjectName", "demo", "-Goal", "opaque goal", "-Actor", "operator", "-Executor", "executor-a", "-InitialLane", "feature-analysis"}
	var out bytes.Buffer
	if err := Run(append(append([]string{}, args...), "-WhatIf", "-Format", "json"), &out); err != nil {
		t.Fatal(err)
	}
	var plan onboardCLIPlan
	if err := json.Unmarshal(out.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	if err := Run(append(append([]string{}, args...), "-OnboardingPublicationStamp", plan.PublicationStamp, "-ExpectedOnboardingPlanSha256", plan.OnboardingPlanSHA256, "-Apply"), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(caseRoot, ".rekit", "onboarding", "commit.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatalf("status must remain readable: %v", err)
	}
	if !strings.Contains(out.String(), `"state": "corrupt"`) || !strings.Contains(out.String(), "ordinary attached commands are blocked") {
		t.Fatalf("status omitted corrupt onboarding diagnostic: %s", out.String())
	}
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "onboarding state is corrupt") {
		t.Fatalf("ordinary attached mutation error = %v", err)
	}
}

func TestRunOnboardRejectsHashAndInputDrift(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "drift")
	base := []string{"-Command", "onboard", "-Target", caseRoot, "-Pack", "_template", "-ProjectName", "demo", "-Goal", "opaque goal", "-Actor", "operator", "-Executor", "executor-a", "-InitialLane", "feature-analysis"}
	var out bytes.Buffer
	if err := Run(append(append([]string{}, base...), "-WhatIf", "-Format", "json"), &out); err != nil {
		t.Fatal(err)
	}
	var plan onboardCLIPlan
	if err := json.Unmarshal(out.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	badHash := strings.Repeat("0", 64)
	if err := Run(append(append([]string{}, base...), "-OnboardingPublicationStamp", plan.PublicationStamp, "-ExpectedOnboardingPlanSha256", badHash, "-Apply"), &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "hash mismatch") {
		t.Fatalf("hash drift error = %v", err)
	}
	drift := append([]string{}, base...)
	for i := range drift {
		if drift[i] == "opaque goal" {
			drift[i] = "changed goal"
		}
	}
	if err := Run(append(drift, "-OnboardingPublicationStamp", plan.PublicationStamp, "-ExpectedOnboardingPlanSha256", plan.OnboardingPlanSHA256, "-Apply"), &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "hash mismatch") {
		t.Fatalf("input drift error = %v", err)
	}
	if _, err := os.Lstat(caseRoot); !os.IsNotExist(err) {
		t.Fatalf("drift rejection wrote target: %v", err)
	}
}
