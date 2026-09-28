# 本仓工具链

本仓只放**入口**，规则实现全部在官方扩展仓 `MrBaoquan/himind-extensions`：

| 入口 | 实现 |
| --- | --- |
| `cmd/himind-repo-check` | `himind-extensions/tools/cmd/himind-repo-check`（清单、索引、签名、幽灵条目的全部门禁） |
| `cmd/himind-release-plan` | `himind-extensions/tools/cmd/himind-release-plan`（tag、制品名、依赖 pin，发布链路的唯一事实） |
| `cmd/himind-catalog-upsert` | `himind-extensions/tools/cmd/himind-catalog-upsert`（把 Release 事实写进市场索引） |
| `cmd/himind-skill-package` / `cmd/himind-plugin-package` | 官方仓同名命令（`.hmskill` / `.hmpkg` 打包） |
| `release/*.ps1` | 官方仓 `tools/release` 的同源副本，只按清单声明搬运制品，不自己拼名字 |

依赖通过 `go.mod` 的 `replace github.com/MrBaoquan/himind-extensions => ../himind-extensions`
指向本地检出，因此：

- 本地开发和发布：把 `himind-extensions` 克隆到本仓同级目录；
- CI：把工具链仓检出到 `.ci/himind-extensions`，再用
  `go mod edit -replace` 指向该目录（见 `.github/workflows/validate.yml`）。

这个结构的好处只有一条：**校验与命名规则只有一份实现**。复制一份 `repo-check` 到本仓
看起来更省事，但两边的门禁会随时间分叉，最终表现为「本仓能发、Agent 装不上」。
