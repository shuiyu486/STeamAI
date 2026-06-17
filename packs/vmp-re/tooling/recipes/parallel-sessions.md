# Parallel sessions tooling recipe

目的：用 `/rekit parallel` 给 VMProtect RE case 创建可续接、可汇总、可 review-first 合并的并行功能分析工作区。

## 日常入口

```text
/rekit parallel
/rekit parallel <feature-name>
```

工具会根据状态提示下一步。显式动作主要用于排障：

```text
/rekit parallel <feature-name> collect
/rekit parallel <feature-name> sync
/rekit parallel <feature-name> resume
/rekit parallel <feature-name> standalone
/rekit parallel <feature-name> close
/rekit parallel doctor
```

## 生成内容

- `.rekit/parallel/<name>/state.json`：机器状态。
- `.rekit/parallel/<name>/timeline.jsonl`：事件流。
- `.rekit/parallel/<name>/checkpoints/latest.json`：短 checkpoint。
- `.rekit/parallel/<name>/prompts/*_RESUME.md`：新会话续接提示词。
- `captures/feature_analysis/<name>_<date>/`：功能会话工作区。

## 设计原则

- CLI 负责状态、模板、汇总和 review packet；LLM 负责解释建议与实际分析。
- Feature session 可写自己的 workspace，不写 canonical。
- `collect/review/promote` 只生成审查包；confirmed 合并仍由 authority session 执行。
- 跨天重启或上下文污染时，重新读取 `FEATURE_RESUME.md`，不要从零开始。

## 止损

如果 feature workspace 长期只有 request 没有 evidence，先退回补证据；如果 request 需要底层 VM 语义，转 authority session，不在功能会话硬猜。
