package packmemoryconsumption

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
	"reflect"
	"sort"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/doctor"
	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/kitmutation"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/releasecheck"
)

const (
	maxConsumptionFileBytes   = 4 * 1024 * 1024
	maxConsumptionIntentBytes = 32 * 1024 * 1024
)

var (
	completedCatalogBuilder = releasecheck.BuildCompletedPackMemoryChangeCatalog
	doctorCase              = doctor.Case
	acquireMutationLease    = func(caseRoot string) (mutationLease, error) { return kitmutation.Acquire(caseRoot) }
	publicationHook         func(Plan, int) error
	artifactCommitHook      func(string, string) error
)

type mutationLease interface{ Unlock() error }

type rootIdentitySource interface {
	Lstat(string) (os.FileInfo, error)
	Open(string) (*os.File, error)
}

type syncState struct {
	SchemaVersion int                         `json:"schemaVersion"`
	TemplateRoot  string                      `json:"templateRoot"`
	TemplatePack  string                      `json:"templatePack"`
	LastSyncAt    string                      `json:"lastSyncAt"`
	Managed       map[string]syncManagedEntry `json:"managed"`
}

type syncManagedEntry struct {
	SourceHash       string `json:"sourceHash"`
	TargetHashAtSync string `json:"targetHashAtSync"`
	LastAction       string `json:"lastAction"`
}

type consumptionIntent struct {
	SchemaVersion      int              `json:"schemaVersion"`
	Kind               string           `json:"kind"`
	CaseRootIdentity   CaseRootIdentity `json:"caseRootIdentity"`
	Plan               Plan             `json:"plan"`
	PlanSHA256         string           `json:"planSha256"`
	TargetBeforeExists bool             `json:"targetBeforeExists"`
	TargetBefore       []byte           `json:"targetBefore,omitempty"`
	TargetBeforeSHA256 string           `json:"targetBeforeSha256,omitempty"`
	StateBeforeExists  bool             `json:"stateBeforeExists"`
	StateBefore        []byte           `json:"stateBefore,omitempty"`
	StateBeforeSHA256  string           `json:"stateBeforeSha256,omitempty"`
	TargetAfter        []byte           `json:"targetAfter"`
	TargetAfterSHA256  string           `json:"targetAfterSha256"`
	StateAfter         []byte           `json:"stateAfter"`
	StateAfterSHA256   string           `json:"stateAfterSha256"`
	BackupAfter        []byte           `json:"backupAfter,omitempty"`
	BackupAfterSHA256  string           `json:"backupAfterSha256,omitempty"`
}

func Discover(repoRoot, caseRoot, pack string) (Discovery, error) {
	repo, caseFull, _, state, catalog, err := prepare(repoRoot, caseRoot, pack)
	if err != nil {
		return Discovery{}, err
	}
	discovery := Discovery{SchemaVersion: SchemaVersion, Kind: KindDiscovery, RepoRoot: repo, CaseRoot: caseFull, Pack: pack, Catalog: catalog, NextSteps: []string{"review an available completed pack-memory change, then run its previewCommand", "Apply requires the exact expectedPlanSha256 returned by the selected WhatIf preview"}, Boundary: consumptionBoundary()}
	for _, change := range catalog.Changes {
		status, err := inspectChange(repo, caseFull, pack, state, change)
		if err != nil {
			return Discovery{}, err
		}
		switch status.State {
		case "available", "content-current-no-receipt":
			discovery.Available = append(discovery.Available, status)
		case "already-consumed":
			discovery.Consumed = append(discovery.Consumed, status)
		default:
			discovery.Conflicts = append(discovery.Conflicts, status)
		}
	}
	return discovery, nil
}

func Preview(repoRoot, caseRoot, pack, changeID string) (Plan, error) {
	plan, _, err := buildPlan(repoRoot, caseRoot, pack, changeID)
	return plan, err
}

func Apply(repoRoot, caseRoot, pack, changeID, expectedPlanSHA256 string) (_ Result, retErr error) {
	if !validSHA256(expectedPlanSHA256) {
		return Result{}, fmt.Errorf("pack-memory selected sync Apply requires a valid -ExpectedPackMemoryConsumptionPlanSha256 from WhatIf")
	}
	if err := validateArtifactID(changeID); err != nil {
		return Result{}, err
	}
	lease, err := acquireMutationLease(caseRoot)
	if err != nil {
		return Result{}, err
	}
	defer func() { retErr = errors.Join(retErr, lease.Unlock()) }()

	repo, caseFull, err := attachedIdentity(repoRoot, caseRoot, pack)
	if err != nil {
		return Result{}, err
	}
	root, err := openPinnedCaseRoot(caseFull)
	if err != nil {
		return Result{}, err
	}
	defer func() { retErr = errors.Join(retErr, root.Close()) }()

	intentPath, err := consumptionIntentPath(caseFull, changeID)
	if err != nil {
		return Result{}, err
	}
	receiptPath, err := consumptionReceiptPath(caseFull, changeID)
	if err != nil {
		return Result{}, err
	}
	intent, intentExists, err := readIntentRoot(root, caseRelative(caseFull, intentPath))
	if err != nil {
		return Result{}, err
	}
	if intentExists {
		if !strings.EqualFold(intent.PlanSHA256, expectedPlanSHA256) {
			return Result{}, fmt.Errorf("pack-memory consumption durable intent has different plan binding: %s", intentPath)
		}
		if err := validateIntentLocal(intent, root, repo, caseFull, pack, changeID); err != nil {
			return Result{}, err
		}
		return executeIntent(root, intent, repoRoot, caseRoot)
	}
	if receipt, exists, err := readReceiptRoot(root, caseRelative(caseFull, receiptPath)); err != nil {
		return Result{}, err
	} else if exists {
		return replayReceipt(root, receipt, repo, caseFull, pack, changeID, expectedPlanSHA256, repoRoot, caseRoot)
	}

	plan, sourceBytes, err := buildPlan(repoRoot, caseRoot, pack, changeID)
	if err != nil {
		return Result{}, err
	}
	if !strings.EqualFold(plan.ExpectedPlanSHA256, expectedPlanSHA256) {
		return Result{}, fmt.Errorf("pack-memory consumption plan sha256 mismatch: got %s want %s; rerun sync -WhatIf", expectedPlanSHA256, plan.ExpectedPlanSHA256)
	}
	if plan.Replay {
		return Result{}, fmt.Errorf("pack-memory consumption replay must be resolved from the pinned case-local receipt")
	}

	targetBytes, targetExists, err := readOptionalAnchored(plan.CaseRoot, plan.TargetPath, "pack-memory consumption target")
	if err != nil {
		return Result{}, err
	}
	if sha256Hex(targetBytes) != plan.TargetSHA256Before || targetExists != (plan.TargetSHA256Before != "") {
		return Result{}, fmt.Errorf("pack-memory consumption target changed after preview; rerun sync -WhatIf")
	}
	stateBytes, stateExists, err := readOptionalAnchored(plan.CaseRoot, plan.StatePath, "pack-memory consumption sync state")
	if err != nil {
		return Result{}, err
	}
	if sha256Hex(stateBytes) != plan.StateSHA256Before || stateExists != (plan.StateSHA256Before != "") {
		return Result{}, fmt.Errorf("pack-memory consumption sync state changed after preview; rerun sync -WhatIf")
	}
	stateOut, err := plannedStateBytes(plan, stateBytes, stateExists)
	if err != nil {
		return Result{}, err
	}
	intent = consumptionIntent{SchemaVersion: SchemaVersion, Kind: "pack-memory-consumption-intent", CaseRootIdentity: root.identity, Plan: plan, PlanSHA256: plan.ExpectedPlanSHA256, TargetBeforeExists: targetExists, TargetBefore: targetBytes, TargetBeforeSHA256: sha256Hex(targetBytes), StateBeforeExists: stateExists, StateBefore: stateBytes, StateBeforeSHA256: sha256Hex(stateBytes), TargetAfter: sourceBytes, TargetAfterSHA256: sha256Hex(sourceBytes), StateAfter: stateOut, StateAfterSHA256: sha256Hex(stateOut)}
	if plan.BackupPath != "" {
		intent.BackupAfter, intent.BackupAfterSHA256 = targetBytes, sha256Hex(targetBytes)
	}
	if err := validateIntent(intent, repo, caseFull, pack, plan.Authority, changeID, true); err != nil {
		return Result{}, err
	}
	intentBytes, err := canonical(intent)
	if err != nil {
		return Result{}, err
	}
	if err := root.commitExclusiveAtomic(caseRelative(plan.CaseRoot, intentPath), intentBytes, "pack-memory consumption durable intent"); err != nil {
		return Result{}, err
	}
	return executeIntent(root, intent, repoRoot, caseRoot)
}

