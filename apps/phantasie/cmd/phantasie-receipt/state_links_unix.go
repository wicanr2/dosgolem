//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package main

import (
	"fmt"
	"os"
	"syscall"
)

func stateLinks(path string, info os.FileInfo) (uint64, error) {
	s, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, fmt.Errorf("無法驗證存檔硬連結：%s", path)
	}
	return uint64(s.Nlink), nil
}
