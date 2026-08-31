package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	"github.com/shuiyu486/re-context-kits/internal/rekit/lanemutation"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

type driverStepPlan struct {
	SchemaVersion                   int                                   `json:"schemaVersion"`
	Command                         string                                `json:"command"`
	CaseRoot                        string                                `json:"caseRoot"`
	Pack                            string                                `json:"pack"`
	IsMutation                      bool                                  `json:"isMutation"`
	Applied                         bool                                  `json:"applied"`
	ReviewRequired                  bool                                  `json:"reviewRequired"`
	RequiresConfirmation            bool                                  `json:"requiresConfirmation"`
	CurrentDriverRequest            mission.MissionCommanderDriverRequest `json:"currentDriverRequest"`
	PreviewResult                   any                                   `json:"previewResult"`
	ApplyDriverRequest              mission.MissionCommanderDriverRequest `json:"applyDriverRequest"`
	ExpectedDriverStepPlanSHA256    string                                `json:"expectedDriverStepPlanSha256"`
	ExpectedDriverStepPreviewSHA256 string                                `json:"expectedDriverStepPreviewSha256"`
	MissionCommanderActionQueue     mission.MissionCommanderActionQueue   `json:"missionCommanderActionQueue"`
	Receipt                         *driverStepReceipt                    `json:"receipt,omitempty"`
	RefreshedStatus                 *statusInventory                      `json:"refreshedStatus,omitempty"`
	Boundary                        []string                              `json:"boundary"`
}

type driverStepReceipt struct {
	State                         string                                 `json:"state"`
	Outcome                       string                                 `json:"outcome"`
	RequestedCommand              string                                 `json:"requestedCommand"`
	ExecutedCommand               string                                 `json:"executedCommand"`
	CommandResultCommand          string                                 `json:"commandResultCommand"`
	ExpectedReceiptCommand        string                                 `json:"expectedReceiptCommand"`
	ExpectedReceiptCommandMatched bool                                   `json:"expectedReceiptCommandMatched"`
	RefreshStatusCommand          string                                 `json:"refreshStatusCommand"`
	RefreshStatusCommandMatched   bool                                   `json:"refreshStatusCommandMatched"`
	RefreshedCurrentDriverRequest *mission.MissionCommanderDriverRequest `json:"refreshedCurrentDriverRequest,omitempty"`
	Boundary                      []string                               `json:"boundary"`
}

type driverStepPlanIdentity struct {
	CurrentDriverRequest mission.MissionCommanderDriverRequest `json:"currentDriverRequest"`
	ApplyDriverRequest   mission.MissionCommanderDriverRequest `json:"applyDriverRequest"`
	PreviewResult        any                                   `json:"previewResult"`
}

func runDriverStep(ctx runtime.Context, opt Options, out io.Writer) error {
	if !ctx.TargetProvided {
		return fmt.Errorf("run-driver-step requires -Target for an attached case")
	}
	if opt.WhatIf && opt.Apply {
		return fmt.Errorf("run-driver-step cannot combine -WhatIf and -Apply")
	}
	if !opt.WhatIf && !opt.Apply {
		return fmt.Errorf("run-driver-step requires -WhatIf or -Apply")
	}
	if strings.ToLower(strings.TrimSpace(opt.Format)) != "json" {
		return fmt.Errorf("run-driver-step supports only -Format json")
	}
	if err := validateDriverStepOuterArgs(opt); err != nil {
		return err
	}
	plan, err := buildDriverStepPlan(ctx, opt)
	if err != nil {
		return err
	}
	if opt.WhatIf {
		if strings.TrimSpace(opt.ExpectedDriverStepPlanSHA256) != "" {
			return fmt.Errorf("run-driver-step -WhatIf does not accept -ExpectedDriverStepPlanSha256")
		}
		diagnostics, err := buildDriverStepDiagnosticsDTO(plan, ctx.Target)
		if err != nil {
			return err
		}
		return writeJSON(out, diagnostics)
	}
	expected := strings.TrimSpace(opt.ExpectedDriverStepPlanSHA256)
	if expected == "" {
		return fmt.Errorf("run-driver-step -Apply requires -ExpectedDriverStepPlanSha256 from -WhatIf")
	}
	if !strings.EqualFold(expected, plan.ExpectedDriverStepPlanSHA256) {
		return fmt.Errorf("run-driver-step expected plan sha256 mismatch: got %s want %s", expected, plan.ExpectedDriverStepPlanSHA256)
	}
	plan, err = applyDriverStepPlan(ctx, opt, plan)
	if err != nil {
		return err
	}
	diagnostics, err := buildDriverStepDiagnosticsDTO(plan, ctx.Target)
	if err != nil {
		return err
	}
	return writeJSON(out, diagnostics)
}

func applyDriverStepPlan(ctx runtime.Context, opt Options, plan driverStepPlan) (driverStepPlan, error) {
	return applyDriverStepPlanWithLease(ctx, opt, plan, nil)
}

