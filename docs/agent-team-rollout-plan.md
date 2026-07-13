# Agent Team rollout plan

## 读取指南

- 本文件是 Agent Team 从"契约已固化、编排未落地"推进到"可端到端 dry-run 验证"的实施计划，对应 `docs/vision.md` Phase 5/6 与 `docs/orchestration-plan.md`。
- 维护者开始新批次前先读本文件顶部：读取指南、实施摘要、执行清单、验证标准、风险与注意事项，再按批次读细节。
- 本文件只写计划与契约压测方法，不替代 `common/policies/agent-team.md` 和 `common/policies/subagents.md` 的契约定义。
- 推进姿态为 **选项 C：契约 dry-run + 临时 case 验证优先**。先压测契约，再按真实缺口决定 ledger runtime（Phase 5）与 bounded dispatch（Phase 6）的顺序。
- 当前状态：PowerShell `/rekit` 仍是公共入口；R0-R7、B/C 系列、D2-D4、Go G1-G5 已完成。Ledger runtime 支持 9 种 kind、`batchId`、overview/handoff/note-List 读层和 intervention/rollback 展示闭环；`continue` auto 流程写 `.rekit/runs/<run-id>/digest.md`，Go backend 可输出 `continue -WhatIf` 非写入 preview，并在 explicit `continue -Apply` 中写 case-local facts/routing/run digest/lane resume/checkpoint/board，同时 defer authority/confirmed。`plan-subagents` 仍是只读计划器，已输出 route/shard/review-loop observability，但不自动 spawn reviewer。Go backend 已默认接管 `sync/promote` review/apply、gate preview/request、note append/list 文本与 JSON、overview 文本/JSON 与缺 board 初始化、start/handoff JSON preview 与显式 apply，以及 continue JSON preview/explicit apply；Batch 122 已用 `_template` pack 的 Go package test 锁定 start → note → continue apply → handoff 最小闭环；无 `-Apply` 的 continue/text 工作线 flow 仍由 PowerShell fallback 承担。Batch 101 后的新阶段导航见 `docs/go-first-convergence-plan.md`；后续优先 Go deterministic runtime 收口、release readiness、Agent Team 真实 dry-run 闭环与 PowerShell 收缩。

## 实施摘要

Agent Team 当前真实状态是**契约层完整、ledger/runtime 基础已落地、自动编排与强制 gate 仍待推进**：

- 已固化：`common/policies/agent-team.md`（角色 + packet + 状态流）、`common/policies/subagents.md`（L1/L2/L3 + output contract）、manifest `subagentRoutes`、`B3` 工作线 runtime、vmp-re pack、`_template` pack。
- 已落地：PowerShell ledger runtime（9 种 kind、batchId、overview/handoff/note-List、run digest）、sync/promote review-first、Go review-only plan/artifacts、Go gate dry-run 与 pending-gate request 写入、Go façade 对 read-only/case lifecycle/sync/promote/ledger/gate/start/handoff/continue preview/apply safe subset 的默认接管，以及 `_template` pack 的 Go package 最小闭环测试。
- 未落地：bounded dispatch 自动 spawn、candidate → verified → confirmed 的机器强制 gate、真实 heavy-tool 执行闭环、PowerShell façade 全量 shim 化、更多非 `vmp-re` pack 的真实 dry-run 脚本闭环。

本计划先用临时 case 端到端 dry-run 压测 main → feature → reviewer → confirmed 全流程，再按 dry-run 暴露的真实需求分批做 ledger runtime、gate 与 bounded dispatch。当前文档保留历史批次记录，新批次以顶部当前状态和 `docs/batch-plan.md` 最新 batch 为准。

## 执行清单

- [x] R0：dry-run 脚手架与范围定义（不写 runtime） — 已建临时 case 与 dry-run 脚本，详见 `docs/agent-team-dryrun-script.md`
- [x] R1：临时 case 端到端契约压测（手动 agent 流） — 产物在临时 case workspace，缺口记于 `docs/agent-team-dryrun-script.md` §6
- [x] R2：契约缺口回写 policy / manifest — agent-team/evidence/handoff/subagents 已补
- [x] R3：决策门 — ledger runtime 优先，理由见 §3.R3
- [x] R4：最小 ledger runtime 切片 — 新增 `note` 命令 + `Add-RekitFactEvent` + overview 聚合 + handoff workspace 引用
- [x] R5：最小 bounded dispatch 切片 — 判定不扩 runtime：spawn 是主会话/Claude Code 职责，runtime 已有 `plan-subagents` + `note` verdict 写回入口
- [x] R6：run digest 与 heavy-tool gate 最小切片 — heavy-tool gate 写入 vmp-re verification overlay，用 `note -Kind request -Status pending-gate` 记录
- [x] R7：批次自审、CHANGELOG、vision/batch-plan 回写

## 验证标准

R0-R2（契约压测阶段）：

1. 临时 case 可按 `agent-team.md` 契约手动跑完 main 派 task packet → feature 产出 evidence/candidate packet → reviewer 出 review packet → main 在 gate 通过后写 confirmed 的全流程。
2. 每类 packet 在实际 agent 输出里字段不缺、可合并、可 diff；output contract 的 `decision,confidence,evidence,risk,next_action` 字段在主 agent 合并台账时够用。
3. 发现的 schema 缺口已回写 `common/policies/agent-team.md` 或 `subagents.md`，并在 `CHANGELOG.md` 记录。
4. 临时 case 删除，未污染 kit 仓库；无真实样本/trace/dump/绝对路径进入模板。
5. `git diff --check` 通过；`./rekit/rekit.ps1 status`、`./rekit/rekit.ps1 doctor` 不回归。

R4-R6（runtime 切片阶段，按 R3 决定激活）：

6. ledger runtime 切片：`.rekit/facts/*.jsonl` 可 append observation/candidate/decision 事件，`/rekit overview` 展示未决 request、冲突 candidate、需人工确认项计数；append-only，可 grep、可 diff。
7. bounded dispatch 切片：`plan-subagents` 输出的分片计划可被主 agent 按 route 启动只读 reviewer 并回收 verdict；reviewer 不能写 authority；失败分片不影响其它分片。
8. run digest：每轮自动动作写 `.rekit/runs/<run-id>/digest.md`，含 inputs/route/packet refs/outputs/decisions/open risks；新会话只读 digest 能理解本轮（Batch 52 已补齐结构化 digest）。
9. heavy-tool gate：full-trace/debug/inject/patch/dump 触发确认问题与 budget 说明，无确认不执行；dry-run 可展示将问什么、将写什么。
10. 临时 case smoke：init/attach/sync/promote 不回归；任一 agent 失败不污染 confirmed。

## 风险与注意事项

