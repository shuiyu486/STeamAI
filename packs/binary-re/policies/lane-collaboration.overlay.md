# VMP 成员协同 overlay

本 overlay 补充 VMProtect x64 trace/devirtualization 的目录成员协作边界，不定义 lane/runtime 状态。

## 角色边界

- handler owner：维护 handler role/opcode semantics candidate、routine IR 与验证计划。
- feature owner：分析功能入口、native wrapper、字符串/import/xref、证据与 VM 阻塞点。
- Reviewer：只读 finding/evidence，给出 lowering 优先级和审查判断，只写 review。

## 单写者

shared IDB、confirmed CSV、routine IR 和最终交付必须由 Commander 指定一名 owner。其他成员只提交 evidence、finding 或定向 lowering request。

## 回流分类

- 通用多会话协作：common policy candidate。
- VMP handler/trace/value-flow 规则：本 overlay 或 `references/binary-re/*.md`。
- 可参数化工具流程：`tooling/recipes/*.md`。
- 具体 RVA/VA、ctx、round、coverage、artifact：只留 case。
