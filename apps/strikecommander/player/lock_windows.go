//go:build windows

package player

import (
	"fmt"
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
)

type ProcessLock struct {
	f *os.File
	o windows.Overlapped
}

func Lock(root string) (*ProcessLock, error) {
	if e := os.MkdirAll(root, 0755); e != nil {
		return nil, e
	}
	f, e := os.OpenFile(filepath.Join(root, "lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	l := &ProcessLock{f: f}
	if e = windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &l.o); e != nil {
		f.Close()
		return nil, fmt.Errorf("已有另一個陸空戰將在執行：%w", e)
	}
	return l, nil
}
func (l *ProcessLock) Close() error {
	if l == nil || l.f == nil {
		return nil
	}
	f := l.f
	l.f = nil
	_ = windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &l.o)
	return f.Close()
}
