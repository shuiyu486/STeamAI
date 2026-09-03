package steamai

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseManifestIsStrictAndBindsRequestedVersion(t *testing.T) {
	digest := strings.Repeat("a", 64)
	revision := strings.Repeat("b", 40)
	data := []byte(`{"schemaVersion":1,"version":"v1.2.3","revision":"` + revision + `","windowsAmd64Sha256":"` + digest + `"}`)
	manifest, err := parseReleaseManifest(data, "v1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Version != "v1.2.3" || manifest.Revision != revision || manifest.WindowsAMD64 != digest {
		t.Fatalf("manifest = %#v", manifest)
	}
	for _, invalid := range [][]byte{
		[]byte(strings.Replace(string(data), "v1.2.3", "v1.2.4", 1)),
		append(append([]byte(nil), data...), []byte(` {}`)...),
		[]byte(strings.Replace(string(data), `}`, `,"unknown":true}`, 1)),
	} {
		if _, err := parseReleaseManifest(invalid, "v1.2.3"); err == nil {
			t.Fatalf("invalid manifest accepted: %s", invalid)
		}
	}
}

func TestRequireCleanCanonicalSourceRejectsDirtyAndUntracked(t *testing.T) {
	source := makeCanonicalSource(t)
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is required")
	}
	if err := requireCleanCanonicalSource(git, source); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(source, "go.mod")
	if err := os.WriteFile(path, []byte("module changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := requireCleanCanonicalSource(git, source); err == nil {
		t.Fatal("dirty canonical checkout accepted")
	}
	cmd := exec.Command(git, "checkout", "--", "go.mod")
	cmd.Dir = source
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "untracked.txt"), []byte("fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := requireCleanCanonicalSource(git, source); err == nil {
		t.Fatal("untracked canonical file accepted")
	}
}

