package cpu

import (
	"fmt"
	"testing"
)

// docs/spec/433-real-mode-system-instructions §8 第 1 項：Model8086 與 Model80186 下即使 Sys 非 nil，
// 0F 仍照現況（POP CS 或失敗）。

// fakeSys 是測試用的 SysBus：任何方法被呼叫都記下來。
type fakeSys struct{ calls int }

func (f *fakeSys) CR(int) (uint32, error)  { f.calls++; return 0x10, nil }
func (f *fakeSys) SetCR(int, uint32) error { f.calls++; return nil }
func (f *fakeSys) DR(int) uint32           { f.calls++; return 0 }
func (f *fakeSys) SetDR(int, uint32)       { f.calls++ }
func (f *fakeSys) GDTR() (uint32, uint16)  { f.calls++; return 0, 0 }
func (f *fakeSys) SetGDTR(uint32, uint16)  { f.calls++ }
func (f *fakeSys) IDTR() (uint32, uint16)  { f.calls++; return 0, 0x3FF }
func (f *fakeSys) SetIDTR(uint32, uint16)  { f.calls++ }
func (f *fakeSys) User() bool              { f.calls++; return false }
func (f *fakeSys) IVTMismatch() bool       { f.calls++; return false }

// 434 §5.2 的增補。
func (f *fakeSys) CPL() uint8               { f.calls++; return 0 }
func (f *fakeSys) ReadSup32(uint32) uint32  { f.calls++; return 0 }
func (f *fakeSys) LDTR() (uint16, SegCache) { f.calls++; return 0, SegCache{} }
func (f *fakeSys) SetLDTR(uint16, SegCache) { f.calls++ }
func (f *fakeSys) TRSel() uint16            { f.calls++; return 0 }

func TestSysOlderModelsIgnoreSys(t *testing.T) {
	for _, model := range []Model{Model8086, Model80186} {
		for _, op2 := range []byte{0x00, 0x01, 0x06, 0x20, 0x22} {
			run := func(sys SysBus) (*CPU, error) {
				b := newTestBus()
				c := New(b)
				c.Model = model
				c.Sys = sys
				c.Seg[CS], c.Seg[SS], c.IP, c.R[SP] = 0x1000, 0x2000, 0, 0x100
				b.mem[0x2000*16+0x100], b.mem[0x2000*16+0x101] = 0x34, 0x12
				copy(b.mem[0x1000*16:], []byte{0x0F, op2, 0xE0, 0x90})
				return c, c.Step()
			}
			ref, refErr := run(nil)
			f := &fakeSys{}
			got, gotErr := run(f)
			if fmt.Sprint(refErr) != fmt.Sprint(gotErr) || ref.Seg != got.Seg || ref.IP != got.IP || ref.R != got.R {
				t.Errorf("model %d 0F %02X：Sys 非 nil 時行為改變（%v / %v）", model, op2, refErr, gotErr)
			}
			if f.calls != 0 {
				t.Errorf("model %d 0F %02X：不應呼叫 Sys", model, op2)
			}
			if model == Model8086 && (refErr != nil || ref.Seg[CS] != 0x1234) {
				t.Errorf("Model8086 的 0F 應是 POP CS：CS=%04X err=%v", ref.Seg[CS], refErr)
			}
		}
	}
}
