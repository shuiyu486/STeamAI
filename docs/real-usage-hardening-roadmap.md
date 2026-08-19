# STeamAI 产品优化路线

## 读取指南

本文件是当前已批准路线的唯一 active source，只展开最近完成批次卡。先由 `docs/context-routing.md` 路由到本文件；恢复工作时只读顶部执行区和当前卡，不预读历史批次实现。已完成的更早批次按 ID 查询 `docs/batch-history.md`，专题合同仍按需进入。

## 实施摘要

当前路线是 `steamai-product-optimization-v1`。四个有序实施阶段 Batch 830～833 已完成：core product closure、唯一 `binary-re`、durable pause/resume/stop，以及 ordinary `binary-re` actual adapter lifecycle。当前唯一工作是路线级完整验证与清理；在它通过前，不把 focused tests、typed contract、Markdown claim 或本地实现完成冒充整条路线验收完成。

产品获取方式保持 `source-clone-first`：canonical repository 是 `https://github.com/shuiyu486/STeamAI`，Go module `github.com/shuiyu486/re-context-kits` 暂作 compatibility identity。Go 与 Claude Code 是预期依赖；不实现 installer、portable ZIP、MSI、winget、下载站或自动更新。current 项目继续拒绝 PATH/外部 kit fallback，默认 quickstart 只保留 `cd <project> → claude → /steamai`。

### 当前指针

| 字段 | 当前值 |
|---|---|
| 路线 | `steamai-product-optimization-v1` |
| 当前批次 | `路线收口` full validation and cleanup |
| 状态 | `in_progress` |
| 唯一允许领取 | `路线收口` |
| 上一批 | `Batch 833` binary-re actual analysis 已完成 |
| 下一批 | `无（路线完成后等待明确新路线）` |
| canonical repository | `https://github.com/shuiyu486/STeamAI` |
| 暂保留兼容身份 | Go module `github.com/shuiyu486/re-context-kits`、内部 `rekit` names、legacy `/rekit` / `.rekit` |

## 执行清单

- [x] **Batch 830 — core product closure（completed）**：关闭 Reviewer strict intake、ordinary-directory typed adoption、project-local promote、current `/steamai` public projection、release truth、actual adapter root binding 与可重定位 Identity v2。
- [x] **Batch 831 — binary-re convergence（completed）**：发布唯一 active `packs/binary-re`；不保留 alias、双写或自动迁移，旧 pack identity 只返回 typed `pack-migration-required`。
- [x] **Batch 832 — durable execution control（completed）**：实现独立 control generation、append-only pause/resume/stop receipt、result birth/currentness binding、held/late ledger、consumer progression guard，以及 durable-first exact local supervisor stop actuation。
- [x] **Batch 833 — binary-re actual analysis（completed）**：VMP/IDA actual adapter 接入 ordinary daily、独立 evidence review 和 accepted-review 后 task binding；terminal durable tail 可恢复且不重启 child、Reviewer 或重复 acknowledgement。
- [ ] **路线收口（current）**：执行完整 fresh tests、vet、module verify、release-check/status/packs/doctor、移动/复制 E2E、真实 Claude acceptance、README/CLAUDE/reference/config/example 核查与临时计划清理，再完成最终 commit/push 和 Git-local inspection。

## 当前批次卡

### Batch 833：binary-re actual analysis

**状态**：completed；路线级完整验证与清理已成为唯一 current 工作。

**目标**：把现有 VMP/IDA actual adapter 下沉为 ordinary self-contained `binary-re` capability，复用 strict gate → dispatch → actual execution → report/receipt → independent evidence review → accepted acknowledgement/task-context binding → member/Reviewer progression；不能复制 live-acceptance 特例、创建第二套 adapter 或新增分析引擎。

**完成结果**：

