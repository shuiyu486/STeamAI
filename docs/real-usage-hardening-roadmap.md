# STeamAI 产品优化路线

## 读取指南

本文件是当前已批准路线的唯一 active source，只展开当前 residual milestone。先由 `docs/context-routing.md` 路由到本文件；恢复工作时只读顶部执行区和当前卡，不预读历史批次实现。已完成的 numbered batches 按 ID 查询 `docs/batch-history.md`，P0～P3 原始批准与旧 task ledger 只作审计证据，不成为第二份路线。

## 实施摘要

当前路线是 `steamai-product-optimization-v1` 的获批 P0～P3 residual closure，现已完成。Batch 830～833、`P0P3-C1`～`P0P3-C4`、retired pack identity review-first migration、binary-re 专项验收校准与最终总验证均已有对应实现和证据；没有把任一局部 batch、inventory 或 synthetic fixture单独提升为总体完成。

当前没有已批准下一路线，不创建 Batch 834、不自选新功能，也不回滚 Batch 830～833 或已保留的 `web-security` baseline。后续只有显式用户路线变更才能建立新的 active work；Git-local v3 receipt、唯一 direct implementation commit与post-push tracking ref继续作为本次completed projection的独立machine publication truth，不由本段prose替代。

产品获取方式保持 `source-clone-first`：Go 与 Claude Code 是预期依赖；不实现 installer。current 项目继续拒绝 PATH/外部 kit fallback，默认 quickstart 只保留 `cd <project> → claude → /steamai`。Go-native、project-local runtime 和现有权限边界保持不变。

### 当前指针

| 字段 | 当前值 |
|---|---|
| 路线 | `steamai-product-optimization-v1` |
| 当前批次 | `路线收口 完整 P0～P3 验证` |
| 状态 | `completed` |
| 唯一允许领取 | 无；当前路线已完成，等待显式用户路线变更 |
| 上一批 | 路线收口完整 P0～P3 验证已通过 |
| 下一批 | 无；当前路线已完成，等待显式用户路线变更 |
| canonical repository | `https://github.com/shuiyu486/STeamAI` |
| 暂保留兼容身份 | Go module `github.com/shuiyu486/re-context-kits`、内部 `rekit` names、legacy `/rekit` / `.rekit` |

## 执行清单

- [x] **Batch 830～833 + validation baseline**：已完成，细节见 `CHANGELOG.md` 与 `docs/batch-history.md`；不代表 residual route完成。
- [x] **P0P3-C1～C3**：capability sinks、orchestration non-authorization、DTO/receipt/session/skill 已闭合；C3 exact skill provenance、immutable stamped publication与审计结论见 `CHANGELOG.md`。
- [x] **P0P3-C4 — runtime ownership**：完成原 #38；69 个 exact runtime owners 通过机器 inventory，continue 锁后 lane identity 与 gate 孤立 report binding 的窄复现已 fail-closed；未发现 control/session/provider 的可复现平行 owner。
- [x] **retired pack identity migration closure**：`vmp-re` / `generic-binary-re` 只经显式 pre-runtime owner review-first、exact-plan、single-write迁移为canonical `binary-re`；root/onboarding projection与receipt replay provenance已闭合，不alias、双写或自动择优。
- [x] **binary-re 专项验收校准**：established status只投影同pack executable owner、production contract与typed verified catalog完全一致的specialties；VMP仅保留已有IDA TSV bounded inspection；repository inventory、synthetic input、real contained child/Claude与未观察的producer/target-tool receipt已typed分层。
- [x] **完整获批方案验收**：原 P0～P3未取消项已逐项复核；focused/affected/full tests、vet、module verify、skill contract、public release/status/packs/doctor、diff/façade、fresh installed project-local real-Claude binary-re/web-security产品路径及最终只读反证均通过。Git-local v3 receipt、唯一 direct commit与post-push ref在completed projection后独立发布。

## 当前批次卡

### 路线收口：

**状态**：completed；C1～C4、retired migration、binary-re专项与最终总验证均已闭合。当前没有已批准下一路线，等待显式用户路线变更。

**来源**：已批准 P0～P3 residual route 的最终验收要求。本卡不新增功能，只核对原批准项、真实产品路径、机器门禁与Git-local publication truth。

**目标**：逐项证明所有未取消的P0～P3要求已有实现与足够证据；任何未闭合项保持open，不用局部测试、readiness inventory或文档claim代替总体完成。

