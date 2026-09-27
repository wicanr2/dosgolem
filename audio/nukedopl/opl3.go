// SPDX-License-Identifier: LGPL-2.1-or-later
//
// Nuked OPL3 模擬器（Go 移植）。
//
// Copyright (C) 2013-2020 Nuke.YKT
// Copyright (C) 2026 Tony Gies (Nuked-OPL3-fast modifications)
//
// 改作聲明：2026-09-27 由 DOSBox-X src/hardware/nukedopl.cpp 與 nukedopl.h
// （Nuked-OPL3-fast 1.8-fast.1 @ fb9afa9；上游 Nuked-OPL3 1.8 @ cfedb09）
// 移植為 Go。函式結構與運算逐一對應 C 原始碼，並固定
// OPL_ENABLE_STEREOEXT=0、OPL_QUIRK_CHANNELSAMPLEDELAY=1（未移植立體聲擴充）。
// C 的整數提升、有號右移與無號溢位語意以明確的型別轉換重現。
// 與 C 版的輸出以 testdata 內的排程逐樣本比對。
//
// Original upstream thanks:
//
//	MAME Development Team(Jarek Burczynski, Tatsuyuki Satoh):
//	    Feedback and Rhythm part calculation information.
//	forums.submarine.org.uk(carbon14, opl3):
//	    Tremolo and phase generator calculation information.
//	OPLx decapsulated(Matthew Gambrell, Olli Niemitalo):
//	    OPL2 ROMs.
//	siliconpr0n.org(John McMaster, digshadow):
//	    YMF262 and VRC VII decaps and die shots.
//
// This library is free software; you can redistribute it and/or modify it
// under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation; either version 2.1 of the License, or (at
// your option) any later version.
//
// This library is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of MERCHANTABILITY
// or FITNESS FOR A PARTICULAR PURPOSE. See the GNU Lesser General Public
// License for more details. A copy is in COPYING.LGPL in this directory.

// Package nukedopl 是 Nuked OPL3（Nuked-OPL3-fast 1.8-fast.1）的 Go 移植，
// 介面依 DOSBox-X adlib.cpp 的用法：New／Reset 指定輸出取樣率，
// WriteRegBuffered 寫入暫存器（帶寫入緩衝延遲），GenerateStream 產生交錯立體聲。
//
// Chip 內部以指標互相參照（slot、channel、調變來源），建立後不可複製值。
package nukedopl

import "math/bits"

const (
	writebufSize  = 1024 // OPL_WRITEBUF_SIZE
	writebufDelay = 2    // OPL_WRITEBUF_DELAY
	rsmFrac       = 10   // RSM_FRAC
)

// 通道型態。
const (
	ch2op   = 0
	ch4op   = 1
	ch4op2  = 2
	chDrum  = 3
	egkNorm = 0x01 // 包絡 key 型態
	egkDrum = 0x02
)

// 包絡階段。
const (
	envAttack  = 0
	envDecay   = 1
	envSustain = 2
	envRelease = 3
)

type slot struct {
	channel    *channel
	chip       *Chip
	mod        *int16
	trem       *uint8
	pgReset    uint32
	pgPhase    uint32
	pgInc      uint32
	out        int16
	fbmod      int16
	prout      int16
	egRout     uint16
	egOut      uint16
	egTlKsl    uint16 // (reg_tl << 2) + (eg_ksl >> kslshift[reg_ksl]) 的快取
	pgPhaseOut uint16
	key        uint8
	egGen      uint8
	regVib     uint8
	regMult    uint8
	regWf      uint8
	slotNum    uint8
	egKsl      uint8
	egKs       uint8
	regType    uint8
	regKsr     uint8
	regKsl     uint8
	regTl      uint8
	regAr      uint8
	regDr      uint8
	regSl      uint8
	regRr      uint8
	egRates    [4]uint8
	egRateHi   [4]uint8
	egRateLo   [4]uint8
}

type channel struct {
	slotz  [2]*slot
	pair   *channel
	chip   *Chip
	out    [4]*int16
	outCnt uint8
	chtype uint8
	fNum   uint16
	block  uint8
	fb     uint8
	con    uint8
	alg    uint8
	ksv    uint8
	cha    uint16
	chb    uint16
	chc    uint16
	chd    uint16
	chNum  uint8
}

type writebuf struct {
	time uint64
	reg  uint16
	data uint8
}

// Chip 對應 C 的 opl3_chip。
type Chip struct {
	channel      [18]channel
	slot         [36]slot
	timer        uint16
	egTimer      uint64
	egTimerrem   uint8
	egState      uint8
	egAdd        uint8
	egTimerLo    uint8
	newm         uint8
	nts          uint8
	rhy          uint8
	vibpos       uint8
	vibshift     uint8
	tremolo      uint8
	tremolopos   uint8
	tremoloshift uint8
	tremoloDirty uint8
	noise        uint32
	zeromod      int16
	// zerotrem 取代 C 的 (uint8_t*)&chip->zeromod：C 以位元組指標讀 zeromod
	// 的低位元組，而 zeromod 永不寫入，恆為 0；Go 不能對 int16 取 *uint8，
	// 改用獨立的恆零欄位，讀值相同。
	zerotrem   uint8
	mixbuff    [4]int32
	rmHhBit2   uint8
	rmHhBit3   uint8
	rmHhBit7   uint8
	rmHhBit8   uint8
	rmTcBit3   uint8
	rmTcBit5   uint8
	rateratio  int32
	samplecnt  int32
	oldsamples [4]int16
	samples    [4]int16

	writebufSamplecnt uint64
	writebufCur       uint32
	writebufLast      uint32
	writebufLasttime  uint64
	writebuf          [writebufSize]writebuf
}

