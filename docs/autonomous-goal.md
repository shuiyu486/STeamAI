# Autonomous Mission Control convergence goal guide

## 读取指南

本文件是给新会话和上下文压缩后的 AI 使用的**简短接手锚点**，不是新的限制清单。用户聊天里的 goal 应保持短，只负责启动长期自主推进；防跑偏的简要实施方向放在本文件和 `docs/batch-plan.md` 顶部承载。

如果用户已经在聊天里给出 goal，以用户聊天里的 goal 为启动语义；若聊天摘要与仓库文档冲突，以仓库文档为准。本文件用于防止方向偏移：继续把 `re-context-kits` 收敛为 **Lane-centric Agent Team Mission Control**。当前阶段从继续打磨底层零件切到 **尽快真实用起来**：先打通最低可用路线，让用户能用自然语言开始 case、继续推进、查看状态、人工插手纠偏、新会话接手；允许半自动，先保证路线顺畅、状态可靠、证据可追，再在真实使用中增强 lane、reviewer、executor、pack-memory 和 tool adapter。

后续每批不需要过度拆小，也不要只做一两行微调。默认做一个中大型、能验证、能降低真实使用阻力或提升日常可用性的 product slice。不要把短 goal 展开成路线清单、候选项、验证命令或停止条件；接手后按 `docs/context-routing.md`、本文件顶部、`docs/batch-plan.md` 顶部和真实 git/本机检查状态自主选择下一批。

## 实施摘要

长期目标保持不变但阶段重点已更新：`re-context-kits` 应成为 Claude Code 中的多会话 Agent Team Mission Control 框架，而不是命令大全或单点全自动安全工具；Go 已是 public command surface 的 deterministic owner。Batch 359 后，主 Agent实际 spawn read-only reviewer → Go-native strict intake/writeback、本机 bounded reviewer E2E、pack-memory promote/reconsume package E2E，以及 authorized execution observation evidence + bounded adapter execution report strict intake/contract projection/read-only validation preflight 已形成底座；runtime 仍不自动 spawn 或管理 session/reviewer，也不执行 heavy-tool。当前优先级是把已有骨架收敛成可真实日常使用的 MVP：主 Agent 能围绕真实 case 开始任务、判断下一步、记录状态/证据、允许人工插手、支持新会话接手；再边用边增强 reviewer/session orchestration、lane executor / tool-adapter live validation、pack-memory product UX、PowerShell-free product path 与跨平台验证。

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

1. 读最小接手上下文：`CLAUDE.md`、`docs/context-routing.md`、本文件顶部、`docs/batch-plan.md` 顶部 current/next、`CHANGELOG.md` 顶部 `Unreleased`，再确认 main 与 origin/main 同步、git 与必要本机状态。
2. 自主选择一个最能让项目“马上更好用”的产品切片，优先围绕真实日常路线：开始 case、继续推进、查看状态、人工插手纠偏、新会话接手、reviewer/subagent 接手、pack-memory 复用。
3. 实施时保持 Go-native 和 PowerShell-free 默认路径；禁止新增 PowerShell runtime logic。
4. 完成后自审、评估，做必要验证、更新对应文档/CHANGELOG；若当前 goal/session 已授权提交推送，则按仓库 cadence 直接提交并推送到 origin/main。
5. 如果长期 goal 未被用户明确停止，完成单批后继续选下一批；不要把单批完成、一次本机检查通过或工作树干净当作长期 goal 完成。
6. 每 3-5 批或一个明显节点做短自评：是否已经更接近“用户能真实日常使用”；若只是连续补字段/summary/text，切回更高层的产品闭环。

当前 milestone 优先级：

1. **最低可用 Mission Control 路线**：用户能用自然语言开始 case、继续推进、查看状态、人工插手纠偏、新会话接手；允许半自动，但必须顺畅、可记录、可恢复。
2. **Mission Commander run loop MVP**：主 Agent/harness 负责实际 spawn/continue/resume；Go runtime 只负责 durable request、receipt、state、hash binding、恢复和审计。
3. **Reviewer/session orchestration UX**：把 ready/running/failed/stale/completed/source-capture/intake 下一步做成 operator 可执行闭环，优先减少人工拼命令。
4. **Pack-memory product UX**：把 review/promote/reconsume 从 proof chain 升级为跨 case 可消费流程，保持 sanitize/review-first，不写真实 case artifact。
5. **Adapter-specific live validation UX**：把 authorized gate → dispatch receipt → external report → validate → record → acknowledgement 做成顺滑接手流程；仍不执行 heavy tool。
6. **嵌入式可维护性收敛**：只在上述产品切片内拆巨型 CLI/projection/test 或类型化 action source/state，不单独做大重构批。

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

不要把本文件当成限制模型发挥的规则清单。它只规定方向和节奏：Mission Control 北极星、先真实用起来的阶段重点、中大型切片、完成后自审、评估、必要时自行调整、继续推进。

真正需要停下询问的情况仍按根目录 `CLAUDE.md`、`docs/mission-control-product-direction.md`、`docs/release-readiness.md` 和 `docs/powershell-deprecation.md` 的既有项目边界处理：新的产品方向变化、破坏性仓库操作且无明显恢复路径、未授权外部副作用、confirmed/authority 写入策略变化、runtime schema 迁移、删除公共入口但没有 Go-native 替代方案，或其它明显不可逆行为。除此之外，默认继续自主推进；PowerShell replacement/removal 本身已获当前阶段授权，只要有 Go-native 替代、文档和验证即可按批次推进。

动态调试、注入、patch、dump、hook、网络、exploit replay、fuzz 等动作只有在 strict validated durable autonomy profile 与 `authorized-gate` decision 完全覆盖 action、exact target、typed budget、stop conditions、output paths、record/notify 和 grant/expiry 时，才可由 executor 在对应边界内不逐步询问地执行；lane 文档 / packet 只表达授权意图。超出授权、出现新风险、需要 confirmed/authority 或需要 promote 到 pack/common 时必须升级。

## 给新会话的接手语句

在发正式 goal 前，可先复制这段给新会话：

```text
请在 re-context-kits 仓库 main 分支接手长期推进；先按 CLAUDE.md、docs/context-routing.md、docs/autonomous-goal.md 顶部、docs/batch-plan.md 顶部和 CHANGELOG.md Unreleased 读取最小上下文，确认当前状态，不要开始改动。我随后会发送正式 goal。
```

## 给新会话的 goal 语句

推荐复制这段短 goal。不要把上面的路线、候选项和停止条件全部塞进 goal；那些由仓库文档负责承载，模型接手后按 `docs/context-routing.md`、本文件顶部和 `docs/batch-plan.md` 顶部执行即可。

```text
继续推进 re-context-kits，把它尽快收敛成可真实日常使用的 LLM 任务指挥台；先打通最低可用闭环，再边用边增强。每批自主选择最有价值的下一步，完成验证、提交和推送后继续推进，不要过早判定 goal 完成。
```

如果只想启动下一批而不是长期 goal，可复制这段：

```text
在 re-context-kits 仓库 main 分支接手下一批。先读 CLAUDE.md、docs/context-routing.md、docs/batch-plan.md 顶部和 CHANGELOG.md Unreleased 确认当前状态，再选择一个中大型可验证产品闭环推进；完成验证、文档、提交和推送后汇报真实状态。
```
