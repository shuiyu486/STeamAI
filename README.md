# re-context-kits

`re-context-kits` 是给 RE case 使用的 Claude Code 上下文模板与 tooling 资产仓库。

一句话：**你平时只用 `/rekit`，不要手动跑脚本；脚本只是 `/rekit` 背后的 backend。**

## 最短使用方式

### 1. 第一次 clone 后

进入 kit 仓库启动 Claude Code：

```powershell
cd <workspaceRoot>\kits\re-context-kits
claude
```

然后直接对 Claude 说：

```text
/rekit init -Target <workspaceRoot>\cases\<caseName> -Pack vmp-re -ProjectName <caseName>
```

或已有 case 接入：

```text
/rekit attach -Target <workspaceRoot>\cases\<caseName> -Pack vmp-re
```

> 这里不需要你手动执行底层脚本。`/rekit` 会调用内部 runtime。

### 2. 之后每天在 case 里

进入 case 启动 Claude Code：

```powershell
cd <workspaceRoot>\cases\<caseName>
claude
```

日常只需要记：

```text
/rekit status
/rekit overview
/rekit continue
/rekit start <name>
/rekit handoff
/rekit sync
/rekit promote
```

排障时再用：

```text
/rekit doctor
```

## 目录模型

推荐 workspace 结构：

```text
<workspaceRoot>\
  kits\
    re-context-kits\              # 模板仓库；canonical /rekit + packs + tooling
  cases\
    <caseName>\                   # 具体 case
  tools\                          # 第三方工具
  shared-artifacts\               # 大文件/共享产物
```

`kits/` 和 `cases/` 是 sibling，不是包含关系。这样多个 case 可以复用同一套模板，同时避免样本、trace、dump、大文件混入模板仓库。

## 为什么第一次要在 kit 里启动 Claude Code

新 case 还没有：

```text
<caseRoot>\.claude\skills\rekit\SKILL.md
<caseRoot>\.rekit\instance.yml
<caseRoot>\.rekit\state.json
```

所以第一次需要在 kit 仓库里使用 canonical `/rekit` 完成 `init/attach`。

完成后，case 里会有 thin shim。以后你在 case 目录启动 Claude Code，也能直接使用 `/rekit`。

## 常用命令

| 命令 | 方向 | 什么时候用 |
|---|---|---|
| `/rekit status` | 只读 | 看当前 case 绑定状态；若目录被移动，只提示，不修复。 |
| `/rekit overview` | case-local 状态 | 显示项目概览、主线/支线、共享事实统计和下一步建议；首次运行会初始化缺失的本地 rekit 状态。 |
| `/rekit continue` | case-local 自动整理 | 收集支线产物、验证候选、路由 request、发布低风险共享事实并刷新接续提示；冲突或高风险写入仍会停下。 |
| `/rekit start <name>` | case-local 状态 | 创建一个新的功能支线，例如 `/rekit start login`；支线只写自己的工作区。 |
| `/rekit handoff` | case-local 状态 | 生成 `.rekit/handovers/latest.md` 新会话接手包，不覆盖 `task-handoff.md`。 |
| `/rekit sync` | kit -> case | 默认生成同步审查包；确认后才用 `-Apply` 写入 managed docs / managed block。 |
| `/rekit promote` | case -> kit | 默认生成回流审查包；确认后才用 `-CreateCandidates` 生成候选或用 `-Apply` 写回 pack。 |
| `/rekit doctor` | 只读 | 排障时详细验证结构；日常不必主动运行。 |
| `/rekit repair` | case metadata | 迁移目录后先预览修复；确认后由 Claude 调用 backend `-Apply`。 |

`validate` 和 `plan-subagents` 仍是 backend/内部命令，不是日常主入口。

## 日常工作流

### 1. 看当前项目状态

```text
/rekit overview
```

它会展示：

- 当前主线和功能支线；
- 共享事实、request、candidate、publication 统计；
- 需要人工确认的事项；
- 推荐下一步。

### 2. 继续推进当前项目

```text
/rekit continue
```

