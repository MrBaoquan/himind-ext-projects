---
name: andx-custom-page-development
description: 开发、测试和交付 AndX Interaction Bridge 定制页；支持任意静态 H5、MyriadBox Web 产物、预览部署和二维码验收。
---

# AndX 定制页开发助手

## 目标

把已确认的展项需求实现为可部署的 AndX 定制页静态 H5，并完成 Bridge 契约测试、打包、预览部署、人工验收、激活和二维码证据闭环。

AndX 定制页运行在宿主页面的 sandbox iframe 中，只使用 `@mrbaoquan/andx` 的 `createCustomPageBridge`。它不直接访问 AndX API、Cookie、OpenID、支付签名、设备命令或 MyriadBox `/providers`。

必需的 `com.himind.andx-custom-page-tools` 未安装或 Capability 不可用时，停止自动执行并提示安装插件；可以继续做只读需求分析和方案设计，但不得声称已完成测试或部署。

## 开发接入（无需手工配置）

1. 管理员在 AndX 后台创建开发者、授予项目（含未来展项）或指定展项的开发/发布权限，复制个人开发接入 JSON。
2. 用户将接入 JSON 粘贴到支持安全导入的 HiMind 展项对话；本机通用安全导入通道在持久化前移除秘密，AndX 扩展验证远程身份及业务权限，宿主加密保管。AI 只收到 `credential_ref`。不要要求用户填写 Profile、环境变量或凭证中心。
3. 使用 `andx.page.access.directory` 列授权项目；指定 AndX 项目编号后列展项。多选时只问一次目标，不从工作台 pid 推断。
4. 使用 `andx.page.workspace.bind` 绑定当前工程；确认真实会话工程及页面根目录，唯一授权目标可直接调用绑定；已有绑定不能静默覆盖。同仓多页面分别绑定各页面根目录，不能使用全局当前展项。每次交付用 `andx.page.workspace.resolve` 读取快照。
5. 新建展项使用 `andx.page.exhibit.create`（需独立创建权限），不需要预先注册设备；发布页面也不依赖在线设备。
6. 外部 AI 客户端若不能在持久化之前拦截，不能保证其自身历史不留密钥。通过 HiMind 安全导入后把无密钥引用交给外部客户端；宿主管控的网关会拒绝声明可识别的残留凭证，不能保证识别任意秘密或编码。
7. 现场设备另用 `andx.page.device.enrollment` 或后台“接入一台设备”签发一次性码，可信宿主换取并保存自己的设备密钥。不能把开发密钥放进设备或网页。

## 先选构建提供方

### MyriadBox 工程

1. 使用已安装的 `com.himind.exhibit-development-tools`。
2. 调用 `exhibit.web.build`，参数为 `target: "web"`。
3. 以构建回执实际返回的静态目录作为 `source_root`，检查其根下有 `index.html`；不猜 `wwwroot` 层级。
4. 不要把 MyriadBox 的 `dist`、APK、`myriadbox.runtime.json`、`sdk/myriadbox.js` 或 Provider 放进 AndX 包。
5. 页面业务代码不得依赖 `window.MyriadBox` 或 `/providers/`。

### 任意框架或已有静态产物

1. 使用项目现有构建方式产出 `dist/`。
2. 确认根目录包含 `index.html`，资源使用相对路径。
3. 媒体、字体和其他资源必须位于 `dist/` 内并被静态打包。
4. 不额外创建第二套 Vite/Webpack 构建器；本技能只消费构建产物。

完整边界见 [build-provider.md](references/build-provider.md)。

## 必须确认的输入

- AndX `project_id` 和可选 `exhibit_id`。
- `page_key`：2-64 位小写字母、数字或连字符。
- `version`：每次构建不可复用。
- `display_mode`：`embedded` 或 `immersive`。
- 最小 Bridge 能力：`required_capabilities` 和 `optional_capabilities`。
- 构建来源、静态目录、目标分辨率、业务状态和验收标准。

Bridge 能力、握手和安全边界见 [bridge-contract.md](references/bridge-contract.md)。

## 开发规则

1. 只使用 Interaction Bridge v1，不修改协议版本和消息名称。
2. 支付请求必须通过宿主代理；页面不得持有金额覆盖、商户号、OpenID 或签名参数。
3. 状态映射统一使用 `IDLE / STARTING / PLAYING / OFFLINE / ERROR`。
4. 首屏必须不依赖大视频加载；媒体优先提供海报和失败兜底。
5. 所有资源使用相对路径；禁止 `<base>`、meta refresh、内联脚本和内联事件。
6. 页面不得直接请求 AndX API；禁止引用 `/providers/`、`window.MyriadBox` 或 `myriadbox.runtime.json`。
7. 移动端至少检查 390×844、430×932 和安全区；`immersive` 模式必须验证全屏铺满。
8. 文案应说明当前状态、失败原因和可执行的下一步，不使用空泛口号。

