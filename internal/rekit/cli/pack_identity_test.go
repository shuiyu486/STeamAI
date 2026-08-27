package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/casebind"
	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/packidentity"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtimebundle"
	"github.com/shuiyu486/re-context-kits/internal/rekit/statemigration"
)

func TestRunPublicPackIdentityPolicyIsTypedAndZeroProgress(t *testing.T) {
	fixtureRoot := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(filepath.Join(fixtureRoot, "packs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixtureRoot, "go.mod"), []byte("module github.com/shuiyu486/re-context-kits\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(fixtureRoot); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldwd); err != nil {
			t.Fatal(err)
		}
	})

	before := snapshotFiles(t, fixtureRoot)
	for _, command := range []string{"status", "doctor"} {
		for _, retired := range packidentity.RetiredIDs() {
			t.Run(command+"/retired/"+retired, func(t *testing.T) {
				var out bytes.Buffer
				err := Run([]string{"-Command", command, "-Pack", retired, "-Format", "json"}, &out)
				if err == nil || !packidentity.IsMigrationRequired(err) || !strings.Contains(err.Error(), packidentity.MigrationRequiredCode) {
					t.Fatalf("%s retired pack error = %v, want typed migration requirement", command, err)
				}
				if out.Len() != 0 {
					t.Fatalf("%s retired pack wrote output before rejection: %q", command, out.String())
				}
				assertSnapshotEqual(t, before, snapshotFiles(t, fixtureRoot))
			})
		}

		t.Run(command+"/unknown", func(t *testing.T) {
			var out bytes.Buffer
			err := Run([]string{"-Command", command, "-Pack", "does-not-exist", "-Format", "json"}, &out)
			if err == nil || packidentity.IsMigrationRequired(err) || !strings.Contains(err.Error(), "missing pack manifest") {
				t.Fatalf("%s unknown pack error = %v, want ordinary missing-pack error", command, err)
			}
			if out.Len() != 0 {
				t.Fatalf("%s unknown pack wrote output before rejection: %q", command, out.String())
			}
			assertSnapshotEqual(t, before, snapshotFiles(t, fixtureRoot))
		})
	}
}

func TestRunMigrateStateIsTheOnlyRetiredPreRuntimeRoute(t *testing.T) {
	for _, retired := range packidentity.RetiredIDs() {
		t.Run(retired, func(t *testing.T) {
			caseRoot := retiredLegacyMigrationCase(t, retired)
			before := snapshotFiles(t, caseRoot)
			var out bytes.Buffer
			err := Run([]string{
				"-Command", "migrate-state",
				"-Target", caseRoot,
				"-Pack", retired,
				"-Format", "json",
			}, &out)
			if err != nil {
				t.Fatal(err)
			}
			var plan statemigration.Plan
			if err := json.Unmarshal(out.Bytes(), &plan); err != nil {
				t.Fatal(err)
			}
			if plan.SourcePack != retired || plan.Pack != packidentity.Canonical || plan.Status != "ready-to-migrate" || !plan.RequiresReview || !plan.RequiresConfirmation {
				t.Fatalf("retired CLI migration plan = %+v", plan)
			}
			assertSnapshotEqual(t, before, snapshotFiles(t, caseRoot))
			if !rekitfs.HandleBoundExactMutationSupported() {
				t.Log("retired CLI Preview is available; successful Apply requires handle-bound exact mutation")
				return
			}

			out.Reset()
			err = Run([]string{
				"-Command", "migrate-state",
				"-Target", caseRoot,
				"-Pack", retired,
				"-ExpectedMigrationPlanSha256", plan.ExpectedPlanSHA256,
				"-Apply",
				"-Format", "json",
			}, &out)
			if err != nil {
				t.Fatal(err)
			}
			var result statemigration.Result
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if !result.Applied || result.SourcePack != retired || result.Pack != packidentity.Canonical {
				t.Fatalf("retired CLI migration result = %+v", result)
			}

			out.Reset()
			err = Run([]string{
				"-Command", "migrate-state",
				"-Target", caseRoot,
				"-Pack", retired,
				"-ExpectedMigrationPlanSha256", plan.ExpectedPlanSHA256,
				"-Apply",
				"-Format", "json",
			}, &out)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result.Applied || !result.Replay || result.SourcePack != retired || result.Pack != packidentity.Canonical {
				t.Fatalf("retired CLI migration replay = %+v", result)
			}
		})
	}
}