func executeIntent(root *anchoredRoot, intent consumptionIntent, repoRoot, caseRoot string) (Result, error) {
	plan := intent.Plan
	if err := root.revalidate(); err != nil {
		return Result{}, err
	}
	if receipt, exists, err := readReceiptRoot(root, caseRelative(plan.CaseRoot, plan.ReceiptPath)); err != nil {
		return Result{}, err
	} else if exists {
		stateBytes, stateExists, err := readOptionalAnchored(plan.CaseRoot, plan.StatePath, "pack-memory consumption replay sync state")
		if err != nil || !stateExists {
			return Result{}, errors.Join(err, fmt.Errorf("pack-memory consumption replay sync state is missing"))
		}
		var state syncState
		if err := decodeStrictJSON(stateBytes, &state); err != nil {
			return Result{}, err
		}
		targetBytes, targetExists, err := readOptionalAnchored(plan.CaseRoot, plan.TargetPath, "pack-memory consumption replay target")
		if err != nil || !targetExists {
			return Result{}, errors.Join(err, fmt.Errorf("pack-memory consumption replay target is missing"))
		}
		if err := validateReceipt(root.identity, plan.RepoRoot, plan.CaseRoot, plan.Pack, plan.Authority, state, stateBytes, targetBytes, receipt); err != nil {
			return Result{}, err
		}
		doctorRows, err := doctorCase(plan.RepoRoot, plan.CaseRoot, plan.Pack)
		if err != nil {
			return Result{}, fmt.Errorf("pack-memory selected sync replay doctor verification failed: %w", err)
		}
		discovery, err := committedDiscovery(plan.RepoRoot, plan.CaseRoot, plan.Pack, plan.Authority, receipt)
		if err != nil {
			return Result{}, err
		}
		plan.IsMutation, plan.Applied, plan.Replay, plan.RequiresReview = true, true, true, false
		return Result{Plan: plan, Receipt: receipt, DoctorRows: doctorRows, Discovery: discovery}, nil
	}
	type publication struct {
		path      string
		label     string
		before    []byte
		beforeOK  bool
		after     []byte
		exclusive bool
	}
	publications := []publication{}
	if plan.BackupPath != "" {
		publications = append(publications, publication{path: plan.BackupPath, label: "pack-memory consumption backup", after: intent.BackupAfter, exclusive: true})
	}
	publications = append(publications,
		publication{path: plan.TargetPath, label: "pack-memory consumption target", before: intent.TargetBefore, beforeOK: intent.TargetBeforeExists, after: intent.TargetAfter},
		publication{path: plan.StatePath, label: "pack-memory consumption sync state", before: intent.StateBefore, beforeOK: intent.StateBeforeExists, after: intent.StateAfter},
	)
	firstMissing := len(publications)
	for index, publication := range publications {
		current, exists, err := root.readOptional(caseRelative(plan.CaseRoot, publication.path), publication.label)
		if err != nil {
			return Result{}, err
		}
		if bytes.Equal(current, publication.after) && exists {
			if firstMissing != len(publications) {
				return Result{}, fmt.Errorf("pack-memory consumption recovery is non-prefix at %s", publication.path)
			}
			continue
		}
		if exists == publication.beforeOK && bytes.Equal(current, publication.before) {
			if firstMissing == len(publications) {
				firstMissing = index
			}
			continue
		}
		return Result{}, fmt.Errorf("pack-memory consumption recovery differs from the exact durable intent prefix at %s", publication.path)
	}
	for index := firstMissing; index < len(publications); index++ {
		if publicationHook != nil {
			if err := publicationHook(plan, index); err != nil {
				return Result{}, err
			}
		}
		publication := publications[index]
		rel := caseRelative(plan.CaseRoot, publication.path)
		var publishErr error
		if publication.exclusive {
			publishErr = root.writeExclusive(rel, publication.after, publication.label)
		} else {
			publishErr = root.replaceExact(rel, publication.before, publication.beforeOK, publication.after, publication.label)
		}
		if publishErr != nil {
			return Result{}, publishErr
		}
	}

	doctorRows, err := doctorCase(plan.RepoRoot, plan.CaseRoot, plan.Pack)
	if err != nil {
		return Result{}, fmt.Errorf("pack-memory selected sync doctor verification failed: %w", err)
	}
	postTarget, exists, err := root.readOptional(caseRelative(plan.CaseRoot, plan.TargetPath), "pack-memory consumption target after apply")
	if err != nil || !exists || !bytes.Equal(postTarget, intent.TargetAfter) {
		return Result{}, fmt.Errorf("pack-memory selected sync post-write target verification failed: %w", err)
	}
	postState, exists, err := root.readOptional(caseRelative(plan.CaseRoot, plan.StatePath), "pack-memory consumption sync state after apply")
	if err != nil || !exists || !bytes.Equal(postState, intent.StateAfter) {
		return Result{}, fmt.Errorf("pack-memory selected sync post-write state verification failed: %w", err)
	}
	receipt := receiptForIntent(intent, len(doctorRows))
	receiptBytes, err := canonical(receipt)
	if err != nil {
		return Result{}, err
	}
	if err := root.commitExclusiveAtomic(caseRelative(plan.CaseRoot, plan.ReceiptPath), receiptBytes, "pack-memory consumption final receipt"); err != nil {
		return Result{}, err
	}
	if err := validateReceipt(root.identity, plan.RepoRoot, plan.CaseRoot, plan.Pack, plan.Authority, mustDecodeState(intent.StateAfter), intent.StateAfter, intent.TargetAfter, receipt); err != nil {
		return Result{}, err
	}
	discovery, err := committedDiscovery(plan.RepoRoot, plan.CaseRoot, plan.Pack, plan.Authority, receipt)
	if err != nil {
		return Result{}, err
	}
	plan.IsMutation, plan.Applied, plan.RequiresReview = true, true, false
	plan.NextSteps = []string{"retain the case-local consumption receipt as final commit evidence", "refresh status or handoff from the target case"}
	return Result{Plan: plan, Receipt: receipt, DoctorRows: doctorRows, Discovery: discovery}, nil
}

