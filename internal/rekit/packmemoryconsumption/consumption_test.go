package packmemoryconsumption

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/doctor"

	"github.com/shuiyu486/re-context-kits/internal/rekit/casebind"
	"github.com/shuiyu486/re-context-kits/internal/rekit/releasecheck"
)

func TestSelectedSyncPreviewApplyReplayAndLocalConflict(t *testing.T) {
	repo, caseRoot, change := writeConsumptionFixture(t)

	discovery, err := Discover(repo, caseRoot, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	if len(discovery.Available) != 1 || discovery.Available[0].ChangeID != change.ChangeID || discovery.Available[0].State != "available" || !strings.Contains(discovery.Available[0].PreviewCommand, "-SelectPackMemoryChange") {
		t.Fatalf("unexpected discovery: %+v", discovery)
	}
	preview, err := Preview(repo, caseRoot, "fixture", change.ChangeID)
	if err != nil {
		t.Fatal(err)
	}
	if preview.IsMutation || preview.Applied || !preview.RequiresReview || preview.ExpectedPlanSHA256 == "" || preview.Action != "overwrite-managed-file-with-backup" || !strings.Contains(preview.ApplyCommand, preview.ExpectedPlanSHA256) {
		t.Fatalf("unexpected preview: %+v", preview)
	}
	if got := readConsumptionFile(t, filepath.Join(caseRoot, "memory.md")); got != "old memory\n" {
		t.Fatalf("WhatIf mutated target: %q", got)
	}

	result, err := Apply(repo, caseRoot, "fixture", change.ChangeID, preview.ExpectedPlanSHA256)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Plan.Applied || result.Plan.Replay || result.Receipt.ChangeID != change.ChangeID || result.Receipt.DoctorRows == 0 || len(result.Discovery.Consumed) != 1 || result.Discovery.Consumed[0].State != "already-consumed" {
		t.Fatalf("unexpected apply result: %+v", result)
	}
	if got := readConsumptionFile(t, filepath.Join(caseRoot, "memory.md")); got != "accepted memory\n" {
		t.Fatalf("selected sync did not consume pack bytes: %q", got)
	}
	if got := readConsumptionFile(t, result.Plan.BackupPath); got != "old memory\n" {
		t.Fatalf("selected sync backup drifted: %q", got)
	}
	if _, err := os.Stat(result.Plan.ReceiptPath); err != nil {
		t.Fatalf("missing final receipt: %v", err)
	}
	replay, err := Apply(repo, caseRoot, "fixture", change.ChangeID, preview.ExpectedPlanSHA256)
	if err != nil || !replay.Plan.Replay || !replay.Plan.Applied {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}

	if err := os.Remove(result.Plan.ReceiptPath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(caseRoot, "memory.md"), []byte("local edit\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	discovery, err = Discover(repo, caseRoot, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	if len(discovery.Conflicts) != 1 || discovery.Conflicts[0].State != "local-conflict" {
		t.Fatalf("local edit did not fail closed: %+v", discovery)
	}
	if _, err := Preview(repo, caseRoot, "fixture", change.ChangeID); err == nil || !strings.Contains(err.Error(), "not eligible") {
		t.Fatalf("local-conflict preview err=%v", err)
	}
}

func TestApplyPropagatesUnlockError(t *testing.T) {
	repo, caseRoot, change := writeConsumptionFixture(t)
	preview, err := Preview(repo, caseRoot, "fixture", change.ChangeID)
	if err != nil {
		t.Fatal(err)
	}
	oldAcquire := acquireMutationLease
	acquireMutationLease = func(string) (mutationLease, error) { return unlockErrorLease{}, nil }
	t.Cleanup(func() { acquireMutationLease = oldAcquire })
	_, err = Apply(repo, caseRoot, "fixture", change.ChangeID, preview.ExpectedPlanSHA256)
	if err == nil || !strings.Contains(err.Error(), "unlock fixture") {
		t.Fatalf("unlock error was not propagated: %v", err)
	}
}

type unlockErrorLease struct{}

func (unlockErrorLease) Unlock() error { return errors.New("unlock fixture") }

func TestApplyRecoversExactPrefixAfterDoctorFailure(t *testing.T) {
	repo, caseRoot, change := writeConsumptionFixture(t)
	preview, err := Preview(repo, caseRoot, "fixture", change.ChangeID)
	if err != nil {
		t.Fatal(err)
	}
	oldDoctor := doctorCase
	calls := 0
	doctorCase = func(repoRoot, caseRoot, pack string) ([]doctor.Row, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("doctor fixture")
		}
		return oldDoctor(repoRoot, caseRoot, pack)
	}
	defer func() { doctorCase = oldDoctor }()
	if _, err := Apply(repo, caseRoot, "fixture", change.ChangeID, preview.ExpectedPlanSHA256); err == nil || !strings.Contains(err.Error(), "doctor fixture") {
		t.Fatalf("expected doctor failure, got %v", err)
	}
	if _, err := os.Stat(consumptionIntentPath(caseRoot, change.ChangeID)); err != nil {
		t.Fatalf("durable intent missing after failure: %v", err)
	}
	if _, err := os.Stat(preview.ReceiptPath); !os.IsNotExist(err) {
		t.Fatalf("receipt committed before doctor passed: %v", err)
	}
	result, err := Apply(repo, caseRoot, "fixture", change.ChangeID, preview.ExpectedPlanSHA256)
	if err != nil || !result.Plan.Applied {
		t.Fatalf("exact-prefix recovery failed: %+v %v", result, err)
	}
}

func TestPendingRecoveryIgnoresProducerCatalogAndSourceDrift(t *testing.T) {
	repo, caseRoot, change := writeConsumptionFixture(t)
	preview, err := Preview(repo, caseRoot, "fixture", change.ChangeID)
	if err != nil {
		t.Fatal(err)
	}
	oldDoctor := doctorCase
	doctorCase = func(string, string, string) ([]doctor.Row, error) { return nil, errors.New("pending") }
	if _, err := Apply(repo, caseRoot, "fixture", change.ChangeID, preview.ExpectedPlanSHA256); err == nil {
		t.Fatal("expected pending Apply failure")
	}
	doctorCase = oldDoctor
	oldBuilder := completedCatalogBuilder
	completedCatalogBuilder = func(string, string) (releasecheck.CompletedPackMemoryChangeCatalog, error) {
		return releasecheck.CompletedPackMemoryChangeCatalog{}, errors.New("producer catalog unavailable")
	}
	writeConsumptionFile(t, preview.SourcePath, "producer source drift\n")
	result, err := Apply(repo, caseRoot, "fixture", change.ChangeID, preview.ExpectedPlanSHA256)
	completedCatalogBuilder = oldBuilder
	t.Cleanup(func() { doctorCase = oldDoctor; completedCatalogBuilder = oldBuilder })
	if err != nil || !result.Plan.Applied {
		t.Fatalf("case-local pending recovery depended on producer: %+v %v", result, err)
	}
	if got := readConsumptionFile(t, preview.TargetPath); got != "accepted memory\n" {
		t.Fatalf("recovery did not use intent bytes: %q", got)
	}
}

func TestCommittedReplayIgnoresProducerCatalogAndSourceDrift(t *testing.T) {
	repo, caseRoot, change := writeConsumptionFixture(t)
	preview, _ := Preview(repo, caseRoot, "fixture", change.ChangeID)
	if _, err := Apply(repo, caseRoot, "fixture", change.ChangeID, preview.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(consumptionIntentPath(caseRoot, change.ChangeID)); err != nil {
		t.Fatal(err)
	}
	oldBuilder := completedCatalogBuilder
	completedCatalogBuilder = func(string, string) (releasecheck.CompletedPackMemoryChangeCatalog, error) {
		return releasecheck.CompletedPackMemoryChangeCatalog{}, errors.New("producer catalog unavailable")
	}
	writeConsumptionFile(t, preview.SourcePath, "producer source drift\n")
	t.Cleanup(func() { completedCatalogBuilder = oldBuilder })
	replay, err := Apply(repo, caseRoot, "fixture", change.ChangeID, preview.ExpectedPlanSHA256)
	if err != nil || !replay.Plan.Replay {
		t.Fatalf("committed replay depended on producer: %+v %v", replay, err)
	}
}

func TestApplyRejectsCommittedReceiptMovedToReplacementCaseRoot(t *testing.T) {
	repo, caseRoot, change := writeConsumptionFixture(t)
	preview, err := Preview(repo, caseRoot, "fixture", change.ChangeID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(repo, caseRoot, "fixture", change.ChangeID, preview.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(consumptionIntentPath(caseRoot, change.ChangeID)); err != nil {
		t.Fatal(err)
	}

	replacement := caseRoot + "-receipt-replacement"
	if err := os.CopyFS(replacement, os.DirFS(caseRoot)); err != nil {
		t.Fatal(err)
	}
	moved := caseRoot + "-receipt-original"
	if err := os.Rename(caseRoot, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, caseRoot); err != nil {
		t.Fatal(err)
	}

	_, err = Apply(repo, caseRoot, "fixture", change.ChangeID, preview.ExpectedPlanSHA256)
	if err == nil || !strings.Contains(err.Error(), "case-root identity mismatch") {
		t.Fatalf("replacement case root replayed the moved receipt: %v", err)
	}
}

func TestApplyRejectsDurableIntentMovedToReplacementCaseRoot(t *testing.T) {
	repo, caseRoot, change := writeConsumptionFixture(t)
	preview, err := Preview(repo, caseRoot, "fixture", change.ChangeID)
	if err != nil {
		t.Fatal(err)
	}
	oldDoctor := doctorCase
	doctorCase = func(string, string, string) ([]doctor.Row, error) {
		return nil, errors.New("leave durable intent pending")
	}
	if _, err := Apply(repo, caseRoot, "fixture", change.ChangeID, preview.ExpectedPlanSHA256); err == nil {
		t.Fatal("expected first Apply to leave a pending intent")
	}
	doctorCase = oldDoctor
	t.Cleanup(func() { doctorCase = oldDoctor })

	intentPath := consumptionIntentPath(caseRoot, change.ChangeID)
	if _, err := os.Stat(intentPath); err != nil {
		t.Fatalf("durable intent was not committed: %v", err)
	}
	replacement := caseRoot + "-replacement"
	if err := os.CopyFS(replacement, os.DirFS(caseRoot)); err != nil {
		t.Fatal(err)
	}
	moved := caseRoot + "-original"
	if err := os.Rename(caseRoot, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, caseRoot); err != nil {
		t.Fatal(err)
	}

	beforeTarget := readConsumptionFile(t, preview.TargetPath)
	_, err = Apply(repo, caseRoot, "fixture", change.ChangeID, preview.ExpectedPlanSHA256)
	if err == nil || !strings.Contains(err.Error(), "case-root identity mismatch") {
		t.Fatalf("replacement case root accepted the moved durable intent: %v", err)
	}
	if got := readConsumptionFile(t, preview.TargetPath); got != beforeTarget {
		t.Fatalf("replacement case root was mutated: got %q want %q", got, beforeTarget)
	}
	if _, statErr := os.Stat(preview.ReceiptPath); !os.IsNotExist(statErr) {
		t.Fatalf("replacement case root received a consumption receipt: %v", statErr)
	}
}

func TestApplyRejectsCaseRootRebindAfterIntent(t *testing.T) {
	repo, caseRoot, change := writeConsumptionFixture(t)
	preview, _ := Preview(repo, caseRoot, "fixture", change.ChangeID)
	oldHook := artifactCommitHook
	renameBlocked := false
	artifactCommitHook = func(label, _ string) error {
		if !strings.Contains(label, "durable intent") {
			return nil
		}
		moved := caseRoot + "-pinned"
		if err := os.Rename(caseRoot, moved); err != nil {
			renameBlocked = true
			return err
		}
		return os.MkdirAll(caseRoot, 0o700)
	}
	t.Cleanup(func() { artifactCommitHook = oldHook })
	_, err := Apply(repo, caseRoot, "fixture", change.ChangeID, preview.ExpectedPlanSHA256)
	if err == nil {
		t.Fatal("root rebind was accepted")
	}
	if runtime.GOOS == "windows" {
		if !renameBlocked {
			t.Fatalf("Windows pinned handle did not block root rename: %v", err)
		}
	} else if !strings.Contains(err.Error(), "case root changed") {
		t.Fatalf("root identity guard did not reject rebind: %v", err)
	}
}

func TestAtomicArtifactCommitLeavesFinalAbsentOnFaultAndRecovers(t *testing.T) {
	for _, kind := range []string{"intent", "receipt"} {
		t.Run(kind, func(t *testing.T) {
			repo, caseRoot, change := writeConsumptionFixture(t)
			preview, _ := Preview(repo, caseRoot, "fixture", change.ChangeID)
			oldHook := artifactCommitHook
			failed := false
			artifactCommitHook = func(label, temp string) error {
				if !failed && strings.Contains(label, kind) {
					failed = true
					return errors.New("artifact commit fault after synced temp: " + temp)
				}
				return nil
			}
			_, err := Apply(repo, caseRoot, "fixture", change.ChangeID, preview.ExpectedPlanSHA256)
			artifactCommitHook = oldHook
			t.Cleanup(func() { artifactCommitHook = oldHook })
			if err == nil || !failed {
				t.Fatalf("expected %s commit fault: %v", kind, err)
			}
			finalPath := consumptionIntentPath(caseRoot, change.ChangeID)
			if kind == "receipt" {
				finalPath = preview.ReceiptPath
			}
			if _, err := os.Stat(finalPath); !os.IsNotExist(err) {
				t.Fatalf("%s final path appeared before atomic commit: %v", kind, err)
			}
			result, err := Apply(repo, caseRoot, "fixture", change.ChangeID, preview.ExpectedPlanSHA256)
			if err != nil || !result.Plan.Applied {
				t.Fatalf("%s torn-temp recovery failed: %+v %v", kind, result, err)
			}
		})
	}
}

func TestApplyRejectsNonPrefixRecovery(t *testing.T) {
	repo, caseRoot, change := writeConsumptionFixture(t)
	preview, err := Preview(repo, caseRoot, "fixture", change.ChangeID)
	if err != nil {
		t.Fatal(err)
	}
	oldHook := publicationHook
	publicationHook = func(_ Plan, index int) error {
		if index == 1 {
			return errors.New("stop after backup")
		}
		return nil
	}
	if _, err := Apply(repo, caseRoot, "fixture", change.ChangeID, preview.ExpectedPlanSHA256); err == nil {
		t.Fatal("expected interrupted publication")
	}
	publicationHook = oldHook
	t.Cleanup(func() { publicationHook = oldHook })
	intent, _, err := readIntent(caseRoot, consumptionIntentPath(caseRoot, change.ChangeID))
	if err != nil {
		t.Fatal(err)
	}
	writeConsumptionFile(t, preview.StatePath, string(intent.StateAfter))
	if _, err := Apply(repo, caseRoot, "fixture", change.ChangeID, preview.ExpectedPlanSHA256); err == nil || !strings.Contains(err.Error(), "non-prefix") {
		t.Fatalf("non-prefix recovery did not fail closed: %v", err)
	}
}

func TestReceiptForgeryAndDriftFailClosed(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Receipt, string, releasecheck.CompletedPackMemoryChange)
		drift  func(string, Receipt)
	}{
		{name: "authority", mutate: func(receipt *Receipt, _ string, _ releasecheck.CompletedPackMemoryChange) {
			receipt.Authority.AuthoritySHA256 = strings.Repeat("c", 64)
		}},
		{name: "repo", mutate: func(receipt *Receipt, _ string, _ releasecheck.CompletedPackMemoryChange) {
			receipt.RepoRoot += "-forged"
		}},
		{name: "plan", mutate: func(receipt *Receipt, _ string, _ releasecheck.CompletedPackMemoryChange) {
			receipt.PlanSHA256 = strings.Repeat("d", 64)
		}},
		{name: "backup", drift: func(caseRoot string, receipt Receipt) {
			writeConsumptionFile(t, filepath.Join(caseRoot, filepath.FromSlash(receipt.BackupPath)), "forged backup\n")
		}},
		{name: "state", drift: func(caseRoot string, _ Receipt) {
			data := readConsumptionFile(t, filepath.Join(caseRoot, ".rekit", "state.json"))
			writeConsumptionFile(t, filepath.Join(caseRoot, ".rekit", "state.json"), strings.Replace(data, "pack-memory-selected-sync", "forged", 1))
		}},
		{name: "target", drift: func(caseRoot string, _ Receipt) {
			writeConsumptionFile(t, filepath.Join(caseRoot, "memory.md"), "forged target\n")
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo, caseRoot, change := writeConsumptionFixture(t)
			preview, _ := Preview(repo, caseRoot, "fixture", change.ChangeID)
			result, err := Apply(repo, caseRoot, "fixture", change.ChangeID, preview.ExpectedPlanSHA256)
			if err != nil {
				t.Fatal(err)
			}
			receipt := result.Receipt
			if tc.mutate != nil {
				tc.mutate(&receipt, caseRoot, change)
				data, _ := canonical(receipt)
				writeConsumptionFile(t, preview.ReceiptPath, string(data))
			}
			if tc.drift != nil {
				tc.drift(caseRoot, receipt)
			}
			if _, err := Apply(repo, caseRoot, "fixture", change.ChangeID, preview.ExpectedPlanSHA256); err == nil {
				t.Fatal("forged or drifted receipt replay succeeded")
			}
		})
	}
}

func TestApplyRejectsStaleSourceTargetAndState(t *testing.T) {
	for _, name := range []string{"source", "target", "state"} {
		t.Run(name, func(t *testing.T) {
			repo, caseRoot, change := writeConsumptionFixture(t)
			preview, err := Preview(repo, caseRoot, "fixture", change.ChangeID)
			if err != nil {
				t.Fatal(err)
			}
			switch name {
			case "source":
				writeConsumptionFile(t, preview.SourcePath, "source drift\n")
			case "target":
				writeConsumptionFile(t, preview.TargetPath, "target drift\n")
			case "state":
				writeConsumptionFile(t, preview.StatePath, syncStateJSON(t, repo, strings.Repeat("e", 64)))
			}
			if _, err := Apply(repo, caseRoot, "fixture", change.ChangeID, preview.ExpectedPlanSHA256); err == nil {
				t.Fatalf("stale %s was accepted", name)
			}
		})
	}
}

func TestApplyConcurrentExactReplay(t *testing.T) {
	repo, caseRoot, change := writeConsumptionFixture(t)
	preview, err := Preview(repo, caseRoot, "fixture", change.ChangeID)
	if err != nil {
		t.Fatal(err)
	}
	const workers = 4
	results := make(chan Result, workers)
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := Apply(repo, caseRoot, "fixture", change.ChangeID, preview.ExpectedPlanSHA256)
			results <- result
			errs <- err
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent Apply failed: %v", err)
		}
	}
	replays := 0
	for result := range results {
		if result.Plan.Replay {
			replays++
		}
	}
	if replays != workers-1 {
		t.Fatalf("got %d replays, want %d", replays, workers-1)
	}
}

