# STeamAI 产品优化路线

## 读取指南

本文件是当前已批准路线的唯一 active source，只展开当前 residual milestone。先由 `docs/context-routing.md` 路由到本文件；恢复工作时只读顶部执行区和当前卡，不预读历史批次实现。已完成的 numbered batches 按 ID 查询 `docs/batch-history.md`，P0～P3 原始批准与旧 task ledger 只作审计证据，不成为第二份路线。

## 实施摘要

当前路线是 `steamai-product-optimization-v1`，现处于获批 P0～P3 residual closure。Batch 830～833、路线级真实 Claude/actual-adapter validation 与 2026-08-24 validation repair 已完成各自局部闭环，但它们不等于全部获批 P0～P3 已完成。2026-08-24 从原始用户批准和 task tool ledger 复核确认：遗留 #50 capability control sinks、#37 P3-4 orchestration authorization boundary、#52 DTO/receipt/session/skill、#38 P2 runtime ownership 在 2026-08-23 被明确恢复为 pending，次日另建 continuation tasks；没有用户取消记录。此前 `completed / no-next` 是假收口，现恢复同一路线 residual closure。

`P0P3-C1` 与 `P0P3-C2` 已完成，当前唯一 residual milestone 是 `P0P3-C3`。主链仍按 `C1` #50 → `C2` #37 → `C3` #52 → `C4` #38 推进；C4 后还须完成已保留的 retired pack identity review-first migration 与 binary-re 专项验收校准，最后才能执行完整 P0～P3 验证。全过程不创建 Batch 834、不自选新功能，也不回滚 Batch 830～833 或已保留的 `web-security` baseline。

产品获取方式保持 `source-clone-first`：Go 与 Claude Code 是预期依赖；不实现 installer。current 项目继续拒绝 PATH/外部 kit fallback，默认 quickstart 只保留 `cd <project> → claude → /steamai`。Go-native、project-local runtime 和现有权限边界保持不变。

### 当前指针

| 字段 | 当前值 |
|---|---|
| 路线 | `steamai-product-optimization-v1` |
| 当前批次 | `P0P3-C3 DTO/receipt/session/skill` |
| 状态 | `in_progress` |
| 唯一允许领取 | `P0P3-C3` |
| 上一批 | `P0P3-C2 orchestration non-authorization` 已完成并通过 fresh local validation |
| 下一批 | `P0P3-C4 runtime ownership` |
| canonical repository | `https://github.com/shuiyu486/STeamAI` |
| 暂保留兼容身份 | Go module `github.com/shuiyu486/re-context-kits`、内部 `rekit` names、legacy `/rekit` / `.rekit` |

## 执行清单

- [x] **Batch 830 — core product closure**：Reviewer strict intake、ordinary-directory adoption、project-local promote、current `/steamai` projection、release truth、adapter root binding 与 relocatable identity。
- [x] **Batch 831 — binary-re convergence**：唯一 active `binary-re`，旧 identity 显式迁移，不 alias、双写或静默重写。
- [x] **Batch 832 — durable execution control**：append-only pause/resume/stop、generation fencing、held/late result 与 durable-first local containment actuation。
- [x] **Batch 833 — binary-re actual analysis**：ordinary daily actual adapter、independent evidence review、accepted-only binding 与 terminal recovery。
- [x] **路线 validation baseline**：默认 `binary-re` 真实 Claude acceptance、`web-security` retained production baseline、Windows LF repair、7/7 local minimum 和 `870d908` post-push receipt。
- [x] **P0P3-C1 — capability control sinks**：adapter parent/private child 使用 inherited exact lane-lock handle；immutable dispatch 与 Reviewer birth packet 冻结 capability/control lineage；current 缺失、stale、paused、stopped 在 effect sink 前拒绝，late result仅保留 raw/held truth；Remote transport observation 不授权。
- [x] **P0P3-C2 — orchestration non-authorization**：完成原 #37；current member handoff/external attempt/observation冻结并消费同一transport execution-control birth lineage，status与checkpoint claim在副作用前拒绝missing/stale control；raw-result recovery只保留exact frozen truth。legacy nil lineage继续兼容，tool/endpoint/provider/transport事实仍不授权。
- [ ] **P0P3-C3 — DTO/receipt/session/skill（current）**：完成原 #52；immutable diagnostics projection、canonical receipt/publication identity、production dependency direction 与 skill provenance。
- [ ] **P0P3-C4 — runtime ownership**：完成原 #38；descriptor-owned binder/validator/handler/profile/policy，消除目标链平行 dispatch 与共享可变投影。
- [ ] **retired pack identity migration closure**：为 `vmp-re` / `generic-binary-re` 到 canonical `binary-re` 提供 review-first、exact-plan、single-write migration，不 alias、双写或自动择优。
- [ ] **binary-re 专项验收校准**：区分 synthetic、real Claude 与真实目标/工具证据，收窄或实证 VMP producer 能力，并投影 established-case enabled specialties。
- [ ] **完整获批方案验收**：逐项复核 P0～P3，运行产品/focused/full gates；未完成项、synthetic-only evidence 或待专项边界不得写成总体完成。

