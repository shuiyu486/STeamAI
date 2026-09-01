# STeamAI 薄核心模板

这是 canonical `/steamai` 使用的声明式模板与验收合同源。`steamai-vnext-thin-core-v1` 已完成；用户入口是 `.claude/skills/steamai/SKILL.md`。

原型只使用：

- Claude Code 原生独立 session、resume、可选跨会话消息和 tactical subagent；
- case 与成员目录的 `CLAUDE.md`；
- 人类可读的 artifact、evidence、finding、review 和 learning candidate；
- Git 与用户确认后的 pack 变更。

它不 import、迁移或调用旧 Go control plane、session host、current loop，也不发布 runtime executable。旧产品控制面与旧项目兼容入口均已删除。

## 原型目录

- `templates/case/CLAUDE.md`：case 共享边界与团队章程。
- `templates/member/CLAUDE.md`：正式成员身份和当前任务。
- `templates/roles/`：按需组队时可选择的角色片段，不是固定团队。
- `templates/research/`：值得跨成员共享和复核的研究产物，包括 append-only review round 与 exact learning review。
- `capabilities.md`：必需/可选 Claude Code 原生能力和降级边界。
- `acceptance.md`：自动 capability/context/file-access probe 与真实独立 session live acceptance。
- `learning-feedback.md`：accepted finding/review 到 exact Git patch、用户确认与 snapshot 不漂移的回流合同。
- `project-skill/SKILL.md`：分发到外部 case 的 canonical skill exact byte mirror。

外部 case 同时得到 `.steamai-vnext/contracts/` 中的模板与 learning 合同，以及 selected pack 与完整 `common/**` 的同 revision snapshot。Fresh preview 绑定 source blob 与 target pre-state；Apply 先在 sibling staging 验证完整 state tree 和 payload digest，再发布 exact project-local skill，最后发布 completed marker。分发后日常不读取 mutable source clone。

## 验证原则

先用临时、无真实样本的 fixture 跑通团队闭环。自动 probe 证明 context 与文件访问能力；至少两个真实独立 session 的 live acceptance 证明可观察、可纠偏和成员直连，二者不能互相替代。只有重复出现且无法由 Claude Code 原生能力与简单文件解决的问题，才允许增加窄职责、无状态 helper。