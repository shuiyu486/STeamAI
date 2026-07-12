# ida-agent-bridge read-only index adapter

## 读取指南

- 本文定义 `ida-agent-bridge` 的只读 index adapter contract，用于 Agent 在已有 IDA sidecar/export 上做 bounded 查询。
- 需要实际操作 IDA、生成全量导出、rename/comment/patch 或连接外部服务时，不适用本文的默认授权，必须走 heavy-tool gate 或用户显式确认。
- 日常引用时优先读取“实施摘要”“执行清单”“验证标准”和“Packet schema”；只有实现 adapter 或排查输出漂移时再读完整细节。

## 实施摘要

`ida-agent-bridge` 在本 pack 中保持 `candidate` 状态。D6 的目标不是安装、连接或驱动 IDA，而是把它能提供给 Agent 的只读索引抽象成稳定 contract：先消费小索引和 sidecar refs，再按需窄范围读取 pseudocode / xref / hexdump 摘要；Markdown 只记录 bounded summary 和 evidence refs，不粘贴完整 decompile/disasm。

默认允许的动作仅限读取已有 export 目录中的文本/JSON/TSV sidecar，并生成小型 adapter packet。任何会创建全量导出、修改 IDB、调试进程、patch/dump、联网安装或发布内容的动作都不属于本 recipe 的默认路径。

## 执行清单

- [ ] 确认已有 bridge/export 目录，或先记录需要用户确认的导出请求。
- [ ] 优先读取 `function_index.tsv`、`strings.tsv`、`imports.tsv`、`xrefs.tsv` 等小索引。
- [ ] 按任务选择少量函数 / 字符串 / import / xref，生成 bounded `ida-index-packet.json` 或等价 summary sidecar。
- [ ] pseudocode / disasm / hexdump 只允许按函数或地址范围窄读，并记录路径、行范围、hash 或 query id。
- [ ] 在 candidate / verification 中引用 packet path、query id、函数名/RVA 和摘要，不复制完整大输出。
- [ ] 若需要 rename/comment/patch、全量导出、动态 attach 或 dump，停止并走 heavy-tool gate。

## 验证标准

- packet 中所有路径使用 case-relative 或 export-relative 路径，不写本机绝对路径。
- packet 明确 `mode: read-only-index`、`sideEffects: ["filesystem-read"]`，且不包含 rename/comment/patch/debug/dump/network 动作。
- 单个 packet 的 selected functions / strings / imports / xrefs 有上限；超限时写 `truncated: true` 与 `droppedCount`。
- Markdown / handoff / candidate 只写摘要和 evidence refs，不粘贴完整 decompile/disasm/hexdump。
- adapter 失败时返回结构化 `errors[]` 与 `nextActions[]`，不无界重试。

## 风险与注意事项

- IDA F5 结果不是 verifier；它只能提供静态线索，VMProtect handler 语义仍需 trace/value-flow 或独立 reviewer 复核。
- 全量导出可能耗时、占磁盘并泄漏样本结构；默认不自动触发。
- rename/comment/patch 会修改共享 IDB 状态，多 Agent 不并发写，且本 recipe 不授权这些动作。
- 不把真实样本名、绝对路径、完整 trace/dump/decompile/disasm 写回 pack 模板。

## Capability card

```yaml
id: ida-agent-bridge-readonly-index
status: candidate
entry: external ida-agent-bridge export directory or read-only query bridge
purpose: Provide bounded static index packets for Agent review without pasting full IDA output into context.
inputs:
  - exportRoot: <case-relative bridge export root>
  - query: <function|string|import|xref selector>
  - limits: <max rows/chars per packet>
outputs:
  - ida-index-packet.json
  - bounded-summary.md
  - evidence refs to index rows or sidecar line ranges
side_effects:
  - filesystem-read
risks:
  - stale export compared with current IDB
  - large index/decompile output if not bounded
  - false confidence from pseudocode-only reasoning
stop_conditions:
  - export directory missing and generating it would require IDA automation
  - query would require full decompile/disasm dump
  - selected output exceeds packet limits
  - task needs rename/comment/patch/debug/dump/network
confirmation_required: false for reading existing bounded exports; true for export generation or any write/heavy action
```

## Packet schema

Adapter 输出建议为 JSON sidecar，最小字段如下：

```json
{
  "schemaVersion": 1,
  "tool": "ida-agent-bridge",
  "mode": "read-only-index",
  "sideEffects": ["filesystem-read"],
  "source": {
    "exportRoot": "tooling/ida-agent-bridge/export",
    "idbRef": "<idb-hash-or-human-label>",
    "generatedAt": "<optional timestamp from export metadata>"
  },
  "limits": {
    "maxFunctions": 20,
    "maxStrings": 50,
    "maxImports": 50,
    "maxXrefs": 100,
    "maxSnippetChars": 4000
  },
  "indexes": {
    "functions": {
      "path": "tooling/ida-agent-bridge/export/function_index.tsv",
      "totalRows": 0,
      "selected": [
        {
          "rva": "0x00000000",
          "name": "sub_00000000",
          "size": 0,
          "basicBlocks": 0,
          "reason": "selected by task query"
        }
      ],
      "truncated": false,
      "droppedCount": 0
    },
    "strings": {
      "path": "tooling/ida-agent-bridge/export/strings.tsv",
      "selected": []
    },
    "imports": {
      "path": "tooling/ida-agent-bridge/export/imports.tsv",
      "selected": []
    },
    "xrefs": {
      "path": "tooling/ida-agent-bridge/export/xrefs.tsv",
      "selected": []
    }
  },
  "snippets": [
    {
      "kind": "pseudocode|disasm|hexdump|xref-summary",
      "target": "sub_00000000",
      "sidecarPath": "tooling/ida-agent-bridge/export/functions/sub_00000000.pseudo.txt",
      "lineRange": "1-80",
      "summary": "bounded summary only; no full output in markdown"
    }
  ],
  "evidenceRefs": [
    "ida-index:functions:sub_00000000",
    "ida-sidecar:functions/sub_00000000.pseudo.txt#L1-L80"
  ],
  "warnings": [],
  "errors": [],
  "nextActions": []
}
```

## 使用流程

1. **发现已有 sidecar**：检查 case-local `tools.local.yml` 或工作区记录中是否有 bridge/export 路径；没有则生成 request，不自动启动 IDA。
2. **读取小索引**：先看 function / strings / imports / xrefs 的 bounded 行；按任务目标筛少量对象。
3. **窄范围补充**：只对 selected objects 读取 pseudocode/disasm/xref/hexdump sidecar 的短片段。
4. **写 packet**：生成 `ida-index-packet.json`，包含 query、limits、selected rows、summary、evidence refs、warnings/errors。
5. **进入 review loop**：candidate 只引用 packet 和 sidecar refs；verification 需要独立 reviewer 或 trace/value-flow 复核。

## 明确禁止

- 自动安装、下载或更新 `ida-agent-bridge`。
- 自动打开 IDA、连接远程服务或生成全量导出。
- rename/comment/patch、修改 IDB、写入 binary、dump 内存或动态调试。
- 把完整 decompile/disasm/hexdump/trace 内容复制进 Markdown、pack docs 或 handoff。
- 把真实样本路径、客户路径、本机绝对路径写入可回流模板。
