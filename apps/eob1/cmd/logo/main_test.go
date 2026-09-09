package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunRealDataWritesBoundedReceipt(t *testing.T) {
	exe, root := os.Getenv("EOB1_ORACLE_EXE"), os.Getenv("EOB1_ORACLE_ROOT")
	if exe == "" || root == "" {
		t.Skip("未設定EOB1_ORACLE_EXE／EOB1_ORACLE_ROOT；私有原版資料測試明確略過")
	}
	out := t.TempDir()
	if err := run(exe, root, out); err != nil {
		t.Fatal(err)
	}
	for name, size := range map[string]int64{"indexed.bin": 64000, "palette.rgb": 768} {
		st, err := os.Stat(filepath.Join(out, name))
		if err != nil {
			t.Fatalf("%s不存在：%v", name, err)
		}
		if st.Size() != size {
			t.Fatalf("%s大小=%d，預期%d", name, st.Size(), size)
		}
	}
	data, err := os.ReadFile(filepath.Join(out, "receipt.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), root) || strings.Contains(string(data), exe) {
		t.Fatal("收據洩漏私有原版路徑")
	}
	var got receipt
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Executable != "START1.EXE" || got.Steps == 0 || len(got.Opened) == 0 {
		t.Fatalf("收據缺少必要欄位：%+v", got)
	}
}