func applyDriverStepPlanWithLease(ctx runtime.Context, opt Options, plan driverStepPlan, lease *projectexecution.Lease) (driverStepPlan, error) {
	result, err := applyDriverStep(
		ctx,
		plan.ApplyDriverRequest,
		plan.ExpectedDriverStepPreviewSHA256,
		opt.currentLoopExecutionControlBinding,
	)
	if err != nil {
		return driverStepPlan{}, err
	}
	plan.IsMutation = true
	plan.Applied = driverStepResultApplied(result)
	plan.ReviewRequired = false
	plan.RequiresConfirmation = false
	plan.PreviewResult = result
	plan.MissionCommanderActionQueue = driverStepResultQueue(result)
	if currentStepBeforeStatusRefreshHook != nil {
		if err := currentStepBeforeStatusRefreshHook(commands.RunDriverStep); err != nil {
			return plan, fmt.Errorf("refresh status after driver step: %w", err)
		}
	}
	refreshOpt, err := optionsWithEffectiveSelectedCurrentLane(opt, plan.ApplyDriverRequest.Lane)
	if err != nil {
		return plan, fmt.Errorf("refresh status after driver step: %w", err)
	}
	refreshed, err := buildInvocationStatusInventoryAfterMutationWithLease(ctx, refreshOpt, lease)
	if err != nil {
		return plan, fmt.Errorf("refresh status after driver step: %w", err)
	}
	receipt, err := driverStepReceiptFor(ctx, plan.CurrentDriverRequest, plan.ApplyDriverRequest, result, refreshed)
	if err != nil {
		return plan, err
	}
	plan.Receipt = &receipt
	plan.RefreshedStatus = &refreshed
	return plan, nil
}

func buildDriverStepPlan(ctx runtime.Context, opt Options) (driverStepPlan, error) {
	status, err := buildInvocationStatusInventory(ctx, opt)
	if err != nil {
		return driverStepPlan{}, err
	}
	return buildDriverStepPlanFromStatus(ctx, status)
}

func buildDriverStepPlanFromStatus(ctx runtime.Context, status statusInventory) (driverStepPlan, error) {
	if status.MissionControlRunbook == nil || status.MissionControlRunbook.CurrentDriverRequest == nil {
		return driverStepPlan{}, fmt.Errorf("run-driver-step requires missionControlRunbook.currentDriverRequest")
	}
	if status.MissionControlRunbook.Scope != "case" {
		return driverStepPlan{}, fmt.Errorf("run-driver-step supports only case-scoped current driver requests; got %q", status.MissionControlRunbook.Scope)
	}
	return buildDriverStepPlanFromRequest(ctx, *status.MissionControlRunbook.CurrentDriverRequest)
}

func buildDriverStepPlanFromRequest(ctx runtime.Context, request mission.MissionCommanderDriverRequest) (driverStepPlan, error) {
	if lane := strings.TrimSpace(request.Lane); lane != "" {
		bindSelectedLaneDriverRequest(&request, lane)
	}
	previewOpt, err := parseBoundedDriverRequest(ctx, request, false)
	if err != nil {
		return driverStepPlan{}, err
	}
	preview, err := previewDriverStep(ctx, previewOpt)
	if err != nil {
		return driverStepPlan{}, err
	}
	if continuePreview, ok := preview.(workstream.ContinueResult); ok {
		if err := validateContinuePreviewApplyRequest(continuePreview); err != nil {
			return driverStepPlan{}, err
		}
	}
	applyRequest, err := driverStepApplyRequest(preview)
	if err != nil {
		return driverStepPlan{}, err
	}
	applyRequest, err = qualifyDriverStepApplyRequest(ctx, applyRequest)
	if err != nil {
		return driverStepPlan{}, err
	}
	if lane := strings.TrimSpace(request.Lane); lane != "" {
		bindSelectedLaneDriverRequest(&applyRequest, lane)
	}
	if _, err := parseBoundedDriverRequest(ctx, applyRequest, true); err != nil {
		return driverStepPlan{}, fmt.Errorf("returned apply driver request: %w", err)
	}
	if request.Invocation == nil || applyRequest.Invocation == nil || request.Invocation.Validate() != nil || applyRequest.Invocation.Validate() != nil || request.Invocation.Command != applyRequest.Invocation.Command {
		return driverStepPlan{}, fmt.Errorf("returned apply driver request command differs from current preview request")
	}
	previewEncoded, err := json.Marshal(preview)
	if err != nil {
		return driverStepPlan{}, err
	}
	previewSum := sha256.Sum256(previewEncoded)
	previewSHA256 := hex.EncodeToString(previewSum[:])
	if completion, ok := preview.(workstream.CompleteResult); ok {
		previewSHA256 = completion.CompletionPlanSHA256
	}
	identity := driverStepPlanIdentity{CurrentDriverRequest: request, ApplyDriverRequest: applyRequest, PreviewResult: preview}
	encoded, err := json.Marshal(identity)
	if err != nil {
		return driverStepPlan{}, err
	}
	sum := sha256.Sum256(encoded)
	return driverStepPlan{
		SchemaVersion:                   1,
		Command:                         commands.RunDriverStep,
		CaseRoot:                        ctx.Target,
		Pack:                            ctx.Pack,
		IsMutation:                      false,
		Applied:                         false,
		ReviewRequired:                  true,
		RequiresConfirmation:            true,
		CurrentDriverRequest:            request,
		PreviewResult:                   preview,
		ApplyDriverRequest:              applyRequest,
		ExpectedDriverStepPlanSHA256:    hex.EncodeToString(sum[:]),
		ExpectedDriverStepPreviewSHA256: previewSHA256,
		MissionCommanderActionQueue:     driverStepResultQueue(preview),
		Boundary: []string{
			"runner accepts only the exact current start, continue, reconcile, or complete WhatIf request and its returned typed Apply request",
			"Apply is hash-bound to the reviewed current request, returned Apply request, and deterministic preview result",
			"the Go runtime does not invoke a shell, spawn or poll sessions, execute reviewer/adapter/heavy tools, or write authority/confirmed state",
			"status is rebuilt after Apply before follow-up work is selected",
		},
	}, nil
}

