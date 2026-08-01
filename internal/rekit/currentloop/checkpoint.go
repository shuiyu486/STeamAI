package currentloop

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/lanemutation"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

const (
	artifactRelRoot  = ".rekit/runs/current-loop-segments"
	maxArtifactBytes = 1 << 20
	recordedAtLayout = "20060102T150405.000000000Z"
	sequenceWidth    = 20
)

var (
	artifactNamePattern = regexp.MustCompile(`^([0-9]{20})\.json$`)
	tempNamePattern     = regexp.MustCompile(`^\.[0-9]{20}\.[a-f0-9]{32}\.json\.tmp$`)
	acquireProject      = lanemutation.AcquireProject
	nowUTC              = func() time.Time { return time.Now().UTC() }
)

type ObservationAlternative struct {
	Kind          string   `json:"kind"`
	RequiredFlags []string `json:"requiredFlags"`
	Constraints   []string `json:"constraints"`
}

type ObservationContract struct {
	Alternatives []ObservationAlternative `json:"alternatives"`
	Boundary     []string                 `json:"boundary"`
}

type Continuation struct {
	Kind                  string               `json:"kind"`
	State                 string               `json:"state"`
	StopCode              string               `json:"stopCode"`
	SegmentMaxSteps       int                  `json:"segmentMaxSteps"`
	AppliedStepsInSegment int                  `json:"appliedStepsInSegment"`
	RemainingMaxSteps     int                  `json:"remainingMaxSteps"`
	SegmentRoute          string               `json:"segmentRoute"`
	SegmentLane           string               `json:"segmentLane"`
	ExpectedRoute         string               `json:"expectedRoute"`
	ExpectedLane          string               `json:"expectedLane"`
	WhatIfCommand         string               `json:"whatIfCommand"`
	ObservationContract   *ObservationContract `json:"observationContract,omitempty"`
	FreshPreviewRequired  bool                 `json:"freshPreviewRequired"`
	CumulativeReceipts    bool                 `json:"cumulativeReceipts"`
	Boundary              []string             `json:"boundary"`
}

type StepReceipt struct {
	State                         string                                 `json:"state"`
	Outcome                       string                                 `json:"outcome"`
	Route                         string                                 `json:"route"`
	NestedCommand                 string                                 `json:"nestedCommand"`
	RefreshedCurrentDriverRequest *mission.MissionCommanderDriverRequest `json:"refreshedCurrentDriverRequest,omitempty"`
	Boundary                      []string                               `json:"boundary"`
}

type StepReceiptBinding struct {
	Step                          int                                    `json:"step"`
	Route                         string                                 `json:"route"`
	Lane                          string                                 `json:"lane"`
	RunLoopStepID                 string                                 `json:"runLoopStepId"`
	ExpectedCurrentStepPlanSHA256 string                                 `json:"expectedCurrentStepPlanSha256"`
	RequestBefore                 mission.MissionCommanderDriverRequest  `json:"requestBefore"`
	RequestBeforeSHA256           string                                 `json:"requestBeforeSha256"`
	CurrentStepReceipt            StepReceipt                            `json:"currentStepReceipt"`
	CurrentStepReceiptSHA256      string                                 `json:"currentStepReceiptSha256"`
	RequestAfter                  *mission.MissionCommanderDriverRequest `json:"requestAfter,omitempty"`
	RequestAfterSHA256            string                                 `json:"requestAfterSha256,omitempty"`
}

type Stop struct {
	Code  string `json:"code"`
	Phase string `json:"phase"`
}

type Payload struct {
	SchemaVersion                       int                                    `json:"schemaVersion"`
	Kind                                string                                 `json:"kind"`
	Sequence                            uint64                                 `json:"sequence"`
	PreviousArtifactSHA256              string                                 `json:"previousArtifactSha256,omitempty"`
	CaseIdentitySHA256                  string                                 `json:"caseIdentitySha256"`
	Pack                                string                                 `json:"pack"`
	Actor                               string                                 `json:"actor"`
	RoutePolicy                         string                                 `json:"routePolicy"`
	InitialCurrentDriverRequest         mission.MissionCommanderDriverRequest  `json:"initialCurrentDriverRequest"`
	InitialCurrentDriverRequestSHA256   string                                 `json:"initialCurrentDriverRequestSha256"`
	ExpectedCurrentLoopPlanSHA256       string                                 `json:"expectedCurrentLoopPlanSha256"`
	SegmentMaxSteps                     int                                    `json:"segmentMaxSteps"`
	AppliedStepsInSegment               int                                    `json:"appliedStepsInSegment"`
	RemainingMaxSteps                   int                                    `json:"remainingMaxSteps"`
	SegmentRoute                        string                                 `json:"segmentRoute"`
	SegmentLane                         string                                 `json:"segmentLane"`
	Stop                                Stop                                   `json:"stop"`
	StepReceipts                        []StepReceiptBinding                   `json:"stepReceipts"`
	StatusAvailable                     bool                                   `json:"statusAvailable"`
	RefreshedCurrentDriverRequest       *mission.MissionCommanderDriverRequest `json:"refreshedCurrentDriverRequest,omitempty"`
	RefreshedCurrentDriverRequestSHA256 string                                 `json:"refreshedCurrentDriverRequestSha256,omitempty"`
	Continuation                        *Continuation                          `json:"continuation,omitempty"`
	CumulativeReceipts                  bool                                   `json:"cumulativeReceipts"`
	NoAutoApply                         bool                                   `json:"noAutoApply"`
	NoAuthority                         bool                                   `json:"noAuthorityOrConfirmed"`
	Boundary                            []string                               `json:"boundary"`
}

