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
- G2/G2.1 已实现 `sync/promote` review-only JSON plan 与 review artifacts：`packet.json`、`summary.md`、bounded diff、sanitized preview；仍拒绝 `sync -Apply`、`promote -Apply/-CreateCandidates/-WhatIf`。
- G2.2/G2.3 已实现 Go gate preview 与 pending-gate request 写入；`gate -Apply` 只 append request JSONL，要求 `-Actor`，不执行 heavy-tool。
- 仍不要默认启用 PowerShell façade 委托；下一步优先评估显式开关、case doctor parity 或 gate request schema parity。

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

### Batch 24：D 系列后续方案落盘

状态：已完成（仅计划文档，未动 runtime）。

目标：承接 C 系列成果，给出后续实施方案，先把 ledger 从"事件账本已对齐草案"推进到"批次、干预、回滚、门禁可闭环"，再考虑 adapter/dispatch。

结果：

- `docs/agent-team-rollout-plan.md` 新增 §6 D 系列：batch / intervention / gate 闭环。
- 推荐顺序：D1 post-merge sanity → D2 batch 模型最小实现 → D3 intervention/rollback 展示闭环 → D4 heavy-tool gate runtime 强制化 dry-run → D5 mock/非敏感 case 试用 → D6 IDA bridge adapter 预研。
- 明确不做：不跳过 D1、不提前 SQLite、不把 gate 登记误当自动授权、IDA bridge 不做硬依赖、不自动 spawn reviewer。
- D2 schema 扩展原则：新增 `batchId` 字段，历史事件无该字段仍可读；不迁移 JSONL。

验证：

```powershell
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun'
go test ./...
git diff --check
```

新会话接手：

- 下一步执行 D1：post-merge sanity + release hygiene。
- D1 只做自检和回写，不动 runtime。
- 若 D1 通过，再进入 D2 batch 模型最小实现。

### Batch 25：D1 post-merge sanity + release hygiene

状态：已完成（仅自检与文档回写，未动 runtime）。

目标：确认 main 上 C 系列提交后状态稳定，D 系列实施前安全网正常。

结果：

- `git status --short` 只显示本批 D 系列计划文档改动（`docs/agent-team-rollout-plan.md`、`docs/batch-plan.md`）。
- `B3.Handoff.ps1` UTF-8 BOM 检查通过（含中文，PS 5.1 解析需要 BOM）。
- `rekit/rekit.ps1` dot-source 顺序确认：Core → State → Policy → Lane → Auto → Handoff → Commands。
- case shim thin boundary 确认：`rekit/templates/case-shim/SKILL.md` 不引用 B3 模块、不复制 runtime 逻辑。
- D1 执行清单已在 `docs/agent-team-rollout-plan.md` §6 勾选。

验证：

```powershell
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun'
go test ./...
git diff --check
```

验证结果：全部通过。

新会话接手：

- 下一步 D2：batch 模型最小实现。
- D2 涉及 schema 扩展（新增 `batchId` 字段），原则：兼容旧 JSONL、历史事件不迁移、读层无 batchId 时正常展示。

### Batch 26：D2 batch 模型最小实现

状态：已完成。

目标：给 ledger event 增加最小 `batchId` 关联能力，让一次 `/rekit continue` 或一组手动 note 可在 overview 中被聚合，并为后续 rollback/intervention 闭环提供引用锚点。

结果：

- `Add-RekitFactEvent` 增加 `-BatchId` 参数；非空时写入 event `batchId` 字段。历史 JSONL 无该字段仍可读，不迁移旧事件。
- `Invoke-RekitNote` 增加 `-BatchId` 参数并透传；`note -List` 对含 `batchId` 的事件追加 `batch=<id>` 展示。
- `Invoke-RekitAuto` 为每次 `continue` run 派生 `batchId = batch-<runId>`，并写入自动收集的 event、publication object、decision event 与 run digest。
- `New-RekitDecision` 增加 `-BatchId` 参数，未显式传入时从原 event 的 `batchId` 继承。
- `overview` 增加“最近 batch”区段，跨 9 种 fact JSONL 聚合含 `batchId` 的事件，显示 event 数、kind 分布与最后时间。
- rollback/intervention 仍沿用 `TargetRef` 指向目标；D2 smoke 验证可用 `TargetRef batch-...` 指向整个 batch。
- D2 测试事件、feature outbox 测试输入和测试 run 目录已清理，未把 smoke 数据留在临时 case。