func validateDriverStepOuterArgs(opt Options) error {
	valueFlags := map[string]bool{
		"-command": true, "--command": true,
		"-target": true, "--target": true,
		"-pack": true, "--pack": true,
		"-lane": true, "--lane": true,
		"-format": true, "--format": true,
		"-expecteddriverstepplansha256": true, "--expected-driver-step-plan-sha256": true,
	}
	switchFlags := map[string]bool{
		"-whatif": true, "--what-if": true,
		"-apply": true, "--apply": true,
	}
	seen := map[string]bool{}
	separatorSeen := false
	for i := 0; i < len(opt.rawArgs); i++ {
		token := opt.rawArgs[i]
		if token == "--" {
			if i != 0 || separatorSeen {
				return fmt.Errorf("run-driver-step accepts -- only once at the start of the argument list")
			}
			separatorSeen = true
			continue
		}
		key := strings.ToLower(strings.SplitN(token, "=", 2)[0])
		if !strings.HasPrefix(key, "-") {
			return fmt.Errorf("run-driver-step contains unsupported positional argument %s", token)
		}
		canonical := key
		switch key {
		case "--command":
			canonical = "-command"
		case "--target":
			canonical = "-target"
		case "--pack":
			canonical = "-pack"
		case "--lane":
			canonical = "-lane"
		case "--format":
			canonical = "-format"
		case "--expected-driver-step-plan-sha256":
			canonical = "-expecteddriverstepplansha256"
		case "--what-if":
			canonical = "-whatif"
		case "--apply":
			canonical = "-apply"
		}
		if seen[canonical] {
			return fmt.Errorf("run-driver-step repeats flag %s", token)
		}
		seen[canonical] = true
		if switchFlags[key] {
			continue
		}
		if !valueFlags[key] {
			return fmt.Errorf("run-driver-step contains unsupported flag %s", token)
		}
		if !strings.Contains(token, "=") {
			if i+1 >= len(opt.rawArgs) || strings.HasPrefix(opt.rawArgs[i+1], "-") {
				return fmt.Errorf("run-driver-step flag %s is missing a value", token)
			}
			i++
		}
	}
	return nil
}

func qualifyDriverStepApplyRequest(ctx runtime.Context, request mission.MissionCommanderDriverRequest) (mission.MissionCommanderDriverRequest, error) {
	if request.Invocation == nil {
		command, err := projectVisibleCommand(ctx.Target, request.Command)
		if err != nil {
			return mission.MissionCommanderDriverRequest{}, err
		}
		request.Command = command
		request, err = mission.MissionCommanderDriverRequestWithTypedCommand(request)
		if err != nil {
			return mission.MissionCommanderDriverRequest{}, err
		}
	} else if err := mission.ValidateMissionCommanderDriverRequest(request); err != nil {
		return mission.MissionCommanderDriverRequest{}, err
	}
	if request.Invocation == nil || !boundedDriverStepCommand(request.Invocation.Command) {
		return mission.MissionCommanderDriverRequest{}, fmt.Errorf("returned Apply request is outside the bounded driver command allowlist")
	}
	projection, err := resolveProjectPublicProjection(ctx.Target)
	if err != nil {
		return mission.MissionCommanderDriverRequest{}, err
	}
	args := append([]string{}, request.Invocation.Arguments...)
	if request.Invocation.Command == commands.Continue {
		if driverCommandHasFlag(args, "-WhatIf", "--what-if") || !driverCommandHasFlag(args, "-Apply", "--apply") {
			return mission.MissionCommanderDriverRequest{}, fmt.Errorf("continue Apply request must come from the exact workstream preview")
		}
	} else if driverCommandHasFlag(args, "-WhatIf", "--what-if") {
		args = driverStepReplacePhase(args, "-Apply")
	} else if !driverCommandHasFlag(args, "-Apply", "--apply") {
		args = append(args, "-Apply")
	}
	if !driverCommandHasFlag(args, "-Target", "--target") {
		args = append(args, "-Target", ctx.Target)
	}
	if !driverCommandHasFlag(args, "-Pack", "--pack") {
		args = append(args, "-Pack", ctx.Pack)
	}
	if !driverCommandHasFlag(args, "-Format", "--format") {
		args = append(args, "-Format", "json")
	}
	invocation, err := commands.NewPublicInvocation(request.Invocation.Command, args...)
	if err != nil {
		return mission.MissionCommanderDriverRequest{}, err
	}
	command, err := invocation.RenderForEntrypoint(projection.entrypoint)
	if err != nil {
		return mission.MissionCommanderDriverRequest{}, err
	}
	request.Invocation = &invocation
	request.Command = command
	request.ExpectedReceipt.Command = command
	if err := mission.ValidateMissionCommanderDriverRequest(request); err != nil {
		return mission.MissionCommanderDriverRequest{}, err
	}
	return request, nil
}

