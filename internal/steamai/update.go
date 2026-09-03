package steamai

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

const (
	defaultReleaseLatestURL = "https://github.com/shuiyu486/STeamAI/releases/latest/download"
	updateDownloadTimeout   = 2 * time.Minute
)

var updateHTTPClient = &http.Client{Timeout: updateDownloadTimeout}

func (a *app) update(args []string) error {
	if len(args) != 0 {
		return errors.New("update 不接受参数")
	}
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		return errors.New("STeamAI v1 update 仅支持 Windows x64")
	}
	active, err := a.platform.ActiveExecutable()
	if err != nil {
		return fmt.Errorf("定位已安装的 steamai.exe: %w", err)
	}
	current, err := a.executable()
	if err != nil {
		return err
	}
	if !sameExecutablePath(active, current) {
		return errors.New("update 必须由 setup 安装的 steamai.exe 执行")
	}
	source, err := a.platform.CanonicalSource()
	if err != nil {
		return fmt.Errorf("尚未完成 STeamAI setup: %w", err)
	}
	if err := a.validateSource(source); err != nil {
		return err
	}
	git, err := a.resolveNativeExecutable("git.exe")
	if err != nil {
		return fmt.Errorf("找不到原生 Git: %w", err)
	}
	baseline, err := captureCanonicalUpdateState(git, source)
	if err != nil {
		return err
	}
	manifestData, err := downloadExact(defaultReleaseLatestURL+"/steamai-release.json", 1<<20)
	if err != nil {
		return err
	}
	manifest, err := parseLatestReleaseManifest(manifestData)
	if err != nil {
		return err
	}
	executableData, err := downloadExact(defaultReleaseLatestURL+"/steamai-windows-amd64.exe", 64<<20)
	if err != nil {
		return err
	}
	if hashUpdateBytes(executableData) != manifest.WindowsAMD64 {
		return errors.New("release executable SHA-256 与 manifest 不匹配")
	}
	replaceSource := baseline.Head != manifest.Revision
	stagedSource := ""
	if replaceSource {
		if err := requireSafeSourceReplacement(git, source, manifest.Revision); err != nil {
			return err
		}
		stagedSource, err = cloneReleaseSource(git, source, manifest.Version, manifest.Revision)
		if err != nil {
			return err
		}
	}
	cleanupStaged := replaceSource
	defer func() {
		if cleanupStaged {
			_ = os.RemoveAll(stagedSource)
		}
	}()
	stagedExe, err := writeUpdateExecutable(source, executableData)
	if err != nil {
		return err
	}
	defer os.Remove(stagedExe)
	if err := validateUpdateExecutable(stagedExe, manifest.Version); err != nil {
		return err
	}
	if replaceSource {
		if err := validateCanonicalSource(stagedSource); err != nil {
			return err
		}
		if err := validateCanonicalGitSource(git, stagedSource); err != nil {
			return err
		}
	}
	currentState, err := captureCanonicalUpdateState(git, source)
	if err != nil {
		return err
	}
	if currentState != baseline {
		return errors.New("canonical checkout 在 update 准备期间发生变化；未执行切换")
	}
	result, err := a.platform.ActivateUpdate(updateInstall{
		Source: source, StagedSource: stagedSource, ReplaceSource: replaceSource,
		Executable: stagedExe, Version: manifest.Version,
		ExpectedHead: baseline.Head, ExpectedStatus: baseline.Status, ExpectedRefs: baseline.Refs,
	})
	if err != nil {
		return err
	}
	cleanupStaged = false
	if result.CleanupPath != "" {
		fmt.Fprintf(a.stdout, "旧 canonical checkout 已保留，请复核后手工删除：%s\n", result.CleanupPath)
	}
	fmt.Fprintf(a.stdout, "STeamAI 已更新到 %s。\n", manifest.Version)
	return nil
}

