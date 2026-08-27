package gate

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/adapterexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/capabilitycontract"
	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/laneowner"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

var adapterExecutionSegmentPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

type AdapterExecutionDispatchResult struct {
	SchemaVersion         int                              `json:"schemaVersion"`
	Command               string                           `json:"command"`
	Kind                  string                           `json:"kind"`
	CaseRoot              string                           `json:"caseRoot"`
	RepoRoot              string                           `json:"repoRoot"`
	Pack                  string                           `json:"pack"`
	IsMutation            bool                             `json:"isMutation"`
	Applied               bool                             `json:"applied"`
	Replay                bool                             `json:"replay,omitempty"`
	GateEventID           string                           `json:"gateEventId"`
	DispatchPath          string                           `json:"dispatchPath"`
	BindingSHA256         string                           `json:"bindingSha256"`
	DispatchSHA256        string                           `json:"dispatchSha256,omitempty"`
	ExpectedBindingSHA256 string                           `json:"expectedBindingSha256,omitempty"`
	Dispatch              adapterexecution.DispatchReceipt `json:"dispatch"`
	ApplyCommand          string                           `json:"applyCommand,omitempty"`
	Boundary              []string                         `json:"boundary"`
	NextSteps             []string                         `json:"nextSteps"`
}

type adapterExecutionDispatchSnapshot struct {
	dispatch     adapterexecution.DispatchReceipt
	bindingSHA   string
	dispatchRel  string
	dispatchFull string
}

type AdapterExecutionReceiptResult struct {
	SchemaVersion         int                      `json:"schemaVersion"`
	Command               string                   `json:"command"`
	Kind                  string                   `json:"kind"`
	CaseRoot              string                   `json:"caseRoot"`
	RepoRoot              string                   `json:"repoRoot"`
	Pack                  string                   `json:"pack"`
	IsMutation            bool                     `json:"isMutation"`
	Applied               bool                     `json:"applied"`
	Replay                bool                     `json:"replay,omitempty"`
	GateEventID           string                   `json:"gateEventId"`
	ReceiptPath           string                   `json:"receiptPath"`
	BindingSHA256         string                   `json:"bindingSha256"`
	ReceiptSHA256         string                   `json:"receiptSha256,omitempty"`
	ExpectedBindingSHA256 string                   `json:"expectedBindingSha256,omitempty"`
	Receipt               adapterexecution.Receipt `json:"receipt"`
	ApplyCommand          string                   `json:"applyCommand,omitempty"`
	ValidateCommand       string                   `json:"validateCommand"`
	Boundary              []string                 `json:"boundary"`
	NextSteps             []string                 `json:"nextSteps"`
}

type adapterExecutionSnapshot struct {
	dispatch    adapterexecution.DispatchReceipt
	receipt     adapterexecution.Receipt
	bindingSHA  string
	receiptRel  string
	receiptFull string
}

func RecordAdapterExecutionDispatch(repoRoot, caseRoot, pack string, opt Options) (_ AdapterExecutionDispatchResult, retErr error) {
	inst, gateEvent, err := authorizedGateEvent(repoRoot, caseRoot, pack, opt)
	if err != nil {
		return AdapterExecutionDispatchResult{}, err
	}
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return AdapterExecutionDispatchResult{}, err
	}
	snapshot, err := prepareAdapterExecutionDispatchSnapshot(inst.CaseRoot, pack, gateEvent, opt, m)
	if err != nil {
		return AdapterExecutionDispatchResult{}, err
	}
	apply := strings.TrimSpace(opt.ExpectedAdapterExecutionDispatchBindingSHA256) != ""
	lockedOpt := opt
	if lockedOpt.ExecutionControlBinding == nil {
		lockedOpt.ExecutionControlBinding = executioncontrol.CloneBinding(snapshot.dispatch.LaunchControl)
	}
	var lease gateLaneMutationLease
	if apply {
		lease, err = acquireGateLaneMutationLease(inst.CaseRoot, gateEvent.Lane)
		if err != nil {
			return AdapterExecutionDispatchResult{}, err
		}
		defer func() { retErr = errors.Join(retErr, lease.Unlock()) }()
		lockedInst, lockedGateEvent, lockedErr := authorizedGateEvent(repoRoot, caseRoot, pack, lockedOpt)
		if lockedErr != nil {
			return AdapterExecutionDispatchResult{}, lockedErr
		}
		if lockedInst.CaseRoot != inst.CaseRoot || lockedGateEvent.EventID != gateEvent.EventID || lockedGateEvent.Lane != gateEvent.Lane {
			return AdapterExecutionDispatchResult{}, fmt.Errorf("authorized gate routing changed while acquiring mutation lease")
		}
		lockedManifest, lockedErr := manifest.Load(repoRoot, pack)
		if lockedErr != nil {
			return AdapterExecutionDispatchResult{}, lockedErr
		}
		snapshot, lockedErr = prepareAdapterExecutionDispatchSnapshot(lockedInst.CaseRoot, pack, lockedGateEvent, lockedOpt, lockedManifest)
		if lockedErr != nil {
			return AdapterExecutionDispatchResult{}, lockedErr
		}
		if lockedErr = lease.Validate(); lockedErr != nil {
			return AdapterExecutionDispatchResult{}, lockedErr
		}
		if lockedErr = requireDispatchExecutionControlWithGateLease(inst.CaseRoot, lease, snapshot.dispatch, lockedOpt.ExecutionControlBinding); lockedErr != nil {
			return AdapterExecutionDispatchResult{}, fmt.Errorf("adapter execution dispatch control is stale: %w", lockedErr)
		}
	}
	result := AdapterExecutionDispatchResult{
		SchemaVersion: 1, Command: "gate", Kind: "adapter-execution-dispatch-result",
		CaseRoot: inst.CaseRoot, RepoRoot: repoRoot, Pack: pack,
		IsMutation: apply, Applied: false, GateEventID: gateEvent.EventID,
		DispatchPath: snapshot.dispatchRel, BindingSHA256: snapshot.bindingSHA,
		Dispatch: snapshot.dispatch,
		Boundary: []string{
			"dispatch records external harness execution intent before the adapter starts; Mission Control runtime does not execute the adapter or heavy tool",
			"dispatch is immutable and bound to current lane owner, selected catalog candidate, harness/session, report path, authorized gate, and budget",
			"dispatch write does not append observation evidence or write authority/confirmed",
			"takeover, session, catalog, gate, or attempt drift requires a distinct authorized gate and dispatch",
		},
	}
	result.ApplyCommand = adapterExecutionDispatchApplySlashCommand(pack, snapshot.dispatch, snapshot.bindingSHA)
	result.NextSteps = []string{"review the pre-execution dispatch binding", "record dispatch with the hash-bound Apply command before starting the external adapter", "only the exact dispatch-bound harness/session may produce the report and completion receipt"}
	if !apply {
		return result, nil
	}
	if !validSHA256String(opt.ExpectedAdapterExecutionDispatchBindingSHA256) {
		return AdapterExecutionDispatchResult{}, fmt.Errorf("gate adapter execution dispatch requires a valid -ExpectedAdapterExecutionDispatchBindingSha256 from preview")
	}
	if !strings.EqualFold(opt.ExpectedAdapterExecutionDispatchBindingSHA256, snapshot.bindingSHA) {
		return AdapterExecutionDispatchResult{}, fmt.Errorf("adapter execution dispatch binding changed after preview: expected %s got %s", opt.ExpectedAdapterExecutionDispatchBindingSHA256, snapshot.bindingSHA)
	}
	existing, present, err := readAdapterExecutionReceiptRaw(inst.CaseRoot, snapshot.dispatchFull, snapshot.dispatchRel)
	if err != nil {
		return AdapterExecutionDispatchResult{}, err
	}
	if present {
		if err := lease.Validate(); err != nil {
			return AdapterExecutionDispatchResult{}, err
		}
		recorded, err := adapterexecution.DecodeDispatch(existing)
		if err != nil {
			return AdapterExecutionDispatchResult{}, fmt.Errorf("existing adapter execution dispatch receipt is invalid: %w", err)
		}
		if !adapterexecution.DispatchSemanticEqual(recorded, snapshot.dispatch) {
			return AdapterExecutionDispatchResult{}, fmt.Errorf("adapter execution dispatch target already exists with different semantic bindings: %s", snapshot.dispatchRel)
		}
		result.Applied = true
		result.Replay = true
		result.ApplyCommand = ""
		result.Dispatch = recorded
		result.DispatchSHA256 = adapterexecution.SHA256(existing)
		result.NextSteps = []string{"exact dispatch already exists; bytes were not rewritten", "do not start a second adapter execution for this dispatch", "complete or inspect the exact bound harness/session attempt"}
		return result, nil
	}
	snapshot.dispatch.RecordedAt = time.Now().UTC().Format(time.RFC3339Nano)
	data, err := adapterexecution.DispatchReceiptBytes(snapshot.dispatch)
	if err != nil {
		return AdapterExecutionDispatchResult{}, err
	}
	if err := writeAdapterExecutionReceipt(inst.CaseRoot, snapshot.dispatchFull, snapshot.dispatchRel, data); err != nil {
		return AdapterExecutionDispatchResult{}, err
	}
	written, present, err := readAdapterExecutionReceiptRaw(inst.CaseRoot, snapshot.dispatchFull, snapshot.dispatchRel)
	if err != nil || !present {
		if err == nil {
			err = fmt.Errorf("adapter execution dispatch disappeared after write")
		}
		return AdapterExecutionDispatchResult{}, err
	}
	if !bytes.Equal(written, data) {
		return AdapterExecutionDispatchResult{}, fmt.Errorf("adapter execution dispatch bytes changed after write: %s", snapshot.dispatchRel)
	}
	if err := lease.Validate(); err != nil {
		return AdapterExecutionDispatchResult{}, fmt.Errorf("adapter execution dispatch may already be durable at %s; mutation lease validation failed after create: %w", snapshot.dispatchRel, err)
	}
	result.Applied = true
	result.ApplyCommand = ""
	result.Dispatch = snapshot.dispatch
	result.DispatchSHA256 = adapterexecution.SHA256(written)
	result.NextSteps = []string{"immutable pre-execution dispatch recorded", "start only the exact bound external harness/session", "completion report and receipt must hash-link this dispatch"}
	return result, nil
}

