//go:generate go run ../../../cmd/skillcontractgen -repo ../../..

package skillcontract

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/sourceartifact"
)

const (
	CanonicalSkillPath  = ".claude/skills/steamai/SKILL.md"
	ProjectTemplatePath = "rekit/templates/steamai-project/SKILL.md"

	MachineAppendixStart = "<!-- steamai:machine-contract:start -->"
	MachineAppendixEnd   = "<!-- steamai:machine-contract:end -->"
)

type Capability struct {
	ID                   string   `json:"id"`
	Audience             string   `json:"audience"`
	Surface              string   `json:"surface"`
	Command              string   `json:"command,omitempty"`
	Mode                 string   `json:"mode,omitempty"`
	Argv                 []string `json:"argv"`
	Effect               string   `json:"effect"`
	Policy               string   `json:"policy"`
	Currentness          string   `json:"currentness,omitempty"`
	ExpectedApplyFlag    string   `json:"expectedApplyFlag,omitempty"`
	ExactApplyFromResult bool     `json:"exactApplyFromResult,omitempty"`
	Notes                string   `json:"notes,omitempty"`
}

func Capabilities() ([]Capability, error) {
	capabilities := []Capability{
		{
			ID:       "public-help",
			Audience: "user",
			Surface:  "public",
			Argv:     []string{"help"},
			Effect:   "read-only",
			Policy:   commands.BoundaryReadOnly,
			Notes:    "shows the bounded no-mode public surface",
		},
	}
	for _, input := range []struct {
		id       string
		audience string
		surface  string
		command  string
		mode     string
		argv     []string
		effect   string
		exact    bool
		notes    string
	}{
		{
			id: "public-status", audience: "user", surface: "public",
			command: commands.Status, mode: commands.MutationModeDefault,
			argv: []string{"status"}, effect: "read-only",
			notes: "publishes only now/reason/next by default",
		},
		{
			id: "public-status-diagnostics", audience: "mission-commander", surface: "public",
			command: commands.Status, mode: commands.MutationModeDefault,
			argv: []string{"status", "--diagnostics"}, effect: "read-only",
			notes: "publishes full typed diagnostics on demand",
		},
		{
			id: "public-continue-preview", audience: "user", surface: "public",
			command: commands.Continue, mode: commands.MutationModeDefault,
			argv: []string{"continue", "--lane", "<SELECTOR>"}, effect: "preview-only", exact: true,
			notes: "--lane is optional when fresh typed state resolves one lane",
		},
		{
			id: "runtime-status-compact", audience: "mission-commander", surface: "runtime",
			command: commands.Status, mode: commands.MutationModeDefault,
			argv: []string{"runtime", "-Command", commands.Status, "-Target", "${CLAUDE_PROJECT_DIR}", "-Format", "compact-json"}, effect: "read-only",
			notes: "zero-write typed status used by the ordinary interaction owner",
		},
		{
			id: "runtime-control-preview", audience: "mission-commander", surface: "runtime",
			command: commands.Control, mode: commands.MutationModeDefault,
			argv: []string{"runtime", "-Command", commands.Control, "-Target", "${CLAUDE_PROJECT_DIR}", "-Lane", "<TYPED_LANE>", "-Action", "<pause|resume|stop>", "-Actor", "<ACTOR>", "-Reason", "<REASON>", "-WhatIf", "-Format", "json"}, effect: "review-first-preview", exact: true,
			notes: "Apply only the exact typed action returned by this preview",
		},
		{
			id: "runtime-bounded-autonomy-preview", audience: "mission-commander", surface: "runtime",
			command: commands.Gate, mode: commands.MutationModeProfile,
			argv: []string{"runtime", "-Command", commands.Gate, "-ProvisionProfile", "-ProfilePreset", "bounded-autonomous-v1", "-ProfileExplicitOptIn", "-Lane", "<TYPED_LANE>", "-Action", "<EXACT_ACTIONS>", "-TargetRef", "<EXACT_TARGETS>", "-RuntimeSeconds", "<SECONDS>", "-DiskMB", "<MB>", "-Requests", "<COUNT>", "-StopConditions", "<STOP_CONDITIONS>", "-OutputPaths", "<CASE_RELATIVE_OUTPUTS>", "-ProfileGrantedBy", "<ACTOR>", "-ProfileGrantedAt", "<RFC3339>", "-ProfileExpiresAt", "<RFC3339>", "-Format", "json"}, effect: "review-first-preview", exact: true,
			notes: "network actions also require -ProfileExternalTargetScope equal to the exact targets",
		},
		{
			id: "runtime-revoke-autonomy-preview", audience: "mission-commander", surface: "runtime",
			command: commands.Gate, mode: commands.MutationModeProfile,
			argv: []string{"runtime", "-Command", commands.Gate, "-RevokeProfile", "-Lane", "<TYPED_LANE>", "-Format", "json"}, effect: "review-first-preview", exact: true,
			notes: "revocation uses the same exact preview and Apply discipline",
		},
	} {
		capability, err := scopedCapability(input.id, input.audience, input.surface, input.command, input.mode, input.argv, input.effect, input.exact, input.notes)
		if err != nil {
			return nil, err
		}
		capabilities = append(capabilities, capability)
	}
	capabilities = append(capabilities,
		Capability{
			ID: "host-daily-goal", Audience: "mission-commander", Surface: "host",
			Argv:   []string{"host", "-daily", "-target", "${CLAUDE_PROJECT_DIR}", "-goal", "<GOAL>"},
			Effect: "typed-daily-owner", Policy: "daily-operation-classifier",
			Notes: "starts only from a fresh goal operation selected by the daily classifier",
		},
		Capability{
			ID: "host-daily-resume", Audience: "mission-commander", Surface: "host",
			Argv:   []string{"host", "-daily", "-target", "${CLAUDE_PROJECT_DIR}", "-lane", "<TYPED_LANE>"},
			Effect: "typed-daily-owner", Policy: "daily-operation-classifier",
			Notes: "-lane selects a fresh typed lane and never implies control intent",
		},
		Capability{
			ID: "host-daily-correction", Audience: "mission-commander", Surface: "host",
			Argv:   []string{"host", "-daily", "-target", "${CLAUDE_PROJECT_DIR}", "-lane", "<TYPED_LANE>", "-correction", "<CORRECTION>"},
			Effect: "typed-daily-owner", Policy: "daily-operation-classifier",
			Notes: "passes the user's correction verbatim to the correction owner",
		},
		Capability{
			ID: "typed-invocation", Audience: "mission-commander", Surface: "runtime",
			Argv:   []string{"runtime", "-Command", "<invocation.command>", "<invocation.arguments...>"},
			Effect: "typed-request-only", Policy: "validated-current-driver-request",
			Notes: "execute only a fresh executable unblocked request whose command and expected receipt are equivalent to its typed invocation",
		},
	)
	if err := validateCapabilities(capabilities); err != nil {
		return nil, err
	}
	return cloneCapabilities(capabilities), nil
}

