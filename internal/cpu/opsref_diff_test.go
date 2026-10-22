package cpu

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"
)

// TestExecuteCoreDifferential 對照值 switch 版 executeCore 與逐字保留的原布林 switch 版 executeCoreRef
// （perf 草稿 3a）：256 個 opcode × 三個機型 × 每格 60 組隨機狀態（實作時另以 300 組跑過一輪）（暫存器、旗標、段、IP、段覆寫、REP、
// 67 定址），兩顆 CPU 從同一狀態各執行一次，比回傳錯誤、panic、整個 CPU 結構（Bus 除外）、寫入與 OUT。
// 記憶體與 IN 由位址雜湊產生，不必每組配置 1 MB。
func TestExecuteCoreDifferential(t *testing.T) {
	models := []Model{Model8086, Model80186, Model80386}
	rng := rand.New(rand.NewSource(1))
	for _, model := range models {
		for op := 0; op < 256; op++ {
			for trial := 0; trial < 60; trial++ {
				seed := rng.Uint64()
				st := randCPUState(rng, model)
				a, b := st, st
				ba, bb := newHashBus(seed), newHashBus(seed)
				a.Bus, b.Bus = ba, bb
				ea, pa := runCore(&a, uint8(op), false)
				eb, pb := runCore(&b, uint8(op), true)
				where := fmt.Sprintf("model %d op %02X trial %d", model, op, trial)
				if fmt.Sprint(ea) != fmt.Sprint(eb) || fmt.Sprint(pa) != fmt.Sprint(pb) {
					t.Fatalf("%s：錯誤 %v／panic %v，參考 %v／%v", where, ea, pa, eb, pb)
				}
				a.Bus, b.Bus = nil, nil
				if a.ev != nil {
					a.ev = &a.evBuf
				}
				if b.ev != nil {
					b.ev = &b.evBuf
				}
				if !reflect.DeepEqual(a, b) {
					t.Fatalf("%s：CPU 狀態不同\n新 %+v\n參 %+v", where, a, b)
				}
				if !reflect.DeepEqual(ba.writes, bb.writes) || !reflect.DeepEqual(ba.outs, bb.outs) {
					t.Fatalf("%s：匯流排寫入不同", where)
				}
			}
		}
	}
}

func runCore(c *CPU, op uint8, ref bool) (err error, p any) {
	defer func() { p = recover() }()
	if ref {
		return c.executeCoreRef(op), nil
	}
	return c.executeCore(op), nil
}

func randCPUState(rng *rand.Rand, model Model) CPU {
	var c CPU
	c.Model = model
	for i := range c.R {
		c.R[i] = uint16(rng.Uint32())
	}
	for i := range c.Seg {
		c.Seg[i] = uint16(rng.Uint32())
	}
	c.IP = uint16(rng.Uint32())
	c.SetFlags(uint16(rng.Uint32()))
	c.segOverride = noSegOverride
	if rng.Intn(4) == 0 {
		c.segOverride = rng.Intn(4)
	}
	switch rng.Intn(4) {
	case 0:
		c.repPrefix = 0xF2
	case 1:
		c.repPrefix = 0xF3
	}
	if model == Model80386 {
		for i := range c.Hi {
			c.Hi[i] = uint16(rng.Uint32())
		}
		c.FSeg, c.GSeg = uint16(rng.Uint32()), uint16(rng.Uint32())
		if rng.Intn(4) == 0 {
			c.addr32, c.ext386 = true, true
		}
	}
	return c
}

// hashBus：讀取＝寫過的值，否則位址雜湊；IN＝埠雜湊。寫入與 OUT 依序記錄。
type hashBus struct {
	seed   uint64
	mem    map[uint32]uint8
	writes [][2]uint32
	outs   [][2]uint32
}

func newHashBus(seed uint64) *hashBus { return &hashBus{seed: seed, mem: map[uint32]uint8{}} }

func mix(x uint64) uint8 {
	x ^= x >> 33
	x *= 0xff51afd7ed558ccd
	x ^= x >> 33
	return uint8(x)
}

func (b *hashBus) Read8(a uint32) uint8 {
	if v, ok := b.mem[a]; ok {
		return v
	}
	return mix(b.seed ^ uint64(a)*0x9E3779B97F4A7C15)
}

func (b *hashBus) Write8(a uint32, v uint8) {
	b.mem[a] = v
	b.writes = append(b.writes, [2]uint32{a, uint32(v)})
}

func (b *hashBus) In8(p uint16) uint8 { return mix(b.seed ^ uint64(p)<<40 ^ 0xABCDEF) }

func (b *hashBus) Out8(p uint16, v uint8) { b.outs = append(b.outs, [2]uint32{uint32(p), uint32(v)}) }
