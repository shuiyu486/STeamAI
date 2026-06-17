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
| `/rekit sync` | kit -> case | 模板仓库更新后，把 managed docs / managed block 同步到当前 case。 |
| `/rekit promote` | case -> kit | 将 case 中可复用经验回流为 managed docs 候选或 tooling 候选。 |
| `/rekit doctor` | 只读 | 排障时详细验证结构；日常不必主动运行。 |
| `/rekit repair` | case metadata | 迁移目录后先预览修复；确认后由 Claude 调用 backend `-Apply`。 |

`validate` 仍是 backend/兼容命令，但不是日常主入口。

## 日常工作流

### 1. 继续逆向分析

按 case 的 `CLAUDE.local.md` 和 `references/vmp-re/task-handoff.md` 工作。

如果只更新当前进度、coverage、handler 列表：

```text
更新 references/vmp-re/task-handoff.md
```

这是 case live state，**不要 promote**。

### 2. 同步模板更新到当前 case

在 case 里：

```text
/rekit sync
```

会同步：

```text
references/vmp-re/README.md
references/vmp-re/workflow-template.md
references/vmp-re/progressive-disclosure.md
references/vmp-re/toolchain-router.md
references/vmp-re/singleton-handler-review.md
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

### 3. 回流可复用经验

在 case 里：

```text
/rekit promote
```

它会同时做两类事：

1. managed docs：安全时生成候选；写回 pack 需要你明确确认。
2. tooling：从 case 工具链文档抽象候选，写入 `packs/vmp-re/tooling/candidates/`。

`promote` 很保守：若 managed docs 含真实绝对路径、样本名、RVA/VA、ctx/round 快照、artifact/capture/trace/dump 路径，会阻止直接回流。工具链经验会先脱敏生成 tooling candidate，由你审查后合入正式 tooling 文档。

`promote` 只允许作用于已经 `attach/init` 的 case，避免从普通目录误回流到 pack。

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
- `packs/<pack>/manifest.yml` 是 managed/local/tooling/budget/promote 规则的单一事实源。
- case-local `.claude/skills/rekit/SKILL.md` 只是 thin shim，不维护业务逻辑。
- `.re-template.yml` 只保留兼容旧入口；新状态看 `.rekit/instance.yml`。
- 不默认安装用户级 skill。
- 不自动 commit / push。
