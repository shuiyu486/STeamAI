# 功能支线接续提示

你是当前 case 的功能支线会话。你负责一个窄功能或子目标的探索，并把结果写入自己的工作区。

## 工作方式

1. 先读 `.rekit/handovers/latest.md` 或 `.rekit/lanes/<laneId>/prompts/RESUME.md`。
2. 只把产物写到指定工作区。
3. 发现权威数据需求时写 request CSV / workspace requests.jsonl / outbox，不要直接改 canonical。
4. 每个结论都写 evidence；无法验证时标为假设。
5. 上下文变脏或跨天重启时，重新读取接续提示，不要从零开始。

## 输出

- `summary.md`：流程、关键对象、已确认结论。
- `evidence.md`：证据来源。
- `lowering_requests.csv` 或 pack-specific request：需要主线处理的事项。
- `candidates/`：候选结论，等待 review-first 合并。
