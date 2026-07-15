# Go-first convergence and release readiness plan

## 读取指南

本文件是 Batch 101 之后的新阶段导航，用于防止长轮次自主实施或上下文压缩后继续沿 PowerShell smoke/catalog/test metadata 惯性扩张。

新会话接手时优先读取：

1. 根目录 `CLAUDE.md` 的项目定位、维护入口、关键边界。
2. `docs/mission-control-product-direction.md` 的 Lane-centric Agent Team Mission Control 产品北极星。
3. `docs/autonomous-goal.md` 的长期自主 goal、阶段性大方向、停止条件和可复制 prompt。
4. 本文件顶部的读取指南、实施摘要、执行清单、验证标准、风险与注意事项。
5. `docs/batch-plan.md` 最新阶段记录。
6. 发布或收口前读取 `docs/release-readiness.md` 的一页门禁和 known gaps。
7. 修改 PowerShell façade/runtime 或 fallback 前读取 `docs/powershell-deprecation.md`。
8. 具体动手前再按需读取 `docs/go-runtime-migration.md`、`docs/agent-team-rollout-plan.md`、`rekit/tests/README.md`、相关 Go/PowerShell runtime 文件。

本文件不是要求一次性完成所有事项；它定义未来几十轮自主推进的方向、阶段切片和停止条件。每轮应选择一个中型到大型、可验证的垂直切片实施，不要把目标拆成只改一两行的微批次。

## 实施摘要

当前项目已经从连续多批自主优化进入 **Go-first 收束 / release readiness / Agent Team 真实闭环** 阶段。Batch 92-101 已把多 pack skeleton 的 PowerShell smoke、matrix、tests guide 和 catalog metadata 收束到可维护状态；下一阶段不应继续扩张 PowerShell 编排能力。

新的北极星：

> Go backend 成为 rekit 的 deterministic runtime owner；PowerShell 逐步降级为 thin shim / legacy compatibility / 少量 façade parity smoke；新的状态、计划、验证、结构化输出和 release gate 优先落在 Go 中；Agent Team 收敛为 Lane-centric Mission Control，通过真实临时 case dry-run 闭环验证 durable member lane、可替换 session executor、Human-in-the-Lane reconcile、预授权 lane autonomy 和 tactical subagent，而不是继续堆叠 smoke catalog。

核心方向：

1. **Go runtime 收口**：把低风险 read-only 命令、case lifecycle、sync/promote、workstream/ledger/gate/continue 逐步迁移到 Go-first。
2. **Release readiness**：把核心 invariant 放入 Go tests / go vet / doctor / 少量 Windows façade smoke，而不是依赖不断扩张的 PowerShell matrix。
3. **Agent Team 真实使用闭环**：用临时 case 验证主 Agent mission brief → member lane packet/status/outbox → user intervention reconcile → replaceable session handoff → tactical subagent review → preauthorized heavy-action request/record → continue/gate/note/handoff 的可接手链路。
4. **减少 PowerShell runtime/测试编排扩张**：除修复既有 smoke/parity 外，不新增 PowerShell 编排层或新的 PowerShell runtime 能力。
5. **Pack-neutral 架构**：减少 `vmp-re` 默认惯性，确保 runtime 行为来自 manifest / policy，而不是领域特判。
6. **新会话接手质量**：文档要记录当前阶段、已完成切片、下一步候选和验证门禁，避免只依赖聊天历史。

## 执行清单

每个自主批次按以下流程执行：

1. **读阶段导航**：先读本文件顶部和 `docs/batch-plan.md` 最新阶段记录。
2. **选中型垂直切片**：每批应包含实现、测试、文档和验证中的多个环节；避免只做 metadata 微调。
3. **Go-first 决策**：新增确定性逻辑优先放入 `internal/rekit/**`；PowerShell 只做 shim、compatibility 或既有 smoke 维护。
4. **迁移而非复制**：若 Go 与 PowerShell 已有双实现，应推进 ownership 收口，避免创建第三套并行逻辑。
5. **保持 case shim thin**：不要把 runtime 逻辑复制进 case-local `/rekit`。
6. **临时 case 验证**：涉及 init/attach/sync/promote/workstream/ledger/gate 时，用临时 case，不在 kit 仓库伪造真实 case state。
7. **自审后写回计划**：每批完成后更新 `docs/batch-plan.md` 或本文件的进度/下一步，记录验证结果和剩余缺口。
8. **验证再收尾**：至少运行相关 Go tests、doctor、必要 smoke、`git diff --check`；失败必须说明原因和是否为既有阻塞。
9. **持续推进**：完成一个批次后，若 goal 仍未完成且未触发停止条件，继续选择下一批。

## 验证标准

基础验证按改动类型递增选择：

- 文档或规划改动：`git diff --check`。
- Go runtime / Go tests 改动：`go test ./...`，并优先加入 `go vet ./...` 作为 release readiness 方向。
- Pack manifest / doctor / inventory 改动：`go test ./...`、`./rekit/rekit.ps1 doctor`、必要时 `./rekit/rekit.ps1 packs`。
- PowerShell façade 委托改动：`./rekit/tests/facade-smoke.ps1`。
- Existing PowerShell smoke 维护：只维护既有 smoke/parity，不新增大编排；必要时运行 `rekit/tests/README.md` 指定的最小相关组合。
- init/attach/sync/promote/workstream/ledger/gate 改动：使用临时 case 验证 no-real-artifact、backup、review-first、containment、no confirmed/authority unless explicitly allowed。
- Agent Team 闭环改动：验证临时 case 中 packet、ledger events、gate request、handoff 和新会话可接手信息完整。

