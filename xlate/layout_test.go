package xlate

import (
	"errors"
	"testing"
)

// spec 202 §3 第 1 項，搬自 psychic_war_cht apps/psychicwar/overlay。

func TestLayout(t *testing.T) {
	got, err := Layout("這裡無法前進。", []int{16, 15})
	if err != nil || string(got[0]) != "這裡無法前進。" || len(got[1]) != 0 {
		t.Errorf("7 字：%q %q %v", string(got[0]), string(got[1]), err)
	}
	s18 := "一二三四五六七八九十一二三四五六七八"
	got, err = Layout(s18, []int{16, 15})
	if err != nil || len(got[0]) != 16 || string(got[1]) != "七八" {
		t.Errorf("18 字：%d %q %v", len(got[0]), string(got[1]), err)
	}
	got, err = Layout(s18+s18[:14*3], []int{16, 15}) // 32 字
	if !errors.Is(err, ErrTooLong) || len(got[0]) != 16 || len(got[1]) != 15 {
		t.Errorf("32 字：%d %d %v", len(got[0]), len(got[1]), err)
	}
	got, _ = Layout("甲\n乙", []int{16, 15})
	if string(got[0]) != "甲" || string(got[1]) != "乙" {
		t.Errorf("換行：%q %q", string(got[0]), string(got[1]))
	}
}
