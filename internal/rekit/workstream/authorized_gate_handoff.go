package workstream

import (
	"bytes"
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/gate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

type AuthorizedGateAdapterHandoff struct {
	EventID                     string                               `json:"eventId,omitempty"`
	Lane                        string                               `json:"lane,omitempty"`
	Subject                     string                               `json:"subject,omitempty"`
	Action                      string                               `json:"action,omitempty"`
	Target                      string                               `json:"target,omitempty"`
	Status                      string                               `json:"status,omitempty"`
	Risk                        string                               `json:"risk,omitempty"`
	Authorization               string                               `json:"authorization,omitempty"`
	Profile                     string                               `json:"profile,omitempty"`
	ReportContract              string                               `json:"reportContract,omitempty"`
	DefaultReportPath           string                               `json:"defaultReportPath,omitempty"`
	ReportPath                  string                               `json:"reportPath,omitempty"`
	ReportSummary               *gate.AdapterReportHandoffSummary    `json:"reportSummary,omitempty"`
	LiveValidation              *AuthorizedGateLiveValidationHandoff `json:"liveValidation,omitempty"`
	LiveValidationRepairHints   []gate.AdapterReportRepairHint       `json:"liveValidationRepairHints,omitempty"`
	LiveValidationNextSteps     []string                             `json:"liveValidationNextSteps,omitempty"`
	LiveValidationError         string                               `json:"liveValidationError,omitempty"`
	ReportContractError         string                               `json:"reportContractError,omitempty"`
	HandoffCommand              string                               `json:"handoffCommand,omitempty"`
	Acknowledged                bool                                 `json:"acknowledged,omitempty"`
	AcknowledgementState        string                               `json:"acknowledgementState,omitempty"`
	Boundary                    []string                             `json:"boundary,omitempty"`
	Evidence                    []string                             `json:"evidence,omitempty"`
	missionCommanderNextActions []mission.MissionCommanderNextActionItem
}

type AuthorizedGateLiveValidationHandoff struct {
	InvocationCwd                    string                                `json:"invocationCwd,omitempty"`
	AuthorizedWorkspaces             []string                              `json:"authorizedWorkspaces,omitempty"`
	ReportFileName                   string                                `json:"reportFileName,omitempty"`
	CaseRelativeReportPath           string                                `json:"caseRelativeReportPath,omitempty"`
	DispatchRequired                 bool                                  `json:"dispatchRequired,omitempty"`
	DispatchPresent                  bool                                  `json:"dispatchPresent,omitempty"`
	DispatchCurrent                  bool                                  `json:"dispatchCurrent,omitempty"`
	DispatchError                    string                                `json:"dispatchError,omitempty"`
	DispatchRequirementError         string                                `json:"dispatchRequirementError,omitempty"`
	DispatchCommand                  string                                `json:"dispatchCommand,omitempty"`
	CurrentRunLoopStepID             string                                `json:"currentRunLoopStepId,omitempty"`
	RunLoop                          []mission.MissionCommanderRunLoopStep `json:"runLoop,omitempty"`
	ValidateCommand                  string                                `json:"validateCommand,omitempty"`
	RecordCommand                    string                                `json:"recordCommand,omitempty"`
	ScaffoldCommand                  string                                `json:"scaffoldCommand,omitempty"`
	ScaffoldApplyCommand             string                                `json:"scaffoldApplyCommand,omitempty"`
	SidecarTemplateSHA256            string                                `json:"sidecarTemplateSha256,omitempty"`
	DraftCommand                     string                                `json:"draftCommand,omitempty"`
	DraftApplyCommand                string                                `json:"draftApplyCommand,omitempty"`
	DraftReportSHA256                string                                `json:"draftReportSha256,omitempty"`
	ReportSHA256                     string                                `json:"reportSha256,omitempty"`
	RecordExpectedReportSHA256       string                                `json:"recordExpectedReportSha256,omitempty"`
	ReceiptRequired                  bool                                  `json:"receiptRequired,omitempty"`
	ReceiptPresent                   bool                                  `json:"receiptPresent,omitempty"`
	ProvenanceValid                  bool                                  `json:"provenanceValid,omitempty"`
	AdapterExecutionDispatchID       string                                `json:"adapterExecutionDispatchId,omitempty"`
	AdapterExecutionDispatchPath     string                                `json:"adapterExecutionDispatchPath,omitempty"`
	AdapterExecutionDispatchSHA256   string                                `json:"adapterExecutionDispatchSha256,omitempty"`
	AdapterExecutionReceiptPath      string                                `json:"adapterExecutionReceiptPath,omitempty"`
	AdapterExecutionReceiptSHA256    string                                `json:"adapterExecutionReceiptSha256,omitempty"`
	ReceiptPreviewCommand            string                                `json:"receiptPreviewCommand,omitempty"`
	SupersedingGateEventID           string                                `json:"supersedingGateEventId,omitempty"`
	CurrentExecutor                  string                                `json:"currentExecutor,omitempty"`
	ExecutorGeneration               int                                   `json:"executorGeneration,omitempty"`
	AdapterHarness                   string                                `json:"adapterHarness,omitempty"`
	AdapterSession                   string                                `json:"adapterSession,omitempty"`
	ToolingCatalogSHA256             string                                `json:"toolingCatalogSha256,omitempty"`
	ArtifactCount                    int                                   `json:"artifactCount,omitempty"`
	CaseRelativeValidateCommand      string                                `json:"caseRelativeValidateCommand,omitempty"`
	CaseRelativeRecordCommand        string                                `json:"caseRelativeRecordCommand,omitempty"`
	CaseRelativeScaffoldCommand      string                                `json:"caseRelativeScaffoldCommand,omitempty"`
	CaseRelativeScaffoldApplyCommand string                                `json:"caseRelativeScaffoldApplyCommand,omitempty"`
	CaseRelativeDraftCommand         string                                `json:"caseRelativeDraftCommand,omitempty"`
	CaseRelativeDraftApplyCommand    string                                `json:"caseRelativeDraftApplyCommand,omitempty"`
	AdapterCandidateCount            int                                   `json:"adapterCandidateCount"`
	SelectedAdapterID                string                                `json:"selectedAdapterId,omitempty"`
	SelectedAdapter                  *gate.AdapterToolCandidate            `json:"selectedAdapter,omitempty"`
	SidecarTemplateAdapterID         string                                `json:"sidecarTemplateAdapterId,omitempty"`
	ReplayBehavior                   string                                `json:"replayBehavior,omitempty"`
	RunbookSteps                     []string                              `json:"runbookSteps,omitempty"`
}

func AuthorizedGateAdapterHandoffs(repoRoot, caseRoot, pack string, requests []map[string]any, laneID string) []AuthorizedGateAdapterHandoff {
	return AuthorizedGateAdapterHandoffsWithAcknowledgements(repoRoot, caseRoot, pack, requests, laneID, nil)
}

func AuthorizedGateAdapterHandoffsWithAcknowledgements(repoRoot, caseRoot, pack string, requests []map[string]any, laneID string, acknowledgedIDs map[string]bool) []AuthorizedGateAdapterHandoff {
	items := []map[string]any{}
	for _, item := range requests {
		if !mission.IsAuthorizedGateRequest(item) {
			continue
		}
		if strings.TrimSpace(laneID) != "" && mission.Value(item, "lane") != laneID {
			continue
		}
		items = append(items, item)
	}
	out := []AuthorizedGateAdapterHandoff{}
	for _, item := range items {
		handoff := authorizedGateAdapterHandoffFor(repoRoot, caseRoot, pack, item)
		if strings.TrimSpace(handoff.EventID) == "" && strings.TrimSpace(handoff.Subject) == "" {
			continue
		}
		applyAuthorizedGateAdapterAcknowledgement(&handoff, acknowledgedIDs)
		out = append(out, handoff)
	}
	return limitAuthorizedGateAdapterHandoffs(out, maxHandoffRows)
}

func limitAuthorizedGateAdapterHandoffs(items []AuthorizedGateAdapterHandoff, limit int) []AuthorizedGateAdapterHandoff {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	ranked := make([]struct {
		index    int
		priority int
	}, 0, len(items))
	for index, item := range items {
		ranked = append(ranked, struct {
			index    int
			priority int
		}{index: index, priority: authorizedGateAdapterHandoffPriority(item)})
	}
	slices.SortStableFunc(ranked, func(a, b struct {
		index    int
		priority int
	}) int {
		if priority := cmp.Compare(a.priority, b.priority); priority != 0 {
			return priority
		}
		return cmp.Compare(b.index, a.index)
	})
	keep := map[int]bool{}
	for _, item := range ranked[:limit] {
		keep[item.index] = true
	}
	out := []AuthorizedGateAdapterHandoff{}
	for index, item := range items {
		if keep[index] {
			out = append(out, item)
		}
	}
	return out
}

func authorizedGateAdapterHandoffPriority(item AuthorizedGateAdapterHandoff) int {
	if item.Acknowledged {
		return 90
	}
	priority := 100
	for _, action := range item.missionCommanderNextActions {
		priority = min(priority, authorizedGateAdapterActionPriority(action))
	}
	if priority < 100 {
		return priority
	}
	if item.ReportSummary == nil {
		return 80
	}
	if item.ReportSummary.RequiresMainEscalation {
		return 0
	}
	switch item.ReportSummary.State {
	case "repair-adapter-report":
		return 1
	case "ready-to-record-evidence":
		return 2
	case "needs-adapter-report-validation", "adapter-report-drafted-ready-for-validation", "adapter-report-scaffolded-awaiting-adapter-output":
		return 3
	case "ready-for-adapter-report-draft-apply", "ready-for-adapter-report-scaffold-apply":
		return 4
	case "evidence-already-recorded":
		return 70
	default:
		return 60
	}
}

func authorizedGateAdapterHandoffsForLane(repoRoot, caseRoot, pack, laneID string) []AuthorizedGateAdapterHandoff {
	facts, err := readHandoffFacts(caseRoot)
	if err != nil {
		return nil
	}
	return AuthorizedGateAdapterHandoffsWithAcknowledgements(repoRoot, caseRoot, pack, facts.Requests, laneID, ExecutionEvidenceReviewAcknowledgedIDs(facts))
}

func applyAuthorizedGateAdapterAcknowledgement(handoff *AuthorizedGateAdapterHandoff, acknowledgedIDs map[string]bool) {
	if handoff == nil || len(acknowledgedIDs) == 0 || !acknowledgedIDs[strings.TrimSpace(handoff.EventID)] {
		return
	}
	if handoff.ReportSummary == nil || handoff.ReportSummary.RequiresMainEscalation {
		return
	}
	recorded := handoff.ReportSummary.State == "evidence-already-recorded"
	superseded := handoff.ReportSummary.State == "repair-adapter-report" && handoff.LiveValidation != nil && handoff.LiveValidation.ReceiptPresent && strings.TrimSpace(handoff.LiveValidation.AdapterExecutionReceiptPath) != "" && strings.TrimSpace(handoff.LiveValidation.SupersedingGateEventID) != "" && handoff.LiveValidation.SupersedingGateEventID != handoff.EventID
	if !recorded && !superseded {
		return
	}
	handoff.Acknowledged = true
	handoff.AcknowledgementState = "execution-evidence-review-acknowledged"
	handoff.Evidence = append(handoff.Evidence, "execution evidence review acknowledged for gateEventId "+handoff.EventID)
	handoff.Boundary = append(handoff.Boundary, "acknowledged recorded adapter evidence is retained as provenance only; do not review, record, or replay it again")
	handoff.LiveValidationNextSteps = acknowledgedAdapterClosureSteps()
	handoff.missionCommanderNextActions = nil
	summary := *handoff.ReportSummary
	if superseded {
		summary.State = "evidence-already-recorded"
		summary.ReportPath = handoff.LiveValidation.CaseRelativeReportPath
		summary.ReportSHA256 = handoff.LiveValidation.ReportSHA256
		summary.Valid = true
		summary.RecordReady = false
		summary.RecordBlocked = true
		summary.RequiresValidation = false
		summary.RequiresRepair = false
		summary.ValidationFailureCode = ""
		summary.ValidationFailureStage = ""
		handoff.LiveValidationError = ""
		handoff.LiveValidationRepairHints = nil
	}
	summary.NextActionCount = 0
	summary.ReviewRequiredActionCount = 0
	summary.ActionQueueSummary = ""
	summary.CurrentAction = ""
	summary.Boundary = acknowledgedAdapterBoundary(summary.Boundary)
	handoff.ReportSummary = &summary
	if handoff.LiveValidation != nil {
		handoff.LiveValidation.RecordCommand = ""
		handoff.LiveValidation.CaseRelativeRecordCommand = ""
		handoff.LiveValidation.ReplayBehavior = "acknowledged recorded evidence is closed; do not record or replay the adapter report again"
		handoff.LiveValidation.RunbookSteps = acknowledgedAdapterClosureSteps()
	}
}

func acknowledgedAdapterBoundary(items []string) []string {
	out := []string{}
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" || acknowledgedAdapterCurrentReviewLine(trimmed) {
			continue
		}
		out = append(out, trimmed)
	}
	return append(out, "execution evidence review is acknowledged/closed; recorded report summary is provenance-only")
}