// New 建立一顆晶片並以 sampleRate（輸出取樣率，Hz）重設。
func New(sampleRate int) *Chip {
	c := new(Chip)
	c.Reset(sampleRate)
	return c
}

/*
   包絡產生器
*/

func envelopeUpdateKSL(s *slot) {
	ksl := int16(int32(kslrom[s.channel.fNum>>6])<<2 - (int32(0x08)-int32(s.channel.block))<<5)
	if ksl < 0 {
		ksl = 0
	}
	s.egKsl = uint8(ksl)
	s.egTlKsl = uint16(int32(s.regTl)<<2 + int32(s.egKsl>>kslshift[s.regKsl]))
}

func envelopeUpdateRate(s *slot) {
	s.egKs = s.channel.ksv >> ((s.regKsr ^ 1) << 1)
	for ii := 0; ii < 4; ii++ {
		rate := s.egKs + (s.egRates[ii] << 2)
		rateHi := rate >> 2
		if rateHi&0x10 != 0 {
			rateHi = 0x0f
		}
		s.egRateHi[ii] = rateHi
		s.egRateLo[ii] = rate & 0x03
	}
}

func envelopeCalc(s *slot) {
	var (
		nonzero bool
		rateHi  uint8
		rateLo  uint8
		regRate uint8
		egShift uint8
		shift   uint8
		egRout  uint16
		egInc   int16
		egOff   bool
		reset   bool
	)

	s.egOut = s.egRout + s.egTlKsl + uint16(*s.trem)
	if s.key != 0 && s.egGen == envRelease {
		reset = true
		regRate = s.egRates[0]
	} else {
		regRate = s.egRates[s.egGen]
	}
	if reset {
		s.pgReset = 1
	} else {
		s.pgReset = 0
	}
	nonzero = regRate != 0
	if reset {
		rateHi = s.egRateHi[0]
		rateLo = s.egRateLo[0]
	} else {
		rateHi = s.egRateHi[s.egGen]
		rateLo = s.egRateLo[s.egGen]
	}
	egShift = rateHi + s.chip.egAdd
	shift = 0
	if nonzero {
		if rateHi < 12 {
			if s.chip.egState != 0 {
				switch egShift {
				case 12:
					shift = 1
				case 13:
					shift = (rateLo >> 1) & 0x01
				case 14:
					shift = rateLo & 0x01
				}
			}
		} else {
			shift = (rateHi & 0x03) + egIncstep[rateLo][s.chip.egTimerLo]
			if shift&0x04 != 0 {
				shift = 0x03
			}
			if shift == 0 {
				shift = s.chip.egState
			}
		}
	}
	egRout = s.egRout
	egInc = 0
	egOff = false
	// 瞬間 attack
	if reset && rateHi == 0x0f {
		egRout = 0x00
	}
	// 包絡關閉
	if s.egRout&0x1f8 == 0x1f8 {
		egOff = true
	}
	if s.egGen != envAttack && !reset && egOff {
		egRout = 0x1ff
	}
	switch s.egGen {
	case envAttack:
		if s.egRout == 0 {
			s.egGen = envDecay
		} else if s.key != 0 && shift > 0 && rateHi != 0x0f {
			// C：~slot->eg_rout 先提升為 int 再取補數（負數），再做有號右移。
			egInc = int16(^int32(s.egRout) >> (4 - shift))
		}
	case envDecay:
		if s.egRout>>4 == uint16(s.regSl) {
			s.egGen = envSustain
		} else if !egOff && !reset && shift > 0 {
			egInc = int16(1) << (shift - 1)
		}
	case envSustain, envRelease:
		if !egOff && !reset && shift > 0 {
			egInc = int16(1) << (shift - 1)
		}
	}
	s.egRout = uint16((int32(egRout) + int32(egInc)) & 0x1ff)
	// key off
	if reset {
		s.egGen = envAttack
	}
	if s.key == 0 {
		s.egGen = envRelease
	}
}

func envelopeKeyOn(s *slot, typ uint8) {
	s.key |= typ
}

func envelopeKeyOff(s *slot, typ uint8) {
	s.key &^= typ
}

/*
   相位產生器
*/

func phaseUpdateInc(s *slot) {
	basefreq := (uint32(s.channel.fNum) << s.channel.block) >> 1
	s.pgInc = (basefreq * uint32(mt[s.regMult])) >> 1
}

