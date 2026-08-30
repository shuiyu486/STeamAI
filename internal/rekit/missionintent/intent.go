package missionintent

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/caseshim"
	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtimebundle"
)

const (
	// Legacy relative constants remain for callers without a case root. New
	// case-local publication code must use ArtifactPaths.
	MissionIntentRel       = ".rekit/mission-intent.json"
	ProjectBindingRel      = ".steamai/project-binding.json"
	IntentRel              = ".rekit/onboarding/intent.json"
	CommitRel              = ".rekit/onboarding/commit.json"
	maxArtifactBytes       = 64 * 1024
	maxIntentArtifactBytes = 8 * 1024 * 1024
	maxRecoveryWrites      = 512
	maxRecoveryBytes       = 5 * 1024 * 1024
)

type Identity struct {
	SchemaVersion int    `json:"schemaVersion"`
	Target        string `json:"target"`
	ProjectID     string `json:"projectId,omitempty"`
	Pack          string `json:"pack"`
	ProjectName   string `json:"projectName"`
	Goal          string `json:"goal"`
	Actor         string `json:"actor"`
	Executor      string `json:"executor"`
	InitialLane   string `json:"initialLane"`
}

type Intent struct {
	SchemaVersion        int              `json:"schemaVersion"`
	Kind                 string           `json:"kind"`
	PublicationStamp     string           `json:"publicationStamp"`
	OnboardingPlanSHA256 string           `json:"onboardingPlanSha256"`
	ProjectBindingSHA256 string           `json:"projectBindingSha256,omitempty"`
	Identity             Identity         `json:"identity"`
	Recovery             RecoveryEnvelope `json:"recovery"`
}

type RecoveryEnvelope struct {
	SchemaVersion    int                `json:"schemaVersion"`
	RepoRoot         string             `json:"repoRoot"`
	CreatedAt        string             `json:"createdAt"`
	Mode             string             `json:"mode,omitempty"`
	BundleManifest   BundleBinding      `json:"bundleManifest"`
	Writes           []RecoveryWrite    `json:"writes,omitempty"`
	AttachedSnapshot []SnapshotArtifact `json:"attachedSnapshot,omitempty"`
}

type BundleBinding struct {
	Path   string `json:"path,omitempty"`
	SHA256 string `json:"sha256,omitempty"`
	Files  int    `json:"files,omitempty"`
}

type ProjectBinding struct {
	SchemaVersion        int    `json:"schemaVersion"`
	ProjectID            string `json:"projectId"`
	Target               string `json:"target"`
	MissionIntentSHA256  string `json:"missionIntentSha256"`
	PublicationStamp     string `json:"publicationStamp"`
	OnboardingPlanSHA256 string `json:"onboardingPlanSha256"`
	NoAuthority          bool   `json:"noAuthority"`
	NoConfirmed          bool   `json:"noConfirmed"`
	NoHeavyTool          bool   `json:"noHeavyTool"`
}

type RecoveryWrite struct {
	Path             string `json:"path"`
	Kind             string `json:"kind"`
	SHA256           string `json:"sha256"`
	Size             int64  `json:"size"`
	Content          []byte `json:"content"`
	PublicationPhase int    `json:"publicationPhase"`
}

type SnapshotArtifact struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

type Commit struct {
	SchemaVersion        int    `json:"schemaVersion"`
	Kind                 string `json:"kind"`
	PublicationStamp     string `json:"publicationStamp"`
	OnboardingPlanSHA256 string `json:"onboardingPlanSha256"`
	MissionIntentSHA256  string `json:"missionIntentSha256"`
	IntentSHA256         string `json:"intentSha256"`
}

// CommittedGeneration contains the exact canonical bytes of one immutable
// current onboarding generation. It is a pure projection and never writes.
type CommittedGeneration struct {
	Identity            Identity
	ProjectBinding      ProjectBinding
	Intent              Intent
	Commit              Commit
	MissionIntentBytes  []byte
	ProjectBindingBytes []byte
	IntentBytes         []byte
	CommitBytes         []byte
}

// PlanHashWrite and PlanHashInput define the stable reviewed onboarding plan
// identity shared by ordinary onboarding and state migration projection.
type PlanHashWrite struct {
	Path             string `json:"path"`
	Kind             string `json:"kind"`
	SHA256           string `json:"sha256"`
	Size             int64  `json:"size"`
	PublicationPhase int    `json:"publicationPhase"`
}

type PlanHashInput struct {
	SchemaVersion int             `json:"schemaVersion"`
	Command       string          `json:"command"`
	CaseRoot      string          `json:"caseRoot"`
	RepoRoot      string          `json:"repoRoot"`
	Pack          string          `json:"pack"`
	ProjectName   string          `json:"projectName"`
	ProvisionID   string          `json:"provisionId"`
	Role          string          `json:"role"`
	CreatedAt     string          `json:"createdAt"`
	LineageSHA256 string          `json:"lineageSha256,omitempty"`
	Writes        []PlanHashWrite `json:"writes"`
}

const (
	AttachedOnboardingPlanCommand = "attached-onboarding-adoption"
	OnboardingPlanSHA256Marker    = "<onboarding-plan-sha256>"
)

type ArtifactPaths struct {
	MissionIntent  string
	ProjectBinding string
	Intent         string
	Commit         string
}

func Paths(caseRoot string) (ArtifactPaths, error) {
	root, err := projectstate.Resolve(caseRoot)
	if err != nil {
		return ArtifactPaths{}, err
	}
	paths := artifactPaths(root)
	view, err := projectstate.ResolveMissionView(caseRoot)
	if err != nil {
		return ArtifactPaths{}, err
	}
	paths.MissionIntent, err = view.Rel("mission-intent.json")
	if err != nil {
		return ArtifactPaths{}, err
	}
	return paths, nil
}

func artifactPaths(root projectstate.Root) ArtifactPaths {
	paths := ArtifactPaths{
		MissionIntent: filepath.ToSlash(filepath.Join(root.Dir, "mission-intent.json")),
		Intent:        filepath.ToSlash(filepath.Join(root.Dir, "onboarding", "intent.json")),
		Commit:        filepath.ToSlash(filepath.Join(root.Dir, "onboarding", "commit.json")),
	}
	if !root.Legacy {
		paths.ProjectBinding = filepath.ToSlash(filepath.Join(root.Dir, "project-binding.json"))
	}
	return paths
}

type Inspection struct {
	State                string           `json:"state"`
	Committed            bool             `json:"committed"`
	PublicationStamp     string           `json:"publicationStamp,omitempty"`
	OnboardingPlanSHA256 string           `json:"onboardingPlanSha256,omitempty"`
	Identity             Identity         `json:"identity"`
	ProjectBinding       *ProjectBinding  `json:"projectBinding,omitempty"`
	MissionIntentSHA256  string           `json:"missionIntentSha256,omitempty"`
	ProjectBindingSHA256 string           `json:"projectBindingSha256,omitempty"`
	IntentSHA256         string           `json:"intentSha256,omitempty"`
	CommitSHA256         string           `json:"commitSha256,omitempty"`
	ApplyArgs            []string         `json:"applyArgs,omitempty"`
	Recovery             RecoveryEnvelope `json:"-"`
}

func MarshalMissionIntent(identity Identity) ([]byte, error) {
	if err := ValidateIdentity(identity); err != nil {
		return nil, err
	}
	return marshalCanonical(identity)
}

func MarshalMissionIntentAt(caseRoot string, identity Identity) ([]byte, error) {
	if err := ValidateIdentityAt(caseRoot, identity); err != nil {
		return nil, err
	}
	return marshalBoundedCanonical(identity, maxArtifactBytes, "mission intent")
}

func MarshalProjectBindingAt(caseRoot string, value ProjectBinding) ([]byte, error) {
	if err := ValidateProjectBindingAt(caseRoot, value); err != nil {
		return nil, err
	}
	return marshalBoundedCanonical(value, maxArtifactBytes, "project binding")
}

func MarshalIntent(value Intent) ([]byte, error) {
	if err := validateLegacyIntent(value); err != nil {
		return nil, err
	}
	return marshalBoundedCanonical(value, maxIntentArtifactBytes, "onboarding intent")
}

func MarshalIntentAt(caseRoot string, value Intent) ([]byte, error) {
	physicalRoot, _, err := resolvePhysicalCaseRoot(caseRoot)
	if err != nil {
		return nil, err
	}
	if value.SchemaVersion == 2 {
		value.Recovery, err = canonicalizeRecoveryEnvelope(physicalRoot, value.Identity, value.Recovery)
		if err != nil {
			return nil, err
		}
	}
	if err := ValidateIntentAt(physicalRoot, value); err != nil {
		return nil, err
	}
	return marshalBoundedCanonical(value, maxIntentArtifactBytes, "onboarding intent")
}

func CanonicalizeProjectedCurrentRecoveryAt(caseRoot string, identity Identity, recovery RecoveryEnvelope) (RecoveryEnvelope, error) {
	physicalRoot, err := projectedPhysicalCaseRoot(caseRoot)
	if err != nil {
		return RecoveryEnvelope{}, err
	}
	if identity.SchemaVersion != 2 {
		return RecoveryEnvelope{}, fmt.Errorf("projected current recovery requires schemaVersion 2 identity")
	}
	return canonicalizeRecoveryEnvelope(physicalRoot, identity, recovery)
}

func MarshalCommit(value Commit) ([]byte, error) {
	if err := validateCommit(value); err != nil {
		return nil, err
	}
	return marshalBoundedCanonical(value, maxArtifactBytes, "onboarding commit")
}

