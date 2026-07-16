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
- Pack manifest / doctor / inventory 改动：`go test ./...`、`go run ./cmd/rekit -- -Command doctor`、必要时 `go run ./cmd/rekit -- -Command packs`。
- PowerShell façade / fallback 委托改动：按需运行 façade compatibility smoke；它不属于默认 release gate。
- Existing PowerShell smoke 维护：只维护既有 smoke/parity，不新增大编排；必要时运行 `rekit/tests/README.md` 指定的最小相关组合。
- init/attach/sync/promote/workstream/ledger/gate 改动：使用临时 case 验证 no-real-artifact、backup、review-first、containment、no confirmed/authority unless explicitly allowed。
- Agent Team 闭环改动：验证临时 case 中 packet、ledger events、gate request、handoff 和新会话可接手信息完整。

Release readiness 目标门禁应逐步收束为：

```text
go run ./cmd/rekit -- -Command release-check -Format json
go run ./cmd/rekit -- -Command status
go run ./cmd/rekit -- -Command packs
go run ./cmd/rekit -- -Command doctor
go test ./...
go vet ./...
git diff --check
```

迁移期 façade / fallback compatibility smoke 只在改对应兼容层时按需追加，不作为默认 release readiness 目标门禁。

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
- PowerShell façade 仍是公共入口；低风险只读、case lifecycle、sync/promote、overview 文本/JSON 与缺 board 初始化、note 文本/JSON 只读查询、note append/what-if、gate what-if/apply request、start/handoff JSON preview、显式 apply 与文本/default 工作线 flow、continue JSON preview、explicit apply 与文本/default preview、plan-subagents review artifact 路径已逐步默认委托 Go；普通 Go-default command rows 已不再回落 PowerShell。
- Batch 92-101 已把 pack smoke helper/matrix/discovery/tests guide/catalog metadata 收束成维护体系。
- Agent Team 契约、ledger 基础、review-first sync/promote、Go review artifacts、Go gate preview/request、Go `plan-subagents` 只读计划器已经存在。

主要缺口：

- Go-native backend 仍需继续取代 PowerShell 作为公共默认 runtime/public entrypoint，而不仅是 façade 委托目标。
- PowerShell 与 Go 在 manifest、sync/promote、workstream/ledger 等领域仍有历史双实现残留与删除批次风险。
- authority/confirmed gate、batch-level replay/resume 等仍有 Go/PowerShell 语义分裂或 façade 暴露不足。
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

- `status`、`packs`、`doctor`、`validate` 默认委派 Go；Batch 224 后这些 read-only 命令的 PowerShell fallback 已退休，`REKIT_GO_DISABLE=1` 不再回落到 PowerShell 业务实现。
- 更新 façade smoke，覆盖默认 Go、已退休 read-only no-fallback、剩余 candidate disable fallback 与错误回退语义。
- 文档标明 Go read-only runtime 已成为默认路径。

完成信号：

- 用户不设置 `REKIT_GO_ENABLE` 也走 Go read-only runtime。
- PowerShell 对这些命令不再作为业务逻辑 owner。

当前进度：Batch 104 已完成低风险 read-only default façade：`status`、`packs`、kit/case `doctor/validate` 默认委托 Go；Batch 113 将已初始化 case 的 `overview -Format json` 与 `note -List -Format json` 只读查询也纳入默认 Go façade；Batch 114 将 `gate -WhatIf` 非写入 heavy-tool gate preview纳入默认 Go façade；Batch 115 将 `gate -Apply` pending-gate request ledger 写入纳入默认 Go façade；Batch 117 将 attached case 的 `start`/`handoff` JSON preview 与显式 `-Apply` 纳入默认 Go façade；Batch 118 将 attached case 的 `continue -WhatIf -Format json` 非写入 preview 纳入默认 Go façade；Batch 119 将 attached case 的 `overview` 文本/JSON 与缺 board 初始化纳入默认 Go façade；Batch 120 将 attached case 的 `note -List` 文本/table/tsv 查询也纳入默认 Go façade；Batch 121 将 explicit `continue -Apply` 的 case-local facts/routing/run digest/resume/board 写入纳入默认 Go façade；Batch 224 将 `status`、`packs`、`doctor` 与 `validate` 的 PowerShell fallback 退休，和 `release-check` 一起进入 no-fallback Go-owned baseline；Batch 225 退休 `plan-subagents` fallback，Batch 226 退休 `gate` fallback，Batch 227 退休 `overview` 与 `note` fallback，Batch 228 退休 `sync` 与 `update` fallback，Batch 230 退休 case lifecycle `attach` / `repair` / `init` / `bootstrap` fallback；`REKIT_GO_DISABLE=1` 不再让这些 Go-owned no-fallback 命令回落到 PowerShell 业务实现；Batch 232 后 `start`、`handoff` 与 `continue` 的文本/default 工作线 flow 也进入 Go-owned no-fallback baseline，fallback retirement candidateCommands 清零；Batch 233 进一步把 `rekit.ps1` legacy runtime module dot-source 移到 Go delegation 与 no-fallback guard 之后，使 default delegation / disabled no-fallback 路径不预加载 retired PowerShell modules；Batch 234 删除已不可达的 legacy dot-source block 与 command switch fallback dispatcher，使公共 façade 只保留 Go delegation、no-fallback guard 和 retired dispatcher error。Stage 2 的核心完成信号已达成，后续转向 removal-candidate PowerShell module 独立删除/归档批次、public Go-native entrypoint 或 blocked migration 设计，而不是继续按已清空的 `candidateCommands[]` 寻找普通 fallback。

