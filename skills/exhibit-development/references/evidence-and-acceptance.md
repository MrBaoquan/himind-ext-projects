# 证据与验收

## 最低证据

- 编译和 Console：无新增 error/exception。
- EditMode/PlayMode：覆盖本轮需求的最小真实链路。
- 截图：UI 或场景变化至少一张目标分辨率截图。
- 视频：完整待机、核心互动、结算或成品演示；无法录制时写清原因。
- 工作台回读：上传后按展项和 Artifact ID 查询成功。

## 清单与检查点的字段契约

证据清单和开发检查点都有固定 schema，位于工作流包的 `artifacts/` 目录：

- `artifacts/evidence-manifest.schema.json`
- `artifacts/development-checkpoint.schema.json`

两份 schema 都设置了 `additionalProperties: false`，多写字段、枚举越界、漏必填字段都会直接校验失败。

### 硬规则：清单只能由能力生成

`evidence-manifest` **必须由 `exhibit.evidence.pack` 生成**，不允许手写、不允许脚本拼装。手写清单是最高频的返工来源，常见错误包括：漏 `workspace_root`、每条 artifact 漏 `path`、顶层塞入 schema 不允许的 `checkpoint_id` / `spec_revision`。

调用形态：

```json
{
  "workspace_root": "<工程根>",
  "items": [
    {
      "path": "Assets/Screenshots/xxx.png",
      "artifact_kind": "development-screenshot",
      "requirement_ids": ["REQ-003"]
    }
  ]
}
```

能力会回填 `path`（绝对路径）、`relative_path`、`file_name`、`size_bytes`、`sha256`、`total_bytes`、`artifact_count`、`generated_at`。

约束：

- `path` 必须在 `workspace_root` 之内，否则报 `evidence path escapes workspace root`。
- 支持的扩展名：`.png` `.jpg` `.jpeg` `.webp` `.gif` `.mp4` `.webm` `.mov` `.mkv` `.pdf` `.txt` `.md` `.json` `.log` `.zip`。
- `items` 不能为空。
- 清单顶层不要加 `checkpoint_id`、`spec_revision`、`exhibit_pid` 等 schema 未声明的字段；这些关联信息写进检查点或上传时的 artifact metadata。

### 检查点字段

`development-checkpoint` 的必填字段与取值：

| 字段 | 要求 |
| --- | --- |
| `schema_version` | 固定 `exhibit-development-checkpoint.v1` |
| `checkpoint_id` | 非空 |
| `summary` | 非空 |
| `requirements` | 每条含 `requirement_id` + `status` |
| `verification` | 必须同时含 `compile`、`console`、`tests`、`visual` |
| `artifacts` | 字符串数组 |
| `created_at` | 非空 |

`requirements[].status` 只允许四个值：`passed` / `failed` / `blocked` / `not_run`。

注意**没有 `partial`**。"部分通过"必须落到 `blocked`（有外部依赖未解决）或 `not_run`（未执行），并在 `note` 里写清已完成与未完成的部分。

`verification.compile` 和 `verification.console` 都是**枚举字符串**（`passed` / `failed` / `not_run`），不是对象。需要补充说明时写进 `note` 之外的字段会校验失败，应改写进 `summary` 或 `tests[]` 文本。

`schema_version` 之外的额外顶层字段（例如 `changed_files`、`regressions_fixed`、`blockers`）不被允许，应把这类信息并入 `summary` 的叙述或 `tests[]`。

## 需求状态判定

| 状态 | 含义 | 判据 |
| --- | --- | --- |
| `passed` | 已由自动化证据覆盖 | 有对应测试或截图，且对应当前规格与当前 revision |
| `not_run` | 本轮未执行 | 明确写出未执行原因 |
| `blocked` | 被外部依赖卡住 | 写出阻塞项与解除条件 |
| `failed` | 执行了但不通过 | 写出失败现象 |

不允许用"总体印象良好"替代逐条判定；也不允许把未执行的需求写成 `passed`。

## Artifact 关联

上传到工作台时，每个 Artifact 必须带：

- `checkpoint_id`
- `spec_revision`
- `requirement_ids`
- `sha256` 和文件大小
- 生成方式、场景、Unity 版本或测试模式的 metadata

`workbench.exhibit.artifact.upload` 的必填参数是 `exhibit_id`、`local_path`、`request_key`、`artifact_kind`。其中 `exhibit_id` 必须是 `workbench.context.resolve` 或 `workbench.business.exhibit.list` 返回的 `pid`，**不能使用 `EX-xxxx` 展示编号**。`request_key` 用作幂等键，重试时必须复用同一个值。

上传后必须用 `workbench.exhibit.artifact.list` 回读确认可检索；没有回读不算上传成功。

## 验收规则

1. 需求矩阵逐条打勾，不允许用总体印象替代。
2. 人工验收项由用户确认，模型不得自判。
3. 截图必须对应当前规格和当前 SVN revision；过期证据不算通过。
4. 视频与截图可用于说明，但不能替代自动化测试结果。
5. 未覆盖项、失败项和假设必须进入最终报告。

## 工作台不可用时的处理

先区分"Provider 未连接"和"能力未激活"：

1. 调 `system_health` 看 `provider.state`；
2. 若 `available` 但调用报未知能力，说明只是没激活，走 `system.capability_search` → `activate`；
3. 确认 Provider 真的不可用时，才允许降级——此时必须：

- 本地生成并保留清单与检查点；
- 在交付输出中写明"未上传工作台、未回读"，并列出本地文件路径与 SHA-256；
- 不得把未上传表述为"已同步"。

降级不是完成状态，只是把上传推迟到 Provider 恢复后。
