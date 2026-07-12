# CTF challenge triage recipe

## 目标

在不连接远程服务、不执行高流量动作的前提下，建立 challenge 别名、类别假设、artifact 摘要、约束条件和 open questions 的最小索引。

## 输入

- 授权范围摘要。
- case-local challenge alias 或 artifact id。
- 已脱敏的题目描述、hint、file metadata、strings、协议/schema、pcap summary 或 local note sidecar。

## 输出

```text
challenge_ref, category, artifact_ref, primitive_hypothesis, constraints, evidence_ref, open_questions
```

输出写入 case-local sidecar 或 lane workspace；聊天和 Markdown 只引用摘要与路径。

## 止损

- 发现 flag、远程地址、账号凭据、payload、solver 私有脚本、比赛私有信息或未公开题解时停止提升到 pack，保留在 case-local 并标记 redaction needed。
- artifact、pcap、dump 或 trace 过大时先按 category / component 分片，不把完整内容粘入聊天或 Markdown。
- 证据不足以判断解题路线时产出 request，不直接升级为 confirmed。
