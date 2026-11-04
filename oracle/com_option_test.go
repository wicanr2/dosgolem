package oracle

import (
	"os"
	"path/filepath"
	"testing"
)

// `docs/spec/443-koukai2-oracle-adapter` §2：零值不猜格式，COM 要明講。
func TestLoadWithCOMOption(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "T.COM")
	// mov ax,4C07h / int 21h
	if err := os.WriteFile(exe, []byte{0xB8, 0x07, 0x4C, 0xCD, 0x21}, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadWith(exe, dir, Options{}); err == nil {
		t.Fatal("零值應該拒絕非 MZ 檔案")
	}
	o, err := LoadWith(exe, dir, Options{COM: true})
	if err != nil {
		t.Fatalf("COM=true 應該可以載入：%v", err)
	}
	defer o.Close()
	err = o.Run(1000)
	ee, ok := err.(*ExitError)
	if !ok {
		t.Fatalf("應該以 AH=4Ch 結束，得到 %v", err)
	}
	if ee.Code != 7 {
		t.Fatalf("結束碼 %d，應為 7", ee.Code)
	}
}
