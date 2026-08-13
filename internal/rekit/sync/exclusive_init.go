package sync

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/casebind"
	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/review"
	"github.com/shuiyu486/re-context-kits/internal/rekit/sourceartifact"
)

// ExclusiveInitOptions supplies the stable identity and optional caller-owned
// marker used when provisioning an isolated verification case.
type ExclusiveInitOptions struct {
	ProjectName             string
	ProvisionID             string
	Role                    string
	CreatedAt               time.Time
	SkipVerificationMarker  bool
	DefaultPublicationPhase int
	ExtraFiles              []ExclusiveInitExtraFile
}

// ExclusiveInitExtraFile adds one deterministic case-local regular file to
// the exclusive initialization package.
type ExclusiveInitExtraFile struct {
	Path             string
	Kind             string
	Content          []byte
	PublicationPhase int
}

// ExclusiveInitWrite is an exact leaf included in an exclusive init package.
type ExclusiveInitWrite struct {
	Path             string `json:"path"`
	Kind             string `json:"kind"`
	TargetPath       string `json:"targetPath"`
	SHA256           string `json:"sha256"`
	Size             int64  `json:"size"`
	Content          []byte `json:"content"`
	PublicationPhase int    `json:"publicationPhase,omitempty"`
}

// ExclusiveInitPlan is a deterministic, complete, no-overwrite case package.
type ExclusiveInitPlan struct {
	SchemaVersion  int                  `json:"schemaVersion"`
	Command        string               `json:"command"`
	CaseRoot       string               `json:"caseRoot"`
	RepoRoot       string               `json:"repoRoot"`
	Pack           string               `json:"pack"`
	ProjectName    string               `json:"projectName"`
	ProvisionID    string               `json:"provisionId"`
	Role           string               `json:"role"`
	CreatedAt      string               `json:"createdAt"`
	Writes         []ExclusiveInitWrite `json:"writes"`
	BlockedActions []string             `json:"blockedActions"`
}

// ExclusiveInitResult describes a successful first apply or exact replay.
type ExclusiveInitResult struct {
	SchemaVersion int                  `json:"schemaVersion"`
	Command       string               `json:"command"`
	CaseRoot      string               `json:"caseRoot"`
	RepoRoot      string               `json:"repoRoot"`
	Pack          string               `json:"pack"`
	ProjectName   string               `json:"projectName"`
	ProvisionID   string               `json:"provisionId"`
	Role          string               `json:"role"`
	CreatedAt     string               `json:"createdAt"`
	Applied       bool                 `json:"applied"`
	Replay        bool                 `json:"replay"`
	Writes        []ExclusiveInitWrite `json:"writes"`
}

// ExclusiveInitBatch reserves every missing root before any planned leaf is
// created. This lets callers provision related roots without a half-created
// first root when a later root collides during reservation.
type ExclusiveInitBatch struct {
	plans     []ExclusiveInitPlan
	parents   []*os.Root
	roots     []*os.Root
	rootNames []string
	rootInfos []os.FileInfo
	created   []bool
	closed    bool
}

// These package-private hooks are deterministic test seams. Production leaves
// them nil.
var (
	exclusiveInitAfterParentPinHook func() error
	exclusiveInitAfterPreflightHook func() error
	exclusiveInitLeafWriteHook      func(stage, path string) error
)

// SetExclusiveInitLeafWriteHookForTest installs a deterministic package-test seam.
func SetExclusiveInitLeafWriteHookForTest(hook func(stage, path string) error) func() {
	previous := exclusiveInitLeafWriteHook
	exclusiveInitLeafWriteHook = hook
	return func() { exclusiveInitLeafWriteHook = previous }
}

// PlanExclusiveInit builds all exact bytes required for a doctor-ready
// attached case. It is read-only and permits planning only for a missing root.
func PlanExclusiveInit(repoRoot, caseRoot, pack string, opt ExclusiveInitOptions) (ExclusiveInitPlan, error) {
	return planExclusiveInit(repoRoot, caseRoot, pack, opt, false)
}

// PlanExclusiveInitReplay rebuilds the deterministic package for an existing
// exact or partial exclusive-init tree so ApplyExclusiveInit can resume it.
func PlanExclusiveInitReplay(repoRoot, caseRoot, pack string, opt ExclusiveInitOptions) (ExclusiveInitPlan, error) {
	return planExclusiveInit(repoRoot, caseRoot, pack, opt, true)
}

