package executioncontrol

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/lanemutation"
	"github.com/shuiyu486/re-context-kits/internal/rekit/laneowner"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

const (
	heldResultDir = "held-results"

	ResultDispositionCurrent            = "current-for-publication"
	ResultDispositionPublished          = "published"
	ResultDispositionHeldWhilePaused    = "held-while-paused"
	ResultDispositionLateAfterStop      = "late-after-stop"
	ResultDispositionStaleControl       = "stale-control-generation"
	ResultDispositionControlHeadChanged = "control-head-changed"
	ResultDispositionStaleExecutor      = "stale-executor-generation"

	maxHeldResults = 10000
)

type ResultBirth struct {
	ControlGeneration    int                `json:"controlGeneration"`
	ControlReceiptSHA256 string             `json:"controlReceiptSha256,omitempty"`
	Owner                laneowner.Snapshot `json:"owner"`
}

type ResultSource struct {
	Kind          string `json:"kind"`
	Ref           string `json:"ref"`
	SHA256        string `json:"sha256"`
	Bytes         int64  `json:"bytes"`
	SessionKind   string `json:"sessionKind"`
	AttemptID     string `json:"attemptId"`
	AttemptSHA256 string `json:"attemptSha256"`
	SessionID     string `json:"sessionId"`
}

type ResultPublicationOptions struct {
	Lane       string
	Birth      ResultBirth
	Source     ResultSource
	Actor      string
	ObservedAt string
}

type ResultPublication struct {
	Published     bool   `json:"published"`
	Held          bool   `json:"held"`
	Disposition   string `json:"disposition"`
	ReceiptPath   string `json:"receiptPath,omitempty"`
	ReceiptSHA256 string `json:"receiptSha256,omitempty"`
}

type ResultHeldError struct {
	Publication ResultPublication
}

func (err *ResultHeldError) Error() string {
	if err == nil {
		return "execution result is held outside canonical publication"
	}
	return fmt.Sprintf("execution result disposition is %s; canonical publication did not run", err.Publication.Disposition)
}

type HeldResultReceipt struct {
	SchemaVersion               int                 `json:"schemaVersion"`
	Kind                        string              `json:"kind"`
	Lane                        string              `json:"lane"`
	Birth                       ResultBirth         `json:"birth"`
	ArrivalControlGeneration    int                 `json:"arrivalControlGeneration"`
	ArrivalControlReceiptSHA256 string              `json:"arrivalControlReceiptSha256,omitempty"`
	ArrivalControlState         string              `json:"arrivalControlState"`
	ArrivalControlPending       bool                `json:"arrivalControlPending"`
	ArrivalOwner                *laneowner.Snapshot `json:"arrivalOwner,omitempty"`
	Source                      ResultSource        `json:"source"`
	Disposition                 string              `json:"disposition"`
	Actor                       string              `json:"actor"`
	ObservedAt                  string              `json:"observedAt"`
	Reason                      string              `json:"reason"`
	ReceiptPath                 string              `json:"receiptPath"`
	Advanced                    bool                `json:"advanced"`
	CanonicalPublication        bool                `json:"canonicalPublication"`
	NoAuthority                 bool                `json:"noAuthority"`
	NoConfirmed                 bool                `json:"noConfirmed"`
	NoHeavyTool                 bool                `json:"noHeavyTool"`
	NoAutoResume                bool                `json:"noAutoResume"`
}

func PrepareResult(
	caseRoot string,
	opt ResultPublicationOptions,
	prepareCurrent ...func() error,
) (result ResultPublication, retErr error) {
	opt = normalizeResultPublicationOptions(opt)
	if err := validateResultPublicationOptions(opt); err != nil {
		return ResultPublication{}, err
	}
	if len(prepareCurrent) > 1 {
		return ResultPublication{}, fmt.Errorf("result preparation accepts at most one current callback")
	}
	lease, err := lanemutation.AcquireLane(caseRoot, opt.Lane)
	if err != nil {
		return ResultPublication{}, err
	}
	defer func() { retErr = errors.Join(retErr, lease.Unlock()) }()
	prepared, err := prepareResultWithLease(caseRoot, lease, opt)
	if err != nil || prepared.Held || len(prepareCurrent) == 0 {
		return prepared, err
	}
	if err := lease.ValidateLaneFor(caseRoot, opt.Lane); err != nil {
		return ResultPublication{}, err
	}
	if prepareCurrent[0] == nil {
		return ResultPublication{}, fmt.Errorf("result preparation current callback is nil")
	}
	if err := prepareCurrent[0](); err != nil {
		return ResultPublication{}, err
	}
	if err := lease.ValidateLaneFor(caseRoot, opt.Lane); err != nil {
		return ResultPublication{}, err
	}
	return prepared, nil
}

