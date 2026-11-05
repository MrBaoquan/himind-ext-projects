# MyriadBox 交付细则

## 工程与桌面

```text
exhibit.json
wwwroot/              # 构建产物
config/               # 现场可修改业务配置与 Schema
media/                # 现场可替换媒体
hardware/IODevice.xml
hardware/bindings.json
hardware/Config/      # IOUI 插件配置
dist/                 # 默认独立交付目录
```

`dist/` 必须包含：

```text
MyriadBox.exe         # NativeAOT 启动器
runtime/              # 固定版本、self-contained Core
exhibit.json
wwwroot/
config/
media/
hardware/
providers/            # 如项目需要
dist.manifest.json
VERSION.txt
```

`dist` 不包含软件分发元数据、更新器、productId、Token 或预检收据。

## AI 调试会话

- `exhibit.web.dev.start` 返回 HTTP、CDP、Bridge 和 session 文件。
- 浏览器模式直接访问 `httpUrl`。
- 真实壳模式由 Playwright `connectOverCDP(cdpUrl)` 连接 WebView2。
- Mock 与真实硬件证据不能混写。
- 调试结束必须调用 `exhibit.web.dev.stop`。

## 证据关联

- `dist.manifest.json` 的 Build ID、Core 版本和 Core SHA-256 是展项交付事实。
- 兼容内容包的 Candidate SHA-256 单独记录，不替代 dist 运行验收。
- 截图、console、日志和报告关联同一 Build ID 和规格 revision。
- 真实按钮、Modbus、动感平台必须保留人工验收状态。

## 失败处理

- 构建失败：保留日志，不生成或替换 dist。
- `dist verify` 失败：阻断交付。
- Provider 启动失败：检查锁、端口和原生依赖，不静默降级为完成。
- 页面 Console 异常：阻断完成。
- 真实硬件未验证：标记人工验收未完成。
## 多目标产物

```text
artifacts/final/
  web/
  win-x64/
  android-arm64/*.apk
  build-manifest.json
  SHA256SUMS.txt
```

- 三端必须来自同一份 `wwwroot/` 构建结果；Android 禁止单独复制页面。
- `web` 使用相对静态资源路径，确保部署到子目录和 Android `appassets` 时都可加载。
- `win-x64` 的 Windows Shell 必须向 WebView2 注入 `MyriadBox` 运行时可执行契约。
- Android APK 必须包含 `lib/arm64-v8a/iodevice.so`、展项 runtime 与 target manifest。

## Android 调试与验收

- Debug APK 使用：

```powershell
myriad build <project> --target android-arm64 --android-webview-debug --iodevice-root <IODevice/Android/Release>
adb install -r <apk>
adb forward tcp:9222 localabstract:webview_devtools_remote_<pid>
```

- 先用 `exhibit.web.apk.verify` 检查 APK 载荷和签名，再使用安装、启动、日志与 CDP 能力。
- Playwright Android CDP 不兼容时，直接连接 page WebSocket，按需调用 `Runtime.evaluate`、`Network.*`、`Page.*`。
- Release APK 必须重新构建且不带 `--android-webview-debug`；安装启动后确认 `webview_devtools_remote` 不存在。
- Android 虚拟显示可能不返回 `Page.captureScreenshot`，此时保留 DOM/Console/网络证据，并明确截图来自同一 H5 的浏览器或 Windows 目标。

## 页面稳定性

- 不把状态推进依赖单一 `transitionend`；WebView/虚拟显示可能丢事件。使用直接条件渲染或带明确时限的状态切换。
- 等待音频 `play()` 失败时不得把媒体库素材误判为不可用；播放失败与素材搜索失败分别记录。
- 切换环境声产生的已取消请求属于预期；只有非取消网络失败和 4xx/5xx 才应阻断。
