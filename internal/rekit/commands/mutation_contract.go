package commands

import (
	"sort"
	"strings"
)

const (
	MutationModeDefault = "default"

	MutationModeAttach                         = "attach"
	MutationModeRepair                         = "repair"
	MutationModeStart                          = "start"
	MutationModeReconcile                      = "reconcile"
	MutationModeOrdinaryOnboarding             = "ordinary-onboarding"
	MutationModeAttachedAdoption               = "attached-adoption"
	MutationModeDirectAppend                   = "direct-append"
	MutationModeCurrentSync                    = "current-sync"
	MutationModeSelectedPackMemorySync         = "selected-pack-memory-sync"
	MutationModePackMemoryConsumerVerification = "pack-memory-consumer-verification"
	MutationModeOrdinarySync                   = "ordinary-sync"
	MutationModeMemberOutputStaging            = "member-output-staging"
	MutationModeCandidateRetirement            = "candidate-retirement"
	MutationModeCandidateProvision             = "candidate-provision"
	MutationModeCandidateVerification          = "candidate-verification"
	MutationModeReviewProofDraft               = "review-proof-draft"
	MutationModeCandidateDecisionDraft         = "candidate-decision-draft"
	MutationModeCandidateDecisionApply         = "candidate-decision-apply"
	MutationModeOrdinaryPromote                = "ordinary-promote"
	MutationModeCreateCandidates               = "create-candidates"
	MutationModeRecordReviewerDispatch         = "record-reviewer-dispatch"
	MutationModeRecordReviewerCompletion       = "record-reviewer-completion"
	MutationModeSaveReviewerResultInput        = "save-reviewer-result-input"
	MutationModeCaptureReviewerResultSource    = "capture-reviewer-result-source"
	MutationModeStageReviewerResult            = "stage-reviewer-result"
	MutationModeCollectReviewerResult          = "collect-reviewer-result"
	MutationModeRetireReviewerRecovery         = "retire-reviewer-recovery"
	MutationModeRetireReviewerPacket           = "retire-reviewer-packet"
	MutationModeRecoverReviewerResult          = "recover-reviewer-result"
	MutationModeRepairReviewerPrompt           = "repair-reviewer-prompt"
	MutationModeAdoptReviewerPacket            = "adopt-reviewer-packet"
	MutationModeReviewerBatchIntake            = "reviewer-batch-intake"
	MutationModeReviewerIntake                 = "reviewer-intake"
	MutationModePlanArtifacts                  = "plan-artifacts"
	MutationModeProfile                        = "profile"
	MutationModeAdapterDispatch                = "adapter-dispatch"
	MutationModeAdapterReceipt                 = "adapter-receipt"
	MutationModeReportScaffold                 = "report-scaffold"
	MutationModeReportDraft                    = "report-draft"
	MutationModeDecision                       = "decision"
	MutationModeExecutionObservation           = "execution-observation"
	MutationModeEventAppend                    = "event-append"
	MutationModeBoardBootstrap                 = "board-bootstrap"
	MutationModeValidationReceipt              = "validation-receipt"
	MutationModeExternalAttempt                = "external-attempt"
	MutationModeExternalDispatch               = "external-dispatch"
	MutationModeExternalTurn                   = "external-turn"
	MutationModeExternalRelay                  = "external-relay"

	MutationCurrentnessStrictPlan      = "strict-plan"
	MutationCurrentnessOuterPlan       = "outer-plan"
	MutationCurrentnessResourceBinding = "resource-binding"
	MutationCurrentnessFreshReplan     = "fresh-replan"
	MutationCurrentnessImplicit        = "implicit"
)

const (
	MutationCarrierTypedInvocation = "typed-invocation"
	MutationCarrierActionQueue     = "action-queue"
	MutationCarrierApplyCommand    = "apply-command"
	MutationCarrierApplyArgs       = "apply-args"
	MutationCarrierRecordCommand   = "record-command"
	MutationCarrierRecordArgs      = "record-args"
	MutationCarrierNone            = "none"
)

