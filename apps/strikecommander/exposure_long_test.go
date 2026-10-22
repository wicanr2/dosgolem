package strikecommander

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// strike_commander 規格 014 §4.2 的英文曝光收據（分期 (b)：基線）掛在既有長測試上：
//   - DOSGOLEM_SC_EXPO 非空時，各案例觀察器開的那次執行另做曝光收據，寫到 <DOSGOLEM_SC_OUT>/expo-<案例>；
//     既有收據照常寫出，與舊收據逐位元組比對即 §4.4 的 A′（合成點只切分推進）；
//   - DOSGOLEM_SC_EXPO_MUT 是負對照（§4.5）：n1（合成點改為每個狀態改變事件之後）、n8（n1 加上台帳只看可見集合）、
//     n29（台帳改用觀察用實例取 stamp 像素）。
//
// 未設定時 runReceiptExpo 等於 RunReceipt。

func expoMutFromEnv(t *testing.T) expoMut {
	t.Helper()
	switch s := os.Getenv("DOSGOLEM_SC_EXPO_MUT"); s {
	case "":
		return 0
	case "n1":
		return expoMutN1
	case "n8":
		return expoMutN1 | expoMutLedgerFromVisible
	case "n29":
		return expoMutObsStamps
	default:
		t.Fatalf("DOSGOLEM_SC_EXPO_MUT %q 不支援", s)
	}
	return 0
}

// runReceiptExpo 是長測試的 RunReceipt；name 為空或觀察器關時不做曝光收據。
func runReceiptExpo(t *testing.T, name string, rc ReceiptConfig) (*ReceiptResult, error) {
	t.Helper()
	on := name != "" && rc.Overlay && os.Getenv("DOSGOLEM_SC_EXPO") != ""
	if on {
		rc.expo = &ExpoConfig{mut: expoMutFromEnv(t)}
		// DOSGOLEM_SC_EXPO_DUMP：逗號分隔的步數，只供審查（寫 3 倍合成圖、台帳合成圖、原版放大圖）。
		if s := os.Getenv("DOSGOLEM_SC_EXPO_DUMP"); s != "" && os.Getenv("DOSGOLEM_SC_OUT") != "" {
			for _, f := range strings.Split(s, ",") {
				var v uint64
				if _, err := fmt.Sscan(f, &v); err != nil {
					t.Fatalf("DOSGOLEM_SC_EXPO_DUMP %q：%v", s, err)
				}
				rc.expo.DumpAt = append(rc.expo.DumpAt, v)
			}
			rc.expo.DumpDir = filepath.Join(os.Getenv("DOSGOLEM_SC_OUT"), "expo-"+strings.ToLower(name)+"-dump")
		}
	}
	res, err := RunReceipt(rc)
	if err != nil || !on {
		return res, err
	}
	x := res.Expo
	out := os.Getenv("DOSGOLEM_SC_OUT")
	if out != "" {
		dir := filepath.Join(out, "expo-"+strings.ToLower(name))
		if err := WriteExpoFiles(dir, x); err != nil {
			t.Fatal(err)
		}
		t.Logf("曝光收據寫到 %s", dir)
	}
	for _, ln := range strings.Split(strings.TrimSpace(string(x.CountsText())), "\n") {
		t.Logf("曝光 %s：%s", name, ln)
	}
	// §3.6.4 第 3 項：有底板入口而台帳讀取器失敗即驗收失敗。
	if x.PlateEvents > 0 && x.ReaderErr != "" {
		t.Errorf("曝光 %s：主選單底板 %d 次，台帳讀取器失敗：%s", name, x.PlateEvents, x.ReaderErr)
	}
	return res, nil
}
