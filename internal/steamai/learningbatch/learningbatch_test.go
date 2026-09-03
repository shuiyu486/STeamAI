package learningbatch

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/shuiyu486/STeamAI/internal/steamai/casebootstrap"
)

func TestMultiCandidateMultiTargetPreviewAndApply(t *testing.T) {
	fixture := newBatchFixture(t)
	beforeHead := runGit(t, fixture.git, fixture.source, "rev-parse", "HEAD")
	beforeIndex := runGit(t, fixture.git, fixture.source, "write-tree")
	beforeSnapshot := mustCurrent(t, fixture.caseRoot).PayloadDigest

	preview, err := BuildPreview(fixture.git, fixture.source, fixture.caseRoot, fixture.request)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Candidates) != 3 || len(preview.Targets) != 2 {
		t.Fatalf("unexpected batch shape: %d candidates, %d targets", len(preview.Candidates), len(preview.Targets))
	}
	if !strings.Contains(preview.HumanPreview, "diff --git a/packs/fixture-pack/method-a.md") ||
		!strings.Contains(preview.HumanPreview, "source-finding:findings/F-001.md sha256:") ||
		!strings.Contains(preview.HumanPreview, "reviewer:reviewer") ||
		!strings.Contains(preview.HumanPreview, ConfirmationPrefix+preview.Identity) {
		t.Fatal("preview 未展示完整 patch、source chain、Reviewer 或 exact confirmation")
	}
	for _, target := range preview.Targets {
		data := mustRead(t, filepath.Join(fixture.source, filepath.FromSlash(target.Path)))
		if hashBytes(data) != target.PreSHA256 {
			t.Fatal("preview 在确认前修改了 canonical target")
		}
	}
	if _, err := Apply(fixture.git, fixture.source, fixture.caseRoot, fixture.request, "确认"); err != ErrConfirmationRequired {
		t.Fatalf("non-exact confirmation returned %v", err)
	}
	if _, err := Apply(fixture.git, fixture.source, fixture.caseRoot, fixture.request, ConfirmationPrefix+preview.Identity); err != nil {
		t.Fatal(err)
	}
	for _, target := range preview.Targets {
		data := mustRead(t, filepath.Join(fixture.source, filepath.FromSlash(target.Path)))
		if hashBytes(data) != target.PostSHA256 {
			t.Fatalf("postimage mismatch: %s", target.Path)
		}
	}
	if got := runGit(t, fixture.git, fixture.source, "rev-parse", "HEAD"); got != beforeHead {
		t.Fatal("apply 修改了 HEAD")
	}
	if got := runGit(t, fixture.git, fixture.source, "write-tree"); got != beforeIndex {
		t.Fatal("apply 修改了 index")
	}
	if got := mustCurrent(t, fixture.caseRoot).PayloadDigest; got != beforeSnapshot {
		t.Fatal("apply 修改了 case snapshot")
	}
}

func TestPatchScopeRequiresFullIndexCurrentPreimage(t *testing.T) {
	fixture := newBatchFixture(t)
	patchPath := filepath.Join(fixture.caseRoot, ".steamai-vnext", filepath.FromSlash(fixture.request.Patch))
	patch := string(mustRead(t, patchPath))
	lines := strings.Split(patch, "\n")
	for index, line := range lines {
		if strings.HasPrefix(line, "index ") {
			parts := strings.Fields(line)
			ids := strings.Split(parts[1], "..")
			lines[index] = "index " + ids[0][:7] + ".." + ids[1][:7] + " " + parts[2]
			break
		}
	}
	writeFile(t, patchPath, []byte(strings.Join(lines, "\n")))
	if _, err := BuildPreview(fixture.git, fixture.source, fixture.caseRoot, fixture.request); err == nil {
		t.Fatal("缺少 full-index identity 的 patch 未被拒绝")
	}
}

