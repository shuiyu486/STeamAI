package casebind

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/sourceartifact"
)

type WritePlan struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	Action string `json:"action"`
}

type InitialState struct {
	SchemaVersion int                 `json:"schemaVersion"`
	TemplateRoot  string              `json:"templateRoot"`
	TemplatePack  string              `json:"templatePack"`
	Managed       map[string]struct{} `json:"managed"`
	Promote       InitialPromoteState `json:"promote"`
}

type InitialPromoteState struct {
	Candidates []string `json:"candidates"`
}

func BindingWrites(caseRoot string) []WritePlan {
	root, err := projectstate.Resolve(caseRoot)
	if err != nil {
		return []WritePlan{{Path: caseRoot, Kind: "state-root-conflict", Action: "blocked"}}
	}
	instancePath := filepath.Join(root.Path, "instance.yml")
	skill := "steamai"
	kind := "project-local-steamai-skill"
	if root.Legacy {
		skill = "rekit"
		kind = "case-local-thin-shim"
	}
	shimPath := filepath.Join(caseRoot, ".claude", "skills", skill, "SKILL.md")
	return []WritePlan{
		{Path: instancePath, Kind: "instance-metadata", Action: ActionFor(instancePath)},
		{Path: shimPath, Kind: kind, Action: ActionFor(shimPath)},
	}
}

func LegacyWrite(caseRoot string) WritePlan {
	path := filepath.Join(caseRoot, ".re-template.yml")
	return WritePlan{Path: path, Kind: "legacy-metadata", Action: ActionFor(path)}
}

func InitialStateWrite(caseRoot string) WritePlan {
	root, err := projectstate.Resolve(caseRoot)
	if err != nil {
		return WritePlan{Path: caseRoot, Kind: "state-root-conflict", Action: "blocked"}
	}
	path := filepath.Join(root.Path, "state.json")
	action := "create"
	if refsf.Exists(path) {
		action = "unchanged"
	}
	return WritePlan{Path: path, Kind: "initial-state", Action: action}
}

func WriteInstance(caseRoot, repoRoot, pack, projectName string) (string, error) {
	root, err := projectstate.Resolve(caseRoot)
	if err != nil {
		return "", err
	}
	instancePath := filepath.Join(root.Path, "instance.yml")
	if !root.Legacy {
		return "", fmt.Errorf("current STeamAI binding requires init to publish a manifest-bound project-local runtime bundle: %s", instancePath)
	}
	if err := os.MkdirAll(filepath.Dir(instancePath), 0o755); err != nil {
		return "", err
	}
	instanceText := InstanceText(caseRoot, repoRoot, pack, projectName)
	if err := os.WriteFile(instancePath, []byte(instanceText), 0o644); err != nil {
		return "", err
	}
	if _, err := WriteLegacyMetadataForAttach(caseRoot, repoRoot, pack); err != nil {
		return "", err
	}
	if _, err := WriteInitialState(caseRoot, repoRoot, pack); err != nil {
		return "", err
	}
	return instancePath, nil
}

func WriteCaseShim(caseRoot, repoRoot string) (string, error) {
	return writeCaseShim(caseRoot, repoRoot, false)
}

func WriteCanonicalCaseShim(caseRoot, repoRoot string) (string, error) {
	return writeCaseShim(caseRoot, repoRoot, true)
}

