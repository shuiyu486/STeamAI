package projectstate

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
)

func TestResolveUsesCurrentRootForNewProject(t *testing.T) {
	caseRoot := t.TempDir()
	root, err := Resolve(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	if root.Dir != CurrentDir || root.Path != filepath.Join(caseRoot, CurrentDir) || root.Existing || root.Legacy {
		t.Fatalf("root = %#v, want new STeamAI root", root)
	}
}

func TestMissionScopedNameOwnsGenerationProjection(t *testing.T) {
	missionScoped := []string{
		"mission-intent.json", "board.json", "policy.yml", "lanes", "facts", "runs", "reviews",
		"reviewer-adoptions", "handovers", "verifications", "external-session-attempts",
		"external-session-attempt-inputs", "external-session-dispatch", "external-session-jobs",
		"external-session-relays", "external-session-observations", "external-session-transport",
		"reopen-operations",
	}
	view := MissionView{
		Root:       Root{Dir: CurrentDir},
		Generation: 2,
	}
	for _, name := range missionScoped {
		if !MissionScopedName(name) {
			t.Fatalf("mission namespace owner omitted %q", name)
		}
		got := view.ProjectStatePath(filepath.ToSlash(filepath.Join(CurrentDir, name, "item.json")))
		want := filepath.ToSlash(filepath.Join(CurrentDir, MissionsDir, "g000002", name, "item.json"))
		if got != want {
			t.Fatalf("ProjectStatePath(%q) = %q, want %q", name, got, want)
		}
	}
	for _, name := range []string{"instance.yml", "project-binding.json", "runtime", "packs", "onboarding", "transitions"} {
		if MissionScopedName(name) {
			t.Fatalf("project-scoped namespace %q was classified mission-scoped", name)
		}
		got := view.ProjectStatePath(filepath.ToSlash(filepath.Join(CurrentDir, name)))
		want := filepath.ToSlash(filepath.Join(CurrentDir, name))
		if got != want {
			t.Fatalf("project-scoped path %q projected to %q", name, got)
		}
	}
}

func TestMissionViewDefaultsToRootGenerationOne(t *testing.T) {
	caseRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(caseRoot, CurrentDir), 0o755); err != nil {
		t.Fatal(err)
	}
	view, err := ResolveMissionView(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	if view.Generation != 1 || view.Path != filepath.Join(caseRoot, CurrentDir) || view.Active != nil {
		t.Fatalf("default mission view = %+v", view)
	}
	board, err := Join(caseRoot, "board.json")
	if err != nil || board != filepath.Join(caseRoot, CurrentDir, "board.json") {
		t.Fatalf("default board path = %q err=%v", board, err)
	}
}

func TestResolveKeepsLegacyProjectOnSingleRoot(t *testing.T) {
	caseRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(caseRoot, LegacyDir), 0o755); err != nil {
		t.Fatal(err)
	}
	root, err := Resolve(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	if root.Dir != LegacyDir || !root.Existing || !root.Legacy {
		t.Fatalf("root = %#v, want existing legacy root", root)
	}
}

func TestResolveRejectsDualMutableRoots(t *testing.T) {
	caseRoot := t.TempDir()
	for _, dir := range []string{CurrentDir, LegacyDir} {
		if err := os.Mkdir(filepath.Join(caseRoot, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Resolve(caseRoot); err == nil {
		t.Fatal("dual mutable roots must fail closed")
	}
}

func TestPublicEntrypointUsesResolvedStateRoot(t *testing.T) {
	for _, test := range []struct {
		name       string
		stateDir   string
		entrypoint string
	}{
		{name: "new current", entrypoint: "/steamai"},
		{name: "existing current", stateDir: CurrentDir, entrypoint: "/steamai"},
		{name: "legacy", stateDir: LegacyDir, entrypoint: "/rekit"},
	} {
		t.Run(test.name, func(t *testing.T) {
			caseRoot := t.TempDir()
			if test.stateDir != "" {
				if err := os.Mkdir(filepath.Join(caseRoot, test.stateDir), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			entrypoint, err := PublicEntrypoint(caseRoot)
			if err != nil {
				t.Fatal(err)
			}
			if entrypoint != test.entrypoint {
				t.Fatalf("entrypoint = %q, want %q", entrypoint, test.entrypoint)
			}
		})
	}
}

func TestProjectPublicCommandUsesResolvedStateRoot(t *testing.T) {
	const (
		legacyCommand  = `/rekit continue feature-login -Reason "review again" -WhatIf -Format json`
		currentCommand = `/steamai continue feature-login -Reason "review again" -WhatIf -Format json`
	)
	for _, test := range []struct {
		name     string
		stateDir string
		want     string
	}{
		{name: "new current", want: currentCommand},
		{name: "existing current", stateDir: CurrentDir, want: currentCommand},
		{name: "legacy", stateDir: LegacyDir, want: legacyCommand},
	} {
		t.Run(test.name, func(t *testing.T) {
			caseRoot := t.TempDir()
			if test.stateDir != "" {
				if err := os.Mkdir(filepath.Join(caseRoot, test.stateDir), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			for _, input := range []string{legacyCommand, currentCommand} {
				got, err := ProjectPublicCommand(caseRoot, input)
				if err != nil {
					t.Fatalf("ProjectPublicCommand(%q): %v", input, err)
				}
				if got != test.want {
					t.Fatalf("ProjectPublicCommand(%q) = %q, want %q", input, got, test.want)
				}
			}
		})
	}
}

func TestProjectPublicCommandRejectsMalformedInvocation(t *testing.T) {
	caseRoot := t.TempDir()
	for _, command := range []string{
		"",
		"status",
		"/steamai",
		"/steamai unknown -WhatIf",
		`/rekit continue "unterminated`,
		"/steamai continue -Command status",
	} {
		if got, err := ProjectPublicCommand(caseRoot, command); err == nil {
			t.Fatalf("ProjectPublicCommand(%q) = %q without an error", command, got)
		}
	}
}

func TestResolveRejectsPartialCurrentMigrationNamespace(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*testing.T, string)
	}{
		{name: "empty", setup: func(t *testing.T, currentPath string) {
			if err := os.Mkdir(filepath.Join(currentPath, "migration"), 0o755); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "receipt missing with residue", setup: func(t *testing.T, currentPath string) {
			migrationPath := filepath.Join(currentPath, "migration")
			if err := os.Mkdir(migrationPath, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(migrationPath, "partial.tmp"), []byte("partial\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "empty receipt", setup: func(t *testing.T, currentPath string) {
			migrationPath := filepath.Join(currentPath, "migration")
			if err := os.Mkdir(migrationPath, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(migrationPath, "state-root-v1.json"), nil, 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "receipt with staged commit link", setup: func(t *testing.T, currentPath string) {
			migrationPath := filepath.Join(currentPath, "migration")
			if err := os.Mkdir(migrationPath, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(migrationPath, "state-root-v1.json"), []byte("{}\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(migrationPath, ".state-root-v1.json.state-migration.tmp"), []byte("{}\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			caseRoot := t.TempDir()
			currentPath := filepath.Join(caseRoot, CurrentDir)
			if err := os.Mkdir(currentPath, 0o755); err != nil {
				t.Fatal(err)
			}
			test.setup(t, currentPath)
			if _, err := Resolve(caseRoot); err == nil || !strings.Contains(err.Error(), "state migration is partial") {
				t.Fatalf("partial migration namespace passed validation: %v", err)
			}
		})
	}
}

func TestResolveAcceptsCommittedCurrentMigrationReceipt(t *testing.T) {
	caseRoot := t.TempDir()
	migrationPath := filepath.Join(caseRoot, CurrentDir, "migration")
	if err := os.MkdirAll(migrationPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(migrationPath, "state-root-v1.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	root, err := Resolve(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	if root.Dir != CurrentDir || !root.Existing || root.Legacy {
		t.Fatalf("root = %#v, want committed current migration root", root)
	}
}

func TestResolveRejectsStateRootRegularFile(t *testing.T) {
	for _, stateDir := range []string{CurrentDir, LegacyDir} {
		t.Run(stateDir, func(t *testing.T) {
			caseRoot := t.TempDir()
			if err := os.WriteFile(filepath.Join(caseRoot, stateDir), []byte("not a directory\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := Resolve(caseRoot); err == nil {
				t.Fatalf("regular file %s state root passed validation", stateDir)
			}
		})
	}
}

func TestResolveRejectsStateRootSymlink(t *testing.T) {
	for _, stateDir := range []string{CurrentDir, LegacyDir} {
		t.Run(stateDir, func(t *testing.T) {
			caseRoot := t.TempDir()
			target := t.TempDir()
			if err := os.Symlink(target, filepath.Join(caseRoot, stateDir)); err != nil {
				t.Skipf("symlink unavailable: %v", err)
			}
			if _, err := Resolve(caseRoot); err == nil {
				t.Fatalf("symlink %s state root passed validation", stateDir)
			}
		})
	}
}

func TestResolveRejectsLegacyMetadataSymlink(t *testing.T) {
	caseRoot := t.TempDir()
	target := filepath.Join(t.TempDir(), "metadata.yml")
	if err := os.WriteFile(target, []byte("templatePack: "+defaults.DefaultPack+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(caseRoot, ".re-template.yml")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := Resolve(caseRoot); err == nil {
		t.Fatal("symlink legacy metadata passed validation")
	}
}

func TestResolveLegacyMetadataWithoutStateRoot(t *testing.T) {
	caseRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(caseRoot, ".re-template.yml"), []byte("templatePack: "+defaults.DefaultPack+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	root, err := Resolve(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	if root.Dir != LegacyDir || root.Existing || !root.Legacy {
		t.Fatalf("root = %#v, want legacy-compatible root", root)
	}
}

func TestResolveRejectsEmptyRoot(t *testing.T) {
	for _, root := range []string{"", " \t\r\n "} {
		if _, err := Resolve(root); err == nil || !strings.Contains(err.Error(), "empty") {
			t.Fatalf("Resolve(%q) error = %v, want empty-root rejection", root, err)
		}
	}
}

func TestResolveAcceptsNonEmptyRelativeRootAsAbsolute(t *testing.T) {
	parent := t.TempDir()
	caseRoot := filepath.Join(parent, "case")
	if err := os.Mkdir(caseRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(parent); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	root, err := Resolve("case")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(caseRoot, CurrentDir); root.Path != want || !filepath.IsAbs(root.Path) {
		t.Fatalf("relative root resolved to %q, want absolute %q", root.Path, want)
	}
}

func TestResolveRejectsOrdinaryFileCaseRoot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "case-file")
	if err := os.WriteFile(path, []byte("not a directory\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(path); err == nil {
		t.Fatal("ordinary file case root passed validation")
	}
}

func TestResolveRejectsOrdinaryFileAsExistingCaseRootAncestor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file-parent")
	if err := os.WriteFile(path, []byte("not a directory\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(filepath.Join(path, "missing-case")); err == nil {
		t.Fatal("case root beneath ordinary file ancestor passed validation")
	}
}

func TestResolveRejectsSymlinkCaseRootAndAncestor(t *testing.T) {
	t.Run("root", func(t *testing.T) {
		target := t.TempDir()
		link := filepath.Join(t.TempDir(), "case-link")
		if err := os.Symlink(target, link); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		if _, err := Resolve(link); err == nil {
			t.Fatal("symlink case root passed validation")
		}
	})
	t.Run("ancestor", func(t *testing.T) {
		target := t.TempDir()
		link := filepath.Join(t.TempDir(), "linked-parent")
		if err := os.Symlink(target, link); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		if _, err := Resolve(filepath.Join(link, "missing-case")); err == nil {
			t.Fatal("case root beneath symlink ancestor passed validation")
		}
	})
}

func TestRelAndJoinRejectUnsafeComponents(t *testing.T) {
	caseRoot := t.TempDir()
	tests := []struct {
		name string
		part string
	}{
		{name: "empty", part: ""},
		{name: "whitespace", part: " \t "},
		{name: "parent", part: ".."},
		{name: "nested parent", part: "a/../b"},
		{name: "multi-level traversal", part: "a/../../b"},
		{name: "absolute", part: filepath.Join(string(filepath.Separator), "absolute")},
	}
	if runtime.GOOS == "windows" {
		tests = append(tests,
			struct{ name, part string }{name: "drive absolute", part: `C:\absolute`},
			struct{ name, part string }{name: "drive relative", part: `C:relative`},
			struct{ name, part string }{name: "volume only", part: `C:`},
			struct{ name, part string }{name: "UNC", part: `\\server\share\path`},
			struct{ name, part string }{name: "slash UNC", part: `//server/share/path`},
			struct{ name, part string }{name: "ADS", part: `state.json:stream`},
			struct{ name, part string }{name: "reserved device", part: `NUL.txt`},
			struct{ name, part string }{name: "trailing dot", part: `state.`},
			struct{ name, part string }{name: "trailing space", part: `state `},
			struct{ name, part string }{name: "invalid character", part: `state?.json`},
			struct{ name, part string }{name: "NUL byte", part: "state\x00.json"},
		)
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got, err := Rel(caseRoot, test.part); err == nil {
				t.Fatalf("Rel accepted %q as %q", test.part, got)
			}
			if got, err := Join(caseRoot, test.part); err == nil {
				t.Fatalf("Join accepted %q as %q", test.part, got)
			}
		})
	}
}

func TestRelAndJoinRejectUnsafeComponentInLaterPosition(t *testing.T) {
	caseRoot := t.TempDir()
	for _, parts := range [][]string{
		{"lanes", "..", "outside"},
		{"lanes", "nested/../../outside", "state.json"},
	} {
		if got, err := Rel(caseRoot, parts...); err == nil {
			t.Fatalf("Rel accepted %#v as %q", parts, got)
		}
		if got, err := Join(caseRoot, parts...); err == nil {
			t.Fatalf("Join accepted %#v as %q", parts, got)
		}
	}
}

func TestRelAndJoinUseSelectedResolvedRoot(t *testing.T) {
	parent := t.TempDir()
	caseRoot := filepath.Join(parent, "case")
	if err := os.Mkdir(caseRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(parent); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	rel, err := Rel("case", "lanes", "main", "lane.json")
	if err != nil {
		t.Fatal(err)
	}
	wantRel := filepath.ToSlash(filepath.Join(CurrentDir, "lanes", "main", "lane.json"))
	if rel != wantRel {
		t.Fatalf("Rel = %q, want %q", rel, wantRel)
	}
	joined, err := Join("case", "lanes", "main", "lane.json")
	if err != nil {
		t.Fatal(err)
	}
	wantJoined := filepath.Join(caseRoot, CurrentDir, "lanes", "main", "lane.json")
	if joined != wantJoined || !filepath.IsAbs(joined) {
		t.Fatalf("Join = %q, want absolute %q", joined, wantJoined)
	}
}

func TestRelAndJoinKeepLegacyRootForReadsAndWrites(t *testing.T) {
	caseRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(caseRoot, LegacyDir), 0o755); err != nil {
		t.Fatal(err)
	}
	rel, err := Rel(caseRoot, "state.json")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.ToSlash(filepath.Join(LegacyDir, "state.json")); rel != want {
		t.Fatalf("Rel = %q, want %q", rel, want)
	}
	joined, err := Join(caseRoot, "state.json")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(caseRoot, LegacyDir, "state.json"); joined != want {
		t.Fatalf("Join = %q, want %q", joined, want)
	}
}
