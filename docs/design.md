# STeamAI design

## 读取指南

本文件是架构总览，不是默认必读清单。日常维护先读 `docs/context-routing.md`，再由 router 选择 active source；只有需要确认四层模型、manifest 边界、sync/promote 方向、current project skill 或 legacy shim 职责时，才读取本文件对应小节。

## 实施摘要

当前产品北极星已收敛为 STeamAI Lane-centric Agent Team Mission Control：用户在一个自包含项目中通过 `/steamai` 指挥主 Agent，durable lane 承载长期身份，项目内 verified bundle 运行 Go-owned deterministic runtime。内部 Go package/command 暂保留 ReKit 命名；`/rekit`、`.rekit` 与 PowerShell façade 仅保留迁移兼容。本文件保留稳定架构边界，具体批次进度与 release 判断转到 `docs/batch-plan.md` / `docs/release-readiness.md`。

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
- current project skill 只调用同项目 verified bundle；legacy `/rekit` shim 在迁移前保持 thin shim。两者都不得复制或另建 runtime 状态机。

## 产品北极星

最终产品方向见 `docs/mission-control-product-direction.md`：STeamAI 应保持 Lane-centric Agent Team Mission Control 的产品边界。用户主要指挥主 Agent / Mission Commander；长期成员身份绑定 durable member lane，而不是绑定旧聊天窗口；Claude Code session 只是可替换 executor；主 Agent 可按需启动短命 tactical subagents；用户可进入任意 lane 纠错、改向或硬切模型，当前通过显式 reconcile 把干预写入 durable state。lane 文档/packet 只表达授权意图；成员 lane 只有在 strict durable autonomy profile + `authorized-gate` decision 完全覆盖 action、exact target、typed budget、stop conditions、output paths、record/notify 和 grant/expiry 时才可执行 heavy/debug/patch/dump/hook/network/exploit-replay。

## 四层模型

1. **Skill UI**：每个 current 项目的 `.claude/skills/steamai/SKILL.md` 提供 `/steamai`；用户通过自然语言指挥主 Agent，而不是记命令目录。仓库 canonical skill 用于发布该项目 skill；legacy `/rekit` 只保留兼容。
2. **Runtime**：项目内 `.steamai/runtime` verified bundle 是 current 运行边界；`cmd/rekit/**` 与 `internal/rekit/**` 是其 Go-owned source/runtime owner，执行 init/sync/promote/validate 并维护 board、facts、lanes、runs、handovers、gate request 等 deterministic state。`rekit/rekit.ps1` 仅 retained compatibility façade，无业务 runtime 或 PowerShell fallback。
3. **Pack**：仓库 `packs/<pack>` 是可复用领域源；current 项目只消费 bundle 绑定的 `.steamai/packs/<pack>`，保存模板、tooling、lane/autonomy policy 与 `manifest.yml`。当前 `binary-re` 与 `web-security` 两条 production vertical slice 已进入统一 mature-pack release admission：`productioncontract` exact 对齐 mature manifest 与 compiled-in adapter，解析 fixture/verifier Go symbol，并把 project-local prompt/policy packet identity 绑定到 dispatch、adapter receipt、Claude launch、detached supervisor 与 recovery。manifest 的 `maturity: mature` 仍只负责候选分类，不能脱离该机器门禁单独证明成熟。
4. **Project instance**：每个 current 项目使用 relocatable `.steamai/instance.yml`、`.steamai/state.json` 和同根 member lane state；legacy-only 项目在显式迁移前继续单写 `.rekit`。两份 mutable root 不得共存。

### Go public command owner

Go public command policy 保持三层单向组合，而不是在总 dispatch `switch` 重复维护：`commands.PublicProfile` 拥有 command-level public/mutation 边界，`commands.MutationContract` 拥有 exact `(command, mode)` currentness 与 carrier，`commands.ScopedCommandDescriptor` 组合二者。CLI scoped runtime registry 现在为全部 public commands 提供唯一 callback owner，并对 descriptor mode coverage fail-closed；这已关闭“只有六个 callback owner”的旧缺口。`ScopedCommandDescriptor` 仍只是 policy catalog 的 composed view，callback 尚未成为 descriptor 自身字段；部分 fixed/default owner 仍复用通用 binder、前置 shape validation 与既有 handler，不能把全量 callback inventory 写成所有命令都已完成专属 binder/validator/handler 内聚。release inventory检查 exact owner route、publication owner 与 public coverage；注释、字符串或缺 callback 的 route 不计入覆盖。

### Durable、diagnostics 与交互边界

Go runtime 的数据流固定为单向四层：durable/workstream domain 拥有状态与 plan/receipt identity；`commands` + `plancontract` 拥有 typed invocation、currentness 与 mutation binding；full diagnostics DTO 是 canonical result 的 wire-faithful immutable clone，只在 clone 上做 current `/steamai` 或 legacy `/rekit` 投影；默认 public interaction DTO 只解码 allowlist 字段，再经纯 reducer 发布“现在/原因/下一步”。projection 不得原地改写 canonical result、不得从 DTO 重算 plan SHA，也不得用 `map[string]any` 作为默认交互 reducer；deep clone 解码保留 JSON number token，避免 diagnostics 中的大整数或事件字段失真。

