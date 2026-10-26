//go:build !unix && !windows

package fsaccess

import (
	"fmt"
	"os"
)

func SingleLink(_ string, _ os.FileInfo) error { return fmt.Errorf("roots: link count unavailable") }
