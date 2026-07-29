# Autonomous Mission Control convergence goal guide

## 读取指南

本文件是给新会话和上下文压缩后的 AI 使用的**简短接手锚点**，不是给维护者人工阅读的长方案，也**不是新的限制清单**。

如果用户已经在聊天里给出 goal，以用户聊天里的 goal 为准；若聊天摘要与仓库文档冲突，以仓库文档为准。本文件用于防止方向偏移：继续把 `re-context-kits` 收敛为 **Lane-centric Agent Team Mission Control**，并把当前阶段重点切到 **PowerShell-free / Go-native / 跨平台** convergence。用户已确认仍希望用 goal 长期推进，但该 goal 应按 milestone 自校准：持续推进不等于无限寻找字段、summary 或投影微调。最终产品北极星见 `docs/mission-control-product-direction.md`，具体路线写回 `docs/batch-plan.md`、`docs/go-first-convergence-plan.md`、`docs/powershell-deprecation.md` 与 `docs/release-readiness.md`。

后续每批不需要过度拆小，也不要只做一两行微调。默认做一个中大型、能验证、能降低真实维护风险或提升实际可用性的 vertical slice。当前节奏校准：不要再连续推进单字段 contract / inventory / metadata 微批次；也不要连续推进字段、summary、handoff detail、text line 在不同 envelope 间的可见性投影。每批必须是用户或 Mission Commander 能感知的 operational slice。若最近 2-3 批都停留在同一子系统的 contract/projection/handoff text 层，下一批必须升级为 Mission Commander run loop、reviewer/session orchestration、adapter-specific live validation、pack-memory UX 或可维护性收敛中的一个完整闭环。若确实需要新增 contract 字段或投影 detail，必须嵌入 Mission Commander orchestration、replaceable session executor、reviewer dispatch/intake/writeback E2E、authorized execution evidence closure、adapter-specific live validation、pack-memory promote/reconsume product UX 或跨平台 product-path E2E，并由 package / CLI / 临时 case / product-path 验证证明其解决真实断点。

## 实施摘要

长期目标保持不变但阶段重点已更新：`re-context-kits` 应成为 Claude Code 中的多会话 Agent Team Mission Control 框架，而不是命令大全；Go 已是 public command surface 的 deterministic owner。Batch 359 后，主 Agent实际 spawn read-only reviewer → Go-native strict intake/writeback、本机 bounded reviewer E2E、pack-memory promote/reconsume package E2E，以及 authorized execution observation evidence + bounded adapter execution report strict intake/contract projection/read-only validation preflight（含 invalid sidecar `valid=false` envelope/failure taxonomy 与 sidecar boundary/escalation marker fail-closed validation）均已形成底座；runtime 仍不自动 spawn 或管理 session/reviewer，也不执行 heavy-tool。当前重点是 replaceable session executor / reviewer orchestration、lane executor / tool-adapter live validation hardening、pack-memory product UX、PowerShell-free product path 与跨平台验证，而不是继续扩 contract/inventory 字段。

核心产品形态：

- 用户主要和主 Agent / Mission Commander 交互。
- 主 Agent 调度 durable member lanes，而不是绑定某个旧聊天窗口。
- 每个 lane 由可替换 Claude Code session executor 执行；旧会话上下文污染、模型变化或用户希望重开时，新会话可读取 handoff / packet / evidence 接手同一 lane。
- 主 Agent 可启动短命 tactical subagents 做搜索、验证、review、小修和 bounded implementation。
- 用户可随时进入任意 lane 打断、纠错、改向、硬切模型或要求新会话接手；当前 `continue` 对 open intervention fail-closed 并要求显式 `reconcile` 写入 durable state，目标是让 Mission Commander 自动发现并准备安全 resolution。
- lane 文档 / task packet 只表达授权意图；只有 strict validated `.rekit/lanes/<lane>/autonomy.json` 加 `authorized-gate` decision 完全覆盖 action、exact target、typed budget、stop conditions、output paths、record/notify 和 grant/expiry 时，executor 才可不逐步询问地执行 heavy-tool、动态调试、patch、dump、hook、网络、exploit replay；越界、新风险、confirmed/authority 或 pack promote 必须升级。
- Go backend 是 canonical runtime owner，负责状态、ledger、gate、release inventory、sync/promote、pack-neutral contract 和跨平台路径；PowerShell 只保留无业务 runtime/no-fallback 的 compatibility façade 与按需 parity residue，不能新增 runtime logic。

## 执行清单

每轮自主推进按这个循环做：

