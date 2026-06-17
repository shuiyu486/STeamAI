---
name: rekit
description: Re-context-kits toolkit entrypoint. Use this skill when the user mentions rekit, re-context-kits, context kits, VMProtect RE templates, attaching a case to a kit, syncing templates into a case, promoting case learnings back to the kit, or validating context-kit structure. Prefer this skill over manually running pack scripts.
disable-model-invocation: true
---

# rekit

`/rekit` 是 `re-context-kits` 的目录级入口。目标是让 kit 仓库 clone 后直接可用，并让每个 case 通过薄 shim 回到本仓库的 canonical runtime。

## 工作模式

- **kit 模式**：当前目录是 `re-context-kits` 仓库，直接使用本仓库的 canonical `/rekit`。
- **case 模式**：当前目录是具体 case，先读取 `.rekit/instance.yml`；若不存在则回退读取 `.re-template.yml`，取得 `templateRoot` 与 `templatePack`，再遵循 template root 中的 canonical `/rekit` 规则。
- **不默认安装用户级 skill**：canonical skill 跟随 git 仓库；case 只生成 `.claude/skills/rekit/SKILL.md` 薄 shim。

## 用户使用方式

默认只给用户展示 `/rekit` 形式，不要让用户手动记或执行底层脚本：

```text
/rekit init -Target <caseRoot> -Pack vmp-re -ProjectName <caseName>
/rekit attach -Target <caseRoot> -Pack vmp-re
/rekit status
/rekit sync
/rekit promote
/rekit doctor
/rekit repair
/rekit plan-subagents
/rekit parallel
/rekit parallel <name>
```

底层 runtime 只作为 `/rekit` 的内部实现；除非用户明确要求排障，不在日常说明中展示。

## 命令语义

| 用户意图 | 行为 |
|---|---|
| `/rekit status` | 只读显示 kit / case 绑定状态；检测迁移但不修复。 |
| `/rekit attach` | 将已有 case 绑定到当前 template root 和 pack。 |
| `/rekit repair` | 预览迁移后的 metadata 修复；用户确认后才写入。 |
| `/rekit init` / `/rekit bootstrap` | 初始化 case metadata、case-local shim 和模板文件。 |
| `/rekit sync` | 默认先生成 LLM 审查包；用户确认具体范围后才执行写入型 sync。 |
| `/rekit promote` | 默认先生成回流审查包；用户确认后才生成候选或写回 pack。 |
| `/rekit doctor` | 验证 kit / case 结构、文档预算和 policy registry。 |
| `/rekit plan-subagents` | 根据 pack manifest 的 `subagentRoutes` 生成只读子 agent 分片计划；不启动 agent，不修改 managed/project source files；会写 `.rekit/reviews/...` 审查产物。 |
| `/rekit parallel` / `/rekit parallel <name>` | 智能并行会话面板；创建/续接功能会话，维护 `.rekit/parallel/**` 状态、resume prompt、feature workspace、collect/review/sync/standalone/close 建议。 |

如果用户没有显式给 `Target`，在 case 模式下使用当前工作目录；在 kit 模式下 `doctor/status` 作用于 kit 本身。`status` 只读检测迁移，不静默修复；`repair` 写入前必须得到用户确认。

## LLM-first review 规则

1. `/rekit sync` 与 `/rekit promote` 默认都是 **review-first**：先生成 `.rekit/reviews/<timestamp>-<command>/packet.json`、`summary.md` 和 bounded diff，再由 Claude 比较优劣、冲突与风险。
2. 用户确认前，不要执行会写入 managed files、pack docs、promote candidates、tooling candidates 或 state 的动作。
3. review 报告必须按大项说明：方向、变化、收益、风险、冲突、推荐动作，并给出可选择项。
4. `/rekit sync` 的确认选项优先使用：同步全部推荐项、只同步无冲突项、逐项选择、取消。
5. `/rekit promote` 的确认选项优先使用：用 `-CreateCandidates` 仅生成候选、只生成 tooling candidate、用 `-Apply` 按报告改写进模板、逐项选择、取消。
6. “继续”“好”“confirm”不能扩大授权；写入前必须确认具体动作、target、pack 与文件范围。
7. 内部 review 模式是强只读；旧式 dry run 不等同于 LLM review。

## 执行规则

1. 使用 canonical runtime 执行实际动作，但不要把底层命令作为用户日常入口展示。
2. `sync` 只做 `kit -> case`：更新 manifest 声明的 managed files 与 managed blocks，并为覆盖前文件创建 backup；不碰 local files。
3. `sync` / `promote` 必须要求目标是已经 `attach/init` 的 case；不要对普通目录或拼错路径隐式创建 case 或生成回流候选。
4. `promote` 只做 `case -> kit` 的 review、`-CreateCandidates` 候选提取或 `-Apply` 显式写回；永不提升 `CLAUDE.local.md`、`task-handoff.md`、`tools.local.yml`、`captures/**`、`artifacts/**`。
5. `promote` 同时处理 tooling：从 case 的工具链文档抽象候选，供人工合入 `tooling/catalog.yml` 或 `tooling/recipes/*`。
6. 若 promote 命中绝对路径、样本名、trace/dump/artifact/capture 路径或明显地址快照，先阻止或生成候选报告，不要静默写回模板。
7. `sync` / `promote` 发现 case 路径迁移但 metadata 未修复时必须拒绝执行，提示用户确认后运行 `repair`。
8. `parallel` 可以写 case-local `.rekit/parallel/**` 和 feature workspace；它不写 pack、不写 confirmed canonical 文件。`collect/review/promote` 生成审查包，confirmed 合并仍由 authority/main session 执行。
9. manifest 中所有文件路径必须是相对路径，并且不能越出 case root 或 pack root。
10. 所有写操作后都运行对应 doctor；失败时如实报告错误与下一步。

## 常用说明模板

对用户解释时优先这样说：

```text
第一次 clone 后，在 kit 仓库启动 Claude Code，然后用 /rekit init 或 /rekit attach。
以后在 case 里只用 /rekit status / sync / promote；sync/promote 会先生成审查报告，确认后才写入。
底层脚本只是内部实现，不需要手动执行。
```
