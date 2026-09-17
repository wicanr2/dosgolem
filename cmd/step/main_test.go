package main

import (
	"encoding/binary"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/oracle"
)

// synthExe 造一支最小的 MZ 執行檔：整個 code 段是 `jmp $`（每道指令固定
// 1 個 DOSBox cycle），足夠驗證 cycles 記帳，不需要原版素材。
//
// 寫法照抄 `oracle/live_internal_test.go` 的 liveOracle：那是另一個套件的
// internal 測試，這裡碰不到，只好自己造一份。
func synthExe(t *testing.T) string {
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
	path := filepath.Join(t.TempDir(), "STEP.EXE")
	if err := os.WriteFile(path, append(hdr, code...), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// §3 第 5 項：`-do "wait:100" -save-state a` 再 `-load-state a -do
// "wait:100" -save-state b`，與一次 `-do "wait:100,wait:100"` 的狀態檔
// cycles 相同。
func TestRunStepTwoCallsMatchOneCall(t *testing.T) {
	exe := synthExe(t)
	root := t.TempDir()
	dir := t.TempDir()
	a := filepath.Join(dir, "a.state")
	b := filepath.Join(dir, "b.state")
	one := filepath.Join(dir, "one.state")

	if _, err := runStep(stepConfig{exe: exe, root: root, perMs: machine.CyclesAT8, do: "wait:100", saveState: a}); err != nil {
		t.Fatal(err)
	}
	if _, err := runStep(stepConfig{exe: exe, root: root, loadState: a, perMs: machine.CyclesAT8, do: "wait:100", saveState: b}); err != nil {
		t.Fatal(err)
	}
	if _, err := runStep(stepConfig{exe: exe, root: root, perMs: machine.CyclesAT8, do: "wait:100,wait:100", saveState: one}); err != nil {
		t.Fatal(err)
	}

	cyclesOf := func(path string) uint64 {
		t.Helper()
		o, err := oracle.Load(exe, root)
		if err != nil {
			t.Fatal(err)
		}
		defer o.Close()
		if err := o.LoadStateFile(path); err != nil {
			t.Fatal(err)
		}
		return o.Cycles()
	}
	cb, co := cyclesOf(b), cyclesOf(one)
	if cb != co {
		t.Errorf("分兩次 step 的 cycles %d，一次 step 的 cycles %d，要相同", cb, co)
	}
	if cb == 0 {
		t.Fatal("cycles 是 0，測試本身沒有量到東西")
	}
}

// runStep 的摘要：步數、cycles 是絕對值，機器毫秒是這一步自己的量。
func TestRunStepSummary(t *testing.T) {
	exe := synthExe(t)
	root := t.TempDir()
	res, err := runStep(stepConfig{exe: exe, root: root, perMs: machine.CyclesXT, do: "wait:200"})
	if err != nil {
		t.Fatal(err)
	}
	if res.deltaCycles != 200*machine.CyclesXT {
		t.Errorf("本步 cycles %d，要 %d", res.deltaCycles, 200*uint64(machine.CyclesXT))
	}
	if d := res.ms - 200; d < -0.01 || d > 0.01 {
		t.Errorf("機器毫秒 %.3f，要 200", res.ms)
	}
	if res.exited {
		t.Error("jmp $ 不會自己結束，exited 不該是 true")
	}
}

// -shot 以 nearest 放大存成 PNG：尺寸要是畫面的 scale 倍。
func TestRunStepShotScales(t *testing.T) {
	exe := synthExe(t)
	root := t.TempDir()
	dir := t.TempDir()
	shot := filepath.Join(dir, "s.png")
	if _, err := runStep(stepConfig{exe: exe, root: root, perMs: machine.CyclesXT, do: "wait:10", shot: shot, scale: 3}); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(shot)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	if b.Dx() != oracle.Width*3 || b.Dy() != oracle.Height*3 {
		t.Errorf("PNG 尺寸 %dx%d，要 %dx%d（scale 3）", b.Dx(), b.Dy(), oracle.Width*3, oracle.Height*3)
	}
}

// 認不得的旗標組合（-do 看不懂）要回錯，不要 panic 或安靜地跑掉。
func TestRunStepRejectsBadScript(t *testing.T) {
	exe := synthExe(t)
	root := t.TempDir()
	if _, err := runStep(stepConfig{exe: exe, root: root, perMs: machine.CyclesXT, do: "jump:1"}); err == nil {
		t.Fatal("-do 看不懂的動作卻沒有回錯")
	}
}
