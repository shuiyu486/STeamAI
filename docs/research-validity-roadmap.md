# STeamAI 研究有效性路线

## 读取指南

- 路线 ID：`steamai-research-validity-v1`。
- 当前状态：本轮两项验证均已完整执行。已知报告 Reviewer 校准为 `complete / pass-limited`，格式/类别均10/10；新9题研究对照为 `complete / tie`，新版0胜、9平、0负，无无效题，没有证明相对提升。旧观察器错误及 Binary 作者 API 失败原件保留，用户明确授权的输入准备重试不冒充首次成功。
- 完整路由见 `docs/context-routing.md`，短投影见 `docs/batch-plan.md`。本文只保存本轮窄范围与执行状态，不预填 pass/go。
- `docs/research-execution-roadmap.md` 保留专业取证、预测/联合研究已验事实与原对照 `incomplete/inconclusive`；`docs/verified-learning-roadmap.md` 保留旧校准原件、费用、SHA 与未完成门槛，不续跑或重判旧 suite。

## 批次与范围

| 批次 | 本轮工作 | 状态 |
|---|---|---|
| RV-01 | 在现有 Reviewer 表测试覆盖 reason 字符串合法、数组/缺失/null/空白/额外字段拒绝；prompt 示例走同一 parser | model-free focused tests 通过；不放宽 parser |
| RV-02 | 少量当前配置身份、Read 范围与 Reviewer 协议预检 | 4次调用完成；身份原失败另有离线纠正，双轮隔离/恢复与首次 Reviewer 格式绑定通过，未花满6次额度 |
| RV-03 | 全新9题研究对照：Binary/Web/joint 各正向、反例、unknown | complete / tie：18个production Fresh cases、36次研究调用与9次首次盲审全部完成；新版0胜9平0负，无无效题，完整封存后解盲，未证明提升 |
| RV-04 | 5类×2已知报告 Reviewer 校准（report-copy）：改善、平局、退化、授权退化、呈现更好但证据更弱 | complete / pass-limited：20运输arms、10首审全部完成，格式10/10、类别10/10；生产闭合通过，限本次已知报告，不生成go attestation |

## 执行边界

- 新 suite 执行前冻结题目、评分、pinned 输入、当前配置解析后的模型/CLI、预算与停止规则；身份以真实 assistant 消息核对，不强制模型、不改配置。旧预算与 runtime 记录不当作本轮新冻结。
- 研究对照使用新题，不续跑已解盲的旧9题；完整评分冻结后再解盲，失败、超时、负向、平局、未知及费用原样保留，不补问、不重试刷通过。
- 已知报告校准只检查当前协议下的 Reviewer 判断，复用既有五类 controls、每类两次配对与判据，不新增门槛；报告复制不证明生成式研究增益，不自动产生 calibration go 或 V3/V4 资格。
- 生成式桥接、候选晋级、learning preview、Apply 全部取消于本轮；不改产品 runtime、pack/skill/MCP 配置，不提交推送，不新增研究控制面或通用外部控制器。
- Reviewer 协议回归仅修改 test 文件并复用现有 fake model event；机械测试不调用模型、不读取旧 case。真实执行由各自新冻结入口留证，验收分层仍见 `vnext/acceptance.md`。

## 当前执行记录

- 冻结运行身份为 `gpt-6-astra[1m]`、high、Claude Code 2.1.250（exe SHA `63403e00…`），不改全局配置。首个身份调用7.007秒、`$0.017030`，production `runArm` completed；外部观察器却把所有 `tool_use_result` 当成 Read file 对象，误拒原生 `StructuredOutput` 的字符串结果。原输出和 `passed=false` 保留，另在源码副本离线修正并绑定 correction（SHA `6947f7bc…`）；原模型、schema、预算和答案不变，额外模型调用0。
- 此后仅执行尚未发生的双轮scope和一次Reviewer预检：scope R1/R2 为12.825/8.140秒，分别 `$0.031705`/`$0.006998`，目录外sentinel被拒绝，第二轮只读新增材料并正确回忆前轮随机值；Reviewer 19.939秒、`$0.038146`，三份输入各一次完整Read，单次StructuredOutput，reason字符串与entry/output SHA均正确。共4次调用、已知费用 `$0.093879`，预检结果 SHA `b10ec5d2…`；未用完6次额度，不代表正式校准通过。
- 新输入准备并行执行Binary作者与已知controls作者各一次。Binary作者79.559秒后返回 `API Error: OpenAI messages stream read failed`，生产记录为failed；原始终态虽报告 `$0.009830`，modelUsage却只有辅助模型、缺主执行用量，完整费用未知。controls作者207.086秒完成、`$0.070640`，只形成待独立审查初稿。未生成完整9题，未启动独立输入审查、正式研究arms或正式校准，按冻结规则暂停新增付费，不重试作者或换模型。API流中断的更深原因未经证明。
- 首次暂停时，离线核验3份源码/构建记录共825个文件、当时全部已执行raw/result绑定与CLI身份通过；暂停结果 SHA `1038eca6…`。截至该暂停点共6次CLI调用，已报告合计 `$0.174349`，含失败终态所报金额但不含其未知主执行费用，不是完整账单。当时两项正式有效性验证均未完成；该阶段记录不覆盖后续新授权续行结果。

