# Batch implementation plan

## 读取指南

本文件只是当前路线的短投影，不保存完整实施日志。完整路由见 `docs/context-routing.md`；当前状态以 `docs/professional-delegation-roadmap.md` 为准。旧研究对照、低干预启动、自主协作合同轮与 verified-learning 的各自证据保留在原路线，不因切换路线改判完成。

## Current projection

| 字段 | 当前值 |
|---|---|
| 路线 | `steamai-professional-delegation-v1` |
| source | `docs/professional-delegation-roadmap.md` |
| 当前批次 | PD-01~PD-06：专业负责人接入（REA 固定材料/合同 → 真实可见任务 → 有限对照 → Strix 资格核查 → 首个外部团队任务） |
| 当前状态 | PD-01~06 全部有结论：03 部分（GUI IDB 未验且无已打开 IDB，需用户环境）、04 正式匹配盲测 tie（2026-10-24，探索记录保留）、05 通过（限定条件：计价网关核验、停止后新开工）、06 已交付并经三轮独立复审 accepted。详见当前路线 |
| 当前边界 | 不自动安装/升级/全局注册外部工具；`upstream/**` 不进 learningTargets；外部 agent 团队通过六项资格验收前不自主运行；不改 Go production |
| 既有事实 | 低干预 A/B 第九窗口两审均 tie；自主协作对照 inconclusive；研究有效性 pass-limited/tie——均不据此宣称增益 |
| 不变 | 原生成员/消息/恢复、主＋最多一辅、经验只回主包、旧 current 零改写、learning exact confirmation |
| 未声称 | 接入即研究增益、上游材料等同本机已装版本、机械验收等同 live 验收 |

## 验证标准

- 对应内容合同与 production Fresh 物化回归，详见当前路线。
- `go test -count=1 -p=2 -timeout=30m ./...`
- `go vet ./...`
- `git diff --check`
- 默认不调用模型、IDA、HTTP、REA 或 Strix；真实行为须按 `vnext/acceptance.md` 专业负责人接入节另行授权，保留失败和未知。
