# STeamAI case 合同与模板

`vnext/**` 是 canonical `/steamai` Fresh 时物化到 case-local `contracts/` 的声明式合同源。当前产品路线见 `docs/windows-native-product-roadmap.md`；用户入口是 Windows 原生 `steamai.exe` + `.claude/skills/steamai/SKILL.md`。

本目录只使用 Markdown 声明：

- case 与成员目录的 `CLAUDE.md`；
- Claude Code 原生独立 session、resume/attach/respawn、跨会话消息和 tactical subagent；
- artifact、evidence、finding、append-only review、immutable learning candidate 与 exact batch review；
- Git 与用户确认后的 canonical working-tree pack 变更。

它不包含 runtime executable 或脚本。生产 `steamai.exe` 位于 `cmd/steamai` / `internal/steamai`，只负责安装/Fresh/update/uninstall/卸载后窄自清理/exact apply/可见启动/瞬时互斥，不承担团队控制面。旧 Go control plane、session host、PowerShell façade、adapter、旧项目 importer 与兼容入口均已删除。

## 目录

- `templates/case/CLAUDE.md`：case 共享边界与 durable roster。
- `templates/member/CLAUDE.md`：正式成员身份和当前任务。
- `templates/roles/`：Commander 按需组队时合并的角色片段。
- `templates/research/`：artifact/evidence/finding/review/candidate/batch review 模板。
- `capabilities.md`：Claude Code 原生能力和降级边界。
- `acceptance.md`：自动与真实 Windows/Claude Code 验收分层。
- `learning-feedback.md`：accepted evidence chain → candidate eligibility → thematic exact batch → 用户确认 → working-tree apply。

project-local `/steamai` 直接来自 Fresh preview 时 canonical working tree 中 current stage-0 tracked skill 的 exact bytes；HEAD blob 只作历史 anchor，`vnext/` 不保存第二份 skill mirror。

外部 case 同时得到 `.steamai-vnext/contracts/` 中全部模板与 learning 合同，以及 selected pack 与完整 `common/**` snapshot。Fresh preview 绑定 source record 与 target pre-state；Apply 先在 sibling staging 验证完整 state tree 和 payload digest，再发布 exact project-local skill，最后发布 completed marker。分发后日常 current case 不读取 mutable source checkout。

## 验证原则

自动 tests 证明 deterministic filesystem/Git contract；真实 Windows setup/PATH/process、可见 Claude Code 会话、用户纠偏和跨会话消息必须按 `acceptance.md` 独立验收。只有重复出现且无法由 Claude Code 原生能力与简单文件解决的问题，才允许增加窄职责、无状态 helper。
