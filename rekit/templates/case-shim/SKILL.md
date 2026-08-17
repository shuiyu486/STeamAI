---
name: rekit
description: Automatically use inside a legacy .rekit case attached to the STeamAI repository for natural-language status, start, continue, correction, handoff, sync/promote review, or gated tooling requests. This is a thin metadata redirect to the canonical compatibility skill.
---

# rekit case shim

本 skill 是 legacy case-local 薄 shim，不包含业务逻辑。真正的 canonical `/rekit` compatibility skill 位于 metadata 绑定的 STeamAI repository checkout；解释与执行由 canonical runtime / Go-native backend 负责。Repository 在 GitHub 上改名不会自动改写既有绝对 `templateRoot`，因此本 shim继续以绑定metadata为准。

## 跳转步骤

1. 从当前目录向上定位 case root，读取 `.rekit/instance.yml`。
2. 若 `.rekit/instance.yml` 不存在，回退读取 `.re-template.yml`。
3. 取得 `templateRoot` 与 `templatePack`。
4. 读取并完全遵循：

```text
<templateRoot>/.claude/skills/rekit/SKILL.md
```

5. 把用户当前自然语言请求原样交给 canonical skill 归一化。shim 不解释 lane 路由、不拼 Apply 参数、不维护状态机，也不展示底层脚本或 CLI 命令。

## First screen

新会话 first screen 先使用 `/rekit` 的只读 status 语义，确认 `status case shim ready=true` 与 `installedShimMatchesTemplate=true`，再从 fresh typed current action 和 durable artifacts 接手。Reviewer、evidence、adapter review blocker 优先于普通 handoff；project handoff 的 pack-memory 与 next-batch action queues 只用于 durable takeover，不替代 fresh status。

如果 shim/binding drift，只进入 canonical repair preview；用户精确确认前不修复。status 查询不得写 case、不得启动 Claude。

## 不变量

- 不要在本 shim 里维护模板规则；规则只来自 canonical `/rekit` 与 `<templateRoot>/packs/<templatePack>/manifest.yml`。
- 不要读取或修改用户级 `~/.claude/skills`。
- 不要在 shim 中复制逻辑，包括 route priority、loop、fallback、gate、sync/promote 或命令执行细节。
- `sync` / `promote` 默认必须 review-first；确认具体 target、pack 和文件范围前不写入。
- attached case 之外的普通目录不得被隐式接管；拼错路径也不得创建 case 或候选。
- heavy action 必须由 canonical strict profile + `authorized-gate` 决定；shim 不授权、不执行。
- shim 不写 authority/confirmed，不执行 heavy tool，不修改 managed/project source。
