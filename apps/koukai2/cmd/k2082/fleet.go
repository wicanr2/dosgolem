package main

// 批次 082 項 1：NPC 艦隊（docs/spec/060）的觀測。每個 hook 記一筆 f 事件，內容由旗標字母決定：
//
//	F  艦隊結構（DS:BX，133 bytes）          x  上下文（byte_5443A、國別殘值、時段、地點、視窗、玩家指標）
//	t  壓縮表（70 艦隊各 10 bytes、120 人物各 6 bytes）  a  AX（返回值或參數）   d  DX
//	T  目標人物（AX）、其艦隊、主角人物與 nation 表      p  AX 指到的 SS 內 4 bytes（航點）
//	q  進入時存下的 SS 指標所指的 4 bytes（用在 2A0CA 返回）  m  F.+0x1C 港口的 10 個類別價格修正
//
// 只觀測，不改被觀測程式的記憶體。每個 hook 去重（中斷返回造成的重複觸發）。

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/wicanr2/dosgolem/oracle"
)

func u16(o *oracle.Oracle, b Bind, off uint16) uint16 { return o.Word(b.DS(off)) }
func u8(o *oracle.Oracle, b Bind, off uint16) uint8  { return o.Byte(b.DS(off)) }

const (
	fleetBase  = 0x2E76
	fleetSize  = 133
	nFleets    = 70
	personBase = 0x12B6
	personSize = 50
	nPersons   = 120
)

type fspec struct {
	ida   uint32
	label string
	flags string
}

var fleetSpecs = []fspec{
	{0x2AC19, "fl_in", "Fxt"}, {0x2ACBE, "fl_out", "Fx"},
	{0x2A6D8, "p6D8_in", "Fxt"}, {0x2A748, "p6D8_out", "F"}, {0x2A759, "p6D8_out", "F"},
	{0x2A4A5, "p4A5_in", "Fxt"}, {0x2A6D7, "p4A5_out", "Fx"},
	{0x2AA79, "pAA79_in", "Fxt"}, {0x2AB8A, "pAA79_out", "Fa"},
	{0x2A7A8, "p7A8_in", "FxtTa"}, {0x2AA78, "p7A8_out", "FxtT"},
	{0x2A75A, "p75A_in", "F"}, {0x2A7A7, "p75A_out", "F"},
	{0x4407F, "d407F_in", "Fx"}, {0x440BE, "d407F_out", "F"},
	{0x43CB4, "h0_in", "Fxmn"}, {0x43DBE, "h0_out", "Fxm"},
	{0x43DBF, "h1_in", "Fxmne"}, {0x43F47, "h1_out", "Fmne"},
	{0x43F48, "h2_in", "Fxmne"}, {0x44075, "h2_out", "Fxme"},
	{0x43CB3, "c9d_out", "a"}, {0x43C9C, "c2d_out", "a"}, {0x43920, "c4e_out", "a"},
	{0x43B0B, "b0b_in", "Fxn"}, {0x43C2C, "b0b_out", "F"},
	{0x43921, "s921_in", "Fad"}, {0x439E8, "s921_out", "F"},
	{0x439E9, "s9E9_in", "Fadt"}, {0x43AAE, "s9E9_out", "F"}, {0x43B0A, "s9E9_out", "F"},
	{0x2A16D, "a16D_in", "Fxt"}, {0x2A1DF, "a16D_out", "F"},
	{0x2A1E0, "a1E0_in", "Fxp"}, {0x2A4A4, "a1E0_out", "F"},
	{0x2A0CA, "a0CA_in", "F"}, {0x2A16C, "a0CA_out", "Fq"},
	{0x341C2, "r1C2_in", "F"}, {0x34267, "r1C2_out", "Fa"},
	{0x28299, "d299_in", "F"}, {0x282DE, "d299_out", "F"},
	{0x2AFE8, "w_in", "x"}, {0x2B1A2, "w_out", "x"},
}

var lastTFid = 0xFF

