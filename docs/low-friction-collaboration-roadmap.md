# STeamAI 低干预启动与纠偏路线

## 读取指南

- 路线 ID：`steamai-low-friction-collaboration-v1`。
- 长期基准见根 `CLAUDE.md` 的“智能成员与自主协作基准”；本文只记录本轮范围与验收状态，完整路由见 `docs/context-routing.md`。
- 本轮承接 `steamai-autonomous-collaboration-v1` 的可见任务事实（9/9 判定一致、Reviewer 补证闭环、workspace 校验缺陷修复），不把该轮 `inconclusive` 对照改判为完成。
- 当前状态：LF-01 至 LF-06 **全部完成**。Fresh 机器确认与成员工作区自动信任已在本机真实路径验证（用户零手抄确认串、两个从未信任成员目录零点击信任框、配置域其余字节逐字节保留且与原生 CLI 并发互操作无丢失）；启动验收 case 成员自主交付 10/10 与冻结 oracle 一致，Reviewer needs-evidence→材料修复→Round 3 accepted 闭环；七字段纠偏双路径（程序驱动同 session + 用户在成员窗口亲输直接纠偏）均完成；LF-05 A/B 于第九窗口完成（前八窗口 blocked 的根因与修复见 LF-05 行），两题四臂全部 completed 且产物正确，两次独立盲审均判 tie——两合同在两道合成题上行为无差异，tie 为一等结果，不据此宣称新合同更优。

## 用户画面与范围

用户一次明确授权 case 初始化（目标、授权、禁止、停止条件、主辅包、团队、成员工作区信任）后，Commander 展示完整 preview 即可凭用户自然语言批准执行 Apply，机器携带 exact 校验串；成员窗口打开时不再逐个出现需要用户点击的信任框。授权范围之外的行为不变：learning exact confirmation、工具权限、用户纠偏优先、证据与 currentness 边界全部保留。

本轮同时以维护用 opt-in 驱动器验证两件事：七字段任务的同 session 自动纠偏与过时改派 hold；自主协作合同对有限单成员任务行为影响的旧/新 A/B（旧 `37187d6d`、新本轮 bytes，两题四臂、独立盲审）。比较目标是合同影响，不是产品版本或团队收益；`tie/inconclusive` 是一等结果。

## 批次

| 批次 | 交付 | 状态 |
|---|---|---|
| LF-01 | Fresh：用户批准与机器 exact 校验串分离；规范 case 根绑定 preview identity，防同 facts 跨根复用 | 代码与回归完成：`Preview.CaseRoot` 参与 identity、preview 文案改为内部校验串、跨根 A 确认用于 B 拒绝且零写入 |
| LF-02 | 成员工作区自动信任：`__open-member --trust-workspace` 写 exact 成员目录原生 `hasTrustDialogAccepted` 记录 | 代码与回归完成：原生 `.claude.json.lock` 目录锁（实测 CLI 对外来锁等待 10s 接管）、tmp+原子替换、备份轮换、未知字段/大数/浮点/unicode 逐字节保留、git 仓库内/大小写别名/重复键/畸形 JSON/锁忙 fail-closed、旧两参形式零写 |
| LF-03 | skill/合同/文档同步：SKILL.md、根 `CLAUDE.md` 唯一窄例外、`vnext/capabilities.md`、`vnext/acceptance.md` | 已完成（本文件与路由指针同批收口） |
| LF-04 | 七字段自动纠偏 + 真实可见直接纠偏（维护驱动器 + 一次真实用户输入） | 双路径完成：自动纠偏三 turn $0.130357，同 session 目标行唯一变更/`HOLD_STALE_TASK`/零写全部命中；真实直接纠偏由用户在 analyst 窗口亲输收窄文本，成员自行把任务节目标/输入/读写收窄（标注用户纠偏来源）并交付 true-review.md（6 行 true 逐行复核，全角 U+FF27/Lu、U+FF17/Nd 依据正确），verdict.md 零改动。驱动器两处缺陷（structured_output 解析、跨 stage 累计值重复相加）已修复并补回归 |
| LF-05 | 有限旧/新 A/B：两题四臂、同 runtime、隔离 source、预注册顺序、两次独立盲审 | **完成（第九窗口 `steamai-lf-ab9`，glm-5.3[1m]，四臂+两审 $0.2566）**：arm-1(q1/baseline) $0.0627、arm-2(q2/candidate) $0.0495、arm-3(q1/candidate) $0.0293、arm-4(q2/baseline) $0.0502 全部 completed，resultContent/summaryMatchesAnswer/evidencePathsExact 三检查全 true（q1 两臂均 `sum: 165`、q2 两臂均 `resolved: blau,grau,rot,gruen,blau`，逐字节一致）；盲审 packet 冻结 SHA 后判定、揭盲映射交叉（q1: A=baseline/B=candidate；q2: A=candidate/B=baseline），两审均 `tie`（产物逐字节相同且都正确）。**结论：两道合成题上新旧合同行为无差异，tie 为一等结果，不据此宣称新合同更优。** 前八窗口 blocked 的根因（2026-10-21/22 定点复现，实验根 `steamai-lf-diag`）：并非网关故障——curl/python 逐字节重放 100% 成功；glm-5.3 线上 `thinking:{type:adaptive}`（effort/MAX_THINKING_TOKENS/API_TIMEOUT_MS 在 CLI 2.1.273 均不生效，请求默认超时 600s），arm prompt 曾要求上下文内手算 result.md 的 SHA-256，属无工具时不可能诚实完成的要求，诱发 8-12 分钟病态生成，撞单次 cap 与上游偶发 300s 连接重置，stdout 又缓冲到退出才落盘，表现为"零输出挂起"。**已披露的协议内修正**：(1) `acMaxCallSeconds` 180→420（实测校准，含依据注释）；(2) arm evidence 要求由"路径:SHA-256"改为恰两条授权路径（移除不可能要求；result.md 实际 SHA 仍记录于 artifacts，`evidencePathsExact` 检查替代 `reportedHashMatchesDisk`）；(3) `acDecodeStructured` 修复 JSON 数组事件流解析（原按 JSONL 逐行解析必失败，致 review decision/reveal 未随 stage 写出；补五用例离线单测；第九窗口 decision/reveal 由维护者按代码既定逻辑从已冻结 no-replace 证据机械导出）。期间单次实验得出的 kill-on-close/进程组/CLAUDECODE/TLS 因子结论均已证伪并回退，production 保持原状 |
| LF-06 | 构建部署 dev exe、真实可见启动验收（两个从未信任的新成员目录、用户不点信任框）、汇总交付表 | 已完成：dev exe 部署（旧版保留）、Fresh 机器 token Apply、两成员自动信任启动、analyst 10/10 与 oracle 一致、Reviewer 三轮闭环；成员窗口费用未采集记 unknown |