func planExclusiveInit(repoRoot, caseRoot, pack string, opt ExclusiveInitOptions, allowExisting bool) (ExclusiveInitPlan, error) {
	caseFull, err := filepath.Abs(caseRoot)
	if err != nil {
		return ExclusiveInitPlan{}, err
	}
	repoFull, err := filepath.Abs(repoRoot)
	if err != nil {
		return ExclusiveInitPlan{}, err
	}
	if casebind.SamePath(caseFull, repoFull) {
		return ExclusiveInitPlan{}, fmt.Errorf("exclusive init target must be an external case directory, not the kit repo root: %s", caseFull)
	}
	if _, err := os.Lstat(caseFull); err == nil && !allowExisting {
		return ExclusiveInitPlan{}, fmt.Errorf("exclusive init refuses existing case root: %s", caseFull)
	} else if err != nil && !os.IsNotExist(err) {
		return ExclusiveInitPlan{}, err
	}
	projectName := strings.TrimSpace(opt.ProjectName)
	if projectName == "" {
		return ExclusiveInitPlan{}, fmt.Errorf("exclusive init requires ProjectName")
	}
	provisionID := strings.TrimSpace(opt.ProvisionID)
	if provisionID == "" {
		return ExclusiveInitPlan{}, fmt.Errorf("exclusive init requires ProvisionID")
	}
	role := strings.TrimSpace(opt.Role)
	if role == "" {
		return ExclusiveInitPlan{}, fmt.Errorf("exclusive init requires Role")
	}
	if opt.CreatedAt.IsZero() {
		return ExclusiveInitPlan{}, fmt.Errorf("exclusive init requires CreatedAt")
	}
	createdAt := opt.CreatedAt.UTC().Format(time.RFC3339Nano)
	m, err := manifest.Load(repoFull, pack)
	if err != nil {
		return ExclusiveInitPlan{}, err
	}
	if err := m.ValidateSchema(); err != nil {
		return ExclusiveInitPlan{}, err
	}

	if opt.DefaultPublicationPhase < 0 {
		return ExclusiveInitPlan{}, fmt.Errorf("exclusive init requires a non-negative default publication phase")
	}
	builder := exclusiveInitBuilder{caseRoot: caseFull, paths: map[string]struct{}{}, defaultPhase: opt.DefaultPublicationPhase}
	if err := builder.add(".rekit/instance.yml", "instance-metadata", []byte(casebind.InstanceText(caseFull, repoFull, pack, projectName))); err != nil {
		return ExclusiveInitPlan{}, err
	}
	shimPath := filepath.Join(repoFull, "rekit", "templates", "case-shim", "SKILL.md")
	shim, err := sourceartifact.ReadCanonical(shimPath)
	if err != nil {
		return ExclusiveInitPlan{}, fmt.Errorf("missing case shim template: %s", shimPath)
	}
	if err := builder.add(".claude/skills/rekit/SKILL.md", "case-local-thin-shim", shim); err != nil {
		return ExclusiveInitPlan{}, err
	}
	legacy := "templateRoot: " + repoFull + "\r\n" +
		"rekitMode: case-local-shim\r\n" +
		"templatePack: " + pack + "\r\n" +
		"templateVersion: 0.0.0\r\n"
	if err := builder.add(".re-template.yml", "legacy-metadata", []byte(legacy)); err != nil {
		return ExclusiveInitPlan{}, err
	}

	for _, rel := range m.ManagedFiles {
		source, err := m.SourcePath(rel)
		if err != nil {
			return ExclusiveInitPlan{}, err
		}
		content, err := sourceartifact.ReadCanonical(source)
		if err != nil {
			return ExclusiveInitPlan{}, err
		}
		if err := builder.add(rel, "managed-file", content); err != nil {
			return ExclusiveInitPlan{}, err
		}
	}
	for _, rel := range m.TemplateFiles {
		source, err := m.SourcePath(rel)
		if err != nil {
			return ExclusiveInitPlan{}, err
		}
		content, err := sourceartifact.ReadCanonical(source)
		if err != nil {
			return ExclusiveInitPlan{}, err
		}
		planned := strings.ReplaceAll(string(content), "<PROJECT_NAME>", projectName)
		planned = strings.ReplaceAll(planned, "<PROJECT_ROOT>", caseFull)
		if err := builder.add(strings.TrimSuffix(rel, ".template.md")+".md", "template-file", []byte(planned)); err != nil {
			return ExclusiveInitPlan{}, err
		}
	}
	blockSource, err := m.SourcePath(m.ManagedBlock["source"])
	if err != nil {
		return ExclusiveInitPlan{}, err
	}
	block, err := sourceartifact.ReadCanonical(blockSource)
	if err != nil {
		return ExclusiveInitPlan{}, err
	}
	blockHost := review.ApplyManagedBlock("", m.ManagedBlock["blockId"], string(block))
	if err := builder.add(m.ManagedBlock["file"], "managed-block", []byte(blockHost)); err != nil {
		return ExclusiveInitPlan{}, err
	}
	if source, err := m.SourcePath("examples/gitignore.example"); err == nil {
		if content, readErr := sourceartifact.ReadCanonical(source); readErr == nil {
			if err := builder.add(".gitignore", "support-file", content); err != nil {
				return ExclusiveInitPlan{}, err
			}
		} else if !os.IsNotExist(readErr) {
			return ExclusiveInitPlan{}, readErr
		}
	}
	managedState := map[string]syncManagedEntry{}
	for _, rel := range m.ManagedFiles {
		source, err := m.SourcePath(rel)
		if err != nil {
			return ExclusiveInitPlan{}, err
		}
		content, err := sourceartifact.ReadCanonical(source)
		if err != nil {
			return ExclusiveInitPlan{}, err
		}
		hash := sha256Bytes(content)
		managedState[rel] = syncManagedEntry{SourceHash: hash, TargetHashAtSync: hash, LastAction: "sync"}
	}
	state := syncState{SchemaVersion: 1, TemplateRoot: repoFull, TemplatePack: pack, LastSyncAt: createdAt, Managed: managedState}
	stateBytes, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return ExclusiveInitPlan{}, err
	}
	if err := builder.add(".rekit/state.json", "initial-state", append(stateBytes, '\n')); err != nil {
		return ExclusiveInitPlan{}, err
	}
	if !opt.SkipVerificationMarker {
		marker := struct {
			SchemaVersion int    `json:"schemaVersion"`
			Kind          string `json:"kind"`
			ProvisionID   string `json:"provisionId"`
			Role          string `json:"role"`
			CreatedAt     string `json:"createdAt"`
		}{SchemaVersion: 1, Kind: "exclusive-case-provision", ProvisionID: provisionID, Role: role, CreatedAt: createdAt}
		markerBytes, err := json.MarshalIndent(marker, "", "  ")
		if err != nil {
			return ExclusiveInitPlan{}, err
		}
		if err := builder.add(".rekit/verification-role.json", "verification-role-marker", append(markerBytes, '\n'), 0); err != nil {
			return ExclusiveInitPlan{}, err
		}
	}
	for _, extra := range opt.ExtraFiles {
		kind := strings.TrimSpace(extra.Kind)
		if kind == "" {
			kind = "caller-extra"
		}
		if extra.PublicationPhase < 0 {
			return ExclusiveInitPlan{}, fmt.Errorf("exclusive init extra file has invalid publication phase: %s", extra.Path)
		}
		if err := builder.add(extra.Path, kind, append([]byte(nil), extra.Content...), extra.PublicationPhase); err != nil {
			return ExclusiveInitPlan{}, err
		}
	}
	sort.Slice(builder.writes, func(i, j int) bool {
		if builder.writes[i].PublicationPhase != builder.writes[j].PublicationPhase {
			return builder.writes[i].PublicationPhase < builder.writes[j].PublicationPhase
		}
		return builder.writes[i].Path < builder.writes[j].Path
	})
	return ExclusiveInitPlan{
		SchemaVersion: 1, Command: "exclusive-init", CaseRoot: caseFull, RepoRoot: repoFull, Pack: pack,
		ProjectName: projectName, ProvisionID: provisionID, Role: role, CreatedAt: createdAt,
		Writes:         builder.writes,
		BlockedActions: []string{"existing root takeover", "overwrite", "backup", "force", "authority/confirmed writes", "heavy-tool execution"},
	}, nil
}

