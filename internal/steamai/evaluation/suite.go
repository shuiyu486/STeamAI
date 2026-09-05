package evaluation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

var suiteManifestPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,62}\.json$`)

type SuiteManifest struct {
	SchemaVersion            int         `json:"schemaVersion"`
	SuiteSpec                BoundFile   `json:"suiteSpec"`
	Rubric                   BoundFile   `json:"rubric"`
	VerifiedLearningContract BoundFile   `json:"verifiedLearningContract"`
	Model                    string      `json:"model"`
	ClaudeCode               string      `json:"claudeCode"`
	Platform                 string      `json:"platform"`
	ToolProfile              string      `json:"toolProfile"`
	Slots                    []SuiteSlot `json:"slots"`
	Identity                 string      `json:"identity"`
}

type SuiteSlot struct {
	SlotID            string `json:"slotId"`
	ExpectedClass     string `json:"expectedClass"`
	RunManifest       string `json:"runManifest"`
	RunManifestSHA    string `json:"runManifestSha256"`
	RunBundleIdentity string `json:"runBundleIdentity"`
	ObservedClass     string `json:"observedClass"`
}

type SuiteFinalizeRequest struct {
	Name            string              `json:"name"`
	SuiteSpec       string              `json:"suiteSpec"`
	SuiteSpecSHA256 string              `json:"suiteSpecSha256"`
	Slots           []SuiteFinalizeSlot `json:"slots"`
}

type SuiteFinalizeSlot struct {
	SlotID        string `json:"slotId"`
	ObservedClass string `json:"observedClass"`
}

func DecodeSuiteFinalizeRequest(reader io.Reader) (SuiteFinalizeRequest, error) {
	var request SuiteFinalizeRequest
	decoder := json.NewDecoder(io.LimitReader(reader, maxRequestBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return SuiteFinalizeRequest{}, ErrInvalid
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return SuiteFinalizeRequest{}, ErrInvalid
	}
	return request, nil
}

func FinalizeSuite(caseRoot string, request SuiteFinalizeRequest) (VerifiedSuite, error) {
	if !suiteManifestPattern.MatchString(request.Name) || !suiteManifestPattern.MatchString(request.SuiteSpec) ||
		!shaPattern.MatchString(request.SuiteSpecSHA256) {
		return VerifiedSuite{}, ErrInvalid
	}
	stateRoot, specsRoot, runsRoot, workRoot, err := evaluationRoots(caseRoot)
	if err != nil {
		return VerifiedSuite{}, err
	}
	if err := requireEvaluationLayout(caseRoot, stateRoot, specsRoot, runsRoot, workRoot); err != nil {
		return VerifiedSuite{}, err
	}
	specPath, err := resolveBoundPath(specsRoot, request.SuiteSpec)
	if err != nil {
		return VerifiedSuite{}, err
	}
	specData, err := readBoundFile(specPath, request.SuiteSpecSHA256)
	if err != nil {
		return VerifiedSuite{}, err
	}
	spec, err := ValidateSuiteSpec(specData)
	if err != nil || len(request.Slots) != len(spec.Slots) {
		return VerifiedSuite{}, ErrInvalid
	}
	observed := map[string]string{}
	for _, slot := range request.Slots {
		if !idPattern.MatchString(slot.SlotID) || observed[slot.SlotID] != "" || !validObservedClass(slot.ObservedClass) {
			return VerifiedSuite{}, ErrInvalid
		}
		observed[slot.SlotID] = slot.ObservedClass
	}
	suite := SuiteManifest{
		SchemaVersion: 1, SuiteSpec: BoundFile{Path: request.SuiteSpec, SHA256: request.SuiteSpecSHA256},
		Rubric: spec.Rubric, VerifiedLearningContract: spec.VerifiedLearningContract,
		Model: spec.Model, ClaudeCode: spec.ClaudeCode, Platform: spec.Platform, ToolProfile: spec.ToolProfile,
	}
	for _, slot := range spec.Slots {
		observedClass, ok := observed[slot.SlotID]
		if !ok {
			return VerifiedSuite{}, ErrInvalid
		}
		rel := slot.SlotID + "/manifest.json"
		bundle, err := VerifyBundle(runsRoot, rel, "calibration", slot.ControlPatch.SHA256, false)
		if err != nil || bundle.Manifest.RunID != slot.SlotID || bundle.Manifest.SlotID != slot.SlotID ||
			bundle.Manifest.ExpectedClass != slot.ExpectedClass || bundle.Manifest.SuiteSpec != suite.SuiteSpec ||
			bundle.Manifest.Scenario != slot.Scenario || bundle.Manifest.Rubric != spec.Rubric ||
			bundle.Manifest.VerifiedLearningContract != spec.VerifiedLearningContract {
			return VerifiedSuite{}, ErrInvalid
		}
		suite.Slots = append(suite.Slots, SuiteSlot{
			SlotID: slot.SlotID, ExpectedClass: slot.ExpectedClass, RunManifest: rel,
			RunManifestSHA: bundle.ManifestSHA256, RunBundleIdentity: bundle.Manifest.Identity, ObservedClass: observedClass,
		})
	}
	target, err := resolveBoundPath(runsRoot, request.Name)
	if err != nil {
		return VerifiedSuite{}, err
	}
	if err := WriteSuiteManifest(target, suite); err != nil {
		return VerifiedSuite{}, err
	}
	return VerifySuiteClosure(runsRoot, request.Name)
}

type VerifiedSuite struct {
	Manifest       SuiteManifest
	Spec           SuiteSpec
	ManifestData   []byte
	ManifestSHA256 string
}

func VerifySuite(runsRoot, relativeManifest string) (VerifiedSuite, error) {
	return verifySuite(runsRoot, relativeManifest, true)
}

func VerifySuiteClosure(runsRoot, relativeManifest string) (VerifiedSuite, error) {
	return verifySuite(runsRoot, relativeManifest, false)
}

func verifySuite(runsRoot, relativeManifest string, requireGo bool) (VerifiedSuite, error) {
	path, err := resolveBoundPath(runsRoot, relativeManifest)
	if err != nil || !suiteManifestPattern.MatchString(relativeManifest) {
		return VerifiedSuite{}, ErrInvalid
	}
	data, err := readLimitedBundleFile(path)
	if err != nil {
		return VerifiedSuite{}, err
	}
	var suite SuiteManifest
	if err := strictSuiteJSON(data, &suite); err != nil || suite.SchemaVersion != 1 || suite.SuiteSpec.Path == "" ||
		!shaPattern.MatchString(suite.SuiteSpec.SHA256) || suite.Rubric.Path == "" || !shaPattern.MatchString(suite.Rubric.SHA256) ||
		suite.VerifiedLearningContract.Path != "verified-learning.md" || !shaPattern.MatchString(suite.VerifiedLearningContract.SHA256) ||
		!modelPattern.MatchString(suite.Model) || suite.ClaudeCode == "" || suite.Platform == "" || suite.ToolProfile != ToolProfile() ||
		len(suite.Slots) < 10 || suite.Identity != SuiteIdentity(suite) {
		return VerifiedSuite{}, ErrInvalid
	}
	specPath, err := resolveBoundPath(filepath.Join(filepath.Dir(runsRoot), "specs"), suite.SuiteSpec.Path)
	if err != nil {
		return VerifiedSuite{}, ErrInvalid
	}
	specData, err := readBoundFile(specPath, suite.SuiteSpec.SHA256)
	if err != nil {
		return VerifiedSuite{}, ErrInvalid
	}
	spec, err := ValidateSuiteSpec(specData)
	if err != nil || spec.Rubric != suite.Rubric || spec.VerifiedLearningContract != suite.VerifiedLearningContract ||
		spec.Model != suite.Model || spec.ClaudeCode != suite.ClaudeCode || spec.Platform != suite.Platform ||
		spec.ToolProfile != suite.ToolProfile || len(spec.Slots) != len(suite.Slots) {
		return VerifiedSuite{}, ErrInvalid
	}
	stateRoot := filepath.Dir(filepath.Dir(runsRoot))
	specsRoot := filepath.Join(stateRoot, "evaluations", "specs")
	if rubricPath, pathErr := resolveBoundPath(specsRoot, spec.Rubric.Path); pathErr != nil {
		return VerifiedSuite{}, ErrInvalid
	} else if _, readErr := readBoundFile(rubricPath, spec.Rubric.SHA256); readErr != nil {
		return VerifiedSuite{}, ErrInvalid
	}
	contractPath := filepath.Join(stateRoot, "contracts", spec.VerifiedLearningContract.Path)
	if _, err := readBoundFile(contractPath, spec.VerifiedLearningContract.SHA256); err != nil {
		return VerifiedSuite{}, ErrInvalid
	}
	patchesRoot := filepath.Join(stateRoot, "learnings", "patches")
	for _, slot := range spec.Slots {
		scenarioPath, pathErr := resolveBoundPath(specsRoot, slot.Scenario.Path)
		if pathErr != nil {
			return VerifiedSuite{}, ErrInvalid
		}
		if _, readErr := readBoundFile(scenarioPath, slot.Scenario.SHA256); readErr != nil {
			return VerifiedSuite{}, ErrInvalid
		}
		patchPath, pathErr := resolveBoundPath(patchesRoot, slot.ControlPatch.Path)
		if pathErr != nil {
			return VerifiedSuite{}, ErrInvalid
		}
		if _, readErr := readBoundFile(patchPath, slot.ControlPatch.SHA256); readErr != nil {
			return VerifiedSuite{}, ErrInvalid
		}
	}
	seenSlots := map[string]bool{}
	seenRuns := map[string]bool{}
	classes := map[string]int{}
	entries, err := os.ReadDir(runsRoot)
	if err != nil {
		return VerifiedSuite{}, ErrInvalid
	}
	calibrationRuns := map[string]bool{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifestPath := filepath.Join(runsRoot, entry.Name(), "manifest.json")
		manifestData, readErr := readLimitedBundleFile(manifestPath)
		if _, statErr := os.Lstat(manifestPath); os.IsNotExist(statErr) {
			continue
		}
		if readErr != nil {
			return VerifiedSuite{}, ErrInvalid
		}
		var manifest BundleManifest
		if strictBundleJSON(manifestData, &manifest) != nil {
			return VerifiedSuite{}, ErrInvalid
		}
		if manifest.Purpose == "calibration" && manifest.SuiteSpec == suite.SuiteSpec {
			calibrationRuns[entry.Name()+"/manifest.json"] = true
		}
	}
	for _, slot := range suite.Slots {
		if !idPattern.MatchString(slot.SlotID) || seenSlots[slot.SlotID] || seenRuns[slot.RunManifest] || !validControlClass(slot.ExpectedClass) ||
			!validObservedClass(slot.ObservedClass) || !shaPattern.MatchString(slot.RunManifestSHA) || !shaPattern.MatchString(slot.RunBundleIdentity) {
			return VerifiedSuite{}, ErrInvalid
		}
		expectedSlot, ok := suiteSpecSlot(spec, slot.SlotID)
		if !ok || expectedSlot.ExpectedClass != slot.ExpectedClass {
			return VerifiedSuite{}, ErrInvalid
		}
		if slot.RunManifest != slot.SlotID+"/manifest.json" {
			return VerifiedSuite{}, ErrInvalid
		}
		seenSlots[slot.SlotID] = true
		seenRuns[slot.RunManifest] = true
		classes[slot.ExpectedClass]++
		rel, err := RelativeManifestPath(slot.RunManifest)
		if err != nil {
			return VerifiedSuite{}, ErrInvalid
		}
		bundle, err := VerifyBundle(runsRoot, rel, "calibration", expectedSlot.ControlPatch.SHA256, requireGo)
		if err != nil || bundle.ManifestSHA256 != slot.RunManifestSHA || bundle.Manifest.Identity != slot.RunBundleIdentity ||
			bundle.Manifest.RunID != slot.SlotID || bundle.Manifest.SlotID != slot.SlotID || bundle.Manifest.ExpectedClass != slot.ExpectedClass ||
			bundle.Manifest.SuiteSpec != suite.SuiteSpec || bundle.Manifest.Scenario != expectedSlot.Scenario || bundle.Manifest.Rubric != suite.Rubric ||
			bundle.Manifest.VerifiedLearningContract != suite.VerifiedLearningContract {
			return VerifiedSuite{}, ErrInvalid
		}
		for _, arm := range bundle.Manifest.Arms {
			recordData, err := readBundleMember(filepath.Join(runsRoot, filepath.Dir(slot.RunManifest)), arm.Record, arm.RecordSHA256)
			if err != nil {
				return VerifiedSuite{}, ErrInvalid
			}
			var record RunRecord
			if strictBundleJSON(recordData, &record) != nil || record.Runtime.Model != suite.Model || record.Runtime.ClaudeCode != suite.ClaudeCode ||
				record.Runtime.OS != suite.Platform || record.Runtime.ToolProfile != suite.ToolProfile {
				return VerifiedSuite{}, ErrInvalid
			}
		}
	}
	if len(calibrationRuns) != len(seenRuns) {
		return VerifiedSuite{}, ErrInvalid
	}
	for run := range calibrationRuns {
		if !seenRuns[run] {
			return VerifiedSuite{}, ErrInvalid
		}
	}
	for _, class := range []string{"improvement", "neutral", "regression", "authorization-regression", "prettier-weaker-evidence"} {
		if classes[class] < 2 {
			return VerifiedSuite{}, ErrInvalid
		}
	}
	if requireGo && !calibrationPasses(suite.Slots) {
		return VerifiedSuite{}, ErrInvalid
	}
	return VerifiedSuite{Manifest: suite, Spec: spec, ManifestData: data, ManifestSHA256: Hash(data)}, nil
}

func SuiteIdentity(suite SuiteManifest) string {
	copy := suite
	copy.Identity = ""
	copy.Slots = append([]SuiteSlot(nil), copy.Slots...)
	sort.Slice(copy.Slots, func(i, j int) bool { return copy.Slots[i].SlotID < copy.Slots[j].SlotID })
	data, _ := json.Marshal(copy)
	return Hash(append(data, '\n'))
}

func calibrationPasses(slots []SuiteSlot) bool {
	for _, slot := range slots {
		switch slot.ExpectedClass {
		case "improvement":
			if slot.ObservedClass != "improved" {
				return false
			}
		case "neutral":
			if slot.ObservedClass != "neutral" {
				return false
			}
		case "regression", "prettier-weaker-evidence":
			if slot.ObservedClass != "regressed" && slot.ObservedClass != "rejected" {
				return false
			}
		case "authorization-regression":
			if slot.ObservedClass != "rejected" {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func validObservedClass(value string) bool {
	switch value {
	case "improved", "neutral", "regressed", "rejected", "inconclusive":
		return true
	default:
		return false
	}
}

func validControlClass(value string) bool {
	switch value {
	case "improvement", "neutral", "regression", "authorization-regression", "prettier-weaker-evidence":
		return true
	default:
		return false
	}
}

func strictSuiteJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("suite manifest 后存在额外 JSON")
	}
	return nil
}

func BundleResults(runsRoot string, bundle BundleManifest) ([]ResultRecord, error) {
	results := make([]ResultRecord, 0, len(bundle.Arms))
	for _, arm := range bundle.Arms {
		recordData, err := readBundleMember(filepath.Join(runsRoot, bundle.RunID), arm.Record, arm.RecordSHA256)
		if err != nil {
			return nil, ErrInvalid
		}
		var record RunRecord
		if strictBundleJSON(recordData, &record) != nil || record.ArmLabel != arm.Label {
			return nil, ErrInvalid
		}
		results = append(results, record.Result)
	}
	return results, nil
}

func VerifiedBundleRuntime(runsRoot string, bundle BundleManifest) (RuntimeIdentity, error) {
	var runtimeIdentity RuntimeIdentity
	for _, arm := range bundle.Arms {
		recordData, err := readBundleMember(filepath.Join(runsRoot, bundle.RunID), arm.Record, arm.RecordSHA256)
		if err != nil {
			return RuntimeIdentity{}, ErrInvalid
		}
		var record RunRecord
		if strictBundleJSON(recordData, &record) != nil || record.Runtime.Model == "" || record.Runtime.ToolProfile != ToolProfile() {
			return RuntimeIdentity{}, ErrInvalid
		}
		if runtimeIdentity == (RuntimeIdentity{}) {
			runtimeIdentity = record.Runtime
		} else if record.Runtime != runtimeIdentity {
			return RuntimeIdentity{}, ErrInvalid
		}
	}
	if runtimeIdentity == (RuntimeIdentity{}) {
		return RuntimeIdentity{}, ErrInvalid
	}
	return runtimeIdentity, nil
}

func WriteSuiteManifest(path string, suite SuiteManifest) error {
	if !suiteManifestPattern.MatchString(filepath.Base(path)) {
		return ErrInvalid
	}
	suite.Identity = SuiteIdentity(suite)
	data, err := json.MarshalIndent(suite, "", "  ")
	if err != nil {
		return fmt.Errorf("编码 suite manifest: %w", err)
	}
	data = append(data, '\n')
	var decoded SuiteManifest
	if err := strictSuiteJSON(data, &decoded); err != nil || decoded.Identity != SuiteIdentity(decoded) {
		return ErrInvalid
	}
	return writeImmutableJSONFile(path, data)
}

func writeImmutableJSONFile(path string, data []byte) error {
	parent := filepath.Dir(path)
	if err := requirePlainPath(parent, parent, true); err != nil {
		return ErrInvalid
	}
	temporary, err := os.CreateTemp(parent, ".steamai-evaluation-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Link(temporaryPath, path); err != nil {
		return err
	}
	if err := os.Remove(temporaryPath); err != nil {
		return err
	}
	cleanup = false
	return nil
}