// vibratoInc 是 C 在 OPL3_PhaseGenerate 與 OPL3_ProcessSlot 快速路徑中
// 重複的顫音相位增量計算（兩處運算相同）。
func vibratoInc(s *slot) uint32 {
	chip := s.chip
	fNum := s.channel.fNum
	rng := int8((fNum >> 7) & 7)
	vibpos := chip.vibpos
	if vibpos&3 == 0 {
		rng = 0
	} else if vibpos&1 != 0 {
		rng >>= 1
	}
	rng >>= chip.vibshift
	if vibpos&4 != 0 {
		rng = -rng
	}
	fNum = uint16(int32(fNum) + int32(rng))
	basefreq := (uint32(fNum) << s.channel.block) >> 1
	return (basefreq * uint32(mt[s.regMult])) >> 1
}

func phaseGenerate(s *slot) {
	chip := s.chip
	var phaseinc uint32
	if s.regVib != 0 {
		phaseinc = vibratoInc(s)
	} else {
		phaseinc = s.pgInc
	}
	phase := uint16(s.pgPhase >> 9)
	if s.pgReset != 0 {
		s.pgPhase = 0
	}
	s.pgPhase += phaseinc
	// 節奏模式：依 slot 編號分派（13 = hh、16 = sd、17 = tc）。
	noise := chip.noise
	s.pgPhaseOut = phase
	switch s.slotNum {
	case 13: // hh
		chip.rmHhBit2 = uint8(phase>>2) & 1
		chip.rmHhBit3 = uint8(phase>>3) & 1
		chip.rmHhBit7 = uint8(phase>>7) & 1
		chip.rmHhBit8 = uint8(phase>>8) & 1
		if chip.rhy&0x20 != 0 {
			rmXor := (chip.rmHhBit2 ^ chip.rmHhBit7) |
				(chip.rmHhBit3 ^ chip.rmTcBit5) |
				(chip.rmTcBit3 ^ chip.rmTcBit5)
			s.pgPhaseOut = uint16(rmXor) << 9
			if uint32(rmXor)^(noise&1) != 0 {
				s.pgPhaseOut |= 0xd0
			} else {
				s.pgPhaseOut |= 0x34
			}
		}
	case 16: // sd
		if chip.rhy&0x20 != 0 {
			s.pgPhaseOut = uint16(chip.rmHhBit8)<<9 |
				uint16(uint32(chip.rmHhBit8)^(noise&1))<<8
		}
	case 17: // tc
		if chip.rhy&0x20 != 0 {
			chip.rmTcBit3 = uint8(phase>>3) & 1
			chip.rmTcBit5 = uint8(phase>>5) & 1
			rmXor := (chip.rmHhBit2 ^ chip.rmHhBit7) |
				(chip.rmHhBit3 ^ chip.rmTcBit5) |
				(chip.rmTcBit3 ^ chip.rmTcBit5)
			s.pgPhaseOut = uint16(rmXor)<<9 | 0x80
		}
	}
	nBit := ((noise >> 14) ^ noise) & 0x01
	chip.noise = (noise >> 1) | (nBit << 22)
}

/*
   Slot
*/

func slotWrite20(s *slot, data uint8) {
	if (data>>7)&0x01 != 0 {
		s.trem = &s.chip.tremolo
	} else {
		s.trem = &s.chip.zerotrem
	}
	s.regVib = (data >> 6) & 0x01
	s.regType = (data >> 5) & 0x01
	if s.regType != 0 {
		s.egRates[2] = 0
	} else {
		s.egRates[2] = s.regRr
	}
	s.regKsr = (data >> 4) & 0x01
	s.regMult = data & 0x0f
	envelopeUpdateRate(s)
	phaseUpdateInc(s)
}

func slotWrite40(s *slot, data uint8) {
	s.regKsl = (data >> 6) & 0x03
	s.regTl = data & 0x3f
	envelopeUpdateKSL(s)
}

func slotWrite60(s *slot, data uint8) {
	s.regAr = (data >> 4) & 0x0f
	s.regDr = data & 0x0f
	s.egRates[0] = s.regAr
	s.egRates[1] = s.regDr
	envelopeUpdateRate(s)
}

func slotWrite80(s *slot, data uint8) {
	s.regSl = (data >> 4) & 0x0f
	if s.regSl == 0x0f {
		s.regSl = 0x1f
	}
	s.regRr = data & 0x0f
	if s.regType != 0 {
		s.egRates[2] = 0
	} else {
		s.egRates[2] = s.regRr
	}
	s.egRates[3] = s.regRr
	envelopeUpdateRate(s)
}

func slotWriteE0(s *slot, data uint8) {
	s.regWf = data & 0x07
	if s.chip.newm == 0x00 {
		s.regWf &= 0x03
	}
}

func slotGenerate(s *slot) {
	phase := uint16(int32(s.pgPhaseOut) + int32(*s.mod))
	envelope := s.egOut
	wfData := logsinWF[s.regWf][phase&0x3ff]
	neg := uint16(int16(wfData) >> 15)
	level := uint32(wfData&0x7fff) + uint32(envelope)<<3
	if level > 0x1fff {
		level = 0x1fff
	}
	s.out = int16((exprom[level&0xff] >> (level >> 8)) ^ neg)
}

