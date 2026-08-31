package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

// MissionSnapshotOptions selects one typed, read-only Mission Control view.
// A held project lease is validated and reused; without one the status builder
// retains its existing shared-lease behavior for current .steamai projects.
type MissionSnapshotOptions struct {
	CaseRoot              string
	Pack                  string
	SelectedCurrentLane   string
	ProjectExecutionLease *projectexecution.Lease
}

// MissionControlSnapshot is the narrow typed view consumed by in-process
// session hosts. It deliberately omits the full status presentation inventory.
type MissionControlSnapshot struct {
	CaseRoot        string
	Pack            string
	Mode            string
	MissionControl  *MissionControlRunbookSnapshot
	CaseMission     *MissionCaseSnapshot
	MemberExecution *MissionMemberExecutionSnapshot
}

type MissionControlRunbookSnapshot struct {
	Scope                      string
	CurrentDriverRequest       *mission.MissionCommanderDriverRequest
	CurrentDriverRequestSHA256 string
}

type MissionCaseSnapshot struct {
	Summary                           string
	MissionCommanderActionQueue       mission.MissionCommanderActionQueue
	ReviewerDispatchIntakeActionQueue mission.MissionCommanderActionQueue
	MissionCompletion                 *MissionCompletionSnapshot
}

type MissionCompletionSnapshot struct {
	Ready                 bool
	State                 string
	OperationallyComplete bool
}

type MissionMemberExecutionSnapshot struct {
	State                  string
	Lane                   string
	ReviewerPlanCommand    string
	ReviewerPlanInvocation *commands.PublicInvocation
}

// ReadMissionSnapshot reads Mission Control directly from the typed status
// inventory. It does not invoke a command handler or serialize through JSON.
func ReadMissionSnapshot(opt MissionSnapshotOptions) (MissionControlSnapshot, error) {
	caseRoot := strings.TrimSpace(opt.CaseRoot)
	if caseRoot == "" {
		return MissionControlSnapshot{}, fmt.Errorf("mission snapshot requires an exact case root")
	}
	ctx, statusOpt, err := missionControlContext(caseRoot, opt.Pack)
	if err != nil {
		return MissionControlSnapshot{}, err
	}
	statusOpt.SelectedCurrentLane = strings.TrimSpace(opt.SelectedCurrentLane)
	status, err := buildInvocationStatusInventoryWithLease(
		ctx,
		statusOpt,
		opt.ProjectExecutionLease,
	)
	if err != nil {
		return MissionControlSnapshot{}, err
	}
	return missionControlSnapshotFromStatus(status), nil
}

// DriverStepPreview is an opaque, exact WhatIf result. ApplyDriverStep accepts
// only this value so callers cannot reconstruct a mutation request from prose
// or from a partial public DTO.
type DriverStepPreview struct {
	CaseRoot                        string
	Pack                            string
	CurrentDriverRequest            mission.MissionCommanderDriverRequest
	PreviewResult                   any
	ApplyDriverRequest              mission.MissionCommanderDriverRequest
	ExpectedDriverStepPlanSHA256    string
	ExpectedDriverStepPreviewSHA256 string
	MissionCommanderActionQueue     mission.MissionCommanderActionQueue

	plan         driverStepPlan
	ctx          runtime.Context
	exactRequest bool
	identitySHA  string
}

type DriverStepPreviewOptions struct {
	CaseRoot              string
	Pack                  string
	SelectedCurrentLane   string
	ExactDriverRequest    *mission.MissionCommanderDriverRequest
	ProjectExecutionLease *projectexecution.Lease
}

type DriverStepApplyOptions struct {
	ProjectExecutionLease   *projectexecution.Lease
	ExecutionControlBinding *executioncontrol.Binding
}

type DriverStepApplyResult struct {
	Command         string
	Applied         bool
	Committed       bool
	ReceiptCommand  string
	Reconcile       *DriverStepReconcileResult
	Completion      *workstream.CompleteResult
	RefreshedStatus MissionControlSnapshot
}

