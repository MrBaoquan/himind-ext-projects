---
name: exhibit-web-development
description: 使用 MyriadBox 模板、AI/CDP 调试、体验门禁和多目标构建开发 H5 展项；用户提到网页展项、WebView、WebBridge、IODevice、外部视频或 APK 交付时使用。
---

# 展项 Web 开发助手

## 目标

把已确认的展项需求实现为可离线运行、可接实体交互、可复制并双击 `MyriadBox.exe` 运行的独立 `dist`。

业务事实源仍在工作台；确定性动作由 MyriadBox CLI 与 `com.himind.exhibit-development-tools` 完成。

## 分发边界

1. MyriadBox Core 由组织软件分发更新；展项不接软件分发。
2. 展项交付物是独立 `dist/`，其中包含根级 `MyriadBox.exe`、固定 Core `runtime/`、内容、配置、媒体、Provider 和 `dist.manifest.json`。
3. 展项 `dist` 不得包含分发地址、productId、凭据、自动更新器、`distribution.json` 或预检收据。
4. 已交付展项需要更新 Core 时，重新构建并替换 `dist`；不要尝试让展项自己更新 Core。

## 必用同伴

1. 模板、调试、体验检查、构建、dist 校验和日志使用 `com.himind.exhibit-development-tools`。
2. 文案层级、转场、持续动态和人工体验验收使用官方 `exhibit-experience-direction` Skill。
3. 脚本、规格和证据读写使用内置 Workbench Provider。
4. 浏览器调试使用 AI 工具已有的 Playwright/CDP 能力，不自建浏览器自动化内核。
5. Core 发布使用官方 `software-distribution` 能力的 `distribution.*`，不复制分发协议。

## 开始项目

1. 先调用 `exhibit.web.template.list`，按框架、媒体、Provider 和 AI 调试能力选择模板。
2. 需要新建工程时调用 `exhibit.web.template.create`，不要手工复制模板目录。
3. 读取 `.myriadbox/template.json`、`exhibit.json`、`config/*.json`、`hardware/bindings.json`。
4. 记录展项 `pid`、规格 revision、模板 id/version、工作区与目标分辨率。
5. 没有已发布规格时停止开发，不根据聊天猜测业务事实。

## AI 调试循环

1. 用 `exhibit.web.dev.start` 启动开发会话；纯页面可用浏览器模式。需要真实节点数据但不想启动全屏壳时传 `provider=true`，由本地 sidecar 读取 `config/private/andx.node.json` 并代理 `/providers/*`；真实壳调试传 `shell=true`。
2. 无真实硬件时传 `mock=true`，禁止把 Mock 结果写成真机证据。需要真实数据时传 `provider=true` 且不能与 `mock=true` 同时使用；密钥只写私有配置，不进入 H5、日志或证据。
3. 读取返回的 `httpUrl`、`cdpUrl`、`bridgeUrl` 和 `sessionFile`。
4. Playwright 使用 `connectOverCDP(cdpUrl)` 连接 WebView2；浏览器模式可直接打开 `httpUrl`。
5. 每个有界变更至少检查：页面入口、console error、网络失败、目标分辨率截图、Provider 可用/离线态。
6. 视频展项检查 Range、播放开始、外部替换配置；实体展项检查输入事件、输出回写和安全停止。
7. UI 变更检查主操作文案、状态反馈、转场是否卡死、页面动态是否仍在运行，以及 reduced-motion 下是否可操作。
8. 调试结束调用 `exhibit.web.dev.stop`，不得遗留 Shell 或 Provider 进程。
9. 调试异常先用 `exhibit.web.logs.read` 查看最近日志，再决定修改。

## 体验门禁

1. 每个有界 UI 变更完成后调用 `exhibit.web.experience.inspect`，传入展项 `project_root`。
2. 返回 `ok=false` 时必须修复 findings 并重跑，不得先构建再补文案或 reduced-motion。
3. 自动检查只覆盖静态信号；仍需在浏览器或真机检查状态动画、转场重复触发、静音/声音、遮挡和长时间运行。
4. 构建前至少保留首屏、处理中、成功、错误、离线和 reduced-motion 状态证据。
5. 体验报告与截图进入 `exhibit.evidence.pack`，DevelopmentCheckpoint 的 `verification.experience` 必须为 `passed`。

