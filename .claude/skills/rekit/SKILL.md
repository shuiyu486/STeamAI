---
name: rekit
description: Legacy compatibility and maintenance skill for legacy-only .rekit projects, central STeamAI repository maintenance, or an explicit /rekit diagnostic request. Current .steamai projects use the project-local /steamai skill instead.
---

# rekit compatibility

`/rekit` 是迁移期兼容与维护诊断入口，不是 current STeamAI 项目的默认 UX。新项目使用 project-local `/steamai`、`.steamai` 和 verified runtime bundle；用户日常用自然语言指挥主 Agent。

底层 Go CLI 是 canonical runtime；`rekit.ps1` 只是 retained compatibility façade。legacy case 只生成 `.claude/skills/rekit/SKILL.md` 薄 shim，底层 runtime 只作为 `/rekit` 的内部实现。不要为 current 项目调用中央 kit source runtime，也不要安装或修改用户级 skill。

## 先确定工作位置

从当前目录向上做一次确定性项目分类：

1. 同时发现 `.steamai` 与 `.rekit`：立即 fail-closed；不得合并、择优、双写或把其中一个当 alias。
2. 发现 `.steamai`，或发现 project-local `.claude/skills/steamai/SKILL.md`：停止本 skill，交给该项目的 `/steamai`；不得从中央 kit 运行 `/rekit` 代替它。
3. 只发现 `.rekit/instance.yml`：这是 legacy-only 项目；从 metadata 取得 `templateRoot`、`templatePack` 和 case root，再使用 `<templateRoot>/.claude/skills/rekit/SKILL.md` 的 deterministic compatibility 协议。
4. 当前目录是 STeamAI 实现仓库：仅在维护本仓库或用户显式要求 `/rekit` 诊断时使用本 skill。GitHub canonical identity 是 `shuiyu486/STeamAI`；旧 Go module/internal names 不用于判断 repository identity。
5. 普通目录且没有上述标记：不要自动接入或创建 legacy state；引导用户从该目录使用 `/steamai`／自然语言走 current 安全接入。

路径、binding 或 shim 不可信时，只做 fresh status/repair preview；不要扫描零散 `.rekit` 文件拼出状态，也不要静默改写 metadata。

## Legacy 自然语言意图

| 用户表达 | 应执行的产品动作 |
|---|---|
| “现在到哪了”“进展怎样”“下一步是什么” | 只读取 public `status` JSON；零写入、零 Claude launch |
| “在新位置开始，目标是……” | 仅对缺失或已安全接入的 case 调用 daily goal owner |
| “继续上次任务” | 对 attached case 调用 daily resume owner |
| “复核不对，按这个意见重做……” | 把用户原话交给 daily correction owner，不自行编造 intervention 字段 |
| “交给新会话”“给我接手信息” | 读取 fresh status；只有用户明确要求发布时才走 canonical handoff preview/Apply |
| “同步模板”“整理可复用经验” | `sync` / `promote` 始终 review-first，先给范围和差异，再等精确确认 |
| “运行调试/patch/dump/network/其它 heavy action” | 只进入 canonical gate；没有 strict durable profile 与 `authorized-gate` 就停止 |

意图无法由 fresh typed state 唯一确定时，只问一个问题。typed queue 的非 follow-up actions 涉及多个不同 lane 时展示 typed choices，停止且不调用 daily host。

## 固定执行协议

### 1. 状态查询

仅在 legacy-only case 中，从可信 `templateRoot` 执行 `go run ./cmd/rekit -- -Command status -Target <case-root> -Format json`；pack 从 attached metadata 派生，不接受冲突 override。current `.steamai` 项目不得使用这条中央 kit 路径。

case 模式只消费 fresh `caseShim`、`caseMission`、`onboarding`、case-scoped `missionControlRunbook`、current driver/action queue 和 typed blockers/choices；忽略 `projectHandoff`，不得把 kit 路线、批次、commit/push 或 release 验证当成 case 进度。

状态查询不得调用 daily host、Apply 或 Claude。若 current action 为 `actionId=case-mission-onboarding` 或 `state=case-board-missing`，只说明任务尚未开始并请用户给目标。只输出当前处境、一个下一步和必要选择。

