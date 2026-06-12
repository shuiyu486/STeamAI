# re-context-kits

私有 RE 上下文模板工具包，用于把 Claude Code 逆向工程项目组织成可复用、按需路由、渐进式披露、可回流优化的文档体系。

## 当前 packs

| Pack | 用途 |
|---|---|
| `packs/vmp-re` | VMProtect x64 trace-based devirtualization 上下文模板。 |

## 推荐目录模型

```text
C:\AI\m_projects\RE\
  kits\
    re-context-kits\          # 本仓库，canonical /rekit 与 packs
  cases\
    <case-name>\              # 具体 RE 项目实例
  tools\                      # 第三方工具
  shared-artifacts\           # 跨项目大文件/临时产物，不进 git
```

## Clone 后直接使用

在本仓库目录启动 Claude Code 后，仓库内 skill 会提供 `/rekit`。

```text
/rekit status
/rekit validate
/rekit init C:\AI\m_projects\RE\cases\new-vmp-case
```

也可以直接调用 deterministic backend：

```powershell
pwsh C:\AI\m_projects\RE\kits\re-context-kits\rekit\rekit.ps1 status
pwsh C:\AI\m_projects\RE\kits\re-context-kits\rekit\rekit.ps1 validate -Target C:\AI\m_projects\RE\kits\re-context-kits
```

不需要默认安装用户级 skill；case 通过 `.claude/skills/rekit` 薄 shim 回到本仓库的 canonical `/rekit`。

## 绑定已有项目

```powershell
pwsh C:\AI\m_projects\RE\kits\re-context-kits\rekit\rekit.ps1 attach `
  -Target C:\AI\m_projects\RE\cases\streamfab-vmp `
  -Pack vmp-re
```

该命令会生成：

```text
<case>\.claude\skills\rekit\SKILL.md
<case>\.rekit\instance.yml
<case>\.rekit\state.json
```

## 新项目初始化

```powershell
pwsh C:\AI\m_projects\RE\kits\re-context-kits\rekit\rekit.ps1 init `
  -Target C:\AI\m_projects\RE\cases\new-vmp-case `
  -Pack vmp-re `
  -ProjectName new-vmp-case
```

兼容旧入口仍可用：

```powershell
pwsh C:\AI\m_projects\RE\kits\re-context-kits\packs\vmp-re\scripts\bootstrap.ps1 `
  -Target C:\AI\m_projects\RE\cases\new-vmp-case `
  -ProjectName new-vmp-case
```

## 同步与回流

```powershell
# kit -> case：同步 managed docs 与 managed block
pwsh C:\AI\m_projects\RE\kits\re-context-kits\rekit\rekit.ps1 sync `
  -Target C:\AI\m_projects\RE\cases\streamfab-vmp

# case -> kit：默认生成候选，不直接写回 pack
pwsh C:\AI\m_projects\RE\kits\re-context-kits\rekit\rekit.ps1 promote `
  -Target C:\AI\m_projects\RE\cases\streamfab-vmp `
  -WhatIf

# 明确确认后才写回 pack
pwsh C:\AI\m_projects\RE\kits\re-context-kits\rekit\rekit.ps1 promote `
  -Target C:\AI\m_projects\RE\cases\streamfab-vmp `
  -Apply
```

## 设计原则

- `/rekit` 是用户入口，`rekit/rekit.ps1` 是 deterministic runtime。
- `packs/<pack>/manifest.yml` 是 managed/local/budget/promote 规则的单一事实源。
- case 只保存 `.rekit/instance.yml`、`.rekit/state.json` 和薄 shim。
- 模板仓库只放可复用文档、脚本和脱敏示例。
- 具体项目的样本路径、coverage、handler 列表、trace/dump 不进入模板。
- `task-handoff.md` 是 case live doc，由每轮任务维护，不自动 promote。
- 不自动 commit/push；模板改进完成后由用户在本仓库正常 `git diff` / `git commit` / `git push`。
