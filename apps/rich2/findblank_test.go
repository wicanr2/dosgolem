package rich2

import (
	"testing"

	"github.com/wicanr2/dosgolem/oracle"
)

// blankScreen 造一張測試畫面：底色綠、指定區塊換色號。
func blankScreen(fill uint8) []uint8 {
	px := make([]uint8, oracle.Width*oracle.Height)
	for i := range px {
		px[i] = fill
	}
	return px
}

func greenPalette() [256][3]uint8 {
	var pal [256][3]uint8
	// 縣市全綠。
	for c := countyFirst; c <= countyLast; c++ {
		pal[c] = [3]uint8{0, 130, 0}
	}
	return pal
}

func paint(px []uint8, x0, y0, w, h int, c uint8) {
	for y := y0; y < y0+h; y++ {
		for x := x0; x < x0+w; x++ {
			px[y*oracle.Width+x] = c
		}
	}
}

// TestFindBlankPicksTheWhiteCounty 正常情況：某一個縣市的調色盤被設成白。
func TestFindBlankPicksTheWhiteCounty(t *testing.T) {
	px := blankScreen(countyFirst)
	pal := greenPalette()
	paint(px, 20, 60, 10, 10, 200)
	pal[200] = [3]uint8{255, 255, 255}
	got := findBlankIn(px, pal)
	if got.N != 100 || got.Colour != 200 || got.CX != 24 || got.CY != 64 {
		t.Fatalf("找到 %+v，預期 100 點、色號 200、中心 (24,64)", got)
	}
}

// TestFindBlankIgnoresOrdinaryWhite 是這一支存在的理由。
//
// 畫面上的白色文字走的是**一般的白**（色號 15），不是縣市色號。
// 本函式原本沒有限制色號，理由是「色號 15 一個點都找不到」——
// **那句話只在一個已經修掉的 dosgolem bug 底下成立**（`int 10h AH=10h`
// 的 `AL=10h` 被接成設屬性暫存器，色號 15 的 DAC 從來沒被寫成白）。
// 共用層修對之後，`SolvePassword` 在三題都答對之後把白色文字當成第四題的
// 留白區，然後回報「四個顏色都不被接受」。
//
// 教訓寫成規則：**「某個東西從來不出現」如果沒有結構上的理由，
// 就不能當判準——它只是還沒出現。**
func TestFindBlankIgnoresOrdinaryWhite(t *testing.T) {
	px := blankScreen(countyFirst)
	pal := greenPalette()
	// 一般的白色文字，落在地圖範圍內。
	paint(px, 30, 100, 8, 8, 15)
	pal[15] = [3]uint8{255, 255, 255}
	if got := findBlankIn(px, pal); got.N != 0 {
		t.Fatalf("把色號 %d 的白當成留白區了：%+v", got.Colour, got)
	}
	// 同一張畫面加上真正的留白縣市，要找得到、而且只算縣市那一塊。
	paint(px, 20, 60, 10, 10, 200)
	pal[200] = [3]uint8{255, 255, 255}
	if got := findBlankIn(px, pal); got.N != 100 || got.Colour != 200 {
		t.Fatalf("有白色文字干擾時找到 %+v，預期 100 點、色號 200", got)
	}
}

// TestFindBlankIgnoresOutsideTheMap 地圖範圍外的白不算。
func TestFindBlankIgnoresOutsideTheMap(t *testing.T) {
	px := blankScreen(countyFirst)
	pal := greenPalette()
	paint(px, mapRight+5, 60, 10, 10, 200)
	pal[200] = [3]uint8{255, 255, 255}
	if got := findBlankIn(px, pal); got.N != 0 {
		t.Fatalf("地圖範圍外的白被算進去了：%+v", got)
	}
}
