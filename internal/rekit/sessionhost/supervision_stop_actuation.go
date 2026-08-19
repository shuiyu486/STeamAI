package sessionhost

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/lanemutation"
	"github.com/shuiyu486/re-context-kits/internal/rekit/laneowner"
)

const (
	supervisionStopActuationRequestKind     = "claude-host-run-stop-actuation-request"
	supervisionStopActuationObservationKind = "claude-host-run-stop-actuation-observation"
	supervisionStopActuationPollInterval    = 50 * time.Millisecond
)

type supervisionStopActuationContext struct {
	paths      supervisionPaths
	spec       supervisionSpec
	specSHA256 string
}

type supervisionStopActuationRequest struct {
	SchemaVersion          int                        `json:"schemaVersion"`
	Kind                   string                     `json:"kind"`
	RunID                  string                     `json:"runId"`
	SpecSHA256             string                     `json:"specSha256"`
	SessionID              string                     `json:"sessionId"`
	LaunchControl          claudeLaunchControlBinding `json:"launchControl"`
	StopReceiptPath        string                     `json:"stopReceiptPath"`
	StopReceiptSHA256      string                     `json:"stopReceiptSha256"`
	StopReceipt            executioncontrol.Receipt   `json:"stopReceipt"`
	StopCommittedAt        string                     `json:"stopCommittedAt"`
	OwnedContainmentOnly   bool                       `json:"ownedContainmentOnly"`
	NoRemoteSessionControl bool                       `json:"noRemoteSessionControl"`
	NoAuthority            bool                       `json:"noAuthority"`
	NoConfirmed            bool                       `json:"noConfirmed"`
	NoHeavyTool            bool                       `json:"noHeavyTool"`
}

type supervisionStopActuationObservation struct {
	SchemaVersion             int    `json:"schemaVersion"`
	Kind                      string `json:"kind"`
	RunID                     string `json:"runId"`
	SpecSHA256                string `json:"specSha256"`
	SessionID                 string `json:"sessionId"`
	Lane                      string `json:"lane"`
	StopControlGeneration     int    `json:"stopControlGeneration"`
	StopControlReceiptSHA256  string `json:"stopControlReceiptSha256"`
	RequestPath               string `json:"requestPath"`
	RequestSHA256             string `json:"requestSha256"`
	RequestPublished          bool   `json:"requestPublished"`
	Outcome                   string `json:"outcome"`
	ContainmentCloseAttempted bool   `json:"containmentCloseAttempted"`
	ContainmentCloseSucceeded bool   `json:"containmentCloseSucceeded"`
	NoProcessTerminationClaim bool   `json:"noProcessTerminationClaim"`
	NoRemoteSessionControl    bool   `json:"noRemoteSessionControl"`
	NoAuthority               bool   `json:"noAuthority"`
	NoConfirmed               bool   `json:"noConfirmed"`
	NoHeavyTool               bool   `json:"noHeavyTool"`
	Error                     string `json:"error,omitempty"`
	ObservedAt                string `json:"observedAt"`
}

type supervisionStopActuationResult struct {
	RequestPath       string
	ObservationPath   string
	ObservationSHA256 string
	Err               error
}

func watchSupervisionStopActuation(
	ctx context.Context,
	scope supervisionStopActuationContext,
	closeOwnedContainment func() error,
) supervisionStopActuationResult {
	if ctx == nil {
		return supervisionStopActuationResult{Err: fmt.Errorf("Claude stop actuation watcher context is missing")}
	}
	if err := validateSupervisionStopActuationContext(scope); err != nil {
		return supervisionStopActuationResult{Err: err}
	}
	for {
		request, ready, err := inspectSupervisionStopActuationRequest(scope)
		if err != nil {
			return supervisionStopActuationResult{Err: err}
		}
		if ready {
			if err := validateSupervisionStopActuationContext(scope); err != nil {
				return supervisionStopActuationResult{Err: err}
			}
			return applySupervisionStopActuation(scope, request, closeOwnedContainment)
		}
		timer := time.NewTimer(supervisionStopActuationPollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			request, ready, err := inspectSupervisionStopActuationRequest(scope)
			if err != nil {
				return supervisionStopActuationResult{Err: err}
			}
			if !ready {
				return supervisionStopActuationResult{}
			}
			if err := validateSupervisionStopActuationContext(scope); err != nil {
				return supervisionStopActuationResult{Err: err}
			}
			return applySupervisionStopActuation(scope, request, closeOwnedContainment)
		case <-timer.C:
		}
	}
}

