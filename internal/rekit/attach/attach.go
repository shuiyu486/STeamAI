package attach

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
)

type Options struct {
	ProjectName string
}

type WritePlan struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	Action string `json:"action"`
}

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
	instancePath := filepath.Join(p.CaseRoot, ".rekit", "instance.yml")
	if err := os.MkdirAll(filepath.Dir(instancePath), 0o755); err != nil {
		return ApplyResult{}, err
	}
	if err := os.WriteFile(instancePath, []byte(instanceText(p.CaseRoot, p.RepoRoot, p.Pack, p.ProjectName)), 0o644); err != nil {
		return ApplyResult{}, err
	}

	shimSource := filepath.Join(p.RepoRoot, "rekit", "templates", "case-shim", "SKILL.md")
	shimText, err := os.ReadFile(shimSource)
	if err != nil {
		return ApplyResult{}, fmt.Errorf("missing case shim template: %s", shimSource)
	}
	shimPath := filepath.Join(p.CaseRoot, ".claude", "skills", "rekit", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(shimPath), 0o755); err != nil {
		return ApplyResult{}, err
	}
	if err := os.WriteFile(shimPath, shimText, 0o644); err != nil {
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
		NextSteps:     []string{"attach wrote only .rekit/instance.yml and the case-local thin shim", "run sync/init separately before expecting full case doctor validation to pass"},
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
			return PreviewPlan{}, fmt.Errorf("case metadata points to a different directory. Run 'rekit repair -Target %q -Apply' after confirming the move", caseRoot)
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
	writes := []WritePlan{
		{Path: filepath.Join(caseRoot, ".rekit", "instance.yml"), Kind: "instance-metadata", Action: actionFor(filepath.Join(caseRoot, ".rekit", "instance.yml"))},
		{Path: filepath.Join(caseRoot, ".claude", "skills", "rekit", "SKILL.md"), Kind: "case-local-thin-shim", Action: actionFor(filepath.Join(caseRoot, ".claude", "skills", "rekit", "SKILL.md"))},
	}
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

func instanceText(caseRoot, repoRoot, pack, projectName string) string {
	return "schemaVersion: 1\n" +
		"templateRoot: " + repoRoot + "\n" +
		"templatePack: " + pack + "\n" +
		"projectName: " + projectName + "\n" +
		"projectRoot: " + caseRoot + "\n" +
		"mode: case-local-shim\n"
}

func actionFor(path string) string {
	if refsf.Exists(path) {
		return "refresh"
	}
	return "create"
}

func projectNameFromRoot(caseRoot string) string {
	return filepath.Base(strings.TrimRight(filepath.Clean(caseRoot), string(filepath.Separator)))
}

func samePath(a, b string) bool {
	left, err := filepath.Abs(a)
	if err != nil {
		return false
	}
	right, err := filepath.Abs(b)
	if err != nil {
		return false
	}
	left = strings.TrimRight(filepath.Clean(left), string(filepath.Separator))
	right = strings.TrimRight(filepath.Clean(right), string(filepath.Separator))
	return strings.EqualFold(left, right)
}
