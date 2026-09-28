// Command himind-plugin-package 把构建好的插件目录打成 .hmpkg 制品。
//
// 入口复用官方扩展仓 tooling/commands 的实现。
package main

import "github.com/MrBaoquan/himind-extensions/tooling/commands"

func main() {
	commands.PluginPackageMain()
}
