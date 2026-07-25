---
name: rekit
description: Case-local shim for re-context-kits. Use when working inside this case and the user asks for rekit, syncing context templates, promoting case learnings, validating the context-kit binding, or updating managed pack reference docs.
disable-model-invocation: true
---

# rekit case shim

本 skill 是 case-local 薄 shim，不包含业务逻辑。真正的 canonical `/rekit` 位于绑定的 `re-context-kits` 仓库中；实际解释与执行由 canonical runtime / Go-native backend 负责。

## 使用步骤

1. 读取当前 case 根目录的 `.rekit/instance.yml`。
2. 如果 `.rekit/instance.yml` 不存在，回退读取 `.re-template.yml`。
3. 从 metadata 中取得：
   - `templateRoot`
   - `templatePack`
4. 读取并遵循：

```text
<templateRoot>/.claude/skills/rekit/SKILL.md
```

5. 按 canonical skill 的 LLM-first 和日常工作流语义执行 `/rekit`。`overview/continue/start/handoff` 由 canonical runtime 解释；`sync` / `promote` 默认先生成 review 包，让 Claude 输出优劣/冲突报告并取得用户明确确认后，才执行写入动作。shim 只做 metadata 跳转，不展示底层脚本或 CLI 命令。
6. 新会话 first screen 先使用 `/rekit`，确认 `status case shim ready=true`、`installedShimMatchesTemplate=true`，再按 `status case mission` 的 queue/current action 与 durable artifacts 接手；active reviewer/evidence/adapter review blocker 优先于普通 handoff/continue follow-up；如果 shim drift，先走 repair preview，不直接修复；默认 `/rekit` 还会先显示 compact Mission Commander first-screen strip，帮助快速定位 case current action 与 pack-memory current action。

## 规则

- 不要在本 shim 里维护模板规则；所有规则以 canonical `/rekit` 和 `<templateRoot>/packs/<templatePack>/manifest.yml` 为准。
- 不要读取或修改用户级 `~/.claude/skills`。
- `status` 只读检测迁移；需要修复路径时必须由用户确认后运行 canonical `repair`。
- `sync` / `promote` 只允许作用于已经绑定的 case；不要对普通目录或拼错路径隐式创建 case 或生成回流候选。
- `overview/continue/start/handoff` 是当前推荐的日常入口；优先让 canonical runtime 处理，不要在 shim 中复制逻辑，不要在 shim 中维护 fallback 或命令执行细节。
- `overview` 只是项目总览；多工作线时应使用 `continue main` 或 `continue <name>` 明确接手对象。
- `handoff` 无参数生成项目级索引；`handoff main` 或 `handoff <name>` 生成指定工作线接手文档。
- `continue <name>` 可以自动发布低风险事实、路由 request、处理 verifier 通过的 candidate；覆盖/删除 authority、冲突、schema change、外部副作用或破坏性动作仍必须问用户。
- `sync` / `promote` 默认必须 review-first。
- 不要 promote live state，例如 `CLAUDE.local.md` 全文、`task-handoff.md`、`tools.local.yml`、`captures/**`、`artifacts/**`。
- 工具链经验通过 canonical `promote` 生成 tooling 候选，候选位置为 `<templateRoot>/packs/<templatePack>/tooling/candidates/`。
