package autonomy

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
	"strings"
	"time"

	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/lanemutation"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
)

const (
	ProfileOperationProvision = "provision"
	ProfileOperationRevoke    = "revoke"

	maxProvisionDuration = 15 * time.Minute
	maxProfileBytes      = 1 << 20
)

var (
	profileMutationLanePattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9._-]*[a-z0-9])?$`)
	exactProfilePlanSHA256     = regexp.MustCompile(`^[0-9a-f]{64}$`)

	profileMutationBeforeCommitHook func(ProfileMutationPlan) error
)

// ProfileProvisionOptions describes the exact strict profile to provision.
// The caller supplies grant timestamps so Preview remains stable and reviewable.
type ProfileProvisionOptions struct {
	RepoRoot string
	CaseRoot string
	Pack     string
	Lane     string
	Profile  Profile
}

// ProfileRevokeOptions identifies the lane whose profile will return to the
// canonical manual-gate default.
type ProfileRevokeOptions struct {
	RepoRoot string
	CaseRoot string
	Pack     string
	Lane     string
}

// ProfileMutationPlan is the review-first semantic binding consumed by a
// future CLI gate mode. ExpectedPlanSHA256 excludes presentation fields and
// binds the case, repo, pack, lane, exact current profile state, and planned
// strict profile.
type ProfileMutationPlan struct {
	SchemaVersion        int      `json:"schemaVersion"`
	Operation            string   `json:"operation"`
	RepoRoot             string   `json:"repoRoot"`
	CaseRoot             string   `json:"caseRoot"`
	Pack                 string   `json:"pack"`
	Lane                 string   `json:"lane"`
	ProfilePath          string   `json:"profilePath"`
	CurrentProfileExists bool     `json:"currentProfileExists"`
	CurrentProfileSHA256 string   `json:"currentProfileSha256,omitempty"`
	PlannedProfile       Profile  `json:"plannedProfile"`
	PlannedProfileSHA256 string   `json:"plannedProfileSha256"`
	ExpectedPlanSHA256   string   `json:"expectedPlanSha256"`
	IsMutation           bool     `json:"isMutation"`
	Replay               bool     `json:"replay,omitempty"`
	ReviewRequired       bool     `json:"reviewRequired"`
	RequiresConfirmation bool     `json:"requiresConfirmation"`
	Boundary             []string `json:"boundary"`

	currentBytes []byte
	plannedBytes []byte
}

// ProfileMutationResult reports whether the exact reviewed profile was
// published or was already current. It does not create a gate decision.
type ProfileMutationResult struct {
	Plan           ProfileMutationPlan `json:"plan"`
	Applied        bool                `json:"applied"`
	AlreadyApplied bool                `json:"alreadyApplied"`
	ProfileSHA256  string              `json:"profileSha256"`
}

type profileMutationBinding struct {
	SchemaVersion        int     `json:"schemaVersion"`
	Operation            string  `json:"operation"`
	RepoRoot             string  `json:"repoRoot"`
	CaseRoot             string  `json:"caseRoot"`
	Pack                 string  `json:"pack"`
	Lane                 string  `json:"lane"`
	ProfilePath          string  `json:"profilePath"`
	CurrentProfileExists bool    `json:"currentProfileExists"`
	CurrentProfileSHA256 string  `json:"currentProfileSha256"`
	PlannedProfile       Profile `json:"plannedProfile"`
	PlannedProfileSHA256 string  `json:"plannedProfileSha256"`
}

// PreviewProvision validates and binds a preauthorized profile without
// writing autonomy.json.
func PreviewProvision(opt ProfileProvisionOptions) (ProfileMutationPlan, error) {
	return buildProfileMutationPlan(
		ProfileOperationProvision,
		opt.RepoRoot,
		opt.CaseRoot,
		opt.Pack,
		opt.Lane,
		opt.Profile,
	)
}

// PreviewRevoke validates the current profile and previews an exact write of
// DefaultProfile(lane), without mutating autonomy.json.
func PreviewRevoke(opt ProfileRevokeOptions) (ProfileMutationPlan, error) {
	return buildProfileMutationPlan(
		ProfileOperationRevoke,
		opt.RepoRoot,
		opt.CaseRoot,
		opt.Pack,
		opt.Lane,
		DefaultProfile(opt.Lane),
	)
}

// ApplyProfilePlan acquires the lane mutation lease, rebuilds the plan from
// fresh durable state, checks the exact expected hash, and atomically replaces
// autonomy.json. A replay requires its own fresh Preview and is reported as
// AlreadyApplied without rewriting the file.
func ApplyProfilePlan(plan ProfileMutationPlan, expectedPlanSHA256 string) (_ ProfileMutationResult, retErr error) {
	if !exactProfilePlanSHA256.MatchString(expectedPlanSHA256) {
		return ProfileMutationResult{}, fmt.Errorf("autonomy profile Apply requires a valid exact expected plan sha256 from Preview")
	}
	if plan.ExpectedPlanSHA256 != expectedPlanSHA256 {
		return ProfileMutationResult{}, fmt.Errorf("autonomy profile expected plan sha256 does not match the supplied plan")
	}
	actualPlanSHA256, err := profileMutationPlanSHA256(plan)
	if err != nil || actualPlanSHA256 != expectedPlanSHA256 {
		return ProfileMutationResult{}, fmt.Errorf("autonomy profile supplied plan does not match its semantic sha256")
	}
	if plan.Operation != ProfileOperationProvision && plan.Operation != ProfileOperationRevoke {
		return ProfileMutationResult{}, fmt.Errorf("autonomy profile plan has unsupported operation: %s", plan.Operation)
	}

	lease, err := lanemutation.AcquireOpenLane(plan.CaseRoot, plan.Lane, "autonomy profile mutation")
	if err != nil {
		return ProfileMutationResult{}, fmt.Errorf("acquire autonomy profile lane mutation lease: %w", err)
	}
	defer func() {
		retErr = errors.Join(retErr, lease.Unlock())
	}()

	var fresh ProfileMutationPlan
	switch plan.Operation {
	case ProfileOperationProvision:
		fresh, err = buildProfileMutationPlan(
			ProfileOperationProvision,
			plan.RepoRoot,
			plan.CaseRoot,
			plan.Pack,
			plan.Lane,
			plan.PlannedProfile,
		)
	case ProfileOperationRevoke:
		fresh, err = buildProfileMutationPlan(
			ProfileOperationRevoke,
			plan.RepoRoot,
			plan.CaseRoot,
			plan.Pack,
			plan.Lane,
			DefaultProfile(plan.Lane),
		)
	}
	if err != nil {
		return ProfileMutationResult{}, err
	}
	if fresh.ExpectedPlanSHA256 != expectedPlanSHA256 {
		if plan.Replay || !fresh.Replay || !sameProfileMutationDesired(plan, fresh) {
			return ProfileMutationResult{}, fmt.Errorf("autonomy profile plan changed after Preview")
		}
	}
	if err := lease.Validate(); err != nil {
		return ProfileMutationResult{}, err
	}

	if fresh.Replay {
		if err := revalidateProfilePredecessor(fresh, lease); err != nil {
			return ProfileMutationResult{}, err
		}
		return ProfileMutationResult{
			Plan:           fresh,
			AlreadyApplied: true,
			ProfileSHA256:  fresh.CurrentProfileSHA256,
		}, nil
	}
	if err := replaceProfileAtomic(fresh, lease); err != nil {
		return ProfileMutationResult{}, err
	}
	return ProfileMutationResult{
		Plan:          fresh,
		Applied:       true,
		ProfileSHA256: fresh.PlannedProfileSHA256,
	}, nil
}

func buildProfileMutationPlan(operation, repoRoot, caseRoot, pack, lane string, planned Profile) (ProfileMutationPlan, error) {
	lane = strings.TrimSpace(lane)
	if lane == "" || !profileMutationLanePattern.MatchString(lane) {
		return ProfileMutationPlan{}, fmt.Errorf("invalid lane id for autonomy profile mutation: %q", lane)
	}
	if strings.TrimSpace(pack) == "" {
		return ProfileMutationPlan{}, fmt.Errorf("autonomy profile mutation requires pack")
	}

	repoRoot, err := filepath.Abs(strings.TrimSpace(repoRoot))
	if err != nil {
		return ProfileMutationPlan{}, err
	}
	caseRoot, err = filepath.Abs(strings.TrimSpace(caseRoot))
	if err != nil {
		return ProfileMutationPlan{}, err
	}
	inst, err := instance.AssertAttached(caseRoot, repoRoot, strings.TrimSpace(pack))
	if err != nil {
		return ProfileMutationPlan{}, err
	}
	m, err := manifest.Load(repoRoot, strings.TrimSpace(pack))
	if err != nil {
		return ProfileMutationPlan{}, err
	}
	repoRoot, caseRoot, pack = m.RepoRoot, inst.CaseRoot, m.Pack

	if err := validateProfileLane(caseRoot, lane); err != nil {
		return ProfileMutationPlan{}, err
	}
	current, currentBytes, currentExists, err := readCurrentProfileBytes(caseRoot, lane)
	if err != nil {
		return ProfileMutationPlan{}, err
	}
	if currentExists {
		if err := Validate(current, lane, m, caseRoot); err != nil {
			return ProfileMutationPlan{}, fmt.Errorf("current autonomy profile is invalid: %w", err)
		}
	}

	switch operation {
	case ProfileOperationProvision:
		if err := validateProvisionProfile(planned, lane, m, caseRoot, time.Now().UTC()); err != nil {
			return ProfileMutationPlan{}, err
		}
		sameDesired, err := profilesEqual(current, planned)
		if err != nil {
			return ProfileMutationPlan{}, err
		}
		if !currentExists {
			return ProfileMutationPlan{}, fmt.Errorf("autonomy profile provision requires an existing exact default manual-gate profile")
		}
		if !sameDesired {
			isDefault, err := profilesEqual(current, DefaultProfile(lane))
			if err != nil {
				return ProfileMutationPlan{}, err
			}
			if !isDefault {
				return ProfileMutationPlan{}, fmt.Errorf("autonomy profile provision refuses a non-default current profile")
			}
		}
	case ProfileOperationRevoke:
		planned = DefaultProfile(lane)
		if err := Validate(planned, lane, m, caseRoot); err != nil {
			return ProfileMutationPlan{}, err
		}
		if currentExists {
			isDefault, err := profilesEqual(current, planned)
			if err != nil {
				return ProfileMutationPlan{}, err
			}
			if !isDefault && strings.TrimSpace(current.Mode) != ModePreauthorized {
				return ProfileMutationPlan{}, fmt.Errorf("autonomy profile revoke refuses a current profile not owned by preauthorized provisioning")
			}
		}
	default:
		return ProfileMutationPlan{}, fmt.Errorf("unsupported autonomy profile operation: %s", operation)
	}

	plannedBytes, err := marshalProfile(planned)
	if err != nil {
		return ProfileMutationPlan{}, err
	}
	currentSHA := ""
	if currentExists {
		currentSHA = profileBytesSHA256(currentBytes)
	}
	plannedSHA := profileBytesSHA256(plannedBytes)
	replay := currentExists && bytes.Equal(currentBytes, plannedBytes)
	if !replay && currentExists {
		replay, err = profilesEqual(current, planned)
		if err != nil {
			return ProfileMutationPlan{}, err
		}
	}

	plan := ProfileMutationPlan{
		SchemaVersion:        1,
		Operation:            operation,
		RepoRoot:             repoRoot,
		CaseRoot:             caseRoot,
		Pack:                 pack,
		Lane:                 lane,
		ProfilePath:          RelPath(lane),
		CurrentProfileExists: currentExists,
		CurrentProfileSHA256: currentSHA,
		PlannedProfile:       planned,
		PlannedProfileSHA256: plannedSHA,
		IsMutation:           !replay,
		Replay:               replay,
		ReviewRequired:       true,
		RequiresConfirmation: !replay,
		Boundary: []string{
			"Preview writes nothing; Apply requires the exact reviewed semantic plan hash",
			"Apply rebuilds under the lane mutation lease and refuses current profile drift",
			"profile mutation does not create or authorize a gate decision and does not execute heavy tools",
		},
		currentBytes: append([]byte(nil), currentBytes...),
		plannedBytes: append([]byte(nil), plannedBytes...),
	}
	plan.ExpectedPlanSHA256, err = profileMutationPlanSHA256(plan)
	if err != nil {
		return ProfileMutationPlan{}, err
	}
	return plan, nil
}

func validateProvisionProfile(profile Profile, lane string, m *manifest.Manifest, caseRoot string, now time.Time) error {
	if profile.Mode != ModePreauthorized {
		return fmt.Errorf("autonomy profile provision permits only mode=%s", ModePreauthorized)
	}
	if profile.Lane != lane {
		return fmt.Errorf("autonomy profile provision requires exact lane %s", lane)
	}
	if err := Validate(profile, lane, m, caseRoot); err != nil {
		return err
	}

	declared := map[string]bool{}
	for _, action := range m.HeavyToolGateIDs() {
		declared[action] = true
	}
	if len(declared) == 0 {
		return fmt.Errorf("autonomy profile provision requires pack heavyToolGates")
	}
	for _, action := range append(append([]string{}, profile.AllowedActions...), profile.DeniedActions...) {
		if !declared[action] {
			return fmt.Errorf("autonomy profile action is not declared in pack heavyToolGates: %s", action)
		}
	}

	grantedAt, err := parseRequiredTime("grantedAt", profile.GrantedAt)
	if err != nil {
		return err
	}
	expiresAt, err := parseRequiredTime("expiresAt", profile.ExpiresAt)
	if err != nil {
		return err
	}
	if grantedAt.After(now) {
		return fmt.Errorf("autonomy profile grantedAt must not be in the future")
	}
	if !expiresAt.After(grantedAt) {
		return fmt.Errorf("autonomy profile expiresAt must be after grantedAt")
	}
	if expiresAt.Sub(grantedAt) > maxProvisionDuration {
		return fmt.Errorf("autonomy profile preauthorization duration exceeds %s", maxProvisionDuration)
	}
	if !now.Before(expiresAt) {
		return fmt.Errorf("autonomy profile provision refuses an expired profile")
	}
	return nil
}

func validateProfileLane(caseRoot, lane string) error {
	lanePath, err := refsf.SafeJoin(caseRoot, filepath.ToSlash(filepath.Join(".rekit", "lanes", lane, "lane.json")))
	if err != nil {
		return err
	}
	data, err := refsf.ReadStableRegularFileAnchored(caseRoot, lanePath, "autonomy profile lane", maxProfileBytes)
	if err != nil {
		return fmt.Errorf("read autonomy profile lane: %w", err)
	}
	var identity struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &identity); err != nil {
		return fmt.Errorf("invalid autonomy profile lane json: %w", err)
	}
	if identity.ID != lane {
		return fmt.Errorf("autonomy profile lane identity mismatch: got %s want %s", identity.ID, lane)
	}
	return nil
}

func readCurrentProfileBytes(caseRoot, lane string) (Profile, []byte, bool, error) {
	path, err := Path(caseRoot, lane)
	if err != nil {
		return Profile{}, nil, false, err
	}
	data, err := refsf.ReadStableRegularFileAnchored(caseRoot, path, "lane autonomy profile", maxProfileBytes)
	if errors.Is(err, os.ErrNotExist) {
		return DefaultProfile(lane), nil, false, nil
	}
	if err != nil {
		return Profile{}, nil, false, err
	}
	profile, err := decodeProfileBytes(data, path)
	if err != nil {
		return Profile{}, nil, true, err
	}
	return profile, data, true, nil
}

func decodeProfileBytes(data []byte, path string) (Profile, error) {
	var profile Profile
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&profile); err != nil {
		return Profile{}, fmt.Errorf("invalid autonomy profile %s: %w", path, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return Profile{}, fmt.Errorf("invalid autonomy profile %s: trailing data", path)
	}
	return profile, nil
}

func profilesEqual(left, right Profile) (bool, error) {
	leftBytes, err := marshalProfile(left)
	if err != nil {
		return false, err
	}
	rightBytes, err := marshalProfile(right)
	if err != nil {
		return false, err
	}
	return bytes.Equal(leftBytes, rightBytes), nil
}

func marshalProfile(profile Profile) ([]byte, error) {
	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func profileMutationPlanSHA256(plan ProfileMutationPlan) (string, error) {
	binding := profileMutationBinding{
		SchemaVersion:        plan.SchemaVersion,
		Operation:            plan.Operation,
		RepoRoot:             plan.RepoRoot,
		CaseRoot:             plan.CaseRoot,
		Pack:                 plan.Pack,
		Lane:                 plan.Lane,
		ProfilePath:          plan.ProfilePath,
		CurrentProfileExists: plan.CurrentProfileExists,
		CurrentProfileSHA256: plan.CurrentProfileSHA256,
		PlannedProfile:       plan.PlannedProfile,
		PlannedProfileSHA256: plan.PlannedProfileSHA256,
	}
	data, err := json.Marshal(binding)
	if err != nil {
		return "", err
	}
	return profileBytesSHA256(data), nil
}

func profileBytesSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func sameProfileMutationDesired(reviewed, fresh ProfileMutationPlan) bool {
	if reviewed.SchemaVersion != fresh.SchemaVersion ||
		reviewed.Operation != fresh.Operation ||
		reviewed.RepoRoot != fresh.RepoRoot ||
		reviewed.CaseRoot != fresh.CaseRoot ||
		reviewed.Pack != fresh.Pack ||
		reviewed.Lane != fresh.Lane ||
		reviewed.ProfilePath != fresh.ProfilePath ||
		reviewed.PlannedProfileSHA256 != fresh.PlannedProfileSHA256 {
		return false
	}
	equal, err := profilesEqual(reviewed.PlannedProfile, fresh.PlannedProfile)
	return err == nil && equal
}

func revalidateProfilePredecessor(plan ProfileMutationPlan, lease *lanemutation.Lease) error {
	current, currentBytes, exists, err := readCurrentProfileBytes(plan.CaseRoot, plan.Lane)
	if err != nil {
		return err
	}
	_ = current
	if exists != plan.CurrentProfileExists || !bytes.Equal(currentBytes, plan.currentBytes) {
		return fmt.Errorf("autonomy profile current bytes changed before replay completion")
	}
	if err := lease.Validate(); err != nil {
		return err
	}
	return nil
}

func replaceProfileAtomic(plan ProfileMutationPlan, lease *lanemutation.Lease) error {
	path, err := Path(plan.CaseRoot, plan.Lane)
	if err != nil {
		return err
	}
	parentPath := filepath.Dir(path)
	before, err := os.Lstat(parentPath)
	if err != nil || !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("autonomy profile parent must be a non-symlink directory: %s", parentPath)
	}
	parent, err := os.OpenRoot(parentPath)
	if err != nil {
		return err
	}
	defer parent.Close()
	opened, openErr := parent.Lstat(".")
	after, afterErr := os.Lstat(parentPath)
	if openErr != nil || afterErr != nil || !os.SameFile(before, opened) || !os.SameFile(opened, after) {
		return fmt.Errorf("autonomy profile parent changed while opening: %s", parentPath)
	}

	current, exists, err := readProfileRoot(parent, path)
	if err != nil {
		return err
	}
	if exists != plan.CurrentProfileExists || !bytes.Equal(current, plan.currentBytes) {
		return fmt.Errorf("autonomy profile current bytes changed before temporary publication")
	}
	if err := lease.Validate(); err != nil {
		return err
	}

	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	tempName := "." + profileFileName + ".profile-mutation-" + hex.EncodeToString(nonce) + ".tmp"
	file, err := parent.OpenFile(tempName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = parent.Remove(tempName)
		}
	}()
	written, writeErr := file.Write(plan.plannedBytes)
	syncErr := file.Sync()
	openedTemp, statErr := file.Stat()
	closeErr := file.Close()
	afterTemp, afterTempErr := parent.Lstat(tempName)
	if writeErr != nil || written != len(plan.plannedBytes) || syncErr != nil || statErr != nil || closeErr != nil || afterTempErr != nil || afterTemp.Mode()&os.ModeSymlink != 0 || !afterTemp.Mode().IsRegular() || !os.SameFile(openedTemp, afterTemp) {
		return fmt.Errorf("autonomy profile temporary publication failed: %w", errors.Join(writeErr, syncErr, statErr, closeErr, afterTempErr))
	}
	tempBytes, tempExists, err := readProfileRoot(parent, filepath.Join(parentPath, tempName))
	if err != nil || !tempExists || !bytes.Equal(tempBytes, plan.plannedBytes) {
		return fmt.Errorf("autonomy profile temporary bytes changed before commit: %w", err)
	}

	if profileMutationBeforeCommitHook != nil {
		if err := profileMutationBeforeCommitHook(plan); err != nil {
			return err
		}
	}
	current, exists, err = readProfileRoot(parent, path)
	if err != nil {
		return err
	}
	if exists != plan.CurrentProfileExists || !bytes.Equal(current, plan.currentBytes) {
		return fmt.Errorf("autonomy profile current bytes changed immediately before atomic replacement")
	}
	if err := lease.Validate(); err != nil {
		return err
	}
	if err := parent.Rename(tempName, profileFileName); err != nil {
		return fmt.Errorf("atomically replace autonomy profile: %w", err)
	}
	removeTemp = false

	published, exists, err := readProfileRoot(parent, path)
	if err != nil || !exists || !bytes.Equal(published, plan.plannedBytes) {
		return fmt.Errorf("published autonomy profile bytes differ after atomic replacement: %w", err)
	}
	if _, err := decodeProfileBytes(published, path); err != nil {
		return err
	}
	if err := lease.Validate(); err != nil {
		return err
	}
	return nil
}

func readProfileRoot(parent *os.Root, path string) ([]byte, bool, error) {
	name := filepath.Base(path)
	before, err := parent.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() || before.Size() < 1 || before.Size() > maxProfileBytes {
		return nil, false, fmt.Errorf("autonomy profile must be a bounded regular non-symlink file: %s", path)
	}
	file, err := parent.Open(name)
	if err != nil {
		return nil, false, err
	}
	opened, statErr := file.Stat()
	data, readErr := io.ReadAll(io.LimitReader(file, maxProfileBytes+1))
	closeErr := file.Close()
	after, afterErr := parent.Lstat(name)
	if statErr != nil || readErr != nil || closeErr != nil || afterErr != nil || !os.SameFile(before, opened) || !os.SameFile(opened, after) || int64(len(data)) != opened.Size() || len(data) > maxProfileBytes {
		return nil, false, fmt.Errorf("autonomy profile changed while reading: %s: %w", path, errors.Join(statErr, readErr, closeErr, afterErr))
	}
	return data, true, nil
}