// MutationContract records the currentness and exact-action protocol of one
// command mode. Mode-specific entries prevent a command with several distinct
// write routes from being assigned one fictional command-wide hash.
type MutationContract struct {
	Command                  string   `json:"command"`
	Mode                     string   `json:"mode"`
	Selector                 string   `json:"selector,omitempty"`
	Currentness              string   `json:"currentness"`
	ExpectedFlag             string   `json:"expectedFlag,omitempty"`
	ExpectedAliases          []string `json:"expectedAliases,omitempty"`
	NestedBinding            bool     `json:"nestedBinding"`
	NestedExpectedFlags      []string `json:"nestedExpectedFlags,omitempty"`
	NestedBindingRef         string   `json:"nestedBindingRef,omitempty"`
	ExactCarriers            []string `json:"exactCarriers"`
	PartialProgress          string   `json:"partialProgress"`
	ExecutablePlanValidation bool     `json:"executablePlanValidation"`
	Confirmed                bool     `json:"confirmed"`
	Notes                    string   `json:"notes,omitempty"`
}

var mutationContracts = []MutationContract{
	strictPlanContract(Bootstrap, "-ExpectedInitPlanSha256", "--expected-init-plan-sha256", MutationCarrierApplyCommand, MutationCarrierApplyArgs),
	strictPlanContract(Complete, "-ExpectedCompletePlanSha256", "--expected-complete-plan-sha256", MutationCarrierApplyCommand),
	strictPlanContract(Continue, "-ExpectedContinuePlanSha256", "--expected-continue-plan-sha256", MutationCarrierTypedInvocation, MutationCarrierActionQueue),
	strictPlanContract(Control, "-ExpectedControlPlanSha256", "--expected-control-plan-sha256", MutationCarrierApplyCommand),
	strictPlanContract(Handoff, "-ExpectedHandoffPlanSha256", "--expected-handoff-plan-sha256", MutationCarrierTypedInvocation, MutationCarrierActionQueue, MutationCarrierApplyCommand, MutationCarrierApplyArgs),
	strictPlanContract(Init, "-ExpectedInitPlanSha256", "--expected-init-plan-sha256", MutationCarrierApplyCommand, MutationCarrierApplyArgs),
	strictPlanContract(MigrateState, "-ExpectedMigrationPlanSha256", "--expected-migration-plan-sha256", MutationCarrierApplyCommand, MutationCarrierApplyArgs),
	strictExecutableModeContract(Onboard, MutationModeOrdinaryOnboarding, "-ExpectedOnboardingPlanSha256", "--expected-onboarding-plan-sha256", MutationCarrierApplyCommand, MutationCarrierApplyArgs),
	strictExecutableModeContract(Onboard, MutationModeAttachedAdoption, "-ExpectedOnboardingPlanSha256", "--expected-onboarding-plan-sha256", MutationCarrierApplyCommand, MutationCarrierApplyArgs),
	strictPlanContract(Reopen, "-ExpectedReopenPlanSha256", "--expected-reopen-plan-sha256", MutationCarrierApplyCommand),

	outerPlanContract(RunDriverStep, MutationModeDefault, "-ExpectedDriverStepPlanSha256", "--expected-driver-step-plan-sha256", []string{"-ExpectedDriverStepPreviewSha256"}, MutationCarrierTypedInvocation, MutationCarrierActionQueue),
	outerPlanContract(RunCurrentStep, MutationModeDefault, "-ExpectedCurrentStepPlanSha256", "--expected-current-step-plan-sha256", []string{"-ExpectedMemberExecutionPlanSha256", "-ExpectedDriverStepPlanSha256", "-ExpectedReviewerStepPlanSha256"}, MutationCarrierTypedInvocation, MutationCarrierActionQueue),
	outerPlanContract(RunCurrentLoop, MutationModeDefault, "-ExpectedCurrentLoopPlanSha256", "--expected-current-loop-plan-sha256", []string{"-ExpectedCurrentStepPlanSha256", "-ExpectedMemberExecutionPlanSha256", "-ExpectedCurrentLoopCheckpointSha256", "-ExpectedCurrentLoopObservationSha256", "-ExpectedCurrentLoopReviewerAttemptSha256"}, MutationCarrierNone),
	outerPlanContract(RunReviewerStep, MutationModeDefault, "-ExpectedReviewerStepPlanSha256", "--expected-reviewer-step-plan-sha256", reviewerStepNestedExpectedFlags(), MutationCarrierTypedInvocation, MutationCarrierActionQueue),
	outerPlanContract(RunReviewerWave, MutationModeDefault, "-ExpectedReviewerWavePlanSha256", "--expected-reviewer-wave-plan-sha256", reviewerWaveNestedExpectedFlags(), MutationCarrierNone),

	strictModeContract(Sync, MutationModeCurrentSync, "-ExpectedCurrentSyncPlanSha256", "--expected-current-sync-plan-sha256", MutationCarrierApplyArgs),
	strictModeContract(Sync, MutationModeSelectedPackMemorySync, "-ExpectedPackMemoryConsumptionPlanSha256", "--expected-pack-memory-consumption-plan-sha256", MutationCarrierApplyCommand),
	unboundMutationContract(Sync, MutationModePackMemoryConsumerVerification, MutationCurrentnessFreshReplan, MutationCarrierNone),
	unboundMutationContract(Sync, MutationModeOrdinarySync, MutationCurrentnessFreshReplan, MutationCarrierNone),
	unboundMutationContract(Update, MutationModeOrdinarySync, MutationCurrentnessFreshReplan, MutationCarrierNone),
	strictModeContract(NextBatch, MutationModeDefault, "-ExpectedNextBatchPlanSha256", "--expected-next-batch-plan-sha256", MutationCarrierActionQueue),

	unboundMutationContract(Note, MutationModeDirectAppend, MutationCurrentnessImplicit, MutationCarrierNone),
	resourceBindingContract(Note, MutationModeEventAppend, "-ExpectedNoteEventSha256", "--expected-note-event-sha256", []string{"-ExpectedExecutionControlBindingJson"}, MutationCarrierRecordCommand, MutationCarrierRecordArgs),
	resourceBindingContract(Promote, MutationModeMemberOutputStaging, "-ExpectedMemberOutputStagingPlanSha256", "--expected-member-output-staging-plan-sha256", []string{"-ExpectedMemberExecutionPlanSha256"}, MutationCarrierApplyCommand),
	strictModeContract(Promote, MutationModeCandidateRetirement, "-ExpectedRetirementSha256", "--expected-retirement-sha256", MutationCarrierApplyCommand, MutationCarrierActionQueue),
	strictModeContract(Promote, MutationModeCandidateProvision, "-ExpectedProvisionSha256", "--expected-provision-sha256", MutationCarrierApplyCommand, MutationCarrierActionQueue),
	unboundMutationContract(Promote, MutationModeCandidateVerification, MutationCurrentnessFreshReplan, MutationCarrierNone),
	strictModeContract(Promote, MutationModeReviewProofDraft, "-ExpectedProofSha256", "--expected-proof-sha256", MutationCarrierApplyCommand, MutationCarrierActionQueue),
	strictModeContract(Promote, MutationModeCandidateDecisionDraft, "-ExpectedDecisionSha256", "--expected-decision-sha256", MutationCarrierApplyCommand, MutationCarrierActionQueue),
	unboundMutationContract(Promote, MutationModeCandidateDecisionApply, MutationCurrentnessFreshReplan, MutationCarrierNone),
	unboundMutationContract(Promote, MutationModeOrdinaryPromote, MutationCurrentnessFreshReplan, MutationCarrierNone),
	unboundMutationContract(Promote, MutationModeCreateCandidates, MutationCurrentnessImplicit, MutationCarrierNone),

	resourceBindingContract(PlanSubagents, MutationModeRecordReviewerDispatch, "-ExpectedReviewerDispatchBindingSha256", "--expected-reviewer-dispatch-binding-sha256", nil, MutationCarrierApplyCommand, MutationCarrierActionQueue),
	resourceBindingContract(PlanSubagents, MutationModeRecordReviewerCompletion, "-ExpectedReviewerDispatchReceiptSha256", "--expected-reviewer-dispatch-receipt-sha256", []string{"-ExpectedReviewerResultInputSha256"}, MutationCarrierApplyCommand, MutationCarrierActionQueue),
	resourceBindingContract(PlanSubagents, MutationModeSaveReviewerResultInput, "-ExpectedReviewerResultInputSha256", "--expected-reviewer-result-input-sha256", nil, MutationCarrierApplyCommand, MutationCarrierActionQueue),
	resourceBindingContract(PlanSubagents, MutationModeCaptureReviewerResultSource, "-ExpectedReviewerResultInputSha256", "--expected-reviewer-result-input-sha256", nil, MutationCarrierApplyCommand, MutationCarrierActionQueue),
	resourceBindingContract(PlanSubagents, MutationModeStageReviewerResult, "-ExpectedSourceSha256", "--expected-source-sha256", nil, MutationCarrierApplyCommand, MutationCarrierActionQueue),
	resourceBindingContract(PlanSubagents, MutationModeCollectReviewerResult, "-ExpectedCandidateSha256", "--expected-candidate-sha256", nil, MutationCarrierApplyCommand, MutationCarrierActionQueue),
	resourceBindingContract(PlanSubagents, MutationModeRetireReviewerRecovery, "-ExpectedIntentSha256", "--expected-intent-sha256", []string{"-ExpectedCanonicalSha256"}, MutationCarrierApplyCommand, MutationCarrierActionQueue),
	resourceBindingContract(PlanSubagents, MutationModeRetireReviewerPacket, "-ExpectedPacketSha256", "--expected-packet-sha256", []string{"-ExpectedIntegritySha256"}, MutationCarrierApplyCommand, MutationCarrierActionQueue),
	resourceBindingContract(PlanSubagents, MutationModeRecoverReviewerResult, "-ExpectedCandidateSha256", "--expected-candidate-sha256", []string{"-ExpectedReviewerResultSha256"}, MutationCarrierApplyCommand, MutationCarrierActionQueue),
	resourceBindingContract(PlanSubagents, MutationModeRepairReviewerPrompt, "-ExpectedPromptSha256", "--expected-prompt-sha256", nil, MutationCarrierApplyCommand, MutationCarrierActionQueue),
	unboundMutationContract(PlanSubagents, MutationModeAdoptReviewerPacket, MutationCurrentnessFreshReplan, MutationCarrierNone),
	unboundMutationContract(PlanSubagents, MutationModeReviewerBatchIntake, MutationCurrentnessFreshReplan, MutationCarrierNone),
	unboundMutationContract(PlanSubagents, MutationModeReviewerIntake, MutationCurrentnessFreshReplan, MutationCarrierNone),
	unboundMutationContract(PlanSubagents, MutationModePlanArtifacts, MutationCurrentnessImplicit, MutationCarrierNone),

	strictModeContract(Gate, MutationModeProfile, "-ExpectedProfilePlanSha256", "--expected-profile-plan-sha256", MutationCarrierNone),
	resourceBindingContract(Gate, MutationModeAdapterDispatch, "-ExpectedAdapterExecutionDispatchBindingSha256", "--expected-adapter-execution-dispatch-binding-sha256", nil, MutationCarrierApplyCommand),
	resourceBindingContract(Gate, MutationModeAdapterReceipt, "-ExpectedAdapterExecutionBindingSha256", "--expected-adapter-execution-binding-sha256", []string{"-ExpectedAdapterExecutionReceiptSha256"}, MutationCarrierApplyCommand),
	resourceBindingContract(Gate, MutationModeReportScaffold, "-ExpectedExecutionReportSha256", "--expected-execution-report-sha256", nil, MutationCarrierApplyCommand),
	resourceBindingContract(Gate, MutationModeReportDraft, "-ExpectedExecutionReportSha256", "--expected-execution-report-sha256", nil, MutationCarrierRecordCommand),
	unboundMutationContract(Gate, MutationModeDecision, MutationCurrentnessFreshReplan, MutationCarrierNone),
	resourceBindingContract(Gate, MutationModeExecutionObservation, "-ExpectedAdapterExecutionReceiptSha256", "--expected-adapter-execution-receipt-sha256", nil, MutationCarrierNone),

	unboundMutationContract(Attach, MutationModeDefault, MutationCurrentnessFreshReplan, MutationCarrierNone),
	unboundMutationContract(Repair, MutationModeDefault, MutationCurrentnessFreshReplan, MutationCarrierNone),
	unboundMutationContract(Start, MutationModeDefault, MutationCurrentnessFreshReplan, MutationCarrierNone),
	unboundMutationContract(Reconcile, MutationModeDefault, MutationCurrentnessFreshReplan, MutationCarrierNone),
	unboundMutationContract(Overview, MutationModeBoardBootstrap, MutationCurrentnessImplicit, MutationCarrierNone),
	unboundMutationContract(ReleaseRun, MutationModeValidationReceipt, MutationCurrentnessImplicit, MutationCarrierNone),

	resourceBindingContract(RunCurrentLoop, MutationModeExternalAttempt, "-ExpectedExternalSessionAttemptPlanSha256", "--expected-external-session-attempt-plan-sha256", []string{"-ExpectedCurrentLoopCheckpointSha256", "-ExpectedExternalSessionJobSha256", "-ExpectedExternalSessionAttemptSha256"}, MutationCarrierApplyCommand),
	resourceBindingContract(RunCurrentLoop, MutationModeExternalDispatch, "-ExpectedExternalSessionDispatchPlanSha256", "--expected-external-session-dispatch-plan-sha256", []string{"-ExpectedCurrentLoopCheckpointSha256", "-ExpectedExternalSessionJobSha256", "-ExpectedExternalSessionAttemptSha256", "-ExpectedExternalSessionDispatchSha256", "-ExpectedExternalSessionClaimSha256"}, MutationCarrierApplyCommand),
	resourceBindingContract(RunCurrentLoop, MutationModeExternalTurn, "-ExpectedExternalSessionTurnPlanSha256", "--expected-external-session-turn-plan-sha256", []string{"-ExpectedCurrentLoopCheckpointSha256", "-ExpectedExternalSessionJobSha256", "-ExpectedExternalSessionSubmissionSha256", "-ExpectedExternalSessionRelayPlanSha256"}, MutationCarrierApplyCommand),
	resourceBindingContract(RunCurrentLoop, MutationModeExternalRelay, "-ExpectedExternalSessionRelayPlanSha256", "--expected-external-session-relay-plan-sha256", []string{"-ExpectedCurrentLoopCheckpointSha256", "-ExpectedExternalSessionJobSha256", "-ExpectedExternalSessionSubmissionSha256"}, MutationCarrierApplyCommand),
}