func scopedCapability(id, audience, surface, command, mode string, argv []string, effect string, exact bool, notes string) (Capability, error) {
	descriptor, ok := commands.ScopedCommandDescriptorFor(command, mode)
	if !ok {
		return Capability{}, fmt.Errorf("skill capability %s has no scoped command descriptor for %s/%s", id, command, mode)
	}
	capability := Capability{
		ID: id, Audience: audience, Surface: surface, Command: command, Mode: mode,
		Argv: append([]string{}, argv...), Effect: effect, Policy: descriptor.Profile.MutationBoundary,
		ExactApplyFromResult: exact, Notes: notes,
	}
	if descriptor.Mutation != nil {
		capability.Currentness = descriptor.Mutation.Currentness
		capability.ExpectedApplyFlag = descriptor.Mutation.ExpectedFlag
	}
	return capability, nil
}

func validateCapabilities(capabilities []Capability) error {
	seen := map[string]bool{}
	for _, capability := range capabilities {
		if capability.ID == "" || seen[capability.ID] {
			return fmt.Errorf("skill capability id is empty or duplicated: %s", capability.ID)
		}
		seen[capability.ID] = true
		if capability.Audience == "" || capability.Surface == "" || len(capability.Argv) == 0 || capability.Effect == "" || capability.Policy == "" {
			return fmt.Errorf("skill capability %s is incomplete", capability.ID)
		}
		if capability.ExactApplyFromResult && capability.Command != "" && capability.ExpectedApplyFlag == "" {
			return fmt.Errorf("skill capability %s lacks its catalog-owned Apply binding", capability.ID)
		}
	}
	return nil
}

func cloneCapabilities(capabilities []Capability) []Capability {
	out := make([]Capability, len(capabilities))
	for index, capability := range capabilities {
		out[index] = capability
		out[index].Argv = append([]string{}, capability.Argv...)
	}
	return out
}

func marshalArgv(argv []string) ([]byte, error) {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(argv); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(out.Bytes(), []byte{'\n'}), nil
}

