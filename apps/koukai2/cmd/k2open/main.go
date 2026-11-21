// Command k2open 冷啟動《大航海時代 II》（KOUKAI2.COM），不送任何輸入，讓片頭
// （OPEN.EXE）自然播完，直到 MAIN.EXE 被載入為止。每個 FMDRV 計時單位擷取一次畫面，
// 並在 OPEN.EXE 的場景函式與等待、調色盤函式入口記錄呼叫，輸出片頭的對拍 oracle。
//
// 計時單位（koukai2 `docs/spec/063-opening.md` 的「時間單位」）：
// FMDRV.COM 的 IRQ0 處理（IDA 10200..10212）每個 IRQ0 讓 byte_10123 加 0x35，進位時
// 讓倒數器 word_10126 減 1（不為 0 才減）。本工具在進位後的位址 10207 掛 hook，
// 每觸發一次就是一個計時單位，不論倒數器是否為 0。
//
// 輸出（-out 目錄）：
//
//	frames.tsv    每個計時單位一行：序號、IRQ0 次數、步數、畫面雜湊、是否與上一格不同、是否新圖
//	calls.tsv     場景與等待、調色盤函式入口的呼叫紀錄（暫存器與堆疊前 4 個字）
//	png/<雜湊>.png  每個沒見過的畫面一張（雜湊是寬高加 ScreenRGB 的 sha256）
//	timeline.md   各場景起訖與等待呼叫次數
//	video.ffconcat  給 ffmpeg concat 的圖序列（只含畫面變化的格）
//
// 所有時間都是指令數與 IRQ0 次數，同一份輸入跑兩次得到同一個結果。
// ⚠ 本專案不含任何原版檔案，-root 由玩家自備。
package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/wicanr2/dosgolem/apps/koukai2"
	"github.com/wicanr2/dosgolem/oracle"
)

// FMDRV.COM 的 IDA 線性位址（koukai2 `re/FMDRV.COM/listing.asm`）。
const (
	fmCarry   uint32 = 0x10207 // byte_10123 進位後的第一道指令（jnb 沒跳才會走到）
	fmDecr    uint32 = 0x1020E // dec word_10126
	fmAcc     uint32 = 0x10123 // byte_10123：每個 IRQ0 加 0x35
	fmCounter uint32 = 0x10126 // word_10126：倒數器，INT 66h AX=F100h 讀、F101h 寫
)

type hookDef struct {
	addr uint32
	name string
	role string // scene、wait、pal、end
}

// 片頭 12 個場景（063「場景」）與收尾。
var sceneHooks = []hookDef{
	{0x15348, "scene01", "scene"}, {0x155CE, "scene02", "scene"}, {0x15E7A, "scene03", "scene"},
	{0x1614E, "scene04", "scene"}, {0x165AF, "scene05", "scene"}, {0x16A58, "scene06", "scene"},
	{0x16E0C, "scene07", "scene"}, {0x17240, "scene08", "scene"}, {0x17681, "scene09", "scene"},
	{0x17A7E, "scene10", "scene"}, {0x17CB0, "scene11", "scene"}, {0x18204, "scene12", "scene"},
	{0x18747, "shutdown", "end"},
}

