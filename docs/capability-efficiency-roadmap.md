# STeamAI 实用能力与效率升级路线

## 读取指南与当前状态

- 路线 ID：`steamai-capability-efficiency-v1`；制定于 2026-10-25，基线 HEAD `60576c08`（前一轮文档收束提交）。
- 本文件是本轮批次状态、接续点与结果的唯一入口。用户已全权授权本方案范围内的仓库改动、后台验证、合成任务评测与 Git 提交推送；**公开发布新 Release 与新增第三方安装仍需单独批准**。验证纪律沿用根 `CLAUDE.md`/`vnext/acceptance.md`：测试默认 Sonnet 档（Haiku 仅极简探针或 headless 且 Sonnet 档无命令工具）、默认无窗口后台通道、可见窗口/真实鼠标仅用户同意时间窗。
- 批次全部 done 后本文件按既定规则收束：长期行为写回合同/pack，保留验收结论与失败记录，删除执行计划。

## 目标（冻结）

常见项目（含普通目录与 Git 工程）顺畅启动；成员自主解决更复杂问题；交付证据默认可靠、复核更快；游戏自动化可安全停止且常规动作不慢到不可用。相比基线：用户介入更少、完成更快、质量不降。**不做**：Hub/数据库/守护进程/新控制面、降低证据与授权门槛、留下第二套长期执行计划。

## 批次

| 批次 | 内容 | 依赖 | 状态 |
|---|---|---|---|
| U0 | 工作区收尾提交、基线测试快照、评测协议与题目规格冻结 | 无 | done |
| U1 | Git 内 case 信任边界、模型档位落实到代码与测试、工具可用性四态报告、错误信息可执行化 | U0 | active |
| U2 | 自动化执行链：可取消定位、输入前复核停止/状态、坐标校验负例、异常退出资源清理 | U0 | pending |
| U3 | 协作与上下文精简、证据生成与复核提效 | U0 | pending |
| U4 | 逆向/Web 复杂任务能力、自动化定位缓存与性能 | U1–U3 | pending |
| U5 | 冻结评测：12 配对＋6 留出＋6 裸臂 | U4 | pending |
| U6 | 安装一致性、发布候选就绪（不发布）、文档收束 | U5 | pending |

## 评测协议（U0 冻结）

