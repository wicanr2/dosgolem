package xlate

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// buildGolemFont 組一份 GOLEMFNT 格式的位元組（spec 202 §2.1），glyphs 依序寫入。
func buildGolemFont(t *testing.T, w, h int, glyphs []struct {
	cp     rune
	source byte
	glyph  []byte
}) []byte {
	t.Helper()
	var buf bytes.Buffer
	buf.WriteString(golemFontMagic)
	binary.Write(&buf, binary.LittleEndian, uint16(w))
	binary.Write(&buf, binary.LittleEndian, uint16(h))
	binary.Write(&buf, binary.LittleEndian, uint32(len(glyphs)))
	for _, g := range glyphs {
		binary.Write(&buf, binary.LittleEndian, uint32(g.cp))
		buf.WriteByte(g.source)
		buf.Write(g.glyph)
	}
	return buf.Bytes()
}

// spec 202 §3 第 3 項：寫一個 2 字的 16×15 字型再讀回，字模相同；magic 錯、長度錯回錯。
func TestLoadFontRoundTrip(t *testing.T) {
	w, h := 16, 15
	rowBytes := (w + 7) / 8 // 2
	g1 := make([]byte, h*rowBytes)
	g1[0] = 0x80
	g2 := make([]byte, h*rowBytes)
	g2[1] = 0x01

	data := buildGolemFont(t, w, h, []struct {
		cp     rune
		source byte
		glyph  []byte
	}{
		{'一', 0, g1},
		{'二', 7, g2}, // source 呼叫端自訂，這裡不解讀，值是多少都該被忽略
	})

	path := filepath.Join(t.TempDir(), "font.bin")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	f, err := LoadFont(path)
	if err != nil {
		t.Fatalf("LoadFont: %v", err)
	}
	if f.W != w || f.H != h {
		t.Fatalf("尺寸 %dx%d，要 %dx%d", f.W, f.H, w, h)
	}
	if len(f.Glyphs) != 2 {
		t.Fatalf("字數 %d，要 2", len(f.Glyphs))
	}
	if got := f.Glyphs['一']; !bytes.Equal(got, g1) {
		t.Errorf("字模 一 不同：%v", got)
	}
	if got := f.Glyphs['二']; !bytes.Equal(got, g2) {
		t.Errorf("字模 二 不同：%v", got)
	}
}

func TestLoadFontBadMagic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad-magic.bin")
	data := buildGolemFont(t, 8, 8, nil)
	data[0] = 'X' // 弄壞 magic
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFont(path); err == nil {
		t.Error("magic 錯應該回錯")
	}
}

func TestLoadFontBadLength(t *testing.T) {
	path := filepath.Join(t.TempDir(), "short.bin")
	data := buildGolemFont(t, 8, 8, nil)
	// 宣告字數但檔案內容截斷。
	var buf bytes.Buffer
	buf.Write(data[:golemFontHeaderLen])
	binary.LittleEndian.PutUint32(buf.Bytes()[12:16], 1) // 宣告 1 字，內容不補
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFont(path); err == nil {
		t.Error("長度不對應該回錯")
	}
}

func TestLoadFontMissingFile(t *testing.T) {
	if _, err := LoadFont(filepath.Join(t.TempDir(), "nope.bin")); err == nil {
		t.Error("檔案不存在應該回錯")
	}
}
