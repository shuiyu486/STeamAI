package vnextcontract

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTemplatesRenderCompleteCaseFixture(t *testing.T) {
	repo := repoRoot(t)
	caseRoot := t.TempDir()
	stateRoot := filepath.Join(caseRoot, ".steamai-vnext")

	replacements := map[string]string{
		"{{CASE_NAME}}":          "fixture-case",
		"{{GOAL}}":               "verify one synthetic security-research hypothesis",
		"{{AUTHORIZED_SCOPE}}":   "fixture files under this temporary directory",
		"{{PROHIBITED_ACTIONS}}": "network, injection, patching, or external side effects",
		"{{STOP_CONDITIONS}}":    "scope drift or user correction",
		"{{PACK_NAME}}":          "binary-re",
		"{{PACK_REVISION}}":      "test-fixture-revision",
		"{{PACK_SNAPSHOT_TREE}}": "test-fixture-tree",
		"{{TEAM_ROSTER}}":        "- static-analysis: owner of F-001\n- reviewer: independent evidence reviewer",
	}
	renderTemplate(t, repo, "vnext/templates/case/CLAUDE.md", filepath.Join(stateRoot, "CLAUDE.md"), replacements)

	memberReplacements := cloneReplacements(replacements)
	memberReplacements["{{MEMBER_NAME}}"] = "static-analysis"
	memberReplacements["{{ROLE}}"] = "analysis member"
	memberReplacements["{{RESPONSIBILITY}}"] = "bounded static evidence collection"
	memberReplacements["{{TASK_GOAL}}"] = "produce E-001 and draft F-001"
	memberReplacements["{{INPUTS}}"] = "../../artifacts/index.md alias fixture-primary"
	memberReplacements["{{ALLOWED_READS}}"] = "../../artifacts/index.md and case-local synthetic fixture"
	memberReplacements["{{ALLOWED_WRITES}}"] = "../../evidence/E-001.md and ../../findings/F-001.md"
	memberReplacements["{{DELIVERABLES}}"] = "../../evidence/E-001.md and ../../findings/F-001.md"
	memberReplacements["{{STOP_OR_ESCALATE}}"] = "requesting heavy action or scope expansion"
	memberReplacements["{{EXIT_CONDITIONS}}"] = "Reviewer accepts or requests bounded evidence"
	memberReplacements["{{ROLE_SPECIFIC_RULES}}"] = "do not modify the Reviewer's files"
	renderTemplate(t, repo, "vnext/templates/member/CLAUDE.md", filepath.Join(stateRoot, "members", "static-analysis", "CLAUDE.md"), memberReplacements)

	reviewerReplacements := cloneReplacements(memberReplacements)
	reviewerReplacements["{{MEMBER_NAME}}"] = "reviewer"
	reviewerReplacements["{{ROLE}}"] = "Reviewer"
	reviewerReplacements["{{RESPONSIBILITY}}"] = "independent review at explicit checkpoints"
	reviewerReplacements["{{TASK_GOAL}}"] = "review F-001 against E-001"
	reviewerReplacements["{{INPUTS}}"] = "../../findings/F-001.md and ../../evidence/E-001.md"
	reviewerReplacements["{{ALLOWED_READS}}"] = "../../artifacts/index.md, ../../evidence/E-001.md, and ../../findings/F-001.md"
	reviewerReplacements["{{ALLOWED_WRITES}}"] = "../../reviews/R-001.md only"
	reviewerReplacements["{{DELIVERABLES}}"] = "../../reviews/R-001.md"
	reviewerReplacements["{{ROLE_SPECIFIC_RULES}}"] = "read-only artifact/evidence/finding; no heavy action; write only ../../reviews/"
	renderTemplate(t, repo, "vnext/templates/member/CLAUDE.md", filepath.Join(stateRoot, "members", "reviewer", "CLAUDE.md"), reviewerReplacements)

	for _, rel := range []string{
		"artifacts/index.md",
		"evidence/E-001.md",
		"findings/F-001.md",
		"reviews/R-001.md",
		"learnings/candidates/L-001.md",
	} {
		template := "vnext/templates/research/" + map[string]string{
			"artifacts/index.md":            "artifact-index.md",
			"evidence/E-001.md":             "evidence.md",
			"findings/F-001.md":             "finding.md",
			"reviews/R-001.md":              "review.md",
			"learnings/candidates/L-001.md": "learning-candidate.md",
		}[rel]
		renderTemplate(t, repo, template, filepath.Join(stateRoot, filepath.FromSlash(rel)), researchFixtureReplacements())
	}

	for _, rel := range []string{
		"CLAUDE.md",
		"members/static-analysis/CLAUDE.md",
		"members/reviewer/CLAUDE.md",
		"artifacts/index.md",
		"evidence/E-001.md",
		"findings/F-001.md",
		"reviews/R-001.md",
		"learnings/candidates/L-001.md",
	} {
		data, err := os.ReadFile(filepath.Join(stateRoot, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("fixture omitted %s: %v", rel, err)
		}
		if strings.Contains(string(data), "{{") {
			t.Fatalf("fixture %s retained an unresolved placeholder", rel)
		}
	}

	analysisMemberPath := filepath.Join(stateRoot, "members", "static-analysis")
	assertMemberTaskPath(t, analysisMemberPath, stateRoot, "../../artifacts/index.md", "artifacts/index.md")
	assertMemberTaskPath(t, analysisMemberPath, stateRoot, "../../evidence/E-001.md", "evidence/E-001.md")
	assertMemberTaskPath(t, analysisMemberPath, stateRoot, "../../findings/F-001.md", "findings/F-001.md")

	reviewerText := readFixtureFile(t, stateRoot, "members/reviewer/CLAUDE.md")
	for _, required := range []string{
		"允许写入：`../../reviews/R-001.md only`",
		"read-only artifact/evidence/finding; no heavy action; write only ../../reviews/",
	} {
		if !strings.Contains(reviewerText, required) {
			t.Fatalf("Reviewer fixture omitted independent write boundary %q", required)
		}
	}
	if strings.Contains(reviewerText, "evidence/E-001.md and findings/F-001.md") {
		t.Fatal("Reviewer fixture inherited execution-member write scope")
	}
	reviewerPath := filepath.Join(stateRoot, "members", "reviewer")
	assertMemberTaskPath(t, reviewerPath, stateRoot, "../../reviews/R-001.md", "reviews/R-001.md")

	reviewText := readFixtureFile(t, stateRoot, "reviews/R-001.md")
	if !strings.Contains(reviewText, "Decision：`accepted`") || strings.Contains(reviewText, "accepted |") {
		t.Fatalf("review fixture retained a non-concrete decision: %s", reviewText)
	}
	learningText := readFixtureFile(t, stateRoot, "learnings/candidates/L-001.md")
	if !strings.Contains(learningText, "Kind：`method`") || strings.Contains(learningText, "method |") {
		t.Fatalf("learning fixture retained a non-concrete kind: %s", learningText)
	}

	memberPath := filepath.Join(stateRoot, "members", "static-analysis")
	if !pathWithin(memberPath, stateRoot) || !pathWithin(stateRoot, caseRoot) {
		t.Fatal("member directory is not nested below the shared case instruction root")
	}
}

func TestNativeCapabilityContractKeepsVisibleSessionDefault(t *testing.T) {
	repo := repoRoot(t)
	contract := readPrototypeFile(t, repo, "vnext/capabilities.md")
	for _, required := range []string{
		"普通、可见的 Claude Code 会话",
		"claude --add-dir <CASE_ROOT>",
		"claude agents --json --all",
		"claude logs <id>",
		"claude attach <id>",
		"不是 durable、exactly-once 消息队列",
		"无跨会话消息",
		"不得回退旧 Go runtime",
		"不把 session ID",
		"vnext/acceptance.md",
	} {
		assertContains(t, contract, required, "native capability contract")
	}
	acceptance := readPrototypeFile(t, repo, "vnext/acceptance.md")
	for _, required := range []string{
		"STEAMAI_VNEXT_LIVE_ACCEPTANCE=1",
		"至少两个真实独立 Claude Code session",
		"compare-before-update",
		"一名 owner 和最多一名 verifier",
		"3 名执行成员或 1 名 Reviewer",
		"needs-evidence",
		"claude --resume <session-id>",
		"跨会话 `SendMessage` 不能冒充 user/direct-session correction",
		"HOLD_STALE_TASK",
		"自动 probe 与 live acceptance 都通过",
		"不解析 transcript JSONL",
	} {
		assertContains(t, acceptance, required, "native live acceptance contract")
	}
}

func assertMemberTaskPath(t *testing.T, memberRoot, stateRoot, declared, wantRel string) {
	t.Helper()
	resolved := filepath.Clean(filepath.Join(memberRoot, filepath.FromSlash(declared)))
	want := filepath.Clean(filepath.Join(stateRoot, filepath.FromSlash(wantRel)))
	if resolved != want {
		t.Fatalf("member task path %q resolved to %s, want %s", declared, resolved, want)
	}
}

func readFixtureFile(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read fixture %s: %v", rel, err)
	}
	return string(data)
}

