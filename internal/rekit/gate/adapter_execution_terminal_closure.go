package gate

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/adapterexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

// AdapterExecutionTerminalClosureOptions binds a failed or aborted report to
// the immutable dispatch and a durable launch proof. This recovery path closes
// an old attempt only; it never validates current execution authorization.
type AdapterExecutionTerminalClosureOptions struct {
	GateEventID                   string
	DispatchPath                  string
	ExpectedDispatchSHA256        string
	ExecutionReportPath           string
	ExpectedExecutionReportSHA256 string
	ExecutionExitStatus           string
	RecoveryProofPath             string
	ExpectedRecoveryProofSHA256   string
	Actor                         string
	ExecutionControlBinding       *executioncontrol.Binding
	ValidateExactArtifacts        func() error
}

type AdapterExecutionTerminalClosureResult struct {
	ReceiptPath        string
	ReceiptSHA256      string
	ObservationEventID string
	ReceiptReplay      bool
	ObservationReplay  bool
}

// RecordAdapterExecutionTerminalClosure records only failed/aborted terminal
// provenance for an already-launched immutable dispatch. Current owner,
// catalog, and autonomy drift cannot turn this API into a new execution grant.
func RecordAdapterExecutionTerminalClosure(
	repoRoot,
	caseRoot,
	pack string,
	opt AdapterExecutionTerminalClosureOptions,
) (_ AdapterExecutionTerminalClosureResult, retErr error) {
	if strings.TrimSpace(opt.Actor) == "" ||
		strings.TrimSpace(opt.ExecutionExitStatus) == "" ||
		!validSHA256String(opt.ExpectedDispatchSHA256) ||
		!validSHA256String(opt.ExpectedExecutionReportSHA256) ||
		!validSHA256String(opt.ExpectedRecoveryProofSHA256) {
		return AdapterExecutionTerminalClosureResult{}, fmt.Errorf("adapter terminal closure requires actor, exit status, and exact dispatch/report/recovery-proof hashes")
	}
	inst, gateEvent, err := authorizedGateEvent(repoRoot, caseRoot, pack, Options{
		GateEventID: strings.TrimSpace(opt.GateEventID),
	})
	if err != nil {
		return AdapterExecutionTerminalClosureResult{}, err
	}
	lease, err := acquireGateLaneMutationLease(inst.CaseRoot, gateEvent.Lane)
	if err != nil {
		return AdapterExecutionTerminalClosureResult{}, err
	}
	defer func() { retErr = errors.Join(retErr, lease.Unlock()) }()

	lockedInst, lockedEvent, err := authorizedGateEvent(repoRoot, caseRoot, pack, Options{
		GateEventID: strings.TrimSpace(opt.GateEventID),
	})
	if err != nil {
		return AdapterExecutionTerminalClosureResult{}, err
	}
	if lockedInst.CaseRoot != inst.CaseRoot || lockedEvent.EventID != gateEvent.EventID || lockedEvent.Lane != gateEvent.Lane {
		return AdapterExecutionTerminalClosureResult{}, fmt.Errorf("adapter terminal closure gate routing changed while acquiring mutation lease")
	}
	gateEvent = lockedEvent
	if err := lease.Validate(); err != nil {
		return AdapterExecutionTerminalClosureResult{}, err
	}
	dispatchRel, dispatchFull, err := adapterExecutionDispatchPath(inst.CaseRoot, gateEvent.Lane, gateEvent.EventID)
	if err != nil {
		return AdapterExecutionTerminalClosureResult{}, err
	}
	if cleanAdapterTerminalPath(opt.DispatchPath) != dispatchRel {
		return AdapterExecutionTerminalClosureResult{}, fmt.Errorf("adapter terminal closure dispatch path must match canonical path: %s", dispatchRel)
	}
	dispatchData, present, err := readAdapterExecutionReceiptRaw(inst.CaseRoot, dispatchFull, dispatchRel)
	if err != nil || !present {
		if err == nil {
			err = fmt.Errorf("immutable dispatch is missing")
		}
		return AdapterExecutionTerminalClosureResult{}, err
	}
	dispatchSHA := adapterexecution.SHA256(dispatchData)
	if !strings.EqualFold(dispatchSHA, opt.ExpectedDispatchSHA256) {
		return AdapterExecutionTerminalClosureResult{}, fmt.Errorf("adapter terminal closure dispatch sha256 changed")
	}
	dispatch, err := adapterexecution.DecodeDispatch(dispatchData)
	if err != nil {
		return AdapterExecutionTerminalClosureResult{}, err
	}
	expectedGate, err := adapterExecutionGateBinding(gateEvent)
	if err != nil {
		return AdapterExecutionTerminalClosureResult{}, err
	}
	if dispatch.Gate.GateEventID != gateEvent.EventID ||
		dispatch.Owner.Lane != gateEvent.Lane ||
		dispatch.Adapter.Pack != pack ||
		!reflect.DeepEqual(dispatch.Gate, expectedGate) {
		return AdapterExecutionTerminalClosureResult{}, fmt.Errorf("adapter terminal closure immutable dispatch does not match the authorized request ledger")
	}
	if err := requireDispatchExecutionControlWithGateLease(inst.CaseRoot, lease, dispatch, opt.ExecutionControlBinding); err != nil {
		return AdapterExecutionTerminalClosureResult{}, fmt.Errorf("adapter terminal closure control is stale: %w", err)
	}
	if opt.ValidateExactArtifacts != nil {
		if err := opt.ValidateExactArtifacts(); err != nil {
			return AdapterExecutionTerminalClosureResult{}, fmt.Errorf("adapter terminal closure exact artifact validation: %w", err)
		}
		if err := lease.Validate(); err != nil {
			return AdapterExecutionTerminalClosureResult{}, err
		}
	}

	reportRel, reportSHA, report, err := readAdapterExecutionReport(
		inst.CaseRoot,
		gateEvent,
		"",
		opt.ExecutionReportPath,
	)
	if err != nil {
		return AdapterExecutionTerminalClosureResult{}, err
	}
	if report == nil || reportRel != dispatch.ReportPath ||
		!strings.EqualFold(reportSHA, opt.ExpectedExecutionReportSHA256) {
		return AdapterExecutionTerminalClosureResult{}, fmt.Errorf("adapter terminal closure report path or sha256 changed")
	}
	if report.Status != "failed" && report.Status != "aborted" {
		return AdapterExecutionTerminalClosureResult{}, fmt.Errorf("adapter terminal closure accepts only failed or aborted reports")
	}
	if len(report.OutputRefs) != 0 || len(report.EvidenceRefs) != 0 {
		return AdapterExecutionTerminalClosureResult{}, fmt.Errorf("adapter terminal closure report must not claim success artifacts")
	}
	if err := validateAdapterReportDispatch(report, dispatch, dispatchRel, dispatchSHA, int64(len(dispatchData))); err != nil {
		return AdapterExecutionTerminalClosureResult{}, err
	}

	proofRel, err := validateCaseRelativePath(inst.CaseRoot, "adapter terminal recovery proof", opt.RecoveryProofPath)
	if err != nil {
		return AdapterExecutionTerminalClosureResult{}, err
	}
	if proofRel == reportRel || !outputRefsWithinGate(gateEvent.Gate.OutputPaths, []string{proofRel}) {
		return AdapterExecutionTerminalClosureResult{}, fmt.Errorf("adapter terminal recovery proof must be a distinct file within authorized outputPaths")
	}
	proofFull := filepath.Join(inst.CaseRoot, filepath.FromSlash(proofRel))
	proof, err := stableFileBinding(inst.CaseRoot, proofFull, proofRel)
	if err != nil {
		return AdapterExecutionTerminalClosureResult{}, err
	}
	if !strings.EqualFold(proof.SHA256, opt.ExpectedRecoveryProofSHA256) ||
		!strings.Contains(strings.ToLower(report.Escalation), strings.ToLower(proof.SHA256)) {
		return AdapterExecutionTerminalClosureResult{}, fmt.Errorf("adapter terminal report does not bind the exact durable recovery proof")
	}

	reportFull, _, err := executionReportPath(inst.CaseRoot, reportRel)
	if err != nil {
		return AdapterExecutionTerminalClosureResult{}, err
	}
	reportBinding, err := stableFileBinding(inst.CaseRoot, reportFull, reportRel)
	if err != nil || !strings.EqualFold(reportBinding.SHA256, reportSHA) {
		return AdapterExecutionTerminalClosureResult{}, fmt.Errorf("adapter terminal report changed while preparing closure: %w", err)
	}
	receipt := adapterexecution.Receipt{
		SchemaVersion: 1,
		Kind:          "adapter-execution-receipt",
		Dispatch: adapterexecution.DispatchBinding{
			DispatchID: dispatch.DispatchID,
			Path:       dispatchRel,
			SHA256:     dispatchSHA,
			Bytes:      int64(len(dispatchData)),
		},
		Gate:    dispatch.Gate,
		Adapter: dispatch.Adapter,
		Owner:   dispatch.Owner,
		Execution: adapterexecution.ExecutionBinding{
			Outcome:          report.Status,
			ExitStatus:       strings.TrimSpace(opt.ExecutionExitStatus),
			AuthorizedBudget: dispatch.Gate.AuthorizedBudget,
			ActualBudget:     report.ActualBudget,
			BoundaryHits:     append([]string{}, report.BoundaryHits...),
			Escalation:       report.Escalation,
		},
		Report:      reportBinding,
		Artifacts:   []adapterexecution.ArtifactBinding{},
		Actor:       strings.TrimSpace(opt.Actor),
		Capability:  dispatch.Capability,
		NoExecute:   true,
		NoAuthority: true,
	}
	bindingSHA, err := adapterexecution.BindingSHA256(receipt)
	if err != nil {
		return AdapterExecutionTerminalClosureResult{}, err
	}
	receipt.ReceiptID = bindingSHA
	if err := adapterexecution.ValidateCompletionDispatchLineage(
		receipt,
		dispatch,
		dispatchRel,
		dispatchSHA,
		int64(len(dispatchData)),
	); err != nil {
		return AdapterExecutionTerminalClosureResult{}, err
	}

	receiptRel, receiptFull, err := adapterExecutionReceiptPath(inst.CaseRoot, gateEvent.Lane, gateEvent.EventID)
	if err != nil {
		return AdapterExecutionTerminalClosureResult{}, err
	}
	result := AdapterExecutionTerminalClosureResult{ReceiptPath: receiptRel}
	existing, receiptPresent, err := readAdapterExecutionReceiptRaw(inst.CaseRoot, receiptFull, receiptRel)
	if err != nil {
		return result, err
	}
	if receiptPresent {
		recorded, decodeErr := adapterexecution.Decode(existing)
		if decodeErr != nil {
			return result, fmt.Errorf("existing adapter terminal receipt is invalid: %w", decodeErr)
		}
		if !adapterexecution.SemanticEqual(recorded, receipt) {
			return result, fmt.Errorf("existing adapter terminal receipt differs from the terminal report")
		}
		receipt = recorded
		result.ReceiptReplay = true
		result.ReceiptSHA256 = adapterexecution.SHA256(existing)
	} else {
		receipt.RecordedAt = time.Now().UTC().Format(time.RFC3339Nano)
		data, encodeErr := adapterexecution.ReceiptBytes(receipt)
		if encodeErr != nil {
			return result, encodeErr
		}
		if err := lease.Validate(); err != nil {
			return result, err
		}
		if err := writeAdapterExecutionReceipt(inst.CaseRoot, receiptFull, receiptRel, data); err != nil {
			return result, err
		}
		written, writtenPresent, readErr := readAdapterExecutionReceiptRaw(inst.CaseRoot, receiptFull, receiptRel)
		if readErr != nil || !writtenPresent || !bytes.Equal(written, data) {
			if readErr == nil {
				readErr = fmt.Errorf("adapter terminal receipt changed after write")
			}
			return result, readErr
		}
		result.ReceiptSHA256 = adapterexecution.SHA256(written)
	}

	adapterContext := adapterToolCandidateFromDispatch(dispatch.Adapter.Candidate)
	observation := ExecutionEvidencePreview{
		SchemaVersion: 1,
		Kind:          "observation",
		Lane:          gateEvent.Lane,
		Subject:       "terminal closure for " + mission.Subject(gateEventMap(gateEvent)),
		Summary:       report.Summary,
		Status:        report.Status,
		Actor:         strings.TrimSpace(opt.Actor),
		Risk:          gateEvent.Risk,
		Target:        gateEvent.Target,
		BatchID:       gateEvent.BatchID,
		Related:       []string{gateEvent.EventID},
		Gate:          gateEvent.Gate,
		Execution: ExecutionEvidenceDetails{
			Status:                         report.Status,
			ActualBudget:                   report.ActualBudget,
			BoundaryHits:                   append([]string{}, report.BoundaryHits...),
			Escalation:                     report.Escalation,
			GateEventID:                    gateEvent.EventID,
			GateStatus:                     gateEvent.Status,
			Authorization:                  dispatch.Gate.Authorization.Decision,
			RecordRequired:                 dispatch.Gate.Authorization.RecordRequired,
			NotifyMainOn:                   append([]string{}, dispatch.Gate.Authorization.NotifyMainOn...),
			ExecutionReportPath:            reportRel,
			ExecutionReportSHA256:          reportSHA,
			AdapterExecutionDispatchPath:   dispatchRel,
			AdapterExecutionDispatchSHA256: dispatchSHA,
			AdapterExecutionDispatch:       &dispatch,
			AdapterExecutionReceiptPath:    receiptRel,
			AdapterExecutionReceiptSHA256:  result.ReceiptSHA256,
			AdapterExecution:               &receipt,
			AdapterContext:                 &adapterContext,
			Adapter:                        report,
		},
	}
	observation.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	observation.EventID = executionEventID(observation)
	result.ObservationEventID = observation.EventID
	facts, err := mission.ReadStrictFact(inst.CaseRoot, "observation")
	if err != nil {
		return result, err
	}
	for _, fact := range facts {
		if mission.Value(fact, "eventId") == observation.EventID {
			result.ObservationReplay = true
			return result, nil
		}
	}
	if err := lease.Validate(); err != nil {
		return result, err
	}
	if _, _, err := mission.AppendFact(inst.CaseRoot, "observation", observation); err != nil {
		return result, err
	}
	if err := lease.Validate(); err != nil {
		return result, err
	}
	return result, nil
}

func cleanAdapterTerminalPath(value string) string {
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(strings.TrimSpace(value))))
	if clean == "." {
		return ""
	}
	return clean
}

func adapterToolCandidateFromDispatch(candidate adapterexecution.Candidate) AdapterToolCandidate {
	return AdapterToolCandidate{
		ID:                  candidate.ID,
		Status:              candidate.Status,
		Entry:               candidate.Entry,
		Purpose:             candidate.Purpose,
		SideEffects:         append([]string{}, candidate.SideEffects...),
		GateActions:         append([]string{}, candidate.GateActions...),
		ToolingCatalogPath:  candidate.ToolingCatalogPath,
		StopConditionHints:  append([]string{}, candidate.StopConditionHints...),
		RecordOnlyAfterGate: candidate.RecordOnlyAfterGate,
	}
}
