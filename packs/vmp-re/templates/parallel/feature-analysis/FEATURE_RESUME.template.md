# FEATURE_RESUME：<SESSION_NAME>

你是功能分析会话的续接。不要从零开始。

## 读取顺序

1. `.rekit/parallel/<SESSION_NAME>/checkpoints/latest.json`
2. `<WORKSPACE>/summary.md`
3. `<WORKSPACE>/evidence.md`
4. `<WORKSPACE>/lowering_requests.csv`
5. `<WORKSPACE>/inbox/from-main.jsonl`

## 当前状态

- status: `<STATUS>`
- lifecycle: `<LIFECYCLE_MODE>`
- unresolved lowering requests: `<UNRESOLVED_REQUESTS>`
- candidate rows: `<CANDIDATE_ROWS>`

## 下一步

<NEXT_ACTION>

## 边界

继续把产物写入工作区，不要写 canonical 文件。需要主线处理的内容写入 `outbox/to-main.jsonl` 或 `lowering_requests.csv`。
