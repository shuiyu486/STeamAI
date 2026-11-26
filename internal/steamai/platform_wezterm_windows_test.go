//go:build windows

package steamai

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

type weztermCLICall struct {
	cliPath string
	args    []string
	env     []string
}

func installWeztermCLIFake(t *testing.T, respond func(index int, call weztermCLICall) (string, string, error)) *[]weztermCLICall {
	t.Helper()
	calls := &[]weztermCLICall{}
	original := runWeztermCLI
	runWeztermCLI = func(cliPath string, args []string, env []string) (string, string, error) {
		call := weztermCLICall{cliPath: cliPath, args: append([]string(nil), args...), env: append([]string(nil), env...)}
		*calls = append(*calls, call)
		return respond(len(*calls)-1, call)
	}
	t.Cleanup(func() { runWeztermCLI = original })
	return calls
}

// genuineExitError 用真实子进程产生 *exec.ExitError，避免伪造结构体导致
// 诊断格式化时 nil ProcessState 崩溃。
func genuineExitError(t *testing.T, code int) error {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestWeztermHelperProcess$", "--", strconv.Itoa(code))
	cmd.Env = append(os.Environ(), "GO_WANT_WEZTERM_HELPER_PROCESS=1")
	err := cmd.Run()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("helper process did not produce exit error: %v", err)
	}
	return err
}

func TestWeztermHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_WEZTERM_HELPER_PROCESS") != "1" {
		return
	}
	code, err := strconv.Atoi(os.Args[len(os.Args)-1])
	if err != nil {
		code = 3
	}
	os.Exit(code)
}

func clearWeztermEnvironment(t *testing.T) string {
	t.Helper()
	cliDir := t.TempDir()
	for _, name := range []string{weztermPaneEnv, weztermTermProgramEnv, weztermExecutableEnv, weztermExecutableDirEnv, memberWindowAddressEnv} {
		t.Setenv(name, "")
	}
	t.Setenv("PATH", cliDir)
	return cliDir
}

func installRunningWeztermCLI(t *testing.T) string {
	t.Helper()
	cliDir := t.TempDir()
	cliPath := filepath.Join(cliDir, weztermCLIName)
	if err := os.WriteFile(cliPath, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(weztermExecutableEnv, filepath.Join(cliDir, "wezterm-gui.exe"))
	t.Setenv(weztermExecutableDirEnv, "")
	t.Setenv("PATH", filepath.Join(t.TempDir(), "empty"))
	return cliPath
}

func weztermMemberSpec() processSpec {
	return processSpec{
		Path: `C:\Users\demo\AppData\Local\STeamAI\bin\steamai.exe`,
		Args: []string{"__member-session", "--config-dir", `C:\Users\demo\.claude-custom`, "--tier", "standard", "--high", "sonnet", "--standard", "haiku", "alice"},
		Dir:  `C:\AI\cases\demo`,
		Env:  []string{"PATH=C:\\Windows", "WEZTERM_PANE=7", "WEZTERM_UNIX_SOCKET=" + `C:\Users\demo\.local\share\wezterm\gui-sock-23052`},
	}
}

func TestOpenVisibleWezTermSkipsWithoutWeztermTrace(t *testing.T) {
	clearWeztermEnvironment(t)
	calls := installWeztermCLIFake(t, func(int, weztermCLICall) (string, string, error) {
		t.Error("未检测到 WezTerm 时不应调用 CLI")
		return "", "", nil
	})
	attempted, err := openVisibleWezTerm(weztermMemberSpec())
	if attempted || err != nil {
		t.Fatalf("attempted=%v err=%v，期望保留原生 console 路径", attempted, err)
	}
	if len(*calls) != 0 {
		t.Fatalf("意外调用 CLI: %v", *calls)
	}
}

func TestWeztermMuxSessionWithLostPaneDoesNotStartAnOwnedWindow(t *testing.T) {
	clearWeztermEnvironment(t)
	t.Setenv(memberWindowAddressEnv, memberWindowWeztermMux)
	if handled, err := openVisibleWezTerm(weztermMemberSpec()); !handled || err == nil {
		t.Fatal("已指定由 mux 开成员窗口时，不能回退到会话 Job 内开窗")
	}
}

func TestOpenVisibleWezTermOpensTabInCurrentWindow(t *testing.T) {
	clearWeztermEnvironment(t)
	cliPath := installRunningWeztermCLI(t)
	t.Setenv(weztermPaneEnv, "7")
	calls := installWeztermCLIFake(t, func(int, weztermCLICall) (string, string, error) {
		return "42\n", "", nil
	})
	spec := weztermMemberSpec()
	attempted, err := openVisibleWezTerm(spec)
	if !attempted || err != nil {
		t.Fatalf("attempted=%v err=%v", attempted, err)
	}
	if len(*calls) != 1 {
		t.Fatalf("期望恰好一次 CLI 调用，实际 %d 次", len(*calls))
	}
	call := (*calls)[0]
	wantArgs := append([]string{"cli", "--no-auto-start", "spawn", "--pane-id", "7", "--cwd", spec.Dir, "--", spec.Path}, spec.Args...)
	if !reflect.DeepEqual(call.args, wantArgs) {
		t.Fatalf("spawn 参数不精确:\n got %q\nwant %q", call.args, wantArgs)
	}
	if call.cliPath != cliPath {
		t.Fatalf("CLI 路径 = %q，期望运行中实例同目录的 %q", call.cliPath, cliPath)
	}
	if !reflect.DeepEqual(call.env, spec.Env) {
		t.Fatalf("CLI 子进程环境应按 spec.Env 原样传递:\n got %q\nwant %q", call.env, spec.Env)
	}
}

func TestOpenVisibleWezTermNewWindowWithoutCurrentPane(t *testing.T) {
	clearWeztermEnvironment(t)
	installRunningWeztermCLI(t)
	t.Setenv(weztermTermProgramEnv, weztermTermProgramValue)
	calls := installWeztermCLIFake(t, func(int, weztermCLICall) (string, string, error) {
		return "43\n", "", nil
	})
	spec := weztermMemberSpec()
	attempted, err := openVisibleWezTerm(spec)
	if !attempted || err != nil {
		t.Fatalf("attempted=%v err=%v", attempted, err)
	}
	if len(*calls) != 1 {
		t.Fatalf("期望恰好一次 CLI 调用，实际 %d 次", len(*calls))
	}
	args := (*calls)[0].args
	wantArgs := append([]string{"cli", "--no-auto-start", "spawn", "--new-window", "--cwd", spec.Dir, "--", spec.Path}, spec.Args...)
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("新窗口参数不精确:\n got %q\nwant %q", args, wantArgs)
	}
}

