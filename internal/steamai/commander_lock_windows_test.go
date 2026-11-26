//go:build windows

package steamai

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

// Commander 锁与会话生命周期用真实 Windows 进程链验证。替身全部是测试二进制自身的无窗口子进程，
// 只用测试自己的 TEMP 目录、唯一 mutex 名和唯一退出事件名，不接触真实 case 或用户进程。
//
//	go test（启动器）
//	├─ 哨兵：别的项目/窗口的替身，启动器的普通子进程，不属于任何会话 Job
//	└─ Commander 会话替身                              [Commander 的 Job]
//	   ├─ 工具替身 → 下级工具替身（多层）                [同一 Job]
//	   ├─ detached/no-window 工具替身                    [同一 Job]
//	   └─ __open-member 替身 → 经成员窗口通道向启动器请求固定成员
//	      └─ 成员启动器替身（启动器在自己层级创建，不在任何会话 Job 内）→ RunAttached（无锁）→ 成员会话替身 [成员自己的 Job]
//	         ├─ 工具替身、detached 工具替身              [成员的 Job]
//	         └─ __open-member 替身 → 经成员启动器的通道 → 独立子进程替身（成员启动器创建）
//
// 每个替身启动后按 PID 打开进程句柄并核验镜像，句柄保留到测试结束；之后的存活、退出、终止只走句柄。
// 测试清理时触发退出事件，全部替身（包括没能及时固定的）自行退出，不靠按名字或 PID 扫杀。

const (
	helperModeEnv     = "STEAMAI_TEST_COMMANDER_MODE"
	helperLabelEnv    = "STEAMAI_TEST_COMMANDER_LABEL"
	helperDirEnv      = "STEAMAI_TEST_COMMANDER_DIR"
	helperRoleEnv     = "STEAMAI_TEST_COMMANDER_ROLE"
	helperNameEnv     = "STEAMAI_TEST_COMMANDER_NAME"
	helperShutdownEnv = "STEAMAI_TEST_COMMANDER_SHUTDOWN"

	helperRunArg = "-test.run=^TestCommanderHelperProcess$"

	// 成员窗口通道只放行这两个固定成员名；请求不携带路径或环境变量。
	commanderMemberName   = "fixture-member"
	memberIndependentName = "fixture-independent"
	fixtureCaseID         = "fixture-case"

	helperDetachedProcess       = 0x00000008
	helperCreateNewProcessGroup = 0x00000200
	// 替身到点自退：测试进程意外消失、没人触发退出事件时也不留常驻进程。
	helperLifetime = 60 * time.Second
	// 测试终止替身时的退出码，用来核对启动器如实报告被终止的会话。
	helperTerminatedCode = 0xEE01
	// 取锁之后旧 Commander 至多还能落地一次在途心跳。
	helperBeatSlack                = 50 * time.Millisecond
	processQueryLimitedInformation = 0x1000
)

var (
	helperCreateEventW              = kernel32.NewProc("CreateEventW")
	helperOpenEventW                = kernel32.NewProc("OpenEventW")
	helperSetEvent                  = kernel32.NewProc("SetEvent")
	helperQueryFullProcessImageName = kernel32.NewProc("QueryFullProcessImageNameW")

	commanderLabels = []string{"commander", "commander-tool", "commander-subtool", "commander-detached", "commander-independent"}
	memberLabels    = []string{"member", "member-tool", "member-subtool", "member-detached", "member-independent"}

	// 会话 Job 内：会话本身与它的工具（普通、多层、detached）。
	commanderGroup = commanderLabels[:4]
	memberGroup    = memberLabels[:4]
	// Commander 结束后必须仍在世：成员启动器、正式成员整条链、别的项目的哨兵。
	commanderSurvivors = append(commanderLabels[4:], append(memberLabels, "sentinel")...)
)

// TestCommanderHelperProcess 只在带 helper 环境变量再执行时工作。
func TestCommanderHelperProcess(t *testing.T) {
	if os.Getenv(helperModeEnv) == "" {
		return
	}
	os.Exit(commanderHelperMain())
}

// helperConfig 是再执行测试二进制所需的环境：mode 决定替身行为，dir 同时是工作目录和报告目录。
type helperConfig struct {
	mode, label, dir, role, name, shutdown string
}

