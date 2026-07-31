package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
	"github.com/shuiyu486/re-context-kits/internal/rekit/subagents"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

type reviewerStepPlan struct {
	SchemaVersion                  int                                    `json:"schemaVersion"`
	Command                        string                                 `json:"command"`
	CaseRoot                       string                                 `json:"caseRoot"`
	Pack                           string                                 `json:"pack"`
	IsMutation                     bool                                   `json:"isMutation"`
	Applied                        bool                                   `json:"applied"`
	ReviewRequired                 bool                                   `json:"reviewRequired"`
	RequiresConfirmation           bool                                   `json:"requiresConfirmation"`
	CurrentDriverRequest           mission.MissionCommanderDriverRequest  `json:"currentDriverRequest"`
	PreviewDriverRequest           *mission.MissionCommanderDriverRequest `json:"previewDriverRequest,omitempty"`
	PreviewResult                  any                                    `json:"previewResult,omitempty"`
	ApplyDriverRequest             *mission.MissionCommanderDriverRequest `json:"applyDriverRequest,omitempty"`
	ExpectedReviewerStepPlanSHA256 string                                 `json:"expectedReviewerStepPlanSha256,omitempty"`
	MissionCommanderActionQueue    mission.MissionCommanderActionQueue    `json:"missionCommanderActionQueue"`
	ExternalHandoff                *reviewerStepExternalHandoff           `json:"externalHandoff,omitempty"`
	Receipt                        *reviewerStepReceipt                   `json:"receipt,omitempty"`
	RefreshedStatus                *statusInventory                       `json:"refreshedStatus,omitempty"`
	Boundary                       []string                               `json:"boundary"`
}

type reviewerStepExternalHandoff struct {
	State                         string                               `json:"state"`
	RunLoopStepID                 string                               `json:"runLoopStepId"`
	RequiredInputs                []string                             `json:"requiredInputs"`
	AgentToolRequest              *workstream.ReviewerAgentToolRequest `json:"agentToolRequest,omitempty"`
	DispatchPromptPath            string                               `json:"dispatchPromptPath,omitempty"`
	DispatchPromptSHA256          string                               `json:"dispatchPromptSha256,omitempty"`
	ReviewerResultDropPath        string                               `json:"reviewerResultDropPath,omitempty"`
	RecordDispatchPreviewTemplate string                               `json:"recordDispatchPreviewTemplate,omitempty"`
	Boundary                      []string                             `json:"boundary"`
}

type reviewerStepReceipt struct {
	State                         string                                 `json:"state"`
	Outcome                       string                                 `json:"outcome"`
	RequestedRunLoopStepID        string                                 `json:"requestedRunLoopStepId"`
	RequestedState                string                                 `json:"requestedState"`
	ExecutedCommand               string                                 `json:"executedCommand"`
	CommandResultCommand          string                                 `json:"commandResultCommand"`
	RefreshStatusCommand          string                                 `json:"refreshStatusCommand"`
	RefreshStatusCommandMatched   bool                                   `json:"refreshStatusCommandMatched"`
	RefreshedCurrentDriverRequest *mission.MissionCommanderDriverRequest `json:"refreshedCurrentDriverRequest,omitempty"`
	Boundary                      []string                               `json:"boundary"`
}

type reviewerStepPlanIdentity struct {
	CurrentDriverRequest mission.MissionCommanderDriverRequest `json:"currentDriverRequest"`
	PreviewDriverRequest mission.MissionCommanderDriverRequest `json:"previewDriverRequest"`
	ApplyDriverRequest   mission.MissionCommanderDriverRequest `json:"applyDriverRequest"`
}

