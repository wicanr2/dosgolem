package strikecommander

// 012-strike-3d-hd-rerasterize §3.2、§3.6：只讀擷取 R1–R6
// 與 R10 元素範圍；1 倍自檢由 hd3d 套件處理，不畫 N 倍畫面。

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d"
	"github.com/wicanr2/dosgolem/apps/strikecommander/realspace"
)

var rasterTriggerIPs = [...]uint16{0x000c, 0x0416, 0x083d, 0x0ea8, 0x0c79, 0x0048, 0x004a, 0x020c, 0x04a6, 0x0685}

type rasterFrameKey struct{ ss, sp uint16 }
type rasterContext struct {
	key     rasterFrameKey
	element hd3d.FarPtr
	method  hd3d.FarPtr
	unknown bool
	before  []byte
	start   int
}
type rasterHit struct {
	kind    string
	call    hd3d.Call
	desc    [6]uint16
	list    []uint16
	ctx     rasterContext
	after   []byte
	fault   string
	element RasterElement
}
type rasterRecorder struct {
	tables hd3d.Tables
	stack  []rasterContext
	blobs  map[[32]byte][]byte
}

// NewSceneRecorder 加上 012 的 1 倍呼叫錄製與自檢。tables 應由
// hd3d.TablesFromEXE 驗證玩家的原版 EXE 後取得。這不會啟用 HD 合成。
func NewSceneRecorder(lib *realspace.Library, tables hd3d.Tables) *ModelRecorder {
	r := NewModelRecorder(lib)
	tables.R6Table = append([]byte(nil), tables.R6Table...)
	r.raster = &rasterRecorder{tables: tables}
	return r
}

func (r *rasterRecorder) reset() { r.stack, r.blobs = nil, nil }

// 每次命中仍讀取現場 bytes；只在指令確認執行後合併相同複本。
// 快取跟隨群組清空，不跨玩家畫面累積。
func (r *rasterRecorder) intern(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	if r.blobs == nil {
		r.blobs = make(map[[32]byte][]byte)
	}
	hash := sha256.Sum256(b)
	if old, ok := r.blobs[hash]; ok && bytes.Equal(old, b) {
		return old
	}
	r.blobs[hash] = b
	return b
}

func (r *rasterRecorder) internCapture(c *hd3d.Capture) {
	for i := range c.Spans {
		c.Spans[i].Data = r.intern(c.Spans[i].Data)
	}
}

func (r *rasterRecorder) matches(v modelView, base uint16) bool {
	ip, cs := v.IP(), v.CS()
	switch ip {
	case 0x000c, 0x0416, 0x083d, 0x0ea8:
		return cs == base+0x1d5e
	case 0x0c79, 0x020c, 0x04a6, 0x0685:
		return cs == base+0x2198
	case 0x01ed:
		return cs == base+0x54b0
	case 0x0048, 0x004a:
		return cs == base+0x27f5
	}
	return false
}

func rasterKnownMethod(base uint16, p hd3d.FarPtr) bool {
	if p.Seg == base+0x314a && p.Off == 0x00af {
		return true
	}
	if p.Seg == base+0x2a57 && p.Off == 0x000c {
		return true
	}
	if p.Seg == base+0x2579 {
		switch p.Off {
		case 0x0a7b, 0x0e82, 0x12ac, 0x1503, 0x18aa, 0x1e4e, 0x213a:
			return true
		}
	}
	return false
}

func rasterBuffer(v modelView, g *ModelGroup) ([]byte, bool) {
	if g == nil || len(g.List) == 0 {
		return nil, false
	}
	b := make([]byte, 64000)
	return b, modelRead(v, g.Desc[0], g.List[0], b)
}