func commanderHelperMain() int {
	h := helperConfig{
		mode:     os.Getenv(helperModeEnv),
		label:    os.Getenv(helperLabelEnv),
		dir:      os.Getenv(helperDirEnv),
		role:     os.Getenv(helperRoleEnv),
		name:     os.Getenv(helperNameEnv),
		shutdown: os.Getenv(helperShutdownEnv),
	}
	watchHelperShutdown(h.shutdown)
	exe, err := os.Executable()
	if err != nil {
		return 3
	}
	switch h.mode {
	case "launcher":
		// 与 openCommander 相同：先取锁，再以启动器身份运行直属 Commander，并只替它放行固定成员。
		lease, err := (nativePlatform{}).AcquireCommander(h.name)
		if err != nil {
			return h.fail(err)
		}
		defer lease.release()
		session := h.session("commander", exe, lease.handle)
		session.OpenMemberWindow = fixtureMemberWindow(exe, commanderMemberName, h.commanderMember())
		if err := (nativePlatform{}).RunAttached(session, nil, nil, nil); err != nil {
			return h.fail(err)
		}
		return 0
	case "session":
		return h.runSession(exe)
	case "open-member":
		// steamai.exe __open-member：会话内的工具入口。严格 Job 内不能自己开窗口，
		// 只把固定成员名交给启动器的成员窗口通道，由启动器在自己层级创建，随后立即返回。
		// Commander 的独立成员是正式成员会话；成员会话自己的独立子进程只是长驻替身。
		name := memberIndependentName
		if h.role == "commander" {
			name = commanderMemberName
		}
		address := os.Getenv(memberWindowAddressEnv)
		if address == "" {
			return h.fail(errors.New("会话没有成员窗口通道"))
		}
		// 两档别名由启动器自己解析，请求只带固定成员身份与档位。
		err := requestMemberWindow(address, memberWindowRequest{
			Name: name, CaseID: fixtureCaseID, Tier: tierStandard,
		})
		if err != nil {
			return h.fail(err)
		}
		return 0
	case "member-launcher":
		// pane 内的 steamai.exe __member-session：正式成员会话不带锁，在成员子目录里运行；
		// 成员自己的独立子进程同样只经本启动器的通道创建。
		if err := h.report(); err != nil {
			return h.fail(err)
		}
		member := h.child("session", "member")
		member.role, member.dir = "member", filepath.Join(h.dir, "member")
		spec := member.spec(exe, 0)
		spec.OpenMemberWindow = fixtureMemberWindow(exe, memberIndependentName, member.child("tool", "member-independent"))
		if err := (nativePlatform{}).RunAttached(spec, nil, nil, nil); err != nil {
			return h.fail(err)
		}
		return 0
	case "tool":
		return h.runTool(exe)
	}
	return 2 // 未识别的模式：启动后主动以非零退出。
}

// runSession 模拟 Claude Code 会话：派生多层工具、detached 工具，并经工具入口打开一个独立成员。
func (h helperConfig) runSession(exe string) int {
	if err := h.report(); err != nil {
		return h.fail(err)
	}
	// 普通工具与 Claude Code 派生工具一致：bInheritHandles=TRUE 且没有 handle list；下级工具形成多层。
	if err := h.child("tool", h.label+"-tool").spawn(exe, createNoWindow); err != nil {
		return h.fail(err)
	}
	if err := h.child("tool", h.label+"-detached").spawn(exe, helperDetachedProcess|helperCreateNewProcessGroup|createNoWindow); err != nil {
		return h.fail(err)
	}
	// 独立成员一律经会话内的工具入口向启动器请求，不在会话 Job 内创建。
	if err := h.child("open-member", h.label+"-independent").spawn(exe, createNoWindow); err != nil {
		return h.fail(err)
	}
	return h.waitForStop()
}

// commanderMember 是 Commander 的启动器放行固定成员后，在自己层级创建的成员启动器替身。
func (h helperConfig) commanderMember() helperConfig {
	return helperConfig{mode: "member-launcher", label: "commander-independent", dir: h.dir, role: "commander", shutdown: h.shutdown}
}

// fixtureMemberWindow 是启动器一侧的成员窗口通道：只放行本测试固定的成员身份（成员名与 case），
// 目标进程始终由启动器按自己的配置拼装，请求里没有任何路径或环境变量可被采用。
func fixtureMemberWindow(exe, allowed string, target helperConfig) func(memberWindowRequest) error {
	return func(request memberWindowRequest) error {
		if request.Name != allowed || request.CaseID != fixtureCaseID {
			return errors.New("成员窗口通道只放行本测试的固定成员: " + allowed)
		}
		return startIndependentMember(target.spec(exe, 0), createNoWindow)
	}
}

