package koukai2

import (
	"encoding/hex"
	"os"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"
)

type bootRun struct {
	cps       []Checkpoint
	rng       *RNGTrace
	writes    []SeedWrite
	titleDiff int
	clock     string
}

func runBoot(t *testing.T, root string) bootRun {
	t.Helper()
	o, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	o.SetScratch(t.TempDir())
	if err := ToMain(o, true); err != nil {
		t.Fatal(err)
	}
	b, ok := Current(o)
	if !ok {
		t.Fatal("MAIN.EXE 不在 EXEC 紀錄")
	}
	var r bootRun
	r.rng = WatchRNG(o, b)
	WatchSeedWriters(o, b, &r.writes)
	o.OnCall(b.Code(AddrOneDay), func(o *oracle.Oracle) {
		if r.clock == "" {
			r.clock = hex.EncodeToString(o.Bytes(b.DS(DSClock), 4))
		}
	})
	img, err := os.ReadFile(root + "/MAIN.EXE")
	if err != nil {
		t.Fatal(err)
	}
	err = ToMainLoop(o, func(step string) {
		r.cps = append(r.cps, Capture(o, step))
		if step == "title" {
			mem := o.Bytes(b.DS(DSStateLo), int(DSStateHi-DSStateLo))
			for i := range mem {
				if mem[i] != img[0x3D350+int(DSStateLo)+i] {
					r.titleDiff++
				}
			}
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// `docs/spec/443-koukai2-oracle-adapter` §5。需要原版，缺檔就 skip。
func TestBootDeterministic(t *testing.T) {
	root := os.Getenv("DOSGOLEM_KOUKAI2_ROOT")
	if root == "" {
		t.Skip("沒有設 DOSGOLEM_KOUKAI2_ROOT")
	}
	if _, err := os.Stat(root + "/KOUKAI2.COM"); err != nil {
		t.Skipf("缺原版：%v", err)
	}
	a, b := runBoot(t, root), runBoot(t, root)
	if len(a.cps) != len(b.cps) {
		t.Fatalf("比對點數 %d != %d", len(a.cps), len(b.cps))
	}
	for i := range a.cps {
		if a.cps[i] != b.cps[i] {
			t.Errorf("比對點 %s 不同：\n%+v\n%+v", a.cps[i].Name, a.cps[i], b.cps[i])
		}
	}
	if a.rng.Calls != b.rng.Calls {
		t.Errorf("亂數次數 %d != %d", a.rng.Calls, b.rng.Calls)
	}
	if a.rng.First != 0 {
		t.Errorf("第一個種子 %08X，應為 0", a.rng.First)
	}
	if a.rng.Mismatch != 0 {
		t.Errorf("LCG 不符 %d：%s", a.rng.Mismatch, a.rng.FirstBad)
	}
	if len(a.writes) != 0 {
		t.Errorf("種子有其他寫入端：%+v", a.writes[0])
	}
	if a.titleDiff != 0 {
		t.Errorf("標題畫面 DS:0CA4..8A18 與檔案差 %d bytes", a.titleDiff)
	}
	if a.clock != "15041018" {
		t.Errorf("第一天時鐘 %s，應為 15041018", a.clock)
	}
}
