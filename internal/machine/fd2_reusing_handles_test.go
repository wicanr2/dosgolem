package machine

import (
	"bytes"
	"testing"
)

type fd2ReusableTestFile struct{ *bytes.Reader }

func (f *fd2ReusableTestFile) Close() error { return nil }
func TestFD2DOSConstructorReusesClosedHandles(t *testing.T) {
	for _, s := range []*FD2StartupDOS{NewFD2StartupDOS(nil), {}} {
		for i := 0; i < 300; i++ {
			h, code := s.handles().Add(&fd2ReusableTestFile{bytes.NewReader([]byte("x"))}, "TMP")
			if code != 0 || h != 5 {
				t.Fatalf("反覆開關越出模式表：%d %d", h, code)
			}
			if s.handles().Close(h) != 0 {
				t.Fatal("關閉失敗")
			}
		}
	}
}
