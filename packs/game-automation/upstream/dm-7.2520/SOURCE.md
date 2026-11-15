# 大漠资料来源

## 固定输入

- 来源：用户提供的 7.2520 SDK 随附 `大漠插件接口说明.chm`；只读取资料，未执行 SDK。版本标签来自用户提供的 SDK 归档，不伪称 CHM 具有独立厂商版本签名；实际运行仍需 `Ver` 核对。
- CHM bytes：`1897774`；SHA-256：`8c041a1a03e300427c335e7b5dfc0458a44cb4283aaf95ec9c21ce73b2d27427`。
- 原文著作权归大漠插件文档作者；本目录仅保留本项目接入所需节选，不附 SDK、许可证/注册码、原 CHM、真实游戏脚本、字库或本机绝对路径，不声明获得 SDK 再分发权。
- 旧 `dm-api` 插件固定来源 `8954106a69c0bd5cf17cfd72409982226e721186` 只作历史查询对照；本目录正文直接来自上述 CHM，**不是从其空模板补猜**。离线查询不依赖安装插件或访问 GitHub。

## 转换与选择

用 Windows HTML Help 的 `-decompile` 在临时目录提取，不打开帮助页、不执行其中脚本/ActiveX。以 GB18030 严格解码 HTML（原 meta 为 GB2312），读取 body 可见文本，去掉 style/script，按段落/换行分隔并归一化空白；没有使用 LLM 补写原文参数。

常用接口保留功能、原型、参数、返回值和注意事项，去掉旧宿主的可执行示例；文字示例中的目标地名替换成 `TARGET_TEXT_A/B/C`。每页列出归一化后的原文行号。绑定、许可和错误页为明确标注的节选：归档本身未收编驱动、隐藏或改变安全配置/网络身份的页面；缺项不表示不存在，未收录接口由所在 case 按授权补读原始 CHM 后正常使用。

`threading.md` 保留高级语言 STA/MTA 和每线程初始化要求，`EnableShareDict.md` 保留共享字库的主对象与并发限制。`README.md`、`cpp-mfc.md`、`color-format.md` 是维护者编写的路由/解释，非官方原文；颜色指南的原始来源是下表 Ocr/OcrEx 页。目录不进入 learningTargets。

## 节选校验

下表的原 HTML SHA-256 用于重新提取时追溯；运行合同测试不要求本机 CHM。节选 bytes/SHA-256 按提交中的 UTF-8 LF 文件校验，每页不超过 16 KiB；更改正文必须重新核对原文并更新本表。它证明固定材料没有漂移，不证明 SDK 或游戏实测通过。