验证：

```powershell
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun'
go test ./...
git diff --check
```

验证结果：全部通过。

新会话接手：

- 下一步 D3：intervention / rollback 展示闭环。
- D3 优先做读层与手动闭环：overview/handoff/note-List 能从 batch、target、action、approvedBy 还原“发生了什么干预、影响哪个 batch、如何回滚”。
- 仍不要把 `pending-gate` 或 intervention 登记误当成自动授权；confirmed/authority 写入继续人工确认。

### Batch 27：D3 intervention / rollback 展示闭环

状态：已完成。

目标：让 intervention/rollback 不只是可写入 JSONL，还能被 overview、handoff 和 note 查询读出，方便新会话判断“发生过什么干预、影响哪个 batch、是否已回滚”。

结果：

- `overview` 增加三个区段：
  - `未解决 intervention`：过滤 status 非终态或空 status 的 intervention，展示 action、target、status、batch 与 summary。
  - `最近 intervention`：展示 action、target、approvedBy、scope、batch。
  - `最近 rollback`：展示 target、status、batch、reason。
- `Write-RekitLaneHandoff` 增加 `## intervention` 与 `## rollback` 区段（各 latest 5，仅本 lane，有事件才显示）。
- `note -List`：
  - intervention 展示 action、target、approvedBy、scope、status、reason、batch。
  - rollback 展示 target、status、reason、batch。
- D3 只增强读层，不自动执行 heavy-tool、不自动回滚文件、不迁移历史 JSONL。
- D3 smoke 写入的 intervention/rollback 测试事件、临时 handoff artifact 已清理；latest handoff 已重新生成，确认无 `d3-*` / `batch-d3-smoke` 残留。

验证：

```powershell
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun'
go test ./...
git diff --check
```

验证结果：全部通过。

新会话接手：

- 下一步 D4：heavy-tool gate runtime 强制化 dry-run 方案。
- D4 仍只做 dry-run / review-first gate 设计与最小 runtime 入口，不执行 full-trace/debug/inject/patch/dump/network。
- 用户确认只覆盖列明 scope，不能被“继续”扩大。

### Batch 28：D4 Go gate dry-run 最小切片

状态：已完成。

目标：按“Go 更适合 deterministic runtime”的方向调整 D4，停止继续扩大 PowerShell runtime，把 heavy-tool gate 的非写入预览逻辑落到 Go backend。

结果：

- 新增 `internal/rekit/gate`：生成 gate dry-run JSON plan。
- Go CLI 新增 `-Command gate`，仅支持 `-WhatIf`；无 `-WhatIf` 直接拒绝，避免误写 ledger 或误执行工具。
- gate plan 字段：
  - 顶层：`schemaVersion`、`command=gate`、`caseRoot`、`repoRoot`、`pack`、`isMutation=false`、`reviewRequired=true`、`requiresConfirmation=true`。
  - `eventPreview`：`kind=request`、`status=pending-gate`、`lane`、`subject`、`summary`、`risk`、`target`、`batchId`。
  - `eventPreview.gate`：`action`、`scope`、`budget`、`triedLightSteps`、`stopConditions`、`deniedUntilUserConfirmation`。
- 校验 attached case 与 `.rekit/board.json` lane id；未知 lane 拒绝。
- D4 smoke 使用临时 case 输出 non-mutating JSON plan；未写 JSONL、未执行 full-trace/debug/inject/patch/dump/network、未产生需清理的 case state。
- `docs/go-runtime-migration.md` 更新 G2.2，明确 PowerShell façade 暂不默认委托；`gate -WhatIf` 属可选安全集合。