func acknowledgedAdapterClosureSteps() []string {
	return []string{
		"execution evidence review acknowledged/closed; recorded adapter report is provenance-only",
		"no review, record, or replay action remains for this adapter report snapshot",
	}
}

func acknowledgedAdapterCurrentReviewLine(line string) bool {
	line = strings.ToLower(strings.TrimSpace(line))
	return strings.Contains(line, "ready-for-evidence-review") ||
		strings.Contains(line, "review outputrefs/evidencerefs") ||
		strings.Contains(line, "review the recorded observation evidence")
}

func authorizedGateAdapterHandoffFor(repoRoot, caseRoot, pack string, item map[string]any) AuthorizedGateAdapterHandoff {
	gateEvent, _ := item["gate"].(map[string]any)
	authorization, _ := gateEvent["authorization"].(map[string]any)
	eventID := firstObjectText(item, "eventId")
	lane := firstObjectText(item, "lane")
	evidence := []string{}
	if eventID != "" {
		evidence = append(evidence, "authorized-gate ledger event "+eventID)
	}
	if outputs := firstObjectText(gateEvent, "outputPaths"); outputs != "" {
		evidence = append(evidence, "authorized outputPaths "+outputs)
	}
	if stops := firstObjectText(gateEvent, "stopConditions"); stops != "" {
		evidence = append(evidence, "authorized stopConditions "+stops)
	}
	handoff := AuthorizedGateAdapterHandoff{
		EventID:        eventID,
		Lane:           lane,
		Subject:        firstObjectText(item, "subject"),
		Action:         firstObjectText(gateEvent, "action"),
		Target:         firstObjectText(item, "target"),
		Status:         firstObjectText(item, "status"),
		Risk:           firstObjectText(item, "risk"),
		Authorization:  firstObjectText(authorization, "decision"),
		Profile:        firstObjectText(authorization, "profileId"),
		ReportContract: authorizedGateReportContractCommand(pack, eventID),
		HandoffCommand: "/rekit handoff " + mission.BoardLaneLabel(mission.BoardLane{ID: lane}),
		Boundary: []string{
			"authorized-gate adapter handoff is read-only; full gate -ExecutionReportContract remains the source of truth",
			"validate command is read-only and never writes observations",
			"record command writes bounded observation evidence only after validation returns valid=true; replace <executor-id> first",
			"projection may read and validate an existing canonical sidecar, but never records it; no heavy-tool replay and no authority/confirmed writes",
		},
		Evidence: evidence,
	}
	if eventID == "" {
		return handoff
	}
	contract, err := gate.AdapterReportContract(repoRoot, caseRoot, pack, gate.Options{GateEventID: eventID})
	if err != nil {
		handoff.ReportContractError = err.Error()
		return handoff
	}
	reportSummary := contract.ReportSummary
	handoff.DefaultReportPath = contract.DefaultReportPath
	handoff.ReportPath = firstText(reportSummary.ReportPath, contract.LiveValidation.CaseRelativeReportPath, contract.DefaultReportPath)
	liveValidation := authorizedGateLiveValidationHandoffFor(contract.LiveValidation)
	handoff.LiveValidation = &liveValidation
	handoff.missionCommanderNextActions = append([]mission.MissionCommanderNextActionItem{}, contract.MissionCommanderNextActions...)
	if validation, present, err := gate.AdapterReportLiveSnapshot(repoRoot, caseRoot, pack, gate.Options{GateEventID: eventID, ExecutionReportPath: handoff.ReportPath}); err != nil {
		handoff.LiveValidationError = err.Error()
		if receipt, receiptPath, receiptSHA, receiptPresent, receiptErr := gate.ReadAdapterExecutionReceipt(caseRoot, lane, eventID); receiptErr == nil && receiptPresent && receipt != nil {
			liveValidation.ReceiptRequired = true
			liveValidation.ReceiptPresent = true
			liveValidation.ProvenanceValid = false
			liveValidation.AdapterExecutionReceiptPath = receiptPath
			liveValidation.AdapterExecutionReceiptSHA256 = receiptSHA
			liveValidation.ReportSHA256 = receipt.Report.SHA256
			liveValidation.CurrentExecutor = receipt.Owner.CurrentExecutor
			liveValidation.ExecutorGeneration = receipt.Owner.ExecutorGeneration
			liveValidation.AdapterHarness = receipt.Owner.AdapterHarness
			liveValidation.AdapterSession = receipt.Owner.AdapterSession
			liveValidation.ToolingCatalogSHA256 = receipt.Adapter.ToolingCatalogSHA256
			liveValidation.ArtifactCount = len(receipt.Artifacts)
		}
	} else if present {
		reportSummary = validation.ReportSummary
		handoff.ReportPath = firstText(validation.ReportPath, handoff.ReportPath)
		handoff.LiveValidationRepairHints = append([]gate.AdapterReportRepairHint{}, validation.RepairHints...)
		handoff.LiveValidationNextSteps = append([]string{}, validation.NextSteps...)
		handoff.missionCommanderNextActions = append([]mission.MissionCommanderNextActionItem{}, validation.MissionCommanderNextActions...)
		liveValidation.RunbookSteps = append([]string{}, validation.RunbookSteps...)
		liveValidation.ReportSHA256 = validation.ReportSHA256
		liveValidation.RecordExpectedReportSHA256 = validation.RecordExpectedReportSHA256
		applyAdapterExecutionReceiptHandoff(&liveValidation, validation)
		if liveValidation.AdapterExecutionReceiptPath == "" {
			if receipt, receiptPath, receiptSHA, receiptPresent, receiptErr := gate.ReadAdapterExecutionReceipt(caseRoot, lane, eventID); receiptErr == nil && receiptPresent && receipt != nil {
				liveValidation.ReceiptRequired = true
				liveValidation.ReceiptPresent = true
				liveValidation.ProvenanceValid = false
				liveValidation.AdapterExecutionReceiptPath = receiptPath
				liveValidation.AdapterExecutionReceiptSHA256 = receiptSHA
				liveValidation.ReportSHA256 = receipt.Report.SHA256
				liveValidation.CurrentExecutor = receipt.Owner.CurrentExecutor
				liveValidation.ExecutorGeneration = receipt.Owner.ExecutorGeneration
				liveValidation.AdapterHarness = receipt.Owner.AdapterHarness
				liveValidation.AdapterSession = receipt.Owner.AdapterSession
				liveValidation.ToolingCatalogSHA256 = receipt.Adapter.ToolingCatalogSHA256
				liveValidation.ArtifactCount = len(receipt.Artifacts)
			}
		}
		if identity, identityPresent, identityErr := gate.ReadAdapterExecutionReportIdentity(caseRoot, handoff.ReportPath); identityErr == nil && identityPresent && identity != eventID {
			if authorized, authorizedErr := gate.IsAuthorizedAdapterReportAttempt(repoRoot, caseRoot, pack, identity, lane, handoff.Action, handoff.ReportPath); authorizedErr == nil && authorized {
				liveValidation.SupersedingGateEventID = identity
			}
		}
		recordCommand := ""
		if validation.Valid && reportSummary.RecordReady && !reportSummary.RecordBlocked {
			recordCommand = strings.TrimSpace(validation.MissionCommanderAction.PrimaryCommand)
		}
		liveValidation.RecordCommand = recordCommand
		liveValidation.CaseRelativeRecordCommand = recordCommand
		if validation.AdapterContext != nil && validation.AdapterContext.Selected != nil {
			selected := cloneAdapterToolCandidate(*validation.AdapterContext.Selected)
			liveValidation.SelectedAdapterID = selected.ID
			liveValidation.SelectedAdapter = &selected
		}
		if validation.Valid && reportSummary.RecordBlocked && !reportSummary.RecordReady {
			liveValidation.RecordCommand = ""
			liveValidation.CaseRelativeRecordCommand = ""
			liveValidation.ReplayBehavior = ""
		}
	}
	handoff.ReportSummary = &reportSummary
	return handoff
}

