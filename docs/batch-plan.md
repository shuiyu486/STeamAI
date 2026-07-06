# Batch implementation plan

## 目的

本文件记录后续批次计划，避免上下文压缩后只依赖聊天历史。

## 已完成批次

### Batch 0：项目定位与维护入口

- 新增根目录 `CLAUDE.md`。
- README 顶部改为 RE Agent Team 框架定位。
- 新增 `docs/vision.md`。

### Batch 1：VMP Agent Team 与工具路由

- 新增 `packs/vmp-re/references/vmp-re/agent-driven-re.md`。
- `workflow-template.md` 增加轻到重路线。
- `toolchain-router.md` 增加重型工具升级门禁。
- tooling catalog / recipe 增加 `ida-agent-bridge` 候选。
- `manifest.yml` 将 `agent-driven-re.md` 加入 managed/promote files。

### Batch 2：通用契约文档

- 新增 `common/policies/agent-team.md`。
- 新增 `common/policies/tool-adapters.md`。
- 新增 `docs/pack-authoring.md`。
- 新增 `docs/evidence-ledger.md`。
- 新增 `docs/orchestration-plan.md`。

## 近期已完成批次与验证

### Batch 3：pack template 与多 pack 骨架

状态：已完成。

目标：降低新增 `unpack-pe`、`android-native`、`ollvm`、`generic-binary-re` 的成本。

已创建产物：

```text
docs/pack-authoring.md 增补 checklist
packs/_template/manifest.yml
packs/_template/CLAUDE.local.snippet.md
packs/_template/references/template/README.md
packs/_template/references/template/workflow-template.md
packs/_template/references/template/toolchain-router.md
packs/_template/tooling/catalog.yml
```

说明：已创建实际 `packs/_template` 目录。该目录的 manifest `name` 为 `_template`，文档中明确它只作为作者模板，不代表真实 case 领域 pack。

### Batch 4：runtime bugfix preparation

状态：已完成。

目标：单独处理既有 `rekit/lib/B3.Lane.ps1` PowerShell 解析问题。

结果：

- 根因是 Windows PowerShell 5.1 对 UTF-8 无 BOM 且含非 ASCII 字符的 `.ps1` 可能按 ANSI 解析，导致中文字符串 mojibake 并破坏语法。
- 已给含非 ASCII 的 PowerShell runtime 文件补 UTF-8 BOM：`B3.Auto.ps1`、`B3.Commands.ps1`、`B3.Lane.ps1`、`Promote.ps1`、`Review.ps1`、`Validate.ps1`。
- 修复未涉及 instance/state schema 迁移。

验证：

```powershell
.\rekit\rekit.ps1 status
.\rekit\rekit.ps1 doctor
```

### Batch 5：case-local smoke test

状态：已完成。

目标：用临时 case 验证 init/attach/sync/promote 边界。

结果：

- 使用临时目录验证 `init -> status -> doctor -> sync review -> promote review`。
- 使用临时目录验证 `attach -> status -> sync review -> sync -Apply -> doctor -> promote review`。
- 临时目录验证后删除，未使用真实样本、外部工具、trace、dump 或 case 私有产物。
- 发现并修复裸 `attach` 后 `sync` review 遇到空 `CLAUDE.local.md` host text 的参数绑定问题；修复为允许 `Get-RekitManagedBlockAppliedText` 接收空字符串，保持 review-first 边界不变。
- 复核后补齐 apply/review 一致性：当 `CLAUDE.local.md` 已存在但内容为空白时，`sync -Apply` 与 sync review 一样生成默认 `# Project Context` 包装。

验证：

```powershell
.\rekit\rekit.ps1 status
.\rekit\rekit.ps1 doctor
# 临时 case smoke test：init/attach/sync/promote 边界均通过
```

### Batch 6：doctor / manifest safety net

状态：已完成。

目标：先增强部署前验证网，避免后续 pack-neutral、review/apply 闭环和 ledger runtime 改动在弱校验下引入隐性回归。

结果：

- `doctor` 增加 manifest schema 安全检查：`promoteFiles` 必须是 `managedFiles` 子集、`managedBlock` 必须显式声明 `file/blockId/source`、`toolingCandidateSources` 必须显式声明、`syncPolicy` 只能使用当前 runtime 支持的策略值、`promoteDenyPatterns` 必须是有效 regex。
- 非 `vmp-re` pack 的 manifest 路径不允许声明 `vmp-re` 路径，避免新 pack 误用旧默认值。
- policy overlay 校验会检查 `extends` 是否指向已注册 common policy。
- case `doctor` 增加 thin shim 校验，要求 case-local `.claude/skills/rekit/SKILL.md` 与 canonical thin shim 模板一致。
- case `doctor` 增加 `.rekit/board.json`、lane `lane.json`、lane/workspace JSONL 的基础可解析性和路径边界校验。
- `packs/_template` 补齐 `policies/manifest.yml` 与 `policies/README.md`，并修正 deny pattern 反斜杠，使 `_template` 可以作为可验证的新 pack 起点。

验证：

```powershell
.\rekit\rekit.ps1 doctor
.\rekit\rekit.ps1 doctor -Pack _template
git diff --check
```

### Batch 7：pack-neutral 工作线基础

状态：已完成。

