package sessionhost

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/cli"
	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/subagents"
)

type currentStepPlan struct {
	Pack                          string                                `json:"pack"`
	ExpectedCurrentStepPlanSHA256 string                                `json:"expectedCurrentStepPlanSha256,omitempty"`
	CurrentDriverRequest          mission.MissionCommanderDriverRequest `json:"currentDriverRequest"`
	MemberExecution               *memberexecution.Plan                 `json:"memberExecution,omitempty"`
	ReviewerStep                  *reviewerStep                         `json:"reviewerStep,omitempty"`
	ExternalSessionStep           *externalSessionStep                  `json:"externalSessionStep,omitempty"`
}

func currentStepIsEvidenceReviewStop(plan currentStepPlan) bool {
	return plan.MemberExecution == nil &&
		plan.ReviewerStep == nil &&
		plan.ExternalSessionStep == nil &&
		strings.HasPrefix(strings.TrimSpace(plan.CurrentDriverRequest.Source), "executionEvidenceReview")
}

type currentLoopPlan struct {
	ExpectedCurrentLoopPlanSHA256 string                  `json:"expectedCurrentLoopPlanSha256,omitempty"`
	InitialCurrentStep            *currentStepPlan        `json:"initialCurrentStep,omitempty"`
	Applied                       bool                    `json:"applied,omitempty"`
	AppliedSteps                  int                     `json:"appliedSteps,omitempty"`
	StopReason                    currentLoopStopReason   `json:"stopReason"`
	SegmentCheckpoint             *currentLoopCheckpoint  `json:"segmentCheckpoint,omitempty"`
	FinalStatus                   *currentLoopFinalStatus `json:"finalStatus,omitempty"`
}

type currentLoopStopReason struct {
	Code    string `json:"code,omitempty"`
	Phase   string `json:"phase,omitempty"`
	Message string `json:"message,omitempty"`
}

type currentLoopCheckpoint struct {
	State    string `json:"state,omitempty"`
	StopCode string `json:"stopCode,omitempty"`
	Ready    bool   `json:"ready,omitempty"`
}

type currentLoopFinalStatus struct {
	CurrentMode string `json:"currentMode,omitempty"`
}

type reviewerStep struct {
	ExternalHandoff *reviewerExternalHandoff `json:"externalHandoff,omitempty"`
}

type boundReviewerStepPlan struct {
	PacketID                       string                          `json:"packetId"`
	PacketPath                     string                          `json:"packetPath"`
	TargetLane                     string                          `json:"targetLane"`
	ShardID                        string                          `json:"shardId"`
	ExpectedReviewerStepPlanSHA256 string                          `json:"expectedReviewerStepPlanSha256,omitempty"`
	ReviewerResultSnapshot         *reviewerResultSnapshotIdentity `json:"reviewerResultSnapshot,omitempty"`
	ExternalHandoff                *reviewerExternalHandoff        `json:"externalHandoff,omitempty"`
}

type reviewerResultSnapshotIdentity struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type reviewerExternalHandoff struct {
	State                         string `json:"state"`
	RunLoopStepID                 string `json:"runLoopStepId"`
	DispatchPromptPath            string `json:"dispatchPromptPath,omitempty"`
	DispatchPromptSHA256          string `json:"dispatchPromptSha256,omitempty"`
	ReviewerResultInputPath       string `json:"reviewerResultInputPath,omitempty"`
	ReviewerResultSourcePath      string `json:"reviewerResultSourcePath,omitempty"`
	ReviewerDispatchID            string `json:"reviewerDispatchId,omitempty"`
	ReviewerDispatchReceiptPath   string `json:"reviewerDispatchReceiptPath,omitempty"`
	ReviewerDispatchReceiptSHA256 string `json:"reviewerDispatchReceiptSha256,omitempty"`
	ReviewerHarness               string `json:"reviewerHarness,omitempty"`
	ReviewerSession               string `json:"reviewerSession,omitempty"`
}

type externalSessionStep struct {
	Mode           string                                            `json:"mode"`
	Attempt        *attemptPlan                                      `json:"attempt,omitempty"`
	Dispatch       *dispatchPlan                                     `json:"dispatch,omitempty"`
	HarnessPackage *mission.CurrentLoopExternalSessionHarnessPackage `json:"harnessPackage,omitempty"`
}

