---
name: krpano-tour-assistant
description: 指导用 pano CLI 创建、构建、编辑和发布 krpano 全景漫游项目；当用户要求生成全景漫游、处理 vtour 工程、添加热点、切片同步、本地预览或打包发布时使用。
---

# krpano 全景项目助手

用 `pano` 命令行工具推进 krpano 全景漫游工程：建项目、放全景图、生成瓦片、可视化编辑热点、本地预览、打包发布。本技能只承载操作知识和对真实输出的判读；所有构建、切片、XML 生成都由 `pano` 完成，不要手工重写 `scenes.xml`、`skin/groups.xml`、`skin/groups_data.xml`。

## 适用与不适用

适用：创建新全景项目、给已有 vtour 工程加场景或改分组、生成或重生成瓦片、在浏览器编辑器里加热点、改场景标题和默认视角、本地预览、打包发布（含 OSS 或增量发布）、升级内置框架文件、把本地 panohper 源码链接进项目调试。

不适用：制作或修图全景素材本身、替代 krpano 编译器、为没有 krpano 授权的机器生成瓦片、部署到远端服务器或配置 CDN。

## 一、先确认环境

按顺序做，任一步不满足就停下来说明缺什么，不要跳过直接构建。

1. **CLI 是否可用**：执行 `pano --version`。可用时继续；不可用时说明需要安装，并在用户确认后执行：

   ```bash
   npm install -g @mrbaoquan/pano-cli
   ```

   私有或内网环境用本地源：`npm install -g @mrbaoquan/pano-cli --registry http://upm.andcrane.com:4873`。当前版本线为 2.5.x，`pano --version` 输出与安装源不一致时以实际输出为准。

2. **Node.js**：`node -v` 需 ≥ 18（CLI 是 ES module 包）。

3. **krpano 授权目录**：切片依赖 `krpanotools`。`pano config --global` 可查到已配置的全局路径；未配置时向用户索取 krpano 安装目录（1.20+），确认该目录下同时存在 `krpanotools.exe`（非 Windows 为 `krpanotools`）和 `templates/vtour-multires.config`，再执行：

   ```bash
   pano config --global --krpano "<krpano 安装目录>"
   ```

   找不到 `krpanotools` 时不要用其他工具伪造瓦片。

4. **工作目录**：`pano` 自动判断当前目录是项目根（存在 `vtour/scenes.config.json`）还是 `vtour/` 目录本身，两者都能执行。跨目录执行会读到错误的工程，务必先 `cd` 到目标工程。

## 二、场景路由

| 用户意图 | 命令 |
| --- | --- |
| 新建全景项目 | `pano create <name>` |
| 首次生成瓦片和框架文件 | `pano init`（结构缺文件时 `pano init --repair`） |
| 新增了全景图、要增量切片 | `pano sync` |
| 重切全部或多张瓦片 | `pano sync --force` / `pano sync --force-scene <场景名...>` |
| 只改了分组、标题、视角 | `pano xml` |
| 浏览器里可视化编辑热点、视角、排序 | `pano serve --edit` |
| 用 VTour Editor 传统方式改热点 | `pano editor` → 编辑 → `pano save` |
| 只看效果 | `pano serve [-p 端口]` |
| 打包出发布目录和压缩包 | `pano publish [--skip-panos] [--compress-images] [--oss]` |
| 清掉旧瓦片重来 | `pano clean` |
| CLI 升级后同步内置框架 | `pano upgrade [--check]` |
| 链接本地 panohper 源码调试 | `pano dev panohper <path>` / `pano dev --status` / `pano dev --reset panohper` |
| 查/改配置 | `pano config [--global] [--krpano <path>] [--source <dir>]` |

命令参数、行为差异和真实输出样例见 `references/commands.md`；`scenes.config.json` 字段含义见 `references/project-config.md`；报错定位见 `references/troubleshooting.md`。

## 三、项目结构与约定

```
my-tour/
├── vtour/
│   ├── scenes.config.json   # 工程配置：krpano 路径、源目录、分组、场景覆盖
│   ├── tour.xml             # 框架入口，pano upgrade 会覆盖
│   ├── main.xml             # 项目自定义逻辑，升级不覆盖
│   ├── scenes.xml           # 自动生成
│   ├── skin/groups.xml      # 自动生成
│   ├── skin/groups_data.xml # 自动生成
│   ├── hotspots/            # 热点，按场景文件名一一对应
│   ├── panos/               # 切片瓦片，自动生成
│   └── panohper/            # 内置交互组件库
└── panoramas/               # 全景图源目录，按子文件夹分组
```

需要记住的硬约定：

