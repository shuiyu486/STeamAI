package promote

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

// ErrProjectLocalBundlePackMutation identifies promote operations that would
// mutate the pack tree carried by a project-local verified runtime bundle.
var ErrProjectLocalBundlePackMutation = errors.New("promote pack-tree mutation is blocked for a project-local verified bundle")

// refuseProjectLocalBundlePackMutation classifies repoRoot itself instead of
// trusting caller case metadata. This leaves central and legacy source repos
// available while preventing a legacy caller or path alias from treating a
// current project's delivery bundle as an authoritative pack source.
func refuseProjectLocalBundlePackMutation(repoRoot, _ string) error {
	inst, projectLocal, err := projectLocalBundleInstanceForRepoRoot(repoRoot)
	if err != nil || !projectLocal {
		return err
	}
	if err := requireProjectLocalPhysicalBinding(inst.TemplateRoot, repoRoot, "templateRoot and repoRoot"); err != nil {
		return err
	}
	if err := requireProjectLocalPhysicalBinding(inst.BundleRoot, filepath.Join(repoRoot, "runtime"), "bundleRoot and repoRoot/runtime"); err != nil {
		return err
	}
	return ErrProjectLocalBundlePackMutation
}

func projectLocalBundleInstanceForRepoRoot(repoRoot string) (instance.Instance, bool, error) {
	repoRoot, err := filepath.Abs(strings.TrimSpace(repoRoot))
	if err != nil {
		return instance.Instance{}, false, err
	}
	repoRoot = filepath.Clean(repoRoot)
	info, err := os.Stat(repoRoot)
	if err != nil {
		return instance.Instance{}, false, err
	}
	if !info.IsDir() {
		return instance.Instance{}, false, fmt.Errorf("promote repoRoot must be a directory: %s", repoRoot)
	}
	physicalRoot, err := promotePhysicalPath(repoRoot)
	if err != nil {
		return instance.Instance{}, false, fmt.Errorf("resolve promote repoRoot physical identity: %w", err)
	}

	owner, err := filepath.Abs(filepath.Dir(filepath.Clean(physicalRoot)))
	if err != nil {
		return instance.Instance{}, false, err
	}
	owner = filepath.Clean(owner)
	canonicalStateRoot := filepath.Join(owner, projectstate.CurrentDir)
	same, sameErr := refsf.SameExistingPath(canonicalStateRoot, repoRoot)
	if os.IsNotExist(sameErr) || !same && sameErr == nil {
		return instance.Instance{}, false, nil
	}
	if sameErr != nil {
		return instance.Instance{}, false, fmt.Errorf("compare promote repoRoot physical identity: %w", sameErr)
	}
	root, resolveErr := projectstate.Resolve(owner)
	if resolveErr != nil {
		return instance.Instance{}, false, fmt.Errorf("%w: cannot validate repoRoot project state owner: %v", ErrProjectLocalBundlePackMutation, resolveErr)
	}
	if !root.Existing || root.Legacy {
		return instance.Instance{}, false, fmt.Errorf("%w: repoRoot physical identity has an invalid current state owner", ErrProjectLocalBundlePackMutation)
	}
	inst, readErr := instance.Read(owner)
	if readErr != nil {
		return instance.Instance{}, false, fmt.Errorf("%w: cannot read repoRoot project-local metadata: %v", ErrProjectLocalBundlePackMutation, readErr)
	}
	declaredProjectLocalBundle := inst.Mode == "project-local-bundle"
	if inst.Source != "steamai" || inst.StateDir != projectstate.CurrentDir || inst.SchemaVersion < 2 || !declaredProjectLocalBundle {
		return instance.Instance{}, false, fmt.Errorf("%w: repoRoot project-local metadata does not have the required source, schemaVersion, and mode", ErrProjectLocalBundlePackMutation)
	}
	return inst, true, nil
}

func requireProjectLocalPhysicalBinding(recorded, expected, label string) error {
	same, err := refsf.SameExistingPath(recorded, expected)
	if err != nil {
		return fmt.Errorf("%w: cannot confirm physical binding for %s: %v", ErrProjectLocalBundlePackMutation, label, err)
	}
	if !same {
		return fmt.Errorf("%w: physical binding mismatch for %s", ErrProjectLocalBundlePackMutation, label)
	}
	return nil
}
