package main

// 批次 089：NPC 視窗外的跳躍 2A1E0（060 第 6.5 節）。
//   jcase:名稱=Fdx;Fdy;Pdx;Pdy+名稱=…（數字用分號隔開，因為 -do 以逗號分動作）   2A1E0 的第 n 次進入（去重）把艦隊 F 的座標與航點 p 改成案例 n（相對視窗原點 L、T），之後原版程式照跑。
//                                         改寫是測試條件（輸入），每次都記 poke 事件；AX = p 的位址、BX = F 的位址。超過案例數的進入不改，照常記錄。
// 觀測（每次 2A1E0 進入到返回之間）：j_in（F 的 133 bytes、p、視窗變數）、4135C 的 (x, y) 與返回（land）、33164 之前的 pos_set、341C2 的進入（plan）、j_out（F 的 133 bytes）。
// 4135C 的返回值是模型的輸入（地形是原版的資料，模型逐次比對呼叫序列，不另存地圖）。

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/oracle"
)

type jcase089 struct {
	name                 string
	fdx, fdy, pdx, pdy int
}

// 092：jonly:L:T 設定後，視窗原點不是 (L, T) 的 2A1E0 進入一律不記錄、不改（傳送後視窗重載前的進入會用到舊視窗）。
var jOnlyL, jOnlyT = -1, -1

func (e *env) installJump089(spec string) error {
	o := e.o
	b, _ := CurrentOrMain(o)
	var cases []jcase089
	if spec != "" {
		for _, c := range strings.Split(spec, "+") {
			name, v, ok := strings.Cut(c, "=")
			if !ok {
				return fmt.Errorf("jcase 格式 名稱=Fdx;Fdy;Pdx;Pdy")
			}
			p := strings.Split(v, ";")
			if len(p) != 4 {
				return fmt.Errorf("jcase 要 4 個數字")
			}
			var n [4]int
			for i := range n {
				x, err := strconv.Atoi(p[i])
				if err != nil {
					return err
				}
				n[i] = x
			}
			cases = append(cases, jcase089{name, n[0], n[1], n[2], n[3]})
		}
	}
	d := &dedup{}
	count := 0
	active := false
	recording := false
	var fptr, pptr uint16
	var landX, landY uint16
	var landPending bool
	window := func(o *oracle.Oracle) (int, int) {
		L := 720*int(u8(o, b, 0x06CA)) + int(u16(o, b, 0xBDA6))
		T := int(u16(o, b, 0xBDAA))
		return L, T
	}
	o.OnCall(b.Code(0x2A1E0), func(o *oracle.Oracle) {
		if d.again(o) {
			return
		}
		r := o.Regs()
		pptr, fptr = r.AX, r.BX
		L, T := window(o)
		if jOnlyL >= 0 && (L != jOnlyL || T != jOnlyT) {
			return
		}
		name := ""
		if count < len(cases) {
			c := cases[count]
			name = c.name
			oldF := o.Bytes(b.DS(fptr), 4)
			oldP := o.Bytes(b.DS(pptr), 4)
			fx, fy, px, py := L+c.fdx, T+c.fdy, L+c.pdx, T+c.pdy
			o.SetWord(b.DS(fptr), uint16(fx))
			o.SetWord(b.DS(fptr+2), uint16(fy))
			o.SetWord(b.DS(pptr), uint16(px))
			o.SetWord(b.DS(pptr+2), uint16(py))
			e.log.Emit(Ev78{Kind: "poke", Label: "jcase:" + c.name, Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{
				"n": count, "F_old": hex.EncodeToString(oldF), "p_old": hex.EncodeToString(oldP),
				"F_new": []int{fx, fy}, "p_new": []int{px, py}, "L": L, "T": T}})
		}
		x := map[string]any{"n": count, "case": name, "fptr": int(fptr), "pptr": int(pptr), "L": L, "T": T,
			"F": hx(o, b, fptr, 0x85), "p": hx(o, b, pptr, 4), "w06C8": int(u16(o, b, 0x06C8)), "w06CA": int(u8(o, b, 0x06CA)),
			"wBDA6": int(u16(o, b, 0xBDA6)), "wBDAA": int(u16(o, b, 0xBDAA)), "pf": int(u16(o, b, 0xC2D4)), "at_sea": int(u8(o, b, 0x0CAE))}
		e.log.Emit(Ev78{Kind: "b", Label: "j_in", IDA: "2A1E0", Step: o.Steps(), Tick: o.Ticks(), Regs: regMap78(o), Extra: x})
		count++
		active, recording = true, true
	})
	dl := &dedup{}
	o.OnCall(b.Code(0x4135C), func(o *oracle.Oracle) {
		if !active || !recording || dl.again(o) {
			return
		}
		r := o.Regs()
		landX, landY, landPending = r.AX, r.DX, true
	})
	o.OnCall(b.Code(0x413B0), func(o *oracle.Oracle) {
		if !active || !recording || !landPending {
			return
		}
		landPending = false
		e.log.Emit(Ev78{Kind: "b", Label: "land", IDA: "4135C", Step: o.Steps(), Tick: o.Ticks(),
			Extra: map[string]any{"x": int(landX), "y": int(landY), "ret": int(o.Regs().AX)}})
	})
	// 2A392：F.x、F.y 寫入之後、33164（到達判定）之前
	o.OnCall(b.Code(0x2A392), func(o *oracle.Oracle) {
		if !active || !recording {
			return
		}
		recording = false
		e.log.Emit(Ev78{Kind: "b", Label: "pos_set", IDA: "2A392", Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"F": hx(o, b, fptr, 0x85)}})
	})
	o.OnCall(b.Code(0x341C2), func(o *oracle.Oracle) {
		if !active {
			return
		}
		e.log.Emit(Ev78{Kind: "b", Label: "plan", IDA: "341C2", Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"bx": int(o.Regs().BX)}})
	})
	// 2A4A0：返回前（BP 仍有效）
	o.OnCall(b.Code(0x2A4A0), func(o *oracle.Oracle) {
		if !active {
			return
		}
		active = false
		e.log.Emit(Ev78{Kind: "b", Label: "j_out", IDA: "2A4A0", Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"F": hx(o, b, fptr, 0x85), "p": hx(o, b, pptr, 4)}})
	})
	return nil
}
