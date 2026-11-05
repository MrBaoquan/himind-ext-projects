---
name: exhibit-development
description: 把展项脚本转换成可追踪开发规格，并在 UNIHPer Unity 工程中按证据驱动完成实现、验证与工作台同步。用户提到展项脚本、展项开发、开发规格、开发截图、成品演示或展项交付证据时使用。
---

# 展项开发助手

## 目标

把展项脚本变成可开发、可验证、可追踪的工程任务，并在不破坏 UNIHPer 生命周期和项目边界的前提下完成实现与证据同步。

工作台是项目、展项介绍、开发规格和证据的业务事实源；SVN 是工程源码事实源；Agent 只执行本机操作。原始 DOC/DOCX 脚本只在本地解析，不上传工作台。展项业务参数必须先解析为稳定的 `pid`，不能使用 `EX-xxxx` 展示编号冒充路由 ID。

## 必用同伴

1. 涉及 Unity C#、场景、Prefab、资源、UPM 或 PlayMode 时，必须同时使用官方 `unihper-unity-development` Skill。本项目 Skill 只补充展项语义，不复制其生命周期、UI、资源和 MCP 规则；MCP 就绪门禁以该 Skill 十三、13.1 为准。
2. 确定性脚本解析、规格校验、工程检查和证据清单使用 `com.himind.exhibit-development-tools`。
3. 工作台脚本、规格和证据读写使用内置 Workbench Provider，不自行拼接 Dashboard HTTP。
4. VCS、跑测、截图和构建必须通过稳定 Capability 或项目既有命令，不猜测路径和凭据。

## 能力发现

Agent 默认只暴露通用 Bootstrap 能力，业务能力需要先激活才能调用。固定顺序：

```text
system.capability_search(query)  → 拿到能力 id 与 availability
system.capability_activate(ids)  → 激活到当前会话
system.capability_invoke(id, args)
```

常见展项能力：

| capability_id | 必填参数 | 说明 |
| --- | --- | --- |
| `exhibit.script.parse` | `script_path` | 本地解析，旧版 .doc 返回明确阻塞 |
| `exhibit.script.spec.validate` | `content_json` 或 `spec_path` | 发布规格前必调 |
| `exhibit.project.inspect` | `project_root` | 读 Unity 版本、UNIHPer 配置、程序集与 **MCP 就绪状态**（`mcp` 段） |
| `exhibit.evidence.pack` | `workspace_root` + `items[]` | 生成带 SHA-256 的证据清单 |
| `workbench.context.resolve` | `query` | 用展项名换稳定 `pid` 的唯一正路 |
| `workbench.exhibit.artifact.upload` | `exhibit_id`、`local_path`、`request_key`、`artifact_kind` | `request_key` 是幂等键 |
| `workbench.exhibit.artifact.list` | — | 上传后回读确认 |

搜索为空不代表能力不存在，先换更短的查询词（展项能力统一以 `exhibit` 开头）再判阻塞。

## 开始前

0. **先确认 MCP 就绪**，这是自动化验证的前提，不通过则不得宣称完成：
   - 最省事的方式是调用 `exhibit.project.inspect(project_root)`，读它返回的 `mcp` 段；
   - `mcp.state` 为 `ready` 且 `version_match` 为 `true` 才算可用；
   - `not_running` 说明工程没开：按 `ProjectSettings/ProjectVersion.txt` 的 `m_EditorVersion` 启动对应 Unity，再重新 inspect 直到 `ready`；
   - `reloading` 等它变 `ready`，不要在此期间调用工具；
   - `multiple_instances` 或 `notes` 提示其它工程在跑时，先用 `set_active_instance` 显式指定目标，避免写入落到错误工程；
   - `version_mismatch` 必须先解决引擎版本，拿错版本会触发不可逆的工程升级。

   完整判定与手工兜底（直接读 `%USERPROFILE%/.unity-mcp/unity-mcp-status-*.json`）见 `unihper-unity-development` 13.1。