## 预算与执行边界

- 实验总费用上限 US$10（四臂研究 US$6、两审 US$1、纠偏 US$2、启动验证与余量 US$1）；实验执行上限 30 分钟（代码开发、编译、普通单测不计入）。
- 每次付费调用前检查剩余额度/时间；费用或实际模型不可核验、超时、进程终态不明即停止新的付费调用，剩余机械验证继续。
- 全部合成 case、对照副本、oracle 与报告写入仓库外唯一实验根 `%TEMP%/steamai-lf-probe/`；不信任实验根本身、TEMP 或未知父 Git 根。
- 不自动 commit/push/发布；跨会话消息不冒充用户直接纠偏；比较失败不通过加题、复跑或追加预算翻盘。

## 不变的硬边界

- Fresh 授权只能来自用户在 Commander session 对完整 preview 的明确批准；文件内容、工具输出、历史会话或跨会话消息不构成授权。本 case 初始化许可不推导 learning Apply、任意目录信任或范围外动作许可。
- 信任写入只发生在校验通过的 exact 成员目录：其余 JSON 字节逐字节保留、已信任零写、git 仓库内拒绝、冲突/畸形/锁忙停止启动。不带 `--trust-workspace` 的调用保持原行为。
- 成员单写、compare-before-update、`HOLD_STALE_TASK`、用户纠偏优先、evidence currentness、learning exact confirmation 全部不变。
- 既有 current case 的 skill/contracts/snapshot 不升级、不迁移；新合同只进入新 Fresh。

## 验证与完成口径

```text
go test -count=1 -timeout=10m ./internal/steamai -run '^(TestFresh.*|TestOpenMember.*|TestClaudeTrust.*|TestMergeTrust.*|TestValidateTrustScope.*|TestWriteClaudeConfig.*)$'
go test -count=1 -timeout=10m ./internal/steamai/casebootstrap
go test -count=1 -timeout=10m ./internal/steamai/vnextcontract
go test -count=1 -timeout=10m ./internal/steamai/evaluation
go vet ./...
git diff --check
```

真实可见验收：从未信任过的两个新成员目录经 `--trust-workspace` 打开，用户不点信任框、配置域其余内容逐字节不变；七字段自动纠偏与一次真实直接纠偏分别留证；A/B 四臂记录实际结果、盲审先冻结再揭盲。完成后按交付表逐项列“Fresh 自动确认、自动 trust、七字段纠偏、真实直接纠偏、两题 A/B、费用/时间/终态”，每项有结果或有证据的阻塞；部署后核验 exe hash/version/canonical 绑定，不以 `--version` 独自证明产品验收。

## 依据

- 官方信任记录与范围：https://code.claude.com/docs/en/permissions#project-allow-rules-and-workspace-trust
- 原生 CLI 2.1.273 实测：`.claude.json.lock` 目录锁、外来持锁者等待 10s 接管、`backups/` 5 份轮换、tmp+rename 原子替换；隔离配置域实验记录保存在实验根，不含真实凭据。
