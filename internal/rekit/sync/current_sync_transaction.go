package sync

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtimebundle"
	"github.com/shuiyu486/re-context-kits/internal/rekit/statemigration"
)

const currentSyncMaxTransactionBytes = int64(32 << 20)

type currentSyncOwner struct {
	SchemaVersion     int               `json:"schemaVersion"`
	Kind              string            `json:"kind"`
	PlanSHA256        string            `json:"planSha256"`
	TransactionSHA256 string            `json:"transactionSha256"`
	TransactionPath   string            `json:"transactionPath"`
	Intent            currentSyncIntent `json:"intent"`
}

type currentSyncIntentRoot struct {
	Name         string               `json:"name"`
	Before       CurrentSyncInventory `json:"before"`
	After        CurrentSyncInventory `json:"after"`
	StagePath    string               `json:"stagePath"`
	PreviousPath string               `json:"previousPath"`
	Mutate       bool                 `json:"mutate"`
}

type currentSyncIntentLeaf struct {
	Path         string `json:"path"`
	Kind         string `json:"kind"`
	StagePath    string `json:"stagePath"`
	PreviousPath string `json:"previousPath"`
	BeforeExists bool   `json:"beforeExists"`
	BeforeSHA256 string `json:"beforeSha256,omitempty"`
	BeforeSize   int64  `json:"beforeSize,omitempty"`
	AfterExists  bool   `json:"afterExists"`
	AfterSHA256  string `json:"afterSha256,omitempty"`
	AfterSize    int64  `json:"afterSize,omitempty"`
	Mode         uint32 `json:"mode"`
	Mutate       bool   `json:"mutate"`
	ActivateLast bool   `json:"activateLast,omitempty"`
}

type currentSyncIntent struct {
	SchemaVersion     int                     `json:"schemaVersion"`
	Kind              string                  `json:"kind"`
	PlanSHA256        string                  `json:"planSha256"`
	TransactionSHA256 string                  `json:"transactionSha256"`
	TransactionPath   string                  `json:"transactionPath"`
	CaseRootIdentity  statemigration.Identity `json:"caseRootIdentity"`
	StateRootIdentity statemigration.Identity `json:"stateRootIdentity"`
	Plan              CurrentSyncPlan         `json:"plan"`
	Roots             []currentSyncIntentRoot `json:"roots"`
	Leaves            []currentSyncIntentLeaf `json:"leaves"`
	NoAuthority       bool                    `json:"noAuthority"`
	NoConfirmed       bool                    `json:"noConfirmed"`
	NoHeavyTool       bool                    `json:"noHeavyTool"`
	NoSyncPromote     bool                    `json:"noSyncPromote"`
}

type currentSyncTransactionIdentity struct {
	SchemaVersion     int                     `json:"schemaVersion"`
	Kind              string                  `json:"kind"`
	PlanSHA256        string                  `json:"planSha256"`
	TransactionPath   string                  `json:"transactionPath"`
	CaseRootIdentity  statemigration.Identity `json:"caseRootIdentity"`
	StateRootIdentity statemigration.Identity `json:"stateRootIdentity"`
	Plan              CurrentSyncPlan         `json:"plan"`
	Roots             []currentSyncIntentRoot `json:"roots"`
	Leaves            []currentSyncIntentLeaf `json:"leaves"`
	NoAuthority       bool                    `json:"noAuthority"`
	NoConfirmed       bool                    `json:"noConfirmed"`
	NoHeavyTool       bool                    `json:"noHeavyTool"`
	NoSyncPromote     bool                    `json:"noSyncPromote"`
}

type currentSyncProgressOperation struct {
	Sequence int    `json:"sequence"`
	Kind     string `json:"kind"`
	Target   string `json:"target,omitempty"`
}

type currentSyncProgress struct {
	SchemaVersion     int                            `json:"schemaVersion"`
	Kind              string                         `json:"kind"`
	TransactionSHA256 string                         `json:"transactionSha256"`
	Generation        int                            `json:"generation"`
	Phase             string                         `json:"phase"`
	Completed         []currentSyncProgressOperation `json:"completed,omitempty"`
	Pending           *currentSyncProgressOperation  `json:"pending,omitempty"`
	ProgressSHA256    string                         `json:"progressSha256"`
}

type currentSyncProgressIdentity struct {
	SchemaVersion     int                            `json:"schemaVersion"`
	Kind              string                         `json:"kind"`
	TransactionSHA256 string                         `json:"transactionSha256"`
	Generation        int                            `json:"generation"`
	Phase             string                         `json:"phase"`
	Completed         []currentSyncProgressOperation `json:"completed,omitempty"`
	Pending           *currentSyncProgressOperation  `json:"pending,omitempty"`
}

type currentSyncReceipt struct {
	SchemaVersion     int                  `json:"schemaVersion"`
	Kind              string               `json:"kind"`
	Command           string               `json:"command"`
	State             string               `json:"state"`
	PlanSHA256        string               `json:"planSha256"`
	TransactionSHA256 string               `json:"transactionSha256"`
	TransactionPath   string               `json:"transactionPath"`
	ProgressSHA256    string               `json:"progressSha256"`
	Pack              string               `json:"pack"`
	ProjectName       string               `json:"projectName"`
	Controlled        CurrentSyncInventory `json:"controlled"`
	Targets           CurrentSyncInventory `json:"targets"`
	Manifest          CurrentSyncBinding   `json:"manifest"`
	RuntimeExecutable CurrentSyncBinding   `json:"runtimeExecutable"`
	NoAuthority       bool                 `json:"noAuthority"`
	NoConfirmed       bool                 `json:"noConfirmed"`
	NoHeavyTool       bool                 `json:"noHeavyTool"`
	NoSyncPromote     bool                 `json:"noSyncPromote"`
	ReceiptSHA256     string               `json:"receiptSha256"`
}

type currentSyncReceiptIdentity struct {
	SchemaVersion     int                  `json:"schemaVersion"`
	Kind              string               `json:"kind"`
	Command           string               `json:"command"`
	State             string               `json:"state"`
	PlanSHA256        string               `json:"planSha256"`
	TransactionSHA256 string               `json:"transactionSha256"`
	TransactionPath   string               `json:"transactionPath"`
	ProgressSHA256    string               `json:"progressSha256"`
	Pack              string               `json:"pack"`
	ProjectName       string               `json:"projectName"`
	Controlled        CurrentSyncInventory `json:"controlled"`
	Targets           CurrentSyncInventory `json:"targets"`
	Manifest          CurrentSyncBinding   `json:"manifest"`
	RuntimeExecutable CurrentSyncBinding   `json:"runtimeExecutable"`
	NoAuthority       bool                 `json:"noAuthority"`
	NoConfirmed       bool                 `json:"noConfirmed"`
	NoHeavyTool       bool                 `json:"noHeavyTool"`
	NoSyncPromote     bool                 `json:"noSyncPromote"`
}

// CurrentSyncResult is the durable public result of an exact current project
// refresh or a strict lost-response replay.
type CurrentSyncResult struct {
	SchemaVersion     int                 `json:"schemaVersion"`
	Kind              string              `json:"kind"`
	Command           string              `json:"command"`
	Status            string              `json:"status"`
	CaseRoot          string              `json:"caseRoot"`
	Pack              string              `json:"pack"`
	IsMutation        bool                `json:"isMutation"`
	Applied           bool                `json:"applied"`
	Replay            bool                `json:"replay"`
	AlreadyCurrent    bool                `json:"alreadyCurrent"`
	PlanSHA256        string              `json:"planSha256"`
	TransactionSHA256 string              `json:"transactionSha256"`
	ReceiptPath       string              `json:"receiptPath"`
	Receipt           *currentSyncReceipt `json:"receipt"`
}

type currentSyncResult = CurrentSyncResult

func buildCurrentSyncIntent(plan CurrentSyncPlan) (currentSyncIntent, error) {
	if err := validateCurrentSyncPlanForIntent(plan); err != nil {
		return currentSyncIntent{}, err
	}
	plan.prepared = nil
	intent := currentSyncIntent{
		SchemaVersion:     currentSyncSchemaVersion,
		Kind:              currentSyncIntentKind,
		PlanSHA256:        plan.ExpectedPlanSHA256,
		CaseRootIdentity:  plan.CaseRootIdentity,
		StateRootIdentity: plan.StateRootIdentity,
		Plan:              plan,
		NoAuthority:       true,
		NoConfirmed:       true,
		NoHeavyTool:       true,
		NoSyncPromote:     true,
	}
	intent.TransactionPath = currentSyncTransactionPath(intent.PlanSHA256)
	var err error
	intent.Roots, err = currentSyncIntentRoots(plan, intent.TransactionPath)
	if err != nil {
		return currentSyncIntent{}, err
	}
	intent.Leaves, err = currentSyncIntentLeaves(plan, intent.TransactionPath)
	if err != nil {
		return currentSyncIntent{}, err
	}
	intent.TransactionSHA256, err = currentSyncCanonicalSHA(currentSyncTransactionIdentityFor(intent))
	if err != nil {
		return currentSyncIntent{}, err
	}
	stateRoot := filepath.Join(plan.CaseRoot, projectstate.CurrentDir)
	if err := validateCurrentSyncIntent(intent, plan.CaseRoot, stateRoot); err != nil {
		return currentSyncIntent{}, err
	}
	return intent, nil
}

