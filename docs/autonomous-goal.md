# Autonomous Go-first convergence goal guide

## 读取指南

本文件给新会话接手后执行长期自主推进 goal 使用，重点防止上下文压缩后方向偏移、批次过小或重新沿 PowerShell smoke/catalog 惯性扩张。新会话应按以下顺序读取：

1. 根目录 `CLAUDE.md` 的项目定位、维护入口、验证命令和关键边界。
2. 本文件顶部的实施摘要、执行清单、验证标准、风险与注意事项。
3. `docs/go-first-convergence-plan.md` 的阶段状态与 Stage 1-8 完成信号。
4. `docs/release-readiness.md` 的当前 release gate、CI、known gaps。
5. `docs/batch-plan.md` 末尾最近批次，确认上一轮实际完成到哪里。
6. 具体动手前按方向读取 `docs/powershell-deprecation.md`、`docs/evidence-ledger.md`、`docs/agent-team-rollout-plan.md`、`rekit/tests/README.md` 或相关源码。

本文件不是要求一次性执行所有方向。它定义未来几十到上百轮自主推进的北极星、可选大方向、阶段性小方向、停止条件、验证和可复制 goal 语句。每轮应选择一个中大型 vertical slice，完成实现、测试、文档、验证、提交推送中的完整闭环；不要把目标拆成只改一两行的微批次。

## 实施摘要

截至 Batch 140：

- Go backend 已成为多数 deterministic runtime owner：`release-check`、`status`、`packs`、`doctor/validate`、case lifecycle、sync/promote、overview、note、gate、start/handoff、continue safe subset。
- PowerShell 仍是公共 façade / legacy fallback / Windows parity 层；默认 CI 已由 `.github/workflows/release-gate.yml` 覆盖 Go release checks 和少量 Windows façade smoke。
- 当前安全领域 skeleton pack 均已有真实临时 case dry-run；后续不应继续机械堆 smoke，而应转向 release readiness、PowerShell deprecation 准备、policy/ledger schema hardening、pack-neutral invariants 和接手质量。
- actual heavy-tool、网络/设备/调试/dump/patch/hook、authority/confirmed 自动写入、历史 case state 自动迁移仍是明确禁止作为普通批次顺手迁移的边界。

推荐后续大方向按优先级：

1. **PowerShell deprecation 准备与 fallback 收口**：先 inventory、freeze、retirement checklist 和 invariant；不要直接删除。
2. **Policy / ledger schema append-only hardening**：提高 schema 验证和读层兼容；不自动迁移历史 JSONL。
3. **Pack-neutral invariant hardening**：用 Go tests 锁住非 `vmp-re` pack 不泄漏 vmp 语义，减少领域惯性。
4. **Release gate / CI maturity**：让 CI 和 `release-check` 更能解释缺口、分类失败和指导维护者，但不默认运行大型 matrix。
5. **新会话接手质量与 goal governance**：强化 handoff、batch-plan、文档入口和 stop hook 友好提示，确保长期自主推进不断线。

## 执行清单

每个自主批次必须按以下闭环推进：

1. **重新审查现状**：读取本文件、`docs/go-first-convergence-plan.md`、`docs/release-readiness.md`、`docs/batch-plan.md` 末尾。
2. **选择一个中大型 vertical slice**：每批应能独立验证并降低一个真实风险；避免只改文案或只加一条 invariant 后停止。
3. **明确边界**：判断是否涉及外部副作用、动态调试、dump/patch/hook、authority/confirmed、runtime schema migration 或历史 case rewrite；若涉及，停下询问用户。
4. **Go-first 实施**：确定性逻辑优先在 `internal/rekit/**` 和 Go tests 中落地；PowerShell 只做 façade、fallback、compatibility 或既有 smoke 维护。
5. **文档同步**：每批更新 `docs/batch-plan.md`；若改变 release gate、PowerShell 策略、ledger schema 或 pack-neutral 边界，同步更新相应文档。
6. **验证**：运行与改动匹配的 Go tests、`go vet`、`release-check`、doctor、必要 smoke、`git diff --check`。
7. **自审**：检查是否引入 parallel implementation、orphan、真实 case state、绝对路径、样本/trace/dump/capture/payload/IOC/flag 泄漏。
8. **提交推送**：完成验证后提交并推送 `main`。
9. **继续下一批或明确停止条件**：若用户没有叫停且仍有明显未完成项，继续选择下一批；如果用户要求“先别执行”某些大方向，则只完成当前批次并暂停那些方向。

## 验证标准

基础验证组合按改动类型选择：