func MarshalCommitAt(caseRoot string, value Commit) ([]byte, error) {
	if err := ValidateCommitAt(caseRoot, value); err != nil {
		return nil, err
	}
	return marshalBoundedCanonical(value, maxArtifactBytes, "onboarding commit")
}

// MarshalCommittedV2Projected builds one future current generation without
// consulting the selected on-disk state root. Callers must validate the
// supplied identity, recovery envelope, and source lineage before projection.
func MarshalCommittedV2Projected(caseRoot string, identity Identity, recovery RecoveryEnvelope, stamp, planSHA256 string) (CommittedGeneration, error) {
	if !validStamp(stamp) || !validPlanHash(planSHA256) {
		return CommittedGeneration{}, fmt.Errorf("invalid projected current onboarding generation")
	}
	canonicalRecovery, err := CanonicalizeProjectedCurrentRecoveryAt(caseRoot, identity, recovery)
	if err != nil {
		return CommittedGeneration{}, err
	}
	if err := ValidateProjectedCurrentRecoveryAt(caseRoot, identity, canonicalRecovery); err != nil {
		return CommittedGeneration{}, err
	}
	recovery = canonicalRecovery
	missionBytes, err := marshalBoundedCanonical(identity, maxArtifactBytes, "mission intent")
	if err != nil {
		return CommittedGeneration{}, err
	}
	binding := ProjectBinding{
		SchemaVersion: 1, ProjectID: identity.ProjectID, Target: ".", MissionIntentSHA256: SHA256(missionBytes),
		PublicationStamp: stamp, OnboardingPlanSHA256: planSHA256, NoAuthority: true, NoConfirmed: true, NoHeavyTool: true,
	}
	bindingBytes, err := marshalBoundedCanonical(binding, maxArtifactBytes, "project binding")
	if err != nil {
		return CommittedGeneration{}, err
	}
	intent := Intent{
		SchemaVersion: 2, Kind: "mission-onboarding-intent", PublicationStamp: stamp, OnboardingPlanSHA256: planSHA256,
		ProjectBindingSHA256: SHA256(bindingBytes), Identity: identity, Recovery: recovery,
	}
	intentBytes, err := marshalBoundedCanonical(intent, maxIntentArtifactBytes, "onboarding intent")
	if err != nil {
		return CommittedGeneration{}, err
	}
	commit := Commit{
		SchemaVersion: 1, Kind: "mission-onboarding-commit", PublicationStamp: stamp, OnboardingPlanSHA256: planSHA256,
		MissionIntentSHA256: SHA256(missionBytes), IntentSHA256: SHA256(intentBytes),
	}
	commitBytes, err := marshalBoundedCanonical(commit, maxArtifactBytes, "onboarding commit")
	if err != nil {
		return CommittedGeneration{}, err
	}
	return CommittedGeneration{
		Identity: identity, ProjectBinding: binding, Intent: intent, Commit: commit,
		MissionIntentBytes: missionBytes, ProjectBindingBytes: bindingBytes, IntentBytes: intentBytes, CommitBytes: commitBytes,
	}, nil
}

// ValidateProjectedCurrentRecoveryAt validates a future .steamai generation
// while the selected project can still be legacy-only during migration preview.
func ValidateProjectedCurrentRecoveryAt(caseRoot string, identity Identity, recovery RecoveryEnvelope) error {
	physicalRoot, err := projectedPhysicalCaseRoot(caseRoot)
	if err != nil {
		return err
	}
	if identity.SchemaVersion != 2 || identity.Target != "." || !validProjectID(identity.ProjectID) {
		return fmt.Errorf("invalid projected current onboarding identity")
	}
	if err := validateIdentityFields(identity); err != nil {
		return err
	}
	if recovery.SchemaVersion != 1 || recovery.RepoRoot != "." {
		return fmt.Errorf("projected current onboarding recovery must be relocatable")
	}
	if _, err := time.Parse(time.RFC3339Nano, recovery.CreatedAt); err != nil {
		return fmt.Errorf("invalid projected onboarding recovery createdAt: %w", err)
	}
	projectedRoot := projectstate.Root{Dir: projectstate.CurrentDir, Path: filepath.Join(physicalRoot, projectstate.CurrentDir), Existing: true}
	return validateRecoveryEnvelope(projectedRoot, physicalRoot, identity, recovery)
}

func HashOnboardingPlan(value PlanHashInput) (string, error) {
	if value.SchemaVersion != 1 || strings.TrimSpace(value.Command) == "" || strings.TrimSpace(value.CaseRoot) == "" || strings.TrimSpace(value.RepoRoot) == "" || strings.TrimSpace(value.Pack) == "" || strings.TrimSpace(value.ProjectName) == "" || strings.TrimSpace(value.ProvisionID) == "" || strings.TrimSpace(value.Role) == "" || strings.TrimSpace(value.CreatedAt) == "" || len(value.Writes) == 0 {
		return "", fmt.Errorf("invalid onboarding plan hash input")
	}
	if value.LineageSHA256 != "" && !validSHA256(value.LineageSHA256) {
		return "", fmt.Errorf("invalid onboarding plan lineage hash")
	}
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return SHA256(data), nil
}

func DeriveProjectedProjectID(sourceCommitSHA256 string) (string, error) {
	if !validSHA256(sourceCommitSHA256) {
		return "", fmt.Errorf("projected current onboarding requires exact source commit sha256")
	}
	return SHA256([]byte("steamai-retired-onboarding-project-id-v1\x00" + strings.ToLower(sourceCommitSHA256)))[:16], nil
}

func ValidateIdentity(identity Identity) error {
	if identity.SchemaVersion != 1 {
		return fmt.Errorf("mission intent requires schemaVersion 1")
	}
	if identity.ProjectID != "" {
		return fmt.Errorf("mission intent schemaVersion 1 must not contain projectId")
	}
	if err := validateIdentityFields(identity); err != nil {
		return err
	}
	target, err := filepath.Abs(identity.Target)
	if err != nil || !samePath(target, identity.Target) {
		return fmt.Errorf("mission intent Target must be canonical absolute path: %s", identity.Target)
	}
	return nil
}

func ValidateIdentityAt(caseRoot string, identity Identity) error {
	physicalRoot, stateRoot, err := resolvePhysicalCaseRoot(caseRoot)
	if err != nil {
		return err
	}
	if identity.SchemaVersion == 1 {
		if err := ValidateIdentity(identity); err != nil {
			return err
		}
		if !samePath(identity.Target, physicalRoot) {
			return fmt.Errorf("mission intent Target does not match physical case root")
		}
		return nil
	}
	if identity.SchemaVersion != 2 {
		return fmt.Errorf("mission intent requires schemaVersion 1 or 2")
	}
	if stateRoot.Legacy {
		return fmt.Errorf("mission intent schemaVersion 2 requires current .steamai state root")
	}
	if identity.Target != "." {
		return fmt.Errorf("mission intent schemaVersion 2 Target must be logical current project root: .")
	}
	if !validProjectID(identity.ProjectID) {
		return fmt.Errorf("mission intent schemaVersion 2 requires a 16 lowercase hex projectId")
	}
	return validateIdentityFields(identity)
}

func ValidateProjectBindingAt(caseRoot string, value ProjectBinding) error {
	_, stateRoot, err := resolvePhysicalCaseRoot(caseRoot)
	if err != nil {
		return err
	}
	if stateRoot.Legacy {
		return fmt.Errorf("project binding requires current .steamai state root")
	}
	if value.SchemaVersion != 1 || !validProjectID(value.ProjectID) || value.Target != "." || !validSHA256(value.MissionIntentSHA256) || !validStamp(value.PublicationStamp) || !validPlanHash(value.OnboardingPlanSHA256) {
		return fmt.Errorf("invalid project binding identity")
	}
	if !value.NoAuthority || !value.NoConfirmed || !value.NoHeavyTool {
		return fmt.Errorf("project binding must preserve no-authority, no-confirmed, and no-heavy boundaries")
	}
	return nil
}

func ValidateIntentAt(caseRoot string, value Intent) error {
	physicalRoot, _, err := resolvePhysicalCaseRoot(caseRoot)
	if err != nil {
		return err
	}
	if value.SchemaVersion == 1 {
		if err := ValidateIdentityAt(physicalRoot, value.Identity); err != nil {
			return err
		}
		return validateLegacyIntent(value)
	}
	if value.SchemaVersion != 2 || value.Kind != "mission-onboarding-intent" || !validStamp(value.PublicationStamp) || !validPlanHash(value.OnboardingPlanSHA256) || !validSHA256(value.ProjectBindingSHA256) {
		return fmt.Errorf("invalid onboarding intent identity")
	}
	if err := ValidateIdentityAt(physicalRoot, value.Identity); err != nil {
		return err
	}
	return ValidateRecoveryEnvelopeAt(physicalRoot, value.Identity, value.Recovery)
}

func ValidateCommitAt(caseRoot string, value Commit) error {
	_, stateRoot, err := resolvePhysicalCaseRoot(caseRoot)
	if err != nil {
		return err
	}
	if stateRoot.Legacy {
		return fmt.Errorf("v2 onboarding commit requires current .steamai state root")
	}
	return validateCommit(value)
}

func GenerateProjectID() (string, error) {
	var value [8]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}

func validateLegacyIntent(value Intent) error {
	if value.SchemaVersion != 1 || value.Kind != "mission-onboarding-intent" || !validStamp(value.PublicationStamp) || !validPlanHash(value.OnboardingPlanSHA256) || value.ProjectBindingSHA256 != "" {
		return fmt.Errorf("invalid onboarding intent identity")
	}
	if err := ValidateIdentity(value.Identity); err != nil {
		return err
	}
	return ValidateRecoveryEnvelope(value.Identity, value.Recovery)
}

