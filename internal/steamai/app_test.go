package steamai

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type fakePlatform struct {
	supported        bool
	source           string
	activeExe        string
	installedExe     string
	installedSource  string
	installedVersion string
	update           updateInstall
	updateResult     updateResult
	updateErr        error
	uninstallResult  uninstallResult
	uninstallErr     error
	uninstallCurrent string
	acquireErr       error
	attached         []processSpec
	visible          []processSpec
}

func (f *fakePlatform) Supported() bool { return f.supported }
func (f *fakePlatform) CanonicalSource() (string, error) {
	if f.source == "" {
		return "", errors.New("missing source")
	}
	return f.source, nil
}
func (f *fakePlatform) ActiveExecutable() (string, error) {
	if f.activeExe == "" {
		return "", errors.New("missing active executable")
	}
	return f.activeExe, nil
}
func (f *fakePlatform) Install(exe, source, version string) error {
	f.installedExe, f.installedSource, f.installedVersion = exe, source, version
	return nil
}
func (f *fakePlatform) ActivateUpdate(update updateInstall) (updateResult, error) {
	f.update = update
	return f.updateResult, f.updateErr
}
func (f *fakePlatform) Uninstall(current string) (uninstallResult, error) {
	f.uninstallCurrent = current
	return f.uninstallResult, f.uninstallErr
}
func (f *fakePlatform) CaseIdentity(path string) (string, error) {
	return "fixture:" + path, nil
}
func (f *fakePlatform) AcquireCommander(string) (commanderLease, error) {
	if f.acquireErr != nil {
		return commanderLease{}, f.acquireErr
	}
	return commanderLease{handle: 42, release: func() {}}, nil
}
func (f *fakePlatform) RunAttached(spec processSpec, _ io.Reader, _, _ io.Writer) error {
	f.attached = append(f.attached, spec)
	return nil
}
func (f *fakePlatform) OpenVisible(spec processSpec) error {
	f.visible = append(f.visible, spec)
	return nil
}

func TestCommanderLaunchUsesFreshAndCurrentSkillSources(t *testing.T) {
	root := t.TempDir()
	source := makeCanonicalSource(t)
	claude := makeFakeExecutable(t, "claude.exe")

	for _, test := range []struct {
		name     string
		prepare  func(string)
		wantArgs []string
	}{
		{name: "fresh", wantArgs: []string{"/steamai", "--add-dir", source}},
		{name: "current", prepare: func(root string) { materializeCurrentCaseFixture(t, root) }, wantArgs: []string{"/steamai"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			caseRoot := filepath.Join(root, test.name)
			if err := os.MkdirAll(caseRoot, 0o755); err != nil {
				t.Fatal(err)
			}
			if test.prepare != nil {
				test.prepare(caseRoot)
			}
			p := &fakePlatform{supported: true, source: source}
			a := testApp(t, p, caseRoot, claude)
			if err := a.run(nil); err != nil {
				t.Fatal(err)
			}
			if len(p.attached) != 1 {
				t.Fatalf("attached launches = %d", len(p.attached))
			}
			got := p.attached[0]
			if !reflect.DeepEqual(got.Args, test.wantArgs) {
				t.Fatalf("args = %#v want %#v", got.Args, test.wantArgs)
			}
			if got.Dir != caseRoot {
				t.Fatalf("cwd = %s want %s", got.Dir, caseRoot)
			}
			if !reflect.DeepEqual(got.InheritedHandles, []uintptr{42}) {
				t.Fatalf("Commander did not inherit its mutex handle: %#v", got.InheritedHandles)
			}
			for _, item := range got.Env {
				if strings.HasPrefix(strings.ToUpper(item), "CLAUDECODE=") {
					t.Fatal("nested Claude marker leaked")
				}
			}
		})
	}
}

