package cli

import (
	"encoding/hex"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/autonomy"
	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
)

func runGateProfileMode(ctx runtime.Context, target string, opt Options, format string, out io.Writer) error {
	if opt.Gate.ProvisionProfile == opt.Gate.RevokeProfile {
		return fmt.Errorf("gate profile mode requires exactly one of -ProvisionProfile or -RevokeProfile")
	}
	if opt.WhatIf {
		return fmt.Errorf("gate profile mode uses read-only preview by default; omit -WhatIf")
	}
	if opt.Apply && strings.TrimSpace(opt.Gate.ExpectedProfilePlanSHA256) == "" {
		return fmt.Errorf("gate profile -Apply requires -ExpectedProfilePlanSha256 from preview")
	}
	if !opt.Apply && strings.TrimSpace(opt.Gate.ExpectedProfilePlanSHA256) != "" {
		return fmt.Errorf("gate -ExpectedProfilePlanSha256 is only valid with profile -Apply")
	}
	if strings.TrimSpace(opt.Gate.Lane) == "" {
		return fmt.Errorf("gate profile mode requires -Lane")
	}

	var (
		plan autonomy.ProfileMutationPlan
		err  error
	)
	if opt.Gate.ProvisionProfile {
		preset := strings.TrimSpace(opt.Gate.ProfilePreset)
		if preset != "" {
			if preset != autonomy.ManagedAutonomousPresetV1 {
				return fmt.Errorf("gate -ProfilePreset has unsupported value %q; supported: %s", preset, autonomy.ManagedAutonomousPresetV1)
			}
			if !opt.Gate.ProfileExplicitOptIn {
				return fmt.Errorf("gate -ProfilePreset=%s requires explicit -ProfileExplicitOptIn", preset)
			}
			if strings.TrimSpace(opt.Gate.ProfileID) != "" {
				return fmt.Errorf("gate -ProfilePreset=%s generates its managed profile identity; omit -ProfileId", preset)
			}
			plan, err = autonomy.PreviewManagedAutonomousPreset(autonomy.ManagedAutonomousPresetOptions{
				RepoRoot:            ctx.RepoRoot,
				CaseRoot:            target,
				Pack:                ctx.Pack,
				Lane:                opt.Gate.Lane,
				Preset:              preset,
				ExplicitOptIn:       true,
				Actions:             gateProfileList(opt.Gate.Action),
				Targets:             gateProfileList(opt.Gate.TargetRef),
				Budget:              autonomy.Budget{RuntimeSeconds: opt.Gate.RuntimeSeconds, DiskMB: opt.Gate.DiskMB, Requests: opt.Gate.Requests},
				StopConditions:      gateProfileList(opt.Gate.StopConditions),
				OutputPaths:         gateProfileList(opt.Gate.OutputPaths),
				GrantedBy:           opt.Gate.ProfileGrantedBy,
				GrantedAt:           opt.Gate.ProfileGrantedAt,
				ExpiresAt:           opt.Gate.ProfileExpiresAt,
				ExternalTargetScope: gateProfileList(opt.Gate.ProfileExternalTargetScope),
			})
		} else {
			if opt.Gate.ProfileExplicitOptIn {
				return fmt.Errorf("gate -ProfileExplicitOptIn requires -ProfilePreset=%s", autonomy.ManagedAutonomousPresetV1)
			}
			if strings.TrimSpace(opt.Gate.ProfileExternalTargetScope) != "" {
				return fmt.Errorf("gate -ProfileExternalTargetScope requires -ProfilePreset=%s", autonomy.ManagedAutonomousPresetV1)
			}
			profile, profileErr := gateProvisionProfile(ctx, opt)
			if profileErr != nil {
				return profileErr
			}
			plan, err = autonomy.PreviewProvision(autonomy.ProfileProvisionOptions{
				RepoRoot: ctx.RepoRoot,
				CaseRoot: target,
				Pack:     ctx.Pack,
				Lane:     opt.Gate.Lane,
				Profile:  profile,
			})
		}
	} else {
		if gateProfileProvisionFieldsPresent(opt) {
			return fmt.Errorf("gate -RevokeProfile accepts only lane and expected plan identity fields")
		}
		plan, err = autonomy.PreviewRevoke(autonomy.ProfileRevokeOptions{
			RepoRoot: ctx.RepoRoot,
			CaseRoot: target,
			Pack:     ctx.Pack,
			Lane:     opt.Gate.Lane,
		})
	}
	if err != nil {
		return err
	}
	if !opt.Apply {
		if format == "json" {
			return writeJSON(out, plan)
		}
		return writeGateProfilePlanText(out, plan)
	}
	result, err := autonomy.ApplyProfilePlan(plan, strings.ToLower(strings.TrimSpace(opt.Gate.ExpectedProfilePlanSHA256)))
	if err != nil {
		return err
	}
	if format == "json" {
		return writeJSON(out, result)
	}
	return writeGateProfileResultText(out, result)
}