func adapterExecutionLaunchControl(
	caseRoot string,
	owner laneowner.Snapshot,
	capability capabilitycontract.Binding,
	provided *executioncontrol.Binding,
) (*executioncontrol.Binding, error) {
	stateRoot, err := projectstate.Resolve(caseRoot)
	if err != nil {
		return nil, err
	}
	if stateRoot.Legacy && provided == nil {
		return nil, nil
	}
	if provided != nil {
		if err := executioncontrol.ValidateBinding(*provided); err != nil {
			return nil, err
		}
		if provided.Lane != owner.Lane || provided.Owner != owner || provided.Capability != capability {
			return nil, fmt.Errorf("adapter execution control does not match current lane owner and capability")
		}
		return executioncontrol.CloneBinding(provided), nil
	}
	binding, err := executioncontrol.CaptureBinding(caseRoot, owner, capability)
	if err != nil {
		return nil, fmt.Errorf("capture adapter execution launch control: %w", err)
	}
	return &binding, nil
}

func prepareAdapterExecutionDispatchSnapshot(caseRoot, pack string, gateEvent EventPreview, opt Options, m *manifest.Manifest) (adapterExecutionDispatchSnapshot, error) {
	owner, err := laneowner.Read(caseRoot, gateEvent.Lane)
	if err != nil {
		return adapterExecutionDispatchSnapshot{}, err
	}
	if strings.TrimSpace(opt.Executor) == "" || opt.ExpectedExecutorGeneration <= 0 {
		return adapterExecutionDispatchSnapshot{}, fmt.Errorf("gate adapter execution dispatch requires -Executor and -ExpectedExecutorGeneration for the current durable lane owner")
	}
	if opt.Executor != owner.CurrentExecutor || opt.ExpectedExecutorGeneration != owner.ExecutorGeneration {
		return adapterExecutionDispatchSnapshot{}, fmt.Errorf("adapter execution dispatch owner is stale: current executor=%s generation=%d", owner.CurrentExecutor, owner.ExecutorGeneration)
	}
	if strings.TrimSpace(opt.AdapterID) == "" || strings.TrimSpace(opt.AdapterHarness) == "" || strings.TrimSpace(opt.AdapterSession) == "" || strings.TrimSpace(opt.Actor) == "" {
		return adapterExecutionDispatchSnapshot{}, fmt.Errorf("gate adapter execution dispatch requires -AdapterId, -AdapterHarness, -AdapterSession, and -Actor")
	}
	for label, value := range map[string]string{"AdapterHarness": opt.AdapterHarness, "AdapterSession": opt.AdapterSession, "Actor": opt.Actor} {
		if len(strings.TrimSpace(value)) > 256 || strings.ContainsAny(value, "\r\n") {
			return adapterExecutionDispatchSnapshot{}, fmt.Errorf("adapter execution dispatch %s must be a bounded single-line value", label)
		}
	}
	reportPath := strings.TrimSpace(opt.ExecutionReportPath)
	if reportPath == "" {
		reportPath = adapterReportDefaultPath(gateEvent.Gate.OutputPaths)
	}
	reportFull, reportRel, err := executionReportPath(caseRoot, reportPath)
	if err == nil && !outputRefsWithinGate(gateEvent.Gate.OutputPaths, []string{reportRel}) {
		if cwdFull, cwdRel, ok, cwdErr := cwdAuthorizedExecutionReportPath(caseRoot, gateEvent, opt.ExecutionReportCwd, reportPath); cwdErr != nil {
			return adapterExecutionDispatchSnapshot{}, cwdErr
		} else if ok {
			reportFull = cwdFull
			reportRel = cwdRel
		} else {
			err = fmt.Errorf("report path is outside authorized outputPaths")
		}
	}
	if err != nil || !outputRefsWithinGate(gateEvent.Gate.OutputPaths, []string{reportRel}) {
		return adapterExecutionDispatchSnapshot{}, fmt.Errorf("adapter execution dispatch report path must stay within authorized gate outputPaths")
	}
	dispatchRel, dispatchFull, err := adapterExecutionDispatchPath(caseRoot, gateEvent.Lane, gateEvent.EventID)
	if err != nil {
		return adapterExecutionDispatchSnapshot{}, err
	}
	if reportData, reportPresent, reportErr := readAdapterReportRaw(caseRoot, reportFull, reportRel); reportErr != nil {
		return adapterExecutionDispatchSnapshot{}, reportErr
	} else if reportPresent {
		_, dispatchPresent, dispatchErr := readAdapterExecutionReceiptRaw(caseRoot, dispatchFull, dispatchRel)
		if dispatchErr != nil {
			return adapterExecutionDispatchSnapshot{}, dispatchErr
		}
		if !dispatchPresent {
			contract := adapterReportContract("", caseRoot, pack, gateEvent, m)
			scaffold, scaffoldErr := adapterReportScaffoldBytes(contract.LiveValidation.SidecarTemplate)
			if scaffoldErr != nil || !bytes.Equal(reportData, scaffold) {
				return adapterExecutionDispatchSnapshot{}, fmt.Errorf("adapter execution dispatch must be recorded before the external execution report exists")
			}
		}
	}
	candidate, catalogSHA, catalogBytes, err := strictAdapterCandidateSnapshot(m, gateEvent, opt.AdapterID)
	if err != nil {
		return adapterExecutionDispatchSnapshot{}, err
	}
	candidateBinding := adapterExecutionCandidate(candidate)
	candidateSHA, err := adapterexecution.CandidateSHA256(candidateBinding)
	if err != nil {
		return adapterExecutionDispatchSnapshot{}, err
	}
	gateBinding, err := adapterExecutionGateBinding(gateEvent)
	if err != nil {
		return adapterExecutionDispatchSnapshot{}, err
	}
	capability, err := capabilitycontract.Bind(capabilitycontract.AuthorizedHeavy())
	if err != nil {
		return adapterExecutionDispatchSnapshot{}, err
	}
	launchControl, err := adapterExecutionLaunchControl(caseRoot, owner, capability, opt.ExecutionControlBinding)
	if err != nil {
		return adapterExecutionDispatchSnapshot{}, err
	}
	dispatch := adapterexecution.DispatchReceipt{
		SchemaVersion: 1, Kind: "adapter-execution-dispatch-receipt", Gate: gateBinding,
		Adapter:       adapterexecution.AdapterBinding{Pack: pack, AdapterID: candidate.ID, ToolingCatalogPath: candidate.ToolingCatalogPath, ToolingCatalogSHA256: catalogSHA, ToolingCatalogBytes: catalogBytes, Candidate: candidateBinding, CandidateSnapshotSHA256: candidateSHA},
		Owner:         adapterexecution.OwnerBinding{Lane: owner.Lane, CurrentExecutor: owner.CurrentExecutor, ExecutorGeneration: owner.ExecutorGeneration, AdapterHarness: strings.TrimSpace(opt.AdapterHarness), AdapterSession: strings.TrimSpace(opt.AdapterSession), BindingMode: "durable-lane-owner"},
		ReportPath:    reportRel,
		Actor:         strings.TrimSpace(opt.Actor),
		Capability:    capability,
		LaunchControl: launchControl,
		NoExecute:     true,
		NoObservation: true,
		NoAuthority:   true,
	}
	bindingSHA, err := adapterexecution.DispatchBindingSHA256(dispatch)
	if err != nil {
		return adapterExecutionDispatchSnapshot{}, err
	}
	dispatch.DispatchID = bindingSHA
	rel, full, err := adapterExecutionDispatchPath(caseRoot, gateEvent.Lane, gateEvent.EventID)
	if err != nil {
		return adapterExecutionDispatchSnapshot{}, err
	}
	return adapterExecutionDispatchSnapshot{dispatch: dispatch, bindingSHA: bindingSHA, dispatchRel: rel, dispatchFull: full}, nil
}

