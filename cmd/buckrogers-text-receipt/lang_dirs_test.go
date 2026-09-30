package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// Buck repo spec 041 §3.9: -lang-dir／-lang-font／-font-root.
func TestReceiptLangs(t *testing.T) {
	root := t.TempDir()
	text := filepath.Join(root, "text")
	if err := os.MkdirAll(filepath.Join(root, "font"), 0o755); err != nil {
		t.Fatal(err)
	}
	cn := filepath.Join(root, "font", "buckrogers-zh-CN.golemfnt")
	if err := os.WriteFile(cn, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Nothing named: no extra lane (zh-TW receipts unchanged).
	if l, d, f := receiptLangs(text, "", "tw.fnt", "", nil, langPathFlag{}, langPathFlag{}); l != nil || d != nil || f != nil {
		t.Fatalf("無語言旗標應不載入：%v %v %v", l, d, f)
	}
	// -lang zh-CN: directory defaults to -live-text-dir, font auto-loaded.
	l, d, f := receiptLangs(text, "", "tw.fnt", "zh-CN", []string{"en", "zh-TW"}, langPathFlag{}, langPathFlag{})
	if !reflect.DeepEqual(l, []string{"zh-CN"}) || d["zh-CN"] != text || f["zh-CN"] != cn {
		t.Fatalf("%v %v %v", l, d, f)
	}
	// zz through the generic flags, default font = overlay font; ja named by
	// a switch without a font stays without one (the runtime turns it off).
	dirs := langPathFlag{}
	if err := dirs.Set("zz=/zz"); err != nil {
		t.Fatal(err)
	}
	fonts := langPathFlag{}
	if err := fonts.Set("zh-CN=/cn.fnt"); err != nil {
		t.Fatal(err)
	}
	l, d, f = receiptLangs(text, "/other", "tw.fnt", "", []string{"ja"}, dirs, fonts)
	if !reflect.DeepEqual(l, []string{"zh-CN", "ja", "zz"}) || d["zz"] != "/zz" || f["zz"] != "tw.fnt" || f["zh-CN"] != "/cn.fnt" {
		t.Fatalf("%v %v %v", l, d, f)
	}
	if _, ok := f["ja"]; ok {
		t.Fatal("ja 沒有字型不應憑空給")
	}
	for _, bad := range []string{"zh-CN", "=x", "xx=/d"} {
		if err := (langPathFlag{}).Set(bad); err == nil {
			t.Errorf("%q 應失敗", bad)
		}
	}
	if err := fonts.Set("zh-CN=/again"); err == nil {
		t.Error("重複指定應失敗")
	}
}
