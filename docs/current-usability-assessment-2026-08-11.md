# 当前真实可用性评估（2026-08-11）

> 本文件是完成 `daily-product-closure-v1` 四个产品闭环后的评估快照，不是新的路线或 release green 证明。

## 读取指南

- 首选入口：先由 `docs/context-routing.md` 的“当前真实可用性 / 产品化程度复评”场景路由到本文件。
- 只想知道项目是否已经“坏掉”或能否日用：读“实施摘要”“真实结论”和“完成度口径”。
- 准备安排下一阶段：读“还差什么”和“推荐下一阶段”。
- 验证细节只用于复核本次判断；不要把本文件加入默认 read-first，也不要用它替代 active route、release receipt 或当前仓库实跑。

## 实施摘要

项目没有坏掉，也没有沿错误方向失去迭代能力。此前最明显的问题是：底层状态、恢复、Reviewer、授权和证据能力很深，但用户仍要知道入口、第二段调用、lane、已有目录接入规则和 adapter 生命周期。`DPC-01`～`DPC-04` 已把这四个日常断点收成了可实际运行的 Windows + Claude Code + `vmp-re` 产品路径：

1. 自然语言自动进入薄 `/rekit`，状态只给一条人话动作；
2. 一次 goal/correction 操作可完成 member → independent Reviewer → completion，拒绝则停下来等真实纠偏；
3. 普通非空目录先零写入分类和 exact plan 预览，明确确认后才原地接入，并保留原文件；
4. 已有 IDA TSV 索引可在 exact profile + `authorized-gate` 下由固定只读 adapter 查询，结果进入 packet/report/receipt/observation、独立证据复核、member 和 Reviewer lineage。

本次真实验收启动了真实 Claude member/Reviewer 和真实 contained adapter child，不使用手写 LLM 结果或伪造 packet/receipt。最终 receipt 为 `passed=true`，DPC-04、profile revoke、evidence review、terminal replay 和 attached-directory recovery 全部通过，临时 case 自动清理。

## 执行清单

- [x] 核对薄自然语言入口、stable `DailyUserAction` 和多 lane typed choices。
- [x] 验证一次用户操作内的真实 member、独立 Reviewer、completion/correction 和 terminal replay。
- [x] 验证普通非空目录零写入 preview、hash-bound Apply、sentinel 保留和 Windows exact rollback。
- [x] 验证 `vmp-re` IDA index request/profile/gate/dispatch/child/receipt/observation/evidence/member/Reviewer 全链。
- [x] 验证 profile 恢复为默认 `manual-gate`，terminal replay 不重复启动 child 或 Claude。
- [x] 验证 attached case 的 member/Reviewer cutpoint、零 Claude completion recovery 和 mutation-free replay。
- [x] 最终源码真实 acceptance、全仓 tests/vet、公开 inventory 与只读终审已通过；完成态文档字节的 release minimum 在提交前重跑，commit/push 后再核对 `HEAD == origin/main` 与 clean tree。

## 验证标准

本报告分开判断四件事：

1. **底层闭环是否真实**：必须有真实 Claude 进程、真实 contained child、strict receipt 和可重放结果。
2. **窄目标是否日用**：Windows + Claude Code + `vmp-re` 用户不再需要记 lane、SHA、session ID、第二段 Reviewer 调用或普通目录初始化细节。
3. **通用产品是否可发布**：是否有预编译安装、普通用户界面、第二个成熟领域 pack 和当前 release 证据。
4. **安全边界是否保持**：自然语言不能自动变成 profile、authorized gate、authority/confirmed、sync/promote 或未授权 heavy action。

当前 1、2 已达到可试日用；3 仍未完成；4 保持 fail-closed。

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
| Windows + Claude Code + `vmp-re` 日常窄路径 | 85%～90% | 四个主要产品断点已闭合；适合维护者和高级用户开始真实日用 |
| 普通用户交互体验 | 65%～70% | 自然语言入口和人话 action 已有，但仍依赖 Claude Code 环境，没有独立界面或安装体验 |
| 多领域安全产品 | 25%～30% | `vmp-re` 是唯一成熟领域，其余主要是 skeleton |
| 通用可安装发布产品 | 45%～50% | 核心产品路径已形成，但 installer、binary release、正式支持矩阵和当前远程发布证据仍不足 |

因此，不能再用之前“面向普通用户完整产品 35%～40%”描述当前 **Windows + Claude Code + `vmp-re` 窄目标**；该窄目标已经接近九成。若问题是“无需源码、无需 Go、带独立界面、覆盖多个领域的通用成品”，它仍大约只有一半，这不是四闭环失败，而是另一个更大的交付范围。

## 真实结论

- **项目是否歪掉或坏掉？** 没有。底层能力此前过深、入口过薄的问题已经被四个闭环针对性收口，而且真实进程验收通过。
- **现在能不能用？** Windows + 已登录 Claude Code + `vmp-re` 可以开始真实日用，尤其适合已有文本证据、IDA 导出索引和明确目标的 case。
- **是不是普通用户开箱即用产品？** 还不是。仍需要源码、Go 和 Claude Code；没有安装器、独立界面和多领域成熟度。
- **之前 token 是否都浪费了？** 不是。大部分投入形成了 currentness、恢复、Reviewer、授权和证据底座；问题是这些能力长期没有形成日常入口。DPC-01～04 正是在把已存在的底座变成可感知产品。

## 还差什么

1. **安装与交付**：预编译 `rekit` / `rekit-host` / `rekit-adapter-host`、版本化 release、安装/升级/卸载和首次登录检查。
2. **第二个成熟 pack**：优先选择 `web-security` 或 `generic-binary-re`，做真实 session + concrete adapter + Reviewer 的完整产品场景。
3. **更多低风险 concrete adapters**：继续固定用途、显式编译绑定，不引入动态 plugin registry；真实需求出现后再扩。
4. **普通用户第一屏**：可考虑轻量 TUI/GUI，但应消费现有 typed status/action，不能建立第二套状态机。
5. **发布证据**：完成当前本机 release minimum，并在正式发布或周期复审时取得绑定当前 HEAD 的远程三平台结果。
6. **更强进程隔离**：只有产品威胁模型要求“child 即使有缺陷也绝不能联网”时，才立项 AppContainer/WFP 等 OS 级隔离。

## 推荐下一阶段

下一阶段不要再做 summary/visibility 微调，也不要同时扩 8 个 pack。建议先完成当前四闭环整体验收和 commit/push，再从以下两项中选一个中大型产品闭环：

1. **可安装 Windows developer preview**：预编译三个 Go binary、版本/升级边界、首次运行 doctor 和最短自然语言 quickstart；
2. **第二个深 pack vertical slice**：一个真实用户目标、一个固定低风险 adapter、member/Reviewer/证据/replay 全链。

无论选择哪项，都继续复用现有 typed runtime 和 durable truth，不新增产品状态机、动态插件系统或 PowerShell runtime logic。