func runReviewerStep(ctx runtime.Context, opt Options, out io.Writer) error {
	if !ctx.TargetProvided {
		return fmt.Errorf("run-reviewer-step requires -Target for an attached case")
	}
	if opt.WhatIf && opt.Apply {
		return fmt.Errorf("run-reviewer-step cannot combine -WhatIf and -Apply")
	}
	if !opt.WhatIf && !opt.Apply {
		return fmt.Errorf("run-reviewer-step requires -WhatIf or -Apply")
	}
	if strings.ToLower(strings.TrimSpace(opt.Format)) != "json" {
		return fmt.Errorf("run-reviewer-step supports only -Format json")
	}
	if err := validateReviewerStepOuterArgs(opt); err != nil {
		return err
	}
	plan, err := buildReviewerStepPlan(ctx, opt)
	if err != nil {
		return err
	}
	if opt.WhatIf {
		if strings.TrimSpace(opt.ExpectedReviewerStepPlanSHA256) != "" {
			return fmt.Errorf("run-reviewer-step -WhatIf does not accept -ExpectedReviewerStepPlanSha256")
		}
		return writeJSON(out, plan)
	}
	if plan.ExternalHandoff != nil {
		return fmt.Errorf("run-reviewer-step current step requires an external harness action before Apply")
	}
	expected := strings.TrimSpace(opt.ExpectedReviewerStepPlanSHA256)
	if expected == "" {
		return fmt.Errorf("run-reviewer-step -Apply requires -ExpectedReviewerStepPlanSha256 from -WhatIf")
	}
	if !strings.EqualFold(expected, plan.ExpectedReviewerStepPlanSHA256) {
		return fmt.Errorf("run-reviewer-step expected plan sha256 mismatch: got %s want %s", expected, plan.ExpectedReviewerStepPlanSHA256)
	}
	if plan.ApplyDriverRequest == nil {
		return fmt.Errorf("run-reviewer-step preview omitted a typed Apply driver request")
	}
	result, err := applyReviewerStep(ctx, *plan.ApplyDriverRequest)
	if err != nil {
		return err
	}
	refreshed, err := buildStatusInventory(ctx, statusPackSource(ctx, opt))
	if err != nil {
		return fmt.Errorf("refresh status after reviewer step: %w", err)
	}
	receipt, err := reviewerStepReceiptFor(ctx, plan.CurrentDriverRequest, *plan.ApplyDriverRequest, result, refreshed)
	if err != nil {
		return err
	}
	plan.IsMutation = true
	plan.Applied = reviewerStepResultApplied(result)
	plan.ReviewRequired = false
	plan.RequiresConfirmation = false
	plan.PreviewResult = result
	plan.Receipt = &receipt
	plan.RefreshedStatus = &refreshed
	plan.MissionCommanderActionQueue = reviewerStepResultQueue(result)
	return writeJSON(out, plan)
}

func buildReviewerStepPlan(ctx runtime.Context, opt Options) (reviewerStepPlan, error) {
	status, err := buildStatusInventory(ctx, statusPackSource(ctx, opt))
	if err != nil {
		return reviewerStepPlan{}, err
	}
	pkg, err := reviewerStepOperatorPackage(status)
	if err != nil {
		return reviewerStepPlan{}, err
	}
	request := *pkg.CurrentDriverRequest
	previewRequest, external, err := reviewerStepPreparedRequest(ctx, opt, pkg)
	if err != nil {
		return reviewerStepPlan{}, err
	}
	plan := reviewerStepPlan{
		SchemaVersion:        1,
		Command:              commands.RunReviewerStep,
		CaseRoot:             ctx.Target,
		Pack:                 ctx.Pack,
		CurrentDriverRequest: request,
		ExternalHandoff:      external,
		ReviewRequired:       true,
		RequiresConfirmation: external == nil,
		Boundary: []string{
			"runner consumes only reviewerDispatchIntakeSummary.operatorPackage.currentDriverRequest and its current run-loop step",
			"spawn-reviewer and reviewer JSON production remain external harness actions; the Go runtime never invokes Agent tool or fabricates reviewer output",
			"outer plan hash binds the durable current request, resolved preview request, and typed Apply request; each reviewer handler retains its own artifact hashes, packet binding, lock, and currentness checks",
			"deterministic reviewer artifact and intake mutations remain preview-first and use only the typed Apply request returned by the matching handler",
			"the Go runtime does not execute heavy tools or write authority/confirmed state",
			"status is rebuilt after Apply before follow-up reviewer work is selected",
		},
	}
	if external != nil {
		return plan, nil
	}
	plan.PreviewDriverRequest = &previewRequest
	previewOpt, err := parseBoundedReviewerRequest(ctx, previewRequest, false)
	if err != nil {
		return reviewerStepPlan{}, err
	}
	preview, err := previewReviewerStep(ctx, previewOpt)
	if err != nil {
		return reviewerStepPlan{}, err
	}
	applyRequest, err := reviewerStepApplyRequest(preview)
	if err != nil {
		return reviewerStepPlan{}, err
	}
	applyRequest, err = qualifyReviewerStepRequest(ctx, applyRequest)
	if err != nil {
		return reviewerStepPlan{}, err
	}
	applyOpt, err := parseBoundedReviewerRequest(ctx, applyRequest, true)
	if err != nil {
		return reviewerStepPlan{}, fmt.Errorf("returned reviewer Apply driver request: %w", err)
	}
	if reviewerStepMode(previewOpt) != reviewerStepMode(applyOpt) {
		return reviewerStepPlan{}, fmt.Errorf("returned reviewer Apply driver request mode differs from current preview request")
	}
	identity := reviewerStepPlanIdentity{
		CurrentDriverRequest: request,
		PreviewDriverRequest: previewRequest,
		ApplyDriverRequest:   applyRequest,
	}
	encoded, err := json.Marshal(identity)
	if err != nil {
		return reviewerStepPlan{}, err
	}
	sum := sha256.Sum256(encoded)
	plan.PreviewResult = preview
	plan.ApplyDriverRequest = &applyRequest
	plan.ExpectedReviewerStepPlanSHA256 = hex.EncodeToString(sum[:])
	plan.MissionCommanderActionQueue = reviewerStepResultQueue(preview)
	return plan, nil
}