// slotGenerateSilent：呼叫端已證明 eg_out >= 0x180，exp 查表結果必為 0，
// 輸出只剩波形的符號位元。
func slotGenerateSilent(s *slot) {
	phase := uint16(int32(s.pgPhaseOut) + int32(*s.mod))
	wfData := logsinWF[s.regWf][phase&0x3ff]
	s.out = int16(wfData) >> 15
}

func slotCalcFB(s *slot) {
	if s.channel.fb != 0x00 {
		s.fbmod = int16((int32(s.prout) + int32(s.out)) >> (0x09 - s.channel.fb))
	} else {
		s.fbmod = 0
	}
	s.prout = s.out
}

/*
   Channel
*/

func channelUpdateRhythm(chip *Chip, data uint8) {
	chip.rhy = data & 0x3f
	if chip.rhy&0x20 != 0 {
		channel6 := &chip.channel[6]
		channel7 := &chip.channel[7]
		channel8 := &chip.channel[8]
		channel6.out[0] = &channel6.slotz[1].out
		channel6.out[1] = &channel6.slotz[1].out
		channel6.out[2] = &chip.zeromod
		channel6.out[3] = &chip.zeromod
		channel6.outCnt = 2
		channel7.out[0] = &channel7.slotz[0].out
		channel7.out[1] = &channel7.slotz[0].out
		channel7.out[2] = &channel7.slotz[1].out
		channel7.out[3] = &channel7.slotz[1].out
		channel7.outCnt = 4
		channel8.out[0] = &channel8.slotz[0].out
		channel8.out[1] = &channel8.slotz[0].out
		channel8.out[2] = &channel8.slotz[1].out
		channel8.out[3] = &channel8.slotz[1].out
		channel8.outCnt = 4
		for chnum := 6; chnum < 9; chnum++ {
			chip.channel[chnum].chtype = chDrum
		}
		channelSetupAlg(channel6)
		channelSetupAlg(channel7)
		channelSetupAlg(channel8)
		// hh
		if chip.rhy&0x01 != 0 {
			envelopeKeyOn(channel7.slotz[0], egkDrum)
		} else {
			envelopeKeyOff(channel7.slotz[0], egkDrum)
		}
		// tc
		if chip.rhy&0x02 != 0 {
			envelopeKeyOn(channel8.slotz[1], egkDrum)
		} else {
			envelopeKeyOff(channel8.slotz[1], egkDrum)
		}
		// tom
		if chip.rhy&0x04 != 0 {
			envelopeKeyOn(channel8.slotz[0], egkDrum)
		} else {
			envelopeKeyOff(channel8.slotz[0], egkDrum)
		}
		// sd
		if chip.rhy&0x08 != 0 {
			envelopeKeyOn(channel7.slotz[1], egkDrum)
		} else {
			envelopeKeyOff(channel7.slotz[1], egkDrum)
		}
		// bd
		if chip.rhy&0x10 != 0 {
			envelopeKeyOn(channel6.slotz[0], egkDrum)
			envelopeKeyOn(channel6.slotz[1], egkDrum)
		} else {
			envelopeKeyOff(channel6.slotz[0], egkDrum)
			envelopeKeyOff(channel6.slotz[1], egkDrum)
		}
	} else {
		for chnum := 6; chnum < 9; chnum++ {
			chip.channel[chnum].chtype = ch2op
			channelSetupAlg(&chip.channel[chnum])
			envelopeKeyOff(chip.channel[chnum].slotz[0], egkDrum)
			envelopeKeyOff(chip.channel[chnum].slotz[1], egkDrum)
		}
	}
}

// channelUpdateFreq 是 C 的 Channel{A0,B0} 共用的後半段：刷新兩個 slot 的
// KSL、包絡速率與相位增量（呼叫順序與 C 相同）。
func channelUpdateFreq(ch *channel) {
	envelopeUpdateKSL(ch.slotz[0])
	envelopeUpdateKSL(ch.slotz[1])
	envelopeUpdateRate(ch.slotz[0])
	envelopeUpdateRate(ch.slotz[1])
	phaseUpdateInc(ch.slotz[0])
	phaseUpdateInc(ch.slotz[1])
}

func channelKsv(ch *channel) uint8 {
	return (ch.block << 1) | uint8((ch.fNum>>(0x09-ch.chip.nts))&0x01)
}

func channelWriteA0(ch *channel, data uint8) {
	if ch.chip.newm != 0 && ch.chtype == ch4op2 {
		return
	}
	ch.fNum = (ch.fNum & 0x300) | uint16(data)
	ch.ksv = channelKsv(ch)
	channelUpdateFreq(ch)
	if ch.chip.newm != 0 && ch.chtype == ch4op {
		ch.pair.fNum = ch.fNum
		ch.pair.ksv = ch.ksv
		channelUpdateFreq(ch.pair)
	}
}

func channelWriteB0(ch *channel, data uint8) {
	if ch.chip.newm != 0 && ch.chtype == ch4op2 {
		return
	}
	ch.fNum = (ch.fNum & 0xff) | (uint16(data&0x03) << 8)
	ch.block = (data >> 2) & 0x07
	ch.ksv = channelKsv(ch)
	channelUpdateFreq(ch)
	if ch.chip.newm != 0 && ch.chtype == ch4op {
		ch.pair.fNum = ch.fNum
		ch.pair.block = ch.block
		ch.pair.ksv = ch.ksv
		channelUpdateFreq(ch.pair)
	}
}

