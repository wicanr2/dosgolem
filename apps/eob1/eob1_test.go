package eob1_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
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

func TestToTitleInvalidSaveErrorAndReturnRealData(t *testing.T) {
	tests := []struct {
		name string
		path func(*oracle.Oracle) error
		want string
		port uint64
	}{
		{"error", eob1.ToTitleInvalidSaveError, "aaa57e4d66a41bc0eaeb464a9318c5aa5bd1c92e369c5a6df633be21e44f3c43", 4},
		{"return", eob1.ToTitleInvalidSaveReturn, "caa3082b3e8cb5ee15547555669eb82e954982fe919674d5481271e06a253dc0", 6},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			o := loadInvalidSaveData(t)
			defer o.Close()
			if err := test.path(o); err != nil {
				t.Fatal(err)
			}
			if got := digest(o.Indexed()); got != test.want {
				t.Fatalf("無效存檔全畫面SHA-256=%s", got)
			}
			if got := o.PortReads()[0x60]; got != test.port {
				t.Fatalf("無效存檔路徑port 60h讀取%d次，預期%d次", got, test.port)
			}
		})
	}
}

func TestToSavedGameEntranceRealData(t *testing.T) {
	o := loadRealData(t)
	defer o.Close()
	if err := eob1.ToSavedGameEntrance(o); err != nil {
		t.Fatal(err)
	}
	if got := digest(o.Indexed()); got != "823a0224b25517894968eb0cd1ba95bc5e8e3533084b31a75964f13c93639e1b" {
		t.Fatalf("有效存檔全畫面SHA-256=%s", got)
	}
	if got := digestRegion(o.Indexed(), 0, 0, 176, 120); got != "13de273d44682a02ac7b72d4dc3baa8356d1b1394b5e05cbb7f88a0f17963546" {
		t.Fatalf("有效存檔地城視窗SHA-256=%s", got)
	}
	if got := o.PortReads()[0x60]; got != 4 {
		t.Fatalf("有效存檔載入port 60h讀取%d次，預期兩鍵make／break共4次", got)
	}
}

func TestToSavedGameProtectionCastOnArielRealData(t *testing.T) {
	o := loadRealData(t)
	defer o.Close()
	if err := eob1.ToSavedGameProtectionCastOnAriel(o); err != nil {
		t.Fatal(err)
	}
	if got := digestRegion(o.Indexed(), 0, 168, 320, 32); got != "fd53da2992f12eda5ce14e4256a3e5f08aeea342aeac3f5ef5f4eb1b5ffbb934" {
		t.Fatalf("Protection From Evil施放訊息區SHA-256=%s", got)
	}
	if got := o.PortReads()[0x60]; got != 32 {
		t.Fatalf("Protection From Evil施放port 60h讀取%d次，預期十六鍵make／break共32次", got)
	}
}

func TestSavedGameProtectionTargetIgnoresZAndEnterRealData(t *testing.T) {
	o := loadRealData(t)
	defer o.Close()
	if err := eob1.ToSavedGameProtectionTargeting(o); err != nil {
		t.Fatalf("%v current=%s port=%d", err, digest(o.Indexed()), o.PortReads()[0x60])
	}
	want := append([]byte(nil), o.Indexed()...)
	for _, key := range []oracle.Key{oracle.KeyZ, oracle.KeyEnter} {
		o.PressKey(key)
		if err := o.Run(1_000_000); err != nil {
			t.Fatal(err)
		}
	}
	if got := digest(o.Indexed()); got != digest(want) {
		t.Fatalf("Z／Enter改變原版目標畫面SHA-256=%s", got)
	}
	if got := o.PortReads()[0x60]; got != 36 {
		t.Fatalf("原版目標鍵盤探針port 60h讀取%d次，預期十八鍵make／break共36次", got)
	}
}