func reviewerStepOperatorPackage(status statusInventory) (*workstream.ReviewerDispatchOperatorPackage, error) {
	if status.CaseMission == nil || status.CaseMission.ReviewerDispatchIntakeSummary.OperatorPackage == nil {
		return nil, fmt.Errorf("run-reviewer-step requires caseMission.reviewerDispatchIntakeSummary.operatorPackage")
	}
	pkg := status.CaseMission.ReviewerDispatchIntakeSummary.OperatorPackage
	if !pkg.Ready || pkg.Current == nil || pkg.CurrentDriverRequest == nil {
		return nil, fmt.Errorf("run-reviewer-step requires a ready reviewer operator package with currentDriverRequest")
	}
	if pkg.CurrentDriverRequest.Source != "reviewerDispatchOperatorPackage" {
		return nil, fmt.Errorf("run-reviewer-step rejects reviewer driver request source %q", pkg.CurrentDriverRequest.Source)
	}
	if strings.TrimSpace(pkg.CurrentDriverRequest.RunLoopStepID) == "" || pkg.CurrentDriverRequest.RunLoopStepID != pkg.CurrentRunLoopStepID {
		return nil, fmt.Errorf("run-reviewer-step reviewer run-loop step identity is inconsistent")
	}
	return pkg, nil
}

