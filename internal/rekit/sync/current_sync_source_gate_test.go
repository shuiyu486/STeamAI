package sync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRebuildCurrentSyncPlanForStagingRequiresExactReviewedIdentity(t *testing.T) {
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

	fresh, err := rebuildCurrentSyncPlanForStaging(
		fixture.repoRoot,
		fixture.caseRoot,
		fixture.pack,
		plan.ExpectedPlanSHA256,
		opt,
		intent,
	)
	if err != nil {
		t.Fatal(err)
	}
	if fresh.prepared == nil || fresh.ExpectedPlanSHA256 != plan.ExpectedPlanSHA256 ||
		!currentSyncCanonicalEqual(
			currentSyncIdentity(fresh),
			currentSyncIdentity(plan),
		) {
		t.Fatalf("current sync staging rebuild lost reviewed identity: %+v", fresh)
	}

	tests := []struct {
		name     string
		repoRoot string
		caseRoot string
		pack     string
		expected string
		opt      CurrentSyncOptions
	}{
		{
			name:     "plan-sha",
			repoRoot: fixture.repoRoot,
			caseRoot: fixture.caseRoot,
			pack:     fixture.pack,
			expected: strings.Repeat("f", 64),
			opt:      opt,
		},
		{
			name:     "source-root",
			repoRoot: filepath.Dir(fixture.repoRoot),
			caseRoot: fixture.caseRoot,
			pack:     fixture.pack,
			expected: plan.ExpectedPlanSHA256,
			opt:      opt,
		},
		{
			name:     "case-root",
			repoRoot: fixture.repoRoot,
			caseRoot: filepath.Dir(fixture.caseRoot),
			pack:     fixture.pack,
			expected: plan.ExpectedPlanSHA256,
			opt:      opt,
		},
		{
			name:     "pack",
			repoRoot: fixture.repoRoot,
			caseRoot: fixture.caseRoot,
			pack:     "other-pack",
			expected: plan.ExpectedPlanSHA256,
			opt:      opt,
		},
		{
			name:     "command",
			repoRoot: fixture.repoRoot,
			caseRoot: fixture.caseRoot,
			pack:     fixture.pack,
			expected: plan.ExpectedPlanSHA256,
			opt: CurrentSyncOptions{
				Command:          "update",
				ProjectName:      opt.ProjectName,
				SourceExecutable: opt.SourceExecutable,
			},
		},
		{
			name:     "project-name",
			repoRoot: fixture.repoRoot,
			caseRoot: fixture.caseRoot,
			pack:     fixture.pack,
			expected: plan.ExpectedPlanSHA256,
			opt: CurrentSyncOptions{
				Command:          opt.Command,
				ProjectName:      "other-project",
				SourceExecutable: opt.SourceExecutable,
			},
		},
		{
			name:     "source-executable",
			repoRoot: fixture.repoRoot,
			caseRoot: fixture.caseRoot,
			pack:     fixture.pack,
			expected: plan.ExpectedPlanSHA256,
			opt: CurrentSyncOptions{
				Command:     opt.Command,
				ProjectName: opt.ProjectName,
				SourceExecutable: filepath.Join(
					filepath.Dir(opt.SourceExecutable),
					"other-executable",
				),
			},
		},
		{
			name:     "force",
			repoRoot: fixture.repoRoot,
			caseRoot: fixture.caseRoot,
			pack:     fixture.pack,
			expected: plan.ExpectedPlanSHA256,
			opt: CurrentSyncOptions{
				Command:             opt.Command,
				ProjectName:         opt.ProjectName,
				SourceExecutable:    opt.SourceExecutable,
				ForceLocalTemplates: true,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := rebuildCurrentSyncPlanForStaging(
				test.repoRoot,
				test.caseRoot,
				test.pack,
				test.expected,
				test.opt,
				intent,
			)
			if err == nil {
				t.Fatalf("current sync staging accepted %s drift", test.name)
			}
		})
	}
}

func TestRebuildCurrentSyncPlanForStagingRejectsSourceDrift(t *testing.T) {
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
	path := ""
	for _, write := range plan.Writes {
		if strings.TrimSpace(write.SourcePath) != "" {
			path = write.SourcePath
			break
		}
	}
	if path == "" {
		t.Fatal("current sync fixture has no source-backed write")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, []byte("\nsource drift\n")...), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err = rebuildCurrentSyncPlanForStaging(
		fixture.repoRoot,
		fixture.caseRoot,
		fixture.pack,
		plan.ExpectedPlanSHA256,
		opt,
		intent,
	)
	if err == nil || !strings.Contains(err.Error(), "plan changed after preview") {
		t.Fatalf("current sync staging source drift error = %v", err)
	}
}