// 等待與調色盤函式。凡是呼叫 sub_1186D（INT 66h F100h 讀）或 sub_11860（F101h 寫）的函式，
// 以及它們的上層（淡入淡出、揭示）都列在這裡，名稱用 IDA 的 sub_ 名，語意見 063。
var timerHooks = []hookDef{
	{0x14836, "sub_14836", "wait"}, // 計時器歸零（F101h = 0）
	{0x14846, "sub_14846", "wait"}, // 計時器 += AX，不等待（arm）
	{0x1485D, "sub_1485D", "wait"}, // arm 後等到倒數器歸零
	{0x14871, "sub_14871", "wait"}, // AX * 6 後 sub_1485D
	{0x14883, "sub_14883", "wait"}, // byte_1A0F3 * AX * 2 後 sub_14871
	{0x14899, "sub_14899", "wait"}, // 設計時器，等鍵或歸零
	{0x14956, "sub_14956", "wait"}, // 音樂旗標包住的 sub_1485D
	{0x14F0D, "sub_14F0D", "wait"}, // 可跳過的等待（Enter、Space、滑鼠）
	{0x14F31, "sub_14F31", "wait"},
	{0x14F6D, "sub_14F6D", "wait"},
	{0x14FB8, "sub_14FB8", "wait"},
	{0x14FD5, "sub_14FD5", "wait"}, // wait_esc（只有 Esc 能跳過）
	{0x1500A, "sub_1500A", "wait"},
	{0x14D0D, "sub_14D0D", "pal"}, // 調色盤漸變一階
	{0x14DBC, "sub_14DBC", "pal"},
	{0x14DD9, "sub_14DD9", "pal"},
	{0x14DE2, "sub_14DE2", "pal"},
	{0x14DEB, "sub_14DEB", "pal"},
	{0x14EE4, "sub_14EE4", "pal"}, // 跳過鍵處理：音樂淡出、漸變到黑、exit
	{0x150A2, "sub_150A2", "pal"},
	{0x150CD, "sub_150CD", "pal"},
	{0x150E4, "sub_150E4", "pal"},
	{0x150F2, "sub_150F2", "pal"}, // 揭示（內含 arm 8）
	{0x151A9, "sub_151A9", "pal"}, // 揭示
	{0x1532C, "sub_1532C", "pal"}, // 漸變到黑並清畫面
}

// 等待呼叫次數只算會停下來等的入口。sub_14846 是 arm（被上面各函式與場景直接呼叫），
// 另列一欄；sub_14871、sub_14883 是 sub_1485D 的包裝，不重複算。
var waitEntries = []uint32{0x1485D, 0x14899, 0x14F0D, 0x14F31, 0x14F6D, 0x14FD5, 0x1500A, 0x14FB8}

type callRec struct {
	seq    int
	scene  int
	unit   uint64
	irq0   uint64
	steps  uint64
	def    hookDef
	regs   oracle.Regs
	st     [4]uint16
	remain uint16
	dup    bool
}

type frameRec struct {
	unit   uint64
	irq0   uint64
	steps  uint64
	hash   string
	chg    bool
	isNew  bool
	remain uint16
	acc    uint8
	mode   uint8
	scene  int
	kind   string
}

type app struct {
	o      *oracle.Oracle
	open   koukai2.Bind
	fm     koukai2.Bind
	outDir string

	unit     uint64
	scene    int
	lastHash string
	seen     map[string]bool
	frames   []frameRec
	calls    []callRec
	lastCall map[uint32]callRec
	dupCount int
	newPNG   int
	verifyN  int
	verified int
	badVer   int
	pngBytes int64
	t0       time.Time

	framesW *bufio.Writer
	callsW  *bufio.Writer
}

