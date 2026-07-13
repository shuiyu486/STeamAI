package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
)

type Context struct {
	RuntimeRoot    string
	RepoRoot       string
	Cwd            string
	Target         string
	TargetProvided bool
	Pack           string
}

func New(target, pack string) (Context, error) {
	cwd, err := refsf.FullPath("")
	if err != nil {
		return Context{}, err
	}
	repo, err := discoverRepoRoot(cwd)
	if err != nil {
		return Context{}, err
	}
	if pack == "" {
		pack = defaults.DefaultPack
	}
	targetProvided := strings.TrimSpace(target) != ""
	resolvedTarget := cwd
	if targetProvided {
		resolvedTarget = target
	}
	resolvedTarget, err = refsf.FullPath(resolvedTarget)
	if err != nil {
		return Context{}, err
	}
	return Context{RuntimeRoot: filepath.Join(repo, "rekit"), RepoRoot: repo, Cwd: cwd, Target: resolvedTarget, TargetProvided: targetProvided, Pack: pack}, nil
}

func discoverRepoRoot(cwd string) (string, error) {
	if repo, ok := findRepoRoot(cwd); ok {
		return repo, nil
	}
	if exe, err := os.Executable(); err == nil {
		if repo, ok := findRepoRoot(filepath.Dir(exe)); ok {
			return repo, nil
		}
	}
	return "", fmt.Errorf("unable to locate rekit repo root from %s", cwd)
}

func findRepoRoot(start string) (string, bool) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", false
	}
	if st, err := os.Stat(dir); err == nil && !st.IsDir() {
		dir = filepath.Dir(dir)
	}
	for {
		if refsf.Exists(filepath.Join(dir, "rekit", "rekit.ps1")) && refsf.Exists(filepath.Join(dir, "packs")) {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}