func PrepareResultWithLease(
	caseRoot string,
	lease *lanemutation.Lease,
	opt ResultPublicationOptions,
) (ResultPublication, error) {
	opt = normalizeResultPublicationOptions(opt)
	if err := validateResultPublicationOptions(opt); err != nil {
		return ResultPublication{}, err
	}
	if lease == nil {
		return ResultPublication{}, fmt.Errorf("result preparation requires an existing workstream mutation lease")
	}
	if err := lease.ValidateLaneFor(caseRoot, opt.Lane); err != nil {
		return ResultPublication{}, err
	}
	return prepareResultWithLease(caseRoot, lease, opt)
}

func PrepareResultWithProjectLease(
	caseRoot string,
	lease *lanemutation.Lease,
	opt ResultPublicationOptions,
) (ResultPublication, error) {
	opt = normalizeResultPublicationOptions(opt)
	if err := validateResultPublicationOptions(opt); err != nil {
		return ResultPublication{}, err
	}
	if lease == nil {
		return ResultPublication{}, fmt.Errorf("result preparation requires an existing project mutation lease")
	}
	validate := func() error { return lease.ValidateProjectFor(caseRoot) }
	if err := validate(); err != nil {
		return ResultPublication{}, err
	}
	return prepareResultWithValidation(caseRoot, opt, validate)
}

func PublishResult(
	caseRoot string,
	opt ResultPublicationOptions,
	publishCanonical func() error,
) (result ResultPublication, retErr error) {
	opt = normalizeResultPublicationOptions(opt)
	if err := validateResultPublication(opt, publishCanonical); err != nil {
		return ResultPublication{}, err
	}
	lease, err := lanemutation.AcquireLane(caseRoot, opt.Lane)
	if err != nil {
		return ResultPublication{}, err
	}
	defer func() { retErr = errors.Join(retErr, lease.Unlock()) }()
	return publishResultWithLease(caseRoot, lease, opt, publishCanonical)
}

func PublishResultWithLease(
	caseRoot string,
	lease *lanemutation.Lease,
	opt ResultPublicationOptions,
	publishCanonical func() error,
) (ResultPublication, error) {
	opt = normalizeResultPublicationOptions(opt)
	if err := validateResultPublication(opt, publishCanonical); err != nil {
		return ResultPublication{}, err
	}
	if lease == nil {
		return ResultPublication{}, fmt.Errorf("result publication requires an existing workstream mutation lease")
	}
	if err := lease.ValidateLaneFor(caseRoot, opt.Lane); err != nil {
		return ResultPublication{}, err
	}
	return publishResultWithLease(caseRoot, lease, opt, publishCanonical)
}

func PublishResultWithProjectLease(
	caseRoot string,
	lease *lanemutation.Lease,
	opt ResultPublicationOptions,
	publishCanonical func() error,
) (ResultPublication, error) {
	opt = normalizeResultPublicationOptions(opt)
	if err := validateResultPublication(opt, publishCanonical); err != nil {
		return ResultPublication{}, err
	}
	if lease == nil {
		return ResultPublication{}, fmt.Errorf("result publication requires an existing project mutation lease")
	}
	validate := func() error { return lease.ValidateProjectFor(caseRoot) }
	if err := validate(); err != nil {
		return ResultPublication{}, err
	}
	return publishResultWithValidation(caseRoot, opt, publishCanonical, validate)
}