func validateCommit(value Commit) error {
	if value.SchemaVersion != 1 || value.Kind != "mission-onboarding-commit" || !validStamp(value.PublicationStamp) || !validSHA256(value.OnboardingPlanSHA256) || !validSHA256(value.MissionIntentSHA256) || !validSHA256(value.IntentSHA256) {
		return fmt.Errorf("invalid onboarding commit identity")
	}
	return nil
}

func validateIdentityFields(identity Identity) error {
	fields := map[string]string{
		"Target": identity.Target, "Pack": identity.Pack, "ProjectName": identity.ProjectName,
		"Goal": identity.Goal, "Actor": identity.Actor, "Executor": identity.Executor, "InitialLane": identity.InitialLane,
	}
	for name, value := range fields {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("mission intent requires %s", name)
		}
	}
	return nil
}

func projectedPhysicalCaseRoot(caseRoot string) (string, error) {
	if strings.TrimSpace(caseRoot) == "" {
		return "", fmt.Errorf("physical case root is empty")
	}
	physicalRoot, err := filepath.Abs(caseRoot)
	if err != nil {
		return "", err
	}
	physicalRoot = filepath.Clean(physicalRoot)
	current := physicalRoot
	for {
		info, statErr := os.Lstat(current)
		if statErr == nil {
			if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return "", fmt.Errorf("physical case root existing ancestor is not a regular directory: %s", current)
			}
			if err := validateProjectedPathComponents(current); err != nil {
				return "", err
			}
			return physicalRoot, nil
		}
		if !os.IsNotExist(statErr) {
			return "", statErr
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("physical case root has no existing directory ancestor: %s", physicalRoot)
		}
		current = parent
	}
}

func validateProjectedPathComponents(path string) error {
	current := filepath.Clean(path)
	for {
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("physical case root contains a non-directory or reparse component: %s", current)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return nil
		}
		current = parent
	}
}

func resolvePhysicalCaseRoot(caseRoot string) (string, projectstate.Root, error) {
	if strings.TrimSpace(caseRoot) == "" {
		return "", projectstate.Root{}, fmt.Errorf("physical case root is empty")
	}
	physicalRoot, err := filepath.Abs(caseRoot)
	if err != nil {
		return "", projectstate.Root{}, err
	}
	physicalRoot = filepath.Clean(physicalRoot)
	stateRoot, err := projectstate.Resolve(physicalRoot)
	if err != nil {
		return "", projectstate.Root{}, err
	}
	return physicalRoot, stateRoot, nil
}

func marshalBoundedCanonical(value any, limit int, label string) ([]byte, error) {
	data, err := marshalCanonical(value)
	if err != nil {
		return nil, err
	}
	if len(data) > limit {
		return nil, fmt.Errorf("canonical %s exceeds %d bytes", label, limit)
	}
	return data, nil
}

func ValidateRecoveryEnvelope(identity Identity, envelope RecoveryEnvelope) error {
	if identity.SchemaVersion == 2 {
		return fmt.Errorf("mission intent schemaVersion 2 recovery validation requires an explicit physical case root")
	}
	stateRoot, err := projectstate.Resolve(identity.Target)
	if err != nil {
		return err
	}
	return validateRecoveryEnvelope(stateRoot, identity.Target, identity, envelope)
}

func ValidateRecoveryEnvelopeAt(caseRoot string, identity Identity, envelope RecoveryEnvelope) error {
	physicalRoot, stateRoot, err := resolvePhysicalCaseRoot(caseRoot)
	if err != nil {
		return err
	}
	if err := ValidateIdentityAt(physicalRoot, identity); err != nil {
		return err
	}
	if identity.SchemaVersion == 2 {
		envelope, err = canonicalizeRecoveryEnvelope(physicalRoot, identity, envelope)
		if err != nil {
			return err
		}
	}
	return validateRecoveryEnvelope(stateRoot, physicalRoot, identity, envelope)
}

func validateRecoveryEnvelope(stateRoot projectstate.Root, physicalRoot string, identity Identity, envelope RecoveryEnvelope) error {
	if envelope.SchemaVersion != 1 || strings.TrimSpace(envelope.RepoRoot) == "" || strings.TrimSpace(envelope.CreatedAt) == "" {
		return fmt.Errorf("onboarding recovery envelope is missing stable identity")
	}
	if _, err := time.Parse(time.RFC3339Nano, envelope.CreatedAt); err != nil {
		return fmt.Errorf("invalid onboarding recovery createdAt: %w", err)
	}
	if envelope.Mode == "attached-adoption" {
		return validateAttachedSnapshot(stateRoot, envelope, identity.SchemaVersion == 2)
	}
	if envelope.Mode != "" || len(envelope.AttachedSnapshot) != 0 {
		return fmt.Errorf("onboarding recovery envelope has an unsupported mode")
	}
	if stateRoot.Legacy {
		if !filepath.IsAbs(envelope.RepoRoot) {
			return fmt.Errorf("legacy onboarding recovery repoRoot must be absolute")
		}
		if envelope.BundleManifest != (BundleBinding{}) {
			return fmt.Errorf("legacy onboarding recovery must not bind a project-local runtime bundle")
		}
	} else {
		if envelope.RepoRoot != "." {
			return fmt.Errorf("current onboarding recovery repoRoot must be relocatable")
		}
		if envelope.BundleManifest.Path != runtimebundle.ManifestRel || !validSHA256(envelope.BundleManifest.SHA256) || envelope.BundleManifest.Files < 1 || envelope.BundleManifest.Files > maxRecoveryWrites {
			return fmt.Errorf("current onboarding recovery requires a bounded runtime bundle manifest binding")
		}
	}
	if len(envelope.Writes) == 0 || len(envelope.Writes) > maxRecoveryWrites {
		return fmt.Errorf("onboarding recovery write count is outside bounds")
	}
	seen := map[string]struct{}{}
	required := recoveryRequiredPaths(stateRoot, false)
	total := 0
	lastPhase := -1
	lastPath := ""
	for _, write := range envelope.Writes {
		rel, key, err := validateRecoveryPath(write.Path)
		if err != nil {
			return err
		}
		if _, ok := seen[key]; ok {
			return fmt.Errorf("duplicate onboarding recovery path: %s", rel)
		}
		seen[key] = struct{}{}
		if write.PublicationPhase != 1 {
			return fmt.Errorf("onboarding recovery ordinary write must use publication phase 1: %s", rel)
		}
		if identity.SchemaVersion == 2 && isCurrentGeneratedArtifact(key) {
			return fmt.Errorf("onboarding recovery envelope must not embed generated artifact: %s", rel)
		}
		if err := validateRecoveryWrite(stateRoot, physicalRoot, identity, envelope, key, write); err != nil {
			return err
		}
		if strings.TrimSpace(write.Kind) == "" || write.PublicationPhase < lastPhase || (write.PublicationPhase == lastPhase && lastPath != "" && write.Path <= lastPath) {
			return fmt.Errorf("onboarding recovery writes are not ordered: %s", rel)
		}
		lastPhase, lastPath = write.PublicationPhase, write.Path
		if write.Kind == "runtime-executable" {
			if len(write.Content) != 0 || write.Size < 1 || !validSHA256(write.SHA256) {
				return fmt.Errorf("onboarding recovery executable source binding is invalid: %s", rel)
			}
		} else {
			sum := SHA256(write.Content)
			if write.Size != int64(len(write.Content)) || !strings.EqualFold(write.SHA256, sum) {
				return fmt.Errorf("onboarding recovery content binding mismatch: %s", rel)
			}
			total += len(write.Content)
		}
		if total > maxRecoveryBytes {
			return fmt.Errorf("onboarding recovery content exceeds %d bytes", maxRecoveryBytes)
		}
	}
	for path, kind := range required {
		if _, ok := seen[path]; !ok {
			return fmt.Errorf("onboarding recovery is missing required %s write: %s", kind, path)
		}
	}
	for path := range caseshim.PackRecoveryWrites(identity.Pack) {
		if _, ok := seen[strings.ToLower(path)]; !ok {
			return fmt.Errorf("onboarding recovery is missing trusted pack write: %s", path)
		}
	}
	if _, ok := seen["claude.local.md"]; !ok {
		return fmt.Errorf("onboarding recovery is missing required managed-block write: CLAUDE.local.md")
	}
	for _, path := range caseshim.ExpectedSupportPaths(identity.Pack) {
		if _, ok := seen[strings.ToLower(path)]; !ok {
			return fmt.Errorf("onboarding recovery is missing required support write: %s", path)
		}
	}
	if !stateRoot.Legacy {
		if err := validateBundleRecoveryInventory(identity, envelope, seen); err != nil {
			return err
		}
	}
	return nil
}