type attemptPlan struct {
	AttemptSHA256 string `json:"attemptSha256"`
	Attempt       struct {
		Generation int `json:"generation"`
	} `json:"attempt"`
}

type dispatchPlan struct {
	AttemptSHA256 string `json:"attemptSha256"`
}

func requireRunningHandoffForPackage(pkg mission.CurrentLoopExternalSessionHarnessPackage, fresh currentStepPlan) error {
	before := currentStepPlan{ExternalSessionStep: &externalSessionStep{Mode: "running-handoff", HarnessPackage: &pkg}}
	return requireSameRunningHandoff(before, fresh)
}

func requireSameRunningHandoff(before, fresh currentStepPlan) error {
	if before.ExternalSessionStep == nil || fresh.ExternalSessionStep == nil ||
		before.ExternalSessionStep.Mode != "running-handoff" || fresh.ExternalSessionStep.Mode != "running-handoff" ||
		before.ExternalSessionStep.HarnessPackage == nil || fresh.ExternalSessionStep.HarnessPackage == nil ||
		before.ExternalSessionStep.HarnessPackage.Launch == nil || fresh.ExternalSessionStep.HarnessPackage.Launch == nil {
		return fmt.Errorf("external session changed before exact supervised result publication")
	}
	left := before.ExternalSessionStep.HarnessPackage.Launch.Attempt
	right := fresh.ExternalSessionStep.HarnessPackage.Launch.Attempt
	if left.AttemptID != right.AttemptID || left.AttemptSHA256 != right.AttemptSHA256 || left.Generation != right.Generation || left.Session != right.Session ||
		before.ExternalSessionStep.HarnessPackage.JobSHA256 != fresh.ExternalSessionStep.HarnessPackage.JobSHA256 ||
		before.ExternalSessionStep.HarnessPackage.CheckpointSHA256 != fresh.ExternalSessionStep.HarnessPackage.CheckpointSHA256 {
		return fmt.Errorf("external session attempt, session, job, or checkpoint changed before exact supervised result publication")
	}
	return nil
}

func memberIntakeComplete(caseRoot, selected string, plan currentStepPlan) bool {
	if plan.ExternalSessionStep != nil || plan.ReviewerStep != nil {
		return false
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		return false
	}
	lanes := mission.OpenBoardLanes(board.Lanes)
	if selected = strings.TrimSpace(selected); selected != "" {
		lane, ok := mission.LookupBoardLane(lanes, selected, false)
		if !ok {
			return false
		}
		lanes = []mission.BoardLane{lane}
	}
	for _, lane := range lanes {
		if strings.TrimSpace(lane.CurrentExecutor) == "" || lane.ExecutorGeneration < 1 {
			continue
		}
		latest, ok, err := memberexecution.Latest(caseRoot, lane.ID)
		if err == nil && ok && latest.State == "intake-ready" && latest.Owner.Executor == lane.CurrentExecutor && latest.Owner.ExecutorGeneration == lane.ExecutorGeneration {
			return true
		}
	}
	return false
}

func reviewerDispatchReady(plan currentStepPlan) bool {
	return plan.ReviewerStep != nil && plan.ReviewerStep.ExternalHandoff != nil &&
		plan.ReviewerStep.ExternalHandoff.State == "ready-for-reviewer-dispatch" &&
		plan.ReviewerStep.ExternalHandoff.RunLoopStepID == "spawn-reviewer"
}

func reviewerSessionPending(plan currentStepPlan) bool {
	return plan.ReviewerStep != nil && plan.ReviewerStep.ExternalHandoff != nil &&
		plan.ReviewerStep.ExternalHandoff.State == "reviewer-session-running-unknown" &&
		plan.ReviewerStep.ExternalHandoff.RunLoopStepID == "save-result-input" &&
		strings.TrimSpace(plan.ReviewerStep.ExternalHandoff.ReviewerSession) != ""
}

func reviewerActorStep(plan currentStepPlan) bool {
	if plan.ReviewerStep == nil || plan.ReviewerStep.ExternalHandoff == nil {
		return false
	}
	switch plan.ReviewerStep.ExternalHandoff.RunLoopStepID {
	case "verify-prompt", "record-completion", "source-capture", "stage-candidate", "collect-result", "intake-results":
		return true
	default:
		return false
	}
}

