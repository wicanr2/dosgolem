package strikecommander

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"
)

// Run with GOMAXPROCS=1, --cpus 1, -benchtime=20x -count=5. Each measured
// invocation renders groups 001..020 once; loading, hashing and output are
// outside the timer. The benchmark framework's N=1 calibration is unreported.
func BenchmarkModelSceneOriginal(b *testing.B) {
	f, lib, tables := modelSceneSetup(b)
	if runtime.GOMAXPROCS(0) != 1 {
		b.Fatal("acceptance benchmark requires GOMAXPROCS=1")
	}
	out := os.Getenv("SC_MODEL_BENCH_OUT")
	if out == "" {
		b.Skip("set SC_MODEL_BENCH_OUT to local bench-* directory")
	}
	if len(filepath.Base(out)) < 6 || filepath.Base(out)[:6] != "bench-" {
		b.Fatal("benchmark receipt must have bench- prefix")
	}
	if e := os.MkdirAll(out, 0755); e != nil {
		b.Fatal(e)
	}
	for _, c := range []struct {
		name   string
		report int
	}{{"s760", 14}, {"s800f2", 15}} {
		groups := make([]ModelGroup, 20)
		for i := range groups {
			groups[i] = modelSceneLoad(b, f, c.report, i+1)
		}
		for _, n := range []int{2, 3} {
			b.Run(fmt.Sprintf("%s/N%d", c.name, n), func(b *testing.B) {
				times := make([]int64, b.N)
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					start := time.Now()
					r, e := groups[i%20].RenderModelDiagnostics(lib, tables, n)
					times[i] = time.Since(start).Nanoseconds()
					if e != nil || !r.Passed {
						b.Fatalf("render failed: %v %+v", e, r.Diagnostics)
					}
				}
				b.StopTimer()
				if b.N != 20 {
					return
				}
				sorted := append([]int64(nil), times...)
				sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
				b.ReportMetric(float64(sorted[9])/1e6, "p50-ms")
				b.ReportMetric(float64(sorted[18])/1e6, "p95-ms")
				b.ReportMetric(float64(sorted[19])/1e6, "max-ms")
				for run := 1; ; run++ {
					name := filepath.Join(out, fmt.Sprintf("%s-n%d-round-%02d.json", c.name, n, run))
					if _, e := os.Stat(name); e == nil {
						continue
					}
					modelSceneWrite(b, name, modelSceneJSON(b, map[string]any{"Case": c.name, "N": n, "Groups": "001..020", "RenderNanoseconds": times, "P50Nanoseconds": sorted[9], "P95Nanoseconds": sorted[18], "MaxNanoseconds": sorted[19], "Go": runtime.Version(), "GOARCH": runtime.GOARCH, "GOMAXPROCS": runtime.GOMAXPROCS(0), "InputsSHA256": f.SHA256, "Scope": "Render only, no I/O; exactly 20 fixed group renders per measured round; no product budget gate"}))
					break
				}
			})
		}
	}
}