### Stage 3：case lifecycle 写入路径 Go 化

目标：迁移边界清晰的写入命令，为后续 sync/promote/workstream 收口铺路。

推荐批次形态：

- 让 `attach`、`repair`、`init/bootstrap` apply 在明确参数/确认边界下走 Go。
- 用临时 case 验证 metadata、thin shim、managed docs、template files、state、doctor。
- 删除或降级 PowerShell 对应业务逻辑为 legacy fallback。

完成信号：

- 新 case 创建/绑定的 deterministic 写入由 Go 负责。
- case-local shim 仍保持 thin，不复制 runtime。

当前进度：Batch 105 已完成第一组 case lifecycle default façade：`attach`、`repair`、`init/bootstrap` 的非写入预览与显式 `-Apply` 写入默认委托 Go；Batch 230 已退休 case lifecycle PowerShell fallback，`REKIT_GO_DISABLE=1`、裸 lifecycle 命令或 Go delegation 不可用时直接 no-fallback。Go `attach` 语义补齐 PowerShell binding baseline，会写 `.rekit/instance.yml`、legacy `.re-template.yml`、初始 `.rekit/state.json` 与 thin shim；Go 委托路径下 `attach`、`init/bootstrap` 仍要求显式 `-WhatIf` 或 `-Apply`。Stage 3 的核心完成信号已基本达成，后续可继续做 case lifecycle parity hardening，或进入 Stage 4 收口 sync/promote review/apply。

### Stage 4：sync/promote review/apply 收口

目标：统一 kit → case 与 case → kit 的 review-first/apply/candidate 行为到 Go。

推荐批次形态：

- Batch 106 已完成第一步：`/rekit sync` review、`sync -Apply` 实际写入与 JSON apply preview 默认走 Go；Batch 228 已退休 `sync` / `update` PowerShell fallback，`REKIT_GO_DISABLE=1` 与文本 dry-run 均报 no-fallback error。
- Batch 107 已完成 promote 非写入默认 façade：`/rekit promote` review、review artifacts、`promote -CreateCandidates -WhatIf -Format json` 与 `promote -Apply -WhatIf -Format json` 默认走 Go。
- Batch 108 将 `promote -CreateCandidates` 实际候选写入纳入默认 Go façade，继续保留文本 what-if fallback、`REKIT_GO_DISABLE=1` fallback 与当时的 `promote -Apply` 实际 pack source 写入 fallback/manual 边界。
- Batch 112 将 `/rekit promote -Apply` 实际 pack source 写入纳入默认 Go façade，仍要求显式 `-Apply`、禁止与 `-CreateCandidates`/review artifacts 混用，并依赖 backup/deny/validation/restore/package tests/smoke。
- Batch 229 退休 `promote` PowerShell fallback；文本 promote what-if、disabled JSON preview 与 `REKIT_GO_DISABLE=1` promote apply 均报 no-fallback error。
- Batch 109 开始把 promote candidates/apply 中可单元化的 sanitize、candidate index、containment/no-write、restore helper 迁到 Go package tests；Batch 110 将 sync apply/preview 的 backup、managed block、template force、state refresh 与 backup escape guard 迁入 Go package tests；Batch 111 将 promote apply 的 what-if no-write、backup/validation rows、blocked deny 与 validation failure restore 迁入 Go package tests。

完成信号：

- sync/promote 不再长期维护 Go/PowerShell 双实现。
- review artifact 语义在文档中明确：不写 managed/authority/pack 内容，但可写 `.rekit/reviews/**` review artifacts。

