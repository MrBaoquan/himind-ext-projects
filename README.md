# HiMind Agent 项目扩展仓

存放**具体项目交付流程**的 HiMind Agent 扩展。这里的扩展只服务某个项目的实际环境，
离开该项目就没有意义；通用能力仍然放在官方扩展仓
[MrBaoquan/himind-extensions](https://github.com/MrBaoquan/himind-extensions)。

## 本仓扩展

| 扩展 | 类型 | 稳定 ID | 说明 |
| --- | --- | --- | --- |
| 展馆预约小程序上传 | workflow | `com.himind.workflow.wx-venue-upload` | 展馆预约小程序：选展馆与环境构建、冻结候选、上传微信体验版 |
| UPM 包开发规范 | skill | `com.himind.skill.upm-package-development` | Unity UPM 包的结构、程序集、测试与内网发布规范；UNIHper 作为软依赖补充 |
| 展项开发工具 | plugin | `com.himind.exhibit-development-tools` | 解析脚本、校验规格、检查 Unity/H5 与体验门禁、构建内容包并生成证据清单 |
| 展项开发助手 | skill | `com.himind.skill.exhibit-development` | 把脚本转成可追踪规格，并按 UNIHPer 约束完成开发与证据同步 |
| 展项开发 | workflow | `com.himind.workflow.exhibit-development` | 脚本导入、规格确认、UNIHPer 开发、证据上传与人工验收 |
| 展项 Web 开发助手 | skill | `com.himind.skill.exhibit-web-development` | MyriadBox H5、Provider、体验门禁、Candidate 与证据规则 |
| 展项体验方向 | skill | `com.himind.skill.exhibit-experience-direction` | H5 展项去 AI 味文案、稳定转场、持续动态、性能与体验验收 |
| 展项 Web 开发 | workflow | `com.himind.workflow.exhibit-web-development` | H5 实现、体验门禁、多目标构建、WebBridge 冒烟与证据回读 |

## 在 Agent 里安装

1. 打开「能力 → 来源管理」，添加 GitHub 来源 `MrBaoquan/himind-ext-projects`；
2. 在「能力 → 市场」里安装工作流，或在「工作流」里直接启动。

来源带 `.himind/catalog.json`，Agent 可以据此检查更新；本仓发布带 RSA-PSS/SHA-256
签名，安装时会验签。

### 本地工作副本

开发中的扩展不依赖 Release Catalog。未发布时 `.himind/catalog.json` 为空是正常状态，
本地来源读取仓库根目录的 `extensions.json`：

```powershell
himind-agent extension source add-local himind-ext-projects <repo> extensions.json
himind-agent extension source refresh
himind-agent extension source install plugin com.himind.exhibit-development-tools
himind-agent extension source install skill com.himind.skill.exhibit-web-development
himind-agent extension source install workflow com.himind.workflow.exhibit-web-development
```

安装后可在扩展工作区看到源码项目，在“我的能力”和“工作流”中使用对应版本。

## 发布

发布由仓库维护者在本机执行，不依赖 GitHub Actions。工作流会先经 `himind-agent`
编译、测试、确认，产出不可变制品与扩展锁，再签名、创建 Release 并更新市场索引：

```powershell
$env:HIMIND_EXTENSION_SIGNING_KEY_ID = 'himind-production-2026'
$env:HIMIND_EXTENSION_SIGNING_PRIVATE_KEY_PATH = "$env:LOCALAPPDATA\HiMind\signing\private\himind-production-2026.key.pem"

./tools/release/publish-extension.ps1 `
  -Kind workflow `
  -ExtensionPath workflows/wx-venue-upload `
  -Repository MrBaoquan/himind-ext-projects `
  -AgentExecutable F:\WebProjects\项目看板\himind-agent\target\release\himind-agent.exe `
  -AgentProfile development
```

`HIMIND_EXTENSION_SIGNING_*` 是发布脚本这一组变量，和 Agent 运行时的
`HIMIND_SIGNING_*` 无关；用来上传 Release 的 GitHub App 私钥
（`*.private-key.pem`）不是签名密钥，用它签出来的清单装不上。

发布前必须先把提交推到 `origin/main`：发布清单里的源码提交与 Release 目标提交都取自
`HEAD`，远端取不到就会被拒绝。发布脚本同时执行 `go test ./...` 与仓库自检，扩展仓
不自洽时不会产生 Release。

发布链路要求 `himind-extensions` 检出在同级目录（`../himind-extensions`），命名、
校验、打包与索引逻辑全部复用官方仓的同一份实现，见 [tools/README.md](tools/README.md)。


