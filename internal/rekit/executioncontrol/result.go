package executioncontrol

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/capabilitycontract"
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

const (
	LegacyResultBirthSchemaVersion       = 1
	LegacyHeldResultReceiptSchemaVersion = 1
	HeldResultReceiptSchemaVersion       = 2
)

type ResultBirth struct {
	SchemaVersion        int                        `json:"schemaVersion"`
	ControlGeneration    int                        `json:"controlGeneration"`
	ControlReceiptSHA256 string                     `json:"controlReceiptSha256,omitempty"`
	Owner                laneowner.Snapshot         `json:"owner"`
	Capability           capabilitycontract.Binding `json:"capability"`
}

// LegacyResultBirth is retained only for strict historical inspection. It is
// never normalized into a current ResultBirth.
type LegacyResultBirth struct {
	ControlGeneration    int                `json:"controlGeneration"`
	ControlReceiptSHA256 string             `json:"controlReceiptSha256,omitempty"`
	Owner                laneowner.Snapshot `json:"owner"`
}

type VersionedResultBirth struct {
	Version     int
	Current     *ResultBirth
	Legacy      *LegacyResultBirth
	Raw         []byte
	WholeSHA256 string
}

func DecodeVersionedResultBirth(data []byte) (VersionedResultBirth, error) {
	var probe map[string]json.RawMessage
	if err := decodeStrict(data, &probe); err != nil {
		return VersionedResultBirth{}, fmt.Errorf("decode result birth: %w", err)
	}
	result := VersionedResultBirth{Raw: append([]byte(nil), data...), WholeSHA256: hash(data)}
	if rawVersion, ok := probe["schemaVersion"]; ok {
		if err := json.Unmarshal(rawVersion, &result.Version); err != nil {
			return VersionedResultBirth{}, fmt.Errorf("result birth schema version is invalid: %w", err)
		}
		if result.Version != ResultBirthSchemaVersion {
			return VersionedResultBirth{}, fmt.Errorf("result birth schema version is unsupported: %d", result.Version)
		}
		var current ResultBirth
		if err := decodeStrict(data, &current); err != nil {
			return VersionedResultBirth{}, fmt.Errorf("decode current result birth: %w", err)
		}
		if err := validateResultBirth(current); err != nil {
			return VersionedResultBirth{}, err
		}
		result.Current = &current
		return result, nil
	}
	result.Version = LegacyResultBirthSchemaVersion
	var legacy legacyResultBirthWire
	if err := decodeStrict(data, &legacy); err != nil {
		return VersionedResultBirth{}, fmt.Errorf("decode legacy result birth: %w", err)
	}
	if legacy.SchemaVersion != nil && *legacy.SchemaVersion != LegacyResultBirthSchemaVersion {
		return VersionedResultBirth{}, fmt.Errorf("legacy result birth schema version is unsupported: %d", *legacy.SchemaVersion)
	}
	value := LegacyResultBirth{
		ControlGeneration:    legacy.ControlGeneration,
		ControlReceiptSHA256: legacy.ControlReceiptSHA256,
		Owner:                legacy.Owner,
	}
	if err := validateLegacyResultBirth(value); err != nil {
		return VersionedResultBirth{}, err
	}
	result.Legacy = &value
	return result, nil
}

type legacyResultBirthWire struct {
	SchemaVersion        *int               `json:"schemaVersion,omitempty"`
	ControlGeneration    int                `json:"controlGeneration"`
	ControlReceiptSHA256 string             `json:"controlReceiptSHA256,omitempty"`
	Owner                laneowner.Snapshot `json:"owner"`
}

