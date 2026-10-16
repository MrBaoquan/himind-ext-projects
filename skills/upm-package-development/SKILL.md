---
name: upm-package-development
description: 规范 Unity UPM 包的结构、程序集、测试、版本与内网发布，UNIHper 作为可选增强。用户要新建或改造 UPM 包、配置内网 registry、排查包安装或编译问题时使用。
---

# UPM 包开发规范

以目标工程实际的 Unity 版本、包清单和程序集定义为准，完成符合 Unity 官方包约定的实现与发布。不要照搬其他仓库的包名、命名空间、平台声明或版本号。

## 开始前检查

1. 确认目标目录是 Unity 工程：定位 `ProjectSettings/ProjectVersion.txt`、`Packages/manifest.json`、`Packages/packages-lock.json`。
2. 确认包的开发与安装形态：embedded（`Packages/<pkg>`）、本地路径（`file:`）、tarball、Git URL、内网 scoped registry。形态决定测试能否被自动发现，先判断再动手。
3. 读目标包现有 `package.json`、全部 `.asmdef`、`CHANGELOG.md` 与目录布局，沿用既有命名空间、程序集命名和目录习惯。
4. 确认平台与 Unity 版本：`unity` 字段、`includePlatforms` / `excludePlatforms`、需要支持的构建目标。
5. 修改前给出文件清单。不要改动 `Library/`、`Temp/`、`Logs/`、`obj/`、`bin/`、`UserSettings/` 和第三方包缓存。
6. 包在独立仓库、通过内网 registry 分发时，发布目标固定为 `http://upm.andcrane.com:4873`；不要改用公共 npm 源。
7. 新建包前先确定包名前缀，规则见「三、package.json」第 1 条；改造既有包只沿用其现有前缀，不要改写。

## 一、先判定要不要做成包

| 情形 | 处置 |
| --- | --- |
| 多工程共享、需要版本化与独立升级 | 做成 UPM 包 |
| 需要跨项目交付、需要可复现安装 | 做成 UPM 包 |
| 只服务当前工程、无复用计划 | 放 `Assets/`，不要为一次性代码建包 |
| 只是编辑器窗口脚本、无分发需求 | 放 `Assets/Editor/`，不要建包 |

需求同时命中「包」和「工程内脚本」时，先问复用范围。为一次性代码建包会让后续每次改动都背上版本与兼容成本。

## 二、目录布局

采用 Unity 官方包布局，不要另起炉灶：

```plain
<package-root>
  ├── package.json
  ├── README.md
  ├── CHANGELOG.md
  ├── LICENSE.md              # 未在 package.json 写 license 时必须提供
  ├── Third Party Notices.md  # 含第三方 DLL 或代码时提供
  ├── Runtime/                # 随包编译的运行时代码
  ├── Editor/                 # 仅编辑器使用的代码（可选）
  ├── Tests/
  │   ├── Editor/             # EditMode 测试
  │   └── Runtime/            # PlayMode 测试
  ├── Samples~/               # 按需导入的示例（导入后 GUID 需保留）
  └── Documentation~/         # 包内文档，不进工程
```

- 以 `~` 结尾的目录是「Unity 不导入」的唯一手段：`Samples~`、`Documentation~`、`Tests~`。不要用其他方式隐藏目录。
- **包内文档一律放 `Documentation~/`**。不要用 `docs/`：它会被当成普通目录导入，包内 Markdown 会变成工程资源。
- `Runtime/` 与 `Editor/` 是约定目录，不改变导入管线；真正的隔离靠 `.asmdef`。
- `Editor/` 里的代码可以引用 `Runtime/`；`Runtime/` 不得引用 `Editor/`。
- 包内文档只放使用说明。设计与方案文档留在项目文档目录，不要在包里新建规范类文档。
- 第三方 DLL 要写明来源与许可；不要把无关的第三方 DLL 打进包里（会产生意外隐式引用）。

## 三、package.json

```json
{
  "name": "com.parful.<包名>",
  "version": "1.0.0",
  "displayName": "显示名",
  "description": "一句话说明这个包做什么",
  "unity": "2020.3",
  "unityRelease": "0f1",
  "documentationUrl": "http://readme.andcrane.com:6875/books/upm/page/<页面ID>",
  "changelogUrl": "http://upm.andcrane.com:4873/-/web/detail/com.parful.<包名>",
  "license": "UNLICENSED",
  "keywords": ["..."],
  "author": { "name": "MrBaoquan", "email": "..." },
  "dependencies": { "com.unity.nuget.newtonsoft-json": "3.2.1" },
  "samples": [
    { "displayName": "示例", "description": "说明", "path": "Samples~/示例" }
  ]
}
```

