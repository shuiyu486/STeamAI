# STeamAI 产品优化路线

## 读取指南

本文件是当前已批准路线的唯一 active source，只展开当前批次卡。先由 `docs/context-routing.md` 路由到本文件；恢复工作时只读顶部执行区和当前卡，不预读后续阶段的实现细节。已完成批次按 ID 查询 `docs/batch-history.md`，专题合同仍按需进入。

## 实施摘要

当前路线是 `steamai-product-optimization-v1`。路线固定为四个有序阶段：先关闭真实产品闭环，再把 `vmp-re` 与 `generic-binary-re` 收敛为唯一 `binary-re`，随后实现 durable pause/resume/stop，最后把 VMP/IDA actual adapter 作为 `binary-re` capability 接入普通真实分析闭环。Batch 830 已完成，Batch 831 是唯一允许领取的下一批；后一阶段继续等待前一阶段 fresh 验证完成。

产品获取方式保持 `source-clone-first`：canonical repository 是 `https://github.com/shuiyu486/STeamAI`，Go module `github.com/shuiyu486/re-context-kits` 暂作 compatibility identity。Go 与 Claude Code 是预期依赖；不实现 installer、portable ZIP、MSI、winget、下载站或自动更新。current 项目继续拒绝 PATH/外部 kit fallback，默认 quickstart 只保留 `cd <project> → claude → /steamai`。

### 当前指针

| 字段 | 当前值 |
|---|---|
| 路线 | `steamai-product-optimization-v1` |
| 当前批次 | `Batch 830` core product closure |
| 状态 | `completed` |
| 唯一允许领取 | `Batch 831` |
| 上一批 | `Batch 829` GitHub repository identity migration 已完成并归档 |
| 下一批 | `Batch 831` |
| canonical repository | `https://github.com/shuiyu486/STeamAI` |
| 暂保留兼容身份 | Go module `github.com/shuiyu486/re-context-kits`、内部 `rekit` names、legacy `/rekit` / `.rekit` |

## 执行清单

- [x] **Batch 830 — core product closure（completed）**：已关闭 Reviewer strict intake、ordinary-directory typed adoption、project-local promote、current `/steamai` public projection、release truth、actual adapter root binding 与可重定位 Identity v2，并完成受影响包、临时项目和真实 Claude 边界验证。
- [ ] **Batch 831 — binary-re convergence（next）**：以成熟 `vmp-re` 为能力主体，吸收 generic recipes/task types/deny patterns，发布唯一 active `packs/binary-re`；不保留 alias、双写或自动迁移，旧身份只返回 typed migration-required。
- [ ] **Batch 832 — durable execution control（locked）**：实现独立 control generation、append-only pause/resume/stop receipts、durable-first stop、late-result held ledger 与进程 actuation 分层；不把 control 当授权。
- [ ] **Batch 833 — binary-re actual analysis（locked）**：把 VMP/IDA adapter 作为 `binary-re` capability 接入 ordinary daily、独立 evidence review 和 accepted-review 后绑定；用 harmless synthetic input 完成真实 E2E 与 terminal replay。
- [ ] **路线收口**：每批完成本批 fresh gate、commit/push 与 post-push Git-local inspection并直接继续下一批；四阶段全部完成后再执行路线级完整 fresh tests、vet、module verify、release-check/status/packs/doctor、移动/复制 E2E、真实 Claude acceptance与临时计划/评估文档清理。

## 当前批次卡

### Batch 830：core product closure

**状态**：completed；Batch 831 已解锁为唯一允许领取的下一批。

**目标**：让自包含项目的 identity、复制/移动恢复、project-local promote、public projection、ordinary-directory adoption、release truth 与真实 Claude 日常路径在真实调用链上闭合，而不是只在合同或单点测试中成立。

**完成结果**：

- current Identity v2 发布 stable project ID、logical `target="."` 与 exact project binding；project-local runtime 在 central source 删除后可复制、整体移动、exact replay并继续运行，旧 current v1 移动后明确 fail-closed；
- project-local promote 以 attached project 的 verified delivery root 为 owner；decision/proof 写入固定在 active state root 的 `reviews/**`，pack-tree direct/physical alias、symlink 与 Windows junction 在第一笔副作用前拒绝；
- current `/steamai` 与 legacy `/rekit` 只投影显式 typed structure，malformed slash command fail-closed，普通 prose 与 durable/source identity 不改写；
- Reviewer strict namespace、ordinary-directory hash-bound adoption、release truth 与 actual adapter project-local root binding 同批闭合；
- 真实 Claude gate 一次完成 member→独立 Reviewer reject→人工 correction→replacement member→独立 Reviewer accept→completion；重复 gate 的 generation 2 Reviewer 语义拒绝保持等待纠偏并正确清理，没有用重试伪造完成；
- Identity/onboarding/self-contained relocation、promote no-reparse、public projection 与 CLI 产品链 focused/fresh tests 已通过；完成态 `release-check -Format json` inventory 返回 `ready=true`。最终本机 release minimum 只以冻结工作树上的 fresh machine receipt 为准。

**完成边界**：

- Batch 830 未提前修改 pack identity、注册 pause/resume/stop 或增加 Batch 833 普通 binary adapter surface；
- 未新增 PowerShell runtime logic、installer、PATH fallback、authority/confirmed 或未授权 heavy action；
- 本地 machine receipt、workflow definition 与 tracking ref 不等于 remote CI green 或 formal release；
- Batch 830 在 Batch 831 真正开始时再由 canonical rotation 归档，不在完成提交中提前复制到 `docs/batch-history.md`。

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

每批先跑对应 focused tests，再在冻结工作树执行有增量价值的完整门禁。workflow definition、本地 receipt、tracking ref 与 Markdown claim 都不能提升为 remote CI green 或 formal release。

## 风险与注意事项

- 四个阶段已批准不等于可以并发混写；只有当前批次可修改产品，后续蓝图仅作解锁后的输入。
- `binary-re` convergence 是 identity cutover，不做 alias 或机械全文替换；历史批次和旧 recovery generation 保留当时事实。
- durable control 与 executor generation、gate authorization 分离；process termination 不是 durable stop 成功判据。
- `.steamai` / `.rekit` 继续 dual-read/single-write，both fail-closed；current 项目不得回退中央 source runtime。
- `rekit/rekit.ps1` 仅保留 legacy compatibility façade；本路线不新增 PowerShell runtime logic，公共 façade 删除仍受独立门禁约束。

## 路线变更记录

- 2026-08-17：基于真实源码、fresh tests、临时项目、真实 Claude member/Reviewer 与 actual adapter 复评，批准 `steamai-product-optimization-v1` 四阶段路线；明确 source-clone-first、无 installer、唯一 `binary-re`。
- 2026-08-17：`steamai-repository-identity-v1` / Batch 829 完成后归档；canonical repository 保持 `shuiyu486/STeamAI`，Go/internal/legacy identity 继续兼容。