func inspectSupervisionStopActuationRequest(
	scope supervisionStopActuationContext,
) (request supervisionStopActuationRequest, ready bool, retErr error) {
	binding := scope.spec.LaunchControl
	if binding == nil {
		return supervisionStopActuationRequest{}, false, nil
	}
	lease, err := lanemutation.AcquireLane(scope.spec.Target, binding.Lane)
	if err != nil {
		return supervisionStopActuationRequest{}, false, err
	}
	defer func() { retErr = errors.Join(retErr, lease.Unlock()) }()
	return inspectSupervisionStopActuationRequestWithLease(scope, lease)
}

func inspectSupervisionStopActuationRequestWithLease(
	scope supervisionStopActuationContext,
	lease *lanemutation.Lease,
) (supervisionStopActuationRequest, bool, error) {
	binding := scope.spec.LaunchControl
	if binding == nil {
		return supervisionStopActuationRequest{}, false, nil
	}
	if lease == nil {
		return supervisionStopActuationRequest{}, false, fmt.Errorf("Claude stop actuation inspection requires an existing lane mutation lease")
	}
	if err := lease.ValidateLaneFor(scope.spec.Target, binding.Lane); err != nil {
		return supervisionStopActuationRequest{}, false, err
	}
	inspection, err := executioncontrol.Inspect(scope.spec.Target, binding.Lane)
	if err != nil {
		return supervisionStopActuationRequest{}, false, fmt.Errorf("inspect durable lane stop for exact Claude supervisor run: %w", err)
	}
	if inspection.Pending || inspection.State != executioncontrol.StateStopped {
		return supervisionStopActuationRequest{}, false, nil
	}
	if inspection.CurrentGeneration <= binding.ControlGeneration ||
		inspection.CurrentOwner == nil || *inspection.CurrentOwner != binding.Owner ||
		len(inspection.Receipts) == 0 {
		return supervisionStopActuationRequest{}, false, fmt.Errorf("durable lane stop does not own exact Claude supervisor run %s", scope.spec.RunID)
	}
	stop := inspection.Receipts[len(inspection.Receipts)-1]
	if stop.Action != executioncontrol.ActionStop ||
		stop.State != executioncontrol.StateStopped ||
		stop.ControlGeneration != inspection.CurrentGeneration ||
		stop.Owner != binding.Owner {
		return supervisionStopActuationRequest{}, false, fmt.Errorf("durable lane stop head is inconsistent for exact Claude supervisor run %s", scope.spec.RunID)
	}
	stopData, err := readSupervisionStopReceipt(scope.spec.Target, stop, inspection.CurrentReceiptPath)
	if err != nil {
		return supervisionStopActuationRequest{}, false, err
	}
	if !strings.EqualFold(bytesSHA256(stopData), inspection.CurrentReceiptSHA256) {
		return supervisionStopActuationRequest{}, false, fmt.Errorf("durable lane stop receipt sha256 does not match its inspected head")
	}
	owner, err := laneowner.Read(scope.spec.Target, binding.Lane)
	if err != nil {
		return supervisionStopActuationRequest{}, false, err
	}
	if owner != binding.Owner {
		return supervisionStopActuationRequest{}, false, fmt.Errorf("lane execution control stop owner changed before actuation request")
	}
	if err := lease.ValidateLaneFor(scope.spec.Target, binding.Lane); err != nil {
		return supervisionStopActuationRequest{}, false, err
	}
	return supervisionStopActuationRequest{
		SchemaVersion:          1,
		Kind:                   supervisionStopActuationRequestKind,
		RunID:                  scope.spec.RunID,
		SpecSHA256:             scope.specSHA256,
		SessionID:              scope.spec.SessionID,
		LaunchControl:          *cloneClaudeLaunchControlBinding(binding),
		StopReceiptPath:        inspection.CurrentReceiptPath,
		StopReceiptSHA256:      inspection.CurrentReceiptSHA256,
		StopReceipt:            stop,
		StopCommittedAt:        stop.CommittedAt,
		OwnedContainmentOnly:   true,
		NoRemoteSessionControl: true,
		NoAuthority:            true,
		NoConfirmed:            true,
		NoHeavyTool:            true,
	}, true, nil
}

