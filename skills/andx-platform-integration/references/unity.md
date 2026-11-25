# Unity 与 UNIHper

> 状态：active ｜ 最后更新：2026-11-24 ｜ 负责人：AndX 维护者

## 版本与初始化

读取 ProjectSettings/ProjectVersion.txt、Packages/manifest.json、packages-lock.json，定位实际 `com.parful.andx`。检查 UNIHper、IOToolkit、UniRx 及 asmdef/version define。读包 README、Documentation~/usage.md 和公开类型，不修改包缓存或用升级引擎代替修复。

- 独立工程按该包公开入口初始化一次；明确 Direct/Edge/Auto 配置，报告实际解析结果的脱敏摘要，Auto 不代表连接必成功。
- UNIHper 工程配合 `unihper-unity-development`。当前实现由 AndXUNIHperSettings 加载时初始化；业务不再 Init，配置/资源就绪后调用，不在过早 Awake 读取配置。
- 配置目录以实际序列化属性及平台为准，不把桌面 StreamingAssets 路径套到 Android/WebGL；重载使用该版本公开桥接入口。
- 凭据与 NodeId 配套，但由部署设施注入。真实密钥不得进入 XML 模板、StreamingAssets 制品、截图、交接 CSV、源码和日志。

## 按能力适配

分享/收图复用 SDK 门面与 payload 类型，处理二维码、终态、超时、取消、下载/展示失败。场景退出取消任务/轮询、释放订阅及本适配器拥有的纹理，不释放外部共享资源。

AI 先发现服务端能力与策略，计费任务由明确操作触发，不在初始化自动调用；厂商密钥留服务端，生成结果回流媒体。

支付履约阅读包内 AIP 运行时与 Sample，使用对应版本的 ExhibitRuntime 和 IExhibitRuntimeAdapter；初始化节点不等于绑定履约运行时。当前文档区分 start→STARTING、真实可交互后 NotifyPlayingAsync、结束后 NotifyCompletedAsync，具体签名以安装版本为准。

物理动作通过应用本地受控映射；验证主线程、重入、取消、幂等安全停止、命令持久化与崩溃恢复。未知执行结果不自动重放不可逆动作。历史 Adapter 即使提供任意 channel/value，也不能直接把公网输入转成 IODevice 原始写入。

## 平台差异

- WebGL 是浏览器，静态配置不能保管 NodeSecret；确认目标版本的浏览器传输、鉴权、CORS/同源、TLS 与 WebSocket。BFF/Sidecar 需实际可达；HTTPS 页面访问不安全环回可能受限，Editor 不代替浏览器构建验证。
- Android 核对 ABI、IL2CPP/AOT、裁剪、原生依赖、权限与可信凭据设施；需设备启动、暂停/退出、断网恢复和安全停止证据。
- Unity 自动化连接正确工程和引擎版本；不可用时如实标记 PlayMode 待验，不能用静态检查代替运行。

按请求能力做编译、相关 EditMode/PlayMode、目标构建、服务联调和设备验收。收图至少完成/取消/过期；履约另验重复 start、真实 PLAYING、超时、停止与恢复。纯分享不强加整套硬件验收。
