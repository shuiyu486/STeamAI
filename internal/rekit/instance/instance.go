package instance

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/missionintent"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

const (
	StateDirSTeamAI = projectstate.CurrentDir
	StateDirRekit   = projectstate.LegacyDir
)

type Instance struct {
	CaseRoot             string
	InstancePath         string
	Source               string
	StateDir             string
	SchemaVersion        int
	Mode                 string
	TemplateRoot         string
	BundleRoot           string
	BundleManifest       string
	BundleManifestSHA256 string
	TemplatePack         string
	ProjectName          string
	ProjectRoot          string
}

func LooksLikeCase(path string) bool {
	ok, err := CheckCase(path)
	return err == nil && ok
}

func CheckCase(path string) (bool, error) {
	root, err := projectstate.Resolve(path)
	if err != nil {
		return false, err
	}
	instancePath := filepath.Join(root.Path, "instance.yml")
	if exists, err := regularMetadataExists(instancePath); err != nil {
		return false, err
	} else if exists {
		return true, nil
	}
	if root.Legacy {
		return regularMetadataExists(filepath.Join(path, ".re-template.yml"))
	}
	return false, nil
}

func Read(target string) (Instance, error) {
	caseRoot, err := filepath.Abs(target)
	if err != nil {
		return Instance{}, err
	}
	root, err := projectstate.Resolve(caseRoot)
	if err != nil {
		return Instance{}, err
	}
	instancePath := filepath.Join(root.Path, "instance.yml")
	legacyPath := filepath.Join(caseRoot, ".re-template.yml")
	if refsf.Exists(instancePath) {
		source := "steamai"
		if root.Legacy {
			source = "instance"
		}
		return readInstance(caseRoot, instancePath, source, root.Dir)
	}
	if root.Legacy && refsf.Exists(legacyPath) {
		values, err := readScalarFile(legacyPath)
		if err != nil {
			return Instance{}, err
		}
		return Instance{CaseRoot: caseRoot, InstancePath: legacyPath, Source: "legacy", StateDir: StateDirRekit, TemplateRoot: values["templateRoot"], TemplatePack: valueOr(values["templatePack"], defaults.DefaultPack), ProjectName: projectName(caseRoot), ProjectRoot: valueOr(values["currentProjectPath"], caseRoot)}, nil
	}
	return Instance{CaseRoot: caseRoot, InstancePath: instancePath, Source: "missing", StateDir: root.Dir, TemplatePack: defaults.DefaultPack, ProjectName: projectName(caseRoot), ProjectRoot: caseRoot}, nil
}

func readInstance(caseRoot, instancePath, source, stateDir string) (Instance, error) {
	values, err := readScalarFile(instancePath)
	if err != nil {
		return Instance{}, err
	}
	schemaVersion := 1
	if raw := strings.TrimSpace(values["schemaVersion"]); raw != "" {
		if _, err := fmt.Sscanf(raw, "%d", &schemaVersion); err != nil || (schemaVersion != 1 && schemaVersion != 2) {
			return Instance{}, fmt.Errorf("unsupported instance metadata schemaVersion in %s: %s", instancePath, raw)
		}
	}
	anchor := filepath.Dir(instancePath)
	resolvePath := func(key, fallback string) (string, error) {
		value := strings.TrimSpace(valueOr(values[key], fallback))
		if value == "" {
			return "", nil
		}
		if schemaVersion >= 2 && !filepath.IsAbs(value) {
			value = filepath.Join(anchor, filepath.FromSlash(value))
		}
		full, err := filepath.Abs(value)
		if err != nil {
			return "", err
		}
		return filepath.Clean(full), nil
	}
	templateRoot, err := resolvePath("templateRoot", "")
	if err != nil {
		return Instance{}, err
	}
	projectRoot, err := resolvePath("projectRoot", caseRoot)
	if err != nil {
		return Instance{}, err
	}
	bundleRoot, err := resolvePath("bundleRoot", "")
	if err != nil {
		return Instance{}, err
	}
	bundleManifest := strings.TrimSpace(values["bundleManifest"])
	if schemaVersion >= 2 {
		if values["templateRoot"] == "" || filepath.IsAbs(filepath.FromSlash(values["templateRoot"])) || values["projectRoot"] == "" || filepath.IsAbs(filepath.FromSlash(values["projectRoot"])) || values["bundleRoot"] == "" || filepath.IsAbs(filepath.FromSlash(values["bundleRoot"])) {
			return Instance{}, fmt.Errorf("schema v2 project-local metadata paths must be relative to %s", anchor)
		}
		if bundleManifest == "" || filepath.IsAbs(filepath.FromSlash(bundleManifest)) {
			return Instance{}, fmt.Errorf("schema v2 project-local metadata requires a relative bundleManifest")
		}
		cleanManifest := filepath.ToSlash(filepath.Clean(filepath.FromSlash(bundleManifest)))
		if cleanManifest == "." || cleanManifest == ".." || strings.HasPrefix(cleanManifest, "../") {
			return Instance{}, fmt.Errorf("schema v2 bundleManifest escapes its metadata root: %s", bundleManifest)
		}
		bundleManifest = cleanManifest
	}
	return Instance{CaseRoot: caseRoot, InstancePath: instancePath, Source: source, StateDir: stateDir, SchemaVersion: schemaVersion, Mode: strings.TrimSpace(values["mode"]), TemplateRoot: templateRoot, BundleRoot: bundleRoot, BundleManifest: bundleManifest, BundleManifestSHA256: strings.ToLower(strings.TrimSpace(values["bundleManifestSHA256"])), TemplatePack: valueOr(values["templatePack"], defaults.DefaultPack), ProjectName: valueOr(values["projectName"], projectName(caseRoot)), ProjectRoot: projectRoot}, nil
}

