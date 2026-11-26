//go:build windows

package steamai

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// WezTerm 成员窗口的窄职责实现：当前进程带 WezTerm 痕迹时接管 OpenVisible，
// 成员优先开当前窗口的新标签；没有当前 pane 但检测到 WezTerm 时开一次明确的
// 新 WezTerm 窗口；两种情况失败都以可执行诊断如实报错，不回退其它 terminal，
// 不重复重试，也不向既有 pane 发送任何输入。完全没有 WezTerm 痕迹时，若
// PATH 上能找到 wezterm.exe，也用独立新窗口打开成员；否则保留原生 console。
//
// 行为依据（针对本机安装的 wezterm 20240203-110809-5046fc22，--help 实测与
// 同 tag 源码核对）：
//   - `wezterm cli spawn [PROG]...` 的 PROG 经 CommandBuilder::from_argv 以
//     argv 精确传递（wezterm/src/cli/spawn_command.rs），成功输出新 pane id，
//     不等待成员程序退出；
//   - `--pane-id`（默认取 WEZTERM_PANE 所在窗口）开新标签，`--new-window`
//     开新窗口，`--cwd` 指定工作目录；
//   - `--no-auto-start` 阻止在找不到 server 时自动拉起新实例（本机 --help 确认），
//     保证不会无声多开 terminal；
//   - pane 进程环境来自 mux server 进程环境叠加注册表（HKLM+HKCU）覆写
//     （pty/src/cmdbuilder.rs 的 get_base_env），与 steamai.exe 自身环境无关，
//     且 CLI 没有任何环境注入通道；成员会话的环境清理由 spec 指向的
//     `steamai __member-session` helper 在 tab 内自行完成。

const (
	weztermPaneEnv          = "WEZTERM_PANE"
	weztermTermProgramEnv   = "TERM_PROGRAM"
	weztermTermProgramValue = "WezTerm"
	weztermExecutableEnv    = "WEZTERM_EXECUTABLE"
	weztermExecutableDirEnv = "WEZTERM_EXECUTABLE_DIR"
	weztermSocketEnv        = "WEZTERM_UNIX_SOCKET"
	weztermCLIName          = "wezterm.exe"
	weztermCLITimeout       = 15 * time.Second
	weztermStderrLimit      = 400
)

// runWeztermCLI 执行一次 wezterm CLI 调用并返回 stdout/stderr；测试注入替换。
var runWeztermCLI = func(cliPath string, args []string, env []string) (string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), weztermCLITimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, cliPath, args...)
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createUnicodeEnv}
	runErr := cmd.Run()
	if ctx.Err() != nil {
		return stdout.String(), stderr.String(), fmt.Errorf("wezterm CLI 超时（上限 %s）: %w", weztermCLITimeout, ctx.Err())
	}
	return stdout.String(), stderr.String(), runErr
}

// openVisibleWezTerm 在检测到 WezTerm 会话时接管成员可见启动。
// 返回 attempted=false 表示没有 WezTerm 痕迹，调用方保留原生 console 路径；
// 返回 attempted=true 时 err 为 nil 表示新标签/新窗口已创建，否则为可执行诊断。
func openVisibleWezTerm(spec processSpec) (bool, error) {
	pane, present := weztermDetect()
	if !present {
		if os.Getenv(memberWindowAddressEnv) == memberWindowWeztermMux {
			return true, fmt.Errorf("当前会话已无法定位原 WezTerm 窗口，未创建成员；不会回退到受此会话控制的窗口")
		}
		return startVisibleWeztermWindow(spec)
	}
	cliPath, err := resolveWeztermCLI(pane)
	if err != nil {
		return true, err
	}
	args := []string{"cli", "--no-auto-start", "spawn"}
	label := "新 WezTerm 窗口"
	if pane != "" {
		args = append(args, "--pane-id", pane)
		label = "当前窗口新标签"
	} else {
		args = append(args, "--new-window")
	}
	args = append(args, "--cwd", spec.Dir, "--", spec.Path)
	args = append(args, spec.Args...)
	_, stderr, runErr := runWeztermCLI(cliPath, args, spec.Env)
	if runErr != nil {
		return true, weztermDiagnosticError(cliPath, pane, weztermAttempt{label: label, err: runErr, stderr: stderr})
	}
	return true, nil
}

