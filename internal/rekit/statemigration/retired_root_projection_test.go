package statemigration

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/casehealth"
	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/packidentity"
	"github.com/shuiyu486/re-context-kits/internal/rekit/sourceartifact"
)

func TestRetiredMigrationProjectsCanonicalRootStateAndHealth(t *testing.T) {
	for _, sourcePack := range packidentity.RetiredIDs() {
		t.Run(sourcePack, func(t *testing.T) {
			if !rekitfs.HandleBoundExactMutationSupported() {
				t.Skip("retired root projection Apply requires handle-bound exact mutation")
			}
			fixture := newRetiredMigrationFixture(t, sourcePack)
			m, err := manifest.Load(fixture.repoRoot, packidentity.Canonical)
			if err != nil {
				t.Fatal(err)
			}

			exactManaged := m.ManagedFiles[0]
			exactSource, err := m.SourcePath(exactManaged)
			if err != nil {
				t.Fatal(err)
			}
			exactBytes, err := sourceartifact.ReadCanonical(exactSource)
			if err != nil {
				t.Fatal(err)
			}
			writeFixtureFile(t, filepath.Join(fixture.caseRoot, filepath.FromSlash(exactManaged)), exactBytes)

			templateRel := strings.TrimSuffix(m.TemplateFiles[0], ".template.md") + ".md"
			localTemplate := []byte("# Local handoff\r\n\r\nPreserve this project-owned content.\r\n")
			writeFixtureFile(t, filepath.Join(fixture.caseRoot, filepath.FromSlash(templateRel)), localTemplate)
			localGitignore := []byte("custom-output/\r\n")
			writeFixtureFile(t, filepath.Join(fixture.caseRoot, ".gitignore"), localGitignore)

			prefix := []byte("# User-owned context\r\n\r\n")
			suffix := []byte("\r\n\r\n## User-owned suffix\r\nkeep me\r\n")
			retiredBlock := sourceartifact.CanonicalText([]byte(strings.TrimSpace(retiredRouterFixture(sourcePack))))
			writeFixtureFile(t, filepath.Join(fixture.caseRoot, "CLAUDE.local.md"), append(append(append([]byte{}, prefix...), retiredBlock...), suffix...))
			writeFixtureFile(t, filepath.Join(fixture.caseRoot, "references", sourcePack, "README.md"), []byte("retired local bytes\r\n"))
			writeRetiredStateManagedFixture(t, fixture)

			before := snapshotTree(t, fixture.caseRoot)
			plan, err := Preview(fixture.repoRoot, fixture.caseRoot, sourcePack)
			if err != nil {
				t.Fatal(err)
			}
			if len(plan.RootFiles) != len(m.ManagedFiles)+len(m.TemplateFiles)+2 {
				t.Fatalf("canonical root transition count = %d, want %d: %+v", len(plan.RootFiles), len(m.ManagedFiles)+len(m.TemplateFiles)+2, plan.RootFiles)
			}
			assertRootTransitionAction(t, plan.RootFiles, exactManaged, "unchanged")
			assertRootTransitionAction(t, plan.RootFiles, templateRel, "preserve-existing-template")
			assertRootTransitionAction(t, plan.RootFiles, "CLAUDE.local.md", "replace-managed-block")
			assertRootTransitionAction(t, plan.RootFiles, ".gitignore", "preserve-existing-support")
			if after := snapshotTree(t, fixture.caseRoot); !equalSnapshot(before, after) {
				t.Fatal("retired root projection preview mutated the project")
			}

			result, err := Apply(fixture.repoRoot, fixture.caseRoot, sourcePack, plan.ExpectedPlanSHA256)
			if err != nil {
				t.Fatal(err)
			}
			if result.Receipt == nil || len(result.Receipt.RootFiles) != len(plan.RootFiles) {
				t.Fatalf("migration receipt omitted root transitions: %+v", result.Receipt)
			}
			for _, rel := range m.ManagedFiles {
				path := filepath.Join(fixture.caseRoot, filepath.FromSlash(rel))
				if info, statErr := os.Lstat(path); statErr != nil || !info.Mode().IsRegular() {
					t.Fatalf("missing canonical managed file %s: info=%v err=%v transitions=%+v receipt=%+v", path, info, statErr, plan.RootFiles, result.Receipt)
				}
			}
			if got, err := os.ReadFile(filepath.Join(fixture.caseRoot, filepath.FromSlash(templateRel))); err != nil || !bytes.Equal(got, localTemplate) {
				t.Fatalf("local template was not preserved: %q err=%v", got, err)
			}
			if got, err := os.ReadFile(filepath.Join(fixture.caseRoot, ".gitignore")); err != nil || !bytes.Equal(got, localGitignore) {
				t.Fatalf("existing support file was not preserved: %q err=%v", got, err)
			}
			host, err := os.ReadFile(filepath.Join(fixture.caseRoot, "CLAUDE.local.md"))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.HasPrefix(host, prefix) || !bytes.HasSuffix(host, suffix) || bytes.Contains(host, []byte(retiredManagedBlockID(sourcePack))) || bytes.Count(host, []byte("<!-- BEGIN binary-re-template:router")) != 1 {
				t.Fatalf("managed block migration did not preserve user bytes or publish one canonical block:\n%s", host)
			}
			if got, err := os.ReadFile(filepath.Join(fixture.caseRoot, "references", sourcePack, "README.md")); err != nil || string(got) != "retired local bytes\r\n" {
				t.Fatalf("retired local path was removed or changed: %q err=%v", got, err)
			}

			stateBytes, err := os.ReadFile(filepath.Join(fixture.caseRoot, ".steamai", "state.json"))
			if err != nil {
				t.Fatal(err)
			}
			var state struct {
				TemplateRoot string `json:"templateRoot"`
				TemplatePack string `json:"templatePack"`
				LastSyncAt   string `json:"lastSyncAt"`
				Managed      map[string]struct {
					SourceHash       string `json:"sourceHash"`
					TargetHashAtSync string `json:"targetHashAtSync"`
					LastAction       string `json:"lastAction"`
				} `json:"managed"`
			}
			if err := json.Unmarshal(stateBytes, &state); err != nil {
				t.Fatal(err)
			}
			if state.TemplateRoot != "." || state.TemplatePack != packidentity.Canonical || state.LastSyncAt != "2026-08-14T00:00:00Z" {
				t.Fatalf("canonical state identity drifted: %+v", state)
			}
			if _, ok := state.Managed["references/"+sourcePack+"/README.md"]; ok {
				t.Fatal("retired sync-owned managed entry was retained")
			}
			if local, ok := state.Managed["local/custom.md"]; !ok || local.LastAction != "manual" || local.SourceHash != "local-source" {
				t.Fatalf("non-sync managed entry was not preserved: %+v", local)
			}
			for _, rel := range m.ManagedFiles {
				entry, ok := state.Managed[rel]
				transition, found := rootTransition(plan.RootFiles, rel)
				if !ok || !found || entry.LastAction != "sync" || entry.SourceHash != transition.AfterSHA256 || entry.TargetHashAtSync != transition.AfterSHA256 {
					t.Fatalf("canonical managed state differs for %s: entry=%+v transition=%+v", rel, entry, transition)
				}
			}
			if _, err := casehealth.Static(filepath.Join(fixture.caseRoot, ".steamai"), fixture.caseRoot, packidentity.Canonical); err != nil {
				t.Fatalf("migrated retired project is not canonical-static-healthy: %v", err)
			}
		})
	}
}