type envelope struct {
	SchemaVersion int             `json:"schemaVersion"`
	Kind          string          `json:"kind"`
	Sequence      uint64          `json:"sequence"`
	RecordedAt    string          `json:"recordedAt"`
	PayloadSHA256 string          `json:"payloadSha256"`
	PayloadBytes  int             `json:"payloadBytes"`
	Payload       json.RawMessage `json:"payload"`
}

type Inspection struct {
	State             string        `json:"state"`
	Ready             bool          `json:"ready"`
	Sequence          uint64        `json:"sequence,omitempty"`
	ArtifactPath      string        `json:"artifactPath,omitempty"`
	ArtifactSHA256    string        `json:"artifactSha256,omitempty"`
	ArtifactBytes     int           `json:"artifactBytes,omitempty"`
	PayloadSHA256     string        `json:"payloadSha256,omitempty"`
	RecordedAt        string        `json:"recordedAt,omitempty"`
	StopCode          string        `json:"stopCode,omitempty"`
	StopPhase         string        `json:"stopPhase,omitempty"`
	SegmentMaxSteps   int           `json:"segmentMaxSteps,omitempty"`
	AppliedSteps      int           `json:"appliedStepsInSegment,omitempty"`
	RemainingMaxSteps int           `json:"remainingMaxSteps,omitempty"`
	SegmentRoute      string        `json:"segmentRoute,omitempty"`
	SegmentLane       string        `json:"segmentLane,omitempty"`
	ExpectedRoute     string        `json:"expectedRoute,omitempty"`
	ExpectedLane      string        `json:"expectedLane,omitempty"`
	Continuation      *Continuation `json:"continuation,omitempty"`
	Warnings          []string      `json:"warnings,omitempty"`
	Boundary          []string      `json:"boundary"`
}

func RequestSHA256(request mission.MissionCommanderDriverRequest) (string, error) {
	data, err := json.Marshal(request)
	if err != nil {
		return "", err
	}
	return sha256Hex(data), nil
}

func ValueSHA256(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return sha256Hex(data), nil
}

