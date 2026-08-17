# 真实使用加固与日常产品收口路线图

## 读取指南

本文件是当前已批准路线的唯一 active source，只内联当前批次卡。先由 `docs/context-routing.md` 路由到本文件；本轮涉及 STeamAI 自包含项目时，再按需读取 `docs/steamai-self-contained-project.md` 对应章节和相关 Go symbols。`docs/batch-plan.md` 只做短投影，Batch 827 及更早历史按 ID 查询 `docs/batch-history.md`。

## 实施摘要

`real-usage-hardening-v1`、`daily-product-closure-v1`、`remote-control-reviewer-transport-v1` 与 `windows-mission-control-usability-v1` 均已完成。当前路线是 `steamai-self-contained-project-v1`，也已完成并把默认产品从“中央 kit + case-local `/rekit` thin shim”迁移为“一个真实项目目录 = 一个自包含 STeamAI 项目”：新入口 `/steamai`、current 状态根 `.steamai`、project-local verified runtime/pack；旧 `/rekit` 与 `.rekit` 只保留显式兼容和 review-first 迁移。当前没有已批准下一路线。

本路线同时关闭四个已确认用户断点：默认 status 过大、复杂故障不说人话、授权逐次打断、项目复制后依赖中央 kit。它们必须作为一个产品闭环落地，不能拆成连续 contract/metadata 微批次。远程 CI、Linux/macOS、三平台兼容、独立安装器、GUI/TUI 和仓库/Go module/内部 executable 机械改名不属于当前完成口径。

### 当前指针

| 字段 | 当前值 |
|---|---|
| 路线 | `steamai-self-contained-project-v1` |
| 当前批次 | `Batch 828` STeamAI self-contained project closure |
| 状态 | `completed` |
| 唯一允许领取 | `Batch 828`（已完成的历史绑定；不可继续领取） |
| 上一批 | `Batch 827` maintenance hotspot decomposition and full Windows acceptance 已完成 |
| 下一批 | 无；等待用户明确改变路线 |
| canonical 代码入口 | `internal/rekit/projectstate/**`、`internal/rekit/runtime{,bundle}/**`、`internal/rekit/sync/**`、`internal/rekit/cli/**`、`internal/rekit/sessionhost/**`、`internal/rekit/autonomy/**` |
| 产品合同 | `docs/steamai-self-contained-project.md` |
| 最近结果 | Windows state-root、compact status、人话 recovery、bounded autonomy、runtime bundle、copied-directory/no-central-kit、legacy migration、current-sync process recovery 与 durable supervisor handoff已通过；独立 capability 复核无 surviving blocker，完整 local minimum通过；未授权 commit/push，cadence 为 `implementation-pending` |

## 执行清单

1. 用 shared `projectstate` owner 完成 `.steamai` / `.rekit` dual-read、single-write；empty/path escape/file/reparse/root switch/dual-root 全部 fail-closed，production mutable owner 不再硬编码 current `.rekit`。
2. ordinary init/attach 为新项目发布 `.claude/skills/steamai/SKILL.md`、relocatable `.steamai/instance.yml`、verified project-local runtime bundle、selected pack、common 与必要 assets；统一 `.steamai/packs/<pack>` 布局。
3. runtime resolution 对 current target 优先使用项目内 bundle，拒绝 PATH/外部 kit fallback；复制项目到新路径并移除中央 kit 后，project-local status/packs/doctor/daily 仍可运行。
4. 默认 Mission Commander refresh 使用 4 KiB `compact-json`；完整保留 typed request/choices，超限或 identity invalid 返回小型 blocked envelope，full JSON 只按需诊断。
5. daily action 统一“现在、原因、下一步”，只从 typed failure/mutation boundary 投影 `auto-recovered|retryable|user-decision-required`；不把 STeamAI 描述成 Claude Code 安装/登录管理器。
6. `bounded-autonomous-v1` 只允许显式 managed preset：单 project/lane、manifest exact actions、exact targets、有限 budget、完整 stop、case-relative outputs、最长 15 分钟、record/revoke/evaluate currentness；不宣称无限权限。
7. 新增独立 `.rekit` → `.steamai` review-first migration：zero-write preview、exact plan SHA Apply、durable receipt、safe replay、skill/relocatable metadata 转换；禁止双写、自动合并、自动择优或权限提升。
8. 更新 README、canonical/project skill、产品方向、使用指南、router、CHANGELOG 与历史评估指针；默认 quickstart 只保留 `cd <project> → claude → /steamai`，旧命令参考降级为 compatibility/maintenance。
9. current/legacy/dual-root/copied-directory/compact/recovery/autonomy/migration Windows E2E、独立边界复核与完整 release minimum 均已完成；canonical `release-run` 的步骤、receipt和inspection外部命令统一由有界进程树runner收口，根退出、deadline或64 MiB输出上限均终止 containment 内的剩余子孙，逃逸writer只允许5秒有限pipe drain；在冻结工作树上重新执行 exact 7/7 minimum，只有全绿才写 Git-local machine receipt，当前 receipt 状态以 fresh `release-check` 为准。

