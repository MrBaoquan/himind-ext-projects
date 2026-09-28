// Command himind-catalog-upsert 把一次发布写进本仓的市场索引。
//
// 入口复用官方扩展仓 tooling/commands 的实现：索引记录只从 Release 清单派生，
// 不重新推断字段。
package main

import "github.com/MrBaoquan/himind-extensions/tooling/commands"

func main() {
	commands.CatalogUpsertMain()
}
