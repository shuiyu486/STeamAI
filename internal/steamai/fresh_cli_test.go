package steamai

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/STeamAI/internal/steamai/casebootstrap"
)

func TestFreshRejectsCanonicalSourceAsCaseOrAncestor(t *testing.T) {
	git, source := canonicalFreshFixture(t)
	facts := `{"name":"synthetic-case","goal":"verify source separation","authorization":"temporary fixture files only","prohibited":"network or real artifacts","stop":"scope drift","pack":"fixture-pack","members":[]}`
	for _, test := range []struct {
		name, caseRoot string
	}{
		{name: "same directory", caseRoot: source},
		{name: "child directory", caseRoot: filepath.Join(source, "nested-case")},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := os.MkdirAll(test.caseRoot, 0o755); err != nil {
				t.Fatal(err)
			}
			p := &fakePlatform{supported: true, source: source}
			a := newApp(p, strings.NewReader(facts), io.Discard, io.Discard, "test")
			a.cwd = func() (string, error) { return test.caseRoot, nil }
			a.validateSource = func(string) error { return nil }
			a.lookPath = func(string) (string, error) { return git, nil }
			if err := a.run([]string{"__fresh-preview"}); err == nil || !strings.Contains(err.Error(), "canonical source") {
				t.Fatalf("source/case overlap 返回 %v", err)
			}
		})
	}
}

func TestFreshHiddenCommandsUseStdinFactsAndExactConfirmation(t *testing.T) {
	for _, auxPack := range []string{"", "aux-pack"} {
		t.Run("aux="+auxPack, func(t *testing.T) { testFreshHiddenCommands(t, auxPack) })
	}
}

func testFreshHiddenCommands(t *testing.T, auxPack string) {
	t.Helper()
	_, source := canonicalFreshFixture(t)
	caseRoot := t.TempDir()
	facts := `{"name":"synthetic-case","goal":"verify hidden fresh commands","authorization":"temporary fixture files only","prohibited":"network or real artifacts","stop":"scope drift","pack":"fixture-pack","members":[]}`
	if auxPack != "" {
		facts = strings.Replace(facts, `"members":[]`, `"auxPack":"`+auxPack+`","members":[]`, 1)
	}
	nativeGit := nativeTestGit(t)
	p := &fakePlatform{supported: true, source: source}
	a := newApp(p, strings.NewReader(facts), io.Discard, io.Discard, "test")
	a.cwd = func() (string, error) { return caseRoot, nil }
	a.lookPath = func(name string) (string, error) {
		if name == "git.exe" {
			return nativeGit, nil
		}
		return "", errors.New("not found")
	}
	a.validateSource = func(string) error { return nil }
	var output bytes.Buffer
	a.stdout = &output
	if err := a.run([]string{"__fresh-preview"}); err != nil {
		t.Fatal(err)
	}
	prefix := casebootstrap.ConfirmationPrefix
	index := strings.LastIndex(output.String(), prefix)
	if index < 0 {
		t.Fatal("hidden preview 未输出 exact confirmation")
	}
	confirmation := strings.TrimSpace(output.String()[index:])
	if auxPack != "" {
		if !strings.Contains(output.String(), "- Auxiliary pack：`"+auxPack+"`") {
			t.Fatal("CLI preview 未展示辅助身份")
		}
		a.stdin = strings.NewReader(facts)
		if err := a.run([]string{"__fresh-apply", "--confirmation", "确认"}); !errors.Is(err, casebootstrap.ErrConfirmationRequired) {
			t.Fatalf("非 exact 确认被接受: %v", err)
		}
	}

	a.stdin = strings.NewReader(facts)
	output.Reset()
	if err := a.run([]string{"__fresh-apply", "--confirmation", confirmation}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "STeamAI case 已创建") {
		t.Fatal("hidden apply 未报告成功")
	}
	if state, err := inspectCase(caseRoot); err != nil || state != caseCurrent {
		t.Fatalf("hidden apply 没有建立 current case: state=%v err=%v", state, err)
	}
	identity, err := casebootstrap.InspectCurrent(caseRoot)
	if err != nil || identity.Pack != "fixture-pack" || identity.AuxPack != auxPack {
		t.Fatalf("CLI 主辅身份不匹配: %+v %v", identity, err)
	}
}

func TestFreshHiddenCommandsRejectAmbiguousPackFacts(t *testing.T) {
	git, source := canonicalFreshFixture(t)
	for _, selection := range []string{
		`"pack":"fixture-pack","pack":"fixture-pack"`,
		`"pack":"fixture-pack","auxPack":"aux-pack","auxPack":"aux-pack"`,
		`"pack":"fixture-pack","auxPack":""`,
		`"pack":"fixture-pack","auxPack":"fixture-pack"`,
	} {
		t.Run(selection, func(t *testing.T) {
			root := t.TempDir()
			facts := `{"name":"case","goal":"fixture","authorization":"local fixture","prohibited":"network","stop":"scope drift","members":[],` + selection + `}`
			p := &fakePlatform{supported: true, source: source}
			a := newApp(p, strings.NewReader(facts), io.Discard, io.Discard, "test")
			a.cwd = func() (string, error) { return root, nil }
			a.validateSource = func(string) error { return nil }
			a.lookPath = func(string) (string, error) { return git, nil }
			if err := a.run([]string{"__fresh-preview"}); err == nil {
				t.Fatal("CLI 接受歧义 facts")
			}
			if _, err := os.Lstat(filepath.Join(root, ".steamai-vnext")); !os.IsNotExist(err) {
				t.Fatalf("无效 facts 留下 state: %v", err)
			}
		})
	}
}
