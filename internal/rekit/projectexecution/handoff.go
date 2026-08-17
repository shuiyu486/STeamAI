package projectexecution

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectlock"
)

const (
	HandoffSchemaVersion    = 1
	HandoffKind             = "project-execution-supervisor-handoff"
	handoffCancellationKind = "project-execution-supervisor-handoff-canceled"
	handoffCancellationMax  = 1024
)

type Handoff struct {
	SchemaVersion int    `json:"schemaVersion"`
	Kind          string `json:"kind"`
	ProjectKey    string `json:"projectKey"`
	RunID         string `json:"runId"`
	SpecSHA256    string `json:"specSha256"`
	SessionID     string `json:"sessionId"`
}

type handoffCancellation struct {
	SchemaVersion int    `json:"schemaVersion"`
	Kind          string `json:"kind"`
	ProjectKey    string `json:"projectKey"`
	RunID         string `json:"runId"`
	SpecSHA256    string `json:"specSha256"`
	SessionID     string `json:"sessionId"`
}

var handoffHandleBoundExactMutationSupported = rekitfs.HandleBoundExactMutationSupported

// DurableHandoffSupported reports whether pending supervisor handoffs can be
// published and consumed with handle-bound exact filesystem mutation.
func DurableHandoffSupported() bool {
	return handoffHandleBoundExactMutationSupported()
}

func requireDurableHandoffSupport() error {
	if DurableHandoffSupported() {
		return nil
	}
	return fmt.Errorf("project execution supervisor handoff requires handle-bound exact filesystem mutation support")
}

func NewHandoff(caseRoot, runID, specSHA256, sessionID string) (Handoff, error) {
	key, err := projectlock.CanonicalProjectKey(caseRoot)
	if err != nil {
		return Handoff{}, err
	}
	handoff := Handoff{
		SchemaVersion: HandoffSchemaVersion,
		Kind:          HandoffKind,
		ProjectKey:    key,
		RunID:         strings.ToLower(strings.TrimSpace(runID)),
		SpecSHA256:    strings.ToLower(strings.TrimSpace(specSHA256)),
		SessionID:     strings.TrimSpace(sessionID),
	}
	if err := validateHandoff(handoff, key); err != nil {
		return Handoff{}, err
	}
	return handoff, nil
}

// PublishHandoff installs the exact pending marker before the supervisor child
// is started. A canceled run can never be published again, preventing an old
// child from claiming a later ABA replay of the same binding.
func PublishHandoff(caseRoot string, handoff Handoff) error {
	if err := requireDurableHandoffSupport(); err != nil {
		return err
	}
	rootPath, key, err := handoffRoot(caseRoot)
	if err != nil {
		return err
	}
	if err := validateHandoff(handoff, key); err != nil {
		return err
	}
	canceled, err := handoffCanceled(rootPath, handoff)
	if err != nil {
		return err
	}
	if canceled {
		return fmt.Errorf("project execution supervisor handoff is permanently canceled for run %s", handoff.RunID)
	}
	if err := validateHandoffCancellationCapacity(rootPath, handoff); err != nil {
		return err
	}
	data, err := handoffData(handoff)
	if err != nil {
		return err
	}
	_, err = rekitfs.WriteExclusiveRegularFileAnchoredWriteThrough(
		rootPath,
		handoffPendingName(key),
		"project execution supervisor handoff intent",
		data,
	)
	return err
}

// ClaimHandoff consumes the exact pending marker. The child may call it only
// after acquiring and validating its own shared execution lease. Cancellation
// is checked both before and after removal so an old run cannot survive a
// maintenance cancellation or an exact-binding replay.
func ClaimHandoff(caseRoot string, handoff Handoff) error {
	rootPath, key, err := handoffRoot(caseRoot)
	if err != nil {
		return err
	}
	if err := validateHandoff(handoff, key); err != nil {
		return err
	}
	canceled, err := handoffCanceled(rootPath, handoff)
	if err != nil {
		return err
	}
	if canceled {
		return fmt.Errorf("project execution supervisor handoff is permanently canceled for run %s", handoff.RunID)
	}
	removed, err := removeExpectedHandoffAt(rootPath, key, handoff)
	if err != nil {
		return err
	}
	if !removed {
		return fmt.Errorf("project execution supervisor handoff is no longer pending for run %s", handoff.RunID)
	}
	canceled, err = handoffCanceled(rootPath, handoff)
	if err != nil {
		return err
	}
	if canceled {
		return fmt.Errorf("project execution supervisor handoff was canceled while run %s was claimed", handoff.RunID)
	}
	return nil
}

