# VMP RE lane collaboration

> 用途：在 VMProtect x64 case 中，把底层 handler lowering 主线与具体功能分析支线协同推进，同时保持 confirmed 数据单写者和上下文可续接。

## 最短使用

在 case 中日常只需要：

```text
/rekit board
/rekit auto
/rekit lane start feature-analysis <feature-name>
/rekit lane resume <laneId>
```

如果 `<feature-name>` 不存在，`lane start` 会创建 feature workspace 和续接提示词；如果已存在，用 `lane resume` 刷新续接提示，`auto` 负责收集、验证、路由和发布低风险事实。

## 推荐分工

| lane | 职责 | 可写 |
|---|---|---|
| Authority / main lane | handler lowering、confirmed CSV、routine IR、handoff、验证 | canonical 文件 |
| Feature lane | 功能入口、字符串/import/xref、native wrapper、证据、VM 阻塞点 | 自己的 workspace |
| Merge review | 只读审查 feature 产物，给主线建议 | 不写 |

## VMP feature workspace

默认路径：

```text
captures/feature_analysis/<name>_<yyyymmdd>/
```

关键文件：

```text
START_HERE.md
summary.md
evidence.md
vm_blockers.csv
lowering_requests.csv
candidates/handler_roles.csv
candidates/opcode_semantics.csv
prompts/FEATURE_RESUME.md
inbox/from-main.jsonl
outbox/to-main.jsonl
```

## request 规则

遇到 VMProtected 阻塞点时，写 `lowering_requests.csv`：

```csv
request_id,feature,rva,handler,reason,evidence,priority,status,main_response,notes
```

- `status` 为空、`open`、`pending`、`needs_authority` 均表示未处理。
- `resolved`、`done`、`closed`、`accepted`、`rejected` 表示主线已处理。
- 不要把 unknown handler 猜成业务逻辑；记录证据和需求。

## 主线结束后的 standalone

如果主线已经完成而 feature 还没完成，将 feature lane 标记为 standalone / needs-authority 后继续 native 周边、字符串/import/xref 和证据整理；需要新的底层语义时继续写 request，未来由 authority lane 或临时 authority 会话处理。

## 禁止

- Feature lane 不写 `captures/vm_opcode_semantics_confirmed.csv`、`captures/vm_handler_roles_confirmed.csv`、`references/vmp-re/task-handoff.md`。
- 不并发写 IDB 注释/rename/type。
- 不把完整 trace、disasm、decompile、dump 内容复制进 Markdown。
- 不 promote 样本名、RVA/VA、ctx/round、captures/artifacts 路径。
