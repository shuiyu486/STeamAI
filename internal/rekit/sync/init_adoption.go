package sync

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/casebind"
	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/review"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtimebundle"
	"github.com/shuiyu486/re-context-kits/internal/rekit/skillcontract"
	"github.com/shuiyu486/re-context-kits/internal/rekit/sourceartifact"
)

type ordinaryInitPublication struct {
	write WriteResult
	data  []byte
}

type ordinaryInitPublishedFile struct {
	publication  ordinaryInitPublication
	info         os.FileInfo
	handle       *os.File
	rollbackData []byte
	created      bool
	dataKnown    bool
}

type ordinaryInitPublishedDirectory struct {
	rel    string
	info   os.FileInfo
	handle *os.File
}

type ordinaryInitRollbackState struct {
	files       []ordinaryInitPublishedFile
	directories []ordinaryInitPublishedDirectory
}

func applyOrdinaryInit(plan InitPlan, lease mutationLease) (result ApplyResult, retErr error) {
	leaseOwned := true
	defer func() {
		if leaseOwned {
			retErr = errors.Join(retErr, lease.Unlock())
		}
	}()
	if err := ordinaryInitRollbackCapability(); err != nil {
		return ApplyResult{}, err
	}

	if ordinaryInitAfterPlanHook != nil {
		if err := ordinaryInitAfterPlanHook(plan); err != nil {
			return ApplyResult{}, err
		}
	}
	fresh, err := InitPreview(plan.RepoRoot, plan.CaseRoot, plan.Pack, ApplyOptions{
		ProjectName: plan.ProjectName, CreateLocalFiles: true, Command: plan.Command,
	})
	if err != nil {
		return ApplyResult{}, err
	}
	if plan.TargetClass == "missing" {
		if fresh.TargetClass != "ordinary-directory" || !fresh.AdoptionReady {
			return ApplyResult{}, fmt.Errorf("%s plan changed before new-project publication; rerun -WhatIf", plan.Command)
		}
		fresh.ExpectedPlanSHA256 = plan.ExpectedPlanSHA256
	} else if fresh.TargetClass != "ordinary-directory" || !fresh.AdoptionReady || !strings.EqualFold(fresh.ExpectedPlanSHA256, plan.ExpectedPlanSHA256) {
		return ApplyResult{}, fmt.Errorf("%s plan changed before ordinary-directory publication; rerun -WhatIf", plan.Command)
	}
	publications, err := ordinaryInitPublications(fresh)
	if err != nil {
		return ApplyResult{}, err
	}
	rootInfo, err := refsf.ValidateNonReparseDirectory(fresh.CaseRoot, "ordinary init target")
	if err != nil {
		return ApplyResult{}, err
	}
	if err := refsf.ValidateTreeNoReparse(fresh.CaseRoot, "ordinary init target"); err != nil {
		return ApplyResult{}, err
	}
	current, err := InitPreview(fresh.RepoRoot, fresh.CaseRoot, fresh.Pack, ApplyOptions{
		ProjectName: fresh.ProjectName, CreateLocalFiles: true, Command: fresh.Command,
	})
	if err != nil {
		return ApplyResult{}, err
	}
	if current.TargetClass != "ordinary-directory" || !current.AdoptionReady {
		return ApplyResult{}, fmt.Errorf("%s plan changed before ordinary-directory publication; rerun -WhatIf", plan.Command)
	}
	if plan.TargetClass != "missing" && !strings.EqualFold(current.ExpectedPlanSHA256, fresh.ExpectedPlanSHA256) {
		return ApplyResult{}, fmt.Errorf("%s plan changed before ordinary-directory publication; rerun -WhatIf", plan.Command)
	}
	root, err := os.OpenRoot(fresh.CaseRoot)
	if err != nil {
		return ApplyResult{}, err
	}
	rootOwned := true
	defer func() {
		if rootOwned {
			retErr = errors.Join(retErr, root.Close())
		}
	}()

	published := ordinaryInitRollbackState{}
	closeHandles := func() error {
		var closeErr error
		for index := len(published.files) - 1; index >= 0; index-- {
			if published.files[index].handle != nil {
				closeErr = errors.Join(closeErr, published.files[index].handle.Close())
				published.files[index].handle = nil
			}
		}
		for index := len(published.directories) - 1; index >= 0; index-- {
			if published.directories[index].handle != nil {
				closeErr = errors.Join(closeErr, published.directories[index].handle.Close())
				published.directories[index].handle = nil
			}
		}
		return closeErr
	}
	defer func() { retErr = errors.Join(retErr, closeHandles()) }()

	opened, err := root.Lstat(".")
	if err != nil || !os.SameFile(rootInfo, opened) {
		return ApplyResult{}, fmt.Errorf("%s ordinary-directory root changed before publication", plan.Command)
	}
	if err := ordinaryInitPreflight(root, fresh, false); err != nil {
		return ApplyResult{}, err
	}
	rollback := func(cause error) error {
		if ordinaryInitBeforeRollbackHook != nil {
			cause = errors.Join(cause, ordinaryInitBeforeRollbackHook("before-rollback", fresh))
		}
		return errors.Join(cause, ordinaryInitRollback(root, fresh.CaseRoot, published))
	}
	for index, publication := range publications {
		file, directories, err := ordinaryInitWriteExclusive(root, fresh.CaseRoot, publication)
		published.directories = append(published.directories, directories...)
		if file.created {
			published.files = append(published.files, file)
		}
		if err != nil {
			return ApplyResult{}, rollback(err)
		}
		if ordinaryInitAfterPublicationHook != nil {
			if err := ordinaryInitAfterPublicationHook(index+1, fresh); err != nil {
				return ApplyResult{}, rollback(err)
			}
		}
	}
	if ordinaryInitBeforeFinalValidationHook != nil {
		if err := ordinaryInitBeforeFinalValidationHook(fresh); err != nil {
			return ApplyResult{}, rollback(err)
		}
	}
	currentRoot, err := os.Lstat(fresh.CaseRoot)
	if err != nil || !os.SameFile(rootInfo, currentRoot) {
		return ApplyResult{}, rollback(fmt.Errorf("%s ordinary-directory root changed during publication: %w", plan.Command, err))
	}
	if err := refsf.ValidateNoReparseComponents(fresh.CaseRoot); err != nil {
		return ApplyResult{}, rollback(err)
	}
	if err := ordinaryInitPreflight(root, fresh, true); err != nil {
		return ApplyResult{}, rollback(err)
	}
	if err := ordinaryInitPublicationsCurrent(root, fresh.CaseRoot, published); err != nil {
		return ApplyResult{}, rollback(err)
	}
	if err := ordinaryInitSourcesCurrent(fresh); err != nil {
		return ApplyResult{}, rollback(err)
	}
	result = ApplyResult{
		SchemaVersion: 1, Command: fresh.Command, CaseRoot: fresh.CaseRoot,
		RepoRoot: fresh.RepoRoot, Pack: fresh.Pack, IsMutation: true, Applied: true,
		Writes:    fresh.Writes,
		NextSteps: []string{"run doctor after apply", "existing ordinary-directory files were preserved by create-only publication"},
	}

	cleanupErr := closeHandles()
	if rootOwned {
		rootOwned = false
		cleanupErr = errors.Join(cleanupErr, root.Close())
	}
	if leaseOwned {
		leaseOwned = false
		cleanupErr = errors.Join(cleanupErr, lease.Unlock())
	}
	if cleanupErr != nil {
		result.NextSteps = append(result.NextSteps,
			"ordinary init committed, but resource cleanup reported an error; do not retry the original plan: "+cleanupErr.Error(),
		)
	}
	return result, nil
}