func Write(repoRoot, caseRoot, pack string, payload Payload) (inspection Inspection, resultErr error) {
	lease, err := acquireProject(caseRoot)
	if err != nil {
		return Inspection{}, fmt.Errorf("acquire current-loop checkpoint publication lease: %w", err)
	}
	defer func() {
		if unlockErr := lease.Unlock(); unlockErr != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("release current-loop checkpoint publication lease: %w", unlockErr))
		}
	}()
	identity, err := caseIdentity(repoRoot, caseRoot, pack)
	if err != nil {
		return Inspection{}, err
	}
	root, err := openArtifactRoot(caseRoot, true)
	if err != nil {
		return Inspection{}, err
	}
	defer root.Close()
	if err := removePublicationTemps(root); err != nil {
		return Inspection{}, fmt.Errorf("recover current-loop checkpoint publication temps: %w", err)
	}
	records, err := readArtifactChain(root, caseRoot)
	if err != nil {
		return Inspection{}, fmt.Errorf("validate current-loop checkpoint chain before publication: %w", err)
	}
	payload.SchemaVersion = 1
	payload.Kind = "current-loop-segment-checkpoint"
	payload.Sequence = uint64(len(records) + 1)
	if len(records) > 0 {
		payload.PreviousArtifactSHA256 = records[len(records)-1].artifactSHA256
	} else {
		payload.PreviousArtifactSHA256 = ""
	}
	payload.CaseIdentitySHA256 = identity
	payload.Pack = strings.TrimSpace(pack)
	payload.NoAutoApply = true
	payload.NoAuthority = true
	payload.CumulativeReceipts = false
	if err := validatePayloadForCase(payload, caseRoot); err != nil {
		return Inspection{}, err
	}
	payloadData, err := json.Marshal(payload)
	if err != nil {
		return Inspection{}, err
	}
	payloadSHA256 := sha256Hex(payloadData)
	recordedAt := nowUTC().UTC().Format(recordedAtLayout)
	artifact := envelope{
		SchemaVersion: 1,
		Kind:          "current-loop-segment-checkpoint-envelope",
		Sequence:      payload.Sequence,
		RecordedAt:    recordedAt,
		PayloadSHA256: payloadSHA256,
		PayloadBytes:  len(payloadData),
		Payload:       payloadData,
	}
	artifactData, err := canonicalEnvelope(artifact)
	if err != nil {
		return Inspection{}, err
	}
	name := sequenceText(payload.Sequence) + ".json"
	temp, err := publicationTempName(payload.Sequence)
	if err != nil {
		return Inspection{}, err
	}
	file, err := root.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return Inspection{}, fmt.Errorf("create current-loop checkpoint temp artifact: %w", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = root.Remove(temp)
		}
	}()
	if _, err := file.Write(artifactData); err != nil {
		_ = file.Close()
		return Inspection{}, err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return Inspection{}, err
	}
	if err := file.Close(); err != nil {
		return Inspection{}, err
	}
	if err := root.Link(temp, name); err != nil {
		return Inspection{}, fmt.Errorf("publish current-loop checkpoint artifact: %w", err)
	}
	if err := root.Remove(temp); err != nil {
		return Inspection{}, err
	}
	cleanup = false
	published, err := readStableRegular(root, name, maxArtifactBytes)
	if err != nil {
		return Inspection{}, err
	}
	if !bytes.Equal(published, artifactData) {
		return Inspection{}, fmt.Errorf("published current-loop checkpoint bytes changed: %s", name)
	}
	if directory, err := root.Open("."); err == nil {
		_ = directory.Sync()
		_ = directory.Close()
	}
	inspection = inspectRoot(root, identity, caseRoot, pack, payload.RefreshedCurrentDriverRequest)
	publishedRel := filepath.ToSlash(filepath.Join(artifactRelRoot, name))
	if inspection.State == "invalid" || inspection.State == "missing" || inspection.ArtifactPath != publishedRel || !strings.EqualFold(inspection.PayloadSHA256, payloadSHA256) {
		return inspection, fmt.Errorf("published current-loop checkpoint is not the exact latest strict artifact: %s", strings.Join(inspection.Warnings, "; "))
	}
	return inspection, nil
}

func InspectAttached(caseRoot string, current *mission.MissionCommanderDriverRequest) Inspection {
	inst, err := instance.Read(caseRoot)
	if err != nil {
		return FailedInspection("current-loop checkpoint attached case metadata cannot be read: " + err.Error())
	}
	return Inspect(inst.TemplateRoot, caseRoot, inst.TemplatePack, current)
}

func FailedInspection(warning string) Inspection {
	base := Inspection{State: "write-failed", Boundary: inspectionBoundary()}
	base.Warnings = []string{strings.TrimSpace(warning)}
	return base
}

func Inspect(repoRoot, caseRoot, pack string, current *mission.MissionCommanderDriverRequest) Inspection {
	base := Inspection{State: "missing", Boundary: inspectionBoundary()}
	identity, err := caseIdentity(repoRoot, caseRoot, pack)
	if err != nil {
		return invalidInspection(base, "current-loop checkpoint case identity cannot be validated: "+err.Error())
	}
	root, err := openArtifactRoot(caseRoot, false)
	if os.IsNotExist(err) {
		return base
	}
	if err != nil {
		return invalidInspection(base, "current-loop checkpoint namespace is invalid: "+err.Error())
	}
	defer root.Close()
	return inspectRoot(root, identity, caseRoot, pack, current)
}