func normalizeResultPublicationOptions(opt ResultPublicationOptions) ResultPublicationOptions {
	opt.Lane = strings.TrimSpace(opt.Lane)
	opt.Actor = strings.TrimSpace(opt.Actor)
	opt.ObservedAt = strings.TrimSpace(opt.ObservedAt)
	return opt
}

func validateResultPublication(opt ResultPublicationOptions, publishCanonical func() error) error {
	if publishCanonical == nil {
		return fmt.Errorf("result publication requires a canonical publication callback")
	}
	return validateResultPublicationOptions(opt)
}

func publishResultWithLease(
	caseRoot string,
	lease *lanemutation.Lease,
	opt ResultPublicationOptions,
	publishCanonical func() error,
) (ResultPublication, error) {
	return publishResultWithValidation(
		caseRoot,
		opt,
		publishCanonical,
		func() error { return lease.ValidateLaneFor(caseRoot, opt.Lane) },
	)
}

func publishResultWithValidation(
	caseRoot string,
	opt ResultPublicationOptions,
	publishCanonical func() error,
	validateOwner func() error,
) (ResultPublication, error) {
	prepared, err := prepareResultWithValidation(caseRoot, opt, validateOwner)
	if err != nil || prepared.Held {
		return prepared, err
	}
	if prepared.Disposition != ResultDispositionCurrent {
		return ResultPublication{}, fmt.Errorf("result preparation returned unexpected disposition %q", prepared.Disposition)
	}
	if err := validateOwner(); err != nil {
		return ResultPublication{}, err
	}
	if err := publishCanonical(); err != nil {
		return ResultPublication{}, err
	}
	if err := validateOwner(); err != nil {
		return ResultPublication{}, fmt.Errorf("canonical result publication may already be durable: %w", err)
	}
	return ResultPublication{Published: true, Disposition: ResultDispositionPublished}, nil
}

func prepareResultWithLease(
	caseRoot string,
	lease *lanemutation.Lease,
	opt ResultPublicationOptions,
) (ResultPublication, error) {
	return prepareResultWithValidation(
		caseRoot,
		opt,
		func() error { return lease.ValidateLaneFor(caseRoot, opt.Lane) },
	)
}

func prepareResultWithValidation(
	caseRoot string,
	opt ResultPublicationOptions,
	validateOwner func() error,
) (ResultPublication, error) {
	if validateOwner == nil {
		return ResultPublication{}, fmt.Errorf("result preparation requires a mutation owner validator")
	}
	if err := validateOwner(); err != nil {
		return ResultPublication{}, err
	}
	inspection, err := Inspect(caseRoot, opt.Lane)
	if err != nil {
		return ResultPublication{}, err
	}
	currentOwner, err := laneowner.Read(caseRoot, opt.Lane)
	if err != nil {
		return ResultPublication{}, err
	}
	heldPath, err := heldResultPath(caseRoot, opt)
	if err != nil {
		return ResultPublication{}, err
	}
	if existing, data, ok, err := readHeldResult(caseRoot, heldPath); err != nil {
		return ResultPublication{}, err
	} else if ok {
		if err := validateHeldResultReceipt(existing, opt, heldPath); err != nil {
			return ResultPublication{}, err
		}
		return ResultPublication{
			Held:          true,
			Disposition:   existing.Disposition,
			ReceiptPath:   existing.ReceiptPath,
			ReceiptSHA256: hash(data),
		}, nil
	}
	disposition, reason := classifyResultPublication(opt.Birth, inspection, currentOwner)
	if disposition == ResultDispositionCurrent {
		return ResultPublication{Disposition: disposition}, nil
	}
	receipt, data, err := heldResultFor(caseRoot, opt, inspection, currentOwner, disposition, reason)
	if err != nil {
		return ResultPublication{}, err
	}
	if err := ensureHeldResultCapacity(caseRoot, opt.Lane, filepath.Base(receipt.ReceiptPath)); err != nil {
		return ResultPublication{}, err
	}
	if err := publish(caseRoot, receipt.ReceiptPath, "held execution result receipt", data); err != nil {
		return ResultPublication{}, err
	}
	if err := validateOwner(); err != nil {
		return ResultPublication{}, fmt.Errorf("held execution result receipt may already be durable: %w", err)
	}
	return ResultPublication{
		Held:          true,
		Disposition:   disposition,
		ReceiptPath:   receipt.ReceiptPath,
		ReceiptSHA256: hash(data),
	}, nil
}

