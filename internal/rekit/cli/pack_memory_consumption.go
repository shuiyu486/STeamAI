package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/packmemoryconsumption"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
)

var (
	packMemoryConsumptionDiscover = packmemoryconsumption.Discover
	packMemoryConsumptionPreview  = packmemoryconsumption.Preview
	packMemoryConsumptionApply    = packmemoryconsumption.Apply
	packMemoryConsumerUseVerify   = packmemoryconsumption.VerifyConsumerUse
)

type packMemoryConsumptionStatus struct {
	Ready                       bool                                     `json:"ready"`
	Summary                     string                                   `json:"summary"`
	Discovery                   packmemoryconsumption.Discovery          `json:"discovery"`
	MissionCommanderNextActions []mission.MissionCommanderNextActionItem `json:"missionCommanderNextActions,omitempty"`
	MissionCommanderActionQueue mission.MissionCommanderActionQueue      `json:"missionCommanderActionQueue"`
	Warnings                    []string                                 `json:"warnings,omitempty"`
}

func buildPackMemoryConsumptionStatus(repoRoot, caseRoot, pack string) *packMemoryConsumptionStatus {
	discovery, err := packMemoryConsumptionDiscover(repoRoot, caseRoot, pack)
	if err != nil {
		return &packMemoryConsumptionStatus{
			Ready:    false,
			Summary:  "pack-memory consumption discovery unavailable",
			Warnings: []string{err.Error()},
		}
	}
	actions := make([]mission.MissionCommanderNextActionItem, 0, len(discovery.Available))
	for _, change := range discovery.Available {
		if strings.TrimSpace(change.PreviewCommand) == "" {
			continue
		}
		actions = append(actions, mission.MissionCommanderNextActionItem{
			State:          "pack-memory-change-available",
			Command:        change.PreviewCommand,
			Source:         "completed-pack-memory-catalog",
			ActionID:       "consume-pack-memory-change-" + change.ChangeID,
			RequiresReview: true,
			Reasons:        []string{"completed verified pack-memory change is available to the current attached case", "selected sync requires review and exact WhatIf plan hash"},
			Boundary:       append([]string{}, discovery.Boundary...),
		})
	}
	summary := fmt.Sprintf("completed pack-memory changes available=%d consumed=%d conflicts=%d", len(discovery.Available), len(discovery.Consumed), len(discovery.Conflicts))
	return &packMemoryConsumptionStatus{Ready: len(discovery.Conflicts) == 0, Summary: summary, Discovery: discovery, MissionCommanderNextActions: actions, MissionCommanderActionQueue: mission.MissionCommanderActionQueueFor(actions)}
}

func runPackMemoryConsumerUseVerification(ctx runtime.Context, opt Options, out io.Writer) error {
	if !strings.EqualFold(strings.TrimSpace(opt.Command), "sync") {
		return fmt.Errorf("pack-memory consumer-use verification is available only through sync")
	}
	if strings.TrimSpace(opt.SelectPackMemoryChange) == "" || strings.TrimSpace(opt.MemberExecutionAttemptID) == "" || strings.TrimSpace(opt.MemberOutputStagingLane) == "" || strings.TrimSpace(opt.PackMemoryConsumerOutputPath) == "" {
		return fmt.Errorf("sync -VerifyPackMemoryConsumerUse requires -SelectPackMemoryChange, -Lane, -MemberExecutionAttemptId, and -PackMemoryConsumerOutputPath")
	}
	if opt.Apply || opt.WhatIf || opt.Review || opt.Force || opt.CreateCandidates || strings.TrimSpace(opt.ExpectedPackMemoryConsumptionPlanSHA256) != "" || wantsReviewArtifacts(opt) {
		return fmt.Errorf("pack-memory consumer-use verification is read-only and cannot be combined with mutation, preview, review, or candidate options")
	}
	if format, err := workstreamFormat(opt.Format); err != nil || format != "json" {
		return fmt.Errorf("pack-memory consumer-use verification requires -Format json")
	}
	target, err := commandTarget(ctx, "sync", "attached case")
	if err != nil {
		return err
	}
	proof, err := packMemoryConsumerUseVerify(ctx.RepoRoot, target, ctx.Pack, packmemoryconsumption.ConsumerUseOptions{ChangeID: opt.SelectPackMemoryChange, Lane: opt.MemberOutputStagingLane, AttemptID: opt.MemberExecutionAttemptID, OutputPath: opt.PackMemoryConsumerOutputPath})
	if err != nil {
		return err
	}
	return writeJSON(out, proof)
}

func runPackMemorySelectedSync(ctx runtime.Context, opt Options, out io.Writer) error {
	if !strings.EqualFold(strings.TrimSpace(opt.Command), "sync") {
		return fmt.Errorf("pack-memory selected sync is available only through sync")
	}
	if opt.WhatIf == opt.Apply {
		return fmt.Errorf("pack-memory selected sync requires exactly one of -WhatIf or -Apply")
	}
	if opt.Review || opt.Force || opt.CreateCandidates || wantsReviewArtifacts(opt) || strings.TrimSpace(opt.MemberOutputStagingLane) != "" || strings.TrimSpace(opt.MemberExecutionAttemptID) != "" || opt.StageMemberOutput || strings.TrimSpace(opt.MemberOutputPath) != "" || strings.TrimSpace(opt.ManagedTargetPath) != "" || strings.TrimSpace(opt.ExpectedMemberOutputStagingPlanSHA256) != "" {
		return fmt.Errorf("pack-memory selected sync cannot be combined with ordinary sync review, Force, candidate, lane, member execution, or staging options")
	}
	if opt.WhatIf && strings.TrimSpace(opt.ExpectedPackMemoryConsumptionPlanSHA256) != "" {
		return fmt.Errorf("pack-memory selected sync -WhatIf does not accept -ExpectedPackMemoryConsumptionPlanSha256")
	}
	if opt.Apply && strings.TrimSpace(opt.ExpectedPackMemoryConsumptionPlanSHA256) == "" {
		return fmt.Errorf("pack-memory selected sync -Apply requires -ExpectedPackMemoryConsumptionPlanSha256 from WhatIf")
	}
	if format, err := workstreamFormat(opt.Format); err != nil || format != "json" {
		return fmt.Errorf("pack-memory selected sync requires -Format json")
	}
	target, err := commandTarget(ctx, "sync", "attached case")
	if err != nil {
		return err
	}
	if opt.WhatIf {
		plan, err := packMemoryConsumptionPreview(ctx.RepoRoot, target, ctx.Pack, opt.SelectPackMemoryChange)
		if err != nil {
			return err
		}
		return writeJSON(out, plan)
	}
	result, err := packMemoryConsumptionApply(ctx.RepoRoot, target, ctx.Pack, opt.SelectPackMemoryChange, opt.ExpectedPackMemoryConsumptionPlanSHA256)
	if err != nil {
		return err
	}
	return writeJSON(out, result)
}
