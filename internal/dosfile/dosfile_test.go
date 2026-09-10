package dosfile

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

// 這一份釘的是**兩個前端都必須同意**的語意。任何一條寫錯，症狀都是
// 「同一個檔在 16 位元程式裡讀得到、在 32 位元程式裡少一段」——
// 而沒有人會懷疑是 seek 的號誌不同。

// fakeFile 是一份放在記憶體裡的檔，可以被指定成「讀到一半就出錯」。
type fakeFile struct {
	*bytes.Reader
	failAfter int // −1 ＝ 不出錯
	read      int
	closed    bool
}

func newFake(data string) *fakeFile {
	return &fakeFile{Reader: bytes.NewReader([]byte(data)), failAfter: -1}
}

func (f *fakeFile) Read(p []byte) (int, error) {
	if f.failAfter >= 0 && f.read >= f.failAfter {
		return 0, errors.New("裝置錯誤")
	}
	n, err := f.Reader.Read(p)
	f.read += n
	return n, err
}

func (f *fakeFile) Close() error { f.closed = true; return nil }

// 讀到檔尾要回「0 個位元組 ＋ 成功」。
//
// 回失敗的話程式走錯誤路徑，而那多半是「檔案損毀」的訊息——
// 檔案其實好好的。
func TestReadAtEOFIsNotAnError(t *testing.T) {
	f := newFake("abc")
	buf := make([]byte, 8)
	n, code := Read(f, buf)
	if code != 0 || n != 3 {
		t.Fatalf("第一次讀回 (%d, %d)，預期 (3, 0)", n, code)
	}
	n, code = Read(f, buf)
	if code != 0 || n != 0 {
		t.Fatalf("檔尾讀回 (%d, %d)，預期 (0, 0)——EOF 不是錯誤", n, code)
	}
}

// 真的裝置錯誤才回錯誤碼。
func TestReadReportsRealErrors(t *testing.T) {
	f := newFake("abcdef")
	f.failAfter = 0
	if _, code := Read(f, make([]byte, 4)); code != ErrAccessDenied {
		t.Errorf("裝置錯誤回 %d，預期 %d", code, ErrAccessDenied)
	}
}

// 字元裝置（nil）要回 EOF，不能崩。
func TestReadOnCharacterDeviceReturnsEOF(t *testing.T) {
	if n, code := Read(nil, make([]byte, 4)); n != 0 || code != 0 {
		t.Errorf("字元裝置回 (%d, %d)，預期 (0, 0)", n, code)
	}
	if _, code := Seek(nil, 0, 0); code != ErrInvalidFunction {
		t.Errorf("字元裝置 seek 回 %d，預期 %d", code, ErrInvalidFunction)
	}
}

// `CX:DX` 是**有號**的：從檔尾往回退要真的往回退。
//
// 當成無號的話會 seek 到 4 GB 之外，之後的讀取回 0 個位元組——
// 看起來像「檔案是空的」。
func TestSeekTreatsOffsetAsSigned(t *testing.T) {
	if got := SignedOffset(0xFFFF, 0xFF9C); got != -100 {
		t.Fatalf("CX:DX = FFFF:FF9C 組出 %d，預期 −100", got)
	}
	f := newFake("0123456789")
	pos, code := Seek(f, SignedOffset(0xFFFF, 0xFFFD), 2) // 檔尾 −3
	if code != 0 || pos != 7 {
		t.Fatalf("從檔尾退 3 回 (%d, %d)，預期 (7, 0)", pos, code)
	}
	buf := make([]byte, 8)
	n, _ := Read(f, buf)
	if string(buf[:n]) != "789" {
		t.Errorf("讀到 %q，預期 789——位移被當成無號了", buf[:n])
	}
}

// origin 只有 0／1／2。
//
// 放行別的值的話 io 套件會當成 0（檔頭），而程式以為自己從檔尾往回找。
func TestSeekRejectsUnknownOrigin(t *testing.T) {
	f := newFake("0123456789")
	Seek(f, 5, 0)
	for _, origin := range []uint8{3, 0x80, 0xFF} {
		if _, code := Seek(f, 0, origin); code != ErrInvalidFunction {
			t.Errorf("origin=%d 回 %d，預期 %d", origin, code, ErrInvalidFunction)
		}
	}
	// 失敗之後指標要留在原位。
	pos, _ := f.Seek(0, io.SeekCurrent)
	if pos != 5 {
		t.Errorf("失敗的 seek 把指標移到 %d，預期留在 5", pos)
	}
}

