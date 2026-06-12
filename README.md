# re-context-kits

私有 RE 上下文模板仓库，用于把 Claude Code 逆向工程项目组织成可复用、按需路由、渐进式披露的文档体系。

## 当前 packs

| Pack | 用途 |
|---|---|
| `packs/vmp-re` | VMProtect x64 trace-based devirtualization 上下文模板。 |

## 推荐目录模型

```text
C:\AI\m_projects\RE\
  kits\
    re-context-kits\          # 本仓库
  cases\
    <case-name>\              # 具体 RE 项目实例
  tools\                      # 第三方工具
  shared-artifacts\           # 跨项目大文件/临时产物，不进 git
```

## 新项目使用

```powershell
pwsh C:\AI\m_projects\RE\kits\re-context-kits\packs\vmp-re\scripts\bootstrap.ps1 `
  -Target C:\AI\m_projects\RE\cases\new-vmp-case `
  -ProjectName new-vmp-case
```

## 已有项目更新模板

```powershell
pwsh C:\AI\m_projects\RE\kits\re-context-kits\packs\vmp-re\scripts\update.ps1 `
  -Target C:\AI\m_projects\RE\cases\streamfab-vmp
```

## 验证模板或项目

```powershell
pwsh C:\AI\m_projects\RE\kits\re-context-kits\packs\vmp-re\scripts\validate.ps1
pwsh C:\AI\m_projects\RE\kits\re-context-kits\packs\vmp-re\scripts\validate.ps1 -Target C:\AI\m_projects\RE\cases\streamfab-vmp
```

## 设计原则

- 模板仓库只放可复用文档、脚本和脱敏示例。
- 具体项目的样本路径、coverage、handler 列表、trace/dump 不进入模板。
- 项目内 `task-handoff.md` 是活文档，由每轮任务维护。
- 常驻 `CLAUDE.local.md` 保持短，只放路由入口。