约束：

1. `name` 用反向域名全小写，形如 `com.<组织>.<包名>`。**`name` 一经发布不可更改**，改名等于发布新包（见第十节）。
   - **既有归属优先**：仓库/工程已有包名前缀的一律沿用，不重新决策。内网现存先例：`com.parful.*`（作者 `MrBaoquan`）、`com.software.*`（其他作者）、`com.szkt.*`、`com.andcrane.*`。改造既有包时**不得**改写其前缀。
   - **新建包按作者取默认值**：本机作者 `MrBaoquan` 用 `com.parful.*`；其他作者用 `com.software.*`。
   - 作者身份以仓库/工程的既有归属为准（`author.name`、包名历史、Git 身份）。归属不明时先确认，不要按个人习惯挑前缀——前缀决定包进哪个 registry scope（见第 10.4 节），也是最难改的一处。
2. `version` 严格 `major.minor.patch`，遵循 SemVer；新包从 `0.1.0` 起步，稳定后发 `1.0.0`。
3. `unity` 写 `major.minor`，指向最低兼容版本；需要更精确时补 `unityRelease`（`unity` 缺省时 `unityRelease` 无效）。
4. `dependencies` **不支持范围语法**，只能写具体版本；只列真实需要的包，不要把可选依赖写进去（会被强制安装）。
5. `samples[].path` 必须指向 `Samples~` 下的目录。若示例目录内存在 `.sample.json`，则整个 `samples` 数组会被忽略，元数据以 `.sample.json` 为准，不要两处都写。
6. `type` 为引擎内部保留字段，不要使用。
7. `documentationUrl` / `changelogUrl` / `licensesUrl` 必须指向真实可访问的地址，**不要留占位值**。Unity 包模板自带的 `https://example.com/licensing.html` 就是占位域名，复制模板后必须替换或删除该字段。内网包可以填内网文档站与内网 registry 详情页（`http://upm.andcrane.com:4873/-/web/detail/<包名>`）——但 `changelogUrl` 指向 registry 时，该包名必须**真的发布过**，否则是 404 死链。拿不出真实地址时直接省略字段，改由 `Documentation~` 与包内 `LICENSE.md` 兜底。
8. 未写 `license` 字段时，包根**必须**有 `LICENSE.md`。内网专有包建议写成 `"license": "UNLICENSED"`（本项目先例：`server/package.json`）并随包附 `LICENSE.md` 写明版权与使用限制；对外开源的包才填真实 SPDX 标识（如 `MIT`、`Apache-2.0`）。

## 四、程序集定义（.asmdef）

含代码的包必须有 `.asmdef`：`Packages/` 下的脚本只有被程序集定义收拢才会编译。

运行时程序集：

```json
{
  "name": "<包名>.Runtime",
  "rootNamespace": "<命名空间>",
  "references": [],
  "includePlatforms": [],
  "excludePlatforms": [],
  "allowUnsafeCode": false,
  "overrideReferences": false,
  "autoReferenced": true,
  "defineConstraints": [],
  "versionDefines": [],
  "noEngineReferences": false
}
```

编辑器程序集：在上面的基础上设 `"includePlatforms": ["Editor"]`，`references` 里加运行时程序集。

测试程序集（Unity 2020–2022 写法）：

```json
{
  "name": "<包名>.Tests.Runtime",
  "rootNamespace": "<命名空间>.Tests",
  "references": ["<包名>.Runtime", "UnityEngine.TestRunner"],
  "includePlatforms": [],
  "excludePlatforms": [],
  "overrideReferences": true,
  "precompiledReferences": ["nunit.framework.dll"],
  "autoReferenced": false,
  "defineConstraints": ["UNITY_INCLUDE_TESTS"]
}
```

EditMode 测试程序集再加 `"includePlatforms": ["Editor"]` 与 `"UnityEditor.TestRunner"`。较新的 Unity 生成的是精简写法（`optionalUnityReferences: ["TestAssemblies"]`），以目标工程实际生成为准。

约束：

