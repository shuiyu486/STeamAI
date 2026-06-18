# MAIN_RESUME：<SESSION_NAME>

你是主线/authority 会话的续接。目标是消费功能会话的 request/candidate，而不是重做功能分析。

## 读取

- `.rekit/parallel/<SESSION_NAME>/state.json`
- `<WORKSPACE>/lowering_requests.csv`
- 最近 review packet（如有）：`<LAST_REVIEW_ROOT>`

## 当前状态

- unresolved lowering requests: `<UNRESOLVED_REQUESTS>`
- candidate rows: `<CANDIDATE_ROWS>`
- unresolved candidate rows: `<CANDIDATE_UNRESOLVED>`

## 职责

- 只把功能会话产物当候选证据。
- confirmed CSV、routine IR、handoff 更新仍由本会话单写者处理。
- 处理完后运行 `/rekit parallel <SESSION_NAME> sync` 或写明主线响应。
