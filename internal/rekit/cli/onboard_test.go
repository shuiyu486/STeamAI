package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	"github.com/shuiyu486/re-context-kits/internal/rekit/missionintent"
	"github.com/shuiyu486/re-context-kits/internal/rekit/plancontract"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	syncreview "github.com/shuiyu486/re-context-kits/internal/rekit/sync"
)

type onboardCLIPlan struct {
	Command              string   `json:"command"`
	CaseRoot             string   `json:"caseRoot"`
	ProjectID            string   `json:"projectId"`
	PublicationStamp     string   `json:"publicationStamp"`
	OnboardingPlanSHA256 string   `json:"onboardingPlanSha256"`
	ApplyCommand         string   `json:"applyCommand"`
	ApplyArgs            []string `json:"applyArgs"`
}

func retargetOnboardCLIApplyArgs(t *testing.T, args []string, target string) []string {
	t.Helper()
	out := append([]string{}, args...)
	targets := 0
	for index := 0; index < len(out); index++ {
		if out[index] != "-Target" {
			continue
		}
		if index+1 >= len(out) {
			t.Fatal("onboard ApplyArgs contains a missing -Target value")
		}
		out[index+1] = target
		targets++
		index++
	}
	if targets != 1 {
		t.Fatalf("onboard ApplyArgs contains %d -Target selectors: %v", targets, args)
	}
	return out
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
	if plan.Command != "onboard" || plan.CaseRoot != caseRoot || len(plan.ProjectID) != 16 || plan.OnboardingPlanSHA256 == "" || !strings.HasPrefix(plan.ApplyCommand, "/steamai onboard ") || !strings.Contains(plan.ApplyCommand, plan.PublicationStamp) || len(plan.ApplyArgs) == 0 {
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
	var applied struct {
		CaseRoot   string                   `json:"caseRoot"`
		ProjectID  string                   `json:"projectId"`
		Applied    bool                     `json:"applied"`
		Replay     bool                     `json:"replay"`
		Inspection missionintent.Inspection `json:"inspection"`
	}
	if err := json.Unmarshal(out.Bytes(), &applied); err != nil {
		t.Fatal(err)
	}
	if !applied.Applied || applied.Replay || applied.ProjectID != plan.ProjectID || applied.Inspection.Identity.SchemaVersion != 2 || applied.Inspection.Identity.Target != "." || applied.Inspection.Identity.ProjectID != plan.ProjectID {
		t.Fatalf("onboard Apply did not commit the previewed v2 identity: %+v", applied)
	}
	originalCaseRoot := caseRoot
	caseRoot = filepath.Join(filepath.Dir(originalCaseRoot), "moved-onboard-case")
	if err := os.Rename(originalCaseRoot, caseRoot); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(originalCaseRoot); !os.IsNotExist(err) {
		t.Fatalf("old onboarding physical root remains after move: %v", err)
	}
	apply = retargetOnboardCLIApplyArgs(t, apply, caseRoot)
	boardPath, err := projectstate.Join(caseRoot, "board.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(boardPath); !os.IsNotExist(err) {
		t.Fatalf("onboard created board: %v", err)
	}
	out.Reset()
	var overviewStatus struct {
		Onboarding            missionintent.Inspection             `json:"onboarding"`
		MissionControlRunbook *statusMissionControlRunbookSnapshot `json:"missionControlRunbook"`
	}
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out.Bytes(), &overviewStatus); err != nil {
		t.Fatal(err)
	}
	if overviewStatus.Onboarding.State != "committed" || !overviewStatus.Onboarding.Committed ||
		overviewStatus.Onboarding.Identity != applied.Inspection.Identity || overviewStatus.Onboarding.ProjectBinding == nil ||
		overviewStatus.Onboarding.ProjectBinding.ProjectID != plan.ProjectID || overviewStatus.MissionControlRunbook == nil ||
		overviewStatus.MissionControlRunbook.Quickstart == nil || !strings.Contains(overviewStatus.MissionControlRunbook.Quickstart.Command, "/steamai overview ") {
		t.Fatalf("moved status did not expose the same committed v2 identity and overview route: %+v\n%s", overviewStatus, out.String())
	}
	takeover := overviewStatus.MissionControlRunbook.ReplacementExecutorTakeoverPackage
	missionIntentRel, err := projectstate.Rel(caseRoot, "mission-intent.json")
	if err != nil {
		t.Fatal(err)
	}
	if takeover == nil || !containsSubstring(takeover.TargetDocuments, missionIntentRel) {
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
	if startRequest == nil || !strings.Contains(startRequest.Command, "/steamai start ") || !strings.Contains(startRequest.Command, ` analysis `) || !strings.Contains(startRequest.Command, `-Executor executor-a`) || !strings.Contains(startRequest.Command, `-Actor operator`) {
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
	lanePath, err := projectstate.Join(caseRoot, "lanes", "feature-analysis", "lane.json")
	if err != nil {
		t.Fatal(err)
	}
	laneBytes, err := os.ReadFile(lanePath)
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
	caseRoot := filepath.Join(t.TempDir(), "binary-re-onboard-case")
	base := []string{"-Command", "onboard", "-Target", caseRoot, "-Pack", defaults.DefaultPack, "-ProjectName", "binary-re-journey", "-Goal", "opaque default-pack goal", "-Actor", "operator", "-Executor", "executor-a", "-InitialLane", "binary-analysis-live-check"}
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
	selectedStatusArgs := append(append([]string{}, statusArgs...), "-Lane", "binary-analysis-live-check")
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
	if startRequest == nil || startRequest.Lane != "binary-analysis-live-check" || startRequest.Label != "live-check" {
		t.Fatalf("default-pack status did not emit the exact round-trip start route: %+v", startRequest)
	}
	startInvocation, err := commands.ParsePublicInvocation(startRequest.Command)
	if err != nil {
		t.Fatalf("default-pack status emitted an invalid typed start route: %v", err)
	}
	wantStartInvocation, err := commands.NewPublicInvocation(
		commands.Start,
		"-Target", caseRoot,
		"live-check",
		"-Executor", "executor-a",
		"-Actor", "operator",
		"-WhatIf",
		"-Format", "json",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !startInvocation.Equivalent(wantStartInvocation) {
		t.Fatalf("default-pack status start route drifted: got=%+v want=%+v", startInvocation, wantStartInvocation)
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

	lanePath, err := projectstate.Join(caseRoot, "lanes", "binary-analysis-live-check", "lane.json")
	if err != nil {
		t.Fatal(err)
	}
	laneBytes, err := os.ReadFile(lanePath)
	if err != nil {
		t.Fatal(err)
	}
	var lane struct {
		ID                 string   `json:"id"`
		Type               string   `json:"type"`
		Name               string   `json:"name"`
		CurrentExecutor    string   `json:"currentExecutor"`
		ExecutorGeneration int      `json:"executorGeneration"`
		ReadOnly           []string `json:"readOnly"`
	}
	if err := json.Unmarshal(laneBytes, &lane); err != nil {
		t.Fatal(err)
	}
	if lane.ID != "binary-analysis-live-check" || lane.Type != "binary-analysis" || lane.Name != "live-check" || lane.CurrentExecutor != "executor-a" || lane.ExecutorGeneration != 1 {
		t.Fatalf("default-pack initial lane did not preserve exact identity and owner: %+v", lane)
	}
	if strings.Join(lane.ReadOnly, ",") != "references/binary-re/**,.steamai/facts/**" {
		t.Fatalf("default-pack current lane did not project read-only state to .steamai: %+v", lane.ReadOnly)
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
	if current == nil || current.Lane != "binary-analysis-live-check" || !strings.Contains(current.Command, "-Lane binary-analysis-live-check") || !strings.Contains(current.Command, "-Executor executor-a -ExpectedExecutorGeneration 1") {
		t.Fatalf("default-pack status did not focus the owned initial feature lane after exact lane creation: %+v", current)
	}
	if current.Source == "committedMissionIntent" || strings.Contains(current.Command, "/steamai start ") {
		t.Fatalf("default-pack status repeated committed mission-intent bootstrap after exact lane creation: %+v", current)
	}

	loop := runCurrentLoopResult(t, []string{"-Command", "run-current-loop", "-Target", caseRoot, "-Pack", defaults.DefaultPack, "-MaxSteps", "2", "-Lane", "binary-analysis-live-check", "-WhatIf", "-Format", "json"})
	if loop.InitialCurrentStep == nil || loop.InitialCurrentStep.MemberExecution == nil || loop.ExpectedCurrentLoopPlanSHA256 == "" {
		t.Fatalf("default-pack current-loop preview omitted the owned member dispatch: %+v", loop)
	}
	memberPlan := loop.InitialCurrentStep.MemberExecution
	appliedLoop := runCurrentLoopResult(t, []string{
		"-Command", "run-current-loop", "-Target", caseRoot, "-Pack", defaults.DefaultPack, "-MaxSteps", "2",
		"-Lane", "binary-analysis-live-check",
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

func TestRunStatusPublishesReadOnlyOnboardingPackChoices(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "fresh-project")
	if err := os.MkdirAll(caseRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	before := snapshotFiles(t, caseRoot)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "binary-re", "-Format", "compact-json"}, &out); err != nil {
		t.Fatal(err)
	}
	var status struct {
		Mode       string `json:"mode"`
		Onboarding struct {
			State        string `json:"state"`
			SelectedPack string `json:"selectedPack"`
			PackChoices  []struct {
				ID         string `json:"id"`
				Maturity   string `json:"maturity"`
				Selected   bool   `json:"selected"`
				Selectable bool   `json:"selectable"`
			} `json:"packChoices"`
		} `json:"onboarding"`
		MissionControlRunbook *statusMissionControlRunbookSnapshot `json:"missionControlRunbook"`
	}
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.Mode != "case-onboarding-required" || status.Onboarding.State != "absent" || status.Onboarding.SelectedPack != "binary-re" || status.MissionControlRunbook == nil || status.MissionControlRunbook.CurrentDriverRequest != nil {
		t.Fatalf("fresh project status omitted the non-executable onboarding choice state: %+v\n%s", status, out.String())
	}
	byID := map[string]struct {
		Maturity   string
		Selected   bool
		Selectable bool
	}{}
	for _, choice := range status.Onboarding.PackChoices {
		byID[choice.ID] = struct {
			Maturity   string
			Selected   bool
			Selectable bool
		}{choice.Maturity, choice.Selected, choice.Selectable}
	}
	if binary, ok := byID["binary-re"]; !ok || binary.Maturity != "mature" || !binary.Selected || !binary.Selectable {
		t.Fatalf("fresh project pack choices omitted selected production pack: %+v", byID)
	}
	if _, ok := byID["_template"]; ok {
		t.Fatalf("fresh project pack choices exposed authoring template: %+v", byID)
	}
	assertSnapshotEqual(t, before, snapshotFiles(t, caseRoot))
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
	paths, err := missionintent.Paths(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	restore := syncreview.SetExclusiveInitLeafWriteHookForTest(func(stage, path string) error {
		if stage == "before-publish" && path != paths.Intent {
			return os.ErrClosed
		}
		return nil
	})
	if err := Run(plan.ApplyArgs, &bytes.Buffer{}); err == nil {
		t.Fatal("hooked onboard unexpectedly completed")
	}
	restore()
	instancePath, err := projectstate.Join(caseRoot, "instance.yml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(instancePath); !os.IsNotExist(err) {
		t.Fatalf("partial status fixture unexpectedly has instance metadata: %v", err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var status struct {
		Mode       string `json:"mode"`
		Onboarding struct {
			State        string   `json:"state"`
			SelectedPack string   `json:"selectedPack"`
			PackChoices  []any    `json:"packChoices"`
			ApplyArgs    []string `json:"applyArgs"`
		} `json:"onboarding"`
		CaseMission struct {
			MissionCommanderActionQueue missionCommanderActionQueueSnapshot `json:"missionCommanderActionQueue"`
		} `json:"caseMission"`
		MissionControlRunbook *statusMissionControlRunbookSnapshot `json:"missionControlRunbook"`
	}
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	request := status.MissionControlRunbook.CurrentDriverRequest
	if status.Mode != "case-onboarding-pending" || status.Onboarding.State != "pending" || status.Onboarding.SelectedPack != "_template" || len(status.Onboarding.PackChoices) != 0 || len(status.Onboarding.ApplyArgs) == 0 || request == nil || !request.CommandExecutable || !request.RequiresReview || request.State != "onboarding-apply-ready" || request.Source != "onboarding.pendingPublication" || status.CaseMission.MissionCommanderActionQueue.Counts.Total != 1 || status.CaseMission.MissionCommanderActionQueue.CurrentDriverRequest == nil || !publicCommandsEquivalent(request.Command, status.CaseMission.MissionCommanderActionQueue.CurrentDriverRequest.Command) {
		t.Fatalf("fresh pending status omitted its single exact recovery request: %+v\n%s", status, out.String())
	}
	exact, err := commands.ExactActionFromCLIArgs(status.Onboarding.ApplyArgs)
	if err != nil {
		t.Fatal(err)
	}
	projected, err := commands.ParsePublicInvocation(request.Command)
	if err != nil || !exact.Invocation.Equivalent(projected) {
		t.Fatalf("pending status request drifted from durable applyArgs: request=%+v args=%+v err=%v", request, status.Onboarding.ApplyArgs, err)
	}

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "compact-json"}, &out); err != nil {
		t.Fatal(err)
	}
	var compactRaw map[string]any
	if err := json.Unmarshal(out.Bytes(), &compactRaw); err != nil {
		t.Fatal(err)
	}
	onboardingRaw, ok := compactRaw["onboarding"].(map[string]any)
	if !ok {
		t.Fatalf("compact onboarding is not an object: %s", out.String())
	}
	for _, forbidden := range []string{"onboardingPlanSha256", "publicationStamp", "applyArgs", "identity", "projectBinding"} {
		if _, present := onboardingRaw[forbidden]; present {
			t.Fatalf("compact onboarding duplicated diagnostic field %q: %s", forbidden, out.String())
		}
	}
	var compact statusCompactInventory
	if err := json.Unmarshal(out.Bytes(), &compact); err != nil {
		t.Fatal(err)
	}
	if compact.MissionControlRunbook == nil || compact.MissionControlRunbook.CurrentDriverRequest == nil || !publicCommandsEquivalent(compact.MissionControlRunbook.CurrentDriverRequest.Command, request.Command) || compact.Onboarding == nil || compact.Onboarding.SelectedPack != "_template" || len(compact.Onboarding.PackChoices) != 0 {
		t.Fatalf("compact pending status omitted canonical request or leaked choices: %+v", compact)
	}

	applyArgs := rekitCommandCLIArgs(t, request.Command)
	out.Reset()
	if err := Run(applyArgs, &out); err != nil {
		t.Fatalf("pending status exact recovery request failed: args=%+v err=%v\n%s", applyArgs, err, out.String())
	}
	var applied struct {
		Applied    bool                     `json:"applied"`
		Inspection missionintent.Inspection `json:"inspection"`
	}
	if err := json.Unmarshal(out.Bytes(), &applied); err != nil {
		t.Fatal(err)
	}
	if !applied.Applied || !applied.Inspection.Committed || applied.Inspection.State != "committed" {
		t.Fatalf("pending status exact recovery request did not commit onboarding: %+v", applied)
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
	if !slices.Contains(continueApplyArgs, "-Apply") || slices.Contains(continueApplyArgs, "-WhatIf") {
		t.Fatalf("onboard journey continue preview did not return Apply-only request: %+v", continueApplyArgs)
	}
	continueApplyArgs = append(continueApplyArgs, "-Pack", "_template")
	runCompletionJSON(t, &out, continueApplyArgs, nil)

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

	featureEvidenceRel := projectStateRel(t, caseRoot, "lanes", "feature-analysis", "workspace", "completion-evidence.md")
	featureEvidence := filepath.Join(caseRoot, filepath.FromSlash(featureEvidenceRel))
	writeCompletionEvidence(t, featureEvidence, "reviewed initial analysis closure")
	featureFirst := applyCompletion(t, &out, caseRoot, previewCompletion(t, &out, caseRoot, "analysis", featureEvidenceRel))
	mainEvidenceRel := projectStateRel(t, caseRoot, "lanes", "main", "workspace", "completion-evidence.md")
	mainEvidence := filepath.Join(caseRoot, filepath.FromSlash(mainEvidenceRel))
	writeCompletionEvidence(t, mainEvidence, "reviewed initial aggregate closure")
	mainFirst := applyCompletion(t, &out, caseRoot, previewCompletion(t, &out, caseRoot, "main", mainEvidenceRel))
	if featureFirst.CompletionReceipt == nil || mainFirst.CompletionReceipt == nil || featureFirst.CompletionReceipt.Sequence != 1 || mainFirst.CompletionReceipt.Sequence != 1 {
		t.Fatalf("first public journey completion receipts missing: feature=%+v main=%+v", featureFirst, mainFirst)
	}
	runCompletionJSON(t, &out, []string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &status)
	if status.CaseMission == nil || status.CaseMission.MissionCompletion == nil || status.CaseMission.MissionCompletion.State != "mission-complete" || !status.CaseMission.MissionCompletion.OperationallyComplete || status.CaseMission.MissionCompletion.CompletedLaneCount != 2 || status.CaseMission.MissionCompletion.OpenLaneCount != 0 {
		t.Fatalf("first public journey closure did not reach terminal state: %+v", status.CaseMission)
	}

	reopenEvidenceRel := projectStateRel(t, caseRoot, "reopen-evidence.md")
	reopenEvidence := filepath.Join(caseRoot, filepath.FromSlash(reopenEvidenceRel))
	writeCompletionEvidence(t, reopenEvidence, "review found additional analysis work")
	var reopenPreview reopenProductResult
	runCompletionJSON(t, &out, []string{"-Command", "reopen", "analysis", "-Target", caseRoot, "-Pack", "_template", "-Actor", "operator", "-Reason", "review requires fresh analysis evidence", "-EvidenceRefs", reopenEvidenceRel, "-WhatIf", "-Format", "json"}, &reopenPreview)
	var reopened reopenProductResult
	runCompletionJSON(t, &out, append(rekitCommandCLIArgs(t, reopenPreview.ApplyCommand), "-Target", caseRoot, "-Pack", "_template"), &reopened)
	if !reopened.Applied || reopened.OperationCommit == nil || !reopened.OperationCommit.NoAuthority || !reopened.OperationCommit.NoConfirmed {
		t.Fatalf("public journey reopen receipt is not truthful: %+v", reopened)
	}
	runCompletionJSON(t, &out, []string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &status)
	if status.MissionControlRunbook == nil || status.MissionControlRunbook.Quickstart == nil || status.MissionControlRunbook.Quickstart.CurrentDriverRequest == nil {
		t.Fatalf("fresh status omitted post-reopen route: %+v", status)
	}

	featureEvidenceRel2 := projectStateRel(t, caseRoot, "lanes", "feature-analysis", "workspace", "completion-evidence-2.md")
	featureEvidence2 := filepath.Join(caseRoot, filepath.FromSlash(featureEvidenceRel2))
	writeCompletionEvidence(t, featureEvidence2, "fresh analysis closure after reopen")
	featureSecond := applyCompletion(t, &out, caseRoot, previewCompletion(t, &out, caseRoot, "analysis", featureEvidenceRel2))
	mainEvidenceRel2 := projectStateRel(t, caseRoot, "lanes", "main", "workspace", "completion-evidence-2.md")
	mainEvidence2 := filepath.Join(caseRoot, filepath.FromSlash(mainEvidenceRel2))
	writeCompletionEvidence(t, mainEvidence2, "fresh aggregate closure after reopen")
	mainSecond := applyCompletion(t, &out, caseRoot, previewCompletion(t, &out, caseRoot, "main", mainEvidenceRel2))
	if featureSecond.CompletionReceipt == nil || featureSecond.CompletionReceipt.Sequence != 3 || mainSecond.CompletionReceipt == nil || mainSecond.CompletionReceipt.Sequence != 3 {
		t.Fatalf("public journey recompletion generation drifted: feature=%+v main=%+v", featureSecond.CompletionReceipt, mainSecond.CompletionReceipt)
	}
	runCompletionJSON(t, &out, []string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &status)
	if status.CaseMission == nil || status.CaseMission.MissionCompletion == nil || status.CaseMission.MissionCompletion.State != "mission-complete" || !status.CaseMission.MissionCompletion.OperationallyComplete || status.CaseMission.MissionCompletion.CompletedLaneCount != 2 || status.CaseMission.MissionCompletion.OpenLaneCount != 0 {
		t.Fatalf("public journey recompletion did not restore terminal state: %+v", status.CaseMission)
	}
	for _, name := range []string{"authority.jsonl", "confirmed.jsonl"} {
		path := projectStatePath(t, caseRoot, "facts", name)
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("public journey wrote forbidden %s: %v", path, err)
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
	if err := Run(append(append([]string{}, args...), "-ProjectId", plan.ProjectID, "-OnboardingPublicationStamp", plan.PublicationStamp, "-ExpectedOnboardingPlanSha256", plan.OnboardingPlanSHA256, "-Apply"), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	paths, err := missionintent.Paths(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(caseRoot, filepath.FromSlash(paths.Commit)), []byte("{}\n"), 0o644); err != nil {
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

func TestRunStatusRejectsRelocatedOldCurrentV1Onboarding(t *testing.T) {
	base := t.TempDir()
	originalRoot := filepath.Join(base, "old-current-v1")
	args := []string{"-Command", "onboard", "-Target", originalRoot, "-Pack", "_template", "-ProjectName", "demo", "-Goal", "opaque goal", "-Actor", "operator", "-Executor", "executor-a", "-InitialLane", "feature-analysis"}
	var out bytes.Buffer
	if err := Run(append(append([]string{}, args...), "-WhatIf", "-Format", "json"), &out); err != nil {
		t.Fatal(err)
	}
	var plan onboardCLIPlan
	if err := json.Unmarshal(out.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run(plan.ApplyArgs, &out); err != nil {
		t.Fatal(err)
	}
	writeOldCurrentV1Onboarding(t, originalRoot)
	originalInspection, err := missionintent.Inspect(originalRoot)
	if err != nil || !originalInspection.Committed || originalInspection.Identity.SchemaVersion != 1 || originalInspection.Identity.Target != originalRoot {
		t.Fatalf("old current v1 fixture is not committed at its original root: inspection=%+v err=%v", originalInspection, err)
	}

	movedRoot := filepath.Join(base, "moved-old-current-v1")
	if err := os.Rename(originalRoot, movedRoot); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", movedRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatalf("relocated old current v1 status must remain readable: %v", err)
	}
	var status struct {
		Mode string `json:"mode"`
		Case struct {
			Moved bool `json:"moved"`
		} `json:"case"`
		Onboarding  missionintent.Inspection `json:"onboarding"`
		CaseMission *struct {
			Ready   bool   `json:"ready"`
			Summary string `json:"summary"`
		} `json:"caseMission"`
	}
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.Mode != "case" || status.Case.Moved || status.Onboarding.State != "corrupt" || status.Onboarding.Committed ||
		status.CaseMission == nil || status.CaseMission.Ready ||
		!strings.Contains(status.CaseMission.Summary, "mission intent Target does not match inspected case root") ||
		!strings.Contains(out.String(), "ordinary attached commands are blocked") {
		t.Fatalf("relocated old current v1 status did not fail closed specifically on onboarding identity: %+v\n%s", status, out.String())
	}
	if err := Run([]string{"-Command", "overview", "-Target", movedRoot, "-Pack", "_template", "-Format", "json"}, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "onboarding state is corrupt") {
		t.Fatalf("relocated old current v1 mutation error = %v", err)
	}
}

func writeOldCurrentV1Onboarding(t *testing.T, caseRoot string) {
	t.Helper()
	const stamp = "20260817-010203004"
	planSHA256 := strings.Repeat("a", 64)
	identity := missionintent.Identity{
		SchemaVersion: 1,
		Target:        caseRoot,
		Pack:          "_template",
		ProjectName:   "demo",
		Goal:          "opaque goal",
		Actor:         "operator",
		Executor:      "executor-a",
		InitialLane:   "feature-analysis",
	}
	missionBytes, err := missionintent.MarshalMissionIntent(identity)
	if err != nil {
		t.Fatal(err)
	}
	recovery := missionintent.RecoveryEnvelope{
		SchemaVersion: 1,
		RepoRoot:      ".",
		CreatedAt:     "2026-08-17T01:02:03.004Z",
		Mode:          "attached-adoption",
		AttachedSnapshot: []missionintent.SnapshotArtifact{
			{Path: ".claude/skills/steamai/SKILL.md", Kind: "project-local-steamai-skill", SHA256: strings.Repeat("1", 64), Size: 1},
			{Path: ".steamai/instance.yml", Kind: "instance-metadata", SHA256: strings.Repeat("2", 64), Size: 1},
			{Path: ".steamai/state.json", Kind: "sync-state", SHA256: strings.Repeat("3", 64), Size: 1},
		},
	}
	intentBytes, err := missionintent.MarshalIntent(missionintent.Intent{
		SchemaVersion:        1,
		Kind:                 "mission-onboarding-intent",
		PublicationStamp:     stamp,
		OnboardingPlanSHA256: planSHA256,
		Identity:             identity,
		Recovery:             recovery,
	})
	if err != nil {
		t.Fatal(err)
	}
	commitBytes, err := missionintent.MarshalCommit(missionintent.Commit{
		SchemaVersion:        1,
		Kind:                 "mission-onboarding-commit",
		PublicationStamp:     stamp,
		OnboardingPlanSHA256: planSHA256,
		MissionIntentSHA256:  missionintent.SHA256(missionBytes),
		IntentSHA256:         missionintent.SHA256(intentBytes),
	})
	if err != nil {
		t.Fatal(err)
	}
	paths, err := missionintent.Paths(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	for path, data := range map[string][]byte{
		paths.MissionIntent: missionBytes,
		paths.Intent:        intentBytes,
		paths.Commit:        commitBytes,
	} {
		if err := os.WriteFile(filepath.Join(caseRoot, filepath.FromSlash(path)), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Remove(filepath.Join(caseRoot, filepath.FromSlash(paths.ProjectBinding))); err != nil {
		t.Fatal(err)
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
	err := Run(append(append([]string{}, base...), "-ProjectId", plan.ProjectID, "-OnboardingPublicationStamp", plan.PublicationStamp, "-ExpectedOnboardingPlanSha256", badHash, "-Apply"), &bytes.Buffer{})
	failure, typed := plancontract.FromError(err)
	if err == nil || !typed || failure.Code != plancontract.CodePlanMismatch || failure.MutationApplied || failure.MutationBoundary != "none" {
		t.Fatalf("hash drift error = %v failure=%+v typed=%t", err, failure, typed)
	}
	drift := append([]string{}, base...)
	for i := range drift {
		if drift[i] == "opaque goal" {
			drift[i] = "changed goal"
		}
	}
	err = Run(append(drift, "-ProjectId", plan.ProjectID, "-OnboardingPublicationStamp", plan.PublicationStamp, "-ExpectedOnboardingPlanSha256", plan.OnboardingPlanSHA256, "-Apply"), &bytes.Buffer{})
	failure, typed = plancontract.FromError(err)
	if err == nil || !typed || failure.Code != plancontract.CodePlanMismatch || failure.MutationApplied || failure.MutationBoundary != "none" {
		t.Fatalf("input drift error = %v failure=%+v typed=%t", err, failure, typed)
	}
	if _, err := os.Lstat(caseRoot); !os.IsNotExist(err) {
		t.Fatalf("drift rejection wrote target: %v", err)
	}
}