func inspectRoot(root *os.Root, identity, caseRoot, pack string, current *mission.MissionCommanderDriverRequest) Inspection {
	base := Inspection{State: "missing", Boundary: inspectionBoundary()}
	records, err := readArtifactChain(root, caseRoot)
	if err != nil {
		if artifactErr, ok := errors.AsType[*artifactChainError](err); ok {
			base.ArtifactPath = filepath.ToSlash(filepath.Join(artifactRelRoot, artifactErr.name))
		}
		return invalidInspection(base, "current-loop checkpoint chain is invalid: "+err.Error())
	}
	if len(records) == 0 {
		return base
	}
	latest := records[len(records)-1]
	name := latest.name
	artifact := latest.artifact
	payload := latest.payload
	base.Sequence = payload.Sequence
	base.ArtifactPath = filepath.ToSlash(filepath.Join(artifactRelRoot, name))
	base.ArtifactSHA256 = latest.artifactSHA256
	base.ArtifactBytes = len(latest.data)
	base.PayloadSHA256 = artifact.PayloadSHA256
	base.RecordedAt = artifact.RecordedAt
	base.StopCode = payload.Stop.Code
	base.StopPhase = payload.Stop.Phase
	base.SegmentMaxSteps = payload.SegmentMaxSteps
	base.AppliedSteps = payload.AppliedStepsInSegment
	base.RemainingMaxSteps = payload.RemainingMaxSteps
	base.SegmentRoute = payload.SegmentRoute
	base.SegmentLane = payload.SegmentLane
	if payload.Continuation != nil {
		base.ExpectedRoute = payload.Continuation.ExpectedRoute
		base.ExpectedLane = payload.Continuation.ExpectedLane
	}
	if !strings.EqualFold(payload.Pack, strings.TrimSpace(pack)) || !strings.EqualFold(payload.CaseIdentitySHA256, identity) {
		return invalidInspection(base, "latest current-loop checkpoint case or pack identity does not match the attached case")
	}
	if !payload.StatusAvailable {
		base.State = "status-unavailable"
		base.Warnings = []string{"latest current-loop segment applied durable work but did not finish a trustworthy status refresh; rerun status and start a fresh reviewed loop instead of recovering its budget"}
		return base
	}
	if payload.RefreshedCurrentDriverRequest == nil {
		if current != nil {
			base.State = "stale-current-driver-request"
			base.Warnings = []string{"latest current-loop checkpoint is terminal history, but current durable status now has a driver request; do not recover or treat the checkpoint as current terminal state"}
			return base
		}
		base.State = "terminal"
		base.Warnings = []string{"latest current-loop checkpoint is valid terminal history and has no current driver request or recoverable continuation"}
		return base
	}
	if current == nil {
		base.State = "stale-current-driver-request"
		base.Warnings = []string{"latest current-loop checkpoint expects a refreshed current driver request, but current status has none"}
		return base
	}
	currentSHA256, err := RequestSHA256(*current)
	if err != nil {
		return invalidInspection(base, "current driver request cannot be hashed: "+err.Error())
	}
	if !strings.EqualFold(currentSHA256, payload.RefreshedCurrentDriverRequestSHA256) {
		base.State = "stale-current-driver-request"
		base.Warnings = []string{"latest current-loop checkpoint is valid history but its refreshed current driver request no longer matches current durable status; do not recover its remaining budget"}
		return base
	}
	if payload.Continuation == nil {
		base.State = "terminal"
		base.Warnings = []string{"latest current-loop checkpoint is valid history, but its stop does not permit recovery of previous segment budget"}
		return base
	}
	base.State = "ready"
	base.Ready = true
	base.Continuation = payload.Continuation
	return base
}

type artifactRecord struct {
	name           string
	data           []byte
	artifact       envelope
	payload        Payload
	artifactSHA256 string
}

func removePublicationTemps(root *os.Root) error {
	directory, err := root.Open(".")
	if err != nil {
		return err
	}
	entries, err := directory.ReadDir(-1)
	_ = directory.Close()
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if !tempNamePattern.MatchString(name) {
			continue
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("publication temp must be a regular file: %s", name)
		}
		if err := root.Remove(name); err != nil {
			return fmt.Errorf("remove stale publication temp %s: %w", name, err)
		}
	}
	return nil
}

type artifactChainError struct {
	name string
	err  error
}

func (e *artifactChainError) Error() string {
	return e.err.Error()
}

func (e *artifactChainError) Unwrap() error {
	return e.err
}

func readArtifactChain(root *os.Root, caseRoot string) ([]artifactRecord, error) {
	directory, err := root.Open(".")
	if err != nil {
		return nil, fmt.Errorf("open namespace: %w", err)
	}
	entries, err := directory.ReadDir(-1)
	_ = directory.Close()
	if err != nil {
		return nil, fmt.Errorf("list namespace: %w", err)
	}
	names := make(map[uint64]string, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if tempNamePattern.MatchString(name) && entry.Type().IsRegular() {
			return nil, fmt.Errorf("namespace contains an incomplete publication temp: %s", name)
		}
		match := artifactNamePattern.FindStringSubmatch(name)
		if len(match) != 2 || !entry.Type().IsRegular() {
			return nil, fmt.Errorf("namespace contains an unexpected entry: %s", name)
		}
		sequence, err := strconv.ParseUint(match[1], 10, 64)
		if err != nil || sequence == 0 || sequenceText(sequence) != match[1] {
			return nil, fmt.Errorf("artifact sequence is invalid: %s", name)
		}
		if _, exists := names[sequence]; exists {
			return nil, fmt.Errorf("artifact sequence fork detected: %s", match[1])
		}
		names[sequence] = name
	}
	records := make([]artifactRecord, 0, len(names))
	previousSHA256 := ""
	for sequence := uint64(1); sequence <= uint64(len(names)); sequence++ {
		name, ok := names[sequence]
		if !ok {
			return nil, fmt.Errorf("artifact sequence gap at %s", sequenceText(sequence))
		}
		data, err := readStableRegular(root, name, maxArtifactBytes)
		if err != nil {
			return nil, fmt.Errorf("read artifact %s safely: %w", name, err)
		}
		artifact, payload, err := decodeArtifact(data, name)
		if err != nil {
			return nil, &artifactChainError{name: name, err: err}
		}
		if payload.Sequence != sequence || artifact.Sequence != sequence {
			return nil, fmt.Errorf("artifact sequence does not match filename: %s", name)
		}
		if err := validatePayloadForCase(payload, caseRoot); err != nil {
			return nil, &artifactChainError{name: name, err: fmt.Errorf("current-loop checkpoint payload lineage is invalid: %w", err)}
		}
		if !strings.EqualFold(payload.PreviousArtifactSHA256, previousSHA256) {
			return nil, fmt.Errorf("artifact previous hash chain is broken: %s", name)
		}
		artifactSHA256 := sha256Hex(data)
		records = append(records, artifactRecord{name: name, data: data, artifact: artifact, payload: payload, artifactSHA256: artifactSHA256})
		previousSHA256 = artifactSHA256
	}
	return records, nil
}

