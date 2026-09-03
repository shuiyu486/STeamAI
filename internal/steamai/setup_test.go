package steamai

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestSetupBindsExistingCanonicalSource(t *testing.T) {
	t.Setenv("LOCALAPPDATA", "")
	source := makeCanonicalSource(t)
	launcher := makeFakeExecutable(t, "steamai.exe")
	p := &fakePlatform{supported: true}
	a := newApp(p, nil, io.Discard, io.Discard, "test")
	a.executable = func() (string, error) { return launcher, nil }
	a.validateSource = validateCanonicalSource
	if err := a.run([]string{"setup", "--source", source}); err != nil {
		t.Fatal(err)
	}
	if p.installedExe != launcher || p.installedSource != source || p.installedVersion != "test" {
		t.Fatalf("install got exe=%q source=%q version=%q", p.installedExe, p.installedSource, p.installedVersion)
	}
}

func TestSetupRejectsInvalidOrAmbiguousSource(t *testing.T) {
	launcher := makeFakeExecutable(t, "steamai.exe")
	p := &fakePlatform{supported: true}
	a := newApp(p, nil, io.Discard, io.Discard, "test")
	a.executable = func() (string, error) { return launcher, nil }
	a.validateSource = validateCanonicalSource
	if err := a.run([]string{"setup", "--source", t.TempDir()}); err == nil {
		t.Fatal("invalid source accepted")
	}
	if err := a.run([]string{"setup", "--source", makeCanonicalSource(t), "--clone-url", defaultCloneURL}); err == nil {
		t.Fatal("source and clone URL accepted together")
	}
}

func TestCloneSourceUsesNativeGitAndStaging(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "source")
	git := makeFakeExecutable(t, "git.exe")
	a := newApp(&fakePlatform{supported: true}, nil, io.Discard, io.Discard, "test")
	a.lookPath = func(name string) (string, error) {
		if name == "git.exe" {
			return git, nil
		}
		return "", errors.New("not found")
	}
	// 真实 clone 由 Git product-path gate 覆盖；本测试锁住非 HTTPS 与随机 sibling staging 边界。
	if err := a.cloneSource("http://example.invalid/repo", target); err == nil {
		t.Fatal("non-HTTPS clone URL accepted")
	}
	if err := os.MkdirAll(target+".staging", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := a.cloneSource(defaultCloneURL, target); err == nil {
		t.Fatal("fake Git clone unexpectedly succeeded")
	}
}

func TestSetupPathDecisionPreservesOwnershipAcrossRepeatedSetup(t *testing.T) {
	binDir := `C:\Users\Case\AppData\Local\STeamAI\bin`
	owned, add := setupPathDecision(`C:\Tools`, binDir, "", false)
	if !owned || !add {
		t.Fatalf("first setup decision = owned:%t add:%t", owned, add)
	}
	owned, add = setupPathDecision(`C:\Tools;`+binDir, binDir, "true", true)
	if !owned || add {
		t.Fatalf("repeated setup lost ownership: owned:%t add:%t", owned, add)
	}
	owned, add = setupPathDecision(`C:\Tools;`+binDir, binDir, "false", true)
	if owned || add {
		t.Fatalf("pre-existing PATH was claimed: owned:%t add:%t", owned, add)
	}
}

func TestPathListContainsIsCaseInsensitive(t *testing.T) {
	list := `C:\Tools;"C:\Users\Case\AppData\Local\STeamAI\bin";C:\Other`
	candidate := `c:\users\case\appdata\local\steamai\bin\`
	if !pathListContains(list, candidate) {
		t.Fatal("PATH did not match equivalent entry")
	}
	if pathListContains(`C:\Tools`, `C:\Other`) {
		t.Fatal("PATH matched unrelated entry")
	}
	got, removed := removePathListEntry(list, candidate)
	if !removed || got != `C:\Tools;C:\Other` {
		t.Fatalf("remove PATH entry = %q, %t", got, removed)
	}
}
