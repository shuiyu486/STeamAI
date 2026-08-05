# Batch implementation plan

## 读取指南

先读本节的 current milestone / current batch state / next candidates；旧批次只在需要考古、验证历史决策或 release/debug 溯源时按 Batch ID 搜索 `docs/batch-history.md`。不要默认从 Batch 0 顺序重读数百个历史批次。产品方向以 `docs/mission-control-product-direction.md` 为准，持续执行方式见 `docs/autonomous-goal.md`。

## 实施摘要

Batch 359 后，Go-owned/no-fallback public command surface、durable lanes、显式 reconcile、typed autonomy preflight、Mission brief / executor action、bounded reviewer dispatch → strict intake → verification-before-decision writeback → post-validation 的本机闭环、pack-memory promote/reconsume package E2E，以及 authorized execution observation evidence + bounded adapter execution report strict intake（含 authorized stopCondition boundaryHits、status summary enforcement、workspace-relative 与 case-relative machine-readable handoff）已形成底座。当前阶段继续从 contract/inventory field increments 转向 replaceable session executor / reviewer orchestration、lane executor / tool-adapter live validation、pack-memory product UX、Windows 本机 product-path 稳定与真实 release verification。

## 执行清单

### Current milestone

**最低可用 Mission Control 收敛 + truthful release readiness**：当前阶段从继续打磨底层零件切到尽快真实用起来。优先让用户能用自然语言开始 case、继续推进、查看状态、人工插手纠偏、新会话接手；允许半自动，但必须顺畅、可记录、可恢复。2026-08-01 已恢复真实 Linux/macOS/Windows release green；当前重新聚焦 Mission Commander 长期 run-loop，在不放宽每段 WhatIf/hash-bound Apply 与 Human-in-the-Lane review 边界的前提下，减少主 Agent 跨 lane/reviewer segment 持续推进时的手工预算与路由编排。

### Next candidates / goal guardrails

用户仍希望继续用短 goal 长期推进；goal 只负责启动自主推进，不承载完整路线和停止条件。压缩上下文后优先按 `docs/autonomous-goal.md` 顶部的“先真实用起来”阶段方向选择下一批：围绕开始 case、继续推进、查看状态、人工插手纠偏、新会话接手、reviewer/subagent 接手或 pack-memory 复用做一个 Windows 本机可验证产品闭环；不要继续把 adapter provenance / live validation / summary / projection 拆成单字段微调。

首选候选：

1. **最低可用 Mission Control 路线**：把开始 case、继续推进、状态总览、人工插手纠偏、新会话接手串成一条可复制、可验证、可恢复的日常路线。
2. **Mission Commander run loop MVP**：主 Agent/harness 实际驱动 ready lane 或 reviewer session 的最小 run loop；Go runtime 只记录 request/receipt/state，不 spawn/poll/stop Claude Code 进程。
3. **Reviewer/session orchestration UX**：把 ready/running/failed/stale/completed/source-capture/intake 的 operator next step 做成一条可复制、可恢复、可验证路径。
4. **Pack-memory product UX**：把 promote/reconsume 从 proof chain 推进为跨 case 可消费的 review-first workflow。
5. **嵌入式可维护性收敛**：只在上述 slice 中拆巨型 CLI/projection/test 或类型化 action source/state，不单独做大重构批。

### Current batch state









### Batch 819：external session harness launch and return package closure

状态：已完成。

目标：把 external member/reviewer session 的外部 harness 启动、replacement owner 和回传协议收敛为同源 stateful `externalSessionJob.harnessPackage`。Fresh job只给review-first attempt request；记录immutable attempt后，running package一次提供exact launch input path/SHA、tool/agent type、current generation owner、每个outcome的strict submission JSON、result-first writes、submission-last路径和fresh status命令。Invalid/replacement与submission-ready分别撤销旧launch并路由replacement review或复合external-result turn；status JSON/text、durable Markdown和replacement takeover消费同一对象。

边界：Member launch SHA绑定`memberexecution` durable commit已认证的handoff bytes，Inspect后二次读取发生替换时fail-closed；reviewer prompt bytes同样必须匹配dispatch SHA。Go runtime不spawn/poll/stop外部session，不调用Agent tool，不执行heavy-tool，不写authority/confirmed；本批不新增public command或PowerShell runtime logic，不自动执行reviewer/adapter/pack-memory/gate/sync/promote mutation，也不等待或声明remote CI green。