func readSupervisionStopReceipt(
	caseRoot string,
	expected executioncontrol.Receipt,
	rel string,
) ([]byte, error) {
	data, err := rekitfs.ReadStableRegularFileAnchored(
		caseRoot,
		filepath.Join(caseRoot, filepath.FromSlash(rel)),
		"lane execution control stop receipt",
		256*1024,
	)
	if err != nil {
		return nil, err
	}
	var actual executioncontrol.Receipt
	if err := strictJSON(data, &actual); err != nil {
		return nil, fmt.Errorf("decode lane execution control stop receipt: %w", err)
	}
	if actual != expected {
		return nil, fmt.Errorf("lane execution control stop receipt changed after inspection")
	}
	return data, nil
}

func applySupervisionStopActuation(
	scope supervisionStopActuationContext,
	request supervisionStopActuationRequest,
	closeOwnedContainment func() error,
) (result supervisionStopActuationResult) {
	requestPath, observationPath := supervisionStopActuationArtifactPaths(request.StopReceipt.ControlGeneration)
	requestData, err := marshalSupervisionStopActuationJSON(request)
	if err != nil {
		return supervisionStopActuationResult{Err: err}
	}
	requestSHA := bytesSHA256(requestData)
	result = supervisionStopActuationResult{RequestPath: requestPath, ObservationPath: observationPath}

	if observation, data, ok, err := readSupervisionStopActuationObservation(scope, observationPath); err != nil {
		result.Err = err
		return result
	} else if ok {
		result.ObservationSHA256 = bytesSHA256(data)
		if observation.Outcome != "owned-containment-closed" {
			result.Err = errors.New(observation.Error)
		}
		return result
	}

	lease, leaseErr := lanemutation.AcquireLane(scope.spec.Target, request.LaunchControl.Lane)
	if leaseErr != nil {
		result.Err = leaseErr
		return result
	}
	defer func() {
		result.Err = errors.Join(result.Err, lease.Unlock())
	}()
	if err := lease.ValidateLaneFor(scope.spec.Target, request.LaunchControl.Lane); err != nil {
		result.Err = err
		return result
	}
	current, ready, err := inspectSupervisionStopActuationRequestWithLease(scope, lease)
	if err != nil {
		result.Err = err
		return result
	}
	if !ready || current != request {
		result.Err = fmt.Errorf("durable lane stop changed before exact Claude actuation request publication")
		return result
	}
	_, requestErr := rekitfs.WriteExclusiveRegularFileAnchoredWriteThrough(
		scope.paths.runRoot,
		requestPath,
		"Claude supervision stop actuation request",
		requestData,
	)
	if requestErr == nil {
		if err := lease.ValidateLaneFor(scope.spec.Target, request.LaunchControl.Lane); err != nil {
			requestErr = err
		}
	}
	observation := supervisionStopActuationObservation{
		SchemaVersion:             1,
		Kind:                      supervisionStopActuationObservationKind,
		RunID:                     scope.spec.RunID,
		SpecSHA256:                scope.specSHA256,
		SessionID:                 scope.spec.SessionID,
		Lane:                      request.LaunchControl.Lane,
		StopControlGeneration:     request.StopReceipt.ControlGeneration,
		StopControlReceiptSHA256:  request.StopReceiptSHA256,
		RequestPath:               requestPath,
		RequestSHA256:             requestSHA,
		RequestPublished:          requestErr == nil,
		NoProcessTerminationClaim: true,
		NoRemoteSessionControl:    true,
		NoAuthority:               true,
		NoConfirmed:               true,
		NoHeavyTool:               true,
		ObservedAt:                nowRFC3339Nano(),
	}
	var actuationErr error
	switch {
	case requestErr != nil:
		observation.Outcome = "request-publication-failed"
		observation.Error = truncate(oneLine(requestErr.Error()), 2048)
	case closeOwnedContainment == nil:
		actuationErr = fmt.Errorf("exact Claude supervisor run has no owned containment actuator")
		observation.Outcome = "actuation-failed"
		observation.Error = actuationErr.Error()
	default:
		observation.ContainmentCloseAttempted = true
		actuationErr = closeOwnedContainment()
		if actuationErr != nil {
			observation.Outcome = "actuation-failed"
			observation.Error = truncate(oneLine(actuationErr.Error()), 2048)
		} else {
			observation.Outcome = "owned-containment-closed"
			observation.ContainmentCloseSucceeded = true
		}
	}
	observationData, marshalErr := marshalSupervisionStopActuationJSON(observation)
	if marshalErr != nil {
		result.Err = errors.Join(requestErr, actuationErr, marshalErr)
		return result
	}
	_, observationErr := rekitfs.WriteExclusiveRegularFileAnchoredWriteThrough(
		scope.paths.runRoot,
		observationPath,
		"Claude supervision stop actuation observation",
		observationData,
	)
	if observationErr == nil {
		result.ObservationSHA256 = bytesSHA256(observationData)
	}
	result.Err = errors.Join(requestErr, actuationErr, observationErr)
	return result
}