func main() {
	root := flag.String("root", "/orig", "原版素材目錄")
	out := flag.String("out", "/out/opening", "輸出目錄")
	maxUnits := flag.Uint64("max-units", 0, "測試用：跑滿這麼多計時單位就停（0 = 播到 MAIN.EXE 載入）")
	budget := flag.Uint64("budget", 3_000_000_000, "指令數上限")
	verify := flag.Int("verify", 20, "每幾張新 PNG 讀回來比對一次雜湊（0 = 不比）")
	head := flag.String("head", "", "dosgolem Git HEAD（寫進 timeline.md）")
	cmdline := flag.String("cmdline", "", "完整執行指令（寫進 timeline.md）")
	flag.Parse()

	o, err := koukai2.Load(*root)
	if err != nil {
		die(err)
	}
	defer o.Close()

	a := &app{o: o, outDir: *out, seen: map[string]bool{}, lastCall: map[uint32]callRec{},
		verifyN: *verify, t0: time.Now()}
	if err := os.MkdirAll(filepath.Join(*out, "png"), 0o755); err != nil {
		die(err)
	}
	ff, err := os.Create(filepath.Join(*out, "frames.tsv"))
	if err != nil {
		die(err)
	}
	defer ff.Close()
	cf, err := os.Create(filepath.Join(*out, "calls.tsv"))
	if err != nil {
		die(err)
	}
	defer cf.Close()
	a.framesW, a.callsW = bufio.NewWriter(ff), bufio.NewWriter(cf)
	fmt.Fprintln(a.framesW, "unit\tirq0\tsteps\thash\tchanged\tnew_png\tremain_w10126\tacc_b10123\tvmode\tscene\tkind")
	fmt.Fprintln(a.callsW, "seq\tscene\tunit\tirq0\tsteps\taddr\tname\trole\tdup\tAX\tBX\tCX\tDX\tSI\tDI\tBP\tSP\tFLAGS\tstk0\tstk1\tstk2\tstk3\tremain_w10126")

	// 等 OPEN.EXE 載入才掛 hook，避免踩到 KOUKAI2.COM 或 FMDRV.COM。
	// 每 256 道才查 EXEC 紀錄，查一次要複製整個紀錄，每道都查太貴。
	if err := o.RunUntil(oracle.NewCond("OPEN.EXE 載入", func(o *oracle.Oracle) bool {
		if o.Steps()&0xFF != 0 {
			return false
		}
		b, ok := koukai2.Current(o)
		return ok && b.Name == "OPEN.EXE"
	}), oracle.Budget(*budget)); err != nil {
		die(err)
	}
	a.open, _ = koukai2.Current(o)
	for _, r := range o.ExecLog() {
		if strings.EqualFold(r.Base, "FMDRV.COM") && r.TSR {
			a.fm = koukai2.Bind{Name: "FMDRV.COM", PSP: r.PSP - 0x10, Running: true}
		}
	}
	if a.fm.Name == "" {
		die(fmt.Errorf("EXEC 紀錄沒有常駐的 FMDRV.COM"))
	}
	a.proof(*root)

	// 計時單位 hook：進位後第一道指令。IRQ 處理執行時 IF = 0，不會重複觸發。
	o.OnCall(a.fm.Code(fmCarry), func(o *oracle.Oracle) {
		if !a.inOpen(o) {
			return
		}
		a.unit++
		a.capture(o, "unit")
	})
	// 場景與等待、調色盤函式入口。
	for _, h := range append(append([]hookDef{}, sceneHooks...), timerHooks...) {
		h := h
		o.OnCall(a.open.Code(h.addr), func(o *oracle.Oracle) { a.onCall(o, h) })
	}

	a.capture(o, "start")
	stop := oracle.NewCond("MAIN.EXE 載入", func(o *oracle.Oracle) bool {
		if *maxUnits > 0 && a.unit >= *maxUnits {
			return true
		}
		if o.Steps()&0xFF != 0 {
			return false
		}
		return koukai2.IsMain(o)
	})
	runErr := o.RunUntil(stop, oracle.Budget(*budget))
	if runErr != nil {
		fmt.Printf("執行停止：%v\n", runErr)
	}
	a.capture(o, "end")
	a.framesW.Flush()
	a.callsW.Flush()

	hz := o.TimerHz()
	fmt.Printf("結束 步數 %d IRQ0 %d 單位 %d 新圖 %d 呼叫 %d（重複觸發 %d） 位於 %s\n",
		o.Steps(), o.Ticks(), a.unit, a.newPNG, len(a.calls), a.dupCount, koukai2.Describe(o))
	fmt.Printf("PNG 讀回比對 %d 張，不符 %d 張；PNG 共 %d bytes；牆鐘 %s\n",
		a.verified, a.badVer, a.pngBytes, time.Since(a.t0).Round(time.Second))
	for _, r := range o.ExecLog() {
		fmt.Printf("  EXEC %-10s PSP=%04X exit=%d ended=%v tsr=%v\n", r.Base, r.PSP, r.Exit, r.Ended, r.TSR)
	}
	if u := o.Unimplemented(); len(u) > 0 {
		fmt.Printf("未實作：%v\n", u)
	}
	if m := o.Missing(); len(m) > 0 {
		fmt.Printf("缺檔：%v\n", m)
	}
	if err := a.writeConcat(hz); err != nil {
		die(err)
	}
	if err := a.writeTimeline(*root, *head, *cmdline, hz, runErr); err != nil {
		die(err)
	}
	if runErr != nil && *maxUnits == 0 {
		os.Exit(1)
	}
}

