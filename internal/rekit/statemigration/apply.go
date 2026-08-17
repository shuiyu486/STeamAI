package statemigration

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/kitmutation"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtimebundle"
	"github.com/shuiyu486/re-context-kits/internal/rekit/sourceartifact"
)

var (
	beforeCommitHook             func() error
	afterStateRenameHook         func() error
	beforeReceiptPublicationHook func() error
)

func SetBeforeCommitHookForTest(hook func() error) func() {
	previous := beforeCommitHook
	beforeCommitHook = hook
	return func() { beforeCommitHook = previous }
}

func SetAfterStateRenameHookForTest(hook func() error) func() {
	previous := afterStateRenameHook
	afterStateRenameHook = hook
	return func() { afterStateRenameHook = previous }
}

func SetBeforeReceiptPublicationHookForTest(hook func() error) func() {
	previous := beforeReceiptPublicationHook
	beforeReceiptPublicationHook = hook
	return func() { beforeReceiptPublicationHook = previous }
}

func Apply(repoRoot, caseRoot, pack, expectedPlanSHA256 string) (Result, error) {
	expectedPlanSHA256 = strings.ToLower(strings.TrimSpace(expectedPlanSHA256))
	if !validSHA256(expectedPlanSHA256) {
		return Result{}, fmt.Errorf("state migration Apply requires an exact 64-hex expected plan SHA-256")
	}
	caseRoot, err := filepath.Abs(strings.TrimSpace(caseRoot))
	if err != nil {
		return Result{}, err
	}

	if current, currentErr := build(repoRoot, caseRoot, pack); currentErr == nil && current.AlreadyCurrent {
		return applyAlreadyCurrent(current, expectedPlanSHA256)
	}

	lease, err := kitmutation.Acquire(caseRoot)
	if err != nil {
		return Result{}, err
	}
	defer lease.Unlock()
	plan, err := build(repoRoot, caseRoot, pack)
	if err != nil {
		return Result{}, err
	}
	if plan.AlreadyCurrent {
		return applyAlreadyCurrent(plan, expectedPlanSHA256)
	}
	if plan.prepared == nil || !strings.EqualFold(plan.ExpectedPlanSHA256, expectedPlanSHA256) {
		return Result{}, fmt.Errorf("state migration plan SHA-256 mismatch: expected=%s current=%s", expectedPlanSHA256, plan.ExpectedPlanSHA256)
	}
	if err := applyPrepared(plan); err != nil {
		return Result{}, err
	}
	return Result{
		SchemaVersion: SchemaVersion, Kind: ResultKind, Command: Command, Status: "migrated", CaseRoot: plan.CaseRoot, Pack: plan.Pack,
		IsMutation: true, Applied: true, Replay: false, AlreadyCurrent: false, PlanSHA256: plan.ExpectedPlanSHA256,
		ReceiptPath: filepath.Join(plan.CaseRoot, filepath.FromSlash(ReceiptRel)), Receipt: &plan.prepared.receiptValue, Writes: plan.Writes,
		NextSteps: []string{"continue from the project-local /steamai entrypoint", "run status and doctor from the migrated project"},
	}, nil
}

func applyAlreadyCurrent(plan Plan, expectedPlanSHA256 string) (Result, error) {
	if !plan.Replay {
		return Result{}, fmt.Errorf("state migration project is already current; preview is the no-op result and no Apply plan SHA-256 exists")
	}
	if plan.prepared == nil || !strings.EqualFold(plan.ExpectedPlanSHA256, expectedPlanSHA256) {
		return Result{}, fmt.Errorf("state migration replay plan SHA-256 differs from durable receipt")
	}
	return replayResult(plan), nil
}

