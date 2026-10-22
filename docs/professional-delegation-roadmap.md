# STeamAI 专业负责人接入路线

## 读取指南

- 路线 ID：`steamai-professional-delegation-v1`。
- 长期基准见根 `CLAUDE.md` 的"智能成员与自主协作基准"；本文只记录本轮范围与验收状态，完整路由见 `docs/context-routing.md`。
- 承接已完成的低干预启动路线事实（Fresh 机器确认、成员工作区自动信任、七字段纠偏、有限 A/B tie），不改判任何历史对照或 `inconclusive` 结论。

## 用户画面与范围

用户只向 Commander 说明研究或交付目标；Commander 在派单时即把专业问题交给配置了固定专业能力的普通 `execution` 负责人（不新增成员等级、任务字段或 Go 调度状态）：

- **专业工具负责人**：成员直接使用固定版本专业工具（当前为 REA 与既有 IDA MCP 的按路线选择；后续 Burp/Playwright/Airtest 同模式），从任务开始按所选入口执行，不先运行较弱的默认流程。
- **外部 agent 团队**（Strix 等）：由一名对接负责人经其官方接口委托单一有界任务；内部 agent 不是 durable member 也不是 tactical subagent；未通过资格验收不自主运行。
- 成果进入现有 artifact→evidence→finding→review 链；外部工具的 `pass`/完整性校验不自动等于 accepted；用户纠偏、learning exact confirmation、heavy action 边界全部不变。

## 批次

| 批次 | 交付 | 状态 |
|---|---|---|
| PD-01 | 新路线、合同与路由收口：本文、SKILL.md 路由条款、analysis-member 角色条款、`vnext/capabilities.md` 外部工具接入节、`vnext/acceptance.md` 专业负责人 gate、context-routing/batch-plan/README/根 `CLAUDE.md` 指针 | 完成（机械合同；未安装任何外部工具） |
| PD-02 | binary-re 固定 REA 上游材料与接入说明：`upstream/rea/**`（6.7.0 / commit `27dc42d1`，拼接 SHA-256 机械校验）、`tooling/integrations/rea.md`、manifest 声明、不进入 learningTargets | 完成（材料固定与合同测试；未注册 MCP、未调用模型/工具） |
| PD-03 | 真实可见单成员逆向任务端到端：成员 local-scope MCP 可见与正确性、A→B→A 切换、正例命中、导出交接、GUI IDB 保留、不可用路径报告缺口 | 未执行：需用户批准安装/局部注册后按 acceptance PD-03 gate 验收 |
| PD-04 | 有限对照：A=直接 Claude Code+REA vs B=STeamAI 负责人+同一 REA；同模型/材料/预算/未见题 | 未执行；tie/更差为一等结果 |
| PD-05 | Strix `v1.7.0` 有界外部执行资格核查：范围、可确认停止（含内部 agent）、含子代理费用、纠偏生效、产物取回、实际模型六项 | 未执行；已知 `--max-budget` 覆盖子代理但为响应后检查，停止/时限/并发/热纠偏接口未核实；通过前不自主运行 |
| PD-06 | 首个外部团队任务交付（一名负责人、单一有界任务、可停止、成果可复核） | 未执行 |
| 后续 | 游戏脚本负责人单平台试点（Airtest/Playwright，不自建运行器） | 未排期；纯开发任务是否属于安全研究 case 需先收口范围 |

## 预算与授权边界

- 本轮机械交付（PD-01/PD-02）不安装外部工具、不注册 MCP、不调用模型；live gate（PD-03 起）逐项取得用户批准，预算在执行前单独给出，旧实验预算不沿用。
- REA/Strix 等第三方项目自带的使用约束与许可（含"仅授权目标"要求）始终适用；它们的 agent 不因由 Claude Code 启动而自动继承 Claude Code 逐工具权限。
- 不为接入新增通用 adapter host、后台 supervisor、任务数据库或 `steamai.exe` 配置管理。

## 不变的硬边界

- Fresh 授权、learning exact confirmation、成员单写、`HOLD_STALE_TASK`、用户纠偏优先、evidence currentness 全部不变。
- `upstream/**` 固定材料不进入 learningTargets；经验只回主包既有合格 Markdown。
- 已有 current case 不升级、不迁移；新合同只进入新 Fresh。
- 已完成路线的历史结论不改判：低干预 A/B tie、自主协作 inconclusive、研究有效性 pass-limited/tie 等保留原文。

## 已知事实与不声称

- REA 能力与限制来自对 `rea-agents-6.7.0`（commit `27dc42d1`）的源码/文档审计，未在本机运行；IDA attached 仅 legacy 工具目录、headless 不继承已有 IDB、IDA 结果为 live 等限制已写入接入说明。
- Strix 控制面结论来自 `v1.7.0` 官方文档，未实测。
- 不声称：接入即研究增益、外部团队可自主运行、上游材料等同本机已安装版本、PD-02 机械验收等同 PD-03 live 验收。

## 验证与完成口径

机械部分（本轮）：

```text
go test -count=1 ./internal/steamai/vnextcontract
go test -count=1 -p=2 -timeout=30m ./...
go vet ./...
git diff --check
```

live 部分按 `vnext/acceptance.md` 专业负责人接入节逐 gate 执行并在此表更新状态；每项有结果或有证据的阻塞，缺证不降标、不重试刷通过。
