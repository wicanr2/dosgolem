package eob1_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"

	"github.com/wicanr2/dosgolem/apps/eob1"
	"github.com/wicanr2/dosgolem/oracle"
)

func TestToWestwoodLogoRealData(t *testing.T) {
	o := loadRealData(t)
	defer o.Close()
	if err := eob1.ToWestwoodLogo(o); err != nil {
		t.Fatal(err)
	}
	assertFrame(t, o,
		"b4c544b0dce39640934e0edb9147f79e9c8d34b6dcfc18e2e942abe11a14d0e4",
		"b075ca30a7550c75464d347e4a3a562201d29f09c57faa7153c67bc3d393f6fd")
}

func TestToTitleMenuRealData(t *testing.T) {
	o := loadRealData(t)
	defer o.Close()
	if err := eob1.ToTitleMenu(o); err != nil {
		t.Fatal(err)
	}
	assertFrame(t, o,
		"caa3082b3e8cb5ee15547555669eb82e954982fe919674d5481271e06a253dc0",
		"bf08a3424f429a6cb5400a8ddef50f8ee35ed03deb0de51a307799bbc6b9687f")
	if got := o.PortReads()[0x60]; got != 2 {
		t.Fatalf("主選單前port 60h讀取%d次，預期make／break共2次", got)
	}
}

func TestToNewPartyCreationRealData(t *testing.T) {
	o := loadRealData(t)
	defer o.Close()
	if err := eob1.ToNewPartyCreation(o); err != nil {
		t.Fatal(err)
	}
	if got := digestRegion(o.Indexed(), 32, 8, 256, 40); got != "2496edf386e334b90af5de1a670f171e21dd1591b0b574be84847d69177d85f0" {
		t.Fatalf("建角標題區SHA-256=%s", got)
	}
	if got := digestRegion(o.Indexed(), 130, 55, 180, 140); got != "39cb98087f15b0604262c4767b6a7df50593a4d29e604b4a778db58a2df3e3f6" {
		t.Fatalf("建角操作說明區SHA-256=%s", got)
	}
	if got := o.PortReads()[0x60]; got != 6 {
		t.Fatalf("建角入口前port 60h讀取%d次，預期三鍵make／break共6次", got)
	}
}

func TestToFirstCharacterRaceRealData(t *testing.T) {
	o := loadRealData(t)
	defer o.Close()
	if err := eob1.ToFirstCharacterRace(o); err != nil {
		t.Fatal(err)
	}
	if got := digestRegion(o.Indexed(), 138, 60, 170, 130); got != "34cd6ac1502742323eb299439fe6066154336a9f42f372457eec4d8b5e3b13f6" {
		t.Fatalf("第一角色種族選擇區SHA-256=%s", got)
	}
	if got := o.PortReads()[0x60]; got != 8 {
		t.Fatalf("第一角色種族頁前port 60h讀取%d次，預期四鍵make／break共8次", got)
	}
}

func TestToFirstCharacterClassRealData(t *testing.T) {
	o := loadRealData(t)
	defer o.Close()
	if err := eob1.ToFirstCharacterClass(o); err != nil {
		t.Fatal(err)
	}
	if got := digestRegion(o.Indexed(), 138, 60, 170, 130); got != "731c3a60b6eec89515314cf85aa352bcd4efad241d3aea7bb5106454aee7b890" {
		t.Fatalf("第一角色職業選擇區SHA-256=%s", got)
	}
	if got := o.PortReads()[0x60]; got != 10 {
		t.Fatalf("第一角色職業頁前port 60h讀取%d次，預期五鍵make／break共10次", got)
	}
}

func TestToFirstCharacterAlignmentRealData(t *testing.T) {
	o := loadRealData(t)
	defer o.Close()
	if err := eob1.ToFirstCharacterAlignment(o); err != nil {
		t.Fatal(err)
	}
	if got := digestRegion(o.Indexed(), 138, 60, 170, 130); got != "aade7402dce32dc8a71b9ca7d1ed057836487dc67b27e0fc482cef512da62b40" {
		t.Fatalf("第一角色陣營選擇區SHA-256=%s", got)
	}
	if got := o.PortReads()[0x60]; got != 12 {
		t.Fatalf("第一角色陣營頁前port 60h讀取%d次，預期六鍵make／break共12次", got)
	}
}

func TestToFirstCharacterStatsRealData(t *testing.T) {
	o := loadRealData(t)
	defer o.Close()
	if err := eob1.ToFirstCharacterStats(o); err != nil {
		t.Fatal(err)
	}
	if got := digestRegion(o.Indexed(), 130, 55, 180, 140); got != "d57c5b9740bf8708984ab7da91f950755a1255d06f0bcc4555500d294f95d42e" {
		t.Fatalf("第一角色屬性／肖像區SHA-256=%s", got)
	}
	if got := o.PortReads()[0x60]; got != 14 {
		t.Fatalf("第一角色屬性頁前port 60h讀取%d次，預期七鍵make／break共14次", got)
	}
}

func loadRealData(t *testing.T) *oracle.Oracle {
	t.Helper()
	exe, root := os.Getenv("EOB1_ORACLE_EXE"), os.Getenv("EOB1_ORACLE_ROOT")
	if exe == "" || root == "" {
		t.Skip("未設定EOB1_ORACLE_EXE／EOB1_ORACLE_ROOT；私有原版資料測試明確略過")
	}
	o, err := oracle.Load(exe, root)
	if err != nil {
		t.Fatal(err)
	}
	return o
}

func assertFrame(t *testing.T, o *oracle.Oracle, wantIndexed, wantPalette string) {
	t.Helper()
	if got := digest(o.Indexed()); got != wantIndexed {
		t.Fatalf("色號SHA-256=%s，預期%s", got, wantIndexed)
	}
	pal := o.Palette()
	flat := make([]byte, 0, 768)
	for _, rgb := range pal {
		flat = append(flat, rgb[:]...)
	}
	if got := digest(flat); got != wantPalette {
		t.Fatalf("palette SHA-256=%s，預期%s", got, wantPalette)
	}
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func digestRegion(indexed []byte, x, y, width, height int) string {
	region := make([]byte, 0, width*height)
	for row := y; row < y+height; row++ {
		start := row*oracle.Width + x
		region = append(region, indexed[start:start+width]...)
	}
	return digest(region)
}
