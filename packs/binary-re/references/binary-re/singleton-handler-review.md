# Singleton handler 复核流程

适用于已授权 case 中 1-count / low-count、alias-heavy handler 的窄问题：这一处调用最终改变了什么，哪些解释已被排除？优先复核已有有界材料，不默认重新 trace，不把 formula fit 当作证据。

## 先限定输入和假设

在现有 evidence/finding 中说明以下内容，不依赖固定轮次文件名或作者口头背景：

- **输入身份**：artifact alias、case-relative 位置、SHA-256/bytes、来源工具与配置；候选对应的对象/构建、handler、occurrence 与指令范围。摘要、CSV、反汇编等导出要能回指同一来源及其覆盖范围。
- **坐标和环境**：区分文件偏移、RVA、运行时地址；说明映射依据、old/new VSP、位宽、字节序、入口/出口定义，以及执行或仿真环境、初始 context。对象、坐标或环境未对齐，不能拼接成同一条因果链。
- **一个待判定解释**：明确 final `newVSP+offset` 的预期 payload、来源、宽度和 VSP delta；同时写出会否定它的观察，例如后续覆盖、地址 alias 或出口尚未到达。bridge 假设也要给出可检查的 control/key/dispatch 效果。
- **适用前提和排除条件**：明确当前材料能看到哪些输入、写入和出口。截断片段、身份不明、VSP 映射缺失或外部调用影响未覆盖时，只回答已覆盖的局部问题，不推断完整 opcode 或整类 handler。

同一来源的不同导出不是独立证据；一致只能说明这些表示相容。独立观察也须对齐对象、坐标和环境，差异不能靠“多数一致”消掉。单次 occurrence 可支持该次 final effect，不自动证明所有输入下的通用公式。

## 从出口反查 final effect

1. 定位这次调用的入口与出口，并核对 old VSP reads、bytecode reads、VSP delta 与 final new VSP。出口缺失时先标明可见边界。
2. 按最终 slot 的地址及字节范围，倒查**最后仍生效的 final VSP payload write**。把各次地址计算映射到同一坐标；不同表达式可能指向同一存储位置（pointer/source alias）。
3. 检查 later overwrite，包括部分宽度、重叠写入和经 alias 的覆盖。early staged write 若被覆盖，只作中间过程；部分覆盖要分别说明存活字节，不能沿用整值公式。
4. 区分 VM stack payload 与 native `rsp` scratch/junk，以及 VIP/key/control 更新。解释最终保留值的来源和读取时点；不能仅因写地址“像栈”就认定 payload。
5. 将 formula fit 与真实读写逐项对应。`LOW_OCCURRENCE`、`ADD_XOR_OR_ALIAS`、`MANY_EXACT_FORMULAS`、`POINTER_ALIAS`、`SOURCE_POINTER_ALIAS` 都是待消歧线索，不是自动接受或自动归类条件。

## 只做能改变结论的最小检查

每个影响结论的不确定点都给出一个最小区分检查，并提前说明两种结果会怎样改变结论；不要为了多一份材料而扩大 trace。

| 不确定点 | 优先检查已有材料 | 结果如何影响判断 |
|---|---|---|
| early 算术是否保留 | 同次调用中目标字节从该写入到出口的后续写入 | 完整覆盖则否定 early final-effect 解释；无覆盖且范围完整才保留；缺尾段则 unknown。 |
| ADD/XOR 等公式同时拟合 | 对照实际指令与操作数数据流；若不足，提出一个使候选结果不同的输入 | 指令链或区分观察排除不符解释；仍同值不算验证，继续限定为来源关系或 unknown。 |
| 两个地址是否 alias | 在同一时点/坐标下核对有效地址、宽度与覆盖范围 | 相同或重叠则重算 final effect；可证不重叠才排除 alias；映射不足则 unknown。 |
| 两份观察相互矛盾 | 先核对来源、构建、occurrence、地址基准与初始 context | 未对齐则分开记录；对齐后仍矛盾则提出窄复核，不择优采用。 |
| 未见 payload 是否是 bridge | 确认观察范围是否覆盖出口及相关内存写，并检查明确的 control/key/dispatch 变化 | 覆盖完整且无 payload、role 效果可追溯才支持局部 bridge；缺观察不等于无 payload。 |

已有材料无法区分时，在 F 中留下缺口和下一检查，通过 native session 定向消息告知 Commander；允许停止并保持 unknown。只有获准的具体动态动作及工具权限满足时，才使用已确认存在的 case 工具取得额外有界观察，并限定目标、输入变化、时间/输出预算、隔离与停止条件。本 pack 不提供 mining/rebuild executor，也不要求调用未交付脚本。

## 结论与复核边界

| 可证明范围 | 合理结论 | 不应声称 |
|---|---|---|
| final slot 的来源、宽度及存活字节可追溯 | 当前 occurrence 的 opcode/source effect 候选；公式未定可只描述 source layout | 从单样本推广整类语义。 |
| 完整相关观察中无 stable VSP payload，且 control/key/dispatch/VMExit 效果明确 | 当前覆盖范围的 bridge/control role 候选 | 仅凭“没有看到写入”证明 bridge，或否定其他环境的 payload。 |
| final write、alias、出口或环境无法对齐 | keep unknown；列出已证明局部效果和最小补证 | 用保守命名掩盖没有证据。 |

原始输出留 case-local artifact；E 记录身份、定位、方法和观察，F 记录解释、反证、限制与下一检查，重要结论交独立 Reviewer 在 R 中复核。证据推翻解释时更新 F 并定向通知，停止或收窄依赖旧解释的工作；实质改派由 Commander 决定。恢复使用 native session 与现有 E/F/R，不新增 handoff 文件。若 case 已有派生语义表，只能依据复核结果按现有单写边界更新；表中标签和公式拟合都不替代 E/F/R，不能由本流程自动合入。

## 合成抽象案例

以下仅是方法示例，不是真实样本、工具输出或审查结果。

- **正向**：抽象 handler H 在完整调用范围内把输入 A 复制到 final slot S；无重叠覆盖，宽度和地址映射明确。可支持“本次 S 最终来自 A”，不据此宣称所有输入都遵循某个公式。
- **反例**：摘要把 early `A+B` 标为 final payload，但后续通过 alias 对 S 全宽覆盖为 C。该反证否定加法 final-effect，应改述本次最终来源 C；同源的另一个 CSV 仍写加法不构成独立支持。
- **unknown**：片段只显示 key 更新，未覆盖出口且一处写地址无法映射到 VSP。不能归类 bridge；保留“可见 key 更新”，提出核对出口附近目标写入的最小补证，缺授权或材料就停止。
