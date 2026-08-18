# STeamAI 产品优化路线

## 读取指南

本文件是当前已批准路线的唯一 active source，只展开当前批次卡。先由 `docs/context-routing.md` 路由到本文件；恢复工作时只读顶部执行区和当前卡，不预读后续阶段的实现细节。已完成批次按 ID 查询 `docs/batch-history.md`，专题合同仍按需进入。

## 实施摘要

当前路线是 `steamai-product-optimization-v1`。路线固定为四个有序阶段：先关闭真实产品闭环，再把 `vmp-re` 与 `generic-binary-re` 收敛为唯一 `binary-re`，随后实现 durable pause/resume/stop，最后把 VMP/IDA actual adapter 作为 `binary-re` capability 接入普通真实分析闭环。Batch 830 与 Batch 831 已完成，Batch 832 是唯一允许领取的下一批；后一阶段继续等待前一阶段 fresh 验证完成。

产品获取方式保持 `source-clone-first`：canonical repository 是 `https://github.com/shuiyu486/STeamAI`，Go module `github.com/shuiyu486/re-context-kits` 暂作 compatibility identity。Go 与 Claude Code 是预期依赖；不实现 installer、portable ZIP、MSI、winget、下载站或自动更新。current 项目继续拒绝 PATH/外部 kit fallback，默认 quickstart 只保留 `cd <project> → claude → /steamai`。

### 当前指针

| 字段 | 当前值 |
|---|---|
| 路线 | `steamai-product-optimization-v1` |
| 当前批次 | `Batch 831` binary-re convergence |
| 状态 | `completed` |
| 唯一允许领取 | `Batch 832` |
| 上一批 | `Batch 830` core product closure 已完成并归档 |
| 下一批 | `Batch 832` |
| canonical repository | `https://github.com/shuiyu486/STeamAI` |
| 暂保留兼容身份 | Go module `github.com/shuiyu486/re-context-kits`、内部 `rekit` names、legacy `/rekit` / `.rekit` |

## 执行清单

- [x] **Batch 830 — core product closure（completed）**：已关闭 Reviewer strict intake、ordinary-directory typed adoption、project-local promote、current `/steamai` public projection、release truth、actual adapter root binding 与可重定位 Identity v2，并完成受影响包、临时项目和真实 Claude 边界验证。
- [x] **Batch 831 — binary-re convergence（completed）**：以成熟 `vmp-re` 为能力主体，吸收 generic recipes/task types/deny patterns，发布唯一 active `packs/binary-re`；不保留 alias、双写或自动迁移，旧身份只返回 typed `pack-migration-required`。
- [ ] **Batch 832 — durable execution control（next）**：实现独立 control generation、append-only pause/resume/stop receipts、durable-first stop、late-result held ledger 与进程 actuation 分层；不把 control 当授权。
- [ ] **Batch 833 — binary-re actual analysis（locked）**：把 VMP/IDA adapter 作为 `binary-re` capability 接入 ordinary daily、独立 evidence review 和 accepted-review 后绑定；用 harmless synthetic input 完成真实 E2E 与 terminal replay。
- [ ] **路线收口**：每批完成本批 fresh gate、commit/push 与 post-push Git-local inspection并直接继续下一批；四阶段全部完成后再执行路线级完整 fresh tests、vet、module verify、release-check/status/packs/doctor、移动/复制 E2E、真实 Claude acceptance与临时计划/评估文档清理。

## 当前批次卡

### Batch 831：binary-re convergence

**状态**：completed；Batch 832 已解锁为唯一允许领取的下一批。

**目标**：把两个重叠二进制逆向 pack 收敛为唯一 active `binary-re`，保留成熟 VMP/IDA 能力并吸收通用 static triage、function/API behavior review、Agent Team routes 与 recipes；同时固定 source-clone-first 获取方式和 current project-local verified runtime 边界。

**完成结果**：

- 新增集中 `packidentity` policy，默认 pack 改为 `binary-re`；显式参数、attached metadata 与 embedded runtime bundle 中的 `vmp-re` / `generic-binary-re` 在写入或输出前返回 typed `pack-migration-required`，未知 pack 保持普通 missing/unknown 语义；
- active inventory 只保留 `packs/binary-re`，成熟 VMP/IDA references、policies、tooling 与通用 static/function behavior recipes 合并到同一 manifest；旧 identity 不保留 alias、不双写、不自动迁移；
- onboarding、manifest、instance、runtime、runtime bundle、host 与 adapter 入口统一执行 pack identity policy；`binary-analysis` lane label 往返和 `.steamai` / `.rekit` typed read-only path projection按 selected state root工作；
- README、schema、façade默认值、smoke catalog与release说明固定 canonical clone URL、Go + Claude Code预期依赖和唯一 mature `binary-re`，current项目仍只运行project-local verified runtime bundle；
- 受影响 Go packages fresh tests、catalog/inventory/discovery smoke、compatibility façade smoke与current `binary-re` hash-bound init→lane→review packet→ledger/gate→handoff dry-run均已通过。

**完成边界**：

- Batch 831 未提前注册 pause/resume/stop control surface，也未增加 Batch 833 ordinary VMP/IDA actual adapter执行能力；
- 未机械改名 Go module/internal `rekit`、legacy `/rekit` / `.rekit` 或历史事实，trusted recovery generations保持append-only；
- 未新增 PowerShell runtime logic、installer、PATH fallback、authority/confirmed或未授权heavy action；
- 本地focused gate与Git-local inspection不等于remote CI green或formal release；Batch 831在Batch 832真正开始时再由canonical rotation归档。

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

- 2026-08-18：Batch 830归档；Batch 831完成唯一active `binary-re`、retired identity typed migration与source-clone-first收敛，解锁Batch 832 durable execution control。
- 2026-08-17：基于真实源码、fresh tests、临时项目、真实 Claude member/Reviewer 与 actual adapter 复评，批准 `steamai-product-optimization-v1` 四阶段路线；明确 source-clone-first、无 installer、唯一 `binary-re`。
- 2026-08-17：`steamai-repository-identity-v1` / Batch 829 完成后归档；canonical repository 保持 `shuiyu486/STeamAI`，Go/internal/legacy identity 继续兼容。