func decodeArtifact(data []byte, name string) (envelope, Payload, error) {
	var artifact envelope
	if err := decodeStrict(data, &artifact); err != nil {
		return envelope{}, Payload{}, fmt.Errorf("current-loop checkpoint envelope is invalid: %w", err)
	}
	match := artifactNamePattern.FindStringSubmatch(name)
	if len(match) != 2 {
		return envelope{}, Payload{}, fmt.Errorf("current-loop checkpoint filename is invalid")
	}
	if artifact.SchemaVersion != 1 || artifact.Kind != "current-loop-segment-checkpoint-envelope" {
		return envelope{}, Payload{}, fmt.Errorf("current-loop checkpoint envelope contract is unsupported")
	}
	if _, err := time.Parse(recordedAtLayout, artifact.RecordedAt); err != nil {
		return envelope{}, Payload{}, fmt.Errorf("current-loop checkpoint recordedAt is invalid")
	}
	var payload Payload
	if err := decodeStrict(artifact.Payload, &payload); err != nil {
		return envelope{}, Payload{}, fmt.Errorf("current-loop checkpoint payload is invalid: %w", err)
	}
	canonicalPayload, err := json.Marshal(payload)
	if err != nil {
		return envelope{}, Payload{}, err
	}
	if !bytes.Equal(artifact.Payload, canonicalPayload) || artifact.PayloadBytes != len(canonicalPayload) || !strings.EqualFold(artifact.PayloadSHA256, sha256Hex(canonicalPayload)) {
		return envelope{}, Payload{}, fmt.Errorf("current-loop checkpoint payload bytes or sha256 do not match canonical payload")
	}
	canonicalArtifact, err := canonicalEnvelope(artifact)
	if err != nil {
		return envelope{}, Payload{}, err
	}
	if !bytes.Equal(data, canonicalArtifact) {
		return envelope{}, Payload{}, fmt.Errorf("current-loop checkpoint exact bytes do not match canonical envelope")
	}
	if err := validatePayload(payload); err != nil {
		return envelope{}, Payload{}, fmt.Errorf("current-loop checkpoint payload contract is invalid: %w", err)
	}
	return artifact, payload, nil
}

func canonicalEnvelope(artifact envelope) ([]byte, error) {
	data, err := json.Marshal(artifact)
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func sequenceText(sequence uint64) string {
	return fmt.Sprintf("%0*d", sequenceWidth, sequence)
}

func publicationTempName(sequence uint64) (string, error) {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", fmt.Errorf("generate current-loop checkpoint temp nonce: %w", err)
	}
	return "." + sequenceText(sequence) + "." + hex.EncodeToString(nonce[:]) + ".json.tmp", nil
}

func validatePayloadForCase(payload Payload, caseRoot string) error {
	if err := validatePayload(payload); err != nil {
		return err
	}
	identity := struct {
		SchemaVersion                 int                                   `json:"schemaVersion"`
		CaseRoot                      string                                `json:"caseRoot"`
		Pack                          string                                `json:"pack"`
		RoutePolicy                   string                                `json:"routePolicy"`
		MaxSteps                      int                                   `json:"maxSteps"`
		Actor                         string                                `json:"actor"`
		InitialRoute                  string                                `json:"initialRoute"`
		InitialLane                   string                                `json:"initialLane"`
		InitialCurrentDriverRequest   mission.MissionCommanderDriverRequest `json:"initialCurrentDriverRequest"`
		ExpectedCurrentStepPlanSHA256 string                                `json:"expectedCurrentStepPlanSha256"`
	}{
		SchemaVersion:                 1,
		CaseRoot:                      caseRoot,
		Pack:                          payload.Pack,
		RoutePolicy:                   payload.RoutePolicy,
		MaxSteps:                      payload.SegmentMaxSteps,
		Actor:                         payload.Actor,
		InitialRoute:                  payload.SegmentRoute,
		InitialLane:                   payload.SegmentLane,
		InitialCurrentDriverRequest:   payload.InitialCurrentDriverRequest,
		ExpectedCurrentStepPlanSHA256: payload.StepReceipts[0].ExpectedCurrentStepPlanSHA256,
	}
	encoded, err := json.Marshal(identity)
	if err != nil {
		return err
	}
	if !strings.EqualFold(sha256Hex(encoded), payload.ExpectedCurrentLoopPlanSHA256) {
		return fmt.Errorf("current-loop plan hash does not match segment identity")
	}
	return nil
}

