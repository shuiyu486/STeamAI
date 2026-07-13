# Pack authoring guide

## 读取指南

本文件用于新增或维护 `packs/<pack>`。如果只使用现有 `vmp-re`，不需要先读全文。

新增 pack 前先确认：该能力是否真的是新的安全领域，还是应作为现有 pack（当前主要是 `vmp-re`）的 reference、tooling recipe 或 common policy 改进。候选方向可以包括 Web/API 安全、恶意样本分析、漏洞研究、CTF/靶场、Android native、OLLVM 或通用二进制分析，但不要盲目复制 `vmp-re`。

## 最小 pack 结构

```text
packs/<pack>/
  manifest.yml
  CLAUDE.local.snippet.md
  references/<pack>/README.md
  references/<pack>/agent-team.md
  references/<pack>/workflow-template.md
  references/<pack>/toolchain-router.md
  policies/README.md
  policies/*.overlay.md
  prompts/*.md
  tooling/README.md
  tooling/catalog.yml
  tooling/recipes/*.md
```

不是每个目录第一天都要完整实现，但 `manifest.yml`、`README.md`、`agent-team.md`、`workflow-template.md`、`toolchain-router.md` 必须先有清晰边界。

## manifest 必填方向

`manifest.yml` 是 pack 的单一事实源。至少应声明：

```yaml
schemaVersion: 1
name: <pack-name>
version: <semver-like>
description: <one-line>
# maturity: mature | skeleton | template | experimental
maturity: skeleton
managedFiles:
  - references/<pack>/README.md
  - references/<pack>/agent-team.md
  - references/<pack>/workflow-template.md
  - references/<pack>/toolchain-router.md
templateFiles:
  - references/<pack>/task-handoff.template.md
localNeverOverwrite:
  - CLAUDE.local.md
  - references/<pack>/task-handoff.md
  - tools.local.yml
managedBlock:
  file: CLAUDE.local.md
  blockId: <pack>:router
  source: CLAUDE.local.snippet.md
syncPolicy:
  managedFiles: overwrite-with-backup
  templateFiles: create-if-missing
  localFiles: never-overwrite
workstreamDefaults:
  defaultAuthorityLane: main
  defaultStartLaneType: feature
  handoffPath: references/<pack>/task-handoff.md
  backupRoot: .rekit/backups/sync
  requestDefaultTargetLane: main
authorityFiles:
  - references/<pack>/task-handoff.md
commonPolicies: []
policyOverlays: []
subagentRoutes:
  - id: <pack>:bounded-review
    taskTypes: candidate-review,evidence-review,tooling-review,security-assessment
    trigger: fixed-boundary read-only review for candidate evidence or tooling notes
    shardBasis: item
    targetItemsPerAgent: 4
    maxParallel: 3
    reference: references/<pack>/agent-team.md
    policyOverlay:
    subagentPermissions: read-only
    mainAgentOwns: ledger-writeback,validation,handoff-update,authority-confirmation
    outputContract: item,decision,confidence,evidence,risk,next_action,tier_used,tool_scope,defer_reason
toolingFiles: []
promoteFiles:
  - references/<pack>/README.md
  - references/<pack>/agent-team.md
  - references/<pack>/workflow-template.md
  - references/<pack>/toolchain-router.md
promptFiles: []
laneTypes:
  - id: main
    title: 主线
    authority: true
    workspaceRoot: workspace/main
    canWrite: references/<pack>/task-handoff.md
    readOnly: .rekit/facts/**
    outputs: publication,decision,observation
  - id: feature
    title: 功能分析
    authority: false
    workspaceRoot: workspace/features
    canWrite: own-workspace
    readOnly: references/<pack>/**,.rekit/facts/**
    outputs: observation,request,candidate,summary
toolingCandidateSources:
  - references/<pack>/toolchain-router.md
promoteDenyPatterns:
  - "C:\\"
  - "artifacts[\\/]"
  - "captures[\\/]"
  - "[A-Za-z0-9_.-]*trace[A-Za-z0-9_.-]*\\.(csv|jsonl|log|txt|bin)"
  - "[A-Za-z0-9_.-]*dump[A-Za-z0-9_.-]*\\.(dmp|bin|raw|exe|dll)"
  - "\\.dmp\\b"
  - "0x[0-9A-Fa-f]{6,}"
  - "ctx[0-9]+"
  - "round[0-9]+"
  - "Task #[0-9]+"
budgets:
  defaultMarkdown: 16384
```