func adapterExecutionDispatchPreviewSlashCommand(pack string, gateEvent EventPreview, reportPath, adapterID string) string {
	return adapterReportSlashCommand([]string{"gate", "-Pack", pack, "-GateEventId", gateEvent.EventID, "-RecordAdapterExecutionDispatch", "-ExecutionReportPath", reportPath, "-AdapterId", adapterID, "-Executor", "<current-executor>", "-ExpectedExecutorGeneration", "<current-generation>", "-AdapterHarness", "<harness>", "-AdapterSession", "<session>", "-Actor", "<recorded-by>", "-Format", "json"})
}

func adapterExecutionDispatchApplySlashCommand(pack string, dispatch adapterexecution.DispatchReceipt, bindingSHA string) string {
	args := []string{"gate", "-Pack", pack, "-GateEventId", dispatch.Gate.GateEventID, "-RecordAdapterExecutionDispatch", "-ExecutionReportPath", dispatch.ReportPath, "-AdapterId", dispatch.Adapter.AdapterID, "-Executor", dispatch.Owner.CurrentExecutor, "-ExpectedExecutorGeneration", fmt.Sprintf("%d", dispatch.Owner.ExecutorGeneration), "-AdapterHarness", dispatch.Owner.AdapterHarness, "-AdapterSession", dispatch.Owner.AdapterSession, "-Actor", dispatch.Actor, "-ExpectedAdapterExecutionDispatchBindingSha256", bindingSHA, "-Apply", "-Format", "json"}
	return adapterReportSlashCommand(args)
}

