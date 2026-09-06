# STeamAI research capability v1 路线

## 读取指南

- 路线 ID：`steamai-research-capability-v1`。
- 当前状态：`双领域方法、关键分岔点决策与主辅 pack 已实现；默认回归通过；32 轮真实静态对照完成，四组质量平局，未证明研究增益；原生成员双包读取、会话保存、同 session 恢复与正常退出已验证`。
- 本文只保存本路线目标、范围与证据。完整路由见 `docs/context-routing.md`，短投影见 `docs/batch-plan.md`。
- verified-learning 的既有证据与待验项仍在 `docs/verified-learning-roadmap.md`，不改写为已完成，也不借其 bounded Reviewer pass 证明本路线增益。

## 目标与用户画面

用户继续主要指挥 Commander；成员仍是独立目录中的普通、可见 Claude Code，自行维护当前任务并使用原生 session 上下文与恢复。本路线不补一套记忆、交接或调度系统。

1. Binary RE 和 Web/API 同时各深化一个窄方法：输入与适用前提明确、能处理反例、结论可复查。
2. 关键研究分岔点先选能改变判断的检查，反证出现后实际停止、收窄或改派；不能把报告更长当进展。
3. Fresh 可以固定一个主 pack 和最多一个不同的辅助 pack。按任务读取，仍是一个 case、一份授权、一套团队。
4. 辅助只提供方法；经验只回主 pack。已有 case 不补装、不迁移、不覆盖。

## 批次与边界

| 批次 | 交付 | 机械/内容验收 | 真实效果 |
|---|---|---|---|
| RC-01 | Binary RE 低样本 final-effect/证据复核；Web/API 请求假设、有界验证、差异归因、修复证据 | 内容与路由合同通过 | 双领域静态对照完成；质量平局，未证明增益 |
| RC-02 | 关键分岔点行为；沿用七项任务、E/F/R、成员单写与纠偏优先 | 内容与原生边界合同通过 | 有提示的选择/反证响应对照平局；自发决策增益未证 |
| RC-03 | 主＋可选一个辅助 pack 的 Fresh/current、按需路由、主包单写 learning | production API/CLI 机械测试通过 | 双向 Fresh/current、真实双包消费、可见会话保存与同 session 恢复通过；learning 仍只按机械测试范围 |

### 专业方法

- 先核对对象、输入身份、环境、适用与排除条件，再使用方法。相似关键词不是适用证据。
- Binary RE 区分公式拟合与 final effect、覆盖写与稳定 payload；同源导出不冒充独立证据。未交付脚本不当成可用工具。
- Web/API 区分发送状态、响应差异与安全结论；修复前后条件可比，且正常行为不回归。已有 schema 不等于执行器。
- 合成教学例只说明方法判断，不包含真实样本、trace/dump/capture、凭据、绝对 case 路径或 case 进度，不预填真实验收结果。

### 研究决策

Commander 定义关键问题与投入边界，owner 自主选检查。在重要分岔点说明支持/反驳条件，不为机械小任务增加格式负担。受材料、工具或授权阻塞时如实报告，不能把未验证写成已推翻。最终交付对齐用户问题，而非 finding 数量。

### 主辅 pack

- `Pack/PackTree` 仍指主包；`auxPack` 是可选单值。snapshot 沿用一个 `steamai-case-snapshot-v2` parser，`aux-pack` 与 `aux-pack-tree` 成对可选，不建 legacy reader。
- 一次冻结主辅与完整 common，保留各自目录，common 只复制一次；exact preview、source records、tree、全部 writes 与 digest 绑定主辅选择。
- current 仅检查自身固定文件；无辅助字段就是单包，不要求旧 case 补字段或新模板。新版能读未漂移旧单包，不承诺旧 executable 能读新双包。
- 辅助包不增加权限或成员；主辅方法冲突由 owner/Commander 基于适用条件处理，Go 不判断研究语义。
- learning 使用既有主包双 manifest、destination scope、accepted source chain 和 exact Apply。完整 snapshot digest 含辅助内容；辅助 destination 和跨包 batch 拒绝。主包经验不能暗中依赖未来单包 case 缺少的辅助文件。
- 不扩展 readonly runner，不新建 promotion/capability 系统。辅助输入进入比较时两臂固定一致，实际读取与适用范围由相应实验和 Reviewer 核查，不宣称新增自动上下文资格判定。

## 验证

默认执行相关 production/contract tests，然后：

```text
go test -count=1 -p=2 -timeout=30m ./...
go vet ./...
git diff --check
```

需要验证单/双包 exact Fresh、旧 current 零写、辅助身份/字节/路径漂移拒绝、common 不重复、学习主包成功而辅助/混合目标拒绝。异常测试保持其余 binding 自洽，避免只因外层 hash 错误而误以为覆盖了语义条件。

真实路径按 `vnext/acceptance.md` 的 research capability 章节执行：独立 Fresh 的现行、方法增强、决策增强、最终组合，实际加载对应 pinned 内容；同题/模型/预算，避免原生 session/auto memory 泄漏前组答案。模型选择遵循当前配置并冻结。默认不启动模型、不执行请求、不写回经验、不发布 Release。

