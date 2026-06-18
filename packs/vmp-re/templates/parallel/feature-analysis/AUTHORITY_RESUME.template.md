# AUTHORITY_RESUME：<SESSION_NAME>

当主线已结束但功能会话遇到 VM 阻塞时，用此提示词开临时 authority 会话。

读取：

- `.rekit/parallel/<SESSION_NAME>/checkpoints/latest.json`
- `<WORKSPACE>/lowering_requests.csv`
- `references/vmp-re/task-handoff.md`

任务：只处理功能会话提交的高价值 lowering request。处理后不要直接修改功能结论；通过 `/rekit parallel <SESSION_NAME> sync` 回传。
