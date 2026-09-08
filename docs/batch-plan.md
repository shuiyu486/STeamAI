# Batch implementation plan

## 读取指南

本文件只是当前路线的短投影，不保存完整实施日志。完整路由见 `docs/context-routing.md`；当前状态以 `docs/research-validity-roadmap.md` 为准。专业取证/联合研究已验事实与原对照未完成结论留在 `docs/research-execution-roadmap.md`。已交付单点方法/主辅与真实平局事实留在 `docs/research-capability-roadmap.md`；旧学习门槛留在 `docs/verified-learning-roadmap.md`，不因切换路线改判完成。

## Current projection

| 字段 | 当前值 |
|---|---|
| 路线 | `steamai-research-validity-v1` |
| source | `docs/research-validity-roadmap.md` |
| 当前批次 | Reviewer 协议回归、少量预检、新9题研究对照、5类×2已知报告 Reviewer 校准（report-copy） |
| 当前状态 | 两项均完整执行：已知报告校准 complete/pass-limited，格式/类别10/10；新9题对照 complete/tie，新版0胜9平0负，无无效题，未证明相对提升；旧失败及费用缺口保留 |
| 当前边界 | 不做生成式桥接、候选晋级、learning preview 或 Apply；不改产品 runtime 或 pack/skill/MCP 配置 |
| 既有事实 | A/B/C 实现、真实工具与有界联合协作已验，详见 `docs/research-execution-roadmap.md`；不在本短投影复制历史日志 |
| 历史未通过 | 原研究对照 `incomplete/inconclusive`；旧已知报告校准 `complete/inconclusive`，不续刷或改判 |
| 不变 | 原生成员/消息/恢复、主＋最多一辅、经验只回主包、旧 current 零改写 |
| 未声称 | 实现即研究增益、报告复制即生成式 go、suite 闭合即验收通过或 V3/V4 |

## 验证标准

- `python -m unittest discover -s tests/pack_tooling -p "test_*.py"`
- `go test -count=1 -p=2 -timeout=30m ./...`
- `go vet ./...`
- `git diff --check`
- 默认不调用模型、IDA、HTTP；真实工具、模型、成员、learning 分别按 `vnext/acceptance.md` 验收并保留失败和未知。
