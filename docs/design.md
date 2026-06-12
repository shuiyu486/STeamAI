# re-context-kits design

## 四层模型

1. **Skill UI**：`.claude/skills/rekit/SKILL.md`，clone 后在 kit 仓库内直接提供 `/rekit`。
2. **Runtime**：`rekit/rekit.ps1` 与 `rekit/lib/*.ps1`，执行 attach/init/sync/promote/validate。
3. **Pack**：`packs/<pack>`，保存可复用模板、示例、snippet 与 `manifest.yml`。
4. **Instance**：每个 case 的 `.rekit/instance.yml`、`.rekit/state.json`、case-local `.claude/skills/rekit` shim。

## managed vs local

- Managed files：由 pack 管理，可 `sync` 到 case，例如 `references/vmp-re/workflow-template.md`。
- Managed block：只替换文件中的标记块，例如 `CLAUDE.local.md` 的 router block。
- Template files：只在缺失时创建，例如 `task-handoff.template.md -> task-handoff.md`。
- Local files：项目自己的 live state，例如 `task-handoff.md`、`tools.local.yml`，永不自动覆盖、永不自动 promote。

## manifest 是单一事实源

`packs/<pack>/manifest.yml` 定义：

- pack 名称、版本和描述
- managed files
- template files
- local files
- managed block
- budgets
- sync policy
- promote files
- promote deny patterns

脚本不应再维护另一份 managed file 列表。旧 pack 脚本只作为兼容 wrapper，转调 `rekit/rekit.ps1`。

## Bootstrap / attach / init

- `attach`：为已有 case 生成 `.rekit/instance.yml`、`.rekit/state.json` 和 case-local `/rekit` shim，不覆盖 managed docs。
- `init` / `bootstrap`：执行 attach，然后按 manifest 将 pack 内容落地到 case。
- `bootstrap` 不是用户级安装，也不写入 `~/.claude/skills`。

## Sync

`sync` 是 `kit -> case`：

1. 读取 case `.rekit/instance.yml`。
2. 读取 pack `manifest.yml`。
3. 更新 managed files，覆盖前备份。
4. 更新 managed block。
5. 对 template files 只 create-if-missing。
6. 更新 `.rekit/state.json`。
7. 运行 validate。

`sync` 不碰 local files，不删除 case 文档，不自动 merge live state。

## Promote

`promote` 是 `case -> kit`，但默认保守：

- 只扫描 manifest 中 `promoteFiles` 声明的 managed docs。
- 默认生成候选或 dry-run 输出；写回 pack 需要明确 `-Apply`。
- 命中 deny patterns 时阻止，例如绝对路径、artifact/trace 路径、`.exe/.dll`、地址快照、round/task 状态。
- 永不 promote `CLAUDE.local.md` 全文、`task-handoff.md`、`tools.local.yml`、`captures/**`、`artifacts/**`。

## Case shim

case-local `.claude/skills/rekit/SKILL.md` 是薄 shim：

1. 读取 `.rekit/instance.yml`，回退 `.re-template.yml`。
2. 取得 `templateRoot` 和 `templatePack`。
3. 读取 `<templateRoot>/.claude/skills/rekit/SKILL.md`。
4. 调用 `<templateRoot>/rekit/rekit.ps1`。

shim 不复制业务逻辑；后续只维护 kit 仓库中的 canonical `/rekit`。

## 兼容策略

- `.re-template.yml` 暂时保留，用于旧项目与旧脚本兼容。
- `packs/<pack>/scripts/bootstrap.ps1`、`update.ps1`、`validate.ps1` 保留为 wrapper。
- 后续如果需要 plugin/marketplace，再在当前结构上增加 `.claude-plugin/plugin.json`，不影响 runtime 与 packs。