// ReserveExclusiveInitBatch validates every plan and exclusively reserves all
// missing roots before any planned leaf is created. Existing roots must already
// be exact or partial exact replays of their prepared plans.
func ReserveExclusiveInitBatch(plans ...ExclusiveInitPlan) (*ExclusiveInitBatch, error) {
	batch := &ExclusiveInitBatch{
		plans:     append([]ExclusiveInitPlan(nil), plans...),
		parents:   make([]*os.Root, len(plans)),
		roots:     make([]*os.Root, len(plans)),
		rootNames: make([]string, len(plans)),
		rootInfos: make([]os.FileInfo, len(plans)),
		created:   make([]bool, len(plans)),
	}
	seen := map[string]struct{}{}
	for i, plan := range batch.plans {
		if err := validateExclusiveInitPlan(plan); err != nil {
			batch.Rollback()
			return nil, err
		}
		key := strings.ToLower(filepath.Clean(plan.CaseRoot))
		if _, ok := seen[key]; ok {
			batch.Rollback()
			return nil, fmt.Errorf("exclusive init batch has duplicate root: %s", plan.CaseRoot)
		}
		seen[key] = struct{}{}

		parent, err := openExclusiveInitParent(filepath.Dir(plan.CaseRoot))
		if err != nil {
			batch.Rollback()
			return nil, err
		}
		batch.parents[i] = parent
		batch.rootNames[i] = filepath.Base(plan.CaseRoot)
		if exclusiveInitAfterParentPinHook != nil {
			if err := exclusiveInitAfterParentPinHook(); err != nil {
				batch.Rollback()
				return nil, err
			}
		}

		root, rootInfo, created, err := reserveExclusiveInitRoot(parent, batch.rootNames[i], plan.CaseRoot)
		if err != nil {
			batch.Rollback()
			return nil, err
		}
		batch.roots[i] = root
		batch.rootInfos[i] = rootInfo
		batch.created[i] = created
		if err := verifyExclusiveInitReplayAllowPartial(root, plan); err != nil {
			batch.Rollback()
			return nil, err
		}
	}
	return batch, nil
}