func driverStepReplacePhase(fields []string, phase string) []string {
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		if strings.EqualFold(field, "-WhatIf") || strings.EqualFold(field, "--what-if") ||
			strings.EqualFold(field, "-Apply") || strings.EqualFold(field, "--apply") {
			continue
		}
		out = append(out, field)
	}
	return append(out, phase)
}

func parseBoundedDriverRequest(ctx runtime.Context, request mission.MissionCommanderDriverRequest, apply bool) (Options, error) {
	if !request.CommandExecutable || request.Blocked {
		return Options{}, fmt.Errorf("current driver request is not runnable: executable=%t blocked=%t kind=%s", request.CommandExecutable, request.Blocked, request.Kind)
	}
	if !apply && !request.RequiresReview {
		return Options{}, fmt.Errorf("current driver request is outside the review-first runner boundary")
	}
	if err := mission.ValidateMissionCommanderDriverRequest(request); err != nil {
		return Options{}, err
	}
	if request.Invocation == nil {
		return Options{}, fmt.Errorf("driver request command requires a typed invocation")
	}
	projection, err := resolveProjectPublicProjection(ctx.Target)
	if err != nil {
		return Options{}, err
	}
	fields, err := splitDriverCommand(request.Command)
	if err != nil || len(fields) < 2 || fields[0] != projection.entrypoint {
		return Options{}, fmt.Errorf("driver request command must use the canonical project entrypoint")
	}
	if !boundedDriverStepCommand(request.Invocation.Command) {
		return Options{}, fmt.Errorf("driver request command %q is outside the run-driver-step allowlist", request.Invocation.Command)
	}
	if err := validateBoundedDriverTokens(request.Invocation.Command, request.Invocation.Arguments, apply); err != nil {
		return Options{}, err
	}
	args, err := request.Invocation.CLIArgs()
	if err != nil {
		return Options{}, err
	}
	inner, err := Parse(args)
	if err != nil {
		return Options{}, err
	}
	if apply {
		if !inner.Apply || inner.WhatIf {
			return Options{}, fmt.Errorf("returned driver request must be Apply-only")
		}
	} else if !inner.WhatIf || inner.Apply {
		return Options{}, fmt.Errorf("current driver request must be WhatIf-only")
	}
	if strings.TrimSpace(inner.Target) == "" || !sameDriverStepPath(inner.Target, ctx.Target) {
		return Options{}, fmt.Errorf("driver request target must match the attached case")
	}
	if inner.PackProvided && !strings.EqualFold(strings.TrimSpace(inner.Pack), strings.TrimSpace(ctx.Pack)) {
		return Options{}, fmt.Errorf("driver request pack must match the attached case pack")
	}
	if strings.ToLower(strings.TrimSpace(inner.Format)) != "json" {
		return Options{}, fmt.Errorf("driver request must use -Format json")
	}
	if inner.CreateCandidates || inner.Review || inner.Force || inner.List || wantsReviewArtifacts(inner) {
		return Options{}, fmt.Errorf("driver request contains flags outside the bounded runner contract")
	}
	return inner, nil
}