func strictPlanContract(command, flag, alias string, carriers ...string) MutationContract {
	contract := strictModeContract(command, MutationModeDefault, flag, alias, carriers...)
	contract.ExecutablePlanValidation = true
	return contract
}

func strictModeContract(command, mode, flag, alias string, carriers ...string) MutationContract {
	return boundMutationContract(command, mode, MutationCurrentnessStrictPlan, flag, alias, nil, carriers...)
}

func strictExecutableModeContract(command, mode, flag, alias string, carriers ...string) MutationContract {
	contract := strictModeContract(command, mode, flag, alias, carriers...)
	contract.ExecutablePlanValidation = true
	return contract
}

func outerPlanContract(command, mode, flag, alias string, nested []string, carriers ...string) MutationContract {
	return boundMutationContract(command, mode, MutationCurrentnessOuterPlan, flag, alias, nested, carriers...)
}

func resourceBindingContract(command, mode, flag, alias string, nested []string, carriers ...string) MutationContract {
	return boundMutationContract(command, mode, MutationCurrentnessResourceBinding, flag, alias, nested, carriers...)
}

func boundMutationContract(command, mode, currentness, flag, alias string, nested []string, carriers ...string) MutationContract {
	return MutationContract{
		Command:             command,
		Mode:                mode,
		Currentness:         currentness,
		ExpectedFlag:        flag,
		ExpectedAliases:     []string{flag, alias},
		NestedBinding:       len(nested) > 0,
		NestedExpectedFlags: append([]string{}, nested...),
		ExactCarriers:       append([]string{}, carriers...),
		PartialProgress:     mutationPartialProgress(currentness, command, mode),
		Confirmed:           true,
	}
}