func buildPlan(repoRoot, caseRoot, pack, changeID string) (Plan, []byte, error) {
	if err := validateArtifactID(changeID); err != nil {
		return Plan{}, nil, err
	}
	repo, caseFull, _, state, catalog, err := prepare(repoRoot, caseRoot, pack)
	if err != nil {
		return Plan{}, nil, err
	}
	change, ok := selectedChange(catalog, changeID)
	if !ok {
		return Plan{}, nil, fmt.Errorf("completed pack-memory change not found: %s", changeID)
	}
	status, err := inspectChange(repo, caseFull, pack, state, change)
	if err != nil {
		return Plan{}, nil, err
	}
	if status.State == "local-conflict" || status.State == "invalid-receipt" {
		return Plan{}, nil, fmt.Errorf("pack-memory change %s is not eligible for selected sync: %s", changeID, strings.Join(status.Warnings, "; "))
	}
	if status.State == "already-consumed" {
		receipt, _, err := readReceipt(caseFull, status.ReceiptPath)
		if err != nil {
			return Plan{}, nil, err
		}
		plan, err := planFromReceipt(change, receipt, status.ReceiptPath)
		if err != nil {
			return Plan{}, nil, err
		}
		plan.Replay = true
		return plan, nil, nil
	}
	sourcePath := filepath.Join(repo, filepath.FromSlash(change.SourcePath))
	sourceBytes, err := refsf.ReadStableRegularFileAnchored(repo, sourcePath, "completed pack-memory change source", maxConsumptionFileBytes)
	if err != nil || !strings.EqualFold(sha256Hex(sourceBytes), change.SourceSHA256) {
		return Plan{}, nil, fmt.Errorf("completed pack-memory change source drifted: %s: %w", sourcePath, err)
	}
	statePath, err := projectstate.Join(caseFull, "state.json")
	if err != nil {
		return Plan{}, nil, err
	}
	stateBytes, _, err := readOptionalAnchored(caseFull, statePath, "pack-memory consumption sync state")
	if err != nil {
		return Plan{}, nil, err
	}
	plan, err := makePlan(repo, caseFull, pack, change, status.TargetSHA256, sha256Hex(stateBytes))
	return plan, sourceBytes, err
}

func makePlan(repo, caseRoot, pack string, change releasecheck.CompletedPackMemoryChange, targetBefore, stateBefore string) (Plan, error) {
	targetPath, err := refsf.SafeJoin(caseRoot, change.ManagedPath)
	if err != nil {
		return Plan{}, err
	}
	stateRoot, err := projectstate.Resolve(caseRoot)
	if err != nil {
		return Plan{}, err
	}
	receiptPath := filepath.Join(stateRoot.Path, "pack-memory", "consumptions", change.ChangeID+".json")
	backupPath, action := "", "create-managed-file"
	if targetBefore != "" && !strings.EqualFold(targetBefore, change.SourceSHA256) {
		action = "overwrite-managed-file-with-backup"
		backupPath = filepath.Join(stateRoot.Path, "backups", "pack-memory", change.ChangeID, filepath.FromSlash(change.ManagedPath))
	} else if strings.EqualFold(targetBefore, change.SourceSHA256) {
		action = "record-current-content-consumption"
	}
	plan := Plan{SchemaVersion: SchemaVersion, Kind: KindPlan, Command: "sync", RepoRoot: repo, CaseRoot: caseRoot, Pack: pack, ChangeID: change.ChangeID, ManagedPath: change.ManagedPath, SourcePath: filepath.Join(repo, filepath.FromSlash(change.SourcePath)), SourceSHA256: change.SourceSHA256, TargetPath: targetPath, TargetSHA256Before: targetBefore, StatePath: filepath.Join(stateRoot.Path, "state.json"), StateSHA256Before: stateBefore, ReceiptPath: receiptPath, BackupPath: backupPath, Action: action, Authority: change, RequiresReview: true, NextSteps: []string{"review the selected managed path, exact hashes, backup, and producer proof authority", "run the returned expected-hash Apply command only after confirming this scope"}, Boundary: consumptionBoundary()}
	plan.ExpectedPlanSHA256, err = planHash(plan)
	if err != nil {
		return Plan{}, err
	}
	plan.ApplyCommand = selectedSyncCommand(caseRoot, pack, change.ChangeID, plan.ExpectedPlanSHA256, true)
	return plan, nil
}

func attachedIdentity(repoRoot, caseRoot, pack string) (string, string, error) {
	repo, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", "", err
	}
	inst, err := instance.AssertAttached(caseRoot, repo, pack)
	if err != nil {
		return "", "", err
	}
	return repo, inst.CaseRoot, nil
}