func TestCurrentCaseRejectsMalformedSnapshotMarker(t *testing.T) {
	caseRoot := t.TempDir()
	materializeCurrentCaseFixture(t, caseRoot)
	if err := os.WriteFile(filepath.Join(caseRoot, ".steamai-vnext", "pack-snapshot", "snapshot.yml"), []byte("fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	state, err := inspectCase(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	if state != casePartial {
		t.Fatalf("malformed snapshot state = %v", state)
	}
}

func TestOrphanSkillIsFreshAndCollisionIsDeferredToExactPreview(t *testing.T) {
	caseRoot := t.TempDir()
	path := filepath.Join(caseRoot, ".claude", "skills", "steamai", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("orphan skill\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	state, err := inspectCase(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	if state != caseFresh {
		t.Fatalf("orphan skill state = %v", state)
	}
}

func TestPartialCaseAndDuplicateCommanderFailClosed(t *testing.T) {
	caseRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(caseRoot, ".steamai-vnext"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := &fakePlatform{supported: true, source: makeCanonicalSource(t)}
	a := testApp(t, p, caseRoot, makeFakeExecutable(t, "claude.exe"))
	if err := a.run(nil); !errors.Is(err, errPartialCase) {
		t.Fatalf("partial error = %v", err)
	}
	if len(p.attached) != 0 {
		t.Fatal("partial case launched Claude")
	}

	fresh := t.TempDir()
	p = &fakePlatform{supported: true, source: makeCanonicalSource(t), acquireErr: errCommanderRunning}
	a = testApp(t, p, fresh, makeFakeExecutable(t, "claude.exe"))
	if err := a.run(nil); !errors.Is(err, errCommanderRunning) {
		t.Fatalf("duplicate error = %v", err)
	}
	if len(p.attached) != 0 {
		t.Fatal("duplicate Commander launched Claude")
	}
}

func TestOpenMemberIsVisibleAndBoundToMemberDirectory(t *testing.T) {
	caseRoot := t.TempDir()
	materializeCurrentCaseFixture(t, caseRoot)
	member := filepath.Join(caseRoot, ".steamai-vnext", "members", "static-analysis")
	if err := os.MkdirAll(member, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(member, "CLAUDE.md"), []byte("# Member\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	markerPath := filepath.Join(caseRoot, ".steamai-vnext", "CLAUDE.md")
	marker, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatal(err)
	}
	marker = bytes.Replace(marker, []byte("| none | execution | inactive | none |"), []byte("| static-analysis | execution | active | `.steamai-vnext/members/static-analysis/CLAUDE.md` |"), 1)
	if err := os.WriteFile(markerPath, marker, 0o644); err != nil {
		t.Fatal(err)
	}
	claude := makeFakeExecutable(t, "claude.exe")
	p := &fakePlatform{supported: true}
	a := testApp(t, p, caseRoot, claude)
	if err := a.run([]string{"__open-member", "static-analysis"}); err != nil {
		t.Fatal(err)
	}
	if len(p.visible) != 1 {
		t.Fatalf("visible launches = %d", len(p.visible))
	}
	got := p.visible[0]
	if got.Dir != member {
		t.Fatalf("member cwd = %s", got.Dir)
	}
	if !reflect.DeepEqual(got.Args, []string{memberInitialPrompt, "--add-dir", caseRoot}) {
		t.Fatalf("member args = %#v", got.Args)
	}

	for _, invalid := range []string{"../escape", "UPPER", "con", "a/b"} {
		if err := a.run([]string{"__open-member", invalid}); err == nil {
			t.Fatalf("invalid member %q accepted", invalid)
		}
	}
}

func TestCommandSurfaceRejectsControlPlaneCommands(t *testing.T) {
	p := &fakePlatform{supported: true}
	a := testApp(t, p, t.TempDir(), makeFakeExecutable(t, "claude.exe"))
	for _, command := range []string{"status", "task", "message", "session", "resume", "repair", "migrate", "sync", "contribute"} {
		if err := a.run([]string{command}); err == nil {
			t.Fatalf("control-plane command %q accepted", command)
		}
	}
	var output bytes.Buffer
	a.stdout = &output
	if err := a.run([]string{"--help"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "status") {
		t.Fatal("help exposed control-plane command")
	}
}

func TestWithoutEnvironmentIsCaseInsensitive(t *testing.T) {
	got := withoutEnvironment([]string{"Path=x", "ClaudeCode=nested", "KEEP=yes"}, "CLAUDECODE")
	if !reflect.DeepEqual(got, []string{"Path=x", "KEEP=yes"}) {
		t.Fatalf("environment = %#v", got)
	}
}

func testApp(t *testing.T, p platform, cwd, claude string) *app {
	t.Helper()
	a := newApp(p, strings.NewReader(""), io.Discard, io.Discard, "test")
	a.cwd = func() (string, error) { return cwd, nil }
	a.lookPath = func(name string) (string, error) {
		if name == "claude.exe" {
			return claude, nil
		}
		return "", errors.New("not found")
	}
	a.executable = func() (string, error) { return makeFakeExecutable(t, "steamai.exe"), nil }
	a.validateSource = validateCanonicalSource
	return a
}

func testGit(t *testing.T) string {
	t.Helper()
	for _, name := range []string{"git.exe", "git"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	t.Skip("git is required")
	return ""
}

func nativeTestGit(t *testing.T) string {
	t.Helper()
	git := testGit(t)
	if strings.EqualFold(filepath.Ext(git), ".exe") {
		return git
	}
	data, err := os.ReadFile(git)
	if err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "git.exe")
	if err := os.WriteFile(alias, data, 0o755); err != nil {
		t.Fatal(err)
	}
	return alias
}

func makeCanonicalSource(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".claude", "skills", "steamai"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module "+canonicalModule+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".claude", "skills", "steamai", "SKILL.md"), []byte("# Skill\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git := testGit(t)
	for _, args := range [][]string{
		{"init", "--quiet"},
		{"config", "user.name", "STeamAI fixture"},
		{"config", "user.email", "fixture@example.invalid"},
		{"add", "--", "go.mod", ".claude/skills/steamai/SKILL.md"},
		{"commit", "--quiet", "-m", "fixture"},
	} {
		cmd := exec.Command(git, args...)
		cmd.Dir = root
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, strings.TrimSpace(string(output)))
		}
	}
	return root
}

func makeFakeExecutable(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("fixture executable"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}
