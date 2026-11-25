# 平台选择与版本事实

> 状态：active ｜ 最后更新：2026-11-24 ｜ 负责人：AndX 维护者

## 识别输入

| 维度 | 实际读取的证据 |
| --- | --- |
| 工程/宿主 | Unity ProjectSettings、Packages；UE uproject/uplugin/Build.cs；H5 package.json；MyriadBox exhibit.json、模板与宿主配置 |
| 目标 | Windows、Android ABI/API、WebGL、纯 Web，构建脚本与设备 |
| 能力 | 分享、收图、媒体、AI、支付签票/查询、AIP 履约、业务交互 |
| 拓扑 | BFF、宿主 Provider、节点直连、环回 Sidecar、Relay、Hub |
| 身份 | 稳定业务 ID、节点授权与凭据注入责任方，只检查来源和存在性 |
| 版本 | SDK/宿主/插件、锁文件、ref/commit、契约来源与 dirty 状态 |

## 选择顺序

1. 先区分定制 iframe、公开 token 页面、展项侧浏览器、原生宿主；它们信任边界不同。
2. 定制 iframe 使用 Bridge；公开 token 页只使用公开客户端；展项浏览器使用同源 BFF；MyriadBox 使用实际版本支持的宿主 Provider/WebBridge。
3. Unity 优先复用已安装 `com.parful.andx` 的对应能力，UNIHper 自动初始化与独立初始化互斥。不要因架构推荐 IOUI 就强制迁移可用原生 SDK。
4. IODevice 新增交互按 IOUI-ANDX/单 Host 边界评估；非 IODevice、多 Host 或独立升级评估 Sidecar，均先证明实现存在。
5. 多屏按 Hub 授权与 NetIO 局域网传输分工。UE/WebGL 每项检查实现与条件，不能从平台名推断可用性。

## 事实源

AndX 源码在手时，从实际根目录读取以下相对路径，不写死机器盘符：

- `docs/README.md`、`docs/standards/engineering-standards.md`：地图与规范。
- `docs/design/andx-ioui-exhibit-interaction-architecture.md`：架构边界。
- `docs/design/andx-ioui-implementation-plan.md`：规划，不是实现证明。
- `andx-sdk-spec/`：package.json、README、AIP/Bridge 契约、fixtures 和校验器。
- `packages/andx-web/`：公开导出、类型、测试与文档。
- `unity/com.parful.andx/`：独立子模块，单独记录版本与修改状态。

只有消费工程时使用安装包与匹配版本契约/文档；缺失则指出具体所需材料，不假设完整 AndX 仓库必然存在。

## 首版观察边界

2026-11-24 只读核对看到 Unity 包标识 2.4.1、Web SDK 0.1.0、契约包 0.4.0。它们是编写时观察，不是安装要求、最新版本或已发布证明；目标工程以自身锁定版本为准。

已找到 Unity 独立/UNIHper 初始化、AIP 履约入口，以及 Web 公开/BFF/Bridge 导出。本次没有验证 UE、WebGL、Android 或 Hub/NetIO 现场可用性。

旧章节/配置可能仍含 EdgeKey 或原始 IO 示例。字段存在不代表应启用；对照目标契约、服务端与适配层，冲突写入缺口，禁止用旧例绕过安全边界。

## 路由实例

- UNIHper 接扫码收图：Unity 路线，复用自动配置，验证完成/取消；不强加控制租约与发布流程。
- MyriadBox 显示真实节点状态：宿主 Provider 路线，验证当前能力；不把浏览器 Mock 当真实数据。
- /x/ 手机定制页：交接给定制页技能，页面不访问宿主 Provider。
- UE Android 远控：先检查插件/ABI/回调/安全停止，未有证据的能力待验。
