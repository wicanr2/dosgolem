package mixer

import (
	"encoding/binary"
	"math"
	"sync"
)

// Ring 是 Update goroutine 與播放 goroutine 之間唯一共用的物件
// （`docs/spec/240` §3.5）：Push 由合成端呼叫，Read 由播放端呼叫。
// 超過上限時丟最舊的樣本；不足時補 0。
type Ring struct {
	mu        sync.Mutex
	buf       []float32 // 交錯立體聲
	max       int       // 以 float32 個數計
	underruns uint64
	dropped   uint64
}

// NewRing 建一個上限為 maxMillis 毫秒的環形緩衝。
func NewRing(maxMillis int) *Ring {
	return &Ring{max: SampleRate * maxMillis / 1000 * 2}
}

// Push 附加樣本；超過上限丟最舊的。
func (r *Ring) Push(s []float32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf = append(r.buf, s...)
	if over := len(r.buf) - r.max; over > 0 {
		over += over & 1 // 對齊立體聲框
		r.dropped += uint64(over / 2)
		r.buf = append(r.buf[:0], r.buf[over:]...)
	}
}

// Buffered 回目前緩衝的立體聲框數。
func (r *Ring) Buffered() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.buf) / 2
}

// Stats 回緩衝不足次數與丟棄的框數。
func (r *Ring) Stats() (underruns, dropped uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.underruns, r.dropped
}

// Read 以 float32 little-endian 交錯立體聲填滿 p（ebiten NewPlayerF32 的格式），
// 長度一律回 len(p) 對齊 8 bytes 的部分；樣本不足時補 0 並記一次不足。
func (r *Ring) Read(p []byte) (int, error) {
	n := len(p) / 8 * 8
	want := n / 4
	r.mu.Lock()
	have := len(r.buf)
	if have < want {
		if have < want && n > 0 {
			r.underruns++
		}
	} else {
		have = want
	}
	for i := 0; i < have; i++ {
		binary.LittleEndian.PutUint32(p[4*i:], math.Float32bits(r.buf[i]))
	}
	r.buf = append(r.buf[:0], r.buf[have:]...)
	r.mu.Unlock()
	for i := have; i < want; i++ {
		binary.LittleEndian.PutUint32(p[4*i:], 0)
	}
	return n, nil
}
