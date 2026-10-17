package dos

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// MSCDEX 的契約測試，規格 `docs/spec/434-mscdex-minimal` §5。
// 期望值照 MSCDEX 2.x 的介面與 DOSBox-X 的回傳慣例自己列。

const testTOC = `
1 data 0
2 audio 8524      # 14,747 frames
3 audio 23271
leadout 42194
`

func newCDTest(t *testing.T) (*machine.Machine, *DOS) {
	t.Helper()
	m, d := newTest(t)
	cd, err := ParseCDTOC(3, testTOC)
	if err != nil {
		t.Fatal(err)
	}
	d.CD = cd
	return m, d
}

// cdReq 在 0000:3000 放一個 device request（IOCTL 的緩衝區在 0000:3100），送 AX=1510h。
func cdReq(m *machine.Machine, d *DOS, fn uint8, fill func(req, buf uint32)) (status uint16, buf uint32) {
	req, buf := uint32(0x3000), uint32(0x3100)
	for i := uint32(0); i < 0x20; i++ {
		m.Write8(req+i, 0)
	}
	m.Write8(req, 0x1A)
	m.Write8(req+2, fn)
	m.Write16(req+0x0E, uint16(buf))
	m.Write16(req+0x10, 0)
	if fill != nil {
		fill(req, buf)
	}
	m.CPU.Seg[cpu.ES], m.CPU.R[cpu.BX], m.CPU.R[cpu.CX] = 0, uint16(req), 3
	call(m, d, 0x2F, 0x1510)
	return m.Read16(req + 3), buf
}

func TestMSCDEXInstallCheckAndDriveList(t *testing.T) {
	m, d := newCDTest(t)
	m.CPU.R[cpu.BX] = 0
	call(m, d, 0x2F, 0x1500)
	if bx, cx := m.CPU.R[cpu.BX], m.CPU.R[cpu.CX]; bx != 1 || cx != 3 {
		t.Fatalf("1500h：BX=%d CX=%d，預期 1 台、第一台是 D:（3）", bx, cx)
	}
	m.CPU.Seg[cpu.ES], m.CPU.R[cpu.BX] = 0, 0x3200
	call(m, d, 0x2F, 0x150D)
	if got := m.Read8(0x3200); got != 3 {
		t.Errorf("150Dh 寫入 %d，預期 3", got)
	}
	call(m, d, 0x2F, 0x150C)
	if bx := m.CPU.R[cpu.BX]; bx < 0x0200 {
		t.Errorf("150Ch 版本 %04X，三國志V 要求至少 0200h", bx)
	}
	m.CPU.R[cpu.CX] = 3
	call(m, d, 0x2F, 0x150B)
	if ax, bx := m.CPU.R[cpu.AX], m.CPU.R[cpu.BX]; ax == 0 || bx != 0xADAD {
		t.Errorf("150Bh D:：AX=%04X BX=%04X，預期非 0 與 ADADh", ax, bx)
	}
	// 1501h：子單位 ＋ far 指標，指到的 header +14h 是磁碟機字母（1 起算）。
	m.CPU.Seg[cpu.ES], m.CPU.R[cpu.BX] = 0, 0x3300
	call(m, d, 0x2F, 0x1501)
	off, seg := m.Read16(0x3301), m.Read16(0x3303)
	hdr := uint32(seg)*16 + uint32(off)
	if got := string([]byte{m.Read8(hdr + 0x0A), m.Read8(hdr + 0x0B), m.Read8(hdr + 0x0C), m.Read8(hdr + 0x0D)}); got != "MSCD" {
		t.Errorf("device header 名稱 %q", got)
	}
	if got := m.Read8(hdr + 0x14); got != 4 {
		t.Errorf("header 的磁碟機字母 %d，預期 4（D:）", got)
	}
}

func TestMSCDEXWithoutDriveIsUnchanged(t *testing.T) {
	m, d := newTest(t) // 沒設 d.CD
	m.CPU.R[cpu.BX] = 0
	call(m, d, 0x2F, 0x1500)
	if bx := m.CPU.R[cpu.BX]; bx != 0 {
		t.Errorf("沒有光碟機時 1500h 回 BX=%d，預期 0", bx)
	}
}