Release readiness 目标门禁应逐步收束为：

```powershell
go test ./...
go vet ./...
.\rekit\rekit.ps1 doctor
.\rekit\tests\facade-smoke.ps1
# 按改动类型选择少量临时 case smoke，不默认运行大型 PowerShell matrix
git diff --check
```

## 风险与注意事项

- 不要继续新增 PowerShell 编排能力；除非只是维护既有 smoke、修复 compatibility 或验证 façade parity。
- 不要把 `catalog.json`、matrix、tests guide 继续扩展成事实上的 PowerShell test orchestrator；若需要 release gate，优先做 Go tests / CI。
- 不要把真实样本、trace、dump、capture、payload、flag、客户信息、目标信息、绝对 case 路径或 case-specific 进度写入模板仓库。
- 不要绕过 review-first / dry-run / packet 化边界直接写 confirmed、authority、pack 文件或外部副作用。
- 动态调试、注入、patch、dump、hook、脱壳写文件、网络扫描、设备连接、fuzz、exploit replay 等若已在当前 lane 文档/packet/autonomy profile 中明确预授权，可在 scope、预算、止损、输出路径和记录要求内自主执行；未授权或越界时必须显式询问用户。
- runtime schema 迁移、破坏性删除、外部发布、新的产品方向变化或难以判断的架构取舍必须停下询问。
- 中大型批次不等于混杂批次；每批仍应围绕一个 coherent vertical slice，避免跨太多子系统导致难以验证。

## 当前状态基线（Batch 101 后）

已完成：

- 多领域 skeleton pack 已建立，`vmp-re` 是首个 mature pack，其它安全领域 pack 主要为 skeleton。
- Go backend 已覆盖大量命令面，包括 `status`、`packs`、`doctor/validate`、`attach`、`repair`、`init/bootstrap`、`sync/update`、`promote`、`overview`、`start`、`handoff`、`continue`、`plan-subagents`、`gate`、`note`。
- PowerShell façade 仍是公共入口；低风险只读、case lifecycle、sync/promote、overview 文本/JSON 与缺 board 初始化、note 文本/JSON 只读查询、note append/what-if、gate what-if/apply request、start/handoff JSON preview 与显式 apply、continue JSON preview 与 explicit apply、plan-subagents review artifact 路径已逐步默认委托 Go；无 `-Apply` 的文本工作流仍按边界回落 PowerShell。
- Batch 92-101 已把 pack smoke helper/matrix/discovery/tests guide/catalog metadata 收束成维护体系。
- Agent Team 契约、ledger 基础、review-first sync/promote、Go review artifacts、Go gate preview/request、Go `plan-subagents` 只读计划器已经存在。

主要缺口：

- Go 还不是默认 runtime owner。
- PowerShell 与 Go 在 manifest、sync/promote、workstream/ledger 等领域存在双实现漂移风险。
- 无 `-Apply` 的文本工作线 flow、authority/confirmed gate、batch-level replay/resume 等仍有 Go/PowerShell 语义分裂或 façade 暴露不足。
- Release gate 仍偏人工 PowerShell smoke 组合，缺少 Go-first invariant tests / CI。
- `vmp-re` 默认仍散落在 runtime 和文档中，pack-neutral 收口未完成。

## 阶段性实施方向

### Stage 1：Go-first release gate 与 invariant migration

目标：把最近 PowerShell catalog/matrix/discovery 中的核心确定性规则迁入 Go tests，停止继续扩张 PowerShell 编排。

推荐批次形态：

- 新增 Go test 覆盖 `rekit/tests/catalog.json` schema、全部 `.ps1` 覆盖、recommended minimum 必含关键命令、related docs 存在性。
- 新增 Go test 覆盖 skeleton pack discovery：manifest inventory、matrix/wrapper/catalog 一致性。
- 将 `go vet ./...` 纳入 release readiness 文档或 tests guide 的 Go-first minimum。
- 保留 `catalog-smoke.ps1` / `pack-smoke-matrix-selftest.ps1` 作为 compatibility，不再扩展新特性。

完成信号：

- `go test ./...` 能捕获新增 smoke 未登记、skeleton pack wrapper/catalog 不一致等核心漂移。
- tests guide 明确 PowerShell smoke 是 compatibility/parity 层，不是未来扩张方向。

当前进度：Batch 103 已迁入第一组 Go release invariant tests：`catalog.json` schema/全部 `.ps1` 覆盖/recommended minimum/related docs，以及 schema-valid skeleton pack 与 catalog、wrapper、matrix 的一致性；`go vet ./...` 已纳入 tests guide 和 catalog recommended minimum。Stage 1 仍可继续补更深的 Go invariant（例如把 `pack-inventory-smoke.ps1`、continue apply smoke metadata 或代表性 skeleton pack no-write smoke Go 化），但不再扩张 PowerShell catalog/matrix 功能。

### Stage 2：低风险 Go default façade

目标：让 `/rekit` 对低风险 read-only 命令默认使用 Go，开始 PowerShell shim 化。

推荐批次形态：