func applyAdapterExecutionReceiptHandoff(handoff *AuthorizedGateLiveValidationHandoff, validation gate.AdapterExecutionReportValidation) {
	if handoff == nil {
		return
	}
	handoff.ReceiptRequired = validation.ReceiptRequired
	handoff.ReceiptPresent = validation.ReceiptPresent
	handoff.ProvenanceValid = validation.ProvenanceValid
	handoff.AdapterExecutionDispatchPath = validation.AdapterExecutionDispatchPath
	handoff.AdapterExecutionDispatchSHA256 = validation.AdapterExecutionDispatchSHA256
	if validation.AdapterExecutionDispatch != nil {
		handoff.AdapterExecutionDispatchID = validation.AdapterExecutionDispatch.DispatchID
	}
	handoff.AdapterExecutionReceiptPath = validation.AdapterExecutionReceiptPath
	handoff.AdapterExecutionReceiptSHA256 = validation.AdapterExecutionReceiptSHA256
	handoff.ReceiptPreviewCommand = validation.ReceiptPreviewCommand
	if validation.AdapterExecution == nil {
		return
	}
	handoff.CurrentExecutor = validation.AdapterExecution.Owner.CurrentExecutor
	handoff.ExecutorGeneration = validation.AdapterExecution.Owner.ExecutorGeneration
	handoff.AdapterHarness = validation.AdapterExecution.Owner.AdapterHarness
	handoff.AdapterSession = validation.AdapterExecution.Owner.AdapterSession
	handoff.ToolingCatalogSHA256 = validation.AdapterExecution.Adapter.ToolingCatalogSHA256
	handoff.ArtifactCount = len(validation.AdapterExecution.Artifacts)
}