func currentSyncTransactionPath(planSHA256 string) string {
	return currentSyncTransactionsRel + "/" + strings.ToLower(planSHA256)
}

func currentSyncArchivedIntentPath(intent currentSyncIntent) string {
	return intent.TransactionPath + "/" + currentSyncArchivedIntent
}

func currentSyncProgressDir(intent currentSyncIntent) string {
	return intent.TransactionPath + "/" + currentSyncProgressDirName
}

func currentSyncProgressPath(intent currentSyncIntent, generation int) string {
	return fmt.Sprintf("%s/%08d.json", currentSyncProgressDir(intent), generation)
}

func newCurrentSyncProgress(intent currentSyncIntent) (currentSyncProgress, error) {
	if err := validateCurrentSyncIntentStructure(intent); err != nil {
		return currentSyncProgress{}, err
	}
	progress := currentSyncProgress{
		SchemaVersion:     currentSyncSchemaVersion,
		Kind:              currentSyncProgressKind,
		TransactionSHA256: intent.TransactionSHA256,
		Generation:        1,
		Phase:             "prepared",
	}
	var err error
	progress.ProgressSHA256, err = currentSyncCanonicalSHA(currentSyncProgressIdentityFor(progress))
	if err != nil {
		return currentSyncProgress{}, err
	}
	return progress, nil
}

func currentSyncProgressIdentityFor(progress currentSyncProgress) currentSyncProgressIdentity {
	return currentSyncProgressIdentity{
		SchemaVersion:     progress.SchemaVersion,
		Kind:              progress.Kind,
		TransactionSHA256: progress.TransactionSHA256,
		Generation:        progress.Generation,
		Phase:             progress.Phase,
		Completed:         progress.Completed,
		Pending:           progress.Pending,
	}
}

func currentSyncRootIdentity(path string) (statemigration.Identity, error) {
	root, identity, err := statemigration.OpenRootIdentity(path)
	if err != nil {
		return statemigration.Identity{}, err
	}
	if err := root.Close(); err != nil {
		return statemigration.Identity{}, err
	}
	return identity, nil
}

func currentSyncValidateRootIdentity(path string, expected statemigration.Identity, label string) error {
	current, err := currentSyncRootIdentity(path)
	if err != nil {
		return fmt.Errorf("current sync durable intent %s physical identity is unavailable: %w", label, err)
	}
	if current != expected {
		return fmt.Errorf("current sync durable intent %s physical identity changed", label)
	}
	return nil
}

func buildCurrentSyncReceipt(intent currentSyncIntent, progress currentSyncProgress) (currentSyncReceipt, error) {
	if err := validateCurrentSyncIntentStructure(intent); err != nil {
		return currentSyncReceipt{}, err
	}
	if err := validateCurrentSyncProgress(progress, intent); err != nil {
		return currentSyncReceipt{}, err
	}
	if !currentSyncReceiptCommitPending(progress) {
		return currentSyncReceipt{}, fmt.Errorf("current sync receipt requires pending receipt commit progress")
	}
	runtimeExecutable, ok := currentSyncRuntimeExecutableBinding(intent.Plan.NextControlled)
	if !ok {
		return currentSyncReceipt{}, fmt.Errorf("current sync receipt runtime executable binding is unavailable")
	}
	receipt := currentSyncReceipt{
		SchemaVersion:     currentSyncSchemaVersion,
		Kind:              currentSyncReceiptKind,
		Command:           intent.Plan.Command,
		State:             "committed",
		PlanSHA256:        intent.PlanSHA256,
		TransactionSHA256: intent.TransactionSHA256,
		TransactionPath:   intent.TransactionPath,
		ProgressSHA256:    progress.ProgressSHA256,
		Pack:              intent.Plan.Pack,
		ProjectName:       intent.Plan.ProjectName,
		Controlled:        intent.Plan.NextControlled,
		Targets:           intent.Plan.NextTargets,
		Manifest:          intent.Plan.NextManifest,
		RuntimeExecutable: runtimeExecutable,
		NoAuthority:       true,
		NoConfirmed:       true,
		NoHeavyTool:       true,
		NoSyncPromote:     true,
	}
	var err error
	receipt.ReceiptSHA256, err = currentSyncCanonicalSHA(currentSyncReceiptIdentityFor(receipt))
	if err != nil {
		return currentSyncReceipt{}, err
	}
	if err := validateCurrentSyncReceipt(receipt, intent, progress); err != nil {
		return currentSyncReceipt{}, err
	}
	return receipt, nil
}

func currentSyncReceiptCommitPending(progress currentSyncProgress) bool {
	return progress.Phase == "activated" && progress.Pending != nil && progress.Pending.Kind == "receipt-stage-to-live"
}

func currentSyncRuntimeExecutableBinding(inventory CurrentSyncInventory) (CurrentSyncBinding, bool) {
	manifestPrefix := projectstate.CurrentDir + "/runtime/"
	for _, binding := range inventory.Entries {
		if binding.Kind == "runtime-executable" && strings.HasPrefix(binding.Path, manifestPrefix) {
			return binding, true
		}
	}
	return CurrentSyncBinding{}, false
}

func currentSyncReceiptIdentityFor(receipt currentSyncReceipt) currentSyncReceiptIdentity {
	return currentSyncReceiptIdentity{
		SchemaVersion:     receipt.SchemaVersion,
		Kind:              receipt.Kind,
		Command:           receipt.Command,
		State:             receipt.State,
		PlanSHA256:        receipt.PlanSHA256,
		TransactionSHA256: receipt.TransactionSHA256,
		TransactionPath:   receipt.TransactionPath,
		ProgressSHA256:    receipt.ProgressSHA256,
		Pack:              receipt.Pack,
		ProjectName:       receipt.ProjectName,
		Controlled:        receipt.Controlled,
		Targets:           receipt.Targets,
		Manifest:          receipt.Manifest,
		RuntimeExecutable: receipt.RuntimeExecutable,
		NoAuthority:       receipt.NoAuthority,
		NoConfirmed:       receipt.NoConfirmed,
		NoHeavyTool:       receipt.NoHeavyTool,
		NoSyncPromote:     receipt.NoSyncPromote,
	}
}

func validateCurrentSyncReceipt(receipt currentSyncReceipt, intent currentSyncIntent, progress currentSyncProgress) error {
	if err := validateCurrentSyncIntentStructure(intent); err != nil {
		return err
	}
	if err := validateCurrentSyncProgress(progress, intent); err != nil {
		return err
	}
	expectedExecutable, executableOK := currentSyncRuntimeExecutableBinding(intent.Plan.NextControlled)
	if receipt.SchemaVersion != currentSyncSchemaVersion || receipt.Kind != currentSyncReceiptKind ||
		receipt.Command != intent.Plan.Command || receipt.State != "committed" ||
		!strings.EqualFold(receipt.PlanSHA256, intent.PlanSHA256) ||
		!strings.EqualFold(receipt.TransactionSHA256, intent.TransactionSHA256) || receipt.TransactionPath != intent.TransactionPath ||
		!strings.EqualFold(receipt.ProgressSHA256, progress.ProgressSHA256) ||
		receipt.Pack != intent.Plan.Pack || receipt.ProjectName != intent.Plan.ProjectName ||
		!currentSyncReceiptCommitPending(progress) || !executableOK ||
		!currentSyncCanonicalEqual(receipt.Controlled, intent.Plan.NextControlled) ||
		!currentSyncCanonicalEqual(receipt.Targets, intent.Plan.NextTargets) ||
		!currentSyncCanonicalEqual(receipt.Manifest, intent.Plan.NextManifest) ||
		!currentSyncCanonicalEqual(receipt.RuntimeExecutable, expectedExecutable) ||
		!receipt.NoAuthority || !receipt.NoConfirmed || !receipt.NoHeavyTool || !receipt.NoSyncPromote {
		return fmt.Errorf("current sync receipt identity is invalid")
	}
	if err := validateCurrentSyncInventory(receipt.Controlled); err != nil {
		return err
	}
	if err := validateCurrentSyncInventory(receipt.Targets); err != nil {
		return err
	}
	if err := validateCurrentSyncBinding(receipt.Manifest, "runtime-bundle-manifest"); err != nil {
		return err
	}
	if err := validateCurrentSyncBinding(receipt.RuntimeExecutable, "runtime-executable"); err != nil {
		return err
	}
	receiptSHA, err := currentSyncCanonicalSHA(currentSyncReceiptIdentityFor(receipt))
	if err != nil || !validCurrentSyncSHA(receipt.ReceiptSHA256) || !strings.EqualFold(receiptSHA, receipt.ReceiptSHA256) {
		return fmt.Errorf("current sync receipt binding is invalid")
	}
	return nil
}

func readCurrentSyncReceipt(stateRoot string) (currentSyncReceipt, bool, error) {
	path := filepath.Join(stateRoot, filepath.FromSlash(currentSyncReceiptRel))
	data, exists, err := currentSyncReadOptional(stateRoot, path, "current sync durable receipt", currentSyncMaxTransactionBytes)
	if err != nil || !exists {
		return currentSyncReceipt{}, exists, err
	}
	var receipt currentSyncReceipt
	if err := decodeCurrentSyncCanonical(data, &receipt, "durable receipt"); err != nil {
		return currentSyncReceipt{}, false, err
	}
	return receipt, true, nil
}