// inOpen 回目前最後一支未結束的程式是不是 OPEN.EXE。OPEN.EXE 結束、MAIN.EXE 載入之後，
// 同一個線性位址是 MAIN.EXE 的程式碼，hook 不能再記。
func (a *app) inOpen(o *oracle.Oracle) bool {
	b, ok := koukai2.Current(o)
	return ok && b.Name == "OPEN.EXE"
}

// proof 印出計時變數的實際位址與位元組，作為「這個變數就是計時器」的證據。
func (a *app) proof(root string) {
	o := a.o
	fmt.Printf("FMDRV.COM PSP=%04X（COM 檔：IDA 段 1000 = PSP 段，IDA 10100 是檔案第 0 byte，IDA 線性 L 對應執行期 %04X:(L-10000)）\n", a.fm.PSP+0x10, a.fm.PSP+0x10)
	for _, p := range []struct {
		lin  uint32
		n    int
		note string
		want []byte
	}{
		{0x10200, 14, "IRQ0 處理：add byte_10123,35h; jnb; cmp word_10126,0; jz; dec word_10126", []byte{0x80, 0x06, 0x23, 0x01, 0x35, 0x73, 0x0B, 0x83, 0x3E, 0x26, 0x01, 0x00, 0x74, 0x04}},
		{0x10212, 4, "dec byte_10118+9（下一段）", []byte{0xFE, 0x0E, 0x21, 0x01}},
		{0x10170, 5, "INT 66h AX=F100h：mov bx,cs:word_10126", []byte{0x2E, 0x8B, 0x1E, 0x26, 0x01}},
		{0x10178, 5, "INT 66h AX=F101h：mov cs:word_10126,bx", []byte{0x2E, 0x89, 0x1E, 0x26, 0x01}},
		{0x1017D, 6, "F101h 同時 mov cs:byte_10123,0", []byte{0x2E, 0xC6, 0x06, 0x23, 0x01, 0x00}},
		{fmAcc, 1, "byte_10123 目前值", nil},
		{fmCounter, 2, "word_10126 目前值", nil},
	} {
		ad := a.fm.Code(p.lin)
		got := o.Bytes(ad, p.n)
		fmt.Printf("  FMDRV %05X 執行期線性 %05X bytes % X  %s\n", p.lin, ad.Linear(), got, p.note)
		if p.want != nil && !bytes.Equal(got, p.want) {
			die(fmt.Errorf("FMDRV %05X bytes 與 listing 不符：% X，預期 % X", p.lin, got, p.want))
		}
	}
	for _, h := range []hookDef{{0x14846, "", ""}, {0x1485D, "", ""}, {0x14FD5, "", ""}, {0x1186D, "", ""}, {0x11860, "", ""}} {
		ad := a.open.Code(h.addr)
		fmt.Printf("  OPEN  %05X 執行期線性 %05X bytes % X\n", h.addr, ad.Linear(), o.Bytes(ad, 16))
	}
	fmt.Printf("OPEN.EXE PSP=%04X 載入段 %04X 步數 %d IRQ0 %d TimerHz %.4f 除數 %d\n",
		a.open.PSP, a.open.PSP+0x10, o.Steps(), o.Ticks(), o.TimerHz(), o.TimerDivisor())
	_ = root
}