func applyPrepared(plan Plan) error {
	prepared := plan.prepared
	caseRoot, caseIdentity, err := OpenRootIdentity(plan.CaseRoot)
	if err != nil {
		return err
	}
	defer caseRoot.Close()
	if caseIdentity != plan.CaseRootIdentity {
		return fmt.Errorf("state migration project root identity changed after preview")
	}
	openedCase, err := caseRoot.Lstat(".")
	if err != nil || !os.SameFile(prepared.caseInfo, openedCase) {
		return fmt.Errorf("state migration project root changed after preview")
	}
	if _, err := caseRoot.Lstat(".steamai"); !os.IsNotExist(err) {
		if err == nil {
			return fmt.Errorf("state migration target .steamai appeared after preview")
		}
		return err
	}
	legacyRoot, sourceIdentity, err := OpenRootIdentity(filepath.Join(plan.CaseRoot, ".rekit"))
	if err != nil {
		return err
	}
	defer legacyRoot.Close()
	if sourceIdentity != plan.SourceRootIdentity {
		return fmt.Errorf("legacy state root identity changed after preview")
	}
	openedSource, err := legacyRoot.Lstat(".")
	if err != nil || !os.SameFile(prepared.sourceInfo, openedSource) {
		return fmt.Errorf("legacy state root changed after preview")
	}
	entries, _, inventory, err := inventoryRoot(legacyRoot, ".rekit")
	if err != nil {
		return err
	}
	if inventory.SHA256 != plan.LegacyInventory.SHA256 || !inventoryEntriesEqual(entries, prepared.legacyEntries) {
		return fmt.Errorf("legacy state inventory changed after preview")
	}
	if err := validateExternalBindings(plan, prepared); err != nil {
		return err
	}
	if err := validatePublicationSources(plan, prepared.publications); err != nil {
		return err
	}
	if err := validatePublicationTargetsMissing(legacyRoot, prepared.publications); err != nil {
		return err
	}
	if beforeCommitHook != nil {
		if err := beforeCommitHook(); err != nil {
			return err
		}
	}
	if current, err := caseRoot.Lstat(".rekit"); err != nil || !os.SameFile(prepared.sourceInfo, current) {
		return fmt.Errorf("legacy state root changed immediately before migration commit")
	}
	if _, err := caseRoot.Lstat(".steamai"); !os.IsNotExist(err) {
		return fmt.Errorf("migration target .steamai is no longer missing")
	}
	entries, _, inventory, err = inventoryRoot(legacyRoot, ".rekit")
	if err != nil {
		return err
	}
	if inventory.SHA256 != plan.LegacyInventory.SHA256 || !inventoryEntriesEqual(entries, prepared.legacyEntries) {
		return fmt.Errorf("legacy state inventory changed immediately before migration commit")
	}
	if err := validateExternalBindings(plan, prepared); err != nil {
		return err
	}
	if err := validatePublicationSources(plan, prepared.publications); err != nil {
		return err
	}
	if err := validatePublicationTargetsMissing(legacyRoot, prepared.publications); err != nil {
		return err
	}
	if err := legacyRoot.Close(); err != nil {
		return err
	}
	if err := caseRoot.Rename(".rekit", ".steamai"); err != nil {
		return fmt.Errorf("atomically rename legacy state root: %w", err)
	}

	// From this point onward any failure intentionally leaves a partial .steamai
	// and no .rekit. The empty migration namespace is the deterministic fence that
	// makes every projectstate owner fail closed until the receipt is published.
	if err := caseRoot.Mkdir(filepath.Join(".steamai", "migration"), 0o700); err != nil {
		return fmt.Errorf("create state migration commit fence: %w", err)
	}
	if afterStateRenameHook != nil {
		if err := afterStateRenameHook(); err != nil {
			return err
		}
	}
	currentInfo, err := caseRoot.Lstat(".steamai")
	if err != nil {
		return fmt.Errorf("inspect renamed state root identity: %w", err)
	}
	if !os.SameFile(prepared.sourceInfo, currentInfo) {
		return fmt.Errorf("renamed state root identity changed")
	}
	currentRoot, currentIdentity, err := OpenRootIdentity(filepath.Join(plan.CaseRoot, ".steamai"))
	if err != nil {
		return err
	}
	defer currentRoot.Close()
	if currentIdentity != plan.SourceRootIdentity {
		return fmt.Errorf("renamed state root filesystem identity changed")
	}
	if err := publishCurrentRoot(currentRoot, prepared.publications); err != nil {
		return err
	}
	currentSkillRel := filepath.ToSlash(filepath.Join(".claude", "skills", "steamai", "SKILL.md"))
	currentSkillData, err := plannedSkillData(plan)
	if err != nil {
		return err
	}
	if _, err := rekitfs.WriteExclusiveRegularFileAnchoredWriteThrough(plan.CaseRoot, currentSkillRel, "project-local STeamAI skill", currentSkillData); err != nil {
		return err
	}
	legacySkillRel := filepath.ToSlash(filepath.Join(".claude", "skills", "rekit", "SKILL.md"))
	legacySkillPath := filepath.Join(plan.CaseRoot, filepath.FromSlash(legacySkillRel))
	legacySkill, err := rekitfs.ReadStableRegularFileAnchored(plan.CaseRoot, legacySkillPath, "legacy project skill", maxSkill)
	if err != nil {
		return fmt.Errorf("read legacy project skill before retirement: %w", err)
	}
	if !bytes.Equal(legacySkill, prepared.legacySkill) {
		return fmt.Errorf("legacy project skill changed before retirement")
	}
	if err := removeExactRegularFile(plan.CaseRoot, legacySkillRel, prepared.legacySkill); err != nil {
		return err
	}
	if err := removeEmptyLegacySkillDirs(caseRoot); err != nil {
		return err
	}
	receipt, ok := publicationFor(prepared.publications, strings.TrimPrefix(ReceiptRel, ".steamai/"))
	if !ok {
		return fmt.Errorf("migration plan omits durable receipt publication")
	}
	receiptRel := filepath.FromSlash(receipt.rel)
	receiptTempRel := filepath.Join(filepath.Dir(receiptRel), ".state-root-v1.json.state-migration.tmp")
	if err := createRootFile(currentRoot, receiptTempRel, receipt.data, receipt.mode); err != nil {
		return fmt.Errorf("stage durable state migration receipt: %w", err)
	}
	defer func() {
		// If a concurrent target appeared, retain the temp entry so the
		// migration namespace cannot be mistaken for an exact committed marker.
		if _, err := currentRoot.Lstat(receiptRel); os.IsNotExist(err) {
			_ = currentRoot.Remove(receiptTempRel)
		}
	}()
	if beforeReceiptPublicationHook != nil {
		if err := beforeReceiptPublicationHook(); err != nil {
			return err
		}
	}
	if err := validateMigratedBeforeReceipt(plan, caseRoot, currentRoot, receiptTempRel); err != nil {
		return err
	}
	// Link publishes the exact staged inode without overwriting a late target.
	// Until the temp link is removed, the namespace has two entries and every
	// projectstate owner remains fenced. Removal is therefore the commit point.
	if err := currentRoot.Link(receiptTempRel, receiptRel); err != nil {
		return fmt.Errorf("publish durable state migration receipt without overwrite: %w", err)
	}
	if err := currentRoot.Remove(receiptTempRel); err != nil {
		return fmt.Errorf("commit durable state migration receipt: %w", err)
	}
	return nil
}