func validateBoundedDriverTokens(command string, fields []string, apply bool) error {
	valueFlags := map[string]bool{
		"-target": true, "--target": true,
		"-pack": true, "--pack": true,
		"-format": true, "--format": true,
		"-name": true, "--name": true,
		"-lane": true, "--lane": true,
		"-interventionid": true, "--intervention-id": true,
		"-executor": true, "--executor": true,
		"-actor": true, "--actor": true,
		"-reason": true, "--reason": true,
		"-summary": true, "--summary": true,
		"-evidencerefs": true, "--evidence-refs": true,
		"-expectedcontinueplansha256": true, "--expected-continue-plan-sha256": true,
		"-expectedcompleteplansha256": true, "--expected-complete-plan-sha256": true,
		"-expectedexecutorgeneration": true, "--expected-executor-generation": true,
	}
	switchFlags := map[string]bool{
		"-whatif": true, "--what-if": true,
		"-apply": true, "--apply": true,
	}
	canonicalFlag := func(key string) string {
		key = strings.ToLower(strings.TrimSpace(key))
		switch key {
		case "--target":
			return "-target"
		case "--pack":
			return "-pack"
		case "--format":
			return "-format"
		case "--name":
			return "-name"
		case "--lane":
			return "-lane"
		case "--intervention-id":
			return "-interventionid"
		case "--executor":
			return "-executor"
		case "--actor":
			return "-actor"
		case "--reason":
			return "-reason"
		case "--summary":
			return "-summary"
		case "--evidence-refs":
			return "-evidencerefs"
		case "--expected-continue-plan-sha256":
			return "-expectedcontinueplansha256"
		case "--expected-complete-plan-sha256":
			return "-expectedcompleteplansha256"
		case "--expected-executor-generation":
			return "-expectedexecutorgeneration"
		case "--what-if":
			return "-whatif"
		case "--apply":
			return "-apply"
		default:
			return key
		}
	}
	seen := map[string]bool{}
	positionals := 0
	for i := 0; i < len(fields); i++ {
		field := fields[i]
		if !strings.HasPrefix(field, "-") {
			positionals++
			if positionals > 1 {
				return fmt.Errorf("driver request has too many positional arguments")
			}
			continue
		}
		key := canonicalFlag(field)
		if seen[key] {
			return fmt.Errorf("driver request repeats flag %s", field)
		}
		seen[key] = true
		if switchFlags[strings.ToLower(field)] || switchFlags[key] {
			continue
		}
		if !valueFlags[strings.ToLower(field)] && !valueFlags[key] {
			return fmt.Errorf("driver request contains unsupported flag %s", field)
		}
		if i+1 >= len(fields) || strings.HasPrefix(fields[i+1], "-") {
			return fmt.Errorf("driver request flag %s is missing a value", field)
		}
		i++
	}
	selectorKinds := positionals
	if seen["-lane"] {
		selectorKinds++
	}
	switch command {
	case commands.Start:
		if seen["-name"] {
			selectorKinds++
		}
		if seen["-lane"] && seen["-name"] {
			selectorKinds--
		}
		if selectorKinds != 1 {
			return fmt.Errorf("start driver request requires exactly one lane selector")
		}
		if seen["-interventionid"] || seen["-expectedexecutorgeneration"] || seen["-summary"] || (seen["-actor"] || seen["-reason"]) && !seen["-executor"] {
			return fmt.Errorf("start driver request contains flags outside its bounded contract")
		}
	case commands.Continue:
		if selectorKinds != 1 {
			return fmt.Errorf("continue driver request requires exactly one lane selector")
		}
		if seen["-name"] || seen["-interventionid"] || seen["-actor"] || seen["-reason"] || seen["-summary"] || seen["-expectedcompleteplansha256"] {
			return fmt.Errorf("continue driver request contains unsupported flag(s) for its bounded contract")
		}
		if apply != seen["-expectedcontinueplansha256"] {
			return fmt.Errorf("continue driver request plan hash does not match preview/apply mode")
		}
	case commands.Complete:
		if selectorKinds != 1 {
			return fmt.Errorf("complete driver request requires exactly one lane selector")
		}
		if !seen["-actor"] || !seen["-reason"] || !seen["-evidencerefs"] {
			return fmt.Errorf("complete driver request requires -Actor, -Reason, and -EvidenceRefs")
		}
		if seen["-name"] || seen["-interventionid"] || seen["-executor"] || seen["-summary"] || seen["-expectedexecutorgeneration"] {
			return fmt.Errorf("complete driver request contains flags outside its bounded contract")
		}
		if apply != seen["-expectedcompleteplansha256"] {
			return fmt.Errorf("complete driver request plan hash does not match preview/apply mode")
		}
	case commands.Reconcile:
		if selectorKinds != 1 {
			return fmt.Errorf("reconcile driver request requires exactly one lane selector")
		}
		if !seen["-interventionid"] {
			return fmt.Errorf("reconcile driver request requires -InterventionId")
		}
		if seen["-name"] || seen["-expectedexecutorgeneration"] {
			return fmt.Errorf("reconcile driver request contains flags outside its bounded contract")
		}
	default:
		return fmt.Errorf("driver request command %q is outside the run-driver-step allowlist", command)
	}
	hasWhatIf := seen["-whatif"]
	hasApply := seen["-apply"]
	if apply && (!hasApply || hasWhatIf) {
		return fmt.Errorf("returned driver request must be Apply-only")
	}
	if !apply && (!hasWhatIf || hasApply) {
		return fmt.Errorf("current driver request must be WhatIf-only")
	}
	return nil
}

