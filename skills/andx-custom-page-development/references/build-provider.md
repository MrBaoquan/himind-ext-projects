# 构建提供方边界

## MyriadBox Provider

```powershell
exhibit.web.build(
  project_root = $workspace,
  cli_path = $myriadCli,
  target = "web",
  output_dir = "$workspace/artifacts/andx-web"
)
```

以 `exhibit.web.build` 实际回执的静态目录为准，并检查根目录 `index.html`；不同版本可能采用不同层级，不能猜路径。

不得进入 AndX 包：

- `myriadbox.runtime.json`
- `sdk/myriadbox.js`
- `sdk/myriadbox.d.ts`
- Windows/Android 产物
- `providers/`、`hardware/`、`config/private/`

不得在页面中使用：

- `window.MyriadBox`
- `/providers/`
- MyriadBox WebBridge

## 任意框架 Provider

- 接受 `dist/`、`build/` 等已构建静态目录。
- 根目录必须有 `index.html`。
- Vite 等项目使用相对 `base`，避免部署到 `/interaction-pages/...` 时 404。
- 构建工具和依赖由项目自己维护；本扩展不实现第二套前端构建链。