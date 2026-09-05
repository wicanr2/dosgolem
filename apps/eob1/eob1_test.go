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
	exe, root := os.Getenv("EOB1_ORACLE_EXE"), os.Getenv("EOB1_ORACLE_ROOT")
	if exe == "" || root == "" {
		t.Skip("未設定EOB1_ORACLE_EXE／EOB1_ORACLE_ROOT；私有原版資料測試明確略過")
	}
	o, err := oracle.Load(exe, root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	if err := eob1.ToWestwoodLogo(o); err != nil {
		t.Fatal(err)
	}
	if got := digest(o.Indexed()); got != "b4c544b0dce39640934e0edb9147f79e9c8d34b6dcfc18e2e942abe11a14d0e4" {
		t.Fatalf("Westwood標誌色號SHA-256=%s", got)
	}
	pal := o.Palette()
	flat := make([]byte, 0, 768)
	for _, rgb := range pal {
		flat = append(flat, rgb[:]...)
	}
	if got := digest(flat); got != "b075ca30a7550c75464d347e4a3a562201d29f09c57faa7153c67bc3d393f6fd" {
		t.Fatalf("Westwood標誌palette SHA-256=%s", got)
	}
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
