# 转场与持续动态

## 状态模型

每个互动场景至少维护两类状态：

- `businessState`：业务是否已提交、成功、失败或可重试。
- `displayState`：当前画面呈现到哪一步。

业务状态必须立即更新；显示状态通过显式计时器推进。动画事件只能增强画面，不能决定流程是否完成。

```ts
function commitAction() {
  if (businessState !== 'ready') return
  businessState = 'submitting'
  submit()

  setTimeout(() => {
    displayState = 'processing'
  }, 120)
}
```

不要这样写：

```ts
element.addEventListener('transitionend', finishOrder)
```

原因：低端 WebView、后台标签页、reduced-motion 或样式覆盖都可能不触发动画结束事件，导致流程永久停住。

## 时序预算

| 动作 | 建议时长 | 目的 |
| --- | --- | --- |
| 按压反馈 | 80–140ms | 证明触摸已被接收 |
| 内容替换 | 220–360ms | 保持空间连续 |
| 场景切换 | 360–600ms | 表示阶段变化 |
| 错误出现 | 0–120ms | 优先恢复可操作性 |
| reduced-motion | 0–100ms | 保留状态变化，不制造位移 |

一次操作链路中的装饰动画总时长超过 800ms 时，必须说明它传达了哪项必要信息。

## 常用转场

### 内容替换

- 旧内容淡出并轻微位移，新内容从相反方向进入。
- 先锁定业务状态，再替换内容。
- 容器尺寸保持稳定，避免布局跳动。

### 遮罩过渡

- 使用单色或主题材质遮罩覆盖，再切换底层内容。
- 遮罩动画只用 `opacity` 和 `transform`。
- 错误状态不排队使用遮罩。

### 结果显现

- 先生成可辨认的缩略结果，再补细节。
- 结果出现时同时提供保存、重试或离场动作。
- 不让游客看一段没有进度信息的“庆祝动画”。

### 返回修改

- 保留已经输入的内容。
- 使用与进入相反的空间方向，帮助游客建立位置感。
- 返回后焦点回到上次操作的控件。

## 持续动态的三种职责

### 环境动态

用于建立空间感，如缓慢漂移的光、轨道、材质高光、背景层次。它不能遮挡主体，也不能暗示可点击。

### 状态动态

用于表达系统真实状态：上传百分比、生成阶段、连接重试、队列位置、设备就绪。没有真实状态时不要伪造进度。

### 交互反馈

用于确认操作：按压、选中、拖动惯性、输入范围、确认脉冲。反馈必须在 140ms 内出现。

每屏至少覆盖两类；至少一类必须帮助游客完成任务。待机页可以强化环境动态，但主入口必须清楚。

## reduced-motion

必须同时处理 CSS 与脚本：

```css
@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after {
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
    scroll-behavior: auto !important;
    transition-duration: 0.01ms !important;
  }
}
```

脚本中的旋转、视差、自动镜头和环境循环也要停止，不只是缩短 CSS。状态切换仍要可见。

## Android WebView 注意

- 避免大面积 `filter: blur()`、`backdrop-filter`、复杂遮罩和多层半透明叠加大面积移动。
- 不使用依赖 `transitionend` 的页面切换。
- 不在不可见页面继续 `requestAnimationFrame`。
- 视频与大图加载时降低环境动画复杂度。
- 使用真实目标机测试触摸、GPU 内存和长时间运行，而不是只看桌面浏览器。

## 动态审查

- 动画是否解释了状态变化。
- 删除该动画后，游客是否仍知道发生了什么；若答案是否定，应补状态信息而不是加特效。
- 环境动态是否始终低于内容对比度。
- 页面隐藏、离线和 reduced-motion 时是否停止高成本循环。
- 连续操作是否清理旧计时器和动画帧。