func previewDriverStep(ctx runtime.Context, opt Options) (any, error) {
	switch opt.Command {
	case commands.Start:
		return workstream.StartPreview(ctx.RepoRoot, ctx.Target, ctx.Pack, opt.Start)
	case commands.Continue:
		return workstream.ContinuePreview(ctx.RepoRoot, ctx.Target, ctx.Pack, opt.Continue)
	case commands.Reconcile:
		return workstream.ReconcilePreview(ctx.RepoRoot, ctx.Target, ctx.Pack, opt.Reconcile)
	case commands.Complete:
		return workstream.CompletePreview(ctx.RepoRoot, ctx.Target, ctx.Pack, opt.Complete)
	default:
		return nil, fmt.Errorf("unsupported bounded driver preview: %s", opt.Command)
	}
}

var (
	driverStepApplyBeforeMutationHook    func(string) error
	driverStepAfterPreviewValidationHook func() error
)

func applyDriverStep(
	ctx runtime.Context,
	request mission.MissionCommanderDriverRequest,
	expectedPreviewSHA256 string,
	binding *executioncontrol.Binding,
) (any, error) {
	opt, err := parseBoundedDriverRequest(ctx, request, true)
	if err != nil {
		return nil, currentStepZeroProgressError{cause: err}
	}
	if driverStepApplyBeforeMutationHook != nil {
		if err := driverStepApplyBeforeMutationHook(opt.Command); err != nil {
			return nil, currentStepZeroProgressError{cause: err}
		}
	}
	validateLane := func(lease *lanemutation.Lease) error {
		if binding == nil {
			return nil
		}
		return executioncontrol.RequireCurrentBindingWithLease(ctx.Target, lease, *binding)
	}
	validateProject := func(lease *lanemutation.Lease) error {
		if binding == nil {
			return nil
		}
		return executioncontrol.RequireCurrentBindingWithProjectLease(ctx.Target, lease, *binding)
	}
	switch opt.Command {
	case commands.Start:
		opt.Start.ExpectedPreviewSHA256 = expectedPreviewSHA256
		result, err := workstream.StartApplyValidated(ctx.RepoRoot, ctx.Target, ctx.Pack, opt.Start, validateLane)
		if workstream.IsZeroProgress(err) {
			return nil, currentStepZeroProgressError{cause: err}
		}
		return result, err
	case commands.Continue:
		opt.Continue.ExpectedPreviewSHA256 = expectedPreviewSHA256
		opt.Continue.AfterPreviewValidation = driverStepAfterPreviewValidationHook
		result, err := workstream.ContinueApplyValidated(ctx.RepoRoot, ctx.Target, ctx.Pack, opt.Continue, validateLane)
		if workstream.IsZeroProgress(err) {
			return nil, currentStepZeroProgressError{cause: err}
		}
		return result, err
	case commands.Reconcile:
		opt.Reconcile.ExpectedPreviewSHA256 = expectedPreviewSHA256
		result, err := workstream.ReconcileApplyValidated(ctx.RepoRoot, ctx.Target, ctx.Pack, opt.Reconcile, validateLane)
		if workstream.IsZeroProgress(err) {
			return nil, currentStepZeroProgressError{cause: err}
		}
		return result, err
	case commands.Complete:
		return workstream.CompleteApplyValidated(ctx.RepoRoot, ctx.Target, ctx.Pack, opt.Complete, validateProject)
	default:
		return nil, fmt.Errorf("unsupported bounded driver apply: %s", opt.Command)
	}
}

