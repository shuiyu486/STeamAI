package vnextcontract

import (
	"strings"
	"testing"
)

// 文本合同与 native argv 单测互补，不声称已通过前台或真实模型验收。
func TestRoleTierConfigAndDesktopRouting(t *testing.T) {
	repo := repoRoot(t)
	for rel, required := range map[string][]string{
		"common/policies/agent-team.md": {
			"steamai models", "--tier high|standard",
			"%LOCALAPPDATA%\\STeamAI\\models.json", "允许同档",
			"high=opus", "standard=sonnet",
			"历史的 high / standard 角色意图",
			"每批最多 5 个且不嵌套", "不修改全局模型配置",
			"不是计费硬上限", "已有会话不强切", "Agent(model=...)",
		},
		"common/policies/subagents.md": {
			"每批最多 5 个且不嵌套", "standard 档", "Agent(model=...)",
			"fork 型子 agent 继承父会话模型", "不为此修改原生 Claude 环境",
		},
		"common/policies/context-budget.md": {"不在主会话重复执行", "不读取子 agent 完整 transcript"},
		"vnext/templates/case/CLAUDE.md": {
			"high 档", "standard 档", "steamai models",
			"不把 `high` / `standard` 当 Claude 模型名", "每批最多 5 个且不嵌套",
		},
		"vnext/templates/member/CLAUDE.md": {
			"standard 档", "steamai models", "Agent(model=...)",
			"不把 `high` / `standard` 当 Claude 模型名", "每批最多 5 个且不嵌套",
		},
		".claude/skills/steamai/SKILL.md": {
			"--tier high|standard", "steamai models", "%LOCALAPPDATA%\\STeamAI\\models.json",
			"WezTerm", "common/tooling/windows-desktop-mcp.md",
			"fork 型子 agent 继承父会话模型",
			"claude --model <high 档当前别名>",
		},
		"vnext/capabilities.md": {
			"__member-session", "--tier <high|standard>", "--high <别名>", "--standard <别名>",
			"CLAUDE_CONFIG_DIR", "不接受任意 exe/env 载荷", "--scope user", "--add-dir",
			"steamai models", "models.json", "只接受 `opus`、`sonnet` 或 `haiku`",
			"--append-system-prompt", "--model <所选别名>",
			"历史的 high / standard 角色意图",
			"不从 WezTerm server 环境或磁盘重新取值",
			"不重映射", "不是计费硬上限", "Agent(model=...)",
		},
		"common/policies/tool-adapters.md": {"../tooling/windows-desktop-mcp.md", "多个会话不得同时操作同一桌面"},
		"common/policies/README.md":        {"../tooling/windows-desktop-mcp.md"},
		"common/tooling/windows-desktop-mcp.md": {
			"windows-mcp==0.8.9", "ANONYMIZED_TELEMETRY=false", "--offline",
			"Screenshot,Snapshot,Click,Type,Scroll,Move,Shortcut,Wait", "ControlStatus", "9",
			"--scope user", "不假定 `--add-dir`", "不暴露 `PowerShell`",
			"UAC", "前台", "用户明确同意时间窗", "不能用协议通过替代它",
			"e90e5b02b3e545991f203ce93cefb436a326861657968591653fa254105dc85d",
		},
		"vnext/acceptance.md": {
			"WezTerm live", "Desktop MCP", "参数机械验证不代替",
			"steamai models", "models.json",
		},
		"README.md": {
			"终端、模型和桌面操作", "需要新版 `steamai.exe`", "已有会话不强切",
			"steamai models --high sonnet --standard haiku",
			"steamai models --high opus --standard sonnet",
			"恢复默认", "不是计费硬上限",
		},
		"CLAUDE.md": {
			"steamai models", "models.json", "不是计费硬上限",
			"claude --model <high 档当前别名>",
		},
	} {
		text := readPrototypeFile(t, repo, rel)
		for _, phrase := range required {
			assertContains(t, text, phrase, rel)
		}
	}
}

// high/standard 是 STeamAI 档位名，不是 Claude 模型名：当前角色文档不得把档位当
// `--model` 值、把别名当 `--tier` 值，也不得保留旧 `--model opus|sonnet` 入口形态。
func TestTierNamesNeverBecomeClaudeModelArguments(t *testing.T) {
	repo := repoRoot(t)
	for _, rel := range []string{
		"README.md",
		"CLAUDE.md",
		".claude/skills/steamai/SKILL.md",
		"common/policies/agent-team.md",
		"common/policies/subagents.md",
		"common/policies/context-budget.md",
		"vnext/capabilities.md",
		"vnext/acceptance.md",
		"vnext/templates/case/CLAUDE.md",
		"vnext/templates/member/CLAUDE.md",
	} {
		text := readPrototypeFile(t, repo, rel)
		for _, forbidden := range []string{
			"--model high", "--model standard",
			"--model opus|sonnet", "--model <opus或sonnet>",
			"--tier opus", "--tier sonnet", "--tier opus|sonnet",
		} {
			if strings.Contains(text, forbidden) {
				t.Errorf("current role doc %s confuses tier and Claude model alias via %q", rel, forbidden)
			}
		}
	}
}
