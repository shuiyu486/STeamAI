package sync

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/casebind"
	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/missionintent"
	"github.com/shuiyu486/re-context-kits/internal/rekit/review"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtimebundle"
	"github.com/shuiyu486/re-context-kits/internal/rekit/sourceartifact"
)

type initWriteIdentity struct {
	Path                string `json:"path"`
	Kind                string `json:"kind"`
	Action              string `json:"action"`
	SourceContentSHA256 string `json:"sourceContentSha256"`
	TargetExists        bool   `json:"targetExists"`
	TargetKind          string `json:"targetKind"`
	TargetContentSHA256 string `json:"targetContentSha256,omitempty"`
}

type initPlanIdentity struct {
	SchemaVersion      int                 `json:"schemaVersion"`
	Command            string              `json:"command"`
	CaseRoot           string              `json:"caseRoot"`
	RepoRoot           string              `json:"repoRoot"`
	Pack               string              `json:"pack"`
	ProjectName        string              `json:"projectName"`
	TargetClass        string              `json:"targetClass"`
	ManifestSHA256     string              `json:"manifestSha256"`
	GitignoreAvailable bool                `json:"gitignoreAvailable"`
	Writes             []initWriteIdentity `json:"writes"`
}

type initSyncStateIdentity struct {
	SchemaVersion int                         `json:"schemaVersion"`
	TemplateRoot  string                      `json:"templateRoot"`
	TemplatePack  string                      `json:"templatePack"`
	Managed       map[string]syncManagedEntry `json:"managed"`
}

func finalizeInitPlan(plan InitPlan) (InitPlan, error) {
	plan.initSourceSHA256 = map[string]string{}
	plan.initTargetSHA256 = map[string]string{}
	identity := initPlanIdentity{
		SchemaVersion:      1,
		Command:            plan.Command,
		CaseRoot:           plan.CaseRoot,
		RepoRoot:           plan.RepoRoot,
		Pack:               plan.Pack,
		ProjectName:        plan.ProjectName,
		TargetClass:        plan.TargetClass,
		ManifestSHA256:     plan.initManifestSHA256,
		GitignoreAvailable: plan.initGitignorePresent,
		Writes:             make([]initWriteIdentity, 0, len(plan.Writes)),
	}
	for _, write := range plan.Writes {
		sourceSHA, err := initWriteSourceSHA256(plan, write)
		if err != nil {
			return InitPlan{}, err
		}
		exists, kind, targetSHA, err := initTargetBinding(write.TargetPath)
		if err != nil {
			return InitPlan{}, err
		}
		identity.Writes = append(identity.Writes, initWriteIdentity{
			Path: write.Path, Kind: write.Kind, Action: write.Action,
			SourceContentSHA256: sourceSHA, TargetExists: exists,
			TargetKind: kind, TargetContentSHA256: targetSHA,
		})
		plan.initSourceSHA256[write.Path] = sourceSHA
		plan.initTargetSHA256[write.Path] = targetSHA
	}
	encoded, err := json.Marshal(identity)
	if err != nil {
		return InitPlan{}, err
	}
	sum := sha256.Sum256(encoded)
	plan.ExpectedPlanSHA256 = hex.EncodeToString(sum[:])
	plan.AdoptionBlockers = ordinaryInitAdoptionBlockers(plan)
	plan.AdoptionReady = plan.TargetClass == "ordinary-directory" && len(plan.AdoptionBlockers) == 0
	plan.ApplyArgs = []string{
		"-Command", plan.Command, "-Target", plan.CaseRoot, "-Pack", plan.Pack,
		"-ProjectName", plan.ProjectName, "-ExpectedInitPlanSha256", plan.ExpectedPlanSHA256,
		"-Apply", "-Format", "json",
	}
	return plan, nil
}

