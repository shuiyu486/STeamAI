package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtimebundle"
	syncreview "github.com/shuiyu486/re-context-kits/internal/rekit/sync"
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
	return NewWithCwd(target, pack, "")
}

// NewProjectLocal resolves runtime state from the executable owner while
// preserving whether the caller explicitly supplied a target.
func NewProjectLocal(projectRoot, target, pack, cwdOverride string) (Context, error) {
	targetProvided := strings.TrimSpace(target) != ""
	resolvedTarget, err := ResolveProjectLocalTarget(
		projectRoot,
		target,
		cwdOverride,
	)
	if err != nil {
		return Context{}, err
	}
	ctx, err := NewWithCwd(resolvedTarget, pack, cwdOverride)
	if err != nil {
		return Context{}, err
	}
	ctx.TargetProvided = targetProvided
	return ctx, nil
}

// ValidateRunningExecutable is a no-op for central maintenance binaries. A
// process launched from a project-local .steamai bundle must match both the
// schema v2 instance binding and the exact canonical runtime manifest before
// any runtime or host command is dispatched.
func ValidateRunningExecutable() error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve running STeamAI executable: %w", err)
	}
	return ValidateExecutable(executable)
}

// RunningExecutableProjectRoot returns the physical project root only when the
// current process is in the canonical project-local runtime layout. It does not
// validate mutable instance or bundle metadata and is therefore suitable only
// for selecting a recovery-aware validation path.
func RunningExecutableProjectRoot() (string, bool, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", false, fmt.Errorf("resolve running STeamAI executable: %w", err)
	}
	assetRoot, projectLocal, err := runtimebundle.AssetRootForExecutable(executable)
	if err != nil || !projectLocal {
		return "", projectLocal, err
	}
	return filepath.Dir(assetRoot), true, nil
}

// ResolveProjectLocalTarget binds one project-local process to its physical
// owner project. An omitted target resolves to the owner without changing the
// caller-visible TargetProvided contract; an explicit target must be the same
// non-reparse directory.
func ResolveProjectLocalTarget(projectRoot, target, cwdOverride string) (string, error) {
	projectRoot, err := filepath.Abs(strings.TrimSpace(projectRoot))
	if err != nil {
		return "", err
	}
	projectRoot = filepath.Clean(projectRoot)
	if _, err := refsf.ValidateNonReparseDirectory(projectRoot, "project-local STeamAI executable owner"); err != nil {
		return "", err
	}
	if strings.TrimSpace(target) == "" {
		return projectRoot, nil
	}
	cwd, err := refsf.FullPath(cwdOverride)
	if err != nil {
		return "", err
	}
	resolvedTarget := strings.TrimSpace(target)
	if !filepath.IsAbs(resolvedTarget) {
		resolvedTarget = filepath.Join(cwd, resolvedTarget)
	}
	resolvedTarget, err = filepath.Abs(resolvedTarget)
	if err != nil {
		return "", err
	}
	resolvedTarget = filepath.Clean(resolvedTarget)
	if _, err := refsf.ValidateNonReparseDirectory(resolvedTarget, "project-local STeamAI invocation target"); err != nil {
		return "", err
	}
	same, err := refsf.SameExistingPath(projectRoot, resolvedTarget)
	if err != nil {
		return "", fmt.Errorf("compare project-local STeamAI executable owner and invocation target: %w", err)
	}
	if !same {
		return "", fmt.Errorf(
			"running project-local STeamAI executable target mismatch: executableProject=%s target=%s",
			projectRoot,
			resolvedTarget,
		)
	}
	return projectRoot, nil
}

// ValidateRunningExecutableForRecovery preserves ordinary bundle validation as
// the primary route. Only a canonical project-local process whose durable
// current-sync plan binds its exact bytes may use the recovery fallback.
func ValidateRunningExecutableForRecovery(caseRoot string) error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve running STeamAI executable: %w", err)
	}
	return ValidateExecutableForRecovery(executable, caseRoot)
}

func ValidateExecutableForRecovery(executable, caseRoot string) error {
	ordinaryErr := ValidateExecutable(executable)
	assetRoot, projectLocal, layoutErr :=
		runtimebundle.AssetRootForExecutable(executable)
	if layoutErr != nil || !projectLocal {
		if ordinaryErr != nil {
			return ordinaryErr
		}
		return layoutErr
	}
	caseRoot, err := ResolveProjectLocalTarget(
		filepath.Dir(assetRoot),
		caseRoot,
		"",
	)
	if err != nil {
		if ordinaryErr != nil {
			return fmt.Errorf("%w; ordinary validation: %v", err, ordinaryErr)
		}
		return err
	}
	if ordinaryErr == nil {
		return nil
	}
	if err := syncreview.ValidateCurrentSyncRecoveryExecutable(
		caseRoot,
		executable,
	); err != nil {
		return fmt.Errorf(
			"validate running project-local STeamAI recovery executable: %w; ordinary validation: %v",
			err,
			ordinaryErr,
		)
	}
	return nil
}