目标：将 B3 工作线默认主线、默认 start 类型、长期 handoff 路径、sync backup root、authority files 和 request 默认路由从 runtime 硬编码迁移到 manifest，避免后续多 pack 继承 `vmp-re` 语义。

结果：

- manifest 解析新增 `workstreamDefaults` 与 `authorityFiles`。
- `vmp-re` manifest 显式声明 `devirt-main`、`feature-analysis`、`references/vmp-re/task-handoff.md`、`references/vmp-re/.backup` 和 VMP confirmed CSV authority files，保持现有行为兼容。
- `_template` manifest 新增 pack-neutral `main` / `feature` laneTypes 和工作线默认值，可用于派生新 pack 时验证 `overview/start/continue/handoff` 不泄漏 VMP 路径。
- `B3.State` 默认 authority lane 改由 manifest 驱动。
- `B3.Commands` 的 `start`、`continue` 主线 handoff 提示、项目/工作线 handoff 引用改由 manifest 驱动。
- `B3.Auto` 的 authority allowlist 和 request 默认 target lane 改由 manifest 驱动。
- `sync` backup root 与 sync review backup preview 改由 manifest 驱动。
- `doctor` 校验新增工作线默认值、authorityFiles 和非 `vmp-re` pack 的 VMP 路径污染检查。

验证：

```powershell
.\rekit\rekit.ps1 doctor
.\rekit\rekit.ps1 doctor -Pack _template
# 临时 case smoke：_template init / overview / start / continue / handoff 不应出现 vmp-re/devirt-main
# vmp-re 临时 case smoke：init / overview / start / continue / handoff 行为保持兼容
git diff --check
```

### Batch 8：Go runtime migration plan

状态：已完成 G1 skeleton，并完成 G2 review-only plan skeleton（仍不默认接入 PowerShell façade）。

目标：在不破坏现有 `/rekit`、case-local thin shim、pack wrapper、旧 case metadata 和 review-first 边界的前提下，将复杂 deterministic runtime 逐步迁移到 Go backend。

方案：

- 采用 PowerShell façade + Go deterministic backend。
- `rekit/rekit.ps1` 继续作为公共 ABI 和 fallback；Go binary 缺失或命令未覆盖时继续走 PowerShell。
- G1 Go 实现只读能力：manifest parser、status、pack doctor/validate。
- G2 第一版实现 `sync/promote` review-only plan skeleton；review artifact 写入、写入命令、工作线、authority gate 分后续 G2.1-G6 迁移。
- 迁移计划详见 `docs/go-runtime-migration.md`。

G1/G2 验证：

```powershell
go test ./... # 覆盖 manifest parser、CLI parser、repo root discovery、doctor target guard 和 review-only guard
go vet ./...
go run ./cmd/rekit -- -Command status
go run ./cmd/rekit -- -Command doctor
go run ./cmd/rekit -- -Command doctor -Target .
go run ./cmd/rekit -- -Command doctor -Target .\does-not-exist # 预期报错，不得误报 pack validation ok
go run ./cmd/rekit -- -Command doctor -Pack _template
# 使用临时 case 验证 go sync/promote review-only plan；sync -Apply / promote -CreateCandidates 预期被拒绝
.\rekit\rekit.ps1 status
.\rekit\rekit.ps1 doctor
.\packs\vmp-re\scripts\validate.ps1
git diff --check
```

新会话接手：

- 先确认 `main` 包含提交 `4ae3f78 Add Go runtime skeleton`，并运行 `git status --short` 确认工作区干净。
- 必读 `docs/go-runtime-migration.md`、本节、`CHANGELOG.md`、`internal/rekit/**` 和 `rekit/rekit.ps1`。
- 当前 Go backend 仅手动运行；`rekit/rekit.ps1` 仍是公共入口和 canonical ABI，尚未默认委托 Go。
- G1 已实现 `status`、pack `doctor/validate`、manifest/policy validation 与 guard tests；case doctor 尚未实现，遇 case target 必须显式拒绝。
- G2 已实现 `sync/promote` review-only JSON plan skeleton；只输出非写入 JSON，不写 `.rekit/reviews/**`、case 或 pack，并拒绝 `sync -Apply`、`promote -Apply/-CreateCandidates/-WhatIf`。
- 下一步优先做 G2.1：补齐 review artifact 写入、bounded diff、sanitized preview、PowerShell review packet 字段 parity、临时 case fixture / golden tests。
- 仍不要默认启用 PowerShell façade 委托；等 G2.1 parity 与 smoke test 足够后，再设计显式开关。

### Batch 9：Agent Team rollout 计划落盘

状态：已完成（仅计划文档，未动 runtime）。

目标：把"Agent Team 契约已固化、运行编排层未落地"的真实状态评估与后续推进路线沉淀为可接手文档，避免上下文压缩后只留聊天历史。

结果：

- 新增 `docs/agent-team-rollout-plan.md`，记录已落地/未落地评估、选项 C 推进姿态、R0-R7 分批切片、R3 决策门与验证标准。
- README、CLAUDE.md、`docs/vision.md`（执行清单 + 候选清单）接入 rollout 计划入口。
- CHANGELOG 记录本次计划落盘。
- 未改 runtime、policy、manifest；未创建临时 case；未执行任何 heavy-tool 动作。

