package main

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckFrameDump(t *testing.T) {
	scaleScript := map[int][]scriptAction{7: {{scale: true}}}
	plain := map[int][]scriptAction{7: {{help: true}, {lang: true}}}
	cases := []struct {
		name   string
		frames int
		dir    string
		every  int
		script map[int][]scriptAction
		want   int
		bad    string // 非空：錯誤訊息須含這段
	}{
		{"關閉", 100, "", frameEveryUnset, plain, 0, ""},
		{"關閉但給了 every", 100, "", 3, plain, 0, "-frame-every 須搭配"},
		{"預設間隔", 100, "d", frameEveryUnset, plain, 2, ""},
		{"指定間隔", 100, "d", 3, nil, 3, ""},
		{"下限", 100, "d", 1, nil, 1, ""},
		{"上限", 100, "d", 60, nil, 60, ""},
		{"零", 100, "d", 0, nil, 0, "1–60"},
		{"超出上限", 100, "d", 61, nil, 0, "1–60"},
		{"負間隔", 100, "d", -2, nil, 0, "1–60"},
		{"互動模式", 0, "d", 2, nil, 0, "-frames"},
		{"負 frames", -5, "d", 2, nil, 0, "-frames"},
		{"含 scale", 100, "d", 2, scaleScript, 0, "scale"},
	}
	for _, c := range cases {
		got, err := checkFrameDump(c.frames, c.dir, c.every, c.script)
		if c.bad != "" {
			if err == nil || !strings.Contains(err.Error(), c.bad) {
				t.Errorf("%s：錯誤 %v，要含 %q", c.name, err, c.bad)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("%s：得 %d、%v，要 %d", c.name, got, err, c.want)
		}
	}
}

func TestNewFrameDumperDir(t *testing.T) {
	root := t.TempDir()
	fresh := filepath.Join(root, "a", "b")
	if _, err := newFrameDumper(fresh, 2); err != nil {
		t.Fatalf("不存在的目錄要建立：%v", err)
	}
	if _, err := newFrameDumper(fresh, 2); err != nil {
		t.Fatalf("已存在的空目錄可用：%v", err)
	}
	if err := os.WriteFile(filepath.Join(fresh, "x.png"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := newFrameDumper(fresh, 2); err == nil || !strings.Contains(err.Error(), "為空") {
		t.Fatalf("非空目錄要報錯，得 %v", err)
	}
	file := filepath.Join(root, "f")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := newFrameDumper(file, 2); err == nil {
		t.Fatal("路徑是檔案要報錯")
	}
}

func gradient(w, h int) []byte {
	pix := make([]byte, 4*w*h)
	for i := 0; i < w*h; i++ {
		pix[4*i], pix[4*i+1], pix[4*i+2], pix[4*i+3] = byte(i), byte(i>>8), byte(i*7), 255
	}
	return pix
}

func TestFrameDumpNameAndPixels(t *testing.T) {
	d, err := newFrameDumper(t.TempDir(), 3)
	if err != nil {
		t.Fatal(err)
	}
	want := gradient(640, 400)
	calls := 0
	compose := func() ([]byte, bool, error) { calls++; return want, true, nil }
	for f := 1; f <= 12; f++ {
		if err := d.maybeWrite(f, 2, compose); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 4 {
		t.Fatalf("compose 只在間隔的倍數呼叫：%d 次，要 4", calls)
	}
	var names []string
	ents, _ := os.ReadDir(d.dir)
	for _, e := range ents {
		names = append(names, e.Name())
	}
	if got, w := strings.Join(names, ","), "frame-00000003.png,frame-00000006.png,frame-00000009.png,frame-00000012.png"; got != w {
		t.Fatalf("檔名 %s，要 %s", got, w)
	}
	if d.count != 4 || d.first != 3 || d.last != 12 {
		t.Fatalf("count=%d first=%d last=%d", d.count, d.first, d.last)
	}
	if !strings.Contains(d.summary(), "count=4 first=3 last=12") {
		t.Fatalf("summary %q", d.summary())
	}
	b, err := os.ReadFile(filepath.Join(d.dir, "frame-00000012.png"))
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	// 全不透明的 NRGBA 會被編成不含 alpha 的 PNG，解碼成 *image.RGBA；逐像素比。
	if img.Bounds() != image.Rect(0, 0, 640, 400) {
		t.Fatalf("尺寸 %v", img.Bounds())
	}
	got := make([]byte, 0, len(want))
	for y := 0; y < 400; y++ {
		for x := 0; x < 640; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			got = append(got, c.R, c.G, c.B, c.A)
		}
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("解碼後的像素與輸入不同（%T）", img)
	}
}

func TestFrameDumpNoFrameYetSkips(t *testing.T) {
	d, _ := newFrameDumper(t.TempDir(), 1)
	ready := false
	compose := func() ([]byte, bool, error) { return gradient(320, 200), ready, nil }
	if err := d.maybeWrite(1, 1, compose); err != nil || d.count != 0 {
		t.Fatalf("尚無畫格要略過：err=%v count=%d", err, d.count)
	}
	ready = true
	if err := d.maybeWrite(2, 1, compose); err != nil || d.count != 1 || d.first != 2 {
		t.Fatalf("之後正常輸出：err=%v count=%d first=%d", err, d.count, d.first)
	}
}

func TestFrameDumpComposeErrorPropagates(t *testing.T) {
	d, _ := newFrameDumper(t.TempDir(), 1)
	boom := errors.New("boom")
	if err := d.maybeWrite(1, 1, func() ([]byte, bool, error) { return nil, false, boom }); !errors.Is(err, boom) {
		t.Fatalf("得 %v", err)
	}
}

func TestFrameDumpSizeMismatch(t *testing.T) {
	d, _ := newFrameDumper(t.TempDir(), 1)
	if err := d.write(1, 320, 200, make([]byte, 10)); err == nil {
		t.Fatal("像素數不符要報錯")
	}
	if ents, _ := os.ReadDir(d.dir); len(ents) != 0 {
		t.Fatalf("不符時不留檔：%d 個", len(ents))
	}
}

func TestFrameDumpEncodeFailureRemovesPartial(t *testing.T) {
	d, _ := newFrameDumper(t.TempDir(), 1)
	old := frameEncode
	defer func() { frameEncode = old }()
	frameEncode = func(w io.Writer, _ image.Image) error {
		w.Write([]byte("partial"))
		return errors.New("disk full")
	}
	if err := d.write(5, 320, 200, gradient(320, 200)); err == nil {
		t.Fatal("編碼失敗要回報")
	}
	if ents, _ := os.ReadDir(d.dir); len(ents) != 0 {
		t.Fatalf("半成品要刪除：留下 %d 個", len(ents))
	}
	if d.count != 0 {
		t.Fatalf("失敗不計入 count=%d", d.count)
	}
}

func TestFrameDumpExistingFileKept(t *testing.T) {
	d, _ := newFrameDumper(t.TempDir(), 1)
	path := filepath.Join(d.dir, frameDumpName(7))
	if err := os.WriteFile(path, []byte("mine"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := d.write(7, 320, 200, gradient(320, 200)); err == nil {
		t.Fatal("O_EXCL：已存在的檔要報錯")
	}
	if b, _ := os.ReadFile(path); string(b) != "mine" {
		t.Fatalf("已存在的檔不可被刪或覆寫：%q", b)
	}
}
