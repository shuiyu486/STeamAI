package sync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStageCurrentSyncMaterialPublishesExactReplayableBundle(t *testing.T) {
	fixture := newCurrentSyncFixture(t, "")
	opt := CurrentSyncOptions{
		Command:          "sync",
		ProjectName:      "refreshed-project",
		SourceExecutable: fixture.sourceExecutable,
	}
	plan, err := CurrentSyncPreview(
		fixture.repoRoot,
		fixture.caseRoot,
		fixture.pack,
		opt,
	)
	if err != nil {
		t.Fatal(err)
	}
	intent, err := buildCurrentSyncIntent(plan)
	if err != nil {
		t.Fatal(err)
	}
	liveBefore, err := os.ReadFile(fixture.targetExecutable)
	if err != nil {
		t.Fatal(err)
	}

	if err := stageCurrentSyncMaterial(fixture.caseRoot, intent, plan); err != nil {
		t.Fatal(err)
	}
	if err := stageCurrentSyncMaterial(fixture.caseRoot, intent, plan); err != nil {
		t.Fatalf("current sync exact staging replay failed: %v", err)
	}
	if err := validateCurrentSyncStagedState(fixture.caseRoot, intent); err != nil {
		t.Fatal(err)
	}
	liveAfter, err := os.ReadFile(fixture.targetExecutable)
	if err != nil {
		t.Fatal(err)
	}
	if string(liveAfter) != string(liveBefore) {
		t.Fatal("current sync staging changed the live runtime executable")
	}
}

func TestCurrentSyncStagedStateSurvivesMissingSource(t *testing.T) {
	fixture := newCurrentSyncFixture(t, "")
	opt := CurrentSyncOptions{
		Command:          "sync",
		ProjectName:      "refreshed-project",
		SourceExecutable: fixture.sourceExecutable,
	}
	plan, err := CurrentSyncPreview(
		fixture.repoRoot,
		fixture.caseRoot,
		fixture.pack,
		opt,
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
	if err := os.RemoveAll(fixture.repoRoot); err != nil {
		t.Fatal(err)
	}
	if err := validateCurrentSyncStagedState(fixture.caseRoot, intent); err != nil {
		t.Fatalf("frozen current sync staging depended on missing source: %v", err)
	}
}

func TestStageCurrentSyncMaterialRejectsExistingDrift(t *testing.T) {
	fixture := newCurrentSyncFixture(t, "")
	opt := CurrentSyncOptions{
		Command:          "sync",
		ProjectName:      "refreshed-project",
		SourceExecutable: fixture.sourceExecutable,
	}
	plan, err := CurrentSyncPreview(
		fixture.repoRoot,
		fixture.caseRoot,
		fixture.pack,
		opt,
	)
	if err != nil {
		t.Fatal(err)
	}
	intent, err := buildCurrentSyncIntent(plan)
	if err != nil {
		t.Fatal(err)
	}
	files, err := currentSyncStageMaterial(intent, plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("current sync fixture produced no staged files")
	}
	driftPath := filepath.Join(
		fixture.caseRoot,
		".steamai",
		filepath.FromSlash(files[0].Path),
	)
	if err := os.MkdirAll(filepath.Dir(driftPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(driftPath, []byte("drifted\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := stageCurrentSyncMaterial(fixture.caseRoot, intent, plan); err == nil ||
		!strings.Contains(err.Error(), "differs") {
		t.Fatalf("current sync accepted drifted staged file: %v", err)
	}
}

func TestStageCurrentSyncMaterialRejectsUnplannedObjects(t *testing.T) {
	fixture := newCurrentSyncFixture(t, "")
	opt := CurrentSyncOptions{
		Command:          "sync",
		ProjectName:      "refreshed-project",
		SourceExecutable: fixture.sourceExecutable,
	}
	plan, err := CurrentSyncPreview(
		fixture.repoRoot,
		fixture.caseRoot,
		fixture.pack,
		opt,
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

	t.Run("controlled", func(t *testing.T) {
		extra := filepath.Join(
			fixture.caseRoot,
			".steamai",
			filepath.FromSlash(intent.TransactionPath),
			"stage",
			"controlled",
			"common",
			"unplanned.txt",
		)
		if err := os.WriteFile(extra, []byte("unplanned\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := stageCurrentSyncMaterial(fixture.caseRoot, intent, plan); err == nil ||
			!strings.Contains(err.Error(), "unplanned files") {
			t.Fatalf("current sync accepted unplanned controlled file: %v", err)
		}
		if err := os.Remove(extra); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("leaf", func(t *testing.T) {
		extra := filepath.Join(
			fixture.caseRoot,
			".steamai",
			filepath.FromSlash(intent.TransactionPath),
			"stage",
			"leaves",
			"999999.bin",
		)
		if err := os.MkdirAll(filepath.Dir(extra), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(extra, []byte("unplanned\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := stageCurrentSyncMaterial(fixture.caseRoot, intent, plan); err == nil ||
			!strings.Contains(err.Error(), "do not equal") {
			t.Fatalf("current sync accepted unplanned staged leaf: %v", err)
		}
	})
}
