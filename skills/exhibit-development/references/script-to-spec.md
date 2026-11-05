# 本地脚本到介绍与开发规格

原始 DOC/DOCX 只在本地解析。先根据解析结果生成可编辑 Markdown 展项介绍并写回工作台，再生成结构化开发规格；工作台不保存脚本文件。

## 输出契约

```json
{
  "schema_version": "exhibit-development-spec.v1",
  "exhibit_pid": "stable-pid",
  "source_script": {"script_id": "...", "version": 1, "sha256": "..."},
  "metadata": {"name": "...", "hall": "...", "duration_seconds": 180, "hardware": []},
  "video_segments": [{"segment_id": "video-01", "visual": "", "voiceover": "", "subtitle": "", "audio": "", "source_ref": ""}],
  "interaction_states": [{"state_id": "", "entry": "", "actions": [], "completion_rule": "", "source_ref": ""}],
  "requirements": [{"requirement_id": "REQ-001", "description": "", "acceptance": [], "evidence": [], "source_ref": ""}],
  "unknowns": []
}
```

`requirements` 每条都是必填五项：`requirement_id`、`description`、`source_ref`、`acceptance`、`evidence`。缺任一项，`exhibit.script.spec.validate` 会拒绝。

## 来源引用

`source_ref` 必须指向真实来源，不能写"脚本中提到"这类模糊表述：

| 来源类型 | 写法 | 使用场景 |
| --- | --- | --- |
| 文档段落 | `docx:段落-03` / `docx:video/row:05` | 来自 DOC/DOCX 解析结果 |
| 表格单元格 | `docx:table-1/row:互动界面/陶器层` | 来自脚本表格 |
| 工作台事实 | `workbench:exhibit-resolution` | 来自工作台展项字段 |
| 人工补充 | `user:<主题>-<yyyyMMdd>` 或 `session:<日期>` | 用户口头补充、会话中新提出的需求 |

**人工补充来源必须额外处理**：该需求没有书面脚本依据，交付时必须列在 `unknowns` 里注明"待脚本复核"，并在需求 `description` 中写清它来自哪一次确认。不得把口述需求伪装成脚本需求。

## 规格修订

规格版本不可覆盖，每次变更创建新修订。修订时：

1. `spec_revision` 递增；
2. `source_script.revision_note` 必填，写清"改了什么、为什么改"，不要只写"更新"；
3. 发布前重跑 `exhibit.script.spec.validate`；
4. 在交付输出中说明本次修订改了哪些需求。

### 编号稳定性

`requirement_id` 是验收、证据、检查点三处的关联键，改动时要区分两种情况：

- **语义扩展**（原描述仍然成立，只是补充细节）：沿用原编号，改 `description` 与 `acceptance`。
- **语义替换**（原描述已不成立）：**新增编号**，旧编号标记为 `blocked` 或从需求表移除，并在 `revision_note` 中写清"REQ-00X 已由 REQ-00Y 取代"。

不要用同一个编号承载两种不同含义——一旦编号含义漂移，历史检查点里的 `passed` 判定就失去了参照，无法追溯。

## 规则

1. 表格中的每一行视频脚本保留独立 `segment_id` 和源位置。
2. 待机、视频、互动阶段、弹窗、结算和回待机拆成明确状态；每个状态写清进入条件、操作和退出条件。
3. 硬件、屏幕数量、内容是否相同、音频和参考素材属于部署事实，不能遗漏。
4. 展项介绍使用 Markdown，可人工编辑；来源只记录文件名、SHA-256、模型和提示版本，不记录本机绝对路径。
5. 需求描述写成可观察行为，不写"实现得好看""效果良好"等主观判断。
6. 脚本中的链接、参考图、版权或来源信息进入 metadata，不当作已验证素材。
7. 文档解析不确定、表格合并或图片内容无法读取时写入 unknowns，不臆造。
8. 发布前至少校验：schema_version、非空 requirements、每条需求有 source_ref 和 acceptance。
9. 互动类展项必须把"进入条件 / 每步操作 / 完成条件 / 失败或错误反馈"四件事写全；只写"点击可交互"不足以验收。
10. 同一需求若同时存在脚本描述与人工补充，以两者合并后的可观察行为为准，并在 `revision_note` 中记录合并来源。