func validateCurrentSyncReplayState(caseRoot, stateRoot string, receipt currentSyncReceipt) (currentSyncIntent, currentSyncProgress, error) {
	intent, err := readCurrentSyncArchivedIntent(stateRoot, receipt.TransactionPath)
	if err != nil {
		return currentSyncIntent{}, currentSyncProgress{}, err
	}
	if active, exists, err := readCurrentSyncIntent(caseRoot, stateRoot); err != nil {
		return currentSyncIntent{}, currentSyncProgress{}, err
	} else if exists && !currentSyncCanonicalEqual(active, intent) {
		return currentSyncIntent{}, currentSyncProgress{}, fmt.Errorf("current sync replay active intent differs from archived transaction")
	}
	if err := validateCurrentSyncReplayRoots(caseRoot, stateRoot, intent); err != nil {
		return currentSyncIntent{}, currentSyncProgress{}, err
	}
	history, err := readCurrentSyncProgressHistory(stateRoot, intent)
	if err != nil {
		return currentSyncIntent{}, currentSyncProgress{}, err
	}
	receiptIndex := -1
	for index := range history {
		if !strings.EqualFold(history[index].ProgressSHA256, receipt.ProgressSHA256) {
			continue
		}
		if receiptIndex >= 0 {
			return currentSyncIntent{}, currentSyncProgress{}, fmt.Errorf("current sync replay receipt progress binding is ambiguous")
		}
		receiptIndex = index
	}
	if receiptIndex < 0 || receiptIndex+4 != len(history) {
		return currentSyncIntent{}, currentSyncProgress{}, fmt.Errorf("current sync replay receipt commit lineage is incomplete")
	}
	receiptPending := history[receiptIndex]
	receiptPublished := history[receiptIndex+1]
	commitPending := history[receiptIndex+2]
	committed := history[receiptIndex+3]
	if !currentSyncReceiptCommitPending(receiptPending) ||
		receiptPublished.Pending != nil || receiptPublished.Phase != "activated" ||
		commitPending.Pending == nil || commitPending.Pending.Kind != "receipt-committed" || commitPending.Phase != "activated" ||
		committed.Pending != nil || committed.Phase != "receipt-committed" {
		return currentSyncIntent{}, currentSyncProgress{}, fmt.Errorf("current sync replay receipt commit lineage is invalid")
	}
	for _, transition := range [][2]currentSyncProgress{
		{receiptPending, receiptPublished},
		{receiptPublished, commitPending},
		{commitPending, committed},
	} {
		if err := validateCurrentSyncProgressTransition(transition[0], transition[1], intent); err != nil {
			return currentSyncIntent{}, currentSyncProgress{}, err
		}
	}
	if err := validateCurrentSyncReceipt(receipt, intent, receiptPending); err != nil {
		return currentSyncIntent{}, currentSyncProgress{}, err
	}
	if err := validateCurrentSyncLiveReceiptState(caseRoot, stateRoot, intent, receipt); err != nil {
		return currentSyncIntent{}, currentSyncProgress{}, err
	}
	return intent, committed, nil
}

func validateCurrentSyncReplayRoots(caseRoot, stateRoot string, intent currentSyncIntent) error {
	caseFull, err := filepath.Abs(caseRoot)
	if err != nil || !sameCurrentSyncPath(caseFull, intent.Plan.CaseRoot) {
		return fmt.Errorf("current sync replay case root is invalid")
	}
	stateFull, err := filepath.Abs(stateRoot)
	if err != nil || !sameCurrentSyncPath(stateFull, filepath.Join(caseFull, projectstate.CurrentDir)) {
		return fmt.Errorf("current sync replay state root is invalid")
	}
	if intent.CaseRootIdentity != intent.Plan.CaseRootIdentity || intent.StateRootIdentity != intent.Plan.StateRootIdentity {
		return fmt.Errorf("current sync replay filesystem identity binding is invalid")
	}
	if err := intent.CaseRootIdentity.Validate(); err != nil {
		return err
	}
	if err := intent.StateRootIdentity.Validate(); err != nil {
		return err
	}
	if err := currentSyncValidateRootIdentity(caseFull, intent.CaseRootIdentity, "replay case root"); err != nil {
		return err
	}
	return currentSyncValidateRootIdentity(stateFull, intent.StateRootIdentity, "replay state root")
}

func validateCurrentSyncLiveReceiptState(caseRoot, stateRoot string, intent currentSyncIntent, receipt currentSyncReceipt) error {
	if err := validateCurrentSyncActiveInstance(caseRoot, stateRoot, receipt); err != nil {
		return err
	}
	manifestPath := filepath.ToSlash(filepath.Join(projectstate.CurrentDir, runtimebundle.ManifestRel))
	if receipt.Manifest.Path != manifestPath {
		return fmt.Errorf("current sync replay manifest path differs from durable receipt")
	}
	manifestData, err := rekitfs.ReadStableRegularFileAnchored(stateRoot, filepath.Join(stateRoot, filepath.FromSlash(runtimebundle.ManifestRel)), "current sync replay manifest", 1<<20)
	if err != nil {
		return err
	}
	if currentSyncSHA(manifestData) != strings.ToLower(receipt.Manifest.SHA256) || int64(len(manifestData)) != receipt.Manifest.Size {
		return fmt.Errorf("current sync replay manifest differs from durable receipt")
	}
	manifest, err := runtimebundle.Validate(stateRoot, runtimebundle.ManifestRel, receipt.Manifest.SHA256, receipt.Pack)
	if err != nil {
		return fmt.Errorf("current sync replay bundle validation failed: %w", err)
	}
	executablePath := runtimebundle.ExecutablePath(stateRoot, manifest)
	executableData, err := rekitfs.ReadStableRegularFileAnchored(stateRoot, executablePath, "current sync replay runtime executable", currentSyncMaxFileBytes)
	if err != nil {
		return err
	}
	if filepath.ToSlash(receipt.RuntimeExecutable.Path) != filepath.ToSlash(filepath.Join(projectstate.CurrentDir, manifest.Executable.Path)) ||
		currentSyncSHA(executableData) != strings.ToLower(receipt.RuntimeExecutable.SHA256) || int64(len(executableData)) != receipt.RuntimeExecutable.Size {
		return fmt.Errorf("current sync replay runtime executable differs from durable receipt")
	}
	controlled, err := currentSyncLiveControlledInventory(stateRoot, receipt.Controlled)
	if err != nil {
		return err
	}
	if !currentSyncCanonicalEqual(controlled, receipt.Controlled) {
		return fmt.Errorf("current sync replay controlled inventory differs from durable receipt")
	}
	targets, err := currentSyncLiveTargetInventory(caseRoot, intent, receipt.Targets)
	if err != nil {
		return err
	}
	if !currentSyncCanonicalEqual(targets, receipt.Targets) {
		return fmt.Errorf("current sync replay target inventory differs from durable receipt")
	}
	return nil
}

func validateCurrentSyncActiveInstance(caseRoot, stateRoot string, receipt currentSyncReceipt) error {
	inst, err := instance.Read(caseRoot)
	if err != nil {
		return fmt.Errorf("current sync replay active instance is invalid: %w", err)
	}
	if inst.Source != "steamai" || inst.StateDir != projectstate.CurrentDir || inst.SchemaVersion != 2 || inst.Mode != "project-local-bundle" ||
		inst.TemplatePack != receipt.Pack || inst.ProjectName != receipt.ProjectName || inst.BundleManifest != runtimebundle.ManifestRel ||
		!strings.EqualFold(inst.BundleManifestSHA256, receipt.Manifest.SHA256) ||
		!sameCurrentSyncPath(inst.InstancePath, filepath.Join(stateRoot, "instance.yml")) ||
		!sameCurrentSyncPath(inst.TemplateRoot, stateRoot) ||
		!sameCurrentSyncPath(inst.BundleRoot, filepath.Join(stateRoot, "runtime")) ||
		!sameCurrentSyncPath(inst.ProjectRoot, caseRoot) {
		return fmt.Errorf("current sync replay active instance differs from durable receipt")
	}
	return nil
}

func currentSyncLiveControlledInventory(stateRoot string, expected CurrentSyncInventory) (CurrentSyncInventory, error) {
	if err := validateCurrentSyncInventory(expected); err != nil {
		return CurrentSyncInventory{}, err
	}
	expectedByPath := make(map[string]CurrentSyncBinding, len(expected.Entries))
	for _, binding := range expected.Entries {
		if _, ok := currentSyncControlledBindingRoot(binding.Path); !ok {
			return CurrentSyncInventory{}, fmt.Errorf("current sync replay controlled path is invalid: %s", binding.Path)
		}
		expectedByPath[strings.ToLower(binding.Path)] = binding
	}
	entries := make([]CurrentSyncBinding, 0, len(expected.Entries))
	seen := map[string]bool{}
	for _, root := range currentSyncControlledRoots {
		paths, err := rekitfs.WalkRegularFilesAnchored(stateRoot, root, "current sync replay controlled tree", currentSyncMaxFiles)
		if err != nil {
			return CurrentSyncInventory{}, err
		}
		for _, path := range paths {
			if len(entries) >= currentSyncMaxFiles {
				return CurrentSyncInventory{}, fmt.Errorf("current sync replay controlled inventory exceeds %d files", currentSyncMaxFiles)
			}
			rel, err := filepath.Rel(stateRoot, path)
			if err != nil {
				return CurrentSyncInventory{}, err
			}
			bindingPath := filepath.ToSlash(filepath.Join(projectstate.CurrentDir, rel))
			key := strings.ToLower(bindingPath)
			expectedBinding, ok := expectedByPath[key]
			if !ok || expectedBinding.Path != bindingPath || seen[key] {
				return CurrentSyncInventory{}, fmt.Errorf("current sync replay controlled tree contains an unplanned file: %s", bindingPath)
			}
			binding, err := currentSyncLiveBinding(stateRoot, path, expectedBinding, "current sync replay controlled file")
			if err != nil {
				return CurrentSyncInventory{}, err
			}
			entries = append(entries, binding)
			seen[key] = true
		}
	}
	if len(seen) != len(expectedByPath) {
		return CurrentSyncInventory{}, fmt.Errorf("current sync replay controlled tree is missing a planned file")
	}
	return currentSyncInventory(entries)
}

