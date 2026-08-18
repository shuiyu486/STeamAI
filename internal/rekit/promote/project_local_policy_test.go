package promote

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/casebind"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtimebundle"
)

type promotePathAlias struct {
	name string
	path string
}

func availablePromoteDirectoryAliases(t *testing.T, parent, target, stem string) []promotePathAlias {
	t.Helper()
	aliases := []promotePathAlias{}
	symlink := filepath.Join(parent, stem+"-symlink")
	if err := os.Symlink(target, symlink); err == nil {
		aliases = append(aliases, promotePathAlias{name: "symlink-alias", path: symlink})
	}
	if runtime.GOOS == "windows" {
		junction := filepath.Join(parent, stem+"-junction")
		if _, err := exec.Command("cmd.exe", "/c", "mklink", "/J", junction, target).CombinedOutput(); err == nil {
			aliases = append(aliases, promotePathAlias{name: "junction-alias", path: junction})
		}
	}
	return aliases
}

func TestProjectLocalPolicyBlocksLegacyCallerRepoRootAliasesBeforeWrite(t *testing.T) {
	repoRoot, ownerCaseRoot, pack := projectLocalPromoteFixture(t)
	aliases := []promotePathAlias{
		{name: "direct", path: repoRoot},
		{name: "clean-path-alias", path: repoRoot + string(filepath.Separator) + "runtime" + string(filepath.Separator) + ".."},
	}
	aliases = append(aliases, availablePromoteDirectoryAliases(t, t.TempDir(), repoRoot, "project-bundle")...)

	for _, alias := range aliases {
		t.Run(alias.name, func(t *testing.T) {
			legacyCaseRoot := legacyPromoteCallerForRepo(t, alias.path, pack)
			ownerBefore := snapshotPromoteControlledTree(t, ownerCaseRoot)
			callerBefore := snapshotPromoteControlledTree(t, legacyCaseRoot)

			_, err := CreateCandidates(alias.path, legacyCaseRoot, pack, CandidateOptions{})
			if !errors.Is(err, ErrProjectLocalBundlePackMutation) {
				t.Fatalf("error = %v, want project-local bundle refusal", err)
			}
			if after := snapshotPromoteControlledTree(t, ownerCaseRoot); !reflect.DeepEqual(after, ownerBefore) {
				t.Fatalf("repoRoot alias refusal changed project-local owner tree\nbefore: %#v\nafter:  %#v", ownerBefore, after)
			}
			if after := snapshotPromoteControlledTree(t, legacyCaseRoot); !reflect.DeepEqual(after, callerBefore) {
				t.Fatalf("repoRoot alias refusal changed legacy caller tree\nbefore: %#v\nafter:  %#v", callerBefore, after)
			}
		})
	}
}

func TestProjectLocalBundlePackMutationsFailBeforeAnyWrite(t *testing.T) {
	repoRoot, caseRoot, pack := projectLocalPromoteFixture(t)
	before := snapshotPromoteControlledTree(t, caseRoot)

	tests := []struct {
		name string
		run  func() error
	}{
		{"Apply", func() error { _, err := Apply(repoRoot, caseRoot, pack, ApplyOptions{}); return err }},
		{"CreateCandidates", func() error { _, err := CreateCandidates(repoRoot, caseRoot, pack, CandidateOptions{}); return err }},
		{"ApplyCandidateDecisions", func() error {
			_, err := ApplyCandidateDecisions(repoRoot, caseRoot, pack, CandidateDecisionOptions{})
			return err
		}},
		{"DraftCandidateReviewProof", func() error {
			_, err := DraftCandidateReviewProof(repoRoot, caseRoot, pack, CandidateReviewProofDraftOptions{})
			return err
		}},
		{"DraftCandidateLifecycleProof", func() error {
			_, err := DraftCandidateLifecycleProof(repoRoot, caseRoot, pack, CandidateReviewProofDraftOptions{})
			return err
		}},
		{"ProvisionCandidateVerificationCases", func() error {
			_, err := ProvisionCandidateVerificationCases(repoRoot, caseRoot, pack, CandidateVerificationProvisionOptions{})
			return err
		}},
		{"RetireCandidateVerificationWorkspace", func() error {
			_, err := RetireCandidateVerificationWorkspace(repoRoot, caseRoot, pack, CandidateVerificationRetirementOptions{})
			return err
		}},
		{"VerifyCandidateDecision", func() error {
			_, err := VerifyCandidateDecision(repoRoot, caseRoot, pack, CandidateDecisionVerificationOptions{})
			return err
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run()
			if !errors.Is(err, ErrProjectLocalBundlePackMutation) {
				t.Fatalf("error = %v, want errors.Is(..., ErrProjectLocalBundlePackMutation)", err)
			}
			after := snapshotPromoteControlledTree(t, caseRoot)
			if !reflect.DeepEqual(after, before) {
				t.Fatalf("controlled tree changed before policy refusal\nbefore: %#v\nafter:  %#v", before, after)
			}
		})
	}

	for _, path := range []string{
		filepath.Join(repoRoot, "packs", pack, "promote-candidates"),
		filepath.Join(repoRoot, "packs", pack, "tooling", "candidates"),
	} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("policy refusal left candidate, lock, backup, or receipt directory %s: %v", path, err)
		}
	}
}