func TestApplyRejectsSymlinkParents(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ordinary symlink creation may require Windows developer mode; junction coverage is in reparse_windows_test.go")
	}
	for _, name := range []string{"target", "state"} {
		t.Run(name, func(t *testing.T) {
			repo, caseRoot, change := writeConsumptionFixture(t)
			preview, _ := Preview(repo, caseRoot, "fixture", change.ChangeID)
			outside := t.TempDir()
			if name == "target" {
				if err := os.Remove(preview.TargetPath); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(outside, "target"), preview.TargetPath); err != nil {
					t.Fatal(err)
				}
			} else {
				stateDir := filepath.Dir(preview.StatePath)
				moved := stateDir + "-real"
				if err := os.Rename(stateDir, moved); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, stateDir); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := Apply(repo, caseRoot, "fixture", change.ChangeID, preview.ExpectedPlanSHA256); err == nil {
				t.Fatal("symlink parent/target was accepted")
			}
		})
	}
}

func TestIntentBindsExactPlanAndOriginalBytes(t *testing.T) {
	repo, caseRoot, change := writeConsumptionFixture(t)
	preview, _ := Preview(repo, caseRoot, "fixture", change.ChangeID)
	oldDoctor := doctorCase
	doctorCase = func(string, string, string) ([]doctor.Row, error) { return nil, errors.New("stop") }
	_, _ = Apply(repo, caseRoot, "fixture", change.ChangeID, preview.ExpectedPlanSHA256)
	doctorCase = oldDoctor
	intent, exists, err := readIntent(caseRoot, consumptionIntentPath(caseRoot, change.ChangeID))
	if err != nil || !exists {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(intent.Plan, preview) || string(intent.TargetBefore) != "old memory\n" || sha256Hex(intent.StateBefore) != preview.StateSHA256Before || intent.TargetAfterSHA256 != change.SourceSHA256 {
		t.Fatalf("intent does not bind exact plan and bytes: %+v", intent)
	}
	if _, err := os.Stat(preview.ReceiptPath); !os.IsNotExist(err) {
		t.Fatalf("receipt unexpectedly committed: %v", err)
	}
	t.Cleanup(func() { doctorCase = oldDoctor })
}

func writeConsumptionFixture(t *testing.T) (string, string, releasecheck.CompletedPackMemoryChange) {
	t.Helper()
	repo := t.TempDir()
	caseRoot := filepath.Join(t.TempDir(), "consumer")
	packRoot := filepath.Join(repo, "packs", "fixture")
	sourceRepo, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	canonicalSkill := "底层 Go CLI 是 canonical runtime\n`rekit.ps1` 只是 retained compatibility façade\ncase 只生成 `.claude/skills/rekit/SKILL.md` 薄 shim\n底层 runtime 只作为 `/rekit` 的内部实现\n"
	writeConsumptionFile(t, filepath.Join(repo, ".claude", "skills", "rekit", "SKILL.md"), canonicalSkill)
	copyConsumptionFixtureFile(t, filepath.Join(sourceRepo, "rekit", "templates", "case-shim", "SKILL.md"), filepath.Join(repo, "rekit", "templates", "case-shim", "SKILL.md"))
	writeConsumptionFile(t, filepath.Join(packRoot, "manifest.yml"), `schemaVersion: 1
name: fixture
version: 1.0.0
description: fixture
maturity: experimental
managedFiles:
  - memory.md
templateFiles: []
localNeverOverwrite: []
promoteFiles:
  - memory.md
commonPolicies: []
policyOverlays: []
toolingFiles: []
promptFiles: []
laneTypes:
  - id: main
    title: Main
    authority: true
    workspaceRoot: workspace/main
    canWrite: memory.md
    readOnly: .rekit/facts/**
    outputs: publication
  - id: feature
    title: Feature
    authority: false
    workspaceRoot: workspace/features
    canWrite: own-workspace
    readOnly: memory.md
    outputs: observation
authorityFiles:
  - memory.md
toolingCandidateSources:
  - memory.md
subagentRoutes: []
heavyToolGates:
  - id: debug
    title: Debug
    sideEffects: debug,filesystem-write
    defaultRisk: high
    requiresConfirmation: true
    stopConditions: timeout
promoteDenyPatterns:
  - "artifacts[\\/]"
managedBlock:
  file: CLAUDE.md
  blockId: rekit:fixture
  source: managed-block.md
syncPolicy:
  managedFiles: overwrite-with-backup
  templateFiles: create-if-missing
  localFiles: never-overwrite
workstreamDefaults:
  defaultAuthorityLane: main
  defaultStartLaneType: feature
  backupRoot: .rekit/backups/sync
  requestDefaultTargetLane: main
budgets:
  defaultMarkdown: 16384
`)
	writeConsumptionFile(t, filepath.Join(packRoot, "memory.md"), "accepted memory\n")
	writeConsumptionFile(t, filepath.Join(packRoot, "managed-block.md"), "managed block\n")
	writeConsumptionFile(t, filepath.Join(repo, "common", "policies", "manifest.yml"), "policies:\n")
	writeConsumptionFile(t, filepath.Join(repo, "common", "policies", "README.md"), "policies\n")
	writeConsumptionFile(t, filepath.Join(packRoot, "policies", "manifest.yml"), "overlays:\n")
	writeConsumptionFile(t, filepath.Join(packRoot, "policies", "README.md"), "overlays\n")
	if err := os.MkdirAll(caseRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := casebind.WriteInstance(caseRoot, repo, "fixture", "consumer"); err != nil {
		t.Fatal(err)
	}
	if _, err := casebind.WriteLegacyMetadataForAttach(caseRoot, repo, "fixture"); err != nil {
		t.Fatal(err)
	}
	if _, err := casebind.WriteCaseShim(caseRoot, repo); err != nil {
		t.Fatal(err)
	}
	writeConsumptionFile(t, filepath.Join(caseRoot, "memory.md"), "old memory\n")
	writeConsumptionFile(t, filepath.Join(caseRoot, "CLAUDE.md"), "<!-- BEGIN rekit:fixture -->\nmanaged block\n<!-- END rekit:fixture -->\n")
	writeConsumptionFile(t, filepath.Join(caseRoot, ".rekit", "state.json"), syncStateJSON(t, repo, sha256Hex([]byte("old memory\n"))))

	change := releasecheck.CompletedPackMemoryChange{ChangeID: "pack-memory-change-" + strings.Repeat("a", 64), ManagedPath: "memory.md", SourcePath: "packs/fixture/memory.md", SourceSHA256: sha256Hex([]byte("accepted memory\n")), AuthoritySHA256: strings.Repeat("b", 64)}
	oldBuilder := completedCatalogBuilder
	completedCatalogBuilder = func(repoRoot, pack string) (releasecheck.CompletedPackMemoryChangeCatalog, error) {
		return releasecheck.CompletedPackMemoryChangeCatalog{SchemaVersion: 1, Kind: "completed-pack-memory-change-catalog", RepoRoot: repo, Pack: "fixture", Changes: []releasecheck.CompletedPackMemoryChange{change}, Warnings: []string{}}, nil
	}
	t.Cleanup(func() { completedCatalogBuilder = oldBuilder })
	return repo, caseRoot, change
}

func syncStateJSON(t *testing.T, repo, hash string) string {
	t.Helper()
	state := syncState{SchemaVersion: 1, TemplateRoot: repo, TemplatePack: "fixture", LastSyncAt: "2026-08-05T00:00:00Z", Managed: map[string]syncManagedEntry{"memory.md": {SourceHash: hash, TargetHashAtSync: hash, LastAction: "sync"}}}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return string(data) + "\n"
}

func copyConsumptionFixtureFile(t *testing.T, source, target string) {
	t.Helper()
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	writeConsumptionFile(t, target, string(data))
}

func writeConsumptionFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readConsumptionFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
