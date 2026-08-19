package sessionhost

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/adapterexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/adapterhost"
	"github.com/shuiyu486/re-context-kits/internal/rekit/autonomy"
	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/gate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewerresult"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

const liveAcceptanceVMPIDAQueryTerm = "needle_dispatch"

func prepareLiveAcceptanceVMPIDA(caseRoot, pack, lane string, proof *LiveAcceptanceVMPIDA) error {
	if proof == nil {
		return fmt.Errorf("VMP IDA live acceptance adapter proof is missing")
	}
	if proof.GateEventID != "" {
		return nil
	}
	if !strings.EqualFold(pack, liveAcceptancePack) || strings.TrimSpace(lane) == "" {
		return fmt.Errorf("VMP IDA live acceptance adapter preparation requires the exact pack and lane")
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		return fmt.Errorf("read VMP IDA live acceptance lane workspace: %w", err)
	}
	selectedLane, ok := mission.LookupBoardLane(board.Lanes, lane, false)
	if !ok || strings.TrimSpace(selectedLane.Workspace) == "" {
		return fmt.Errorf("VMP IDA live acceptance selected lane workspace is unavailable")
	}
	outputRoot := filepath.ToSlash(filepath.Join(selectedLane.Workspace, "ida-index", "session-1"))
	if err := writeLiveAcceptanceVMPIDAIndexes(caseRoot); err != nil {
		return err
	}
	requestPreview, err := adapterhost.PreviewVMPIDAIndexRequestForQuery(caseRoot, adapterhost.VMPIDAIndexDefaultExportRoot, adapterhost.VMPIDAIndexQuery{
		SchemaVersion: 1, Terms: []string{liveAcceptanceVMPIDAQueryTerm}, MaxRowsPerIndex: 10,
	})
	if err != nil {
		return err
	}
	published, err := adapterhost.PublishVMPIDAIndexRequest(caseRoot, requestPreview)
	if err != nil {
		return err
	}
	if published.RequestPath != requestPreview.RequestPath || published.RequestSHA256 != requestPreview.RequestSHA256 {
		return fmt.Errorf("VMP IDA live acceptance request publication drifted from preview")
	}
	proof.RequestPath, proof.RequestSHA256 = published.RequestPath, published.RequestSHA256

	currentProfile, _, profileExists, err := autonomy.Read(caseRoot, lane)
	if err != nil || !profileExists || !reflect.DeepEqual(currentProfile, autonomy.DefaultProfile(lane)) {
		return fmt.Errorf("VMP IDA live acceptance requires the exact existing manual profile before public provision: %w", err)
	}
	now := time.Now().UTC()
	profileArgs := []string{
		"-Command", "gate", "-Target", caseRoot, "-Pack", pack, "-Format", "json",
		"-ProvisionProfile", "-Lane", lane, "-Action", "inspect", "-TargetRef", published.RequestPath,
		"-ProfileId", "dpc04-vmp-ida-index", "-ProfileGrantedBy", "rekit-live-acceptance",
		"-ProfileGrantedAt", now.Add(-time.Second).Format(time.RFC3339Nano),
		"-ProfileExpiresAt", now.Add(10 * time.Minute).Format(time.RFC3339Nano),
		"-RuntimeSeconds", "10", "-DiskMB", "4", "-Requests", "1",
		"-StopConditions", "scope-drift,source-drift,output-exceeds-bounded-evidence-packet",
		"-OutputPaths", outputRoot,
	}
	var profilePlan autonomy.ProfileMutationPlan
	if err := runPublicCLI(profileArgs, &profilePlan); err != nil {
		return fmt.Errorf("VMP IDA live acceptance public profile preview: %w", err)
	}
	if !profilePlan.IsMutation || !profilePlan.ReviewRequired || !profilePlan.RequiresConfirmation || profilePlan.ExpectedPlanSHA256 == "" || profilePlan.PlannedProfile.Mode != autonomy.ModePreauthorized || !reflect.DeepEqual(profilePlan.PlannedProfile.AllowedActions, []string{"inspect"}) {
		return fmt.Errorf("VMP IDA live acceptance public profile preview omitted the exact reviewed mutation")
	}
	proof.ProfilePreviewSHA256 = profilePlan.ExpectedPlanSHA256
	proof.DeniedActions = append([]string{}, profilePlan.PlannedProfile.DeniedActions...)
	var profileResult autonomy.ProfileMutationResult
	if err := runPublicCLI(append(profileArgs, "-Apply", "-ExpectedProfilePlanSha256", profilePlan.ExpectedPlanSHA256), &profileResult); err != nil {
		return fmt.Errorf("VMP IDA live acceptance public profile Apply: %w", err)
	}
	if !profileResult.Applied || profileResult.AlreadyApplied || !strings.EqualFold(profileResult.ProfileSHA256, profilePlan.PlannedProfileSHA256) {
		return fmt.Errorf("VMP IDA live acceptance public profile Apply drifted from preview")
	}
	proof.ProfileSHA256 = profileResult.ProfileSHA256

	gateArgs := []string{
		"-Command", "gate", "-Target", caseRoot, "-Pack", pack, "-Format", "json", "-Apply",
		"-Action", "inspect", "-Lane", lane, "-Actor", "rekit-live-acceptance",
		"-Subject", "inspect synthetic existing IDA indexes",
		"-Summary", "bounded literal read-only inspection for DPC-04 acceptance",
		"-TargetRef", published.RequestPath,
		"-RuntimeSeconds", "10", "-DiskMB", "4", "-Requests", "1",
		"-OutputPaths", outputRoot,
		"-StopConditions", "scope-drift,source-drift,output-exceeds-bounded-evidence-packet",
	}
	var authorized gate.ApplyResult
	if err := runPublicCLI(gateArgs, &authorized); err != nil || !authorized.Applied || authorized.Event == nil || authorized.Event.Gate.Authorization.Decision != autonomy.DecisionPreauthorized {
		return fmt.Errorf("VMP IDA live acceptance gate was not strictly preauthorized through the public route: %w", err)
	}
	proof.GateEventID = authorized.EventID
	proof.Authorization = authorized.Event.Gate.Authorization.Decision
	return nil
}

