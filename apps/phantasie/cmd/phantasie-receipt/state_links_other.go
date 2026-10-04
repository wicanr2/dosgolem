//go:build !linux && !darwin && !windows && !freebsd && !netbsd && !openbsd && !dragonfly

package main

import (
	"fmt"
	"os"
)

func stateLinks(path string, _ os.FileInfo) (uint64, error) {
	return 0, fmt.Errorf("此平台無法驗證存檔硬連結：%s", path)
}
