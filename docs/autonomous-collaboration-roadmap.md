# STeamAI 自主协作优化路线

## 读取指南

- 路线 ID：`steamai-autonomous-collaboration-v1`。
- 长期基准见根 `CLAUDE.md` 的“智能成员与自主协作基准”；本文只记录本轮范围与验收状态，完整路由见 `docs/context-routing.md`。
- 当前状态：首批合同调整、静态回归、production Fresh 分发与一次真实成员行为对照已完成；合同/测试/文档三项独立审查未发现高信心问题。真实对照结果为 `inconclusive`，未证明自主协作减少请示、更有效或更省，不宣称能力已提升。
- 上轮有效性事实保留在 `docs/research-validity-roadmap.md`：已知报告校准 `complete/pass-limited`，新9题 `complete/tie`、0胜9平0负；不重跑旧题或改判为本轮收益。更早工具/联合证据按 router 定点读取，不默认串读。

## 用户画面与范围

用户说明要解决的问题、允许范围和投入边界；Commander 按结果分工，成员自主选工具、换思路和请求有界帮助。简单问题不强行多人参与，普通探索不逐步请示；重要结论、实质冲突、最终交付和 learning 继续按现有合同审查。

首批只调整实际会加载的 canonical skill、case 共享规则及 analysis/Reviewer 角色，并补对应合同与 production Fresh 分发测试。共同原则在 case 层供成员读取，角色只说明自身职责，不另加任务字段、方法卡、消息格式或交接文件。既有按需路由、研究分岔点和证据要求继续复用，不能把这些已有能力换名宣称新增。

## 批次

| 批次 | 交付 | 状态 |
|---|---|---|
| AC-01 | 根基准、结果导向任务、按需方法/协作与关键审查的合同调整 | 已完成；不增加任务字段或运行状态 |
| AC-02 | 静态合同回归与真实仓库模板经 production Fresh 物化、旧 current 不漂移 | 已通过；仅证明合同与机械分发 |
| AC-03 | 观察实际受影响的成员路径，按需验证相对收益 | 已执行；`inconclusive`，旧合同 3/3 完成且结果正确，新合同 1/3 正确、1/3 自报与落盘不一致、1/3 超时；未证明收益 |

## 不变的硬边界

- 自主选择只在当前任务、case 授权、工具权限与明确方法前置条件内发生；方法调整不等于正式改派。新增动作、范围或状态变化仍按已有要求确认。
- 不重复索要的是已满足且边界未变的同一确认，不是授权一次后无限执行。Fresh/learning exact confirmation、禁止自动 retry、资源/停止条件、成员单写、用户纠偏优先与 evidence currentness 不放宽。
- 不改 Go production、初始化 schema、launcher、evaluation、学习资格或工具配置，不新增研究执行器、控制面或持久状态。
- 只影响后续 Fresh 固定的新 skill/contracts；既有 current case 不更新、不迁移。辅助包只供方法，learning 仍只回主包。

## 验证与完成口径

默认合同测试检查规则与硬边界仍存在；production Fresh 测试读取真实仓库的 tracked working-tree bytes，验证 case、skill 和两类角色确实物化，并确认 source 后续变化只进入另一个 Fresh，不漂移已有 current。两者不证明 LLM 已按新规则执行。

```text
go test -count=1 -v ./internal/steamai/vnextcontract -run '^(TestAutonomousCollaboration.*|TestResearchCapabilityKeepsNativeMembersAndScopedMethods|TestResearchTemplatesPreserveEvidenceAndLearningBoundary)$'
go test -count=1 -v ./internal/steamai/casebootstrap -run '^TestRepositoryRoleContractsMaterializeThroughFresh$'
git diff --check
```

本轮真实模板 Fresh 单测通过（8.012 秒）；`go test -count=1 -p=2 -timeout=30m ./...` 全部 package、`go vet ./...` 与 `git diff --check` 通过。角色/权限、测试质量、文档边界三项独立审查未发现高信心问题。pack/Python 与工具实现未改，不重跑 IDA/HTTP 或 Python 工具验收。

AC-03 使用两个仓库外 synthetic Fresh case，各执行三个相同的 Read/Write 任务；旧合同 3/3 实际结果正确，新合同仅 parity 1/3 正确，negative 出现模型结构化自报“奇数”但实际 `result.txt` 为“偶数”，bounded-choice 在10分钟驱动总时限内超时且无结果文件。旧版三次调用报告费用合计 `$0.270410`，新版已完成两次合计 `$0.195343`，超时调用的费用/进程终态不完整；另有一次最初 schema 参数错误未启动模型，费用为0。所有输出、stderr、结果文件和异常状态保留在系统临时目录，不写入仓库。该结果只支持 `inconclusive`，不能归因于自主规则本身，也不证明真实产品收益；没有用户可见窗口、跨会话消息、真实目标、IDA、HTTP、客户端、learning Apply、提交推送或 Release 验收。

真实路径按 `vnext/acceptance.md` 的自主协作小节执行，不把完整评测仪式塞进日常 case。重点观察问题是否解决、必要确认是否守住、用户无谓干预和无效协作是否减少；规则已读却未遵守时保留为执行失败，不自动追加同义指令。没有相对比较就不声称更准、更快或更省，完成验证也不等于产品能力跃迁，不自动获得 V2/V3/V4 或 learning Apply 资格。