func prepare(repoRoot, caseRoot, pack string) (string, string, *manifest.Manifest, syncState, releasecheck.CompletedPackMemoryChangeCatalog, error) {
	repo, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", "", nil, syncState{}, releasecheck.CompletedPackMemoryChangeCatalog{}, err
	}
	inst, err := instance.AssertAttached(caseRoot, repo, pack)
	if err != nil {
		return "", "", nil, syncState{}, releasecheck.CompletedPackMemoryChangeCatalog{}, err
	}
	m, err := manifest.Load(repo, pack)
	if err != nil {
		return "", "", nil, syncState{}, releasecheck.CompletedPackMemoryChangeCatalog{}, err
	}
	catalog, err := completedCatalogBuilder(repo, pack)
	if err != nil {
		return "", "", nil, syncState{}, releasecheck.CompletedPackMemoryChangeCatalog{}, err
	}
	state := syncState{Managed: map[string]syncManagedEntry{}}
	statePath, err := projectstate.Join(inst.CaseRoot, "state.json")
	if err != nil {
		return "", "", nil, syncState{}, releasecheck.CompletedPackMemoryChangeCatalog{}, err
	}
	if data, exists, err := readOptionalAnchored(inst.CaseRoot, statePath, "pack-memory consumption sync state"); err != nil {
		return "", "", nil, syncState{}, releasecheck.CompletedPackMemoryChangeCatalog{}, err
	} else if exists {
		if err := decodeStrictJSON(data, &state); err != nil {
			return "", "", nil, syncState{}, releasecheck.CompletedPackMemoryChangeCatalog{}, fmt.Errorf("decode sync state: %w", err)
		}
		if state.SchemaVersion != 1 || !samePath(state.TemplateRoot, repo) || state.TemplatePack != pack || state.Managed == nil {
			return "", "", nil, syncState{}, releasecheck.CompletedPackMemoryChangeCatalog{}, fmt.Errorf("sync state binding mismatch: %s", statePath)
		}
	}
	return repo, inst.CaseRoot, m, state, catalog, nil
}

func inspectChange(repo, caseRoot, pack string, state syncState, change releasecheck.CompletedPackMemoryChange) (ChangeStatus, error) {
	targetPath, err := refsf.SafeJoin(caseRoot, change.ManagedPath)
	if err != nil {
		return ChangeStatus{}, err
	}
	targetBytes, targetExists, err := readOptionalAnchored(caseRoot, targetPath, "pack-memory consumption target")
	if err != nil {
		return ChangeStatus{}, err
	}
	targetHash := sha256Hex(targetBytes)
	status := ChangeStatus{ChangeID: change.ChangeID, ManagedPath: change.ManagedPath, SourceSHA256: change.SourceSHA256, TargetSHA256: targetHash, State: "available"}
	if entry, ok := state.Managed[change.ManagedPath]; ok {
		status.TargetHashAtSync = entry.TargetHashAtSync
		if targetExists && entry.TargetHashAtSync != "" && !strings.EqualFold(targetHash, entry.TargetHashAtSync) && !strings.EqualFold(targetHash, change.SourceSHA256) {
			status.State, status.Warnings = "local-conflict", []string{"target changed since the last recorded sync; selected sync refuses overwrite"}
			return status, nil
		}
	}
	receiptPath, err := consumptionReceiptPath(caseRoot, change.ChangeID)
	if err != nil {
		return ChangeStatus{}, err
	}
	status.ReceiptPath = receiptPath
	if receipt, exists, err := readReceipt(caseRoot, receiptPath); err != nil {
		status.State, status.Warnings = "invalid-receipt", []string{err.Error()}
		return status, nil
	} else if exists {
		stateBytes, err := canonical(state)
		if err != nil {
			return ChangeStatus{}, err
		}
		identity, identityErr := currentCaseRootIdentity(caseRoot)
		if identityErr != nil {
			status.State, status.Warnings = "invalid-receipt", []string{identityErr.Error()}
			return status, nil
		}
		if err := validateReceipt(identity, repo, caseRoot, pack, change, state, stateBytes, targetBytes, receipt); err != nil {
			status.State, status.Warnings = "invalid-receipt", []string{err.Error()}
			return status, nil
		}
		status.State = "already-consumed"
		return status, nil
	}
	if targetExists && strings.EqualFold(targetHash, change.SourceSHA256) {
		status.State = "content-current-no-receipt"
	}
	status.PreviewCommand = selectedSyncCommand(caseRoot, pack, change.ChangeID, "", false)
	return status, nil
}

func replayReceipt(root *anchoredRoot, receipt Receipt, repo, caseRoot, pack, changeID, expected, repoRoot, requestedCaseRoot string) (Result, error) {
	if receipt.ChangeID != changeID || !strings.EqualFold(receipt.PlanSHA256, expected) {
		return Result{}, fmt.Errorf("pack-memory consumption receipt has different replay binding")
	}
	change := receipt.Authority
	receiptPath, err := consumptionReceiptPath(caseRoot, changeID)
	if err != nil {
		return Result{}, err
	}
	plan, err := planFromReceipt(change, receipt, receiptPath)
	if err != nil {
		return Result{}, err
	}
	if !samePath(plan.RepoRoot, repo) || !samePath(plan.CaseRoot, caseRoot) || plan.Pack != pack || plan.ManagedPath != change.ManagedPath {
		return Result{}, fmt.Errorf("pack-memory consumption receipt identity binding mismatch")
	}
	stateBytes, stateExists, err := root.readOptional(caseRelative(caseRoot, plan.StatePath), "pack-memory consumption replay sync state")
	if err != nil || !stateExists {
		return Result{}, errors.Join(err, fmt.Errorf("pack-memory consumption replay sync state is missing"))
	}
	var state syncState
	if err := decodeStrictJSON(stateBytes, &state); err != nil {
		return Result{}, err
	}
	targetBytes, targetExists, err := root.readOptional(caseRelative(caseRoot, plan.TargetPath), "pack-memory consumption replay target")
	if err != nil || !targetExists {
		return Result{}, errors.Join(err, fmt.Errorf("pack-memory consumption replay target is missing"))
	}
	if err := validateReceipt(root.identity, repo, caseRoot, pack, change, state, stateBytes, targetBytes, receipt); err != nil {
		return Result{}, err
	}
	if err := root.revalidate(); err != nil {
		return Result{}, err
	}
	doctorRows, err := doctorCase(repo, caseRoot, pack)
	if err != nil {
		return Result{}, fmt.Errorf("pack-memory selected sync replay doctor verification failed: %w", err)
	}
	discovery, err := committedDiscovery(repo, caseRoot, pack, change, receipt)
	if err != nil {
		return Result{}, err
	}
	plan.IsMutation, plan.Applied, plan.Replay, plan.RequiresReview = true, true, true, false
	return Result{Plan: plan, Receipt: receipt, DoctorRows: doctorRows, Discovery: discovery}, nil
}

