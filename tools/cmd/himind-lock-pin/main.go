// Command himind-lock-pin 把工作流扩展锁里的依赖摘要改写为依赖制品的载荷摘要。
//
// 入口复用官方扩展仓 tooling/commands 的实现：依赖定位与发布清单只有一份规则。
package main

import "github.com/MrBaoquan/himind-extensions/tooling/commands"

func main() {
	commands.LockPinMain()
}