func TestPatchScopeRejectsAbbreviatedNewBlob(t *testing.T) {
	fixture := newBatchFixture(t)
	patchPath := filepath.Join(fixture.caseRoot, ".steamai-vnext", filepath.FromSlash(fixture.request.Patch))
	lines := strings.Split(string(mustRead(t, patchPath)), "\n")
	for index, line := range lines {
		if strings.HasPrefix(line, "index ") {
			parts := strings.Fields(line)
			ids := strings.Split(parts[1], "..")
			lines[index] = "index " + ids[0] + ".." + ids[1][:7] + " " + parts[2]
			break
		}
	}
	writeFile(t, patchPath, []byte(strings.Join(lines, "\n")))
	if _, err := BuildPreview(fixture.git, fixture.source, fixture.caseRoot, fixture.request); err == nil {
		t.Fatal("缩写 new blob identity 的 patch 未被拒绝")
	}
}

func TestApplyUsesConfirmedPatchBytes(t *testing.T) {
	fixture := newBatchFixture(t)
	preview, err := BuildPreview(fixture.git, fixture.source, fixture.caseRoot, fixture.request)
	if err != nil {
		t.Fatal(err)
	}
	patchPath := filepath.Join(fixture.caseRoot, ".steamai-vnext", filepath.FromSlash(fixture.request.Patch))
	writeFile(t, patchPath, append(mustRead(t, patchPath), []byte("\n# replaced after preview\n")...))
	if err := applyPreview(fixture.git, fixture.source, fixture.caseRoot, preview); err != nil {
		t.Fatal(err)
	}
	for _, target := range preview.Targets {
		data := mustRead(t, filepath.Join(fixture.source, filepath.FromSlash(target.Path)))
		if hashBytes(data) != target.PostSHA256 {
			t.Fatalf("confirmed patch postimage mismatch: %s", target.Path)
		}
	}
}

func TestApplyRejectsPreviewBaselineDrift(t *testing.T) {
	fixture := newBatchFixture(t)
	preview, err := BuildPreview(fixture.git, fixture.source, fixture.caseRoot, fixture.request)
	if err != nil {
		t.Fatal(err)
	}
	preview.CanonicalHead = strings.Repeat("f", 40)
	if err := applyPreview(fixture.git, fixture.source, fixture.caseRoot, preview); err == nil {
		t.Fatal("canonical HEAD 与 preview 不匹配时仍执行 Apply")
	}
	preview.CanonicalHead = runGit(t, fixture.git, fixture.source, "rev-parse", "HEAD")
	preview.SnapshotDigest = "sha256:" + strings.Repeat("e", 64)
	if err := applyPreview(fixture.git, fixture.source, fixture.caseRoot, preview); err == nil {
		t.Fatal("case snapshot 与 preview 不匹配时仍执行 Apply")
	}
}

