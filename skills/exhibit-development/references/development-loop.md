# 开发循环

## 单轮步骤

1. 选择规格中的一个需求或一个紧密相关模块。
2. 读取相关 UNIHPer 源码、场景和资源配置，确认修改位置。
3. 获取工作区写 Lease，记录起始 SVN revision。
4. 做最小可回滚修改；保留 `.meta` 和 GUID。
5. 刷新 Unity、编译并读取 Console。存在编译错误或新增异常立即修复。
6. 运行最小 EditMode/PlayMode 测试；UI 和交互变化截图。
7. 对照需求 acceptance，不满足则继续本轮或拆下一轮。
8. 释放 Lease，生成 Checkpoint 和证据。

## 第 0 步：确认 MCP 就绪

每轮开始前先确认自动化通道可用，否则后面的第 5–6 步都无法真正执行：

1. 读 `%USERPROFILE%\.unity-mcp\unity-mcp-status-*.json`；
2. 确认存在指向当前工程的记录、`reloading: false`；
3. 比对 `unity_version` 与工程 `ProjectSettings/ProjectVersion.txt`；
4. 工程未打开时，按 `ProjectVersion.txt` 的版本启动对应 Unity，等心跳就绪后再继续。

完整判定规则见 `unihper-unity-development` 十三、13.1。MCP 不可用而继续改代码，等于放弃验证环节。

## 单轮改动边界

- 一轮只处理一个需求或一个紧密相关模块，不夹带无关重构。
- 修改前记录文件、场景、Prefab 和资源范围。
- 不修改工程范围外的文件，不动 `Library`、`Temp`、`Logs` 和包缓存。

## 验证顺序

固定按以下顺序推进，前一步不过不进入下一步：

1. 编译（`refresh_unity` + `read_console`）
2. EditMode 测试
3. PlayMode 测试（必须走真实 UI 事件链，不能直接调业务方法）
4. Console 复查（无新增 error / exception）
5. 截图（UI 或交互变化）

测试在 PlayMode 之前失败时，先修好再继续，不要"先跑通 PlayMode 再看看"。

## 必须记录的现象

以下几类现象容易被误判为"工具故障"而跳过验证，实际都有明确处理方式，必须记录现象而不是静默重试：

- 连接类报错（`No Unity Editor instances found`、`Request timed out`）：先分类再重试；
- 测试框架自身抛出的异常（例如 `PlaymodeLauncher` 在通过 API 停止 PlayMode 时的 `NullReferenceException`）：属于包内问题，与展项代码无关，要写明并排除在回归之外；
- Unity 在 `Assets/` 留下的 `InitTestScene*.unity`：收尾清理。

## 停止条件

- 规格歧义或缺少业务事实：停止并写入 unknowns。
- 目标工程 API 与官方 Skill 示例不一致：以工程源码为准，先核对再改。
- 资源、场景或工具链缺失：返回结构化阻塞，不假装自动化成功。
- 需要迁移、删除、提交、发布或工作区外写入：请求用户确认。

## 结果

每轮必须留下：改动范围、验证命令、Console 结论、需求覆盖、证据路径和下一轮目标。