type DriverStepReconcileResult struct {
	Actor              string
	Executor           string
	PreviousExecutor   string
	ExecutorGeneration int
}

// PreviewDriverStep builds the exact bounded driver WhatIf plan in-process.
func PreviewDriverStep(opt DriverStepPreviewOptions) (DriverStepPreview, error) {
	caseRoot := strings.TrimSpace(opt.CaseRoot)
	if caseRoot == "" {
		return DriverStepPreview{}, fmt.Errorf("driver step preview requires an exact case root")
	}
	ctx, driverOpt, err := missionControlContext(caseRoot, opt.Pack)
	if err != nil {
		return DriverStepPreview{}, err
	}
	driverOpt.Command = commands.RunDriverStep
	driverOpt.SelectedCurrentLane = strings.TrimSpace(opt.SelectedCurrentLane)
	var plan driverStepPlan
	if opt.ExactDriverRequest == nil {
		plan, err = buildDriverStepPlanWithLease(ctx, driverOpt, opt.ProjectExecutionLease)
	} else {
		if err := mission.ValidateMissionCommanderDriverRequest(*opt.ExactDriverRequest); err != nil {
			return DriverStepPreview{}, fmt.Errorf("exact driver request is invalid: %w", err)
		}
		if lane := strings.TrimSpace(opt.ExactDriverRequest.Lane); lane == "" || lane != driverOpt.SelectedCurrentLane {
			return DriverStepPreview{}, fmt.Errorf("exact driver request does not match the selected lane")
		}
		plan, err = buildDriverStepPlanFromRequest(ctx, *opt.ExactDriverRequest)
	}
	if err != nil {
		return DriverStepPreview{}, err
	}
	if plan.IsMutation || plan.Applied || strings.TrimSpace(plan.ExpectedDriverStepPlanSHA256) == "" {
		return DriverStepPreview{}, fmt.Errorf("driver step preview omitted the zero-write hash-bound plan")
	}
	preview := DriverStepPreview{
		CaseRoot:                        plan.CaseRoot,
		Pack:                            plan.Pack,
		CurrentDriverRequest:            plan.CurrentDriverRequest,
		PreviewResult:                   plan.PreviewResult,
		ApplyDriverRequest:              plan.ApplyDriverRequest,
		ExpectedDriverStepPlanSHA256:    plan.ExpectedDriverStepPlanSHA256,
		ExpectedDriverStepPreviewSHA256: plan.ExpectedDriverStepPreviewSHA256,
		MissionCommanderActionQueue:     plan.MissionCommanderActionQueue,
		plan:                            plan,
		ctx:                             ctx,
		exactRequest:                    opt.ExactDriverRequest != nil,
	}
	preview.identitySHA, err = driverStepPreviewIdentitySHA256(preview)
	if err != nil {
		return DriverStepPreview{}, err
	}
	return preview, nil
}

// ApplyDriverStep revalidates the current typed plan and then applies exactly
// the reviewed preview. It returns typed mutation and refreshed-status results.
func SetDriverStepStatusRefreshHookForTest(hook func(string) error) func() {
	previous := currentStepBeforeStatusRefreshHook
	currentStepBeforeStatusRefreshHook = hook
	return func() { currentStepBeforeStatusRefreshHook = previous }
}

