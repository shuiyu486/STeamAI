package sessionhost

import (
	"bufio"
	"bytes"
	"context"
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
	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/gate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewerresult"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

const (
	liveAcceptanceVMPIDAOutputRoot = "captures/feature_analysis/feature-mission/ida-index/session-1"
	liveAcceptanceVMPIDAReportPath = liveAcceptanceVMPIDAOutputRoot + "/adapter-report.json"
	liveAcceptanceVMPIDAQueryTerm  = "needle_dispatch"
)

func prepareLiveAcceptanceVMPIDA(parent context.Context, dailyOpt DailyOptions, caseRoot, pack, lane string, proof *LiveAcceptanceVMPIDA) error {
	if proof == nil || proof.Run.GateEventID != "" {
		return fmt.Errorf("VMP IDA live acceptance adapter proof is missing or already populated")
	}
	if !strings.EqualFold(pack, liveAcceptancePack) || strings.TrimSpace(lane) == "" {
		return fmt.Errorf("VMP IDA live acceptance adapter preparation requires the exact pack and lane")
	}
	repoRoot, err := currentRepoRoot()
	if err != nil {
		return err
	}
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
		"-OutputPaths", liveAcceptanceVMPIDAOutputRoot,
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
		"-OutputPaths", liveAcceptanceVMPIDAOutputRoot,
		"-StopConditions", "scope-drift,source-drift,output-exceeds-bounded-evidence-packet",
	}
	var authorized gate.ApplyResult
	if err := runPublicCLI(gateArgs, &authorized); err != nil || !authorized.Applied || authorized.Event == nil || authorized.Event.Gate.Authorization.Decision != autonomy.DecisionPreauthorized {
		return fmt.Errorf("VMP IDA live acceptance gate was not strictly preauthorized through the public route: %w", err)
	}
	proof.GateEventID = authorized.EventID
	proof.Authorization = authorized.Event.Gate.Authorization.Decision
	runOpt := liveAcceptanceVMPIDARunOptions(repoRoot, caseRoot, lane, proof)
	run, processID, err := adapterhost.RunAuthorizedGateProcess(proof.AdapterPath, runOpt, 20*time.Second)
	if err != nil {
		return err
	}
	if processID <= 0 || !run.ChildLaunched || run.Replay || run.ObservationEventID == "" || run.TaskBindingSHA256 == "" || !run.ProfileRevoked || run.ProfileAlreadyManual || !run.NoNetwork || !run.NoAuthority {
		return fmt.Errorf("VMP IDA live acceptance authorized run omitted child, evidence, binding, or revoke: %+v", run)
	}
	proof.AdapterProcessID = processID
	proof.Run = run
	if err := acknowledgeLiveAcceptanceVMPIDAEvidence(parent, dailyOpt, caseRoot, lane, pack, proof); err != nil {
		return err
	}
	return nil
}