func ordinaryInitPublications(plan InitPlan) ([]ordinaryInitPublication, error) {
	publications := []ordinaryInitPublication{}
	for _, write := range plan.Writes {
		if write.Action == "unchanged" || strings.HasPrefix(write.Action, "skip-existing-") {
			continue
		}
		data, err := ordinaryInitWriteBytes(plan, write)
		if err != nil {
			return nil, err
		}
		expected := plan.initSourceSHA256[write.Path]
		if write.Kind == "sync-state" {
			var state syncState
			if err := json.Unmarshal(data, &state); err != nil {
				return nil, err
			}
			semantic, err := json.Marshal(initSyncStateIdentity{
				SchemaVersion: state.SchemaVersion,
				TemplateRoot:  state.TemplateRoot,
				TemplatePack:  state.TemplatePack,
				Managed:       state.Managed,
			})
			if err != nil || !strings.EqualFold(sha256Bytes(semantic), expected) {
				return nil, fmt.Errorf("ordinary init sync state changed after preview: %s", write.Path)
			}
		} else if !strings.EqualFold(sha256Bytes(data), expected) {
			return nil, fmt.Errorf("ordinary init source changed after preview: %s", write.Path)
		}
		publications = append(publications, ordinaryInitPublication{write: write, data: data})
	}
	return publications, nil
}