func TestRunRetiredMigrationRejectsCrossCommandFlagsBeforeOutputOrWrite(t *testing.T) {
	for _, apply := range []bool{false, true} {
		name := "preview"
		if apply {
			name = "apply"
		}
		t.Run(name, func(t *testing.T) {
			caseRoot := retiredLegacyMigrationCase(t, packidentity.RetiredVMP)
			before := snapshotFiles(t, caseRoot)
			args := []string{
				"-Command", "migrate-state",
				"-Target", caseRoot,
				"-Pack", packidentity.RetiredVMP,
				"-ExpectedContinuePlanSha256", strings.Repeat("a", 64),
				"-Format", "json",
			}
			if apply {
				args = append(args, "-ExpectedMigrationPlanSha256", strings.Repeat("b", 64), "-Apply")
			}
			var out bytes.Buffer
			err := Run(args, &out)
			if err == nil || !strings.Contains(err.Error(), "supported only by continue") || out.Len() != 0 {
				t.Fatalf("retired %s ignored a cross-command flag: err=%v out=%q", name, err, out.String())
			}
			assertSnapshotEqual(t, before, snapshotFiles(t, caseRoot))
		})
	}
}

func TestRunRetiredMigrationFallbackRejectsBrokenLegacySourceWithoutOutput(t *testing.T) {
	caseRoot := retiredLegacyMigrationCase(t, packidentity.RetiredVMP)
	metadataPath := filepath.Join(caseRoot, ".re-template.yml")
	metadata, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	metadata = bytes.Replace(metadata, []byte("rekitMode: case-local-shim"), []byte("rekitMode: corrupt"), 1)
	if err := os.WriteFile(metadataPath, metadata, 0o644); err != nil {
		t.Fatal(err)
	}
	before := snapshotFiles(t, caseRoot)
	var out bytes.Buffer
	err = Run([]string{"-Command", "migrate-state", "-Target", caseRoot, "-Pack", packidentity.RetiredVMP, "-Format", "json"}, &out)
	if err == nil || !strings.Contains(err.Error(), "metadata") || out.Len() != 0 {
		t.Fatalf("broken retired source escaped fallback boundary: err=%v out=%q", err, out.String())
	}
	assertSnapshotEqual(t, before, snapshotFiles(t, caseRoot))
}

func TestRunRetiredMigrationFallbackRejectsCurrentWithoutReceipt(t *testing.T) {
	if !rekitfs.HandleBoundExactMutationSupported() {
		t.Skip("retired CLI replay setup requires handle-bound exact mutation")
	}
	caseRoot := retiredLegacyMigrationCase(t, packidentity.RetiredGeneric)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "migrate-state", "-Target", caseRoot, "-Pack", packidentity.RetiredGeneric, "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var plan statemigration.Plan
	if err := json.Unmarshal(out.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "migrate-state", "-Target", caseRoot, "-Pack", packidentity.RetiredGeneric, "-ExpectedMigrationPlanSha256", plan.ExpectedPlanSHA256, "-Apply", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(caseRoot, ".steamai", "migration")); err != nil {
		t.Fatal(err)
	}
	before := snapshotFiles(t, caseRoot)
	out.Reset()
	err := Run([]string{"-Command", "migrate-state", "-Target", caseRoot, "-Pack", packidentity.RetiredGeneric, "-Format", "json"}, &out)
	if err == nil || out.Len() != 0 {
		t.Fatalf("current project without receipt escaped retired fallback: err=%v out=%q", err, out.String())
	}
	assertSnapshotEqual(t, before, snapshotFiles(t, caseRoot))
}

func TestRunOrdinaryCommandsStillRejectRetiredAttachedPack(t *testing.T) {
	for _, retired := range packidentity.RetiredIDs() {
		t.Run(retired, func(t *testing.T) {
			caseRoot := retiredLegacyMigrationCase(t, retired)
			before := snapshotFiles(t, caseRoot)
			for _, command := range []string{"status", "doctor"} {
				var out bytes.Buffer
				err := Run([]string{"-Command", command, "-Target", caseRoot, "-Pack", retired, "-Format", "json"}, &out)
				if err == nil || !packidentity.IsMigrationRequired(err) || out.Len() != 0 {
					t.Fatalf("%s escaped retired admission boundary: err=%v out=%q", command, err, out.String())
				}
				assertSnapshotEqual(t, before, snapshotFiles(t, caseRoot))
			}
		})
	}
}