1. 程序集命名 `<包名>.<层>`，`<层>` 取 `Runtime` / `Editor` / `Tests.Editor` / `Tests.Runtime` 四者之一；包名段用包的主标识（`com.parful.andx` → `AndX`）。组织段可有可无：`AndX.Runtime` 与 `Parful.AndX.Runtime` **都合规**，但**同一包内风格必须一致**，不要一半带组织段一半不带。既有包沿用其现有风格，不要为了「统一风格」而改名——改名程序集属对外可见变更，按第 8 条提版本。
2. 运行时程序集**必须显式声明 `rootNamespace`**，避免全局命名空间污染。
3. 运行时程序集不得引用 `UnityEditor`；需要编辑器能力就单开 `Editor` 程序集。
4. `references` 用程序集名或 GUID，**同一文件内不得混用**两种形式。
5. `includePlatforms` 与 `excludePlatforms` 互斥：一个非空时另一个必须为空。
6. 测试程序集保持 `autoReferenced: false` + `UNITY_INCLUDE_TESTS` 约束，避免测试类型进入正式构建。
7. 只有确定不需要引擎类型的纯 C# 程序集才设 `noEngineReferences: true`。
8. 改动 `autoReferenced`、增删程序集、增删平台、增删 `defineConstraints` 都属于**对外可见变更**，按第六节的版本规则提版本。

## 五、可选依赖与可选框架支持（以 UNIHper 为例）

框架增强必须是**软依赖**：用户没装 UNIHper 时包照常编译运行，装了之后自动启用增强。

实现方式：独立集成程序集 + `versionDefines` + `defineConstraints`。

```json
{
  "name": "<包名>.Integration.Unihper",
  "rootNamespace": "<命名空间>.Integration.Unihper",
  "references": ["<包名>.Runtime", "UniHper"],
  "includePlatforms": [],
  "excludePlatforms": [],
  "autoReferenced": false,
  "defineConstraints": ["UNIHPER_PRESENT"],
  "versionDefines": [
    {
      "name": "com.parful.unihper",
      "expression": "1.26.410",
      "define": "UNIHPER_PRESENT"
    }
  ]
}
```

要点：

1. **不要**把 `com.parful.unihper` 写进 `package.json` 的 `dependencies`——那会强制所有使用者安装它。
2. `defineConstraints` 不满足时 Unity 直接跳过该程序集，因此 `references` 里指向缺失程序集不会报错。这正是软依赖成立的原因。
3. 版本表达式语义随 Unity 版本而异（常见写法是「不低于该版本」），表达式取值与边界以目标 Unity 版本的 Version Defines 文档为准；**不要写空表达式**，也不要照抄示例里的版本号——写目标包真实的最低兼容版本（示例中的 `1.26.410` 取自内网 `com.parful.unihper` 现况）。
4. 源码用 `#if UNIHPER_PRESENT` 包裹，且只在集成程序集内使用框架类型；核心运行时程序集不得出现任何 UNIHper 符号。
5. UNIHper 提供的能力（可在增强层使用，不要重复实现）：`Managements` 门面、UGUI 页面基类与 `UIPage` 特性、UI Toolkit 页面基类、`UConfig` 配置模型（`SerializedAt` / `SerializeWith`）、`Managements.Resource` 资源加载、`SceneScriptBase` 场景脚本。
6. 增强层不得改变核心层的公开行为；只做类型转换、生命周期桥接与配置读写适配。

## 六、Unity 原生平台适配约束

1. **主线程**：运行时 API 只能在主线程调用。异步续接可能落到线程池，需要在适配层把调用编组回主线程（登记主线程同步上下文），不要要求调用方保证线程；纯逻辑层保持 `ConfigureAwait(false)`，线程亲和属于适配层职责。
2. **网络**：统一用 `UnityWebRequest`。`System.Net.Http` 在 WebGL 不可用，不要作为唯一实现路径。
3. **文件**：WebGL 无本地文件系统；移动端只能写 `Application.persistentDataPath`；`StreamingAssets` 只读，且 Android 上位于 APK 内，不能当普通文件读取。
4. **多线程**：WebGL 无线程，`Task.Run` 之类不会真正并行。不要在 WebGL 目标上依赖后台线程完成任务。
5. **IL2CPP / AOT**：`System.Reflection.Emit`、`dynamic`、未实例化的泛型虚方法不可用。需要反射的地方改用显式注册或接口。
6. **代码剥离**：IL2CPP 托管代码剥离会删掉仅被反射引用的类型；用 `link.xml` 或 `[Preserve]` 保住。
7. **序列化**：公开字段与 `[SerializeField]` 字段的类型、名称变更都是 breaking change；移除或改名序列化成员按 major 提版本。
8. **平台分支**：用 `UNITY_ANDROID`、`UNITY_WEBGL`、`UNITY_EDITOR` 等内置宏。不要在运行时程序集里用 `#if UNITY_EDITOR` 代替编辑器程序集划分。
9. **渲染管线**：需要跨 Built-in / URP / HDRP 时用 `versionDefines` 检测管线包并分支出对应实现，或在文档中显式声明只支持某一种。
10. **资源引用**：用 GUID 引用资源，不要硬编码 `Assets/...` 路径。
11. **调试与日志**：不要用 `console.log` 式的无条件打印；日志要能按开关关闭，且不得输出凭据、令牌或用户隐私数据。
12. **密钥**：包内不得硬编码任何密钥、令牌或账号；一律由运行时注入。

