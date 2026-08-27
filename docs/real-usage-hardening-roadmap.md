# STeamAI 产品优化路线

## 读取指南

本文件是当前已批准路线的唯一 active source，只展开当前 residual milestone。先由 `docs/context-routing.md` 路由到本文件；恢复工作时只读顶部执行区和当前卡，不预读历史批次实现。已完成的 numbered batches 按 ID 查询 `docs/batch-history.md`，P0～P3 原始批准与旧 task ledger 只作审计证据，不成为第二份路线。

## 实施摘要

当前路线是 `steamai-product-optimization-v1` 的获批 P0～P3 residual closure。Batch 830～833与validation repair只是局部证据；2026-08-24～25复核确认 C1～C4 已按序闭合，但不能把局部完成提升为总体完成。

`P0P3-C1`、`P0P3-C2`、`P0P3-C3`、`P0P3-C4` 与 retired pack identity review-first migration 已完成。当前 residual work 只剩 binary-re 专项验收校准，完成后才能执行完整 P0～P3 验证。全过程不创建 Batch 834、不自选新功能，也不回滚 Batch 830～833 或已保留的 `web-security` baseline。

产品获取方式保持 `source-clone-first`：Go 与 Claude Code 是预期依赖；不实现 installer。current 项目继续拒绝 PATH/外部 kit fallback，默认 quickstart 只保留 `cd <project> → claude → /steamai`。Go-native、project-local runtime 和现有权限边界保持不变。

### 当前指针

| 字段 | 当前值 |
|---|---|
| 路线 | `steamai-product-optimization-v1` |
| 当前批次 | `路线收口 binary-re 专项验收校准` |
| 状态 | `in_progress` |
| 唯一允许领取 | 路线收口 |
| 上一批 | retired pack identity migration 已完成并通过 focused/package/document validation |
| 下一批 | 完整 P0～P3 验证 |
| canonical repository | `https://github.com/shuiyu486/STeamAI` |
| 暂保留兼容身份 | Go module `github.com/shuiyu486/re-context-kits`、内部 `rekit` names、legacy `/rekit` / `.rekit` |

## 执行清单

- [x] **Batch 830～833 + validation baseline**：已完成，细节见 `CHANGELOG.md` 与 `docs/batch-history.md`；不代表 residual route完成。
- [x] **P0P3-C1～C3**：capability sinks、orchestration non-authorization、DTO/receipt/session/skill 已闭合；C3 exact skill provenance、immutable stamped publication与审计结论见 `CHANGELOG.md`。
- [x] **P0P3-C4 — runtime ownership**：完成原 #38；69 个 exact runtime owners 通过机器 inventory，continue 锁后 lane identity 与 gate 孤立 report binding 的窄复现已 fail-closed；未发现 control/session/provider 的可复现平行 owner。
- [x] **retired pack identity migration closure**：`vmp-re` / `generic-binary-re` 只经显式 pre-runtime owner review-first、exact-plan、single-write迁移为canonical `binary-re`；root/onboarding projection与receipt replay provenance已闭合，不alias、双写或自动择优。
- [ ] **binary-re 专项验收校准**：区分 synthetic、real Claude 与真实目标/工具证据，收窄或实证 VMP producer 能力，并投影 established-case enabled specialties。
- [ ] **完整获批方案验收**：逐项复核 P0～P3，运行产品/focused/full gates；未完成项、synthetic-only evidence 或待专项边界不得写成总体完成。

## 当前批次卡

### 路线收口：

**状态**：in_progress；retired migration 已完成，本卡完成并验证后才解锁完整 P0～P3 验证。

**来源**：已批准 P0～P3 residual route 的 binary-re 专项验收校准。前置 C1～C4 与 retired migration 已闭合；本卡不新增 pack、通用分析引擎或未授权执行能力。

**目标**：让 established-case status显式投影 enabled specialties；校准VMP producer能力声明；把synthetic、real Claude与真实目标/工具receipt分层，保证public maturity与实际证据一致。

**执行边界**：

1. 先从现有 manifest/catalog/status/adapter/acceptance owners审计 established-case specialties与VMP producer证据，不创建平行状态机或第二套capability registry。
2. VMP 能力只保留已有 IDA index 的 bounded inspection，或由真实 strict-gated producer与独立review receipt实证；不能用fixture、catalog字段或prompt声明替代producer证据。
3. synthetic fixture、real Claude session、真实目标/工具 execution receipt必须使用typed字段清楚分层；三者不能相互冒充。
4. 不写真实样本、trace/dump/capture、绝对case路径或case-specific进度；不执行未授权heavy action，不改变authority/confirmed边界。
5. focused产品路径与fresh package验证通过后才切换完整P0～P3验收；本卡局部成功不代表路线总体完成。

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

- 2026-08-28：retired pack identity migration完成；两个retired identities的review-first transaction、canonical root/onboarding projection、exact replay provenance、copy/move与capability fail-closed已通过审查和focused/package/document validation；active card切换到binary-re专项验收校准，完整P0～P3验证仍未完成。
- 2026-08-25：`P0P3-C4` 完成；scoped owner inventory、continue lock-after-reparse identity guard、gate orphaned report-binding rejection 与未复现候选边界见 `CHANGELOG.md`。
- 2026-08-25：`P0P3-C3` 完成并切换到 C4；exact skill provenance、immutable stamped publication与审计结论见 `CHANGELOG.md`。
- 2026-08-25：`P0P3-C1/C2` 完成；capability/control lineage 与 orchestration non-authorization 细节见 `CHANGELOG.md`。
- 2026-08-24：原始 P0～P3批准与 task tool ledger 复核发现 #37/#38/#50/#52 明确 pending 且无取消记录；撤销 `completed / no-next` 假收口，保持同一路线并恢复 `P0P3-C1`～`C4` residual sequence。
- 2026-08-24：validation repair以 `870d908` direct implementation commit推送并通过post-push receipt；该结果只关闭validation truth，不关闭 residual product work。
- 2026-08-20～23：P0～P3 大部分 architecture/public UX/production admission slices落地；shared capability contract与command owner inventory完成，四项 dependent closure被保留为 pending。
- 2026-08-19：Batch 833完成ordinary `binary-re` actual adapter lifecycle、independent evidence review、accepted-review binding与terminal durable-tail recovery。
- 2026-08-17：用户批准调整后的四阶段路线；2026-08-20 又明确批准更大的 P0～P3 优化方案及架构收敛顺序。
