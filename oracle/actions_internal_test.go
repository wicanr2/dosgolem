package oracle

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// `docs/spec/201`：逐步操作。合成的 `jmp $` 程式（`liveOracle`，定義在
// `live_internal_test.go`），不需要原版素材。

// §3 第 1 項：ParseActions 解出正確的項數、時間與鍵；壞輸入整串回錯。
func TestParseActionsValid(t *testing.T) {
	acts, err := ParseActions("tap:Up,wait:500,hold:Space:2000,type:ab")
	if err != nil {
		t.Fatal(err)
	}
	if len(acts) != 7 {
		t.Fatalf("解出 %d 項，要 1＋1＋1＋4＝7 項：%+v", len(acts), acts)
	}
	up, _ := dos.KeyNamed("Up")
	space, _ := dos.KeyNamed("Space")
	a, _ := dos.KeyForRune('a')
	b, _ := dos.KeyForRune('b')
	want := []Action{
		{Kind: ActionTap, Scan: up.Scan, MS: 150},
		{Kind: ActionWait, MS: 500},
		{Kind: ActionHold, Scan: space.Scan, MS: 2000},
		{Kind: ActionTap, Scan: a.Scan, MS: 150},
		{Kind: ActionWait, MS: 150},
		{Kind: ActionTap, Scan: b.Scan, MS: 150},
		{Kind: ActionWait, MS: 150},
	}
	for i, w := range want {
		if acts[i] != w {
			t.Errorf("第 %d 項 = %+v，要 %+v", i, acts[i], w)
		}
	}

	// 不分大小寫（`dos.KeyNamed` 本身區分大小寫，`201` §2.1 要求不分）。
	lower, err := ParseActions("tap:up:150")
	if err != nil {
		t.Fatal(err)
	}
	if len(lower) != 1 || lower[0].Scan != up.Scan {
		t.Errorf("小寫鍵名 %+v，要跟 Up 一樣是 %02X", lower, up.Scan)
	}
}

// §3 第 1 項：認不得的鍵、ms ≤ 0、未知動作，整串回錯。
func TestParseActionsRejectsWholeString(t *testing.T) {
	for _, s := range []string{"tap:Nope", "wait:0", "wait:-5", "jump:1", "hold:Space"} {
		if acts, err := ParseActions(s); err == nil {
			t.Errorf("%q 應該回錯，卻解出 %+v", s, acts)
		}
	}
}

// §3 第 2 項：wait:1000 在 750 cycles/ms 下跑了 750,000 個 cycle；
// chunkMs 16.67（預設）時 onChunk 呼叫 60 次。
func TestRunActionsWaitCyclesAndChunkCount(t *testing.T) {
	o := liveOracle(t)
	o.SetDOSBoxCycles(machine.CyclesAT8)
	acts, err := ParseActions("wait:1000")
	if err != nil {
		t.Fatal(err)
	}
	before := o.Cycles()
	chunks := 0
	if err := o.RunActions(acts, 0, func() { chunks++ }); err != nil {
		t.Fatal(err)
	}
	if d := o.Cycles() - before; d != 750_000 { // jmp $ 一道 1 個 cycle，剛好停在目標
		t.Errorf("跑了 %d 個 cycle，要 750,000", d)
	}
	if chunks != 60 {
		t.Errorf("onChunk 呼叫 %d 次，要 60 次（16.67 ms 一段）", chunks)
	}
}

// 沒開 DOSBox cycles 時整個呼叫要回錯誤，而且不能有副作用（不按鍵）。
func TestRunActionsRequiresDOSBoxCycles(t *testing.T) {
	o := liveOracle(t)
	acts, err := ParseActions("tap:Up:150")
	if err != nil {
		t.Fatal(err)
	}
	if err := o.RunActions(acts, 0, nil); err == nil {
		t.Fatal("沒開 DOSBox cycles 卻沒有回錯誤")
	}
	if len(o.m.ScheduledKeys()) != 0 {
		t.Errorf("回錯之前不該有任何按鍵排程：%v", o.m.ScheduledKeys())
	}
}