func liveAcceptanceVMPIDARunOptions(repoRoot, caseRoot, lane string, proof *LiveAcceptanceVMPIDA) adapterhost.AuthorizedRunOptions {
	return adapterhost.AuthorizedRunOptions{
		RepoRoot: repoRoot, CaseRoot: caseRoot, Pack: liveAcceptancePack,
		GateEventID: proof.GateEventID, ExecutionReportPath: liveAcceptanceVMPIDAReportPath,
		AdapterSession: "dpc04-vmp-ida-" + lane, Actor: "mission-commander",
	}
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

type liveAcceptanceVMPIDASelectedEvidence struct {
	IndexName    string
	Source       adapterhost.VMPIDAIndexInputBinding
	Line         int
	Row          string
	MatchedTerms []string
	EvidenceRef  string
}

type liveAcceptanceVMPIDAEvidenceReviewInput struct {
	SchemaVersion      int                                   `json:"schemaVersion"`
	Kind               string                                `json:"kind"`
	GateEventID        string                                `json:"gateEventId"`
	ObservationEventID string                                `json:"observationEventId"`
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
	Selected           liveAcceptanceVMPIDASelectedEvidence  `json:"selected"`
	EvidenceRefs       []string                              `json:"evidenceRefs"`
	NoAuthority        bool                                  `json:"noAuthorityOrConfirmed"`
	NoHeavyTool        bool                                  `json:"noHeavyTool"`
}

func requireAcceptedLiveAcceptanceVMPIDAEvidenceReview(decision evidenceReviewResponse) error {
	if err := validateEvidenceReviewResponse(decision); err != nil {
		return err
	}
	if decision.Decision != "accepted" {
		return fmt.Errorf("independent VMP IDA evidence review rejected the recorded evidence: %s", decision.Reason)
	}
	return nil
}

func acknowledgeLiveAcceptanceVMPIDAEvidence(parent context.Context, dailyOpt DailyOptions, caseRoot, lane, pack string, proof *LiveAcceptanceVMPIDA) error {
	facts, err := mission.ReadStrictLedgerFacts(caseRoot)
	if err != nil {
		return err
	}
	items := mission.ExecutionEvidenceReviewItemsWithLedgerFacts(facts, lane, nil, 0)
	if len(items) != 1 || items[0].GateEventID != proof.GateEventID || items[0].Acknowledgement == nil {
		return fmt.Errorf("VMP IDA live acceptance observation did not enter exact evidence review")
	}
	input, err := inspectLiveAcceptanceVMPIDAEvidence(caseRoot, lane, items[0], proof)
	if err != nil {
		return err
	}
	decision, sessionID, err := runLiveAcceptanceVMPIDAEvidenceReview(parent, dailyOpt, caseRoot, input)
	if err != nil {
		return err
	}
	proof.EvidenceReviewSessionID = sessionID
	proof.EvidenceReviewDecision = decision.Decision
	proof.SelectedEvidenceRef = input.Selected.EvidenceRef
	if err := requireAcceptedLiveAcceptanceVMPIDAEvidenceReview(decision); err != nil {
		return err
	}
	createdAt := time.Now().UTC().Format(time.RFC3339Nano)
	ackArgs := []string{
		"-Command", "note", "-Target", caseRoot, "-Pack", pack, "-Format", "json", "-WhatIf",
		"-Kind", "verification", "-Lane", lane,
		"-Subject", "execution evidence review accepted",
		"-Summary", "accepted recorded execution evidence for gateEventId " + proof.GateEventID,
		"-Verifier", "tool-review", "-Verdict", "accepted", "-Status", "resolved",
		"-Related", strings.Join([]string{items[0].EventID, items[0].GateEventID}, ","),
		"-Reason", decision.Summary + "; " + decision.Reason,
		"-TargetRef", proof.RequestPath,
		"-EvidenceRefs", strings.Join(input.EvidenceRefs, ","),
		"-CreatedAt", createdAt,
	}
	var preview publicNoteResult
	if err := runPublicCLI(ackArgs, &preview); err != nil || preview.IsMutation || preview.Applied || preview.EventSHA256 == "" || len(preview.RecordArgs) == 0 {
		return fmt.Errorf("VMP IDA live acceptance evidence acknowledgement preview failed: %w", err)
	}
	var applied publicNoteResult
	if err := runPublicCLI(preview.RecordArgs, &applied); err != nil || !applied.Applied || !strings.EqualFold(applied.EventSHA256, preview.EventSHA256) {
		return fmt.Errorf("VMP IDA live acceptance evidence acknowledgement Apply failed: %w", err)
	}
	proof.AcknowledgementEventID = applied.EventID
	facts, err = mission.ReadStrictLedgerFacts(caseRoot)
	if err != nil {
		return err
	}
	proof.EvidenceReviewCleared = len(mission.ExecutionEvidenceReviewItemsWithLedgerFacts(facts, lane, nil, 0)) == 0
	if !proof.EvidenceReviewCleared {
		return fmt.Errorf("VMP IDA live acceptance evidence acknowledgement did not clear review")
	}
	return assertLiveAcceptanceNoAuthority(caseRoot, lane)
}

func inspectLiveAcceptanceVMPIDAEvidence(caseRoot, lane string, item mission.ExecutionEvidenceReviewItem, proof *LiveAcceptanceVMPIDA) (liveAcceptanceVMPIDAEvidenceReviewInput, error) {
	if proof == nil || item.EventID != proof.Run.ObservationEventID || item.Target != proof.RequestPath ||
		item.ExecutionReportPath != proof.Run.ReportPath || !strings.EqualFold(item.ExecutionReportSHA256, proof.Run.ReportSHA256) ||
		item.AdapterExecutionDispatchPath != proof.Run.DispatchPath || !strings.EqualFold(item.AdapterExecutionDispatchSHA256, proof.Run.DispatchSHA256) ||
		item.AdapterExecutionReceiptPath != proof.Run.ReceiptPath || !strings.EqualFold(item.AdapterExecutionReceiptSHA256, proof.Run.ReceiptSHA256) ||
		item.AdapterID != adapterhost.VMPIDAIndexAdapterID || item.AdapterSession != proof.Run.AdapterSession {
		return liveAcceptanceVMPIDAEvidenceReviewInput{}, fmt.Errorf("VMP IDA evidence review observation lineage drifted")
	}
	request, err := adapterhost.ReadVMPIDAIndexRequest(caseRoot, proof.RequestPath)
	if err != nil || request.RequestPath != proof.RequestPath || !strings.EqualFold(request.RequestSHA256, proof.RequestSHA256) {
		return liveAcceptanceVMPIDAEvidenceReviewInput{}, fmt.Errorf("VMP IDA evidence review request drifted: %w", err)
	}
	packetData, err := readLiveAcceptanceVMPIDAFile(caseRoot, proof.Run.PacketPath, "VMP IDA live acceptance packet review", adapterhost.VMPIDAIndexMaxPacketBytes)
	if err != nil || !strings.EqualFold(bytesSHA256(packetData), proof.Run.PacketSHA256) {
		return liveAcceptanceVMPIDAEvidenceReviewInput{}, fmt.Errorf("VMP IDA evidence review packet hash drifted: %w", err)
	}
	var packet adapterhost.VMPIDAIndexPacket
	if err := strictJSON(packetData, &packet); err != nil {
		return liveAcceptanceVMPIDAEvidenceReviewInput{}, fmt.Errorf("decode VMP IDA evidence review packet: %w", err)
	}
	canonicalPacket, err := json.MarshalIndent(packet, "", "  ")
	canonicalPacket = append(canonicalPacket, '\n')
	if err != nil || !bytes.Equal(packetData, canonicalPacket) || packet.AdapterID != adapterhost.VMPIDAIndexAdapterID || packet.RequestPath != proof.RequestPath || !strings.EqualFold(packet.RequestSHA256, proof.RequestSHA256) || !reflect.DeepEqual(packet.Sources, request.Request.Inputs) {
		return liveAcceptanceVMPIDAEvidenceReviewInput{}, fmt.Errorf("VMP IDA evidence review packet is not canonical or request/source-bound")
	}
	selected, err := selectLiveAcceptanceVMPIDARow(caseRoot, packet)
	if err != nil {
		return liveAcceptanceVMPIDAEvidenceReviewInput{}, err
	}
	reportData, err := readLiveAcceptanceVMPIDAFile(caseRoot, proof.Run.ReportPath, "VMP IDA live acceptance report review", 1<<20)
	if err != nil || !strings.EqualFold(bytesSHA256(reportData), proof.Run.ReportSHA256) {
		return liveAcceptanceVMPIDAEvidenceReviewInput{}, fmt.Errorf("VMP IDA evidence review report hash drifted: %w", err)
	}
	var report gate.AdapterReport
	if err := strictJSON(reportData, &report); err != nil || report.AdapterID != adapterhost.VMPIDAIndexAdapterID || report.Status != "succeeded" || report.GateEventID != proof.GateEventID || report.Dispatch == nil || report.Dispatch.Path != proof.Run.DispatchPath || !strings.EqualFold(report.Dispatch.SHA256, proof.Run.DispatchSHA256) || !slices.Equal(report.EvidenceRefs, []string{proof.Run.PacketPath}) {
		return liveAcceptanceVMPIDAEvidenceReviewInput{}, fmt.Errorf("VMP IDA evidence review report lineage drifted: %w", err)
	}
	receipt, receiptPath, receiptSHA, present, err := gate.ReadAdapterExecutionReceipt(caseRoot, lane, proof.GateEventID)
	if err != nil || !present || receipt == nil || receiptPath != proof.Run.ReceiptPath || !strings.EqualFold(receiptSHA, proof.Run.ReceiptSHA256) {
		return liveAcceptanceVMPIDAEvidenceReviewInput{}, fmt.Errorf("VMP IDA evidence review receipt drifted: %w", err)
	}
	if err := validateLiveAcceptanceVMPIDAReceipt(*receipt, item, proof); err != nil {
		return liveAcceptanceVMPIDAEvidenceReviewInput{}, err
	}
	return liveAcceptanceVMPIDAEvidenceReviewInput{
		SchemaVersion: 1, Kind: "vmp-ida-index-evidence-review", GateEventID: proof.GateEventID, ObservationEventID: proof.Run.ObservationEventID,
		RequestPath: proof.RequestPath, RequestSHA256: proof.RequestSHA256, Sources: append([]adapterhost.VMPIDAIndexInputBinding{}, request.Request.Inputs...),
		PacketPath: proof.Run.PacketPath, PacketSHA256: proof.Run.PacketSHA256, ReportPath: proof.Run.ReportPath, ReportSHA256: proof.Run.ReportSHA256,
		DispatchPath: proof.Run.DispatchPath, DispatchSHA256: proof.Run.DispatchSHA256, ReceiptPath: proof.Run.ReceiptPath, ReceiptSHA256: proof.Run.ReceiptSHA256,
		Selected: selected, EvidenceRefs: []string{proof.Run.PacketPath, proof.Run.ReportPath, proof.Run.ReceiptPath, selected.EvidenceRef}, NoAuthority: true, NoHeavyTool: true,
	}, nil
}

func selectLiveAcceptanceVMPIDARow(caseRoot string, packet adapterhost.VMPIDAIndexPacket) (liveAcceptanceVMPIDASelectedEvidence, error) {
	matches := []liveAcceptanceVMPIDASelectedEvidence{}
	for _, index := range packet.Indexes {
		for _, row := range index.Selected {
			if !slices.Contains(row.MatchedTerms, liveAcceptanceVMPIDAQueryTerm) {
				continue
			}
			sourceData, err := readLiveAcceptanceVMPIDAFile(caseRoot, index.Source.Path, "VMP IDA selected source", adapterhost.VMPIDAIndexMaxInputBytes)
			if err != nil || !strings.EqualFold(bytesSHA256(sourceData), index.Source.SHA256) || int64(len(sourceData)) != index.Source.Bytes {
				return liveAcceptanceVMPIDASelectedEvidence{}, fmt.Errorf("VMP IDA selected source drifted: %w", err)
			}
			line, err := liveAcceptanceVMPIDALine(sourceData, row.Line)
			expectedRef := fmt.Sprintf("ida-index:%s:%s#L%d", index.Name, index.Source.Path, row.Line)
			if err != nil || line != row.Row || row.EvidenceRef != expectedRef || !strings.Contains(strings.ToLower(row.Row), strings.ToLower(liveAcceptanceVMPIDAQueryTerm)) {
				return liveAcceptanceVMPIDASelectedEvidence{}, fmt.Errorf("VMP IDA selected row does not match its exact source line or evidence ref")
			}
			matches = append(matches, liveAcceptanceVMPIDASelectedEvidence{IndexName: index.Name, Source: index.Source, Line: row.Line, Row: row.Row, MatchedTerms: append([]string{}, row.MatchedTerms...), EvidenceRef: row.EvidenceRef})
		}
	}
	if len(matches) < 1 || !slices.Contains(packet.EvidenceRefs, matches[0].EvidenceRef) {
		return liveAcceptanceVMPIDASelectedEvidence{}, fmt.Errorf("VMP IDA packet lacks an exact selected literal evidence ref")
	}
	return matches[0], nil
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

func validateLiveAcceptanceVMPIDAReceipt(receipt adapterexecution.Receipt, item mission.ExecutionEvidenceReviewItem, proof *LiveAcceptanceVMPIDA) error {
	if receipt.Dispatch.Path != proof.Run.DispatchPath || !strings.EqualFold(receipt.Dispatch.SHA256, proof.Run.DispatchSHA256) ||
		receipt.Gate.GateEventID != proof.GateEventID || receipt.Gate.Target != proof.RequestPath || receipt.Adapter.AdapterID != adapterhost.VMPIDAIndexAdapterID ||
		receipt.Owner.AdapterSession != proof.Run.AdapterSession || receipt.Report.Path != proof.Run.ReportPath || !strings.EqualFold(receipt.Report.SHA256, proof.Run.ReportSHA256) ||
		len(receipt.Artifacts) != 1 || receipt.Artifacts[0].Path != proof.Run.PacketPath || !strings.EqualFold(receipt.Artifacts[0].SHA256, proof.Run.PacketSHA256) ||
		!slices.Equal(receipt.Artifacts[0].Roles, []string{"evidence", "output"}) || item.AdapterExecutionArtifactCount != 1 {
		return fmt.Errorf("VMP IDA evidence review receipt does not bind exact dispatch, gate, owner, report, and packet")
	}
	return nil
}

func runLiveAcceptanceVMPIDAEvidenceReview(parent context.Context, dailyOpt DailyOptions, caseRoot string, input liveAcceptanceVMPIDAEvidenceReviewInput) (evidenceReviewResponse, string, error) {
	inputData, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return evidenceReviewResponse{}, "", err
	}
	inputData = append(inputData, '\n')
	inputPath := filepath.ToSlash(filepath.Join(filepath.Dir(input.PacketPath), "evidence-review-input.json"))
	if _, err := rekitfs.WriteExclusiveRegularFileAnchored(caseRoot, inputPath, "VMP IDA evidence review input", inputData); err != nil {
		return evidenceReviewResponse{}, "", err
	}
	sessionID, err := newUUID()
	if err != nil {
		return evidenceReviewResponse{}, "", err
	}
	pkg := mission.CurrentLoopExternalSessionHarnessPackage{
		SchemaVersion: 1, State: "launch-ready", CaseRoot: caseRoot, SessionKind: "mission-commander-evidence-review",
		Launch: &mission.CurrentLoopExternalSessionHarnessLaunch{
			Ready: true, Tool: "Claude Code Agent", AgentType: "read-only-evidence-reviewer", ReadOnly: true,
			Input:          mission.CurrentLoopExternalSessionHarnessInput{Path: inputPath, SHA256: bytesSHA256(inputData), Role: "mission-commander-evidence-review-input"},
			ExpectedOutput: "one strict accepted/rejected evidence review decision bound to the exact selected row, observation, and receipt",
			Attempt:        mission.CurrentLoopExternalSessionAttempt{AttemptID: "evidence-review-" + input.GateEventID, AttemptSHA256: bytesSHA256(inputData), Generation: 1, Harness: defaultHarness, Session: sessionID, Actor: "mission-commander", StartedAt: time.Now().UTC().Format(time.RFC3339Nano)},
		},
	}
	opt := Options{Target: caseRoot, Pack: liveAcceptancePack, Actor: "mission-commander", ClaudePath: dailyOpt.ClaudePath, ExpectedClaudeExecutableSHA256: dailyOpt.ExpectedClaudeExecutableSHA256, ExpectedClaudeExecutablePublisher: dailyOpt.ExpectedClaudeExecutablePublisher, Model: dailyOpt.Model, Timeout: dailyOpt.Timeout, MaxAttempts: 1}
	run := runClaude(parent, opt, pkg, sessionID, nil)
	if !run.success() {
		return evidenceReviewResponse{}, sessionID, fmt.Errorf("independent VMP IDA evidence review failed: %s", run.failureReason())
	}
	if err := validateClaudeStructuredResult(pkg, run); err != nil {
		return evidenceReviewResponse{}, sessionID, err
	}
	var response evidenceReviewResponse
	if err := strictJSON(run.structuredOutput, &response); err != nil {
		return evidenceReviewResponse{}, sessionID, err
	}
	if response.SelectedEvidenceRef != input.Selected.EvidenceRef || response.ObservationEventID != input.ObservationEventID || !strings.EqualFold(response.ReceiptSHA256, input.ReceiptSHA256) || !slices.Equal(response.EvidenceRefs, input.EvidenceRefs) {
		return evidenceReviewResponse{}, sessionID, fmt.Errorf("independent VMP IDA evidence review decision drifted from exact input lineage")
	}
	return response, sessionID, nil
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

func validateLiveAcceptanceVMPIDAMemberArtifact(member memberexecution.Inspection, evidence liveAcceptanceVMPIDAEvidenceReviewInput, proof *LiveAcceptanceVMPIDA) error {
	if proof == nil || member.TaskContext == nil || member.TaskContext.Binding == nil || member.TaskContext.Binding.Kind != "vmp-ida-index-evidence" || member.Manifest == nil {
		return fmt.Errorf("replacement member task context omitted vmp-ida-index-evidence binding or strict result manifest")
	}
	values := member.TaskContext.Binding.Values
	expected := map[string]string{
		"gate-event-id": proof.GateEventID, "profile-hash": proof.ProfileSHA256,
		"request-path": proof.RequestPath, "request-sha256": proof.RequestSHA256,
		"packet-path": proof.Run.PacketPath, "packet-sha256": proof.Run.PacketSHA256,
		"report-path": proof.Run.ReportPath, "report-sha256": proof.Run.ReportSHA256,
		"dispatch-path": proof.Run.DispatchPath, "dispatch-sha256": proof.Run.DispatchSHA256,
		"receipt-path": proof.Run.ReceiptPath, "receipt-sha256": proof.Run.ReceiptSHA256,
		"observation-event-id": proof.Run.ObservationEventID,
	}
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