func ValidateExecutable(executable string) error {
	assetRoot, projectLocal, err := runtimebundle.AssetRootForExecutable(executable)
	if err != nil {
		return err
	}
	if !projectLocal {
		return nil
	}
	caseRoot := filepath.Dir(assetRoot)
	inst, err := instance.Read(caseRoot)
	if err != nil {
		return fmt.Errorf("read project-local STeamAI executable binding: %w", err)
	}
	if inst.Source == "missing" || inst.SchemaVersion < 2 || inst.Mode != "project-local-bundle" {
		return fmt.Errorf("project-local STeamAI executable requires schema v2 project-local-bundle metadata: %s", inst.InstancePath)
	}
	if !refsf.SamePath(inst.TemplateRoot, assetRoot) ||
		!refsf.SamePath(inst.BundleRoot, filepath.Join(assetRoot, "runtime")) ||
		strings.TrimSpace(inst.BundleManifestSHA256) == "" {
		return fmt.Errorf("project-local STeamAI executable metadata does not bind its canonical asset and runtime roots: %s", inst.InstancePath)
	}
	manifest, err := runtimebundle.Validate(assetRoot, inst.BundleManifest, inst.BundleManifestSHA256, inst.TemplatePack)
	if err != nil {
		return fmt.Errorf("validate running project-local STeamAI bundle: %w", err)
	}
	if err := runtimebundle.ValidateExecutableIdentity(assetRoot, executable, manifest); err != nil {
		return err
	}
	return nil
}

func NewWithCwd(target, pack, cwdOverride string) (Context, error) {
	cwd, err := refsf.FullPath(cwdOverride)
	if err != nil {
		return Context{}, err
	}
	packProvided := strings.TrimSpace(pack) != ""
	if !packProvided {
		pack = defaults.DefaultPack
	}
	targetProvided := strings.TrimSpace(target) != ""
	resolvedTarget := cwd
	if targetProvided {
		resolvedTarget = target
		if !filepath.IsAbs(resolvedTarget) {
			resolvedTarget = filepath.Join(cwd, resolvedTarget)
		}
	}
	resolvedTarget, err = filepath.Abs(resolvedTarget)
	if err != nil {
		return Context{}, err
	}
	if !targetProvided {
		caseRoot, found, findErr := findCaseRoot(cwd)
		if findErr != nil {
			return Context{}, findErr
		}
		if found {
			resolvedTarget = caseRoot
		}
	}
	if repo, runtimeRoot, bundlePack, ok, bundleErr := bundleRootFromCaseMetadata(resolvedTarget); bundleErr != nil {
		return Context{}, bundleErr
	} else if ok {
		if packProvided && !strings.EqualFold(pack, bundlePack) {
			return Context{}, fmt.Errorf("requested pack differs from project-local STeamAI bundle: requested=%s bundle=%s", pack, bundlePack)
		}
		return Context{RuntimeRoot: runtimeRoot, RepoRoot: repo, Cwd: cwd, Target: resolvedTarget, TargetProvided: targetProvided, Pack: bundlePack}, nil
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

func bundleRootFromCaseMetadata(target string) (string, string, string, bool, error) {
	inst, err := instance.Read(target)
	if err != nil {
		return "", "", "", false, err
	}
	if inst.Source == "missing" || inst.SchemaVersion < 2 || inst.Mode != "project-local-bundle" {
		return "", "", "", false, nil
	}
	assetRoot := strings.TrimSpace(inst.TemplateRoot)
	runtimeRoot := strings.TrimSpace(inst.BundleRoot)
	if assetRoot == "" || runtimeRoot == "" || strings.TrimSpace(inst.BundleManifestSHA256) == "" {
		return "", "", "", false, fmt.Errorf("schema v2 STeamAI project metadata omits its strict bundle binding: %s", inst.InstancePath)
	}
	if !refsf.SamePath(filepath.Join(assetRoot, "runtime"), runtimeRoot) {
		return "", "", "", false, fmt.Errorf("schema v2 STeamAI bundleRoot has an invalid layout: %s", runtimeRoot)
	}
	manifest, err := runtimebundle.Validate(assetRoot, inst.BundleManifest, inst.BundleManifestSHA256, inst.TemplatePack)
	if err != nil {
		return "", "", "", false, fmt.Errorf("validate project-local STeamAI runtime bundle: %w", err)
	}
	return assetRoot, runtimeRoot, manifest.Pack, true, nil
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
		if isRekitRepoRoot(dir) {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func isRekitRepoRoot(dir string) bool {
	packs, err := os.Stat(filepath.Join(dir, "packs"))
	if err != nil || !packs.IsDir() {
		return false
	}
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return false
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		return line == "module github.com/shuiyu486/re-context-kits"
	}
	return false
}

func findCaseRoot(start string) (string, bool, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", false, err
	}
	if st, err := os.Lstat(dir); err == nil && !st.IsDir() {
		dir = filepath.Dir(dir)
	} else if err != nil {
		return "", false, err
	}
	for {
		looksLikeCase, err := instance.CheckCase(dir)
		if err != nil {
			return "", false, fmt.Errorf("inspect case root candidate %s: %w", dir, err)
		}
		if looksLikeCase {
			return dir, true, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false, nil
		}
		dir = parent
	}
}