```powershell
go run ./cmd/rekit -- -Command release-check -Format json
go test ./...
go vet ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

可选追加：

- 改 PowerShell façade 默认委托或 fallback：追加 `./rekit/tests/facade-smoke.ps1`。
- 改 release gate / CI / release readiness：追加 `go test ./internal/rekit/manifest` 和相关 focused invariant。
- 改 ledger / workstream / gate：追加相关 Go package tests；涉及 case 行为时使用临时 case smoke。
- 改 pack skeleton 或 route：优先对应 pack smoke 或 Go package invariant；只有改 helper/matrix 时才运行大型 matrix。
- 纯文档 goal / roadmap：至少运行 `git diff --check`；若新增文档入口或 release invariant 链接，运行 `go test ./internal/rekit/manifest`。

通过标准：

- `release-check` 输出 `ready=true`，`gateProfile.ready=true`，known gaps 仍真实保留。
- `go test` / `go vet` 无失败。
- doctor 输出 `pack validation ok`。
- `git diff --check` 无 whitespace error；Windows LF/CRLF warning 可记录。
- 所有临时 case / review artifact smoke 清理自身产物。
- 文档必须说明本批边界和验证结果。

## 风险与注意事项

硬性边界：

- 不把真实样本、trace、dump、capture、pcap、crash、payload、客户信息、flag、IOC、绝对 case 路径或 case-specific 进度写入本仓库。
- 不执行真实网络请求、扫描、fuzz、exploit replay、debug、dump、patch、hook、设备连接或其它外部副作用。
- `gate -WhatIf` 只预览 pending-gate request；`gate -Apply` 只写 pending-gate request，不执行 heavy-tool。
- `continue -Apply` 只写 case-local facts/routing/run digest/lane resume/checkpoint/board；不写 authority/confirmed。
- `sync/promote` 保持 review-first；实际写入前必须确认范围，pack source 写入依赖 backup/deny/restore。
- PowerShell 删除、fallback retirement、policy schema migration、历史 case state 自动改写、authority/confirmed 自动写入均必须单独批次、清晰恢复策略和用户授权。
- 不把大型 PowerShell matrix、真实临时 case smoke 或 heavy-tool gate 放入默认 CI。

批次规模边界：

- 中大型批次不是“混杂批次”。每批只围绕一个 coherent vertical slice。
- 避免“只改一行文档/只加一个字段”就提交；但如果是 stop hook 要求的文档收尾，可以作为小收尾批次处理。
- 若一个方向明显超过单批容量，拆成 2-4 个可验证 vertical slice，而不是一次性跨多个子系统。

## 大方向与阶段性实施小方向

### 方向 A：PowerShell deprecation 准备与 fallback 收口

目标：把 PowerShell 从 runtime owner 收束为 façade / legacy compatibility / 少量 parity smoke，但不直接删除、不破坏旧 case。

可分批实施：

1. **Frozen module inventory**：生成或维护 Go-readable inventory，列出 `rekit/lib/*.ps1` 模块状态、owner、fallback 依赖、覆盖 smoke、删除前置条件；用 Go invariant 锁定与 `docs/powershell-deprecation.md` 一致。
2. **Fallback retirement checklist**：为每个 Go-owned 命令输出 retirement readiness：是否有 Go tests、facade smoke、`REKIT_GO_DISABLE=1` 覆盖、旧 case smoke、文档入口、恢复策略。
3. **No new PowerShell owner invariant**：静态测试防止新增 PowerShell business logic、默认委托扩张或 blocked command 进入 safe set。
4. **Legacy text flow containment**：将无 `-Apply` 文本工作线 flow 的边界写成 invariant / docs，不新增状态模型。
5. **Candidate-only removal planning**：只生成删除候选和风险清单，不删除；真正删除必须另行询问用户。

不要做：直接删除 `rekit/lib/*.ps1`、删除 `REKIT_GO_DISABLE=1`、改 case shim、迁移 heavy-tool / authority / confirmed。

### 方向 B：Policy / ledger schema append-only hardening

目标：增强 ledger schema 的验证、读层兼容和文档一致性，同时保持 append-only，不自动迁移历史 JSONL。

可分批实施：

1. **Ledger schema invariant**：将 `docs/evidence-ledger.md` 中事件 kind、必需字段、枚举和 runtime 支持矩阵转成 Go tests 或 schema helper。
2. **Historical compatibility tests**：构造最小历史 JSONL fixture，验证旧 `action`/`decision`、缺字段、旧 status 仍可被 overview/handoff/note-list 读取。
3. **WhatIf schema diagnostics**：增强 `note -WhatIf` / `gate -WhatIf` 的非写入诊断，提示缺字段、非法枚举、lane mismatch。
4. **Policy docs single source**：减少 `common/policies/**` 与 `docs/evidence-ledger.md` 重复定义，明确谁是 schema source、谁是使用说明。
5. **Migration plan only**：如确需 schema migration，先写 migration plan 和 rollback，不自动改历史 case state。

不要做：批量重写历史 `.rekit/facts/*.jsonl`、自动确认 authority/confirmed、把 schema migration 放进普通 `continue -Apply`。

### 方向 C：Pack-neutral invariant hardening

目标：让 runtime 更像 pack-neutral framework，非 `vmp-re` pack 的 init/doctor/route/workstream/handoff 不泄漏 vmp 路径、lane 或 authority 语义。

可分批实施：

1. **Default pack decision audit**：集中检查 Go/PowerShell/文档中的 `vmp-re` 默认点，区分示例、默认 pack、业务特判。
2. **Pack-neutral Go invariant**：为 manifest inventory、route selection、start workspace、handoff leakage guard 加 Go tests，抽样覆盖 2-3 个 skeleton pack，而不是新增大量 smoke。
3. **Manifest-driven deny / workspace rules**：让 pack-specific deny patterns、workspace roots、authority lanes 只来自 manifest 或 pack policy。
4. **Example hygiene**：README/模板中保留 `vmp-re` 示例时明确“示例不是默认边界”，避免新会话误以为项目只服务 VMP。
5. **New pack onboarding checklist**：当新增 pack 时，要求 manifest、pack smoke、真实 dry-run 或 Go package invariant 成套落地。

不要做：为每个 pack 无限新增 smoke、把 vmp 专用逻辑复制到 generic runtime、在模板中写真实样本路径。

### 方向 D：Release gate / CI maturity

目标：让 release gate 更可解释、更适合 CI 和新会话接手，同时保持轻量默认。

可分批实施：

1. **CI result diagnostics**：让 `release-check` 输出更明确的 category、owner、howToFix、required/optional 分类。
2. **Workflow invariant expansion**：锁定 CI 不运行大型 matrix、不触发 heavy-tool、不依赖真实 case，同时保证 Go/Windows 两层覆盖不漂移。
3. **Release readiness preflight command**：仍只做 inventory / plan，不执行测试编排；避免变成 PowerShell test orchestrator。
4. **Docs consistency tests**：检查 README、CLAUDE、release-readiness、tests guide 对 release gate 的描述一致。
5. **Manual release checklist**：补充 tag/version/changelog 发布前人工检查，但不自动发布外部服务。

不要做：默认 CI 跑 pack matrix、真实临时 case dry-run 全量、网络/设备/heavy-tool、自动发布 release。

### 方向 E：新会话接手质量与 goal governance

目标：让新会话无需读取长聊天历史，也能按正确方向推进几十轮。

可分批实施：

1. **Handoff doc template for maintainers**：新增或强化维护者接手模板，包含当前 batch、下一批候选、验证、停止条件。
2. **Batch-plan top index**：在 `docs/batch-plan.md` 顶部增加最近批次索引或“当前 next candidates”，减少每次读取超长文档成本。
3. **Goal drift guard docs**：把“不要完成一两批就宣布 goal 完成”“不要微批次”“不要扩张 PowerShell 编排”写入更靠前位置和 invariant 可检查位置。
4. **Stop hook friendly checklist**：明确每轮修改文件后必须做文档收尾、验证、任务状态清理。
5. **New-session prompt snippets**：维护本文件中的接手开场白和长期 goal 语句。

不要做：只依赖聊天上下文、只在 final 总结里记录计划、不更新文档。

## 推荐执行顺序

如果用户没有指定方向，建议按以下节奏轮转，避免单一方向钻太深：

1. 先做 1-2 批 **方向 E**，提高新会话接手质量和 batch-plan 顶部索引。
2. 做 2-4 批 **方向 A**，完成 PowerShell deprecation inventory / checklist / invariant，但不删除。
3. 做 2-4 批 **方向 C**，把 pack-neutral 不泄漏语义转成 Go invariants。
4. 做 2-4 批 **方向 B**，增强 ledger schema append-only 验证和历史兼容。
5. 穿插 **方向 D**，提高 release-check / CI 诊断能力。
6. 每 5-8 批回看 `docs/go-first-convergence-plan.md`、`docs/release-readiness.md`、`docs/batch-plan.md`，调整 next candidates。

## 新会话接手开场白

可以先发给新会话这段，让它建立上下文，但暂不开始长期执行：

```text
你将在 C:\AI\m_projects\RE\re-context-kits 继续维护 re-context-kits。请先读取根目录 CLAUDE.md、docs/autonomous-goal.md、docs/go-first-convergence-plan.md、docs/release-readiness.md、docs/batch-plan.md 末尾最近批次，以及必要时 docs/powershell-deprecation.md / rekit/tests/README.md。先不要实施，先用简短中文回复你对当前阶段、主要安全边界、后续优先方向和停止条件的理解。
```

## 可直接复制的长期 goal 语句

### 推荐总 goal（中大型自主推进）

```text
在 C:\AI\m_projects\RE\re-context-kits 中，按照 docs/autonomous-goal.md、docs/go-first-convergence-plan.md 和 docs/release-readiness.md 长期持续自主推进 Go-first convergence / release readiness / PowerShell deprecation readiness / policy-ledger hardening / pack-neutral invariant hardening。每轮选择一个中大型、可验证的 vertical slice，不要做只改一两行的微批次；每批必须包含实现或 invariant、文档写回、验证、自审、提交并推送 main。完成一批后重新审查现状和 docs/batch-plan.md 最新记录，继续选择下一批；不要把阶段性进展当成 goal 完成。

优先方向按 docs/autonomous-goal.md：1) 新会话接手质量与 goal governance；2) PowerShell deprecation 准备与 fallback 收口，但不直接删除；3) pack-neutral invariant hardening，用 Go tests 锁住非 vmp-re 不泄漏 vmp 语义；4) policy / ledger schema append-only hardening，兼容历史 JSONL，不自动迁移历史 case state；5) release gate / CI maturity，保持默认 CI 轻量，不运行大型 matrix 或 heavy-tool。

硬性边界：不要执行真实网络请求、扫描、fuzz、exploit replay、debug、dump、patch、hook、设备连接或 heavy-tool；不要写 authority/confirmed；不要自动迁移 policy schema 或历史 case state；不要把真实样本、trace、dump、capture、payload、flag、IOC、客户信息、绝对 case 路径写入仓库；不要删除 PowerShell runtime 或 REKIT_GO_DISABLE fallback，除非单独批次、清晰恢复策略并得到用户明确授权。遇到产品方向变化、破坏性动作、外部副作用、runtime schema migration、历史数据 rewrite、authority/confirmed 自动写入或难以判断的架构取舍时停下询问。否则持续自主推进几十到上百轮。
```

### 保守版 goal（先做文档 / invariant / readiness，不碰高风险 runtime）

```text
在 C:\AI\m_projects\RE\re-context-kits 中，先按 docs/autonomous-goal.md 做 5-10 批低风险但中大型的 readiness / invariant / documentation vertical slice：强化新会话接手质量、batch-plan 顶部索引、PowerShell deprecation inventory/checklist、pack-neutral Go invariants 和 release-check diagnostics。不要删除 PowerShell 代码，不迁移 policy schema，不改历史 case state，不执行 heavy-tool，不写 authority/confirmed。每批完成后运行匹配验证，更新文档，提交并推送 main，然后继续下一批，直到这些低风险 readiness 方向明显收束。
```

### PowerShell deprecation readiness 专项 goal

```text
在 C:\AI\m_projects\RE\re-context-kits 中，围绕 docs/autonomous-goal.md 的“方向 A：PowerShell deprecation 准备与 fallback 收口”连续推进若干中大型批次。目标是建立 frozen module inventory、fallback retirement checklist、no-new-PowerShell-owner invariant 和 legacy text flow containment。不要直接删除 PowerShell runtime，不删除 REKIT_GO_DISABLE，不改变 case-local shim，不迁移 heavy-tool/authority/confirmed。每批更新 docs/powershell-deprecation.md、docs/batch-plan.md 和必要 release invariants，验证后提交推送 main。
```

### Policy / ledger schema 专项 goal

```text
在 C:\AI\m_projects\RE\re-context-kits 中，围绕 docs/autonomous-goal.md 的“方向 B：Policy / ledger schema append-only hardening”连续推进若干中大型批次。目标是把 docs/evidence-ledger.md 的 kind/字段/枚举/runtime 支持矩阵转为 Go invariants 或 schema helper，补历史 JSONL 兼容测试，增强 WhatIf 非写入诊断，并减少 policy 文档重复定义。必须保持 append-only，不自动迁移历史 case state，不写 authority/confirmed，不把 schema migration 放进普通 continue/gate apply。每批验证、更新文档、提交推送 main。
```

### Pack-neutral invariant 专项目标

```text
在 C:\AI\m_projects\RE\re-context-kits 中，围绕 docs/autonomous-goal.md 的“方向 C：Pack-neutral invariant hardening”连续推进若干中大型批次。目标是减少 vmp-re 默认惯性，用 Go tests / release invariants 锁住非 vmp pack 的 init/doctor/route/workstream/handoff 不泄漏 vmp 路径、lane 或 authority 语义，并让 workspace roots、deny patterns、authority lanes 尽量 manifest-driven。不要为每个 pack 无限新增 smoke，不复制 vmp 专用逻辑进 generic runtime。每批验证、更新文档、提交推送 main。
```

## 推荐每批 final 摘要格式

每批最终回复保持简洁：

```text
完成 Batch N：<标题>
- 主要改动：...
- 文档更新：...
- 验证：...
- 提交：<hash> <message>
- 下一批建议：<一个中大型 vertical slice>
```

如果仍有任务 in_progress，不要输出最终总结；先更新任务状态。
