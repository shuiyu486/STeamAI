# 当前真实可用性评估（2026-08-11）

> 本文件是完成 `daily-product-closure-v1` 与 `windows-mission-control-usability-v1` 后的 Windows-only 历史评估快照，不是新的路线或远程 release green 证明。后续 STeamAI 自包含路线的 copied-directory / project-local bundle、恢复、自治和 Windows local minimum 已完成；`web-security` 也已完成本文当时列为候选的第二条 mature production vertical slice。因此本文保留的旧评分、“唯一成熟领域”与“第二个成熟 pack 待做”只代表 2026-08-11 时点，不是当前事实。当前完成态与真实边界以 `docs/real-usage-hardening-roadmap.md`、`docs/steamai-self-contained-project.md`、`docs/release-readiness.md` 和 fresh `release-check` 为准。

## 读取指南

- 首选入口：先由 `docs/context-routing.md` 的“当前真实可用性 / 产品化程度复评”场景路由到本文件。
- 只想知道项目是否已经“坏掉”或能否日用：读“实施摘要”“真实结论”和“完成度口径”。
- 只有用户明确批准新的产品化专项后，才读“未来产品化候选”和“历史候选路线”；它们不是当前待办。
- 验证细节只用于复核本次判断；不要把本文件加入默认 read-first，也不要用它替代 active route、release receipt 或当前仓库实跑。
- 当前“能不能正常使用”的默认口径只看 Windows 本机源码路径。远程 CI、Linux/macOS、三平台兼容和预编译安装包是以后单独做的事；除非用户明确启动正式发布、跨平台或安装交付专项，否则不纳入当前评分，也不作为阻塞。

## 实施摘要

项目没有坏掉，也没有沿错误方向失去迭代能力。此前最明显的问题是：底层状态、恢复、Reviewer、授权和证据能力很深，但用户仍要知道入口、第二段调用、lane、已有目录接入规则和 adapter 生命周期。`DPC-01`～`DPC-04` 已把这四个日常断点收成了可实际运行的 Windows + Claude Code + `vmp-re` 产品路径：

1. 自然语言自动进入薄 `/rekit`，状态只给一条人话动作；
2. 一次 goal/correction 操作可完成 member → independent Reviewer → completion，拒绝则停下来等真实纠偏；
3. 普通非空目录先零写入分类和 exact plan 预览，明确确认后才原地接入，并保留原文件；
4. 已有 IDA TSV 索引可在 exact profile + `authorized-gate` 下由固定只读 adapter 查询，结果进入 packet/report/receipt/observation、独立证据复核、member 和 Reviewer lineage。

本次真实验收启动了真实 Claude member/Reviewer 和真实 contained adapter child，不使用手写 LLM 结果或伪造 packet/receipt。最终 receipt 为 `passed=true`，DPC-04、profile revoke、evidence review、terminal replay 和 attached-directory recovery 全部通过，临时 case 自动清理。后续 Windows 好用化路线又统一了 canonical text、自然语言终态纠偏、typed invocation 和执行时授权复核，并拆分四个维护热点；最新源码组合验收中 member/Reviewer 各 3 次完成，VMP adapter/child、replacement review、attached recovery 与 identity-bound cleanup 全部通过。

## 执行清单

- [x] 核对薄自然语言入口、stable `DailyUserAction` 和多 lane typed choices。
- [x] 验证一次用户操作内的真实 member、独立 Reviewer、completion/correction 和 terminal replay。
- [x] 验证普通非空目录零写入 preview、hash-bound Apply、sentinel 保留和 Windows exact rollback。
- [x] 验证 `vmp-re` IDA index request/profile/gate/dispatch/child/receipt/observation/evidence/member/Reviewer 全链。
- [x] 验证 profile 恢复为默认 `manual-gate`，terminal replay 不重复启动 child 或 Claude。
- [x] 验证 attached case 的 member/Reviewer cutpoint、零 Claude completion recovery 和 mutation-free replay。
- [x] 直接审查 session host 与 handoff 调用链并补回归：host restart 后 replacement budget 不归零，supervision fence 不绕过 durable generation 上限，structured `outcome=failed` 不计 completion，project/lane handoff 只发布各自 exact executable route。
- [x] 最终源码真实 acceptance、全仓 tests/vet/module verify、公开 inventory、release gate 与只读终审已通过；完成态文档字节在提交前完成最终 diff 与 gate 复核。

