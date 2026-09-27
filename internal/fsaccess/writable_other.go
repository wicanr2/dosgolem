//go:build !unix

package fsaccess

import "os"

// Writable 在沒有 access(2) 的平台以建立再刪除暫存檔判定。
func Writable(dir string) error {
	f, err := os.CreateTemp(dir, ".writable-*")
	if err != nil {
		return err
	}
	name := f.Name()
	if err := f.Close(); err != nil {
		os.Remove(name)
		return err
	}
	return os.Remove(name)
}