func validateSupervisionStopActuationContext(scope supervisionStopActuationContext) error {
	if scope.spec.SchemaVersion != 1 || scope.spec.Kind != supervisionSpecKind ||
		!validClaudeLaunchSHA256(scope.spec.RunID) ||
		!validClaudeLaunchSHA256(scope.specSHA256) ||
		strings.TrimSpace(scope.spec.SessionID) == "" ||
		scope.spec.LaunchControl == nil {
		return fmt.Errorf("Claude stop actuation context is incomplete")
	}
	if err := validateClaudeLaunchControlBinding(*scope.spec.LaunchControl); err != nil {
		return err
	}
	expectedPaths := supervisionPathsForRun(scope.paths.root, scope.spec.RunID)
	if scope.paths != expectedPaths {
		return fmt.Errorf("Claude stop actuation paths do not match the exact supervisor run")
	}
	data, err := rekitfs.ReadStableRegularFileAnchored(
		scope.paths.runRoot,
		scope.paths.spec,
		"Claude supervision stop actuation spec",
		2*1024*1024,
	)
	if err != nil {
		return err
	}
	if !strings.EqualFold(bytesSHA256(data), scope.specSHA256) {
		return fmt.Errorf("Claude stop actuation spec sha256 changed")
	}
	var actual supervisionSpec
	if err := strictJSON(data, &actual); err != nil {
		return fmt.Errorf("decode Claude stop actuation spec: %w", err)
	}
	actualCanonical, err := json.Marshal(actual)
	if err != nil {
		return err
	}
	expectedCanonical, err := json.Marshal(scope.spec)
	if err != nil {
		return err
	}
	if !bytes.Equal(actualCanonical, expectedCanonical) {
		return fmt.Errorf("Claude stop actuation spec differs from the exact owned run")
	}
	return nil
}

func supervisionStopActuationArtifactPaths(generation int) (string, string) {
	base := fmt.Sprintf("%020d", generation)
	return filepath.ToSlash(filepath.Join("stop-actuation", base+".request.json")),
		filepath.ToSlash(filepath.Join("stop-actuation", base+".observation.json"))
}