func validatePayload(payload Payload) error {
	if payload.SchemaVersion != 1 || payload.Kind != "current-loop-segment-checkpoint" {
		return fmt.Errorf("unsupported schemaVersion or kind")
	}
	if payload.Sequence == 0 || payload.Sequence == 1 && payload.PreviousArtifactSHA256 != "" || payload.Sequence > 1 && !isSHA256(payload.PreviousArtifactSHA256) {
		return fmt.Errorf("segment sequence or previous artifact hash is invalid")
	}
	if !isSHA256(payload.CaseIdentitySHA256) || strings.TrimSpace(payload.Pack) == "" || strings.TrimSpace(payload.RoutePolicy) == "" || !isSHA256(payload.InitialCurrentDriverRequestSHA256) || !isSHA256(payload.ExpectedCurrentLoopPlanSHA256) {
		return fmt.Errorf("case, pack, route policy, initial request, and loop plan identity must be present")
	}
	initialSHA256, err := RequestSHA256(payload.InitialCurrentDriverRequest)
	if err != nil || !strings.EqualFold(initialSHA256, payload.InitialCurrentDriverRequestSHA256) {
		return fmt.Errorf("initial current driver request hash does not match")
	}
	if payload.SegmentMaxSteps < 1 || payload.SegmentMaxSteps > 20 || payload.AppliedStepsInSegment < 1 || payload.AppliedStepsInSegment > payload.SegmentMaxSteps || payload.RemainingMaxSteps != payload.SegmentMaxSteps-payload.AppliedStepsInSegment {
		return fmt.Errorf("segment budget is inconsistent")
	}
	if len(payload.StepReceipts) != payload.AppliedStepsInSegment || strings.TrimSpace(payload.SegmentRoute) == "" || strings.TrimSpace(payload.SegmentLane) == "" || strings.TrimSpace(payload.Stop.Code) == "" || strings.TrimSpace(payload.Stop.Phase) == "" {
		return fmt.Errorf("segment identity, stop, or receipt count is inconsistent")
	}
	for index, receipt := range payload.StepReceipts {
		if receipt.Step != index+1 || receipt.Route != payload.SegmentRoute || receipt.Lane != payload.SegmentLane || strings.TrimSpace(receipt.RunLoopStepID) == "" || !isSHA256(receipt.ExpectedCurrentStepPlanSHA256) || !isSHA256(receipt.RequestBeforeSHA256) || !isSHA256(receipt.CurrentStepReceiptSHA256) || receipt.RequestAfterSHA256 != "" && !isSHA256(receipt.RequestAfterSHA256) {
			return fmt.Errorf("step receipt binding %d is invalid", index+1)
		}
		requestBeforeSHA256, err := RequestSHA256(receipt.RequestBefore)
		if err != nil || !strings.EqualFold(requestBeforeSHA256, receipt.RequestBeforeSHA256) {
			return fmt.Errorf("step receipt binding %d request-before hash does not match", index+1)
		}
		currentStepReceiptSHA256, err := ValueSHA256(receipt.CurrentStepReceipt)
		if err != nil || !strings.EqualFold(currentStepReceiptSHA256, receipt.CurrentStepReceiptSHA256) {
			return fmt.Errorf("step receipt binding %d current-step receipt hash does not match", index+1)
		}
		if receipt.CurrentStepReceipt.Route != receipt.Route || strings.TrimSpace(receipt.CurrentStepReceipt.NestedCommand) == "" {
			return fmt.Errorf("step receipt binding %d current-step receipt is inconsistent", index+1)
		}
		switch receipt.CurrentStepReceipt.State {
		case "refreshed":
			if receipt.CurrentStepReceipt.Outcome != "current-step-applied" {
				return fmt.Errorf("step receipt binding %d refreshed outcome is invalid", index+1)
			}
		case "refresh-failed":
			if receipt.CurrentStepReceipt.Outcome != "current-step-applied-status-refresh-failed" || index != len(payload.StepReceipts)-1 || receipt.RequestAfter != nil || receipt.RequestAfterSHA256 != "" || receipt.CurrentStepReceipt.RefreshedCurrentDriverRequest != nil || payload.StatusAvailable || payload.Continuation != nil {
				return fmt.Errorf("step receipt binding %d refresh-failed state is inconsistent", index+1)
			}
		default:
			return fmt.Errorf("step receipt binding %d current-step receipt state is invalid", index+1)
		}
		if receipt.RequestAfter == nil {
			if receipt.RequestAfterSHA256 != "" || receipt.CurrentStepReceipt.RefreshedCurrentDriverRequest != nil || index != len(payload.StepReceipts)-1 || payload.StatusAvailable && payload.RefreshedCurrentDriverRequest != nil {
				return fmt.Errorf("step receipt binding %d missing request-after is inconsistent", index+1)
			}
		} else {
			requestAfterSHA256, err := RequestSHA256(*receipt.RequestAfter)
			if err != nil || !strings.EqualFold(requestAfterSHA256, receipt.RequestAfterSHA256) || receipt.CurrentStepReceipt.RefreshedCurrentDriverRequest == nil || !requestsEqual(*receipt.CurrentStepReceipt.RefreshedCurrentDriverRequest, *receipt.RequestAfter) {
				return fmt.Errorf("step receipt binding %d request-after or refreshed receipt does not match", index+1)
			}
		}
		if index == 0 && !requestsEqual(receipt.RequestBefore, payload.InitialCurrentDriverRequest) {
			return fmt.Errorf("first step request does not match segment initial request")
		}
		if index > 0 {
			previous := payload.StepReceipts[index-1]
			if previous.RequestAfter == nil || !requestsEqual(*previous.RequestAfter, receipt.RequestBefore) {
				return fmt.Errorf("step receipt chain is broken before step %d", index+1)
			}
		}
	}
	if payload.CumulativeReceipts || !payload.NoAutoApply || !payload.NoAuthority {
		return fmt.Errorf("checkpoint boundary flags are invalid")
	}
	if payload.StatusAvailable {
		if payload.RefreshedCurrentDriverRequest == nil {
			if payload.RefreshedCurrentDriverRequestSHA256 != "" || payload.Continuation != nil {
				return fmt.Errorf("terminal available status cannot expose a request hash or continuation")
			}
		} else {
			if !isSHA256(payload.RefreshedCurrentDriverRequestSHA256) {
				return fmt.Errorf("available status current driver request hash is invalid")
			}
			requestSHA256, err := RequestSHA256(*payload.RefreshedCurrentDriverRequest)
			if err != nil || !strings.EqualFold(requestSHA256, payload.RefreshedCurrentDriverRequestSHA256) {
				return fmt.Errorf("refreshed current driver request hash does not match")
			}
			last := payload.StepReceipts[len(payload.StepReceipts)-1]
			if last.RequestAfter == nil || !requestsEqual(*last.RequestAfter, *payload.RefreshedCurrentDriverRequest) {
				return fmt.Errorf("last step request-after does not match refreshed status request")
			}
		}
	} else {
		if payload.RefreshedCurrentDriverRequest != nil || payload.RefreshedCurrentDriverRequestSHA256 != "" || payload.Continuation != nil {
			return fmt.Errorf("unavailable status cannot expose a current request or continuation")
		}
		last := payload.StepReceipts[len(payload.StepReceipts)-1]
		if last.CurrentStepReceipt.State != "refresh-failed" {
			return fmt.Errorf("unavailable status requires a final refresh-failed receipt")
		}
	}
	if payload.Continuation != nil {
		continuation := payload.Continuation
		if continuation.Kind != "current-loop-campaign-continuation" || continuation.StopCode != payload.Stop.Code || continuation.SegmentMaxSteps != payload.SegmentMaxSteps || continuation.AppliedStepsInSegment != payload.AppliedStepsInSegment || continuation.RemainingMaxSteps != payload.RemainingMaxSteps || continuation.SegmentRoute != payload.SegmentRoute || continuation.SegmentLane != payload.SegmentLane || continuation.RemainingMaxSteps < 1 || !continuation.FreshPreviewRequired || continuation.CumulativeReceipts || strings.TrimSpace(continuation.ExpectedRoute) == "" || strings.TrimSpace(continuation.ExpectedLane) == "" || strings.TrimSpace(continuation.WhatIfCommand) == "" {
			return fmt.Errorf("campaign continuation does not match its segment checkpoint")
		}
		switch continuation.StopCode {
		case "route-policy", "human-intervention", "external-reviewer-handoff":
		default:
			return fmt.Errorf("campaign continuation stop code is unsupported")
		}
		if continuation.StopCode == "external-reviewer-handoff" && (continuation.ObservationContract == nil || len(continuation.ObservationContract.Alternatives) == 0) {
			return fmt.Errorf("external reviewer continuation requires an observation contract")
		}
		if continuation.StopCode != "external-reviewer-handoff" && continuation.ObservationContract != nil {
			return fmt.Errorf("non-reviewer continuation cannot include an observation contract")
		}
	}
	return nil
}

