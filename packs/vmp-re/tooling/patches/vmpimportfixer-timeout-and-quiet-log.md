# VMP import fixer short-test hardening

## 背景

公开 import fixer 在 VMProtect 3.7+、on-the-fly 解密、merged handler、rolling key 场景下经常无稳定命中，但仍可能产生大量日志或候选爆炸。

## 可复用补丁思路

- 增加 timeout。
- 增加候选数量上限。
- 增加 quiet / summary-only log 模式。
- 对 unresolved/missing export/emulation failed 做快速聚合摘要。
- 输出只保留是否命中 protected import、候选数量、失败类型。

## 止损条件

- 无 protected import 明确命中。
- 候选快速膨胀且无可验证 IAT/ImportDir 修复。
- 只产生 unresolved 或 missing export。
- emulation failed 且无法稳定复现。
- 需要人工长时间盯进度。

## 后续动作

止损后转入真实 VMEnter context + Unicorn trace，不继续在 import fixer 路线消耗。