func marshalSupervisionStopActuationJSON(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func readSupervisionStopActuationObservation(
	scope supervisionStopActuationContext,
	path string,
) (supervisionStopActuationObservation, []byte, bool, error) {
	var observation supervisionStopActuationObservation
	data, err := rekitfs.ReadStableRegularFileAnchored(
		scope.paths.runRoot,
		filepath.Join(scope.paths.runRoot, filepath.FromSlash(path)),
		"Claude supervision stop actuation observation",
		256*1024,
	)
	if errors.Is(err, os.ErrNotExist) {
		return observation, nil, false, nil
	}
	if err != nil {
		return observation, nil, false, err
	}
	if err := strictJSON(data, &observation); err != nil {
		return observation, nil, false, fmt.Errorf("decode Claude supervision stop actuation observation: %w", err)
	}
	if err := validateSupervisionStopActuationObservation(scope, path, observation); err != nil {
		return observation, nil, false, err
	}
	return observation, data, true, nil
}

func validateTerminalStopActuation(
	paths supervisionPaths,
	spec supervisionSpec,
	specSHA string,
	terminal supervisionTerminal,
) error {
	lineagePresent := strings.TrimSpace(terminal.StopActuationRequestPath) != "" ||
		strings.TrimSpace(terminal.StopActuationObservationPath) != "" ||
		strings.TrimSpace(terminal.StopActuationObservationSHA256) != ""
	if !lineagePresent {
		return nil
	}
	if terminal.StopActuationRequestPath == "" || terminal.StopActuationObservationPath == "" ||
		!validClaudeLaunchSHA256(terminal.StopActuationObservationSHA256) || spec.LaunchControl == nil {
		return fmt.Errorf("Claude supervision terminal stop actuation lineage is incomplete")
	}
	scope := supervisionStopActuationContext{paths: paths, spec: spec, specSHA256: specSHA}
	observation, data, ok, err := readSupervisionStopActuationObservation(scope, terminal.StopActuationObservationPath)
	if err != nil {
		return fmt.Errorf("read Claude supervision terminal stop actuation observation: %w", err)
	}
	if !ok {
		return fmt.Errorf("Claude supervision terminal stop actuation observation is missing")
	}
	if terminal.StopActuationRequestPath != observation.RequestPath ||
		!strings.EqualFold(terminal.StopActuationObservationSHA256, bytesSHA256(data)) {
		return fmt.Errorf("Claude supervision terminal stop actuation lineage changed")
	}
	if observation.Outcome == "owned-containment-closed" {
		if terminal.StopActuationError != "" {
			return fmt.Errorf("successful Claude stop actuation terminal carries an error")
		}
	} else if strings.TrimSpace(terminal.StopActuationError) == "" {
		return fmt.Errorf("failed Claude stop actuation terminal omitted its error")
	}
	return nil
}

func validateSupervisionStopActuationObservation(
	scope supervisionStopActuationContext,
	path string,
	observation supervisionStopActuationObservation,
) error {
	requestPath, observationPath := supervisionStopActuationArtifactPaths(observation.StopControlGeneration)
	if observation.SchemaVersion != 1 || observation.Kind != supervisionStopActuationObservationKind ||
		observation.RunID != scope.spec.RunID || observation.SpecSHA256 != scope.specSHA256 ||
		observation.SessionID != scope.spec.SessionID || scope.spec.LaunchControl == nil ||
		observation.Lane != scope.spec.LaunchControl.Lane ||
		observation.StopControlGeneration <= scope.spec.LaunchControl.ControlGeneration ||
		!validClaudeLaunchSHA256(observation.StopControlReceiptSHA256) ||
		path != observationPath || observation.RequestPath != requestPath ||
		!validClaudeLaunchSHA256(observation.RequestSHA256) ||
		!observation.NoProcessTerminationClaim || !observation.NoRemoteSessionControl ||
		!observation.NoAuthority || !observation.NoConfirmed || !observation.NoHeavyTool {
		return fmt.Errorf("Claude supervision stop actuation observation does not match the exact run")
	}
	if _, err := time.Parse(time.RFC3339Nano, observation.ObservedAt); err != nil {
		return fmt.Errorf("Claude supervision stop actuation observedAt is invalid: %w", err)
	}
	switch observation.Outcome {
	case "owned-containment-closed":
		if !observation.RequestPublished || !observation.ContainmentCloseAttempted ||
			!observation.ContainmentCloseSucceeded || observation.Error != "" {
			return fmt.Errorf("successful Claude stop actuation observation is inconsistent")
		}
	case "actuation-failed":
		if !observation.RequestPublished || !observation.ContainmentCloseAttempted ||
			observation.ContainmentCloseSucceeded || strings.TrimSpace(observation.Error) == "" {
			return fmt.Errorf("failed Claude stop actuation observation is inconsistent")
		}
	case "request-publication-failed":
		if observation.RequestPublished || observation.ContainmentCloseAttempted ||
			observation.ContainmentCloseSucceeded || strings.TrimSpace(observation.Error) == "" {
			return fmt.Errorf("failed Claude stop request observation is inconsistent")
		}
	default:
		return fmt.Errorf("unsupported Claude stop actuation outcome %q", observation.Outcome)
	}
	if observation.RequestPublished {
		requestData, err := rekitfs.ReadStableRegularFileAnchored(
			scope.paths.runRoot,
			filepath.Join(scope.paths.runRoot, filepath.FromSlash(requestPath)),
			"Claude supervision stop actuation request",
			256*1024,
		)
		if err != nil {
			return fmt.Errorf("read Claude supervision stop actuation request: %w", err)
		}
		if !strings.EqualFold(bytesSHA256(requestData), observation.RequestSHA256) {
			return fmt.Errorf("Claude supervision stop actuation request binding is invalid")
		}
		var request supervisionStopActuationRequest
		if err := strictJSON(requestData, &request); err != nil {
			return fmt.Errorf("decode Claude supervision stop actuation request: %w", err)
		}
		if err := validateSupervisionStopActuationRequest(scope, requestPath, request); err != nil {
			return err
		}
		if request.StopReceipt.ControlGeneration != observation.StopControlGeneration ||
			!strings.EqualFold(request.StopReceiptSHA256, observation.StopControlReceiptSHA256) {
			return fmt.Errorf("Claude supervision stop actuation observation changed its stop receipt lineage")
		}
	}
	return nil
}

func validateSupervisionStopActuationRequest(
	scope supervisionStopActuationContext,
	path string,
	request supervisionStopActuationRequest,
) error {
	expectedPath, _ := supervisionStopActuationArtifactPaths(request.StopReceipt.ControlGeneration)
	if request.SchemaVersion != 1 || request.Kind != supervisionStopActuationRequestKind ||
		request.RunID != scope.spec.RunID || request.SpecSHA256 != scope.specSHA256 ||
		request.SessionID != scope.spec.SessionID || scope.spec.LaunchControl == nil ||
		request.LaunchControl != *scope.spec.LaunchControl || path != expectedPath ||
		request.StopReceipt.Action != executioncontrol.ActionStop ||
		request.StopReceipt.State != executioncontrol.StateStopped ||
		request.StopReceipt.ControlGeneration <= request.LaunchControl.ControlGeneration ||
		request.StopReceipt.Owner != request.LaunchControl.Owner ||
		request.StopReceiptPath == "" || !validClaudeLaunchSHA256(request.StopReceiptSHA256) ||
		request.StopCommittedAt != request.StopReceipt.CommittedAt ||
		!request.OwnedContainmentOnly || !request.NoRemoteSessionControl ||
		!request.NoAuthority || !request.NoConfirmed || !request.NoHeavyTool {
		return fmt.Errorf("Claude supervision stop actuation request does not match the exact run and durable stop")
	}
	return nil
}
