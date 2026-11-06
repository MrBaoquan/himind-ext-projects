# scenes.config.json 字段说明

位置：`vtour/scenes.config.json`。全局配置 `~/.pano-cli/config.json` 只提供 `krpanoToolsPath` 回落，其余字段一律读项目配置。

```jsonc
{
  "krpanoToolsPath": "",            // krpanotools 所在目录，空则用全局配置
  "panoSourceDir": "../panoramas",  // 全景图源目录，相对 vtour/
  "startScene": "",                 // 起始场景名 = JPG 文件名（不含扩展名）
  "publishDir": "../publish",       // 发布输出目录，相对 vtour/；缺省 ../publish

  "viewDefaults": {
    "fovmin": 70,                   // 项目级最小视野，场景未覆盖时继承
    "fovmax": 140                   // 项目级最大视野
  },

  "ossBaseUrl": "https://xxx.com",  // publish --oss 时的瓦片基础地址
  "ossBucket": "oss://bucket/path", // publish --oss 打印的同步目标

  "groups": [
    {
      "name": "展厅A",              // 分组显示名
      "folders": ["1-展厅A"],       // 源目录下的子文件夹名；"." 表示源目录根
      "scenes": ["入口", "中庭"]     // 可选：组内场景顺序，由排序编辑器写入
    }
  ],

  "sceneOverrides": {
    "入口": {
      "title": "序厅",              // 场景显示标题
      "ath": 0, "atv": 0, "fov": 90,// 默认视角
      "fovmin": 70, "fovmax": 140   // 场景级视野限制
    }
  }
}
```

## 必填与默认

- 校验要求存在 `panoSourceDir`、`startScene`、`groups`，且 `groups` 非空；缺项报 `scenes.config.json 缺少必填字段`。
- `groups` 由 `pano init` / `pano sync` / `pano xml` 扫描源目录自动生成并回写。文件夹名前缀的数字和分隔符会被剥掉作为分组名（`0-展馆外观` → `展馆外观`）；已存在的分组名会被保留，不会因重扫被覆盖。
- `groups[].scenes` 由编辑器的排序面板写入，用于固定组内顺序；未列出的新场景追加到组尾。注意它会被 `init`/`sync`/`xml` 的分组重扫清除，见下一节。
- `viewDefaults` 由编辑器「全局视野」维护；`sceneOverrides[*].fovmin/fovmax` 由「场景设置」维护，留空表示继承全局值。
- `sceneOverrides` 的键是**场景名（文件名去扩展名）**，不是 `scene_xxx` id。

## 分组顺序会被 CLI 重扫覆盖

`pano init`、`pano sync`、`pano xml` 三个命令都会先调用分组同步：从源目录重新扫出子文件夹，**整体重建 `groups` 数组并写回配置**。实测结果：

- `groups[].scenes`（组内场景顺序）会在重扫后**整个字段消失**，随后的 XML 按自然排序生成，手写或编辑器保存的顺序失效；
- `groups[].name` 只在文件夹路径能对上时保留，改名或移动文件夹等于新建分组。

真正会保留 `groups[].scenes` 的路径只有两条：编辑器里的「生成 XML」（走 `/api/rebuild-xml`）和 `pano publish`（两者都不调用分组同步）。

因此顺序要求严格的工程：

1. 先用 `pano serve --edit` 的排序面板调好顺序；
2. 之后不要再跑 `pano xml` / `pano sync`，需要刷新 XML 用编辑器里的「生成 XML」；
3. 必须跑 `pano sync`（例如新增了全景图）时，接受顺序回到自然排序，重新排一次；
4. 想长期固定顺序，就改文件夹名加编号前缀，让自然排序等于期望顺序——这是唯一不会被覆盖的做法。

## 扫描规则

- 只识别 `.jpg` 和 `.jpeg`（大小写不敏感），其他格式静默忽略。
- 源目录根下的 JPG 归入第一个分组，仅在没有任何含图子文件夹时才生成 `默认分组`（`folders: ["."]`）。
- 文件夹内按自然排序（数字按数值）扫描，`2.jpg` 排在 `10.jpg` 之前。
- 场景 id 由文件名编码而来（非 ASCII 字符按 krpano 规则映射成小写字母），`scenes.xml` 中写作 `scene_<编码>`；用户侧始终用场景名，不要自己推算这个 id。

## 生成产物对照

| 文件 | 来源 | 可否手改 |
| --- | --- | --- |
| `scenes.xml` | 扫描结果 + multires 缓存 + `sceneOverrides` | 否 |
| `skin/groups.xml` | 分组过滤逻辑 | 否 |
| `skin/groups_data.xml` | 分组按钮数据 | 否 |
| `.multires_cache.json` | 切片结果 | 否 |
| `_tiles.config` | 每次切片动态生成 | 否 |
| `main.xml` | 模板首次生成 | 是，升级不覆盖 |
| `hotspots/*.xml` | VTour Editor / `serve --edit` / 手写 | 是 |
| `hotspots/info/*.json` | 编辑器图文热点数据 | 是，结构需匹配现有条目 |

## 多分辨率与瓦片

- 瓦片配置固定为 `multires=true`、`tilesize=512`、`levels=auto`、`converttocube=true`，由 CLI 生成 `_tiles.config`，不要改。
- 缓存缺失时 `pano xml` 会从 `panos/<编码>.tiles` 反推，推不出则写入 `512,1024,2048,4096` 并告警——这只是占位值，真正重新切片要用 `pano sync --force-scene`。