func applyReviewerFailure(opt Options, reason string) error {
	reason = truncate(oneLine(strings.TrimSpace(reason)), 1024)
	if reason == "" {
		reason = "Claude reviewer session failed"
	}
	args := []string{"-ReviewerOutcome", "failed", "-ReviewerExitStatus", reason, "-Actor", opt.Actor}
	plan, err := runCurrentStep(opt, args, false)
	if err != nil {
		return err
	}
	if strings.TrimSpace(plan.ExpectedCurrentStepPlanSHA256) == "" {
		return fmt.Errorf("reviewer failure preview omitted the hash-bound plan")
	}
	return applyCurrentStep(opt, plan, args)
}

func runCurrentStep(opt Options, extra []string, apply bool) (currentStepPlan, error) {
	return runCurrentStepWithReviewerSnapshot(opt, extra, apply, nil)
}

func runCurrentStepWithReviewerSnapshot(opt Options, extra []string, apply bool, snapshot *subagents.ReviewerResultInputSnapshot) (currentStepPlan, error) {
	return runCurrentStepWithReviewerPublication(opt, extra, apply, snapshot, nil)
}

func runCurrentStepWithReviewerPublication(
	opt Options,
	extra []string,
	apply bool,
	snapshot *subagents.ReviewerResultInputSnapshot,
	publication *executioncontrol.ResultPublicationOptions,
) (currentStepPlan, error) {
	if opt.reviewerBinding != nil {
		return runBoundReviewerStepWithPublication(opt, extra, apply, snapshot, publication)
	}
	return runCurrentStepWithPrivatePublication(opt, extra, apply, snapshot, publication, nil)
}

func runCurrentStepWithReplacementPublication(
	opt Options,
	extra []string,
	apply bool,
	publication *executioncontrol.ResultPublicationOptions,
) (currentStepPlan, error) {
	if opt.reviewerBinding != nil {
		return currentStepPlan{}, fmt.Errorf("bound Reviewer route does not support external attempt replacement")
	}
	return runCurrentStepWithPrivatePublication(opt, extra, apply, nil, nil, publication)
}