func (a *app) uninstall(args []string) error {
	if len(args) != 0 {
		return errors.New("uninstall 不接受参数")
	}
	active, err := a.platform.ActiveExecutable()
	if err != nil {
		return fmt.Errorf("定位已安装的 steamai.exe: %w", err)
	}
	current, err := a.executable()
	if err != nil {
		return err
	}
	if !sameExecutablePath(active, current) {
		return errors.New("uninstall 必须由 setup 安装的 steamai.exe 执行")
	}
	result, err := a.platform.Uninstall(current)
	if err != nil {
		return err
	}
	fmt.Fprintln(a.stdout, "STeamAI 入口已卸载；canonical checkout 与所有 case 已保留。")
	if result.Source != "" {
		fmt.Fprintln(a.stdout, "保留的 canonical checkout:", result.Source)
	}
	if result.CleanupDeferred {
		fmt.Fprintln(a.stdout, "当前 steamai.exe 已交给原生清理进程，将在本进程退出后删除。")
	}
	if result.CleanupHelper != "" {
		fmt.Fprintln(a.stdout, "可手工删除的临时 uninstall helper:", result.CleanupHelper)
	}
	return nil
}

func sameExecutablePath(left, right string) bool {
	left, leftErr := filepath.Abs(left)
	right, rightErr := filepath.Abs(right)
	return leftErr == nil && rightErr == nil && strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}

type canonicalUpdateState struct {
	Head   string
	Status string
	Refs   string
}

func captureCanonicalUpdateState(git, source string) (canonicalUpdateState, error) {
	head, err := gitUpdateOutput(git, source, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return canonicalUpdateState{}, errors.New("canonical checkout 没有有效 HEAD commit")
	}
	status, err := gitUpdateOutput(git, source, "status", "--porcelain=v1", "--untracked-files=all")
	if err != nil {
		return canonicalUpdateState{}, err
	}
	if strings.TrimSpace(status) != "" {
		return canonicalUpdateState{}, errors.New("canonical checkout 有未提交或未跟踪内容；请先 local commit，再运行 update")
	}
	others, err := gitUpdateOutput(git, source, "ls-files", "--others", "--directory", "--no-empty-directory")
	if err != nil {
		return canonicalUpdateState{}, err
	}
	if strings.TrimSpace(others) != "" {
		return canonicalUpdateState{}, errors.New("canonical checkout 有 ignored 或未跟踪内容；update 不会删除这些本机文件")
	}
	modules, err := gitUpdateOutput(git, source, "submodule", "status", "--recursive")
	if err != nil {
		return canonicalUpdateState{}, err
	}
	if strings.TrimSpace(modules) != "" {
		return canonicalUpdateState{}, errors.New("canonical checkout 含 Git submodule；v1 update 不会替换其本机状态")
	}
	refs, err := localReferenceSnapshot(git, source)
	if err != nil {
		return canonicalUpdateState{}, err
	}
	return canonicalUpdateState{Head: strings.TrimSpace(head), Status: status, Refs: refs}, nil
}

func requireCleanCanonicalSource(git, source string) error {
	_, err := captureCanonicalUpdateState(git, source)
	return err
}

func localReferenceSnapshot(git, source string) (string, error) {
	refs, err := gitUpdateOutput(git, source, "for-each-ref", "--format=%(refname)%00%(objectname)", "refs/heads", "refs/stash")
	if err != nil {
		return "", err
	}
	lines := strings.FieldsFunc(refs, func(r rune) bool { return r == '\n' || r == '\r' })
	sort.Strings(lines)
	return hashUpdateBytes([]byte(strings.Join(lines, "\n") + "\n")), nil
}

func requireSafeSourceReplacement(git, source, revision string) error {
	origin, err := gitUpdateOutput(git, source, "remote", "get-url", "origin")
	if err != nil {
		return errors.New("canonical checkout 缺少 origin；update 不会猜测来源")
	}
	if !strings.HasPrefix(strings.TrimSpace(origin), "https://") {
		return errors.New("canonical origin 必须使用 HTTPS")
	}
	if _, err := gitUpdateOutput(git, source, "fetch", "--quiet", "--no-tags", "origin", revision); err != nil {
		return fmt.Errorf("获取 release revision: %w", err)
	}
	return requireSourceAncestor(git, source, revision)
}

