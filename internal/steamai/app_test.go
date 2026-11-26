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
func (f *fakePlatform) AcquireCanonicalMutation(string) (commanderLease, error) {
	if f.acquireErr != nil {
		return commanderLease{}, f.acquireErr
	}
	return commanderLease{handle: 43, release: func() {}}, nil
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
	t.Setenv("LOCALAPPDATA", t.TempDir())
	t.Setenv("CLAUDECODE", "1")
	t.Setenv("CLAUDE_CODE_CHILD_SESSION", "1")
	t.Setenv("ANTHROPIC_MODEL", "configured-model")
	root := t.TempDir()
	source := makeCanonicalSource(t)
	claude := makeFakeExecutable(t, "claude.exe")

	for _, test := range []struct {
		name     string
		cliArgs  []string
		prepare  func(string)
		custom   bool
		wantArgs []string
	}{
		{name: "fresh", wantArgs: []string{"/steamai", "--add-dir", source}},
		{name: "current", prepare: func(root string) { materializeCurrentCaseFixture(t, root) }, wantArgs: []string{"/steamai"}},
		{name: "current auxiliary", prepare: func(root string) { materializeCurrentCaseFixtureWithAuxPack(t, root, "aux-pack") }, wantArgs: []string{"/steamai"}},
		{name: "configured fresh", custom: true, wantArgs: []string{"/steamai", "--add-dir", source}},
		{name: "configured current", custom: true, prepare: func(root string) { materializeCurrentCaseFixture(t, root) }, wantArgs: []string{"/steamai"}},
		{name: "pick fresh", cliArgs: []string{"-r"}, wantArgs: []string{"--resume", "--add-dir", source}},
		{name: "continue fresh", cliArgs: []string{"--continue"}, wantArgs: []string{"--continue", "--add-dir", source}},
		{name: "pick current", cliArgs: []string{"--resume"}, custom: true, prepare: func(root string) { materializeCurrentCaseFixture(t, root) }, wantArgs: []string{"--resume"}},
		{name: "continue current", cliArgs: []string{"-c"}, custom: true, prepare: func(root string) { materializeCurrentCaseFixture(t, root) }, wantArgs: []string{"--continue"}},
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
			high, standard := "opus", "sonnet"
			if test.custom {
				t.Setenv("LOCALAPPDATA", t.TempDir())
				high, standard = "sonnet", "haiku"
				if err := a.run([]string{"models", "--high", high, "--standard", standard}); err != nil {
					t.Fatal(err)
				}
			}
			if err := a.run(test.cliArgs); err != nil {
				t.Fatal(err)
			}
			if len(p.attached) != 1 {
				t.Fatalf("attached launches = %d", len(p.attached))
			}
			got := p.attached[0]
			assertRoleLaunch(t, got.Args, high, high, standard, test.wantArgs)
			wantCaseEntry := test.prepare != nil && len(test.cliArgs) == 0
			if strings.Contains(got.Args[3], ".steamai-vnext/CLAUDE.md") != wantCaseEntry {
				t.Fatalf("只有已有项目新开的 Commander 才应收到接班入口: %v", got.Args)
			}
			if got.Dir != caseRoot {
				t.Fatalf("cwd = %s want %s", got.Dir, caseRoot)
			}
			if got.CommanderLock != 42 {
				t.Fatalf("Commander was not handed its mutex handle: %v", got.CommanderLock)
			}
			assertIndependentSessionEnvironment(t, got.Env)
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

func TestCurrentAuxPackCorruptionFailsClosedBeforeLaunch(t *testing.T) {
	for _, pack := range []string{"fixture-pack", "aux-pack"} {
		t.Run(pack, func(t *testing.T) {
			root := t.TempDir()
			materializeCurrentCaseFixtureWithAuxPack(t, root, "aux-pack")
			path := filepath.Join(root, ".steamai-vnext/pack-snapshot/packs", pack, "router.md")
			if err := os.WriteFile(path, []byte("corrupt\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			p := &fakePlatform{supported: true}
			a := testApp(t, p, root, makeFakeExecutable(t, "claude.exe"))
			if err := a.run(nil); !errors.Is(err, errPartialCase) {
				t.Fatalf("损坏 %s 未 fail-closed: %v", pack, err)
			}
			if len(p.attached) != 0 {
				t.Fatal("损坏的 case 启动了 Claude")
			}
		})
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
	t.Setenv("LOCALAPPDATA", t.TempDir())
	caseRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(caseRoot, ".steamai-vnext"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := &fakePlatform{supported: true, source: makeCanonicalSource(t)}
	a := testApp(t, p, caseRoot, makeFakeExecutable(t, "claude.exe"))
	for _, args := range [][]string{nil, {"-r"}, {"-c"}} {
		if err := a.run(args); !errors.Is(err, errPartialCase) {
			t.Fatalf("partial case %v: %v", args, err)
		}
	}
	if len(p.attached) != 0 {
		t.Fatal("partial case launched Claude")
	}

	fresh := t.TempDir()
	p = &fakePlatform{supported: true, source: makeCanonicalSource(t), acquireErr: errCommanderRunning}
	a = testApp(t, p, fresh, makeFakeExecutable(t, "claude.exe"))
	for _, args := range [][]string{nil, {"--resume"}, {"--continue"}} {
		if err := a.run(args); !errors.Is(err, errCommanderRunning) {
			t.Fatalf("duplicate Commander %v: %v", args, err)
		}
	}
	if len(p.attached) != 0 {
		t.Fatal("duplicate Commander launched Claude")
	}
}

func TestOpenMemberIsVisibleAndBoundToMemberDirectory(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("CLAUDECODE", "1")
	t.Setenv("CLAUDE_CODE_CHILD_SESSION", "1")
	t.Setenv("ANTHROPIC_MODEL", "configured-model")
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
	if got.Dir != caseRoot || filepath.Base(got.Path) != "steamai.exe" {
		t.Fatalf("member helper path=%q cwd=%q", got.Path, got.Dir)
	}
	if !reflect.DeepEqual(got.Args, []string{"__member-session", "--config-dir", "-", "--tier", "standard", "--high", "opus", "--standard", "sonnet", "static-analysis"}) {
		t.Fatalf("member args = %#v", got.Args)
	}
	assertIndependentSessionEnvironment(t, got.Env)
	// pane 可能重新继承 server 的 child 标记和不同配置域，helper 必须再次清洗。
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	if err := a.run(got.Args); err != nil {
		t.Fatal(err)
	}
	if len(p.attached) != 1 || p.attached[0].Dir != member || p.attached[0].Path != claude {
		t.Fatal("pane 内 Claude 路径或成员 cwd 不符")
	}
	memberArgs := []string{"--name", "static-analysis", memberInitialPrompt, "--add-dir", caseRoot}
	assertRoleLaunch(t, p.attached[0].Args, "sonnet", "opus", "sonnet", memberArgs)
	assertIndependentSessionEnvironment(t, p.attached[0].Env)
	for _, entry := range p.attached[0].Env {
		if strings.HasPrefix(strings.ToUpper(entry), "CLAUDE_CONFIG_DIR=") {
			t.Fatal("默认配置域被 server 的配置域覆盖")
		}
	}
	configDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", configDir)
	if err := a.run([]string{"models", "--high", "sonnet", "--standard", "haiku"}); err != nil {
		t.Fatal(err)
	}
	if err := a.run([]string{"__open-member", "--model", "opus", "static-analysis"}); err != nil {
		t.Fatal(err)
	}
	if len(p.visible) != 2 || !reflect.DeepEqual(p.visible[1].Args, []string{"__member-session", "--config-dir", configDir, "--tier", "high", "--high", "sonnet", "--standard", "haiku", "static-analysis"}) {
		t.Fatal("旧 skill 的高档请求未使用所选模型与配置域")
	}
	assertIndependentSessionEnvironment(t, p.visible[1].Env)
	if err := a.run([]string{"__open-member", "--tier", "standard", "static-analysis"}); err != nil {
		t.Fatal(err)
	}
	// 模拟 pane 从 server 取得不同配置目录：两档应当由父启动器冻结。
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	t.Setenv("LOCALAPPDATA", t.TempDir())
	for i, want := range []string{"sonnet", "haiku"} {
		if err := a.run(p.visible[i+1].Args); err != nil {
			t.Fatal(err)
		}
		spec := p.attached[i+1]
		assertRoleLaunch(t, spec.Args, want, "sonnet", "haiku", memberArgs)
		if !strings.Contains(strings.Join(spec.Env, "\n"), "CLAUDE_CONFIG_DIR="+configDir) {
			t.Fatal("pane 内 Claude 未保留所选配置域")
		}
		assertIndependentSessionEnvironment(t, spec.Env)
	}
	for _, args := range [][]string{
		{"__member-session"},
		{"__member-session", "--config-dir", "relative", "--tier", "high", "--high", "sonnet", "--standard", "haiku", "static-analysis"},
		{"__member-session", "--config-dir", "-", "--tier", "sonnet", "--high", "sonnet", "--standard", "haiku", "static-analysis"},
		{"__member-session", "--config-dir", "-", "--tier", "high", "--high", "invalid", "--standard", "haiku", "static-analysis"},
		{"__member-session", "--config-dir", "-", "--tier", "standard", "--high", "sonnet", "--standard", "--debug", "static-analysis"},
		{"__member-session", "--config-dir", "-", "--tier", "high", "--high", "sonnet", "--standard", "haiku", "../escape"},
		{"__member-session", "--config-dir", "-", "--tier", "high", "--high", "sonnet", "--standard", "haiku", "static-analysis", "--trust-workspace"},
	} {
		if err := a.run(args); err == nil {
			t.Fatalf("invalid pane helper accepted: %v", args)
		}
	}
	if len(p.attached) != 3 {
		t.Fatal("invalid helper options launched a member")
	}

	for _, invalid := range []string{"../escape", "UPPER", "con", "a/b"} {
		if err := a.run([]string{"__open-member", invalid}); err == nil {
			t.Fatalf("invalid member %q accepted", invalid)
		}
	}

	// 父启动器只开当前项目已验证的成员；请求不能自带目录、命令或工作区信任。
	id, _ := p.CaseIdentity(caseRoot)
	open := p.attached[0].OpenMemberWindow // 该成员来自默认配置域，不得借用当前 server 的配置域。
	req := memberWindowRequest{CaseID: id, Name: "static-analysis", Tier: tierStandard}
	n := len(p.visible)
	for _, change := range []func(*memberWindowRequest){
		func(r *memberWindowRequest) { r.CaseID = "other" },
		func(r *memberWindowRequest) { r.Name = "../escape" },
		func(r *memberWindowRequest) { r.Name = "missing" },
		func(r *memberWindowRequest) { r.Tier = "--help" },
	} {
		bad := req
		change(&bad)
		if err := open(bad); err == nil || len(p.visible) != n {
			t.Fatal("无效的窗口请求打开了成员")
		}
	}
	stale := a.memberWindowOpener(caseRoot, "old-id", "-")
	old := req
	old.CaseID = "old-id"
	if err := stale(old); err == nil || len(p.visible) != n {
		t.Fatal("绑定的项目物理身份已变，不能打开成员")
	}
	t.Setenv(memberWindowAddressEnv, "stale-parent-route")
	for i, config := range []string{"-", configDir} {
		if err := p.attached[i].OpenMemberWindow(req); err != nil || len(p.visible) != n+i+1 {
			t.Fatalf("父启动器未创建成员窗口: %v", err)
		}
		got := p.visible[n+i]
		if got.Dir != caseRoot || !reflect.DeepEqual(got.Args, []string{"__member-session", "--config-dir", config, "--tier", "standard", "--high", "opus", "--standard", "sonnet", "static-analysis"}) {
			t.Fatalf("成员窗口未使用父端绑定的项目、配置域和本机档位: %v", got.Args)
		}
		assertIndependentSessionEnvironment(t, got.Env)
	}
	if err := a.run([]string{"__open-member", "static-analysis"}); err == nil || len(p.visible) != n+2 {
		t.Fatal("失效的父会话入口不能回退到本地重复创建窗口")
	}
}

func TestInvalidModelSettingsStopBeforeMemberTrustOrLaunch(t *testing.T) {
	root := t.TempDir()
	t.Setenv("LOCALAPPDATA", root)
	if err := os.Mkdir(filepath.Join(root, "STeamAI"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "STeamAI", "models.json"), []byte(`{"high":"sonnet","standard":"typo"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", profile)
	configPath := filepath.Join(profile, ".claude.json")
	const initial = `{"projects":{}}`
	if err := os.WriteFile(configPath, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	caseRoot, _ := prepareTrustedMemberCase(t, "worker")
	p := &fakePlatform{supported: true}
	a := testApp(t, p, caseRoot, makeFakeExecutable(t, "claude.exe"))
	for _, args := range [][]string{nil, {"__open-member", "--trust-workspace", "worker"}} {
		if err := a.run(args); err == nil {
			t.Fatal("非法配置不应自动退回默认档")
		}
	}
	data, err := os.ReadFile(configPath)
	if err != nil || string(data) != initial || len(p.visible) != 0 || len(p.attached) != 0 {
		t.Fatal("非法档位配置修改了信任或启动了会话")
	}
}

func TestMemberLaunchOptions(t *testing.T) {
	for _, test := range []struct {
		args  []string
		trust bool
		tier  string
	}{
		{[]string{"worker"}, false, "standard"},
		{[]string{"--trust-workspace", "worker"}, true, "standard"},
		{[]string{"--tier", "high", "worker"}, false, "high"},
		{[]string{"--tier", "standard", "--trust-workspace", "worker"}, true, "standard"},
		{[]string{"--model", "opus", "worker"}, false, "high"},
		{[]string{"--trust-workspace", "--model", "sonnet", "worker"}, true, "standard"},
	} {
		name, trust, tier, err := memberLaunchOptions(test.args)
		if err != nil || name != "worker" || trust != test.trust || tier != test.tier {
			t.Fatalf("%v: name=%q trust=%v tier=%q err=%v", test.args, name, trust, tier, err)
		}
	}
	for _, args := range [][]string{
		nil, {"--tier"}, {"--tier", "haiku", "worker"}, {"--tier", "high", "--model", "opus", "worker"},
		{"--model", "sonnet", "--tier", "standard", "worker"}, {"", "worker"},
		{"--model"}, {"--model", "opus"}, {"--model", "unknown", "worker"},
		{"--model", "opus", "--model", "sonnet", "worker"},
		{"--trust-workspace", "--trust-workspace", "worker"},
		{"--unknown", "worker"}, {"worker", "second"}, {"--model", "--trust-workspace", "worker"},
	} {
		if _, _, _, err := memberLaunchOptions(args); err == nil {
			t.Fatalf("invalid member options accepted: %v", args)
		}
	}
}

func assertRoleLaunch(t *testing.T, args []string, model, high, standard string, wantTail []string) {
	t.Helper()
	if len(args) < 4 || args[0] != "--model" || args[1] != model || args[2] != "--append-system-prompt" || !reflect.DeepEqual(args[4:], wantTail) {
		t.Fatalf("Claude 启动参数不符: %q", args)
	}
	for _, phrase := range []string{"high=" + high, "standard=" + standard, "默认显式传 model=" + standard, "确需高档时传 model=" + high} {
		if !strings.Contains(args[3], phrase) {
			t.Fatalf("角色及子 agent 提示缺少 %q", phrase)
		}
	}
}

func assertIndependentSessionEnvironment(t *testing.T, env []string) {
	t.Helper()
	modelPreserved := false
	for _, item := range env {
		key, value, _ := strings.Cut(item, "=")
		switch strings.ToUpper(key) {
		case "CLAUDECODE", "CLAUDE_CODE_CHILD_SESSION", memberWindowAddressEnv:
			t.Fatalf("独立会话继承了父级会话标记：%s", key)
		case "ANTHROPIC_MODEL":
			modelPreserved = value == "configured-model"
		}
	}
	if !modelPreserved {
		t.Fatal("独立会话不得改变用户模型配置")
	}
}

func TestCommanderHistoryRefusesMemberDirectory(t *testing.T) {
	cwd := filepath.Join(t.TempDir(), ".steamai-vnext", "members", "reader")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}
	p := &fakePlatform{supported: true}
	a := testApp(t, p, cwd, makeFakeExecutable(t, "claude.exe"))
	for _, flag := range []string{"-r", "-c"} {
		if err := a.run([]string{flag}); err == nil || !strings.Contains(err.Error(), "原项目根目录") {
			t.Fatalf("从成员目录运行 %s：%v", flag, err)
		}
	}
	if len(p.attached) != 0 {
		t.Fatal("不应把成员作为 Commander 恢复")
	}
}

func TestHistoryFlagsRejectExtraArgumentsBeforeLaunch(t *testing.T) {
	p := &fakePlatform{supported: true}
	a := testApp(t, p, t.TempDir(), makeFakeExecutable(t, "claude.exe"))
	for _, args := range [][]string{{"-r", "-c"}, {"--continue", "text"}, {"--resume", "other-project"}, {"-c", "--add-dir", "."}} {
		if err := a.run(args); err == nil || !strings.Contains(err.Error(), "不接受其他参数") {
			t.Fatalf("unexpected result for %v: %v", args, err)
		}
	}
	if len(p.attached) != 0 || len(p.visible) != 0 {
		t.Fatal("多余的参数不应启动会话")
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
	for _, flag := range []string{"steamai -r", "--resume", "steamai -c", "--continue"} {
		if !strings.Contains(output.String(), flag) {
			t.Errorf("帮助中缺少历史入口 %s", flag)
		}
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
	t.Setenv(memberWindowAddressEnv, "") // 测试不能联系运行 go test 的真实会话。
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
