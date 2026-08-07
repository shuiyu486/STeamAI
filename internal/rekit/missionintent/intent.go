package missionintent

import (
	"bytes"
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
)

const (
	MissionIntentRel       = ".rekit/mission-intent.json"
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
	Identity             Identity         `json:"identity"`
	Recovery             RecoveryEnvelope `json:"recovery"`
}

type RecoveryEnvelope struct {
	SchemaVersion    int                `json:"schemaVersion"`
	RepoRoot         string             `json:"repoRoot"`
	CreatedAt        string             `json:"createdAt"`
	Mode             string             `json:"mode,omitempty"`
	Writes           []RecoveryWrite    `json:"writes,omitempty"`
	AttachedSnapshot []SnapshotArtifact `json:"attachedSnapshot,omitempty"`
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

type Inspection struct {
	State                string           `json:"state"`
	Committed            bool             `json:"committed"`
	PublicationStamp     string           `json:"publicationStamp,omitempty"`
	OnboardingPlanSHA256 string           `json:"onboardingPlanSha256,omitempty"`
	Identity             Identity         `json:"identity"`
	MissionIntentSHA256  string           `json:"missionIntentSha256,omitempty"`
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

func MarshalIntent(value Intent) ([]byte, error) {
	if value.SchemaVersion != 1 || value.Kind != "mission-onboarding-intent" || !validStamp(value.PublicationStamp) || !validPlanHash(value.OnboardingPlanSHA256) {
		return nil, fmt.Errorf("invalid onboarding intent identity")
	}
	if err := ValidateIdentity(value.Identity); err != nil {
		return nil, err
	}
	if err := ValidateRecoveryEnvelope(value.Identity, value.Recovery); err != nil {
		return nil, err
	}
	data, err := marshalCanonical(value)
	if err != nil {
		return nil, err
	}
	if len(data) > maxIntentArtifactBytes {
		return nil, fmt.Errorf("canonical onboarding intent exceeds %d bytes", maxIntentArtifactBytes)
	}
	return data, nil
}

func MarshalCommit(value Commit) ([]byte, error) {
	if value.SchemaVersion != 1 || value.Kind != "mission-onboarding-commit" || !validStamp(value.PublicationStamp) || !validSHA256(value.OnboardingPlanSHA256) || !validSHA256(value.MissionIntentSHA256) || !validSHA256(value.IntentSHA256) {
		return nil, fmt.Errorf("invalid onboarding commit identity")
	}
	return marshalCanonical(value)
}

func ValidateIdentity(identity Identity) error {
	if identity.SchemaVersion != 1 {
		return fmt.Errorf("mission intent requires schemaVersion 1")
	}
	fields := map[string]string{
		"Target": identity.Target, "Pack": identity.Pack, "ProjectName": identity.ProjectName,
		"Goal": identity.Goal, "Actor": identity.Actor, "Executor": identity.Executor, "InitialLane": identity.InitialLane,
	}
	for name, value := range fields {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("mission intent requires %s", name)
		}
	}
	target, err := filepath.Abs(identity.Target)
	if err != nil || !samePath(target, identity.Target) {
		return fmt.Errorf("mission intent Target must be canonical absolute path: %s", identity.Target)
	}
	return nil
}

func ValidateRecoveryEnvelope(identity Identity, envelope RecoveryEnvelope) error {
	if envelope.SchemaVersion != 1 || strings.TrimSpace(envelope.RepoRoot) == "" || strings.TrimSpace(envelope.CreatedAt) == "" {
		return fmt.Errorf("onboarding recovery envelope is missing stable identity")
	}
	if _, err := filepath.Abs(envelope.RepoRoot); err != nil || !filepath.IsAbs(envelope.RepoRoot) {
		return fmt.Errorf("onboarding recovery repoRoot must be absolute")
	}
	if _, err := time.Parse(time.RFC3339Nano, envelope.CreatedAt); err != nil {
		return fmt.Errorf("invalid onboarding recovery createdAt: %w", err)
	}
	if envelope.Mode == "attached-adoption" {
		return validateAttachedSnapshot(envelope)
	}
	if envelope.Mode != "" || len(envelope.AttachedSnapshot) != 0 {
		return fmt.Errorf("onboarding recovery envelope has an unsupported mode")
	}
	if len(envelope.Writes) == 0 || len(envelope.Writes) > maxRecoveryWrites {
		return fmt.Errorf("onboarding recovery write count is outside bounds")
	}
	seen := map[string]struct{}{}
	required := map[string]string{
		".rekit/instance.yml":           "instance-metadata",
		".claude/skills/rekit/skill.md": "case-local-thin-shim",
		".re-template.yml":              "legacy-metadata",
		".rekit/state.json":             "initial-state",
	}
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
		if err := validateRecoveryWrite(identity, envelope, key, write); err != nil {
			return err
		}
		if strings.TrimSpace(write.Kind) == "" || write.PublicationPhase < lastPhase || (write.PublicationPhase == lastPhase && lastPath != "" && write.Path <= lastPath) {
			return fmt.Errorf("onboarding recovery writes are not ordered: %s", rel)
		}
		lastPhase, lastPath = write.PublicationPhase, write.Path
		sum := SHA256(write.Content)
		if write.Size != int64(len(write.Content)) || !strings.EqualFold(write.SHA256, sum) {
			return fmt.Errorf("onboarding recovery content binding mismatch: %s", rel)
		}
		total += len(write.Content)
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
	return nil
}

func validateAttachedSnapshot(envelope RecoveryEnvelope) error {
	if len(envelope.Writes) != 0 || len(envelope.AttachedSnapshot) == 0 || len(envelope.AttachedSnapshot) > maxRecoveryWrites {
		return fmt.Errorf("attached onboarding recovery snapshot count is outside bounds")
	}
	required := map[string]string{
		".rekit/instance.yml":           "instance-metadata",
		".claude/skills/rekit/skill.md": "case-local-thin-shim",
		".re-template.yml":              "legacy-metadata",
		".rekit/state.json":             "initial-state",
	}
	seen := map[string]struct{}{}
	var total int64
	lastPath := ""
	for _, artifact := range envelope.AttachedSnapshot {
		rel, key, err := validateRecoveryPath(artifact.Path)
		if err != nil {
			return err
		}
		if _, ok := seen[key]; ok {
			return fmt.Errorf("duplicate attached onboarding snapshot path: %s", rel)
		}
		seen[key] = struct{}{}
		if artifact.Path <= lastPath {
			return fmt.Errorf("attached onboarding snapshot is not ordered: %s", rel)
		}
		fixedKinds := map[string]string{
			".rekit/instance.yml":           "instance-metadata",
			".claude/skills/rekit/skill.md": "case-local-thin-shim",
			".re-template.yml":              "legacy-metadata",
			".rekit/state.json":             "sync-state",
		}
		if expected, fixed := fixedKinds[key]; fixed {
			if artifact.Path != canonicalRecoveryPath(key) || artifact.Kind != expected {
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

func validateRecoveryWrite(identity Identity, envelope RecoveryEnvelope, key string, write RecoveryWrite) error {
	fixedKinds := map[string]string{
		".rekit/instance.yml":           "instance-metadata",
		".claude/skills/rekit/skill.md": "case-local-thin-shim",
		".re-template.yml":              "legacy-metadata",
		".rekit/state.json":             "initial-state",
		".gitignore":                    "support-file",
	}
	if expected, ok := fixedKinds[key]; ok {
		if write.Path != canonicalRecoveryPath(key) {
			return fmt.Errorf("onboarding recovery fixed artifact requires canonical path casing: %s", write.Path)
		}
		if write.Kind != expected {
			return fmt.Errorf("onboarding recovery write %s requires kind %s", write.Path, expected)
		}
	}
	if isRecoveryControlPath(key) {
		return fmt.Errorf("onboarding recovery rejects runtime/control namespace: %s", write.Path)
	}
	switch key {
	case ".rekit/instance.yml":
		expected := "schemaVersion: 1\n" +
			"templateRoot: " + envelope.RepoRoot + "\n" +
			"templatePack: " + identity.Pack + "\n" +
			"projectName: " + identity.ProjectName + "\n" +
			"projectRoot: " + identity.Target + "\n" +
			"mode: case-local-shim\n"
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
	case ".rekit/state.json":
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
		if err := decodeCanonical(write.Content, &state); err != nil || state.SchemaVersion != 1 || !samePath(state.TemplateRoot, envelope.RepoRoot) || state.TemplatePack != identity.Pack || state.LastSyncAt != envelope.CreatedAt {
			return fmt.Errorf("onboarding recovery initial state does not bind repo/pack/generation")
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
		case "managed-file", "template-file":
			normalized := append([]byte{}, write.Content...)
			if write.Kind == "template-file" {
				normalized = []byte(strings.ReplaceAll(strings.ReplaceAll(string(normalized), identity.Target, "<PROJECT_ROOT>"), identity.ProjectName, "<PROJECT_NAME>"))
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

func canonicalRecoveryPath(key string) string {
	return map[string]string{
		".rekit/instance.yml":           ".rekit/instance.yml",
		".claude/skills/rekit/skill.md": ".claude/skills/rekit/SKILL.md",
		".re-template.yml":              ".re-template.yml",
		".rekit/state.json":             ".rekit/state.json",
		".gitignore":                    ".gitignore",
	}[key]
}

func isRecoveryControlPath(key string) bool {
	if key == ".rekit/instance.yml" || key == ".rekit/state.json" || key == ".re-template.yml" || key == ".claude/skills/rekit/skill.md" {
		return false
	}
	for _, prefix := range []string{".rekit/", ".claude/skills/", "lanes/", "board/", "facts/", "gate/", "ledger/", "authority/", "confirmed/"} {
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
	caseRoot, err := filepath.Abs(caseRoot)
	if err != nil {
		return Inspection{State: "corrupt"}, err
	}
	caseRoot = filepath.Clean(caseRoot)
	root, rootInfo, present, err := openPinnedCaseRoot(caseRoot)
	if err != nil {
		return Inspection{State: "corrupt"}, err
	}
	if !present {
		return Inspection{State: "absent"}, nil
	}
	rekitRoot, rekitInfo, rekitPresent, err := openPinnedChildRoot(root, caseRoot, ".rekit")
	if err != nil {
		_ = root.Close()
		return Inspection{State: "corrupt"}, err
	}
	var onboardingRoot *os.Root
	var onboardingInfo os.FileInfo
	if rekitPresent {
		onboardingRoot, onboardingInfo, _, err = openPinnedChildRoot(rekitRoot, filepath.Join(caseRoot, ".rekit"), "onboarding")
		if err != nil {
			_ = rekitRoot.Close()
			_ = root.Close()
			return Inspection{State: "corrupt"}, err
		}
	}
	defer func() {
		bindingErr := validateInspectionNamespaceBindings(caseRoot, root, rootInfo, rekitRoot, rekitInfo, onboardingRoot, onboardingInfo)
		if onboardingRoot != nil {
			_ = onboardingRoot.Close()
		}
		if rekitRoot != nil {
			_ = rekitRoot.Close()
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
	if err := validateInspectionNamespaceBindings(caseRoot, root, rootInfo, rekitRoot, rekitInfo, onboardingRoot, onboardingInfo); err != nil {
		return Inspection{State: "corrupt"}, err
	}
	var missionBytes, intentBytes, commitBytes []byte
	var missionPresent, intentPresent, commitPresent bool
	if rekitRoot != nil {
		missionBytes, missionPresent, err = readStrictPinned(rekitRoot, filepath.Join(caseRoot, ".rekit"), "mission-intent.json", MissionIntentRel, maxArtifactBytes)
		if err != nil {
			return Inspection{State: "corrupt"}, err
		}
	}
	if onboardingRoot != nil {
		intentBytes, intentPresent, err = readStrictPinned(onboardingRoot, filepath.Join(caseRoot, ".rekit", "onboarding"), "intent.json", IntentRel, maxIntentArtifactBytes)
		if err != nil {
			return Inspection{State: "corrupt"}, err
		}
		commitBytes, commitPresent, err = readStrictPinned(onboardingRoot, filepath.Join(caseRoot, ".rekit", "onboarding"), "commit.json", CommitRel, maxArtifactBytes)
		if err != nil {
			return Inspection{State: "corrupt"}, err
		}
	}
	if !missionPresent && !intentPresent && !commitPresent {
		return Inspection{State: "absent"}, nil
	}
	if !intentPresent {
		return Inspection{State: "corrupt"}, fmt.Errorf("onboarding intent is missing")
	}
	var intent Intent
	if err := decodeCanonical(intentBytes, &intent); err != nil {
		return Inspection{State: "corrupt"}, fmt.Errorf("invalid onboarding intent: %w", err)
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
	inspection = Inspection{
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

func validateInspectionNamespaceBindings(caseRoot string, root *os.Root, rootInfo os.FileInfo, rekitRoot *os.Root, rekitInfo os.FileInfo, onboardingRoot *os.Root, onboardingInfo os.FileInfo) error {
	if err := validatePinnedCaseRootBinding(caseRoot, root, rootInfo); err != nil {
		return err
	}
	if rekitRoot != nil {
		if err := validatePinnedChildBinding(root, rekitRoot, rekitInfo, caseRoot, ".rekit"); err != nil {
			return err
		}
	}
	if onboardingRoot != nil {
		if err := validatePinnedChildBinding(rekitRoot, onboardingRoot, onboardingInfo, filepath.Join(caseRoot, ".rekit"), "onboarding"); err != nil {
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
