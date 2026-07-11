# Changelog

## Unreleased

### Added

- 新增仓库根目录 `CLAUDE.md`，提供 Claude Code 维护本 kit/template/runtime 仓库的入口说明、分层改动边界和验证命令。
- 新增 `docs/vision.md`，定义网络安全研究 / 安全工程 Agent Team 框架定位、模块边界和后续阶段实施方案；`vmp-re` 是首个成熟 pack。
- 新增 `references/vmp-re/agent-driven-re.md`，定义 VMP RE Agent Team 的角色、packet、candidate→confirmed 流程和人工门禁。
- 在 `docs/vision.md` 中新增批次执行协议，明确后续可分批实施、自审调整、需停下询问的边界和计划写回文档要求。
- 新增 `common/policies/agent-team.md` 与 `common/policies/tool-adapters.md`，沉淀跨 pack Agent Team 和外部工具 adapter 通用契约。
- 新增 `docs/agent-team-usage.md`，说明新架构使用方式、旧 case 兼容、主线/功能支线工作流和后续优化空间。
- 新增 `docs/reference-absorption.md`，映射参考文章、`ida-agent-bridge`、`clark-utov` 的吸收点、当前落地能力和后续优化项。
- 新增 `docs/pack-authoring.md`、`docs/evidence-ledger.md`、`docs/orchestration-plan.md` 和 `docs/batch-plan.md`，分别记录 pack 编写、证据账本、半自动编排和后续批次计划。
- 新增 `packs/_template/` pack 作者骨架，用于后续创建 `web-security`、`malware-analysis`、`vuln-research`、`ctf`、`unpack-pe`、`android-native`、`ollvm` 等新 pack。
- 增强 `doctor` 的 manifest、policy overlay、case thin shim、board/lane 和 JSONL 校验，作为后续 runtime 架构调整的安全网。
- 为 `packs/_template` 补齐最小 policy overlay registry，使模板 pack 可通过 pack validation。
- 将 B3 工作线默认主线、默认 start 类型、长期 handoff 路径、sync backup root、authority files 和 request 默认路由改为 manifest 驱动，减少 `vmp-re` 硬编码。
- 新增 Go runtime 迁移方案文档，明确 PowerShell façade + Go deterministic backend 的渐进迁移路线。
- 新增 Go read-only runtime skeleton，支持手动运行 `status`、pack `doctor/validate` 与 manifest/policy validation，并补充 manifest/CLI/runtime guard tests；未实现 case doctor 时显式拒绝 case target。
- 新增 Go `sync/promote` review-only plan skeleton，先输出非写入 JSON plan，并拒绝 `sync -Apply`、`promote -CreateCandidates/-Apply` 等写入路径。
- 新增 `docs/agent-team-rollout-plan.md`，记录 Agent Team 当前"契约已固化、编排未落地"的真实状态，并按选项 C（契约 dry-run 优先）给出 R0-R7 分批实施计划与 R3 决策门。
- 在 README、CLAUDE.md、`docs/vision.md` 执行清单与候选清单中接入 Agent Team rollout 计划入口。
- 新增 `docs/agent-team-dryrun-script.md`，定义 R0 端到端契约压测脚本（mock 对象 + S1-S5 五步 + 字段缺口记录区），并在 kit 仓库外创建临时 case `agent-team-dryrun` 供 R1 执行。
- Agent Team rollout R1-R7 完成：R1 契约压测暴露 6 类缺口；R2 回写 `common/policies/agent-team.md`（evidence_id、decision event 字段、packet-vs-event 关系、lane id 规范化）、`evidence.md`、`handoff.md`（workspace packet 引用）、`subagents.md`（L1 tool_scope）；R3 决策 ledger runtime 优先；R4 新增内部 `note` 命令与 `Add-RekitFactEvent`，让 agent 手动 append observation/candidate/decision/request/publication 到 `.rekit/facts/*.jsonl`，overview 聚合手写事件，handoff 增加 workspace packet 引用区段，overview pending 计数对齐 decision event；R5 判定不扩 runtime（spawn 是主会话职责）；R6 heavy-tool gate 写入 vmp-re verification overlay。
- 修复 `B3.Core.ps1` `Join-RekitRelativePath` 在 Windows PowerShell 5.1 下因 `[System.IO.Path]::GetRelativePath` 不存在而崩溃的问题，改用 PS 5.1 兼容的路径前缀比较实现。
- 在 `docs/agent-team-rollout-plan.md` 追加 §4 Kit runtime hardening 计划：B1 `note` schema 校验、B2 overview 未决/pending/冲突明细、B3 handoff 引用 decision event，作为 R0-R7 后续优化路线。
- Kit runtime hardening B1-B3 完成：B1 `Add-RekitFactEvent` 加 confidence/decision/status 枚举校验、evidenceRefs 非空校验、lane 存在性校验（`Invoke-RekitNote` 传 Board）；B2 `overview` 增加"未决 candidate"（含同 subject 冲突标记）和"pending-gate"明细区段，限 N=10，超出提示"另有 N 条"；B3 `Write-RekitLaneHandoff` 增加 `## decision` 区段，列 latest 5 条 decision 摘要。临时 case 验证非法值被拒、合法值写入、overview 明细正确、handoff 含 decision 区段。
- Kit runtime hardening B5-B7 完成：B5 `Write-RekitLaneHandoff` 增加 `## pending-gate` 区段，decision 区段兼容 `action` 字段（auto `continue` 写的 decision 用 `action`，`note` 用 `decision`，两套并存）；B6 `overview` 增加"最近 decision"明细区段，兼容 `action` 字段；B7 `Invoke-RekitNote` 加 `-List` 查询模式，按 kind/lane 过滤列出事件，限 N=20。临时 case 验证 handoff 含 decision+pending-gate、overview 含最近 decision、`note -List` 全部/按 kind/按 kind+lane 过滤正确。
- 在 `docs/agent-team-rollout-plan.md` 追加 §5 Ledger schema 对齐与架构治理（C 系列）：基于 `docs/evidence-ledger.md` 草案审查发现 runtime ledger 字段偏离草案（自定 decision 枚举、缺 actor/risk/related/candidateId/claim/verifier/confirmedBy/writes、缺 4 种 kind、auto 用 action 字段），按草案 Runtime 落地顺序分 C1-C8 八批对齐，兼容历史 JSONL 不迁移。
- Agent Team rollout C1 完成：`orchestration-plan.md` O2 标注 bounded dispatch 已被 rollout R5 否决（保留设计参考）；`vision.md` §8 候选清单第 7 条同步 `packs/_template` 与 `B3.Lane` 修复已完成、后续按 batch-plan/rollout §4-§5 推进；`rollout-plan.md` §"当前状态"段同步 R0-R7 已完成状态；`.claude/skills/rekit/SKILL.md` 命令清单补 `note`（含 `-List` 查询，标注为账本写入/查询入口与 schema 校验）。纯文档批次，未动 runtime。
- Agent Team rollout C2 完成：`Add-RekitFactEvent` 基础字段对齐 `docs/evidence-ledger.md` 草案——补 `-Actor`/`-Risk`/`-Related` 参数与 `schemaVersion: 1`/`actor`/`risk`/`related` 字段；`validDecision` 改 `accept|reject|defer|supersede`（`confirm` 写入被拒）；`validStatus` 取草案 6 值与 runtime 已落地 3 值的并集（`open|accepted|rejected|superseded|resolved|deferred|pending-gate|confirmed|needs_more_evidence`），保留 `pending-gate`/`confirmed`/`needs_more_evidence` 不破坏 R6 heavy-tool gate 契约（`packs/vmp-re/policies/verification.overlay.md` 用 `note -Kind request -Status pending-gate` 登记 gate、`overview`/`handoff` 读层过滤 `pending-gate`）；`Invoke-RekitNote` 补 param + 透传新参数；`overview` `terminalStatus` 补 `accepted`/`resolved`。临时 case 验证 accept 写入、confirm 被拒、accepted status、pending-gate 保留、actor/risk/schemaVersion 字段落地、历史 JSONL 仍可读、overview 不崩。
- Agent Team rollout C3 完成：补齐 `docs/evidence-ledger.md` 草案定义的全部 9 种事件类型——`Add-RekitFactEvent`/`Get-RekitFactFilePath` ValidateSet 扩 `hypothesis`/`verification`/`intervention`/`rollback`；`Get-RekitBoardPaths` 补 4 路径属性（`Hypotheses`/`Verifications`/`Interventions`/`Rollbacks`）；`Ensure-RekitBoard` 初始化 4 个新空 JSONL；`Validate.ps1` `Test-RekitWorkstreamState` 与 `B3.Auto.ps1` `Get-RekitKnownEventIds` 校验/去重循环同步补 4 文件；verification kind 加 `-TargetRef`/`-Verifier`（manual-review|schema-check|focused-trace|parity|cross-run|tool-review）/`-Verdict`（accepted|rejected|inconclusive|needs-more-evidence）扩展字段与校验，intervention kind 加 `-Action`（override|rollback|heavy-tool-approval|schema-migration|external-side-effect）/`-ApprovedBy`/`-Scope`/`-Expires` 扩展字段与校验；`Invoke-RekitNote` param 补扩展字段并透传，`note -List` 展示 verification/intervention/candidate-risk/decision-confirmedBy 字段。临时 case 验证 4 新文件初始化、5 种新 kind 写入、非法 verdict/action 被拒、扩展字段落地、`note -List` 展示新字段。
- Agent Team rollout C4 完成：`New-RekitDecision`（`B3.Auto.ps1`）字段对齐 `docs/evidence-ledger.md` 草案——`action` 字段改 `decision`（值映射 `auto-publish`/`auto-route`/`auto-apply-authority`/`auto-accept-shared`→`accept`，`pending-user`/`defer`→`defer`），补 `schemaVersion:1`/`confirmedBy:runtime`/`subject`/`summary`，authority 写入时附 `writes`（file/backup/diff），`extra` 保留给非 authority 场景；9 处 `Add-RekitJsonLine -Path $paths.Decisions` 调用保留直接写（原方案"改走 `Add-RekitFactEvent` 统一写入路径"因 eventId 去重会破坏 auto 流程对同一 eventId 写两条记录的设计而调整）；读层（overview/handoff/note-List）已兼容 `action`/`decision` 双字段（B5/B6 实现）。临时 case `continue` smoke 验证新 decision event 含 `decision:accept`/`confirmedBy:runtime`，历史 `action` 字段 decision 仍可读，overview "最近 decision" 兼容展示。
- Agent Team rollout C5 完成：展示层对齐草案新字段——`overview` "最近 decision" 显示 `by=<actor|confirmedBy>`（actor 优先、fallback confirmedBy）；`Write-RekitLaneHandoff` `## decision` 区段显示 `by=<confirmedBy|actor>`；`note -List` decision 展示从 `confirmedBy=` 改为 `by=` 并 fallback `actor`（`note` 写的 decision 用 `-Actor`，auto 写的用 `confirmedBy`，统一展示为 `by=`）；verification/intervention/candidate-risk 展示已在 C3 落地。临时 case 验证 overview/handoff/note-List 三处 `by=main` 显示正确，历史无 actor/confirmedBy 的事件 `by=` 为空（append-only 不回填）。
- Agent Team rollout C6 完成：policy 文档去重，建立 single source of truth——`evidence.md` 改为 `agent-team.md` 的证据原则补充（顶部声明不重复定义 packet schema，packet/event 字段指向 `agent-team.md` 与 `docs/evidence-ledger.md`，更新 `note`/`continue` 入口与 9 种 kind 引用）；`handoff.md` 补 `## decision event 引用` 小节（指向 `evidence-ledger.md` decision schema，`by` 字段 confirmedBy 优先 fallback actor、`## pending-gate` 区段）；`tool-output.md` 内容并入 `tool-adapters.md` "输出契约"小节（含报告格式模板），`tool-output.md` 改为 deprecated 指向，`policies/manifest.yml` title 标 deprecated（保留注册避免外部引用断链）；`subagents.md` 补 overlay 可 extend `decision` 枚举说明 + ledger decision 与 reviewer output contract `needs_l2|needs_l3` 是不同层的澄清；`review-first.md`/`write-boundaries.md` 顶部互引（review-first 偏流程、write-boundaries 偏边界）。doctor 验证 policy registry 与 budget 不回归。
- Agent Team rollout C7 完成：handoff 子系统独立成 `rekit/lib/B3.Handoff.ps1`，含 `Get-RekitLatestRunDigestPath`/`Write-RekitProjectHandoff`/`Write-RekitLaneHandoff`/`Write-RekitHandoff` 4 函数（从 `B3.Commands.ps1` 机械移动，行为不变）；`rekit.ps1` dot-source `B3.Handoff.ps1`（在 `B3.Commands` 前）；`B3.Commands.ps1` 从 534 行降到 339 行，聚焦用户级命令入口。`B3.Handoff.ps1` 用 UTF-8 with BOM 写入（含中文，PS 5.1 解析需 BOM，与 `B3.Commands.ps1`/`B3.Auto.ps1` 一致）。临时 case 验证 `handoff main` 与项目级 handoff 生成不回归、handoff 文件区段（新会话开场/推荐读取/边界）正确、doctor 双通过。
- Agent Team rollout C8 完成（C 系列收尾）：`docs/reference-absorption.md` 候选清单勾选 evidence ledger runtime 项（行 37 改 `[x]`，heavy-tool gate runtime/bounded dispatch 加注当前形态），更新"当前已落地/尚未落地"描述、clark-utov 映射表（runtime ledger 9 种 kind + decision 字段对齐草案，batch 模型与 intervention 强制门禁待实现）、能力表（ledger 草案 → runtime 已落地）、"还没落地"清单（ledger runtime 已落地，batch/intervention 待实现）、ledger 最小实现注（已完成 9 种 kind 文件 + note/continue 写入 + overview/handoff/note-List 读层）。C 系列自审：C1-C8 八批全部完成，runtime ledger 字段集与枚举对齐 `docs/evidence-ledger.md` 草案，读层兼容历史 `action`/`confirm`/`pending-gate` 值，append-only 不迁移旧事件，policy single source of truth，handoff 子系统独立，B3.Commands.ps1 降到 339 行。最终验证 `doctor` 双通过、`go test ./...` 通过、`git diff --check` 通过。
- Agent Team rollout D 系列启动：`docs/agent-team-rollout-plan.md` 新增 §6 batch / intervention / gate 闭环路线，并完成 D1 post-merge sanity（main 状态、B3.Handoff.ps1 UTF-8 BOM、dot-source 顺序、case shim thin boundary 与验证安全网均通过）。
- Agent Team rollout D2 完成：ledger event 增加可选 `batchId` 字段；`note` 支持 `-BatchId` 写入与 `note -List` batch 展示；`continue` 自动派生 `batch-<runId>` 并写入 event/decision/publication/digest；overview 新增“最近 batch”跨 9 种 fact JSONL 聚合；rollback/intervention 可通过 `TargetRef batch-...` 指向整批事件。历史 JSONL 不迁移、无 `batchId` 事件正常读取。
- Agent Team rollout D3 完成：overview 增加“未解决 intervention”“最近 intervention”“最近 rollback”读层；lane handoff 增加 `## intervention` 与 `## rollback` 区段；`note -List` 对 intervention/rollback 展示 target/action/approvedBy/scope/status/reason/batch。该批只增强展示闭环，不自动执行 heavy-tool、不自动回滚文件、不迁移历史 JSONL。
- Agent Team rollout D4 完成：按 Go backend 优先方向新增 `internal/rekit/gate` 与 Go CLI `-Command gate -WhatIf`，输出非写入 heavy-tool gate JSON plan（`eventPreview.kind=request`、`status=pending-gate`、`requiresConfirmation=true`、含 action/scope/budget/triedLightSteps/stopConditions/target/batchId），校验 attached case + lane id；不默认接入 PowerShell façade、不写 ledger、不执行 heavy-tool。
- Go runtime G2.3 完成：`-Command gate -Apply` 显式 append `.rekit/facts/requests.jsonl` pending-gate request，要求 `-Actor`，与 `-WhatIf` 互斥；eventId 由 gate 语义字段派生，重复写入返回 `applied=false`/`duplicate eventId`。该路径只写 gate request，不执行 heavy-tool、不写 confirmed/authority，也暂不列入 PowerShell 默认委托安全集合。
- Go runtime G2.1 完成：`sync/promote` review-only backend 增加 `-ReviewOutputDir` / `-PacketPath` / `-DiffPath` artifact 写入，输出 `packet.json`、`summary.md`、bounded diff 与 promote sanitized preview；该路径只写 review artifact，`isMutation=false`、`writesArtifacts=true`，不写 managed docs、pack 或 candidates。
- Go runtime G2.4 完成：`rekit.ps1` 增加显式 Go façade 委托开关（`REKIT_GO_ENABLE` / `REKIT_GO_DISABLE` / `REKIT_GO_EXE`），仅覆盖安全集合：`status`、kit doctor/validate、`sync/promote` review-only artifact、`gate -WhatIf` dry-run；默认仍走 PowerShell，不委托写入命令、case doctor、`gate -Apply`、`note` 或工作线命令。
- Go runtime G2.5 完成：Go `doctor/validate` 增加 attached case 只读校验，覆盖 instance/legacy metadata、case-local shim parity、managed/template files、managed block、facts JSONL、board/lane/workspace JSONL；PowerShell façade 显式启用 Go 时也可委托 case doctor。
- Go runtime G2.6 完成：新增 `rekit/tests/facade-smoke.ps1`，覆盖默认不委托、显式 Go 安全集合、`REKIT_GO_DISABLE` 优先级、`sync` 写入 flags 回退 PowerShell、`gate -WhatIf` 非写入委托与未启用时拒绝。
- Go runtime G3.1 完成：新增 `internal/rekit/attach` 与 Go CLI `-Command attach`，支持 `-WhatIf` 非写入预览和 `-Apply` 只写 `.rekit/instance.yml` + case-local thin shim；不写 managed docs、legacy metadata、state、board/facts/lanes，也暂不纳入 PowerShell façade 委托。
- Go runtime G3.2 完成：新增 `internal/rekit/casebind` 与 `internal/rekit/repair`，支持 Go CLI `-Command repair` 默认/`-WhatIf` 非写入预览和 `-Apply` 刷新 `.rekit/instance.yml`、`.re-template.yml`、case-local thin shim；不写 managed docs、board/facts/lanes 或 authority，也暂不纳入 PowerShell façade 委托。
- Gate request schema parity 完成：PowerShell `overview`、lane `handoff`、`note -List -Kind request` 现在展示 Go `gate -Apply` 写入 pending-gate request 的 `actor/risk/target/batchId/gate{action,scope,budget,triedLightSteps,stopConditions}` 字段；新增 `rekit/tests/gate-parity-smoke.ps1` 覆盖 Go 写入 + PowerShell 三处读层展示。
- G3.3 `sync -Apply` 迁移预研完成：新增 `docs/sync-apply-migration.md` 固化 PowerShell 写入语义、Go 迁移契约与 S1-S18 测试矩阵；新增 `rekit/tests/sync-review-parity-smoke.ps1` 验证 PowerShell/Go sync review action 与 bounded diff parity，仍未实现 Go 写入。
- G3.4 Go `sync -Apply` 手动路径完成：Go CLI 显式 `-Command sync -Apply` 可刷新 metadata/shim/legacy metadata、同步 managed files、处理 template create/skip/`-Force`、更新 managed block、写 `.rekit/state.json` 并报告 backup/writes；新增 `rekit/tests/sync-apply-smoke.ps1` 覆盖临时 case apply、backup、state、Go/PowerShell doctor。该写入路径仍不经 PowerShell façade 委托。
- G3.5 Go `init/bootstrap` 手动路径完成：Go CLI 显式 `-Command init|bootstrap -WhatIf` 输出非写入预览，`-Apply` 可创建临时 case 的 metadata/shim/legacy metadata、managed files、template files、managed block 与 `.rekit/state.json`；新增 `docs/init-bootstrap-migration.md` 和 `rekit/tests/init-bootstrap-smoke.ps1` 覆盖 preview 无副作用、apply、`-Force`、Go/PowerShell doctor 与 façade fallback。该写入路径仍不经 PowerShell façade 委托。
- G3.6 `promote -CreateCandidates` 迁移预研完成：新增 `docs/promote-candidates-migration.md` 固化 PowerShell candidate 写入、tooling sanitization、deny pattern 与 Go 迁移契约；新增 `rekit/tests/promote-candidates-preflight-smoke.ps1` 验证 PowerShell `-WhatIf -CreateCandidates` baseline、Go promote review artifact/sanitized preview parity、Go 写入 guard 与 façade fallback。本批不写 pack candidates，不实现 Go `promote -CreateCandidates` 写入。
- G3.7 Go `promote -CreateCandidates` 手动路径完成：新增 `promote.CreateCandidates` helper 与 CLI JSON result，支持 `-WhatIf` 非写入预览和显式 candidate/index/tooling candidate 写入；新增 `rekit/tests/promote-candidates-apply-smoke.ps1` 覆盖临时 case candidate 写入、blocked deny、tooling sanitization、pack-root containment、cleanup 与 façade fallback。该路径仍不经 PowerShell façade 委托，不实现 `promote -Apply`，不覆盖 pack managed docs。
- G3.8/G3.9 `promote -Apply` 迁移完成：`docs/promote-apply-migration.md` 固化 PowerShell apply baseline、backup/deny/validation/cleanup 语义与 Go 迁移契约；Go backend 新增 `promote.Apply` helper 与 CLI `-Apply/-Apply -WhatIf` JSON result，支持非写入 preview、safe managed docs backup 后写回 pack source、blocked deny 不写、写入后 pack validation；新增 `rekit/tests/promote-apply-preflight-smoke.ps1` 与 `rekit/tests/promote-apply-smoke.ps1` 覆盖 PowerShell baseline、Go apply、backup、pack-root containment、cleanup 与 façade fallback。该路径仍不经 PowerShell façade 委托，不写 authority/confirmed，不执行 heavy-tool。
- G4.1 Go `overview` 只读路径完成：新增 `internal/rekit/overview` renderer 与 CLI `-Command overview`，读取既有 `.rekit/board.json` 与 9 类 facts JSONL，输出工作线、共享事实、未决 candidate、pending-gate、decision、batch、intervention、rollback 和下一步建议；缺 board 时拒绝并提示先用 PowerShell overview 初始化。新增 `rekit/tests/overview-readonly-smoke.ps1` 验证只读、缺 board guard、Go gate request 展示字段和 façade fallback；公共 PowerShell façade 仍不委托工作线命令。
- G4.2 Go `start` 手动路径完成：新增 `internal/rekit/workstream` start helper 与 CLI `-Command start -WhatIf/-Apply`，支持非写入预览、显式初始化 board/facts/policy/default authority lane、创建或进入 feature lane、刷新 lane resume/checkpoint 与 board；新增 `rekit/tests/start-apply-smoke.ps1` 覆盖 preview 只读、apply scaffold、existing/force、Go/PowerShell doctor 与 façade fallback。公共 PowerShell façade 仍不委托工作线命令，不写 authority/confirmed，不执行 heavy-tool。
- G4.3 Go `handoff` 手动路径完成：新增 `workstream` handoff helper 与 CLI `-Command handoff -WhatIf/-Apply`，支持非写入预览、显式写项目级/工作线级 handoff、刷新 lane resume/checkpoint，并展示 workspace packet、decision、pending-gate、intervention、rollback 区段；新增 `rekit/tests/handoff-apply-smoke.ps1` 覆盖 preview 只读、apply 输出、Go/PowerShell doctor 与 façade fallback。公共 PowerShell façade 仍不委托工作线命令，不写 authority/confirmed，不执行 heavy-tool。
- G4.4 Go `plan-subagents` 手动路径完成：新增 `internal/rekit/subagents` plan helper 与 CLI `-Command plan-subagents`，支持 route/taskType 选择、`Items`/`ItemsFile` 分片、`ItemsPerAgent`/`MaxParallel` override，以及 review packet/summary artifact 写入；新增 `rekit/tests/plan-subagents-smoke.ps1` 覆盖 attached case、out-of-case guard、missing routes、Go/PowerShell doctor 与 façade fallback。公共 PowerShell façade 仍不委托该内部命令，不启动 subagent、不写 board/facts/lanes/authority/confirmed，不执行 heavy-tool。
- Agent Team review loop Batch A 启动：收敛通用 contract，明确 reviewer output decision 与 ledger decision 分层，canonical decision enum 使用 `accept|reject|defer|supersede`，历史 `confirm`/`confirmed`/`action` 仅作为读层兼容；`agent-team.md` 更新 `/rekit note` 手动 append 与 `/rekit continue` 自动抽取两条 facts event 路径，`evidence-ledger.md` 补 request/heavy-tool gate 扩展字段与 `needs_more_evidence` → `needs-more-evidence` 归一化说明。
- Agent Team review loop Batch B 启动：同步 VMP managed docs 与 manifest route；`agent-driven-re.md` 补 canonical contract 指向、`evidence_id`、runtime normalized lane id、`output_contract`、accepted decision 与 confirmed/authority 写入分层；`vmp-re` 两条 `subagentRoutes.outputContract` 补 `tier_used,tool_scope`，对齐通用 subagent contract。
- Agent Team review loop Batch C 完成：新增 `rekit/tests/agent-team-review-loop-smoke.ps1`，覆盖 `plan-subagents` review packet、`note -Kind verification`、`note -Kind decision`、`note -List`、`overview` 与 lane `handoff` 的最小闭环；该 smoke 只使用临时 case，不启动 subagent、不写 confirmed/authority、不执行 heavy-tool。
- Agent Team review loop Batch D 完成：PowerShell 与 Go `overview` 现在展示最近 verification，lane `handoff` 增加 `## verification` 区段；相关 smoke 与 Go CLI fixtures 覆盖 reviewer verdict 到 main decision 的可见性。该批只增强读层展示，不写 confirmed/authority、不执行 heavy-tool。
- Agent Team review loop Batch E 完成：`/rekit continue` digest 升级为结构化摘要，记录 inputs、route、packet refs、outputs、decisions 与 open risks，并在 `status.json` 写入对应索引字段；新增 `rekit/tests/continue-digest-smoke.ps1` 覆盖临时 case。该批不启动 subagent、不写 confirmed/authority、不执行 heavy-tool。
- Agent Team review loop Batch F 完成：重新评估 Go `note` / `continue` 迁移，结论是下一步优先 Go `note` 手动路径（先 `-List` 只读，再 append），`continue` 因 authority append、routing、digest/status 与 lane/board 刷新副作用继续暂缓到 G5 gate/parity tests 完整后。
- G4.5a Go `note -List` 只读路径完成：新增 `internal/rekit/note` 与 CLI `-Command note -List`，读取 9 类 facts JSONL，支持 `-Kind`/`-Lane` 过滤并展示 candidate/request/decision/verification/intervention/rollback 关键字段；该路径只读、不写 board/facts/lanes/handoff/authority/confirmed，不纳入 PowerShell façade 委托。
- G4.5b Go `note` append 手动路径完成：CLI `-Command note` 可显式 append 9 类 ledger event 到 `.rekit/facts/*.jsonl`，支持 PowerShell 对齐 enum/schema 校验、lane guard、eventId dedupe 与 `-WhatIf` 非写入预览；该路径只写 facts JSONL，不写 board/lane/handoff/authority/confirmed，也不纳入 PowerShell façade 委托。
- G5 preflight baseline 完成：新增 `rekit/tests/continue-preflight-smoke.ps1`，覆盖 PowerShell `continue` 的 authority append gate matrix（evidence、accepted verifier、confidence、CSV schema、conflict、max rows、allowlist）、backup/bounded diff、CSV 失败恢复、request routing 幂等、digest/status parity 与 `-WhatIf` no-write，作为后续 Go `continue -WhatIf` 迁移前测试网。
- G5 Go `continue -WhatIf` 预览路径完成：新增 `internal/rekit/workstream/continue.go` 与 CLI `-Command continue -WhatIf`，读取既有 board/lane/outbox/workspace 输出非写入 JSON preview，包含 inputs、packet refs、收集事件、routing/authority 决策预览、wouldWrites 与 blocked actions；新增 `rekit/tests/continue-whatif-smoke.ps1` 覆盖临时 case 预览、no-write、unsupported apply guard 与 PowerShell façade fallback。该路径不写 facts/run/board/lane/authority/confirmed，不纳入 façade 委托，不执行 heavy-tool。
- Batch 58 bounded dispatch 可观测性增强：Go 与 PowerShell `plan-subagents` review packet/summary 增加 `observability` 与 `reviewLoop`，记录 route 选择原因、review artifact 路径、shard 初始 `planned` 状态、blocked runtime actions、spawn/merge owner、verdict writeback 和 completion criteria；`plan-subagents` smoke 覆盖 Go 与 PowerShell fallback artifacts。该路径仍不启动 subagent、不写 board/facts/lanes/handoff/authority/confirmed。
- Batch 59 项目定位纠偏：将顶层定位从 RE-only 修正为面向网络安全研究与安全工程任务的 Claude Code Agent Team 框架，明确 `vmp-re` 是首个成熟 pack / 验证场而非最终边界，并同步 README、CLAUDE.md、vision、reference absorption、usage、pack authoring、skill、case shim、design 与 pack template 描述。
- Batch 60 完成：`packs/_template` 新增 managed `agent-team.md` 与两条 pack-neutral `subagentRoutes`，让新安全领域 pack 复制模板后即可通过 `plan-subagents` 生成 bounded review packet；Go manifest schema 与 PowerShell doctor 增加 subagent route 必填字段、正整数分片、reference 边界和 route id 唯一性校验，smoke 覆盖 Go 与 PowerShell fallback 的 `_template` route packet。
- Batch 61 启动：新增首个非 RE pack 骨架 `packs/web-security`，覆盖 Web/API 安全评估的 scope、Agent Team routes、toolchain router、tooling catalog、passive triage 与 bounded request replay recipe，并补 smoke 验证 Go/PowerShell doctor、init 与 `plan-subagents` route packet。

