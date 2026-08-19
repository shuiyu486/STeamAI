# STeamAI 产品优化路线

## 读取指南

本文件是当前已批准路线的唯一 active source，只展开最近完成批次卡。先由 `docs/context-routing.md` 路由到本文件；恢复工作时只读顶部执行区和当前卡，不预读后续阶段的实现细节。已完成的更早批次按 ID 查询 `docs/batch-history.md`，专题合同仍按需进入。

## 实施摘要

当前路线是 `steamai-product-optimization-v1`。路线固定为四个有序阶段：先关闭真实产品闭环，再把 `vmp-re` 与 `generic-binary-re` 收敛为唯一 `binary-re`，随后实现 durable pause/resume/stop，最后把 VMP/IDA actual adapter 作为 `binary-re` capability 接入普通真实分析闭环。Batch 830～832 已完成，Batch 833 是唯一允许领取的下一批；路线级完整验证仍等四阶段全部完成后统一执行。

产品获取方式保持 `source-clone-first`：canonical repository 是 `https://github.com/shuiyu486/STeamAI`，Go module `github.com/shuiyu486/re-context-kits` 暂作 compatibility identity。Go 与 Claude Code 是预期依赖；不实现 installer、portable ZIP、MSI、winget、下载站或自动更新。current 项目继续拒绝 PATH/外部 kit fallback，默认 quickstart 只保留 `cd <project> → claude → /steamai`。

### 当前指针

| 字段 | 当前值 |
|---|---|
| 路线 | `steamai-product-optimization-v1` |
| 当前批次 | `Batch 832` durable execution control |
| 状态 | `completed` |
| 唯一允许领取 | `Batch 833` |
| 上一批 | `Batch 831` binary-re convergence 已完成并归档 |
| 下一批 | `Batch 833` binary-re actual analysis |
| canonical repository | `https://github.com/shuiyu486/STeamAI` |
| 暂保留兼容身份 | Go module `github.com/shuiyu486/re-context-kits`、内部 `rekit` names、legacy `/rekit` / `.rekit` |

## 执行清单

- [x] **Batch 830 — core product closure（completed）**：关闭 Reviewer strict intake、ordinary-directory typed adoption、project-local promote、current `/steamai` public projection、release truth、actual adapter root binding 与可重定位 Identity v2。
- [x] **Batch 831 — binary-re convergence（completed）**：发布唯一 active `packs/binary-re`；不保留 alias、双写或自动迁移，旧 pack identity 只返回 typed `pack-migration-required`。
- [x] **Batch 832 — durable execution control（completed）**：实现独立 control generation、append-only pause/resume/stop receipt、result birth/currentness binding、held/late ledger、consumer progression guard，以及 durable-first exact local supervisor stop actuation。
- [ ] **Batch 833 — binary-re actual analysis（next）**：把 VMP/IDA adapter 作为 `binary-re` capability 接入 ordinary daily、独立 evidence review 和 accepted-review 后绑定；用 harmless synthetic input 完成真实 E2E 与 terminal replay。
- [ ] **路线收口**：Batch 833 完成后执行路线级完整 fresh tests、vet、module verify、release-check/status/packs/doctor、移动/复制 E2E、真实 Claude acceptance 与临时计划/评估文档清理，再完成最终 commit/push 和 Git-local inspection。

## 当前批次卡

### Batch 832：durable execution control

**状态**：completed；Batch 833 已解锁为唯一允许领取的下一批。

**目标**：让用户能从 `/steamai` 或自然语言 review-first 地暂停、恢复或停止 exact lane；控制状态必须先 durable 提交，运行结果、consumer progression 和本地进程 actuation 都只能在同一 control generation/current owner 下继续，且 control 不能成为授权来源。

**完成结果**：