func validateBundleRecoveryInventory(identity Identity, envelope RecoveryEnvelope, _ map[string]struct{}) error {
	manifestPath := filepath.ToSlash(filepath.Join(projectstate.CurrentDir, runtimebundle.ManifestRel))
	manifestWrite, ok := recoveryWriteByPath(envelope.Writes, manifestPath)
	if !ok || manifestWrite.Kind != "runtime-bundle-manifest" || !strings.EqualFold(manifestWrite.SHA256, envelope.BundleManifest.SHA256) {
		return fmt.Errorf("onboarding recovery is missing its exact runtime bundle manifest write")
	}
	manifest, err := runtimebundle.ValidateManifestData(manifestWrite.Content, envelope.BundleManifest.SHA256, identity.Pack)
	if err != nil {
		return fmt.Errorf("onboarding recovery runtime bundle manifest is invalid: %w", err)
	}
	if envelope.BundleManifest.Files != len(manifest.Files)+1 {
		return fmt.Errorf("onboarding recovery runtime bundle file count differs from manifest")
	}
	expected := map[string]runtimebundle.Artifact{}
	executablePath := filepath.ToSlash(filepath.Join(projectstate.CurrentDir, filepath.FromSlash(manifest.Executable.Path)))
	expected[strings.ToLower(executablePath)] = manifest.Executable
	for _, artifact := range manifest.Files {
		path := filepath.ToSlash(filepath.Join(projectstate.CurrentDir, filepath.FromSlash(artifact.Path)))
		expected[strings.ToLower(path)] = artifact
	}
	for key, artifact := range expected {
		write, ok := recoveryWriteByKey(envelope.Writes, key)
		if !ok || write.Path != canonicalBundleRecoveryPath(artifact.Path) || write.Kind != artifact.Kind || write.Size != artifact.Size || !strings.EqualFold(write.SHA256, artifact.SHA256) {
			return fmt.Errorf("onboarding recovery bundle inventory differs from manifest: %s", artifact.Path)
		}
		if artifact.Kind == "runtime-executable" {
			if len(write.Content) != 0 {
				return fmt.Errorf("onboarding recovery must not embed the runtime executable")
			}
		} else if int64(len(write.Content)) != artifact.Size || !strings.EqualFold(SHA256(write.Content), artifact.SHA256) {
			return fmt.Errorf("onboarding recovery embedded bundle asset differs from manifest: %s", artifact.Path)
		}
	}
	for _, write := range envelope.Writes {
		if !isBundleRecoveryKind(write.Kind) || write.Kind == "runtime-bundle-manifest" {
			continue
		}
		if _, ok := expected[strings.ToLower(write.Path)]; !ok {
			return fmt.Errorf("onboarding recovery contains bundle asset absent from manifest: %s", write.Path)
		}
	}
	return nil
}

func canonicalBundleRecoveryPath(path string) string {
	return filepath.ToSlash(filepath.Join(projectstate.CurrentDir, filepath.FromSlash(path)))
}

func recoveryWriteByPath(writes []RecoveryWrite, path string) (RecoveryWrite, bool) {
	return recoveryWriteByKey(writes, strings.ToLower(path))
}

func recoveryWriteByKey(writes []RecoveryWrite, key string) (RecoveryWrite, bool) {
	for _, write := range writes {
		if strings.EqualFold(write.Path, key) {
			return write, true
		}
	}
	return RecoveryWrite{}, false
}

func validateAttachedSnapshot(stateRoot projectstate.Root, envelope RecoveryEnvelope, requireRelocatable bool) error {
	if len(envelope.Writes) != 0 || len(envelope.AttachedSnapshot) == 0 || len(envelope.AttachedSnapshot) > maxRecoveryWrites {
		return fmt.Errorf("attached onboarding recovery snapshot count is outside bounds")
	}
	if requireRelocatable && envelope.RepoRoot != "." {
		return fmt.Errorf("current schemaVersion 2 attached onboarding recovery repoRoot must be relocatable")
	}
	if requireRelocatable && envelope.BundleManifest != (BundleBinding{}) {
		return fmt.Errorf("current schemaVersion 2 attached onboarding recovery must not contain a runtime bundle binding")
	}
	required := recoveryRequiredPaths(stateRoot, true)
	seen := map[string]struct{}{}
	var total int64
	lastPath := ""
	for _, artifact := range envelope.AttachedSnapshot {
		rel, key, err := validateRecoveryPath(artifact.Path)
		if err != nil {
			return err
		}
		if requireRelocatable && isCurrentGeneratedArtifact(key) {
			return fmt.Errorf("onboarding recovery envelope must not embed generated artifact: %s", rel)
		}
		if _, ok := seen[key]; ok {
			return fmt.Errorf("duplicate attached onboarding snapshot path: %s", rel)
		}
		seen[key] = struct{}{}
		if artifact.Path <= lastPath {
			return fmt.Errorf("attached onboarding snapshot is not ordered: %s", rel)
		}
		fixedKinds := recoveryRequiredPaths(stateRoot, true)
		if expected, fixed := fixedKinds[key]; fixed {
			if artifact.Path != canonicalRecoveryPath(stateRoot, key) || artifact.Kind != expected {
				return fmt.Errorf("attached onboarding snapshot fixed artifact %s requires kind %s and canonical path casing", artifact.Path, expected)
			}
		} else if isRecoveryControlPath(key) || artifact.Kind != "doctor-validated-artifact" {
			return fmt.Errorf("attached onboarding snapshot rejects artifact kind or runtime/control namespace: %s", artifact.Path)
		}
		if artifact.Size < 1 || artifact.Size > maxRecoveryBytes || !validSHA256(artifact.SHA256) {
			return fmt.Errorf("attached onboarding snapshot binding is invalid: %s", rel)
		}
		total += artifact.Size
		if total > maxRecoveryBytes {
			return fmt.Errorf("attached onboarding snapshot exceeds %d bytes", maxRecoveryBytes)
		}
		lastPath = artifact.Path
	}
	for path, kind := range required {
		if _, ok := seen[path]; !ok {
			return fmt.Errorf("attached onboarding snapshot is missing required %s artifact: %s", kind, path)
		}
	}
	return nil
}

func validateRecoveryPath(value string) (string, string, error) {
	rel := filepath.ToSlash(filepath.Clean(filepath.FromSlash(strings.TrimSpace(value))))
	if rel == "" || rel == "." || filepath.IsAbs(filepath.FromSlash(rel)) || rel == ".." || strings.HasPrefix(rel, "../") || rel != value {
		return "", "", fmt.Errorf("invalid onboarding recovery path: %q", value)
	}
	key := strings.ToLower(rel)
	if key == strings.ToLower(filepath.ToSlash(IntentRel)) || key == strings.ToLower(filepath.ToSlash(MissionIntentRel)) || key == strings.ToLower(filepath.ToSlash(CommitRel)) {
		return "", "", fmt.Errorf("onboarding recovery envelope must not embed generated artifact: %s", rel)
	}
	return rel, key, nil
}

func isCurrentGeneratedArtifact(key string) bool {
	for _, path := range []string{
		filepath.ToSlash(filepath.Join(projectstate.CurrentDir, "mission-intent.json")),
		ProjectBindingRel,
		filepath.ToSlash(filepath.Join(projectstate.CurrentDir, "onboarding", "intent.json")),
		filepath.ToSlash(filepath.Join(projectstate.CurrentDir, "onboarding", "commit.json")),
	} {
		if strings.EqualFold(key, path) {
			return true
		}
	}
	return false
}

func isBundleRecoveryKind(kind string) bool {
	switch strings.TrimSpace(kind) {
	case "runtime-executable", "pack-asset", "common-asset", "runtime-asset", "runtime-bundle-manifest":
		return true
	default:
		return false
	}
}

