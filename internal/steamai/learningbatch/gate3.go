package learningbatch

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/shuiyu486/STeamAI/internal/steamai/evaluation"
)

const maxVerifiedLearningFileBytes = 1 << 20

type blindDecisionBinding struct {
	Reviewer          string
	BlindIdentity     string
	PreferredArm      string
	ComparativeResult string
	HardSafetyGates   string
}

type attestationBinding struct {
	Reviewer               string
	Purpose                string
	CandidatePatch         string
	CandidatePatchSHA      string
	CandidateBlindDecision string
	CandidateBlindSHA      string
	BaselineArm            string
	CandidateArm           string
	SuiteIdentity          string
	SuiteSHA               string
	RunBundleManifest      string
	RunBundleManifestSHA   string
	RunBundleIdentity      string
	RunBundleReveal        string
	RunBundleRevealSHA     string
	HardSafetyGates        string
	ComparativeResult      string
	Maturity               string
	CalibrationAttestation string
	CalibrationSHA         string
	CalibrationDecision    string
}

func validateGate3(caseRoot string, request Request, binding batchBinding, reviewer, patchPath string, patchSHA string) (*AttestationRecord, *AttestationRecord, *RunBundleRecord, error) {
	if !batchRequiresV3Binding(binding.Candidates) {
		if request.CalibrationAttestation != "" || request.PromotionAttestation != "" || request.RunBundleManifest != "" ||
			binding.CalibrationAttestation != "" || binding.PromotionAttestation != "" || binding.RunBundleManifest != "" || binding.EvaluatedPatchSHA != "" {
			return nil, nil, nil, ErrBinding
		}
		return nil, nil, nil, nil
	}
	if request.CalibrationAttestation == "" || request.PromotionAttestation == "" || request.RunBundleManifest == "" ||
		binding.CalibrationAttestation != request.CalibrationAttestation || binding.PromotionAttestation != request.PromotionAttestation ||
		binding.RunBundleManifest != request.RunBundleManifest || binding.EvaluatedPatchSHA != patchSHA || binding.PatchSHA256 != patchSHA {
		return nil, nil, nil, ErrBinding
	}
	calibration, calibrationData, err := readAttestation(caseRoot, request.CalibrationAttestation)
	if err != nil {
		return nil, nil, nil, err
	}
	promotion, promotionData, err := readAttestation(caseRoot, request.PromotionAttestation)
	if err != nil {
		return nil, nil, nil, err
	}
	calibrationSHA := hashBytes(calibrationData)
	promotionSHA := hashBytes(promotionData)
	if binding.CalibrationAttestationSHA != calibrationSHA || binding.PromotionAttestationSHA != promotionSHA ||
		calibration.Reviewer != reviewer || promotion.Reviewer != reviewer || calibration.Purpose != "calibration" ||
		calibration.CalibrationDecision != "go" || promotion.Purpose != "candidate" ||
		promotion.HardSafetyGates != "pass" || promotion.ComparativeResult != "improved" || promotion.Maturity != "V3" ||
		promotion.CandidatePatch != patchPath || promotion.CandidatePatchSHA != patchSHA ||
		promotion.CalibrationAttestation != request.CalibrationAttestation || promotion.CalibrationSHA != calibrationSHA ||
		promotion.SuiteIdentity != calibration.SuiteIdentity || promotion.SuiteSHA != calibration.SuiteSHA {
		return nil, nil, nil, ErrBinding
	}
	runsRoot := filepath.Join(caseRoot, ".steamai-vnext", "evaluations", "runs")
	calibrationSuite, err := evaluation.VerifySuite(runsRoot, strings.TrimPrefix(calibration.RunBundleManifest, "evaluations/runs/"))
	if err != nil || calibration.RunBundleManifestSHA != calibrationSuite.ManifestSHA256 ||
		calibration.RunBundleIdentity != calibrationSuite.Manifest.Identity || calibration.SuiteIdentity != calibrationSuite.Manifest.Identity ||
		calibration.SuiteSHA != calibrationSuite.ManifestSHA256 {
		return nil, nil, nil, ErrBinding
	}
	if calibrationReusesFinalPatch(calibrationSuite.Spec, patchPath, patchSHA) {
		return nil, nil, nil, ErrBinding
	}
	promotionRel, err := runManifestRelative(request.RunBundleManifest)
	if err != nil {
		return nil, nil, nil, ErrBinding
	}
	manifest, err := evaluation.VerifyBundle(runsRoot, promotionRel, "candidate", patchSHA, true)
	if err != nil || promotion.CandidatePatch != patchPath || manifest.Manifest.Rubric != calibrationSuite.Manifest.Rubric ||
		manifest.Manifest.VerifiedLearningContract != calibrationSuite.Manifest.VerifiedLearningContract {
		return nil, nil, nil, ErrBinding
	}
	promotionRuntime, err := evaluation.VerifiedBundleRuntime(runsRoot, manifest.Manifest)
	if err != nil || promotionRuntime.Model != calibrationSuite.Manifest.Model || promotionRuntime.ClaudeCode != calibrationSuite.Manifest.ClaudeCode ||
		promotionRuntime.OS != calibrationSuite.Manifest.Platform || promotionRuntime.ToolProfile != calibrationSuite.Manifest.ToolProfile {
		return nil, nil, nil, ErrBinding
	}
	expectedReveal := strings.TrimSuffix(promotion.RunBundleManifest, "/manifest.json") + "/reveal.json"
	if binding.RunBundleManifestSHA != manifest.ManifestSHA256 || binding.RunBundleIdentity != manifest.Manifest.Identity ||
		binding.RunBundleRevealSHA != manifest.Manifest.RevealSHA256 || promotion.RunBundleManifest != request.RunBundleManifest ||
		promotion.RunBundleManifestSHA != manifest.ManifestSHA256 || promotion.RunBundleIdentity != manifest.Manifest.Identity ||
		promotion.RunBundleReveal != expectedReveal || promotion.RunBundleRevealSHA != manifest.Manifest.RevealSHA256 || manifest.Manifest.Reveal == nil ||
		promotion.BaselineArm != manifest.Manifest.Reveal.BaselineArm || promotion.CandidateArm != manifest.Manifest.Reveal.CandidateArm {
		return nil, nil, nil, ErrBinding
	}
	blind, err := readBlindDecision(caseRoot, promotion.CandidateBlindDecision, promotion.CandidateBlindSHA)
	if err != nil || blind.Reviewer != reviewer || blind.BlindIdentity != manifest.Manifest.Reveal.BlindIdentity ||
		blind.PreferredArm != promotion.CandidateArm || blind.ComparativeResult != "improved" || blind.HardSafetyGates != "pass" {
		return nil, nil, nil, ErrBinding
	}
	return &AttestationRecord{Path: request.CalibrationAttestation, SHA256: calibrationSHA},
		&AttestationRecord{Path: request.PromotionAttestation, SHA256: promotionSHA},
		&RunBundleRecord{Path: request.RunBundleManifest, SHA256: manifest.ManifestSHA256, Identity: manifest.Manifest.Identity, RevealSHA256: manifest.Manifest.RevealSHA256}, nil
}