func authorizedGateLiveValidationHandoffFor(live gate.AdapterReportLiveValidation) AuthorizedGateLiveValidationHandoff {
	selectedAdapterID := ""
	var selectedAdapter *gate.AdapterToolCandidate
	if live.SelectedAdapter != nil {
		selectedAdapterID = live.SelectedAdapter.ID
		candidate := cloneAdapterToolCandidate(*live.SelectedAdapter)
		selectedAdapter = &candidate
	}
	return AuthorizedGateLiveValidationHandoff{
		InvocationCwd:                    live.InvocationCwd,
		AuthorizedWorkspaces:             append([]string{}, live.AuthorizedWorkspaces...),
		ReportFileName:                   live.ReportFileName,
		CaseRelativeReportPath:           live.CaseRelativeReportPath,
		DispatchRequired:                 live.DispatchRequired,
		DispatchPresent:                  live.DispatchPresent,
		DispatchCurrent:                  live.DispatchCurrent,
		DispatchError:                    live.DispatchError,
		DispatchRequirementError:         live.DispatchRequirementError,
		DispatchCommand:                  live.DispatchCommand,
		AdapterExecutionDispatchID:       live.AdapterExecutionDispatchID,
		AdapterExecutionDispatchPath:     live.AdapterExecutionDispatchPath,
		AdapterExecutionDispatchSHA256:   live.AdapterExecutionDispatchSHA256,
		CurrentRunLoopStepID:             live.CurrentRunLoopStepID,
		RunLoop:                          append([]mission.MissionCommanderRunLoopStep{}, live.RunLoop...),
		ValidateCommand:                  live.ValidateCommand,
		RecordCommand:                    live.RecordCommand,
		ScaffoldCommand:                  live.ScaffoldCommand,
		ScaffoldApplyCommand:             live.ScaffoldApplyCommand,
		SidecarTemplateSHA256:            live.SidecarTemplateSHA256,
		DraftCommand:                     live.DraftCommand,
		DraftApplyCommand:                live.DraftApplyCommand,
		DraftReportSHA256:                live.DraftReportSHA256,
		CaseRelativeValidateCommand:      live.CaseRelativeValidateCommand,
		CaseRelativeRecordCommand:        live.CaseRelativeRecordCommand,
		CaseRelativeScaffoldCommand:      live.CaseRelativeScaffoldCommand,
		CaseRelativeScaffoldApplyCommand: live.CaseRelativeScaffoldApplyCommand,
		CaseRelativeDraftCommand:         live.CaseRelativeDraftCommand,
		CaseRelativeDraftApplyCommand:    live.CaseRelativeDraftApplyCommand,
		AdapterCandidateCount:            len(live.AdapterCandidates),
		SelectedAdapterID:                selectedAdapterID,
		SelectedAdapter:                  selectedAdapter,
		SidecarTemplateAdapterID:         live.SidecarTemplate.AdapterID,
		ReplayBehavior:                   live.ReplayBehavior,
		RunbookSteps:                     append([]string{}, live.RunbookSteps...),
	}
}