func (h helperConfig) runTool(exe string) int {
	if err := h.report(); err != nil {
		return h.fail(err)
	}
	if parent, found := strings.CutSuffix(h.label, "-tool"); found {
		if err := h.child("tool", parent+"-subtool").spawn(exe, createNoWindow); err != nil {
			return h.fail(err)
		}
	}
	time.Sleep(helperLifetime)
	return 0
}

func (h helperConfig) child(mode, label string) helperConfig {
	return helperConfig{mode: mode, label: label, dir: h.dir, role: h.role, shutdown: h.shutdown}
}

func (h helperConfig) session(role, exe string, lock uintptr) processSpec {
	session := h.child("session", role)
	session.role = role
	return session.spec(exe, lock)
}

func (h helperConfig) env() []string {
	// 替身一律模拟没有 WezTerm 痕迹的原生会话：成员窗口只经启动器的通道创建，
	// 与 go test 自己跑在哪个终端里无关。
	env := withoutEnvironment(os.Environ(),
		helperModeEnv, helperLabelEnv, helperDirEnv, helperRoleEnv, helperNameEnv, helperShutdownEnv,
		weztermPaneEnv, weztermTermProgramEnv)
	for _, item := range [][2]string{
		{helperModeEnv, h.mode}, {helperLabelEnv, h.label}, {helperDirEnv, h.dir},
		{helperRoleEnv, h.role}, {helperNameEnv, h.name}, {helperShutdownEnv, h.shutdown},
	} {
		if item[1] != "" {
			env = append(env, item[0]+"="+item[1])
		}
	}
	return env
}

func (h helperConfig) spec(exe string, lock uintptr) processSpec {
	return processSpec{Path: exe, Args: []string{helperRunArg}, Dir: h.dir, Env: h.env(), CommanderLock: lock}
}

// spawn 创建子替身并立即放开父侧句柄。
func (h helperConfig) spawn(exe string, flags uint32) error {
	spec := h.spec(exe, 0)
	app, err := syscall.UTF16PtrFromString(spec.Path)
	if err != nil {
		return err
	}
	commandLine, err := syscall.UTF16PtrFromString(strings.Join(append([]string{syscall.EscapeArg(spec.Path)}, spec.Args...), " "))
	if err != nil {
		return err
	}
	env, err := windowsEnvironmentBlock(spec.Env)
	if err != nil {
		return err
	}
	dir, err := syscall.UTF16PtrFromString(spec.Dir)
	if err != nil {
		return err
	}
	startup := syscall.StartupInfo{Cb: uint32(unsafe.Sizeof(syscall.StartupInfo{}))}
	var info syscall.ProcessInformation
	if err = syscall.CreateProcess(app, commandLine, nil, nil, true, flags|createUnicodeEnv, &env[0], dir, &startup, &info); err != nil {
		return err
	}
	_ = syscall.CloseHandle(info.Thread)
	_ = syscall.CloseHandle(info.Process)
	return nil
}

// watchHelperShutdown 等测试结束时的退出通知；通知已经不在说明测试已结束。
func watchHelperShutdown(name string) {
	if name == "" {
		return
	}
	pointer, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return
	}
	handle, _, _ := helperOpenEventW.Call(syscall.SYNCHRONIZE, 0, uintptr(unsafe.Pointer(pointer)))
	if handle == 0 {
		os.Exit(0)
	}
	go func() {
		_, _ = syscall.WaitForSingleObject(syscall.Handle(handle), syscall.INFINITE)
		os.Exit(0)
	}()
}

// waitForStop 等用户退出（测试写出 stop 文件），每轮记一次心跳；到点自退。
func (h helperConfig) waitForStop() int {
	stop := filepath.Join(h.dir, "stop")
	for deadline := time.Now().Add(helperLifetime); time.Now().Before(deadline); {
		if _, err := os.Stat(stop); err == nil {
			return 0
		}
		_ = writeHelperFile(filepath.Join(h.dir, h.label+".beat"), strconv.FormatInt(time.Now().UnixMilli(), 10))
		time.Sleep(20 * time.Millisecond)
	}
	return 5
}