它会自动整理 case-local 状态：收集支线 outbox/workspace 事件、发布低风险共享事实、路由 request、验证候选并刷新接续提示。

安全边界：只有 candidate 同时满足 evidence、accepted verifier、confidence 阈值、CSV schema、无冲突、backup、diff、max rows 时，才允许自动 append authority CSV；覆盖/删除 authority、冲突、schema change、外部副作用或破坏性动作仍必须问用户。

### 3. 开一个功能支线

```text
/rekit start login
```

这会创建一个功能支线，例如 `feature-login`。功能支线用于专项分析、证据收集、候选结论和 request；它默认不能写 confirmed CSV、`routine_ir.*` 或 `references/vmp-re/task-handoff.md`。

主线/支线不是级别高低，而是写入权限不同：

| 类型 | 职责 | 可写 |
|---|---|---|
| 主线 | 维护最终结论、验证和长期 handoff | canonical 文件 |
| 功能支线 | 分析某个功能、收集证据、提出候选和 request | 自己的 workspace |

### 4. 换新会话

```text
/rekit handoff
```

它会生成：

```text
.rekit/handovers/latest.md
.rekit/handovers/<timestamp>.md
```

新会话里直接说：

```text
按 .rekit/handovers/latest.md 接手继续。
```

这个接手包只引用 `references/vmp-re/task-handoff.md`，不会覆盖它。

### 5. 同步模板更新到当前 case

在 case 里：

```text
/rekit sync
```

默认只生成 `.rekit/reviews/<timestamp>-sync/packet.json`、`summary.md` 和 bounded diff。Claude 复核冲突与收益、你确认具体范围后，才执行写入型同步（backend 为 `sync -Apply`）。

写入型同步会同步：

```text
references/vmp-re/README.md
references/vmp-re/workflow-template.md
references/vmp-re/progressive-disclosure.md
references/vmp-re/toolchain-router.md
references/vmp-re/singleton-handler-review.md
references/vmp-re/lane-collaboration.md
CLAUDE.local.md 中的 managed router block
```

不会覆盖：

```text
references/vmp-re/task-handoff.md
tools.local.yml
captures/**
artifacts/**
CLAUDE.local.md 中 block 外的 case 私有内容
```

`sync` 只允许作用于已经 `attach/init` 的 case。若目标目录拼错或还未绑定，会直接失败，不会静默创建假 case。

### 6. 回流可复用经验

在 case 里：

```text
/rekit promote
```

默认只生成 `.rekit/reviews/<timestamp>-promote/packet.json`、`summary.md`、bounded diff 和安全的脱敏 preview。Claude 复核后，你再选择明确写入动作：

1. `-CreateCandidates`：生成 managed docs 候选或 tooling candidate。
2. `-Apply`：按已确认内容写回 pack。

`promote` 很保守：若 managed docs 含真实绝对路径、样本名、RVA/VA、ctx/round 快照、artifact/capture/trace/dump 路径，会阻止直接回流。工具链经验只有在脱敏后不再命中 deny pattern 时才写 sanitized preview；候选由你审查后合入正式 tooling 文档。

`promote` 只允许作用于已经 `attach/init` 的 case，避免从普通目录误回流到 pack。

## 内部状态模型

日常不用理解这些文件，但排障或 review 时可能会看到：

| 路径 | 内容 |
|---|---|
| `.rekit/board.json` | 项目概览的机器状态。 |
| `.rekit/lanes/<id>/` | 每条工作线的事件、任务、inbox/outbox 和接续提示。 |
| `.rekit/facts/*.jsonl` | 共享事实、request、candidate、publication、decision。 |
| `.rekit/runs/<run-id>/digest.md` | `/rekit continue` 每轮摘要。 |
| `.rekit/handovers/latest.md` | 新会话接手包。 |

字段名中仍保留 `lane`，这是内部 schema 名称；用户层统一称“工作线 / 主线 / 功能支线”。

## 高级/内部：子 agent 分片计划