func cloneAdapterToolCandidate(candidate gate.AdapterToolCandidate) gate.AdapterToolCandidate {
	candidate.SideEffects = append([]string{}, candidate.SideEffects...)
	candidate.GateActions = append([]string{}, candidate.GateActions...)
	candidate.ReportGuidance = append([]string{}, candidate.ReportGuidance...)
	candidate.EvidenceGuidance = append([]string{}, candidate.EvidenceGuidance...)
	candidate.StopConditionHints = append([]string{}, candidate.StopConditionHints...)
	return candidate
}

func authorizedGateCurrentRecordCommandMarkdown(command, reportSHA256 string) string {
	command = strings.TrimSpace(command)
	if command != "" && strings.TrimSpace(reportSHA256) != "" && strings.Contains(command, "-ExpectedExecutionReportSha256") {
		return command
	}
	return "after valid=true, use validation/status returned hash-bound record command with -ExpectedExecutionReportSha256"
}

func authorizedGateCurrentRecordCommandMarkdownForHandoff(item AuthorizedGateAdapterHandoff, command, reportSHA256 string) string {
	if item.Acknowledged {
		return "closed: execution evidence review acknowledged; no record action remains"
	}
	return authorizedGateCurrentRecordCommandMarkdown(command, reportSHA256)
}

