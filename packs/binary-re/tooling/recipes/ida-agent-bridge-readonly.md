# ida-agent-bridge read-only index adapter

## 读取指南

- 本文定义 `ida-agent-bridge` 的只读 index adapter contract，用于 Agent 在已有 IDA sidecar/export 上做 bounded 查询。
- 需要实际操作 IDA、生成全量导出、rename/comment/patch 或连接外部服务时，不适用本文的默认授权，必须走 heavy-tool gate 或用户显式确认。
- 日常引用时优先读取“实施摘要”“执行清单”“验证标准”和“Packet schema”；只有实现 adapter 或排查输出漂移时再读完整细节。

## 实施摘要

外部 `ida-agent-bridge` 在本 pack 中保持 `candidate` 状态，不安装、不连接也不由 catalog `entry` 启动。DPC-04 另行落地了 compiled-in `vmp-ida-index-inspector`：它只消费本 contract 中已经验证的固定 TSV 子集，以 literal query 生成 bounded packet，并在 exact generated profile + canonical `authorized-gate` 下进入 dispatch/receipt/observation、独立 evidence review、member 与 Reviewer 链。

fixed runtime path 只读取已有 `function_index.tsv`（必需）和可选 `strings.tsv` / `imports.tsv` / `xrefs.tsv`；不读取 pseudocode/disassembly sidecar，不打开 IDB，不启动 IDA，不联网。更广的 bridge 能力仍只是 candidate recipe，任何全量导出、IDB 修改、调试、patch/dump 或外部连接都不属于默认路径。

## 执行清单

- [ ] 确认已有 bridge/export 目录；缺少 export 时停止，不自动启动 IDA 生成。
- [ ] 对 fixed runtime path 只使用 1～16 个 literal term，先生成内容寻址 request 和 exact profile preview。
- [ ] 只有 profile 明确确认并得到 current `authorized-gate` 后，才运行 compiled-in `vmp-ida-index-inspector`。
- [ ] 复核 bounded packet/report/receipt/observation、profile 已恢复 manual，以及 independent evidence review 决策。
- [ ] member/Reviewer 只引用 exact selected row、evidence ref、packet、receipt 和 observation lineage，不复制完整大输出。
- [ ] 若需要 pseudocode/disasm/hexdump、rename/comment/patch、全量导出、动态 attach 或 dump，fixed adapter 停止并回到对应独立 gate/recipe。

## 验证标准

- packet 中所有路径使用 case-relative 或 export-relative 路径，不写本机绝对路径。
- packet 明确 `mode: read-only-index`、`sideEffects: ["filesystem-read", "bounded-packet-write"]`；写入只限 exact output root 的 bounded packet，不包含 rename/comment/patch/debug/dump/network 动作。
- 单个 packet 的 selected functions / strings / imports / xrefs 有上限；超限时写 `truncated: true` 与 `droppedCount`。
- Markdown / handoff / candidate 只写摘要和 evidence refs，不粘贴完整 decompile/disasm/hexdump。
- adapter 失败时写 dispatch-bound `failed` / `aborted` terminal report，零 packet artifact，且同一 dispatch 不重启 child；packet 内的 `errors[]` / `nextActions[]` 只用于已成功解析的有界检查结果。
- Windows contained child 在 resume 前发布 parent-owned launch proof；该 proof 使用 anchored create-only write-through publication。它覆盖进程崩溃恢复并强化文件/metadata 落盘请求，但不宣称跨所有文件系统与硬件的断电原子性。success seal 只在 exact packet/report 发布并完成最终 source、lane lease 与 runtime budget 检查后写入。

## 风险与注意事项

- IDA F5 结果不是 verifier；它只能提供静态线索，VMProtect handler 语义仍需 trace/value-flow 或独立 reviewer 复核。
- 全量导出可能耗时、占磁盘并泄漏样本结构；默认不自动触发。
- rename/comment/patch 会修改共享 IDB 状态，多 Agent 不并发写，且本 recipe 不授权这些动作。
- 不把真实样本名、绝对路径、完整 trace/dump/decompile/disasm 写回 pack 模板。

## Fixed runtime contract

`vmp-ida-index-inspector` 只接受以下输入：

```text
tooling/ida-agent-bridge/export/
  function_index.tsv   # 必需
  strings.tsv          # 可选
  imports.tsv          # 可选
  xrefs.tsv             # 可选
```

- query 为 1～16 个单行 UTF-8 literal term，每项 1～128 字符；case-insensitive substring matching，不支持 regex、glob、DSL 或 quoting/control characters。
- 每个文件最多 1 MiB、每行最多 64 KiB；每类最多 200 rows；packet 最多 256 KiB。
- request filename 绑定 canonical request bytes SHA-256；request 绑定每个输入的 path/SHA/bytes 与 aggregate SHA。
- profile 仅允许 exact `inspect`、该 request path、固定 output/stop conditions 和最长 15 分钟 expiry；child 前后 input/source drift 均 fail-closed。
- 成功路径为 request → profile → `authorized-gate` → immutable dispatch → compiled-in child → committed packet/report → receipt/observation → profile revoke → independent evidence review → member/Reviewer。
- `NoNetwork` 只表示该 fixed child 没有网络代码路径；不是 OS 级 socket 隔离。

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
  - bounded-packet-write
risks:
  - stale export compared with current IDB
  - large index/decompile output if not bounded
  - false confidence from pseudocode-only reasoning
stop_conditions:
  - export directory missing and generating it would require IDA automation
  - query would require full decompile/disasm dump
  - selected output exceeds packet limits
  - task needs rename/comment/patch/debug/dump/network
confirmation_required: false for reading existing bounded exports; the fixed bounded packet write requires its strict profile + authorized-gate; export generation or any other write/heavy action requires a separate confirmation
```

## Packet schema

Adapter 输出建议为 JSON sidecar，最小字段如下：

```json
{
  "schemaVersion": 1,
  "tool": "ida-agent-bridge",
  "mode": "read-only-index",
  "sideEffects": ["filesystem-read", "bounded-packet-write"],
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