func (i Instance) Moved() bool {
	if i.Source == "missing" || strings.TrimSpace(i.ProjectRoot) == "" {
		return false
	}
	recorded, err := filepath.Abs(i.ProjectRoot)
	if err != nil {
		return true
	}
	actual, err := filepath.Abs(i.CaseRoot)
	if err != nil {
		return true
	}
	return !samePath(recorded, actual)
}

func MovedRepairPreviewError(caseRoot, pack string) error {
	pack = strings.TrimSpace(pack)
	if pack == "" {
		pack = defaults.DefaultPack
	}
	return fmt.Errorf("case metadata points to a different directory. Run '/rekit repair -Target %s -Pack %s -WhatIf -Format text' to preview metadata and thin-shim refresh; run repair -Apply only after explicit confirmation", quoteCommandArg(caseRoot), pack)
}

func quoteCommandArg(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
}

func AssertAttached(target, repoRoot, pack string) (Instance, error) {
	return assertAttached(target, repoRoot, pack)
}

func assertAttached(target, repoRoot, pack string) (Instance, error) {
	caseRoot, err := filepath.Abs(target)
	if err != nil {
		return Instance{}, err
	}
	if !refsf.Exists(caseRoot) {
		return Instance{}, fmt.Errorf("case directory does not exist. Use 'rekit init -Target %q' to create a new case", caseRoot)
	}
	inst, err := Read(caseRoot)
	if err != nil {
		return Instance{}, err
	}
	if inst.Source == "missing" {
		return Instance{}, fmt.Errorf("target is not an attached rekit case. Use 'rekit attach -Target %q' or 'rekit init -Target %q' first", caseRoot, caseRoot)
	}
	if inst.Moved() {
		return Instance{}, MovedRepairPreviewError(caseRoot, pack)
	}
	if strings.TrimSpace(inst.TemplateRoot) == "" {
		return Instance{}, fmt.Errorf("missing templateRoot in case metadata: %s", caseRoot)
	}
	expected, _ := filepath.Abs(repoRoot)
	actual, _ := filepath.Abs(inst.TemplateRoot)
	if !samePath(actual, expected) {
		return Instance{}, fmt.Errorf("case is attached to a different templateRoot: %s", inst.TemplateRoot)
	}
	if !strings.EqualFold(inst.TemplatePack, pack) {
		return Instance{}, fmt.Errorf("case is attached to a different templatePack: %s", inst.TemplatePack)
	}
	if err := missionintent.AssertCommittedOrAbsent(caseRoot); err != nil {
		return Instance{}, err
	}
	return inst, nil
}

func samePath(left, right string) bool {
	same, err := refsf.SameExistingPath(left, right)
	return err == nil && same
}

func regularMetadataExists(path string) (bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := refsf.ValidateNoReparseComponents(path); err != nil {
		return false, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return false, fmt.Errorf("case metadata must be a regular non-symlink file: %s", path)
	}
	return true, nil
}

func readScalarFile(path string) (map[string]string, error) {
	if exists, err := regularMetadataExists(path); err != nil {
		return nil, err
	} else if !exists {
		return nil, os.ErrNotExist
	}
	b, err := refsf.ReadStableRegularFileAnchored(filepath.Dir(path), path, "case metadata", 8192)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for line := range strings.SplitSeq(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := convertValue(parts[1])
		if key != "" {
			out[key] = value
		}
	}
	return out, nil
}

func convertValue(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= 2 {
		if (strings.HasPrefix(v, "\"") && strings.HasSuffix(v, "\"")) || (strings.HasPrefix(v, "'") && strings.HasSuffix(v, "'")) {
			return v[1 : len(v)-1]
		}
	}
	return v
}

func projectName(caseRoot string) string {
	return filepath.Base(strings.TrimRight(filepath.Clean(caseRoot), string(filepath.Separator)))
}

func valueOr(value, def string) string {
	if strings.TrimSpace(value) == "" {
		return def
	}
	return value
}