// Rollback removes only empty roots created by this reservation, using their
// pinned parent namespace, then closes all handles. Once apply has populated a
// root, recovery is intentionally driven by the durable caller intent.
func (batch *ExclusiveInitBatch) Rollback() {
	if batch == nil || batch.closed {
		return
	}
	for i := len(batch.roots) - 1; i >= 0; i-- {
		if batch.roots[i] == nil {
			continue
		}
		removable := false
		if batch.created[i] && batch.parents[i] != nil {
			leafInfo, leafErr := batch.parents[i].Lstat(batch.rootNames[i])
			rootInfo, rootErr := batch.roots[i].Stat(".")
			removable = leafErr == nil && rootErr == nil && os.SameFile(leafInfo, batch.rootInfos[i]) && os.SameFile(rootInfo, batch.rootInfos[i])
		}
		_ = batch.roots[i].Close()
		batch.roots[i] = nil
		if removable {
			_ = batch.parents[i].Remove(batch.rootNames[i])
		}
	}
	batch.created = nil
	_ = batch.closeRoots()
}

// Close releases every pinned root handle without removing reserved roots.
func (batch *ExclusiveInitBatch) Close() error {
	if batch == nil {
		return nil
	}
	return batch.closeRoots()
}

func (batch *ExclusiveInitBatch) closeRoots() error {
	if batch.closed {
		return nil
	}
	batch.closed = true
	var closeErr error
	for i := len(batch.roots) - 1; i >= 0; i-- {
		if batch.roots[i] != nil {
			closeErr = errors.Join(closeErr, batch.roots[i].Close())
		}
		if batch.parents[i] != nil {
			closeErr = errors.Join(closeErr, batch.parents[i].Close())
		}
	}
	batch.roots = nil
	batch.parents = nil
	return closeErr
}

// ValidateRoots verifies that every reserved root is still reachable through
// the pinned parent under its reserved name and still has the pinned identity.
// Callers use this around path-based validation while the batch handles remain
// open.
func (batch *ExclusiveInitBatch) ValidateRoots() error {
	if batch == nil {
		return fmt.Errorf("exclusive init batch is nil")
	}
	if batch.closed || len(batch.roots) != len(batch.plans) || len(batch.parents) != len(batch.plans) {
		return fmt.Errorf("exclusive init batch is closed")
	}
	for i, plan := range batch.plans {
		if batch.parents[i] == nil || batch.roots[i] == nil || batch.rootInfos[i] == nil {
			return fmt.Errorf("exclusive init batch root is not reserved: %s", plan.CaseRoot)
		}
		leafInfo, leafErr := batch.parents[i].Lstat(batch.rootNames[i])
		rootInfo, rootErr := batch.roots[i].Stat(".")
		if leafErr != nil || rootErr != nil || leafInfo.Mode()&os.ModeSymlink != 0 || !leafInfo.IsDir() || !os.SameFile(leafInfo, batch.rootInfos[i]) || !os.SameFile(rootInfo, batch.rootInfos[i]) {
			return fmt.Errorf("exclusive init reserved root identity changed: %s", plan.CaseRoot)
		}
	}
	return nil
}

