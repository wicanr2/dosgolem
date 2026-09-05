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

func TestToFirstCharacterReviewRealData(t *testing.T) {
	o := loadRealData(t)
	defer o.Close()
	if err := eob1.ToFirstCharacterReview(o); err != nil {
		t.Fatal(err)
	}
	if got := digestRegion(o.Indexed(), 130, 55, 180, 140); got != "1f1bbba36dc25a84f6ce2ebb4f01a60dc64d2b7ddc63bb9ffa3d25d8e47c164f" {
		t.Fatalf("第一角色檢視操作區SHA-256=%s", got)
	}
	if got := o.PortReads()[0x60]; got != 16 {
		t.Fatalf("第一角色檢視頁前port 60h讀取%d次，預期八鍵make／break共16次", got)
	}
}

func TestToFirstCharacterNameRealData(t *testing.T) {
	o := loadRealData(t)
	defer o.Close()
	if err := eob1.ToFirstCharacterName(o); err != nil {
		t.Fatal(err)
	}
	if got := digest(o.Indexed()); got != "ae05e3fff97f306ec714c79603509ec45a4778f152d8ccca2943faef23c47a89" {
		t.Fatalf("第一角色姓名頁SHA-256=%s", got)
	}
	if got := o.MouseCalls()[0x000C]; got != 3 {
		t.Fatalf("姓名頁前AX=0Ch註冊%d次，預期3次", got)
	}
}

func TestToFirstCharacterALFARealData(t *testing.T) {
	o := loadRealData(t)
	defer o.Close()
	if err := eob1.ToFirstCharacterALFA(o); err != nil {
		t.Fatal(err)
	}
	if got := digestRegion(o.Indexed(), 5, 100, 70, 16); got != "cbc568f33b26cf8dd7809e3f5081b8dc9c63049de4ab7f7caf6c406443109b7c" {
		t.Fatalf("第一角色ALFA姓名列SHA-256=%s", got)
	}
	if got := o.PortReads()[0x60]; got != 26 {
		t.Fatalf("ALFA完成前port 60h讀取%d次，預期十三鍵make／break共26次", got)
	}
}

func TestToSecondCharacterRaceRealData(t *testing.T) {
	o := loadRealData(t)
	defer o.Close()
	if err := eob1.ToSecondCharacterRace(o); err != nil {
		t.Fatal(err)
	}
	if got := digestRegion(o.Indexed(), 138, 60, 170, 130); got != "0ac2799d93a1c51adcd884eeefa770b5dc01fbfba36311000a8b5f9d586fc2b3" {
		t.Fatalf("第二角色種族選擇區SHA-256=%s", got)
	}
}

func TestToSecondCharacterReviewRealData(t *testing.T) {
	o := loadRealData(t)
	defer o.Close()
	if err := eob1.ToSecondCharacterReview(o); err != nil {
		t.Fatal(err)
	}
	if got := digestRegion(o.Indexed(), 130, 55, 180, 140); got != "46c7b7647dd6ae8102a8b970315b5597c422a0690308b05668d548d6bdc40759" {
		t.Fatalf("第二角色檢視操作區SHA-256=%s", got)
	}
	if got := o.PortReads()[0x60]; got != 34 {
		t.Fatalf("第二角色檢視頁前port 60h讀取%d次，預期十七鍵make／break共34次", got)
	}
}

func TestToSecondCharacterNameAndBETARealData(t *testing.T) {
	o := loadRealData(t)
	defer o.Close()
	if err := eob1.ToSecondCharacterBETA(o); err != nil {
		t.Fatal(err)
	}
	if got := digestRegion(o.Indexed(), 75, 100, 70, 16); got != "26e91595e8209faf10fa52e34ea9d00477ee0fd6c823d3aa1432e623d2a2258c" {
		t.Fatalf("第二角色BETA姓名列SHA-256=%s", got)
	}
	if got := o.PortReads()[0x60]; got != 44 {
		t.Fatalf("BETA完成前port 60h讀取%d次，預期二十二鍵make／break共44次", got)
	}
}

func TestToThirdCharacterRaceRealData(t *testing.T) {
	o := loadRealData(t)
	defer o.Close()
	if err := eob1.ToThirdCharacterRace(o); err != nil {
		t.Fatal(err)
	}
	if got := digestRegion(o.Indexed(), 130, 55, 180, 140); got != "b566dd781770ae6af1cd569cc84d82e37a38fe82b81405da0c979722f6e0041c" {
		t.Fatalf("第三角色種族選擇區SHA-256=%s", got)
	}
}

func TestToThirdCharacterReviewRealData(t *testing.T) {
	o := loadRealData(t)
	defer o.Close()
	if err := eob1.ToThirdCharacterReview(o); err != nil {
		t.Fatal(err)
	}
	if got := digestRegion(o.Indexed(), 130, 55, 180, 140); got != "832af588ffc848efb145567ac80698b71481fee9da43b2e63892139712998948" {
		t.Fatalf("第三角色檢視操作區SHA-256=%s", got)
	}
	if got := o.PortReads()[0x60]; got != 52 {
		t.Fatalf("第三角色檢視頁前port 60h讀取%d次，預期二十六鍵make／break共52次", got)
	}
}

func TestToThirdCharacterNameAndGAMMARealData(t *testing.T) {
	o := loadRealData(t)
	defer o.Close()
	if err := eob1.ToThirdCharacterGAMMA(o); err != nil {
		t.Fatal(err)
	}
	if got := digestRegion(o.Indexed(), 5, 160, 70, 16); got != "84db489bbd3ecd5cc1050df46382eb3468083941f2fcf2584907c85af15bdef1" {
		t.Fatalf("第三角色GAMMA姓名列SHA-256=%s", got)
	}
	if got := o.PortReads()[0x60]; got != 64 {
		t.Fatalf("GAMMA完成前port 60h讀取%d次，預期三十二鍵make／break共64次", got)
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