- 用户随后明确要求“再试试，我需要完成这两项”。仅对未产题的Binary作者建立新授权attempt，成功费用 `$0.132255`；旧失败与费用缺口不删。Web/joint首次作者及四组首次独立内容审查均完成，9次输入准备尝试中8次completed、1次旧API失败；成功输入准备已知费用 `$0.655443`，另有旧失败终态所报 `$0.009830` 及未知主执行费用。全部新输入与审查SHA冻结为 `af735556…`。已知校准通过production Fresh/PrepareSuite冻结10个slots后开始首次执行；预检与旧盲题未重跑。

- 本轮 `RV-KNOWN` 的20个运输arms与10个首次Reviewer全部completed，383.63秒、`$1.112176`；10份内层reason均为字符串，entry/output SHA匹配，五类各两次全部符合预注册判据。production `FinalizeSuite`/`VerifySuite`均通过，case current，suite SHA `8d4f786e…`、结果 SHA `675f6498…`，决定为 `complete / pass-limited`。离线复核原始输出、gold、裁决、映射和构建绑定一致，audit SHA `8c150931…`。RK05理由把task/evidence文件标签说反，但所选entry与类别正确；不能将10/10类别正确夸大为每句理由无误。没有go attestation，不迁移到生成式候选资格。

- 新9题固定比较 `96e33591` 与 `3ba51196`，Binary/Web/joint各正向、反例、unknown；18个production Fresh case与独立session的36次研究调用全部completed，2012.56秒、`$3.579855`。9次首次独立盲审全部有效，649.57秒、`$0.588581`；完整裁决封存后解盲，新版0胜、9平、0负，各领域均3平、无hard failure、无无效题，结果为 `complete / tie`。研究＋盲审总费用 `$4.168436`，结果 SHA `4709ce40…`、audit SHA `aed4f70e…`；没有补问、重跑或解盲后调方法。
- 研究离线核验确认161次完整Read与原件bytes相同，18个case文件集合除预先约定的所选补充卡外未变；盲包完整保留原始两轮回答、实际所见卡与输出SHA。新版Binary三题实际读取bounded-behavior-chain，Web三题读取request-sequence-review，joint三题读取bounded-behavior-chain与client-api-joint-review；不是只复制snapshot或读取README。两版9题均获五项满分，存在评分饱和与题目区分度有限的问题：只能说本组未观察到差异，不能证明两版普遍等价，也不能证明新方法毫无价值或已带来提升。
- 最终本轮共88次CLI调用：4预检、9输入准备尝试（含保留的1次API失败及用户另行授权的重试）、30已知校准、36研究和9首盲审。已报告合计 `$6.039764`，不含原Binary失败中未知的主执行费用，仍非完整账单；名义授权上限保持 `$39`。总账 SHA `92692b9c…`，两项结果与原始审计分别绑定。无生成式go attestation、learning candidate、preview、Apply、提交推送或Release；不继续扩题刷出提升。

## 最短维护验证

```text
go test -count=1 ./internal/steamai/evaluation -run '^TestBoundedReviewer'
go test -count=1 ./internal/steamai/vnextcontract -run '^(TestCanonicalCurrentSurfacesDoNotInvokeRemovedRuntime|TestNativeCapabilityContractKeepsVisibleSessionDefault|TestResearchExecution.*)$'
git diff --check
```

本轮 Reviewer 与 vnextcontract focused tests、`go test -count=1 -p=2 -timeout=30m ./...` 全部package、`go vet ./...` 及 `git diff --check` 均通过。pack/Python未改，不重跑IDA/HTTP或真人路径。上述维护回归不替代 RV-02–04 的真实预检或研究/校准结果。
