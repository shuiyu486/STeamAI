# Batch implementation plan

## 读取指南

本文件只是当前路线的短投影，不保存完整实施日志。完整路由见 `docs/context-routing.md`；当前状态以 `docs/low-friction-collaboration-roadmap.md` 为准。旧研究对照、专业工具/联合行为、verified-learning 与自主协作合同轮的各自证据保留在原路线，不因切换路线改判完成。

## Current projection

| 字段 | 当前值 |
|---|---|
| 路线 | `steamai-low-friction-collaboration-v1` |
| source | `docs/low-friction-collaboration-roadmap.md` |
| 当前批次 | LF-01~LF-06：Fresh 机器确认、成员工作区自动信任、七字段纠偏与有限旧/新 A/B、真实可见验收与交付表 |
| 当前状态 | LF-01~LF-06 全部完成：A/B 第九窗口四臂+两审全 completed、两审均 tie（两合同在两道合成题上无差异，tie 为一等结果）；前八窗口"挂起"根因已定位并披露修正（详见 low-friction roadmap LF-05 行）；实验总消费约 $3/预算 $10 |
| 当前边界 | 不开放通用 `--yes` 旁路或配置管理；learning exact confirmation、成员单写、证据边界不变；实验预算 US$10/30min |
| 既有事实 | 自主协作合同轮可见任务 9/9 与 workspace 修复完成，对照 inconclusive；不据此宣称收益 |
| 不变 | 原生成员/消息/恢复、主＋最多一辅、经验只回主包、旧 current 零改写 |
| 未声称 | 合同实现即研究增益、A/B 必然 improved、验证完成即能力升级 |

## 验证标准

- 对应内容合同与 production Fresh 物化回归，详见当前路线。
- `go test -count=1 -p=2 -timeout=30m ./...`
- `go vet ./...`
- `git diff --check`
- 默认不调用模型、IDA、HTTP；真实行为须按 `vnext/acceptance.md` 另行授权，保留失败和未知，不重复完整历史旅程。