// report 写出 <dir>/<label>.pid。先写临时文件再改名，测试侧只会读到完整内容。
func (h helperConfig) report() error {
	return writeHelperFile(filepath.Join(h.dir, h.label+".pid"), strconv.Itoa(os.Getpid()))
}

// fail 把失败原因写到 <dir>/<label>.error 供测试读取，并以非零退出。
func (h helperConfig) fail(cause error) int {
	_ = writeHelperFile(filepath.Join(h.dir, h.label+".error"), cause.Error())
	return 7
}

func writeHelperFile(path, text string) error {
	if err := os.WriteFile(path+".tmp", []byte(text), 0o644); err != nil {
		return err
	}
	return os.Rename(path+".tmp", path)
}

// pinnedProcess 是已核验属于本次测试二进制的进程句柄，保留到测试结束。
type pinnedProcess struct {
	label  string
	handle syscall.Handle
}

func (p *pinnedProcess) ended() bool {
	event, err := syscall.WaitForSingleObject(p.handle, 0)
	return err == nil && event == syscall.WAIT_OBJECT_0
}

func (p *pinnedProcess) waitEnded(timeout time.Duration) bool {
	event, err := syscall.WaitForSingleObject(p.handle, uint32(timeout.Milliseconds()))
	return err == nil && event == syscall.WAIT_OBJECT_0
}

func (p *pinnedProcess) terminate() {
	_ = syscall.TerminateProcess(p.handle, helperTerminatedCode)
}

// helperTree 按固定句柄跟踪本次测试启动的替身，并持有通知全部替身退出的唯一事件。
type helperTree struct {
	exe      string
	dir      string
	shutdown string
	event    syscall.Handle
	procs    []*pinnedProcess
	cmds     []*exec.Cmd
}

// newHelperTree 准备 TEMP 目录、退出事件和"别的项目"的哨兵。
func newHelperTree(t *testing.T) *helperTree {
	t.Helper()
	tree := &helperTree{exe: helperExecutable(t), dir: t.TempDir()}
	// 事件名带时间戳和 PID：同包测试并发再执行、或别的进程同时跑同一套用例时互不干扰。
	tree.shutdown = `Local\STeamAI.Test.` + strconv.FormatInt(time.Now().UnixNano(), 36) + "." + strconv.Itoa(os.Getpid())
	name, err := syscall.UTF16PtrFromString(tree.shutdown)
	if err != nil {
		t.Fatal(err)
	}
	event, _, callErr := helperCreateEventW.Call(0, 1, 0, uintptr(unsafe.Pointer(name)))
	if event == 0 {
		t.Fatal(callErr)
	}
	if errors.Is(callErr, syscall.ERROR_ALREADY_EXISTS) {
		_ = syscall.CloseHandle(syscall.Handle(event))
		t.Fatalf("退出事件名重复: %s", tree.shutdown)
	}
	tree.event = syscall.Handle(event)
	// 成员启动器替身以成员子目录作为成员会话的工作和报告目录。
	if err := os.MkdirAll(filepath.Join(tree.dir, "member"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// 先通知全部替身自行退出；已固定却没有退出的再按句柄终止。
		_, _, _ = helperSetEvent.Call(uintptr(tree.event))
		deadline := time.Now().Add(10 * time.Second)
		for _, p := range tree.procs {
			if !p.waitEnded(time.Second) {
				p.terminate()
			}
			if remaining := time.Until(deadline); remaining > 0 {
				p.waitEnded(remaining)
			}
			_ = syscall.CloseHandle(p.handle)
		}
		for _, cmd := range tree.cmds {
			_ = cmd.Wait()
		}
		_ = syscall.CloseHandle(tree.event)
	})
	tree.startCommand(t, tree.config("tool", "sentinel"), "sentinel")
	return tree
}

func (tree *helperTree) config(mode, label string) helperConfig {
	return helperConfig{mode: mode, label: label, dir: tree.dir, shutdown: tree.shutdown}
}

// startCommand 用启动器的普通方式（exec.Command）启动一个不在会话 Job 里的替身。
func (tree *helperTree) startCommand(t *testing.T, cfg helperConfig, label string) *pinnedProcess {
	t.Helper()
	cmd := exec.Command(tree.exe, helperRunArg)
	cmd.Dir = cfg.dir
	cmd.Env = cfg.env()
	if err := cmd.Start(); err != nil {
		t.Fatalf("启动 %s 替身: %v", label, err)
	}
	tree.cmds = append(tree.cmds, cmd)
	return tree.pinPID(t, label, uint32(cmd.Process.Pid))
}

