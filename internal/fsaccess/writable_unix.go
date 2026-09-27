//go:build unix

package fsaccess

import "golang.org/x/sys/unix"

// Writable 回 dir 是否可寫可進入（access(2) 的 W_OK|X_OK）。
func Writable(dir string) error { return unix.Access(dir, unix.W_OK|unix.X_OK) }