func reviewerStepPreparedRequest(ctx runtime.Context, opt Options, pkg *workstream.ReviewerDispatchOperatorPackage) (mission.MissionCommanderDriverRequest, *reviewerStepExternalHandoff, error) {
	current := pkg.Current
	stepID := strings.TrimSpace(pkg.CurrentRunLoopStepID)
	actor := strings.TrimSpace(opt.Note.Actor)
	command := ""
	required := []string{}
	switch stepID {
	case "spawn-reviewer":
		harness := strings.TrimSpace(opt.ReviewerHarness)
		session := strings.TrimSpace(opt.ReviewerSession)
		if harness == "" || session == "" || actor == "" {
			required = []string{"invoke agentToolRequest in the external main-agent harness", "rerun with -ReviewerHarness, -ReviewerSession, and -Actor after the harness accepts the session"}
			return mission.MissionCommanderDriverRequest{}, reviewerStepExternal(pkg, required), nil
		}
		command = current.ReviewerDispatchRecordCommand
		command = replaceReviewerStepToken(command, "<harness>", harness)
		command = replaceReviewerStepToken(command, "<session-id>", session)
	case "save-result-input":
		if strings.TrimSpace(opt.ReviewerOutcome) != "" {
			if !strings.EqualFold(strings.TrimSpace(opt.ReviewerOutcome), "failed") || strings.TrimSpace(opt.ReviewerExitStatus) == "" || actor == "" {
				return mission.MissionCommanderDriverRequest{}, nil, fmt.Errorf("run-reviewer-step running reviewer failure receipt requires -ReviewerOutcome failed, -ReviewerExitStatus, and -Actor")
			}
			if strings.TrimSpace(current.ReviewerDispatchID) == "" {
				return mission.MissionCommanderDriverRequest{}, nil, fmt.Errorf("run-reviewer-step cannot record completion without a current reviewer dispatch id")
			}
			command = joinDriverCommand([]string{
				"/rekit", commands.PlanSubagents,
				"-PacketPath", pkg.PacketPath,
				"-RecordReviewerCompletion",
				"-ReviewerDispatchId", current.ReviewerDispatchID,
				"-ReviewerOutcome", "failed",
				"-ReviewerExitStatus", strings.TrimSpace(opt.ReviewerExitStatus),
				"-Lane", pkg.TargetLane,
				"-Actor", actor,
				"-WhatIf", "-Format", "json",
			})
		} else {
			sourcePath := strings.TrimSpace(opt.ReviewerResultInputSourcePath)
			if sourcePath == "" || actor == "" {
				required = []string{"wait for the external reviewer session to return exactly one ReviewerResult JSON object", "save the JSON to a case-local source path", "rerun with -ReviewerResultInputSourcePath and -Actor; or use -ReviewerOutcome failed with -ReviewerExitStatus and -Actor"}
				return mission.MissionCommanderDriverRequest{}, reviewerStepExternal(pkg, required), nil
			}
			command = current.ReviewerResultInputSavePreviewCommand
			command = replaceReviewerStepToken(command, "<reviewer-result-json-path>", sourcePath)
		}
	case "verify-prompt", "record-completion", "source-capture", "stage-candidate", "collect-result", "intake-results":
		if actor == "" {
			return mission.MissionCommanderDriverRequest{}, reviewerStepExternal(pkg, []string{"rerun with the explicit -Actor that owns this reviewer operation"}), nil
		}
		command = pkg.CurrentDriverRequest.Command
	default:
		return mission.MissionCommanderDriverRequest{}, nil, fmt.Errorf("reviewer run-loop step %q is outside the run-reviewer-step allowlist", stepID)
	}
	if strings.TrimSpace(command) == "" {
		return mission.MissionCommanderDriverRequest{}, nil, fmt.Errorf("reviewer run-loop step %q omitted its bounded preview command", stepID)
	}
	command = replaceReviewerStepToken(command, "<main-agent>", actor)
	request := *pkg.CurrentDriverRequest
	request.Kind = "preview-command"
	request.Command = command
	request.Guidance = ""
	request.CommandExecutable = true
	request.Blocked = false
	request.RequiresReview = true
	qualified, err := qualifyReviewerStepRequest(ctx, request)
	if err != nil {
		return mission.MissionCommanderDriverRequest{}, nil, err
	}
	return qualified, nil, nil
}

func reviewerStepExternal(pkg *workstream.ReviewerDispatchOperatorPackage, required []string) *reviewerStepExternalHandoff {
	current := pkg.Current
	return &reviewerStepExternalHandoff{
		State:                         current.State,
		RunLoopStepID:                 pkg.CurrentRunLoopStepID,
		RequiredInputs:                required,
		AgentToolRequest:              current.AgentToolRequest,
		DispatchPromptPath:            current.DispatchPromptPath,
		DispatchPromptSHA256:          current.DispatchPromptSHA256,
		ReviewerResultDropPath:        current.ReviewerResultDropPath,
		RecordDispatchPreviewTemplate: current.ReviewerDispatchRecordCommand,
		Boundary: []string{
			"external handoff is read-only and cannot be passed to run-reviewer-step -Apply",
			"only the external main-agent harness invokes Agent tool and supplies real harness/session observations or reviewer JSON",
			"reviewer work remains read-only and must not write case files, facts, authority, confirmed state, or execute heavy tools",
		},
	}
}

func replaceReviewerStepToken(command, placeholder, value string) string {
	fields, err := splitDriverCommand(command)
	if err != nil {
		return command
	}
	for idx := range fields {
		if fields[idx] == placeholder {
			fields[idx] = value
		}
	}
	return joinDriverCommand(fields)
}

func qualifyReviewerStepRequest(ctx runtime.Context, request mission.MissionCommanderDriverRequest) (mission.MissionCommanderDriverRequest, error) {
	fields, err := splitDriverCommand(request.Command)
	if err != nil {
		return mission.MissionCommanderDriverRequest{}, err
	}
	if len(fields) < 3 || fields[0] != "/rekit" || fields[1] != commands.PlanSubagents {
		return mission.MissionCommanderDriverRequest{}, fmt.Errorf("reviewer request must use /rekit plan-subagents")
	}
	args := fields[2:]
	if !driverCommandHasFlag(args, "-Target", "--target") {
		fields = append(fields, "-Target", ctx.Target)
	}
	if !driverCommandHasFlag(args, "-Pack", "--pack") {
		fields = append(fields, "-Pack", ctx.Pack)
	}
	if !driverCommandHasFlag(args, "-Format", "--format") {
		fields = append(fields, "-Format", "json")
	}
	request.Command = joinDriverCommand(fields)
	return request, nil
}