func TestOpenVisibleWezTermReportsFailureWithoutRetry(t *testing.T) {
	clearWeztermEnvironment(t)
	cliPath := installRunningWeztermCLI(t)
	t.Setenv(weztermPaneEnv, "7")
	calls := installWeztermCLIFake(t, func(int, weztermCLICall) (string, string, error) {
		return "", "boom-tab\r\nsecond line", genuineExitError(t, 3)
	})
	attempted, err := openVisibleWezTerm(weztermMemberSpec())
	if !attempted || err == nil {
		t.Fatalf("attempted=%v err=%v，期望如实报错", attempted, err)
	}
	if len(*calls) != 1 {
		t.Fatalf("失败后不得重试或换窗口，实际调用 %d 次", len(*calls))
	}
	message := err.Error()
	for _, fragment := range []string{cliPath, "WEZTERM_PANE=7", "当前窗口新标签", "exit status 3", "boom-tab second line", "不回退其它 terminal"} {
		if !strings.Contains(message, fragment) {
			t.Fatalf("诊断缺少关键信息 %q: %s", fragment, message)
		}
	}
}

func TestOpenVisibleWezTermNewWindowFailureReported(t *testing.T) {
	clearWeztermEnvironment(t)
	installRunningWeztermCLI(t)
	t.Setenv(weztermTermProgramEnv, weztermTermProgramValue)
	calls := installWeztermCLIFake(t, func(int, weztermCLICall) (string, string, error) {
		return "", "boom-window", genuineExitError(t, 4)
	})
	attempted, err := openVisibleWezTerm(weztermMemberSpec())
	if !attempted || err == nil {
		t.Fatalf("attempted=%v err=%v，期望如实报错", attempted, err)
	}
	if len(*calls) != 1 {
		t.Fatalf("期望恰好一次 CLI 调用，实际 %d 次", len(*calls))
	}
	message := err.Error()
	if !strings.Contains(message, "新 WezTerm 窗口") || !strings.Contains(message, "boom-window") {
		t.Fatalf("诊断缺少新窗口失败信息: %s", message)
	}
}

func TestOpenVisibleWezTermTimeoutReported(t *testing.T) {
	clearWeztermEnvironment(t)
	installRunningWeztermCLI(t)
	t.Setenv(weztermPaneEnv, "7")
	calls := installWeztermCLIFake(t, func(int, weztermCLICall) (string, string, error) {
		return "", "", fmt.Errorf("wezterm CLI 超时（上限 %s）: %w", weztermCLITimeout, context.DeadlineExceeded)
	})
	attempted, err := openVisibleWezTerm(weztermMemberSpec())
	if !attempted || err == nil {
		t.Fatalf("attempted=%v err=%v，期望如实报错", attempted, err)
	}
	if len(*calls) != 1 {
		t.Fatalf("超时后不得重试，实际调用 %d 次", len(*calls))
	}
	if !strings.Contains(err.Error(), "超时") {
		t.Fatalf("诊断缺少超时说明: %s", err.Error())
	}
}

