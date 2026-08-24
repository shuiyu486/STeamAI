package statemigration

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/casebind"
	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/plancontract"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtimebundle"
)

func TestPreviewIsZeroWriteAndApplyPreservesDurableBytes(t *testing.T) {
	fixture := newMigrationFixture(t)
	before := snapshotTree(t, fixture.caseRoot)
	plan, err := Preview(fixture.repoRoot, fixture.caseRoot, fixture.pack)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != "ready-to-migrate" || plan.IsMutation || plan.Applied || plan.Replay || plan.AlreadyCurrent || !plan.RequiresReview || !plan.RequiresConfirmation || !validSHA256(plan.ExpectedPlanSHA256) || plan.LegacyInventory.Files < 5 {
		t.Fatalf("unexpected migration preview: %+v", plan)
	}
	for _, binding := range []FileBinding{plan.LegacyInstance, plan.LegacyState, plan.LegacySkill, plan.CurrentInstance, plan.CurrentState, plan.CurrentSkill, plan.BundleManifest} {
		if filepath.IsAbs(filepath.FromSlash(binding.Path)) || !filepath.IsLocal(filepath.FromSlash(binding.Path)) {
			t.Fatalf("migration plan binding escapes the project-local namespace: %+v", binding)
		}
	}
	for _, write := range plan.Writes {
		if filepath.IsAbs(filepath.FromSlash(write.Path)) || !filepath.IsLocal(filepath.FromSlash(write.Path)) {
			t.Fatalf("migration write escapes the project-local namespace: %+v", write)
		}
	}
	if len(plan.ApplyArgs) < 1 || !containsExactArg(plan.ApplyArgs, "-ExpectedMigrationPlanSha256", plan.ExpectedPlanSHA256) || !containsExactFlag(plan.ApplyArgs, "-Apply") || plan.ApplyCommand == "" {
		t.Fatalf("migration preview omits exact hash-bound Apply carrier: command=%q args=%+v", plan.ApplyCommand, plan.ApplyArgs)
	}
	commandAction, err := commands.ExactActionFromCommand(plan.ApplyCommand)
	if err != nil {
		t.Fatal(err)
	}
	argsAction, err := commands.ExactActionFromCLIArgs(plan.ApplyArgs)
	if err != nil || !commandAction.Equivalent(argsAction) || !strings.HasPrefix(plan.ApplyCommand, commands.LegacyPublicEntrypoint+" migrate-state ") {
		t.Fatalf("migration exact Apply carrier drifted: command=%q args=%v err=%v", plan.ApplyCommand, plan.ApplyArgs, err)
	}
	if err := commandAction.ValidatePlanApply(commands.MigrateState, plan.ExpectedPlanSHA256); err != nil {
		t.Fatalf("migration exact Apply binding: %v", err)
	}
	receiptWrite, ok := migrationWrite(plan.Writes, ReceiptRel)
	if !ok || receiptWrite.Kind != "state-root-migration-receipt" || receiptWrite.Action != "create" || receiptWrite.SHA256 != "" || receiptWrite.Size != 0 {
		t.Fatalf("migration preview omits the plan-bound self-referential receipt publication: %+v", receiptWrite)
	}
	if after := snapshotTree(t, fixture.caseRoot); !equalSnapshot(before, after) {
		t.Fatalf("preview mutated the project: before=%v after=%v", before, after)
	}

	result, err := Apply(fixture.repoRoot, fixture.caseRoot, fixture.pack, plan.ExpectedPlanSHA256)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied || !result.IsMutation || result.Replay || result.AlreadyCurrent || result.Status != "migrated" || result.Receipt == nil || result.Receipt.PlanSHA256 != plan.ExpectedPlanSHA256 {
		t.Fatalf("unexpected migration result: %+v", result)
	}
	assertMissing(t, filepath.Join(fixture.caseRoot, ".rekit"))
	assertMissing(t, filepath.Join(fixture.caseRoot, ".claude", "skills", "rekit", "SKILL.md"))
	assertFile(t, filepath.Join(fixture.caseRoot, ".claude", "skills", "steamai", "SKILL.md"))
	currentRoot, currentIdentity, err := OpenRootIdentity(filepath.Join(fixture.caseRoot, ".steamai"))
	if err != nil {
		t.Fatal(err)
	}
	if err := currentRoot.Close(); err != nil {
		t.Fatal(err)
	}
	if currentIdentity != plan.SourceRootIdentity || result.Receipt.Before.RootIdentity != result.Receipt.After.RootIdentity || result.Receipt.After.RootIdentity != plan.SourceRootIdentity {
		t.Fatalf("same-parent rename changed filesystem identity: plan=%+v current=%+v receipt=%+v", plan.SourceRootIdentity, currentIdentity, result.Receipt)
	}
	for rel, expected := range fixture.durable {
		actual, err := os.ReadFile(filepath.Join(fixture.caseRoot, ".steamai", filepath.FromSlash(rel)))
		if err != nil || !bytes.Equal(actual, expected) {
			t.Fatalf("durable bytes changed for %s: err=%v actual=%q expected=%q", rel, err, actual, expected)
		}
	}
	legacyMetadataAfter, err := os.ReadFile(filepath.Join(fixture.caseRoot, ".re-template.yml"))
	if err != nil || result.Receipt.LegacyMetadata == nil || sha256Hex(legacyMetadataAfter) != result.Receipt.LegacyMetadata.SHA256 || int64(len(legacyMetadataAfter)) != result.Receipt.LegacyMetadata.Size {
		t.Fatalf("legacy compatibility metadata changed or lost its receipt binding: err=%v receipt=%+v", err, result.Receipt.LegacyMetadata)
	}
	receiptBytes, err := os.ReadFile(filepath.Join(fixture.caseRoot, filepath.FromSlash(ReceiptRel)))
	if err != nil {
		t.Fatal(err)
	}
	for _, absolute := range []string{fixture.caseRoot, fixture.repoRoot} {
		if bytes.Contains(receiptBytes, []byte(absolute)) {
			t.Fatalf("migration receipt leaked absolute project or repository path %q: %s", absolute, receiptBytes)
		}
	}
	assertMissing(t, filepath.Join(fixture.caseRoot, ".steamai", "migration", ".state-root-v1.json.state-migration.tmp"))
	metadata, err := os.ReadFile(filepath.Join(fixture.caseRoot, ".steamai", "instance.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(metadata, []byte(fixture.caseRoot)) || bytes.Contains(metadata, []byte(fixture.repoRoot)) || !bytes.Contains(metadata, []byte("templateRoot: .")) || !bytes.Contains(metadata, []byte("projectRoot: ..")) {
		t.Fatalf("migrated metadata is not relocatable: %s", metadata)
	}
	state, err := os.ReadFile(filepath.Join(fixture.caseRoot, ".steamai", "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(state, []byte(fixture.repoRoot)) || !bytes.Contains(state, []byte(`"templateRoot": "."`)) {
		t.Fatalf("migrated sync state is not relocatable: %s", state)
	}
	inst, err := instance.Read(fixture.caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	if inst.Source != "steamai" || inst.SchemaVersion != 2 || inst.Mode != "project-local-bundle" || inst.Moved() || inst.TemplateRoot != filepath.Join(fixture.caseRoot, ".steamai") || inst.ProjectRoot != fixture.caseRoot {
		t.Fatalf("unexpected migrated instance: %+v", inst)
	}
}

func TestPreviewRejectsMissingLegacyMetadata(t *testing.T) {
	fixture := newMigrationFixture(t)
	if err := os.Remove(filepath.Join(fixture.caseRoot, ".re-template.yml")); err != nil {
		t.Fatal(err)
	}
	if _, err := Preview(fixture.repoRoot, fixture.caseRoot, fixture.pack); err == nil || !strings.Contains(err.Error(), "requires complete legacy metadata") {
		t.Fatalf("expected missing legacy metadata rejection, got %v", err)
	}
}

func TestApplyRejectsLegacySkillResidue(t *testing.T) {
	fixture := newMigrationFixture(t)
	writeFixtureFile(t, filepath.Join(fixture.caseRoot, ".claude", "skills", "rekit", "extra.txt"), []byte("residue"))
	if _, err := Preview(fixture.repoRoot, fixture.caseRoot, fixture.pack); err == nil || !strings.Contains(err.Error(), "unplanned residue") {
		t.Fatalf("expected legacy skill residue rejection, got %v", err)
	}
}

func TestApplyRejectsInvalidSHAAndLegacyDrift(t *testing.T) {
	fixture := newMigrationFixture(t)
	plan, err := Preview(fixture.repoRoot, fixture.caseRoot, fixture.pack)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Apply(fixture.repoRoot, fixture.caseRoot, fixture.pack, "bad")
	failure, typed := plancontract.FromError(err)
	if err == nil || !typed || failure.Code != plancontract.CodePlanInvalid || failure.MutationApplied || failure.MutationBoundary != "none" {
		t.Fatalf("invalid migration plan failure=%+v typed=%t err=%v", failure, typed, err)
	}
	path := filepath.Join(fixture.caseRoot, ".rekit", "facts", "mission.jsonl")
	if err := os.WriteFile(path, []byte("drift\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = Apply(fixture.repoRoot, fixture.caseRoot, fixture.pack, plan.ExpectedPlanSHA256)
	failure, typed = plancontract.FromError(err)
	if err == nil || !typed || failure.Code != plancontract.CodePlanMismatch || failure.MutationApplied || failure.MutationBoundary != "none" {
		t.Fatalf("expected legacy inventory drift to change plan hash, got %v failure=%+v typed=%t", err, failure, typed)
	}
	assertFile(t, filepath.Join(fixture.caseRoot, ".rekit", "instance.yml"))
	assertMissing(t, filepath.Join(fixture.caseRoot, ".steamai"))
}

func TestApplyRejectsLateCurrentnessDrift(t *testing.T) {
	fixture := newMigrationFixture(t)
	plan, err := Preview(fixture.repoRoot, fixture.caseRoot, fixture.pack)
	if err != nil {
		t.Fatal(err)
	}
	restore := SetBeforeCommitHookForTest(func() error {
		return os.WriteFile(filepath.Join(fixture.caseRoot, ".steamai"), []byte("collision"), 0o600)
	})
	defer restore()
	if _, err := Apply(fixture.repoRoot, fixture.caseRoot, fixture.pack, plan.ExpectedPlanSHA256); err == nil || !strings.Contains(err.Error(), ".steamai is no longer missing") {
		t.Fatalf("expected late target drift to fail, got %v", err)
	}
	assertFile(t, filepath.Join(fixture.caseRoot, ".rekit", "instance.yml"))
}

func TestApplyRejectsRuntimeSourceReplacementAfterPreview(t *testing.T) {
	fixture := newMigrationFixture(t)
	plan, err := Preview(fixture.repoRoot, fixture.caseRoot, fixture.pack)
	if err != nil {
		t.Fatal(err)
	}
	runtimeSource, err := runtimebundle.SourceExecutable()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(runtimeSource, []byte("replaced runtime executable"), 0o700); err != nil {
		t.Fatal(err)
	}
	_, err = Apply(fixture.repoRoot, fixture.caseRoot, fixture.pack, plan.ExpectedPlanSHA256)
	failure, typed := plancontract.FromError(err)
	if err == nil || !typed || failure.Code != plancontract.CodePlanMismatch || failure.MutationApplied || failure.MutationBoundary != "none" {
		t.Fatalf("expected replaced runtime source to invalidate the exact plan, got %v failure=%+v typed=%t", err, failure, typed)
	}
	assertFile(t, filepath.Join(fixture.caseRoot, ".rekit", "instance.yml"))
	assertMissing(t, filepath.Join(fixture.caseRoot, ".steamai"))
}

func TestApplyRejectsLateLegacyInventoryAndPublicationTargetDrift(t *testing.T) {
	t.Run("durable inventory", func(t *testing.T) {
		fixture := newMigrationFixture(t)
		plan, err := Preview(fixture.repoRoot, fixture.caseRoot, fixture.pack)
		if err != nil {
			t.Fatal(err)
		}
		restore := SetBeforeCommitHookForTest(func() error {
			return os.WriteFile(filepath.Join(fixture.caseRoot, ".rekit", "facts", "mission.jsonl"), []byte("late drift\n"), 0o600)
		})
		defer restore()
		if _, err := Apply(fixture.repoRoot, fixture.caseRoot, fixture.pack, plan.ExpectedPlanSHA256); err == nil || !strings.Contains(err.Error(), "inventory changed immediately before migration commit") {
			t.Fatalf("expected late inventory drift to fail, got %v", err)
		}
		assertFile(t, filepath.Join(fixture.caseRoot, ".rekit", "instance.yml"))
		assertMissing(t, filepath.Join(fixture.caseRoot, ".steamai"))
	})

	t.Run("publication target", func(t *testing.T) {
		fixture := newMigrationFixture(t)
		plan, err := Preview(fixture.repoRoot, fixture.caseRoot, fixture.pack)
		if err != nil {
			t.Fatal(err)
		}
		target := ""
		for _, write := range plan.Writes {
			if write.Action == "create" && strings.HasPrefix(write.Path, ".steamai/") && write.Path != ReceiptRel {
				target = strings.TrimPrefix(write.Path, ".steamai/")
				break
			}
		}
		if target == "" {
			t.Fatal("migration preview has no create-only state publication")
		}
		restore := SetBeforeCommitHookForTest(func() error {
			return os.MkdirAll(filepath.Join(fixture.caseRoot, ".rekit", filepath.FromSlash(target)), 0o700)
		})
		defer restore()
		if _, err := Apply(fixture.repoRoot, fixture.caseRoot, fixture.pack, plan.ExpectedPlanSHA256); err == nil || !strings.Contains(err.Error(), "inventory changed immediately before migration commit") {
			t.Fatalf("expected late publication target drift to fail, got %v", err)
		}
		assertFile(t, filepath.Join(fixture.caseRoot, ".rekit", "instance.yml"))
		assertMissing(t, filepath.Join(fixture.caseRoot, ".steamai"))
	})
}

func TestPostRenameFailureLeavesDeterministicPartialCurrentRoot(t *testing.T) {
	fixture := newMigrationFixture(t)
	plan, err := Preview(fixture.repoRoot, fixture.caseRoot, fixture.pack)
	if err != nil {
		t.Fatal(err)
	}
	restore := SetAfterStateRenameHookForTest(func() error { return os.ErrPermission })
	defer restore()
	if _, err := Apply(fixture.repoRoot, fixture.caseRoot, fixture.pack, plan.ExpectedPlanSHA256); err == nil || !strings.Contains(err.Error(), os.ErrPermission.Error()) {
		t.Fatalf("expected injected post-rename failure, got %v", err)
	}
	assertMissing(t, filepath.Join(fixture.caseRoot, ".rekit"))
	assertFile(t, filepath.Join(fixture.caseRoot, ".steamai", "instance.yml"))
	assertMissing(t, filepath.Join(fixture.caseRoot, filepath.FromSlash(ReceiptRel)))
	if info, err := os.Stat(filepath.Join(fixture.caseRoot, ".steamai", "migration")); err != nil || !info.IsDir() {
		t.Fatalf("post-rename failure omitted deterministic migration fence: info=%v err=%v", info, err)
	}
	if _, err := Preview(fixture.repoRoot, fixture.caseRoot, fixture.pack); err == nil {
		t.Fatal("expected partial current root to fail closed")
	}
	if _, err := Apply(fixture.repoRoot, fixture.caseRoot, fixture.pack, plan.ExpectedPlanSHA256); err == nil {
		t.Fatal("expected partial current root Apply to fail closed")
	}
	assertMissing(t, filepath.Join(fixture.caseRoot, ".rekit"))
}

func TestFailureBeforeReceiptPublicationKeepsAllStateOwnersFenced(t *testing.T) {
	fixture := newMigrationFixture(t)
	plan, err := Preview(fixture.repoRoot, fixture.caseRoot, fixture.pack)
	if err != nil {
		t.Fatal(err)
	}
	restore := SetBeforeReceiptPublicationHookForTest(func() error { return os.ErrPermission })
	defer restore()
	if _, err := Apply(fixture.repoRoot, fixture.caseRoot, fixture.pack, plan.ExpectedPlanSHA256); err == nil || !strings.Contains(err.Error(), os.ErrPermission.Error()) {
		t.Fatalf("expected pre-receipt failure, got %v", err)
	}
	assertMissing(t, filepath.Join(fixture.caseRoot, ".rekit"))
	assertMissing(t, filepath.Join(fixture.caseRoot, filepath.FromSlash(ReceiptRel)))
	assertMissing(t, filepath.Join(fixture.caseRoot, ".steamai", "migration", ".state-root-v1.json.state-migration.tmp"))
	assertFile(t, filepath.Join(fixture.caseRoot, ".claude", "skills", "steamai", "SKILL.md"))
	assertMissing(t, filepath.Join(fixture.caseRoot, ".claude", "skills", "rekit", "SKILL.md"))
	if _, err := projectstate.Resolve(fixture.caseRoot); err == nil || !strings.Contains(err.Error(), "state migration is partial") {
		t.Fatalf("projectstate owner accepted pre-receipt partial migration: %v", err)
	}
	if _, err := Preview(fixture.repoRoot, fixture.caseRoot, fixture.pack); err == nil {
		t.Fatal("migration owner accepted pre-receipt partial migration")
	}
}

func TestLateReceiptCollisionIsNeverOverwrittenAndKeepsOwnersFenced(t *testing.T) {
	fixture := newMigrationFixture(t)
	plan, err := Preview(fixture.repoRoot, fixture.caseRoot, fixture.pack)
	if err != nil {
		t.Fatal(err)
	}
	collision := []byte("late collision\n")
	restore := SetBeforeReceiptPublicationHookForTest(func() error {
		return os.WriteFile(filepath.Join(fixture.caseRoot, filepath.FromSlash(ReceiptRel)), collision, 0o600)
	})
	defer restore()
	if _, err := Apply(fixture.repoRoot, fixture.caseRoot, fixture.pack, plan.ExpectedPlanSHA256); err == nil {
		t.Fatal("expected late receipt collision to fail closed")
	}
	actual, err := os.ReadFile(filepath.Join(fixture.caseRoot, filepath.FromSlash(ReceiptRel)))
	if err != nil || !bytes.Equal(actual, collision) {
		t.Fatalf("late receipt collision was overwritten: err=%v actual=%q", err, actual)
	}
	assertFile(t, filepath.Join(fixture.caseRoot, ".steamai", "migration", ".state-root-v1.json.state-migration.tmp"))
	if _, err := projectstate.Resolve(fixture.caseRoot); err == nil || !strings.Contains(err.Error(), "state migration is partial") {
		t.Fatalf("projectstate owner accepted collided migration marker: %v", err)
	}
	if _, err := Preview(fixture.repoRoot, fixture.caseRoot, fixture.pack); err == nil {
		t.Fatal("migration owner accepted collided migration marker")
	}
}

func TestPreviewRejectsDualRootAndPartialCurrentSkill(t *testing.T) {
	fixture := newMigrationFixture(t)
	if err := os.Mkdir(filepath.Join(fixture.caseRoot, ".steamai"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Preview(fixture.repoRoot, fixture.caseRoot, fixture.pack); err == nil || !strings.Contains(err.Error(), "dual roots") {
		t.Fatalf("expected dual root rejection, got %v", err)
	}

	fixture = newMigrationFixture(t)
	writeFixtureFile(t, filepath.Join(fixture.caseRoot, ".claude", "skills", "steamai", "SKILL.md"), []byte("partial"))
	if _, err := Preview(fixture.repoRoot, fixture.caseRoot, fixture.pack); err == nil || !strings.Contains(err.Error(), "partial current project skill") {
		t.Fatalf("expected partial current skill rejection, got %v", err)
	}
}

func TestApplyExactReplayAndReceiptTamperFailClosed(t *testing.T) {
	fixture := newMigrationFixture(t)
	plan, err := Preview(fixture.repoRoot, fixture.caseRoot, fixture.pack)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(fixture.repoRoot, fixture.caseRoot, fixture.pack, plan.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	before := snapshotTree(t, fixture.caseRoot)
	replay, err := Apply(fixture.repoRoot, fixture.caseRoot, fixture.pack, plan.ExpectedPlanSHA256)
	if err != nil {
		t.Fatal(err)
	}
	if replay.Applied || replay.IsMutation || !replay.Replay || !replay.AlreadyCurrent || replay.Status != "already-current" {
		t.Fatalf("unexpected replay: %+v", replay)
	}
	if after := snapshotTree(t, fixture.caseRoot); !equalSnapshot(before, after) {
		t.Fatal("exact replay mutated the project")
	}
	if _, err := Apply(fixture.repoRoot, fixture.caseRoot, fixture.pack, strings.Repeat("f", 64)); err == nil {
		t.Fatal("expected different replay hash to fail")
	}
	receiptPath := filepath.Join(fixture.caseRoot, filepath.FromSlash(ReceiptRel))
	receipt, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	receipt = bytes.Replace(receipt, []byte(`"noAuthority": true`), []byte(`"noAuthority": false`), 1)
	if err := os.WriteFile(receiptPath, receipt, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Preview(fixture.repoRoot, fixture.caseRoot, fixture.pack); err == nil {
		t.Fatal("expected tampered receipt to fail closed")
	}
}

func TestCurrentOnlyWithoutMigrationReceiptIsExplicitNoOp(t *testing.T) {
	fixture := newMigrationFixture(t)
	plan, err := Preview(fixture.repoRoot, fixture.caseRoot, fixture.pack)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(fixture.repoRoot, fixture.caseRoot, fixture.pack, plan.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(fixture.caseRoot, ".steamai", "migration")); err != nil {
		t.Fatal(err)
	}
	before := snapshotTree(t, fixture.caseRoot)
	preview, err := Preview(fixture.repoRoot, fixture.caseRoot, fixture.pack)
	if err != nil {
		t.Fatal(err)
	}
	if !preview.AlreadyCurrent || preview.Replay || preview.Status != "already-current" || preview.ExpectedPlanSHA256 != "" || preview.RequiresReview || preview.RequiresConfirmation || preview.CurrentInstance.Size < 1 || preview.CurrentState.Size < 1 || preview.CurrentSkill.Size < 1 || preview.BundleManifest.Size < 1 {
		t.Fatalf("unexpected current-only preview: %+v", preview)
	}
	if _, err := Apply(fixture.repoRoot, fixture.caseRoot, fixture.pack, strings.Repeat("a", 64)); err == nil || !strings.Contains(err.Error(), "already current") {
		t.Fatalf("current-only project accepted an unissued Apply plan SHA-256: %v", err)
	}
	if after := snapshotTree(t, fixture.caseRoot); !equalSnapshot(before, after) {
		t.Fatal("current-only preview or rejected Apply mutated the project")
	}
}

func TestCurrentReplayRejectsLegacyMetadataTamper(t *testing.T) {
	fixture := newMigrationFixture(t)
	plan, err := Preview(fixture.repoRoot, fixture.caseRoot, fixture.pack)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(fixture.repoRoot, fixture.caseRoot, fixture.pack, plan.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixture.caseRoot, ".re-template.yml"), []byte("tampered: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Preview(fixture.repoRoot, fixture.caseRoot, fixture.pack); err == nil || !strings.Contains(err.Error(), "legacy metadata binding differs") {
		t.Fatalf("expected legacy metadata tamper to fail closed, got %v", err)
	}
}

func TestCurrentMigrationNamespaceWithoutReceiptFailsClosed(t *testing.T) {
	fixture := newMigrationFixture(t)
	plan, err := Preview(fixture.repoRoot, fixture.caseRoot, fixture.pack)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(fixture.repoRoot, fixture.caseRoot, fixture.pack, plan.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	receiptPath := filepath.Join(fixture.caseRoot, filepath.FromSlash(ReceiptRel))
	if err := os.Remove(receiptPath); err != nil {
		t.Fatal(err)
	}
	if _, err := Preview(fixture.repoRoot, fixture.caseRoot, fixture.pack); err == nil || !strings.Contains(err.Error(), "partial state migration namespace") {
		t.Fatalf("expected missing receipt in migration namespace to fail closed, got %v", err)
	}
}

func TestMovedCurrentProjectRemainsAlreadyCurrent(t *testing.T) {
	fixture := newMigrationFixture(t)
	plan, err := Preview(fixture.repoRoot, fixture.caseRoot, fixture.pack)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(fixture.repoRoot, fixture.caseRoot, fixture.pack, plan.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	copyRoot := filepath.Join(t.TempDir(), "copied-project")
	copyTree(t, fixture.caseRoot, copyRoot)
	preview, err := Preview(fixture.repoRoot, copyRoot, fixture.pack)
	if err != nil {
		t.Fatal(err)
	}
	if !preview.AlreadyCurrent || !preview.Replay || preview.Status != "already-current" || preview.ExpectedPlanSHA256 != plan.ExpectedPlanSHA256 {
		t.Fatalf("unexpected copied project preview: %+v", preview)
	}
	metadata, err := os.ReadFile(filepath.Join(copyRoot, ".steamai", "instance.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(metadata, []byte(fixture.caseRoot)) {
		t.Fatalf("copied metadata retained old project path: %s", metadata)
	}
}

func TestPreviewRejectsSymlinkInLegacyTree(t *testing.T) {
	fixture := newMigrationFixture(t)
	link := filepath.Join(fixture.caseRoot, ".rekit", "facts", "link.jsonl")
	if err := os.Symlink(filepath.Join(fixture.caseRoot, ".rekit", "facts", "mission.jsonl"), link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := Preview(fixture.repoRoot, fixture.caseRoot, fixture.pack); err == nil {
		t.Fatal("expected legacy tree symlink to fail closed")
	}
}

func TestPreviewRejectsWindowsJunctionInLegacyTree(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows junction test")
	}
	fixture := newMigrationFixture(t)
	target := t.TempDir()
	junction := filepath.Join(fixture.caseRoot, ".rekit", "junction")
	if output, err := exec.Command("cmd.exe", "/c", "mklink", "/J", junction, target).CombinedOutput(); err != nil {
		t.Skipf("junction unavailable: %v: %s", err, output)
	}
	if _, err := Preview(fixture.repoRoot, fixture.caseRoot, fixture.pack); err == nil {
		t.Fatal("expected legacy tree junction to fail closed")
	}
}

type migrationFixture struct {
	repoRoot string
	caseRoot string
	pack     string
	durable  map[string][]byte
}

func newMigrationFixture(t *testing.T) migrationFixture {
	t.Helper()
	repoRoot := repositoryRoot(t)
	caseRoot := filepath.Join(t.TempDir(), "legacy-project")
	pack := "_template"
	if err := os.MkdirAll(filepath.Join(caseRoot, ".rekit"), 0o700); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(t.TempDir(), "steamai-test-runtime")
	if os.PathSeparator == '\\' {
		executable += ".exe"
	}
	writeFixtureFile(t, executable, []byte("test runtime executable"))
	restore := runtimebundle.SetExecutableSourceForTest(executable)
	t.Cleanup(restore)

	instanceData := []byte(casebind.InstanceText(caseRoot, repoRoot, pack, "legacy-project"))
	stateData, err := json.MarshalIndent(map[string]any{
		"schemaVersion": 1,
		"templateRoot":  repoRoot,
		"templatePack":  pack,
		"lastSyncAt":    "2026-08-14T00:00:00Z",
		"managed":       map[string]any{},
	}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	stateData = append(stateData, '\n')
	writeFixtureFile(t, filepath.Join(caseRoot, ".rekit", "instance.yml"), instanceData)
	writeFixtureFile(t, filepath.Join(caseRoot, ".rekit", "state.json"), stateData)
	legacyMetadata := []byte("templateRoot: " + repoRoot + "\r\ntemplatePack: " + pack + "\r\ncurrentProjectPath: " + caseRoot + "\r\n")
	writeFixtureFile(t, filepath.Join(caseRoot, ".re-template.yml"), legacyMetadata)
	legacySkill, err := os.ReadFile(filepath.Join(repoRoot, "rekit", "templates", "case-shim", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, filepath.Join(caseRoot, ".claude", "skills", "rekit", "SKILL.md"), legacySkill)

	durable := map[string][]byte{
		"board.json":                   []byte("{\"schemaVersion\":1}\n"),
		"facts/mission.jsonl":          []byte("{\"fact\":\"preserve\"}\n"),
		"evidence/ledger.jsonl":        []byte("{\"evidence\":\"preserve\"}\n"),
		"lanes/lead/gate.jsonl":        []byte("{\"decision\":\"authorized-gate\"}\n"),
		"lanes/lead/autonomy.json":     []byte("{\"mode\":\"manual\"}\n"),
		"lanes/lead/prompts/RESUME.md": []byte("resume bytes\r\n"),
	}
	for rel, data := range durable {
		writeFixtureFile(t, filepath.Join(caseRoot, ".rekit", filepath.FromSlash(rel)), data)
	}
	return migrationFixture{repoRoot: repoRoot, caseRoot: caseRoot, pack: pack, durable: durable}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func writeFixtureFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertFile(t *testing.T, path string) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("expected regular file %s: info=%v err=%v", path, info, err)
	}
}

func assertMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("expected missing path %s: %v", path, err)
	}
}

func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			out[filepath.ToSlash(rel)] = "dir"
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = sha256Hex(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func containsExactArg(args []string, name, value string) bool {
	for index := 0; index+1 < len(args); index++ {
		if args[index] == name && args[index+1] == value {
			return true
		}
	}
	return false
}

func containsExactFlag(args []string, flag string) bool {
	return slices.Contains(args, flag)
}

func migrationWrite(writes []Write, path string) (Write, bool) {
	for _, write := range writes {
		if write.Path == path {
			return write, true
		}
	}
	return Write{}, false
}

func equalSnapshot(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}

func copyTree(t *testing.T, source, target string) {
	t.Helper()
	err := filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		dest := filepath.Join(target, rel)
		if entry.IsDir() {
			return os.MkdirAll(dest, 0o700)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dest, data, 0o600)
	})
	if err != nil {
		t.Fatal(err)
	}
}
