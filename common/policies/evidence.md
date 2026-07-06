# Evidence, citations, and claim quality

本文件是 `agent-team.md` 的证据原则补充，不重复定义 packet schema。packet 字段、状态流、decision event schema 见 `agent-team.md` 与 `docs/evidence-ledger.md`。

目标：让分析结论可追溯、可复核，避免把猜测当事实。

## 证据原则

- 结论应绑定文件、行号、命令输出、hash、trace 片段或明确来源。
- 大段原始证据不进入主会话；只返回关键定位和摘要。
- 低样本、高风险或存在歧义的结论应标记置信度。
- 分歧结论默认 deferred，除非有独立验证。

## 子 agent 证据限制

每个 item 最多返回 3 条关键证据。需要更多证据时，返回证据文件路径和定位，让主 agent 按需读取。

## packet 与 event

evidence packet 是 agent 产出文件（带 `evidence_id`，写在 lane workspace）；runtime 从 packet 抽取 event append 到 `.rekit/facts/*.jsonl`。手动 append 入口是 `/rekit note`（见 `.claude/skills/rekit/SKILL.md`），auto 流程入口是 `/rekit continue`。event kind 与字段对齐 `docs/evidence-ledger.md` 草案（9 种 kind + 基础字段 + per-kind 扩展）。低样本、alias、未 cross-run 必须在 candidate `limitations` 写明。
