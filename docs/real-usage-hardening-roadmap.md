# STeamAI 产品优化路线

## 读取指南

本文件是当前已批准路线的唯一 active source，只展开最近完成批次卡。先由 `docs/context-routing.md` 路由到本文件；恢复工作时只读顶部执行区和当前卡，不预读历史批次实现。已完成的更早批次按 ID 查询 `docs/batch-history.md`，专题合同仍按需进入。

## 实施摘要

当前路线是 `steamai-product-optimization-v1`，且已完成。四个有序实施阶段 Batch 830～833 与路线级真实验收均已收口：core product closure、唯一 `binary-re`、durable pause/resume/stop、ordinary `binary-re` actual adapter lifecycle，以及组合行为的真实 Claude/actual adapter/recovery 验证。当前没有已批准的下一路线。

产品获取方式保持 `source-clone-first`：canonical repository 是 `https://github.com/shuiyu486/STeamAI`，Go module `github.com/shuiyu486/re-context-kits` 暂作 compatibility identity。Go 与 Claude Code 是预期依赖；不实现 installer、portable ZIP、MSI、winget、下载站或自动更新。current 项目继续拒绝 PATH/外部 kit fallback，默认 quickstart 只保留 `cd <project> → claude → /steamai`。

### 当前指针

| 字段 | 当前值 |
|---|---|
| 路线 | `steamai-product-optimization-v1` |
| 当前批次 | `路线收口` full validation and cleanup |
| 状态 | `completed` |
| 唯一允许领取 | `无` |
| 上一批 | `Batch 833` binary-re actual analysis 已完成 |
| 下一批 | `无（等待明确新路线）` |
| canonical repository | `https://github.com/shuiyu486/STeamAI` |
| 暂保留兼容身份 | Go module `github.com/shuiyu486/re-context-kits`、内部 `rekit` names、legacy `/rekit` / `.rekit` |

## 执行清单

- [x] **Batch 830 — core product closure（completed）**：关闭 Reviewer strict intake、ordinary-directory typed adoption、project-local promote、current `/steamai` public projection、release truth、actual adapter root binding 与可重定位 Identity v2。
- [x] **Batch 831 — binary-re convergence（completed）**：发布唯一 active `packs/binary-re`；不保留 alias、双写或自动迁移，旧 pack identity 只返回 typed `pack-migration-required`。
- [x] **Batch 832 — durable execution control（completed）**：实现独立 control generation、append-only pause/resume/stop receipt、result birth/currentness binding、held/late ledger、consumer progression guard，以及 durable-first exact local supervisor stop actuation。
- [x] **Batch 833 — binary-re actual analysis（completed）**：VMP/IDA actual adapter 接入 ordinary daily、独立 evidence review 和 accepted-review 后 task binding；terminal durable tail 可恢复且不重启 child、Reviewer 或重复 acknowledgement。
- [x] **路线收口（completed）**：默认 `binary-re` 真实 Claude acceptance、actual adapter、terminal/attached recovery、README/CLAUDE/reference/config/example 核查与临时计划清理均已闭合；frozen 7-step local minimum、exact implementation commit 与 tracking ref 由 Git-local v2 receipt/post-push inspection 动态验证。

## 当前批次卡

### 路线收口：full validation and cleanup

**状态**：completed；当前没有已批准下一批，不创建 Batch 834。

**目标**：以 frozen-tree fresh tests、本机 release receipt、自包含复制/移动 E2E、一次默认 `binary-re` 真实 Claude acceptance、actual adapter、文档/示例核查和 Git-local post-push inspection 验证 Batch 830～833 的组合行为；不能用 focused hook、typed contract、Markdown claim、cross-compile 或远程 workflow 定义代替真实完成证据。

**完成结果**：

