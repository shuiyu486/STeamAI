package promote

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

type CandidateLifecycleProofDraftResult struct {
	SchemaVersion               int                                 `json:"schemaVersion"`
	Command                     string                              `json:"command"`
	Kind                        string                              `json:"kind"`
	Mode                        string                              `json:"mode"`
	CaseRoot                    string                              `json:"caseRoot"`
	RepoRoot                    string                              `json:"repoRoot"`
	Pack                        string                              `json:"pack"`
	PacketPath                  string                              `json:"packetPath"`
	PacketHash                  string                              `json:"packetHash"`
	ProofPath                   string                              `json:"proofPath"`
	ProofType                   string                              `json:"proofType"`
	ProofSHA256                 string                              `json:"proofSha256"`
	CandidatePath               string                              `json:"candidatePath"`
	PackTarget                  string                              `json:"packTarget,omitempty"`
	Reason                      string                              `json:"reason"`
	Actor                       string                              `json:"actor"`
	IsMutation                  bool                                `json:"isMutation"`
	Applied                     bool                                `json:"applied"`
	AlreadyWritten              bool                                `json:"alreadyWritten,omitempty"`
	Proof                       CandidateLifecycleProofNote         `json:"proof"`
	PreviewCommand              string                              `json:"previewCommand,omitempty"`
	ApplyCommand                string                              `json:"applyCommand,omitempty"`
	MissionCommanderAction      mission.MissionCommanderAction      `json:"missionCommanderAction"`
	MissionCommanderActionQueue mission.MissionCommanderActionQueue `json:"missionCommanderActionQueue"`
	NextSteps                   []string                            `json:"nextSteps"`
	Boundary                    []string                            `json:"boundary"`
}

type CandidateLifecycleProofNote struct {
	SchemaVersion int                               `json:"schemaVersion"`
	Kind          string                            `json:"kind"`
	Pack          string                            `json:"pack"`
	ProofType     string                            `json:"proofType"`
	CandidatePath string                            `json:"candidatePath"`
	PackTarget    string                            `json:"packTarget,omitempty"`
	Reason        string                            `json:"reason"`
	Actor         string                            `json:"actor"`
	EvidenceRefs  []CandidateDecisionEvidence       `json:"evidenceRefs"`
	ReviewItem    CandidateLifecycleProofReviewItem `json:"reviewItem"`
	Checks        []CandidateLifecycleProofCheck    `json:"checks"`
	Boundary      []string                          `json:"boundary"`
}

type CandidateLifecycleProofReviewItem struct {
	CandidatePath string `json:"candidatePath"`
	PackTarget    string `json:"packTarget,omitempty"`
	ProofType     string `json:"proofType"`
	Stage         string `json:"stage"`
}

type CandidateLifecycleProofCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Summary string `json:"summary"`
}

type preparedCandidateLifecycleProofDraft struct {
	result     CandidateLifecycleProofDraftResult
	proofBytes []byte
}

func DraftCandidateLifecycleProof(repoRoot, caseRoot, pack string, opt CandidateReviewProofDraftOptions) (CandidateLifecycleProofDraftResult, error) {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return CandidateLifecycleProofDraftResult{}, err
	}
	if opt.WhatIf && strings.TrimSpace(opt.ExpectedProofSHA256) != "" {
		return CandidateLifecycleProofDraftResult{}, fmt.Errorf("candidate lifecycle proof draft WhatIf does not accept -ExpectedProofSha256")
	}
	if !opt.WhatIf {
		decoded, decodeErr := hex.DecodeString(strings.TrimSpace(opt.ExpectedProofSHA256))
		if decodeErr != nil || len(decoded) != sha256.Size {
			return CandidateLifecycleProofDraftResult{}, fmt.Errorf("candidate lifecycle proof draft Apply requires a valid -ExpectedProofSha256 from WhatIf")
		}
	}
	prepared, err := prepareCandidateLifecycleProofDraft(repoRoot, inst.CaseRoot, pack, opt)
	if err != nil {
		return CandidateLifecycleProofDraftResult{}, err
	}
	result := prepared.result
	if opt.WhatIf {
		return finalizeCandidateLifecycleProofDraftResult(result), nil
	}
	if !strings.EqualFold(result.ProofSHA256, strings.TrimSpace(opt.ExpectedProofSHA256)) {
		return CandidateLifecycleProofDraftResult{}, fmt.Errorf("candidate lifecycle proof draft changed after preview")
	}
	already, err := writeCandidateReviewProofDraftFile(repoRoot, result.ProofPath, prepared.proofBytes)
	if err != nil {
		return CandidateLifecycleProofDraftResult{}, err
	}
	result.Applied = true
	result.AlreadyWritten = already
	if !already {
		result.Mode = "lifecycle-proof-drafted"
		result.NextSteps = []string{"review the drafted lifecycle proof note", "rerun release-check or status to refresh pack-memory proof summary", "continue cleanup/reconsume flow only after required lifecycle proof is present"}
	}
	return finalizeCandidateLifecycleProofDraftResult(result), nil
}

