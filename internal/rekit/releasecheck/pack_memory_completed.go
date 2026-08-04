package releasecheck

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
)

const completedPackMemoryChangeCatalogKind = "completed-pack-memory-change-catalog"

// CompletedPackMemoryChangeCatalog is a read-only inventory of accepted pack
// memory changes whose verification, retirement, cleanup, and reconsume proof
// chain is complete.
type CompletedPackMemoryChangeCatalog struct {
	SchemaVersion int                         `json:"schemaVersion"`
	Kind          string                      `json:"kind"`
	RepoRoot      string                      `json:"repoRoot"`
	Pack          string                      `json:"pack"`
	Changes       []CompletedPackMemoryChange `json:"changes"`
	Warnings      []string                    `json:"warnings"`
}

// CompletedPackMemoryChange describes one accepted managed file change and
// the strict receipt/proof authority that completed it.
type CompletedPackMemoryChange struct {
	ChangeID                string                         `json:"changeId"`
	ManagedPath             string                         `json:"managedPath"`
	SourcePath              string                         `json:"sourcePath"`
	SourceSHA256            string                         `json:"sourceSha256"`
	AuthoritySHA256         string                         `json:"authoritySha256"`
	DecisionReceiptPath     string                         `json:"decisionReceiptPath"`
	DecisionReceiptSHA256   string                         `json:"decisionReceiptSha256"`
	VerificationProofPath   string                         `json:"verificationProofPath"`
	VerificationProofSHA256 string                         `json:"verificationProofSha256"`
	RetirementIntentPath    string                         `json:"retirementIntentPath"`
	RetirementIntentSHA256  string                         `json:"retirementIntentSha256"`
	RetirementReceiptPath   string                         `json:"retirementReceiptPath"`
	RetirementReceiptSHA256 string                         `json:"retirementReceiptSha256"`
	CleanupProof            CompletedPackMemoryChangeProof `json:"cleanupProof"`
	PackDoctorProof         CompletedPackMemoryChangeProof `json:"packDoctorProof"`
	FreshReconsumeProof     CompletedPackMemoryChangeProof `json:"freshReconsumeProof"`
	AttachedReconsumeProof  CompletedPackMemoryChangeProof `json:"attachedReconsumeProof"`
}

// CompletedPackMemoryChangeProof identifies a strict, repo-local proof
// artifact.
type CompletedPackMemoryChangeProof struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// BuildCompletedPackMemoryChangeCatalog scans only the selected pack's
// repo-local manifest, decision receipts, and strict proof artifacts. It does
// not inspect the case registry or mutate the repository.
func BuildCompletedPackMemoryChangeCatalog(repoRoot, pack string) (CompletedPackMemoryChangeCatalog, error) {
	repo, err := filepath.Abs(repoRoot)
	if err != nil {
		return CompletedPackMemoryChangeCatalog{}, err
	}
	m, err := manifest.Load(repo, pack)
	if err != nil {
		return CompletedPackMemoryChangeCatalog{}, err
	}
	catalog := CompletedPackMemoryChangeCatalog{
		SchemaVersion: 1,
		Kind:          completedPackMemoryChangeCatalogKind,
		RepoRoot:      repo,
		Pack:          m.Pack,
		Changes:       []CompletedPackMemoryChange{},
		Warnings:      []string{},
	}

	status, err := releaseHandoffPackMemoryCandidateStatus(repo, m.Summary())
	if err != nil {
		return CompletedPackMemoryChangeCatalog{}, err
	}
	managedTargets, err := completedPackMemoryManagedTargets(m)
	if err != nil {
		return CompletedPackMemoryChangeCatalog{}, err
	}
	proofRoot := filepath.Join(repo, filepath.FromSlash(status.ProofRoot))

	for _, receipt := range status.DecisionReceipts {
		if !receipt.VerificationComplete || !receipt.Retired {
			continue
		}
		for _, action := range receipt.Actions {
			if action.Decision != "accept" || action.Kind != "managed-doc" {
				continue
			}
			managedPath, ok := completedPackMemoryManagedPath(managedTargets, action.packTargetFull)
			if !ok {
				continue
			}
			sourceSHA256 := fileSHA256ReleaseHandoff(action.packTargetFull)
			if strings.TrimSpace(sourceSHA256) == "" {
				continue
			}

			proofArtifacts, err := completedPackMemoryActionProofs(status, receipt, action, proofRoot)
			if err != nil {
				return CompletedPackMemoryChangeCatalog{}, err
			}
			cleanup, ok := proofArtifacts["candidate-cleanup-proof"]
			if !ok || !cleanup.ProofPresent {
				continue
			}
			packDoctor, ok := proofArtifacts["pack-doctor-output"]
			if !ok || !packDoctor.ProofPresent {
				continue
			}
			fresh, ok := proofArtifacts["fresh-case-reconsume-proof"]
			if !ok || !fresh.ProofPresent {
				continue
			}
			attached, ok := proofArtifacts["attached-case-reconsume-proof"]
			if !ok || !attached.ProofPresent {
				continue
			}

			change, err := completedPackMemoryChange(repo, m.Pack, managedPath, sourceSHA256, receipt, action, cleanup, packDoctor, fresh, attached)
			if err != nil {
				return CompletedPackMemoryChangeCatalog{}, err
			}
			catalog.Changes = append(catalog.Changes, change)
		}
	}
	sort.Slice(catalog.Changes, func(i, j int) bool {
		if catalog.Changes[i].ManagedPath != catalog.Changes[j].ManagedPath {
			return catalog.Changes[i].ManagedPath < catalog.Changes[j].ManagedPath
		}
		return catalog.Changes[i].ChangeID < catalog.Changes[j].ChangeID
	})
	return catalog, nil
}

