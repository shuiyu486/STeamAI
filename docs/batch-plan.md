# Batch implementation plan

## 读取指南

本文件只是当前路线的短投影，不保存完整实施日志。完整路由见 `docs/context-routing.md`；当前状态以 `docs/research-execution-roadmap.md` 为准。已交付单点方法/主辅与真实平局事实留在 `docs/research-capability-roadmap.md`；旧学习门槛留在 `docs/verified-learning-roadmap.md`，不因切换路线改判完成。

## Current projection

| 字段 | 当前值 |
|---|---|
| 路线 | `steamai-research-execution-v1` |
| source | `docs/research-execution-roadmap.md` |
| 当前批次 | A/B/C 实现与回归完成；RE-05 和 VL-CLOSE 保留真实未完成验收，超时后暂停新增付费 |
| 状态 | 实现与默认回归通过；36研究轮完成，盲评超时/可见onboarding导致完整验收仍待收口 |
| 目标 | A 定点专业取证；B 跨步骤解释/预测；C 同 case 客户端/API 联合研究 |
| 实现边界 | 一个显式 IDAPython exporter；原生 curl；现有 E/F/R 直接绑定全部连接，不建执行/图/任务平台 |
| 不变 | 原生成员/消息/恢复、主＋最多一辅、经验只回主包、旧 current 零改写 |
| 已验子范围 | 27项 Python、全量 Go/vet；14次真实 loopback GET；当前 high 模型身份与 Read/resume；36研究轮、首题盲评平局；旧原件 verifier 与锁恢复子路径 |
| 待验 | 实际 IDA、成员联合行为/可见协作、9题静态对照结论；正式学习适用校准/候选比较/确认 |
| 未声称 | 实现即研究增益、fake API 即 IDA、旧 report-copy pass 即 go、合成 Apply 即真实 canonical 回流或 V4 |

## 验证标准

- `python -m unittest discover -s tests/pack_tooling -p "test_*.py"`
- `go test -count=1 -p=2 -timeout=30m ./...`
- `go vet ./...`
- `git diff --check`
- 默认不调用模型、IDA、HTTP；真实工具、模型、成员、learning 分别按 `vnext/acceptance.md` 验收并保留失败和未知。