当前进度：Batch 106 已将 sync 侧第一组收口为默认 Go façade：`/rekit sync` review、`sync -Apply` 实际写入、`sync -Apply -WhatIf -Format json` 非写入 preview 默认委托 Go；Batch 228 已退休 `sync` / `update` PowerShell fallback，`REKIT_GO_DISABLE=1` 与文本 `sync -Apply -WhatIf` 均报 no-fallback error。Batch 107 已将 promote 非写入路径收口为默认 Go façade：`/rekit promote` review、review artifact 写入、`promote -CreateCandidates -WhatIf -Format json` 与 `promote -Apply -WhatIf -Format json` 默认委托 Go。Batch 108 进一步将 `promote -CreateCandidates` 实际候选写入纳入默认 Go façade。Batch 109 开始把 promote candidates/apply 的可单元化写入边界迁入 `internal/rekit/promote` package tests；Batch 110 继续把 sync apply/preview 的可单元化写入边界迁入 `internal/rekit/sync` package tests；Batch 111 继续把 promote apply 的 what-if no-write、backup/validation rows、blocked deny 与 validation failure restore 迁入 `internal/rekit/promote` package tests。Batch 112 将 `promote -Apply` 实际 pack source 写入也纳入默认 Go façade；Batch 229 退休 `promote` PowerShell fallback，使文本 promote what-if 与 disabled promote façade 均进入 no-fallback guard。Stage 4 后续优先处理剩余 fallback candidate、PowerShell runtime 删除策略，或继续把尚未覆盖的 parity/deny/restore 边界收进 Go tests。

### Stage 5：workstream / ledger / gate / continue Go 化

目标：让 Agent Team 日常状态流从 PowerShell runtime 收口到 Go。