func completedPackMemoryManagedTargets(m *manifest.Manifest) (map[string]string, error) {
	targets := make(map[string]string, len(m.ManagedFiles))
	for _, managedPath := range m.ManagedFiles {
		full, err := m.SourcePath(managedPath)
		if err != nil {
			return nil, err
		}
		targets[filepath.Clean(full)] = filepath.ToSlash(filepath.Clean(filepath.FromSlash(managedPath)))
	}
	return targets, nil
}

func completedPackMemoryManagedPath(targets map[string]string, target string) (string, bool) {
	for full, managedPath := range targets {
		if sameReleaseHandoffPath(full, target) {
			return managedPath, true
		}
	}
	return "", false
}

func completedPackMemoryActionProofs(status ReleaseHandoffPackMemoryCandidateStatus, receipt ReleaseHandoffPackMemoryCandidateDecisionReceipt, action ReleaseHandoffPackMemoryCandidateDecisionReceiptAction, proofRoot string) (map[string]ReleaseHandoffPackMemoryCandidateReviewArtifact, error) {
	artifacts, err := packMemoryCandidateDecisionCleanupArtifacts(ReleaseHandoffPackMemoryCandidateStatus{
		Pack:             status.Pack,
		CandidateRoot:    status.CandidateRoot,
		ToolingRoot:      status.ToolingRoot,
		ProofRoot:        status.ProofRoot,
		IndexPath:        status.IndexPath,
		DecisionReceipts: []ReleaseHandoffPackMemoryCandidateDecisionReceipt{receipt},
		repoRootFull:     status.repoRootFull,
	}, proofRoot)
	if err != nil {
		return nil, err
	}
	proofs := make(map[string]ReleaseHandoffPackMemoryCandidateReviewArtifact, 4)
	for _, artifact := range artifacts {
		if artifact.CandidatePath == action.CandidatePath && artifact.PackTarget == action.PackTarget {
			proofs[artifact.Name] = artifact
		}
	}
	return proofs, nil
}

