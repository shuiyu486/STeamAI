# re-context-kits

`re-context-kits` 是给 RE case 使用的 Claude Code 上下文模板工具包。

一句话：**你平时在具体 case 里工作，直接用 `/rekit`；模板仓库负责保存通用模板和回流改进。**

## 当前状态

当前已有一个 pack：

| Pack | 用途 |
|---|---|
| `vmp-re` | VMProtect x64 trace-based devirtualization 上下文模板。 |

当前 `streamfab-vmp` case 已经绑定到本仓库：

```text
case:         C:\AI\m_projects\RE\cases\streamfab-vmp
kit:          C:\AI\m_projects\RE\kits\re-context-kits
pack:         vmp-re
case shim:    C:\AI\m_projects\RE\cases\streamfab-vmp\.claude\skills\rekit\SKILL.md
case state:   C:\AI\m_projects\RE\cases\streamfab-vmp\.rekit\instance.yml
```

所以你现在不需要再安装，也不需要再 attach。进入 `streamfab-vmp` 后就可以用。

## 你在 streamfab-vmp 里该怎么用

在 case 目录启动 Claude Code：

```powershell
cd C:\AI\m_projects\RE\cases\streamfab-vmp
claude
```

然后使用：

```text
/rekit status
/rekit validate
/rekit sync
/rekit promote
```

对应含义：

| 命令 | 方向 | 什么时候用 |
|---|---|---|
| `/rekit status` | 只读 | 看当前 case 绑定到哪个 kit / pack。 |
| `/rekit validate` | 只读 | 收尾前验证 case 和模板文档结构是否正常。 |
| `/rekit sync` | kit -> case | 模板仓库更新后，把 managed docs 同步到当前 case。 |
| `/rekit promote` | case -> kit | 你在 case 文档里沉淀了通用经验，想回流到模板仓库。 |

> 推荐日常只记这四个命令。旧的 PowerShell 脚本是兼容入口，不是日常主入口。

## 最常见工作流

### 1. 日常继续逆向分析

直接按 case 的 `CLAUDE.local.md` 和 `references/vmp-re/task-handoff.md` 工作即可。

如果只更新当前进度、coverage、handler 列表：

```text
更新 references/vmp-re/task-handoff.md
```

这类是 case live state，**不要 promote**。

### 2. 收尾前验证

```text
/rekit validate
```

或直接跑 backend：

```powershell
pwsh C:\AI\m_projects\RE\kits\re-context-kits\rekit\rekit.ps1 validate `
  -Target C:\AI\m_projects\RE\cases\streamfab-vmp
