package r5asset

import (
	"encoding/binary"
	"math"
)

func vertexAt(b []byte, m meshData, p int, i uint64) (Vertex, error) {
	if p < 0 || p >= m.Count || p >= len(m.Prim) || i >= m.Prim[p].Vertices {
		return Vertex{}, reject(Structure, 0, 120)
	}
	pr := m.Prim[p]
	var v Vertex
	for role, n := range [4]int{3, 3, 4, 2} {
		for k := 0; k < n; k++ {
			off := pr.Offset[role] + (i*uint64(n)+uint64(k))*4
			if off > uint64(len(b)) || uint64(len(b))-off < 4 {
				return Vertex{}, reject(Structure, 0, 121)
			}
			x := math.Float32frombits(binary.LittleEndian.Uint32(b[int(off) : int(off)+4]))
			switch role {
			case 0:
				v.Position[k] = x
			case 1:
				v.Normal[k] = x
			case 2:
				v.Tangent[k] = x
			case 3:
				v.UV[k] = x
			}
		}
	}
	return v, nil
}
func indexAt(b []byte, m meshData, p int, i uint64) (uint32, error) {
	if p < 0 || p >= m.Count || p >= len(m.Prim) || i >= m.Prim[p].Indices {
		return 0, reject(Structure, 0, 122)
	}
	off := m.Prim[p].Offset[4] + i*4
	if off > uint64(len(b)) || uint64(len(b))-off < 4 {
		return 0, reject(Structure, 0, 123)
	}
	return binary.LittleEndian.Uint32(b[int(off) : int(off)+4]), nil
}