- `status`、`packs`、`doctor`、`validate` 默认委派 Go，保留 env disable/fallback。
- 更新 façade smoke，覆盖默认 Go、disable fallback、错误回退语义。
- 文档标明 Go read-only runtime 已成为默认路径。

完成信号：

- 用户不设置 `REKIT_GO_ENABLE` 也走 Go read-only runtime。
- PowerShell 对这些命令不再作为业务逻辑 owner。

当前进度：Batch 104 已完成低风险 read-only default façade：`status`、`packs`、kit/case `doctor/validate` 默认委托 Go；Batch 113 将已初始化 case 的 `overview -Format json` 与 `note -List -Format json` 只读查询也纳入默认 Go façade；Batch 114 将 `gate -WhatIf` 非写入 heavy-tool gate preview纳入默认 Go façade；Batch 115 将 `gate -Apply` pending-gate request ledger 写入纳入默认 Go façade；Batch 117 将 attached case 的 `start`/`handoff` JSON preview 与显式 `-Apply` 纳入默认 Go façade；Batch 118 将 attached case 的 `continue -WhatIf -Format json` 非写入 preview 纳入默认 Go façade；Batch 119 将 attached case 的 `overview` 文本/JSON 与缺 board 初始化纳入默认 Go façade；Batch 120 将 attached case 的 `note -List` 文本/table/tsv 查询也纳入默认 Go façade；Batch 121 将 explicit `continue -Apply` 的 case-local facts/routing/run digest/resume/board 写入纳入默认 Go façade；`REKIT_GO_DISABLE=1` 强制禁用 Go façade 委托。Stage 2 的核心完成信号已达成，后续可继续观察 façade smoke 与真实 dry-run case，或补充错误 fallback 文档/测试。

### Stage 3：case lifecycle 写入路径 Go 化

目标：迁移边界清晰的写入命令，为后续 sync/promote/workstream 收口铺路。

推荐批次形态：

- 让 `attach`、`repair`、`init/bootstrap` apply 在明确参数/确认边界下走 Go。
- 用临时 case 验证 metadata、thin shim、managed docs、template files、state、doctor。
- 删除或降级 PowerShell 对应业务逻辑为 legacy fallback。

完成信号：

- 新 case 创建/绑定的 deterministic 写入由 Go 负责。
- case-local shim 仍保持 thin，不复制 runtime。

当前进度：Batch 105 已完成第一组 case lifecycle default façade：`attach`、`repair`、`init/bootstrap` 的非写入预览与显式 `-Apply` 写入默认委托 Go，`REKIT_GO_DISABLE=1` 可强制回退 PowerShell。Go `attach` 语义补齐 PowerShell binding baseline，会写 `.rekit/instance.yml`、legacy `.re-template.yml`、初始 `.rekit/state.json` 与 thin shim；Go 委托路径下 `attach`、`init/bootstrap` 仍要求显式 `-WhatIf` 或 `-Apply`，裸命令保留为 PowerShell legacy compatibility。Stage 3 的核心完成信号已基本达成，后续可继续做 case lifecycle parity hardening，或进入 Stage 4 收口 sync/promote review/apply。

### Stage 4：sync/promote review/apply 收口

目标：统一 kit → case 与 case → kit 的 review-first/apply/candidate 行为到 Go。

推荐批次形态：

- Batch 106 已完成第一步：`/rekit sync` review、`sync -Apply` 实际写入与 JSON apply preview 默认走 Go，`REKIT_GO_DISABLE=1` 与文本 dry-run 保留 PowerShell fallback。
- Batch 107 已完成 promote 非写入默认 façade：`/rekit promote` review、review artifacts、`promote -CreateCandidates -WhatIf -Format json` 与 `promote -Apply -WhatIf -Format json` 默认走 Go。
- Batch 108 将 `promote -CreateCandidates` 实际候选写入纳入默认 Go façade，继续保留文本 what-if fallback、`REKIT_GO_DISABLE=1` fallback 与 `promote -Apply` 实际 pack source 写入 fallback/manual 边界。
- Batch 112 将 `/rekit promote -Apply` 实际 pack source 写入纳入默认 Go façade，仍要求显式 `-Apply`、禁止与 `-CreateCandidates`/review artifacts 混用，保留文本 what-if 与 `REKIT_GO_DISABLE=1` PowerShell fallback，并依赖 backup/deny/validation/restore/package tests/smoke。
- Batch 109 开始把 promote candidates/apply 中可单元化的 sanitize、candidate index、containment/no-write、restore helper 迁到 Go package tests；Batch 110 将 sync apply/preview 的 backup、managed block、template force、state refresh 与 backup escape guard 迁入 Go package tests；Batch 111 将 promote apply 的 what-if no-write、backup/validation rows、blocked deny 与 validation failure restore 迁入 Go package tests。

完成信号：

- sync/promote 不再长期维护 Go/PowerShell 双实现。
- review artifact 语义在文档中明确：不写 managed/authority/pack 内容，但可写 `.rekit/reviews/**` review artifacts。

