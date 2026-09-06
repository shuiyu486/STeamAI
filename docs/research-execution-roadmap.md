# STeamAI 专业实操与联合研究路线

## 读取指南

- 路线 ID：`steamai-research-execution-v1`。
- 当前状态：`A/B/C 实现与默认回归通过；36研究轮完成、盲评因超时不完整，IDA与完整可见成员旅程未验完`。
- 本文只保存本轮范围、状态和证据；完整路由在 `docs/context-routing.md`，短投影在 `docs/batch-plan.md`。
- 旧 `research-capability` 的方法/主辅与32轮平局事实不改写；verified-learning 正式证据与待验项仍归 `docs/verified-learning-roadmap.md`，本轮并行收尾，不预填 go。

## 目标与用户画面

1. A：通过实际工具取得一段能改变判断的新证据，不只说明缺什么。Binary 是已打开的授权 IDA 数据库副本定点导出，Web 是原生 curl 显式单次 loopback GET。
2. B：说明少量处理/状态步骤为什么相连，并以尚未查看的分支检验解释。单点索引不强制增加格式。
3. C：同一 case 联合客户端/本地模块与 API 证据，核对版本、对象、编码、请求、服务结果和客户端解释，不把两个局部 accepted 合并成整体 accepted。
4. 独立成员、原生 session/消息、任务单写和用户纠偏不变；一主最多一辅，learning 仍只回主包。

## 批次与边界

| 批次 | 内容 | 状态 |
|---|---|---|
| RE-01 | 工具与模型前置、三类切片和新保留题冻结 | 9题及当前high配置已冻结，真实身份/Read范围/resume通过；尚无真实 IDA 入口证据 |
| RE-02 | 唯一 IDA 定点 exporter、schema/直接测试与 curl recipe | 实现与27项 Python tests 通过；真实 curl 11场景通过，IDA live 未验 |
| RE-03 | Binary/Web 跨步骤连接与预测验证 | 方法与相关内容合同已实现；研究效果待验 |
| RE-04 | 单源 client/API 联合方法与真实双包 case | 方法已实现；3条真实合成客户端/API 路径通过，原生成员整体审查待验 |
| RE-05 | 默认回归、真实工具/成员旅程、9题独立对照 | 全量 Go/Python/vet 通过；36研究轮完成，Q02盲评超时后停止；可见旅程未完成 |
| VL-CLOSE | 旧证据离线核对、适用校准/最终单主候选、锁文件恢复 | 旧原件/锁恢复通过；新20运输arms完成但验收代码误拒导致无Reviewer，原suite inconclusive；晋级/Apply未完成 |

### 唯一定点脚本

`packs/binary-re/tooling/scripts/export_function_evidence.py` 显式运行于已授权的 IDA Windows x64 稳定数据库副本，仅读取指定函数/最多两个调用点和有界引用；不安装或启动宿主，不分析/执行目标，不写 IDB，不默认调用反编译器。范围、条目、输出及合作式截止必须检查；读取失败或漂移不发布成功结果。宿主后台行为、autosave 与阻塞 API 不由 Python 脚本假装强隔离。

脚本是 pack 工具，不是 steamai façade、adapter host 或通用 runner；原生 Fresh 已可固定 tracked regular 资产，不新增快照格式。`vnext/**` 仍仅 Markdown，catalog 不执行，learningTargets 不含脚本/schema。

### 证据与联合判断

综合 F 直接绑定全部支撑连接的 E，R 审查连接与整体 claim，局部 F 只导航；不假设存在 F→F 的自动传递 currentness。事后解释不等于预测，反复读同源导出不等于独立复现。pack revision/digest 不证明被研究客户端/API 的版本对应。缺连接时保留局部结论并给 joint unknown，不扩权限取得确定答案。

C 首批为字段构造/编码→单次 GET→权限结果→客户端解析，受控 fixture 预置状态。不因多步研究引入 POST/body、批量请求、自动 retry 或多步 schema；客户端执行必须单独在具体动作范围内，IDA 只读许可不授权运行。

## 验证

默认不启动模型、IDA、HTTP 或可见会话：