func parseBoundedReviewerRequest(ctx runtime.Context, request mission.MissionCommanderDriverRequest, apply bool) (Options, error) {
	if !request.CommandExecutable || request.Blocked {
		return Options{}, fmt.Errorf("reviewer driver request is not runnable: executable=%t blocked=%t kind=%s", request.CommandExecutable, request.Blocked, request.Kind)
	}
	if !request.RequiresReview {
		return Options{}, fmt.Errorf("reviewer driver request is outside the review-first runner boundary")
	}
	fields, err := splitDriverCommand(request.Command)
	if err != nil {
		return Options{}, err
	}
	if len(fields) < 3 || fields[0] != "/rekit" || fields[1] != commands.PlanSubagents {
		return Options{}, fmt.Errorf("reviewer driver request must use /rekit plan-subagents")
	}
	if err := validateBoundedReviewerTokens(fields[2:], apply); err != nil {
		return Options{}, err
	}
	inner, err := Parse(append([]string{"-Command", commands.PlanSubagents}, fields[2:]...))
	if err != nil {
		return Options{}, err
	}
	if strings.TrimSpace(inner.Target) == "" || !sameDriverStepPath(inner.Target, ctx.Target) {
		return Options{}, fmt.Errorf("reviewer driver request target must match the attached case")
	}
	if inner.PackProvided && !strings.EqualFold(strings.TrimSpace(inner.Pack), strings.TrimSpace(ctx.Pack)) {
		return Options{}, fmt.Errorf("reviewer driver request pack must match the attached case pack")
	}
	if strings.ToLower(strings.TrimSpace(inner.Format)) != "json" {
		return Options{}, fmt.Errorf("reviewer driver request must use -Format json")
	}
	if reviewerStepMode(inner) == "" {
		return Options{}, fmt.Errorf("reviewer driver request mode is outside the bounded reviewer runner contract")
	}
	return inner, nil
}

func validateBoundedReviewerTokens(fields []string, apply bool) error {
	valueFlags := map[string]bool{
		"-packetpath": true, "-shardid": true, "-lane": true, "-actor": true,
		"-reviewerharness": true, "-reviewersession": true,
		"-reviewerdispatchid": true, "-revieweroutcome": true, "-reviewerexitstatus": true,
		"-reviewerresultinputsourcepath": true, "-reviewerresultinputpath": true,
		"-reviewerresultsourcepath": true, "-expectedpromptsha256": true,
		"-expectedreviewerdispatchbindingsha256": true, "-expectedreviewerdispatchreceiptsha256": true,
		"-expectedreviewerresultinputsha256": true, "-expectedsourcesha256": true,
		"-expectedcandidatesha256": true,
		"-target":                  true, "-pack": true, "-format": true,
	}
	switchFlags := map[string]bool{
		"-repairreviewerpromptartifact": true, "-recordreviewerdispatch": true,
		"-recordreviewercompletion": true, "-savereviewerresultinput": true,
		"-capturereviewerresultsource": true, "-stagereviewerresult": true,
		"-collectreviewerresult": true, "-readyreviewerresults": true,
		"-whatif": true, "-apply": true,
	}
	canonical := func(value string) string {
		value = strings.ToLower(strings.TrimSpace(value))
		value = strings.TrimPrefix(value, "--")
		value = strings.TrimPrefix(value, "-")
		return "-" + strings.ReplaceAll(value, "-", "")
	}
	seen := map[string]bool{}
	modeCount := 0
	for idx := 0; idx < len(fields); idx++ {
		field := fields[idx]
		if !strings.HasPrefix(field, "-") {
			return fmt.Errorf("reviewer driver request contains unsupported positional argument %s", field)
		}
		key := canonical(strings.SplitN(field, "=", 2)[0])
		if seen[key] {
			return fmt.Errorf("reviewer driver request repeats flag %s", field)
		}
		seen[key] = true
		if switchFlags[key] {
			if key != "-whatif" && key != "-apply" {
				modeCount++
			}
			continue
		}
		if !valueFlags[key] {
			return fmt.Errorf("reviewer driver request contains unsupported flag %s", field)
		}
		if strings.Contains(field, "=") {
			continue
		}
		if idx+1 >= len(fields) || strings.HasPrefix(fields[idx+1], "-") {
			return fmt.Errorf("reviewer driver request flag %s is missing a value", field)
		}
		if strings.HasPrefix(fields[idx+1], "<") && strings.HasSuffix(fields[idx+1], ">") {
			return fmt.Errorf("reviewer driver request contains unresolved placeholder %s", fields[idx+1])
		}
		idx++
	}
	if modeCount != 1 {
		return fmt.Errorf("reviewer driver request requires exactly one bounded reviewer mode")
	}
	hasWhatIf := seen["-whatif"]
	hasApply := seen["-apply"]
	if apply && (!hasApply || hasWhatIf) {
		return fmt.Errorf("returned reviewer driver request must be Apply-only")
	}
	if !apply && (!hasWhatIf || hasApply) {
		return fmt.Errorf("current reviewer driver request must be WhatIf-only")
	}
	return nil
}

