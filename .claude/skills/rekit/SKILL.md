---
name: rekit
description: Re-context-kits toolkit entrypoint. Use this skill when the user mentions rekit, re-context-kits, context kits, VMProtect RE templates, attaching a case to a kit, syncing templates into a case, promoting case learnings back to the kit, or validating context-kit structure. Prefer this skill over manually running pack scripts.
disable-model-invocation: true
---

# rekit

`/rekit` 是 `re-context-kits` 的目录级入口。目标是让 kit 仓库 clone 后直接可用，并让每个 case 通过薄 shim 回到本仓库的 canonical runtime。

## 工作模式

- **kit 模式**：当前目录是 `re-context-kits` 仓库，直接使用本仓库的 canonical `/rekit`。
- **case 模式**：当前目录是具体 case，先读取 `.rekit/instance.yml`；若不存在则回退读取 `.re-template.yml`，取得 `templateRoot` 与 `templatePack`，再调用 `<templateRoot>/rekit/rekit.ps1`。
- **不默认安装用户级 skill**：canonical skill 跟随 git 仓库；case 只生成 `.claude/skills/rekit/SKILL.md` 薄 shim。

## 用户使用方式

默认给用户展示 `/rekit` 形式，不要让用户手动记 backend PowerShell：

```text
/rekit init -Target <caseRoot> -Pack vmp-re -ProjectName <caseName>
/rekit attach -Target <caseRoot> -Pack vmp-re
/rekit status
/rekit sync
/rekit promote
/rekit doctor
/rekit repair
```

backend 命令只在实际执行、自动化、CI、排障或用户明确要求时展示。

## 命令语义

| 用户意图 | backend |
|---|---|
| `/rekit status` | `pwsh <templateRoot>/rekit/rekit.ps1 status [-Target <caseRoot>]` |
| `/rekit attach` | `pwsh <templateRoot>/rekit/rekit.ps1 attach -Target <caseRoot> -Pack vmp-re` |
| `/rekit repair` | `pwsh <templateRoot>/rekit/rekit.ps1 repair -Target <caseRoot>` 预览；确认后用 `-Apply` 修复迁移后的 metadata |
| `/rekit init` / `/rekit bootstrap` | `pwsh <templateRoot>/rekit/rekit.ps1 init -Target <caseRoot> -Pack vmp-re` |
| `/rekit sync` | 默认先运行 `pwsh <templateRoot>/rekit/rekit.ps1 sync -Target <caseRoot> -Review`，生成 LLM 审查包；用户确认后才执行写入型 sync |
| `/rekit promote` | 默认先运行 `pwsh <templateRoot>/rekit/rekit.ps1 promote -Target <caseRoot> -Review`，生成回流审查包；用户确认后才生成候选或写回 pack |
| `/rekit doctor` | `pwsh <templateRoot>/rekit/rekit.ps1 doctor [-Target <caseRoot>]`；`validate` 是兼容别名 |

如果用户没有显式给 `Target`，在 case 模式下使用当前工作目录；在 kit 模式下 `doctor/status` 作用于 kit 本身。`status` 只读检测迁移，不静默修复；`repair -Apply` 才会写 metadata。

## LLM-first review 规则

1. `/rekit sync` 与 `/rekit promote` 默认都是 **review-first**：先生成 `.rekit/reviews/<timestamp>-<command>/packet.json`、`summary.md` 和 bounded diff，再由 Claude 比较优劣、冲突与风险。
2. 用户确认前，不要执行会写入 managed files、pack docs、promote candidates、tooling candidates 或 state 的 backend。
3. review 报告必须按大项说明：方向、变化、收益、风险、冲突、推荐动作，并给出可选择项。
4. `/rekit sync` 的确认选项优先使用：同步全部推荐项、只同步无冲突项、逐项选择、取消。
5. `/rekit promote` 的确认选项优先使用：仅生成候选、只生成 tooling candidate、按报告改写进模板、逐项选择、取消。
6. “继续”“好”“confirm”不能扩大授权；写入前必须确认具体动作、target、pack 与文件范围。
7. backend 的 `-Review` 是强只读；`-Review -Apply` 应拒绝。`-WhatIf` 只是旧式 stdout dry run，不等同于 LLM review。

## 执行规则

1. 优先调用 `rekit/rekit.ps1`，但不要把它作为用户日常入口展示。
2. `sync` 只做 `kit -> case`：更新 manifest 声明的 managed files 与 managed blocks，并为覆盖前文件创建 backup；不碰 local files。
3. `sync` / `promote` 必须要求目标是已经 `attach/init` 的 case；不要对普通目录或拼错路径隐式创建 case 或生成回流候选。
4. `promote` 只做 `case -> kit` 的候选提取或显式 `-Apply` 写回；永不提升 `CLAUDE.local.md`、`task-handoff.md`、`tools.local.yml`、`captures/**`、`artifacts/**`。
5. `promote` 同时处理 tooling：从 case 的工具链文档抽象候选，写入 `packs/<pack>/tooling/candidates/`，供人工合入 `tooling/catalog.yml` 或 `tooling/recipes/*`。
6. 若 promote 命中绝对路径、样本名、trace/dump/artifact/capture 路径或明显地址快照，先阻止或生成候选报告，不要静默写回模板。
7. `sync` / `promote` 发现 case 路径迁移但 metadata 未修复时必须拒绝执行，提示用户确认后运行 `repair -Apply`。
8. manifest 中所有文件路径必须是相对路径，并且不能越出 case root 或 pack root。
9. 所有写操作后都运行对应 doctor/validate；失败时如实报告错误与下一步。

## 常用说明模板

对用户解释时优先这样说：

```text
第一次 clone 后，在 kit 仓库启动 Claude Code，然后用 /rekit init 或 /rekit attach。
以后在 case 里只用 /rekit status / sync / promote；sync/promote 会先生成审查报告，确认后才写入。
脚本只是 backend，不需要手动执行。
```