func RecordAdapterExecutionReceipt(repoRoot, caseRoot, pack string, opt Options) (_ AdapterExecutionReceiptResult, retErr error) {
	inst, gateEvent, err := authorizedGateEvent(repoRoot, caseRoot, pack, opt)
	if err != nil {
		return AdapterExecutionReceiptResult{}, err
	}
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return AdapterExecutionReceiptResult{}, err
	}
	snapshot, err := prepareAdapterExecutionSnapshot(inst.CaseRoot, pack, gateEvent, opt, m)
	if err != nil {
		return AdapterExecutionReceiptResult{}, err
	}
	apply := strings.TrimSpace(opt.ExpectedAdapterExecutionBindingSHA256) != ""
	var lease gateLaneMutationLease
	if apply {
		var leaseErr error
		lease, leaseErr = acquireGateLaneMutationLease(inst.CaseRoot, gateEvent.Lane)
		if leaseErr != nil {
			return AdapterExecutionReceiptResult{}, leaseErr
		}
		defer func() { retErr = errors.Join(retErr, lease.Unlock()) }()
		lockedInst, lockedGateEvent, lockedErr := authorizedGateEvent(repoRoot, caseRoot, pack, opt)
		if lockedErr != nil {
			return AdapterExecutionReceiptResult{}, lockedErr
		}
		if lockedInst.CaseRoot != inst.CaseRoot || lockedGateEvent.EventID != gateEvent.EventID || lockedGateEvent.Lane != gateEvent.Lane {
			return AdapterExecutionReceiptResult{}, fmt.Errorf("authorized gate routing changed while acquiring mutation lease")
		}
		lockedManifest, lockedErr := manifest.Load(repoRoot, pack)
		if lockedErr != nil {
			return AdapterExecutionReceiptResult{}, lockedErr
		}
		snapshot, lockedErr = prepareAdapterExecutionSnapshot(lockedInst.CaseRoot, pack, lockedGateEvent, opt, lockedManifest)
		if lockedErr != nil {
			return AdapterExecutionReceiptResult{}, lockedErr
		}
		if lockedErr = lease.Validate(); lockedErr != nil {
			return AdapterExecutionReceiptResult{}, lockedErr
		}
		if lockedErr = requireDispatchExecutionControlWithGateLease(inst.CaseRoot, lease, snapshot.dispatch, opt.ExecutionControlBinding); lockedErr != nil {
			return AdapterExecutionReceiptResult{}, fmt.Errorf("adapter execution receipt control is stale: %w", lockedErr)
		}
	}
	result := AdapterExecutionReceiptResult{
		SchemaVersion:   1,
		Command:         "gate",
		Kind:            "adapter-execution-receipt-result",
		CaseRoot:        inst.CaseRoot,
		RepoRoot:        repoRoot,
		Pack:            pack,
		IsMutation:      apply,
		Applied:         false,
		GateEventID:     gateEvent.EventID,
		ReceiptPath:     snapshot.receiptRel,
		BindingSHA256:   snapshot.bindingSHA,
		Receipt:         snapshot.receipt,
		ValidateCommand: adapterReportValidateSlashCommand(pack, gateEvent.EventID, snapshot.receipt.Report.Path),
		Boundary: []string{
			"receipt records external harness observation only; Mission Control runtime does not execute the adapter or heavy tool",
			"receipt is immutable and bound to current lane owner, selected catalog candidate, report, and artifact bytes",
			"receipt write does not append observation evidence or write authority/confirmed",
			"takeover or catalog/report/artifact drift requires a new authorized gate rather than receipt adoption",
		},
	}
	result.ApplyCommand = adapterExecutionReceiptApplySlashCommand(pack, snapshot.receipt, snapshot.bindingSHA)
	result.NextSteps = []string{"review the semantic binding and exact hashes", "record the immutable receipt with the hash-bound Apply command", "rerun read-only report validation before recording observation evidence"}
	if !apply {
		return result, nil
	}
	if !validSHA256String(opt.ExpectedAdapterExecutionBindingSHA256) {
		return AdapterExecutionReceiptResult{}, fmt.Errorf("gate adapter execution receipt requires a valid -ExpectedAdapterExecutionBindingSha256 from preview")
	}
	if !strings.EqualFold(opt.ExpectedAdapterExecutionBindingSHA256, snapshot.bindingSHA) {
		return AdapterExecutionReceiptResult{}, fmt.Errorf("adapter execution receipt binding changed after preview: expected %s got %s", opt.ExpectedAdapterExecutionBindingSHA256, snapshot.bindingSHA)
	}
	existing, present, err := readAdapterExecutionReceiptRaw(inst.CaseRoot, snapshot.receiptFull, snapshot.receiptRel)
	if err != nil {
		return AdapterExecutionReceiptResult{}, err
	}
	if present {
		if err := lease.Validate(); err != nil {
			return AdapterExecutionReceiptResult{}, err
		}
		recorded, err := adapterexecution.Decode(existing)
		if err != nil {
			return AdapterExecutionReceiptResult{}, fmt.Errorf("existing adapter execution receipt is invalid: %w", err)
		}
		if !adapterexecution.SemanticEqual(recorded, snapshot.receipt) {
			return AdapterExecutionReceiptResult{}, fmt.Errorf("adapter execution receipt target already exists with different semantic bindings: %s", snapshot.receiptRel)
		}
		result.Applied = true
		result.Replay = true
		result.ApplyCommand = ""
		result.Receipt = recorded
		result.ReceiptSHA256 = adapterexecution.SHA256(existing)
		result.NextSteps = []string{"exact receipt already exists; bytes were not rewritten", "rerun read-only report validation", "do not rerun the adapter solely because receipt Apply was replayed"}
		return result, nil
	}
	snapshot.receipt.RecordedAt = time.Now().UTC().Format(time.RFC3339Nano)
	data, err := adapterexecution.ReceiptBytes(snapshot.receipt)
	if err != nil {
		return AdapterExecutionReceiptResult{}, err
	}
	if err := writeAdapterExecutionReceipt(inst.CaseRoot, snapshot.receiptFull, snapshot.receiptRel, data); err != nil {
		return AdapterExecutionReceiptResult{}, err
	}
	written, present, err := readAdapterExecutionReceiptRaw(inst.CaseRoot, snapshot.receiptFull, snapshot.receiptRel)
	if err != nil || !present {
		if err == nil {
			err = fmt.Errorf("adapter execution receipt disappeared after write")
		}
		return AdapterExecutionReceiptResult{}, err
	}
	if !bytes.Equal(written, data) {
		return AdapterExecutionReceiptResult{}, fmt.Errorf("adapter execution receipt bytes changed after write: %s", snapshot.receiptRel)
	}
	if err := lease.Validate(); err != nil {
		return AdapterExecutionReceiptResult{}, fmt.Errorf("adapter execution receipt may already be durable at %s; mutation lease validation failed after create: %w", snapshot.receiptRel, err)
	}
	result.Applied = true
	result.ApplyCommand = ""
	result.Receipt = snapshot.receipt
	result.ReceiptSHA256 = adapterexecution.SHA256(written)
	result.NextSteps = []string{"immutable adapter execution receipt recorded", "rerun read-only report validation", "record observation evidence only with validation returned report/receipt hash-bound command"}
	return result, nil
}

func adapterExecutionReceiptRequired(caseRoot string, gateEvent EventPreview, m *manifest.Manifest) (bool, error) {
	if m == nil {
		return false, nil
	}
	ownerExists, err := laneowner.Exists(caseRoot, gateEvent.Lane)
	if err != nil {
		return false, err
	}
	if !ownerExists {
		return false, nil
	}
	candidates, err := strictAdapterToolCandidates(m, gateEvent)
	if err != nil {
		return false, err
	}
	return len(candidates) > 0, nil
}

func strictAdapterToolCandidates(m *manifest.Manifest, event EventPreview) ([]AdapterToolCandidate, error) {
	if m == nil {
		return nil, nil
	}
	action := strings.ToLower(strings.TrimSpace(event.Gate.Action))
	if action == "" {
		return nil, fmt.Errorf("adapter execution provenance requires an authorized gate action")
	}
	gateSpec, ok := m.HeavyToolGate(action)
	if !ok {
		return nil, fmt.Errorf("authorized action has no manifest heavy tool gate: %s", event.Gate.Action)
	}
	candidates := []AdapterToolCandidate{}
	for _, rel := range m.ToolingFiles {
		rel = strings.TrimSpace(rel)
		if rel == "" || !strings.HasSuffix(strings.ToLower(rel), "catalog.yml") {
			continue
		}
		path, err := m.SourcePath(rel)
		if err != nil {
			return nil, fmt.Errorf("resolve tooling catalog %s: %w", rel, err)
		}
		before, err := stableFileBinding(m.PackRoot, path, rel)
		if err != nil {
			return nil, fmt.Errorf("read tooling catalog %s: %w", rel, err)
		}
		catalogData, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read tooling catalog %s: %w", rel, err)
		}
		catalog, err := manifest.ParseToolCatalog(catalogData, m.Pack)
		if err != nil {
			return nil, fmt.Errorf("parse tooling catalog %s: %w", rel, err)
		}
		after, err := stableFileBinding(m.PackRoot, path, rel)
		if err != nil || before != after {
			return nil, fmt.Errorf("tooling catalog changed while selecting adapter: %s", rel)
		}
		for _, item := range catalog.Tools {
			candidate := adapterToolCandidateFromCatalogItem(item, rel, action, gateSpec)
			if candidate.ID != "" {
				candidates = append(candidates, candidate)
			}
		}
	}
	return candidates, nil
}

