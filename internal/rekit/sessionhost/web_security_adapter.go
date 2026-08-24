package sessionhost

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/adapterexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/adapterhost"
	"github.com/shuiyu486/re-context-kits/internal/rekit/capabilitycontract"
	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	"github.com/shuiyu486/re-context-kits/internal/rekit/gate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instructionpacket"
	"github.com/shuiyu486/re-context-kits/internal/rekit/lanemutation"
	"github.com/shuiyu486/re-context-kits/internal/rekit/laneowner"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/websecurity"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

const (
	webSecurityPack = "web-security"

	webSecurityAdapterLifecycleKind       = "web-security-adapter-lifecycle"
	webSecurityAdapterExecutionIntentKind = "web-security-adapter-execution-intent"
	webSecurityAdapterExecutionResultKind = "web-security-adapter-execution-result"

	webSecurityOpenAPIReviewInputKind    = "web-security-openapi-inventory-evidence-review"
	webSecurityReplayReviewInputKind     = "web-security-bounded-replay-evidence-review"
	webSecurityOpenAPIReviewIntentKind   = "web-security-openapi-inventory-evidence-review-intent"
	webSecurityOpenAPIReviewDecisionKind = "web-security-openapi-inventory-evidence-review-decision"
	webSecurityOpenAPIReviewClosureKind  = "web-security-openapi-inventory-evidence-review-closure"
	webSecurityReplayReviewIntentKind    = "web-security-bounded-replay-evidence-review-intent"
	webSecurityReplayReviewDecisionKind  = "web-security-bounded-replay-evidence-review-decision"
	webSecurityReplayReviewClosureKind   = "web-security-bounded-replay-evidence-review-closure"
	webSecurityOpenAPIReviewInputRole    = "mission-commander-web-security-openapi-inventory-evidence-review-input"
	webSecurityReplayReviewInputRole     = "mission-commander-web-security-bounded-replay-evidence-review-input"
	webSecurityOpenAPIMemberBindingKind  = "web-security-openapi-inventory-evidence"
	webSecurityReplayMemberBindingKind   = "web-security-bounded-replay-evidence"
	webSecurityAdapterActor              = "mission-commander"
	webSecurityAdapterProcessTimeout     = 30 * time.Second
)

var webSecurityEvidenceReviewRunClaude = runClaude

type WebSecurityAdapterLifecycleResult struct {
	SchemaVersion                int                                 `json:"schemaVersion"`
	Kind                         string                              `json:"kind"`
	State                        string                              `json:"state"`
	GateEventID                  string                              `json:"gateEventId"`
	AdapterID                    string                              `json:"adapterId"`
	InputPath                    string                              `json:"inputPath"`
	InputSHA256                  string                              `json:"inputSha256"`
	ExecutionIntentPath          string                              `json:"executionIntentPath"`
	ExecutionIntentSHA256        string                              `json:"executionIntentSha256"`
	ExecutionResultPath          string                              `json:"executionResultPath,omitempty"`
	ExecutionResultSHA256        string                              `json:"executionResultSha256,omitempty"`
	ExecutionStatus              string                              `json:"executionStatus,omitempty"`
	ExecutionExitStatus          string                              `json:"executionExitStatus,omitempty"`
	AdapterProcessID             int                                 `json:"adapterProcessId,omitempty"`
	ChildLaunched                bool                                `json:"childLaunched"`
	AdapterReplay                bool                                `json:"adapterReplay"`
	Run                          *adapterhost.AuthorizedRunResult    `json:"run,omitempty"`
	EvidenceReviewInputPath      string                              `json:"evidenceReviewInputPath,omitempty"`
	EvidenceReviewInputSHA256    string                              `json:"evidenceReviewInputSha256,omitempty"`
	EvidenceReviewIntentPath     string                              `json:"evidenceReviewIntentPath,omitempty"`
	EvidenceReviewIntentSHA256   string                              `json:"evidenceReviewIntentSha256,omitempty"`
	EvidenceReviewDecisionPath   string                              `json:"evidenceReviewDecisionPath,omitempty"`
	EvidenceReviewDecisionSHA256 string                              `json:"evidenceReviewDecisionSha256,omitempty"`
	EvidenceReviewSession        string                              `json:"evidenceReviewSession,omitempty"`
	EvidenceReviewDecision       string                              `json:"evidenceReviewDecision,omitempty"`
	EvidenceReviewReplay         bool                                `json:"evidenceReviewReplay"`
	ResultPublication            *executioncontrol.ResultPublication `json:"resultPublication,omitempty"`
	AcknowledgementEventID       string                              `json:"acknowledgementEventId,omitempty"`
	AcknowledgementSHA256        string                              `json:"acknowledgementSha256,omitempty"`
	ClosurePath                  string                              `json:"closurePath,omitempty"`
	ClosureSHA256                string                              `json:"closureSha256,omitempty"`
	TaskBindingPath              string                              `json:"taskBindingPath,omitempty"`
	TaskBindingSHA256            string                              `json:"taskBindingSha256,omitempty"`
	SelectedEvidenceRef          string                              `json:"selectedEvidenceRef,omitempty"`
	ReadyForMember               bool                                `json:"readyForMember"`
	NoAuthority                  bool                                `json:"noAuthorityOrConfirmed"`
	NoHeavyToolAfterExecution    bool                                `json:"noHeavyToolAfterExecution"`
}

type webSecurityAdapterSelection struct {
	Handoff      workstream.AuthorizedGateAdapterHandoff
	AdapterID    string
	Input        websecurity.FileBinding
	Replay       websecurity.ReplayRequest
	CreatedAt    string
	Acknowledged bool
}

type webSecurityAdapterExecutionIntent binaryREAdapterExecutionIntent

type webSecurityAdapterExecutionResult binaryREAdapterExecutionResult

type webSecurityOpenAPIReviewInput struct {
	SchemaVersion       int                             `json:"schemaVersion"`
	Kind                string                          `json:"kind"`
	AdapterID           string                          `json:"adapterId"`
	GateEventID         string                          `json:"gateEventId"`
	ObservationEventID  string                          `json:"observationEventId"`
	ObservationPath     string                          `json:"observationPath"`
	ObservationSHA256   string                          `json:"observationSha256"`
	ProfileSHA256       string                          `json:"profileSha256"`
	Source              websecurity.FileBinding         `json:"source"`
	InventoryPath       string                          `json:"inventoryPath"`
	InventorySHA256     string                          `json:"inventorySha256"`
	ReportPath          string                          `json:"reportPath"`
	ReportSHA256        string                          `json:"reportSha256"`
	DispatchPath        string                          `json:"dispatchPath"`
	DispatchSHA256      string                          `json:"dispatchSha256"`
	ReceiptPath         string                          `json:"receiptPath"`
	ReceiptSHA256       string                          `json:"receiptSha256"`
	OpenAPIVersion      string                          `json:"openapiVersion"`
	Servers             []websecurity.Server            `json:"servers"`
	AuthSchemes         []websecurity.AuthScheme        `json:"authSchemes"`
	EndpointCount       int                             `json:"endpointCount"`
	WarningCount        int                             `json:"warningCount"`
	Boundaries          websecurity.InventoryBoundaries `json:"boundaries"`
	EvidenceRefs        []string                        `json:"evidenceRefs"`
	SelectedEvidenceRef string                          `json:"selectedEvidenceRef"`
	NoAuthority         bool                            `json:"noAuthorityOrConfirmed"`
	NoHeavyTool         bool                            `json:"noHeavyTool"`
}