验证：

```powershell
go test ./...
go run ./cmd/rekit -- -Command gate -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun' -Pack vmp-re -WhatIf -Action full-trace -Lane feature-handler-0x40a010 -Subject 'D4 gate dry-run smoke' -Summary 'preview heavy-tool gate only' -TargetRef batch-d4-smoke -BatchId batch-d4-smoke -Scope 'handler only' -Budget '60s' -TriedLightSteps 'static review,overview' -StopConditions 'timeout,unexpected side effect'
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun'
git diff --check
```

验证结果：全部通过。

新会话接手：

- 下一步优先 Go 方向，不再把 D5/D6 大量堆到 PowerShell。
- 建议下一批：G2.3 gate confirm-to-ledger review-first 写入路径，或 G2.1 review artifact 写入 parity；继续保持 PowerShell façade 不默认委托。
- 用户已授权后续批次自主审查、评估、调整并继续执行；只有破坏性动作、外部副作用、真实 heavy-tool 执行、confirmed/authority 写入或难判架构取舍再停。

### Batch 29：G2.3 Go gate pending-gate ledger 写入

状态：已完成。

目标：在 D4 Go gate dry-run 之后，补上 Go backend 的最小落账路径：把 gate preview 显式写入 `.rekit/facts/requests.jsonl` 的 `pending-gate` request，但仍不执行 heavy-tool、不写 confirmed/authority。

结果：

- `internal/rekit/gate` 增加 `Apply` 路径，复用 dry-run preview 构造与 lane 校验。
- Go CLI `-Command gate` 现在要求显式选择一种模式：
  - `-WhatIf`：非写入 JSON preview。
  - `-Apply`：只 append pending-gate request JSONL。
  - 两者同时传入会被拒绝；两者都不传也会被拒绝。
- `gate -Apply` 要求 `-Actor`，用于记录谁批准写入 pending-gate request；无 `-Actor` 拒绝。
- 写入事件字段：`schemaVersion:1`、`kind:request`、`status:pending-gate`、`lane`、`subject`、`summary`、`actor`、`risk`、`target`、`batchId`、`gate{action,scope,budget,triedLightSteps,stopConditions,requiresConfirmation,deniedUntilUserConfirmation}`、`eventId`。
- `eventId` 由 gate 语义字段派生；重复写入返回 `applied=false` 与 `reason=duplicate eventId`，不重复 append。
- Go tests 覆盖：模式 guard、dry-run plan、未知 lane、apply 写入、apply 要求 actor、重复 eventId 幂等。
- 临时 case smoke 执行 `gate -Apply` 后已清理测试 request，未留下 G2.3 测试状态。
- PowerShell façade 仍不默认委托 Go；`gate -Apply` 也暂不列入默认委托安全集合，因为它已经是 case ledger mutation。

验证：

```powershell
go test ./...
go run ./cmd/rekit -- -Command gate -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun' -Pack vmp-re -Apply -Action debug -Lane feature-handler-0x40a010 -Actor runtime-test -Subject 'G2.3 gate apply smoke' -Summary 'append pending gate request only' -TargetRef batch-g23-apply-smoke -BatchId batch-g23-apply-smoke -Scope 'handler only' -Budget '30s' -TriedLightSteps 'static review' -StopConditions 'timeout'
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun'
git diff --check
```

验证结果：全部通过；smoke 写入已清理。

### Batch 30：G2.1 Go review artifact 写入 parity

状态：已完成。

目标：在 Go `sync/promote` review-only JSON plan 基础上补齐 review artifact 写入，使 Go backend 能输出可交给人工/LLM review 的 packet、summary、bounded diff 与 sanitized preview，但仍不执行 `sync -Apply`、不写 pack、不创建 candidates。