## 测试与验真

1. `andx.page.project.inspect`：检查静态目录、入口、文件类型、大小和 MyriadBox 运行时边界。
2. `andx.page.contract.validate`：检查 page key、版本、展示模式和 Bridge 能力。
3. `andx.page.test.run`：使用真实 `@mrbaoquan/andx` SDK 运行 sandbox iframe Bridge 夹具。
4. 可选使用 `exhibit.web.experience.inspect` 检查文案、动态、`prefers-reduced-motion` 和转场规则。
5. 在真实 `/x/{publicCode}` 入口完成 Console、Network、截图和微信双入口验收。

`andx.page.test.run` 证明协议和 SDK 契约，不替代页面业务 UI、真实支付、微信环境或现场体验验收。

## 打包与预览

1. `andx.page.package` 生成不可变 ZIP、`manifest.json` 和 SHA-256；动作白名单通过 `page_actions` 写入 manifest。
2. `andx.page.deploy.preview` 根据工程绑定，通过短期调用 Broker 使用个人开发凭证上传候选；不传原始密钥。
3. 读取回执的 `preview.previewUrl` 打开真实 Bridge 隔离预览，支付、内容提交和现场动作停用。此时 `/x/` 仍为现有线上版本。
4. 上传前必须核对现有版本的 SHA-256；相同版本但 SHA 不同直接阻断。
5. 健康检查失败时不得进入人工生产审批。

部署参数和安全规则见 [delivery-gates.md](references/delivery-gates.md)。

## 生产激活与二维码

1. 预览上传、Bridge 契约测试和隔离候选页面验收通过后，才进入生产审批。
2. 激活必须由人工明确批准，并提交预览回执中的部署编号、SHA-256 和 `expected_activation_id`。出现并发版本变化时停止，不刷新修订后静默重试。
3. 现场二维码固定编码 `/x/{publicCode}`，不得编码静态版本 URL。
4. 激活后验证当前生效版本、静态资源、宿主入口和二维码，并保留回执。
5. 回滚只按 AndX 精确激活历史执行，不猜测目标版本。

## 失败处理

先按 [troubleshooting.md](references/troubleshooting.md) 分类，不要反复重试同一动作：

- 静态资源 404：检查相对路径和包根目录。
- Bridge 握手失败：检查 sandbox、`parentOrigin`、frame source 和协议版本。
- 能力缺失：检查 required/optional 声明和宿主授权。
- 支付无结果：等待宿主最终态，不以客户端回调代替服务端状态。
- 上传冲突：核对 page key、version 和 SHA，不覆盖不可变版本。
- 健康检查失败：查看部署 URL、Content-Type 和 Nginx/MinIO 链路。

## 完成标准

- 静态契约、Bridge 测试、打包和预览健康检查全部通过。
- 真实 `/x/` 页面证据包含截图、Console、Network、制品 SHA 和当前版本。
- 人工确认微信双入口与二维码。
- 生产激活有审批、精确回执和可回滚目标。
设备接入码是 bearer 凭证。设备接入能力只向普通工具结果返回私有交接路径；不要读文件并粘贴到对话，交由可信设备部署流程消费，使用后清理。工作流默认 preview_ready 出口，生产需明确选择发布出口。

## 当前接入版本

本流程最低 HiMind Agent 0.3.94、AndX 定制页工具 0.2.0。授权展项目录按 `nextCursor` 继续查询，向目录能力传入 `after_id`，不要把第一页当作全部展项。原生 AI 页面在粘贴阶段本机导入，随后按当前任务工程明确绑定；不使用共享全局展项。

后台复制的完整 JSON 应保留 `importProtocol: "himind.credential-import.v1"` 和 `extensionId`。工作台仅识别这个通用信封；普通云端聊天不调用本机导入、不新增 Agent 在线依赖。信封导入遇离线、扩展缺失、授权或验证失败时不保存、不发送原文，只显示通用错误；成功仅持久化脱敏回执。本机 Agent 保管秘密，AndX 扩展解释业务，工作台服务端只拒收未处理信封。任意自由文本密钥、未支持编码/附件、外部客户端历史和直连 Runtime 不在完整保护承诺内。

MyriadBox 构建的已知启动包装由检查工具写入独立内容寻址适配目录，原始构建不变；仅移除平台生成的启动包装。业务仍依赖宿主、父目录资源或未知内联脚本时拒绝交付，需要显式移植。SDK 默认从工程及父级 node_modules 查找。Workflow 统一开发入口，构建来源只选一次，默认止于预览；发布说明有默认值。

确定性工具自动登记候选/Bridge/部署/证据回执；规格与候选验收报告经 andx.page.receipt.save 登记，发布回执只能来自真实 API。