// CancelPendingHandoff durably cancels the one active marker after an exclusive
// project execution lease has been acquired and before any kit or lane mutation.
// The cancellation receipt is published before pending cleanup, so replay can
// safely finish a crash between those two operations.
func CancelPendingHandoff(caseRoot string) (Handoff, bool, error) {
	rootPath, key, err := handoffRoot(caseRoot)
	if err != nil {
		return Handoff{}, false, err
	}
	handoff, err := readHandoff(rootPath, key)
	if errors.Is(err, os.ErrNotExist) {
		return Handoff{}, false, nil
	}
	if err != nil {
		return Handoff{}, false, err
	}
	if err := validateHandoff(handoff, key); err != nil {
		return Handoff{}, false, err
	}
	if err := requireDurableHandoffSupport(); err != nil {
		return Handoff{}, false, err
	}
	if err := publishHandoffCancellation(rootPath, handoff); err != nil {
		return Handoff{}, false, err
	}
	if err := removeHandoffFile(rootPath, key, handoff); err != nil {
		return Handoff{}, false, err
	}
	return handoff, true, nil
}

// CancelHandoff durably cancels only the expected run marker. Missing is
// accepted so startup failure and supervision fencing remain idempotent. A
// different pending run is left untouched.
func CancelHandoff(caseRoot string, expected Handoff) error {
	rootPath, key, err := handoffRoot(caseRoot)
	if err != nil {
		return err
	}
	if err := validateHandoff(expected, key); err != nil {
		return err
	}
	actual, err := readHandoff(rootPath, key)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !handoffEqual(actual, expected) {
		return nil
	}
	if err := requireDurableHandoffSupport(); err != nil {
		return err
	}
	if err := publishHandoffCancellation(rootPath, expected); err != nil {
		return err
	}
	return removeHandoffFile(rootPath, key, expected)
}

func removeExpectedHandoffAt(rootPath, key string, expected Handoff) (bool, error) {
	actual, err := readHandoff(rootPath, key)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !handoffEqual(actual, expected) {
		return false, fmt.Errorf("project execution supervisor handoff intent changed")
	}
	if err := requireDurableHandoffSupport(); err != nil {
		return false, err
	}
	if err := removeHandoffFile(rootPath, key, expected); err != nil {
		return false, err
	}
	return true, nil
}

func handoffRoot(caseRoot string) (string, string, error) {
	casePath, err := filepath.Abs(caseRoot)
	if err != nil {
		return "", "", err
	}
	key, err := projectlock.CanonicalProjectKey(casePath)
	if err != nil {
		return "", "", err
	}
	rootPath, err := projectlock.WorkstreamRoot()
	if err != nil {
		return "", "", err
	}
	if err := os.MkdirAll(rootPath, 0o700); err != nil {
		return "", "", err
	}
	if _, err := rekitfs.ValidateNonReparseDirectory(rootPath, "project execution handoff root"); err != nil {
		return "", "", err
	}
	return rootPath, key, nil
}

func readHandoff(rootPath, key string) (Handoff, error) {
	path := filepath.Join(rootPath, handoffPendingName(key))
	data, err := rekitfs.ReadStableRegularFileAnchored(
		rootPath,
		path,
		"project execution supervisor handoff intent",
		64*1024,
	)
	if err != nil {
		return Handoff{}, err
	}
	var handoff Handoff
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&handoff); err != nil {
		return Handoff{}, fmt.Errorf("decode project execution supervisor handoff intent: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return Handoff{}, fmt.Errorf("decode project execution supervisor handoff intent: trailing data: %w", err)
	}
	return handoff, nil
}

func removeHandoffFile(rootPath, key string, handoff Handoff) error {
	data, err := handoffData(handoff)
	if err != nil {
		return err
	}
	root, err := rekitfs.OpenAnchoredRoot(rootPath)
	if err != nil {
		return err
	}
	removeErr := root.RemoveExactFile(handoffPendingName(key), data, 0o600)
	validateErr := root.Validate()
	closeErr := root.Close()
	return errors.Join(removeErr, validateErr, closeErr)
}