func writeCaseShim(caseRoot, repoRoot string, canonical bool) (string, error) {
	root, err := projectstate.Resolve(caseRoot)
	if err != nil {
		return "", err
	}
	if !root.Legacy {
		return writeSTeamAISkill(caseRoot, repoRoot, canonical)
	}
	shimSource := filepath.Join(repoRoot, "rekit", "templates", "case-shim", "SKILL.md")
	var shimText []byte
	if canonical {
		shimText, err = sourceartifact.ReadCanonical(shimSource)
	} else {
		shimText, err = os.ReadFile(shimSource)
	}
	if err != nil {
		return "", fmt.Errorf("missing case shim template: %s", shimSource)
	}
	shimPath := filepath.Join(caseRoot, ".claude", "skills", "rekit", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(shimPath), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(shimPath, shimText, 0o644); err != nil {
		return "", err
	}
	return shimPath, nil
}

func writeSTeamAISkill(caseRoot, repoRoot string, canonical bool) (string, error) {
	source := filepath.Join(repoRoot, "rekit", "templates", "steamai-project", "SKILL.md")
	var (
		text []byte
		err  error
	)
	if canonical {
		text, err = sourceartifact.ReadCanonical(source)
	} else {
		text, err = os.ReadFile(source)
	}
	if err != nil {
		return "", fmt.Errorf("missing project-local STeamAI skill: %s", source)
	}
	path := filepath.Join(caseRoot, ".claude", "skills", "steamai", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, text, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func WriteLegacyMetadata(caseRoot, repoRoot, pack string) (string, error) {
	legacyPath := filepath.Join(caseRoot, ".re-template.yml")
	if err := SetYAMLScalar(legacyPath, "templateRoot", repoRoot); err != nil {
		return "", err
	}
	if err := SetYAMLScalar(legacyPath, "rekitMode", "case-local-shim"); err != nil {
		return "", err
	}
	if err := SetYAMLScalar(legacyPath, "currentProjectPath", caseRoot); err != nil {
		return "", err
	}
	if err := SetYAMLScalar(legacyPath, "templatePack", pack); err != nil {
		return "", err
	}
	if err := ensureYAMLScalar(legacyPath, "templateVersion", "0.0.0"); err != nil {
		return "", err
	}
	return legacyPath, nil
}

func WriteLegacyMetadataForAttach(caseRoot, repoRoot, pack string) (string, error) {
	legacyPath := filepath.Join(caseRoot, ".re-template.yml")
	legacyExists := refsf.Exists(legacyPath)
	if err := SetYAMLScalar(legacyPath, "templateRoot", repoRoot); err != nil {
		return "", err
	}
	if err := SetYAMLScalar(legacyPath, "rekitMode", "case-local-shim"); err != nil {
		return "", err
	}
	if !legacyExists {
		if err := SetYAMLScalar(legacyPath, "templatePack", pack); err != nil {
			return "", err
		}
		if err := SetYAMLScalar(legacyPath, "templateVersion", "0.0.0"); err != nil {
			return "", err
		}
	}
	return legacyPath, nil
}

func WriteInitialState(caseRoot, repoRoot, pack string) (string, error) {
	statePath, err := projectstate.Join(caseRoot, "state.json")
	if err != nil {
		return "", err
	}
	if refsf.Exists(statePath) {
		return statePath, nil
	}
	state := InitialState{
		SchemaVersion: 1,
		TemplateRoot:  repoRoot,
		TemplatePack:  pack,
		Managed:       map[string]struct{}{},
		Promote:       InitialPromoteState{Candidates: []string{}},
	}
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(statePath, append(b, '\n'), 0o644); err != nil {
		return "", err
	}
	return statePath, nil
}

func SetYAMLScalar(path, key, value string) error {
	var text string
	if b, err := os.ReadFile(path); err == nil {
		text = string(b)
	} else if !os.IsNotExist(err) {
		return err
	}
	return writeYAMLScalar(path, key, value, text, true)
}

func ensureYAMLScalar(path, key, value string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := string(b)
	if hasYAMLKey(text, key) {
		return nil
	}
	return writeYAMLScalar(path, key, value, text, false)
}

func writeYAMLScalar(path, key, value, text string, replace bool) error {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	updated := false
	for i, line := range lines {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[0]) == key {
			if replace {
				lines[i] = key + ": " + value
			}
			updated = true
		}
	}
	if !updated {
		if strings.TrimSpace(text) == "" {
			lines = []string{key + ": " + value}
		} else {
			for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
				lines = lines[:len(lines)-1]
			}
			lines = append(lines, key+": "+value)
		}
	}
	out := strings.Join(lines, "\r\n")
	if !strings.HasSuffix(out, "\r\n") {
		out += "\r\n"
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(out), 0o644)
}

func hasYAMLKey(text, key string) bool {
	for line := range strings.SplitSeq(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[0]) == key {
			return true
		}
	}
	return false
}

func InstanceText(caseRoot, repoRoot, pack, projectName string) string {
	return "schemaVersion: 1\n" +
		"templateRoot: " + repoRoot + "\n" +
		"templatePack: " + pack + "\n" +
		"projectName: " + projectName + "\n" +
		"projectRoot: " + caseRoot + "\n" +
		"mode: case-local-shim\n"
}

func STeamAIInstanceText(_ string, pack, projectName string, bundle ...string) string {
	bundleManifest := "runtime/manifest.json"
	bundleSHA256 := ""
	if len(bundle) > 0 && strings.TrimSpace(bundle[0]) != "" {
		bundleManifest = filepath.ToSlash(filepath.Clean(filepath.FromSlash(strings.TrimSpace(bundle[0]))))
	}
	if len(bundle) > 1 {
		bundleSHA256 = strings.ToLower(strings.TrimSpace(bundle[1]))
	}
	text := "schemaVersion: 2\n" +
		"brand: STeamAI\n" +
		"stateNamespace: steamai\n" +
		"templateRoot: .\n" +
		"bundleRoot: runtime\n" +
		"bundleManifest: " + bundleManifest + "\n"
	if bundleSHA256 != "" {
		text += "bundleManifestSHA256: " + bundleSHA256 + "\n"
	}
	return text +
		"templatePack: " + pack + "\n" +
		"projectName: " + projectName + "\n" +
		"projectRoot: ..\n" +
		"mode: project-local-bundle\n"
}

func ActionFor(path string) string {
	if refsf.Exists(path) {
		return "refresh"
	}
	return "create"
}

func ProjectNameFromRoot(caseRoot string) string {
	return filepath.Base(strings.TrimRight(filepath.Clean(caseRoot), string(filepath.Separator)))
}

func SamePath(a, b string) bool {
	return refsf.SamePath(a, b)
}

func SameExistingPath(a, b string) bool {
	same, err := refsf.SameExistingPath(a, b)
	return err == nil && same
}
