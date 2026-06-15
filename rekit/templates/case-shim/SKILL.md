---
name: rekit
description: Case-local shim for re-context-kits. Use when working inside this case and the user asks for rekit, syncing context templates, promoting case learnings, validating the context-kit binding, or updating managed RE reference docs.
disable-model-invocation: true
---

# rekit case shim

本 skill 是 case-local 薄 shim，不包含业务逻辑。真正的 canonical `/rekit` 位于绑定的 `re-context-kits` 仓库中。

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

5. 调用 canonical backend：

```powershell
pwsh "<templateRoot>/rekit/rekit.ps1" <status|repair|attach|init|sync|promote|doctor> -Target "<当前 case 根目录>"
```

## 规则

- 不要在本 shim 里维护模板规则；所有规则以 canonical `/rekit` 和 `<templateRoot>/packs/<templatePack>/manifest.yml` 为准。
- 不要读取或修改用户级 `~/.claude/skills`。
- `status` 只读检测迁移；需要修复路径时必须由用户确认后运行 canonical `repair -Apply`。
- 不要 promote live state，例如 `CLAUDE.local.md` 全文、`task-handoff.md`、`tools.local.yml`、`captures/**`、`artifacts/**`。
- 工具链经验通过 canonical `promote` 生成 tooling 候选，候选位置为 `<templateRoot>/packs/<templatePack>/tooling/candidates/`。
