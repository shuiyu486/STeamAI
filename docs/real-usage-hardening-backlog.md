# 真实使用加固未来批次 backlog

## 读取指南

本文件只保存 `real-usage-hardening-v1` 尚未成为当前项的未来批次卡。日常接手不要默认读取；只有当前批次完成、需要核对下一张卡，或代码事实要求调整后续顺序时，才从 `docs/real-usage-hardening-roadmap.md` 路由到对应 RH 编号。

## 实施摘要

未来批次按“降低用户操作 → 补失败与恢复 → 扩大真实能力面 → Windows soak → 条件跨平台”的顺序排列。当前 source、状态、当前卡和解锁决定始终由 `docs/real-usage-hardening-roadmap.md` 持有；本文件不是独立选题源。

## 执行清单

### RH-04：host/process 中断后的真实续跑

**用户断点**：structured output 已可恢复，但 host 在 Claude 运行中退出时，fresh host 对只有 accepted running receipt、没有 exact recovery artifact 的 session 仍只能报错，长任务可能需要人工判断是否重跑。

**范围内**：建立 durable host-run supervision，使进程启动后、输出返回后、result-first 写入后、submission 后和 intake 中断后都能由 fresh host 判断“继续收取、exact 恢复、fence 后 replacement 或已完成”，且不重复启动成功 session。

**范围外**：不声称无法证明的 liveness，不用 PID 单独证明当前性，不无限轮询。

**focused 验收**：各 lifecycle cut point 的 crash/restart 自动测试；至少选择一个 real-Claude cut point 做显式 gate，真实输出不得由测试生成。

**真实证据**：fresh host 从 case-local durable state 恢复；成功 output 只发布一次；旧 generation 迟到结果被拒绝；无证据时明确 replacement 而非猜测成功。

**停止/升级条件**：若跨平台无法安全重连进程，先完成 Windows 文件/receipt 驱动方案；不得为了“恢复”放松 output/session/currentness SHA 绑定。

### RH-05：Reviewer 拒绝后的纠偏闭环

**用户断点**：现有 live gate 只证明 Reviewer 接受；真实 Reviewer 返回 `reject/rejected` 后，用户还没有被真实 gate 证明能看懂原因、纠偏、replacement、重新审核并完成。

**范围内**：Reviewer reject 保持 lane open；Mission Commander 提供证据绑定的 reject 摘要和 correction 入口；新 member 读取原目标、被拒证据和人工纠偏；独立新 Reviewer 重新审核，只有 canonical accept 才可 complete。

**范围外**：不把 reject 自动改写为 accept，不由 host 编造纠偏，不复用拒绝 Reviewer 的 accept receipt。

**focused 验收**：deterministic reject lineage/tamper tests；一个故意缺失明确要求的真实 member 结果触发真实 Reviewer reject，随后真实 replacement 修正并由新 Reviewer accept。

**真实证据**：receipt 记录首次 reject、correction、owner generation 变化、第二次真实 review 和最终 accepted lineage；所有 ReviewerResult 来自真实 Claude。

**停止/升级条件**：如果模型在 bounded 场景未稳定 reject，调整任务的机器可验证要求，不手写 ReviewerResult 或篡改 decision。

### RH-06：一个 harmless read-only adapter 的真实执行闭环

**用户断点**：adapter dispatch/report/validate/observation contract 已很严格，但主要证据是 deterministic fixture/package E2E；尚未证明 host/lane executor 在授权范围内能真实启动一个 adapter、收取报告并回到 mission。

**范围内**：选择一个无网络、无调试、无 patch 的 read-only adapter，对临时无敏感 fixture 执行；完整走 strict autonomy profile + authorized-gate（若 contract 要求）、dispatch-before-execution、真实 adapter report、receipt、read-only validate、observation acknowledgement 和 mission resume。

**范围外**：不执行真实样本、网络、exploit replay 或其它 heavy action；不扩大默认授权。

**focused 验收**：adapter package/gate/CLI；一个真实本机 adapter live gate，断言报告 bytes 确由 adapter 进程生成而非测试手写。

**真实证据**：dispatch 早于执行；report/artifact SHA 与 receipt/observation 同源；越界路径、预算、owner 或 late result fail-closed；临时 case 清理。

**停止/升级条件**：没有适合的 harmless adapter 时，先实现最小 Go-owned read-only adapter host，不借本批引入 heavy tooling。

### RH-07：真实结果到跨 case pack-memory 复用

