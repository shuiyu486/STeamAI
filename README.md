# re-context-kits

`re-context-kits` 是给 RE case 使用的 Claude Code 上下文模板与 tooling 资产仓库。

一句话：**你平时在具体 case 里工作，直接用 `/rekit`；本仓库保存通用模板、工具经验和可回流改进。**

## 当前 streamfab-vmp 怎么用

当前 case 已经绑定到本仓库：

```text
case:         C:\AI\m_projects\RE\cases\streamfab-vmp
kit:          C:\AI\m_projects\RE\kits\re-context-kits
pack:         vmp-re
case shim:    C:\AI\m_projects\RE\cases\streamfab-vmp\.claude\skills\rekit\SKILL.md
case state:   C:\AI\m_projects\RE\cases\streamfab-vmp\.rekit\instance.yml
```

进入 case 后启动 Claude Code：

```powershell
cd C:\AI\m_projects\RE\cases\streamfab-vmp
claude
```

日常只需要记：

```text
/rekit status
/rekit sync
/rekit promote
```

| 命令 | 方向 | 什么时候用 |
|---|---|---|
| `/rekit status` | 只读 | 看当前 case 绑定状态；若目录被移动，只提示，不修复。 |
| `/rekit sync` | kit -> case | 模板仓库更新后，把 managed docs / managed block 同步到当前 case。 |
| `/rekit promote` | case -> kit | 将 case 中可复用经验回流为 managed docs 候选或 tooling 候选。 |
| `/rekit doctor` | 只读 | 排障时详细验证结构；日常不必主动运行。 |

`validate` 仍是 backend/兼容命令，但 README 不再把它作为日常主入口。

## 日常工作流

### 1. 继续逆向分析

按 case 的 `CLAUDE.local.md` 和 `references/vmp-re/task-handoff.md` 工作。

如果只更新当前进度、coverage、handler 列表：

```text
更新 references/vmp-re/task-handoff.md
```

这是 case live state，**不要 promote**。

### 2. 同步模板更新到当前 case

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

### 3. 回流可复用经验

```text
/rekit promote
```

它会同时做两类事：

1. managed docs：安全时生成候选或在 `-Apply` 时写回 pack。
2. tooling：从 case 工具链文档抽象候选，写入 `packs/vmp-re/tooling/candidates/`。

后端预览命令：

```powershell
pwsh C:\AI\m_projects\RE\kits\re-context-kits\rekit\rekit.ps1 promote `
  -Target C:\AI\m_projects\RE\cases\streamfab-vmp `
  -WhatIf
```

明确确认后才写回 managed docs：

```powershell
pwsh C:\AI\m_projects\RE\kits\re-context-kits\rekit\rekit.ps1 promote `
  -Target C:\AI\m_projects\RE\cases\streamfab-vmp `
  -Apply
```

`promote` 很保守：若 managed docs 含 `StreamFab`、真实路径、RVA/VA、ctx/round 快照、artifact/trace 路径，会阻止直接回流。工具链经验会先脱敏生成 tooling candidate，由你审查后合入正式 tooling 文档。

## 工具经验保存在哪里

工具经验不会只留在当前 case。现在分两层：

| 层级 | 路径 | 内容 |
|---|---|---|
| 通用 tooling 资产 | `packs/vmp-re/tooling/` | 工具 catalog、recipes、脚本模板化清单、补丁/止损经验。 |
| 当前 case 状态 | `cases/streamfab-vmp/references/vmp-re/toolchain-router.md` | 当前样本具体脚本、路径、工具结论和状态。 |

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

假设新目录是：

```text
D:\RE\cases\streamfab-vmp
```

### 1. 复制 case 目录

先关闭正在使用该 case 的 Claude Code、IDA、x64dbg、trace 脚本等进程：

```powershell
robocopy C:\AI\m_projects\RE\cases\streamfab-vmp D:\RE\cases\streamfab-vmp /E
```

