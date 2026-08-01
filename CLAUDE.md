# CLAUDE.md

## 项目定位

`re-context-kits` 是面向网络安全研究与安全工程任务的 Claude Code Agent Team Mission Control 框架，用于组织主 Agent / Mission Commander、durable member lanes、可替换 Claude Code session executors、短命 tactical subagents、领域工具链、证据账本、验证门禁和可复用安全领域 pack。

产品北极星见 `docs/mission-control-product-direction.md`：用户主要指挥主 Agent；`/rekit` 与 Go backend 是底层 deterministic runtime/API；`rekit.ps1` 只作为 retained compatibility façade；长期成员身份绑定 lane，不绑定旧聊天窗口。lane 文档/packet 只能表达授权意图；heavy action 的确定性授权依据是 strict validated `.rekit/lanes/<lane>/autonomy.json` 加 `gate` 记录的 `authorized-gate` decision。

本仓库不是具体安全 case、RE case、自动脱壳器、逆向引擎、漏洞挖掘器或渗透执行器。不要因为 README 的 case 初始化示例而在 kit 仓库内创建无关 case；只有验证 `init`、`attach`、`sync`、`promote` 或 workstream 行为时才创建临时 case。

## 上下文路由 / 渐进式披露

先读 `docs/context-routing.md`，再按需读其它文档顶部区域。不要默认串读所有 durable docs，也不要默认读取 `docs/batch-history.md` 全文。修改、创建或维护文档时也要保持按需路由与渐进披露：顶部短执行区，细节按章节/专文路由，不把历史、长日志或完整设计重新塞回 active docs。文档索引、推荐话术和接手清单不是默认必读清单；如果出现 5 个以上 read-first 文件，先压缩为 `docs/context-routing.md` + 当前场景入口 + 顶部区。

每轮默认最小集：

1. 本文件顶部边界。
2. `docs/context-routing.md`。
3. `docs/batch-plan.md` 顶部 current milestone / current batch state / next candidates。
4. `CHANGELOG.md` 顶部 `Unreleased`。
5. 真实状态：`git status --short`、必要 focused tests、本机 release gate；远程 CI 只在需要 release 判断时检查。

Batch 推送节奏（仅在当前 goal/session 已明确授权 commit/push 时适用）：若 goal 指定在 `main` 接手，先确认 `main` 与 `origin/main` 同步且工作树干净，再在 `main` 上提交并推送到 `origin/main`。正常最多两次 push，先推 implementation commit（代码、测试、文档、本地验证），再推 release inspection commit（只记录 implementation commit 触发的远程 run）。不要为 release inspection commit 自己触发的 CI 再追加第三个记录提交；除非出现不同于既有 GitHub Actions runner/billing `steps=[]` blocker 的新远程信号，否则只在当前上下文或下一批 planning 中引用既有 blocker。

按需路由：

| 场景 | 首选入口 |
|---|---|
| 产品方向 | `docs/mission-control-product-direction.md` 顶部 |
| 架构边界 / 四层模型 | `docs/design.md` 顶部和对应小节 |
| 自主短 goal / 接手 cadence | `docs/autonomous-goal.md` 顶部 |
| 当前批次 / 下一步 | `docs/batch-plan.md` 顶部 |
| 旧批次考古 | `docs/batch-history.md` 中按 Batch ID 搜索 |
| release / CI 判断 | `docs/release-readiness.md` 顶部 + `release-check -Format json` |
| PowerShell façade / removal | `docs/powershell-deprecation.md` 顶部和相关矩阵行 |
| runtime 调用链 / symbol 影响面 | CodeGraph MCP 优先查询 `internal/rekit/**` / `cmd/rekit/**`；legacy 迁移只按需读 `docs/go-runtime-migration.md` 顶部 |
| ledger / evidence 字段语义 | `docs/evidence-ledger.md` 顶部和对应事件类型 |
| 旧 case 迁移 / moved metadata | `docs/case-migration.md` 顶部和对应步骤 |
| 文档减压 / 路由审计 | `docs/context-routing.md` + 目标文档顶部；用搜索定位旧 read-first 列表 |
| smoke 选择 | `rekit/tests/README.md` 对应类别 |

## 维护入口

- `/rekit` skill：`.claude/skills/rekit/SKILL.md`
- Go backend：`cmd/rekit/main.go`、`internal/rekit/**`
- PowerShell compatibility façade：`rekit/rekit.ps1`（不新增业务 runtime）
- pack：`packs/<pack>/**`；首个 mature pack 是 `packs/vmp-re/**`，安全领域 skeleton 包括 `web-security`、`malware-analysis`、`vuln-research`、`ctf`、`unpack-pe`、`ollvm`、`android-native`、`generic-binary-re`
- pack manifest：`packs/<pack>/manifest.yml`
- common policies/prompts：`common/**`
- release / docs：`docs/context-routing.md`、`docs/release-readiness.md`、`docs/powershell-deprecation.md`、`docs/batch-plan.md`、`CHANGELOG.md`

## CodeGraph 使用边界