func TestToSavedGameProtectionBookClosedRealData(t *testing.T) {
	o := loadRealData(t)
	defer o.Close()
	if err := eob1.ToSavedGameProtectionBookClosed(o); err != nil {
		t.Fatal(err)
	}
	if got := digestRegion(o.Indexed(), 0, 0, 176, 100); got != "689c6d86fd088a78820474561dc899094714b396184bd492598e4f36d066100a" {
		t.Fatalf("法術書關閉後地城視窗SHA-256=%s", got)
	}
	if got := o.PortReads()[0x60]; got != 32 {
		t.Fatalf("法術書中止關閉port 60h讀取%d次，預期十六鍵make／break共32次", got)
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

func TestToFourthCharacterRaceRealData(t *testing.T) {
	o := loadRealData(t)
	defer o.Close()
	if err := eob1.ToFourthCharacterRace(o); err != nil {
		t.Fatal(err)
	}
	if got := digestRegion(o.Indexed(), 130, 55, 180, 140); got != "b566dd781770ae6af1cd569cc84d82e37a38fe82b81405da0c979722f6e0041c" {
		t.Fatalf("第四角色種族選擇區SHA-256=%s", got)
	}
}

func TestToFourthCharacterReviewRealData(t *testing.T) {
	o := loadRealData(t)
	defer o.Close()
	if err := eob1.ToFourthCharacterReview(o); err != nil {
		t.Fatal(err)
	}
	if got := digestRegion(o.Indexed(), 130, 55, 180, 140); got != "43c8dc52f9f80fad0b54067648f574b297f8dd194a0b40dcd302c1abe3cc6ab9" {
		t.Fatalf("第四角色檢視操作區SHA-256=%s", got)
	}
	if got := o.PortReads()[0x60]; got != 72 {
		t.Fatalf("第四角色檢視頁前port 60h讀取%d次，預期三十六鍵make／break共72次", got)
	}
}

func TestToFourthCharacterNameAndDELTARealData(t *testing.T) {
	o := loadRealData(t)
	defer o.Close()
	if err := eob1.ToFourthCharacterDELTA(o); err != nil {
		t.Fatal(err)
	}
	if got := digestRegion(o.Indexed(), 75, 160, 70, 16); got != "5565425afaedff49bad39cd17eac469ec74ee8459698501d1623834ef15c2584" {
		t.Fatalf("第四角色DELTA姓名列SHA-256=%s", got)
	}
	if got := o.PortReads()[0x60]; got != 84 {
		t.Fatalf("DELTA完成前port 60h讀取%d次，預期四十二鍵make／break共84次", got)
	}
}

func TestToLevel1EntranceRealData(t *testing.T) {
	o := loadRealData(t)
	defer o.Close()
	if err := eob1.ToLevel1Entrance(o); err != nil {
		t.Fatal(err)
	}
	if got := digestRegion(o.Indexed(), 0, 0, 176, 120); got != "2ef2c0240070bce02b59735c5266fc6163eee170ea8c135982a469f04bb2abbc" {
		t.Fatalf("LEVEL1第一格視窗SHA-256=%s", got)
	}
	if got := o.PortReads()[0x60]; got != 84 {
		t.Fatalf("LEVEL1入口前port 60h讀取%d次，預期四十二鍵make／break共84次", got)
	}
}

func TestToLevel1InventoryCharacterPagesRealData(t *testing.T) {
	tests := []struct {
		name string
		path func(*oracle.Oracle) error
		want string
	}{
		{"open ALFA", eob1.ToLevel1FirstInventory, "2d77f8c80423d94e0d28ba9a562a6df86abbe0a587f10ef46e4d19e1b2e95809"},
		{"next BETA", eob1.ToLevel1InventoryNextCharacter, "21d28f3144af98f890f13139469364a9334b78443858d709bba3c81c231350df"},
		{"previous ALFA", eob1.ToLevel1InventoryPreviousCharacter, "2d77f8c80423d94e0d28ba9a562a6df86abbe0a587f10ef46e4d19e1b2e95809"},
		{"return exploration", eob1.ToLevel1InventoryReturn, "b4fd5e6d1a6b2740cc9f390f50d28c65649a4a7c99832e7f27766340a0ebcf26"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			o := loadRealData(t)
			defer o.Close()
			if err := test.path(o); err != nil {
				t.Fatal(err)
			}
			if got := digest(o.Indexed()); got != test.want {
				t.Fatalf("全畫面SHA-256=%s", got)
			}
			if got := o.PortReads()[0x60]; got != 84 {
				t.Fatalf("物品欄路徑port 60h讀取%d次，預期四十二鍵make／break共84次", got)
			}
		})
	}
}

func TestToLevel1CharacterExchangeCancelRealData(t *testing.T) {
	tests := []struct {
		name string
		path func(*oracle.Oracle) error
		want string
	}{
		{"selected", eob1.ToLevel1CharacterExchangeSelected, "7106257fa9aafd131af18e4623ddb4a094f922f32fcdee95a3d0c887ac6752df"},
		{"cancelled", eob1.ToLevel1CharacterExchangeCancel, "3064a9c9eea5e6f1be2f18915868cd076cce96778e0e25bf9fb0e12995bd325f"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			o := loadRealData(t)
			defer o.Close()
			if err := test.path(o); err != nil {
				t.Fatal(err)
			}
			if got := digest(o.Indexed()); got != test.want {
				t.Fatalf("角色交換全畫面SHA-256=%s", got)
			}
			if got := digestRegion(o.Indexed(), 0, 0, 176, 120); got != "2ef2c0240070bce02b59735c5266fc6163eee170ea8c135982a469f04bb2abbc" {
				t.Fatalf("角色交換左側視窗SHA-256=%s", got)
			}
			if got := o.PortReads()[0x60]; got != 84 {
				t.Fatalf("角色交換port 60h讀取%d次，預期四十二鍵make／break共84次", got)
			}
		})
	}
}

