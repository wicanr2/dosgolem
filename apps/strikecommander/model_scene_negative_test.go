package strikecommander

import (
	"os"
	"strings"
	"testing"
)

// K16 隔離 B 的狀態總量閘門。C 刻意不提供，避免其他失敗掩蓋比例。
func TestModelSceneBPerStateGate(t *testing.T) {
	if os.Getenv("SC_MODEL_B_GATE") != "1" {
		t.Skip("optional original B-state gate")
	}
	f, lib, tables := modelSceneSetup(t)
	selected := "," + os.Getenv("SC_MODEL_B_CASES") + ","
	for _, tc := range []struct {
		name   string
		report int
	}{{"s760", 14}, {"s800f2", 15}, {"n1", 16}, {"n2", 17}} {
		if selected != ",," && !strings.Contains(selected, ","+tc.name+",") {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			var eligible, zeroX, zeroY, hardX, hardY uint64
			for group := 1; group <= 30; group++ {
				g := modelSceneLoad(t, f, tc.report, group)
				g.Raster = nil // B-only；仍使用原始相機、矩陣、投影及原檔庫。
				r, err := g.RenderModelDiagnostics(lib, tables, 2)
				if err != nil {
					t.Fatal(err)
				}
				eligible += r.B.Eligible
				zeroX += r.B.ZeroX
				zeroY += r.B.ZeroY
				hardX += r.B.HardFailureX
				hardY += r.B.HardFailureY
			}
			t.Logf("state=%s groups=30 eligible=%d zero_x=%d zero_y=%d hard_x=%d hard_y=%d", tc.name, eligible, zeroX, zeroY, hardX, hardY)
			if eligible == 0 || zeroX*100 < eligible*99 || zeroY*100 < eligible*99 {
				t.Error("state aggregate B ratio below 99%")
			}
			if hardX != 0 || hardY != 0 {
				t.Error("B hard error set failed")
			}
		})
	}
}
