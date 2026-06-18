# Lane collaboration tooling recipe

目的：用 B3 `lane/auto` 给 VMProtect RE case 创建可续接、可汇总、可 review-first 合并的功能分析工作区。

## 日常入口

```text
/rekit board
/rekit auto
/rekit lane start feature-analysis <feature-name>
/rekit lane resume <laneId>
```

工具会根据 board/lane 状态提示下一步。显式动作主要用于排障和内部自动化。

## 生成内容

- `.rekit/lanes/<laneId>/lane.json`：机器状态。
- `.rekit/lanes/<laneId>/events.jsonl`：事件流。
- `.rekit/lanes/<laneId>/tasks.jsonl`：待处理任务。
- `.rekit/lanes/<laneId>/checkpoints/latest.json`：短 checkpoint。
- `.rekit/lanes/<laneId>/prompts/RESUME.md`：新会话续接提示词。
- `captures/feature_analysis/<laneId>/`：功能会话工作区。

## 设计原则

- CLI 负责状态、模板、汇总和 review packet；LLM 负责解释建议与实际分析。
- Feature lane 可写自己的 workspace，不写 canonical。
- `auto/review/promote` 只生成或消费审查包；confirmed 合并仍由 authority lane 执行。
- 跨天重启或上下文污染时，重新读取 lane 的 `RESUME.md`，不要从零开始。

## 止损

如果 feature workspace 长期只有 request 没有 evidence，先退回补证据；如果 request 需要底层 VM 语义，转 authority lane，不在功能支线硬猜。