- **不要把 dry-run 当成 runtime 实现**：R0-R2 只压测契约，不往 `rekit/lib` 加编排逻辑；避免做出"跑得通但契约不够用"的返工。
- **不要在 kit 仓库伪造 case state**：临时 case 用完即删；只有验证 `init/attach/sync/promote/continue/start/handoff` 行为时才创建。
- **dry-run 不执行真实 heavy-tool 动作**：full trace、动态调试、注入、patch、dump、网络、外部副作用一律 dry-run 或 mock，不碰真实样本。
- **不绕过 review-first**：sync/promote 仍默认 review；confirmed/authority 写入仍需人工确认；dry-run 中的 "confirmed" 只写临时 case workspace，不写 kit 模板。
- **不复制 runtime 逻辑到 case shim**：case-local `/rekit` 保持 thin shim。
- **Go runtime 渐进接入**：R4-R6 初期以 PowerShell runtime 为准；当前 Go façade 已按低风险 read-only、case lifecycle、sync/promote、ledger/gate、start/handoff 显式 apply 与 continue preview/apply safe subset 逐步默认启用。新增默认委托前必须有 smoke/package test 覆盖，`REKIT_GO_DISABLE=1` 继续作为 fallback。
- **R3 是决策门，不是自动推进**：dry-run 结果可能指向 ledger 优先、dispatch 优先、或两者并行；必须显式记录决策理由，不能默认全做。
- **schema 改动走文档先**：packet schema 缺口先回写 policy 文档，再考虑 runtime 校验；不在 dry-run 阶段做 schema 迁移。
- **批次写回**：每批完成后回写本文件、`docs/batch-plan.md`、`CHANGELOG.md` 与 `docs/vision.md` 执行清单；后续所有实施计划必须先落到文档，并随代码提交推送到远程 `main`，不能只留在聊天上下文。

## 1. 当前 Agent Team 状态评估

### 1.1 已落地

| 层 | 产物 | 状态 |
|---|---|---|
| 角色与 packet 契约 | `common/policies/agent-team.md` | main/feature/tooling/reviewer/verifier/human 六角色 + task/evidence/candidate/review/stuck 五类 packet + draft→candidate→review→confirmed 状态流 |
| 子 agent 契约 | `common/policies/subagents.md` + manifest `subagentRoutes` | L1/L2/L3 升级、bounded parallelism、output contract；vmp-re 声明 `bounded-review`、`lane-feature-analysis` 两条 route |
| 工作线 runtime | `rekit/lib/B3.*.ps1` | manifest 驱动主线/支线/authorityFiles/handoff；`status`/`doctor`/`overview`/`continue`/`start`/`handoff` 可跑 |
| pack 领域层 | `packs/vmp-re/**`、`packs/_template/**` | workflow-template、toolchain-router、tooling recipes、policy overlay 齐全且通过 budget 校验 |
| sync/promote review-first | `rekit/lib/Sync.ps1`、`Promote.ps1`、`Review.ps1` | review packet + bounded diff + sanitized preview；写入需确认 |
| Go runtime backend | `internal/rekit/**`、`cmd/rekit/main.go` | G1/G2.5 只读 status/doctor/manifest/case doctor + G2 sync/promote review/apply、G2.2/G2.3 gate preview/request 与 Stage 5 read/ledger/workstream layer 已逐步接入 façade；Batch 114 起 `gate -WhatIf` 默认委托 Go，Batch 115 起 `gate -Apply` pending-gate request 写入默认委托 Go，Batch 116 起 `note` append / `note -WhatIf` facts JSONL 写入/预览默认委托 Go，Batch 117 起 `start`/`handoff` JSON preview 与显式 apply 默认委托 Go，Batch 118 起 `continue -WhatIf -Format json` 非写入 preview 默认委托 Go，Batch 119 起 overview 文本/JSON 与缺 board 初始化默认委托 Go，Batch 120 起 `note -List` 文本/JSON 只读查询默认委托 Go，Batch 121 起 explicit `continue -Apply` case-local facts/routing/run digest/resume/board 写入默认委托 Go，Batch 122 起 `_template` pack package E2E 覆盖 start/note/continue/handoff 闭环，authority/confirmed 仍需人工确认 |

### 1.2 未落地

| 缺口 | 当前形态 | 期望形态 |
|---|---|---|
| bounded dispatch | `plan-subagents` 只输出分片计划，不启动 agent | 主 agent 按 route 启动只读 reviewer，回收 verdict，合并 accepted/rejected/deferred |
| evidence ledger runtime 强制 gate | `.rekit/facts/*.jsonl` append/聚合/查询已落地；candidate/verification/decision/intervention/rollback 可记账，但 confirmed/authority 仍需人工确认 | candidate → verified → confirmed 的机器强制 gate 与 authority 写入保护 |
| batch-level replay/resume | `batchId` 与 run digest 已落地；overview 可聚合 batch | batch-level replay、resume、整体接受/回滚自动化 |
| heavy-tool gate runtime | Go `gate -WhatIf` 可预览，`gate -Apply` 只写 pending-gate request；不执行工具 | runtime 在 full-trace/debug/inject/patch/dump 前强制确认 packet，并在确认后执行受控动作 |
| reviewer/verifier spawn 路径 | 靠主会话自觉 | 强契约：route → packet → spawn → verdict → merge |

## 2. 推进姿态：选项 C

**先契约 dry-run，再按真实缺口决定 runtime 顺序。**

理由：

- 契约文档很完整但没有经实际 agent token 流压测；直接做 runtime 编排（选项 A）容易做出"跑得通但契约不够用"的返工。
- 直接做 ledger runtime（选项 B）会让"Agent Team 能跑"的可见进展慢，且 ledger 字段没经压测也可能返工。
- 选项 C 用最低成本把契约压测一遍，再按真实缺口决定 A/B 顺序，符合 vision "先契约后自动化、小步可验证" 原则与 CLAUDE.md "最小可行但不牺牲架构清晰" 约束。

不选 A 的理由：编排先于账本会导致 digest 与 facts 各写一套，后续回填成本高。
不选 B 的理由：ledger 先于编排会让字段设计脱离真实 agent 输出，且可见进展慢。

## 3. 分批实施

### R0：dry-run 脚手架与范围定义

**目标：** 在不写 runtime 的前提下，定义一次可重复的端到端契约压测。

**切片：**

1. 选定一个临时 case 目录（kit 仓库外），用 `/rekit init -Target <tmp> -Pack vmp-re -ProjectName agent-team-dryrun`。
2. 准备一份 mock 分析对象（不引入真实样本；可用纯文本/伪 RVA 占位，仅用于让 packet 字段有内容）。
3. 写一份 `docs/agent-team-dryrun-script.md`（或本文件附录），固定 dry-run 的步骤、packet 样例、期望输出。
4. 不改 runtime、不改 policy。

**产物：** 临时 case（`C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun`，kit 仓库外）、`docs/agent-team-dryrun-script.md`。

**完成状态：** R0 已完成。临时 case `init`/`status`/`doctor` 通过；dry-run 脚本覆盖 S1-S5 五步与字段缺口记录区，供 R1 执行。

**验证：** 临时 case `status`/`doctor` 通过；dry-run 脚本可被另一个会话照着跑。