// startVisibleWeztermWindow 处理无 WezTerm 痕迹的情形：PATH 上找得到 wezterm.exe
// 就用一次独立的 WezTerm 新窗口打开成员，找不到返回 attempted=false 保留原生 console。
// --always-new-process 的语义是"本进程等待命令完成"（本机 --help 实测），因此只
// Start 不 Wait，避免阻塞到成员窗口关闭。
func startVisibleWeztermWindow(spec processSpec) (bool, error) {
	weztermPath, err := exec.LookPath(weztermCLIName)
	if err != nil {
		return false, nil
	}
	args := append([]string{"start", "--always-new-process", "--cwd", spec.Dir, "--", spec.Path}, spec.Args...)
	if err := startWeztermWindow(weztermPath, args, spec.Env); err != nil {
		return true, fmt.Errorf("PATH 定位到 WezTerm（%s）但成员新窗口进程启动失败: %w；按合同不回退其它 terminal", weztermPath, err)
	}
	return true, nil
}

// 独立 WezTerm 新窗口由自己的 GUI 承载，不依附调用者的控制台或会话 Job。
var startWeztermWindow = func(weztermPath string, args []string, env []string) error {
	return startIndependentMember(processSpec{Path: weztermPath, Args: args, Env: env}, createNoWindow)
}

// 会话与成员窗口沿用同一终端选择规则；显式传入的会话环境不一定等于启动器环境。
func sessionUsesWeztermMux(env []string) bool {
	if env == nil {
		_, present := weztermDetect()
		return present
	}
	var pane, program string
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		switch {
		case strings.EqualFold(key, weztermPaneEnv):
			pane = value
		case strings.EqualFold(key, weztermTermProgramEnv):
			program = value
		}
	}
	_, present := weztermContext(pane, program)
	return present
}

// weztermDetect 读取（只读，不修改）当前进程环境中的 WezTerm 痕迹：
// WEZTERM_PANE 存在表示当前就在某个 pane 内；否则 TERM_PROGRAM=WezTerm
// 表示源自 WezTerm 但没有可定位的当前 pane。
func weztermDetect() (pane string, present bool) {
	return weztermContext(os.Getenv(weztermPaneEnv), os.Getenv(weztermTermProgramEnv))
}

func weztermContext(pane, program string) (string, bool) {
	pane = strings.TrimSpace(pane)
	return pane, pane != "" || program == weztermTermProgramValue
}

// resolveWeztermCLI 定位 wezterm CLI 可执行文件：优先当前 gui 实例同目录
// （与运行中的 server 同版本），其次 WEZTERM_EXECUTABLE_DIR，最后 PATH。
func resolveWeztermCLI(pane string) (string, error) {
	var candidates []string
	if exe := os.Getenv(weztermExecutableEnv); exe != "" {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), weztermCLIName))
	}
	if dir := os.Getenv(weztermExecutableDirEnv); dir != "" {
		candidates = append(candidates, filepath.Join(dir, weztermCLIName))
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() {
			return candidate, nil
		}
	}
	if found, err := exec.LookPath(weztermCLIName); err == nil {
		return found, nil
	}
	return "", fmt.Errorf("检测到 WezTerm 会话（%s）但找不到可用的 %s（已检查 WEZTERM_EXECUTABLE 同目录、WEZTERM_EXECUTABLE_DIR 与 PATH）；不回退其它 terminal，请确认 WezTerm 安装完整后重试",
		weztermDetectionDetail(pane), weztermCLIName)
}

type weztermAttempt struct {
	label  string
	err    error
	stderr string
}

func weztermDiagnosticError(cliPath, pane string, attempts ...weztermAttempt) error {
	var detail strings.Builder
	for _, attempt := range attempts {
		fmt.Fprintf(&detail, "；%s: %v", attempt.label, attempt.err)
		if trimmed := weztermOneLine(attempt.stderr); trimmed != "" {
			detail.WriteString("，stderr: ")
			detail.WriteString(trimmed)
		}
	}
	return fmt.Errorf("检测到 WezTerm 会话（%s，CLI=%s）但成员窗口创建失败%s；按合同不回退其它 terminal、不重复重试，请依据上述错误处理 WezTerm 后重试",
		weztermDetectionDetail(pane), cliPath, detail.String())
}

func weztermDetectionDetail(pane string) string {
	socket := os.Getenv(weztermSocketEnv)
	if socket == "" {
		socket = "<未设置>"
	}
	if pane != "" {
		return fmt.Sprintf("WEZTERM_PANE=%s，WEZTERM_UNIX_SOCKET=%s", pane, socket)
	}
	return fmt.Sprintf("TERM_PROGRAM=WezTerm（无当前 pane），WEZTERM_UNIX_SOCKET=%s", socket)
}

func weztermOneLine(text string) string {
	text = strings.ReplaceAll(text, "\r\n", " ")
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\r", " ")
	text = strings.TrimSpace(text)
	if len([]rune(text)) <= weztermStderrLimit {
		return text
	}
	runes := []rune(text)
	return string(runes[:weztermStderrLimit]) + "…"
}
