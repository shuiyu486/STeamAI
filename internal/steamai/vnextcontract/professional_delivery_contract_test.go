package vnextcontract

import (
	"regexp"
	"testing"
)

// TestProfessionalDeliveryDualTrackContract 验证 DLV-01 研究/开发双交付合同：
// 同一套七字段与产物目录表达两类任务，代码路径分配有界，开发 accepted 不升级
// maturity，pack 工具按用途接纳且不自动安装。
func TestProfessionalDeliveryDualTrackContract(t *testing.T) {
	repo := repoRoot(t)

	caseTemplate := readPrototypeFile(t, repo, "vnext/templates/case/CLAUDE.md")
	for _, required := range []string{
		"明确授权的安全研究或自动化开发 case",
		"不为开发任务伪造安全 finding",
		"源码留在项目原位置",
		"复用同一套 `artifacts/`",
	} {
		assertContains(t, caseTemplate, required, "case dual-track contract")
	}

	memberTemplate := readPrototypeFile(t, repo, "vnext/templates/member/CLAUDE.md")
	for _, required := range []string{
		"case-relative 项目源码路径",
		"同一源码文件不得由多名成员并发写入",
		"整合他人改动前先核对当前基线与用户手工修改",
	} {
		assertContains(t, memberTemplate, required, "member code-path contract")
	}

	reviewerRole := readPrototypeFile(t, repo, "vnext/templates/roles/reviewer.md")
	for _, required := range []string{
		"明确分配给该审查的项目源码、exact diff 与测试回执",
		"开发交付审查绑定所审源码版本或 exact diff",
		"不自动取得 V0–V4 maturity 或 learning 资格",
		"独立运行测试由获准 verifier 承担",
	} {
		assertContains(t, reviewerRole, required, "reviewer dual-track contract")
	}

	analysisRole := readPrototypeFile(t, repo, "vnext/templates/roles/analysis-member.md")
	for _, required := range []string{
		"开发类任务在明确分配的项目源码路径内交付代码、补丁、测试观察与使用说明",
		"测试回执按 evidence 记录",
		"不为开发任务伪造安全 finding",
	} {
		assertContains(t, analysisRole, required, "analysis-member delivery contract")
	}

	skill := readPrototypeFile(t, repo, ".claude/skills/steamai/SKILL.md")
	for _, required := range []string{
		"安全研究或自动化开发 case",
		"开发类任务把要写的 case-relative 项目源码范围列入成员 `allowedWrites`",
		"绑定所审源码版本或 exact diff 与相关测试证据",
		"只表示此次代码交付经审查，不自动取得 V0–V4 maturity 或 learning 资格",
	} {
		assertContains(t, skill, required, "canonical dual-track behavior")
	}

	capabilities := readPrototypeFile(t, repo, "vnext/capabilities.md")
	for _, required := range []string{
		"非 MCP 形态的专业工具",
		"pack 工具按具体用途接纳",
		"先证明上游/native 能力不足",
		"继续禁止通用运行器、adapter host",
		"工具脚本不进入 learningTargets",
	} {
		assertContains(t, capabilities, required, "capabilities tool-admission contract")
	}

	rootClaude := readPrototypeFile(t, repo, "CLAUDE.md")
	for _, required := range []string{
		"面向已授权安全研究与自动化开发",
		"pack 工具按具体用途接纳",
		"继续禁止通用运行器与 adapter host",
	} {
		assertContains(t, rootClaude, required, "root CLAUDE.md positioning")
	}

	readme := readPrototypeFile(t, repo, "README.md")
	assertContains(t, readme, "已授权安全研究与自动化开发", "README.md positioning")

	acceptance := readPrototypeFile(t, repo, "vnext/acceptance.md")
	for _, required := range []string{
		"steamai-professional-delivery-v1",
		"研究/开发双交付合同",
		"不自动取得 V0–V4 或 learning 资格",
		"不安装、不注册、不调用 Playwright/Airtest/Strix/REA",
		"fake adapter",
	} {
		assertContains(t, acceptance, required, "acceptance professional-delivery gates")
	}
}

// TestMemberTemplateKeepsCanonicalSevenFieldSet 确认双交付合同没有为开发任务
// 新增必填字段：member 模板的 placeholder 集合仍是既有 11 项。
func TestMemberTemplateKeepsCanonicalSevenFieldSet(t *testing.T) {
	repo := repoRoot(t)
	memberTemplate := readPrototypeFile(t, repo, "vnext/templates/member/CLAUDE.md")
	re := regexp.MustCompile(`\{\{[A-Z_0-9]+\}\}`)
	got := map[string]bool{}
	for _, m := range re.FindAllString(memberTemplate, -1) {
		got[m] = true
	}
	want := []string{
		"{{MEMBER_NAME}}", "{{ROLE}}", "{{RESPONSIBILITY}}",
		"{{TASK_GOAL}}", "{{INPUTS}}", "{{ALLOWED_READS}}", "{{ALLOWED_WRITES}}",
		"{{DELIVERABLES}}", "{{STOP_OR_ESCALATE}}", "{{EXIT_CONDITIONS}}",
		"{{ROLE_SPECIFIC_RULES}}",
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("member template lost canonical placeholder %s", w)
		}
		delete(got, w)
	}
	for extra := range got {
		t.Errorf("member template introduced non-canonical placeholder %s", extra)
	}
}
