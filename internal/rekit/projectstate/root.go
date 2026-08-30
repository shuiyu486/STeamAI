package projectstate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
)

const (
	CurrentDir = ".steamai"
	LegacyDir  = ".rekit"
)

type Root struct {
	Dir      string
	Path     string
	Existing bool
	Legacy   bool
}

func Resolve(caseRoot string) (Root, error) {
	caseRoot = strings.TrimSpace(caseRoot)
	if caseRoot == "" {
		return Root{}, fmt.Errorf("project root is empty")
	}
	caseRoot, err := filepath.Abs(caseRoot)
	if err != nil {
		return Root{}, err
	}
	caseRoot = filepath.Clean(caseRoot)
	if err := validateCaseRoot(caseRoot); err != nil {
		return Root{}, err
	}
	currentPath := filepath.Join(caseRoot, CurrentDir)
	legacyPath := filepath.Join(caseRoot, LegacyDir)
	currentExists, err := pathExists(currentPath)
	if err != nil {
		return Root{}, err
	}
	legacyExists, err := pathExists(legacyPath)
	if err != nil {
		return Root{}, err
	}
	if currentExists && legacyExists {
		return Root{}, fmt.Errorf("project contains both %s and %s; mutable STeamAI state roots must not coexist", CurrentDir, LegacyDir)
	}
	if currentExists {
		if _, err := refsf.ValidateNonReparseDirectory(currentPath, "STeamAI state root"); err != nil {
			return Root{}, err
		}
		if err := validateCurrentMigrationNamespace(currentPath); err != nil {
			return Root{}, err
		}
		return Root{Dir: CurrentDir, Path: currentPath, Existing: true}, nil
	}
	if legacyExists {
		if _, err := refsf.ValidateNonReparseDirectory(legacyPath, "legacy rekit state root"); err != nil {
			return Root{}, err
		}
		return Root{Dir: LegacyDir, Path: legacyPath, Existing: true, Legacy: true}, nil
	}
	legacyMetadataPath := filepath.Join(caseRoot, ".re-template.yml")
	legacyMetadata, err := pathExists(legacyMetadataPath)
	if err != nil {
		return Root{}, err
	}
	if legacyMetadata {
		if err := validateLegacyMetadata(legacyMetadataPath); err != nil {
			return Root{}, err
		}
		return Root{Dir: LegacyDir, Path: legacyPath, Legacy: true}, nil
	}
	return Root{Dir: CurrentDir, Path: currentPath}, nil
}

func PublicEntrypoint(caseRoot string) (string, error) {
	root, err := Resolve(caseRoot)
	if err != nil {
		return "", err
	}
	if root.Legacy {
		return commands.LegacyPublicEntrypoint, nil
	}
	return commands.CurrentPublicEntrypoint, nil
}

func ProjectPublicCommand(caseRoot, command string) (string, error) {
	invocation, err := commands.ParsePublicInvocation(command)
	if err != nil {
		return "", err
	}
	entrypoint, err := PublicEntrypoint(caseRoot)
	if err != nil {
		return "", err
	}
	command = strings.TrimSpace(command)
	if strings.HasPrefix(command, entrypoint+" ") {
		return command, nil
	}
	return invocation.RenderForEntrypoint(entrypoint)
}

func Rel(caseRoot string, parts ...string) (string, error) {
	root, err := Resolve(caseRoot)
	if err != nil {
		return "", err
	}
	if missionScopedParts(parts) {
		view, err := ResolveMissionView(caseRoot)
		if err != nil {
			return "", err
		}
		return view.Rel(parts...)
	}
	local, err := validateParts(parts)
	if err != nil {
		return "", err
	}
	path := root.Path
	if local != "" {
		path = filepath.Join(path, local)
	}
	if err := validateContained(root.Path, path); err != nil {
		return "", err
	}
	rel, err := filepath.Rel(filepath.Dir(root.Path), path)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}

func Join(caseRoot string, parts ...string) (string, error) {
	root, err := Resolve(caseRoot)
	if err != nil {
		return "", err
	}
	if missionScopedParts(parts) {
		view, err := ResolveMissionView(caseRoot)
		if err != nil {
			return "", err
		}
		return view.Join(parts...)
	}
	local, err := validateParts(parts)
	if err != nil {
		return "", err
	}
	path := root.Path
	if local != "" {
		path = filepath.Join(path, local)
	}
	if err := validateContained(root.Path, path); err != nil {
		return "", err
	}
	return path, nil
}