func currentSyncLiveTargetInventory(caseRoot string, intent currentSyncIntent, expected CurrentSyncInventory) (CurrentSyncInventory, error) {
	if err := validateCurrentSyncInventory(expected); err != nil {
		return CurrentSyncInventory{}, err
	}
	for _, leaf := range intent.Leaves {
		if leaf.AfterExists {
			continue
		}
		if !currentSyncSafeTargetRel(leaf.Path) {
			return CurrentSyncInventory{}, fmt.Errorf("current sync replay deleted target path is invalid: %s", leaf.Path)
		}
		path := filepath.Join(caseRoot, filepath.FromSlash(leaf.Path))
		if _, err := os.Lstat(path); err == nil {
			return CurrentSyncInventory{}, fmt.Errorf("current sync replay deleted target still exists: %s", leaf.Path)
		} else if !os.IsNotExist(err) {
			return CurrentSyncInventory{}, err
		}
		if err := rekitfs.ValidateNoReparseComponents(path); err != nil {
			return CurrentSyncInventory{}, err
		}
	}
	entries := make([]CurrentSyncBinding, 0, len(expected.Entries))
	for _, binding := range expected.Entries {
		if !currentSyncSafeTargetRel(binding.Path) {
			return CurrentSyncInventory{}, fmt.Errorf("current sync replay target path is invalid: %s", binding.Path)
		}
		entry, err := currentSyncLiveBinding(caseRoot, filepath.Join(caseRoot, filepath.FromSlash(binding.Path)), binding, "current sync replay target file")
		if err != nil {
			return CurrentSyncInventory{}, err
		}
		entries = append(entries, entry)
	}
	return currentSyncInventory(entries)
}

func currentSyncLiveBinding(anchor, path string, expected CurrentSyncBinding, label string) (CurrentSyncBinding, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return CurrentSyncBinding{}, err
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return CurrentSyncBinding{}, fmt.Errorf("%s must be a regular non-symlink file: %s", label, path)
	}
	data, err := currentSyncReadFile(anchor, path, label, currentSyncMaxFileBytes, true)
	if err != nil {
		return CurrentSyncBinding{}, err
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(before, after) {
		return CurrentSyncBinding{}, fmt.Errorf("%s changed while validating: %s", label, path)
	}
	if !currentSyncModeMatches(os.FileMode(expected.Mode), after.Mode()) {
		return CurrentSyncBinding{}, fmt.Errorf("%s mode differs from durable receipt: %s", label, path)
	}
	return currentSyncBinding(expected.Path, expected.Kind, data, os.FileMode(expected.Mode)), nil
}

func currentSyncModeMatches(expected, actual os.FileMode) bool {
	if runtime.GOOS == "windows" {
		return expected.Perm()&0o200 == actual.Perm()&0o200
	}
	return expected.Perm() == actual.Perm()
}

func currentSyncReplayResult(caseRoot, expectedPlanSHA256 string) (_ currentSyncResult, exists bool, retErr error) {
	caseFull, stateRoot, err := currentSyncRoots(caseRoot)
	if err != nil {
		return currentSyncResult{}, false, err
	}
	lease, err := acquireCurrentSyncRefreshLease(caseFull)
	if err != nil {
		return currentSyncResult{}, false, err
	}
	defer func() { retErr = errors.Join(retErr, lease.Unlock()) }()
	if err := lease.Validate(); err != nil {
		return currentSyncResult{}, false, err
	}
	receipt, exists, err := readCurrentSyncReceipt(stateRoot.Path)
	if err != nil || !exists {
		return currentSyncResult{}, exists, err
	}
	if !validCurrentSyncSHA(expectedPlanSHA256) || !strings.EqualFold(receipt.PlanSHA256, expectedPlanSHA256) {
		return currentSyncResult{}, true, fmt.Errorf("current sync durable receipt does not match the expected reviewed plan")
	}
	if _, _, err := validateCurrentSyncReplayState(caseFull, stateRoot.Path, receipt); err != nil {
		return currentSyncResult{}, true, err
	}
	if active, activeExists, err := readCurrentSyncIntent(caseFull, stateRoot.Path); err != nil {
		return currentSyncResult{}, true, err
	} else if activeExists {
		return currentSyncResult{}, true, fmt.Errorf("current sync committed transaction still requires exact active intent cleanup: %s", active.PlanSHA256)
	}
	if owner, ownerExists, err := readCurrentSyncOwner(caseFull, stateRoot.Path); err != nil {
		return currentSyncResult{}, true, err
	} else if ownerExists {
		return currentSyncResult{}, true, fmt.Errorf("current sync committed transaction still requires exact transaction owner cleanup: %s", owner.PlanSHA256)
	}
	if err := lease.Validate(); err != nil {
		return currentSyncResult{}, true, err
	}
	currentReceipt, receiptExists, err := readCurrentSyncReceipt(stateRoot.Path)
	if err != nil || !receiptExists || !currentSyncCanonicalEqual(currentReceipt, receipt) {
		return currentSyncResult{}, true, fmt.Errorf("current sync durable receipt changed during strict replay: %w", err)
	}
	if _, _, err := validateCurrentSyncReplayState(caseFull, stateRoot.Path, receipt); err != nil {
		return currentSyncResult{}, true, err
	}
	if err := lease.Validate(); err != nil {
		return currentSyncResult{}, true, err
	}
	currentReceipt, receiptExists, err = readCurrentSyncReceipt(stateRoot.Path)
	if err != nil || !receiptExists || !currentSyncCanonicalEqual(currentReceipt, receipt) {
		return currentSyncResult{}, true, fmt.Errorf("current sync durable receipt changed before strict replay return: %w", err)
	}
	if _, activeExists, err := readCurrentSyncIntent(caseFull, stateRoot.Path); err != nil || activeExists {
		return currentSyncResult{}, true, fmt.Errorf("current sync active intent changed before strict replay return: %w", err)
	}
	if _, ownerExists, err := readCurrentSyncOwner(caseFull, stateRoot.Path); err != nil || ownerExists {
		return currentSyncResult{}, true, fmt.Errorf("current sync transaction owner changed before strict replay return: %w", err)
	}
	return currentSyncResult{
		SchemaVersion:     currentSyncSchemaVersion,
		Kind:              currentSyncResultKind,
		Command:           receipt.Command,
		Status:            "already-current",
		CaseRoot:          caseFull,
		Pack:              receipt.Pack,
		IsMutation:        false,
		Applied:           false,
		Replay:            true,
		AlreadyCurrent:    true,
		PlanSHA256:        receipt.PlanSHA256,
		TransactionSHA256: receipt.TransactionSHA256,
		ReceiptPath:       projectstate.CurrentDir + "/" + currentSyncReceiptRel,
		Receipt:           &receipt,
	}, true, nil
}

func currentSyncTransactionIdentityFor(intent currentSyncIntent) currentSyncTransactionIdentity {
	return currentSyncTransactionIdentity{
		SchemaVersion:     intent.SchemaVersion,
		Kind:              intent.Kind,
		PlanSHA256:        intent.PlanSHA256,
		TransactionPath:   intent.TransactionPath,
		CaseRootIdentity:  intent.CaseRootIdentity,
		StateRootIdentity: intent.StateRootIdentity,
		Plan:              intent.Plan,
		Roots:             intent.Roots,
		Leaves:            intent.Leaves,
		NoAuthority:       intent.NoAuthority,
		NoConfirmed:       intent.NoConfirmed,
		NoHeavyTool:       intent.NoHeavyTool,
		NoSyncPromote:     intent.NoSyncPromote,
	}
}

func readCurrentSyncIntent(caseRoot, stateRoot string) (currentSyncIntent, bool, error) {
	path := filepath.Join(stateRoot, filepath.FromSlash(currentSyncIntentRel))
	data, exists, err := currentSyncReadOptional(caseRoot, path, "current sync durable intent", currentSyncMaxTransactionBytes)
	if err != nil || !exists {
		return currentSyncIntent{}, exists, err
	}
	var intent currentSyncIntent
	if err := decodeCurrentSyncCanonical(data, &intent, "durable intent"); err != nil {
		return currentSyncIntent{}, false, err
	}
	if err := validateCurrentSyncIntentProject(intent, caseRoot, stateRoot); err != nil {
		return currentSyncIntent{}, false, err
	}
	return intent, true, nil
}

func currentSyncMaxProgressGenerations(intent currentSyncIntent) (int, error) {
	if err := validateCurrentSyncIntentStructure(intent); err != nil {
		return 0, err
	}
	operations := len(currentSyncExpectedProgressOperations(intent))
	if operations < 1 || operations > currentSyncMaxFiles*4+32 {
		return 0, fmt.Errorf("current sync durable progress operation count is invalid")
	}
	return 1 + operations*2, nil
}

