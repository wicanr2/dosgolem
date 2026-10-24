package machine

import "testing"

// 規格 461 §5 第 6 條：給外部測試套件（machine_test，可 import internal/dos 跑實模式開機路徑）用的重播與報告。

// PIT461Report 見 pit461Report。
func PIT461Report(t *testing.T, name string, ws []PortWrite) (writes, diffs int, finalDiv uint32) {
	t.Helper()
	r := pit461Replay(ws)
	pit461Report(t, name, r)
	return r.Writes, len(r.Diffs), r.FinalDiv
}
