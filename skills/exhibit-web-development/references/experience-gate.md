# 展项体验门禁

## 运行时机

在以下时点调用 `exhibit.web.experience.inspect`：

1. 完成一个页面或主要交互后。
2. 修改按钮、进度、错误、空状态或离线状态后。
3. 新增或替换转场、动态背景、媒体播放后。
4. 构建 Windows/Android/H5 产物前。
5. 交付前作为最终静态门禁。

输入：

```json
{ "project_root": "F:\\Exhibits\\Postcard" }
```

## 结果解释

- `ok=true`：静态规则通过，不代表性能和真机体验已经通过。
- `ok=false`：至少一项硬门禁失败，禁止进入构建或宣称交付完成。
- `checks[].findings`：包含相对路径、行号和命中文本；修复后重跑，不保留旧报告冒充通过。
- `checks[].status=warn`：记录保留理由，并在浏览器或真机补充证据。

核心检查：

| ID | 检查内容 |
| --- | --- |
| `copy.forbidden` | 空泛旅程、宏大口号、装饰英文 Slogan 等套话 |
| `copy.buttons` | 按钮缺少可读动作、只有 GO/START 或没有文字与 aria-label |
| `copy.error-states` | 错误只写“系统异常/操作失败”，没有原因、内容状态或下一步 |
| `motion.animation-events` | 代码依赖 `transitionend`/`animationend` 推进业务 |
| `motion.transition-timing` | CSS 转场超过 800ms，需证明信息价值并补证据 |
| `motion.reduced-motion` | 缺少 `prefers-reduced-motion` 分支 |
| `motion.live-signals` | 缺少至少两类动态信号 |

## 自动检查之外

静态检查无法证明以下项目，必须通过浏览器、Windows Shell 或目标 Android 设备补充：

1. 连续触发主操作 20 次不卡死、不重复提交。
2. 业务状态先推进，画面转场后完成。
3. 每屏至少两类动态且不遮挡主内容。
4. reduced-motion 下动画循环和视差确实停止。
5. 首屏、处理中、成功、失败和离线状态均可读。
6. Console 零未处理错误，网络无非预期失败。
7. 10 分钟运行后计时器、音频、视频和 Provider 轮询没有持续累积。
8. 触摸、声音、视频和实体 Provider 已在目标设备人工确认。

## 证据

将以下内容交给 `exhibit.evidence.pack`：

- `experience.inspect` 的最终 JSON。
- 五类状态截图和 reduced-motion 证据。
- 浏览器/真机 Console 与网络摘要。
- 20 次主操作结果和性能摘要。
- Mock/Real Provider 标记。

DevelopmentCheckpoint 中：

```json
{
  "verification": {
    "compile": "passed",
    "console": "passed",
    "tests": ["experience.inspect: passed", "browser flow: passed"],
    "visual": ["idle", "processing", "success", "error", "offline", "reduced-motion"],
    "experience": "passed"
  }
}
```