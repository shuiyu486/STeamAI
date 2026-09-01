package vnextcontract

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDay2ContractsKeepRosterTaskAndObservationSeparate(t *testing.T) {
	repo := repoRoot(t)
	caseTemplate := readPrototypeFile(t, repo, "vnext/templates/case/CLAUDE.md")
	for _, required := range []string{
		"durable roster 的唯一 source",
		"`active`：已分配当前正式任务",
		"`completed`：最近正式任务满足退出条件",
		"`inactive`：被暂停、合并或暂时不需要",
		"计入 3 名 execution + 1 名 Reviewer 上限",
		"不表示当前存在可见 session",
		"`durable`",
		"`observed-now`",
		"`unknown`",
		"roster `active` 与 session `unknown` 可以同时成立",
		"不得创建 `status.md`",
	} {
		assertContains(t, caseTemplate, required, "case day-2 contract")
	}

	memberTemplate := readPrototypeFile(t, repo, "vnext/templates/member/CLAUDE.md")
	for _, required := range []string{
		"本成员是本文件的产品单写者",
		"expected current task 的全部七项",
		"返回 `HOLD_STALE_TASK`",
		"零覆盖",
		"两个可写 session",
		"直到用户直接选择一个 session",
		"本文件不复制 roster",
	} {
		assertContains(t, memberTemplate, required, "member day-2 contract")
	}
	if strings.Contains(memberTemplate, "{{TEAM_ROSTER}}") || strings.Contains(memberTemplate, "{{TEAM_ROSTER_ROWS}}") {
		t.Fatal("member template retains a duplicate durable roster")
	}

	skill := readPrototypeFile(t, repo, ".claude/skills/steamai/SKILL.md")
	for _, required := range []string{
		"roster lifecycle 的唯一 durable source",
		"首次启动后由成员本人单写",
		"`HOLD_STALE_TASK`、零覆盖",
		"`durable (<case-relative source>)`",
		"未观察到不能写 offline/completed",
		"状态回答不写入 `status.md`",
	} {
		assertContains(t, skill, required, "canonical day-2 behavior")
	}
}

