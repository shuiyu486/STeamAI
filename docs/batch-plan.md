# Batch implementation plan

## 读取指南

本文件只是当前路线的短投影，不选题、不保存完整实施日志。先由 `docs/context-routing.md` 选择场景；实施当前路线时，以 `docs/real-usage-hardening-roadmap.md` 的当前卡为唯一 source。STeamAI 自包含细节按需读 `docs/steamai-self-contained-project.md`；完整历史只在 `docs/batch-history.md` 按 ID 查询。

## 实施摘要

当前路线是 `steamai-self-contained-project-v1`，Batch 828 已完成 Windows 本机产品闭环：默认产品从中央 kit + case thin shim 迁移为一个真实项目目录内自带 `/steamai`、`.steamai`、verified runtime 和 selected pack 的 STeamAI 项目，并同时关闭 compact status、人话 recovery、bounded autonomy 和 legacy migration。主体 implementation commit 已推送到 `origin/main`；post-push 检查暴露的 Windows LF/CRLF checkout receipt 误报按 same-batch repair 合同收口。当前没有已批准下一批；最终 cadence 只以 fresh machine receipt 和本地 tracking ref 为准，不声称未读取的 remote CI green。

## 执行清单

### Current projection

| 字段 | 当前值 |
|---|---|
| 路线 | `steamai-self-contained-project-v1` |
| source | `docs/real-usage-hardening-roadmap.md` |
| 当前批次 | `Batch 828` STeamAI self-contained project closure |
| 状态 | `completed` |
| 唯一允许领取 | `Batch 828`（已完成的历史绑定；不可继续领取） |
| 上一批 | `Batch 827` maintenance hotspot decomposition and full Windows acceptance 已完成 |
| 下一批 | 无；等待用户明确改变路线 |

### Current batch state

STeamAI `/steamai` skill、`.steamai` dual-read/single-write state owner、compact status、人话 typed recovery、retry/handoff correctness、`bounded-autonomous-v1`、project-local runtime bundle、copied-directory/no-central-kit、hash-bound legacy migration、current-sync process recovery 与 durable supervisor handoff 均已实现并有 package/focused/E2E 证据。独立边界复核已关闭两个跨平台 capability 缺口；Git-local receipt 继续绑定 raw bytes 与 clean-filter blob，只额外接受 exact LF blob 的纯 CRLF checkout 物化，任意其它 byte drift 仍 fail-closed。Windows 完整 release minimum 已通过，路线停在 completed/no-next，不自动选题。

### Batch 828：STeamAI self-contained project closure

状态：已完成。

目标：一个真实项目目录携带自己的 skill、状态、runtime 和 selected pack；用户在项目中启动已有 Claude Code 后直接使用 `/steamai` 或自然语言。复制项目且中央 kit 不可用时，status/packs/doctor/daily 仍能工作。

验证结果：Windows current/legacy/dual-root、bundle/copy、compact/recovery/autonomy/migration 与真实 product-path E2E 通过；独立 capability 复核无 surviving blocker；显式 45 分钟上限的完整 Go tests、`go vet ./...`、`go mod verify`、public CLI `status` / `packs` / `doctor` 和 `git diff --check` 通过。local/CI canonical 命令保持有限 `go test -count=1 -p=2 -timeout=30m ./...`；`-count=1`禁用 package test-result cache并保证 frozen bytes fresh execution（仍可复用 Go build cache），`-p=2`限制跨 package并发，30分钟是逐 package test binary上限，release-run另以45分钟硬上限覆盖整条多 package command，普通步骤及receipt/inspection Git命令为5分钟。共享进程树runner在Windows使用suspended→non-breakaway Job→resume、在Unix使用独立process group；根退出、deadline或64 MiB输出上限均终止 containment 内的剩余子孙并回收root，随后只做5秒有限pipe drain；逃逸writer未关闭时明确失败，不再只杀根PID或永久等待后代pipe EOF。lease race 与 Windows reparse fixture保留10秒局部 watchdog。canonical `release-run` 只在冻结工作树上以新 exact 7/7 profile全绿后发布 Git-local machine receipt，当前 receipt 状态以 fresh `release-check` 为准。Linux/Darwin/FreeBSD 受影响包 cross-compile 通过，但仅为 compile-only，不是非 Windows runtime E2E。主体 implementation commit 已推送；同批 receipt repair 的最终 readiness 以 fresh post-push inspection 为准，不声称 remote CI green。

## 验证标准

- 本文件与路线图的 route/current/state/claim/next 必须一致；冲突时 fail-closed。
- active plan 只保留一个 compact batch 摘要；完整历史只在 `docs/batch-history.md`。
- runtime bundle、copied-directory、legacy migration 和 Windows local minimum 均有真实通过证据，Batch 828 才标为 completed。
- 远程 CI、Linux/macOS runtime E2E、三平台产品兼容、GUI/TUI 和独立安装器不参与当前完成判断；cross-compile 不得冒充平台运行证据。

## 风险与注意事项

- 本文件不是第二份 roadmap；具体合同只在当前卡与 `docs/steamai-self-contained-project.md`。
- 不把完整 receipts、逐轮日志、未来 backlog 或大 inventory 塞回本文件。
- 当前完成只表示 Windows 本机产品闭环与 local validation；implementation/repair 是否完成 push 只能由 fresh local refs、clean worktree 与 machine receipt 判断，也不自动解锁下一批或证明 remote CI green。
- 非 Windows 的 current-sync Apply 与 current durable detached-supervisor handoff 依赖 handle-bound exact mutation，当前在任何持久化副作用前 fail-closed；read-only/preview 与 legacy compatibility 仍按各自合同可用。