### R1：临时 case 端到端契约压测

**目标：** 手动按 `agent-team.md` 契约跑一次全流程，暴露 packet schema 与 output contract 缺口。

**切片：**

1. 主 agent 读 `agent-team.md`、`subagents.md`、vmp-re `subagentRoutes`，按 `task packet` 字段派一个 task 给 feature lane。
2. feature agent（可用主会话切换视角或子 agent 模拟）产出 `evidence packet` + `candidate packet`，只写 lane workspace。
3. reviewer agent 按 `review packet` 字段做只读复核，输出 `decision,confidence,evidence,risk,next_action`。
4. 主 agent 合并 verdict，在 gate 通过后写 confirmed（仅临时 case workspace，不写 authority CSV）。
5. 全程记录：哪些字段实际用到、哪些缺、哪些多余、output contract 是否够合并、handoff 是否能从 packet 还原。

**产物：** dry-run 产物（packet 样例、verdict 台账、字段缺口清单）。

**验证：** 全流程走完；字段缺口清单可读；未执行任何真实 heavy-tool 动作。

### R2：契约缺口回写

**目标：** 把 R1 暴露的 schema 缺口回写 policy/manifest，不改 runtime。

**切片：**

1. 按 R1 字段缺口清单更新 `common/policies/agent-team.md` 或 `subagents.md` 的 packet 字段。
2. 若 vmp-re `subagentRoutes` 的 `outputContract` 字段不够，更新 `packs/vmp-re/manifest.yml`（仅契约字段，不增 route）。
3. 在 `CHANGELOG.md` 记录契约调整。
4. 重新跑 `doctor` 确认 manifest/policy 校验不回归。

**产物：** policy/manifest 契约补丁、CHANGELOG。

**验证：** `doctor` 通过；`git diff --check` 通过；契约文档与 dry-run 实际字段一致。

### R3：决策门

**目标：** 基于 R1/R2 结果显式决定 Phase 5（ledger runtime）与 Phase 6（bounded dispatch）的实施顺序与切片大小。

**决策输出（写入本文件 §3.R3）：**

- ledger 优先 / dispatch 优先 / 并行的理由。
- 每个切片的最小可验证范围。
- 是否需要先做 run digest 或 heavy-tool gate 的子切片。

**必须停下询问用户的情况：** 决策涉及 runtime schema 迁移、破坏性动作、或难以判断的架构取舍时，先问用户。

### R3 决策结果

**决策：ledger runtime 优先（R4 先做），bounded dispatch（R5）后做。**

R1 关键发现：facts JSONL 存储骨架（`.rekit/facts/observations|candidates|decisions|publications|requests.jsonl`）已由 `start` 自动创建，但 runtime 没有"手动 append 单条 event"入口，仅在 `continue` auto 流程中由 `B3.Auto.ps1` 从 lane CSV/workspace 扫描写入；手写 packet 不被 `overview` 计数。

理由：

1. R1 已证实"agent 产出无法被 runtime 感知"是当前最大断点，直接影响 overview / handoff / stuck statistics 的可信度。
2. bounded dispatch 的 reviewer verdict 合并需要 ledger 作为 single source of truth；若 dispatch 先上线，digest 与 facts 会各写一套，后续回填成本高。
3. dispatch 涉及 spawn 子 agent，复杂度更高，应建立在已验证的 ledger 之上。

R4 切片范围因此定为：补"手动 append event"入口 + `overview` 聚合手写 event + handoff 引用 workspace packet。不涉及 schema 迁移（event 字段沿用 R2 回写后的契约）。

### R4：最小 ledger runtime 切片（条件激活）

**激活条件：** R3 指向 ledger 优先或并行。

**切片：**

1. 在 `rekit/lib/B3.State.ps1`（或新模块）实现 `.rekit/facts/*.jsonl` 的 append：observation、candidate、decision 三类事件先落地，intervention/rollback 后续。
2. 事件字段按 `docs/evidence-ledger.md` + R2 回写后的契约。
3. `/rekit overview` 增加未决 request、冲突 candidate、需人工确认项计数。
4. append-only，不删事件；rejected/superseded 用反向事件。

**验证：** 见验证标准 §6。

### R5：最小 bounded dispatch 切片（条件激活）

**激活条件：** R3 指向 dispatch 优先或并行。

**判定：不扩 runtime。** spawn 子 agent 是主会话/Claude Code 的职责，不是 PowerShell runtime 职责。runtime 侧已有的 `plan-subagents`（输出分片计划）+ R4a 的 `note -Kind decision`（verdict 写回）已构成 dispatch 所需的 runtime 支撑。真启 reviewer 由主会话用 Agent 工具完成，verdict 通过 `note` 写回 ledger，主 agent 合并台账。因此 R5 无需新增 runtime 代码，判定为已完成于 R4a + 现有 `plan-subagents`。

后续若要让 `/rekit continue` 自动按 route spawn reviewer，属于 Phase 6 后段，等 ledger runtime（R4）和 dispatch 契约在多个真实 case 验证后再做。

### R6：run digest 与 heavy-tool gate 最小切片

**切片：**

1. run digest：沿用 `continue` auto 流程已有的 `.rekit/runs/<run-id>/digest.md`，不新增 runtime；`note` 是手动单事件入口，不产生 digest（手写 packet 本身是 source of truth）。
2. heavy-tool gate：写入 `packs/vmp-re/policies/verification.overlay.md`，规定 full-trace/debug/inject/patch/dump/network 触发前用 `note -Kind request -Status pending-gate` 记录 gate 事件（含 tried_light_steps/budget/stop_conditions），用户确认后才执行，执行后用 `note -Kind observation` 记录结果。runtime 当前不强制 gate，这是 agent 行为契约。

**验证：** policy 已落盘；gate 事件可通过 `note` 写入并被 overview 计数。runtime 强制 gate 属 Phase 6 后段。

### R7：批次自审与回写

**切片：**

1. 自审 diff：架构是否清晰、模块边界是否保持、是否引入同步副作用或维护风险。
2. 回写 `docs/batch-plan.md`、`CHANGELOG.md`、`docs/vision.md` 执行清单。
3. 临时 case 删除确认；kit 仓库 `git status` 干净（除计划文档外）。

## 4. 决策表

| 你现在的情况 | 推荐动作 |
|---|---|
| 想压测契约 | 跑 R0 → R1 → R2 |
| R1 暴露字段缺口 | R2 回写 policy/manifest，再回 R1 复跑 |
| R2 完成 | 进 R3 决策门，决定 R4/R5 顺序 |
| 想让 Agent Team 可编排 | R3 指向 dispatch 优先 → R5 |
| 想让 Agent Team 可审计 | R3 指向 ledger 优先 → R4 |
| R4/R5 稳定后 | R6 digest + heavy-tool gate |
| 每批结束 | R7 自审 + 回写 |

## 4. Kit runtime hardening（R0-R7 后续优化）

### 读取指南