Daily session 路径正在收敛为低层纯 transition reducer → typed effect → supervisor/session executor → publication coordinator；当前 pure reducer 已覆盖部分 completion/current-loop path，session executor、supervisor 和 publication 仍有既有 owner 需要继续统一，不能把现状描述成所有路径都已由 reducer 先行。进程内 Mission Control status 与 bounded driver-step 由 `cli.ReadMissionSnapshot`、`cli.PreviewDriverStep` 和 `cli.ApplyDriverStep` 这组窄 typed seam 直接复用 canonical status inventory、plan hash、Apply 与 fresh-status owner；session host 不再为这些路径调用 CLI 后反解 private JSON DTO，public JSON façade、current/legacy entrypoint projection 和 durable owner 保持不变。active-lane correction 也只做协调：必须显式选择 current open non-authority lane，append 绑定旧 executor/generation 的 typed intervention，再消费 fresh status 发布的 exact reconcile request；reconcile 保持 executor、将 generation 推进一次并使旧结果继续 stale/held。它不另建 correction 状态机、不终止进程、不启动 Claude，也不授予 authority/confirmed、gate 或 heavy-tool 权限；Reviewer rejection 与 terminal completion 仍分别归既有 rejection reconcile 与 `reopen` owner。production adapter 仍复用 canonical CLI/public driver owner，不复制 request/receipt 状态机。durable identity、权限、containment 与 heavy-action 边界始终由既有 domain/runtime owner 验证；presentation 或 transport observation 不授予权限。

### Skill 机器合同与测试 fixture owner

canonical `.claude/skills/steamai/SKILL.md` 是人工交互与安全边界的唯一源；`rekit/templates/steamai-project/SKILL.md` 是其生成镜像。人工区只说明意图、确认、权限与停止条件，不手写 executable 路径、argv 或 Apply hash flag；固定 front door、daily/control/profile bridge 和通用 typed invocation 由 `internal/rekit/skillcontract` 从 scoped command descriptor/currentness owner 生成 marker appendix。`go generate ./internal/rekit/skillcontract` 负责显式同步，`defaultdocs` / `release-check` 已能检查 stale appendix、marker 缺失和人工区平行机器合同；但 direct bundle/init provenance 与 CI 中显式 `skillcontractgen -check` 仍是待补的 P2-5 release contract，生成 appendix 不能单独证明该项已完全闭合。

`internal/rekit/testfixture` 只构造合法 current/legacy binding shell：current 强制单一 `.steamai`、schema v2 metadata、verified project-local bundle 与 manifest SHA；legacy 强制单一 `.rekit` 且不发布 current runtime。board、lane、gate、migration、malformed state 与 current-sync drift 仍由各领域测试拥有，不能把通用 fixture 扩成状态 DSL；这样参数化双根测试复用 binding 事实，但仍能精确表达领域边界。

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

- current `attach`：为已有项目生成 relocatable `.steamai/instance.yml`、`.steamai/state.json` 和项目级 `/steamai` skill，不覆盖 managed docs。
- current `init`：在 attach 基础上发布 verified project-local runtime、selected pack、common/runtime assets，并按 manifest 落地 managed 内容；缺失或篡改 bundle 时 fail-closed，不从 PATH 或中央 kit 补齐。
- legacy-only 项目的 `attach/init/bootstrap` 在迁移前继续使用 `.rekit` 与 `/rekit` 兼容入口；不得创建第二状态根。
- Windows source-clone-first 首次接入由 canonical clone 内单次构建的 unified `cmd/rekit` image 通过 external-only 顶层 `bootstrap` mode协调；它复用 daily adoption → canonical init preview/Apply，不建立第二状态机。交互确认在同一进程内部携带 exact plan SHA，用户不填写 SHA；`-format json` 固定 preview-only。Apply 只返回 `ready-to-continue`、manifest-bound original-goal continuation 与 `NoAutoResume=true`，不启动 Claude、不写 onboarding mission。
- `bootstrap` 不是用户级安装，也不写入 `~/.claude/skills`、PATH 或全局 plugin，不使用 PowerShell runtime；project-local executable不能用它接入另一个目录。

## Sync

`sync` 是可复用 pack source -> 当前 attached project：

1. 解析唯一 active state root 并读取 instance metadata。
2. 读取其绑定的 pack `manifest.yml`。
3. 更新 managed files，覆盖前备份。
4. 更新 managed block。
5. 对 template files 只 create-if-missing。
6. 更新同一 active root 的 `state.json`。
7. 运行 validate。

`sync` 不碰 local files，不删除项目文档，不自动 merge live state。`sync` 只允许作用于已经 `attach/init` 的项目；普通目录、拼错路径、dual root 或 runtime/pack identity 漂移会失败。

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

## Project skill 与 legacy shim

current 项目的 `.claude/skills/steamai/SKILL.md` 是薄 Mission Control UI：

1. 使用 `${CLAUDE_PROJECT_DIR}` 定位 exact project root。
2. 要求 `.steamai` 是唯一 mutable root，并验证 relocatable metadata。
3. 验证 `.steamai/runtime/manifest.json` 与 bundle executable/pack/assets identity。
4. 只调用项目内 bundle，不通过 PATH、用户级 plugin、Go source 或外部 kit 回退。

legacy-only 项目的 `.claude/skills/rekit/SKILL.md` 继续读取 `.rekit/instance.yml`（必要时兼容 `.re-template.yml`）并保持 thin shim。显式 migration 完成后切换为 current project skill；两种入口都不复制业务逻辑。

## 兼容策略

- `/rekit`、`.rekit`、`.re-template.yml` 与旧 pack wrappers 暂时保留，用于已存在项目和维护 API；它们不是新项目默认。
- migration 必须 zero-write preview、exact plan SHA Apply 和 durable receipt；禁止双写、自动合并或自动择优。
- 不再以 plugin/marketplace 作为默认产品入口；一个真实项目目录就是自包含 STeamAI 项目。