func readCurrentSyncArchivedIntent(stateRoot, transactionPath string) (currentSyncIntent, error) {
	clean, err := currentSyncNormalizeTargetRel(transactionPath)
	if err != nil || clean != transactionPath || !strings.HasPrefix(transactionPath, currentSyncTransactionsRel+"/") {
		return currentSyncIntent{}, fmt.Errorf("current sync archived intent transaction path is invalid")
	}
	path := filepath.Join(stateRoot, filepath.FromSlash(transactionPath), currentSyncArchivedIntent)
	data, err := rekitfs.ReadStableRegularFileAnchored(stateRoot, path, "current sync archived intent", currentSyncMaxTransactionBytes)
	if err != nil {
		return currentSyncIntent{}, err
	}
	var intent currentSyncIntent
	if err := decodeCurrentSyncCanonical(data, &intent, "archived intent"); err != nil {
		return currentSyncIntent{}, err
	}
	if err := validateCurrentSyncIntentStructure(intent); err != nil {
		return currentSyncIntent{}, err
	}
	if intent.TransactionPath != transactionPath {
		return currentSyncIntent{}, fmt.Errorf("current sync archived intent transaction binding is invalid")
	}
	return intent, nil
}

func readCurrentSyncProgressHistory(stateRoot string, intent currentSyncIntent) ([]currentSyncProgress, error) {
	if err := validateCurrentSyncIntentStructure(intent); err != nil {
		return nil, err
	}
	progressDir := filepath.Join(stateRoot, filepath.FromSlash(currentSyncProgressDir(intent)))
	maxGenerations, err := currentSyncMaxProgressGenerations(intent)
	if err != nil {
		return nil, err
	}
	paths, err := rekitfs.ListRegularFilesAnchored(stateRoot, strings.TrimPrefix(currentSyncProgressDir(intent), projectstate.CurrentDir+"/"), "current sync durable progress", maxGenerations)
	if err != nil {
		return nil, err
	}
	history := make([]currentSyncProgress, 0, len(paths))
	for index, path := range paths {
		expectedName := fmt.Sprintf("%08d.json", index+1)
		if filepath.Base(path) != expectedName || !sameCurrentSyncPath(filepath.Dir(path), progressDir) {
			return nil, fmt.Errorf("current sync durable progress generations are not contiguous")
		}
		data, readErr := rekitfs.ReadStableRegularFileAnchored(stateRoot, path, "current sync durable progress", currentSyncMaxTransactionBytes)
		if readErr != nil {
			return nil, readErr
		}
		var progress currentSyncProgress
		if err := decodeCurrentSyncCanonical(data, &progress, "durable progress"); err != nil {
			return nil, err
		}
		if progress.Generation != index+1 {
			return nil, fmt.Errorf("current sync durable progress generation is invalid")
		}
		if err := validateCurrentSyncProgress(progress, intent); err != nil {
			return nil, err
		}
		if index > 0 {
			if err := validateCurrentSyncProgressTransition(history[index-1], progress, intent); err != nil {
				return nil, err
			}
		}
		history = append(history, progress)
	}
	return history, nil
}

func readCurrentSyncProgress(stateRoot string, intent currentSyncIntent) (currentSyncProgress, bool, error) {
	history, err := readCurrentSyncProgressHistory(stateRoot, intent)
	if err != nil {
		return currentSyncProgress{}, false, err
	}
	if len(history) == 0 {
		return currentSyncProgress{}, false, nil
	}
	return history[len(history)-1], true, nil
}

func decodeCurrentSyncCanonical(data []byte, target any, label string) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode current sync %s: %w", label, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("current sync %s has trailing data", label)
	}
	canonical, err := currentSyncCanonicalData(target)
	if err != nil || !bytes.Equal(data, canonical) {
		return fmt.Errorf("current sync %s is not canonical", label)
	}
	return nil
}

func validateCurrentSyncProgress(progress currentSyncProgress, intent currentSyncIntent) error {
	if err := validateCurrentSyncIntentStructure(intent); err != nil {
		return err
	}
	if progress.SchemaVersion != currentSyncSchemaVersion || progress.Kind != currentSyncProgressKind ||
		!strings.EqualFold(progress.TransactionSHA256, intent.TransactionSHA256) || progress.Generation < 1 ||
		!validCurrentSyncSHA(progress.ProgressSHA256) {
		return fmt.Errorf("current sync durable progress identity is invalid")
	}
	progressSHA, err := currentSyncCanonicalSHA(currentSyncProgressIdentityFor(progress))
	if err != nil || !strings.EqualFold(progressSHA, progress.ProgressSHA256) {
		return fmt.Errorf("current sync durable progress binding is invalid")
	}
	if err := validateCurrentSyncProgressState(progress, intent); err != nil {
		return err
	}
	return nil
}

func validateCurrentSyncProgressState(progress currentSyncProgress, intent currentSyncIntent) error {
	expected := currentSyncExpectedProgressOperations(intent)
	if len(progress.Completed) > len(expected) {
		return fmt.Errorf("current sync durable progress completed operations are invalid")
	}
	for index, operation := range progress.Completed {
		if !currentSyncCanonicalEqual(operation, expected[index]) {
			return fmt.Errorf("current sync durable progress completed operation %d is invalid", index+1)
		}
	}
	if progress.Pending != nil {
		if len(progress.Completed) >= len(expected) || !currentSyncCanonicalEqual(*progress.Pending, expected[len(progress.Completed)]) {
			return fmt.Errorf("current sync durable progress pending operation is invalid")
		}
	}
	completedPhase := currentSyncProgressPhaseForCompleted(progress.Completed)
	if progress.Phase != completedPhase {
		return fmt.Errorf("current sync durable progress phase is inconsistent")
	}
	return nil
}

func currentSyncExpectedProgressOperations(intent currentSyncIntent) []currentSyncProgressOperation {
	result := []currentSyncProgressOperation{}
	sequence := 1
	appendOperation := func(kind, target string) {
		result = append(result, currentSyncProgressOperation{Sequence: sequence, Kind: kind, Target: target})
		sequence++
	}
	appendOperation("stage-validated", "")
	for _, root := range intent.Roots {
		if !root.Mutate {
			continue
		}
		appendOperation("root-live-to-previous", root.Name)
		appendOperation("root-stage-to-live", root.Name)
	}
	for _, leaf := range intent.Leaves {
		if leaf.ActivateLast || !leaf.Mutate {
			continue
		}
		if leaf.BeforeExists {
			appendOperation("leaf-live-to-previous", leaf.Path)
		}
		if leaf.AfterExists {
			appendOperation("leaf-stage-to-live", leaf.Path)
		}
	}
	appendOperation("ready-to-activate", projectstate.CurrentDir+"/instance.yml")
	for _, leaf := range intent.Leaves {
		if !leaf.ActivateLast || !leaf.Mutate {
			continue
		}
		if leaf.BeforeExists {
			appendOperation("activation-live-to-previous", leaf.Path)
		}
		appendOperation("activation-stage-to-live", leaf.Path)
	}
	appendOperation("activated", projectstate.CurrentDir+"/instance.yml")
	appendOperation("bundle-validated", runtimebundle.ManifestRel)
	if intent.Plan.PreviousReceipt != nil {
		appendOperation("receipt-live-to-previous", currentSyncReceiptRel)
	}
	appendOperation("receipt-stage-to-live", currentSyncReceiptRel)
	appendOperation("receipt-committed", currentSyncReceiptRel)
	return result
}

func currentSyncProgressPhaseForCompleted(completed []currentSyncProgressOperation) string {
	phase := "prepared"
	for _, operation := range completed {
		switch operation.Kind {
		case "stage-validated", "root-live-to-previous", "root-stage-to-live", "leaf-live-to-previous", "leaf-stage-to-live":
			phase = "publishing"
		case "ready-to-activate", "activation-live-to-previous", "activation-stage-to-live":
			phase = "ready-to-activate"
		case "activated", "bundle-validated", "receipt-live-to-previous", "receipt-stage-to-live":
			phase = "activated"
		case "receipt-committed":
			phase = "receipt-committed"
		}
	}
	return phase
}

func validateCurrentSyncProgressTransition(previous, next currentSyncProgress, intent currentSyncIntent) error {
	if err := validateCurrentSyncProgress(previous, intent); err != nil {
		return err
	}
	if err := validateCurrentSyncProgress(next, intent); err != nil {
		return err
	}
	if next.Generation != previous.Generation+1 ||
		!currentSyncProgressOperationPrefix(previous.Completed, next.Completed) ||
		currentSyncProgressPhaseRank(next.Phase) < currentSyncProgressPhaseRank(previous.Phase) {
		return fmt.Errorf("current sync durable progress transition is not monotonic")
	}
	if previous.Pending == nil {
		if len(next.Completed) != len(previous.Completed) || next.Pending == nil {
			return fmt.Errorf("current sync durable progress must publish the next pending operation before completion")
		}
	} else {
		if next.Pending != nil || len(next.Completed) != len(previous.Completed)+1 ||
			!currentSyncCanonicalEqual(next.Completed[len(next.Completed)-1], *previous.Pending) {
			return fmt.Errorf("current sync durable progress must complete only the pending operation")
		}
	}
	return nil
}

