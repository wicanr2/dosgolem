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
	start     int       // 預填門檻（float32 個數）：未播放時累積到這麼多才開始
	playing   bool
	fadeIn    int // 恢復播放後還要淡入的框數
	underruns uint64
	dropped   uint64
}

// fadeFrames 是斷流淡出、恢復淡入的長度（2 毫秒）。
const fadeFrames = SampleRate / 500

// NewRing 建一個上限為 maxMillis 毫秒的環形緩衝。
// 播放前先預填 maxMillis 的一半；斷流時淡出後回到預填狀態，避免波形被硬切成 0 的爆音。
func NewRing(maxMillis int) *Ring {
	return &Ring{max: SampleRate * maxMillis / 1000 * 2, start: SampleRate * maxMillis / 2000 * 2}
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
// 長度一律回 len(p) 對齊 8 bytes 的部分。未達預填門檻時回靜音；樣本不足時把手上的
// 樣本淡出、記一次不足並回到預填狀態；恢復時淡入。
func (r *Ring) Read(p []byte) (int, error) {
	n := len(p) / 8 * 8
	want := n / 4
	out := make([]float32, want)
	r.mu.Lock()
	if !r.playing && len(r.buf) >= r.start {
		r.playing, r.fadeIn = true, fadeFrames
	}
	if r.playing {
		have := len(r.buf)
		if have > want {
			have = want
		}
		copy(out, r.buf[:have])
		r.buf = append(r.buf[:0], r.buf[have:]...)
		for f := 0; f < have/2 && r.fadeIn > 0; f++ {
			g := float32(fadeFrames-r.fadeIn) / fadeFrames
			out[2*f] *= g
			out[2*f+1] *= g
			r.fadeIn--
		}
		if have < want {
			r.underruns++
			r.playing = false
			frames := have / 2
			k := fadeFrames
			if k > frames {
				k = frames
			}
			for i := 0; i < k; i++ {
				g := float32(k-i) / float32(k+1)
				f := frames - k + i
				out[2*f] *= g
				out[2*f+1] *= g
			}
		}
	}
	r.mu.Unlock()
	for i, v := range out {
		binary.LittleEndian.PutUint32(p[4*i:], math.Float32bits(v))
	}
	return n, nil
}