func validateExternalBindings(plan Plan, prepared *preparedPlan) error {
	legacySkillPath := filepath.Join(plan.CaseRoot, filepath.FromSlash(plan.LegacySkill.Path))
	legacySkill, err := rekitfs.ReadStableRegularFileAnchored(plan.CaseRoot, legacySkillPath, "legacy project skill", maxSkill)
	if err != nil {
		return fmt.Errorf("read legacy project skill after preview: %w", err)
	}
	if !bytes.Equal(legacySkill, prepared.legacySkill) {
		return fmt.Errorf("legacy project skill changed after preview")
	}
	currentSkillPath := filepath.Join(plan.CaseRoot, filepath.FromSlash(plan.CurrentSkill.Path))
	if exists, _, err := lstatOptional(currentSkillPath); err != nil {
		return err
	} else if exists {
		return fmt.Errorf("current project skill appeared after preview")
	}
	if plan.LegacyMetadata != nil {
		path := filepath.Join(plan.CaseRoot, filepath.FromSlash(plan.LegacyMetadata.Path))
		data, err := rekitfs.ReadStableRegularFileAnchored(plan.CaseRoot, path, "legacy project metadata", maxMetadata)
		if err != nil {
			return fmt.Errorf("read legacy project metadata after preview: %w", err)
		}
		if !bytes.Equal(data, prepared.legacyMetadata) {
			return fmt.Errorf("legacy project metadata changed after preview")
		}
	} else if exists, _, err := lstatOptional(filepath.Join(plan.CaseRoot, ".re-template.yml")); err != nil {
		return err
	} else if exists {
		return fmt.Errorf("legacy project metadata appeared after preview")
	}
	return nil
}

