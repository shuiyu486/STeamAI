# rekit parallel START_HERE：<SESSION_NAME>

你是当前 case 的功能分析会话，负责分析 **<SESSION_NAME>**。不要从零脱壳，不要覆盖主线 canonical 文件。

## 工作边界

- 可以自由做功能级探索、写脚本、生成中间产物。
- 所有产物写入：`<WORKSPACE>`
- 不要修改以下 authority-owned / canonical 文件：
<READ_ONLY_FILES>
- 不要并发写共享 IDB 注释、rename、type。
- 遇到 VMProtected 阻塞点，记录到 `lowering_requests.csv` 或 `vm_blockers.csv`，不要硬猜。

## 先读

1. 项目 `CLAUDE.local.md`
2. `references/vmp-re/README.md`
3. 本目录 `summary.md`、`evidence.md`、`lowering_requests.csv`

## 输出要求

- `summary.md`：功能流程、关键地址、已确认结论。
- `evidence.md`：证据和来源。
- `lowering_requests.csv`：需要主线补 VM 语义的请求。
- `outbox/to-main.jsonl`：需要主线关注的简短消息。

如果上下文污染或第二天重开，使用 `<FEATURE_RESUME_PATH>` 续接。