- 本节是 R0-R7 完成后的后续优化计划，方向：kit runtime 强化（用户选择"没有真实 case，先强化 kit runtime"）。
- 不碰 bounded dispatch 自动 spawn（R5 已判定 runtime 不扩）和 Go 迁移（按 `docs/go-runtime-migration.md` 独立推进）。
- 每批保持小步可验证，沿用 R0-R7 的临时 case `agent-team-dryrun` 验证。
- 开始新批次前先读本节顶部：实施摘要、执行清单、验证标准、风险与注意事项。

### 实施摘要

R0-R7 把 ledger 从"断的"推进到"可写"（`note` 命令 + overview 计数 + handoff workspace 引用），但仍有三个真实缺口：

1. **`note` 无 schema 校验**：`note -Kind observation -Confidence garbage` 也会写入，污染账本。
2. **`overview` 只计数不列明细**：stuck statistics 形同虚设，用户看不到"哪些 candidate 未决、哪些 gate pending"。
3. **`handoff` 只扫 workspace 文件，不读 decision event**：新会话看不到最新 decision 摘要，只看到 packet 文件路径。

本节按 B1-B3 三批补这三个缺口，把 ledger 从"可写"推进到"可查可聚合"。不扩 ABI（仍是内部 `note` 命令），不碰 schema 迁移（event 字段沿用 R2 回写后的契约）。

### 执行清单

- [x] B1：`note` schema 校验 — `Add-RekitFactEvent` 校验 confidence/decision/status 枚举、evidenceRefs 非空、lane 存在性（Board 传入时）
- [x] B2：`overview` 未决/pending/冲突明细 — 列出未决 candidate（含冲突标记）、pending-gate request，限 N=10
- [x] B3：`handoff` 引用 decision event — `## decision` 区段列 latest 5 条 decision 摘要
- [x] B4：批次自审 + CHANGELOG + batch-plan 回写
- [x] B5：`handoff` 补 pending-gate 区段 + decision 兼容 `action` 字段（auto 流程用 `action`，`note` 用 `decision`）
- [x] B6：`overview` 补"最近 decision"明细，兼容 `action` 字段
- [x] B7：`note -List` 查询（按 kind/lane 过滤，限 N=20）

### 验证标准

B1：

1. `note -Kind observation -Confidence garbage` 被拒绝，不写入 JSONL，错误信息明确指出非法字段。
2. `note -Kind observation -Confidence low` 仍正常写入。
3. `note -Kind decision -Lane <不存在的 lane>` 被拒绝（lane 存在性校验）。
4. `note -Kind candidate -EvidenceRefs "id1,id2"` 写入后 `evidenceRefs` 字段为数组 `["id1","id2"]`。
5. 临时 case `doctor` 不回归；kit `pack validation ok`。

B2：

1. `overview` 在有未决 candidate 时列出 subject + summary + confidence（不只计数）。
2. `overview` 在有 `Status=pending-gate` request 时列出 subject + summary（heavy-tool gate 可见）。
3. `overview` 在有同 subject 冲突 candidate（不同 claim）时标记冲突。
4. 计数仍保留（向后兼容）。
5. 临时 case `doctor` 不回归。

B3：

1. `note -Kind decision` 后，`/rekit handoff <lane>` 含 latest N 条 decision 摘要（subject + decision + reason）。
2. handoff 仍保留 workspace packet 引用区段（B3 不覆盖 R4b）。
3. 无 decision 时不出现空区段。
4. 临时 case `doctor` 不回归。

B4：

1. `git diff --check` 通过。
2. `./rekit/rekit.ps1 doctor` / `doctor -Target <tmpCase>` 通过。
3. `go test ./...` 通过（不涉及 Go，但确认不回归）。
4. CHANGELOG、batch-plan 记录 B1-B3。

### 风险与注意事项

- **B1 校验不扩成 schema 迁移**：只校验枚举和格式，不引入 JSON Schema 库或新 schema 版本；event 字段沿用 R2 契约。
- **B1 lane 存在性校验要读 board**：`note` 已 `Ensure-RekitBoard`，校验时读 board.lanes 即可，不额外开文件。
- **B2 冲突检测保持简单**：按 subject 聚合，同 subject 多个 open candidate 即标记冲突；不做语义相似度。
- **B2 overview 输出体积**：明细列表要限 N 条（建议 10），避免大 case 刷屏；超出提示"另有 N 条"。
- **B3 handoff decision 摘要限 N 条**：建议 latest 5，避免 handoff 膨胀。
- **不碰 bounded dispatch / Go 迁移**：这两条独立线，不在本节范围。
- **批次写回**：每批完成后回写本节、`docs/batch-plan.md`、`CHANGELOG.md`。

### 4.B1：`note` schema 校验

**目标：** 阻止非法输入污染账本。

**切片：**

1. 在 `Add-RekitFactEvent`（`B3.State.ps1`）加校验：
   - `Confidence` ∈ `{low,medium,high,''}`
   - `Decision` ∈ `{confirm,reject,defer,supersede,''}`（仅 kind=decision 时校验）
   - `Status` ∈ `{open,deferred,pending-gate,confirmed,rejected,superseded,needs_more_evidence,''}`
   - `Lane` 必须存在于 `board.lanes`（读 `Ensure-RekitBoard` 返回的 board）
2. `EvidenceRefs` 已是数组，校验每项非空字符串。
3. 非法时 throw 明确错误（字段名 + 合法值），不写入。
4. `Invoke-RekitNote`（`B3.Commands.ps1`）不重复校验，由 `Add-RekitFactEvent` 统一把关。

**改动文件：** `rekit/lib/B3.State.ps1`。

**验证：** 见验证标准 B1。

### 4.B2：`overview` 未决/pending/冲突明细

**目标：** 让 stuck statistics 可读，不只计数。

**切片：**

1. `Show-RekitOverview`（`B3.Commands.ps1`）在读 facts JSONL 后，除计数外增加明细区段：
   - "未决 candidate"：列出 `candidates.jsonl` 中 `status` 非终态的 latest N 条（subject + summary + confidence）。
   - "pending-gate"：列出 `requests.jsonl` 中 `status=pending-gate` 的条目（subject + summary）。
   - "冲突 candidate"：按 subject 聚合 candidates，同 subject 多个 open 即标记冲突。
2. N=10，超出提示"另有 N 条"。
3. 计数区段保留（向后兼容）。

**改动文件：** `rekit/lib/B3.Commands.ps1`。

**验证：** 见验证标准 B2。

### 4.B3：`handoff` 引用 decision event

**目标：** 新会话从 handoff 看到最新 decision，不必读 workspace 文件全文。

**切片：**

1. `Write-RekitLaneHandoff`（`B3.Commands.ps1`）在 workspace packet 区段后增加 "decision" 区段：
   - 读 `.rekit/facts/decisions.jsonl`，过滤 `lane == $laneId`，取 latest N=5 条。
   - 每条列 `subject + decision + reason`。
