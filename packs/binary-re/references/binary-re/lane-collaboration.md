# VMP RE 工作线协同

> 用途：在 VMProtect x64 case 中，把底层 handler lowering 主线与具体功能分析支线协同推进，同时保持 confirmed 数据单写者和上下文可续接。

## 最短使用

在 case 中日常只需要：

```text
/steamai overview
/steamai continue main
/steamai continue <feature-name>
/steamai start <feature-name>
/steamai handoff
/steamai handoff <feature-name>
```

`overview` 只是项目总览，不代表当前会话已选择工作线。如果 `<feature-name>` 不存在，`start` 会创建功能分析工作区和接续提示；`continue main` 接手主线，`continue <feature-name>` 接手指定功能支线；无参数 `handoff` 生成项目级索引，`handoff main` / `handoff <feature-name>` 生成指定工作线接手文档。以下路径按 resolved state root 投影；current 项目使用 `.steamai`，legacy-only 项目使用 `.rekit`。

## 推荐分工

批量合入审查或 request/candidate 复核可以由主 agent 按固定分片交给只读子 agent；主线仍独占 confirmed CSV、验证和 handoff 更新。

| 工作线 | 职责 | 可写 |
|---|---|---|
| 主线 | handler lowering、confirmed CSV、routine IR、handoff、验证 | canonical 文件 |
| 功能支线 | 功能入口、字符串/import/xref、native wrapper、证据、VM 阻塞点 | 自己的 workspace |
| 合入审查 | 只读审查功能支线产物，给主线建议 | 不写 |

主线/支线不是能力高低，而是写入权限不同：主线维护最终结论，功能支线收集证据、提出候选和 request。

## VMP feature workspace

默认路径：

```text
captures/feature_analysis/<feature-id>/
```

关键文件：

```text
summary.md
evidence.md
notes.md
lowering_requests.csv
observations.jsonl
requests.jsonl
candidates.jsonl
publications.jsonl
candidates/handler_roles.csv
candidates/opcode_semantics.csv
```

内部状态和接续提示位于：

```text
.steamai/lanes/<laneId>/prompts/RESUME.md
.steamai/handovers/latest.md                 # 项目级索引
.steamai/handovers/<laneId>-latest.md        # 指定工作线接手文档
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

如果主线已经完成而功能支线还没完成，将支线标记为 standalone / needs-authority 后继续 native 周边、字符串/import/xref 和证据整理；需要新的底层语义时继续写 request，未来由主线或临时主线会话处理。

## 禁止

- 功能支线不写 `captures/vm_opcode_semantics_confirmed.csv`、`captures/vm_handler_roles_confirmed.csv`、`references/binary-re/task-handoff.md`。
- 不并发写 IDB 注释/rename/type。
- 不把完整 trace、disasm、decompile、dump 内容复制进 Markdown。
- 不 promote 样本名、RVA/VA、ctx/round、captures/artifacts 路径。