func validateRecordedAdapterExecutionReceipt(caseRoot, pack string, gateEvent EventPreview, reportRel, reportSHA string, report *AdapterReport, m *manifest.Manifest, opt Options) (*adapterexecution.Receipt, string, string, error) {
	rel, full, err := adapterExecutionReceiptPath(caseRoot, gateEvent.Lane, gateEvent.EventID)
	if err != nil {
		return nil, "", "", err
	}
	if requested := strings.TrimSpace(opt.AdapterExecutionReceiptPath); requested != "" {
		clean, pathErr := validateCaseRelativePath(caseRoot, "adapter execution receipt path", requested)
		if pathErr != nil || filepath.Clean(filepath.FromSlash(clean)) != filepath.Clean(filepath.FromSlash(rel)) {
			return nil, rel, "", fmt.Errorf("adapter execution receipt path must match canonical path: %s", rel)
		}
	}
	candidate, catalogSHA, catalogBytes, err := strictAdapterCandidateSnapshot(m, gateEvent, report.AdapterID)
	if err != nil {
		return nil, rel, "", err
	}
	data, present, err := readAdapterExecutionReceiptRaw(caseRoot, full, rel)
	if err != nil {
		return nil, rel, "", err
	}
	if !present {
		return nil, rel, "", fmt.Errorf("adapter execution provenance requires an immutable receipt before evidence record")
	}
	receipt, err := adapterexecution.Decode(data)
	if err != nil {
		return nil, rel, adapterexecution.SHA256(data), err
	}
	dispatch, dispatchRel, dispatchSHA, dispatchBytes, err :=
		readCurrentAdapterExecutionDispatch(caseRoot, pack, gateEvent, m)
	if err != nil {
		return nil, rel, adapterexecution.SHA256(data), err
	}
	if err := adapterexecution.ValidateCompletionDispatchLineage(
		receipt,
		dispatch,
		dispatchRel,
		dispatchSHA,
		dispatchBytes,
	); err != nil {
		return nil, rel, adapterexecution.SHA256(data), err
	}
	if err := validateAdapterReportDispatch(
		report,
		dispatch,
		dispatchRel,
		dispatchSHA,
		dispatchBytes,
	); err != nil {
		return nil, rel, adapterexecution.SHA256(data), err
	}
	owner, err := laneowner.Read(caseRoot, gateEvent.Lane)
	if err != nil {
		return nil, rel, adapterexecution.SHA256(data), err
	}
	if receipt.Owner.CurrentExecutor != owner.CurrentExecutor || receipt.Owner.ExecutorGeneration != owner.ExecutorGeneration {
		return nil, rel, adapterexecution.SHA256(data), fmt.Errorf("adapter execution receipt owner is stale: current executor=%s generation=%d", owner.CurrentExecutor, owner.ExecutorGeneration)
	}
	if strings.TrimSpace(opt.Executor) != "" && opt.Executor != owner.CurrentExecutor {
		return nil, rel, adapterexecution.SHA256(data), fmt.Errorf("adapter execution record executor does not match current lane owner")
	}
	if opt.ExpectedExecutorGeneration > 0 && opt.ExpectedExecutorGeneration != owner.ExecutorGeneration {
		return nil, rel, adapterexecution.SHA256(data), fmt.Errorf("adapter execution record generation does not match current lane owner")
	}
	candidateBinding := adapterExecutionCandidate(candidate)
	candidateSHA, _ := adapterexecution.CandidateSHA256(candidateBinding)
	if receipt.Adapter.Pack != pack || receipt.Adapter.AdapterID != report.AdapterID || receipt.Adapter.ToolingCatalogPath != candidate.ToolingCatalogPath || !strings.EqualFold(receipt.Adapter.ToolingCatalogSHA256, catalogSHA) || receipt.Adapter.ToolingCatalogBytes != catalogBytes || !strings.EqualFold(receipt.Adapter.CandidateSnapshotSHA256, candidateSHA) {
		return nil, rel, adapterexecution.SHA256(data), fmt.Errorf("adapter execution receipt catalog selection drifted")
	}
	currentGateBinding, err := adapterExecutionGateBinding(gateEvent)
	if err != nil {
		return nil, rel, adapterexecution.SHA256(data), err
	}
	if !reflect.DeepEqual(receipt.Gate, currentGateBinding) {
		return nil, rel, adapterexecution.SHA256(data), fmt.Errorf("adapter execution receipt authorized gate binding drifted")
	}
	if receipt.Report.Path != reportRel || !strings.EqualFold(receipt.Report.SHA256, reportSHA) {
		return nil, rel, adapterexecution.SHA256(data), fmt.Errorf("adapter execution receipt report binding drifted")
	}
	currentReport, err := stableFileBinding(caseRoot, filepath.Join(caseRoot, filepath.FromSlash(reportRel)), reportRel)
	if err != nil || currentReport != receipt.Report {
		return nil, rel, adapterexecution.SHA256(data), fmt.Errorf("adapter execution receipt report bytes drifted")
	}
	artifacts, err := adapterArtifactBindings(caseRoot, gateEvent, report, reportRel)
	if err != nil {
		return nil, rel, adapterexecution.SHA256(data), err
	}
	if !artifactBindingsEqual(receipt.Artifacts, artifacts) {
		return nil, rel, adapterexecution.SHA256(data), fmt.Errorf("adapter execution receipt artifact bytes drifted")
	}
	if receipt.Execution.Outcome != report.Status || receipt.Execution.ActualBudget != report.ActualBudget || receipt.Execution.AuthorizedBudget != gateEvent.Gate.RequestedBudget {
		return nil, rel, adapterexecution.SHA256(data), fmt.Errorf("adapter execution receipt outcome or budget drifted")
	}
	receiptSHA := adapterexecution.SHA256(data)
	if expected := strings.TrimSpace(opt.ExpectedAdapterExecutionReceiptSHA256); expected != "" {
		if !validSHA256String(expected) || !strings.EqualFold(expected, receiptSHA) {
			return nil, rel, receiptSHA, fmt.Errorf("adapter execution receipt sha256 changed after validation: expected %s got %s", expected, receiptSHA)
		}
	}
	return &receipt, rel, receiptSHA, nil
}

func validateAdapterReportDispatch(report *AdapterReport, dispatch adapterexecution.DispatchReceipt, dispatchPath, dispatchSHA string, dispatchBytes int64) error {
	if report == nil || report.Dispatch == nil {
		return fmt.Errorf("adapter execution report is missing immutable dispatch binding")
	}
	if dispatchBytes <= 0 {
		return fmt.Errorf("adapter execution dispatch byte binding is invalid")
	}
	expected := adapterexecution.ReportDispatchBinding{
		DispatchID: dispatch.DispatchID,
		Path:       dispatchPath,
		SHA256:     dispatchSHA,
	}
	if *report.Dispatch != expected {
		return fmt.Errorf("adapter execution report dispatch binding drifted")
	}
	return nil
}

func artifactBindingsEqual(left, right []adapterexecution.ArtifactBinding) bool {
	if len(left) != len(right) {
		return false
	}
	leftCopy := append([]adapterexecution.ArtifactBinding{}, left...)
	rightCopy := append([]adapterexecution.ArtifactBinding{}, right...)
	sort.Slice(leftCopy, func(i, j int) bool { return leftCopy[i].Path < leftCopy[j].Path })
	sort.Slice(rightCopy, func(i, j int) bool { return rightCopy[i].Path < rightCopy[j].Path })
	for i := range leftCopy {
		if leftCopy[i].Path != rightCopy[i].Path || leftCopy[i].SHA256 != rightCopy[i].SHA256 || leftCopy[i].Bytes != rightCopy[i].Bytes || strings.Join(leftCopy[i].Roles, ",") != strings.Join(rightCopy[i].Roles, ",") {
			return false
		}
	}
	return true
}

func adapterExecutionReceiptPreviewSlashCommand(pack string, gateEvent EventPreview, reportPath, adapterID string) string {
	return adapterReportSlashCommand([]string{"gate", "-Pack", pack, "-GateEventId", gateEvent.EventID, "-RecordAdapterExecutionReceipt", "-ExecutionReportPath", reportPath, "-AdapterId", adapterID, "-Executor", "<current-executor>", "-ExpectedExecutorGeneration", "<current-generation>", "-AdapterHarness", "<harness>", "-AdapterSession", "<session>", "-ExecutionExitStatus", "<exit-status>", "-Actor", "<recorded-by>", "-Format", "json"})
}

func inspectAdapterExecutionDispatch(caseRoot, pack string, gateEvent EventPreview, m *manifest.Manifest) (adapterexecution.DispatchReceipt, string, string, int64, bool, error) {
	dispatchRel, dispatchFull, err := adapterExecutionDispatchPath(caseRoot, gateEvent.Lane, gateEvent.EventID)
	if err != nil {
		return adapterexecution.DispatchReceipt{}, "", "", 0, false, err
	}
	_, present, err := readAdapterExecutionReceiptRaw(caseRoot, dispatchFull, dispatchRel)
	if err != nil {
		return adapterexecution.DispatchReceipt{}, dispatchRel, "", 0, true, err
	}
	if !present {
		return adapterexecution.DispatchReceipt{}, dispatchRel, "", 0, false, nil
	}
	dispatch, path, sha, bytes, err := readCurrentAdapterExecutionDispatch(caseRoot, pack, gateEvent, m)
	return dispatch, path, sha, bytes, true, err
}