当前进度：Batch 106 已将 sync 侧第一组收口为默认 Go façade：`/rekit sync` review、`sync -Apply` 实际写入、`sync -Apply -WhatIf -Format json` 非写入 preview 默认委托 Go；`REKIT_GO_DISABLE=1` 强制回退 PowerShell，文本 `sync -Apply -WhatIf` 保留 PowerShell dry-run。Batch 107 已将 promote 非写入路径收口为默认 Go façade：`/rekit promote` review、review artifact 写入、`promote -CreateCandidates -WhatIf -Format json` 与 `promote -Apply -WhatIf -Format json` 默认委托 Go。Batch 108 进一步将 `promote -CreateCandidates` 实际候选写入纳入默认 Go façade；`REKIT_GO_DISABLE=1` 强制回退 PowerShell，文本 promote what-if 与 `promote -Apply` 实际 pack source 写入继续走 PowerShell fallback 或手动 Go CLI。Batch 109 开始把 promote candidates/apply 的可单元化写入边界迁入 `internal/rekit/promote` package tests；Batch 110 继续把 sync apply/preview 的可单元化写入边界迁入 `internal/rekit/sync` package tests；Batch 111 继续把 promote apply 的 what-if no-write、backup/validation rows、blocked deny 与 validation failure restore 迁入 `internal/rekit/promote` package tests。Batch 112 将 `promote -Apply` 实际 pack source 写入也纳入默认 Go façade；Stage 4 后续优先处理 sync/promote 双实现收尾、PowerShell fallback 降级策略，或继续把尚未覆盖的 parity/deny/restore 边界收进 Go tests。

### Stage 5：workstream / ledger / gate / continue Go 化

目标：让 Agent Team 日常状态流从 PowerShell runtime 收口到 Go。

当前进度：Batch 113 先把无需新增写入面的 `overview -Format json` 与 `note -List -Format json` 只读查询升级为默认 Go façade 委托；Batch 114 将 `gate -WhatIf` 非写入 heavy-tool gate preview 升级为默认 Go façade；Batch 115 将 `gate -Apply` pending-gate request ledger 写入升级为默认 Go façade，并补 `internal/rekit/gate` package tests 覆盖 no-write、actor guard、幂等和 no authority/confirmed/artifacts；Batch 116 将 attached case 的 `note` append 与 `note -WhatIf` facts JSONL 写入/预览升级为默认 Go façade，并补 `internal/rekit/note` package tests 覆盖 no-write、schema/lane guard、幂等和 no authority/confirmed；Batch 117 将 attached case 的 `start`/`handoff` JSON preview 与显式 `-Apply` 升级为默认 Go façade；Batch 118 将 attached case 的 `continue -WhatIf -Format json` 非写入 preview 升级为默认 Go façade；Batch 119 将 attached case 缺 board 时的 overview case-local board/facts/policy/default authority lane 初始化迁入 Go，并让 overview 文本/JSON 默认经 façade 委托 Go；Batch 120 将 attached case 的 `note -List` 文本/table/tsv 只读查询也纳入默认 Go façade；Batch 121 将 explicit `continue -Apply` 纳入默认 Go façade，写 case-local facts/routing/run digest/lane resume/checkpoint 与 board refresh，同时将 authority/confirmed 写入强制 defer；Batch 122 新增 `_template` pack 的 Go CLI package E2E 测试，覆盖 start → note → continue apply → lane/project handoff 的 case-local 闭环；Batch 123 继续新增 `_template` pack 的 Go CLI package E2E 测试，覆盖 start → plan-subagents review artifact → gate apply pending-gate request → overview → lane handoff 的 bounded dispatch / heavy-tool gate 可见性链路；Batch 124 新增 `_template` pack 的 Go CLI package E2E 测试，覆盖 candidate note → reviewer verification → main decision → note list → overview → handoff 的 reviewer verdict / merge decision 可见性链路；Batch 125 新增 `generic-binary-re` Go CLI package E2E 测试，覆盖 non-feature `binary-analysis` lane、pack-specific route、candidate/gate/verification/decision、overview 与 handoff 的 pack-neutral 闭环；Batch 126 新增 `web-security` Go CLI package E2E 测试，覆盖非 RE-only Web/API `feature` lane、`web-security:feature-analysis` route、network pending-gate、candidate/verification/decision、overview 与 handoff 的 pack-neutral 闭环；Batch 127 新增 `web-security-agent-team-dryrun-smoke.ps1`，通过公共 `/rekit` façade 与真实临时 case 跑通同一 Web/API Agent Team dry-run 链路；Batch 189 让 Go-owned overview 文本/JSON 输出 `missionBrief`，并让 lane handoff 写出 `## Mission Control brief`，把 ready/blocked lanes、pending gates、open decisions、interventions、next agent actions 与 escalations 汇总成主 Agent / 新会话可直接消费的 Mission Control 状态；Batch 191 对齐 lane handoff 与 overview 的 blocker 语义，确保 pending gate、open intervention、open candidate/decision 都会让对应 lane 显示为 blocked，并补 decision-only/candidate-only parity 覆盖；Batch 192 将同一 Mission Control brief 扩展到项目级 handoff 索引，让新会话从 `.rekit/handovers/latest.md` 也能先看到 ready/blocked lanes、pending gates、open decisions、interventions、next agent actions 与 escalations。Batch 193 将 Mission Control brief 聚合逻辑抽入 `internal/rekit/mission`，overview、project handoff 与 lane handoff 复用同一 blocker、line item、next action 与 escalation helper，降低后续 open decision / gate / intervention 语义再次漂移的风险。Batch 194 将同一 brief 暴露到 Go `handoff` JSON envelope，项目级与指定 lane 的 `-WhatIf/-Apply -Format json` 输出都包含结构化 `missionBrief`，自动化可直接消费 handoff preview/apply 结果而不必解析 Markdown。Batch 195 将同一 brief 扩展到 Go `continue` JSON envelope 与 run artifacts，`continue -WhatIf/-Apply -Format json`、`.rekit/runs/<run-id>/status.json` 和 `digest.md` 都能暴露 apply 前/后的 Mission Control 状态，lane executor 继续后不必另跑 overview/handoff 才能看到 ready/blocked lanes、open decisions、gate/intervention 和 next actions。Batch 196 将同一 brief 扩展到 Go `start` JSON envelope，`start -WhatIf/-Apply -Format json` 在创建或进入 lane 前后直接输出 `missionBrief`，让自动化在新 lane 初始化后立即获得全局 Mission Control 状态。Batch 197 将同一 brief 扩展到 Go `gate` preview/apply JSON envelope，`gate -WhatIf/-Apply -Format json` 可直接显示 pending-gate request 写入前后的 Mission Control 状态，而实际 heavy-tool 仍不执行。Batch 198 将 case-local Mission Control snapshot 读取逻辑抽入 `internal/rekit/mission`，让 `start`、`handoff` facts loading 和 `gate` 复用 shared board/facts JSONL reader、`CaseBrief` 与 board lane adapter，降低后续 `missionBrief` envelope 在 pending-gate、open intervention、open candidate/decision blocker 语义上的漂移风险。Batch 199 继续让 Go `overview` 复用 `mission.ReadBoard`、strict ledger facts reader、pending decision/batch aggregation 与 board lane label helper，移除 overview-local board/facts JSONL 与 label 复制逻辑。Batch 200 继续让 Go `note` 复用 mission shared fact-file mapping、strict facts JSONL reader 与 board lane guard，移除 note-local JSONL reader、board struct 与 fact-file switch 复制逻辑。Batch 201 将 note/gate 的 board lane guard 进一步收口到 mission shared helper，复用 known-lane 列表、missing/empty board 诊断与大小写敏感/不敏感 lookup 语义。实际 heavy-tool 执行、无 `-Apply` 的文本工作线 flow、authority/confirmed 写入与 policy schema 迁移仍需单独 gate。