所有路径必须是相对路径，不能越出 pack root 或 case root。

## 文件职责

| 文件 | 职责 |
|---|---|
| `references/<pack>/README.md` | case 内按需路由入口，不承载长教程全文。 |
| `agent-team.md` | pack 默认 subagent routes、packet 输出契约和 review-first 合并边界。 |
| `workflow-template.md` | 领域主流程和验证路线。 |
| `toolchain-router.md` | 工具选择、状态、升级门禁和止损条件。 |
| `CLAUDE.local.snippet.md` | case-local router block，不写 case 私有事实。 |
| `policies/*.overlay.md` | 对 common policy 的领域化补充。 |
| `prompts/*.md` | Agent 会话角色提示。 |
| `tooling/catalog.yml` | 工具 capability card。 |
| `tooling/recipes/*.md` | 按任务阶段组织的工具用法。 |

## `_template` 骨架

本仓库提供 `packs/_template/` 作为新 pack 的起点；已落地的 `packs/web-security/`、`packs/malware-analysis/`、`packs/vuln-research/`、`packs/ctf/`、`packs/unpack-pe/`、`packs/ollvm/`、`packs/android-native/` 与 `packs/generic-binary-re/` 可作为安全领域 skeleton 参考。创建真实 pack 时复制该目录并替换：

- `schemaVersion`、`name`、`version`、`description`、`maturity` 与 `managedBlock.file/blockId/source`；`schemaVersion` 必须显式声明为 `1`，不能缺失或使用未支持版本；`name` / `version` 必须显式声明，不能依赖 runtime 用 pack id 或 `0.0.0` 补齐；`description` 必须显式声明一行用途摘要；managed block 三个字段必须显式声明，不能依赖 runtime 注入默认 host、blockId 或 source。
- `managedFiles`、`templateFiles`、`localNeverOverwrite`、`promoteFiles`、`commonPolicies`、`policyOverlays`、`subagentRoutes`、`toolingFiles`、`promptFiles`、`toolingCandidateSources`、`authorityFiles`、`promoteDenyPatterns`、`heavyToolGates` 与 `laneTypes`；这些 schema-critical list key 必须显式声明，允许空列表的字段也不能缺 key，runtime 不再用 loader 阶段 fallback 或非空检查替代 schema validation。
- `references/template/**` 路径和目录名。
- `subagentRoutes` 的 namespaced route id（例如 `<pack>:bounded-review`）、taskTypes、trigger、shardBasis 和 outputContract；trigger 必须显式非空，说明何时触发该 route；taskTypes、mainAgentOwns 与 outputContract 是逗号/分号分隔列表，不能包含空项。
- `tooling/catalog.yml` 的工具条目。
- `commonPolicies`、`policyOverlays`、`subagentRoutes`、`toolingFiles`、`promptFiles`、`promoteFiles`、`syncPolicy`、`workstreamDefaults`、`toolingCandidateSources`、`authorityFiles`、`promoteDenyPatterns`、budgets、`heavyToolGates.requiresConfirmation` 和 `laneTypes`；可为空的 route/policy/tooling/prompt list 也必须显式声明；`promoteFiles` 必须显式声明允许 case → kit 回流的非空 managed 文件子集，不能依赖 `managedFiles` fallback；`toolingCandidateSources`、`authorityFiles`、`promoteDenyPatterns`、`heavyToolGates` 与 `laneTypes` 必须显式声明且按 contract 保持非空；`syncPolicy` 必须显式声明当前 runtime 支持的 managed/template/local 三项策略，不能缺 key 或使用未支持值；`workstreamDefaults` 必须显式声明默认 authority/start lane、backupRoot 与 request 默认路由，不能依赖 runtime fallback；budgets 必须显式声明 map 与 `defaultMarkdown` 正整数，不能依赖 runtime 注入默认预算；每个 heavy tool gate 必须显式声明 `requiresConfirmation: true`；`laneTypes` 必须显式声明 title、workspaceRoot、authority true/false、canWrite/readOnly/outputs，不能依赖 runtime 注入 lane 名称、workspace 默认值或把缺失 authority 当作 false；新增 pack 应从 `_template` 的通用 baseline 开始再按领域补充。

