module github.com/MrBaoquan/himind-ext-projects

go 1.22

require github.com/MrBaoquan/himind-extensions v0.0.0

// 发布链路（命名、校验、打包、索引）只有一份实现，放在官方扩展仓。本仓只保留
// 薄入口，规则不会在多个仓之间漂移。本地开发把 himind-extensions 克隆到同级
// 目录即可；CI 里用 `go mod edit -replace` 指向当次检出目录。
replace github.com/MrBaoquan/himind-extensions => ../himind-extensions