func TestReviewRoundsAreAppendOnlyAndCurrentnessBound(t *testing.T) {
	repo := repoRoot(t)
	reviewTemplate := readPrototypeFile(t, repo, "vnext/templates/research/review.md")
	roundTemplate := readPrototypeFile(t, repo, "vnext/templates/research/review-round.md")
	reviewerRole := readPrototypeFile(t, repo, "vnext/templates/roles/reviewer.md")
	for _, text := range []string{reviewTemplate, roundTemplate} {
		for _, required := range []string{
			"{{REVIEW_ROUND}}",
			"{{PREVIOUS_REVIEW_ROUND_OR_NONE}}",
			"{{FINDING_REF}}",
			"{{FINDING_SHA256}}",
			"{{REVIEWED_EVIDENCE_REFS_WITH_SHA256}}",
			"{{DECISION}}",
		} {
			assertContains(t, text, required, "review round template")
		}
	}
	for _, required := range []string{
		"只追加连续 round",
		"不覆盖历史",
		"更换 Reviewer 时新建 review 文件",
		"旧 `accepted` 为 stale",
	} {
		assertContains(t, reviewerRole, required, "Reviewer append-only role")
	}

	root := t.TempDir()
	finding := filepath.Join(root, "findings", "F-001.md")
	evidence := filepath.Join(root, "evidence", "E-001.md")
	review := filepath.Join(root, "reviews", "R-001.md")
	writeDay2Fixture(t, finding, "# F-001\n\nClaim bounded by E-001.\n")
	writeDay2Fixture(t, evidence, "# E-001\n\nSynthetic observation.\n")
	findingSHA := day2SHA(readDay2Fixture(t, finding))
	evidenceSHA := day2SHA(readDay2Fixture(t, evidence))
	round1 := renderDay2ReviewRound(roundTemplate, "1", "none", "needs-evidence", findingSHA, evidenceSHA)
	writeDay2Fixture(t, review, round1)
	round1Before := readDay2Fixture(t, review)

	writeDay2Fixture(t, evidence, "# E-001\n\nSynthetic observation with bounded reproduction.\n")
	evidenceSHA2 := day2SHA(readDay2Fixture(t, evidence))
	round2 := renderDay2ReviewRound(roundTemplate, "2", "1", "accepted", findingSHA, evidenceSHA2)
	file, err := os.OpenFile(review, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("\n" + round2); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	got := string(readDay2Fixture(t, review))
	if !strings.HasPrefix(got, string(round1Before)) {
		t.Fatal("Reviewer append rewrote the previous needs-evidence round")
	}
	if !day2ReviewCurrent(got, "reviewer", findingSHA, map[string]string{"../evidence/E-001.md": evidenceSHA2}, "accepted") {
		rounds, ok := parseDay2ReviewRounds(got)
		t.Fatalf("current accepted round did not bind the latest finding/evidence hashes: parsed=%v rounds=%+v\n%s", ok, rounds, got)
	}
	writeDay2Fixture(t, finding, "# F-001\n\nChanged claim after acceptance.\n")
	if day2ReviewCurrent(got, "reviewer", day2SHA(readDay2Fixture(t, finding)), map[string]string{"../evidence/E-001.md": evidenceSHA2}, "accepted") {
		t.Fatal("accepted review stayed current after finding drift")
	}

	malformed := []struct {
		name string
		text string
	}{
		{"missing confidence", strings.Replace(round2, "- Confidence：`high`\n", "", 1)},
		{"skipped round", renderDay2ReviewRound(roundTemplate, "3", "2", "accepted", findingSHA, evidenceSHA2)},
		{"wrong previous", renderDay2ReviewRound(roundTemplate, "2", "none", "accepted", findingSHA, evidenceSHA2)},
		{"reviewer changed", strings.Replace(round2, "- Reviewer：`reviewer`", "- Reviewer：`other-reviewer`", 1)},
		{"hash outside evidence refs", strings.Replace(strings.Replace(round2, "- ../evidence/E-001.md — "+evidenceSHA2, "- ../evidence/E-001.md — "+strings.Repeat("0", 64), 1), "synthetic review", "synthetic review "+evidenceSHA2, 1)},
	}
	for _, test := range malformed {
		t.Run(test.name, func(t *testing.T) {
			candidate := string(round1Before) + "\n" + test.text
			if day2ReviewCurrent(candidate, "reviewer", findingSHA, map[string]string{"../evidence/E-001.md": evidenceSHA2}, "accepted") {
				t.Fatal("malformed review round was treated as current")
			}
		})
	}
}

func TestRepositoryDoesNotAddParallelDay2StateSurfaces(t *testing.T) {
	repo := repoRoot(t)
	for _, rel := range []string{
		"vnext/templates/case/status.md",
		"vnext/templates/case/roster.md",
		"vnext/templates/case/sessions.md",
		"vnext/templates/case/tasks.md",
		"vnext/templates/case/messages.md",
	} {
		if _, err := os.Lstat(filepath.Join(repo, filepath.FromSlash(rel))); err == nil {
			t.Errorf("parallel day-2 state surface exists: %s", rel)
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
}

func renderDay2ReviewRound(template, round, previous, decision, findingSHA, evidenceSHA string) string {
	replacements := map[string]string{
		"{{REVIEW_ROUND}}":                       round,
		"{{PREVIOUS_REVIEW_ROUND_OR_NONE}}":      previous,
		"{{REVIEWER}}":                           "reviewer",
		"{{FINDING_REF}}":                        "../findings/F-001.md",
		"{{FINDING_SHA256}}":                     findingSHA,
		"{{DECISION}}":                           decision,
		"{{CONFIDENCE}}":                         "high",
		"{{REVIEW_SUMMARY}}":                     "synthetic review",
		"{{REVIEWED_EVIDENCE_REFS_WITH_SHA256}}": "- ../evidence/E-001.md — " + evidenceSHA,
		"{{RISKS_OR_GAPS}}":                      "synthetic-only scope",
		"{{NEXT_ACTION}}":                        "return to owner or accept",
	}
	for key, value := range replacements {
		template = strings.ReplaceAll(template, key, value)
	}
	return template
}

type day2ReviewRound struct {
	Number       int
	Previous     string
	Reviewer     string
	Finding      string
	FindingSHA   string
	Decision     string
	Confidence   string
	EvidenceRefs map[string]string
	Summary      string
	Risks        string
	NextAction   string
}

func day2ReviewCurrent(review, reviewer, findingSHA string, evidenceRefs map[string]string, decision string) bool {
	rounds, ok := parseDay2ReviewRounds(review)
	if !ok || len(rounds) == 0 {
		return false
	}
	for i, round := range rounds {
		if round.Number != i+1 || round.Reviewer != reviewer || round.Finding == "" || round.FindingSHA == "" ||
			round.Decision == "" || round.Confidence == "" || round.Summary == "" || round.Risks == "" || round.NextAction == "" || len(round.EvidenceRefs) == 0 {
			return false
		}
		wantPrevious := "none"
		if i > 0 {
			wantPrevious = fmt.Sprintf("%d", i)
		}
		if round.Previous != wantPrevious {
			return false
		}
	}
	last := rounds[len(rounds)-1]
	if last.FindingSHA != findingSHA || last.Decision != decision || len(last.EvidenceRefs) != len(evidenceRefs) {
		return false
	}
	for path, sha := range evidenceRefs {
		if last.EvidenceRefs[path] != sha {
			return false
		}
	}
	return true
}

func parseDay2ReviewRounds(review string) ([]day2ReviewRound, bool) {
	const heading = "## Review round `"
	var chunks []string
	for _, part := range strings.Split(review, heading)[1:] {
		chunks = append(chunks, heading+part)
	}
	if len(chunks) == 0 {
		return nil, false
	}
	var rounds []day2ReviewRound
	for _, chunk := range chunks {
		lines := strings.Split(chunk, "\n")
		if len(lines) < 2 || !strings.HasSuffix(lines[0], "`") {
			return nil, false
		}
		numberText := strings.TrimSuffix(strings.TrimPrefix(lines[0], heading), "`")
		number := 0
		if _, err := fmt.Sscanf(numberText, "%d", &number); err != nil || number < 1 {
			return nil, false
		}
		round := day2ReviewRound{Number: number, EvidenceRefs: map[string]string{}}
		sections := map[string][]string{}
		currentSection := ""
		for _, line := range lines[1:] {
			switch line {
			case "### 判断", "### 检查的证据", "### 风险或缺口", "### 下一步":
				currentSection = line
				continue
			}
			if strings.HasPrefix(line, "- ") {
				if key, value, ok := day2ReviewField(line); ok {
					currentSection = ""
					switch key {
					case "Previous round":
						round.Previous = value
					case "Reviewer":
						round.Reviewer = value
					case "Finding":
						round.Finding = value
					case "Finding SHA-256":
						round.FindingSHA = value
					case "Decision":
						round.Decision = value
					case "Confidence":
						round.Confidence = value
					default:
						return nil, false
					}
					continue
				}
			}
			if currentSection != "" {
				if strings.HasPrefix(line, "### ") || (strings.HasPrefix(line, "- ") && currentSection != "### 检查的证据") {
					return nil, false
				}
				sections[currentSection] = append(sections[currentSection], line)
				continue
			}
			key, value, ok := day2ReviewField(line)
			if !ok {
				if strings.TrimSpace(line) != "" {
					return nil, false
				}
				continue
			}
			switch key {
			case "Previous round":
				round.Previous = value
			case "Reviewer":
				round.Reviewer = value
			case "Finding":
				round.Finding = value
			case "Finding SHA-256":
				round.FindingSHA = value
			case "Decision":
				round.Decision = value
			case "Confidence":
				round.Confidence = value
			default:
				return nil, false
			}
		}
		round.Summary = strings.TrimSpace(strings.Join(sections["### 判断"], "\n"))
		round.Risks = strings.TrimSpace(strings.Join(sections["### 风险或缺口"], "\n"))
		round.NextAction = strings.TrimSpace(strings.Join(sections["### 下一步"], "\n"))
		for _, line := range sections["### 检查的证据"] {
			line = strings.Trim(strings.TrimSpace(line), "`")
			if line == "" {
				continue
			}
			path, sha, ok := strings.Cut(strings.TrimPrefix(line, "- "), " — ")
			if !ok || !strings.HasPrefix(line, "- ") || path == "" || len(sha) != 64 || round.EvidenceRefs[path] != "" {
				return nil, false
			}
			round.EvidenceRefs[path] = sha
		}
		rounds = append(rounds, round)
	}
	return rounds, true
}

func day2ReviewField(line string) (string, string, bool) {
	if !strings.HasPrefix(line, "- ") {
		return "", "", false
	}
	key, value, ok := strings.Cut(strings.TrimPrefix(line, "- "), "：`")
	if !ok || !strings.HasSuffix(value, "`") {
		return "", "", false
	}
	return key, strings.TrimSuffix(value, "`"), true
}

func writeDay2Fixture(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readDay2Fixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func day2SHA(data []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(data))
}
