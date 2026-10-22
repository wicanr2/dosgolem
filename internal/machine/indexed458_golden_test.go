package machine

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// docs/spec/458-session-indexed-into-and-videowrite-ctx §3.1 第 2、3 項的金樣雜湊與固定內容。
//
// 本檔只用 2436db8 已有的 API（Indexed、SetVideoMode、Out8），同一個檔案在 2436db8 上原樣通過
// （§5 動工前第 1 步）。新 API 的測試在 indexed458_test.go，共用這裡的建法。
//
// 金樣產生方式：在 2436db8 的獨立樹（git archive 2436db8 解到
// strike_commander/workplace/impl458/base-2436db8/），把本檔複製進 internal/machine，
// 常數留空時執行
//
//	go test -count=1 -run '^TestIndexed458Golden$' -v ./internal/machine
//
// 測試以 t.Errorf 印出 Indexed() 的 SHA-256，再填回下列常數；填好後在同一棵樹重跑通過。
// 產生日 2026-10-23，commit 2436db8c6a1418293a7590cfb3dc96402c7c996b，容器映像 dosgolem-go-ebiten:1.24-r1（go1.24.13）。

const (
	golden458Unchained = "d4b172348379d30413c4ca8779f7fd8379b17256b2c191faaef3dd1e4473c8a6" // 非鏈結 13h（seed458Unchained），2436db8
	golden458Planar12  = "a841eeb7812cd51c0785283c9674dfd88686f94edef4aa818dde80c5f7e22c69" // mode 12h（seed458Planar12），2436db8
	golden458Planar0D  = "ea085e5706054cc6a30c29cf28ae2b9b40afa209c7084e22a741cf57b7c26933" // mode 0Dh（seed458Planar0D），2436db8
)

const (
	seed458Unchained = 0x458A
	seed458Planar12  = 0x4512
	seed458Planar0D  = 0x450D
)

// lcg458 是測試自寫的線性同餘產生器（不用全域亂數；常數取自 Numerical Recipes）。
type lcg458 uint32

func (l *lcg458) next() uint8 {
	*l = *l*1664525 + 1013904223
	return uint8(*l >> 24)
}

// fill458Planes 以 seed 填滿四個平面。
func fill458Planes(m *Machine, seed uint32) {
	r := lcg458(seed)
	for p := 0; p < 4; p++ {
		for i := range m.VGA.Planes[p] {
			m.VGA.Planes[p][i] = r.next()
		}
	}
}

// crtc458 經埠 3D4h／3D5h 寫 CRTC（與程式寫暫存器同一條路）。
func crtc458(m *Machine, idx, v uint8) {
	m.Out8(0x3D4, idx)
	m.Out8(0x3D5, v)
}

// setLineCompare458 把 line compare 設成 lc（10 位元：18h 低 8 位、07h bit4、09h bit6），其餘位元不動。
func setLineCompare458(m *Machine, lc int) {
	c7, c9 := m.VGA.crtc[0x07]&^0x10, m.VGA.crtc[0x09]&^0x40
	c7 |= uint8(lc>>8&1) << 4
	c9 |= uint8(lc>>9&1) << 6
	crtc458(m, 0x18, uint8(lc))
	crtc458(m, 0x07, c7)
	crtc458(m, 0x09, c9)
}

// setup458Unchained：mode 13h 後經埠 3C4h／3C5h 清鏈結位元（Mode X），顯示起點暫存器 0123h、
// 列距 28h（列距 80 bytes）、line compare 200 掃描線（mode 13h 每列掃兩次 → 第 100 列）。
func setup458Unchained(m *Machine) {
	m.SetVideoMode(0x13)
	m.Out8(0x3C4, 0x04)
	m.Out8(0x3C5, 0x06) // bit3（chain4）清掉
	fill458Planes(m, seed458Unchained)
	crtc458(m, 0x0C, 0x01)
	crtc458(m, 0x0D, 0x23)
	crtc458(m, 0x13, 0x28)
	setLineCompare458(m, 200)
}

// setup458Planar：16 色平面模式 mode，顯示起點暫存器 0321h，line compare lc（不除掃描數）。
func setup458Planar(m *Machine, mode uint8, seed uint32, lc int) {
	m.SetVideoMode(mode)
	fill458Planes(m, seed)
	crtc458(m, 0x0C, 0x03)
	crtc458(m, 0x0D, 0x21)
	setLineCompare458(m, lc)
}

type case458 struct {
	name   string
	golden string
	setup  func(m *Machine)
}

func cases458() []case458 {
	return []case458{
		{"非鏈結 13h", golden458Unchained, setup458Unchained},
		{"mode 12h", golden458Planar12, func(m *Machine) { setup458Planar(m, 0x12, seed458Planar12, 300) }},
		{"mode 0Dh", golden458Planar0D, func(m *Machine) { setup458Planar(m, 0x0D, seed458Planar0D, 150) }},
	}
}

func sha458(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// TestIndexed458Golden：Indexed() 對固定內容的 SHA-256 等於 2436db8 錄下的金樣。
func TestIndexed458Golden(t *testing.T) {
	for _, c := range cases458() {
		m := New()
		c.setup(m)
		got := m.Indexed()
		w, h := m.VideoSize()
		t.Logf("%s：VideoSize %d×%d、DisplayStart %#x、LineCompare %d、len %d、unchained %v、planarOn %v",
			c.name, w, h, m.DisplayStart(), m.VGA.LineCompare(), len(got), m.Unchained(), m.planarOn)
		if s := sha458(got); s != c.golden {
			t.Errorf("%s：Indexed() SHA-256 %s，金樣 %q", c.name, s, c.golden)
		}
	}
}
