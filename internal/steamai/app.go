package steamai

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/shuiyu486/STeamAI/internal/steamai/casebootstrap"
	"github.com/shuiyu486/STeamAI/internal/steamai/learningbatch"
)

const canonicalModule = "github.com/shuiyu486/STeamAI"

var (
	errUnsupportedPlatform      = errors.New("STeamAI 原生入口当前只支持 Windows 10/11 x64")
	errPartialCase              = errors.New("当前目录包含不完整或冲突的 STeamAI 状态")
	errCommanderRunning         = errors.New("当前 case 已有 Commander 正在运行，请切换到原窗口")
	errCanonicalMutationRunning = errors.New("canonical checkout 正在被另一个 learning apply 或 update 修改；请稍后重试")
	memberNamePattern           = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
	gitIdentityPattern          = regexp.MustCompile(`^[0-9a-f]{40,64}$`)
	sha256Pattern               = regexp.MustCompile(`^[0-9a-f]{64}$`)
	windowsReservedNames        = map[string]struct{}{
		"con": {}, "prn": {}, "aux": {}, "nul": {},
		"com1": {}, "com2": {}, "com3": {}, "com4": {}, "com5": {}, "com6": {}, "com7": {}, "com8": {}, "com9": {},
		"lpt1": {}, "lpt2": {}, "lpt3": {}, "lpt4": {}, "lpt5": {}, "lpt6": {}, "lpt7": {}, "lpt8": {}, "lpt9": {},
	}
)

const memberInitialPrompt = "读取当前目录 CLAUDE.md 与父级 case 规则，开始执行当前正式任务。"

type caseState uint8

const (
	caseFresh caseState = iota
	caseCurrent
	casePartial
)

type processSpec struct {
	Path             string
	Args             []string
	Dir              string
	Env              []string
	InheritedHandles []uintptr
}

type commanderLease struct {
	handle  uintptr
	release func()
}

type updateInstall struct {
	Source               string
	StagedSource         string
	ReplaceSource        bool
	Executable           string
	ExecutableSHA256     string
	Version              string
	ExpectedHead         string
	ExpectedStatus       string
	ExpectedRefs         string
	ExpectedStagedHead   string
	ExpectedStagedStatus string
	ExpectedStagedRefs   string
}

type updateResult struct {
	CleanupPath          string
	PreserveStagedSource bool
}

type uninstallResult struct {
	Source          string
	CleanupDeferred bool
	CleanupHelper   string
}

type platform interface {
	Supported() bool
	CanonicalSource() (string, error)
	ActiveExecutable() (string, error)
	Install(executable, source, version string) error
	ActivateUpdate(updateInstall) (updateResult, error)
	Uninstall(currentExecutable string) (uninstallResult, error)
	CaseIdentity(path string) (string, error)
	AcquireCommander(name string) (commanderLease, error)
	AcquireCanonicalMutation(name string) (commanderLease, error)
	RunAttached(processSpec, io.Reader, io.Writer, io.Writer) error
	OpenVisible(processSpec) error
}

type app struct {
	platform       platform
	stdin          io.Reader
	stdout         io.Writer
	stderr         io.Writer
	version        string
	cwd            func() (string, error)
	executable     func() (string, error)
	lookPath       func(string) (string, error)
	validateSource func(string) error
}

func Main(args []string, stdin io.Reader, stdout, stderr io.Writer, version string) int {
	a := newApp(nativePlatform{}, stdin, stdout, stderr, version)
	if err := a.run(args); err != nil {
		fmt.Fprintln(stderr, "steamai:", err)
		return 1
	}
	return 0
}

func newApp(p platform, stdin io.Reader, stdout, stderr io.Writer, version string) *app {
	a := &app{
		platform:   p,
		stdin:      stdin,
		stdout:     stdout,
		stderr:     stderr,
		version:    version,
		cwd:        os.Getwd,
		executable: os.Executable,
		lookPath:   exec.LookPath,
	}
	a.validateSource = func(source string) error {
		if err := validateCanonicalSource(source); err != nil {
			return err
		}
		git, err := a.resolveNativeExecutable("git.exe")
		if err != nil {
			return fmt.Errorf("找不到原生 Git: %w", err)
		}
		return validateCanonicalGitSource(git, source)
	}
	return a
}