func ApplyDriverStep(preview DriverStepPreview, opt DriverStepApplyOptions) (DriverStepApplyResult, error) {
	if strings.TrimSpace(preview.ExpectedDriverStepPlanSHA256) == "" || preview.ctx.Target == "" || strings.TrimSpace(preview.identitySHA) == "" {
		return DriverStepApplyResult{}, fmt.Errorf("driver step Apply requires an exact typed preview")
	}
	identitySHA, err := driverStepPreviewIdentitySHA256(preview)
	if err != nil {
		return DriverStepApplyResult{}, err
	}
	if identitySHA != preview.identitySHA {
		return DriverStepApplyResult{}, fmt.Errorf("driver step preview identity was modified after review")
	}
	if preview.ExpectedDriverStepPlanSHA256 != preview.plan.ExpectedDriverStepPlanSHA256 ||
		preview.ExpectedDriverStepPreviewSHA256 != preview.plan.ExpectedDriverStepPreviewSHA256 {
		return DriverStepApplyResult{}, fmt.Errorf("driver step preview identity was modified after review")
	}
	ctx := preview.ctx
	if opt.ProjectExecutionLease != nil {
		if err := opt.ProjectExecutionLease.ValidateFor(ctx.Target); err != nil {
			return DriverStepApplyResult{}, err
		}
	}
	driverOpt := Options{
		Command:                            commands.RunDriverStep,
		Target:                             ctx.Target,
		Pack:                               ctx.Pack,
		SelectedCurrentLane:                strings.TrimSpace(preview.plan.CurrentDriverRequest.Lane),
		currentLoopExecutionControlBinding: opt.ExecutionControlBinding,
	}
	if _, ok := preview.plan.PreviewResult.(workstream.StartResult); ok {
		driverOpt.SelectedCurrentLane = ""
	}
	var fresh driverStepPlan
	if preview.exactRequest {
		fresh, err = buildDriverStepPlanFromRequest(ctx, preview.plan.CurrentDriverRequest)
	} else {
		fresh, err = buildDriverStepPlanWithLease(ctx, driverOpt, opt.ProjectExecutionLease)
	}
	if err != nil {
		return DriverStepApplyResult{}, err
	}
	if fresh.ExpectedDriverStepPlanSHA256 != preview.plan.ExpectedDriverStepPlanSHA256 {
		return DriverStepApplyResult{}, fmt.Errorf("driver step preview is stale; current typed plan differs")
	}
	applied, err := applyDriverStepPlanWithLease(
		ctx,
		driverOpt,
		fresh,
		opt.ProjectExecutionLease,
	)
	if err != nil {
		out := driverStepApplyResultFromPlan(applied)
		return out, err
	}
	if opt.ProjectExecutionLease != nil {
		if err := opt.ProjectExecutionLease.ValidateFor(ctx.Target); err != nil {
			return DriverStepApplyResult{}, err
		}
	}
	return driverStepApplyResultFromPlan(applied), nil
}

func driverStepApplyResultFromPlan(applied driverStepPlan) DriverStepApplyResult {
	out := DriverStepApplyResult{
		Command:   appliedDriverStepCommand(applied),
		Applied:   applied.Applied,
		Committed: applied.Applied,
	}
	if applied.Receipt != nil {
		out.ReceiptCommand = applied.Receipt.CommandResultCommand
	}
	if value, ok := applied.PreviewResult.(workstream.ReconcileResult); ok {
		out.Reconcile = &DriverStepReconcileResult{
			Actor:              value.Actor,
			Executor:           value.Executor,
			PreviousExecutor:   value.PreviousExecutor,
			ExecutorGeneration: value.ExecutorGeneration,
		}
	}
	if value, ok := applied.PreviewResult.(workstream.CompleteResult); ok {
		copy := value
		out.Completion = &copy
	}
	if applied.RefreshedStatus != nil {
		out.RefreshedStatus = missionControlSnapshotFromStatus(*applied.RefreshedStatus)
	}
	return out
}