func validateResultPublicationOptions(opt ResultPublicationOptions) error {
	if opt.Lane == "" || opt.Birth.Owner.Lane != opt.Lane ||
		strings.TrimSpace(opt.Birth.Owner.CurrentExecutor) == "" || opt.Birth.Owner.ExecutorGeneration < 1 ||
		opt.Birth.ControlGeneration < 0 {
		return fmt.Errorf("result publication requires an exact lane, birth control head, and durable owner")
	}
	if opt.Birth.ControlGeneration == 0 {
		if strings.TrimSpace(opt.Birth.ControlReceiptSHA256) != "" {
			return fmt.Errorf("initial result birth control head must not name a receipt")
		}
	} else if !validSHA256(opt.Birth.ControlReceiptSHA256) {
		return fmt.Errorf("result birth control receipt sha256 is invalid")
	}
	if opt.Actor == "" || strings.ContainsAny(opt.Actor, "\r\n") {
		return fmt.Errorf("result publication actor must be one non-empty line")
	}
	if _, err := time.Parse(time.RFC3339Nano, opt.ObservedAt); err != nil {
		return fmt.Errorf("result publication observedAt must be RFC3339Nano: %w", err)
	}
	for name, value := range map[string]string{
		"source kind": opt.Source.Kind, "source ref": opt.Source.Ref,
		"session kind": opt.Source.SessionKind, "attempt id": opt.Source.AttemptID,
		"attempt sha256": opt.Source.AttemptSHA256, "session id": opt.Source.SessionID,
	} {
		if strings.TrimSpace(value) == "" || strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("result publication %s must be one non-empty line", name)
		}
	}
	if !validSHA256(opt.Source.SHA256) || opt.Source.Bytes < 0 {
		return fmt.Errorf("result publication source requires exact bounded bytes and sha256")
	}
	return nil
}

func classifyResultPublication(birth ResultBirth, inspection Inspection, currentOwner laneowner.Snapshot) (string, string) {
	switch {
	case inspection.Pending:
		return ResultDispositionControlHeadChanged, fmt.Sprintf("execution control generation %d is pending", inspection.PendingGeneration)
	case inspection.State == StatePaused:
		return ResultDispositionHeldWhilePaused, "lane execution is paused"
	case inspection.State == StateStopped:
		return ResultDispositionLateAfterStop, "lane execution is durably stopped"
	case inspection.CurrentGeneration != birth.ControlGeneration ||
		!strings.EqualFold(inspection.CurrentReceiptSHA256, birth.ControlReceiptSHA256):
		return ResultDispositionStaleControl, "result was produced under an earlier execution control head"
	case currentOwner != birth.Owner:
		return ResultDispositionStaleExecutor, "result was produced by an earlier durable executor generation"
	default:
		return ResultDispositionCurrent, ""
	}
}

func heldResultPath(caseRoot string, opt ResultPublicationOptions) (string, error) {
	identity := struct {
		Lane   string       `json:"lane"`
		Birth  ResultBirth  `json:"birth"`
		Source ResultSource `json:"source"`
	}{Lane: opt.Lane, Birth: opt.Birth, Source: opt.Source}
	identityData, err := json.Marshal(identity)
	if err != nil {
		return "", err
	}
	return projectstate.Rel(
		caseRoot,
		"lanes", opt.Lane, controlDir, heldResultDir,
		hash(identityData)+".json",
	)
}

func readHeldResult(caseRoot, rel string) (HeldResultReceipt, []byte, bool, error) {
	path, err := rekitfs.SafeJoin(caseRoot, rel)
	if err != nil {
		return HeldResultReceipt{}, nil, false, err
	}
	data, err := rekitfs.ReadStableRegularFileAnchored(caseRoot, path, "held execution result receipt", maxJSONBytes)
	if os.IsNotExist(err) {
		return HeldResultReceipt{}, nil, false, nil
	}
	if err != nil {
		return HeldResultReceipt{}, nil, false, err
	}
	var receipt HeldResultReceipt
	if err := decodeStrict(data, &receipt); err != nil {
		return HeldResultReceipt{}, nil, false, fmt.Errorf("decode held execution result receipt: %w", err)
	}
	return receipt, data, true, nil
}