2. 无 decision 时不出现空区段。
3. 保留 workspace packet 引用区段（R4b）不动。

**改动文件：** `rekit/lib/B3.Commands.ps1`。

**验证：** 见验证标准 B3。

### 4.B4：批次自审与回写

**切片：**

1. 自审 diff：架构是否清晰、校验逻辑是否在单一函数、overview 明细是否限 N。
2. 回写 `docs/batch-plan.md`、`CHANGELOG.md`、本节执行清单。
3. 临时 case 保留供后续验证。

## 5. Ledger schema 对齐与架构治理（C 系列）

### 读取指南

- 本节是 B1-B7 完成后的后续优化计划，方向：让 runtime ledger 对齐 `docs/evidence-ledger.md` 草案（吸收 `clark-utov` 的设计契约），消除"runtime 自定一套字段、偏离草案"的漂移。
- 起因：B 系列 `Add-RekitFactEvent` 自定了 `decision: confirm|reject|defer|supersede`、status 含 `pending-gate`/`needs_more_evidence`、缺 `actor`/`risk`/`related`/`candidateId`/`claim`/`limitations`/`verifier`/`verdict`/`confirmedBy`/`writes`/`batchId`，且 auto 流程用 `action` 字段而非草案的 `decision`。这是"造轮子"——`evidence-ledger.md` 草案已在本仓库，runtime 应对齐它而非另定。
- 每批保持小步可验证，沿用临时 case `agent-team-dryrun`。
- schema 改动遵循"兼容旧数据、不破坏现有 case、每次只改一个 schema 点"原则；遇到不兼容或破坏性改动停下问用户。

### 实施摘要

`evidence-ledger.md` 草案定义了 9 种事件类型（observation/hypothesis/candidate/verification/decision/intervention/rollback/publication/request）和基础字段（schemaVersion/eventId/kind/time/actor/lane/subject/summary/evidenceRefs/related/status/risk/confidence），以及 candidate/verification/decision/intervention 的扩展字段。runtime 当前只实现 5 种 kind，字段集是自定子集。

本节按 `evidence-ledger.md` §"Runtime 落地顺序"（digest → facts JSONL 统一 → overview stuck statistics → 索引优化）分 C1-C8 八批对齐，不拆循环依赖、不跨语言共享，YAGNI。

### 偏差清单（runtime vs 草案）

| 草案定义 | runtime 当前实现 | 偏差 |
|---|---|---|
| 9 种 kind | 5 种（缺 hypothesis/verification/intervention/rollback） | 漏 4 种 |
| `decision: accept\|reject\|defer\|supersede` | `confirm\|reject\|defer\|supersede` + auto 用 `action` | `accept` vs `confirm`，且 auto 另一套 |
| `status: open\|accepted\|rejected\|superseded\|resolved\|deferred` | `open\|deferred\|pending-gate\|confirmed\|rejected\|superseded\|needs_more_evidence` | 枚举集不同（C2 取并集：保留 runtime 3 值不破坏 R6 契约，新增 accepted/resolved） |
| `actor` 字段 | 无 | 缺 |
| `risk` 字段 | 无 | 缺 |
| `related` 字段 | 无 | 缺 |
| candidate: `candidateId`/`claim`/`limitations`/`proposedAuthorityWrite`/`nextAction` | 无 | 几乎全缺 |
| verification kind: `verifier`/`verdict` | 无该 kind | 缺 |
| decision: `confirmedBy`/`writes` | 无 | 缺 |
| intervention: `action: override\|rollback\|heavy-tool-approval\|...` | 无该 kind | 缺 |
| `batchId` | 只有 `runId` | 不等价 |

### 执行清单

- [x] C1：docs 冲突修正 + SKILL.md 补 note 命令
- [x] C2：`Add-RekitFactEvent` 基础字段对齐草案（补 actor/risk/related，decision 枚举改 accept|reject|defer|supersede，status 枚举对齐）
- [x] C3：补 4 种事件类型 hypothesis/verification/intervention/rollback 的 ValidateSet + 扩展字段
- [x] C4：auto 流程 `New-RekitDecision` 改用草案 decision 字段 + confirmedBy/writes，读层兼容历史 action
- [x] C5：overview/handoff/note-List 展示对齐新字段（actor/risk/verdict 等）
- [x] C6：policy 文档去重（evidence.md 降级、handoff.md 补 decision、tool-output.md 并入 tool-adapters.md）
- [x] C7：handoff 子系统独立成 B3.Handoff.ps1
- [x] C8：自审 + CHANGELOG + batch-plan 回写

### 验证标准

C1：

1. `orchestration-plan.md` O2 标注"已被 rollout R5 否决"。
2. `vision.md` §8 候选清单同步 `batch-plan.md` 实际完成状态。
3. `rollout-plan.md` §"当前状态"段同步 R4-R6 已完成。
4. `SKILL.md` 补 `note` 命令（含 -List），标注为"账本写入/查询入口"。
5. `doctor` / `git diff --check` 通过。

C2：

1. `note -Kind decision -Decision accept` 写入成功；`-Decision confirm` 被拒（枚举改为 accept|reject|defer|supersede）。
2. `note -Kind observation -Actor main -Risk low` 写入后 JSONL 含 actor/risk 字段。
3. `note -Kind candidate -Status accepted` 写入成功；`-Status pending-gate`（heavy-tool gate）仍可写入（R6 契约保留）；读层把 `confirmed` 归一为 `accepted` 展示。
4. 历史 JSONL 仍可读（overview/handoff/note-List 不崩）。
5. `doctor` 通过。

C3：

1. `note -Kind hypothesis` / `-Kind verification` / `-Kind intervention` / `-Kind rollback` 写入成功。
2. verification kind 支持 `-Verifier manual-review -Verdict accepted -Target cand-...`。
3. intervention kind 支持 `-Action override -ApprovedBy user -Scope ...`。
4. `note -List` 能展示新 kind。
5. `doctor` 通过。

C4：

1. `continue` auto 流程写的 decision event 含 `decision`（不再是 `action`）+ `confirmedBy: runtime` + `writes`（如有 authority 写入）。
2. 历史 `action` 字段的 decision event 仍可被 overview/handoff 读（兼容层）。
3. 临时 case `continue` smoke 不回归。
4. `doctor` 通过。

C5：

1. overview "最近 decision" 显示 actor/confirmedBy（如有）。
2. handoff decision 区段显示 confirmedBy。
3. `note -List -Kind verification` 显示 verifier/verdict。
4. `doctor` 通过。

C6：

1. `evidence.md` 改为 `agent-team.md` 的原则补充，不再独立定义 packet schema。
2. `handoff.md` 补 decision event 引用。
3. `tool-output.md` 并入 `tool-adapters.md`（或标注为 deprecated 指向）。
4. `doctor` 通过。

C7：