维护本仓库自身代码、模板和文档时，分析 `internal/rekit/**`、`cmd/rekit/**`、`rekit/rekit.ps1`、pack/common/docs 的结构、调用链、symbol 定义或改动影响面，优先使用 CodeGraph MCP（`codegraph_explore`）；CodeGraph 返回源码视为已读。

不要把 CodeGraph 当成样本、二进制、trace、dump、capture、case artifact 或目标环境分析工具。处理具体安全/RE case 时，仍按 pack、lane packet、工具链、evidence ledger 和授权边界执行。

## 当前推进原则

当前阶段优先把已有骨架收敛到可真实日常使用：用户能用自然语言开始 case、继续推进、查看状态、人工插手纠偏、新会话接手；允许半自动，但必须顺畅、可记录、可恢复。PowerShell-free default/product path、Go-native、跨平台与 truthful release readiness 继续作为底座约束推进；远程 Linux/macOS/Windows CI 和 macOS/Linux product-path 在 GitHub runner/billing blocker 解除或发布前保持 known gap，不挤占本机 Windows 可验证的最低可用 Mission Control / executor / reviewer / pack-memory 闭环迭代效率。

禁止新增 PowerShell runtime logic。PowerShell convergence batch 应实际减少 retained residue 或完成删除门禁；其它 batch 可推进 lane、reconcile、autonomy、reviewer dispatch/intake、authorized execution evidence、adapter-specific live validation、pack-memory 或文档上下文路由闭环。

不要连续推进单字段 contract / inventory / metadata 微批次；新增 contract 字段必须嵌入 Mission Commander、replaceable executor、reviewer writeback、authorized execution evidence、adapter-specific live validation、pack-memory UX 或 product path，并由 package / CLI / 临时 case / product-path 验证证明解决真实断点。

同样避免连续做“把某个字段、summary、handoff detail、text line 从 A 投影到 B”的可见性微调。单批可做投影补齐，但连续出现时必须升级为中大型能力闭环：先写清用户或 Mission Commander 能感知的断点，再把相关字段/文本作为该闭环的支撑，而不是为了寻找 `latestX` / `summaryX` / `contextX` 继续局部最优。

PowerShell replacement/removal 不再因“删除 PowerShell”本身停下询问，但必须有 Go-native 替代、文档和验证；若要删除公共入口且替代、恢复或真实 release-gate-green 条件不完整，仍需升级。

## 关键边界

- `sync` 是 kit -> case；`promote` 是 case -> kit；二者默认 review-first，写入前必须确认具体范围。
- `continue -Apply` 不写 authority/confirmed、不执行 heavy-tool。
- `gate -Apply` 默认只写 pending-gate / authorized-gate request ledger decision；传入 execution fields 时只写 authorized execution observation evidence，不执行实际 heavy action。
- actual heavy/debug/patch/dump/hook/network/exploit replay 由 lane executor 或 tool adapter 在 strict durable autonomy profile + `authorized-gate` 范围内执行，并写回 evidence/ledger。
- confirmed/authority 写入、runtime schema 迁移、公共 façade 删除门禁不完整、未授权外部副作用、产品方向变化或难以判断的架构取舍，需要升级。
- 不要把真实样本、trace、dump、capture、artifact、绝对 case 路径、payload、flag、客户信息或 case-specific 进度写入模板仓库。
- case-local `/rekit` 必须保持 thin shim，回到 kit 仓库 canonical runtime；不要复制 runtime 逻辑。

## 验证命令

仓库级推荐最小集：

```text
go run ./cmd/rekit -- -Command release-check -Format json
go run ./cmd/rekit -- -Command status
go run ./cmd/rekit -- -Command packs
go run ./cmd/rekit -- -Command doctor
go test ./...
go vet ./...
git diff --check
```

默认远程 CI workflow 是 `.github/workflows/release-gate.yml`，定义 Linux、Windows、macOS Go-native release checks，并先运行`go vet`再运行Go tests；`release-check` 的 `ciReleaseGate.ready=true` 只验证 workflow/inventory 定义，不代表远程 jobs 已获得 runner 或实际通过。`status`使用lightweight project handoff，不执行完整release audit，不能替代`release-check`。2026-08-01 run `30674507300` 已恢复真实三平台runner执行，但CLI package均在600秒test timeout处失败，并暴露Windows跨盘符/namespace、macOS canonical path与Linux文件错误契约差异；Batch 785 implementation run `30687208204`三平台均在`Release inventory`因latest batch未标记`已完成`而fail-closed；Batch 786 run `30690553758`证明三平台inventory/status/packs/doctor/vet通过，并把剩余Go test差异收敛为macOS system temp alias fixture、Linux反斜杠fixture路径和Windows CRLF workflow mutation fixture。Batch 787 implementation run `30692412838`已真实完成并在Linux/macOS/Windows全部通过inventory/status/packs/doctor/vet与完整Go tests，当前默认三平台Go-native release workflow已有remote green证据。发布结论仍必须读取GitHub Actions实际状态，不能仅凭inventory ready或沿用旧`steps=[]`判断。

按需追加：改 façade/compatibility 时运行 `rekit/tests/facade-smoke.ps1`；改 pack wrapper 时运行对应 pack validate/smoke；涉及 workstream/ledger/gate/sync/promote 写入时用临时 case 验证。