func projectLiveAcceptanceVMPIDALifecycle(
	caseRoot,
	lane string,
	lifecycle *BinaryREAdapterLifecycleResult,
	proof *LiveAcceptanceVMPIDA,
) error {
	if proof == nil || lifecycle == nil || lifecycle.Run == nil {
		return fmt.Errorf("VMP IDA live acceptance ordinary lifecycle proof is missing")
	}
	publication := lifecycle.ResultPublication
	if lifecycle.SchemaVersion != 1 || lifecycle.Kind != binaryREAdapterLifecycleKind ||
		lifecycle.State != "ready-for-member" || !lifecycle.ReadyForMember ||
		lifecycle.AdapterID != adapterhost.VMPIDAIndexAdapterID ||
		lifecycle.GateEventID != proof.GateEventID || lifecycle.RequestPath != proof.RequestPath ||
		!strings.EqualFold(lifecycle.RequestSHA256, proof.RequestSHA256) ||
		lifecycle.ExecutionStatus != "succeeded" || lifecycle.AdapterProcessID <= 0 ||
		!lifecycle.ChildLaunched || lifecycle.AdapterReplay || lifecycle.EvidenceReviewReplay ||
		lifecycle.EvidenceReviewDecision != "accepted" || publication == nil ||
		!publication.Published || publication.Held || publication.Disposition != executioncontrol.ResultDispositionPublished ||
		!lifecycle.NoAuthority || !lifecycle.NoHeavyToolAfterExecution {
		return fmt.Errorf("VMP IDA live acceptance did not traverse one fresh ordinary adapter lifecycle: %+v", lifecycle)
	}
	for label, pair := range map[string][2]string{
		"execution intent":         {lifecycle.ExecutionIntentPath, lifecycle.ExecutionIntentSHA256},
		"execution result":         {lifecycle.ExecutionResultPath, lifecycle.ExecutionResultSHA256},
		"evidence review input":    {lifecycle.EvidenceReviewInputPath, lifecycle.EvidenceReviewInputSHA256},
		"evidence review intent":   {lifecycle.EvidenceReviewIntentPath, lifecycle.EvidenceReviewIntentSHA256},
		"evidence review decision": {lifecycle.EvidenceReviewDecisionPath, lifecycle.EvidenceReviewDecisionSHA256},
		"evidence review closure":  {lifecycle.ClosurePath, lifecycle.ClosureSHA256},
		"member task binding":      {lifecycle.TaskBindingPath, lifecycle.TaskBindingSHA256},
	} {
		if strings.TrimSpace(pair[0]) == "" || !validBinaryRESHA256(pair[1]) {
			return fmt.Errorf("VMP IDA ordinary lifecycle omitted exact %s lineage", label)
		}
	}
	if lifecycle.EvidenceReviewSession == "" || lifecycle.AcknowledgementEventID == "" ||
		!validBinaryRESHA256(lifecycle.AcknowledgementSHA256) || lifecycle.SelectedEvidenceRef == "" {
		return fmt.Errorf("VMP IDA ordinary lifecycle omitted independent review or acknowledgement lineage")
	}
	run := *lifecycle.Run
	if run.ChildProcessID <= 0 || !run.ChildLaunched || run.Replay || !run.ProfileRevoked ||
		run.ProfileAlreadyManual || run.TaskBindingPath != "" || run.TaskBindingSHA256 != "" ||
		!run.NoNetwork || run.NoNetworkBoundary != adapterhost.VMPIDAIndexNoNetworkBoundary || !run.NoAuthority {
		return fmt.Errorf("VMP IDA ordinary lifecycle did not execute one contained child with deferred binding: %+v", run)
	}
	copy := *lifecycle
	runCopy := run
	publicationCopy := *publication
	copy.Run = &runCopy
	copy.ResultPublication = &publicationCopy
	proof.Lifecycle = &copy
	proof.AdapterProcessID = lifecycle.AdapterProcessID
	proof.Run = run
	proof.AcknowledgementEventID = lifecycle.AcknowledgementEventID
	proof.EvidenceReviewSessionID = lifecycle.EvidenceReviewSession
	proof.EvidenceReviewDecision = lifecycle.EvidenceReviewDecision
	proof.SelectedEvidenceRef = lifecycle.SelectedEvidenceRef

	evidence, err := currentLiveAcceptanceVMPIDAEvidence(caseRoot, lane, proof)
	if err != nil {
		return err
	}
	if evidence.ProfileSHA256 != proof.ProfileSHA256 || evidence.Selected.EvidenceRef != proof.SelectedEvidenceRef {
		return fmt.Errorf("VMP IDA ordinary lifecycle evidence drifted from its profile or selected row")
	}
	executionIntent, executionIntentData, found, err := readBinaryREAdapterArtifact[binaryREAdapterExecutionIntent](
		caseRoot,
		lifecycle.ExecutionIntentPath,
		"VMP IDA live acceptance ordinary execution intent",
	)
	if err != nil {
		return err
	}
	if !found || !strings.EqualFold(bytesSHA256(executionIntentData), lifecycle.ExecutionIntentSHA256) ||
		executionIntent.Lane != lane || executionIntent.GateEventID != proof.GateEventID ||
		executionIntent.RequestPath != proof.RequestPath || !strings.EqualFold(executionIntent.RequestSHA256, proof.RequestSHA256) {
		return fmt.Errorf("VMP IDA ordinary lifecycle execution intent drifted from the acceptance route")
	}
	binding, bindingPath, bindingSHA, err := memberexecution.ReadTaskBindingForOwner(
		caseRoot,
		lane,
		executionIntent.Control.Owner.ExecutorGeneration,
	)
	if err != nil {
		return err
	}
	expected := liveAcceptanceVMPIDATaskBinding(proof, evidence)
	if binding == nil || bindingPath != lifecycle.TaskBindingPath ||
		!strings.EqualFold(bindingSHA, lifecycle.TaskBindingSHA256) || !reflect.DeepEqual(*binding, expected) {
		return fmt.Errorf("VMP IDA ordinary lifecycle member binding drifted from reviewed evidence")
	}
	facts, err := mission.ReadStrictLedgerFacts(caseRoot)
	if err != nil {
		return err
	}
	proof.EvidenceReviewCleared = len(mission.ExecutionEvidenceReviewItemsWithLedgerFacts(facts, lane, nil, 0)) == 0
	if !proof.EvidenceReviewCleared {
		return fmt.Errorf("VMP IDA ordinary lifecycle acknowledgement did not clear evidence review")
	}
	return assertLiveAcceptanceNoAuthority(caseRoot, lane)
}