结果：

- Go CLI 增加 `-ReviewOutputDir`、`-PacketPath`、`-DiffPath` 参数；`sync/promote` review-only 路径检测到这些参数或 `-Review` 时写 artifact，而不是只把 plan 输出到 stdout。
- `internal/rekit/review` 增加 artifact writer：
  - `packet.json`：包含 plan、item path metadata、diff/preview 路径、`reviewRequired=true`。
  - `summary.md`：给 Claude/维护者的短 review 指南。
  - `diffs/combined.diff`：bounded diff 聚合；即使没有 diff 也创建空文件，保证调用方路径稳定。
  - `diffs/*.diff`：按 item 输出 bounded diff，限制变更行数和单行长度。
  - `previews/*.sanitized-preview.md`：promote tooling candidate source 的脱敏预览。
- `sync` review item 补充 source/target path 与 template planned text，使 managed-file、template-file、managed-block、support-file 可生成 bounded diff。
- `promote` review item 补充 case/pack path；tooling candidate source 生成带来源 header 的 sanitized preview，只在 deny pattern 清理后无残留时写 preview。
- artifact 写入结果返回 `isMutation=false`、`writesArtifacts=true`；这只表示写 review packet/diff/preview，不代表写 managed docs、pack 或 candidates。
- Go tests 覆盖 sync artifact、promote artifact、combined diff、sanitized preview，以及既有 review-only/gate guard。
- PowerShell façade 仍不默认委托 Go；G2.1 只完成手动 Go CLI parity。

验证：

```powershell
go test ./...
go run ./cmd/rekit -- -Command sync -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun' -Pack vmp-re -ReviewOutputDir <tmp>\sync
go run ./cmd/rekit -- -Command promote -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun' -Pack vmp-re -ReviewOutputDir <tmp>\promote
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun'
git diff --check
```

验证结果：全部通过；manual smoke 产生的 review artifact 位于临时目录，不写入 kit 仓库，也不修改 case managed docs/pack/candidates。

### Batch 31：G2.4 PowerShell façade 显式 Go 委托开关

状态：已完成。

目标：在 G2/G2.1/G2.2/G2.3 手动 Go CLI 验证后，给 `rekit.ps1` 增加低风险、显式启用的 Go backend 委托开关；默认仍保持 PowerShell canonical 入口和 fallback。

结果：

- `rekit.ps1` 增加环境变量：
  - `REKIT_GO_ENABLE=1`：允许安全集合委托 Go。
  - `REKIT_GO_DISABLE=1`：强制 fallback PowerShell，优先级高于 enable。
  - `REKIT_GO_EXE=<path>`：可选指定已构建 Go backend；未指定时优先 `rekit/bin/rekit-go.exe`，否则使用 `go run ./cmd/rekit --`。
- 委托安全集合：
  - `status`；
  - kit-root `doctor` / `validate`；case doctor 继续 PowerShell fallback；
  - `sync/update` review-only（含 `-ReviewOutputDir` / `-PacketPath` / `-DiffPath` artifact）；
  - `promote` review-only（含 artifact）；
  - `gate -WhatIf` dry-run。
- 明确不委托：`sync -Apply`、`promote -Apply/-CreateCandidates`、`gate -Apply`、case doctor、工作线命令、`note`。
- `rekit.ps1` 现在暴露 `gate` 参数集合（`-Action`、`-Lane`、`-Subject`、`-Summary`、`-Risk`、`-TargetRef`、`-BatchId`、`-Scope`、`-Budget`、`-TriedLightSteps`、`-StopConditions`），但只有 `REKIT_GO_ENABLE=1` 且 `-WhatIf` 时委托；未启用时明确拒绝，避免误以为 PowerShell 有 gate executor。
- Go CLI parser 兼容 `go run ./cmd/rekit -- ...` 中的 `--` 分隔符，并新增单元测试。
- `.claude/skills/rekit/SKILL.md` 与 `docs/go-runtime-migration.md` 记录 façade 默认关闭、显式开关和安全集合。

