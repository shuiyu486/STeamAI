# STeamAI 产品优化路线

## 读取指南

本文件是当前已批准路线的唯一 active source，只展开当前 residual milestone。先由 `docs/context-routing.md` 路由到本文件；恢复工作时只读顶部执行区和当前卡，不预读历史批次实现。已完成的 numbered batches 按 ID 查询 `docs/batch-history.md`，P0～P3 原始批准与旧 task ledger 只作审计证据，不成为第二份路线。

## 实施摘要

当前路线是 `steamai-product-optimization-v1`，现处于获批 P0～P3 residual closure。Batch 830～833、路线级真实 Claude/actual-adapter validation 与 2026-08-24 validation repair 已完成各自局部闭环，但它们不等于全部获批 P0～P3 已完成。2026-08-24 从原始用户批准和 task tool ledger 复核确认：遗留 #50 capability control sinks、#37 P3-4 orchestration authorization boundary、#52 DTO/receipt/session/skill、#38 P2 runtime ownership 在 2026-08-23 被明确恢复为 pending，次日另建 continuation tasks；没有用户取消记录。此前 `completed / no-next` 是假收口，现恢复同一路线 residual closure。

本次不创建 Batch 834、不自选新功能，也不回滚 Batch 830～833 或已保留的 `web-security` baseline。唯一顺序为 `P0P3-C1` #50 → `P0P3-C2` #37 → `P0P3-C3` #52 → `P0P3-C4` #38；只有四项及整体产品验证均完成，路线才能重新标记 `completed`。

产品获取方式保持 `source-clone-first`：Go 与 Claude Code 是预期依赖；不实现 installer。current 项目继续拒绝 PATH/外部 kit fallback，默认 quickstart 只保留 `cd <project> → claude → /steamai`。Go-native、project-local runtime 和现有权限边界保持不变。

### 当前指针

| 字段 | 当前值 |
|---|---|
| 路线 | `steamai-product-optimization-v1` |
| 当前批次 | `P0P3-C1 capability control sinks` |
| 状态 | `in_progress` |
| 唯一允许领取 | `P0P3-C1` |
| 上一批 | `Batch 833` binary-re actual analysis 与随后 validation repair 已完成 |
| 下一批 | `P0P3-C2 orchestration non-authorization` |
| canonical repository | `https://github.com/shuiyu486/STeamAI` |
| 暂保留兼容身份 | Go module `github.com/shuiyu486/re-context-kits`、内部 `rekit` names、legacy `/rekit` / `.rekit` |

## 执行清单

- [x] **Batch 830 — core product closure**：Reviewer strict intake、ordinary-directory adoption、project-local promote、current `/steamai` projection、release truth、adapter root binding 与 relocatable identity。
- [x] **Batch 831 — binary-re convergence**：唯一 active `binary-re`，旧 identity 显式迁移，不 alias、双写或静默重写。
- [x] **Batch 832 — durable execution control**：append-only pause/resume/stop、generation fencing、held/late result 与 durable-first local containment actuation。
- [x] **Batch 833 — binary-re actual analysis**：ordinary daily actual adapter、independent evidence review、accepted-only binding 与 terminal recovery。
- [x] **路线 validation baseline**：默认 `binary-re` 真实 Claude acceptance、`web-security` retained production baseline、Windows LF repair、7/7 local minimum 和 `870d908` post-push receipt。
- [ ] **P0P3-C1 — capability control sinks（current）**：完成原 #50；effect sink 前消费 exact capability/control lineage，pause/stop/stale fail-closed，late result held。
- [ ] **P0P3-C2 — orchestration non-authorization**：完成原 #37；custom tools/MCP/tactical subagents/Reviewer 依附 durable typed command、owner/generation/currentness/gate，外部 observation 不授予权限。
- [ ] **P0P3-C3 — DTO/receipt/session/skill**：完成原 #52；immutable diagnostics projection、canonical receipt/publication identity、production dependency direction 与 skill provenance。
- [ ] **P0P3-C4 — runtime ownership**：完成原 #38；descriptor-owned binder/validator/handler/profile/policy，消除目标链平行 dispatch 与共享可变投影。
- [ ] **完整获批方案验收**：逐项复核 P0～P3，运行产品/focused/full gates；未完成项、synthetic-only evidence 或待专项边界不得写成总体完成。