func channelSetupAlg(ch *channel) {
	zero := &ch.chip.zeromod
	if ch.chtype == chDrum {
		if ch.chNum == 7 || ch.chNum == 8 {
			ch.slotz[0].mod = zero
			ch.slotz[1].mod = zero
			return
		}
		switch ch.alg & 0x01 {
		case 0x00:
			ch.slotz[0].mod = &ch.slotz[0].fbmod
			ch.slotz[1].mod = &ch.slotz[0].out
		case 0x01:
			ch.slotz[0].mod = &ch.slotz[0].fbmod
			ch.slotz[1].mod = zero
		}
		return
	}
	if ch.alg&0x08 != 0 {
		return
	}
	if ch.alg&0x04 != 0 {
		p := ch.pair
		p.out[0] = zero
		p.out[1] = zero
		p.out[2] = zero
		p.out[3] = zero
		p.outCnt = 0
		switch ch.alg & 0x03 {
		case 0x00:
			p.slotz[0].mod = &p.slotz[0].fbmod
			p.slotz[1].mod = &p.slotz[0].out
			ch.slotz[0].mod = &p.slotz[1].out
			ch.slotz[1].mod = &ch.slotz[0].out
			ch.out[0] = &ch.slotz[1].out
			ch.out[1] = zero
			ch.out[2] = zero
			ch.out[3] = zero
			ch.outCnt = 1
		case 0x01:
			p.slotz[0].mod = &p.slotz[0].fbmod
			p.slotz[1].mod = &p.slotz[0].out
			ch.slotz[0].mod = zero
			ch.slotz[1].mod = &ch.slotz[0].out
			ch.out[0] = &p.slotz[1].out
			ch.out[1] = &ch.slotz[1].out
			ch.out[2] = zero
			ch.out[3] = zero
			ch.outCnt = 2
		case 0x02:
			p.slotz[0].mod = &p.slotz[0].fbmod
			p.slotz[1].mod = zero
			ch.slotz[0].mod = &p.slotz[1].out
			ch.slotz[1].mod = &ch.slotz[0].out
			ch.out[0] = &p.slotz[0].out
			ch.out[1] = &ch.slotz[1].out
			ch.out[2] = zero
			ch.out[3] = zero
			ch.outCnt = 2
		case 0x03:
			p.slotz[0].mod = &p.slotz[0].fbmod
			p.slotz[1].mod = zero
			ch.slotz[0].mod = &p.slotz[1].out
			ch.slotz[1].mod = zero
			ch.out[0] = &p.slotz[0].out
			ch.out[1] = &ch.slotz[0].out
			ch.out[2] = &ch.slotz[1].out
			ch.out[3] = zero
			ch.outCnt = 3
		}
	} else {
		switch ch.alg & 0x01 {
		case 0x00:
			ch.slotz[0].mod = &ch.slotz[0].fbmod
			ch.slotz[1].mod = &ch.slotz[0].out
			ch.out[0] = &ch.slotz[1].out
			ch.out[1] = zero
			ch.out[2] = zero
			ch.out[3] = zero
			ch.outCnt = 1
		case 0x01:
			ch.slotz[0].mod = &ch.slotz[0].fbmod
			ch.slotz[1].mod = zero
			ch.out[0] = &ch.slotz[0].out
			ch.out[1] = &ch.slotz[1].out
			ch.out[2] = zero
			ch.out[3] = zero
			ch.outCnt = 2
		}
	}
}

func channelUpdateAlg(ch *channel) {
	ch.alg = ch.con
	if ch.chip.newm != 0 {
		if ch.chtype == ch4op {
			ch.pair.alg = 0x04 | (ch.con << 1) | ch.pair.con
			ch.alg = 0x08
			channelSetupAlg(ch.pair)
		} else if ch.chtype == ch4op2 {
			ch.alg = 0x04 | (ch.pair.con << 1) | ch.con
			ch.pair.alg = 0x08
			channelSetupAlg(ch)
		} else {
			channelSetupAlg(ch)
		}
	} else {
		channelSetupAlg(ch)
	}
}

func bitMask16(b uint8) uint16 {
	if b != 0 {
		return 0xffff // C：~0 截成 uint16
	}
	return 0
}

func channelWriteC0(ch *channel, data uint8) {
	ch.fb = (data & 0x0e) >> 1
	ch.con = data & 0x01
	channelUpdateAlg(ch)
	if ch.chip.newm != 0 {
		ch.cha = bitMask16((data >> 4) & 0x01)
		ch.chb = bitMask16((data >> 5) & 0x01)
		ch.chc = bitMask16((data >> 6) & 0x01)
		ch.chd = bitMask16((data >> 7) & 0x01)
	} else {
		ch.cha, ch.chb = 0xffff, 0xffff
		// 上游 TODO：相容模式下 DAC2 輸出是否關閉尚待實機確認。
		ch.chc, ch.chd = 0, 0
	}
}