func reviewerStepMode(opt Options) string {
	modes := []struct {
		active bool
		name   string
	}{
		{opt.RepairReviewerPromptArtifact, "repair-prompt"},
		{opt.RecordReviewerDispatch, "record-dispatch"},
		{opt.RecordReviewerCompletion, "record-completion"},
		{opt.SaveReviewerResultInput, "save-result-input"},
		{opt.CaptureReviewerResultSource, "source-capture"},
		{opt.StageReviewerResult, "stage-candidate"},
		{opt.CollectReviewerResult, "collect-result"},
		{opt.ReadyReviewerResults, "intake-results"},
	}
	selected := ""
	for _, mode := range modes {
		if !mode.active {
			continue
		}
		if selected != "" {
			return ""
		}
		selected = mode.name
	}
	return selected
}

func previewReviewerStep(ctx runtime.Context, opt Options) (any, error) {
	return executeReviewerStep(ctx, opt)
}

func applyReviewerStep(ctx runtime.Context, request mission.MissionCommanderDriverRequest) (any, error) {
	opt, err := parseBoundedReviewerRequest(ctx, request, true)
	if err != nil {
		return nil, err
	}
	return executeReviewerStep(ctx, opt)
}

func executeReviewerStep(ctx runtime.Context, opt Options) (any, error) {
	switch reviewerStepMode(opt) {
	case "repair-prompt":
		return subagents.RepairReviewerPromptArtifact(ctx.RepoRoot, ctx.Target, ctx.Pack, subagents.ReviewerPromptArtifactRepairOptions{PacketPath: opt.PacketPath, ShardID: opt.ShardID, Lane: opt.Note.Lane, Actor: opt.Note.Actor, ExpectedPromptSHA256: opt.ExpectedPromptSHA256, WhatIf: opt.WhatIf})
	case "record-dispatch":
		return subagents.RecordReviewerSessionDispatch(ctx.RepoRoot, ctx.Target, ctx.Pack, subagents.ReviewerSessionDispatchOptions{PacketPath: opt.PacketPath, ShardID: opt.ShardID, Lane: opt.Note.Lane, Actor: opt.Note.Actor, ReviewerHarness: opt.ReviewerHarness, ReviewerSession: opt.ReviewerSession, ExpectedBindingSHA256: opt.ExpectedReviewerDispatchBindingSHA256, WhatIf: opt.WhatIf})
	case "record-completion":
		return subagents.RecordReviewerSessionCompletion(ctx.RepoRoot, ctx.Target, ctx.Pack, subagents.ReviewerSessionCompletionOptions{PacketPath: opt.PacketPath, DispatchID: opt.ReviewerDispatchID, Lane: opt.Note.Lane, Actor: opt.Note.Actor, Outcome: opt.ReviewerOutcome, ExitStatus: opt.ReviewerExitStatus, ReviewerResultInputPath: opt.ReviewerResultInputPath, ExpectedDispatchReceiptSHA256: opt.ExpectedReviewerDispatchReceiptSHA256, ExpectedReviewerResultSHA256: opt.ExpectedReviewerResultInputSHA256, WhatIf: opt.WhatIf})
	case "save-result-input":
		return subagents.SaveReviewerResultInput(ctx.RepoRoot, ctx.Target, ctx.Pack, subagents.ReviewerResultInputSaveOptions{PacketPath: opt.PacketPath, ShardID: opt.ShardID, SourcePath: opt.ReviewerResultInputSourcePath, Lane: opt.Note.Lane, Actor: opt.Note.Actor, ExpectedReviewerResultSHA256: opt.ExpectedReviewerResultInputSHA256, WhatIf: opt.WhatIf})
	case "source-capture":
		return subagents.CaptureReviewerResultSource(ctx.RepoRoot, ctx.Target, ctx.Pack, subagents.ReviewerResultSourceCaptureOptions{PacketPath: opt.PacketPath, ShardID: opt.ShardID, InputPath: opt.ReviewerResultInputPath, Lane: opt.Note.Lane, Actor: opt.Note.Actor, ExpectedReviewerResultSHA256: opt.ExpectedReviewerResultInputSHA256, WhatIf: opt.WhatIf})
	case "stage-candidate":
		return subagents.StageReviewerResult(ctx.RepoRoot, ctx.Target, ctx.Pack, subagents.ReviewerResultStagingOptions{PacketPath: opt.PacketPath, ShardID: opt.ShardID, SourcePath: opt.ReviewerResultSourcePath, Lane: opt.Note.Lane, Actor: opt.Note.Actor, ExpectedSourceSHA256: opt.ExpectedSourceSHA256, WhatIf: opt.WhatIf})
	case "collect-result":
		return subagents.CollectReviewerResult(ctx.RepoRoot, ctx.Target, ctx.Pack, subagents.ReviewerResultCollectionOptions{PacketPath: opt.PacketPath, ShardID: opt.ShardID, Lane: opt.Note.Lane, Actor: opt.Note.Actor, ExpectedCandidateSHA256: opt.ExpectedCandidateSHA256, WhatIf: opt.WhatIf})
	case "intake-results":
		return subagents.IntakeReadyReviewerResults(ctx.RepoRoot, ctx.Target, ctx.Pack, subagents.ReviewerBatchIntakeOptions{PacketPath: opt.PacketPath, Lane: opt.Note.Lane, Actor: opt.Note.Actor, WhatIf: opt.WhatIf})
	default:
		return nil, fmt.Errorf("unsupported bounded reviewer step")
	}
}