### Fixed

- 修复 Windows PowerShell 5.1 解析含非 ASCII 的 runtime `.ps1` 文件时可能因 UTF-8 无 BOM 产生 mojibake 并破坏语法的问题。
- 修复裸 `attach` 后执行 `sync` review 时，空 `CLAUDE.local.md` host text 被 PowerShell 参数绑定拒绝的问题，并统一空白 host 下 sync review 与 `sync -Apply` 的 managed block 写入结果。
- 修复 G3.7 promote candidates Go 单测与 apply smoke 的清理策略：对已有 `promote-candidates/index.json`、tooling candidates 和原始目录结构做 snapshot/restore，避免验证覆盖人工候选索引或残留空 backup/candidate 目录污染 pack。

### Verified

- 使用临时 case 完成 `init/status/doctor/sync/promote` 与 `attach/status/sync/sync -Apply/doctor/promote` smoke test，验证 case-local 边界与 review-first 流程。

### Changed

- 更新 README 顶部定位，将项目说明从单纯 context kit 扩展为面向网络安全研究与安全工程任务的 Claude Code Agent Team 框架，并区分维护本仓库与接入安全 case（当前以 `vmp-re` 为例）的入口。
- 扩展 `vmp-re` 工作流与工具路由，加入轻到重分析路线、重型工具升级门禁和 `ida-agent-bridge` 候选工具说明。
- 将日常 `/rekit` 工作流收敛为 `overview / continue / start / handoff`，移除公开的 `board / auto / lane / policy` 旧入口。
- 将原集中式 B3 PowerShell runtime 拆分为 `B3.Core/State/Policy/Lane/Auto/Commands` 模块，并新增项目级 handoff 生成。
- 更新 README、skill、case shim、policy/reference/prompt 文档，用户层统一使用“工作线 / 主线 / 功能支线”术语。
- 明确 `/rekit overview` 只是项目总览，`/rekit continue main|<name>` 才选择工作线；`/rekit handoff` 改为项目级接手索引，并新增指定工作线 handoff。