func channelKeyOn(ch *channel) {
	if ch.chip.newm != 0 {
		if ch.chtype == ch4op {
			envelopeKeyOn(ch.slotz[0], egkNorm)
			envelopeKeyOn(ch.slotz[1], egkNorm)
			envelopeKeyOn(ch.pair.slotz[0], egkNorm)
			envelopeKeyOn(ch.pair.slotz[1], egkNorm)
		} else if ch.chtype == ch2op || ch.chtype == chDrum {
			envelopeKeyOn(ch.slotz[0], egkNorm)
			envelopeKeyOn(ch.slotz[1], egkNorm)
		}
	} else {
		envelopeKeyOn(ch.slotz[0], egkNorm)
		envelopeKeyOn(ch.slotz[1], egkNorm)
	}
}

func channelKeyOff(ch *channel) {
	if ch.chip.newm != 0 {
		if ch.chtype == ch4op {
			envelopeKeyOff(ch.slotz[0], egkNorm)
			envelopeKeyOff(ch.slotz[1], egkNorm)
			envelopeKeyOff(ch.pair.slotz[0], egkNorm)
			envelopeKeyOff(ch.pair.slotz[1], egkNorm)
		} else if ch.chtype == ch2op || ch.chtype == chDrum {
			envelopeKeyOff(ch.slotz[0], egkNorm)
			envelopeKeyOff(ch.slotz[1], egkNorm)
		}
	} else {
		envelopeKeyOff(ch.slotz[0], egkNorm)
		envelopeKeyOff(ch.slotz[1], egkNorm)
	}
}

func channelSet4Op(chip *Chip, data uint8) {
	for bit := uint8(0); bit < 6; bit++ {
		chnum := bit
		if bit >= 3 {
			chnum += 9 - 3
		}
		if (data>>bit)&0x01 != 0 {
			chip.channel[chnum].chtype = ch4op
			chip.channel[chnum+3].chtype = ch4op2
			channelUpdateAlg(&chip.channel[chnum])
		} else {
			chip.channel[chnum].chtype = ch2op
			chip.channel[chnum+3].chtype = ch2op
			channelUpdateAlg(&chip.channel[chnum])
			channelUpdateAlg(&chip.channel[chnum+3])
		}
	}
}

func clipSample(sample int32) int16 {
	if sample > 32767 {
		sample = 32767
	} else if sample < -32768 {
		sample = -32768
	}
	return int16(sample)
}

func isRhythmPhaseSlot(n uint8) bool {
	return n == 13 || n == 16 || n == 17
}

func processSlot(s *slot) {
	// 快速路徑：key-off、完全衰減、非節奏相位 slot。包絡速率機在此不會改變
	// eg_rout，但完整路徑仍會更新回授歷史、eg_out/eg_gen/pg_reset、相位、
	// 噪音與輸出，這裡照做。
	if s.key == 0 && s.egRout == 0x1ff && !isRhythmPhaseSlot(s.slotNum) {
		chip := s.chip
		noise := chip.noise
		nBit := ((noise >> 14) ^ noise) & 0x01

		if s.channel.fb == 0 && s.pgInc == 0 && s.out == 0 &&
			*s.mod == 0 && s.egTlKsl == 0 && *s.trem == 0 &&
			s.pgPhase == 0 && s.regVib == 0 && s.regWf == 0 {
			s.fbmod = 0
			s.prout = 0
			s.egOut = 0x1ff
			s.pgReset = 0
			s.egGen = envRelease
			s.pgPhaseOut = 0
			chip.noise = (noise >> 1) | (nBit << 22)
			return
		}

		slotCalcFB(s)

		s.egOut = s.egRout + s.egTlKsl + uint16(*s.trem)
		s.pgReset = 0
		s.egGen = envRelease

		var phaseinc uint32
		if s.regVib != 0 {
			phaseinc = vibratoInc(s)
		} else {
			phaseinc = s.pgInc
		}

		phase := uint16(s.pgPhase >> 9)
		s.pgPhase += phaseinc
		s.pgPhaseOut = phase
		chip.noise = (noise >> 1) | (nBit << 22)

		// 此處 eg_out >= 0x1ff，靜音捷徑必然成立。
		slotGenerateSilent(s)
		return
	}
	if s.egGen == envSustain && s.key != 0 && s.egRates[envSustain] == 0 {
		slotCalcFB(s)
		s.egOut = s.egRout + s.egTlKsl + uint16(*s.trem)
		s.pgReset = 0
		if s.egRout&0x1f8 == 0x1f8 {
			s.egRout = 0x1ff
		}

		if s.regVib == 0 && !isRhythmPhaseSlot(s.slotNum) {
			chip := s.chip
			noise := chip.noise
			nBit := ((noise >> 14) ^ noise) & 0x01
			phase := uint16(s.pgPhase >> 9)

			s.pgPhase += s.pgInc
			s.pgPhaseOut = phase
			chip.noise = (noise >> 1) | (nBit << 22)
		} else {
			phaseGenerate(s)
		}

		slotGenerate(s)
		return
	}
	slotCalcFB(s)
	envelopeCalc(s)
	phaseGenerate(s)
	slotGenerate(s)
}