func ordinaryInitWriteBytes(plan InitPlan, write WriteResult) ([]byte, error) {
	switch write.Kind {
	case "instance-metadata":
		if strings.HasPrefix(filepath.ToSlash(write.Path), ".steamai/") {
			return []byte(casebind.STeamAIInstanceText(plan.CaseRoot, plan.Pack, plan.ProjectName, runtimebundle.ManifestRel, plan.bundleManifestSHA256)), nil
		}
		return []byte(casebind.InstanceText(plan.CaseRoot, plan.RepoRoot, plan.Pack, plan.ProjectName)), nil
	case "legacy-metadata":
		return []byte("templateRoot: " + plan.RepoRoot + "\r\n" +
			"rekitMode: case-local-shim\r\n" +
			"templatePack: " + plan.Pack + "\r\n" +
			"templateVersion: 0.0.0\r\n"), nil
	case "sync-state":
		return ordinaryInitStateBytes(plan)
	}
	if len(write.rawContent) > 0 {
		return append([]byte(nil), write.rawContent...), nil
	}
	var (
		data []byte
		err  error
	)
	if write.Kind == "runtime-executable" || write.Kind == "pack-asset" || write.Kind == "common-asset" || write.Kind == "runtime-asset" {
		data, err = os.ReadFile(write.SourcePath)
	} else {
		data, err = sourceartifact.ReadCanonical(write.SourcePath)
	}
	if err != nil {
		return nil, err
	}
	switch write.Kind {
	case "template-file":
		text := strings.ReplaceAll(string(data), "<PROJECT_NAME>", plan.ProjectName)
		text = strings.ReplaceAll(text, "<PROJECT_ROOT>", plan.CaseRoot)
		return []byte(text), nil
	case "managed-block":
		if strings.TrimSpace(write.blockID) == "" {
			return nil, fmt.Errorf("ordinary init managed block id is missing: %s", write.Path)
		}
		return []byte(review.ApplyManagedBlock("", write.blockID, string(data))), nil
	default:
		return data, nil
	}
}

func ordinaryInitStateBytes(plan InitPlan) ([]byte, error) {
	identity, err := initSyncStateIdentityForPlan(plan)
	if err != nil {
		return nil, err
	}
	state := syncState{
		SchemaVersion: identity.SchemaVersion,
		TemplateRoot:  identity.TemplateRoot,
		TemplatePack:  identity.TemplatePack,
		LastSyncAt:    time.Now().Format("2006-01-02T15:04:05-07:00"),
		Managed:       identity.Managed,
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func ordinaryInitPreflight(root *os.Root, plan InitPlan, afterPublication bool) error {
	for _, write := range plan.Writes {
		rel, err := ordinaryInitRelative(plan.CaseRoot, write.TargetPath)
		if err != nil {
			return err
		}
		info, err := root.Lstat(rel)
		preservesExisting := write.Action == "unchanged" || strings.HasPrefix(write.Action, "skip-existing-")
		if preservesExisting {
			if err != nil {
				return fmt.Errorf("ordinary init preserved target changed after preview: %s: %w", write.Path, err)
			}
			if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
				return fmt.Errorf("ordinary init preserved target is not a regular non-symlink file: %s", write.Path)
			}
			data, err := root.ReadFile(rel)
			if err != nil {
				return err
			}
			after, err := root.Lstat(rel)
			if err != nil || !after.Mode().IsRegular() || after.Mode()&os.ModeSymlink != 0 || !os.SameFile(info, after) || after.Size() != int64(len(data)) {
				return fmt.Errorf("ordinary init preserved target changed while reading: %s: %w", write.Path, err)
			}
			expected := plan.initTargetSHA256[write.Path]
			if !strings.EqualFold(sha256Bytes(data), expected) {
				return fmt.Errorf("ordinary init preserved target changed after preview: %s", write.Path)
			}
			continue
		}
		if afterPublication {
			if err != nil {
				return fmt.Errorf("ordinary init published target is missing: %s: %w", write.Path, err)
			}
			if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
				return fmt.Errorf("ordinary init published target is not a regular non-symlink file: %s", write.Path)
			}
			continue
		}
		if !os.IsNotExist(err) {
			if err == nil {
				return fmt.Errorf("ordinary init create-only target now exists: %s", write.Path)
			}
			return err
		}
	}
	return nil
}