func writeLiveAcceptanceVMPIDAIndexes(caseRoot string) error {
	files := map[string]string{
		"function_index.tsv": "rva\tname\tsize\n0x1000\tneedle_dispatch\t32\n0x1100\thelper\t16\n",
		"strings.tsv":        "address\tvalue\n0x2000\tneedle_dispatch marker\n",
		"imports.tsv":        "module\tname\nKERNEL32.dll\tVirtualAlloc\n",
		"xrefs.tsv":          "from\tto\n0x1000\tneedle_dispatch\n",
	}
	for name, content := range files {
		rel := filepath.ToSlash(filepath.Join(adapterhost.VMPIDAIndexDefaultExportRoot, name))
		if err := rekitfs.WriteNewExclusiveRegularFileAnchored(caseRoot, rel, "VMP IDA live acceptance synthetic IDA index", []byte(content)); err != nil {
			return err
		}
	}
	return nil
}

type binaryREVMPIDASelectedEvidence struct {
	IndexName    string
	Source       adapterhost.VMPIDAIndexInputBinding
	Line         int
	Row          string
	MatchedTerms []string
	EvidenceRef  string
}

type binaryREVMPIDAEvidenceReviewInput struct {
	SchemaVersion      int                                   `json:"schemaVersion"`
	Kind               string                                `json:"kind"`
	GateEventID        string                                `json:"gateEventId"`
	ObservationEventID string                                `json:"observationEventId"`
	ProfileSHA256      string                                `json:"profileSha256,omitempty"`
	RequestPath        string                                `json:"requestPath"`
	RequestSHA256      string                                `json:"requestSha256"`
	Sources            []adapterhost.VMPIDAIndexInputBinding `json:"sources"`
	PacketPath         string                                `json:"packetPath"`
	PacketSHA256       string                                `json:"packetSha256"`
	ReportPath         string                                `json:"reportPath"`
	ReportSHA256       string                                `json:"reportSha256"`
	DispatchPath       string                                `json:"dispatchPath"`
	DispatchSHA256     string                                `json:"dispatchSha256"`
	ReceiptPath        string                                `json:"receiptPath"`
	ReceiptSHA256      string                                `json:"receiptSha256"`
	Selected           binaryREVMPIDASelectedEvidence        `json:"selected"`
	EvidenceRefs       []string                              `json:"evidenceRefs"`
	NoAuthority        bool                                  `json:"noAuthorityOrConfirmed"`
	NoHeavyTool        bool                                  `json:"noHeavyTool"`
}