### 2. 开始或继续

只有 legacy-only 项目的用户明确表达开始/继续时，才从可信 `templateRoot` 内部调用 Go-owned `rekit-host -daily`：

```text
fresh goal: go run ./cmd/rekit-host -daily -target <case-root> -goal <用户目标>
resume:     go run ./cmd/rekit-host -daily -target <case-root> -lane <typed-choice-id>
correction: go run ./cmd/rekit-host -daily -target <case-root> -lane <typed-choice-id> -correction <用户原话>
```

`rekit-host` 后不能插入 `--`。单 lane 可省略 `-lane`；多 lane 先展示 `action.choices[]`，选定后用 choice `id`。选择前零启动、零写入，后续保持同一 selector。

每次只相信返回的 `action.code`、`action.requiresInput`、`action.choices[]`、typed `failure` 与 fresh durable status。Remote Control 只消费 fresh typed request；opaque endpoint 不是 durable identity，`uncertain` 立即停止且不重发、不做 same-job replacement，transport 不授予 heavy action 或 authority/confirmed。

随后按 action code 处理：

- `completed`：说明当前阶段已完成；`ready-to-continue`：结果已保存，后续先刷新 status。
- `waiting-for-correction`：停下，只请用户补充纠偏。
- `directory-adoption-required` / `confirmation-required` / `ready-for-evidence-review`：展示精确计划或有界证据，等待确认。
- `blocked`：说明一条人话阻塞，不猜路由。
- `failed`：只用 typed recovery/next action，不把失败说成完成。

返回后如需继续，必须重新读取 fresh status。不要根据 `FinalState` 文案、文件是否存在或错误字符串自制下一步。

### 3. 写入与确认

- 开始/继续/纠偏只授权对应 daily 操作，不能扩大为其它副作用。
- repair、handoff、sync、promote、目录接入或 profile 变更都先展示 exact preview，再确认 target、pack、范围和动作。
- `sync` 是 kit → case；`promote` 是 case → kit；均 review-first。
- `continue -Apply` 不写 authority/confirmed、不执行 heavy tool。
- gate 只记录 request/evidence；actual heavy action 仍要求 strict durable profile + exact `authorized-gate`。
- 不自动写 authority/confirmed、提升 manual lane 或把口头确认当 authorized gate。
- legacy 项目迁移到 current 状态根必须使用独立 `migrate-state` zero-write preview → exact hash-bound Apply → durable receipt；不得边迁移边继续写 `.rekit`。

### 4. 停止条件

以下情况立即停止并给一条人话动作：dual root、current `/steamai` owner 已存在、blocked/route drift、intervention/Reviewer rejection/intake failure、identity/owner/hash stale、多选未选、目录覆盖、gate/profile/budget/output 越界，以及 authority/confirmed/schema migration/公共入口删除/未授权副作用。

不回退 legacy command，不绕过确认，不用 LLM 文案替代 canonical decision。

## 用户输出格式

默认只给 1～3 个短句：

1. **现在**：已完成、可继续、等纠偏、需确认、被阻塞或失败；
2. **下一步**：一个用户可理解的动作；
3. **选择**：仅在 `requiresInput=true` 时列出 typed choices。

不要把 lane ID、actor、SHA、session ID、generation、绝对内部路径或底层命令当成用户必填项。用户明确要求 debug/maintenance 时，才展开 typed detail。

## 按需维护入口

- current canonical skill：`.claude/skills/steamai/SKILL.md`
- 文档路由：`docs/context-routing.md`
- Agent Team 日常与接手：`docs/agent-team-usage.md`
- active route/current card：`docs/real-usage-hardening-roadmap.md`
- 已完成四闭环设计：`docs/daily-product-closure-plan.md`（仅复核历史完成证据时按需读取）
- Remote Control Reviewer transport 使用与边界：`docs/agent-team-usage.md` 对应小节；active状态以路线图为准
- release 与 PowerShell 退役边界：由 router 分别进入 `docs/release-readiness.md`、`docs/powershell-deprecation.md`