func TestToLevel1FirstForwardStepRealData(t *testing.T) {
	o := loadRealData(t)
	defer o.Close()
	if err := eob1.ToLevel1FirstForwardStep(o); err != nil {
		t.Fatal(err)
	}
	if got := digestRegion(o.Indexed(), 0, 0, 176, 120); got != "7fc8b674d93e02578a2dfe79e54232899c4870bb2122e1b89eaf040477e72f08" {
		t.Fatalf("LEVEL1第一步視窗SHA-256=%s", got)
	}
	if got := o.PortReads()[0x60]; got != 88 {
		t.Fatalf("LEVEL1第一步前port 60h讀取%d次，預期四十四鍵make／break共88次", got)
	}
}

func TestToLevel1FirstPickupRealData(t *testing.T) {
	o := loadRealData(t)
	defer o.Close()
	if err := eob1.ToLevel1FirstPickup(o); err != nil {
		t.Fatal(err)
	}
	if got := digestRegion(o.Indexed(), 0, 0, 176, 120); got != "1d123d4fe9a5a1001446f2d180601e8713fa2dd02f00caac4c9f406f31b59375" {
		t.Fatalf("LEVEL1起始石塊拾取視窗SHA-256=%s", got)
	}
	if got := digest(o.Indexed()); got != "e282a5980cb8ce9d5fee0dc64b6d66ad399cabf3e3bae6edb1cde64e6618909f" {
		t.Fatalf("LEVEL1起始石塊拾取全畫面SHA-256=%s", got)
	}
	if got := o.PortReads()[0x60]; got != 84 {
		t.Fatalf("LEVEL1拾取前port 60h讀取%d次，預期四十二鍵make／break共84次", got)
	}
}

func TestToLevel1FirstDropRealData(t *testing.T) {
	o := loadRealData(t)
	defer o.Close()
	if err := eob1.ToLevel1FirstDrop(o); err != nil {
		t.Fatal(err)
	}
	if got := digestRegion(o.Indexed(), 0, 0, 176, 120); got != "4cf790c278d6a9115094c61eebacf894d13ad12a118403ed25056c42523352d2" {
		t.Fatalf("LEVEL1起始石塊放回視窗SHA-256=%s", got)
	}
	if got := digest(o.Indexed()); got != "9ce3f36cf412325f991ef4bf542361661376e840fb1fcc17436c1cb99028d9f3" {
		t.Fatalf("LEVEL1起始石塊放回全畫面SHA-256=%s", got)
	}
	if got := o.PortReads()[0x60]; got != 84 {
		t.Fatalf("LEVEL1放回後port 60h讀取%d次，預期四十二鍵make／break共84次", got)
	}
}

func TestToLevel1CampRealData(t *testing.T) {
	o := loadRealData(t)
	defer o.Close()
	if err := eob1.ToLevel1Camp(o); err != nil {
		t.Fatal(err)
	}
	if got := digest(o.Indexed()); got != "79e080c1a738e5ecd7d26ff379d7a126cb9ba4cee7ea998af94555b7a3d4f8ae" {
		t.Fatalf("LEVEL1 CAMP根選單SHA-256=%s", got)
	}
	if got := o.PortReads()[0x60]; got != 84 {
		t.Fatalf("LEVEL1 CAMP前port 60h讀取%d次，預期四十二鍵make／break共84次", got)
	}
}