func RenderMachineAppendix() (string, error) {
	capabilities, err := Capabilities()
	if err != nil {
		return "", err
	}
	var out strings.Builder
	out.WriteString(MachineAppendixStart + "\n")
	out.WriteString("## 机器命令附录（生成，禁止手改）\n\n")
	out.WriteString("本节由 `internal/rekit/skillcontract` 的 typed capability catalog 生成。它不是普通用户命令目录；所有 argv 都传给 manifest 绑定的同一个项目内 executable，不经过 shell 拼接。\n\n")
	for _, capability := range capabilities {
		argv, err := marshalArgv(capability.Argv)
		if err != nil {
			return "", err
		}
		metadata := []string{
			"audience=`" + capability.Audience + "`",
			"effect=`" + capability.Effect + "`",
			"policy=`" + capability.Policy + "`",
		}
		if capability.Currentness != "" {
			metadata = append(metadata, "currentness=`"+capability.Currentness+"`")
		}
		if capability.ExpectedApplyFlag != "" {
			metadata = append(metadata, "Apply binding=`"+capability.ExpectedApplyFlag+"`")
		}
		fmt.Fprintf(&out, "- `%s`：`%s`；%s", capability.ID, argv, strings.Join(metadata, "；"))
		if capability.ExactApplyFromResult {
			out.WriteString("；只消费 preview/result 返回的 exact Apply，不重建参数")
		}
		if capability.Notes != "" {
			out.WriteString("；")
			out.WriteString(capability.Notes)
		}
		out.WriteString("。\n")
	}
	out.WriteString("\n固定桥之外只允许 `typed-invocation`；不得从 prose、guidance、hash、transport observation 或旧 request 重建可执行命令。\n")
	out.WriteString(MachineAppendixEnd)
	return out.String(), nil
}

func ValidateDocument(data []byte) error {
	normalized := string(sourceartifact.SemanticText(data))
	start, end, err := machineAppendixBounds(normalized)
	if err != nil {
		return err
	}
	humanOwned := normalized[:start] + normalized[end:]
	if err := validateHumanOwnedText(humanOwned); err != nil {
		return err
	}
	actual, err := extractMachineAppendix(normalized)
	if err != nil {
		return err
	}
	expected, err := RenderMachineAppendix()
	if err != nil {
		return err
	}
	if actual != expected {
		return fmt.Errorf("generated STeamAI machine command appendix is stale")
	}
	return nil
}

func validateHumanOwnedText(text string) error {
	if !strings.Contains(text, "机器命令附录是固定 front door、deterministic owner bridge、argv 与 Apply binding 的唯一 owner") {
		return fmt.Errorf("STeamAI skill human guidance must name the generated appendix as the only machine command owner")
	}
	for _, fragment := range []string{
		`.steamai\\runtime\\bin`,
		"runtime -Command",
		`["runtime", "-Command"`,
		"-ExpectedContinuePlanSha256",
		"-ExpectedControlPlanSha256",
		"-ExpectedProfilePlanSha256",
	} {
		if strings.Contains(text, fragment) {
			return fmt.Errorf("STeamAI skill human guidance duplicates generated machine contract fragment: %s", fragment)
		}
	}
	return nil
}

func ReplaceMachineAppendix(data []byte) ([]byte, error) {
	crlf := bytes.Contains(data, []byte("\r\n"))
	normalized := string(sourceartifact.SemanticText(data))
	start, end, err := machineAppendixBounds(normalized)
	if err != nil {
		return nil, err
	}
	rendered, err := RenderMachineAppendix()
	if err != nil {
		return nil, err
	}
	updated := normalized[:start] + rendered + normalized[end:]
	if crlf {
		updated = strings.ReplaceAll(updated, "\n", "\r\n")
	}
	return []byte(updated), nil
}

func Synchronize(repoRoot string) error {
	return synchronize(repoRoot, writeIfChanged)
}

// Check verifies the exact canonical/project-local skill pair without writing.
func Check(repoRoot string) error {
	_, err := ReadValidatedProjectTemplate(repoRoot)
	return err
}