## 当前批次卡

### P0P3-C1：capability control sinks

**状态**：in_progress；唯一允许领取 `P0P3-C1`。

**来源**：2026-08-20 获批 P0～P3 的架构/授权边界，及旧 task #50。其前置 #48 exact production instruction consumption 与 #49 shared capability contract 已完成；本卡不重做 registry 或 production packs。

**目标**：审计并闭合 adapter parent/private child、direct Reviewer lifecycle mutation、Remote transport/current model-tool handoff 的真实 effect sinks。每个 sink 必须在副作用前消费 exact capability contract 与 execution-control lineage；paused/stopped/stale generation 拒绝推进，late result只进入 held truth。transport、endpoint、delivery/provider acknowledgement、hooks、SessionStore、Agent Teams、MCP 或 custom tool presence 均不能构造 authority、confirmed、`authorized-gate` 或 heavy-tool grant。

**验收**：

1. 为每个仍存在的 effect sink提供复现测试，证明缺失、stale、paused、stopped 或不匹配 lineage 在副作用前 fail-closed；不得只检查 DTO 字段存在。
2. 合法 current binding 保持现有 adapter/Reviewer/transport 产品路径，不创建平行 state machine。
3. late result保存 raw/held truth，但不能进入 live output、Reviewer writeback、completion、checkpoint 或 task binding。
4. focused packages、受影响产品路径与 canonical local minimum通过；权限/authority/confirmed 无隐式写入。
5. 完成后只解锁 `P0P3-C2`，不得跳到 DTO/runtime track 或新 pack。

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

每个 residual milestone 先跑直接相关复现/focused tests；完整门禁只在闭合行为后运行，避免重复验证。`release-check.ready`、文档勾选、字段存在、cross-compile 或 synthetic fixture 都不能单独证明产品完成。路线完成必须同时满足原 P0～P3逐项证据、真实用户路径、fresh machine validation、direct commit 与本地 tracking ref。

## 风险与注意事项

- 不把“某次 local validation green”再次提升为“全部产品方案完成”；route completion与machine validation是两条独立真值。
- 不把四项 residual 扩张为 GUI、installer、新 pack、通用代理或落地机出口；这些不是当前四项的隐含内容。如需新增，必须作为独立用户目标与验收范围处理。
- 不重写 durable state、gate、receipt、generation 或权限模型；优先复用现有 owners，删除平行实现而非再加一层。
- `gate -Apply`仍只记录 decision/observation；actual heavy action只在 strict profile + fresh `authorized-gate` 内执行。
- 不把真实样本、trace、dump、capture、artifact、绝对 case 路径、payload、flag、客户信息或 case-specific 进度写入模板仓库。
- 不声称未读取的 remote CI green、非 Windows runtime E2E、真实跨机器 Remote Control E2E 或 formal release。

## 路线变更记录

- 2026-08-24：原始 P0～P3批准与 task tool ledger 复核发现 #37/#38/#50/#52 明确 pending 且无取消记录；撤销 `completed / no-next` 假收口，保持同一路线并恢复 `P0P3-C1`～`C4` residual sequence。
- 2026-08-24：validation repair以 `870d908` direct implementation commit推送并通过post-push receipt；该结果只关闭validation truth，不关闭 residual product work。
- 2026-08-20～23：P0～P3 大部分 architecture/public UX/production admission slices落地；shared capability contract与command owner inventory完成，四项 dependent closure被保留为 pending。
- 2026-08-19：Batch 833完成ordinary `binary-re` actual adapter lifecycle、independent evidence review、accepted-review binding与terminal durable-tail recovery。
- 2026-08-17：用户批准调整后的四阶段路线；2026-08-20 又明确批准更大的 P0～P3 优化方案及架构收敛顺序。