func (a *app) run(args []string) error {
	if !a.platform.Supported() {
		return errUnsupportedPlatform
	}
	if len(args) == 0 {
		return a.openCommander()
	}
	switch args[0] {
	case "help", "--help", "-h":
		fmt.Fprint(a.stdout, usageText)
		return nil
	case "version", "--version", "-v":
		fmt.Fprintln(a.stdout, a.version)
		return nil
	case "setup":
		return a.setup(args[1:])
	case "update":
		return a.update(args[1:])
	case "uninstall":
		return a.uninstall(args[1:])
	case "__uninstall-cleanup":
		if len(args) < 5 {
			return errors.New("内部 uninstall cleanup 参数无效")
		}
		return runUninstallCleanup(args[1:])
	case "__fresh-preview":
		if len(args) != 1 {
			return errors.New("内部 Fresh preview 参数无效")
		}
		return a.freshPreview()
	case "__fresh-apply":
		return a.freshApply(args[1:])
	case "__learning-batch-preview":
		if len(args) != 1 {
			return errors.New("内部 learning batch preview 参数无效")
		}
		return a.learningBatchPreview()
	case "__learning-batch-apply":
		return a.learningBatchApply(args[1:])
	case "__evaluation-suite-prepare":
		if len(args) != 1 {
			return errors.New("内部 evaluation suite prepare 参数无效")
		}
		return a.evaluationSuitePrepare()
	case "__evaluation-run":
		if len(args) != 1 {
			return errors.New("内部 evaluation run 参数无效")
		}
		return a.evaluationRun()
	case "__evaluation-suite-finalize":
		if len(args) != 1 {
			return errors.New("内部 evaluation suite finalize 参数无效")
		}
		return a.evaluationSuiteFinalize()
	case "__open-member":
		if len(args) != 2 {
			return errors.New("内部成员启动参数无效")
		}
		return a.openMember(args[1])
	default:
		return fmt.Errorf("未知命令 %q；运行 steamai --help 查看支持的入口", args[0])
	}
}

const usageText = `STeamAI Windows 原生入口

用法：
  steamai                         创建或继续当前 case
  steamai setup [--source PATH]   安装并绑定 canonical STeamAI checkout
  steamai update                  更新到最新正式版 exe 与 canonical checkout
  steamai uninstall               卸载入口并保留 checkout 与 case
  steamai --version               显示版本
`

func (a *app) freshPreview() error {
	git, source, caseRoot, facts, err := a.freshInputs()
	if err != nil {
		return err
	}
	preview, err := casebootstrap.BuildPreview(git, source, caseRoot, facts)
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(a.stdout, preview.HumanPreview)
	return err
}

func (a *app) freshApply(args []string) error {
	if len(args) != 2 || args[0] != "--confirmation" || strings.TrimSpace(args[1]) == "" {
		return errors.New("内部 Fresh apply 参数无效")
	}
	git, source, caseRoot, facts, err := a.freshInputs()
	if err != nil {
		return err
	}
	preview, err := casebootstrap.Apply(git, source, caseRoot, facts, args[1])
	if err != nil {
		return err
	}
	fmt.Fprintf(a.stdout, "STeamAI case 已创建：%s\n", preview.Identity)
	return nil
}

func (a *app) learningBatchPreview() error {
	git, source, caseRoot, request, err := a.learningBatchInputs()
	if err != nil {
		return err
	}
	preview, err := learningbatch.BuildPreview(git, source, caseRoot, request)
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(a.stdout, preview.HumanPreview)
	return err
}

func (a *app) learningBatchApply(args []string) error {
	if len(args) != 2 || args[0] != "--confirmation" || strings.TrimSpace(args[1]) == "" {
		return errors.New("内部 learning batch apply 参数无效")
	}
	git, source, caseRoot, request, err := a.learningBatchInputs()
	if err != nil {
		return err
	}
	lease, err := a.acquireCanonicalMutation(source)
	if err != nil {
		return err
	}
	defer lease.release()
	preview, err := learningbatch.Apply(git, source, caseRoot, request, args[1])
	if err != nil {
		return err
	}
	fmt.Fprintf(a.stdout, "STeamAI learning batch 已应用到本机 canonical working tree：%s\n", preview.Identity)
	return nil
}

