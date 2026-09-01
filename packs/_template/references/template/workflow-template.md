# Template workflow

> 复制到新 pack 后，替换为该领域的轻到重路线、证据要求和验证标准。

## Scope baseline

开始前在 case 规则或成员任务中记录：目标、授权范围、允许/禁止动作、预期产出、停止条件和非目标。

## Light-to-heavy route

```text
passive triage
  -> bounded evidence
  -> finding candidate
  -> independent review
  -> accepted finding
  -> heavy action only when the light path is insufficient
```

升级 heavy action 前必须记录：轻路径为何不足、exact target、隔离与网络策略、运行/磁盘/输出预算、case-relative output、rollback 和 stop conditions，并取得用户对该具体动作的确认。

## Team loop

1. Commander 为问题指定一名 owner；必要时再指定一名 verifier。
2. owner 只提交可追溯 evidence 和 finding，不广播探索过程。
3. Reviewer 只读复核，证据不足时将 `needs-evidence` 返回原 owner。
4. Commander 只交付有 evidence、review 和未决风险说明的结论。
5. 只有 accepted finding/review 才能进入 learning candidate。

## Validation checklist

- 不含真实样本、凭据、客户信息、绝对路径或大工具输出。
- finding 能追溯到 evidence，review 直接引用两者。
- heavy action 的用户确认、工具权限、预算和止损均与 exact scope 一致。
- case-local 进度不回流 pack；learning exact patch 在用户确认前不写 canonical pack。