func prepareCandidateLifecycleProofDraft(repoRoot, caseRoot, pack string, opt CandidateReviewProofDraftOptions) (preparedCandidateLifecycleProofDraft, error) {
	packetPath, packetBytes, err := readStrictCandidateDecisionFile(opt.PacketPath, "candidate review packet")
	if err != nil {
		return preparedCandidateLifecycleProofDraft{}, err
	}
	var packet CandidateReviewPacket
	if err := decodeStrictJSON(packetBytes, &packet); err != nil {
		return preparedCandidateLifecycleProofDraft{}, fmt.Errorf("decode candidate review packet: %w", err)
	}
	if packet.SchemaVersion != 1 || packet.Kind != "pack-memory-candidate-review" || packet.Command != "promote" || packet.CandidateResult.Pack != pack || !sameCandidateDecisionPath(packet.CandidateResult.RepoRoot, repoRoot) || !sameCandidateDecisionPath(packet.CandidateResult.CaseRoot, caseRoot) {
		return preparedCandidateLifecycleProofDraft{}, fmt.Errorf("candidate review packet binding mismatch")
	}
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return preparedCandidateLifecycleProofDraft{}, err
	}
	canonicalCandidateRoot := filepath.Join(m.PackRoot, "promote-candidates")
	canonicalToolingRoot := filepath.Join(m.PackRoot, "tooling", "candidates")
	if !sameCandidateDecisionPath(packet.CandidateResult.CandidateRoot, canonicalCandidateRoot) || !sameCandidateDecisionPath(packet.CandidateResult.ToolingRoot, canonicalToolingRoot) {
		return preparedCandidateLifecycleProofDraft{}, fmt.Errorf("candidate review packet roots do not match canonical pack candidate roots")
	}
	proofType := strings.TrimSpace(opt.ProofType)
	if !candidateLifecycleProofDraftType(proofType) {
		return preparedCandidateLifecycleProofDraft{}, fmt.Errorf("candidate lifecycle proof draft supports only -ProofType pack-doctor-output, fresh-case-reconsume-proof, or attached-case-reconsume-proof")
	}
	if strings.TrimSpace(opt.DecisionPath) != "" {
		return preparedCandidateLifecycleProofDraft{}, fmt.Errorf("candidate lifecycle proof draft does not accept -CandidateDecisionPath")
	}
	if strings.TrimSpace(opt.Decision) != "" {
		return preparedCandidateLifecycleProofDraft{}, fmt.Errorf("candidate lifecycle proof draft does not accept -ProofDecision")
	}
	candidatePath := strings.TrimSpace(opt.CandidatePath)
	if candidatePath == "" {
		return preparedCandidateLifecycleProofDraft{}, fmt.Errorf("candidate lifecycle proof draft requires -CandidatePath")
	}
	reviewItem, err := candidateReviewProofItem(packet, candidatePath, repoRoot)
	if err != nil {
		return preparedCandidateLifecycleProofDraft{}, err
	}
	reviewItemRoot := canonicalCandidateRoot
	if reviewItem.Kind == "tooling-candidate-source" {
		reviewItemRoot = canonicalToolingRoot
	}
	if err := assertInsideRoot(reviewItemRoot, reviewItem.CandidatePath); err != nil {
		return preparedCandidateLifecycleProofDraft{}, err
	}
	if strings.TrimSpace(opt.Reason) == "" {
		return preparedCandidateLifecycleProofDraft{}, fmt.Errorf("candidate lifecycle proof draft requires -Reason")
	}
	if strings.TrimSpace(opt.Actor) == "" {
		return preparedCandidateLifecycleProofDraft{}, fmt.Errorf("candidate lifecycle proof draft requires -Actor")
	}
	evidenceRefs, err := parseCandidateLifecycleProofEvidenceRefs(repoRoot, opt.EvidenceRefs)
	if err != nil {
		return preparedCandidateLifecycleProofDraft{}, err
	}
	if len(evidenceRefs) == 0 {
		return preparedCandidateLifecycleProofDraft{}, fmt.Errorf("candidate lifecycle proof draft requires at least one repo-local -EvidenceRefs item")
	}
	proofPath, err := candidateReviewProofDraftPath(repoRoot, packet.CandidateResult.CandidateRoot, opt.ProofPath, proofType, reviewItem.CandidatePath)
	if err != nil {
		return preparedCandidateLifecycleProofDraft{}, err
	}
	packetHash := sha256Hex(packetBytes)
	candidateRel := candidateReviewProofRepoRelative(repoRoot, reviewItem.CandidatePath)
	packTarget := candidateLifecycleProofPackTarget(repoRoot, pack, reviewItem)
	proof := CandidateLifecycleProofNote{
		SchemaVersion: 1,
		Kind:          "pack-memory-candidate-lifecycle-proof",
		Pack:          pack,
		ProofType:     proofType,
		CandidatePath: candidateRel,
		PackTarget:    packTarget,
		Reason:        strings.TrimSpace(opt.Reason),
		Actor:         strings.TrimSpace(opt.Actor),
		EvidenceRefs:  evidenceRefs,
		ReviewItem: CandidateLifecycleProofReviewItem{
			CandidatePath: candidateRel,
			PackTarget:    packTarget,
			ProofType:     proofType,
			Stage:         candidateReviewProofArtifactStage(proofType),
		},
		Checks:   candidateLifecycleProofDraftChecks(proofType),
		Boundary: candidateLifecycleProofDraftBoundary(),
	}
	proofBytes, err := json.MarshalIndent(proof, "", "  ")
	if err != nil {
		return preparedCandidateLifecycleProofDraft{}, err
	}
	proofBytes = append(proofBytes, '\n')
	proofSHA256 := sha256Hex(proofBytes)
	previewCommand := candidateLifecycleProofDraftCommand(packetPath, proofPath, proofType, reviewItem.CandidatePath, proof.Reason, proof.Actor, opt.EvidenceRefs, "", false)
	applyCommand := candidateLifecycleProofDraftCommand(packetPath, proofPath, proofType, reviewItem.CandidatePath, proof.Reason, proof.Actor, opt.EvidenceRefs, proofSHA256, true)
	result := CandidateLifecycleProofDraftResult{
		SchemaVersion:  1,
		Command:        "promote",
		Kind:           "pack-memory-candidate-lifecycle-proof-draft",
		Mode:           "lifecycle-proof-draft-preview",
		CaseRoot:       caseRoot,
		RepoRoot:       repoRoot,
		Pack:           pack,
		PacketPath:     packetPath,
		PacketHash:     packetHash,
		ProofPath:      proofPath,
		ProofType:      proofType,
		ProofSHA256:    proofSHA256,
		CandidatePath:  reviewItem.CandidatePath,
		PackTarget:     packTarget,
		Reason:         proof.Reason,
		Actor:          proof.Actor,
		IsMutation:     !opt.WhatIf,
		Applied:        false,
		Proof:          proof,
		PreviewCommand: previewCommand,
		ApplyCommand:   applyCommand,
		NextSteps:      []string{"review the deterministic lifecycle proof note and exact hash", "write only the lifecycle proof note with the returned expected-hash Apply command", "rerun release-check or status to refresh pack-memory proof summary"},
		Boundary:       candidateLifecycleProofDraftBoundary(),
	}
	if existing, err := os.ReadFile(proofPath); err == nil {
		if bytes.Equal(existing, proofBytes) {
			result.Mode = "already-drafted"
			result.ApplyCommand = ""
			result.NextSteps = []string{"the exact lifecycle proof note already exists", "rerun release-check or status to refresh pack-memory proof summary"}
		} else {
			return preparedCandidateLifecycleProofDraft{}, fmt.Errorf("candidate lifecycle proof draft target already exists with different bytes: %s", proofPath)
		}
	} else if !os.IsNotExist(err) {
		return preparedCandidateLifecycleProofDraft{}, err
	}
	return preparedCandidateLifecycleProofDraft{result: result, proofBytes: proofBytes}, nil
}

