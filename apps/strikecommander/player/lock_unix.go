//go:build linux || darwin

package player

import (
	"fmt"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
)

type ProcessLock struct{ f *os.File }

func Lock(root string) (*ProcessLock, error) {
	if e := os.MkdirAll(root, 0755); e != nil {
		return nil, e
	}
	f, e := os.OpenFile(filepath.Join(root, "lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	if e = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); e != nil {
		f.Close()
		return nil, fmt.Errorf("已有另一個陸空戰將在執行：%w", e)
	}
	return &ProcessLock{f}, nil
}
func (l *ProcessLock) Close() error {
	if l == nil || l.f == nil {
		return nil
	}
	f := l.f
	l.f = nil
	_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
	return f.Close()
}
