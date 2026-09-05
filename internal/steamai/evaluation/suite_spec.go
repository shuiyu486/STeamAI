package evaluation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
)

type SuiteSpec struct {
	SchemaVersion            int             `json:"schemaVersion"`
	Rubric                   BoundFile       `json:"rubric"`
	VerifiedLearningContract BoundFile       `json:"verifiedLearningContract"`
	Model                    string          `json:"model"`
	ClaudeCode               string          `json:"claudeCode"`
	Platform                 string          `json:"platform"`
	ToolProfile              string          `json:"toolProfile"`
	Slots                    []SuiteSpecSlot `json:"slots"`
	Identity                 string          `json:"identity"`
}

type SuiteSpecSlot struct {
	SlotID        string    `json:"slotId"`
	ExpectedClass string    `json:"expectedClass"`
	Scenario      BoundFile `json:"scenario"`
	ControlPatch  BoundFile `json:"controlPatch"`
}

type SuitePrepareRequest struct {
	Name                     string             `json:"name"`
	Rubric                   string             `json:"rubric"`
	RubricSHA256             string             `json:"rubricSha256"`
	VerifiedLearningContract string             `json:"verifiedLearningContract"`
	ContractSHA256           string             `json:"verifiedLearningContractSha256"`
	Model                    string             `json:"model"`
	ClaudeCode               string             `json:"claudeCode"`
	Platform                 string             `json:"platform"`
	ToolProfile              string             `json:"toolProfile"`
	Slots                    []SuitePrepareSlot `json:"slots"`
}

type SuitePrepareSlot struct {
	SlotID             string `json:"slotId"`
	ExpectedClass      string `json:"expectedClass"`
	Scenario           string `json:"scenario"`
	ScenarioSHA256     string `json:"scenarioSha256"`
	ControlPatch       string `json:"controlPatch"`
	ControlPatchSHA256 string `json:"controlPatchSha256"`
}

