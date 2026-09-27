package main

import (
	"syscall"
	"unsafe"
)

// messageBox 讓沒有主控台的 Windows 版看得到致命錯誤（規格 035 §3.3）。
func messageBox(title, text string) {
	t, err1 := syscall.UTF16PtrFromString(title)
	m, err2 := syscall.UTF16PtrFromString(text)
	if err1 != nil || err2 != nil {
		return
	}
	const mbIconError = 0x10
	syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), mbIconError)
}