func validateContinuePreviewApplyRequest(result workstream.ContinueResult) error {
	if result.Blocked {
		if result.ContinueOwnerGuardRecovery != nil {
			recovery := result.ContinueOwnerGuardRecovery
			return fmt.Errorf("continue preview did not return an exact hash-bound Apply request: preview is blocked by the current executor owner guard: reason=%s received=%s/%d current=%s/%d request=%s", recovery.Reason, recovery.ReceivedExecutor, recovery.ReceivedExecutorGeneration, recovery.CurrentExecutor, recovery.CurrentExecutorGeneration, recovery.CurrentContinueCommand)
		}
		return fmt.Errorf("continue preview did not return an exact hash-bound Apply request: preview is blocked")
	}
	planSHA256 := strings.TrimSpace(result.ContinuePlanSHA256)
	decodedPlanSHA256, err := hex.DecodeString(planSHA256)
	if err != nil || len(decodedPlanSHA256) != sha256.Size {
		return fmt.Errorf("continue preview did not return an exact hash-bound Apply request: missing or invalid continue plan sha256")
	}
	request := result.MissionCommanderActionQueue.CurrentDriverRequest
	if request == nil {
		return fmt.Errorf("continue preview did not return an exact hash-bound Apply request: typed driver request is missing")
	}
	if request.Blocked || !request.CommandExecutable || request.Invocation == nil {
		return fmt.Errorf("continue preview did not return an exact hash-bound Apply request: typed driver request is blocked or not executable")
	}
	if current := result.MissionCommanderActionQueue.CurrentAction; current == nil ||
		current.Source != "missionCommanderActions" ||
		current.State != "needs-continue-apply" ||
		current.Blocked || current.Invocation == nil ||
		current.Command != request.Command {
		return fmt.Errorf("continue preview did not return an exact hash-bound Apply request: action queue parity is invalid")
	}
	invocation := *request.Invocation
	if invocation.Command != commands.Continue {
		return fmt.Errorf("continue preview did not return an exact hash-bound Apply request: returned command is %q", invocation.Command)
	}
	hasApply := invocation.HasFlag("-Apply") || invocation.HasFlag("--apply")
	hasWhatIf := invocation.HasFlag("-WhatIf") || invocation.HasFlag("--what-if")
	if !hasApply || hasWhatIf {
		return fmt.Errorf("continue preview did not return an exact hash-bound Apply request: returned invocation is not Apply-only")
	}
	boundPlanSHA256, present, valid := invocation.FlagValue(
		"-ExpectedContinuePlanSha256",
		"--expected-continue-plan-sha256",
	)
	if !present || !valid || !strings.EqualFold(strings.TrimSpace(boundPlanSHA256), planSHA256) {
		return fmt.Errorf("continue preview did not return an exact hash-bound Apply request: returned invocation plan hash does not match the preview")
	}
	if err := commands.ValidateExecutableContinueInvocation(invocation); err != nil {
		return fmt.Errorf("continue preview did not return an exact hash-bound Apply request: %w", err)
	}
	if err := mission.ValidateMissionCommanderDriverRequest(*request); err != nil {
		return fmt.Errorf("continue preview did not return an exact hash-bound Apply request: %w", err)
	}
	return nil
}

func driverStepApplyRequest(result any) (mission.MissionCommanderDriverRequest, error) {
	if completion, ok := result.(workstream.CompleteResult); ok {
		if completion.Blocked || strings.TrimSpace(completion.ApplyCommand) == "" || strings.TrimSpace(completion.CompletionPlanSHA256) == "" {
			return mission.MissionCommanderDriverRequest{}, fmt.Errorf("complete preview omitted an unblocked exact Apply request")
		}
		return mission.MissionCommanderDriverRequest{
			Kind:              "execute-command",
			RunLoopStepID:     "apply-lane-completion",
			State:             "completion-apply-ready",
			Source:            "laneCompletion.completePreview",
			Lane:              completion.Lane.ID,
			Label:             completion.Selector,
			ActionID:          "apply-lane-completion-" + completion.Lane.ID,
			Command:           completion.ApplyCommand,
			CommandExecutable: true,
			Boundary: []string{
				"Apply consumes the exact CompletionPlanSHA256 returned by workstream CompletePreview",
				"completion remains subject to owner, evidence, blocker, lease, and currentness revalidation",
			},
		}, nil
	}
	request := driverStepResultQueue(result).CurrentDriverRequest
	if request == nil {
		return mission.MissionCommanderDriverRequest{}, fmt.Errorf("driver preview omitted a typed Apply driver request")
	}
	return *request, nil
}

func driverStepResultQueue(result any) mission.MissionCommanderActionQueue {
	switch value := result.(type) {
	case workstream.StartResult:
		return value.MissionCommanderActionQueue
	case workstream.ContinueResult:
		return value.MissionCommanderActionQueue
	case workstream.ReconcileResult:
		return value.MissionCommanderActionQueue
	case workstream.CompleteResult:
		return value.MissionCommanderActionQueue
	default:
		return mission.MissionCommanderActionQueue{}
	}
}

func driverStepResultApplied(result any) bool {
	switch value := result.(type) {
	case workstream.StartResult:
		return value.Applied
	case workstream.ContinueResult:
		return value.Applied
	case workstream.ReconcileResult:
		return value.Applied
	case workstream.CompleteResult:
		return value.Applied
	default:
		return false
	}
}

func driverStepResultCommand(result any) string {
	switch value := result.(type) {
	case workstream.StartResult:
		return value.Command
	case workstream.ContinueResult:
		return value.Command
	case workstream.ReconcileResult:
		return value.Command
	case workstream.CompleteResult:
		return value.Command
	default:
		return ""
	}
}