func TestBatchRejectsDriftAndIneligibleReview(t *testing.T) {
	t.Run("target drift", func(t *testing.T) {
		fixture := newBatchFixture(t)
		preview, err := BuildPreview(fixture.git, fixture.source, fixture.caseRoot, fixture.request)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(fixture.source, "packs", "fixture-pack", "method-a.md")
		writeFile(t, path, append(mustRead(t, path), []byte("concurrent\n")...))
		if _, err := Apply(fixture.git, fixture.source, fixture.caseRoot, fixture.request, ConfirmationPrefix+preview.Identity); err == nil {
			t.Fatal("target drift 未使旧确认失效")
		}
	})
	t.Run("review decision", func(t *testing.T) {
		fixture := newBatchFixture(t)
		path := filepath.Join(fixture.caseRoot, ".steamai-vnext", "reviews", "R-L-001.md")
		data := strings.Replace(string(mustRead(t, path)), "Decision：`eligible`", "Decision：`ineligible`", 1)
		writeFile(t, path, []byte(data))
		if _, err := BuildPreview(fixture.git, fixture.source, fixture.caseRoot, fixture.request); err == nil {
			t.Fatal("ineligible candidate review 未被拒绝")
		}
	})
	t.Run("missing reviewer checks", func(t *testing.T) {
		fixture := newBatchFixture(t)
		path := filepath.Join(fixture.caseRoot, ".steamai-vnext", "reviews", "R-L-001.md")
		data := strings.Replace(string(mustRead(t, path)), "- Evidence/generalization：`pass`\n", "", 1)
		writeFile(t, path, []byte(data))
		if _, err := BuildPreview(fixture.git, fixture.source, fixture.caseRoot, fixture.request); err == nil {
			t.Fatal("缺少 eligibility 检查结果的 review 未被拒绝")
		}
	})
	t.Run("reviewer not in roster", func(t *testing.T) {
		fixture := newBatchFixture(t)
		path := filepath.Join(fixture.caseRoot, ".steamai-vnext", "reviews", "R-L-001.md")
		data := strings.Replace(string(mustRead(t, path)), "Reviewer 单写者：`reviewer`", "Reviewer 单写者：`other`", 1)
		writeFile(t, path, []byte(data))
		if _, err := BuildPreview(fixture.git, fixture.source, fixture.caseRoot, fixture.request); err == nil {
			t.Fatal("roster 外 Reviewer 未被拒绝")
		}
	})
	t.Run("case inside canonical source", func(t *testing.T) {
		fixture := newBatchFixture(t)
		inside := filepath.Join(fixture.source, "nested-case")
		if err := os.Rename(fixture.caseRoot, inside); err != nil {
			t.Fatal(err)
		}
		if _, err := BuildPreview(fixture.git, fixture.source, inside, fixture.request); err == nil {
			t.Fatal("canonical source 内部 case 未被拒绝")
		}
	})
}

type batchFixture struct {
	git, source, caseRoot string
	request               Request
}