// Apply populates or completes every reserved root. It intentionally retains
// all pinned handles; the caller owns their lifetime through Rollback or Close.
func (batch *ExclusiveInitBatch) Apply() ([]ExclusiveInitResult, error) {
	if err := batch.ValidateRoots(); err != nil {
		return nil, err
	}
	for i, plan := range batch.plans {
		if err := verifyExclusiveInitReplayAllowPartial(batch.roots[i], plan); err != nil {
			return nil, err
		}
	}
	if exclusiveInitAfterPreflightHook != nil {
		if err := exclusiveInitAfterPreflightHook(); err != nil {
			return nil, err
		}
	}
	if err := batch.ValidateRoots(); err != nil {
		return nil, err
	}
	results := make([]ExclusiveInitResult, 0, len(batch.plans))
	for i, plan := range batch.plans {
		result, err := applyExclusiveInitReserved(batch.roots[i], plan, batch.created[i])
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	for i := range batch.created {
		batch.created[i] = false
	}
	if err := batch.ValidateRoots(); err != nil {
		return nil, err
	}
	return results, nil
}

// ApplyExclusiveInit applies an exact plan. First apply requires the case root
// to be absent. Replays accept the complete or partial exact planned tree.
func ApplyExclusiveInit(plan ExclusiveInitPlan) (ExclusiveInitResult, error) {
	batch, err := ReserveExclusiveInitBatch(plan)
	if err != nil {
		return ExclusiveInitResult{}, err
	}
	defer batch.Rollback()
	results, err := batch.Apply()
	if err != nil {
		return ExclusiveInitResult{}, err
	}
	return results[0], nil
}

func applyExclusiveInitReserved(root *os.Root, plan ExclusiveInitPlan, firstApply bool) (ExclusiveInitResult, error) {
	if err := verifyExclusiveInitReplay(root, plan); err != nil {
		var replayErr *exclusiveInitReplayError
		if !errors.As(err, &replayErr) || !replayErr.partialExact {
			return ExclusiveInitResult{}, err
		}
		if err := completeExclusiveInitReplay(root, plan); err != nil {
			return ExclusiveInitResult{}, err
		}
		if err := verifyExclusiveInitReplay(root, plan); err != nil {
			return ExclusiveInitResult{}, err
		}
	}
	if err := reconcileCompletedExclusiveInitTemps(root, plan); err != nil {
		return ExclusiveInitResult{}, err
	}
	return exclusiveInitResult(plan, !firstApply), nil
}

func verifyExclusiveInitReplayAllowPartial(root *os.Root, plan ExclusiveInitPlan) error {
	err := verifyExclusiveInitReplay(root, plan)
	var replayErr *exclusiveInitReplayError
	if errors.As(err, &replayErr) && replayErr.partialExact {
		return nil
	}
	return err
}

type exclusiveInitBuilder struct {
	caseRoot     string
	paths        map[string]struct{}
	writes       []ExclusiveInitWrite
	defaultPhase int
}

func (b *exclusiveInitBuilder) add(rel, kind string, content []byte, phases ...int) error {
	phase := b.defaultPhase
	if len(phases) > 1 {
		return fmt.Errorf("exclusive init write has multiple publication phases: %s", rel)
	}
	if len(phases) == 1 {
		phase = phases[0]
	}
	rel, target, err := exclusiveInitPath(b.caseRoot, rel)
	if err != nil {
		return err
	}
	key := strings.ToLower(rel)
	if _, exists := b.paths[key]; exists {
		return fmt.Errorf("exclusive init duplicate write path: %s", rel)
	}
	b.paths[key] = struct{}{}
	sum := sha256.Sum256(content)
	b.writes = append(b.writes, ExclusiveInitWrite{Path: rel, Kind: kind, TargetPath: target, SHA256: hex.EncodeToString(sum[:]), Size: int64(len(content)), Content: append([]byte(nil), content...), PublicationPhase: phase})
	return nil
}

func validateExclusiveInitPlan(plan ExclusiveInitPlan) error {
	if plan.SchemaVersion != 1 || plan.Command != "exclusive-init" {
		return fmt.Errorf("invalid exclusive init plan identity")
	}
	caseFull, err := filepath.Abs(plan.CaseRoot)
	if err != nil || !casebind.SamePath(caseFull, plan.CaseRoot) {
		return fmt.Errorf("invalid exclusive init case root: %s", plan.CaseRoot)
	}
	if strings.TrimSpace(plan.ProjectName) == "" || strings.TrimSpace(plan.ProvisionID) == "" || strings.TrimSpace(plan.Role) == "" || strings.TrimSpace(plan.CreatedAt) == "" {
		return fmt.Errorf("exclusive init plan is missing stable identity fields")
	}
	if _, err := time.Parse(time.RFC3339Nano, plan.CreatedAt); err != nil {
		return fmt.Errorf("invalid exclusive init CreatedAt: %w", err)
	}
	seen := map[string]struct{}{}
	lastPhase := -1
	lastPath := ""
	for _, write := range plan.Writes {
		rel, target, err := exclusiveInitPath(plan.CaseRoot, write.Path)
		if err != nil {
			return err
		}
		if rel != write.Path || !casebind.SamePath(target, write.TargetPath) {
			return fmt.Errorf("exclusive init write target mismatch: %s", write.Path)
		}
		if write.PublicationPhase < 0 || write.PublicationPhase < lastPhase || (write.PublicationPhase == lastPhase && lastPath != "" && write.Path <= lastPath) {
			return fmt.Errorf("exclusive init writes are not ordered by publication phase and path: %s", write.Path)
		}
		lastPhase = write.PublicationPhase
		lastPath = write.Path
		key := strings.ToLower(rel)
		if _, ok := seen[key]; ok {
			return fmt.Errorf("exclusive init duplicate write path: %s", rel)
		}
		seen[key] = struct{}{}
		sum := sha256.Sum256(write.Content)
		if write.Size != int64(len(write.Content)) || write.SHA256 != hex.EncodeToString(sum[:]) {
			return fmt.Errorf("exclusive init write content binding mismatch: %s", rel)
		}
	}
	if len(plan.Writes) == 0 {
		return fmt.Errorf("exclusive init plan has no writes")
	}
	return nil
}

type exclusiveInitReplayError struct {
	message      string
	partialExact bool
}

func (e *exclusiveInitReplayError) Error() string { return e.message }

func openExclusiveInitParent(parentPath string) (*os.Root, error) {
	parentPath = filepath.Clean(parentPath)
	volume := filepath.VolumeName(parentPath)
	anchorPath := string(filepath.Separator)
	if volume != "" {
		anchorPath = volume + string(filepath.Separator)
	}
	anchorInfo, err := os.Lstat(anchorPath)
	if err != nil {
		return nil, err
	}
	if err := rejectExclusiveInitReparsePath(anchorPath); err != nil {
		return nil, err
	}
	if anchorInfo.Mode()&os.ModeSymlink != 0 || !anchorInfo.IsDir() {
		return nil, fmt.Errorf("exclusive init root parent is not a regular directory: %s", anchorPath)
	}
	current, err := os.OpenRoot(anchorPath)
	if err != nil {
		return nil, err
	}
	openedInfo, err := current.Stat(".")
	if err != nil || !os.SameFile(anchorInfo, openedInfo) {
		_ = current.Close()
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("exclusive init root parent changed while opening: %s", anchorPath)
	}

	rel, err := filepath.Rel(anchorPath, parentPath)
	if err != nil {
		_ = current.Close()
		return nil, err
	}
	if rel == "." {
		return current, nil
	}
	currentPath := anchorPath
	for component := range strings.SplitSeq(rel, string(filepath.Separator)) {
		info, err := current.Lstat(component)
		if err != nil {
			_ = current.Close()
			return nil, err
		}
		currentPath = filepath.Join(currentPath, component)
		if err := rejectExclusiveInitReparsePath(currentPath); err != nil {
			_ = current.Close()
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			_ = current.Close()
			return nil, fmt.Errorf("exclusive init root parent is not a regular directory: %s", currentPath)
		}
		next, err := current.OpenRoot(component)
		if err != nil {
			_ = current.Close()
			return nil, err
		}
		openedInfo, err := next.Stat(".")
		if err != nil || !os.SameFile(info, openedInfo) {
			_ = next.Close()
			_ = current.Close()
			if err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("exclusive init root parent changed while opening: %s", currentPath)
		}
		_ = current.Close()
		current = next
	}
	return current, nil
}

func reserveExclusiveInitRoot(parent *os.Root, name, caseRoot string) (*os.Root, os.FileInfo, bool, error) {
	info, err := parent.Lstat(name)
	created := false
	if os.IsNotExist(err) {
		if err := parent.Mkdir(name, 0o755); err != nil {
			return nil, nil, false, fmt.Errorf("exclusive init reserve root: %w", err)
		}
		created = true
		info, err = parent.Lstat(name)
	}
	if err != nil {
		if created {
			_ = parent.Remove(name)
		}
		return nil, nil, false, err
	}
	if err := rejectExclusiveInitReparsePath(caseRoot); err != nil {
		return nil, nil, false, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, nil, false, fmt.Errorf("exclusive init case root is not a regular directory: %s", caseRoot)
	}
	root, err := parent.OpenRoot(name)
	if err != nil {
		if created {
			_ = parent.Remove(name)
		}
		return nil, nil, false, err
	}
	openedInfo, err := root.Stat(".")
	if err != nil || !os.SameFile(info, openedInfo) {
		_ = root.Close()
		if created {
			_ = parent.Remove(name)
		}
		if err != nil {
			return nil, nil, false, err
		}
		return nil, nil, false, fmt.Errorf("exclusive init case root changed while opening: %s", caseRoot)
	}
	return root, openedInfo, created, nil
}

func verifyExclusiveInitReplay(root *os.Root, plan ExclusiveInitPlan) error {
	planned := make(map[string]ExclusiveInitWrite, len(plan.Writes))
	ownedTemps := make(map[string]ExclusiveInitWrite, len(plan.Writes))
	plannedDirs := map[string]struct{}{`.`: {}}
	for _, write := range plan.Writes {
		rel := path.Clean(write.Path)
		planned[rel] = write
		ownedTemps[path.Clean(filepath.ToSlash(exclusiveInitTempLeafName(filepath.FromSlash(write.Path), write)))] = write
		for parent := path.Dir(rel); parent != "."; parent = path.Dir(parent) {
			plannedDirs[parent] = struct{}{}
		}
	}
	seen := map[string]struct{}{}
	err := fs.WalkDir(root.FS(), ".", func(rel string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if rel == "." {
			return nil
		}
		info, err := root.Lstat(filepath.FromSlash(rel))
		if err != nil {
			return err
		}
		if err := rejectExclusiveInitReparsePath(filepath.Join(plan.CaseRoot, filepath.FromSlash(rel))); err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("exclusive init replay rejects symlink: %s", rel)
		}
		if info.IsDir() {
			if _, ok := plannedDirs[path.Clean(rel)]; !ok {
				return fmt.Errorf("exclusive init replay rejects unplanned directory: %s", rel)
			}
			return nil
		}
		write, ok := planned[path.Clean(rel)]
		if !ok {
			if _, owned := ownedTemps[path.Clean(rel)]; owned {
				if !info.Mode().IsRegular() {
					return fmt.Errorf("exclusive init owned temp is not a regular file: %s", rel)
				}
				return nil
			}
			return fmt.Errorf("exclusive init replay rejects unplanned object: %s", rel)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("exclusive init replay rejects non-regular object: %s", rel)
		}
		content, err := root.ReadFile(filepath.FromSlash(rel))
		if err != nil {
			return err
		}
		if !bytes.Equal(content, write.Content) {
			return fmt.Errorf("exclusive init replay rejects different bytes: %s", write.Path)
		}
		seen[path.Clean(rel)] = struct{}{}
		return nil
	})
	if err != nil {
		return err
	}
	firstMissing := ""
	for _, write := range plan.Writes {
		rel := path.Clean(write.Path)
		if _, ok := seen[rel]; !ok {
			if firstMissing == "" {
				firstMissing = rel
			}
			continue
		}
		if firstMissing != "" {
			return fmt.Errorf("exclusive init replay rejects non-prefix publication: planned leaf %s exists after missing predecessor %s", rel, firstMissing)
		}
	}
	if firstMissing != "" {
		return &exclusiveInitReplayError{message: fmt.Sprintf("exclusive init replay is partial exact; missing planned leaf: %s", firstMissing), partialExact: true}
	}
	return nil
}