func TestRetiredRootProjectionReplacesTrustedOlderCanonicalManagedFile(t *testing.T) {
	if !rekitfs.HandleBoundExactMutationSupported() {
		t.Skip("exact managed-file replacement requires handle-bound mutation support")
	}
	fixture := newRetiredMigrationFixture(t, packidentity.RetiredVMP)
	const rel = "references/binary-re/README.md"
	oldBytes, err := os.ReadFile(filepath.Join("testdata", "vmp-re-readme-pre-cutover.md"))
	if err != nil {
		t.Fatal(err)
	}
	oldBytes = sourceartifact.CanonicalText(oldBytes)
	if got := sha256Hex(oldBytes); got != "dbb5dcf99256d2ef8fecd46ea573483299f6547c7359c39b222aa41bb1f08a70" {
		t.Fatalf("pre-cutover fixture hash = %s", got)
	}
	writeFixtureFile(t, filepath.Join(fixture.caseRoot, filepath.FromSlash(rel)), oldBytes)
	before := snapshotTree(t, fixture.caseRoot)

	plan, err := Preview(fixture.repoRoot, fixture.caseRoot, fixture.pack)
	if err != nil {
		t.Fatal(err)
	}
	transition, ok := rootTransition(plan.RootFiles, rel)
	if !ok || transition.Action != "replace-managed-file" || transition.BeforeSHA256 != sha256Hex(oldBytes) || transition.BeforeSize != int64(len(oldBytes)) {
		t.Fatalf("trusted old canonical transition = %+v", transition)
	}
	if after := snapshotTree(t, fixture.caseRoot); !equalSnapshot(before, after) {
		t.Fatal("trusted old canonical preview mutated the project")
	}

	result, err := Apply(fixture.repoRoot, fixture.caseRoot, fixture.pack, plan.ExpectedPlanSHA256)
	if err != nil {
		t.Fatal(err)
	}
	currentSource, err := os.ReadFile(filepath.Join(fixture.repoRoot, "packs", "binary-re", filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	currentBytes := sourceartifact.CanonicalText(currentSource)
	actual, err := os.ReadFile(filepath.Join(fixture.caseRoot, filepath.FromSlash(rel)))
	if err != nil || !bytes.Equal(actual, currentBytes) {
		t.Fatalf("trusted old canonical generation was not replaced exactly: err=%v", err)
	}
	receiptTransition, ok := rootTransition(result.Receipt.RootFiles, rel)
	if !ok || receiptTransition != transition || transition.AfterSHA256 != sha256Hex(currentBytes) || transition.SourceSHA256 != sha256Hex(currentBytes) {
		t.Fatalf("trusted replacement receipt differs from preview/source: preview=%+v receipt=%+v", transition, receiptTransition)
	}
	replay, err := Preview(fixture.repoRoot, fixture.caseRoot, fixture.pack)
	if err != nil || !replay.Replay || replay.ExpectedPlanSHA256 != plan.ExpectedPlanSHA256 {
		t.Fatalf("trusted replacement replay = %+v err=%v", replay, err)
	}
}

func TestRetiredRootProjectionRejectsUntrustedTargetsWithoutWriting(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*testing.T, migrationFixture)
		want  string
	}{
		{
			name: "managed file conflict",
			setup: func(t *testing.T, fixture migrationFixture) {
				writeFixtureFile(t, filepath.Join(fixture.caseRoot, "references", "binary-re", "README.md"), []byte("untrusted local replacement\n"))
			},
			want: "managed-file target differs from trusted canonical generations",
		},
		{
			name: "unknown managed block",
			setup: func(t *testing.T, fixture migrationFixture) {
				writeFixtureFile(t, filepath.Join(fixture.caseRoot, "CLAUDE.local.md"), []byte("<!-- BEGIN vmp-re-template:router v0.2.0 -->\nmutated\n<!-- END vmp-re-template:router -->\n"))
			},
			want: "retired managed block does not match an accepted generation",
		},
		{
			name: "empty local template",
			setup: func(t *testing.T, fixture migrationFixture) {
				writeFixtureFile(t, filepath.Join(fixture.caseRoot, "references", "binary-re", "task-handoff.md"), nil)
			},
			want: "existing template-file must be non-empty",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newRetiredMigrationFixture(t, packidentity.RetiredVMP)
			tc.setup(t, fixture)
			before := snapshotTree(t, fixture.caseRoot)
			if _, err := Preview(fixture.repoRoot, fixture.caseRoot, fixture.pack); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q, got %v", tc.want, err)
			}
			if after := snapshotTree(t, fixture.caseRoot); !equalSnapshot(before, after) {
				t.Fatal("rejected retired root projection mutated the project")
			}
		})
	}
}