func newBatchFixture(t *testing.T) batchFixture {
	t.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is required")
	}
	root := t.TempDir()
	source := filepath.Join(root, "canonical")
	files := map[string]string{
		".claude/skills/steamai/SKILL.md":          "# Fixture skill\n",
		"vnext/learning-feedback.md":               "# Learning contract\n",
		"vnext/templates/case/CLAUDE.md":           "# {{CASE_NAME}}\n- Case 名称：`{{CASE_NAME}}`\n- 研究目标：`{{GOAL}}`\n- 授权范围：`{{AUTHORIZED_SCOPE}}`\n- 禁止事项：`{{PROHIBITED_ACTIONS}}`\n- 全局停止条件：`{{STOP_CONDITIONS}}`\n- Selected pack：`{{PACK_NAME}}`\n- Source revision：`{{PACK_REVISION}}`\n- Pack tree：`{{PACK_SNAPSHOT_TREE}}`\n- Common tree：`{{COMMON_SNAPSHOT_TREE}}`\n- Snapshot digest：`{{SNAPSHOT_DIGEST}}`\n\n| Member | Kind | Durable state | Member source |\n|---|---|---|---|\n{{TEAM_ROSTER_ROWS}}\n",
		"vnext/templates/member/CLAUDE.md":         "# {{MEMBER_NAME}}\n{{ROLE}}\n{{RESPONSIBILITY}}\n{{TASK_GOAL}}\n{{INPUTS}}\n{{ALLOWED_READS}}\n{{ALLOWED_WRITES}}\n{{DELIVERABLES}}\n{{STOP_OR_ESCALATE}}\n{{EXIT_CONDITIONS}}\n{{ROLE_SPECIFIC_RULES}}\n",
		"vnext/templates/roles/analysis-member.md": "# Analysis\n",
		"vnext/templates/roles/reviewer.md":        "# Reviewer\n",
		"vnext/templates/research/evidence.md":     "# Evidence\n",
		"packs/fixture-pack/manifest.yml":          "schemaVersion: 2\nname: fixture-pack\nentrypoints:\n  router: router.md\nlearningTargets:\n  - method-*.md\ndenyPatterns:\n  - forbidden-marker\n",
		"packs/fixture-pack/router.md":             "# Router\n",
		"packs/fixture-pack/method-a.md":           "# Method A\n\n- Existing rule.\n",
		"packs/fixture-pack/method-b.md":           "# Method B\n\n- Existing rule.\n",
		"common/policy.md":                         "# Policy\n",
	}
	for rel, text := range files {
		writeFile(t, filepath.Join(source, filepath.FromSlash(rel)), []byte(text))
	}
	runGit(t, git, source, "init", "--quiet")
	runGit(t, git, source, "config", "user.name", "STeamAI fixture")
	runGit(t, git, source, "config", "user.email", "fixture@example.invalid")
	runGit(t, git, source, "add", "--", ".")
	runGit(t, git, source, "commit", "--quiet", "-m", "fixture")
	caseRoot := filepath.Join(root, "case")
	if err := os.MkdirAll(caseRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	facts := casebootstrap.Facts{
		Name: "synthetic-case", Goal: "verify learning batch", Authorization: "temporary fixture files only",
		Prohibited: "network or real artifacts", Stop: "scope drift", Pack: "fixture-pack",
		Members: []casebootstrap.MemberFacts{{
			Name: "reviewer", Kind: "reviewer", Role: "Reviewer", Responsibility: "review fixture evidence",
			TaskGoal: "review learning candidates and batch", Inputs: "../../findings/F-001.md",
			AllowedReads: "../../findings/F-001.md", AllowedWrites: "../../reviews/R-fixture.md",
			Deliverables: "../../reviews/R-fixture.md", StopOrEscalate: "scope drift", ExitConditions: "review complete",
		}},
	}
	fresh, err := casebootstrap.BuildPreview(git, source, caseRoot, facts)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := casebootstrap.Apply(git, source, caseRoot, facts, casebootstrap.ConfirmationPrefix+fresh.Identity); err != nil {
		t.Fatal(err)
	}
	identity := mustCurrent(t, caseRoot)

	artifact := []byte("synthetic fixture observation")
	artifactRel := "fixtures/primary.bin"
	writeFile(t, filepath.Join(caseRoot, filepath.FromSlash(artifactRel)), artifact)
	artifactSHA := hashBytes(artifact)
	index := fmt.Sprintf("# Artifact Index\n\n## `fixture-primary`\n\n- 相对路径：`%s`\n- SHA-256：`%s`\n- Bytes：`%d`\n- 来源说明：`synthetic fixture`\n- 授权范围：`read-only contract test`\n- 备注：`none`\n", artifactRel, artifactSHA, len(artifact))
	writeFile(t, filepath.Join(caseRoot, ".steamai-vnext", "artifacts", "index.md"), []byte(index))
	evidenceRel := "evidence/E-001.md"
	evidence := fmt.Sprintf("# E-001\n\n- Artifact alias：`fixture-primary`\n- Artifact path：`%s`\n- Artifact SHA-256：`%s`\n- Artifact bytes：`%d`\n- Authorized use：`read-only contract test`\n", artifactRel, artifactSHA, len(artifact))
	writeFile(t, filepath.Join(caseRoot, ".steamai-vnext", filepath.FromSlash(evidenceRel)), []byte(evidence))
	findingRel := "findings/F-001.md"
	finding := []byte("# F-001\n\n## Evidence\n\n- `../evidence/E-001.md`\n")
	writeFile(t, filepath.Join(caseRoot, ".steamai-vnext", filepath.FromSlash(findingRel)), finding)
	sourceReviewRel := "reviews/R-001.md"
	sourceReview := fmt.Sprintf("# R-001\n\n## Review round `1`\n\n- Previous round：`none`\n- Reviewer：`reviewer`\n- Finding：`../findings/F-001.md`\n- Finding SHA-256：`%s`\n- Decision：`accepted`\n- Confidence：`high`\n\n### 判断\n\nSynthetic.\n\n### 检查的证据\n\n- ../evidence/E-001.md — %s\n\n### 风险或缺口\n\nFixture only.\n", hashBytes(finding), hashBytes([]byte(evidence)))
	writeFile(t, filepath.Join(caseRoot, ".steamai-vnext", filepath.FromSlash(sourceReviewRel)), []byte(sourceReview))

	candidateSpecs := []struct{ id, target, lesson string }{
		{"001", "packs/fixture-pack/method-a.md", "Record counterexamples."},
		{"002", "packs/fixture-pack/method-a.md", "State evidence limits."},
		{"003", "packs/fixture-pack/method-b.md", "Keep verification reproducible."},
	}
	request := Request{Patch: "learnings/patches/LB-001.patch", BatchReview: "reviews/R-LB-001.md"}
	var candidateRecords []CandidateRecord
	for _, spec := range candidateSpecs {
		candidateRel := "learnings/candidates/L-" + spec.id + ".md"
		reviewRel := "reviews/R-L-" + spec.id + ".md"
		candidate := renderCandidate(identity, findingRel, hashBytes(finding), sourceReviewRel, hashBytes([]byte(sourceReview)), spec.target, spec.lesson)
		candidateSHA := hashBytes([]byte(candidate))
		review := fmt.Sprintf("# R-L-%s\n\n- Reviewer 单写者：`reviewer`\n- Candidate：`%s`\n- Candidate SHA-256：`%s`\n- Source finding：`%s`\n- Source finding SHA-256：`%s`\n- Source accepted review：`%s`\n- Source review SHA-256：`%s`\n- Selected pack：`%s`\n- Source revision：`%s`\n- Pack tree：`%s`\n- Common tree：`%s`\n- Snapshot digest：`%s`\n- Proposed destination：`%s`\n\n## Checkpoint A — Eligibility\n\n- Decision：`eligible`\n- Evidence/generalization：`pass`\n- Applicability/counterexamples：`pass`\n- Dedup/conflict：`pass`\n- Redaction/denyPatterns：`pass`\n- Target allowlist/currentness：`pass`\n", spec.id, candidateRel, candidateSHA, findingRel, hashBytes(finding), sourceReviewRel, hashBytes([]byte(sourceReview)), identity.Pack, identity.Revision, identity.PackTree, identity.CommonTree, identity.PayloadDigest, spec.target)
		writeFile(t, filepath.Join(caseRoot, ".steamai-vnext", filepath.FromSlash(candidateRel)), []byte(candidate))
		writeFile(t, filepath.Join(caseRoot, ".steamai-vnext", filepath.FromSlash(reviewRel)), []byte(review))
		request.CandidateReviews = append(request.CandidateReviews, CandidateReviewRef{Candidate: candidateRel, Review: reviewRel})
		candidateRecords = append(candidateRecords, CandidateRecord{CandidatePath: candidateRel, CandidateSHA256: candidateSHA, ReviewPath: reviewRel, ReviewSHA256: hashBytes([]byte(review)), Reviewer: "reviewer", Destination: spec.target})
	}
	proposal := filepath.Join(root, "proposal")
	runGit(t, git, root, "clone", "--quiet", "--no-local", source, proposal)
	writeFile(t, filepath.Join(proposal, "packs", "fixture-pack", "method-a.md"), []byte("# Method A\n\n- Existing rule.\n- Record counterexamples.\n- State evidence limits.\n"))
	writeFile(t, filepath.Join(proposal, "packs", "fixture-pack", "method-b.md"), []byte("# Method B\n\n- Existing rule.\n- Keep verification reproducible.\n"))
	patch := runGitRaw(t, git, proposal, "diff", "--binary", "--full-index", "--no-ext-diff", "--", "packs/fixture-pack/method-a.md", "packs/fixture-pack/method-b.md")
	writeFile(t, filepath.Join(caseRoot, ".steamai-vnext", filepath.FromSlash(request.Patch)), patch)
	targets, _, err := capturePatchImages(git, source, filepath.Join(caseRoot, ".steamai-vnext", filepath.FromSlash(request.Patch)), []string{"packs/fixture-pack/method-a.md", "packs/fixture-pack/method-b.md"})
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(candidateRecords, func(i, j int) bool { return candidateRecords[i].CandidatePath < candidateRecords[j].CandidatePath })
	batchReview := renderBatchReview(identity, runGit(t, git, source, "rev-parse", "HEAD"), request, candidateRecords, targets, hashBytes(patch))
	writeFile(t, filepath.Join(caseRoot, ".steamai-vnext", filepath.FromSlash(request.BatchReview)), []byte(batchReview))
	return batchFixture{git: git, source: source, caseRoot: caseRoot, request: request}
}

