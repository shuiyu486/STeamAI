package workstream

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/gate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

type AuthorizedGateAdapterHandoff struct {
	EventID                   string                               `json:"eventId,omitempty"`
	Lane                      string                               `json:"lane,omitempty"`
	Subject                   string                               `json:"subject,omitempty"`
	Action                    string                               `json:"action,omitempty"`
	Target                    string                               `json:"target,omitempty"`
	Status                    string                               `json:"status,omitempty"`
	Risk                      string                               `json:"risk,omitempty"`
	Authorization             string                               `json:"authorization,omitempty"`
	Profile                   string                               `json:"profile,omitempty"`
	ReportContract            string                               `json:"reportContract,omitempty"`
	DefaultReportPath         string                               `json:"defaultReportPath,omitempty"`
	ReportPath                string                               `json:"reportPath,omitempty"`
	ReportSummary             *gate.AdapterReportHandoffSummary    `json:"reportSummary,omitempty"`
	LiveValidation            *AuthorizedGateLiveValidationHandoff `json:"liveValidation,omitempty"`
	LiveValidationRepairHints []gate.AdapterReportRepairHint       `json:"liveValidationRepairHints,omitempty"`
	LiveValidationNextSteps   []string                             `json:"liveValidationNextSteps,omitempty"`
	LiveValidationError       string                               `json:"liveValidationError,omitempty"`
	ReportContractError       string                               `json:"reportContractError,omitempty"`
	HandoffCommand            string                               `json:"handoffCommand,omitempty"`
	Boundary                  []string                             `json:"boundary,omitempty"`
	Evidence                  []string                             `json:"evidence,omitempty"`
}

type AuthorizedGateLiveValidationHandoff struct {
	InvocationCwd               string                     `json:"invocationCwd,omitempty"`
	AuthorizedWorkspaces        []string                   `json:"authorizedWorkspaces,omitempty"`
	ReportFileName              string                     `json:"reportFileName,omitempty"`
	CaseRelativeReportPath      string                     `json:"caseRelativeReportPath,omitempty"`
	ValidateCommand             string                     `json:"validateCommand,omitempty"`
	RecordCommand               string                     `json:"recordCommand,omitempty"`
	CaseRelativeValidateCommand string                     `json:"caseRelativeValidateCommand,omitempty"`
	CaseRelativeRecordCommand   string                     `json:"caseRelativeRecordCommand,omitempty"`
	AdapterCandidateCount       int                        `json:"adapterCandidateCount"`
	SelectedAdapterID           string                     `json:"selectedAdapterId,omitempty"`
	SelectedAdapter             *gate.AdapterToolCandidate `json:"selectedAdapter,omitempty"`
	SidecarTemplateAdapterID    string                     `json:"sidecarTemplateAdapterId,omitempty"`
	ReplayBehavior              string                     `json:"replayBehavior,omitempty"`
}

func AuthorizedGateAdapterHandoffs(repoRoot, caseRoot, pack string, requests []map[string]any, laneID string) []AuthorizedGateAdapterHandoff {
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
	for _, item := range lastObjects(items, maxHandoffRows) {
		handoff := authorizedGateAdapterHandoffFor(repoRoot, caseRoot, pack, item)
		if strings.TrimSpace(handoff.EventID) == "" && strings.TrimSpace(handoff.Subject) == "" {
			continue
		}
		out = append(out, handoff)
	}
	return out
}

