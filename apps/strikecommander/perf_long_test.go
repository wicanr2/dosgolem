package strikecommander

// 觀察器路徑的速度基準（perf-cpu 分支第二段）：飛行案例 Q 截短到 DOSGOLEM_PERF_STOP（絕對步數），
// 以 runFlightCase 跑一種 run（DOSGOLEM_PERF_RUN：off、noflight、on23、on3），記錄牆鐘時間與停止 Digest。
// on23 且設 DOSGOLEM_SC_OUT 時照 TestFlightCase 寫收據（flight-q-on23/），供跨版本逐位元組比對。
// 沒有設 DOSGOLEM_PERF_STOP 時 skip。

import (
	"os"
	"testing"
	"time"
)

func TestSCPerfFlight(t *testing.T) {
	stop := os.Getenv("DOSGOLEM_PERF_STOP")
	if stop == "" {
		t.Skip("DOSGOLEM_PERF_STOP 未設")
	}
	flightEnv(t)
	run := os.Getenv("DOSGOLEM_PERF_RUN")
	if run == "" {
		run = "on23"
	}
	m, ok := flightRunModes[run]
	if !ok {
		t.Fatalf("DOSGOLEM_PERF_RUN %q 不認得", run)
	}
	t.Setenv("DOSGOLEM_SC_FLIGHT_STOP", stop)
	start := time.Now()
	res, sums := runFlightCase(t, "Q", m)
	el := time.Since(start)
	d := res.Digest
	t.Logf("PERF run=%s stop=%d shots=%d elapsed=%.3fs", run, d.Steps, len(sums), el.Seconds())
	t.Logf("HASH mem=%x cpu=%x idx=%x pal=%x ext=%s", d.MemorySHA256, d.CPUSHA256, d.IndexedSHA256, d.PaletteSHA256, d.ExtSHA256)
	for _, s := range sums {
		t.Logf("SHOT %s %d idx=%s comp2=%s comp3=%s", s.name, s.steps, s.idxSHA, s.comp[2], s.comp[3])
	}
}
