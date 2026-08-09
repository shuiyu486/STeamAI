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
7. packet-bound Reviewer 的 launch 必须从 strict durable dispatch receipt path/SHA 及其绑定的 exact packet path/SHA 构造结构化 packet/route/shard/ordered-items/session/output-contract identity；direct host、generic external-session package 与 detached supervisor child 必须无损传递并在启动前重验，不能把自由文本 `ExpectedOutput` 当作可信 identity。
8. packet-bound Reviewer 的真实结果必须以 outer reviewer plan-bound 的 in-memory snapshot 进入 canonical route；写 result/submission 或 generic relay artifact 前先按同一 durable receipt 与 packet 重验 packet/route/shard/ordered-items/session，并要求 `routeOutput` 精确包含 output contract 的全部 non-empty string fields且没有unknown field；随后在 shard lock 内重验dispatch prompt exact bytes/SHA、current dispatch/currentness与snapshot identity。placeholder、drifted identity或不完整pack-specific output不得落盘，也不允许先发布host relay source再以其充当authority。
9. completion 除了验证 accepted decision/verdict、packet/session/input/receipt/current-owner lineage，还必须要求 packet shard items 与 canonical `ReviewerResult.items` 完全一致且都绑定当前 member manifest；ledger `target` 只负责选择该 manifest 的 candidate，不能代替 reviewed-items 证明。
10. TaskContext 的 action-ready currentness 与终态历史快照验证必须分层：执行/发布前继续严格绑定当前 RESUME/checkpoint/owner/correction 和 exact pack contract；completion 合法刷新 lane 文档后，receipt 只能按 immutable TaskContext 内部 artifact hashes、mission intent 与当前 exact pack manifest/route/fields复核历史attempt，不能要求历史artifact SHA仍等于新的lane文档。
11. 跨 case consumer 只允许当前 owner generation 的 strict `pack-memory-consumer` binding；其 `changeId`、source/receipt/plan SHA 必须与 current selected-sync receipt 一致，quote 必须来自 predecessor 到 accepted successor 的新增内容。

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

`cmd/rekit-host` 是 deterministic `rekit` runtime 外的 Go-owned session executor；它不增加 public runtime command 或平行 receipt 协议。入口只接收目标 case 及可选 actor/model/timeout/attempt budget；省略 `-pack` 时在任何 host操作前从 attached case metadata解析并固定 pack，使parent、supervision spec/child与启动前validator共享同一identity。其余 attempt/session identity、RFC3339Nano 时间、路径和 SHA 均从 `run-current-step` 自动取得或生成。

```text
go run ./cmd/rekit-host -target <attached-case>
```

host 以 `--safe-mode -p --output-format json --json-schema ... --session-id <uuid> --permission-mode dontAsk` 启动真实 Claude Code；允许工具固定为只读 `Read,Glob,Grep`。进程启动后立即记录 accepted launch，完成后只把真实 `structured_output` 的 bytes 写入 member outputs 或 ReviewerResult，再按 runtime 模板最后写 submission，并循环执行 result-turn/strict intake。durable member attempt与external-session attempt是不同命名空间；member launch通过immutable task-context path/SHA、task自身lane/attempt inspection和currentness绑定，不要求两个attempt ID字符串相等。pack-memory focus下钻时，current-loop checkpoint绑定实际case request，fresh status才能继续unified external session而不重复exact dispatch。进程失败或结果非法时，在 attempt budget 内自动生成 replacement generation；最后一次才提交 failed observation。

### 失败与 replacement

- spawn 前失败：记录 failed launch，自动请求 replacement generation。
- session 非零退出或无合法结果：记录失败原因，保留 stdout/stderr 的有界诊断，自动 replacement；RH-07仓库外receipt只投影bounded typed phase/attempt failure，并按Windows大小写与slash/backslash等价规则脱敏known child-host、Claude和isolated-kit路径。
- 结果写入后 submission/intake 失败：不得重跑已完成 session；从 durable artifacts 恢复 exact submission/intake。
- replacement 启动后旧 session 的迟到结果必须被现有 generation/currentness guard 拒绝。

## Live gate

live gate 独立于普通 hermetic `go test`：它调用真实 Claude Code，使用无敏感内容的临时任务，并输出机器可读 receipt。至少覆盖：

1. 自然语言创建并启动任务；
2. 第一代真实 member 返回明确不满足 correction-only 要求的可验证产物；
3. 第一轮独立真实 Reviewer 在 `MaxAttempts` 预算内可以重试，但每次 launch 都必须真实 returned 并有 completion；最终必须形成 canonical `reject/rejected`，old manifest 随后 zero-launch、mutation-free 停止重放；
4. public intervention 写入与 rejected manifest、packet/result/input/receipts/events/session/historical owner 绑定的人工纠偏；
5. replacement session 从 durable handoff 接手，并在 immutable `TaskContext` 中读取原 goal、correction 与 canonical rejection evidence；
6. 第二代新 manifest 由独立新 Reviewer session canonical accept 后完成 feature lane；
7. fresh status 判断 mission complete；
8. 断言测试代码未写 ReviewerResult/member output/submission 内容；
9. receipt 为 `passed=true`、`manualPlaceholders=0`、`manualResultWrites=0`、feature lane `closed`、`cleanup=removed`。

显式命令示例：

```text
go run ./cmd/rekit-host -live-acceptance -pack "<_template-or-web-security>" -goal "<bounded-natural-language-goal>" -correction "<human-correction>" -receipt "<outside-case-receipt.json>"
```

省略 `-pack` 时仍回归 fresh 默认 `vmp-re`；跨 pack gate 只允许 `_template` / `web-security`，并要求 current TaskContext 的 manifest SHA、feature-analysis route 与 output fields 来自 exact selected pack，Reviewer rejection/acceptance route 与其 exact match。ordinary daily 不接受 pack override。普通 `go test ./...` 不执行该 gate。2026-08-06 最终代码的 fresh `vmp-re` fixed-15 实测得到两次真实 member completion、一次独立真实 Reviewer completion、零手工 placeholder/result write、严格 reviewer lineage、feature-lane completion 与成功自动清理；全仓 `go test ./... -timeout 40m -count=1`、`go vet ./...`、release-check/status/packs/doctor 与 diff check 同轮通过。

RH-09 Windows 连续试用显式运行：

```text
go run ./cmd/rekit-host -live-soak-acceptance -goal "<bounded-natural-language-goal>" -correction "<human-correction>" -receipt "<outside-repository-receipt.json>"
```

该聚合 gate 顺序执行默认 `vmp-re`、`_template`、`web-security` 三条既有 exact-pack 真实链，并追加既有 supervision 五阶段中断恢复；运行失败不删除记录，仍尝试后续任务和恢复并发布最终 receipt。Receipt 分开汇总 task-level 最终成功率与 attempt-level 原始成功率；只有 typed `reviewer-semantic-or-lineage` 失败允许一次 fresh-case retry，首次失败仍进入 attempts、failure counts、session、duration 与 cleanup totals，provider/contract/intake/cleanup/timeout 等失败不自动 retry。通过要求 task-level 3/3、fresh/existing/correction/replacement/Reviewer/replay 全闭合、恢复 durable identity 与一次 output publication 完整、全部 disposable case root 真实创建并清理、`manualPlaceholders=0`、`manualResultWrites=0`。