// seek 到負的位置要失敗，而且**退回原位**。
//
// 留在半途的話，呼叫端只看 CF，接著會從一個它沒指定過的位置繼續讀。
func TestSeekFailureRestoresPosition(t *testing.T) {
	f := newFake("0123456789")
	Seek(f, 4, 0)
	if _, code := Seek(f, -100, 1); code == 0 {
		t.Fatal("往檔頭之前 seek 竟然成功")
	}
	if pos, _ := f.Seek(0, io.SeekCurrent); pos != 4 {
		t.Errorf("失敗之後指標在 %d，預期留在 4", pos)
	}
}

// handle 從 5 開始配：0–4 是 DOS 開好的標準 handle。
//
// 從 0 開始配的話，第一個開的檔會蓋掉 stdin，而程式印出來的東西
// 會跑進那個檔裡。
func TestTableStartsAboveStandardHandles(t *testing.T) {
	tab := NewTable()
	h, code := tab.Add(newFake("x"), "A.DAT")
	if code != 0 || h != FirstHandle {
		t.Fatalf("第一個 handle 是 %d（碼 %d），預期 %d", h, code, FirstHandle)
	}
	if tab.Name(h) != "A.DAT" {
		t.Errorf("名字是 %q，預期 A.DAT", tab.Name(h))
	}
}

// 關過的號碼**不重用**。
//
// 重用的話，程式關掉之後又拿舊號碼去讀，讀到的是別人的檔——它不會報錯。
func TestTableDoesNotRecycleHandles(t *testing.T) {
	tab := NewTable()
	first, _ := tab.Add(newFake("a"), "A")
	if code := tab.Close(first); code != 0 {
		t.Fatalf("關檔回 %d", code)
	}
	if code := tab.Close(first); code != ErrInvalidHandle {
		t.Errorf("關第二次回 %d，預期 %d", code, ErrInvalidHandle)
	}
	second, _ := tab.Add(newFake("b"), "B")
	if second == first {
		t.Errorf("第二個 handle 又是 %d——舊指標會讀到新檔", second)
	}
	if _, code := tab.Read(first, make([]byte, 1)); code != ErrInvalidHandle {
		t.Errorf("讀關掉的 handle 回 %d，預期 %d", code, ErrInvalidHandle)
	}
}

// 表要真的把檔關掉，不能只從 map 移除。
func TestTableCloseAllClosesFiles(t *testing.T) {
	tab := NewTable()
	a, b := newFake("a"), newFake("b")
	tab.Add(a, "A")
	tab.Add(b, "B")
	if err := tab.CloseAll(); err != nil {
		t.Fatalf("CloseAll 回 %v", err)
	}
	if !a.closed || !b.closed {
		t.Errorf("檔沒有真的關掉：a=%t b=%t", a.closed, b.closed)
	}
	if tab.Has(FirstHandle) || tab.Has(FirstHandle+1) {
		t.Error("CloseAll 之後表裡還有東西")
	}
}

// 零值的 Table 也要能用：前端常常直接造一個空的結構。
func TestZeroValueTableWorks(t *testing.T) {
	var tab Table
	h, code := tab.Add(newFake("hello"), "H")
	if code != 0 {
		t.Fatalf("零值表配 handle 回 %d", code)
	}
	buf := make([]byte, 5)
	if n, code := tab.Read(h, buf); code != 0 || string(buf[:n]) != "hello" {
		t.Errorf("讀回 %q（碼 %d）", buf[:n], code)
	}
}

func TestReusingTableKeepsLiveFilesAndReusesHoles(t *testing.T) {
	tab := NewReusingTable()
	live, _ := tab.Add(newFake("live"), "LIVE")
	for i := 0; i < 300; i++ {
		h, code := tab.Add(newFake("temp"), "TMP")
		if code != 0 || h != 6 {
			t.Fatalf("重用代號=%d code=%d", h, code)
		}
		if tab.Close(h) != 0 {
			t.Fatal("關閉失敗")
		}
		if _, code := tab.Read(h, make([]byte, 1)); code != ErrInvalidHandle {
			t.Fatal("未重開的代號仍有效")
		}
	}
	if live != 5 || tab.Name(live) != "LIVE" {
		t.Fatal("存活檔被覆蓋")
	}
	h, _ := tab.Add(newFake("second"), "SECOND")
	tab.Close(live)
	hole, _ := tab.Add(newFake("new"), "NEW")
	if hole != 5 || h != 6 || tab.Name(h) != "SECOND" {
		t.Fatal("未採最低空洞或覆蓋存活檔")
	}
	tab.CloseAll()
}
