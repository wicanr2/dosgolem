package machine

import (
	"bytes"
	"testing"
)

// TestProgramPathGoesToEnvironmentTail 釘住 `docs/spec/194-stubseg-font-collision-and-program-path` §2：
// 環境區塊尾端是 argv[0]；沒指定時維持中性預設，指定時照寫。
func TestProgramPathGoesToEnvironmentTail(t *testing.T) {
	for _, tc := range []struct{ set, want string }{{"", `C:\PROG.EXE`}, {`C:\START.EXE`, `C:\START.EXE`}} {
		m := New()
		m.ProgramPath = tc.set
		if err := m.LoadCOM([]byte{0xC3}); err != nil {
			t.Fatal(err)
		}
		env := m.Mem[uint32(EnvSeg)*16 : uint32(EnvSeg)*16+256]
		want := append([]byte{0x01, 0x00}, append([]byte(tc.want), 0)...)
		if !bytes.Contains(env, want) {
			t.Fatalf("ProgramPath=%q：環境區塊沒有 %q", tc.set, tc.want)
		}
	}
}
