package sync

import (
	"os"
	"path/filepath"
	"testing"

	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
)

func TestInspectCurrentSyncExpectedFileClassifiesExactAbsentAndDrift(t *testing.T) {
	fixture := newCurrentSyncFixture(t, "")
	plan, intent := stageCurrentSyncExactObjectFixture(t, fixture)
	root, err := rekitfs.OpenAnchoredRoot(fixture.caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	var staged currentSyncRenameSpec
	found := false
	for _, operation := range currentSyncExpectedProgressOperations(intent) {
		if operation.Kind != "leaf-stage-to-live" {
			continue
		}
		staged, err = currentSyncRenameOperationSpec(intent, operation, nil)
		if err != nil {
			t.Fatal(err)
		}
		found = true
		break
	}
	if !found {
		t.Fatal("current sync fixture has no staged file rename")
	}
	observed, err := inspectCurrentSyncExpectedObject(
		root,
		staged.SourcePath,
		staged.Expected,
	)
	if err != nil || observed.State != currentSyncObjectExact ||
		observed.Info == nil || len(observed.Data) == 0 {
		t.Fatalf("current sync staged file state=%+v err=%v", observed, err)
	}

	absentPath := currentSyncStatePath(intent.TransactionPath + "/previous/missing.bin")
	observed, err = inspectCurrentSyncExpectedObject(
		root,
		absentPath,
		staged.Expected,
	)
	if err != nil || observed.State != currentSyncObjectAbsent {
		t.Fatalf("current sync absent file state=%+v err=%v", observed, err)
	}

	path := filepath.Join(fixture.caseRoot, filepath.FromSlash(staged.SourcePath))
	if err := os.WriteFile(path, []byte("drifted staged file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	observed, err = inspectCurrentSyncExpectedObject(
		root,
		staged.SourcePath,
		staged.Expected,
	)
	if err != nil || observed.State != currentSyncObjectDrifted {
		t.Fatalf("current sync drifted file state=%+v err=%v plan=%s", observed, err, plan.ExpectedPlanSHA256)
	}
}

func TestInspectCurrentSyncExpectedTreeRejectsExtraAndMissingFiles(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(t *testing.T, fixture currentSyncFixtureState, spec currentSyncRenameSpec)
	}{
		{
			name: "extra",
			mutate: func(t *testing.T, fixture currentSyncFixtureState, spec currentSyncRenameSpec) {
				extra := filepath.Join(
					fixture.caseRoot,
					filepath.FromSlash(spec.SourcePath),
					"unplanned-current-sync-file.txt",
				)
				if err := os.WriteFile(extra, []byte("unplanned\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "missing",
			mutate: func(t *testing.T, fixture currentSyncFixtureState, spec currentSyncRenameSpec) {
				if spec.Expected.Inventory == nil || len(spec.Expected.Inventory.Entries) == 0 {
					t.Fatal("current sync fixture tree has no files")
				}
				binding := spec.Expected.Inventory.Entries[0]
				rel := binding.Path[len(spec.Expected.BindingBasePath)+1:]
				if err := os.Remove(filepath.Join(
					fixture.caseRoot,
					filepath.FromSlash(spec.SourcePath),
					filepath.FromSlash(rel),
				)); err != nil {
					t.Fatal(err)
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newCurrentSyncFixture(t, "")
			_, intent := stageCurrentSyncExactObjectFixture(t, fixture)
			var spec currentSyncRenameSpec
			found := false
			for _, operation := range currentSyncExpectedProgressOperations(intent) {
				if operation.Kind != "root-stage-to-live" {
					continue
				}
				var err error
				spec, err = currentSyncRenameOperationSpec(intent, operation, nil)
				if err != nil {
					t.Fatal(err)
				}
				found = true
				break
			}
			if !found {
				t.Fatal("current sync fixture has no staged tree rename")
			}
			root, err := rekitfs.OpenAnchoredRoot(fixture.caseRoot)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			observed, err := inspectCurrentSyncExpectedObject(
				root,
				spec.SourcePath,
				spec.Expected,
			)
			if err != nil || observed.State != currentSyncObjectExact || observed.Info == nil {
				t.Fatalf("current sync exact tree state=%+v err=%v", observed, err)
			}
			test.mutate(t, fixture, spec)
			observed, err = inspectCurrentSyncExpectedObject(
				root,
				spec.SourcePath,
				spec.Expected,
			)
			if err != nil || observed.State != currentSyncObjectDrifted {
				t.Fatalf("current sync drifted tree state=%+v err=%v", observed, err)
			}
		})
	}
}

func stageCurrentSyncExactObjectFixture(
	t *testing.T,
	fixture currentSyncFixtureState,
) (CurrentSyncPlan, currentSyncIntent) {
	t.Helper()
	plan, err := CurrentSyncPreview(
		fixture.repoRoot,
		fixture.caseRoot,
		fixture.pack,
		CurrentSyncOptions{
			Command:          "sync",
			ProjectName:      "refreshed-project",
			SourceExecutable: fixture.sourceExecutable,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	intent, err := buildCurrentSyncIntent(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := stageCurrentSyncMaterial(fixture.caseRoot, intent, plan); err != nil {
		t.Fatal(err)
	}
	return plan, intent
}