当前进度：Batch 113 先把无需新增写入面的 `overview -Format json` 与 `note -List -Format json` 只读查询升级为默认 Go façade 委托；Batch 114 将 `gate -WhatIf` 非写入 heavy-tool gate preview 升级为默认 Go façade；Batch 115 将 `gate -Apply` pending-gate request ledger 写入升级为默认 Go façade，并补 `internal/rekit/gate` package tests 覆盖 no-write、actor guard、幂等和 no authority/confirmed/artifacts；Batch 116 将 attached case 的 `note` append 与 `note -WhatIf` facts JSONL 写入/预览升级为默认 Go façade，并补 `internal/rekit/note` package tests 覆盖 no-write、schema/lane guard、幂等和 no authority/confirmed；Batch 117 将 attached case 的 `start`/`handoff` JSON preview 与显式 `-Apply` 升级为默认 Go façade；Batch 118 将 attached case 的 `continue -WhatIf -Format json` 非写入 preview 升级为默认 Go façade；Batch 119 将 attached case 缺 board 时的 overview case-local board/facts/policy/default authority lane 初始化迁入 Go，并让 overview 文本/JSON 默认经 façade 委托 Go；Batch 120 将 attached case 的 `note -List` 文本/table/tsv 只读查询也纳入默认 Go façade；Batch 121 将 explicit `continue -Apply` 纳入默认 Go façade，写 case-local facts/routing/run digest/lane resume/checkpoint 与 board refresh，同时将 authority/confirmed 写入强制 defer；Batch 122 新增 `_template` pack 的 Go CLI package E2E 测试，覆盖 start → note → continue apply → lane/project handoff 的 case-local 闭环；Batch 123 继续新增 `_template` pack 的 Go CLI package E2E 测试，覆盖 start → plan-subagents review artifact → gate apply pending-gate request → overview → lane handoff 的 bounded dispatch / heavy-tool gate 可见性链路；Batch 124 新增 `_template` pack 的 Go CLI package E2E 测试，覆盖 candidate note → reviewer verification → main decision → note list → overview → handoff 的 reviewer verdict / merge decision 可见性链路；Batch 125 新增 `generic-binary-re` Go CLI package E2E 测试，覆盖 non-feature `binary-analysis` lane、pack-specific route、candidate/gate/verification/decision、overview 与 handoff 的 pack-neutral 闭环；Batch 126 新增 `web-security` Go CLI package E2E 测试，覆盖非 RE-only Web/API `feature` lane、`web-security:feature-analysis` route、network pending-gate、candidate/verification/decision、overview 与 handoff 的 pack-neutral 闭环；Batch 127 新增 `web-security-agent-team-dryrun-smoke.ps1`，通过公共 `/rekit` façade 与真实临时 case 跑通同一 Web/API Agent Team dry-run 链路；Batch 189 让 Go-owned overview 文本/JSON 输出 `missionBrief`，并让 lane handoff 写出 `## Mission Control brief`，把 ready/blocked lanes、pending gates、open decisions、interventions、next agent actions 与 escalations 汇总成主 Agent / 新会话可直接消费的 Mission Control 状态；Batch 191 对齐 lane handoff 与 overview 的 blocker 语义，确保 pending gate、open intervention、open candidate/decision 都会让对应 lane 显示为 blocked，并补 decision-only/candidate-only parity 覆盖；Batch 192 将同一 Mission Control brief 扩展到项目级 handoff 索引，让新会话从 `.rekit/handovers/latest.md` 也能先看到 ready/blocked lanes、pending gates、open decisions、interventions、next agent actions 与 escalations。Batch 193 将 Mission Control brief 聚合逻辑抽入 `internal/rekit/mission`，overview、project handoff 与 lane handoff 复用同一 blocker、line item、next action 与 escalation helper，降低后续 open decision / gate / intervention 语义再次漂移的风险。Batch 194 将同一 brief 暴露到 Go `handoff` JSON envelope，项目级与指定 lane 的 `-WhatIf/-Apply -Format json` 输出都包含结构化 `missionBrief`，自动化可直接消费 handoff preview/apply 结果而不必解析 Markdown。Batch 195 将同一 brief 扩展到 Go `continue` JSON envelope 与 run artifacts，`continue -WhatIf/-Apply -Format json`、`.rekit/runs/<run-id>/status.json` 和 `digest.md` 都能暴露 apply 前/后的 Mission Control 状态，lane executor 继续后不必另跑 overview/handoff 才能看到 ready/blocked lanes、open decisions、gate/intervention 和 next actions。Batch 196 将同一 brief 扩展到 Go `start` JSON envelope，`start -WhatIf/-Apply -Format json` 在创建或进入 lane 前后直接输出 `missionBrief`，让自动化在新 lane 初始化后立即获得全局 Mission Control 状态。Batch 197 将同一 brief 扩展到 Go `gate` preview/apply JSON envelope，`gate -WhatIf/-Apply -Format json` 可直接显示 pending-gate request 写入前后的 Mission Control 状态，而实际 heavy-tool 仍不执行。Batch 198 将 case-local Mission Control snapshot 读取逻辑抽入 `internal/rekit/mission`，让 `start`、`handoff` facts loading 和 `gate` 复用 shared board/facts JSONL reader、`CaseBrief` 与 board lane adapter，降低后续 `missionBrief` envelope 在 pending-gate、open intervention、open candidate/decision blocker 语义上的漂移风险。Batch 199 继续让 Go `overview` 复用 `mission.ReadBoard`、strict ledger facts reader、pending decision/batch aggregation 与 board lane label helper，移除 overview-local board/facts JSONL 与 label 复制逻辑。Batch 200 继续让 Go `note` 复用 mission shared fact-file mapping、strict facts JSONL reader 与 board lane guard，移除 note-local JSONL reader、board struct 与 fact-file switch 复制逻辑。Batch 201 将 note/gate 的 board lane guard 进一步收口到 mission shared helper，复用 known-lane 列表、missing/empty board 诊断与大小写敏感/不敏感 lookup 语义。Batch 202 继续让 workstream board type/readBoard/open-lane filter 复用 mission shared board snapshot helpers，移除 start/handoff/continue 共享路径上的 board struct、JSON parser 与 open-lane status filter 复制逻辑。Batch 203 继续让 workstream facts 初始化、handoff facts 读取、continue known event id 扫描与 shared facts preview/apply 路径复用 mission shared ledger kind/file/path helpers，移除共享 facts 文件名列表漂移点。Batch 204 继续让 workstream handoff/continue 的 facts snapshot 复用 `mission.ReadLedgerFacts` / `mission.LedgerFacts`，删除 handoff-local facts struct 与 adapter，让 handoff Markdown、handoff JSON 和 continue Mission Control brief 共享同一 ledger snapshot reader。Batch 205 继续让 gate duplicate eventId 检查与 workstream lane/workspace JSONL 读取复用 `mission.ReadJSONLineObjects` / `mission.Value`，移除 gate-local scanner/unmarshal 与 workstream-local passthrough wrapper，进一步压缩 Go runtime 内 JSONL reader 漂移面。Batch 206 继续把 case doctor 的 JSONL validation 收口到 `mission.ValidateJSONLines` / shared scanner，移除 doctor-local scanner/unmarshal，让 strict validation 与 reader 共用同一 BOM、missing-file 和 line iteration 基础。Batch 207 继续让 Go `note` 的 ledger kind 顺序、kind validation、duplicate eventId scan 与 append facts path 复用 `mission.LedgerKinds` / `mission.FactRelPath`，并让 doctor facts validation 遍历 `mission.FactRelPaths`，移除 note-local ledger kind list 与 doctor-local facts file list。Batch 208 继续将 workstream lane-local / workspace-local JSONL 文件集合收口到 `internal/rekit/workstream` helper，让 `start` 初始化、`continue` lane output/input refs 与 `doctor` lane/workspace JSONL validation 复用同一 local JSONL path source，同时保持这些 files 与 shared `.rekit/facts/*.jsonl` ledger helpers 的边界清楚。Batch 209 继续补齐 workstream lane/workspace JSONL path helpers，让 `start` 的初始化/event/resume、`continue` 的 request routing 与 duplicate scan、以及 `doctor` validation 直接复用 `LaneJSONLPaths` / `WorkspaceJSONLPaths` / lane-local single-path helpers，进一步移除 events/tasks/inbox/outbox path 拼装漂移点。Batch 210 继续将 workstream `continue` 的 local event -> shared facts ledger promotion helper 化，preview/apply 的 facts would-write 与 append 路径统一经 `mission.FactRelPath(kind)`，删除 continue-local append closure 和散落 fact path 拼装。Batch 211 继续把 `continue` duplicate eventId scan 收口到 `mission.ReadLedgerEventIDs`，让 preview/apply skip 已知 event 使用 shared facts file list、non-strict reader 和 eventId extraction helper，删除 continue-local `.rekit/facts` scanner。Batch 212 继续把 `note` append duplicate eventId scan 收口到 `mission.ReadStrictLedgerEventIDs`，让 note 保持 strict malformed JSONL 报错语义的同时复用 shared facts file list 和 eventId extraction helper，删除 note-local full ledger scan。Batch 213 继续把 note/gate/workstream 的 JSONL append 写入收口到 `mission.AppendJSONLine`，统一 JSON marshal、append flags 与 CRLF 行写入，删除 note/gate-local append boilerplate 和 workstream-local append helper。Batch 214 继续把 shared facts path 与 append 收口到 `mission.FactPath` / `mission.AppendFact`，让 note append、gate pending-gate request 与 continue facts promotion 复用同一 fact kind → path、safe join、parent mkdir 与 JSONL append helper，删除各命令的 fact path/mkdir/append 并行实现。Batch 215 继续把 shared facts read 与 per-kind eventId scan 收口到 `mission.ReadFact` / `mission.ReadStrictFact` / `mission.ReadFactEventIDs`，让 ledger snapshot、note list strict read 与 gate duplicate scan 复用同一 fact path helper 和 strict/non-strict reader 语义。Batch 231 进一步退休 `start` / `handoff` / `continue` 已 Go-owned structured invocation 的 disabled/unavailable fallback：JSON preview 与 explicit apply 在 `REKIT_GO_DISABLE=1` 或 Go delegation 不可用时直接 no-fallback，仍保留无 `-Apply` 文本工作线 compatibility。Batch 232 将文本/default 工作线 flow 也委托 Go text output 并退休 whole-command PowerShell fallback，使 `start` / `handoff` / `continue` 全部进入 no-fallback baseline；actual heavy-tool 执行、authority/confirmed 写入与 policy schema 迁移仍需单独 gate。

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