func requireSourceAncestor(git, source, revision string) error {
	if _, err := gitUpdateOutput(git, source, "merge-base", "--is-ancestor", "HEAD", revision); err != nil {
		return errors.New("canonical checkout 含未发布的本地 commit 或与 release 分叉；update 不会丢弃或合并")
	}
	refs, err := gitUpdateOutput(git, source, "for-each-ref", "--format=%(refname)", "refs/heads", "refs/stash")
	if err != nil {
		return err
	}
	for ref := range strings.FieldsSeq(refs) {
		if _, err := gitUpdateOutput(git, source, "merge-base", "--is-ancestor", ref, revision); err != nil {
			return fmt.Errorf("canonical checkout 的本地引用 %s 含 release 未包含的 commit；update 不会删除它", ref)
		}
	}
	return nil
}

func cloneReleaseSource(git, source, version, revision string) (string, error) {
	parent := filepath.Dir(source)
	staged, err := os.MkdirTemp(parent, ".steamai-source-update-")
	if err != nil {
		return "", err
	}
	_ = os.Remove(staged)
	origin, err := gitUpdateOutput(git, source, "remote", "get-url", "origin")
	if err != nil {
		return "", errors.New("canonical checkout 缺少 origin；update 不会猜测来源")
	}
	origin = strings.TrimSpace(origin)
	if !strings.HasPrefix(origin, "https://") {
		return "", errors.New("canonical origin 必须使用 HTTPS")
	}
	if _, err := gitUpdateOutput(git, parent, "clone", "--quiet", "--no-local", "--branch", version, "--single-branch", "--", origin, staged); err != nil {
		_ = os.RemoveAll(staged)
		return "", fmt.Errorf("clone release source: %w", err)
	}
	head, err := gitUpdateOutput(git, staged, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil || strings.TrimSpace(head) != revision {
		_ = os.RemoveAll(staged)
		return "", errors.New("release tag 与 manifest revision 不匹配")
	}
	if err := requireCleanCanonicalSource(git, staged); err != nil {
		_ = os.RemoveAll(staged)
		return "", errors.New("staged release source 不是 clean checkout")
	}
	return staged, nil
}

func writeUpdateExecutable(source string, data []byte) (string, error) {
	file, err := os.CreateTemp(filepath.Dir(source), ".steamai-update-*.exe")
	if err != nil {
		return "", err
	}
	path := file.Name()
	if _, err := file.Write(data); err != nil {
		file.Close()
		os.Remove(path)
		return "", err
	}
	if err := file.Close(); err != nil {
		os.Remove(path)
		return "", err
	}
	return path, nil
}

func validateUpdateExecutable(path, version string) error {
	cmd := exec.Command(path, "--version")
	cmd.Env = withoutEnvironment(os.Environ(), "CLAUDECODE")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("release executable 兼容检查失败: %s: %w", strings.TrimSpace(string(output)), err)
	}
	if strings.TrimSpace(string(output)) != version {
		return errors.New("release executable --version 与 manifest 不匹配")
	}
	return nil
}

func downloadExact(url string, limit int64) ([]byte, error) {
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	response, err := updateHTTPClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("下载 %s: %w", url, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("下载 %s: HTTP %s", url, response.Status)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit || len(data) == 0 {
		return nil, errors.New("下载内容为空或超过大小限制")
	}
	return data, nil
}

func gitUpdateOutput(git, dir string, args ...string) (string, error) {
	cmd := exec.Command(git, args...)
	cmd.Dir = dir
	cmd.Env = withoutEnvironment(os.Environ(), "CLAUDECODE")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(output)), err)
	}
	return string(output), nil
}

func hashUpdateBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
