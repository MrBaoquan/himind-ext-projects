# 展项开发

项目级展项开发主入口。原始 DOC/DOCX 只在本地解析，工作台只保存可编辑 Markdown 展项介绍、开发规格和开发证据。

## 入口

- `script_sync`：本地解析脚本、生成并同步 Markdown 介绍、生成并发布开发规格。
- `development`：从已确认的展项规格继续实现，结合 UNIHPer 约束完成验证并同步证据。

## 依赖

- Skill：`com.himind.skill.exhibit-development`
- Skill：`com.mrbaoquan.skill.unihper-unity-development`
- Plugin：`com.himind.exhibit-development-tools`
- 工作台内置 Provider：展项介绍、规格与证据能力

## 输入

- `exhibit_id`：展项稳定 PID，不是 `EX-xxxx`。
- `script_path`：本地脚本路径；旧版 `.doc` 需先转换为 `.docx`，文件不会上传工作台。
- `workspace_root`：Unity / UNIHPer 工程目录。
- `evidence_path`、`evidence_kind`、`evidence_request_key`：可选的开发证据同步参数。

## 边界

Workflow 不直接实现 Word 解析、Unity API、SVN 管理或工作台 HTTP。确定性操作走插件，业务读写走内置 Provider，AI 开发规则走 Skill。脚本文件只在本机处理，工作台不保存原文件。