验证：

```powershell
.\rekit\rekit.ps1 status
.\rekit\rekit.ps1 doctor
git diff --check
```

新会话接手：

- 先读 `docs/agent-team-rollout-plan.md` 顶部读取指南、实施摘要、执行清单、验证标准。
- R0-R2 只压测契约，不写 runtime；R3 是决策门，需显式记录 ledger/dispatch 顺序理由。
- dry-run 不执行真实 heavy-tool 动作；临时 case 用完即删。

### Batch 10：Agent Team rollout R0 dry-run 脚手架

状态：已完成（仅临时 case + 脚本文档，未动 runtime/policy/manifest）。

目标：为 R1 契约压测准备可重复的端到端 dry-run 脚手架。

结果：

- 在 kit 仓库外创建临时 case `C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun`，用 `/rekit init -Pack vmp-re -ProjectName agent-team-dryrun` 初始化。
- 临时 case `status`/`doctor` 通过，`instance validation ok`。
- 新增 `docs/agent-team-dryrun-script.md`，定义 mock 对象、S1-S5 五步脚本（main 派 task → feature 产 evidence/candidate → reviewer 出 verdict → main 合并 + gate → handoff 还原）、字段缺口记录区与验证标准。
- 未执行任何 heavy-tool 动作；未写 authority CSV；未写 kit 模板；未改 runtime/policy/manifest。

验证：

```powershell
.\rekit\rekit.ps1 -Command init -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun' -Pack vmp-re -ProjectName agent-team-dryrun
.\rekit\rekit.ps1 -Command status   # 在临时 case 内
.\rekit\rekit.ps1 -Command doctor   # 在临时 case 内
git diff --check
```

新会话接手：

- R1 按 `docs/agent-team-dryrun-script.md` S1-S5 执行，把字段缺口填入该文件 §6。
- R1 不写 runtime；仅手动产 packet 文件到临时 case workspace。
- R2 按缺口回写 `common/policies/agent-team.md` 或 `subagents.md`，再回 R1 复跑。

### Batch 11：Agent Team rollout R1-R7

状态：已完成。

目标：执行端到端契约压测、回写缺口、按 R3 决策做最小 ledger runtime 切片，并把 heavy-tool gate 与 bounded dispatch 的边界判定写回文档。

结果：

- R1：在临时 case `agent-team-dryrun` 跑 S1-S5，产出 `dryrun-001.md`（task/evidence/candidate/review/decision 全 packet），暴露 6 类缺口（G1 handoff 不引用 workspace packet、G2 overview 不聚合手写 packet、G3 packet schema 字段缺失、G4 workspace 路径契约不一致、G5 defer 状态无标准位置、G6 lane id 规范化），记于 `docs/agent-team-dryrun-script.md` §6。
- R2：回写 `common/policies/agent-team.md`（evidence packet 补 `evidence_id`、新增 decision event 字段模板、packet-vs-event 关系、lane id 规范化）、`evidence.md`、`handoff.md`（workspace packet 引用）、`subagents.md`（L1 tool_scope 可省）。
- R3：决策 ledger runtime 优先；R4 后做 R5。
- R4：新增 `rekit/lib/B3.State.ps1` `Add-RekitFactEvent` + `Get-RekitFactFilePath`；新增内部 `note` 命令（`rekit/rekit.ps1` 分发 + `B3.Commands.ps1` `Invoke-RekitNote`），append-only、按 eventId 去重、路由到对应 facts JSONL；`overview` 共享事实计数现反映手写事件，pending 计数对齐 `decision=defer`/`status` 非终态；`Write-RekitLaneHandoff` 增加 workspace packet 引用区段。
- R5：判定不扩 runtime。spawn 子 agent 是主会话/Claude Code 职责；runtime 侧 `plan-subagents`（分片计划）+ `note`（verdict 写回）已足够。自动 spawn 属 Phase 6 后段。
- R6：heavy-tool gate 写入 `packs/vmp-re/policies/verification.overlay.md`，规定 full-trace/debug/inject/patch/dump/network 触发前用 `note -Kind request -Status pending-gate` 记录 gate 事件，用户确认后执行，执行后用 `note -Kind observation` 记录结果。runtime 不强制 gate。
- R7：自审 diff，回写本节、CHANGELOG、`docs/agent-team-rollout-plan.md` 执行清单。
- 顺带修复 `B3.Core.ps1` `Join-RekitRelativePath` 在 Windows PowerShell 5.1 下崩溃的既有 bug（`[System.IO.Path]::GetRelativePath` 在 .NET Framework 不存在）。

验证：

```powershell
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun'
# 临时 case note smoke：
.\rekit\rekit.ps1 -Command note -Target <tmpCase> -Pack vmp-re -Kind observation -Lane feature-handler-0x40a010 -Subject ... -Summary ... -Confidence low
.\rekit\rekit.ps1 -Command overview -Target <tmpCase>   # observation/candidate/需要确认 计数应反映 note 事件
.\rekit\rekit.ps1 -Command handoff -Target <tmpCase> handler-0x40a010  # handoff 应含 workspace packet 区段
go test ./...
git diff --check
```

新会话接手：

