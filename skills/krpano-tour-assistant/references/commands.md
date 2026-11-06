# pano 命令参考

以下内容对齐 pano-cli 2.5.x 的实际行为，包括被文档省略的隐含操作。命令在项目根目录或 `vtour/` 目录下执行等价。

## 全局选项

| 选项 | 作用 |
| --- | --- |
| `-v, --verbose` | 输出调试信息，含实际执行的 krpanotools 命令 |
| `-q, --quiet` | 只输出警告和错误 |
| `-V, --version` | 版本号 |

## create

```bash
pano create <name>
```

在**当前目录**下新建 `<name>/`，目录已存在则直接失败。过程分两部分：

1. 询问 krpanotools 所在目录。有全局配置时提示「（回车使用全局配置: ...）」，直接回车即采用；输入新路径时只校验目录下是否有 `krpanotools.exe`，不校验 `templates/`。
2. 生成 `vtour/scenes.config.json`、`tour.xml`、`main.xml`、`WORKFLOW.md`、`hotspots/`，以及 `panoramas/default/`、`.gitignore`、`README.md`。

交互提示无法跳过，自动化环境需先运行 `pano config --global --krpano <path>`，再用空行回车确认。

## init

```bash
pano init
pano init --repair
```

不带参数时按六步执行：读配置并校验（要求 `krpanoToolsPath` 已配置且 `krpanotools` 存在）→ 扫描源目录同步分组 → 用第一张全景图调用 `krpanotools makepano` 生成 `tour.html`、`tour.js`、`skin/`、`plugins/` → 覆盖 `tour.xml`、复制 `panohper/`、补 `main.xml` 与 `pano-runtime.js` 并注入 `tour.html` 引用 → 为全部场景生成瓦片 → 生成 `scenes.xml` 与 `skin/groups_data.xml`。

- 已存在 `vtour/tour.html` 时整体跳过初始化，只提示；这种情况应该用 `pano sync`。
- 源目录没有可用 JPG 时报错 `未找到全景图`，不会生成空工程。
- `--repair` 只补缺失的框架文件（`tour.xml`、`main.xml`、`panohper/`、`hotspots/`、`skin/`、`WORKFLOW.md`、`groups_data.xml`），**不生成瓦片**，适合 CLI 升级后修结构。

## sync

```bash
pano sync
pano sync --force
pano sync --force-scene <场景名> [场景名...]
```

扫描源目录 → 同步分组 → 只为缺少产物或标记强制的场景生成瓦片 → 重写 `scenes.xml` 和 `skin/groups_data.xml`。

- 默认增量：已有瓦片和多分辨率缓存命中即跳过。
- `--force` 重切全部场景。
- `--force-scene` 接受多个位置参数，值必须是**场景名（JPG 文件名去扩展名）**；有任一名称不存在就整体报错 `未找到指定场景: ...`，此时不会写入任何内容。
- 耗时与图片数量和 `krpanotools` 速度成正比，几十张图片可能数分钟。
- 会重扫源目录重建 `groups` 并写回配置，`groups[].scenes` 排序被清除；需要保序请改用编辑器「生成 XML」或 `pano publish`。

## xml

```bash
pano xml
```

只重新扫描并生成 XML，不切片。缺多分辨率缓存时从 `panos/<场景>.tiles` 目录结构反推，推不出就写入默认值 `512,1024,2048,4096` 并给出警告。适合改标题、改分组、改视野参数后的快速刷新。

注意它同样会重扫并重建 `groups`，`groups[].scenes` 排序会被清掉（实测：执行前 `scenes:["入口","中庭"]`，执行后该字段消失，XML 按自然排序生成）。要保住编辑器排好的顺序，用编辑器里的「生成 XML」（`POST /api/rebuild-xml`），它只重写 `scenes.xml` 和 `groups_data.xml`，不碰配置。

## serve

```bash
pano serve
pano serve -p 3000
pano serve --edit
pano serve -e -p 8092
```

启动本地静态服务器，根目录是当前 `vtour/`，首页重定向到 `tour.html`，启动后自动用默认浏览器打开。

- 未传 `-p` 时从 8090 起自动向上找空闲端口，最多尝试 20 个；传了 `-p` 而端口被占用则直接失败。
- 服务器常驻前台，按 Ctrl+C 停止；自动化场景下用后台方式启动，确认 `tour.html` 返回 200 后再继续。
- 内置 API：`/api/config`、`/api/scenes`、`/api/view-defaults`、`/api/scene-settings`、`/api/scene-view`、`/api/hotspots`、`/api/hotspot`、`/api/hotspot-info`、`/api/upload-asset`、`/api/sort`、`/api/rebuild-xml`。
- `--edit` 额外提供 `/__editor__.js` 并把编辑器脚本注入 `tour.html`。

编辑器面板能力：

