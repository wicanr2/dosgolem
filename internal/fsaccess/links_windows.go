//go:build windows

package fsaccess

import (
	"fmt"
	"golang.org/x/sys/windows"
	"os"
)

func SingleLink(path string, _ os.FileInfo) error {
	p, e := windows.UTF16PtrFromString(path)
	if e != nil {
		return e
	}
	h, e := windows.CreateFile(p, windows.FILE_READ_ATTRIBUTES, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if e != nil {
		return e
	}
	defer windows.CloseHandle(h)
	var i windows.ByHandleFileInformation
	if e = windows.GetFileInformationByHandle(h, &i); e != nil {
		return e
	}
	if i.NumberOfLinks != 1 {
		return fmt.Errorf("roots: not a single-link file")
	}
	return nil
}
