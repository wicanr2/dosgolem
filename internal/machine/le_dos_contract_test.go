package machine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu386"
	"github.com/wicanr2/dosgolem/internal/dosfile"
)

// 保護模式 DOS 服務層的契約測試（`docs/spec/184-mvp-scope-review` 批次 3）。
//
// 這一批不是 FD2 專屬的：任何一支 DOS/4GW 程式都會踩到主控台輸出、關檔、
// 結束。期望值照 DOS 的定義自己列。

// newPMDOS 造一台只有一小段記憶體的 32 位元機器與服務層。
func newPMDOS(t *testing.T) (*cpu386.CPU, *FD2StartupDOS) {
	t.Helper()
	bus := startupBus(make([]byte, 0x400))
	c := cpu386.New(bus)
	c.Seg[cpu386.SegDS] = 0x160
	c.SetDescriptor(0x160, cpu386.Descriptor{Limit: 0x3FF, Writable: true})
	return c, &FD2StartupDOS{}
}

// putString 把一段位元組放進 DS 指到的記憶體。
func putString(c *cpu386.CPU, off uint32, s string) {
	c.WriteSegmentBytes(c.Seg[cpu386.SegDS], off, []byte(s))
}

// `AH=40h` 寫到 handle 1／2 要收進 Console。
//
// 不收的話，程式印的錯誤訊息全部消失——看起來像「它什麼都沒說就停了」，
// 而實際上它正在解釋自己為什麼停。
func TestPMWriteReachesConsole(t *testing.T) {
	c, s := newPMDOS(t)
	putString(c, 0x100, "hello")
	c.R[cpu386.EAX], c.R[cpu386.EBX] = 0x4000, 1
	c.R[cpu386.ECX], c.R[cpu386.EDX] = 5, 0x100
	if !s.Handle(c, 0x21) {
		t.Fatal("AH=40h 沒實作")
	}
	if c.EFlags&cpu386.CF != 0 {
		t.Fatalf("寫失敗：EAX=%08X", c.R[cpu386.EAX])
	}
	if got := string(s.Console); got != "hello" {
		t.Fatalf("Console 是 %q，預期 hello", got)
	}
	if got := uint16(c.R[cpu386.EAX]); got != 5 {
		t.Errorf("回報寫了 %d 個位元組，預期 5", got)
	}

	// stderr（handle 2）走同一條。
	putString(c, 0x100, "!")
	c.R[cpu386.EAX], c.R[cpu386.EBX] = 0x4000, 2
	c.R[cpu386.ECX] = 1
	s.Handle(c, 0x21)
	if got := string(s.Console); got != "hello!" {
		t.Errorf("stderr 之後 Console 是 %q，預期 hello!", got)
	}
}

// 寫到檔案 handle 要**失敗**，不能假裝成功。
//
// 這一層的檔案提供者是唯讀的。回成功的話程式以為資料落地了，
// 接著把它讀回來，然後拿到舊內容——而它完全不會懷疑寫入那一步。
func TestPMWriteToFileHandleFails(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "A.DAT"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	provider, err := OpenDirectoryReadOnlyFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { provider.Close() })

	bus := startupBus(make([]byte, 0x400))
	c := cpu386.New(bus)
	c.Seg[cpu386.SegDS] = 0x160
	c.SetDescriptor(0x160, cpu386.Descriptor{Limit: 0x3FF, Writable: true})
	s := NewFD2StartupDOS(provider)
	t.Cleanup(func() { s.Close() })

	putString(c, 0x100, "A.DAT\x00")
	c.R[cpu386.EAX], c.R[cpu386.EDX] = 0x3d00, 0x100
	if !s.Handle(c, 0x21) || c.EFlags&cpu386.CF != 0 {
		t.Fatalf("開檔失敗：EAX=%08X", c.R[cpu386.EAX])
	}
	handle := uint32(uint16(c.R[cpu386.EAX]))

	c.R[cpu386.EAX], c.R[cpu386.EBX] = 0x4000, handle
	c.R[cpu386.ECX], c.R[cpu386.EDX] = 3, 0x100
	s.Handle(c, 0x21)
	if c.EFlags&cpu386.CF == 0 {
		t.Fatal("寫進唯讀的檔案 handle 竟然成功")
	}
	if got := uint16(c.R[cpu386.EAX]); got != dosfile.ErrAccessDenied {
		t.Errorf("錯誤碼 %d，預期 %d（拒絕存取）", got, dosfile.ErrAccessDenied)
	}
	if data, _ := os.ReadFile(filepath.Join(root, "A.DAT")); string(data) != "old" {
		t.Errorf("檔案內容變成 %q——唯讀的來源目錄被動到了", data)
	}
}