- **配对**：基线版（HEAD `60576c08` 物化的 Fresh case）vs 候选版（U4 完成后物化的 Fresh case），同题各一次 headless 运行，同模型档（默认 Sonnet 档；该档 headless 无命令工具时降 Haiku 并如实记录为观察条件）、同预算与 turns 规则、无热纠偏。
- **任务**：12 配对（3 线×4）＋6 留出（3 线×2，仅终验）＋6 裸臂（从配对题选，3 线×2，裸 Claude Code＋同批工具说明）。合计约 42 次正式运行＋必要复核。
- **题目规格**（材料在 U5 前按本规格构造并冻结 SHA 于 `%TEMP%\steamai-pd\u-cap\`，含 ANSWER-KEY；全部合成、无真实目标）：
  - 逆向 8：跨函数数据流 ≥2、间接调用/函数表 ≥2、版本差异 ≥2、无符号/误导符号 ≥2；每题含 1 个“不可比较/应保持 unknown”设计点。
  - Web 8：越权/对象归属 ≥2、状态转移 ≥2、限流/请求头处理 ≥2、修复与合法回归 ≥2；每题含合法对照与至少 1 个“仅静态线索不能定论”陷阱。
  - 游戏 8：弹窗/位移扰动 ≥2、非幂等＋中断恢复 ≥2、多阶段流程 ≥2、停止语义 ≥2；headless fixture 必须校验点击坐标。
- **评分维度**：结论正确性（对照 ANSWER-KEY）、证据可复核性、覆盖、无效步骤、用户介入、turns、时长、费用。
- **效果门槛**（正确性门槛全过为前提）：配对任务中位完成时间 −25%；不必要用户介入基线>0 时 −50%（原为零保持零）；日常必读重复内容 −30%（核对实际读取）；同档同范围费用不增（为质量增加须单独说明）；质量与失败保留不降。正确性过而效果 tie：只交付可靠性改善，不宣称效率升级。
- **纪律**：独立目标副本与 case 目录；session/auto memory 不跨组；网关 5xx 等基础设施失败可同额度重跑并保留原记录；不挑成功回合；tie/更差为一等结果。

## 接续点

- 当前批次：U2 尾段（llm 定位模式 headless 六场景后台验证中）＋U1 收尾提交；随后 U3。
- 已完成：U0（基线+协议冻结）；U1 代码与合同（Git 内 case 信任锚定边界、Sonnet 默认档落实 vnextcontract/evaluation helper 与四份合同文档）；U2 主体（prepare/输入分离+checkpoint 可取消定位+输入前/pending 后双复核+暂停保持观察冻结推进、仓库 12 用例、坐标校验入 fixture、带纹理模板修复纯色伪匹配并 6/6）。
- 实测事实（U2）：坐标校验生效前 template 模式 aircv 恒报 (104,26) 全部误定位（1/6）——证实并拦截了 DLV-05 记录的纯色模板伪匹配；渲染加纹理+按 bbox 裁剪模板后 template 模式 6/6、零误定位。历史 headless 6/6 的 template 定位从未正确（消息通道不校验坐标所致），旧记录限制已闭合并升级为真实拦截。
- 已知未解（更新）：~~同步定位停止缺口~~（U2 修复：prepare checkpoint+双复核+仓库负例）；~~headless 模板伪匹配~~（U2 修复：纹理模板+坐标校验）；Git 内 case 信任（U1 修复，待 headless 真实探针补验）；llm 定位模式六场景（进行中）。

## 批次结果区

### U1 真实项目启动（2026-10-25，代码完成；live 探针待补）

- **Git 内 case 信任边界**：`claude_trust.go` 新增 `gitAnchor`，`validateTrustScope(memberRoot, caseRoot)` 允许两种锚——exact 成员目录与 case 根（case 建在仓库根，原生信任范围恰等于 case）；case 位于仓库子目录或成员与 case 根间有嵌套仓库时拒绝并列出可执行下一步（手动信任框/调整 case 位置）。`CLAUDE.md`/`capabilities.md`/`SKILL.md`/`acceptance.md` 四处合同同步；测试改为三布局正负例（`TestValidateTrustScopeAnchorsGitTrustToCaseBoundary`）。
- **Sonnet 默认档落实到代码**：`resolveLiveModel`（evaluation live helper）默认先 Sonnet 后 Haiku；vnextcontract probe `freezeLiveProbeModel` 优先 Sonnet 槽位缺失降 Haiku（原固定 Haiku）；acceptance.md 同步。测试表全部更新（Sonnet 优先/降级/无效不回退）。
- 验证：`go test ./internal/steamai ./evaluation ./vnextcontract ./casebootstrap` 全绿（信任+模型改动后）。
- headless 探针（2026-10-25，$0.06）：未信任 git 目录中 `claude -p` 正常返回（无信任阻塞）——headless 会话不受工作区信任影响，信任仅作用于可见窗口启动；探针因此无法区分信任状态。git 内（case 在仓库根）自动信任的可见窗口全链验证未做（需用户同意时间窗，可选），边界如实保留：代码依据原生锚定语义（case-at-repo-root 时锚=case=授权边界）。

### U2 自动化执行链（2026-10-25，主体完成；llm 模式验证中）

- **workflow.py 执行链重构**：`StopRequested` + `checkpoint()`；observe/prepare 接受 checkpoint，prepare（耗时定位）与 act（短输入）分离；输入发出前与 pending 落盘后双复核控制文件；暂停保持观察但冻结决策推进（不累计 waited/retries、不决策、仅目标消失停），resume 重置上下文。poll_interval 构造校验 (0,0.2]。模板/recipe/integration/README 四处 pack 文档同步。
- **仓库测试**：新增 `tests/pack_tooling/test_windows_flow.py` 13 用例——正常链/停止前后/observe 期间停止/prepare 无回调停止/协作式 prepare 停止/子进程取消收尸/pending 后停止不 commit/暂停丢弃过期准备并重观察/prepare 异常无 pending/输入失败保留 pending/pending 重启暂停/轮询界校验；Python pack tooling 39/39。
- **fixture 坐标校验（修伪匹配验证缺口）**：`fake_game_headless.py` 消息须带 `at` 且落按钮 bbox，误定位记 `mislocated-click` 拒绝；`airtest_device.py` 新接口（prepare/act 分离、`llm_locate` Popen 轮询可取消+checkpoint 杀子进程）、消息携带坐标；fixture 单测 20/20（新增取消收尸用例）。
- **实测**：坐标校验生效后旧纯色模板 aircv 恒报 (104,26)→1/6（真实暴露 DLV-05 伪匹配限制）；渲染按钮加纹理（边框+文字）+按固定 bbox 裁剪模板后 template 模式 **6/6、零误定位**（`report-template-textured-6of6.json`）；estop 0.113-0.127s（等待段）。
- 进行中：llm 定位模式六场景（真实模型调用，后台）。

### U0 基线与协议冻结（2026-10-25，done）

- 工作区收尾：前一轮文档清理（删除 `docs/professional-delivery-roadmap.md`、`docs/batch-plan.md`，新增 `docs/professional-delivery-results.md`，12 文件 +135/−378）提交为 `60576c08` 并推送。
- 基线测试快照（HEAD `60576c08`）：`go vet` OK；`go test -count=1 -p=2 ./...` 全 ok（steamai 31.6s、casebootstrap 107.3s、evaluation 4.8s、learningbatch 94.0s、vnextcontract 1.8s）；Python pack tooling 27/27。
- 评测协议与题目规格：本文件"评测协议"节冻结；材料构造与 SHA 冻结在 U5 前完成。