type webSecurityReplayReviewInput struct {
	SchemaVersion       int                          `json:"schemaVersion"`
	Kind                string                       `json:"kind"`
	AdapterID           string                       `json:"adapterId"`
	GateEventID         string                       `json:"gateEventId"`
	ObservationEventID  string                       `json:"observationEventId"`
	ObservationPath     string                       `json:"observationPath"`
	ObservationSHA256   string                       `json:"observationSha256"`
	ProfileSHA256       string                       `json:"profileSha256"`
	Request             websecurity.FileBinding      `json:"request"`
	Inventory           websecurity.FileBinding      `json:"inventory"`
	ResultPath          string                       `json:"resultPath"`
	ResultSHA256        string                       `json:"resultSha256"`
	ReportPath          string                       `json:"reportPath"`
	ReportSHA256        string                       `json:"reportSha256"`
	DispatchPath        string                       `json:"dispatchPath"`
	DispatchSHA256      string                       `json:"dispatchSha256"`
	ReceiptPath         string                       `json:"receiptPath"`
	ReceiptSHA256       string                       `json:"receiptSha256"`
	Target              websecurity.ReplayTarget     `json:"target"`
	Operation           websecurity.ReplayOperation  `json:"operation"`
	Status              string                       `json:"status"`
	Delivery            websecurity.ReplayDelivery   `json:"delivery"`
	Actual              *websecurity.ActualResponse  `json:"actual,omitempty"`
	Diff                *websecurity.ReplayDiff      `json:"diff,omitempty"`
	Limits              websecurity.ReplayLimits     `json:"limits"`
	Boundaries          websecurity.ReplayBoundaries `json:"boundaries"`
	EvidenceRefs        []string                     `json:"evidenceRefs"`
	SelectedEvidenceRef string                       `json:"selectedEvidenceRef"`
	NoAuthority         bool                         `json:"noAuthorityOrConfirmed"`
	NoHeavyTool         bool                         `json:"noHeavyTool"`
}

func prepareProductionAdapterBeforeMember(parent context.Context, opt DailyOptions, result *DailyResult) (bool, error) {
	if result == nil {
		return false, fmt.Errorf("daily result is missing before production adapter preparation")
	}
	switch strings.TrimSpace(result.Pack) {
	case webSecurityPack:
		return prepareWebSecurityAdapterBeforeMember(parent, opt, result)
	default:
		return prepareBinaryREAdapterBeforeMember(parent, opt, result)
	}
}

func prepareWebSecurityAdapterBeforeMember(parent context.Context, opt DailyOptions, result *DailyResult) (bool, error) {
	lifecycle, found, err := runWebSecurityAdapterLifecycle(parent, opt, result.CaseRoot, result.Pack, result.Lane)
	if err != nil {
		return false, fmt.Errorf("prepare ordinary web-security adapter lifecycle: %w", err)
	}
	if !found {
		return true, nil
	}
	result.WebSecurityAdapter = &lifecycle
	if lifecycle.ReadyForMember {
		return true, nil
	}
	result.FinalState = lifecycle.State
	result.Blocked = true
	result.Replay = lifecycle.AdapterReplay || lifecycle.EvidenceReviewReplay
	return false, nil
}

func runWebSecurityAdapterLifecycle(
	parent context.Context,
	dailyOpt DailyOptions,
	caseRoot,
	pack,
	lane string,
) (WebSecurityAdapterLifecycleResult, bool, error) {
	if strings.TrimSpace(pack) != webSecurityPack {
		return WebSecurityAdapterLifecycleResult{}, false, nil
	}
	repoRoot, err := runtimeContextForDailyPack(caseRoot, pack)
	if err != nil {
		return WebSecurityAdapterLifecycleResult{}, false, err
	}
	selection, found, err := discoverWebSecurityAdapterSelection(repoRoot, caseRoot, pack, lane)
	if err != nil || !found {
		return WebSecurityAdapterLifecycleResult{}, found, err
	}
	instructionIdentity, err := currentProductionInstructionIdentity(caseRoot, pack)
	if err != nil {
		return WebSecurityAdapterLifecycleResult{}, true, err
	}
	intent, intentPath, intentSHA, err := ensureWebSecurityAdapterExecutionIntent(caseRoot, pack, lane, selection, instructionIdentity)
	if err != nil {
		return WebSecurityAdapterLifecycleResult{}, true, err
	}
	result := WebSecurityAdapterLifecycleResult{
		SchemaVersion: 1, Kind: webSecurityAdapterLifecycleKind, State: "execution-ready",
		GateEventID: selection.Handoff.EventID, AdapterID: selection.AdapterID,
		InputPath: selection.Input.Path, InputSHA256: selection.Input.SHA256,
		ExecutionIntentPath: intentPath, ExecutionIntentSHA256: intentSHA,
		NoAuthority: true, NoHeavyToolAfterExecution: true,
	}

	executionResultPath, err := binaryREAdapterArtifactPath(caseRoot, lane, intent.GateEventID, "execution-result.json")
	if err != nil {
		return result, true, err
	}
	storedExecution, storedData, executionReplay, err := readBinaryREAdapterArtifact[webSecurityAdapterExecutionResult](
		caseRoot, executionResultPath, "web-security adapter execution result",
	)
	if err != nil {
		return result, true, err
	}
	var run adapterhost.AuthorizedRunResult
	if executionReplay {
		if err := validateWebSecurityAdapterExecutionResult(storedExecution, caseRoot, intent, intentPath, intentSHA); err != nil {
			return result, true, err
		}
		if err := requireBinaryREControlCurrent(caseRoot, storedExecution.Control); err != nil {
			return result, true, err
		}
		run = storedExecution.Run
		result.ExecutionResultPath = executionResultPath
		result.ExecutionResultSHA256 = bytesSHA256(storedData)
		result.AdapterReplay = true
	} else {
		adapterPath := strings.TrimSpace(dailyOpt.webSecurityAdapterPath)
		if adapterPath == "" {
			adapterPath, err = os.Executable()
			if err != nil {
				return result, true, err
			}
		}
		runner := dailyOpt.webSecurityAdapterRunner
		if runner == nil {
			runner = adapterhost.RunAuthorizedGateProcess
		}
		processID := 0
		run, processID, err = runner(adapterPath, adapterhost.AuthorizedRunOptions{
			RepoRoot: repoRoot, CaseRoot: caseRoot, Pack: pack,
			GateEventID: selection.Handoff.EventID, ExecutionReportPath: intent.ReportPath,
			AdapterID: selection.AdapterID, AdapterSession: intent.AdapterSession, Actor: intent.Actor,
			DeferSuccessfulTaskBinding: true,
			ExecutionControlBinding:    executioncontrol.CloneBinding(&intent.Control),
			InstructionIdentity:        cloneProductionInstructionIdentityPointer(&intent.InstructionIdentity),
		}, webSecurityAdapterProcessTimeout)
		result.AdapterProcessID = processID
		result.ChildLaunched = run.ChildLaunched
		result.AdapterReplay = run.Replay
		if err != nil {
			return result, true, err
		}
		if err := validateWebSecurityAuthorizedRun(caseRoot, intent, run); err != nil {
			return result, true, err
		}
		storedExecution = webSecurityAdapterExecutionResult{
			SchemaVersion: 1, Kind: webSecurityAdapterExecutionResultKind,
			GateEventID: intent.GateEventID, ExecutionIntentPath: intentPath,
			ExecutionIntentSHA256: intentSHA, Control: intent.Control,
			InstructionIdentity: cloneProductionInstructionIdentity(intent.InstructionIdentity), Run: run,
			NoAuthority: true, NoHeavyToolAfterExecution: true,
		}
		executionSHA, replay, persistErr := persistWebSecurityAdapterExecutionResult(
			caseRoot, executionResultPath, storedExecution, intent, intentPath, intentSHA,
		)
		if persistErr != nil {
			return result, true, persistErr
		}
		result.ExecutionResultPath = executionResultPath
		result.ExecutionResultSHA256 = executionSHA
		result.AdapterReplay = result.AdapterReplay || replay
	}
	result.ExecutionStatus = run.ExecutionStatus
	result.ExecutionExitStatus = run.ExecutionExitStatus
	runCopy := run
	result.Run = &runCopy
	if run.PacketPath == "" {
		result.State = "adapter-execution-" + run.ExecutionStatus
		return result, true, nil
	}
	if err := requireBinaryREControlCurrent(caseRoot, intent.Control); err != nil {
		return result, true, err
	}

	item, observationPath, observationSHA, err := ensureBinaryREObservationSnapshot(caseRoot, lane, run)
	if err != nil {
		return result, true, err
	}
	inputPath, err := binaryREAdapterArtifactPath(caseRoot, lane, intent.GateEventID, "evidence-review", "input.json")
	if err != nil {
		return result, true, err
	}
	inputSHA := ""
	binding := binaryREEvidenceReviewBinding{}
	var inventoryInput *webSecurityOpenAPIReviewInput
	var replayInput *webSecurityReplayReviewInput
	if intent.AdapterID == websecurity.InventoryAdapterID {
		input, inspectErr := inspectWebSecurityOpenAPIEvidence(caseRoot, lane, item, selection, run, observationPath, observationSHA)
		if inspectErr != nil {
			return result, true, inspectErr
		}
		inputSHA, err = writeBinaryREAdapterArtifact(caseRoot, inputPath, "web-security OpenAPI evidence review input", input)
		binding = webSecurityOpenAPIReviewBinding(input)
		inventoryInput = &input
	} else {
		input, inspectErr := inspectWebSecurityReplayEvidence(caseRoot, lane, item, selection, run, observationPath, observationSHA)
		if inspectErr != nil {
			return result, true, inspectErr
		}
		inputSHA, err = writeBinaryREAdapterArtifact(caseRoot, inputPath, "web-security bounded replay evidence review input", input)
		binding = webSecurityReplayReviewBinding(input)
		replayInput = &input
	}
	if err != nil {
		return result, true, err
	}
	result.EvidenceReviewInputPath = inputPath
	result.EvidenceReviewInputSHA256 = inputSHA
	reviewExecutionIntent := binaryREAdapterExecutionIntent(intent)
	reviewIntent, reviewIntentPath, reviewIntentSHA, err := ensureBinaryREEvidenceReviewIntent(
		caseRoot, lane, reviewExecutionIntent, intentPath, intentSHA, inputPath, inputSHA,
	)
	if err != nil {
		return result, true, err
	}
	result.EvidenceReviewIntentPath = reviewIntentPath
	result.EvidenceReviewIntentSHA256 = reviewIntentSHA
	decision, decisionPath, decisionSHA, publication, replay, err := runBinaryREEvidenceReview(
		parent, dailyOpt, caseRoot, pack, lane, intent.AdapterID, binding, inputPath, inputSHA,
		reviewIntent, reviewIntentPath, reviewIntentSHA,
	)
	result.EvidenceReviewDecisionPath = decisionPath
	result.EvidenceReviewDecisionSHA256 = decisionSHA
	result.EvidenceReviewSession = reviewIntent.SessionID
	result.EvidenceReviewDecision = decision.Response.Decision
	result.EvidenceReviewReplay = replay
	result.ResultPublication = &publication
	result.SelectedEvidenceRef = binding.SelectedEvidenceRef
	if err != nil {
		return result, true, err
	}
	if publication.Held {
		result.State = publication.Disposition
		return result, true, nil
	}
	if decision.Response.Decision != "accepted" {
		result.State = "evidence-review-rejected"
		return result, true, nil
	}

	ackID, ackSHA, err := acknowledgeBinaryREEvidenceReview(
		caseRoot, pack, lane, item, binding, reviewIntent, decision.Response,
	)
	if err != nil {
		return result, true, err
	}
	closurePath, err := binaryREAdapterArtifactPath(caseRoot, lane, intent.GateEventID, "evidence-review", "closure.json")
	if err != nil {
		return result, true, err
	}
	_, _, closureKind := binaryREEvidenceReviewKinds(intent.AdapterID)
	closure := binaryREEvidenceReviewClosure{
		SchemaVersion: 1, Kind: closureKind, GateEventID: intent.GateEventID,
		ExecutionIntentPath: intentPath, ExecutionIntentSHA256: intentSHA,
		ExecutionResultPath: executionResultPath, ExecutionResultSHA256: result.ExecutionResultSHA256,
		InputPath: inputPath, InputSHA256: inputSHA,
		IntentPath: reviewIntentPath, IntentSHA256: reviewIntentSHA,
		DecisionPath: decisionPath, DecisionSHA256: decisionSHA,
		SessionID: reviewIntent.SessionID, AcknowledgementEventID: ackID,
		AcknowledgementSHA256: ackSHA, SelectedEvidenceRef: binding.SelectedEvidenceRef,
		Control: intent.Control, ClosedAt: reviewIntent.AcknowledgementCreatedAt,
		NoAuthority: true, NoHeavyTool: true,
	}
	closureSHA, err := writeBinaryREAdapterArtifact(caseRoot, closurePath, "web-security evidence review closure", closure)
	if err != nil {
		return result, true, err
	}
	memberBinding := memberexecution.TaskBinding{}
	if inventoryInput != nil {
		memberBinding = webSecurityOpenAPIMemberTaskBinding(
			*inventoryInput, run.ExecutionStatus, intentPath, intentSHA, executionResultPath, result.ExecutionResultSHA256,
			inputPath, inputSHA, reviewIntentPath, reviewIntentSHA, decisionPath, decisionSHA,
			closurePath, closureSHA, reviewIntent.SessionID, ackID, ackSHA,
		)
	} else {
		memberBinding = webSecurityReplayMemberTaskBinding(
			*replayInput, run.ExecutionStatus, intentPath, intentSHA, executionResultPath, result.ExecutionResultSHA256,
			inputPath, inputSHA, reviewIntentPath, reviewIntentSHA, decisionPath, decisionSHA,
			closurePath, closureSHA, reviewIntent.SessionID, ackID, ackSHA,
		)
	}
	bindingPath, bindingSHA, err := bindBinaryREEvidenceForMember(caseRoot, lane, intent.Control, memberBinding)
	if err != nil {
		return result, true, err
	}
	result.State = "ready-for-member"
	result.ReadyForMember = true
	result.AcknowledgementEventID = ackID
	result.AcknowledgementSHA256 = ackSHA
	result.ClosurePath = closurePath
	result.ClosureSHA256 = closureSHA
	result.TaskBindingPath = bindingPath
	result.TaskBindingSHA256 = bindingSHA
	return result, true, nil
}

