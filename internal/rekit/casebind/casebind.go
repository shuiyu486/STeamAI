package casebind

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
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
	instancePath := filepath.Join(caseRoot, ".rekit", "instance.yml")
	shimPath := filepath.Join(caseRoot, ".claude", "skills", "rekit", "SKILL.md")
	return []WritePlan{
		{Path: instancePath, Kind: "instance-metadata", Action: ActionFor(instancePath)},
		{Path: shimPath, Kind: "case-local-thin-shim", Action: ActionFor(shimPath)},
	}
}

func LegacyWrite(caseRoot string) WritePlan {
	path := filepath.Join(caseRoot, ".re-template.yml")
	return WritePlan{Path: path, Kind: "legacy-metadata", Action: ActionFor(path)}
}

func InitialStateWrite(caseRoot string) WritePlan {
	path := filepath.Join(caseRoot, ".rekit", "state.json")
	action := "create"
	if refsf.Exists(path) {
		action = "unchanged"
	}
	return WritePlan{Path: path, Kind: "initial-state", Action: action}
}

func WriteInstance(caseRoot, repoRoot, pack, projectName string) (string, error) {
	instancePath := filepath.Join(caseRoot, ".rekit", "instance.yml")
	if err := os.MkdirAll(filepath.Dir(instancePath), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(instancePath, []byte(InstanceText(caseRoot, repoRoot, pack, projectName)), 0o644); err != nil {
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
	shimSource := filepath.Join(repoRoot, "rekit", "templates", "case-shim", "SKILL.md")
	var (
		shimText []byte
		err      error
	)
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
	statePath := filepath.Join(caseRoot, ".rekit", "state.json")
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
