# CLAUDE.md

## 项目定位

本仓库是 STeamAI 的 canonical 实现仓库（`https://github.com/shuiyu486/STeamAI`）：面向安全研究的 Claude Code Agent Team Mission Control，组织 Mission Commander、durable lanes、可替换 session executors、短命 subagents、领域 pack、证据和门禁。旧 `github.com/shuiyu486/re-context-kits` 暂时只作为 Go module compatibility identity 保留。

产品北极星见 `docs/mission-control-product-direction.md`：一个真实项目目录就是一个自包含 STeamAI 项目。用户在目录内启动已有 Claude Code，主要指挥主 Agent；新项目入口 `/steamai`、current 根 `.steamai`，runtime/selected pack 随项目发布。`/rekit`、`.rekit`、中央 kit/thin-shim 仅迁移兼容。成员身份绑定 lane；heavy action 只认 active root 的 strict autonomy profile + `authorized-gate`。

本仓库不是具体安全/RE case 或自动分析、漏洞挖掘、渗透引擎。仅为验证 `onboard/init/attach/sync/promote` 或 workstream 行为创建临时 case。

## 文档不变量 / 上下文路由

**本项目文档必须做成按需路由、渐进式披露的样式。** `docs/context-routing.md` 是唯一完整路由表；本文件只保留稳定边界。

只读必要边界、router 和真实仓库状态，再由 router 选择**一个**场景入口；不要默认串读路线图、batch plan、CHANGELOG、release readiness 或历史。active docs 只留当前决策/卡/短指针；未来、inventory 和旧日志进 backlog、专题或 archive。

## 维护入口

- `/steamai` canonical skill：`.claude/skills/steamai/SKILL.md`；legacy `/rekit` compatibility skill：`.claude/skills/rekit/SKILL.md`
- STeamAI 自包含项目合同：`docs/steamai-self-contained-project.md`
- Go deterministic backend：`cmd/rekit/main.go`、`internal/rekit/**`；内部 executable/package 暂不机械改名
- Claude session host：`cmd/rekit-host/main.go`、`internal/rekit/sessionhost/**`；日常 `-daily`，维护 gate 为 `-live-acceptance`、Windows soak/pack-memory acceptance；均不替代 currentness、授权或 strict intake
- External session / Remote Control：`internal/rekit/externalsession/**`、`internal/rekit/cli/current_loop_external_transport.go`；仅 durable Reviewer dispatch 显式 opt-in，opaque endpoint 不是 durable identity
- Read-only adapter：`cmd/rekit-adapter-{host,acceptance}/**`、`internal/rekit/adapterhost/**`；只消费 strict gate/dispatch，`gate` 不启动进程
- PowerShell compatibility façade：`rekit/rekit.ps1`（不新增业务 runtime）
- pack：`packs/<pack>/**`；manifest：`packs/<pack>/manifest.yml`；common policies/prompts：`common/**`
- release/docs 入口：`docs/context-routing.md`、`docs/real-usage-hardening-roadmap.md`、`docs/release-readiness.md`、`docs/powershell-deprecation.md`、`CHANGELOG.md`

## CodeGraph 使用边界

维护本仓库自身代码、模板和文档时，分析 `internal/rekit/**`、`cmd/rekit/**`、`rekit/rekit.ps1`、pack/common/docs 的结构、调用链、symbol 定义或改动影响面，优先使用 CodeGraph MCP（`codegraph_explore`）；CodeGraph 返回源码视为已读。

不要把 CodeGraph 当成样本、二进制、trace、dump、capture、case artifact 或目标环境分析工具。处理具体安全/RE case 时，仍按 pack、lane packet、工具链、evidence ledger 和授权边界执行。

## 当前推进原则

当前已批准路线是 `steamai-product-optimization-v1`，Batch 830～833 严格串行；Batch 830～832 已完成，当前只实施 Batch 833，完成后再做路线级完整验证与清理。保持 source-clone-first，Go + Claude Code 是预期依赖，不做 installer。canonical repository/clone 使用 `shuiyu486/STeamAI`；Go module/internal `rekit`、legacy `/rekit` / `.rekit` 暂保留兼容。完成态只认 Git-local machine receipt 和本地 tracking ref；远程 CI、跨平台 runtime E2E 仅作专项证据，cross-compile 不等于运行证据。