func discoverWebSecurityAdapterSelection(repoRoot, caseRoot, pack, lane string) (webSecurityAdapterSelection, bool, error) {
	facts, err := mission.ReadStrictLedgerFacts(caseRoot)
	if err != nil {
		return webSecurityAdapterSelection{}, false, err
	}
	acknowledged := workstream.ExecutionEvidenceReviewAcknowledgedIDs(facts)
	matches := []webSecurityAdapterSelection{}
	for _, requestEvent := range facts.Requests {
		handoffs := workstream.AuthorizedGateAdapterHandoffsWithAcknowledgements(
			repoRoot, caseRoot, pack, []map[string]any{requestEvent}, lane, acknowledged,
		)
		if len(handoffs) == 0 {
			continue
		}
		if len(handoffs) != 1 {
			return webSecurityAdapterSelection{}, false, fmt.Errorf("web-security adapter gate projection is not unique")
		}
		handoff := handoffs[0]
		adapterID := ""
		switch handoff.Action {
		case "inspect":
			adapterID = websecurity.InventoryAdapterID
		case "network":
			adapterID = websecurity.ReplayAdapterID
		default:
			continue
		}
		if handoff.Status != "authorized-gate" || handoff.Authorization != "preauthorized" {
			return webSecurityAdapterSelection{}, false, fmt.Errorf("web-security gate authorization projection drifted")
		}
		if handoff.LiveValidation == nil {
			return webSecurityAdapterSelection{}, false, fmt.Errorf("web-security gate omitted live adapter validation")
		}
		live := handoff.LiveValidation
		var candidate *gate.AdapterToolCandidate
		for _, item := range live.AdapterCandidates {
			if item.ID != adapterID {
				continue
			}
			if candidate != nil {
				return webSecurityAdapterSelection{}, false, fmt.Errorf("web-security gate has duplicate candidate %s", adapterID)
			}
			copy := item
			candidate = &copy
		}
		if candidate == nil || !slices.Contains(candidate.GateActions, handoff.Action) {
			return webSecurityAdapterSelection{}, false, fmt.Errorf("web-security gate omitted exact candidate %s", adapterID)
		}
		if live.SelectedAdapter == nil || live.SelectedAdapterID != adapterID || live.SelectedAdapter.ID != adapterID ||
			live.SidecarTemplateAdapterID != adapterID {
			return webSecurityAdapterSelection{}, false, fmt.Errorf("web-security gate did not select the exact catalog adapter")
		}
		if live.DispatchPresent && (!live.DispatchCurrent || live.AdapterExecutionDispatchPath == "" || live.AdapterExecutionDispatchSHA256 == "") {
			return webSecurityAdapterSelection{}, false, fmt.Errorf("web-security gate immutable dispatch is stale or incomplete")
		}
		if strings.TrimSpace(handoff.ReportContractError) != "" || strings.TrimSpace(handoff.ReportPath) == "" {
			return webSecurityAdapterSelection{}, false, fmt.Errorf("web-security gate report contract is unavailable: %s", handoff.ReportContractError)
		}
		createdAt := mission.Value(requestEvent, "createdAt")
		if _, timeErr := time.Parse(time.RFC3339Nano, createdAt); timeErr != nil {
			return webSecurityAdapterSelection{}, false, fmt.Errorf("web-security gate createdAt is invalid: %w", timeErr)
		}
		inputPath := strings.TrimSpace(handoff.Target)
		limit := websecurity.MaxOpenAPIBytes
		if adapterID == websecurity.ReplayAdapterID {
			limit = websecurity.MaxReplayRequestBytes
		}
		inputData, readErr := readLiveAcceptanceVMPIDAFile(caseRoot, inputPath, "web-security exact gate input", int64(limit))
		if readErr != nil {
			return webSecurityAdapterSelection{}, false, readErr
		}
		input, bindErr := websecurity.BindFile(inputPath, inputData, limit)
		if bindErr != nil {
			return webSecurityAdapterSelection{}, false, bindErr
		}
		if adapterID == websecurity.ReplayAdapterID {
			if bindingErr := websecurity.ValidateReplayRequestBinding(input); bindingErr != nil {
				return webSecurityAdapterSelection{}, false, bindingErr
			}
		}
		selection := webSecurityAdapterSelection{
			Handoff: handoff, AdapterID: adapterID, Input: input,
			CreatedAt: createdAt, Acknowledged: acknowledged[handoff.EventID],
		}
		if adapterID == websecurity.ReplayAdapterID {
			selection.Replay, err = websecurity.DecodeReplayRequest(inputData)
			if err != nil {
				return webSecurityAdapterSelection{}, false, fmt.Errorf("read bounded replay gate request: %w", err)
			}
		}
		if selection.Acknowledged {
			complete, completionErr := completedWebSecurityAdapterLifecycle(caseRoot, lane, selection)
			if completionErr != nil {
				return webSecurityAdapterSelection{}, false, completionErr
			}
			if complete {
				continue
			}
		}
		matches = append(matches, selection)
	}
	if len(matches) == 0 {
		return webSecurityAdapterSelection{}, false, nil
	}
	if len(matches) != 1 {
		return webSecurityAdapterSelection{}, false, fmt.Errorf("multiple exact web-security authorized gates require an explicit distinct lane route")
	}
	return matches[0], true, nil
}