func TestMSCDEXAudioDiskAndTrackInfo(t *testing.T) {
	m, d := newCDTest(t)
	st, buf := cdReq(m, d, 0x03, func(_, buf uint32) { m.Write8(buf, 0x0A) })
	if st != 0x0100 {
		t.Fatalf("IOCTL 0Ah 狀態 %04X", st)
	}
	// leadout 42194 ＋ 150 ＝ 42344 frames ＝ 9:24:44
	if got := []uint8{m.Read8(buf + 1), m.Read8(buf + 2), m.Read8(buf + 5), m.Read8(buf + 4), m.Read8(buf + 3)}; got[0] != 1 || got[1] != 3 || got[2] != 9 || got[3] != 24 || got[4] != 44 {
		t.Errorf("光碟資訊 首末軌 %d %d、leadout %d:%d:%d，預期 1 3、9:24:44", got[0], got[1], got[2], got[3], got[4])
	}
	// 第 2 軌起點 8524 ＋ 150 ＝ 8674 ＝ 1:55:49，音軌 attr 0。
	_, buf = cdReq(m, d, 0x03, func(_, buf uint32) { m.Write8(buf, 0x0B); m.Write8(buf+1, 2) })
	if mm, ss, ff, attr := m.Read8(buf+4), m.Read8(buf+3), m.Read8(buf+2), m.Read8(buf+6); mm != 1 || ss != 55 || ff != 49 || attr != 0 {
		t.Errorf("第 2 軌 %d:%d:%d attr %02X，預期 1:55:49 attr 00", mm, ss, ff, attr)
	}
	_, buf = cdReq(m, d, 0x03, func(_, buf uint32) { m.Write8(buf, 0x0B); m.Write8(buf+1, 1) })
	if attr := m.Read8(buf + 6); attr != 0x40 {
		t.Errorf("第 1 軌 attr %02X，預期資料軌 40", attr)
	}
}

func TestMSCDEXPlayStopResume(t *testing.T) {
	m, d := newCDTest(t)
	// 從第 2 軌（HSG）播 75 frames（1 秒）。
	st, _ := cdReq(m, d, 0x84, func(req, _ uint32) {
		m.Write8(req+0x0D, 0)
		m.Write16(req+0x0E, 8524)
		m.Write16(req+0x12, 75)
	})
	if st&0x0200 == 0 {
		t.Fatalf("播放後狀態 %04X，預期 BUSY（0200h）", st)
	}
	// 半秒後暫停：位置前進了，狀態不再 BUSY。
	m.Steps += uint64(m.StepsPerSecondNow() / 2)
	st, _ = cdReq(m, d, 0x85, nil)
	if st&0x0200 != 0 {
		t.Errorf("STOP 之後狀態 %04X 還是 BUSY", st)
	}
	if !d.CD.paused || d.CD.start <= 8524 {
		t.Errorf("第一次 STOP 要暫停並記下位置：paused=%v start=%d", d.CD.paused, d.CD.start)
	}
	// RESUME：再播；播完剩下的時間後自己停。
	st, _ = cdReq(m, d, 0x88, nil)
	if st&0x0200 == 0 {
		t.Errorf("RESUME 之後狀態 %04X，預期 BUSY", st)
	}
	m.Steps += uint64(m.StepsPerSecondNow())
	_, buf := cdReq(m, d, 0x03, func(_, buf uint32) { m.Write8(buf, 0x06) })
	if st := m.Read16(buf + 1); st&(1<<10) != 0 {
		t.Errorf("播完之後裝置狀態 %04X 還寫播放中", st)
	}
	// 已停時再 STOP 是重設。
	cdReq(m, d, 0x85, nil)
	cdReq(m, d, 0x85, nil)
	if d.CD.paused || d.CD.start != 0 {
		t.Errorf("已停時 STOP 要重設：paused=%v start=%d", d.CD.paused, d.CD.start)
	}
}

func TestParseCDTOCRejectsGaps(t *testing.T) {
	if _, err := ParseCDTOC(3, "1 data 0\n3 audio 100\nleadout 200\n"); err == nil {
		t.Error("軌號跳號竟然接受")
	}
	if _, err := ParseCDTOC(3, "1 data 0\n"); err == nil {
		t.Error("沒有 leadout 竟然接受")
	}
}

// `AX=4408h`（`docs/spec/435-ioctl-removable-media`）：硬碟回 1、光碟機與不存在的代號設 CF。
func TestIOCTLRemovableMedia(t *testing.T) {
	m, d := newCDTest(t)
	cases := []struct {
		name string
		bl   uint8
		cf   bool
		ax   uint16
	}{
		{"預設磁碟機（C:）", 0, false, 1},
		{"C:", 3, false, 1},
		{"D:（光碟）", 4, true, 0x01},
		{"A:（不存在）", 1, true, 0x0F},
		{"Z:（不存在）", 26, true, 0x0F},
	}
	for _, tc := range cases {
		m.CPU.R[cpu.BX] = uint16(tc.bl)
		m.CPU.SetFlags(m.CPU.Flags &^ cpu.CF)
		call(m, d, 0x21, 0x4408)
		cf := m.CPU.Flags&cpu.CF != 0
		if cf != tc.cf || m.CPU.R[cpu.AX] != tc.ax {
			t.Errorf("%s：CF=%v AX=%04X，預期 CF=%v AX=%04X", tc.name, cf, m.CPU.R[cpu.AX], tc.cf, tc.ax)
		}
	}
}