type liveAcceptanceVMPIDASelectedEvidence = binaryREVMPIDASelectedEvidence
type liveAcceptanceVMPIDAEvidenceReviewInput = binaryREVMPIDAEvidenceReviewInput

func requireAcceptedLiveAcceptanceVMPIDAEvidenceReview(decision evidenceReviewResponse) error {
	if err := validateEvidenceReviewResponse(decision); err != nil {
		return err
	}
	if decision.Decision != "accepted" {
		return fmt.Errorf("independent VMP IDA evidence review rejected the recorded evidence: %s", decision.Reason)
	}
	return nil
}

func inspectLiveAcceptanceVMPIDAEvidence(caseRoot, lane string, item mission.ExecutionEvidenceReviewItem, proof *LiveAcceptanceVMPIDA) (liveAcceptanceVMPIDAEvidenceReviewInput, error) {
	if proof == nil {
		return liveAcceptanceVMPIDAEvidenceReviewInput{}, fmt.Errorf("VMP IDA evidence review proof is missing")
	}
	return inspectBinaryREVMPIDAEvidence(caseRoot, lane, item, proof.RequestPath, proof.RequestSHA256, proof.Run)
}

func inspectBinaryREVMPIDAEvidence(
	caseRoot,
	lane string,
	item mission.ExecutionEvidenceReviewItem,
	requestPath,
	requestSHA string,
	run adapterhost.AuthorizedRunResult,
) (binaryREVMPIDAEvidenceReviewInput, error) {
	if item.GateEventID != run.GateEventID || item.EventID != run.ObservationEventID || item.Target != requestPath ||
		item.ExecutionReportPath != run.ReportPath || !strings.EqualFold(item.ExecutionReportSHA256, run.ReportSHA256) ||
		item.AdapterExecutionDispatchPath != run.DispatchPath || !strings.EqualFold(item.AdapterExecutionDispatchSHA256, run.DispatchSHA256) ||
		item.AdapterExecutionReceiptPath != run.ReceiptPath || !strings.EqualFold(item.AdapterExecutionReceiptSHA256, run.ReceiptSHA256) ||
		item.AdapterID != adapterhost.VMPIDAIndexAdapterID || item.AdapterSession != run.AdapterSession {
		return binaryREVMPIDAEvidenceReviewInput{}, fmt.Errorf("VMP IDA evidence review observation lineage drifted")
	}
	request, err := adapterhost.ReadVMPIDAIndexRequest(caseRoot, requestPath)
	if err != nil || request.RequestPath != requestPath || !strings.EqualFold(request.RequestSHA256, requestSHA) {
		return binaryREVMPIDAEvidenceReviewInput{}, fmt.Errorf("VMP IDA evidence review request drifted: %w", err)
	}
	packetData, err := readLiveAcceptanceVMPIDAFile(caseRoot, run.PacketPath, "VMP IDA packet review", adapterhost.VMPIDAIndexMaxPacketBytes)
	if err != nil || !strings.EqualFold(bytesSHA256(packetData), run.PacketSHA256) {
		return binaryREVMPIDAEvidenceReviewInput{}, fmt.Errorf("VMP IDA evidence review packet hash drifted: %w", err)
	}
	var packet adapterhost.VMPIDAIndexPacket
	if err := strictJSON(packetData, &packet); err != nil {
		return binaryREVMPIDAEvidenceReviewInput{}, fmt.Errorf("decode VMP IDA evidence review packet: %w", err)
	}
	canonicalPacket, err := json.MarshalIndent(packet, "", "  ")
	canonicalPacket = append(canonicalPacket, '\n')
	if err != nil || !bytes.Equal(packetData, canonicalPacket) || packet.AdapterID != adapterhost.VMPIDAIndexAdapterID || packet.RequestPath != requestPath || !strings.EqualFold(packet.RequestSHA256, requestSHA) || !reflect.DeepEqual(packet.Sources, request.Request.Inputs) {
		return binaryREVMPIDAEvidenceReviewInput{}, fmt.Errorf("VMP IDA evidence review packet is not canonical or request/source-bound")
	}
	selected, err := selectBinaryREVMPIDARow(caseRoot, packet)
	if err != nil {
		return binaryREVMPIDAEvidenceReviewInput{}, err
	}
	reportData, err := readLiveAcceptanceVMPIDAFile(caseRoot, run.ReportPath, "VMP IDA report review", 1<<20)
	if err != nil || !strings.EqualFold(bytesSHA256(reportData), run.ReportSHA256) {
		return binaryREVMPIDAEvidenceReviewInput{}, fmt.Errorf("VMP IDA evidence review report hash drifted: %w", err)
	}
	var report gate.AdapterReport
	if err := strictJSON(reportData, &report); err != nil || report.AdapterID != adapterhost.VMPIDAIndexAdapterID || report.Status != "succeeded" || report.GateEventID != run.GateEventID || report.Dispatch == nil || report.Dispatch.Path != run.DispatchPath || !strings.EqualFold(report.Dispatch.SHA256, run.DispatchSHA256) || !slices.Equal(report.EvidenceRefs, []string{run.PacketPath}) {
		return binaryREVMPIDAEvidenceReviewInput{}, fmt.Errorf("VMP IDA evidence review report lineage drifted: %w", err)
	}
	receipt, receiptPath, receiptSHA, present, err := gate.ReadAdapterExecutionReceipt(caseRoot, lane, run.GateEventID)
	if err != nil || !present || receipt == nil || receiptPath != run.ReceiptPath || !strings.EqualFold(receiptSHA, run.ReceiptSHA256) {
		return binaryREVMPIDAEvidenceReviewInput{}, fmt.Errorf("VMP IDA evidence review receipt drifted: %w", err)
	}
	if err := validateBinaryREVMPIDAReceipt(*receipt, item, requestPath, run); err != nil {
		return binaryREVMPIDAEvidenceReviewInput{}, err
	}
	return binaryREVMPIDAEvidenceReviewInput{
		SchemaVersion: 1, Kind: "vmp-ida-index-evidence-review", GateEventID: run.GateEventID, ObservationEventID: run.ObservationEventID, ProfileSHA256: receipt.Gate.Authorization.ProfileHash,
		RequestPath: requestPath, RequestSHA256: requestSHA, Sources: append([]adapterhost.VMPIDAIndexInputBinding{}, request.Request.Inputs...),
		PacketPath: run.PacketPath, PacketSHA256: run.PacketSHA256, ReportPath: run.ReportPath, ReportSHA256: run.ReportSHA256,
		DispatchPath: run.DispatchPath, DispatchSHA256: run.DispatchSHA256, ReceiptPath: run.ReceiptPath, ReceiptSHA256: run.ReceiptSHA256,
		Selected: selected, EvidenceRefs: []string{run.PacketPath, run.ReportPath, run.ReceiptPath, selected.EvidenceRef}, NoAuthority: true, NoHeavyTool: true,
	}, nil
}