func (r *rasterRecorder) capture(v modelView, base uint16, g *ModelGroup) *rasterHit {
	h := &rasterHit{}
	h.call.Steps, h.call.DS, h.call.SS, h.call.SP = v.Steps(), v.DS(), v.SS(), v.SP()
	ok := true
	ip := v.IP()
	if ip == 0x020c || ip == 0x04a6 || ip == 0x0685 {
		h.kind = "variant"
		h.fault = fmt.Sprintf("R5 未移植的後段段函式 +2198:%04X", ip)
		return h
	}
	if ip == 0x0048 {
		h.kind = "entry"
		h.ctx.key = rasterFrameKey{v.SS(), v.SP()}
		p := modelFar(v, v.DS(), v.BX(), &ok)
		h.ctx.method = hd3d.FarPtr{Off: p.Off, Seg: p.Seg}
		p = modelFar(v, v.SS(), v.SP(), &ok)
		h.ctx.element = hd3d.FarPtr{Off: p.Off, Seg: p.Seg}
		h.element = RasterElement{Steps: v.Steps(), Pointer: h.ctx.element, Method: h.ctx.method}
		ok = modelRead(v, p.Seg, p.Off, h.element.Header[:]) && ok
		h.ctx.unknown = !rasterKnownMethod(base, h.ctx.method)
		if h.ctx.unknown {
			var read bool
			h.ctx.before, read = rasterBuffer(v, g)
			ok = ok && read
		}
		if !ok {
			h.fault = "R10 入口讀取失敗"
		}
		return h
	}
	if ip == 0x004a {
		h.kind = "return"
		h.ctx.key = rasterFrameKey{v.SS(), v.SP()}
		for i := len(r.stack) - 1; i >= 0; i-- {
			if r.stack[i].key == h.ctx.key {
				h.ctx = r.stack[i]
				if h.ctx.unknown {
					h.after, ok = rasterBuffer(v, g)
				}
				break
			}
		}
		if !ok {
			h.fault = "R10 返回讀取失敗"
		}
		return h
	}
	h.kind = "call"
	c := &h.call
	c.Steps, c.DS, c.SS, c.SP = v.Steps(), v.DS(), v.SS(), v.SP()
	nargs := 0
	switch ip {
	case 0x000c:
		c.Kind, nargs = hd3d.R1, 4
	case 0x0416:
		c.Kind, nargs = hd3d.R2, 6
	case 0x083d:
		c.Kind, nargs = hd3d.R3, 4
	case 0x0ea8:
		c.Kind, nargs = hd3d.R4, 9
	case 0x0c79:
		c.Kind, nargs = hd3d.R5, 13
	case 0x01ed:
		c.Kind, nargs = hd3d.R6, 5
	}
	// R1–R4 同段內的重入呼叫不重複擷取。
	if ip != 0x0c79 && ip != 0x01ed && modelWord(v, v.SS(), v.SP()+2, &ok) == base+0x1d5e {
		h.kind = "reentrant"
		return h
	}
	for i := 0; i < nargs; i++ {
		c.Params[i] = modelWord(v, v.SS(), v.SP()+4+uint16(i*2), &ok)
	}
	for i := range c.Desc {
		c.Desc[i] = modelWord(v, v.DS(), c.Params[0]+uint16(i*2), &ok)
	}
	h.desc = c.Desc
	if c.Desc[2] > c.Desc[4] || c.Desc[3] > c.Desc[5] || c.Desc[4] > 319 || c.Desc[5] > 199 {
		h.fault = "光柵化描述元越界"
		c.ReadFailed = true
		return h
	}
	if g != nil && len(g.List) == 0 {
		h.list = make([]uint16, int(c.Desc[5])+2)
		for i := range h.list {
			h.list[i] = modelWord(v, v.DS(), c.Desc[1]+uint16(i*2), &ok)
		}
	}
	if c.Kind == hd3d.R6 {
		var color [1]byte
		ok = modelRead(v, v.DS(), 0x38be, color[:]) && ok
		c.Aux.Color0 = color[0]
		ok = modelRead(v, v.DS(), 0x38c0, color[:]) && ok
		c.Aux.Color1 = color[0]
		c.Aux.HasFlags, c.Aux.Flags = true, v.Flags()
	} else if c.Kind == hd3d.R5 {
		c.Verts = make([]byte, 66)
		for i := 0; i < 3; i++ {
			ok = modelRead(v, c.Params[2], c.Params[1+2*i], c.Verts[i*22:(i+1)*22]) && ok
		}
		table := modelFar(v, v.DS(), 0x13da, &ok)
		c.Aux.TableOff = table.Off
		c.Aux.Table = make([]byte, 0xc00)
		ok = modelRead(v, table.Seg, table.Off, c.Aux.Table) && ok
		c.Aux.HasResid = true
		c.Aux.ResA5 = modelWord(v, v.SS(), v.SP()-2-0xa5, &ok)
		c.Aux.Res9F = modelWord(v, v.SS(), v.SP()-2-0x9f, &ok)
		c.Tex.Pixels, ok = rasterCaptureTexture(v, c.Params[8], c.Params[7], c.Params[9], c.Verts, 2, ok)
		if c.Params[11] != 0 || c.Params[12] != 0 {
			c.Tex.Shade, ok = rasterCaptureSpan(v, c.Params[12], c.Params[11], 256, ok)
		}
	} else {
		n := int(c.Params[1])
		c.Verts = make([]byte, n*22)
		ok = modelRead(v, c.Params[3], c.Params[2], c.Verts) && ok
		if c.Kind == hd3d.R2 {
			c.Table = make([]byte, 256)
			ok = modelRead(v, c.Params[5], c.Params[4], c.Table) && ok
		}
		if c.Kind == hd3d.R4 {
			c.Tex.Pixels, ok = rasterCaptureTexture(v, c.Params[5], c.Params[4], c.Params[6], c.Verts, 0, ok)
			if c.Params[7] != 0 || c.Params[8] != 0 {
				c.Tex.Shade, ok = rasterCaptureSpan(v, c.Params[8], c.Params[7], 256, ok)
			}
		}
	}
	c.ReadFailed = !ok
	if !ok {
		h.fault = "光柵化輸入讀取失敗"
	}
	return h
}

