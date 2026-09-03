package steamai

import (
	"errors"
	"io"
	"os/exec"
	"strings"
	"testing"
)

func TestLearningBatchHiddenCommandsRequireCurrentCaseAndStrictRequest(t *testing.T) {
	caseRoot := t.TempDir()
	materializeCurrentCaseFixture(t, caseRoot)
	p := &fakePlatform{supported: true, source: caseRoot}
	a := newApp(p, strings.NewReader(`{"unknown":true}`), io.Discard, io.Discard, "test")
	a.cwd = func() (string, error) { return caseRoot, nil }
	a.validateSource = func(string) error { return nil }
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is required")
	}
	a.lookPath = func(name string) (string, error) {
		if name == "git.exe" {
			return git, nil
		}
		return "", errors.New("not found")
	}
	if err := a.run([]string{"__learning-batch-preview"}); err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("hidden preview 未拒绝 unknown request field: %v", err)
	}
	if err := a.run([]string{"__learning-batch-apply"}); err == nil || !strings.Contains(err.Error(), "参数无效") {
		t.Fatalf("hidden apply 未拒绝缺失 confirmation: %v", err)
	}
}
