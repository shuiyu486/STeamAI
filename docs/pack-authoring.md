# Pack authoring guide

## 读取指南

本文件用于新增或维护 `packs/<pack>`。普通 case 先读 selected pack snapshot 中的 `references/<pack>/README.md`；只有需要改变 pack 结构、团队建议、工具边界或 learning 目标时才读本文件。manifest 是声明式索引，不是 runtime schema。

## 原则

- 一个 pack 只保存可跨 case 复用、已脱敏的领域方法与边界。
- 不保存真实样本、trace/dump/capture、payload、flag、凭据、客户信息、绝对 case 路径或 case 进度。
- pack 只能建议职责；durable member 由 Commander 按需创建，active team 仍受 3 名执行成员 + 1 名 Reviewer 上限约束。
- heavy action 不由 manifest 自动授权。必须同时落在明确 case scope、获得针对具体动作的用户确认，并通过 Claude Code 工具权限；范围、预算或副作用漂移时停止。
- learning 只从 accepted finding/review 提炼，经 Reviewer 检查证据、通用性、冲突、重复与脱敏，再由用户确认完整 exact patch。
- selected pack 与 `common/**` policy closure 从同一 exact Git revision 物化到 case-local 只读 snapshot；case 日常不读取 mutable source clone。

## 最小结构

```text
packs/<pack>/
  manifest.yml
  CLAUDE.local.snippet.md
  references/<pack>/
    README.md
    agent-team.md
    workflow-template.md
    toolchain-router.md
    task-handoff.template.md
  policies/
    README.md
  tooling/
    README.md
    catalog.yml
    recipes/*.md
```

`README.md` 是按需路由入口，不是长必读清单。`agent-team.md` 只描述领域职责、owner/verifier/Reviewer 分工和产出；`workflow-template.md` 描述研究阶段与停止条件；`toolchain-router.md` 描述工具选择、风险与证据要求；tooling recipe 不执行动作，只给出可审查步骤。

## Manifest

新 pack 从 `packs/_template/manifest.yml` 复制，使用当前声明格式：

```yaml
schemaVersion: 2
name: <pack-name>
version: 1.0.0
description: <one-line>
maturity: declarative-vnext

entrypoints:
  router: references/<pack>/README.md
  team: references/<pack>/agent-team.md
  workflow: references/<pack>/workflow-template.md
  tooling: references/<pack>/toolchain-router.md

references: []
templates: []
policies: []
tooling: []

teamHints:
  suggestedMembers: 1-2
  reviewer: on-important-finding-or-delivery
  ownerPerQuestion: 1
  verifierPerQuestion: 1
  durableTeamLimit: 3-executors-plus-1-reviewer

heavyActions:
  - id: <action>
    title: <human-readable>
    risk: high
    requiredAuthorization: exact-case-scope-and-specific-user-confirmation
    requiredToolPermission: true
    stopConditions: [scope-drift, budget-exhausted, unexpected-side-effect]

learningTargets: []
denyPatterns:
  - real-case-identifiers
  - raw-artifacts
  - credentials-or-tokens
  - absolute-case-paths
  - customer-environment-details
budgets:
  CLAUDE.local.md: 8192
  defaultMarkdown: 16384
```

要求：

- 所有路径相对、存在且位于 pack root 或显式 `../../common/policies/` closure。
- `entrypoints` 指向可读的按需入口。
- `teamHints` 是建议，不是 task/owner database。
- `heavyActions` 只声明风险、授权条件与止损条件，不创建 gate、receipt、lease 或 durable autonomy 状态。
- `learningTargets` 是可生成 exact patch 的 tracked Markdown 目标集合；未知路径不自动接受。
- `denyPatterns` 必须覆盖领域特有的 case 私有信息和原始 artifact。
- `_template` 只用于 authoring，不能作为真实 case 的 selected pack。

## 实施步骤

1. 判断该能力是否确实需要独立领域 pack；单个方法优先更新现有 reference 或 tooling recipe。
2. 从 `packs/_template/` 复制最小结构并替换名称、路径和领域边界。
3. 先写 router、workflow、team 与 tooling 入口，再补最少必要 recipe。
4. 为每个 heavy action 写明 exact scope、具体用户确认、工具权限和停止条件。
5. 检查所有示例只有 synthetic placeholder，不含真实 case 数据。
6. 运行 focused repository contract、完整 Go suite、`go vet ./...` 与 `git diff --check`。
7. 经验回流遵循 `vnext/learning-feedback.md`；不直接从 case 整文覆盖 pack，不自动 commit/push。

## 验证

```text
go test -count=1 -run 'TestAllPackManifestsUseThinDeclarativeShape|TestPackAndCommonSourcesDoNotExposeLegacyCommands' ./internal/steamai/vnextcontract
go test -count=1 -p=2 -timeout=30m ./...
go vet ./...
git diff --check
```

还应人工确认：

- manifest 声明路径完整且没有旧 runtime、lane、ledger 或命令入口；
- pack/common snapshot 来自同一 revision；
- team 文档没有让成员自行创建 durable member 或改变 case 授权；
- Reviewer 不修改原 evidence/finding；
- heavy action 不能由模板、消息或 `CLAUDE.md` 自动授权；
- learning candidate 与 patch 已脱敏、单目标且可复核。