验证：

```powershell
go test ./...
$env:REKIT_GO_ENABLE='1'; .\rekit\rekit.ps1 -Command status
$env:REKIT_GO_ENABLE='1'; .\rekit\rekit.ps1 -Command doctor
$env:REKIT_GO_ENABLE='1'; .\rekit\rekit.ps1 -Command sync -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun' -Pack vmp-re -ReviewOutputDir <tmp>\sync
$env:REKIT_GO_ENABLE='1'; .\rekit\rekit.ps1 -Command gate -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun' -Pack vmp-re -WhatIf -Action debug -Lane feature-handler-0x40a010
Remove-Item Env:\REKIT_GO_ENABLE
.\rekit\rekit.ps1 -Command gate -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun' -Pack vmp-re -WhatIf -Action debug -Lane feature-handler-0x40a010 # 预期拒绝
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun'
git diff --check
```

验证结果：全部通过；`gate` 未启用 façade 时按预期拒绝。review artifact smoke 写到临时目录，不修改 kit 或 case managed docs。

### Batch 32：G2.5 Go case doctor parity

状态：已完成。

目标：补齐 Go backend attached case `doctor/validate` 的只读校验，使显式 Go façade 可以覆盖 case doctor，但不写 case state、不修复 shim、不迁移 schema。

结果：

- 新增 `internal/rekit/doctor.Case`：复用 `instance.AssertAttached` 与 manifest parser，只读校验 attached case。
- Go case doctor 覆盖：
  - `.rekit/instance.yml` / `.re-template.yml`（存在时非空且预算内）；
  - case-local `.claude/skills/rekit/SKILL.md` 非空、预算内，并与 canonical thin shim template 完全一致；
  - canonical skill 非空、预算内；
  - manifest managed files 与 template target files 非空、预算内；
  - managed block host 含 begin/end marker；
  - facts 9 种 JSONL、board.json、lane.json、lane root JSONL 与 workspace JSONL 可解析；
  - board `caseRoot` 与实际 case root 一致；lane id 与目录名一致，workspace/laneRoot 不逃逸 case root。
- Go CLI `doctor/validate` 对 attached case 输出 `instance validation ok`；非 case target 仍报错，不误报 pack validation。
- PowerShell façade 显式启用 `REKIT_GO_ENABLE=1` 时可委托 case doctor；默认仍走 PowerShell。
- Go tests 增加 full attached `_template` case doctor smoke 与 shim drift 失败用例。
- UTF-8 BOM 兼容：Go case doctor 读取 JSON/JSONL 时忽略行首 BOM，兼容 Windows PowerShell 5.1 可能生成的 BOM 文件。

验证：

```powershell
go test ./...
go run ./cmd/rekit -- -Command doctor -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun' -Pack vmp-re
$env:REKIT_GO_ENABLE='1'; .\rekit\rekit.ps1 -Command doctor -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun' -Pack vmp-re
Remove-Item Env:\REKIT_GO_ENABLE
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun'
git diff --check
```

验证结果：全部通过；case doctor 只读，不写 case state，不修复任何文件。

### Batch 33：G2.6 PowerShell façade 回归 smoke

状态：已完成。

目标：给 G2.4/G2.5 的 PowerShell façade 显式 Go 委托加回归 smoke，避免后续迁移误把写入命令、`gate -Apply` 或默认路径委托到 Go。

结果：

- 新增 `rekit/tests/facade-smoke.ps1`，使用独立 `powershell.exe` 子进程测试 façade 行为，避免当前会话环境变量污染结果。
- 覆盖默认行为：
  - 未设置 `REKIT_GO_ENABLE` 时，`status` 仍走 PowerShell，输出 `rekit runtime:`。
  - 未设置 `REKIT_GO_ENABLE` 时，`gate -WhatIf` 被 façade 拒绝，提示需启用 Go 或手动运行 Go CLI。