func completedPackMemoryChange(repo, pack, managedPath, sourceSHA256 string, receipt ReleaseHandoffPackMemoryCandidateDecisionReceipt, action ReleaseHandoffPackMemoryCandidateDecisionReceiptAction, cleanup, packDoctor, fresh, attached ReleaseHandoffPackMemoryCandidateReviewArtifact) (CompletedPackMemoryChange, error) {
	decisionReceipt, err := completedPackMemoryArtifact(repo, receipt.Path)
	if err != nil {
		return CompletedPackMemoryChange{}, fmt.Errorf("hash completed pack-memory decision receipt: %w", err)
	}
	verification, err := completedPackMemoryArtifact(repo, receipt.VerificationProofPath)
	if err != nil {
		return CompletedPackMemoryChange{}, fmt.Errorf("hash completed pack-memory verification proof: %w", err)
	}
	retirementIntent, err := completedPackMemoryArtifact(repo, receipt.RetirementIntentPath)
	if err != nil {
		return CompletedPackMemoryChange{}, fmt.Errorf("hash completed pack-memory retirement intent: %w", err)
	}
	retirementReceipt, err := completedPackMemoryArtifact(repo, receipt.RetirementReceiptPath)
	if err != nil {
		return CompletedPackMemoryChange{}, fmt.Errorf("hash completed pack-memory retirement receipt: %w", err)
	}
	cleanupProof, err := completedPackMemoryProof(repo, cleanup)
	if err != nil {
		return CompletedPackMemoryChange{}, err
	}
	packDoctorProof, err := completedPackMemoryProof(repo, packDoctor)
	if err != nil {
		return CompletedPackMemoryChange{}, err
	}
	freshProof, err := completedPackMemoryProof(repo, fresh)
	if err != nil {
		return CompletedPackMemoryChange{}, err
	}
	attachedProof, err := completedPackMemoryProof(repo, attached)
	if err != nil {
		return CompletedPackMemoryChange{}, err
	}

	changeID := sha256ReleaseHandoff([]byte(strings.Join([]string{
		pack,
		decisionReceipt.SHA256,
		verification.SHA256,
		retirementIntent.SHA256,
		retirementReceipt.SHA256,
		action.PackTarget,
		sourceSHA256,
	}, "\x00")))
	authoritySHA256 := sha256ReleaseHandoff([]byte(strings.Join([]string{
		changeID,
		receipt.packetHash,
		receipt.decisionHash,
		receipt.actionsHash,
		cleanupProof.SHA256,
		packDoctorProof.SHA256,
		freshProof.SHA256,
		attachedProof.SHA256,
	}, "\x00")))
	return CompletedPackMemoryChange{
		ChangeID:                "pack-memory-change-" + changeID,
		ManagedPath:             managedPath,
		SourcePath:              releaseHandoffRepoRelative(repo, action.packTargetFull),
		SourceSHA256:            sourceSHA256,
		AuthoritySHA256:         authoritySHA256,
		DecisionReceiptPath:     decisionReceipt.Path,
		DecisionReceiptSHA256:   decisionReceipt.SHA256,
		VerificationProofPath:   verification.Path,
		VerificationProofSHA256: verification.SHA256,
		RetirementIntentPath:    retirementIntent.Path,
		RetirementIntentSHA256:  retirementIntent.SHA256,
		RetirementReceiptPath:   retirementReceipt.Path,
		RetirementReceiptSHA256: retirementReceipt.SHA256,
		CleanupProof:            cleanupProof,
		PackDoctorProof:         packDoctorProof,
		FreshReconsumeProof:     freshProof,
		AttachedReconsumeProof:  attachedProof,
	}, nil
}

func completedPackMemoryProof(repo string, artifact ReleaseHandoffPackMemoryCandidateReviewArtifact) (CompletedPackMemoryChangeProof, error) {
	proof, err := completedPackMemoryArtifact(repo, artifact.ProofPath)
	if err != nil {
		return CompletedPackMemoryChangeProof{}, fmt.Errorf("hash completed pack-memory %s: %w", artifact.Name, err)
	}
	return proof, nil
}

func completedPackMemoryArtifact(repo, path string) (CompletedPackMemoryChangeProof, error) {
	if strings.TrimSpace(path) == "" {
		return CompletedPackMemoryChangeProof{}, fmt.Errorf("artifact path is empty")
	}
	full := filepath.Join(repo, filepath.FromSlash(path))
	data, err := os.ReadFile(full)
	if err != nil {
		return CompletedPackMemoryChangeProof{}, err
	}
	return CompletedPackMemoryChangeProof{Path: releaseHandoffRepoRelative(repo, full), SHA256: sha256ReleaseHandoff(data)}, nil
}
