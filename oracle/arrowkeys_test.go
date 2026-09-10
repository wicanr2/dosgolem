package oracle_test

import (
	"testing"

	"github.com/wicanr2/dosgolem/oracle"
)

// TestArrowKeyScanCodes 釘住四個方向鍵的 set-1 掃描碼。
//
// 為什麼值得一支測試：**左右鍵很容易被當成「用不到」而漏掉**。選單類的畫面
// 只讀上下，所以上下先補了、左右一直沒有；直到某個只有左右能操作的畫面
// （大富翁2 的遊樂場，`rich2/docs/re/139` §2）才發現送不出去，而那時的症狀
// 是「棋子整場不動」——看起來像遊戲不吃鍵盤，不像少了兩個常數。
//
// 正反對照：四個鍵的碼互不相同，而且與 Esc／Enter 也不同（打錯一個字就會
// 送成別的鍵，那種錯誤不會報，只會讓畫面沒反應）。
func TestArrowKeyScanCodes(t *testing.T) {
	for _, c := range []struct {
		name string
		key  oracle.Key
		want uint8
	}{
		{"上", oracle.KeyUp, 0x48},
		{"左", oracle.KeyLeft, 0x4B},
		{"右", oracle.KeyRight, 0x4D},
		{"下", oracle.KeyDown, 0x50},
	} {
		if uint8(c.key) != c.want {
			t.Errorf("%s鍵的掃描碼是 %#x，應該是 %#x", c.name, uint8(c.key), c.want)
		}
	}
	seen := map[oracle.Key]string{
		oracle.KeyEscape: "Esc", oracle.KeyEnter: "Enter",
		oracle.KeyUp: "上", oracle.KeyDown: "下",
	}
	for _, c := range []struct {
		key  oracle.Key
		name string
	}{{oracle.KeyLeft, "左"}, {oracle.KeyRight, "右"}} {
		if other, dup := seen[c.key]; dup {
			t.Errorf("%s鍵與 %s 鍵撞碼（都是 %#x）", c.name, other, uint8(c.key))
		}
		seen[c.key] = c.name
	}
}