func driverStepPreviewIdentitySHA256(preview DriverStepPreview) (string, error) {
	identity := struct {
		CaseRoot                        string                                `json:"caseRoot"`
		Pack                            string                                `json:"pack"`
		CurrentDriverRequest            mission.MissionCommanderDriverRequest `json:"currentDriverRequest"`
		PreviewResult                   any                                   `json:"previewResult"`
		ApplyDriverRequest              mission.MissionCommanderDriverRequest `json:"applyDriverRequest"`
		ExpectedDriverStepPlanSHA256    string                                `json:"expectedDriverStepPlanSha256"`
		ExpectedDriverStepPreviewSHA256 string                                `json:"expectedDriverStepPreviewSha256"`
		MissionCommanderActionQueue     mission.MissionCommanderActionQueue   `json:"missionCommanderActionQueue"`
		ExactRequest                    bool                                  `json:"exactRequest"`
	}{
		CaseRoot:                        preview.CaseRoot,
		Pack:                            preview.Pack,
		CurrentDriverRequest:            preview.CurrentDriverRequest,
		PreviewResult:                   preview.PreviewResult,
		ApplyDriverRequest:              preview.ApplyDriverRequest,
		ExpectedDriverStepPlanSHA256:    preview.ExpectedDriverStepPlanSHA256,
		ExpectedDriverStepPreviewSHA256: preview.ExpectedDriverStepPreviewSHA256,
		MissionCommanderActionQueue:     preview.MissionCommanderActionQueue,
		ExactRequest:                    preview.exactRequest,
	}
	encoded, err := json.Marshal(identity)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func buildDriverStepPlanWithLease(ctx runtime.Context, opt Options, lease *projectexecution.Lease) (driverStepPlan, error) {
	status, err := buildInvocationStatusInventoryWithLease(ctx, opt, lease)
	if err != nil {
		return driverStepPlan{}, err
	}
	return buildDriverStepPlanFromStatus(ctx, status)
}

func appliedDriverStepCommand(plan driverStepPlan) string {
	return driverStepResultCommand(plan.PreviewResult)
}

func missionControlContext(caseRoot, pack string) (runtime.Context, Options, error) {
	pack = strings.TrimSpace(pack)
	ctx, err := runtime.NewWithCwd(caseRoot, pack, "")
	if err != nil {
		return runtime.Context{}, Options{}, err
	}
	opt := Options{
		Command:      commands.Status,
		Target:       ctx.Target,
		Pack:         ctx.Pack,
		PackProvided: pack != "",
	}
	if !opt.PackProvided && instance.LooksLikeCase(ctx.Target) {
		inst, err := instance.Read(ctx.Target)
		if err != nil {
			return runtime.Context{}, Options{}, err
		}
		if strings.TrimSpace(inst.TemplatePack) != "" {
			ctx.Pack = strings.TrimSpace(inst.TemplatePack)
			opt.Pack = ctx.Pack
		}
	}
	return ctx, opt, nil
}

func missionControlSnapshotFromStatus(status statusInventory) MissionControlSnapshot {
	out := MissionControlSnapshot{
		CaseRoot: status.Target,
		Pack:     status.Pack,
		Mode:     status.Mode,
	}
	if runbook := status.MissionControlRunbook; runbook != nil {
		out.MissionControl = &MissionControlRunbookSnapshot{
			Scope:                      runbook.Scope,
			CurrentDriverRequest:       runbook.CurrentDriverRequest,
			CurrentDriverRequestSHA256: runbook.CurrentDriverRequestSHA256,
		}
	}
	if caseMission := status.CaseMission; caseMission != nil {
		out.CaseMission = &MissionCaseSnapshot{
			Summary:                           caseMission.Summary,
			MissionCommanderActionQueue:       caseMission.MissionCommanderActionQueue,
			ReviewerDispatchIntakeActionQueue: caseMission.ReviewerDispatchIntakeActionQueue,
		}
		if completion := caseMission.MissionCompletion; completion != nil {
			out.CaseMission.MissionCompletion = &MissionCompletionSnapshot{
				Ready:                 completion.Ready,
				State:                 completion.State,
				OperationallyComplete: completion.OperationallyComplete,
			}
		}
	}
	if member := status.MemberExecution; member != nil {
		out.MemberExecution = &MissionMemberExecutionSnapshot{
			State:                  member.State,
			Lane:                   member.Lane,
			ReviewerPlanCommand:    member.ReviewerPlanCommand,
			ReviewerPlanInvocation: member.ReviewerPlanInvocation,
		}
	}
	return out
}
