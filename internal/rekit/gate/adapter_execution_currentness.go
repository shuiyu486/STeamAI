package gate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/adapterexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/autonomy"
	"github.com/shuiyu486/re-context-kits/internal/rekit/laneowner"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
)

// AdapterExecutionCurrentness is a read-only snapshot proving that an
// immutable dispatch is still covered by the current durable authorization.
type AdapterExecutionCurrentness struct {
	SchemaVersion  int                           `json:"schemaVersion"`
	Kind           string                        `json:"kind"`
	GateEventID    string                        `json:"gateEventId"`
	DispatchPath   string                        `json:"dispatchPath"`
	DispatchSHA256 string                        `json:"dispatchSha256"`
	DispatchBytes  int64                         `json:"dispatchBytes"`
	ProfilePath    string                        `json:"profilePath"`
	ProfileSHA256  string                        `json:"profileSha256"`
	Owner          adapterexecution.OwnerBinding `json:"owner"`
}

// ValidateAdapterExecutionCurrentness re-reads the strict profile, authorized
// gate, durable owner, and immutable dispatch. It never creates, renews, or
// migrates authorization.
func ValidateAdapterExecutionCurrentness(
	repoRoot,
	caseRoot string,
	expected adapterexecution.DispatchReceipt,
	expectedPath,
	expectedSHA256 string,
) (AdapterExecutionCurrentness, error) {
	result := AdapterExecutionCurrentness{
		SchemaVersion: 1,
		Kind:          "adapter-execution-authorization-currentness",
		GateEventID:   strings.TrimSpace(expected.Gate.GateEventID),
	}
	if err := adapterexecution.ValidateDispatch(expected); err != nil {
		return result, err
	}
	expectedPath = strings.TrimSpace(expectedPath)
	expectedSHA256 = strings.TrimSpace(expectedSHA256)
	if expectedPath == "" || !validSHA256String(expectedSHA256) {
		return result, fmt.Errorf("adapter execution currentness requires an exact dispatch path and sha256")
	}

	current, currentPath, currentSHA256, currentBytes, err :=
		ReadCurrentAdapterExecutionDispatch(
			repoRoot,
			caseRoot,
			expected.Adapter.Pack,
			expected.Gate.GateEventID,
		)
	if err != nil {
		return result, err
	}
	if currentPath != expectedPath ||
		!strings.EqualFold(currentSHA256, expectedSHA256) ||
		!adapterexecution.DispatchSemanticEqual(current, expected) {
		return result, fmt.Errorf("adapter execution dispatch is no longer current")
	}

	owner, err := laneowner.Read(caseRoot, expected.Owner.Lane)
	if err != nil {
		return result, err
	}
	if owner.Lane != expected.Owner.Lane ||
		owner.CurrentExecutor != expected.Owner.CurrentExecutor ||
		owner.ExecutorGeneration != expected.Owner.ExecutorGeneration {
		return result, fmt.Errorf(
			"adapter execution owner is stale: current executor=%s generation=%d",
			owner.CurrentExecutor,
			owner.ExecutorGeneration,
		)
	}

	m, err := manifest.Load(repoRoot, expected.Adapter.Pack)
	if err != nil {
		return result, err
	}
	profile, profilePath, exists, err := autonomy.Read(caseRoot, expected.Owner.Lane)
	if err != nil {
		return result, fmt.Errorf("read current adapter execution autonomy profile: %w", err)
	}
	if !exists {
		return result, fmt.Errorf("current adapter execution autonomy profile is missing")
	}
	profileSHA256 := autonomy.FileHash(profilePath)
	fresh, freshPath, freshExists, err := autonomy.Read(caseRoot, expected.Owner.Lane)
	if err != nil {
		return result, fmt.Errorf("re-read current adapter execution autonomy profile: %w", err)
	}
	if !freshExists || freshPath != profilePath || profileSHA256 == "" ||
		autonomy.FileHash(profilePath) != profileSHA256 ||
		!reflect.DeepEqual(profile, fresh) {
		return result, fmt.Errorf("adapter execution autonomy profile changed while reading")
	}
	profile = fresh
	if err := autonomy.Validate(profile, expected.Owner.Lane, m, caseRoot); err != nil {
		return result, err
	}
	if profile.Mode != autonomy.ModePreauthorized &&
		profile.Mode != autonomy.ModeAutonomous {
		return result, fmt.Errorf(
			"adapter execution autonomy profile is not current preauthorized or autonomous",
		)
	}
	if autonomy.IsExpired(profile, time.Now().UTC()) {
		return result, fmt.Errorf("adapter execution autonomy profile is expired")
	}

	authorization := expected.Gate.Authorization
	profileRel := autonomy.RelPath(expected.Owner.Lane)
	if authorization.Decision != autonomy.DecisionPreauthorized ||
		authorization.RequiresConfirmation ||
		authorization.ProfilePath != profileRel ||
		!strings.EqualFold(authorization.ProfileHash, profileSHA256) {
		return result, fmt.Errorf("adapter execution authorization profile hash, path, or decision drifted")
	}
	request := autonomy.Request{
		Lane:           expected.Owner.Lane,
		Action:         expected.Gate.Action,
		Target:         expected.Gate.Target,
		Budget:         expected.Gate.AuthorizedBudget,
		StopConditions: append([]string{}, expected.Gate.StopConditions...),
		OutputPaths:    append([]string{}, expected.Gate.OutputPaths...),
	}
	freshDecision := autonomy.Evaluate(
		profile,
		profileRel,
		true,
		profileSHA256,
		request,
		time.Now().UTC(),
	)
	if !adapterAuthorizationDecisionEqual(freshDecision, authorization) {
		return result, fmt.Errorf("adapter execution authorization decision drifted from the current profile")
	}

	result.DispatchPath = currentPath
	result.DispatchSHA256 = currentSHA256
	result.DispatchBytes = currentBytes
	result.ProfilePath = profileRel
	result.ProfileSHA256 = profileSHA256
	result.Owner = current.Owner
	return result, nil
}

func adapterAuthorizationDecisionEqual(left, right autonomy.Decision) bool {
	leftData, leftErr := json.Marshal(left)
	rightData, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftData, rightData)
}
