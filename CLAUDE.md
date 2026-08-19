# CLAUDE.md

## 项目定位

本仓库是 STeamAI canonical 实现（`https://github.com/shuiyu486/STeamAI`）：面向安全研究的 Claude Code Agent Team Mission Control，组织 Mission Commander、durable lanes、可替换 session executors、短命 subagents、领域 pack、证据和门禁。旧 `github.com/shuiyu486/re-context-kits` 仅保留为 Go module compatibility identity。

产品北极星见 `docs/mission-control-product-direction.md`：一个真实项目目录就是一个自包含 STeamAI 项目。用户在目录内启动已有 Claude Code，主要指挥主 Agent；新项目用 `/steamai`、`.steamai` 和随项目发布的 runtime/selected pack。`/rekit`、`.rekit`、中央 kit/thin-shim 仅用于迁移兼容。成员身份绑定 lane；heavy action 只认 active root 的 strict autonomy profile + `authorized-gate`。

本仓库不是具体安全/RE case 或自动分析、漏洞挖掘、渗透引擎。只为验证产品行为创建临时 case。

## 文档不变量 / 上下文路由

本项目文档必须做成按需路由、渐进式披露的样式；`docs/context-routing.md` 是唯一完整路由表，本文件只保留稳定边界。

只读必要边界、router 和真实仓库状态，再由 router 选择一个场景入口；不要默认串读路线图、batch plan、CHANGELOG、release readiness 或历史。active docs 只留当前决策/卡/短指针；未来、inventory 和旧日志进入 backlog、专题或 archive。

## 维护入口

- `/steamai` canonical skill：`.claude/skills/steamai/SKILL.md`；legacy `/rekit` compatibility skill：`.claude/skills/rekit/SKILL.md`
- 自包含项目合同：`docs/steamai-self-contained-project.md`
- Go backend：`cmd/rekit/main.go`、`internal/rekit/**`；内部 executable/package 暂不机械改名
- session host：`cmd/rekit-host/main.go`、`internal/rekit/sessionhost/**`；日常 `-daily`，维护 gate 为 `-live-acceptance` 及专项 acceptance
- External session / Remote Control：`internal/rekit/externalsession/**`、`internal/rekit/cli/current_loop_external_transport.go`；仅 durable Reviewer dispatch 显式 opt-in
- Read-only adapter：`cmd/rekit-adapter-{host,acceptance}/**`、`internal/rekit/adapterhost/**`
- PowerShell compatibility façade：`rekit/rekit.ps1`（不新增业务 runtime）
- pack：`packs/<pack>/**`；manifest：`packs/<pack>/manifest.yml`；common：`common/**`
- release/docs：`docs/context-routing.md`、`docs/real-usage-hardening-roadmap.md`、`docs/release-readiness.md`、`docs/powershell-deprecation.md`、`CHANGELOG.md`

## CodeGraph 使用边界

维护本仓库代码、模板和文档时，分析结构、调用链、symbol 或影响面优先用 CodeGraph MCP（`codegraph_explore`）；其源码返回视为已读。不要用 CodeGraph 分析样本、二进制、trace、dump、capture、case artifact 或目标环境；具体安全/RE case 仍走 pack、lane packet、工具链、evidence ledger 和授权边界。

## 当前推进原则

当前已批准路线是 `steamai-product-optimization-v1`：Batch 830～833 与路线级真实验收已完成，当前没有已批准的下一路线；不自动新增 Batch 834 或选择其它产品功能。保持 source-clone-first，Go + Claude Code 是预期依赖，不做 installer。canonical repository/clone 使用 `shuiyu486/STeamAI`；Go module/internal `rekit`、legacy `/rekit` / `.rekit` 暂保留兼容。完成态只认 Git-local machine receipt 和本地 tracking ref；远程 CI、跨平台 runtime E2E 仅作专项证据，cross-compile 不等于运行证据。

保持 PowerShell-free/Go-native；禁止新增 PowerShell runtime logic。避免单字段 contract/inventory/metadata 微批次，把支撑改动并入产品闭环。公共入口删除门禁不完整则升级。

