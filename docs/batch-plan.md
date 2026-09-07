# Batch implementation plan

## 读取指南

本文件只是当前路线的短投影，不保存完整实施日志。完整路由见 `docs/context-routing.md`；当前状态以 `docs/research-execution-roadmap.md` 为准。已交付单点方法/主辅与真实平局事实留在 `docs/research-capability-roadmap.md`；旧学习门槛留在 `docs/verified-learning-roadmap.md`，不因切换路线改判完成。

## Current projection

| 字段 | 当前值 |
|---|---|
| 路线 | `steamai-research-execution-v1` |
| source | `docs/research-execution-roadmap.md` |
| 当前批次 | A/B/C 实现、实际 IDA 定点导出与真实 loopback/client 路径完成；原研究对照按超时规则闭合为 incomplete/inconclusive；RE-05 可见成员验收按证据分层收口；VL-CLOSE 已闭合为 inconclusive |
| 状态 | 真人 Owner 纠偏、三个 production 命名可见成员、双来源身份核对及三条逐 msg_id 原生送达已验；完整 stale-task HOLD 与联合 E/F/R 往返未闭合；未启动的 Attempt 09 仓库外脚手架已删除，不重复真人纠偏或整趟验收 |
| 目标 | A 定点专业取证；B 跨步骤解释/预测；C 同 case 客户端/API 联合研究 |
| 实现边界 | 一个显式 IDAPython exporter；原生 curl；现有 E/F/R 直接绑定全部连接，不建执行/图/任务平台 |
| 不变 | 原生成员/消息/恢复、主＋最多一辅、经验只回主包、旧 current 零改写 |
| 已验子范围 | 27项 Python、全量 Go/vet；14次真实 loopback GET；IDA 9.3/IDAPython 9.3.0 定点导出；当前 high 模型身份与 Read/resume；36次研究调用、首题盲评平局及超时停止闭包；真人纠偏、命名可见成员、inventory＋ListAgents 身份交叉核对和 success/incoming msg_id 消息链；旧原件 verifier 与锁恢复子路径；生成式 calibration 的10次首次 Reviewer及 complete/inconclusive closure |
| 未闭合 | stale task 已送达但未在会话清理前返回 `HOLD_STALE_TASK`；可见成员没有产出联合 E/F/R 与整体复审。这两项如未来重验必须拆成独立 live gate，不再串成整趟 attempt。未来只有另建新 calibration suite 且产生真实 go/eligible candidate 时，才可能进入比较与 exact confirmation |
| 未声称 | 实现即研究增益、fake API 即 IDA、旧 report-copy pass 即 go、complete/inconclusive 即校准通过、合成 Apply 即真实 canonical 回流或 V4 |

## 验证标准

- `python -m unittest discover -s tests/pack_tooling -p "test_*.py"`
- `go test -count=1 -p=2 -timeout=30m ./...`
- `go vet ./...`
- `git diff --check`
- 默认不调用模型、IDA、HTTP；真实工具、模型、成员、learning 分别按 `vnext/acceptance.md` 验收并保留失败和未知。
