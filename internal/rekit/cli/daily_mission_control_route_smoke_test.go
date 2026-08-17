package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

func TestRunDailyMissionControlRouteSmokeProductPath(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer

	var onboardingStatus struct {
		MissionControlRunbook *statusMissionControlRunbookSnapshot `json:"missionControlRunbook"`
	}
	runDailyMissionControlRouteJSON(t, &out, []string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &onboardingStatus)
	onboardingRunbook := onboardingStatus.MissionControlRunbook
	if onboardingRunbook == nil || onboardingRunbook.Quickstart == nil || !onboardingRunbook.Quickstart.Ready || !onboardingRunbook.Quickstart.CommandExecutable || onboardingRunbook.Quickstart.CurrentDriverRequest == nil || !strings.Contains(onboardingRunbook.Quickstart.Command, "/rekit overview -Target ") {
		t.Fatalf("daily route should start from executable case-onboarding quickstart: %+v", onboardingRunbook)
	}
	if _, err := os.Stat(filepath.Join(caseRoot, ".rekit", "board.json")); !os.IsNotExist(err) {
		t.Fatalf("onboarding status should stay read-only, err=%v", err)
	}
	onboardingArgs, ok := missionCommanderDriverRequestCommandCLIArgs(t, onboardingRunbook.Quickstart.CurrentDriverRequest)
	if !ok {
		t.Fatalf("case-onboarding quickstart should be executable: %+v", onboardingRunbook.Quickstart.CurrentDriverRequest)
	}
	out.Reset()
	if err := Run(onboardingArgs, &out); err != nil {
		t.Fatalf("case-onboarding quickstart failed: args=%+v err=%v\n%s", onboardingArgs, err, out.String())
	}
	assertFileExists(t, filepath.Join(caseRoot, ".rekit", "board.json"))

	var readyStatus struct {
		MissionControlRunbook *statusMissionControlRunbookSnapshot `json:"missionControlRunbook"`
	}
	runDailyMissionControlRouteJSON(t, &out, []string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &readyStatus)
	readyRunbook := readyStatus.MissionControlRunbook
	if readyRunbook == nil || readyRunbook.Quickstart == nil || readyRunbook.Quickstart.CurrentDriverRequest == nil || readyRunbook.Quickstart.DriverKind != "preview-command" || !readyRunbook.Quickstart.CommandExecutable || !strings.Contains(readyRunbook.Quickstart.Command, "/rekit continue -Target ") || !strings.Contains(readyRunbook.Quickstart.Command, " main") || !strings.Contains(readyRunbook.Quickstart.Command, "-WhatIf -Format json") {
		t.Fatalf("onboarding refresh should advance to main continue preview: %+v", readyRunbook)
	}
	beforeContinuePreview := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	continuePreviewArgs, ok := missionCommanderDriverRequestCommandCLIArgs(t, readyRunbook.Quickstart.CurrentDriverRequest)
	if !ok {
		t.Fatalf("main continue quickstart should be executable: %+v", readyRunbook.Quickstart.CurrentDriverRequest)
	}
	var continuePreview struct {
		Command                     string                              `json:"command"`
		Applied                     bool                                `json:"applied"`
		Blocked                     bool                                `json:"blocked"`
		MissionCommanderActionQueue missionCommanderActionQueueSnapshot `json:"missionCommanderActionQueue"`
	}
	runDailyMissionControlRouteJSON(t, &out, continuePreviewArgs, &continuePreview)
	continueApplyRequest := requireMissionCommanderDriverRequest(t, continuePreview.MissionCommanderActionQueue, "execute-command", "apply-or-run-current", "/rekit continue main", true, false, false)
	if continuePreview.Command != "continue" || continuePreview.Applied || continuePreview.Blocked || continueApplyRequest.Command == readyRunbook.Quickstart.Command {
		t.Fatalf("continue preview should return a case-local apply request: preview=%+v request=%+v", continuePreview, continueApplyRequest)
	}
	assertSnapshotEqual(t, beforeContinuePreview, snapshotFiles(t, filepath.Join(caseRoot, ".rekit")))

	continueApplyArgs, ok := missionCommanderDriverRequestCommandCLIArgs(t, continueApplyRequest)
	if !ok {
		t.Fatalf("continue preview returned request should be executable: %+v", continueApplyRequest)
	}
	continueApplyArgs = append(continueApplyArgs, "-Target", caseRoot, "-Pack", "_template", "-Apply", "-Format", "json")
	var continued struct {
		Command                       string                                 `json:"command"`
		RunID                         string                                 `json:"runId"`
		BatchID                       string                                 `json:"batchId"`
		Applied                       bool                                   `json:"applied"`
		Blocked                       bool                                   `json:"blocked"`
		MissionCommanderActionQueue   missionCommanderActionQueueSnapshot    `json:"missionCommanderActionQueue"`
		MissionCommanderDriverReceipt *missionCommanderDriverReceiptSnapshot `json:"missionCommanderDriverReceipt"`
		Writes                        []startWrite                           `json:"writes"`
	}
	runDailyMissionControlRouteJSON(t, &out, continueApplyArgs, &continued)
	continuedRequest := requireMissionCommanderDriverRequest(t, continued.MissionCommanderActionQueue, "execute-command", "apply-or-run-current", "/rekit continue main", true, false, false)
	if continued.Command != "continue" || !continued.Applied || continued.Blocked || continued.RunID == "" || continued.BatchID != "batch-"+continued.RunID || continued.MissionCommanderDriverReceipt == nil || continued.MissionCommanderDriverReceipt.RefreshedCurrentDriverRequest == nil || continued.MissionCommanderDriverReceipt.RefreshedCurrentDriverRequest.Command != continuedRequest.Command {
		t.Fatalf("continue apply should persist a durable run receipt: result=%+v request=%+v", continued, continuedRequest)
	}
	statusPath := assertStartWrite(t, continued.Writes, ".rekit/runs/"+continued.RunID+"/status.json", "write").TargetPath
	digestPath := assertStartWrite(t, continued.Writes, ".rekit/runs/"+continued.RunID+"/digest.md", "write").TargetPath
	statusBytes, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatal(err)
	}
	var runStatus struct {
		RunID                         string                                 `json:"runId"`
		BatchID                       string                                 `json:"batchId"`
		MissionCommanderDriverReceipt *missionCommanderDriverReceiptSnapshot `json:"missionCommanderDriverReceipt"`
	}
	if err := json.Unmarshal(statusBytes, &runStatus); err != nil {
		t.Fatalf("daily route run status did not decode: %v\n%s", err, string(statusBytes))
	}
	if runStatus.RunID != continued.RunID || runStatus.BatchID != continued.BatchID || runStatus.MissionCommanderDriverReceipt == nil || runStatus.MissionCommanderDriverReceipt.RefreshedCurrentDriverRequest == nil || runStatus.MissionCommanderDriverReceipt.RefreshedCurrentDriverRequest.Command != continuedRequest.Command || !containsSubstring(runStatus.MissionCommanderDriverReceipt.Boundary, "continue -Apply does not write authority/confirmed state or execute heavy tools") {
		t.Fatalf("daily route run status omitted durable receipt identity or boundary: %+v", runStatus)
	}
	digestBytes, err := os.ReadFile(digestPath)
	if err != nil {
		t.Fatal(err)
	}
	digestText := string(digestBytes)
	for _, want := range []string{"## Mission Commander driver receipt", "- runId: `" + continued.RunID + "`", "- batchId: `" + continued.BatchID + "`", "- command: `" + continuedRequest.Command + "`", "continue -Apply does not write authority/confirmed state or execute heavy tools"} {
		if !strings.Contains(digestText, want) {
			t.Fatalf("daily route run digest omitted %q:\n%s", want, digestText)
		}
	}

	noteArgs := []string{"-Command", "note", "-Target", caseRoot, "-Pack", "_template", "-Kind", "intervention", "-Lane", "main", "-Subject", "manual route correction", "-Summary", "operator redirected the current mission", "-Action", "override", "-Status", "open", "-Actor", "lead", "-EventId", "int-daily-route-1", "-CreatedAt", "2026-08-03T00:00:00Z", "-Format", "json"}
	out.Reset()
	var notePreview struct {
		Applied       bool   `json:"applied"`
		EventSHA256   string `json:"eventSha256"`
		RecordCommand string `json:"recordCommand"`
	}
	runDailyMissionControlRouteJSON(t, &out, append(append([]string{}, noteArgs...), "-WhatIf"), &notePreview)
	if notePreview.Applied || notePreview.EventSHA256 == "" || !strings.Contains(notePreview.RecordCommand, "-ExpectedNoteEventSha256 "+notePreview.EventSHA256) {
		t.Fatalf("public note preview omitted exact record route: %+v", notePreview)
	}
	out.Reset()
	if err := Run(rekitCommandCLIArgs(t, notePreview.RecordCommand), &out); err != nil {
		t.Fatal(err)
	}

	var blockedStatus struct {
		MissionControlRunbook *statusMissionControlRunbookSnapshot `json:"missionControlRunbook"`
	}
	runDailyMissionControlRouteJSON(t, &out, []string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &blockedStatus)
	blockedRunbook := blockedStatus.MissionControlRunbook
	if blockedRunbook == nil || blockedRunbook.Quickstart == nil || blockedRunbook.Quickstart.CurrentDriverRequest == nil || !blockedRunbook.Quickstart.RequiresReview || !strings.Contains(blockedRunbook.Quickstart.Command, "/rekit reconcile -Target ") || !strings.Contains(blockedRunbook.Quickstart.Command, " main -InterventionId int-daily-route-1 -WhatIf -Format json") {
		t.Fatalf("open intervention should replace continue with reconcile quickstart: %+v", blockedRunbook)
	}
	beforeReconcilePreview := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	reconcilePreviewArgs, ok := missionCommanderDriverRequestCommandCLIArgs(t, blockedRunbook.Quickstart.CurrentDriverRequest)
	if !ok {
		t.Fatalf("reconcile quickstart should be executable: %+v", blockedRunbook.Quickstart.CurrentDriverRequest)
	}
	var reconcilePreview struct {
		Command                       string                                 `json:"command"`
		Applied                       bool                                   `json:"applied"`
		RequiresConfirmation          bool                                   `json:"requiresConfirmation"`
		MissionCommanderActionQueue   missionCommanderActionQueueSnapshot    `json:"missionCommanderActionQueue"`
		MissionCommanderDriverReceipt *missionCommanderDriverReceiptSnapshot `json:"missionCommanderDriverReceipt"`
	}
	runDailyMissionControlRouteJSON(t, &out, reconcilePreviewArgs, &reconcilePreview)
	reconcileApplyRequest := reconcilePreview.MissionCommanderActionQueue.CurrentDriverRequest
	if reconcilePreview.Command != "reconcile" || reconcilePreview.Applied || !reconcilePreview.RequiresConfirmation || reconcileApplyRequest == nil || !reconcileApplyRequest.CommandExecutable || !reconcileApplyRequest.RequiresReview || !strings.Contains(reconcileApplyRequest.Command, "/rekit reconcile main -InterventionId int-daily-route-1 -Apply") || reconcileApplyRequest.ExpectedReceipt.RefreshStatusCommand != blockedRunbook.Quickstart.RefreshStatusCommand || reconcilePreview.MissionCommanderDriverReceipt == nil || reconcilePreview.MissionCommanderDriverReceipt.RefreshedCurrentDriverRequest == nil || reconcilePreview.MissionCommanderDriverReceipt.RefreshedCurrentDriverRequest.Command != reconcileApplyRequest.Command {
		t.Fatalf("reconcile preview should return typed apply request and refresh route: preview=%+v request=%+v", reconcilePreview, reconcileApplyRequest)
	}
	assertSnapshotEqual(t, beforeReconcilePreview, snapshotFiles(t, filepath.Join(caseRoot, ".rekit")))

	reconcileApplyArgs, ok := missionCommanderDriverRequestCommandCLIArgs(t, reconcileApplyRequest)
	if !ok {
		t.Fatalf("reconcile apply request should be executable: %+v", reconcileApplyRequest)
	}
	reconcileApplyArgs = append(reconcileApplyArgs, "-Target", caseRoot, "-Pack", "_template", "-Format", "json")
	var reconciled struct {
		Command                       string                                 `json:"command"`
		Applied                       bool                                   `json:"applied"`
		ResolutionEventID             string                                 `json:"resolutionEventId"`
		MissionCommanderActionQueue   missionCommanderActionQueueSnapshot    `json:"missionCommanderActionQueue"`
		MissionCommanderDriverReceipt *missionCommanderDriverReceiptSnapshot `json:"missionCommanderDriverReceipt"`
		Writes                        []startWrite                           `json:"writes"`
	}
	runDailyMissionControlRouteJSON(t, &out, reconcileApplyArgs, &reconciled)
	reconciledRequest := requireMissionCommanderDriverRequest(t, reconciled.MissionCommanderActionQueue, "execute-command", "apply-or-run-current", "/rekit continue main -Executor main-agent -ExpectedExecutorGeneration 1", true, false, false)
	if reconciled.Command != "reconcile" || !reconciled.Applied || reconciled.ResolutionEventID == "" || reconciled.MissionCommanderDriverReceipt == nil || reconciled.MissionCommanderDriverReceipt.RefreshedCurrentDriverRequest == nil || reconciled.MissionCommanderDriverReceipt.RefreshedCurrentDriverRequest.Command != reconciledRequest.Command || reconciledRequest.ExpectedReceipt.RefreshStatusCommand != blockedRunbook.Quickstart.RefreshStatusCommand {
		t.Fatalf("reconcile apply should restore the durable continue request: result=%+v request=%+v", reconciled, reconciledRequest)
	}
	assertStartWrite(t, reconciled.Writes, ".rekit/facts/interventions.jsonl", "append")

	var reconciledStatus struct {
		MissionControlRunbook *statusMissionControlRunbookSnapshot `json:"missionControlRunbook"`
	}
	diagnosticsArgs := rekitCommandCLIArgs(t, blockedRunbook.Quickstart.RefreshStatusCommand)
	for idx := range diagnosticsArgs {
		if strings.EqualFold(diagnosticsArgs[idx], "compact-json") {
			diagnosticsArgs[idx] = "json"
		}
	}
	runDailyMissionControlRouteJSON(t, &out, diagnosticsArgs, &reconciledStatus)
	reconciledRunbook := reconciledStatus.MissionControlRunbook
	if reconciledRunbook == nil || reconciledRunbook.Quickstart == nil || reconciledRunbook.Quickstart.CurrentDriverRequest == nil || !strings.Contains(reconciledRunbook.Quickstart.Command, "/rekit continue -Target ") || !strings.Contains(reconciledRunbook.Quickstart.Command, " main -Executor main-agent -ExpectedExecutorGeneration 1 -WhatIf -Format json") || reconciledRunbook.HandoffPreviewDriverRequest == nil {
		t.Fatalf("reconcile refresh should restore continue quickstart and handoff route: %+v", reconciledRunbook)
	}

	if !strings.Contains(reconciledRunbook.HandoffPreviewDriverRequest.Command, `-Lane "main" -WhatIf`) || reconciledRunbook.HandoffApplyDriverRequest != nil || reconciledRunbook.HandoffApplyCommand != "" {
		t.Fatalf("selected status should expose only an exact lane handoff preview: %+v", reconciledRunbook)
	}
	beforeHandoffPreview := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	handoffPreviewArgs := []string{"-Command", "handoff", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Format", "json"}
	var handoffPreview struct {
		Command                    string                              `json:"command"`
		Applied                    bool                                `json:"applied"`
		Project                    bool                                `json:"project"`
		PublicationPlanSHA256      string                              `json:"publicationPlanSha256"`
		PublicationStamp           string                              `json:"publicationStamp"`
		ApplyCommand               string                              `json:"applyCommand"`
		DailyMissionControlRunbook *dailyMissionControlRunbookSnapshot `json:"dailyMissionControlRunbook"`
		Writes                     []startWrite                        `json:"writes"`
	}
	runDailyMissionControlRouteJSON(t, &out, handoffPreviewArgs, &handoffPreview)
	if handoffPreview.Command != "handoff" || handoffPreview.Applied || !handoffPreview.Project || len(handoffPreview.PublicationPlanSHA256) != 64 || handoffPreview.PublicationStamp == "" || !strings.Contains(handoffPreview.ApplyCommand, "-ExpectedHandoffPlanSha256 "+handoffPreview.PublicationPlanSHA256) || !strings.Contains(handoffPreview.ApplyCommand, "-HandoffPublicationStamp "+handoffPreview.PublicationStamp) || handoffPreview.DailyMissionControlRunbook == nil || handoffPreview.DailyMissionControlRunbook.HandoffApplyDriverRequest == nil || !handoffPreview.DailyMissionControlRunbook.HandoffApplyDriverRequest.CommandExecutable || handoffPreview.DailyMissionControlRunbook.HandoffApplyDriverRequest.Command != handoffPreview.ApplyCommand {
		t.Fatalf("handoff preview should return executable exact hash-bound durable handoff apply request: %+v", handoffPreview)
	}
	assertStartWrite(t, handoffPreview.Writes, ".rekit/handovers/latest.md", "would-write-latest-project-handoff")
	assertSnapshotEqual(t, beforeHandoffPreview, snapshotFiles(t, filepath.Join(caseRoot, ".rekit")))

	handoffApplyArgs, ok := missionCommanderDriverRequestCommandCLIArgs(t, handoffPreview.DailyMissionControlRunbook.HandoffApplyDriverRequest)
	if !ok {
		t.Fatalf("handoff apply request should be executable: %+v", handoffPreview.DailyMissionControlRunbook.HandoffApplyDriverRequest)
	}
	var handoffApplied struct {
		Command                            string                                      `json:"command"`
		Applied                            bool                                        `json:"applied"`
		Project                            bool                                        `json:"project"`
		ReplacementExecutorTakeoverPackage *replacementExecutorTakeoverPackageSnapshot `json:"replacementExecutorTakeoverPackage"`
		Writes                             []startWrite                                `json:"writes"`
	}
	out.Reset()
	if err := runHashBoundHandoffApply(t, handoffApplyArgs, &out); err != nil {
		t.Fatalf("daily Mission Control handoff apply failed: args=%+v err=%v", handoffApplyArgs, err)
	}
	if err := json.Unmarshal(out.Bytes(), &handoffApplied); err != nil {
		t.Fatalf("daily Mission Control handoff apply stdout is not JSON: %v\n%s", err, out.String())
	}
	if handoffApplied.Command != "handoff" || !handoffApplied.Applied || !handoffApplied.Project || handoffApplied.ReplacementExecutorTakeoverPackage == nil || !handoffApplied.ReplacementExecutorTakeoverPackage.Ready || handoffApplied.ReplacementExecutorTakeoverPackage.RefreshStatusCommand == "" {
		t.Fatalf("handoff apply should return replacement executor takeover package: %+v", handoffApplied)
	}
	projectHandoffPath := assertStartWrite(t, handoffApplied.Writes, ".rekit/handovers/latest.md", "write-latest-project-handoff").TargetPath
	projectHandoffText, err := os.ReadFile(projectHandoffPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(projectHandoffText), handoffPreview.ApplyCommand) {
		t.Fatalf("durable project handoff omitted exact hash-bound Apply route %q:\n%s", handoffPreview.ApplyCommand, string(projectHandoffText))
	}
	projectArtifactPath := assertStartWrite(t, handoffApplied.Writes, ".rekit/handovers/latest-replacement-executor-takeover.json", "write-latest-replacement-executor-takeover-package").TargetPath
	assertDailyMissionControlTakeoverArtifact(t, projectArtifactPath, handoffApplied.ReplacementExecutorTakeoverPackage.CurrentDriverRequest.Command, handoffApplied.ReplacementExecutorTakeoverPackage.RefreshStatusCommand, ".rekit/handovers/latest-replacement-executor-takeover.json")

	beforeLaneHandoffPreview := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	var laneHandoffPreview struct {
		Command                    string                              `json:"command"`
		Applied                    bool                                `json:"applied"`
		Project                    bool                                `json:"project"`
		DailyMissionControlRunbook *dailyMissionControlRunbookSnapshot `json:"dailyMissionControlRunbook"`
		Writes                     []startWrite                        `json:"writes"`
	}
	runDailyMissionControlRouteJSON(t, &out, []string{"-Command", "handoff", "-Target", caseRoot, "-Pack", "_template", "main", "-WhatIf", "-Format", "json"}, &laneHandoffPreview)
	if laneHandoffPreview.Command != "handoff" || laneHandoffPreview.Applied || laneHandoffPreview.Project || laneHandoffPreview.DailyMissionControlRunbook == nil || laneHandoffPreview.DailyMissionControlRunbook.HandoffApplyDriverRequest == nil || !laneHandoffPreview.DailyMissionControlRunbook.HandoffApplyDriverRequest.CommandExecutable {
		t.Fatalf("lane handoff preview should return executable durable handoff apply request: %+v", laneHandoffPreview)
	}
	assertStartWrite(t, laneHandoffPreview.Writes, ".rekit/handovers/main-latest.md", "would-write-latest-lane-handoff")
	assertSnapshotEqual(t, beforeLaneHandoffPreview, snapshotFiles(t, filepath.Join(caseRoot, ".rekit")))

	laneHandoffApplyArgs, ok := missionCommanderDriverRequestCommandCLIArgs(t, laneHandoffPreview.DailyMissionControlRunbook.HandoffApplyDriverRequest)
	if !ok {
		t.Fatalf("lane handoff apply request should be executable: %+v", laneHandoffPreview.DailyMissionControlRunbook.HandoffApplyDriverRequest)
	}
	var laneHandoffApplied struct {
		Command                            string                                      `json:"command"`
		Applied                            bool                                        `json:"applied"`
		Project                            bool                                        `json:"project"`
		ReplacementExecutorTakeoverPackage *replacementExecutorTakeoverPackageSnapshot `json:"replacementExecutorTakeoverPackage"`
		Writes                             []startWrite                                `json:"writes"`
	}
	runDailyMissionControlRouteJSON(t, &out, laneHandoffApplyArgs, &laneHandoffApplied)
	if laneHandoffApplied.Command != "handoff" || !laneHandoffApplied.Applied || laneHandoffApplied.Project || laneHandoffApplied.ReplacementExecutorTakeoverPackage == nil || !laneHandoffApplied.ReplacementExecutorTakeoverPackage.Ready {
		t.Fatalf("lane handoff apply should return replacement executor takeover package: %+v", laneHandoffApplied)
	}
	laneArtifactPath := assertStartWrite(t, laneHandoffApplied.Writes, ".rekit/handovers/main-latest-replacement-executor-takeover.json", "write-latest-replacement-executor-takeover-package").TargetPath
	laneArtifact := assertDailyMissionControlTakeoverArtifact(t, laneArtifactPath, laneHandoffApplied.ReplacementExecutorTakeoverPackage.CurrentDriverRequest.Command, laneHandoffApplied.ReplacementExecutorTakeoverPackage.RefreshStatusCommand, ".rekit/handovers/main-latest-replacement-executor-takeover.json")

	var takeoverStatus struct {
		MissionControlRunbook *statusMissionControlRunbookSnapshot `json:"missionControlRunbook"`
	}
	takeoverArgs := rekitCommandCLIArgs(t, laneArtifact.RefreshStatusCommand)
	for idx := range takeoverArgs {
		if strings.EqualFold(takeoverArgs[idx], "compact-json") {
			takeoverArgs[idx] = "json"
		}
	}
	runDailyMissionControlRouteJSON(t, &out, takeoverArgs, &takeoverStatus)
	takeoverRunbook := takeoverStatus.MissionControlRunbook
	if takeoverRunbook == nil || takeoverRunbook.Quickstart == nil || takeoverRunbook.Quickstart.CurrentDriverRequest == nil || takeoverRunbook.ReplacementExecutorTakeoverPackage == nil || !takeoverRunbook.ReplacementExecutorTakeoverPackage.Ready || !takeoverRunbook.ReplacementExecutorTakeoverPackage.DurableArtifactFresh || takeoverRunbook.ReplacementExecutorTakeoverPackage.DurableArtifactPath != ".rekit/handovers/main-latest-replacement-executor-takeover.json" || takeoverRunbook.ReplacementExecutorTakeoverPackage.DurableArtifactSHA256 == "" || takeoverRunbook.ReplacementExecutorTakeoverPackage.DurableArtifactRequestSHA256 == "" || takeoverRunbook.ReplacementExecutorTakeoverPackage.DurableArtifactRequestSHA256 != takeoverRunbook.ReplacementExecutorTakeoverPackage.CurrentDriverRequestSHA256 || takeoverRunbook.ReplacementExecutorTakeoverPackage.CurrentDriverRequest.Command != takeoverRunbook.Quickstart.CurrentDriverRequest.Command {
		var takeoverPackage *replacementExecutorTakeoverPackageSnapshot
		if takeoverRunbook != nil {
			takeoverPackage = takeoverRunbook.ReplacementExecutorTakeoverPackage
		}
		t.Fatalf("new-session status should consume the fresh durable takeover route: runbook=%+v package=%+v", takeoverRunbook, takeoverPackage)
	}
	artifactBytes, err := os.ReadFile(laneArtifactPath)
	if err != nil {
		t.Fatal(err)
	}
	var tampered mission.ReplacementExecutorTakeoverPackage
	if err := json.Unmarshal(artifactBytes, &tampered); err != nil {
		t.Fatal(err)
	}
	tampered.CurrentDriverRequest.ExpectedReceipt.Boundary = append(tampered.CurrentDriverRequest.ExpectedReceipt.Boundary, "tampered receipt boundary")
	tamperedBytes, err := json.MarshalIndent(tampered, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(laneArtifactPath, append(tamperedBytes, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	var tamperedStatus struct {
		MissionControlRunbook *statusMissionControlRunbookSnapshot `json:"missionControlRunbook"`
	}
	runDailyMissionControlRouteJSON(t, &out, takeoverArgs, &tamperedStatus)
	tamperedPackage := tamperedStatus.MissionControlRunbook.ReplacementExecutorTakeoverPackage
	if tamperedPackage == nil || tamperedPackage.DurableArtifactFresh || tamperedPackage.DurableArtifactState != "mixed-generation" || tamperedPackage.DurableArtifactRefreshDriverRequest == nil {
		t.Fatalf("tampered nested takeover request should fail closed with a refresh route: %+v", tamperedPackage)
	}

	var packageTampered mission.ReplacementExecutorTakeoverPackage
	if err := json.Unmarshal(artifactBytes, &packageTampered); err != nil {
		t.Fatal(err)
	}
	packageTampered.RunbookSteps = append(packageTampered.RunbookSteps, "tampered takeover instruction")
	packageTamperedBytes, err := json.MarshalIndent(packageTampered, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(laneArtifactPath, append(packageTamperedBytes, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	var packageTamperedStatus struct {
		MissionControlRunbook *statusMissionControlRunbookSnapshot `json:"missionControlRunbook"`
	}
	runDailyMissionControlRouteJSON(t, &out, takeoverArgs, &packageTamperedStatus)
	packageTamperedResult := packageTamperedStatus.MissionControlRunbook.ReplacementExecutorTakeoverPackage
	if packageTamperedResult == nil || packageTamperedResult.DurableArtifactFresh || packageTamperedResult.DurableArtifactState != "mixed-generation" || packageTamperedResult.DurableArtifactRefreshDriverRequest == nil {
		t.Fatalf("tampered takeover package behavior should fail closed with a refresh route: %+v", packageTamperedResult)
	}

	var operatorTampered mission.ReplacementExecutorTakeoverPackage
	if err := json.Unmarshal(artifactBytes, &operatorTampered); err != nil {
		t.Fatal(err)
	}
	if operatorTampered.CurrentLoopOperator == nil || operatorTampered.CurrentLoopOperator.SelectedDriverRequest == nil {
		t.Fatalf("fresh takeover artifact should carry a current-loop operator request: %+v", operatorTampered.CurrentLoopOperator)
	}
	tamperedOperatorCommand := "/rekit continue main -Apply -Format json"
	operatorTampered.CurrentLoopOperator.SelectedDriverRequest.Command = tamperedOperatorCommand
	if operatorTampered.CurrentLoopOperator.StartDriverRequest != nil {
		operatorTampered.CurrentLoopOperator.StartDriverRequest.Command = tamperedOperatorCommand
	}
	if operatorTampered.CurrentLoopOperator.ResumeDriverRequest != nil {
		operatorTampered.CurrentLoopOperator.ResumeDriverRequest.Command = tamperedOperatorCommand
	}
	operatorTamperedBytes, err := json.MarshalIndent(operatorTampered, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(laneArtifactPath, append(operatorTamperedBytes, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	var operatorTamperedStatus struct {
		MissionControlRunbook *statusMissionControlRunbookSnapshot `json:"missionControlRunbook"`
	}
	runDailyMissionControlRouteJSON(t, &out, takeoverArgs, &operatorTamperedStatus)
	operatorTamperedResult := operatorTamperedStatus.MissionControlRunbook.ReplacementExecutorTakeoverPackage
	if operatorTamperedResult == nil || operatorTamperedResult.DurableArtifactFresh || operatorTamperedResult.DurableArtifactState != "mixed-generation" || operatorTamperedResult.DurableArtifactRefreshDriverRequest == nil {
		t.Fatalf("tampered current-loop operator request should fail closed with a refresh route: %+v", operatorTamperedResult)
	}

	unknownFieldBytes := bytes.TrimSpace(artifactBytes)
	unknownFieldBytes = append(append([]byte{}, unknownFieldBytes[:len(unknownFieldBytes)-1]...), []byte(",\n  \"unexpected\": true\n}\n")...)
	if err := os.WriteFile(laneArtifactPath, unknownFieldBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	var unknownStatus struct {
		MissionControlRunbook *statusMissionControlRunbookSnapshot `json:"missionControlRunbook"`
	}
	runDailyMissionControlRouteJSON(t, &out, takeoverArgs, &unknownStatus)
	unknownPackage := unknownStatus.MissionControlRunbook.ReplacementExecutorTakeoverPackage
	if unknownPackage == nil || unknownPackage.DurableArtifactFresh || unknownPackage.DurableArtifactState != "mixed-generation" {
		t.Fatalf("unknown takeover artifact fields should fail strict decoding: %+v", unknownPackage)
	}

	assertFileNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "authority.jsonl"))
	assertFileNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "confirmed.jsonl"))
}

func assertDailyMissionControlTakeoverArtifact(t *testing.T, path, command, refreshCommand, relativePath string) replacementExecutorTakeoverPackageSnapshot {
	t.Helper()
	artifactBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var artifact replacementExecutorTakeoverPackageSnapshot
	if err := json.Unmarshal(artifactBytes, &artifact); err != nil {
		t.Fatalf("durable takeover artifact did not decode: %v\n%s", err, string(artifactBytes))
	}
	if !artifact.Ready || artifact.CurrentDriverRequest.Command != command || artifact.RefreshStatusCommand != refreshCommand || artifact.CurrentDriverRequestSHA256 == "" || !containsSubstring(artifact.RunbookSteps, "read "+relativePath+" before using any prior chat context") {
		t.Fatalf("durable takeover artifact should preserve the current request and refresh route: %+v", artifact)
	}
	return artifact
}

func runDailyMissionControlRouteJSON(t *testing.T, out *bytes.Buffer, args []string, target any) {
	t.Helper()
	out.Reset()
	if err := Run(args, out); err != nil {
		t.Fatalf("daily Mission Control route command failed: args=%+v err=%v\n%s", args, err, out.String())
	}
	if err := json.Unmarshal(out.Bytes(), target); err != nil {
		t.Fatalf("daily Mission Control route result did not decode: args=%+v err=%v\n%s", args, err, out.String())
	}
}
