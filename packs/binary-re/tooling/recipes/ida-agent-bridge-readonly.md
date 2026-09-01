# IDA sidecar bounded review

## 目的

在已有 case-local IDA export/sidecar 上做 bounded、只读查询，让成员和 Reviewer 引用少量 selected rows，而不是把完整 IDA 输出灌入上下文。本 recipe 不安装或启动 IDA，不打开 IDB，不联网，不修改 rename/comment/type/patch。

## 输入边界

```text
<case-relative-export>/
  function_index.tsv   # 必需
  strings.tsv          # 可选
  imports.tsv          # 可选
  xrefs.tsv            # 可选
```

- 使用 1～16 个 literal terms；不支持 regex、glob 或 DSL。
- 每个输入、每行、每类 selected rows 和总 packet 都必须有上限。
- request 绑定每个输入的相对路径、SHA-256 和 bytes；读取前后漂移则停止。
- 输出只写 case-local bounded packet；Markdown 只引用 selected row、摘要和 evidence ref。

## 使用流程

1. 确认 export 已存在；缺失时停止，不自动生成。
2. 读取小索引并筛选与任务直接相关的少量 rows。
3. 必要时只读取 selected object 的短 pseudocode/disasm/xref/hexdump sidecar 片段。
4. owner 写 evidence/finding；Reviewer 只读 packet 和 evidence 后写 review。
5. 需要全量导出、启动 IDA、shared IDB writeback、debug、patch、dump 或 network 时，回到独立 heavy-action request。

## Heavy action request

向用户展示 exact action/target、为何现有 sidecar不足、隔离、预算、case-relative output、rollback 和 stop conditions；只有用户具体确认且 Claude Code 工具权限允许时执行。只读 sidecar review 本身不授权任何新导出或 IDB 修改。

## 风险

- export 可能相对当前 IDB 陈旧，必须标记来源与限制。
- pseudocode 不是 verifier；关键 handler 结论仍需指令级、trace/value-flow 或独立 Reviewer 支持。
- 不把真实样本名、绝对路径、完整 trace/dump/decompile/disasm 写回 pack。
