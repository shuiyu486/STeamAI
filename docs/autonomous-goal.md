# Autonomous Go-first convergence goal guide

## 读取指南

本文件是给新会话和上下文压缩后的 AI 使用的简短接手锚点，不是给维护者人工阅读的长方案，也不是新的限制清单。

如果用户已经在聊天里给出 goal，以用户聊天里的 goal 为准；本文件只用于防止方向偏移：继续 Go-first convergence，做中大型 vertical slice，每批完成后自审、评估、必要时自行低风险调整，然后继续推进。

## 实施摘要

长期目标保持不变：让 `rekit` 的 deterministic runtime owner 继续向 Go backend 收束，PowerShell 保持 façade / legacy / parity，release readiness 由 Go-first gate 与轻量 CI 支撑，Agent Team 与 pack-neutral 能力持续增强。

后续每批不需要过度拆小，也不要只做一两行微调。默认做一个中大型、能验证、能降低真实维护风险的 vertical slice。

大方向只有五个：

1. **Go-first release readiness**：强化 `release-check`、Go tests、`go vet`、doctor、轻量 CI 的解释力和稳定性。
2. **PowerShell deprecation readiness**：整理 inventory、freeze、fallback retirement 条件和 invariant；先准备，别急着删。
3. **Pack-neutral hardening**：减少 `vmp-re` 惯性，让 skeleton pack、route、workspace、handoff 更 manifest-driven。
4. **Policy / ledger hardening**：增强 append-only ledger、历史兼容、WhatIf 诊断和 policy 文档一致性。
5. **接手质量与持续推进治理**：让新会话能直接接手，靠 batch-plan / handoff / invariant 防偏移，而不是靠长聊天上下文。

## 执行清单

每轮自主推进按这个循环做：

1. 读最近状态：`CLAUDE.md`、`docs/go-first-convergence-plan.md`、`docs/release-readiness.md`、`docs/batch-plan.md` 末尾。
2. 从五个大方向里选一个中大型 vertical slice。
3. 实施时优先 Go-first；PowerShell 只做 façade、fallback、compatibility 或必要 smoke 维护。
4. 完成后自审：看架构是否更清晰、是否有重复逻辑、是否需要顺手做低风险调整。
5. 自行做必要调整，不因小的低风险文档/测试/invariant 补齐而停下来问用户。
6. 验证、更新 batch-plan 或相关文档、提交并推送，然后继续下一批。

## 验证标准

按改动类型选择验证，不要求每批都跑所有 smoke。常用基础组合：

```powershell
go run ./cmd/rekit -- -Command release-check -Format json
go test ./...
go vet ./...
.\rekit\rekit.ps1 -Command doctor
git diff --check
```

如果改 façade，追加 `facade-smoke.ps1`；如果改 sync/promote/workstream/ledger/gate，使用临时 case 或对应 package tests；如果只是 roadmap / goal 文档，跑 focused invariant 和 `git diff --check` 即可。

## 风险与注意事项

不要把本文件当成限制模型发挥的规则清单。它只规定方向和节奏：中大型切片、自审、评估、必要时自行调整、继续推进。

真正需要停下询问的情况仍按根目录 `CLAUDE.md`、`docs/release-readiness.md` 和 `docs/powershell-deprecation.md` 的既有项目边界处理：产品方向变化、破坏性动作、外部副作用、重大架构取舍、历史状态迁移、PowerShell 删除、或其它明显不可逆行为。除此之外，默认继续自主推进。

## 给新会话的 goal 语句

推荐直接复制这段给新会话：

```text
在 C:\AI\m_projects\RE\re-context-kits 中，长期持续自主推进 Go-first convergence 和 release readiness。每轮选择一个中大型、可验证的 vertical slice，不要只做一两行微批次；完成后自行审查、评估效果、判断是否需要低风险调整，需要就自行调整，然后验证、更新 docs/batch-plan.md 或相关设计文档、提交并推送 main，接着继续下一批，不要把阶段性进展当成 goal 完成。

大方向只围绕五类：1) Go-first release readiness；2) PowerShell deprecation readiness；3) pack-neutral hardening；4) policy / ledger hardening；5) 新会话接手质量与持续推进治理。不要机械堆 PowerShell smoke/catalog，也不要为了安全感写过长约束。除产品方向变化、破坏性动作、外部副作用、重大架构取舍、历史状态迁移、PowerShell 删除等明显需要用户决策的情况外，自主判断并持续推进。
```