func reviewerStepApplyRequest(result any) (mission.MissionCommanderDriverRequest, error) {
	request := reviewerStepResultQueue(result).CurrentDriverRequest
	if request == nil {
		return mission.MissionCommanderDriverRequest{}, fmt.Errorf("reviewer preview omitted a typed Apply driver request")
	}
	return *request, nil
}

func reviewerStepResultQueue(result any) mission.MissionCommanderActionQueue {
	switch value := result.(type) {
	case subagents.ReviewerPromptArtifactRepairResult:
		return value.MissionCommanderActionQueue
	case subagents.ReviewerSessionReceiptResult:
		return value.MissionCommanderActionQueue
	case subagents.ReviewerResultInputSaveResult:
		return value.MissionCommanderActionQueue
	case subagents.ReviewerResultSourceCaptureResult:
		return value.MissionCommanderActionQueue
	case subagents.ReviewerResultStagingResult:
		return value.MissionCommanderActionQueue
	case subagents.ReviewerResultCollectionResult:
		return value.MissionCommanderActionQueue
	case subagents.ReviewerBatchIntakeResult:
		return value.MissionCommanderActionQueue
	default:
		return mission.MissionCommanderActionQueue{}
	}
}

func reviewerStepResultApplied(result any) bool {
	switch value := result.(type) {
	case subagents.ReviewerPromptArtifactRepairResult:
		return value.Applied
	case subagents.ReviewerSessionReceiptResult:
		return value.Applied
	case subagents.ReviewerResultInputSaveResult:
		return value.Applied
	case subagents.ReviewerResultSourceCaptureResult:
		return value.Applied
	case subagents.ReviewerResultStagingResult:
		return value.Applied
	case subagents.ReviewerResultCollectionResult:
		return value.Applied
	case subagents.ReviewerBatchIntakeResult:
		return value.Applied
	default:
		return false
	}
}

