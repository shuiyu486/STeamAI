package evaluation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
)

const maxBundleFileBytes = 1 << 20

var manifestPathPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,62}/manifest\.json$`)

func validSuiteSpecBinding(purpose string, binding BoundFile) bool {
	if purpose == "calibration" {
		return binding.Path != "" && shaPattern.MatchString(binding.SHA256)
	}
	return binding == (BoundFile{})
}

func validRunSlot(purpose, slotID, expectedClass string) bool {
	if purpose == "calibration" {
		return idPattern.MatchString(slotID) && validControlClass(expectedClass)
	}
	return purpose == "candidate" && slotID == "" && expectedClass == ""
}

type VerifiedBundle struct {
	Manifest       BundleManifest
	ManifestData   []byte
	ManifestSHA256 string
	ReviewPacket   BlindReviewPacket
}

func VerifyBundle(runsRoot, relativeManifest, expectedPurpose, expectedPatchSHA string, requireCompleted bool) (VerifiedBundle, error) {
	rootInfo, rootErr := os.Lstat(runsRoot)
	if rootErr != nil || !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 || rejectReparse(runsRoot) != nil {
		return VerifiedBundle{}, ErrInvalid
	}
	manifestPath, err := resolveBoundPath(runsRoot, relativeManifest)
	if err != nil || !manifestPathPattern.MatchString(relativeManifest) {
		return VerifiedBundle{}, ErrInvalid
	}
	if err := requirePlainTree(runsRoot, filepath.Dir(manifestPath)); err != nil {
		return VerifiedBundle{}, ErrInvalid
	}
	manifestData, err := readLimitedBundleFile(manifestPath)
	if err != nil {
		return VerifiedBundle{}, err
	}
	var manifest BundleManifest
	if err := strictBundleJSON(manifestData, &manifest); err != nil || manifest.SchemaVersion != 1 || !idPattern.MatchString(manifest.RunID) ||
		manifest.Purpose != expectedPurpose || !validRunSlot(manifest.Purpose, manifest.SlotID, manifest.ExpectedClass) ||
		!validSuiteSpecBinding(manifest.Purpose, manifest.SuiteSpec) || manifest.Scenario.Path == "" || !shaPattern.MatchString(manifest.Scenario.SHA256) ||
		manifest.Rubric.Path == "" || !shaPattern.MatchString(manifest.Rubric.SHA256) ||
		manifest.VerifiedLearningContract.Path != "verified-learning.md" || !shaPattern.MatchString(manifest.VerifiedLearningContract.SHA256) || len(manifest.Arms) != 2 ||
		manifest.ReviewPacket.Path != "blind-review.json" || !shaPattern.MatchString(manifest.ReviewPacket.SHA256) ||
		!shaPattern.MatchString(manifest.RevealSHA256) ||
		manifest.Identity != BundleIdentity(manifest) || filepath.Base(filepath.Dir(manifestPath)) != manifest.RunID {
		return VerifiedBundle{}, ErrInvalid
	}
	revealData, err := readLimitedBundleFile(filepath.Join(filepath.Dir(manifestPath), "reveal.json"))
	if err != nil {
		return VerifiedBundle{}, err
	}
	var reveal RevealRecord
	if err := strictBundleJSON(revealData, &reveal); err != nil || Hash(revealData) != manifest.RevealSHA256 ||
		reveal.SchemaVersion != 1 || reveal.RunID != manifest.RunID || reveal.BlindIdentity != BlindBundleIdentity(manifest) ||
		reveal.BaselineArm == reveal.CandidateArm || !shaPattern.MatchString(reveal.CandidatePatchSHA256) ||
		(expectedPatchSHA != "" && reveal.CandidatePatchSHA256 != expectedPatchSHA) {
		return VerifiedBundle{}, ErrInvalid
	}
	manifest.Reveal = &reveal
	if !shaPattern.MatchString(reveal.CommitmentNonce) || !shaPattern.MatchString(reveal.BaselinePackSHA256) ||
		!shaPattern.MatchString(reveal.CandidatePackSHA256) || reveal.BaselinePackSHA256 == reveal.CandidatePackSHA256 {
		return VerifiedBundle{}, ErrInvalid
	}
	packetData, err := readBundleMember(filepath.Dir(manifestPath), manifest.ReviewPacket.Path, manifest.ReviewPacket.SHA256)
	if err != nil {
		return VerifiedBundle{}, err
	}
	var packet BlindReviewPacket
	if err := strictBundleJSON(packetData, &packet); err != nil || packet.SchemaVersion != 1 || packet.RunID != manifest.RunID || len(packet.Entries) != len(manifest.Arms) {
		return VerifiedBundle{}, ErrInvalid
	}
	orderedArms := append([]ArmRecord(nil), manifest.Arms...)
	sort.Slice(orderedArms, func(i, j int) bool { return orderedArms[i].Label < orderedArms[j].Label })
	entries := make(map[string]BlindReviewEntry, len(packet.Entries))
	seenLabels := map[string]bool{}
	for index, entry := range packet.Entries {
		expectedEntry := fmt.Sprintf("entry-%d", index)
		if entry.Entry != expectedEntry || entry.ArmLabel != orderedArms[index].Label || entries[entry.ArmLabel].Entry != "" {
			return VerifiedBundle{}, ErrInvalid
		}
		entries[entry.ArmLabel] = entry
	}
	for _, arm := range manifest.Arms {
		if arm.Label == "" || seenLabels[arm.Label] || !shaPattern.MatchString(arm.RecordSHA256) ||
			!shaPattern.MatchString(arm.OutputSHA256) || !shaPattern.MatchString(arm.StderrSHA256) ||
			filepath.Base(arm.Record) != arm.Record || filepath.Base(arm.Output) != arm.Output || filepath.Base(arm.Stderr) != arm.Stderr {
			return VerifiedBundle{}, ErrInvalid
		}
		seenLabels[arm.Label] = true
		recordData, err := readBundleMember(filepath.Dir(manifestPath), arm.Record, arm.RecordSHA256)
		if err != nil {
			return VerifiedBundle{}, err
		}
		outputData, err := readBundleMember(filepath.Dir(manifestPath), arm.Output, arm.OutputSHA256)
		if err != nil {
			return VerifiedBundle{}, err
		}
		stderrData, err := readBundleMember(filepath.Dir(manifestPath), arm.Stderr, arm.StderrSHA256)
		if err != nil {
			return VerifiedBundle{}, err
		}
		var record RunRecord
		if err := strictBundleJSON(recordData, &record); err != nil || record.SchemaVersion != 1 || record.RunID != manifest.RunID ||
			record.Purpose != manifest.Purpose || record.SlotID != manifest.SlotID || record.ExpectedClass != manifest.ExpectedClass ||
			record.SuiteSpec != manifest.SuiteSpec || record.ArmLabel != arm.Label || record.Scenario != manifest.Scenario || record.Rubric != manifest.Rubric ||
			record.VerifiedLearningContract != manifest.VerifiedLearningContract ||
			!shaPattern.MatchString(record.PackCommitment) || !modelPattern.MatchString(record.Runtime.Model) || record.Runtime.ClaudeCode == "" ||
			record.Runtime.OS == "" || record.Runtime.ToolProfile != ToolProfile() || record.Budget.MaxSeconds < 30 ||
			record.Budget.MaxSeconds > 3600 || record.Budget.MaxBudgetUSD <= 0 || record.Budget.MaxBudgetUSD > 100 ||
			record.Budget.ActualMillis < 0 || (record.Budget.ActualUSD != nil && *record.Budget.ActualUSD < 0) ||
			record.Result.OutputSHA256 != arm.OutputSHA256 || record.Result.OutputBytes != len(outputData) ||
			record.Result.StderrSHA256 != arm.StderrSHA256 || record.Result.StderrBytes != len(stderrData) ||
			(record.Result.SafetyGate != "pass" && record.Result.SafetyGate != "fail") {
			return VerifiedBundle{}, ErrInvalid
		}
		switch record.Result.Status {
		case "completed":
			safety, cost, err := validateModelOutput(outputData, record.Runtime.Model)
			if err != nil || record.Result.ExitCode != 0 || record.Result.SafetyGate != safety ||
				cost == nil || record.Budget.ActualUSD == nil || *cost != *record.Budget.ActualUSD || *cost > record.Budget.MaxBudgetUSD {
				return VerifiedBundle{}, ErrInvalid
			}
		case "failed":
			if record.Result.ExitCode == 0 || record.Result.Error == "" {
				return VerifiedBundle{}, ErrInvalid
			}
		case "timeout", "cancelled", "invalid-output":
			if record.Result.ExitCode != -1 || record.Result.Error == "" {
				return VerifiedBundle{}, ErrInvalid
			}
		default:
			return VerifiedBundle{}, ErrInvalid
		}
		if requireCompleted && (record.Result.Status != "completed" || record.Result.ExitCode != 0 || record.Result.SafetyGate != "pass") {
			return VerifiedBundle{}, ErrInvalid
		}
		var packSHA string
		switch arm.Label {
		case manifest.Reveal.BaselineArm:
			packSHA = manifest.Reveal.BaselinePackSHA256
		case manifest.Reveal.CandidateArm:
			packSHA = manifest.Reveal.CandidatePackSHA256
		default:
			return VerifiedBundle{}, ErrInvalid
		}
		if record.PackCommitment != packCommitment(manifest.Reveal.CommitmentNonce, arm.Label, packSHA) {
			return VerifiedBundle{}, ErrInvalid
		}
		entry, present := entries[arm.Label]
		expectedEntry, entryErr := blindReviewEntry(entry.Entry, arm, record, outputData)
		if !present || entryErr != nil || !reflect.DeepEqual(entry, expectedEntry) {
			return VerifiedBundle{}, ErrInvalid
		}
	}
	return VerifiedBundle{Manifest: manifest, ManifestData: manifestData, ManifestSHA256: Hash(manifestData), ReviewPacket: packet}, nil
}

func readBundleMember(root, name, expectedSHA string) ([]byte, error) {
	path := filepath.Join(root, name)
	inside, err := filepath.Rel(root, path)
	if err != nil || inside != name {
		return nil, ErrInvalid
	}
	data, err := readLimitedBundleFile(path)
	if err != nil {
		return nil, err
	}
	if Hash(data) != expectedSHA {
		return nil, ErrInvalid
	}
	return data, nil
}

func readLimitedBundleFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || rejectReparse(path) != nil {
		return nil, ErrInvalid
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxBundleFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxBundleFileBytes {
		return nil, errors.New("evaluation bundle 文件超过大小限制")
	}
	return data, nil
}

func strictBundleJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("JSON 后存在额外内容")
	}
	return nil
}

func RelativeManifestPath(path string) (string, error) {
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	if clean != path || strings.Contains(path, "\\") || !manifestPathPattern.MatchString(clean) {
		return "", ErrInvalid
	}
	return clean, nil
}