func unboundMutationContract(command, mode, currentness string, carriers ...string) MutationContract {
	return MutationContract{
		Command:         command,
		Mode:            mode,
		Currentness:     currentness,
		ExactCarriers:   append([]string{}, carriers...),
		PartialProgress: mutationPartialProgress(currentness, command, mode),
		Confirmed:       true,
	}
}

func mutationPartialProgress(currentness, command, mode string) string {
	switch {
	case command == RunReviewerWave:
		return "ordered-prefix"
	case command == RunCurrentLoop && mode == MutationModeDefault:
		return "ordered-prefix"
	case currentness == MutationCurrentnessOuterPlan:
		return "delegated"
	case currentness == MutationCurrentnessStrictPlan:
		return "recoverable"
	case currentness == MutationCurrentnessResourceBinding:
		return "none"
	case currentness == MutationCurrentnessImplicit:
		return "possible-unconfirmed"
	default:
		return "possible-unconfirmed"
	}
}

func reviewerWaveNestedExpectedFlags() []string {
	return []string{
		"-ExpectedReviewerDispatchBindingSha256",
		"-ExpectedReviewerDispatchReceiptSha256",
		"-ExpectedReviewerResultInputSha256",
		"-ExpectedReviewerResultSha256",
	}
}

func reviewerStepNestedExpectedFlags() []string {
	return []string{
		"-ExpectedPromptSha256",
		"-ExpectedReviewerDispatchBindingSha256",
		"-ExpectedReviewerDispatchReceiptSha256",
		"-ExpectedReviewerResultInputSha256",
		"-ExpectedSourceSha256",
		"-ExpectedCandidateSha256",
	}
}

