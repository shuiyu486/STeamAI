# re-context-kits

`re-context-kits` 是面向网络安全研究与安全工程任务的 Claude Code Agent Team 框架，用于把多 Agent 协作、领域工具链、证据账本、工作线管理、验证门禁和可复用安全领域 pack 组织成可持续迭代的 case workspace。

当前阶段，它已经提供 `/rekit` case 管理、首个成熟 pack `vmp-re`、非 RE pack 骨架 `web-security`、`malware-analysis`、`vuln-research` 与 `ctf`、工作线协同、handoff、sync/promote 和 tooling 经验沉淀；`vmp-re` 是验证框架的第一个重点领域，不是最终边界。长期目标是逐步扩展到逆向工程、恶意样本分析、漏洞研究、Web/API 安全评估、授权测试/靶场/CTF、Android native、OLLVM 等多类安全任务。

当前项目不是全自动脱壳器、自动逆向引擎、自动漏洞挖掘器、自动恶意样本分析平台或通用自动渗透平台；它优先提供可审计、可交接、review-first 的 Agent Team 底座。

一句话：**用户日常只用 `/rekit`；维护者迭代 runtime、pack、policy、tooling 和文档。脚本只是 `/rekit` 背后的 backend。**

## 项目路线

- 新架构使用与旧 case 兼容：`docs/agent-team-usage.md`
- 参考资料吸收映射：`docs/reference-absorption.md`
- 长期愿景与阶段实施方案：`docs/vision.md`
- 当前架构说明：`docs/design.md`
- 后续批次计划：`docs/batch-plan.md`
- pack 编写指南：`docs/pack-authoring.md`（新 pack 可从 `packs/_template/` 复制；`packs/web-security/`、`packs/malware-analysis/`、`packs/vuln-research/` 与 `packs/ctf/` 是首批非 RE pack 骨架）
- evidence / intervention 账本草案：`docs/evidence-ledger.md`
- 半自动 orchestration 计划：`docs/orchestration-plan.md`
- Agent Team rollout 计划：`docs/agent-team-rollout-plan.md`
- VMP/RE Agent Team 工作方式：`packs/vmp-re/references/vmp-re/agent-driven-re.md`
- sync/promote 机制：`docs/promote-sync.md`
- case 迁移说明：`docs/case-migration.md`
- Go backend 渐进迁移：`docs/go-runtime-migration.md`

## 如果你在维护本仓库

本仓库本身不是具体安全 case，也不是具体 RE case。维护时优先看根目录 `CLAUDE.md` 和 `docs/vision.md`，再按职责修改：

- `/rekit` skill：`.claude/skills/rekit/SKILL.md`
- runtime：`rekit/rekit.ps1`、`rekit/lib/*.ps1`、`cmd/rekit/**`、`internal/rekit/**`
- 领域 pack：`packs/<pack>/**`
- 通用 policy / prompt：`common/**`
- 设计与路线：`docs/**`

不要因为下面的 case 初始化示例而在本仓库内伪造 case state；只有验证 `init/attach/sync/promote` 行为时才创建临时 case。