验证结果：harness/renderer/replacement takeover focused regressions通过；受影响`externalsession/currentloop/memberexecution/subagents/workstream/mission/cli`七包完整回归通过（CLI 382.446秒）；修复后全仓`go test ./... -count=1`通过（CLI 223.115秒），`go vet ./...`、status、10-pack packs、doctor与`git diff --check`通过。独立终审发现text/Markdown遗漏strict template内容及member二次读取SHA未绑定durable commit两项Important；修复后原审查者定向确认均关闭，无剩余高置信Critical/Important。完成态`release-check -Format json`返回`ready=true`；统一`release-run -Format json`以7/7通过（511.536秒，其中完整Go tests 508.352秒），全部步骤attempts=1并生成Git-local v2 validation receipt。Implementation commit/push随后记录；普通batch不等待或声明remote CI green。

### Batch 818：external session harness run-loop closure

状态：已完成。

目标：把外部member/reviewer session的启动所有权、replacement takeover、result-first/submission-last回传与既有reviewed external-result turn串成durable机器协议。Fresh status给出非可执行占位attempt template；具体identity经WhatIf/hash-bound Apply记录immutable generation，replacement精确supersede current receipt。每代使用独立submission/output/result namespace，旧session迟到写入不阻塞current owner；relay与replacement共享project mutation lease，terminal publication绑定exact attempt identity并继续既有strict intake/checkpoint resume。

边界：Go runtime只记录request/receipt/state，不spawn/poll/stop外部session，不调用shell/Agent tool，不执行heavy-tool，不写或推断authority/confirmed。PowerShell只做参数验证和逐字透传，不解析attempt/submission、不计算generation/hash/currentness。合法current submission阻止replacement；invalid submission保留警告并允许显式下一代恢复；后半段失败保留truthful relay。

验证结果：`externalsession`完整测试、external-session CLI focused suite、attempt/relay重复与fencing/invalid recovery压力回归、受影响`externalsession/currentloop/memberexecution/subagents/workstream/mission/cli`七包完整回归（最终CLI 241.894秒）及PowerShell façade smoke通过；全仓`go test ./... -count=1`通过（CLI 238.690秒），`go vet ./...`、status、10-pack packs、doctor、canonical skill预算32730/32768 bytes与`git diff --check`通过。独立终审发现并关闭lease内live job重建、accepted reviewer dispatch exact session lineage、public attempt committed replay/exact receipt SHA三项Important，定向复核无剩余高置信Critical/Important。`-race`额外检查因本机Go未启用cgo而未运行，不影响既有确定性lease竞态回归；统一`release-run -Format json`以7/7通过（482.532秒，其中完整Go tests 479.789秒），全部步骤attempts=1并生成Git-local v2 validation receipt。Implementation commit/push与fresh post-push接力随后记录，普通batch不等待或声明remote CI green。

## 活动文档维护规则

- 本文件只保留当前批次与最近完成批次，常态最多出现 2 个 `### Batch N` 段落；没有进行中批次时只保留最近完成批次。
- 规划新批次时，`next-batch` 先把更早的 active batch 原样追加到 `docs/batch-history.md`，再写入新的当前批次；历史已存在的同一批次必须内容一致，否则停止并要求人工复核。
- 阶段方向变化只更新本文件顶部 Current milestone / Next candidates 与 `docs/autonomous-goal.md` 顶部实施区；不要把逐轮日志、完整验证输出或旧候选继续堆入长期方向区。
- 历史事实不删除；考古时按 Batch ID 搜索 `docs/batch-history.md`，日常接手不要读取归档全文。

## 验证标准

每个 active batch 记录实际执行过的命令及结果；`release-check`/`ciReleaseGate.ready` 只算 inventory readiness，不能替代本地命令执行或远程 job conclusions。优先保持 coherent vertical slice，不用逐字段 metadata batch 维持连续推进。

历史上普通 batch 曾使用两次 push（implementation + release inspection）；自 Batch 792 起，日常节奏改为 Windows 本机完整验证后只做一次 implementation push 并立即继续，远程 workflow 异步、非阻塞。旧批次中的 inspection 记录保留为历史事实，不作为当前 cadence。

## 风险与注意事项

- `docs/batch-plan.md` 是 active/next 的 durable source，不是已完成批次日志。
- `docs/batch-history.md` 是历史归档；不要把它重新并回活动文档，也不要在默认 handoff/read-first 中要求全文读取。
- `CHANGELOG.md` 只记录必要的用户可见变化和边界；逐步实现细节留在 batch history。
- 只有当前用户 goal/session 明确授权时才 commit/push 指定分支。

## 历史批次归档

完整历史已拆到 `docs/batch-history.md`。除非要查旧 batch 细节、验证历史决策或做 release/debug 溯源，不要默认读取历史归档全文。
