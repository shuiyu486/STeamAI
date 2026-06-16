# Context budget

本文件是兼容入口。长期规范已迁移到 policy registry：

- `common/policies/context-budget.md`：通用上下文预算与渐进式披露。
- `common/policies/subagents.md`：通用子 agent 分派与 bounded parallelism。
- `common/policies/tool-output.md`：大工具输出处理。
- `packs/<pack>/policies/*.overlay.md`：具体 pack 的领域化补充。

维护规则：新增横切规范优先放入 `common/policies/`；pack-specific 细节放入 `packs/<pack>/policies/`，不要在本兼容入口继续扩展长内容。