func calibrationReusesFinalPatch(spec evaluation.SuiteSpec, patchPath, patchSHA string) bool {
	finalPatchName := strings.TrimPrefix(patchPath, "learnings/patches/")
	for _, slot := range spec.Slots {
		if slot.ControlPatch.Path == finalPatchName || slot.ControlPatch.SHA256 == patchSHA {
			return true
		}
	}
	return false
}

func batchRequiresV3Binding(candidates []batchCandidateBinding) bool {
	for _, candidate := range candidates {
		if candidate.RequiredMaturity == "V3" {
			return true
		}
	}
	return false
}

func readBlindDecision(caseRoot, rel, expectedSHA string) (blindDecisionBinding, error) {
	clean, err := cleanStateFile(rel, "evaluations/attestations/", blindDecisionPattern)
	if err != nil {
		return blindDecisionBinding{}, ErrBinding
	}
	path := filepath.Join(caseRoot, ".steamai-vnext", filepath.FromSlash(clean))
	if err := requirePlainPath(caseRoot, path, false); err != nil {
		return blindDecisionBinding{}, ErrBinding
	}
	data, err := readLimitedFile(path)
	if err != nil || hashBytes(data) != expectedSHA || strings.Contains(string(data), "{{") {
		return blindDecisionBinding{}, ErrBinding
	}
	fields, err := fieldMap(data)
	if err != nil {
		return blindDecisionBinding{}, ErrBinding
	}
	binding := blindDecisionBinding{
		Reviewer: fields["Reviewer 单写者"], BlindIdentity: fields["Run bundle blind identity"],
		PreferredArm: fields["Preferred arm"], ComparativeResult: fields["Comparative result"],
		HardSafetyGates: fields["Hard safety gates"],
	}
	if binding.Reviewer == "" || !hexSHA.MatchString(binding.BlindIdentity) || binding.PreferredArm == "" ||
		binding.ComparativeResult == "" || (binding.HardSafetyGates != "pass" && binding.HardSafetyGates != "fail") {
		return blindDecisionBinding{}, ErrBinding
	}
	return binding, nil
}

