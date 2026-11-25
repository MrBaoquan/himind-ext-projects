# Interaction Bridge v1 开发边界

- 协议版本固定为 `1.0`。
- 定制页通过 `createCustomPageBridge` 接入，不手写不可控的 `postMessage`。
- iframe 必须使用 `sandbox="allow-scripts"`，不能增加 `allow-same-origin`。
- 页面只接收宿主授权的能力：
  - `andx\u002esession.read`
  - `andx\u002estatus.read`
  - `andx\u002epayment.request`
  - `andx\u002epayment.events`
  - `andx\u002elifecycle.events`
  - `andx\u002evisibility.events`
- 页面不能直接访问 AndX API、Cookie、CSRF、OpenID、支付签名或设备协议。
- 支付金额以 ready 上下文和服务端会话为准；页面不得传入金额覆盖。
- 状态必须映射到 `IDLE / STARTING / PLAYING / OFFLINE / ERROR`，并处理 `SESSION_EXPIRED` 和 `bridge.revoked`。
- 用户取消支付后恢复可操作状态；支付成功后等待宿主最终态，不以客户端回调代替服务端订单状态。
协议标识中的 `\u002e` 表示 JSON 的点号转义，解析后仍是原始 Bridge 协议字符串；这些是 iframe SDK 的协议权限，不是 Agent 工具依赖。