1. `B3.Handoff.ps1` 含 4 函数（Get-RekitLatestRunDigestPath/Write-RekitProjectHandoff/Write-RekitLaneHandoff/Write-RekitHandoff）。
2. `B3.Commands.ps1` 降到 ~320 行。
3. `rekit.ps1` dot-source 新模块。
4. handoff 命令行为不变（smoke 通过）。
5. `doctor` 通过。

C8：

1. `git diff --check` 通过。
2. `go test ./...` 通过。
3. CHANGELOG、batch-plan 记录 C1-C7。
4. `reference-absorption.md` 候选清单勾选 ledger runtime 项（如完成）。

### 风险与注意事项

- **schema 改动兼容性**：C2/C3/C4 改字段名和枚举，历史 JSONL 含旧值（`confirm`/`action`/`pending-gate`）。读层必须兼容，写入用新枚举。不迁移历史数据（append-only 原则，旧事件保留原样）。
- **每次只改一个 schema 点**：C2 改基础字段+枚举，C3 加 kind，C4 改 auto decision 字段——不合并批次，每批验证后再进下一批。
- **不破坏现有 case**：临时 case `agent-team-dryrun` 的 facts JSONL 在 C2 后仍可读；若发现不兼容，回滚该批。
- **不拆循环依赖**：State↔Lane↔Policy 循环虽不优雅但可工作，C 系列不碰，YAGNI。
- **不跨语言共享**：Go 是 read-only skeleton，重复风险可控；等 Go 扩展写路径再说。
- **policy 去重不丢信息**：C6 合并文档时确保 single source of truth，其它处改引用而非删内容。
- **handoff 拆分是机械移动**：C7 只搬函数不改逻辑，dot-source 顺序注意依赖。
- **批次写回**：每批完成后回写本节、`docs/batch-plan.md`、`CHANGELOG.md`。
- **schema 判断由维护者把关**：用户授权维护者判断 schema 改动是否需停下；遇真正破坏兼容性或难判架构取舍时仍停下问。

### 5.C1：docs 冲突修正 + SKILL.md 补 note

**目标：** 消除 docs 间滞后/冲突，让 agent 能发现 `note` 命令。

**切片：**

1. `docs/orchestration-plan.md` O2 段加注"bounded dispatch 已被 `agent-team-rollout-plan.md` R5 否决；本节保留设计参考"。
2. `docs/vision.md` §8 候选清单第 7 条更新为"`packs/_template` 与 B3.Lane 修复已完成；后续按 `docs/batch-plan.md` 推进"。
3. `docs/agent-team-rollout-plan.md` §"当前状态"段（行 9 附近）同步 R4-R6 已完成。
4. `.claude/skills/rekit/SKILL.md` 命令清单补 `note`，说明为"账本写入/查询入口，heavy-tool gate 登记、decision/observation 手动落账"，并说明 `-List` 查询。
5. 不改 runtime。

**改动文件：** `docs/orchestration-plan.md`、`docs/vision.md`、`docs/agent-team-rollout-plan.md`、`.claude/skills/rekit/SKILL.md`。

**验证：** 见验证标准 C1。

### 5.C2：`Add-RekitFactEvent` 基础字段对齐草案

**目标：** 让 `note` 写入的字段集和枚举对齐 `evidence-ledger.md` 基础字段。

**切片：**

1. `Add-RekitFactEvent`（`B3.State.ps1`）参数补 `-Actor`、`-Risk`、`-Related`。
2. `validDecision` 改为 `accept|reject|defer|supersede`（草案定义）。
3. `validStatus` 改为草案 6 值 ∪ runtime 已落地 3 值 = `open|accepted|rejected|superseded|resolved|deferred|pending-gate|confirmed|needs_more_evidence`。**保留 `pending-gate`/`confirmed`/`needs_more_evidence`**：R6 heavy-tool gate 契约（`packs/vmp-re/policies/verification.overlay.md` 用 `note -Kind request -Status pending-gate` 登记 gate、`overview`/`handoff` 读层过滤 `pending-gate`）依赖这三个值，移除会破坏现有 case 与 overlay；草案枚举是对齐方向但不以破坏已落地契约为代价。新增 `accepted`/`resolved` 供新写。读层把 `confirmed` 归一为 `accepted` 展示。
4. event hashtable 补 `schemaVersion: 1`、`actor`、`risk`、`related`（非空时）。
5. 读层（overview/handoff/note-List）兼容旧枚举（`confirm`→`accept`、`pending-gate` 归入 intervention 提示）。
6. `Invoke-RekitNote` 参数透传 `-Actor`/`-Risk`/`-Related`。

**改动文件：** `rekit/lib/B3.State.ps1`、`rekit/lib/B3.Commands.ps1`、`rekit/rekit.ps1`（note 分发解析新参数）。

**验证：** 见验证标准 C2。

**schema 判断：** 枚举值改名（`confirm`→`accept`）是 schema 演进，但读层兼容旧值、历史数据不迁移、append-only 保留旧事件，不破坏现有 case。可做。

### 5.C3：补 4 种事件类型

**目标：** 让 `note` 支持草案定义的全部 9 种 kind。

**切片：**

1. `Add-RekitFactEvent` ValidateSet 补 `hypothesis`/`verification`/`intervention`/`rollback`。
2. `Get-RekitFactFilePath` 补 4 个新文件路由：`hypotheses.jsonl`/`verifications.jsonl`/`interventions.jsonl`/`rollbacks.jsonl`。
3. `Get-RekitBoardPaths` 补 4 个路径属性。
4. `Ensure-RekitBoard` 初始化 4 个新空文件。
5. verification kind 扩展字段：`-Target`（指向 cand-id）、`-Verifier`（manual-review|schema-check|focused-trace|parity|cross-run|tool-review）、`-Verdict`（accepted|rejected|inconclusive|needs-more-evidence）、`-Notes`。
6. intervention kind 扩展字段：`-Action`（override|rollback|heavy-tool-approval|schema-migration|external-side-effect）、`-ApprovedBy`、`-Scope`、`-Expires`。
7. hypothesis/rollback 沿用基础字段。
8. `Invoke-RekitNote` / `note -List` 支持新 kind 和字段。

**改动文件：** `rekit/lib/B3.State.ps1`、`rekit/lib/B3.Commands.ps1`。

**验证：** 见验证标准 C3。

**schema 判断：** 新增 kind 是 schema 扩展，不影响现有 5 种 kind 的读写，不破坏。可做。

### 5.C4：auto 流程 decision 字段对齐

**目标：** 消除 `action` vs `decision` 字段分裂，auto 写的 decision event 也用草案 `decision` 字段。

**切片：**