## 七、.meta 与版本控制

1. Unity 会导入的每个文件和目录都必须有同名 `.meta` 并入库，包括文件夹。缺失会导致每次导入重新分配 GUID，引用随之丢失。
2. `~` 结尾的目录 Unity 不导入，目录**自身**不需要 `.meta`（`Samples~`、`Documentation~`、`Tests~` 一样）。但 **`Samples~` 内部**的资源必须有 `.meta`：导入样例时内容会被拷贝进 `Assets/`，缺 `.meta` 就会重新分配 GUID，样例场景/预制体对脚本和贴图的引用会直接断掉（Unity 官方包如 `com.unity.addressables` 即如此）。`Documentation~`、`Tests~` 的内容永不进 `Assets/`，不需要 `.meta`。
3. 孤儿 `.meta`（找不到对应资源）必须删除，否则每次导入都会往控制台刷告警。
4. 移动或改名资源时连同 `.meta` 一起移动，保留 GUID。让 GUID 变化等同于「删旧增新」，属于 major 变更。
5. 新建文件/目录的 `.meta` 可以用唯一随机 GUID 手工补齐（Unity 首次导入会沿用，官方包也是这么做的），但**不得**用编造的 GUID 替换已导入资源的既有 GUID。
6. 构建产物与本地文件不进版本库：`obj/`、`bin/`、`*.csproj`、`*.user`、`*.tgz`、`Library/`。
7. 包为独立仓库时，包根与仓库根通常同级；文件名大小写要保持稳定，避免在大小写不敏感的文件系统上产生假重命名。

## 八、测试

分三层，各管一段，不要用一层的结果代替另一层：

| 层 | 位置 | 运行方式 | 覆盖范围 |
| --- | --- | --- | --- |
| 引擎无关核 | `Tests~/*.csproj`（可用 `Compile Include` 直接引入 `Runtime/` 中无引擎依赖的源码） | `dotnet test` | 纯逻辑、序列化、契约常量 |
| EditMode | `Tests/Editor` | Unity Test Runner / `-runTests -testPlatform EditMode` | 编辑器 API、资产与载荷编码 |
| PlayMode | `Tests/Runtime` | Unity Test Runner / `-runTests -testPlatform PlayMode` | 生命周期、线程编组、真实请求链路 |

约束：

1. **`dotnet test` 全绿不等于引擎可编译**。引擎适配层引用 `UnityEngine`，外部 csproj 并不编译这些文件，必须另设真实 Unity 工程编译门禁。
2. 包在 `Packages/` 下（embedded）时测试程序集会被自动发现；包在工程外时，需要在项目 manifest 的 `testables` 中列出包名才能运行其测试。
3. 被测对象是进程级全局单例时，测试程序集必须禁用并行（如 `[assembly: CollectionBehavior(DisableTestParallelization = true)]`），否则用例间会互相覆盖状态。
4. 契约常量（错误码、路径、请求头、枚举）在多处各存一份时，用双向集合断言守护：任一侧多出或缺少都让测试失败。
5. 批处理门禁命令：

```bash
Unity -batchmode -nographics -quit \
  -projectPath <工程路径> \
  -runTests -testPlatform EditMode \
  -testResults <结果xml路径> \
  -logFile -
```

6. 至少验证一次真实安装路径：用干净工程按目标分发方式（registry / Git URL / tarball）安装后再跑一次冒烟。

## 九、文档与变更日志