推荐批次形态：

- Go overview 已支持 board/facts/lanes 初始化并经默认 façade 覆盖文本/JSON，消除了“先跑 PowerShell overview”的依赖。
- Go note append 成为默认路径，保留 eventId 去重、lane 校验、JSONL append。
- Go start/handoff apply 和 gate apply 通过 façade 暴露，保持 confirmation/gate semantics。
- Go continue 从 preview 扩展到可控 apply：写 digest、accepted ledger events、pending gate request 或 apply plan；confirmed/authority 仍按 policy gate。

完成信号：

- 临时 case 的 start → note → continue → handoff 可主要通过 Go runtime 完成；Batch 122 已用 `_template` pack 的 Go package test 锁定最小闭环，Batch 123 已补 `_template` pack 的 plan-subagents → gate request → overview/handoff 可见性闭环，Batch 124 已补 reviewer verification → main decision → overview/handoff 可见性闭环，Batch 125 已补 `generic-binary-re` 非 feature lane / pack-specific route 的 pack-neutral 可见性闭环，Batch 126 已补 `web-security` 非 RE-only route / network gate 的 pack-neutral 可见性闭环，Batch 127 已把 `web-security` 闭环提升为公共 façade 下的真实临时 case dry-run smoke，Batch 132 已将 `generic-binary-re` binary-analysis/debug gate 闭环也提升为公共 façade 下的真实临时 case dry-run smoke，Batch 133 已将 `malware-analysis` sample-analysis/network sandbox gate 闭环提升为公共 façade 下的真实临时 case dry-run smoke。
- heavy-tool / authority / confirmed 仍有明确 gate 和用户确认边界。

### Stage 6：Agent Team 真实 dry-run 闭环

目标：用实际临时 case 验证 Agent Team 框架是否可被新会话接手和持续推进。

推荐批次形态：