func validatePublicationSources(plan Plan, publications []plannedFile) error {
	for _, publication := range publications {
		if publication.source == "" {
			continue
		}
		data, err := stableSource(publication.source)
		if err != nil {
			return fmt.Errorf("read migration publication source after preview: %s: %w", publication.source, err)
		}
		if !bytes.Equal(data, publication.data) {
			return fmt.Errorf("migration publication source changed after preview: %s", publication.source)
		}
	}
	currentSkill, err := plannedSkillData(plan)
	if err != nil {
		return err
	}
	if sha256Hex(currentSkill) != plan.CurrentSkill.SHA256 {
		return fmt.Errorf("current project skill source changed after preview")
	}
	return nil
}

func validatePublicationTargetsMissing(root *os.Root, publications []plannedFile) error {
	for _, publication := range publications {
		if publication.rel == "instance.yml" || publication.rel == "state.json" {
			continue
		}
		if _, err := root.Lstat(filepath.FromSlash(publication.rel)); err == nil {
			return fmt.Errorf("legacy state root contains a partial current publication target: %s", publication.rel)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func publishCurrentRoot(root *os.Root, publications []plannedFile) error {
	for _, metadata := range []string{"state.json", "instance.yml"} {
		publication, ok := publicationFor(publications, metadata)
		if !ok {
			return fmt.Errorf("migration plan omits %s", metadata)
		}
		if err := replaceRootFile(root, filepath.FromSlash(publication.rel), publication.data, publication.mode); err != nil {
			return err
		}
	}
	for _, publication := range publications {
		if publication.rel == "instance.yml" || publication.rel == "state.json" || publication.rel == strings.TrimPrefix(ReceiptRel, ".steamai/") {
			continue
		}
		if err := createRootFile(root, filepath.FromSlash(publication.rel), publication.data, publication.mode); err != nil {
			return err
		}
	}
	return nil
}

func publicationFor(publications []plannedFile, rel string) (plannedFile, bool) {
	for _, publication := range publications {
		if publication.rel == rel {
			return publication, true
		}
	}
	return plannedFile{}, false
}

func createRootFile(root *os.Root, rel string, data []byte, mode os.FileMode) error {
	if err := mkdirRootNoFollow(root, filepath.Dir(rel)); err != nil {
		return err
	}
	file, err := root.OpenFile(rel, os.O_WRONLY|os.O_CREATE|os.O_EXCL|os.O_SYNC, mode.Perm())
	if err != nil {
		return err
	}
	written, writeErr := file.Write(data)
	syncErr := file.Sync()
	closeErr := file.Close()
	if writeErr != nil || written != len(data) || syncErr != nil || closeErr != nil {
		_ = root.Remove(rel)
		return fmt.Errorf("publish migration file %s: %w", filepath.ToSlash(rel), errors.Join(writeErr, syncErr, closeErr))
	}
	if mode.Perm()&0o111 != 0 {
		if err := root.Chmod(rel, mode.Perm()); err != nil {
			return err
		}
	}
	return validateRootFile(root, rel, data)
}

func mkdirRootNoFollow(root *os.Root, rel string) error {
	clean := filepath.Clean(rel)
	if clean == "." {
		return nil
	}
	current := ""
	for component := range strings.SplitSeq(clean, string(filepath.Separator)) {
		if component == "" || component == "." || component == ".." {
			return fmt.Errorf("migration publication directory is invalid: %s", rel)
		}
		current = filepath.Join(current, component)
		info, err := root.Lstat(current)
		if os.IsNotExist(err) {
			if err := root.Mkdir(current, 0o700); err != nil && !os.IsExist(err) {
				return err
			}
			info, err = root.Lstat(current)
		}
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("migration publication directory component is not a non-symlink directory: %s", filepath.ToSlash(current))
		}
	}
	return nil
}

func replaceRootFile(root *os.Root, rel string, data []byte, mode os.FileMode) error {
	before, err := root.Lstat(rel)
	if err != nil || before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return fmt.Errorf("migration replacement target must be an existing regular file: %s", filepath.ToSlash(rel))
	}
	temp := "." + filepath.Base(rel) + ".state-migration.tmp"
	temp = filepath.Join(filepath.Dir(rel), temp)
	if _, err := root.Lstat(temp); err == nil {
		return fmt.Errorf("migration temporary target already exists: %s", filepath.ToSlash(temp))
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := createRootFile(root, temp, data, mode); err != nil {
		return err
	}
	if err := root.Rename(temp, rel); err != nil {
		_ = root.Remove(temp)
		return fmt.Errorf("replace migration file %s: %w", filepath.ToSlash(rel), err)
	}
	return validateRootFile(root, rel, data)
}

func validateRootFile(root *os.Root, rel string, expected []byte) error {
	info, err := root.Lstat(rel)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() != int64(len(expected)) {
		return fmt.Errorf("published migration file is invalid: %s", filepath.ToSlash(rel))
	}
	data, err := readRootFile(root, rel, info, int64(len(expected))+1)
	if err != nil {
		return fmt.Errorf("read published migration file %s: %w", filepath.ToSlash(rel), err)
	}
	if !bytes.Equal(data, expected) {
		return fmt.Errorf("published migration file bytes differ: %s", filepath.ToSlash(rel))
	}
	return nil
}

func removeExactRegularFile(caseRoot, rel string, expected []byte) error {
	root, err := os.OpenRoot(caseRoot)
	if err != nil {
		return err
	}
	defer root.Close()
	info, err := root.Lstat(filepath.FromSlash(rel))
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("legacy project skill is no longer a regular file")
	}
	data, err := readRootFile(root, filepath.FromSlash(rel), info, int64(len(expected))+1)
	if err != nil {
		return fmt.Errorf("read legacy project skill before removal: %w", err)
	}
	if !bytes.Equal(data, expected) {
		return fmt.Errorf("legacy project skill changed before removal")
	}
	return root.Remove(filepath.FromSlash(rel))
}

func removeEmptyLegacySkillDirs(root *os.Root) error {
	rel := filepath.Join(".claude", "skills", "rekit")
	file, err := root.Open(rel)
	if err != nil {
		return err
	}
	entries, readErr := file.ReadDir(1)
	closeErr := file.Close()
	if readErr != nil && readErr != io.EOF {
		return readErr
	}
	if closeErr != nil {
		return closeErr
	}
	if len(entries) != 0 {
		return fmt.Errorf("legacy project skill directory contains unplanned residue")
	}
	return root.Remove(rel)
}

func plannedSkillData(plan Plan) ([]byte, error) {
	for _, write := range plan.Writes {
		if write.Kind == "project-local-steamai-skill" {
			path := filepath.Join(plan.RepoRoot, "rekit", "templates", "steamai-project", "SKILL.md")
			data, err := sourceartifact.ReadCanonical(path)
			if err != nil {
				return nil, fmt.Errorf("read canonical project-local STeamAI skill after preview: %w", err)
			}
			if sha256Hex(data) != plan.CurrentSkill.SHA256 {
				return nil, fmt.Errorf("canonical project-local STeamAI skill changed after preview")
			}
			return data, nil
		}
	}
	return nil, fmt.Errorf("migration plan omits current project skill")
}

func validateMigratedBeforeReceipt(plan Plan, caseRoot, currentRoot *os.Root, receiptTempRel string) error {
	openedCase, err := caseRoot.Lstat(".")
	if err != nil || !os.SameFile(plan.prepared.caseInfo, openedCase) {
		return fmt.Errorf("state migration project root changed before receipt commit")
	}
	if _, err := rekitfs.ValidateNonReparseDirectory(plan.CaseRoot, "state migration project root before receipt commit"); err != nil {
		return err
	}
	lexicalCase, lexicalIdentity, err := OpenRootIdentity(plan.CaseRoot)
	if err != nil {
		return fmt.Errorf("reopen state migration project root before receipt commit: %w", err)
	}
	lexicalCaseInfo, lexicalCaseErr := lexicalCase.Lstat(".")
	lexicalCurrentInfo, lexicalCurrentErr := lexicalCase.Lstat(".steamai")
	lexicalCloseErr := lexicalCase.Close()
	if lexicalCaseErr != nil || lexicalCurrentErr != nil || lexicalCloseErr != nil {
		return fmt.Errorf("revalidate state migration project path before receipt commit: %w", errors.Join(lexicalCaseErr, lexicalCurrentErr, lexicalCloseErr))
	}
	if lexicalIdentity != plan.CaseRootIdentity || !os.SameFile(openedCase, lexicalCaseInfo) {
		return fmt.Errorf("state migration project path identity changed before receipt commit")
	}
	currentAtCaseRoot, err := caseRoot.Lstat(".steamai")
	if err != nil {
		return fmt.Errorf("inspect current state root before receipt commit: %w", err)
	}
	openedCurrent, err := currentRoot.Lstat(".")
	if err != nil || !os.SameFile(currentAtCaseRoot, openedCurrent) || !os.SameFile(openedCurrent, lexicalCurrentInfo) {
		return fmt.Errorf("current state root path changed before receipt commit")
	}
	if _, err := runtimebundle.Validate(filepath.Join(plan.CaseRoot, ".steamai"), runtimebundle.ManifestRel, plan.BundleManifest.SHA256, plan.Pack); err != nil {
		return err
	}
	identity, err := IdentityForRoot(currentRoot)
	if err != nil {
		return err
	}
	if identity != plan.SourceRootIdentity {
		return fmt.Errorf("migrated state root identity differs")
	}
	entries, _, _, err := inventoryRoot(currentRoot, ".steamai")
	if err != nil {
		return err
	}
	expectedEntries := append([]InventoryEntry(nil), plan.prepared.plannedEntries...)
	expectedEntries = append(expectedEntries,
		InventoryEntry{Path: filepath.ToSlash(receiptTempRel), Kind: "file", SHA256: sha256Hex(plan.prepared.receipt), Size: int64(len(plan.prepared.receipt))},
		InventoryEntry{Path: "migration", Kind: "directory"},
	)
	if !inventoryEntriesEqual(entries, expectedEntries) {
		return fmt.Errorf("migrated state inventory differs from exact pre-commit plan")
	}
	if plan.prepared.receiptValue.After.InventorySHA != inventoryForEntries(".steamai", plan.prepared.plannedEntries).SHA256 {
		return fmt.Errorf("migration receipt after inventory binding differs")
	}
	if _, err := caseRoot.Lstat(".rekit"); !os.IsNotExist(err) {
		return fmt.Errorf("legacy state root still exists after migration")
	}
	if _, err := caseRoot.Lstat(filepath.Join(".claude", "skills", "rekit", "SKILL.md")); !os.IsNotExist(err) {
		return fmt.Errorf("legacy project skill still exists after migration")
	}
	currentSkillPath := filepath.Join(plan.CaseRoot, filepath.FromSlash(plan.CurrentSkill.Path))
	currentSkill, err := rekitfs.ReadStableRegularFileAnchored(plan.CaseRoot, currentSkillPath, "current project skill before migration commit", maxSkill)
	if err != nil {
		return err
	}
	if sha256Hex(currentSkill) != plan.CurrentSkill.SHA256 || int64(len(currentSkill)) != plan.CurrentSkill.Size {
		return fmt.Errorf("current project skill differs before migration commit")
	}
	return validateRootFile(currentRoot, receiptTempRel, plan.prepared.receipt)
}

func inventoryWithoutReceipt(entries []InventoryEntry) Inventory {
	out := make([]InventoryEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.Path == "migration" || entry.Path == strings.TrimPrefix(ReceiptRel, ".steamai/") {
			continue
		}
		out = append(out, entry)
	}
	return inventoryForEntries(".steamai", out)
}

func inventoryEntriesEqual(left, right []InventoryEntry) bool {
	if len(left) != len(right) {
		return false
	}
	left = append([]InventoryEntry(nil), left...)
	right = append([]InventoryEntry(nil), right...)
	sortEntries := func(entries []InventoryEntry) {
		for i := 0; i < len(entries); i++ {
			for j := i + 1; j < len(entries); j++ {
				if entries[j].Path < entries[i].Path {
					entries[i], entries[j] = entries[j], entries[i]
				}
			}
		}
	}
	sortEntries(left)
	sortEntries(right)
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func replayResult(plan Plan) Result {
	receipt := plan.prepared.receiptValue
	return Result{
		SchemaVersion: SchemaVersion, Kind: ResultKind, Command: Command, Status: "already-current", CaseRoot: plan.CaseRoot, Pack: plan.Pack,
		IsMutation: false, Applied: false, Replay: true, AlreadyCurrent: true, PlanSHA256: receipt.PlanSHA256,
		ReceiptPath: filepath.Join(plan.CaseRoot, filepath.FromSlash(ReceiptRel)), Receipt: &receipt,
		NextSteps: []string{"continue using the project-local /steamai entrypoint"},
	}
}