**用户断点**：pack-memory 已有 promote/reconsume proof chain，但普通使用仍可能停在 CLI 证据流程，且尚未用真实 Claude 产出的可复用、无敏感内容知识证明 producer → review → second case consume。

**范围内**：在隔离临时 kit/case 中，从真实 member 结果提取一项通用、可清理的 recipe/checklist；经过 sanitize、review-first promote 和独立验证，第二 fresh case 通过 selected sync/reconsume 消费并由真实 member 引用。

**范围外**：不向真实仓库 pack 写测试知识，不携带 case 路径、目标、样本、trace、payload 或客户数据。

**focused 验收**：pack-memory/promote/sync/CLI；真实 producer 和 consumer Claude sessions；隔离模板根清理。

**真实证据**：candidate/source/review/decision/reconsume hashes 完整；consumer 明确使用 promoted 内容；零 case-private 泄漏。

**停止/升级条件**：任何 sanitize 或 authority 证据不足即保持 review pending；不自动 promote 到当前仓库。

### RH-08：跨 pack 真实 session 兼容

**用户断点**：真实验收固定默认 `vmp-re`，无法证明 host/task context/reviewer schema 对其它安全 pack 仍成立。

**范围内**：参数化非默认 pack 的 harmless live gate；先选择 `_template`，再选择一个适合无敏感只读任务的安全领域 skeleton pack；验证 onboarding、member、correction、replacement、Reviewer、completion 与 pack-specific expected output。

**范围外**：不宣称 skeleton pack 已成熟，不执行领域 heavy tool，不为每个 pack 复制 host 逻辑。

**focused 验收**：manifest/default-pack invariants、sessionhost/CLI；每个选定 pack 一个显式真实 Claude receipt。

**真实证据**：receipt 记录 exact pack；task context 和 output contract 来自该 pack；默认 `vmp-re` 回归继续通过。

**停止/升级条件**：pack schema 或 prompt 不足时只修共享边界或该 pack 的最小真实断点；不得把 pack-specific 字段硬编码进 host。

### RH-09：Windows 日常试用与稳定性门槛

**用户断点**：单次 gate 通过不等于可连续日常使用，尤其是长路径、清理、Claude 配额波动、重复请求和多个 case 接力。

**范围内**：在 Windows 连续运行至少 3 个 bounded、无敏感真实任务，覆盖 fresh case、existing case、人工纠偏、一次故障恢复和 terminal replay；输出一份仓库外机器 receipt 汇总成功率、人工底层输入数、耗时、replacement 和 cleanup。

**范围外**：不为了提高成功率放宽 strict intake 或预先生成 LLM 结果；不把 receipt 中的绝对临时路径提交进仓库。

**focused 验收**：完整 Windows release minimum；显式真实 soak gate。

**真实证据**：所有 case cleanup；`manualResultWrites=0`；用户仍不填写 ID/时间/路径/SHA；失败必须分类而非从统计中删除。

**停止/升级条件**：若 3 次中任一产品链失败，回到对应已完成批次修复并记录 reopen 理由；RH-09 未通过不得进入跨平台扩展。

### RH-10：跨平台 product path（条件批次）

**用户断点**：Windows 是当前支持门槛，macOS/Linux 的真实 Claude product path 尚无等价证据。

**解锁条件**：RH-01 至 RH-09 全部完成，且进入正式发布、跨平台专项或周期复审窗口；否则保持 `deferred`，不能抢占 Windows 日常可用性。

**范围内**：在可用 runner/host 上执行不含敏感内容的普通 public route gate；修复路径、executable resolution、signals、permissions 和 cleanup 的共享差异。

**范围外**：不把 compile-only 或 workflow inventory 当真实 product-path green，不因无 runner 伪造远程结论。

**真实证据**：各平台实际 job/session conclusion 与 receipt；无法获得 runner 时明确 `blocked`。

**停止/升级条件**：远程成本、登录态或 runner 需外部授权时升级；不能以该阻塞延长本地微批次。

## 验证标准

- 只有 active roadmap 明确解锁的下一批可从本文件提升为当前卡。
- 提升时复制对应卡到 active roadmap，并从本文件移除已激活卡；完成记录最终进入 batch history。
- 所有声称 LLM 成功的证据仍必须来自真实 Claude Code structured-output envelope；deterministic fixture 不得冒充 live provenance。

## 风险与注意事项

- 不把本文件加入默认 `readFirst[]`，也不在新会话接手时预读全文。
- 本文件不拥有 current/state；与 active roadmap 冲突时以 active roadmap 为准并停止自动领取。
- 不因拆文档改变批次顺序、验收或授权边界；拆分只降低上下文负担。