- ordinary goal、resume 与 correction 在 exact lane start/reconcile 后、real member 前进入同一 `runBinaryREAdapterLifecycle`；没有 matching authorized gate 时保持普通 member 路径，有 gate但 lifecycle 未 ready 时阻止 member越过证据闭环；
- project-local verified executable 通过同一 private dispatcher承载 authorized parent与fixed no-network-codepath child，`cmd/rekit`、`cmd/rekit-host`和legacy adapter-host复用该入口；actual child前在lane mutation lease内重验exact execution-control binding；
- actual run前先durable写入content-addressed execution intent，terminal result再固化为exact replay artifact；successful adapter binding延迟到independent evidence review accepted之后，failed/aborted或deferred terminal replay不会补写旧binding；
- evidence review严格绑定request、selected source row、packet、report、dispatch、receipt、observation、raw Claude result和control birth；accepted acknowledgement走public `note` preview/exact Apply，execution-control binding只保留在bounded record route中，不进入ledger event或改变event SHA；
- decision、closure和historical owner-generation task binding支持acknowledged crash-tail恢复：缺closure或binding时只补durable tail，不重启adapter、child或Reviewer，不重写receipt、不重复acknowledgement；完整terminal lifecycle discovery零写入跳过；
- live acceptance不再维护平行adapter/review/ack实现，只准备harmless synthetic IDA index与strict gate并投影ordinary lifecycle；输出根由fresh selected lane workspace派生，不硬编码旧lane identity。

**完成证据**：

- focused fresh lifecycle test实际启动embedded authorized parent与fixed child，覆盖execution-intent-before-run、deferred binding、strict selected-row lineage、published decision、terminal exact replay／different-bytes drift、缺closure、缺binding、完成态tree hash不变和paused旧control result拒绝；
- adapterhost、memberexecution、note、sessionhost、CLI binding与三个Go executable入口的直接相关fresh tests通过，`git diff --check`通过；
- focused evidence Reviewer使用deterministic strict hook以避免本批重复启动真实Claude，但仍经过durable Claude recovery、raw-result identity与execution-control publication；因此真实Claude member→Reviewer→completion acceptance仍明确留在路线收口，不被本卡伪称已验证。

**完成边界**：

- `gate -Apply`仍只写decision/observation，actual heavy action只在strict durable profile + fresh `authorized-gate`内执行；request SHA、transport、endpoint、receipt或control binding都不授予authority/confirmed；
- fixed child的`NoNetwork`只说明代码路径不包含网络功能，不声称OS级socket隔离；没有安装/启动IDA、执行catalog entry、增加自动脱壳/分析引擎或写入真实样本、trace、dump、capture、payload、客户信息；
- ordinary terminal replay不启动adapter、不重复evidence Reviewer或acknowledgement；stale/paused/stopped control和owner drift继续fail-closed。

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

路线收口还必须执行移动/复制 E2E、真实 Claude acceptance 和 harmless synthetic actual adapter E2E，并检查 Git-local machine receipt/tracking ref。workflow definition、本地 receipt、cross-compile、focused hook 或 Markdown claim 都不能提升为 remote CI green、跨平台 runtime E2E 或 formal release。

## 风险与注意事项

- 四个实现阶段完成不等于路线验收完成；当前只允许路线收口，不自行生成 Batch 834 或选择新产品功能。
- `binary-re` convergence 是 identity cutover，不做 alias 或机械全文替换；历史批次和旧 recovery generation 保留当时事实。
- durable control 与 executor generation、gate authorization分离；process termination不是durable stop成功判据，actuation failure不回滚stopped。
- `.steamai` / `.rekit` 继续 dual-read/single-write，both fail-closed；current 项目不得回退中央 source runtime。
- `rekit/rekit.ps1` 仅保留 legacy compatibility façade；公共 façade 删除仍受独立门禁约束。

## 路线变更记录

- 2026-08-19：Batch 833完成ordinary `binary-re` actual adapter lifecycle、independent evidence review、accepted-review binding与terminal durable-tail recovery；解锁路线级完整验证与清理，真实Claude acceptance仍待该阶段执行。
- 2026-08-19：Batch 832完成durable pause/resume/stop、held/late result isolation、consumer progression guard与exact local supervisor durable-first stop actuation，解锁Batch 833。
- 2026-08-18：Batch 831完成唯一active `binary-re`、retired identity typed migration与source-clone-first收敛，解锁Batch 832。
- 2026-08-17：基于真实源码、fresh tests、临时项目、真实 Claude member/Reviewer 与 actual adapter 复评，批准 `steamai-product-optimization-v1` 四阶段路线；明确 source-clone-first、无 installer、唯一 `binary-re`。
- 2026-08-17：`steamai-repository-identity-v1` / Batch 829 完成后归档；canonical repository 保持 `shuiyu486/STeamAI`，Go/internal/legacy identity 继续兼容。