// `AH=09h` 的字串**結尾是 `$`**，不是 NUL。
//
// 當成 NUL 結尾的話，`$` 之後那一段會一起印出來（多半是下一個字串），
// 而畫面上看起來像「訊息接錯了」。
func TestPMPrintStringStopsAtDollar(t *testing.T) {
	c, s := newPMDOS(t)
	putString(c, 0x100, "Error 42$SECRET\x00")
	c.R[cpu386.EAX], c.R[cpu386.EDX] = 0x0900, 0x100
	if !s.Handle(c, 0x21) {
		t.Fatal("AH=09h 沒實作")
	}
	if got := string(s.Console); got != "Error 42" {
		t.Fatalf("Console 是 %q，預期 Error 42", got)
	}
}

// `AH=02h` 的字元在 **DL**，不是 AL。
func TestPMCharOutReadsDL(t *testing.T) {
	c, s := newPMDOS(t)
	c.R[cpu386.EAX], c.R[cpu386.EDX] = 0x0200, 'Z'
	if !s.Handle(c, 0x21) {
		t.Fatal("AH=02h 沒實作")
	}
	if got := string(s.Console); got != "Z" {
		t.Errorf("Console 是 %q，預期 Z——字元讀錯暫存器了", got)
	}
}

// `AH=3Eh` 關過的 handle 要真的不見。
func TestPMCloseInvalidatesHandle(t *testing.T) {
	c, s := newPMDOS(t)
	c.R[cpu386.EAX], c.R[cpu386.EBX] = 0x3e00, 99
	if !s.Handle(c, 0x21) {
		t.Fatal("AH=3Eh 沒實作")
	}
	if c.EFlags&cpu386.CF == 0 {
		t.Fatal("關一個不存在的 handle 竟然成功")
	}
	if got := uint16(c.R[cpu386.EAX]); got != dosfile.ErrInvalidHandle {
		t.Errorf("錯誤碼 %d，預期 %d", got, dosfile.ErrInvalidHandle)
	}
}

// `AH=4Ch` 要記下來，而且帶回離開碼。
//
// 只清 CF 就回去的話，CPU 會從程式結束之後那一段記憶體繼續執行——
// 那裡的位元組不再是有意義的碼，而症狀是「跑到一半進了亂碼」。
func TestPMExitIsRecorded(t *testing.T) {
	c, s := newPMDOS(t)
	c.R[cpu386.EAX] = 0x4C03
	if !s.Handle(c, 0x21) {
		t.Fatal("AH=4Ch 沒實作")
	}
	if !s.Exited {
		t.Fatal("結束沒有被記下來")
	}
	if s.ExitCode != 3 {
		t.Errorf("離開碼是 %d，預期 3", s.ExitCode)
	}
}

// 補上這一批**不能破壞 FD2 的啟動握手**。
//
// 握手是靠呼叫順序認的（`calls` 計數器）；新加的服務要是把 `AH=30h`
// 之類的先攔走，握手就永遠停在第一步——而症狀是「程式一開始就不動了」。
func TestPMGeneralServicesDoNotDisturbStartup(t *testing.T) {
	c, s := newPMDOS(t)
	// 先做幾次通用服務。
	c.R[cpu386.EAX], c.R[cpu386.EDX] = 0x0200, 'x'
	s.Handle(c, 0x21)
	c.R[cpu386.EAX], c.R[cpu386.EBX] = 0x3e00, 99
	s.Handle(c, 0x21)
	if s.Calls() != 0 {
		t.Fatalf("通用服務把握手計數器推到 %d，預期還是 0", s.Calls())
	}
	// 握手照樣要成立。
	c.R[cpu386.EAX], c.R[cpu386.EBX] = 0x3000, 0x50484152
	if !s.Handle(c, 0x21) || uint16(c.R[cpu386.EAX]) != 0x1606 {
		t.Fatalf("握手第一步壞了：EAX=%08X", c.R[cpu386.EAX])
	}
	if s.Calls() != 1 {
		t.Errorf("握手之後計數器是 %d，預期 1", s.Calls())
	}
}

// 沒做的服務仍然要被拒絕。
//
// 安靜地放行會讓「這支程式踩到我們沒做的東西」看不出來——
// 而它會照著沒有初始化的暫存器往下走。
func TestPMUnknownServiceIsStillRejected(t *testing.T) {
	c, s := newPMDOS(t)
	for _, ax := range []uint32{0x2900, 0x3c00, 0x4100, 0x5600} {
		c.R[cpu386.EAX] = ax
		if s.Handle(c, 0x21) {
			t.Errorf("AX=%04X 沒實作卻被放行", ax)
		}
	}
}
