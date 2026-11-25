# 常见问题

## 打包失败

- `index.html` 不在 source root：MyriadBox 使用 `web/wwwroot`，不要传 `web/`。
- 根绝对资源：把 `/assets/...` 改为 `./assets/...`。
- 内联脚本或事件：改为外部脚本和 `addEventListener`。
- 引用了 MyriadBox SDK：AndX 定制页只能使用 `@mrbaoquan/andx`。
- 文件过大或数量超限：检查媒体、sourcemap、node_modules 和构建残留。

## Bridge 测试失败

- 握手超时：检查 sandbox、parentOrigin 和协议版本。
- 能力缺失：required/optional 声明与宿主授权不一致。
- 状态读取失败：宿主未授权 `andx\u002estatus.read`。
- 支付失败：检查宿主支付上下文、金额一致性和最终态查询。
- 撤销未触发：宿主生命周期事件或测试夹具时序问题。

## 部署失败

- Profile 缺失：配置 `ANDX_PAGE_PROFILE_<NAME>`。
- 401/403：检查项目级部署身份，不要使用明文 Token 参数。
- 409：相同 page key/version 已存在，核对 SHA；禁止覆盖不可变版本。
- 健康检查失败：检查静态版本 URL、Content-Type、Nginx 和 MinIO。