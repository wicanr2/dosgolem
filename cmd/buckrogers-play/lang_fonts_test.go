package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Buck repo spec 041 §3.7: -lang-fonts points the non-zh-TW fonts at a
// local directory (workplace/lang-fonts); a language without its file
// there stays without a font (the runtime turns it off).
func TestLiveOptionsLangFontDir(t *testing.T) {
	d := t.TempDir()
	cn := filepath.Join(d, "buckrogers-zh-CN.golemfnt")
	if err := os.WriteFile(cn, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	o := liveOptions("/text", "/tw.fnt", d)
	if o.LangFonts["zh-CN"] != cn {
		t.Fatalf("zh-CN 字型 = %q", o.LangFonts["zh-CN"])
	}
	if _, ok := o.LangFonts["ja"]; ok {
		t.Fatal("ja 沒有字型檔，不應指定")
	}
	if len(o.Langs) != 3 || o.TextDir != "/text" || o.FontPath != "/tw.fnt" {
		t.Fatalf("%+v", o)
	}
}