## 证据与结论边界

- 2026-09-06 默认全量验证及 live 后最终回归通过：`go test -count=1 -p=2 -timeout=30m ./...`，五个含测试 package 全部通过；`go vet ./...` 与 `git diff --check` 通过。一次性 Go 验收胶水已归档并从产品工作区移除；默认测试进程显式关闭三个 live 开关，未调用模型、HTTP 或正式产品 live 路径。
- focused 验证覆盖双向主辅 Fresh/learning、旧单包固定 bytes 零改写、自洽异常 payload/marker、确认后辅助漂移和辅助/混合 learning 拒绝。审查修复了重复辅助身份导致发布后才失败的问题：Fresh 生成端与 current 读取端复用同一纯 marker 校验，在发布前拒绝；也修正了 Web 方法把预设身份对照误当混杂因素的表述。
- 2026-09-06 已执行一次冻结的双领域静态行为对照：4 道新编合成题 × 4 组（baseline / method-only / decision-only / combined）× 2 轮，共 32 轮真实 Claude Code 调用。16 个独立 case 均通过 production API Fresh→exact Apply→InspectCurrent；各组固定对应 source/skill/templates/pack bytes，第二轮只 resume 本题 session，关闭本轮 auto memory，不改全局配置。所有轮次成功读取指定 member/case/skill/manifest/router/method；题目、协议和 case 全部文件验后未变。
- 本机 Windows 10 x64、Claude Code `2.1.236`；requested model 按配置冻结为 `gpt-6-astra[1m]`，每条主 assistant 消息均为 `gpt-6-astra`。每轮 CLI 预算阈值 `$0.75`、超时 180 秒；32 轮总成本 `$2.214175`，单独身份探针 `$0.00624`。CLI 预算不是服务端硬上限。原生 session 已逐 exact cwd 用 `project purge` 清理，脱敏前的验收材料只保留仓库外。
- 真实 `__open-member` 旅程发现并修复正式会话的继承环境缺陷：从 Commander 工具启动时，`CLAUDE_CODE_CHILD_SESSION=1` 会让 Claude Code 关闭 transcript 保存。现在仅在正式 Commander/member 的新进程环境中移除此标记与原有 `CLAUDECODE`，不强设持久化、不改全局、不改变模型；新增 Fresh/current/双包 Commander 与 member 回归先失败、修复后通过。先前未保存的会话无法追补历史。
- 独立 Reviewer 只读脱身份的题目/答案 packet，逐项进行语义评分；评分后再按 output SHA 解盲归组。四组均为 `16/16`，全部四题平局：本次没有观察到可计分的质量增益或回归，且出现满分上限，不能证明一般任务等效。baseline / method-only / decision-only / combined 的费用分别为 `$0.579618 / $0.545314 / $0.578933 / $0.510310`；单次费用差异不证明稳定效率提升。
- 该对照要求显式给出支持/反驳判据，只能检查有提示下的静态判断、下一步选择与反证响应，不能证明无提示的 Commander 自发决策、多会话协作或实际 HTTP 行为；四题一次观察也不建立统计显著性。没有按验收答案再调整方法或重跑题目。
- 双向主辅组合均通过本机真实文件/Git 的 production Fresh/current。另一个主 Web/辅 Binary 的独立只读 Claude Code 调用实际成功读取自身与 case 规则、两包 manifest/router/方法共 8 个文件，正确区分主辅身份、授权优先级、未知发送不可重试与同源摘要不独立；仅使用 Read，actual model 为 `gpt-6-astra`，费用 `$0.144708`，case 全部文件未改写。这不是可见窗口内消费证据。
- 修复后的 Windows `__open-member` 从 exact member cwd 启动普通可见 Claude Code，保留 native transcript；同一 session 的成功 Read 记录覆盖主 Binary/辅 Web 两份方法，actual model 为 `gpt-6-astra`。用原生 `--resume` 恢复该 session 后，成员仅新增指定结果文件，回复完成标记；通过私有 console 输入一次 `/exit` 正常退出。Fresh 原有文件和 snapshot 全部未变，current 深验通过；会话已用限定私有配置和 exact cwd 的原生 purge 清理。Win32 可见性是系统 API 观察，不是用户实际看见或手工纠偏证明。
- 可见旅程保留了全部失败：成员目录内放测试配置被 current 拒绝、隔离配置首次启动引导超时、继承 child 标记关闭 transcript、测试文件权限路径错误，以及一次 console 尚未就绪导致的恢复前观察失败。只有确认原因并修正的步骤才补验，不删除失败记录，不把它们计为研究对照胜负。可见交互费用没有完整计量；不能将 `$2.365123` 的已知 print 调用费用冒充全部成本。没有重跑上述 32 轮研究对照。
- 静态合同、fixture 或 Go tests 不能证明研究更准、更快；效果不显著、负向、超时与不确定结果都保留，不重跑刷绿。
- 原 verified-learning 尚待 exact calibration attestation、最终 candidate comparison 等门槛仍按其路线处理。新组合不能自动继承旧 pass；真实 V4 仍只能来自后续逐份 opt-in 的实际证据。