func TestDraftCandidateDecisionsRestrictsWritesToStateReviews(t *testing.T) {
	repoRoot, caseRoot, pack := promoteFixture(t)
	created, _, _ := candidateDecisionFixture(t, repoRoot, caseRoot, pack, "draft-state-reviews")
	stateRoot := filepath.Join(caseRoot, projectstate.LegacyDir)
	validPath := filepath.Join(stateRoot, "reviews", "draft-state-reviews", "decisions.json")
	base := CandidateDecisionDraftOptions{
		PacketPath:   created.ReviewWorkspace.PacketPath,
		Decision:     "accept-managed-reject-tooling",
		Reason:       "reviewed bounded candidate diff",
		Actor:        "mission-commander",
		EvidenceRefs: created.ReviewWorkspace.CombinedDiffPath,
		WhatIf:       true,
	}

	t.Run("valid legacy reviews preview remains zero-write", func(t *testing.T) {
		before := snapshotPromoteControlledTree(t, caseRoot)
		opt := base
		opt.DecisionPath = validPath
		preview, err := DraftCandidateDecisions(repoRoot, caseRoot, pack, opt)
		if err != nil {
			t.Fatal(err)
		}
		if preview.IsMutation || preview.Applied || preview.DecisionPath != validPath {
			t.Fatalf("unexpected valid reviews preview: %+v", preview)
		}
		if after := snapshotPromoteControlledTree(t, caseRoot); !reflect.DeepEqual(after, before) {
			t.Fatalf("valid WhatIf changed case tree\nbefore: %#v\nafter:  %#v", before, after)
		}
	})

	for _, tc := range []struct {
		name string
		path string
	}{
		{name: "state pack tree", path: filepath.Join(stateRoot, "packs", pack, "decisions.json")},
		{name: "case pack tree", path: filepath.Join(caseRoot, "packs", pack, "decisions.json")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := snapshotPromoteControlledTree(t, caseRoot)
			opt := base
			opt.DecisionPath = tc.path
			opt.WhatIf = false
			opt.ExpectedDecisionSHA256 = strings.Repeat("0", 64)
			if _, err := DraftCandidateDecisions(repoRoot, caseRoot, pack, opt); err == nil || !strings.Contains(err.Error(), "resolved state reviews namespace") {
				t.Fatalf("error = %v, want resolved reviews namespace refusal", err)
			}
			if after := snapshotPromoteControlledTree(t, caseRoot); !reflect.DeepEqual(after, before) {
				t.Fatalf("direct pack-path refusal changed case tree\nbefore: %#v\nafter:  %#v", before, after)
			}
		})
	}

	reviewsRoot := filepath.Join(stateRoot, "reviews")
	packRoot := filepath.Join(stateRoot, "packs")
	if err := os.MkdirAll(packRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	aliases := availablePromoteDirectoryAliases(t, reviewsRoot, packRoot, "pack-alias")
	if len(aliases) == 0 {
		t.Log("directory alias creation unavailable; direct pack-path negatives remain covered")
	}
	for _, alias := range aliases {
		t.Run("reviews "+alias.name+" to pack tree", func(t *testing.T) {
			before := snapshotPromoteControlledTree(t, caseRoot)
			opt := base
			opt.DecisionPath = filepath.Join(alias.path, "decisions.json")
			opt.WhatIf = false
			opt.ExpectedDecisionSHA256 = strings.Repeat("0", 64)
			if _, err := DraftCandidateDecisions(repoRoot, caseRoot, pack, opt); err == nil || (!strings.Contains(err.Error(), "symlink") && !strings.Contains(err.Error(), "reparse")) {
				t.Fatalf("error = %v, want alias refusal", err)
			}
			if after := snapshotPromoteControlledTree(t, caseRoot); !reflect.DeepEqual(after, before) {
				t.Fatalf("alias refusal changed case tree\nbefore: %#v\nafter:  %#v", before, after)
			}
			if entries, err := os.ReadDir(packRoot); err != nil || len(entries) != 0 {
				t.Fatalf("alias refusal wrote pack tree: entries=%v err=%v", entries, err)
			}
		})
	}
}