func currentSyncProgressOperationPrefix(prefix, value []currentSyncProgressOperation) bool {
	if len(prefix) > len(value) {
		return false
	}
	for index := range prefix {
		if !currentSyncCanonicalEqual(prefix[index], value[index]) {
			return false
		}
	}
	return true
}

func currentSyncProgressPhaseRank(phase string) int {
	switch phase {
	case "prepared":
		return 0
	case "publishing":
		return 1
	case "ready-to-activate":
		return 2
	case "activated":
		return 3
	case "receipt-committed":
		return 4
	default:
		return -1
	}
}

func currentSyncSignProgress(progress currentSyncProgress) (currentSyncProgress, error) {
	var err error
	progress.ProgressSHA256, err = currentSyncCanonicalSHA(currentSyncProgressIdentityFor(progress))
	return progress, err
}

func publishCurrentSyncIntent(caseRoot string, intent currentSyncIntent) (bool, error) {
	if err := validateCurrentSyncIntentStructure(intent); err != nil {
		return false, err
	}
	owner := currentSyncOwnerFor(intent)
	ownerData, err := currentSyncCanonicalData(owner)
	if err != nil {
		return false, err
	}
	ownerReplay, err := rekitfs.WriteExclusiveRegularFileAnchoredWriteThrough(
		caseRoot,
		projectstate.CurrentDir+"/"+currentSyncOwnerRel,
		"current sync transaction owner",
		ownerData,
	)
	if err != nil {
		return false, err
	}
	data, err := currentSyncCanonicalData(intent)
	if err != nil {
		return false, err
	}
	archivedReplay, err := rekitfs.WriteExclusiveRegularFileAnchoredWriteThrough(caseRoot, projectstate.CurrentDir+"/"+currentSyncArchivedIntentPath(intent), "current sync archived intent", data)
	if err != nil {
		return false, err
	}
	activeReplay, err := rekitfs.WriteExclusiveRegularFileAnchoredWriteThrough(caseRoot, projectstate.CurrentDir+"/"+currentSyncIntentRel, "current sync durable intent", data)
	return ownerReplay && archivedReplay && activeReplay, err
}

func currentSyncOwnerFor(intent currentSyncIntent) currentSyncOwner {
	return currentSyncOwner{
		SchemaVersion:     currentSyncSchemaVersion,
		Kind:              "steamai-current-sync-owner",
		PlanSHA256:        intent.PlanSHA256,
		TransactionSHA256: intent.TransactionSHA256,
		TransactionPath:   intent.TransactionPath,
		Intent:            intent,
	}
}

func validateCurrentSyncOwner(owner currentSyncOwner) error {
	if owner.SchemaVersion != currentSyncSchemaVersion ||
		owner.Kind != "steamai-current-sync-owner" ||
		!validCurrentSyncSHA(owner.PlanSHA256) ||
		!validCurrentSyncSHA(owner.TransactionSHA256) ||
		owner.TransactionPath != currentSyncTransactionPath(owner.PlanSHA256) ||
		owner.Intent.PlanSHA256 != owner.PlanSHA256 ||
		owner.Intent.TransactionSHA256 != owner.TransactionSHA256 ||
		owner.Intent.TransactionPath != owner.TransactionPath {
		return fmt.Errorf("current sync transaction owner identity is invalid")
	}
	return validateCurrentSyncIntentStructure(owner.Intent)
}

func readCurrentSyncOwner(
	caseRoot,
	stateRoot string,
) (currentSyncOwner, bool, error) {
	path := filepath.Join(stateRoot, filepath.FromSlash(currentSyncOwnerRel))
	data, exists, err := currentSyncReadOptional(
		caseRoot,
		path,
		"current sync transaction owner",
		currentSyncMaxTransactionBytes,
	)
	if err != nil || !exists {
		return currentSyncOwner{}, exists, err
	}
	var owner currentSyncOwner
	if err := decodeCurrentSyncCanonical(data, &owner, "transaction owner"); err != nil {
		return currentSyncOwner{}, false, err
	}
	if err := validateCurrentSyncOwner(owner); err != nil {
		return currentSyncOwner{}, false, err
	}
	return owner, true, nil
}

func publishCurrentSyncProgress(caseRoot string, intent currentSyncIntent, progress currentSyncProgress) (bool, error) {
	if err := validateCurrentSyncProgress(progress, intent); err != nil {
		return false, err
	}
	if progress.Generation > 1 {
		previous, exists, err := readCurrentSyncProgress(filepath.Join(caseRoot, projectstate.CurrentDir), intent)
		if err != nil {
			return false, err
		}
		if !exists || previous.Generation != progress.Generation-1 {
			return false, fmt.Errorf("current sync durable progress predecessor is missing")
		}
		if err := validateCurrentSyncProgressTransition(previous, progress, intent); err != nil {
			return false, err
		}
	}
	data, err := currentSyncCanonicalData(progress)
	if err != nil {
		return false, err
	}
	return rekitfs.WriteExclusiveRegularFileAnchoredWriteThrough(caseRoot, projectstate.CurrentDir+"/"+currentSyncProgressPath(intent, progress.Generation), "current sync durable progress", data)
}

func currentSyncBeginProgressOperation(progress currentSyncProgress, intent currentSyncIntent) (currentSyncProgress, error) {
	if err := validateCurrentSyncProgress(progress, intent); err != nil {
		return currentSyncProgress{}, err
	}
	if progress.Pending != nil {
		return currentSyncProgress{}, fmt.Errorf("current sync durable progress already has a pending operation")
	}
	expected := currentSyncExpectedProgressOperations(intent)
	if len(progress.Completed) >= len(expected) {
		return currentSyncProgress{}, fmt.Errorf("current sync durable progress is already complete")
	}
	next := progress
	next.Generation++
	operation := expected[len(progress.Completed)]
	next.Pending = &operation
	return currentSyncSignProgress(next)
}

func currentSyncCompleteProgressOperation(progress currentSyncProgress, intent currentSyncIntent) (currentSyncProgress, error) {
	if err := validateCurrentSyncProgress(progress, intent); err != nil {
		return currentSyncProgress{}, err
	}
	if progress.Pending == nil {
		return currentSyncProgress{}, fmt.Errorf("current sync durable progress has no pending operation")
	}
	next := progress
	next.Generation++
	next.Completed = append(append([]currentSyncProgressOperation{}, progress.Completed...), *progress.Pending)
	next.Pending = nil
	next.Phase = currentSyncProgressPhaseForCompleted(next.Completed)
	return currentSyncSignProgress(next)
}

func validateCurrentSyncIntent(intent currentSyncIntent, caseRoot, stateRoot string) error {
	if err := validateCurrentSyncIntentProject(intent, caseRoot, stateRoot); err != nil {
		return err
	}
	return validateCurrentSyncIntentSource(intent)
}

func validateCurrentSyncIntentProject(intent currentSyncIntent, caseRoot, stateRoot string) error {
	if err := validateCurrentSyncIntentStructure(intent); err != nil {
		return err
	}
	caseFull, err := filepath.Abs(caseRoot)
	if err != nil || !sameCurrentSyncPath(caseFull, intent.Plan.CaseRoot) {
		return fmt.Errorf("current sync durable intent case root is invalid")
	}
	stateFull, err := filepath.Abs(stateRoot)
	if err != nil || !sameCurrentSyncPath(stateFull, filepath.Join(caseFull, projectstate.CurrentDir)) {
		return fmt.Errorf("current sync durable intent state root is invalid")
	}
	if intent.CaseRootIdentity != intent.Plan.CaseRootIdentity || intent.StateRootIdentity != intent.Plan.StateRootIdentity {
		return fmt.Errorf("current sync durable intent filesystem identity binding is invalid")
	}
	if err := intent.CaseRootIdentity.Validate(); err != nil {
		return err
	}
	if err := intent.StateRootIdentity.Validate(); err != nil {
		return err
	}
	if err := currentSyncValidateRootIdentity(caseFull, intent.CaseRootIdentity, "case root"); err != nil {
		return err
	}
	return currentSyncValidateRootIdentity(stateFull, intent.StateRootIdentity, "state root")
}

func validateCurrentSyncIntentSource(intent currentSyncIntent) error {
	return currentSyncValidateRootIdentity(intent.Plan.SourceRepoRoot, intent.Plan.SourceRootIdentity, "source root")
}

func validateCurrentSyncIntentStructure(intent currentSyncIntent) error {
	if intent.SchemaVersion != currentSyncSchemaVersion || intent.Kind != currentSyncIntentKind {
		return fmt.Errorf("current sync durable intent identity is invalid")
	}
	if !validCurrentSyncSHA(intent.PlanSHA256) || !strings.EqualFold(intent.PlanSHA256, intent.Plan.ExpectedPlanSHA256) {
		return fmt.Errorf("current sync durable intent plan binding is invalid")
	}
	if intent.TransactionPath != currentSyncTransactionPath(intent.PlanSHA256) {
		return fmt.Errorf("current sync durable intent transaction path is invalid")
	}
	identitySHA, err := currentSyncCanonicalSHA(currentSyncIdentity(intent.Plan))
	if err != nil || !strings.EqualFold(identitySHA, intent.PlanSHA256) {
		return fmt.Errorf("current sync durable intent plan identity is invalid")
	}
	if err := validateCurrentSyncPlanForIntent(intent.Plan); err != nil {
		return fmt.Errorf("current sync durable intent plan is invalid: %w", err)
	}
	if !validCurrentSyncSHA(intent.TransactionSHA256) {
		return fmt.Errorf("current sync durable intent transaction binding is invalid")
	}
	transactionSHA, err := currentSyncCanonicalSHA(currentSyncTransactionIdentityFor(intent))
	if err != nil || !strings.EqualFold(transactionSHA, intent.TransactionSHA256) {
		return fmt.Errorf("current sync durable intent transaction identity is invalid")
	}
	if !intent.NoAuthority || !intent.NoConfirmed || !intent.NoHeavyTool || !intent.NoSyncPromote {
		return fmt.Errorf("current sync durable intent safety boundary is invalid")
	}
	expectedRoots, err := currentSyncIntentRoots(intent.Plan, intent.TransactionPath)
	if err != nil || !currentSyncCanonicalEqual(intent.Roots, expectedRoots) {
		return fmt.Errorf("current sync durable intent controlled roots do not equal the reviewed plan")
	}
	expectedLeaves, err := currentSyncIntentLeaves(intent.Plan, intent.TransactionPath)
	if err != nil || !currentSyncCanonicalEqual(intent.Leaves, expectedLeaves) {
		return fmt.Errorf("current sync durable intent leaves do not equal the reviewed plan")
	}
	return nil
}