func authorizedGateAdapterHandoffsForLane(repoRoot, caseRoot, pack, laneID string) []AuthorizedGateAdapterHandoff {
	facts, err := readHandoffFacts(caseRoot)
	if err != nil {
		return nil
	}
	return AuthorizedGateAdapterHandoffs(repoRoot, caseRoot, pack, facts.Requests, laneID)
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
	if validation, present, err := gate.AdapterReportLiveSnapshot(repoRoot, caseRoot, pack, gate.Options{GateEventID: eventID, ExecutionReportPath: handoff.ReportPath}); err != nil {
		handoff.LiveValidationError = err.Error()
	} else if present {
		reportSummary = validation.ReportSummary
		handoff.ReportPath = firstText(validation.ReportPath, handoff.ReportPath)
		handoff.LiveValidationRepairHints = append([]gate.AdapterReportRepairHint{}, validation.RepairHints...)
		handoff.LiveValidationNextSteps = append([]string{}, validation.NextSteps...)
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

func authorizedGateLiveValidationHandoffFor(live gate.AdapterReportLiveValidation) AuthorizedGateLiveValidationHandoff {
	selectedAdapterID := ""
	var selectedAdapter *gate.AdapterToolCandidate
	if live.SelectedAdapter != nil {
		selectedAdapterID = live.SelectedAdapter.ID
		candidate := cloneAdapterToolCandidate(*live.SelectedAdapter)
		selectedAdapter = &candidate
	}
	return AuthorizedGateLiveValidationHandoff{
		InvocationCwd:               live.InvocationCwd,
		AuthorizedWorkspaces:        append([]string{}, live.AuthorizedWorkspaces...),
		ReportFileName:              live.ReportFileName,
		CaseRelativeReportPath:      live.CaseRelativeReportPath,
		ValidateCommand:             live.ValidateCommand,
		RecordCommand:               live.RecordCommand,
		CaseRelativeValidateCommand: live.CaseRelativeValidateCommand,
		CaseRelativeRecordCommand:   live.CaseRelativeRecordCommand,
		AdapterCandidateCount:       len(live.AdapterCandidates),
		SelectedAdapterID:           selectedAdapterID,
		SelectedAdapter:             selectedAdapter,
		SidecarTemplateAdapterID:    live.SidecarTemplate.AdapterID,
		ReplayBehavior:              live.ReplayBehavior,
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
	state, reportPresent, valid, recordReady, recordBlocked, currentAction := "", false, false, false, false, ""
	allowedStatuses, allowedOutputs, authorizedStops, adapterCandidates := 0, 0, 0, 0
	if item.ReportSummary != nil {
		state = item.ReportSummary.State
		reportPresent = item.ReportSummary.ReportPresent
		valid = item.ReportSummary.Valid
		recordReady = item.ReportSummary.RecordReady
		recordBlocked = item.ReportSummary.RecordBlocked
		currentAction = item.ReportSummary.CurrentAction
		allowedStatuses = item.ReportSummary.AllowedStatusCount
		allowedOutputs = item.ReportSummary.AllowedOutputPathCount
		authorizedStops = item.ReportSummary.AuthorizedStopCount
		adapterCandidates = item.ReportSummary.AdapterCandidateCount
	}
	fmt.Fprintf(out, "- authorized gate adapter handoff: eventId=%s lane=%s action=%s state=%s reportPath=%s defaultReportPath=%s reportPresent=%t valid=%t recordReady=%t recordBlocked=%t currentAction=%s\n", item.EventID, item.Lane, item.Action, state, item.ReportPath, item.DefaultReportPath, reportPresent, valid, recordReady, recordBlocked, currentAction)
	fmt.Fprintf(out, "  - report contract: `%s`\n", item.ReportContract)
	fmt.Fprintf(out, "  - counts: allowedStatuses=%d allowedOutputPaths=%d authorizedStops=%d adapterCandidates=%d\n", allowedStatuses, allowedOutputs, authorizedStops, adapterCandidates)
	if live := item.LiveValidation; live != nil {
		fmt.Fprintf(out, "  - live validation: reportFileName=%s caseRelativeReportPath=%s adapterCandidates=%d selectedAdapter=%s sidecarAdapter=%s\n", live.ReportFileName, live.CaseRelativeReportPath, live.AdapterCandidateCount, live.SelectedAdapterID, live.SidecarTemplateAdapterID)
		if live.SelectedAdapter != nil {
			writeAuthorizedGateSelectedAdapterMarkdown(out, *live.SelectedAdapter)
		}
		fmt.Fprintf(out, "  - validate: `%s`\n", live.ValidateCommand)
		fmt.Fprintf(out, "  - record: `%s`\n", live.RecordCommand)
		fmt.Fprintf(out, "  - case validate: `%s`\n", live.CaseRelativeValidateCommand)
		fmt.Fprintf(out, "  - case record: `%s`\n", live.CaseRelativeRecordCommand)
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