func TestOpenVisibleWezTermRequiresCLIWhenDetected(t *testing.T) {
	clearWeztermEnvironment(t)
	t.Setenv(weztermPaneEnv, "7")
	calls := installWeztermCLIFake(t, func(int, weztermCLICall) (string, string, error) {
		t.Error("CLI 不可定位时不应尝试调用")
		return "", "", nil
	})
	attempted, err := openVisibleWezTerm(weztermMemberSpec())
	if !attempted || err == nil {
		t.Fatalf("attempted=%v err=%v，期望报错而非伪装回退", attempted, err)
	}
	if len(*calls) != 0 {
		t.Fatalf("不应调用 CLI: %v", *calls)
	}
	message := err.Error()
	if !strings.Contains(message, weztermCLIName) || !strings.Contains(message, "不回退其它 terminal") {
		t.Fatalf("诊断缺少定位失败信息: %s", message)
	}
}

func TestResolveWeztermCLIPreferences(t *testing.T) {
	t.Run("运行中实例同目录优先", func(t *testing.T) {
		clearWeztermEnvironment(t)
		instanceDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(instanceDir, weztermCLIName), nil, 0o755); err != nil {
			t.Fatal(err)
		}
		pathDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(pathDir, weztermCLIName), nil, 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv(weztermExecutableEnv, filepath.Join(instanceDir, "wezterm-gui.exe"))
		t.Setenv("PATH", pathDir)
		got, err := resolveWeztermCLI("7")
		if err != nil || got != filepath.Join(instanceDir, weztermCLIName) {
			t.Fatalf("got %q err=%v，期望优先运行中实例同目录", got, err)
		}
	})
	t.Run("EXECUTABLE_DIR 次之", func(t *testing.T) {
		clearWeztermEnvironment(t)
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, weztermCLIName), nil, 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv(weztermExecutableDirEnv, dir)
		got, err := resolveWeztermCLI("7")
		if err != nil || got != filepath.Join(dir, weztermCLIName) {
			t.Fatalf("got %q err=%v", got, err)
		}
	})
	t.Run("PATH 兜底", func(t *testing.T) {
		clearWeztermEnvironment(t)
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, weztermCLIName), nil, 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", dir)
		got, err := resolveWeztermCLI("7")
		if err != nil || got != filepath.Join(dir, weztermCLIName) {
			t.Fatalf("got %q err=%v", got, err)
		}
	})
	t.Run("无 pane 时诊断标注 TERM_PROGRAM", func(t *testing.T) {
		clearWeztermEnvironment(t)
		_, err := resolveWeztermCLI("")
		if err == nil {
			t.Fatal("期望定位失败")
		}
		if !strings.Contains(err.Error(), "TERM_PROGRAM=WezTerm") {
			t.Fatalf("诊断缺少检测依据: %s", err.Error())
		}
	})
}

func TestWeztermDetect(t *testing.T) {
	clearWeztermEnvironment(t)
	if pane, present := weztermDetect(); present {
		t.Fatalf("无痕迹时 present=%v pane=%q", present, pane)
	}
	t.Setenv(weztermPaneEnv, " 9 ")
	if pane, present := weztermDetect(); !present || pane != "9" {
		t.Fatalf("pane=%q present=%v，期望去除空白后识别", pane, present)
	}
	t.Setenv(weztermPaneEnv, "")
	t.Setenv(weztermTermProgramEnv, "vscode")
	if _, present := weztermDetect(); present {
		t.Fatal("TERM_PROGRAM 非 WezTerm 不应视为检测到")
	}
	t.Setenv(weztermTermProgramEnv, weztermTermProgramValue)
	if pane, present := weztermDetect(); !present || pane != "" {
		t.Fatalf("pane=%q present=%v，期望 TERM_PROGRAM 兜底检测", pane, present)
	}
}

func TestWeztermOneLineTruncates(t *testing.T) {
	long := strings.Repeat("错", weztermStderrLimit+50)
	got := weztermOneLine("a\r\n" + long)
	if !strings.HasPrefix(got, "a ") || !strings.HasSuffix(got, "…") {
		t.Fatalf("多行与截断处理异常: %q", got)
	}
	if len([]rune(got)) > weztermStderrLimit+10 {
		t.Fatalf("截断后仍过长: %d", len([]rune(got)))
	}
}