func validateReceipt(identity CaseRootIdentity, repo, caseRoot, pack string, change releasecheck.CompletedPackMemoryChange, state syncState, stateBytes, targetBytes []byte, receipt Receipt) error {
	if !reflect.DeepEqual(receipt.CaseRootIdentity, identity) {
		return fmt.Errorf("case-local consumption receipt case-root identity mismatch")
	}
	if receipt.SchemaVersion != SchemaVersion || receipt.Kind != KindReceipt || !validCaseRootIdentity(receipt.CaseRootIdentity) || !samePath(receipt.RepoRoot, repo) || !samePath(receipt.CaseRoot, caseRoot) || receipt.Pack != pack || receipt.ChangeID != change.ChangeID || receipt.ManagedPath != change.ManagedPath || !reflect.DeepEqual(receipt.Authority, change) || !strings.EqualFold(receipt.SourceSHA256, change.SourceSHA256) || !strings.EqualFold(receipt.TargetSHA256After, change.SourceSHA256) || !strings.EqualFold(sha256Hex(targetBytes), change.SourceSHA256) || !strings.EqualFold(receipt.StateSHA256After, sha256Hex(stateBytes)) || receipt.DoctorRows <= 0 || !receipt.NoAuthority || !receipt.NoConfirmed || !receipt.NoHeavyTool || !reflect.DeepEqual(receipt.Boundary, consumptionBoundary()) {
		return fmt.Errorf("case-local consumption receipt binding mismatch")
	}
	entry, ok := state.Managed[change.ManagedPath]
	if !ok || !strings.EqualFold(entry.SourceHash, change.SourceSHA256) || !strings.EqualFold(entry.TargetHashAtSync, change.SourceSHA256) || entry.LastAction != "pack-memory-selected-sync" || state.SchemaVersion != 1 || !samePath(state.TemplateRoot, repo) || state.TemplatePack != pack {
		return fmt.Errorf("case-local consumption receipt managed state binding mismatch")
	}
	expectedPlan, err := makePlan(repo, caseRoot, pack, change, receipt.TargetSHA256Before, receipt.StateSHA256Before)
	if err != nil || !strings.EqualFold(receipt.PlanSHA256, expectedPlan.ExpectedPlanSHA256) {
		return fmt.Errorf("case-local consumption receipt plan binding mismatch")
	}
	expectedBackup := caseStoredPath(caseRoot, expectedPlan.BackupPath)
	if receipt.BackupPath != expectedBackup {
		return fmt.Errorf("case-local consumption receipt backup path binding mismatch")
	}
	if expectedBackup != "" {
		backupPath, err := refsf.SafeJoin(caseRoot, expectedBackup)
		if err != nil {
			return err
		}
		backup, exists, err := readOptionalAnchored(caseRoot, backupPath, "pack-memory consumption receipt backup")
		if err != nil || !exists || !strings.EqualFold(sha256Hex(backup), receipt.TargetSHA256Before) {
			return fmt.Errorf("case-local consumption receipt backup bytes binding mismatch: %w", err)
		}
	}
	return nil
}

func validateIntentLocal(intent consumptionIntent, root *anchoredRoot, repo, caseRoot, pack, changeID string) error {
	if !reflect.DeepEqual(intent.CaseRootIdentity, root.identity) {
		return fmt.Errorf("pack-memory consumption durable intent case-root identity mismatch")
	}
	return validateIntent(intent, repo, caseRoot, pack, intent.Plan.Authority, changeID, false)
}

func validateIntent(intent consumptionIntent, repo, caseRoot, pack string, change releasecheck.CompletedPackMemoryChange, changeID string, verifySource bool) error {
	plan := intent.Plan
	if !validCaseRootIdentity(intent.CaseRootIdentity) || intent.SchemaVersion != SchemaVersion || intent.Kind != "pack-memory-consumption-intent" || !samePath(plan.RepoRoot, repo) || !samePath(plan.CaseRoot, caseRoot) || plan.Pack != pack || plan.ChangeID != changeID || change.ChangeID != changeID || plan.ManagedPath != change.ManagedPath || !reflect.DeepEqual(plan.Authority, change) || intent.PlanSHA256 != plan.ExpectedPlanSHA256 {
		return fmt.Errorf("pack-memory consumption durable intent binding mismatch")
	}
	hash, err := planHash(plan)
	if err != nil || !strings.EqualFold(hash, intent.PlanSHA256) {
		return fmt.Errorf("pack-memory consumption durable intent plan hash mismatch")
	}
	expected, err := makePlan(repo, caseRoot, pack, change, intent.TargetBeforeSHA256, intent.StateBeforeSHA256)
	if err != nil || !reflect.DeepEqual(expected, plan) {
		return fmt.Errorf("pack-memory consumption durable intent exact plan mismatch")
	}
	if intent.TargetBeforeExists != (intent.TargetBeforeSHA256 != "") || sha256Hex(intent.TargetBefore) != intent.TargetBeforeSHA256 || intent.StateBeforeExists != (intent.StateBeforeSHA256 != "") || sha256Hex(intent.StateBefore) != intent.StateBeforeSHA256 || sha256Hex(intent.TargetAfter) != intent.TargetAfterSHA256 || !strings.EqualFold(intent.TargetAfterSHA256, change.SourceSHA256) {
		return fmt.Errorf("pack-memory consumption durable intent original or target bytes mismatch")
	}
	if verifySource && !bytes.Equal(intent.TargetAfter, mustReadSource(repo, change)) {
		return fmt.Errorf("pack-memory consumption durable intent source bytes mismatch")
	}
	stateOut, err := plannedStateBytes(plan, intent.StateBefore, intent.StateBeforeExists)
	if err != nil || !bytes.Equal(stateOut, intent.StateAfter) || sha256Hex(intent.StateAfter) != intent.StateAfterSHA256 {
		return fmt.Errorf("pack-memory consumption durable intent expected state mismatch")
	}
	if plan.BackupPath == "" {
		if len(intent.BackupAfter) != 0 || intent.BackupAfterSHA256 != "" {
			return fmt.Errorf("pack-memory consumption durable intent unexpected backup binding")
		}
	} else if !bytes.Equal(intent.BackupAfter, intent.TargetBefore) || intent.BackupAfterSHA256 != intent.TargetBeforeSHA256 {
		return fmt.Errorf("pack-memory consumption durable intent backup bytes mismatch")
	}
	return nil
}

func validCaseRootIdentity(identity CaseRootIdentity) bool {
	switch identity.Scheme {
	case "unix-dev-inode-v1":
		return identity.Inode != 0 && identity.VolumeSerial == 0 && identity.FileIndex == 0
	case "windows-volume-file-index-v1":
		return identity.VolumeSerial != 0 && identity.FileIndex != 0 && identity.Device == 0 && identity.Inode == 0
	default:
		return false
	}
}