- Agent Team rollout 已完成 R0-R7；后续若要做 Phase 6 自动 spawn reviewer，先在多个真实 case 验证 `note` + `plan-subagents` 闭环。
- `note` 是内部命令，不向用户展示；SKILL.md 未列入日常入口。
- heavy-tool gate 当前是 agent 行为契约，runtime 不强制；Phase 6 才考虑 runtime 强制确认。
- 临时 case `C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun` 可保留供后续验证，或用 `Remove-Item -Recurse -Force` 清理。

### Batch 12：Kit runtime hardening 计划落盘

状态：已完成（仅计划文档，未动 runtime）。

目标：把 R0-R7 后续 kit runtime 强化的 B1-B3 切片沉淀为可接手文档。

结果：

- 在 `docs/agent-team-rollout-plan.md` 追加 §4 Kit runtime hardening，含 B1 `note` schema 校验、B2 overview 未决/pending/冲突明细、B3 handoff 引用 decision event 三批切片，每批含改动文件、验证标准。
- 不碰 bounded dispatch 自动 spawn 与 Go 迁移（独立线）。
- 未改 runtime/policy/manifest；未创建新临时 case。

验证：

```powershell
.\rekit\rekit.ps1 status
.\rekit\rekit.ps1 doctor
git diff --check
```

新会话接手：

- 先读 `docs/agent-team-rollout-plan.md` §4 顶部读取指南、实施摘要、执行清单、验证标准。
- B1-B3 沿用临时 case `agent-team-dryrun` 验证。
- 每批完成后回写本节、CHANGELOG、rollout plan §4 执行清单。

### Batch 13：Kit runtime hardening B1-B3

状态：已完成。

目标：把 ledger 从"可写"推进到"可查可聚合"，补 R0-R7 留的三个缺口（note 无校验、overview 只计数、handoff 不读 decision）。

结果：

- B1：`rekit/lib/B3.State.ps1` `Add-RekitFactEvent` 加 schema 校验：`Confidence` ∈ low/medium/high、`Decision` ∈ confirm/reject/defer/supersede（仅 kind=decision）、`Status` ∈ open/deferred/pending-gate/confirmed/rejected/superseded/needs_more_evidence、`EvidenceRefs` 每项非空、`Lane` 存在性（Board 传入时，兼容 `IDictionary` 与 PSObject）。`Invoke-RekitNote` 传 Board 给 `Add-RekitFactEvent`。非法输入 throw 明确错误，不写入。
- B2：`rekit/lib/B3.Commands.ps1` `Show-RekitOverview` 在"共享事实"计数后增加明细区段："未决 candidate"（status 非终态，latest N=10，同 subject 多 open 标 `[冲突]`）、"pending-gate"（status=pending-gate 的 request，latest N=10），超出提示"另有 N 条"。计数区段保留。
- B3：`rekit/lib/B3.Commands.ps1` `Write-RekitLaneHandoff` 在 workspace packet 区段后增加 `## decision` 区段，读 `decisions.jsonl` 过滤本 lane，取 latest 5 条列 `subject + decision + reason`。无 decision 时不出现空区段。
- 自审：`Add-RekitFactEvent` 仅被 `Invoke-RekitNote` 调用（auto 流程不调），lane 校验总生效；校验逻辑集中在单一函数；overview 明细限 N=10、handoff decision 限 N=5。

验证：

```powershell
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun'
# B1：note -Confidence garbage / -Lane no-such-lane 被拒；合法值写入
# B2：overview 显示未决 candidate + [冲突] + pending-gate 明细
# B3：handoff <lane> 含 ## decision 区段
go test ./...
git diff --check
```

新会话接手：

- B1-B3 已完成并通过临时 case 验证。
- 后续若做 Phase 6 自动 spawn reviewer 或 heavy-tool gate runtime 强制，先在多个真实 case 验证 `note` + `overview` + `handoff` 闭环。
- 临时 case `agent-team-dryrun` 保留供后续验证。

### Batch 15：Ledger schema 对齐计划落盘

状态：已完成（仅计划文档，未动 runtime）。

目标：审查发现 runtime ledger 字段偏离 `docs/evidence-ledger.md` 草案（自定 decision 枚举、缺字段、缺 4 种 kind、auto 用 action 字段），把对齐方案沉淀为可接手文档。

结果：

- 在 `docs/agent-team-rollout-plan.md` 追加 §5 Ledger schema 对齐与架构治理（C 系列），含偏差清单、C1-C8 八批切片、每批验证标准与 schema 判断。
- C1：docs 冲突修正 + SKILL.md 补 note。
- C2：`Add-RekitFactEvent` 基础字段对齐草案（补 actor/risk/related，decision 枚举改 accept|reject|defer|supersede，status 枚举对齐）。
- C3：补 4 种 kind（hypothesis/verification/intervention/rollback）+ 扩展字段。
- C4：auto 流程 `New-RekitDecision` 改用草案 decision 字段 + confirmedBy/writes，6 处 Add-RekitJsonLine 改走 Add-RekitFactEvent（统一写入路径）。
- C5：overview/handoff/note-List 展示对齐新字段。
- C6：policy 文档去重（evidence.md 降级、handoff.md 补 decision、tool-output.md 并入 tool-adapters.md）。
- C7：handoff 子系统独立成 B3.Handoff.ps1。
- C8：自审 + 回写。
- schema 改动遵循"兼容旧数据、不破坏现有 case、每次只改一个 schema 点"原则；C4 涉及 auto 流程逻辑改动，回归风险中等，做完先 smoke 再决定是否停下。
- 未改 runtime/policy/manifest；未创建新临时 case。

