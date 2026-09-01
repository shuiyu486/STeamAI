# VMP RE 成员协同

> 用于把 handler lowering 主线与具体功能分析并行推进，同时保持共享结论单写者。这里描述职责，不创建 lane 状态机。

## 推荐分工

| 成员 | 职责 | 默认写入 |
|---|---|---|
| Handler analysis owner | handler lowering、confirmed candidate、routine IR 验证 | 自己的 member workspace 与指定 evidence/finding |
| Feature analysis owner | 功能入口、字符串/import/xref、native wrapper、VM 阻塞点 | 自己的 member workspace 与指定 evidence/finding |
| Reviewer | 只读审查 evidence/finding，判断 accepted/needs-evidence/disputed | `reviews/` |

共享 confirmed CSV、IDB 注释/rename/type 和最终报告只能有一名 Commander 指定的写入 owner。其他成员提交 candidate/evidence，不并发写共享对象。

## 功能分析 workspace

使用 case-local、成员拥有的目录，例如：

```text
.steamai-vnext/members/<feature-member>/workspace/
  summary.md
  evidence.md
  lowering-requests.md
  candidates/
```

任务、停止条件与允许写入以该成员 `CLAUDE.md` 为准；跨会话消息不能覆盖更近的用户纠偏，也不能扩大授权。

## 阻塞请求

遇到 VMProtected 阻塞点时，向 handler owner 发送定向请求，包含 subject、原因、evidence ref、优先级、期望输出和停止条件。不要把 unknown handler 猜成业务逻辑。

## 禁止

- 功能分析成员不写共享 confirmed CSV、routine IR、最终报告或共享 IDB 状态。
- 不把完整 trace、disasm、decompile、dump 内容复制进 Markdown。
- 不把样本名、RVA/VA、ctx/round、artifact 路径回流 pack。