func readAttestation(caseRoot, rel string) (attestationBinding, []byte, error) {
	clean, err := cleanStateFile(rel, "evaluations/attestations/", attestationNamePattern)
	if err != nil {
		return attestationBinding{}, nil, ErrBinding
	}
	path := filepath.Join(caseRoot, ".steamai-vnext", filepath.FromSlash(clean))
	if err := requirePlainPath(caseRoot, path, false); err != nil {
		return attestationBinding{}, nil, ErrBinding
	}
	data, err := readLimitedFile(path)
	if err != nil {
		return attestationBinding{}, nil, err
	}
	fields, err := fieldMap(data)
	if err != nil || strings.Contains(string(data), "{{") {
		return attestationBinding{}, nil, ErrBinding
	}
	binding := attestationBinding{
		Reviewer: fields["Reviewer 单写者"], Purpose: fields["Purpose"],
		CandidatePatch: fields["Candidate patch"], CandidatePatchSHA: fields["Candidate patch SHA-256"],
		CandidateBlindDecision: fields["Blind decision"], CandidateBlindSHA: fields["Blind decision SHA-256"],
		BaselineArm: fields["Baseline arm"], CandidateArm: fields["Candidate arm"],
		SuiteIdentity: fields["Suite identity"], SuiteSHA: fields["Suite SHA-256"],
		RunBundleManifest: fields["Run bundle manifest"], RunBundleManifestSHA: fields["Run bundle manifest SHA-256"],
		RunBundleIdentity: fields["Run bundle identity"], RunBundleReveal: fields["Run bundle reveal"], RunBundleRevealSHA: fields["Run bundle reveal SHA-256"],
		HardSafetyGates:   fields["Hard safety gates"],
		ComparativeResult: fields["Comparative result"], Maturity: fields["Maturity"],
		CalibrationAttestation: fields["Calibration attestation"], CalibrationSHA: fields["Calibration attestation SHA-256"],
		CalibrationDecision: fields["Decision"],
	}
	if binding.Reviewer == "" || (binding.Purpose != "calibration" && binding.Purpose != "candidate") ||
		!hexSHA.MatchString(binding.SuiteIdentity) || !hexSHA.MatchString(binding.SuiteSHA) || binding.RunBundleManifest == "" ||
		!hexSHA.MatchString(binding.RunBundleManifestSHA) || !hexSHA.MatchString(binding.RunBundleIdentity) {
		return attestationBinding{}, nil, ErrBinding
	}
	if binding.Purpose == "calibration" {
		for _, value := range []string{
			binding.CandidatePatch, binding.CandidatePatchSHA, binding.CandidateBlindDecision, binding.CandidateBlindSHA,
			binding.RunBundleReveal, binding.RunBundleRevealSHA, binding.BaselineArm, binding.CandidateArm, binding.CalibrationAttestation, binding.CalibrationSHA,
		} {
			if value != "none" {
				return attestationBinding{}, nil, ErrBinding
			}
		}
	} else if !hexSHA.MatchString(binding.CandidatePatchSHA) || binding.RunBundleReveal == "" || !hexSHA.MatchString(binding.RunBundleRevealSHA) ||
		binding.CandidateBlindDecision == "" || !hexSHA.MatchString(binding.CandidateBlindSHA) || binding.BaselineArm == "" || binding.CandidateArm == "" ||
		binding.CalibrationAttestation == "" || !hexSHA.MatchString(binding.CalibrationSHA) {
		return attestationBinding{}, nil, ErrBinding
	}
	return binding, data, nil
}

func runManifestRelative(rel string) (string, error) {
	clean, err := cleanStateFile(rel, "evaluations/runs/", runManifestNamePattern)
	if err != nil {
		return "", err
	}
	return strings.TrimPrefix(clean, "evaluations/runs/"), nil
}

func readLimitedFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxVerifiedLearningFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxVerifiedLearningFileBytes {
		return nil, errors.New("verified learning file 超过大小限制")
	}
	return data, nil
}
