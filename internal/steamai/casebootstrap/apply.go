package casebootstrap

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func Apply(git, source, caseRoot string, facts Facts, confirmation string) (Preview, error) {
	preview, err := BuildPreview(git, source, caseRoot, facts)
	if err != nil {
		return Preview{}, err
	}
	if confirmation != ConfirmationPrefix+preview.Identity {
		return Preview{}, ErrConfirmationRequired
	}
	return preview, applyPreview(git, source, caseRoot, preview)
}

func applyPreview(git, source, caseRoot string, preview Preview) error {
	caseRoot, err := filepath.Abs(caseRoot)
	if err != nil {
		return err
	}
	stateRoot := filepath.Join(caseRoot, ".steamai-vnext")
	if exists, err := exists(stateRoot); err != nil {
		return err
	} else if exists {
		return ErrTargetDrift
	}
	staging, err := os.MkdirTemp(caseRoot, ".steamai-vnext.staging-")
	if err != nil {
		return err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(staging)
		}
	}()
	for _, rel := range []string{
		"evidence", "findings", "reviews", "learnings/candidates", "learnings/patches", "members",
		"evaluations/specs", "evaluations/runs", "evaluations/attestations", "evaluations/outcomes", "evaluations/work",
	} {
		if err := os.MkdirAll(filepath.Join(staging, filepath.FromSlash(rel)), 0o755); err != nil {
			return err
		}
	}
	for _, write := range preview.Writes {
		rel, ok := strings.CutPrefix(write.TargetPath, ".steamai-vnext/")
		if !ok {
			continue
		}
		if err := writeExactFile(filepath.Join(staging, filepath.FromSlash(rel)), write.Data); err != nil {
			return err
		}
	}
	if err := verifyStaging(staging, preview.Writes); err != nil {
		return err
	}
	current, err := BuildPreview(git, source, caseRoot, preview.Facts)
	if err != nil {
		if errors.Is(err, ErrCollision) || errors.Is(err, ErrPartialCase) {
			return fmt.Errorf("%w: %v", ErrTargetDrift, err)
		}
		return fmt.Errorf("%w: %v", ErrSourceDrift, err)
	}
	if current.Identity != preview.Identity {
		if current.Revision != preview.Revision || current.SourceDigest != preview.SourceDigest || current.SnapshotDigest != preview.SnapshotDigest {
			return ErrSourceDrift
		}
		return ErrTargetDrift
	}
	skill := writeByTarget(preview.Writes, ".claude/skills/steamai/SKILL.md")
	if skill.TargetPath == "" {
		return errors.New("Fresh preview 缺少 project-local skill")
	}
	if err := publishSkill(caseRoot, preview.Identity, skill); err != nil {
		return err
	}
	if exists, err := exists(stateRoot); err != nil {
		return err
	} else if exists {
		return ErrTargetDrift
	}
	if err := os.Rename(staging, stateRoot); err != nil {
		return fmt.Errorf("发布 completed case state: %w", err)
	}
	cleanup = false
	if err := ValidateCurrent(caseRoot); err != nil {
		return fmt.Errorf("发布后 current 验证失败: %w", err)
	}
	return nil
}

func publishSkill(caseRoot, identity string, write PlannedWrite) error {
	target := filepath.Join(caseRoot, filepath.FromSlash(write.TargetPath))
	if write.TargetAction == "unchanged" {
		if err := rejectReparse(target); err != nil {
			return ErrTargetDrift
		}
		data, err := os.ReadFile(target)
		if err != nil || hashBytes(data) != write.SHA256 {
			return ErrTargetDrift
		}
		return nil
	}
	if write.TargetAction != "create" {
		return ErrCollision
	}
	if err := validateTargetPath(caseRoot, write.TargetPath); err != nil {
		return err
	}
	if exists, err := exists(target); err != nil {
		return err
	} else if exists {
		return ErrTargetDrift
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	temporaryFile, err := os.CreateTemp(filepath.Dir(target), ".SKILL.md.steamai-"+identity[:12]+"-*.tmp")
	if err != nil {
		return err
	}
	temporary := temporaryFile.Name()
	defer os.Remove(temporary)
	if _, err := temporaryFile.Write(write.Data); err != nil {
		temporaryFile.Close()
		return err
	}
	if err := temporaryFile.Close(); err != nil {
		return err
	}
	data, err := os.ReadFile(temporary)
	if err != nil || hashBytes(data) != write.SHA256 {
		return ErrTargetDrift
	}
	if err := os.Link(temporary, target); err != nil {
		if present, _ := exists(target); present {
			return ErrTargetDrift
		}
		return err
	}
	published, err := os.ReadFile(target)
	if err != nil || hashBytes(published) != write.SHA256 {
		return ErrTargetDrift
	}
	return nil
}

func verifyStaging(staging string, writes []PlannedWrite) error {
	expected := map[string]string{}
	for _, write := range writes {
		if rel, ok := strings.CutPrefix(write.TargetPath, ".steamai-vnext/"); ok {
			expected[rel] = write.SHA256
		}
	}
	actual := map[string]string{}
	err := filepath.WalkDir(staging, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := rejectReparse(path); err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return ErrCollision
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(staging, path)
		if err != nil {
			return err
		}
		actual[filepath.ToSlash(rel)] = hashBytes(data)
		return nil
	})
	if err != nil {
		return err
	}
	if len(expected) != len(actual) {
		return ErrTargetDrift
	}
	for path, digest := range expected {
		if actual[path] != digest {
			return ErrTargetDrift
		}
	}
	return nil
}

func writeExactFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func writeByTarget(writes []PlannedWrite, target string) PlannedWrite {
	for _, write := range writes {
		if write.TargetPath == target {
			return write
		}
	}
	return PlannedWrite{}
}

func ActualSnapshotDigest(caseRoot string) (string, error) {
	metadata, err := parseSnapshot(filepath.Join(caseRoot, ".steamai-vnext", "pack-snapshot", "snapshot.yml"))
	if err != nil {
		return "", err
	}
	var records []SourceRecord
	for _, record := range metadata.Files {
		path := filepath.Join(caseRoot, ".steamai-vnext", "pack-snapshot", filepath.FromSlash(record.Path))
		if err := requirePlainPath(caseRoot, path, false); err != nil {
			return "", err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		record.Data = data
		record.Bytes = len(data)
		record.SHA256 = hashBytes(data)
		records = append(records, record)
	}
	var lines []string
	for _, record := range records {
		lines = append(lines, strings.Join([]string{
			record.Path, record.GitMode, record.ContentBlob, fmt.Sprint(record.Bytes), record.SHA256,
		}, "\x00"))
	}
	sort.Strings(lines)
	return "sha256:" + hashBytes([]byte(strings.Join(lines, "\n")+"\n")), nil
}
