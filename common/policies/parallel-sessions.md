# Parallel sessions

目标：让一个 case 可以同时推进主线/authority 工作与多个功能探索会话，并通过持久化状态、短提示词、候选产物和 review-first 合并来避免上下文污染与互相覆盖。

## 核心模型

- **Authority session**：维护权威数据、最终写入、验证和 handoff。它可以是长期主会话，也可以是临时恢复出来的 authority 会话。
- **Feature session**：围绕一个窄功能自由探索，写自己的工作区、request、candidate 和 evidence。
- **Merge review**：只读审查 feature 产物，把结论分为 confirmed candidate、needs-authority、case-only、promote-candidate。

## 默认入口

日常不要记大量子命令。默认使用：

```text
/rekit parallel
/rekit parallel <name>
```

工具应根据状态提示下一步：创建、续接、collect、sync、review、standalone 或 close。显式子命令只用于排障和自动化。

## 持久化续接

每个 parallel session 必须保存：

```text
.rekit/parallel/<name>/state.json
.rekit/parallel/<name>/timeline.jsonl
.rekit/parallel/<name>/checkpoints/latest.json
.rekit/parallel/<name>/prompts/*_RESUME.md
```

聊天上下文不是事实源。跨天、重启电脑、`/compact` 后上下文污染，均通过 `*_RESUME.md` 与 `checkpoints/latest.json` 接续。

## 写入边界

Feature session 可以写自己的工作区，但默认不能写 canonical 文件。Authority session 负责：

- confirmed data 写入；
- shared database / external tool state 写入；
- final validation；
- main handoff update。

Feature 产物进入 canonical 前必须 review-first。

## 生命周期

建议状态：

```text
open -> needs-review -> reviewed -> synced -> standalone | closed
```

主线结束但 feature 未结束时，将 feature 转为 `standalone` 或 `needs-authority`。它仍可继续分析、收集证据和提交 request，但不能自动写 canonical。

## 产物契约

通用 feature workspace 至少包含：

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
