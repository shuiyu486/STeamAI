# Pack authoring guide

## 读取指南

本文件用于新增或维护 `packs/<pack>`。如果只使用现有 `vmp-re`，不需要先读全文。

新增 pack 前先确认：该能力是否真的是新领域，还是应作为 `vmp-re` 的 reference、tooling recipe 或 common policy 改进。

## 最小 pack 结构

```text
packs/<pack>/
  manifest.yml
  CLAUDE.local.snippet.md
  references/<pack>/README.md
  references/<pack>/workflow-template.md
  references/<pack>/toolchain-router.md
  policies/README.md
  policies/*.overlay.md
  prompts/*.md
  tooling/README.md
  tooling/catalog.yml
  tooling/recipes/*.md
```

不是每个目录第一天都要完整实现，但 `manifest.yml`、`README.md`、`workflow-template.md`、`toolchain-router.md` 必须先有清晰边界。

## manifest 必填方向

`manifest.yml` 是 pack 的单一事实源。至少应声明：

```yaml
schemaVersion: 1
name: <pack-name>
version: <semver-like>
description: <one-line>
managedFiles: []
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
toolingFiles: []
promoteFiles: []
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
promoteDenyPatterns: []
budgets:
  defaultMarkdown: 16384
```

所有路径必须是相对路径，不能越出 pack root 或 case root。

## 文件职责

| 文件 | 职责 |
|---|---|
| `references/<pack>/README.md` | case 内按需路由入口，不承载长教程全文。 |
| `workflow-template.md` | 领域主流程和验证路线。 |
| `toolchain-router.md` | 工具选择、状态、升级门禁和止损条件。 |
| `CLAUDE.local.snippet.md` | case-local router block，不写 case 私有事实。 |
| `policies/*.overlay.md` | 对 common policy 的领域化补充。 |
| `prompts/*.md` | Agent 会话角色提示。 |
| `tooling/catalog.yml` | 工具 capability card。 |
| `tooling/recipes/*.md` | 按任务阶段组织的工具用法。 |

## `_template` 骨架

本仓库提供 `packs/_template/` 作为新 pack 的起点。创建真实 pack 时复制该目录并替换：

- `name`、`description`、`blockId`。
- `references/template/**` 路径和目录名。
- `tooling/catalog.yml` 的工具条目。
- `commonPolicies`、`policyOverlays`、`promoteDenyPatterns` 和 budgets。

`_template` 只作为作者模板，不代表可直接用于真实 case 的领域 pack。

## 新 pack 实施步骤

1. 写 `docs` 或 issue 级设计草案，明确 pack 目标和非目标。
2. 从 `packs/_template/` 复制最小目录并改名。
3. 写 `references/<pack>/README.md`、`workflow-template.md`、`toolchain-router.md`。
4. 写 `CLAUDE.local.snippet.md`，只放短 router block。
5. 补 tooling catalog 和至少一个 recipe。
6. 用临时 case 验证 `init/attach/sync/promote`。
7. 只有两个以上 pack 重复出现相同规则时，才抽到 `common/` 或 runtime。

## 禁止

- 不复制 `vmp-re` 全套文档后只替换名字。
- 不把真实样本、客户信息、RVA/VA、trace/dump、artifact 路径写入 pack。
- 不在 pack 中硬编码本机工具路径；使用 `<caseRoot>`、`<toolsRoot>`、`<target>` 占位。
- 不让 pack script 复制 runtime 逻辑；旧脚本只能 wrapper 到 `rekit/rekit.ps1`。
- 不把未验证 case 经验直接写成通用规则。

## 验证标准

- `git diff --check` 通过。
- `manifest.yml` 路径均为相对路径。
- managed/template/local 边界清晰。
- 新 pack 初始化不会覆盖 case-local 文件。
- promote deny patterns 覆盖绝对路径、artifact/capture/trace/dump、地址快照和 case 状态。