func missionScopedParts(parts []string) bool {
	if len(parts) == 0 {
		return false
	}
	first := strings.TrimSpace(filepath.ToSlash(parts[0]))
	if index := strings.IndexByte(first, '/'); index >= 0 {
		first = first[:index]
	}
	return MissionScopedName(first)
}

// CurrentRel is only for compile-time constant, trusted path fragments. Dynamic
// values must use Rel so invalid local paths are reported instead of normalized.
func CurrentRel(parts ...string) string {
	clean := append([]string{CurrentDir}, parts...)
	return filepath.ToSlash(filepath.Join(clean...))
}

func validateCaseRoot(caseRoot string) error {
	current := caseRoot
	for {
		_, err := os.Lstat(current)
		if err == nil {
			label := "project root existing ancestor"
			if current == caseRoot {
				label = "project root"
			}
			_, err = refsf.ValidateNonReparseDirectory(current, label)
			return err
		}
		if !os.IsNotExist(err) {
			return err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return fmt.Errorf("project root has no existing directory ancestor: %s", caseRoot)
		}
		current = parent
	}
}

func validateParts(parts []string) (string, error) {
	components := make([]string, 0, len(parts))
	for _, raw := range parts {
		part := strings.TrimSpace(raw)
		if part == "" {
			return "", fmt.Errorf("project state path contains an empty component")
		}
		if part != raw {
			return "", fmt.Errorf("project state path component has leading or trailing whitespace: %q", raw)
		}
		local := filepath.FromSlash(part)
		if filepath.IsAbs(local) || filepath.VolumeName(local) != "" || !filepath.IsLocal(local) {
			return "", fmt.Errorf("project state path component must be a local relative path: %s", raw)
		}
		rawComponents := strings.FieldsFunc(local, func(r rune) bool {
			return r == '/' || r == '\\'
		})
		if len(rawComponents) == 0 {
			return "", fmt.Errorf("project state path contains an invalid component: %s", raw)
		}
		for _, component := range rawComponents {
			if component == "." || component == ".." || !filepath.IsLocal(component) {
				return "", fmt.Errorf("project state path contains an invalid component: %s", raw)
			}
			if strings.ContainsAny(component, ":\x00") || (isWindowsPathSemantics() && (strings.ContainsAny(component, `<>"|?*`) || component != strings.TrimRight(component, ". "))) {
				return "", fmt.Errorf("project state path contains an invalid Windows-local component: %s", raw)
			}
		}
		components = append(components, local)
	}
	return filepath.Join(components...), nil
}

func isWindowsPathSemantics() bool {
	return filepath.Separator == '\\'
}

func validateContained(rootPath, path string) error {
	rel, err := filepath.Rel(rootPath, path)
	if err != nil || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("project state path escapes selected root: %s", path)
	}
	return nil
}

func validateCurrentMigrationNamespace(currentPath string) error {
	migrationPath := filepath.Join(currentPath, "migration")
	migrationExists, err := pathExists(migrationPath)
	if err != nil || !migrationExists {
		return err
	}
	if _, err := refsf.ValidateNonReparseDirectory(migrationPath, "STeamAI state migration namespace"); err != nil {
		return fmt.Errorf("state migration is partial; mutable project state is blocked: %w", err)
	}
	entries, err := os.ReadDir(migrationPath)
	if err != nil {
		return fmt.Errorf("state migration is partial; mutable project state is blocked: %w", err)
	}
	if len(entries) != 1 || entries[0].Name() != "state-root-v1.json" {
		return fmt.Errorf("state migration is partial; mutable project state is blocked until the exact receipt is present")
	}
	info, err := entries[0].Info()
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() < 1 {
		if err != nil {
			return fmt.Errorf("state migration is partial; mutable project state is blocked: %w", err)
		}
		return fmt.Errorf("state migration is partial; mutable project state is blocked until the exact receipt is a non-empty regular file")
	}
	return nil
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

func validateLegacyMetadata(path string) error {
	if err := refsf.ValidateNoReparseComponents(path); err != nil {
		return fmt.Errorf("legacy metadata: %w", err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("legacy metadata must be a regular non-symlink file: %s", path)
	}
	return nil
}