func ensureWebSecurityAdapterExecutionIntent(
	caseRoot,
	pack,
	lane string,
	selection webSecurityAdapterSelection,
	instructionIdentity instructionpacket.Identity,
) (_ webSecurityAdapterExecutionIntent, _ string, _ string, retErr error) {
	path, err := binaryREAdapterArtifactPath(caseRoot, lane, selection.Handoff.EventID, "execution-intent.json")
	if err != nil {
		return webSecurityAdapterExecutionIntent{}, "", "", err
	}
	if existing, data, found, readErr := readBinaryREAdapterArtifact[webSecurityAdapterExecutionIntent](caseRoot, path, "web-security adapter execution intent"); readErr != nil {
		return webSecurityAdapterExecutionIntent{}, "", "", readErr
	} else if found {
		if err := validateWebSecurityAdapterExecutionIntent(caseRoot, webSecurityPack, existing, lane, selection); err != nil {
			return webSecurityAdapterExecutionIntent{}, "", "", err
		}
		if err := requireBinaryREControlCurrent(caseRoot, existing.Control); err != nil {
			return webSecurityAdapterExecutionIntent{}, "", "", err
		}
		return existing, path, bytesSHA256(data), nil
	}
	owner, err := laneowner.Read(caseRoot, lane)
	if err != nil {
		return webSecurityAdapterExecutionIntent{}, "", "", err
	}
	capability, err := capabilitycontract.Bind(capabilitycontract.AuthorizedHeavy())
	if err != nil {
		return webSecurityAdapterExecutionIntent{}, "", "", err
	}
	control, err := executioncontrol.CaptureBinding(caseRoot, owner, capability)
	if err != nil {
		return webSecurityAdapterExecutionIntent{}, "", "", err
	}
	intent := webSecurityAdapterExecutionIntent{
		SchemaVersion: 1, Kind: webSecurityAdapterExecutionIntentKind, AdapterID: selection.AdapterID, Lane: lane,
		GateEventID: selection.Handoff.EventID, RequestPath: selection.Input.Path, RequestSHA256: selection.Input.SHA256,
		ReportPath: selection.Handoff.ReportPath, AdapterSession: webSecurityAdapterSession(selection.AdapterID, selection.Handoff.EventID),
		Actor: webSecurityAdapterActor, Owner: owner, Control: control,
		InstructionIdentity: cloneProductionInstructionIdentity(instructionIdentity),
		CreatedAt:           selection.CreatedAt, NoAuthority: true, NoAutoGate: true,
	}
	if err := validateWebSecurityAdapterExecutionIntent(caseRoot, pack, intent, lane, selection); err != nil {
		return webSecurityAdapterExecutionIntent{}, "", "", err
	}
	lease, err := lanemutation.AcquireLane(caseRoot, lane)
	if err != nil {
		return webSecurityAdapterExecutionIntent{}, "", "", err
	}
	defer func() { retErr = errors.Join(retErr, lease.Unlock()) }()
	if err := executioncontrol.RequireCurrentBindingWithLease(caseRoot, lease, control); err != nil {
		return webSecurityAdapterExecutionIntent{}, "", "", err
	}
	if existing, data, found, readErr := readBinaryREAdapterArtifact[webSecurityAdapterExecutionIntent](caseRoot, path, "web-security adapter execution intent"); readErr != nil {
		return webSecurityAdapterExecutionIntent{}, "", "", readErr
	} else if found {
		if err := validateWebSecurityAdapterExecutionIntent(caseRoot, webSecurityPack, existing, lane, selection); err != nil {
			return webSecurityAdapterExecutionIntent{}, "", "", err
		}
		return existing, path, bytesSHA256(data), nil
	}
	sha, err := writeBinaryREAdapterArtifact(caseRoot, path, "web-security adapter execution intent", intent)
	if err != nil {
		return webSecurityAdapterExecutionIntent{}, "", "", err
	}
	if err := lease.Validate(); err != nil {
		return webSecurityAdapterExecutionIntent{}, "", "", fmt.Errorf("web-security adapter execution intent may already be durable: %w", err)
	}
	return intent, path, sha, nil
}

func validateWebSecurityAdapterExecutionIntent(caseRoot, pack string, intent webSecurityAdapterExecutionIntent, lane string, selection webSecurityAdapterSelection) error {
	if intent.SchemaVersion != 1 || intent.Kind != webSecurityAdapterExecutionIntentKind || intent.AdapterID != selection.AdapterID ||
		intent.Lane != lane || intent.GateEventID != selection.Handoff.EventID || intent.RequestPath != selection.Input.Path ||
		!strings.EqualFold(intent.RequestSHA256, selection.Input.SHA256) || intent.ReportPath != selection.Handoff.ReportPath ||
		intent.AdapterSession != webSecurityAdapterSession(intent.AdapterID, selection.Handoff.EventID) ||
		intent.Actor != webSecurityAdapterActor || intent.Owner != intent.Control.Owner || intent.Control.Lane != lane ||
		intent.CreatedAt != selection.CreatedAt || !intent.NoAuthority || !intent.NoAutoGate {
		return fmt.Errorf("web-security adapter execution intent drifted from the exact gate, input, owner, or boundary")
	}
	if !isWebSecurityAdapterID(intent.AdapterID) {
		return fmt.Errorf("web-security adapter execution intent selected an unsupported adapter")
	}
	if _, err := time.Parse(time.RFC3339Nano, intent.CreatedAt); err != nil {
		return err
	}
	if err := validateCurrentProductionInstructionIdentity(caseRoot, pack, intent.InstructionIdentity); err != nil {
		return fmt.Errorf("web-security adapter execution intent instruction identity: %w", err)
	}
	return executioncontrol.ValidateBinding(intent.Control)
}