- 覆盖显式 Go 委托安全集合：
  - `REKIT_GO_ENABLE=1` 时，`status` 输出 `rekit go backend:`。
  - `doctor -Target <case>` 输出 `instance validation ok`。
  - `sync -ReviewOutputDir <tmp>` 返回 `writesArtifacts=true`，并写 `packet.json` 与 `diffs/combined.diff`。
  - `gate -WhatIf` 返回非写入 plan，含 `isMutation=false` 与 `pending-gate`。
- 覆盖拒绝/回退边界：
  - `sync -Apply -WhatIf` 不委托 Go，而走 PowerShell dry-run，输出 `would attach case`。
  - `REKIT_GO_DISABLE=1` 优先级高于 `REKIT_GO_ENABLE=1`，`status` 回退 PowerShell。
- 该 smoke 不写 managed docs、不写 pack、不写 candidates、不写 ledger；review artifact 只写 `$env:TEMP\rekit-facade-smoke-sync`。

验证：

```powershell
go test ./...
.\rekit\tests\facade-smoke.ps1 -CaseRoot 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun' -Pack vmp-re
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun'
git diff --check
```

验证结果：全部通过；smoke 仅产生临时 review artifact。

新会话接手：

- 下一步继续 Go 方向，优先 gate request schema parity，或评估 `attach/repair` 这类低风险写入命令迁移。
- 默认仍不要开启 Go 委托；只有维护者显式设置 `REKIT_GO_ENABLE=1` 时使用安全集合。
- `gate -Apply` 仍不经 façade 委托；pending-gate request 也不代表 heavy-tool 已获执行授权。

## 后续实施计划：G3+ Go 写入命令迁移

### Batch 34：G3.1 Go attach 迁移

状态：已完成。

目标：把低风险 `attach` 写入路径迁移到 Go backend，但仍保持 preview-first 边界：默认不写；只有显式 `-Apply` 才写 `.rekit/instance.yml` 与 case-local thin shim。该批不接管 `init/bootstrap`，不同步 managed docs，不创建 board/facts/lanes，不纳入 PowerShell façade 默认或显式委托安全集合。

结果：

- 新增 `internal/rekit/attach`，提供 `Preview` 与 `Apply` 两条路径。
- Go CLI 新增 `-Command attach` 与 `-ProjectName` 解析：
  - `-WhatIf`：输出 JSON plan，包含 metadata/shim 写入路径、repoRoot、pack、projectName、`isMutation=false`、`reviewRequired=true`、`requiresConfirmation=true`。
  - `-Apply`：仅写 `.rekit/instance.yml` 与 `.claude/skills/rekit/SKILL.md` thin shim，返回 `isMutation=true`、`applied=true` 与写入路径。
  - 无 `-WhatIf/-Apply` 拒绝；两者同时传入也拒绝。
- metadata 内容与 PowerShell `Write-RekitInstance` 的核心字段保持兼容：`schemaVersion`、`templateRoot`、`templatePack`、`projectName`、`projectRoot`、`mode: case-local-shim`。
- shim 直接复制 `rekit/templates/case-shim/SKILL.md`，不复制 runtime 逻辑到 case。
- `attach -WhatIf` 对不存在目标只预览，不创建目录或文件；`attach -Apply` 可创建目标目录。
- 若目标已绑定到不同 `templateRoot` / `templatePack`，拒绝覆盖并提示走 repair 路径。
- Go tests 覆盖 mode guard、preview 不写文件、apply 只写 metadata+shim、不同绑定拒绝。
- 自审决定：Go attach 暂不写 `.re-template.yml` 与 `.rekit/state.json`，保持本批“只写 instance + thin shim”的低风险边界；legacy/state 兼容仍由 PowerShell attach/init 与后续 repair/sync 批次覆盖。

验证：