- 建立一个临时 case dry-run 脚本或 Go-driven test harness，覆盖 main lane、feature lane、reviewer verification、candidate status、gate request、handoff；Batch 122 已先用 `_template` pack 的 Go package test 覆盖 start/note/continue/handoff 最小闭环，Batch 123 已继续覆盖 plan-subagents review artifact、gate request、overview 与 handoff 展示，Batch 124 已继续覆盖 reviewer verification、candidate/decision note list、overview 与 handoff 展示，Batch 125 已继续覆盖 `generic-binary-re` 的 `binary-analysis` lane、`workspace/binary/**`、pack-specific route、candidate/gate/verification/decision 与 handoff 展示；Batch 126 已继续覆盖 `web-security` 的 endpoint/feature route、network pending-gate、candidate/verification/decision 与 handoff 展示，Batch 127 已新增 `web-security` 真实临时 case smoke 覆盖公共 façade 下的 init/start/plan-subagents/gate/note/overview/handoff/doctor dry-run，Batch 132 已新增 `generic-binary-re` 真实临时 case smoke 覆盖 binary-analysis lane、debug pending-gate、candidate/verification/decision 与 handoff leakage guard，Batch 133 已新增 `malware-analysis` 真实临时 case smoke 覆盖 sample-analysis lane、network/sandbox pending-gate、candidate/verification/decision 与 handoff leakage guard，Batch 134 已新增 `vuln-research` 真实临时 case smoke 覆盖 vuln-analysis lane、debug/repro pending-gate、candidate/verification/decision 与 handoff leakage guard，Batch 135 已新增 `ctf` 真实临时 case smoke 覆盖 challenge-analysis lane、remote/network pending-gate、candidate/verification/decision 与 handoff leakage guard，Batch 136 已新增 `unpack-pe` 真实临时 case smoke 覆盖 unpack-analysis lane、dump/unpack pending-gate、candidate/verification/decision 与 handoff leakage guard，Batch 137 已新增 `ollvm` 真实临时 case smoke 覆盖 obfuscation-analysis lane、full-trace/CFG pending-gate、candidate/verification/decision 与 handoff leakage guard，Batch 138 已新增 `android-native` 真实临时 case smoke 覆盖 native-analysis lane、inject/hook pending-gate、candidate/verification/decision 与 handoff leakage guard。
- 不需要自动 spawn reviewer；重点验证 packet、ledger、handoff、gate、continue digest 是否足够完整。
- 安全领域 skeleton pack 已全部拥有真实临时 case dry-run；后续 Stage 6/7 工作转向抽样强化、跨 pack 组合验证、release readiness/CI 门禁或新 pack 引入时的真实 dry-run。

完成信号：

- 新会话能从 handoff / overview / ledger digest 判断下一步；Batch 189 已把 Mission Control brief 纳入 overview JSON/text 与 lane handoff，减少主 Agent 接手时遍历完整 ledger 的负担。
- Agent Team 不再只是契约文档和单点 smoke，而有可重复 dry-run 闭环。

### Stage 7：pack-neutral hardening

目标：减少 `vmp-re` 默认惯性，让 runtime 更像 pack-neutral framework。

推荐批次形态：