`_template` 只作为作者模板，不代表可直接用于真实 case 的领域 pack。

## 新 pack 实施步骤

1. 写 `docs` 或 issue 级设计草案，明确 pack 目标和非目标。
2. 从 `packs/_template/` 复制最小目录并改名。
3. 写 `references/<pack>/README.md`、`agent-team.md`、`workflow-template.md`、`toolchain-router.md`。
4. 写 `CLAUDE.local.snippet.md`，只放短 router block。
5. 补 tooling catalog 和至少一个 recipe。
6. 用 `plan-subagents` 验证 route packet / summary，再用临时 case 验证 `init/attach/sync/promote`；新增 skeleton pack smoke 时优先复用 `rekit/tests/pack-smoke-lib.ps1`，让 wrapper 只声明 pack id、safe case prefix、route task type、expected route 和 output contract 字段。
7. 只有两个以上 pack 重复出现相同规则时，才抽到 `common/`、runtime 或测试 helper。

## 禁止

- 不复制 `vmp-re` 全套文档后只替换名字。
- 不把真实样本、客户信息、RVA/VA、trace/dump、artifact 路径写入 pack。
- 不在 pack 中硬编码本机工具路径；使用 `<caseRoot>`、`<toolsRoot>`、`<target>` 占位。
- 不让 pack script 复制 runtime 逻辑；旧脚本只能 wrapper 到 `rekit/rekit.ps1`。
- 不把未验证 case 经验直接写成通用规则。

## 验证标准

更完整的 smoke 选择表见 `rekit/tests/README.md`；本节只列新增/维护 pack 的最低标准。

- `git diff --check` 通过。
- `/rekit packs` 能列出新 pack，且该行 `maturity` 来自 manifest 显式字段，`schema=ok`、route / managed / tooling / authority 计数符合预期；自动化检查可用 `/rekit packs -Format json` 消费同一 inventory。
- `manifest.yml` 路径均为相对路径。
- managed/template/local 边界清晰。
- `subagentRoutes.reference` 指向 managed/template/local 文件，route id 使用 namespaced 形式且唯一，`taskTypes`、`trigger`、`shardBasis`、`targetItemsPerAgent`、`maxParallel`、`subagentPermissions`、`mainAgentOwns`、`outputContract` 齐全，且逗号/分号分隔字段不含空项。
- 新 pack 初始化不会覆盖 case-local 文件。
- skeleton pack smoke 通过 `rekit/tests/pack-smoke-lib.ps1` 或等价验证覆盖 Go/PowerShell doctor、Go init、case doctor、`plan-subagents` route packet、promote review managed-doc candidate 和 no-write 边界；临时 case prefix 不能让 pack 名中的通用词触发 case-specific deny pattern 误拦截；需要全量、子集或机器可读回归时使用 `rekit/tests/pack-smoke-matrix.ps1`（支持 `-Format json`），需要检查 matrix 清单是否覆盖所有 schema-valid skeleton pack 时使用 `-DiscoveryOnly`，修改 matrix 输出契约时运行 `pack-smoke-matrix-selftest.ps1`。
- promote deny patterns 覆盖绝对路径、artifact/capture/trace/dump、地址快照和 case 状态。
