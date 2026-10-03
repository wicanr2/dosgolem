package dos

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// 這份測試的位移算式一律在測試內以字面算術寫出（(y/2)×80 + (y%2)×2000h），不呼叫被測的
// machine.CGAScanlineOffset，使位移公式的錯誤不會被讀寫互相抵消。

const testCGABase = 0xB8000

func cgaAddr(y, x int) uint32 { return testCGABase + uint32(y/2)*80 + uint32(y%2)*0x2000 + uint32(x) }

// fillPattern 以位元組 (y, x) = (y×7 + x×3 + 1) & FF 填滿 200×80 位元組的畫面（每條掃描線互異）。
func fillPattern(m *machine.Machine) {
	for y := 0; y < 200; y++ {
		for x := 0; x < 80; x++ {
			m.Mem[cgaAddr(y, x)] = byte(y*7 + x*3 + 1)
		}
	}
}

func frameHash(m *machine.Machine) string {
	h := sha256.New()
	for y := 0; y < 200; y++ {
		for x := 0; x < 80; x++ {
			h.Write([]byte{m.Mem[cgaAddr(y, x)]})
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

func int10Scroll(m *machine.Machine, d *DOS, ah, al, bh, ch, cl, dh, dl uint8) {
	m.CPU.R[cpu.BX] = uint16(bh) << 8
	m.CPU.R[cpu.CX] = uint16(ch)<<8 | uint16(cl)
	m.CPU.R[cpu.DX] = uint16(dh)<<8 | uint16(dl)
	call(m, d, 0x10, uint16(ah)<<8|uint16(al))
}

// 獨立向量：tools/cga_scroll_vectors.py 依 DOSBox-X INT10_ScrollWindow 的語意產生，這裡讀同一份 TSV 當字面期望值。
func TestCGAScrollVectors(t *testing.T) {
	f, err := os.Open("testdata/cga_scroll_vectors.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	n := 0
	for sc.Scan() {
		c := strings.Split(sc.Text(), "\t")
		if len(c) != 9 || c[0] == "name" {
			continue
		}
		ahv, _ := strconv.ParseUint(c[1], 16, 8)
		nums := make([]uint8, 6)
		for i := range nums {
			v, err := strconv.ParseUint(c[2+i], 10, 8)
			if err != nil {
				t.Fatalf("%s: %v", c[0], err)
			}
			nums[i] = uint8(v)
		}
		m, d := newTest(t)
		m.SetVideoMode(0x04)
		fillPattern(m)
		int10Scroll(m, d, uint8(ahv), nums[0], nums[1], nums[2], nums[3], nums[4], nums[5])
		if got := frameHash(m); got != c[8] {
			t.Errorf("%s: 畫面雜湊 %s，要 %s", c[0], got, c[8])
		}
		n++
	}
	if n < 20 {
		t.Fatalf("只讀到 %d 個向量", n)
	}
}

// 清除：視窗內全部位元組成為填色，視窗外一個位元組都不變（含左右緣相鄰位元組、上下緣相鄰掃描線、交錯 bank）。
func TestCGAClearWindowTouchesOnlyWindow(t *testing.T) {
	m, d := newTest(t)
	m.SetVideoMode(0x04)
	fillPattern(m)
	int10Scroll(m, d, 0x06, 0, 3, 2, 3, 5, 10) // 列 2–5、欄 3–10，填色 BH=3 → FF
	for y := 0; y < 200; y++ {
		for x := 0; x < 80; x++ {
			inside := y >= 2*8 && y < 6*8 && x >= 3*2 && x < 11*2
			want := byte(y*7 + x*3 + 1)
			if inside {
				want = 0xFF
			}
			if got := m.Mem[cgaAddr(y, x)]; got != want {
				t.Fatalf("(y=%d, x=%d) = %#x，要 %#x（視窗內=%v）", y, x, got, want, inside)
			}
		}
	}
}

// 上捲 1 列：視窗內容往上移 8 條掃描線，底部 1 列填色；視窗外不變。
func TestCGAScrollUpOneRow(t *testing.T) {
	m, d := newTest(t)
	m.SetVideoMode(0x04)
	fillPattern(m)
	int10Scroll(m, d, 0x06, 1, 0, 2, 3, 5, 10)
	for y := 0; y < 200; y++ {
		for x := 0; x < 80; x++ {
			inside := y >= 2*8 && y < 6*8 && x >= 3*2 && x < 11*2
			want := byte(y*7 + x*3 + 1)
			if inside {
				if y < 5*8 {
					sy := y + 8
					want = byte(sy*7 + x*3 + 1)
				} else {
					want = 0
				}
			}
			if got := m.Mem[cgaAddr(y, x)]; got != want {
				t.Fatalf("(y=%d, x=%d) = %#x，要 %#x", y, x, got, want)
			}
		}
	}
}

// 下捲 1 列：往下移，頂部填色（BH=1 → 55h）。
func TestCGAScrollDownOneRowFill(t *testing.T) {
	m, d := newTest(t)
	m.SetVideoMode(0x04)
	fillPattern(m)
	int10Scroll(m, d, 0x07, 1, 1, 2, 3, 5, 10)
	for y := 2 * 8; y < 6*8; y++ {
		for x := 3 * 2; x < 11*2; x++ {
			want := byte(0x55)
			if y >= 3*8 {
				want = byte((y-8)*7 + x*3 + 1)
			}
			if got := m.Mem[cgaAddr(y, x)]; got != want {
				t.Fatalf("(y=%d, x=%d) = %#x，要 %#x", y, x, got, want)
			}
		}
	}
}

// 夾邊與不動作：上緣大於下緣不動作；右下超出時夾到 (39, 24)；CH=30、DH=24（先比較後夾邊）不動作。
func TestCGAScrollClampAndNoop(t *testing.T) {
	m, d := newTest(t)
	m.SetVideoMode(0x04)
	fillPattern(m)
	before := frameHash(m)
	int10Scroll(m, d, 0x06, 0, 0, 5, 3, 3, 10)   // 上緣大於下緣
	int10Scroll(m, d, 0x06, 0, 0, 2, 12, 5, 10)  // 左緣大於右緣
	int10Scroll(m, d, 0x06, 0, 0, 30, 3, 24, 10) // CH=30、DH=24：先比較，不動作（先夾邊會清第 24 列）
	if frameHash(m) != before {
		t.Errorf("不動作的呼叫改了畫面")
	}
	int10Scroll(m, d, 0x06, 0, 0, 20, 30, 40, 79) // 夾到 (39, 24)
	for y := 20 * 8; y < 200; y++ {
		for x := 30 * 2; x < 80; x++ {
			if got := m.Mem[cgaAddr(y, x)]; got != 0 {
				t.Fatalf("夾邊清除後 (y=%d, x=%d) = %#x，要 0", y, x, got)
			}
		}
	}
	// 視窗外保持：視窗上方一條掃描線與左方一個位元組
	y1, x1 := 19*8+7, 30*2
	if got, want := m.Mem[cgaAddr(y1, x1)], byte(y1*7+x1*3+1); got != want {
		t.Errorf("夾邊清除不應動到視窗上方的位元組，得 %#x，要 %#x", got, want)
	}
	y2, x2 := 20*8, 30*2-1
	if got, want := m.Mem[cgaAddr(y2, x2)], byte(y2*7+x2*3+1); got != want {
		t.Errorf("夾邊清除不應動到視窗左方的位元組，得 %#x，要 %#x", got, want)
	}
}

// 非 CGA 模式（mode 13h）：AH=06h 不動視訊記憶體；AH=07h、AH=0Bh 仍記 note。
func TestCGAServicesIgnoredOutsideCGAModes(t *testing.T) {
	m, d := newTest(t)
	m.SetVideoMode(0x13)
	m.Mem[0xA0000+100] = 0x7E
	m.Mem[testCGABase+50] = 0x7E
	int10Scroll(m, d, 0x06, 0, 3, 0, 0, 24, 39)
	if m.Mem[0xA0000+100] != 0x7E || m.Mem[testCGABase+50] != 0x7E {
		t.Errorf("mode 13h 的 AH=06h 不應動視訊記憶體")
	}
	int10Scroll(m, d, 0x07, 1, 0, 0, 0, 24, 39)
	m.CPU.R[cpu.BX] = 0x0001
	call(m, d, 0x10, 0x0B00)
	if n := d.Unimplemented[Call{Int: 0x10, AH: 0x07, AL: 1}]; n != 1 {
		t.Errorf("mode 13h 的 AH=07h 應記一筆 note，得 %d", n)
	}
	if n := d.Unimplemented[Call{Int: 0x10, AH: 0x0B, AL: 0}]; n != 1 {
		t.Errorf("mode 13h 的 AH=0Bh 應記一筆 note，得 %d", n)
	}
	if n := d.Unimplemented[Call{Int: 0x10, AH: 0x06, AL: 3}]; n != 0 {
		t.Errorf("AH=06h 在非 CGA 模式維持收下就好，不記 note，得 %d", n)
	}
}

// AH=0Bh：BH=0 設背景並保留高 3 位元，BH=1 設調色盤並保留其他位元。
func TestCGAAH0BSetsColorSelect(t *testing.T) {
	m, d := newTest(t)
	m.SetVideoMode(0x04)
	if m.ColorSelect() != 0x30 {
		t.Fatalf("設模式後色彩選擇 = %#x", m.ColorSelect())
	}
	m.CPU.R[cpu.BX] = 0x002A // BH=0，BL=2Ah
	call(m, d, 0x10, 0x0B00)
	if got := m.ColorSelect(); got != 0x2A {
		t.Errorf("BH=0 BL=2Ah 後 = %#x，要 2Ah（保留高 3 位元 20h，低 5 位元換成 0Ah，bit 4 一併被覆寫）", got)
	}
	m.CPU.R[cpu.BX] = 0x0100 // BH=1，BL=0：調色盤 0
	call(m, d, 0x10, 0x0B00)
	if got := m.ColorSelect(); got != 0x0A {
		t.Errorf("BH=1 BL=0 後 = %#x，要 0Ah（只清 bit 5）", got)
	}
	m.CPU.R[cpu.BX] = 0x0101 // BH=1，BL=1：調色盤 1
	call(m, d, 0x10, 0x0B00)
	if got := m.ColorSelect(); got != 0x2A {
		t.Errorf("BH=1 BL=1 後 = %#x，要 2Ah（只設 bit 5）", got)
	}
}