验证：

```powershell
.\rekit\rekit.ps1 status
.\rekit\rekit.ps1 doctor
git diff --check
```

新会话接手：

- 先读 `docs/agent-team-rollout-plan.md` §5 顶部读取指南、偏差清单、执行清单。
- C1-C8 沿用临时 case `agent-team-dryrun` 验证。
- 每批 schema 改动由维护者判断是否需停下问用户；遇真正破坏兼容性或难判架构取舍时停下。
- 对齐 `docs/evidence-ledger.md` 草案，不另定 schema。

状态：已完成。

目标：补 B1-B3 的对称缺口（handoff 没补 pending-gate、overview 没补 decision 明细）+ ledger 查询能力（`note -List`），并修正 auto 流程与 `note` 的 decision 字段不一致问题。

结果：

- B5：`Write-RekitLaneHandoff` 在 `## decision` 区段后增加 `## pending-gate` 区段（本 lane latest 5 条 pending-gate request）；decision 显示兼容 `action` 字段（auto `continue` 用 `action`，`note` 用 `decision`），`subject` 缺失时 fallback 到 `kind`。
- B6：`Show-RekitOverview` 在"pending-gate"区段后增加"最近 decision"明细区段（latest N=10），兼容 `action` 字段，显示 `subject + lane + decision + reason`。
- B7：`Invoke-RekitNote` 加 `-List` 开关：按 `Kind`（空则全部 5 类）+ `Lane`（可选）过滤，每类限 N=20，超出提示"另有 N 条"；candidate 显示 confidence/status，decision 显示 decision/action，request 显示 status。
- 发现并修正隐藏字段不一致：`Add-RekitFactEvent` 用 `decision` 字段，`New-RekitDecision`（auto 流程）用 `action` 字段，B5/B6 兼容两者，避免 auto 写的 decision 在 handoff/overview 显示为空。

验证：

```powershell
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun'
# B5：handoff <lane> 含 ## decision + ## pending-gate
# B6：overview 含"最近 decision"区段
# B7：note -List / note -List -Kind decision / note -List -Kind candidate -Lane <id>
go test ./...
git diff --check
```

新会话接手：

- B1-B7 已完成，ledger 从"可写"推进到"可查可聚合 + 可查询"。
- auto `continue` 与 `note` 的 decision 字段不一致仍存在（`action` vs `decision`），B5/B6 在显示层兼容；若后续要统一字段名，需改 `New-RekitDecision` 并迁移历史 JSONL，属 schema 迁移，需停下询问用户。
- 临时 case `agent-team-dryrun` 保留供后续验证。

### Batch 16：C1 docs 冲突修正 + SKILL.md 补 note

状态：已完成。

目标：消除 docs 间滞后/冲突，让 agent 能发现 `note` 命令；C 系列纯文档首切，最低风险。

结果：

- `docs/orchestration-plan.md` O2 段加状态注：bounded dispatch 已被 `agent-team-rollout-plan.md` R5 否决（spawn 是主会话职责），本节保留作 Phase 6 后段设计参考。
- `docs/vision.md` §8 候选清单第 7 条更新为"`packs/_template/` 与 `B3.Lane` 修复已完成；后续按 batch-plan 与 rollout §4-§5 推进"。
- `docs/agent-team-rollout-plan.md` §"当前状态"段同步 R0-R7 已完成（`note`+`Add-RekitFactEvent`+overview+handoff 已落地），后续按 §4/§5 推进。
- `.claude/skills/rekit/SKILL.md` 命令清单补 `note`（含 `-List` 查询），标注为账本写入/查询入口、heavy-tool gate 登记、schema 校验。
- 未动 runtime/policy/manifest；未创建新临时 case。

验证：