func TestRetiredRootProjectionRejectsLateTargetDriftBeforeRename(t *testing.T) {
	if !rekitfs.HandleBoundExactMutationSupported() {
		t.Skip("late retired root drift hook follows the exact mutation capability gate")
	}
	fixture := newRetiredMigrationFixture(t, packidentity.RetiredGeneric)
	plan, err := Preview(fixture.repoRoot, fixture.caseRoot, fixture.pack)
	if err != nil {
		t.Fatal(err)
	}
	restore := SetBeforeCommitHookForTest(func() error {
		writeFixtureFile(t, filepath.Join(fixture.caseRoot, "references", "binary-re", "README.md"), []byte("late collision\n"))
		return nil
	})
	defer restore()
	if _, err := Apply(fixture.repoRoot, fixture.caseRoot, fixture.pack, plan.ExpectedPlanSHA256); err == nil || !strings.Contains(err.Error(), "root target changed immediately before migration commit") {
		t.Fatalf("expected late root drift rejection, got %v", err)
	}
	assertFile(t, filepath.Join(fixture.caseRoot, ".rekit", "instance.yml"))
	assertMissing(t, filepath.Join(fixture.caseRoot, ".steamai"))
}

func writeRetiredStateManagedFixture(t *testing.T, fixture migrationFixture) {
	t.Helper()
	path := filepath.Join(fixture.caseRoot, ".rekit", "state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var state map[string]any
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	state["managed"] = map[string]any{
		"references/" + fixture.pack + "/README.md": map[string]any{
			"sourceHash": "retired-source", "targetHashAtSync": "retired-target", "lastAction": "sync",
		},
		"local/custom.md": map[string]any{
			"sourceHash": "local-source", "targetHashAtSync": "local-target", "lastAction": "manual",
		},
	}
	data, err = json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertRootTransitionAction(t *testing.T, transitions []RootFileTransition, path, action string) {
	t.Helper()
	transition, ok := rootTransition(transitions, path)
	if !ok || transition.Action != action {
		t.Fatalf("root transition %s = %+v, want action %s", path, transition, action)
	}
}

func rootTransition(transitions []RootFileTransition, path string) (RootFileTransition, bool) {
	for _, transition := range transitions {
		if transition.Path == filepath.ToSlash(path) {
			return transition, true
		}
	}
	return RootFileTransition{}, false
}

func retiredManagedBlockID(pack string) string {
	if pack == packidentity.RetiredVMP {
		return "vmp-re-template:router"
	}
	return "generic-binary-re:router"
}

func retiredRouterFixture(pack string) string {
	if pack == packidentity.RetiredVMP {
		return retiredVMPRouterFixture
	}
	return retiredGenericRouterFixture
}

const retiredVMPRouterFixture = `<!-- BEGIN vmp-re-template:router v0.2.0 -->
## VMP RE 按需路由 / 渐进式披露

常驻只读本文件。遇到具体任务时，再读取 ` + "`references/vmp-re/README.md`" + ` 中对应文档。

| 任务 | 读取 |
|---|---|
| 新会话接手当前进度 | ` + "`references/vmp-re/task-handoff.md`" + ` |
| 复用到新的 VMProtect x64 样本 | ` + "`references/vmp-re/workflow-template.md`" + ` |
| 选择/复查工具链与止损条件 | ` + "`references/vmp-re/toolchain-router.md`" + ` |
| 控制上下文膨胀、子 agent 并行复核、避免 ` + "`/compact`" + ` / EOF 问题 | ` + "`references/vmp-re/progressive-disclosure.md`" + ` |
| 批量只读复核 / 子 agent 分片规划 | ` + "`references/vmp-re/progressive-disclosure.md`" + ` |
| 继续 singleton handler 复核 | ` + "`references/vmp-re/singleton-handler-review.md`" + ` |

规则：不要整段读取归档长文档；优先读 CSV、summary、当前任务文档和必要行范围。批量 handler/trace/value-flow 复核先过 delegation gate，由主 agent 或自动流程生成固定分片计划（必要时内部使用 ` + "`plan-subagents`" + `）或固定分片子 agent。每轮推进后同步 ` + "`task-handoff.md`" + `；发现可复用经验时通过 ` + "`/rekit promote`" + ` 的 review-first 流程回流模板，用户确认具体动作前不要写候选或 pack。
<!-- END vmp-re-template:router -->`

const retiredGenericRouterFixture = `<!-- BEGIN generic-binary-re:router -->
# Generic binary RE pack router

本 pack 用于授权通用二进制逆向、static triage、function behavior analysis、API behavior review 与 tooling sidecar review 场景的 Agent Team 工作流。它是多安全领域 pack 的最小骨架，当前重点是 route、ledger、handoff、tooling adapter 和 review-first 契约，不是自动逆向引擎、样本执行器、patch 平台或漏洞/恶意样本专项 pack 的替代品。

- 路由入口：` + "`references/generic-binary-re/README.md`" + `
- Agent Team route：` + "`references/generic-binary-re/agent-team.md`" + `
- 工作流：` + "`references/generic-binary-re/workflow-template.md`" + `
- 工具路由：` + "`references/generic-binary-re/toolchain-router.md`" + `

规则：

- 样本、完整二进制、dump、trace、memory、patch、完整函数体、符号表、IOC、hash、客户上下文和绝对路径留在 case-local workspace 或 sidecar，不写回 pack。
- 动态执行、调试、trace、dump、patch、批量反编译、自动重命名/注释、外部联网或写回分析数据库必须有隔离、预算和 stop condition，并先经 ` + "`/rekit gate`" + ` preflight；只有本次显式用户确认，或 strict durable autonomy profile + 对应 ` + "`authorized-gate`" + `，才允许 executor 执行。
- 子 agent 默认只读或仅写自己的 workspace；main agent 负责 ledger writeback、handoff、review merge 和 authority 确认。
<!-- END generic-binary-re:router -->`