func ordinaryInitPublicationsCurrent(root *os.Root, caseRoot string, published ordinaryInitRollbackState) error {
	for index := range published.files {
		file := &published.files[index]
		rel, err := ordinaryInitRelative(caseRoot, file.publication.write.TargetPath)
		if err != nil {
			return err
		}
		data, readErr := root.ReadFile(rel)
		info, statErr := root.Lstat(rel)
		var handleInfo os.FileInfo
		var handleErr error
		if file.handle == nil {
			handleErr = fmt.Errorf("ordinary init rollback handle is missing")
		} else {
			handleInfo, handleErr = file.handle.Stat()
		}
		if readErr != nil || statErr != nil || handleErr != nil || file.info == nil || !bytes.Equal(data, file.publication.data) || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || !os.SameFile(file.info, info) || !os.SameFile(file.info, handleInfo) {
			return fmt.Errorf("ordinary init published target changed during publication: %s: %w", file.publication.write.Path, errors.Join(readErr, statErr, handleErr))
		}
	}
	return nil
}

func validateOrdinaryInitProjectSkillSource(plan InitPlan) error {
	for _, write := range plan.Writes {
		if write.Kind != "project-local-steamai-skill" ||
			len(write.rawContent) == 0 {
			continue
		}
		current, err := skillcontract.ReadValidatedProjectTemplate(plan.RepoRoot)
		if err != nil {
			return fmt.Errorf(
				"ordinary init skill provenance changed during publication: %s: %w",
				write.Path,
				err,
			)
		}
		if !bytes.Equal(
			sourceartifact.CanonicalText(current),
			write.rawContent,
		) {
			return fmt.Errorf(
				"ordinary init source changed during publication: %s",
				write.Path,
			)
		}
		return nil
	}
	return nil
}

func ordinaryInitSourcesCurrent(plan InitPlan) error {
	if err := validateOrdinaryInitProjectSkillSource(plan); err != nil {
		return err
	}
	manifestBytes, err := os.ReadFile(filepath.Join(plan.RepoRoot, "packs", filepath.FromSlash(plan.Pack), "manifest.yml"))
	if err != nil || !strings.EqualFold(sha256Bytes(sourceartifact.SemanticText(manifestBytes)), plan.initManifestSHA256) {
		return fmt.Errorf("ordinary init manifest changed during publication: %w", err)
	}
	gitignorePath := filepath.Join(plan.RepoRoot, "packs", filepath.FromSlash(plan.Pack), "examples", "gitignore.example")
	_, gitignoreErr := os.Lstat(gitignorePath)
	gitignorePresent := gitignoreErr == nil
	if (!gitignorePresent && !os.IsNotExist(gitignoreErr)) || gitignorePresent != plan.initGitignorePresent {
		return fmt.Errorf("ordinary init optional support source changed during publication: %s: %w", gitignorePath, gitignoreErr)
	}
	for _, write := range plan.Writes {
		current, err := initWriteSourceSHA256(plan, write)
		if err != nil {
			return fmt.Errorf("ordinary init source currentness failed at %s: %w", write.Path, err)
		}
		if !strings.EqualFold(current, plan.initSourceSHA256[write.Path]) {
			return fmt.Errorf("ordinary init source changed during publication: %s", write.Path)
		}
	}
	return nil
}

