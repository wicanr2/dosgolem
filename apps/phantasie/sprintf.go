package phantasie

import "github.com/wicanr2/dosgolem/oracle"

// OffSprintf 是 `sprintf(dest, fmt, ...)` 的入口（映像偏移，近呼叫，C 慣例）：
// `[SP+2]` 目的緩衝區、`[SP+4]` 格式字串指標、其後是可變引數。
// 訊息類文字多半先以它組進緩衝區，再以緩衝區當格式字串交給繪字函式。
const OffSprintf = 0x3E35

// SprintfEvent 是一次 sprintf 呼叫（入口時的參數）。
type SprintfEvent struct {
	Step   uint64
	Caller uint16
	Dest   uint16
	FmtPtr uint16
	Fmt    []byte
	Args   []uint16 // 格式字串之後的 12 個堆疊字組
}

// CaptureSprintf 在 sprintf 入口掛唯讀 hook。
func CaptureSprintf(o *oracle.Oracle, img uint16, fn func(SprintfEvent)) {
	dg := img + DGroupParas
	o.OnCall(oracle.Far(img, OffSprintf), func(o *oracle.Oracle) {
		ev := SprintfEvent{
			Step:   o.Steps(),
			Caller: o.StackWord(0),
			Dest:   o.StackWord(1),
			FmtPtr: o.StackWord(2),
		}
		ev.Fmt = cstr(o, dg, ev.FmtPtr, 80)
		for i := 0; i < 12; i++ {
			ev.Args = append(ev.Args, o.StackWord(3+i))
		}
		fn(ev)
	})
}