func plannedStateBytes(plan Plan, before []byte, exists bool) ([]byte, error) {
	state := syncState{Managed: map[string]syncManagedEntry{}}
	if exists {
		if err := decodeStrictJSON(before, &state); err != nil {
			return nil, fmt.Errorf("decode sync state: %w", err)
		}
	}
	if state.Managed == nil {
		state.Managed = map[string]syncManagedEntry{}
	}
	state.SchemaVersion, state.TemplateRoot, state.TemplatePack = 1, plan.RepoRoot, plan.Pack
	state.Managed[plan.ManagedPath] = syncManagedEntry{SourceHash: plan.SourceSHA256, TargetHashAtSync: plan.SourceSHA256, LastAction: "pack-memory-selected-sync"}
	return canonical(state)
}

func receiptForIntent(intent consumptionIntent, doctorRows int) Receipt {
	plan := intent.Plan
	return Receipt{SchemaVersion: SchemaVersion, Kind: KindReceipt, RepoRoot: plan.RepoRoot, CaseRoot: plan.CaseRoot, CaseRootIdentity: intent.CaseRootIdentity, Pack: plan.Pack, ChangeID: plan.ChangeID, ManagedPath: plan.ManagedPath, SourceSHA256: plan.SourceSHA256, TargetSHA256Before: plan.TargetSHA256Before, TargetSHA256After: intent.TargetAfterSHA256, StateSHA256Before: plan.StateSHA256Before, StateSHA256After: intent.StateAfterSHA256, BackupPath: caseStoredPath(plan.CaseRoot, plan.BackupPath), PlanSHA256: plan.ExpectedPlanSHA256, Authority: plan.Authority, DoctorRows: doctorRows, NoAuthority: true, NoConfirmed: true, NoHeavyTool: true, Boundary: consumptionBoundary()}
}

func committedDiscovery(repo, caseRoot, pack string, change releasecheck.CompletedPackMemoryChange, receipt Receipt) (Discovery, error) {
	receiptPath, err := consumptionReceiptPath(caseRoot, change.ChangeID)
	if err != nil {
		return Discovery{}, err
	}
	status := ChangeStatus{ChangeID: change.ChangeID, ManagedPath: change.ManagedPath, SourceSHA256: change.SourceSHA256, TargetSHA256: receipt.TargetSHA256After, TargetHashAtSync: receipt.TargetSHA256After, State: "already-consumed", ReceiptPath: receiptPath}
	catalog := releasecheck.CompletedPackMemoryChangeCatalog{SchemaVersion: 1, Kind: "completed-pack-memory-change-catalog", RepoRoot: repo, Pack: pack, Changes: []releasecheck.CompletedPackMemoryChange{change}, Warnings: []string{}}
	return Discovery{SchemaVersion: SchemaVersion, Kind: KindDiscovery, RepoRoot: repo, CaseRoot: caseRoot, Pack: pack, Consumed: []ChangeStatus{status}, Catalog: catalog, NextSteps: []string{"the exact completed change is committed by its strict case-local receipt"}, Boundary: consumptionBoundary()}, nil
}

func selectedChange(catalog releasecheck.CompletedPackMemoryChangeCatalog, changeID string) (releasecheck.CompletedPackMemoryChange, bool) {
	for _, change := range catalog.Changes {
		if change.ChangeID == strings.TrimSpace(changeID) {
			return change, true
		}
	}
	return releasecheck.CompletedPackMemoryChange{}, false
}

func readReceipt(caseRoot, path string) (Receipt, bool, error) {
	root, err := openPinnedCaseRoot(caseRoot)
	if err != nil {
		return Receipt{}, false, err
	}
	defer root.Close()
	return readReceiptRoot(root, caseRelative(caseRoot, path))
}

func readReceiptRoot(root *anchoredRoot, rel string) (Receipt, bool, error) {
	data, exists, err := root.readOptional(rel, "pack-memory consumption receipt")
	if err != nil || !exists {
		return Receipt{}, exists, err
	}
	var receipt Receipt
	if err := decodeStrictJSON(data, &receipt); err != nil {
		return Receipt{}, false, fmt.Errorf("decode pack-memory consumption receipt: %w", err)
	}
	if receipt.SchemaVersion != SchemaVersion || receipt.Kind != KindReceipt || !validSHA256(receipt.PlanSHA256) {
		return Receipt{}, false, fmt.Errorf("pack-memory consumption receipt schema mismatch: %s", rel)
	}
	return receipt, true, nil
}

func readIntent(caseRoot, path string) (consumptionIntent, bool, error) {
	root, err := openPinnedCaseRoot(caseRoot)
	if err != nil {
		return consumptionIntent{}, false, err
	}
	defer root.Close()
	return readIntentRoot(root, caseRelative(caseRoot, path))
}

func readIntentRoot(root *anchoredRoot, rel string) (consumptionIntent, bool, error) {
	data, exists, err := root.readOptionalLimit(rel, "pack-memory consumption durable intent", maxConsumptionIntentBytes)
	if err != nil || !exists {
		return consumptionIntent{}, exists, err
	}
	var intent consumptionIntent
	if err := decodeStrictJSON(data, &intent); err != nil {
		return consumptionIntent{}, false, fmt.Errorf("decode pack-memory consumption durable intent: %w", err)
	}
	return intent, true, nil
}

func readOptionalAnchored(caseRoot, path, label string) ([]byte, bool, error) {
	data, err := refsf.ReadStableRegularFileAnchored(caseRoot, path, label, maxConsumptionFileBytes)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	return data, err == nil, err
}

type anchoredRoot struct {
	path     string
	root     *os.Root
	info     os.FileInfo
	identity CaseRootIdentity
}

func openPinnedCaseRoot(caseRoot string) (*anchoredRoot, error) {
	path, err := filepath.Abs(caseRoot)
	if err != nil {
		return nil, err
	}
	before, err := os.Lstat(path)
	if err != nil || !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("pack-memory consumption case root must be a non-symlink directory: %s", path)
	}
	if err := rejectReparseAncestors(path); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	opened, err := root.Lstat(".")
	if err != nil || !os.SameFile(before, opened) {
		root.Close()
		return nil, fmt.Errorf("pack-memory consumption case root changed while opening: %s", path)
	}
	identity, err := caseRootIdentity(root)
	if err != nil {
		root.Close()
		return nil, fmt.Errorf("identify pack-memory consumption case root: %w", err)
	}
	return &anchoredRoot{path: path, root: root, info: opened, identity: identity}, nil
}

func (root *anchoredRoot) Close() error { return root.root.Close() }

