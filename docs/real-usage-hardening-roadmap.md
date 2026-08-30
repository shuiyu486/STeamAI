# STeamAI 架构与产品收敛路线

## 读取指南

本文件是当前已批准路线的唯一 active source。新会话先由 `docs/context-routing.md` 路由到这里，只读取顶部执行区与当前批次卡；`docs/batch-plan.md` 只是短投影，不是第二选题源。上一条 `steamai-product-optimization-v1` 已完成，其实现与验证事实按需从 `CHANGELOG.md`、`docs/batch-history.md` 和 Git history 追溯，不重新成为当前工作。

## 实施摘要

当前路线是 `steamai-architecture-product-convergence-v1`，由用户于 2026-08-29 明确批准。目标不是继续增加字段、summary、inventory 或 projection，而是按证据修复正确性断点，消除重复 command/projection owner，补齐普通用户任务生命周期与恢复体验，并以真实产品路径证明改造有效。

路线采用四个锁定阶段：`APC-01` 正确性与验证可信度 → `APC-02` typed command / projection 单一 owner → `APC-03` mission 生命周期与公开 UX → `APC-04` 真实产品验收与架构收口。每阶段必须形成可运行闭环或删除旧 owner；不得用拆文件、增加 DTO、补文案或新增平行兼容层冒充架构收敛。

产品获取方式继续保持 `source-clone-first`；不实现 installer，不新增 GUI/TUI 或 PowerShell runtime logic，并明确拒绝 PATH/外部 kit fallback。默认 quickstart 只保留 `cd <project> → claude → /steamai`；本路线也不改写 authority/confirmed、strict gate、generation、receipt 与 execution-control 安全边界。

### 当前指针

| 字段 | 当前值 |
|---|---|
| 路线 | `steamai-architecture-product-convergence-v1` |
| 当前批次 | `APC-04 真实产品验收与架构收口` |
| 状态 | `completed` |
| 唯一允许领取 | `none` |
| 上一批 | `APC-03` 已完成 |
| 下一批 | `none`；等待新的明确批准路线 |
| canonical repository | `https://github.com/shuiyu486/STeamAI` |
| 暂保留兼容身份 | Go module `github.com/shuiyu486/re-context-kits`、内部 `rekit` names、legacy `/rekit` / `.rekit` |

## 执行清单

- [x] **APC-01 — 正确性与验证可信度**：已修复 standalone host/unified runtime image role 与 stable-byte publication、`continue` partial-prefix recovery 和 semantic dedup、Git-local receipt/Git fixture隔离；真实 subprocess、fault-injection、focused/affected/full canonical validation均通过。
- [x] **APC-02 — typed command / projection 单一 owner**：typed invocation 已成为 executable selector/carrier/rendering 的事实源；Overview 与 direct step/current-loop diagnostics 统一先做 detached typed projection，current/legacy × text/table/JSON 隔离通过；shadow command catalog/parser 和绕过 projector 的 direct response 已删除。
- [x] **APC-03 — mission 生命周期与公开 UX**：`details-required`、review-first continue、missing-board recovery、mature pack/source-clone guidance与 completed mission successor generation 已形成单一公开闭环；successor exact recovery、project identity/commit chain与 status shared read fence 已通过对抗性和产品回归。
- [x] **APC-04 — 真实产品验收与架构收口**：Windows fresh canonical/vet/diff/contract/public gates、自包含/复制/篡改/双根、`binary-re`/`web-security`、successor generation与两条显式 real-Claude 产品路径均有直接运行证据；独立终审无 Important/Blocker。

### 锁定顺序

| 阶段 | 解锁条件 |
|---|---|
| `APC-01` | 当前唯一允许实施 |
| `APC-02` | `APC-01` focused/fault-injection/affected/full validation 全部通过 |
| `APC-03` | `APC-02` 删除旧 owner 且 current/legacy × text/table/JSON 一致性通过 |
| `APC-04` | `APC-03` 的普通用户闭环通过临时项目产品验收 |
| 路线总体完成 | 四阶段全部完成；fresh machine validation、direct commit 与本地 tracking ref独立证明 publication truth |

## 当前批次卡

### APC-04：真实产品验收与架构收口

**状态**：completed；无当前 owner，不自动创建下一路线。

**收口结论**：

