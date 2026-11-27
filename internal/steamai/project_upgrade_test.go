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

func TestCheckProjectNeedsNoInstallationAndNeverLaunches(t *testing.T) {
	p := &fakePlatform{supported: true, acquireErr: errors.New("检查不应取锁")}
	root := t.TempDir()
	a := testApp(t, p, root, "")
	a.lookPath = func(name string) (string, error) { t.Fatalf("检查不应查找 %s", name); return "", nil }
	a.validateSource = func(string) error { t.Fatal("不应加载安装来源"); return nil }
	var out bytes.Buffer
	a.stdout = &out
	for _, args := range [][]string{{"check", "--apply"}, {"check", "other"}, {"upgrade", "."}, {"upgrade", "--apply", "bad"}, {"check"}} {
		if err := a.run(args); err == nil {
			t.Fatalf("应拒绝 %v", args)
		}
	}
	if files, err := os.ReadDir(root); err != nil || len(files) != 0 {
		t.Fatal("检查空目录不应初始化")
	}
	materializeCurrentCaseFixtureWithAuxPack(t, root, "aux-pack", "builder")
	if err := a.run([]string{"check"}); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"项目框架完整", "fixture-pack", "aux-pack", "sha256:"} {
		if !strings.Contains(out.String(), s) {
			t.Fatalf("缺少 %s", s)
		}
	}
	if strings.Contains(out.String(), "builder") {
		t.Fatal("检查不应显示成员或任务")
	}
	if err := os.Mkdir(filepath.Join(root, ".steamai-vnext/.upgrade-pending"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := a.run([]string{"check"}); err == nil || !strings.Contains(err.Error(), ".upgrade-pending") {
		t.Fatalf("未能识别未收尾升级: %v", err)
	}
	p.acquireErr = nil // 上面的只读检查不能取锁；下面是正常的 Commander 启动。
	if err := a.run([]string{"-c"}); !errors.Is(err, errPartialCase) {
		t.Fatalf("带未收尾升级时启动: %v", err)
	}
	if err := os.Remove(filepath.Join(root, ".steamai-vnext/.upgrade-pending")); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(root, ".steamai-vnext/pack-snapshot/snapshot.yml")
	private := []byte("fixture-not-for-output\n")
	if err := os.WriteFile(manifest, private, 0644); err != nil {
		t.Fatal(err)
	}
	if err := a.run([]string{"check"}); err == nil || !strings.Contains(err.Error(), "snapshot.yml") || strings.Contains(err.Error(), "fixture-not-for-output") {
		t.Fatalf("错误位置或隐私边界：%v", err)
	}
	if b, err := os.ReadFile(manifest); err != nil || !bytes.Equal(b, private) {
		t.Fatal("检查修改了无效清单")
	}
	a.cwd = func() (string, error) { return filepath.Join(root, ".steamai-vnext/members/builder"), nil }
	if err := a.run([]string{"check"}); err == nil || !strings.Contains(err.Error(), "项目根目录") {
		t.Fatalf("成员目录被当成新项目: %v", err)
	}
	if len(p.attached)+len(p.visible) != 0 {
		t.Fatal("项目检查不应启动会话")
	}
}

type beforeUpgradeAnswer struct {
	io.Reader
	before func()
}

func (r *beforeUpgradeAnswer) Read(p []byte) (int, error) {
	if r.before != nil {
		r.before()
		r.before = nil
	}
	return r.Reader.Read(p)
}

func TestCommanderOffersExactUpgradeWithoutChangingHowHistoryOpens(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	git, source := canonicalFreshFixture(t)
	root := t.TempDir()
	facts := casebootstrap.Facts{Name: "fixture", Goal: "test", Authorization: "fixture", Prohibited: "network", Stop: "drift", Pack: "fixture-pack"}
	fresh, err := casebootstrap.BuildPreview(git, source, root, facts)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := casebootstrap.Apply(git, source, root, facts, casebootstrap.ConfirmationPrefix+fresh.Identity); err != nil {
		t.Fatal(err)
	}
	policy := filepath.Join(root, ".steamai-vnext/pack-snapshot/common/policy.md")
	initial, err := os.ReadFile(policy)
	if err != nil {
		t.Fatal(err)
	}
	claude := makeFakeExecutable(t, "claude.exe")
	p := &fakePlatform{supported: true, source: source}
	a := testApp(t, p, root, claude)
	a.validateSource = func(string) error { return nil }
	a.lookPath = func(name string) (string, error) {
		if name == "claude.exe" {
			return claude, nil
		}
		if name != "git.exe" {
			t.Fatalf("意外查找 %s", name)
		}
		return git, nil
	}
	var out bytes.Buffer
	a.stdout = &out
	setPolicy := func(value string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(source, "common/policy.md"), []byte(value), 0644); err != nil {
			t.Fatal(err)
		}
	}
	checkPolicy := func(want string) {
		t.Helper()
		got, err := os.ReadFile(policy)
		if err != nil || string(got) != want {
			t.Fatalf("项目内容 = %q，读错误 = %v", got, err)
		}
	}
	setPolicy("v2\n") // HEAD 未变，不能只看版本号。
	start := func(args []string, answer string, prompted bool) {
		t.Helper()
		out.Reset()
		n := len(p.attached)
		a.stdin = strings.NewReader(answer)
		if err := a.run(args); err != nil {
			t.Fatal(err)
		}
		if len(p.attached) != n+1 || strings.Contains(out.String(), "输入 Y") != prompted {
			t.Fatalf("启动次数或更新提示不符：%s", out.String())
		}
	}
	start(nil, "Y\n", false) // 不是终端，不能把 stdin 当作授权或读走它。
	if rest, _ := io.ReadAll(a.stdin); string(rest) != "Y\n" {
		t.Fatal("吃掉重定向输入")
	}
	checkPolicy(string(initial))
	a.upgradeConsole = func(io.Reader, io.Writer) bool { return true }
	p.acquireErr = errCommanderRunning
	if err := a.run(nil); !errors.Is(err, errCommanderRunning) || out.Len() != 0 {
		t.Fatalf("另有 Commander 时仍尝试更新：%v", err)
	}
	p.acquireErr = nil
	start([]string{"-c"}, "\r\n", true)
	checkPolicy(string(initial))
	if spec := p.attached[len(p.attached)-1]; spec.Args[len(spec.Args)-1] != "--continue" || spec.Dir != root {
		t.Fatal("跳过更新丢弃了续聊入口")
	}
	// 确认到达时来源已不同：不写、不启动，也不自动把 Y 用到新计划上。
	n := len(p.attached)
	a.stdin = &beforeUpgradeAnswer{Reader: strings.NewReader("Y\n"), before: func() { setPolicy("v3\n") }}
	if err := a.run(nil); err == nil || len(p.attached) != n {
		t.Fatalf("预览过期仍然应用或打开：%v", err)
	}
	checkPolicy(string(initial))
	start([]string{"-r"}, "D\r\ny\r\nnext input\n", true)
	checkPolicy("v3\n")
	if !strings.Contains(out.String(), "pack-snapshot/common/policy.md") || strings.Contains(out.String(), "--apply") {
		t.Fatal("展开的详情不正确或仍需手抄命令")
	}
	if rest, _ := io.ReadAll(a.stdin); string(rest) != "next input\n" {
		t.Fatal("读走了后续会话的输入")
	}
	if spec := p.attached[len(p.attached)-1]; spec.Args[len(spec.Args)-1] != "--resume" || spec.Dir != root || strings.Contains(strings.Join(spec.Args, " "), source) {
		t.Fatal("采用新版后改变了历史入口或授权目录")
	}
	start(nil, "", false) // 新版本相同，不提问、不阻塞。
	p.source = ""
	start(nil, "", false) // 没有本机来源也能使用完整现有项目。
	checkPolicy("v3\n")
}

