# Autonomous Mission Control convergence goal guide

## 读取指南

本文件是给新会话和上下文压缩后的 AI 使用的简短接手锚点，不是给维护者人工阅读的长方案，也不是新的限制清单。

如果用户已经在聊天里给出 goal，以用户聊天里的 goal 为准；本文件用于防止方向偏移：继续把 `re-context-kits` 收敛为 **Lane-centric Agent Team Mission Control**，同时保持 Go-first deterministic runtime / release readiness 路线。最终产品北极星见 `docs/mission-control-product-direction.md`。

后续每批不需要过度拆小，也不要只做一两行微调。默认做一个中大型、能验证、能降低真实维护风险或提升实际可用性的 vertical slice。

## 实施摘要

长期目标保持不变但产品表达已收敛：`re-context-kits` 应成为 Claude Code 中的多会话 Agent Team Mission Control 框架，而不是命令大全。

核心产品形态：

- 用户主要和主 Agent / Mission Commander 交互。
- 主 Agent 调度 durable member lanes，而不是绑定某个旧聊天窗口。
- 每个 lane 由可替换 Claude Code session executor 执行；旧会话上下文污染、模型变化或用户希望重开时，新会话可读取 handoff / packet / evidence 接手同一 lane。
- 主 Agent 可启动短命 tactical subagents 做搜索、验证、review、小修和 bounded implementation。
- 用户可随时进入任意 lane 打断、纠错、改向、硬切模型或要求新会话接手；lane 继续时必须 reconcile 干预并写入 durable state。
- 在 lane 文档 / task packet / autonomy profile 明确预授权的 target scope、预算、stop conditions、output paths 和记录要求内，成员 lane 可自主执行 heavy-tool、动态调试、patch、dump、hook、网络、exploit replay 等动作；越界、新风险、confirmed/authority 或 pack promote 必须升级。
- Go backend 继续作为 deterministic runtime owner 收束状态、ledger、gate、release inventory、sync/promote 和 pack-neutral contract；PowerShell 保持 façade / legacy / parity / fallback。

## 执行清单

每轮自主推进按这个循环做：

1. 读最近状态：`CLAUDE.md`、`docs/mission-control-product-direction.md`、`docs/autonomous-goal.md`、`docs/go-first-convergence-plan.md`、`docs/release-readiness.md`、`docs/batch-plan.md` 末尾。
2. 从下面七个大方向里选一个中大型 vertical slice。
3. 实施时优先 Go-first；PowerShell 只做 façade、fallback、compatibility 或必要 smoke 维护。
4. 完成后自审：看是否更接近 Mission Control 北极星、架构是否更清晰、是否有重复逻辑、是否需要顺手做低风险调整。
5. 自行做必要调整，不因小的低风险文档/测试/invariant 补齐而停下来问用户。
6. 验证、更新 `docs/batch-plan.md` 或相关文档、必要时更新 `CHANGELOG.md`，然后按用户当前会话授权决定是否提交/推送。

大方向只围绕七类：

1. **Mission Control UX**：减少用户面对的命令，把 `/rekit` 作为主 Agent / runtime API，而不是主要 UX。
2. **Lane protocol**：固化 packet、status、outbox、handoff、intervention、autonomy profile 和 reconcile。
3. **Replaceable session executor**：长期成员身份绑定 lane，新会话可接手，旧会话可废弃。
4. **Tactical subagents**：主 Agent 用短命 agent 做 bounded 搜索、验证、review、小修和文档一致性检查。
5. **Pre-authorized lane autonomy**：把 heavy/debug/patch/dump/hook/network/exploit-replay 的授权边界做成可记录、可审计、可止损的 lane contract。
6. **Pack-based team memory**：把复用经验 review/promote 回 pack/common，使 Agent Team 越用越强。
7. **Go-first deterministic substrate**：继续收束 Go backend ownership、release readiness、PowerShell deprecation readiness、pack-neutral hardening 和 policy / ledger hardening。

## 验证标准

按改动类型选择验证，不要求每批都跑所有 smoke。常用基础组合：