## 使用方式：把 kit 接入安全 case（当前以 `vmp-re` 为例）

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
| `/rekit status` | 只读 | 看当前 case 绑定状态；若目录被移动，只提示，不修复；`-Format json` 输出机器可读 status envelope。 |
| `/rekit packs` | 只读 | 维护者查看当前 kit 内所有 pack 的成熟度、schema、route、managed/tooling 和 authority lane 概览；`-Format json` 输出机器可读 inventory。 |
| `/rekit overview` | case-local 状态 | 显示项目概览、主线/支线、共享事实统计和下一步建议；维护自动化可用 `-Format json` 消费 overview envelope；只表示总览，不代表当前会话已选择工作线。 |
| `/rekit continue main` | case-local 自动整理 | 明确接手主线并整理相关状态；多工作线时不要用无参数 `continue` 盲猜；维护自动化可用 `-WhatIf -Format json` 消费非写入 continue 计划。 |
| `/rekit continue <name>` | case-local 自动整理 | 明确接手某条功能支线，只整理该支线的 workspace/outbox 并刷新接续提示；`-WhatIf -Format json` 可预览收集、路由和 authority append 计划。 |
| `/rekit start <name>` | case-local 状态 | 创建或进入一个功能支线，例如 `/rekit start login`；支线只写自己的工作区；维护自动化可用 `-WhatIf -Format json` 消费非写入 start 计划。 |
| `/rekit handoff` | case-local 状态 | 生成项目级接手索引 `.rekit/handovers/latest.md`；维护自动化可用 `-WhatIf -Format json` 消费写入预览；不代表某个会话。 |
| `/rekit handoff <name>` | case-local 状态 | 生成指定工作线接手文档，例如 `/rekit handoff main` 或 `/rekit handoff login`；`-WhatIf -Format json` 可预览目标工作线 handoff 写入计划。 |
| `/rekit sync` | kit -> case | 默认生成同步审查包；确认后才用 `-Apply` 写入 managed docs / managed block。 |
| `/rekit promote` | case -> kit | 默认生成回流审查包；确认后才用 `-CreateCandidates` 生成候选或用 `-Apply` 写回 pack。 |
| `/rekit doctor` | 只读 | 排障时详细验证结构；日常不必主动运行；维护自动化可用 `-Format json` 消费验证 rows。 |
| `/rekit repair` | case metadata | 迁移目录后先预览修复；确认后由 Claude 调用 backend `-Apply`。 |

`validate` 和 `plan-subagents` 仍是 backend/内部命令，不是日常主入口；`packs` 是维护者/排障入口，用于多 pack 发现和矩阵验证；`note -List -Format json` 可供维护自动化读取 ledger events。

## 日常工作流

### 1. 看当前项目状态

```text
/rekit overview
```

它会展示：

- 当前主线和功能支线；
- 共享事实、request、candidate、publication 统计；
- 未决 candidate、pending-gate、最近 verification / decision 等 review loop 摘要；
- 需要人工确认的事项；
- 推荐下一步。

### 2. 选择并继续某条工作线

```text
/rekit continue main
/rekit continue login
```

`overview` 只是项目总览，不代表当前会话已经选择主线或支线。多条 open 工作线时，无参数 `/rekit continue` 只会列出选择，不会盲目推进。需要自动化预览时可先运行 `/rekit continue login -WhatIf -Format json` 获取只读计划。明确选择后，它会自动整理对应工作线的 case-local 状态：收集 outbox/workspace 事件、发布低风险共享事实、路由 request、验证候选并刷新接续提示。

安全边界：只有 candidate 同时满足 evidence、accepted verifier、confidence 阈值、CSV schema、无冲突、backup、diff、max rows 时，才允许自动 append authority CSV；覆盖/删除 authority、冲突、schema change、外部副作用或破坏性动作仍必须问用户。

### 3. 开一个功能支线

```text
/rekit start login
```

这会创建一个功能支线，例如 `feature-login`。需要自动化预览时可先运行 `/rekit start login -WhatIf -Format json` 获取只读写入计划。功能支线用于专项分析、证据收集、候选结论和 request；它默认不能写 confirmed CSV、`routine_ir.*` 或 `references/vmp-re/task-handoff.md`。

主线/支线不是级别高低，而是写入权限不同：

| 类型 | 职责 | 可写 |
|---|---|---|
| 主线 | 维护最终结论、验证和长期 handoff | canonical 文件 |
| 功能支线 | 分析某个功能、收集证据、提出候选和 request | 自己的 workspace |

### 4. 换新会话

项目级索引用：

```text
/rekit handoff
```

指定工作线接手文档用：

```text
/rekit handoff main
/rekit handoff login
```

它会生成：

```text
.rekit/handovers/latest.md                 # 项目级索引
.rekit/handovers/devirt-main-latest.md     # 主线接手
.rekit/handovers/feature-login-latest.md   # 功能支线接手
```

新会话应先明确接哪条工作线，例如：

```text
按 .rekit/handovers/devirt-main-latest.md 接手，然后 /rekit continue main。
```

工作线接手文档会附带本工作线的 workspace packet、最近 verification、decision、pending-gate、intervention 和 rollback 摘要，便于新会话看到 reviewer verdict 到 main decision 的状态。

这些接手文档只引用 `references/vmp-re/task-handoff.md`，不会覆盖它。

### 5. 同步模板更新到当前 case

在 case 里：

```text
/rekit sync
```

