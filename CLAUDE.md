# CLAUDE.md

## 项目定位

本仓库是 STeamAI canonical 实现（`https://github.com/shuiyu486/STeamAI`）：安全研究向 Claude Code Agent Team Mission Control，组织 Mission Commander、lanes、sessions、subagents、pack、证据和门禁。`github.com/shuiyu486/re-context-kits` 只是 Go module compatibility identity。

产品北极星见 `docs/mission-control-product-direction.md`：一个真实项目目录就是一个自包含 STeamAI 项目。用户在目录内启动已有 Claude Code，主要指挥主 Agent；新项目用 `/steamai`、`.steamai` 和随项目发布的 runtime/selected pack。`/rekit`、`.rekit`、中央 kit/thin-shim 仅用于迁移兼容。成员身份绑定 lane；heavy action 只认 active root 的 strict autonomy profile + `authorized-gate`。

本仓库不是安全/RE case 或自动分析、漏洞挖掘、渗透引擎；只为验证产品行为创建临时 case。

## 文档不变量 / 上下文路由

本项目文档必须做成按需路由、渐进式披露的样式；`docs/context-routing.md` 是唯一完整路由表。只读必要边界、router、仓库状态，不串读路线图、batch plan、CHANGELOG、release readiness 或历史；active docs 只留当前决策/短指针。

## 维护入口

- `/steamai` canonical skill：`.claude/skills/steamai/SKILL.md`；legacy `/rekit` compatibility skill：`.claude/skills/rekit/SKILL.md`
- Go front door：`cmd/rekit/main.go`、`internal/rekit/**`；public owner 是 `internal/rekit/cli/public_frontdoor.go`，dispatch 不重建状态机
- session/external/adapter：`cmd/rekit-host/**`、`internal/rekit/{sessionhost,externalsession}/**`、`cmd/rekit-adapter-{host,acceptance}/**`
- compatibility：`rekit/rekit.ps1`（不新增业务 runtime）；pack/common：`packs/<pack>/**`、`common/**`
- release/docs：`docs/context-routing.md`、`docs/release-readiness.md`、`docs/powershell-deprecation.md`

## CodeGraph 使用边界

维护源码、模板、文档时优先用 CodeGraph 查看结构、调用链与影响面；返回源码视为已读。不得用于样本、二进制、trace/dump/capture 或 case artifact；安全/RE case 仍走 pack、lane packet、工具链、证据和授权边界。

## 当前推进原则

当前已批准路线是 `steamai-architecture-product-convergence-v1`；四阶段已完成，当前无 owner/next batch。source-clone-first，不做 installer，兼容 legacy `/rekit`/`.rekit`。Machine publication 认 Git-local typed receipt、direct commit、本地 tracking ref；cross-compile 不是运行证据。

保持 PowerShell-free/Go-native，不增 PowerShell runtime logic。避免单字段微批次；公共入口删除门禁不完整则升级。

## 关键边界

- `sync` 是 kit -> case；`promote` 是 case -> kit；均 review-first，写前确认范围。
- `continue -Apply` 不写 authority/confirmed、不执行 heavy-tool。
- Executable Mission Commander request 携带与 command/receipt 一致的 bounded typed invocation；多 lane 未选时不发布，selected artifact 只用 exact `-Lane`。Request SHA 只证明 currentness，不授权。
- 普通 public `continue` 只按 `-WhatIf -Format json` preview → 同 selector/owner/generation 的 exact `-Apply` → fresh status 推进。`continuePlanSha256` 绑定完整 mutation snapshot，Apply 原样携带 `-ExpectedContinuePlanSha256`；blocked preview 不发布 Apply。不得手工拼 phase、复用已执行 request，或分开改写 command/typed invocation。
- daily 自然语言 operation 只由一个 classifier 选择 resume/goal/correction/control/adoption；`-Lane` 只是 selector，不得被解释为 control intent。
- 自然语言纠偏从 fresh typed state 唯一选路：Reviewer rejection 走既有 correction/reconcile；committed completion 只走 `reopen` preview/exact Apply，返回 `ready-to-continue` + `NoAutoResume`，不自动接管或启动 Claude。
- `mission-complete` 后的新目标只走 successor preview/exact Apply：绑定 predecessor intent/closure，commit 与 active pointer 最后发布，激活 `.steamai/missions/gNNNNNN` 并保留旧 audit tree；返回 `ready-to-continue` + `NoAutoResume`，不启动 Claude。用户不填 SHA；same-goal 只 committed replay，legacy/stale/corrupt partial transition fail-closed。
- `gate -Apply` 只写 gate decision 或 authorized execution observation，不执行 heavy action；actual heavy action 由 executor/adapter 在 strict profile + `authorized-gate` 内执行并留证。
- transport/endpoint/delivery observation 不授予权限；Remote Control uncertain delivery 不重发或 same-job replacement；opaque endpoint 不是 durable identity。
- authority/confirmed、schema migration、公共 façade 删除门禁、未授权副作用或难判断架构取舍，需要升级。
- 模板仓库不得写入真实样本、trace/dump/capture、artifact、绝对 case 路径、payload、flag 或 case 进度。
- 新项目 `/steamai` 只用 project-local verified runtime bundle，不得回退机器 PATH 或外部 kit；legacy `/rekit` thin shim 只兼容旧 `.rekit`。
- `.steamai`/`.rekit` dual-read/single-write：current-only 写 `.steamai`，legacy-only 写 `.rekit`，neither 选 `.steamai`，both fail-closed；不双写、合并、择优或接受 reparse alias。
- case public JSON 的 project-local typed command 由 resolved state root 统一投影：current 只显示 `/steamai`，legacy 只显示 `/rekit`；只遍历显式 typed structure，不替换 JSON prose 或 durable/source identity。
- project-local no-mode `help/status/continue` 只做用户摘要；diagnostics 与 exact preview/apply 规则见 `.claude/skills/steamai/SKILL.md`，current 默认不得泄漏 `/rekit`。
- fresh 未接入 status 只读发布 schema-valid、非 template pack choices；pending onboarding publication 不重新选 pack，只发布绑定既有 identity/stamp/plan 的唯一 exact Apply recovery；committed missing-board 仍走 `overview` bootstrap。三者不得混用，status 不写项目。
- `bounded-autonomous-v1` 只显式 opt-in 单 lane/exact action/exact target/有限预算/短 expiry，每次仍重验并留证；不授予无限权限、authority/confirmed、sync/promote 或 schema migration。
- exact lane `control` 使用独立 append-only generation + review-first stamp/hash Apply；pause 不做 OS suspend；stop 先 durable 提交，仅 exact local supervisor owner 可关闭其 containment。actuation 失败不回滚 stopped，process termination 不证明 durable stop，opaque Remote Control 不受管。current member handoff/checkpoint/attempt/observation 共用 birth control lineage，missing/stale 只读或 held、不补采 generation；legacy nil 兼容。旧 generation 不推进 live output、Reviewer、completion/checkpoint。
- current-sync Apply 与 current `.steamai` detached-supervisor handoff 依赖 handle-bound exact filesystem mutation；Windows 提供，非 Windows 持久化副作用前 fail-closed；read-only/preview 与 legacy compatibility 保持可用。

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