```powershell
go run ./cmd/rekit -- -Command release-check -Format json
go test ./...
go vet ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

如果改 façade，追加 `facade-smoke.ps1`；如果改 sync/promote/workstream/ledger/gate，使用临时 case 或对应 package tests；如果只是 roadmap / goal 文档，跑 focused invariant、`release-check -Format json` 和 `git diff --check` 即可。

Mission Control 相关批次还应检查：

- 是否减少用户直接面对的底层命令，而不是增加命令负担。
- 是否让主 Agent 更容易从 `.rekit` state 判断 lane 状态、阻塞和下一步。
- 是否支持新会话接手 lane，而不是依赖旧聊天上下文。
- 是否把用户在 lane 内的纠错/改向/模型硬切吸收到 durable state。
- 是否把预授权 heavy action 限制在明确 target scope、预算、stop conditions、output paths 和记录要求内。
- 是否不把真实样本、trace、dump、capture、payload、flag、客户信息、目标信息、绝对 case 路径或 case-specific 进度写入模板仓库。

## 风险与注意事项

不要把本文件当成限制模型发挥的规则清单。它只规定方向和节奏：Mission Control 北极星、中大型切片、完成后自审、评估、必要时自行调整、继续推进。

真正需要停下询问的情况仍按根目录 `CLAUDE.md`、`docs/mission-control-product-direction.md`、`docs/release-readiness.md` 和 `docs/powershell-deprecation.md` 的既有项目边界处理：新的产品方向变化、破坏性仓库操作、未授权外部副作用、confirmed/authority 写入策略变化、runtime schema 迁移、PowerShell 删除、或其它明显不可逆行为。除此之外，默认继续自主推进。

动态调试、注入、patch、dump、hook、网络、exploit replay、fuzz 等动作若已在当前 lane 文档 / packet / autonomy profile 中明确预授权，可在 scope、预算、止损、输出路径和记录要求内自主执行；超出授权、出现新风险、需要 confirmed/authority 或需要 promote 到 pack/common 时必须升级。

## 给新会话的接手语句

在发正式 goal 前，可先复制这段给新会话：

```text
在 C:\AI\m_projects\RE\re-context-kits 继续维护项目。先读取 CLAUDE.md、docs/mission-control-product-direction.md、docs/autonomous-goal.md、docs/go-first-convergence-plan.md、docs/release-readiness.md 和 docs/batch-plan.md 最新批次。当前产品方向已经确认：re-context-kits 要收敛为 Lane-centric Agent Team Mission Control，而不是命令大全；主 Agent 统筹 durable member lanes，lane 由可替换 Claude Code session executor 执行，也可用短命 tactical subagents；用户可随时介入 lane，lane 需 reconcile 干预；在 lane 文档/packet/autonomy profile 预授权范围内可自主执行 heavy/debug/patch/dump/hook/network/exploit-replay 等动作，但必须有 target scope、预算、止损、记录和升级边界。先只读取并确认接手，不要立刻改文件，等我发正式 goal。
```

## 给新会话的 goal 语句

推荐直接复制这段给新会话：

```text
在 C:\AI\m_projects\RE\re-context-kits 中，长期自主推进 re-context-kits 向 Lane-centric Agent Team Mission Control 收敛。每轮选择一个中大型、可验证、能降低真实维护风险或提高实际可用性的 vertical slice，不要只做一两行微批次；完成后自行审查、评估效果、做必要低风险调整，然后验证、更新 docs/batch-plan.md 或相关设计文档、必要时更新 CHANGELOG，再按当前会话授权提交并推送 main，接着继续下一批，不要把阶段性进展当成 goal 完成。

产品北极星：用户主要和主 Agent / Mission Commander 交互；主 Agent 调度 durable member lanes，而不是绑定旧聊天窗口；每个 lane 可由长期 Claude Code 会话执行，也可由新会话接手；主 Agent 可启动短命 tactical subagents 做搜索、验证、review、小修和 bounded implementation；用户可随时进入任意 lane 纠错、改向、硬切模型或要求新会话接手，lane 必须自动 reconcile 干预并写入 durable state；在 lane 文档/packet/autonomy profile 明确预授权的 target scope、预算、stop conditions、output paths 和记录要求内，成员 lane 可以自主执行 heavy-tool、动态调试、patch、dump、hook、exploit replay、网络扫描/请求回放等动作，不必每一步都打断用户，但超出范围、出现新风险、需要 confirmed/authority 或 pack promote 时必须升级。

实施主线围绕七类：1) Mission Control UX：减少用户面对的命令，把 /rekit 作为 runtime API 而不是主要 UX；2) lane protocol：packet/status/outbox/handoff/intervention/autonomy profile/reconcile；3) replaceable session executor：长期成员身份绑定 lane，新会话可接手，旧会话可废弃；4) tactical subagents：主 Agent 用短命 agent 做 bounded 搜索、验证、review、小修；5) pre-authorized lane autonomy：把 heavy/debug/patch/dump/hook/network/exploit-replay 的授权边界做成可记录、可审计、可止损的 lane contract；6) pack-based team memory：把复用经验 review/promote 回 pack/common；7) Go-first deterministic substrate：继续让 Go backend 收束状态、ledger、gate、release inventory、sync/promote 和 pack-neutral contract，PowerShell 只保留 façade/fallback/compatibility。

每批开始前先读 CLAUDE.md、docs/mission-control-product-direction.md、docs/autonomous-goal.md、docs/go-first-convergence-plan.md、docs/release-readiness.md 和 docs/batch-plan.md 最新批次；优先做中大型 vertical slice，并自审是否偏离 Mission Control 北极星。除新的产品方向变化、破坏性仓库操作、未授权外部副作用、runtime schema 迁移、PowerShell 删除、confirmed/authority 写入策略变化或难以判断的架构取舍外，自主判断并持续推进。
```