### 2. 在新目录检查状态

```powershell
cd D:\RE\cases\streamfab-vmp
claude
/rekit status
```

如果 `.rekit/instance.yml` 里的旧 `projectRoot` 和当前目录不一致，`status` 只会提示，不会静默修改。

### 3. 确认后修复 metadata

确认这是你预期的迁移后，再运行：

```text
/rekit repair
```

后端命令默认也只预览：

```powershell
pwsh C:\AI\m_projects\RE\kits\re-context-kits\rekit\rekit.ps1 repair `
  -Target D:\RE\cases\streamfab-vmp
```

确认无误后显式写入：

```powershell
pwsh C:\AI\m_projects\RE\kits\re-context-kits\rekit\rekit.ps1 repair `
  -Target D:\RE\cases\streamfab-vmp `
  -Apply
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

或 backend：

```powershell
pwsh C:\AI\m_projects\RE\kits\re-context-kits\rekit\rekit.ps1 doctor `
  -Target D:\RE\cases\streamfab-vmp
```

### 5. 检查旧绝对路径

迁移后还要搜索只属于旧 case 根目录的绝对路径：

```text
CLAUDE.local.md
.re-template.yml
references/vmp-re/task-handoff.md
自写脚本中的 PROJECT_ROOT / workdir / output path
```

样本路径如 `C:\m_Software\...\StreamFab64.exe` 如果没有变化，不需要改。

## 新 case 怎么接入

### 新建 case

```powershell
pwsh C:\AI\m_projects\RE\kits\re-context-kits\rekit\rekit.ps1 init `
  -Target C:\AI\m_projects\RE\cases\new-vmp-case `
  -Pack vmp-re `
  -ProjectName new-vmp-case
```

### 已有 case 接入

```powershell
pwsh C:\AI\m_projects\RE\kits\re-context-kits\rekit\rekit.ps1 attach `
  -Target C:\AI\m_projects\RE\cases\existing-case `
  -Pack vmp-re
```

`attach` 只绑定 case 和生成 shim/state，不主动覆盖 managed docs。

## 目录模型

`kits/` 和 `cases/` 是同一个 RE workspace 下的 sibling 目录，不是包含关系。`re-context-kits` 是共享模板/工具资产仓库；`cases/<case>` 才是具体样本项目。这样可以让多个 case 复用同一套模板，同时避免样本、trace、dump、大文件混入模板仓库。

```text
C:\AI\m_projects\RE\
  kits\
    re-context-kits\              # 模板仓库；canonical /rekit + packs + tooling
      .claude\skills\rekit\
      rekit\
      packs\vmp-re\
        references\vmp-re\
        tooling\
  cases\
    streamfab-vmp\                # 当前具体 case
      .claude\skills\rekit\       # case-local thin shim
      .rekit\                     # instance.yml / state.json
      CLAUDE.local.md
      references\vmp-re\
      captures\
      artifacts\
  tools\                          # 第三方工具
  shared-artifacts\               # 大文件/共享产物
```

## 架构边界

- `/rekit` 是用户入口。
- `rekit/rekit.ps1` 是确定性 runtime。
- `packs/<pack>/manifest.yml` 是 managed/local/tooling/budget/promote 规则的单一事实源。
- case-local `.claude/skills/rekit/SKILL.md` 只是 thin shim，不维护业务逻辑。
- `.re-template.yml` 只保留兼容旧入口；新状态看 `.rekit/instance.yml`。
- 不默认安装用户级 skill。
- 不自动 commit / push。

## 旧脚本说明

这些旧入口仍可用，但只是 wrapper：

```text
packs/vmp-re/scripts/bootstrap.ps1
packs/vmp-re/scripts/update.ps1
packs/vmp-re/scripts/validate.ps1
packs/vmp-re/scripts/promote.ps1
```

正常使用时优先用 `/rekit` 或 `rekit/rekit.ps1`。
