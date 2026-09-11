package vgm

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// enc 是測試的慣用寫法：時間基準取 SampleRate，於是**一道指令 ＝ 一個取樣**，
// 延遲的數字可以直接讀。
func enc(t *testing.T, ws []Write) ([]byte, Stats) {
	t.Helper()
	var b bytes.Buffer
	st, err := Encode(&b, ws, SampleRate)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	return b.Bytes(), st
}

func u32(t *testing.T, b []byte, off int) uint32 {
	t.Helper()
	return binary.LittleEndian.Uint32(b[off:])
}

// TestHeaderLayout 檔頭的五個欄位。
//
// **時脈欄位在 0x50 而資料在 0x100**，這兩件事互相牽制：資料起點往前搬
// 到 0x40 會蓋掉時脈，而播放器認不出晶片時是**安靜地播出無聲**，
// 不是報錯（`docs/spec/190-opl-vgm-dump` §5）。
func TestHeaderLayout(t *testing.T) {
	b, st := enc(t, []Write{{Reg: 0x20, Val: 0x01}, {Reg: 0x40, Val: 0x10, Step: 100}})

	if string(b[:4]) != "Vgm " {
		t.Errorf("識別字 %q", b[:4])
	}
	if v := u32(t, b, 0x08); v != 0x151 {
		t.Errorf("版本 %#x，要 0x151", v)
	}
	if v := u32(t, b, 0x34); v != dataStart-0x34 {
		t.Errorf("資料位移 %#x，要 %#x（資料落在 0x100）", v, dataStart-0x34)
	}
	if v := u32(t, b, 0x04); v != uint32(len(b)-4) {
		t.Errorf("EOF 位移 %d，檔案剩下 %d 個位元組", v, len(b)-4)
	}
	if v := u32(t, b, 0x50); v != ym3812Clock {
		t.Errorf("0x50 的 YM3812 時脈 %d，要 %d", v, ym3812Clock)
	}
	if v := u32(t, b, 0x5C); v != 0 {
		t.Errorf("沒碰過第二組卻填了 YMF262 時脈 %d", v)
	}
	if v := u32(t, b, 0x18); v != 100 {
		t.Errorf("總取樣數 %d，兩筆相差 100 道指令＝100 個取樣", v)
	}
	if v := u32(t, b, 0x1C); v != 0 {
		t.Errorf("循環位移 %d，不猜循環點就該是 0", v)
	}
	if st.Chip != "YM3812" || st.Writes != 2 || st.Samples != 100 {
		t.Errorf("Stats = %+v", st)
	}
}

// TestBodyIsWritesAndWaitsThenTerminator 資料段的內容逐位元組。
//
// **0x66 少不得**：播放器讀到檔尾才發現沒有結束標記，多數的反應是把
// 後面的垃圾當成命令。
func TestBodyIsWritesAndWaitsThenTerminator(t *testing.T) {
	b, _ := enc(t, []Write{{Reg: 0x20, Val: 0x01}, {Reg: 0x40, Val: 0x10, Step: 100}})
	body := b[dataStart:]
	want := []byte{
		0x5A, 0x20, 0x01, // YM3812 ← 20h = 01h
		0x61, 100, 0, // 等 100 個取樣
		0x5A, 0x40, 0x10,
		0x66, // 結束
	}
	if !bytes.Equal(body, want) {
		t.Errorf("資料段\n 得到 % 02X\n 應該 % 02X", body, want)
	}
}

// TestFirstWriteIsTheZeroPoint 零點取第一筆寫入，不是步數 0。
//
// 開機到第一個音符之間的空白不錄——那是載入時間，不是樂曲的一部分，
// 而且它的長度取決於執行器多快。
func TestFirstWriteIsTheZeroPoint(t *testing.T) {
	b, st := enc(t, []Write{{Reg: 1, Step: 9_000_000}, {Reg: 2, Step: 9_000_050}})
	if st.Samples != 50 {
		t.Fatalf("總取樣數 %d，兩筆只差 50 道指令", st.Samples)
	}
	if b[dataStart] != 0x5A {
		t.Errorf("資料段第一個位元組 %02X，應該直接是寫入而不是一段延遲", b[dataStart])
	}
}