func driverStepReceiptFor(ctx runtime.Context, current, apply mission.MissionCommanderDriverRequest, result any, refreshed statusInventory) (driverStepReceipt, error) {
	resultCommand := driverStepResultCommand(result)
	if resultCommand == "" || !strings.EqualFold(resultCommand, driverStepCommandName(apply.Command)) {
		return driverStepReceipt{}, fmt.Errorf("driver step command result identity mismatch: result=%q apply=%q", resultCommand, apply.Command)
	}
	expectedCommandMatched := strings.TrimSpace(current.ExpectedReceipt.Command) == strings.TrimSpace(current.Command)
	refreshMatched := driverStepRefreshCommandMatches(ctx, current.ExpectedReceipt.RefreshStatusCommand)
	if !expectedCommandMatched || !refreshMatched {
		return driverStepReceipt{}, fmt.Errorf("driver step expected receipt mismatch: commandMatched=%t refreshMatched=%t", expectedCommandMatched, refreshMatched)
	}
	var refreshedRequest *mission.MissionCommanderDriverRequest
	if refreshed.MissionControlRunbook != nil {
		refreshedRequest = refreshed.MissionControlRunbook.CurrentDriverRequest
	}
	outcome := "not-applied"
	if driverStepResultApplied(result) {
		outcome = "applied"
	}
	return driverStepReceipt{
		State:                         "refreshed",
		Outcome:                       outcome,
		RequestedCommand:              current.Command,
		ExecutedCommand:               apply.Command,
		CommandResultCommand:          resultCommand,
		ExpectedReceiptCommand:        current.ExpectedReceipt.Command,
		ExpectedReceiptCommandMatched: true,
		RefreshStatusCommand:          current.ExpectedReceipt.RefreshStatusCommand,
		RefreshStatusCommandMatched:   true,
		RefreshedCurrentDriverRequest: refreshedRequest,
		Boundary: []string{
			"receipt validates the current request identity, returned Apply request result, and explicit status refresh route",
			"receipt does not imply authority/confirmed state or heavy-tool execution",
		},
	}, nil
}

func driverStepRefreshCommandMatches(ctx runtime.Context, command string) bool {
	fields, err := splitDriverCommand(command)
	if err != nil || len(fields) < 2 || !driverStepEntrypointMatches(ctx, fields[0]) {
		return false
	}
	opt, err := Parse(append([]string{"-Command", fields[1]}, fields[2:]...))
	return err == nil && opt.Command == commands.Status && strings.TrimSpace(opt.Target) != "" && sameDriverStepPath(opt.Target, ctx.Target) && strings.EqualFold(strings.TrimSpace(opt.Format), "compact-json") && !opt.Apply && !opt.WhatIf
}

func driverStepEntrypointMatches(ctx runtime.Context, entrypoint string) bool {
	projection, err := resolveProjectPublicProjection(ctx.Target)
	return err == nil && entrypoint == projection.entrypoint
}

func boundedDriverStepCommand(command string) bool {
	switch strings.ToLower(strings.TrimSpace(command)) {
	case commands.Start, commands.Continue, commands.Reconcile, commands.Complete:
		return true
	default:
		return false
	}
}

func driverStepCommandName(command string) string {
	fields, err := splitDriverCommand(command)
	if err != nil || len(fields) < 2 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(fields[1]))
}

func sameDriverStepPath(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	return leftErr == nil && rightErr == nil && strings.EqualFold(filepath.Clean(leftAbs), filepath.Clean(rightAbs))
}

func driverCommandHasFlag(fields []string, names ...string) bool {
	for _, field := range fields {
		for _, name := range names {
			if strings.EqualFold(field, name) {
				return true
			}
		}
	}
	return false
}

func joinDriverCommand(fields []string) string {
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		if strings.ContainsAny(field, " \t\r\n\"") {
			out = append(out, `"`+strings.ReplaceAll(field, `"`, `\"`)+`"`)
		} else {
			out = append(out, field)
		}
	}
	return strings.Join(out, " ")
}

func SplitPublicCommand(command string) ([]string, error) {
	invocation, err := commands.ParsePublicInvocation(command)
	if err != nil {
		return nil, err
	}
	return invocation.CLIArgs()
}

func splitDriverCommand(command string) ([]string, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil, fmt.Errorf("driver request command is empty")
	}
	fields := []string{}
	var current strings.Builder
	inQuote := false
	inField := false
	for i := 0; i < len(command); i++ {
		ch := command[i]
		if inQuote {
			inField = true
			if ch == '\\' && i+1 < len(command) && command[i+1] == '"' {
				current.WriteByte('"')
				i++
				continue
			}
			if ch == '"' {
				inQuote = false
				continue
			}
			current.WriteByte(ch)
			continue
		}
		switch ch {
		case ' ', '\t', '\n', '\r':
			if inField {
				fields = append(fields, current.String())
				current.Reset()
				inField = false
			}
		case '"':
			inQuote = true
			inField = true
		default:
			inField = true
			current.WriteByte(ch)
		}
	}
	if inQuote {
		return nil, fmt.Errorf("unterminated quote in driver request command")
	}
	if inField {
		fields = append(fields, current.String())
	}
	return fields, nil
}
