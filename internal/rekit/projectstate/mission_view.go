package projectstate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
)

const (
	MissionsDir         = "missions"
	MissionActiveFile   = "active.json"
	MissionManifestFile = "manifest.json"
)

var missionIDPattern = regexp.MustCompile(`^mission-g[0-9]{6}-[a-f0-9]{16}$`)

type MissionActive struct {
	SchemaVersion       int    `json:"schemaVersion"`
	Kind                string `json:"kind"`
	ProjectID           string `json:"projectId"`
	Generation          int    `json:"generation"`
	MissionID           string `json:"missionId"`
	MissionIntentSHA256 string `json:"missionIntentSha256"`
	ManifestSHA256      string `json:"manifestSha256"`
	TransitionID        string `json:"transitionId"`
	TransitionCommitSHA string `json:"transitionCommitSha256"`
}

type MissionView struct {
	Root                Root
	Generation          int
	MissionID           string
	MissionIntentSHA256 string
	ActivePointerSHA256 string
	Path                string
	Active              *MissionActive
}

type missionGenerationManifest struct {
	SchemaVersion      int    `json:"schemaVersion"`
	Kind               string `json:"kind"`
	ProjectID          string `json:"projectId"`
	Pack               string `json:"pack"`
	PackManifestSHA256 string `json:"packManifestSha256"`
	AuthorityLane      string `json:"authorityLane"`
	Generation         int    `json:"generation"`
	MissionID          string `json:"missionId"`
	MissionIntentSHA   string `json:"missionIntentSha256"`
	PreviousClosureSHA string `json:"previousClosureSha256"`
	TransitionID       string `json:"transitionId"`
	PlanSHA256         string `json:"planSha256"`
	NoAuthority        bool   `json:"noAuthority"`
	NoConfirmed        bool   `json:"noConfirmed"`
	NoHeavyTool        bool   `json:"noHeavyTool"`
}

type missionGenerationCommit struct {
	SchemaVersion  int    `json:"schemaVersion"`
	Kind           string `json:"kind"`
	State          string `json:"state"`
	Generation     int    `json:"generation"`
	MissionID      string `json:"missionId"`
	ManifestSHA256 string `json:"manifestSha256"`
	PlanSHA256     string `json:"planSha256"`
}

type missionTransitionIntent struct {
	SchemaVersion      int             `json:"schemaVersion"`
	Kind               string          `json:"kind"`
	TransitionID       string          `json:"transitionId"`
	PublicationStamp   string          `json:"publicationStamp"`
	ProjectID          string          `json:"projectId"`
	Pack               string          `json:"pack"`
	PreviousGeneration int             `json:"previousGeneration"`
	Generation         int             `json:"generation"`
	MissionID          string          `json:"missionId"`
	Goal               string          `json:"goal"`
	Actor              string          `json:"actor"`
	InitialLane        string          `json:"initialLane"`
	Executor           string          `json:"executor"`
	PreviousMissionSHA string          `json:"previousMissionSha256"`
	PreviousClosureSHA string          `json:"previousClosureSha256"`
	PreviousCompletion json.RawMessage `json:"previousCompletion"`
	PlanSHA256         string          `json:"planSha256"`
	NoAuthority        bool            `json:"noAuthority"`
	NoConfirmed        bool            `json:"noConfirmed"`
	NoHeavyTool        bool            `json:"noHeavyTool"`
	NoAutoResume       bool            `json:"noAutoResume"`
}