// pinPID 把 PID 换成进程句柄并核验镜像；确认是本次测试二进制后才纳入等待和终止。
func (tree *helperTree) pinPID(t *testing.T, label string, pid uint32) *pinnedProcess {
	t.Helper()
	handle, err := syscall.OpenProcess(syscall.SYNCHRONIZE|processQueryLimitedInformation|syscall.PROCESS_TERMINATE, false, pid)
	if err != nil {
		t.Fatalf("打开 %s 替身(pid %d)的进程句柄: %v", label, pid, err)
	}
	image, err := processImage(handle)
	if err != nil || !strings.EqualFold(image, tree.exe) {
		_ = syscall.CloseHandle(handle)
		t.Fatalf("%s 替身(pid %d)不是本次测试二进制: image=%q err=%v", label, pid, image, err)
	}
	p := &pinnedProcess{label: label, handle: handle}
	tree.procs = append(tree.procs, p)
	return p
}

// pinFile 等替身写出自己的 PID 后固定句柄。
func (tree *helperTree) pinFile(t *testing.T, dir, label string) *pinnedProcess {
	t.Helper()
	path := filepath.Join(dir, label+".pid")
	if !waitHelperFile(path, 15*time.Second, func() bool { return helperFailures(tree.dir) != "" }) {
		t.Fatalf("%s 替身没有启动: %s", label, helperFailureReport(tree.dir))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 32)
	if err != nil {
		t.Fatalf("%s 替身的 pid 文件无效: %q", label, data)
	}
	return tree.pinPID(t, label, uint32(pid))
}

// pinSessionTree 固定 Commander 会话、它的工具、独立成员整条链。
func (tree *helperTree) pinSessionTree(t *testing.T) {
	t.Helper()
	for _, label := range commanderLabels {
		tree.pinFile(t, tree.dir, label)
	}
	for _, label := range memberLabels {
		tree.pinFile(t, filepath.Join(tree.dir, "member"), label)
	}
}

func (tree *helperTree) get(label string) *pinnedProcess {
	for _, p := range tree.procs {
		if p.label == label {
			return p
		}
	}
	panic("未固定的替身: " + label) // 只会是测试代码自身的错误。
}

// requireEnded 等组内替身全部退出。
func (tree *helperTree) requireEnded(t *testing.T, labels ...string) {
	t.Helper()
	for _, label := range labels {
		if p := tree.get(label); !p.waitEnded(10 * time.Second) {
			t.Fatalf("%s 在会话结束后仍未退出", label)
		}
	}
}

// requireAlive 断言这些替身没有被波及。
func (tree *helperTree) requireAlive(t *testing.T, labels ...string) {
	t.Helper()
	for _, label := range labels {
		if tree.get(label).ended() {
			t.Fatalf("%s 不属于已结束的会话，却被一并结束", label)
		}
	}
}

// requireSilentSince 断言会话在 since 之后没有再运行：锁副本随进程句柄表一起释放，
// 之后若还有心跳，说明锁已经交出去而旧 Commander 还在写。容忍至多一次在途心跳。
func (tree *helperTree) requireSilentSince(t *testing.T, label string, since time.Time) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(tree.dir, label+".beat"))
	if err != nil {
		t.Fatalf("读取 %s 的心跳: %v", label, err)
	}
	beat, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		t.Fatalf("%s 的心跳无效: %q", label, data)
	}
	if last := time.UnixMilli(beat); last.After(since.Add(helperBeatSlack)) {
		t.Fatalf("锁已交给下一个 Commander 之后旧 Commander 仍在运行：最后一次心跳 %s，取锁于 %s",
			last.Format("15:04:05.000"), since.Format("15:04:05.000"))
	}
}

func processImage(handle syscall.Handle) (string, error) {
	var (
		buffer [1024]uint16
		size   = uint32(len(buffer))
	)
	ok, _, err := helperQueryFullProcessImageName.Call(uintptr(handle), 0, uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&size)))
	if ok == 0 {
		return "", err
	}
	return syscall.UTF16ToString(buffer[:size]), nil
}

func helperExecutable(t *testing.T) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return exe
}