func gateProvisionProfile(ctx runtime.Context, opt Options) (autonomy.Profile, error) {
	profileID := strings.TrimSpace(opt.Gate.ProfileID)
	grantedBy := strings.TrimSpace(opt.Gate.ProfileGrantedBy)
	grantedAt := strings.TrimSpace(opt.Gate.ProfileGrantedAt)
	expiresAt := strings.TrimSpace(opt.Gate.ProfileExpiresAt)
	action := strings.ToLower(strings.TrimSpace(opt.Gate.Action))
	targetRef := strings.TrimSpace(opt.Gate.TargetRef)
	if profileID == "" || grantedBy == "" || grantedAt == "" || expiresAt == "" {
		return autonomy.Profile{}, fmt.Errorf("gate -ProvisionProfile requires -ProfileId, -ProfileGrantedBy, -ProfileGrantedAt, and -ProfileExpiresAt")
	}
	if action == "" || targetRef == "" {
		return autonomy.Profile{}, fmt.Errorf("gate -ProvisionProfile requires exact -Action and -TargetRef")
	}
	stopConditions := gateProfileList(opt.Gate.StopConditions)
	outputPaths := gateProfileList(opt.Gate.OutputPaths)
	if len(stopConditions) == 0 || len(outputPaths) == 0 {
		return autonomy.Profile{}, fmt.Errorf("gate -ProvisionProfile requires bounded -StopConditions and -OutputPaths")
	}
	m, err := manifest.Load(ctx.RepoRoot, ctx.Pack)
	if err != nil {
		return autonomy.Profile{}, err
	}
	if ctx.Pack != defaults.DefaultPack || action != "inspect" {
		return autonomy.Profile{}, fmt.Errorf(
			"gate -ProvisionProfile currently permits only pack=%s action=inspect for the fixed IDA index adapter",
			defaults.DefaultPack,
		)
	}
	if err := validateVMPIDAProfileTarget(targetRef); err != nil {
		return autonomy.Profile{}, err
	}
	expectedStopConditions := []string{
		"scope-drift",
		"source-drift",
		"output-exceeds-bounded-evidence-packet",
	}
	if !reflect.DeepEqual(stopConditions, expectedStopConditions) {
		return autonomy.Profile{}, fmt.Errorf(
			"gate -ProvisionProfile requires the fixed VMP IDA stop conditions",
		)
	}
	if len(outputPaths) != 1 {
		return autonomy.Profile{}, fmt.Errorf(
			"gate -ProvisionProfile requires one lane-owned VMP IDA output path",
		)
	}
	board, err := mission.ReadBoard(ctx.Target)
	if err != nil {
		return autonomy.Profile{}, fmt.Errorf("read VMP IDA profile lane workspace: %w", err)
	}
	lane, ok := mission.LookupBoardLane(board.Lanes, opt.Gate.Lane, false)
	if !ok || !validVMPIDAProfileOutputPath(lane.Workspace, outputPaths[0]) {
		return autonomy.Profile{}, fmt.Errorf(
			"gate -ProvisionProfile requires one output path under the selected lane workspace for VMP IDA evidence",
		)
	}
	if opt.Gate.RuntimeSeconds < 1 || opt.Gate.RuntimeSeconds > 900 ||
		opt.Gate.DiskMB < 1 || opt.Gate.DiskMB > 4 ||
		opt.Gate.Requests != 1 {
		return autonomy.Profile{}, fmt.Errorf(
			"gate -ProvisionProfile requires VMP IDA budget runtimeSeconds=1..900 diskMB=1..4 requests=1",
		)
	}
	denied := make([]string, 0, len(m.HeavyToolGateIDs()))
	for _, candidate := range m.HeavyToolGateIDs() {
		if candidate != action {
			denied = append(denied, candidate)
		}
	}
	sort.Strings(denied)
	return autonomy.Profile{
		SchemaVersion:  1,
		ProfileID:      profileID,
		Lane:           strings.TrimSpace(opt.Gate.Lane),
		Mode:           autonomy.ModePreauthorized,
		AllowedActions: []string{action},
		DeniedActions:  denied,
		TargetScope:    []autonomy.Target{{Match: "exact", Value: targetRef}},
		Budget: autonomy.Budget{
			RuntimeSeconds: opt.Gate.RuntimeSeconds,
			DiskMB:         opt.Gate.DiskMB,
			Requests:       opt.Gate.Requests,
		},
		StopConditions: stopConditions,
		OutputPaths:    outputPaths,
		RecordRequired: true,
		NotifyMainOn:   []string{"boundary-hit", "new-risk", "destructive-change", "authority-write-needed"},
		GrantedBy:      grantedBy,
		GrantedAt:      grantedAt,
		ExpiresAt:      expiresAt,
	}, nil
}