func classifyInitTarget(caseRoot, repoRoot, pack string) (string, instance.Instance, error) {
	if _, err := os.Lstat(caseRoot); os.IsNotExist(err) {
		if err := refsf.ValidateNoReparseComponents(filepath.Dir(caseRoot)); err != nil {
			return "invalid", instance.Instance{}, err
		}
		inst, readErr := instance.Read(caseRoot)
		return "missing", inst, readErr
	} else if err != nil {
		return "invalid", instance.Instance{}, err
	}
	if _, err := refsf.ValidateNonReparseDirectory(caseRoot, "init target"); err != nil {
		return "invalid", instance.Instance{}, err
	}
	inst, err := instance.Read(caseRoot)
	if err != nil {
		return "invalid", instance.Instance{}, err
	}
	if inst.Source == "missing" {
		if err := refsf.ValidateTreeNoReparse(caseRoot, "init ordinary target"); err != nil {
			return "invalid", instance.Instance{}, err
		}
		for _, stateDir := range []string{".steamai", ".rekit"} {
			if _, err := os.Lstat(filepath.Join(caseRoot, stateDir)); err == nil {
				return "invalid", instance.Instance{}, fmt.Errorf("init target contains partial %s state", stateDir)
			} else if !os.IsNotExist(err) {
				return "invalid", instance.Instance{}, err
			}
		}
		for _, skill := range []string{"steamai", "rekit"} {
			if _, err := os.Lstat(filepath.Join(caseRoot, ".claude", "skills", skill, "SKILL.md")); err == nil {
				return "invalid", instance.Instance{}, fmt.Errorf("init target contains a partial project-local %s skill", skill)
			} else if !os.IsNotExist(err) {
				return "invalid", instance.Instance{}, err
			}
		}
		return "ordinary-directory", inst, nil
	}
	if inst.Moved() {
		return "invalid", instance.Instance{}, instance.MovedRepairPreviewError(caseRoot, pack)
	}
	if strings.TrimSpace(inst.TemplateRoot) == "" {
		return "invalid", instance.Instance{}, fmt.Errorf("case is attached to a missing templateRoot")
	}
	if !casebind.SameExistingPath(inst.TemplateRoot, repoRoot) {
		return "invalid", instance.Instance{}, fmt.Errorf("case is attached to a different templateRoot or it is missing: %s", inst.TemplateRoot)
	}
	if strings.TrimSpace(inst.TemplatePack) != "" && !strings.EqualFold(inst.TemplatePack, pack) {
		return "invalid", instance.Instance{}, fmt.Errorf("case is attached to a different templatePack: %s", inst.TemplatePack)
	}
	inspection, err := missionintent.Inspect(caseRoot)
	if err != nil {
		return "invalid", instance.Instance{}, err
	}
	if inspection.State == "absent" {
		return "attached-case", inst, nil
	}
	return "mission-case", inst, nil
}

func initWriteSourceSHA256(plan InitPlan, write WriteResult) (string, error) {
	if write.Kind == "sync-state" {
		identity, err := initSyncStateIdentityForPlan(plan)
		if err != nil {
			return "", err
		}
		data, err := json.Marshal(identity)
		if err != nil {
			return "", err
		}
		return sha256Bytes(data), nil
	}
	if len(write.rawContent) > 0 {
		return sha256Bytes(write.rawContent), nil
	}
	if strings.TrimSpace(write.SourcePath) != "" {
		canonicalSources := initPublishesCanonicalText(plan.TargetClass) && write.Kind != "runtime-executable" && write.Kind != "pack-asset" && write.Kind != "common-asset" && write.Kind != "runtime-asset"
		data, err := os.ReadFile(write.SourcePath)
		if canonicalSources {
			data, err = sourceartifact.ReadCanonical(write.SourcePath)
		}
		if err != nil {
			return "", err
		}
		switch write.Kind {
		case "template-file":
			text := strings.ReplaceAll(string(data), "<PROJECT_NAME>", plan.ProjectName)
			text = strings.ReplaceAll(text, "<PROJECT_ROOT>", plan.CaseRoot)
			data = []byte(text)
		case "managed-block":
			if strings.TrimSpace(write.blockID) == "" {
				return "", fmt.Errorf("managed block init write omitted block id: %s", write.Path)
			}
			data = []byte(review.ApplyManagedBlock("", write.blockID, string(data)))
		}
		return sha256Bytes(data), nil
	}
	var data []byte
	switch write.Kind {
	case "instance-metadata":
		if strings.HasPrefix(filepath.ToSlash(write.Path), ".steamai/") {
			data = []byte(casebind.STeamAIInstanceText(plan.CaseRoot, plan.Pack, plan.ProjectName, runtimebundle.ManifestRel, plan.bundleManifestSHA256))
		} else {
			data = []byte(casebind.InstanceText(plan.CaseRoot, plan.RepoRoot, plan.Pack, plan.ProjectName))
		}
	case "legacy-metadata":
		data = []byte("templateRoot: " + plan.RepoRoot + "\r\n" +
			"rekitMode: case-local-shim\r\n" +
			"templatePack: " + plan.Pack + "\r\n" +
			"templateVersion: 0.0.0\r\n")
	default:
		generated := struct {
			Path        string `json:"path"`
			Kind        string `json:"kind"`
			CaseRoot    string `json:"caseRoot"`
			RepoRoot    string `json:"repoRoot"`
			Pack        string `json:"pack"`
			ProjectName string `json:"projectName"`
		}{write.Path, write.Kind, plan.CaseRoot, plan.RepoRoot, plan.Pack, plan.ProjectName}
		var err error
		data, err = json.Marshal(generated)
		if err != nil {
			return "", err
		}
	}
	return sha256Bytes(data), nil
}

