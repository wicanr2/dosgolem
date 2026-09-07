package main

import "testing"

// TestParsePokes 釘住 `-poke` 的格式。寫錯的話會安靜地什麼都不做——
// 畫面照樣出得來，只是狀態不是你以為的那個。
func TestParsePokes(t *testing.T) {
	ps, err := parsePokes("0040:0049@100000=07;lin:400@1500=AA BB")
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 2 {
		t.Fatalf("解出 %d 筆，預期 2", len(ps))
	}
	if ps[0].addr != 0x449 || ps[0].at != 100000 || len(ps[0].data) != 1 || ps[0].data[0] != 7 {
		t.Fatalf("第一筆不對：%+v", ps[0])
	}
	if ps[1].addr != 0x400 || ps[1].at != 1500 || len(ps[1].data) != 2 {
		t.Fatalf("第二筆不對：%+v", ps[1])
	}
	for _, bad := range []string{"0040:0049=07", "0040:0049@100", "0040:0049@100=ZZ", "0040:0049@100=", "zz@1=07"} {
		if _, err := parsePokes(bad); err == nil {
			t.Fatalf("%q 應該要報錯", bad)
		}
	}
}