func validateRecoveryWrite(stateRoot projectstate.Root, physicalRoot string, identity Identity, envelope RecoveryEnvelope, key string, write RecoveryWrite) error {
	statePrefix := strings.ToLower(filepath.ToSlash(stateRoot.Dir))
	fixedKinds := recoveryRequiredPaths(stateRoot, false)
	fixedKinds[".gitignore"] = "support-file"
	if expected, ok := fixedKinds[key]; ok {
		if write.Path != canonicalRecoveryPath(stateRoot, key) {
			return fmt.Errorf("onboarding recovery fixed artifact requires canonical path casing: %s", write.Path)
		}
		if write.Kind != expected {
			return fmt.Errorf("onboarding recovery write %s requires kind %s", write.Path, expected)
		}
	}
	if isRecoveryControlPath(key) && !(strings.HasPrefix(key, strings.ToLower(projectstate.CurrentDir)+"/") && (isBundleRecoveryKind(write.Kind) || write.Kind == "runtime-bundle-manifest")) {
		return fmt.Errorf("onboarding recovery rejects runtime/control namespace: %s", write.Path)
	}
	switch key {
	case statePrefix + "/instance.yml":
		var expected string
		if stateRoot.Legacy {
			expected = "schemaVersion: 1\n" +
				"templateRoot: " + envelope.RepoRoot + "\n" +
				"templatePack: " + identity.Pack + "\n" +
				"projectName: " + identity.ProjectName + "\n" +
				"projectRoot: " + physicalRoot + "\n" +
				"mode: case-local-shim\n"
		} else {
			expected = "schemaVersion: 2\n" +
				"brand: STeamAI\n" +
				"stateNamespace: steamai\n" +
				"templateRoot: .\n" +
				"bundleRoot: runtime\n" +
				"bundleManifest: " + runtimebundle.ManifestRel + "\n" +
				"bundleManifestSHA256: " + strings.ToLower(envelope.BundleManifest.SHA256) + "\n" +
				"templatePack: " + identity.Pack + "\n" +
				"projectName: " + identity.ProjectName + "\n" +
				"projectRoot: ..\n" +
				"mode: project-local-bundle\n"
		}
		if string(write.Content) != expected {
			return fmt.Errorf("onboarding recovery instance metadata does not bind identity/repo/pack")
		}
	case ".re-template.yml":
		expected := "templateRoot: " + envelope.RepoRoot + "\r\n" +
			"rekitMode: case-local-shim\r\n" +
			"templatePack: " + identity.Pack + "\r\n" +
			"templateVersion: 0.0.0\r\n"
		if string(write.Content) != expected {
			return fmt.Errorf("onboarding recovery legacy metadata does not bind repo/pack")
		}
	case ".claude/skills/rekit/skill.md":
		if err := caseshim.ValidateSHA256(SHA256(write.Content)); err != nil {
			return fmt.Errorf("onboarding recovery %w", err)
		}
	case ".claude/skills/steamai/skill.md":
		if len(write.Content) == 0 {
			return fmt.Errorf("onboarding recovery project-local STeamAI skill is empty")
		}
	case statePrefix + "/state.json":
		var state struct {
			SchemaVersion int    `json:"schemaVersion"`
			TemplateRoot  string `json:"templateRoot"`
			TemplatePack  string `json:"templatePack"`
			LastSyncAt    string `json:"lastSyncAt"`
			Managed       map[string]struct {
				SourceHash       string `json:"sourceHash"`
				TargetHashAtSync string `json:"targetHashAtSync"`
				LastAction       string `json:"lastAction"`
			} `json:"managed"`
		}
		if err := decodeCanonical(write.Content, &state); err != nil || state.SchemaVersion != 1 || state.TemplatePack != identity.Pack || state.LastSyncAt != envelope.CreatedAt {
			return fmt.Errorf("onboarding recovery initial state does not bind repo/pack/generation")
		}
		if stateRoot.Legacy {
			if !samePath(state.TemplateRoot, envelope.RepoRoot) {
				return fmt.Errorf("onboarding recovery initial state does not bind legacy repo root")
			}
		} else if state.TemplateRoot != "." {
			return fmt.Errorf("onboarding recovery current initial state must use relocatable templateRoot")
		}
		expectedManaged := map[string]string{}
		for _, candidate := range envelope.Writes {
			if candidate.Kind == "managed-file" {
				expectedManaged[candidate.Path] = candidate.SHA256
			}
		}
		if len(state.Managed) != len(expectedManaged) {
			return fmt.Errorf("onboarding recovery initial state managed inventory differs from recovery writes")
		}
		for path, hash := range expectedManaged {
			entry, ok := state.Managed[path]
			if !ok || !strings.EqualFold(entry.SourceHash, hash) || !strings.EqualFold(entry.TargetHashAtSync, hash) || entry.LastAction != "sync" {
				return fmt.Errorf("onboarding recovery initial state managed entry differs from recovery write: %s", path)
			}
		}
	default:
		switch write.Kind {
		case "runtime-executable":
			if stateRoot.Legacy || len(write.Content) != 0 || !strings.HasPrefix(key, strings.ToLower(projectstate.CurrentDir)+"/") {
				return fmt.Errorf("onboarding recovery executable source binding is invalid: %s", write.Path)
			}
		case "pack-asset", "common-asset", "runtime-asset":
			if stateRoot.Legacy || len(write.Content) == 0 || !strings.HasPrefix(key, strings.ToLower(projectstate.CurrentDir)+"/") {
				return fmt.Errorf("onboarding recovery embedded bundle asset is invalid: %s", write.Path)
			}
		case "runtime-bundle-manifest":
			if stateRoot.Legacy || key != strings.ToLower(filepath.ToSlash(filepath.Join(projectstate.CurrentDir, runtimebundle.ManifestRel))) || SHA256(write.Content) != envelope.BundleManifest.SHA256 {
				return fmt.Errorf("onboarding recovery runtime manifest differs from its bundle binding")
			}
		case "managed-file", "template-file":
			normalized := append([]byte{}, write.Content...)
			if write.Kind == "template-file" {
				if identity.SchemaVersion == 1 {
					normalized = []byte(strings.ReplaceAll(strings.ReplaceAll(string(normalized), identity.Target, "<PROJECT_ROOT>"), identity.ProjectName, "<PROJECT_NAME>"))
				} else {
					var err error
					normalized, err = canonicalTemplateRecoveryContent(normalized, physicalRoot, identity.ProjectName)
					if err != nil {
						return fmt.Errorf("onboarding recovery template identity binding is invalid: %s: %w", write.Path, err)
					}
					if !bytes.Equal(normalized, write.Content) {
						return fmt.Errorf("onboarding recovery template must use canonical project markers: %s", write.Path)
					}
				}
			}
			if err := caseshim.ValidatePackRecoveryWrite(identity.Pack, write.Path, write.Kind, SHA256(normalized)); err != nil {
				return fmt.Errorf("onboarding recovery %s: %w", write.Path, err)
			}
		case "managed-block":
			if key != "claude.local.md" {
				return fmt.Errorf("onboarding recovery managed block path is not allowed: %s", write.Path)
			}
			if err := caseshim.ValidateManagedBlockSHA256(identity.Pack, SHA256(write.Content)); err != nil {
				return fmt.Errorf("onboarding recovery %w", err)
			}
		case "support-file":
			if key != ".gitignore" {
				return fmt.Errorf("onboarding recovery support path is not allowed: %s", write.Path)
			}
			if err := caseshim.ValidateSupportSHA256(identity.Pack, key, SHA256(write.Content)); err != nil {
				return fmt.Errorf("onboarding recovery %w", err)
			}
		default:
			return fmt.Errorf("onboarding recovery rejects unrecognized write kind %q for %s", write.Kind, write.Path)
		}
	}
	return nil
}

func canonicalizeRecoveryEnvelope(physicalRoot string, identity Identity, envelope RecoveryEnvelope) (RecoveryEnvelope, error) {
	if identity.SchemaVersion != 2 || envelope.Mode == "attached-adoption" {
		return envelope, nil
	}
	canonical := envelope
	canonical.Writes = append([]RecoveryWrite{}, envelope.Writes...)
	for index := range canonical.Writes {
		write, err := canonicalRecoveryWriteAt(physicalRoot, identity, canonical.Writes[index])
		if err != nil {
			return RecoveryEnvelope{}, err
		}
		canonical.Writes[index] = write
	}
	return canonical, nil
}

// CanonicalRecoveryWriteAt converts only explicit template identity bindings
// to their durable placeholder representation. Other bytes are copied exactly.
func CanonicalRecoveryWriteAt(caseRoot string, identity Identity, write RecoveryWrite) (RecoveryWrite, error) {
	physicalRoot, _, err := resolvePhysicalCaseRoot(caseRoot)
	if err != nil {
		return RecoveryWrite{}, err
	}
	if err := ValidateIdentityAt(physicalRoot, identity); err != nil {
		return RecoveryWrite{}, err
	}
	return canonicalRecoveryWriteAt(physicalRoot, identity, write)
}

func canonicalRecoveryWriteAt(physicalRoot string, identity Identity, write RecoveryWrite) (RecoveryWrite, error) {
	canonical := write
	canonical.Content = append([]byte{}, write.Content...)
	if identity.SchemaVersion != 2 || write.Kind != "template-file" {
		return canonical, nil
	}
	if write.Size != int64(len(write.Content)) || !strings.EqualFold(write.SHA256, SHA256(write.Content)) {
		return RecoveryWrite{}, fmt.Errorf("onboarding recovery content binding mismatch: %s", write.Path)
	}
	content, err := canonicalTemplateRecoveryWrite(physicalRoot, identity, write)
	if err != nil {
		return RecoveryWrite{}, fmt.Errorf("onboarding recovery template identity binding is invalid: %s: %w", write.Path, err)
	}
	canonical.Content = content
	canonical.Size = int64(len(content))
	canonical.SHA256 = SHA256(content)
	return canonical, nil
}

// MaterializeRecoveryWriteAt expands only the two explicit project template
// markers for the supplied physical root. It never rewrites arbitrary prose.
func MaterializeRecoveryWriteAt(caseRoot string, identity Identity, write RecoveryWrite) (RecoveryWrite, error) {
	physicalRoot, _, err := resolvePhysicalCaseRoot(caseRoot)
	if err != nil {
		return RecoveryWrite{}, err
	}
	if err := ValidateIdentityAt(physicalRoot, identity); err != nil {
		return RecoveryWrite{}, err
	}
	materialized := write
	materialized.Content = append([]byte{}, write.Content...)
	if identity.SchemaVersion != 2 || write.Kind != "template-file" {
		return materialized, nil
	}
	canonical, err := canonicalRecoveryWriteAt(physicalRoot, identity, write)
	if err != nil {
		return RecoveryWrite{}, err
	}
	text := string(canonical.Content)
	if strings.Count(text, "<PROJECT_ROOT>") != 1 || strings.Count(text, "<PROJECT_NAME>") != 1 {
		return RecoveryWrite{}, fmt.Errorf("onboarding recovery template markers are absent or ambiguous: %s", write.Path)
	}
	text = strings.Replace(text, "<PROJECT_ROOT>", physicalRoot, 1)
	text = strings.Replace(text, "<PROJECT_NAME>", identity.ProjectName, 1)
	materialized.Content = []byte(text)
	materialized.Size = int64(len(materialized.Content))
	materialized.SHA256 = SHA256(materialized.Content)
	return materialized, nil
}

func canonicalTemplateRecoveryWrite(physicalRoot string, identity Identity, write RecoveryWrite) ([]byte, error) {
	content, err := canonicalTemplateRecoveryContent(write.Content, physicalRoot, identity.ProjectName)
	if err != nil {
		return nil, err
	}
	if err := caseshim.ValidatePackRecoveryWrite(identity.Pack, write.Path, write.Kind, SHA256(content)); err != nil {
		return nil, err
	}
	return content, nil
}

