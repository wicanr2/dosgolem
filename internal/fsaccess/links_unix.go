//go:build unix

package fsaccess

import (
	"fmt"
	"os"
	"syscall"
)

func SingleLink(_ string, st os.FileInfo) error {
	v, ok := st.Sys().(*syscall.Stat_t)
	if !ok || v.Nlink != 1 {
		return fmt.Errorf("roots: not a proven single-link file")
	}
	return nil
}
