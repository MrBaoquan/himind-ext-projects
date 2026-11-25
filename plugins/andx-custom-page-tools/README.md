# AndX 定制页工具

项目级确定性插件，把任意静态 H5 目录转换为 AndX Interaction Bridge v1 可部署包。

## 能力

- `andx.page.project.inspect`：检查入口、文件类型、大小、路径和 MyriadBox 运行时边界。
- `andx.page.contract.validate`：校验页面标识、展示模式、Bridge 能力和静态包约束。
- `andx.page.package`：生成不可变 ZIP、`manifest.json` 和 SHA-256。
- `andx.page.test.run`：用真实 `@mrbaoquan/andx` SDK 在 Chrome sandbox iframe 中验证 Bridge。
- `andx.page.deploy.preview`：通过工程绑定与凭证 Broker 上传候选隔离预览、执行健康检查并返回幂等回执。
- `andx.page.deploy.activate`：核对 SHA 和健康状态后激活工程绑定展项版本（旧 Profile 保留项目默认兼容）。
- `andx.page.qr.export`：导出固定 `/x/{publicCode}` 互动二维码 PNG。
- `andx.page.evidence.pack`：汇总候选包、二维码和验收附件为 SHA-256 证据清单。

`andx.page.test.run` 覆盖握手、能力协商、状态读取、支付成功/取消、支付事件、展项状态事件、Bridge 撤销、Console 与运行时异常。测试使用 Node 内置 CDP，不依赖 Playwright 或 npm 安装。

## 构建来源

- 任意框架的静态输出，例如 `dist/`。
- MyriadBox 构建回执指向的静态目录，根下必须有 `index.html`。
- `myriadbox.runtime.json`、`sdk/myriadbox.js` 和 `sdk/myriadbox.d.ts` 会被排除。
- 页面引用 MyriadBox SDK、`myriadbox.runtime.json` 或 `/providers/` 时直接拒绝，避免把 MyriadBox 运行模型带入 AndX sandbox iframe。

插件只处理构建后的静态文件，不运行 Vite、Webpack、npm 或 MyriadBox CLI；MyriadBox 构建由 `com.himind.exhibit-development-tools` 的 `exhibit.web.build(target=web)` 完成。

## 安全边界

- `project_root`、`source_root` 和 `output_zip` 均限制在项目目录内。
- 拒绝符号链接、路径穿越、敏感文件、sourcemap、原生库和未支持扩展名。
- `index.html` 禁止内联脚本、内联事件、`base`、meta refresh 和根绝对资源路径。
- ZIP 限制 50MB，解压后限制 100MB，最多 500 个文件（含 manifest）。
## 默认开发接入

管理员复制开发者接入 JSON → HiMind 对话安全导入 → 读取授权目录 → 绑定工程目标 → 开发测试/打包 → 上传隔离预览 → 审批精确制品 → 激活及固定二维码。

用户无需配置 Profile 或环境变量。Agent 在审批前将当前工程解析为明确的 credential_ref、AndX 项目和展项，调用 Broker 只允许本次用途与凭证实例。插件不接受原始开发密钥。页面发布不依赖设备。

上传回执包含制品 SHA、部署编号、预览 URL 和 `expected_activation_id`。激活必须沿用该修订；并发变化返回冲突，不能静默刷新后覆盖。回滚以历史目的制品摘要为准。

`andx.page.device.enrollment` 仅在单独授予设备接入权限时签发短时一次性码。它与页面发布权限分离，不返回设备长期密钥；一次性码写入 Agent 私有交接文件，普通工具/模型结果仅含 private_path、展项与截止时间，禁止读取并复制到普通对话。

旧版 `profile` 参数显式提供时仍走旧管理 API，仅用于存量兼容，不是新接入流程。

## 工作流回执与自动发现

最低 Agent 0.3.94。Bridge SDK 默认查找工程/父级 node_modules 的 @mrbaoquan/andx，找不到时明确提示安装。工具成功后把精确 JSON、SHA、大小和文件 URI 登记为 Workflow Artifact；`andx.page.receipt.save` 仅接收规格和验收报告，不允许输入伪造发布回执。

MyriadBox Web 回执会触发已知平台启动包装适配，结果写入工程 artifacts/andx-static 的独立内容寻址目录，保留原产物。未知脚本、宿主依赖与越界资源仍失败。

## 通用宿主与 AndX 扩展边界

HiMind 只处理 secure_import / workspace_binding 声明、秘密存储、引用、可信工程与修订。AndX 格式、开发者验证结果、授权目录、创建展项、project/exhibit/pageKey 和发布语义全部在本扩展。验证处理器 credential.import.verify 仅接收宿主移除秘密后的声明文档和受控身份探测响应，不能作为普通模型能力调用。

后台接入信息含通用 importProtocol 与 extensionId；缺少/停用扩展、声明或网络权限不匹配时拒绝导入，原文不保存。宿主的原生 Runtime/ACP/Workbench 路径受控；外部客户端和直接绕过宿主的 Runtime 历史不在保障范围。

业务能力：andx.page.access.directory、andx.page.exhibit.create、andx.page.workspace.bind、andx.page.workspace.resolve。绑定根目录必须位于真实会话工程内，可按子页面独立绑定。发布前固定完整目标与 binding_revision，预览和激活回执保持同一修订；审批后换绑会被拒绝。