func TestToLevel1CampExitKeyboardRealData(t *testing.T) {
	tests := []struct {
		name string
		path func(*oracle.Oracle) error
		want string
		port uint64
	}{
		{"selected", eob1.ToLevel1CampExitSelected, "36a8f388e9a480cdf041124127c2e4d55cec59859aaafed32ed15807a6f0565e", 86},
		{"confirmed", eob1.ToLevel1CampExitConfirmed, "6766d6f9ae4084a3f02789c5d05261ac9e22e112ca51fd710c1a68c7e6407a96", 88},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			o := loadRealData(t)
			defer o.Close()
			if err := test.path(o); err != nil {
				t.Fatal(err)
			}
			if got := digest(o.Indexed()); got != test.want {
				t.Fatalf("CAMP Exit %s全畫面SHA-256=%s", test.name, got)
			}
			if got := o.PortReads()[0x60]; got != test.port {
				t.Fatalf("CAMP Exit %s port 60h讀取%d次，預期%d次", test.name, got, test.port)
			}
		})
	}
}

func TestToLevel1CampGameOptionsRealData(t *testing.T) {
	tests := []struct {
		name string
		path func(*oracle.Oracle) error
		want string
		port uint64
	}{
		{"options", eob1.ToLevel1CampGameOptions, "2207602e9d4e144c6fadea7d30cd4a189b9c09cb5fd0269d2ed8ed6f0a86d912", 96},
		{"exit", eob1.ToLevel1CampGameOptionsExit, "6766d6f9ae4084a3f02789c5d05261ac9e22e112ca51fd710c1a68c7e6407a96", 100},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			o := loadRealData(t)
			defer o.Close()
			if err := test.path(o); err != nil {
				t.Fatal(err)
			}
			if test.name == "options" && digest(o.Indexed()) != test.want {
				t.Fatalf("Game Options全畫面SHA-256=%s", digest(o.Indexed()))
			}
			if test.name == "exit" && digestRegion(o.Indexed(), 0, 0, 176, 120) != "2ef2c0240070bce02b59735c5266fc6163eee170ea8c135982a469f04bb2abbc" {
				t.Fatalf("Game Options返回地城視窗SHA-256=%s", digestRegion(o.Indexed(), 0, 0, 176, 120))
			}
			if got := o.PortReads()[0x60]; got != test.port {
				t.Fatalf("Game Options %s port 60h讀取%d次，預期%d次", test.name, got, test.port)
			}
		})
	}
}

func TestToLevel1CampSaveCancelRealData(t *testing.T) {
	tests := []struct {
		name string
		path func(*oracle.Oracle) error
		want string
	}{
		{"confirmation", eob1.ToLevel1CampSaveConfirmation, "40002452b7022088be0995c5dcc3a98c394786e3481774db36ab50c2c978e768"},
		{"cancel", eob1.ToLevel1CampSaveCancel, "152007f23678f6d424ebe6db999fba60044b044ed645930cb36c4d1f17011d49"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			o := loadRealData(t)
			defer o.Close()
			if err := test.path(o); err != nil {
				t.Fatal(err)
			}
			var got string
			if test.name == "confirmation" {
				got = digest(o.Indexed())
			} else {
				got = digestRegion(o.Indexed(), 0, 0, 176, 96)
			}
			if got != test.want {
				t.Fatalf("Save Game %s SHA-256=%s", test.name, got)
			}
			if got := o.PortReads()[0x60]; got != 100 {
				t.Fatalf("Save Game %s port 60h讀取%d次，預期100次", test.name, got)
			}
			for _, name := range o.Opened() {
				if strings.HasSuffix(strings.ToUpper(name), "EOBDATA.SAV") {
					t.Fatalf("Save Game %s不應開啟原版存檔：%s", test.name, name)
				}
			}
		})
	}
}

func TestToLevel1CampSaveWrittenRealData(t *testing.T) {
	o := loadWritableData(t)
	defer o.Close()
	if err := o.AllowFileWrites("EOBDATA.SAV", "LEVELS.TMP"); err != nil {
		t.Fatal(err)
	}
	if err := eob1.ToLevel1CampSaveWritten(o); err != nil {
		t.Fatal(err)
	}
	if got := digestRegion(o.Indexed(), 0, 0, 176, 96); got != "a2ca1d151d25aa8df20113ae78f130df05a1daed3c102173bbb02f557ee13d0c" {
		t.Fatalf("Save Game完成SHA-256=%s", got)
	}
	if got := o.PortReads()[0x60]; got != 102 {
		t.Fatalf("Save Game完成port 60h讀取%d次，預期102次", got)
	}
	wantWrites := []oracleWrite{{"EOBDATA.SAV", 1458}, {"EOBDATA.SAV", 64}, {"EOBDATA.SAV", 7000}, {"EOBDATA.SAV", 24480}, {"EOBDATA.SAV", 1}, {"EOBDATA.SAV", 1}, {"EOBDATA.SAV", 1}, {"EOBDATA.SAV", 96}}
	gotWrites := o.Wrote()
	if len(gotWrites) != len(wantWrites) && len(gotWrites) != len(wantWrites)+1 {
		t.Fatalf("Save Game寫入%d段，預期8段主檔加可選暫存檔：%v", len(gotWrites), gotWrites)
	}
	for i, want := range wantWrites {
		if gotWrites[i].Name != want.name || gotWrites[i].N != want.n {
			t.Fatalf("Save Game寫入第%d段=%v，預期%+v", i, gotWrites[i], want)
		}
	}
	if len(gotWrites) == len(wantWrites)+1 && (gotWrites[8].Name != "LEVELS.TMP" || gotWrites[8].N != 24480) {
		t.Fatalf("Save Game暫存檔寫入=%v，預期LEVELS.TMP 24480 bytes", gotWrites[8])
	}
}

