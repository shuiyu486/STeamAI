package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
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
	if !targetProvided {
		if caseRoot, ok := findCaseRoot(cwd); ok {
			resolvedTarget = caseRoot
		}
	}
	repo, err := discoverRepoRoot(cwd)
	if err != nil {
		if targetProvided {
			repo, err = discoverRepoRoot(resolvedTarget)
		}
		if err != nil {
			repo, err = repoRootFromCaseMetadata(resolvedTarget)
		}
		if err != nil {
			return Context{}, err
		}
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

func repoRootFromCaseMetadata(target string) (string, error) {
	inst, err := instance.Read(target)
	if err != nil {
		return "", err
	}
	repo := strings.TrimSpace(inst.TemplateRoot)
	if repo == "" {
		return "", fmt.Errorf("unable to locate rekit repo root from case metadata in %s", target)
	}
	repo, err = refsf.FullPath(repo)
	if err != nil {
		return "", err
	}
	if found, ok := findRepoRoot(repo); ok {
		return found, nil
	}
	return "", fmt.Errorf("case metadata templateRoot is not a rekit repo root: %s", repo)
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

func findCaseRoot(start string) (string, bool) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", false
	}
	if st, err := os.Stat(dir); err == nil && !st.IsDir() {
		dir = filepath.Dir(dir)
	}
	for {
		if instance.LooksLikeCase(dir) {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}