## 0.2.0 - 2026-06-12

### Added

- 新增目录级 canonical `/rekit` skill，clone 后在 kit 仓库内直接可用。
- 新增 `rekit/rekit.ps1` runtime 与 Manifest / Instance / Sync / Promote / Validate 模块。
- 新增 case-local `/rekit` shim 模板与 `.rekit/instance.yml` / `state.json` 实例模型。
- 新增 `promote` 工作流，用于将 case 中可复用 managed docs 生成候选或显式写回 pack。
- 新增 `packs/vmp-re/tooling/`，保存工具 catalog、recipes、脚本模板化清单、补丁/止损经验和 promote 候选。
- 新增 `docs/promote-sync.md`。
- 新增 manifest 路径 root containment 检查，避免 managed/promote/tooling 路径越出 case 或 pack 根目录。

### Changed

- `packs/vmp-re/manifest.yml` 升级为 sync/promote/managed block/tooling/budget 的单一事实源。
- `sync/promote/doctor` 对显式 case target 增加 attached-case guard，避免拼错路径时隐式创建假 case 或从普通目录回流。
- `promoteDenyPatterns` 收紧为覆盖 artifact/capture/trace/dump 路径与更通用 ctx/round 状态。
- `bootstrap.ps1`、`update.ps1`、`validate.ps1` 改为兼容 wrapper，转调 `rekit/rekit.ps1`。
- README 与 design 文档改为以 `/rekit`、runtime、pack、instance 四层模型说明，并去除具体 case 路径示例。

## 0.1.0 - 2026-06-12

### Added

- 初版 `vmp-re` pack。
- 按需路由、渐进式披露、工具链路由、singleton handler 复核模板。
- `bootstrap.ps1` / `update.ps1` / `validate.ps1`。
- 四层目录模型：`kits/`、`cases/`、`tools/`、`shared-artifacts/`。
