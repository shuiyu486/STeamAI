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
	"github.com/shuiyu486/re-context-kits/internal/rekit/packidentity"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtimebundle"
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
