package statemigration

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/caseshim"
	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/packidentity"
	"github.com/shuiyu486/re-context-kits/internal/rekit/sourceartifact"
)

const maxRootProjectionFile = int64(16 << 20)

type plannedRootFile struct {
	transition   RootFileTransition
	before       []byte
	after        []byte
	beforeExists bool
	source       string
	sourceData   []byte
}

var retiredManagedBlockSpanSHA256 = map[string]string{
	packidentity.RetiredVMP:     "4918a102df6c021fc18e8d40c1bf66994c3be87eceb1f8a25d8470ba05ffb716",
	packidentity.RetiredGeneric: "c92340764b075d2f176e2cd4cc8ffe2de8f7bf351cdb80560545db39807bb6a1",
}

func planRetiredRootFiles(caseRoot, sourcePack, projectName string, m *manifest.Manifest) ([]RootFileTransition, []plannedRootFile, error) {
	if !packidentity.IsRetired(sourcePack) {
		return nil, nil, nil
	}
	if m == nil || m.Pack != packidentity.Canonical {
		return nil, nil, fmt.Errorf("retired pack migration root projection requires canonical binary-re manifest")
	}
	root, err := rekitfs.OpenAnchoredRoot(caseRoot)
	if err != nil {
		return nil, nil, err
	}
	defer root.Close()

	planned := []plannedRootFile{}
	seen := map[string]string{}
	add := func(value plannedRootFile) error {
		rel, err := cleanRootTransitionPath(value.transition.Path)
		if err != nil {
			return err
		}
		value.transition.Path = rel
		key := strings.ToLower(rel)
		if previous, ok := seen[key]; ok {
			return fmt.Errorf("retired migration root target is declared more than once: %s (%s and %s)", rel, previous, value.transition.Kind)
		}
		for other, kind := range seen {
			if strings.HasPrefix(key, other+"/") || strings.HasPrefix(other, key+"/") {
				return fmt.Errorf("retired migration root targets overlap: %s (%s) and %s (%s)", rel, value.transition.Kind, other, kind)
			}
		}
		seen[key] = value.transition.Kind
		planned = append(planned, value)
		return nil
	}

	for _, rel := range m.ManagedFiles {
		source, err := m.SourcePath(rel)
		if err != nil {
			return nil, nil, err
		}
		after, err := stableCanonicalRootSource(source, m.BudgetLimit(rel))
		if err != nil {
			return nil, nil, err
		}
		before, info, exists, err := inspectRootTarget(root, rel, "retired migration managed-file target", m.BudgetLimit(rel))
		if err != nil {
			return nil, nil, err
		}
		action := "create-managed-file"
		if exists {
			action = "unchanged"
			if !bytes.Equal(before, after) {
				if !trustedCanonicalManagedFile(rel, before, after) {
					return nil, nil, fmt.Errorf("retired migration managed-file target differs from trusted canonical generations: %s", filepath.ToSlash(rel))
				}
				action = "replace-managed-file"
			}
		}
		mode := os.FileMode(0o644)
		if exists && action == "unchanged" {
			mode = info.Mode().Perm()
		}
		if err := add(newPlannedRootFile(rel, "managed-file", action, before, info, exists, after, mode, rootSourceRel(m, rel), source, after)); err != nil {
			return nil, nil, err
		}
	}

	for _, rel := range m.TemplateFiles {
		source, err := m.SourcePath(rel)
		if err != nil {
			return nil, nil, err
		}
		sourceData, err := stableCanonicalRootSource(source, maxRootProjectionFile)
		if err != nil {
			return nil, nil, err
		}
		targetRel := strings.TrimSuffix(rel, ".template.md") + ".md"
		materialized := []byte(strings.ReplaceAll(strings.ReplaceAll(string(sourceData), "<PROJECT_NAME>", projectName), "<PROJECT_ROOT>", caseRoot))
		before, info, exists, err := inspectRootTarget(root, targetRel, "retired migration template-file target", m.BudgetLimit(targetRel))
		if err != nil {
			return nil, nil, err
		}
		action := "create-template-file"
		after := materialized
		mode := os.FileMode(0o644)
		if exists {
			if len(before) == 0 {
				return nil, nil, fmt.Errorf("retired migration existing template-file must be non-empty: %s", filepath.ToSlash(targetRel))
			}
			action = "preserve-existing-template"
			after = before
			mode = info.Mode().Perm()
		}
		if err := add(newPlannedRootFile(targetRel, "template-file", action, before, info, exists, after, mode, rootSourceRel(m, rel), source, sourceData)); err != nil {
			return nil, nil, err
		}
	}

	blockSourceRel := m.ManagedBlock["source"]
	blockSource, err := m.SourcePath(blockSourceRel)
	if err != nil {
		return nil, nil, err
	}
	blockData, err := stableCanonicalRootSource(blockSource, m.BudgetLimit(m.ManagedBlock["file"]))
	if err != nil {
		return nil, nil, err
	}
	blockRel := m.ManagedBlock["file"]
	blockBefore, blockInfo, blockExists, err := inspectRootTarget(root, blockRel, "retired migration managed-block target", m.BudgetLimit(blockRel))
	if err != nil {
		return nil, nil, err
	}
	blockAction := "create-managed-block-host"
	blockAfter := append(append([]byte("# Project Context\r\n\r\n"), bytes.TrimSpace(blockData)...), []byte("\r\n")...)
	blockMode := os.FileMode(0o644)
	if blockExists {
		blockAfter, err = replaceRetiredManagedBlock(blockBefore, sourcePack, blockData)
		if err != nil {
			return nil, nil, err
		}
		blockAction = "replace-managed-block"
		blockMode = blockInfo.Mode().Perm()
	}
	if err := add(newPlannedRootFile(blockRel, "managed-block", blockAction, blockBefore, blockInfo, blockExists, blockAfter, blockMode, rootSourceRel(m, blockSourceRel), blockSource, blockData)); err != nil {
		return nil, nil, err
	}

	gitignoreSource, err := m.SourcePath("examples/gitignore.example")
	if err != nil {
		return nil, nil, err
	}
	gitignoreData, sourceErr := stableCanonicalRootSource(gitignoreSource, maxRootProjectionFile)
	if sourceErr != nil && !errors.Is(sourceErr, os.ErrNotExist) {
		return nil, nil, sourceErr
	}
	if sourceErr == nil {
		before, info, exists, err := inspectRootTarget(root, ".gitignore", "retired migration support-file target", maxRootProjectionFile)
		if err != nil {
			return nil, nil, err
		}
		action := "create-support-file"
		after := gitignoreData
		mode := os.FileMode(0o644)
		if exists {
			action = "preserve-existing-support"
			after = before
			mode = info.Mode().Perm()
		}
		if err := add(newPlannedRootFile(".gitignore", "support-file", action, before, info, exists, after, mode, rootSourceRel(m, "examples/gitignore.example"), gitignoreSource, gitignoreData)); err != nil {
			return nil, nil, err
		}
	}

	sort.Slice(planned, func(i, j int) bool { return planned[i].transition.Path < planned[j].transition.Path })
	transitions := make([]RootFileTransition, 0, len(planned))
	for _, value := range planned {
		transitions = append(transitions, value.transition)
	}
	return transitions, planned, nil
}

