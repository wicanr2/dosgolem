package main

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/machine"
)

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

func TestParseHolds(t *testing.T) {
	hs, err := parseHolds("space@100+2000, 39@50+1000ms", machine.StepsPerSecond())
	if err != nil {
		t.Fatal(err)
	}
	if len(hs) != 2 || hs[0].scan != 0x39 || hs[0].at != 100 || hs[0].dur != 2000 {
		t.Fatalf("第一組解成 %+v", hs)
	}
	if want := uint64(machine.StepsPerSecond()); hs[1].scan != 0x39 || hs[1].at != 50 || hs[1].dur != want {
		t.Errorf("第二組 %+v，長度要 %d（1000ms）", hs[1], want)
	}
	for _, bad := range []string{"space@100", "space+100", "nope@1+1", "space@1+0", "space@x+1"} {
		if _, err := parseHolds(bad, machine.StepsPerSecond()); err == nil {
			t.Errorf("%q 應該報錯", bad)
		}
	}
}

func TestParseCycles(t *testing.T) {
	for spec, want := range map[string]uint64{"xt": machine.CyclesXT, "AT8": machine.CyclesAT8, "at12": machine.CyclesAT12, "500": 500} {
		if got, err := parseCycles(spec); err != nil || got != want {
			t.Errorf("%q → %d, %v；要 %d", spec, got, err, want)
		}
	}
	for _, bad := range []string{"", "0", "fast", "-1"} {
		if _, err := parseCycles(bad); err == nil {
			t.Errorf("%q 應該報錯", bad)
		}
	}
	hs, err := parseHolds("space@0+1000ms", float64(machine.CyclesXT*1000))
	if err != nil || hs[0].dur != machine.CyclesXT*1000 {
		t.Errorf("XT 速度 1000ms 解成 %+v, %v；要 %d 道指令", hs, err, machine.CyclesXT*1000)
	}
}