- preliminary full suite 暴露的 recovery canonical bytes、默认 lane fixture、paused Reviewer diagnostic package、external Reviewer exact owner/lane、Remote Control return binding/`observedAt` 与 cross-route checkpoint lane/hash 漂移均已修复；strict production guards 未放宽；
- member output contract 现在要求 submission-root-relative path，并只对 exact current attempt 的三个 host-owned root 做确定性归一化；任意其它 `.steamai`/`.rekit` path 继续 fail-closed，Reviewer evidence validation 未放宽；
- 默认 `binary-re` 真实 gate 以 fresh case 走通 2 代 member、2 名独立 Reviewer、首次 rejection、zero-launch rejected replay、人工 correction、actual embedded parent/fixed child、independent evidence review、replacement acceptance、completion 与 terminal no-child/no-Claude replay；attached case另走通 member/Reviewer cutpoint 与 completion recovery。总计 3 个 member、3 个 Reviewer，`manualPlaceholders=0`、`manualResultWrites=0`，fresh/attached cleanup 均为 `removed`；
- actual adapter 证据精确绑定 selected row `0x1000\tneedle_dispatch\t32` 与 `ida-index:functions:tooling/ida-agent-bridge/export/function_index.tsv#L2`；accepted review 后才发布 task binding，profile 已 revoke，未写 authority/confirmed；
- `CLAUDE.md` 保持 8 KiB 内；README、Agent Team usage、reference、release readiness 与 binary-re recipe 已收敛到 source-clone-first、唯一 active `binary-re` 和 ordinary lifecycle；复制/移动/no-central-kit E2E由 canonical full suite执行。

**完成边界**：

- 只处理完整验证暴露的真实缺口，不扩大为新功能、installer、动态 plugin registry、第二套 adapter 或新 pack；
- `gate -Apply` 仍只写 decision/observation；actual heavy action 只在 strict durable profile + fresh `authorized-gate` 内执行，request SHA、transport、endpoint、receipt 或 control binding 都不授予 authority/confirmed；
- 不把真实样本、trace、dump、capture、artifact、绝对 case 路径、payload、flag、客户信息或 case-specific 进度写入模板仓库；
- 本地验证不冒充 remote CI green、跨平台 runtime E2E 或 formal release。

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

本卡的 local completion 只由成功 frozen `release-run` 写出的 Git-local v2 machine receipt及其 post-commit/tracking-ref inspection 动态授权；缺失、stale、非 direct commit、artifact drift、非 main 或不同步均 fail-closed。真实 Claude gate 与 harmless synthetic actual adapter E2E 是独立产品证据；workflow definition、本地 receipt、cross-compile、focused hook 或 Markdown claim 都不能提升为 remote CI green、跨平台 runtime E2E 或 formal release。

## 风险与注意事项

- 当前路线已完成但没有自动解锁下一路线；不自行生成 Batch 834、选择新产品功能或消费候选池。
- `binary-re` convergence 是 identity cutover，不做 alias 或机械全文替换；历史批次和旧 recovery generation 保留当时事实。
- durable control 与 executor generation、gate authorization分离；process termination不是durable stop成功判据，actuation failure不回滚stopped。
- `.steamai` / `.rekit` 继续 dual-read/single-write，both fail-closed；current 项目不得回退中央 source runtime。
- `rekit/rekit.ps1` 仅保留 legacy compatibility façade；公共 façade 删除仍受独立门禁约束。

## 路线变更记录

- 2026-08-20：路线收口真实 default `binary-re` gate通过：3个member、3个独立Reviewer、rejection→correction→acceptance→completion、actual parent/child、evidence review、terminal/attached recovery与全部cleanup闭合；文档/fixture/output-path contract 漂移同步修复，路线转为completed且不自动创建Batch 834。
- 2026-08-19：Batch 833完成ordinary `binary-re` actual adapter lifecycle、independent evidence review、accepted-review binding与terminal durable-tail recovery；解锁路线级完整验证与清理。
- 2026-08-19：Batch 832完成durable pause/resume/stop、held/late result isolation、consumer progression guard与exact local supervisor durable-first stop actuation，解锁Batch 833。
- 2026-08-18：Batch 831完成唯一active `binary-re`、retired identity typed migration与source-clone-first收敛，解锁Batch 832。
- 2026-08-17：基于真实源码、fresh tests、临时项目、真实 Claude member/Reviewer 与 actual adapter 复评，批准 `steamai-product-optimization-v1` 四阶段路线；明确 source-clone-first、无 installer、唯一 `binary-re`。
- 2026-08-17：`steamai-repository-identity-v1` / Batch 829 完成后归档；canonical repository 保持 `shuiyu486/STeamAI`，Go/internal/legacy identity 继续兼容。
