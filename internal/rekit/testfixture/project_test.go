package testfixture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtimebundle"
)

func TestNewProjectBuildsOnlyLegalCurrentAndLegacyBindings(t *testing.T) {
	for _, fixture := range []struct {
		layout Layout
		dir    string
		source string
		schema int
	}{
		{layout: CurrentProject, dir: projectstate.CurrentDir, source: "steamai", schema: 2},
		{layout: LegacyCase, dir: projectstate.LegacyDir, source: "instance", schema: 1},
	} {
		t.Run(fixture.layout.String(), func(t *testing.T) {
			project := NewProject(t, ProjectOptions{
				Layout: fixture.layout,
				Pack:   "_template",
				Components: Components{
					InitialState:   true,
					ProjectSkill:   true,
					LegacyMetadata: fixture.layout == LegacyCase,
				},
			})
			inst, err := instance.Read(project.CaseRoot)
			if err != nil {
				t.Fatal(err)
			}
			if inst.Source != fixture.source || inst.SchemaVersion != fixture.schema || inst.StateDir != fixture.dir || inst.TemplatePack != "_template" {
				t.Fatalf("unexpected %s instance: %+v", fixture.layout, inst)
			}
			if project.StateRoot != filepath.Join(project.CaseRoot, fixture.dir) || project.InstancePath != filepath.Join(project.StateRoot, "instance.yml") {
				t.Fatalf("unexpected %s paths: %+v", fixture.layout, project)
			}
			if _, err := os.Stat(project.StatePath); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(project.SkillPath); err != nil {
				t.Fatal(err)
			}

			other := projectstate.LegacyDir
			if fixture.layout == LegacyCase {
				other = projectstate.CurrentDir
			}
			if _, err := os.Lstat(filepath.Join(project.CaseRoot, other)); !os.IsNotExist(err) {
				t.Fatalf("%s fixture created the non-selected root: %v", fixture.layout, err)
			}
			legacySkill := filepath.Join(project.CaseRoot, ".claude", "skills", "rekit", "SKILL.md")
			currentSkill := filepath.Join(project.CaseRoot, ".claude", "skills", "steamai", "SKILL.md")
			if fixture.layout == CurrentProject {
				if project.BundleManifestSHA256 == "" || project.RuntimeRepoRoot != project.StateRoot || project.BundleManifestPath == "" || project.ExecutablePath == "" {
					t.Fatalf("current fixture omitted verified bundle binding: %+v", project)
				}
				if _, err := runtimebundle.Validate(project.StateRoot, runtimebundle.ManifestRel, project.BundleManifestSHA256, project.Pack); err != nil {
					t.Fatal(err)
				}
				if _, err := os.Lstat(filepath.Join(project.CaseRoot, ".re-template.yml")); !os.IsNotExist(err) {
					t.Fatalf("current fixture created legacy metadata: %v", err)
				}
				if _, err := os.Lstat(legacySkill); !os.IsNotExist(err) {
					t.Fatalf("current fixture created legacy skill: %v", err)
				}
			} else {
				if project.BundleManifestSHA256 != "" || project.RuntimeRoot != "" || project.ExecutablePath != "" || project.RuntimeRepoRoot != project.SourceRepoRoot {
					t.Fatalf("legacy fixture exposed current runtime binding: %+v", project)
				}
				if _, err := os.Stat(filepath.Join(project.CaseRoot, ".re-template.yml")); err != nil {
					t.Fatal(err)
				}
				if _, err := os.Lstat(currentSkill); !os.IsNotExist(err) {
					t.Fatalf("legacy fixture created current skill: %v", err)
				}
			}
		})
	}
}

func TestValidateProjectOptionsRejectsCrossLayoutComponents(t *testing.T) {
	for _, fixture := range []struct {
		name string
		opt  ProjectOptions
	}{
		{
			name: "current legacy metadata",
			opt: ProjectOptions{
				Layout:     CurrentProject,
				Components: Components{LegacyMetadata: true},
			},
		},
		{
			name: "legacy current executable",
			opt: ProjectOptions{
				Layout:           LegacyCase,
				ExecutableSource: filepath.Join(t.TempDir(), runtimebundle.ExecutableName()),
			},
		},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			if err := validateProjectOptions(fixture.opt); err == nil {
				t.Fatalf("cross-layout fixture was accepted: %+v", fixture.opt)
			}
		})
	}
}

func TestCurrentStateUsesRelocatableTemplateRoot(t *testing.T) {
	project := NewProject(t, ProjectOptions{
		Layout:     CurrentProject,
		Pack:       "_template",
		Components: Components{InitialState: true},
	})
	data, err := os.ReadFile(project.StatePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"templateRoot": "."`) || strings.Contains(string(data), project.SourceRepoRoot) {
		t.Fatalf("current state is not relocatable: %s", data)
	}
}