// §3 第 3 項：tap:Up:150 排出一個 48h 按下碼與一個 C8h 放開碼，
// 相隔 150 ms 份的步數（±1 段）。
func TestRunActionsTapSchedulesPressAndRelease(t *testing.T) {
	o := liveOracle(t)
	o.SetDOSBoxCycles(machine.CyclesXT)
	acts, err := ParseActions("tap:Up:150")
	if err != nil {
		t.Fatal(err)
	}
	if err := o.RunActions(acts, 0, nil); err != nil {
		t.Fatal(err)
	}
	ks := o.m.ScheduledKeys()
	var press, release *machine.ScheduledKey
	for i := range ks {
		switch {
		case ks[i].Scan == 0x48 && !ks[i].Break && press == nil:
			press = &ks[i]
		case ks[i].Scan == 0x48 && ks[i].Break:
			release = &ks[i]
		}
	}
	if press == nil || release == nil {
		t.Fatalf("排程裡沒有 48h 按下碼或 C8h 放開碼：%+v", ks)
	}
	chunkCycles := uint64(math.Round(1000.0 / 60.0 * float64(machine.CyclesXT)))
	want := uint64(math.Round(150 * float64(machine.CyclesXT)))
	gap := release.Step - press.Step
	if d := int64(gap) - int64(want); d < -int64(chunkCycles) || d > int64(chunkCycles) {
		t.Errorf("按下到放開相隔 %d 步，要 %d ±%d（一段）", gap, want, chunkCycles)
	}
}

// §3 第 4 項：同一狀態、同一腳本跑兩次，存出的狀態檔逐位元組相同。
//
// ⚠ **不能用 `liveOracle(t)`。** 那支每次呼叫各自 `t.TempDir()` 兩次
// （執行檔目錄與素材目錄各一次），兩次呼叫拿到的目錄字串必然不同；
// `d.Root` 是狀態檔的一部分（讀檔之後遊戲還要再開檔，見 `199` §3.4），
// 字串不同，兩份狀態檔就不會逐位元組相同——即使 CPU、記憶體、按鍵排程
// 完全一樣，也會被誤判成「不是決定性的」。這裡固定用同一組 exe／root
// 字串各 Load 一次，只讓「動作腳本」這個變數存在。
func TestRunActionsDeterministic(t *testing.T) {
	exePath, root := synthExeAndRoot(t)
	run := func() []byte {
		o, err := Load(exePath, root)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(o.Close)
		o.SetDOSBoxCycles(machine.CyclesAT8)
		acts, err := ParseActions("tap:Up:150,wait:300,hold:Space:500,down:Left,wait:100,up:Left,type:hi")
		if err != nil {
			t.Fatal(err)
		}
		if err := o.RunActions(acts, 0, nil); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "s.state")
		if err := o.SaveStateFile(path); err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	a, b := run(), run()
	if !bytes.Equal(a, b) {
		t.Error("同一狀態、同一腳本跑兩次，狀態檔不是逐位元組相同")
	}
}

// synthExeAndRoot 造一組固定的（執行檔路徑, 素材目錄）給決定性測試用——
// 兩次要用**同一組**字串，見上面 TestRunActionsDeterministic 的說明。
// 執行檔內容與 liveOracle 相同（整段 `jmp $`），只是不各自 t.TempDir()。
func synthExeAndRoot(t *testing.T) (exePath, root string) {
	t.Helper()
	code := make([]byte, 0x400)
	copy(code, []byte{0xEB, 0xFE}) // jmp $
	hdr := make([]byte, 32)
	put := func(off int, v uint16) { binary.LittleEndian.PutUint16(hdr[off:], v) }
	total := len(hdr) + len(code)
	copy(hdr, "MZ")
	put(0x02, uint16(total%512))
	put(0x04, uint16((total+511)/512))
	put(0x08, 2)
	put(0x0A, 0x10)
	put(0x0C, 0xFFFF)
	put(0x10, 0x0400)
	put(0x18, 0x001C)
	dir := t.TempDir()
	exePath = filepath.Join(dir, "LIVE.EXE")
	if err := os.WriteFile(exePath, append(hdr, code...), 0o644); err != nil {
		t.Fatal(err)
	}
	root = filepath.Join(dir, "root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	return exePath, root
}