// mixSide 對應 C 的兩個 18 通道混音迴圈。first 為 true 時是第一個迴圈
// （cha／chc 遮罩，cha|chc 為 0 的通道略過），否則是第二個迴圈（chb／chd）。
func (c *Chip) mixSide(first bool) (int32, int32) {
	var mix0, mix1 int32
	for ii := 0; ii < 18; ii++ {
		ch := &c.channel[ii]
		if ch.outCnt == 0 {
			continue
		}
		if first && ch.cha|ch.chc == 0 {
			continue
		}
		out := &ch.out
		accm := *out[0]
		if ch.outCnt > 1 {
			accm += *out[1]
			if ch.outCnt > 2 {
				accm += *out[2]
				if ch.outCnt > 3 {
					accm += *out[3]
				}
			}
		}
		// C：(int16_t)(accm & mask)，accm 以符號延伸提升為 int。
		if first {
			mix0 += int32(int16(uint16(accm) & ch.cha))
		} else {
			mix0 += int32(int16(uint16(accm) & ch.chb))
		}
		if first {
			mix1 += int32(int16(uint16(accm) & ch.chc))
		} else {
			mix1 += int32(int16(uint16(accm) & ch.chd))
		}
	}
	return mix0, mix1
}

// generate4Ch 對應 C 的 OPL3_Generate4Ch（OPL_QUIRK_CHANNELSAMPLEDELAY=1）。
func (c *Chip) generate4Ch(buf4 *[4]int16) {
	buf4[1] = clipSample(c.mixbuff[1])
	buf4[3] = clipSample(c.mixbuff[3])

	for ii := 0; ii < 15; ii++ {
		processSlot(&c.slot[ii])
	}

	c.mixbuff[0], c.mixbuff[2] = c.mixSide(true)

	for ii := 15; ii < 18; ii++ {
		processSlot(&c.slot[ii])
	}

	buf4[0] = clipSample(c.mixbuff[0])
	buf4[2] = clipSample(c.mixbuff[2])

	for ii := 18; ii < 33; ii++ {
		processSlot(&c.slot[ii])
	}

	c.mixbuff[1], c.mixbuff[3] = c.mixSide(false)

	for ii := 33; ii < 36; ii++ {
		processSlot(&c.slot[ii])
	}

	updateTremolo := c.tremoloDirty
	if c.timer&0x3f == 0x3f {
		c.tremolopos++
		if c.tremolopos == 210 {
			c.tremolopos = 0
		}
		updateTremolo = 1
	}
	if updateTremolo != 0 {
		if c.tremolopos < 105 {
			c.tremolo = c.tremolopos >> c.tremoloshift
		} else {
			c.tremolo = (210 - c.tremolopos) >> c.tremoloshift
		}
		c.tremoloDirty = 0
	}

	if c.timer&0x3ff == 0x3ff {
		c.vibpos = (c.vibpos + 1) & 7
	}

	c.timer++

	if c.egState != 0 {
		egTimerLow := uint32(c.egTimer) & 0x1fff
		if egTimerLow == 0 {
			c.egAdd = 0
		} else {
			c.egAdd = uint8(bits.TrailingZeros32(egTimerLow)) + 1
		}
		c.egTimerLo = uint8(c.egTimer & 0x3)
	}

	if c.egTimerrem != 0 || c.egState != 0 {
		if c.egTimer == 0xfffffffff {
			c.egTimer = 0
			c.egTimerrem = 1
		} else {
			c.egTimer++
			c.egTimerrem = 0
		}
	}

	c.egState ^= 1

	for {
		wb := &c.writebuf[c.writebufCur]
		if wb.time > c.writebufSamplecnt {
			break
		}
		if wb.reg&0x200 == 0 {
			break
		}
		wb.reg &= 0x1ff
		c.WriteReg(wb.reg, wb.data)
		c.writebufCur = (c.writebufCur + 1) % writebufSize
	}
	c.writebufSamplecnt++
}

// generate4ChResampled 對應 C 的 OPL3_Generate4ChResampled：49716 Hz 線性內插到輸出率。
func (c *Chip) generate4ChResampled(buf4 *[4]int16) {
	for c.samplecnt >= c.rateratio {
		c.oldsamples = c.samples
		c.generate4Ch(&c.samples)
		c.samplecnt -= c.rateratio
	}
	for i := 0; i < 4; i++ {
		buf4[i] = int16((int32(c.oldsamples[i])*(c.rateratio-c.samplecnt) +
			int32(c.samples[i])*c.samplecnt) / c.rateratio)
	}
	c.samplecnt += 1 << rsmFrac
}