func TestRunRejectsRetiredAttachedPackBeforeOutputOrWrite(t *testing.T) {
	for _, retired := range packidentity.RetiredIDs() {
		t.Run(retired, func(t *testing.T) {
			caseRoot := filepath.Join(t.TempDir(), "case")
			stateRoot := filepath.Join(caseRoot, ".rekit")
			if err := os.MkdirAll(stateRoot, 0o755); err != nil {
				t.Fatal(err)
			}
			metadata := "templateRoot: " + repoRoot(t) + "\n" +
				"templatePack: " + retired + "\n" +
				"projectName: retired-pack-case\n" +
				"projectRoot: " + caseRoot + "\n"
			if err := os.WriteFile(filepath.Join(stateRoot, "instance.yml"), []byte(metadata), 0o644); err != nil {
				t.Fatal(err)
			}

			before := snapshotFiles(t, caseRoot)
			for _, command := range []string{"status", "doctor"} {
				var out bytes.Buffer
				err := Run([]string{"-Command", command, "-Target", caseRoot, "-Format", "json"}, &out)
				if err == nil || !packidentity.IsMigrationRequired(err) {
					t.Fatalf("%s attached retired pack error = %v, want typed migration requirement", command, err)
				}
				if out.Len() != 0 {
					t.Fatalf("%s attached retired pack wrote output before rejection: %q", command, out.String())
				}
				assertSnapshotEqual(t, before, snapshotFiles(t, caseRoot))
			}
		})
	}
}

func retiredLegacyMigrationCase(t *testing.T, pack string) string {
	t.Helper()
	root := repoRoot(t)
	caseRoot := filepath.Join(t.TempDir(), "retired-legacy-case")
	stateRoot := filepath.Join(caseRoot, ".rekit")
	if err := os.MkdirAll(stateRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	instanceData := []byte(casebind.InstanceText(caseRoot, root, pack, "retired-legacy-case"))
	if err := os.WriteFile(filepath.Join(stateRoot, "instance.yml"), instanceData, 0o644); err != nil {
		t.Fatal(err)
	}
	stateData, err := json.MarshalIndent(map[string]any{
		"schemaVersion": 1,
		"templateRoot":  root,
		"templatePack":  pack,
		"lastSyncAt":    "2026-08-14T00:00:00Z",
		"managed":       map[string]any{},
	}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateRoot, "state.json"), append(stateData, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	legacyMetadata := "templateRoot: " + root + "\n" +
		"rekitMode: case-local-shim\n" +
		"templatePack: " + pack + "\n" +
		"currentProjectPath: " + caseRoot + "\n"
	if err := os.WriteFile(filepath.Join(caseRoot, ".re-template.yml"), []byte(legacyMetadata), 0o644); err != nil {
		t.Fatal(err)
	}
	legacySkill, err := os.ReadFile(filepath.Join(root, "rekit", "templates", "case-shim", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	legacySkillPath := filepath.Join(caseRoot, ".claude", "skills", "rekit", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(legacySkillPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacySkillPath, legacySkill, 0o644); err != nil {
		t.Fatal(err)
	}
	return caseRoot
}

func TestRunProjectLocalRejectsRetiredEmbeddedBundleBeforeOutputOrWrite(t *testing.T) {
	for _, retired := range packidentity.RetiredIDs() {
		t.Run(retired, func(t *testing.T) {
			caseRoot := filepath.Join(t.TempDir(), "case")
			executable := filepath.Join(t.TempDir(), runtimebundle.ExecutableName())
			if err := os.WriteFile(executable, []byte("fixture runtime executable\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			plan, err := runtimebundle.PublishForTest(caseRoot, repoRoot(t), "_template", executable)
			if err != nil {
				t.Fatal(err)
			}
			manifest := plan.Manifest
			manifest.Pack = retired
			manifestData, err := json.MarshalIndent(manifest, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			manifestData = append(manifestData, '\n')
			manifestPath := filepath.Join(caseRoot, ".steamai", filepath.FromSlash(runtimebundle.ManifestRel))
			if err := os.WriteFile(manifestPath, manifestData, 0o644); err != nil {
				t.Fatal(err)
			}
			manifestSHA256 := fmt.Sprintf("%x", sha256.Sum256(manifestData))
			metadata := casebind.STeamAIInstanceText(caseRoot, "_template", "retired-bundle-case", runtimebundle.ManifestRel, manifestSHA256)
			if err := os.WriteFile(filepath.Join(caseRoot, ".steamai", "instance.yml"), []byte(metadata), 0o644); err != nil {
				t.Fatal(err)
			}

			before := snapshotFiles(t, caseRoot)
			var out bytes.Buffer
			err = RunProjectLocal([]string{"-Command", "status", "-Format", "json"}, &out, caseRoot)
			if err == nil || !packidentity.IsMigrationRequired(err) {
				t.Fatalf("project-local retired bundle error = %v, want typed migration requirement", err)
			}
			if out.Len() != 0 {
				t.Fatalf("project-local retired bundle wrote output before rejection: %q", out.String())
			}
			assertSnapshotEqual(t, before, snapshotFiles(t, caseRoot))
		})
	}
}
