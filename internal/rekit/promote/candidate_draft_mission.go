package promote

import (
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

const candidateDraftRefreshStatusCommand = "/rekit status -Format json"

func finalizeCandidateDecisionDraftResult(result CandidateDecisionDraftResult) CandidateDecisionDraftResult {
	state, command, requiresReview := candidateDraftDriver(result.Applied, result.AlreadyWritten, result.Mode, result.ApplyCommand, "pack-memory-decision")
	result.MissionCommanderAction, result.MissionCommanderActionQueue = candidateDraftMissionCommanderActionQueue(
		result.Pack,
		state,
		"packMemoryCandidateDecisionDraft",
		"pack-memory-decision-draft:"+shortHash(result.DecisionPath),
		command,
		requiresReview,
		result.NextSteps,
		result.Boundary,
	)
	return result
}

func finalizeCandidateReviewProofDraftResult(result CandidateReviewProofDraftResult) CandidateReviewProofDraftResult {
	state, command, requiresReview := candidateDraftDriver(result.Applied, result.AlreadyWritten, result.Mode, result.ApplyCommand, "pack-memory-proof")
	result.MissionCommanderAction, result.MissionCommanderActionQueue = candidateDraftMissionCommanderActionQueue(
		result.Pack,
		state,
		"packMemoryCandidateProofDraft",
		"pack-memory-proof-draft:"+result.ProofType+":"+shortHash(result.ProofPath),
		command,
		requiresReview,
		result.NextSteps,
		result.Boundary,
	)
	return result
}

func finalizeCandidateLifecycleProofDraftResult(result CandidateLifecycleProofDraftResult) CandidateLifecycleProofDraftResult {
	state, command, requiresReview := candidateDraftDriver(result.Applied, result.AlreadyWritten, result.Mode, result.ApplyCommand, "pack-memory-lifecycle-proof")
	result.MissionCommanderAction, result.MissionCommanderActionQueue = candidateDraftMissionCommanderActionQueue(
		result.Pack,
		state,
		"packMemoryCandidateLifecycleProofDraft",
		"pack-memory-lifecycle-proof-draft:"+result.ProofType+":"+shortHash(result.ProofPath),
		command,
		requiresReview,
		result.NextSteps,
		result.Boundary,
	)
	return result
}

func candidateDraftDriver(applied, alreadyWritten bool, mode, applyCommand, prefix string) (state, command string, requiresReview bool) {
	if !applied && !candidateDraftAlreadyExists(alreadyWritten, mode, applyCommand) {
		return "ready-for-" + prefix + "-draft-apply", strings.TrimSpace(applyCommand), true
	}
	if candidateDraftAlreadyExists(alreadyWritten, mode, applyCommand) {
		return prefix + "-already-drafted-refresh-required", candidateDraftRefreshStatusCommand, false
	}
	return prefix + "-drafted-refresh-required", candidateDraftRefreshStatusCommand, false
}

func candidateDraftAlreadyExists(alreadyWritten bool, mode, applyCommand string) bool {
	return alreadyWritten || strings.Contains(strings.TrimSpace(mode), "already-drafted") || strings.TrimSpace(applyCommand) == ""
}

func candidateDraftMissionCommanderActionQueue(pack, state, source, actionID, command string, requiresReview bool, reasons, boundary []string) (mission.MissionCommanderAction, mission.MissionCommanderActionQueue) {
	command = strings.TrimSpace(command)
	if command == "" {
		command = candidateDraftRefreshStatusCommand
		requiresReview = false
	}
	boundary = mission.UniqueStrings(append([]string{}, boundary...))
	action := mission.MissionCommanderAction{
		State:          state,
		Prompt:         "Follow the returned pack-memory candidate driver request, then refresh status or release-check before selecting follow-up work.",
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
	return action, queue
}