func selectBinaryREVMPIDARow(caseRoot string, packet adapterhost.VMPIDAIndexPacket) (binaryREVMPIDASelectedEvidence, error) {
	if len(packet.Query.Terms) == 0 {
		return binaryREVMPIDASelectedEvidence{}, fmt.Errorf("VMP IDA packet query has no literal terms")
	}
	for _, index := range packet.Indexes {
		if len(index.Selected) == 0 {
			continue
		}
		sourceData, err := readLiveAcceptanceVMPIDAFile(caseRoot, index.Source.Path, "VMP IDA selected source", adapterhost.VMPIDAIndexMaxInputBytes)
		if err != nil || !strings.EqualFold(bytesSHA256(sourceData), index.Source.SHA256) || int64(len(sourceData)) != index.Source.Bytes {
			return binaryREVMPIDASelectedEvidence{}, fmt.Errorf("VMP IDA selected source drifted: %w", err)
		}
		for _, row := range index.Selected {
			if len(row.MatchedTerms) == 0 {
				return binaryREVMPIDASelectedEvidence{}, fmt.Errorf("VMP IDA selected row has no matched literal terms")
			}
			for _, term := range row.MatchedTerms {
				if !containsFold(packet.Query.Terms, term) || !strings.Contains(strings.ToLower(row.Row), strings.ToLower(term)) {
					return binaryREVMPIDASelectedEvidence{}, fmt.Errorf("VMP IDA selected row contains an unbound matched literal term")
				}
			}
			line, err := liveAcceptanceVMPIDALine(sourceData, row.Line)
			expectedRef := fmt.Sprintf("ida-index:%s:%s#L%d", index.Name, index.Source.Path, row.Line)
			if err != nil || line != row.Row || row.EvidenceRef != expectedRef || !slices.Contains(packet.EvidenceRefs, row.EvidenceRef) {
				return binaryREVMPIDASelectedEvidence{}, fmt.Errorf("VMP IDA selected row does not match its exact source line or evidence ref")
			}
			return binaryREVMPIDASelectedEvidence{
				IndexName: index.Name, Source: index.Source, Line: row.Line, Row: row.Row,
				MatchedTerms: append([]string{}, row.MatchedTerms...), EvidenceRef: row.EvidenceRef,
			}, nil
		}
	}
	return binaryREVMPIDASelectedEvidence{}, fmt.Errorf("VMP IDA packet lacks an exact selected literal evidence ref")
}