func trustedCanonicalManagedFile(rel string, before, after []byte) bool {
	contract := caseshim.PackRecoveryWrites(packidentity.Canonical)[filepath.ToSlash(rel)]
	if contract.Kind != "managed-file" {
		return false
	}
	if _, ok := contract.AcceptedSHA256s[sha256Hex(before)]; !ok {
		return false
	}
	_, ok := contract.AcceptedSHA256s[sha256Hex(after)]
	return ok
}

func newPlannedRootFile(rel, kind, action string, before []byte, info os.FileInfo, exists bool, after []byte, afterMode os.FileMode, sourceRel, source string, sourceData []byte) plannedRootFile {
	transition := RootFileTransition{
		Path:         filepath.ToSlash(rel),
		Kind:         kind,
		Action:       action,
		AfterSHA256:  sha256Hex(after),
		AfterSize:    int64(len(after)),
		AfterMode:    uint32(afterMode.Perm()),
		SourcePath:   filepath.ToSlash(sourceRel),
		SourceSHA256: sha256Hex(sourceData),
	}
	if exists {
		transition.BeforeSHA256 = sha256Hex(before)
		transition.BeforeSize = int64(len(before))
		transition.BeforeMode = uint32(info.Mode().Perm())
	}
	return plannedRootFile{
		transition: transition, before: append([]byte(nil), before...), after: append([]byte(nil), after...), beforeExists: exists,
		source: source, sourceData: append([]byte(nil), sourceData...),
	}
}