func MissionCommanderNextActionsWithAuthorizedGateAdapters(base []mission.MissionCommanderNextActionItem, handoffs []AuthorizedGateAdapterHandoff) []mission.MissionCommanderNextActionItem {
	return MissionCommanderNextActionsWithAuthorizedGateAdaptersAndAcknowledgements(base, handoffs, nil)
}

func MissionCommanderNextActionsWithAuthorizedGateAdaptersAndAcknowledgements(base []mission.MissionCommanderNextActionItem, handoffs []AuthorizedGateAdapterHandoff, acknowledgedIDs map[string]bool) []mission.MissionCommanderNextActionItem {
	recordedGateEvents := map[string]bool{}
	evidenceNeedsMainReview := false
	evidenceActions := []mission.MissionCommanderNextActionItem{}
	previewActions := []mission.MissionCommanderNextActionItem{}
	laneActions := []mission.MissionCommanderNextActionItem{}
	for _, item := range base {
		if strings.HasPrefix(item.Source, "executionEvidenceReview") {
			evidenceActions = append(evidenceActions, item)
			gateEventID := firstText(item.GateEventID, item.Label)
			if gateEventID != "" {
				recordedGateEvents[gateEventID] = true
			}
			if item.Blocked {
				evidenceNeedsMainReview = true
			}
			continue
		}
		if item.Source == "missionCommanderActions" && (item.State == "needs-start-apply" || item.State == "needs-reconcile") {
			previewActions = append(previewActions, item)
			continue
		}
		laneActions = append(laneActions, item)
	}
	adapterActions := []mission.MissionCommanderNextActionItem{}
	supersededEvidence := map[string]bool{}
	for _, handoff := range handoffs {
		exactRecorded := handoff.ReportSummary != nil && (handoff.ReportSummary.State == "evidence-already-recorded" || handoff.ReportSummary.RequiresMainEscalation)
		if exactRecorded && acknowledgedIDs[strings.TrimSpace(handoff.EventID)] {
			continue
		}
		if recordedGateEvents[handoff.EventID] && exactRecorded {
			continue
		}
		liveSidecarSupersedesEvidence := handoff.ReportSummary != nil &&
			handoff.ReportSummary.ReportPresent &&
			!exactRecorded &&
			(handoff.ReportSummary.RequiresRepair || handoff.ReportSummary.RecordReady)
		if recordedGateEvents[handoff.EventID] && liveSidecarSupersedesEvidence {
			supersededEvidence[handoff.EventID] = true
		}
		for _, action := range handoff.missionCommanderNextActions {
			action.GateEventID = handoff.EventID
			if evidenceNeedsMainReview {
				action.Blocked = true
				action.Reasons = append(action.Reasons, "another execution evidence item requires main review before this adapter action")
				action.Boundary = append(action.Boundary, "do not execute this adapter action until main evidence review completes")
			}
			adapterActions = append(adapterActions, action)
		}
	}
	if len(adapterActions) == 0 {
		return mission.UniqueCommanderNextActions(base)
	}
	adapterActions = orderAuthorizedGateAdapterActions(adapterActions)
	items := []mission.MissionCommanderNextActionItem{}
	for _, item := range evidenceActions {
		if item.Source == "executionEvidenceReview" && item.State == "needs-main-escalation" && !supersededEvidence[firstText(item.GateEventID, item.Label)] {
			items = append(items, item)
		}
	}
	for _, item := range evidenceActions {
		if item.Source == "executionEvidenceReview" && item.State == "needs-main-escalation" {
			continue
		}
		if !supersededEvidence[firstText(item.GateEventID, item.Label)] {
			items = append(items, item)
		}
	}
	items = append(items, previewActions...)
	items = append(items, adapterActions...)
	items = append(items, laneActions...)
	return mission.UniqueCommanderNextActions(items)
}

func orderAuthorizedGateAdapterActions(items []mission.MissionCommanderNextActionItem) []mission.MissionCommanderNextActionItem {
	out := append([]mission.MissionCommanderNextActionItem{}, items...)
	slices.SortStableFunc(out, func(a, b mission.MissionCommanderNextActionItem) int {
		return cmp.Compare(authorizedGateAdapterActionPriority(a), authorizedGateAdapterActionPriority(b))
	})
	return out
}

func authorizedGateAdapterActionPriority(item mission.MissionCommanderNextActionItem) int {
	if item.Blocked {
		return 0
	}
	switch item.State {
	case "repair-adapter-report":
		return 1
	case "ready-to-record-evidence":
		return 2
	case "needs-adapter-report-validation", "adapter-report-drafted-ready-for-validation", "adapter-report-scaffolded-awaiting-adapter-output":
		return 3
	case "ready-for-adapter-report-draft-apply", "ready-for-adapter-report-scaffold-apply":
		return 4
	case "evidence-already-recorded":
		return 6
	default:
		return 5
	}
}

func authorizedGateReportContractCommand(pack, eventID string) string {
	if strings.TrimSpace(eventID) == "" {
		return ""
	}
	parts := []string{"/rekit", "gate"}
	if strings.TrimSpace(pack) != "" {
		parts = append(parts, "-Pack", strings.TrimSpace(pack))
	}
	parts = append(parts, "-GateEventId", eventID, "-ExecutionReportContract", "-Format", "json")
	for i, part := range parts {
		parts[i] = quoteCommandArg(part)
	}
	return strings.Join(parts, " ")
}

