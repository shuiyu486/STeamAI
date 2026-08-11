package adapterhost

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/autonomy"
	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/gate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/note"
	"github.com/shuiyu486/re-context-kits/internal/rekit/processguard"
	syncreview "github.com/shuiyu486/re-context-kits/internal/rekit/sync"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

const (
	liveAcceptancePack     = "_template"
	liveAcceptanceLaneName = "readonly-inspection"
	liveAcceptanceExecutor = "rh06-adapter-executor"
	liveAcceptanceTarget   = "fixture/input.txt"
	liveAcceptanceReport   = "workspace/features/readonly-inspection/inspect/session-1/adapter-report.json"
)

type liveAcceptanceTestHooks struct {
	afterCaseQuarantine func(string) error
}

type LiveAcceptanceOptions struct {
	RepoRoot    string
	AdapterPath string
	ReceiptPath string
	testHooks   *liveAcceptanceTestHooks
}

type LiveAcceptanceReceipt struct {
	SchemaVersion      int      `json:"schemaVersion"`
	Kind               string   `json:"kind"`
	Passed             bool     `json:"passed"`
	RepoRoot           string   `json:"repoRoot"`
	CaseRoot           string   `json:"caseRoot"`
	Pack               string   `json:"pack"`
	Lane               string   `json:"lane"`
	GateEventID        string   `json:"gateEventId"`
	Authorization      string   `json:"authorization"`
	AdapterID          string   `json:"adapterId"`
	AdapterHarness     string   `json:"adapterHarness"`
	AdapterSession     string   `json:"adapterSession"`
	AdapterPath        string   `json:"adapterPath"`
	AdapterSHA256      string   `json:"adapterSha256"`
	AdapterProcessID   int      `json:"adapterProcessId"`
	DispatchPath       string   `json:"dispatchPath"`
	DispatchSHA256     string   `json:"dispatchSha256"`
	InputPath          string   `json:"inputPath"`
	InputSHA256        string   `json:"inputSha256"`
	ArtifactPath       string   `json:"artifactPath"`
	ArtifactSHA256     string   `json:"artifactSha256"`
	ReportPath         string   `json:"reportPath"`
	ReportSHA256       string   `json:"reportSha256"`
	ReceiptPath        string   `json:"receiptPath"`
	ReceiptSHA256      string   `json:"receiptSha256"`
	ObservationEventID string   `json:"observationEventId"`
	AcknowledgementID  string   `json:"acknowledgementEventId"`
	MissionResumeState string   `json:"missionResumeState"`
	ReceiptOutputPath  string   `json:"receiptOutputPath"`
	Cleanup            string   `json:"cleanup"`
	NoNetwork          bool     `json:"noNetwork"`
	ReadOnlyInput      bool     `json:"readOnlyInput"`
	NoAuthority        bool     `json:"noAuthorityOrConfirmed"`
	StartedAt          string   `json:"startedAt"`
	CompletedAt        string   `json:"completedAt"`
	Boundary           []string `json:"boundary"`
}