func webSecurityAdapterSession(adapterID, gateEventID string) string {
	return "web-security-" + shortStableIdentity(adapterID) + "-" + shortStableIdentity(gateEventID)
}

func validateWebSecurityAuthorizedRun(caseRoot string, intent webSecurityAdapterExecutionIntent, run adapterhost.AuthorizedRunResult) error {
	expectedKind := "openapi-inventory-authorized-run"
	expectedNoNetwork := true
	expectedBoundary := adapterhost.VMPIDAIndexNoNetworkBoundary
	if intent.AdapterID == websecurity.ReplayAdapterID {
		expectedKind = "bounded-replay-authorized-run"
		expectedNoNetwork = false
		expectedBoundary = adapterhost.BoundedReplayNetworkBoundary
	}
	if run.SchemaVersion != 1 || run.Kind != expectedKind || !casePathEqual(run.CaseRoot, caseRoot) ||
		run.Pack != webSecurityPack || run.GateEventID != intent.GateEventID || run.AdapterID != intent.AdapterID ||
		run.AdapterSession != intent.AdapterSession || run.ReportPath != intent.ReportPath ||
		run.DispatchPath == "" || !validBinaryRESHA256(run.DispatchSHA256) || !validBinaryRESHA256(run.ReportSHA256) ||
		run.ReceiptPath == "" || !validBinaryRESHA256(run.ReceiptSHA256) || run.ObservationEventID == "" ||
		run.TaskBindingPath != "" || run.TaskBindingSHA256 != "" || run.ProfilePath == "" ||
		!validBinaryRESHA256(run.ProfileSHA256) || (!run.ProfileRevoked && !run.ProfileAlreadyManual) ||
		run.ExecutionStatus == "" || run.ExecutionExitStatus == "" || run.NoNetwork != expectedNoNetwork ||
		run.NoNetworkBoundary != expectedBoundary || !run.NoAuthority || !equalProductionInstructionIdentityPointers(run.InstructionIdentity, &intent.InstructionIdentity) {
		return fmt.Errorf("web-security authorized adapter result omitted deferred-review or strict execution lineage")
	}
	if run.ExecutionStatus != "succeeded" && run.ExecutionStatus != "failed" && run.ExecutionStatus != "aborted" {
		return fmt.Errorf("web-security authorized adapter result is not terminal")
	}
	if (run.PacketPath == "") != (run.PacketSHA256 == "") || (run.PacketSHA256 != "" && !validBinaryRESHA256(run.PacketSHA256)) {
		return fmt.Errorf("web-security terminal adapter result contains an invalid typed artifact binding")
	}
	if run.ExecutionStatus == "succeeded" && run.PacketPath == "" {
		return fmt.Errorf("web-security successful adapter result omitted its typed artifact")
	}
	return nil
}

func validateWebSecurityAdapterExecutionResult(
	result webSecurityAdapterExecutionResult,
	caseRoot string,
	intent webSecurityAdapterExecutionIntent,
	intentPath,
	intentSHA string,
) error {
	if result.SchemaVersion != 1 || result.Kind != webSecurityAdapterExecutionResultKind ||
		result.InstructionIdentity.ReceiptKind != result.Kind ||
		result.GateEventID != intent.GateEventID || result.ExecutionIntentPath != intentPath ||
		!strings.EqualFold(result.ExecutionIntentSHA256, intentSHA) || result.Control != intent.Control ||
		!instructionpacket.EqualIdentity(result.InstructionIdentity, intent.InstructionIdentity) ||
		!result.NoAuthority || !result.NoHeavyToolAfterExecution {
		return fmt.Errorf("web-security adapter execution result drifted from its exact intent or control birth")
	}
	if err := executioncontrol.ValidateBinding(result.Control); err != nil {
		return err
	}
	return validateWebSecurityAuthorizedRun(caseRoot, intent, result.Run)
}

func persistWebSecurityAdapterExecutionResult(
	caseRoot,
	path string,
	result webSecurityAdapterExecutionResult,
	intent webSecurityAdapterExecutionIntent,
	intentPath,
	intentSHA string,
) (_ string, _ bool, retErr error) {
	if err := validateWebSecurityAdapterExecutionResult(result, caseRoot, intent, intentPath, intentSHA); err != nil {
		return "", false, err
	}
	lease, err := lanemutation.AcquireLane(caseRoot, intent.Lane)
	if err != nil {
		return "", false, err
	}
	defer func() { retErr = errors.Join(retErr, lease.Unlock()) }()
	if err := executioncontrol.RequireCurrentBindingWithLease(caseRoot, lease, intent.Control); err != nil {
		return "", false, err
	}
	if existing, data, found, readErr := readBinaryREAdapterArtifact[webSecurityAdapterExecutionResult](caseRoot, path, "web-security adapter execution result"); readErr != nil {
		return "", false, readErr
	} else if found {
		if err := validateWebSecurityAdapterExecutionResult(existing, caseRoot, intent, intentPath, intentSHA); err != nil {
			return "", false, err
		}
		if !reflect.DeepEqual(existing, result) {
			return "", false, fmt.Errorf("web-security adapter execution result differs from the durable terminal result")
		}
		return bytesSHA256(data), true, nil
	}
	sha, err := writeBinaryREAdapterArtifact(caseRoot, path, "web-security adapter execution result", result)
	if err != nil {
		return "", false, err
	}
	if err := lease.Validate(); err != nil {
		return "", false, fmt.Errorf("web-security adapter execution result may already be durable: %w", err)
	}
	return sha, false, nil
}

func inspectWebSecurityOpenAPIEvidence(
	caseRoot,
	lane string,
	item mission.ExecutionEvidenceReviewItem,
	selection webSecurityAdapterSelection,
	run adapterhost.AuthorizedRunResult,
	observationPath,
	observationSHA string,
) (webSecurityOpenAPIReviewInput, error) {
	receipt, err := validateWebSecurityEvidenceLineage(caseRoot, lane, item, selection, run)
	if err != nil {
		return webSecurityOpenAPIReviewInput{}, err
	}
	artifactData, err := readLiveAcceptanceVMPIDAFile(caseRoot, run.PacketPath, "web-security OpenAPI inventory evidence", websecurity.MaxInventoryBytes)
	if err != nil || !strings.EqualFold(bytesSHA256(artifactData), run.PacketSHA256) {
		return webSecurityOpenAPIReviewInput{}, fmt.Errorf("web-security OpenAPI inventory artifact hash drifted: %w", err)
	}
	inventory, err := websecurity.DecodeInventory(artifactData)
	if err != nil || inventory.Source != selection.Input || inventory.AdapterID != websecurity.InventoryAdapterID {
		return webSecurityOpenAPIReviewInput{}, fmt.Errorf("web-security OpenAPI inventory artifact is invalid or source-drifted: %w", err)
	}
	sourceData, err := readLiveAcceptanceVMPIDAFile(caseRoot, inventory.Source.Path, "web-security OpenAPI source", websecurity.MaxOpenAPIBytes)
	if err != nil || int64(len(sourceData)) != inventory.Source.Bytes || !strings.EqualFold(websecurity.SHA256(sourceData), inventory.Source.SHA256) {
		return webSecurityOpenAPIReviewInput{}, fmt.Errorf("web-security OpenAPI source drifted before evidence review: %w", err)
	}
	return webSecurityOpenAPIReviewInput{
		SchemaVersion: 1, Kind: webSecurityOpenAPIReviewInputKind, AdapterID: websecurity.InventoryAdapterID,
		GateEventID: run.GateEventID, ObservationEventID: run.ObservationEventID,
		ObservationPath: observationPath, ObservationSHA256: observationSHA,
		ProfileSHA256: receipt.Gate.Authorization.ProfileHash, Source: inventory.Source,
		InventoryPath: run.PacketPath, InventorySHA256: run.PacketSHA256,
		ReportPath: run.ReportPath, ReportSHA256: run.ReportSHA256,
		DispatchPath: run.DispatchPath, DispatchSHA256: run.DispatchSHA256,
		ReceiptPath: run.ReceiptPath, ReceiptSHA256: run.ReceiptSHA256,
		OpenAPIVersion: inventory.OpenAPIVersion, Servers: append([]websecurity.Server{}, inventory.Servers...),
		AuthSchemes: append([]websecurity.AuthScheme{}, inventory.AuthSchemes...), EndpointCount: len(inventory.Endpoints),
		WarningCount: len(inventory.Warnings), Boundaries: inventory.Boundaries,
		EvidenceRefs:        []string{run.PacketPath, run.ReportPath, run.ReceiptPath, observationPath},
		SelectedEvidenceRef: run.PacketPath, NoAuthority: true, NoHeavyTool: true,
	}, nil
}