func validateHeldResultReceipt(receipt HeldResultReceipt, opt ResultPublicationOptions, expectedPath string) error {
	if receipt.SchemaVersion != 1 || receipt.Kind != "lane-execution-held-result" ||
		receipt.Lane != opt.Lane || receipt.Birth != opt.Birth || receipt.Source != opt.Source ||
		receipt.ReceiptPath != expectedPath || receipt.Advanced || receipt.CanonicalPublication ||
		!receipt.NoAuthority || !receipt.NoConfirmed || !receipt.NoHeavyTool || !receipt.NoAutoResume ||
		strings.TrimSpace(receipt.Actor) == "" || strings.TrimSpace(receipt.ObservedAt) == "" || strings.TrimSpace(receipt.Reason) == "" {
		return fmt.Errorf("held execution result receipt does not match its exact raw result identity and strict boundary")
	}
	if receipt.ArrivalOwner == nil || receipt.ArrivalOwner.Lane != opt.Lane ||
		!strings.Contains("|"+strings.Join([]string{
			ResultDispositionHeldWhilePaused,
			ResultDispositionLateAfterStop,
			ResultDispositionStaleControl,
			ResultDispositionControlHeadChanged,
			ResultDispositionStaleExecutor,
		}, "|")+"|", "|"+receipt.Disposition+"|") {
		return fmt.Errorf("held execution result receipt has invalid arrival or disposition")
	}
	if _, err := time.Parse(time.RFC3339Nano, receipt.ObservedAt); err != nil {
		return fmt.Errorf("held execution result receipt observedAt is invalid: %w", err)
	}
	return nil
}

func heldResultFor(
	caseRoot string,
	opt ResultPublicationOptions,
	inspection Inspection,
	currentOwner laneowner.Snapshot,
	disposition,
	reason string,
) (HeldResultReceipt, []byte, error) {
	rel, err := heldResultPath(caseRoot, opt)
	if err != nil {
		return HeldResultReceipt{}, nil, err
	}
	owner := currentOwner
	receipt := HeldResultReceipt{
		SchemaVersion: 1, Kind: "lane-execution-held-result", Lane: opt.Lane, Birth: opt.Birth,
		ArrivalControlGeneration:    inspection.CurrentGeneration,
		ArrivalControlReceiptSHA256: inspection.CurrentReceiptSHA256,
		ArrivalControlState:         inspection.State, ArrivalControlPending: inspection.Pending,
		ArrivalOwner: &owner, Source: opt.Source, Disposition: disposition,
		Actor: opt.Actor, ObservedAt: opt.ObservedAt, Reason: reason, ReceiptPath: rel,
		Advanced: false, CanonicalPublication: false,
		NoAuthority: true, NoConfirmed: true, NoHeavyTool: true, NoAutoResume: true,
	}
	data, err := canonical(receipt)
	return receipt, data, err
}

func ensureHeldResultCapacity(caseRoot, lane, pendingName string) error {
	stateRoot, err := projectstate.Resolve(caseRoot)
	if err != nil {
		return err
	}
	root, err := rekitfs.OpenAnchoredRoot(stateRoot.Path)
	if err != nil {
		return err
	}
	defer root.Close()
	rel := filepath.Join("lanes", lane, controlDir, heldResultDir)
	entries, err := root.ListNoFollow(rel, maxHeldResults)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		info, infoErr := entry.Info()
		if infoErr != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || !strings.HasSuffix(entry.Name(), ".json") {
			return fmt.Errorf("unexpected held execution result artifact: %s", entry.Name())
		}
		if entry.Name() == pendingName {
			return nil
		}
	}
	if len(entries) >= maxHeldResults {
		return fmt.Errorf("lane contains more than %d held execution results", maxHeldResults)
	}
	return nil
}
