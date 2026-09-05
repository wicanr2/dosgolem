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