func inspectWebSecurityReplayEvidence(
	caseRoot,
	lane string,
	item mission.ExecutionEvidenceReviewItem,
	selection webSecurityAdapterSelection,
	run adapterhost.AuthorizedRunResult,
	observationPath,
	observationSHA string,
) (webSecurityReplayReviewInput, error) {
	receipt, err := validateWebSecurityEvidenceLineage(caseRoot, lane, item, selection, run)
	if err != nil {
		return webSecurityReplayReviewInput{}, err
	}
	artifactData, err := readLiveAcceptanceVMPIDAFile(caseRoot, run.PacketPath, "web-security bounded replay evidence", websecurity.MaxReplayResultBytes)
	if err != nil || !strings.EqualFold(bytesSHA256(artifactData), run.PacketSHA256) {
		return webSecurityReplayReviewInput{}, fmt.Errorf("web-security bounded replay artifact hash drifted: %w", err)
	}
	replayResult, err := websecurity.DecodeReplayResult(artifactData)
	if err != nil || replayResult.Request != selection.Input || replayResult.AdapterID != websecurity.ReplayAdapterID {
		return webSecurityReplayReviewInput{}, fmt.Errorf("web-security bounded replay result is invalid or request-drifted: %w", err)
	}
	requestData, err := readLiveAcceptanceVMPIDAFile(caseRoot, selection.Input.Path, "web-security bounded replay request", websecurity.MaxReplayRequestBytes)
	if err != nil || int64(len(requestData)) != selection.Input.Bytes || !strings.EqualFold(websecurity.SHA256(requestData), selection.Input.SHA256) {
		return webSecurityReplayReviewInput{}, fmt.Errorf("web-security bounded replay request drifted before evidence review: %w", err)
	}
	request, err := websecurity.DecodeReplayRequest(requestData)
	if err != nil || request.Inventory != replayResult.Inventory || request.Target != replayResult.Target ||
		request.Operation != replayResult.Operation || request.Limits != replayResult.Limits || request.Boundaries != replayResult.Boundaries {
		return webSecurityReplayReviewInput{}, fmt.Errorf("web-security bounded replay result drifted from its exact secret-free request: %w", err)
	}
	inventoryData, err := readLiveAcceptanceVMPIDAFile(caseRoot, request.Inventory.Path, "web-security bounded replay inventory", websecurity.MaxInventoryBytes)
	if err != nil || int64(len(inventoryData)) != request.Inventory.Bytes || !strings.EqualFold(websecurity.SHA256(inventoryData), request.Inventory.SHA256) {
		return webSecurityReplayReviewInput{}, fmt.Errorf("web-security bounded replay inventory drifted before evidence review: %w", err)
	}
	if _, err := websecurity.DecodeInventory(inventoryData); err != nil {
		return webSecurityReplayReviewInput{}, fmt.Errorf("web-security bounded replay inventory is invalid: %w", err)
	}
	return webSecurityReplayReviewInput{
		SchemaVersion: 1, Kind: webSecurityReplayReviewInputKind, AdapterID: websecurity.ReplayAdapterID,
		GateEventID: run.GateEventID, ObservationEventID: run.ObservationEventID,
		ObservationPath: observationPath, ObservationSHA256: observationSHA,
		ProfileSHA256: receipt.Gate.Authorization.ProfileHash, Request: replayResult.Request, Inventory: replayResult.Inventory,
		ResultPath: run.PacketPath, ResultSHA256: run.PacketSHA256,
		ReportPath: run.ReportPath, ReportSHA256: run.ReportSHA256,
		DispatchPath: run.DispatchPath, DispatchSHA256: run.DispatchSHA256,
		ReceiptPath: run.ReceiptPath, ReceiptSHA256: run.ReceiptSHA256,
		Target: replayResult.Target, Operation: replayResult.Operation, Status: replayResult.Status,
		Delivery: replayResult.Delivery, Actual: replayResult.Actual, Diff: replayResult.Diff,
		Limits: replayResult.Limits, Boundaries: replayResult.Boundaries,
		EvidenceRefs:        []string{run.PacketPath, run.ReportPath, run.ReceiptPath, observationPath},
		SelectedEvidenceRef: run.PacketPath, NoAuthority: true, NoHeavyTool: true,
	}, nil
}

func validateWebSecurityEvidenceLineage(
	caseRoot,
	lane string,
	item mission.ExecutionEvidenceReviewItem,
	selection webSecurityAdapterSelection,
	run adapterhost.AuthorizedRunResult,
) (adapterexecution.Receipt, error) {
	if run.AdapterID != selection.AdapterID || item.GateEventID != run.GateEventID || item.EventID != run.ObservationEventID ||
		item.Target != selection.Input.Path || item.ExecutionReportPath != run.ReportPath ||
		!strings.EqualFold(item.ExecutionReportSHA256, run.ReportSHA256) ||
		item.AdapterExecutionDispatchPath != run.DispatchPath || !strings.EqualFold(item.AdapterExecutionDispatchSHA256, run.DispatchSHA256) ||
		item.AdapterExecutionReceiptPath != run.ReceiptPath || !strings.EqualFold(item.AdapterExecutionReceiptSHA256, run.ReceiptSHA256) ||
		item.AdapterID != selection.AdapterID || item.AdapterSession != run.AdapterSession || item.AdapterExecutionArtifactCount != 1 {
		return adapterexecution.Receipt{}, fmt.Errorf("web-security evidence review observation lineage drifted")
	}
	dispatchData, err := readLiveAcceptanceVMPIDAFile(caseRoot, run.DispatchPath, "web-security evidence review dispatch", binaryREAdapterArtifactMaxBytes)
	if err != nil || !strings.EqualFold(bytesSHA256(dispatchData), run.DispatchSHA256) {
		return adapterexecution.Receipt{}, fmt.Errorf("web-security evidence review dispatch hash drifted: %w", err)
	}
	dispatch, err := adapterexecution.DecodeDispatch(dispatchData)
	if err != nil || dispatch.Gate.GateEventID != run.GateEventID || dispatch.Gate.Action != selection.Handoff.Action ||
		dispatch.Gate.Target != selection.Input.Path || dispatch.Adapter.Pack != webSecurityPack ||
		dispatch.Adapter.AdapterID != selection.AdapterID || dispatch.Owner.AdapterSession != run.AdapterSession ||
		dispatch.ReportPath != run.ReportPath {
		return adapterexecution.Receipt{}, fmt.Errorf("web-security evidence review dispatch drifted: %w", err)
	}
	receipt, receiptPath, receiptSHA, present, err := gate.ReadAdapterExecutionReceipt(caseRoot, lane, run.GateEventID)
	if err != nil || !present || receipt == nil || receiptPath != run.ReceiptPath || !strings.EqualFold(receiptSHA, run.ReceiptSHA256) {
		return adapterexecution.Receipt{}, fmt.Errorf("web-security evidence review receipt drifted: %w", err)
	}
	if err := adapterhost.ValidateWebSecurityReceiptArtifacts(caseRoot, dispatch, run.DispatchPath, run.DispatchSHA256, run, *receipt); err != nil {
		return adapterexecution.Receipt{}, err
	}
	return *receipt, nil
}

