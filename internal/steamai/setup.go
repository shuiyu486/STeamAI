package steamai

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const defaultCloneURL = "https://github.com/shuiyu486/STeamAI.git"

func (a *app) setup(args []string) error {
	var source, cloneURL string
	for len(args) > 0 {
		option := args[0]
		args = args[1:]
		switch option {
		case "--source":
			if len(args) == 0 {
				return errors.New("--source 需要目录")
			}
			source, args = args[0], args[1:]
		case "--clone-url":
			if len(args) == 0 {
				return errors.New("--clone-url 需要 URL")
			}
			cloneURL, args = args[0], args[1:]
		default:
			return fmt.Errorf("未知 setup 参数 %q", option)
		}
	}
	if source != "" && cloneURL != "" {
		return errors.New("--source 与 --clone-url 不能同时使用")
	}
	if source == "" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			return errors.New("LOCALAPPDATA 不可用")
		}
		source = filepath.Join(localAppData, "STeamAI", "source")
		exists, err := pathExists(source)
		if err != nil {
			return err
		}
		if cloneURL != "" && exists {
			return errors.New("--clone-url 不能覆盖已有 canonical source")
		}
		if !exists {
			if cloneURL == "" {
				cloneURL = defaultCloneURL
			}
			if err := a.cloneSource(cloneURL, source); err != nil {
				return err
			}
		}
	}
	source, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	if err := a.validateSource(source); err != nil {
		return err
	}
	executable, err := a.executable()
	if err != nil {
		return fmt.Errorf("定位 steamai.exe: %w", err)
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return err
	}
	if err := validateNativeExecutablePath(executable); err != nil {
		return errors.New("setup 只能由原生 steamai.exe 执行")
	}
	if err := a.platform.Install(executable, source, a.version); err != nil {
		return err
	}
	fmt.Fprintln(a.stdout, "STeamAI 安装完成。请重新打开终端后运行 steamai。")
	fmt.Fprintln(a.stdout, "Canonical source:", source)
	return nil
}

func (a *app) cloneSource(url, target string) error {
	if !strings.HasPrefix(url, "https://") {
		return errors.New("clone URL 必须使用 HTTPS")
	}
	git, err := a.resolveNativeExecutable("git.exe")
	if err != nil {
		return fmt.Errorf("找不到原生 Git: %w", err)
	}
	parent := filepath.Dir(target)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	staging, err := os.MkdirTemp(parent, ".steamai-source-staging-")
	if err != nil {
		return err
	}
	if err := os.Remove(staging); err != nil {
		return err
	}
	cmd := exec.Command(git, "clone", "--", url, staging)
	cmd.Dir = parent
	cmd.Stdin = a.stdin
	cmd.Stdout = a.stdout
	cmd.Stderr = a.stderr
	cmd.Env = withoutEnvironment(os.Environ(), "CLAUDECODE")
	if err := cmd.Run(); err != nil {
		_ = os.RemoveAll(staging)
		return fmt.Errorf("clone canonical source: %w", err)
	}
	if err := validateCanonicalSource(staging); err != nil {
		_ = os.RemoveAll(staging)
		return err
	}
	if err := validateCanonicalGitSource(git, staging); err != nil {
		_ = os.RemoveAll(staging)
		return err
	}
	if exists, err := pathExists(target); err != nil {
		_ = os.RemoveAll(staging)
		return err
	} else if exists {
		_ = os.RemoveAll(staging)
		return errors.New("canonical source 目标在 clone 期间被占用")
	}
	if err := os.Rename(staging, target); err != nil {
		_ = os.RemoveAll(staging)
		return fmt.Errorf("发布 canonical source: %w", err)
	}
	return nil
}