// TestWaitPicksTheShortestForm 三種延遲命令的邊界。
func TestWaitPicksTheShortestForm(t *testing.T) {
	for _, c := range []struct {
		n    uint64
		want []byte
	}{
		{1, []byte{0x70}},
		{16, []byte{0x7F}},
		{17, []byte{0x61, 17, 0}},
		{0xFFFF, []byte{0x61, 0xFF, 0xFF}},
		{0x10000, []byte{0x61, 0xFF, 0xFF, 0x70}},           // 65535 ＋ 1
		{70000, []byte{0x61, 0xFF, 0xFF, 0x61, 0x71, 0x11}}, // 65535 ＋ 4465
	} {
		if got := appendWait(nil, c.n); !bytes.Equal(got, c.want) {
			t.Errorf("等 %d 個取樣 → % 02X，應該是 % 02X", c.n, got, c.want)
		}
	}
}

// TestSecondBankSwitchesToYMF262 摸過第二組就整份改用 OPL3 的命令。
//
// 判準刻意寬：把 OPL3 判成 OPL2 會丟掉一半的寫入（少掉聲部），
// 反過來只是檔頭上的晶片名不同、聲音一樣。
func TestSecondBankSwitchesToYMF262(t *testing.T) {
	b, st := enc(t, []Write{
		{Bank: 0, Reg: 0x20, Val: 0x01},
		{Bank: 1, Reg: 0x05, Val: 0x01},
	})
	if st.Chip != "YMF262" {
		t.Fatalf("晶片判成 %s", st.Chip)
	}
	if v := u32(t, b, 0x5C); v != ymf262Clock {
		t.Errorf("0x5C 的 YMF262 時脈 %d，要 %d", v, ymf262Clock)
	}
	if v := u32(t, b, 0x50); v != 0 {
		t.Errorf("判成 OPL3 卻還填了 YM3812 時脈 %d", v)
	}
	body := b[dataStart:]
	want := []byte{0x5E, 0x20, 0x01, 0x5F, 0x05, 0x01, 0x66}
	if !bytes.Equal(body, want) {
		t.Errorf("資料段\n 得到 % 02X\n 應該 % 02X（第一組走 5E、第二組走 5F）", body, want)
	}
}

// TestTimeBaseScalesTheWholeThing 時間基準是真的有在用。
//
// **這一條在釘住「曲速」。** 換算寫死成某個常數的話，同一份序列不管
// 機器跑多快都會編出同一份 VGM，而那份 VGM 照樣合法、照樣播得出來
// ——錯誤只在速度上，沒有東西會開口。
func TestTimeBaseScalesTheWholeThing(t *testing.T) {
	ws := []Write{{Reg: 1}, {Reg: 2, Step: 1_000_000}}
	var slow, fast bytes.Buffer
	a, err := Encode(&slow, ws, 1_000_000)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Encode(&fast, ws, 4_000_000)
	if err != nil {
		t.Fatal(err)
	}
	if a.Seconds != 1 {
		t.Errorf("1 M 指令／秒跑 1 M 道指令 ＝ %g 秒，應該是 1", a.Seconds)
	}
	if b.Seconds != 0.25 {
		t.Errorf("4 M 指令／秒跑 1 M 道指令 ＝ %g 秒，應該是 0.25", b.Seconds)
	}
}

// TestEmptyIsAnError 空序列回錯誤，不寫出一份沒有音符的 VGM。
//
// 一份合法但空的檔案會一路通過後面每一個步驟，最後在喇叭上變成
// 「沒聲音」，而那時候已經離這裡很遠了。
func TestEmptyIsAnError(t *testing.T) {
	var b bytes.Buffer
	if _, err := Encode(&b, nil, SampleRate); err == nil {
		t.Fatal("空序列應該回錯誤")
	}
	if b.Len() != 0 {
		t.Errorf("空序列不該寫出 %d 個位元組", b.Len())
	}
	if _, err := Encode(&b, []Write{{Reg: 1}}, 0); err == nil {
		t.Error("時間基準 0 應該回錯誤，不該自己猜一個")
	}
}