1. `README.md`：包 ID、最低 Unity 版本、依赖、支持平台、安装方式、快速开始、API 速查。
2. `CHANGELOG.md`：遵循 Keep a Changelog，`## [Unreleased]` 常驻；发布时改为 `## [x.y.z] - YYYY-MM-DD` 并新建空的 `Unreleased`。分类用 `Added` / `Changed` / `Deprecated` / `Removed` / `Fixed` / `Security`，同一版本内按该顺序排列。
3. 破坏性变更必须写明「旧写法 → 新写法」对照。
4. `Documentation~` 用 Markdown，建议章节：About、Installing、Requirements、Usage、Known limitations。
5. 改动公开 API、平台支持或依赖时同步更新文档与包清单，否则视为未完成。
6. 清理模板残留：从 Keep a Changelog 模板生成的 `CHANGELOG.md` 会自带模板自己的历史（`## [1.1.0] - 2019-02-15`、丹麦语/格鲁吉亚语翻译等条目），**必须整段删掉**。版本历史只能记本包真实发布过的版本，日期取 registry 上的实际发布时间或对应 Git tag；改名/换仓库的包要注明前身包名，避免读者误以为新包从未发布过。
7. 文档里引用主仓文件时写明归属（如「AndX 主仓 `docs/design/implementation.md`」），不要留成裸 `docs/...`——对 registry 消费者那是死路径。

## 十、版本与发布

### 10.1 版本号怎么提

| 变更 | 最低增量 |
| --- | --- |
| 删除或重命名程序集、在程序集间移动公开 API、移除平台支持、新增 `defineConstraints`、改动 `autoReferenced`、标记为测试程序集 | major |
| 变更资产 GUID、删除可被引用的资源 | major |
| 重命名或移除公开类型/成员、改签名或返回类型、改枚举成员或底层类型、改序列化字段类型、改名或移除序列化成员、移除已过时 API | major |
| 依赖变更导致行为或 API 不再向后兼容 | major |
| 新增程序集、新增平台支持、移除 `defineConstraints` | minor |
| 新增公开类型/成员/重载、新增资产或内容 | minor |
| 提升 `unity` / `unityRelease` | minor 起 |
| 标记 API 过时（先过时一轮，下一轮 major 再删） | minor |
| 调整 `.asmdef` 的 `references`、开关 `allowUnsafeCode` / `overrideReferences` | patch |
| 内部重构、性能优化、不改变公开面的修 bug、文档与注释 | patch |
| 依赖版本调整（无行为与 API 影响） | patch |
| 改 `name`（包重命名） | 不支持，只能作为新包发布 |

`description` / `keywords` / `displayName` 可在任意版本调整。

### 10.2 发布到内网 registry

发布目标固定为内网 Verdaccio：`http://upm.andcrane.com:4873`。

发布前必须满足：

1. `package.json.version` = `CHANGELOG.md` 顶部版本 = Git tag（`v<x.y.z>`），三者一致。
2. 已通过引擎编译门禁与 EditMode / PlayMode 测试。
3. `.meta` 完整性检查通过（无缺失、无孤儿、`~` 目录自身无 `.meta`、`Samples~` 内部资源带 `.meta`）。
4. 包内不含密钥、凭据、绝对本机路径、`obj/`、`bin/` 与构建产物。
5. 已发布版本不得覆盖，也不得同版本替换；内容变了就提版本。
6. 新包（含改名后的包）在 registry 上必须真的存在该名字与版本。只改 `package.json` 的 `name` / `version` 而不发布，不产生新包，`changelogUrl` 会指向 404，消费端也装不到。

发布步骤：

```bash
# 包根目录；registry 也可写在仓库 .npmrc
npm config set registry http://upm.andcrane.com:4873
npm adduser --registry http://upm.andcrane.com:4873

# 先核对即将打进 tarball 的文件清单，确认没有多余或缺失
npm pack --dry-run

# 发布
npm publish --registry http://upm.andcrane.com:4873
```

发布后不要只看 registry 上出现了条目就收工，必须用干净工程按 scoped registry 方式安装一次并跑通冒烟。改名/换名的包还要确认旧名与新名在 registry 上的并存关系符合预期。

### 10.3 内网 registry 的已知约束

1. registry 必须实现 npm 协议的 `/-/v1/search` 或 `/-/all` 端点，否则 Package Manager 搜索不可用。Verdaccio 满足该要求。
2. Unity **不支持** npm 的 `@scope/name` 命名空间写法；作用域靠消费端 `scopes` 前缀匹配实现。
3. 内网用 `http://` 可以工作（Unity 官方配置示例即以 `http://localhost:4873` 演示），但 Package Manager 会提示非安全连接。条件允许时应为 registry 配 HTTPS。
4. 元数据与 tarball 不在同一服务器时，认证配置需要 `alwaysAuth = true`。