func TestUpgradeInputRequiresACompleteExplicitAnswer(t *testing.T) {
	p := casebootstrap.UpgradePreview{Summary: "更新摘要", Details: "框架文件清单\n"}
	for _, tt := range []struct {
		input    string
		yes, eof bool
	}{
		{"Y\r\n", true, false}, {"y\n", true, false}, {"\n", false, false}, {"n\n", false, false}, {"YES PLEASE\n", false, false}, {"y\rX\n", false, false}, {"d\nd\n\n", false, false}, {"Y", false, true}, {"", false, true},
	} {
		yes, err := chooseUpgrade(strings.NewReader(tt.input), io.Discard, p)
		if yes != tt.yes || errors.Is(err, io.EOF) != tt.eof {
			t.Errorf("输入 %q -> %v, %v", tt.input, yes, err)
		}
	}
}

func TestUpgradeCLIRequiresExactConfirmationAndIdleCommander(t *testing.T) {
	git, source := canonicalFreshFixture(t)
	root := t.TempDir()
	facts := casebootstrap.Facts{Name: "fixture", Goal: "test", Authorization: "fixture", Prohibited: "network", Stop: "drift", Pack: "fixture-pack"}
	fp, err := casebootstrap.BuildPreview(git, source, root, facts)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = casebootstrap.Apply(git, source, root, facts, casebootstrap.ConfirmationPrefix+fp.Identity); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(source, "common/policy.md"), []byte("changed\n"), 0644); err != nil {
		t.Fatal(err)
	}
	p := &fakePlatform{supported: true, source: source, acquireErr: errCommanderRunning}
	a := testApp(t, p, root, "")
	a.validateSource = func(string) error { return nil }
	a.lookPath = func(s string) (string, error) {
		if s != "git.exe" {
			t.Fatalf("不应调用 %s", s)
		}
		return git, nil
	}
	var out bytes.Buffer
	a.stdout = &out
	if err := a.run([]string{"upgrade"}); err != nil {
		t.Fatal(err)
	}
	_, id, ok := strings.Cut(out.String(), "steamai upgrade --apply ")
	id = strings.TrimSpace(id)
	if !ok || !sha256Pattern.MatchString(id) {
		t.Fatal("缺少可复制的确认命令")
	}
	if err := a.run([]string{"upgrade", "--apply", id}); !errors.Is(err, errCommanderRunning) {
		t.Fatalf("Commander 活动时没有拒绝: %v", err)
	}
	path := filepath.Join(root, ".steamai-vnext/pack-snapshot/common/policy.md")
	b, err := os.ReadFile(path)
	if err != nil || string(b) == "changed\n" {
		t.Fatal("项目在确认前或占锁时已被修改")
	}
	p.acquireErr = nil
	if err := a.run([]string{"upgrade", "--apply", id}); err != nil {
		t.Fatal(err)
	}
	if b, err = os.ReadFile(path); err != nil || string(b) != "changed\n" {
		t.Fatal("实际入口没有更新包内容")
	}
	if err := a.run([]string{"check"}); err != nil {
		t.Fatal(err)
	}
	if len(p.attached)+len(p.visible) != 0 {
		t.Fatal("升级不应启动会话")
	}
}