func TestProjectLocalBundleWhatIfAndNonPackWritersRemainAvailable(t *testing.T) {
	repoRoot, caseRoot, pack := projectLocalPromoteFixture(t)
	before := snapshotPromoteControlledTree(t, caseRoot)

	if _, err := Plan(repoRoot, caseRoot, pack); err != nil {
		t.Fatalf("Plan: %v", err)
	}
	applyPreview, err := Apply(repoRoot, caseRoot, pack, ApplyOptions{WhatIf: true})
	if err != nil {
		t.Fatalf("Apply WhatIf: %v", err)
	}
	if applyPreview.IsMutation || applyPreview.Applied {
		t.Fatalf("Apply WhatIf reported mutation: %+v", applyPreview)
	}
	candidatePreview, err := CreateCandidates(repoRoot, caseRoot, pack, CandidateOptions{WhatIf: true})
	if err != nil {
		t.Fatalf("CreateCandidates WhatIf: %v", err)
	}
	if candidatePreview.IsMutation || candidatePreview.Applied {
		t.Fatalf("CreateCandidates WhatIf reported mutation: %+v", candidatePreview)
	}
	if after := snapshotPromoteControlledTree(t, caseRoot); !reflect.DeepEqual(after, before) {
		t.Fatalf("zero-write previews changed controlled tree\nbefore: %#v\nafter:  %#v", before, after)
	}

	workspaceRoot := filepath.Join(caseRoot, ".steamai", "reviews", "project-local-policy")
	workspaceResult, err := WriteCandidateReviewWorkspace(candidatePreview, CandidateArtifactOptions{ReviewOutputDir: workspaceRoot})
	if err != nil {
		t.Fatalf("WriteCandidateReviewWorkspace: %v", err)
	}
	if workspaceResult.ReviewWorkspace == nil {
		t.Fatal("WriteCandidateReviewWorkspace omitted its case-local workspace")
	}
	if _, err := DraftCandidateDecisions(repoRoot, caseRoot, pack, CandidateDecisionDraftOptions{WhatIf: true}); errors.Is(err, ErrProjectLocalBundlePackMutation) {
		t.Fatalf("DraftCandidateDecisions was incorrectly treated as a pack-tree mutation: %v", err)
	}
	if _, err := StageMemberOutput(repoRoot, caseRoot, pack, MemberOutputStagingOptions{WhatIf: true}); errors.Is(err, ErrProjectLocalBundlePackMutation) {
		t.Fatalf("StageMemberOutput was incorrectly treated as a pack-tree mutation: %v", err)
	}
}