func (a *app) learningBatchInputs() (git, source, caseRoot string, request learningbatch.Request, err error) {
	caseRoot, err = a.cwd()
	if err != nil {
		return "", "", "", request, err
	}
	caseRoot, err = filepath.Abs(caseRoot)
	if err != nil {
		return "", "", "", request, err
	}
	state, err := inspectCase(caseRoot)
	if err != nil {
		return "", "", "", request, err
	}
	if state != caseCurrent {
		return "", "", "", request, errPartialCase
	}
	source, err = a.platform.CanonicalSource()
	if err != nil {
		return "", "", "", request, fmt.Errorf("尚未完成 STeamAI setup: %w", err)
	}
	if err = a.validateSource(source); err != nil {
		return "", "", "", request, err
	}
	git, err = a.resolveNativeExecutable("git.exe")
	if err != nil {
		return "", "", "", request, fmt.Errorf("找不到原生 Git: %w", err)
	}
	request, err = learningbatch.DecodeRequest(a.stdin)
	return git, source, caseRoot, request, err
}

func (a *app) freshInputs() (git, source, caseRoot string, facts casebootstrap.Facts, err error) {
	caseRoot, err = a.cwd()
	if err != nil {
		return "", "", "", facts, err
	}
	caseRoot, err = filepath.Abs(caseRoot)
	if err != nil {
		return "", "", "", facts, err
	}
	state, err := inspectCase(caseRoot)
	if err != nil {
		return "", "", "", facts, err
	}
	if state != caseFresh {
		return "", "", "", facts, errPartialCase
	}
	source, err = a.platform.CanonicalSource()
	if err != nil {
		return "", "", "", facts, fmt.Errorf("尚未完成 STeamAI setup: %w", err)
	}
	if err = a.validateSource(source); err != nil {
		return "", "", "", facts, err
	}
	if err = rejectCaseInsideSource(caseRoot, source); err != nil {
		return "", "", "", facts, err
	}
	git, err = a.resolveNativeExecutable("git.exe")
	if err != nil {
		return "", "", "", facts, fmt.Errorf("找不到原生 Git: %w", err)
	}
	facts, err = casebootstrap.DecodeFacts(a.stdin)
	return git, source, caseRoot, facts, err
}

func (a *app) openCommander() error {
	caseRoot, err := a.cwd()
	if err != nil {
		return fmt.Errorf("读取当前目录: %w", err)
	}
	caseRoot, err = filepath.Abs(caseRoot)
	if err != nil {
		return fmt.Errorf("规范化当前目录: %w", err)
	}
	state, err := inspectCase(caseRoot)
	if err != nil {
		return err
	}
	if state == casePartial {
		return errPartialCase
	}
	claude, err := a.resolveNativeExecutable("claude.exe")
	if err != nil {
		return fmt.Errorf("找不到原生 Claude Code: %w", err)
	}
	args := []string{"/steamai"}
	if state == caseFresh {
		source, err := a.platform.CanonicalSource()
		if err != nil {
			return fmt.Errorf("尚未完成 STeamAI setup: %w", err)
		}
		if err := a.validateSource(source); err != nil {
			return err
		}
		args = append(args, "--add-dir", source)
	}
	identity, err := a.platform.CaseIdentity(caseRoot)
	if err != nil {
		return fmt.Errorf("读取 case 物理身份: %w", err)
	}
	lease, err := a.platform.AcquireCommander(commanderMutexName(identity))
	if err != nil {
		if errors.Is(err, errCommanderRunning) {
			return err
		}
		return fmt.Errorf("建立 Commander 单实例保护: %w", err)
	}
	defer lease.release()
	return a.platform.RunAttached(processSpec{
		Path:             claude,
		Args:             args,
		Dir:              caseRoot,
		Env:              withoutEnvironment(os.Environ(), "CLAUDECODE"),
		InheritedHandles: []uintptr{lease.handle},
	}, a.stdin, a.stdout, a.stderr)
}