```powershell
go test ./...
go run ./cmd/rekit -- -Command attach -Target <tmpCase> -Pack vmp-re -WhatIf
go run ./cmd/rekit -- -Command attach -Target <tmpCase> -Pack vmp-re -ProjectName <name> -Apply
go run ./cmd/rekit -- -Command doctor -Target <tmpCase> -Pack vmp-re # 预期只 attach 未 sync 时缺 managed docs
.\rekit\rekit.ps1 -Command attach -Target <tmpCase2> -Pack vmp-re -ProjectName <name>
.\rekit\rekit.ps1 -Command doctor -Target <tmpCase2>
git diff --check
```

验证结果：全部通过；Go attach smoke 临时目录已清理。Go doctor 对只 attach 未 sync 的 case 按预期报告缺 managed docs，未误报完整 case ok。

### Batch 35：G3.2 Go repair 迁移

状态：已完成。

目标：迁移 `repair` 的 moved-case metadata 修复路径。默认 preview；显式 `-Apply` 才更新 metadata 与 shim。不得修改 managed docs、facts、board、lanes 或 authority 文件。

结果：

- 新增 `internal/rekit/casebind`，集中 metadata、legacy metadata 与 case-local thin shim 写入 helper，避免 attach/repair 复制逻辑。
- 新增 `internal/rekit/repair`：
  - 默认 preview（含 `-WhatIf`）输出 JSON plan，`isMutation=false`、`reviewRequired=true`、`requiresConfirmation=true`，展示 metadata source、recorded/new projectRoot、moved 状态与写入计划。
  - `-Apply` 只刷新 `.rekit/instance.yml`、`.claude/skills/rekit/SKILL.md` thin shim 和 legacy `.re-template.yml` 的迁移相关字段。
- Go CLI 新增 `-Command repair`：
  - 要求显式 `-Target`；
  - `-WhatIf` 与 `-Apply` 同时传入会被拒绝；
  - 未传 `-Apply` 时只 preview，不写文件。
- repair 拒绝普通目录、kit repo root、不同 `templateRoot` 或不同 `templatePack`，不静默重绑到当前 kit。
- Go tests 覆盖 preview 不写、apply 刷新 metadata/shim/legacy、不同 binding 拒绝。
- PowerShell façade 仍不委托 `repair`；日常 `/rekit repair` 继续走 PowerShell fallback，Go repair 只作为维护者手动验证路径。

验证：

```powershell
go test ./...
go run ./cmd/rekit -- -Command repair -Target <movedCase> -Pack vmp-re -WhatIf
go run ./cmd/rekit -- -Command repair -Target <movedCase> -Pack vmp-re -Apply
go run ./cmd/rekit -- -Command doctor -Target <movedCase> -Pack vmp-re
.\rekit\tests\facade-smoke.ps1 -CaseRoot 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun' -Pack vmp-re
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun'
git diff --check
```

验证结果：全部通过；repair smoke 使用临时 case，验证后已删除，不污染 kit 仓库。

### Batch 36：gate request schema parity（计划）

状态：可与 G3.1/G3.2 后并行评估。

目标：让 Go `gate -Apply` 写入的 pending-gate request 与 PowerShell `note -Kind request` 的字段展示/查询语义完全对齐，包括 `actor/risk/target/batchId/gate` 扩展字段在 overview/handoff/note-List 中的展示策略。

边界：仍不执行 heavy-tool，不写 confirmed/authority，不把 `gate -Apply` 纳入 façade 默认委托。

### Batch 37：G3.3 sync -Apply 迁移预研（计划）

状态：待 attach/repair 稳定后再实施。

目标：只做设计与 golden test 预研，不直接迁移写入。需先补齐 backup、bounded diff、managed block、template local-file skip、失败恢复与旧 case compatibility 的测试矩阵。

停止条件：任何 backup/rollback 语义不清、PowerShell/Go diff 不一致、或旧 case 行为无法解释时，暂停并回到 PowerShell fallback。
