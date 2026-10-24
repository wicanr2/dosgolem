package geometry

import "testing"

func TestSamplesCompactOwnedAndFallback(t *testing.T) {
	tri := sampleTestTriangle([3][2]float64{{0, 0}, {20, 0}, {0, 20}}, [3]int32{3, 5, 7}, 3)
	h, ok, err := EvaluateSampleFast(tri, 1, 1, 0, 0)
	if err != nil || !ok || !h.Coverage {
		t.Fatal("compact sample", h, ok, err)
	}
	first := h
	h.Area = 1
	h.Edges[0] = 2
	again, _, _ := EvaluateSampleFast(tri, 1, 1, 0, 0)
	if again != first {
		t.Fatal("compact sample not owned")
	}
	if allocations := testing.AllocsPerRun(100, func() {
		if _, ok, err := EvaluateSampleFast(tri, 1, 1, 0, 0); err != nil || !ok {
			panic("unexpected compact error")
		}
	}); allocations != 0 {
		t.Fatal("compact path allocates", allocations)
	}
	bigTri := sampleTestTriangle([3][2]float64{{-1e12, -1e12}, {1e12, -1e12}, {0, 1e12}}, [3]int32{3, 5, 7}, 3)
	if _, ok, err := EvaluateSampleFast(bigTri, 1, 1, 0, 0); err != nil || ok {
		t.Fatal("BigInt triangle was narrowed", ok, err)
	}
	for _, args := range [][4]int{{-1, 0, 0, 0}, {960, 0, 0, 0}, {0, 600, 0, 0}, {0, 0, 2, 0}} {
		if _, _, err := EvaluateSampleFast(tri, args[0], args[1], args[2], args[3]); err == nil {
			t.Fatal("compact invalid input")
		}
	}
}

func BenchmarkSamplesCompact(b *testing.B) {
	tri := sampleTestTriangle([3][2]float64{{0, 0}, {20, 0}, {0, 20}}, [3]int32{3, 5, 7}, 3)
	b.ReportAllocs()
	b.ResetTimer()
	for k := 0; k < b.N; k++ {
		if _, ok, err := EvaluateSampleFast(tri, 1, 1, k&1, (k>>1)&1); err != nil || !ok {
			b.Fatal(err)
		}
	}
}
