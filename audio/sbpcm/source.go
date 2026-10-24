// Package sbpcm 將 462 的 SB16 PCM 傳輸排成決定性的 48 kHz 單聲道樣本。
// 這是依機器 Start／Due 放置的硬體規格近似，不宣稱 DAC 逐週期一致。
package sbpcm

import (
	"github.com/wicanr2/dosgolem/internal/machine"
	"math/bits"
)

const SampleRate = 48000

type segment struct{ a, b, start uint64 }
type voice struct {
	data         []byte
	n, delta     uint64
	segments     []segment
	paused, done bool
}

// Stats 是累計診斷與目前保留的聲部數。Reset 不清累計診斷。
type Stats struct {
	Voices                                                          int
	ADPCMSkipped, CycleClockSkipped, CompleteMismatch, OrphanEvents uint64
}

// Source 只能在同一個 goroutine 使用。事件必須在對應回合 Render 前 Apply。
type Source struct {
	sps     uint64
	voices  []*voice
	current *voice
	stats   Stats
}

func New(sps uint64) *Source                  { return &Source{sps: sps} }
func (s *Source) sampleOf(step uint64) uint64 { return step * SampleRate / s.sps }

func (s *Source) finish(v *voice) {
	v.done = true
	for _, p := range v.segments {
		if s.sampleOf(p.a) < s.sampleOf(p.b) {
			return
		}
	}
	v.data = nil
}

// Apply 到達即記下傳輸與區段。每段保留當時的 Start，避免同回合的
// resume 平移影響前一段尚未 Render 的樣本。
func (s *Source) Apply(e machine.SBPCMEvent) {
	if e.Clock == "cycles" {
		s.stats.CycleClockSkipped++
		return
	}
	v := s.current
	if e.Kind == "start" {
		if v != nil {
			if !v.paused {
				v.segments[len(v.segments)-1].b = e.Step
			}
			s.finish(v)
			s.current = nil
		}
		if e.Command == 0x80 {
			return
		}
		if e.Command != 0x14 {
			s.stats.ADPCMSkipped++
			return
		}
		if e.Length == 0 || uint64(len(e.Data)) < uint64(e.Length) || e.Due <= e.Start {
			return
		}
		v = &voice{data: e.Data, n: uint64(e.Length), delta: e.Due - e.Start,
			segments: []segment{{e.Start, e.Due, e.Start}}}
		s.voices = append(s.voices, v)
		s.current = v
		return
	}
	if v == nil {
		s.stats.OrphanEvents++
		return
	}
	last := &v.segments[len(v.segments)-1]
	switch e.Kind {
	case "pause":
		last.b, v.paused = e.Step, true
	case "resume":
		v.segments = append(v.segments, segment{e.Step, e.Due, e.Start})
		v.paused = false
	case "cancel":
		if !v.paused {
			last.b = e.Step
		}
		s.finish(v)
		s.current = nil
	case "complete":
		if e.Step != last.b {
			s.stats.CompleteMismatch++
		}
		s.finish(v)
		s.current = nil
	}
}

// Sample 是純函式，回傳有號 16 位元刻度，不推進時間。
func (s *Source) Sample(j uint64) int32 {
	for _, v := range s.voices {
		if len(v.data) == 0 {
			continue
		}
		for _, p := range v.segments {
			if j < s.sampleOf(p.a) || j >= s.sampleOf(p.b) {
				continue
			}
			var d uint64
			if at, start := j*s.sps, p.start*SampleRate; at >= start {
				d = at - start
			}
			den := uint64(SampleRate) * v.delta
			hi, lo := bits.Mul64(d, v.n)
			k, r := bits.Div64(hi, lo, den)
			hi, lo = bits.Mul64(r, 65536)
			f, _ := bits.Div64(hi, lo, den)
			if k >= v.n {
				k = v.n - 1
			}
			k1 := k + 1
			if k1 >= v.n {
				k1 = v.n - 1
			}
			x0, x1 := int64(v.data[k])-128, int64(v.data[k1])-128
			return int32((x0*(65536-int64(f)) + x1*int64(f)) * 256 >> 16)
		}
	}
	return 0
}

// Prune 丟掉在 j 之前已結束的聲部與樣本切片。
func (s *Source) Prune(j uint64) {
	kept := s.voices[:0]
	for _, v := range s.voices {
		remove := v.done
		for _, p := range v.segments {
			if s.sampleOf(p.b) > j {
				remove = false
			}
		}
		if !remove {
			kept = append(kept, v)
		}
	}
	clear(s.voices[len(kept):])
	s.voices = kept
}

func (s *Source) Reset()       { s.voices, s.current = nil, nil }
func (s *Source) Stats() Stats { st := s.stats; st.Voices = len(s.voices); return st }