`/rekit plan-subagents` 是内部只读计划器，用于主 agent 或自动流程在批量复核时按 handler、trace、tooling diff 等固定边界生成分片审查产物。它不启动 agent，也不修改 managed docs 或项目源文件；日常不需要用户手动调用。

## 工具经验保存在哪里

工具经验不会只留在当前 case。现在分两层：

| 层级 | 路径 | 内容 |
|---|---|---|
| 通用 tooling 资产 | `packs/vmp-re/tooling/` | 工具 catalog、recipes、脚本模板化清单、补丁/止损经验。 |
| 当前 case 状态 | `<caseRoot>/references/vmp-re/toolchain-router.md` | 当前样本具体脚本、路径、工具结论和状态。 |

通用 tooling 资产包括：

```text
packs/vmp-re/tooling/catalog.yml
packs/vmp-re/tooling/recipes/public-tool-triage.md
packs/vmp-re/tooling/recipes/lane-collaboration.md
packs/vmp-re/tooling/recipes/vmenter-context-probe.md
packs/vmp-re/tooling/recipes/unicorn-trace.md
packs/vmp-re/tooling/recipes/focused-handler-review.md
packs/vmp-re/tooling/recipes/value-flow-mining.md
packs/vmp-re/tooling/recipes/ida-x64dbg-mcp.md
packs/vmp-re/tooling/scripts/README.md
packs/vmp-re/tooling/patches/vmpimportfixer-timeout-and-quiet-log.md
```

原则：具体样本名、RVA、ctx、coverage 留在 case；可复用工具路线、脚本接口、短测/止损经验进 tooling。

## 迁移已有 case 到新目录

推荐流程：**先复制，确认修复 metadata，再验证新目录，最后归档旧目录**。

### 1. 复制 case 目录

先关闭正在使用该 case 的 Claude Code、IDA、x64dbg、trace 脚本等进程：

```powershell
robocopy <oldCaseRoot> <newCaseRoot> /E
```

### 2. 在新目录检查状态

```powershell
cd <newCaseRoot>
claude
```

然后：

```text
/rekit status
```

如果 `.rekit/instance.yml` 里的旧 `projectRoot` 和当前目录不一致，`status` 只会提示，不会静默修改。

### 3. 确认后修复 metadata

确认这是你预期的迁移后：

```text
/rekit repair
```

`repair` 默认只预览。需要写入时，直接告诉 Claude：

```text
确认修复，执行 repair -Apply
```

`repair -Apply` 会更新：

```text
.rekit/instance.yml
.re-template.yml
.claude/skills/rekit/SKILL.md
```

### 4. 排障验证

```text
/rekit doctor
```

### 5. 检查旧绝对路径

迁移后还要搜索只属于旧 case 根目录的绝对路径：

```text
CLAUDE.local.md
.re-template.yml
references/vmp-re/task-handoff.md
自写脚本中的 PROJECT_ROOT / workdir / output path
```

目标样本路径如果没有变化，不需要改。

## 后端脚本什么时候用

正常情况下不用。

这些入口只是为了自动化、CI、排障或旧流程兼容：

```text
rekit/rekit.ps1
packs/vmp-re/scripts/bootstrap.ps1
packs/vmp-re/scripts/update.ps1
packs/vmp-re/scripts/validate.ps1
packs/vmp-re/scripts/promote.ps1
```

如果 README 前面能用 `/rekit` 表达，就不要让用户手动跑脚本。

## 架构边界

- `/rekit` 是用户入口。
- `rekit/rekit.ps1` 是确定性 runtime，只是 backend。
- 工作流 runtime 已拆为 `rekit/lib/B3.*.ps1`，按 Core / State / Policy / Lane / Auto / Commands 分层。
- `packs/<pack>/manifest.yml` 是 managed/local/tooling/budget/promote 规则的单一事实源。
- case-local `.claude/skills/rekit/SKILL.md` 只是 thin shim，不维护业务逻辑。
- `.re-template.yml` 只保留兼容旧入口；新状态看 `.rekit/instance.yml`。
- 不默认安装用户级 skill。
- 不自动 commit / push。