func containsFold(items []string, expected string) bool {
	for _, item := range items {
		if strings.EqualFold(item, expected) {
			return true
		}
	}
	return false
}

func liveAcceptanceVMPIDALine(data []byte, lineNumber int) (string, error) {
	if lineNumber < 1 {
		return "", fmt.Errorf("VMP IDA selected line number is invalid")
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 4096), adapterhost.VMPIDAIndexMaxLineBytes+2)
	line := 0
	for scanner.Scan() {
		line++
		if line == lineNumber {
			return strings.TrimSuffix(scanner.Text(), "\r"), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("VMP IDA selected line %d is absent", lineNumber)
}

func validateBinaryREVMPIDAReceipt(
	receipt adapterexecution.Receipt,
	item mission.ExecutionEvidenceReviewItem,
	requestPath string,
	run adapterhost.AuthorizedRunResult,
) error {
	if receipt.Dispatch.Path != run.DispatchPath || !strings.EqualFold(receipt.Dispatch.SHA256, run.DispatchSHA256) ||
		receipt.Gate.GateEventID != run.GateEventID || receipt.Gate.Target != requestPath || receipt.Adapter.AdapterID != adapterhost.VMPIDAIndexAdapterID ||
		receipt.Owner.AdapterSession != run.AdapterSession || receipt.Report.Path != run.ReportPath || !strings.EqualFold(receipt.Report.SHA256, run.ReportSHA256) ||
		len(receipt.Artifacts) != 1 || receipt.Artifacts[0].Path != run.PacketPath || !strings.EqualFold(receipt.Artifacts[0].SHA256, run.PacketSHA256) ||
		!slices.Equal(receipt.Artifacts[0].Roles, []string{"evidence", "output"}) || item.AdapterExecutionArtifactCount != 1 {
		return fmt.Errorf("VMP IDA evidence review receipt does not bind exact dispatch, gate, owner, report, and packet")
	}
	return nil
}

func readLiveAcceptanceVMPIDAFile(caseRoot, rel, label string, limit int64) ([]byte, error) {
	full, err := rekitfs.SafeJoin(caseRoot, rel)
	if err != nil {
		return nil, err
	}
	return rekitfs.ReadStableRegularFileAnchored(caseRoot, full, label, limit)
}

func currentLiveAcceptanceVMPIDAEvidence(caseRoot, lane string, proof *LiveAcceptanceVMPIDA) (liveAcceptanceVMPIDAEvidenceReviewInput, error) {
	if proof == nil || proof.Run.ObservationEventID == "" {
		return liveAcceptanceVMPIDAEvidenceReviewInput{}, fmt.Errorf("VMP IDA live acceptance proof is incomplete")
	}
	facts, err := mission.ReadStrictLedgerFacts(caseRoot)
	if err != nil {
		return liveAcceptanceVMPIDAEvidenceReviewInput{}, err
	}
	var matched *mission.ExecutionEvidenceReviewItem
	for _, observation := range facts.Observations {
		item, ok := mission.ExecutionEvidenceReviewItemFromObservation(observation, lane, nil)
		if !ok || item.EventID != proof.Run.ObservationEventID {
			continue
		}
		if matched != nil {
			return liveAcceptanceVMPIDAEvidenceReviewInput{}, fmt.Errorf("VMP IDA observation identity is not unique")
		}
		copy := item
		matched = &copy
	}
	if matched == nil {
		return liveAcceptanceVMPIDAEvidenceReviewInput{}, fmt.Errorf("VMP IDA current observation lineage is missing")
	}
	return inspectLiveAcceptanceVMPIDAEvidence(caseRoot, lane, *matched, proof)
}

func validateLiveAcceptanceVMPIDAMember(caseRoot string, member memberexecution.Inspection, proof *LiveAcceptanceVMPIDA) error {
	evidence, err := currentLiveAcceptanceVMPIDAEvidence(caseRoot, member.Owner.Lane, proof)
	if err != nil {
		return err
	}
	if err := validateLiveAcceptanceVMPIDAMemberArtifact(member, evidence, proof); err != nil {
		return err
	}
	proof.SelectedEvidenceRef = evidence.Selected.EvidenceRef
	proof.MemberBindingVerified = true
	return nil
}

func liveAcceptanceVMPIDATaskBinding(
	proof *LiveAcceptanceVMPIDA,
	evidence liveAcceptanceVMPIDAEvidenceReviewInput,
) memberexecution.TaskBinding {
	if proof == nil || proof.Lifecycle == nil {
		return memberexecution.TaskBinding{}
	}
	lifecycle := proof.Lifecycle
	return binaryREMemberTaskBinding(
		evidence,
		lifecycle.ExecutionIntentPath,
		lifecycle.ExecutionIntentSHA256,
		lifecycle.ExecutionResultPath,
		lifecycle.ExecutionResultSHA256,
		lifecycle.EvidenceReviewInputPath,
		lifecycle.EvidenceReviewInputSHA256,
		lifecycle.EvidenceReviewIntentPath,
		lifecycle.EvidenceReviewIntentSHA256,
		lifecycle.EvidenceReviewDecisionPath,
		lifecycle.EvidenceReviewDecisionSHA256,
		lifecycle.ClosurePath,
		lifecycle.ClosureSHA256,
		lifecycle.EvidenceReviewSession,
		lifecycle.AcknowledgementEventID,
		lifecycle.AcknowledgementSHA256,
	)
}

func validateLiveAcceptanceVMPIDAMemberArtifact(member memberexecution.Inspection, evidence liveAcceptanceVMPIDAEvidenceReviewInput, proof *LiveAcceptanceVMPIDA) error {
	if proof == nil || proof.Lifecycle == nil || member.TaskContext == nil || member.TaskContext.Binding == nil || member.TaskContext.Binding.Kind != "vmp-ida-index-evidence" || member.Manifest == nil {
		return fmt.Errorf("replacement member task context omitted ordinary vmp-ida-index-evidence lifecycle binding or strict result manifest")
	}
	values := member.TaskContext.Binding.Values
	expected := liveAcceptanceVMPIDATaskBinding(proof, evidence).Values
	if len(values) != len(expected) {
		return fmt.Errorf("replacement member task binding contains an unexpected field set")
	}
	for key, value := range expected {
		if values[key] != value {
			return fmt.Errorf("replacement member task binding %s drifted", key)
		}
	}
	if strings.TrimSpace(member.Manifest.ReviewerItemsPath) == "" {
		return fmt.Errorf("replacement member manifest omitted reviewerItemsPath")
	}
	var reviewerItems []byte
	for _, output := range member.Manifest.Outputs {
		outputPath, joinErr := rekitfs.SafeJoin(member.OutputsRoot, output.Path)
		if joinErr != nil {
			return fmt.Errorf("replacement member declared unsafe output path %q: %w", output.Path, joinErr)
		}
		data, readErr := rekitfs.ReadStableRegularFileAnchored(member.OutputsRoot, outputPath, "VMP IDA live acceptance member output", output.Bytes)
		if readErr != nil || int64(len(data)) != output.Bytes || !strings.EqualFold(bytesSHA256(data), output.SHA256) {
			return fmt.Errorf("replacement member output %q drifted from its strict manifest: %w", output.Path, readErr)
		}
		if filepath.ToSlash(output.Path) == filepath.ToSlash(member.Manifest.ReviewerItemsPath) {
			reviewerItems = data
		}
	}
	if len(reviewerItems) == 0 {
		return fmt.Errorf("replacement member reviewerItemsPath does not resolve to a non-empty current output")
	}
	for label, required := range map[string]string{
		"selected evidence ref": evidence.Selected.EvidenceRef,
		"packet path":           evidence.PacketPath,
		"receipt path":          evidence.ReceiptPath,
		"observation event ID":  evidence.ObservationEventID,
	} {
		if !bytes.Contains(reviewerItems, []byte(required)) {
			return fmt.Errorf("replacement member reviewer items omitted %s", label)
		}
	}
	rowJSON, _ := json.Marshal(evidence.Selected.Row)
	rowEscaped := bytes.Trim(rowJSON, `"`)
	if !bytes.Contains(reviewerItems, []byte(evidence.Selected.Row)) && !bytes.Contains(reviewerItems, rowEscaped) {
		return fmt.Errorf("replacement member reviewer items omitted selected exact row")
	}
	return nil
}

func validateLiveAcceptanceVMPIDAReviewer(caseRoot string, member memberexecution.Inspection, acceptance workstream.MemberReviewerAcceptance, proof *LiveAcceptanceVMPIDA) error {
	if proof == nil || !proof.MemberBindingVerified {
		return fmt.Errorf("VMP IDA Reviewer acceptance did not follow verified member evidence")
	}
	evidence, err := currentLiveAcceptanceVMPIDAEvidence(caseRoot, member.Owner.Lane, proof)
	if err != nil {
		return err
	}
	data, err := rekitfs.ReadStableRegularFileAnchored(caseRoot, acceptance.ReviewerResultInputPath, "VMP IDA live acceptance Reviewer result", reviewerresult.MaxBytes)
	if err != nil {
		return err
	}
	if err := validateLiveAcceptanceVMPIDAReviewerArtifact(caseRoot, member, acceptance, evidence, data); err != nil {
		return err
	}
	proof.ReviewerLineageVerified = true
	return assertLiveAcceptanceNoAuthority(caseRoot, member.Owner.Lane)
}

func validateLiveAcceptanceVMPIDAReviewerArtifact(caseRoot string, member memberexecution.Inspection, acceptance workstream.MemberReviewerAcceptance, evidence liveAcceptanceVMPIDAEvidenceReviewInput, data []byte) error {
	if member.Manifest == nil || member.TaskContext == nil || member.TaskContext.OutputContract == nil || acceptance.OwnerGeneration != member.Owner.ExecutorGeneration || acceptance.OwnerExecutor != member.Owner.Executor || acceptance.ManifestSHA256 != member.ManifestSHA256 || acceptance.RouteID != member.TaskContext.OutputContract.RouteID {
		return fmt.Errorf("VMP IDA Reviewer acceptance did not bind the current member manifest, route, and generation")
	}
	if int64(len(data)) != acceptance.ReviewerResultInputBytes || !strings.EqualFold(bytesSHA256(data), acceptance.ReviewerResultInputSHA256) {
		return fmt.Errorf("VMP IDA Reviewer result input drifted from canonical acceptance")
	}
	result, err := reviewerresult.Decode(data)
	if err != nil {
		return err
	}
	manifestRef := relativeLiveAcceptancePath(caseRoot, member.ManifestPath)
	if result.PacketID != acceptance.PacketID || result.RouteID != acceptance.RouteID || result.ShardID != acceptance.ShardID || result.ReviewerSession != acceptance.ReviewerSession || result.Decision != "accept" || result.RecommendedVerdict != "accepted" || !slices.Contains(result.Items, manifestRef) {
		return fmt.Errorf("VMP IDA Reviewer result did not preserve exact acceptance identity")
	}
	routeOutput, _ := json.Marshal(result.RouteOutput)
	for label, required := range map[string]string{
		"selected evidence ref": evidence.Selected.EvidenceRef,
		"observation event ID":  evidence.ObservationEventID,
	} {
		if !strings.Contains(result.Summary, required) && !bytes.Contains(routeOutput, []byte(required)) {
			return fmt.Errorf("VMP IDA Reviewer result summary/routeOutput omitted %s", label)
		}
	}
	for label, required := range map[string]string{"packet path": evidence.PacketPath, "receipt path": evidence.ReceiptPath} {
		if !slices.Contains(result.EvidenceRefs, required) {
			return fmt.Errorf("VMP IDA Reviewer result evidenceRefs omitted %s", label)
		}
	}
	return nil
}

func assertLiveAcceptanceNoAuthority(caseRoot, lane string) error {
	for _, kind := range []string{"authority", "confirmed"} {
		_, path, err := mission.FactPath(caseRoot, kind)
		if err != nil {
			return fmt.Errorf("resolve VMP IDA live acceptance %s ledger path: %w", kind, err)
		}
		if _, err := os.Lstat(path); err == nil {
			return fmt.Errorf("VMP IDA live acceptance unexpectedly wrote %s ledger state", kind)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("inspect VMP IDA live acceptance %s ledger state: %w", kind, err)
		}
	}
	lane = strings.TrimSpace(lane)
	if lane == "" {
		return fmt.Errorf("VMP IDA live acceptance exact lane is missing")
	}
	profile, _, exists, err := autonomy.Read(caseRoot, lane)
	if err != nil || !exists || !reflect.DeepEqual(profile, autonomy.DefaultProfile(lane)) {
		return fmt.Errorf("VMP IDA live acceptance did not restore the exact manual profile: %w", err)
	}
	return nil
}