type missionIdentity struct {
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

type missionProjectBinding struct {
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

func ResolveMissionView(caseRoot string) (MissionView, error) {
	root, err := Resolve(caseRoot)
	if err != nil {
		return MissionView{}, err
	}
	view := MissionView{Root: root, Generation: 1, Path: root.Path}
	if root.Legacy || !root.Existing {
		return view, nil
	}
	path := filepath.Join(root.Path, MissionsDir, MissionActiveFile)
	data, err := refsf.ReadStableRegularFileAnchored(root.Path, path, "active mission pointer", 64<<10)
	if err != nil {
		if _, statErr := os.Lstat(path); os.IsNotExist(statErr) {
			return view, nil
		}
		return MissionView{}, err
	}
	var active MissionActive
	if err := decodeMissionCanonical(data, &active); err != nil {
		return MissionView{}, fmt.Errorf("invalid active mission pointer: %w", err)
	}
	if err := validateMissionActive(active); err != nil {
		return MissionView{}, err
	}
	generationDir := missionGenerationDir(active.Generation)
	manifestPath := filepath.Join(root.Path, MissionsDir, generationDir, MissionManifestFile)
	manifest, err := refsf.ReadStableRegularFileAnchored(root.Path, manifestPath, "active mission manifest", 4<<20)
	if err != nil {
		return MissionView{}, fmt.Errorf("read active mission manifest: %w", err)
	}
	if !strings.EqualFold(missionSHA256(manifest), active.ManifestSHA256) {
		return MissionView{}, fmt.Errorf("active mission manifest differs from the pointer binding")
	}
	var generationManifest missionGenerationManifest
	if err := decodeMissionCanonical(manifest, &generationManifest); err != nil ||
		generationManifest.SchemaVersion != 1 ||
		generationManifest.Kind != "mission-generation-manifest" ||
		generationManifest.ProjectID != active.ProjectID ||
		strings.TrimSpace(generationManifest.Pack) == "" ||
		!validMissionSHA256(generationManifest.PackManifestSHA256) ||
		strings.TrimSpace(generationManifest.AuthorityLane) == "" ||
		generationManifest.Generation != active.Generation ||
		generationManifest.MissionID != active.MissionID ||
		!strings.EqualFold(generationManifest.MissionIntentSHA, active.MissionIntentSHA256) ||
		generationManifest.TransitionID != active.TransitionID ||
		!validMissionSHA256(generationManifest.PreviousClosureSHA) ||
		!validMissionSHA256(generationManifest.PlanSHA256) ||
		!generationManifest.NoAuthority || !generationManifest.NoConfirmed || !generationManifest.NoHeavyTool {
		return MissionView{}, fmt.Errorf("active mission manifest does not bind the exact pointer")
	}
	generationCommitPath := filepath.Join(root.Path, MissionsDir, generationDir, "commit.json")
	generationCommitBytes, err := refsf.ReadStableRegularFileAnchored(root.Path, generationCommitPath, "active mission generation commit", 1<<20)
	if err != nil {
		return MissionView{}, fmt.Errorf("read active mission generation commit: %w", err)
	}
	var generationCommit missionGenerationCommit
	if err := decodeMissionCanonical(generationCommitBytes, &generationCommit); err != nil ||
		generationCommit.SchemaVersion != 1 ||
		generationCommit.Kind != "mission-generation-commit" ||
		generationCommit.State != "committed" ||
		generationCommit.Generation != active.Generation ||
		generationCommit.MissionID != active.MissionID ||
		!strings.EqualFold(generationCommit.ManifestSHA256, active.ManifestSHA256) ||
		!strings.EqualFold(generationCommit.PlanSHA256, generationManifest.PlanSHA256) {
		return MissionView{}, fmt.Errorf("active mission generation commit does not bind the manifest")
	}
	transitionPath := filepath.Join(root.Path, "transitions", "successor", active.TransitionID, "commit.json")
	transition, err := refsf.ReadStableRegularFileAnchored(root.Path, transitionPath, "active mission transition commit", 1<<20)
	if err != nil {
		return MissionView{}, fmt.Errorf("read active mission transition commit: %w", err)
	}
	if !strings.EqualFold(missionSHA256(transition), active.TransitionCommitSHA) {
		return MissionView{}, fmt.Errorf("active mission transition commit differs from the pointer binding")
	}
	var transitionCommit struct {
		SchemaVersion  int    `json:"schemaVersion"`
		Kind           string `json:"kind"`
		State          string `json:"state"`
		TransitionID   string `json:"transitionId"`
		PlanSHA256     string `json:"planSha256"`
		IntentSHA256   string `json:"intentSha256"`
		Generation     int    `json:"generation"`
		MissionID      string `json:"missionId"`
		ManifestSHA256 string `json:"manifestSha256"`
		CommittedAt    string `json:"committedAt"`
		NoAuthority    bool   `json:"noAuthority"`
		NoConfirmed    bool   `json:"noConfirmed"`
		NoHeavyTool    bool   `json:"noHeavyTool"`
		NoAutoResume   bool   `json:"noAutoResume"`
	}
	if err := decodeMissionCanonical(transition, &transitionCommit); err != nil || transitionCommit.SchemaVersion != 1 || transitionCommit.Kind != "mission-successor-commit" || transitionCommit.State != "committed" || transitionCommit.TransitionID != active.TransitionID || transitionCommit.Generation != active.Generation || transitionCommit.MissionID != active.MissionID || !strings.EqualFold(transitionCommit.ManifestSHA256, active.ManifestSHA256) || !strings.EqualFold(transitionCommit.PlanSHA256, generationManifest.PlanSHA256) || !validMissionSHA256(transitionCommit.IntentSHA256) || !transitionCommit.NoAuthority || !transitionCommit.NoConfirmed || !transitionCommit.NoHeavyTool || !transitionCommit.NoAutoResume {
		return MissionView{}, fmt.Errorf("active mission transition commit does not bind the exact pointer")
	}
	intentPath := filepath.Join(root.Path, "transitions", "successor", active.TransitionID, "intent.json")
	intent, err := refsf.ReadStableRegularFileAnchored(root.Path, intentPath, "active mission transition intent", 1<<20)
	if err != nil {
		return MissionView{}, fmt.Errorf("read active mission transition intent: %w", err)
	}
	if !strings.EqualFold(missionSHA256(intent), transitionCommit.IntentSHA256) {
		return MissionView{}, fmt.Errorf("active mission transition intent differs from the commit binding")
	}
	var transitionIntent missionTransitionIntent
	if err := decodeMissionCanonical(intent, &transitionIntent); err != nil ||
		transitionIntent.SchemaVersion != 1 ||
		transitionIntent.Kind != "mission-successor-intent" ||
		transitionIntent.TransitionID != active.TransitionID ||
		transitionIntent.ProjectID != active.ProjectID ||
		transitionIntent.Pack != generationManifest.Pack ||
		transitionIntent.Generation != active.Generation ||
		transitionIntent.MissionID != active.MissionID ||
		!strings.EqualFold(transitionIntent.PlanSHA256, generationManifest.PlanSHA256) ||
		!strings.EqualFold(transitionIntent.PreviousClosureSHA, generationManifest.PreviousClosureSHA) ||
		!transitionIntent.NoAuthority || !transitionIntent.NoConfirmed || !transitionIntent.NoHeavyTool || !transitionIntent.NoAutoResume {
		return MissionView{}, fmt.Errorf("active mission transition intent does not bind the exact pointer")
	}
	missionPath := filepath.Join(root.Path, MissionsDir, generationDir, "mission-intent.json")
	mission, err := refsf.ReadStableRegularFileAnchored(root.Path, missionPath, "active mission intent", 64<<10)
	if err != nil {
		return MissionView{}, fmt.Errorf("read active mission intent: %w", err)
	}
	if !strings.EqualFold(missionSHA256(mission), active.MissionIntentSHA256) {
		return MissionView{}, fmt.Errorf("active mission intent differs from the pointer binding")
	}
	var identity missionIdentity
	if err := decodeMissionCanonical(mission, &identity); err != nil ||
		identity.SchemaVersion != 2 || identity.Target != "." ||
		identity.ProjectID != active.ProjectID || identity.Pack != generationManifest.Pack ||
		identity.Goal != transitionIntent.Goal || identity.Actor != transitionIntent.Actor ||
		identity.Executor != transitionIntent.Executor || identity.InitialLane != transitionIntent.InitialLane {
		return MissionView{}, fmt.Errorf("active mission intent does not bind the active transition")
	}
	projectBindingPath := filepath.Join(root.Path, "project-binding.json")
	projectBindingBytes, err := refsf.ReadStableRegularFileAnchored(root.Path, projectBindingPath, "active mission project binding", 64<<10)
	if err != nil {
		return MissionView{}, fmt.Errorf("read active mission project binding: %w", err)
	}
	var projectBinding missionProjectBinding
	if err := decodeMissionCanonical(projectBindingBytes, &projectBinding); err != nil ||
		projectBinding.SchemaVersion != 1 || projectBinding.Target != "." ||
		projectBinding.ProjectID != active.ProjectID ||
		!validMissionSHA256(projectBinding.MissionIntentSHA256) ||
		!validMissionSHA256(projectBinding.OnboardingPlanSHA256) ||
		!projectBinding.NoAuthority || !projectBinding.NoConfirmed || !projectBinding.NoHeavyTool {
		return MissionView{}, fmt.Errorf("active mission project binding does not bind the project identity")
	}
	view.Generation = active.Generation
	view.MissionID = active.MissionID
	view.MissionIntentSHA256 = strings.ToLower(active.MissionIntentSHA256)
	view.ActivePointerSHA256 = missionSHA256(data)
	view.Path = filepath.Join(root.Path, MissionsDir, generationDir)
	view.Active = &active
	return view, nil
}

func (view MissionView) ValidateCurrent(caseRoot string) error {
	current, err := ResolveMissionView(caseRoot)
	if err != nil {
		return err
	}
	if !SameMissionView(current, view) {
		return fmt.Errorf("active mission namespace changed: got generation=%d mission=%q path=%s, want generation=%d mission=%q path=%s", current.Generation, current.MissionID, current.Path, view.Generation, view.MissionID, view.Path)
	}
	return nil
}

func SameMissionView(left, right MissionView) bool {
	return left.Generation == right.Generation &&
		left.MissionID == right.MissionID &&
		left.MissionIntentSHA256 == right.MissionIntentSHA256 &&
		left.ActivePointerSHA256 == right.ActivePointerSHA256 &&
		filepath.Clean(left.Path) == filepath.Clean(right.Path)
}

func (view MissionView) Join(parts ...string) (string, error) {
	local, err := validateParts(parts)
	if err != nil {
		return "", err
	}
	path := view.Path
	if local != "" {
		path = filepath.Join(path, local)
	}
	if err := validateContained(view.Path, path); err != nil {
		return "", err
	}
	return path, nil
}

func (view MissionView) Rel(parts ...string) (string, error) {
	path, err := view.Join(parts...)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(filepath.Dir(view.Root.Path), path)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}

func (view MissionView) ProjectStatePath(path string) string {
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(strings.TrimSpace(path))))
	for _, stateDir := range []string{CurrentDir, LegacyDir} {
		if clean != stateDir && !strings.HasPrefix(clean, stateDir+"/") {
			continue
		}
		suffix := strings.TrimPrefix(clean, stateDir)
		if view.Generation == 1 {
			return view.Root.Dir + suffix
		}
		trimmed := strings.TrimPrefix(suffix, "/")
		first := trimmed
		if separator := strings.IndexByte(first, '/'); separator >= 0 {
			first = first[:separator]
		}
		if MissionScopedName(first) {
			return filepath.ToSlash(filepath.Join(view.Root.Dir, MissionsDir, missionGenerationDir(view.Generation), filepath.FromSlash(trimmed)))
		}
		return view.Root.Dir + suffix
	}
	return path
}

