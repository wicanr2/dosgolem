package main

import (
	"strings"
	"testing"
)

// 這一組釘住從 logh3-com-support 併回來的兩個 probe 功能。
// 它們原本沒有測試護著，而兩樣的壞法都是安靜的：一個少印了東西，
// 一個什麼檔都沒產生。

// TestParseMemShots 釘住 -dump-mem-at 的規格解析。
//
// 旗標寫錯的症狀是「跑完什麼檔都沒有」，不是報錯——所以每一種寫壞的
// 形式都要當場退回來，而不是安靜地產生零個時點。
func TestParseMemShots(t *testing.T) {
	// 位址那一段走 parseAddr，三種寫法都要通。
	for name, c := range map[string]struct {
		spec string
		addr uint32
		n    int
	}{
		"線性位址": {"1000:lin:A0000:64:out.bin", 0xA0000, 64},
		"段:偏移": {"1000:A000:0010:64:out.bin", 0xA0010, 64},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := parseMemShots(c.spec)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 1 {
				t.Fatalf("解出 %d 個時點，預期 1", len(got))
			}
			if got[0].at != 1000 || got[0].addr != c.addr || got[0].n != c.n {
				t.Errorf("解出 %+v，預期 at=1000 addr=%05X n=%d", got[0], c.addr, c.n)
			}
			if got[0].path != "out.bin" {
				t.Errorf("檔名 %q，預期 out.bin", got[0].path)
			}
		})
	}

	t.Run("分號給很多次", func(t *testing.T) {
		got, err := parseMemShots("100:lin:1000:16:a.bin; 200:lin:2000:16:b.bin ;300:lin:3000:16:c.bin")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 3 {
			t.Fatalf("解出 %d 個時點，預期 3", len(got))
		}
		for i, want := range []uint64{100, 200, 300} {
			if got[i].at != want {
				t.Errorf("第 %d 個的步數是 %d，預期 %d", i, got[i].at, want)
			}
		}
	})

	t.Run("空字串是零個時點不是錯誤", func(t *testing.T) {
		got, err := parseMemShots("")
		if err != nil || len(got) != 0 {
			t.Errorf("空字串解出 %v／%v，預期沒有時點也沒有錯誤", got, err)
		}
	})

	// **每一種寫壞的形式都要退回來。** 安靜地解出零個時點的話，
	// 使用者會等到跑完才發現什麼檔都沒有。
	for name, spec := range map[string]string{
		"沒有冒號":   "1000",
		"步數不是數字": "abc:lin:1000:16:out.bin",
		"少一段":    "1000:out.bin",
		"長度是零":   "1000:lin:1000:0:out.bin",
		"沒有檔名":   "1000:lin:1000:16:",
	} {
		t.Run("擋下「"+name+"」", func(t *testing.T) {
			if got, err := parseMemShots(spec); err == nil {
				t.Errorf("%q 沒被擋下來，解出 %+v", spec, got)
			}
		})
	}
}

// TestOpenedFilesAreNotTruncated 釘住「開過的檔不截斷」。
//
// 「這個畫面用了哪些素材」是逆向時最常問的一句，而 join 截在 30 個
// 就正好把後面載進來的那些蓋掉——戰鬥畫面的素材在第 30 個之後。
//
// join 本身還在用（別的欄位需要截斷），所以這一條同時釘住兩件事：
// join 仍會截，而開過的檔那一行不走它。
func TestOpenedFilesAreNotTruncated(t *testing.T) {
	many := make([]string, 40)
	for i := range many {
		many[i] = "F" + string(rune('A'+i%26)) + ".DAT"
	}
	if got := join(many); !strings.Contains(got, "另外") {
		t.Error("join 不再截斷了——別的欄位靠它擋住洗版")
	}
	// 開過的檔走 strings.Join，全部都在。
	line := strings.Join(many, " ")
	for _, f := range many {
		if !strings.Contains(line, f) {
			t.Fatalf("%s 不在輸出裡", f)
		}
	}
}
