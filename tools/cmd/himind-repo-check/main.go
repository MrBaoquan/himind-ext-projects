// Command himind-repo-check 校验本仓作为一个扩展仓是否自洽。
//
// 门禁只有一份实现：官方扩展仓的 tooling/commands。本仓只保留入口，避免
// 「本仓能发、Agent 装不上」这种由规则分叉带来的故障。
package main

import (
	"fmt"
	"os"

	"github.com/MrBaoquan/himind-extensions/tooling/commands"
)

func main() {
	if err := commands.RepoCheckMain(); err != nil {
		fmt.Fprintln(os.Stderr, "extension repository is invalid:", err)
		os.Exit(1)
	}
}