func reviewerStepResultCommand(result any) string {
	switch value := result.(type) {
	case subagents.ReviewerPromptArtifactRepairResult:
		return value.Command
	case subagents.ReviewerSessionReceiptResult:
		return value.Command
	case subagents.ReviewerResultInputSaveResult:
		return value.Command
	case subagents.ReviewerResultSourceCaptureResult:
		return value.Command
	case subagents.ReviewerResultStagingResult:
		return value.Command
	case subagents.ReviewerResultCollectionResult:
		return value.Command
	case subagents.ReviewerBatchIntakeResult:
		return value.Command
	default:
		return ""
	}
}

func reviewerStepReceiptFor(ctx runtime.Context, current, apply mission.MissionCommanderDriverRequest, result any, refreshed statusInventory) (reviewerStepReceipt, error) {
	resultCommand := reviewerStepResultCommand(result)
	if resultCommand != commands.PlanSubagents {
		return reviewerStepReceipt{}, fmt.Errorf("reviewer step command result identity mismatch: result=%q", resultCommand)
	}
	if !driverStepRefreshCommandMatches(ctx, current.ExpectedReceipt.RefreshStatusCommand) {
		return reviewerStepReceipt{}, fmt.Errorf("reviewer step expected refresh receipt mismatch")
	}
	var refreshedRequest *mission.MissionCommanderDriverRequest
	if refreshed.CaseMission != nil {
		if pkg := refreshed.CaseMission.ReviewerDispatchIntakeSummary.OperatorPackage; pkg != nil {
			refreshedRequest = pkg.CurrentDriverRequest
		}
	}
	outcome := "not-applied"
	if reviewerStepResultApplied(result) {
		outcome = "applied"
	}
	return reviewerStepReceipt{
		State:                         "refreshed",
		Outcome:                       outcome,
		RequestedRunLoopStepID:        current.RunLoopStepID,
		RequestedState:                current.State,
		ExecutedCommand:               apply.Command,
		CommandResultCommand:          resultCommand,
		RefreshStatusCommand:          current.ExpectedReceipt.RefreshStatusCommand,
		RefreshStatusCommandMatched:   true,
		RefreshedCurrentDriverRequest: refreshedRequest,
		Boundary: []string{
			"receipt binds the reviewed reviewer operator request, matching typed Apply result, and explicit durable status refresh",
			"receipt does not imply reviewer session execution, heavy-tool execution, or authority/confirmed state",
		},
	}, nil
}

func validateReviewerStepOuterArgs(opt Options) error {
	valueFlags := map[string]bool{
		"-command": true, "-target": true, "-pack": true, "-format": true,
		"-expectedreviewerstepplansha256": true, "-actor": true,
		"-reviewerharness": true, "-reviewersession": true,
		"-reviewerresultinputsourcepath": true, "-revieweroutcome": true,
		"-reviewerexitstatus": true,
	}
	switchFlags := map[string]bool{"-whatif": true, "-apply": true}
	canonical := func(value string) string {
		value = strings.ToLower(strings.TrimSpace(value))
		value = strings.TrimPrefix(value, "--")
		value = strings.TrimPrefix(value, "-")
		return "-" + strings.ReplaceAll(value, "-", "")
	}
	seen := map[string]bool{}
	separatorSeen := false
	for idx := 0; idx < len(opt.rawArgs); idx++ {
		token := opt.rawArgs[idx]
		if token == "--" {
			if idx != 0 || separatorSeen {
				return fmt.Errorf("run-reviewer-step accepts -- only once at the start of the argument list")
			}
			separatorSeen = true
			continue
		}
		if !strings.HasPrefix(token, "-") {
			return fmt.Errorf("run-reviewer-step contains unsupported positional argument %s", token)
		}
		key := canonical(strings.SplitN(token, "=", 2)[0])
		if seen[key] {
			return fmt.Errorf("run-reviewer-step repeats flag %s", token)
		}
		seen[key] = true
		if switchFlags[key] {
			continue
		}
		if !valueFlags[key] {
			return fmt.Errorf("run-reviewer-step contains unsupported flag %s", token)
		}
		if !strings.Contains(token, "=") {
			if idx+1 >= len(opt.rawArgs) || strings.HasPrefix(opt.rawArgs[idx+1], "-") {
				return fmt.Errorf("run-reviewer-step flag %s is missing a value", token)
			}
			idx++
		}
	}
	return nil
}