func candidateLifecycleProofDraftType(proofType string) bool {
	switch strings.TrimSpace(proofType) {
	case "pack-doctor-output", "fresh-case-reconsume-proof", "attached-case-reconsume-proof":
		return true
	default:
		return false
	}
}

func candidateLifecycleProofPackTarget(repoRoot, pack string, item CandidateReviewProofReviewItemRef) string {
	if item.Kind == "tooling-candidate-source" {
		return filepath.ToSlash(filepath.Join("packs", pack, "tooling"))
	}
	packTarget := strings.TrimSpace(candidateReviewProofRepoRelative(repoRoot, item.PackTarget))
	if packTarget == "" {
		return ""
	}
	packTarget = filepath.ToSlash(packTarget)
	if strings.HasPrefix(packTarget, "packs/") {
		return packTarget
	}
	return filepath.ToSlash(filepath.Join("packs", pack, filepath.FromSlash(packTarget)))
}

func parseCandidateLifecycleProofEvidenceRefs(repoRoot, value string) ([]CandidateDecisionEvidence, error) {
	refs := splitCandidateDecisionDraftList(value)
	out := []CandidateDecisionEvidence{}
	seen := map[string]bool{}
	for _, ref := range refs {
		evidence, err := candidateDecisionDraftEvidenceRef(repoRoot, repoRoot, ref)
		if err != nil {
			return nil, err
		}
		evidence.Path = candidateReviewProofEvidencePath(repoRoot, repoRoot, evidence.Path)
		key := filepath.ToSlash(filepath.Clean(filepath.FromSlash(evidence.Path)))
		if seen[key] {
			return nil, fmt.Errorf("candidate lifecycle proof evidence duplicated: %s", evidence.Path)
		}
		seen[key] = true
		out = append(out, evidence)
	}
	return out, nil
}