```powershell
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

新会话接手：

- C1 已完成，下一步 C2：`Add-RekitFactEvent` 基础字段对齐草案（补 actor/risk/related，decision 枚举改 accept|reject|defer|supersede，status 枚举对齐）。涉及 schema 改动，读层兼容旧值、历史数据不迁移。
- C 系列沿用临时 case `agent-team-dryrun` 验证。
- 每批 schema 改动由维护者判断是否需停下问用户。

### Batch 17：C2 Add-RekitFactEvent 基础字段对齐草案

状态：已完成。

目标：让 `note` 写入的字段集和枚举对齐 `docs/evidence-ledger.md` 基础字段，消除 runtime 自定 decision 枚举（`confirm` vs 草案 `accept`）的漂移。

结果：

- `Add-RekitFactEvent`（`B3.State.ps1`）：补 `-Actor`/`-Risk`/`-Related` 参数；event hashtable 补 `schemaVersion: 1`、`actor`、`risk`、`related`（Related 用 `Split-RekitScalarList` 拆成数组）。
- `validDecision` 改 `accept|reject|defer|supersede`（`confirm` 写入被拒）。
- `validStatus` 取并集：草案 6 值（`open|accepted|rejected|superseded|resolved|deferred`）∪ runtime 已落地 3 值（`pending-gate|confirmed|needs_more_evidence`）。**保留 runtime 3 值**：R6 heavy-tool gate 契约（`packs/vmp-re/policies/verification.overlay.md` 用 `note -Kind request -Status pending-gate` 登记 gate、`overview`/`handoff` 读层过滤 `pending-gate`）依赖这三个值，移除会破坏现有 case 与 overlay。
- `Invoke-RekitNote`（`B3.Commands.ps1`）：param 块补 `-Actor`/`-Risk`/`-Related` 并透传 `Add-RekitFactEvent`；`rekit.ps1` note 分发是泛型 `-Name Value` 解析，新参数自动透传无需改。
- `Show-RekitOverview`：`terminalStatus` 补 `accepted`/`resolved`，与新枚举一致；`pending-gate` 读层过滤保留。
- 用户决策：status 枚举对齐方式选"并集接受"（保留 pending-gate 等），不选"严格对齐草案"（会破坏 R6 契约）也不选"暂不动 status"（C2 收益不足）。

验证：

```powershell
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun'
# note -Kind decision -Decision accept 写入成功；-Decision confirm 被拒
# note -Kind candidate -Status accepted 写入成功；-Status pending-gate 仍可写入
# 写入的 JSONL 含 schemaVersion:1 / actor / risk 字段
# 历史 JSONL 仍可读（overview 不崩）
git diff --check
```

新会话接手：

- C2 已完成。下一步 C3：补 4 种事件类型 hypothesis/verification/intervention/rollback 的 ValidateSet + 文件路由 + 扩展字段。
- 临时 case `agent-team-dryrun` 保留；C2 测试事件已清理。
- 读层兼容历史 `confirm`/`action`/`pending-gate` 值；append-only 不迁移旧事件。

### Batch 18：C3 补 4 种事件类型

状态：已完成。

目标：让 `note` 支持草案定义的全部 9 种 kind，runtime ledger kind 集对齐 `docs/evidence-ledger.md`。

结果：

- `Get-RekitBoardPaths`（`B3.State.ps1`）补 4 路径属性：`Hypotheses`/`Verifications`/`Interventions`/`Rollbacks`（`facts\hypotheses.jsonl` 等）。
- `Ensure-RekitBoard` 初始化 4 个新空 JSONL；`Validate.ps1` `Test-RekitWorkstreamState` 校验循环补 4 文件；`B3.Auto.ps1` `Get-RekitKnownEventIds` 去重循环补 4 文件（auto 流程暂不向新 kind 写事件，C4 才动 auto）。
- `Get-RekitFactFilePath` 与 `Add-RekitFactEvent` ValidateSet 扩 4 kind。
- verification kind 扩展字段与校验：`-TargetRef`（指向 cand-id，写入字段名 `target`）、`-Verifier`（manual-review|schema-check|focused-trace|parity|cross-run|tool-review）、`-Verdict`（accepted|rejected|inconclusive|needs-more-evidence）。
- intervention kind 扩展字段与校验：`-Action`（override|rollback|heavy-tool-approval|schema-migration|external-side-effect）、`-ApprovedBy`、`-Scope`、`-Expires`。
- hypothesis/rollback 沿用基础字段（无额外扩展字段校验）。
- `Invoke-RekitNote` param 补扩展字段（`TargetRef` 避免与已有 `$Target` 冲突）并透传；`note -List` 展示 verification（verifier/verdict/target）、intervention（action/approvedBy/scope）、candidate（risk）、decision（confirmedBy）字段；错误提示 kind 列表更新为 9 种。
- 命名冲突处理：`Invoke-RekitNote` 已有 `$Target`（case 路径）参数，verification 指向 cand-id 的参数命名 `$TargetRef`，透传给 `Add-RekitFactEvent` 的 `-Target`（写入 event['target']）。

验证：

```powershell
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun'
# note -Kind hypothesis/verification/intervention/rollback 写入成功
# note -Kind verification -Verdict garbage / -Kind intervention -Action hack 被拒
# verification jsonl 含 target/verifier/verdict；intervention jsonl 含 action/approvedBy/scope
# note -List -Kind verification 展示 verifier/verdict/target
git diff --check
```

新会话接手：

- C3 已完成。下一步 C4：auto 流程 `New-RekitDecision` 改用草案 decision 字段（`action`→`decision`）+ `confirmedBy`/`writes`，6 处 `Add-RekitJsonLine` 改走 `Add-RekitFactEvent`。涉及 auto 流程逻辑改动，回归风险中等，做完先 smoke 再决定是否停下。
- 临时 case `agent-team-dryrun` 保留；C3 测试事件已清理（4 新文件为空）。
- schema 扩展，不影响现有 5 种 kind 读写。

### Batch 19：C4 auto 流程 decision 字段对齐

状态：已完成。

目标：消除 `action` vs `decision` 字段分裂，auto `continue` 流程写的 decision event 对齐 `docs/evidence-ledger.md` 草案 `decision` 字段 + `confirmedBy`/`writes`。

结果：

- `New-RekitDecision`（`B3.Auto.ps1`）改字段：
  - `action` → `decision`，值映射：`auto-publish`/`auto-route`/`auto-apply-authority`/`auto-accept-shared`→`accept`；`pending-user`/`defer`→`defer`。
  - 补 `schemaVersion:1`、`kind:'decision'`（原用原 event kind，现固定 decision）、`confirmedBy:'runtime'`、`subject`、`summary`。
  - authority 写入时附 `writes`（`@{file;backup;diff}`），非 authority 场景 `Extra` 仍走 `extra` 字段。
- 9 处 `Add-RekitJsonLine -Path $paths.Decisions` 调用**保留直接写**：原方案"改走 `Add-RekitFactEvent` 统一写入路径"因 `Add-RekitFactEvent` 的 eventId 去重会破坏 auto 流程对同一 eventId 写两条记录（原 event 写 kind 文件 + decision 写 decisions.jsonl）的 by design 行为。C4 核心目标（消除字段分裂）通过 `New-RekitDecision` 字段对齐实现，写入路径不动。
- 读层（overview `最近 decision`/handoff `## decision`/note-List）已兼容 `action`/`decision` 双字段（B5/B6 实现 `$dec = $it.decision; if empty $dec = $it.action`），新写 `decision` 字段直接可读。
- 方案文档同步：`rollout-plan.md` §5.C4 切片 4 改注"保留直接写"及理由。

