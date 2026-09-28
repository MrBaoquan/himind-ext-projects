// Command himind-release-plan 打印一次发布的命名事实，可按同一份事实写发布清单。
//
// 入口复用官方扩展仓 tooling/commands 的实现：tag、制品名、锁名与依赖 pin
// 只有一份规则。
package main

import "github.com/MrBaoquan/himind-extensions/tooling/commands"

func main() {
	commands.ReleasePlanMain()
}