func candidateLifecycleProofDraftChecks(proofType string) []CandidateLifecycleProofCheck {
	switch strings.TrimSpace(proofType) {
	case "pack-doctor-output":
		return []CandidateLifecycleProofCheck{{Name: "pack-doctor", Status: "passed", Summary: "reviewed repo-local evidence ref attests pack doctor passed"}}
	case "fresh-case-reconsume-proof":
		return []CandidateLifecycleProofCheck{
			{Name: "fresh-case-reconsume", Status: "passed", Summary: "reviewed repo-local evidence ref attests fresh case reconsume passed"},
			{Name: "pack-doctor", Status: "passed", Summary: "reviewed repo-local evidence ref attests pack doctor passed"},
		}
	case "attached-case-reconsume-proof":
		return []CandidateLifecycleProofCheck{
			{Name: "attached-case-reconsume", Status: "passed", Summary: "reviewed repo-local evidence ref attests attached case reconsume passed"},
			{Name: "pack-doctor", Status: "passed", Summary: "reviewed repo-local evidence ref attests pack doctor passed"},
		}
	default:
		return nil
	}
}

func candidateLifecycleProofDraftCommand(packetPath, proofPath, proofType, candidatePath, reason, actor, evidenceRefs, expectedProofSHA256 string, apply bool) string {
	base := "/rekit promote -PacketPath " + quoteCandidateDecisionArg(packetPath) +
		" -DraftReviewProof -ProofPath " + quoteCandidateDecisionArg(proofPath) +
		" -ProofType " + quoteCandidateDecisionArg(proofType) +
		" -CandidatePath " + quoteCandidateDecisionArg(candidatePath) +
		" -Reason " + quoteCandidateDecisionArg(reason) +
		" -Actor " + quoteCandidateDecisionArg(actor) +
		" -EvidenceRefs " + quoteCandidateDecisionArg(evidenceRefs)
	if apply {
		return base + " -ExpectedProofSha256 " + quoteCandidateDecisionArg(expectedProofSHA256) + " -Apply -Format json"
	}
	return base + " -WhatIf -Format json"
}

func candidateLifecycleProofDraftBoundary() []string {
	return []string{
		"draft writes only a repo-local pack-memory lifecycle proof note under promote-candidates/review-artifacts",
		"draft records reviewed repo-local evidence refs but does not run doctor, init, reconsume, or heavy tools",
		"Apply requires the exact proofSha256 returned by WhatIf",
		"lifecycle proof note must not contain case-specific artifacts, traces, dumps, captures, payloads, flags, or customer data",
	}
}