| 文件 | CHM 内路径 | 原 HTML SHA-256 | 节选 bytes | 节选 SHA-256 |
|---|---|---|---|---|
| `BindWindow.md` | `后台设置/BindWindow.htm` | `aac1da8f949e5e9851d5b1fd5ba8807999aa66aeedb372d152e17601a16a2fc5` | 3284 | `cc959e5f0fc2e3c82306a46c2a81ecb10dc6c73abc6a541d7cd630696007b036` |
| `BindWindowEx.md` | `后台设置/BindWindowEx.htm` | `bc705860d8a6b004fc13a3ebcb4e6d7ad834fdb8b4ff7a7fac056434aade6bb8` | 3565 | `2661af51f44eb90a64fb383892c5c9ae92e33c646eccd399f7fea1db14d76d72` |
| `Capture.md` | `图色/Capture.htm` | `7f726d8493e9d770a4e9e4bce1ccfef2ca907309ad39360860b8708b67626ca0` | 772 | `a0e8b6ce6c0014976e0f7e5bb2d1d0ef9dd90f46c9de2e137d1dd5e27e5ecb3c` |
| `ClientToScreen.md` | `窗口/ClientToScreen.htm` | `eac3e911482754ec4f887d9025a412f2b27d7adb069764dec6a514f2002e7047` | 566 | `00607586045107e9f9cab6390dddedac29d5bff59225acc54e8c5f71cd0b365f` |
| `CmpColor.md` | `图色/CmpColor.htm` | `3f7fac0111d53f2839f8c32463a0a8832fce045cc0fca2dc2e3391383a56d0f3` | 785 | `e3b487ac27ea252ba4048c1bd38b08a9e880d3d15f78f7d2c4cb69085677bd73` |
| `EnableShareDict.md` | `文字识别/EnableShareDict.htm` | `ede77cf029d2b53d3616efa05bd0d353cd0dd3c69e1de35fbac4cfd8e7d2c407` | 1426 | `78cb7317dc3fa054ab0825a80d97fc0c27030bde5f125b427bba685be272c380` |
| `EnumWindow.md` | `窗口/EnumWindow.htm` | `26504c7159fbec7eca26b34cd7394850e9585e8560c1e5e13f2679235f7b5788` | 1255 | `3d62a0f830d770283173325459428c7f8339b5ba5c0925b3f53b68b1ba9ed60b` |
| `FindColor.md` | `图色/FindColor.htm` | `8f0232e31b82ea86f48fe5a62a795e6e4a75304fd4fd79eb0bb0ef496752f443` | 1544 | `929a87230714f6dde87abfaa62b05a26cbbbeb3f90b784d406fd8eaf4d8895ac` |
| `FindPic.md` | `图色/FindPic.htm` | `f383aa8218b908ca1596e1902f088a9332e6a2a7b90bb5b797ac4ab3d29cc1f4` | 1492 | `27327fe9f52492be1efa69a52c3eadc35fd19ad4aab4a57a4790840edee8b784` |
| `FindStr.md` | `文字识别/FindStr.htm` | `db1c6f4c02b55480d16d9c88800a1dbcfc77af0c2227270af9827d9297e571b8` | 1687 | `f388ac7329c6d80c62333899556fc638717805b90a0eac697b7cdc0402f4df21` |
| `FindStrEx.md` | `文字识别/FindStrEx.htm` | `3f6000eff90d1f79aefe2f2ce3438f866f7a05457ba577428a8823bdfecfd8ed` | 1721 | `3eabcf5b7f2406cd9b64930cd1d697f9bf3e797f278eedf29876b8752b006590` |
| `FindStrFast.md` | `文字识别/FindStrFast.htm` | `fe114c1ad9be90be49a7b882f6caeaaf35a858109ddb7e83a6a351a35b08fe18` | 1791 | `ecfaeceb64c231e8ac2090354cf67e924748a15885a8c3368545d88c81a30001` |
| `FindWindow.md` | `窗口/FindWindow.htm` | `c7518c0703c45009908c64ddb25f4a9a0b7190bb13c78d70a97653af0bb23a7a` | 688 | `5253ea4f8d5eda001eebb212454a8193a2013b6a3e6000b0374f0799c04d6bd6` |
| `GetBasePath.md` | `基本设置/GetBasePath.htm` | `797129ca97cb415f14c15fbe251242264ba79ccb839d07f94cea9eea4b1fdd5a` | 456 | `4633a2b8c089d7a861bda62d78a2fbdf9e97b4d487087339b17ec8ff5f94bd9a` |
| `GetBindWindow.md` | `后台设置/GetBindWindow.htm` | `5a7913d689df974ee1c7c2f4d777d5ac640741de7aa90578c2906c6a21c5f268` | 465 | `76b3ea14c4d60566c420baf26a283b1f5ac3cd1e7c09125161eceeb2547e14f1` |
| `GetClientSize.md` | `窗口/GetClientSize.htm` | `4aa2db237e571b921585fdc2095e17684be5e3ac245de83818fae4007a0f6595` | 573 | `7d19835d8ab7b1120b18398368db2c8226b7d54a3eb79e38fcf5e3a60894f0bd` |
| `GetDmCount.md` | `基本设置/GetDmCount.htm` | `433a38243ca55b3466604804304d40bf06e62a4ff976c3cf8e1d376f1b8dd58a` | 439 | `b06225b13aab3a4966d36c21f61a284c49582ec2e405a374d5b9ce2b89bb6f2b` |
| `GetID.md` | `基本设置/GetID.htm` | `36be9b6af03cba8852ddc18b66705be6d3f7a7b729793ae452789609f53a1bee` | 522 | `6740e5c5c7c53869d50293d59c4e45d55e9e47b6a96348792799e42d2532c82e` |
| `GetLastError.md` | `基本设置/GetLastError.htm` | `e63a78339771a6830a19c873be3050739003592284b8266f1297a833d01be4eb` | 1540 | `ac104ca1544d3bb580b92ec8774f29a16c89b33bc4f7ae49a44e25998b59b8d8` |
| `GetNowDict.md` | `文字识别/GetNowDict.htm` | `e2a9f09dd67a22032de2167a0a4e2dd9db8d764df8ab6144025439c791d22c7d` | 441 | `3e08b7f5c70e37e2b49c8904cd54576c448d674ae237341f3c66f30f5fb13c7e` |
| `GetPath.md` | `基本设置/GetPath.htm` | `d8360c7c87a106fa0d2e0dba9ce7501dc68b7224227bab9803e2d0b24ef5dc26` | 467 | `b054ab29b5fd73c54f92bbd968e3288a8ce9807a3cd060590bd8fc9cc794eb54` |
| `GetWindowClass.md` | `窗口/GetWindowClass.htm` | `e4a629648abfb98b990c29111384e69665c4ab12043bd64fac3d79e5bec1effd` | 473 | `1728b59d7a490b349cc0f85768eb12d078f1ee0ccbcd4b30620f990e5a9ddb9e` |
| `GetWindowProcessId.md` | `窗口/GetWindowProcessId.htm` | `0ea831db83e52ced16ea96be2dc2c25e15352e3fc85aae0d0be61e03e929b559` | 506 | `3efea11f6a11e64105872da198b75f4cf7f377b44a62d81f11b1cef199dd0923` |
| `GetWindowTitle.md` | `窗口/GetWindowTitle.htm` | `2cfabd2918b93a7a3d9ff7130b7a95e7d106a7d393693c1f9e127c29e4c31cf6` | 473 | `d24f0437d04d329ef555f34934c360a1b27f6d787fae1630b5a8e1dfa70e8c07` |
| `IsBind.md` | `后台设置/IsBind.htm` | `9da4670b6fe7c0bd9499307a5a293a4a2de353f07da3bfd5371ccf8d41834718` | 531 | `03305aa511f5e55207050959c1c536ee32b35fffde674c91c85e40804a56feaf` |
| `KeyDown.md` | `键鼠/KeyDown.htm` | `2c908e2d0ce374c31bf7536d055bcc8e2b33425091f53ceed7d6a8a531a52af3` | 461 | `3f4bb684cc73225c65b673b459e4c69363b5b2e11f936cc1f041a665fce03d62` |
| `KeyPress.md` | `键鼠/KeyPress.htm` | `60071f3c51d563d60ec02d2f5491a8954d89886ea3bf6fd7943f0e7d28d06820` | 464 | `ad4b3e42163db4a81246599351e2d7e95a758af50ec99f7adc63741daf6405db` |
| `KeyUp.md` | `键鼠/KeyUp.htm` | `86bdf985e1291b44ea894527d8fad7b3ccf8d1b1f9db1fda55c3d4972c024858` | 453 | `f6595b0391dac824cf3fe31b3233e799273087b00c1255d9916c7a8fcc2c69b5` |
| `LeftClick.md` | `键鼠/LeftClick.htm` | `216d312fb35c3262b872913f53b61c65605c2a1ee3d74cc3cd654def94dd9222` | 413 | `5e05211387a9ebfea5590d7b988d09b632a32125cef4831849b3c55454cf62ea` |
| `MoveTo.md` | `键鼠/MoveTo.htm` | `73eaf2b76fc064b170f4d11f937d917f578306630afc604becbde17ea8c4dc96` | 469 | `d66066166928cc75c28dc860107583bdd9f9af03004758bffacf47f3e2215fbd` |
| `Ocr.md` | `文字识别/Ocr.htm` | `3c4b10c854fb9fc7e6344e372c9274a1dd115e11329a63b09c65f47e094fe92b` | 1021 | `a045642755ab5d0a1c1535e395b6efee6da7a205ff396ffb1080edc5ff024169` |
| `OcrEx.md` | `文字识别/OcrEx.htm` | `476bf99667d5a58e32549b4056bc1fedf868ac61638a99b2bb5de4bd994052c4` | 1110 | `325419709cb3196d87a8983d56fecff48c9047ec1398ea43e58a42fd16fc8780` |
| `Reg.md` | `基本设置/Reg.htm` | `f2eb457524019b7ab7089d9c3aac5d24ea386d36f77ebb5b37041a2d327b86a3` | 2345 | `2936aa71a15fce8eecd854af51dbd217f2786a126b8306c1a1313be69f4ec50b` |
| `RightClick.md` | `键鼠/RightClick.htm` | `dd18dcea9aefcee54bf8149e8e7fd0898ec734dd47519002f7bac7fd9e749c85` | 416 | `4c8dd7a2d50ae8b3618a54e43ff732444046da146d3cdd0ac545409236c21d92` |
| `ScreenToClient.md` | `窗口/ScreenToClient.htm` | `5712bc24075db0e82c7be54eda6d74959aed1d767d11fe4bee4cf8bb05039a9d` | 566 | `2479c3ade5339f7eb99be072da160cbb3a3bdd561949eb3f89e903e45996a9f2` |
| `SetDict.md` | `文字识别/SetDict.htm` | `ff5706f005a941153e18cf28ac5d6dd37014fbad3f9265039729ba4e6df47b36` | 634 | `fa277072ce5772e6d7c187109315b2146143ff8f9c3190d58641fefc27786085` |
| `SetKeypadDelay.md` | `键鼠/SetKeypadDelay.htm` | `fff171f5cab793b3d5855c58f3f4dd45995a86f00c57eac788b35ff9c8c3087c` | 876 | `896387eaafd84b927369480d4b68114c622904aaee01c34cac858dcf825aedc9` |
| `SetMouseDelay.md` | `键鼠/SetMouseDelay.htm` | `4c7531ed1d5bf1d9f70bf14bc6d7a48242871ba3d9e9ab5d3e8925d86735bca5` | 932 | `e06dad8224b99077d5bc7daf2fefd44983554f269bd4bc9149f4a369e79abce0` |
| `SetPath.md` | `基本设置/SetPath.htm` | `4ebc39e8aaaabeafb25239bd76211c8154dd5a4f5b63e093a2fd79095d0eb40c` | 599 | `38c6465abf2923002f6c36016dc420061f209c4b767d0ccdac82cd945af9f4ac` |
| `UnBindWindow.md` | `后台设置/UnBindWindow.htm` | `79bd6e2e4c1a828299fdd0ed3590c75c756cff76844be558bef12390f5631b4b` | 480 | `f731af3aff5a6a2e84042cf4faf2e2c0935c70cc85c1b86ea7b649bb803ed278` |
| `UseDict.md` | `文字识别/UseDict.htm` | `0f58f27705b53bdf5aa73dac0604b8b1158f2e649fced5321cb4489abc6362f2` | 552 | `04e7bf25af151aa3a96d2019f7da1fcfee9ab0d7ab27fd61152723593f522e32` |
| `Ver.md` | `基本设置/Ver.htm` | `f675f22beb154a5e56bf62463de8d9270e047bb5d98d03b786f99e6f2b1745aa` | 428 | `1428b14337e70a39b1fc7d2a544059c2da290fa42b595dae5bc44759fd32e55c` |
| `binding-scope.md` | `常见问题/后台绑定会对哪些接口有影响.htm` | `08118c6a7408e08171f803da86f98dc2f37f165d183316165016670e0a591239` | 641 | `60e16a41142e8c156b11c5871ea38d00d08f343c6244dedb43fe5ee6ec4365c7` |
| `input-channel.md` | `常见问题/为什么不能和别的后台插件混用.htm` | `df0cfe61de15d1b8e8c64655bea1398fec24d510cde826c30eff71142f1df9c9` | 578 | `79c36bb7a04b0f9e411b01171456fdcdfb9670fb4e296b0c4b7ba6d498446321` |
| `object-scope.md` | `常见问题/插件接口生效范围.htm` | `fc37fe951d810c1e595857874fd7ead6b3ffdb8fd56ebe9483534c77125a7708` | 610 | `781736f2f2b82be32ac321c18845bed839d8794e05f02b4e44d34217f0015ef3` |
| `threading.md` | `常见问题/如何在多线程下使用.htm` | `35c596e8b41493c94002a96da2193ec0cd6d76e4cfa6cb6359aec8ba8a8093d5` | 2143 | `33a9d7698dce4eb77b7f6ab0a8bcd84dd94fe513f37765bdebf61977388a7b20` |
