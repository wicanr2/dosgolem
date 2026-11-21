package mixer

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"

	"github.com/wicanr2/dosgolem/internal/dos"
)

// CD 音軌來源的契約測試（`docs/spec/440-mixer-cd-audio` §4）。
// 每秒 48000 步，所以步數就是輸出樣本序號。

const cdTestSPS = SampleRate

// cdFixture 造一片 30 扇區的光碟：第 1 軌資料（LBA 0），第 2 軌音訊（LBA 10）。
// 映像裡第 j 個立體聲樣本（從 LBA 0 起算）左聲道是 7j mod 30000（斜率大於 1，內插的分數部分才看得出來），右聲道是它的負值。
func cdFixture(t *testing.T) (*dos.CDROM, *bytes.Reader) {
	t.Helper()
	cd, err := dos.ParseCDTOC(3, "1 data 0 0\n2 audio 10 23520\nleadout 30\n")
	if err != nil {
		t.Fatal(err)
	}
	img := make([]byte, 30*cdSectorBytes)
	for j := 0; j < len(img)/4; j++ {
		v := int16(j * 7 % 30000)
		binary.LittleEndian.PutUint16(img[j*4:], uint16(v))
		binary.LittleEndian.PutUint16(img[j*4+2:], uint16(-v))
	}
	return cd, bytes.NewReader(img)
}

// cdPlays 是三段播放：A 在 ToStep 停、B 播滿 1 扇區自然結束、C 落在資料軌。
var cdPlays = []dos.CDPlay{
	{FromStep: 100, ToStep: 400, Start: 10, Length: 5},
	{FromStep: 1000, Start: 12, Length: 1},
	{FromStep: 3000, Start: 0, Length: 2},
}

// wantCD 依規格的式子獨立算出輸出樣本 n 的左聲道值。
func wantCD(n int64) int64 {
	val := func(start, k int64) int64 { return (start*588 + k) * 7 % 30000 }
	seg := func(from, stop, start, length int64) (int64, bool) {
		if n < from || (stop != 0 && n >= stop) {
			return 0, false
		}
		num := (n - from) * 44100
		k, f := num/48000, num%48000
		total := length * 588
		if k >= total {
			return 0, true
		}
		s0, s1 := val(start, k), val(start, k)
		if k+1 < total {
			s1 = val(start, k+1)
		}
		return s0 + (s1-s0)*f/48000, true
	}
	if n >= 3000 {
		return 0 // 資料軌
	}
	if v, ok := seg(1000, 0, 12, 1); ok {
		return v
	}
	v, _ := seg(100, 400, 10, 5)
	return v
}

func cdRender(t *testing.T, cuts []uint64) []float32 {
	t.Helper()
	cd, img := cdFixture(t)
	cd.Plays = cdPlays
	m := New(cdTestSPS)
	m.AttachCD(cd, img)
	var out []float32
	prev := uint64(0)
	for _, c := range cuts {
		out = m.Render(out, prev, c)
		prev = c
	}
	if m.CDErrors() != 0 {
		t.Fatalf("讀映像錯誤 %d 次", m.CDErrors())
	}
	return out
}

func TestCDInterpolatesAndStops(t *testing.T) {
	out := cdRender(t, []uint64{4000})
	if len(out) != 8000 {
		t.Fatalf("%d 個 float，要 8000", len(out))
	}
	for n := int64(0); n < 4000; n++ {
		l := int64(math.Round(float64(out[2*n]) * 32768))
		r := int64(math.Round(float64(out[2*n+1]) * 32768))
		want := wantCD(n)
		if l != want || r != -want {
			t.Fatalf("樣本 %d：(%d, %d)，要 (%d, %d)", n, l, r, want, -want)
		}
	}
	// 抽查邊界：A 的最後一個樣本有值、ToStep 之後是 0；B 播滿 640 個輸出樣本後是 0。
	if wantCD(399) == 0 || wantCD(400) != 0 || wantCD(1639) == 0 || wantCD(1640) != 0 {
		t.Fatal("測試本身的邊界算錯")
	}
}

func TestCDPositionIgnoresRenderSplit(t *testing.T) {
	whole := cdRender(t, []uint64{4000})
	split := cdRender(t, []uint64{1, 137, 399, 400, 1001, 1002, 2500, 4000})
	if len(whole) != len(split) {
		t.Fatalf("長度 %d 與 %d", len(whole), len(split))
	}
	for i := range whole {
		if whole[i] != split[i] {
			t.Fatalf("第 %d 個 float 不同：%v 與 %v", i, whole[i], split[i])
		}
	}
}

func TestCDSilentOnDataTrack(t *testing.T) {
	out := cdRender(t, []uint64{4000})
	for i := 3000 * 2; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("資料軌的位置 %d 有聲音 %v", i/2, out[i])
		}
	}
}