func rasterCaptureSpan(v modelView, seg, off uint16, n int, ok bool) (hd3d.Capture, bool) {
	b := make([]byte, n)
	ok = modelRead(v, seg, off, b) && ok
	return hd3d.Capture{Spans: []hd3d.Span{{Off: off, Data: b}}}, ok
}

func rasterCaptureTexture(v modelView, seg, off, width uint16, verts []byte, pad int, ok bool) (hd3d.Capture, bool) {
	var out hd3d.Capture
	if len(verts) < 22 || len(verts)%22 != 0 {
		return out, false
	}
	minU, minV, maxU, maxV := 256, 256, -1, -1
	for i := 0; i < len(verts); i += 22 {
		u := int(binary.LittleEndian.Uint16(verts[i+6:]) >> 8)
		w := int(binary.LittleEndian.Uint16(verts[i+8:]) >> 8)
		minU, minV, maxU, maxV = min(minU, u), min(minV, w), max(maxU, u), max(maxV, w)
	}
	minU, minV, maxU, maxV = minU-pad, minV-pad, maxU+pad, maxV+pad
	for y := minV; y <= maxV; y++ {
		rowOff := uint16(int(off) + y*int(width) + minU)
		b := make([]byte, maxU-minU+1)
		ok = modelRead(v, seg, rowOff, b) && ok
		out.Spans = append(out.Spans, hd3d.Span{Off: rowOff, Data: b})
	}
	return out, ok
}

func (r *rasterRecorder) append(g *ModelGroup, c hd3d.Call) {
	if len(g.Calls) >= 0xffff {
		g.RasterProblems = append(g.RasterProblems, "光柵化呼叫序號超界")
		return
	}
	c.Seq = uint16(len(g.Calls))
	c.Verts, c.Table, c.Aux.Table = r.intern(c.Verts), r.intern(c.Table), r.intern(c.Aux.Table)
	r.internCapture(&c.Tex.Pixels)
	r.internCapture(&c.Tex.Shade)
	if len(r.stack) > 0 {
		c.Element = r.stack[len(r.stack)-1].element
	}
	g.Calls = append(g.Calls, c)
}

func (r *rasterRecorder) apply(g *ModelGroup, h *rasterHit) {
	if h.fault != "" {
		g.RasterProblems = append(g.RasterProblems, h.fault)
	}
	switch h.kind {
	case "call":
		if len(g.List) == 0 {
			g.Desc = h.desc
			g.List = h.list
		}
		if g.Desc != h.call.Desc {
			g.RasterProblems = append(g.RasterProblems, "光柵化描述元不一致")
		}
		r.append(g, h.call)
	case "entry":
		g.Elements = append(g.Elements, h.element)
		h.ctx.start = len(g.Calls)
		r.stack = append(r.stack, h.ctx)
	case "return":
		idx := -1
		for i := len(r.stack) - 1; i >= 0; i-- {
			if r.stack[i].key == h.ctx.key {
				idx = i
				break
			}
		}
		if idx < 0 {
			g.RasterProblems = append(g.RasterProblems, "R10 返回沒有配對")
			return
		}
		ctx := r.stack[idx]
		if idx != len(r.stack)-1 {
			g.RasterProblems = append(g.RasterProblems, "R10 返回跳過內層框")
		}
		if ctx.unknown {
			patch, diag := hd3d.MakePatch(ctx.before, h.after, g.Calls[ctx.start:], g.List, r.tables)
			if len(diag.Faults) != 0 {
				g.RasterProblems = append(g.RasterProblems, diag.Faults...)
			}
			call := h.call
			call.Kind, call.Desc, call.Patch, call.Element = hd3d.BlackBox, g.Desc, patch, ctx.element
			r.append(g, call)
		}
		r.stack = r.stack[:idx]
	}
}

func (r *rasterRecorder) check(g *ModelGroup) {
	if g.ReadFailed {
		g.RasterProblems = append(g.RasterProblems, "群組必要輸入讀取失敗")
	}
	if len(r.stack) != 0 {
		g.RasterProblems = append(g.RasterProblems, "封存時R10框未關閉")
	}
	result := hd3d.Replay(hd3d.Group{Key: g.Key, Calls: g.Calls, List: g.List, Buf3D: g.Buf3D}, r.tables)
	if len(g.RasterProblems) != 0 {
		result.Valid, result.Repaintable = false, false
		result.Diagnostics.Faults = append(result.Diagnostics.Faults, g.RasterProblems...)
	}
	g.Raster = &result
	g.ReplayReady = result.Repaintable
}
