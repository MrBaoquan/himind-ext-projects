# 展项 Web 开发

把已确认的展项规格实现为 MyriadBox 独立 `dist`/Android APK，通过 AI/CDP 调试、多目标构建、APK 校验、设备验证和证据回读形成可验收闭环。

## 核心边界

- MyriadBox Core 由组织软件分发更新。
- 展项不接软件分发，交付物是包含固定 Core、`MyriadBox.exe` 和内容的独立文件夹。
- Candidate/内容包用于工作台证据，不替代 `dist` 的实际可运行验收。

## 步骤

| 步骤 | 类型 | 说明 |
| --- | --- | --- |
| EXW-PROJECT-INSPECT | capability | 检查工程、框架、配置和 Provider |
| EXW-DEVELOP | runtime | 选择/创建模板，启动 shell+mock 调试会话，Playwright/CDP 检查 |
| EXW-EXPERIENCE-INSPECT | capability | 检查文案、按钮、转场事件、reduced-motion 与动态信号 |
| EXW-BUILD | capability | 构建 Web、Windows dist 与 Android APK |
| EXW-DIST-VERIFY | capability | 校验启动器、固定 Core 和 manifest，禁止分发元数据 |
| EXW-APK-VERIFY | capability | 校验 Android arm64 APK 载荷、包信息和签名 |
| EXW-PACKAGE | capability | 兼容内容包与 Candidate 证据 |
| EXW-SMOKE | capability | WebBridge HTTP/WS/写入/安全停止 |
| EXW-EVIDENCE-PACK | capability | 生成证据指纹 |
| EXW-REVIEW | manual | 用户确认真实硬件和现场体验 |

## 输入

- `workspace_root`：展项工程目录。
- `cli_path`：MyriadBox `myriad.exe`。
- `template_id`、`project_name`：可选的模板创建信息。
- `bridge_path`、`smoke_script_path`：实体互动的冒烟工具。
- `package_output_dir`：可选兼容内容包输出目录。

## 验收

- `dist/MyriadBox.exe` 可从非源码目录启动。
- `dist` 含固定 Core 和 `dist.manifest.json`。
- `dist` 不含 productId、分发地址、凭据或更新器。
- AI 调试产生 console、截图和 Provider 模式证据。
- `exhibit.web.experience.inspect` 在构建前通过；`ok=false` 时禁止构建与交付。
- 真实设备与动感平台未人工确认时不得标记完成。

## 多目标与 Android

- `target=all` 时统一产出 `web/`、`win-x64/`、`android-arm64/`、`build-manifest.json` 和 `SHA256SUMS.txt`。
- Android Debug 流程：构建 Debug APK → `exhibit.web.apk.verify` → 安装 → 启动 → CDP → 日志。
- Release APK 必须移除 `android_webview_debug` 后重建，并确认没有开放 WebView 调试端口。
- Android 虚拟显示无法截图时，保留 DOM、Console、网络和设备日志证据，视觉截图注明来自同一 H5 的浏览器或 Windows 目标。