1. 读最近状态：`CLAUDE.md`、`docs/context-routing.md`、`docs/batch-plan.md` 顶部 current/next、`CHANGELOG.md` 顶部 `Unreleased`，并检查 git、本地 gate 与远程 CI 实际状态；再按 `docs/context-routing.md` 只读取当前场景需要的文档顶部区。
2. 从下面大方向里选一个 coherent 中大型 vertical slice，优先选择能解决真实 Mission Commander/产品路径断点、提升当前 Windows 本机 Go-native product-path 稳定性、减少 retained PowerShell 依赖或增强 Mission Control 可用性的切片；macOS/Linux/远程三平台 CI 在 runner/billing blocker 解除前只作为 release known gap 和可延后 readiness 工作，不要让它挤占 Windows 本机可验证的 executor/reviewer/Mission Commander/pack-memory/product-path 闭环；不要继续拆 schema-field metadata 微批次，也不要让多个连续批次只扩 contract / inventory 字段或做 envelope-to-envelope summary/text 投影，而缺少 executor/reviewer/Mission Commander/pack-memory/product-path 的实际闭环。
3. 实施时优先 Go-native；禁止新增 PowerShell runtime logic。若迁移期必须保留 PowerShell，只能作为 legacy compatibility，并写清依赖方、阻塞原因和删除条件。
4. 完成后自审、评估：看是否更接近 Mission Control 北极星，是否减少 PowerShell 默认路径，架构是否清晰，是否有重复逻辑，是否需要顺手做低风险调整。
5. 自行做必要调整，不因小的低风险文档/测试/invariant 补齐而停下来问用户。
6. 验证、更新 `docs/batch-plan.md` 或相关设计文档、必要时更新 `CHANGELOG.md`，然后按用户当前会话授权提交并推送；已授权 batch 正常最多两次 push：implementation commit 覆盖代码/测试/文档/本地验证，release inspection commit 只记录 implementation commit 的远程 run，不为 inspection commit 自己触发的 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` runner/billing blocker 的新信号。
7. 如果长期目标未整体完成，先重新校准运行事实、active milestone 和风险；无升级条件时把下一批写入 `docs/batch-plan.md` 的 active/next 区并继续，不把单个 batch、inventory ready、一次提交、本地验证通过或工作树干净视为 goal 完成。
8. 每完成 3-5 个 batch 或一个明显 milestone 后，必须先做一次简短自评：当前是否仍在消除真实 Mission Commander / executor / reviewer / adapter / pack-memory 断点；若发现只是连续补 projection/summary/text 或局部 contract，停止该方向并选择更高层的 operational closure。该自评写入 `docs/batch-plan.md` 顶部即可，不新建长报告。

当前 milestone 优先级：

1. **Mission Commander run loop MVP**：主 Agent/harness 负责实际 spawn/continue/resume；Go runtime 只负责 durable request、receipt、state、hash binding、恢复和审计。先做一条 Windows 本机可验证的最小 run loop，不把 Go runtime 变成 Claude Code 进程管理器。
2. **Reviewer/session orchestration UX**：在既有 immutable reviewer dispatch/completion receipt 上，把 ready/running/failed/stale/completed/source-capture/intake 下一步做成 operator 可执行闭环，优先减少人工拼命令。
3. **Adapter-specific live validation UX**：把 authorized gate → dispatch receipt → external report → validate → record → acknowledgement 的 managed path 做成顺滑接手流程；仍不执行 heavy tool。
4. **Pack-memory product UX**：把 review/promote/reconsume 从 proof chain 升级为跨 case 可消费流程，保持 sanitize/review-first，不写真实 case artifact。
5. **可维护性收敛**：只在上述 vertical slice 内拆巨型 CLI/projection/test，优先类型化 action source/state 与共享 mission snapshot；不单独做大重构批次。

大方向只围绕八类：

1. **Mission Control UX**：减少用户面对的命令，把 `/rekit` 作为主 Agent / runtime API，而不是主要 UX。
2. **Lane protocol**：固化 packet、status、outbox、handoff、intervention、autonomy profile 和 reconcile。
3. **Replaceable session executor**：长期成员身份绑定 lane，新会话可接手，旧会话可废弃。
4. **Tactical subagents**：主 Agent 用短命 agent 做 bounded 搜索、验证、review、小修和文档一致性检查。
5. **Pre-authorized lane autonomy**：把 heavy/debug/patch/dump/hook/network/exploit-replay 的授权边界做成可记录、可审计、可止损的 lane contract。
6. **Pack-based team memory**：把复用经验 review/promote 回 pack/common，使 Agent Team 越用越强。
7. **Go-first deterministic substrate / PowerShell-free Go-native convergence**：继续收束 Go backend ownership、release readiness、PowerShell replacement/removal、pack-neutral hardening 和 policy / ledger hardening。
8. **Cross-platform readiness**：让默认入口、验证、release gate、case shim 和文档路径逐步不依赖 PowerShell；短期以 Windows 本机稳定可用为优先，远程 Linux/macOS/Windows CI 与 macOS/Linux product-path 在 runner/billing blocker 解除或需要发布前再提高优先级。

## 验证标准

按改动类型选择验证，不要求每批都跑所有 smoke。常用基础组合优先用 Go-native 路径：

```text
go run ./cmd/rekit -- -Command release-check -Format json
go run ./cmd/rekit -- -Command status
go run ./cmd/rekit -- -Command packs
go run ./cmd/rekit -- -Command doctor
go test ./...
go vet ./...
git diff --check
```

如果改迁移期 façade / fallback，再追加 `facade-smoke.ps1` 或 `REKIT_GO_DISABLE=1` 兼容验证；如果改 sync/promote/workstream/ledger/gate，使用临时 case 或对应 package tests；如果只是 roadmap / goal 文档，跑 focused invariant、`release-check -Format json` 和 `git diff --check` 即可。

Mission Control 相关批次还应检查：

- 是否减少用户直接面对的底层命令，而不是增加命令负担。
- 是否让主 Agent 更容易从 `.rekit` state 判断 lane 状态、阻塞和下一步。
- 是否支持新会话接手 lane，而不是依赖旧聊天上下文。
- 是否把用户在 lane 内的纠错/改向/模型硬切吸收到 durable state。
- 是否减少 PowerShell 默认入口、默认验证、release gate 或 case shim 依赖。
- 是否把预授权 heavy action 限制在明确 target scope、预算、stop conditions、output paths 和记录要求内。
- 是否不把真实样本、trace、dump、capture、payload、flag、客户信息、目标信息、绝对 case 路径或 case-specific 进度写入模板仓库。

## 风险与注意事项

不要把本文件当成限制模型发挥的规则清单。它只规定方向和节奏：Mission Control 北极星、PowerShell-free / Go-native / 跨平台阶段重点、中大型切片、完成后自审、评估、必要时自行调整、继续推进。

真正需要停下询问的情况仍按根目录 `CLAUDE.md`、`docs/mission-control-product-direction.md`、`docs/release-readiness.md` 和 `docs/powershell-deprecation.md` 的既有项目边界处理：新的产品方向变化、破坏性仓库操作且无明显恢复路径、未授权外部副作用、confirmed/authority 写入策略变化、runtime schema 迁移、删除公共入口但没有 Go-native 替代方案，或其它明显不可逆行为。除此之外，默认继续自主推进；PowerShell replacement/removal 本身已获当前阶段授权，只要有 Go-native 替代、文档和验证即可按批次推进。

动态调试、注入、patch、dump、hook、网络、exploit replay、fuzz 等动作只有在 strict validated durable autonomy profile 与 `authorized-gate` decision 完全覆盖 action、exact target、typed budget、stop conditions、output paths、record/notify 和 grant/expiry 时，才可由 executor 在对应边界内不逐步询问地执行；lane 文档 / packet 只表达授权意图。超出授权、出现新风险、需要 confirmed/authority 或需要 promote 到 pack/common 时必须升级。

## 给新会话的接手语句

在发正式 goal 前，可先复制这段给新会话：

```text
请在 re-context-kits 仓库 main 分支接手长期推进；先按仓库路由读取最小上下文，确认 main 与 origin/main 同步、工作树干净和当前状态，不要开始改动。我随后会发送正式 goal。
```

## 给新会话的 goal 语句

推荐复制这段短 goal。不要把上面的路线、候选项和停止条件全部塞进 goal；那些由仓库文档负责承载，模型接手后按 `docs/context-routing.md`、本文件顶部和 `docs/batch-plan.md` 顶部执行即可。

```text
在 re-context-kits 仓库 main 分支长期自主推进项目成为可实际运行的 Lane-centric Agent Team Mission Control。接手后先读 CLAUDE.md、docs/context-routing.md、docs/autonomous-goal.md 顶部、docs/batch-plan.md 顶部和 CHANGELOG.md Unreleased 校准状态；每轮选择中大型、端到端、可验证的产品闭环，完成验证、文档、提交和推送后继续下一轮；除非遇到必须由我决策的事项，否则不要停止，也不要把单批完成当作长期 goal 完成。
```

如果只想启动下一批而不是长期 goal，可复制这段：

```text
在 re-context-kits 仓库 main 分支接手下一批。先读 CLAUDE.md、docs/context-routing.md、docs/batch-plan.md 顶部和 CHANGELOG.md Unreleased 确认当前状态，再选择一个中大型可验证产品闭环推进；完成验证、文档、提交和推送后汇报真实状态。
```