func validateVMPIDAProfileTarget(targetRef string) error {
	const prefix = "tooling/ida-agent-bridge/requests/"
	targetRef = filepath.ToSlash(filepath.Clean(filepath.FromSlash(strings.TrimSpace(targetRef))))
	if !strings.HasPrefix(targetRef, prefix) ||
		filepath.Ext(targetRef) != ".json" ||
		strings.Contains(strings.TrimPrefix(targetRef, prefix), "/") {
		return fmt.Errorf(
			"gate -ProvisionProfile requires a content-addressed VMP IDA request target",
		)
	}
	identity := strings.TrimSuffix(
		strings.TrimPrefix(targetRef, prefix),
		".json",
	)
	decoded, err := hex.DecodeString(identity)
	if err != nil || len(decoded) != 32 || identity != strings.ToLower(identity) {
		return fmt.Errorf(
			"gate -ProvisionProfile requires a content-addressed VMP IDA request target",
		)
	}
	return nil
}

func validVMPIDAProfileOutputPath(workspace, outputPath string) bool {
	workspace = filepath.ToSlash(filepath.Clean(filepath.FromSlash(strings.TrimSpace(workspace))))
	outputPath = filepath.ToSlash(filepath.Clean(filepath.FromSlash(strings.TrimSpace(outputPath))))
	if workspace == "." || workspace == ".." || strings.HasPrefix(workspace, "../") ||
		outputPath == "." || outputPath == ".." || strings.HasPrefix(outputPath, "../") ||
		outputPath == workspace || !strings.HasPrefix(outputPath, workspace+"/") {
		return false
	}
	for component := range strings.SplitSeq(outputPath, "/") {
		if strings.EqualFold(component, "ida-index") {
			return true
		}
	}
	return false
}

func gateProfileProvisionFieldsPresent(opt Options) bool {
	return strings.TrimSpace(opt.Gate.ProfilePreset) != "" || opt.Gate.ProfileExplicitOptIn ||
		strings.TrimSpace(opt.Gate.ProfileExternalTargetScope) != "" ||
		strings.TrimSpace(opt.Gate.ProfileID) != "" ||
		strings.TrimSpace(opt.Gate.ProfileGrantedBy) != "" ||
		strings.TrimSpace(opt.Gate.ProfileGrantedAt) != "" ||
		strings.TrimSpace(opt.Gate.ProfileExpiresAt) != "" ||
		strings.TrimSpace(opt.Gate.Action) != "" ||
		strings.TrimSpace(opt.Gate.TargetRef) != "" ||
		opt.Gate.RuntimeSeconds != 0 || opt.Gate.DiskMB != 0 || opt.Gate.Requests != 0 ||
		strings.TrimSpace(opt.Gate.StopConditions) != "" || strings.TrimSpace(opt.Gate.OutputPaths) != ""
}