```text
python -m unittest discover -s tests/pack_tooling -p "test_*.py"
go test -count=1 -p=2 -timeout=30m ./...
go vet ./...
git diff --check
```

真实层分别证明：实际 IDA 定点导出；真实 loopback GET 与业务对照；客户端/API 联合链；原生可见成员、定向补证与整体复审。fake IDA、Go fixture、文本断言、同源重复导出不替代对应实机层。人工纠偏只能由实际用户输入证明，不以自动 console 输入或成员消息冒充。

效果对照使用9道未用于调优的静态题（Binary/Web/joint各正向、反例、unknown），当前基线与最终版共18个独立case/arms，每arm最多两轮，另9次盲评。运行前冻结题目、标准、current配置解析后的模型、预算与真实pinned内容；隔离session/auto memory，不改全局配置。任务不额外提示预测步骤，实际行为与结论分别检查。全unknown不算改善，正确限定缺证不扣成错误；负向、平局、失败和费用全部保留，不重跑刷强。

批准的名义总预算为40美元以内，计划分项38.60美元；CLI budget不是账单硬上限，不完整的interactive费用须明确。身份不符、硬边界失败或达到预算就停止，不切模型、不降标准。静态比较不证明实际工具效率或普遍能力；V3/V4资格仍遵循各自证据。

## 当前证据与未完成条件

