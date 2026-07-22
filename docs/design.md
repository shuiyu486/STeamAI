# re-context-kits design

## 读取指南

本文件是架构总览，不是默认必读清单。日常维护先读 `docs/context-routing.md` 和 `docs/batch-plan.md` 顶部；只有需要确认四层模型、manifest 边界、sync/promote 方向或 case shim 职责时，才读取本文件对应小节。

## 实施摘要

当前产品北极星已收敛为 Lane-centric Agent Team Mission Control：用户指挥主 Agent，durable lane 承载长期身份，Go-native `/rekit` 是 canonical runtime，PowerShell façade 仅保留兼容。本文件保留稳定架构边界，具体批次进度与 release 判断转到 `docs/batch-plan.md` / `docs/release-readiness.md`。

## 执行清单

- 架构判断先确认本文件顶部四层模型和 managed/local 边界。
- runtime 调用链或 symbol 影响面优先用 CodeGraph 查询 `internal/rekit/**` / `cmd/rekit/**`。
- 旧迁移细节按需读 `docs/go-runtime-migration.md`、`docs/powershell-deprecation.md` 或历史 batch，不把它们并回本文件。
- 新增设计内容先判断是否应放入专题文档、pack reference 或 `docs/context-routing.md` 路由表。

## 验证标准

- 新维护者只读 `docs/context-routing.md` + 本文件顶部即可判断设计入口，不需要串读全部 durable docs。
- 本文件只描述稳定边界；当前状态、批次验证和远程 CI 结论不在这里维护。
- 修改架构边界后至少运行 `go run ./cmd/rekit -- -Command release-check -Format json` 和相关 focused tests。

## 风险与注意事项

- 渐进式披露不是删除设计事实；细节应放在正确专题文档并由 `docs/context-routing.md` 路由。
- 不要把 batch 日志、release 记录、真实 case 进度或工具输出塞入本架构总览。
- case-local shim 必须保持 thin shim，回到 kit 仓库 canonical runtime；不要复制 runtime logic。

## 产品北极星

最终产品方向见 `docs/mission-control-product-direction.md`：`re-context-kits` 应收敛为 Lane-centric Agent Team Mission Control。用户主要指挥主 Agent / Mission Commander；长期成员身份绑定 durable member lane，而不是绑定旧聊天窗口；Claude Code session 只是可替换 executor；主 Agent 可按需启动短命 tactical subagents；用户可进入任意 lane 纠错、改向或硬切模型，当前通过显式 reconcile 把干预写入 durable state。lane 文档/packet 只表达授权意图；成员 lane 只有在 strict durable autonomy profile + `authorized-gate` decision 完全覆盖 action、exact target、typed budget、stop conditions、output paths、record/notify 和 grant/expiry 时才可执行 heavy/debug/patch/dump/hook/network/exploit-replay。

## 四层模型

1. **Skill UI**：`.claude/skills/rekit/SKILL.md`，clone 后在 kit 仓库内直接提供 `/rekit`；用户层应优先表现为主 Agent mission control，而不是命令目录。
2. **Runtime**：`cmd/rekit/**` 与 `internal/rekit/**` 是 Go-owned canonical runtime，执行 attach/init/sync/promote/validate 并维护 board、facts、lanes、runs、handovers、gate request 等 deterministic state；`rekit/rekit.ps1` 仅 retained compatibility façade，无业务 runtime 或 PowerShell fallback。
3. **Pack**：`packs/<pack>`，保存某个安全领域的可复用模板、tooling 资产、示例、snippet、lane/autonomy policy 与 `manifest.yml`；当前首个成熟 pack 是 `vmp-re`。
4. **Instance**：每个 case 的 `.rekit/instance.yml`、`.rekit/state.json`、case-local `.claude/skills/rekit` shim，以及 case-local member lane state。

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
- tooling files 和 tooling candidate sources

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

`sync` 不碰 local files，不删除 case 文档，不自动 merge live state。`sync` 只允许作用于已经 `attach/init` 的 case；普通目录或拼错路径会失败。

## Promote

`promote` 是 `case -> kit`，但默认保守：

- 扫描 manifest 中 `promoteFiles` 声明的 managed docs。
- 同时读取 `toolingCandidateSources`，将 case 工具链经验脱敏为 tooling candidate。
- 默认生成候选或 dry-run 输出；写回 managed docs 需要明确 `-Apply`。
- tooling candidate 默认写入 `packs/<pack>/tooling/candidates/`，由人工审查后合入 `tooling/catalog.yml` 或 `tooling/recipes/*`。
- 合入后的 tooling 资产留在 pack 中，fresh case 通过 attached metadata 重新消费 pack tooling；`sync` 不复制 tooling files 到 case，避免把可复用经验误写成 case-local 私有事实。
- 目标必须是已经 `attach/init` 的 case；普通目录不会参与候选生成或写回。
- 命中 deny patterns 时阻止，例如绝对路径、artifact/capture/trace/dump 路径、`.exe/.dll`、地址快照、round/task 状态。
- 永不 promote `CLAUDE.local.md` 全文、`task-handoff.md`、`tools.local.yml`、`captures/**`、`artifacts/**`。

## 路径边界

manifest 中所有文件路径必须是相对路径，并且 normalize 后不能越出对应 root。runtime 对 pack source、case target、managed block 和 tooling candidate source 统一做 containment check。

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
