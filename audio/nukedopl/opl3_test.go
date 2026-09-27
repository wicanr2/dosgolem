// SPDX-License-Identifier: LGPL-2.1-or-later
//
// Nuked OPL3 Go 移植與 C 參考的逐樣本一致性測試。
//
// Copyright (C) 2013-2020 Nuke.YKT
// Copyright (C) 2026 Tony Gies (Nuked-OPL3-fast modifications)
//
// 改作聲明：2026-09-27 為 Go 移植新增的測試（非上游檔案）。預期樣本由
// DOSBox-X src/hardware/nukedopl.cpp 以 g++ 編譯的參考程式產生，排程與樣本
// 放在 testdata/（產生方式見 SOURCE.md）。
//
// This library is free software; you can redistribute it and/or modify it
// under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation; either version 2.1 of the License, or (at
// your option) any later version. See COPYING.LGPL in this directory.

package nukedopl

import (
	"bufio"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

type schedWrite struct {
	idx  uint64
	addr uint16
	val  uint8
}

type schedule struct {
	rate   int
	end    uint64
	writes []schedWrite
}

// readSchedule 讀排程檔：rate／end 行與「樣本序號 位址(hex) 值(hex)」行。
func readSchedule(path string) (*schedule, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	s := &schedule{}
	sc := bufio.NewScanner(f)
	line := 0
	for sc.Scan() {
		line++
		t := strings.TrimSpace(sc.Text())
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		fs := strings.Fields(t)
		switch {
		case fs[0] == "rate" && len(fs) == 2:
			v, err := strconv.Atoi(fs[1])
			if err != nil {
				return nil, fmt.Errorf("%s:%d: %v", path, line, err)
			}
			s.rate = v
		case fs[0] == "end" && len(fs) == 2:
			v, err := strconv.ParseUint(fs[1], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("%s:%d: %v", path, line, err)
			}
			s.end = v
		case len(fs) == 3:
			idx, err1 := strconv.ParseUint(fs[0], 10, 64)
			a, err2 := strconv.ParseUint(fs[1], 16, 16)
			v, err3 := strconv.ParseUint(fs[2], 16, 8)
			if err1 != nil || err2 != nil || err3 != nil {
				return nil, fmt.Errorf("%s:%d: bad line %q", path, line, t)
			}
			if n := len(s.writes); n > 0 && idx < s.writes[n-1].idx {
				return nil, fmt.Errorf("%s:%d: non-monotonic index", path, line)
			}
			s.writes = append(s.writes, schedWrite{idx, uint16(a), uint8(v)})
		default:
			return nil, fmt.Errorf("%s:%d: bad line %q", path, line, t)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if s.rate == 0 {
		return nil, fmt.Errorf("%s: rate missing", path)
	}
	return s, nil
}

// render 照 C 參考程式的流程：到達寫入的樣本序號前先 GenerateStream，再 WriteRegBuffered。
// 以 1024 框為一批呼叫，與 DOSBox-X adlib.cpp 的分批一致（分批不影響結果）。
func render(s *schedule) []int16 {
	c := New(s.rate)
	out := make([]int16, 0, 2*s.end)
	buf := make([]int16, 2*1024)
	var pos uint64
	gen := func(n uint64) {
		for n > 0 {
			todo := n
			if todo > 1024 {
				todo = 1024
			}
			c.GenerateStream(buf[:2*todo])
			out = append(out, buf[:2*todo]...)
			n -= todo
			pos += todo
		}
	}
	for _, w := range s.writes {
		gen(w.idx - pos)
		c.WriteRegBuffered(w.addr, w.val)
	}
	gen(s.end - pos)
	return out
}

func readExpected(path string) ([]int16, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	raw, err := io.ReadAll(zr)
	if err != nil {
		return nil, err
	}
	if len(raw)%4 != 0 {
		return nil, fmt.Errorf("%s: %d bytes is not a whole number of stereo frames", path, len(raw))
	}
	out := make([]int16, len(raw)/2)
	for i := range out {
		out[i] = int16(binary.LittleEndian.Uint16(raw[2*i:]))
	}
	return out, nil
}

func TestConformanceWithCReference(t *testing.T) {
	names := []string{"tone", "perc", "random"}
	rates := []int{48000, 49716}
	for _, name := range names {
		for _, rate := range rates {
			base := fmt.Sprintf("%s_%d", name, rate)
			t.Run(base, func(t *testing.T) {
				s, err := readSchedule(filepath.Join("testdata", base+".sched"))
				if err != nil {
					t.Fatal(err)
				}
				if s.rate != rate {
					t.Fatalf("schedule rate %d, want %d", s.rate, rate)
				}
				want, err := readExpected(filepath.Join("testdata", base+".s16.gz"))
				if err != nil {
					t.Fatal(err)
				}
				got := render(s)
				if len(got) != len(want) {
					t.Fatalf("length: got %d samples, want %d", len(got), len(want))
				}
				mismatch, first := 0, -1
				nonzero := 0
				for i := range want {
					if want[i] != 0 {
						nonzero++
					}
					if got[i] != want[i] {
						if first < 0 {
							first = i
						}
						mismatch++
					}
				}
				if mismatch != 0 {
					t.Fatalf("%d/%d samples differ; first at sample %d (frame %d, ch %d): got %d want %d",
						mismatch, len(want), first, first/2, first%2, got[first], want[first])
				}
				// 參考輸出必須有聲音，否則比對沒有意義。
				if nonzero < len(want)/10 {
					t.Fatalf("reference output nearly silent: %d/%d nonzero", nonzero, len(want))
				}
				t.Logf("%d frames, %d writes, %d nonzero samples: identical", len(want)/2, len(s.writes), nonzero)
			})
		}
	}
}

// TestRateratio 固定 DOSBox-X 用到的兩個取樣率的重新取樣比例。
func TestRateratio(t *testing.T) {
	for _, tc := range []struct{ rate, want int }{{48000, 988}, {49716, 1024}, {44100, 908}} {
		if got := New(tc.rate).rateratio; int(got) != tc.want {
			t.Errorf("rate %d: rateratio %d, want %d", tc.rate, got, tc.want)
		}
	}
}

// TestResetReusesChip 確認 Reset 後重跑與新建晶片結果相同（指標重新指向本晶片）。
func TestResetReusesChip(t *testing.T) {
	s, err := readSchedule(filepath.Join("testdata", "tone_48000.sched"))
	if err != nil {
		t.Fatal(err)
	}
	want := render(s)
	c := New(12345)
	c.WriteReg(0xb0, 0x3f)
	buf := make([]int16, 200)
	c.GenerateStream(buf)
	c.Reset(s.rate)
	got := make([]int16, 0, len(want))
	var pos uint64
	b := make([]int16, 2)
	for _, w := range s.writes {
		for ; pos < w.idx; pos++ {
			c.GenerateStream(b)
			got = append(got, b...)
		}
		c.WriteRegBuffered(w.addr, w.val)
	}
	for ; pos < s.end; pos++ {
		c.GenerateStream(b)
		got = append(got, b...)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sample %d: got %d want %d", i, got[i], want[i])
		}
	}
}