func MissionScopedName(name string) bool {
	switch strings.TrimSpace(name) {
	case "mission-intent.json", "board.json", "policy.yml", "lanes", "facts", "runs", "reviews", "reviewer-adoptions", "handovers", "verifications", "external-session-attempts", "external-session-attempt-inputs", "external-session-dispatch", "external-session-jobs", "external-session-relays", "external-session-observations", "external-session-transport", "reopen-operations":
		return true
	default:
		return false
	}
}

func MissionGenerationDir(generation int) (string, error) {
	if generation < 2 || generation > 999999 {
		return "", fmt.Errorf("successor mission generation must be between 2 and 999999")
	}
	return missionGenerationDir(generation), nil
}

func MissionGenerationPath(caseRoot string, generation int, parts ...string) (string, error) {
	root, err := Resolve(caseRoot)
	if err != nil {
		return "", err
	}
	if root.Legacy || !root.Existing {
		return "", fmt.Errorf("successor mission requires an existing current project")
	}
	dir, err := MissionGenerationDir(generation)
	if err != nil {
		return "", err
	}
	all := append([]string{MissionsDir, dir}, parts...)
	local, err := validateParts(all)
	if err != nil {
		return "", err
	}
	path := filepath.Join(root.Path, local)
	if err := validateContained(root.Path, path); err != nil {
		return "", err
	}
	return path, nil
}