## 当前批次卡

### P0P3-C3：DTO/receipt/session/skill

**状态**：in_progress；唯一允许领取 `P0P3-C3`。

**来源**：2026-08-20 获批 P0～P3 的 DTO/receipt/session/skill 合同，及旧 task #52。前置 `P0P3-C2` 已闭合真实 member/model-tool handoff 的非授权 birth/currentness 边界；本卡不重写 session architecture，不实现尚未接入的 hooks、SessionStore、Agent Teams 或 MCP provider。

**目标**：核验 canonical DTO、receipt/publication identity、session handoff与project-local skill provenance的真实消费链；优先补 non-mutating `skillcontractgen -check`、direct bundle/init provenance与production dependency direction 的已复现缺口。

**验收**：

1. 生成合同的 check mode零写入，并能拒绝skill/runtime bundle provenance漂移。
2. public diagnostics DTO、durable receipt/publication与session handoff只投影唯一canonical identity，不从prose、transport acknowledgement或外部PATH重建身份。
3. current项目只消费project-local verified runtime/skill bundle；legacy兼容保持dual-read/single-write，不新增installer或PowerShell runtime logic。
4. focused/product/full local validation通过；只解锁`P0P3-C4`，不跳到pack migration、binary-re专项或自选路线。

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
- 不把 residual work扩张为 GUI、installer、新 pack、通用代理或落地机出口；这些不是当前路线的隐含内容。如需新增，必须作为独立用户目标与验收范围处理。
- 不重写 durable state、gate、receipt、generation 或权限模型；优先复用现有 owners，删除平行实现而非再加一层。
- `gate -Apply`仍只记录 decision/observation；actual heavy action只在 strict profile + fresh `authorized-gate` 内执行。
- 不把真实样本、trace、dump、capture、artifact、绝对 case 路径、payload、flag、客户信息或 case-specific 进度写入模板仓库。
- 不声称未读取的 remote CI green、非 Windows runtime E2E、真实跨机器 Remote Control E2E 或 formal release。

## 路线变更记录

- 2026-08-25：`P0P3-C2` 完成。current member handoff、checkpoint、external attempt/submission/observation共用exact transport execution-control birth lineage；missing/stale control只保留diagnostic/raw/held truth，ordinary status隐藏旧checkpoint carrier，checkpoint claim与member writer在副作用前fail-closed；不能消费旧checkpoint的fresh typed request仍可建立新birth，pause→resume不能重建新generation或推进旧结果。legacy nil lineage继续兼容；Remote uncertain delivery与transport/provider observation不授权。fresh focused/full Go tests、vet、module verify及公开release/status/packs/doctor通过，唯一current切换到`P0P3-C3`。
- 2026-08-25：`P0P3-C1` 完成。adapter parent/private child、immutable adapter dispatch、Reviewer birth/session/result 与 Remote transport 权限边界已按 existing lane lease、execution-control binding 和 capability contract闭合；fresh focused/full Go tests、vet、module verify及公开 release/status/packs/doctor通过，唯一 current切换到 `P0P3-C2`。
- 2026-08-24：原始 P0～P3批准与 task tool ledger 复核发现 #37/#38/#50/#52 明确 pending 且无取消记录；撤销 `completed / no-next` 假收口，保持同一路线并恢复 `P0P3-C1`～`C4` residual sequence。
- 2026-08-24：validation repair以 `870d908` direct implementation commit推送并通过post-push receipt；该结果只关闭validation truth，不关闭 residual product work。
- 2026-08-20～23：P0～P3 大部分 architecture/public UX/production admission slices落地；shared capability contract与command owner inventory完成，四项 dependent closure被保留为 pending。
- 2026-08-19：Batch 833完成ordinary `binary-re` actual adapter lifecycle、independent evidence review、accepted-review binding与terminal durable-tail recovery。
- 2026-08-17：用户批准调整后的四阶段路线；2026-08-20 又明确批准更大的 P0～P3 优化方案及架构收敛顺序。
