package vnextcontract

import "testing"

// 文本合同与 native argv 单测互补，不声称已通过前台或真实模型验收。
func TestRoleModelsAndDesktopRouting(t *testing.T) {
	repo := repoRoot(t)
	for rel, required := range map[string][]string{
		"common/policies/agent-team.md": {
			"ANTHROPIC_DEFAULT_OPUS_MODEL", "ANTHROPIC_DEFAULT_SONNET_MODEL",
			"--model opus|sonnet", "每批最多 5 个且不嵌套", "不修改全局模型配置",
		},
		"common/policies/subagents.md":      {"每批最多 5 个且不嵌套", "显式选 `sonnet`"},
		"common/policies/context-budget.md": {"不在主会话重复执行", "不读取子 agent 完整 transcript"},
		"vnext/templates/case/CLAUDE.md":    {"ANTHROPIC_DEFAULT_OPUS_MODEL", "每批最多 5 个且不嵌套"},
		"vnext/templates/member/CLAUDE.md":  {"显式 Sonnet 子 agent", "每批最多 5 个且不嵌套"},
		".claude/skills/steamai/SKILL.md":   {"--model opus", "--model opus|sonnet", "WezTerm", "common/tooling/windows-desktop-mcp.md"},
		"vnext/capabilities.md":             {"__member-session", "CLAUDE_CONFIG_DIR", "不接受任意 exe/env 载荷", "--scope user", "--add-dir"},
		"common/policies/tool-adapters.md":  {"../tooling/windows-desktop-mcp.md", "多个会话不得同时操作同一桌面"},
		"common/policies/README.md":         {"../tooling/windows-desktop-mcp.md"},
		"common/tooling/windows-desktop-mcp.md": {
			"windows-mcp==0.8.9", "ANONYMIZED_TELEMETRY=false", "--offline",
			"Screenshot,Snapshot,Click,Type,Scroll,Move,Shortcut,Wait", "ControlStatus", "9",
			"--scope user", "不假定 `--add-dir`", "不暴露 `PowerShell`",
			"UAC", "前台", "用户明确同意时间窗", "不能用协议通过替代它",
			"e90e5b02b3e545991f203ce93cefb436a326861657968591653fa254105dc85d",
		},
		"vnext/acceptance.md": {"WezTerm live", "Desktop MCP", "Opus 参数机械验证不代替"},
		"README.md":           {"终端、模型和桌面操作", "需要新版 `steamai.exe`", "已有会话不强切"},
	} {
		text := readPrototypeFile(t, repo, rel)
		for _, phrase := range required {
			assertContains(t, text, phrase, rel)
		}
	}
}