func MissionActivePath(caseRoot string) (string, error) {
	return Join(caseRoot, MissionsDir, MissionActiveFile)
}

func MissionID(generation int, planSHA256 string) (string, error) {
	if _, err := MissionGenerationDir(generation); err != nil {
		return "", err
	}
	if !validMissionSHA256(planSHA256) {
		return "", fmt.Errorf("mission plan SHA-256 is invalid")
	}
	return fmt.Sprintf("mission-g%06d-%s", generation, strings.ToLower(planSHA256)[:16]), nil
}

func ValidateMissionActive(active MissionActive) error {
	return validateMissionActive(active)
}

func MarshalMissionCanonical(value any) ([]byte, error) {
	return json.MarshalIndent(value, "", "  ")
}

func MissionSHA256(data []byte) string {
	return missionSHA256(data)
}

func missionGenerationDir(generation int) string {
	return fmt.Sprintf("g%06d", generation)
}

func validateMissionActive(active MissionActive) error {
	if active.SchemaVersion != 1 || active.Kind != "active-mission-pointer" ||
		active.Generation < 2 || active.Generation > 999999 ||
		!missionIDPattern.MatchString(active.MissionID) ||
		strings.TrimSpace(active.ProjectID) == "" ||
		!validMissionSHA256(active.MissionIntentSHA256) ||
		!validMissionSHA256(active.ManifestSHA256) ||
		strings.TrimSpace(active.TransitionID) == "" ||
		!validMissionSHA256(active.TransitionCommitSHA) {
		return fmt.Errorf("active mission pointer is invalid")
	}
	if active.MissionID[:15] != fmt.Sprintf("mission-g%06d", active.Generation) {
		return fmt.Errorf("active mission pointer generation differs from mission identity")
	}
	return nil
}

func decodeMissionCanonical(data []byte, target any) error {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.More() {
		return fmt.Errorf("mission artifact contains trailing JSON")
	}
	canonical, err := MarshalMissionCanonical(target)
	if err != nil {
		return err
	}
	if string(data) != string(canonical) && string(data) != string(append(canonical, '\n')) {
		return fmt.Errorf("mission artifact is not canonical")
	}
	return nil
}

func validMissionSHA256(value string) bool {
	decoded, err := hex.DecodeString(strings.TrimSpace(value))
	return err == nil && len(decoded) == sha256.Size
}

func missionSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
