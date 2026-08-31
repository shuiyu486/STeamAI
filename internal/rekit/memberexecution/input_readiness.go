package memberexecution

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
)

const (
	TaskBindingArtifactAnalysis   = "artifact-analysis"
	TaskBindingWorkspaceInventory = "workspace-inventory"

	inputArtifactPathKey   = "artifact-path"
	inputArtifactSHAKey    = "artifact-sha256"
	inputArtifactBytesKey  = "artifact-bytes"
	inputWorkspaceScopeKey = "workspace-scope"
	maxArtifactInputBytes  = int64(32 << 20)
)

func ArtifactAnalysisTaskBinding(caseRoot, artifactPath string) (TaskBinding, error) {
	artifactPath, fullPath, err := canonicalInputPath(caseRoot, artifactPath, false)
	if err != nil {
		return TaskBinding{}, err
	}
	data, err := rekitfs.ReadStableRegularFileAnchored(caseRoot, fullPath, "member artifact-analysis input", maxArtifactInputBytes)
	if err != nil {
		return TaskBinding{}, err
	}
	if len(data) == 0 {
		return TaskBinding{}, fmt.Errorf("artifact-analysis task input must not be empty")
	}
	return TaskBinding{
		Kind: TaskBindingArtifactAnalysis,
		Values: map[string]string{
			inputArtifactPathKey:  artifactPath,
			inputArtifactSHAKey:   hash(data),
			inputArtifactBytesKey: strconv.FormatInt(int64(len(data)), 10),
		},
	}, nil
}

func WorkspaceInventoryTaskBinding(caseRoot, scope string) (TaskBinding, error) {
	scope, fullPath, err := canonicalInputPath(caseRoot, scope, true)
	if err != nil {
		return TaskBinding{}, err
	}
	if _, err := rekitfs.ValidateNonReparseDirectory(fullPath, "member workspace-inventory scope"); err != nil {
		return TaskBinding{}, err
	}
	return TaskBinding{
		Kind: TaskBindingWorkspaceInventory,
		Values: map[string]string{
			inputWorkspaceScopeKey: scope,
		},
	}, nil
}

func ValidateTaskInputBinding(caseRoot string, binding TaskBinding) error {
	binding, err := validateTaskBinding(binding)
	if err != nil {
		return err
	}
	switch binding.Kind {
	case TaskBindingArtifactAnalysis:
		if len(binding.Values) != 3 || !validSHA(binding.Values[inputArtifactSHAKey]) {
			return fmt.Errorf("artifact-analysis task binding is incomplete")
		}
		expectedBytes, err := strconv.ParseInt(binding.Values[inputArtifactBytesKey], 10, 64)
		if err != nil || expectedBytes < 1 || expectedBytes > maxArtifactInputBytes {
			return fmt.Errorf("artifact-analysis task binding size is invalid")
		}
		_, fullPath, err := canonicalInputPath(caseRoot, binding.Values[inputArtifactPathKey], false)
		if err != nil {
			return err
		}
		data, err := rekitfs.ReadStableRegularFileAnchored(caseRoot, fullPath, "member artifact-analysis input", maxArtifactInputBytes)
		if err != nil {
			return fmt.Errorf("read artifact-analysis task input: %w", err)
		}
		if int64(len(data)) != expectedBytes || !strings.EqualFold(hash(data), binding.Values[inputArtifactSHAKey]) {
			return fmt.Errorf("artifact-analysis task input changed")
		}
	case TaskBindingWorkspaceInventory:
		if len(binding.Values) != 1 {
			return fmt.Errorf("workspace-inventory task binding is incomplete")
		}
		_, fullPath, err := canonicalInputPath(caseRoot, binding.Values[inputWorkspaceScopeKey], true)
		if err != nil {
			return err
		}
		if _, err := rekitfs.ValidateNonReparseDirectory(fullPath, "member workspace-inventory scope"); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported member task input binding: %s", binding.Kind)
	}
	return nil
}

func IsTaskInputBinding(binding TaskBinding) bool {
	return binding.Kind == TaskBindingArtifactAnalysis || binding.Kind == TaskBindingWorkspaceInventory
}

func canonicalInputPath(caseRoot, value string, allowRoot bool) (string, string, error) {
	caseRoot, err := filepath.Abs(strings.TrimSpace(caseRoot))
	if err != nil {
		return "", "", err
	}
	value = filepath.ToSlash(strings.TrimSpace(value))
	if value == "." && allowRoot {
		return value, caseRoot, nil
	}
	if !validRelative(value) {
		return "", "", fmt.Errorf("member task input path must be canonical and case-relative")
	}
	fullPath, err := rekitfs.SafeJoin(caseRoot, value)
	if err != nil {
		return "", "", err
	}
	return value, fullPath, nil
}