// Reset 對應 C 的 OPL3_Reset：清空晶片並設定輸出取樣率。
func (c *Chip) Reset(sampleRate int) {
	*c = Chip{}
	for slotnum := 0; slotnum < 36; slotnum++ {
		s := &c.slot[slotnum]
		s.chip = c
		s.mod = &c.zeromod
		s.egRout = 0x1ff
		s.egOut = 0x1ff
		s.egGen = envRelease
		s.trem = &c.zerotrem
		s.egRates = [4]uint8{}
		s.slotNum = uint8(slotnum)
	}
	for channum := 0; channum < 18; channum++ {
		ch := &c.channel[channum]
		localChSlot := chSlot[channum]
		ch.slotz[0] = &c.slot[localChSlot]
		ch.slotz[1] = &c.slot[localChSlot+3]
		c.slot[localChSlot].channel = ch
		c.slot[localChSlot+3].channel = ch
		if channum%9 < 3 {
			ch.pair = &c.channel[channum+3]
		} else if channum%9 < 6 {
			ch.pair = &c.channel[channum-3]
		}
		ch.chip = c
		ch.out[0] = &c.zeromod
		ch.out[1] = &c.zeromod
		ch.out[2] = &c.zeromod
		ch.out[3] = &c.zeromod
		ch.outCnt = 0
		ch.chtype = ch2op
		ch.cha = 0xffff
		ch.chb = 0xffff
		ch.chNum = uint8(channum)
		channelSetupAlg(ch)
	}
	c.noise = 1
	// C：(samplerate << RSM_FRAC) / 49716，samplerate 為 uint32。
	c.rateratio = int32((uint32(sampleRate) << rsmFrac) / 49716)
	c.tremoloshift = 4
	c.vibshift = 1
}

// WriteReg 對應 C 的 OPL3_WriteReg：立即寫入暫存器（不經寫入緩衝）。
// reg 的 bit 8 選第二組。
func (c *Chip) WriteReg(reg uint16, v uint8) {
	high := uint8((reg >> 8) & 0x01)
	regm := uint8(reg & 0xff)
	switch regm & 0xf0 {
	case 0x00:
		if high != 0 {
			switch regm & 0x0f {
			case 0x04:
				channelSet4Op(c, v)
			case 0x05:
				c.newm = v & 0x01
			}
		} else {
			switch regm & 0x0f {
			case 0x08:
				c.nts = (v >> 6) & 0x01
			}
		}
	case 0x20, 0x30:
		if n := adSlot[regm&0x1f]; n >= 0 {
			slotWrite20(&c.slot[18*int(high)+int(n)], v)
		}
	case 0x40, 0x50:
		if n := adSlot[regm&0x1f]; n >= 0 {
			slotWrite40(&c.slot[18*int(high)+int(n)], v)
		}
	case 0x60, 0x70:
		if n := adSlot[regm&0x1f]; n >= 0 {
			slotWrite60(&c.slot[18*int(high)+int(n)], v)
		}
	case 0x80, 0x90:
		if n := adSlot[regm&0x1f]; n >= 0 {
			slotWrite80(&c.slot[18*int(high)+int(n)], v)
		}
	case 0xe0, 0xf0:
		if n := adSlot[regm&0x1f]; n >= 0 {
			slotWriteE0(&c.slot[18*int(high)+int(n)], v)
		}
	case 0xa0:
		if regm&0x0f < 9 {
			channelWriteA0(&c.channel[9*int(high)+int(regm&0x0f)], v)
		}
	case 0xb0:
		if regm == 0xbd && high == 0 {
			tremoloshift := (((v >> 7) ^ 1) << 1) + 2
			if c.tremoloshift != tremoloshift {
				c.tremoloDirty = 1
			}
			c.tremoloshift = tremoloshift
			c.vibshift = ((v >> 6) & 0x01) ^ 1
			channelUpdateRhythm(c, v)
		} else if regm&0x0f < 9 {
			ch := &c.channel[9*int(high)+int(regm&0x0f)]
			channelWriteB0(ch, v)
			if v&0x20 != 0 {
				channelKeyOn(ch)
			} else {
				channelKeyOff(ch)
			}
		}
	case 0xc0:
		if regm&0x0f < 9 {
			channelWriteC0(&c.channel[9*int(high)+int(regm&0x0f)], v)
		}
	}
}

// WriteRegBuffered 對應 C 的 OPL3_WriteRegBuffered：寫入排進緩衝，
// 至少間隔 OPL_WRITEBUF_DELAY（2）個內部樣本才生效；緩衝滿（1024 筆）時
// 立即套用最舊的一筆。
func (c *Chip) WriteRegBuffered(reg uint16, v uint8) {
	writebufLast := c.writebufLast
	wb := &c.writebuf[writebufLast]

	if wb.reg&0x200 != 0 {
		c.WriteReg(wb.reg&0x1ff, wb.data)

		c.writebufCur = (writebufLast + 1) % writebufSize
		c.writebufSamplecnt = wb.time
	}

	wb.reg = reg | 0x200
	wb.data = v
	time1 := c.writebufLasttime + writebufDelay
	time2 := c.writebufSamplecnt

	if time1 < time2 {
		time1 = time2
	}

	wb.time = time1
	c.writebufLasttime = time1
	c.writebufLast = (writebufLast + 1) % writebufSize
}

// GenerateStream 對應 C 的 OPL3_GenerateStream：產生 len(dst)/2 個交錯立體聲框
// （重新取樣到 Reset 指定的輸出率）。
func (c *Chip) GenerateStream(dst []int16) {
	var samples [4]int16
	n := len(dst) / 2
	for i := 0; i < n; i++ {
		c.generate4ChResampled(&samples)
		dst[2*i] = samples[0]
		dst[2*i+1] = samples[1]
	}
}
