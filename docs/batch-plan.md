# Batch implementation plan

## 读取指南

本文件只是当前路线的短投影，不保存完整实施日志。完整路由见 `docs/context-routing.md`；当前状态以 `docs/autonomous-collaboration-roadmap.md` 为准。旧研究对照、专业工具/联合行为与 verified-learning 的各自证据保留在原路线，不因切换路线改判完成。

## Current projection

| 字段 | 当前值 |
|---|---|
| 路线 | `steamai-autonomous-collaboration-v1` |
| source | `docs/autonomous-collaboration-roadmap.md` |
| 当前批次 | AC-04~AC-07：本机正式安装前提、一个真实可见任务验收、按证据最小修复与收口 |
| 当前状态 | 可见任务完成：owner 判定 9/9 与冻结 oracle 一致，Round 1 needs-evidence→补证→Round 2 accepted；修复 `validateMembers` 成员目录单文件限制并补回归，全量测试/vet 通过；无对照数据，相对收益未证实 |
| 当前边界 | 不改 Go production、工具配置或执行器，不放宽权限/确认、单写、证据与学习资格 |
| 既有事实 | 上轮研究 complete/tie，已知报告校准 complete/pass-limited；不证明本轮收益 |
| 不变 | 原生成员/消息/恢复、主＋最多一辅、经验只回主包、旧 current 零改写 |
| 未声称 | 规则越少必然更强、合同实现即研究增益、验证完成即 V2/V3/V4 |

## 验证标准

- 对应内容合同与 production Fresh 物化回归，详见当前路线。
- `go test -count=1 -p=2 -timeout=30m ./...`
- `go vet ./...`
- `git diff --check`
- 默认不调用模型、IDA、HTTP；真实行为须按 `vnext/acceptance.md` 另行授权，保留失败和未知，不重复完整历史旅程。
