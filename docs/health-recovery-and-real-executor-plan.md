# 真实健康恢复与 Claude 会话执行闭环

## 读取指南

先读“实施摘要、执行清单、验证标准、风险与注意事项”。阶段 1 完成后，阶段 2 只按“真实执行者 MVP”章节实施；不要回到 external-session receipt/字段微调，也不要把测试生成的结果当作 Claude 输出。

## 实施摘要

本轮纠偏分三阶段：先恢复默认 `vmp-re` 产品路线和仓库测试的真实健康；再在 deterministic Go runtime 外补上仓库内 Claude Code host/executor；最后建立由真实进程驱动的自然语言验收门禁。用户不再填写 session ID、时间、结果路径或 SHA；所有 LLM 结果必须来自真实 Claude Code 进程，测试代码不得直接写入伪造结果。

## 执行清单

- [x] 统一 onboarding、status 与 start 的 lane ID 正反向解析。
- [x] 增加默认 `vmp-re` emitted-route public E2E。
- [x] 让 local validation evidence 从已验证 receipt steps 派生。
- [x] 完成全仓测试、vet、diff check 与真实临时 case 验证。
- [x] 实现真实 Claude Code host/executor：`cmd/rekit-host` 已完成真实进程启动、自动 attempt/claim、结果落盘、submission-last、strict intake、失败 replacement 与 durable structured-output recovery。
- [x] 建立自然语言 → 真实 session → 结果 → 纠偏 → replacement → 独立 Reviewer → feature completion 的 explicit live gate；2026-08-06 fresh `vmp-re` 验收通过并自动清理临时 case。

## 验证标准

固定产品验收链：

```text
用户自然语言开始任务
→ 自动启动实际 Claude 会话
→ 自动收取结果
→ 人工纠偏
→ 新会话接手
→ 完成任务
```

阶段 1 必须满足：

1. 默认 `vmp-re` 的 `InitialLane=feature-analysis-live-check` 经 onboard emitted `applyArgs`、status emitted overview、status emitted start preview/apply 后，精确创建同名 lane。
2. 创建后的 fresh status 不再返回 `committedMissionIntent` start bootstrap。
3. validated local receipt 的七个 canonical steps 与 status evidence 同源。
4. focused tests、完整 `go test ./...`、`go vet ./...`、`git diff --check` 通过。

阶段 2/3 必须满足：

1. 启动真实 `claude` CLI 进程或等价真实 Claude Code session，不允许测试 double 产出 LLM 内容。
2. host 自动生成并记录 harness/session identity、时间、路径、SHA 和 submission；用户不填写这些字段。
3. 启动失败、非零退出、超时/失联和 invalid result 能自动进入 replacement，并 fence 旧 generation。
4. live gate 记录用户显式操作数、手工 placeholder/文件写入数、session launch/completion 数和恢复步骤；owner generation、external attempt generation 与本次 host run 与 run-local launch ordinal 分字段记录。
5. 同一 member manifest 已有 strict accepted Reviewer lineage 后不再规划第二个 Reviewer；状态直接转向 evidence-bound feature completion。
6. accepted completion 必须从 canonical `ReviewerResult` 重验 `decision=accept` 与 `recommendedVerdict=accepted`；即使 packet、session、input SHA 和 completion receipt 都有效，账本也不能把真实 reject 结果伪称为 accepted lineage。

## 风险与注意事项

- 保留已存在 `feature-*` lane ID 兼容；不得为修复新建路线强制迁移旧 case。
- Go runtime 的授权边界不变：host 只能消费已授权的 durable request；heavy action 仍要求 strict autonomy profile + `authorized-gate`。
- 不新增 PowerShell runtime logic。
- 真实 Claude live gate 可能需要本机登录态与配额；缺失时必须报告为 live gate blocked，不能退化为伪造结果。
- 测试必须使用系统临时 case，结束后清理，不把 case-specific 数据写入仓库。

## 真实执行者 MVP

### 责任边界

- `rekit` runtime：继续拥有 request、claim、launch/submission receipt、currentness、replacement fencing 和 strict intake。
- 新 host/executor：拥有发现当前 job、自动 claim、启动/观察/停止真实 Claude Code、收集真实结果、写 result-first artifacts、提交 submission、刷新 current step。
- Claude session：读取 runtime 提供的 immutable prompt/handoff，执行允许的 lane 工作并按 host 约定返回机器可收取的结果。

### 最小产品入口

`cmd/rekit-host` 是 deterministic `rekit` runtime 外的 Go-owned session executor；它不增加 public runtime command 或平行 receipt 协议。入口只接收目标 case 及可选 actor/model/timeout/attempt budget，其余 attempt/session identity、RFC3339Nano 时间、路径和 SHA 均从 `run-current-step` 自动取得或生成。

```text
go run ./cmd/rekit-host -target <attached-case>
```

host 以 `--safe-mode -p --output-format json --json-schema ... --session-id <uuid> --permission-mode dontAsk` 启动真实 Claude Code；允许工具固定为只读 `Read,Glob,Grep`。进程启动后立即记录 accepted launch，完成后只把真实 `structured_output` 的 bytes 写入 member outputs 或 ReviewerResult，再按 runtime 模板最后写 submission，并循环执行 result-turn/strict intake。进程失败或结果非法时，在 attempt budget 内自动生成 replacement generation；最后一次才提交 failed observation。

### 失败与 replacement

- spawn 前失败：记录 failed launch，自动请求 replacement generation。
- session 非零退出或无合法结果：记录失败原因，保留 stdout/stderr 的有界诊断，自动 replacement。
- 结果写入后 submission/intake 失败：不得重跑已完成 session；从 durable artifacts 恢复 exact submission/intake。
- replacement 启动后旧 session 的迟到结果必须被现有 generation/currentness guard 拒绝。

## Live gate

live gate 独立于普通 hermetic `go test`：它调用真实 Claude Code，使用无敏感内容的临时任务，并输出机器可读 receipt。至少覆盖：

1. 自然语言创建并启动任务；
2. 第一个真实 session 返回可验证产物；
3. public intervention 写入人工纠偏；
4. replacement session 从 durable handoff 接手；
5. 第二个真实 session 完成剩余工作；
6. fresh status 判断 mission complete；
7. 断言测试代码未写 ReviewerResult/member output/submission 内容。
8. 一个 current manifest 只启动一个独立 Reviewer；accepted writeback 后不重复规划审核。
9. receipt 为 `passed=true`、`manualPlaceholders=0`、`manualResultWrites=0`、feature lane `closed`、`cleanup=removed`。

显式命令示例：

```text
go run ./cmd/rekit-host -live-acceptance -goal "<bounded-natural-language-goal>" -correction "<human-correction>" -receipt "<outside-case-receipt.json>"
```

普通 `go test ./...` 不执行该 gate。2026-08-06 最终代码的 fresh `vmp-re` fixed-15 实测得到两次真实 member completion、一次独立真实 Reviewer completion、零手工 placeholder/result write、严格 reviewer lineage、feature-lane completion 与成功自动清理；全仓 `go test ./... -timeout 40m -count=1`、`go vet ./...`、release-check/status/packs/doctor 与 diff check 同轮通过。