func validateCurrentSyncPlanForIntent(plan CurrentSyncPlan) error {
	if plan.SchemaVersion != currentSyncSchemaVersion || plan.Kind != currentSyncPlanKind || plan.Command != "sync" {
		return fmt.Errorf("current sync reviewed plan identity is invalid")
	}
	if plan.ForceLocalTemplates || plan.IsMutation || plan.Applied || plan.Replay || plan.AlreadyCurrent || plan.RecoveryPending || !plan.RequiresReview || !plan.RequiresConfirmation {
		return fmt.Errorf("current sync reviewed plan state is invalid")
	}
	if err := plan.CaseRootIdentity.Validate(); err != nil {
		return err
	}
	if err := plan.StateRootIdentity.Validate(); err != nil {
		return err
	}
	if err := plan.SourceRootIdentity.Validate(); err != nil {
		return err
	}
	if plan.SourceExecutableFile.Identity == nil {
		return fmt.Errorf("current sync source executable physical identity is missing")
	}
	if err := plan.SourceExecutableFile.Identity.Validate(); err != nil {
		return err
	}
	if err := validateCurrentSyncBinding(plan.SourceExecutableFile, "runtime-executable-source"); err != nil {
		return err
	}
	if !sameCurrentSyncPath(filepath.FromSlash(plan.SourceExecutableFile.Path), plan.SourceExecutable) {
		return fmt.Errorf("current sync source executable path binding is invalid")
	}
	manifestPath := projectstate.CurrentDir + "/" + runtimebundle.ManifestRel
	if err := validateCurrentSyncBinding(plan.CurrentManifest, "runtime-bundle-manifest"); err != nil || plan.CurrentManifest.Path != manifestPath {
		return fmt.Errorf("current sync current manifest binding is invalid")
	}
	if err := validateCurrentSyncBinding(plan.NextManifest, "runtime-bundle-manifest"); err != nil || plan.NextManifest.Path != manifestPath {
		return fmt.Errorf("current sync next manifest binding is invalid")
	}
	if plan.PreviousReceipt != nil {
		if err := validateCurrentSyncBinding(*plan.PreviousReceipt, "current-sync-receipt"); err != nil || plan.PreviousReceipt.Path != projectstate.CurrentDir+"/"+currentSyncReceiptRel {
			return fmt.Errorf("current sync previous receipt binding is invalid")
		}
	}
	for _, inventory := range []CurrentSyncInventory{plan.CurrentControlled, plan.NextControlled, plan.CurrentTargets, plan.NextTargets} {
		if err := validateCurrentSyncInventory(inventory); err != nil {
			return err
		}
	}
	if err := validateCurrentSyncControlledWrites(plan); err != nil {
		return err
	}
	if _, err := currentSyncIntentLeaves(plan, currentSyncTransactionPath(plan.ExpectedPlanSHA256)); err != nil {
		return err
	}
	identitySHA, err := currentSyncCanonicalSHA(currentSyncIdentity(plan))
	if err != nil || !validCurrentSyncSHA(plan.ExpectedPlanSHA256) || !strings.EqualFold(identitySHA, plan.ExpectedPlanSHA256) {
		return fmt.Errorf("current sync reviewed plan SHA-256 is invalid")
	}
	alreadyCurrent := plan.CurrentControlled.SHA256 == plan.NextControlled.SHA256 && plan.CurrentTargets.SHA256 == plan.NextTargets.SHA256 && len(plan.ObsoleteControlled) == 0
	if plan.AlreadyCurrent != alreadyCurrent {
		return fmt.Errorf("current sync reviewed plan currentness is invalid")
	}
	expectedStatus := "ready-to-refresh"
	if alreadyCurrent {
		expectedStatus = "already-current"
	}
	if plan.Status != expectedStatus {
		return fmt.Errorf("current sync reviewed plan status is invalid")
	}
	if !currentSyncStringSlicesEqual(plan.ApplyArgs, currentSyncExpectedApplyArgs(plan)) {
		return fmt.Errorf("current sync reviewed plan ApplyArgs are not exact")
	}
	return nil
}

func currentSyncExpectedApplyArgs(plan CurrentSyncPlan) []string {
	return []string{
		"-Command", plan.Command,
		"-Target", plan.CaseRoot,
		"-Pack", plan.Pack,
		"-ProjectName", plan.ProjectName,
		"-SourceRepoRoot", plan.SourceRepoRoot,
		"-SourceExecutable", plan.SourceExecutable,
		"-ExpectedCurrentSyncPlanSha256", plan.ExpectedPlanSHA256,
		"-Apply",
		"-Format", "json",
	}
}

func validateCurrentSyncBinding(binding CurrentSyncBinding, kind string) error {
	if binding.Path == "" || binding.Kind != kind || !validCurrentSyncSHA(binding.SHA256) || binding.Size < 0 || binding.Mode == 0 {
		return fmt.Errorf("current sync %s binding is invalid: %s", kind, binding.Path)
	}
	return nil
}

func validateCurrentSyncInventory(inventory CurrentSyncInventory) error {
	canonical, err := currentSyncInventory(inventory.Entries)
	if err != nil || !currentSyncCanonicalEqual(inventory, canonical) {
		return fmt.Errorf("current sync inventory does not match its canonical entries")
	}
	for _, entry := range inventory.Entries {
		if !validCurrentSyncSHA(entry.SHA256) || entry.Mode == 0 {
			return fmt.Errorf("current sync inventory binding is invalid: %s", entry.Path)
		}
	}
	return nil
}

func currentSyncIntentRoots(plan CurrentSyncPlan, transactionPath string) ([]currentSyncIntentRoot, error) {
	if !validCurrentSyncSHA(plan.ExpectedPlanSHA256) || transactionPath != currentSyncTransactionPath(plan.ExpectedPlanSHA256) {
		return nil, fmt.Errorf("current sync controlled-root transaction path is invalid")
	}
	if err := validateCurrentSyncInventory(plan.CurrentControlled); err != nil {
		return nil, err
	}
	if err := validateCurrentSyncInventory(plan.NextControlled); err != nil {
		return nil, err
	}
	before := map[string][]CurrentSyncBinding{}
	after := map[string][]CurrentSyncBinding{}
	for _, item := range []struct {
		inventory CurrentSyncInventory
		target    map[string][]CurrentSyncBinding
	}{{plan.CurrentControlled, before}, {plan.NextControlled, after}} {
		for _, binding := range item.inventory.Entries {
			root, ok := currentSyncControlledBindingRoot(binding.Path)
			if !ok {
				return nil, fmt.Errorf("current sync controlled inventory path is invalid: %s", binding.Path)
			}
			item.target[root] = append(item.target[root], binding)
		}
	}
	roots := make([]currentSyncIntentRoot, 0, len(currentSyncControlledRoots))
	for _, root := range currentSyncControlledRoots {
		left, err := currentSyncInventory(before[root])
		if err != nil {
			return nil, err
		}
		right, err := currentSyncInventory(after[root])
		if err != nil {
			return nil, err
		}
		roots = append(roots, currentSyncIntentRoot{
			Name:         root,
			Before:       left,
			After:        right,
			StagePath:    transactionPath + "/stage/controlled/" + root,
			PreviousPath: transactionPath + "/previous/" + root,
			Mutate:       !currentSyncCanonicalEqual(left, right),
		})
	}
	return roots, nil
}

func currentSyncControlledBindingRoot(value string) (string, bool) {
	clean, err := currentSyncNormalizeTargetRel(value)
	if err != nil || clean != value || !strings.HasPrefix(clean, projectstate.CurrentDir+"/") {
		return "", false
	}
	remainder := strings.TrimPrefix(clean, projectstate.CurrentDir+"/")
	root, leaf, ok := strings.Cut(remainder, "/")
	return root, ok && leaf != "" && currentSyncControlledRoot(root)
}