func ReadCurrentAdapterExecutionDispatch(repoRoot, caseRoot, pack, gateEventID string) (adapterexecution.DispatchReceipt, string, string, int64, error) {
	inst, gateEvent, err := authorizedGateEvent(repoRoot, caseRoot, pack, Options{
		GateEventID: strings.TrimSpace(gateEventID),
	})
	if err != nil {
		return adapterexecution.DispatchReceipt{}, "", "", 0, err
	}
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return adapterexecution.DispatchReceipt{}, "", "", 0, err
	}
	return readCurrentAdapterExecutionDispatch(
		inst.CaseRoot,
		pack,
		gateEvent,
		m,
	)
}

func readCurrentAdapterExecutionDispatch(caseRoot, pack string, gateEvent EventPreview, m *manifest.Manifest) (adapterexecution.DispatchReceipt, string, string, int64, error) {
	dispatchRel, dispatchFull, err := adapterExecutionDispatchPath(caseRoot, gateEvent.Lane, gateEvent.EventID)
	if err != nil {
		return adapterexecution.DispatchReceipt{}, "", "", 0, err
	}
	data, present, err := readAdapterExecutionReceiptRaw(caseRoot, dispatchFull, dispatchRel)
	if err != nil {
		return adapterexecution.DispatchReceipt{}, dispatchRel, "", 0, err
	}
	if !present {
		return adapterexecution.DispatchReceipt{}, dispatchRel, "", 0, fmt.Errorf("adapter execution completion requires an immutable dispatch recorded before external execution")
	}
	dispatch, err := adapterexecution.DecodeDispatch(data)
	if err != nil {
		return adapterexecution.DispatchReceipt{}, dispatchRel, adapterexecution.SHA256(data), int64(len(data)), err
	}
	current, err := prepareAdapterExecutionDispatchSnapshot(caseRoot, pack, gateEvent, Options{
		GateEventID: gateEvent.EventID, ExecutionReportPath: dispatch.ReportPath, AdapterID: dispatch.Adapter.AdapterID,
		Executor: dispatch.Owner.CurrentExecutor, ExpectedExecutorGeneration: dispatch.Owner.ExecutorGeneration,
		AdapterHarness: dispatch.Owner.AdapterHarness, AdapterSession: dispatch.Owner.AdapterSession, Actor: dispatch.Actor,
		ExecutionControlBinding: executioncontrol.CloneBinding(dispatch.LaunchControl),
	}, m)
	if err != nil {
		return dispatch, dispatchRel, adapterexecution.SHA256(data), int64(len(data)), err
	}
	if !adapterexecution.DispatchSemanticEqual(dispatch, current.dispatch) {
		return dispatch, dispatchRel, adapterexecution.SHA256(data), int64(len(data)), fmt.Errorf("adapter execution dispatch gate, owner, catalog, session, or report path drifted")
	}
	return dispatch, dispatchRel, adapterexecution.SHA256(data), int64(len(data)), nil
}

func prepareAdapterExecutionSnapshot(caseRoot, pack string, gateEvent EventPreview, opt Options, m *manifest.Manifest) (adapterExecutionSnapshot, error) {
	dispatch, dispatchRel, dispatchSHA, dispatchBytes, err := readCurrentAdapterExecutionDispatch(caseRoot, pack, gateEvent, m)
	if err != nil {
		return adapterExecutionSnapshot{}, err
	}
	owner, err := laneowner.Read(caseRoot, gateEvent.Lane)
	if err != nil {
		return adapterExecutionSnapshot{}, err
	}
	if strings.TrimSpace(opt.Executor) == "" || opt.ExpectedExecutorGeneration <= 0 {
		return adapterExecutionSnapshot{}, fmt.Errorf("gate adapter execution receipt requires -Executor and -ExpectedExecutorGeneration for the current durable lane owner")
	}
	if opt.Executor != owner.CurrentExecutor || opt.ExpectedExecutorGeneration != owner.ExecutorGeneration {
		return adapterExecutionSnapshot{}, fmt.Errorf("adapter execution receipt owner is stale: current executor=%s generation=%d", owner.CurrentExecutor, owner.ExecutorGeneration)
	}
	if strings.TrimSpace(opt.AdapterHarness) == "" || strings.TrimSpace(opt.AdapterSession) == "" || strings.TrimSpace(opt.ExecutionExitStatus) == "" || strings.TrimSpace(opt.Actor) == "" {
		return adapterExecutionSnapshot{}, fmt.Errorf("gate adapter execution receipt requires -AdapterHarness, -AdapterSession, -ExecutionExitStatus, and -Actor")
	}
	reportRel, reportSHA, report, err := readAdapterExecutionReport(caseRoot, gateEvent, opt.ExecutionReportCwd, opt.ExecutionReportPath)
	if err != nil {
		return adapterExecutionSnapshot{}, err
	}
	if report == nil {
		return adapterExecutionSnapshot{}, fmt.Errorf("gate adapter execution receipt requires -ExecutionReportPath")
	}
	if strings.TrimSpace(opt.AdapterID) != "" && !strings.EqualFold(opt.AdapterID, report.AdapterID) {
		return adapterExecutionSnapshot{}, fmt.Errorf("adapter execution receipt adapterId does not match report")
	}
	candidate, catalogSHA, catalogBytes, err := strictAdapterCandidateSnapshot(m, gateEvent, report.AdapterID)
	if err != nil {
		return adapterExecutionSnapshot{}, err
	}
	candidateBinding := adapterExecutionCandidate(candidate)
	candidateSHA, err := adapterexecution.CandidateSHA256(candidateBinding)
	if err != nil {
		return adapterExecutionSnapshot{}, err
	}
	gateBinding, err := adapterExecutionGateBinding(gateEvent)
	if err != nil {
		return adapterExecutionSnapshot{}, err
	}
	reportFull, _, err := executionReportPath(caseRoot, reportRel)
	if err != nil {
		return adapterExecutionSnapshot{}, err
	}
	reportSnapshot, err := stableFileBinding(caseRoot, reportFull, reportRel)
	if err != nil {
		return adapterExecutionSnapshot{}, err
	}
	if !strings.EqualFold(reportSnapshot.SHA256, reportSHA) {
		return adapterExecutionSnapshot{}, fmt.Errorf("adapter execution report changed while preparing receipt")
	}
	artifacts, err := adapterArtifactBindings(caseRoot, gateEvent, report, reportRel)
	if err != nil {
		return adapterExecutionSnapshot{}, err
	}
	if dispatch.ReportPath != reportRel || dispatch.Owner.CurrentExecutor != owner.CurrentExecutor || dispatch.Owner.ExecutorGeneration != owner.ExecutorGeneration || dispatch.Owner.AdapterHarness != strings.TrimSpace(opt.AdapterHarness) || dispatch.Owner.AdapterSession != strings.TrimSpace(opt.AdapterSession) {
		return adapterExecutionSnapshot{}, fmt.Errorf("adapter execution completion does not match immutable dispatch owner/session/report bindings")
	}
	if err := validateAdapterReportDispatch(report, dispatch, dispatchRel, dispatchSHA, dispatchBytes); err != nil {
		return adapterExecutionSnapshot{}, err
	}
	receipt := adapterexecution.Receipt{
		SchemaVersion: 1, Kind: "adapter-execution-receipt", Dispatch: adapterexecution.DispatchBinding{DispatchID: dispatch.DispatchID, Path: dispatchRel, SHA256: dispatchSHA, Bytes: dispatchBytes}, Gate: gateBinding,
		Adapter:   adapterexecution.AdapterBinding{Pack: pack, AdapterID: report.AdapterID, ToolingCatalogPath: candidate.ToolingCatalogPath, ToolingCatalogSHA256: catalogSHA, ToolingCatalogBytes: catalogBytes, Candidate: candidateBinding, CandidateSnapshotSHA256: candidateSHA},
		Owner:     dispatch.Owner,
		Execution: adapterexecution.ExecutionBinding{Outcome: report.Status, ExitStatus: strings.TrimSpace(opt.ExecutionExitStatus), AuthorizedBudget: gateEvent.Gate.RequestedBudget, ActualBudget: report.ActualBudget, BoundaryHits: append([]string{}, report.BoundaryHits...), Escalation: report.Escalation},
		Report:    reportSnapshot, Artifacts: artifacts, Actor: strings.TrimSpace(opt.Actor), Capability: dispatch.Capability, NoExecute: true, NoAuthority: true,
	}
	bindingSHA, err := adapterexecution.BindingSHA256(receipt)
	if err != nil {
		return adapterExecutionSnapshot{}, err
	}
	receipt.ReceiptID = bindingSHA
	if err := adapterexecution.ValidateCompletionDispatchLineage(receipt, dispatch, dispatchRel, dispatchSHA, dispatchBytes); err != nil {
		return adapterExecutionSnapshot{}, err
	}
	rel, full, err := adapterExecutionReceiptPath(caseRoot, gateEvent.Lane, gateEvent.EventID)
	if err != nil {
		return adapterExecutionSnapshot{}, err
	}
	return adapterExecutionSnapshot{dispatch: dispatch, receipt: receipt, bindingSHA: bindingSHA, receiptRel: rel, receiptFull: full}, nil
}