- 抽出单一 default pack 决策点，减少 Go/PowerShell 中散落的 `vmp-re` 字符串；Batch 150 已新增 Go `internal/rekit/defaults.DefaultPack`，并用 release invariant 防止 production Go runtime 重新散落 default pack literal。
- 将 fallback deny baseline generic 化，pack-specific deny patterns 与 heavy-tool gate action 仅由 manifest 声明；Batch 151 已移除 Go manifest 对 `promoteDenyPatterns` 的隐式 fallback，schema / pack authoring 文档要求 pack 显式声明 deny baseline；Batch 152 已移除 Go manifest 对 `promoteFiles` 的 `managedFiles` 隐式 fallback，要求 pack 显式声明允许回流的 managed 文件子集；Batch 153 已移除 Go manifest 对 `budgets.defaultMarkdown` 的隐式注入，要求 pack 显式声明默认 Markdown/text 预算；Batch 154 已移除 Go manifest 对 `laneTypes.title/workspaceRoot` 的隐式默认值，要求 pack 显式声明工作线展示名、workspace 与边界字段；Batch 155 已移除 Go manifest 对 `managedBlock.file/blockId/source` 的隐式默认值，要求 pack 显式声明 managed block 三要素；Batch 156 已移除 Go manifest 对 `name` / `version` 的隐式默认值，要求 pack 显式声明 identity 字段；Batch 157 已将 `description` 纳入 schema-valid pack 的显式 metadata contract；Batch 158 已要求 `laneTypes.authority` 显式声明 true/false，避免缺失或非法值被静默解析为 false；Batch 159 已要求 `heavyToolGates.requiresConfirmation` 显式声明 true，避免 gate confirmation 被宽松 bool parser 静默降级；Batch 160 已要求 `managedFiles` / `templateFiles` / `localNeverOverwrite` 三个 required list 显式声明，允许空列表但不能缺 key；Batch 161 已要求 `schemaVersion` 显式声明为 `1`，缺失或未支持版本不再进入 schema-valid manifest；Batch 162 已将 `schemaVersion` 暴露到 pack summary、status/packs/release-check JSON 与 release handoff `schemaVersionReady`，让 manifest contract 版本 readiness 可被机器审计；Batch 163 已要求 `syncPolicy` map 本身显式声明，并对 managed/template/local 三项策略给出 key-level unsupported-value 诊断；Batch 164 已要求 `workstreamDefaults` 与 `budgets` runtime map 本身显式声明，避免工作线默认路由、backup root 或预算边界只靠空 map / runtime safety fallback 表达；Batch 165 已扩展 required list presence 记录到 `promoteFiles`、`toolingCandidateSources`、`authorityFiles`、`promoteDenyPatterns`、`heavyToolGates` 与 `laneTypes`，区分缺 key 与显式空列表；Batch 166 已继续把 `commonPolicies`、`policyOverlays`、`subagentRoutes`、`toolingFiles` 与 `promptFiles` 纳入统一 list presence contract，让可选 route/policy/tooling/prompt 列表也必须显式声明；Batch 167 已要求每个 `subagentRoutes.trigger` 显式非空，避免 route 缺少触发条件说明仍进入 schema-valid inventory；Batch 168 已要求每个 `subagentRoutes.id` 使用 namespaced id（例如 `<pack>:<route>`），避免 route 来源在跨 pack inventory 与 review packet 中变得模糊；Batch 169 已拒绝 `subagentRoutes.taskTypes`、`mainAgentOwns` 与 `outputContract` 的空列表项，避免不可消费的 route list 字段进入 schema-valid inventory；Batch 170 已要求 `subagentRoutes.id` namespace 匹配当前 pack id，避免复制 `_template` 或其它 pack route id 后仍进入 schema-valid inventory；Batch 171 已要求 `subagentRoutes.subagentPermissions` 使用 runtime 支持的显式值，避免未知权限字符串进入 schema-valid route contract；Batch 172 已要求非空 `subagentRoutes.policyOverlay` 必须来自当前 manifest 显式声明的 `policyOverlays` 列表，避免 route 引用未登记 pack 文件；Batch 173 已要求 `subagentRoutes.shardBasis` 使用小写 slug 或 `-or-` 分隔组合，避免不可消费的 shard 维度进入 schema-valid route contract；Batch 174 已要求 `subagentRoutes.taskTypes`、`mainAgentOwns` 与 `outputContract` 的列表 token 使用机器可消费的小写 slug/snake token，避免不可匹配或不可序列化的列表 token 进入 schema-valid route contract；Batch 175 已要求 `subagentRoutes.targetItemsPerAgent` 保持在 1-64、`maxParallel` 保持在 1-16，避免极端分片大小或并发上限进入 schema-valid route contract；Batch 176 已要求 `commonPolicies` 使用小写 slug 且去重，并要求 `policyOverlays`、`toolingFiles` 与 `promptFiles` 的非空项落在受支持路径范围内，避免不可消费 policy/tooling/prompt 列表项进入 schema-valid manifest；Batch 177 已要求 `heavyToolGates.sideEffects` / `stopConditions` 保留并拒绝空项、拒绝重复项，让重型工具门禁 metadata 在 schema-valid manifest 中保持可消费；Batch 178 已要求 `laneTypes.id` 使用小写 slug，并要求 `canWrite`、`readOnly` 与 `outputs` 列表项拒绝空项/重复项，`outputs` 使用机器可消费的小写 slug/snake token；Batch 179 已要求 `toolingCandidateSources`、`authorityFiles` 与 `promoteDenyPatterns` 拒绝空项/重复项，并保持 source/authority path 可定位、deny pattern 可编译；Batch 180 已要求 `managedFiles`、`templateFiles`、`localNeverOverwrite` 与 `promoteFiles` 的路径项非空、相对安全且不重复，`templateFiles` 使用 `.template.md` 源文件，`promoteFiles` 保持去重后的 managed 子集；Batch 181 已要求 `name` 使用稳定 machine id、`version` 使用 semver-like 值、`managedBlock` 仅包含 file/blockId/source 且 blockId 为 namespaced id，并要求 `syncPolicy`、`workstreamDefaults` 与 `budgets` map key 保持受支持且可消费；Batch 182 已要求 `subagentRoutes.taskTypes`、`mainAgentOwns` 与 `outputContract` 的列表 token 去重，避免 route contract 携带重复项；Batch 183 已要求 `subagentRoutes.id` 的 namespace 精确匹配当前 pack id，且 route 名为小写 slug；Batch 184 已停止 loader 对 `heavyToolGates.id/defaultRisk` 的静默小写归一化，并要求 gate id / risk 使用明确小写值；Batch 185 已要求 `heavyToolGates.stopConditions` 使用机器可消费的小写 slug/snake token，并将现有 pack manifest 的默认止损条件从人类短语迁移为 token；Batch 186 已把同一 token contract 扩展到 Go gate runtime 的用户 `-StopConditions` override，确保 pending-gate preview/request ledger 不写入人类短语式止损条件；Batch 187 已把 `medium/high/critical` 小写 risk scalar contract 扩展到 Go gate runtime 的用户 `-Risk` override，并在 gate fallback 路径复核 manifest `defaultRisk`。
- Batch 125 已先选 `generic-binary-re` 作为 pack-neutral Go package E2E 试点，Batch 126 已继续选 `web-security` 验证非 RE-only pack，Batch 127 已将 `web-security` 转向真实临时 case dry-run 工作线，Batch 132 已将 `generic-binary-re` 转向真实临时 case dry-run 工作线，Batch 133 已将 `malware-analysis` 转向真实临时 case dry-run 工作线，Batch 134 已将 `vuln-research` 转向真实临时 case dry-run 工作线，Batch 135 已将 `ctf` 转向真实临时 case dry-run 工作线，Batch 136 已将 `unpack-pe` 转向真实临时 case dry-run 工作线，Batch 137 已将 `ollvm` 转向真实临时 case dry-run 工作线，Batch 138 已将 `android-native` 转向真实临时 case dry-run 工作线；Batch 143 将 heavy-tool gate action 清单纳入 pack manifest 与 Go gate runtime，后续转向 release readiness/CI 门禁、跨 pack 抽样强化或新 pack 引入时的真实 dry-run。

完成信号：

