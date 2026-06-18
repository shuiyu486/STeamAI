# 工作线协同与连续性

目标：让一个 case 可以同时推进一条主线与多个功能支线，并通过持久化状态、短接续提示、候选产物、共享事实和 review-first 合并来避免上下文污染与互相覆盖。

> 内部 schema 仍使用 `lane` 这个字段名；用户层统一说“工作线 / 主线 / 功能支线”。

## 核心模型

- **主线**：维护权威数据、最终写入、验证和长期 handoff。它可以是长期主会话，也可以是临时恢复出来的主线会话。
- **功能支线**：围绕一个窄功能自由探索，写自己的工作区、request、candidate 和 evidence。
- **合入审查**：只读审查支线产物，把结论分为 confirmed candidate、needs-authority、case-only、promote-candidate。

## 默认入口

日常不要记大量子命令。默认使用：

```text
/rekit overview
/rekit continue
/rekit start <name>
/rekit handoff
```

`overview` 看项目状态，`continue` 自动整理和推进，`start` 创建功能支线，`handoff` 生成新会话接手包。显式底层动作只用于内部 runtime、排障和自动化。

## 持久化续接

每条工作线必须保存：

```text
.rekit/lanes/<laneId>/lane.json
.rekit/lanes/<laneId>/events.jsonl
.rekit/lanes/<laneId>/tasks.jsonl
.rekit/lanes/<laneId>/checkpoints/latest.json
.rekit/lanes/<laneId>/prompts/RESUME.md
```

聊天上下文不是事实源。跨天、重启电脑、`/compact` 后上下文污染，均通过 `.rekit/handovers/latest.md`、`RESUME.md` 与 `checkpoints/latest.json` 接续。

## 写入边界

功能支线可以写自己的工作区，但默认不能写 canonical 文件。主线负责：

- confirmed data 写入；
- shared database / external tool state 写入；
- final validation；
- main handoff update。

功能支线产物进入 canonical 前必须 review-first。

## 生命周期

建议状态：

```text
open -> needs-review -> reviewed -> synced -> standalone | closed
```

主线结束但功能支线未结束时，将支线转为 `standalone` 或 `needs-authority`。它仍可继续分析、收集证据和提交 request，但不能自动写 canonical。

## 产物契约

通用功能支线 workspace 至少包含：

```text
summary.md
evidence.md
notes.md
lowering_requests.csv 或 domain_requests.csv
candidates/
inbox/
outbox/
prompts/
```

pack 可以扩展领域字段，但不要把大 trace、dump、artifact 内容复制到 Markdown。
