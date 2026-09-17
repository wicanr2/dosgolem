package xlate

import "testing"

// spec 202 §3 第 5 項：位址連續＋remain 遞減是同一行；remain 不遞減（useRemain）是新行；
// 位址跳號是新行。
func TestLineTrackerSameLine(t *testing.T) {
	var lt LineTracker
	if !lt.Hit(0x6289, 5, true) {
		t.Error("第一次命中沒有上一次可比，應該是新行")
	}
	if lt.Hit(0x628A, 4, true) {
		t.Error("位址+1、remain-1（同一行的下一個字）不該是新行")
	}
	if lt.Hit(0x628B, 3, true) {
		t.Error("位址+1、remain 繼續遞減，仍是同一行")
	}
}

func TestLineTrackerRemainNotDecreasing(t *testing.T) {
	var lt LineTracker
	lt.Hit(0x6289, 5, true)
	if !lt.Hit(0x628A, 5, true) {
		t.Error("useRemain 時 remain 沒遞減（沒等於上一次 -1），應該判成新行")
	}
}

func TestLineTrackerAddressJump(t *testing.T) {
	var lt LineTracker
	lt.Hit(0x6289, 5, true)
	lt.Hit(0x628A, 4, true)
	if !lt.Hit(0x7000, 3, true) {
		t.Error("位址跳號應該是新行")
	}
}

// useRemain=false 時只看位址，不管 remain 有沒有遞減。
func TestLineTrackerIgnoreRemain(t *testing.T) {
	var lt LineTracker
	lt.Hit(0x1000, 99, false)
	if lt.Hit(0x1001, 1, false) {
		t.Error("不看 remain 時，位址+1 應該仍是同一行")
	}
	if !lt.Hit(0x2000, 1, false) {
		t.Error("位址跳號應該是新行")
	}
}