func RunLiveAcceptance(opt LiveAcceptanceOptions) (receipt LiveAcceptanceReceipt, retErr error) {
	started := time.Now().UTC()
	receipt = LiveAcceptanceReceipt{
		SchemaVersion:  1,
		Kind:           "rekit-readonly-adapter-live-acceptance-receipt",
		Pack:           liveAcceptancePack,
		AdapterID:      readonlyInspectorID,
		AdapterHarness: adapterHarness,
		StartedAt:      started.Format(time.RFC3339Nano),
		Cleanup:        "pending",
		NoNetwork:      true,
		ReadOnlyInput:  true,
		NoAuthority:    true,
		Boundary: []string{
			"explicit opt-in live gate for one harmless case-local fixture",
			"immutable dispatch is recorded before the adapter process starts",
			"the adapter process performs no network, debug, patch, dump, hook, or target mutation",
			"receipt, validation, observation, and acknowledgement preserve dispatch/report/artifact lineage",
			"the disposable case is removed before this function returns",
			"no authority or confirmed ledger state is written",
		},
	}
	repoRoot, err := filepath.Abs(strings.TrimSpace(opt.RepoRoot))
	if err != nil {
		return receipt, err
	}
	adapterPath, err := filepath.Abs(strings.TrimSpace(opt.AdapterPath))
	if err != nil {
		return receipt, err
	}
	receiptPath, err := filepath.Abs(strings.TrimSpace(opt.ReceiptPath))
	if err != nil {
		return receipt, err
	}
	if strings.TrimSpace(opt.AdapterPath) == "" || strings.TrimSpace(opt.ReceiptPath) == "" {
		return receipt, fmt.Errorf("adapter live acceptance requires -adapter and -receipt")
	}
	adapterBinding, err := processguard.LockExecutable(adapterPath, 128<<20)
	if err != nil {
		return receipt, err
	}
	defer func() { retErr = errors.Join(retErr, adapterBinding.Close()) }()
	adapterSHA := adapterBinding.SHA256()
	receipt.RepoRoot = repoRoot
	if pathWithin(repoRoot, receiptPath) {
		return receipt, fmt.Errorf("adapter live acceptance receipt must be outside the repository: %s", receiptPath)
	}
	receipt.AdapterPath = adapterPath
	receipt.AdapterSHA256 = adapterSHA
	receipt.ReceiptOutputPath = receiptPath

	caseRoot, err := os.MkdirTemp("", "rekit-rh06-adapter-live-")
	if err != nil {
		return receipt, err
	}
	receipt.CaseRoot = caseRoot
	caseIdentity, err := captureLiveAcceptanceCase(caseRoot, "", nil)
	if err != nil {
		_ = os.Remove(caseRoot)
		return receipt, err
	}
	defer func() {
		defer func() { retErr = errors.Join(retErr, caseIdentity.Close()) }()
		var afterQuarantine func(string) error
		if opt.testHooks != nil {
			afterQuarantine = opt.testHooks.afterCaseQuarantine
		}
		cleanupErr := caseIdentity.cleanup(afterQuarantine)
		if cleanupErr != nil {
			receipt.Passed = false
			receipt.Cleanup = "failed"
			retErr = errors.Join(retErr, cleanupErr)
			return
		}
		receipt.Cleanup = "removed"
	}()
	marker := []byte("rekit-rh06-readonly-adapter-live-acceptance\n")
	markerName := ".rekit-rh06-live-marker"
	markerRoot, err := os.OpenRoot(caseRoot)
	if err != nil {
		return receipt, err
	}
	markerFile, err := markerRoot.OpenFile(markerName, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err == nil {
		_, err = markerFile.Write(marker)
	}
	if markerFile != nil {
		err = errors.Join(err, markerFile.Close())
	}
	err = errors.Join(err, markerRoot.Close())
	if err != nil {
		return receipt, err
	}
	if err := caseIdentity.bindMarker(markerName, marker); err != nil {
		return receipt, err
	}
	if pathWithin(caseRoot, receiptPath) {
		return receipt, fmt.Errorf("adapter live acceptance receipt must be outside the disposable case: %s", receiptPath)
	}

	initOpt := syncreview.ApplyOptions{
		ProjectName:      "rekit-rh06-adapter-live",
		CreateLocalFiles: true,
		Command:          "init",
	}
	initPlan, err := syncreview.InitPreview(repoRoot, caseRoot, liveAcceptancePack, initOpt)
	if err != nil || initPlan.IsMutation || !initPlan.ReviewRequired || !initPlan.RequiresConfirmation {
		return receipt, fmt.Errorf("adapter live acceptance init preview failed review-first validation: %w", err)
	}
	initOpt.ExpectedPlanSHA256 = initPlan.ExpectedPlanSHA256
	initialized, err := syncreview.Apply(repoRoot, caseRoot, liveAcceptancePack, initOpt)
	if err != nil || !initialized.Applied {
		return receipt, fmt.Errorf("adapter live acceptance init Apply failed: %w", err)
	}

	startOpt := workstream.StartOptions{
		Name:     liveAcceptanceLaneName,
		Selector: liveAcceptanceLaneName,
		Actor:    "rh06-live-acceptance",
		Executor: liveAcceptanceExecutor,
	}
	startPlan, err := workstream.StartPreview(repoRoot, caseRoot, liveAcceptancePack, startOpt)
	if err != nil || startPlan.Applied || !startPlan.RequiresConfirmation {
		return receipt, fmt.Errorf("adapter live acceptance start preview failed: %w", err)
	}
	startData, err := json.Marshal(startPlan)
	if err != nil {
		return receipt, err
	}
	startOpt.ExpectedPreviewSHA256 = sha256Hex(startData)
	startedLane, err := workstream.StartApply(repoRoot, caseRoot, liveAcceptancePack, startOpt)
	if err != nil || !startedLane.Applied || startedLane.Lane.CurrentExecutor != liveAcceptanceExecutor || startedLane.Lane.ExecutorGeneration != 1 {
		return receipt, fmt.Errorf("adapter live acceptance start Apply failed: %w", err)
	}
	receipt.Lane = startedLane.Lane.ID

	if err := writeLiveFixture(caseRoot); err != nil {
		return receipt, err
	}
	inputBefore, err := stableLiveFile(caseRoot, liveAcceptanceTarget, maxFixtureBytes)
	if err != nil {
		return receipt, err
	}
	profile := autonomy.Profile{
		SchemaVersion:  1,
		ProfileID:      "rh06-readonly-inspect",
		Lane:           receipt.Lane,
		Mode:           autonomy.ModePreauthorized,
		AllowedActions: []string{"inspect"},
		DeniedActions:  []string{},
		TargetScope:    []autonomy.Target{{Match: "exact", Value: liveAcceptanceTarget}},
		Budget:         autonomy.Budget{RuntimeSeconds: 10, DiskMB: 4, Requests: 1},
		StopConditions: []string{"timeout", "scope-drift", "budget-exhausted"},
		OutputPaths:    []string{"workspace/features/readonly-inspection/inspect"},
		RecordRequired: true,
		NotifyMainOn:   []string{"boundary-hit", "new-risk"},
		GrantedBy:      "rh06-live-acceptance",
		GrantedAt:      started.Format(time.RFC3339Nano),
		ExpiresAt:      started.Add(time.Hour).Format(time.RFC3339Nano),
	}
	profileData, err := canonicalJSON(profile)
	if err != nil {
		return receipt, err
	}
	if err := os.WriteFile(filepath.Join(caseRoot, ".rekit", "lanes", receipt.Lane, "autonomy.json"), profileData, 0o600); err != nil {
		return receipt, err
	}

	authorized, err := gate.Apply(repoRoot, caseRoot, liveAcceptancePack, gate.Options{
		Action: "inspect", Lane: receipt.Lane, Actor: "rh06-live-acceptance",
		Subject: "inspect harmless fixture", Summary: "read one bounded case-local text fixture",
		TargetRef: liveAcceptanceTarget, RuntimeSeconds: 10, DiskMB: 4, Requests: 1,
		OutputPaths:    "workspace/features/readonly-inspection/inspect/session-1",
		StopConditions: "timeout,scope-drift,budget-exhausted",
	})
	if err != nil || !authorized.Applied || authorized.Event == nil || authorized.Event.Gate.Authorization.Decision != autonomy.DecisionPreauthorized {
		return receipt, fmt.Errorf("adapter live acceptance gate was not strictly preauthorized: %w", err)
	}
	receipt.GateEventID = authorized.EventID
	receipt.Authorization = authorized.Event.Gate.Authorization.Decision
	receipt.AdapterSession = "rh06-adapter-session-1"
	dispatchOpt := gate.Options{
		GateEventID: authorized.EventID, ExecutionReportPath: liveAcceptanceReport,
		AdapterID: readonlyInspectorID, Executor: liveAcceptanceExecutor,
		ExpectedExecutorGeneration: 1, AdapterHarness: adapterHarness,
		AdapterSession: receipt.AdapterSession, Actor: "mission-commander",
	}
	dispatchPreview, err := gate.RecordAdapterExecutionDispatch(repoRoot, caseRoot, liveAcceptancePack, dispatchOpt)
	if err != nil || dispatchPreview.Applied || dispatchPreview.BindingSHA256 == "" {
		return receipt, fmt.Errorf("adapter live acceptance dispatch preview failed: %w", err)
	}
	dispatchOpt.ExpectedAdapterExecutionDispatchBindingSHA256 = dispatchPreview.BindingSHA256
	dispatch, err := gate.RecordAdapterExecutionDispatch(repoRoot, caseRoot, liveAcceptancePack, dispatchOpt)
	if err != nil || !dispatch.Applied || dispatch.DispatchSHA256 == "" {
		return receipt, fmt.Errorf("adapter live acceptance dispatch Apply failed: %w", err)
	}
	receipt.DispatchPath = dispatch.DispatchPath
	receipt.DispatchSHA256 = dispatch.DispatchSHA256

	adapterResult, childPID, err := runLiveAdapter(adapterBinding, repoRoot, caseRoot, authorized.EventID, dispatch.DispatchSHA256, 10*time.Second)
	if err != nil {
		return receipt, err
	}
	if err := validateLiveAdapterResult(adapterResult, childPID, adapterSHA, caseRoot, receipt.Lane, authorized.EventID, receipt.AdapterSession, dispatch.DispatchPath, dispatch.DispatchSHA256); err != nil {
		return receipt, err
	}
	if err := adapterBinding.Validate(); err != nil {
		return receipt, fmt.Errorf("adapter executable changed during live acceptance: %w", err)
	}
	inputAfter, err := stableLiveFile(caseRoot, adapterResult.InputPath, maxFixtureBytes)
	if err != nil || !bytes.Equal(inputBefore, inputAfter) || sha256Hex(inputAfter) != adapterResult.InputSHA256 {
		return receipt, fmt.Errorf("adapter live process changed input bytes")
	}
	artifactData, err := stableLiveFile(caseRoot, adapterResult.ArtifactPath, 4<<20)
	if err != nil || sha256Hex(artifactData) != adapterResult.ArtifactSHA256 {
		return receipt, fmt.Errorf("adapter live artifact provenance mismatch")
	}
	reportData, err := stableLiveFile(caseRoot, adapterResult.ReportPath, 4<<20)
	if err != nil || sha256Hex(reportData) != adapterResult.ReportSHA256 {
		return receipt, fmt.Errorf("adapter live report provenance mismatch")
	}
	receipt.AdapterProcessID = adapterResult.ProcessID
	receipt.InputPath = adapterResult.InputPath
	receipt.InputSHA256 = adapterResult.InputSHA256
	receipt.ArtifactPath = adapterResult.ArtifactPath
	receipt.ArtifactSHA256 = adapterResult.ArtifactSHA256
	receipt.ReportPath = adapterResult.ReportPath
	receipt.ReportSHA256 = adapterResult.ReportSHA256

	receiptOpt := gate.Options{
		GateEventID: authorized.EventID, ExecutionReportPath: adapterResult.ReportPath,
		AdapterID: readonlyInspectorID, Executor: liveAcceptanceExecutor,
		ExpectedExecutorGeneration: 1, AdapterHarness: adapterHarness,
		AdapterSession: receipt.AdapterSession, ExecutionExitStatus: "completed",
		Actor: "mission-commander",
	}
	receiptPreview, err := gate.RecordAdapterExecutionReceipt(repoRoot, caseRoot, liveAcceptancePack, receiptOpt)
	if err != nil || receiptPreview.Applied || receiptPreview.BindingSHA256 == "" {
		return receipt, fmt.Errorf("adapter live acceptance receipt preview failed: %w", err)
	}
	receiptOpt.ExpectedAdapterExecutionBindingSHA256 = receiptPreview.BindingSHA256
	recordedReceipt, err := gate.RecordAdapterExecutionReceipt(repoRoot, caseRoot, liveAcceptancePack, receiptOpt)
	if err != nil || !recordedReceipt.Applied || recordedReceipt.ReceiptSHA256 == "" {
		return receipt, fmt.Errorf("adapter live acceptance receipt Apply failed: %w", err)
	}
	receipt.ReceiptPath = recordedReceipt.ReceiptPath
	receipt.ReceiptSHA256 = recordedReceipt.ReceiptSHA256

	validation, err := gate.ValidateAdapterExecutionReport(repoRoot, caseRoot, liveAcceptancePack, gate.Options{
		GateEventID: authorized.EventID, ExecutionReportPath: adapterResult.ReportPath,
	})
	if err != nil || !validation.Valid || !validation.ProvenanceValid || validation.AdapterExecutionReceiptSHA256 != recordedReceipt.ReceiptSHA256 {
		return receipt, fmt.Errorf("adapter live acceptance report validation failed: %w", err)
	}
	observation, err := gate.RecordExecution(repoRoot, caseRoot, liveAcceptancePack, gate.Options{
		GateEventID: authorized.EventID, Actor: "mission-commander",
		ExecutionReportPath:                   validation.ReportPath,
		ExpectedExecutionReportSHA256:         validation.RecordExpectedReportSHA256,
		AdapterExecutionReceiptPath:           validation.AdapterExecutionReceiptPath,
		ExpectedAdapterExecutionReceiptSHA256: validation.AdapterExecutionReceiptSHA256,
		Executor:                              liveAcceptanceExecutor, ExpectedExecutorGeneration: 1,
	})
	if err != nil || !observation.Applied || observation.ExecutionEvidence == nil || observation.MissionCommanderDriverReceipt == nil || observation.MissionCommanderDriverReceipt.State != "refreshed" {
		return receipt, fmt.Errorf("adapter live acceptance observation record failed: %w", err)
	}
	receipt.ObservationEventID = observation.EventID
	verifiedReceipt, _, verifiedReceiptSHA, present, err := gate.ReadAdapterExecutionReceipt(caseRoot, receipt.Lane, authorized.EventID)
	if err != nil || !present || verifiedReceipt == nil || verifiedReceiptSHA != recordedReceipt.ReceiptSHA256 {
		return receipt, fmt.Errorf("adapter live acceptance receipt lacks observation provenance: %w", err)
	}

	facts, err := mission.ReadStrictLedgerFacts(caseRoot)
	if err != nil {
		return receipt, err
	}
	review := mission.ExecutionEvidenceReviewItemsWithLedgerFacts(facts, receipt.Lane, nil, 0)
	if len(review) != 1 || review[0].Acknowledgement == nil {
		return receipt, fmt.Errorf("adapter live acceptance observation did not enter acknowledgement review")
	}
	ackTime := time.Now().UTC().Format(time.RFC3339Nano)
	ackOpt := note.Options{
		Kind: "verification", Lane: receipt.Lane,
		Subject:  "execution evidence review accepted",
		Summary:  "accepted recorded execution evidence for gateEventId " + authorized.EventID,
		Verifier: "manual-review", Verdict: "accepted", Status: "resolved",
		Related:      strings.Join([]string{review[0].EventID, review[0].GateEventID}, ","),
		Reason:       "reviewed outputRefs/evidenceRefs before closing execution evidence review",
		Target:       adapterResult.InputPath,
		EvidenceRefs: strings.Join([]string{adapterResult.ArtifactPath, adapterResult.ReportPath}, ","),
		CreatedAt:    ackTime,
	}
	ackPreview, err := note.Append(repoRoot, caseRoot, liveAcceptancePack, ackOpt, true)
	if err != nil || ackPreview.Applied || ackPreview.EventSHA256 == "" {
		return receipt, fmt.Errorf("adapter live acceptance acknowledgement preview failed: %w", err)
	}
	ackOpt.ExpectedEventSHA256 = ackPreview.EventSHA256
	acknowledged, err := note.Append(repoRoot, caseRoot, liveAcceptancePack, ackOpt, false)
	if err != nil || !acknowledged.Applied || acknowledged.MissionCommanderAction.State != "ready-to-continue" {
		return receipt, fmt.Errorf("adapter live acceptance acknowledgement Apply failed: %w", err)
	}
	receipt.AcknowledgementID = acknowledged.EventID
	receipt.MissionResumeState = acknowledged.MissionCommanderAction.State
	facts, err = mission.ReadStrictLedgerFacts(caseRoot)
	if err != nil {
		return receipt, err
	}
	if pending := mission.ExecutionEvidenceReviewItemsWithLedgerFacts(facts, receipt.Lane, nil, 0); len(pending) != 0 {
		return receipt, fmt.Errorf("adapter live acceptance acknowledgement did not clear evidence review")
	}
	for _, kind := range []string{"authority", "confirmed"} {
		if _, err := os.Lstat(filepath.Join(caseRoot, ".rekit", "facts", mission.FactFileName(kind))); !os.IsNotExist(err) {
			return receipt, fmt.Errorf("adapter live acceptance unexpectedly wrote %s ledger state", kind)
		}
	}
	inputFinal, err := stableLiveFile(caseRoot, adapterResult.InputPath, maxFixtureBytes)
	if err != nil || !bytes.Equal(inputBefore, inputFinal) || sha256Hex(inputFinal) != adapterResult.InputSHA256 {
		return receipt, fmt.Errorf("adapter live input changed before acceptance completion")
	}
	receipt.Passed = true
	receipt.CompletedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return receipt, nil
}

func WriteLiveAcceptanceReceipt(path string, receipt LiveAcceptanceReceipt) error {
	path, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return err
	}
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("adapter live acceptance receipt path is required")
	}
	anchorPath := filepath.VolumeName(path) + string(filepath.Separator)
	if anchorPath == "" {
		anchorPath = string(filepath.Separator)
	}
	rel, err := filepath.Rel(anchorPath, path)
	if err != nil || filepath.IsAbs(rel) || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("adapter live acceptance receipt path escapes its volume root: %s", path)
	}
	data, err := canonicalJSON(receipt)
	if err != nil {
		return err
	}
	if err := rekitfs.WriteNewExclusiveRegularFileAnchored(anchorPath, filepath.ToSlash(rel), "adapter live acceptance receipt", data); err != nil {
		return fmt.Errorf("publish adapter live acceptance receipt %s: %w", path, err)
	}
	return nil
}