func (a *app) onCall(o *oracle.Oracle, h hookDef) {
	if !a.inOpen(o) {
		return
	}
	r := o.Regs()
	rec := callRec{
		seq: len(a.calls), scene: a.scene, unit: a.unit, irq0: o.Ticks(), steps: o.Steps(), def: h, regs: r,
		st:     [4]uint16{o.StackWord(0), o.StackWord(1), o.StackWord(2), o.StackWord(3)},
		remain: o.Word(a.fm.Code(fmCounter)),
	}
	// OnCall 在中斷返回時會重複觸發（dosgolem 規格 443 §4）：同一位址、暫存器與堆疊全同，
	// 中間隔著剛好一次以上的 IRQ0 與一小段指令數。標記為 dup，統計時排除。
	if l, ok := a.lastCall[h.addr]; ok && l.regs == r && l.st == rec.st &&
		rec.irq0 > l.irq0 && rec.steps-l.steps <= 2000 {
		rec.dup = true
		a.dupCount++
	}
	if !rec.dup {
		switch h.role {
		case "scene":
			a.scene = sceneIndex(h.addr)
			rec.scene = a.scene
		case "end":
			a.scene = 13
			rec.scene = 13
		}
	}
	a.calls = append(a.calls, rec)
	a.lastCall[h.addr] = rec
	dup := 0
	if rec.dup {
		dup = 1
	}
	fmt.Fprintf(a.callsW, "%d\t%d\t%d\t%d\t%d\t%05X\t%s\t%s\t%d\t%04X\t%04X\t%04X\t%04X\t%04X\t%04X\t%04X\t%04X\t%04X\t%04X\t%04X\t%04X\t%04X\t%d\n",
		rec.seq, rec.scene, rec.unit, rec.irq0, rec.steps, h.addr, h.name, h.role, dup,
		r.AX, r.BX, r.CX, r.DX, r.SI, r.DI, r.BP, r.SP, r.Flags,
		rec.st[0], rec.st[1], rec.st[2], rec.st[3], rec.remain)
}

func sceneIndex(addr uint32) int {
	for i, h := range sceneHooks {
		if h.addr == addr && h.role == "scene" {
			return i + 1
		}
	}
	return 0
}

// frameHash 是畫面的身分：寬、高加 RGB。
func frameHash(w, h int, rgb []uint8) string {
	s := sha256.New()
	fmt.Fprintf(s, "%dx%d:", w, h)
	s.Write(rgb)
	return hex.EncodeToString(s.Sum(nil))
}

func (a *app) capture(o *oracle.Oracle, kind string) {
	w, h, rgb := o.ScreenRGB()
	hs := frameHash(w, h, rgb)
	fr := frameRec{
		unit: a.unit, irq0: o.Ticks(), steps: o.Steps(), hash: hs, chg: hs != a.lastHash,
		remain: o.Word(a.fm.Code(fmCounter)), acc: o.Byte(a.fm.Code(fmAcc)),
		mode: o.VideoMode(), scene: a.scene, kind: kind,
	}
	if !a.seen[hs] {
		a.seen[hs] = true
		fr.isNew = true
		a.newPNG++
		path := filepath.Join(a.outDir, "png", hs+".png")
		if err := o.WritePNG(path); err != nil {
			die(err)
		}
		if fi, err := os.Stat(path); err == nil {
			a.pngBytes += fi.Size()
		}
		if a.verifyN > 0 && (a.newPNG == 1 || a.newPNG%a.verifyN == 0) {
			a.verifyPNG(path, hs)
		}
	}
	a.lastHash = hs
	a.frames = append(a.frames, fr)
	fmt.Fprintf(a.framesW, "%d\t%d\t%d\t%s\t%d\t%d\t%d\t%d\t%02X\t%d\t%s\n",
		fr.unit, fr.irq0, fr.steps, fr.hash, b2i(fr.chg), b2i(fr.isNew), fr.remain, fr.acc, fr.mode, fr.scene, fr.kind)
	if kind == "unit" && a.unit%500 == 0 {
		a.framesW.Flush()
		a.callsW.Flush()
		fmt.Printf("進度 單位 %d IRQ0 %d 步數 %d 場景 %d 新圖 %d 牆鐘 %s\n",
			a.unit, fr.irq0, fr.steps, a.scene, a.newPNG, time.Since(a.t0).Round(time.Second))
	}
}

