package promote

import (
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

const candidatePostDecisionStatusCommand = candidateDraftRefreshStatusCommand

func finalizeCandidateDecisionResult(result CandidateDecisionResult) CandidateDecisionResult {
	state, command, requiresReview := candidateDecisionDriver(result)
	result.MissionCommanderAction, result.MissionCommanderActionQueue = candidatePostDecisionMissionCommanderActionQueue(
		result.Pack,
		state,
		"packMemoryCandidateDecision",
		"pack-memory-candidate-decision:"+shortHash(result.PacketHash+result.DecisionPath),
		command,
		requiresReview,
		result.NextSteps,
		result.Boundary,
	)
	return result
}

func finalizeCandidateVerificationProvisionResult(result CandidateVerificationProvisionResult) CandidateVerificationProvisionResult {
	state, command, requiresReview := candidateVerificationProvisionDriver(result)
	result.MissionCommanderAction, result.MissionCommanderActionQueue = candidatePostDecisionMissionCommanderActionQueue(
		result.Pack,
		state,
		"packMemoryCandidateVerificationProvision",
		"pack-memory-verification-provision:"+shortHash(result.DecisionSHA256+result.WorkspaceRoot),
		command,
		requiresReview,
		result.NextSteps,
		result.Boundary,
	)
	return result
}

func finalizeCandidateDecisionVerificationResult(result CandidateDecisionVerificationResult) CandidateDecisionVerificationResult {
	state, command, requiresReview := candidateDecisionVerificationDriver(result)
	result.MissionCommanderAction, result.MissionCommanderActionQueue = candidatePostDecisionMissionCommanderActionQueue(
		result.Pack,
		state,
		"packMemoryCandidateVerification",
		"pack-memory-candidate-verification:"+shortHash(result.PacketHash+result.DecisionHash),
		command,
		requiresReview,
		result.NextSteps,
		result.Boundary,
	)
	return result
}

func finalizeCandidateVerificationRetirementResult(result CandidateVerificationRetirementResult) CandidateVerificationRetirementResult {
	state, command, requiresReview := candidateVerificationRetirementDriver(result)
	result.MissionCommanderAction, result.MissionCommanderActionQueue = candidatePostDecisionMissionCommanderActionQueue(
		result.Pack,
		state,
		"packMemoryCandidateVerificationRetirement",
		"pack-memory-verification-retirement:"+shortHash(result.DecisionSHA256+result.WorkspaceRoot),
		command,
		requiresReview,
		result.NextSteps,
		result.Boundary,
	)
	return result
}

func candidateDecisionDriver(result CandidateDecisionResult) (state, command string, requiresReview bool) {
	if result.RecoveryRequired || result.FailedAction != "" || result.RolledBack {
		return "pack-memory-candidate-decision-recovery-required", candidatePostDecisionStatusCommand, true
	}
	if !result.Applied || !result.IsMutation {
		return "ready-for-pack-memory-candidate-decision-apply", candidateDecisionApplyCommand(result), true
	}
	if result.Receipt != nil && result.Receipt.VerificationPending && result.Receipt.VerificationProvisionCommand != "" {
		return "ready-for-pack-memory-verification-provision-preview", candidatePromoteCommandWithTarget(result.Receipt.VerificationProvisionCommand, result.CaseRoot), true
	}
	return "pack-memory-candidate-decision-applied-refresh-required", candidatePostDecisionStatusCommand, false
}

func candidateVerificationProvisionDriver(result CandidateVerificationProvisionResult) (state, command string, requiresReview bool) {
	if !result.Applied || !result.IsMutation {
		return "ready-for-pack-memory-verification-provision-apply", strings.TrimSpace(result.ApplyCommand), true
	}
	if result.VerificationPreviewCommand != "" {
		return "ready-for-pack-memory-candidate-verification-preview", candidatePromoteCommandWithTarget(result.VerificationPreviewCommand, result.SourceCaseRoot), true
	}
	return "pack-memory-verification-provisioned-refresh-required", candidatePostDecisionStatusCommand, false
}

func candidateDecisionVerificationDriver(result CandidateDecisionVerificationResult) (state, command string, requiresReview bool) {
	if !result.Applied || !result.IsMutation {
		if strings.TrimSpace(result.PacketPath) == "" || strings.TrimSpace(result.DecisionPath) == "" {
			return "pack-memory-candidate-verification-refresh-required", candidatePostDecisionStatusCommand, false
		}
		return "ready-for-pack-memory-candidate-verification-apply", candidateDecisionVerificationApplyCommand(result), true
	}
	if result.RetirementPreviewCommand != "" {
		return "ready-for-pack-memory-verification-retirement-preview", candidatePromoteCommandWithTarget(result.RetirementPreviewCommand, result.CaseRoot), true
	}
	return "pack-memory-candidate-verification-verified-refresh-required", candidatePostDecisionStatusCommand, false
}

func candidateVerificationRetirementDriver(result CandidateVerificationRetirementResult) (state, command string, requiresReview bool) {
	if !result.Applied || !result.IsMutation {
		if result.Replay || result.Mode == "already-retired" {
			return "pack-memory-verification-already-retired-refresh-required", candidatePostDecisionStatusCommand, false
		}
		return "ready-for-pack-memory-verification-retirement-apply", strings.TrimSpace(result.ApplyCommand), true
	}
	if result.Replay || result.Mode == "already-retired" {
		return "pack-memory-verification-already-retired-refresh-required", candidatePostDecisionStatusCommand, false
	}
	return "pack-memory-verification-retired-refresh-required", candidatePostDecisionStatusCommand, false
}

func candidatePostDecisionMissionCommanderActionQueue(pack, state, source, actionID, command string, requiresReview bool, reasons, boundary []string) (*mission.MissionCommanderAction, *mission.MissionCommanderActionQueue) {
	command = strings.TrimSpace(command)
	if command == "" {
		command = candidatePostDecisionStatusCommand
		requiresReview = false
	}
	boundary = mission.UniqueStrings(append([]string{}, boundary...))
	action := &mission.MissionCommanderAction{
		State:          state,
		Prompt:         "Follow the returned pack-memory post-decision driver request, then refresh status or release-check before selecting follow-up work.",
		PrimaryCommand: command,
		Boundary:       boundary,
	}
	queue := mission.MissionCommanderActionQueueFor([]mission.MissionCommanderNextActionItem{{
		Label:          strings.TrimSpace(pack),
		ActionID:       strings.TrimSpace(actionID),
		State:          state,
		Command:        command,
		Source:         strings.TrimSpace(source),
		RequiresReview: requiresReview,
		Reasons:        append([]string{}, reasons...),
		Boundary:       boundary,
	}})
	return action, &queue
}

func candidateDecisionApplyCommand(result CandidateDecisionResult) string {
	return "/rekit promote -Target " + quoteCandidateDecisionArg(result.CaseRoot) +
		" -PacketPath " + quoteCandidateDecisionArg(result.PacketPath) +
		" -CandidateDecisionPath " + quoteCandidateDecisionArg(result.DecisionPath) +
		" -Apply -Format json"
}

func candidateDecisionVerificationApplyCommand(result CandidateDecisionVerificationResult) string {
	return "/rekit promote -Target " + quoteCandidateDecisionArg(result.CaseRoot) +
		" -PacketPath " + quoteCandidateDecisionArg(result.PacketPath) +
		" -CandidateDecisionPath " + quoteCandidateDecisionArg(result.DecisionPath) +
		" -VerifyCandidateDecision -FreshCaseRoot " + quoteCandidateDecisionArg(result.FreshCaseRoot) +
		" -AttachedCaseRoot " + quoteCandidateDecisionArg(result.AttachedCaseRoot) +
		" -Apply -Format json"
}

func finalizeCandidateDecisionReturn(result CandidateDecisionResult, err error) (CandidateDecisionResult, error) {
	if result.SchemaVersion == 0 {
		return result, err
	}
	result = finalizeCandidateDecisionResult(result)
	if applyErr, ok := err.(*CandidateDecisionApplyError); ok {
		applyErr.Result = result
	}
	return result, err
}

func candidatePromoteCommandWithTarget(command, caseRoot string) string {
	command = strings.TrimSpace(command)
	if command == "" || strings.Contains(command, " -Target ") || strings.TrimSpace(caseRoot) == "" {
		return command
	}
	for _, prefix := range []string{"/rekit promote ", "rekit promote "} {
		if rest, ok := strings.CutPrefix(command, prefix); ok {
			return prefix + "-Target " + quoteCandidateDecisionArg(caseRoot) + " " + rest
		}
	}
	return command
}