func adapterExecutionCandidate(candidate AdapterToolCandidate) adapterexecution.Candidate {
	return adapterexecution.Candidate{ID: candidate.ID, Status: candidate.Status, Entry: candidate.Entry, Purpose: candidate.Purpose, SideEffects: append([]string{}, candidate.SideEffects...), GateActions: append([]string{}, candidate.GateActions...), ToolingCatalogPath: candidate.ToolingCatalogPath, StopConditionHints: append([]string{}, candidate.StopConditionHints...), RecordOnlyAfterGate: candidate.RecordOnlyAfterGate}
}

func adapterExecutionGateBinding(gateEvent EventPreview) (adapterexecution.GateBinding, error) {
	binding := adapterexecution.GateBinding{
		GateEventID:      gateEvent.EventID,
		Lane:             gateEvent.Lane,
		Action:           gateEvent.Gate.Action,
		Target:           gateEvent.Target,
		Authorization:    gateEvent.Gate.Authorization,
		AuthorizedBudget: gateEvent.Gate.RequestedBudget,
		OutputPaths:      append([]string{}, gateEvent.Gate.OutputPaths...),
		StopConditions:   append([]string{}, gateEvent.Gate.StopConditions...),
	}
	hash, err := adapterexecution.GateSHA256(binding)
	if err != nil {
		return adapterexecution.GateBinding{}, err
	}
	binding.SnapshotSHA256 = hash
	return binding, nil
}

func strictAdapterCandidateSnapshot(m *manifest.Manifest, event EventPreview, adapterID string) (AdapterToolCandidate, string, int64, error) {
	candidates, err := strictAdapterToolCandidates(m, event)
	if err != nil {
		return AdapterToolCandidate{}, "", 0, err
	}
	matches := []AdapterToolCandidate{}
	for _, candidate := range candidates {
		if strings.EqualFold(candidate.ID, adapterID) {
			matches = append(matches, candidate)
		}
	}
	if len(matches) != 1 {
		return AdapterToolCandidate{}, "", 0, fmt.Errorf("adapter execution provenance requires exactly one selected tooling catalog candidate for %q; found %d", adapterID, len(matches))
	}
	selected := matches[0]
	path, err := m.SourcePath(selected.ToolingCatalogPath)
	if err != nil {
		return AdapterToolCandidate{}, "", 0, err
	}
	catalog, err := stableFileBinding(m.PackRoot, path, selected.ToolingCatalogPath)
	if err != nil {
		return AdapterToolCandidate{}, "", 0, fmt.Errorf("read tooling catalog %s: %w", selected.ToolingCatalogPath, err)
	}
	return selected, catalog.SHA256, catalog.Bytes, nil
}

func adapterArtifactBindings(caseRoot string, gateEvent EventPreview, report *AdapterReport, reportRel string) ([]adapterexecution.ArtifactBinding, error) {
	roles := map[string]map[string]bool{}
	for _, item := range report.OutputRefs {
		if roles[item] == nil {
			roles[item] = map[string]bool{}
		}
		roles[item]["output"] = true
	}
	for _, item := range report.EvidenceRefs {
		if roles[item] == nil {
			roles[item] = map[string]bool{}
		}
		roles[item]["evidence"] = true
	}
	paths := make([]string, 0, len(roles))
	for path := range roles {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	out := make([]adapterexecution.ArtifactBinding, 0, len(paths))
	for _, rel := range paths {
		if rel == reportRel {
			return nil, fmt.Errorf("adapter execution report cannot reference itself as an output/evidence artifact")
		}
		if !outputRefsWithinGate(gateEvent.Gate.OutputPaths, []string{rel}) {
			return nil, fmt.Errorf("adapter execution artifact must stay within authorized outputPaths: %s", rel)
		}
		full, err := refsf.SafeJoin(caseRoot, rel)
		if err != nil {
			return nil, err
		}
		file, err := stableFileBinding(caseRoot, full, rel)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("read adapter execution artifact %s: missing artifact", rel)
			}
			return nil, fmt.Errorf("read adapter execution artifact %s: %w", rel, err)
		}
		itemRoles := []string{}
		for role := range roles[rel] {
			itemRoles = append(itemRoles, role)
		}
		sort.Strings(itemRoles)
		out = append(out, adapterexecution.ArtifactBinding{Path: file.Path, Roles: itemRoles, SHA256: file.SHA256, Bytes: file.Bytes})
	}
	return out, nil
}

func stableFileBinding(root, full, rel string) (adapterexecution.FileBinding, error) {
	if err := rejectSymlinkComponents(root, full); err != nil {
		return adapterexecution.FileBinding{}, err
	}
	st, err := os.Lstat(full)
	if err != nil {
		return adapterexecution.FileBinding{}, err
	}
	if st.Mode()&os.ModeSymlink != 0 || !st.Mode().IsRegular() {
		return adapterexecution.FileBinding{}, fmt.Errorf("path must be a regular non-symlink file: %s", rel)
	}
	f, err := os.Open(full)
	if err != nil {
		return adapterexecution.FileBinding{}, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(st, opened) {
		return adapterexecution.FileBinding{}, fmt.Errorf("file changed or is not regular: %s", rel)
	}
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return adapterexecution.FileBinding{}, err
	}
	post, err := os.Lstat(full)
	if err != nil || post.Mode()&os.ModeSymlink != 0 || !post.Mode().IsRegular() || !os.SameFile(opened, post) || post.Size() != n {
		return adapterexecution.FileBinding{}, fmt.Errorf("file changed while hashing: %s", rel)
	}
	return adapterexecution.FileBinding{Path: filepath.ToSlash(filepath.Clean(filepath.FromSlash(rel))), SHA256: hex.EncodeToString(h.Sum(nil)), Bytes: n}, nil
}

func rejectSymlinkComponents(root, full string) error {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	fullAbs, err := filepath.Abs(full)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(rootAbs, fullAbs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("path escapes root: %s", full)
	}
	current := rootAbs
	for part := range strings.SplitSeq(rel, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		st, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if st.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("path must not traverse symlink: %s", current)
		}
	}
	return nil
}