func webSecurityOpenAPIReviewBinding(input webSecurityOpenAPIReviewInput) binaryREEvidenceReviewBinding {
	return binaryREEvidenceReviewBinding{
		Kind: input.Kind, GateEventID: input.GateEventID, ObservationEventID: input.ObservationEventID,
		ObservationPath: input.ObservationPath, ObservationSHA256: input.ObservationSHA256,
		ProfileSHA256: input.ProfileSHA256, TargetPath: input.Source.Path, TargetSHA256: input.Source.SHA256,
		ArtifactPath: input.InventoryPath, ArtifactSHA256: input.InventorySHA256,
		ReportPath: input.ReportPath, ReportSHA256: input.ReportSHA256,
		DispatchPath: input.DispatchPath, DispatchSHA256: input.DispatchSHA256,
		ReceiptPath: input.ReceiptPath, ReceiptSHA256: input.ReceiptSHA256,
		SelectedEvidenceRef: input.SelectedEvidenceRef, EvidenceRefs: append([]string{}, input.EvidenceRefs...),
		NoAuthority: input.NoAuthority, NoHeavyTool: input.NoHeavyTool,
	}
}

func webSecurityReplayReviewBinding(input webSecurityReplayReviewInput) binaryREEvidenceReviewBinding {
	return binaryREEvidenceReviewBinding{
		Kind: input.Kind, GateEventID: input.GateEventID, ObservationEventID: input.ObservationEventID,
		ObservationPath: input.ObservationPath, ObservationSHA256: input.ObservationSHA256,
		ProfileSHA256: input.ProfileSHA256, TargetPath: input.Request.Path, TargetSHA256: input.Request.SHA256,
		ArtifactPath: input.ResultPath, ArtifactSHA256: input.ResultSHA256,
		ReportPath: input.ReportPath, ReportSHA256: input.ReportSHA256,
		DispatchPath: input.DispatchPath, DispatchSHA256: input.DispatchSHA256,
		ReceiptPath: input.ReceiptPath, ReceiptSHA256: input.ReceiptSHA256,
		SelectedEvidenceRef: input.SelectedEvidenceRef, EvidenceRefs: append([]string{}, input.EvidenceRefs...),
		NoAuthority: input.NoAuthority, NoHeavyTool: input.NoHeavyTool,
	}
}

func webSecurityOpenAPIMemberTaskBinding(
	input webSecurityOpenAPIReviewInput,
	executionStatus,
	executionIntentPath,
	executionIntentSHA,
	executionResultPath,
	executionResultSHA,
	inputPath,
	inputSHA,
	intentPath,
	intentSHA,
	decisionPath,
	decisionSHA,
	closurePath,
	closureSHA,
	sessionID,
	ackID,
	ackSHA string,
) memberexecution.TaskBinding {
	return webSecurityMemberTaskBinding(
		webSecurityOpenAPIMemberBindingKind, input.GateEventID, websecurity.InventoryAdapterID,
		input.Source.Path, input.Source.SHA256, input.InventoryPath, input.InventorySHA256,
		input.ReportPath, input.ReportSHA256, input.DispatchPath, input.DispatchSHA256,
		input.ReceiptPath, input.ReceiptSHA256, input.ObservationEventID,
		input.ObservationPath, input.ObservationSHA256, input.SelectedEvidenceRef,
		executionStatus, executionIntentPath, executionIntentSHA, executionResultPath, executionResultSHA,
		inputPath, inputSHA, intentPath, intentSHA, decisionPath, decisionSHA, closurePath, closureSHA,
		sessionID, ackID, ackSHA,
	)
}

func webSecurityReplayMemberTaskBinding(
	input webSecurityReplayReviewInput,
	executionStatus,
	executionIntentPath,
	executionIntentSHA,
	executionResultPath,
	executionResultSHA,
	inputPath,
	inputSHA,
	intentPath,
	intentSHA,
	decisionPath,
	decisionSHA,
	closurePath,
	closureSHA,
	sessionID,
	ackID,
	ackSHA string,
) memberexecution.TaskBinding {
	return webSecurityMemberTaskBinding(
		webSecurityReplayMemberBindingKind, input.GateEventID, websecurity.ReplayAdapterID,
		input.Request.Path, input.Request.SHA256, input.ResultPath, input.ResultSHA256,
		input.ReportPath, input.ReportSHA256, input.DispatchPath, input.DispatchSHA256,
		input.ReceiptPath, input.ReceiptSHA256, input.ObservationEventID,
		input.ObservationPath, input.ObservationSHA256, input.SelectedEvidenceRef,
		executionStatus, executionIntentPath, executionIntentSHA, executionResultPath, executionResultSHA,
		inputPath, inputSHA, intentPath, intentSHA, decisionPath, decisionSHA, closurePath, closureSHA,
		sessionID, ackID, ackSHA,
	)
}

func webSecurityMemberTaskBinding(
	kind,
	gateEventID,
	adapterID,
	targetPath,
	targetSHA,
	artifactPath,
	artifactSHA,
	reportPath,
	reportSHA,
	dispatchPath,
	dispatchSHA,
	receiptPath,
	receiptSHA,
	observationEventID,
	observationPath,
	observationSHA,
	selectedEvidenceRef,
	executionStatus,
	executionIntentPath,
	executionIntentSHA,
	executionResultPath,
	executionResultSHA,
	inputPath,
	inputSHA,
	intentPath,
	intentSHA,
	decisionPath,
	decisionSHA,
	closurePath,
	closureSHA,
	sessionID,
	ackID,
	ackSHA string,
) memberexecution.TaskBinding {
	return memberexecution.TaskBinding{
		Kind: kind,
		Values: map[string]string{
			"gate-event-id": gateEventID, "adapter-id": adapterID,
			"input-path": targetPath, "input-sha256": targetSHA,
			"artifact-path": artifactPath, "artifact-sha256": artifactSHA,
			"report-path": reportPath, "report-sha256": reportSHA,
			"dispatch-path": dispatchPath, "dispatch-sha256": dispatchSHA,
			"receipt-path": receiptPath, "receipt-sha256": receiptSHA,
			"observation-event-id": observationEventID,
			"observation-path":     observationPath, "observation-sha256": observationSHA,
			"selected-evidence-ref": selectedEvidenceRef,
			"execution-status":      executionStatus,
			"execution-intent-path": executionIntentPath, "execution-intent-sha256": executionIntentSHA,
			"execution-result-path": executionResultPath, "execution-result-sha256": executionResultSHA,
			"evidence-review-input-path": inputPath, "evidence-review-input-sha256": inputSHA,
			"evidence-review-intent-path": intentPath, "evidence-review-intent-sha256": intentSHA,
			"evidence-review-decision-path": decisionPath, "evidence-review-decision-sha256": decisionSHA,
			"evidence-review-closure-path": closurePath, "evidence-review-closure-sha256": closureSHA,
			"evidence-review-session-id":   sessionID,
			"evidence-review-ack-event-id": ackID, "evidence-review-ack-event-sha256": ackSHA,
		},
	}
}