// TestToLevel1CampSaveReloadRealData以同一個可丟棄資料副本完成「新隊伍→
// LEVEL1→CAMP存檔→重啟→標題LOAD」。這條鏈同時釘住存檔寫入與從
// EOBDATA6.PAK解析ITEML1.CPS，避免只驗各自孤立的helper。
func TestToLevel1CampSaveReloadRealData(t *testing.T) {
	o := loadWritableData(t)
	if err := o.AllowFileWrites("EOBDATA.SAV", "LEVELS.TMP"); err != nil {
		o.Close()
		t.Fatal(err)
	}
	if err := eob1.ToLevel1CampSaveWritten(o); err != nil {
		o.Close()
		t.Fatal(err)
	}
	o.Close()

	o = loadWritableData(t)
	defer o.Close()
	if err := eob1.ToTitleMenu(o); err != nil {
		t.Fatal(err)
	}
	o.PressKey(oracle.KeyEnter)
	// 標題畫面本身已經穩定，不能立即用ScreenIdle，否則會在輸入尚未
	// 由IRQ1消費前誤把標題畫面當成載入完成。
	if err := o.Run(5_000_000); err != nil {
		t.Fatalf("重啟後執行標題LOAD：%v", err)
	}
	// 存檔從Game Options完成後，原版重載會重畫含現行方向與地面物件的
	// 地城視窗；這不是新隊伍剛進LEVEL1時的過渡幀。
	const level1View = "13de273d44682a02ac7b72d4dc3baa8356d1b1394b5e05cbb7f88a0f17963546"
	var nextScreenCheck uint64
	loaded := oracle.NewCond("重載後LEVEL1入口", func(o *oracle.Oracle) bool {
		if o.Steps() < nextScreenCheck {
			return false
		}
		nextScreenCheck = o.Steps() + 10_000
		return digestRegion(o.Indexed(), 0, 0, 176, 120) == level1View
	})
	if err := o.RunUntil(loaded, oracle.Budget(40_000_000)); err != nil {
		t.Fatalf("重啟後載入新存檔未穩定：%v；缺檔=%v；缺檔存取=%v",
			err, o.Missing(), o.MissingAccesses())
	}
	if got := digestRegion(o.Indexed(), 0, 0, 176, 120); got != level1View {
		t.Fatalf("重啟後載入新存檔未回到LEVEL1入口：地城視窗SHA-256=%s；已開啟=%v；缺檔=%v；缺檔存取=%v",
			got, o.Opened(), o.Missing(), o.MissingAccesses())
	}
	opened := strings.ToUpper(strings.Join(o.Opened(), "\n"))
	if !strings.Contains(opened, "EOBDATA6.PAK") {
		t.Fatalf("存檔重載資源鏈不完整：%s", opened)
	}
}

type oracleWrite struct {
	name string
	n    int
}

func TestToLevel1CampMemorizeSelectedRealData(t *testing.T) {
	o := loadRealData(t)
	defer o.Close()
	if err := eob1.ToLevel1CampMemorizeSelected(o); err != nil {
		t.Fatal(err)
	}
	if got := digest(o.Indexed()); got != "8bd3e9866787810347ed38d1929b8dbb1a12683359037056f5fc369df3b2962c" {
		t.Fatalf("LEVEL1 CAMP第二列SHA-256=%s", got)
	}
	if got := o.PortReads()[0x60]; got != 86 {
		t.Fatalf("LEVEL1 CAMP第二列port 60h讀取%d次，預期四十三鍵make／break共86次", got)
	}
}