// waitHelperFile 等文件出现；期间 abort（可为 nil）返回 true 就提前结束。
func waitHelperFile(path string, timeout time.Duration, abort func() bool) bool {
	for deadline := time.Now().Add(timeout); ; {
		if _, err := os.Stat(path); err == nil {
			return true
		}
		if (abort != nil && abort()) || time.Now().After(deadline) {
			return false
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// helperFailures 汇总替身写出的错误报告；没有则返回空串。
func helperFailures(root string) string {
	var messages []string
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && strings.HasSuffix(entry.Name(), ".error") {
			if data, readErr := os.ReadFile(path); readErr == nil {
				messages = append(messages, entry.Name()+"="+strings.TrimSpace(string(data)))
			}
		}
		return nil
	})
	return strings.Join(messages, "；")
}

func helperFailureReport(root string) string {
	if failures := helperFailures(root); failures != "" {
		return failures
	}
	return "目录内没有替身错误报告"
}

func requireCommanderRejected(t *testing.T, name string, want error) {
	t.Helper()
	if _, err := (nativePlatform{}).AcquireCommander(name); !errors.Is(err, want) {
		t.Fatalf("AcquireCommander = %v, want %v", err, want)
	}
}

// requireLockFreeWhileMemberAlive：Commander 已结束而独立成员仍在世时，锁必须可以交给下一个 Commander。
func requireLockFreeWhileMemberAlive(t *testing.T, name string) {
	t.Helper()
	second, err := (nativePlatform{}).AcquireCommander(name)
	if err != nil {
		t.Fatalf("独立成员仍在世，Commander 锁却被占用: %v", err)
	}
	second.release()
}

func waitHelperDone(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(30 * time.Second):
		t.Fatal("启动器没有随会话结束返回")
		return nil
	}
}

// commanderFixture 以测试进程为启动器：取锁后经 RunAttached 运行直属 Commander 替身。
type commanderFixture struct {
	tree  *helperTree
	lease commanderLease
	done  <-chan error
}

func startCommanderTree(t *testing.T, name string) *commanderFixture {
	t.Helper()
	tree := newHelperTree(t)
	lease, err := (nativePlatform{}).AcquireCommander(name)
	if err != nil {
		t.Fatal(err)
	}
	// 测试进程就是 Commander 的启动器：只替会话放行固定成员，并在自己层级创建成员启动器。
	session := tree.config("session", "commander").session("commander", tree.exe, lease.handle)
	session.OpenMemberWindow = fixtureMemberWindow(tree.exe, commanderMemberName, tree.config("launcher", "launcher").commanderMember())
	done := make(chan error, 1)
	go func() {
		done <- (nativePlatform{}).RunAttached(session, nil, nil, nil)
	}()
	tree.pinSessionTree(t)
	return &commanderFixture{tree: tree, lease: lease, done: done}
}

// startLauncherTree 用独立的启动器替身（与 openCommander 相同的取锁 + RunAttached），以便把启动器整体杀掉。
func startLauncherTree(t *testing.T, name string) (*helperTree, *pinnedProcess) {
	t.Helper()
	tree := newHelperTree(t)
	launcherConfig := tree.config("launcher", "launcher")
	launcherConfig.name = name
	launcher := tree.startCommand(t, launcherConfig, "launcher")
	tree.pinSessionTree(t)
	return tree, launcher
}