func canonicalTemplateRecoveryContent(content []byte, physicalRoot, projectName string) ([]byte, error) {
	text := string(content)
	var err error
	text, err = replaceOneTemplateBinding(text, "<PROJECT_ROOT>", physicalRoot, "physical project root")
	if err != nil {
		return nil, err
	}
	text, err = replaceOneTemplateBinding(text, "<PROJECT_NAME>", projectName, "project name")
	if err != nil {
		return nil, err
	}
	return []byte(text), nil
}

func replaceOneTemplateBinding(text, marker, materialized, label string) (string, error) {
	switch strings.Count(text, marker) {
	case 1:
		return text, nil
	case 0:
		if materialized == "" || strings.Count(text, materialized) != 1 {
			return "", fmt.Errorf("%s binding is absent or ambiguous", label)
		}
		return strings.Replace(text, materialized, marker, 1), nil
	default:
		return "", fmt.Errorf("%s marker is ambiguous", label)
	}
}

func recoveryRequiredPaths(stateRoot projectstate.Root, attached bool) map[string]string {
	statePrefix := strings.ToLower(filepath.ToSlash(stateRoot.Dir))
	instanceKind, stateKind := "instance-metadata", "initial-state"
	skillPath, skillKind := ".claude/skills/steamai/skill.md", "project-local-steamai-skill"
	required := map[string]string{
		statePrefix + "/instance.yml": instanceKind,
		statePrefix + "/state.json":   stateKind,
		skillPath:                     skillKind,
	}
	if stateRoot.Legacy {
		required[".claude/skills/rekit/skill.md"] = "case-local-thin-shim"
		required[".re-template.yml"] = "legacy-metadata"
		delete(required, skillPath)
	}
	if attached {
		required[statePrefix+"/state.json"] = "sync-state"
	}
	return required
}

func canonicalRecoveryPath(stateRoot projectstate.Root, key string) string {
	statePrefix := strings.ToLower(filepath.ToSlash(stateRoot.Dir))
	return map[string]string{
		statePrefix + "/instance.yml":     filepath.ToSlash(filepath.Join(stateRoot.Dir, "instance.yml")),
		statePrefix + "/state.json":       filepath.ToSlash(filepath.Join(stateRoot.Dir, "state.json")),
		".claude/skills/rekit/skill.md":   ".claude/skills/rekit/SKILL.md",
		".claude/skills/steamai/skill.md": ".claude/skills/steamai/SKILL.md",
		".re-template.yml":                ".re-template.yml",
		".gitignore":                      ".gitignore",
	}[key]
}