func installWeztermStartFake(t *testing.T, respond func(call weztermCLICall) error) *[]weztermCLICall {
	t.Helper()
	calls := &[]weztermCLICall{}
	original := startWeztermWindow
	startWeztermWindow = func(weztermPath string, args []string, env []string) error {
		call := weztermCLICall{cliPath: weztermPath, args: append([]string(nil), args...), env: append([]string(nil), env...)}
		*calls = append(*calls, call)
		return respond(call)
	}
	t.Cleanup(func() { startWeztermWindow = original })
	return calls
}

func putWeztermOnPathOnly(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, weztermCLIName), nil, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{weztermPaneEnv, weztermTermProgramEnv, weztermExecutableEnv, weztermExecutableDirEnv, memberWindowAddressEnv} {
		t.Setenv(name, "")
	}
	t.Setenv("PATH", dir)
	return filepath.Join(dir, weztermCLIName)
}

func TestOpenVisibleWezTermStartsWindowFromPathWithoutTrace(t *testing.T) {
	weztermPath := putWeztermOnPathOnly(t)
	cliCalls := installWeztermCLIFake(t, func(int, weztermCLICall) (string, string, error) {
		t.Error("无 WezTerm 痕迹时不应调用 cli spawn")
		return "", "", nil
	})
	startCalls := installWeztermStartFake(t, func(weztermCLICall) error { return nil })
	spec := weztermMemberSpec()
	attempted, err := openVisibleWezTerm(spec)
	if !attempted || err != nil {
		t.Fatalf("attempted=%v err=%v", attempted, err)
	}
	if len(*cliCalls) != 0 || len(*startCalls) != 1 {
		t.Fatalf("期望恰好一次 start 调用，cli=%d start=%d", len(*cliCalls), len(*startCalls))
	}
	call := (*startCalls)[0]
	wantArgs := append([]string{"start", "--always-new-process", "--cwd", spec.Dir, "--", spec.Path}, spec.Args...)
	if !reflect.DeepEqual(call.args, wantArgs) {
		t.Fatalf("start 参数不精确:\n got %q\nwant %q", call.args, wantArgs)
	}
	if call.cliPath != weztermPath {
		t.Fatalf("start 路径 = %q，期望 PATH 定位的 %q", call.cliPath, weztermPath)
	}
	if !reflect.DeepEqual(call.env, spec.Env) {
		t.Fatalf("start 子进程环境应按 spec.Env 原样传递:\n got %q\nwant %q", call.env, spec.Env)
	}
}

func TestOpenVisibleWezTermStartWindowFailureReported(t *testing.T) {
	weztermPath := putWeztermOnPathOnly(t)
	startCalls := installWeztermStartFake(t, func(weztermCLICall) error {
		return fmt.Errorf("Access is denied")
	})
	attempted, err := openVisibleWezTerm(weztermMemberSpec())
	if !attempted || err == nil {
		t.Fatalf("attempted=%v err=%v，期望如实报错", attempted, err)
	}
	if len(*startCalls) != 1 {
		t.Fatalf("期望恰好一次 start 调用，实际 %d 次", len(*startCalls))
	}
	message := err.Error()
	if !strings.Contains(message, weztermPath) || !strings.Contains(message, "不回退其它 terminal") || !strings.Contains(message, "Access is denied") {
		t.Fatalf("诊断缺少关键信息: %s", message)
	}
}

func TestOpenVisibleWezTermConsoleWhenWeztermNotOnPath(t *testing.T) {
	clearWeztermEnvironment(t)
	cliCalls := installWeztermCLIFake(t, func(int, weztermCLICall) (string, string, error) {
		t.Error("PATH 无 WezTerm 时不应调用 CLI")
		return "", "", nil
	})
	startCalls := installWeztermStartFake(t, func(weztermCLICall) error {
		t.Error("PATH 无 WezTerm 时不应启动新窗口进程")
		return nil
	})
	attempted, err := openVisibleWezTerm(weztermMemberSpec())
	if attempted || err != nil {
		t.Fatalf("attempted=%v err=%v，期望保留原生 console 路径", attempted, err)
	}
	if len(*cliCalls) != 0 || len(*startCalls) != 0 {
		t.Fatalf("不应有任何 WezTerm 调用: cli=%d start=%d", len(*cliCalls), len(*startCalls))
	}
}

func TestNativeOpenVisibleDelegatesToWezterm(t *testing.T) {
	clearWeztermEnvironment(t)
	installRunningWeztermCLI(t)
	t.Setenv(weztermPaneEnv, "7")
	calls := installWeztermCLIFake(t, func(int, weztermCLICall) (string, string, error) {
		return "44\n", "", nil
	})
	if err := (nativePlatform{}).OpenVisible(weztermMemberSpec()); err != nil {
		t.Fatalf("OpenVisible 应委托 WezTerm 路径: %v", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("期望恰好一次 CLI 调用，实际 %d 次", len(*calls))
	}
}