func completeExclusiveInitReplay(root *os.Root, plan ExclusiveInitPlan) error {
	for _, write := range plan.Writes {
		name := filepath.FromSlash(write.Path)
		info, err := root.Lstat(name)
		if os.IsNotExist(err) {
			if err := createExclusiveInitLeaf(root, write); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if err := rejectExclusiveInitReparsePath(filepath.Join(plan.CaseRoot, name)); err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("exclusive init replay rejects non-regular planned leaf: %s", write.Path)
		}
		content, err := root.ReadFile(name)
		if err != nil {
			return err
		}
		if !bytes.Equal(content, write.Content) {
			return fmt.Errorf("exclusive init replay rejects different bytes: %s", write.Path)
		}
	}
	return nil
}

func createExclusiveInitLeaf(root *os.Root, write ExclusiveInitWrite) error {
	name := filepath.FromSlash(write.Path)
	if err := mkdirAllExclusiveCase(root, filepath.Dir(name)); err != nil {
		return err
	}
	tempName := exclusiveInitTempLeafName(name, write)
	if err := reconcileExclusiveInitTempLeaf(root, tempName, write); err != nil {
		return err
	}
	if _, err := root.Lstat(tempName); os.IsNotExist(err) {
		file, openErr := root.OpenFile(tempName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if openErr != nil {
			return fmt.Errorf("exclusive init create temp for %s: %w", write.Path, openErr)
		}
		writeErr := error(nil)
		if exclusiveInitLeafWriteHook != nil {
			writeErr = exclusiveInitLeafWriteHook("before-temp-write", write.Path)
		}
		if writeErr == nil {
			written, err := file.Write(write.Content)
			if err != nil {
				writeErr = err
			} else if written != len(write.Content) {
				writeErr = io.ErrShortWrite
			}
		}
		if writeErr == nil {
			writeErr = file.Sync()
		}
		if closeErr := file.Close(); writeErr == nil {
			writeErr = closeErr
		}
		if writeErr != nil {
			_ = root.Remove(tempName)
			return fmt.Errorf("exclusive init write %s: %w", write.Path, writeErr)
		}
	} else if err != nil {
		return err
	}
	if err := verifyExclusiveInitLeafBytes(root, tempName, write); err != nil {
		return err
	}
	if exclusiveInitLeafWriteHook != nil {
		if err := exclusiveInitLeafWriteHook("before-publish", write.Path); err != nil {
			return err
		}
	}
	if err := root.Link(tempName, name); err != nil {
		return fmt.Errorf("exclusive init publish %s without replacement: %w", write.Path, err)
	}
	if err := verifyExclusiveInitLeafBytes(root, name, write); err != nil {
		removeExclusiveInitPublishedLeaf(root, tempName, name)
		return err
	}
	if exclusiveInitLeafWriteHook != nil {
		if err := exclusiveInitLeafWriteHook("after-publish-before-temp-remove", write.Path); err != nil {
			return err
		}
	}
	if err := removeExclusiveInitTempForFinal(root, tempName, name, write); err != nil {
		return err
	}
	return nil
}

func exclusiveInitTempLeafName(name string, write ExclusiveInitWrite) string {
	base := filepath.Base(name)
	return filepath.Join(filepath.Dir(name), "."+base+".rekit-exclusive-init-"+write.SHA256[:16]+".tmp")
}

func reconcileExclusiveInitTempLeaf(root *os.Root, tempName string, write ExclusiveInitWrite) error {
	info, err := root.Lstat(tempName)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("exclusive init owned temp is not a regular file: %s", filepath.ToSlash(tempName))
	}
	if err := verifyExclusiveInitLeafBytes(root, tempName, write); err != nil {
		if removeErr := root.Remove(tempName); removeErr != nil {
			return fmt.Errorf("exclusive init remove incomplete owned temp %s: %w", filepath.ToSlash(tempName), removeErr)
		}
	}
	return nil
}

