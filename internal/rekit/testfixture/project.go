package testfixture

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/casebind"
	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtimebundle"
)

type Layout uint8

const (
	CurrentProject Layout = iota
	LegacyCase
)

type Components struct {
	InitialState   bool
	ProjectSkill   bool
	LegacyMetadata bool
}

type ProjectOptions struct {
	Layout           Layout
	SourceRepo       string
	CaseRoot         string
	Pack             string
	ProjectName      string
	ExecutableSource string
	Components       Components
}

type Project struct {
	Layout               Layout
	CaseRoot             string
	StateRoot            string
	SourceRepoRoot       string
	RuntimeRepoRoot      string
	Pack                 string
	ProjectName          string
	InstancePath         string
	StatePath            string
	SkillPath            string
	RuntimeRoot          string
	ExecutablePath       string
	BundleManifestPath   string
	BundleManifestSHA256 string
}

func validateProjectOptions(opt ProjectOptions) error {
	if opt.Layout != CurrentProject && opt.Layout != LegacyCase {
		return fmt.Errorf("unsupported project fixture layout: %d", opt.Layout)
	}
	if opt.Layout == CurrentProject && opt.Components.LegacyMetadata {
		return fmt.Errorf("current project fixture cannot publish legacy metadata")
	}
	if opt.Layout == LegacyCase && strings.TrimSpace(opt.ExecutableSource) != "" {
		return fmt.Errorf("legacy case fixture cannot publish a current runtime executable")
	}
	return nil
}

func NewProject(t testing.TB, opt ProjectOptions) Project {
	t.Helper()
	if err := validateProjectOptions(opt); err != nil {
		t.Fatal(err)
	}

	sourceRepo := strings.TrimSpace(opt.SourceRepo)
	if sourceRepo == "" {
		sourceRepo = repositoryRoot(t)
	}
	sourceRepo = absolutePath(t, sourceRepo, "source repository")
	caseRoot := strings.TrimSpace(opt.CaseRoot)
	if caseRoot == "" {
		caseRoot = t.TempDir()
	}
	caseRoot = absolutePath(t, caseRoot, "case root")
	if err := os.MkdirAll(caseRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	pack := strings.TrimSpace(opt.Pack)
	if pack == "" {
		pack = defaults.DefaultPack
	}
	projectName := strings.TrimSpace(opt.ProjectName)
	if projectName == "" {
		projectName = "test-project"
	}

	project := Project{
		Layout: opt.Layout, CaseRoot: caseRoot, SourceRepoRoot: sourceRepo,
		Pack: pack, ProjectName: projectName,
	}
	if opt.Layout == CurrentProject {
		buildCurrentProject(t, &project, opt)
	} else {
		buildLegacyCase(t, &project, opt)
	}
	return project
}

func buildCurrentProject(t testing.TB, project *Project, opt ProjectOptions) {
	t.Helper()
	project.StateRoot = filepath.Join(project.CaseRoot, projectstate.CurrentDir)
	project.RuntimeRepoRoot = project.StateRoot
	project.RuntimeRoot = filepath.Join(project.StateRoot, "runtime")

	executableSource := strings.TrimSpace(opt.ExecutableSource)
	if executableSource == "" {
		executableSource = filepath.Join(t.TempDir(), runtimebundle.ExecutableName())
		writeFile(t, executableSource, []byte("STeamAI fixture runtime\n"), 0o755)
	} else {
		executableSource = absolutePath(t, executableSource, "fixture executable")
	}
	bundle, err := runtimebundle.PublishForTest(project.CaseRoot, project.SourceRepoRoot, project.Pack, executableSource)
	if err != nil {
		t.Fatal(err)
	}
	project.BundleManifestSHA256 = bundle.ManifestSHA256
	project.BundleManifestPath = filepath.Join(project.StateRoot, filepath.FromSlash(runtimebundle.ManifestRel))
	project.ExecutablePath = filepath.Join(project.StateRoot, filepath.FromSlash(bundle.Manifest.Executable.Path))
	project.InstancePath = filepath.Join(project.StateRoot, "instance.yml")
	writeFile(t, project.InstancePath, []byte(casebind.STeamAIInstanceText(
		project.CaseRoot,
		project.Pack,
		project.ProjectName,
		runtimebundle.ManifestRel,
		bundle.ManifestSHA256,
	)), 0o644)
	if _, err := runtimebundle.Validate(project.StateRoot, runtimebundle.ManifestRel, bundle.ManifestSHA256, project.Pack); err != nil {
		t.Fatalf("validate current project fixture bundle: %v", err)
	}
	if opt.Components.InitialState {
		project.StatePath = writeInitialState(t, project.StateRoot, ".", project.Pack)
	}
	if opt.Components.ProjectSkill {
		project.SkillPath, err = casebind.WriteCanonicalCaseShim(project.CaseRoot, project.SourceRepoRoot)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func buildLegacyCase(t testing.TB, project *Project, opt ProjectOptions) {
	t.Helper()
	project.StateRoot = filepath.Join(project.CaseRoot, projectstate.LegacyDir)
	project.RuntimeRepoRoot = project.SourceRepoRoot
	project.InstancePath = filepath.Join(project.StateRoot, "instance.yml")
	writeFile(t, project.InstancePath, []byte(casebind.InstanceText(
		project.CaseRoot,
		project.SourceRepoRoot,
		project.Pack,
		project.ProjectName,
	)), 0o644)
	var err error
	if opt.Components.LegacyMetadata {
		_, err = casebind.WriteLegacyMetadata(project.CaseRoot, project.SourceRepoRoot, project.Pack)
		if err != nil {
			t.Fatal(err)
		}
	}
	if opt.Components.InitialState {
		project.StatePath = writeInitialState(t, project.StateRoot, project.SourceRepoRoot, project.Pack)
	}
	if opt.Components.ProjectSkill {
		project.SkillPath, err = casebind.WriteCanonicalCaseShim(project.CaseRoot, project.SourceRepoRoot)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func writeInitialState(t testing.TB, stateRoot, templateRoot, pack string) string {
	t.Helper()
	state := casebind.InitialState{
		SchemaVersion: 1,
		TemplateRoot:  templateRoot,
		TemplatePack:  pack,
		Managed:       map[string]struct{}{},
		Promote:       casebind.InitialPromoteState{Candidates: []string{}},
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(stateRoot, "state.json")
	writeFile(t, path, append(data, '\n'), 0o644)
	return path
}

func repositoryRoot(t testing.TB) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve fixture repository root")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

func absolutePath(t testing.TB, path, label string) string {
	t.Helper()
	full, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("resolve %s: %v", label, err)
	}
	return filepath.Clean(full)
}

func writeFile(t testing.TB, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if len(data) == 0 {
		t.Fatalf("fixture file is empty: %s", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatal(err)
	}
}

func (layout Layout) String() string {
	switch layout {
	case CurrentProject:
		return "current"
	case LegacyCase:
		return "legacy"
	default:
		return fmt.Sprintf("layout-%d", layout)
	}
}