详细门禁见 [experience-gate.md](references/experience-gate.md)。
## 多目标构建

1. 页面仍以 `wwwroot/` 为唯一业务源；Vue/React 等框架通过 `exhibit.json` 的 `build.command` 输出静态文件。
2. 三端构建优先调用 `exhibit.web.build` 对应能力；插件暂未暴露 target 时，直接调用 `myriad build <project> --target all --iodevice-root <Binaries/Android/Release>`。
3. `--target all` 必须生成 `web/`、`win-x64/`、`android-arm64/`、`build-manifest.json` 和 `SHA256SUMS.txt`。
4. Android 包必须从同一个 `web/` 构建目录装包，禁止另复制一份页面。
5. Android `.so`、插件配置和展项 `hardware/` 分开管理：可执行载荷来自 IODevice 发布目录，现场配置来自展项工程。
6. Android Debug 测试用 WebView DevTools：

```powershell
adb forward tcp:9222 localabstract:webview_devtools_remote_<pid>
# GET http://127.0.0.1:9222/json/list
```

7. Release APK 不开放 WebView 调试。Android 原生桥事件用 `MyriadBox.on('device.event', handler)`，安全停止用 `MyriadBox.safeStop()`。
8. APK 当前验证范围为 `arm64-v8a` / API 24+；设备插件仍按现场逐项验收，不能因为打包成功就宣称协议已验收。
9. Android WebView 的 Playwright `connectOverCDP` 可能因 `Browser.setDownloadBehavior` 不兼容而失败；此时直接连接 `/json/list` 返回的 page WebSocket，使用 `Runtime.evaluate`、`Network.*` 和 `Page.*` 完成 DOM、Console 与网络验证。
10. 不要在 Android 虚拟显示环境中假设 `Page.captureScreenshot` 可用；不可用时保留 DOM/Console/网络证据，并用同一 H5 的浏览器或 Windows CDP 截图补充视觉证据，明确标注来源。
## 构建与交付

1. Windows 修改后调用 `exhibit.web.build`；默认必须生成项目内 `dist/`。跨平台交付再按上面的多目标流程生成 APK 与 Web 静态包。
2. 调用 `exhibit.web.dist.verify`；失败时修复，不得跳过。
3. 用 `dist/MyriadBox.exe` 从项目外或临时复制目录启动一次，证明不依赖全局 Runtime。
4. 检查 `dist.manifest.json` 中的 Core 版本、Core SHA-256、Builder ID 与文件数。
5. 需要归档时用 `myriad dist archive`；普通展项交付不上传软件分发。
6. Candidate 内容包只作为工作台证据或兼容制品，不替代独立 dist 的可运行验收。

## 证据

1. 截图、console 摘要、体验检查、日志、dist manifest 和测试结果使用 `exhibit.evidence.pack` 生成指纹。
2. 证据 metadata 标记 `provider_mode=mock|real`、`template_id`、`template_version`、Core 版本和 Build ID。
3. 真实按钮、Modbus、动感平台必须由用户确认，不能以 Mock 或自动截图代替。
4. 不得上传原始脚本、聊天记录、本机绝对路径、Token、Cookie 或私钥。

## 禁止

- 不让展项连接软件分发或读取 Core 更新凭据。
- 不把 Core `runtime/` 替换成安装机上的任意全局版本；必须来自模板锁定的 Core。
- 不使用 `file://`、CDN、内网绝对路径或页面内硬编码设备编号。
- 不在同一 hardware 项目同时运行 IOStudio 和 WebBridge。
- 不在未执行体验检查、`dist verify`、未检查 Console、未完成人工硬件确认时宣称交付完成。
- 不删除、放宽或跳过体验检查来让构建通过。

## 输出

- 模板 id/version、展项 pid、规格 revision
- 开发会话用途、CDP/浏览器结果、Mock/真机状态
- 体验检查结果、文案/转场/动态证据与人工待验项
- `dist`/APK/Web 路径、Build ID、Core 版本与 SHA-256、多目标清单和校验结果
- 修改文件、证据清单和未完成/人工验收项

