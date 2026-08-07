# Autonomous Mission Control goal guide

## 读取指南

本文件只提供简短 goal，不是路线、接手清单或产品设计。先由 `docs/context-routing.md` 选择当前场景；真实使用路线的 current/state/next 只读 `docs/real-usage-hardening-roadmap.md`，不要从本文件、聊天摘要或历史批次选题。

## 实施摘要

聊天 goal 只负责启动或继续**已批准路线**。它不授权自由创造批次，不授权 commit/push，不授权外部副作用，也不放宽 Human-in-the-Lane、WhatIf/hash-bound Apply、strict intake、heavy action gate 或 authority/confirmed 边界。

## 执行清单

1. 从 router 指向的 active source 读取当前卡；真实 git、源码和测试结果可纠正文档中过期事实。
2. 当前卡为 `in_progress`/`blocked` 时只继续该卡；完成后只领取 active source 明确解锁的下一卡。
3. 如果代码事实推翻当前卡，先更新 active source 的理由、验收和指针；不得从候选池、future backlog、batch history 或聊天摘要静默跳批。
4. 每批完成 focused/live/Windows 本机验证和文档写回后才可关闭；commit/push 仍需当前会话明确授权。

## 验证标准

- goal 保持短，不复制 read-first 清单、未来批次、完整验证命令或产品方向。
- 新会话只靠 router + active source + 真实仓库状态即可恢复唯一允许工作。
- 默认继续自主推进仅表示继续**已批准路线**；未通过当前卡门槛时保持 open。

## 风险与注意事项

- `docs/real-usage-hardening-backlog.md` 只保存未来卡，不是当前选题源，不默认读取。
- heavy/debug/patch/dump/hook/network/exploit replay 仍要求 strict durable autonomy profile 与 `authorized-gate` 完整覆盖。
- 产品方向变化、confirmed/authority 策略变化、runtime schema 迁移、无完整替代的公共入口删除或未授权外部副作用必须升级。

## 给新会话的 goal 语句

```text
继续执行 re-context-kits 的已批准真实使用加固路线。先按 docs/context-routing.md 选择场景，只领取 docs/real-usage-hardening-roadmap.md 当前卡；完成该卡全部真实验收并同步指针后，才能继续明确解锁的下一卡。不要从 future backlog、历史或聊天摘要另选工作，也不要自动 commit/push。
```