func reconcileCompletedExclusiveInitTemps(root *os.Root, plan ExclusiveInitPlan) error {
	for _, write := range plan.Writes {
		name := filepath.FromSlash(write.Path)
		tempName := exclusiveInitTempLeafName(name, write)
		if _, err := root.Lstat(tempName); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return err
		}
		if err := removeExclusiveInitTempForFinal(root, tempName, name, write); err != nil {
			return err
		}
	}
	return nil
}

func removeExclusiveInitTempForFinal(root *os.Root, tempName, name string, write ExclusiveInitWrite) error {
	if err := verifyExclusiveInitLeafBytes(root, tempName, write); err != nil {
		return fmt.Errorf("exclusive init cannot reconcile owned temp for %s: %w", write.Path, err)
	}
	if err := verifyExclusiveInitLeafBytes(root, name, write); err != nil {
		return err
	}
	tempInfo, tempErr := root.Lstat(tempName)
	finalInfo, finalErr := root.Lstat(name)
	if tempErr != nil || finalErr != nil || !os.SameFile(tempInfo, finalInfo) {
		return fmt.Errorf("exclusive init owned temp and final leaf have different identities: %s", write.Path)
	}
	if err := root.Remove(tempName); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("exclusive init remove temp for %s: %w", write.Path, err)
	}
	return nil
}