默认只生成 `.rekit/reviews/<timestamp>-sync/packet.json`、`summary.md` 和 bounded diff。Claude 复核冲突与收益、你确认具体范围后，才执行写入型同步（backend 为 `sync -Apply`）。

写入型同步会同步：

```text
references/vmp-re/README.md
references/vmp-re/agent-driven-re.md
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

默认 `sync -Apply` 不会覆盖已存在的本地 template files；只有显式 `-Force` 才会在写入前备份并覆盖 manifest 声明的本地模板目标。

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
| `.rekit/facts/*.jsonl` | append-only ledger：observation、hypothesis、candidate、verification、decision、intervention、rollback、publication、request。 |
| `.rekit/runs/<run-id>/digest.md` | `/rekit continue` 每轮摘要，记录 inputs、route、packet refs、outputs、decisions、open risks。 |
| `.rekit/handovers/latest.md` | 项目级接手索引。 |
| `.rekit/handovers/<laneId>-latest.md` | 指定工作线接手文档。 |

字段名中仍保留 `lane`，这是内部 schema 名称；用户层统一称“工作线 / 主线 / 功能支线”。

## 高级/内部：子 agent 分片计划

`/rekit plan-subagents` 是内部只读计划器，用于主 agent 或自动流程在批量复核时按 handler、trace、tooling diff 等固定边界生成分片审查产物。它不启动 agent，也不修改 managed docs 或项目源文件；日常不需要用户手动调用。

生成的 `packet.json` / `summary.md` 会标出 route 选择原因、每个 shard 的初始 `planned` 状态、spawn/merge 责任、verdict 写回入口和被 runtime 阻止的动作（例如 runtime 不自动 spawn、子 agent 不写文件）。这些字段只帮助主会话审计 bounded dispatch，不代表 runtime 会启动 reviewer。

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
cmd/rekit/main.go                  # Go backend CLI，默认不直接作为用户入口
rekit/tests/facade-smoke.ps1       # façade 委托回归 smoke
packs/vmp-re/scripts/bootstrap.ps1
packs/vmp-re/scripts/update.ps1
packs/vmp-re/scripts/validate.ps1
packs/vmp-re/scripts/promote.ps1
```

如果 README 前面能用 `/rekit` 表达，就不要让用户手动跑脚本。

## 架构边界

- `/rekit` 是用户入口。
- `rekit/rekit.ps1` 是稳定 PowerShell façade / fallback，只是 backend。
- Go backend 位于 `cmd/rekit/**` 与 `internal/rekit/**`；默认不启用，维护者显式设置 `REKIT_GO_ENABLE=1` 后才委托安全集合（status、doctor/validate、sync/promote review-only、sync `-Apply -WhatIf -Format json`、promote `-CreateCandidates -WhatIf -Format json`、promote `-Apply -WhatIf -Format json`、gate -WhatIf、attach `-WhatIf -Format json`、repair `-Format json`、init/bootstrap `-WhatIf -Format json`、已初始化 case 的 overview `-Format json`、note `-List -Format json`，以及 start/handoff/continue 的 `-WhatIf -Format json` 非写入预览）。Go CLI 另有手动验证路径，例如 `overview` 只读、`start -WhatIf/-Apply`、`handoff -WhatIf/-Apply`、`plan-subagents` review artifact、`attach -WhatIf/-Apply`、`repair -WhatIf/-Apply`、`sync -Apply/-Apply -WhatIf`、`init/bootstrap -WhatIf/-Apply`、`promote -CreateCandidates`（可配 `-WhatIf` 预览）和 `promote -Apply/-Apply -WhatIf`；文本工作线命令、内部命令、attach/repair/init/bootstrap 文本预览、sync/promote candidate/apply 实际写入、note append 和写入命令暂不经 façade 委托。
- 工作流 runtime 已拆为 `rekit/lib/B3.*.ps1`，按 Core / State / Policy / Lane / Auto / Commands 分层。
- `packs/<pack>/manifest.yml` 是 managed/local/tooling/budget/promote 规则的单一事实源。
- case-local `.claude/skills/rekit/SKILL.md` 只是 thin shim，不维护业务逻辑。
- `.re-template.yml` 只保留兼容旧入口；新状态看 `.rekit/instance.yml`。
- 不默认安装用户级 skill。
- 不自动 commit / push。