- 非 vmp pack 的 init/doctor/plan-subagents/workstream dry-run 不泄漏 vmp 路径、lane 或 authority 语义。
- runtime 不含领域业务逻辑；default pack 决策点已在 Batch 150 收口到 Go `defaults.DefaultPack`，Batch 151/152/153/154/155/156/157/158/159/160/161/163/164/165/166/167/168/169/170/171/172/173/174/175/176/177/178/179/180/181/182/183/184/185 已取消 Go manifest 的 deny baseline、promoteFiles、syncPolicy、workstreamDefaults/budgets map、defaultMarkdown、laneTypes title/workspaceRoot/authority/list-item、managedBlock、identity、description、heavyToolGates requiresConfirmation/list-item/scalar/stop-condition-token、required/optional list presence、managed/template/local/promote file list-item、scalar/map field item、subagent route trigger/id/list-field/namespace ownership/slug/permissions/policy overlay/shard/list-token/list-duplicate/numeric-bounds contract、policy/tooling/prompt/source/authority/deny list-item contract 与 schemaVersion fallback/缺省有效状态；Batch 162 已把 schemaVersion readiness 纳入 machine-readable inventory 和 release handoff，gate action、默认/override risk scalar、默认/override stop conditions token、确认门禁、promote deny baseline、回流文件范围、sync 写入策略、默认文档预算、工作线 workspace/display/authority/backup/request 路由边界、managed block 三要素、pack identity、用途摘要、schema-critical list presence 与 schema contract version 来自 pack manifest，而不是 Go runtime 硬编码或聊天上下文。

### Stage 8：CI / release readiness / deprecation plan

目标：形成可发布、可维护、可接手的稳定门禁。

推荐批次形态：

- 建立轻量 CI：Go checks + doctor + 少量 Windows façade smoke；Batch 140 已新增 `.github/workflows/release-gate.yml`，运行 `release-check`、`go test`、`go vet`、Windows doctor 与 façade smoke；不要把大型 PowerShell matrix 作为默认必跑。
- 编写 release checklist：versioning、CHANGELOG、known gaps、PowerShell legacy status、pack maturity matrix；Batch 128 已新增 `docs/release-readiness.md` 作为一页 release gate / known gaps 入口。
- 明确 PowerShell runtime deprecation strategy：哪些模块 legacy-only、哪些命令 Go-owned、何时删除或冻结；Batch 129 已新增 `docs/powershell-deprecation.md` 记录命令归属、模块状态、freeze gates 与禁止迁移清单。

完成信号：

- 新会话能通过一页 current-state/release checklist 了解项目状态；Batch 128 已新增 `docs/release-readiness.md` 并用 Go release invariant 锁定核心章节、命令、pack matrix 和 known gaps；Batch 141 已新增 `docs/autonomous-goal.md`，把未来几十到上百轮的中大型 autonomous goal、停止条件、执行顺序和可复制 prompt 固化为接手指南。
- release gate 可在本机和 CI 中稳定执行；Batch 130 新增 Go-owned `release-check` inventory，用 JSON envelope 汇总 recommended minimum、文档入口、pack schema、边界与 known gaps，作为本机/CI release gate 的确定性前置检查；Batch 139 已进一步输出 `gateProfile`，把 recommended minimum 解析为 step kind、repo-local path、present/resolved 状态，供本机/CI 在执行前消费；Batch 140 已新增轻量 GitHub Actions release gate；Batch 143 新增 `heavyToolGateActions[]`，让 release inventory 暴露当前 pack manifest 声明的 heavy-tool gate action 集合；Batch 144 新增 `ciReleaseGate` inventory，对照 `.github/workflows/release-gate.yml` 的 required jobs、commands 与 forbidden broad/heavy steps 发现 CI 漂移；Batch 146 新增 `releaseHandoff` summary，把 read-first docs、readiness signals、latest batch、validation commands 与 next actions 放入 release-check envelope，提升新会话和维护者接手质量；Batch 147 新增 `releaseHandoff.releaseNotes` freshness gate，确保最新完成 batch 已进入 CHANGELOG `Unreleased`；Batch 148 新增 `releaseHandoff.knownGaps[]`，把 release readiness Known gaps 转为机器可读 category/summary 供新会话和 release maintainer 先读；Batch 149 新增 `releaseHandoff.packMaturity`，汇总 pack maturity、schema validity 与每 pack heavy-tool gate readiness。
- PowerShell runtime deprecation strategy 有明确文档入口、模块归属和 freeze/removal gates；Batch 129 已新增 `docs/powershell-deprecation.md`，Batch 131 已新增 Go release invariant 锁定默认 façade 委托集合、blocked heavy-tool/authority/confirmed 不进入默认委托；Batch 142 已将 PowerShell deprecation inventory 纳入 Go-owned `release-check`，对照 `rekit/rekit.ps1` 默认委托集合、命令归属矩阵、`rekit/lib/*.ps1` 模块清单、freeze gates 与 blocked migrations 输出 `powerShellDeprecation.ready`；Batch 190 已将 `plan-subagents` review artifacts 纳入默认 Go façade 并保留 fallback；实际删除仍需单独批次验证。

## 自主推进优先级建议

若没有用户进一步指定，优先顺序为：

1. Stage 1：Go-first release gate 与 invariant migration。
2. Stage 2：低风险 Go default façade。
3. Stage 3：case lifecycle 写入路径 Go 化。
4. Stage 4：sync/promote 收口。
5. Stage 5：workstream/ledger/gate/continue Go 化。
6. Stage 6：Agent Team dry-run 闭环。
7. Stage 7：pack-neutral hardening。
8. Stage 8：CI/release readiness/deprecation plan。

每个阶段可以拆成多个中大型批次；每批完成后更新 `docs/batch-plan.md` 的最新状态，必要时更新本文件的完成信号。