func isRecoveryControlPath(key string) bool {
	if key == ".rekit/instance.yml" || key == ".rekit/state.json" || key == ".steamai/instance.yml" || key == ".steamai/state.json" || key == ".re-template.yml" || key == ".claude/skills/rekit/skill.md" || key == ".claude/skills/steamai/skill.md" {
		return false
	}
	for _, prefix := range []string{".rekit/", ".steamai/", ".claude/skills/", "lanes/", "board/", "facts/", "gate/", "ledger/", "authority/", "confirmed/"} {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	for segment := range strings.SplitSeq(key, "/") {
		switch segment {
		case "authority", "confirmed", "facts", "fact", "gate", "gates", "ledger", "ledgers", "lane", "lanes", "board", "boards", "evidence":
			return true
		}
	}
	return false
}

var inspectPinnedHook func(caseRoot string) error

// SetInspectPinnedHookForTest installs a deterministic race seam after the
// case root has been pinned and before onboarding artifacts are read.
func SetInspectPinnedHookForTest(hook func(caseRoot string) error) func() {
	previous := inspectPinnedHook
	inspectPinnedHook = hook
	return func() { inspectPinnedHook = previous }
}

func Inspect(caseRoot string) (inspection Inspection, resultErr error) {
	caseRoot, stateRoot, err := resolvePhysicalCaseRoot(caseRoot)
	if err != nil {
		return Inspection{State: "corrupt"}, err
	}
	paths := artifactPaths(stateRoot)
	root, rootInfo, present, err := openPinnedCaseRoot(caseRoot)
	if err != nil {
		return Inspection{State: "corrupt"}, err
	}
	if !present {
		return Inspection{State: "absent"}, nil
	}
	stateNamespaceRoot, stateNamespaceInfo, stateNamespacePresent, err := openPinnedChildRoot(root, caseRoot, stateRoot.Dir)
	if err != nil {
		_ = root.Close()
		return Inspection{State: "corrupt"}, err
	}
	var onboardingRoot *os.Root
	var onboardingInfo os.FileInfo
	if stateNamespacePresent {
		onboardingRoot, onboardingInfo, _, err = openPinnedChildRoot(stateNamespaceRoot, stateRoot.Path, "onboarding")
		if err != nil {
			_ = stateNamespaceRoot.Close()
			_ = root.Close()
			return Inspection{State: "corrupt"}, err
		}
	}
	defer func() {
		bindingErr := validateInspectionNamespaceBindings(caseRoot, stateRoot, root, rootInfo, stateNamespaceRoot, stateNamespaceInfo, onboardingRoot, onboardingInfo)
		if onboardingRoot != nil {
			_ = onboardingRoot.Close()
		}
		if stateNamespaceRoot != nil {
			_ = stateNamespaceRoot.Close()
		}
		_ = root.Close()
		if bindingErr != nil {
			inspection = Inspection{State: "corrupt"}
			resultErr = bindingErr
		}
	}()
	if inspectPinnedHook != nil {
		if err := inspectPinnedHook(caseRoot); err != nil {
			return Inspection{State: "corrupt"}, err
		}
	}
	if err := validateInspectionNamespaceBindings(caseRoot, stateRoot, root, rootInfo, stateNamespaceRoot, stateNamespaceInfo, onboardingRoot, onboardingInfo); err != nil {
		return Inspection{State: "corrupt"}, err
	}
	var missionBytes, bindingBytes, intentBytes, commitBytes []byte
	var missionPresent, bindingPresent, intentPresent, commitPresent bool
	if stateNamespaceRoot != nil {
		missionBytes, missionPresent, err = readStrictPinned(stateNamespaceRoot, stateRoot.Path, "mission-intent.json", paths.MissionIntent, maxArtifactBytes)
		if err != nil {
			return Inspection{State: "corrupt"}, err
		}
		if !stateRoot.Legacy {
			bindingBytes, bindingPresent, err = readStrictPinned(stateNamespaceRoot, stateRoot.Path, "project-binding.json", paths.ProjectBinding, maxArtifactBytes)
			if err != nil {
				return Inspection{State: "corrupt"}, err
			}
		}
	}
	if onboardingRoot != nil {
		onboardingPath := filepath.Join(stateRoot.Path, "onboarding")
		intentBytes, intentPresent, err = readStrictPinned(onboardingRoot, onboardingPath, "intent.json", paths.Intent, maxIntentArtifactBytes)
		if err != nil {
			return Inspection{State: "corrupt"}, err
		}
		commitBytes, commitPresent, err = readStrictPinned(onboardingRoot, onboardingPath, "commit.json", paths.Commit, maxArtifactBytes)
		if err != nil {
			return Inspection{State: "corrupt"}, err
		}
	}
	if !missionPresent && !bindingPresent && !intentPresent && !commitPresent {
		return Inspection{State: "absent"}, nil
	}
	if !intentPresent {
		return Inspection{State: "corrupt"}, fmt.Errorf("onboarding intent is missing")
	}
	var intent Intent
	if err := decodeCanonical(intentBytes, &intent); err != nil {
		return Inspection{State: "corrupt"}, fmt.Errorf("invalid onboarding intent: %w", err)
	}
	if intent.SchemaVersion == 1 {
		return inspectV1(caseRoot, stateRoot, missionBytes, missionPresent, bindingPresent, intentBytes, intent, commitBytes, commitPresent)
	}
	inspection, err = inspectV2(caseRoot, stateRoot, missionBytes, missionPresent, bindingBytes, bindingPresent, intentBytes, intent, commitBytes, commitPresent)
	if err != nil || !inspection.Committed || stateRoot.Legacy {
		return inspection, err
	}
	view, err := projectstate.ResolveMissionView(caseRoot)
	if err != nil {
		return Inspection{State: "corrupt"}, err
	}
	if view.Generation == 1 {
		return inspection, nil
	}
	activeMissionPath := filepath.Join(view.Path, "mission-intent.json")
	activeMissionBytes, err := rekitfs.ReadStableRegularFileAnchored(view.Path, activeMissionPath, "active successor mission intent", maxArtifactBytes)
	if err != nil {
		return Inspection{State: "corrupt"}, err
	}
	var activeIdentity Identity
	if err := decodeCanonical(activeMissionBytes, &activeIdentity); err != nil {
		return Inspection{State: "corrupt"}, fmt.Errorf("invalid active successor mission intent: %w", err)
	}
	canonicalMission, err := MarshalMissionIntentAt(caseRoot, activeIdentity)
	if err != nil || !bytes.Equal(canonicalMission, activeMissionBytes) || activeIdentity.ProjectID != inspection.Identity.ProjectID || activeIdentity.Pack != inspection.Identity.Pack || !strings.EqualFold(SHA256(activeMissionBytes), view.MissionIntentSHA256) {
		return Inspection{State: "corrupt"}, fmt.Errorf("active successor mission identity is not bound to the project and active pointer")
	}
	inspection.Identity = activeIdentity
	inspection.MissionIntentSHA256 = SHA256(activeMissionBytes)
	return inspection, nil

}

func inspectV1(caseRoot string, _ projectstate.Root, missionBytes []byte, missionPresent, bindingPresent bool, intentBytes []byte, intent Intent, commitBytes []byte, commitPresent bool) (Inspection, error) {
	if bindingPresent {
		return Inspection{State: "corrupt"}, fmt.Errorf("onboarding schemaVersion 1 must not contain a project binding")
	}
	canonicalIntent, err := MarshalIntent(intent)
	if err != nil || !bytes.Equal(canonicalIntent, intentBytes) {
		return Inspection{State: "corrupt"}, fmt.Errorf("onboarding intent is not canonical")
	}
	identity := intent.Identity
	if !samePath(identity.Target, caseRoot) {
		return Inspection{State: "corrupt"}, fmt.Errorf("mission intent Target does not match inspected case root")
	}
	if !missionPresent {
		if commitPresent {
			return Inspection{State: "corrupt"}, fmt.Errorf("onboarding commit exists without mission intent")
		}
		return Inspection{State: "pending", PublicationStamp: intent.PublicationStamp, OnboardingPlanSHA256: intent.OnboardingPlanSHA256, Identity: identity, IntentSHA256: SHA256(intentBytes), Recovery: intent.Recovery}, nil
	}
	var missionIdentity Identity
	if err := decodeCanonical(missionBytes, &missionIdentity); err != nil {
		return Inspection{State: "corrupt"}, fmt.Errorf("invalid mission intent: %w", err)
	}
	canonicalMission, err := MarshalMissionIntent(missionIdentity)
	if err != nil || !bytes.Equal(canonicalMission, missionBytes) || missionIdentity != identity {
		return Inspection{State: "corrupt"}, fmt.Errorf("mission intent does not match onboarding intent identity")
	}
	inspection := Inspection{
		State: "pending", PublicationStamp: intent.PublicationStamp, OnboardingPlanSHA256: intent.OnboardingPlanSHA256,
		Identity: identity, MissionIntentSHA256: SHA256(missionBytes), IntentSHA256: SHA256(intentBytes), Recovery: intent.Recovery,
	}
	if !commitPresent {
		return inspection, nil
	}
	var commit Commit
	if err := decodeCanonical(commitBytes, &commit); err != nil {
		return Inspection{State: "corrupt"}, fmt.Errorf("invalid onboarding commit: %w", err)
	}
	canonicalCommit, err := MarshalCommit(commit)
	if err != nil || !bytes.Equal(canonicalCommit, commitBytes) || commit.PublicationStamp != intent.PublicationStamp || commit.OnboardingPlanSHA256 != intent.OnboardingPlanSHA256 || !strings.EqualFold(commit.MissionIntentSHA256, inspection.MissionIntentSHA256) || !strings.EqualFold(commit.IntentSHA256, inspection.IntentSHA256) {
		return Inspection{State: "corrupt"}, fmt.Errorf("onboarding commit does not bind the exact intent generation")
	}
	inspection.State = "committed"
	inspection.Committed = true
	inspection.CommitSHA256 = SHA256(commitBytes)
	return inspection, nil
}

func inspectV2(caseRoot string, stateRoot projectstate.Root, missionBytes []byte, missionPresent bool, bindingBytes []byte, bindingPresent bool, intentBytes []byte, intent Intent, commitBytes []byte, commitPresent bool) (Inspection, error) {
	if stateRoot.Legacy {
		return Inspection{State: "corrupt"}, fmt.Errorf("onboarding schemaVersion 2 requires current .steamai state root")
	}
	canonicalIntent, err := MarshalIntentAt(caseRoot, intent)
	if err != nil || !bytes.Equal(canonicalIntent, intentBytes) {
		return Inspection{State: "corrupt"}, fmt.Errorf("onboarding intent is not canonical")
	}
	inspection := Inspection{
		State: "pending", PublicationStamp: intent.PublicationStamp, OnboardingPlanSHA256: intent.OnboardingPlanSHA256,
		Identity: intent.Identity, IntentSHA256: SHA256(intentBytes), Recovery: intent.Recovery,
	}
	if !missionPresent {
		if bindingPresent || commitPresent {
			return Inspection{State: "corrupt"}, fmt.Errorf("onboarding generation contains project binding or commit without mission intent")
		}
		return inspection, nil
	}
	var missionIdentity Identity
	if err := decodeCanonical(missionBytes, &missionIdentity); err != nil {
		return Inspection{State: "corrupt"}, fmt.Errorf("invalid mission intent: %w", err)
	}
	canonicalMission, err := MarshalMissionIntentAt(caseRoot, missionIdentity)
	if err != nil || !bytes.Equal(canonicalMission, missionBytes) || missionIdentity != intent.Identity {
		return Inspection{State: "corrupt"}, fmt.Errorf("mission intent does not match onboarding intent identity")
	}
	missionSHA := SHA256(missionBytes)
	inspection.Identity = missionIdentity
	inspection.MissionIntentSHA256 = missionSHA
	if !bindingPresent {
		if commitPresent {
			return Inspection{State: "corrupt"}, fmt.Errorf("onboarding commit exists without project binding")
		}
		return inspection, nil
	}
	var binding ProjectBinding
	if err := decodeCanonical(bindingBytes, &binding); err != nil {
		return Inspection{State: "corrupt"}, fmt.Errorf("invalid project binding: %w", err)
	}
	canonicalBinding, err := MarshalProjectBindingAt(caseRoot, binding)
	bindingSHA := SHA256(bindingBytes)
	if err != nil || !bytes.Equal(canonicalBinding, bindingBytes) || binding.ProjectID != missionIdentity.ProjectID || binding.Target != missionIdentity.Target || !strings.EqualFold(binding.MissionIntentSHA256, missionSHA) || binding.PublicationStamp != intent.PublicationStamp || binding.OnboardingPlanSHA256 != intent.OnboardingPlanSHA256 || !strings.EqualFold(intent.ProjectBindingSHA256, bindingSHA) {
		return Inspection{State: "corrupt"}, fmt.Errorf("project binding does not bind the exact mission and onboarding generation")
	}
	bindingCopy := binding
	inspection.ProjectBinding = &bindingCopy
	inspection.ProjectBindingSHA256 = bindingSHA
	if !commitPresent {
		return inspection, nil
	}
	var commit Commit
	if err := decodeCanonical(commitBytes, &commit); err != nil {
		return Inspection{State: "corrupt"}, fmt.Errorf("invalid onboarding commit: %w", err)
	}
	canonicalCommit, err := MarshalCommitAt(caseRoot, commit)
	if err != nil || !bytes.Equal(canonicalCommit, commitBytes) || commit.PublicationStamp != intent.PublicationStamp || commit.OnboardingPlanSHA256 != intent.OnboardingPlanSHA256 || !strings.EqualFold(commit.MissionIntentSHA256, inspection.MissionIntentSHA256) || !strings.EqualFold(commit.IntentSHA256, inspection.IntentSHA256) {
		return Inspection{State: "corrupt"}, fmt.Errorf("onboarding commit does not bind the exact intent generation")
	}
	inspection.State = "committed"
	inspection.Committed = true
	inspection.CommitSHA256 = SHA256(commitBytes)
	return inspection, nil
}

func ValidateCommittedV2ProjectedAt(caseRoot string, expected CommittedGeneration) error {
	physicalRoot, err := projectedPhysicalCaseRoot(caseRoot)
	if err != nil {
		return err
	}
	var identity Identity
	if err := decodeCanonical(expected.MissionIntentBytes, &identity); err != nil {
		return fmt.Errorf("invalid projected mission intent: %w", err)
	}
	var binding ProjectBinding
	if err := decodeCanonical(expected.ProjectBindingBytes, &binding); err != nil {
		return fmt.Errorf("invalid projected project binding: %w", err)
	}
	var intent Intent
	if err := decodeCanonical(expected.IntentBytes, &intent); err != nil {
		return fmt.Errorf("invalid projected onboarding intent: %w", err)
	}
	var commit Commit
	if err := decodeCanonical(expected.CommitBytes, &commit); err != nil {
		return fmt.Errorf("invalid projected onboarding commit: %w", err)
	}
	if identity.SchemaVersion != 2 || identity.Target != "." || !validProjectID(identity.ProjectID) || intent.SchemaVersion != 2 || intent.Identity != identity || binding.ProjectID != identity.ProjectID || binding.Target != "." || binding.MissionIntentSHA256 != SHA256(expected.MissionIntentBytes) || binding.PublicationStamp != intent.PublicationStamp || binding.OnboardingPlanSHA256 != intent.OnboardingPlanSHA256 || intent.ProjectBindingSHA256 != SHA256(expected.ProjectBindingBytes) || commit.PublicationStamp != intent.PublicationStamp || commit.OnboardingPlanSHA256 != intent.OnboardingPlanSHA256 || commit.MissionIntentSHA256 != SHA256(expected.MissionIntentBytes) || commit.IntentSHA256 != SHA256(expected.IntentBytes) || !binding.NoAuthority || !binding.NoConfirmed || !binding.NoHeavyTool {
		return fmt.Errorf("projected current onboarding generation hash chain is invalid")
	}
	if err := ValidateProjectedCurrentRecoveryAt(physicalRoot, identity, intent.Recovery); err != nil {
		return err
	}
	for _, artifact := range []struct {
		rel  string
		data []byte
	}{
		{filepath.ToSlash(filepath.Join(projectstate.CurrentDir, "mission-intent.json")), expected.MissionIntentBytes},
		{ProjectBindingRel, expected.ProjectBindingBytes},
		{filepath.ToSlash(filepath.Join(projectstate.CurrentDir, "onboarding", "intent.json")), expected.IntentBytes},
		{filepath.ToSlash(filepath.Join(projectstate.CurrentDir, "onboarding", "commit.json")), expected.CommitBytes},
	} {
		path := filepath.Join(physicalRoot, filepath.FromSlash(artifact.rel))
		data, err := rekitfs.ReadStableRegularFileAnchored(physicalRoot, path, "projected current onboarding artifact", maxIntentArtifactBytes)
		if err != nil {
			return fmt.Errorf("read projected current onboarding artifact %s: %w", artifact.rel, err)
		}
		if !bytes.Equal(data, artifact.data) {
			return fmt.Errorf("projected current onboarding artifact differs: %s", artifact.rel)
		}
	}
	return nil
}

func AssertCommittedOrAbsent(caseRoot string) error {
	inspection, err := Inspect(caseRoot)
	if err != nil {
		return fmt.Errorf("onboarding state is corrupt; ordinary attached commands are blocked: %w", err)
	}
	if inspection.State == "pending" {
		return fmt.Errorf("onboarding publication is pending; rerun the exact onboard -Apply command with stamp %s and plan hash %s", inspection.PublicationStamp, inspection.OnboardingPlanSHA256)
	}
	return nil
}

func SHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func marshalCanonical(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func decodeCanonical(data []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("trailing JSON data")
	}
	return nil
}

func openPinnedCaseRoot(caseRoot string) (*os.Root, os.FileInfo, bool, error) {
	caseRoot = filepath.Clean(caseRoot)
	volume := filepath.VolumeName(caseRoot)
	anchor := string(filepath.Separator)
	if volume != "" {
		anchor = volume + string(filepath.Separator)
	}
	anchorInfo, err := os.Lstat(anchor)
	if err != nil {
		return nil, nil, false, err
	}
	if err := rejectReparsePath(anchor); err != nil {
		return nil, nil, false, err
	}
	current, err := os.OpenRoot(anchor)
	if err != nil {
		return nil, nil, false, err
	}
	opened, err := current.Stat(".")
	if err != nil || !os.SameFile(anchorInfo, opened) {
		_ = current.Close()
		return nil, nil, false, fmt.Errorf("onboarding case root anchor changed while opening: %s", anchor)
	}
	rel, err := filepath.Rel(anchor, caseRoot)
	if err != nil {
		_ = current.Close()
		return nil, nil, false, err
	}
	currentPath := anchor
	if rel != "." {
		for component := range strings.SplitSeq(rel, string(filepath.Separator)) {
			if component == "" || component == "." || component == ".." {
				_ = current.Close()
				return nil, nil, false, fmt.Errorf("invalid onboarding case root component: %s", caseRoot)
			}
			info, statErr := current.Lstat(component)
			if os.IsNotExist(statErr) {
				_ = current.Close()
				return nil, nil, false, nil
			}
			if statErr != nil {
				_ = current.Close()
				return nil, nil, false, statErr
			}
			currentPath = filepath.Join(currentPath, component)
			if err := rejectReparsePath(currentPath); err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
				_ = current.Close()
				if err != nil {
					return nil, nil, false, err
				}
				return nil, nil, false, fmt.Errorf("onboarding case root ancestor is not a regular directory: %s", currentPath)
			}
			next, openErr := current.OpenRoot(component)
			if openErr != nil {
				_ = current.Close()
				return nil, nil, false, openErr
			}
			nextInfo, statErr := next.Stat(".")
			if statErr != nil || !os.SameFile(info, nextInfo) {
				_ = next.Close()
				_ = current.Close()
				return nil, nil, false, fmt.Errorf("onboarding case root ancestor changed while opening: %s", currentPath)
			}
			_ = current.Close()
			current = next
			opened = nextInfo
		}
	}
	return current, opened, true, nil
}