1. `New-RekitDecision`（`B3.Auto.ps1`）改字段：`action` → `decision`，值改为草案枚举子集（auto 场景用 `accept`/`defer`/`reject`，`pending-user` 改为 `decision=defer` + `confirmedBy=runtime` + 标注需用户确认）。
2. 补 `confirmedBy: runtime`、`writes`（authority 写入时附 file/backup/diff）。
3. 读层（overview/handoff/note-List）兼容历史 `action` 字段：`$dec = $it.decision; if empty $dec = $it.action`。
4. `continue` auto 流程的 6 处 `Add-RekitJsonLine -Path $paths.Decisions` **保留直接写**（不改走 `Add-RekitFactEvent`）。原方案"统一写入路径"会因 `Add-RekitFactEvent` 的 eventId 去重破坏 auto 流程——auto 对同一 eventId 写两条记录（原 event 写到 kind 文件 + decision 写到 decisions.jsonl）是 by design，去重会拒第二条。C4 核心目标是消除 `action` vs `decision` 字段分裂，通过 `New-RekitDecision` 字段对齐实现，写入路径保留 `Add-RekitJsonLine` 直写。
5. 临时 case `continue` smoke 验证 auto 写入新字段、旧字段仍可读。

**改动文件：** `rekit/lib/B3.Auto.ps1`、`rekit/lib/B3.Commands.ps1`（读层兼容）。

**验证：** 见验证标准 C4。

**schema 判断：** auto 写入字段从 `action` 改 `decision` 是 schema 演进；读层兼容历史 `action`，append-only 保留旧事件。**但** `continue` 6 处改走 `Add-RekitFactEvent` 涉及 auto 流程逻辑改动，回归风险中等——这批做完先在临时 case 跑 `continue` smoke，若回归立即停下问用户。

### 5.C5：展示层对齐新字段

**目标：** overview/handoff/note-List 展示草案字段（actor/risk/confirmedBy/verdict 等）。

**切片：**

1. `Show-RekitOverview` "最近 decision" 显示 `actor`/`confirmedBy`（如有）。
2. `Write-RekitLaneHandoff` decision 区段显示 `confirmedBy`。
3. `Invoke-RekitNote -List` 对 verification 显示 `verifier`/`verdict`，对 intervention 显示 `action`/`approvedBy`，对 candidate 显示 `risk`。
4. 不改写入逻辑。

**改动文件：** `rekit/lib/B3.Commands.ps1`。

**验证：** 见验证标准 C5。

### 5.C6：policy 文档去重

**目标：** 消除 policy 7 处重叠，建立 single source of truth。

**切片：**

1. `evidence.md` 改为 `agent-team.md` 的"原则补充"小节，删除重复的 packet schema 定义，保留"3 条证据限制""packet↔event 关系"补充。
2. `handoff.md` 补"decision event 引用"（指向 `agent-team.md` decision event schema）。
3. `tool-output.md` 并入 `tool-adapters.md` 的"输出契约"小节，`tool-output.md` 改为 deprecated 指向。
4. `subagents.md` decision 枚举说明 overlay 如何 extend 通用 `accept|reject|defer|needs_l2|needs_l3`。
5. `review-first.md`/`write-boundaries.md` 互引，明确 review-first 偏流程、write-boundaries 偏边界。

**改动文件：** `common/policies/evidence.md`、`handoff.md`、`tool-output.md`、`tool-adapters.md`、`subagents.md`、`review-first.md`、`write-boundaries.md`。

**验证：** 见验证标准 C6。

### 5.C7：handoff 子系统独立

**目标：** `B3.Commands.ps1` 降到 ~320 行，handoff 边界更清晰。

**切片：**

1. 新建 `rekit/lib/B3.Handoff.ps1`，搬入 `Get-RekitLatestRunDigestPath`、`Write-RekitProjectHandoff`、`Write-RekitLaneHandoff`、`Write-RekitHandoff` 4 函数。
2. `rekit.ps1` dot-source `B3.Handoff.ps1`（在 `B3.Commands` 前，因 Commands 调 handoff 函数）。
3. `B3.Commands.ps1` 删除这 4 函数。
4. handoff 命令行为不变。

**改动文件：** 新建 `rekit/lib/B3.Handoff.ps1`、改 `rekit/rekit.ps1`、`rekit/lib/B3.Commands.ps1`。

**验证：** 见验证标准 C7。

### 5.C8：自审与回写

**切片：**

1. 自审 diff：schema 是否每批单点改、读层是否兼容、文档是否对齐。
2. 回写 `docs/batch-plan.md`、`CHANGELOG.md`、本节执行清单。
3. `docs/reference-absorption.md` 候选清单勾选已落地项。
4. 临时 case 保留供后续验证。

## 6. 后续实施方案（D 系列：batch / intervention / gate 闭环）

### 读取指南

- 本节是 C 系列完成后的下一阶段方案，方向：把 ledger 从"事件账本已对齐草案"推进到"批次、干预、回滚、门禁可闭环"。
- 优先级：先 D1 稳定性自检，再 D2 最小 batch 模型；不要直接跳到 IDA bridge adapter 或自动 dispatch。
- D 系列继续遵循：兼容旧 JSONL、append-only 不迁移历史、每批一个可验证切片、临时 case 验证、confirmed/authority/heavy-tool 仍需人工确认。

### 实施摘要

C 系列已经完成 9 种 ledger kind、基础字段、展示层、policy 去重与 handoff 模块拆分。D1-D6 已继续补齐 post-merge sanity、`batchId` 批次摘要、intervention/rollback 展示闭环、Go gate dry-run 预览、mock/非敏感 case 的 candidate → verification → decision → batch → intervention/rollback → handoff 闭环 smoke，以及 `ida-agent-bridge` 只读 index packet contract。

D 系列 D1-D6 已完成：ledger 已具备 batch / intervention / rollback / gate dry-run 的闭环验证，`ida-agent-bridge` 也已补只读 index adapter contract / recipe / capability card。后续若继续推进，应基于多个真实 case 的反馈再考虑 adapter 实现或受控执行闭环；当前仍不要求安装或连接 IDA，也不让 runtime 直接控制 IDA。

### 执行清单

- [x] D1：post-merge sanity + release hygiene
- [x] D2：batch 模型最小实现（`batchId` + batch 摘要 + rollback 引用）
- [x] D3：intervention / rollback 展示闭环（overview/handoff/note-List）
- [x] D4：heavy-tool gate runtime 强制化 dry-run 方案（Go backend 非写入 plan）
- [x] D5：真实 case dry-run 试用（mock/非敏感 case）
- [x] D6：IDA bridge adapter 预研（只读 packet contract，不接 runtime 强依赖）

### 验证标准

D1：

1. `git status --short` 只剩明确无关的本地未跟踪/未提交文件，或完全干净。
2. `./rekit/rekit.ps1 -Command doctor` 通过。
3. 临时 case `doctor` 通过。
4. `go test ./...` 通过。
5. `git diff --check` 通过。
6. 确认 `B3.Handoff.ps1` UTF-8 BOM、dot-source 顺序、case shim thin boundary 不回归。

D2：

1. `note` 支持 `-BatchId`，event JSONL 非空时写入 `batchId`。
2. `continue` runId 可派生/关联 batchId，auto decision 写入 `batchId`。
3. rollback / intervention 可通过 `-TargetRef batch-...` 引用 batch。
4. overview 显示最近 batch 摘要（限 N，避免刷屏）。
5. 历史无 batchId 的 JSONL 仍可读。

