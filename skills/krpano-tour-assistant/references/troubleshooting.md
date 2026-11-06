# 报错定位与处理

`pano` 的报错基本分为三类：配置类（`ConfigError`）、工具缺失（`ToolNotFoundError`）、构建类（`BuildError`）。异常直接打印 message 并以退出码 1 结束；成功退出码为 0。判读依据是命令输出，不是文件是否存在。

## 环境与安装

| 现象 | 原因 | 处理 |
| --- | --- | --- |
| `Cannot find module ...\node_modules\@mrbaoquan\pano-cli\bin\pano.js` | 全局包目录不完整（升级中断或包被清理） | 重新安装：`npm install -g @mrbaoquan/pano-cli`，然后 `pano --version` 确认 |
| 命令存在但输出乱码或版本对不上 | PATH 上有多个 node 版本或旧 shim | 用 `where pano` / `which pano` 确认实际执行文件，必要时用完整路径或重装 |
| `Cannot find package 'commander'` | 直接 `node bin/pano.js` 运行未安装依赖的源码 | 使用安装后的 `pano` 命令，不要在解包目录里裸跑源码 |
| `ERR_REQUIRE_ESM` 或语法报错 | Node.js < 18 | 升级 Node.js 到 18 以上 |

## 配置

| 现象 | 原因 | 处理 |
| --- | --- | --- |
| `scenes.config.json 未找到: ...` | 当前目录既不是工程根也不含 `vtour/scenes.config.json` | `cd` 到项目根或 `vtour/` 后重试 |
| `scenes.config.json 缺少必填字段: ...` | 手改了配置导致 `panoSourceDir` / `startScene` / `groups` 缺失 | 补齐字段；`groups` 为空也会报错 |
| `scenes.config.json 中 groups 不能为空` | 源目录下没有含 JPG 的子文件夹，且配置里分组被清空 | 放图到源目录子文件夹后 `pano sync` |
| `全景图源目录不存在: ...` | `panoSourceDir` 相对 `vtour/` 解析后的目录不存在 | 修正路径或创建目录 |
| `缺少 krpanoToolsPath（瓦片生成需要）` | 项目与全局配置都没有 krpano 目录 | `pano config --global --krpano <path>` 或 `pano config --krpano <path>` |

## krpanotools

| 现象 | 原因 | 处理 |
| --- | --- | --- |
| `krpanotools 未找到: ...\krpanotools.exe` | 配置目录里没有可执行文件（层级填深了一层或填成 `templates`） | 填 `krpanotools.exe` 所在目录本身 |
| `vtour-multires.config 未找到: ...\templates\vtour-multires.config` | krpano 版本过旧或拷贝不完整 | 使用 1.20+ 的完整安装副本 |
| `krpanotools 执行失败: ...` | 图片损坏、磁盘空间不足、路径含特殊字符 | 看 stderr 原文；先单独确认图片能用 krpano 打开 |

## 初始化与同步

| 现象 | 原因 | 处理 |
| --- | --- | --- |
| `vtour 已包含 tour.html，跳过初始化。` | 工程已初始化 | 新增场景用 `pano sync`；确实要重建，先明确删除 `tour.html`、`tour.js`、`skin/`、`plugins/`、`panos/` |
| `未找到全景图。请先将全景图放入 panoSourceDir 对应的分组文件夹中。` | 源目录里没有 `.jpg`/`.jpeg` | 确认扩展名（PNG/WebP 不算），确认放进了扫描到的子文件夹 |
| `未找到指定场景: a, b` | `--force-scene` 传了场景 id 或带扩展名的文件名 | 用 JPG 文件名去扩展名后的场景名 |
| 切片很慢、输出只有 `[1/2]` 递增 | 正常进度输出，不是卡死 | 等待或用后台执行；不要重复启动导致文件争用 |

## 编辑与发布

| 现象 | 原因 | 处理 |
| --- | --- | --- |
| `当前处于编辑模式（场景已内联），请先运行: pano save` | 用过 `pano editor` 没 `save` | 先 `pano save` |
| `scenes.xml 不存在，请先运行: pano xml` | 自动产物被删 | `pano xml`（或 `pano sync` 重建瓦片与 XML） |
| `tour.xml 中既无 include 也无 scene，请先运行 pano xml` | `tour.xml` 被覆盖成其他内容 | `pano init --repair` 补框架文件后重跑 `pano xml` |
| `当前目录不是已初始化的 vtour 工程。请先运行 pano init。` | 在没初始化的工程上跑 `upgrade` | 先 `pano init` |
| `端口 8090-... 均被占用，请使用 -p 手动指定端口` | 连续 20 个端口被占用 | `pano serve -p <空闲端口>` |
| `端口 <n> 已被占用，请使用其他端口` | 指定了 `-p` 且被占用 | 换端口，或去掉 `-p` 让 CLI 自动递增 |
| `压缩包生成失败：未找到可用的 tar 或 Python 环境` | 系统缺 `tar` 和 Python | 安装其一；Windows 10+ 自带 `tar` |
| 发布目录被占用导致 `EPERM` | 资源管理器或网站服务打开着文件 | 关闭占用后重试；CLI 会退化为清空内容而非删除目录 |
| `使用 --oss 时，scenes.config.json 中必须配置 ossBaseUrl` | 配置缺 OSS 地址 | 补齐 `ossBaseUrl`（`ossBucket` 用于打印同步命令） |

## 结果判读要点

- `pano init` / `pano sync` 成功会打印场景映射表（分组 → `scene_xxx = 场景名`），以它为准确认场景与分组数量。
- `pano publish` 成功会打印发布目录、文件数、总大小和 `publish-v<时间戳>.tar.gz` 路径；`--oss` 时只打印 `ossutil sync` 提示，文件并未上传。
- `pano xml` 对缺少缓存的新场景会打印 `无 multires 数据，使用默认值` 警告；此时瓦片并未重新生成，别当成已同步。
- 任何命令以非 0 退出码结束时，先复位上次操作留下的中间文件（`_tiles.config`、`_makepano_temp.xml`），再重试。