func initSyncStateIdentityForPlan(plan InitPlan) (initSyncStateIdentity, error) {
	managed := map[string]syncManagedEntry{}
	for _, candidate := range plan.Writes {
		if candidate.Kind != "managed-file" {
			continue
		}
		hash := strings.TrimSpace(plan.initSourceSHA256[candidate.Path])
		if hash == "" {
			var err error
			hash, err = initWriteSourceSHA256(plan, candidate)
			if err != nil {
				return initSyncStateIdentity{}, err
			}
		}
		targetHash := hash
		if candidate.Action == "unchanged" {
			targetHash = strings.TrimSpace(plan.initTargetSHA256[candidate.Path])
			if targetHash == "" {
				return initSyncStateIdentity{}, fmt.Errorf("unchanged init target hash is missing: %s", candidate.Path)
			}
		}
		managed[candidate.Path] = syncManagedEntry{
			SourceHash:       hash,
			TargetHashAtSync: targetHash,
			LastAction:       "sync",
		}
	}
	templateRoot := plan.RepoRoot
	if plan.bundleManifestSHA256 != "" {
		templateRoot = "."
	}
	return initSyncStateIdentity{
		SchemaVersion: 1,
		TemplateRoot:  templateRoot,
		TemplatePack:  plan.Pack,
		Managed:       managed,
	}, nil
}

func initTargetBinding(path string) (bool, string, string, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, "missing", "", nil
	}
	if err != nil {
		return false, "", "", err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return true, "symlink", "", nil
	}
	if info.IsDir() {
		return true, "directory", "", nil
	}
	if !info.Mode().IsRegular() {
		return true, "other", "", nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false, "", "", err
	}
	post, err := os.Lstat(path)
	if err != nil || !post.Mode().IsRegular() || !os.SameFile(info, post) || post.Size() != int64(len(data)) {
		return false, "", "", fmt.Errorf("init target changed while hashing: %s", path)
	}
	return true, "regular-file", sha256Bytes(data), nil
}

func ordinaryInitAdoptionBlockers(plan InitPlan) []string {
	if plan.TargetClass != "ordinary-directory" {
		return nil
	}
	allowed := map[string]bool{
		"create": true, "create-managed-file": true, "create-local-template-file": true,
		"create-managed-block-host": true, "create-support-file": true,
		"unchanged": true, "skip-existing-local-file": true, "skip-existing-support-file": true,
	}
	blockers := []string{}
	for _, write := range plan.Writes {
		exists, _, _, err := initTargetBinding(write.TargetPath)
		if err != nil {
			blockers = append(blockers, write.Path+":unreadable-target")
			continue
		}
		preservesExisting := write.Action == "unchanged" || strings.HasPrefix(write.Action, "skip-existing-")
		if !allowed[write.Action] || (exists && !preservesExisting) {
			blockers = append(blockers, write.Path+":"+write.Action)
		}
	}
	sort.Strings(blockers)
	return blockers
}

func validInitPlanSHA256(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func sha256Bytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