func currentCaseRootIdentity(caseRoot string) (CaseRootIdentity, error) {
	root, err := openPinnedCaseRoot(caseRoot)
	if err != nil {
		return CaseRootIdentity{}, err
	}
	defer root.Close()
	return root.identity, nil
}

func (root *anchoredRoot) parent(rel, label string, create bool) (*os.Root, string, error) {
	clean := filepath.Clean(filepath.FromSlash(rel))
	if filepath.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return nil, "", fmt.Errorf("%s path escapes anchored case root: %s", label, rel)
	}
	current, err := root.root.OpenRoot(".")
	if err != nil {
		return nil, "", err
	}
	walked := []string{}
	for component := range strings.SplitSeq(filepath.Dir(clean), string(filepath.Separator)) {
		if component == "." {
			continue
		}
		if component == "" || component == ".." {
			current.Close()
			return nil, "", fmt.Errorf("%s contains invalid parent component", label)
		}
		walked = append(walked, component)
		before, statErr := current.Lstat(component)
		if statErr != nil && create {
			if err := current.Mkdir(component, 0o700); err != nil && !os.IsExist(err) {
				current.Close()
				return nil, "", errors.Join(statErr, err)
			}
			before, statErr = current.Lstat(component)
		}
		if statErr != nil {
			current.Close()
			return nil, "", statErr
		}
		if !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
			current.Close()
			return nil, "", fmt.Errorf("%s parent component must be a non-symlink directory: %s", label, component)
		}
		if err := rejectReparsePath(filepath.Join(root.path, filepath.Join(walked...))); err != nil {
			current.Close()
			return nil, "", err
		}
		next, err := current.OpenRoot(component)
		if err != nil {
			current.Close()
			return nil, "", err
		}
		opened, openedErr := next.Lstat(".")
		after, afterErr := current.Lstat(component)
		if openedErr != nil || afterErr != nil || !opened.IsDir() || !os.SameFile(before, opened) || !os.SameFile(opened, after) {
			next.Close()
			current.Close()
			return nil, "", fmt.Errorf("%s parent component changed while opening: %s", label, component)
		}
		current.Close()
		current = next
	}
	return current, filepath.Base(clean), nil
}

func (root *anchoredRoot) readOptional(rel, label string) ([]byte, bool, error) {
	return root.readOptionalLimit(rel, label, maxConsumptionFileBytes)
}

func (root *anchoredRoot) readOptionalLimit(rel, label string, limit int64) ([]byte, bool, error) {
	parent, name, err := root.parent(rel, label, false)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	defer parent.Close()
	data, err := readParentFileLimit(parent, name, filepath.Join(root.path, filepath.FromSlash(rel)), label, limit)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	return data, err == nil, err
}

func readParentFile(parent *os.Root, name, path, label string) ([]byte, error) {
	return readParentFileLimit(parent, name, path, label, maxConsumptionFileBytes)
}

