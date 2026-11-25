# Web、MyriadBox 与定制页

> 状态：active ｜ 最后更新：2026-11-24 ｜ 负责人：AndX 维护者

## 四类页面

| 类型 | 路线 | 身份边界 |
| --- | --- | --- |
| 公开扫码页 | createPublicAndX | 公开 token 授权范围 |
| 展项侧浏览器 | createAndX＋同源 BFF | HttpOnly 会话＋CSRF，服务端保管密钥 |
| MyriadBox 页面 | 实际宿主 SDK/Provider/WebBridge | 宿主保管节点凭据，按真实能力适配 |
| AndX 定制 iframe | createCustomPageBridge | 受限 Bridge，复用定制页技能 |

先读锁文件与导出类型。BFF HTTP 客户端不能直接指向任意 MyriadBox Provider 地址，必须验证路由、鉴权与响应形状或复用已有适配。

## 普通 Web

沿用工程构建器。节点会话由可信服务端创建，bootstrap key 不进入前端环境变量、页面、localStorage、源码映射或日志。检查节点 allowlist、Cookie 范围、CSRF 与同源部署。

按安装版本调用，先跑“收图创建→二维码→状态→展示”，覆盖过期/取消/未授权和卸载后停止请求。RxJS 可选，项目未使用则不额外引入。

支付签票/查询不等于设备已启动；由权威履约运行时确认，前端金额或 exhibitId 不覆盖服务端授权。

## MyriadBox

读取 exhibit.json、模板、运行时注入与实际 SDK，配合 `exhibit-web-development` 复用模板、Provider、CDP 和构建工具。私有节点配置只由可信进程读取，证据标注 provider_mode=mock 或 real。

普通浏览器不会天然拥有宿主桥。真实节点用受支持 Provider 开发会话，原生桥用真实壳验证；缺能力显示不可用，不静默切 Mock。

Windows/Android 复用同一 H5 源；宿主注入、插件 ABI 与配置分别验证。WebView2 不代替 Android WebView/JNI，Debug 结果不代替 Release 包。设备由宿主持有，退出/暂停/断线仍执行本地安全策略。

UI 改动复用体验门禁；仅 API 适配验证相关状态与错误。构建前发现实际工具 Schema，产物路径从返回值/manifest 获取，不猜目录。

## 定制页交接

AndX sandbox iframe 转给 `andx-custom-page-development`：静态来源、page key、版本、display mode、Bridge 能力和稳定项目/展项 ID；缺项在对应阶段补齐。

定制页不访问 `/providers/`、window.MyriadBox、任意 AndX API 或支付签名。复用 MyriadBox Web 包时先移除宿主 API 依赖，改打包目录不等于完成移植。sandbox/origin/source、支付代理、预览、激活和二维码沿用现有交付技能。
