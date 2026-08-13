# CLAUDE.md

## 项目定位

`re-context-kits` 是面向网络安全研究与安全工程任务的 Claude Code Agent Team Mission Control 框架，用于组织主 Agent / Mission Commander、durable member lanes、可替换 Claude Code session executors、短命 tactical subagents、领域工具链、证据账本、验证门禁和可复用安全领域 pack。

产品北极星见 `docs/mission-control-product-direction.md`：用户主要指挥主 Agent；`/rekit` 与 Go backend 是底层 deterministic runtime/API；`rekit.ps1` 只作为 retained compatibility façade；长期成员身份绑定 lane，不绑定旧聊天窗口。lane 文档/packet 只能表达授权意图；heavy action 的确定性授权依据是 strict validated `.rekit/lanes/<lane>/autonomy.json` 加 `gate` 记录的 `authorized-gate` decision。

本仓库不是具体安全 case、RE case、自动脱壳器、逆向引擎、漏洞挖掘器或渗透执行器。不要因为 README 的 case 初始化示例而在 kit 仓库内创建无关 case；只有验证 `onboard`、`init`、`attach`、`sync`、`promote` 或 workstream 行为时才创建临时 case。

## 文档不变量 / 上下文路由

**本项目文档必须做成按需路由、渐进式披露的样式。** `docs/context-routing.md` 是唯一完整路由表；本文件只保留稳定边界。

默认只读本文件必要边界、router 和真实仓库状态，再由 router 选择**一个**场景入口及指定章节。不要默认串读路线图、batch plan、CHANGELOG、release readiness、历史计划或 `docs/batch-history.md`；机器 `readFirst[]` 也必须先指向 router。

active docs 只保留当前决策、当前卡和短指针；未来批次、完整 inventory、旧日志与历史设计进入 backlog、专题或 archive。出现 5 个以上 read-first 文件、多份完整路由表或同一 current pointer 的长副本时，先做文档减压。

## 维护入口

- `/rekit` skill：`.claude/skills/rekit/SKILL.md`
- Go deterministic backend：`cmd/rekit/main.go`、`internal/rekit/**`
- Claude session host：`cmd/rekit-host/main.go`、`internal/rekit/sessionhost/**`；日常用 `-daily`；维护 gate 为 `-live-acceptance`（默认 `vmp-re`，或 `_template` / `web-security`）、Windows `-live-soak-acceptance` 和 `-live-pack-memory-acceptance`。Soak 仅对 typed Reviewer semantic/lineage failure 保留首败并 fresh retry 一次；均不替代 currentness、授权或 strict intake
- External session / Remote Control companion：`internal/rekit/externalsession/**`、`internal/rekit/cli/current_loop_external_transport.go`；Remote Control 仅在 durable Reviewer dispatch 显式 opt-in时使用，local `sessionhost`仍是Windows默认provider，opaque endpoint不是durable identity
- Read-only adapter/验收：`cmd/rekit-adapter-{host,acceptance}/**`、`internal/rekit/adapterhost/**`；只消费 strict gate/dispatch，`gate` 不启动进程
- PowerShell compatibility façade：`rekit/rekit.ps1`（不新增业务 runtime）
- pack：`packs/<pack>/**`；manifest：`packs/<pack>/manifest.yml`；common policies/prompts：`common/**`
- release/docs 入口：`docs/context-routing.md`、`docs/real-usage-hardening-roadmap.md`、`docs/release-readiness.md`、`docs/powershell-deprecation.md`、`CHANGELOG.md`

## CodeGraph 使用边界

维护本仓库自身代码、模板和文档时，分析 `internal/rekit/**`、`cmd/rekit/**`、`rekit/rekit.ps1`、pack/common/docs 的结构、调用链、symbol 定义或改动影响面，优先使用 CodeGraph MCP（`codegraph_explore`）；CodeGraph 返回源码视为已读。

不要把 CodeGraph 当成样本、二进制、trace、dump、capture、case artifact 或目标环境分析工具。处理具体安全/RE case 时，仍按 pack、lane packet、工具链、evidence ledger 和授权边界执行。

## 当前推进原则

当前先收敛真实日用：自然语言开始/继续/查看/纠偏和新会话接手须顺畅、可记录、可恢复。当前支持与日常完成门槛以 Windows 本机为准；远程 CI、Linux/macOS、三平台兼容和安装包仅专项检查。保持 PowerShell-free/Go-native 底座。

禁止新增 PowerShell runtime logic；convergence batch须减少 residue 或完成删除门禁。其它 batch 可推进 lane/reconcile/autonomy、Reviewer、authorized execution、adapter、pack-memory 或文档路由闭环。

不要连续推进单字段 contract/inventory/metadata 微批次；支撑字段须并入 Mission Commander、executor、Reviewer writeback、authorized execution、adapter、pack-memory UX 或 product path 的中大型闭环。

PowerShell replacement/removal 不再因“删除 PowerShell”本身停下询问，但必须有 Go-native 替代、文档和验证；若要删除公共入口且替代、恢复或真实 release-gate-green 条件不完整，仍需升级。

## 关键边界

- `sync` 是 kit -> case；`promote` 是 case -> kit；二者默认 review-first，写入前必须确认具体范围。
- `continue -Apply` 不写 authority/confirmed、不执行 heavy-tool。
- Executable Mission Commander request必须携带与command/expected receipt一致的bounded typed ReKit invocation；多lane未选择时不发布request/SHA/operator/takeover，selected artifact只使用唯一exact durable `-Lane`。Request SHA只证明currentness，不能授予authority/confirmed、gate或heavy-action权限。
- 自然语言纠偏必须从 fresh typed state 唯一选路：Reviewer rejection 继续走既有 correction/reconcile owner；committed completion 只走 existing `reopen` preview/exact Apply，返回 `ready-to-continue` 且 `NoAutoResume`，不得建立第二状态机或自动接管/启动 Claude。
- `gate -Apply` 默认只写 pending-gate / authorized-gate request ledger decision；传入 execution fields 时只写 authorized execution observation evidence，不执行实际 heavy action。
- actual heavy/debug/patch/dump/hook/network/exploit replay 由 lane executor 或 tool adapter 在 strict durable autonomy profile + `authorized-gate` 范围内执行，并写回 evidence/ledger。
- 跨会话 transport message、endpoint discovery或delivery observation都不能授予authority/confirmed或heavy-action授权；Remote Control uncertain delivery不得自动重发或same-job replacement，必须新建durable Reviewer dispatch/session/job。
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

默认远程 CI workflow 是 `.github/workflows/release-gate.yml`，异步定义 Linux、Windows、macOS Go-native checks，并先运行 `go vet` 再运行 Go tests；普通 batch 不等待它。`release-check` 的 `ciReleaseGate.ready=true` 只验证 workflow/inventory 定义，不代表远程 jobs 实际通过；`status` 使用 lightweight project handoff，不执行完整 release audit。仅正式发布、跨平台专项或周期复审读取 GitHub Actions 实际状态；日常迭代以 Windows 本机 focused tests 和完整 release minimum 为完成依据。

按需追加：改 façade/compatibility 时运行 `rekit/tests/facade-smoke.ps1`；改 pack wrapper 时运行对应 pack validate/smoke；涉及 workstream/ledger/gate/sync/promote 写入时用临时 case 验证。