func verifyExclusiveInitLeafBytes(root *os.Root, name string, write ExclusiveInitWrite) error {
	before, err := root.Lstat(name)
	if err != nil {
		return err
	}
	if err := rejectExclusiveInitReparsePath(filepath.Join(root.Name(), name)); err != nil {
		return err
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return fmt.Errorf("exclusive init leaf is not regular: %s", filepath.ToSlash(name))
	}
	file, err := root.Open(name)
	if err != nil {
		return err
	}
	opened, statErr := file.Stat()
	content, readErr := io.ReadAll(io.LimitReader(file, write.Size+1))
	closeErr := file.Close()
	if statErr != nil {
		return statErr
	}
	if readErr != nil {
		return readErr
	}
	if closeErr != nil {
		return closeErr
	}
	after, err := root.Lstat(name)
	if err != nil || !os.SameFile(before, opened) || !os.SameFile(opened, after) || !bytes.Equal(content, write.Content) {
		return fmt.Errorf("exclusive init leaf has different bytes or identity: %s", write.Path)
	}
	return nil
}

func removeExclusiveInitPublishedLeaf(root *os.Root, tempName, name string) {
	tempInfo, tempErr := root.Lstat(tempName)
	finalInfo, finalErr := root.Lstat(name)
	if tempErr == nil && finalErr == nil && os.SameFile(tempInfo, finalInfo) {
		_ = root.Remove(name)
	}
}

func mkdirAllExclusiveCase(root *os.Root, dir string) error {
	if dir == "." {
		return nil
	}
	current := ""
	for part := range strings.SplitSeq(dir, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := root.Lstat(current)
		if os.IsNotExist(err) {
			if err := root.Mkdir(current, 0o755); err != nil && !os.IsExist(err) {
				return err
			}
			info, err = root.Lstat(current)
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("exclusive init parent is not a regular directory: %s", filepath.ToSlash(current))
		}
	}
	return nil
}

func exclusiveInitPath(caseRoot, rel string) (string, string, error) {
	rel = filepath.ToSlash(filepath.Clean(filepath.FromSlash(strings.TrimSpace(rel))))
	if rel == "" || rel == "." || filepath.IsAbs(filepath.FromSlash(rel)) || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", "", fmt.Errorf("invalid exclusive init relative path: %q", rel)
	}
	target, err := refsf.SafeJoin(caseRoot, rel)
	if err != nil {
		return "", "", err
	}
	return rel, target, nil
}

func exclusiveInitResult(plan ExclusiveInitPlan, replay bool) ExclusiveInitResult {
	return ExclusiveInitResult{
		SchemaVersion: 1, Command: plan.Command, CaseRoot: plan.CaseRoot, RepoRoot: plan.RepoRoot, Pack: plan.Pack,
		ProjectName: plan.ProjectName, ProvisionID: plan.ProvisionID, Role: plan.Role, CreatedAt: plan.CreatedAt,
		Applied: true, Replay: replay, Writes: plan.Writes,
	}
}