## 验证标准

当前默认只判断三件事：

1. **底层流程是否真的跑通**：必须有真实 Claude 进程、真实只读工具、完整记录和可恢复结果。
2. **Windows 日常使用是否顺畅**：Windows + Claude Code + `vmp-re` 用户不再需要记 lane、SHA、session ID、第二段 Reviewer 调用或普通目录初始化细节。
3. **安全边界是否保持**：自然语言不能自动变成 profile、authorized gate、authority/confirmed、sync/promote 或未授权 heavy action。

这三项决定当前能不能用。预编译安装、Linux/macOS、三平台兼容、远程发布结果、独立界面和更多成熟 pack 都属于以后单独批准的产品化专项，不计入当前完成度，也不用于给 Windows 结果扣分。当前底层流程和 Windows 窄路径已达到可试日用，安全边界保持 fail-closed。

## 风险与注意事项

- `NoNetwork=true` 的当前含义仍是固定 Go child 没有网络代码路径，不是 AppContainer/WFP 级 OS socket 隔离。
- IDA 闭环只读取已导出的固定 TSV，不启动 IDA、不打开 IDB、不联网，也不做 rename/comment/patch/debug/dump。
- `release-check ready=true` 只说明 inventory 定义可用；最终 release minimum、远程 CI 和正式发布证据仍需分别判断。
- 其它 8 个领域 pack 仍主要是 skeleton；不能用一个成熟 `vmp-re` 路径代表多领域产品已经成熟。
- 当前仍要求源码仓库、Go、已安装并登录的 Claude Code；没有 installer、预编译发行物或独立 GUI/TUI/Web UI。

## 本次真实证据

DPC-04 最终显式 live acceptance 观察到：

- `passed=true`；
- 3 次真实 member launch/completion；
- 3 次真实 independent Reviewer launch/completion；
- 第一代 member 因缺少 IDA evidence 被真实 Reviewer 拒绝；
- correction 后 generation 2 member 使用 exact 13-field `vmp-ida-index-evidence` binding；
- fixed-purpose adapter 只执行 `inspect`，`debug/dump/full-trace/inject/network/patch/symex` 保持 denied；
- independent Mission Commander evidence review 为 `accepted`，随后以 `tool-review` 写入 acknowledgement；
- member binding 与 Reviewer lineage 均 verified；
- profile 已 revoke 回默认 manual；
- terminal replay 无 child、无 Claude；
- attached ordinary-directory case 的 member cutpoint、Reviewer cutpoint、zero-Claude completion recovery、terminal replay 和原文件 hash 均通过；
- fresh 与 attached 临时 case 都被清理；
- 无 authority/confirmed 写入，无手工 LLM result write。

## 四个原始用户断点现在怎样

| 原断点 | 现在 | 仍保留的边界 |
|---|---|---|
| 不知道该输入 `/rekit` 还是底层命令 | 自然语言可自动进入薄 skill；status 是只读人话动作 | 查询不会顺便写入或启动 Claude |
| member 完成后还要记得再调用 Reviewer | goal/correction owner 会在 fresh typed route 下继续 Reviewer 和 completion | Reviewer reject 时停止，不自动编造纠偏 |
| 普通已有目录会被生硬拒绝 | 先只读分类并展示 `initialize-in-place` / `cancel`；确认后 hash-bound 接入 | collision、partial state、reparse、drift 全部 fail-closed |
| `vmp-re` 只有 IDA recipe，没有真实工具闭环 | 固定 TSV literal inspector 已进入 authorized execution 与证据链 | 不是通用 plugin，也不控制 IDA 或执行 catalog entry |

## 完成度口径

这些数字不是任务数量平均值，而是按实际使用目标估算：