func (a *app) openMember(name string) error {
	if !memberNamePattern.MatchString(name) {
		return errors.New("成员名称无效")
	}
	if _, reserved := windowsReservedNames[strings.ToLower(name)]; reserved {
		return errors.New("成员名称是 Windows 保留名称")
	}
	caseRoot, err := a.cwd()
	if err != nil {
		return err
	}
	caseRoot, err = filepath.Abs(caseRoot)
	if err != nil {
		return err
	}
	state, err := inspectCase(caseRoot)
	if err != nil {
		return err
	}
	if state != caseCurrent {
		return errors.New("只能从完整 current case 启动成员")
	}
	memberRoot := filepath.Join(caseRoot, ".steamai-vnext", "members", name)
	if !pathWithin(memberRoot, filepath.Join(caseRoot, ".steamai-vnext", "members")) {
		return errors.New("成员路径越出 current case")
	}
	if err := requirePlainPath(caseRoot, memberRoot, true); err != nil {
		return fmt.Errorf("成员目录无效: %w", err)
	}
	if err := requirePlainPath(caseRoot, filepath.Join(memberRoot, "CLAUDE.md"), false); err != nil {
		return fmt.Errorf("成员身份文件无效: %w", err)
	}
	claude, err := a.resolveNativeExecutable("claude.exe")
	if err != nil {
		return fmt.Errorf("找不到原生 Claude Code: %w", err)
	}
	return a.platform.OpenVisible(processSpec{
		Path: claude,
		Args: []string{memberInitialPrompt, "--add-dir", caseRoot},
		Dir:  memberRoot,
		Env:  withoutEnvironment(os.Environ(), "CLAUDECODE"),
	})
}

func rejectCaseInsideSource(caseRoot, source string) error {
	caseRoot, err := filepath.Abs(caseRoot)
	if err != nil {
		return err
	}
	source, err = filepath.Abs(source)
	if err != nil {
		return err
	}
	caseInfo, err := os.Stat(caseRoot)
	if err != nil {
		return err
	}
	sourceInfo, err := os.Stat(source)
	if err != nil {
		return err
	}
	if os.SameFile(caseInfo, sourceInfo) {
		return errors.New("canonical source 本身不能作为安全研究 case")
	}
	current := caseRoot
	for {
		info, statErr := os.Stat(current)
		if statErr != nil {
			return statErr
		}
		if os.SameFile(info, sourceInfo) {
			return errors.New("安全研究 case 不能位于 canonical source 内部")
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return nil
}

func inspectCase(root string) (caseState, error) {
	if err := requirePlainDirectory(root); err != nil {
		return casePartial, fmt.Errorf("case 根目录无效: %w", err)
	}
	stateRoot := filepath.Join(root, ".steamai-vnext")
	stateExists, stateErr := pathExists(stateRoot)
	if stateErr != nil {
		return casePartial, stateErr
	}
	if !stateExists {
		// An exact project-local skill may remain if state publication failed
		// after its no-replace publish. Fresh bootstrap will compare it against
		// the current canonical bytes and treat only an exact match as unchanged.
		return caseFresh, nil
	}
	if err := casebootstrap.ValidateCurrent(root); err != nil {
		return casePartial, nil
	}
	return caseCurrent, nil
}

func pathExists(path string) (bool, error) {
	_, err := os.Lstat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func validateCanonicalSource(source string) error {
	source, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	if err := requirePlainDirectory(source); err != nil {
		return fmt.Errorf("canonical source 目录无效 %s: %w", source, err)
	}
	for _, path := range []string{
		filepath.Join(source, ".claude", "skills", "steamai"),
	} {
		if err := requirePlainDirectory(path); err != nil {
			return fmt.Errorf("canonical source 目录无效 %s: %w", path, err)
		}
	}
	for _, path := range []string{
		filepath.Join(source, ".git"),
		filepath.Join(source, ".claude", "skills", "steamai", "SKILL.md"),
		filepath.Join(source, "go.mod"),
	} {
		if err := requirePlainFileOrDirectory(path); err != nil {
			return fmt.Errorf("canonical source 文件无效 %s: %w", path, err)
		}
	}
	module, err := os.ReadFile(filepath.Join(source, "go.mod"))
	if err != nil {
		return err
	}
	if !strings.HasPrefix(string(module), "module "+canonicalModule+"\n") {
		return errors.New("canonical source identity 不匹配")
	}
	if exists, err := pathExists(filepath.Join(source, ".steamai-vnext")); err != nil || exists {
		return errors.New("canonical source 不能同时是一个 STeamAI case")
	}
	return nil
}

func validateCanonicalGitSource(git, source string) error {
	cmd := exec.Command(git, "rev-parse", "--show-toplevel")
	cmd.Dir = source
	cmd.Env = withoutEnvironment(os.Environ(), "CLAUDECODE")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("读取 canonical Git root: %w", err)
	}
	top, err := filepath.Abs(strings.TrimSpace(string(output)))
	if err != nil {
		return err
	}
	source, err = filepath.Abs(source)
	if err != nil {
		return err
	}
	sourceInfo, err := os.Stat(source)
	if err != nil {
		return err
	}
	topInfo, err := os.Stat(top)
	if err != nil {
		return err
	}
	if !os.SameFile(sourceInfo, topInfo) {
		return errors.New("canonical source 不是 Git worktree 根目录")
	}
	if err := runGitCheck(git, source, "rev-parse", "--verify", "HEAD^{commit}"); err != nil {
		return errors.New("canonical source 没有有效 HEAD commit")
	}
	skillRel := ".claude/skills/steamai/SKILL.md"
	if err := runGitCheck(git, source, "ls-files", "--error-unmatch", "--stage", "--", skillRel); err != nil {
		return errors.New("canonical skill 不是 tracked stage-0 文件")
	}
	cmd = exec.Command(git, "ls-files", "--unmerged", "--", skillRel)
	cmd.Dir = source
	cmd.Env = withoutEnvironment(os.Environ(), "CLAUDECODE")
	unmerged, err := cmd.Output()
	if err != nil || len(bytes.TrimSpace(unmerged)) != 0 {
		return errors.New("canonical skill 存在 unmerged index 状态")
	}
	return nil
}

func runGitCheck(git, dir string, args ...string) error {
	cmd := exec.Command(git, args...)
	cmd.Dir = dir
	cmd.Env = withoutEnvironment(os.Environ(), "CLAUDECODE")
	return cmd.Run()
}

func requirePlainPath(root, path string, directory bool) error {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	if !pathWithin(path, root) {
		return errors.New("路径越出允许范围")
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}
	current := root
	if err := requirePlainDirectory(current); err != nil {
		return err
	}
	parts := strings.Split(rel, string(filepath.Separator))
	for index, part := range parts {
		if part == "." || part == "" {
			continue
		}
		current = filepath.Join(current, part)
		last := index == len(parts)-1
		if last && !directory {
			if err := requirePlainFile(current); err != nil {
				return err
			}
			continue
		}
		if err := requirePlainDirectory(current); err != nil {
			return err
		}
	}
	return nil
}

func requirePlainFileOrDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || (!info.IsDir() && !info.Mode().IsRegular()) {
		return errors.New("不是普通文件或目录")
	}
	return rejectReparse(path)
}

func requirePlainDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("不是普通目录")
	}
	return rejectReparse(path)
}

func requirePlainFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return errors.New("不是普通文件")
	}
	return rejectReparse(path)
}

func pathWithin(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func commanderMutexName(caseIdentity string) string {
	sum := sha256.Sum256([]byte(caseIdentity))
	return `Local\STeamAI.Commander.` + hex.EncodeToString(sum[:16])
}

func canonicalMutationMutexName(source string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(source))))
	return `Local\STeamAI.CanonicalMutation.` + hex.EncodeToString(sum[:16])
}

func (a *app) acquireCanonicalMutation(source string) (commanderLease, error) {
	lease, err := a.platform.AcquireCanonicalMutation(canonicalMutationMutexName(source))
	if err != nil {
		if errors.Is(err, errCanonicalMutationRunning) {
			return commanderLease{}, err
		}
		return commanderLease{}, fmt.Errorf("建立 canonical mutation 互斥: %w", err)
	}
	return lease, nil
}

func withoutEnvironment(env []string, names ...string) []string {
	blocked := make(map[string]struct{}, len(names))
	for _, name := range names {
		blocked[strings.ToUpper(name)] = struct{}{}
	}
	out := make([]string, 0, len(env))
	for _, item := range env {
		key, _, ok := strings.Cut(item, "=")
		if ok {
			if _, found := blocked[strings.ToUpper(key)]; found {
				continue
			}
		}
		out = append(out, item)
	}
	return out
}

func (a *app) resolveNativeExecutable(name string) (string, error) {
	path, err := a.lookPath(name)
	if err != nil {
		return "", err
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if err := validateNativeExecutablePath(path); err != nil {
		return "", err
	}
	return path, nil
}

func validateNativeExecutablePath(path string) error {
	if !strings.EqualFold(filepath.Ext(path), ".exe") {
		return errors.New("拒绝脚本或非原生可执行入口")
	}
	return requirePlainFile(path)
}