- **只扫描 JPG**：源目录下只认 `.jpg` / `.jpeg`。PNG、WebP、TIFF 会被静默忽略，先转 JPG 再同步。
- **全景图必须是等距柱状投影**，常规宽高比为 2:1；比例异常时先提示用户确认素材。
- **分组由子文件夹决定**：文件夹名前缀数字和分隔符会被去掉作为分组名（`1-展厅A` → `展厅A`）。想调整分组就是移动图片文件所在文件夹，然后 `pano sync` 或 `pano xml`。
- **场景名 = 文件名（不含扩展名）**：`scenes.config.json` 的 `startScene`、`sceneOverrides`、`--force-scene` 全部用这个名字，不是 `scene_xxx` 的 id。
- **热点按文件名绑定**：场景 `餐厅1` 的热点文件是 `hotspots/餐厅1.xml`，与分组无关。
- **两种模式**：Include 模式（正常，`tour.xml` include `scenes.xml`）和 Editor 模式（场景内联进 `tour.xml`）。进入编辑后必须 `pano save` 回到 Include 模式，否则 `pano publish` 会拒绝。
- **组内顺序是易失的**：`pano init` / `sync` / `xml` 都会重扫源目录并重建 `groups`，把编辑器排序写入的 `groups[].scenes` 整段清掉（已实测）。只有编辑器的「生成 XML」和 `pano publish` 会保留它。用户要求固定顺序时，优先用文件夹名编号让自然排序等于期望顺序，而不是只依赖排序面板。

## 四、执行规范

1. **一次一步**：执行一条 `pano` 命令，读完输出再决定下一条。不要在一次回复里连续跑 `init` + `sync` + `publish`。
2. **瓦片生成耗时长**：`init` / `sync` / `sync --force` 会调用 `krpanotools` 逐张切片，几十张全景图可能持续数分钟。用较长超时或后台执行，不要把「没有立即输出」当作失败。
3. **不要手改自动生成文件**：`scenes.xml`、`skin/groups.xml`、`skin/groups_data.xml`、`panos/`、`.multires_cache.json`、`_tiles.config` 都由 CLI 维护。需要改内容就改 `scenes.config.json` 或 `hotspots/*.xml`，再跑 `pano xml` 或 `pano sync`。
4. **先看配置再动手**：不确定工程状态时先跑 `pano config`，确认 `krpanoToolsPath`、`panoSourceDir`、分组和起始场景，再执行写操作。
5. **交互式命令要预先满足前提**：`pano create` 会当场询问 krpano 目录并等待输入，非交互环境里必须先配好全局路径，并用空行回车接受默认值，否则进程会挂住。
6. **报告真实结果**：以命令输出的场景映射表和瓦片计数为准汇报，不要根据文件存在与否推断成功。

## 五、风险与确认

以下操作在执行前必须说明影响并取得用户同意：

| 操作 | 风险 |
| --- | --- |
| `pano clean` | 删除 `panos/` 全部瓦片、`scenes.xml` 和分辨率缓存，只能靠 `pano sync` 重建 |
| `pano publish` | 先整目录删除发布目录（默认 `../publish`，可被 `publishDir` 改写），再重新生成；目录配置错会删到别的地方 |
| `pano upgrade` | 用 CLI 内置模板覆盖 `tour.xml`、`panohper/` 和 `pano-runtime.js`，项目侧对这些文件的修改会丢失；建议先跑 `--check` |
| `pano sync --force` | 重切全部瓦片，耗时与体积都显著增加 |
| `pano init` 重建 | 已存在 `tour.html` 时会跳过；用户要求「重新初始化」时需明确先删哪些文件 |
| `pano publish --oss` | 只改瓦片路径指向 OSS，不负责上传；不要把它说成已完成部署 |
| `pano dev panohper <path>` | 把项目内 `panohper/` 换成符号链接，`pano upgrade` 会跳过它 |

额外约束：

- 只在用户指定的工程目录内读写；不要清理工程外的路径，不要把发布目录指向剧本外的位置。
- 不对 krpano 授权文件做复制、破解或跨机分发。
- `--oss` 需要的 `ossBaseUrl` / `ossBucket` 从 `scenes.config.json` 读取，缺了直接报错，不要编造地址。

## 六、常见判断

- `ConfigError: scenes.config.json 未找到`：当前目录不是工程根也不是 `vtour/`，先定位工程。
- `ToolNotFoundError: krpanotools 未找到`：`krpanoToolsPath` 指向的目录不含 `krpanotools.exe`；改配置，不要改代码。
- `vtour 已包含 tour.html，跳过初始化`：工程已初始化，新增场景应该用 `pano sync`。
- `当前目录不是已初始化的 vtour 工程`（`upgrade`）：还没跑过 `pano init`。
- `当前处于编辑模式…请先运行: pano save`（`publish`）：先 `save` 回 Include 模式。
- `端口已被占用`（`serve -p`）：换端口；不指定 `-p` 时 CLI 会自动向上递增，最多尝试 20 个。
- 分组或顺序不对：调整 `panoramas/` 下的文件夹和 `scenes.config.json` 的 `groups[].scenes`，再 `pano xml`。

详细报错与处理方式见 `references/troubleshooting.md`。

## 七、交付说明

完成后向用户说明：工程路径、改动了哪些命令产生的文件、场景与分组数量（引用映射表原话）、瓦片是否重建、发布目录与压缩包路径（`publish-v<时间戳>.tar.gz`）、以及尚未验证的部分（例如未在浏览器里实际点开热点、未做服务器上传）。不要宣称没有执行过的发布或部署已经完成。