// ReadValidatedProjectTemplate returns the exact template bytes that passed the
// canonical skill contract. Callers can publish these bytes without rereading.
func ReadValidatedProjectTemplate(repoRoot string) ([]byte, error) {
	repoRoot, err := filepath.Abs(strings.TrimSpace(repoRoot))
	if err != nil {
		return nil, err
	}
	canonicalPath := filepath.Join(
		repoRoot,
		filepath.FromSlash(CanonicalSkillPath),
	)
	templatePath := filepath.Join(
		repoRoot,
		filepath.FromSlash(ProjectTemplatePath),
	)
	canonical, err := rekitfs.ReadStableRegularFileAnchored(
		repoRoot,
		canonicalPath,
		"canonical /steamai skill",
		1<<20,
	)
	if err != nil {
		return nil, err
	}
	template, err := rekitfs.ReadStableRegularFileAnchored(
		repoRoot,
		templatePath,
		"project-local /steamai template",
		1<<20,
	)
	if err != nil {
		return nil, err
	}
	if err := ValidatePair(canonical, template); err != nil {
		return nil, err
	}
	return append([]byte(nil), template...), nil
}

type skillContractFileState struct {
	path   string
	data   []byte
	mode   os.FileMode
	exists bool
}

func synchronize(repoRoot string, writeFile func(string, []byte) error) error {
	repoRoot, err := filepath.Abs(strings.TrimSpace(repoRoot))
	if err != nil {
		return err
	}
	canonicalPath := filepath.Join(repoRoot, filepath.FromSlash(CanonicalSkillPath))
	canonical, err := inspectSkillContractFile(canonicalPath, true)
	if err != nil {
		return err
	}
	updated, err := ReplaceMachineAppendix(canonical.data)
	if err != nil {
		return err
	}
	if err := ValidateDocument(updated); err != nil {
		return err
	}
	templatePath := filepath.Join(repoRoot, filepath.FromSlash(ProjectTemplatePath))
	template, err := inspectSkillContractFile(templatePath, false)
	if err != nil {
		return err
	}

	written := []skillContractFileState{}
	for _, target := range []skillContractFileState{canonical, template} {
		if target.exists && bytes.Equal(target.data, updated) {
			continue
		}
		if err := writeFile(target.path, updated); err != nil {
			rollback := append(written, target)
			return errors.Join(err, restoreSkillContractFiles(rollback))
		}
		written = append(written, target)
	}
	return nil
}

func inspectSkillContractFile(path string, required bool) (skillContractFileState, error) {
	state := skillContractFileState{path: path}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) && !required {
		return state, nil
	}
	if err != nil {
		return state, err
	}
	if !info.Mode().IsRegular() {
		return state, fmt.Errorf("STeamAI skill target must be a regular file: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return state, err
	}
	state.data = data
	state.mode = info.Mode().Perm()
	state.exists = true
	return state, nil
}

func restoreSkillContractFiles(states []skillContractFileState) error {
	var restoreErr error
	for index := len(states) - 1; index >= 0; index-- {
		state := states[index]
		if state.exists {
			if err := os.MkdirAll(filepath.Dir(state.path), 0o755); err != nil {
				restoreErr = errors.Join(restoreErr, err)
				continue
			}
			if err := os.WriteFile(state.path, state.data, state.mode); err != nil {
				restoreErr = errors.Join(restoreErr, err)
			}
			continue
		}
		if err := os.Remove(state.path); err != nil && !os.IsNotExist(err) {
			restoreErr = errors.Join(restoreErr, err)
		}
	}
	return restoreErr
}

func ValidatePair(canonical, projectTemplate []byte) error {
	if err := ValidateDocument(canonical); err != nil {
		return fmt.Errorf("canonical /steamai skill: %w", err)
	}
	if err := ValidateDocument(projectTemplate); err != nil {
		return fmt.Errorf("project-local /steamai template: %w", err)
	}
	if !bytes.Equal(sourceartifact.SemanticText(canonical), sourceartifact.SemanticText(projectTemplate)) {
		return fmt.Errorf("project-local /steamai template is not generated from the canonical skill")
	}
	return nil
}

func extractMachineAppendix(text string) (string, error) {
	start, end, err := machineAppendixBounds(text)
	if err != nil {
		return "", err
	}
	return text[start:end], nil
}

func machineAppendixBounds(text string) (int, int, error) {
	if strings.Count(text, MachineAppendixStart) != 1 || strings.Count(text, MachineAppendixEnd) != 1 {
		return 0, 0, fmt.Errorf("STeamAI skill requires exactly one generated machine command appendix")
	}
	start := strings.Index(text, MachineAppendixStart)
	endMarker := strings.Index(text, MachineAppendixEnd)
	if start < 0 || endMarker < start {
		return 0, 0, fmt.Errorf("STeamAI machine command appendix markers are invalid")
	}
	return start, endMarker + len(MachineAppendixEnd), nil
}

func writeIfChanged(path string, data []byte) error {
	current, err := os.ReadFile(path)
	if err == nil && bytes.Equal(current, data) {
		return nil
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