func validateCurrentSyncControlledWrites(plan CurrentSyncPlan) error {
	before := currentSyncBindingMap(plan.CurrentControlled)
	after := currentSyncBindingMap(plan.NextControlled)
	seen := map[string]bool{}
	for _, write := range plan.Writes {
		if _, ok := currentSyncControlledBindingRoot(write.Path); !ok {
			continue
		}
		key := strings.ToLower(write.Path)
		if seen[key] {
			return fmt.Errorf("current sync controlled write is duplicated: %s", write.Path)
		}
		seen[key] = true
		if err := validateCurrentSyncWriteBinding(write, before[key], after[key]); err != nil {
			return err
		}
	}
	union := currentSyncBindingUnion(before, after)
	if len(seen) != len(union) {
		return fmt.Errorf("current sync controlled writes do not cover the exact inventory transition")
	}
	obsolete := []string{}
	for key, binding := range before {
		if _, ok := after[key]; !ok {
			obsolete = append(obsolete, binding.Path)
		}
	}
	sort.Strings(obsolete)
	if !currentSyncStringSlicesEqual(plan.ObsoleteControlled, obsolete) {
		return fmt.Errorf("current sync obsolete controlled files do not match the exact inventory transition")
	}
	return nil
}

func currentSyncIntentLeaves(plan CurrentSyncPlan, transactionPath string) ([]currentSyncIntentLeaf, error) {
	if !validCurrentSyncSHA(plan.ExpectedPlanSHA256) || transactionPath != currentSyncTransactionPath(plan.ExpectedPlanSHA256) {
		return nil, fmt.Errorf("current sync leaf transaction path is invalid")
	}
	if err := validateCurrentSyncInventory(plan.CurrentTargets); err != nil {
		return nil, err
	}
	if err := validateCurrentSyncInventory(plan.NextTargets); err != nil {
		return nil, err
	}
	before := currentSyncBindingMap(plan.CurrentTargets)
	after := currentSyncBindingMap(plan.NextTargets)
	for _, binding := range append(append([]CurrentSyncBinding{}, plan.CurrentTargets.Entries...), plan.NextTargets.Entries...) {
		if !currentSyncSafeTargetRel(binding.Path) {
			return nil, fmt.Errorf("current sync leaf inventory path is invalid: %s", binding.Path)
		}
		if _, controlled := currentSyncControlledBindingRoot(binding.Path); controlled {
			return nil, fmt.Errorf("current sync controlled path appears in leaf inventory: %s", binding.Path)
		}
	}
	writes := map[string]CurrentSyncWrite{}
	for _, write := range plan.Writes {
		if _, controlled := currentSyncControlledBindingRoot(write.Path); controlled {
			continue
		}
		clean, err := currentSyncNormalizeTargetRel(write.Path)
		if err != nil || clean != write.Path || !currentSyncSafeTargetRel(clean) {
			return nil, fmt.Errorf("current sync leaf write path is invalid: %s", write.Path)
		}
		key := strings.ToLower(clean)
		if _, exists := writes[key]; exists {
			return nil, fmt.Errorf("current sync leaf write is duplicated: %s", write.Path)
		}
		writes[key] = write
	}
	union := currentSyncBindingUnion(before, after)
	if len(writes) != len(union) {
		return nil, fmt.Errorf("current sync leaf writes do not cover the exact target transition")
	}
	paths := make([]string, 0, len(union))
	for _, binding := range union {
		paths = append(paths, binding.Path)
	}
	sort.Strings(paths)
	leaves := make([]currentSyncIntentLeaf, 0, len(paths))
	for index, path := range paths {
		key := strings.ToLower(path)
		write, ok := writes[key]
		if !ok {
			return nil, fmt.Errorf("current sync leaf write is missing: %s", path)
		}
		left, leftOK := before[key]
		right, rightOK := after[key]
		if err := validateCurrentSyncWriteBinding(write, left, right); err != nil {
			return nil, err
		}
		mode := left.Mode
		if rightOK {
			mode = right.Mode
		}
		if leftOK && rightOK && left.Mode != right.Mode {
			return nil, fmt.Errorf("current sync leaf mode transition is unsupported: %s", path)
		}
		mutate := leftOK != rightOK
		if leftOK && rightOK {
			mutate = !strings.EqualFold(left.SHA256, right.SHA256) || left.Size != right.Size || left.Mode != right.Mode
		}
		leaf := currentSyncIntentLeaf{
			Path:         path,
			Kind:         write.Kind,
			StagePath:    fmt.Sprintf("%s/stage/leaves/%06d.bin", transactionPath, index),
			PreviousPath: fmt.Sprintf("%s/previous/leaves/%06d.bin", transactionPath, index),
			BeforeExists: write.BeforeExists,
			BeforeSHA256: write.BeforeSHA256,
			BeforeSize:   write.BeforeSize,
			AfterExists:  write.AfterExists,
			AfterSHA256:  write.AfterSHA256,
			AfterSize:    write.AfterSize,
			Mode:         mode,
			Mutate:       mutate,
			ActivateLast: path == projectstate.CurrentDir+"/instance.yml",
		}
		leaves = append(leaves, leaf)
	}
	activateLast := 0
	for _, leaf := range leaves {
		if leaf.ActivateLast {
			activateLast++
			if !leaf.AfterExists {
				return nil, fmt.Errorf("current sync final instance activation cannot delete instance.yml")
			}
		}
	}
	if activateLast != 1 {
		return nil, fmt.Errorf("current sync must bind exactly one final instance activation")
	}
	return leaves, nil
}

func validateCurrentSyncWriteBinding(write CurrentSyncWrite, before, after CurrentSyncBinding) error {
	beforeExists := before.Path != ""
	afterExists := after.Path != ""
	if strings.TrimSpace(write.Action) == "" || strings.TrimSpace(write.Kind) == "" || write.BeforeExists != beforeExists || write.AfterExists != afterExists {
		return fmt.Errorf("current sync write existence binding is invalid: %s", write.Path)
	}
	if beforeExists {
		if write.Path != before.Path || write.Kind != before.Kind || !strings.EqualFold(write.BeforeSHA256, before.SHA256) || write.BeforeSize != before.Size {
			return fmt.Errorf("current sync write before binding is invalid: %s", write.Path)
		}
	} else if write.BeforeSHA256 != "" || write.BeforeSize != 0 {
		return fmt.Errorf("current sync absent write before binding is invalid: %s", write.Path)
	}
	if afterExists {
		if write.Path != after.Path || write.Kind != after.Kind || !strings.EqualFold(write.AfterSHA256, after.SHA256) || write.AfterSize != after.Size {
			return fmt.Errorf("current sync write after binding is invalid: %s", write.Path)
		}
	} else if write.AfterSHA256 != "" || write.AfterSize != 0 {
		return fmt.Errorf("current sync absent write after binding is invalid: %s", write.Path)
	}
	return nil
}

func currentSyncBindingMap(inventory CurrentSyncInventory) map[string]CurrentSyncBinding {
	result := make(map[string]CurrentSyncBinding, len(inventory.Entries))
	for _, binding := range inventory.Entries {
		result[strings.ToLower(binding.Path)] = binding
	}
	return result
}

func currentSyncBindingUnion(left, right map[string]CurrentSyncBinding) map[string]CurrentSyncBinding {
	result := make(map[string]CurrentSyncBinding, len(left)+len(right))
	for key, binding := range left {
		result[key] = binding
	}
	for key, binding := range right {
		if previous, ok := result[key]; ok && previous.Path != binding.Path {
			continue
		}
		result[key] = binding
	}
	return result
}

func currentSyncCanonicalData(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func currentSyncCanonicalEqual(left, right any) bool {
	leftData, leftErr := json.Marshal(left)
	rightData, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftData, rightData)
}

func currentSyncStringSlicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func currentSyncControlledRoot(value string) bool {
	for _, root := range currentSyncControlledRoots {
		if value == root {
			return true
		}
	}
	return false
}

func currentSyncSafeTargetRel(value string) bool {
	clean, err := currentSyncNormalizeTargetRel(value)
	if err != nil {
		return false
	}
	if clean == projectstate.CurrentDir+"/instance.yml" || clean == projectstate.CurrentDir+"/state.json" {
		return true
	}
	return clean != projectstate.CurrentDir && !strings.HasPrefix(clean, projectstate.CurrentDir+"/") && clean != projectstate.LegacyDir && !strings.HasPrefix(clean, projectstate.LegacyDir+"/")
}

func currentSyncNormalizeTargetRel(value string) (string, error) {
	raw := strings.TrimSpace(value)
	if raw == "" || raw != value || strings.ContainsRune(raw, '\x00') {
		return "", fmt.Errorf("current sync target path is not a strict relative path: %q", value)
	}
	if strings.Contains(raw, `\`) {
		return "", fmt.Errorf("current sync target path must use portable slash separators: %s", value)
	}
	if strings.HasPrefix(raw, "/") || (len(raw) >= 2 && raw[1] == ':') {
		return "", fmt.Errorf("current sync target path is not a strict relative path: %s", value)
	}
	parts := strings.Split(raw, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." || strings.TrimSpace(part) != part ||
			strings.ContainsAny(part, `:<>"|?*`) || part != strings.TrimRight(part, ". ") || currentSyncReservedWindowsName(part) {
			return "", fmt.Errorf("current sync target path contains an invalid portable component: %s", value)
		}
	}
	return strings.Join(parts, "/"), nil
}

func currentSyncReservedWindowsName(value string) bool {
	base, _, _ := strings.Cut(strings.ToUpper(value), ".")
	switch base {
	case "CON", "PRN", "AUX", "NUL", "CLOCK$":
		return true
	}
	return len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9'
}

func sameCurrentSyncPath(left, right string) bool {
	left, leftErr := filepath.Abs(left)
	right, rightErr := filepath.Abs(right)
	return leftErr == nil && rightErr == nil && strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}
