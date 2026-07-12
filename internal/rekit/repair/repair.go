package repair

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/casebind"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
)

type Options struct {
	ProjectName string
}

type Plan struct {
	SchemaVersion        int                  `json:"schemaVersion"`
	Command              string               `json:"command"`
	CaseRoot             string               `json:"caseRoot"`
	RepoRoot             string               `json:"repoRoot"`
	Pack                 string               `json:"pack"`
	ProjectName          string               `json:"projectName"`
	IsMutation           bool                 `json:"isMutation"`
	ReviewRequired       bool                 `json:"reviewRequired"`
	RequiresConfirmation bool                 `json:"requiresConfirmation"`
	MetadataSource       string               `json:"metadataSource"`
	RecordedProjectRoot  string               `json:"recordedProjectRoot"`
	NewProjectRoot       string               `json:"newProjectRoot"`
	Moved                bool                 `json:"moved"`
	Writes               []casebind.WritePlan `json:"writes"`
	BlockedActions       []string             `json:"blockedActions"`
	NextSteps            []string             `json:"nextSteps"`
}

type ApplyResult struct {
	SchemaVersion       int                  `json:"schemaVersion"`
	Command             string               `json:"command"`
	CaseRoot            string               `json:"caseRoot"`
	RepoRoot            string               `json:"repoRoot"`
	Pack                string               `json:"pack"`
	ProjectName         string               `json:"projectName"`
	IsMutation          bool                 `json:"isMutation"`
	Applied             bool                 `json:"applied"`
	MetadataSource      string               `json:"metadataSource"`
	RecordedProjectRoot string               `json:"recordedProjectRoot"`
	NewProjectRoot      string               `json:"newProjectRoot"`
	Moved               bool                 `json:"moved"`
	Writes              []casebind.WritePlan `json:"writes"`
	NextSteps           []string             `json:"nextSteps"`
}

func Preview(repoRoot, target, pack string, opt Options) (Plan, error) {
	p, err := buildPlan(repoRoot, target, pack, opt)
	if err != nil {
		return Plan{}, err
	}
	p.IsMutation = false
	p.ReviewRequired = true
	p.RequiresConfirmation = true
	p.NextSteps = []string{"review this plan, then re-run repair with -Apply to refresh metadata and the thin shim"}
	return p, nil
}

func Apply(repoRoot, target, pack string, opt Options) (ApplyResult, error) {
	p, err := buildPlan(repoRoot, target, pack, opt)
	if err != nil {
		return ApplyResult{}, err
	}
	if err := os.MkdirAll(p.CaseRoot, 0o755); err != nil {
		return ApplyResult{}, err
	}
	if _, err := casebind.WriteInstance(p.CaseRoot, p.RepoRoot, p.Pack, p.ProjectName); err != nil {
		return ApplyResult{}, err
	}
	if _, err := casebind.WriteCaseShim(p.CaseRoot, p.RepoRoot); err != nil {
		return ApplyResult{}, err
	}
	if _, err := casebind.WriteLegacyMetadata(p.CaseRoot, p.RepoRoot, p.Pack); err != nil {
		return ApplyResult{}, err
	}
	return ApplyResult{
		SchemaVersion:       1,
		Command:             "repair",
		CaseRoot:            p.CaseRoot,
		RepoRoot:            p.RepoRoot,
		Pack:                p.Pack,
		ProjectName:         p.ProjectName,
		IsMutation:          true,
		Applied:             true,
		MetadataSource:      p.MetadataSource,
		RecordedProjectRoot: p.RecordedProjectRoot,
		NewProjectRoot:      p.NewProjectRoot,
		Moved:               p.Moved,
		Writes:              p.Writes,
		NextSteps:           []string{"repair refreshed metadata and the case-local thin shim", "run doctor to validate the attached case"},
	}, nil
}

func buildPlan(repoRoot, target, pack string, opt Options) (Plan, error) {
	caseRoot, err := filepath.Abs(target)
	if err != nil {
		return Plan{}, err
	}
	repoFull, err := filepath.Abs(repoRoot)
	if err != nil {
		return Plan{}, err
	}
	if casebind.SamePath(caseRoot, repoFull) {
		return Plan{}, fmt.Errorf("repair target must be an external case directory, not the kit repo root: %s", caseRoot)
	}
	if _, err := manifest.Load(repoFull, pack); err != nil {
		return Plan{}, err
	}
	inst, err := instance.Read(caseRoot)
	if err != nil {
		return Plan{}, err
	}
	if inst.Source == "missing" {
		return Plan{}, fmt.Errorf("target is not an attached rekit case. Use 'rekit attach -Target %q' or 'rekit init -Target %q' first", caseRoot, caseRoot)
	}
	if strings.TrimSpace(inst.TemplateRoot) == "" {
		return Plan{}, fmt.Errorf("missing templateRoot in case metadata: %s", caseRoot)
	}
	if !casebind.SamePath(inst.TemplateRoot, repoFull) {
		return Plan{}, fmt.Errorf("case is attached to a different templateRoot: %s", inst.TemplateRoot)
	}
	if strings.TrimSpace(inst.TemplatePack) != "" && !strings.EqualFold(inst.TemplatePack, pack) {
		return Plan{}, fmt.Errorf("case is attached to a different templatePack: %s", inst.TemplatePack)
	}
	projectName := strings.TrimSpace(opt.ProjectName)
	if projectName == "" {
		projectName = strings.TrimSpace(inst.ProjectName)
	}
	if projectName == "" {
		projectName = casebind.ProjectNameFromRoot(caseRoot)
	}
	writes := append([]casebind.WritePlan{}, casebind.BindingWrites(caseRoot)...)
	writes = append(writes, casebind.LegacyWrite(caseRoot), casebind.InitialStateWrite(caseRoot))
	return Plan{
		SchemaVersion:       1,
		Command:             "repair",
		CaseRoot:            caseRoot,
		RepoRoot:            repoFull,
		Pack:                pack,
		ProjectName:         projectName,
		MetadataSource:      inst.Source,
		RecordedProjectRoot: inst.ProjectRoot,
		NewProjectRoot:      caseRoot,
		Moved:               inst.Moved(),
		Writes:              writes,
		BlockedActions:      []string{"managed docs sync", "board/facts/lanes mutation", "authority writes", "pack writes"},
	}, nil
}