// verifyPNG 把 PNG 讀回來，用同一個雜湊函式算，和擷取當下的 ScreenRGB 雜湊比對。
func (a *app) verifyPNG(path, want string) {
	a.verified++
	f, err := os.Open(path)
	if err != nil {
		a.badVer++
		fmt.Printf("驗證失敗：%v\n", err)
		return
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		a.badVer++
		fmt.Printf("驗證失敗：%s 解不開：%v\n", path, err)
		return
	}
	b := img.Bounds()
	rgb := make([]uint8, 0, b.Dx()*b.Dy()*3)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			rgb = append(rgb, uint8(r>>8), uint8(g>>8), uint8(bl>>8))
		}
	}
	if got := frameHash(b.Dx(), b.Dy(), rgb); got != want {
		a.badVer++
		fmt.Printf("驗證不符：%s 讀回 %s\n", path, got)
	}
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// writeConcat 寫 ffmpeg concat 清單：只含畫面變化的格，每格顯示到下一次變化為止。
// 時間軸用 IRQ0 次數 / TimerHz（模擬的壁鐘，與 docs/playtest/001-boot.md 同一基準）。
func (a *app) writeConcat(hz float64) error {
	var sb strings.Builder
	sb.WriteString("ffconcat version 1.0\n")
	type pt struct {
		t    float64
		hash string
	}
	var pts []pt
	for _, f := range a.frames {
		if f.chg || len(pts) == 0 {
			pts = append(pts, pt{float64(f.irq0) / hz, f.hash})
		}
	}
	endT := float64(a.frames[len(a.frames)-1].irq0) / hz
	for i, p := range pts {
		next := endT
		if i+1 < len(pts) {
			next = pts[i+1].t
		}
		d := next - p.t
		if d < 0.001 {
			d = 0.001
		}
		fmt.Fprintf(&sb, "file 'png/%s.png'\nduration %.6f\n", p.hash, d)
	}
	fmt.Fprintf(&sb, "file 'png/%s.png'\n", pts[len(pts)-1].hash)
	return os.WriteFile(filepath.Join(a.outDir, "video.ffconcat"), []byte(sb.String()), 0o644)
}

type seg struct {
	name                      string
	fn                        string
	u0, u1, i0, i1, s0, s1    uint64
	counts                    map[uint32]int
	arm                       int
	waits                     int
	armUnits                  uint64 // sub_14846 的 AX 總和：呼叫端排程的等待單位
	changes, uniqueNew, calls int
}

