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
| U1 | Git 内 case 信任边界、模型档位落实到代码与测试、工具可用性四态报告、错误信息可执行化 | U0 | done |
| U2 | 自动化执行链：可取消定位、输入前复核停止/状态、坐标校验负例、异常退出资源清理 | U0 | done |
| U3 | 协作与上下文精简、证据生成与复核提效 | U0 | done |
| U4 | 逆向/Web 复杂任务能力、自动化定位缓存与性能 | U1–U3 | done |
| U5 | 冻结评测：12 配对＋6 留出＋6 裸臂 | U4 | done |
| U6 | 安装一致性、发布候选就绪（不发布）、文档收束 | U5 | done |

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

- 无 active 批次（U0–U6 全部 done，收束见 U6 结果区）。出现新实质目标时另建路线，不追加本文件。
- 关键指针：评测材料与判分 `%TEMP%\steamai-pd\u-cap\`；游戏 fixture `%TEMP%\steamai-pd\dlv05-game\`；本路线工程改动=提交 `9dfb7998`（U1/U2）、`dda97bed`（U3）、`2afda305`（U4）。
- 已完成：U0（基线+协议冻结）；U1（Git 内 case 信任锚定边界、Sonnet 默认档入码、headless 探针结论）；U2（prepare/输入分离+checkpoint 可取消定位+双复核+暂停冻结推进、仓库 12 用例、坐标校验与纹理模板 6/6、llm 定位六场景 6/6）；U3（SKILL 精简+证据防漂移条款）；U4（LLM 定位蒸馏 28.2s→0.013s、逆向间接调用/无符号与 Web 状态转移/限流方法补缺）。

## 批次结果区

### U6 安装一致性与路线收束（2026-10-25，done）

- **安装一致性**：新 `steamai.exe`（含 U1 信任边界与模型档位改动）构建、uninstall＋setup 更新本机安装并验证（PATH 生效、源码仓库拒绝信息含 `/add-dir` 可执行指引、uninstall helper 按合同自清理）。
- **发布候选就绪（不发布）**：`go vet` 干净、Python pack tooling 39/39、`git diff --check` 干净；Go 全包测试于 U4 提交（`2afda305`）时全绿，其后仅文档改动。**公开发布新 Release 仍需用户单独批准，本路线未执行。**
- **路线收束**：U0–U6 全部 done；长期行为已写回（U1 合同四处同步、U2 pack 四文档、U3 SKILL/evidence、U4 方法五文档、U5 无合同变更）。评测材料、判分表与 runner 保留在 `%TEMP%\steamai-pd\u-cap\`（tasks/、runs/、blind/、scores.csv、SHA256SUMS 含修订追加），本文件只保留验收结论与失败/缺陷记录，执行细节以该目录为准。
- **遗留（如实）**：git 内 case-at-repo-root 可见窗口全链验证可选未做（headless 探针证明信任仅作用可见窗口）；游戏 6 并发资源竞争的具体瓶颈未深挖（以 2 并发规避）；pack 方法库增益的证明条件（更弱模型/更复杂真实任务）留待未来路线，本轮结论为 tie 不改判。

### U5 冻结评测（2026-10-25，done）

- **执行**：24 题构造并冻结（SHA256SUMS 48 文件）；实际运行 42 次（12 配对×2 臂、6 留出×2 臂、6 裸臂——留出组按同协议双臂跑完，未留"仅终验"差异，如实记录）；arm=candidate `2afda305` vs baseline `60576c08` 的 git archive 等价快照；headless claude（Sonnet 档）、匿名判分（18 对映射 anon-manifest.json）、维护方对照 ANSWER-KEY 评分（scores.csv、score_sheet.py 汇总）。
- **正确性门槛：全过**——18 对双臂 36/36 全对（avg 100%）、裸臂 6/6 全对；RE-IC1 出题笔误（T[15] 缺失）被两臂一致抓住，KEY 修订双正确路径。
- **效果门槛：全 tie**——配对 12 题每题两臂同分；文本中位时长 130s vs 131s、费用 $2.51 vs $2.57；游戏双臂均触及 1500s 会话上限而产物同级；无任何维度达到 −25%/−50% 门槛。按冻结协议：**只交付可靠性改善，不宣称效率升级**。
- **核心发现（如实）**：强模型（Sonnet 档）＋自解释环境下，pack 方法库的边际增益在正确性与效率上均不可测量；方法路由存在差异（candidate 直达 U4 专项方法节、baseline 绕通用分析）但未转化为结果差异；裸臂更快（中位 90s vs ~130s）——读方法材料的时间未换来正确性。方法库的价值须由更弱模型/更复杂真实任务/专业工具链场景的后续证据证明，本轮不据此宣称或否定。
- **评测材料缺陷 2 处（均被两臂一致正确处理并如实记录）**：GA-PP1 fixture 弹窗无关闭路径（popup 后永久 DIALOG，任务不可达）——两臂零违规＋如实报告未完成；GA-PP2 move_px=480 使按钮移出截图（bbox 负坐标）——两臂识别 BLOCKED＋对照位移场景验证脚本正确。GA-MS2 同款弹窗缺陷（未跑）。缺陷题按"fail-safe 正确行为"修订 KEY 判分，两臂同分。
- **执行事实**：游戏 6 并发批因资源竞争全灭（5 题零交付，保留 `*.timeout-first-attempt.*` 记录），2 并发重跑后全部交付；游戏 run 普遍"产物在预算内交付、会话收尾超时"（1500s 上限），判分按产物不按会话退出码；GA-MS1 bare 是唯一自然退出的游戏 run（1440s rc=0）。
- 可靠性改善（非本轮 LLM 评测度量，由 U1–U4 工程实测证明）：停止缺口修复（estop 0.113–0.182s、双复核、13 负例）、伪匹配真实拦截（坐标校验+纹理模板 6/6）、定位蒸馏 28.2s→0.013s、Git 内 case 信任边界、Sonnet 默认档入码。

### U4 定位蒸馏与专业方法补缺（2026-10-25，done）

- **LLM 定位蒸馏**（游戏线性能，fixture `airtest_device.py`）：首次 llm 定位成功后把 bbox 裁剪为本次运行内本地模板；后续同目标先本地 aircv 匹配（毫秒级、全图搜索可跟踪窗口位移），失配（外观/尺寸变化）自动回退 llm 并更新模板。只存内存不落盘（进程重启失效，符合重启不带状态语义）。实测：同按钮首次 llm 28.2s → 蒸馏命中 **0.013s**（约 2000×），命中坐标通过目标端 bbox 校验（`runs/distill-smoke`）；llm 六场景 6/6 语义不变（`llm_calls_total=8`）；fixture 单测 23/23（新增蒸馏命中不调 llm、位移跟踪 delta≤2px、失配回退、StopRequested 不留半成品）。方法入 pack `tooling/integrations/airtest.md`（定位蒸馏条：纪律=命中不豁免坐标校验与点击后状态复核）。harness join 超时按定位模式自适应（llm/windowed 120s）修复 sc4 误报。
- **逆向/Web 方法补缺**（对照 U5 题型 grep 审查确认三缺口后补）：新 recipe `indirect-call-resolution.md`（引用点定位、表来源判定、idx 边界封闭性、运行期改写整点 keep unknown，含合成案例）；`static-binary-triage.md` 增无符号/误导符号节（结构证据建临时别名、归因止步结构等价、符号与结构矛盾并列记录）；`request-sequence-review.md` 增状态转移节（前态+操作+后态三段证据、两次 200 不证明转移、只读序列天然对照）；`request-replay.md` 差异归因表增请求头/限流行（计数维度先确定再比较）并强化限流下禁止自动重试。manifest 与两包 README 路由表登记。
- 验证：`go test ./...` 全 ok、`go vet` 干净、Python pack tooling 39/39；`git diff --check` 干净。

### U3 协作与上下文精简、证据提效（2026-10-25，done）

- **SKILL.md 精简**：团队协作章程 11 条→指针＋3 条 Commander 侧规则（细则以 case `CLAUDE.md` 协作章程为唯一 durable source，本文件不重复维护第二份；保留 roster lifecycle/成员单写/`HOLD_STALE_TASK` 零覆盖锚短语）；研究产物节→指针＋2 条合同要点（目录结构以 case `CLAUDE.md` 为准，保留 artifact tuple 绑定与"绑定所审源码版本或 exact diff"）。19700→18599 chars（−5.6%，重复条款去重为指针；合同必需的 3 个断言短语以最短形式保留在 Commander 规则内）。
- **证据防脚本漂移条款**（`common/policies/evidence.md`）：复跑或续跑沿用既有登记脚本；确需修改先更新 artifact 登记再执行，回执绑定执行时实际使用的脚本 SHA——堵住"事后换脚本不重跑"使旧回执失去 exact 绑定的缺口。
- 验证：vnextcontract 全绿（精简首轮曾误删 3 个合同断言短语致 `TestPrototypeSkillDefinesNativeTeamBoundary`/`TestAutonomousCollaborationKeepsMethodsFlexibleAndBoundariesExact` 红，已定位补回）；`go test ./...` 全 ok、`go vet` 干净、Python pack tooling 39/39。
- 如实记录：U3"证据生成与复核提效"实际交付为 evidence 防漂移条款与 SKILL 去重，未含更大范围的模板重构——按"概念数不增长"约束，复核提速主要收益应来自 U4 的方法深化与 U5 评测实测，不在本轮预铺。

### U1 真实项目启动（2026-10-25，done）

- **Git 内 case 信任边界**：`claude_trust.go` 新增 `gitAnchor`，`validateTrustScope(memberRoot, caseRoot)` 允许两种锚——exact 成员目录与 case 根（case 建在仓库根，原生信任范围恰等于 case）；case 位于仓库子目录或成员与 case 根间有嵌套仓库时拒绝并列出可执行下一步（手动信任框/调整 case 位置）。`CLAUDE.md`/`capabilities.md`/`SKILL.md`/`acceptance.md` 四处合同同步；测试改为三布局正负例（`TestValidateTrustScopeAnchorsGitTrustToCaseBoundary`）。
- **Sonnet 默认档落实到代码**：`resolveLiveModel`（evaluation live helper）默认先 Sonnet 后 Haiku；vnextcontract probe `freezeLiveProbeModel` 优先 Sonnet 槽位缺失降 Haiku（原固定 Haiku）；acceptance.md 同步。测试表全部更新（Sonnet 优先/降级/无效不回退）。
- 验证：`go test ./internal/steamai ./evaluation ./vnextcontract ./casebootstrap` 全绿（信任+模型改动后）。
- headless 探针（2026-10-25，$0.06）：未信任 git 目录中 `claude -p` 正常返回（无信任阻塞）——headless 会话不受工作区信任影响，信任仅作用于可见窗口启动；探针因此无法区分信任状态。git 内（case 在仓库根）自动信任的可见窗口全链验证未做（需用户同意时间窗，可选），边界如实保留：代码依据原生锚定语义（case-at-repo-root 时锚=case=授权边界）。

### U2 自动化执行链（2026-10-25，done）

- **workflow.py 执行链重构**：`StopRequested` + `checkpoint()`；observe/prepare 接受 checkpoint，prepare（耗时定位）与 act（短输入）分离；输入发出前与 pending 落盘后双复核控制文件；暂停保持观察但冻结决策推进（不累计 waited/retries、不决策、仅目标消失停），resume 重置上下文。poll_interval 构造校验 (0,0.2]。模板/recipe/integration/README 四处 pack 文档同步。
- **仓库测试**：新增 `tests/pack_tooling/test_windows_flow.py` 13 用例——正常链/停止前后/observe 期间停止/prepare 无回调停止/协作式 prepare 停止/子进程取消收尸/pending 后停止不 commit/暂停丢弃过期准备并重观察/prepare 异常无 pending/输入失败保留 pending/pending 重启暂停/轮询界校验；Python pack tooling 39/39。
- **fixture 坐标校验（修伪匹配验证缺口）**：`fake_game_headless.py` 消息须带 `at` 且落按钮 bbox，误定位记 `mislocated-click` 拒绝；`airtest_device.py` 新接口（prepare/act 分离、`llm_locate` Popen 轮询可取消+checkpoint 杀子进程）、消息携带坐标；fixture 单测 20/20（新增取消收尸用例）。
- **实测**：坐标校验生效后旧纯色模板 aircv 恒报 (104,26)→1/6（真实暴露 DLV-05 伪匹配限制）；渲染按钮加纹理（边框+文字）+按固定 bbox 裁剪模板后 template 模式 **6/6、零误定位**（`report-template-textured-6of6.json`）；estop 0.113-0.127s（等待段）。
- **llm 定位模式六场景**：**6/6**（Sonnet 档，两轮全过，`report-llm-6of6.json`）。首轮 sc4 曾报 "start not fired"，根因是 harness join 超时默认 30s 小于 llm 定位单次 30–60s（重试可达 90s）——误报而非执行链缺陷；harness join 超时已按定位模式自适应（llm/windowed 120s、template 30s），修后 llm 与 template 模式各连续两轮 6/6。

### U0 基线与协议冻结（2026-10-25，done）

- 工作区收尾：前一轮文档清理（删除 `docs/professional-delivery-roadmap.md`、`docs/batch-plan.md`，新增 `docs/professional-delivery-results.md`，12 文件 +135/−378）提交为 `60576c08` 并推送。
- 基线测试快照（HEAD `60576c08`）：`go vet` OK；`go test -count=1 -p=2 ./...` 全 ok（steamai 31.6s、casebootstrap 107.3s、evaluation 4.8s、learningbatch 94.0s、vnextcontract 1.8s）；Python pack tooling 27/27。
- 评测协议与题目规格：本文件"评测协议"节冻结；材料构造与 SHA 冻结在 U5 前完成。