**执行边界**：

1. 从原P0～P3批准、task ledger与当前源码逐项建立完成矩阵，合并重复项，不重新实现已闭合能力。
2. 运行产品入口、focused/full tests、vet/module/skill contract、release/status/packs/doctor与diff门禁；cross-compile不替代runtime evidence。
3. 核对真实用户路径、fresh machine receipt、direct commit与本地tracking ref；不把synthetic fixture、repository inventory或remote workflow定义提升为真实外部执行证据。
4. 不写真实样本、trace/dump/capture、绝对case路径或case-specific进度；不执行未授权heavy action，不改变authority/confirmed边界。
5. 只有全部未取消项均有实现与证据才关闭路线；否则仅记录具体缺口并继续修复。

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

每个 residual milestone 先跑直接相关复现/focused tests；完整门禁只在闭合行为后运行，避免重复验证。`release-check.ready`、文档勾选、字段存在、cross-compile 或 synthetic fixture 都不能单独证明产品完成。本路线的实现/行为验收已满足原 P0～P3逐项证据与真实用户路径；fresh machine validation、direct commit 与本地 tracking ref仍由Git-local typed receipt独立证明。

## 风险与注意事项

- 不把“某次 local validation green”再次提升为“全部产品方案完成”；route completion与machine validation是两条独立真值。
- 不把 residual work扩张为 GUI、installer、新 pack、通用代理或落地机出口；这些不是当前路线的隐含内容。如需新增，必须作为独立用户目标与验收范围处理。
- 不重写 durable state、gate、receipt、generation 或权限模型；优先复用现有 owners，删除平行实现而非再加一层。
- `gate -Apply`仍只记录 decision/observation；actual heavy action只在 strict profile + fresh `authorized-gate` 内执行。
- 不把真实样本、trace、dump、capture、artifact、绝对 case 路径、payload、flag、客户信息或 case-specific 进度写入模板仓库。
- 不声称未读取的 remote CI green、非 Windows runtime E2E、真实跨机器 Remote Control E2E 或 formal release。

## 路线变更记录

- 2026-08-28：完整 P0～P3总验证完成；原未取消项逐项复核，focused/affected/full tests、vet/module/skill、public gates、façade/diff、fresh installed project-local real-Claude binary-re/web-security路径和最终定向反证均通过。路线切换为`completed / no-next`；不创建Batch 834，不声称remote CI green，machine receipt/commit/post-push继续独立发布。
- 2026-08-28：binary-re专项验收校准完成；established status exact specialties、逐pack executable owner、typed catalog、VMP existing-index能力上限与synthetic/real/未观察证据层已通过review和fresh affected-package validation；active card切换到完整P0～P3验证。
- 2026-08-28：retired pack identity migration完成；两个retired identities的review-first transaction、canonical root/onboarding projection、exact replay provenance、copy/move与capability fail-closed已通过审查和focused/package/document validation；active card切换到binary-re专项验收校准，完整P0～P3验证仍未完成。
- 2026-08-25：`P0P3-C4` 完成；scoped owner inventory、continue lock-after-reparse identity guard、gate orphaned report-binding rejection 与未复现候选边界见 `CHANGELOG.md`。
- 2026-08-25：`P0P3-C3` 完成并切换到 C4；exact skill provenance、immutable stamped publication与审计结论见 `CHANGELOG.md`。
- 2026-08-25：`P0P3-C1/C2` 完成；capability/control lineage 与 orchestration non-authorization 细节见 `CHANGELOG.md`。
- 2026-08-24：原始 P0～P3批准与 task tool ledger 复核发现 #37/#38/#50/#52 明确 pending 且无取消记录；撤销 `completed / no-next` 假收口，保持同一路线并恢复 `P0P3-C1`～`C4` residual sequence。
- 2026-08-24：validation repair以 `870d908` direct implementation commit推送并通过post-push receipt；该结果只关闭validation truth，不关闭 residual product work。
- 2026-08-20～23：P0～P3 大部分 architecture/public UX/production admission slices落地；shared capability contract与command owner inventory完成，四项 dependent closure被保留为 pending。
- 2026-08-19：Batch 833完成ordinary `binary-re` actual adapter lifecycle、independent evidence review、accepted-review binding与terminal durable-tail recovery。
- 2026-08-17：用户批准调整后的四阶段路线；2026-08-20 又明确批准更大的 P0～P3 优化方案及架构收敛顺序。