func (a *app) writeTimeline(root, head, cmdline string, hz float64, runErr error) error {
	last := a.frames[len(a.frames)-1]
	// 邊界：OPEN 載入（第一格）、各場景第一次非重複呼叫、shutdown、結束。
	type bound struct {
		name, fn     string
		u, i, s      uint64
		sceneNo      int
		haveBoundary bool
	}
	first := a.frames[0]
	var bs []bound
	bs = append(bs, bound{"啟動（OPEN.EXE 載入到場景 1）", "_main 185AA", first.unit, first.irq0, first.steps, 0, true})
	seenScene := map[int]bool{}
	for _, c := range a.calls {
		if c.dup {
			continue
		}
		switch c.def.role {
		case "scene":
			n := sceneIndex(c.def.addr)
			if !seenScene[n] {
				seenScene[n] = true
				bs = append(bs, bound{fmt.Sprintf("場景 %d", n), fmt.Sprintf("%05X", c.def.addr), c.unit, c.irq0, c.steps, n, true})
			}
		case "end":
			if !seenScene[13] {
				seenScene[13] = true
				bs = append(bs, bound{"收尾（sub_18747 到 MAIN.EXE 載入）", "18747", c.unit, c.irq0, c.steps, 13, true})
			}
		}
	}
	segs := make([]*seg, len(bs))
	for k, b := range bs {
		s := &seg{name: b.name, fn: b.fn, u0: b.u, i0: b.i, s0: b.s, counts: map[uint32]int{}, armUnits: 0}
		if k+1 < len(bs) {
			s.u1, s.i1, s.s1 = bs[k+1].u, bs[k+1].i, bs[k+1].s
		} else {
			s.u1, s.i1, s.s1 = last.unit, last.irq0, last.steps
		}
		segs[k] = s
	}
	segOf := func(steps uint64) *seg {
		var cur *seg
		for _, s := range segs {
			if steps >= s.s0 {
				cur = s
			}
		}
		return cur
	}
	isWait := map[uint32]bool{}
	for _, w := range waitEntries {
		isWait[w] = true
	}
	for _, c := range a.calls {
		if c.dup {
			continue
		}
		s := segOf(c.steps)
		if s == nil {
			continue
		}
		s.calls++
		s.counts[c.def.addr]++
		if c.def.addr == 0x14846 {
			s.arm++
			s.armUnits += uint64(c.regs.AX)
		}
		if isWait[c.def.addr] {
			s.waits++
		}
	}
	for _, f := range a.frames {
		s := segOf(f.steps)
		if s == nil {
			continue
		}
		if f.chg && f.kind == "unit" {
			s.changes++
		}
		if f.isNew {
			s.uniqueNew++
		}
	}

	sum := func(p string) string {
		b, err := os.ReadFile(filepath.Join(root, p))
		if err != nil {
			return "讀不到 " + p
		}
		return fmt.Sprintf("%x", sha256.Sum256(b))
	}
	var sb strings.Builder
	w := func(f string, args ...any) { fmt.Fprintf(&sb, f, args...) }
	w("# 片頭計時線（dosgolem 冷啟動，不送輸入）\n\n")
	w("## 輸入與環境\n\n")
	w("| 項目 | 值 |\n|---|---|\n")
	w("| dosgolem HEAD | `%s` |\n", head)
	w("| `OPEN.EXE` SHA-256 | `%s` |\n", sum("OPEN.EXE"))
	w("| `KOUKAI2.COM` SHA-256 | `%s` |\n", sum("KOUKAI2.COM"))
	w("| `FMDRV.COM` SHA-256 | `%s` |\n", sum("FMDRV.COM"))
	w("| 執行指令 | `%s` |\n", cmdline)
	w("| TimerHz、除數 | %.4f Hz、%d |\n", hz, a.o.TimerDivisor())
	w("| FMDRV.COM PSP | `%04X`（COM：IDA 段 `1000` 對應執行期段 `%04X`，IDA `10100` 是檔案第 0 byte） |\n", a.fm.PSP+0x10, a.fm.PSP+0x10)
	w("| OPEN.EXE PSP | `%04X`（IDA `10000` 對應執行期段 `%04X`） |\n", a.open.PSP, a.open.PSP+0x10)
	st := "MAIN.EXE 載入（步數粒度 256）"
	if runErr != nil {
		st = fmt.Sprintf("未完成：%v", runErr)
	}
	w("| 結束條件 | %s |\n\n", st)

	w("## 計時單位\n\n")
	w("- 計數點：`FMDRV.COM` IDA `10207`（`add byte_10123, 35h` 進位後的第一道指令），每觸發一次為 1 單位。單位 0 是 OPEN.EXE 載入的那一刻。\n")
	w("- 秒數欄：單位 / 60.3（063「時間單位」的名目頻率；精確值 %.3f Hz = TimerHz x 53 / 256），以及 IRQ0 次數 / TimerHz（模擬壁鐘）。\n", hz*53/256)
	w("- `INT 66h AX=F101h` 寫入計時器時會同時把 `byte_10123` 清為 0（`FMDRV.COM 1017D`），每次 arm 之後累加器從 0 重新開始，所以單位數與 IRQ0 的換算小於名目值。\n")
	w("- 全程 IRQ0 %d 次，單位 %d 個，單位 / IRQ0 = %.5f（名目 53/256 = %.5f）。\n", last.irq0-first.irq0, last.unit-first.unit,
		float64(last.unit-first.unit)/float64(last.irq0-first.irq0), 53.0/256.0)
	hist := map[uint64]int{}
	for i := 1; i < len(a.frames); i++ {
		if a.frames[i].kind == "unit" {
			hist[a.frames[i].irq0-a.frames[i-1].irq0]++
		}
	}
	var hk []uint64
	for k := range hist {
		hk = append(hk, k)
	}
	sort.Slice(hk, func(i, j int) bool { return hk[i] < hk[j] })
	w("- 相鄰單位的 IRQ0 間隔分布：")
	for i, k := range hk {
		if i > 0 {
			w("、")
		}
		w("%d 個 IRQ0 x %d 次", k, hist[k])
	}
	w("。\n\n")

	w("## 場景起訖\n\n")
	w("| 區段 | 入口 | 起（單位） | 迄（單位） | 單位數 | 秒（單位 / 60.3） | 起（IRQ0） | 迄（IRQ0） | 秒（IRQ0 / %.2f） | 起（步數） | 等待呼叫 | arm（14846）次數 | arm 排程單位（ΣAX） | 畫面變化格 | 新圖 |\n", hz)
	w("|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|\n")
	var totU, totI uint64
	for _, s := range segs {
		du, di := s.u1-s.u0, s.i1-s.i0
		w("| %s | `%s` | %d | %d | %d | %.2f | %d | %d | %.2f | %d | %d | %d | %d | %d | %d |\n",
			s.name, s.fn, s.u0, s.u1, du, float64(du)/60.3, s.i0, s.i1, float64(di)/hz, s.s0, s.waits, s.arm, s.armUnits, s.changes, s.uniqueNew)
		totU += du
		totI += di
	}
	var firstSceneU, firstSceneI uint64
	if len(segs) > 1 {
		firstSceneU, firstSceneI = segs[1].u0, segs[1].i0
	}
	w("| 合計（OPEN.EXE 載入到結束） |  | %d | %d | %d | %.2f | %d | %d | %.2f |  |  |  |  |  |  |\n\n",
		first.unit, last.unit, totU, float64(totU)/60.3, first.irq0, last.irq0, float64(totI)/hz)
	w("場景 1 開始到收尾結束：%d 單位 = %.2f 秒（單位 / 60.3），IRQ0 %d = %.2f 秒。\n\n",
		last.unit-firstSceneU, float64(last.unit-firstSceneU)/60.3, last.irq0-firstSceneI, float64(last.irq0-firstSceneI)/hz)

	w("## 等待呼叫分項\n\n")
	w("「等待呼叫」= 以下入口的非重複呼叫數合計：")
	for i, wa := range waitEntries {
		if i > 0 {
			w("、")
		}
		w("`%05X`", wa)
	}
	w("。`14871`、`14883` 是 `1485D` 的包裝，`14846` 是 arm（把計時器加上 AX 個單位，不等待），不算。`14F0D` 沒有參數，等的是先前 arm 的剩餘單位。\n\n")
	w("| 區段 |")
	for _, wa := range waitEntries {
		w(" %05X |", wa)
	}
	w(" 14846 arm |\n|---|")
	for range waitEntries {
		w("---|")
	}
	w("---|\n")
	for _, s := range segs {
		w("| %s |", s.name)
		for _, wa := range waitEntries {
			w(" %d |", s.counts[wa])
		}
		w(" %d |\n", s.arm)
	}
	w("\n## 調色盤與其他計時函式呼叫數（非重複）\n\n")
	var palAddrs []uint32
	for _, h := range timerHooks {
		if !isWait[h.addr] && h.addr != 0x14846 {
			palAddrs = append(palAddrs, h.addr)
		}
	}
	sort.Slice(palAddrs, func(i, j int) bool { return palAddrs[i] < palAddrs[j] })
	w("| 區段 |")
	for _, p := range palAddrs {
		w(" %05X |", p)
	}
	w("\n|---|")
	for range palAddrs {
		w("---|")
	}
	w("\n")
	for _, s := range segs {
		w("| %s |", s.name)
		for _, p := range palAddrs {
			w(" %d |", s.counts[p])
		}
		w("\n")
	}

	w("\n## 擷取統計\n\n")
	chg := 0
	for _, f := range a.frames {
		if f.chg && f.kind == "unit" {
			chg++
		}
	}
	w("- frames.tsv %d 行（含 start、end 各 1 行），畫面與上一格不同 %d 格，新圖（PNG）%d 張，PNG 合計 %d bytes。\n", len(a.frames), chg, a.newPNG, a.pngBytes)
	w("- calls.tsv %d 行，其中重複觸發（dup=1，OnCall 在中斷返回時的既有行為）%d 行，統計已排除。\n", len(a.calls), a.dupCount)
	w("- PNG 讀回比對 %d 張，雜湊不符 %d 張。\n", a.verified, a.badVer)
	return os.WriteFile(filepath.Join(a.outDir, "timeline.md"), []byte(sb.String()), 0o644)
}

func die(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