func ordinaryInitRollback(root *os.Root, caseRoot string, published ordinaryInitRollbackState) error {
	var rollbackErr error
	for index := len(published.files) - 1; index >= 0; index-- {
		file := &published.files[index]
		rel, err := ordinaryInitRelative(caseRoot, file.publication.write.TargetPath)
		if err != nil {
			rollbackErr = errors.Join(rollbackErr, err)
			continue
		}
		data, readErr := root.ReadFile(rel)
		info, statErr := root.Lstat(rel)
		var handleInfo os.FileInfo
		var handleErr error
		if file.handle == nil {
			handleErr = fmt.Errorf("ordinary init rollback handle is missing")
		} else {
			handleInfo, handleErr = file.handle.Stat()
		}
		if !file.dataKnown || readErr != nil || statErr != nil || handleErr != nil || file.info == nil || !bytes.Equal(data, file.rollbackData) || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || !os.SameFile(file.info, info) || !os.SameFile(file.info, handleInfo) {
			rollbackErr = errors.Join(rollbackErr,
				fmt.Errorf("ordinary init rollback preserved changed publication: %s", file.publication.write.Path),
				readErr, statErr, handleErr)
			continue
		}
		quarantine := ordinaryInitRollbackQuarantineName(rel, file.info, false)
		rebound, removeErr := ordinaryInitRemoveExact(root, rel, quarantine, file.handle, file.info, false, ordinaryInitRollbackAfterIdentityHook)
		file.handle = nil
		if removeErr != nil {
			rollbackErr = errors.Join(rollbackErr, removeErr)
			continue
		}
		if rebound {
			rollbackErr = errors.Join(rollbackErr, fmt.Errorf("ordinary init rollback publication name rebound: %s", rel))
		}
	}
	for index := len(published.directories) - 1; index >= 0; index-- {
		directory := &published.directories[index]
		info, statErr := root.Lstat(directory.rel)
		var handleInfo os.FileInfo
		var handleErr error
		if directory.handle == nil {
			handleErr = fmt.Errorf("ordinary init rollback directory handle is missing")
		} else {
			handleInfo, handleErr = directory.handle.Stat()
		}
		if statErr != nil || handleErr != nil || directory.info == nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || !os.SameFile(directory.info, info) || !os.SameFile(directory.info, handleInfo) {
			rollbackErr = errors.Join(rollbackErr,
				fmt.Errorf("ordinary init rollback preserved changed directory: %s", directory.rel), statErr, handleErr)
			continue
		}
		dir, openErr := root.Open(directory.rel)
		var entries []os.DirEntry
		var readErr, closeErr error
		if openErr == nil {
			entries, readErr = dir.ReadDir(-1)
			closeErr = dir.Close()
		}
		if openErr != nil || readErr != nil || closeErr != nil || len(entries) != 0 {
			rollbackErr = errors.Join(rollbackErr,
				fmt.Errorf("ordinary init rollback preserved non-empty directory: %s", directory.rel), openErr, readErr, closeErr)
			continue
		}
		quarantine := ordinaryInitRollbackQuarantineName(directory.rel, directory.info, true)
		rebound, removeErr := ordinaryInitRemoveExact(root, directory.rel, quarantine, directory.handle, directory.info, true, ordinaryInitRollbackAfterIdentityHook)
		directory.handle = nil
		if removeErr != nil {
			rollbackErr = errors.Join(rollbackErr, removeErr)
			continue
		}
		if rebound {
			rollbackErr = errors.Join(rollbackErr, fmt.Errorf("ordinary init rollback directory name rebound: %s", directory.rel))
		}
	}
	return rollbackErr
}

func ordinaryInitRollbackQuarantineName(rel string, info os.FileInfo, directory bool) string {
	kind := "file"
	if directory {
		kind = "directory"
	}
	identity := fmt.Appendf(nil, "%s\x00%s\x00%d\x00%d", kind, filepath.ToSlash(rel), info.Size(), info.ModTime().UnixNano())
	sum := sha256Bytes(identity)
	return filepath.Join(filepath.Dir(rel), ".rekit-ordinary-init-rollback-"+sum[:24])
}