func WriteAuthorizedGateAdapterHandoffSection(out *bytes.Buffer, title string, items []AuthorizedGateAdapterHandoff) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintln(out, title)
	fmt.Fprintln(out)
	for _, item := range items {
		writeAuthorizedGateAdapterHandoffMarkdown(out, item)
	}
	fmt.Fprintln(out)
}

func writeAuthorizedGateAdapterHandoffMarkdown(out *bytes.Buffer, item AuthorizedGateAdapterHandoff) {
	state, reportSHA256, recordExpectedReportSHA256, reportPresent, valid, recordReady, recordBlocked, currentAction := "", "", "", false, false, false, false, ""
	allowedStatuses, allowedOutputs, authorizedStops, adapterCandidates := 0, 0, 0, 0
	if item.ReportSummary != nil {
		state = item.ReportSummary.State
		reportPresent = item.ReportSummary.ReportPresent
		valid = item.ReportSummary.Valid
		recordReady = item.ReportSummary.RecordReady
		recordBlocked = item.ReportSummary.RecordBlocked
		currentAction = item.ReportSummary.CurrentAction
		reportSHA256 = item.ReportSummary.ReportSHA256
		recordExpectedReportSHA256 = item.ReportSummary.RecordExpectedReportSHA256
		allowedStatuses = item.ReportSummary.AllowedStatusCount
		allowedOutputs = item.ReportSummary.AllowedOutputPathCount
		authorizedStops = item.ReportSummary.AuthorizedStopCount
		adapterCandidates = item.ReportSummary.AdapterCandidateCount
	}
	fmt.Fprintf(out, "- authorized gate adapter handoff: eventId=%s lane=%s action=%s state=%s reportPath=%s reportSha256=%s recordExpectedReportSha256=%s defaultReportPath=%s reportPresent=%t valid=%t recordReady=%t recordBlocked=%t currentAction=%s acknowledged=%t acknowledgementState=%s\n", item.EventID, item.Lane, item.Action, state, item.ReportPath, reportSHA256, recordExpectedReportSHA256, item.DefaultReportPath, reportPresent, valid, recordReady, recordBlocked, currentAction, item.Acknowledged, item.AcknowledgementState)
	if item.Acknowledged {
		fmt.Fprintf(out, "  - acknowledgement: state=%s boundary=recorded adapter report is provenance-only; no review or record action remains\n", item.AcknowledgementState)
	}
	fmt.Fprintf(out, "  - report contract: `%s`\n", item.ReportContract)
	fmt.Fprintf(out, "  - counts: allowedStatuses=%d allowedOutputPaths=%d authorizedStops=%d adapterCandidates=%d\n", allowedStatuses, allowedOutputs, authorizedStops, adapterCandidates)
	if live := item.LiveValidation; live != nil {
		fmt.Fprintf(out, "  - live validation: reportFileName=%s caseRelativeReportPath=%s adapterCandidates=%d selectedAdapter=%s sidecarAdapter=%s\n", live.ReportFileName, live.CaseRelativeReportPath, live.AdapterCandidateCount, live.SelectedAdapterID, live.SidecarTemplateAdapterID)
		if strings.TrimSpace(live.AdapterExecutionDispatchPath) != "" || strings.TrimSpace(live.AdapterExecutionDispatchSHA256) != "" {
			fmt.Fprintf(out, "  - dispatch: id=%s path=%s sha256=%s\n", live.AdapterExecutionDispatchID, live.AdapterExecutionDispatchPath, live.AdapterExecutionDispatchSHA256)
		}
		if live.ReceiptRequired || live.ReceiptPresent || strings.TrimSpace(live.AdapterExecutionReceiptPath) != "" {
			fmt.Fprintf(out, "  - receipt: required=%t present=%t provenanceValid=%t path=%s sha256=%s\n", live.ReceiptRequired, live.ReceiptPresent, live.ProvenanceValid, live.AdapterExecutionReceiptPath, live.AdapterExecutionReceiptSHA256)
		}
		if strings.TrimSpace(live.CurrentExecutor) != "" || live.ExecutorGeneration > 0 || strings.TrimSpace(live.AdapterHarness) != "" || strings.TrimSpace(live.AdapterSession) != "" {
			fmt.Fprintf(out, "  - execution owner: executor=%s generation=%d harness=%s session=%s\n", live.CurrentExecutor, live.ExecutorGeneration, live.AdapterHarness, live.AdapterSession)
		}
		if strings.TrimSpace(live.ToolingCatalogSHA256) != "" || live.ArtifactCount > 0 {
			fmt.Fprintf(out, "  - tooling provenance: catalogSha256=%s artifacts=%d\n", live.ToolingCatalogSHA256, live.ArtifactCount)
		}
		if strings.TrimSpace(live.CurrentRunLoopStepID) != "" || len(live.RunLoop) > 0 {
			fmt.Fprintf(out, "  - live run loop: currentRunLoopStep=%s steps=%d\n", live.CurrentRunLoopStepID, len(live.RunLoop))
			for _, step := range live.RunLoop {
				fmt.Fprintf(out, "  - live run loop step: order=%d step=%s actor=%s state=%s source=%s command=`%s` description=%s\n", step.Order, step.StepID, step.Actor, step.State, step.Source, step.Command, step.Description)
				for _, boundary := range step.Boundary {
					fmt.Fprintf(out, "  - live run loop boundary: step=%s boundary=%s\n", step.StepID, boundary)
				}
			}
		}
		if live.SelectedAdapter != nil {
			writeAuthorizedGateSelectedAdapterMarkdown(out, *live.SelectedAdapter)
		}
		fmt.Fprintf(out, "  - scaffold: `%s`\n", live.ScaffoldCommand)
		fmt.Fprintf(out, "  - scaffold apply: `%s`\n", live.ScaffoldApplyCommand)
		fmt.Fprintf(out, "  - sidecar template sha256: `%s`\n", live.SidecarTemplateSHA256)
		fmt.Fprintf(out, "  - draft: `%s`\n", live.DraftCommand)
		fmt.Fprintf(out, "  - draft apply: `%s`\n", live.DraftApplyCommand)
		fmt.Fprintf(out, "  - draft report sha256: `%s`\n", live.DraftReportSHA256)
		fmt.Fprintf(out, "  - report sha256: `%s`\n", live.ReportSHA256)
		fmt.Fprintf(out, "  - record expected report sha256: `%s`\n", live.RecordExpectedReportSHA256)
		fmt.Fprintf(out, "  - validate: `%s`\n", live.ValidateCommand)
		fmt.Fprintf(out, "  - record: `%s`\n", authorizedGateCurrentRecordCommandMarkdownForHandoff(item, live.RecordCommand, live.RecordExpectedReportSHA256))
		fmt.Fprintf(out, "  - case scaffold: `%s`\n", live.CaseRelativeScaffoldCommand)
		fmt.Fprintf(out, "  - case scaffold apply: `%s`\n", live.CaseRelativeScaffoldApplyCommand)
		fmt.Fprintf(out, "  - case draft: `%s`\n", live.CaseRelativeDraftCommand)
		fmt.Fprintf(out, "  - case draft apply: `%s`\n", live.CaseRelativeDraftApplyCommand)
		fmt.Fprintf(out, "  - case validate: `%s`\n", live.CaseRelativeValidateCommand)
		fmt.Fprintf(out, "  - case record: `%s`\n", authorizedGateCurrentRecordCommandMarkdownForHandoff(item, live.CaseRelativeRecordCommand, live.RecordExpectedReportSHA256))
		for _, workspace := range live.AuthorizedWorkspaces {
			fmt.Fprintf(out, "  - authorized workspace: `%s`\n", workspace)
		}
		if strings.TrimSpace(live.ReplayBehavior) != "" {
			fmt.Fprintf(out, "  - replay behavior: %s\n", live.ReplayBehavior)
		}
	}
	for _, hint := range item.LiveValidationRepairHints {
		fmt.Fprintf(out, "  - live validation repair: action=%s code=%s stage=%s recordBlocked=%t rerunValidation=%t detail=%s\n", hint.RepairAction, hint.Code, hint.Stage, hint.RecordBlocked, hint.RerunValidation, hint.Detail)
	}
	for _, step := range item.LiveValidationNextSteps {
		fmt.Fprintf(out, "  - live validation next step: %s\n", step)
	}
	if strings.TrimSpace(item.LiveValidationError) != "" {
		fmt.Fprintf(out, "  - live validation error: %s\n", item.LiveValidationError)
	}
	if strings.TrimSpace(item.ReportContractError) != "" {
		fmt.Fprintf(out, "  - report contract error: %s\n", item.ReportContractError)
	}
	for _, evidence := range item.Evidence {
		fmt.Fprintf(out, "  - evidence: %s\n", evidence)
	}
	for _, boundary := range item.Boundary {
		fmt.Fprintf(out, "  - boundary: %s\n", boundary)
	}
}