- 2026-09-06 实施基线为 `96e33591`，开始时工作区干净。原生工具版本已核对：curl 8.13.0、Go 1.26.3 windows/amd64、Python 3.10.5、Claude Code 2.1.236；随后首个模型身份探针通过，实际 assistant 为 `gpt-6-astra`，2.322秒、CLI报告费用 `$0.006235`，无工具调用。
- PATH 与当前可用工具清单没有提供 IDA 入口，不等于断言未安装。需明确可用的合法 IDA/IDAPython 入口后才能完成相关真实验收；不扫描本机目录、不自动安装或以其它后端替代计完成。
- 新资产的 production Fresh 回归实际暴露并修复空 `head-blob` 解析问题：stage-0 新文件合法没有 HEAD blob，旧解析先 TrimSpace 删除分隔空格，导致 Apply 后 current 深验失败。现在先解析分隔再规范化值，保留合法缺省 HEAD anchor；其它空必需字段与畸形记录仍拒绝。新脚本/schema 主辅两个方向的 committed/staged-new 固定、漂移拒绝与旧 literal current 零改写 focused tests 通过（35.420 秒），没有提前提交 fixture 掩盖新资产路径。
- Windows 真实共享锁的生产文件恢复子路径通过：active/previous executable、published/backup source 四个场景，确为 Win32 handle 限制而非注入错误；解除本次 handle 后旧 bytes 恢复。未调用会读取 HKCU 的完整 ActivateUpdate，因此不代表完整安装、Registry 或 Release 验收；显式 gate 与范围见 `vnext/acceptance.md`。
- B/C 三篇方法与共享路由已写入，8 项直接相关的模板/原生成员/方法合同 focused tests 通过，三篇均小于16 KiB。唯一 IDA exporter 的26项 Python tests 通过（0.044 秒），新增 ResearchExecution 合同通过（0.012 秒）；fake IDA/内容测试不证明实际 IDA 或研究行为。
- 真实原生 curl 11场景与合成客户端3场景共14次 loopback GET 全部符合冻结判据：身份、动态字段、合法路径回归、对象/版本错配、禁止跟随 redirect、已知/未知长度 body 超限、送达后 timeout、输出 collision 均保留。chunked 超限 exit63、部分 body 为262144 bytes；1秒测试 deadline 的 timeout 为1.016秒/exit28，服务端记录确认送达，无重试；collision 保留原件并识别数字后缀，不接受为 exact target 成功。3条客户端运行观察为200/403/409与loaded/denied/version-mismatch；不提供 IDA 静态连接证明。40个引用文件 SHA/bytes 复核一致，receipt SHA `f7d9a54560d37a821a3887e407c833b0e10aca19b58213f6c53e77547d576c28`，本轮服务已停止。此层不证明 LLM 联合研究、独立 Reviewer 或用户纠偏。
- 独立9题及标准在方法实施之外冻结，freeze SHA `dd02e5e8b6ef9b14cbbc4dc11b1631c9050155d4a0f759f70898b4e405ba7d7c`；每 arm R1 从3张索引选1张，R2只见所选卡。评分按实际所见材料，题作者了解批准主题但未看新实现，不属于外部基准；未先试题调难度。首个配置冻结为 `gpt-6-astra[1m]`、CLI2.1.236、effort xhigh，并由真实探针的 assistant 消息核验。网络中断恢复后只读核对发现当前 effort 已为 high，模型环境选择未变；旧身份文件和探针保留，后续调用须按当前配置另行冻结/核验，不混用不同 runtime 的证据。
- 独立审查发现并修复 IDA recipe 加载污染：默认 SourceFileLoader 会在 pinned scripts 目录写入 bytecode，导致 current 路径集合漂移；现直接 `runpy.run_path` 加载 exact `.py`，不更改全局配置或放宽 current。新增测试执行 recipe 的实际加载片段及生产 main，旧加载方式红测、新方式绿测，27项 Python tests 通过（0.070秒）；仅 mock IDA，不代表真实宿主。Fresh 与 learningbatch 两个完整包回归分别通过（106.977秒、111.554秒）。
- 仅含交付源码的隔离副本完成全量回归：272个stage-0文件，明确列入11个新增交付文件，排除临时验收源码及bytecode；main的index未变。27项 Python tests（0.072秒）、`go test -count=1 -p=2 -timeout=30m ./...` 全部包、`go vet ./...` 与暂存差异检查通过，均未启动模型/IDA/HTTP/可见会话。最后的证据状态文档更新不改变方法与生产代码，交付前仍核对差异。
- 当前 high runtime 的独立身份探针通过（13.374秒、$0.006240）；真实 Read 范围/两轮 resume 通过（22.190秒＋11.861秒、$0.052475）：case内读取成功、已存在的外部无害文件明确因 dontAsk 权限拒绝，第二轮同session仅读新材料并回忆前轮随机值。前置专项已知费用共$0.064950；不含开发会话或未知interactive费用。
- 可见联合旅程本次不完整：生产 `__open-member` 成功启动一个可见 Reviewer 窗口，但外部 observer 未识别被控制台折行的主题选择提示，120秒无READY后定点清理；未启动其余两成员，未进入研究session、消息或整体review。现有case仍current，不据此报告产品历史保存回归，不自动重开刷通过。另有测试私有配置的冗余 Write 规则警告，已有正确 Edit 规则覆盖文件写入，不是已确认阻塞原因。人工纠偏与实际联合成员协作仍未验。
- 18个production Fresh对照case的36轮研究全部completed，1164.43秒，CLI报告研究费用$1.966009。两版本同用对应pack README条件路由，没有单给新版预测提示。匿名盲评仅第一题完成（49.78秒、$0.106692），机械裁决为tie；第二题180秒timeout、无stdout/actual-model/费用记录，不能把超时归因为已证实的模型质量问题，其余7题按全局停止未调用。全部领域的效果结论为incomplete/inconclusive，不宣称跃迁、统计显著或全题平局；独立复核确认首题12:12 tie与原材料、观察前预测及范围限定一致，无hard failure；36研究记录与首题Reviewer共97次Read全部与相应文件bytes一致、无offset/limit、完整行数，无截断。Q02只有零字节stdout与deadline错误，原因未知，不能仅据共有的unrecognized_model警告归因为网络/CLI/模型拒绝。
- 已知专项费用小计$3.125946（两runtime探针、scope两轮、36研究轮、第一题盲评、新学习20运输arms）；第二题timeout与可见onboarding费用未知，不能当0或完整账单。没有重跑已执行题、换模型或降标准。全部临时repo验收代码已按exact bytes归档到仓库外并删除，未stage/commit/push/Release或真实learning Apply。
- 成员完整旅程、研究增益和旧学习正式收尾尚未完成。本文随实际结果更新，不把计划写成证据。
