# VMP lane collaboration overlay

本 overlay 扩展 `common/policies/lane-collaboration.md`，用于 VMProtect x64 trace-based devirtualization case。功能支线公开入口使用 B3 lane/auto。

## VMP 角色边界

- Authority / main lane：维护 handler role/opcode semantics confirmed CSV、routine IR、superinstruction、task-handoff 和最终验证。
- Feature lane：分析某个业务功能的入口、native wrapper、字符串/import/xref、证据与 VM 阻塞点。
- Merge review：只读审查 feature 产物，给主线提供 lowering 优先级和候选合并建议。

## VMP 单写者

Feature lane 默认不得写：

```text
captures/vm_opcode_semantics_confirmed.csv
captures/vm_handler_roles_confirmed.csv
captures/routine_ir.*
captures/routine_ir.superinstructions.*
references/vmp-re/task-handoff.md
```

共享 IDB 注释、rename、type 也按单写者处理。

## VMP request 字段

`lowering_requests.csv` 用于把功能支线遇到的 VM 阻塞点交给 authority：

```csv
request_id,feature,rva,handler,reason,evidence,priority,status,main_response,notes
```

`vm_blockers.csv` 用于记录更宽泛的阻塞点：

```csv
blocker_id,feature,rva,va,kind,evidence,need,status,owner,notes
```

## standalone 规则

主线完成后，feature lane 可以 standalone 继续 native 周边分析和证据整理；新的 VM 语义需求保留在 request CSV，未来由 authority lane 或临时 authority 会话处理。

## 回流分类

- 通用多会话提示词 / 生命周期：common policy 或 common prompt。
- VMP 阻塞点、handler lowering、trace/IR 协作规则：本 overlay 或 `references/vmp-re/lane-collaboration.md`。
- 可参数化工具流程：`tooling/recipes/lane-collaboration.md`。
- 具体 RVA/VA、ctx、round、coverage、captures/artifacts：只留 case。