func caseIdentity(repoRoot, caseRoot, pack string) (string, error) {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return "", err
	}
	data, err := readStablePath(inst.InstancePath, maxArtifactBytes)
	if err != nil {
		return "", fmt.Errorf("read attached case identity: %w", err)
	}
	canonical, err := filepath.EvalSymlinks(inst.CaseRoot)
	if err != nil {
		return "", err
	}
	canonical, err = filepath.Abs(canonical)
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" {
		canonical = strings.ToLower(canonical)
	}
	return sha256Hex(append(append([]byte(canonical), 0), data...)), nil
}

func openArtifactRoot(caseRoot string, create bool) (*os.Root, error) {
	caseRoot, err := filepath.Abs(caseRoot)
	if err != nil {
		return nil, err
	}
	caseInfo, err := os.Lstat(caseRoot)
	if err != nil {
		return nil, err
	}
	if caseInfo.Mode()&os.ModeSymlink != 0 || !caseInfo.IsDir() {
		return nil, fmt.Errorf("case root must be a non-symlink directory")
	}
	current, err := os.OpenRoot(caseRoot)
	if err != nil {
		return nil, err
	}
	components := []string{".rekit", "runs", "current-loop-segments"}
	for index, component := range components {
		before, statErr := current.Lstat(component)
		if os.IsNotExist(statErr) && create && index > 0 {
			if err := current.Mkdir(component, 0o700); err != nil && !os.IsExist(err) {
				_ = current.Close()
				return nil, err
			}
			before, statErr = current.Lstat(component)
		}
		if statErr != nil {
			_ = current.Close()
			return nil, statErr
		}
		if before.Mode()&os.ModeSymlink != 0 || !before.IsDir() {
			_ = current.Close()
			return nil, fmt.Errorf("current-loop checkpoint namespace component must be a non-symlink directory: %s", strings.Join(components[:index+1], "/"))
		}
		next, err := current.OpenRoot(component)
		if err != nil {
			_ = current.Close()
			return nil, err
		}
		opened, openErr := next.Stat(".")
		after, afterErr := current.Lstat(component)
		if openErr != nil || afterErr != nil || after.Mode()&os.ModeSymlink != 0 || !after.IsDir() || !os.SameFile(before, opened) || !os.SameFile(opened, after) {
			_ = next.Close()
			_ = current.Close()
			return nil, fmt.Errorf("current-loop checkpoint namespace changed while opening: %s", strings.Join(components[:index+1], "/"))
		}
		if err := current.Close(); err != nil {
			_ = next.Close()
			return nil, err
		}
		current = next
	}
	return current, nil
}

