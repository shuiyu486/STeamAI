# Autonomous Mission Control convergence goal guide

## 读取指南

本文件是给新会话和上下文压缩后的 AI 使用的**简短接手锚点**，不是给维护者人工阅读的长方案，也**不是新的限制清单**。

如果用户已经在聊天里给出 goal，以用户聊天里的 goal 为准；若聊天摘要与仓库文档冲突，以仓库文档为准。本文件用于防止方向偏移：继续把 `re-context-kits` 收敛为 **Lane-centric Agent Team Mission Control**，并把当前阶段重点切到 **PowerShell-free / Go-native / 跨平台** convergence。最终产品北极星见 `docs/mission-control-product-direction.md`，具体路线写回 `docs/batch-plan.md`、`docs/go-first-convergence-plan.md`、`docs/powershell-deprecation.md` 与 `docs/release-readiness.md`。

后续每批不需要过度拆小，也不要只做一两行微调。默认做一个中大型、能验证、能降低真实维护风险或提升实际可用性的 vertical slice。

## 实施摘要

长期目标保持不变但阶段重点已更新：`re-context-kits` 应成为 Claude Code 中的多会话 Agent Team Mission Control 框架，而不是命令大全；runtime 应收敛为 Go-native deterministic substrate，而不是继续依赖 PowerShell façade/fallback。

核心产品形态：

- 用户主要和主 Agent / Mission Commander 交互。
- 主 Agent 调度 durable member lanes，而不是绑定某个旧聊天窗口。
- 每个 lane 由可替换 Claude Code session executor 执行；旧会话上下文污染、模型变化或用户希望重开时，新会话可读取 handoff / packet / evidence 接手同一 lane。
- 主 Agent 可启动短命 tactical subagents 做搜索、验证、review、小修和 bounded implementation。
- 用户可随时进入任意 lane 打断、纠错、改向、硬切模型或要求新会话接手；lane 继续时必须 reconcile 干预并写入 durable state。
- 在 lane 文档 / task packet / autonomy profile 明确预授权的 target scope、预算、stop conditions、output paths 和记录要求内，成员 lane 可自主执行 heavy-tool、动态调试、patch、dump、hook、网络、exploit replay 等动作；越界、新风险、confirmed/authority 或 pack promote 必须升级。
- Go backend 是 canonical runtime owner，负责状态、ledger、gate、release inventory、sync/promote、pack-neutral contract 和跨平台路径；PowerShell 只允许作为迁移期 legacy/compatibility residue，不能新增 runtime logic。

## 执行清单

每轮自主推进按这个循环做：

1. 读最近状态：`CLAUDE.md`、`docs/mission-control-product-direction.md`、`docs/autonomous-goal.md`、`docs/go-first-convergence-plan.md`、`docs/powershell-deprecation.md`、`docs/release-readiness.md`、`docs/batch-plan.md` 末尾、`CHANGELOG.md`。
2. 从下面大方向里选一个中大型 vertical slice，优先选择能减少 PowerShell 依赖、提升 Go-native / macOS/Linux/Windows 跨平台能力或增强 Mission Control 可用性的切片。
3. 实施时优先 Go-native；禁止新增 PowerShell runtime logic。若迁移期必须保留 PowerShell，只能作为 legacy compatibility，并写清依赖方、阻塞原因和删除条件。
4. 完成后自审、评估：看是否更接近 Mission Control 北极星，是否减少 PowerShell 默认路径，架构是否清晰，是否有重复逻辑，是否需要顺手做低风险调整。
5. 自行做必要调整，不因小的低风险文档/测试/invariant 补齐而停下来问用户。
6. 验证、更新 `docs/batch-plan.md` 或相关设计文档、必要时更新 `CHANGELOG.md`，然后按用户当前会话授权提交并推送。
7. 如果长期目标未整体完成，默认继续自主推进，不把单个 batch、一次提交、验证通过或工作树干净视为 goal 完成。

大方向只围绕八类：