| 面板 | 写入位置 |
| --- | --- |
| 设为默认视角 | `scenes.config.json` → `sceneOverrides[场景].ath/atv/fov` |
| 场景设置 | `sceneOverrides[场景].title/fovmin/fovmax` |
| 全局视野 | `scenes.config.json` → `viewDefaults.fovmin/fovmax` |
| 热点管理 | `hotspots/<场景>.xml`，图文资源配置在 `hotspots/info/<场景>__<热点>.json` |
| 排序管理 | `scenes.config.json` → `groups[].scenes` |
| 生成 XML | 重写 `scenes.xml`、`skin/groups.xml`、`skin/groups_data.xml` |

热点类型：跳转场景、信息面板、打开链接、播放视频、绿幕讲解，各自有独立的字段；编辑器内新建热点时字段由面板写入，手工改 XML 时按 `hotspots/*.xml` 现有条目结构补，不要凭猜测新增未知属性。

## editor / save

```bash
pano editor
pano save
```

`editor` 把 `scenes.xml` 里的场景内联进 `tour.xml`（去掉 `<include url="scenes.xml">`），供 VTour Editor 可视化编辑热点；已内联时只提示。`save` 把 `tour.xml` 中的热点抽回 `hotspots/`，恢复 include，并重新生成 `scenes.xml`、`skin/groups.xml`、`skin/groups_data.xml`。

这组命令与 `pano serve --edit` 是两条不同的编辑路线，不要混用：`serve --edit` 全程保持 include 模式。

## publish

```bash
pano publish
pano publish --skip-panos
pano publish --compress-images
pano publish --oss
```

七步：定位发布目录 → 重新扫描并生成 `scenes.xml` / `groups_data.xml` → 准备运行态 `pano-runtime.js` → 复制 `tour.html`、`tour.js`、`tour.xml`、`main.xml`、`scenes.xml` → 复制资源目录 → 生成 tar.gz → 统计。

关键行为：

- 发布目录 = `scenes.config.json` 的 `publishDir`，缺省 `../publish`（相对 `vtour/`）。**已存在时先整个删除**，删除失败（被占用）才退化为清空内容。
- `skipPanos` 时不复制 `panos/`，且会在提示里要求服务器保留旧瓦片。
- `--oss` 跳过 `panos/` 复制，把 `scenes.xml` 的瓦片路径指向 `ossBaseUrl`；只打印 `ossutil sync` 命令，不上传。
- 布局资源会被重写：`hotspots/info/*.json` 汇总成 `hotspots/info/index.json`，引用的本地图片/音频/二维码复制到 `runtime-assets/<hash>.<ext>` 并改指向。
- `--compress-images` 对 `assets/`、`runtime-assets/`、`panohper/example/assets` 下 ≥250KB 的 JPG/PNG/WebP 压缩，只改发布副本，不动源文件。
- 压缩包命名 `publish-vYYYYMMDD-HHmmss.tar.gz`，放在发布目录的同级；优先用 `tar -czf`，不可用时退回 Python `tarfile`，两者都缺则报错。
- 处于 Editor 模式（`tour.xml` 已内联场景）或 `scenes.xml` 缺失时直接拒绝执行。

## clean

```bash
pano clean
```

删除 `panos/` 下全部内容并重建空目录，再删除 `.multires_cache.json`、`scenes.xml`、`_tiles.config`、`_makepano_temp.xml`。之后必须 `pano sync`（或 `init`）才能恢复。

## config

```bash
pano config
pano config --krpano "F:\path\to\krpano"
pano config --source "../panoramas"
pano config --global
pano config --global --krpano "F:\path\to\krpano"
```

无参数时打印生效配置。项目配置写 `vtour/scenes.config.json`，全局配置写 `~/.pano-cli/config.json`；项目配置优先。只有 `krpanoToolsPath` 会从全局回落，其他字段不继承。

## upgrade

```bash
pano upgrade
pano upgrade --check
```

要求已初始化（存在 `tour.html`），否则报错。`--check` 只比对 `tour.xml`、`panohper/`、`main.xml` 是否与内置模板一致。执行时：

- 覆盖 `panohper/`（符号链接则跳过并提示先 `pano dev --reset`）、`tour.xml`、`pano-runtime.js`、`WORKFLOW.md`；
- 补 `main.xml`（不覆盖；旧版 `project.xml` / `custom.xml` 会重命名迁移）；
- 在 `tour.html` 里补 `pano-runtime.js` 引用。

`main.xml`、`hotspots/`、`panos/`、`assets/` 中的业务数据不受影响，但项目对 `tour.xml` 的自定义修改会丢失。

## dev

```bash
pano dev panohper ../panohper
pano dev --status
pano dev --reset panohper
```

把 `vtour/panohper/` 换成指向本地源码仓库的符号链接，便于实时调试组件库；`--status` 查看当前链接状态，`--reset` 恢复内置副本。链接期间 `pano upgrade` 不会覆盖 `panohper/`。

## 依赖

- Node.js ≥ 18（ES module；`sharp` 随包安装，用于图片压缩）
- krpano 1.20+ 的商业授权副本，需提供 `krpanotools` 与 `templates/vtour-multires.config`
- `publish` 生成压缩包需要 `tar`（Windows 10+ 自带）或本机 Python