func TestSafeSourceReplacementRequiresFastForward(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is required")
	}
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	run := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command(git, args...)
		cmd.Dir = dir
		out, runErr := cmd.CombinedOutput()
		if runErr != nil {
			t.Fatalf("git %v: %v: %s", args, runErr, out)
		}
		return strings.TrimSpace(string(out))
	}
	run(root, "init", "--bare", "--quiet", remote)
	source := filepath.Join(root, "source")
	run(root, "clone", "--quiet", "file:///"+filepath.ToSlash(remote), source)
	run(source, "config", "user.name", "fixture")
	run(source, "config", "user.email", "fixture@example.invalid")
	if err := os.WriteFile(filepath.Join(source, "file.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(source, "add", "file.txt")
	run(source, "commit", "--quiet", "-m", "one")
	run(source, "push", "--quiet", "origin", "HEAD")
	first := run(source, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(source, "file.txt"), []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(source, "commit", "--quiet", "-am", "two")
	second := run(source, "rev-parse", "HEAD")
	run(source, "push", "--quiet", "origin", "HEAD")
	run(source, "reset", "--hard", "--quiet", first)
	if err := requireSourceAncestor(git, source, second); err != nil {
		t.Fatalf("fast-forward release rejected: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "local.txt"), []byte("local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(source, "add", "local.txt")
	run(source, "commit", "--quiet", "-m", "local")
	if err := requireSourceAncestor(git, source, second); err == nil {
		t.Fatal("local commit was allowed to be discarded by update")
	}
}

func TestUpdateReportsSourceBackupWithoutDeletingIt(t *testing.T) {
	backup := t.TempDir()
	marker := filepath.Join(backup, "local-data")
	if err := os.WriteFile(marker, []byte("preserve\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	if result := (updateResult{CleanupPath: backup}); result.CleanupPath != "" {
		fmt.Fprintf(&output, "旧 canonical checkout 已保留，请复核后手工删除：%s\n", result.CleanupPath)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("source backup was removed: %v", err)
	}
	if !strings.Contains(output.String(), backup) {
		t.Fatal("source backup path was not reported")
	}
}

func TestUninstallCommandPreservesReportedSource(t *testing.T) {
	current := makeFakeExecutable(t, "steamai.exe")
	p := &fakePlatform{supported: true, uninstallResult: uninstallResult{Source: `C:\STeamAI\source`, CleanupDeferred: true, CleanupHelper: `C:\STeamAI\bin\.steamai-uninstall-helper-42.exe`}}
	var output strings.Builder
	p.activeExe = current
	a := newApp(p, nil, &output, io.Discard, "test")
	a.executable = func() (string, error) { return current, nil }
	if err := a.run([]string{"uninstall"}); err != nil {
		t.Fatal(err)
	}
	if p.uninstallCurrent != current {
		t.Fatalf("uninstall current executable = %q", p.uninstallCurrent)
	}
	text := output.String()
	for _, required := range []string{"canonical checkout 与所有 case 已保留", `C:\STeamAI\source`, "本进程退出后删除", `.steamai-uninstall-helper-42.exe`} {
		if !strings.Contains(text, required) {
			t.Fatalf("uninstall output missing %q: %s", required, text)
		}
	}
	if err := a.run([]string{"uninstall", "extra"}); err == nil {
		t.Fatal("uninstall accepted extra arguments")
	}
}

func TestUninstallRejectsUninstalledCaller(t *testing.T) {
	current := makeFakeExecutable(t, "downloaded-steamai.exe")
	p := &fakePlatform{supported: true, activeExe: makeFakeExecutable(t, "installed-steamai.exe")}
	a := newApp(p, nil, io.Discard, io.Discard, "test")
	a.executable = func() (string, error) { return current, nil }
	if err := a.run([]string{"uninstall"}); err == nil {
		t.Fatal("uninstall accepted a non-installed caller")
	}
	if p.uninstallCurrent != "" {
		t.Fatal("platform uninstall called for a non-installed executable")
	}
}

func TestUninstallCleanupRejectsIncompleteArguments(t *testing.T) {
	p := &fakePlatform{supported: true}
	a := newApp(p, nil, io.Discard, io.Discard, "test")
	if err := a.run([]string{"__uninstall-cleanup"}); err == nil {
		t.Fatal("incomplete uninstall cleanup arguments accepted")
	}
}

func TestUpdateCommandRejectsArguments(t *testing.T) {
	p := &fakePlatform{supported: true}
	a := newApp(p, nil, io.Discard, io.Discard, "test")
	for _, args := range [][]string{{"update", "latest"}, {"update", "v1.2.3"}, {"update", "extra", "value"}} {
		if err := a.run(args); err == nil {
			t.Fatalf("update arguments accepted: %#v", args)
		}
	}
}

func TestLatestReleaseManifestProvidesExactVersion(t *testing.T) {
	digest := strings.Repeat("a", 64)
	revision := strings.Repeat("b", 40)
	data := []byte(`{"schemaVersion":1,"version":"v1.2.3","revision":"` + revision + `","windowsAmd64Sha256":"` + digest + `"}`)
	manifest, err := parseLatestReleaseManifest(data)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Version != "v1.2.3" {
		t.Fatalf("latest manifest version = %q", manifest.Version)
	}
	for _, version := range []string{"latest", "v1", "v01.2.3", "v1.2.3+build"} {
		invalid := []byte(strings.Replace(string(data), "v1.2.3", version, 1))
		if _, err := parseLatestReleaseManifest(invalid); err == nil {
			t.Fatalf("latest manifest accepted non-release version %q", version)
		}
	}
}

func TestCaptureCanonicalUpdateStateRejectsIgnoredFiles(t *testing.T) {
	source := makeCanonicalSource(t)
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is required")
	}
	if err := os.WriteFile(filepath.Join(source, ".gitignore"), []byte("*.local.yml\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(git, "add", ".gitignore")
	cmd.Dir = source
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v: %s", err, output)
	}
	cmd = exec.Command(git, "commit", "--quiet", "-m", "ignore fixture")
	cmd.Dir = source
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, output)
	}
	if err := os.WriteFile(filepath.Join(source, "private.local.yml"), []byte("fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := captureCanonicalUpdateState(git, source); err == nil {
		t.Fatal("ignored local file was accepted before source replacement")
	}
}

func TestSourceAncestorRejectsUnpublishedOtherBranch(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is required")
	}
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command(git, args...)
		cmd.Dir = repo
		out, runErr := cmd.CombinedOutput()
		if runErr != nil {
			t.Fatalf("git %v: %v: %s", args, runErr, out)
		}
		return strings.TrimSpace(string(out))
	}
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	run("init", "--quiet")
	run("config", "user.name", "fixture")
	run("config", "user.email", "fixture@example.invalid")
	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "--quiet", "-m", "base")
	baseBranch := run("branch", "--show-current")
	run("checkout", "--quiet", "-b", "learning")
	if err := os.WriteFile(filepath.Join(repo, "learning.txt"), []byte("learning\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "--quiet", "-m", "local learning")
	run("checkout", "--quiet", baseBranch)
	if err := os.WriteFile(filepath.Join(repo, "release.txt"), []byte("release\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "--quiet", "-m", "release")
	release := run("rev-parse", "HEAD")
	if err := requireSourceAncestor(git, repo, release); err == nil {
		t.Fatal("unpublished commit on another local branch was accepted")
	}
}