func renderTemplate(t *testing.T, repo, templateRel, target string, replacements map[string]string) {
	t.Helper()
	text := readPrototypeFile(t, repo, templateRel)
	for placeholder, value := range replacements {
		text = strings.ReplaceAll(text, placeholder, value)
	}
	if strings.Contains(text, "{{") {
		t.Fatalf("template %s lacks fixture replacement for %s", templateRel, unresolvedPlaceholder(text))
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func researchFixtureReplacements() map[string]string {
	return map[string]string{
		"{{ARTIFACT_ALIAS}}":                    "fixture-primary",
		"{{CASE_RELATIVE_PATH}}":                "fixtures/primary.bin",
		"{{SHA256}}":                            strings.Repeat("a", 64),
		"{{BYTES}}":                             "16",
		"{{SOURCE_NOTE}}":                       "synthetic fixture",
		"{{AUTHORIZED_USE}}":                    "read-only contract test",
		"{{LIMITATIONS}}":                       "synthetic evidence only",
		"{{EVIDENCE_ID}}":                       "E-001",
		"{{SUBJECT}}":                           "fixture behavior",
		"{{OWNER}}":                             "static-analysis",
		"{{METHOD}}":                            "bounded static inspection",
		"{{EVIDENCE_REF}}":                      "fixtures/summary.txt:1",
		"{{OBSERVATION}}":                       "synthetic marker is present",
		"{{CONFIDENCE}}":                        "high",
		"{{FINDING_ID}}":                        "F-001",
		"{{TITLE}}":                             "synthetic finding",
		"{{VERIFIER_OR_NONE}}":                  "reviewer",
		"{{CLAIM}}":                             "the fixture contains the documented marker",
		"{{UNPROVEN_OR_LIMITATIONS}}":           "no claim about real artifacts",
		"{{REVIEW_ID}}":                         "R-001",
		"{{REVIEWER}}":                          "reviewer",
		"{{REVIEW_SUMMARY}}":                    "E-001 supports the bounded claim",
		"{{RISKS_OR_GAPS}}":                     "synthetic-only scope",
		"{{NEXT_ACTION}}":                       "accept within fixture scope",
		"{{LEARNING_ID}}":                       "L-001",
		"{{DECISION}}":                          "accepted",
		"{{KIND}}":                              "method",
		"{{GENERAL_SCOPE}}":                     "template contract testing",
		"{{CASE_LOCAL_REFS}}":                   "F-001, R-001",
		"{{PACK_RELATIVE_PATH}}":                "references/testing/fixture-rule.md",
		"{{PACK_SNAPSHOT}}":                     "binary-re@test-fixture",
		"{{GENERALIZED_LESSON}}":                "keep claims bounded to cited evidence",
		"{{APPLICABILITY_AND_COUNTEREXAMPLES}}": "not applicable when evidence is missing",
	}
}

func cloneReplacements(source map[string]string) map[string]string {
	out := make(map[string]string, len(source))
	maps.Copy(out, source)
	return out
}

func unresolvedPlaceholder(text string) string {
	start := strings.Index(text, "{{")
	if start < 0 {
		return ""
	}
	end := strings.Index(text[start:], "}}")
	if end < 0 {
		return text[start:]
	}
	return text[start : start+end+2]
}

func pathWithin(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
