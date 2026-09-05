# Write boundaries and external side effects

本文件偏**边界**（什么能写、主/子 agent 写权限、外部副作用分级）；需要审查的写入如何走 review 流程见 `review-first.md`。两者配合：边界决定是否需要审查，流程决定审查如何进行。

目标：把可逆的本地编辑、难以恢复的本地改动、外部动作清晰区分，避免误写、误删或误发布。

## 默认边界

- 只读探索不需要用户确认，但要避免无界读取大文件。
- 覆盖、删除、移动已有文件前，应确认目标和内容是否符合预期。
- 外部动作默认需要明确授权，例如 push、发布、远程 API 写入、PR 评论。
- 安全边界、凭据、权限、供应链相关动作必须更严格审查。

## 主 agent 与子 agent

- 主 agent 负责写入、验证和对外动作。
- 子 agent 默认只读。
- 若子 agent 需要写入，必须使用隔离工作区，并在主 agent 合并前审查 diff。

## Verified learning 写入

- replay/evaluation spec、run bundle、attestation 和 field outcome 采用 immutable/no-replace 或 append-only；失败、超时、无效输出、neutral、regressed 和 inconclusive 都保留，不能覆盖为成功。
- field outcome 仅在当前 case 用户对本份脱敏记录明确 opt-in 后写入；不得自动遥测、扫描或汇总其它 case。

## 报告要求

如执行失败、跳过或部分完成，必须如实说明：

- 已执行什么。
- 未执行什么。
- 失败原因和原始错误摘要。
- 是否有残留未提交或未验证状态。
