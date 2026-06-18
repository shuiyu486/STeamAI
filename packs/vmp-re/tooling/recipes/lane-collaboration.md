# 工作线协同 tooling recipe

目的：用 `/rekit overview/continue/start/handoff` 给 VMProtect RE case 创建可续接、可汇总、可 review-first 合并的功能分析工作区。

## 日常入口

```text
/rekit overview
/rekit continue
/rekit start <feature-name>
/rekit handoff
```

工具会根据项目概览和工作线状态提示下一步。显式底层动作主要用于排障和内部自动化。

## 生成内容

- `.rekit/lanes/<laneId>/lane.json`：机器状态。
- `.rekit/lanes/<laneId>/events.jsonl`：事件流。
- `.rekit/lanes/<laneId>/tasks.jsonl`：待处理任务。
- `.rekit/lanes/<laneId>/checkpoints/latest.json`：短 checkpoint。
- `.rekit/lanes/<laneId>/prompts/RESUME.md`：新会话接续提示。
- `.rekit/handovers/latest.md`：项目级新会话接手包。
- `captures/feature_analysis/<laneId>/`：功能支线工作区。

## 设计原则

- CLI 负责状态、模板、汇总和 review packet；LLM 负责解释建议与实际分析。
- 功能支线可写自己的 workspace，不写 canonical。
- `continue/review/promote` 只生成或消费审查包；confirmed 合并仍由主线执行。
- 跨天重启或上下文污染时，重新读取 `.rekit/handovers/latest.md` 或工作线 `RESUME.md`，不要从零开始。

## 止损

如果功能支线 workspace 长期只有 request 没有 evidence，先退回补证据；如果 request 需要底层 VM 语义，转主线，不在功能支线硬猜。