1. **Mission Control UX**：减少用户面对的命令，把 `/rekit` 作为主 Agent / runtime API，而不是主要 UX。
2. **Lane protocol**：固化 packet、status、outbox、handoff、intervention、autonomy profile 和 reconcile。
3. **Replaceable session executor**：长期成员身份绑定 lane，新会话可接手，旧会话可废弃。
4. **Tactical subagents**：主 Agent 用短命 agent 做 bounded 搜索、验证、review、小修和文档一致性检查。
5. **Pre-authorized lane autonomy**：把 heavy/debug/patch/dump/hook/network/exploit-replay 的授权边界做成可记录、可审计、可止损的 lane contract。
6. **Pack-based team memory**：把复用经验 review/promote 回 pack/common，使 Agent Team 越用越强。
7. **Go-first deterministic substrate / PowerShell-free Go-native convergence**：继续收束 Go backend ownership、release readiness、PowerShell replacement/removal、pack-neutral hardening 和 policy / ledger hardening。
8. **Cross-platform readiness**：让默认入口、验证、release gate、case shim 和文档路径逐步不依赖 PowerShell，并面向 macOS/Linux/Windows 可用性收敛。

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

动态调试、注入、patch、dump、hook、网络、exploit replay、fuzz 等动作若已在当前 lane 文档 / packet / autonomy profile 中明确预授权，可在 scope、预算、止损、输出路径和记录要求内自主执行；超出授权、出现新风险、需要 confirmed/authority 或需要 promote 到 pack/common 时必须升级。

## 给新会话的接手语句

在发正式 goal 前，可先复制这段给新会话：

```text
请在 C:\AI\m_projects\RE\re-context-kits 接手长期推进；先读取仓库文档校准方向，当前重点是 PowerShell-free、Go-native、跨平台的 Lane-centric Agent Team Mission Control，不要把单个 batch、提交或工作树干净视为长期 goal 完成。先只读取并确认接手，等我发正式 goal。
```

## 给新会话的 goal 语句

推荐直接复制这段给新会话，保持短而明确：

```text
在 C:\AI\m_projects\RE\re-context-kits 中，长期、自主、连续推进项目向 docs/mission-control-product-direction.md 定义的 Lane-centric Agent Team Mission Control 收敛。

当前阶段优先推进 PowerShell-free / Go-native / 跨平台收敛：让 Go CLI/backend 成为 canonical runtime，让默认入口、验证、release gate、case shim 和文档路径逐步不依赖 PowerShell；迁移期可短暂保留 PowerShell，但禁止新增 PowerShell runtime logic，每个相关 batch 都应减少 PowerShell 依赖。

为防止上下文压缩导致方向偏移，每轮开始先用仓库文档校准方向，优先读取 docs/mission-control-product-direction.md、docs/autonomous-goal.md、docs/batch-plan.md、docs/go-first-convergence-plan.md、docs/powershell-deprecation.md、docs/release-readiness.md、README.md、CHANGELOG.md。若聊天摘要与仓库文档冲突，以仓库文档为准。

每轮选择一个中大型、可验证、能提升真实可用性或降低维护风险的 vertical slice，不做一两行微批次。优先方向包括 PowerShell replacement/removal、Go-native validation、release readiness、macOS/Linux 支持、Mission Commander 体验、lane handoff/resume、lane autonomy profile、pack-neutral dry-run 与跨 pack 可用性。

详细路线、关键决策、验证结果、下一步方向和未完成风险必须持续写回 repo docs，尤其是 docs/batch-plan.md；涉及 PowerShell、Go-native、跨平台或 release readiness 时，同步更新相关设计文档。不要只把计划和结论留在聊天上下文中。

每批完成后运行必要验证，更新必要文档和 CHANGELOG，提交并推送 main；如果长期目标未整体完成，继续下一批。除产品方向变化、无替代方案的公共入口删除、schema 迁移、confirmed/authority 策略变化、未授权外部副作用或难以判断的架构取舍外，自主判断并持续推进。

不要声称长期 goal 已完成，除非 PowerShell 已退出默认 runtime/入口/验证/release gate，macOS/Linux 默认路径可用并文档化，且 Mission Commander、durable lanes、replaceable session executor、Human-in-the-Lane、lane autonomy profile、pack-based team memory 与 Go-native runtime 的核心闭环均已实现、验证并文档化。
```
