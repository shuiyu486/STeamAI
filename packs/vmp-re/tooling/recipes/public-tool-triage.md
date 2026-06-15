# Public tool triage

## 目标

快速判断公开 unpack/import/devirtualizer 工具是否值得继续投入；无明确命中时尽快转 trace-based pipeline。

## 候选工具

- VMP-Imports-Deobfuscator
- VMPImportFixer
- vmpfix
- NoVmp
- VMPStatic
- minivmpfix

## 短测规则

1. 先静态识别 PE sections、entropy、imports、VMEnter candidates。
2. 每个工具都设置 timeout、候选上限、日志静音。
3. 只看是否出现明确 protected import 命中、可解释的 VMEnter 模板匹配或稳定输出。
4. 若出现 unresolved/missing export/emulation failed、候选爆炸、模板断言失败，立即止损。
5. 不把完整 build log、工具 README、超大输出贴进上下文；记录摘要、版本、参数和失败特征。

## 经验结论

VMProtect 3.7+ x64、on-the-fly 解密、merged handler、rolling key 场景下，旧 import fixer 和静态 devirtualizer 往往只能作为短测工具，不应作为主线。

## 转入主线的条件

公开工具无明确命中后，转入：

```text
真实 VMEnter context -> Unicorn trace -> focused trace -> value-flow -> conservative lowering
```