```

### 3. 把模板更新同步到 streamfab-vmp

当 `re-context-kits` 里的模板更新后，在 `streamfab-vmp` 中执行：

```text
/rekit sync
```

它会同步 manifest 声明的 managed docs 和 managed block，例如：

```text
references/vmp-re/README.md
references/vmp-re/workflow-template.md
references/vmp-re/progressive-disclosure.md
references/vmp-re/toolchain-router.md
references/vmp-re/singleton-handler-review.md
CLAUDE.local.md 中的 managed router block
```

它不会覆盖：

```text
references/vmp-re/task-handoff.md
tools.local.yml
captures/**
artifacts/**
CLAUDE.local.md 中 block 外的 case 私有内容
```

### 4. 把 case 里的通用经验回流到模板

如果你在 `streamfab-vmp` 的 managed docs 中总结了通用经验，先预览：

```text
/rekit promote
```

或者：

```powershell
pwsh C:\AI\m_projects\RE\kits\re-context-kits\rekit\rekit.ps1 promote `
  -Target C:\AI\m_projects\RE\cases\streamfab-vmp `
  -WhatIf
```

确认安全后才写回模板：

```powershell
pwsh C:\AI\m_projects\RE\kits\re-context-kits\rekit\rekit.ps1 promote `
  -Target C:\AI\m_projects\RE\cases\streamfab-vmp `
  -Apply
```

`promote` 默认很保守。若文档含 `StreamFab`、真实路径、RVA/VA、ctx/round 快照、artifact/trace 路径，会阻止回流，避免污染通用模板。

回流后，在模板仓库正常审查并提交：

```powershell
cd C:\AI\m_projects\RE\kits\re-context-kits
git diff
git status
git add -A
git commit -m "..."
git push origin main
```

## 迁移已有 case 到新目录

如果你想把当前 `streamfab-vmp` 从旧目录迁到更干净的位置，推荐流程是：**先复制，验证新目录可用，再归档旧目录**。

假设新目录是：

```text
D:\RE\cases\streamfab-vmp
```

### 1. 复制 case 目录

先关闭正在使用该 case 的 Claude Code、IDA、x64dbg、trace 脚本等进程，再复制目录。

PowerShell 示例：

```powershell
robocopy C:\AI\m_projects\RE\cases\streamfab-vmp D:\RE\cases\streamfab-vmp /E
```

如果 `captures/`、`artifacts/` 很大，也可以先只迁移文档和脚本，后续再单独搬大文件；但不要在未确认前删除旧目录。

### 2. 在新目录重新 attach

`templateRoot` 如果仍然是原来的 kit 仓库路径，不需要改；执行 attach 会重写 `.rekit/instance.yml`、刷新 case-local `/rekit` shim，并补齐兼容 metadata：

```powershell
pwsh C:\AI\m_projects\RE\kits\re-context-kits\rekit\rekit.ps1 attach `
  -Target D:\RE\cases\streamfab-vmp `
  -Pack vmp-re `
  -ProjectName streamfab-vmp
```

### 3. 验证新目录

```powershell
pwsh C:\AI\m_projects\RE\kits\re-context-kits\rekit\rekit.ps1 validate `
  -Target D:\RE\cases\streamfab-vmp
```

验证通过后，在新目录启动 Claude Code：

```powershell
cd D:\RE\cases\streamfab-vmp
claude
/rekit status
/rekit validate
```

### 4. 检查绝对路径

当前 case 文档和脚本里可能含旧绝对路径，例如 `C:\AI\m_projects\RE\cases\streamfab-vmp`。迁移后建议搜索并只更新确实需要跟随项目根目录变化的路径。

重点检查：

```text
CLAUDE.local.md
.re-template.yml
references/vmp-re/task-handoff.md
自写脚本中的 PROJECT_ROOT / workdir / output path
```

样本路径如 `C:\m_Software\...\StreamFab64.exe` 如果没有变化，不需要改。

### 5. 确认后再处理旧目录

只有新目录通过 validate、关键脚本能运行、必要大文件已确认可访问后，再归档或删除旧目录。

## 新 case 怎么接入

### 新建 case

```powershell
pwsh C:\AI\m_projects\RE\kits\re-context-kits\rekit\rekit.ps1 init `
  -Target C:\AI\m_projects\RE\cases\new-vmp-case `
  -Pack vmp-re `
  -ProjectName new-vmp-case
```

会生成：

```text
<case>\.claude\skills\rekit\SKILL.md
<case>\.rekit\instance.yml
<case>\.rekit\state.json
<case>\references\vmp-re\...
<case>\CLAUDE.local.md
```

### 已有 case 接入

```powershell
pwsh C:\AI\m_projects\RE\kits\re-context-kits\rekit\rekit.ps1 attach `
  -Target C:\AI\m_projects\RE\cases\existing-case `
  -Pack vmp-re
```

`attach` 只绑定 case 和生成 shim/state，不主动覆盖 managed docs。

## 目录模型

```text
C:\AI\m_projects\RE\
  kits\
    re-context-kits\              # 模板仓库；canonical /rekit + packs
      .claude\skills\rekit\
      rekit\
      packs\vmp-re\
  cases\
    streamfab-vmp\                # 当前具体 case
      .claude\skills\rekit\       # case-local thin shim
      .rekit\                     # instance.yml / state.json
      CLAUDE.local.md
      references\vmp-re\
      captures\
      artifacts\
```

## 架构边界

- `/rekit` 是用户入口。
- `rekit/rekit.ps1` 是确定性 runtime。
- `packs/<pack>/manifest.yml` 是 managed/local/budget/promote 规则的单一事实源。
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
