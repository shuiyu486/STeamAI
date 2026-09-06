# Batch implementation plan

## 读取指南

本文件只是当前路线的短投影，不保存完整实施日志。完整路由见 `docs/context-routing.md`；当前状态以 `docs/research-capability-roadmap.md` 为准。verified-learning 既有证据与未完成门槛保留在 `docs/verified-learning-roadmap.md`，不因切换路线而改判完成。

## Current projection

| 字段 | 当前值 |
|---|---|
| 路线 | `steamai-research-capability-v1` |
| source | `docs/research-capability-roadmap.md` |
| 当前批次 | `RC-01 双领域方法`、`RC-02 分岔点决策`、`RC-03 主辅 pack` |
| 状态 | 已实现；默认回归通过；32 轮真实静态对照已完成且四组平局；可见会话保存/双包消费/恢复已验证 |
| 目标 | Binary RE/Web/API 各深化一项方法；重要分岔点选区分性检查；同 case 主＋可选一个辅助包 |
| 不变 | 独立 Claude Code 成员/原生上下文与恢复；单 Commander；辅助只读、经验只回主包；旧 current 零改写 |
| 已验证 | production Fresh/current/learning 边界、旧单包零改写、双领域方法合同；真实双向 Fresh、双包读取、Windows 可见成员保存/同 session resume/正常退出；完整 tests/vet/diff check |
| 真实结论 | 四组均 16/16，未观察到质量增益/回归；发现并修复正式启动继承 child 标记导致 transcript 不保存的问题 |
| 未声称 | 自发决策/一般研究增益、人工纠偏与跨会话实投递、实际 HTTP、新 calibration/promotion 或 V4；没有正式 learning 回流与 Release |

合成与文本 tests 只证明机械/内容合同，不证明研究更准、更快，也不授予 V2/V3/V4。原 verified-learning 的 attestation/comparison/field evidence 门槛继续按原路线与 `vnext/acceptance.md` 处理。

## 验证标准

- 先直接相关 production 与 contract tests；默认不调用模型。
- `go test -count=1 -p=2 -timeout=30m ./...`
- `go vet ./...`
- `git diff --check`
- 真实模型、可见窗口、请求和 learning apply 分别取得所需授权后验收；失败、不确定和负结果保留。