func adapterExecutionDispatchPath(caseRoot, laneID, gateEventID string) (string, string, error) {
	if !adapterExecutionSegmentPattern.MatchString(laneID) || !adapterExecutionSegmentPattern.MatchString(gateEventID) {
		return "", "", fmt.Errorf("invalid adapter execution dispatch path identity")
	}
	rel, err := projectstate.Rel(caseRoot, "lanes", laneID, "adapter-executions", gateEventID, "dispatch.json")
	if err != nil {
		return "", "", err
	}
	full, err := refsf.SafeJoin(caseRoot, rel)
	return rel, full, err
}

func adapterExecutionReceiptPath(caseRoot, laneID, gateEventID string) (string, string, error) {
	if !adapterExecutionSegmentPattern.MatchString(laneID) || !adapterExecutionSegmentPattern.MatchString(gateEventID) {
		return "", "", fmt.Errorf("invalid adapter execution receipt path identity")
	}
	rel, err := projectstate.Rel(caseRoot, "lanes", laneID, "adapter-executions", gateEventID, "receipt.json")
	if err != nil {
		return "", "", err
	}
	full, err := refsf.SafeJoin(caseRoot, rel)
	return rel, full, err
}

func adapterExecutionReceiptExists(caseRoot string, gateEvent EventPreview) (bool, error) {
	_, full, err := adapterExecutionReceiptPath(caseRoot, gateEvent.Lane, gateEvent.EventID)
	if err != nil {
		return false, err
	}
	if _, err := os.Lstat(full); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return true, err
	}
	return true, nil
}

func ReadAdapterExecutionReceipt(caseRoot, lane, gateEventID string) (*adapterexecution.Receipt, string, string, bool, error) {
	rel, full, err := adapterExecutionReceiptPath(caseRoot, lane, gateEventID)
	if err != nil {
		return nil, "", "", false, err
	}
	data, present, err := readAdapterExecutionReceiptRaw(caseRoot, full, rel)
	if err != nil || !present {
		return nil, rel, "", present, err
	}
	receiptSHA := adapterexecution.SHA256(data)
	receipt, err := adapterexecution.Decode(data)
	if err != nil {
		return nil, rel, "", true, fmt.Errorf("adapter execution receipt is invalid: %w", err)
	}
	if receipt.Gate.GateEventID != strings.TrimSpace(gateEventID) || receipt.Gate.Lane != strings.TrimSpace(lane) || receipt.Owner.Lane != strings.TrimSpace(lane) {
		return nil, rel, "", true, fmt.Errorf("adapter execution receipt identity does not match requested lane/gate: %s", rel)
	}
	observations, err := mission.ReadStrictFact(caseRoot, "observation")
	if err != nil {
		return nil, rel, "", true, err
	}
	for _, observation := range observations {
		item, ok := mission.ExecutionEvidenceReviewItemFromObservation(observation, lane, nil)
		if !ok || item.GateEventID != gateEventID {
			continue
		}
		if filepath.Clean(filepath.FromSlash(item.AdapterExecutionReceiptPath)) != filepath.Clean(filepath.FromSlash(rel)) || !strings.EqualFold(item.AdapterExecutionReceiptSHA256, receiptSHA) || !strings.EqualFold(item.ExecutionReportSHA256, receipt.Report.SHA256) {
			return nil, rel, "", true, fmt.Errorf("adapter execution receipt does not match recorded observation provenance: %s", rel)
		}
		return &receipt, rel, receiptSHA, true, nil
	}
	return nil, rel, "", true, fmt.Errorf("adapter execution receipt has no recorded observation provenance: %s", rel)
}

func readAdapterExecutionReceiptRaw(caseRoot, full, rel string) ([]byte, bool, error) {
	if _, err := os.Lstat(full); os.IsNotExist(err) {
		return nil, false, nil
	} else if err != nil {
		return nil, true, err
	}
	file, err := stableFileBinding(caseRoot, full, rel)
	if err != nil {
		return nil, true, err
	}
	if file.Bytes > 1<<20 {
		return nil, true, fmt.Errorf("adapter execution receipt is too large: %s", rel)
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return nil, true, err
	}
	if int64(len(data)) != file.Bytes || !strings.EqualFold(adapterexecution.SHA256(data), file.SHA256) {
		return nil, true, fmt.Errorf("adapter execution receipt changed after stable read: %s", rel)
	}
	return data, true, nil
}

var adapterExecutionReceiptWriteHook func(stage, rel string) error

func writeAdapterExecutionReceipt(caseRoot, full, rel string, data []byte) error {
	parent := filepath.Dir(full)
	if err := ensureAdapterExecutionReceiptParent(caseRoot, parent); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(parent, ".adapter-execution-receipt-*.tmp")
	if err != nil {
		return fmt.Errorf("create adapter execution receipt temp for %s: %w", rel, err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	writeErr := error(nil)
	if adapterExecutionReceiptWriteHook != nil {
		writeErr = adapterExecutionReceiptWriteHook("before-temp-write", rel)
	}
	if writeErr == nil {
		written, err := tmp.Write(data)
		if err != nil {
			writeErr = err
		} else if written != len(data) {
			writeErr = io.ErrShortWrite
		}
	}
	if writeErr == nil {
		writeErr = tmp.Sync()
	}
	if closeErr := tmp.Close(); writeErr == nil {
		writeErr = closeErr
	}
	if writeErr != nil {
		return fmt.Errorf("write adapter execution receipt temp for %s: %w", rel, writeErr)
	}
	if adapterExecutionReceiptWriteHook != nil {
		if err := adapterExecutionReceiptWriteHook("before-publish", rel); err != nil {
			return err
		}
	}
	if err := os.Link(tmpPath, full); err != nil {
		return fmt.Errorf("publish adapter execution receipt %s without replacement: %w", rel, err)
	}
	return nil
}

func ensureAdapterExecutionReceiptParent(caseRoot, parent string) error {
	caseAbs, err := filepath.Abs(caseRoot)
	if err != nil {
		return err
	}
	parentAbs, err := filepath.Abs(parent)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(caseAbs, parentAbs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("adapter execution receipt parent escapes case root")
	}
	current := caseAbs
	for part := range strings.SplitSeq(rel, string(filepath.Separator)) {
		if part == "" || part == "." || part == ".." {
			continue
		}
		current = filepath.Join(current, part)
		st, err := os.Lstat(current)
		if os.IsNotExist(err) {
			if err := os.Mkdir(current, 0o700); err != nil && !os.IsExist(err) {
				return err
			}
			st, err = os.Lstat(current)
		}
		if err != nil {
			return err
		}
		if st.Mode()&os.ModeSymlink != 0 || !st.Mode().IsDir() {
			return fmt.Errorf("adapter execution receipt parent must be a non-symlink directory: %s", current)
		}
	}
	return nil
}

func adapterExecutionReceiptApplySlashCommand(pack string, receipt adapterexecution.Receipt, bindingSHA string) string {
	args := []string{"gate", "-Pack", pack, "-GateEventId", receipt.Gate.GateEventID, "-RecordAdapterExecutionReceipt", "-ExecutionReportPath", receipt.Report.Path, "-AdapterId", receipt.Adapter.AdapterID, "-Executor", receipt.Owner.CurrentExecutor, "-ExpectedExecutorGeneration", fmt.Sprintf("%d", receipt.Owner.ExecutorGeneration), "-AdapterHarness", receipt.Owner.AdapterHarness, "-AdapterSession", receipt.Owner.AdapterSession, "-ExecutionExitStatus", receipt.Execution.ExitStatus, "-Actor", receipt.Actor, "-ExpectedAdapterExecutionBindingSha256", bindingSHA, "-Apply", "-Format", "json"}
	return adapterReportSlashCommand(args)
}

func validSHA256String(value string) bool {
	data, err := hex.DecodeString(strings.TrimSpace(value))
	return err == nil && len(data) == sha256.Size
}
