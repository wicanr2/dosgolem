package phantasie

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"
)

// 合成 COM 只提供已核對的入口簽章與最小 BIOS helper，不含遊戲資料。
// 正常遊玩驗收另跑專案路線，本合成程式驗證 guard 的邊界與 IRQ 去重。
func waitFixture(t *testing.T, entry uint16, smallSP bool, badSig bool) (*oracle.Oracle, *KeyGate, uint16) {
	t.Helper()
	code := make([]byte, 0x5000)
	put := func(off uint16, b []byte) { copy(code[int(off)-0x100:], b) }
	main := []byte{0xE8, 0, 0, 0xEB, 0xFE}
	if smallSP {
		main = append([]byte{0xBC, 0x04, 0}, main...)
	}
	call := 0x100
	if smallSP {
		call += 3
	}
	binary.LittleEndian.PutUint16(main[len(main)-4:], entry-uint16(call+3))
	put(0x100, main)
	sig := append([]byte(nil), waitKeySignature...)
	if badSig {
		sig[1] = 0x90
	}
	put(OffWaitKey, sig)
	// 清空已排入 BIOS 的鍵，再返回。
	put(0x3933, []byte{0xB4, 1, 0xCD, 0x16, 0x74, 6, 0xB4, 0, 0xCD, 0x16, 0xEB, 0xF4, 0xC3})
	// int86 的最小替身：標準 BIOS AH00 讀鍵，實際消耗端不由 KeyGate 計數。
	put(0x4E60, []byte{0xB4, 0, 0xCD, 0x16, 0xC3})
	path := filepath.Join(t.TempDir(), "wait.com")
	if err := os.WriteFile(path, code, 0600); err != nil {
		t.Fatal(err)
	}
	o, err := oracle.LoadProgram(path, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(o.Close)
	img := o.IP().Seg
	return o, NewKeyGate(o, img), img
}

func reachWait(t *testing.T, o *oracle.Oracle) {
	t.Helper()
	var w *oracle.InputWaitError
	if err := o.Run(1000); !errors.As(err, &w) {
		t.Fatalf("等待=%v", err)
	}
}

func TestWaitGateHoldAndResume(t *testing.T) {
	for _, wrap := range []bool{false, true} {
		t.Run(map[bool]string{false: "normal", true: "wrapped-SP"}[wrap], func(t *testing.T) {
			o, g, img := waitFixture(t, OffWaitKey, wrap, false)
			reachWait(t, o)
			regs := o.Regs()
			mem := o.Bytes(oracle.Far(0, 0), 1<<20)
			step, tick, reads := o.Steps(), o.Ticks(), g.Reads
			for i := 0; i < 4; i++ {
				reachWait(t, o)
				if o.Regs() != regs || !bytes.Equal(mem, o.Bytes(oracle.Far(0, 0), 1<<20)) || o.Steps() != step || o.Ticks() != tick || g.Reads != reads || g.Gated != 0 || g.Dups != 0 || o.KeysPending() != 0 || o.KeysConsumed() != 0 {
					t.Fatal("無鍵停留改變狀態")
				}
			}
			g.Press(0, "Return")
			if err := o.RunUntil(oracle.At(oracle.Far(img, OffWaitKeyDone)), oracle.Budget(1000)); err != nil {
				t.Fatal(err)
			}
			if g.Gated != 1 || g.LastSent != reads || o.KeysConsumed() != 1 || o.KeysPending() != 0 {
				t.Fatalf("gated=%d consumed=%d pending=%d", g.Gated, o.KeysConsumed(), o.KeysPending())
			}
			if err := o.Run(5); err != nil || g.waitOpen {
				t.Fatalf("return=%v open=%v", err, g.waitOpen)
			}
		})
	}
}

func TestWaitGateRejectsIdentityAndQueueErrors(t *testing.T) {
	t.Run("signature", func(t *testing.T) {
		o, g, img := waitFixture(t, OffWaitKey, false, true)
		err := o.Run(20)
		var w *oracle.InputWaitError
		if err == nil || errors.As(err, &w) || o.IP().Linear() != oracle.Far(img, OffWaitKey).Linear() || g.Reads != 0 {
			t.Fatalf("signature=%v ip=%s reads=%d", err, o.IP(), g.Reads)
		}
	})
	t.Run("unopened", func(t *testing.T) {
		o, _, _ := waitFixture(t, OffWaitKeyCall, false, false)
		if err := o.Run(20); err == nil {
			t.Fatal("未開啟竟可繼續")
		}
	})
	for _, kind := range []string{"future", "skip", "invalid", "wrong-SP"} {
		t.Run(kind, func(t *testing.T) {
			o, g, _ := waitFixture(t, OffWaitKey, false, false)
			reachWait(t, o)
			switch kind {
			case "future":
				g.Press(o.Steps()+1, "Return")
			case "skip":
				g.PressAfterReads(1, "Return")
			case "invalid":
				g.Press(0, "not-a-key")
			case "wrong-SP":
				g.waitSP++
			}
			before := o.Regs()
			mem := o.Bytes(oracle.Far(0, 0), 1<<20)
			pending := g.Pending()
			reads := g.Reads
			for i := 0; i < 2; i++ {
				err := o.Run(10)
				var w *oracle.InputWaitError
				if err == nil || errors.As(err, &w) || o.Regs() != before || !bytes.Equal(mem, o.Bytes(oracle.Far(0, 0), 1<<20)) || g.Pending() != pending || g.Reads != reads || g.Gated != 0 || o.KeysPending() != 0 {
					t.Fatalf("%s 非原子拒絕：%v", kind, err)
				}
			}
		})
	}
}

func TestWaitGateIRQDoesNotResend(t *testing.T) {
	// 先量出合成函式到停點所需步數，再以合成自迴圈對齊第一個 IRQ0。
	a, _, _ := waitFixture(t, OffWaitKey, false, false)
	reachWait(t, a)
	cost := a.Steps() - 1
	o, g, img := waitFixture(t, OffWaitKey, false, false)
	// 在合成資料內測試用 CallNear，不作正常玩家路線證據。
	o.SetStepGuard(nil)
	// 第一個 call 還未執行，改走合成入口末尾的自迴圈。
	o.CallNear(oracle.Far(img, 0x103), 0x103, oracle.CallRegs{})
	target := o.TickRate() - cost
	if err := o.Run(target); err != nil {
		t.Fatal(err)
	}
	o.SetStepGuard(g.guard)
	o.CallNear(oracle.Far(img, OffWaitKey), 0x103, oracle.CallRegs{})
	reachWait(t, o)
	if o.Steps() != o.TickRate() {
		t.Fatalf("未對齊 IRQ：step=%d rate=%d", o.Steps(), o.TickRate())
	}
	reads := g.Reads
	g.Press(0, "Return")
	if err := o.Run(1); err != nil {
		t.Fatal(err)
	}
	if o.Ticks() != 1 || o.IP().Linear() == oracle.Far(img, OffWaitKeyCall).Linear() {
		t.Fatal("IRQ 未先攔下 call")
	}
	if err := o.RunUntil(oracle.At(oracle.Far(img, OffWaitKeyDone)), oracle.Budget(1000)); err != nil {
		t.Fatal(err)
	}
	if g.Reads != reads || g.Gated != 1 || o.KeysConsumed() != 1 || o.KeysPending() != 0 {
		t.Fatalf("IRQ 重送：reads=%d gated=%d consumed=%d pending=%d", g.Reads, g.Gated, o.KeysConsumed(), o.KeysPending())
	}
}