## 关键边界

- `sync` 是 kit -> case；`promote` 是 case -> kit；均默认 review-first，写入前确认具体范围。
- `continue -Apply` 不写 authority/confirmed、不执行 heavy-tool。
- Executable Mission Commander request 携带与 command/receipt 一致的 bounded typed invocation；多 lane 未选时不发布 executable request，selected artifact 只用唯一 exact `-Lane`。Request SHA 只证明 currentness，不授予权限。
- 自然语言纠偏从 fresh typed state 唯一选路：Reviewer rejection 走既有 correction/reconcile；committed completion 只走 `reopen` preview/exact Apply，返回 `ready-to-continue` + `NoAutoResume`，不自动接管或启动 Claude。
- `gate -Apply` 只写 gate decision 或 authorized execution observation，不执行 heavy action；actual heavy action 由 executor/adapter 在 strict profile + `authorized-gate` 内执行并留证。
- transport/endpoint/delivery observation 不授予权限；Remote Control uncertain delivery 不重发或 same-job replacement；opaque endpoint 不是 durable identity。
- authority/confirmed、schema migration、公共 façade 删除门禁不完整、未授权副作用、产品方向或难判断架构取舍，需要升级。
- 不要把真实样本、trace、dump、capture、artifact、绝对 case 路径、payload、flag、客户信息或 case-specific 进度写入模板仓库。
- 新项目 `/steamai` 必须使用 project-local verified runtime bundle，不得回退机器 PATH 或外部 kit；legacy `/rekit` thin shim 只兼容旧 `.rekit` 项目。
- `.steamai` 与 `.rekit` dual-read/single-write：current-only 单写 `.steamai`，legacy-only 单写 `.rekit`，neither 选择 `.steamai`，both fail-closed；禁止双写、自动合并、自动择优或 reparse alias。
- case public JSON 的 project-local typed command 由 resolved state root 统一投影：current 只显示 `/steamai`，legacy 只显示 `/rekit`；只遍历显式 typed structure，不替换 JSON prose 或 durable/source identity。
- `bounded-autonomous-v1` 只显式 opt-in 单 lane/exact action/exact target/有限预算/短 expiry，每次仍重验并留证；不授予无限权限、authority/confirmed、sync/promote 或 schema migration。
- exact lane `control` 使用独立 append-only generation 与 review-first stamp/hash Apply；pause 不做 OS suspend，stop 先 durable 提交且仅 exact local supervisor owner 可关闭自己持有的 containment。actuation 失败不回滚 stopped，process termination 不是 durable stop 成功判据，opaque Remote Control session 不受本路径管理；旧 generation 结果不得推进 live output、Reviewer、completion 或 checkpoint。
- current-sync Apply 与 current `.steamai` durable detached-supervisor handoff 依赖 handle-bound exact filesystem mutation；Windows 提供该能力，非 Windows 在 lease/spec/intent/cancellation 等持久化副作用前 fail-closed。Read-only/preview 与 legacy `.rekit` zero-handoff compatibility 不应被一并禁用。

## 验证命令

```text
go run ./cmd/rekit -- -Command release-check -Format json
go run ./cmd/rekit -- -Command status
go run ./cmd/rekit -- -Command packs
go run ./cmd/rekit -- -Command doctor
go test -count=1 -p=2 -timeout=30m ./...
go vet ./...
git diff --check
```

Canonical Go tests 用 `-count=1`、`-p=2`、`-timeout=30m`；`release-run` 整步上限 45 分钟，其余命令 5 分钟。Windows Job／Unix 进程组清理子孙，64 MiB 输出后仅 drain 5 秒。远程 workflow 先 vet 后 tests；`ciReleaseGate.ready` 只验证定义，不代表 remote green。完整边界见 `docs/release-readiness.md`。

按需追加：改 façade/compatibility 时运行 `rekit/tests/facade-smoke.ps1`；改 pack wrapper 时运行对应 validate/smoke；涉及 workstream/ledger/gate/sync/promote 写入时用临时 case 验证。
