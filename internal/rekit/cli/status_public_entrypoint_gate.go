package cli

import (
	"fmt"

	"github.com/shuiyu486/re-context-kits/internal/rekit/currentloop"
)

type projectPublicCommandField struct {
	label string
	value *string
}

func projectPublicCommandFields(entrypoint string, fields ...projectPublicCommandField) error {
	for _, field := range fields {
		if field.value == nil {
			continue
		}
		projected, err := projectPublicCommandForEntrypoint(*field.value, entrypoint)
		if err != nil {
			return fmt.Errorf("%s: %w", field.label, err)
		}
		*field.value = projected
	}
	return nil
}

func projectPendingGateHandoffPublicEntrypoint(handoff *statusPendingGateHandoff, entrypoint string) error {
	if handoff == nil {
		return nil
	}
	return projectPublicCommandFields(entrypoint,
		projectPublicCommandField{"reviewCommand", &handoff.ReviewCommand},
		projectPublicCommandField{"whatIfCommand", &handoff.WhatIfCommand},
		projectPublicCommandField{"applyCommand", &handoff.ApplyCommand},
	)
}

func projectAuthorizedGateHandoffPublicEntrypoint(handoff *statusAuthorizedGateHandoff, entrypoint string) error {
	if handoff == nil {
		return nil
	}
	if err := projectPublicCommandFields(entrypoint,
		projectPublicCommandField{"reportContract", &handoff.ReportContract},
		projectPublicCommandField{"handoffCommand", &handoff.HandoffCommand},
	); err != nil {
		return err
	}
	if handoff.ReportSummary != nil {
		if err := projectPublicCommandFields(entrypoint,
			projectPublicCommandField{"reportSummary.currentAction", &handoff.ReportSummary.CurrentAction},
		); err != nil {
			return err
		}
	}
	for index := range handoff.LiveValidationRepairHints {
		if err := projectPublicCommandFields(entrypoint,
			projectPublicCommandField{
				fmt.Sprintf("liveValidationRepairHints[%d].repairAction", index),
				&handoff.LiveValidationRepairHints[index].RepairAction,
			},
		); err != nil {
			return err
		}
	}
	if err := projectAuthorizedGateLiveValidationPublicEntrypoint(handoff.LiveValidation, entrypoint); err != nil {
		return fmt.Errorf("liveValidation: %w", err)
	}
	return nil
}

func projectAuthorizedGateLiveValidationPublicEntrypoint(handoff *statusAuthorizedGateLiveValidationHandoff, entrypoint string) error {
	if handoff == nil {
		return nil
	}
	if err := projectPublicCommandFields(entrypoint,
		projectPublicCommandField{"dispatchCommand", &handoff.DispatchCommand},
		projectPublicCommandField{"validateCommand", &handoff.ValidateCommand},
		projectPublicCommandField{"recordCommand", &handoff.RecordCommand},
		projectPublicCommandField{"scaffoldCommand", &handoff.ScaffoldCommand},
		projectPublicCommandField{"scaffoldApplyCommand", &handoff.ScaffoldApplyCommand},
		projectPublicCommandField{"draftCommand", &handoff.DraftCommand},
		projectPublicCommandField{"draftApplyCommand", &handoff.DraftApplyCommand},
		projectPublicCommandField{"receiptPreviewCommand", &handoff.ReceiptPreviewCommand},
		projectPublicCommandField{"caseRelativeValidateCommand", &handoff.CaseRelativeValidateCommand},
		projectPublicCommandField{"caseRelativeRecordCommand", &handoff.CaseRelativeRecordCommand},
		projectPublicCommandField{"caseRelativeScaffoldCommand", &handoff.CaseRelativeScaffoldCommand},
		projectPublicCommandField{"caseRelativeScaffoldApplyCommand", &handoff.CaseRelativeScaffoldApplyCommand},
		projectPublicCommandField{"caseRelativeDraftCommand", &handoff.CaseRelativeDraftCommand},
		projectPublicCommandField{"caseRelativeDraftApplyCommand", &handoff.CaseRelativeDraftApplyCommand},
	); err != nil {
		return err
	}
	for index := range handoff.RunLoop {
		if err := projectPublicCommandFields(entrypoint,
			projectPublicCommandField{
				fmt.Sprintf("runLoop[%d].command", index),
				&handoff.RunLoop[index].Command,
			},
		); err != nil {
			return err
		}
	}
	return nil
}

func projectOpenDecisionHandoffPublicEntrypoint(handoff *statusOpenDecisionHandoff, entrypoint string) error {
	if handoff == nil {
		return nil
	}
	return projectPublicCommandFields(entrypoint,
		projectPublicCommandField{"sourceCommand", &handoff.SourceCommand},
		projectPublicCommandField{"reviewCommand", &handoff.ReviewCommand},
		projectPublicCommandField{"whatIfCommand", &handoff.WhatIfCommand},
		projectPublicCommandField{"recordCommand", &handoff.RecordCommand},
	)
}

func projectInterventionHandoffPublicEntrypoint(handoff *statusInterventionHandoff, entrypoint string) error {
	if handoff == nil {
		return nil
	}
	return projectPublicCommandFields(entrypoint,
		projectPublicCommandField{"reviewCommand", &handoff.ReviewCommand},
		projectPublicCommandField{"whatIfCommand", &handoff.WhatIfCommand},
		projectPublicCommandField{"applyCommand", &handoff.ApplyCommand},
	)
}

func projectCurrentLoopSegmentPublicEntrypoint(inspection *currentloop.Inspection, entrypoint string) error {
	if inspection == nil {
		return nil
	}
	if err := projectMissionCommanderDriverRequestPublicEntrypoint(&inspection.ResumeDriverRequest, entrypoint); err != nil {
		return fmt.Errorf("resumeDriverRequest: %w", err)
	}
	if err := projectMissionCommanderDriverRequestPublicEntrypoint(&inspection.RefreshedCurrentDriverRequest, entrypoint); err != nil {
		return fmt.Errorf("refreshedCurrentDriverRequest: %w", err)
	}
	if err := projectCurrentLoopContinuationPublicEntrypoint(inspection.Continuation, entrypoint); err != nil {
		return fmt.Errorf("continuation: %w", err)
	}
	return projectPublicCommandFields(entrypoint,
		projectPublicCommandField{"legacyUnboundWhatIfCommand", &inspection.LegacyUnboundWhatIfCommand},
	)
}

func projectCurrentLoopContinuationPublicEntrypoint(continuation *currentloop.Continuation, entrypoint string) error {
	if continuation == nil {
		return nil
	}
	if err := projectPublicCommandFields(entrypoint,
		projectPublicCommandField{"whatIfCommand", &continuation.WhatIfCommand},
	); err != nil {
		return err
	}
	if continuation.ObservationContract != nil {
		for index := range continuation.ObservationContract.Alternatives {
			if err := projectPublicCommandFields(entrypoint,
				projectPublicCommandField{
					fmt.Sprintf("observationContract.alternatives[%d].previewCommandTemplate", index),
					&continuation.ObservationContract.Alternatives[index].PreviewCommandTemplate,
				},
			); err != nil {
				return err
			}
		}
	}
	if continuation.ExternalMemberHandoff != nil {
		if err := projectCurrentLoopObservationContractPublicEntrypoint(
			&continuation.ExternalMemberHandoff.ObservationContract,
			entrypoint,
		); err != nil {
			return fmt.Errorf("externalMemberHandoff.observationContract: %w", err)
		}
	}
	return nil
}