func validateHandoffCancellationCapacity(rootPath string, handoff Handoff) (retErr error) {
	root, err := rekitfs.OpenAnchoredRoot(rootPath)
	if err != nil {
		return err
	}
	defer func() {
		retErr = errors.Join(retErr, root.Close())
	}()
	directory := handoffCancellationDir(handoff.ProjectKey)
	if _, err := root.Lstat(directory); errors.Is(err, os.ErrNotExist) {
		return root.Validate()
	} else if err != nil {
		return err
	}
	names, err := root.ListRegularFilesNoFollow(directory, handoffCancellationMax)
	if err != nil {
		return fmt.Errorf("validate project execution supervisor handoff cancellation capacity: %w", err)
	}
	if len(names) >= handoffCancellationMax {
		return fmt.Errorf("project execution supervisor handoff cancellation history reached its fail-closed limit of %d records", handoffCancellationMax)
	}
	return root.Validate()
}

func publishHandoffCancellation(rootPath string, handoff Handoff) error {
	cancellation := handoffCancellationFor(handoff)
	data, err := handoffCancellationData(cancellation)
	if err != nil {
		return err
	}
	_, err = rekitfs.WriteExclusiveRegularFileAnchoredWriteThrough(
		rootPath,
		handoffCancellationName(handoff),
		"project execution supervisor handoff cancellation",
		data,
	)
	return err
}

func handoffCanceled(rootPath string, handoff Handoff) (bool, error) {
	path := filepath.Join(rootPath, handoffCancellationName(handoff))
	data, err := rekitfs.ReadStableRegularFileAnchored(
		rootPath,
		path,
		"project execution supervisor handoff cancellation",
		64*1024,
	)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var cancellation handoffCancellation
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cancellation); err != nil {
		return false, fmt.Errorf("decode project execution supervisor handoff cancellation: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("decode project execution supervisor handoff cancellation: trailing data: %w", err)
	}
	expected := handoffCancellationFor(handoff)
	if cancellation != expected {
		return false, fmt.Errorf("project execution supervisor handoff cancellation binding is invalid")
	}
	return true, nil
}

func handoffCancellationFor(handoff Handoff) handoffCancellation {
	return handoffCancellation{
		SchemaVersion: handoff.SchemaVersion,
		Kind:          handoffCancellationKind,
		ProjectKey:    handoff.ProjectKey,
		RunID:         handoff.RunID,
		SpecSHA256:    handoff.SpecSHA256,
		SessionID:     handoff.SessionID,
	}
}

func handoffCancellationData(cancellation handoffCancellation) ([]byte, error) {
	data, err := json.MarshalIndent(cancellation, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func handoffData(handoff Handoff) ([]byte, error) {
	data, err := json.MarshalIndent(handoff, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func validateHandoff(handoff Handoff, key string) error {
	if handoff.SchemaVersion != HandoffSchemaVersion || handoff.Kind != HandoffKind ||
		handoff.ProjectKey != key || !validHandoffSHA(handoff.ProjectKey) ||
		!validHandoffSHA(handoff.RunID) || !validHandoffSHA(handoff.SpecSHA256) ||
		!validHandoffSession(handoff.SessionID) {
		return fmt.Errorf("project execution supervisor handoff binding is invalid")
	}
	return nil
}

func handoffEqual(left, right Handoff) bool {
	return left == right
}

func handoffPendingName(key string) string {
	return "case-" + key + ".execution-v1.handoff.json"
}

func handoffCancellationDir(key string) string {
	return "case-" + key + ".execution-v1.handoff-canceled"
}

func handoffCancellationName(handoff Handoff) string {
	identity := handoff.RunID + "\x00" + handoff.SpecSHA256 + "\x00" + handoff.SessionID
	return filepath.Join(
		handoffCancellationDir(handoff.ProjectKey),
		handoffDigest(identity)+".json",
	)
}

func handoffDigest(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func validHandoffSHA(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func validHandoffSession(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && len(value) <= 256 && !strings.ContainsAny(value, "\r\n/\\")
}