1. APC-01～APC-03 已整体通过 fresh canonical/public gates；actor-bound exact `applyArgs` 最后修复后再次全仓通过。
2. Windows、source-clone/self-contained、copy/tamper/dual-root、mature pack和显式real-Claude路径均有直接证据；synthetic、cross-compile、workflow definition与`release-check.ready`未被提升。
3. typed invocation与mission namespace各有单一production owner，status完整DTO受同一shared lease保护。production Go规模仍高，尤其`cli`、`workstream`、`sessionhost`和`releasecheck`；没有为减行数合并不同安全状态机。独立复审无Important/Blocker。
4. 本路线不声称remote CI green、非Windows runtime E2E或formal release，也不自动合并`main`。

## 路线级验证标准

- 每阶段必须修复一个可复现缺陷、删除一个重复 owner、走通一个真实用户闭环，或产生可量化的复杂度净减；只增加字段/summary/inventory不算完成。
- current `.steamai` live command carrier只显示 `/steamai`；legacy `.rekit`保留 `/rekit`。不得全局替换 prose 或 durable/source identity。
- 普通用户不填写 SHA、generation、session ID、内部路径或 maintenance flags；需要 exact binding时由主 Agent消费 typed carrier。
- 架构收口不是把 `cli.go` 拆文件：`cli` 包与重复 projection/parser 必须有实际净减，旧 owner必须删除。
- `release-check.ready`、文档勾选、cross-compile、synthetic fixture或一次测试通过都不能单独证明路线完成。

## 风险与注意事项

- 保留真正不同的安全状态机：workstream durable mission、external session provenance、execution control generation、sessionhost process lifecycle不得为减少行数而合并。
- authority/confirmed、heavy action、sync/promote、schema migration与公共 façade 删除仍遵守既有升级门禁。
- 不写真实样本、trace/dump/capture、artifact、绝对 case路径、payload、客户信息或 case-specific进度。
- 不声称未实际运行的 remote CI green、非 Windows runtime E2E、真实跨机器 Remote Control E2E或 formal release。

## 路线变更记录

- 2026-08-31：`APC-04` 与四阶段路线完成。Windows fresh canonical suite、vet、diff/skill/default-doc合同和public release commands通过；source-clone/self-contained、copy/tamper/dual-root、`binary-re`/`web-security`、generation 1→2→3 successor及中断恢复有直接运行证据，两条显式 installed real-Claude E2E实际通过。终审发现并关闭non-default actor未进入successor exact `applyArgs`的最后断点，compiled unified host原样执行回归及修复后全仓门禁通过；独立复核无Important/Blocker。未运行或声称remote CI green、非Windows runtime E2E或formal release。
- 2026-08-30：`APC-03` 完成。完成后的不同新目标现走 review-first successor generation；active pointer/manifest/generation commit/transition intent/project identity形成 fail-closed chain，任意 durable prefix可 exact recovery，pack authority projection纳入 reviewed plan。current status聚合持有 shared project lease，sessionhost复用既有 lease避免Windows非重入。focused/affected、CLI生命周期夹具、vet、skill/default-doc合同与文档预算通过；进入`APC-04`最终验收。
- 2026-08-30：`APC-02` 完成。executable typed invocation、selector与command rendering owner完成收敛；Overview和direct current-loop/step/external diagnostics统一detached projection，current/legacy × text/table/JSON及深层typed request矩阵通过；shadow catalog/parser与旧direct response owner已删除。focused、full CLI、canonical full suite、vet、module、skill-contract及public release commands通过；独立只读审计无correctness finding。唯一当前owner推进到`APC-03`。
- 2026-08-29：`APC-01` 完成。unified runtime stable-byte role/publication、onboarding recovery、adapter formal receipt、`continue` component recovery + semantic dedup、hermetic Git/receipt fixtures及Windows executable cleanup均已闭合；四轮独立复验无finding，focused/affected/full canonical gates通过。唯一当前owner推进到`APC-02`。
- 2026-08-29：用户明确批准完整四阶段架构与产品收敛实施；建立 `steamai-architecture-product-convergence-v1`，以 `APC-01` 为唯一初始 owner。路线基于 fresh code audit、临时项目实测与 full Review Panel：保留有效安全状态机，停止字段/projection微批次，按正确性 → owner收敛 → 产品 UX → 真实验收推进。
- 2026-08-28：上一条 `steamai-product-optimization-v1` 完成 P0～P3 residual closure并切换为 completed/no-next；其事实保留在历史、CHANGELOG 与 Git-local receipts中，不再拥有当前选题。