func completedWebSecurityAdapterLifecycle(caseRoot, lane string, selection webSecurityAdapterSelection) (bool, error) {
	intentPath, err := binaryREAdapterArtifactPath(caseRoot, lane, selection.Handoff.EventID, "execution-intent.json")
	if err != nil {
		return false, err
	}
	intent, intentData, found, err := readBinaryREAdapterArtifact[webSecurityAdapterExecutionIntent](caseRoot, intentPath, "web-security adapter execution intent")
	if err != nil {
		return false, err
	}
	if !found {
		return true, nil
	}
	if err := validateWebSecurityAdapterExecutionIntent(caseRoot, webSecurityPack, intent, lane, selection); err != nil {
		return false, err
	}
	intentSHA := bytesSHA256(intentData)
	executionResultPath, err := binaryREAdapterArtifactPath(caseRoot, lane, intent.GateEventID, "execution-result.json")
	if err != nil {
		return false, err
	}
	executionResult, executionData, found, err := readBinaryREAdapterArtifact[webSecurityAdapterExecutionResult](caseRoot, executionResultPath, "web-security adapter execution result")
	if err != nil {
		return false, err
	}
	if !found {
		return false, fmt.Errorf("acknowledged web-security lifecycle omitted its durable execution result")
	}
	if err := validateWebSecurityAdapterExecutionResult(executionResult, caseRoot, intent, intentPath, intentSHA); err != nil {
		return false, err
	}
	if executionResult.Run.PacketPath == "" {
		return true, nil
	}
	executionSHA := bytesSHA256(executionData)
	item, observationPath, observationSHA, err := ensureBinaryREObservationSnapshot(caseRoot, lane, executionResult.Run)
	if err != nil {
		return false, err
	}
	inputPath, err := binaryREAdapterArtifactPath(caseRoot, lane, intent.GateEventID, "evidence-review", "input.json")
	if err != nil {
		return false, err
	}
	inputData := []byte(nil)
	binding := binaryREEvidenceReviewBinding{}
	memberBinding := memberexecution.TaskBinding{}
	var inventoryInput *webSecurityOpenAPIReviewInput
	var replayInput *webSecurityReplayReviewInput
	if selection.AdapterID == websecurity.InventoryAdapterID {
		expected, inspectErr := inspectWebSecurityOpenAPIEvidence(caseRoot, lane, item, selection, executionResult.Run, observationPath, observationSHA)
		if inspectErr != nil {
			return false, inspectErr
		}
		input, data, present, readErr := readBinaryREAdapterArtifact[webSecurityOpenAPIReviewInput](caseRoot, inputPath, "web-security OpenAPI evidence review input")
		if readErr != nil {
			return false, readErr
		}
		if !present || !reflect.DeepEqual(input, expected) {
			return false, fmt.Errorf("acknowledged web-security OpenAPI evidence review input is missing or drifted")
		}
		inputData = data
		binding = webSecurityOpenAPIReviewBinding(input)
		inventoryInput = &input
	} else {
		expected, inspectErr := inspectWebSecurityReplayEvidence(caseRoot, lane, item, selection, executionResult.Run, observationPath, observationSHA)
		if inspectErr != nil {
			return false, inspectErr
		}
		input, data, present, readErr := readBinaryREAdapterArtifact[webSecurityReplayReviewInput](caseRoot, inputPath, "web-security bounded replay evidence review input")
		if readErr != nil {
			return false, readErr
		}
		if !present || !reflect.DeepEqual(input, expected) {
			return false, fmt.Errorf("acknowledged web-security bounded replay evidence review input is missing or drifted")
		}
		inputData = data
		binding = webSecurityReplayReviewBinding(input)
		replayInput = &input
	}
	inputSHA := bytesSHA256(inputData)
	reviewIntentPath, err := binaryREAdapterArtifactPath(caseRoot, lane, intent.GateEventID, "evidence-review", "intent.json")
	if err != nil {
		return false, err
	}
	reviewIntent, reviewIntentData, found, err := readBinaryREAdapterArtifact[binaryREEvidenceReviewIntent](caseRoot, reviewIntentPath, "web-security evidence review intent")
	if err != nil {
		return false, err
	}
	if !found {
		return false, fmt.Errorf("acknowledged web-security lifecycle omitted its evidence review intent")
	}
	binaryIntent := binaryREAdapterExecutionIntent(intent)
	if err := validateBinaryREEvidenceReviewIntent(reviewIntent, binaryIntent, intentPath, intentSHA, inputPath, inputSHA); err != nil {
		return false, err
	}
	reviewIntentSHA := bytesSHA256(reviewIntentData)
	decisionPath, err := binaryREAdapterArtifactPath(caseRoot, lane, intent.GateEventID, "evidence-review", "decision.json")
	if err != nil {
		return false, err
	}
	decision, decisionData, found, err := readBinaryREAdapterArtifact[binaryREEvidenceReviewDecision](caseRoot, decisionPath, "web-security evidence review decision")
	if err != nil {
		return false, err
	}
	if !found {
		return false, fmt.Errorf("acknowledged web-security lifecycle omitted its evidence review decision")
	}
	decisionSHA := bytesSHA256(decisionData)
	if err := validateBinaryREEvidenceReviewDecision(decision, selection.AdapterID, binding, inputPath, inputSHA, reviewIntent, reviewIntentPath, reviewIntentSHA); err != nil {
		return false, err
	}
	if decision.Response.Decision != "accepted" {
		return false, fmt.Errorf("acknowledged web-security lifecycle has a non-accepted evidence review decision")
	}
	closurePath, err := binaryREAdapterArtifactPath(caseRoot, lane, intent.GateEventID, "evidence-review", "closure.json")
	if err != nil {
		return false, err
	}
	closure, closureData, closureFound, err := readBinaryREAdapterArtifact[binaryREEvidenceReviewClosure](caseRoot, closurePath, "web-security evidence review closure")
	if err != nil {
		return false, err
	}
	if !closureFound {
		return false, nil
	}
	if err := validateBinaryREEvidenceReviewClosure(
		caseRoot, lane, closure, binaryIntent, intentPath, intentSHA, executionResultPath, executionSHA,
		binding, inputPath, inputSHA, reviewIntent, reviewIntentPath, reviewIntentSHA, decision, decisionPath, decisionSHA,
	); err != nil {
		return false, err
	}
	closureSHA := bytesSHA256(closureData)
	if inventoryInput != nil {
		memberBinding = webSecurityOpenAPIMemberTaskBinding(
			*inventoryInput, executionResult.Run.ExecutionStatus, intentPath, intentSHA, executionResultPath, executionSHA,
			inputPath, inputSHA, reviewIntentPath, reviewIntentSHA, decisionPath, decisionSHA,
			closurePath, closureSHA, reviewIntent.SessionID, closure.AcknowledgementEventID, closure.AcknowledgementSHA256,
		)
	} else {
		memberBinding = webSecurityReplayMemberTaskBinding(
			*replayInput, executionResult.Run.ExecutionStatus, intentPath, intentSHA, executionResultPath, executionSHA,
			inputPath, inputSHA, reviewIntentPath, reviewIntentSHA, decisionPath, decisionSHA,
			closurePath, closureSHA, reviewIntent.SessionID, closure.AcknowledgementEventID, closure.AcknowledgementSHA256,
		)
	}
	storedBinding, _, _, err := memberexecution.ReadTaskBindingForOwner(caseRoot, lane, intent.Control.Owner.ExecutorGeneration)
	if err != nil {
		return false, err
	}
	if storedBinding == nil {
		return false, nil
	}
	if !reflect.DeepEqual(*storedBinding, memberBinding) {
		return false, fmt.Errorf("acknowledged web-security member task binding drifted from its reviewed lifecycle")
	}
	return true, nil
}

func webSecurityEvidenceReviewKinds(adapterID string) (string, string, string, bool) {
	switch adapterID {
	case websecurity.InventoryAdapterID:
		return webSecurityOpenAPIReviewIntentKind, webSecurityOpenAPIReviewDecisionKind, webSecurityOpenAPIReviewClosureKind, true
	case websecurity.ReplayAdapterID:
		return webSecurityReplayReviewIntentKind, webSecurityReplayReviewDecisionKind, webSecurityReplayReviewClosureKind, true
	default:
		return "", "", "", false
	}
}

func webSecurityEvidenceReviewInputRole(adapterID string) (string, bool) {
	switch adapterID {
	case websecurity.InventoryAdapterID:
		return webSecurityOpenAPIReviewInputRole, true
	case websecurity.ReplayAdapterID:
		return webSecurityReplayReviewInputRole, true
	default:
		return "", false
	}
}

func webSecurityEvidenceReviewExpectedOutput(adapterID string) (string, bool) {
	switch adapterID {
	case websecurity.InventoryAdapterID:
		return "one strict accepted/rejected evidence review decision bound to the exact secret-free OpenAPI inventory, source, observation, and receipt", true
	case websecurity.ReplayAdapterID:
		return "one strict accepted/rejected evidence review decision bound to the exact redacted bounded replay result, delivery state, observation, and receipt", true
	default:
		return "", false
	}
}

func evidenceReviewAdapterActor(adapterID string) string {
	if isWebSecurityAdapterID(adapterID) {
		return webSecurityAdapterActor
	}
	return binaryREAdapterActor
}

func evidenceReviewAdapterActorForPack(pack string) string {
	if strings.TrimSpace(pack) == webSecurityPack {
		return webSecurityAdapterActor
	}
	return binaryREAdapterActor
}

func isWebSecurityAdapterID(adapterID string) bool {
	return adapterID == websecurity.InventoryAdapterID || adapterID == websecurity.ReplayAdapterID
}