func openPinnedChildRoot(parent *os.Root, parentPath, name string) (*os.Root, os.FileInfo, bool, error) {
	before, err := parent.Lstat(name)
	if os.IsNotExist(err) {
		return nil, nil, false, nil
	}
	if err != nil {
		return nil, nil, false, err
	}
	displayPath := filepath.Join(parentPath, name)
	if err := rejectReparsePath(displayPath); err != nil || before.Mode()&os.ModeSymlink != 0 || !before.IsDir() {
		if err != nil {
			return nil, nil, false, err
		}
		return nil, nil, false, fmt.Errorf("onboarding namespace is not a regular directory: %s", displayPath)
	}
	child, err := parent.OpenRoot(name)
	if err != nil {
		return nil, nil, false, err
	}
	opened, openErr := child.Stat(".")
	after, afterErr := parent.Lstat(name)
	if afterErr == nil {
		afterErr = rejectReparsePath(displayPath)
	}
	if openErr != nil || afterErr != nil || !os.SameFile(before, opened) || !os.SameFile(opened, after) {
		_ = child.Close()
		return nil, nil, false, fmt.Errorf("onboarding namespace changed while opening: %s", displayPath)
	}
	return child, opened, true, nil
}

func validateInspectionNamespaceBindings(caseRoot string, stateRoot projectstate.Root, root *os.Root, rootInfo os.FileInfo, stateNamespaceRoot *os.Root, stateNamespaceInfo os.FileInfo, onboardingRoot *os.Root, onboardingInfo os.FileInfo) error {
	if err := validatePinnedCaseRootBinding(caseRoot, root, rootInfo); err != nil {
		return err
	}
	if stateNamespaceRoot != nil {
		if err := validatePinnedChildBinding(root, stateNamespaceRoot, stateNamespaceInfo, caseRoot, stateRoot.Dir); err != nil {
			return err
		}
	}
	if onboardingRoot != nil {
		if err := validatePinnedChildBinding(stateNamespaceRoot, onboardingRoot, onboardingInfo, stateRoot.Path, "onboarding"); err != nil {
			return err
		}
	}
	return nil
}

func validatePinnedChildBinding(parent, child *os.Root, pinned os.FileInfo, parentPath, name string) error {
	displayPath := filepath.Join(parentPath, name)
	lexical, err := os.Lstat(displayPath)
	if err == nil {
		err = rejectReparsePath(displayPath)
	}
	opened, openErr := child.Stat(".")
	parentInfo, parentErr := parent.Lstat(name)
	if err != nil || openErr != nil || parentErr != nil || !os.SameFile(pinned, opened) || !os.SameFile(opened, lexical) || !os.SameFile(opened, parentInfo) {
		return fmt.Errorf("onboarding namespace changed while inspecting: %s", displayPath)
	}
	return nil
}

func validatePinnedCaseRootBinding(caseRoot string, root *os.Root, pinned os.FileInfo) error {
	lexical, err := os.Lstat(caseRoot)
	if err != nil {
		return err
	}
	if err := rejectReparsePath(caseRoot); err != nil {
		return err
	}
	opened, err := root.Stat(".")
	if err != nil || !os.SameFile(pinned, opened) || !os.SameFile(opened, lexical) {
		return fmt.Errorf("onboarding case root changed while inspecting: %s", caseRoot)
	}
	return nil
}

var inspectArtifactReadHook func(rel string) error

func SetInspectArtifactReadHookForTest(hook func(rel string) error) func() {
	previous := inspectArtifactReadHook
	inspectArtifactReadHook = hook
	return func() { inspectArtifactReadHook = previous }
}

func readStrictPinned(parent *os.Root, parentPath, leaf, rel string, limit int64) ([]byte, bool, error) {
	if limit <= 0 {
		return nil, false, fmt.Errorf("onboarding artifact read limit must be positive")
	}
	before, err := parent.Lstat(leaf)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	displayPath := filepath.Join(parentPath, leaf)
	if err := rejectReparsePath(displayPath); err != nil {
		return nil, false, err
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() || before.Size() <= 0 || before.Size() > limit {
		return nil, false, fmt.Errorf("onboarding artifact is not a bounded regular file: %s", rel)
	}
	file, err := parent.Open(leaf)
	if err != nil {
		return nil, false, err
	}
	opened, statErr := file.Stat()
	data, readErr := io.ReadAll(io.LimitReader(file, limit+1))
	closeErr := file.Close()
	after, afterErr := parent.Lstat(leaf)
	if afterErr == nil {
		afterErr = rejectReparsePath(displayPath)
	}
	if statErr != nil || readErr != nil || closeErr != nil || afterErr != nil || !os.SameFile(before, opened) || !os.SameFile(opened, after) || int64(len(data)) != opened.Size() || int64(len(data)) > limit {
		return nil, false, fmt.Errorf("onboarding artifact changed while reading: %s", rel)
	}
	if inspectArtifactReadHook != nil {
		if err := inspectArtifactReadHook(rel); err != nil {
			return nil, false, err
		}
	}
	return data, true, nil
}

func validPlanHash(value string) bool {
	return value == "<onboarding-plan-sha256>" || validSHA256(value)
}

func validProjectID(value string) bool {
	if len(value) != 16 {
		return false
	}
	for _, character := range value {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return false
		}
	}
	return true
}

func validSHA256(value string) bool {
	if len(strings.TrimSpace(value)) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func samePath(left, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func validStamp(value string) bool {
	if len(value) != len("20060102-150405000") || value[8] != '-' {
		return false
	}
	for i, r := range value {
		if i == 8 {
			continue
		}
		if r < '0' || r > '9' {
			return false
		}
	}
	_, err := time.Parse("20060102-150405.000", value[:15]+"."+value[15:])
	return err == nil
}
