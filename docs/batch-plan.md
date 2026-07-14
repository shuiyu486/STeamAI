# Batch implementation plan

## 目的

本文件记录后续批次计划，避免上下文压缩后只依赖聊天历史。

## 已完成批次

### Batch 0：项目定位与维护入口

- 新增根目录 `CLAUDE.md`。
- README 顶部改为 Agent Team 框架定位（Batch 59 已将 RE-only 描述纠偏为网络安全研究 / 安全工程 Agent Team 框架）。
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

目标：降低新增安全领域 pack 的成本；当时以 `unpack-pe`、`android-native`、`ollvm`、`generic-binary-re` 为候选，Batch 59 后补充 `web-security`、`malware-analysis`、`vuln-research`、`ctf` 等非 RE-only 方向。

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

### Batch 36：gate request schema parity

状态：已完成。

目标：让 Go `gate -Apply` 写入的 pending-gate request 与 PowerShell `note -Kind request` 的字段展示/查询语义完全对齐，包括 `actor/risk/target/batchId/gate` 扩展字段在 overview/handoff/note-List 中的展示策略。

结果：

- `B3.Core.ps1` 新增共享展示 helper：兼容 PSCustomObject / dictionary / array 的字段读取与 scalar 展示，并集中格式化 pending-gate request 明细。
- `overview` 的 `pending-gate（heavy-tool 待确认）` 区段现在展示 `by`、`risk`、`target`、`batch`、`action`、`scope`、`budget`、`tried`、`stop`。
- lane `handoff` 的 `## pending-gate` 区段复用同一 formatter，避免 Go `gate -Apply` 写入的 `gate{...}` 详情在接手文档中丢失。
- `note -List -Kind request` 复用 formatter 展示 `status` 与 gate 扩展字段，并避免 `batchId` 重复输出。
- 新增 `rekit/tests/gate-parity-smoke.ps1`：创建临时 case，通过 Go `gate -Apply` 写入 pending-gate request，再验证 PowerShell `overview`、`note -List` 与 `handoff` 三处展示字段，最后删除临时 case。

边界：仍不执行 heavy-tool，不写 confirmed/authority，不把 `gate -Apply` 纳入 façade 默认委托；本批只增强读层展示与 smoke 验证。

验证：

```powershell
.\rekit\tests\gate-parity-smoke.ps1
go test ./...
.\rekit\tests\facade-smoke.ps1 -CaseRoot 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun' -Pack vmp-re
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun'
git diff --check
```

验证结果：全部通过；gate parity smoke 使用临时 case，验证后删除，不污染 kit 仓库。

### Batch 37：G3.3 sync -Apply 迁移预研

状态：已完成（预研与验证资产；未实现 Go `sync -Apply` 写入）。

目标：只做设计与 golden test 预研，不直接迁移写入。需先补齐 backup、bounded diff、managed block、template local-file skip、失败恢复与旧 case compatibility 的测试矩阵。

结果：

- 新增 `docs/sync-apply-migration.md`，固化 PowerShell `sync -Apply` 当前语义基线、Go 迁移契约、停止条件与 S1-S18 测试矩阵。
- 明确下一批实现 Go `sync -Apply` 前必须先完成双临时 case parity：PowerShell apply 与 Go apply 内容 hash 一致，允许 backup timestamp 差异。
- 明确 Go `sync -Apply` 后续仍是手动 CLI 验证路径；默认 `sync` 保持 review-only，PowerShell façade 暂不委托写入。
- 新增 `rekit/tests/sync-review-parity-smoke.ps1`，用 `_template` 临时 case 构造 managed drift、managed missing、managed block drift、local template existing，验证 PowerShell/Go review action 与 bounded diff parity，且 review-only 不改目标文件。

停止条件：任何 backup/rollback 语义不清、PowerShell/Go diff 不一致、或旧 case 行为无法解释时，暂停并回到 PowerShell fallback。

验证：

```powershell
.\rekit\tests\sync-review-parity-smoke.ps1
go test ./...
.\rekit\tests\facade-smoke.ps1 -CaseRoot 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun' -Pack vmp-re
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun'
git diff --check
```

验证结果：全部通过；sync review parity smoke 使用临时 case，验证后删除，不污染 kit 仓库。

### Batch 38：G3.4 Go sync -Apply 手动路径

状态：已完成。

目标：在 Go backend 中实现显式 `sync -Apply` 写入路径，但只作为维护者手动 CLI 验证路径；默认 `sync` 继续 review-only，PowerShell façade 仍不委托写入命令。

实施范围：

- 新增 Go `sync.Apply` helper，复用 manifest/instance/casebind/review 基础能力。
- 覆盖 PowerShell `sync -Apply` 的核心语义：刷新 metadata/shim/legacy metadata、同步 managed files、template create-if-missing、managed block 更新、可选 `-Force` 覆盖本地模板、`.gitignore` support file create-if-missing、更新 `.rekit/state.json`。
- 写入前对 changed managed files、forced template files、managed block host 创建 backup，backup root 来自 manifest `workstreamDefaults.backupRoot`。
- CLI 输出结构化 JSON apply result，包含 `isMutation=true`、`applied=true`、`backupRoot` 与逐项 writes。
- Go tests 覆盖 apply 写入/backup/state、template skip、`-Force` 覆盖、moved/different binding guard、默认 review-only 不回归。
- 新增 `rekit/tests/sync-apply-smoke.ps1`，对 Go `sync -Apply` 执行后验证 backup、managed file、managed block、template skip/force、state，并运行 Go doctor 与 PowerShell doctor；临时 case 验证后删除。

边界：不实现 `init/bootstrap`，不写 pack，不执行 promote，不写 authority/confirmed，不自动 rollback，不纳入 PowerShell façade 委托安全集合。

停止条件：backup 路径无法证明在 case root 内、state hash 与实际写入不一致、managed block 写入与 PowerShell 语义冲突、旧 case 兼容 guard 不清晰、或 apply 后 doctor 失败。

验证：

```powershell
go test ./...
.\rekit\tests\sync-apply-smoke.ps1
.\rekit\tests\facade-smoke.ps1 -CaseRoot 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun' -Pack vmp-re
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun'
git diff --check
```

验证结果：全部通过；sync apply smoke 使用临时 case，验证后删除，不污染 kit 仓库。

### Batch 39：G3.5 Go init/bootstrap 手动路径

状态：已完成。

目标：在 Go backend 中实现 `init` / `bootstrap` 手动验证路径，使维护者可用 Go CLI 初始化临时 case；公共 `/rekit init` 仍由 PowerShell façade / fallback 处理，Go 写入路径不纳入 façade 委托安全集合。

计划文档：`docs/init-bootstrap-migration.md`。

实施范围：

- 在 Go sync/apply 层增加受控 create-local-files 模式，允许 missing target 进入 init/bootstrap 写入流程，同时继续拒绝 moved case、different templateRoot/templatePack 和 kit repo root target。
- 新增 Go CLI `-Command init|bootstrap`：`-WhatIf` 输出非写入 JSON plan，`-Apply` 执行写入；裸命令拒绝，`-WhatIf` 与 `-Apply` 互斥。
- 复用 G3.4 sync apply 语义落地 metadata/shim/legacy metadata、managed files、template files、managed block、support file 和 `.rekit/state.json`。
- 增加 Go tests 与临时 case smoke，验证 `init -WhatIf` 不写、`init/bootstrap -Apply` 写入完整 case、`-Force` template 覆盖、doctor 双通过、PowerShell façade 不委托。

边界：不写 pack，不执行 promote，不写 authority/confirmed，不创建 confirmed CSV，不执行 heavy-tool/debug/inject/patch/dump/network，不自动 rollback，不迁移 board/facts/lanes 初始化语义，不纳入 PowerShell façade 委托安全集合。

停止条件：missing target guard 无法区分普通目录与 moved/different binding、kit repo root target 保护不清晰、`-WhatIf` 有副作用、apply 后 Go/PowerShell doctor 失败、或实现需要改变 manifest/runtime schema。

验证：

```powershell
go test ./...
.\rekit\tests\init-bootstrap-smoke.ps1
.\rekit\tests\facade-smoke.ps1 -CaseRoot 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun' -Pack vmp-re
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun'
git diff --check
```

验证结果：全部通过；init/bootstrap smoke 使用临时 case，验证后删除，不污染 kit 仓库。

### Batch 40：G3.6 promote -CreateCandidates 迁移预研

状态：已完成。

目标：先固化 PowerShell `promote -CreateCandidates` 写入候选语义、sanitization 规则、阻断条件与 Go review parity 验证资产；本批不直接迁移 Go 写入路径，后续再评估是否实现 Go CLI 手动 `-CreateCandidates`。

计划文档：`docs/promote-candidates-migration.md`。

实施范围：

- 固化 managed doc candidate、tooling sanitized candidate、deny pattern、case-specific pattern、index 写入和 cleanup 的基线。
- 新增 `rekit/tests/promote-candidates-preflight-smoke.ps1`，用临时 case 验证 PowerShell `-WhatIf -CreateCandidates` baseline 与 Go promote review artifact / sanitized preview 对齐。
- 验证 Go backend 仍拒绝 `promote -CreateCandidates`，PowerShell façade 即使启用 Go 也不委托写入命令。
- 更新 Go migration 文档、batch plan 与 changelog。

边界：不写 pack candidates，不执行 `promote -Apply`，不覆盖 pack managed docs，不写 authority/confirmed，不执行 heavy-tool/debug/inject/patch/dump/network，不纳入 PowerShell façade 委托安全集合。

停止条件：sanitization 与 PowerShell baseline 无法对齐、preflight smoke 会残留 pack candidates、deny pattern 未能阻断 case-specific 内容、或需要改变 manifest/runtime schema。

验证：

```powershell
go test ./...
.\rekit\tests\promote-candidates-preflight-smoke.ps1
.\rekit\tests\facade-smoke.ps1 -CaseRoot 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun' -Pack vmp-re
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun'
git diff --check
```

验证结果：全部通过；preflight smoke 使用临时 case，验证后删除，不写 pack candidates，不污染 kit 仓库。

### Batch 41：G3.7 Go promote -CreateCandidates 手动路径

状态：已完成。

目标：在 Go backend 中实现显式 `promote -CreateCandidates` 手动写入路径，用于维护者生成 pack candidate 文件；默认 `promote` 继续 review-only，PowerShell façade 仍不委托写入命令。

计划文档：`docs/promote-candidates-migration.md`。

实施范围：

- 新增 Go `promote.CreateCandidates` helper，复用 existing promote plan、deny pattern 与 tooling sanitization 结果。
- CLI 支持 `-Command promote -CreateCandidates`，可选 `-WhatIf` 输出非写入 preview；拒绝 `-Apply`、拒绝与 review artifact options 混用。
- 写入 managed doc candidates 到 `packs/<pack>/promote-candidates/`，写 `index.json`；写 tooling candidates 到 `packs/<pack>/tooling/candidates/`。
- 输出结构化 JSON result，包含 `isMutation`、created/blocked/skipped 统计、candidate/index/tooling 路径与逐项 writes。
- 增加 Go tests 与临时 case smoke，验证 candidate 写入、blocked deny、tooling sanitization、pack-root containment、cleanup、façade fallback。

边界：不执行 `promote -Apply`，不覆盖 pack managed docs，不写 authority/confirmed，不执行 heavy-tool/debug/inject/patch/dump/network，不纳入 PowerShell façade 委托安全集合。

停止条件：candidate root 无法证明在 pack root 内、sanitization 与 PowerShell baseline 冲突、deny pattern 未能阻断 case-specific 内容、smoke 会残留 candidates、或需要改变 manifest/runtime schema。

验证：

```powershell
go test ./...
.\rekit\tests\promote-candidates-preflight-smoke.ps1
.\rekit\tests\promote-candidates-apply-smoke.ps1
.\rekit\tests\facade-smoke.ps1 -CaseRoot 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun' -Pack vmp-re
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun'
git diff --check
```

结果：

- 新增 Go `promote.CreateCandidates` helper，复用 `Plan` 的 managed promote action、deny pattern 与 tooling sanitization 结果。
- Go CLI 支持 `-Command promote -CreateCandidates`：`-WhatIf` 输出非写入 JSON preview；无 `-WhatIf` 时写 pack root 下的 `promote-candidates/*.candidate.md`、`promote-candidates/index.json` 与 `tooling/candidates/*.candidate.md`。
- `promote -Apply` 仍拒绝；`-CreateCandidates` 拒绝与 review artifact options 混用；PowerShell façade 仍不委托该写入命令。
- 新增 `rekit/tests/promote-candidates-apply-smoke.ps1`，验证 candidate 写入、blocked deny、tooling sanitization、pack-root containment、cleanup 与 façade fallback；更新 preflight smoke 验证 Go `-CreateCandidates -WhatIf` 非写入。

验证结果：全部通过；preflight/apply smoke 使用临时 case，验证后删除；apply smoke 清理本次新增 `promote-candidates/**` 与 `tooling/candidates/**` 文件，未留下 pack candidates。

### Batch 42：G3.8 promote -Apply 迁移预研

状态：已完成（预研与验证资产；未实现 Go `promote -Apply` 写入）。

目标：在实现 Go `promote -Apply` 前，固化 PowerShell apply baseline、backup/deny/validation/cleanup 语义和 Go 迁移契约。`promote -Apply` 会直接覆盖 pack managed docs，风险高于 candidate 写入，因此本批只做 preflight，不迁移写入 helper。

计划文档：`docs/promote-apply-migration.md`。

实施范围：

- 固化 PowerShell `promote -Apply -WhatIf` 非写入 baseline：safe changed managed doc 显示 would promote，deny 内容 blocked，不写 pack source/backups/candidates。
- 固化 PowerShell `promote -Apply` baseline：safe changed managed doc 先 backup 再写 pack source；deny 内容不写；写入后运行 pack validation；smoke 在 finally 中恢复 pack source 并清理新增 backup/candidate。
- 新增 `rekit/tests/promote-apply-preflight-smoke.ps1`，覆盖 PowerShell apply baseline、Go apply guard、façade fallback、backup root containment 与 cleanup。
- 更新 `docs/go-runtime-migration.md`、`docs/promote-apply-migration.md` 与 `CHANGELOG.md`。

边界：不实现 Go `promote -Apply`，不覆盖 pack managed docs（最终状态恢复原文），不写 authority/confirmed，不执行 heavy-tool/debug/inject/patch/dump/network，不纳入 PowerShell façade 委托安全集合。

停止条件：无法可靠恢复 pack source、backup 无法证明在 pack root 内、deny pattern 未能阻断 case-specific 内容、smoke 会残留 backup/candidates、或需要改变 manifest/runtime schema。

验证：

```powershell
go test ./...
.\rekit\tests\promote-apply-preflight-smoke.ps1
.\rekit\tests\facade-smoke.ps1 -CaseRoot 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun' -Pack vmp-re
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun'
git diff --check
```

验证结果：全部通过；preflight smoke 使用临时 case，验证后删除；PowerShell apply baseline 写入的 `_template` pack source 已恢复，新增 backup/candidate 已清理；Go backend 仍拒绝 `promote -Apply`。

### Batch 43：G3.9 Go promote -Apply 手动路径

状态：已完成。

目标：在 Go backend 中实现显式 `promote -Apply` 手动写入路径，用于维护者把已确认的 case managed docs 回流到 pack source；默认 `promote` 继续 review-only，PowerShell façade 仍不委托写入命令。

计划文档：`docs/promote-apply-migration.md`。

实施范围：

- 新增 Go `promote.Apply` helper，复用 `Plan` 的 managed promote action、deny pattern 与 case-specific pattern 结果。
- CLI 支持 `-Command promote -Apply` 与 `-Apply -WhatIf`：`-WhatIf` 输出非写入 JSON preview；无 `-WhatIf` 时先备份 pack source，再写 safe managed docs，并运行 pack validation。
- 写入仅限 `candidate-after-llm-review` 的 managed docs；blocked/skip/unchanged 只进入结构化 result，不写 pack source。
- 输出结构化 JSON result，包含 `isMutation`、`applied`、changed/blocked/skipped 统计、backup/target/source 路径、validation rows、requiresCleanup 与 denied write actions。
- 增加 Go tests 与临时 case smoke，验证 what-if 非写入、backup 内容、blocked deny、pack-root containment、validation、cleanup、façade fallback。

边界：不写 authority/confirmed，不执行 heavy-tool/debug/inject/patch/dump/network，不写 tooling candidates（tooling 仍走 `-CreateCandidates`），不纳入 PowerShell façade 委托安全集合。

停止条件：backup/target 无法证明在 pack root 内、validation 失败无法恢复 pack source、deny pattern 未能阻断 case-specific 内容、smoke 会残留 backup/candidates、或需要改变 manifest/runtime schema。

验证：

```powershell
go test ./...
.\rekit\tests\promote-apply-preflight-smoke.ps1
.\rekit\tests\promote-apply-smoke.ps1
.\rekit\tests\facade-smoke.ps1 -CaseRoot 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun' -Pack vmp-re
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun'
git diff --check
```

验证结果：全部通过；preflight/apply smoke 使用临时 case，验证后删除；Go `promote -Apply -WhatIf` 未写 pack/backups/candidates；Go `promote -Apply` 只写 safe managed docs，blocked deny 未写，backup 内容、validation rows、pack-root containment 与 cleanup 均通过；PowerShell façade 显式 Go enable 下仍 fallback，不委托该写入命令。

### Batch 44：G4.1 Go overview 只读路径

状态：已完成。

目标：启动 G4 工作线命令迁移，先在 Go backend 中实现 `overview` 只读手动路径，用于读取 attached case 的现有 board/facts/lane 状态并输出项目总览；公共 PowerShell façade 默认仍不委托工作线命令。

实施范围：

- 新增 Go workstream/overview 模块，读取 `.rekit/board.json`、9 类 facts JSONL 与 lanes metadata，生成与 PowerShell overview 主要区段对齐的只读摘要。
- CLI 支持 `-Command overview`，要求显式 attached case target；不自动创建 board/facts/lanes，不写 handoff、不写 ledger、不刷新 metadata。
- 输出面向维护者的文本 summary，覆盖工作线、共享事实计数、未决 candidate、pending-gate、最近 decision、最近 batch、未解决/最近 intervention、最近 rollback 与建议下一步。
- 增加 Go tests 与临时 case smoke，验证只读、历史/新 ledger 字段兼容、Go gate request 展示字段、缺失 board 时拒绝并提示使用 PowerShell overview 初始化。
- 更新 `docs/go-runtime-migration.md`、`CHANGELOG.md` 与后续批次计划。

边界：不迁移 `continue/start/handoff` 写入，不执行 heavy-tool/debug/inject/patch/dump/network，不写 authority/confirmed，不默认纳入 PowerShell façade 委托。

停止条件：无法安全兼容现有 board/facts JSONL、需要 runtime schema 迁移、需要 PowerShell overview 的初始化写入语义、或只读 smoke 发现 Go 输出会误导用户进入不存在的工作线。

验证：

```powershell
go test ./...
.\rekit\tests\overview-readonly-smoke.ps1
.\rekit\tests\facade-smoke.ps1 -CaseRoot 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun' -Pack vmp-re
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun'
git diff --check
```

验证结果：全部通过；Go `overview` 只读取既有 board/facts，缺 board 时拒绝且不创建 board/facts/lanes；临时 case smoke 验证 pending-gate gate 详情、未决 candidate 冲突、decision、batch、intervention、rollback 和下一步建议展示；PowerShell façade 显式 Go enable 下仍 fallback，不委托 `overview`；reviewer 未发现高置信问题。

### Batch 45：G4.2 Go start 手动路径

状态：已完成。

目标：在 Go backend 中实现显式 `start` 手动路径，用于维护者创建或进入功能工作线；公共 PowerShell façade 仍不委托工作线命令，用户 `/rekit start <name>` 继续走 PowerShell runtime。

实施范围：

- 新增 Go workstream start helper，复刻 PowerShell `Invoke-RekitStart` 的核心语义：基于 manifest `defaultStartLaneType` 计算 lane id，初始化 board/facts/policy/default authority lane，创建或进入目标 feature lane，并刷新 board。
- CLI 支持 `-Command start -WhatIf` 与 `-Apply`：`-WhatIf` 输出非写入 JSON preview，不创建 board/facts/lanes/workspace；`-Apply` 显式写 `.rekit/board.json`、`.rekit/policy.yml`、facts JSONL、lane state、workspace scaffold、lane resume/checkpoint。
- 支持 `-Force` 刷新已有 lane metadata；无 `-WhatIf/-Apply` 时拒绝，避免手动 Go CLI 裸 `start` 意外写 case state。
- 增加 Go tests 与临时 case smoke，验证 preview 只读、apply 写入范围、existing-lane/force 行为、Go/PowerShell doctor、PowerShell façade fallback。
- 更新 `docs/go-runtime-migration.md`、`README.md`、`CHANGELOG.md` 与后续批次计划。

边界：不迁移 `continue/handoff/plan-subagents`，不写 handoff，不写 authority/confirmed，不执行 heavy-tool/debug/inject/patch/dump/network，不默认纳入 PowerShell façade 委托。

停止条件：lane id/path 无法证明在 case root 内、Go 写入不能通过 case doctor、PowerShell façade 误委托 `start`、或实现需要 runtime schema 迁移。

验证：

```powershell
go test ./...
.\rekit\tests\start-apply-smoke.ps1
.\rekit\tests\facade-smoke.ps1 -CaseRoot '<attachedCase>' -Pack vmp-re
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Target '<attachedCase>'
git diff --check
```

验证结果：全部通过；Go `start -WhatIf` 未写 board/facts/lanes/workspace；Go `start -Apply` 显式初始化 board/facts/policy/default authority lane 并创建 feature lane；existing/force 行为、Go/PowerShell doctor 与 PowerShell façade fallback 均通过；`-Force` 刷新已有 lane 时保留 live inbox/tasks 的 resume/checkpoint；reviewer 发现的 force refresh 状态覆盖与 Batch 45 绝对路径问题已修复；不写 handoff、authority/confirmed，不执行 heavy-tool。

### Batch 46：G4.3 Go handoff 手动路径

状态：已完成。

目标：在 Go backend 中实现显式 `handoff` 手动路径，用于维护者预览或生成项目级/工作线级接手文档；公共 PowerShell façade 仍不委托工作线命令，用户 `/rekit handoff` 继续走 PowerShell runtime。

实施范围：

- 新增 Go workstream handoff helper，复刻 PowerShell `Write-RekitHandoff` / `Write-RekitProjectHandoff` / `Write-RekitLaneHandoff` 的核心输出语义：项目级 `latest.md` 索引、工作线级 `<laneId>-latest.md`、推荐读取、工作线列表、workspace packet、decision、pending-gate、intervention、rollback 与边界区段。
- CLI 支持 `-Command handoff -WhatIf` 与 `-Apply`：`-WhatIf` 输出非写入 JSON preview；`-Apply` 显式写 `.rekit/handovers/**`，并刷新相关 lane `prompts/RESUME.md` / `checkpoints/latest.json`。
- 支持无 selector 生成项目级 handoff；支持 `main`、feature name 与 lane id selector 生成指定工作线 handoff；未知 selector 必须拒绝并列出可选工作线。
- 增加 Go tests 与临时 case smoke，验证 preview 只读、apply 写入范围、项目级/工作线级输出关键区段、Go/PowerShell doctor、PowerShell façade fallback。
- 更新 `docs/go-runtime-migration.md`、`README.md`、`CHANGELOG.md` 与后续批次计划。

边界：不迁移 `continue/plan-subagents`，不创建 board/facts/lanes（缺 board 时拒绝并提示先运行 `start` 或 PowerShell `overview` 初始化），不写 authority/confirmed，不执行 heavy-tool/debug/inject/patch/dump/network，不默认纳入 PowerShell façade 委托。

停止条件：selector 解析不能与 PowerShell 行为对齐、handoff path/lane resume path 无法证明在 case root 内、Go 写入不能通过 case doctor、PowerShell façade 误委托 `handoff`、或实现需要 runtime schema 迁移。

验证：

```powershell
go test ./...
.\rekit\tests\handoff-apply-smoke.ps1
.\rekit\tests\facade-smoke.ps1 -CaseRoot '<attachedCase>' -Pack vmp-re
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Target '<attachedCase>'
git diff --check
```

验证结果：全部通过；Go `handoff -WhatIf` 未写 `.rekit/handovers` 或 lane resume/checkpoint；Go `handoff -Apply` 显式写项目级/工作线级 handoff 并刷新 lane resume/checkpoint；项目级/工作线级关键区段、selector、Go/PowerShell doctor 与 PowerShell façade fallback 均通过；reviewer 发现的 lane id/path containment 与旧 lane `laneRoot` 兼容问题已修复并补回归测试；公共 PowerShell façade 仍不委托 `handoff`，不创建 board/facts/lanes，不写 authority/confirmed，不执行 heavy-tool。

### Batch 47：G4.4 Go plan-subagents 手动路径

状态：已完成。

目标：在 Go backend 中实现显式 `plan-subagents` 手动路径，用于维护者生成只读 subagent 分片 review artifact；公共 PowerShell façade 仍不委托内部/工作线命令，用户日常仍不需要直接调用底层 Go CLI。

实施范围：

- 新增 Go subagent plan helper，复刻 PowerShell `Write-RekitSubagentPlan` 的核心语义：按 manifest `subagentRoutes` 选择 route，按 `Items` / `ItemsFile` 拆分 item，按 `targetItemsPerAgent` / `ItemsPerAgent` 生成 shard，并输出 route、input、shardPolicy、shards、mainAgentResponsibilities、subagentPermissions 与 outputContract。
- CLI 支持 `-Command plan-subagents` 及 `-Route`、`-TaskType`、`-Items`、`-ItemsFile`、`-ItemsPerAgent`、`-MaxParallel`、`-ReviewOutputDir`、`-PacketPath`、`-DiffPath`；默认写 `.rekit/reviews/<timestamp>-plan-subagents` review artifact，或在显式 `-ReviewOutputDir` 下允许 out-of-case artifact。
- 保持 `isMutation=false`、`writesReviewArtifacts=true`、`reviewRequired=true`；只写 review packet / summary / diff 路径清理，不写 board/facts/lanes/handoff/authority，不启动 subagent。
- 增加 Go tests 与临时 case smoke，验证 route/taskType 选择、items 分片、ItemsFile、review artifact 写入、out-of-case guard、Go/PowerShell doctor 与 PowerShell façade fallback。
- 更新 `docs/go-runtime-migration.md`、`README.md`、`CHANGELOG.md` 与后续批次计划。

边界：不迁移 `continue`，不自动启动 agent，不读取/写入真实 RE artifact，不创建或修改 case board/facts/lanes，不写 authority/confirmed，不执行 heavy-tool/debug/inject/patch/dump/network，不默认纳入 PowerShell façade 委托。

停止条件：route selection 与 PowerShell 行为不能对齐、artifact path containment / out-of-case guard 无法证明、PowerShell façade 误委托 `plan-subagents`、Go 写入不能通过 case doctor、或实现需要 manifest/runtime schema 迁移。

验证：

```powershell
go test ./...
.\rekit\tests\plan-subagents-smoke.ps1
.\rekit\tests\facade-smoke.ps1 -CaseRoot '<attachedCase>' -Pack vmp-re
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Target '<attachedCase>'
git diff --check
```

验证结果：全部通过；`plan-subagents` Go CLI 只写 review packet/summary artifact，attached case 默认写 `.rekit/reviews/<timestamp>-plan-subagents`，out-of-case target 必须显式 `-ReviewOutputDir`；route/taskType、Items/ItemsFile 分片、ItemsPerAgent/MaxParallel override、missing routes guard、Go/PowerShell doctor 与 PowerShell façade fallback 均通过。reviewer 未发现 correctness/boundary 高置信问题；project-guideline reviewer 提出的 Batch 47 状态与 mutation flag guard 已修复并补回归测试。公共 PowerShell façade 仍不委托 `plan-subagents`，未写 board/facts/lanes/handoff/authority/confirmed，未启动 subagent，未执行 heavy-tool。

### Batch 48：Agent Team contract 收敛

状态：已完成。

目标：先修正 Agent Team review loop 的通用契约漂移，让 `plan-subagents -> reviewer verdict -> verification/decision ledger -> overview/handoff` 的字段语义一致；本批只改 contract / schema 文档，不改 runtime，不启动 agent，不写 case state。

实施范围：

- 更新 `common/policies/agent-team.md`，将 main decision event canonical enum 收敛为 `accept|reject|defer|supersede`，说明历史 `confirm`/`confirmed`/`action` 仅作为读层兼容值。
- 更新 reviewer verdict 到 ledger 的映射说明：reviewer output `decision` 先进入 `verification.verdict`，再由 main agent 写最终 `decision` event；accepted decision 不等于直接写 authority。
- 更新 packet 文件与 facts event 关系，反映当前已有 `/rekit note` 手动 append 与 `/rekit continue` 自动抽取两条路径。
- 更新 `docs/evidence-ledger.md`，补 `request`/heavy-tool gate 扩展字段、status/decision 命名兼容说明，以及 `needs_more_evidence` packet 与 `needs-more-evidence` ledger verdict 的转换策略。
- 必要时补充 `common/policies/subagents.md` 的层级说明，避免 reviewer output decision 与 ledger decision 混用。
- 更新 `CHANGELOG.md`。

边界：不改 PowerShell/Go runtime，不改 manifest，不改 VMP managed docs（留给 Batch 49），不写 confirmed/authority，不执行 heavy-tool/debug/inject/patch/dump/network，不创建临时 case。

停止条件：发现需要 runtime schema 迁移、历史 JSONL 迁移、或必须改变 `/rekit note`/`continue` 写入行为时暂停并重新评估。

验证：

```powershell
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过；本批只更新 contract/schema 文档与 CHANGELOG，未改 PowerShell/Go runtime、manifest 或 VMP managed docs。`agent-team.md` 已将 main decision canonical enum 收敛为 `accept|reject|defer|supersede`，明确 reviewer output decision 先映射为 `verification.verdict`，accepted decision 不自动写 confirmed/authority；facts event 关系已更新为 `/rekit continue` 自动抽取与 `/rekit note` 手动 append 两条路径。`evidence-ledger.md` 已补 request/heavy-tool gate 扩展字段、status/decision 兼容说明，以及 packet `needs_more_evidence` 到 ledger `needs-more-evidence` 的归一化说明。

### Batch 49：VMP managed docs / manifest route 同步

状态：已完成。

目标：同步 VMP 下发文档与 manifest `subagentRoutes`，让 case-local managed docs、route output contract 与 Batch 48 的通用 Agent Team contract 保持一致。

实施范围：

- 更新 `packs/vmp-re/references/vmp-re/agent-driven-re.md`：补 canonical contract 指向，Evidence packet 示例补 `evidence_id`，Task packet `lane` 改为 runtime normalized lane id，Review packet 示例改 `output_contract`，candidate next_action 从旧 `confirm` 改为 `accept`，状态流明确 main decision 与 confirmed/authority 写入分层。
- 更新 `packs/vmp-re/manifest.yml`：两条 `subagentRoutes.outputContract` 补 `tier_used,tool_scope`，与 `common/policies/subagents.md` 和 VMP overlay 对齐。
- 更新 `CHANGELOG.md` 与本计划。

边界：不改 runtime，不改 common contract，不写 case state，不启动 agent，不执行 heavy-tool/debug/inject/patch/dump/network，不写 confirmed/authority。

停止条件：manifest 校验或 pack validation 因 outputContract 扩展失败，或发现需要修改 managed file schema / sync 语义。

验证：

```powershell
.\rekit\rekit.ps1 -Command doctor
.\packs\vmp-re\scripts\validate.ps1
git diff --check
```

验证结果：全部通过；VMP managed doc 已对齐 Batch 48 contract，`agent-driven-re.md` 明确 canonical contract 来源、补 `evidence_id`、runtime normalized lane id、`output_contract` 与 main decision / confirmed-authority 分层；`packs/vmp-re/manifest.yml` 两条 `subagentRoutes.outputContract` 已补 `tier_used,tool_scope`。本批未改 runtime、未写 case state、未启动 agent、未执行 heavy-tool，pack validation 与 doctor 均通过。

### Batch 50：review loop smoke

状态：已完成。

目标：把 `plan-subagents -> reviewer verdict -> verification/decision ledger -> overview/handoff` 的 Agent Team review loop 固化为可重复 smoke，防止 Batch 48/49 收敛后的 contract 再次漂移。

实施范围：

- 新增 `rekit/tests/agent-team-review-loop-smoke.ps1`，使用临时 case 验证：初始化 case、创建功能支线、生成 `plan-subagents` review packet、检查 route output contract 包含 `tier_used`/`tool_scope`、写入 `verification` 与 `decision` ledger event、用 `note -List` 查询 verification/decision、用 `overview` 与 lane `handoff` 展示 main decision。
- 修复 PowerShell façade `note` 参数透传：显式命名参数（如 `-Lane`/`-Subject`/`-Actor`/`-TargetRef`）应进入 `Invoke-RekitNote`，保留 `RemainingArgs` 解析用于 slash-command 风格兼容。
- 更新 `CHANGELOG.md` 与本计划。

边界：不启动 subagent，不写 confirmed/authority，不执行 full-trace/debug/inject/patch/dump/network，不把 `gate -Apply` 视为 heavy-tool 授权；临时 case 与 review artifact 用完即删，不污染 kit 仓库或 pack 模板。

停止条件：smoke 需要改变 ledger schema、历史 JSONL 迁移、自动 dispatch reviewer，或要求 overview/handoff 新增 verification 展示时暂停并重新评估。

验证：

```powershell
.\rekit\tests\agent-team-review-loop-smoke.ps1
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过；smoke 覆盖 review packet、verification event、main decision event、`note -List` 查询、`overview` decision 展示与 lane `handoff` decision 展示。PowerShell façade `note` 已恢复显式命名参数透传，并保留 `RemainingArgs` 兼容解析。本批仅写临时 case/review artifact 并清理，未启动 subagent、未写 confirmed/authority、未执行 heavy-tool。

### Batch 51：overview / handoff review visibility

状态：已完成。

目标：让 Batch 50 固化的 review loop 在日常接手入口中更直观可见：`overview` 直接显示 reviewer verification，lane `handoff` 显示 reviewer verdict 与 main decision 的相邻状态。

实施范围：

- PowerShell `overview` 增加“最近 verification”区段，展示 `lane`、`verifier`、`verdict`、`target`、`actor` 与 `batchId`。
- PowerShell lane `handoff` 增加 `## verification` 区段，限最近 5 条，与既有 `## decision`、`## pending-gate`、`## intervention`、`## rollback` 并列。
- Go `overview` 与 Go `handoff` 手动路径同步展示 verification，保持 PowerShell/Go 读层语义一致；公共 PowerShell façade 仍不委托工作线命令。
- 修正 `overview` 需要确认计数的 accepted/resolved/deferred status 兼容，让 accepted decision 不再被误计为待确认。
- 扩展 `agent-team-review-loop-smoke.ps1`、`overview-readonly-smoke.ps1`、`handoff-apply-smoke.ps1` 与 Go CLI fixtures，覆盖 verification visibility。
- 更新 README、skill、Go migration 文档、CHANGELOG 与本计划。

边界：本批只增强读层 visibility 与 smoke，不改变 ledger schema，不迁移历史 JSONL，不自动 dispatch reviewer，不启动 subagent，不写 confirmed/authority，不执行 full-trace/debug/inject/patch/dump/network；`gate -Apply` 仍只表示 pending-gate request 写入，不代表 heavy-tool 授权。

停止条件：需要改变 verification schema、把 reviewer verdict 自动转成 main decision、或把 Go 工作线命令纳入 façade 默认委托时暂停并重新评估。

验证：

```powershell
.\rekit\tests\agent-team-review-loop-smoke.ps1
.\rekit\tests\overview-readonly-smoke.ps1
.\rekit\tests\handoff-apply-smoke.ps1
go test ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过；PowerShell/Go `overview` 均显示最近 verification，PowerShell/Go lane `handoff` 均显示 `## verification`，review loop smoke 覆盖 `verification -> decision -> overview/handoff` 可见性。本批未写 confirmed/authority、未启动 subagent、未执行 heavy-tool，临时 case 均清理。

### Batch 52：continue digest 升级

状态：已完成。

目标：把 `/rekit continue` 的 `.rekit/runs/<run-id>/digest.md` 从计数摘要升级为可接手的结构化运行摘要，补齐 `docs/orchestration-plan.md` O4 要求的 inputs、route、packet refs、outputs、decisions 与 open risks。

实施范围：

- PowerShell `Invoke-RekitAuto` 收集本轮扫描到的 lane/workspace 输入 JSONL 与 workspace packet refs，在 digest 中写入 `## 输入`、`## route`、`## packet refs`、`## inputs`、`## outputs`、`## decisions`、`## open risks`。
- `status.json` 增加 `batchId`、`inputs`、`packetRefs` 与 `openRisks` 字段，便于后续 replay/debug 只读索引。
- 保留旧 `## 自动处理` 与 `## 需要关注` 区段，兼容已有接手习惯。
- 新增 `rekit/tests/continue-digest-smoke.ps1`，用临时 case 写入 feature lane outbox 与 workspace packet，验证 digest/status 结构、decision 行与 open risk 行。
- 更新 README、skill、rollout plan、CHANGELOG 与本计划。

边界：本批只增强 PowerShell continue digest 与 smoke；不改变 ledger schema，不迁移历史 JSONL，不自动 dispatch reviewer，不启动 subagent，不写 confirmed/authority，不执行 full-trace/debug/inject/patch/dump/network；`gate -Apply` 仍只表示 pending-gate request 写入，不代表 heavy-tool 授权。

停止条件：需要实现 replay 执行器、改变 continue 自动处理语义、把 Go `continue` 纳入实现范围、或把 open risk 自动转 gate/confirmed 时暂停并重新评估。

验证：

```powershell
.\rekit\tests\continue-digest-smoke.ps1
.\rekit\tests\agent-team-review-loop-smoke.ps1
.\rekit\rekit.ps1 -Command doctor
go test ./...
git diff --check
```

验证结果：全部通过；`continue` digest 现在包含输入、route、workspace packet refs、outputs、decisions 与 open risks，`status.json` 写入 `batchId`、`inputs`、`packetRefs`、`openRisks`；review loop 回归、doctor、Go tests 与 whitespace 检查均通过。reviewer 复核未发现高置信问题。本批未启动 subagent、未写 confirmed/authority、未执行 heavy-tool，临时 case已清理。

### Batch 53：Go note / continue 迁移再评估

状态：已完成（仅评估与文档回写，未改 runtime）。

目标：在 Batch E digest 升级后，重新评估 `note` 与 `continue` 是否适合继续迁移到 Go backend，并明确下一步低风险切片。

结论：

- `note` 建议作为下一条低风险 Go 手动路径：append-only ledger 写入/查询，目标仅 `.rekit/facts/*.jsonl`，可用现有 schema/enums、lane guard、eventId dedupe、`note -List` 展示做 parity；不纳入 PowerShell façade 委托。
- `continue` 暂缓迁移：仍包含 request routing、rule verifier、candidate decision、可选 authority append、digest/status、lane resume/checkpoint、board refresh 等多重副作用；其中 authority append 需要完整 G5 gate/parity tests 后再迁移。
- Batch E 的结构化 digest 已作为未来 Go `continue` 的 parity baseline：inputs、route、packet refs、outputs、decisions、open risks。

后续建议切片：

1. G4.5a Go `note -List` 只读路径：读取 9 类 facts JSONL，按 `-Kind`/`-Lane` 过滤并输出与 PowerShell 主要字段对齐的文本；只读、可先做。
2. G4.5b Go `note` append 手动路径：支持 9 种 kind 与 PowerShell enum/schema 校验、lane guard、eventId dedupe；只写 facts JSONL，不写 board/lane/handoff/authority。
3. G5 preflight：为 `continue` authority/routing/digest 建 parity tests，再决定是否实现 Go `continue -WhatIf` 或 apply 手动路径。

边界：本批不改 runtime、不迁移 schema、不改变 façade 委托集合、不写 case state、不启动 subagent、不执行 full-trace/debug/inject/patch/dump/network、不写 confirmed/authority。

验证：

```powershell
.\rekit\rekit.ps1 -Command doctor
go test ./...
git diff --check
```

验证结果：全部通过；本批仅更新迁移评估与路线文档，确认 `note` 下一步适合按只读 `-List` → append 手动路径迁移，`continue` 继续等待 G5 authority/routing/digest parity tests 完整后再迁移。未改 runtime、未写 case state、未启动 subagent、未执行 heavy-tool、未写 confirmed/authority。

### Batch 54：Go note -List 只读路径

状态：已完成。

目标：先把低风险的 ledger 查询能力迁移为 Go 手动路径，为后续 append 模式复用 facts JSONL 读取、kind 映射与 CLI guard。

实施范围：

- 新增 `internal/rekit/note`：读取 `.rekit/facts/*.jsonl`，支持 9 类 kind（observation、hypothesis、candidate、verification、decision、intervention、rollback、publication、request），按 `-Kind`/`-Lane` 过滤，每类最多展示 20 条。
- Go CLI 新增 `-Command note -List`、`-Kind` 与 note 侧 `-Lane` 过滤；无 `-List` 或组合 `-Apply`/`-WhatIf`/`-CreateCandidates` 时拒绝，避免把 Go `note` 误当写入入口。
- 输出展示 candidate confidence/status/risk、request gate detail、decision by、verification verifier/verdict/target、intervention/rollback target/status/reason/batch 等 PowerShell `note -List` 主要字段。
- 新增 Go tests 覆盖全量只读展示、`-Kind`/`-Lane` 过滤、invalid kind guard、write flag guard 与 `.rekit` snapshot 不变。
- 更新 `docs/go-runtime-migration.md`、CHANGELOG 与本计划。

边界：本批只实现手动 Go CLI 查询；不实现 append，不写 facts/board/lane/handoff/authority/confirmed，不执行 full-trace/debug/inject/patch/dump/network，不纳入 PowerShell façade 委托。

停止条件：需要改变 ledger schema、迁移历史 JSONL、改变 PowerShell `note` 行为、或将 `note` 纳入 façade 委托时暂停并重新评估。

验证：

```powershell
go test ./internal/rekit/cli ./internal/rekit/note
go test ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过；Go `note -List` 只读路径、过滤/guard tests、全量 Go tests、PowerShell doctor 与 whitespace 检查均通过。reviewer 复核未发现高置信问题。本批未写 case state、未启动 subagent、未执行 heavy-tool、未写 confirmed/authority。

### Batch 55：Go note append 手动路径

状态：已完成。

目标：在 G4.5a `note -List` 只读基础上补齐 Go `note` append 手动路径，让主会话可用 Go backend 显式写入 ledger event，同时保持 PowerShell façade 不委托、写入范围仅限 facts JSONL。

实施范围：

- `internal/rekit/note` 增加 `Append`：支持 9 类 kind（observation、hypothesis、candidate、verification、decision、intervention、rollback、publication、request），写入 `.rekit/facts/<kind>.jsonl`。
- Go CLI `-Command note` 无 `-List` 时进入 append；支持 `-WhatIf` 非写入预览；拒绝 `-Apply`/`-CreateCandidates`，避免把 note 混入通用写入模式。
- append 字段对齐 PowerShell：`actor`、`risk`、`related`、`confidence`、`decision`、`reason`、`status`、`batchId`、`evidenceRefs`、`target`、verification `verifier/verdict`、intervention `action/approvedBy/scope/expires`。
- 保持 PowerShell enum/schema guard：confidence、decision、status、verifier、verdict、intervention action；写入前校验 `.rekit/board.json` lane id；eventId 支持显式传入，否则按事件字段（含 createdAt）派生；跨 9 类 facts JSONL 去重。
- 新增 Go tests 覆盖 append 写入、9 类 kind、`-WhatIf` 非写入、eventId dedupe、missing kind/lane、unknown lane、非法 enum 与 unsupported write flags。
- 更新 `docs/go-runtime-migration.md`、CHANGELOG 与本计划。

边界：本批只写 facts JSONL；不写 board/lane/handoff/authority/confirmed，不执行 full-trace/debug/inject/patch/dump/network，不纳入 PowerShell façade 委托，不改变 PowerShell `note` 行为。

停止条件：需要自动写 authority/confirmed、改变 ledger schema、迁移历史 JSONL、改变 façade 委托集合、或实现 request routing/continue 自动化时暂停并重新评估。

验证：

```powershell
go test ./internal/rekit/cli ./internal/rekit/note
go test ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过；targeted Go tests、全量 Go tests、PowerShell doctor 与 whitespace 检查均通过。reviewer 复核发现 `docs/go-runtime-migration.md` façade 策略段仍写“未来补 Go note 手动路径”，已修正为 `note -List` 与 append 均可手动运行但仍不纳入 façade 委托。本批仅写 facts JSONL，不写 board/lane/handoff/authority/confirmed，不执行 heavy-tool。

### Batch 56：continue 迁移前测试网

状态：已完成。

目标：在实现 Go `continue -WhatIf` 之前，先为 PowerShell `continue` 的 authority/routing/digest 副作用补齐可重复 preflight smoke，作为后续 Go parity baseline。

实施范围：

- 新增 `rekit/tests/continue-preflight-smoke.ps1`，使用临时 `vmp-re` case 与 pack-declared CSV authority files，避免把 VMP authority 语义塞进 `_template`。
- 覆盖 authority append 正向路径：evidence、accepted verifier verdict、confidence 阈值、CSV schema、无冲突、allowlist、backup、bounded diff 与 publication/decision writes。
- 覆盖 authority append 拒绝路径：缺 evidence、confidence below threshold、schema invalid、authority key conflict、authorityFiles allowlist 拒绝、max rows。
- 覆盖 CSV append 后校验失败的 backup 恢复路径：通过本地注入 `Import-Csv` 失败验证 target CSV 恢复到 append 前内容，且 backup 已创建。
- 覆盖 request routing 幂等：同一 `requestId + sourceLane` 两个 request event 只写一条 target lane `tasks.jsonl` 与 `inbox.jsonl`。
- 覆盖 run digest/status parity：验证 inputs、route、packet refs、outputs、decisions、open risks 与 `status.json` summary/index 字段。
- 覆盖 `continue -WhatIf` no-write：快照临时 case 全树，验证不创建 runRoot、不写 facts、不刷新 board/lane resume/checkpoint、不改 authority CSV。
- 更新 `docs/go-runtime-migration.md`、CHANGELOG 与本计划，把 G5-preflight 从待补测试网改为已具备 baseline。

边界：本批只新增/运行临时 case smoke；不实现 Go `continue`，不改变 PowerShell `continue` 行为，不纳入 PowerShell façade Go 委托，不启动 subagent，不执行 full-trace/debug/inject/patch/dump/network，不写真实 case confirmed/authority。

停止条件：若后续要实现 Go `continue -Apply`、自动写 authority/confirmed 到真实 case、改变 policy schema、或将 `continue` 纳入 façade 委托，需重新评估并按独立批次推进。

验证：

```powershell
.\rekit\tests\continue-preflight-smoke.ps1
.\rekit\tests\continue-digest-smoke.ps1
.\rekit\rekit.ps1 -Command doctor
go test ./...
git diff --check
```

验证结果：全部通过；`continue-preflight-smoke.ps1`、既有 `continue-digest-smoke.ps1`、PowerShell doctor、全量 Go tests 与 `git diff --check` 均通过（仅出现既有 LF/CRLF warning）。reviewer 复核发现 accepted verifier verdict 缺少独立失败路径断言，已补 `Invoke-RekitAuthorityAppend` 直测：条件均满足但缺 accepted verifier 时必须返回 `missing accepted verifier verdict` 且不追加 CSV。本批只新增临时 case smoke 与文档记录；未实现 Go `continue`，未改变 PowerShell `continue` 行为，未纳入 façade 委托，未启动 subagent，未执行 heavy-tool，未写真实 case confirmed/authority。

### Batch 57：Go continue WhatIf 预览

状态：已完成。

目标：基于 Batch 56 preflight baseline，实现 Go `continue -WhatIf` 非写入预览路径，让维护者能在不触发 PowerShell `continue` 写入副作用的情况下预览 lane outbox/workspace 事件收集、request routing 与 authority candidate 决策。

实施范围：

- 新增 `internal/rekit/workstream/continue.go`，在 Go `workstream` 包内复用 board/lane helper，读取既有 attached case、`.rekit/board.json`、lane `outbox.jsonl`、workspace JSONL、candidate CSV 与 packet markdown。
- 新增 CLI `-Command continue -WhatIf` 手动路径，支持 positional selector 与 `-Lane` selector；缺 selector 且存在多个 open lane 时拒绝，避免误选工作线。
- 输出 JSON preview：`isMutation=false`、`applied=false`、`requiresConfirmation=true`，包含 selected lane、inputs、packetRefs、summary、events、wouldWrites、openRisks、blockedActions 与 nextSteps。
- 预览 request routing：只报告会 append 的 target lane `tasks.jsonl` / `inbox.jsonl`，不实际写入。
- 预览 authority candidate：按 manifest `authorityFiles` allowlist、policy evidence/verifier/confidence/schema/conflict/max rows 等 gate 生成 accept/defer 决策与 wouldWrites；只报告 authority CSV、run backup/diff、facts decision/publication 等潜在写入，不实际写入。
- 新增 `rekit/tests/continue-whatif-smoke.ps1`，使用临时 `vmp-re` case 覆盖 Go preview、全树 no-write、unsupported apply guard 与 PowerShell façade fallback。
- 更新 `docs/go-runtime-migration.md`、CHANGELOG 与本计划。

边界：本批只实现 Go CLI 手动 `continue -WhatIf` preview；不实现 Go `continue -Apply`，不写 facts/run/board/lane/handoff/authority/confirmed，不改变 PowerShell `continue` 行为，不纳入 PowerShell façade 委托，不启动 subagent，不执行 full-trace/debug/inject/patch/dump/network。

停止条件：若后续要实现 Go `continue -Apply`、自动写 authority/confirmed 到真实 case、改变 policy schema、或将 `continue` 纳入 façade 委托，需重新评估并按独立批次推进。

验证：

```powershell
go test ./internal/rekit/workstream ./internal/rekit/cli
go test ./...
.\rekit\tests\continue-whatif-smoke.ps1
.\rekit\tests\continue-preflight-smoke.ps1
.\rekit\tests\continue-digest-smoke.ps1
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过；targeted Go tests、全量 Go tests、`continue-whatif-smoke.ps1`、Batch 56 `continue-preflight-smoke.ps1`、既有 `continue-digest-smoke.ps1`、PowerShell doctor 与 `git diff --check` 均通过（仅出现既有 LF/CRLF warning）。reviewer 复核发现 preview summary 使用 `authorityApplied` 容易误导为已写入 authority，已改为保持 `authorityApplied=0` 并新增 preview-only `authorityWouldAppend` 计数，测试与 smoke 同步断言。本批未实现 Go `continue -Apply`，未改变 PowerShell `continue` 行为，未纳入 façade 委托，未启动 subagent，未执行 heavy-tool，未写 facts/run/board/lane/handoff/authority/confirmed。

### Batch 58：bounded dispatch 可观测性增强

状态：已完成。

目标：在不把 runtime 变成 agent 调度器的前提下，增强 `plan-subagents` review artifacts，让主会话能审计 route 选择、shard 状态、review loop 责任和 verdict 写回闭环。

实施范围：

- Go `internal/rekit/subagents` 的 `Result` / `Packet` 增加 `observability` 与 `reviewLoop`：包含 dispatch mode、route debug、review artifact 路径、每个 shard 初始 `planned` 状态、blocked runtime actions、spawn/merge owner、verdict writeback 与 completion criteria。
- PowerShell `Write-RekitSubagentPlan` 输出同名字段，保持 façade fallback 与 Go 手动路径的 packet/summary parity。
- `summary.md` 增加 bounded dispatch observability 区段，列出 shard status、blocked runtime actions 和 completion criteria，便于主 agent 启动 reviewer 前检查范围。
- 更新 `plan-subagents` Go tests 与 smoke，覆盖 Go packet/summary observability、PowerShell façade fallback packet/summary observability。
- 更新 README、subagents policy、Go migration 文档、reference absorption 与 CHANGELOG。

边界：本批只写 review artifacts；不自动 spawn subagent，不写 facts/board/lane/handoff/authority/confirmed，不执行 full-trace/debug/inject/patch/dump/network，不改变 PowerShell façade 委托集合。

停止条件：如果要让 `/rekit continue` 或 runtime 自动 spawn reviewer、自动合并 verdict、自动写 confirmed/authority，必须按独立批次重新评估并确认。

验证：

```powershell
go test ./internal/rekit/subagents ./internal/rekit/cli
go test ./...
.\rekit\tests\plan-subagents-smoke.ps1
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过；targeted Go tests、全量 Go tests、`plan-subagents-smoke.ps1`、PowerShell doctor 与 `git diff --check` 均通过（仅出现既有 LF/CRLF warning）。reviewer 复核发现两个问题：未知 `-TaskType` fallback 时 `routeDebug.selectedBy` 会误报为 `taskType`，已改为仅在实际匹配 route taskTypes 时标记 `taskType`，否则标记 `manifest-default` 并补 Go 测试；PowerShell façade fallback smoke 对 blocked actions/review loop/no-write 边界断言不足，已补 packet/summary 与 case state no-write 断言。本批只写 review artifacts，未启动 subagent，未写 facts/board/lane/handoff/authority/confirmed，未执行 heavy-tool。

### Batch 59：项目定位纠偏（security Agent Team）

状态：已完成。

目标：将项目顶层定位从 RE-only 纠偏为面向网络安全研究与安全工程任务的 Claude Code Agent Team 框架，明确 `vmp-re` 是首个成熟 pack / 验证场而不是最终边界，并为新会话接手提供一致文档入口。

实施范围：

- README、根目录 `CLAUDE.md` 与 `docs/vision.md` 改为安全 Agent Team 框架定位，保留 `vmp-re` 作为当前成熟示例。
- `docs/reference-absorption.md`、`docs/agent-team-usage.md`、`docs/pack-authoring.md`、`docs/case-migration.md`、`docs/orchestration-plan.md` 与 `docs/design.md` 同步非 RE-only 边界。
- `.claude/skills/rekit/SKILL.md`、case shim 模板和 `packs/_template/manifest.yml` 扩展为 security pack 表述。
- CHANGELOG 记录本批定位纠偏，避免后续新会话沿旧 RE-only 目标继续推进。

边界：本批仅做文档与模板描述纠偏；不改 runtime 行为，不新增真实 pack，不创建或修改真实 case state，不执行 full-trace/debug/inject/patch/dump/network，不写 confirmed/authority，不改变 PowerShell façade / Go backend 委托集合。

停止条件：若后续要改变实际产品方向、引入新安全领域 pack 的 runtime/schema 迁移、或把某类安全任务做成自动执行平台，必须按独立批次重新评估并确认。

验证：

```powershell
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Pack _template
go test ./...
git diff --check
```

验证结果：全部通过；`doctor` / `_template` doctor 均通过，`_template` 仅保留既有 `manifest has no subagentRoutes` warning；`go test ./...` 通过；`git diff --check` 通过（仅出现既有 LF/CRLF warning）。本批只改文档与模板描述，未改 runtime，未新增真实 pack，未创建 case state，未执行 heavy-tool，未写 confirmed/authority。

新会话接手：

- 先读 `README.md`、`CLAUDE.md`、`docs/vision.md` 顶部和本 Batch 59，按“网络安全研究 / 安全工程 Agent Team 框架”理解项目目标。
- `vmp-re` 是当前首个成熟 pack 和验证场；后续多 pack 扩展可考虑 `web-security`、`malware-analysis`、`vuln-research`、`ctf`、`unpack-pe`、`android-native`、`ollvm` 等，但不要把 RE-only 当成最终目标。
- 后续继续推进前，仍遵守 review-first、临时 case 验证、Go façade 默认不接管、heavy-tool/confirmed/authority 需明确确认等边界。

### Batch 60：pack template Agent Team route 契约

状态：已完成。

目标：把 Batch 59 纠偏后的多安全领域 pack 扩展能力从“文档说可扩展”推进到“模板自带可验证 Agent Team route”。新 pack 复制 `_template` 后应立即具备 pack-neutral `subagentRoutes`、route reference、review packet 输出契约和 Go/PowerShell `plan-subagents` 验证路径，而不是先遇到 `manifest has no subagentRoutes` 再补。

实施范围：

- `_template` 新增 `references/template/agent-team.md`，定义 main / feature / reviewer / tooling 边界、默认 route、packet 输出契约、review-first 门禁和新 pack 改写 checklist。
- `_template/manifest.yml` 将 `agent-team.md` 纳入 managed/promote files，并新增 `_template:bounded-review` 与 `_template:lane-feature-analysis` 两条 pack-neutral `subagentRoutes`。
- `docs/pack-authoring.md` 将 `agent-team.md` 与 `subagentRoutes` 升级为新 pack 最小结构与 manifest checklist，明确 route id、taskTypes、shardBasis、权限、mainAgentOwns 和 outputContract 要求。
- Go manifest schema 与 PowerShell doctor 增加 subagent route 硬校验：route id 唯一、必填字段齐全、分片数字为正、reference 留在 managed/template/local 边界内。
- 更新 `plan-subagents` Go tests 与 smoke：`_template` 不再是 missing routes guard，而应能生成 route packet / summary；Go 与 PowerShell fallback 均保持只写 review artifacts、不写 board/facts/lanes/authority。

边界：本批不新增真实领域 pack，不改变 PowerShell façade 委托集合，不自动 spawn subagent，不写 facts/board/lane/handoff/authority/confirmed，不执行 full-trace/debug/inject/patch/dump/network。

停止条件：若后续要把 `plan-subagents` 变成自动 dispatch、让 runtime 自动合并 verdict、或创建真实 `web-security` / `malware-analysis` 等 pack，应作为独立批次重新评估。

验证：

```powershell
go test ./internal/rekit/manifest ./internal/rekit/cli
go test ./...
.\rekit\tests\plan-subagents-smoke.ps1
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Pack _template
git diff --check
```

验证结果：全部通过；`plan-subagents-smoke.ps1` 覆盖 vmp-re Go path、_template Go path、_template PowerShell fallback path 与 no-write 边界；targeted Go tests、全量 `go test ./...`、默认 doctor、`_template` doctor、`git diff --check` 均通过（仅出现既有 LF/CRLF warning）。只读 reviewer 未发现高置信问题。本批未新增真实 pack、未改变 façade 委托集合、未启动 subagent、未写 facts/board/lane/handoff/authority/confirmed、未执行 heavy-tool。

### Batch 61：首个非 RE pack 骨架 web-security

状态：已完成。

目标：在 Batch 60 的 pack-neutral route 契约基础上，新增首个非 RE pack 骨架 `web-security`，用 Web/API 安全评估验证多领域安全 Agent Team 扩展能力，避免项目继续只停留在 `vmp-re` 验证场。

实施范围：

- 新增 `packs/web-security/manifest.yml`、`CLAUDE.local.snippet.md`、policy overlay 空 registry、reference docs、task handoff template、tooling catalog 与两条 recipes。
- Web/API reference docs 覆盖 scope baseline、轻到重路线、pending-gate、endpoint/finding bounded review、request replay 边界、sidecar 与敏感信息留在 case-local 的规则。
- manifest 声明 `web-security:bounded-review` 与 `web-security:feature-analysis` 两条 route，验证非 RE pack 的 `plan-subagents` packet / summary 与 PowerShell fallback。
- 更新 README、CLAUDE.md、vision、reference absorption，把 `web-security` 标为首个非 RE pack 骨架，而不是成熟自动化扫描平台。
- 新增 Go manifest schema test 与 `web-security-pack-smoke.ps1`，覆盖 Go/PowerShell doctor、Go init、case doctor、Go/PowerShell `plan-subagents`、promote review 不被占位文案 deny 误阻断和 no-write 边界。

边界：本批只新增最小 pack 骨架和验证；不执行真实网络请求、扫描、fuzz、登录尝试、exploit replay 或数据导出；不把 Web/API 工具变成硬依赖；不写真实 case confirmed/authority；不改变 PowerShell façade 委托集合。

停止条件：若后续要把 `web-security` 扩展成主动扫描 adapter、认证 replay runtime、漏洞报告 authority schema 或真实外部工具执行，应按独立批次评估 gate 与授权边界。

验证：

```powershell
go test ./internal/rekit/manifest ./internal/rekit/cli
go test ./...
.\rekit\tests\web-security-pack-smoke.ps1
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Pack _template
.\rekit\rekit.ps1 -Command doctor -Pack web-security
git diff --check
```

验证结果：全部通过；targeted Go tests、全量 `go test ./...`、`web-security-pack-smoke.ps1`、默认 doctor、`_template` doctor、`web-security` doctor、`git diff --check` 均通过（仅出现既有 LF/CRLF warning）。只读 reviewer 发现 `authorization:` 占位会被 deny pattern 误阻断，已改为 `auth_scope` 并收窄 Authorization header deny 规则，smoke 增加 promote review 断言确保 safe workflow 修改可进入 `candidate-after-llm-review`。本批未执行真实网络请求、扫描、fuzz、登录尝试、exploit replay 或数据导出；未写真实 case confirmed/authority；未改变 façade 委托集合。

### Batch 62：pack inventory matrix

状态：已完成。

目标：在 `vmp-re`、`_template` 和 `web-security` 并存后，新增只读 pack inventory 矩阵，让维护者不用逐个指定 `-Pack` 即可发现当前 kit 内所有 pack，并快速看到 maturity、schema、route、managed/tooling、authority lane 与版本信息，为后续多安全领域 pack 扩展提供 Go-first 验证入口。

实施范围：

- Go manifest 层新增 `List` / `PackSummary`，扫描 `packs/*/manifest.yml` 并复用 manifest schema 校验；同时收紧 pack id，拒绝路径穿越、绝对路径或含分隔符的 pack 名，并保持 `managedBlock` 显式字段校验与 PowerShell doctor 对齐。
- Go CLI 新增 `-Command packs`，输出只读 TSV 矩阵；PowerShell façade/fallback 同步新增 `/rekit packs`。
- 显式 `REKIT_GO_ENABLE=1` 时，PowerShell façade 可委托 Go `packs`，因该命令只读且不触碰 case state。
- 新增 `rekit/tests/pack-inventory-smoke.ps1`，覆盖 Go、PowerShell fallback 和 Go façade 三条路径，断言 `_template`、`vmp-re`、`web-security` 行计数与 maturity；用 `REKIT_GO_EXE` sentinel 证明 façade 确实委托 Go，且 `REKIT_GO_DISABLE=1` 会回退。
- 更新 README、skill、pack authoring、Go migration、CLAUDE.md 与 CHANGELOG，说明 `packs` 是维护者/排障入口，不是 case 日常主流程。

边界：本批只读列出 pack inventory；不初始化 case、不写 board/facts/lanes/handoff/authority/confirmed、不执行外部工具、不改变 sync/promote review-first 语义；除 `packs` 外不扩大 façade 委托集合。

验证：

```powershell
go test ./internal/rekit/manifest ./internal/rekit/cli
go test ./...
.\rekit\tests\pack-inventory-smoke.ps1
.\rekit\rekit.ps1 -Command packs
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Pack _template
.\rekit\rekit.ps1 -Command doctor -Pack web-security
.\rekit\tests\facade-smoke.ps1 -CaseRoot <fresh-temp-vmp-case> -Pack vmp-re
git diff --check
```

验证结果：全部通过。`go test ./internal/rekit/manifest ./internal/rekit/cli`、`go test ./...`、`pack-inventory-smoke.ps1`、`/rekit packs`、默认 / `_template` / `web-security` 三组 doctor、fresh temp vmp case façade smoke 与 `git diff --check` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。第一次直接复用旧 `agent-team-dryrun` façade fixture 时因该临时 case 的 case-local shim 已过期而被 doctor 拒绝，随后使用本批新建并 `start handler-0x40a010` 的临时 case 验证通过；该失败不是 Batch 62 代码回归。

### Batch 63：manifest 显式 maturity 字段

状态：已完成。

目标：将 Batch 62 `/rekit packs` 中的 pack maturity 从名称/description 启发式推断升级为 `manifest.yml` 显式字段，避免后续更多安全领域 pack 依赖描述文本判定成熟度。

实施范围：

- `_template`、`vmp-re`、`web-security` 三个 pack manifest 均新增 `maturity` 字段，当前取值分别为 `template`、`mature`、`skeleton`。
- Go manifest parser 读取 `maturity`，`PackSummary` 优先使用显式字段；`ValidateSchema` 要求 maturity 非空且属于 `mature` / `template` / `skeleton` / `experimental`。
- PowerShell manifest parser、inventory 和 doctor schema 校验同步读取并校验显式 maturity，与 Go schema 判定保持一致。
- `docs/pack-authoring.md` 更新最小 manifest 示例与验证标准；`CHANGELOG.md` 与 Go runtime 迁移说明同步记录。

边界：本批只调整 pack manifest schema、只读 inventory 显示和文档；不创建/修改 case，不写 board/facts/lanes/handoff/authority/confirmed，不改变 sync/promote review-first 语义，不扩大 façade 委托集合。

验证：

```powershell
go test ./internal/rekit/manifest ./internal/rekit/cli
go test ./...
.\rekit\tests\pack-inventory-smoke.ps1
.\rekit\rekit.ps1 -Command packs
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Pack _template
.\rekit\rekit.ps1 -Command doctor -Pack web-security
git diff --check
```

验证结果：全部通过。`pack-inventory-smoke.ps1` 覆盖 Go、PowerShell fallback、Go façade 委托 sentinel，以及临时缺失/非法 maturity pack 的 `schema=error` 行与错误文本；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。自审发现并修复：pack authoring 示例的 inline comment 会被简易 YAML parser 当成值、说明性 schema 漏列 `maturity`、inventory 对缺失 maturity 不应继续启发式推断。

### Batch 64：pack inventory JSON 输出

状态：已完成。

目标：在 Batch 62-63 的 `/rekit packs` TSV 矩阵基础上，补齐机器可读 JSON 输出，让 CI、新会话接手、pack-neutral 编排和后续多 pack tooling 能直接消费同一 pack inventory，而不需要解析表格文本。

实施范围：

- Go CLI `packs` 增加 `-Format json` / `--format json`，输出只读 envelope：`command`、`schemaVersion`、`isMutation`、`packCount` 与 `packs[]`。
- PowerShell façade / fallback 增加 `-Format` 参数，`/rekit packs -Format json` 输出与 Go 对齐的 envelope；显式 `REKIT_GO_ENABLE=1` 时将 `-Format` 透传给 Go backend。
- `PackSummary.error` 在 JSON 中稳定出现，合法 pack 为空字符串，非法/缺失 schema 的 pack 保留错误文本。
- `rekit/tests/pack-inventory-smoke.ps1` 覆盖 Go、PowerShell fallback、Go façade 的 JSON 输出，以及缺失/非法 maturity pack 的 JSON `schemaValid=false` 与错误文本。
- 更新 README、skill、pack authoring、Go runtime migration 与 CHANGELOG，说明 `-Format json` 是只读机器可读 inventory 输出。

边界：本批只增强只读 pack inventory 输出；不创建/修改 case，不写 board/facts/lanes/handoff/authority/confirmed，不改变 sync/promote review-first 语义，不扩大 façade 委托命令集合。

验证：

```powershell
go test ./internal/rekit/manifest ./internal/rekit/cli
go test ./...
.\rekit\tests\pack-inventory-smoke.ps1
.\rekit\rekit.ps1 -Command packs
.\rekit\rekit.ps1 -Command packs -Format json
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。首次验证中 `pack-inventory-smoke.ps1` 暴露 PowerShell 字符串中 `$Pack:` 被解析为变量作用域的问题，已改为 `${Pack}:` 后重跑通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 65：status JSON 输出

状态：已完成。

目标：延续 Batch 64 的机器可读 envelope 模式，为只读 `/rekit status` 增加 `-Format json`，让 kit/case 绑定状态、manifest counts 与 moved-case 信号可被 CI、新会话接手和后续 Go-first orchestration 直接消费，而不需要解析人类文本。

实施范围：

- Go CLI `status` 接收 `-Format json` / `--format json`，默认文本输出保持兼容；JSON 输出 envelope 包含 `command`、`schemaVersion`、`isMutation`、runtime/template root、pack、target、`targetProvided`、`mode`，并按 kit/case 模式填充 `manifest` 或 `case`。
- PowerShell façade / fallback 的 `status` 同步支持 `-Format json`，显式 `REKIT_GO_ENABLE=1` 时将 `-Format` 透传给 Go backend。
- `rekit/tests/pack-inventory-smoke.ps1` 扩展为覆盖 Go、PowerShell fallback 和 Go façade 的 kit-mode status JSON，并用 sentinel 验证 façade status format 参数透传。
- 更新 README、skill、Go runtime migration 与 CHANGELOG，说明 status JSON 是只读机器可读状态输出。

边界：本批只增强只读 status 输出；不创建/修改 case，不写 board/facts/lanes/handoff/authority/confirmed，不改变 sync/promote review-first 语义，不新增写入型 façade 委托。

验证：

```powershell
go test ./internal/rekit/cli
go test ./...
.\rekit\tests\pack-inventory-smoke.ps1
.\rekit\rekit.ps1 -Command status
.\rekit\rekit.ps1 -Command status -Format json
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`go test ./internal/rekit/cli`、`go test ./...`、`pack-inventory-smoke.ps1`、`/rekit status`、`/rekit status -Format json`、`/rekit doctor` 与 `git diff --check` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 66：doctor / validate JSON 输出

状态：已完成。

目标：延续 Batch 64-65 的机器可读 envelope 模式，为只读 `/rekit doctor` / `validate` 增加 `-Format json`，让 pack/case 验证 rows、mode、summary 与 valid 状态可被 CI、新会话接手和后续编排直接消费，而不需要解析人类文本。

实施范围：

- Go CLI `doctor` / `validate` 接收 `-Format json` / `--format json`，默认文本输出保持兼容；JSON 输出 envelope 包含 `command`、`schemaVersion`、`isMutation`、pack、target、`mode`、`valid`、`summary` 与 `rows[]`。
- Go `doctor.Row` 增加稳定 JSON tags；`doctor` 与 `validate` 共享同一只读验证输出路径。
- PowerShell façade / fallback 的 `doctor` / `validate` 同步支持 `-Format json`，显式 `REKIT_GO_ENABLE=1` 时将 `-Format` 透传给 Go backend。
- `rekit/tests/pack-inventory-smoke.ps1` 扩展为覆盖 Go、PowerShell fallback 和 Go façade 的 kit-mode doctor / validate JSON，并用 sentinel 验证 façade format 参数透传。
- 更新 README、skill、Go runtime migration 与 CHANGELOG，说明 doctor / validate JSON 是只读机器可读验证输出。

边界：本批只增强只读验证输出；不创建/修改 case，不写 board/facts/lanes/handoff/authority/confirmed，不改变 sync/promote review-first 语义，不新增写入型 façade 委托。

验证：

```powershell
go test ./internal/rekit/cli ./internal/rekit/doctor
go test ./...
.\rekit\tests\pack-inventory-smoke.ps1
.\rekit\rekit.ps1 -Command doctor -Format json
.\rekit\rekit.ps1 -Command validate -Format json
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`go test ./internal/rekit/cli ./internal/rekit/doctor`、`go test ./...`、`pack-inventory-smoke.ps1`、`/rekit validate -Format json`、`/rekit doctor` 与 `git diff --check` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 67：note list JSON 输出

状态：已完成。

目标：延续 Batch 64-66 的机器可读 envelope 模式，为只读 `/rekit note -List` 增加 `-Format json`，让 ledger events 可按 kind/lane 被 CI、新会话接手、review loop smoke 和后续编排直接消费，而不需要解析人类文本。

实施范围：

- Go `note` 增加 `ListResult` / `ListGroup`，`note -List -Format json` 输出 `schemaVersion/command/caseRoot/repoRoot/pack/isMutation/kind/lane/eventCount/groups[]`，默认文本输出保持兼容。
- PowerShell `Invoke-RekitNote -List` 同步支持 `-Format json`，复用同一过滤和 maxRows 语义；顶层 `rekit.ps1` 将 `-Format` 透传给 note 命令。
- Go CLI tests 覆盖 note list JSON 只读、kind/lane 过滤与 unsupported format guard；review-loop smoke 增加 PowerShell `note -List -Format json` 验证。
- 更新 README、skill、Go runtime migration 与 CHANGELOG，说明 note list JSON 是只读机器可读 ledger event 输出。

边界：本批只增强只读 ledger 查询输出；不改变 note append schema、不写 board/facts/lanes/handoff/authority/confirmed、不启动 subagent、不执行 heavy-tool、不改变 sync/promote review-first 语义，不新增 façade 委托集合。

验证：

```powershell
go test ./internal/rekit/cli ./internal/rekit/note
go test ./...
.\rekit\tests\agent-team-review-loop-smoke.ps1
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`go test ./internal/rekit/cli ./internal/rekit/note`、`go test ./...`、`agent-team-review-loop-smoke.ps1`、`/rekit doctor` 与 `git diff --check` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 68：overview JSON 输出

状态：已完成。

目标：延续 Batch 64-67 的机器可读 envelope 模式，为 `/rekit overview` 增加 `-Format json`，让新会话接手、CI、smoke 和后续 orchestration 可直接消费 case 的工作线、共享事实计数、review-loop 摘要与建议下一步，而不需要解析人类文本。

实施范围：

- Go `overview` 增加 `BuildInventory` 与 typed envelope，`overview -Format json` 输出 `schemaVersion/command/caseRoot/repoRoot/pack/isMutation/automationMode/lanes/counts/sections/nextSteps`，默认文本输出保持兼容且继续只读。
- PowerShell `Show-RekitOverview` 同步支持 `-Format json`，顶层 `rekit.ps1` 透传 `-Format`；显式 `REKIT_GO_ENABLE=1` 时仍不把 `overview` 纳入 Go façade 委托集合。
- Go CLI tests 覆盖 overview JSON 只读、counts/sections/lanes/nextSteps 与 unsupported format guard；overview readonly smoke 增加 Go CLI 与 PowerShell fallback JSON 验证。
- 更新 README、skill、Go runtime migration 与 CHANGELOG，说明 overview JSON 是机器可读项目总览输出。

边界：本批只增强 overview 输出形态；不改变 case 初始化语义、不新增 façade Go 委托、不写 facts/lanes/handoff/authority/confirmed、不启动 subagent、不执行 heavy-tool、不改变 sync/promote review-first 语义。

验证：

```powershell
go test ./internal/rekit/cli ./internal/rekit/overview
go test ./...
.\rekit\tests\overview-readonly-smoke.ps1
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`go test ./internal/rekit/cli ./internal/rekit/overview`、`go test ./...`、`overview-readonly-smoke.ps1`、`/rekit doctor` 与 `git diff --check` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 69：handoff preview JSON 输出

状态：已完成。

目标：延续 Batch 64-68 的机器可读 envelope 模式，为 PowerShell fallback 的 `/rekit handoff -WhatIf` 增加 `-Format json`，让接手自动化和审查流程可在不写 handoff 文件的情况下消费项目级/工作线级 handoff 写入计划。

实施范围：

- PowerShell `Write-RekitHandoff` 支持 `-WhatIf -Format json`，输出 `schemaVersion/command/caseRoot/repoRoot/pack/isMutation/applied/requiresConfirmation/selector/project/lane/writes/blockedActions/nextSteps` envelope。
- 顶层 `rekit.ps1` 将 `-Format` 透传给 handoff；显式 `REKIT_GO_ENABLE=1` 时仍不把 handoff 纳入 Go façade 委托集合。
- handoff apply smoke 覆盖 façade fallback JSON preview、lane selector resolve、no-write snapshot 与 apply JSON format guard。
- 更新 README、skill、Go runtime migration 与 CHANGELOG，说明 handoff JSON preview 是只读/非写入预览输出。

边界：本批只增强 PowerShell fallback 的 `handoff -WhatIf` 预览输出；不改变 Go handoff apply/preview schema、不新增 façade Go 委托、不写 facts/lanes/handoff/authority/confirmed、不启动 subagent、不执行 heavy-tool、不改变 sync/promote review-first 语义。

验证：

```powershell
go test ./internal/rekit/cli ./internal/rekit/workstream
go test ./...
.\rekit\tests\handoff-apply-smoke.ps1
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`go test ./internal/rekit/cli ./internal/rekit/workstream`、`go test ./...`、`handoff-apply-smoke.ps1`、`/rekit doctor` 与 `git diff --check` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 70：start preview JSON 输出

状态：已完成。

目标：延续 Batch 64-69 的机器可读 envelope 模式，为 PowerShell fallback 的 `/rekit start -WhatIf` 增加 `-Format json`，让自动化和新会话接手可在不创建 lane、board、facts 或 workspace 的情况下消费 feature workstream 创建/进入计划。

实施范围：

- PowerShell `Invoke-RekitStart` 支持 `-WhatIf -Format json`，输出 `schemaVersion/command/caseRoot/repoRoot/pack/isMutation/applied/requiresConfirmation/lane/writes/blockedActions/nextSteps` envelope，并与 Go `StartPreview` 的核心 schema 对齐。
- 顶层 `rekit.ps1` 将 `-Format` 透传给 start；显式 `REKIT_GO_ENABLE=1` 时仍不把 start 纳入 Go façade 委托集合。
- start apply smoke 覆盖 façade fallback JSON preview、no-write snapshot、lane write action 与 apply JSON format guard。
- 更新 README、skill、Go runtime migration 与 CHANGELOG，说明 start JSON preview 是只读/非写入预览输出。

边界：本批只增强 PowerShell fallback 的 `start -WhatIf` 预览输出；不改变 Go start apply/preview schema、不新增 façade Go 委托、不写 board/facts/lanes/workspace/handoff/authority/confirmed、不启动 subagent、不执行 heavy-tool、不改变 sync/promote review-first 语义。

验证：

```powershell
go test ./internal/rekit/cli ./internal/rekit/workstream
go test ./...
.\rekit\tests\start-apply-smoke.ps1
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`go test ./internal/rekit/cli ./internal/rekit/workstream`、`go test ./...`、`start-apply-smoke.ps1`、`/rekit doctor` 与 `git diff --check` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 71：continue preview JSON 输出

状态：已完成。

目标：延续 Batch 64-70 的机器可读 envelope 模式，为 PowerShell fallback 的 `/rekit continue -WhatIf` 增加 `-Format json`，让自动化和新会话接手可在不创建 run、facts、lane resume/checkpoint、board 或 authority 写入的情况下消费 continue 收集、路由、candidate verification 与 authority append 计划。

实施范围：

- PowerShell `Invoke-RekitContinue` 支持 `-WhatIf -Format json`，输出 `schemaVersion/command/caseRoot/repoRoot/pack/isMutation/applied/requiresConfirmation/selector/lane/runId/batchId/summary/inputs/packetRefs/events/openRisks/wouldWrites/blockedActions/nextSteps` envelope。
- 新增 PowerShell preview helper，复用既有 policy、lane output、rule verifier、route target、authority append preflight 逻辑，生成只读 `wouldWrites` 与 per-event decisions；apply + JSON 明确拒绝。
- 顶层 `rekit.ps1` 将 `-Format` 透传给 continue；显式 `REKIT_GO_ENABLE=1` 时仍不把 continue 纳入 Go façade 委托集合。
- `continue-whatif-smoke.ps1` 覆盖 Go continue preview、PowerShell façade fallback text no-write、PowerShell fallback JSON preview、route/authority wouldWrites、no-write snapshot 与 apply JSON format guard。
- 更新 README、skill、Go runtime migration 与 CHANGELOG，说明 continue JSON preview 是只读/非写入预览输出。

边界：本批只增强 PowerShell fallback 的 `continue -WhatIf` 预览输出；不改变 Go continue preview schema、不新增 façade Go 委托、不写 run/facts/lanes/board/handoff/authority/confirmed、不启动 subagent、不执行 heavy-tool、不改变 sync/promote review-first 语义。

验证：

```powershell
go test ./internal/rekit/cli ./internal/rekit/workstream
go test ./...
.\rekit\tests\continue-whatif-smoke.ps1
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`go test ./internal/rekit/cli ./internal/rekit/workstream`、`go test ./...`、`continue-whatif-smoke.ps1`、`/rekit doctor` 与 `git diff --check` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 72：工作线 JSON preview façade 委托

状态：已完成。

目标：在 Batch 69-71 已补齐工作线 `-WhatIf -Format json` 机器可读预览后，缩小 PowerShell preview 复制逻辑的长期压力：显式启用 Go backend 时，让 `start`、`handoff`、`continue` 的 JSON preview 从公共 PowerShell façade 直接委托到 Go backend，同时保留文本预览和写入路径的 PowerShell fallback。

实施范围：

- `rekit/rekit.ps1` 将 `start`、`handoff`、`continue` 纳入显式 Go 安全集合，但只允许 `-WhatIf -Format json` 且未带 `-Apply` / `-CreateCandidates` 时委托。
- Go façade argument builder 对工作线命令复用 PowerShell action target/args 解析，将 case target、selector/name、`-Format json`、`-WhatIf` 和 start `-Force` 转发给 Go backend。
- 文本预览（如 `start -WhatIf login`、`handoff -WhatIf login`、`continue -WhatIf`）仍回退 PowerShell，避免改变用户可读输出；apply/write 路径仍按既有 PowerShell 或手动 Go CLI 边界执行。
- start / handoff / continue smoke 增加 JSON preview 委托断言：`nextSteps` 中可见 Go backend marker，并继续验证 no-write snapshot、fallback 文本预览与 apply JSON guard。
- 更新 README、skill、Go runtime migration 与 CHANGELOG，说明该委托只覆盖非写入 JSON preview，不代表工作线写入命令默认迁入 Go。

边界：本批只调整显式 `REKIT_GO_ENABLE=1` 下的非写入 JSON preview 委托；不默认启用 Go、不委托 overview/plan-subagents/note、不委托任何 start/handoff/continue 写入路径、不写 facts/run/lanes/board/handoff/authority/confirmed、不启动 subagent、不执行 heavy-tool、不改变 sync/promote review-first 语义。

验证：

```powershell
go test ./internal/rekit/cli ./internal/rekit/workstream
go test ./...
.\rekit\tests\start-apply-smoke.ps1
.\rekit\tests\handoff-apply-smoke.ps1
.\rekit\tests\continue-whatif-smoke.ps1
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`go test ./internal/rekit/cli ./internal/rekit/workstream`、`go test ./...`、`start-apply-smoke.ps1`、`handoff-apply-smoke.ps1`、`continue-whatif-smoke.ps1`、`/rekit doctor` 与 `git diff --check` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 73：overview JSON façade 委托

状态：已完成。

目标：继 Batch 72 将工作线 JSON preview 纳入显式 Go façade 后，继续减少 PowerShell JSON 渲染复制面：在不破坏 `/rekit overview` 首次初始化 board 的前提下，让已初始化 case 的 `overview -Format json` 经 PowerShell façade 委托 Go backend。

实施范围：

- `rekit/rekit.ps1` 将 `overview` 纳入显式 Go 安全集合，但只允许 `-Format json`、未带 `-WhatIf`/`-Apply`/`-CreateCandidates`、目标看起来是 attached case 且 `.rekit/board.json` 已存在时委托。
- `Get-RekitGoTarget` 与 argument builder 对 `overview` 转发 resolved target 与 `-Format json`。
- 缺 board 或文本 overview 继续走 PowerShell fallback：缺 board 时仍可由 PowerShell `overview` 初始化 `.rekit/board.json`，文本输出保持用户可读兼容。
- `overview-readonly-smoke.ps1` 增加 `REKIT_GO_EXE` fake backend sentinel，证明 `overview -Format json` 在满足条件时确实经 façade 委托 Go；同时保留 Go CLI JSON、PowerShell fallback 初始化和 no-write snapshot 验证。
- 更新 README、skill、Go runtime migration 与 CHANGELOG，说明 overview JSON 委托只覆盖已初始化 case 的只读机器输出。

边界：本批只调整显式 `REKIT_GO_ENABLE=1` 下的 `overview -Format json` 只读委托；不默认启用 Go、不改变缺 board 初始化语义、不委托 overview 文本输出、不写 facts/lanes/handoff/authority/confirmed、不启动 subagent、不执行 heavy-tool、不改变 sync/promote review-first 语义。

验证：

```powershell
go test ./internal/rekit/cli ./internal/rekit/overview
go test ./...
.\rekit\tests\overview-readonly-smoke.ps1
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`go test ./internal/rekit/cli ./internal/rekit/overview`、`go test ./...`、`overview-readonly-smoke.ps1`、`/rekit doctor` 与 `git diff --check` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 74：note list JSON façade 委托

状态：已完成。

目标：继续收敛只读机器可读路径到 Go backend：在保持 `/rekit note` append 写入仍由 PowerShell fallback 控制的前提下，让已 attach case 的 `note -List -Format json` 经显式 Go façade 委托，减少 PowerShell ledger JSON 查询复制面。

实施范围：

- `rekit/rekit.ps1` 将 `note` 纳入显式 Go 安全集合，但只允许 `-List -Format json`、未带 `-WhatIf`/`-Apply`/`-CreateCandidates` 且目标看起来是 attached case 时委托。
- 新增 façade 层 `RemainingArgs` 轻量解析 helper，用于识别 `note` 的 `-List`、`-Kind`、`-Lane` 和 `-Format`；保留顶层命名参数与旧 RemainingArgs 兼容。
- Go argument builder 对 `note` 转发 resolved target、`-Format json`、`-List`、`-Kind` 和 `-Lane`。
- `agent-team-review-loop-smoke.ps1` 增加 `REKIT_GO_EXE` fake backend sentinel，证明 `note -List -Format json` 满足条件时确实经 façade 委托 Go；同时断言文本 `note -List` 即使显式 Go enable 仍 fallback PowerShell。
- 更新 README、skill、Go runtime migration 与 CHANGELOG，说明 note list JSON 委托只覆盖只读查询，不覆盖 append 写入。

边界：本批只调整显式 `REKIT_GO_ENABLE=1` 下的 `note -List -Format json` 只读委托；不默认启用 Go、不委托 note append、不委托 `note -WhatIf`、不改变 ledger schema、不写 board/facts/lanes/handoff/authority/confirmed、不启动 subagent、不执行 heavy-tool、不改变 sync/promote review-first 语义。

验证：

```powershell
go test ./internal/rekit/cli ./internal/rekit/note
go test ./...
.\rekit\tests\agent-team-review-loop-smoke.ps1
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`go test ./internal/rekit/cli ./internal/rekit/note`、`go test ./...`、`agent-team-review-loop-smoke.ps1`、`/rekit doctor` 与 `git diff --check` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 75：attach/repair JSON preview façade 委托

状态：已完成。

目标：继续把低风险 metadata 预览收敛到 Go backend：在不扩大 attach/repair 写入委托面的前提下，让维护自动化可通过公共 PowerShell façade 消费 `attach -WhatIf -Format json` 与 `repair -Format json` 的非写入 JSON plan。

实施范围：

- `rekit/rekit.ps1` 将 `attach` 与 `repair` 纳入显式 Go 安全集合，但仅允许 JSON 非写入预览委托：`attach` 必须显式 `-Target`、`-WhatIf`、`-Format json` 且不带 `-Apply`/`-CreateCandidates`/review artifact flags；`repair` 必须显式 `-Target`、`-Format json`、目标看起来是 attached case 且不带 `-Apply`/`-CreateCandidates`/review artifact flags。
- Go argument builder 对 `attach`/`repair` 转发 resolved target、`-Format`、`-ProjectName` 和 mode flags；写入路径仍由安全集合拒绝委托。
- `init-bootstrap-smoke.ps1` 增加 `REKIT_GO_EXE` fake backend sentinel，证明 `attach -WhatIf -Format json` 与 `repair -Format json` 满足条件时确实经 façade 委托 Go；同时断言 attach/repair 文本预览即使显式 Go enable 仍 fallback PowerShell。
- 更新 README、skill、Go runtime migration 与 CHANGELOG，说明 attach/repair façade 委托只覆盖 JSON 非写入 metadata 预览，不覆盖文本预览或 `-Apply` 写入。

边界：本批只调整显式 `REKIT_GO_ENABLE=1` 下的 attach/repair JSON 预览委托；不默认启用 Go、不委托 attach/repair `-Apply`、不委托 init/bootstrap、sync apply、promote apply/candidates、工作线 apply、note append、authority/confirmed 更新，不写 board/facts/lanes/handoff/managed docs，不执行 heavy-tool，不改变 sync/promote review-first 语义。

验证：

```powershell
go test ./internal/rekit/cli ./internal/rekit/attach ./internal/rekit/repair
.\rekit\tests\init-bootstrap-smoke.ps1
go test ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`go test ./internal/rekit/cli ./internal/rekit/attach ./internal/rekit/repair`、`init-bootstrap-smoke.ps1`、`go test ./...`、`/rekit doctor` 与 `git diff --check` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 76：init/bootstrap JSON preview façade 委托

状态：已完成。

目标：继续把 case 初始化的非写入机器可读预览收敛到 Go backend：在不扩大 init/bootstrap 写入委托面的前提下，让维护自动化可通过公共 PowerShell façade 消费 `init/bootstrap -WhatIf -Format json` 的初始化 plan。

实施范围：

- `rekit/rekit.ps1` 将 `init` 与 `bootstrap` 纳入显式 Go 安全集合，但仅允许显式 `-Target`、`-WhatIf`、`-Format json` 且不带 `-Apply`/`-CreateCandidates`/review artifact flags 时委托。
- Go argument builder 对 `init`/`bootstrap` 转发 resolved target、`-Format`、`-ProjectName`、`-Force` 和 mode flags；写入路径仍由安全集合拒绝委托。
- `init-bootstrap-smoke.ps1` 增加 `REKIT_GO_EXE` fake backend sentinel，证明 `init -WhatIf -Format json` 与 `bootstrap -WhatIf -Format json` 满足条件时确实经 façade 委托 Go；同时保留 init 文本 preview fallback 断言，确保文本预览不被 fake backend 截获。
- 更新 README、skill、Go runtime migration 与 CHANGELOG，说明 init/bootstrap façade 委托只覆盖 JSON 非写入初始化预览，不覆盖文本预览或 `-Apply` 写入。

边界：本批只调整显式 `REKIT_GO_ENABLE=1` 下的 init/bootstrap JSON 预览委托；不默认启用 Go、不委托 init/bootstrap `-Apply`、不委托 sync apply、promote apply/candidates、工作线 apply、note append、authority/confirmed 更新，不写 case metadata/shim/managed docs/board/facts/lanes/handoff，不执行 heavy-tool，不改变 sync/promote review-first 语义。

验证：

```powershell
go test ./internal/rekit/cli ./internal/rekit/sync
.\rekit\tests\init-bootstrap-smoke.ps1
go test ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`go test ./internal/rekit/cli ./internal/rekit/sync`、`init-bootstrap-smoke.ps1`、`go test ./...`、`/rekit doctor` 与 `git diff --check` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 77：promote candidate JSON preview façade 委托

状态：已完成。

目标：继续把非写入机器可读预览收敛到 Go backend：在不扩大 pack candidate 实际写入委托面的前提下，让维护自动化可通过公共 PowerShell façade 消费 `promote -CreateCandidates -WhatIf -Format json` 的候选生成 plan。

实施范围：

- `rekit/rekit.ps1` 放宽 `promote` 显式 Go 安全集合：保留裸 promote review-only 委托；新增仅当 attached case、`-CreateCandidates -WhatIf -Format json`、无 `-Apply`/`-Review`/review artifact flags 时委托 Go。
- Go argument builder 对 `promote` 转发 `-Format`，让 fake backend sentinel 和真实 Go CLI 都能消费 JSON preview 参数。
- `promote-candidates-preflight-smoke.ps1` 增加 `REKIT_GO_EXE` fake backend sentinel，证明 `promote -CreateCandidates -WhatIf -Format json` 满足条件时确实经 façade 委托 Go；同时保留文本 `-CreateCandidates -WhatIf` fallback 断言，确保非 JSON 预览仍走 PowerShell。
- 更新 README、skill、Go runtime migration、promote candidates migration 与 CHANGELOG，说明 promote façade 委托只覆盖 JSON 非写入 candidate preview，不覆盖实际 candidate 写入或 apply 写入。

边界：本批只调整显式 `REKIT_GO_ENABLE=1` 下的 promote create-candidates JSON what-if 委托；不默认启用 Go、不委托 `promote -CreateCandidates` 实际写入、不委托 `promote -Apply`、不写 pack source、不写 `promote-candidates/**` 或 `tooling/candidates/**`、不写 authority/confirmed、不执行 heavy-tool、不改变 sync/promote review-first 语义。

验证：

```powershell
go test ./internal/rekit/cli ./internal/rekit/promote
.\rekit\tests\promote-candidates-preflight-smoke.ps1
go test ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`go test ./internal/rekit/cli ./internal/rekit/promote`、`promote-candidates-preflight-smoke.ps1`、`go test ./...`、`/rekit doctor` 与 `git diff --check` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 78：promote apply JSON preview façade 委托

状态：已完成。

目标：继续把非写入机器可读预览收敛到 Go backend：在不扩大 pack source 实际写入委托面的前提下，让维护自动化可通过公共 PowerShell façade 消费 `promote -Apply -WhatIf -Format json` 的 pack apply plan。

实施范围：

- `rekit/rekit.ps1` 放宽 `promote` 显式 Go 安全集合：保留裸 promote review-only 与 Batch 77 candidate preview 委托；新增仅当 attached case、`-Apply -WhatIf -Format json`、无 `-CreateCandidates`/`-Review`/review artifact flags 时委托 Go。
- `promote-apply-preflight-smoke.ps1` 增加 `REKIT_GO_EXE` fake backend sentinel，证明 `promote -Apply -WhatIf -Format json` 满足条件时确实经 façade 委托 Go；同时保留文本 `-Apply -WhatIf` fallback 断言，确保非 JSON 预览仍走 PowerShell。
- 更新 README、skill、Go runtime migration、promote apply migration 与 CHANGELOG，说明 promote façade 委托只覆盖 JSON 非写入 apply preview，不覆盖实际 apply 写入。

边界：本批只调整显式 `REKIT_GO_ENABLE=1` 下的 promote apply JSON what-if 委托；不默认启用 Go、不委托 `promote -Apply` 实际写入、不委托 `promote -CreateCandidates` 实际写入、不写 pack source、不写 backup/candidate/tooling candidate、不写 authority/confirmed、不执行 heavy-tool、不改变 sync/promote review-first 语义。

验证：

```powershell
go test ./internal/rekit/cli ./internal/rekit/promote
.\rekit\tests\promote-apply-preflight-smoke.ps1
go test ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`go test ./internal/rekit/cli ./internal/rekit/promote`、`promote-apply-preflight-smoke.ps1`、`go test ./...`、`/rekit doctor` 与 `git diff --check` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 79：sync apply JSON preview façade 委托

状态：已完成。

目标：继续把非写入机器可读预览收敛到 Go backend：在不扩大 case managed docs 实际写入委托面的前提下，让维护自动化可通过公共 PowerShell façade 消费 `sync -Apply -WhatIf -Format json` 的 case apply plan。

实施范围：

- `internal/rekit/sync` 增加 `ApplyPreview`，复用 apply guard、manifest、backup root 与 action 计算，返回 `isMutation=false` / `applied=false` 的 JSON preview；不写 metadata/shim/managed docs/template file/managed block/support file/state，不创建 backup 或 review artifact。
- `internal/rekit/cli` 允许且仅允许 `sync -Apply -WhatIf` 进入非写入 preview；裸 `sync -WhatIf` 继续拒绝，`sync -Apply` 实际写入路径保持不变。
- `rekit/rekit.ps1` 放宽 `sync/update` 显式 Go 安全集合：保留 review-only 委托；新增仅当 attached case、`-Apply -WhatIf -Format json`、无 `-CreateCandidates`/`-Review`/review artifact flags 时委托 Go。
- `sync-apply-smoke.ps1` 增加 Go preview no-write 断言与 `REKIT_GO_EXE` fake backend sentinel，证明 façade JSON preview 委托；同时保留文本 `sync -Apply -WhatIf` fallback 断言。
- `facade-smoke.ps1` 增加真实 Go `sync -Apply -WhatIf -Format json` 委托覆盖，并保留文本 fallback。
- 更新 README、skill、Go runtime migration、sync apply migration 与 CHANGELOG，说明 sync façade 委托只覆盖 JSON 非写入 apply preview，不覆盖实际 apply 写入。

边界：本批只调整显式 `REKIT_GO_ENABLE=1` 下的 sync apply JSON what-if 委托；不默认启用 Go、不委托 `sync -Apply` 实际写入、不写 case metadata/shim/managed docs/template file/managed block/support file/state、不创建 backup/review artifact、不写 pack/promote candidate/tooling candidate、不写 authority/confirmed、不执行 heavy-tool、不改变 sync review-first 语义。

验证：

```powershell
go test ./internal/rekit/cli ./internal/rekit/sync
.\rekit\tests\sync-apply-smoke.ps1
go test ./...
.\rekit\tests\facade-smoke.ps1 -CaseRoot 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun' -Pack vmp-re
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`go test ./internal/rekit/cli ./internal/rekit/sync`、`sync-apply-smoke.ps1`、`go test ./...`、`facade-smoke.ps1`、`/rekit doctor` 与 `git diff --check` 均通过。`facade-smoke.ps1` 初次因保留的 dryrun case shim 落后于 canonical thin shim 被 case doctor 拒绝；执行 `repair -Apply` 刷新该临时 dryrun case metadata/shim 后，`facade-smoke.ps1` 通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 80：Go façade 安全集合 smoke 矩阵补强

状态：已完成。

目标：在 Batch 75-79 连续扩大显式 Go façade JSON preview/read-only 安全集合后，先补强公共入口回归矩阵，避免后续改动误把文本预览或实际写入路径委托给 Go。

实施范围：

- 扩展 `rekit/tests/facade-smoke.ps1`，新增 fake `REKIT_GO_EXE` sentinel helper，黑盒验证应委托组合确实经 Go façade，而不是 PowerShell fallback 生成相似输出。
- 覆盖 attach/repair/init/bootstrap 的 JSON 非写入 preview 委托，以及对应文本 preview fallback。
- 覆盖 `sync -Apply -WhatIf -Format json`、`promote -CreateCandidates -WhatIf -Format json`、`promote -Apply -WhatIf -Format json` 的 JSON 非写入 preview 委托，以及 sync/promote 文本 what-if fallback。
- 覆盖 `overview -Format json`、`note -List -Format json`、`start/handoff/continue -WhatIf -Format json` 的 read-only/preview 委托，以及 note/start 文本 fallback。
- 保留真实 Go status/case doctor/sync review artifact/gate dry-run/sync apply JSON preview 与 `REKIT_GO_DISABLE=1` 优先级覆盖。
- 更新 Go runtime migration 验证矩阵与 CHANGELOG，记录 façade smoke 已作为安全集合矩阵。

边界：本批只增强测试与文档，不修改 runtime 委托逻辑；不新增任何写入委托，不默认启用 Go，不写 case managed docs/pack source/candidate/tooling candidate，不写 authority/confirmed，不执行 heavy-tool，不改变 sync/promote review-first 语义。

验证：

```powershell
.\rekit\tests\facade-smoke.ps1 -CaseRoot 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun' -Pack vmp-re
go test ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`facade-smoke.ps1`、`go test ./...`、`/rekit doctor` 与 `git diff --check` 均通过；`git diff --check` 仅报告既有 `_template` pack 文档 LF/CRLF warning，无 whitespace error。

### Batch 81：façade smoke 自包含化与清理加固

状态：已完成。

目标：承接 Batch 80 的 façade 安全集合矩阵，把它从依赖长期 dryrun case 的回归脚本加固为默认自包含、可重复、少受外部状态影响的 smoke。

实施范围：

- `facade-smoke.ps1` 的默认 `-CaseRoot` 改为空；未显式传入 case 时，脚本在 per-run matrix root 下创建 `_template` 临时 case，运行 `init`、`overview` 和一条 observation note seed，再执行完整 façade 矩阵。
- 保留显式 `-CaseRoot/-Pack` 模式，用于继续验证长期 dryrun case；vmp-re 显式 case 仍使用既有 `feature-handler-0x40a010` gate lane，自包含临时 case 使用 `main` lane。
- 将 sync review artifact、fake `REKIT_GO_EXE` 和临时 case 统一放入 GUID 命名的 matrix root，并在 `finally` 中清理，避免固定 temp 路径与中断残留污染后续运行。
- 保留 Batch 80 的 fake backend sentinel 矩阵与真实 Go status/case doctor/sync review/gate/sync apply preview 覆盖；不改变 runtime 委托逻辑。
- 更新 Go runtime migration 验证矩阵与 CHANGELOG，记录 façade smoke 默认可自包含运行，同时仍支持显式长期 case 验证。

边界：本批只增强测试与文档，不修改 runtime 委托逻辑；不新增任何写入委托，不默认启用 Go。测试写入仅限临时 case fixture，并在 finally 清理；不写 pack source/promote candidates/tooling candidates/authority/confirmed，不执行 heavy-tool，不改变 sync/promote review-first 语义。

验证：

```powershell
.\rekit\tests\facade-smoke.ps1
.\rekit\tests\facade-smoke.ps1 -CaseRoot 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun' -Pack vmp-re
go test ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。默认自包含 `facade-smoke.ps1`、显式 dryrun case `facade-smoke.ps1`、`go test ./...`、`/rekit doctor` 与 `git diff --check` 均通过；`git diff --check` 仅报告既有 `_template` pack 文档及 `facade-smoke.ps1` 的 LF/CRLF warning，无 whitespace error。

### Batch 82：sync apply PowerShell/Go parity smoke

状态：已完成。

目标：补齐 `docs/sync-apply-migration.md` 中 S18 apply parity 验证缺口，用双临时 case 证明 PowerShell 与 Go `sync -Apply` 在 managed docs、template、managed block、metadata/shim 和 sync state 上保持对齐；同时不扩大 façade 写入委托面。

实施范围：

- 新增 `rekit/tests/sync-apply-parity-smoke.ps1`：创建两份 `_template` 临时 case，构造相同 README drift、task-handoff local template、managed block drift；一份走 PowerShell `/rekit sync -Apply`，一份走 Go CLI `sync -Apply`。
- parity smoke 归一化比较 managed docs、template target、managed block host、case-local thin shim、`.rekit/instance.yml`、legacy `.re-template.yml` 与 `.rekit/state.json` managed hashes；case root 与行尾差异做归一化，backup timestamp/path 不做 byte-level 相等。
- parity smoke 覆盖 `sync -Apply -Force`：验证 PowerShell 与 Go 均 overwrite local template、替换 project placeholder，并在两侧创建 backup。
- 发现并修复 PowerShell façade `sync/update -Force` 未传给 `Sync-RekitPack -ForceLocalTemplates` 的问题；该修复只让既有 `-Force` 参数生效，不新增命令或委托面。
- 更新 sync apply migration、Go runtime migration 与 CHANGELOG，记录 S18 已由 parity smoke 覆盖。

边界：本批只补验证与一个参数透传修复；不委托 `sync -Apply` 实际写入到 Go façade，不写 pack source/promote candidates/tooling candidates/authority/confirmed，不执行 heavy-tool。测试写入仅限临时 case，并在 finally 清理。

验证：

```powershell
.\rekit\tests\sync-apply-parity-smoke.ps1
.\rekit\tests\sync-apply-smoke.ps1
.\rekit\tests\facade-smoke.ps1
go test ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`sync-apply-parity-smoke.ps1`、`sync-apply-smoke.ps1`、默认自包含 `facade-smoke.ps1`、`go test ./...`、`/rekit doctor` 与 `git diff --check` 均通过；`git diff --check` 仅报告既有 `_template` pack 文档 LF/CRLF warning，无 whitespace error。

收尾：Stop hook 后补查 README、仓库 CLAUDE.md、canonical `/rekit` skill、Go runtime migration 与 sync apply migration。Go runtime migration 和 sync apply migration 已覆盖 S18 parity、实际 `sync -Apply` 不经 façade 委托和默认自包含 façade smoke；额外补充 README 的 `sync -Force` 本地模板覆盖语义、仓库 CLAUDE.md 的默认自包含 `facade-smoke.ps1` 验证入口，以及 `/rekit` skill 中 template files 默认 create-if-missing / `-Force` overwrite-with-backup 规则。补充后再次验证 `sync-apply-parity-smoke.ps1`、默认自包含 `facade-smoke.ps1`、`go test ./...`、`/rekit doctor` 与 `git diff --check` 全部通过；`git diff --check` 仅报告 LF/CRLF warning，无 whitespace error。

### Batch 83：Agent Team D5 mock/non-sensitive dry-run smoke

状态：已完成。

目标：承接 `docs/agent-team-rollout-plan.md` D5，用 mock/非敏感自包含临时 case 跑通 candidate → verification → decision → batch → intervention/rollback → handoff，验证 ledger/read-layer 闭环真实可用，同时不触发 heavy-tool、authority/confirmed 或外部副作用。

实施范围：

- 新增 `rekit/tests/agent-team-d5-dryrun-smoke.ps1`，默认使用 `_template` pack 在 `C:\AI\m_projects\RE\_dryrun_cases` 下创建 GUID 后缀临时 case。
- smoke 执行 `init -> overview -> start` 后，写入同一 `batchId` 下的 candidate、verification、decision、intervention 与 rollback 事件。
- 覆盖 `note -List` 文本输出：candidate confidence/status/risk/batch，verification verifier/verdict/target/batch，decision decision/by/batch，intervention action/target/approvedBy/scope/status/reason/batch，rollback target/status/reason/batch。
- 覆盖 `note -List -Format json`，确认 5 类事件分组和 `eventCount=5`。
- 覆盖 overview 展示：未决 candidate、最近 verification、最近 decision、最近 batch、未解决 intervention、最近 intervention、最近 rollback。
- 覆盖 lane handoff 展示：`## verification`、`## decision`、`## intervention`、`## rollback` 区段。
- 更新 `docs/agent-team-rollout-plan.md` 将 D5 标为完成；更新 `docs/go-runtime-migration.md` 验证矩阵和 `CHANGELOG.md`。

边界：本批只新增 smoke 与文档；测试写入仅限临时 case `.rekit/facts/**`、`.rekit/board.json`、lane/handoff 等 case-local 状态，并在 `finally` 清理。不写 kit 模板、pack source、promote candidates、tooling candidates、authority/confirmed，不执行 full-trace/debug/inject/patch/dump/network 或外部动作。

验证：

```powershell
.\rekit\tests\agent-team-d5-dryrun-smoke.ps1
.\rekit\tests\agent-team-review-loop-smoke.ps1
.\rekit\tests\gate-parity-smoke.ps1
go test ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`agent-team-d5-dryrun-smoke.ps1`、`agent-team-review-loop-smoke.ps1`、`gate-parity-smoke.ps1`、`go test ./...`、`/rekit doctor` 与 `git diff --check` 均通过；`git diff --check` 仅报告 LF/CRLF warning，无 whitespace error。

### Batch 84：IDA bridge read-only adapter contract

状态：已完成。

目标：承接 `docs/agent-team-rollout-plan.md` D6，把 `ida-agent-bridge` 从 candidate tooling 推进到只读 index packet contract / capability card；明确 Agent 如何消费已有 IDA sidecar/export 的 function/string/import/xref 小索引，同时不安装、不连接、不驱动 IDA，也不接 runtime 强依赖。

实施范围：

- 新增 `packs/vmp-re/tooling/recipes/ida-agent-bridge-readonly.md`，包含读取指南、实施摘要、执行清单、验证标准、风险、capability card、packet schema、使用流程和明确禁止项。
- packet schema 覆盖 `mode: read-only-index`、`sideEffects: ["filesystem-read"]`、source/limits/functions/strings/imports/xrefs/snippets/evidenceRefs/warnings/errors/nextActions。
- `packs/vmp-re/manifest.yml` 将该 recipe 纳入 `toolingFiles`。
- `tooling/catalog.yml`、`tooling/README.md`、`ida-x64dbg-mcp.md` 链接只读 contract。
- `docs/agent-team-rollout-plan.md` 将 D6 标为完成，`docs/reference-absorption.md` 更新 `ida-agent-bridge` 吸收状态，`CHANGELOG.md` 记录 Batch 84。

边界：本批只写文档和 manifest tooling file 列表；不安装、下载、打开或连接 IDA / bridge，不生成全量导出，不实现 runtime-level adapter，不执行 rename/comment/patch/debug/dump/network，不写真实样本路径、绝对路径、完整 decompile/disasm/hexdump/trace 到模板。

验证：

```powershell
.\packs\vmp-re\scripts\validate.ps1
go test ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。首次 `go test ./...` 暴露 pack inventory fixture 仍预期 vmp-re `toolingFiles=11`；新增 recipe 后已同步更新 `internal/rekit/cli/cli_test.go` 为 `toolingFiles=12`，随后 `validate.ps1`、`go test ./...`、`/rekit doctor` 与 `git diff --check` 均通过；`git diff --check` 仅报告 LF/CRLF warning，无 whitespace error。

### Batch 85：malware-analysis pack skeleton

状态：已完成。

目标：承接 Phase 3 多安全领域 pack 扩展，在 `web-security` 之后新增防御性恶意样本分析 pack skeleton，用最小可验证骨架覆盖授权样本分析、事件响应辅助、检测工程与防御性安全研究场景，同时不把项目误导成自动恶意软件执行平台或样本库。

实施范围：

- 新增 `packs/malware-analysis/manifest.yml`、`CLAUDE.local.snippet.md`、policy overlay 空 registry、managed reference docs、task handoff template、tooling catalog 与两条 recipes。
- reference docs 覆盖 scope baseline、静态优先轻到重路线、sample/behavior/IOC bounded review、dynamic/sandbox/network/debug/dump gate、sidecar 与敏感信息留在 case-local 的规则。
- manifest 声明 `malware-analysis:bounded-review` 与 `malware-analysis:sample-analysis` 两条 route，默认 start lane type 为 `sample-analysis`，tooling files 为 `static-triage.md` 与 `sandbox-sidecar-review.md`。
- 新增 `rekit/tests/malware-analysis-pack-smoke.ps1`，覆盖 Go/PowerShell doctor、Go init、case doctor、Go/PowerShell `plan-subagents`、promote review 不被 deny pattern 误阻断和 no-write 边界。
- 更新 pack inventory fixtures，将 `malware-analysis` 纳入 Go CLI 与 PowerShell smoke，同时修正 vmp-re tooling count 为 12。
- 更新 README、CLAUDE.md、vision、reference absorption、pack authoring、agent-team usage、Go migration 与 CHANGELOG，记录 `malware-analysis` 是 skeleton，不是自动恶意样本分析平台。

边界：本批只新增最小 pack 骨架和验证；不执行样本、不上传 sandbox、不联网、不 debug/inject/patch/dump、不接入外部情报服务、不写真实 hash/IOC/样本路径/customer artifact；不写真实 case confirmed/authority；不改变 PowerShell façade 委托集合。

停止条件：若后续要把 `malware-analysis` 扩展成真实 sandbox adapter、动态执行 runner、外部情报查询、检测规则 authority schema 或自动 IOC 发布流程，应作为独立批次评估 gate、隔离和授权边界。

验证：

```powershell
.\rekit\tests\malware-analysis-pack-smoke.ps1
.\rekit\tests\web-security-pack-smoke.ps1
.\rekit\tests\pack-inventory-smoke.ps1
go test ./...
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Pack malware-analysis
git diff --check
```

验证结果：全部通过。`malware-analysis-pack-smoke.ps1`、`web-security-pack-smoke.ps1`、`pack-inventory-smoke.ps1`、`go test ./...`、`/rekit doctor`、`/rekit doctor -Pack malware-analysis` 与 `git diff --check` 均通过；`git diff --check` 仅报告 LF/CRLF warning，无 whitespace error。

### Batch 86：vuln-research pack skeleton

状态：已完成。

目标：承接 Phase 3 多安全领域 pack 扩展，在 `web-security` 与 `malware-analysis` 之后新增授权漏洞研究 pack skeleton，用最小可验证骨架覆盖防御性复现、补丁/崩溃分析、root-cause review 与安全工程验证场景，同时不把项目误导成自动漏洞挖掘器、利用链生成器或攻击执行平台。

实施范围：

- 新增 `packs/vuln-research/manifest.yml`、`CLAUDE.local.snippet.md`、policy overlay 空 registry、managed reference docs、task handoff template、tooling catalog 与两条 recipes。
- reference docs 覆盖 scope baseline、crash/patch/repro 轻到重路线、finding/repro/patch bounded review、active scan/fuzz/exploit replay/live target gate、sidecar 与敏感信息留在 case-local 的规则。
- manifest 声明 `vuln-research:bounded-review` 与 `vuln-research:vuln-analysis` 两条 route，默认 start lane type 为 `vuln-analysis`，tooling files 为 `crash-triage.md` 与 `repro-sidecar-review.md`。
- 新增 `rekit/tests/vuln-research-pack-smoke.ps1`，覆盖 Go/PowerShell doctor、Go init、case doctor、Go/PowerShell `plan-subagents`、promote review 不被 deny pattern 误阻断和 no-write 边界。
- 更新 pack inventory fixtures，将 `vuln-research` 纳入 Go CLI 与 PowerShell smoke。
- 更新 README、CLAUDE.md、vision、reference absorption、pack authoring、agent-team usage、Go migration 与 CHANGELOG，记录 `vuln-research` 是 skeleton，不是自动漏洞挖掘或 exploit replay 平台。

边界：本批只新增最小 pack 骨架和验证；不主动扫描、不 fuzz、不 replay exploit、不访问真实目标、不 debug/dump/patch、不导出数据、不写真实 target/request/response/payload/crash/core/minidump/customer artifact；不写真实 case confirmed/authority；不改变 PowerShell façade 委托集合。

停止条件：若后续要把 `vuln-research` 扩展成真实 fuzz/replay adapter、漏洞报告 authority schema、补丁 diff 自动结论、真实目标验证或自动 disclosure/report 发布流程，应作为独立批次评估 gate、隔离和授权边界。

验证：

```powershell
.\rekit\tests\vuln-research-pack-smoke.ps1
.\rekit\tests\malware-analysis-pack-smoke.ps1
.\rekit\tests\pack-inventory-smoke.ps1
go test ./...
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Pack vuln-research
git diff --check
```

验证结果：全部通过。`vuln-research-pack-smoke.ps1`、`malware-analysis-pack-smoke.ps1`、`pack-inventory-smoke.ps1`、`go test ./...`、`/rekit doctor`、`/rekit doctor -Pack vuln-research` 与 `git diff --check` 均通过；`git diff --check` 仅报告 LF/CRLF warning，无 whitespace error。

### Batch 87：ctf pack skeleton

状态：已完成。

目标：承接 Phase 3 多安全领域 pack 扩展，在 `web-security`、`malware-analysis` 与 `vuln-research` 之后新增授权 CTF/靶场 challenge pack skeleton，用最小可验证骨架覆盖 pwn/web/crypto/rev/forensics/misc challenge 分析、solver/writeup review 与本地复现验证场景，同时不把项目误导成面向真实目标的攻击执行平台、自动利用链平台或 flag 泄漏库。

实施范围：

- 新增 `packs/ctf/manifest.yml`、`CLAUDE.local.snippet.md`、policy overlay 空 registry、managed reference docs、task handoff template、tooling catalog 与两条 recipes。
- reference docs 覆盖 scope baseline、challenge/artifact/solver 轻到重路线、solution/writeup/exploit bounded review、remote/bruteforce/fuzz/exploit replay gate、sidecar 与 flag/payload 等敏感信息留在 case-local 的规则。
- manifest 声明 `ctf:bounded-review` 与 `ctf:challenge-analysis` 两条 route，默认 start lane type 为 `challenge-analysis`，tooling files 为 `challenge-triage.md` 与 `local-repro-review.md`。
- 新增 `rekit/tests/ctf-pack-smoke.ps1`，覆盖 Go/PowerShell doctor、Go init、case doctor、Go/PowerShell `plan-subagents`、promote review 不被 deny pattern 误阻断和 no-write 边界。
- 更新 pack inventory fixtures，将 `ctf` 纳入 Go CLI 与 PowerShell smoke。
- 更新 README、CLAUDE.md、vision、reference absorption、pack authoring、agent-team usage、Go migration 与 CHANGELOG，记录 `ctf` 是 skeleton，不是远程攻击、自动利用或 flag 泄漏平台。

边界：本批只新增最小 pack 骨架和验证；不远程连接、不 bruteforce、不 fuzz、不 replay exploit、不访问真实目标、不 debug/dump/patch、不写 flag/payload/solver/challenge raw/customer artifact；不写真实 case confirmed/authority；不改变 PowerShell façade 委托集合。

停止条件：若后续要把 `ctf` 扩展成真实 remote adapter、bruteforce/fuzz harness、solver authority schema、writeup 发布流程或自动 flag 提交流程，应作为独立批次评估 gate、隔离和授权边界。

验证：

```powershell
.\rekit\tests\ctf-pack-smoke.ps1
.\rekit\tests\vuln-research-pack-smoke.ps1
.\rekit\tests\pack-inventory-smoke.ps1
go test ./...
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Pack ctf
git diff --check
```

验证结果：全部通过。`ctf-pack-smoke.ps1`、`vuln-research-pack-smoke.ps1`、`pack-inventory-smoke.ps1`、`go test ./...`、`/rekit doctor`、`/rekit doctor -Pack ctf` 与 `git diff --check` 均通过；`git diff --check` 仅报告 LF/CRLF warning，无 whitespace error。

### Batch 88：unpack-pe pack skeleton

状态：已完成。

目标：承接 Phase 3 多安全领域 pack 扩展，在 `web-security`、`malware-analysis`、`vuln-research` 与 `ctf` 之后新增授权 PE unpacking / loader triage pack skeleton，用最小可验证骨架覆盖 PE static triage、loader stage 分析、import recovery 摘要、unpack candidate review 与动态动作 gate，同时不把项目误导成自动脱壳器、样本执行器、动态调试平台或 patch/dump 自动化引擎。

实施范围：

- 新增 `packs/unpack-pe/manifest.yml`、`CLAUDE.local.snippet.md`、policy overlay 空 registry、managed reference docs、task handoff template、tooling catalog 与两条 recipes。
- reference docs 覆盖 scope baseline、PE static triage 到 loader/unpack review 的轻到重路线、bounded review、dynamic/debug/dump/patch/import-rebuild gate、sidecar 与样本/hash/dump/trace/patch/unpacked artifact 留在 case-local 的规则。
- manifest 声明 `unpack-pe:bounded-review` 与 `unpack-pe:unpack-analysis` 两条 route，默认 start lane type 为 `unpack-analysis`，tooling files 为 `pe-static-triage.md` 与 `loader-unpack-review.md`。
- 新增 `rekit/tests/unpack-pe-pack-smoke.ps1`，覆盖 Go/PowerShell doctor、Go init、case doctor、Go/PowerShell `plan-subagents`、promote review 不被 deny pattern 误阻断和 no-write 边界。
- 更新 pack inventory fixtures，将 `unpack-pe` 纳入 Go CLI 与 PowerShell smoke。
- 更新 README、CLAUDE.md、vision、reference absorption、pack authoring、agent-team usage、Go migration 与 CHANGELOG，记录 `unpack-pe` 是 skeleton，不是自动脱壳器、样本执行器、动态调试平台或 patch/dump 自动化引擎。

边界：本批只新增最小 pack 骨架和验证；不执行样本、不 debug、不 dump、不 patch、不联网、不写 unpacked binary、不写完整 import table/section bytes/hash/IOC/customer artifact；不写真实 case confirmed/authority；不改变 PowerShell façade 委托集合。

停止条件：若后续要把 `unpack-pe` 扩展成真实 debugger adapter、sandbox adapter、dump/patch/import rebuild 执行器、unpacked artifact authority schema 或自动脱壳流程，应作为独立批次评估 gate、隔离、授权和回滚边界。

验证：

```powershell
.\rekit\tests\unpack-pe-pack-smoke.ps1
.\rekit\tests\ctf-pack-smoke.ps1
.\rekit\tests\pack-inventory-smoke.ps1
go test ./...
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Pack unpack-pe
git diff --check
```

验证结果：全部通过。`unpack-pe-pack-smoke.ps1`、`ctf-pack-smoke.ps1`、`pack-inventory-smoke.ps1`、`go test ./...`、`/rekit doctor`、`/rekit doctor -Pack unpack-pe` 与 `git diff --check` 均通过；`git diff --check` 仅报告 LF/CRLF warning，无 whitespace error。

### Batch 89：ollvm pack skeleton

状态：已完成。

目标：承接 Phase 3 多安全领域 pack 扩展，在 `unpack-pe` 之后新增授权 OLLVM / native obfuscation analysis pack skeleton，用最小可验证骨架覆盖 control-flow flattening triage、opaque predicate review、MBA simplification、string decode review、deobfuscation candidate review 与动态/写入动作 gate，同时不把项目误导成自动反混淆引擎、patch 平台、符号恢复器或样本执行器。

实施范围：

- 新增 `packs/ollvm/manifest.yml`、`CLAUDE.local.snippet.md`、policy overlay 空 registry、managed reference docs、task handoff template、tooling catalog 与两条 recipes。
- reference docs 覆盖 scope baseline、CFG/function triage 到 MBA simplification review 的轻到重路线、bounded review、debug/trace/dump/patch/rename-comment/export gate、sidecar 与样本/hash/full CFG/function body/patch/deobfuscated artifact 留在 case-local 的规则。
- manifest 声明 `ollvm:bounded-review` 与 `ollvm:obfuscation-analysis` 两条 route，默认 start lane type 为 `obfuscation-analysis`，tooling files 为 `control-flow-triage.md` 与 `mba-simplification-review.md`。
- 新增 `rekit/tests/ollvm-pack-smoke.ps1`，覆盖 Go/PowerShell doctor、Go init、case doctor、Go/PowerShell `plan-subagents`、promote review 不被 deny pattern 误阻断和 no-write 边界。
- 更新 pack inventory fixtures，将 `ollvm` 纳入 Go CLI 与 PowerShell smoke。
- 更新 README、CLAUDE.md、vision、reference absorption、pack authoring、agent-team usage、Go migration 与 CHANGELOG，记录 `ollvm` 是 skeleton，不是自动反混淆引擎、patch 平台、符号恢复器或样本执行器。

边界：本批只新增最小 pack 骨架和验证；不执行样本、不 debug、不 trace、不 dump、不 patch、不批量反编译、不自动 rename/comment、不导出 deobfuscated binary、不联网、不写完整 CFG/function body/hash/IOC/customer artifact；不写真实 case confirmed/authority；不改变 PowerShell façade 委托集合。

停止条件：若后续要把 `ollvm` 扩展成真实 debugger/trace adapter、IDB write adapter、patch/export 执行器、symbol/rename authority schema 或自动反混淆流程，应作为独立批次评估 gate、隔离、授权和回滚边界。

验证：

```powershell
.\rekit\tests\ollvm-pack-smoke.ps1
.\rekit\tests\unpack-pe-pack-smoke.ps1
.\rekit\tests\pack-inventory-smoke.ps1
go test ./...
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Pack ollvm
git diff --check
```

验证结果：全部通过。`ollvm-pack-smoke.ps1`、`unpack-pe-pack-smoke.ps1`、`pack-inventory-smoke.ps1`、`go test ./...`、`/rekit doctor`、`/rekit doctor -Pack ollvm` 与 `git diff --check` 均通过；`git diff --check` 仅报告 LF/CRLF warning，无 whitespace error。

### Batch 90：android-native pack skeleton

状态：已完成。

目标：承接 Phase 3 多安全领域 pack 扩展，在 `ollvm` 之后新增授权 Android native / JNI / NDK 分析 pack skeleton，用最小可验证骨架覆盖 APK native triage、JNI bridge 分析、native library/SO review、Frida hook candidate review、emulator sidecar review 与设备/动态动作 gate，同时不把项目误导成 APK 自动逆向平台、设备操控器、hook 执行器或移动端动态测试平台。

实施范围：

- 新增 `packs/android-native/manifest.yml`、`CLAUDE.local.snippet.md`、policy overlay 空 registry、managed reference docs、task handoff template、tooling catalog 与两条 recipes。
- reference docs 覆盖 scope baseline、APK/native/JNI static triage 到 emulator/hook review 的轻到重路线、bounded review、device/emulator/Frida/network/dump/patch/install gate、sidecar 与 APK/package/hash/device/hook/traffic 留在 case-local 的规则。
- manifest 声明 `android-native:bounded-review` 与 `android-native:native-analysis` 两条 route，默认 start lane type 为 `native-analysis`，tooling files 为 `apk-native-triage.md` 与 `emulator-hook-review.md`。
- 新增 `rekit/tests/android-native-pack-smoke.ps1`，覆盖 Go/PowerShell doctor、Go init、case doctor、Go/PowerShell `plan-subagents`、promote review 不被 deny pattern 误阻断和 no-write 边界。
- 更新 pack inventory fixtures，将 `android-native` 纳入 Go CLI 与 PowerShell smoke。
- 更新 README、CLAUDE.md、vision、reference absorption、pack authoring、agent-team usage、Go migration 与 CHANGELOG，记录 `android-native` 是 skeleton，不是 APK 自动逆向平台、设备操控器、hook 执行器或移动端动态测试平台。

边界：本批只新增最小 pack 骨架和验证；不连接设备、不运行 emulator、不 attach Frida、不执行 hook、不抓包、不 dump、不 patch、不重签名、不安装/卸载应用、不联网、不写 APK/package/hash/device id/token/traffic/customer artifact；不写真实 case confirmed/authority；不改变 PowerShell façade 委托集合。

停止条件：若后续要把 `android-native` 扩展成真实 emulator/device adapter、Frida/hook 执行器、traffic capture adapter、APK patch/resign 执行器、mobile authority schema 或动态测试流程，应作为独立批次评估 gate、隔离、授权和回滚边界。

验证：

```powershell
.\rekit\tests\android-native-pack-smoke.ps1
.\rekit\tests\ollvm-pack-smoke.ps1
.\rekit\tests\pack-inventory-smoke.ps1
go test ./...
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Pack android-native
git diff --check
```

验证结果：全部通过。`android-native-pack-smoke.ps1`、`ollvm-pack-smoke.ps1`、`pack-inventory-smoke.ps1`、`go test ./...`、`/rekit doctor`、`/rekit doctor -Pack android-native` 与 `git diff --check` 均通过；`git diff --check` 仅报告 LF/CRLF warning，无 whitespace error。

### Batch 91：generic-binary-re pack skeleton

状态：已完成。

目标：承接 Phase 3 多安全领域 pack 扩展，在 `android-native` 之后新增授权通用二进制逆向 pack skeleton，用最小可验证骨架覆盖 static binary triage、function behavior analysis、API behavior review、format/string analysis、tooling sidecar review 与动态/写入动作 gate，同时不把项目误导成自动逆向引擎、样本执行器、patch 平台或漏洞/恶意样本/VMP/OLLVM 专项 pack 的替代品。

实施范围：

- 新增 `packs/generic-binary-re/manifest.yml`、`CLAUDE.local.snippet.md`、policy overlay 空 registry、managed reference docs、task handoff template、tooling catalog 与两条 recipes。
- reference docs 覆盖 scope baseline、binary/function static triage 到 function behavior review 的轻到重路线、bounded review、debug/trace/dump/patch/rename-comment/database-writeback gate、sidecar 与样本/hash/function body/symbol table/trace/dump/patch 留在 case-local 的规则。
- manifest 声明 `generic-binary-re:bounded-review` 与 `generic-binary-re:binary-analysis` 两条 route，默认 start lane type 为 `binary-analysis`，tooling files 为 `static-binary-triage.md` 与 `function-behavior-review.md`。
- 新增 `rekit/tests/generic-binary-re-pack-smoke.ps1`，覆盖 Go/PowerShell doctor、Go init、case doctor、Go/PowerShell `plan-subagents`、promote review 不被 deny pattern 误阻断和 no-write 边界。
- 更新 pack inventory fixtures，将 `generic-binary-re` 纳入 Go CLI 与 PowerShell smoke。
- 更新 README、CLAUDE.md、vision、reference absorption、pack authoring、agent-team usage、Go migration 与 CHANGELOG，记录 `generic-binary-re` 是 skeleton，不是自动逆向引擎、样本执行器、patch 平台或具体专项 pack 的替代品。

边界：本批只新增最小 pack 骨架和验证；不执行样本、不 debug、不 trace、不 dump、不 patch、不批量反编译、不自动 rename/comment、不写分析数据库、不联网、不写完整二进制/function body/symbol table/hash/IOC/customer artifact；不写真实 case confirmed/authority；不改变 PowerShell façade 委托集合。

停止条件：若后续要把 `generic-binary-re` 扩展成真实 debugger/trace adapter、IDB/database write adapter、patch/export 执行器、symbol/rename authority schema 或自动逆向流程，应作为独立批次评估 gate、隔离、授权和回滚边界。

验证：

```powershell
.\rekit\tests\generic-binary-re-pack-smoke.ps1
.\rekit\tests\android-native-pack-smoke.ps1
.\rekit\tests\pack-inventory-smoke.ps1
go test ./...
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Pack generic-binary-re
git diff --check
```

验证结果：全部通过。`generic-binary-re-pack-smoke.ps1`、`android-native-pack-smoke.ps1`、`pack-inventory-smoke.ps1`、`go test ./...`、`/rekit doctor`、`/rekit doctor -Pack generic-binary-re` 与 `git diff --check` 均通过；`git diff --check` 仅报告 LF/CRLF warning，无 whitespace error。

### Batch 92：pack smoke helper

状态：已完成。

目标：降低多安全领域 skeleton pack smoke 的重复维护成本，把 Batch 85-91 中反复出现的 Go/PowerShell doctor、Go init、case doctor、`plan-subagents` route packet、promote review 与 no-write 边界检查抽成最小 PowerShell helper，同时保持每个 pack 的 task type、route、output contract 和安全临时目录前缀显式配置。

实施范围：

- 新增 `rekit/tests/pack-smoke-lib.ps1`，集中提供 `Invoke-RekitSmoke`、`Invoke-GoRekitSmoke`、文本断言和 `Invoke-RekitPackSmoke`。
- 将 `web-security`、`malware-analysis`、`vuln-research`、`ctf`、`unpack-pe`、`ollvm`、`android-native` 与 `generic-binary-re` 的 pack smoke 迁移为 thin configuration wrapper。
- helper 保留原有验证覆盖：pack doctor、case init、case doctor、managed block、expected case files、Go `plan-subagents` JSON packet、PowerShell façade fallback packet、promote review managed-doc candidate 和 `.rekit/board.json|facts|lanes` no-write checks。
- 每个 wrapper 仍显式声明 safe case prefix，避免把 pack 名中的通用词拆入 case-specific promote deny pattern。
- 更新 pack authoring、Go runtime migration、README、CHANGELOG 与本计划文档，记录 helper 用途和验证入口。

边界：本批只抽取测试辅助逻辑，不改变 runtime、pack manifest、PowerShell façade 委托集合、Go backend 行为、case schema 或 promote/sync 语义；不创建真实 case state；不执行样本、网络、debug、dump、patch、hook、fuzz、exploit replay 或任何外部副作用；不写 authority/confirmed。

停止条件：若后续要把 helper 扩展成自动发现并执行所有 pack smoke、CI matrix、真实 case migration 或 runtime-level pack validation，应作为独立批次评估执行时间、失败定位、临时目录隔离和对外部环境的依赖。

验证：

```powershell
.\rekit\tests\web-security-pack-smoke.ps1
.\rekit\tests\malware-analysis-pack-smoke.ps1
.\rekit\tests\vuln-research-pack-smoke.ps1
.\rekit\tests\ctf-pack-smoke.ps1
.\rekit\tests\unpack-pe-pack-smoke.ps1
.\rekit\tests\ollvm-pack-smoke.ps1
.\rekit\tests\android-native-pack-smoke.ps1
.\rekit\tests\generic-binary-re-pack-smoke.ps1
.\rekit\tests\pack-inventory-smoke.ps1
go test ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。8 个迁移后的 pack smoke、`pack-inventory-smoke.ps1`、`go test ./...`、`/rekit doctor` 与 `git diff --check` 均通过；`git diff --check` 仅报告 LF/CRLF warning，无 whitespace error。

### Batch 93：pack smoke matrix

状态：已完成。

目标：在 Batch 92 的共享 helper 之上新增显式 pack smoke matrix runner，让维护者可一键运行全部安全领域 skeleton pack smoke，也可选择 pack 子集，减少手动串联多个 smoke 命令时的遗漏和失败定位成本。

实施范围：

- 新增 `rekit/tests/pack-smoke-matrix.ps1`，按显式 pack 清单调用现有 `*-pack-smoke.ps1` wrapper。
- 支持默认全量 pack、`-Packs` 子集、逗号分隔选择、`all` 别名、去重、未知 pack guard、`-FailFast` 与共享 `-WorkRoot` 参数。
- matrix 逐个子进程执行 wrapper，输出每个 pack 的 running/passed/failed、elapsedMs 和原始 smoke 输出，失败时汇总 pack 与 exit code。
- 保持各 pack smoke 的验证语义不变：仍由 wrapper + `pack-smoke-lib.ps1` 覆盖 doctor、init、case doctor、`plan-subagents`、promote review 和 no-write 边界。
- 更新 README、Go runtime migration、CHANGELOG 与本计划文档，记录 matrix 用途和维护入口。

边界：本批只新增测试编排脚本，不自动发现 pack、不改变 pack inventory、runtime、manifest schema、Go backend 或 PowerShell façade 委托集合；matrix 只运行已有自包含 smoke，不创建真实 case state；不执行样本、网络、debug、dump、patch、hook、fuzz、exploit replay 或任何外部副作用；不写 authority/confirmed。

停止条件：若后续要把 matrix 接入 CI、按 `/rekit packs` 动态发现 pack、并行化运行、写测试报告 artifact 或跨机器运行，应作为独立批次评估执行时间、日志可读性、临时目录隔离、PowerShell 版本兼容和失败重试策略。

验证：

```powershell
.\rekit\tests\pack-smoke-matrix.ps1 -Packs web-security,generic-binary-re
.\rekit\tests\pack-smoke-matrix.ps1
.\rekit\tests\pack-inventory-smoke.ps1
go test ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。matrix 子集、matrix 全量 8 个 pack、`pack-inventory-smoke.ps1`、`go test ./...`、`/rekit doctor` 与 `git diff --check` 均通过；`git diff --check` 仅报告 LF/CRLF warning，无 whitespace error。

### Batch 94：pack smoke matrix JSON output

状态：已完成。

目标：给 Batch 93 的 pack smoke matrix 增加机器可读输出，让维护者、CI 或后续编排能稳定消费 pack smoke 结果，而不需要解析文本日志；同时保持默认文本输出和失败定位体验不变。

实施范围：

- 扩展 `rekit/tests/pack-smoke-matrix.ps1`，新增 `-Format text|json` 参数，默认保持 `text`。
- JSON envelope 固定输出 `schemaVersion`、`command`、`isMutation`、`workRoot`、`failFast`、`packCount`、`failedCount`、`ok`、`packs[]` 与 `results[]`。
- `results[]` 为每个 pack 提供 `pack`、`script`、`success`、`exitCode`、`elapsedMs` 与原始 `output`，便于失败时直接定位 wrapper 和输出。
- `-Format json` 不输出 running/passed 文本；失败时仍先输出 JSON，再用非零 exit code 标记失败。
- 保持默认文本输出、`-Packs` 子集、`all`、去重、未知 pack guard、`-FailFast` 和共享 `-WorkRoot` 行为不变。
- 更新 README、Go runtime migration、pack authoring、CHANGELOG 与本计划文档，记录 JSON 输出用途和验证入口。

边界：本批只增强测试脚本输出格式，不改变 pack smoke wrapper、shared helper、runtime、pack manifest、Go backend、PowerShell façade 委托集合或验证语义；不创建真实 case state；不执行样本、网络、debug、dump、patch、hook、fuzz、exploit replay 或任何外部副作用；不写 authority/confirmed。

停止条件：若后续要把 JSON 输出接入 CI artifact、引入 JUnit/SARIF、并行化 matrix、自动发现 packs 或长期保存 smoke 结果，应作为独立批次评估报告格式、日志体积、稳定字段、执行时间和失败重试策略。

验证：

```powershell
$json = .\rekit\tests\pack-smoke-matrix.ps1 -Packs web-security,generic-binary-re -Format json | ConvertFrom-Json
.\rekit\tests\pack-smoke-matrix.ps1 -Packs web-security,generic-binary-re
.\rekit\tests\pack-smoke-matrix.ps1 -Format json
.\rekit\tests\pack-inventory-smoke.ps1
go test ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。matrix JSON 子集结构断言、matrix 文本子集、matrix JSON 全量 8 个 pack、`pack-inventory-smoke.ps1`、`go test ./...`、`/rekit doctor` 与 `git diff --check` 均通过；`git diff --check` 仅报告 LF/CRLF warning，无 whitespace error。

### Batch 95：pack smoke discovery guard

状态：已完成。

目标：让 pack smoke matrix 与 `/rekit packs` inventory 保持可检查的一致性，在新增 skeleton pack 后能及时发现缺少 smoke wrapper、matrix 清单遗漏、孤儿 wrapper 或脚本路径缺失，同时不改成自动发现并运行未知脚本。

实施范围：

- 扩展 `rekit/tests/pack-smoke-matrix.ps1`，新增 `-DiscoveryOnly` 模式。
- discovery 通过 PowerShell `/rekit packs -Format json` 读取 inventory，只把 `maturity=skeleton` 且 schema valid 的 pack 视为必须纳入 smoke matrix。
- 输出 `pack-smoke-discovery` envelope，包含 `expectedSkeletonPacks`、`matrixPacks`、`wrapperPacks`、`excludedPacks`、`missingSmokePacks`、`extraMatrixPacks`、`orphanWrapperPacks` 与 `missingScriptPacks`。
- 文本模式显示 ok/failure、缺失/多余/孤儿/脚本缺失项和显式排除项；JSON 模式可供 CI 或后续编排消费。
- 保持默认 matrix 运行、`-Packs`、`-Format json`、`-FailFast` 与各 pack smoke wrapper 行为不变。
- 更新 README、Go runtime migration、pack authoring、CHANGELOG 与本计划文档，记录 discovery guard 的用途和边界。

边界：本批只新增 matrix 清单与 inventory 的只读一致性检查；不自动发现并运行未知 pack smoke、不改变 pack inventory、runtime、manifest schema、Go backend 或 PowerShell façade 委托集合；不创建真实 case state；不执行样本、网络、debug、dump、patch、hook、fuzz、exploit replay 或任何外部副作用；不写 authority/confirmed。

停止条件：若后续要将 discovery guard 接入 CI 必跑、动态生成 matrix、自动创建缺失 wrapper、并行运行所有 pack 或根据 inventory 运行未知脚本，应作为独立批次评估安全边界、失败定位、日志体积和对临时环境的依赖。

验证：

```powershell
.\rekit\tests\pack-smoke-matrix.ps1 -DiscoveryOnly
$json = .\rekit\tests\pack-smoke-matrix.ps1 -DiscoveryOnly -Format json | ConvertFrom-Json
.\rekit\tests\pack-smoke-matrix.ps1 -Packs web-security,generic-binary-re -Format json
.\rekit\tests\pack-smoke-matrix.ps1 -Packs web-security,generic-binary-re
.\rekit\tests\pack-inventory-smoke.ps1
go test ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。discovery 文本与 JSON 断言、matrix JSON 子集、matrix 文本子集、`pack-inventory-smoke.ps1`、`go test ./...`、`/rekit doctor` 与 `git diff --check` 均通过；`git diff --check` 仅报告 LF/CRLF warning，无 whitespace error。

### Batch 96：pack smoke matrix selftest

状态：已完成。

目标：给 `pack-smoke-matrix.ps1` 自身增加回归测试，覆盖 discovery、JSON envelope、子集去重、unknown pack guard 和文本输出，避免 matrix 脚本演进后只能靠人工命令发现输出格式或 guard 回归。

实施范围：

- 新增 `rekit/tests/pack-smoke-matrix-selftest.ps1`，以子进程方式调用 `pack-smoke-matrix.ps1`。
- 覆盖 `-DiscoveryOnly` 文本输出、`-DiscoveryOnly -Format json` envelope 和 missing/extra/orphan/missing-script 空集合断言。
- 覆盖 `-Packs web-security,generic-binary-re -Format json` 的 `pack-smoke-matrix` envelope、result success/exitCode/output 字段。
- 覆盖重复 pack 选择去重、文本模式 running/summary/原始 smoke 输出，以及 unknown pack 非零退出 guard。
- 保持 matrix、shared helper 和单 pack smoke 验证语义不变。
- 更新 README、Go runtime migration、CHANGELOG 与本计划文档，记录 selftest 用途和验证入口。

边界：本批只新增测试脚本，不改变 runtime、pack manifest、Go backend、PowerShell façade 委托集合、matrix 执行语义或单 pack smoke 覆盖；selftest 使用既有自包含临时 case smoke 子集，不创建真实 case state；不执行样本、网络、debug、dump、patch、hook、fuzz、exploit replay 或任何外部副作用；不写 authority/confirmed。

停止条件：若后续要为 matrix 引入 Pester、JUnit/SARIF、mock backend、并行 failure fixture 或 CI artifact，应作为独立批次评估依赖、输出稳定性和执行时间。

验证：

```powershell
.\rekit\tests\pack-smoke-matrix-selftest.ps1
.\rekit\tests\pack-smoke-matrix.ps1 -DiscoveryOnly
.\rekit\tests\pack-inventory-smoke.ps1
go test ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`pack-smoke-matrix-selftest.ps1`、discovery guard、`pack-inventory-smoke.ps1`、`go test ./...`、`/rekit doctor` 与 `git diff --check` 均通过；`git diff --check` 仅报告 LF/CRLF warning，无 whitespace error。

### Batch 97：rekit tests guide

状态：已完成。

目标：把 `rekit/tests` 中持续增长的 smoke 入口整理成维护指南，降低新会话或后续批次选择验证脚本的成本，并把 pack smoke helper/matrix/selftest 的定位和安全边界写入仓库文件，而不是只留在聊天上下文中。

实施范围：

- 新增 `rekit/tests/README.md`，包含读取指南、实施摘要、执行清单、验证标准、风险与注意事项。
- 按 façade/inventory/pack matrix、pack skeleton smoke、sync/promote/init/attach、workstream/ledger/gate/Agent Team 分类梳理常用 smoke。
- 记录 `pack-smoke-lib.ps1`、`pack-smoke-matrix.ps1`、`pack-smoke-matrix-selftest.ps1` 的用途、何时运行和覆盖重点。
- 记录默认 `WorkRoot`、真实 case 禁忌、no-real-artifact/no-external-side-effect 边界，以及按改动类型选择验证组合的决策表。
- 更新 README、CHANGELOG 与本计划文档，链接 tests guide。

边界：本批只新增测试维护文档，不改变 runtime、test scripts、pack manifest、Go backend、PowerShell façade 委托集合或验证语义；不创建 case state；不执行样本、网络、debug、dump、patch、hook、fuzz、exploit replay 或任何外部副作用；不写 authority/confirmed。

停止条件：若后续要将 tests guide 扩展成 CI pipeline、自动测试选择器、Pester suite 或跨机器测试规范，应作为独立批次评估依赖、执行时间、日志格式和失败定位。

验证：

```powershell
.\rekit\tests\pack-smoke-matrix-selftest.ps1
.\rekit\tests\pack-smoke-matrix.ps1 -DiscoveryOnly
.\rekit\tests\pack-inventory-smoke.ps1
go test ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`pack-smoke-matrix-selftest.ps1`、discovery guard、`pack-inventory-smoke.ps1`、`go test ./...`、`/rekit doctor` 与 `git diff --check` 均通过；`git diff --check` 仅报告 LF/CRLF warning，无 whitespace error。

### Batch 98：verification guide cross-links

状态：已完成。

目标：补齐测试维护指南的跨文档入口，让维护者从根 `CLAUDE.md`、pack authoring、Go runtime migration 和 Agent Team usage 文档都能发现 `rekit/tests/README.md`，同时避免各文档重复维护完整 smoke 表。

实施范围：

- `CLAUDE.md` 常用维护入口新增 `rekit/tests/README.md`，验证命令章节提示先按 tests guide 选择 smoke。
- `docs/pack-authoring.md` 验证标准章节标明完整 smoke 选择表在 `rekit/tests/README.md`，本节只保留 pack 最低标准。
- `docs/go-runtime-migration.md` 验证矩阵章节标明该矩阵专注 Go runtime 迁移，跨类别 smoke 选择见 tests guide。
- `docs/agent-team-usage.md` 短期优化项从“固化 smoke test”更新为已有 tests guide 和 pack smoke matrix，后续可继续推进 CI。
- 更新 CHANGELOG 与本计划文档，记录 tests guide cross-link 范围。

边界：本批只补文档链接和说明，不改变 runtime、test scripts、pack manifest、Go backend、PowerShell façade 委托集合或验证语义；不创建 case state；不执行样本、网络、debug、dump、patch、hook、fuzz、exploit replay 或任何外部副作用；不写 authority/confirmed。

停止条件：若后续要把 tests guide 进一步改造成自动测试选择器、CI pipeline 或机器可读 test catalog，应作为独立批次评估格式、依赖、执行时间和失败定位。

验证：

```powershell
.\rekit\tests\pack-smoke-matrix-selftest.ps1
.\rekit\tests\pack-smoke-matrix.ps1 -DiscoveryOnly
.\rekit\tests\pack-inventory-smoke.ps1
go test ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`pack-smoke-matrix-selftest.ps1`、discovery guard、`pack-inventory-smoke.ps1`、`go test ./...`、`/rekit doctor` 与 `git diff --check` 均通过；`git diff --check` 仅报告 LF/CRLF warning，无 whitespace error。

### Batch 99：test catalog JSON

状态：已完成。

目标：在 `rekit/tests/README.md` 的人工维护指南之外，新增机器可读测试目录，为后续自动测试选择器、CI 或新会话快速筛选 smoke 提供稳定导航，同时明确它不是自动执行器。

实施范围：

- 新增 `rekit/tests/catalog.json`，声明 `schemaVersion`、说明、默认 `WorkRoot`、全局安全边界和推荐最小回归组合。
- 为主要 smoke 记录 `id`、`script`、`category`、`purpose`、`recommendedFor`、`supportsWorkRoot`、`supportsCaseRoot`、`riskBoundary` 与 `relatedDocs`。
- 覆盖 façade、inventory、pack matrix/helper、8 个 skeleton pack smoke、`plan-subagents`、sync/promote、Agent Team review loop、gate parity 和 continue preflight 等代表入口。
- 更新 `rekit/tests/README.md`，说明 `catalog.json` 是机器可读导航，不替代 README，也不自动运行测试。
- 更新 README、CHANGELOG 与本计划文档，记录 catalog 的用途和边界。

边界：本批只新增机器可读测试导航文件和说明，不改变 runtime、test scripts、pack manifest、Go backend、PowerShell façade 委托集合或验证语义；catalog 不自动执行任何命令；不创建 case state；不执行样本、网络、debug、dump、patch、hook、fuzz、exploit replay 或任何外部副作用；不写 authority/confirmed。

停止条件：若后续要让 catalog 驱动自动运行、CI matrix、依赖图、并行执行或测试报告生成，应作为独立批次评估 schema 稳定性、执行时间、失败定位和安全边界。

验证：

```powershell
$catalog = Get-Content -LiteralPath .\rekit\tests\catalog.json -Raw | ConvertFrom-Json
.\rekit\tests\pack-smoke-matrix-selftest.ps1
.\rekit\tests\pack-smoke-matrix.ps1 -DiscoveryOnly
.\rekit\tests\pack-inventory-smoke.ps1
go test ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`catalog.json` 解析断言、`pack-smoke-matrix-selftest.ps1`、discovery guard、`pack-inventory-smoke.ps1`、`go test ./...`、`/rekit doctor` 与 `git diff --check` 均通过；`git diff --check` 仅报告 LF/CRLF warning，无 whitespace error。

### Batch 100：test catalog selftest

状态：已完成。

目标：给 `rekit/tests/catalog.json` 增加自测脚本，锁定 catalog schema、字段完整性、脚本/文档路径、唯一 id 和 pack smoke 与 discovery guard 的一致性，避免机器可读测试导航在后续维护中悄悄漂移。

实施范围：

- 新增 `rekit/tests/catalog-smoke.ps1`，只读解析 `catalog.json` 并校验 `schemaVersion`、说明、默认 `WorkRoot`、全局边界、推荐最小回归组合和 tests 列表。
- 校验每个 test entry 的 `id` 唯一且格式稳定，`category` 在允许集合内，`script`、`purpose`、`recommendedFor`、`supportsWorkRoot`、`supportsCaseRoot`、`riskBoundary` 与 `relatedDocs` 齐全。
- 对 `.ps1` 脚本路径和 related docs 做存在性检查；对 `pack-smoke` entries 校验 pack 名与脚本名匹配、支持 `WorkRoot` 且不支持真实 `CaseRoot`。
- 调用 `pack-smoke-matrix.ps1 -DiscoveryOnly -Format json`，确保 catalog 中的 pack smoke entries 与 schema-valid skeleton packs 完全一致。
- 将 `catalog-smoke.ps1` 写入 `catalog.json`，并更新 `rekit/tests/README.md` 与 README 入口。
- 更新 CHANGELOG 与本计划文档，记录 selftest 的用途和边界。

边界：本批只新增只读 catalog 自测和文档链接，不改变 runtime、test scripts 的执行语义、pack manifest、Go backend、PowerShell façade 委托集合或验证语义；`catalog-smoke.ps1` 不运行 case smoke，只运行 discovery guard；不创建 case state；不执行样本、网络、debug、dump、patch、hook、fuzz、exploit replay 或任何外部副作用；不写 authority/confirmed。

停止条件：若后续要把 catalog selftest 扩展为 JSON Schema 文件、Pester suite、CI 必跑 gate 或自动执行 catalog 中的测试，应作为独立批次评估 schema 版本、执行时间、失败定位和安全边界。

验证：

```powershell
.\rekit\tests\catalog-smoke.ps1
.\rekit\tests\pack-smoke-matrix-selftest.ps1
.\rekit\tests\pack-smoke-matrix.ps1 -DiscoveryOnly
.\rekit\tests\pack-inventory-smoke.ps1
go test ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`catalog-smoke.ps1`、`pack-smoke-matrix-selftest.ps1`、discovery guard、`pack-inventory-smoke.ps1`、`go test ./...`、`/rekit doctor` 与 `git diff --check` 均通过；`git diff --check` 仅报告 LF/CRLF warning，无 whitespace error。

### Batch 101：test catalog full script coverage

状态：已完成。

目标：把 `rekit/tests/catalog.json` 从“主要 smoke 导航”提升为覆盖全部 `rekit/tests/*.ps1` 的测试目录，并让 `catalog-smoke.ps1` 在新增脚本未登记时失败，避免后续测试入口悄悄脱离机器可读导航。

实施范围：

- 扩展 `catalog.json` 覆盖当前全部 32 个 `rekit/tests/*.ps1` 文件，包括 init/bootstrap、sync/promote preflight/apply、workstream、Agent Team dry-run、overview、handoff 与 continue smoke；`pack-smoke-lib.ps1` 保持 `pack-helper` 类别，`pack-smoke-matrix.ps1 -DiscoveryOnly` 作为同脚本的 discovery 命令 entry 保留。
- 新增 `case-scaffold` catalog category，用于区分 init/bootstrap scaffold smoke 与 sync/promote、workstream、pack smoke。
- 增强 `catalog-smoke.ps1`：枚举 `rekit/tests/*.ps1` 并断言每个脚本至少有一个 catalog entry，同时继续校验 id、category、字段完整性、脚本/文档路径、pack smoke 与 discovery guard 一致性。
- 更新 `rekit/tests/README.md`、CHANGELOG 与本计划文档，说明 catalog 现在覆盖全部脚本而非仅常用 smoke。

边界：本批只更新测试导航 metadata、自测校验和文档，不改变任何 smoke 的执行语义、runtime、pack manifest、Go backend、PowerShell façade 委托集合或写入路径；`catalog-smoke.ps1` 仍只运行 discovery guard，不执行 case smoke；不接触真实 case，不执行样本、网络、debug、dump、patch、hook、fuzz、exploit replay，不写 authority/confirmed。

停止条件：若后续要让 catalog 自动执行测试、按 category 生成 CI matrix、引入 JSON Schema 文件或拆分 helper/command 多 entry 的 schema version，应作为独立批次评估执行时间、失败定位和安全边界。

验证：

```powershell
.\rekit\tests\catalog-smoke.ps1
.\rekit\tests\pack-smoke-matrix-selftest.ps1
.\rekit\tests\pack-smoke-matrix.ps1 -DiscoveryOnly
.\rekit\tests\pack-inventory-smoke.ps1
go test ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```
### Batch 102：Go-first convergence planning

状态：已完成。

目标：在 Batch 92-101 连续收束 PowerShell smoke/matrix/catalog/test metadata 后，建立新的阶段导航，避免后续几十到上百轮自主实施继续沿 PowerShell 编排扩张惯性推进；明确下一阶段以 Go deterministic runtime owner、release readiness、Agent Team 真实 dry-run 闭环、PowerShell 收缩、pack-neutral hardening 和新会话接手质量为主线。

实施范围：

- 新增 `docs/go-first-convergence-plan.md`，包含读取指南、实施摘要、执行清单、验证标准、风险与注意事项、Batch 101 后状态基线和 Stage 1-8 阶段方向。
- 在 `README.md` 与根 `CLAUDE.md` 的维护入口中链接该阶段计划，方便新会话接手。
- 在 `CHANGELOG.md` 记录该阶段导航文档。

边界：本批只做阶段性规划与接手导航，不修改 runtime、pack manifest、PowerShell façade 委托集合、Go backend 行为、case schema、sync/promote 语义或测试执行语义；不新增 PowerShell 编排能力；不创建真实 case state；不执行样本、网络、debug、dump、patch、hook、fuzz、exploit replay 或任何外部副作用；不写 authority/confirmed。

下一阶段优先级：按 `docs/go-first-convergence-plan.md` 推进，优先 Stage 1 Go-first release gate 与 invariant migration，然后 Stage 2 低风险 Go default façade，再逐步推进 case lifecycle、sync/promote、workstream/ledger/gate/continue、Agent Team dry-run、pack-neutral hardening 与 CI/release readiness。

验证：本批为文档/规划批次，至少执行 `git diff --check`；后续实施批次按 `docs/go-first-convergence-plan.md` 的验证标准选择 Go tests、doctor、façade smoke 与临时 case smoke。

### Batch 103：Go-first release invariant tests

状态：已完成。

目标：承接 `docs/go-first-convergence-plan.md` Stage 1，把 release readiness 中依赖 PowerShell catalog/matrix/discovery 的确定性 invariant 迁入 Go tests，让 Go tests 成为后续 release gate 的主要信号之一。

实施范围：

- 新增 `internal/rekit/manifest/release_invariants_test.go`，读取 `rekit/tests/catalog.json` 并校验 schema version、recommended minimum、全量脚本覆盖、related docs 存在性和字段完整性。
- 增加 skeleton pack smoke discovery invariant，确保 `/rekit packs`、pack smoke matrix、wrapper 和 catalog 中的安全领域 skeleton pack 覆盖一致。
- 在 Go CLI tests 中补默认 pack status JSON contract，锁定默认 pack 与机器可读输出契约。
- 将 `go vet ./...` 纳入 release readiness recommended minimum，推动 release gate 从 PowerShell smoke/catalog 扩张转向 Go-first checks。

边界：本批只迁移 release invariant 与测试契约；不改变 runtime 行为、PowerShell façade 委托集合、pack manifest 语义、case schema 或 sync/promote 写入路径；不创建真实 case state，不执行样本、网络、debug、dump、patch、hook、fuzz、exploit replay 或任何外部副作用。

验证：

```powershell
go test ./internal/rekit/manifest
go test ./internal/rekit/cli
go test ./...
go vet ./...
.\rekit\tests\catalog-smoke.ps1
.\rekit\tests\pack-smoke-matrix.ps1 -DiscoveryOnly
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。实施中曾发现 `catalog.json` 被误写入无关文本导致 JSON 解析失败，已清理；最终 `go test`、`go vet`、catalog smoke、pack smoke discovery、doctor 与 `git diff --check` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 104：read-only Go default façade

状态：已完成。

目标：承接 Stage 2，让低风险 read-only 命令默认由 Go backend 作为 deterministic runtime owner，PowerShell 收缩为 façade / fallback / legacy compatibility，同时保留 `REKIT_GO_DISABLE=1` 作为快速止损。

实施范围：

- `rekit/rekit.ps1` 将 `status`、`packs`、kit/case `doctor` / `validate` 加入默认 Go delegation command 集合；`REKIT_GO_DISABLE=1` 优先级最高，可强制回退 PowerShell。
- `REKIT_GO_ENABLE=1` 保留给扩展 preview/review 安全集合，不因 read-only 默认委托而扩大写入面。
- `facade-smoke.ps1` 增加默认 read-only Go 委托与 fake `REKIT_GO_EXE` sentinel 覆盖，证明未设置 `REKIT_GO_ENABLE` 时也会经 façade 委托 Go，并保留 disable fallback、gate 默认拒绝和文本/write fallback 覆盖。
- `pack-inventory-smoke.ps1` 的 façade sentinel 改为默认委托断言，继续覆盖 packs/status/doctor/validate 参数透传与 disable fallback。
- README、canonical `/rekit` skill、根 `CLAUDE.md`、`rekit/tests/README.md`、`docs/go-runtime-migration.md` 与 `docs/go-first-convergence-plan.md` 同步说明 read-only Go runtime 已成为默认路径。

边界：本批只调整低风险 read-only façade 默认委托；不委托 attach/repair/init/bootstrap 写入，不委托 sync/promote actual apply/candidates 写入，不委托 note append、工作线文本/apply workflow、内部命令、authority/confirmed 更新或 heavy-tool 执行；不修改 pack manifest、case schema、sync/promote 写入语义或 Go backend 命令行为。

验证：

```powershell
.\rekit\tests\facade-smoke.ps1
.\rekit\tests\pack-inventory-smoke.ps1
go test ./...
go vet ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 105：case lifecycle Go default façade

状态：已完成。

目标：承接 Stage 3，把边界清晰的新 case 创建、绑定和 metadata 修复路径从 PowerShell fallback 推进到 Go deterministic runtime owner；公共 `/rekit` 入口稳定不变，但 `attach`、`repair`、`init/bootstrap` 的预览与显式写入默认经 Go backend 执行。

实施范围：

- `rekit/rekit.ps1` 将 `attach`、`repair`、`init`、`bootstrap` 加入默认 Go delegation command 集合；`REKIT_GO_DISABLE=1` 仍可强制回退 PowerShell。
- lifecycle 安全集合保持明确边界：`attach` 只在 `-WhatIf` 或显式 `-Apply` 时委托；`repair` 默认/`-WhatIf` 为非写入预览，显式 `-Apply` 才写；`init/bootstrap` 只在 `-WhatIf` 或显式 `-Apply` 时委托，裸命令继续走 legacy fallback。
- Go `casebind.WriteInstance` 补齐 PowerShell binding baseline：写 `.rekit/instance.yml` 后同步创建/刷新 legacy `.re-template.yml`，并在缺失时创建初始 `.rekit/state.json`（含空 `managed` 与 `promote.candidates`）。
- Go `attach` / `repair` preview 与 apply 的 writes 列表同步报告 legacy metadata 与 initial state；相关 Go tests 更新为锁定 4 项 binding writes，并验证 state / legacy metadata 内容。
- `facade-smoke.ps1` 与 `init-bootstrap-smoke.ps1` 增加 fake `REKIT_GO_EXE` sentinel，证明 case lifecycle 预览/显式 apply 在未设置 `REKIT_GO_ENABLE` 时也默认委托 Go，同时保留 `REKIT_GO_DISABLE=1` fallback 覆盖。
- README、canonical `/rekit` skill、根 `CLAUDE.md`、`docs/go-runtime-migration.md`、`docs/go-first-convergence-plan.md`、`docs/init-bootstrap-migration.md`、`docs/case-migration.md`、`rekit/tests/README.md` 与 CHANGELOG 同步记录 Stage 3 当前进度、边界和验证入口。

边界：本批只迁移 case lifecycle 中可界定的 metadata/thin-shim/case scaffold 写入；不委托裸 `init/bootstrap` 到 Go，不迁移 sync/promote 实际 apply/candidates，不改变 review-first 语义，不写 pack source、authority/confirmed、board/facts/lanes/handoff/runs，不执行样本、网络、debug、dump、patch、hook、fuzz、exploit replay 或任何外部副作用；case-local shim 仍只复制 canonical thin shim，不复制 runtime 逻辑。

验证：

```powershell
go test ./internal/rekit/cli ./internal/rekit/attach ./internal/rekit/repair ./internal/rekit/sync
go test ./...
go vet ./...
.\rekit\tests\init-bootstrap-smoke.ps1
.\rekit\tests\facade-smoke.ps1
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。最终验证中 `go test ./...`、`go vet ./...`、`init-bootstrap-smoke.ps1`、`facade-smoke.ps1`、`/rekit doctor` 与 `git diff --check` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 106：sync Go default façade

状态：已完成。

目标：承接 Stage 4 的第一组 vertical slice，将 `/rekit sync` review、`sync -Apply` 实际写入和 `sync -Apply -WhatIf -Format json` 非写入 preview 收口为默认 Go façade 委托；PowerShell 保留为 `REKIT_GO_DISABLE=1` fallback 与文本 dry-run compatibility。

实施范围：

- `rekit/rekit.ps1` 将 `sync` / `update` 加入默认 Go delegation command 集合。
- `Test-RekitGoDelegationSafe` 对 sync/update 增加边界：目标必须是 attached case；review/default sync、actual `-Apply` 和 `-Apply -WhatIf -Format json` 可委托；文本 `sync -Apply -WhatIf` 继续走 PowerShell dry-run fallback；`-Review` 与 review artifact options 和 `-Apply` 不混用；`REKIT_GO_DISABLE=1` 仍最高优先级回退 PowerShell。
- `facade-smoke.ps1` 增加 default sync review/apply fake backend sentinel，并把 sync apply JSON preview 从显式 enable 委托提升为默认委托断言，同时保留文本 what-if fallback。
- `sync-apply-smoke.ps1` 将 façade preview/apply sentinel 改为默认委托，增加 disable fallback 覆盖；实际 apply 路径通过公共 façade 执行。
- `sync-apply-parity-smoke.ps1` 将 PowerShell 侧改为 `REKIT_GO_DISABLE=1` fallback，将 Go 侧改为默认 façade apply/force，继续比对 managed docs、template target、managed block、metadata/shim 和 `.rekit/state.json` parity，并增加 fake backend sentinel 证明 default apply 委托与 disable fallback。
- Go CLI tests 增加 `sync -Apply` 写入 managed content、backup、managed block 与 state refresh 的契约覆盖。
- `internal/rekit/sync.ApplyPreview` nextSteps 更新，不再声称 actual writes 不经 façade 委托。
- README、canonical `/rekit` skill、根 `CLAUDE.md`、`docs/go-runtime-migration.md`、`docs/go-first-convergence-plan.md`、`docs/sync-apply-migration.md`、`docs/case-migration.md`、`docs/agent-team-usage.md`、`rekit/tests/README.md` 与 CHANGELOG 同步记录 sync 默认 Go 委托边界。

边界：本批只收口 sync 侧 `kit -> case` 路径；仍保持 review-first，即 `/rekit sync` 默认只写 review artifacts，不写 managed docs，写入 managed docs / managed block / template create-if-missing / sync state 仍要求显式 `-Apply`；不迁移 promote review/candidates/apply，不写 pack source、promote candidates、tooling candidates、authority/confirmed，不执行 heavy-tool、网络、debug、dump、patch、hook、fuzz、exploit replay；不创建真实 case state，验证仅使用临时 case。

验证计划：

```powershell
go test ./internal/rekit/cli ./internal/rekit/sync
.\rekit\tests\sync-apply-smoke.ps1
.\rekit\tests\sync-apply-parity-smoke.ps1
.\rekit\tests\sync-review-parity-smoke.ps1
.\rekit\tests\facade-smoke.ps1
go test ./...
go vet ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。targeted `go test ./internal/rekit/cli ./internal/rekit/sync` 与 `sync-apply-smoke.ps1` 已先行通过；最终收尾验证中 `sync-apply-parity-smoke.ps1`、`sync-review-parity-smoke.ps1`、`facade-smoke.ps1`、`go test ./...`、`go vet ./...`、`/rekit doctor` 与 `git diff --check` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 107：promote non-writing Go default façade

状态：已完成。

目标：承接 Stage 4 的第二组 vertical slice，将 `/rekit promote` review、review artifact 写入、`promote -CreateCandidates -WhatIf -Format json` 与 `promote -Apply -WhatIf -Format json` 非写入 preview 收口为默认 Go façade 委托；保留 promote candidate/apply 实际写入为 PowerShell fallback 或手动 Go CLI。

实施范围：

- `rekit/rekit.ps1` 将 `promote` 加入默认 Go delegation command 集合。
- `Test-RekitGoDelegationSafe` 对 promote 增加边界：目标必须是 attached case；默认 review、review artifact 写入、candidate JSON what-if preview 与 apply JSON what-if preview 可委托；文本 promote what-if 与实际 candidate/apply 写入继续 fallback；`REKIT_GO_DISABLE=1` 仍最高优先级回退 PowerShell。
- `facade-smoke.ps1` 增加 default promote review/artifacts/candidate JSON preview/apply JSON preview fake backend sentinel，并保留文本 what-if 与 invalid write combination fallback 覆盖。
- `promote-candidates-preflight-smoke.ps1` 与 `promote-apply-preflight-smoke.ps1` 将 façade JSON preview sentinel 改为默认委托，并增加 disable fallback 覆盖。
- `promote-candidates-apply-smoke.ps1` 与 `promote-apply-smoke.ps1` 保留文本 what-if fallback 覆盖，继续证明实际写入路径不经 default façade 委托。
- README、canonical `/rekit` skill、根 `CLAUDE.md`、`docs/go-runtime-migration.md`、`docs/go-first-convergence-plan.md`、`docs/promote-candidates-migration.md`、`docs/promote-apply-migration.md`、`docs/promote-sync.md`、`rekit/tests/README.md` 与 CHANGELOG 同步记录 promote 非写入默认 Go 委托边界。

边界：本批只收口 promote 侧非写入 review/preview 路径；review artifact 写入仍只代表 `.rekit/reviews/**` 或显式 artifact path，不写 pack source、promote candidates、tooling candidates、authority/confirmed；`promote -CreateCandidates` 实际候选写入与 `promote -Apply` 实际 pack source 写入仍不默认委托 Go；不执行 heavy-tool、网络、debug、dump、patch、hook、fuzz、exploit replay；验证仅使用临时 case。

验证计划：

```powershell
.\rekit\tests\promote-candidates-preflight-smoke.ps1
.\rekit\tests\promote-apply-preflight-smoke.ps1
.\rekit\tests\promote-candidates-apply-smoke.ps1
.\rekit\tests\promote-apply-smoke.ps1
.\rekit\tests\facade-smoke.ps1
go test ./...
go vet ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`promote-candidates-preflight-smoke.ps1`、`promote-apply-preflight-smoke.ps1`、`promote-candidates-apply-smoke.ps1`、`promote-apply-smoke.ps1`、`facade-smoke.ps1`、`go test ./...`、`go vet ./...` 与 `/rekit doctor` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 108：promote candidate write Go default façade

状态：已完成。

目标：承接 Stage 4 的第三组 vertical slice，将 `/rekit promote -CreateCandidates` 实际 candidate/index/tooling candidate 写入纳入默认 Go façade 委托；保留 review-first 用户确认边界、文本 what-if fallback、`REKIT_GO_DISABLE=1` fallback 与 `promote -Apply` 实际 pack source 写入 fallback/manual 边界。

实施范围：

- `rekit/rekit.ps1` 扩展 promote 默认委托 safe set：attached case 且非 `-Force`、非 review artifact 混用时，`promote -CreateCandidates` 实际候选写入默认委托 Go；`-CreateCandidates -WhatIf -Format json` 与 `-Apply -WhatIf -Format json` 继续默认委托 Go；文本 promote what-if 继续走 PowerShell fallback；`promote -Apply` 实际 pack source 写入仍不默认委托。
- `facade-smoke.ps1` 增加 default `promote -CreateCandidates` fake backend sentinel，证明实际 candidate 写入组合确实经默认 Go façade 委托，同时保留文本 what-if fallback 与 invalid apply/create combination fallback。
- `promote-candidates-preflight-smoke.ps1` 增加默认 façade actual create-candidates fake backend sentinel，并保留 PowerShell 文本 what-if baseline、Go review artifact/sanitized preview、JSON preview delegation、disable fallback 与 no-write guard。
- `promote-candidates-apply-smoke.ps1` 将实际 candidate 写入入口从手动 Go CLI 改为公共 PowerShell façade，继续验证 managed candidate、`index.json`、tooling candidate、blocked deny、sanitization、pack-root containment 与 cleanup。
- README、canonical `/rekit` skill、根 `CLAUDE.md`、`docs/go-runtime-migration.md`、`docs/go-first-convergence-plan.md`、`docs/promote-candidates-migration.md`、`docs/promote-sync.md`、`rekit/tests/README.md` 与 CHANGELOG 同步记录 Batch 108 默认委托边界。

边界：本批只把 candidate 目录写入从 PowerShell fallback/手动 Go CLI 收口到默认 Go façade；`promote -CreateCandidates` 仍只写 `packs/<pack>/promote-candidates/**` 与 `packs/<pack>/tooling/candidates/**` 候选，不覆盖正式 pack source，不写 authority/confirmed，不执行 heavy-tool、网络、debug、dump、patch、hook、fuzz、exploit replay；`promote -Apply` 实际 pack source 写入继续 fallback/manual；验证仅使用临时 case，smoke 必须恢复 candidate/tooling candidate tree。

验证计划：

```powershell
.\rekit\tests\promote-candidates-preflight-smoke.ps1
.\rekit\tests\promote-candidates-apply-smoke.ps1
.\rekit\tests\facade-smoke.ps1
go test ./...
go vet ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`promote-candidates-preflight-smoke.ps1`、`promote-candidates-apply-smoke.ps1`、`promote-apply-preflight-smoke.ps1`、`promote-apply-smoke.ps1`、`facade-smoke.ps1`、`go test ./...`、`go vet ./...` 与 `/rekit doctor` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 109：promote package test hardening

状态：已完成。

目标：承接 Stage 4 后续收敛，把 promote candidates/apply 中可单元化的 deny、sanitize、candidate index、containment/no-write 与 restore helper 行为迁入 Go package tests，减少仅依赖 PowerShell smoke 才能发现 promote 写入边界回归的风险。

实施范围：

- 新增 `internal/rekit/promote/promote_test.go`，构造自包含临时 repo、unit pack 与 attached case，不依赖真实 case、不写真实 pack。
- 覆盖 `CreateCandidates(..., WhatIf:true)` 的 no-write 契约：返回 candidate/index/tooling 预览但不创建 `promote-candidates/**`、`tooling/candidates/**` 或 `index.json`。
- 覆盖 `CreateCandidates(..., WhatIf:false)` 的实际 candidate/index/tooling candidate 写入：managed candidate 写入 candidate root，blocked deny 只记录 pack source target，不写 candidate；tooling candidate 写入 tooling candidate root；`index.json` 指向 managed candidate。
- 覆盖 tooling sanitization：case root、绝对路径、artifacts/captures path、trace/dump 文件名、address、ctx/round/task 等 case-specific 信息替换为占位符。
- 覆盖 `uniqueCandidatePath` collision handling 与 `restorePromoteBackups` 只恢复 action=`promote` 的 backup helper 行为。

边界：本批只增加 Go package tests，不改变 runtime 行为、不扩大 façade 委托集合、不写真实 pack candidate、pack source、authority/confirmed，不执行 heavy-tool、网络、debug、dump、patch、hook、fuzz、exploit replay；所有测试数据使用 `t.TempDir()` 自包含 repo/case/pack。

验证计划：

```powershell
go test ./internal/rekit/promote
go test ./internal/rekit/cli ./internal/rekit/promote
.\rekit\tests\promote-candidates-preflight-smoke.ps1
.\rekit\tests\promote-candidates-apply-smoke.ps1
.\rekit\tests\promote-apply-smoke.ps1
go test ./...
go vet ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`go test ./internal/rekit/promote`、`go test ./internal/rekit/cli ./internal/rekit/promote`、`promote-candidates-preflight-smoke.ps1`、`promote-candidates-apply-smoke.ps1`、`promote-apply-smoke.ps1`、`go test ./...`、`go vet ./...` 与 `/rekit doctor` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 110：sync package test hardening

状态：已完成。

目标：承接 Stage 4 后续收敛，把 sync apply/preview 中可单元化的 backup、managed block、template create/force、state refresh 与 backup escape guard 行为迁入 Go package tests，减少仅依赖 PowerShell smoke 才能发现 sync 写入边界回归的风险。

实施范围：

- 新增 `internal/rekit/sync/sync_test.go`，构造自包含临时 repo、unit pack 与 attached case，不依赖真实 case、不写真实 pack。
- 覆盖 `ApplyPreview` no-write 契约：返回 backupRoot 和 planned writes，但不创建 backup 目录，不刷新 metadata/shim/state，不修改 managed docs、template target 或 managed block host。
- 覆盖 `Apply` 实际写入：刷新 instance/legacy metadata/thin shim，覆盖 managed file 并创建 backup，跳过既有 local template，替换 managed block 并备份 host，刷新 `.rekit/state.json` managed hashes。
- 覆盖 `ForceLocalTemplates`：显式 force 时覆盖 template target、替换 `<PROJECT_NAME>` / `<PROJECT_ROOT>` 占位符，并备份原本地模板。
- 覆盖 `backupCaseFile` 越界 guard：拒绝备份 case root 外路径，防止 backup helper 被误用时越界读取。

边界：本批只增加 Go package tests，不改变 runtime 行为、不扩大 façade 委托集合、不写真实 case、pack source、promote candidates、authority/confirmed，不执行 heavy-tool、网络、debug、dump、patch、hook、fuzz、exploit replay；所有测试数据使用 `t.TempDir()` 自包含 repo/case/pack。

验证计划：

```powershell
go test ./internal/rekit/sync
go test ./internal/rekit/cli ./internal/rekit/sync
.\rekit\tests\sync-apply-smoke.ps1
.\rekit\tests\sync-apply-parity-smoke.ps1
.\rekit\tests\sync-review-parity-smoke.ps1
go test ./...
go vet ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`go test ./internal/rekit/sync`、`go test ./internal/rekit/cli ./internal/rekit/sync`、`sync-apply-smoke.ps1`、`sync-apply-parity-smoke.ps1`、`sync-review-parity-smoke.ps1`、`go test ./...`、`go vet ./...` 与 `/rekit doctor` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 111：promote apply restore test hardening

状态：已完成。

目标：承接 Stage 4 后续收敛，把 promote apply 中可单元化的 what-if no-write、backup/validation rows、blocked deny 与 validation failure restore 行为迁入 Go package tests，减少 `promote -Apply` 实际 pack source 写入边界只靠 PowerShell smoke 才能发现回归的风险。

实施范围：

- 扩展 `internal/rekit/promote/promote_test.go`，在自包含临时 repo、unit pack 与 attached case 中覆盖 `promote.Apply`。
- 覆盖 `Apply(..., WhatIf:true)`：输出 `would-promote` 与 backup preview path，但不写 pack source、不创建 backup。
- 覆盖 `Apply(..., WhatIf:false)` 成功路径：safe managed doc 写回 pack source，写入前创建 backup，blocked deny 文件不写，写入后返回 pack validation rows。
- 覆盖 validation failure restore：写入后 doctor/pack validation 失败时返回 restore 提示，并用 backup 恢复已写 pack source。
- 复用自包含 fixture 构造最小 policy registry、manifest、laneTypes、authorityFiles 和 budget，避免触碰真实 pack source 或真实 case。

边界：本批只增加 Go package tests，不改变 runtime 行为、不扩大 façade 委托集合；`promote -Apply` 实际 pack source 写入仍不默认委托 PowerShell façade，不写 authority/confirmed，不执行 heavy-tool、网络、debug、dump、patch、hook、fuzz、exploit replay；所有测试数据使用 `t.TempDir()` 自包含 repo/case/pack。

验证计划：

```powershell
go test ./internal/rekit/promote
go test ./internal/rekit/cli ./internal/rekit/promote
.\rekit\tests\promote-apply-preflight-smoke.ps1
.\rekit\tests\promote-apply-smoke.ps1
.\rekit\tests\promote-candidates-apply-smoke.ps1
go test ./...
go vet ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`go test ./internal/rekit/promote`、`go test ./internal/rekit/cli ./internal/rekit/promote`、`promote-apply-preflight-smoke.ps1`、`promote-apply-smoke.ps1`、`promote-candidates-apply-smoke.ps1`、`go test ./...`、`go vet ./...` 与 `/rekit doctor` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 112：promote apply actual write default Go façade

状态：已完成。

目标：在 Batch 111 已把 `promote.Apply` 的 what-if no-write、backup/validation rows、blocked deny 与 validation failure restore 收进 Go package tests 后，将公共 `/rekit promote -Apply` 实际 pack source 写入纳入默认 Go façade，推进 Stage 4 的 sync/promote ownership 收口，减少 PowerShell promote apply 继续作为默认 runtime owner 的漂移风险。

实施范围：

- 放宽 `rekit/rekit.ps1` 的 promote delegation safe-set：`-Apply` 不再只允许 `-WhatIf -Format json`，实际 `-Apply` 在 attached case、非 `-Force`、无 review artifacts、未混用 `-CreateCandidates`/`-Review` 且 `Format` 为空或 `json` 时默认委托 Go。
- 保留 `promote -Apply -WhatIf -Format json` 非写入预览委托；文本 `promote -Apply -WhatIf` 继续 PowerShell dry-run fallback。
- 保留 `REKIT_GO_DISABLE=1` 强制回退 PowerShell，作为 baseline/parity/escape hatch。
- 更新 `facade-smoke.ps1`，用 fake `REKIT_GO_EXE` 锁定 actual `promote -Apply` 默认委托与非法 `-Apply -CreateCandidates` fallback。
- 更新 `promote-apply-preflight-smoke.ps1`，显式验证 actual apply 默认委托 Go；PowerShell actual apply baseline 改为 `REKIT_GO_DISABLE=1` fallback。
- 更新 `promote-apply-smoke.ps1`，通过公共 façade 执行 actual apply，继续验证 backup、blocked deny、validation rows、pack-root containment、tooling 不写入和 cleanup。

边界：仍必须先经 `/rekit promote` review 与用户确认具体范围后才能运行 `-Apply`；本批只切换明确 `-Apply` 的 runtime owner，不改变 review-first 语义，不允许裸 `promote` 写入，不写 authority/confirmed，不执行 heavy-tool、网络、debug、dump、patch、hook、fuzz、exploit replay。`promote -Apply` 仍只处理 manifest 中 safe managed promote files，依赖 deny/case-specific pattern、pack-root containment、backup、doctor validation 与 validation failure restore；文本 what-if 与 `REKIT_GO_DISABLE=1` fallback 保留 PowerShell。

验证计划：

```powershell
go test ./internal/rekit/promote
go test ./internal/rekit/cli ./internal/rekit/promote
.\rekit\tests\promote-apply-preflight-smoke.ps1
.\rekit\tests\promote-apply-smoke.ps1
.\rekit\tests\promote-candidates-apply-smoke.ps1
.\rekit\tests\facade-smoke.ps1
.\rekit\tests\catalog-smoke.ps1
go test ./...
go vet ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`go test ./internal/rekit/promote`、`go test ./internal/rekit/cli ./internal/rekit/promote`、`promote-apply-preflight-smoke.ps1`、`promote-apply-smoke.ps1`、`promote-candidates-apply-smoke.ps1`、`facade-smoke.ps1`、`catalog-smoke.ps1`、`go test ./...`、`go vet ./...` 与 `/rekit doctor` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 113：overview / note JSON readonly default Go façade

状态：已完成。

目标：承接 Stage 5 的 workstream / ledger / gate / continue Go 化，在不新增写入面的前提下，将已初始化 case 的 `/rekit overview -Format json` 与 `/rekit note -List -Format json` 只读查询从显式 `REKIT_GO_ENABLE=1` 扩展集合升级为默认 Go façade，推进 Agent Team 状态读取路径收口，减少 PowerShell 继续作为机器可读 read layer owner 的漂移风险。

实施范围：

- 扩展 `rekit/rekit.ps1` 默认委托集合，将 `overview` 与 `note` 纳入默认 Go delegation command set，但仍复用原 safe-set guard：`overview` 只在 attached case 且 `.rekit/board.json` 已存在、`Format=json`、无写入 flags 时委托；`note` 只在 attached case、`-List`、`Format=json`、无 `-WhatIf`/写入 flags 时委托。
- 保留 overview 文本输出与缺 board 初始化走 PowerShell fallback，避免改变首次 overview 初始化语义。
- 保留 `note` append、`note -WhatIf` 与文本 `note -List` 走 PowerShell fallback，避免扩大 facts JSONL 写入委托面。
- 保留 `REKIT_GO_DISABLE=1` 强制回退 PowerShell，并在 façade smoke 中增加 overview/note JSON readonly disable fallback 检查。
- 更新 `facade-smoke.ps1`、`overview-readonly-smoke.ps1` 与 `agent-team-review-loop-smoke.ps1`，用 fake `REKIT_GO_EXE` 锁定 overview/note JSON readonly 默认委托，并保留文本 fallback 检查。
- 更新 `CLAUDE.md`、README、`/rekit` skill、Go runtime migration、Go-first convergence、tests guide、catalog metadata 与 changelog，记录 Batch 113 后默认委托边界。

边界：本批只切换两个机器可读只读查询的 façade owner，不写 board/facts/lanes/handoff/authority/confirmed，不执行 heavy-tool、网络、debug、dump、patch、hook、fuzz、exploit replay；不委托 overview 文本/缺 board 初始化、不委托 note append/text list、不改变 gate/workstream preview 仍需显式 `REKIT_GO_ENABLE=1` 的边界。

验证计划：

```powershell
go test ./internal/rekit/cli ./internal/rekit/overview ./internal/rekit/note
.\rekit\tests\facade-smoke.ps1
.\rekit\tests\overview-readonly-smoke.ps1
.\rekit\tests\agent-team-review-loop-smoke.ps1
.\rekit\tests\catalog-smoke.ps1
go test ./...
go vet ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`go test ./internal/rekit/cli ./internal/rekit/overview ./internal/rekit/note`、`facade-smoke.ps1`、`overview-readonly-smoke.ps1`、`agent-team-review-loop-smoke.ps1`、`catalog-smoke.ps1`、`go test ./...`、`go vet ./...` 与 `/rekit doctor` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 114：gate what-if default Go façade

状态：已完成。

目标：承接 Stage 5 的 workstream / ledger / gate / continue Go 化，在不新增实际 heavy-tool 执行或 ledger 写入面的前提下，将 attached case 的 `/rekit gate -WhatIf` 非写入 pending-gate request preview 从显式 `REKIT_GO_ENABLE=1` 扩展集合升级为默认 Go façade，进一步收口 gate preview 的 deterministic runtime owner。

实施范围：

- 扩展 `rekit/rekit.ps1` 默认委托集合，将 `gate` 纳入 default delegation command set，但 safe-set 只允许 attached case、显式 `-WhatIf`、无 `-Apply`/review/candidate/force flags、无 review artifact options，且 `-Format` 为空或 `json` 的非写入 preview。
- 保留 `gate -Apply` 不经默认 façade 委托；该路径仍只能手动运行 Go CLI，且只写 pending-gate request，不执行 heavy-tool、不写 confirmed/authority。
- 保留 actual full-trace/debug/inject/patch/dump/network/fuzz/exploit replay 等 heavy-tool 执行完全不由本批迁移或授权。
- 更新 `facade-smoke.ps1`，用 fake `REKIT_GO_EXE` 锁定默认 gate what-if 委托，并用真实默认 façade dry-run 检查 `isMutation=false` 与 `pending-gate`；同时确认 `gate -Apply` 仍不经默认 façade。
- 更新 `gate-parity-smoke.ps1`，在 Go `gate -Apply` 落账前增加默认 façade `gate -WhatIf` no-write 检查，比较 `.rekit/facts/requests.jsonl` 前后内容不变。
- 更新 README、CLAUDE、`/rekit` skill、Go runtime migration、Go-first convergence、tests guide、catalog metadata 与 changelog，记录 Batch 114 后 gate what-if 默认委托、workstream preview 仍需显式 enable 的边界。

边界：本批只切换 `gate -WhatIf` preview 的 façade owner，不写 board/facts/lanes/handoff/authority/confirmed，不执行 heavy-tool、网络、debug、dump、patch、hook、fuzz、exploit replay；不委托 `gate -Apply`、start/handoff/continue workstream preview/apply、note append、overview 文本/缺 board 初始化或任何文本工作线命令。

验证计划：

```powershell
go test ./internal/rekit/cli ./internal/rekit/gate
.\rekit\tests\facade-smoke.ps1
.\rekit\tests\gate-parity-smoke.ps1
.\rekit\tests\catalog-smoke.ps1
go test ./...
go vet ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`go test ./internal/rekit/cli ./internal/rekit/gate`、`facade-smoke.ps1`、`gate-parity-smoke.ps1`、`catalog-smoke.ps1`、`go test ./...`、`go vet ./...` 与 `/rekit doctor` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 115：gate apply default Go façade

状态：已完成。

目标：承接 Stage 5 的 gate request 收口，在 Batch 114 已将 `/rekit gate -WhatIf` 默认委托 Go 后，将 attached case 的 `/rekit gate -Apply` pending-gate request ledger 写入也升级为默认 Go façade，同时保持 gate apply 只登记请求、不执行 heavy-tool、不写 confirmed/authority 的边界。

实施范围：

- 扩展 `rekit/rekit.ps1` gate safe-set：`gate` 默认委托允许 attached case 的显式 `-WhatIf` 或显式 `-Apply`，禁止二者混用，继续拒绝 `-CreateCandidates`/`-Review`/`-Force`、review artifact options，以及非空非 `json` format。
- 保留 `gate -Apply` 的 Go backend actor guard：缺少 `-Actor` 时拒绝写入；成功时只 append `.rekit/facts/requests.jsonl` 的 `pending-gate` request。
- 新增 `internal/rekit/gate/gate_test.go` package tests，覆盖 dry-run no-write、apply actor guard、pending-gate request 写入、duplicate eventId 幂等，以及不写 authority/confirmed/captures/artifacts。
- 更新 `facade-smoke.ps1`，用 fake `REKIT_GO_EXE` 锁定默认 `gate -Apply` 委托，并保留 missing actor guard 走 Go backend 的检查。
- 更新 `gate-parity-smoke.ps1`，将 request 写入路径改为默认 façade `gate -Apply`，继续验证 façade `gate -WhatIf` no-write 以及 overview/note/handoff 展示 parity。
- 更新 README、CLAUDE、`/rekit` skill、Go runtime migration、Go-first convergence、Agent Team rollout、tests guide、catalog metadata 与 changelog，记录 Batch 115 后 gate apply 默认委托边界。

边界：本批只把 pending-gate request ledger 写入的 façade owner 切到 Go，不执行 full-trace/debug/inject/patch/dump/network/fuzz/exploit replay，不写 confirmed/authority，不改变 note append、overview 文本/缺 board 初始化、start/handoff/continue workstream preview/apply 或其它文本工作线命令。

验证计划：

```powershell
go test ./internal/rekit/gate ./internal/rekit/cli
.\rekit\tests\facade-smoke.ps1
.\rekit\tests\gate-parity-smoke.ps1
.\rekit\tests\catalog-smoke.ps1
go test ./...
go vet ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`go test ./internal/rekit/gate ./internal/rekit/cli`、`facade-smoke.ps1`、`gate-parity-smoke.ps1`、`catalog-smoke.ps1`、`go test ./...`、`go vet ./...` 与 `/rekit doctor` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 116：note append default Go façade

状态：已完成。

目标：承接 Stage 5 的 ledger runtime 收口，在 Batch 113 已将 `note -List -Format json` 默认委托 Go 后，将 attached case 的 `/rekit note` append 与 `/rekit note -WhatIf` preview 升级为默认 Go façade，同时保持只写 facts JSONL、schema/lane guard、eventId 去重、不写 authority/confirmed、不执行 heavy-tool 的边界。

实施范围：

- 扩展 `rekit/rekit.ps1` 的 `note` safe-set：attached case、非 `-Apply`/`-CreateCandidates`/`-Review`/`-Force`、无 review artifact options 时，`note -List -Format json` 继续默认委托 Go；非 list 的 append 与 append `-WhatIf` 在 `Format` 为空或 `json` 时默认委托 Go；文本 `note -List` 继续 PowerShell fallback。
- 扩展 `Get-RekitGoArgs` 的 note 参数转发，将 append 所需的 `Kind`、`Lane`、`Subject`、`Summary`、`Actor`、`Risk`、`Related`、`Confidence`、`Decision`、`Reason`、`Status`、`BatchId`、`TargetRef`、`Verifier`、`Verdict`、`Action`、`ApprovedBy`、`Scope`、`Expires`、`EvidenceRefs` 与 `EventId` 转给 Go backend。
- 新增 `internal/rekit/note/note_test.go` package tests，覆盖 append what-if no-write、append + list、duplicate explicit eventId 幂等、invalid kind / unknown lane guard，以及不写 authority/confirmed。
- 更新 `facade-smoke.ps1`，用 fake `REKIT_GO_EXE` 锁定默认 `note` append 与 `note -WhatIf` 委托，并补 `REKIT_GO_DISABLE=1` 下 note append fallback 检查。
- 更新 `agent-team-review-loop-smoke.ps1`，通过公共 façade 验证 note what-if 不写 observations ledger、verification append 与 decision append JSON envelope，并继续验证 note/list、overview、handoff 展示闭环。
- 更新 README、CLAUDE、`/rekit` skill、Go runtime migration、Go-first convergence、Agent Team rollout、tests guide、catalog metadata 与 changelog，记录 Batch 116 后 note append 默认委托边界。

边界：本批只切换 facts JSONL append/preview 的 façade owner；append 输出从 PowerShell 文本变成 Go JSON envelope 是预期机器可读行为；不写 board/lane/handoff/authority/confirmed，不执行 full-trace/debug/inject/patch/dump/network/fuzz/exploit replay，不改变 overview 文本/缺 board 初始化、文本 `note -List`、start/handoff/continue workstream preview/apply 或其它文本工作线命令。

验证计划：

```powershell
go test ./internal/rekit/note ./internal/rekit/cli
.\rekit\tests\facade-smoke.ps1
.\rekit\tests\agent-team-review-loop-smoke.ps1
.\rekit\tests\catalog-smoke.ps1
go test ./...
go vet ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`go test ./internal/rekit/note ./internal/rekit/cli`、`facade-smoke.ps1`、`agent-team-review-loop-smoke.ps1`、`catalog-smoke.ps1`、`go test ./...`、`go vet ./...` 与 `/rekit doctor` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 117：start/handoff apply default Go façade

状态：已完成。

目标：承接 Stage 5 的 workstream runtime 收口，在 Go `start`/`handoff` 手动路径与 smoke 已覆盖 preview/apply 的基础上，将 attached case 的 `/rekit start` 与 `/rekit handoff` JSON preview 及显式 `-Apply` 路径升级为默认 Go façade，同时保留日常无 `-Apply` 文本工作线 flow、文本 preview 和 `REKIT_GO_DISABLE=1` PowerShell fallback。

实施范围：

- 扩展 `rekit/rekit.ps1` 默认委托集合，将 `start` 与 `handoff` 纳入 default delegation command set。
- 新增 `start` safe-set：attached case、显式 feature selector、无 review/candidate artifact options、`-WhatIf -Format json` 或显式 `-Apply`（`Format` 为空或 `json`）时默认委托 Go；文本 `-WhatIf` 与无 `-Apply` 文本 flow 继续 PowerShell fallback。
- 新增 `handoff` safe-set：attached case 且已有 `.rekit/board.json`、无 review/candidate/force artifact options、`-WhatIf -Format json` 或显式 `-Apply`（`Format` 为空或 `json`）时默认委托 Go；文本 `-WhatIf` 与无 `-Apply` 文本 flow 继续 PowerShell fallback。
- 保留 `continue -WhatIf -Format json` 仍需显式 `REKIT_GO_ENABLE=1`；`continue` apply/text flow 不在本批迁移。
- 更新 `facade-smoke.ps1`，用 fake `REKIT_GO_EXE` 锁定 `start`/`handoff` 默认 JSON preview 与 apply 委托，并保留文本 fallback 与 `REKIT_GO_DISABLE=1` fallback 检查。
- 更新 `start-apply-smoke.ps1` 与 `handoff-apply-smoke.ps1`，通过公共 façade 验证默认 JSON preview/apply 委托，同时继续验证 Go CLI preview/apply、no-write preview、doctor、resume/checkpoint 与 handoff 内容。
- 更新 Go workstream JSON `nextSteps` 文案，移除“manual Go CLI path”措辞，反映 JSON preview/apply 已由默认 façade 接管。
- 更新 README、CLAUDE、`/rekit` skill、Go runtime migration、Go-first convergence、Agent Team rollout、tests guide、catalog metadata 与 changelog，记录 Batch 117 后 start/handoff 默认委托边界。

边界：本批只切换 `start`/`handoff` JSON preview 与显式 apply 的 façade owner；`start -Apply` 只写 case-local board/facts/policy/lane/workspace scaffold，`handoff -Apply` 只写 case-local handoff/resume/checkpoint；不写 authority/confirmed，不执行 full-trace/debug/inject/patch/dump/network/fuzz/exploit replay，不改变 overview 文本/缺 board 初始化、文本 `note -List`、文本工作线 preview、无 `-Apply` 文本 flow、`continue` preview/apply 或其它 ledger 写入命令。

验证计划：

```powershell
go test ./internal/rekit/cli ./internal/rekit/workstream
.\rekit\tests\facade-smoke.ps1
.\rekit\tests\start-apply-smoke.ps1
.\rekit\tests\handoff-apply-smoke.ps1
.\rekit\tests\catalog-smoke.ps1
go test ./...
go vet ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`go test ./internal/rekit/cli ./internal/rekit/workstream`、`facade-smoke.ps1`、`start-apply-smoke.ps1`、`handoff-apply-smoke.ps1`、`catalog-smoke.ps1`、`go test ./...`、`go vet ./...` 与 `/rekit doctor` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 118：continue JSON preview default Go façade

状态：已完成。

目标：承接 Stage 5 的 continue Go 化，在 Go `continue -WhatIf` 非写入 preview、PowerShell fallback JSON preview 与 preflight smoke 已稳定的基础上，将 attached case 的 `/rekit continue -WhatIf -Format json` 从显式 `REKIT_GO_ENABLE=1` 扩展集合升级为默认 Go façade，继续保持 `continue` apply/text workflow、authority append、run digest 写入和工作线文本入口由 PowerShell fallback 管理。

实施范围：

- 扩展 `rekit/rekit.ps1` 默认委托集合，将 `continue` 纳入 default delegation command set。
- 收紧 `continue` safe-set：仅 attached case 且已有 `.rekit/board.json`、显式 `-WhatIf`、无 `-Apply`/`-CreateCandidates`/`-Review`/`-Force`、无 review artifact options、`-Format json` 时默认委托 Go。
- 保持 `continue -WhatIf` 文本 preview、无 `-WhatIf` 的 JSON 调用、`continue` apply/text flow 与 `REKIT_GO_DISABLE=1` 回退 PowerShell。
- 更新 Go `continue` preview 的 `nextSteps` 文案，反映 JSON preview 已由默认 façade 接管但 continue 仍只支持 `-WhatIf`。
- 更新 `facade-smoke.ps1`，用 fake `REKIT_GO_EXE` 锁定 `continue -WhatIf -Format json` 默认委托，并保留文本 preview fallback。
- 更新 `continue-whatif-smoke.ps1`，通过公共 façade 验证默认 JSON preview 委托、no-write、`REKIT_GO_DISABLE=1` PowerShell JSON fallback 与 apply JSON guard。
- 更新 README、CLAUDE、`/rekit` skill、Go runtime migration、Go-first convergence、Agent Team rollout、tests guide、catalog metadata 与 changelog，记录 Batch 118 后 continue JSON preview 默认委托边界。

边界：本批只切换 `continue -WhatIf -Format json` 的 façade owner；该路径只读取既有 board/lane/outbox/workspace 并输出 preview，不写 facts/run/board/lane/handoff/authority/confirmed，不创建 run directory，不刷新 resume/checkpoint，不执行 full-trace/debug/inject/patch/dump/network/fuzz/exploit replay，不迁移 `continue` apply/text workflow，不改变 authority append policy gate、sync/promote review-first 或其它 ledger 写入命令。

验证计划：

```powershell
go test ./internal/rekit/cli ./internal/rekit/workstream
.\rekit\tests\facade-smoke.ps1
.\rekit\tests\continue-whatif-smoke.ps1
.\rekit\tests\catalog-smoke.ps1
go test ./...
go vet ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`go test ./internal/rekit/cli ./internal/rekit/workstream`、`facade-smoke.ps1`、`continue-whatif-smoke.ps1`、`catalog-smoke.ps1`、`go test ./...`、`go vet ./...` 与 `/rekit doctor` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 119：overview board initialization default Go façade

状态：已完成。

目标：承接 Stage 5 的 workstream runtime 收口，把 attached case 缺 `.rekit/board.json` 时的 overview case-local board/facts/policy/default authority lane 初始化迁到 Go，并让 `/rekit overview` 文本输出与 `overview -Format json` 都默认经 PowerShell façade 委托 Go，消除“首次 overview 必须先由 PowerShell 初始化”的语义分裂。

实施范围：

- 新增 Go workstream helper，复用既有 start scaffold 初始化 `.rekit/lanes`、`.rekit/facts`、`.rekit/runs`、`.rekit/reviews`、`.rekit/backups`、`.rekit/policy.yml`、默认 authority lane 与 `.rekit/board.json`。
- 调整 Go overview data loader：缺 `.rekit/board.json` 时先执行 case-local scaffold 初始化，再读取 board/facts；JSON envelope 仅在本次初始化时标记 `isMutation=true`，后续 overview 仍为只读 `isMutation=false`。
- 放宽 `rekit/rekit.ps1` overview safe-set：attached case、无 write/review flags、无 review artifact options、`Format` 为空或 `json` 时默认委托 Go；不再要求 board 已存在。
- 更新 `cli_test.go`，覆盖 Go overview 缺 board 初始化会创建 board、policy、9 类 facts JSONL 与默认 authority lane。
- 更新 `overview-readonly-smoke.ps1`，覆盖 Go 初始化、façade 默认文本初始化、后续 Go/ façade JSON 只读 snapshot 与 fake/default delegation。
- 更新 `facade-smoke.ps1`，锁定 overview 文本和 JSON 默认 fake delegation。
- 更新 README、CLAUDE、`/rekit` skill、Go runtime migration、Go-first convergence、Agent Team rollout、tests guide、catalog metadata 与 changelog，记录 Batch 119 后 overview 初始化与默认委托边界。

边界：本批只写 attached case 的 `.rekit` workstream scaffold（board/facts/policy/default lane 等 case-local state）；不 append facts event，不写 handoff、authority、confirmed、managed docs、pack source 或 review artifacts，不执行 full-trace/debug/inject/patch/dump/network/fuzz/exploit replay，不迁移 `continue` apply/text workflow，不改变 sync/promote review-first 与 heavy-tool/authority gate。

验证计划：

```powershell
go test ./internal/rekit/cli ./internal/rekit/overview ./internal/rekit/workstream
.\rekit\tests\overview-readonly-smoke.ps1
.\rekit\tests\facade-smoke.ps1
.\rekit\tests\catalog-smoke.ps1
go test ./...
go vet ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`go test ./internal/rekit/cli ./internal/rekit/overview ./internal/rekit/workstream`、`overview-readonly-smoke.ps1`、`facade-smoke.ps1`、`catalog-smoke.ps1`、`go test ./...`、`go vet ./...` 与 `/rekit doctor` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 120：note list text default Go façade

状态：已完成。

目标：承接 Stage 5 的 ledger/read layer 收口，在 Batch 113 已将 `note -List -Format json`、Batch 116 已将 note append / what-if 纳入默认 Go façade 后，把 attached case 的 `/rekit note -List` 文本/table/tsv 只读查询也安全纳入默认 Go façade，减少 PowerShell 继续作为 ledger read layer owner 的漂移风险。

实施范围：

- 扩展 `rekit/rekit.ps1` 的 `note` safe-set：attached case、显式 `-List`、非 `-WhatIf`、无 `-Apply`/`-CreateCandidates`/`-Review`/`-Force`、无 review artifact options 时，允许空 format、`table`、`text`、`tsv` 与 `json` 默认委托 Go。
- 在 `Get-RekitGoArgs` 的 note 参数转发中补齐 slash-command style `RemainingArgs` 内 `-Format` 的透传，避免 safe-set 识别 `-Format json` 但 Go backend 收到默认文本 format。
- 保留 note append 与 `note -WhatIf` 既有默认 Go 路径和 JSON envelope；保留 invalid kind/format、write flag、lane/schema guard。
- 更新 `facade-smoke.ps1`，用 fake `REKIT_GO_EXE` 锁定 text 与 JSON `note -List` 默认委托，并补 `REKIT_GO_DISABLE=1` 下文本 list fallback。
- 更新 `agent-team-review-loop-smoke.ps1`，覆盖 review loop 中 text/JSON `note -List` 默认委托、disable fallback，以及 note what-if/append/overview/handoff 展示闭环。
- 更新 README、CLAUDE、`/rekit` skill、Go runtime migration、Go-first convergence、Agent Team rollout、tests guide、catalog metadata 与 changelog，记录 Batch 120 后 note list 文本/table/tsv 与 JSON 查询的默认委托边界。

边界：本批只切换 attached case 的 `note -List` 文本/table/tsv 只读查询 façade owner；不写 facts/board/lane/handoff/authority/confirmed，不执行 full-trace/debug/inject/patch/dump/network/fuzz/exploit replay，不改变 note append/what-if 写入 schema、不迁移 `continue` apply/text workflow、不改变 sync/promote review-first 与 heavy-tool/authority gate。`REKIT_GO_DISABLE=1` 继续作为 PowerShell fallback。

验证计划：

```powershell
go test ./internal/rekit/cli ./internal/rekit/note
.\rekit\tests\facade-smoke.ps1
.\rekit\tests\agent-team-review-loop-smoke.ps1
.\rekit\tests\catalog-smoke.ps1
go test ./...
go vet ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`go test ./internal/rekit/cli ./internal/rekit/note`、`facade-smoke.ps1`、`agent-team-review-loop-smoke.ps1`、`catalog-smoke.ps1`、`go test ./...`、`go vet ./...` 与 `/rekit doctor` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 121：continue apply default Go façade

状态：已完成。

目标：承接 Stage 5 的 continue Go 化，在 Batch 118 已将 `continue -WhatIf -Format json` 非写入 preview 纳入默认 Go façade 后，将 explicit `continue -Apply [selector]` 的 safe subset 收口到 Go backend 与公共 façade，使 start → note/outbox → continue apply → handoff 的 case-local deterministic 状态流主要由 Go runtime 负责，同时保持 authority/confirmed 与 heavy-tool gate 不自动执行。

实施范围：

- 在 Go workstream runtime 中新增 `ContinueApply`，复用 continue context、lane selector、known event 去重与 preview decision 逻辑，写入 `.rekit/facts/*.jsonl`、request routing、run status/digest、lane resume/checkpoint 与 `.rekit/board.json` refresh。
- 扩展 Go CLI `continue`：要求显式 `-WhatIf` 或 `-Apply`，拒绝二者混用与 review artifact flags；`-WhatIf` 输出非写入 preview，`-Apply` 输出 applied JSON envelope。
- 扩展 PowerShell façade safe-set：attached case、已有 board、explicit `-Apply` 且无 review/candidates/force/what-if 混用时，默认委托 Go；无 `-Apply` 的文本工作线 flow 与 `REKIT_GO_DISABLE=1` 继续回退 PowerShell。
- 对 authority candidate 做保守处理：preview 仍显示 would-append；apply 不 append authority/confirmed，而是写 candidate + decision，并将 decision reason 标记为 `authority append requires explicit user confirmation; Go continue -Apply does not write authority/confirmed`。
- 更新 `continue-whatif-smoke.ps1` 覆盖 Go preview no-write、Go apply case-local writes、authority guard、facade JSON preview/apply 默认委托、duplicate skipped 与 disable fallback。
- 更新 `facade-smoke.ps1` 与 Go CLI tests，锁定 `continue -Apply` 默认委托、apply JSON envelope、facts/routing/digest/resume/board 写入和 unsupported mode guard。
- 更新 README、CLAUDE、`/rekit` skill、Go runtime migration、Go-first convergence、Agent Team rollout、tests guide、catalog metadata 与 changelog，记录 Batch 121 后 continue explicit apply 默认委托边界。

边界：本批只迁移 explicit `continue -Apply` 的 case-local deterministic 写入；不写 authority/confirmed，不执行 full-trace/debug/inject/patch/dump/network/fuzz/exploit replay，不迁移无 `-Apply` 的文本工作线 flow，不改变 sync/promote review-first、不改变 policy schema、不自动执行 heavy-tool gate。authority/confirmed 写入仍必须显式用户确认。

验证计划：

```powershell
go test ./internal/rekit/workstream ./internal/rekit/cli
.\rekit\tests\continue-whatif-smoke.ps1
.\rekit\tests\facade-smoke.ps1
.\rekit\tests\catalog-smoke.ps1
go test ./...
go vet ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`go test ./internal/rekit/workstream ./internal/rekit/cli`、`continue-whatif-smoke.ps1`、`facade-smoke.ps1`、`catalog-smoke.ps1`、`go test ./...`、`go vet ./...` 与 `/rekit doctor` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 122：Go workstream E2E package test

状态：已完成。

目标：承接 Stage 5 完成信号与 Stage 6 Go-driven dry-run harness 方向，在 Batch 121 已将 explicit `continue -Apply` 纳入 Go façade 后，用 `_template` pack 的 Go CLI package test 锁定 start → note → lane outbox → continue apply → handoff 的 case-local deterministic 闭环，减少只靠 PowerShell smoke 验证工作线闭环的漂移风险，并验证非 `vmp-re` pack 的 pack-neutral 路径。

实施范围：

- 在 `internal/rekit/cli` package tests 中新增 `TestRunGoWorkstreamE2EStartNoteContinueHandoff`，使用 `_template` pack attached case fixture。
- 测试顺序覆盖 `start -Apply` 创建 feature lane、`note` append 写 observation、写入 feature lane outbox、`continue -Apply` 收集 observation/request/candidate/publication 并写 facts、route request 到 main lane、写 run digest/resume/board，再执行 lane/project `handoff -Apply`。
- 断言 continue 输出的 summary、writes、accepted non-authority candidate、request routing、decision reason、run digest 被 handoff 引用，以及 project/lane handoff 含接续命令。
- 更新 CHANGELOG、Go-first convergence、Go runtime migration、Agent Team rollout、tests guide、catalog metadata 与 batch-plan，记录 Batch 122 的 test harness 边界和后续缺口。

边界：本批只新增 Go package test 与文档，不新增 runtime 写入面、不改变 façade safe-set、不新增 PowerShell 编排；测试使用临时 package fixture，只写 case-local `.rekit/**`、workspace 与 handoff，不写 authority/confirmed，不执行 heavy-tool，不创建真实 case 或外部副作用。

验证计划：

```powershell
go test ./internal/rekit/cli -run TestRunGoWorkstreamE2EStartNoteContinueHandoff -count=1
go test ./internal/rekit/cli
.\rekit\tests\catalog-smoke.ps1
go test ./...
go vet ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`go test ./internal/rekit/cli -run TestRunGoWorkstreamE2EStartNoteContinueHandoff -count=1`、`go test ./internal/rekit/cli`、`catalog-smoke.ps1`、`go test ./...`、`go vet ./...` 与 `/rekit doctor` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 123：Go gate/dispatch E2E package test

状态：已完成。

目标：承接 Stage 6 Agent Team dry-run harness，在 Batch 122 已锁定 `_template` pack start/note/continue/handoff 最小闭环后，继续扩展同一 Go CLI package fixture 到 bounded dispatch 与 heavy-tool gate 可见性链路，验证 `plan-subagents` review artifact、`gate -Apply` pending-gate ledger、`overview` text/JSON 与 lane `handoff -Apply` 能组成可接手闭环。

实施范围：

- 在 `internal/rekit/cli` package tests 中新增 `TestRunGoGateDispatchE2EPlanGateOverviewHandoff`，使用 `_template` pack attached case fixture。
- 测试顺序覆盖 `start -Apply` 创建 feature lane、`plan-subagents` 生成 request-review route packet/summary、`gate -Apply` 写 feature lane pending-gate request、`overview -Format json` 与文本 overview 展示 pending gate，再执行 lane `handoff -Apply` 验证 pending-gate 区段。
- 断言 plan-subagents 的 route/taskType/shard observability、review-loop contract、blocked actions、gate event 详情、requests ledger、overview sections 和 handoff 文本关键字段。
- 更新 CHANGELOG、Go-first convergence、Go runtime migration、Agent Team rollout、tests guide、catalog metadata 与 batch-plan，记录 Batch 123 的 gate/dispatch test harness 边界和后续缺口。

边界：本批只新增 Go package test与文档，不新增 runtime 写入面、不改变 façade safe-set、不新增 PowerShell 编排；测试只写临时 package fixture 的 case-local `.rekit/**`、review artifact 与 handoff，不自动 spawn subagent、不写 authority/confirmed、不执行 heavy-tool、不创建真实 case 或外部副作用。

验证计划：

```powershell
go test ./internal/rekit/cli -run 'TestRunGo(GateDispatchE2EPlanGateOverviewHandoff|WorkstreamE2EStartNoteContinueHandoff)' -count=1
go test ./internal/rekit/cli
.\rekit\tests\catalog-smoke.ps1
go test ./...
go vet ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`go test ./internal/rekit/cli -run 'TestRunGo(GateDispatchE2EPlanGateOverviewHandoff|WorkstreamE2EStartNoteContinueHandoff)' -count=1`、`go test ./internal/rekit/cli`、`catalog-smoke.ps1`、`go test ./...`、`go vet ./...` 与 `/rekit doctor` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 124：Go reviewer/decision E2E package test

状态：已完成。

目标：承接 Stage 6 Agent Team dry-run harness，在 Batch 122/123 已锁定 `_template` pack 的 workstream 与 gate/dispatch 可见性链路后，继续扩展 Go CLI package fixture 到 reviewer verdict 与 main merge decision 可见性链路，验证 candidate、verification、decision、batch 聚合、note list、overview 与 handoff 能组成可接手闭环。

实施范围：

- 在 `internal/rekit/cli` package tests 中新增 `TestRunGoReviewerDecisionE2ENoteOverviewHandoff`，使用 `_template` pack attached case fixture。
- 测试顺序覆盖 `start -Apply` 创建 feature lane、candidate `note` append、reviewer `verification` append、main `decision` append、`note -List` JSON/text 查询、`overview -Format json` 与文本 overview 展示 candidate/verification/decision/batch，再执行 lane `handoff -Apply` 验证 verification 与 decision 区段。
- 断言 note append JSON envelope、facts 写入路径、verification verdict、decision actor/reason、note list 过滤、overview sections/batch 聚合和 handoff 文本关键字段。
- 更新 CHANGELOG、Go-first convergence、Go runtime migration、Agent Team rollout、tests guide、catalog metadata 与 batch-plan，记录 Batch 124 的 reviewer/decision test harness 边界和后续缺口。

边界：本批只新增 Go package test与文档，不新增 runtime 写入面、不改变 façade safe-set、不新增 PowerShell 编排；测试只写临时 package fixture 的 case-local `.rekit/**` 与 handoff，不自动 spawn reviewer、不写 authority/confirmed、不执行 heavy-tool、不创建真实 case 或外部副作用。

验证计划：

```powershell
go test ./internal/rekit/cli -run TestRunGoReviewerDecisionE2ENoteOverviewHandoff -count=1
go test ./internal/rekit/cli
.\rekit\tests\catalog-smoke.ps1
go test ./...
go vet ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`go test ./internal/rekit/cli -run TestRunGoReviewerDecisionE2ENoteOverviewHandoff -count=1`、`go test ./internal/rekit/cli`、`catalog-smoke.ps1`、`go test ./...`、`go vet ./...` 与 `/rekit doctor` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 125：Go generic-binary-re pack-neutral E2E package test

状态：已完成。

目标：承接 Stage 6 Agent Team dry-run harness 与 Stage 7 pack-neutral hardening，在 Batch 122-124 已锁定 `_template` pack 的 workstream、gate/dispatch 与 reviewer/decision 可见性链路后，扩展 Go CLI package fixture 到非 `_template` / 非 `vmp-re` 的 `generic-binary-re` skeleton pack，验证非 feature lane、`workspace/binary/**`、pack-specific route、gate、overview 与 handoff 不泄漏 `_template`/feature/vmp 语义。

实施范围：

- 在 `internal/rekit/cli` package tests 中新增 `TestRunGoGenericBinaryPackNeutralE2EStartPlanGateOverviewHandoff`，使用 `generic-binary-re` attached case fixture。
- 测试顺序覆盖 `start -Apply` 创建 `binary-analysis-sample` lane、`plan-subagents` 选择 `generic-binary-re:binary-analysis` route、candidate `note` append、`gate -Apply` 写 pending-gate request、reviewer `verification` append、main `decision` append、`overview -Format json` 与文本 overview 展示 candidate/gate/verification/decision/batch，再执行 lane `handoff -Apply` 验证 non-feature handoff。
- 断言 `defaultStartLaneType: binary-analysis` 产生 `binary-analysis-sample`、workspace 为 `workspace/binary/binary-analysis-sample`、review-loop `canonical-write` contract、`tool-review` verification、pending-gate request 与 handoff 文本关键字段。
- 增加 handoff 泄漏断言，确保 `feature-login`、`workspace/features`、`references/template` 不出现在 `generic-binary-re` lane handoff 中。
- 更新 CHANGELOG、Go-first convergence、Go runtime migration、Agent Team rollout、tests guide、catalog metadata 与 batch-plan，记录 Batch 125 的 pack-neutral test harness 边界和后续缺口。

边界：本批只新增 Go package test 与文档，不新增 runtime 写入面、不改变 façade safe-set、不新增 PowerShell 编排；测试只写临时 package fixture 的 case-local `.rekit/**`、`workspace/binary/**`、review artifact 与 handoff，不自动 spawn reviewer/subagent、不写 authority/confirmed、不执行样本、不 debug/trace/dump/patch、不联网、不写真实 case 或外部副作用。

验证计划：

```powershell
go test ./internal/rekit/cli -run TestRunGoGenericBinaryPackNeutralE2EStartPlanGateOverviewHandoff -count=1
go test ./internal/rekit/cli
.\rekit\tests\catalog-smoke.ps1
go test ./...
go vet ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`go test ./internal/rekit/cli -run TestRunGoGenericBinaryPackNeutralE2EStartPlanGateOverviewHandoff -count=1`、`go test ./internal/rekit/cli`、`catalog-smoke.ps1`、`go test ./...`、`go vet ./...` 与 `/rekit doctor` 均通过；`catalog-smoke.ps1` 与 `go test ./...` 曾因 `catalog.json` 新增 metadata 后漏逗号失败，已修复并复跑通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 126：Go web-security pack-neutral E2E package test

状态：已完成。

目标：承接 Stage 6 Agent Team dry-run harness 与 Stage 7 pack-neutral hardening，在 Batch 125 已用 `generic-binary-re` 验证非 feature/binary RE pack-neutral 路径后，扩展 Go CLI package fixture 到非 RE-only 的 `web-security` skeleton pack，验证 Web/API pack-specific route、endpoint/feature lane、network gate、overview 与 handoff 不泄漏 generic-binary、vmp 或 template 语义。

实施范围：

- 在 `internal/rekit/cli` package tests 中新增 `TestRunGoWebSecurityPackNeutralE2EStartPlanGateOverviewHandoff`，使用 `web-security` attached case fixture。
- 测试顺序覆盖 `start -Apply` 创建 `feature-authz` lane、`plan-subagents` 选择 `web-security:feature-analysis` route、candidate `note` append、`gate -Apply -Action network` 写 pending-gate request、reviewer `verification` append、main `decision` append、`overview -Format json` 与文本 overview 展示 candidate/gate/verification/decision/batch，再执行 lane `handoff -Apply` 验证 Web/API lane handoff。
- 断言 `defaultStartLaneType: feature` 产生 `feature-authz`、workspace 为 `workspace/features/feature-authz`、review-loop `canonical-write` contract、`manual-review` verification、network pending-gate request 与 handoff 文本关键字段。
- 增加 handoff 泄漏断言，确保 `generic-binary-re`、`workspace/binary`、`binary-analysis-sample`、`references/template`、`vmp-re` 不出现在 `web-security` lane handoff 中。
- 更新 CHANGELOG、Go-first convergence、Go runtime migration、Agent Team rollout、tests guide、catalog metadata 与 batch-plan，记录 Batch 126 的非 RE-only pack-neutral test harness 边界和后续缺口。

边界：本批只新增 Go package test 与文档，不新增 runtime 写入面、不改变 façade safe-set、不新增 PowerShell 编排；测试只写临时 package fixture 的 case-local `.rekit/**`、`workspace/features/**`、review artifact 与 handoff，不自动 spawn reviewer/subagent、不写 authority/confirmed、不执行请求回放/扫描/fuzz/exploit replay/network 操作、不联网、不写真实 case 或外部副作用。

验证计划：

```powershell
go test ./internal/rekit/cli -run TestRunGoWebSecurityPackNeutralE2EStartPlanGateOverviewHandoff -count=1
go test ./internal/rekit/cli
.\rekit\tests\catalog-smoke.ps1
go test ./...
go vet ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`go test ./internal/rekit/cli -run TestRunGoWebSecurityPackNeutralE2EStartPlanGateOverviewHandoff -count=1`、`go test ./internal/rekit/cli`、`catalog-smoke.ps1`、`go test ./...`、`go vet ./...` 与 `/rekit doctor` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 127：web-security Agent Team real dry-run smoke

状态：已完成。

目标：承接 Batch 126 的 `web-security` Go package E2E，把非 RE-only pack-neutral 覆盖推进到真实临时 case smoke，使用公共 `/rekit` façade 跑通 Web/API Agent Team dry-run，验证新会话可从 overview、ledger 与 lane handoff 接手，并继续保持 no spawn、no authority/confirmed、no network/heavy-tool 边界。

实施范围：

- 新增 `rekit/tests/web-security-agent-team-dryrun-smoke.ps1`，默认使用 `C:\AI\m_projects\RE\_dryrun_cases` 下的临时 case，结束时清理 case 与 review artifact。
- 脚本通过公共 `/rekit` façade 覆盖 `init -Apply`、`start -Apply -Format json` 创建 `feature-authz` lane、写入非敏感 workspace packet、`plan-subagents` 选择 `web-security:feature-analysis` route 并生成 review packet、candidate `note` append、`gate -WhatIf -Action network` no-write 预览、`gate -Apply -Action network` pending-gate request、reviewer `verification` append、main `decision` append、`note -List` request/verification、`overview` text/JSON、lane `handoff -Apply -Format json` 与 case `doctor`。
- 断言 review packet 保持 `manual-main-agent`、`runtime does not spawn subagents` 与 main-agent canonical-write contract；断言 gate what-if 不改 requests ledger；断言 overview/handoff 展示 candidate、network pending-gate、verification、decision 与 batch；断言 handoff 不泄漏 `generic-binary-re`、`workspace/binary`、`binary-analysis-sample`、`references/template` 或 `vmp-re`。
- 更新 CHANGELOG、Go-first convergence、Go runtime migration、Agent Team rollout、tests guide、catalog metadata 与 batch-plan，记录 Batch 127 从 package E2E 走向真实临时 case dry-run 的边界和后续缺口。

边界：本批只新增 smoke 与文档，不新增 runtime 写入面、不改变 façade safe-set、不自动 spawn reviewer/subagent；smoke 只写并清理临时 case-local `.rekit/**`、`workspace/features/**` 与 review artifact，不写 authority/confirmed，不执行真实网络请求、请求回放、扫描、fuzz、exploit replay、debug、dump、patch、hook 或外部副作用，不写真实 case 或 kit 模板 case state。

验证计划：

```powershell
.\rekit\tests\web-security-agent-team-dryrun-smoke.ps1
.\rekit\tests\catalog-smoke.ps1
go test ./internal/rekit/cli -run TestRunGoWebSecurityPackNeutralE2EStartPlanGateOverviewHandoff -count=1
go test ./internal/rekit/cli
go test ./...
go vet ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`web-security-agent-team-dryrun-smoke.ps1`、`catalog-smoke.ps1`、`go test ./internal/rekit/cli -run TestRunGoWebSecurityPackNeutralE2EStartPlanGateOverviewHandoff -count=1`、`go test ./internal/rekit/cli`、`go test ./...`、`go vet ./...` 与 `/rekit doctor` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 128：release readiness checklist and invariant

状态：已完成。

目标：承接 Stage 8 CI / release readiness / deprecation plan，在 Batch 127 已补 `web-security` 真实临时 case dry-run 后，新增一页 release readiness checklist，明确本机 release gate、recommended minimum、发布前人工检查、pack maturity matrix、Go-owned/PowerShell legacy 状态与 known gaps，降低新会话接手和 release 前门禁漂移风险。

实施范围：

- 新增 `docs/release-readiness.md`，顶部包含读取指南、实施摘要、执行清单、验证标准、风险与注意事项，并补当前 pack maturity matrix、Go-owned / PowerShell legacy 状态与 known gaps。
- 在 README、仓库 `CLAUDE.md` 与 `docs/go-first-convergence-plan.md` 中接入 `docs/release-readiness.md` 入口；Stage 8 推荐批次与完成信号记录 Batch 128 已新增 checklist。
- 扩展 `internal/rekit/manifest/release_invariants_test.go`，新增 `TestReleaseReadinessChecklistInvariants`，锁定 release checklist 必备章节、catalog recommended minimum、本机 release gate 命令、全部 pack matrix、关键安全边界、known gaps，以及 README/CLAUDE/go-first 入口链接。
- 更新 CHANGELOG 与 batch-plan，记录 Batch 128 的 release readiness 收口边界。

边界：本批只新增 release readiness 文档与 Go invariant test，不新增 CI workflow、不改变 runtime 写入面、不改变 façade safe-set、不新增 PowerShell 编排、不运行大型 matrix；release gate 仍以 Go tests、go vet、doctor、少量 Windows façade smoke 和按改动类型选择的临时 case smoke 为主。

验证计划：

```powershell
go test ./internal/rekit/manifest -run TestReleaseReadinessChecklistInvariants -count=1
go test ./internal/rekit/manifest
.\rekit\tests\facade-smoke.ps1
go test ./...
go vet ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`go test ./internal/rekit/manifest -run TestReleaseReadinessChecklistInvariants -count=1`、`go test ./internal/rekit/manifest`、`facade-smoke.ps1`、`go test ./...`、`go vet ./...` 与 `/rekit doctor` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 129：PowerShell runtime deprecation strategy

状态：已完成。

目标：承接 Stage 8 中“明确 PowerShell runtime deprecation strategy”的未完成项，在 Batch 128 release readiness checklist 之后，新增 PowerShell runtime 冻结/保留/迁移/删除策略，明确哪些命令 Go-owned、哪些路径 legacy-only、哪些动作 blocked，以及 `rekit/lib/*.ps1` 模块状态和 removal gates，防止后续继续扩张 PowerShell runtime 语义。

实施范围：

- 新增 `docs/powershell-deprecation.md`，包含读取指南、实施摘要、执行清单、验证标准、风险与注意事项、命令归属矩阵、PowerShell 模块状态、freeze/deprecation gates 与禁止迁移清单。
- 在 README、仓库 `CLAUDE.md`、`docs/release-readiness.md` 与 `docs/go-first-convergence-plan.md` 中接入 deprecation strategy 入口；Stage 8 推荐批次和完成信号记录 Batch 129 已新增策略文档，但实际删除/冻结仍需单独批次验证。
- 扩展 `internal/rekit/manifest/release_invariants_test.go`，新增 `TestPowerShellDeprecationStrategyInvariants`，锁定 deprecation 文档必备章节、命令归属、全部 `rekit/lib/*.ps1` 模块、禁止迁移清单，以及 README/CLAUDE/go-first/release-readiness 入口链接。
- 更新 CHANGELOG 与 batch-plan，记录 Batch 129 的 deprecation strategy 收口边界。

边界：本批只新增 deprecation strategy 文档与 Go invariant test，不删除 PowerShell 代码、不改变 runtime 写入面、不改变 façade safe-set、不新增 PowerShell 编排、不新增 CI workflow；actual heavy-tool、authority/confirmed、policy schema migration、外部副作用和 case-local shim 逻辑复制仍列为禁止迁移项。

验证计划：

```powershell
go test ./internal/rekit/manifest -run TestPowerShellDeprecationStrategyInvariants -count=1
go test ./internal/rekit/manifest
go test ./...
go vet ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`go test ./internal/rekit/manifest -run TestPowerShellDeprecationStrategyInvariants -count=1`、`go test ./internal/rekit/manifest`、`go test ./...`、`go vet ./...` 与 `/rekit doctor` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 130：Go release-check inventory

状态：已完成。

目标：承接 Stage 8 中“release gate 可在本机和 CI 中稳定执行”的未完成项，在 Batch 128 release readiness checklist 与 Batch 129 PowerShell deprecation strategy 之后，新增 Go-owned、只读、机器可读的 release gate inventory，让本机/CI 在执行耗时验证前先用确定性 JSON envelope 检查 recommended minimum、文档入口、pack schema、安全边界与 known gaps 是否齐备。

实施范围：

- 新增 `internal/rekit/releasecheck` package，读取 `rekit/tests/catalog.json`、`docs/release-readiness.md` 与 pack manifest inventory，输出 `release-check` JSON/text result；字段包含 `recommendedMinimum`、`requiredCommands`、`documents`、`packs`、`boundaries`、`knownGaps` 与 `warnings`。
- 在 Go CLI 新增 `-Command release-check`，默认只读，拒绝 `-Target`、`-Apply`、`-WhatIf`、review artifact 和 list/mutation flags；支持 text/table 与 `-Format json`。
- 在 `rekit/rekit.ps1` 中把 `release-check` 纳入默认 Go façade 委托与 ValidateSet；该命令无 PowerShell fallback，仅透传到 Go backend，避免新增 PowerShell release orchestrator。
- 更新 `rekit/tests/catalog.json` recommendedMinimum、`docs/release-readiness.md`、`rekit/tests/README.md`、`docs/powershell-deprecation.md`、README、CLAUDE、go-first convergence plan 与 CHANGELOG，记录 `release-check` 的 release gate 定位和只读边界。
- 扩展 `internal/rekit/cli/cli_test.go`，覆盖 release-check JSON/text envelope、required command/catalog 对齐、必备文档、pack schema rows、known gaps，以及 target/mutation/format guard；扩展 release invariant 使 catalog 与 checklist 都包含 release-check 命令。

边界：本批不新增 CI workflow、不执行 `go test`/`go vet`/smoke 编排、不写 case/pack/runtime state、不改变 sync/promote/gate/continue 写入面、不自动运行 heavy-tool、不写 authority/confirmed、不删除 PowerShell runtime；`release-check` 只是确定性 inventory 和 release gate 前置检查。

验证计划：

```powershell
go test ./internal/rekit/cli -run TestRunReleaseCheck -count=1
go test ./internal/rekit/cli
go test ./internal/rekit/manifest -run TestReleaseCatalogInvariants -count=1
go run ./cmd/rekit -- -Command release-check -Format json
.\rekit\rekit.ps1 -Command release-check -Format json
.\rekit\tests\catalog-smoke.ps1
.\rekit\tests\facade-smoke.ps1
go test ./...
go vet ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`go test ./internal/rekit/cli -run TestRunReleaseCheck -count=1`、`go test ./internal/rekit/cli`、`go test ./internal/rekit/manifest -run TestReleaseCatalogInvariants -count=1`、`go test ./internal/rekit/manifest`、`go run ./cmd/rekit -- -Command release-check -Format json`、`/rekit release-check -Format json`、`catalog-smoke.ps1`、`facade-smoke.ps1`、`go test ./...`、`go vet ./...` 与 `/rekit doctor` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 131：PowerShell façade freeze invariant

状态：已完成。

目标：承接 Stage 8 中 PowerShell deprecation 的“freeze/removal gates”未完成项，在 Batch 129 deprecation strategy 和 Batch 130 `release-check` inventory 之后，用 Go release invariant 静态锁定 `rekit/rekit.ps1` 默认 Go façade 委托集合、Go-only guard、legacy/internal command 边界与 blocked heavy-tool/authority/confirmed 边界，避免后续无意扩大 PowerShell runtime owner 或默认委托面。

实施范围：

- 扩展 `internal/rekit/manifest/release_invariants_test.go`，新增 `TestPowerShellFacadeFreezeInvariants`，解析 `rekit/rekit.ps1` 中 `Test-RekitGoDefaultDelegationCommand` 的单引号数组和 top-level `ValidateSet`。
- 锁定默认 Go façade 委托集合必须等于当前 Go-owned safe set：`release-check/status/packs/doctor/validate/case lifecycle/sync/promote/overview/note/gate/start/handoff/continue`，并确认 `plan-subagents` 仍只是 ValidateSet 支持的 legacy/internal command，不进入默认 Go 委托集合。
- 锁定 `release-check` 的 no-target safe guard、Go-only fallback message、`gate` Go-only fallback message，以及 `facade-smoke.ps1` 对 `release-check` fake delegation、`REKIT_GO_DISABLE` 与 must-not-run guard 的覆盖信号。
- 更新 `docs/powershell-deprecation.md`、`docs/release-readiness.md`、`docs/go-first-convergence-plan.md`、CHANGELOG 与 batch-plan，记录 Batch 131 将 freeze guard 固化为 Go invariant。

边界：本批只新增/扩展 release invariant 与文档，不改 `rekit/rekit.ps1` 行为、不删除 PowerShell 代码、不改变 façade safe-set、不写 case/pack/runtime state、不新增 CI workflow、不执行 heavy-tool、不写 authority/confirmed、不迁移 policy schema。

验证计划：

```powershell
go test ./internal/rekit/manifest -run TestPowerShellFacadeFreezeInvariants -count=1
go test ./internal/rekit/manifest
go test ./...
go vet ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`go test ./internal/rekit/manifest -run TestPowerShellFacadeFreezeInvariants -count=1`、`go test ./internal/rekit/manifest`、`go test ./...`、`go vet ./...` 与 `/rekit doctor` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 132：generic-binary-re Agent Team real dry-run smoke

状态：已完成。

目标：承接 Stage 6/7 中“更多非 `_template` / 非 `vmp-re` pack 从 package E2E 走向真实临时 case dry-run”的未完成项，把 Batch 125 的 `generic-binary-re` pack-neutral Go package E2E 提升为公共 `/rekit` façade 下可重复执行的真实临时 case dry-run，验证 binary-analysis lane、debug pending-gate、candidate/verification/decision ledger、overview、handoff 和 doctor 的端到端闭环。

实施范围：

- 新增 `rekit/tests/generic-binary-agent-team-dryrun-smoke.ps1`，使用临时 case 运行 `init -Apply`、`start -Apply`、`plan-subagents`、candidate `note`、`gate -WhatIf/-Apply -Action debug`、verification `note`、decision `note`、`note -List`、`overview` text/JSON、lane `handoff -Apply` 与 `doctor`。
- 验证 `gate -WhatIf` 不写 `.rekit/facts/requests.jsonl`，`gate -Apply` 只写 pending-gate request，不执行 debug/heavy-tool、不写 authority/confirmed。
- 验证 `plan-subagents` 选择 `generic-binary-re:binary-analysis` route、`manual-main-agent` dispatch、`runtime does not spawn subagents` blocked action 与 `canonical-write` 主 agent 写入边界。
- 验证 handoff 包含 `workspace/binary/binary-analysis-sample/packet.md`、verification/decision/pending-gate 区段，并不泄漏 `web-security`、`workspace/features`、`feature-authz`、`endpoint-login`、`action=network`、`references/template` 或 `vmp-re` 语义。
- 更新 `rekit/tests/catalog.json`、`rekit/tests/README.md`、`docs/go-first-convergence-plan.md`、`docs/release-readiness.md`、CHANGELOG 与 batch-plan，记录 `generic-binary-re` 真实 dry-run 覆盖。

边界：本批只使用临时 case 和 review artifact；不执行样本、不 attach/debug/trace/dump/patch、不运行 heavy-tool、不联网、不自动 spawn subagent、不写 authority/confirmed、不改变 runtime 写入面、不新增 CI workflow。

验证计划：

```powershell
.\rekit\tests\generic-binary-agent-team-dryrun-smoke.ps1
.\rekit\tests\catalog-smoke.ps1
go test ./internal/rekit/cli -run TestRunGoGenericBinaryPackNeutralE2EStartPlanGateOverviewHandoff -count=1
go test ./internal/rekit/cli
go test ./...
go vet ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`generic-binary-agent-team-dryrun-smoke.ps1`、`catalog-smoke.ps1`、`go test ./internal/rekit/cli -run TestRunGoGenericBinaryPackNeutralE2EStartPlanGateOverviewHandoff -count=1`、`go test ./internal/rekit/cli`、`go test ./...`、`go vet ./...` 与 `/rekit doctor` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 133：malware-analysis Agent Team real dry-run smoke

状态：已完成。

目标：继续承接 Stage 6/7 中“更多非 `_template` / 非 `vmp-re` pack 从 package E2E 走向真实临时 case dry-run”的未完成项，在 Batch 127、132 已覆盖 `web-security` 与 `generic-binary-re` 后，新增 `malware-analysis` 公共 `/rekit` façade 下的真实临时 case dry-run，验证 sample-analysis lane、network/sandbox pending-gate、candidate/verification/decision ledger、overview、handoff 和 doctor 的端到端闭环。

实施范围：

- 新增 `rekit/tests/malware-analysis-agent-team-dryrun-smoke.ps1`，使用临时 case 运行 `init -Apply`、`start -Apply`、`plan-subagents`、candidate `note`、`gate -WhatIf/-Apply -Action network`、verification `note`、decision `note`、`note -List`、`overview` text/JSON、lane `handoff -Apply` 与 `doctor`。
- 验证 `gate -WhatIf` 不写 `.rekit/facts/requests.jsonl`，`gate -Apply` 只写 pending-gate request，不执行 sandbox/network detonation/heavy-tool、不写 authority/confirmed。
- 验证 `plan-subagents` 选择 `malware-analysis:sample-analysis` route、`manual-main-agent` dispatch、`runtime does not spawn subagents` blocked action 与 `canonical-write` 主 agent 写入边界。
- 验证 handoff 包含 `workspace/samples/sample-analysis-triage/packet.md`、verification/decision/pending-gate 区段，并不泄漏 `web-security`、`workspace/features`、`endpoint-login`、`generic-binary-re`、`workspace/binary`、`binary-analysis-sample`、`references/template` 或 `vmp-re` 语义。
- 更新 `rekit/tests/catalog.json`、`rekit/tests/README.md`、`docs/go-first-convergence-plan.md`、`docs/release-readiness.md`、CHANGELOG 与 batch-plan，记录 `malware-analysis` 真实 dry-run 覆盖。

边界：本批只使用临时 case 和 review artifact；不执行样本、不 sandbox detonation、不联网、不 debug/dump/patch、不运行 heavy-tool、不自动 spawn subagent、不写 authority/confirmed、不改变 runtime 写入面、不新增 CI workflow。

验证计划：

```powershell
.\rekit\tests\malware-analysis-agent-team-dryrun-smoke.ps1
.\rekit\tests\catalog-smoke.ps1
.\rekit\tests\malware-analysis-pack-smoke.ps1
go test ./internal/rekit/cli
go test ./...
go vet ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`malware-analysis-agent-team-dryrun-smoke.ps1`、`catalog-smoke.ps1`、`malware-analysis-pack-smoke.ps1`、`go test ./internal/rekit/cli`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json` 与 `/rekit doctor` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。初次验证发现 `rekit/tests/catalog.json` 的新增条目后缺少分隔逗号，已修复并重跑 catalog smoke 与 Go CLI tests。

### Batch 134：vuln-research Agent Team real dry-run smoke

状态：已完成。

目标：继续承接 Stage 6/7 中“更多非 `_template` / 非 `vmp-re` pack 从 package E2E 走向真实临时 case dry-run”的未完成项，在 Batch 127、132、133 已覆盖 `web-security`、`generic-binary-re` 与 `malware-analysis` 后，新增 `vuln-research` 公共 `/rekit` façade 下的真实临时 case dry-run，验证 vuln-analysis lane、debug/repro pending-gate、candidate/verification/decision ledger、overview、handoff 和 doctor 的端到端闭环。

实施范围：

- 新增 `rekit/tests/vuln-research-agent-team-dryrun-smoke.ps1`，使用临时 case 运行 `init -Apply`、`start -Apply`、`plan-subagents`、candidate `note`、`gate -WhatIf/-Apply -Action debug`、verification `note`、decision `note`、`note -List`、`overview` text/JSON、lane `handoff -Apply` 与 `doctor`。
- 验证 `gate -WhatIf` 不写 `.rekit/facts/requests.jsonl`，`gate -Apply` 只写 pending-gate request，不执行 debugger/repro/heavy-tool、不写 authority/confirmed。
- 验证 `plan-subagents` 选择 `vuln-research:vuln-analysis` route、`manual-main-agent` dispatch、`runtime does not spawn subagents` blocked action 与 `canonical-write` 主 agent 写入边界。
- 验证 handoff 包含 `workspace/vulns/vuln-analysis-crash/packet.md`、verification/decision/pending-gate 区段，并不泄漏 `web-security`、`workspace/features`、`endpoint-login`、`generic-binary-re`、`workspace/binary`、`binary-analysis-sample`、`malware-analysis`、`workspace/samples`、`sample-alpha`、`references/template` 或 `vmp-re` 语义。
- 更新 `rekit/tests/catalog.json`、`rekit/tests/README.md`、`docs/go-first-convergence-plan.md`、`docs/release-readiness.md`、CHANGELOG 与 batch-plan，记录 `vuln-research` 真实 dry-run 覆盖。

边界：本批只使用临时 case 和 review artifact；不连接 live target、不主动扫描、不 fuzz、不 replay exploit、不执行 debugger/repro/dump/patch/heavy-tool、不自动 spawn subagent、不写 authority/confirmed、不改变 runtime 写入面、不新增 CI workflow。

验证计划：

```powershell
.\rekit\tests\vuln-research-agent-team-dryrun-smoke.ps1
.\rekit\tests\catalog-smoke.ps1
.\rekit\tests\vuln-research-pack-smoke.ps1
go test ./internal/rekit/cli
go test ./...
go vet ./...
go run ./cmd/rekit -- -Command release-check -Format json
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`vuln-research-agent-team-dryrun-smoke.ps1`、`catalog-smoke.ps1`、`vuln-research-pack-smoke.ps1`、`go test ./internal/rekit/cli`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json` 与 `/rekit doctor` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 135：ctf Agent Team real dry-run smoke

状态：已完成。

目标：继续承接 Stage 6/7 中“更多非 `_template` / 非 `vmp-re` pack 从 package E2E 走向真实临时 case dry-run”的未完成项，在 Batch 127、132、133、134 已覆盖 `web-security`、`generic-binary-re`、`malware-analysis` 与 `vuln-research` 后，新增 `ctf` 公共 `/rekit` façade 下的真实临时 case dry-run，验证 challenge-analysis lane、remote/network pending-gate、candidate/verification/decision ledger、overview、handoff 和 doctor 的端到端闭环。

实施范围：

- 新增 `rekit/tests/ctf-agent-team-dryrun-smoke.ps1`，使用临时 case 运行 `init -Apply`、`start -Apply`、`plan-subagents`、candidate `note`、`gate -WhatIf/-Apply -Action network`、verification `note`、decision `note`、`note -List`、`overview` text/JSON、lane `handoff -Apply` 与 `doctor`。
- 验证 `gate -WhatIf` 不写 `.rekit/facts/requests.jsonl`，`gate -Apply` 只写 pending-gate request，不远程连接、不 brute force、不 fuzz、不 replay exploit、不执行 network/debug/dump/patch/heavy-tool、不写 flag/authority/confirmed。
- 验证 `plan-subagents` 选择 `ctf:challenge-analysis` route、`manual-main-agent` dispatch、`runtime does not spawn subagents` blocked action 与 `canonical-write` 主 agent 写入边界。
- 验证 handoff 包含 `workspace/challenges/challenge-analysis-pwn/packet.md`、verification/decision/pending-gate 区段，并不泄漏 `web-security`、`workspace/features`、`endpoint-login`、`generic-binary-re`、`workspace/binary`、`binary-analysis-sample`、`malware-analysis`、`workspace/samples`、`sample-alpha`、`vuln-research`、`workspace/vulns`、`references/template` 或 `vmp-re` 语义。
- 更新 `rekit/tests/catalog.json`、`rekit/tests/README.md`、`docs/go-first-convergence-plan.md`、`docs/release-readiness.md`、CHANGELOG 与 batch-plan，记录 `ctf` 真实 dry-run 覆盖。

边界：本批只使用临时 case 和 review artifact；不远程连接、不 brute force、不 fuzz、不 replay exploit、不执行 network/debug/dump/patch/heavy-tool、不写 flag/authority/confirmed、不自动 spawn subagent、不改变 runtime 写入面、不新增 CI workflow。

验证计划：

```powershell
.\rekit\tests\ctf-agent-team-dryrun-smoke.ps1
.\rekit\tests\catalog-smoke.ps1
.\rekit\tests\ctf-pack-smoke.ps1
go test ./internal/rekit/cli
go test ./...
go vet ./...
go run ./cmd/rekit -- -Command release-check -Format json
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`ctf-agent-team-dryrun-smoke.ps1`、`catalog-smoke.ps1`、`ctf-pack-smoke.ps1`、`go test ./internal/rekit/cli`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json` 与 `/rekit doctor` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 136：unpack-pe Agent Team real dry-run smoke

状态：已完成。

目标：继续承接 Stage 6/7 中“更多非 `_template` / 非 `vmp-re` pack 从 package E2E 走向真实临时 case dry-run”的未完成项，在 Batch 127、132、133、134、135 已覆盖 `web-security`、`generic-binary-re`、`malware-analysis`、`vuln-research` 与 `ctf` 后，新增 `unpack-pe` 公共 `/rekit` façade 下的真实临时 case dry-run，验证 unpack-analysis lane、dump/unpack pending-gate、candidate/verification/decision ledger、overview、handoff 和 doctor 的端到端闭环。

实施范围：

- 新增 `rekit/tests/unpack-pe-agent-team-dryrun-smoke.ps1`，使用临时 case 运行 `init -Apply`、`start -Apply`、`plan-subagents`、candidate `note`、`gate -WhatIf/-Apply -Action dump`、verification `note`、decision `note`、`note -List`、`overview` text/JSON、lane `handoff -Apply` 与 `doctor`。
- 验证 `gate -WhatIf` 不写 `.rekit/facts/requests.jsonl`，`gate -Apply` 只写 pending-gate request，不执行样本/debug/dump/patch/heavy-tool、不写 unpacked binary、不写 authority/confirmed。
- 验证 `plan-subagents` 选择 `unpack-pe:unpack-analysis` route、`manual-main-agent` dispatch、`runtime does not spawn subagents` blocked action 与 `canonical-write` 主 agent 写入边界。
- 验证 handoff 包含 `workspace/unpack/unpack-analysis-loader/packet.md`、verification/decision/pending-gate 区段，并不泄漏 `web-security`、`workspace/features`、`endpoint-login`、`generic-binary-re`、`workspace/binary`、`binary-analysis-sample`、`malware-analysis`、`workspace/samples`、`sample-alpha`、`vuln-research`、`workspace/vulns`、`ctf`、`workspace/challenges`、`references/template` 或 `vmp-re` 语义。
- 更新 `rekit/tests/catalog.json`、`rekit/tests/README.md`、`docs/go-first-convergence-plan.md`、`docs/release-readiness.md`、CHANGELOG 与 batch-plan，记录 `unpack-pe` 真实 dry-run 覆盖。

边界：本批只使用临时 case 和 review artifact；不执行样本、不 debug/dump/patch、不写 unpacked binary、不运行 heavy-tool、不自动 spawn subagent、不写 authority/confirmed、不改变 runtime 写入面、不新增 CI workflow。

验证计划：

```powershell
.\rekit\tests\unpack-pe-agent-team-dryrun-smoke.ps1
.\rekit\tests\catalog-smoke.ps1
.\rekit\tests\unpack-pe-pack-smoke.ps1
go test ./internal/rekit/cli
go test ./...
go vet ./...
go run ./cmd/rekit -- -Command release-check -Format json
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`unpack-pe-agent-team-dryrun-smoke.ps1`、`catalog-smoke.ps1`、`unpack-pe-pack-smoke.ps1`、`go test ./internal/rekit/cli`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json` 与 `/rekit doctor` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 137：ollvm Agent Team real dry-run smoke

状态：已完成。

目标：继续承接 Stage 6/7 中“更多非 `_template` / 非 `vmp-re` pack 从 package E2E 走向真实临时 case dry-run”的未完成项，在 Batch 127、132、133、134、135、136 已覆盖 `web-security`、`generic-binary-re`、`malware-analysis`、`vuln-research`、`ctf` 与 `unpack-pe` 后，新增 `ollvm` 公共 `/rekit` façade 下的真实临时 case dry-run，验证 obfuscation-analysis lane、full-trace/CFG pending-gate、candidate/verification/decision ledger、overview、handoff 和 doctor 的端到端闭环。

实施范围：

- 新增 `rekit/tests/ollvm-agent-team-dryrun-smoke.ps1`，使用临时 case 运行 `init -Apply`、`start -Apply`、`plan-subagents`、candidate `note`、`gate -WhatIf/-Apply -Action full-trace`、verification `note`、decision `note`、`note -List`、`overview` text/JSON、lane `handoff -Apply` 与 `doctor`。
- 验证 `gate -WhatIf` 不写 `.rekit/facts/requests.jsonl`，`gate -Apply` 只写 pending-gate request，不执行样本/full-trace/dump/patch/heavy-tool、不写 deobfuscated binary、不写 authority/confirmed。
- 验证 `plan-subagents` 选择 `ollvm:obfuscation-analysis` route、`manual-main-agent` dispatch、`runtime does not spawn subagents` blocked action 与 `canonical-write` 主 agent 写入边界。
- 验证 handoff 包含 `workspace/obfuscation/obfuscation-analysis-cfg/packet.md`、verification/decision/pending-gate 区段，并不泄漏 `web-security`、`workspace/features`、`endpoint-login`、`generic-binary-re`、`workspace/binary`、`binary-analysis-sample`、`malware-analysis`、`workspace/samples`、`sample-alpha`、`vuln-research`、`workspace/vulns`、`ctf`、`workspace/challenges`、`unpack-pe`、`workspace/unpack`、`references/template` 或 `vmp-re` 语义。
- 更新 `rekit/tests/catalog.json`、`rekit/tests/README.md`、`docs/go-first-convergence-plan.md`、`docs/release-readiness.md`、CHANGELOG 与 batch-plan，记录 `ollvm` 真实 dry-run 覆盖。

边界：本批只使用临时 case 和 review artifact；不执行样本、不 full-trace/dump/patch、不写 deobfuscated binary、不运行 heavy-tool、不自动 spawn subagent、不写 authority/confirmed、不改变 runtime 写入面、不新增 CI workflow。

验证计划：

```powershell
.\rekit\tests\ollvm-agent-team-dryrun-smoke.ps1
.\rekit\tests\catalog-smoke.ps1
.\rekit\tests\ollvm-pack-smoke.ps1
go test ./internal/rekit/cli
go test ./...
go vet ./...
go run ./cmd/rekit -- -Command release-check -Format json
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`ollvm-agent-team-dryrun-smoke.ps1`、`catalog-smoke.ps1`、`ollvm-pack-smoke.ps1`、`go test ./internal/rekit/cli`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json` 与 `/rekit doctor` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 138：android-native Agent Team real dry-run smoke

状态：已完成。

目标：收束 Stage 6/7 中安全领域 skeleton pack 真实临时 case dry-run 覆盖的最后缺口，在 Batch 127、132-137 已覆盖 `web-security`、`generic-binary-re`、`malware-analysis`、`vuln-research`、`ctf`、`unpack-pe` 与 `ollvm` 后，新增 `android-native` 公共 `/rekit` façade 下的真实临时 case dry-run，验证 native-analysis lane、inject/hook pending-gate、candidate/verification/decision ledger、overview、handoff 和 doctor 的端到端闭环。

实施范围：

- 新增 `rekit/tests/android-native-agent-team-dryrun-smoke.ps1`，使用临时 case 运行 `init -Apply`、`start -Apply`、`plan-subagents`、candidate `note`、`gate -WhatIf/-Apply -Action inject`、verification `note`、decision `note`、`note -List`、`overview` text/JSON、lane `handoff -Apply` 与 `doctor`。
- 验证 `gate -WhatIf` 不写 `.rekit/facts/requests.jsonl`，`gate -Apply` 只写 pending-gate request，不连接设备、不 attach Frida、不执行 hook/inject/dump/patch/heavy-tool、不写 authority/confirmed。
- 验证 `plan-subagents` 选择 `android-native:native-analysis` route、`manual-main-agent` dispatch、`runtime does not spawn subagents` blocked action 与 `canonical-write` 主 agent 写入边界。
- 验证 handoff 包含 `workspace/native/native-analysis-jni/packet.md`、verification/decision/pending-gate 区段，并不泄漏 `web-security`、`workspace/features`、`endpoint-login`、`generic-binary-re`、`workspace/binary`、`binary-analysis-sample`、`malware-analysis`、`workspace/samples`、`sample-alpha`、`vuln-research`、`workspace/vulns`、`ctf`、`workspace/challenges`、`unpack-pe`、`workspace/unpack`、`ollvm`、`workspace/obfuscation`、`references/template` 或 `vmp-re` 语义。
- 更新 `rekit/tests/catalog.json`、`rekit/tests/README.md`、`docs/go-first-convergence-plan.md`、`docs/release-readiness.md`、CHANGELOG 与 batch-plan，记录 `android-native` 真实 dry-run 覆盖，并把后续 Stage 6/7 缺口转向 release readiness/CI 或新 pack 引入。

边界：本批只使用临时 case 和 review artifact；不连接设备、不 attach Frida、不执行 hook/inject/dump/patch/heavy-tool、不自动 spawn subagent、不写 authority/confirmed、不改变 runtime 写入面、不新增 CI workflow。

验证计划：

```powershell
.\rekit\tests\android-native-agent-team-dryrun-smoke.ps1
.\rekit\tests\catalog-smoke.ps1
.\rekit\tests\android-native-pack-smoke.ps1
go test ./internal/rekit/cli
go test ./...
go vet ./...
go run ./cmd/rekit -- -Command release-check -Format json
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`android-native-agent-team-dryrun-smoke.ps1`、`catalog-smoke.ps1`、`android-native-pack-smoke.ps1`、`go test ./internal/rekit/cli`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json` 与 `/rekit doctor` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 139：Go release-check gate profile

状态：已完成。

目标：承接 Stage 8 中 release gate 可在本机和 CI 稳定执行的未完成项，在 Batch 130 Go-owned `release-check` inventory 和 Batch 131 façade freeze invariant 后，增强 `release-check` 的机器可读 gate profile，让 CI/本机脚本可在真正执行命令前解析 recommended minimum 的 step 类型、repo-local path 和 present/resolved 状态，而不是继续扩张 PowerShell 编排或直接新增外部 CI workflow。

实施范围：

- 扩展 `internal/rekit/releasecheck` 的 JSON envelope，新增 `gateProfile`，包含 `name=local-ci-minimum`、`defaultFor=[local,ci]`、`stepCount`、`largeMatrixDefault=false` 和逐 step 的 `kind`、`repoPath`、`present`、`resolved`。
- 将 catalog `recommendedMinimum` 和 release-check `requiredCommands` 统一解析为 `GateStep`，识别 `go-run`、`go-check`、`powershell-smoke`、`powershell-facade` 与 `git-check`；对 repo-local 脚本和入口做存在性检查。
- 扩展 `/rekit release-check` text 输出，展示 gate profile summary、命令 kind 和 repo-local path；JSON 输出保持只读 inventory，不执行任何验证命令。
- 扩展 `internal/rekit/cli` 测试，覆盖 gate profile JSON/text、step kind/path、present/resolved、required command/catalog 对齐与既有文档/pack/known gaps 清单。
- 更新 `docs/release-readiness.md`、`docs/go-first-convergence-plan.md`、CHANGELOG 与 batch-plan，记录 Batch 139 的本机/CI gate profile 定位。

边界：本批不新增 `.github/workflows` 或其它外部 CI workflow，不执行 recommended minimum 编排，不写 case/pack/runtime state，不改变 façade safe-set，不迁移 policy schema，不删除 PowerShell runtime，不执行网络/扫描/fuzz/exploit replay/debug/dump/patch/hook/device/heavy-tool，不写 authority/confirmed。

验证计划：

```powershell
go test ./internal/rekit/cli -run TestRunReleaseCheck -count=1
go test ./internal/rekit/cli
go test ./internal/rekit/manifest
go run ./cmd/rekit -- -Command release-check -Format json
.\rekit\rekit.ps1 -Command release-check -Format json
go test ./...
go vet ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`go test ./internal/rekit/cli -run TestRunReleaseCheck -count=1`、`go test ./internal/rekit/cli`、`go test ./internal/rekit/manifest`、`go run ./cmd/rekit -- -Command release-check -Format json`、`/rekit release-check -Format json`、`go test ./...`、`go vet ./...` 与 `/rekit doctor` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 140：轻量 CI release gate

状态：已完成。

目标：承接 Stage 8 中“建立轻量 CI：Go checks + doctor + 少量 Windows façade smoke；不要把大型 PowerShell matrix 作为默认必跑”的未完成项，在 Batch 139 `release-check` gate profile 之后，新增最小 GitHub Actions release gate，让远程 CI 默认覆盖 Go release inventory、Go tests/vet、Windows doctor 与 façade smoke，同时不执行大型 pack matrix、真实临时 case smoke 或 heavy-tool 相关步骤。

实施范围：

- 新增 `.github/workflows/release-gate.yml`，包含 `Go release checks` job：Ubuntu 上运行 `go run ./cmd/rekit -- -Command release-check -Format json`、`go test ./...` 与 `go vet ./...`。
- 新增 `Windows facade smoke` job：Windows 上运行 `go run ./cmd/rekit -- -Command release-check -Format json`、`.\rekit\rekit.ps1 -Command doctor` 与 `.\rekit\tests\facade-smoke.ps1`。
- 扩展 `internal/rekit/manifest/release_invariants_test.go`，锁定 CI workflow 必含 release-check、Go tests/vet、Windows doctor/façade smoke，并禁止默认运行 `pack-smoke-matrix.ps1`、真实 case smoke 或 heavy-tool 相关步骤。
- 更新 README、CLAUDE.md、`rekit/tests/README.md`、`docs/powershell-deprecation.md`、`docs/release-readiness.md`、`docs/go-first-convergence-plan.md`、CHANGELOG 与 batch-plan，记录轻量 CI gate 已进入默认发布路径，并明确大型 pack matrix / 真实临时 case smoke 不属于默认 CI。

边界：本批只新增轻量 CI gate 与静态 invariant；不运行大型 PowerShell matrix、不运行真实临时 case smoke、不执行 samples/network/scans/fuzzing/exploit replay/debug/dump/patch/hook/device/heavy-tool、不写 authority/confirmed、不迁移 policy schema、不删除 PowerShell runtime、不改变 runtime 写入面。

验证计划：

```powershell
go test ./internal/rekit/manifest -run TestReleaseGateWorkflowInvariants -count=1
go test ./internal/rekit/manifest
go run ./cmd/rekit -- -Command release-check -Format json
go test ./...
go vet ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`go test ./internal/rekit/manifest -run TestReleaseGateWorkflowInvariants -count=1`、`go test ./internal/rekit/manifest`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json` 与 `/rekit doctor` 均通过；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。文档收尾已检查并更新 README、CLAUDE.md、`rekit/tests/README.md` 与 `docs/powershell-deprecation.md`。

### Batch 141：长期自主 goal 与新会话接手指南

状态：已完成。

目标：回应维护者希望“给出后续优化大方向、防止上下文压缩后方向偏移、提供可复制 goal 语句、避免每轮改动过小”的要求，把未来几十到上百轮自主推进的大方向、自审/评估/自行调整循环和新会话接手 prompt 固化为简短仓库锚点，并用 Go release invariant 锁定入口；同时按后续反馈避免写成过长约束清单，以免束缚模型发挥。

实施范围：

- 新增并精简 `docs/autonomous-goal.md`，保留读取指南、实施摘要、执行清单、验证标准、风险与注意事项、五个长期大方向和可直接复制的 goal 语句；强调它只是简短接手锚点，不是新的限制清单。
- 在 README、CLAUDE.md、`docs/go-first-convergence-plan.md`、`docs/release-readiness.md`、`docs/vision.md` 与 `docs/reference-absorption.md` 顶部导航或接手提示中接入 `docs/autonomous-goal.md`，让新会话优先读取该文档。
- 扩展 `internal/rekit/manifest/release_invariants_test.go`，新增 `TestAutonomousGoalGuideInvariants`，锁定文档必备章节、五个大方向、关键边界和入口链接。
- 将 `docs/autonomous-goal.md` 纳入 Go-owned `release-check` required documents，并扩展 CLI JSON/text 测试，确保 release inventory 也把长期 autonomous goal 视为接手门禁入口。
- 更新 CHANGELOG 与 batch-plan，记录 Batch 141 作为 Stage 8 的新会话接手质量/governance 收口批次。

边界：本批只新增长期 goal / 接手指南文档、只读 release-check 文档 inventory 与静态 invariant；不改变 runtime 写入面、PowerShell façade、CI workflow、pack manifest、case state 或 smoke 行为；不执行真实 case、网络、扫描、fuzz、exploit replay、debug、dump、patch、hook、设备连接或 heavy-tool；不写 authority/confirmed；不迁移 policy schema；不删除 PowerShell runtime。

验证计划：

```powershell
go test ./internal/rekit/manifest -run TestAutonomousGoalGuideInvariants -count=1
go test ./internal/rekit/cli -run TestRunReleaseCheck -count=1
go test ./internal/rekit/manifest
go run ./cmd/rekit -- -Command release-check -Format json
go test ./...
go vet ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`go test ./internal/rekit/manifest -run TestAutonomousGoalGuideInvariants -count=1`、`go test ./internal/rekit/cli -run TestRunReleaseCheck -count=1`、`go test ./internal/rekit/manifest`、`go run ./cmd/rekit -- -Command release-check -Format json`、`go test ./...`、`go vet ./...` 与 `/rekit doctor` 均通过；`release-check` documents 已包含 `docs/autonomous-goal.md`；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 142：PowerShell deprecation release-check inventory

状态：已完成。

目标：承接 Stage 8 / PowerShell 收缩方向，在 Batch 129 deprecation strategy、Batch 131 façade freeze invariant、Batch 139 gate profile 和 Batch 140 轻量 CI 之后，把 PowerShell deprecation readiness 从纯文档/静态 invariant 进一步纳入 Go-owned `release-check` 机器可读 inventory，使本机/CI 和新会话能在真正考虑 fallback freeze/removal 前看到命令归属、模块状态、freeze gates、blocked migrations 与实际 façade/default/module 清单是否漂移。

实施范围：

- 扩展 `internal/rekit/releasecheck` JSON envelope，新增 `powerShellDeprecation`，包含 `strategyDocument`、`ready`、`summary`、`commandOwnership[]`、`moduleStatus[]`、`freezeGates[]`、`blockedMigrations[]` 与 `warnings[]`。
- 从 `docs/powershell-deprecation.md` 解析命令归属矩阵、`rekit/lib/*.ps1` 模块状态、Freeze / deprecation gates 与禁止迁移清单，并对照 `rekit/rekit.ps1` 的 `ValidateSet` / `Test-RekitGoDefaultDelegationCommand` 与实际 `rekit/lib/*.ps1` 清单发现漂移。
- 扩展 `/rekit release-check` text 输出，展示 PowerShell deprecation summary、命令/模块/freeze/blocked 计数和 warnings；若 deprecation inventory 发现漂移，则 release-check `ready=false`。
- 扩展 `internal/rekit/cli` release-check JSON/text 测试，锁定 `powerShellDeprecation.ready=true`、代表性 Go-default/legacy/blocked 命令、代表性模块和 text summary。
- 更新 `docs/release-readiness.md`、`docs/powershell-deprecation.md`、`docs/go-first-convergence-plan.md`、CHANGELOG 与 batch-plan，记录 Batch 142 的定位和验证边界；补充 `sync / update` alias 的 deprecation 矩阵归属，避免默认 façade 委托集合与文档漂移。

边界：本批只做只读 release inventory、测试和文档；不删除 PowerShell runtime、不改变默认 façade 委托集合、不扩大写入面、不执行大型 PowerShell matrix、不运行真实临时 case、不迁移 policy schema、不执行 samples/network/scans/fuzz/exploit replay/debug/dump/patch/hook/device/heavy-tool、不写 authority/confirmed。

验证计划：

```powershell
go test ./internal/rekit/releasecheck ./internal/rekit/cli
go test ./internal/rekit/cli -run TestRunReleaseCheck -count=1
go test ./internal/rekit/manifest -run TestPowerShell -count=1
go run ./cmd/rekit -- -Command release-check -Format json
go test ./...
go vet ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

验证结果：全部通过。`go test ./internal/rekit/releasecheck ./internal/rekit/cli`、`go test ./internal/rekit/cli -run TestRunReleaseCheck -count=1`、`go test ./internal/rekit/manifest -run TestPowerShell -count=1`、`go run ./cmd/rekit -- -Command release-check -Format json`、`go test ./...`、`go vet ./...` 与 `/rekit doctor` 均通过；`release-check` JSON 已包含 `powerShellDeprecation.ready=true`、14 条命令归属、14 个 PowerShell 模块、8 个 freeze gate 与 5 条 blocked migration；`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 143：Manifest-driven heavy-tool gate readiness

状态：已完成。

目标：承接 Stage 7 / Stage 8 的 pack-neutral 与 release readiness 方向，把 Go gate runtime 中硬编码的 heavy-tool action 清单推进为 pack manifest 驱动的可审计契约，并让 release inventory 暴露当前 pack 声明的 gate action 集合；保持 `gate -WhatIf/-Apply` 只做 preview / pending-gate request，不执行 heavy-tool。

实施范围：

- 扩展 `internal/rekit/manifest`：新增 `HeavyToolGate` / `heavyToolGates` 解析、schema 校验、summary 计数与 `heavyToolGateActions[]`，要求每个 pack 显式声明 action id、sideEffects、defaultRisk、requiresConfirmation 与 stopConditions。
- 所有 pack manifest（`_template`、`vmp-re` 与安全领域 skeleton packs）新增 `heavyToolGates`，统一声明 `debug`、`dump`、`full-trace`、`inject`、`network`、`patch`、`symex` 的 review-first gate contract。
- `internal/rekit/gate` 改为从 pack manifest 读取允许 action；未声明 action 直接拒绝，默认 `Risk` 与默认 `StopConditions` 来自 manifest，且仍只生成 preview 或 append pending-gate request。
- `release-check -Format json` 新增 `heavyToolGateActions[]`，`/rekit packs -Format json` 暴露每个 pack 的 `heavyToolGates` 与 `heavyToolGateActions[]`；文本 `packs` 保持既有列兼容，避免破坏 PowerShell inventory smoke。
- PowerShell fallback 同步解析、校验和输出 `heavyToolGates` / `heavyToolGateActions[]`，确保 `REKIT_GO_DISABLE=1` 下的 `/rekit packs -Format json` 与 Go inventory 保持契约一致。
- 扩展 Go tests 与 PowerShell smoke 覆盖 manifest gate schema、gate manifest-declared action / undeclared action、release-check gate action inventory、packs JSON inventory、fallback invalid heavyToolGates 与 promote fixture schema 兼容。
- 更新 `rekit/schemas/pack-manifest.schema.yml`、`common/policies/tool-adapters.md`、`docs/release-readiness.md`、`docs/powershell-deprecation.md`、`docs/go-first-convergence-plan.md` 与 CHANGELOG，记录 manifest-driven heavy-tool gate readiness 与 `symex` gate。

边界：本批只做 pack manifest contract、Go preview/ledger 语义、只读 release inventory、PowerShell fallback 兼容修复、测试和文档；不执行 full-trace/debug/inject/patch/dump/network/symex/heavy-tool，不新增外部 adapter，不连接设备或网络，不写 authority/confirmed，不扩大 PowerShell façade 委托集合，不迁移 policy schema，不改变 `gate -Apply` 只写 pending-gate request 的边界。

验证计划：

```powershell
go test ./internal/rekit/manifest ./internal/rekit/gate ./internal/rekit/releasecheck ./internal/rekit/cli
go test ./...
go vet ./...
go run ./cmd/rekit -- -Command release-check -Format json
.\rekit\rekit.ps1 -Command doctor
.\rekit\tests\facade-smoke.ps1
.\rekit\tests\pack-inventory-smoke.ps1
git diff --check
```

验证结果：已通过定向 Go package tests、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`/rekit doctor`、默认自包含 `facade-smoke.ps1` 与 `pack-inventory-smoke.ps1`；`pack-inventory-smoke.ps1` 已覆盖 Go / façade / `REKIT_GO_DISABLE=1` fallback JSON 中的 `heavyToolGates=7` 与 `heavyToolGateActions=[debug,dump,full-trace,inject,network,patch,symex]`，并验证 invalid `heavyToolGates` 在 Go 与 PowerShell fallback 下均标记 schema error。`release-check` JSON 已包含 `heavyToolGateActions=[debug,dump,full-trace,inject,network,patch,symex]`，所有 pack summary 均显示 `heavyToolGates=7`。`git diff --check` 仍仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 144：CI release gate inventory

状态：已完成。

目标：承接 Stage 8 release readiness 方向，把 `.github/workflows/release-gate.yml` 的轻量 CI contract 纳入 Go-owned `release-check` 机器可读 inventory，使本机/CI/新会话能发现 release gate workflow 的 job、required command 或 forbidden broad/heavy step 漂移，而不是只靠静态文档和 manifest invariant。

实施范围：

- 新增 `internal/rekit/releasecheck` 的 `CIReleaseGate` inventory，解析 `.github/workflows/release-gate.yml` 的 workflow checks、required jobs、required commands 与 forbidden broad/heavy steps，并在漂移时让 `release-check.ready=false`。
- `release-check -Format json` 新增 `ciReleaseGate`；text 输出新增 CI release gate summary 和 warnings。
- 扩展 releasecheck / CLI / release invariant tests，覆盖 repo workflow inventory、drift detection、JSON/text envelope 与 release readiness 文档锚点。
- 更新 `docs/release-readiness.md`、`docs/go-first-convergence-plan.md`、CHANGELOG 与本 batch-plan，记录 `ciReleaseGate.ready`、`requiredCommands[]` 和 `forbiddenStrings[]` 的 release readiness 语义。

边界：本批只做确定性 CI workflow inventory、测试和文档；不执行 GitHub Actions、不新增外部服务调用、不改变 CI workflow 本身、不扩大 PowerShell façade、不执行大型 PowerShell matrix、真实临时 case smoke 或 heavy-tool、不写 case/pack runtime state、不写 authority/confirmed。

验证计划：

```powershell
go test ./internal/rekit/releasecheck ./internal/rekit/cli ./internal/rekit/manifest
go test ./...
go vet ./...
go run ./cmd/rekit -- -Command release-check -Format json
.\rekit\rekit.ps1 -Command doctor
.\rekit\tests\facade-smoke.ps1
git diff --check
```

验证结果：已通过 `go test ./internal/rekit/releasecheck ./internal/rekit/cli ./internal/rekit/manifest`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`/rekit doctor` 与 `facade-smoke.ps1`；`release-check` JSON 已包含 `ciReleaseGate.ready=true`、2 个 required jobs、6 条 required commands 与 10 条 forbidden broad/heavy step checks；text 输出显示 `CI release gate: .github/workflows/release-gate.yml ready=true jobs=2 commands=6 forbidden=10`。`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 145：Release-check failure semantics

状态：已完成。

目标：承接 Stage 8 release readiness 方向，修复 `release-check` 作为 CI 前置 inventory 时的退出语义：当 inventory 已经判断 `ready=false` 时，CLI 必须仍输出完整 JSON/text 诊断，但最终返回非零，避免本机/CI 只记录 warning 却错误通过。

实施范围：

- 将 `/rekit release-check` 的输出与 readiness 退出语义拆分为 `emitReleaseCheckResult` 与 `writeReleaseCheckResult`：先输出完整 inventory，再在 `ready=false` 时返回包含 warnings 的错误。
- 扩展 CLI tests，覆盖 JSON 和 text 两种 not-ready 输出均先写诊断再返回错误。
- 更新 `docs/release-readiness.md`、CHANGELOG 与本 batch-plan，记录 `ready=false` 必须非零退出的 release gate 语义。

边界：本批只改变 Go `release-check` 的失败退出语义、测试和文档；不改变 inventory 规则、不执行 CI、不新增 PowerShell runtime/fallback 能力、不运行大型 PowerShell matrix、不执行 heavy-tool、不写 case/pack runtime state、不写 authority/confirmed。

验证计划：

```powershell
go test ./internal/rekit/cli
go test ./...
go vet ./...
go run ./cmd/rekit -- -Command release-check -Format json
.\rekit\rekit.ps1 -Command doctor
.\rekit\tests\facade-smoke.ps1
git diff --check
```

验证结果：已通过 `go test ./internal/rekit/cli`、`go run ./cmd/rekit -- -Command release-check -Format json`（ready path 仍零退出）、`go test ./...`、`go vet ./...`、`/rekit doctor` 与 `facade-smoke.ps1`；新增 CLI tests 覆盖 JSON/text not-ready inventory 均先输出诊断再返回 `release-check not ready` 错误。`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 146：Release handoff summary inventory

状态：已完成。

目标：承接 Stage 8 release readiness / 新会话接手质量方向，把 release maintainer 和后续 AI 最先需要的 read-first 文档、关键 readiness signal、latest batch 摘要、最小验证命令与 next actions 纳入 Go-owned `release-check` envelope，避免接手时必须先通读多个长文档或只依赖聊天上下文。

实施范围：

- 新增 `internal/rekit/releasecheck/release_handoff.go`，构建 `ReleaseHandoff` inventory：`readFirst[]`、`signals[]`、`latestBatch`、`validation[]`、`nextActions[]` 与 warnings。
- `release-check -Format json` 新增 `releaseHandoff`；text 输出新增 release handoff summary，展示 handoff readiness、read-first 数量、signal 数量、validation 数量与 latest batch。
- `releaseHandoff` 对照必读文档存在性、关键 readiness signal、latest `docs/batch-plan.md` 批次状态/目标/验证结果和 gate profile validation commands；缺失时让 `release-check.ready=false` 并输出诊断。
- 扩展 releasecheck / CLI / release invariant tests，覆盖 repo handoff inventory、缺失接手文档 drift detection、JSON/text envelope 与 release readiness 文档锚点。
- 更新 README、`docs/release-readiness.md`、`docs/go-first-convergence-plan.md`、CHANGELOG 与本 batch-plan，记录 `releaseHandoff.ready`、`readFirst[]`、`signals[]`、`latestBatch`、`validation[]` 与 `nextActions[]` 的接手语义。

边界：本批只做确定性 release inventory、测试和文档；不执行 CI、不新增外部服务调用、不改变 PowerShell façade 委托集合、不运行大型 PowerShell matrix、不执行 heavy-tool、不写 case/pack runtime state、不写 authority/confirmed。

验证计划：

```powershell
go test ./internal/rekit/releasecheck ./internal/rekit/cli ./internal/rekit/manifest
go test ./...
go vet ./...
go run ./cmd/rekit -- -Command release-check -Format json
.\rekit\rekit.ps1 -Command doctor
.\rekit\tests\facade-smoke.ps1
git diff --check
```

验证结果：已通过 `go test ./internal/rekit/releasecheck ./internal/rekit/cli ./internal/rekit/manifest`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`/rekit doctor` 与 `facade-smoke.ps1`；`release-check` JSON 已包含 `releaseHandoff.ready=true`、6 个 `readFirst[]`、5 个 `signals[]`、latest batch 摘要、10 条 validation command 与 next actions。`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 147：Release notes freshness gate

状态：已完成。

目标：继续 Stage 8 release readiness / 新会话接手质量方向，把最新完成批次是否已经写入 `CHANGELOG.md` `Unreleased` 纳入 Go-owned `releaseHandoff`，避免 batch-plan 已记录完成但 release notes 漏写，导致发布维护者或后续 AI 只能从聊天上下文补齐用户可见变化。

实施范围：

- `ReleaseHandoff` 新增 `releaseNotes` inventory，记录 `CHANGELOG.md` 路径、`Unreleased` section、latest batch id、covered 状态与 summary。
- `latestBatch` 新增 `batchId`，从 `docs/batch-plan.md` 最新 `### Batch N` 标题解析，用于 release notes freshness 对照。
- `releaseHandoff.signals[]` 新增 `release notes freshness` signal；若 `CHANGELOG.md` 缺失或 `Unreleased` 未覆盖最新 batch id，则 `releaseHandoff.ready=false` 并让 `release-check.ready=false`。
- text `release-check` handoff summary 新增 `releaseNotes=true/false`，便于人工快速看到 freshness 状态。
- 扩展 releasecheck / CLI / release invariant tests，覆盖 repo freshness、stale release notes drift detection、batch id parsing、JSON/text envelope 与 release readiness 文档锚点。
- 更新 `CHANGELOG.md`、`docs/release-readiness.md`、`docs/go-first-convergence-plan.md` 与本 batch-plan。

边界：本批只做确定性 release inventory、测试和文档；不执行 CI、不新增外部服务调用、不改变 PowerShell façade 委托集合、不运行大型 PowerShell matrix、不执行 heavy-tool、不写 case/pack runtime state、不写 authority/confirmed。

验证计划：

```powershell
go test ./internal/rekit/releasecheck ./internal/rekit/cli ./internal/rekit/manifest
go test ./...
go vet ./...
go run ./cmd/rekit -- -Command release-check -Format json
.\rekit\rekit.ps1 -Command doctor
.\rekit\tests\facade-smoke.ps1
git diff --check
```

验证结果：已通过 `go test ./internal/rekit/releasecheck ./internal/rekit/cli ./internal/rekit/manifest`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`/rekit doctor` 与 `facade-smoke.ps1`；`release-check` JSON/text 已包含 `releaseHandoff.releaseNotes.covered=true` 与 `release notes freshness` signal，stale release notes fixture 会让 `release-check.ready=false`。`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 148：Release handoff known gaps inventory

状态：已完成。

目标：继续 Stage 8 release readiness / 新会话接手质量方向，把 `docs/release-readiness.md` `Known gaps` 纳入 Go-owned `releaseHandoff`，让 release maintainer 和后续 AI 能从 `release-check -Format json` 先看到当前缺口的机器可读 category/summary，而不必只依赖长文档或聊天上下文。

实施范围：

- `ReleaseHandoff` 新增 `knownGaps[]` inventory，按 `docs/release-readiness.md` `Known gaps` 条目生成 `index`、`category` 与 compact `summary`。
- `releaseHandoff.signals[]` 新增 `known gaps summary` signal；若 release readiness 没有可解析 known gaps，则 `releaseHandoff.ready=false` 并让 `release-check.ready=false`。
- text `release-check` handoff summary 新增 `knownGaps=<n>`，便于人工快速看到接手缺口数量。
- 扩展 releasecheck / CLI / release invariant tests，覆盖 repo known gaps inventory、缺失 known gaps drift detection、JSON/text envelope 与 release readiness 文档锚点。
- 更新 `CHANGELOG.md`、`docs/release-readiness.md`、`docs/go-first-convergence-plan.md` 与本 batch-plan。

边界：本批只做确定性 release inventory、测试和文档；不执行 CI、不新增外部服务调用、不改变 PowerShell façade 委托集合、不运行大型 PowerShell matrix、不执行 heavy-tool、不写 case/pack runtime state、不写 authority/confirmed。

验证计划：

```powershell
go test ./internal/rekit/releasecheck ./internal/rekit/cli ./internal/rekit/manifest
go test ./...
go vet ./...
go run ./cmd/rekit -- -Command release-check -Format json
.\rekit\rekit.ps1 -Command doctor
.\rekit\tests\facade-smoke.ps1
git diff --check
```

验证结果：已通过 `go test ./internal/rekit/releasecheck ./internal/rekit/cli ./internal/rekit/manifest`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`/rekit doctor` 与 `facade-smoke.ps1`；`release-check` JSON/text 已包含 `releaseHandoff.knownGaps[]`、`known gaps summary` signal 与 `knownGaps=5` handoff summary，缺失 known gaps fixture 会让 `release-check.ready=false`。`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 149：Release handoff pack maturity summary

状态：已完成。

目标：继续 Stage 8 release readiness / 新会话接手质量方向，把 pack maturity matrix 与 manifest heavy-tool gate 覆盖状态纳入 Go-owned `releaseHandoff`，让 release maintainer 和后续 AI 不必遍历完整 `packs[]` 就能先看到 mature/template/skeleton 覆盖、schema validity 和每 pack gate readiness。

实施范围：

- `ReleaseHandoff` 新增 `packMaturity` inventory，记录 pack 总数、`maturityCounts`、`packsByMaturity`、schema validity、manifest heavy-tool gate readiness、全局 gate actions 与每 pack gate status。
- `releaseHandoff.signals[]` 新增 `pack maturity summary` signal；若 pack 缺失、schema invalid 或任一 pack 未声明 heavy-tool gates，则 signal not ready 并让 `release-check.ready=false`。
- text `release-check` handoff summary 新增 `packMaturity=<n>`，便于人工快速看到接手 pack 覆盖数量。
- 扩展 releasecheck / CLI / release invariant tests，覆盖 repo pack maturity inventory、缺失 heavy-tool gate drift detection、JSON/text envelope 与 release readiness 文档锚点。
- 更新 `CHANGELOG.md`、`docs/release-readiness.md`、`docs/go-first-convergence-plan.md` 与本 batch-plan。

边界：本批只做确定性 release inventory、测试和文档；不执行 CI、不新增外部服务调用、不改变 PowerShell façade 委托集合、不运行大型 PowerShell matrix、不执行 heavy-tool、不写 case/pack runtime state、不写 authority/confirmed。

验证计划：

```powershell
go test ./internal/rekit/releasecheck ./internal/rekit/cli ./internal/rekit/manifest
go test ./...
go vet ./...
go run ./cmd/rekit -- -Command release-check -Format json
.\rekit\rekit.ps1 -Command doctor
.\rekit\tests\facade-smoke.ps1
git diff --check
```

验证结果：已通过 `go test ./internal/rekit/releasecheck ./internal/rekit/cli ./internal/rekit/manifest`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`/rekit doctor` 与 `facade-smoke.ps1`；`release-check` JSON/text 已包含 `releaseHandoff.packMaturity`、`pack maturity summary` signal 与 `packMaturity=10` handoff summary，缺失 heavy-tool gate 的 pack maturity fixture 会把 pack maturity inventory 标记为 warnings。`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 150：Go runtime default pack 收口

状态：已完成。

目标：回到 Stage 7 pack-neutral hardening，先把 Go runtime 中散落的默认 pack `vmp-re` 决策收口到单一 Go package，避免后续继续在 CLI parse、runtime context、manifest load/normalize 与 legacy instance metadata fallback 中复制 default pack literal；本批不改变默认 pack 值，只降低 pack-neutral 漂移风险。

实施范围：

- 新增 `internal/rekit/defaults.DefaultPack` 作为 Go runtime 的单一默认 pack 决策点。
- `cli.Parse`、`runtime.New`、`manifest.Load` / `normalizePackID` 与 `instance.Read` 的空 pack / legacy fallback 改为引用 `defaults.DefaultPack`。
- 更新 runtime / instance / manifest / CLI / release handoff 相关 tests，避免默认 pack 断言继续复制 literal。
- 新增 release invariant：扫描 `internal/rekit/**`，只允许 production Go default pack literal 出现在 `internal/rekit/defaults/defaults.go` 与 manifest 非 vmp path guard 文案/检测中；显式 pack fixture test 文件保留 allowlist。
- 更新 `CHANGELOG.md`、`docs/go-first-convergence-plan.md` 与本 batch-plan。

边界：本批只做 Go pack-neutral hardening、测试和文档；不改变默认 pack 值、不改变 PowerShell façade 委托集合、不运行大型 PowerShell matrix、不执行 heavy-tool、不创建或修改真实 case state、不写 authority/confirmed。

验证计划：

```powershell
go test ./internal/rekit/manifest ./internal/rekit/runtime ./internal/rekit/instance ./internal/rekit/cli ./internal/rekit/releasecheck
go test ./...
go vet ./...
go run ./cmd/rekit -- -Command release-check -Format json
.\rekit\rekit.ps1 -Command doctor
.\rekit\tests\facade-smoke.ps1
git diff --check
```

验证结果：初次 `go test ./...` 在本批仍标记“进行中”且缺验证结果时，按预期触发既有 release handoff freshness guard（`release-check` 不允许最新 batch 未完成）；补齐本批完成状态后，已通过 targeted `go test ./internal/rekit/manifest ./internal/rekit/runtime ./internal/rekit/instance ./internal/rekit/cli ./internal/rekit/releasecheck`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`/rekit doctor` 与 `facade-smoke.ps1`；`release-check` 输出 `ready=true`。`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 151：Manifest promote deny baseline 显式化

状态：已完成。

目标：继续 Stage 7 pack-neutral hardening，将 Go manifest loader 中对空 `promoteDenyPatterns` 的隐式 deny baseline 移出 runtime，避免新增 pack 未声明 deny baseline 时静默继承带 RE/VMP 倾向的 artifact/trace/dump 规则；本批不改既有 pack manifest 内容，只让 manifest contract 更显式。

实施范围：

- 移除 `manifest.Load` 对空 `PromoteDenyPatterns` 的隐式 fallback 注入。
- `ValidateSchema` 新增 `manifest must explicitly declare promoteDenyPatterns` 检查，保持正则有效性与空 pattern 检查。
- `rekit/schemas/pack-manifest.schema.yml` 将 `promoteDenyPatterns` 列入 required，并说明 runtime 不提供隐式 deny baseline。
- `docs/pack-authoring.md` 的新 pack 示例改为显式列出 `_template` 通用 baseline，并提醒新增 pack 从该 baseline 开始按领域补充。
- 重构 manifest tests 的 valid fixture，并新增缺失 `promoteDenyPatterns` 的 drift test。
- 更新 `CHANGELOG.md`、`docs/go-first-convergence-plan.md` 与本 batch-plan。

边界：本批只做 pack-neutral manifest contract、测试和文档；不改变既有 pack manifest 内容、不改变 sync/promote apply 行为、不改变 PowerShell façade 委托集合、不运行大型 PowerShell matrix、不执行 heavy-tool、不创建或修改真实 case state、不写 authority/confirmed。

验证计划：

```powershell
go test ./internal/rekit/manifest ./internal/rekit/promote ./internal/rekit/cli ./internal/rekit/releasecheck
go test ./...
go vet ./...
go run ./cmd/rekit -- -Command release-check -Format json
.\rekit\rekit.ps1 -Command doctor
.\rekit\tests\facade-smoke.ps1
git diff --check
```

验证结果：已通过 targeted `go test ./internal/rekit/manifest ./internal/rekit/promote ./internal/rekit/cli ./internal/rekit/releasecheck`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`/rekit doctor` 与 `facade-smoke.ps1`；`release-check` 输出 `ready=true`。`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 152：Manifest promoteFiles 显式化

状态：已完成。

目标：继续 Stage 7 pack-neutral hardening，将 Go manifest loader 中对空 `promoteFiles` 的 `managedFiles` 隐式 fallback 移出 runtime，避免新增 pack 未声明回流范围时静默把全部 managed files 纳入 case → kit promote 面；本批不改既有 pack manifest 内容，只让 manifest contract 更显式。

实施范围：

- 移除 `manifest.Load` 对空 `PromoteFiles` 的 `ManagedFiles` 隐式 fallback 注入。
- `ValidateSchema` 新增 `manifest must explicitly declare promoteFiles` 检查，并继续要求每个 `promoteFiles` entry 必须属于 `managedFiles` 子集。
- `rekit/schemas/pack-manifest.schema.yml` 将 `promoteFiles` 列入 required，并说明 runtime 不再把 `managedFiles` 隐式作为 promote 回流范围。
- `docs/pack-authoring.md` 明确新增 pack 必须显式声明 `promoteFiles`，作为允许 case → kit 回流的 managed 文件子集。
- manifest tests 新增缺失 `promoteFiles` 的 load/no-fallback drift test，确保 loader 不再自动填充 managed files。
- 更新 `CHANGELOG.md`、`docs/go-first-convergence-plan.md` 与本 batch-plan。

边界：本批只做 pack-neutral manifest contract、测试和文档；不改变既有 pack manifest 内容、不改变 sync/promote apply 行为、不改变 PowerShell façade 委托集合、不运行大型 PowerShell matrix、不执行 heavy-tool、不创建或修改真实 case state、不写 authority/confirmed。

验证计划：

```powershell
go test ./internal/rekit/manifest ./internal/rekit/promote ./internal/rekit/cli ./internal/rekit/releasecheck
go test ./...
go vet ./...
go run ./cmd/rekit -- -Command release-check -Format json
.\rekit\rekit.ps1 -Command doctor
.\rekit\tests\facade-smoke.ps1
git diff --check
```

验证结果：已通过 targeted `go test ./internal/rekit/manifest ./internal/rekit/promote ./internal/rekit/cli ./internal/rekit/releasecheck`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`/rekit doctor` 与 `facade-smoke.ps1`；`release-check` 输出 `ready=true`。`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 153：Manifest budgets 默认预算显式化

状态：已完成。

目标：继续 Stage 7 pack-neutral hardening，将 Go manifest loader 中对缺失 `budgets.defaultMarkdown` 的 `16384` 隐式注入移出 manifest load，避免新增 pack 静默继承 runtime 默认 Markdown/text 预算；本批不改既有 pack manifest 内容，只让 manifest contract 更显式。

实施范围：

- 移除 `manifest.Load` 对缺失 `Budgets["defaultMarkdown"]` 的 `16384` 隐式注入。
- `ValidateSchema` 新增 `manifest must explicitly declare budgets.defaultMarkdown` 检查，并要求所有 budgets value 是正整数。
- 保留 `BudgetLimit` 的运行时安全 fallback：当调用方在 schema validation 外直接查询预算时仍返回 `16384`，但 schema-valid pack 必须显式声明默认预算。
- `doctor.Case` 在使用 manifest budgets 前补 `ValidateSchema`，与 pack doctor/list/release inventory 的 schema gate 对齐。
- `rekit/schemas/pack-manifest.schema.yml` 说明 budgets 必须显式声明 `defaultMarkdown` 正整数，runtime 不再在 manifest load 时注入默认预算。
- `docs/pack-authoring.md` 明确新增 pack 的 budgets 不能依赖 runtime 默认注入。
- manifest tests 新增缺失/非法 `budgets.defaultMarkdown` 与 load/no-fallback drift test。
- 更新 `CHANGELOG.md`、`docs/go-first-convergence-plan.md` 与本 batch-plan。

边界：本批只做 pack-neutral manifest contract、测试和文档；不改变既有 pack manifest 内容、不改变 sync/promote apply 行为、不改变 PowerShell façade 委托集合、不运行大型 PowerShell matrix、不执行 heavy-tool、不创建或修改真实 case state、不写 authority/confirmed。

验证计划：

```powershell
go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/promote ./internal/rekit/cli ./internal/rekit/releasecheck
go test ./...
go vet ./...
go run ./cmd/rekit -- -Command release-check -Format json
.\rekit\rekit.ps1 -Command doctor
.\rekit\tests\facade-smoke.ps1
git diff --check
```

验证结果：已通过 targeted `go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/promote ./internal/rekit/cli ./internal/rekit/releasecheck`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`/rekit doctor` 与 `facade-smoke.ps1`；`release-check` 输出 `ready=true`。`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 154：Manifest laneTypes 工作线边界显式化

状态：已完成。

目标：继续 Stage 7 pack-neutral hardening，将 Go manifest parser 中对缺失 `laneTypes.title` 与 `workspaceRoot` 的隐式默认值移出 runtime，并要求 pack 显式声明 lane title、workspaceRoot 与 canWrite/readOnly/outputs 边界，避免新增 pack 静默继承 runtime 的 lane 展示名或 workspace 默认路径；本批不改既有 pack manifest 内容，只让 manifest contract 更显式。

实施范围：

- 移除 `yamlLaneTypes` 对缺失 `title` 的 `id` fallback 与缺失 `workspaceRoot` 的 `captures/lanes` fallback。
- `ValidateSchema` 新增 `validateLaneTypes`，要求 pack 显式声明 `laneTypes`，每个 lane 必须有 id、title、workspaceRoot、canWrite、readOnly 与 outputs，且 lane id 不重复。
- 保持 `defaultAuthorityLane`、`defaultStartLaneType` 与 `requestDefaultTargetLane` 必须引用已声明 lane，并保持 default authority lane 必须 `authority=true`。
- `rekit/schemas/pack-manifest.schema.yml` 说明 `laneTypes` 不再由 runtime 注入 title/workspaceRoot 默认值。
- `docs/pack-authoring.md` 明确新增 pack 的 `laneTypes` 必须显式声明工作线显示名、workspace 和读写/输出边界。
- manifest tests 新增缺失 lane title/workspaceRoot/canWrite 与 load/no-fallback drift test。
- 更新 `CHANGELOG.md`、`docs/go-first-convergence-plan.md` 与本 batch-plan。

边界：本批只做 pack-neutral manifest contract、测试和文档；不改变既有 pack manifest 内容、不改变 sync/promote apply 行为、不改变 PowerShell façade 委托集合、不运行大型 PowerShell matrix、不执行 heavy-tool、不创建或修改真实 case state、不写 authority/confirmed。

验证计划：

```powershell
go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/workstream ./internal/rekit/promote ./internal/rekit/cli ./internal/rekit/releasecheck
go test ./...
go vet ./...
go run ./cmd/rekit -- -Command release-check -Format json
.\rekit\rekit.ps1 -Command doctor
.\rekit\tests\facade-smoke.ps1
git diff --check
```

验证结果：已通过 targeted `go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/workstream ./internal/rekit/promote ./internal/rekit/cli ./internal/rekit/releasecheck`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`/rekit doctor` 与 `facade-smoke.ps1`；`release-check` 输出 `ready=true`。`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 155：Manifest managedBlock 三要素显式化

状态：已完成。

目标：继续 Stage 7 pack-neutral hardening，将 Go manifest loader 中对缺失 `managedBlock.file`、`blockId` 与 `source` 的隐式默认值注入移出 runtime，避免新增 pack 静默继承 `CLAUDE.local.md`、`rekit:router` 或 `CLAUDE.local.snippet.md`；本批不改既有 pack manifest 内容，只让 manifest contract 更显式。

实施范围：

- 移除 `manifest.Load` 对缺失 `ManagedBlock["file"]`、`ManagedBlock["blockId"]` 与 `ManagedBlock["source"]` 的默认值注入。
- 保留并强化 `ValidateSchema` 的 `managedBlock is missing required key` 显式检查，确保 schema-valid pack 必须声明 managed block host、block id 与 source。
- 在 Go sync review、init preview 与 apply/preview 入口使用 managed block 前补 `ValidateSchema`，让缺失 managedBlock 的 manifest 返回 schema 诊断而不是空路径错误。
- 更新 sync tests fixture，使其满足当前显式 manifest contract，并覆盖 sync apply/preview 仍按 managed block 边界运行。
- `rekit/schemas/pack-manifest.schema.yml` 说明 managedBlock 必须显式声明 file/blockId/source，runtime 不再注入默认值。
- `docs/pack-authoring.md` 明确新增 pack 不能依赖 managedBlock 默认 host、blockId 或 source。
- manifest tests 新增 load/no-fallback drift test，确认缺失 `managedBlock.source` 时 loader 不再补 `CLAUDE.local.snippet.md`。
- 更新 `CHANGELOG.md`、`docs/go-first-convergence-plan.md` 与本 batch-plan。

边界：本批只做 pack-neutral manifest contract、测试和文档；不改变既有 pack manifest 内容、不改变 sync/promote apply 行为、不改变 PowerShell façade 委托集合、不运行大型 PowerShell matrix、不执行 heavy-tool、不创建或修改真实 case state、不写 authority/confirmed。

验证计划：

```powershell
go test ./internal/rekit/manifest ./internal/rekit/sync ./internal/rekit/doctor ./internal/rekit/promote ./internal/rekit/cli ./internal/rekit/releasecheck
go test ./...
go vet ./...
go run ./cmd/rekit -- -Command release-check -Format json
.\rekit\rekit.ps1 -Command doctor
.\rekit\tests\facade-smoke.ps1
git diff --check
```

验证结果：已通过 targeted `go test ./internal/rekit/manifest ./internal/rekit/sync ./internal/rekit/doctor ./internal/rekit/promote ./internal/rekit/cli ./internal/rekit/releasecheck`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`/rekit doctor` 与 `facade-smoke.ps1`；`release-check` 输出 `ready=true`。`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 156：Manifest identity 字段显式化

状态：已完成。

目标：继续 Stage 7 pack-neutral hardening，将 Go manifest loader 中对缺失 `name` 与 `version` 的隐式默认值移出 runtime，避免新增 pack 静默继承 pack id 或 `0.0.0` 版本；本批不改既有 pack manifest 内容，只让 manifest identity contract 更显式。

实施范围：

- 移除 `manifest.Load` 对缺失 `name` / `version` 的 fallback，缺失时保持空值。
- 在 `ValidateSchema` 中要求 `name` 与 `version` 显式声明，错误分别为 `name is missing` 与 `version is missing`。
- 更新 manifest tests fixture，并新增 schema validation 与 load/no-fallback drift test，确认缺失 identity 时 loader 不再补 pack id 或 `0.0.0`。
- `rekit/schemas/pack-manifest.schema.yml` 说明 `name` / `version` 必须显式声明且 runtime 不再补默认值。
- `docs/pack-authoring.md` 明确新增 pack 不能依赖 runtime 用 pack id 或 `0.0.0` 补齐 identity。
- 更新 `CHANGELOG.md`、`docs/go-first-convergence-plan.md` 与本 batch-plan。

边界：本批只做 pack-neutral manifest contract、测试和文档；不改变既有 pack manifest 内容、不改变 PowerShell façade 委托集合、不运行大型 PowerShell matrix、不执行 heavy-tool、不创建或修改真实 case state、不写 authority/confirmed。

验证计划：

```powershell
go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/promote ./internal/rekit/cli ./internal/rekit/releasecheck
go test ./...
go vet ./...
go run ./cmd/rekit -- -Command release-check -Format json
.\rekit\rekit.ps1 -Command doctor
.\rekit\tests\facade-smoke.ps1
git diff --check
```

验证结果：已通过 targeted `go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/promote ./internal/rekit/cli ./internal/rekit/releasecheck`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`/rekit doctor` 与 `facade-smoke.ps1`；`release-check` 输出 `ready=true`。初次 targeted run 在本节仍标记进行中且缺验证结果时触发 release handoff 文档门禁，按门禁补齐状态与验证结果后通过。`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 157：Manifest description 字段显式化

状态：已完成。

目标：继续 Stage 7 pack-neutral hardening，将 manifest `description` 从可空展示字段收紧为 schema-valid pack 必须显式声明的一行用途摘要，避免新增 pack 缺少用途说明却进入 release inventory；本批不改既有 pack manifest 内容，只锁定 description metadata contract。

实施范围：

- 在 `ValidateSchema` 中要求 `description` 显式声明，错误为 `description is missing`。
- 更新 manifest tests fixture，并新增 schema validation 与 load/no-fallback drift test，确认缺失 description 时保持空值并由 schema validation 诊断。
- 将 `description` 加入 `rekit/schemas/pack-manifest.schema.yml` required 列表，并说明 schema-valid pack 不允许缺用途摘要。
- 更新 `docs/pack-authoring.md`，明确新增 pack 必须替换并显式声明 description。
- 更新 `CHANGELOG.md`、`docs/go-first-convergence-plan.md` 与本 batch-plan。

边界：本批只做 pack-neutral manifest contract、测试和文档；不改变既有 pack manifest 内容、不改变 PowerShell façade 委托集合、不运行大型 PowerShell matrix、不执行 heavy-tool、不创建或修改真实 case state、不写 authority/confirmed。

验证计划：

```powershell
go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/promote ./internal/rekit/cli ./internal/rekit/releasecheck
go test ./...
go vet ./...
go run ./cmd/rekit -- -Command release-check -Format json
.\rekit\rekit.ps1 -Command doctor
.\rekit\tests\facade-smoke.ps1
git diff --check
```

验证结果：已通过 targeted `go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/promote ./internal/rekit/cli ./internal/rekit/releasecheck`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`/rekit doctor` 与 `facade-smoke.ps1`；`release-check` 输出 `ready=true`。初次 targeted run 在本节仍标记进行中且缺验证结果时触发 release handoff 文档门禁，按门禁补齐状态与验证结果后通过。`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 158：Manifest lane authority 字段显式化

状态：已完成。

目标：继续 Stage 7 pack-neutral hardening，将 `laneTypes.authority` 从宽松 bool 解析收紧为 schema-valid pack 必须显式声明 `true` 或 `false` 的工作线边界，避免新增 pack 缺失或拼错 authority 时被 Go parser 静默当作 false；本批不改既有 pack manifest 内容。

实施范围：

- Go manifest parser 为 `LaneType` 保留原始 `authority` 文本，供 schema validation 区分“显式 false”和“缺失/非法”。
- `ValidateSchema` 要求每个 lane 显式声明 `authority`，且值必须为 `true` 或 `false`。
- manifest tests 补 schema validation 与 load/no-fallback drift test，确认缺失 authority 不再静默成为 schema-valid false lane。
- `rekit/schemas/pack-manifest.schema.yml` 说明 `laneTypes.authority` 必须显式声明 true/false，runtime 不再把缺失或非法 authority 当作 false。
- `docs/pack-authoring.md` 明确新增 pack 的 laneTypes 必须显式声明 authority true/false。
- 更新 `CHANGELOG.md`、`docs/go-first-convergence-plan.md` 与本 batch-plan。

边界：本批只做 pack-neutral manifest contract、测试和文档；不改变既有 pack manifest 内容、不改变 PowerShell façade 委托集合、不运行大型 PowerShell matrix、不执行 heavy-tool、不创建或修改真实 case state、不写 authority/confirmed。

验证计划：

```powershell
go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/promote ./internal/rekit/cli ./internal/rekit/releasecheck
go test ./...
go vet ./...
go run ./cmd/rekit -- -Command release-check -Format json
.\rekit\rekit.ps1 -Command doctor
.\rekit\tests\facade-smoke.ps1
git diff --check
```

验证结果：已通过 targeted `go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/promote ./internal/rekit/cli ./internal/rekit/releasecheck`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`/rekit doctor` 与 `facade-smoke.ps1`；`release-check` 输出 `ready=true`。`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 159：Manifest heavyToolGates requiresConfirmation 字段显式化

状态：已完成。

目标：继续 Stage 7 pack-neutral hardening，将 `heavyToolGates.requiresConfirmation` 从宽松 bool 解析收紧为 schema-valid pack 必须显式声明 `true` 的 heavy-tool 确认门禁，避免新增 pack 缺失或拼错 confirmation gate 时被 Go parser 静默当作 false；本批不改既有 pack manifest 内容。

实施范围：

- Go manifest parser 为 `HeavyToolGate` 保留原始 `requiresConfirmation` 文本，供 schema validation 区分“显式 true”和“缺失/非法/false”。
- `ValidateSchema` 要求每个 heavy-tool gate 显式声明 `requiresConfirmation`，且值必须为 `true`。
- manifest tests 补 schema validation 与 load/no-fallback drift test，确认缺失 requiresConfirmation 不再静默成为 schema-valid gate。
- `rekit/schemas/pack-manifest.schema.yml` 说明 `requiresConfirmation` 必须显式声明 true，runtime 不再把缺失或非法值当作 false。
- `docs/pack-authoring.md` 明确新增 pack 的每个 heavy tool gate 必须显式声明 `requiresConfirmation: true`。
- 更新 `CHANGELOG.md`、`docs/go-first-convergence-plan.md` 与本 batch-plan。

边界：本批只做 pack-neutral manifest contract、测试和文档；不改变既有 pack manifest 内容、不改变 PowerShell façade 委托集合、不运行大型 PowerShell matrix、不执行 heavy-tool、不创建或修改真实 case state、不写 authority/confirmed。

验证计划：

```powershell
go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/promote ./internal/rekit/cli ./internal/rekit/releasecheck
go test ./...
go vet ./...
go run ./cmd/rekit -- -Command release-check -Format json
.\rekit\rekit.ps1 -Command doctor
.\rekit\tests\facade-smoke.ps1
git diff --check
```

验证结果：已通过 targeted `go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/promote ./internal/rekit/cli ./internal/rekit/releasecheck`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`/rekit doctor` 与 `facade-smoke.ps1`；`release-check` 输出 `ready=true`。`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 160：Manifest required list presence 显式化

状态：已完成。

目标：继续 Stage 7 pack-neutral hardening，将 `managedFiles`、`templateFiles` 与 `localNeverOverwrite` 从 loader 默认空 slice / 非空检查收紧为 schema-valid pack 必须显式声明的 required list key；允许空列表，但不能缺 key，避免新增 pack 的 sync/promote/local 边界被 runtime 隐式推断。

实施范围：

- Go manifest parser 记录 required list key 的显式 presence，并移除 load 阶段对空 `managedFiles` 的 hard error，让缺 key 统一由 `ValidateSchema` 诊断。
- `ValidateSchema` 要求 `managedFiles`、`templateFiles` 与 `localNeverOverwrite` 都显式声明；空列表保持合法输入，只表示 pack 有意不声明该类文件。
- manifest tests 补 required-list schema validation、load/no-fallback drift test，并对既有 text fixtures 补显式空列表，避免遮挡原有 fallback 断言。
- promote package tests 的临时 manifest fixture 补齐 required list key，保持 promote apply validation 仍覆盖 pack source 写入/restore 边界。
- `rekit/schemas/pack-manifest.schema.yml`、`docs/pack-authoring.md` 与 `docs/go-first-convergence-plan.md` 明确 required list 必须显式声明。
- 更新 `CHANGELOG.md` 与本 batch-plan。

边界：本批只做 pack-neutral manifest contract、测试和文档；不改变既有 pack manifest 内容、不改变 PowerShell façade 委托集合、不运行大型 PowerShell matrix、不执行 heavy-tool、不创建或修改真实 case state、不写 authority/confirmed。

验证计划：

```powershell
go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/promote ./internal/rekit/cli ./internal/rekit/releasecheck
go test ./...
go vet ./...
go run ./cmd/rekit -- -Command release-check -Format json
.\rekit\rekit.ps1 -Command doctor
.\rekit\tests\facade-smoke.ps1
git diff --check
```

验证结果：已通过 targeted `go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/promote ./internal/rekit/cli ./internal/rekit/releasecheck`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`/rekit doctor` 与 `facade-smoke.ps1`；`release-check` 输出 `ready=true`。`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 161：Manifest schemaVersion 显式化

状态：已完成。

目标：继续 Stage 7 pack-neutral hardening，将 `schemaVersion` 纳入 Go manifest schema validation 的显式 contract，要求 schema-valid pack 必须声明 `schemaVersion: 1`，避免缺失或未支持版本的 manifest 被 runtime 当作默认版本静默接受。

实施范围：

- Go manifest parser 保留 `schemaVersion` 字段原始文本，不在 loader 阶段注入默认版本。
- `ValidateSchema` 要求 `schemaVersion` 显式声明且当前必须等于 `1`；缺失和未支持版本分别给出明确错误。
- manifest tests 补 schema validation 与 load/no-fallback drift test，确认缺失 `schemaVersion` 不再静默进入 schema-valid manifest。
- `rekit/schemas/pack-manifest.schema.yml` 将 `schemaVersion` 加入 required，并说明仅支持显式 `1`。
- `docs/pack-authoring.md`、`docs/go-first-convergence-plan.md` 与 `CHANGELOG.md` 同步记录新 pack authoring contract。

边界：本批只做 pack-neutral manifest contract、测试和文档；不改变既有 pack manifest 内容、不改变 PowerShell façade 委托集合、不运行大型 PowerShell matrix、不执行 heavy-tool、不创建或修改真实 case state、不写 authority/confirmed。

验证计划：

```powershell
go test ./internal/rekit/manifest ./internal/rekit/gate ./internal/rekit/promote ./internal/rekit/sync ./internal/rekit/cli ./internal/rekit/releasecheck
go test ./...
go vet ./...
go run ./cmd/rekit -- -Command release-check -Format json
.\rekit\rekit.ps1 -Command doctor
.\rekit\tests\facade-smoke.ps1
git diff --check
```

验证结果：已通过 targeted `go test ./internal/rekit/manifest ./internal/rekit/gate ./internal/rekit/promote ./internal/rekit/sync ./internal/rekit/cli ./internal/rekit/releasecheck`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`/rekit doctor` 与 `facade-smoke.ps1`；`release-check` 输出 `ready=true`。`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 162：Manifest schemaVersion inventory

状态：已完成。

目标：继续 Stage 7 pack-neutral hardening，将 Batch 161 的 manifest `schemaVersion` contract 暴露到 machine-readable inventory 与 release handoff，使 release gate 不只验证 schema，也能审计 pack manifest schema version readiness。

实施范围：

- `manifest.PackSummary` 增加 `schemaVersion`，`Summary()` 保留 manifest 原始 schema version，`/rekit packs -Format json` 与 `release-check -Format json` 的 `packs[]` 继承该字段。
- `/rekit packs` table 增加 `manifestSchema` 列，`/rekit status -Format json` 的 manifest summary 增加 `schemaVersion`，便于本机和 CI 直接检查当前 pack manifest schema contract。
- `releaseHandoff.packMaturity` 增加 `schemaVersionReady`，每 pack gate status 增加 `schemaVersion`；`pack maturity summary` signal 将 schema version readiness 纳入 ready 条件和 details。
- CLI / manifest / releasecheck tests 覆盖 schemaVersion inventory、table/JSON 输出、release handoff drift detection 与 missing schema version warning。
- `docs/go-first-convergence-plan.md`、`docs/release-readiness.md` 与 `CHANGELOG.md` 同步记录 machine-readable readiness 扩展。

边界：本批只做 pack-neutral manifest inventory、release handoff gate、测试和文档；不改变既有 pack manifest 内容、不改变 PowerShell façade 委托集合、不运行大型 PowerShell matrix、不执行 heavy-tool、不创建或修改真实 case state、不写 authority/confirmed。

验证计划：

```powershell
go test ./internal/rekit/manifest ./internal/rekit/cli ./internal/rekit/releasecheck
go test ./...
go vet ./...
go run ./cmd/rekit -- -Command release-check -Format json
.\rekit\rekit.ps1 -Command doctor
.\rekit\tests\facade-smoke.ps1
git diff --check
```

验证结果：已通过 targeted `go test ./internal/rekit/manifest ./internal/rekit/cli ./internal/rekit/releasecheck`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`/rekit doctor` 与 `facade-smoke.ps1`；`release-check` 输出 `ready=true`。`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 163：Manifest syncPolicy 显式化

状态：已完成。

目标：继续 Stage 7 pack-neutral hardening，将 `syncPolicy` 从只检查当前 map 值收紧为显式 manifest contract，避免新增 pack 缺失 sync 写入策略时被 Go loader 的空 map 状态或泛化 unsupported-value 诊断掩盖。

实施范围：

- Go manifest parser 记录 `syncPolicy` map key 是否在 manifest 中显式声明，不在 loader 阶段补默认策略。
- `ValidateSchema` 新增 `validateSyncPolicy`，要求 `syncPolicy` 显式存在，且 `managedFiles=overwrite-with-backup`、`templateFiles=create-if-missing`、`localFiles=never-overwrite` 三项都必须显式声明。
- unsupported value 诊断从泛化 `syncPolicy has unsupported value` 收紧为 `syncPolicy.<key> has unsupported value: <value>`，便于新增 pack authoring 定位具体策略漂移。
- manifest tests 覆盖缺失 map、缺失 key、未支持值与 load/no-fallback drift，确认缺失 `syncPolicy` 不会被 runtime 补默认策略。
- `docs/pack-authoring.md`、`rekit/schemas/pack-manifest.schema.yml`、`docs/go-first-convergence-plan.md` 与 `CHANGELOG.md` 同步记录 syncPolicy contract。

边界：本批只做 pack-neutral manifest contract、测试和文档；不改变既有 pack manifest 内容、不改变 PowerShell façade 委托集合、不运行大型 PowerShell matrix、不执行 heavy-tool、不创建或修改真实 case state、不写 authority/confirmed。

验证计划：

```powershell
go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/sync ./internal/rekit/cli ./internal/rekit/releasecheck
go test ./...
go vet ./...
go run ./cmd/rekit -- -Command release-check -Format json
.\rekit\rekit.ps1 -Command doctor
.\rekit\tests\facade-smoke.ps1
git diff --check
```

验证结果：已通过 targeted `go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/sync ./internal/rekit/cli ./internal/rekit/releasecheck`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`/rekit doctor` 与 `facade-smoke.ps1`；`release-check` 输出 `ready=true`。`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 164：Manifest runtime maps 显式化

状态：已完成。

目标：继续 Stage 7 pack-neutral hardening，将 `workstreamDefaults` 与 `budgets` 从仅检查子 key 收紧为显式 runtime map contract，避免新增 pack 缺失工作线默认路由、backup root 或预算 map 时被空 map 与运行时安全 fallback 掩盖。

实施范围：

- Go manifest parser 将 `workstreamDefaults` 与 `budgets` 加入显式 map presence 记录，不在 loader 阶段补默认 map。
- `ValidateSchema` 要求 `workstreamDefaults` map 本身显式存在，再检查 defaultAuthorityLane、defaultStartLaneType、backupRoot、requestDefaultTargetLane 等必需子 key；缺 map 与缺 key 分别给出明确诊断。
- `validateBudgets` 要求 `budgets` map 本身显式存在，再检查 `defaultMarkdown` 正整数；`BudgetLimit` 保留调用方 runtime safety fallback，但不再代表 schema-valid manifest contract。
- manifest tests 覆盖缺失 `workstreamDefaults` map、缺失 workstream 子 key、缺失 `budgets` map、缺失 defaultMarkdown 与 load/no-fallback drift。
- `docs/pack-authoring.md`、`rekit/schemas/pack-manifest.schema.yml`、`docs/go-first-convergence-plan.md` 与 `CHANGELOG.md` 同步记录 runtime maps contract。

边界：本批只做 pack-neutral manifest contract、测试和文档；不改变既有 pack manifest 内容、不改变 PowerShell façade 委托集合、不运行大型 PowerShell matrix、不执行 heavy-tool、不创建或修改真实 case state、不写 authority/confirmed。

验证计划：

```powershell
go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/sync ./internal/rekit/cli ./internal/rekit/releasecheck
go test ./...
go vet ./...
go run ./cmd/rekit -- -Command release-check -Format json
.\rekit\rekit.ps1 -Command doctor
.\rekit\tests\facade-smoke.ps1
git diff --check
```

验证结果：已通过 targeted `go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/sync ./internal/rekit/cli ./internal/rekit/releasecheck`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`/rekit doctor` 与 `facade-smoke.ps1`；`release-check` 输出 `ready=true`。`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 165：Manifest required list presence inventory

状态：已完成。

目标：继续 Stage 7 pack-neutral hardening，将更多 schema-critical list 字段纳入 manifest presence contract，区分缺 key 与显式空列表，避免新增 pack 通过 `len==0` 混合诊断掩盖作者意图。

实施范围：

- Go manifest parser 将 `promoteFiles`、`toolingCandidateSources`、`authorityFiles`、`promoteDenyPatterns`、`heavyToolGates` 与 `laneTypes` 加入显式 list presence 记录，和既有 `managedFiles`、`templateFiles`、`localNeverOverwrite` 一起检查。
- `ValidateSchema` 先报告缺失 list key，再对必须非空的 `promoteFiles`、`toolingCandidateSources`、`authorityFiles`、`promoteDenyPatterns`、`heavyToolGates` 与 `laneTypes` 给出非空诊断。
- manifest tests 覆盖新增 required list presence、显式空 contract list 与 `promoteFiles` 缺 key no-fallback drift，确认缺 key 与显式空列表不再共用同一错误路径。
- `docs/pack-authoring.md`、`rekit/schemas/pack-manifest.schema.yml`、`docs/go-first-convergence-plan.md` 与 `CHANGELOG.md` 同步记录 required list presence contract。

边界：本批只做 pack-neutral manifest contract、测试和文档；不改变既有 pack manifest 内容、不改变 PowerShell façade 委托集合、不运行大型 PowerShell matrix、不执行 heavy-tool、不创建或修改真实 case state、不写 authority/confirmed。

验证计划：

```powershell
go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/sync ./internal/rekit/cli ./internal/rekit/releasecheck
go test ./...
go vet ./...
go run ./cmd/rekit -- -Command release-check -Format json
.\rekit\rekit.ps1 -Command doctor
.\rekit\tests\facade-smoke.ps1
git diff --check
```

验证结果：已通过 targeted `go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/sync ./internal/rekit/cli ./internal/rekit/releasecheck`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`/rekit doctor` 与 `facade-smoke.ps1`；`release-check` 输出 `ready=true`。`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 166：Manifest optional route/tooling list presence

状态：已完成。

目标：继续 Stage 7 pack-neutral hardening，将可为空但影响 doctor、route inventory、tooling/prompt 校验的 optional list 字段纳入 manifest presence contract，避免新增 pack 通过 loader 空 slice 隐式省略边界。

实施范围：

- Go manifest parser 将 `commonPolicies`、`policyOverlays`、`subagentRoutes`、`toolingFiles` 与 `promptFiles` 加入统一 `manifestListPresenceKeys`，和既有 required/contract list 一起记录显式 presence。
- `ValidateSchema` 使用同一 key 集合检查 list presence；可为空 list 仍允许 `[]`，但缺 key 会报告 `manifest must explicitly declare <key>`。
- manifest tests 用统一 key 集合覆盖 list presence，并新增 optional list no-fallback drift test，确认缺失 `commonPolicies` 不会被空 slice 视作显式声明。
- `docs/pack-authoring.md`、`rekit/schemas/pack-manifest.schema.yml`、`docs/go-first-convergence-plan.md` 与 `CHANGELOG.md` 同步记录 optional route/tooling list presence contract。

边界：本批只做 pack-neutral manifest contract、测试和文档；不改变既有 pack manifest 内容、不改变 PowerShell façade 委托集合、不运行大型 PowerShell matrix、不执行 heavy-tool、不创建或修改真实 case state、不写 authority/confirmed。

验证计划：

```powershell
go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/sync ./internal/rekit/cli ./internal/rekit/releasecheck
go test ./...
go vet ./...
go run ./cmd/rekit -- -Command release-check -Format json
.\rekit\rekit.ps1 -Command doctor
.\rekit\tests\facade-smoke.ps1
git diff --check
```

验证结果：已通过 targeted `go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/sync ./internal/rekit/cli ./internal/rekit/releasecheck ./internal/rekit/promote`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`/rekit doctor` 与 `facade-smoke.ps1`；`release-check` 输出 `ready=true`。`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 167：Manifest subagent route trigger contract

状态：已完成。

目标：继续 Stage 7 pack-neutral hardening，收紧 `subagentRoutes` item-level contract，要求每个 route 显式说明触发条件，避免新增 pack 只声明任务类型与 shard 维度但缺少触发语义仍进入 schema-valid inventory。

实施范围：

- `ValidateSchema` 在校验 `subagentRoutes` 时要求 `trigger` 显式非空，并对缺失项返回 `subagent route <id> is missing trigger`。
- manifest tests 新增 route trigger 缺失/有效场景，确保 trigger 缺失被 schema validation 捕获。
- `docs/pack-authoring.md`、`rekit/schemas/pack-manifest.schema.yml`、`docs/go-first-convergence-plan.md` 与 `CHANGELOG.md` 同步记录 route trigger contract。

边界：本批只做 pack-neutral manifest contract、测试和文档；不改变既有 pack manifest 内容、不改变 PowerShell façade 委托集合、不运行大型 PowerShell matrix、不执行 heavy-tool、不创建或修改真实 case state、不写 authority/confirmed。

验证计划：

```powershell
go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/sync ./internal/rekit/cli ./internal/rekit/releasecheck
go test ./...
go vet ./...
go run ./cmd/rekit -- -Command release-check -Format json
.\rekit\rekit.ps1 -Command doctor
.\rekit\tests\facade-smoke.ps1
git diff --check
```

验证结果：已通过 targeted `go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/sync ./internal/rekit/cli ./internal/rekit/releasecheck`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`/rekit doctor` 与 `facade-smoke.ps1`；`release-check` 输出 `ready=true`。`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 168：Manifest subagent route id contract

状态：已完成。

目标：继续 Stage 7 pack-neutral hardening，收紧 `subagentRoutes` item-level identity contract，要求 route id 带 pack namespace，避免跨 pack route inventory、review packet 与 plan-subagents 输出无法稳定追踪来源。

实施范围：

- `ValidateSchema` 在校验 `subagentRoutes` 时要求 `id` 匹配 namespaced route 格式（例如 `<pack>:<route>`），并对无命名空间 id 返回 `subagent route has invalid id: <id>`。
- manifest tests 新增 route id 缺命名空间/有效场景，确保 route id contract 被 schema validation 捕获。
- `docs/pack-authoring.md`、`rekit/schemas/pack-manifest.schema.yml`、`docs/go-first-convergence-plan.md` 与 `CHANGELOG.md` 同步记录 route id contract。

边界：本批只做 pack-neutral manifest contract、测试和文档；不改变既有 pack manifest 内容、不改变 PowerShell façade 委托集合、不运行大型 PowerShell matrix、不执行 heavy-tool、不创建或修改真实 case state、不写 authority/confirmed。

验证计划：

```powershell
go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/sync ./internal/rekit/cli ./internal/rekit/releasecheck
go test ./...
go vet ./...
go run ./cmd/rekit -- -Command release-check -Format json
.\rekit\rekit.ps1 -Command doctor
.\rekit\tests\facade-smoke.ps1
git diff --check
```

验证结果：已通过 targeted `go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/sync ./internal/rekit/cli ./internal/rekit/releasecheck`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`/rekit doctor` 与 `facade-smoke.ps1`；`release-check` 输出 `ready=true`。`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 169：Manifest subagent route list-field contract

状态：已完成。

目标：继续 Stage 7 pack-neutral hardening，收紧 `subagentRoutes` 中逗号/分号分隔字段的可消费性，避免空 token、尾随分隔符或仅分隔符内容进入 schema-valid route contract。

实施范围：

- `ValidateSchema` 新增 `validateSubagentRouteListField`，校验 `taskTypes`、`mainAgentOwns` 与 `outputContract` 非空且不含空列表项。
- manifest tests 覆盖 `taskTypes`、`mainAgentOwns` 与 `outputContract` 的空项、尾随分隔符和分号分隔空项场景。
- `docs/pack-authoring.md`、`rekit/schemas/pack-manifest.schema.yml`、`docs/go-first-convergence-plan.md` 与 `CHANGELOG.md` 同步记录 route list-field contract。

边界：本批只做 pack-neutral manifest contract、测试和文档；不改变既有 pack manifest 内容、不改变 PowerShell façade 委托集合、不运行大型 PowerShell matrix、不执行 heavy-tool、不创建或修改真实 case state、不写 authority/confirmed。

验证计划：

```powershell
go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/sync ./internal/rekit/cli ./internal/rekit/releasecheck
go test ./...
go vet ./...
go run ./cmd/rekit -- -Command release-check -Format json
.\rekit\rekit.ps1 -Command doctor
.\rekit\tests\facade-smoke.ps1
git diff --check
```

验证结果：已通过 targeted `go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/sync ./internal/rekit/cli ./internal/rekit/releasecheck`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`/rekit doctor` 与 `facade-smoke.ps1`；`release-check` 输出 `ready=true`。`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 170：Manifest subagent route namespace ownership

状态：已完成。

目标：继续 Stage 7 pack-neutral hardening，收紧 `subagentRoutes.id` namespace ownership，要求 route id 前缀匹配当前 pack id，避免复制 `_template` 或其它 pack route id 后仍进入 schema-valid inventory、review packet 或 plan-subagents 输出。

实施范围：

- `ValidateSchema` 在 namespaced route id 格式校验之后检查 route namespace 与当前 pack id 一致，并对跨 pack namespace 返回 `subagent route id <id> must use pack namespace <pack>`。
- manifest tests 新增 route namespace mismatch / valid namespace 场景，确保 schema validation 能捕获复制模板残留或跨 pack route id。
- `docs/pack-authoring.md`、`rekit/schemas/pack-manifest.schema.yml`、`docs/go-first-convergence-plan.md` 与 `CHANGELOG.md` 同步记录 route namespace ownership contract。

边界：本批只做 pack-neutral manifest contract、测试和文档；不改变既有 pack manifest 内容、不改变 PowerShell façade 委托集合、不运行大型 PowerShell matrix、不执行 heavy-tool、不创建或修改真实 case state、不写 authority/confirmed。

验证计划：

```powershell
go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/sync ./internal/rekit/cli ./internal/rekit/releasecheck
go test ./...
go vet ./...
go run ./cmd/rekit -- -Command release-check -Format json
.\rekit\rekit.ps1 -Command doctor
.\rekit\tests\facade-smoke.ps1
git diff --check
```

验证结果：已通过 targeted `go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/sync ./internal/rekit/cli ./internal/rekit/releasecheck`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`/rekit doctor` 与 `facade-smoke.ps1`；`release-check` 输出 `ready=true`。`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 171：Manifest subagent route permissions contract

状态：已完成。

目标：继续 Stage 7 pack-neutral hardening，收紧 `subagentRoutes.subagentPermissions` contract，要求 route permissions 使用当前 runtime 支持的显式值，避免未知权限字符串进入 schema-valid route contract 后被 plan-subagents 或 reviewer packet 误解。

实施范围：

- `ValidateSchema` 新增 `validateSubagentPermissions`，要求 `subagentPermissions` 只能是 `read-only` 或 `read-only-or-workspace-only`，并对未知值返回 `subagent route <id> has unsupported subagentPermissions: <value>`。
- manifest tests 新增 unsupported / valid permissions 场景，确保 route permissions contract 被 schema validation 捕获。
- `docs/pack-authoring.md`、`rekit/schemas/pack-manifest.schema.yml`、`docs/go-first-convergence-plan.md` 与 `CHANGELOG.md` 同步记录 route permissions contract。

边界：本批只做 pack-neutral manifest contract、测试和文档；不改变既有 pack manifest 内容、不改变 PowerShell façade 委托集合、不运行大型 PowerShell matrix、不执行 heavy-tool、不创建或修改真实 case state、不写 authority/confirmed。

验证计划：

```powershell
go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/sync ./internal/rekit/cli ./internal/rekit/releasecheck
go test ./...
go vet ./...
go run ./cmd/rekit -- -Command release-check -Format json
.\rekit\rekit.ps1 -Command doctor
.\rekit\tests\facade-smoke.ps1
git diff --check
```

验证结果：已通过 targeted `go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/sync ./internal/rekit/cli ./internal/rekit/releasecheck`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`/rekit doctor` 与 `facade-smoke.ps1`；`release-check` 输出 `ready=true`。`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 172：Manifest subagent route policy overlay contract

状态：已完成。

目标：继续 Stage 7 pack-neutral hardening，收紧 `subagentRoutes.policyOverlay` contract，要求非空 route policy overlay 必须来自当前 manifest 显式声明的 `policyOverlays` 列表，避免 route 引用未登记 pack overlay 或复制残留文件后仍进入 schema-valid route contract。

实施范围：

- `ValidateSchema` 为 `policyOverlays` 建立显式声明集合，并要求非空 `subagentRoutes.policyOverlay` 必须来自该集合；未声明时返回 `subagent route <id> policyOverlay is not declared in policyOverlays: <path>`。
- manifest tests 新增 undeclared / declared policy overlay 场景，确保 route overlay contract 被 schema validation 捕获。
- `docs/pack-authoring.md`、`rekit/schemas/pack-manifest.schema.yml`、`docs/go-first-convergence-plan.md` 与 `CHANGELOG.md` 同步记录 route policy overlay contract。

边界：本批只做 pack-neutral manifest contract、测试和文档；不改变既有 pack manifest 内容、不改变 PowerShell façade 委托集合、不运行大型 PowerShell matrix、不执行 heavy-tool、不创建或修改真实 case state、不写 authority/confirmed。

验证计划：

```powershell
go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/sync ./internal/rekit/cli ./internal/rekit/releasecheck
go test ./...
go vet ./...
go run ./cmd/rekit -- -Command release-check -Format json
.\rekit\rekit.ps1 -Command doctor
.\rekit\tests\facade-smoke.ps1
git diff --check
```

验证结果：已通过 targeted `go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/sync ./internal/rekit/cli ./internal/rekit/releasecheck`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`/rekit doctor` 与 `facade-smoke.ps1`；`release-check` 输出 `ready=true`。`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 173：Manifest subagent route shard contract

状态：已完成。

目标：继续 Stage 7 pack-neutral hardening，收紧 `subagentRoutes.shardBasis` contract，要求 route shard basis 使用机器可消费的小写 slug 或 `-or-` 分隔组合，避免空分片项、尾随 `-or-` 或非法字符进入 schema-valid route contract。

实施范围：

- `ValidateSchema` 新增 `validateSubagentShardBasis`，要求 `shardBasis` 非空且每个 `-or-` 分隔 segment 都是小写 slug；非法值返回 `subagent route <id> has invalid shardBasis: <value>`。
- manifest tests 新增 invalid / valid shard basis 场景，确保 route shard contract 被 schema validation 捕获。
- `docs/pack-authoring.md`、`rekit/schemas/pack-manifest.schema.yml`、`docs/go-first-convergence-plan.md` 与 `CHANGELOG.md` 同步记录 route shard contract。

边界：本批只做 pack-neutral manifest contract、测试和文档；不改变既有 pack manifest 内容、不改变 PowerShell façade 委托集合、不运行大型 PowerShell matrix、不执行 heavy-tool、不创建或修改真实 case state、不写 authority/confirmed。

验证计划：

```powershell
go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/sync ./internal/rekit/cli ./internal/rekit/releasecheck
go test ./...
go vet ./...
go run ./cmd/rekit -- -Command release-check -Format json
.\rekit\rekit.ps1 -Command doctor
.\rekit\tests\facade-smoke.ps1
git diff --check
```

验证结果：已通过 targeted `go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/sync ./internal/rekit/cli ./internal/rekit/releasecheck`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`/rekit doctor` 与 `facade-smoke.ps1`；`release-check` 输出 `ready=true`。`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 174：Manifest subagent route list token contract

状态：已完成。

目标：继续 Stage 7 pack-neutral hardening，收紧 `subagentRoutes.taskTypes`、`mainAgentOwns` 与 `outputContract` 的列表 token contract，要求每个 token 使用机器可消费的小写 slug/snake token，避免空项之外的空格、大写或非法字符进入 schema-valid route contract。

实施范围：

- `ValidateSchema` 扩展 `validateSubagentRouteListField`，在拒绝空项之外要求每个 token 匹配小写 slug/snake token；非法值返回 `subagent route <id> has invalid <field> item: <token>`。
- manifest tests 新增 `taskTypes`、`mainAgentOwns` 与 `outputContract` 的非法 token 场景，继续保留空项回归覆盖。
- `docs/pack-authoring.md`、`rekit/schemas/pack-manifest.schema.yml`、`docs/go-first-convergence-plan.md` 与 `CHANGELOG.md` 同步记录 route list token contract。

边界：本批只做 pack-neutral manifest contract、测试和文档；不改变既有 pack manifest 内容、不改变 PowerShell façade 委托集合、不运行大型 PowerShell matrix、不执行 heavy-tool、不创建或修改真实 case state、不写 authority/confirmed。

验证计划：

```powershell
go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/sync ./internal/rekit/cli ./internal/rekit/releasecheck
go test ./...
go vet ./...
go run ./cmd/rekit -- -Command release-check -Format json
.\rekit\rekit.ps1 -Command doctor
.\rekit\tests\facade-smoke.ps1
git diff --check
```

验证结果：已通过 targeted `go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/sync ./internal/rekit/cli ./internal/rekit/releasecheck`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`/rekit doctor` 与 `facade-smoke.ps1`；`release-check` 输出 `ready=true`。`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 175：Manifest subagent route numeric bounds contract

状态：已完成。

目标：继续 Stage 7 pack-neutral hardening，收紧 `subagentRoutes.targetItemsPerAgent` 与 `maxParallel` 的数值边界，要求 route 默认分片大小和并发上限保持在当前 runtime 可安全消费的范围内，避免极端值进入 schema-valid route contract。

实施范围：

- `ValidateSchema` 新增 `validateSubagentRoutePositiveInt`，要求 `targetItemsPerAgent` 是 1-64 的正整数、`maxParallel` 是 1-16 的正整数；超出范围时返回 `subagent route <id> has <field> above supported maximum <n>: <value>`。
- manifest tests 新增 `targetItemsPerAgent` 与 `maxParallel` 的上界回归场景，同时保留原有非正整数诊断。
- `docs/pack-authoring.md`、`rekit/schemas/pack-manifest.schema.yml`、`docs/go-first-convergence-plan.md` 与 `CHANGELOG.md` 同步记录 route numeric bounds contract。

边界：本批只做 pack-neutral manifest contract、测试和文档；不改变既有 pack manifest 内容、不改变 PowerShell façade 委托集合、不运行大型 PowerShell matrix、不执行 heavy-tool、不创建或修改真实 case state、不写 authority/confirmed。

验证计划：

```powershell
go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/sync ./internal/rekit/cli ./internal/rekit/releasecheck
go test ./...
go vet ./...
go run ./cmd/rekit -- -Command release-check -Format json
.\rekit\rekit.ps1 -Command doctor
.\rekit\tests\facade-smoke.ps1
git diff --check
```

验证结果：已通过 targeted `go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/sync ./internal/rekit/cli ./internal/rekit/releasecheck`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`/rekit doctor` 与 `facade-smoke.ps1`；`release-check` 输出 `ready=true`。`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 176：Manifest policy/tooling/prompt list-item contract

状态：已完成。

目标：继续 Stage 7 pack-neutral hardening，收紧 `commonPolicies`、`policyOverlays`、`toolingFiles` 与 `promptFiles` 的列表项 contract，要求 policy id 与辅助文件路径保持机器可消费、可定位且去重，避免可选列表显式存在但携带不可消费 token/path。

实施范围：

- `ValidateSchema` 新增 `validateCommonPolicyIDs`，要求 `commonPolicies` 非空项是稳定小写 slug，且不能为空或重复。
- `ValidateSchema` 新增 pack/repo path list validation：`policyOverlays` 必须是 pack-relative `policies/*.overlay.md`，`toolingFiles` 必须位于 pack-relative `tooling/` 下，`promptFiles` 必须是 repo-relative `common/prompts/*.md` 或 `packs/<pack>/prompts/*.md`，并继续通过 `SourcePath` / `RepoPath` 保留 containment guard。
- manifest tests 新增 invalid policy id、overlay/tooling/prompt path 与 valid list entry 场景；`docs/pack-authoring.md`、`rekit/schemas/pack-manifest.schema.yml`、`docs/go-first-convergence-plan.md` 与 `CHANGELOG.md` 同步记录 policy/tooling/prompt list-item contract。

边界：本批只做 pack-neutral manifest contract、测试和文档；不改变既有 pack manifest 内容、不改变 PowerShell façade 委托集合、不运行大型 PowerShell matrix、不执行 heavy-tool、不创建或修改真实 case state、不写 authority/confirmed。

验证计划：

```powershell
go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/sync ./internal/rekit/cli ./internal/rekit/releasecheck
go test ./...
go vet ./...
go run ./cmd/rekit -- -Command release-check -Format json
.\rekit\rekit.ps1 -Command doctor
.\rekit\tests\facade-smoke.ps1
git diff --check
```

验证结果：已通过 targeted `go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/sync ./internal/rekit/cli ./internal/rekit/releasecheck`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`/rekit doctor` 与 `facade-smoke.ps1`；`release-check` 输出 `ready=true`。`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 177：Manifest heavy-tool gate list-item contract

状态：已完成。

目标：继续 Stage 7 pack-neutral hardening，收紧 `heavyToolGates.sideEffects` 与 `stopConditions` 的列表项 contract，要求 gate metadata 显式声明后仍保持可消费、去重且不会因空列表项被 YAML loader 静默吞掉。

实施范围：

- `splitScalarList` 保留逗号/分号分隔后的空项，使 `ValidateSchema` 能对 heavy-tool gate、lane 与 route 等 list 字段继续给出空项诊断，而不是在 parse 阶段静默丢弃。
- `ValidateSchema` 在 `validateHeavyToolGates` 中要求 `sideEffects` 使用小写 action/effect slug、包含 gate action id，并拒绝空项与重复项；`stopConditions` 拒绝空项与大小写无关重复项。
- manifest tests 新增 sideEffects/stopConditions 空项与重复项回归；`docs/pack-authoring.md`、`rekit/schemas/pack-manifest.schema.yml`、`docs/go-first-convergence-plan.md` 与 `CHANGELOG.md` 同步记录 heavy-tool gate list-item contract。

边界：本批只做 pack-neutral manifest contract、测试和文档；不改变既有 pack manifest 内容、不改变 PowerShell façade 委托集合、不运行大型 PowerShell matrix、不执行 heavy-tool、不创建或修改真实 case state、不写 authority/confirmed。

验证计划：

```powershell
go test ./internal/rekit/manifest ./internal/rekit/gate
go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/sync ./internal/rekit/cli ./internal/rekit/releasecheck
go test ./...
go vet ./...
go run ./cmd/rekit -- -Command release-check -Format json
.\rekit\rekit.ps1 -Command doctor
.\rekit\tests\facade-smoke.ps1
git diff --check
```

验证结果：已通过 targeted `go test ./internal/rekit/manifest ./internal/rekit/gate`、targeted `go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/sync ./internal/rekit/cli ./internal/rekit/releasecheck`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`/rekit doctor` 与 `facade-smoke.ps1`；`release-check` 输出 `ready=true`。`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 178：Manifest lane type list-item contract

状态：已完成。

目标：继续 Stage 7 pack-neutral hardening，收紧 `laneTypes` 的 id 与 list-item contract，要求工作线类型 id、可写/只读边界和输出事件列表在 schema-valid manifest 中保持可消费、可去重且不会携带空列表项。

实施范围：

- `ValidateSchema` 要求 `laneTypes.id` 使用小写 slug，避免带空格、大写或非法字符的 lane type 进入 `/rekit overview/continue/start/handoff` 可消费 inventory。
- `ValidateSchema` 对 `laneTypes.canWrite`、`readOnly` 与 `outputs` 增加空项与大小写无关重复项校验，并要求 `outputs` 使用机器可消费的小写 slug/snake token。
- manifest tests 新增 invalid lane id、重复 canWrite/readOnly、非法 outputs 与重复 outputs 回归；`docs/pack-authoring.md`、`rekit/schemas/pack-manifest.schema.yml`、`docs/go-first-convergence-plan.md` 与 `CHANGELOG.md` 同步记录 lane type list-item contract。

边界：本批只做 pack-neutral manifest contract、测试和文档；不改变既有 pack manifest 内容、不改变 PowerShell façade 委托集合、不运行大型 PowerShell matrix、不执行 heavy-tool、不创建或修改真实 case state、不写 authority/confirmed。

验证计划：

```powershell
go test ./internal/rekit/manifest
go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/sync ./internal/rekit/cli ./internal/rekit/releasecheck
go test ./...
go vet ./...
go run ./cmd/rekit -- -Command release-check -Format json
.\rekit\rekit.ps1 -Command doctor
.\rekit\tests\facade-smoke.ps1
git diff --check
```

验证结果：已通过 targeted `go test ./internal/rekit/manifest`、targeted `go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/sync ./internal/rekit/cli ./internal/rekit/releasecheck`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`/rekit doctor` 与 `facade-smoke.ps1`；`release-check` 输出 `ready=true`。`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。

### Batch 179：Manifest source/authority/deny list-item contract

状态：已完成。

目标：继续 Stage 7 pack-neutral hardening，收紧 `toolingCandidateSources`、`authorityFiles` 与 `promoteDenyPatterns` 的列表项 contract，要求 candidate source、authority allowlist 与 promote deny baseline 在 schema-valid manifest 中非空、可消费且去重。

实施范围：

- `ValidateSchema` 新增 source path list validation，要求 `toolingCandidateSources` 与 `authorityFiles` 的路径项非空、可通过 `SourcePath` 定位且不重复；authority allowlist 后续 writable 检查复用 trim 后路径，避免空格造成边界误判。
- `ValidateSchema` 要求 `promoteDenyPatterns` 非空、可编译且不重复，避免重复 deny baseline 或不可编译 pattern 进入 promote review / candidate filtering。
- manifest tests 新增 tooling source、authority file 与 deny pattern 的空项/重复项/非法 regex 回归；`docs/pack-authoring.md`、`rekit/schemas/pack-manifest.schema.yml`、`docs/go-first-convergence-plan.md` 与 `CHANGELOG.md` 同步记录 source/authority/deny list-item contract。

边界：本批只做 pack-neutral manifest contract、测试和文档；不改变既有 pack manifest 内容、不改变 PowerShell façade 委托集合、不运行大型 PowerShell matrix、不执行 heavy-tool、不创建或修改真实 case state、不写 authority/confirmed。

验证计划：

```powershell
go test ./internal/rekit/manifest
go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/sync ./internal/rekit/cli ./internal/rekit/releasecheck
go test ./...
go vet ./...
go run ./cmd/rekit -- -Command release-check -Format json
.\rekit\rekit.ps1 -Command doctor
.\rekit\tests\facade-smoke.ps1
git diff --check
```

验证结果：已通过 targeted `go test ./internal/rekit/manifest`、targeted `go test ./internal/rekit/manifest ./internal/rekit/doctor ./internal/rekit/sync ./internal/rekit/cli ./internal/rekit/releasecheck`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`/rekit doctor` 与 `facade-smoke.ps1`；`release-check` 输出 `ready=true`。`git diff --check` 仅报告既有 LF/CRLF warning，无 whitespace error。