1. 读取工作台展项上下文、当前 Markdown 展项介绍和当前开发规格。
2. 读取展项工作区绑定；没有绑定时通过工作台创建/检出流程获取，不手动猜测 SVN 地址。
3. 检查目标 Unity 工程：Unity 版本、UNIHPer 配置、程序集注册、场景、资源配置和自动化通道。
4. 读取项目 `.himind/exhibit-profile.json`（如存在）；项目 Profile 优先于本 Skill 的默认假设。
5. 记录 `exhibit_pid`、脚本版本、规格修订、SVN revision、工作区路径和当前 Agent ID。

## 脚本到规格

1. 在本机调用脚本解析能力，得到段落、表格、图片占位和文件哈希；原脚本不得上传工作台。
2. 基于本地解析结果生成可编辑 Markdown 展项介绍，并通过工作台介绍能力写回；介绍只记录源文件名、SHA-256、模型和提示版本，不记录本机绝对路径。
3. 识别展项元数据、硬件、时长、媒体类型、视频段落、互动状态、资源、配音、字幕、音效和验收条件。
4. 每条需求必须包含：
   - 稳定 `requirement_id`
   - 可验证描述
   - `source_ref`，指向脚本段落或表格位置
   - `acceptance`
   - 所需 `evidence`
5. 不把不确定内容补成业务事实。无法确认时写入 `unknowns`，等待人工确认。
6. 发布规格前调用规格校验能力；拒绝空需求、无来源需求和不可验证需求。
7. 规格版本不可覆盖。脚本或需求变化时创建新修订并显示差异。

详细结构见 [script-to-spec.md](references/script-to-spec.md)。

## 开发

1. 先确认规格已由人工发布，再开始代码修改。
2. 获取工作区写 Lease，明确本轮只处理一个需求或一个紧密相关模块。
3. 按 UNIHPer 规则选择实现位置：场景脚本、UIBase/UIPage、UIToolkitBase/UIToolkitPage、UConfig、资源组或服务对象。
4. 修改前记录文件、场景、Prefab 和资源范围；不得修改工程范围外文件。
5. 一次完成一个有界变更，然后立即编译、读取 Console、运行最小测试。
6. UI、场景或交互变化至少保留一张目标分辨率截图；关键交互和成品演示保留视频或明确未录制原因。
7. 测试失败、Console 异常、资源缺失或规格歧义会阻断完成，不以“已经写完代码”代替验收。
8. 生成 DevelopmentCheckpoint，并关联规格修订、SVN revision、通过的需求和证据。

详细循环见 [development-loop.md](references/development-loop.md)。

## 证据与完成

1. 截图、视频、测试报告和构建包必须经证据清单能力计算大小和 SHA-256。
2. 通过 `workbench.exhibit.artifact.upload` 上传到工作台，并回读列表确认可检索。
3. 每个 Artifact 关联 `checkpoint_id`、`spec_revision`、`requirement_ids` 和必要 metadata。
4. 交付前逐条核对需求矩阵；人工验收项不得自判完成。
5. 只有需求覆盖、测试、截图/视频、Console 和工作台回读全部有证据时才报告完成。

详细门禁见 [evidence-and-acceptance.md](references/evidence-and-acceptance.md)。

## 输出

执行结束时返回：

- 展项 `pid`、脚本版本、规格修订、SVN revision
- 本轮完成的需求和未完成的需求
- 修改文件与工作区路径
- 编译、测试、Console 和视觉证据
- 已上传的 Artifact ID、SHA-256 和回读结果
- 阻塞、未覆盖项和下一轮建议

## 禁止

- 不把原始 DOC/DOCX 上传到工作台，也不用工作台脚本版本替代本地解析。
- 不使用脚本中的自然语言直接替代结构化规格。
- 不猜测 UNIHPer 资源键、程序集、UI 类型、场景名或构建入口。
- 不直接调用工作台 HTTP、绕过 Provider 鉴权或复制工作台 CRUD。
- 不把凭据、Token、Cookie 或本机绝对路径写入工程、日志或 Artifact metadata。
- 不在没有证据时宣称完成或交付。
- 不提交、推送、发布或外传，除非用户另行明确授权。