func (e *env) installFleet(only map[string]bool) {
	stacks := map[string][]uint16{}
	portStacks := map[string][]int{}
	var savedPtr uint16
	o := e.o
	b, _ := CurrentOrMain(o)
	for _, sp := range fleetSpecs {
		if len(only) > 0 && !only[sp.label] {
			continue
		}
		sp := sp
		d := &dedup{}
		base := strings.TrimSuffix(strings.TrimSuffix(sp.label, "_in"), "_out")
		isOut := strings.HasSuffix(sp.label, "_out")
		isIn := strings.HasSuffix(sp.label, "_in")
		st := &HookStat82{}
		e.stats["f_"+sp.label] = st
		o.OnCall(b.Code(sp.ida), func(o *oracle.Oracle) {
			if d.again(o) {
				st.Dup++
				return
			}
			st.Total++
			st.Logged++
			r := o.Regs()
			fp := r.BX
			if isIn {
				stacks[base] = append(stacks[base], r.BX)
			}
			if isOut {
				if s := stacks[base]; len(s) > 0 {
					fp = s[len(s)-1]
					stacks[base] = s[:len(s)-1]
				}
			}
			if strings.HasPrefix(sp.label, "h") {
				fp = u16(o, b, 0xBD7C) // 到達處理函式從 word_53ECC 取艦隊指標，不是 BX
			}
			x := map[string]any{}
			if strings.Contains(sp.flags, "F") {
				x["fp"] = int(fp)
				x["F"] = hex.EncodeToString(o.Bytes(b.DS(fp), fleetSize))
			}
			if strings.Contains(sp.flags, "x") {
				x["idx"] = int(u8(o, b, 0xC2EA))
				x["nat"] = int(u8(o, b, 0xBDB8))
				x["slot"] = int(u8(o, b, 0x0535))
				x["port"] = int(u16(o, b, 0x0CAE))
				x["win"] = []int{int(u16(o, b, 0x06C8)), int(u8(o, b, 0x06CA)), int(u16(o, b, 0xBDA6)), int(u16(o, b, 0xBDAA))}
				x["pf"] = int(u16(o, b, 0xC2D4))
				x["pc"] = int(u16(o, b, 0xC2CE))
				x["hero"] = int(u8(o, b, 0x12B5))
				x["rank"] = int(u8(o, b, uint16(0x1267+0x0E*int(u8(o, b, 0x12B5)))))
				x["day"] = hex.EncodeToString(o.Bytes(b.DS(0x0532), 4))
			}
			if strings.Contains(sp.flags, "t") {
				x["tabf"], x["tabp"] = fleetTab(o, b)
			}
			if strings.Contains(sp.flags, "a") {
				x["ax"] = int(r.AX)
			}
			if strings.Contains(sp.flags, "d") {
				x["dx"] = int(r.DX)
			}
			if strings.Contains(sp.flags, "T") {
				x["nation"] = hex.EncodeToString(o.Bytes(b.DS(0x117A), 32*7))
				x["hero_p"] = hex.EncodeToString(o.Bytes(b.DS(u16(o, b, 0xC2CE)), personSize))
				if sp.label == "p7A8_in" {
					t := int(r.AX)
					lastTFid = 0xFF
					if t < nPersons {
						x["t"] = t
						pp := b.DS(uint16(personBase + personSize*t))
						x["tp"] = hex.EncodeToString(o.Bytes(pp, personSize))
						lastTFid = int(o.Byte(oracle.Addr{Seg: pp.Seg, Off: pp.Off + 0x2C}))
					}
				}
				x["tfid"] = lastTFid
				if lastTFid < nFleets {
					x["T"] = hex.EncodeToString(o.Bytes(b.DS(uint16(fleetBase+fleetSize*lastTFid)), fleetSize))
				}
			}
			if strings.Contains(sp.flags, "p") {
				x["p"] = hex.EncodeToString(o.Bytes(oracle.Addr{Seg: r.SS, Off: r.AX}, 4))
			}
			if sp.label == "a0CA_in" {
				savedPtr = r.AX
			}
			if strings.Contains(sp.flags, "q") {
				x["p"] = hex.EncodeToString(o.Bytes(oracle.Addr{Seg: r.SS, Off: savedPtr}, 4))
				x["pp"] = int(savedPtr)
			}
			mport := -1
			if strings.Contains(sp.flags, "m") || strings.Contains(sp.flags, "e") {
				if isOut && len(portStacks[base]) > 0 {
					ps := portStacks[base]
					mport = ps[len(ps)-1]
					portStacks[base] = ps[:len(ps)-1]
				} else {
					mport = int(o.Byte(b.DS(fp + 0x1C)))
					if isIn {
						portStacks[base] = append(portStacks[base], mport)
					}
				}
			}
			if strings.Contains(sp.flags, "m") {
				port := mport
				x["mport"] = port
				if port < 100 {
					x["mod"] = hex.EncodeToString(o.Bytes(b.DS(uint16(0x6A80+37*port+0x0E)), 10))
				}
			}
			if strings.Contains(sp.flags, "n") && !strings.Contains(sp.flags, "T") {
				x["nation"] = hex.EncodeToString(o.Bytes(b.DS(0x117A), 32*7))
				x["hero_p"] = hex.EncodeToString(o.Bytes(b.DS(u16(o, b, 0xC2CE)), personSize))
			}
			if strings.Contains(sp.flags, "e") {
				port := mport
				if port < 100 {
					pe := o.Bytes(b.DS(uint16(0x6A80+37*port)), 37)
					x["pe"] = hex.EncodeToString(pe)
					if z := int(pe[0x21]); z < 13 {
						x["zone"] = hex.EncodeToString(o.Bytes(b.DS(uint16(0x78F4+0x80*z)), 0x80))
					}
				}
			}
			e.log.Emit(Ev82{Kind: "f", Label: sp.label, Step: o.Steps(), Tick: o.Ticks(), Extra: x})
		})
	}
}

// fleetTab 取 70 艦隊各 10 bytes（x、y、+0x1B、+0x1C、+0x27、+0x28、+0x29、+0x2A）與 120 人物各 6 bytes。
func fleetTab(o *oracle.Oracle, b Bind) (string, string) {
	var fb strings.Builder
	for i := 0; i < nFleets; i++ {
		base := uint16(fleetBase + fleetSize*i)
		d := o.Bytes(b.DS(base), fleetSize)
		fb.WriteString(hex.EncodeToString(d[0:4]))
		for _, k := range []int{0x1B, 0x1C, 0x27, 0x28, 0x29, 0x2A} {
			fmt.Fprintf(&fb, "%02x", d[k])
		}
	}
	var pb strings.Builder
	for i := 0; i < nPersons; i++ {
		d := o.Bytes(b.DS(uint16(personBase+personSize*i)), personSize)
		for _, k := range []int{0x1B, 0x2C, 0x2D, 0x2E, 0x2F, 0x31} {
			fmt.Fprintf(&pb, "%02x", d[k])
		}
	}
	return fb.String(), pb.String()
}
