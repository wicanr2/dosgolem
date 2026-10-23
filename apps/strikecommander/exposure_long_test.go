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
//
// 規格 014 分期 (c)：DOSGOLEM_SC_PRESENT 為 on 時觀察器開的那次執行另掛呈現層（ReceiptConfig.Present），為 off 時不掛；
// 兩者都在每個 shot 與停止點取 MachineDigest，寫到 <DOSGOLEM_SC_OUT>/present-<案例>（§4.4 的同狀態 A/B）。
// DOSGOLEM_SC_PRESENT_MUT 是呈現層的負對照（§4.5，逗號分隔：n2、n6、n9、n22、n22b、n24、n27、n30a、n30b、n33、n34、n35）。
func runReceiptExpo(t *testing.T, name string, rc ReceiptConfig) (*ReceiptResult, error) {
	t.Helper()
	pmode := os.Getenv("DOSGOLEM_SC_PRESENT")
	pon := name != "" && rc.Overlay && pmode != ""
	if pon {
		rc.machineDigests = true
		if pmode == "on" {
			rc.Present = true
			m, err := parsePresentMut(os.Getenv("DOSGOLEM_SC_PRESENT_MUT"))
			if err != nil {
				t.Fatal(err)
			}
			rc.presentMut = m
		} else if pmode != "off" {
			t.Fatalf("DOSGOLEM_SC_PRESENT %q 不支援（on、off）", pmode)
		}
	}
	res, err := runReceiptExpoInner(t, name, rc)
	if err != nil || !pon {
		return res, err
	}
	if out := os.Getenv("DOSGOLEM_SC_OUT"); out != "" {
		if err := writePresentFiles(filepath.Join(out, "present-"+strings.ToLower(name)), res); err != nil {
			t.Fatal(err)
		}
	}
	if p := res.Present; p != nil {
		for _, ln := range strings.Split(strings.TrimSpace(string(p.CountsText())), "\n") {
			t.Logf("呈現 %s：%s", name, ln)
		}
	}
	return res, nil
}

// writePresentFiles 寫 digest.txt（停止點 Digest 與 MachineDigest）、shot-digests.tsv；掛了呈現層時另寫
// shots.tsv（§3.9 的差異斷言）、停止點呈現合成 PNG 與呈現層的各檔。
func writePresentFiles(dir string, res *ReceiptResult) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	w := func(name string, b []byte) error { return os.WriteFile(filepath.Join(dir, name), b, 0o644) }
	d := DigestText(res.Digest)
	d = append(d, []byte(fmt.Sprintf("MachineSHA256\t%x\nDOSSHA256\t%x\n", res.MachineDigest.Machine, res.MachineDigest.DOS))...)
	if err := w("digest.txt", d); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("i\tsteps\tmemory\tcpu\tindexed\tpalette\tmachine\tdos\n")
	for i, sd := range res.ShotDigests {
		var md [2][32]byte
		if i < len(res.ShotMachineDigests) {
			md = [2][32]byte{res.ShotMachineDigests[i].Machine, res.ShotMachineDigests[i].DOS}
		}
		fmt.Fprintf(&b, "%d\t%d\t%x\t%x\t%x\t%x\t%x\t%x\n", i, sd.Steps, sd.MemorySHA256, sd.CPUSHA256, sd.IndexedSHA256, sd.PaletteSHA256, md[0], md[1])
	}
	if err := w("shot-digests.tsv", []byte(b.String())); err != nil {
		return err
	}
	p := res.Present
	if p == nil {
		return nil
	}
	b.Reset()
	b.WriteString("shot\tsteps\tseq\tkind\tcapture_steps\tcond\tequal_2x\tequal_3x\twhy\n")
	row := func(name string, steps uint64, ps *PresentShot) {
		if ps == nil {
			return
		}
		fmt.Fprintf(&b, "%s\t%d\t%d\t%s\t%d\t%t\t%t\t%t\t%s\n", name, steps, ps.Info.Seq, ps.Info.Kind, ps.Info.CaptureSteps, ps.Cond, ps.Equal[2], ps.Equal[3], strings.Join(ps.Why, "；"))
	}
	for _, s := range res.Shots {
		row(s.Name, s.Steps, s.Presented)
	}
	row("stop", res.Digest.Steps, res.Presented)
	if err := w("shots.tsv", []byte(b.String())); err != nil {
		return err
	}
	if res.Presented != nil {
		for sc, c := range res.Presented.Composites {
			if err := WriteRGBAPNG(filepath.Join(dir, fmt.Sprintf("stop-presented-%dx.png", sc)), c, sc); err != nil {
				return err
			}
		}
	}
	return p.WriteFiles(dir, w)
}

func runReceiptExpoInner(t *testing.T, name string, rc ReceiptConfig) (*ReceiptResult, error) {
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
		if err := WriteExpoFiles(dir, x, res.Present); err != nil {
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