func runCurrentStepWithPrivatePublication(
	opt Options,
	extra []string,
	apply bool,
	snapshot *subagents.ReviewerResultInputSnapshot,
	reviewerPublication,
	replacementPublication *executioncontrol.ResultPublicationOptions,
) (currentStepPlan, error) {
	args := []string{"-Command", "run-current-step", "-Target", opt.Target}
	if strings.TrimSpace(opt.Pack) != "" {
		args = append(args, "-Pack", opt.Pack)
	}
	args = appendSelectedLaneArg(args, opt.SelectedLane)
	args = append(args, extra...)
	if apply {
		args = append(args, "-Apply")
	} else {
		args = append(args, "-WhatIf")
	}
	args = append(args, "-Format", "json")
	var out bytes.Buffer
	var err error
	if replacementPublication != nil {
		if snapshot != nil || reviewerPublication != nil {
			return currentStepPlan{}, fmt.Errorf("replacement and Reviewer result publication provenance cannot be combined")
		}
		err = cli.RunWithReplacementResultPublication(args, &out, replacementPublication)
	} else {
		err = cli.RunWithReviewerResultPublication(args, &out, snapshot, reviewerPublication)
	}
	if err != nil {
		return currentStepPlan{}, err
	}
	var plan currentStepPlan
	dec := json.NewDecoder(bytes.NewReader(out.Bytes()))
	if err := dec.Decode(&plan); err != nil {
		return currentStepPlan{}, fmt.Errorf("decode run-current-step result: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return currentStepPlan{}, fmt.Errorf("run-current-step returned trailing JSON")
	}
	return plan, nil
}

func runBoundReviewerStep(opt Options, extra []string, apply bool, snapshot *subagents.ReviewerResultInputSnapshot) (currentStepPlan, error) {
	return runBoundReviewerStepWithPublication(opt, extra, apply, snapshot, nil)
}

func runBoundReviewerStepWithPublication(
	opt Options,
	extra []string,
	apply bool,
	snapshot *subagents.ReviewerResultInputSnapshot,
	publication *executioncontrol.ResultPublicationOptions,
) (currentStepPlan, error) {
	if err := validateReviewerBinding(opt); err != nil {
		return currentStepPlan{}, err
	}
	args := []string{"-Command", "run-reviewer-step", "-Target", opt.Target}
	if strings.TrimSpace(opt.Pack) != "" {
		args = append(args, "-Pack", opt.Pack)
	}
	args = appendSelectedLaneArg(args, opt.SelectedLane)
	args = append(args, extra...)
	if apply {
		args = append(args, "-Apply")
	} else {
		args = append(args, "-WhatIf")
	}
	args = append(args, "-Format", "json")
	var out bytes.Buffer
	if err := cli.RunWithReviewerResultPublication(args, &out, snapshot, publication); err != nil {
		return currentStepPlan{}, err
	}
	var plan boundReviewerStepPlan
	dec := json.NewDecoder(bytes.NewReader(out.Bytes()))
	if err := dec.Decode(&plan); err != nil {
		return currentStepPlan{}, fmt.Errorf("decode run-reviewer-step result: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return currentStepPlan{}, fmt.Errorf("run-reviewer-step returned trailing JSON")
	}
	if err := requireReviewerBinding(opt, plan); err != nil {
		return currentStepPlan{}, err
	}
	if err := requireReviewerResultSnapshot(snapshot, plan.ReviewerResultSnapshot); err != nil {
		return currentStepPlan{}, err
	}
	return currentStepPlan{
		Pack:                          opt.Pack,
		ExpectedCurrentStepPlanSHA256: plan.ExpectedReviewerStepPlanSHA256,
		ReviewerStep:                  &reviewerStep{ExternalHandoff: plan.ExternalHandoff},
	}, nil
}

func requireReviewerResultSnapshot(snapshot *subagents.ReviewerResultInputSnapshot, identity *reviewerResultSnapshotIdentity) error {
	if snapshot == nil {
		if identity != nil {
			return fmt.Errorf("reviewer step returned an unexpected result snapshot binding")
		}
		return nil
	}
	if identity == nil || !rekitfs.SamePath(identity.Path, snapshot.Path) ||
		!strings.EqualFold(identity.SHA256, snapshot.SHA256) || identity.Bytes != snapshot.Bytes ||
		snapshot.Bytes != int64(len(snapshot.Data)) || !strings.EqualFold(snapshot.SHA256, bytesSHA256(snapshot.Data)) {
		return fmt.Errorf("reviewer step changed the exact result snapshot binding")
	}
	return nil
}

func validateReviewerBinding(opt Options) error {
	binding := opt.reviewerBinding
	if binding == nil {
		return nil
	}
	for label, value := range map[string]string{
		"packet id":              binding.PacketID,
		"packet path":            binding.PacketPath,
		"packet sha256":          binding.PacketSHA256,
		"lane":                   binding.Lane,
		"shard id":               binding.ShardID,
		"dispatch prompt path":   binding.DispatchPromptPath,
		"dispatch prompt sha256": binding.DispatchPromptSHA256,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("reviewer binding requires %s", label)
		}
	}
	packet, err := rekitfs.ReadStableRegularFileAnchored(opt.Target, binding.PacketPath, "bound reviewer packet", 1<<20)
	if err != nil {
		return err
	}
	if !strings.EqualFold(bytesSHA256(packet), strings.TrimSpace(binding.PacketSHA256)) {
		return fmt.Errorf("bound reviewer packet sha256 changed")
	}
	prompt, err := rekitfs.ReadStableRegularFileAnchored(opt.Target, binding.DispatchPromptPath, "bound reviewer dispatch prompt", 1<<20)
	if err != nil {
		return err
	}
	if !strings.EqualFold(bytesSHA256(prompt), strings.TrimSpace(binding.DispatchPromptSHA256)) {
		return fmt.Errorf("bound reviewer dispatch prompt sha256 changed")
	}
	return nil
}

func requireReviewerBinding(opt Options, plan boundReviewerStepPlan) error {
	binding := opt.reviewerBinding
	if binding == nil {
		return fmt.Errorf("bound reviewer step requires reviewer binding")
	}
	if plan.PacketID != strings.TrimSpace(binding.PacketID) ||
		!rekitfs.SamePath(plan.PacketPath, binding.PacketPath) ||
		plan.TargetLane != strings.TrimSpace(binding.Lane) ||
		plan.ShardID != strings.TrimSpace(binding.ShardID) {
		return fmt.Errorf("reviewer operator package changed from the exact packet, lane, or shard binding")
	}
	if plan.ExternalHandoff != nil && (!rekitfs.SamePath(plan.ExternalHandoff.DispatchPromptPath, binding.DispatchPromptPath) ||
		!strings.EqualFold(plan.ExternalHandoff.DispatchPromptSHA256, strings.TrimSpace(binding.DispatchPromptSHA256))) {
		return fmt.Errorf("reviewer operator package changed from the exact dispatch prompt binding")
	}
	return validateReviewerBinding(opt)
}

func applyMemberDispatchLoop(opt Options) error {
	preview, err := runCurrentLoop(opt, false, "", "")
	if err != nil {
		return err
	}
	if strings.TrimSpace(preview.ExpectedCurrentLoopPlanSHA256) == "" {
		return fmt.Errorf("member dispatch current-loop preview omitted the hash-bound plan")
	}
	memberPlanSHA256 := ""
	if preview.InitialCurrentStep != nil && preview.InitialCurrentStep.MemberExecution != nil {
		memberPlanSHA256 = preview.InitialCurrentStep.MemberExecution.ExpectedPlanSHA256
	}
	if strings.TrimSpace(memberPlanSHA256) == "" {
		return fmt.Errorf("member dispatch current-loop preview omitted the nested member execution plan")
	}
	applied, err := runCurrentLoop(opt, true, preview.ExpectedCurrentLoopPlanSHA256, memberPlanSHA256)
	if err != nil {
		return err
	}
	if !applied.Applied {
		return fmt.Errorf(
			"member dispatch current-loop Apply did not record a durable step: stop=%s phase=%s message=%s appliedSteps=%d checkpoint=%s finalMode=%s",
			strings.TrimSpace(applied.StopReason.Code),
			strings.TrimSpace(applied.StopReason.Phase),
			strings.TrimSpace(applied.StopReason.Message),
			applied.AppliedSteps,
			currentLoopCheckpointSummary(applied.SegmentCheckpoint),
			currentLoopFinalMode(applied.FinalStatus),
		)
	}
	return nil
}

func currentLoopCheckpointSummary(checkpoint *currentLoopCheckpoint) string {
	if checkpoint == nil {
		return "<none>"
	}
	return fmt.Sprintf("%s/%s/ready=%t", strings.TrimSpace(checkpoint.State), strings.TrimSpace(checkpoint.StopCode), checkpoint.Ready)
}

func currentLoopFinalMode(status *currentLoopFinalStatus) string {
	if status == nil || strings.TrimSpace(status.CurrentMode) == "" {
		return "<none>"
	}
	return strings.TrimSpace(status.CurrentMode)
}

func runCurrentLoop(opt Options, apply bool, expected, memberPlanSHA256 string) (currentLoopPlan, error) {
	args := []string{"-Command", "run-current-loop", "-Target", opt.Target}
	if strings.TrimSpace(opt.Pack) != "" {
		args = append(args, "-Pack", opt.Pack)
	}
	args = appendSelectedLaneArg(args, opt.SelectedLane)
	args = append(args, "-MaxSteps", "2")
	if apply {
		args = append(args, "-ExpectedMemberExecutionPlanSha256", memberPlanSHA256, "-ExpectedCurrentLoopPlanSha256", expected, "-Apply")
	} else {
		args = append(args, "-WhatIf")
	}
	args = append(args, "-Format", "json")
	var out bytes.Buffer
	if err := cli.Run(args, &out); err != nil {
		return currentLoopPlan{}, err
	}
	var plan currentLoopPlan
	dec := json.NewDecoder(bytes.NewReader(out.Bytes()))
	if err := dec.Decode(&plan); err != nil {
		return currentLoopPlan{}, fmt.Errorf("decode run-current-loop result: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return currentLoopPlan{}, fmt.Errorf("run-current-loop returned trailing JSON")
	}
	return plan, nil
}

func applyCurrentStep(opt Options, plan currentStepPlan, transitionArgs []string) error {
	return applyCurrentStepWithReviewerSnapshot(opt, plan, transitionArgs, nil)
}

func applyCurrentStepWithReviewerSnapshot(opt Options, plan currentStepPlan, transitionArgs []string, snapshot *subagents.ReviewerResultInputSnapshot) error {
	return applyCurrentStepWithReviewerPublication(opt, plan, transitionArgs, snapshot, nil)
}

func applyCurrentStepWithReviewerPublication(
	opt Options,
	plan currentStepPlan,
	transitionArgs []string,
	snapshot *subagents.ReviewerResultInputSnapshot,
	publication *executioncontrol.ResultPublicationOptions,
) error {
	if strings.TrimSpace(plan.ExpectedCurrentStepPlanSHA256) == "" {
		return fmt.Errorf("current external session step has no deterministic apply hash")
	}
	args := append([]string{}, transitionArgs...)
	expectedFlag := "-ExpectedCurrentStepPlanSha256"
	if opt.reviewerBinding != nil {
		expectedFlag = "-ExpectedReviewerStepPlanSha256"
	}
	args = append(args, expectedFlag, plan.ExpectedCurrentStepPlanSHA256)
	_, err := runCurrentStepWithReviewerPublication(opt, args, true, snapshot, publication)
	return err
}

func applyReplacementAttempt(
	opt Options,
	running currentStepPlan,
	actor string,
	publication executioncontrol.ResultPublicationOptions,
) (string, error) {
	step := running.ExternalSessionStep
	if step == nil || step.Mode != "running-handoff" || step.HarnessPackage == nil || step.HarnessPackage.Launch == nil {
		return "", fmt.Errorf("replacement requires the current accepted running handoff")
	}
	currentAttemptSHA := step.HarnessPackage.Launch.Attempt.AttemptSHA256
	if currentAttemptSHA == "" {
		return "", fmt.Errorf("replacement handoff omitted current attempt sha256")
	}
	session, err := newUUID()
	if err != nil {
		return "", err
	}
	args := attemptArgs(actor, session, currentAttemptSHA)
	plan, err := runCurrentStepWithReplacementPublication(opt, args, false, &publication)
	if err != nil {
		return "", err
	}
	if plan.ExternalSessionStep == nil || plan.ExternalSessionStep.Mode != "replacement-attempt" || plan.ExternalSessionStep.Attempt == nil {
		return "", fmt.Errorf("replacement preview omitted the exact next attempt")
	}
	args = append(args, "-ExpectedCurrentStepPlanSha256", plan.ExpectedCurrentStepPlanSHA256)
	if _, err := runCurrentStepWithReplacementPublication(opt, args, true, &publication); err != nil {
		return "", err
	}
	return session, nil
}

func claudeRunReplacementOutcome(run claudeRun, attemptGeneration, launchOrdinal, attemptsLimit int) (string, bool) {
	if run.success() || attemptsLimit <= 0 || claudeRunAttemptsUsed(attemptGeneration, launchOrdinal) >= attemptsLimit {
		return "", false
	}
	if run.failureDetail != "" {
		return "invalid-result-replacement", true
	}
	return "replacement-requested", true
}

func currentStepAttemptGeneration(plan currentStepPlan) int {
	step := plan.ExternalSessionStep
	if step == nil {
		return 0
	}
	generation := 0
	if step.HarnessPackage != nil && step.HarnessPackage.Launch != nil {
		generation = step.HarnessPackage.Launch.Attempt.Generation
	}
	if step.Attempt != nil && step.Attempt.Attempt.Generation > generation {
		generation = step.Attempt.Attempt.Generation
	}
	return generation
}

func claudeRunAttemptsUsed(attemptGeneration, launchOrdinal int) int {
	if attemptGeneration > launchOrdinal {
		return attemptGeneration
	}
	return launchOrdinal
}

func claudeAttemptLimitReached(attemptGeneration, launchOrdinal, attemptsLimit int) bool {
	return attemptsLimit > 0 && claudeRunAttemptsUsed(attemptGeneration, launchOrdinal) >= attemptsLimit
}

func claudeLaunchLimitReached(attemptGeneration, launchOrdinal, attemptsLimit int) bool {
	return attemptsLimit > 0 && (attemptGeneration > attemptsLimit || launchOrdinal >= attemptsLimit)
}

func claudeRunForSupervisionFence(err *supervisionFencedError, binding *claudeLaunchControlBinding, recovered bool) claudeRun {
	return claudeRun{
		launchControlBinding: cloneClaudeLaunchControlBinding(binding),
		failureCode:          "claude-supervision-fenced",
		waitErr:              err,
		recovered:            recovered,
	}
}

func attemptArgs(actor, session, supersedes string) []string {
	args := []string{
		"-ExternalSessionHarness", defaultHarness,
		"-ExternalSessionId", session,
		"-ExternalSessionActor", actor,
		"-ExternalSessionStartedAt", nowRFC3339Nano(),
	}
	if supersedes != "" {
		args = append(args, "-ExpectedExternalSessionAttemptSha256", supersedes)
	}
	return args
}
