package vnextcontract

import (
	"strings"
	"testing"
)

// TestToolAvailabilityFourStateReportStaysOnDemandAndDeclarative 钉住工具
// 可用性四态报告合同：common/policies/tool-adapters.md 是判定细则的唯一
// 来源，SKILL.md 只保留短指针与四态名/依据/下一步。静态断言只证明合同
// 文本存在，不证明真实工具可用。
func TestToolAvailabilityFourStateReportStaysOnDemandAndDeclarative(t *testing.T) {
	repo := repoRoot(t)
	skill := readPrototypeFile(t, repo, ".claude/skills/steamai/SKILL.md")
	policy := readPrototypeFile(t, repo, "common/policies/tool-adapters.md")

	for _, state := range []string{
		"`未安装`", "`已安装未暴露`", "`会话可见未实测`", "`本任务实测可用`",
	} {
		assertContains(t, skill, state, "SKILL.md")
		assertContains(t, policy, state, "tool-adapters.md")
	}

	// skill 侧只留短指针：四态名、依据/下一步、Commander/member 两种实际
	// 路由与 policy 单一来源。
	for _, required := range []string{
		"common/policies/tool-adapters.md",
		"`.steamai-vnext/pack-snapshot/common/policies/tool-adapters.md`",
		"`../../pack-snapshot/common/policies/tool-adapters.md`",
		"观察依据与阻塞/下一步",
		"唯一来源",
	} {
		assertContains(t, skill, required, "SKILL.md")
	}
	for _, duplicated := range []string{
		"不臆断已安装", "不改判为未安装", "不自动扫描环境",
	} {
		if strings.Contains(skill, duplicated) {
			t.Fatalf("SKILL.md 重复了 tool-adapters.md 的判定细则 %q，违反合同单一来源", duplicated)
		}
	}

	// policy 侧承载完整判定语义与按需边界。
	for _, required := range []string{
		"只对本任务必要", "都不等于可用", "不臆断已安装", "权限阻塞",
		"不改判为未安装", "不自动重试或绕过", "按需报告", "不自动扫描环境",
		"当前会话没有暴露该工具", "尚未完成过一次最小成功操作",
	} {
		assertContains(t, policy, required, "tool-adapters.md")
	}

	// 负例：按需报告不得升级为自动供应或必做启动流程。
	for _, forbidden := range []string{"自动安装依赖", "全量扫描", "steamai __tool"} {
		if strings.Contains(skill, forbidden) || strings.Contains(policy, forbidden) {
			t.Fatalf("工具可用性四态报告引入越界自动化 %q", forbidden)
		}
	}
}