D3：

1. overview 显示 recent intervention、recent rollback、unresolved intervention。
2. handoff 增加 intervention/rollback 摘要区段（无事件不显示）。
3. note -List 对 intervention/rollback 展示 target/action/approvedBy/scope/reason。
4. 不自动执行 heavy-tool，不迁移历史事件。

D4：

1. gate dry-run 能展示将记录什么、需要用户确认什么、scope 是什么。
2. full-trace/debug/inject/patch/dump/network 未确认时只生成 gate packet，不执行外部动作。
3. 用户确认只覆盖列明 scope，不能被"继续"扩大。

D5：

1. mock/非敏感 case 跑通 candidate → verification → decision → batch → intervention/rollback → handoff。
2. 临时 case 用完清理测试事件，不污染 kit 模板。

D6：

1. 只写 adapter contract / recipe / capability card，不要求安装或连接 IDA。
2. 定义只读 index packet（functions/strings/imports/xrefs/selected summary），不读全量大输出。
3. 不让 runtime 直接控制 IDA。

### 风险与注意事项

- **不要跳过 D1**：C 系列刚完成并已推 main，先做 post-merge sanity。
- **不要过早 SQLite**：JSONL 仍足够；只有查询复杂度压垮 runtime 时再考虑索引。
- **不要把 gate 登记误写成自动授权**：`pending-gate` / intervention 只是记录和提示，执行 heavy-tool 仍需用户确认。
- **IDA bridge adapter 不做硬依赖**：先 contract/recipe，后续多个 case 验证后再考虑 adapter。
- **schema 扩展仍需兼容**：D2 增 `batchId` 是新增字段，历史事件无该字段应正常展示。

### 6.D1：post-merge sanity + release hygiene

**目标：** 确认 main 上 C 系列提交后状态稳定，没有把无关本地文件纳入后续工作。

**切片：**

1. 检查 `git status --short`。
2. 运行 kit doctor、临时 case doctor、`go test ./...`、`git diff --check`。
3. 检查 `rekit/rekit.ps1` dot-source 顺序含 `B3.Handoff.ps1`。
4. 检查 `B3.Handoff.ps1` 是 UTF-8 with BOM（含中文，PS 5.1 需要）。
5. 检查 case shim 仍 thin，不复制 runtime 逻辑。
6. 回写 `docs/batch-plan.md`。

### 6.D2：batch 模型最小实现

**目标：** 让一轮自动整理、review 或人工决策可以用 `batchId` 串起来，并支持后续整体 rollback。

**切片：**

1. `Add-RekitFactEvent` / `Invoke-RekitNote` 支持 `-BatchId`。
2. `New-RekitDecision` 支持 batchId（从 event 或 runId 派生）。
3. `continue` auto 流程为本轮生成 stable `batchId`（例如 `batch-<runId>`），写入 decision/publication 等事件。
4. `overview` 增加最近 batch 摘要（latest N，按 batchId 聚合 kind/count/last time）。
5. rollback/intervention 可用 `-TargetRef batch-...` 指向 batch。

**不做：** 不迁移历史 JSONL；不引入 SQLite；不自动回滚文件。

### 6.D3：intervention / rollback 展示闭环

**目标：** 让 intervention/rollback 不只是可写，还能在 overview/handoff 中被新会话看见。

**切片：**

1. overview 增 recent intervention / recent rollback / unresolved intervention。
2. lane handoff 增 `## intervention` 与 `## rollback`（各 latest 5）。
3. note -List 对 rollback 显示 target/reason/status；对 intervention 显示 action/approvedBy/scope/target。

### 6.D4：heavy-tool gate runtime 强制化 dry-run 方案

**目标：** 先实现 runtime gate dry-run，不直接执行 heavy tool；按维护判断，避免继续扩大 PowerShell runtime，D4 改由 Go backend 承接确定性预览逻辑。

**结果：**

1. 新增 Go `internal/rekit/gate`，`go run ./cmd/rekit -- -Command gate -WhatIf ...` 输出非写入 JSON plan。
2. plan 明确 `isMutation=false`、`reviewRequired=true`、`requiresConfirmation=true`，并预览 ledger request：`kind=request`、`status=pending-gate`、`target`/`batchId`、gate action/scope/budget/triedLightSteps/stopConditions。
3. Go CLI 拒绝无 `-WhatIf` 的 gate 调用；校验 attached case 和 `.rekit/board.json` lane id。
4. D4 不写 JSONL、不执行 full-trace/debug/inject/patch/dump/network、不把 dry-run 视为自动授权。

**后续：** 用户确认后的 ledger 写入与执行闭环留给后续 Go 切片；PowerShell façade 暂不默认委托 Go。

### 6.D5：真实 case dry-run 试用

**目标：** 用 mock/非敏感 case 跑完整闭环。

**结果：** 已完成。新增 `rekit/tests/agent-team-d5-dryrun-smoke.ps1`，默认创建 `_template` 自包含临时 case，初始化 overview/start 后用 `/rekit note` 写入同一 `batchId` 下的 candidate、verification、decision、intervention 与 rollback；随后验证 `note -List` 文本/JSON、overview 最近 batch / intervention / rollback 展示、lane handoff 的 verification/decision/intervention/rollback 区段，以及 case doctor。

**边界：** 只写临时 case `.rekit/facts/**` 与 handoff；finally 清理临时 case。不写 kit 模板、pack source、promote candidates、tooling candidates、authority/confirmed，不执行 heavy-tool 或外部动作。

### 6.D6：IDA bridge adapter 预研

**目标：** 只定义只读 adapter contract，不接 runtime 强依赖。

**结果：** 已完成。新增 `packs/vmp-re/tooling/recipes/ida-agent-bridge-readonly.md`，定义 capability card、只读 packet schema、sidecar/evidence ref 规则、limits/truncation、失败结构和明确禁止项；`tooling/catalog.yml`、`tooling/README.md`、`ida-x64dbg-mcp.md` 已链接该 contract，`manifest.yml` 已纳入 tooling files。

**边界：** 保持 `ida-agent-bridge` 为 candidate tooling；不安装、不下载、不打开 IDA、不连接 bridge、不生成全量导出、不 rename/comment/patch、不调试/dump/network，不把完整 decompile/disasm/hexdump/trace 写入 Markdown 或模板。

## 7. 与现有文档的关系

- 本文件是 `docs/vision.md` Phase 5/6 与 `docs/orchestration-plan.md` 的执行细化，不替代它们。
- 契约定义仍在 `common/policies/agent-team.md`、`common/policies/subagents.md`、`packs/vmp-re/manifest.yml`。
- Go runtime 迁移按 `docs/go-runtime-migration.md` 推进；本计划 R4-R6 以 PowerShell runtime 为准，不强行接入 Go façade。
- 批次记录在 `docs/batch-plan.md`；本文件只承载 Agent Team rollout 这一条线。
