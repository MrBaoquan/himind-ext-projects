# 展项开发工具

项目级确定性插件，服务 Unity 与 H5 展项开发流程。H5 能力以 MyriadBox CLI 为唯一执行契约。

## 能力

- `exhibit.script.parse`：解析 `.docx`、`.txt`、`.md`、`.json` 脚本。
- `exhibit.script.spec.validate`：校验开发规格结构。
- `exhibit.project.inspect`：识别 Unity/UNIHPer 工程。
- `exhibit.web.project.inspect`：检查 H5 manifest、框架、配置、媒体与 Provider。
- `exhibit.web.experience.inspect`：检查禁用套话、按钮文案、动画事件依赖、reduced-motion 与动态信号。
- `exhibit.web.template.list|inspect|create`：读取和实例化版本化开始模板。
- `exhibit.web.dev.start|status|stop`：管理 MyriadBox AI 调试会话和 WebView2 CDP；`provider=true` 可在浏览器模式挂载真实 Provider。
- `exhibit.web.build`：构建 Web、Windows dist 与 Android arm64 APK，支持统一多目标输出。
- `exhibit.web.dist.verify`：校验启动器、固定 Core、内容和 manifest，拒绝分发元数据。
- `exhibit.web.apk.verify`：校验 Android arm64 APK 的展项载荷、IODevice `.so`、包信息与签名。
- `exhibit.web.android.install|launch|logs|cdp`：安装、启动、读取日志并转发 Android Debug WebView CDP。
- `exhibit.web.logs.read`：读取最近展项日志。
- `exhibit.runtime.smoke`：验证 WebBridge HTTP/WebSocket、写入和安全停止。
- `exhibit.evidence.pack`：计算证据大小和 SHA-256。

插件不访问工作台、不管理 SVN、不执行任意 Shell。所有本地命令都通过结构化输入和显式 CLI 路径执行；Core 软件发布由官方 `software-distribution` 能力负责，展项不接分发。
## 多目标构建示例

```json
{
  "project_root": "F:/Exhibits/SpaceButton",
  "cli_path": "F:/Tools/MyriadBox/myriad.exe",
  "target": "all",
  "output_dir": "F:/Exhibits/SpaceButton/artifacts/final",
  "core_path": "F:/Tools/MyriadBox/core",
  "launcher_path": "F:/Tools/MyriadBox/MyriadBox.exe",
  "iodevice_root": "O:/DevModules/IODevice/IODevice/Binaries/Android/Release"
}
```

Android Debug 验证链：

```text
exhibit.web.build(target=android-arm64, android_webview_debug=true)
→ exhibit.web.apk.verify
→ exhibit.web.android.install
→ exhibit.web.android.launch
→ exhibit.web.android.cdp
→ exhibit.web.android.logs
```

Release APK 必须重新构建且不设置 `android_webview_debug`。