验证：

```powershell
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun'
# continue feature 触发 auto 流程后，新 decision event 含 decision/confirmedBy/schemaVersion，无 action 字段
# 历史 action 字段 decision 仍可读（overview 最近 decision 兼容展示）
git diff --check
```

新会话接手：

- C4 已完成，回归风险中等但 smoke 通过。下一步 C5：overview/handoff/note-List 展示对齐新字段（actor/risk/confirmedBy/verifier/verdict 等）。
- 临时 case `agent-team-dryrun` 保留；C4 测试事件已清理。
- 读层兼容历史 `action`/`confirm`/`pending-gate` 值；append-only 不迁移旧事件。

### Batch 20：C5 展示层对齐新字段

状态：已完成。

目标：overview/handoff/note-List 展示草案新字段（actor/risk/confirmedBy/verifier/verdict 等），让 agent 从展示看到决策来源。

结果：

- `Show-RekitOverview` "最近 decision"：显示 `by=<actor|confirmedBy>`（actor 优先，fallback confirmedBy），无则省略。
- `Write-RekitLaneHandoff` `## decision` 区段：显示 `by=<confirmedBy|actor>`（confirmedBy 优先，fallback actor）。
- `note -List` decision 展示：从 `confirmedBy=` 改为 `by=` 并 fallback `actor`（`note` 写的 decision 用 `-Actor` 字段，auto 写的用 `confirmedBy` 字段，统一展示为 `by=`）。
- verification/intervention/candidate-risk 字段展示已在 C3 落地，C5 不重复。
- 历史无 actor/confirmedBy 的事件 `by=` 为空（append-only 不回填）。

验证：

```powershell
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun'
# note -Kind decision -Actor main 写入后，overview/handoff/note -List 三处显示 by=main
git diff --check
```

新会话接手：

- C5 已完成。下一步 C6：policy 文档去重（evidence.md 降级、handoff.md 补 decision、tool-output.md 并入 tool-adapters.md）。
- 临时 case `agent-team-dryrun` 保留；C5 测试事件已清理。
- 纯展示层改动，回归风险低。

### Batch 21：C6 policy 文档去重

状态：已完成。

目标：消除 policy 7 处重叠，建立 single source of truth，让 agent 读到一致的契约。

结果：

- `evidence.md` 改为 `agent-team.md` 的证据原则补充：顶部声明不重复定义 packet schema，packet/event 字段指向 `agent-team.md` 与 `docs/evidence-ledger.md`；"packet 与 event"段更新为 `/rekit note` 手动入口 + `/rekit continue` auto 入口 + 9 种 kind 对齐草案。
- `handoff.md` 补 `## decision event 引用` 小节：指向 `evidence-ledger.md` decision schema，`by` 字段 confirmedBy 优先 fallback actor，`## pending-gate` 区段列本 lane pending-gate request。
- `tool-output.md` 内容并入 `tool-adapters.md` "输出契约"小节（含报告格式模板 + "能用脚本统计不手工复制长表"规则）；`tool-output.md` 改为 deprecated 指向，避免外部引用断链；`policies/manifest.yml` `tool-output` title 标 "deprecated; merged into tool-adapters"，保留注册（sync 仍 reference，validate 仍 true，确保 doctor 校验通过）。
- `subagents.md` 补 overlay 可 extend `decision` 枚举说明（通用 `accept|reject|defer|needs_l2|needs_l3`，overlay 可追加领域值但需显式声明）+ ledger decision（`accept|reject|defer|supersede`）与 reviewer output contract（`needs_l2|needs_l3`）是不同层的澄清。
- `review-first.md`/`write-boundaries.md` 顶部互引：review-first 偏流程（先 review 再 write、确认语义），write-boundaries 偏边界（什么能写、子 agent 默认只读），两者配合。

验证：

```powershell
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun'
git diff --check
```

新会话接手：