func renderCandidate(identity casebootstrap.CurrentIdentity, findingRel, findingSHA, reviewRel, reviewSHA, target, lesson string) string {
	return fmt.Sprintf("# Learning\n\n- Source finding：`%s`\n- Source finding SHA-256：`%s`\n- Source accepted review：`%s`\n- Source review SHA-256：`%s`\n- Proposed destination：`%s`\n- Selected pack：`%s`\n- Source revision：`%s`\n- Pack tree：`%s`\n- Common tree：`%s`\n- Snapshot digest：`%s`\n\n## 可复用经验\n\n%s\n", findingRel, findingSHA, reviewRel, reviewSHA, target, identity.Pack, identity.Revision, identity.PackTree, identity.CommonTree, identity.PayloadDigest, lesson)
}

func renderBatchReview(identity casebootstrap.CurrentIdentity, head string, request Request, candidates []CandidateRecord, targets []TargetRecord, patchSHA string) string {
	var out strings.Builder
	fmt.Fprintf(&out, "# R-LB-001\n\n- Reviewer 单写者：`reviewer`\n- Selected pack：`%s`\n- Case revision：`%s`\n- Canonical HEAD：`%s`\n- Snapshot digest：`%s`\n- Patch：`%s`\n- Patch SHA-256：`%s`\n- Added-lines deny result：`clear`\n- `git apply --check` result：`pass`\n- Candidate mapping/theme：`pass`\n- Dedup/conflict/counterexamples：`pass`\n- Redaction：`pass`\n\n## Candidates\n\n", identity.Pack, identity.Revision, head, identity.PayloadDigest, request.Patch, patchSHA)
	for _, item := range candidates {
		fmt.Fprintf(&out, "- Candidate：`%s`\n- Candidate SHA-256：`%s`\n- Eligibility review：`%s`\n- Eligibility review SHA-256：`%s`\n- Destination：`%s`\n", item.CandidatePath, item.CandidateSHA256, item.ReviewPath, item.ReviewSHA256, item.Destination)
	}
	out.WriteString("\n## Targets\n\n")
	for _, item := range targets {
		fmt.Fprintf(&out, "- Target：`%s`\n- Preimage SHA-256：`%s`\n- Preimage bytes：`%d`\n- Postimage SHA-256：`%s`\n- Postimage bytes：`%d`\n", item.Path, item.PreSHA256, item.PreBytes, item.PostSHA256, item.PostBytes)
	}
	out.WriteString("\n## Final decision\n\n- Decision：`accepted`\n")
	return out.String()
}

func mustCurrent(t *testing.T, root string) casebootstrap.CurrentIdentity {
	t.Helper()
	v, err := casebootstrap.InspectCurrent(root)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
func runGit(t *testing.T, git, dir string, args ...string) string {
	t.Helper()
	return strings.TrimSpace(string(runGitRaw(t, git, dir, args...)))
}

func runGitRaw(t *testing.T, git, dir string, args ...string) []byte {
	t.Helper()
	cmd := exec.Command(git, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
	return out
}