func DecodeSuitePrepareRequest(reader io.Reader) (SuitePrepareRequest, error) {
	var request SuitePrepareRequest
	decoder := json.NewDecoder(io.LimitReader(reader, maxRequestBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return SuitePrepareRequest{}, ErrInvalid
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return SuitePrepareRequest{}, ErrInvalid
	}
	return request, nil
}

func PrepareSuite(caseRoot string, request SuitePrepareRequest) (SuiteSpec, string, error) {
	if !suiteManifestPattern.MatchString(request.Name) || request.VerifiedLearningContract != "verified-learning.md" ||
		!shaPattern.MatchString(request.ContractSHA256) || !modelPattern.MatchString(request.Model) || request.ClaudeCode == "" ||
		request.Platform == "" || request.ToolProfile != ToolProfile() {
		return SuiteSpec{}, "", ErrInvalid
	}
	stateRoot, specsRoot, runsRoot, workRoot, err := evaluationRoots(caseRoot)
	if err != nil {
		return SuiteSpec{}, "", err
	}
	if err := requireEvaluationLayout(caseRoot, stateRoot, specsRoot, runsRoot, workRoot); err != nil {
		return SuiteSpec{}, "", err
	}
	contractPath := filepath.Join(stateRoot, "contracts", request.VerifiedLearningContract)
	if _, err := readBoundFile(contractPath, request.ContractSHA256); err != nil {
		return SuiteSpec{}, "", err
	}
	rubricPath, err := resolveBoundPath(specsRoot, request.Rubric)
	if err != nil {
		return SuiteSpec{}, "", err
	}
	if _, err := readBoundFile(rubricPath, request.RubricSHA256); err != nil {
		return SuiteSpec{}, "", err
	}
	spec := SuiteSpec{
		SchemaVersion: 1, Rubric: BoundFile{Path: request.Rubric, SHA256: request.RubricSHA256},
		VerifiedLearningContract: BoundFile{Path: request.VerifiedLearningContract, SHA256: request.ContractSHA256},
		Model:                    request.Model, ClaudeCode: request.ClaudeCode, Platform: request.Platform, ToolProfile: request.ToolProfile,
	}
	for _, slot := range request.Slots {
		scenarioPath, err := resolveBoundPath(specsRoot, slot.Scenario)
		if err != nil {
			return SuiteSpec{}, "", err
		}
		if _, err := readBoundFile(scenarioPath, slot.ScenarioSHA256); err != nil {
			return SuiteSpec{}, "", err
		}
		patchRoot := filepath.Join(stateRoot, "learnings", "patches")
		patchPath, err := resolveBoundPath(patchRoot, slot.ControlPatch)
		if err != nil {
			return SuiteSpec{}, "", err
		}
		if _, err := readBoundFile(patchPath, slot.ControlPatchSHA256); err != nil {
			return SuiteSpec{}, "", err
		}
		spec.Slots = append(spec.Slots, SuiteSpecSlot{
			SlotID: slot.SlotID, ExpectedClass: slot.ExpectedClass,
			Scenario:     BoundFile{Path: slot.Scenario, SHA256: slot.ScenarioSHA256},
			ControlPatch: BoundFile{Path: slot.ControlPatch, SHA256: slot.ControlPatchSHA256},
		})
	}
	target, err := resolveBoundPath(specsRoot, request.Name)
	if err != nil {
		return SuiteSpec{}, "", err
	}
	if err := WriteSuiteSpec(target, spec); err != nil {
		return SuiteSpec{}, "", err
	}
	data, err := readLimitedBundleFile(target)
	if err != nil {
		return SuiteSpec{}, "", err
	}
	validated, err := ValidateSuiteSpec(data)
	return validated, Hash(data), err
}

func ValidateSuiteSpec(data []byte) (SuiteSpec, error) {
	var spec SuiteSpec
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&spec); err != nil {
		return SuiteSpec{}, ErrInvalid
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return SuiteSpec{}, errors.New("suite spec 后存在额外 JSON")
	}
	if spec.SchemaVersion != 1 || spec.Rubric.Path == "" || !shaPattern.MatchString(spec.Rubric.SHA256) ||
		spec.VerifiedLearningContract.Path != "verified-learning.md" || !shaPattern.MatchString(spec.VerifiedLearningContract.SHA256) ||
		!modelPattern.MatchString(spec.Model) || spec.ClaudeCode == "" || spec.Platform == "" || spec.ToolProfile != ToolProfile() || len(spec.Slots) < 10 || len(spec.Slots) > 30 ||
		spec.Identity != SuiteSpecIdentity(spec) {
		return SuiteSpec{}, ErrInvalid
	}
	seen := map[string]bool{}
	seenPatches := map[string]bool{}
	seenPatchSHAs := map[string]bool{}
	classes := map[string]int{}
	for _, slot := range spec.Slots {
		if !idPattern.MatchString(slot.SlotID) || seen[slot.SlotID] || !validControlClass(slot.ExpectedClass) ||
			slot.Scenario.Path == "" || !validRelativeName(slot.Scenario.Path) || !shaPattern.MatchString(slot.Scenario.SHA256) ||
			slot.ControlPatch.Path == "" || !validRelativeName(slot.ControlPatch.Path) || !shaPattern.MatchString(slot.ControlPatch.SHA256) ||
			seenPatches[slot.ControlPatch.Path] || seenPatchSHAs[slot.ControlPatch.SHA256] {
			return SuiteSpec{}, ErrInvalid
		}
		seen[slot.SlotID] = true
		seenPatches[slot.ControlPatch.Path] = true
		seenPatchSHAs[slot.ControlPatch.SHA256] = true
		classes[slot.ExpectedClass]++
	}
	for _, class := range []string{"improvement", "neutral", "regression", "authorization-regression", "prettier-weaker-evidence"} {
		if classes[class] < 2 || classes[class] > 6 {
			return SuiteSpec{}, ErrInvalid
		}
	}
	return spec, nil
}

func WriteSuiteSpec(path string, spec SuiteSpec) error {
	if !suiteManifestPattern.MatchString(filepath.Base(path)) {
		return ErrInvalid
	}
	spec.Identity = SuiteSpecIdentity(spec)
	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return fmt.Errorf("编码 suite spec: %w", err)
	}
	data = append(data, '\n')
	if _, err := ValidateSuiteSpec(data); err != nil {
		return err
	}
	return writeImmutableJSONFile(path, data)
}

func SuiteSpecIdentity(spec SuiteSpec) string {
	copy := spec
	copy.Identity = ""
	copy.Slots = append([]SuiteSpecSlot(nil), copy.Slots...)
	sort.Slice(copy.Slots, func(i, j int) bool { return copy.Slots[i].SlotID < copy.Slots[j].SlotID })
	data, _ := json.Marshal(copy)
	return Hash(append(data, '\n'))
}

func suiteSpecSlot(spec SuiteSpec, id string) (SuiteSpecSlot, bool) {
	for _, slot := range spec.Slots {
		if slot.SlotID == id {
			return slot, true
		}
	}
	return SuiteSpecSlot{}, false
}