### 10.4 消费端接入

项目 `Packages/manifest.json`：

```json
{
  "scopedRegistries": [
    {
      "name": "andcrane",
      "url": "http://upm.andcrane.com:4873",
      "scopes": ["com.parful", "com.software"]
    }
  ],
  "dependencies": {
    "com.parful.example": "1.0.0"
  }
}
```

需要认证时，在用户级 `.upmconfig.toml` 中配置：

```toml
[npmAuth."http://upm.andcrane.com:4873"]
token = "<AUTH TOKEN>"
```

`scopes` 决定哪些包名走该 registry；前缀不在 `scopes` 内时，Package Manager 会转去 Unity 官方源查找并失败（报「找不到包」而不是「未授权」，容易误判）。消费端按**实际用到的包名前缀**补全 `scopes`，不要照抄示例：内网现存前缀有 `com.parful`、`com.software`、`com.szkt`、`com.andcrane`。

## 十一、发布前验收清单

- [ ] 目录布局符合官方约定，`~` 目录用法正确，包内文档在 `Documentation~/`
- [ ] `package.json` 必填与推荐字段齐全，`name` 未变更、前缀符合既有归属或作者默认（`MrBaoquan` → `com.parful.*`，其他 → `com.software.*`），`version` 已提升
- [ ] `unity` 与 `unityRelease` 反映真实最低兼容版本
- [ ] `dependencies` 只含真实依赖，无范围符号，未塞入可选依赖
- [ ] `LICENSE.md` 存在，包根许可声明与 `license` 字段一致；无 `https://example.com/...` 之类占位地址
- [ ] 每个含代码的目录都有 `.asmdef`，运行时程序集有 `rootNamespace` 且未引用 `UnityEditor`
- [ ] 测试程序集有 `UNITY_INCLUDE_TESTS` 约束且 `autoReferenced: false`
- [ ] 可选框架增强走 `versionDefines` + `defineConstraints` 的独立集成程序集
- [ ] `includePlatforms` / `excludePlatforms` 未同时非空
- [ ] `.meta` 无缺失、无孤儿；`~` 目录自身无 `.meta`，`Samples~` 内部资源带 `.meta`
- [ ] 引擎编译门禁零错误，EditMode 与 PlayMode 测试全绿
- [ ] `CHANGELOG.md` 无模板残留，`package.json.version` = CHANGELOG 顶部版本（已发布版本另有对应 Git tag），破坏性变更写了迁移对照
- [ ] `README.md` 与 `Documentation~` 与当前 API 一致，安装方式覆盖真实分发路径
- [ ] 包内无密钥、凭据、绝对本机路径与构建产物
- [ ] registry 上确实存在该包名与版本（`changelogUrl` 可打开），且安装验证通过（干净工程 + 冒烟）

## 十二、禁止事项

- 不修改 `Library/`、`Temp/`、`Logs/`、`obj/`、`bin/` 与第三方包缓存。
- 不手工编造 GUID 冒充 Unity 导入结果（新建文件可用唯一随机 GUID 补齐 `.meta`，但不得替换已导入资源的既有 GUID）；不给 `~` 目录自身放 `.meta`，也不漏掉 `Samples~` 内部资源的 `.meta`。
- 不让运行时程序集引用 `UnityEditor`，不用 `#if UNITY_EDITOR` 代替编辑器程序集划分。
- 不在包内用 `docs/` 放文档，不把可选框架写进 `dependencies`，不在核心层出现可选框架符号。
- 不覆盖已发布版本，不用同一版本替换内容。
- 不擅自改变包名前缀：既有一律沿用，不为非 `MrBaoquan` 作者创建 `com.parful.*` 包，也不把 `com.parful.*` 包改成其他前缀。
- 不只为「统一风格」而重命名既有程序集。
- 不改 `package.json` 的 `name` 却不发布（改名必须真的产生新包），不留占位域名或未发布的 `changelogUrl`。
- 不向公共 npm 源发布内网包，不把包内地址改成公网地址。
- 不在包内硬编码密钥、令牌与账号。
- 不执行版本控制提交、registry 发布或包上传，除非用户明确要求并确认范围。
