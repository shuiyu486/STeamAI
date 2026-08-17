package autonomy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

const (
	ModeManualGate    = "manual-gate"
	ModePreauthorized = "preauthorized"
	ModeAutonomous    = "autonomous"

	ManagedAutonomousPresetV1 = "bounded-autonomous-v1"

	DecisionManualConfirmationRequired = "manual-confirmation-required"
	DecisionPreauthorized              = "preauthorized"
	DecisionDenied                     = "denied"
	DecisionExpired                    = "expired"
	DecisionInvalidProfile             = "invalid-profile"
	DecisionOutOfScope                 = "out-of-scope"
	DecisionBudgetExceeded             = "budget-exceeded"
	DecisionOutputPathDenied           = "output-path-denied"
	DecisionStopConditionMismatch      = "stop-condition-mismatch"
)

const profileFileName = "autonomy.json"

var tokenPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[-_][a-z0-9]+)*$`)

type Profile struct {
	SchemaVersion  int      `json:"schemaVersion"`
	ProfileID      string   `json:"profileId"`
	Lane           string   `json:"lane"`
	Mode           string   `json:"mode"`
	AllowedActions []string `json:"allowedActions"`
	DeniedActions  []string `json:"deniedActions"`
	TargetScope    []Target `json:"targetScope"`
	Budget         Budget   `json:"budget"`
	StopConditions []string `json:"stopConditions"`
	OutputPaths    []string `json:"outputPaths"`
	RecordRequired bool     `json:"recordRequired"`
	NotifyMainOn   []string `json:"notifyMainOn"`
	GrantedBy      string   `json:"grantedBy,omitempty"`
	GrantedAt      string   `json:"grantedAt,omitempty"`
	ExpiresAt      string   `json:"expiresAt,omitempty"`
}

type Target struct {
	Match string `json:"match"`
	Value string `json:"value"`
}

type Budget struct {
	RuntimeSeconds int `json:"runtimeSeconds"`
	DiskMB         int `json:"diskMB"`
	Requests       int `json:"requests"`
}

type Request struct {
	Lane           string
	Action         string
	Target         string
	Budget         Budget
	StopConditions []string
	OutputPaths    []string
}

type Decision struct {
	Decision             string   `json:"decision"`
	Mode                 string   `json:"mode"`
	ProfileID            string   `json:"profileId,omitempty"`
	ProfilePath          string   `json:"profilePath,omitempty"`
	ProfileHash          string   `json:"profileHash,omitempty"`
	Source               string   `json:"source"`
	RequiresConfirmation bool     `json:"requiresConfirmation"`
	Reasons              []string `json:"reasons,omitempty"`
	AllowedActions       []string `json:"allowedActions,omitempty"`
	DeniedActions        []string `json:"deniedActions,omitempty"`
	TargetScope          []Target `json:"targetScope,omitempty"`
	Budget               Budget   `json:"budget"`
	StopConditions       []string `json:"stopConditions,omitempty"`
	OutputPaths          []string `json:"outputPaths,omitempty"`
	RecordRequired       bool     `json:"recordRequired"`
	NotifyMainOn         []string `json:"notifyMainOn,omitempty"`
	GrantedBy            string   `json:"grantedBy,omitempty"`
	GrantedAt            string   `json:"grantedAt,omitempty"`
	ExpiresAt            string   `json:"expiresAt,omitempty"`
}

type Summary struct {
	Mode           string   `json:"mode"`
	ProfileID      string   `json:"profileId,omitempty"`
	ProfilePath    string   `json:"profilePath,omitempty"`
	ProfileHash    string   `json:"profileHash,omitempty"`
	Ready          bool     `json:"ready"`
	Valid          bool     `json:"valid"`
	Expired        bool     `json:"expired"`
	Missing        bool     `json:"missing"`
	RecordRequired bool     `json:"recordRequired"`
	AllowedActions []string `json:"allowedActions,omitempty"`
	DeniedActions  []string `json:"deniedActions,omitempty"`
	OutputPaths    []string `json:"outputPaths,omitempty"`
	NotifyMainOn   []string `json:"notifyMainOn,omitempty"`
	ExpiresAt      string   `json:"expiresAt,omitempty"`
	Error          string   `json:"error,omitempty"`
}

// RelPath preserves the legacy .rekit-relative compatibility surface for
// callers without a case root. Current project code must use RelPathForCase.
func RelPath(laneID string) string {
	return filepath.ToSlash(filepath.Join(projectstate.LegacyDir, "lanes", strings.TrimSpace(laneID), profileFileName))
}

func RelPathForCase(caseRoot, laneID string) (string, error) {
	return projectstate.Rel(caseRoot, "lanes", strings.TrimSpace(laneID), profileFileName)
}

func Path(caseRoot, laneID string) (string, error) {
	rel, err := RelPathForCase(caseRoot, laneID)
	if err != nil {
		return "", err
	}
	return refsf.SafeJoin(caseRoot, rel)
}

func DefaultProfile(laneID string) Profile {
	laneID = strings.TrimSpace(laneID)
	return Profile{
		SchemaVersion:  1,
		ProfileID:      "manual-" + laneID,
		Lane:           laneID,
		Mode:           ModeManualGate,
		RecordRequired: true,
		NotifyMainOn:   []string{"boundary-hit", "new-risk", "destructive-change", "authority-write-needed"},
	}
}

func EnsureManualProfile(caseRoot, laneID string) (string, string, error) {
	rel, err := RelPathForCase(caseRoot, laneID)
	if err != nil {
		return "", "", err
	}
	path, err := Path(caseRoot, laneID)
	if err != nil {
		return "", "", err
	}
	if refsf.Exists(path) {
		return rel, path, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", "", err
	}
	if err := writeProfile(path, DefaultProfile(laneID)); err != nil {
		return "", "", err
	}
	return rel, path, nil
}

func Read(caseRoot, laneID string) (Profile, string, bool, error) {
	path, err := Path(caseRoot, laneID)
	if err != nil {
		return Profile{}, "", false, err
	}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return DefaultProfile(laneID), path, false, nil
	}
	if err != nil {
		return Profile{}, path, true, err
	}
	var profile Profile
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&profile); err != nil {
		return Profile{}, path, true, fmt.Errorf("invalid autonomy profile %s: %w", path, err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		if err == nil {
			return Profile{}, path, true, fmt.Errorf("invalid autonomy profile %s: trailing data", path)
		}
		return Profile{}, path, true, fmt.Errorf("invalid autonomy profile %s: trailing data: %w", path, err)
	}
	return profile, path, true, nil
}

func ReadSummary(caseRoot, laneID string, m *manifest.Manifest) Summary {
	rel, relErr := RelPathForCase(caseRoot, laneID)
	if relErr != nil {
		return Summary{Mode: ModeManualGate, Missing: true, Valid: false, Error: relErr.Error()}
	}
	profile, path, exists, err := Read(caseRoot, laneID)
	if err != nil {
		return Summary{Mode: ModeManualGate, ProfilePath: rel, Missing: !exists, Valid: false, Error: err.Error()}
	}
	if !exists {
		return Summary{Mode: ModeManualGate, ProfileID: profile.ProfileID, ProfilePath: rel, Missing: true, Valid: true, Ready: false, RecordRequired: profile.RecordRequired, NotifyMainOn: append([]string{}, profile.NotifyMainOn...)}
	}
	err = Validate(profile, laneID, m, caseRoot)
	mode := strings.TrimSpace(profile.Mode)
	expired := IsExpired(profile, time.Now().UTC())
	summary := Summary{
		Mode:           mode,
		ProfileID:      strings.TrimSpace(profile.ProfileID),
		ProfilePath:    rel,
		ProfileHash:    FileHash(path),
		Ready:          err == nil && !expired && (mode == ModePreauthorized || mode == ModeAutonomous),
		Valid:          err == nil,
		Expired:        expired,
		RecordRequired: profile.RecordRequired,
		AllowedActions: append([]string{}, profile.AllowedActions...),
		DeniedActions:  append([]string{}, profile.DeniedActions...),
		OutputPaths:    append([]string{}, profile.OutputPaths...),
		NotifyMainOn:   append([]string{}, profile.NotifyMainOn...),
		ExpiresAt:      strings.TrimSpace(profile.ExpiresAt),
	}
	if err != nil {
		summary.Error = err.Error()
	}
	return summary
}

func Validate(profile Profile, laneID string, m *manifest.Manifest, caseRoot string) error {
	mode := strings.TrimSpace(profile.Mode)
	if profile.SchemaVersion != 1 {
		return fmt.Errorf("autonomy profile schemaVersion has unsupported value: %d", profile.SchemaVersion)
	}
	if strings.TrimSpace(profile.ProfileID) == "" {
		return fmt.Errorf("autonomy profile is missing profileId")
	}
	if strings.TrimSpace(profile.Lane) == "" {
		return fmt.Errorf("autonomy profile is missing lane")
	}
	if !strings.EqualFold(strings.TrimSpace(profile.Lane), strings.TrimSpace(laneID)) {
		return fmt.Errorf("autonomy profile lane mismatch: got %s, want %s", profile.Lane, laneID)
	}
	switch mode {
	case ModeManualGate, ModePreauthorized, ModeAutonomous:
	default:
		return fmt.Errorf("autonomy profile mode has unsupported value: %s", profile.Mode)
	}
	if !profile.RecordRequired {
		return fmt.Errorf("autonomy profile must set recordRequired=true")
	}
	allowedManifest := map[string]bool{}
	if m != nil {
		for _, action := range m.HeavyToolGateIDs() {
			allowedManifest[action] = true
		}
	}
	allowed := map[string]bool{}
	for _, action := range profile.AllowedActions {
		action = strings.TrimSpace(action)
		if action == "" || !tokenPattern.MatchString(action) {
			return fmt.Errorf("autonomy profile allowedActions has invalid action: %s", action)
		}
		if len(allowedManifest) > 0 && !allowedManifest[action] {
			return fmt.Errorf("autonomy profile allowed action is not declared in pack heavyToolGates: %s", action)
		}
		if allowed[action] {
			return fmt.Errorf("autonomy profile allowedActions contains duplicate action: %s", action)
		}
		allowed[action] = true
	}
	denied := map[string]bool{}
	for _, action := range profile.DeniedActions {
		action = strings.TrimSpace(action)
		if action == "" || !tokenPattern.MatchString(action) {
			return fmt.Errorf("autonomy profile deniedActions has invalid action: %s", action)
		}
		if len(allowedManifest) > 0 && !allowedManifest[action] {
			return fmt.Errorf("autonomy profile denied action is not declared in pack heavyToolGates: %s", action)
		}
		if denied[action] {
			return fmt.Errorf("autonomy profile deniedActions contains duplicate action: %s", action)
		}
		if allowed[action] {
			return fmt.Errorf("autonomy profile action appears in both allowedActions and deniedActions: %s", action)
		}
		denied[action] = true
	}
	for _, target := range profile.TargetScope {
		if strings.TrimSpace(target.Match) != "exact" {
			return fmt.Errorf("autonomy profile targetScope only supports exact match: %s", target.Match)
		}
		if strings.TrimSpace(target.Value) == "" {
			return fmt.Errorf("autonomy profile targetScope contains an empty value")
		}
	}
	if err := validateBudget(profile.Budget, mode); err != nil {
		return err
	}
	if err := validateTokens("autonomy profile stopConditions", profile.StopConditions); err != nil {
		return err
	}
	if err := validateTokens("autonomy profile notifyMainOn", profile.NotifyMainOn); err != nil {
		return err
	}
	if err := validateOutputPaths(caseRoot, profile.OutputPaths); err != nil {
		return err
	}
	if mode == ModePreauthorized || mode == ModeAutonomous {
		if len(profile.AllowedActions) == 0 {
			return fmt.Errorf("autonomy profile %s mode requires allowedActions", mode)
		}
		if len(profile.TargetScope) == 0 {
			return fmt.Errorf("autonomy profile %s mode requires targetScope", mode)
		}
		if len(profile.OutputPaths) == 0 {
			return fmt.Errorf("autonomy profile %s mode requires outputPaths", mode)
		}
		if len(profile.StopConditions) == 0 {
			return fmt.Errorf("autonomy profile %s mode requires stopConditions", mode)
		}
		if len(profile.NotifyMainOn) == 0 {
			return fmt.Errorf("autonomy profile %s mode requires notifyMainOn", mode)
		}
		if strings.TrimSpace(profile.GrantedBy) == "" {
			return fmt.Errorf("autonomy profile %s mode requires grantedBy", mode)
		}
		if _, err := parseRequiredTime("grantedAt", profile.GrantedAt); err != nil {
			return err
		}
		if _, err := parseRequiredTime("expiresAt", profile.ExpiresAt); err != nil {
			return err
		}
	}
	if mode == ModeAutonomous {
		grantedAt, err := parseRequiredTime("grantedAt", profile.GrantedAt)
		if err != nil {
			return err
		}
		expiresAt, err := parseRequiredTime("expiresAt", profile.ExpiresAt)
		if err != nil {
			return err
		}
		if !expiresAt.After(grantedAt) {
			return fmt.Errorf("autonomy profile expiresAt must be after grantedAt")
		}
		if expiresAt.Sub(grantedAt) > maxProvisionDuration {
			return fmt.Errorf("autonomy profile autonomous duration exceeds %s", maxProvisionDuration)
		}
		if m == nil || strings.TrimSpace(caseRoot) == "" || !isManagedAutonomousProfileV1(profile, laneID, m, caseRoot) {
			return fmt.Errorf("autonomy profile mode=%s is outside managed preset %s", ModeAutonomous, ManagedAutonomousPresetV1)
		}
	}
	return nil
}

func Evaluate(profile Profile, profilePath string, profileExists bool, profileHash string, req Request, now time.Time, m *manifest.Manifest, caseRoot string) Decision {
	mode := strings.TrimSpace(profile.Mode)
	decision := Decision{
		Mode:                 firstText(mode, ModeManualGate),
		ProfileID:            strings.TrimSpace(profile.ProfileID),
		ProfilePath:          profilePath,
		ProfileHash:          profileHash,
		Source:               "lane-autonomy-profile",
		RequiresConfirmation: true,
		AllowedActions:       append([]string{}, profile.AllowedActions...),
		DeniedActions:        append([]string{}, profile.DeniedActions...),
		TargetScope:          append([]Target{}, profile.TargetScope...),
		Budget:               profile.Budget,
		StopConditions:       append([]string{}, profile.StopConditions...),
		OutputPaths:          append([]string{}, profile.OutputPaths...),
		RecordRequired:       profile.RecordRequired,
		NotifyMainOn:         append([]string{}, profile.NotifyMainOn...),
		GrantedBy:            strings.TrimSpace(profile.GrantedBy),
		GrantedAt:            strings.TrimSpace(profile.GrantedAt),
		ExpiresAt:            strings.TrimSpace(profile.ExpiresAt),
	}
	if !profileExists {
		decision.Decision = DecisionManualConfirmationRequired
		decision.Reasons = append(decision.Reasons, "lane autonomy profile is manual-gate")
		return decision
	}
	if mode != ModeManualGate && m == nil {
		decision.Decision = DecisionInvalidProfile
		decision.Reasons = append(decision.Reasons, "lane autonomy preauthorization requires the current pack manifest")
		return decision
	}
	if err := Validate(profile, profile.Lane, m, caseRoot); err != nil {
		decision.Decision = DecisionInvalidProfile
		decision.Reasons = append(decision.Reasons, err.Error())
		return decision
	}
	if mode == ModeManualGate {
		decision.Decision = DecisionManualConfirmationRequired
		decision.Reasons = append(decision.Reasons, "lane autonomy profile is manual-gate")
		return decision
	}
	if mode == ModeAutonomous {
		grantedAt, err := parseRequiredTime("grantedAt", profile.GrantedAt)
		if err != nil || grantedAt.After(now) {
			decision.Decision = DecisionInvalidProfile
			decision.Reasons = append(decision.Reasons, "managed autonomous profile grant is not current")
			return decision
		}
	}
	if strings.TrimSpace(req.Lane) == "" || !strings.EqualFold(strings.TrimSpace(profile.Lane), strings.TrimSpace(req.Lane)) {
		decision.Decision = DecisionOutOfScope
		decision.Reasons = append(decision.Reasons, "request lane does not exactly match lane autonomy profile")
		return decision
	}
	if IsExpired(profile, now) {
		decision.Decision = DecisionExpired
		decision.Reasons = append(decision.Reasons, "lane autonomy profile is expired")
		return decision
	}
	if mode == ModeAutonomous {
		expiresAt, _ := parseRequiredTime("expiresAt", profile.ExpiresAt)
		if time.Duration(req.Budget.RuntimeSeconds)*time.Second > expiresAt.Sub(now) {
			decision.Decision = DecisionExpired
			decision.Reasons = append(decision.Reasons, "requested runtime exceeds the remaining managed autonomous grant duration")
			return decision
		}
	}
	if slices.Contains(profile.DeniedActions, req.Action) {
		decision.Decision = DecisionDenied
		decision.Reasons = append(decision.Reasons, "action is explicitly denied by lane autonomy profile")
		return decision
	}
	if !slices.Contains(profile.AllowedActions, req.Action) {
		decision.Decision = DecisionDenied
		decision.Reasons = append(decision.Reasons, "action is not allowed by lane autonomy profile")
		return decision
	}
	if !targetAllowed(profile.TargetScope, req.Target) {
		decision.Decision = DecisionOutOfScope
		decision.Reasons = append(decision.Reasons, "target does not exactly match lane autonomy targetScope")
		return decision
	}
	if req.Budget.RuntimeSeconds <= 0 || req.Budget.DiskMB <= 0 || req.Budget.Requests <= 0 {
		decision.Decision = DecisionBudgetExceeded
		decision.Reasons = append(decision.Reasons, "requested runtimeSeconds, diskMB, and requests budgets must be positive for preauthorization")
		return decision
	}
	if exceeded, reason := budgetExceeded(profile.Budget, req.Budget); exceeded {
		decision.Decision = DecisionBudgetExceeded
		decision.Reasons = append(decision.Reasons, reason)
		return decision
	}
	actionGate, actionDeclared := m.HeavyToolGate(req.Action)
	if !actionDeclared || !stopConditionsAllowed(
		profile.StopConditions,
		actionGate.StopConditions,
		req.StopConditions,
	) {
		decision.Decision = DecisionStopConditionMismatch
		decision.Reasons = append(decision.Reasons, "requested stopConditions must include every current pack manifest condition and remain covered by lane autonomy profile")
		return decision
	}
	if err := validateOutputPaths(caseRoot, req.OutputPaths); err != nil {
		decision.Decision = DecisionOutputPathDenied
		decision.Reasons = append(decision.Reasons, err.Error())
		return decision
	}
	if !outputPathsAllowed(profile.OutputPaths, req.OutputPaths) {
		decision.Decision = DecisionOutputPathDenied
		decision.Reasons = append(decision.Reasons, "requested outputPaths are not covered by lane autonomy profile")
		return decision
	}
	decision.Decision = DecisionPreauthorized
	decision.RequiresConfirmation = false
	decision.Reasons = append(decision.Reasons, "request is covered by durable lane autonomy profile")
	return decision
}

func FileHash(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func IsExpired(profile Profile, now time.Time) bool {
	expires := strings.TrimSpace(profile.ExpiresAt)
	if expires == "" {
		return false
	}
	parsed, err := time.Parse(time.RFC3339, expires)
	if err != nil {
		return false
	}
	return !now.Before(parsed)
}

func writeProfile(path string, profile Profile) error {
	b, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func validateBudget(b Budget, mode string) error {
	if b.RuntimeSeconds < 0 || b.DiskMB < 0 || b.Requests < 0 {
		return fmt.Errorf("autonomy profile budget values must be non-negative")
	}
	if mode == ModePreauthorized || mode == ModeAutonomous {
		if b.RuntimeSeconds <= 0 || b.DiskMB <= 0 || b.Requests <= 0 {
			return fmt.Errorf("autonomy profile %s mode requires positive runtimeSeconds, diskMB, and requests budgets", mode)
		}
	}
	return nil
}

func validateTokens(field string, values []string) error {
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return fmt.Errorf("%s contains an empty item", field)
		}
		if !tokenPattern.MatchString(value) {
			return fmt.Errorf("%s has invalid item: %s", field, value)
		}
		if seen[value] {
			return fmt.Errorf("%s contains duplicate item: %s", field, value)
		}
		seen[value] = true
	}
	return nil
}

func validateOutputPaths(caseRoot string, paths []string) error {
	seen := map[string]bool{}
	for _, rel := range paths {
		rel = strings.TrimSpace(rel)
		if rel == "" {
			return fmt.Errorf("autonomy profile outputPaths contains an empty item")
		}
		if filepath.IsAbs(rel) {
			return fmt.Errorf("autonomy profile outputPaths must be case-relative: %s", rel)
		}
		if _, err := refsf.SafeJoin(caseRoot, rel); err != nil {
			return fmt.Errorf("autonomy profile outputPaths contains unsafe path %q: %w", rel, err)
		}
		key := filepath.ToSlash(filepath.Clean(filepath.FromSlash(rel)))
		if key == "." || key == ".." || strings.HasPrefix(key, "../") {
			return fmt.Errorf("autonomy profile outputPaths contains unsafe path: %s", rel)
		}
		if seen[key] {
			return fmt.Errorf("autonomy profile outputPaths contains duplicate path: %s", rel)
		}
		seen[key] = true
	}
	return nil
}

func parseRequiredTime(field, value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("autonomy profile requires %s", field)
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("autonomy profile %s must be RFC3339: %s", field, value)
	}
	return parsed, nil
}

func targetAllowed(scope []Target, target string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}
	for _, item := range scope {
		if strings.TrimSpace(item.Match) == "exact" && strings.TrimSpace(item.Value) == target {
			return true
		}
	}
	return false
}

func budgetExceeded(limit Budget, requested Budget) (bool, string) {
	if requested.RuntimeSeconds > limit.RuntimeSeconds {
		return true, "requested runtimeSeconds exceeds lane autonomy budget"
	}
	if requested.DiskMB > limit.DiskMB {
		return true, "requested diskMB exceeds lane autonomy budget"
	}
	if requested.Requests > limit.Requests {
		return true, "requested requests exceeds lane autonomy budget"
	}
	return false, ""
}

func stopConditionsAllowed(allowed, required, requested []string) bool {
	if len(required) == 0 || len(requested) == 0 {
		return false
	}
	allow := map[string]bool{}
	for _, item := range allowed {
		allow[strings.TrimSpace(item)] = true
	}
	request := map[string]bool{}
	for _, item := range requested {
		item = strings.TrimSpace(item)
		if !allow[item] {
			return false
		}
		request[item] = true
	}
	for _, item := range required {
		item = strings.TrimSpace(item)
		if !allow[item] || !request[item] {
			return false
		}
	}
	return true
}

func outputPathsAllowed(allowed, requested []string) bool {
	if len(requested) == 0 {
		return false
	}
	allow := normalizedPaths(allowed)
	for _, item := range normalizedPaths(requested) {
		matched := false
		for _, prefix := range allow {
			if item == prefix || strings.HasPrefix(item, strings.TrimRight(prefix, "/")+"/") {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func normalizedPaths(paths []string) []string {
	out := []string{}
	for _, item := range paths {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		out = append(out, filepath.ToSlash(filepath.Clean(filepath.FromSlash(item))))
	}
	sort.Strings(out)
	return out
}

func firstText(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