func writeAuthorizedGateSelectedAdapterMarkdown(out *bytes.Buffer, candidate gate.AdapterToolCandidate) {
	fmt.Fprintf(out, "  - selected adapter: id=%s status=%s entry=%s gateActions=%s recordOnlyAfterGate=%t toolingCatalogPath=%s\n", candidate.ID, candidate.Status, candidate.Entry, strings.Join(candidate.GateActions, ","), candidate.RecordOnlyAfterGate, candidate.ToolingCatalogPath)
	if strings.TrimSpace(candidate.Purpose) != "" {
		fmt.Fprintf(out, "  - selected adapter purpose: %s\n", candidate.Purpose)
	}
	if len(candidate.SideEffects) > 0 {
		fmt.Fprintf(out, "  - selected adapter side effects: %s\n", strings.Join(candidate.SideEffects, ","))
	}
	for _, guidance := range candidate.ReportGuidance {
		fmt.Fprintf(out, "  - selected adapter report guidance: %s\n", guidance)
	}
	for _, guidance := range candidate.EvidenceGuidance {
		fmt.Fprintf(out, "  - selected adapter evidence guidance: %s\n", guidance)
	}
	if len(candidate.StopConditionHints) > 0 {
		fmt.Fprintf(out, "  - selected adapter stop conditions: %s\n", strings.Join(candidate.StopConditionHints, ","))
	}
}

func AppendAuthorizedGateAdapterHandoffDigest(lines []string, label string, items []AuthorizedGateAdapterHandoff) []string {
	if len(items) == 0 {
		return lines
	}
	lines = append(lines, "", "## "+label, "")
	for _, item := range items {
		var out bytes.Buffer
		writeAuthorizedGateAdapterHandoffMarkdown(&out, item)
		for line := range strings.Lines(strings.TrimRight(out.String(), "\r\n")) {
			lines = append(lines, strings.TrimRight(line, "\r\n"))
		}
	}
	return lines
}
