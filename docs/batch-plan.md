# Batch implementation plan

## 读取指南

本文件只是当前路线的短投影，不保存完整实施日志。完整路由见 `docs/context-routing.md`；目标、状态与接续点以 `docs/professional-delivery-roadmap.md` 为准。旧研究对照与专业接入记录留在各自路线，不凭旧完成标题继承本轮验收。

## Current projection

| 字段 | 当前值 |
|---|---|
| 路线 | `steamai-professional-delivery-v1` |
| source | `docs/professional-delivery-roadmap.md` |
| 目标 | 逆向完整功能分析、Web 实际验证/修复回归、Windows 游戏自动化开发；专业项目能力成为新 Fresh 可消费的交付路径 |
| 当前状态 | 路线收束：DLV-00–08 done（08 四轮 Reviewer 至 accepted）；NE-1 与 G-JOINT 原生消息两项必需项缺口已于 2026-10-25 授权补验闭合；真实鼠标 windowed 复测 6/6 通过（LLM 视觉定位）；遗留 headless 纯色模板零方差伪匹配限制 |
| 下一步 | 无待执行批次；Git 已授权提交推送；维护回到根 CLAUDE.md 与 context-routing 常规入口 |
| 批次 | 00 基线 → 01 合同 → 02 接入 → 03 逆向 / 04 Web / 05 游戏 → 06 联合 → 07 验收 → 08 收尾 |
| 实施细则 | 批实施临时细则已随收口删除；长期行为在 README、根 CLAUDE.md、skill、vnext 模板/acceptance 与各 pack 入口 |
| 当前边界 | 不自动安装或改全局配置，不新增 Go 控制面；计划不等于 live/外部动作/提交发布的授权；范围变化按具体请求处理 |
| 基线缺口 | 旧 GUI 探针不足以证明实际后端分析保留；旧 PD-04C 保留有限答案盲评 none，不承继正式比较结论；详见新路线 |
| 不变 | 原生成员/消息/恢复、主＋最多一辅、成员单写、旧 current 零改写、learning exact confirmation |
| 不再重复讨论 | 游戏自动化已是用户明确目标；按低成本档位和宽松批次额度执行，费用简报，不逐分反复审计 |
| 未声称 | 工程可用等于研究增益（A/B tie 探索观察级）、可见成员窗口路径已验、旧合成/静态证据覆盖真实工具与可见协作 |

## 验证标准

本次仅验证计划路径、引用与现有合同回归。实施按主路线分批验收，默认测试不调用模型或专业工具；结束整轮前再跑相关完整测试。临时细则已在长期行为与结果迁移、引用修复后删除，未删除任何未完成项。
