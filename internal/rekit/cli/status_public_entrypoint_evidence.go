package cli

import (
	"fmt"

	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

func projectExecutionEvidenceReviewItemPublicEntrypoint(item *workstream.ExecutionEvidenceReviewItem, entrypoint string) error {
	if item == nil {
		return nil
	}
	if err := projectPublicCommandFields(entrypoint,
		projectPublicCommandField{"reviewCommand", &item.ReviewCommand},
		projectPublicCommandField{"handoffCommand", &item.HandoffCommand},
	); err != nil {
		return err
	}
	if err := projectExecutionEvidenceAcknowledgementPublicEntrypoint(item.Acknowledgement, entrypoint); err != nil {
		return fmt.Errorf("acknowledgement: %w", err)
	}
	if err := projectMissionCommanderActionPublicEntrypoint(&item.MissionCommanderAction, entrypoint); err != nil {
		return fmt.Errorf("missionCommanderAction: %w", err)
	}
	if err := projectExecutionEvidenceFollowThroughPublicEntrypoint(&item.FollowThrough, entrypoint); err != nil {
		return fmt.Errorf("followThrough: %w", err)
	}
	return nil
}

func projectExecutionEvidenceAcknowledgementPublicEntrypoint(ack *mission.ExecutionEvidenceReviewAcknowledgement, entrypoint string) error {
	if ack == nil {
		return nil
	}
	return projectPublicCommandFields(entrypoint,
		projectPublicCommandField{"acknowledgementReviewCommand", &ack.AcknowledgementReviewCommand},
		projectPublicCommandField{"acceptedPreviewCommand", &ack.AcceptedPreviewCommand},
		projectPublicCommandField{"rejectedPreviewCommand", &ack.RejectedPreviewCommand},
		projectPublicCommandField{"recordCommand", &ack.RecordCommand},
	)
}

func projectExecutionEvidenceFollowThroughPublicEntrypoint(follow *mission.ExecutionEvidenceFollowThrough, entrypoint string) error {
	if follow == nil {
		return nil
	}
	for index := range follow.Outcomes {
		outcome := &follow.Outcomes[index]
		if err := projectPublicCommandFields(entrypoint,
			projectPublicCommandField{fmt.Sprintf("outcomes[%d].command", index), &outcome.Command},
		); err != nil {
			return err
		}
		for commandIndex := range outcome.VerificationCommands {
			if err := projectPublicCommandFields(entrypoint,
				projectPublicCommandField{
					fmt.Sprintf("outcomes[%d].verificationCommands[%d]", index, commandIndex),
					&outcome.VerificationCommands[commandIndex],
				},
			); err != nil {
				return err
			}
		}
	}
	if err := projectMissionCommanderActionQueuePublicEntrypoint(&follow.ActionQueue, entrypoint); err != nil {
		return fmt.Errorf("actionQueue: %w", err)
	}
	return nil
}
