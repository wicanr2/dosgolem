package machine

import (
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// `.COM` 的載入規則全部是「錯了也不會報錯」型：段暫存器指錯、堆疊放錯、
// 進入點差 0x100，程式都照跑，只是跑進垃圾。所以每一條都釘住。

func TestLoadCOMPlacesImageAtEntry(t *testing.T) {
	m := New()
	img := []byte{0xB8, 0x34, 0x12, 0xCB} // mov ax,1234h / retf
	if err := m.LoadCOM(img); err != nil {
		t.Fatal(err)
	}
	base := uint32(PSPSeg)*16 + 0x100
	for i, b := range img {
		if got := m.Read8(base + uint32(i)); got != b {
			t.Fatalf("映像第 %d 個 byte 是 %02X，該是 %02X", i, got, b)
		}
	}
	if m.ImageBase != base || m.ImageLen != len(img) {
		t.Errorf("ImageBase/Len = %05X/%d，該是 %05X/%d", m.ImageBase, m.ImageLen, base, len(img))
	}
}

func TestLoadCOMPointsEverySegmentAtThePSP(t *testing.T) {
	m := New()
	if err := m.LoadCOM([]byte{0xCB}); err != nil {
		t.Fatal(err)
	}
	c := m.CPU
	for _, s := range []struct {
		name string
		idx  int
	}{{"CS", cpu.CS}, {"DS", cpu.DS}, {"ES", cpu.ES}, {"SS", cpu.SS}} {
		if c.Seg[s.idx] != PSPSeg {
			t.Errorf("%s = %04X，該是 %04X", s.name, c.Seg[s.idx], PSPSeg)
		}
	}
	if c.IP != 0x100 {
		t.Errorf("IP = %04X，該是 0100", c.IP)
	}
	if c.R[cpu.SP] != 0xFFFE {
		t.Errorf("SP = %04X，該是 FFFE", c.R[cpu.SP])
	}
	// 進入點之下要有一個 0：程式做 retn 時才會落到 PSP 開頭的 int 20h。
	if v := m.Read16(uint32(PSPSeg)*16 + 0xFFFE); v != 0 {
		t.Errorf("堆疊頂是 %04X，該是 0000", v)
	}
}

func TestLoadCOMBuildsPSPAndLeavesRoomAfterTheSegment(t *testing.T) {
	m := New()
	if err := m.LoadCOM([]byte{0xCB}); err != nil {
		t.Fatal(err)
	}
	// PSP 開頭是 int 20h：`.COM` 的 retn 慣例靠它。
	if a, b := m.Read8(uint32(PSPSeg)*16), m.Read8(uint32(PSPSeg)*16+1); a != 0xCD || b != 0x20 {
		t.Errorf("PSP 開頭是 %02X %02X，該是 CD 20", a, b)
	}
	// `.COM` 名義上擁有整個段，可配置區要在它後面。
	if m.FreeSeg < PSPSeg+0x1000 {
		t.Errorf("FreeSeg = %04X，落在程式自己的段裡", m.FreeSeg)
	}
}

func TestLoadCOMRejectsWhatItCannotPlace(t *testing.T) {
	for _, tt := range []struct {
		name string
		img  []byte
		want string
	}{
		{"空檔", nil, "空的"},
		{"塞不進一個段", make([]byte, 0xFF00), "塞不進"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := New().LoadCOM(tt.img)
			if err == nil {
				t.Fatal("預期要有錯誤，卻成功了")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("錯誤訊息 %q 沒有提到 %q", err, tt.want)
			}
		})
	}
}
