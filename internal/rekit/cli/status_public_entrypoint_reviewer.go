package cli

import (
	"fmt"

	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

func projectReviewerResultCollectionCommandsPublicEntrypoint(commands *workstream.ReviewerResultCollectionCommands, entrypoint string) error {
	if commands == nil {
		return nil
	}
	return projectPublicCommandFields(entrypoint,
		projectPublicCommandField{"previewCommand", &commands.PreviewCommand},
		projectPublicCommandField{"applyCommand", &commands.ApplyCommand},
	)
}

func projectReviewerDispatchIntakeHandoffPublicEntrypoint(handoff *workstream.ReviewerDispatchIntakeHandoff, entrypoint string) error {
	if handoff == nil {
		return nil
	}
	if err := projectPublicCommandFields(entrypoint,
		projectPublicCommandField{"dispatchPromptRepairCommand", &handoff.DispatchPromptRepairCommand},
		projectPublicCommandField{"reviewerDispatchRecordCommand", &handoff.ReviewerDispatchRecordCommand},
		projectPublicCommandField{"reviewerCompletionRecordCommand", &handoff.ReviewerCompletionRecordCommand},
		projectPublicCommandField{"reviewerResultInputSaveCommand", &handoff.ReviewerResultInputSaveCommand},
		projectPublicCommandField{"reviewerResultInputSaveApplyCommand", &handoff.ReviewerResultInputSaveApplyCommand},
		projectPublicCommandField{"reviewerResultSourceCaptureCommand", &handoff.ReviewerResultSourceCaptureCommand},
		projectPublicCommandField{"reviewerResultSourceCaptureApplyCommand", &handoff.ReviewerResultSourceCaptureApplyCommand},
		projectPublicCommandField{"reviewerResultStagingCommand", &handoff.ReviewerResultStagingCommand},
		projectPublicCommandField{"reviewerResultRecoveryCommand", &handoff.ReviewerResultRecoveryCommand},
		projectPublicCommandField{"reviewerResultRecoveryApplyCommand", &handoff.ReviewerResultRecoveryApplyCommand},
		projectPublicCommandField{"reviewerResultRecoveryDispositionCommand", &handoff.ReviewerResultRecoveryDispositionCommand},
		projectPublicCommandField{"dispatchCommand", &handoff.DispatchCommand},
		projectPublicCommandField{"previewCommand", &handoff.PreviewCommand},
		projectPublicCommandField{"applyCommand", &handoff.ApplyCommand},
		projectPublicCommandField{"batchPreviewCommand", &handoff.BatchPreviewCommand},
		projectPublicCommandField{"batchApplyCommand", &handoff.BatchApplyCommand},
		projectPublicCommandField{"refreshStatusCommand", &handoff.RefreshStatusCommand},
		projectPublicCommandField{"ownerAdoptionPreviewCommand", &handoff.OwnerAdoptionPreviewCommand},
		projectPublicCommandField{"packetRetirementPreviewCommand", &handoff.PacketRetirementPreviewCommand},
	); err != nil {
		return err
	}
	if err := projectReviewerResultCollectionCommandsPublicEntrypoint(handoff.ReviewerResultCollectionCommands, entrypoint); err != nil {
		return fmt.Errorf("reviewerResultCollectionCommands: %w", err)
	}
	if err := projectReviewerManagedDispatchPublicEntrypoint(handoff.ManagedDispatch, entrypoint); err != nil {
		return fmt.Errorf("managedDispatch: %w", err)
	}
	return nil
}

func projectReviewerManagedDispatchPublicEntrypoint(handoff *workstream.ReviewerManagedDispatchHandoff, entrypoint string) error {
	if handoff == nil {
		return nil
	}
	return projectPublicCommandFields(entrypoint,
		projectPublicCommandField{"inputSavePreviewCommand", &handoff.InputSavePreviewCommand},
		projectPublicCommandField{"inputSaveApplyCommand", &handoff.InputSaveApplyCommand},
		projectPublicCommandField{"sourceCapturePreviewCommand", &handoff.SourceCapturePreviewCommand},
		projectPublicCommandField{"sourceCaptureApplyCommand", &handoff.SourceCaptureApplyCommand},
		projectPublicCommandField{"stagingPreviewCommand", &handoff.StagingPreviewCommand},
		projectPublicCommandField{"collectionPreviewCommand", &handoff.CollectionPreviewCommand},
		projectPublicCommandField{"collectionApplyCommand", &handoff.CollectionApplyCommand},
		projectPublicCommandField{"intakePreviewCommand", &handoff.IntakePreviewCommand},
		projectPublicCommandField{"intakeApplyCommand", &handoff.IntakeApplyCommand},
		projectPublicCommandField{"dispatchCommand", &handoff.DispatchCommand},
		projectPublicCommandField{"nextAction", &handoff.NextAction},
	)
}

func projectReviewerDispatchOperatorPackagePublicEntrypoint(pkg *workstream.ReviewerDispatchOperatorPackage, entrypoint string) error {
	if pkg == nil {
		return nil
	}
	if err := projectMissionCommanderDriverRequestPublicEntrypoint(&pkg.CurrentDriverRequest, entrypoint); err != nil {
		return fmt.Errorf("currentDriverRequest: %w", err)
	}
	if err := projectPublicCommandFields(entrypoint,
		projectPublicCommandField{"refreshStatusCommand", &pkg.RefreshStatusCommand},
	); err != nil {
		return err
	}
	if err := projectReviewerDispatchOperatorItemPublicEntrypoint(pkg.Current, entrypoint); err != nil {
		return fmt.Errorf("current: %w", err)
	}
	if err := projectReviewerDispatchWavePublicEntrypoint(pkg.Wave, entrypoint); err != nil {
		return fmt.Errorf("wave: %w", err)
	}
	for index := range pkg.RunLoop {
		step := &pkg.RunLoop[index]
		if err := projectPublicCommandFields(entrypoint,
			projectPublicCommandField{fmt.Sprintf("runLoop[%d].command", index), &step.Command},
			projectPublicCommandField{fmt.Sprintf("runLoop[%d].previewCommand", index), &step.PreviewCommand},
			projectPublicCommandField{fmt.Sprintf("runLoop[%d].applyCommand", index), &step.ApplyCommand},
		); err != nil {
			return err
		}
	}
	return nil
}

func projectReviewerDispatchWavePublicEntrypoint(wave *workstream.ReviewerDispatchWavePackage, entrypoint string) error {
	if wave == nil {
		return nil
	}
	collections := []struct {
		label string
		items []workstream.ReviewerDispatchWavePackageItem
	}{
		{"spawnWave", wave.SpawnWave},
		{"active", wave.Active},
		{"returned", wave.Returned},
		{"failed", wave.Failed},
		{"blocked", wave.Blocked},
		{"complete", wave.Complete},
		{"shards", wave.Shards},
	}
	for _, collection := range collections {
		for index := range collection.items {
			if err := projectReviewerDispatchWaveItemPublicEntrypoint(&collection.items[index], entrypoint); err != nil {
				return fmt.Errorf("%s[%d]: %w", collection.label, index, err)
			}
		}
	}
	return nil
}

func projectReviewerDispatchWaveItemPublicEntrypoint(item *workstream.ReviewerDispatchWavePackageItem, entrypoint string) error {
	if item == nil {
		return nil
	}
	if err := projectPublicCommandFields(entrypoint,
		projectPublicCommandField{"recordDispatchCommand", &item.RecordDispatchCommand},
		projectPublicCommandField{"recordCompletionCommand", &item.RecordCompletionCommand},
	); err != nil {
		return err
	}
	if err := projectMissionCommanderDriverRequestPublicEntrypoint(&item.CurrentDriverRequest, entrypoint); err != nil {
		return fmt.Errorf("currentDriverRequest: %w", err)
	}
	if err := projectReviewerDispatchOperatorItemPublicEntrypoint(item.OperatorItem, entrypoint); err != nil {
		return fmt.Errorf("operatorItem: %w", err)
	}
	return nil
}

func projectReviewerDispatchOperatorItemPublicEntrypoint(item *workstream.ReviewerDispatchOperatorPackageItem, entrypoint string) error {
	if item == nil {
		return nil
	}
	return projectPublicCommandFields(entrypoint,
		projectPublicCommandField{"dispatchPromptRepairCommand", &item.DispatchPromptRepairCommand},
		projectPublicCommandField{"reviewerDispatchRecordCommand", &item.ReviewerDispatchRecordCommand},
		projectPublicCommandField{"reviewerCompletionRecordCommand", &item.ReviewerCompletionRecordCommand},
		projectPublicCommandField{"reviewerResultInputSavePreviewCommand", &item.ReviewerResultInputSavePreviewCommand},
		projectPublicCommandField{"reviewerResultInputSaveApplyCommand", &item.ReviewerResultInputSaveApplyCommand},
		projectPublicCommandField{"reviewerResultSourceCapturePreviewCommand", &item.ReviewerResultSourceCapturePreviewCommand},
		projectPublicCommandField{"reviewerResultSourceCaptureApplyCommand", &item.ReviewerResultSourceCaptureApplyCommand},
		projectPublicCommandField{"reviewerResultStagingPreviewCommand", &item.ReviewerResultStagingPreviewCommand},
		projectPublicCommandField{"reviewerResultCollectionPreviewCommand", &item.ReviewerResultCollectionPreviewCommand},
		projectPublicCommandField{"reviewerResultCollectionApplyCommand", &item.ReviewerResultCollectionApplyCommand},
		projectPublicCommandField{"reviewerResultIntakePreviewCommand", &item.ReviewerResultIntakePreviewCommand},
		projectPublicCommandField{"reviewerResultIntakeApplyCommand", &item.ReviewerResultIntakeApplyCommand},
		projectPublicCommandField{"reviewerResultBatchIntakePreviewCommand", &item.ReviewerResultBatchIntakePreviewCommand},
		projectPublicCommandField{"reviewerResultBatchIntakeApplyCommand", &item.ReviewerResultBatchIntakeApplyCommand},
		projectPublicCommandField{"dispatchCommand", &item.DispatchCommand},
		projectPublicCommandField{"nextAction", &item.NextAction},
	)
}

func projectReviewerDispatchIntakeSummaryPublicEntrypoint(summary *workstream.ReviewerDispatchIntakeSummary, entrypoint string) error {
	if summary == nil {
		return nil
	}
	if err := projectPublicCommandFields(entrypoint,
		projectPublicCommandField{"latestReviewerResultSourceCaptureCommand", &summary.LatestReviewerResultSourceCaptureCommand},
		projectPublicCommandField{"latestReviewerResultSourceCaptureApplyCommand", &summary.LatestReviewerResultSourceCaptureApplyCommand},
		projectPublicCommandField{"latestReviewerResultStagingCommand", &summary.LatestReviewerResultStagingCommand},
		projectPublicCommandField{"latestCollectionPreviewCommand", &summary.LatestCollectionPreviewCommand},
		projectPublicCommandField{"latestCollectionApplyCommand", &summary.LatestCollectionApplyCommand},
		projectPublicCommandField{"latestPreviewCommand", &summary.LatestPreviewCommand},
		projectPublicCommandField{"latestApplyCommand", &summary.LatestApplyCommand},
		projectPublicCommandField{"latestBatchPreviewCommand", &summary.LatestBatchPreviewCommand},
		projectPublicCommandField{"latestBatchApplyCommand", &summary.LatestBatchApplyCommand},
		projectPublicCommandField{"nextActionDispatchPromptRepairCommand", &summary.NextActionDispatchPromptRepairCommand},
		projectPublicCommandField{"nextActionReviewerResultSourceCaptureCommand", &summary.NextActionReviewerResultSourceCaptureCommand},
		projectPublicCommandField{"nextActionReviewerResultSourceCaptureApplyCommand", &summary.NextActionReviewerResultSourceCaptureApplyCommand},
		projectPublicCommandField{"nextActionReviewerResultStagingCommand", &summary.NextActionReviewerResultStagingCommand},
		projectPublicCommandField{"nextActionCollectionPreviewCommand", &summary.NextActionCollectionPreviewCommand},
		projectPublicCommandField{"nextActionCollectionApplyCommand", &summary.NextActionCollectionApplyCommand},
		projectPublicCommandField{"nextActionPreviewCommand", &summary.NextActionPreviewCommand},
		projectPublicCommandField{"nextActionApplyCommand", &summary.NextActionApplyCommand},
		projectPublicCommandField{"nextActionBatchPreviewCommand", &summary.NextActionBatchPreviewCommand},
		projectPublicCommandField{"nextActionBatchApplyCommand", &summary.NextActionBatchApplyCommand},
		projectPublicCommandField{"nextActionPacketRetirementPreviewCommand", &summary.NextActionPacketRetirementPreviewCommand},
		projectPublicCommandField{"nextAction", &summary.NextAction},
	); err != nil {
		return err
	}
	if err := projectReviewerDispatchOperatorPackagePublicEntrypoint(summary.OperatorPackage, entrypoint); err != nil {
		return fmt.Errorf("operatorPackage: %w", err)
	}
	return nil
}

func projectReviewerPacketRetirementHandoffPublicEntrypoint(handoff *workstream.ReviewerPacketRetirementHandoff, entrypoint string) error {
	if handoff == nil {
		return nil
	}
	return projectPublicCommandFields(entrypoint,
		projectPublicCommandField{"nextAction", &handoff.NextAction},
	)
}

func projectReviewerPacketRetirementSummaryPublicEntrypoint(summary *workstream.ReviewerPacketRetirementSummary, entrypoint string) error {
	if summary == nil {
		return nil
	}
	return projectPublicCommandFields(entrypoint,
		projectPublicCommandField{"nextAction", &summary.NextAction},
	)
}