| 口径 | 当前判断 | 说明 |
|---|---:|---|
| 底层可靠性、记录与恢复 | 90% | durable state、currentness、Reviewer、授权、receipt、replay 和 Windows 恢复已很深 |
| Windows + Claude Code + `vmp-re` 日常窄路径 | 90% | 主要产品断点及后续文本、纠偏、typed action、授权 currentness 与恢复回归已闭合；适合维护者和高级用户真实日用 |
| 普通用户交互体验 | 70%～75% | 自然语言入口、单一人话 action 和终态纠偏已形成，但仍依赖 Claude Code 环境，没有独立界面或安装体验 |
| 多领域安全产品 | 25%～30% | `vmp-re` 是唯一成熟领域，其余主要是 skeleton |
| 通用可安装发布产品 | 45%～50% | 核心产品路径已形成，但 installer、binary release、正式支持矩阵和当前远程发布证据仍不足 |

因此，当前默认结论只使用 **Windows + Claude Code + `vmp-re` 日常窄路径 90%**。表中其它数字只是未来不同产品范围的历史参考，不能混进当前评价或把 Windows 结果说成“不合格”；只有用户明确批准对应专项时才重新评估。

## 真实结论

- **项目是否歪掉或坏掉？** 没有。底层能力此前过深、入口过薄的问题已经被四个闭环针对性收口，而且真实进程验收通过。
- **现在能不能用？** Windows + 已登录 Claude Code + `vmp-re` 可以开始真实日用，尤其适合已有文本证据、IDA 导出索引和明确目标的 case。
- **是不是普通用户开箱即用产品？** 还不是。仍需要源码、Go 和 Claude Code；没有安装器、独立界面和多领域成熟度。
- **之前 token 是否都浪费了？** 不是。大部分投入形成了 currentness、恢复、Reviewer、授权和证据底座；问题是这些能力长期没有形成日常入口。DPC-01～04 正是在把已存在的底座变成可感知产品。

## 未来产品化候选（当前不启动、不评分）

以下是历史评估记录的未来范围，不是当前缺陷、当前阻塞或已批准路线；除非用户明确选择对应专项，否则不要据此安排下一批或降低 Windows 可用性结论。

1. **安装与交付**：预编译 `rekit` / `rekit-host` / `rekit-adapter-host`、版本化 release、安装/升级/卸载和首次登录检查。
2. **第二个成熟 pack**：优先选择 `web-security` 或 `generic-binary-re`，做真实 session + concrete adapter + Reviewer 的完整产品场景。
3. **更多低风险 concrete adapters**：继续固定用途、显式编译绑定，不引入动态 plugin registry；真实需求出现后再扩。
4. **普通用户第一屏**：可考虑轻量 TUI/GUI，但应消费现有 typed status/action，不能建立第二套状态机。
5. **发布证据**：完成当前本机 release minimum，并在正式发布或周期复审时取得绑定当前 HEAD 的远程三平台结果。
6. **更强进程隔离**：只有产品威胁模型要求“child 即使有缺陷也绝不能联网”时，才立项 AppContainer/WFP 等 OS 级隔离。

## 历史候选路线（当前未批准）

以下两项仅保留 2026-08-11 评估时的历史候选，不构成当前路线。当前仍以 `docs/real-usage-hardening-roadmap.md` 为唯一 active source；没有用户新批准时不得自行启动安装、跨平台或其它产品化专项。

下一阶段不要再做 summary/visibility 微调，也不要同时扩 8 个 pack。若用户以后明确批准产品化专项，可从以下两项中选一个中大型产品闭环：

1. **可安装 Windows developer preview**：预编译三个 Go binary、版本/升级边界、首次运行 doctor 和最短自然语言 quickstart；
2. **第二个深 pack vertical slice**：一个真实用户目标、一个固定低风险 adapter、member/Reviewer/证据/replay 全链。

无论选择哪项，都继续复用现有 typed runtime 和 durable truth，不新增产品状态机、动态插件系统或 PowerShell runtime logic。