func cleanRootTransitionPath(rel string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(strings.TrimSpace(rel)))
	if clean == "." || filepath.IsAbs(clean) || !filepath.IsLocal(clean) {
		return "", fmt.Errorf("retired migration root target escapes the project: %s", rel)
	}
	return filepath.ToSlash(clean), nil
}

func rootSourceRel(m *manifest.Manifest, rel string) string {
	return filepath.ToSlash(filepath.Join("packs", m.Pack, filepath.FromSlash(rel)))
}

func stableCanonicalRootSource(path string, limit int64) ([]byte, error) {
	if limit < 1 || limit > maxRootProjectionFile {
		limit = maxRootProjectionFile
	}
	data, err := rekitfs.ReadStableRegularFileAnchored(filepath.Dir(path), path, "retired migration canonical root source", limit)
	if err != nil {
		return nil, err
	}
	return sourceartifact.CanonicalText(data), nil
}

func inspectRootTarget(root *rekitfs.AnchoredRoot, rel, label string, limit int64) ([]byte, os.FileInfo, bool, error) {
	clean, err := cleanRootTransitionPath(rel)
	if err != nil {
		return nil, nil, false, err
	}
	info, err := root.Lstat(filepath.FromSlash(clean))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, false, nil
	}
	if err != nil {
		return nil, nil, false, fmt.Errorf("inspect %s: %s: %w", label, clean, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > limit {
		return nil, nil, false, fmt.Errorf("%s must be a bounded regular non-symlink file: %s", label, clean)
	}
	data, opened, err := root.ReadStableFile(filepath.FromSlash(clean), limit)
	if err != nil {
		return nil, nil, false, fmt.Errorf("read %s: %s: %w", label, clean, err)
	}
	return data, opened, true, nil
}

func replaceRetiredManagedBlock(host []byte, sourcePack string, canonicalBlock []byte) ([]byte, error) {
	blockID := retiredManagedBlockIDForPack(sourcePack)
	if blockID == "" {
		return nil, fmt.Errorf("retired migration managed block source pack is invalid: %s", sourcePack)
	}
	text := string(sourceartifact.SemanticText(host))
	if strings.Count(text, "<!-- BEGIN ") != 1 || strings.Count(text, "<!-- END ") != 1 || strings.Contains(text, "<!-- BEGIN binary-re-template:router") {
		return nil, fmt.Errorf("retired migration managed block host contains duplicate or unknown generations")
	}
	beginMarker := "<!-- BEGIN " + blockID
	endMarker := "<!-- END " + blockID + " -->"
	begin := strings.Index(text, beginMarker)
	end := strings.Index(text, endMarker)
	if begin < 0 || end < begin {
		return nil, fmt.Errorf("retired migration managed block host omits the exact retired generation")
	}
	end += len(endMarker)
	span := text[begin:end]
	if sha256Hex([]byte(span)) != retiredManagedBlockSpanSHA256[sourcePack] {
		return nil, fmt.Errorf("retired managed block does not match an accepted generation")
	}

	exactBegin := bytes.Index(host, []byte(beginMarker))
	exactEnd := bytes.Index(host, []byte(endMarker))
	if exactBegin < 0 || exactEnd < exactBegin {
		return nil, fmt.Errorf("retired migration managed block exact bytes are invalid")
	}
	exactEnd += len(endMarker)
	after := make([]byte, 0, exactBegin+len(canonicalBlock)+len(host)-exactEnd)
	after = append(after, host[:exactBegin]...)
	after = append(after, bytes.TrimSpace(canonicalBlock)...)
	after = append(after, host[exactEnd:]...)
	return after, nil
}

func retiredManagedBlockIDForPack(pack string) string {
	switch pack {
	case packidentity.RetiredVMP:
		return "vmp-re-template:router"
	case packidentity.RetiredGeneric:
		return "generic-binary-re:router"
	default:
		return ""
	}
}

func rootTransitionWrites(transitions []RootFileTransition) []Write {
	writes := make([]Write, 0, len(transitions))
	for _, transition := range transitions {
		writes = append(writes, Write{
			Path: transition.Path, Kind: transition.Kind, Action: transition.Action,
			SHA256: transition.AfterSHA256, Size: transition.AfterSize, SourcePath: transition.SourcePath,
		})
	}
	return writes
}

func validateRootFileSources(files []plannedRootFile) error {
	for _, file := range files {
		data, err := stableCanonicalRootSource(file.source, maxRootProjectionFile)
		if err != nil {
			return fmt.Errorf("read retired migration root source after preview: %s: %w", file.transition.SourcePath, err)
		}
		if !bytes.Equal(data, file.sourceData) || sha256Hex(data) != file.transition.SourceSHA256 {
			return fmt.Errorf("retired migration root source changed after preview: %s", file.transition.SourcePath)
		}
	}
	return nil
}

func validateRootFileTargets(caseRoot string, files []plannedRootFile, phase string) error {
	root, err := rekitfs.OpenAnchoredRoot(caseRoot)
	if err != nil {
		return err
	}
	defer root.Close()
	for _, file := range files {
		data, info, exists, err := inspectRootTarget(root, file.transition.Path, "retired migration root target", maxRootProjectionFile)
		if err != nil {
			return fmt.Errorf("inspect retired migration root target %s: %w", file.transition.Path, err)
		}
		if exists != file.beforeExists || (exists && (!bytes.Equal(data, file.before) || !rootFileModeMatches(file.transition.BeforeMode, info))) {
			return fmt.Errorf("retired migration root target changed %s: %s", phase, file.transition.Path)
		}
	}
	return nil
}

func applyRootFiles(caseRoot string, files []plannedRootFile) error {
	if len(files) == 0 {
		return nil
	}
	root, err := rekitfs.OpenAnchoredRoot(caseRoot)
	if err != nil {
		return err
	}
	defer root.Close()
	for _, file := range files {
		transition := file.transition
		rel := filepath.FromSlash(transition.Path)
		switch transition.Action {
		case "unchanged", "preserve-existing-template", "preserve-existing-support":
			if err := validateAppliedRootFile(root, file); err != nil {
				return err
			}
		case "create-managed-file", "create-template-file", "create-managed-block-host", "create-support-file":
			if err := root.MkdirAllNoFollow(filepath.Dir(rel), 0o700); err != nil {
				return err
			}
			if _, err := root.WriteExclusiveFileWriteThrough(rel, file.after, os.FileMode(transition.AfterMode), false); err != nil {
				return fmt.Errorf("publish retired migration root file %s: %w", transition.Path, err)
			}
			if err := validateAppliedRootFile(root, file); err != nil {
				return err
			}
		case "replace-managed-file", "replace-managed-block":
			backup := filepath.ToSlash(filepath.Join(".steamai", "migration", ".root-before-"+sha256Hex([]byte(transition.Path))))
			if err := root.RenameFileNoReplaceExact(rel, filepath.FromSlash(backup), file.before, os.FileMode(transition.BeforeMode)); err != nil {
				return fmt.Errorf("stage exact retired migration root replacement %s: %w", transition.Path, err)
			}
			if _, err := root.WriteExclusiveFileWriteThrough(rel, file.after, os.FileMode(transition.AfterMode), false); err != nil {
				return fmt.Errorf("publish exact retired migration root replacement %s: %w", transition.Path, err)
			}
			if err := root.RemoveExactFile(filepath.FromSlash(backup), file.before, os.FileMode(transition.BeforeMode)); err != nil {
				return fmt.Errorf("retire exact retired migration root backup %s: %w", transition.Path, err)
			}
			if err := validateAppliedRootFile(root, file); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported retired migration root transition action %q", transition.Action)
		}
	}
	return root.Validate()
}

func validateAppliedRootFiles(caseRoot string, files []plannedRootFile) error {
	if len(files) == 0 {
		return nil
	}
	root, err := rekitfs.OpenAnchoredRoot(caseRoot)
	if err != nil {
		return err
	}
	defer root.Close()
	for _, file := range files {
		if err := validateAppliedRootFile(root, file); err != nil {
			return err
		}
	}
	return nil
}

func infoMode(info os.FileInfo) uint32 {
	if info == nil {
		return 0
	}
	return uint32(info.Mode().Perm())
}

func rootFileModeMatches(expected uint32, info os.FileInfo) bool {
	if info == nil {
		return false
	}
	actual := uint32(info.Mode().Perm())
	if filepath.Separator == '\\' {
		return expected&0o200 == actual&0o200
	}
	return expected == actual
}

func validateAppliedRootFile(root *rekitfs.AnchoredRoot, file plannedRootFile) error {
	data, info, exists, err := inspectRootTarget(root, file.transition.Path, "migrated root file", maxRootProjectionFile)
	if err != nil {
		return fmt.Errorf("inspect migrated root file %s: %w", file.transition.Path, err)
	}
	if !exists || !bytes.Equal(data, file.after) || sha256Hex(data) != file.transition.AfterSHA256 || int64(len(data)) != file.transition.AfterSize || !rootFileModeMatches(file.transition.AfterMode, info) {
		return fmt.Errorf("migrated root file differs from exact plan: %s (exists=%t info=%v gotSha=%s wantSha=%s gotSize=%d wantSize=%d gotMode=%o wantMode=%o)", file.transition.Path, exists, info, sha256Hex(data), file.transition.AfterSHA256, len(data), file.transition.AfterSize, infoMode(info), file.transition.AfterMode)
	}
	return nil
}

func validateCurrentReceiptRootFiles(caseRoot string, transitions []RootFileTransition) error {
	if len(transitions) == 0 {
		return nil
	}
	root, err := rekitfs.OpenAnchoredRoot(caseRoot)
	if err != nil {
		return err
	}
	defer root.Close()
	for _, transition := range transitions {
		data, info, exists, err := inspectRootTarget(root, transition.Path, "state migration receipt root binding", maxRootProjectionFile)
		if err != nil {
			return fmt.Errorf("inspect state migration receipt root binding %s: %w", transition.Path, err)
		}
		if !exists || sha256Hex(data) != transition.AfterSHA256 || int64(len(data)) != transition.AfterSize || !rootFileModeMatches(transition.AfterMode, info) {
			return fmt.Errorf("state migration receipt root binding differs from current project: %s", transition.Path)
		}
	}
	return nil
}

func validateReceiptRootTransitions(sourcePack, targetPack string, transitions []RootFileTransition) error {
	if !packidentity.IsRetired(sourcePack) {
		if len(transitions) != 0 {
			return fmt.Errorf("state migration receipt has unexpected root transitions")
		}
		return nil
	}
	if targetPack != packidentity.Canonical || len(transitions) == 0 {
		return fmt.Errorf("retired state migration receipt omits canonical root transitions")
	}
	seen := map[string]struct{}{}
	previous := ""
	for _, transition := range transitions {
		path, err := cleanRootTransitionPath(transition.Path)
		if err != nil || path != transition.Path || transition.Path <= previous || transition.Kind == "" || transition.Action == "" || !validSHA256(transition.AfterSHA256) || transition.AfterSize < 0 || transition.AfterMode == 0 || !validSHA256(transition.SourceSHA256) {
			return fmt.Errorf("state migration receipt root transition is invalid: %s", transition.Path)
		}
		if transition.SourcePath == "" {
			return fmt.Errorf("state migration receipt root transition source is missing: %s", transition.Path)
		}
		if sourcePath, err := cleanRootTransitionPath(transition.SourcePath); err != nil || sourcePath != transition.SourcePath {
			return fmt.Errorf("state migration receipt root transition source is invalid: %s", transition.Path)
		}
		if _, ok := seen[strings.ToLower(path)]; ok {
			return fmt.Errorf("state migration receipt root transition is duplicated: %s", path)
		}
		seen[strings.ToLower(path)] = struct{}{}
		previous = transition.Path
		existing := transition.BeforeSHA256 != ""
		if existing {
			if !validSHA256(transition.BeforeSHA256) || transition.BeforeSize < 0 || transition.BeforeMode == 0 {
				return fmt.Errorf("state migration receipt root transition before binding is invalid: %s", transition.Path)
			}
		} else if transition.BeforeSize != 0 || transition.BeforeMode != 0 {
			return fmt.Errorf("state migration receipt root transition missing binding is inconsistent: %s", transition.Path)
		}
		switch transition.Action {
		case "unchanged", "preserve-existing-template", "preserve-existing-support":
			if !existing || transition.BeforeSHA256 != transition.AfterSHA256 || transition.BeforeSize != transition.AfterSize || transition.BeforeMode != transition.AfterMode {
				return fmt.Errorf("state migration receipt preserved root transition is inconsistent: %s", transition.Path)
			}
		case "replace-managed-file", "replace-managed-block":
			if !existing {
				return fmt.Errorf("state migration receipt replacement omits before binding: %s", transition.Path)
			}
		case "create-managed-file", "create-template-file", "create-managed-block-host", "create-support-file":
			if existing {
				return fmt.Errorf("state migration receipt create transition has a before binding: %s", transition.Path)
			}
		default:
			return fmt.Errorf("state migration receipt root transition action is invalid: %s", transition.Path)
		}
	}
	return nil
}