func readParentFileLimit(parent *os.Root, name, path, label string, limit int64) ([]byte, error) {
	before, err := parent.Lstat(name)
	if err != nil {
		return nil, err
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() || before.Size() < 1 || before.Size() > limit {
		return nil, fmt.Errorf("%s must be a bounded non-empty regular file: %s", label, path)
	}
	if err := rejectReparsePath(path); err != nil {
		return nil, err
	}
	file, err := parent.Open(name)
	if err != nil {
		return nil, err
	}
	opened, statErr := file.Stat()
	data, readErr := io.ReadAll(io.LimitReader(file, limit+1))
	closeErr := file.Close()
	after, afterErr := parent.Lstat(name)
	if statErr != nil || readErr != nil || closeErr != nil || afterErr != nil || !os.SameFile(before, opened) || !os.SameFile(opened, after) || int64(len(data)) != opened.Size() || int64(len(data)) > limit {
		return nil, fmt.Errorf("%s changed while reading: %s: %w", label, path, errors.Join(statErr, readErr, closeErr, afterErr))
	}
	return data, nil
}

func (root *anchoredRoot) writeExclusive(rel string, data []byte, label string) error {
	parent, name, err := root.parent(rel, label, true)
	if err != nil {
		return err
	}
	defer parent.Close()
	if existing, err := readParentFile(parent, name, filepath.Join(root.path, filepath.FromSlash(rel)), label); err == nil {
		if bytes.Equal(existing, data) {
			return nil
		}
		return fmt.Errorf("%s already exists with different bytes: %s", label, rel)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return writeExclusiveParent(parent, name, filepath.Join(root.path, filepath.FromSlash(rel)), data, label)
}

func (root *anchoredRoot) commitExclusiveAtomic(rel string, data []byte, label string) error {
	parent, name, err := root.parent(rel, label, true)
	if err != nil {
		return err
	}
	defer parent.Close()
	path := filepath.Join(root.path, filepath.FromSlash(rel))
	if existing, exists, err := readParentOptional(parent, name, path, label); err != nil {
		return err
	} else if exists {
		if bytes.Equal(existing, data) {
			return nil
		}
		return fmt.Errorf("%s already exists with different bytes: %s", label, rel)
	}
	nonce := make([]byte, 8)
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	temp := "." + name + ".owned-" + hex.EncodeToString(nonce) + ".tmp"
	if err := writeExclusiveParent(parent, temp, filepath.Join(filepath.Dir(path), temp), data, label+" owned temporary file"); err != nil {
		return err
	}
	if artifactCommitHook != nil {
		if err := artifactCommitHook(label, filepath.Join(filepath.Dir(path), temp)); err != nil {
			return err
		}
	}
	if err := commitNoReplace(parent, temp, name); err != nil {
		if existing, exists, readErr := readParentOptional(parent, name, path, label); readErr == nil && exists && bytes.Equal(existing, data) {
			_ = parent.Remove(temp)
			return nil
		}
		return err
	}
	published, err := readParentFile(parent, name, path, label)
	if err != nil || !bytes.Equal(published, data) {
		return fmt.Errorf("%s differs after atomic no-overwrite commit: %s: %w", label, rel, err)
	}
	return root.revalidate()
}

func commitNoReplace(parent *os.Root, temp, name string) error {
	if err := parent.Link(temp, name); err != nil {
		return err
	}
	if err := parent.Remove(temp); err != nil {
		return err
	}
	return nil
}

func (root *anchoredRoot) replaceExact(rel string, before []byte, beforeExists bool, after []byte, label string) error {
	parent, name, err := root.parent(rel, label, true)
	if err != nil {
		return err
	}
	defer parent.Close()
	path := filepath.Join(root.path, filepath.FromSlash(rel))
	current, exists, err := readParentOptional(parent, name, path, label)
	if err != nil {
		return err
	}
	if exists && bytes.Equal(current, after) {
		return nil
	}
	if exists != beforeExists || !bytes.Equal(current, before) {
		return fmt.Errorf("%s predecessor differs from durable intent: %s", label, rel)
	}
	temp := "." + name + ".pack-memory-" + sha256Hex(after)[:16] + ".tmp"
	if tempBytes, tempExists, err := readParentOptional(parent, temp, filepath.Join(filepath.Dir(path), temp), label+" temporary file"); err != nil {
		return err
	} else if tempExists && !bytes.Equal(tempBytes, after) {
		return fmt.Errorf("%s temporary file differs: %s", label, temp)
	} else if !tempExists {
		if err := writeExclusiveParent(parent, temp, filepath.Join(filepath.Dir(path), temp), after, label+" temporary file"); err != nil {
			return err
		}
	}
	if err := parent.Rename(temp, name); err != nil {
		return err
	}
	published, err := readParentFile(parent, name, path, label)
	if err != nil || !bytes.Equal(published, after) {
		return fmt.Errorf("%s differs after atomic replacement: %s: %w", label, rel, err)
	}
	return root.revalidate()
}

func readParentOptional(parent *os.Root, name, path, label string) ([]byte, bool, error) {
	data, err := readParentFile(parent, name, path, label)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	return data, err == nil, err
}

func writeExclusiveParent(parent *os.Root, name, path string, data []byte, label string) error {
	limit := maxConsumptionFileBytes
	if strings.Contains(label, "durable intent") {
		limit = maxConsumptionIntentBytes
	}
	if len(data) == 0 || len(data) > limit {
		return fmt.Errorf("%s bytes must be bounded and non-empty", label)
	}
	file, err := parent.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	written, writeErr := file.Write(data)
	syncErr := file.Sync()
	opened, statErr := file.Stat()
	closeErr := file.Close()
	after, afterErr := parent.Lstat(name)
	if writeErr != nil || written != len(data) || syncErr != nil || statErr != nil || closeErr != nil || afterErr != nil || after.Mode()&os.ModeSymlink != 0 || !after.Mode().IsRegular() || !os.SameFile(opened, after) {
		_ = parent.Remove(name)
		return fmt.Errorf("%s exclusive publication failed: %s: %w", label, path, errors.Join(writeErr, syncErr, statErr, closeErr, afterErr))
	}
	if err := rejectReparsePath(path); err != nil {
		_ = parent.Remove(name)
		return err
	}
	return nil
}

func (root *anchoredRoot) revalidate() error {
	current, err := os.Lstat(root.path)
	if err != nil || !os.SameFile(root.info, current) {
		return fmt.Errorf("pack-memory consumption case root changed during publication: %s", root.path)
	}
	return rejectReparseAncestors(root.path)
}

func planHash(plan Plan) (string, error) {
	copyPlan := plan
	copyPlan.ExpectedPlanSHA256, copyPlan.ApplyCommand = "", ""
	data, err := canonical(copyPlan)
	if err != nil {
		return "", err
	}
	return sha256Hex(data), nil
}

func planFromReceipt(change releasecheck.CompletedPackMemoryChange, receipt Receipt, receiptPath string) (Plan, error) {
	plan, err := makePlan(receipt.RepoRoot, receipt.CaseRoot, receipt.Pack, change, receipt.TargetSHA256Before, receipt.StateSHA256Before)
	if err != nil {
		return Plan{}, err
	}
	plan.Action, plan.ReceiptPath, plan.ExpectedPlanSHA256, plan.RequiresReview = "already-consumed", receiptPath, receipt.PlanSHA256, false
	plan.ApplyCommand = selectedSyncCommand(receipt.CaseRoot, receipt.Pack, receipt.ChangeID, receipt.PlanSHA256, true)
	plan.NextSteps = []string{"the exact completed change already has a current case-local consumption receipt"}
	return plan, nil
}

func canonical(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func decodeStrictJSON(data []byte, out any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		return fmt.Errorf("multiple JSON values")
	}
	return nil
}

func sha256Hex(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func validSHA256(value string) bool {
	decoded, err := hex.DecodeString(strings.TrimSpace(value))
	return err == nil && len(decoded) == sha256.Size
}

func validateArtifactID(value string) error {
	if strings.TrimSpace(value) == "" || filepath.Base(value) != value || strings.ContainsAny(value, `/\\`) {
		return fmt.Errorf("invalid completed pack-memory change id: %q", value)
	}
	return nil
}

func selectedSyncCommand(caseRoot, pack, changeID, expected string, apply bool) string {
	command := "/rekit sync -Target " + quoteCommandArg(caseRoot) + " -Pack " + quoteCommandArg(pack) + " -SelectPackMemoryChange " + quoteCommandArg(changeID)
	if apply {
		return command + " -ExpectedPackMemoryConsumptionPlanSha256 " + quoteCommandArg(expected) + " -Apply -Format json"
	}
	return command + " -WhatIf -Format json"
}

func quoteCommandArg(value string) string { return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"` }
func caseRelative(caseRoot, path string) string {
	rel, _ := filepath.Rel(caseRoot, path)
	return filepath.ToSlash(rel)
}
func caseStoredPath(caseRoot, path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	return caseRelative(caseRoot, path)
}
func samePath(left, right string) bool { return refsf.SamePath(left, right) }
func consumptionIntentPath(caseRoot, changeID string) (string, error) {
	return projectstate.Join(caseRoot, "pack-memory", "intents", changeID+".json")
}

func consumptionReceiptPath(caseRoot, changeID string) (string, error) {
	return projectstate.Join(caseRoot, "pack-memory", "consumptions", changeID+".json")
}

func mustReadSource(repo string, change releasecheck.CompletedPackMemoryChange) []byte {
	data, err := refsf.ReadStableRegularFileAnchored(repo, filepath.Join(repo, filepath.FromSlash(change.SourcePath)), "completed pack-memory change source", maxConsumptionFileBytes)
	if err != nil {
		return nil
	}
	return data
}

func mustDecodeState(data []byte) syncState {
	var state syncState
	_ = decodeStrictJSON(data, &state)
	return state
}

func consumptionBoundary() []string {
	boundary := []string{"discovery scans only the selected pack's completed verified change catalog and the explicit attached target case", "selected sync writes only one reviewed managed path, its backup when needed, sync state, and a final case-local consumption receipt", "producer verification proves reusable pack content but does not authorize target-case mutation; Apply requires the exact WhatIf plan hash", "selected sync does not execute heavy tools and does not write authority or confirmed facts"}
	sort.Strings(boundary)
	return boundary
}