func ordinaryInitWriteExclusive(root *os.Root, caseRoot string, publication ordinaryInitPublication) (ordinaryInitPublishedFile, []ordinaryInitPublishedDirectory, error) {
	rel, err := ordinaryInitRelative(caseRoot, publication.write.TargetPath)
	if err != nil {
		return ordinaryInitPublishedFile{}, nil, err
	}
	parent, name, directories, err := ordinaryInitParent(root, caseRoot, rel)
	if err != nil {
		return ordinaryInitPublishedFile{}, directories, err
	}
	defer parent.Close()
	file, err := parent.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return ordinaryInitPublishedFile{}, directories, fmt.Errorf("ordinary init create-only publication refuses collision at %s: %w", publication.write.Path, err)
	}
	created := ordinaryInitPublishedFile{publication: publication, created: true}
	created.info, err = file.Stat()
	if err != nil {
		file.Close()
		return created, directories, fmt.Errorf("ordinary init exclusive publication identity failed at %s: %w", publication.write.Path, err)
	}
	written, writeErr := file.Write(publication.data)
	syncErr := file.Sync()
	closeErr := file.Close()
	after, statErr := parent.Lstat(name)
	created.rollbackData, err = parent.ReadFile(name)
	created.dataKnown = err == nil && bytes.Equal(created.rollbackData, publication.data)
	if writeErr != nil || written != len(publication.data) || syncErr != nil || closeErr != nil || statErr != nil || err != nil || !after.Mode().IsRegular() || after.Mode()&os.ModeSymlink != 0 || !os.SameFile(created.info, after) || !created.dataKnown {
		return created, directories, fmt.Errorf("ordinary init exclusive publication failed at %s: %w", publication.write.Path, errors.Join(writeErr, syncErr, closeErr, statErr, err))
	}
	created.handle, err = ordinaryInitOpenRollbackHandleForApply(publication.write.TargetPath, false)
	if err != nil {
		return created, directories, fmt.Errorf("ordinary init exclusive rollback handle failed at %s: %w", publication.write.Path, err)
	}
	handleInfo, handleErr := created.handle.Stat()
	if handleErr != nil || !os.SameFile(created.info, handleInfo) {
		return created, directories, fmt.Errorf("ordinary init exclusive rollback handle identity failed at %s: %w", publication.write.Path, handleErr)
	}
	return created, directories, nil
}

func ordinaryInitParent(root *os.Root, caseRoot, rel string) (*os.Root, string, []ordinaryInitPublishedDirectory, error) {
	current, err := root.OpenRoot(".")
	if err != nil {
		return nil, "", nil, err
	}
	created := []ordinaryInitPublishedDirectory{}
	walked := []string{}
	for component := range strings.SplitSeq(filepath.Dir(rel), string(filepath.Separator)) {
		if component == "." {
			continue
		}
		if component == "" || component == ".." {
			current.Close()
			return nil, "", created, fmt.Errorf("ordinary init path contains an invalid parent: %s", rel)
		}
		walked = append(walked, component)
		before, statErr := current.Lstat(component)
		createdHere := false
		if os.IsNotExist(statErr) {
			if err := current.Mkdir(component, 0o755); err != nil {
				current.Close()
				return nil, "", created, err
			}
			createdHere = true
			before, statErr = current.Lstat(component)
		}
		if statErr != nil || !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
			current.Close()
			return nil, "", created, fmt.Errorf("ordinary init parent must be a non-symlink directory: %s", filepath.Join(walked...))
		}
		path := filepath.Join(caseRoot, filepath.Join(walked...))
		if err := rejectExclusiveInitReparsePath(path); err != nil {
			current.Close()
			return nil, "", created, err
		}
		next, err := current.OpenRoot(component)
		if err != nil {
			current.Close()
			return nil, "", created, err
		}
		opened, openErr := next.Lstat(".")
		after, afterErr := current.Lstat(component)
		if openErr != nil || afterErr != nil || !os.SameFile(before, opened) || !os.SameFile(opened, after) {
			next.Close()
			current.Close()
			return nil, "", created, fmt.Errorf("ordinary init parent changed while opening: %s", path)
		}
		if createdHere {
			created = append(created, ordinaryInitPublishedDirectory{rel: filepath.Join(walked...), info: opened})
			createdIndex := len(created) - 1
			handle, handleErr := ordinaryInitOpenRollbackHandleForApply(path, true)
			if handleErr != nil {
				next.Close()
				current.Close()
				return nil, "", created, handleErr
			}
			created[createdIndex].handle = handle
			handleInfo, handleStatErr := handle.Stat()
			if handleStatErr != nil || !handleInfo.IsDir() || !os.SameFile(opened, handleInfo) {
				next.Close()
				current.Close()
				return nil, "", created, fmt.Errorf("ordinary init created directory handle identity changed: %s", path)
			}
		}
		current.Close()
		current = next
	}
	return current, filepath.Base(rel), created, nil
}

func ordinaryInitRelative(caseRoot, target string) (string, error) {
	rel, err := filepath.Rel(caseRoot, target)
	if err != nil || filepath.IsAbs(rel) || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("ordinary init target escapes case root: %s", target)
	}
	return filepath.Clean(rel), nil
}
