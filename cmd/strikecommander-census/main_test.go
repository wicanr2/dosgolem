package main

import "testing"

// mem 是測試用的線性記憶體：只放入指定位址的 bytes，其餘讀 0。
type mem map[uint32]uint8

func (m mem) put(cs, ip uint16, b []byte) {
	for i, v := range b {
		m[uint32(cs)<<4+uint32(ip)+uint32(i)] = v
	}
}

func (m mem) read(a uint32) uint8 { return m[a] }

// strikePutsBytes 是 RE 020 §3.1 記下的 STRIKE puts（61BB:0345）前 16 bytes，獨立寫出，不取 putsSig。
var strikePutsBytes = []byte{0x55, 0x8B, 0xEC, 0x83, 0xEC, 0x06, 0x56, 0x57, 0x8B, 0x76, 0x06, 0x8B, 0x7E, 0x08, 0xFF, 0x74}

func TestDetectPutsStrike(t *testing.T) {
	m := mem{}
	m.put(0x61BB, 0x0345, strikePutsBytes)
	p, base, ok := detectPuts(m.read, 0x61BB, 0x0345, putsProfiles)
	if !ok || p.name != "strike" || base != 0x0241 {
		t.Fatalf("STRIKE 61BB:0345：ok=%v profile=%q base=%04X，期望 strike、0241", ok, p.name, base)
	}
	if p.blitSegOff != 0 {
		t.Fatalf("STRIKE 不應有 blit 段偏移，得 %04X", p.blitSegOff)
	}
	// 只限 opttest 時不得認出 STRIKE。
	only, _ := selectProfiles("opttest")
	if _, _, ok := detectPuts(m.read, 0x61BB, 0x0345, only); ok {
		t.Fatal("-puts-profile opttest 卻認出 STRIKE 的 puts")
	}
}

func TestDetectPutsOpttest(t *testing.T) {
	m := mem{}
	m.put(0x3B83, 0x034B, putsSig)
	p, base, ok := detectPuts(m.read, 0x3B83, 0x034B, putsProfiles)
	if !ok || p.name != "opttest" || base != 0x3B83-0x3942 || p.blitSegOff != 0x39FB {
		t.Fatalf("OPTTEST 3B83:034B：ok=%v profile=%q base=%04X blit=%04X", ok, p.name, base, p.blitSegOff)
	}
}

func TestDetectPutsNegative(t *testing.T) {
	m := mem{}
	bad := append([]byte(nil), strikePutsBytes...)
	bad[15] ^= 0xFF // 末 byte 不同
	m.put(0x61BB, 0x0345, bad)
	if _, _, ok := detectPuts(m.read, 0x61BB, 0x0345, putsProfiles); ok {
		t.Fatal("簽章末 byte 不同仍判定為 puts")
	}
	m2 := mem{}
	m2.put(0x61BB, 0x0345, strikePutsBytes)
	// IP 不對。
	if _, _, ok := detectPuts(m2.read, 0x61BB, 0x0346, putsProfiles); ok {
		t.Fatal("IP 0346 判定為 puts")
	}
	// CS 小於段偏移：同樣的 bytes 放在 1000:0345，base 會是負值，不得判定。
	m3 := mem{}
	m3.put(0x1000, 0x0345, strikePutsBytes)
	if _, _, ok := detectPuts(m3.read, 0x1000, 0x0345, putsProfiles); ok {
		t.Fatal("CS 1000 < 5F7A 仍判定為 STRIKE puts")
	}
}

func TestSelectProfiles(t *testing.T) {
	for _, c := range []struct {
		in   string
		n    int
		fail bool
	}{{"auto", 2, false}, {"", 2, false}, {"strike", 1, false}, {"opttest", 1, false}, {"menu", 0, true}} {
		ps, err := selectProfiles(c.in)
		if (err != nil) != c.fail || len(ps) != c.n {
			t.Errorf("%q：%d 個、err=%v", c.in, len(ps), err)
		}
	}
}