保持 PowerShell-free/Go-native；禁止新增 PowerShell runtime logic。避免单字段 contract/inventory/metadata 微批次，把支撑改动并入产品闭环。PowerShell replacement/removal 有 Go-native 替代、文档和验证时可继续；公共入口删除门禁不完整则升级。

## 关键边界

- `sync` 是 kit -> case；`promote` 是 case -> kit；二者默认 review-first，写入前必须确认具体范围。
- `continue -Apply` 不写 authority/confirmed、不执行 heavy-tool。
- Executable Mission Commander request 携带与 command/receipt 一致的 bounded typed invocation；多 lane 未选时不发布 executable request，selected artifact 只用唯一 exact `-Lane`。Request SHA 只证明 currentness，不授予权限。
- 自然语言纠偏从 fresh typed state 唯一选路：Reviewer rejection 走既有 correction/reconcile；committed completion 只走 `reopen` preview/exact Apply，返回 `ready-to-continue` + `NoAutoResume`，不自动接管/启动 Claude。
- `gate -Apply` 只写 gate decision 或 authorized execution observation，不执行 heavy action；actual heavy action 由 executor/adapter 在 strict profile + `authorized-gate` 内执行并留证。
- transport/endpoint/delivery observation 不授予权限；Remote Control uncertain delivery 不重发或 same-job replacement。
- authority/confirmed、schema migration、公共 façade 删除门禁不完整、未授权副作用、产品方向或难判断架构取舍，需要升级。
- 不要把真实样本、trace、dump、capture、artifact、绝对 case 路径、payload、flag、客户信息或 case-specific 进度写入模板仓库。
- 新项目 `/steamai` 必须使用 project-local verified runtime bundle，不得回退机器 PATH 或外部 kit；legacy `/rekit` thin shim 只在迁移完成前兼容旧 `.rekit` 项目。
- `.steamai` 与 `.rekit` dual-read/single-write：current-only 单写 `.steamai`，legacy-only 单写 `.rekit`，neither 的新项目选择 `.steamai`，both fail-closed；禁止双写、自动合并、自动择优或 reparse alias。
- case public JSON 的 project-local typed command 由 resolved state root 统一投影：current `.steamai` 只显示 `/steamai`，legacy `.rekit` 只显示 `/rekit`；只遍历显式 typed structure，不做 JSON 文本替换，不改 prose 或 durable/source identity。
- `bounded-autonomous-v1` 只是显式 opt-in 的单 lane/exact action/exact target/有限预算/短 expiry 档位，每次仍重验并留证；不是无限权限，也不授予 authority/confirmed、sync/promote 或 schema migration。
- exact lane `control` 使用独立 append-only generation 与 review-first stamp/hash Apply；pause 不做 OS suspend，stop 先 durable 提交且只允许 exact local supervisor owner 关闭自己持有的 containment。actuation 失败不回滚 stopped，process termination 不是 durable stop 成功判据，opaque Remote Control session 不受本路径管理；旧 generation 结果不得推进 live output、Reviewer、completion 或 checkpoint。
- current-sync Apply 与 current `.steamai` durable detached-supervisor handoff 依赖 handle-bound exact filesystem mutation；Windows提供该能力，非 Windows 在 lease/spec/intent/cancellation 等持久化副作用前 fail-closed。Read-only/preview 与 legacy `.rekit` zero-handoff compatibility 不应被一并禁用。

## 验证命令

仓库级推荐最小集：

```text
go run ./cmd/rekit -- -Command release-check -Format json
go run ./cmd/rekit -- -Command status
go run ./cmd/rekit -- -Command packs
go run ./cmd/rekit -- -Command doctor
go test -count=1 -p=2 -timeout=30m ./...
go vet ./...
git diff --check
```

Canonical Go tests 以`-count=1`保证fresh execution、`-p=2`限并发、`-timeout=30m`限逐package binary；build cache仍可复用。`release-run`整步上限45分钟，其余命令5分钟；Windows Job／Unix进程组清理子孙，64 MiB输出后仅drain 5秒。远程三平台workflow先vet后tests且普通batch不等待；`ciReleaseGate.ready`只验证定义，不代表远程green。完整边界见`docs/release-readiness.md`。

按需追加：改 façade/compatibility 时运行 `rekit/tests/facade-smoke.ps1`；改 pack wrapper 时运行对应 pack validate/smoke；涉及 workstream/ledger/gate/sync/promote 写入时用临时 case 验证。