func readStablePath(path string, limit int64) ([]byte, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() || before.Size() > limit {
		return nil, fmt.Errorf("path must be a bounded regular non-symlink file: %s", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return nil, fmt.Errorf("file changed while opening: %s", path)
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil || int64(len(data)) > limit {
		return nil, fmt.Errorf("read bounded file %s: %w", path, err)
	}
	after, err := os.Lstat(path)
	if err != nil || after.Mode()&os.ModeSymlink != 0 || !os.SameFile(opened, after) {
		return nil, fmt.Errorf("file path changed after open: %s", path)
	}
	return data, nil
}

func readStableRegular(root *os.Root, name string, limit int64) ([]byte, error) {
	before, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() || before.Size() > limit {
		return nil, fmt.Errorf("artifact must be a bounded regular non-symlink file: %s", name)
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return nil, fmt.Errorf("artifact changed while opening: %s", name)
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil || int64(len(data)) > limit {
		return nil, fmt.Errorf("read bounded artifact %s: %w", name, err)
	}
	after, err := root.Lstat(name)
	if err != nil || after.Mode()&os.ModeSymlink != 0 || !os.SameFile(opened, after) {
		return nil, fmt.Errorf("artifact path changed after open: %s", name)
	}
	return data, nil
}

func decodeStrict(data []byte, out any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("trailing JSON data")
	}
	return nil
}

func inspectionBoundary() []string {
	return []string{
		"only state=ready exposes a campaign continuation; stale, invalid, missing, or status-unavailable checkpoints never recover a previous segment budget",
		"the checkpoint is append-only provenance for one completed bounded segment; it is not authority, confirmed state, an authorization token, or an automatic Apply request",
		"a fresh segment must rebuild current status and all current-step and nested hashes; previous segment hashes and receipts never cross the boundary",
	}
}

func invalidInspection(base Inspection, warning string) Inspection {
	base.State = "invalid"
	base.Ready = false
	base.Continuation = nil
	base.Warnings = []string{warning}
	return base
}

func requestsEqual(left, right mission.MissionCommanderDriverRequest) bool {
	leftSHA256, leftErr := RequestSHA256(left)
	rightSHA256, rightErr := RequestSHA256(right)
	return leftErr == nil && rightErr == nil && strings.EqualFold(leftSHA256, rightSHA256)
}

func isSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil && value == strings.ToLower(value)
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
