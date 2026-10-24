package strikecommander

// 選配重算已凍結的原版觀察。用於公式／辨識突變；不取代錄製器時點驗收。
import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/apps/strikecommander/realspace"
)

func TestModelRecordedCheck(t *testing.T) {
	root, records := os.Getenv("SC_MODEL_ROOT"), os.Getenv("SC_MODEL_RECORDS")
	if root == "" || records == "" {
		t.Skip("requires frozen original model receipts")
	}
	lib, err := realspace.LoadLibrary(root)
	if err != nil {
		t.Fatal(err)
	}
	selected := "," + os.Getenv("SC_MODEL_RECORD_CASES") + ","
	for _, tc := range []struct{ name, report string }{{"s760", "002"}, {"s800f2", "003"}, {"n1", "004"}, {"n2", "005"}} {
		if selected != ",," && !strings.Contains(selected, ","+tc.name+",") {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			var count, passed, appr, apprFailed, xyzMismatch, screenMismatch uint64
			for i := 1; i <= 30; i++ {
				p := filepath.Join(records, "report-"+tc.report, "B", fmt.Sprintf("group-%03d.json", i))
				b, err := os.ReadFile(p)
				if err != nil {
					t.Fatal(err)
				}
				t.Logf("input group=%d sha256=%x", i, sha256.Sum256(b))
				var g ModelGroup
				if err = json.Unmarshal(b, &g); err != nil {
					t.Fatal(err)
				}
				for oi := range g.Objects {
					o := &g.Objects[oi]
					v := realspace.CheckObject(lib, g.Cameras, o)
					count++
					if v.Status == realspace.Passed {
						passed++
					}
					if o.RecVT == 0x314a {
						appr++
						if v.Status != realspace.Passed {
							apprFailed++
						}
					}
					xyzMismatch += v.XYZMismatch
					screenMismatch += v.ScreenMismatch
					if v.Status != realspace.Passed {
						t.Errorf("group=%d obj=%d vt=%04x status=%s reason=%s xyz_mismatch=%d screen_mismatch=%d first=%+v", i, oi, o.RecVT, v.Status, v.Reason, v.XYZMismatch, v.ScreenMismatch, v.First)
					}
				}
			}
			t.Logf("case=%s groups=30 objects=%d passed=%d appr=%d appr_failed=%d xyz_mismatch=%d screen_mismatch=%d", tc.name, count, passed, appr, apprFailed, xyzMismatch, screenMismatch)
		})
	}
}
