package phantasie

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseBatch(t *testing.T) {
	text := "@echo off\r\nREM 註解\r\n:: 另一種註解\r\n:label\r\n\r\nfirst.com\r\n  Second.Com /x\r\nmain\r\n"
	want := []string{"first.com", "Second.Com", "main"}
	if got := ParseBatch(text); !reflect.DeepEqual(got, want) {
		t.Errorf("ParseBatch=%q，預期 %q", got, want)
	}
}

func TestFindBatchNeedsExactlyOne(t *testing.T) {
	dir := t.TempDir()
	if _, err := FindBatch(dir); err == nil {
		t.Error("沒有 .BAT 卻沒回錯")
	}
	os.WriteFile(filepath.Join(dir, "A.BAT"), nil, 0o644)
	if got, err := FindBatch(dir); err != nil || got != "A.BAT" {
		t.Errorf("FindBatch=%q,%v", got, err)
	}
	os.WriteFile(filepath.Join(dir, "b.bat"), nil, 0o644)
	if _, err := FindBatch(dir); err == nil {
		t.Error("兩個 .BAT 卻沒回錯")
	}
}

func TestResolveIsCaseInsensitiveAndTriesExtensions(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "PROG.EXE"), nil, 0o644)
	if p, err := resolve(dir, "prog"); err != nil || filepath.Base(p) != "PROG.EXE" {
		t.Errorf("resolve=%q,%v", p, err)
	}
	if _, err := resolve(dir, "none"); err == nil {
		t.Error("不存在的檔沒回錯")
	}
}