func TestProjectLocalPolicyDoesNotBlockCentralSourceRepos(t *testing.T) {
	t.Run("legacy source case", func(t *testing.T) {
		repoRoot, caseRoot, pack := promoteFixture(t)
		if err := refuseProjectLocalBundlePackMutation(repoRoot, caseRoot); err != nil {
			t.Fatalf("legacy central source policy check: %v", err)
		}
		result, err := CreateCandidates(repoRoot, caseRoot, pack, CandidateOptions{})
		if err != nil {
			t.Fatalf("central CreateCandidates: %v", err)
		}
		if !result.IsMutation || !result.Applied || result.Created == 0 {
			t.Fatalf("central CreateCandidates behavior regressed: %+v", result)
		}
	})

	t.Run("current state case using central source", func(t *testing.T) {
		repoRoot, caseRoot, pack := promoteFixture(t)
		if err := os.Rename(filepath.Join(caseRoot, projectstate.LegacyDir), filepath.Join(caseRoot, projectstate.CurrentDir)); err != nil {
			t.Fatal(err)
		}
		if err := refuseProjectLocalBundlePackMutation(repoRoot, caseRoot); err != nil {
			t.Fatalf("central source policy check: %v", err)
		}
		result, err := CreateCandidates(repoRoot, caseRoot, pack, CandidateOptions{})
		if err != nil {
			t.Fatalf("central CreateCandidates from current state case: %v", err)
		}
		if !result.IsMutation || !result.Applied || result.Created == 0 {
			t.Fatalf("central source behavior regressed: %+v", result)
		}
	})
}