func writeLiveFixture(caseRoot string) error {
	path := filepath.Join(caseRoot, filepath.FromSlash(liveAcceptanceTarget))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte("alpha\nbeta\n"), 0o600)
}

func runLiveAdapter(binding *processguard.ExecutableBinding, repoRoot, caseRoot, gateEventID, dispatchSHA string, timeout time.Duration) (Result, int, error) {
	args := []string{
		"-repo", repoRoot,
		"-target", caseRoot,
		"-pack", liveAcceptancePack,
		"-gate-event-id", gateEventID,
		"-expected-dispatch-sha256", dispatchSHA,
	}
	stdout, _, childPID, err := runContainedProcess(binding, args, nil, timeout)
	if err != nil {
		return Result{}, childPID, err
	}
	var result Result
	dec := json.NewDecoder(bytes.NewReader(stdout))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&result); err != nil {
		return Result{}, childPID, fmt.Errorf("decode adapter live result: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return Result{}, childPID, fmt.Errorf("adapter live result must contain exactly one JSON object")
	}
	return result, childPID, nil
}

func validateLiveAdapterResult(result Result, childPID int, adapterSHA, caseRoot, lane, gateEventID, adapterSession, dispatchPath, dispatchSHA string) error {
	reportPath := cleanCaseRelative(liveAcceptanceReport)
	artifactPath := filepath.ToSlash(filepath.Join(filepath.Dir(reportPath), "inspection.json"))
	if result.SchemaVersion != 1 || result.Kind != "rekit-readonly-adapter-host-result" ||
		!samePath(result.CaseRoot, caseRoot) || result.Pack != liveAcceptancePack || result.GateEventID != gateEventID ||
		result.Lane != lane || result.Executor != liveAcceptanceExecutor || result.Generation != 1 ||
		result.AdapterID != readonlyInspectorID || result.AdapterHarness != adapterHarness || result.AdapterSession != adapterSession ||
		result.ExecutableSHA256 != adapterSHA || result.ProcessID != childPID || childPID <= 0 || childPID == os.Getpid() ||
		result.DispatchPath != dispatchPath || !strings.EqualFold(result.DispatchSHA256, dispatchSHA) ||
		result.InputPath != liveAcceptanceTarget || result.ReportPath != reportPath || result.ArtifactPath != artifactPath ||
		!validSHA256(result.InputSHA256) || !validSHA256(result.ArtifactSHA256) || !validSHA256(result.ReportSHA256) ||
		!result.ReadOnlyInput || !result.NoNetwork || !result.NoAuthority {
		return fmt.Errorf("adapter live process returned inconsistent identity or provenance")
	}
	return nil
}

func stableLiveFile(caseRoot, rel string, limit int64) ([]byte, error) {
	full := filepath.Join(caseRoot, filepath.FromSlash(rel))
	return rekitfs.ReadStableRegularFileAnchored(caseRoot, full, "adapter live file", limit)
}

func samePath(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	return leftErr == nil && rightErr == nil && strings.EqualFold(filepath.Clean(leftAbs), filepath.Clean(rightAbs))
}

func pathWithin(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