func TestCommanderExitEndsItsToolsAndSparesIndependentMembers(t *testing.T) {
	name := commanderMutexName(t.TempDir())
	fixture := startCommanderTree(t, name)

	// 直属 Commander 在世时拒绝第二写者，启动器先放开自己的句柄也一样。
	requireCommanderRejected(t, name, errCommanderRunning)
	fixture.lease.release()
	requireCommanderRejected(t, name, errCommanderRunning)

	// 用户在会话里正常退出。
	if err := os.WriteFile(filepath.Join(fixture.tree.dir, "stop"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := waitHelperDone(t, fixture.done); err != nil {
		t.Fatalf("Commander 自行退出，启动器却报告失败: %v", err)
	}
	fixture.tree.requireEnded(t, commanderGroup...)
	fixture.tree.requireAlive(t, commanderSurvivors...)
	requireLockFreeWhileMemberAlive(t, name)
}

func TestCommanderTerminatedSparesMemberSessionWhichEndsOnlyItsOwnGroup(t *testing.T) {
	name := commanderMutexName(t.TempDir())
	fixture := startCommanderTree(t, name)
	fixture.lease.release()
	requireCommanderRejected(t, name, errCommanderRunning)

	// 只终止 Commander 这一个进程：启动器如实报告被终止，会话内工具随之退出。
	fixture.tree.get("commander").terminate()
	if err := waitHelperDone(t, fixture.done); err == nil || !strings.Contains(err.Error(), "exit status "+strconv.Itoa(helperTerminatedCode)) {
		t.Fatalf("Commander 被终止，启动器返回: %v", err)
	}
	fixture.tree.requireEnded(t, commanderGroup...)
	fixture.tree.requireAlive(t, commanderSurvivors...)
	requireLockFreeWhileMemberAlive(t, name)

	// 关闭成员窗口即成员启动器被杀：只回收成员会话自己的组。
	fixture.tree.get("commander-independent").terminate()
	fixture.tree.requireEnded(t, append([]string{"commander-independent"}, memberGroup...)...)
	fixture.tree.requireAlive(t, "member-independent", "sentinel")
}

func TestCommanderLauncherKilledEndsOwnedGroup(t *testing.T) {
	name := commanderMutexName(t.TempDir())
	tree, launcher := startLauncherTree(t, name)
	requireCommanderRejected(t, name, errCommanderRunning)

	// 启动器被杀：它名下的 Commander 和工具整组由系统结束，锁随直属 Commander 一起空出来。
	launcher.terminate()
	lease, acquiredAt := acquireOnceCommanderEnded(t, name, tree.get("commander"))
	defer lease.release()
	tree.requireEnded(t, commanderGroup...)
	tree.requireSilentSince(t, "commander", acquiredAt)
	tree.requireAlive(t, commanderSurvivors...)
}

// acquireOnceCommanderEnded 在启动器被杀后反复取锁，直到直属 Commander 结束、锁空出来。
func acquireOnceCommanderEnded(t *testing.T, name string, commander *pinnedProcess) (commanderLease, time.Time) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	var endedAt time.Time
	for {
		lease, err := (nativePlatform{}).AcquireCommander(name)
		if err == nil {
			return lease, time.Now()
		}
		if !errors.Is(err, errCommanderRunning) {
			t.Fatalf("取锁失败且不是被占用: %v", err)
		}
		switch {
		case commander.ended():
			if endedAt.IsZero() {
				endedAt = time.Now()
			} else if time.Since(endedAt) > 2*time.Second {
				t.Fatal("直属 Commander 已结束，锁仍被占用")
			}
		case time.Now().After(deadline):
			t.Fatal("启动器被杀 20 秒后直属 Commander 仍未结束，锁也取不到")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestCommanderLaunchFailureStartsNothing(t *testing.T) {
	dir := t.TempDir()
	// 无效句柄：移交锁必然失败，挂起创建的会话由系统收回，从未运行。
	spec := helperConfig{mode: "tool", label: "unstarted", dir: dir}.spec(helperExecutable(t), 0xDEADBEEF)
	if err := (nativePlatform{}).RunAttached(spec, nil, nil, nil); err == nil {
		t.Fatal("移交锁失败却报告成功")
	}
	if waitHelperFile(filepath.Join(dir, "unstarted.pid"), 2*time.Second, nil) {
		t.Fatal("移交锁失败后子进程仍然运行了")
	}
}

func TestCommanderReportsNonzeroExit(t *testing.T) {
	lease, err := (nativePlatform{}).AcquireCommander(commanderMutexName(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	defer lease.release()
	// 未识别的 helper 模式会在启动后主动 exit(2)。
	err = (nativePlatform{}).RunAttached(helperConfig{mode: "unknown", label: "unknown", dir: t.TempDir()}.spec(helperExecutable(t), lease.handle), nil, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "exit status 2") {
		t.Fatalf("Commander 退出码未报告：%v", err)
	}
}

func TestCommanderLockRefusesLegacyHolderWithoutTakeover(t *testing.T) {
	p := nativePlatform{}
	name := commanderMutexName(t.TempDir())
	// 模拟旧入口仍持有原名称的可继承锁。新版不因看不到窗口或子进程而接管。
	legacy, err := acquireNamedMutex(name, errCommanderRunning, true)
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.AcquireCommander(name)
	if !errors.Is(err, errCommanderRunning) {
		t.Fatalf("legacy holder: %v", err)
	}
	legacy.release()
	lease, err := p.AcquireCommander(name)
	if err != nil {
		t.Fatalf("最后一个持有者退出后仍无法获得锁: %v", err)
	}
	lease.release()
}