- 建立轻量 CI：Go checks + Go-native status/packs/doctor + tests/vet；Batch 140 已新增 `.github/workflows/release-gate.yml`，运行 `release-check`、`go test`、`go vet`、Windows doctor 与 façade smoke；Batch 217 将默认 CI 收敛为 Linux/Windows/macOS 三平台 Go-native release checks，不再把 PowerShell façade smoke 作为默认必跑；不要把大型 PowerShell matrix 作为默认必跑。
- 编写 release checklist：versioning、CHANGELOG、known gaps、PowerShell legacy status、pack maturity matrix；Batch 128 已新增 `docs/release-readiness.md` 作为一页 release gate / known gaps 入口。
- 推进 PowerShell-free / Go-native convergence：哪些模块 legacy-only、哪些命令 Go-owned、何时替换、冻结或删除；Batch 129 已新增 `docs/powershell-deprecation.md` 记录命令归属、模块状态、freeze gates 与禁止迁移清单，Batch 216 将该文档升级为 PowerShell-free / Go-native / 跨平台收敛路线，后续默认路径应逐步不依赖 PowerShell。

完成信号：

- 新会话能通过一页 current-state/release checklist 了解项目状态；Batch 128 已新增 `docs/release-readiness.md` 并用 Go release invariant 锁定核心章节、命令、pack matrix 和 known gaps；Batch 141 已新增 `docs/autonomous-goal.md`，把未来几十到上百轮的中大型 autonomous goal、停止条件、执行顺序和可复制 prompt 固化为接手指南。
- release gate 可在本机和 CI 中稳定执行；Batch 130 新增 Go-owned `release-check` inventory，用 JSON envelope 汇总 recommended minimum、文档入口、pack schema、边界与 known gaps，作为本机/CI release gate 的确定性前置检查；Batch 139 已进一步输出 `gateProfile`，把 recommended minimum 解析为 step kind、repo-local path、present/resolved 状态，供本机/CI 在执行前消费；Batch 140 已新增轻量 GitHub Actions release gate；Batch 143 新增 `heavyToolGateActions[]`，让 release inventory 暴露当前 pack manifest 声明的 heavy-tool gate action 集合；Batch 144 新增 `ciReleaseGate` inventory，对照 `.github/workflows/release-gate.yml` 的 required jobs、commands 与 forbidden broad/heavy steps 发现 CI 漂移；Batch 146 新增 `releaseHandoff` summary，把 read-first docs、readiness signals、latest batch、validation commands 与 next actions 放入 release-check envelope，提升新会话和维护者接手质量；Batch 147 新增 `releaseHandoff.releaseNotes` freshness gate，确保最新完成 batch 已进入 CHANGELOG `Unreleased`；Batch 148 新增 `releaseHandoff.knownGaps[]`，把 release readiness Known gaps 转为机器可读 category/summary 供新会话和 release maintainer 先读；Batch 149 新增 `releaseHandoff.packMaturity`，汇总 pack maturity、schema validity 与每 pack heavy-tool gate readiness；Batch 217 将 recommended minimum 与 CI required commands 改为 Go-native release-check/status/packs/doctor/test/vet/diff，并用 CI inventory 禁止默认 workflow 漂移回 `rekit.ps1` 或 `facade-smoke.ps1`；Batch 218 将 case-local thin shim readiness 纳入 Go `release-check` / `doctor`，防止 case shim 再次出现 PowerShell 或 raw CLI 默认入口；Batch 219 将 README、canonical `/rekit` skill、CLAUDE 与 autonomous goal 的默认公开入口纳入 `publicDefaultDocs` readiness 和 release handoff，防止公开文档漂移回 PowerShell façade 命令路径；Batch 220 继续把 release readiness 与 PowerShell / pwsh shell fence drift 纳入 public docs readiness；Batch 221 将 Mission Control 产品方向、Go-first plan 与 PowerShell deprecation roadmap 纳入同一 readiness envelope，避免路线文档重新把 PowerShell façade 命令或 shell fence 变成默认路径；Batch 222 继续把 Go runtime migration history、vision、reference absorption、Agent Team rollout 与 tests guide 纳入 public default docs readiness，确保支撑文档和 smoke 选择指南也保持 Go-native 默认验证路径。
- PowerShell-free / Go-native convergence 有明确文档入口、模块归属和 freeze/removal gates；Batch 129 已新增 `docs/powershell-deprecation.md`，Batch 131 已新增 Go release invariant 锁定默认 façade 委托集合、blocked heavy-tool/authority/confirmed 不进入默认委托；Batch 142 已将 PowerShell deprecation inventory 纳入 Go-owned `release-check`，对照 `rekit/rekit.ps1` 默认委托集合、命令归属矩阵、`rekit/lib/*.ps1` 模块清单、freeze gates 与 blocked migrations 输出 `powerShellDeprecation.ready`；Batch 190 已将 `plan-subagents` review artifacts 纳入默认 Go façade，Batch 225 已退休其 PowerShell fallback；Batch 226 已退休 `gate -WhatIf` / `gate -Apply` pending-gate PowerShell fallback；Batch 228 已退休 `sync` / `update` PowerShell fallback；Batch 216 将后续方向调整为 PowerShell replacement/removal、Go-native validation、macOS/Linux 默认路径和跨平台 release readiness，实际删除按中大型可验证 batch 推进；Batch 218 继续把 case shim PowerShell-free default path 固化为 Go-owned readiness inventory，作为后续 fallback retirement 的前置信号之一；Batch 219 将公开默认文档路径也固化为 Go-owned readiness inventory，确保删除/收缩 fallback 前 README、skill、CLAUDE 与 autonomous goal 不再把 PowerShell façade 命令作为默认入口；Batch 220/221 继续扩大 public docs readiness 覆盖到 release readiness、Mission Control 产品方向、Go-first plan 与 deprecation roadmap，Batch 222 继续覆盖 Go runtime migration、vision、reference absorption、Agent Team rollout 与 tests guide，并锁定这些路线/支撑文档不再用 PowerShell shell fence 或 façade 命令片段表达默认路径；Batch 223 在 `powerShellDeprecation` 下新增 `fallbackRetirement` 子库存，机器可读地区分 Go-default commands、no-fallback commands、fallback removal candidates、blocked commands 与 removal-candidate modules，作为后续 PowerShell fallback removal batch 的确定性前置检查；Batch 235 新增 `powerShellDeprecation.facadeRuntime` 子库存，把 `rekit.ps1` 不再加载 legacy modules、不再包含 command switch fallback dispatcher、仍保留 Go delegation / no-fallback guard / retired dispatcher error 转为 release-check JSON/text 与 release handoff 信号；Batch 236 新增 `powerShellDeprecation.moduleRemoval` 子库存，把 removal-candidate PowerShell module 清单、公共 façade 依赖和未登记模块转为 release-check JSON/text 与 release handoff 信号；Batch 237 新增 `powerShellDeprecation.moduleReferences` 子库存，把 PowerShell module 引用面分类为 active test dependencies、compatibility fixtures、inventory guards、文档/历史引用、removal blockers 与 unclassified references；Batch 238 将 continue preflight 迁到 Go-owned package-test wrapper；Batch 239 将 façade smoke 的 legacy sentinel fixture 收敛为 source invariant + Go delegation/no-fallback checks，并在 release-check text 与 release handoff 中锁定 `activeTests=0 fixtures=0 blockers=0 unclassified=0`；Batch 240 物理删除 `rekit/lib/*.ps1` legacy runtime modules，保留公共 `rekit.ps1` façade，并把 release inventory 基线收敛为 `removalModules=0 retiredModules=13` / `moduleRemoval=true candidates=0 retired=13 facadeDeps=0 undocumented=0`；Batch 241 新增 `powerShellDeprecation.publicFacade` retained inventory，锁定公共 façade 仍存在、19 个 façade command 均为 Go-default/no-fallback、底层 Go-native 替代路径和未来 façade 删除必须独立批次处理；Batch 242 新增顶层 `goNativePublicSurface` inventory 与 `internal/rekit/commands` public command catalog，锁定 `cmd/rekit` entrypoint、19 个 Go-native public commands、unsupported command diagnostic 和 public façade command surface 双向一致；Batch 243 继续扩展该 inventory，新增 `handlerCommands[]` 与 `symbolCommands{}` coverage gate，要求 Go CLI dispatcher handlers、public command symbols 与 catalog 同步覆盖 19 个 public commands；Batch 244 新增 `commandProfiles[]` 与 `mutationBoundaries[]`，把 19 个 public commands 的 read-only、case-local 写入、review-first、kit review-first 等 mutation boundary 纳入 release inventory，并禁止 public profile 标记 actual heavy-tool 或 authority/confirmed 写入；Batch 245 新增 `commandProfileSummary`，把 public command profile catalog 汇总为 `total=19`、`readOnly=5`、`mutating=14`、`writesCase=13`、`writesKit=1`、`reviewFirst=3`、`applyRequired=11`、`heavyTool=0` 与 `authorityConfirmed=0`，并同步 release-check text 与 release handoff signals；Batch 246 新增 `commandProfileGroups`，把 profile summary counts 反向展开为具体 command sets 和 per-boundary groups，让 release inventory 可直接检查 read-only、review-first 与 kit-write 命令集合；Batch 247 新增 `commandProfileBoundaries[]`，把 7 个 mutation/read-only boundary 升级为一等 JSON rows，并与 summary counts 和 groups 互相校验；Batch 248 新增 `commandProfilePolicies[]`，把 public profile guardrails 固化为 5 个 policy rows，要求 no-heavy-tool、no-authority/confirmed、kit-write-review-first、review-first-apply-required 与 known-boundary 策略 violation count 均为 0；Batch 249 新增 `facadeRemovalReady` 与 `facadeRemovalPrerequisites[]`，把 entrypoint、catalog/handler/symbol/profile coverage、mutation boundary inventory、profile policy guards 与 unsupported-command diagnostic 固化为未来公共 façade 删除前置门禁；Batch 250 新增顶层 `publicFacadeRemoval` inventory，跨 `powerShellDeprecation` 与 `goNativePublicSurface` 汇总 public façade retained/boundary documented、façade commands Go-default/no-fallback、Go-native surface ready、legacy runtime detached、legacy module removal settled 与 module reference blockers clear 六条 prerequisites；Batch 251 继续新增 `publicFacadeRemoval.removalPlan` 与 `removal-plan-documented` prerequisite，锁定后续独立 removal batch 必须具备替代入口、恢复计划、验证命令、文档同步、CHANGELOG 与 no PowerShell runtime / no heavy-tool / no authority 边界；Batch 252 继续新增 `publicFacadeRemoval.removalImpact` 与 `removal-impact-inventoried` prerequisite，扫描并分类 `rekit/rekit.ps1`、`rekit.ps1` 与 `facade-smoke.ps1` 引用面，要求 façade 删除前 impact references/categories 可见且 unclassified references 为空；Batch 253 继续新增 `removalImpact.workItems[]`，把每个 impact category 映射到 required action、count 与 paths，确保后续 removal batch 不只知道影响面数量，也知道逐类处理动作；Batch 254 继续为每个 work item 新增 `validationCommands[]`，把后续 removal batch 的逐类处理动作与 Go-native release gate 验证命令绑定；Batch 255 继续在 `removalPlan.recoverySteps[]` 中固化 separate revertable commit、restore public façade、restore synchronized docs 与 rerun Go-native release gate 的机器可读恢复计划；Batch 256 继续新增 `removalPlan.documentationTargets[]`，把未来 removal batch 必须同步的 README、CLAUDE、canonical `/rekit` skill、case shim、release readiness、PowerShell deprecation、Go-first plan、batch-plan 与 CHANGELOG 固化为机器可读文档目标；Batch 257 继续新增 `removalImpact.smokeMigrationTargets[]`，把 façade compatibility smoke 与 façade-dependent smoke 的 Go-native preferred / retire façade assertions 迁移目标固化为机器可读清单；Batch 258 继续新增 `removalImpact.migrationTargets[]`，把全部 public façade references 转为 required / Go-native preferred / validationCommands 绑定的机器可读迁移目标，并对 roadmap/history docs 保留 historical context 边界；Batch 259 继续新增 `removalPlan.executionSteps[]`，把 verify Go-native alternative、migrate public references、retire façade smoke、delete public façade 与 rerun release gate 的 dependencies / input inventory / output artifacts / validation commands / no PowerShell runtime / no external effects 边界固化为机器可读执行步骤；Batch 260 继续新增 `removalPlan.boundaryChecks[]`，把 no PowerShell runtime logic、no actual heavy-tool、no authority/confirmed write、sync/promote review-first、case-local write semantics 与 no external effects 六个 preserved required boundaries 固化为机器可读边界检查并同步 release-check / handoff counts；Batch 261 继续新增 `removalPlan.replacementEntrypoints[]`，把 canonical `/rekit` skill、case-local thin shim、direct Go CLI 与 cross-platform release gate 四个 replacement entrypoints 固化为机器可读替代入口清单并同步 release-check / handoff counts；Batch 262 继续新增 `removalPlan.deletionGates[]`，把 go-native-alternatives-ready、public-references-migrated、facade-smoke-retired、recovery-path-ready 与 release-gate-green 五个 required / blocking deletion gates 固化为机器可读删除门禁并同步 release-check / handoff counts；Batch 263 继续为每个 deletion gate 新增 `exitCriteria[]`，把删除前必须满足的 15 条可判定退出条件纳入 release-check / handoff counts；Batch 264 继续为每个 deletion gate 新增 `verificationArtifacts[]`，把未来 removal batch 删除前必须产出或检查的 15 条证据项纳入 release-check / handoff counts；Batch 265 继续为每个 deletion gate 新增 `blockedExecutionSteps[]`，把五个 gate 对 delete-public-facade / rerun-release-gate 的 10 条阻断关系纳入 release-check / handoff counts；Batch 266 继续为每个 deletion gate 新增 `remediationActions[]`，把 gate 未满足时先执行的 15 条修复动作纳入 release-check / handoff counts；Batch 267 继续为每个 deletion gate 新增 `failureSignals[]`，把 gate 未满足时可观测的 15 条失败信号纳入 release-check / handoff counts；Batch 268 继续为每个 deletion gate 新增 `escalationTriggers[]`，把 gate 未满足且不可本地修复时必须提升到主 Agent / 用户决策的 15 条触发条件纳入 release-check / handoff counts；Batch 269 继续为每个 deletion gate 新增 `escalationEvidence[]`，把触发升级时必须附带的 15 条证据要求纳入 release-check / handoff counts；Batch 270 继续为每个 deletion gate 新增 `escalationRecipients[]`，把触发升级时应该路由到的 15 条责任接收者纳入 release-check / handoff counts；Batch 271 继续为每个 deletion gate 新增 `escalationHandoffSteps[]`，把触发升级时必须执行的 15 条交接步骤纳入 release-check / handoff counts；Batch 272 继续为每个 deletion gate 新增 `escalationDecisionOptions[]`，把触发升级后可选的 15 条决策选项纳入 release-check / handoff counts；Batch 273 继续为每个 deletion gate 新增 `escalationRetryConditions[]`，把升级后允许重试的 15 条条件纳入 release-check / handoff counts；Batch 274 继续为每个 deletion gate 新增 `escalationStopConditions[]`，把升级后必须停止而不能重试的 15 条条件纳入 release-check / handoff counts；Batch 275 继续为每个 deletion gate 新增 `escalationResolutionArtifacts[]`，把升级收束后必须记录的 15 条决策/证据产物纳入 release-check / handoff counts；Batch 276 继续为每个 deletion gate 新增 `escalationClosureChecks[]`，把升级关闭前必须执行的 15 条检查纳入 release-check / handoff counts；Batch 277 继续为每个 deletion gate 新增 `escalationReopenConditions[]`，把升级关闭后必须重新打开的 15 条条件纳入 release-check / handoff counts；Batch 278 继续为每个 deletion gate 新增 `escalationLedgerEvents[]`，把升级过程必须记录的 15 条账本事件纳入 release-check / handoff counts，但这几批仍不删除公共 `rekit/rekit.ps1` façade。

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