func validateLegacyResultBirth(birth LegacyResultBirth) error {
	if birth.ControlGeneration < 0 || birth.Owner.ExecutorGeneration < 1 ||
		strings.TrimSpace(birth.Owner.Lane) == "" || strings.TrimSpace(birth.Owner.CurrentExecutor) == "" {
		return fmt.Errorf("legacy result birth owner or control head is invalid")
	}
	if birth.ControlGeneration == 0 {
		if strings.TrimSpace(birth.ControlReceiptSHA256) != "" {
			return fmt.Errorf("legacy initial result birth control head must not name a receipt")
		}
	} else if !validSHA256(birth.ControlReceiptSHA256) {
		return fmt.Errorf("legacy result birth control receipt sha256 is invalid")
	}
	return nil
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

type LegacyHeldResultReceipt struct {
	SchemaVersion               int                 `json:"schemaVersion"`
	Kind                        string              `json:"kind"`
	Lane                        string              `json:"lane"`
	Birth                       LegacyResultBirth   `json:"birth"`
	ArrivalControlGeneration    int                 `json:"arrivalControlGeneration"`
	ArrivalControlReceiptSHA256 string              `json:"arrivalControlReceiptSHA256,omitempty"`
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

func validateLegacyHeldResultReceipt(receipt LegacyHeldResultReceipt) error {
	if receipt.SchemaVersion != LegacyHeldResultReceiptSchemaVersion || receipt.Kind != "lane-execution-held-result" ||
		strings.TrimSpace(receipt.Lane) == "" || receipt.Birth.Owner.Lane != receipt.Lane ||
		receipt.Advanced || receipt.CanonicalPublication || !receipt.NoAuthority || !receipt.NoConfirmed ||
		!receipt.NoHeavyTool || !receipt.NoAutoResume || strings.TrimSpace(receipt.ReceiptPath) == "" ||
		strings.TrimSpace(receipt.Actor) == "" || strings.TrimSpace(receipt.Reason) == "" {
		return fmt.Errorf("legacy held execution result receipt has invalid identity or strict boundary")
	}
	if err := validateLegacyResultBirth(receipt.Birth); err != nil {
		return err
	}
	if receipt.ArrivalControlGeneration < 0 {
		return fmt.Errorf("legacy held execution result arrival control generation is invalid")
	}
	if receipt.ArrivalControlGeneration == 0 {
		if strings.TrimSpace(receipt.ArrivalControlReceiptSHA256) != "" {
			return fmt.Errorf("legacy initial held execution result arrival must not name a receipt")
		}
	} else if !validSHA256(receipt.ArrivalControlReceiptSHA256) {
		return fmt.Errorf("legacy held execution result arrival receipt sha256 is invalid")
	}
	if receipt.ArrivalOwner == nil || receipt.ArrivalOwner.Lane != receipt.Lane ||
		strings.TrimSpace(receipt.ArrivalOwner.CurrentExecutor) == "" || receipt.ArrivalOwner.ExecutorGeneration < 1 {
		return fmt.Errorf("legacy held execution result arrival owner is invalid")
	}
	if !strings.Contains("|"+strings.Join([]string{
		ResultDispositionHeldWhilePaused,
		ResultDispositionLateAfterStop,
		ResultDispositionStaleControl,
		ResultDispositionControlHeadChanged,
		ResultDispositionStaleExecutor,
	}, "|")+"|", "|"+receipt.Disposition+"|") {
		return fmt.Errorf("legacy held execution result disposition is invalid")
	}
	if err := validateLegacyResultSource(receipt.Source); err != nil {
		return err
	}
	if _, err := time.Parse(time.RFC3339Nano, receipt.ObservedAt); err != nil {
		return fmt.Errorf("legacy held execution result observedAt is invalid: %w", err)
	}
	return nil
}

func validateLegacyResultSource(source ResultSource) error {
	for name, value := range map[string]string{
		"kind": source.Kind, "ref": source.Ref, "session kind": source.SessionKind,
		"attempt id": source.AttemptID, "attempt sha256": source.AttemptSHA256, "session id": source.SessionID,
	} {
		if strings.TrimSpace(value) == "" || strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("legacy held execution result source %s is invalid", name)
		}
	}
	if !validSHA256(source.SHA256) || source.Bytes < 0 {
		return fmt.Errorf("legacy held execution result source requires bounded bytes and sha256")
	}
	return nil
}

type VersionedHeldResultReceipt struct {
	Version     int
	Current     *HeldResultReceipt
	Legacy      *LegacyHeldResultReceipt
	Raw         []byte
	WholeSHA256 string
}

func DecodeVersionedHeldResultReceipt(data []byte) (VersionedHeldResultReceipt, error) {
	var envelope map[string]json.RawMessage
	if err := decodeStrict(data, &envelope); err != nil {
		return VersionedHeldResultReceipt{}, fmt.Errorf("decode held execution result receipt: %w", err)
	}
	rawVersion, ok := envelope["schemaVersion"]
	if !ok {
		return VersionedHeldResultReceipt{}, fmt.Errorf("held execution result receipt schemaVersion is required")
	}
	var version int
	if err := json.Unmarshal(rawVersion, &version); err != nil {
		return VersionedHeldResultReceipt{}, fmt.Errorf("held execution result receipt schemaVersion is invalid: %w", err)
	}
	result := VersionedHeldResultReceipt{
		Version: version, Raw: append([]byte(nil), data...), WholeSHA256: hash(data),
	}
	switch version {
	case HeldResultReceiptSchemaVersion:
		var current HeldResultReceipt
		if err := decodeStrict(data, &current); err != nil {
			return VersionedHeldResultReceipt{}, fmt.Errorf("decode current held execution result receipt: %w", err)
		}
		result.Current = &current
	case LegacyHeldResultReceiptSchemaVersion:
		var legacy LegacyHeldResultReceipt
		if err := decodeStrict(data, &legacy); err != nil {
			return VersionedHeldResultReceipt{}, fmt.Errorf("decode legacy held execution result receipt: %w", err)
		}
		if err := validateLegacyHeldResultReceipt(legacy); err != nil {
			return VersionedHeldResultReceipt{}, err
		}
		result.Legacy = &legacy
	default:
		return VersionedHeldResultReceipt{}, fmt.Errorf("held execution result receipt schema version is unsupported: %d", version)
	}
	return result, nil
}

// ReadHeldResultHistory reads one immutable held-result artifact for diagnostic
// and exact replay inspection. It never converts legacy data into a current
// receipt and never authorizes publication.
func ReadHeldResultHistory(caseRoot, rel string) (VersionedHeldResultReceipt, bool, error) {
	path, err := rekitfs.SafeJoin(caseRoot, rel)
	if err != nil {
		return VersionedHeldResultReceipt{}, false, err
	}
	data, err := rekitfs.ReadStableRegularFileAnchored(caseRoot, path, "held execution result receipt history", maxJSONBytes)
	if errors.Is(err, os.ErrNotExist) {
		return VersionedHeldResultReceipt{}, false, nil
	}
	if err != nil {
		return VersionedHeldResultReceipt{}, false, err
	}
	result, err := DecodeVersionedHeldResultReceipt(data)
	if err != nil {
		return VersionedHeldResultReceipt{}, false, err
	}
	return result, true, nil
}

type HeldResultReceipt struct {
	SchemaVersion               int                        `json:"schemaVersion"`
	Kind                        string                     `json:"kind"`
	Lane                        string                     `json:"lane"`
	Birth                       ResultBirth                `json:"birth"`
	ArrivalControlGeneration    int                        `json:"arrivalControlGeneration"`
	ArrivalControlReceiptSHA256 string                     `json:"arrivalControlReceiptSha256,omitempty"`
	ArrivalControlState         string                     `json:"arrivalControlState"`
	ArrivalControlPending       bool                       `json:"arrivalControlPending"`
	ArrivalOwner                *laneowner.Snapshot        `json:"arrivalOwner,omitempty"`
	Source                      ResultSource               `json:"source"`
	Capability                  capabilitycontract.Binding `json:"capability"`
	Disposition                 string                     `json:"disposition"`
	Actor                       string                     `json:"actor"`
	ObservedAt                  string                     `json:"observedAt"`
	Reason                      string                     `json:"reason"`
	ReceiptPath                 string                     `json:"receiptPath"`
	Advanced                    bool                       `json:"advanced"`
	CanonicalPublication        bool                       `json:"canonicalPublication"`
	NoAuthority                 bool                       `json:"noAuthority"`
	NoConfirmed                 bool                       `json:"noConfirmed"`
	NoHeavyTool                 bool                       `json:"noHeavyTool"`
	NoAutoResume                bool                       `json:"noAutoResume"`
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
	legacyHeldPath, err := legacyHeldResultPath(caseRoot, opt)
	if err != nil {
		return ResultPublication{}, err
	}
	if history, found, err := ReadHeldResultHistory(caseRoot, legacyHeldPath); err != nil {
		return ResultPublication{}, err
	} else if found {
		if history.Legacy == nil {
			return ResultPublication{}, fmt.Errorf("current held execution result receipt occupies its legacy history path")
		}
		return ResultPublication{}, fmt.Errorf("legacy held execution result receipt is decode-only and cannot enter current publication")
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

func validateResultBirth(birth ResultBirth) error {
	if birth.SchemaVersion != ResultBirthSchemaVersion {
		return fmt.Errorf("result birth schema version is unsupported: %d", birth.SchemaVersion)
	}
	if err := capabilitycontract.ValidateBinding(birth.Capability); err != nil {
		return fmt.Errorf("result birth capability is invalid: %w", err)
	}
	if birth.ControlGeneration < 0 || birth.Owner.ExecutorGeneration < 1 ||
		strings.TrimSpace(birth.Owner.Lane) == "" ||
		strings.TrimSpace(birth.Owner.CurrentExecutor) == "" {
		return fmt.Errorf("result birth owner or control head is invalid")
	}
	if birth.ControlGeneration == 0 {
		if strings.TrimSpace(birth.ControlReceiptSHA256) != "" {
			return fmt.Errorf("initial result birth control head must not name a receipt")
		}
	} else if !validSHA256(birth.ControlReceiptSHA256) {
		return fmt.Errorf("result birth control receipt sha256 is invalid")
	}
	return nil
}

func validateResultPublicationOptions(opt ResultPublicationOptions) error {
	if opt.Lane == "" || opt.Birth.Owner.Lane != opt.Lane ||
		strings.TrimSpace(opt.Birth.Owner.CurrentExecutor) == "" || opt.Birth.Owner.ExecutorGeneration < 1 ||
		opt.Birth.ControlGeneration < 0 {
		return fmt.Errorf("result publication requires an exact lane, birth control head, and durable owner")
	}
	if err := validateResultBirth(opt.Birth); err != nil {
		return err
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

func legacyHeldResultPath(caseRoot string, opt ResultPublicationOptions) (string, error) {
	identity := struct {
		Lane   string            `json:"lane"`
		Birth  LegacyResultBirth `json:"birth"`
		Source ResultSource      `json:"source"`
	}{
		Lane: opt.Lane,
		Birth: LegacyResultBirth{
			ControlGeneration:    opt.Birth.ControlGeneration,
			ControlReceiptSHA256: opt.Birth.ControlReceiptSHA256,
			Owner:                opt.Birth.Owner,
		},
		Source: opt.Source,
	}
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

func heldResultPath(caseRoot string, opt ResultPublicationOptions) (string, error) {
	identity := struct {
		Lane       string                     `json:"lane"`
		Birth      ResultBirth                `json:"birth"`
		Source     ResultSource               `json:"source"`
		Capability capabilitycontract.Binding `json:"capability"`
	}{Lane: opt.Lane, Birth: opt.Birth, Source: opt.Source, Capability: opt.Birth.Capability}
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
	versioned, found, err := ReadHeldResultHistory(caseRoot, rel)
	if err != nil || !found {
		return HeldResultReceipt{}, nil, found, err
	}
	if versioned.Current == nil {
		return HeldResultReceipt{}, nil, false, fmt.Errorf("legacy held execution result receipt is decode-only and cannot enter current publication")
	}
	return *versioned.Current, versioned.Raw, true, nil
}

func validateHeldResultReceipt(receipt HeldResultReceipt, opt ResultPublicationOptions, expectedPath string) error {
	if receipt.SchemaVersion != HeldResultReceiptSchemaVersion || receipt.Kind != "lane-execution-held-result" ||
		receipt.Lane != opt.Lane || receipt.Birth != opt.Birth || receipt.Source != opt.Source ||
		receipt.ReceiptPath != expectedPath || receipt.Advanced || receipt.CanonicalPublication ||
		!receipt.NoAuthority || !receipt.NoConfirmed || !receipt.NoHeavyTool || !receipt.NoAutoResume ||
		strings.TrimSpace(receipt.Actor) == "" || strings.TrimSpace(receipt.ObservedAt) == "" || strings.TrimSpace(receipt.Reason) == "" {
		return fmt.Errorf("held execution result receipt does not match its exact raw result identity and strict boundary")
	}
	if err := capabilitycontract.ValidateBinding(receipt.Capability); err != nil {
		return fmt.Errorf("held execution result capability contract is invalid: %w", err)
	}
	if receipt.Capability != opt.Birth.Capability {
		return fmt.Errorf("held execution result capability contract does not match its exact birth lineage")
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
		SchemaVersion: HeldResultReceiptSchemaVersion, Kind: "lane-execution-held-result", Lane: opt.Lane, Birth: opt.Birth,
		ArrivalControlGeneration:    inspection.CurrentGeneration,
		ArrivalControlReceiptSHA256: inspection.CurrentReceiptSHA256,
		ArrivalControlState:         inspection.State, ArrivalControlPending: inspection.Pending,
		ArrivalOwner: &owner, Source: opt.Source, Capability: opt.Birth.Capability, Disposition: disposition,
		Actor: opt.Actor, ObservedAt: opt.ObservedAt, Reason: reason, ReceiptPath: rel,
		Advanced: false, CanonicalPublication: false,
		NoAuthority: true, NoConfirmed: true, NoHeavyTool: true, NoAutoResume: true,
	}
	data, err := canonical(receipt)
	return receipt, data, err
}

func ensureHeldResultCapacity(caseRoot, lane, pendingName string) error {
	view, err := projectstate.ResolveMissionView(caseRoot)
	if err != nil {
		return err
	}
	root, err := rekitfs.OpenAnchoredRoot(view.Path)
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