- C6 已完成。下一步 C7：handoff 子系统独立成 `B3.Handoff.ps1`（机械移动 4 函数，dot-source 顺序注意依赖）。
- 临时 case `agent-team-dryrun` 保留。
- 纯文档去重，未动 runtime；policy registry 校验通过。

### Batch 22：C7 handoff 子系统独立

状态：已完成。

目标：`B3.Commands.ps1` 降到 ~339 行，handoff 边界更清晰，降低后续 AI 维护"牵一发动全身"风险。

结果：

- 新建 `rekit/lib/B3.Handoff.ps1`，含 `Get-RekitLatestRunDigestPath`、`Write-RekitProjectHandoff`、`Write-RekitLaneHandoff`、`Write-RekitHandoff` 4 函数（从 `B3.Commands.ps1` 行 254-448 机械移动，逻辑不改）。
- `rekit/rekit.ps1` dot-source `B3.Handoff.ps1`（在 `B3.Auto.ps1` 后、`B3.Commands.ps1` 前）。
- `B3.Commands.ps1` 从 534 行降到 339 行，聚焦 `Invoke-RekitContinue`/`Invoke-RekitStart`/`Invoke-RekitNote`/`Show-RekitOverview` 等用户级命令入口。
- 编码：`B3.Handoff.ps1` 含中文，用 UTF-8 with BOM 写入（PS 5.1 解析非 ASCII 需 BOM，与既有 `B3.Commands.ps1`/`B3.Auto.ps1` 一致；`B3.Core.ps1`/`B3.State.ps1` 纯 ASCII 无 BOM）。首次用 Write 工具写无 BOM 导致 PS 5.1 mojibake 解析失败，已用 `[System.Text.UTF8Encoding]::new($true)` 重写修复。
- handoff 命令行为不变（`handoff main` 生成工作线文档、无参 `handoff` 生成项目级索引）。

验证：

```powershell
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun'
# handoff main 生成 .rekit/handovers/devirt-main-latest.md，含 ## 新会话开场/## 推荐读取/## 边界
# handoff（无参）生成 .rekit/handovers/latest.md
git diff --check
```

新会话接手：

- C7 已完成。下一步 C8：自审 + CHANGELOG + batch-plan 回写 + reference-absorption 候选清单勾选。
- 临时 case `agent-team-dryrun` 保留。
- 机械移动，回归风险低；编码坑（UTF-8 BOM）已记录。

### Batch 23：C8 自审 + reference-absorption 回写（C 系列收尾）

状态：已完成。

目标：C 系列收尾，回写 `docs/reference-absorption.md` 候选清单，自审 C1-C8。

结果：

- `docs/reference-absorption.md`：
  - 行 37 候选清单勾选 `[x]` evidence ledger runtime append-only JSONL，附注落地位置（`/rekit note` + `Add-RekitFactEvent` 9 种 kind + 读层 + auto decision 对齐）。
  - 行 38/40 heavy-tool gate runtime、bounded dispatch 加注当前形态（R6 overlay + `note -Kind request -Status pending-gate`；R5 判定 runtime 不自动 spawn）。
  - "当前已落地/尚未落地"段：ledger runtime 移入已落地，尚未落地改为 runtime 强制 gate/IDA adapter/自动 dispatch/脱壳引擎。
  - clark-utov 映射表：落地位置补 `B3.State.ps1`/`B3.Auto.ps1`，状态改"设计契约已落地；runtime ledger 9 种 kind + decision 字段已对齐草案，batch 模型与 intervention 强制门禁待实现"。
  - 能力表：evidence/intervention ledger 草案行补 runtime 落地位置与字段对齐说明。
  - "还没落地"清单：append-only ledger runtime 改"已落地，batch 模型尚未实现"；intervention/rollback 改"可手动 note 写入，auto 流程暂不自动写"。
  - ledger 最小实现注改"已完成"。
- C 系列自审：
  - schema 每批单点改：C2 基础字段+枚举、C3 加 4 kind、C4 auto decision 字段，每批验证后进下一批。
  - 读层兼容历史 `action`/`confirm`/`pending-gate`，append-only 不迁移旧事件。
  - policy single source of truth：evidence.md 降级、tool-output.md 并入 tool-adapters.md、互引。
  - handoff 子系统独立，B3.Commands.ps1 534→339 行。
  - 全程未破坏现有 case，临时 case `agent-team-dryrun` 每批清理测试事件。
  - 1 次用户决策（C2 status 枚举并集接受）、1 次方案调整（C4 保留 Add-RekitJsonLine 直写，原因 eventId 去重）、1 次编码坑（C7 UTF-8 BOM），均已记录。

验证：

```powershell
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun'
go test ./...
git diff --check
```

新会话接手：

- C 系列（C1-C8）全部完成。Agent Team 从"契约完整、编排未落地、ledger 断的"推进到"ledger runtime 9 种 kind 对齐草案、读层可查可聚合、policy single source of truth、handoff 子系统独立"。
- 后续方向（不在 C 系列范围）：batch 模型（`batchId`/整体接受/回滚）、intervention runtime 强制门禁、runtime 强制 heavy-tool gate、IDA bridge adapter、多 pack 扩展。按 `docs/vision.md` Phase 5/6 与 `docs/agent-team-rollout-plan.md` §4-§5 后续推进。
- 临时 case `agent-team-dryrun` 保留供后续验证。