func gateProfileOnlyFieldsPresent(opt Options) bool {
	return strings.TrimSpace(opt.Gate.ProfilePreset) != "" || opt.Gate.ProfileExplicitOptIn ||
		strings.TrimSpace(opt.Gate.ProfileExternalTargetScope) != "" ||
		strings.TrimSpace(opt.Gate.ProfileID) != "" ||
		strings.TrimSpace(opt.Gate.ProfileGrantedBy) != "" ||
		strings.TrimSpace(opt.Gate.ProfileGrantedAt) != "" ||
		strings.TrimSpace(opt.Gate.ProfileExpiresAt) != "" ||
		strings.TrimSpace(opt.Gate.ExpectedProfilePlanSHA256) != ""
}

func gateNonProfileModeSelected(opt Options) bool {
	return strings.TrimSpace(opt.Gate.Subject) != "" ||
		strings.TrimSpace(opt.Gate.Summary) != "" ||
		strings.TrimSpace(opt.Gate.Actor) != "" ||
		strings.TrimSpace(opt.Gate.Risk) != "" ||
		strings.TrimSpace(opt.Gate.BatchID) != "" ||
		strings.TrimSpace(opt.Gate.Scope) != "" ||
		strings.TrimSpace(opt.Gate.Budget) != "" ||
		strings.TrimSpace(opt.Gate.TriedLightSteps) != "" ||
		strings.TrimSpace(opt.Gate.GateEventID) != "" ||
		strings.TrimSpace(opt.Gate.ExecutionStatus) != "" ||
		opt.Gate.ActualRuntimeSeconds != 0 || opt.Gate.ActualDiskMB != 0 || opt.Gate.ActualRequests != 0 ||
		strings.TrimSpace(opt.Gate.OutputRefs) != "" ||
		strings.TrimSpace(opt.Gate.EvidenceRefs) != "" ||
		strings.TrimSpace(opt.Gate.BoundaryHits) != "" ||
		strings.TrimSpace(opt.Gate.Escalation) != "" ||
		strings.TrimSpace(opt.Gate.ExecutionReportPath) != "" ||
		opt.Gate.ExecutionReportContract || opt.Gate.ValidateExecutionReport ||
		opt.Gate.ScaffoldExecutionReport || opt.Gate.DraftExecutionReport ||
		opt.Gate.RecordAdapterExecutionDispatch || opt.Gate.RecordAdapterExecutionReceipt ||
		strings.TrimSpace(opt.Gate.ExpectedExecutionReportSHA256) != "" ||
		strings.TrimSpace(opt.Gate.ExpectedAdapterExecutionDispatchBindingSHA256) != "" ||
		strings.TrimSpace(opt.Gate.ExpectedAdapterExecutionBindingSHA256) != "" ||
		strings.TrimSpace(opt.Gate.ExpectedAdapterExecutionReceiptSHA256) != "" ||
		strings.TrimSpace(opt.Gate.AdapterExecutionReceiptPath) != "" ||
		strings.TrimSpace(opt.Gate.AdapterID) != "" ||
		strings.TrimSpace(opt.Gate.Executor) != "" ||
		opt.Gate.ExpectedExecutorGeneration != 0 ||
		strings.TrimSpace(opt.Gate.AdapterHarness) != "" ||
		strings.TrimSpace(opt.Gate.AdapterSession) != "" ||
		strings.TrimSpace(opt.Gate.ExecutionExitStatus) != "" ||
		opt.Gate.EmitDriverReceipt
}

func gateProfileList(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r'
	})
	out := make([]string, 0, len(parts))
	for _, item := range parts {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

func writeGateProfilePlanText(out io.Writer, plan autonomy.ProfileMutationPlan) error {
	_, err := fmt.Fprintf(out, "gate profile：operation=%s mutation=%t replay=%t reviewRequired=%t lane=%s profilePath=%s expectedPlanSha256=%s\n", plan.Operation, plan.IsMutation, plan.Replay, plan.ReviewRequired, plan.Lane, plan.ProfilePath, plan.ExpectedPlanSHA256)
	return err
}

func writeGateProfileResultText(out io.Writer, result autonomy.ProfileMutationResult) error {
	_, err := fmt.Fprintf(out, "gate profile：operation=%s applied=%t alreadyApplied=%t lane=%s profilePath=%s profileSha256=%s\n", result.Plan.Operation, result.Applied, result.AlreadyApplied, result.Plan.Lane, result.Plan.ProfilePath, result.ProfileSHA256)
	return err
}
