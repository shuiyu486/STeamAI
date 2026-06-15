---
name: rekit
description: Re-context-kits toolkit entrypoint. Use this skill when the user mentions rekit, re-context-kits, context kits, VMProtect RE templates, attaching a case to a kit, syncing templates into a case, promoting case learnings back to the kit, or validating context-kit structure. Prefer this skill over manually running pack scripts.
disable-model-invocation: true
---

# rekit

`/rekit` 是 `re-context-kits` 的目录级入口。目标是让 kit 仓库 clone 后直接可用，并让每个 case 通过薄 shim 回到本仓库的 canonical runtime。

## 工作模式

- **kit 模式**：当前目录是 `re-context-kits` 仓库，直接使用本仓库的 `rekit/rekit.ps1`。
- **case 模式**：当前目录是具体 case，先读取 `.rekit/instance.yml`；若不存在则回退读取 `.re-template.yml`，取得 `templateRoot` 与 `templatePack`，再调用 `<templateRoot>/rekit/rekit.ps1`。
- **不默认安装用户级 skill**：canonical skill 跟随 git 仓库；case 只生成 `.claude/skills/rekit/SKILL.md` 薄 shim。

## 命令语义

| 用户意图 | backend |
|---|---|
| `/rekit status` | `pwsh <templateRoot>/rekit/rekit.ps1 status [-Target <case>]` |
| `/rekit attach` | `pwsh <templateRoot>/rekit/rekit.ps1 attach -Target <case> -Pack vmp-re` |
| `/rekit repair` | `pwsh <templateRoot>/rekit/rekit.ps1 repair -Target <case>` 预览；确认后用 `-Apply` 修复迁移后的 metadata |
| `/rekit init` / `/rekit bootstrap` | `pwsh <templateRoot>/rekit/rekit.ps1 init -Target <case> -Pack vmp-re` |
| `/rekit sync` | `pwsh <templateRoot>/rekit/rekit.ps1 sync -Target <case>` |
| `/rekit promote` | `pwsh <templateRoot>/rekit/rekit.ps1 promote -Target <case>`，默认只生成候选；写回 pack 需要 `-Apply` |
| `/rekit doctor` | `pwsh <templateRoot>/rekit/rekit.ps1 doctor [-Target <case>]`；`validate` 是兼容别名 |

如果用户没有显式给 `Target`，在 case 模式下使用当前工作目录；在 kit 模式下 `doctor/status` 作用于 kit 本身。`status` 只读检测迁移，不静默修复；`repair -Apply` 才会写 metadata。

## 执行规则

1. 优先调用 `rekit/rekit.ps1`，不要让用户手动记 `packs/<pack>/scripts/*.ps1`。
2. `sync` 只做 `kit -> case`：更新 manifest 声明的 managed files 与 managed blocks，并为覆盖前文件创建 backup；不碰 local files。
3. `promote` 只做 `case -> kit` 的候选提取或显式 `-Apply` 写回；永不提升 `CLAUDE.local.md`、`task-handoff.md`、`tools.local.yml`、`captures/**`、`artifacts/**`。
4. `promote` 同时处理 tooling：从 case 的工具链文档抽象候选，写入 `packs/<pack>/tooling/candidates/`，供人工合入 `tooling/catalog.yml` 或 `tooling/recipes/*`。
5. 若 promote 命中绝对路径、样本名、trace/dump/artifact 路径或明显地址快照，先阻止或生成候选报告，不要静默写回模板。
6. `sync` / `promote` 发现 case 路径迁移但 metadata 未修复时必须拒绝执行，提示用户确认后运行 `repair -Apply`。
7. 所有写操作后都运行对应 doctor/validate；失败时如实报告错误与下一步。

## 常用示例

```powershell
# 在 kit 仓库内验证 pack
pwsh "C:\AI\m_projects\RE\kits\re-context-kits\rekit\rekit.ps1" validate

# 绑定已有 case，生成 .rekit 与 case-local /rekit shim
pwsh "C:\AI\m_projects\RE\kits\re-context-kits\rekit\rekit.ps1" attach `
  -Target "C:\AI\m_projects\RE\cases\streamfab-vmp" `
  -Pack vmp-re

# 从 kit 同步 managed docs 到 case
pwsh "C:\AI\m_projects\RE\kits\re-context-kits\rekit\rekit.ps1" sync `
  -Target "C:\AI\m_projects\RE\cases\streamfab-vmp"

# 从 case 生成可提升候选，并抽象 tooling 候选；不直接写回敏感内容
pwsh "C:\AI\m_projects\RE\kits\re-context-kits\rekit\rekit.ps1" promote `
  -Target "C:\AI\m_projects\RE\cases\streamfab-vmp" `
  -WhatIf
```