func TestToLevel1CampReturnRealData(t *testing.T) {
	o := loadRealData(t)
	defer o.Close()
	if err := eob1.ToLevel1CampReturn(o); err != nil {
		t.Fatal(err)
	}
	if got := digestRegion(o.Indexed(), 0, 0, 176, 120); got != "2ef2c0240070bce02b59735c5266fc6163eee170ea8c135982a469f04bb2abbc" {
		t.Fatalf("LEVEL1 CAMP返回視窗SHA-256=%s", got)
	}
	if got := o.PortReads()[0x60]; got != 88 {
		t.Fatalf("LEVEL1 CAMP返回port 60h讀取%d次，預期四十四鍵make／break共88次", got)
	}
}

func TestToLevel1MemorizeSpellsRealData(t *testing.T) {
	o := loadRealData(t)
	defer o.Close()
	if err := eob1.ToLevel1MemorizeSpells(o); err != nil {
		t.Fatal(err)
	}
	if got := digest(o.Indexed()); got != "1c430348d1f4d21309d1b553fb3f21597baa97c6682563c9d599ad9b9bf6c00a" {
		t.Fatalf("LEVEL1記憶法術頁SHA-256=%s", got)
	}
	if got := o.PortReads()[0x60]; got != 88 {
		t.Fatalf("LEVEL1記憶法術頁port 60h讀取%d次，預期四十四鍵make／break共88次", got)
	}
}

func TestToLevel1MemorizeFirstSpellRealData(t *testing.T) {
	o := loadRealData(t)
	defer o.Close()
	if err := eob1.ToLevel1MemorizeFirstSpell(o); err != nil {
		t.Fatal(err)
	}
	if got := digest(o.Indexed()); got != "2d78cd6c8a1886cb8d06b14a708d70ffc6fd9529422c0ade2eff36b2df386da8" {
		t.Fatalf("LEVEL1第一個待記憶法術SHA-256=%s", got)
	}
	if got := o.PortReads()[0x60]; got != 90 {
		t.Fatalf("LEVEL1第一個待記憶法術port 60h讀取%d次，預期四十五鍵make／break共90次", got)
	}
}

func TestToLevel1MemorizeReturnRealData(t *testing.T) {
	o := loadRealData(t)
	defer o.Close()
	if err := eob1.ToLevel1MemorizeReturn(o); err != nil {
		t.Fatal(err)
	}
	if got := digest(o.Indexed()); got != "8bd3e9866787810347ed38d1929b8dbb1a12683359037056f5fc369df3b2962c" {
		t.Fatalf("LEVEL1記憶法術返回CAMP SHA-256=%s", got)
	}
	if got := o.PortReads()[0x60]; got != 92 {
		t.Fatalf("LEVEL1記憶法術返回port 60h讀取%d次，預期四十六鍵make／break共92次", got)
	}
}

func TestToLevel1RestAfterMemorizeRealData(t *testing.T) {
	o := loadRealData(t)
	defer o.Close()
	if err := eob1.ToLevel1RestAfterMemorize(o); err != nil {
		t.Fatal(err)
	}
	if got := digest(o.Indexed()); got != "79e080c1a738e5ecd7d26ff379d7a126cb9ba4cee7ea998af94555b7a3d4f8ae" {
		t.Fatalf("LEVEL1正常休息完成SHA-256=%s", got)
	}
	if got := o.PortReads()[0x60]; got != 96 {
		t.Fatalf("LEVEL1正常休息完成port 60h讀取%d次，預期四十八鍵make／break共96次", got)
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

func loadWritableData(t *testing.T) *oracle.Oracle {
	t.Helper()
	exe, root := os.Getenv("EOB1_ORACLE_WRITABLE_EXE"), os.Getenv("EOB1_ORACLE_WRITABLE_ROOT")
	if exe == "" || root == "" {
		t.Skip("未設定EOB1_ORACLE_WRITABLE_EXE／ROOT；可寫覆蓋層測試明確略過")
	}
	o, err := oracle.Load(exe, root)
	if err != nil {
		t.Fatal(err)
	}
	return o
}

func loadInvalidSaveData(t *testing.T) *oracle.Oracle {
	t.Helper()
	exe, root := os.Getenv("EOB1_ORACLE_INVALID_SAVE_EXE"), os.Getenv("EOB1_ORACLE_INVALID_SAVE_ROOT")
	if exe == "" || root == "" {
		t.Skip("未設定EOB1_ORACLE_INVALID_SAVE_EXE／ROOT；無效存檔測試明確略過")
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