func MutationContracts() []MutationContract {
	out := make([]MutationContract, 0, len(mutationContracts))
	for _, contract := range mutationContracts {
		out = append(out, cloneMutationContract(contract))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Command == out[j].Command {
			return out[i].Mode < out[j].Mode
		}
		return out[i].Command < out[j].Command
	})
	return out
}

func MutationContractFor(command, mode string) (MutationContract, bool) {
	command = strings.ToLower(strings.TrimSpace(command))
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode != "" {
		for _, contract := range mutationContracts {
			if contract.Command == command && contract.Mode == mode {
				return cloneMutationContract(contract), true
			}
		}
		return MutationContract{}, false
	}

	var shared *MutationContract
	commandContracts := 0
	strictContracts := 0
	for _, contract := range mutationContracts {
		if contract.Command != command {
			continue
		}
		commandContracts++
		if contract.Currentness != MutationCurrentnessStrictPlan {
			continue
		}
		strictContracts++
		if shared == nil {
			copy := cloneMutationContract(contract)
			shared = &copy
			continue
		}
		if shared.ExpectedFlag != contract.ExpectedFlag ||
			!strings.EqualFold(strings.Join(shared.ExpectedAliases, "\x00"), strings.Join(contract.ExpectedAliases, "\x00")) {
			return MutationContract{}, false
		}
		shared.ExactCarriers = append(shared.ExactCarriers, contract.ExactCarriers...)
		shared.NestedExpectedFlags = append(shared.NestedExpectedFlags, contract.NestedExpectedFlags...)
		shared.NestedBinding = shared.NestedBinding || contract.NestedBinding
	}
	if shared != nil {
		if commandContracts != strictContracts {
			return MutationContract{}, false
		}
		shared.Mode = MutationModeDefault
		shared.ExactCarriers = uniqueStrings(shared.ExactCarriers)
		shared.NestedExpectedFlags = uniqueStrings(shared.NestedExpectedFlags)
		return *shared, true
	}
	for _, contract := range mutationContracts {
		if contract.Command == command && contract.Mode == MutationModeDefault {
			return cloneMutationContract(contract), true
		}
	}
	return MutationContract{}, false
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func cloneMutationContract(contract MutationContract) MutationContract {
	contract.ExpectedAliases = append([]string{}, contract.ExpectedAliases...)
	contract.NestedExpectedFlags = append([]string{}, contract.NestedExpectedFlags...)
	contract.ExactCarriers = append([]string{}, contract.ExactCarriers...)
	return contract
}