func TestDraftCandidateDecisionsAllowsCurrentStateReviews(t *testing.T) {
	repoRoot, caseRoot, pack := promoteFixture(t)
	if err := os.Rename(filepath.Join(caseRoot, projectstate.LegacyDir), filepath.Join(caseRoot, projectstate.CurrentDir)); err != nil {
		t.Fatal(err)
	}
	created, err := CreateCandidates(repoRoot, caseRoot, pack, CandidateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	workspaceRoot := filepath.Join(caseRoot, projectstate.CurrentDir, "reviews", "current-decision-draft")
	created, err = WriteCandidateReviewWorkspace(created, CandidateArtifactOptions{ReviewOutputDir: workspaceRoot})
	if err != nil {
		t.Fatal(err)
	}
	decisionPath := filepath.Join(workspaceRoot, "decisions.json")
	opt := CandidateDecisionDraftOptions{
		PacketPath:   created.ReviewWorkspace.PacketPath,
		DecisionPath: decisionPath,
		Decision:     "accept-managed-reject-tooling",
		Reason:       "reviewed bounded candidate diff",
		Actor:        "mission-commander",
		EvidenceRefs: created.ReviewWorkspace.CombinedDiffPath,
		WhatIf:       true,
	}
	preview, err := DraftCandidateDecisions(repoRoot, caseRoot, pack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if preview.IsMutation || preview.Applied || preview.DecisionPath != decisionPath {
		t.Fatalf("unexpected current reviews preview: %+v", preview)
	}
	if _, err := os.Lstat(decisionPath); !os.IsNotExist(err) {
		t.Fatalf("current reviews WhatIf wrote decision file: %v", err)
	}

	before := snapshotPromoteControlledTree(t, caseRoot)
	packOpt := opt
	packOpt.DecisionPath = filepath.Join(caseRoot, projectstate.CurrentDir, "packs", pack, "decisions.json")
	packOpt.WhatIf = false
	packOpt.ExpectedDecisionSHA256 = strings.Repeat("0", 64)
	if _, err := DraftCandidateDecisions(repoRoot, caseRoot, pack, packOpt); err == nil || !strings.Contains(err.Error(), "resolved state reviews namespace") {
		t.Fatalf("current pack-tree draft error = %v, want reviews namespace refusal", err)
	}
	if after := snapshotPromoteControlledTree(t, caseRoot); !reflect.DeepEqual(after, before) {
		t.Fatalf("current pack-tree refusal changed case tree\nbefore: %#v\nafter:  %#v", before, after)
	}

	opt.WhatIf = false
	opt.ExpectedDecisionSHA256 = preview.DecisionSHA256
	applied, err := DraftCandidateDecisions(repoRoot, caseRoot, pack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if !applied.IsMutation || !applied.Applied || applied.AlreadyWritten {
		t.Fatalf("unexpected current reviews Apply: %+v", applied)
	}
	if info, err := os.Lstat(decisionPath); err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("current reviews draft was not a regular file: %v", err)
	}
}

func TestProjectLocalPolicyFailsClosedWhenPhysicalBindingIsUnavailable(t *testing.T) {
	repoRoot, caseRoot, pack := projectLocalPromoteFixture(t)
	if err := os.Rename(filepath.Join(repoRoot, "runtime"), filepath.Join(repoRoot, "runtime-unbound")); err != nil {
		t.Fatal(err)
	}
	before := snapshotPromoteControlledTree(t, caseRoot)

	_, err := CreateCandidates(repoRoot, caseRoot, pack, CandidateOptions{})
	if !errors.Is(err, ErrProjectLocalBundlePackMutation) || !strings.Contains(err.Error(), "cannot confirm physical binding") {
		t.Fatalf("error = %v, want physical-binding fail-closed sentinel", err)
	}
	if after := snapshotPromoteControlledTree(t, caseRoot); !reflect.DeepEqual(after, before) {
		t.Fatalf("unavailable physical binding changed controlled tree\nbefore: %#v\nafter:  %#v", before, after)
	}
}

func legacyPromoteCallerForRepo(t *testing.T, repoRoot, pack string) string {
	t.Helper()
	caseRoot := filepath.Join(t.TempDir(), "legacy-caller")
	writeText(t, filepath.Join(caseRoot, ".rekit", "instance.yml"),
		"templateRoot: "+repoRoot+"\n"+
			"templatePack: "+pack+"\n"+
			"projectName: legacy-caller\n"+
			"projectRoot: "+caseRoot+"\n")
	return caseRoot
}

func projectLocalPromoteFixture(t *testing.T) (repoRoot, caseRoot, pack string) {
	t.Helper()
	centralRoot, sourceCase, pack := promoteFixture(t)
	for path, text := range map[string]string{
		"common/policies/manifest.yml":             "policies: []\n",
		"common/policies/README.md":                "# Policies\n",
		"rekit/templates/steamai-project/SKILL.md": "# STeamAI\n",
		"rekit/schemas/instance.schema.yml":        "schemaVersion: 1\n",
		"rekit/schemas/pack-manifest.schema.yml":   "schemaVersion: 1\n",
		"rekit/tests/catalog.json":                 "{}\n",
	} {
		writeText(t, filepath.Join(centralRoot, filepath.FromSlash(path)), text)
	}
	executable := filepath.Join(t.TempDir(), runtimebundle.ExecutableName())
	writeText(t, executable, "fixture executable\n")
	if err := os.Chmod(executable, 0o755); err != nil {
		t.Fatal(err)
	}

	caseRoot = filepath.Join(t.TempDir(), "current-project")
	for _, rel := range []string{
		"references/template/README.md",
		"references/template/workflow-template.md",
		"references/template/toolchain-router.md",
	} {
		data, err := os.ReadFile(filepath.Join(sourceCase, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		writeText(t, filepath.Join(caseRoot, filepath.FromSlash(rel)), string(data))
	}
	bundle, err := runtimebundle.PublishForTest(caseRoot, centralRoot, pack, executable)
	if err != nil {
		t.Fatal(err)
	}
	repoRoot = filepath.Join(caseRoot, ".steamai")
	writeText(t, filepath.Join(repoRoot, "instance.yml"), casebind.STeamAIInstanceText(caseRoot, pack, "current-project", runtimebundle.ManifestRel, bundle.ManifestSHA256))
	if _, err := runtimebundle.Validate(repoRoot, runtimebundle.ManifestRel, bundle.ManifestSHA256, pack); err != nil {
		t.Fatalf("project-local fixture is not an exact verified bundle: %v", err)
	}
	return repoRoot, caseRoot, pack
}

type promoteTreeSnapshotEntry struct {
	Mode   fs.FileMode
	Bytes  []byte
	Target string
}

func snapshotPromoteControlledTree(t *testing.T, root string) map[string]promoteTreeSnapshotEntry {
	t.Helper()
	snapshot := map[string]promoteTreeSnapshotEntry{}
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		item := promoteTreeSnapshotEntry{Mode: info.Mode()}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			item.Target, err = os.Readlink(path)
		case info.Mode().IsRegular():
			item.Bytes, err = os.ReadFile(path)
		}
		if err != nil {
			return err
		}
		snapshot[filepath.ToSlash(rel)] = item
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return snapshot
}