## 当前批次卡

### Batch 828：STeamAI self-contained project closure

**目标**：让一个真实项目目录携带自己的 skill、状态、runtime 和 pack，用户只需在项目中启动已有 Claude Code 并使用 `/steamai` 或自然语言；复制项目且中央 kit 不可用时仍能恢复、查询和继续。

**用户断点**：旧流程要求先理解 kit/case、中央 runtime、thin shim、`.rekit` 和大量底层命令；默认 status 可能挤占主 Agent 上下文，复杂故障需要维护者知识，heavy action 又缺少诚实的免逐次询问档位。

**范围内**：

- STeamAI 品牌、`/steamai`、`.steamai` 和 self-contained bundle；
- dual-read/single-write legacy compatibility 与显式 migration；
- compact status、人话 recovery、bounded autonomy；
- retry/fence、structured failure 和 handoff currentness correctness 回归；
- Windows copied-directory product E2E 和文档路由收口。

**范围外**：

- Claude Code 安装、登录或全局 plugin 管理；
- 无限/无记录/无边界授权，case-wide 或 multi-lane v2 grant；
- GUI/TUI、独立 installer、远程 CI、Linux/macOS product E2E；
- 仓库名、Go module、所有 package/executable 的机械改名；
- authority/confirmed 自动写入、自动 sync/promote 或未授权 heavy action。

**当前结果**：`completed`。state-root、`/steamai`、compact recovery、bounded autonomy、project-local bundle/copy、legacy migration、current-sync recovery与durable supervisor handoff均通过focused/E2E；真实hard-kill后late child零启动。独立复核关闭supervision mutation-before-spec和handoff capability-ordering缺口，无surviving blocker。Windows全仓tests、vet、module verify、public CLI、diff和local minimum通过。未授权commit/push，cadence为`implementation-pending`；无下一批，不声称remote CI green。

**完成门槛**：

- current `.steamai`、legacy `.rekit` 和 dual-root fail-closed package/E2E；
- copied project 在中央 kit 不可用时从 project-local runtime 完成 status/packs/doctor/daily；
- compact 输出含换行不超过 4096 bytes，request/choices 与 full identity parity；
- recovery 正反边界、bounded autonomy provision/evaluate/revoke、migration preview/Apply/replay 全绿；
- `go test -count=1 -p=2 -timeout=30m ./...`、`go vet ./...`、`go mod verify`、公开 release-check/status/packs/doctor 与 `git diff --check` 全绿；
- canonical docs 不再把中央 kit/thin shim 或 `/rekit` 当新项目默认。

## 验证标准

- Windows 本机完整 release minimum 全部通过；remote jobs 不参与当前完成判断。
- project metadata 和 bundle 不保留旧项目绝对路径作为运行依据；copy 后重新解析 `${CLAUDE_PROJECT_DIR}`。
- current target 即使从 kit CWD 调用，也不能被中央 source repo 覆盖；bundle 缺失/篡改时 fail-closed，不走 PATH fallback。
- `.steamai` 与 `.rekit` 共存、root object 非普通目录、reparse/junction/symlink、路径逃逸、执行中切根均拒绝。
- migration 不改变 authority/confirmed/gate/autonomy/evidence bytes 的语义，不执行 heavy action。
- route/current/state/claim/next 与 `docs/batch-plan.md` 完全一致；冲突时停止实施。

## 风险与注意事项

- 自包含 runtime bundle 与 copied-directory/no-central-kit 已有 production executable E2E；后续改动不得用仅模拟 central repo attachment 的 fixture 替代该门禁。
- `bounded-autonomous-v1` 的用户名称可以表达“较高自治”，但文档和 runtime 必须清楚显示 exact scope、budget、expiry 与停止条件，不能说成无限全权。
- legacy compatibility 允许明确 `.rekit` literals 和 `/rekit` commands；静态门禁应禁止 current mutable owner 新增直写，不应盲删历史、fixture 或兼容字符串。
- actual heavy-tool、authority/confirmed、sync/promote、未授权外部副作用和公共 façade 删除继续按原 gate 升级。
- package/focused/E2E与独立复核不能单独替代完整 release minimum；本批三者均已关闭。冻结后的 machine receipt只证明同一Windows工作树的local gate，不证明commit/push或remote CI green。
- current-sync Apply 与 current durable detached handoff要求handle-bound exact mutation；Windows支持，非Windows在任何持久化副作用前fail-closed。Read-only/preview和legacy zero-handoff compatibility保留；cross-compile不是runtime evidence。

## 路线变更记录

- 2026-08-16：Batch 828按Windows口径完成；独立复核无surviving blocker，全仓local minimum通过。非Windows exact-mutation owner路径在副作用前fail-closed；cross-compile不冒充runtime E2E。未授权commit/push，cadence为`implementation-pending`，路线completed/no-next。
- 2026-08-14：建立本路线；产品模型固定为自包含项目、`/steamai`、`.steamai`、project-local runtime/pack，旧入口仅迁移兼容。更早历史见`docs/batch-history.md`。
