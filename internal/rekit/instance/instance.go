package instance

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
)

type Instance struct {
	CaseRoot     string
	InstancePath string
	Source       string
	TemplateRoot string
	TemplatePack string
	ProjectName  string
	ProjectRoot  string
}

func LooksLikeCase(path string) bool {
	return refsf.Exists(filepath.Join(path, ".rekit", "instance.yml")) || refsf.Exists(filepath.Join(path, ".re-template.yml"))
}

func Read(target string) (Instance, error) {
	caseRoot, err := filepath.Abs(target)
	if err != nil {
		return Instance{}, err
	}
	instancePath := filepath.Join(caseRoot, ".rekit", "instance.yml")
	legacyPath := filepath.Join(caseRoot, ".re-template.yml")
	if refsf.Exists(instancePath) {
		values, err := readScalarFile(instancePath)
		if err != nil {
			return Instance{}, err
		}
		return Instance{CaseRoot: caseRoot, InstancePath: instancePath, Source: "instance", TemplateRoot: values["templateRoot"], TemplatePack: valueOr(values["templatePack"], defaults.DefaultPack), ProjectName: valueOr(values["projectName"], projectName(caseRoot)), ProjectRoot: valueOr(values["projectRoot"], caseRoot)}, nil
	}
	if refsf.Exists(legacyPath) {
		values, err := readScalarFile(legacyPath)
		if err != nil {
			return Instance{}, err
		}
		return Instance{CaseRoot: caseRoot, InstancePath: legacyPath, Source: "legacy", TemplateRoot: values["templateRoot"], TemplatePack: valueOr(values["templatePack"], defaults.DefaultPack), ProjectName: projectName(caseRoot), ProjectRoot: valueOr(values["currentProjectPath"], caseRoot)}, nil
	}
	return Instance{CaseRoot: caseRoot, InstancePath: instancePath, Source: "missing", TemplatePack: defaults.DefaultPack, ProjectName: projectName(caseRoot), ProjectRoot: caseRoot}, nil
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
	return inst, nil
}

func samePath(left, right string) bool {
	left = strings.TrimRight(filepath.Clean(left), string(filepath.Separator))
	right = strings.TrimRight(filepath.Clean(right), string(filepath.Separator))
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func readScalarFile(path string) (map[string]string, error) {
	b, err := os.ReadFile(path)
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