- 新增 Go-owned public `control` command，`pause` / `resume` / `stop` 使用 per-lane append-only intent/receipt 与独立 `ControlGeneration`；WhatIf 零写入，Apply 必须消费 exact publication stamp 与 plan SHA；current `.steamai` 和 legacy `.rekit` 继续按 resolved state root 单写，dual root 在写入前 fail-closed；
- member、Reviewer、external-session 与 supervisor result 在 birth 时捕获 exact control binding；raw execution truth保留，但paused/stopped/旧generation的结果只进入stable held/late/stale receipt，不进入live outputs、intake、Reviewer writeback、completion或checkpoint progression，resume不自动释放旧结果；
- relay、checkpoint claim、member/reviewer writer 与 completion 在各自 mutation lease 内重新验证 binding，覆盖 pause-before-relay、pause-after-relay-before-claim 与 claim-after-pause-before-writer 三层竞争；paused/stopped public status 隐藏 executable loop/takeover，不靠放宽 projection 恢复；
- `stop` 先提交 durable stopped receipt；exact local supervisor child 随后观察该 head，发布 run-scoped request，仅关闭自己持有的 Windows Job/containment handle并追加 observation。actuation失败不回滚stopped，terminal raw truth继续独立记录，process termination不作为durable stop成功判据；`pause`不做OS suspend；
- opaque Remote Control session、endpoint或delivery observation不进入本地actuation；不按裸PID管理进程，不自动重发uncertain delivery，也不把control receipt、request SHA、transport或process observation提升为authority/confirmed、gate或heavy-action授权；
- public command/profile/release inventory已收敛为32个命令；PowerShell façade只校验并逐字透传control preview/exact Apply参数，不读取control head、不计算generation/hash、不写receipt、不管理进程。

**完成证据**：

- execution-control state/root、CLI control、binding/result publication、consumer progression、external-session lineage、sessionhost recovery/supervision与真实Windows Job close均有focused fresh测试；
- replacement result publication复用既有project lease，修复project→lane嵌套获取死锁；evidence-review zero-launch路线在trusted Claude解析前返回，不因本地Claude路径不可用误阻塞；
- façade smoke、32-command release/default-doc inventory、current/legacy单写与dual-root fail-closed focused tests通过；本批不把本地focused结果冒充remote CI green或formal release。

**完成边界**：

- control generation独立于executor generation、external attempt generation、supervisor run ID与gate authorization；stopped为终态，pause/resume不恢复旧session budget或旧结果；
- 没有新增PowerShell runtime logic、PID kill、OS suspend、Remote Control session管理、authority/confirmed写入或未授权heavy action；
- Batch 833的`binary-re` ordinary actual adapter能力尚未实施，本卡不提前声称真实分析闭环已完成。

## 验证标准

```text
go test -count=1 -p=2 -timeout=30m ./...
go vet ./...
go mod verify
go run ./cmd/rekit -- -Command release-check -Format json
go run ./cmd/rekit -- -Command status
go run ./cmd/rekit -- -Command packs
go run ./cmd/rekit -- -Command doctor
git diff --check
```

每批先跑对应 focused tests；路线级完整门禁只在四阶段收口时统一运行。workflow definition、本地 receipt、tracking ref 与 Markdown claim 都不能提升为 remote CI green 或 formal release。

## 风险与注意事项

- 四个阶段已批准不等于可以并发混写；只有 Batch 833 可继续修改产品，Batch 832 只在发现回归时做同批修复。
- `binary-re` convergence 是 identity cutover，不做 alias 或机械全文替换；历史批次和旧 recovery generation 保留当时事实。
- durable control 与 executor generation、gate authorization分离；process termination不是durable stop成功判据，actuation failure不回滚stopped。
- `.steamai` / `.rekit` 继续 dual-read/single-write，both fail-closed；current 项目不得回退中央 source runtime。
- `rekit/rekit.ps1` 仅保留 legacy compatibility façade；公共 façade 删除仍受独立门禁约束。

## 路线变更记录

- 2026-08-19：Batch 831归档；Batch 832完成durable pause/resume/stop、held/late result isolation、consumer progression guard与exact local supervisor durable-first stop actuation，解锁Batch 833 binary-re actual analysis。
- 2026-08-18：Batch 830归档；Batch 831完成唯一active `binary-re`、retired identity typed migration与source-clone-first收敛，解锁Batch 832 durable execution control。
- 2026-08-17：基于真实源码、fresh tests、临时项目、真实 Claude member/Reviewer 与 actual adapter 复评，批准 `steamai-product-optimization-v1` 四阶段路线；明确 source-clone-first、无 installer、唯一 `binary-re`。
- 2026-08-17：`steamai-repository-identity-v1` / Batch 829 完成后归档；canonical repository 保持 `shuiyu486/STeamAI`，Go/internal/legacy identity 继续兼容。
