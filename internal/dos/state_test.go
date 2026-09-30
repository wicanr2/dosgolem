package dos

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/internal/machine"
)

// newAt 造一台素材目錄指定的機器。
func newAt(t *testing.T, root string) *DOS {
	t.Helper()
	m := machine.New()
	d := New(m, root)
	d.Install()
	d.freeSeg = 0x2000
	return d
}

// 狀態檔要把素材目錄一起存進去。少了它，讀檔之後 resolve 會拿預設的 `.`
// 去找，之後每一次開檔都失敗——**而且不會有任何錯誤**，緩衝區裡留著填充值，
// 遊戲照樣把它當資料用。這一條是在源平合戰對拍時踩到的。
func TestStateKeepsRootSoLaterOpensStillFindFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "KAODATA.GP"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := newAt(t, root).SaveState(&buf); err != nil {
		t.Fatal(err)
	}

	// 讀進一台素材目錄完全不同的機器：讀檔本身不會失敗，
	// 所以只能從「之後還開不開得到檔」看出來。
	d := newAt(t, t.TempDir())
	if err := d.LoadState(bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatal(err)
	}
	if d.Root != root {
		t.Errorf("讀檔後素材目錄是 %q，要 %q", d.Root, root)
	}
	if got := d.resolve(`C:KAODATA.GP`); got == "" {
		t.Error("讀檔後開不到 C:KAODATA.GP")
	}
}

// 舊的狀態檔沒有 Root 欄位，讀進來會是空字串；那時要沿用目前這一台的設定，
// 不要把它清成空的。
func TestOldStateWithoutRootKeepsCurrentRoot(t *testing.T) {
	var buf bytes.Buffer
	d := newAt(t, t.TempDir())
	d.Root = "" // 模擬舊檔：那時候還沒有這個欄位
	if err := d.SaveState(&buf); err != nil {
		t.Fatal(err)
	}

	here := t.TempDir()
	e := newAt(t, here)
	if err := e.LoadState(bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatal(err)
	}
	if e.Root != here {
		t.Errorf("素材目錄變成 %q，要維持 %q", e.Root, here)
	}
}

// 狀態檔要留住 `AX=000Ch` 的事件常式與座標範圍（`docs/spec/196-mouse-handler-survives-state-file`）。
// 遊戲只在開機登記一次事件常式；讀檔後少了它，注入的輸入不會呼叫常式，
// 靠常式畫游標的遊戲看起來像滑鼠沒接上。
func TestStateKeepsMouseHandlerAndRange(t *testing.T) {
	d := newAt(t, t.TempDir())
	d.Mouse.Handler.Seg, d.Mouse.Handler.Off = 0x1234, 0x0056
	d.Mouse.Handler.Mask, d.Mouse.Handler.Set = 0x1F, true
	d.Mouse.MinX, d.Mouse.MaxX, d.Mouse.MinY, d.Mouse.MaxY = 1, 0x27F, 2, 0x18F
	d.Mouse.PressAt[0] = [2]uint16{10, 20}
	d.Mouse.ReleaseAt[1] = [2]uint16{30, 40}
	var buf bytes.Buffer
	if err := d.SaveState(&buf); err != nil {
		t.Fatal(err)
	}

	e := newAt(t, t.TempDir())
	if err := e.LoadState(bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatal(err)
	}
	if e.Mouse.Handler != d.Mouse.Handler {
		t.Errorf("事件常式 %+v，要 %+v", e.Mouse.Handler, d.Mouse.Handler)
	}
	if e.Mouse.MinX != 1 || e.Mouse.MaxX != 0x27F || e.Mouse.MinY != 2 || e.Mouse.MaxY != 0x18F {
		t.Errorf("範圍 %d-%d × %d-%d 沒還原", e.Mouse.MinX, e.Mouse.MaxX, e.Mouse.MinY, e.Mouse.MaxY)
	}
	if e.Mouse.PressAt != d.Mouse.PressAt || e.Mouse.ReleaseAt != d.Mouse.ReleaseAt {
		t.Errorf("按放位置 %v／%v 沒還原", e.Mouse.PressAt, e.Mouse.ReleaseAt)
	}
}
