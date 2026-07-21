package attach

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

type WritePlan = casebind.WritePlan

type PreviewPlan struct {
	SchemaVersion        int         `json:"schemaVersion"`
	Command              string      `json:"command"`
	CaseRoot             string      `json:"caseRoot"`
	RepoRoot             string      `json:"repoRoot"`
	Pack                 string      `json:"pack"`
	ProjectName          string      `json:"projectName"`
	IsMutation           bool        `json:"isMutation"`
	ReviewRequired       bool        `json:"reviewRequired"`
	RequiresConfirmation bool        `json:"requiresConfirmation"`
	Writes               []WritePlan `json:"writes"`
	BlockedActions       []string    `json:"blockedActions"`
	NextSteps            []string    `json:"nextSteps"`
}

type ApplyResult struct {
	SchemaVersion int         `json:"schemaVersion"`
	Command       string      `json:"command"`
	CaseRoot      string      `json:"caseRoot"`
	RepoRoot      string      `json:"repoRoot"`
	Pack          string      `json:"pack"`
	ProjectName   string      `json:"projectName"`
	IsMutation    bool        `json:"isMutation"`
	Applied       bool        `json:"applied"`
	Writes        []WritePlan `json:"writes"`
	NextSteps     []string    `json:"nextSteps"`
}

func Preview(repoRoot, target, pack string, opt Options) (PreviewPlan, error) {
	p, err := buildPlan(repoRoot, target, pack, opt)
	if err != nil {
		return PreviewPlan{}, err
	}
	p.IsMutation = false
	p.ReviewRequired = true
	p.RequiresConfirmation = true
	p.NextSteps = []string{"review this plan, then re-run attach with -Apply to write metadata and the thin shim"}
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

	return ApplyResult{
		SchemaVersion: 1,
		Command:       "attach",
		CaseRoot:      p.CaseRoot,
		RepoRoot:      p.RepoRoot,
		Pack:          p.Pack,
		ProjectName:   p.ProjectName,
		IsMutation:    true,
		Applied:       true,
		Writes:        p.Writes,
		NextSteps:     []string{"attach wrote only case binding metadata, initial state, and the case-local thin shim", "run sync/init separately before expecting full case doctor validation to pass"},
	}, nil
}

func buildPlan(repoRoot, target, pack string, opt Options) (PreviewPlan, error) {
	caseRoot, err := filepath.Abs(target)
	if err != nil {
		return PreviewPlan{}, err
	}
	repoFull, err := filepath.Abs(repoRoot)
	if err != nil {
		return PreviewPlan{}, err
	}
	if samePath(caseRoot, repoFull) {
		return PreviewPlan{}, fmt.Errorf("attach target must be an external case directory, not the kit repo root: %s", caseRoot)
	}
	if _, err := manifest.Load(repoFull, pack); err != nil {
		return PreviewPlan{}, err
	}
	inst, err := instance.Read(caseRoot)
	if err != nil {
		return PreviewPlan{}, err
	}
	if inst.Source != "missing" {
		if inst.Moved() {
			return PreviewPlan{}, instance.MovedRepairPreviewError(caseRoot, pack)
		}
		if strings.TrimSpace(inst.TemplateRoot) != "" && !samePath(inst.TemplateRoot, repoFull) {
			return PreviewPlan{}, fmt.Errorf("case is attached to a different templateRoot: %s", inst.TemplateRoot)
		}
		if strings.TrimSpace(inst.TemplatePack) != "" && !strings.EqualFold(inst.TemplatePack, pack) {
			return PreviewPlan{}, fmt.Errorf("case is attached to a different templatePack: %s", inst.TemplatePack)
		}
	}
	projectName := strings.TrimSpace(opt.ProjectName)
	if projectName == "" {
		projectName = projectNameFromRoot(caseRoot)
	}
	writes := append([]casebind.WritePlan{}, casebind.BindingWrites(caseRoot)...)
	writes = append(writes, casebind.LegacyWrite(caseRoot), casebind.InitialStateWrite(caseRoot))
	return PreviewPlan{
		SchemaVersion:  1,
		Command:        "attach",
		CaseRoot:       caseRoot,
		RepoRoot:       repoFull,
		Pack:           pack,
		ProjectName:    projectName,
		Writes:         writes,
		BlockedActions: []string{"managed docs sync", "board/facts/lanes initialization", "review artifact writes", "pack writes"},
	}, nil
}

func projectNameFromRoot(caseRoot string) string {
	return casebind.ProjectNameFromRoot(caseRoot)
}

func samePath(a, b string) bool {
	return casebind.SamePath(a, b)
}
