// Command shot 是《臥龍傳》的對拍取樣器：開機、依腳本操作、存一張
// 640×400 的 PNG，直接拿去跟原版的 DOSBox-X 擷取逐點比。
//
//	go run ./apps/wolong/cmd/shot -exe KI.EXE -root <原版目錄> \
//	    -script "wait;click:320,179;wait" -out shot.png
//
// 腳本的每一步用 `;` 隔開：
//
//	wait              畫面停住為止
//	steps:N           再跑 N 道指令
//	click:X,Y         在遊戲座標 (X,Y) 點左鍵（0–639 × 0–399）
//	rclick:X,Y        點右鍵（原版的取消／退回走右鍵）
//	tap:X,Y[,N]       瞬按左鍵，按住 N 道指令（預設 20 萬）。**彈出選單要用這個**
//	press / rpress    在目前位置按一下，不移動
//	move:X,Y          只移動游標
//	peek:OFF:N        印出 ds:OFF 起 N 個 byte（十六進位偏移）
//	peek:SEG:OFF:N    印出 SEG:OFF 起 N 個 byte（程式執行期配置的東西要用這個）
//	ipeek:LIN:N       印出 IDA 線性位址 LIN 起 N 個 byte（`cs:word_XXXX` 用這個）
//	tile:TX,TY        把游標移到大地圖的第 (TX,TY) 格（**選據點要用這個**）
//	celltile          印出游標現在在第幾格
//	corps             印出軍團表（現在據點／目標據點／朝向／計時器／狀態）
//	siege:攻方,據點   **直接叫原版開一場攻城戰**（守方由原版自己挑）
//	units[:all]       印出戰場上每個兵的座標與狀態（預設只印活著的）
//	runto:LIN[,N]     跑到 CS:IP 走到 IDA 線性位址 LIN（預算 N 道指令，預設 4 億）
//	save:NAME         把目前的機器狀態記到記憶體裡的一個格子
//	restore:NAME      倒回那個格子——**一次執行裡展開多個變體**
//	hotspots          印出目前畫面的熱區圖（編號 → 像素矩形）
//	at:X,Y            印出某個像素座標上的熱區編號
//	sclick:X,Y        **大地圖上**的畫面座標點擊（先把捲動原點歸零）
//	stap:X,Y          同上，瞬按
//	origin            印出捲動原點與遊戲算出來的畫面座標
//	shot:NAME         當場存一張 <-dir>/NAME.png
//	until:Y/M/D       跑到遊戲日期到某一天（即時制的取樣點寫成日期，不是秒數）
//	clock             印出目前的遊戲日期
//
// `shot` 讓一次執行產出整條時間軸的每一格——**與 DOSBox-X 那邊的
// `capture-metadata.txt` 是同一種寫法**，舊腳本可以照抄過來
// （y 記得加 `-dosbox-y`）。
//
// ⚠ **座標是遊戲座標，不是 DOSBox-X 的視窗座標。**
// 舊腳本的 y 要先除以 1.2（`-dosbox-y` 幫你換）。
//
// ⚠ **本專案不含任何原版檔案**，`-exe` 與 `-root` 都由玩家自備。
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/apps/wolong"
	"github.com/wicanr2/dosgolem/oracle"
)

func main() {
	exe := flag.String("exe", "", "KI.EXE 的路徑（必填）")
	root := flag.String("root", ".", "原版素材目錄")
	script := flag.String("script", "wait", "操作腳本，用 ; 隔開")
	out := flag.String("out", "shot.png", "最後一張輸出的 PNG（腳本沒有 shot 步驟時才用）")
	dir := flag.String("dir", ".", "shot:NAME 步驟寫到哪個目錄")
	full := flag.Bool("full", false, "存完整的 640×480，不裁成 640×400")
	dosboxY := flag.Bool("dosbox-y", false, "腳本裡的 y 是 DOSBox-X 的視窗座標，幫忙換算")
	budget := flag.Uint64("budget", 40_000_000, "每一步的指令數上限")
	fontFull := flag.String("font-full", "", "全形字模檔（預設 END_S13.DAT）")
	fontHalf := flag.String("font-half", "", "半形字模檔（預設 END_S14.DAT）")
	watch := flag.String("watch", "", "攔這些 IDA 線性位址的呼叫，逗號分隔（十六進位）")
	flag.Parse()

	if *exe == "" {
		flag.Usage()
		os.Exit(2)
	}
	o, err := wolong.LoadWith(*exe, *root, *fontFull, *fontHalf)
	if err != nil {
		die(err)
	}
	defer o.Close()

	if *watch != "" {
		if err := installWatches(o, *watch); err != nil {
			die(err)
		}
	}

	shots := 0
	for i, step := range strings.Split(*script, ";") {
		step = strings.TrimSpace(step)
		if step == "" {
			continue
		}
		if err := run(o, step, *dosboxY, *budget, *dir, *full); err != nil {
			die(fmt.Errorf("第 %d 步「%s」：%w", i+1, step, err))
		}
		if strings.HasPrefix(step, "shot:") {
			shots++
		}
		fmt.Printf("第 %d 步「%s」完成，累計 %d 道指令\n", i+1, step, o.Steps())
	}
	if shots > 0 {
		// 腳本自己存過了就不要再存一張沒有名字的——**同一個狀態出現兩份
		// 檔案，之後沒有人分得出哪一份是驗收依據。**
		report(o)
		return
	}

	if *full {
		err = o.WritePNG(*out)
	} else {
		err = wolong.Shot(o, *out)
	}
	if err != nil {
		die(err)
	}
	w, h, _ := o.Screen()
	fmt.Printf("寫出 %s（畫面 %d×%d）\n", *out, w, h)
	report(o)
}

// report 印收工前一定要看的東西。
//
// **「跑得動」與「跑得動但行為不對」的差別在這裡**：沒實作的服務宣告成功，
// 該填的緩衝區沒填就是垃圾，症狀出現在很後面而且完全不指向這裡。
func report(o *oracle.Oracle) {
	full, half, missing := o.FontStats()
	fmt.Printf("字型常式：全形 %d 次、半形 %d 次，讀不到字模 %d 次\n",
		full, half, missing)
	if rep := o.Unimplemented(); len(rep) > 0 {
		fmt.Printf("沒實作的服務（%d 種）：%s\n", len(rep), strings.Join(rep, "、"))
	}
}

// states 是 `save:`／`restore:` 的格子。
//
// ⚠ **不落地成檔案**：裡面是原版的整份記憶體映像，寫到磁碟等於散布它
// （`oracle.State` 的說明）。所以變體只能在**同一次執行裡**展開——
// 而那正好夠用：走到戰場要四億五千萬道指令，之後每個變體只要幾百萬。
var states = map[string]*oracle.State{}

func run(o *oracle.Oracle, step string, dosboxY bool, budget uint64,
	dir string, full bool) error {
	verb, arg, _ := strings.Cut(step, ":")
	switch verb {
	case "wait":
		return o.RunUntil(wolong.Booted(), oracle.Budget(budget))
	case "until":
		f := strings.Split(arg, "/")
		if len(f) != 3 {
			return fmt.Errorf("日期要寫成 年/月/日")
		}
		var n [3]int
		for i, v := range f {
			x, err := strconv.Atoi(strings.TrimSpace(v))
			if err != nil {
				return err
			}
			n[i] = x
		}
		return o.RunUntil(wolong.UntilDate(n[0], n[1], n[2]), oracle.Budget(budget))
	case "clock":
		fmt.Printf("   遊戲時鐘：%s\n", wolong.Clock(o))
		return nil
	case "shot":
		path := filepath.Join(dir, arg+".png")
		if full {
			return o.WritePNG(path)
		}
		return wolong.Shot(o, path)
	case "steps":
		n, err := strconv.ParseUint(arg, 10, 64)
		if err != nil {
			return err
		}
		return o.Run(n)
	case "press", "rpress":
		// 原地按，不移動——既有的 DOSBox 腳本大量用「移過去、再按一次」。
		if verb == "rpress" {
			return o.Press(oracle.Button(1))
		}
		return o.Press()
	case "peek":
		return peek(o, arg)
	case "ipeek":
		return ipeek(o, arg)
	case "tile":
		tx, ty, err := point(arg, false)
		if err != nil {
			return err
		}
		if err := wolong.PointAtTile(o, tx, ty); err != nil {
			return err
		}
		mx, my := o.Mouse()
		fmt.Printf("   游標停在第 (%d,%d) 格，滑鼠 (%d,%d)\n", tx, ty, mx, my)
		return nil
	case "runto":
		// `runto:LIN[,N]`：跑到 CS:IP 走到某個 IDA 線性位址。
		//
		// ⭐ **即時制的取樣點寫成「事件」而不是秒數。** 等一場仗開打
		// 不知道要幾天，但知道它一定會走到 `sub_11B5A`。
		spec, budgetArg, _ := strings.Cut(arg, ",")
		lin, err := strconv.ParseUint(strings.TrimSpace(spec), 16, 32)
		if err != nil {
			return err
		}
		b := uint64(400_000_000)
		if budgetArg != "" {
			if b, err = strconv.ParseUint(strings.TrimSpace(budgetArg), 10, 64); err != nil {
				return err
			}
		}
		if err := o.RunUntil(oracle.At(o.IDA(uint32(lin))), oracle.Budget(b)); err != nil {
			return err
		}
		fmt.Printf("   走到 %05X（遊戲時鐘 %s）\n", lin, wolong.Clock(o))
		return nil
	case "save":
		states[arg] = o.Save()
		fmt.Printf("   記下狀態「%s」（第 %d 道指令）\n", arg, o.Steps())
		return nil
	case "restore":
		st, ok := states[arg]
		if !ok {
			return fmt.Errorf("沒有記過狀態「%s」", arg)
		}
		o.Restore(st)
		fmt.Printf("   倒回狀態「%s」（第 %d 道指令）\n", arg, o.Steps())
		return nil
	case "units":
		us := wolong.Units(o, arg != "all")
		fmt.Printf("   場上 %d 個兵（側/隊/位 座標 體力 命令 旗標）：\n", len(us))
		for _, u := range us {
			fmt.Printf("     %d/%d/%d (%2d,%2d) 體%3d 令%02X→%02X 旗%02X\n",
				u.Side, u.Squad, u.Slot, u.X, u.Y, u.Stamina,
				u.Order, u.NewOrder, u.Flags)
		}
		return nil
	case "siege":
		// `siege:攻方軍團,據點`——直接叫原版開一場攻城戰。
		x, y, err := point(arg, false)
		if err != nil {
			return err
		}
		if err := wolong.OpenSiege(o, x, y); err != nil {
			return err
		}
		fmt.Printf("   叫原版開攻城戰：軍團 %d 打據點 %d\n", x, y)
		return nil
	case "corps":
		cs := wolong.CorpsTable(o)
		fmt.Printf("   軍團 %d 支：\n", len(cs))
		for _, c := range cs {
			slots := ""
			for _, sl := range c.Slots {
				slots += fmt.Sprintf(" %d/%d", sl.Men, sl.Kind)
			}
			fmt.Printf("     #%3d 勢力%2d 主將%3d 旗標%02X 兵力%5d 士氣%4d "+
				"現在據點%3d (%4d,%4d) 目標據點%3d 朝向%d 計時%3d 間隔%2d 狀態%02X 槽%s\n",
				c.Index, c.Faction, c.No, c.Flags, c.Troops, c.Morale,
				c.At/8, c.X, c.Y, c.Target, c.Facing, c.Timer, c.Interval, c.Stage, slots)
		}
		return nil
	case "celltile":
		x, y := wolong.CursorTile(o)
		fmt.Printf("   游標所在的格 ＝ (%d,%d)\n", x, y)
		return nil
	case "origin":
		ox, oy := wolong.ScrollOrigin(o)
		cx, cy := wolong.ScreenCursor(o)
		x0, x1, y0, y1 := o.MouseRange()
		mx, my := o.Mouse()
		fmt.Printf("   捲動原點 (%d,%d)，畫面游標 (%d,%d)，滑鼠 (%d,%d)，"+
			"驅動範圍 %d–%d × %d–%d\n", ox, oy, cx, cy, mx, my, x0, x1, y0, y1)
		return nil
	case "sclick", "stap":
		x, y, err := point(arg, dosboxY)
		if err != nil {
			return err
		}
		if verb == "stap" {
			return wolong.TapScreen(o, x, y)
		}
		return wolong.ClickScreen(o, x, y)
	case "hotspots":
		// ⭐ **「點了沒反應」的直接答案。** 座標對不對攔得到，
		// 「那裡到底有沒有東西可點」只有這張圖知道。
		zs := wolong.HotzoneBoxes(o)
		if len(zs) == 0 {
			fmt.Println("   （目前沒有熱區圖——遊戲還沒登記，或還沒進到有熱區的畫面）")
			return nil
		}
		fmt.Printf("   熱區 %d 個：\n", len(zs))
		for _, z := range zs {
			fmt.Printf("     #%3d  x %3d..%3d  y %3d..%3d  （%d 格）\n",
				z.ID, z.X, z.X+z.W-1, z.Y, z.Y+z.H-1, z.Cells)
		}
		return nil
	case "at":
		x, y, err := point(arg, dosboxY)
		if err != nil {
			return err
		}
		fmt.Printf("   (%d,%d) 的熱區編號 ＝ %d\n", x, y, wolong.HotzoneAt(o, x, y))
		return nil
	case "click", "rclick", "move", "tap":
		spec := arg
		hold := uint64(0)
		if verb == "tap" {
			// tap:X,Y[,N]
			if i := strings.LastIndex(arg, ","); strings.Count(arg, ",") == 2 {
				n, err := strconv.ParseUint(strings.TrimSpace(arg[i+1:]), 10, 64)
				if err != nil {
					return err
				}
				hold, spec = n, arg[:i]
			}
		}
		x, y, err := point(spec, dosboxY)
		if err != nil {
			return err
		}
		switch verb {
		case "move":
			o.MoveMouse(x, y)
			return nil
		case "rclick":
			return o.Click(x, y, oracle.Button(1))
		case "tap":
			if hold > 0 {
				return o.Tap(x, y, oracle.Hold(hold))
			}
			return o.Tap(x, y)
		}
		return o.Click(x, y)
	}
	return fmt.Errorf("看不懂的動作")
}

// installWatches 掛 OnCall。
//
// ⚠ **掛在沒人呼叫的位址上印不出東西，與「沒接上」長得一模一樣。**
// 正對照要用一個已知會被呼叫的位址。
func installWatches(o *oracle.Oracle, spec string) error {
	for _, s := range strings.Split(spec, ",") {
		s = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), "0x"))
		lin, err := strconv.ParseUint(s, 16, 32)
		if err != nil {
			return fmt.Errorf("看不懂的位址 %q：%w", s, err)
		}
		addr := o.IDA(uint32(lin))
		label := strings.ToUpper(s)
		o.OnCall(addr, func(o *oracle.Oracle) {
			r := o.Regs()
			// ⚠ **near 與 far 的返回位址在堆疊上的版面不同**，
			// 挑一種安靜地印會讓另一種讀到垃圾——而垃圾看起來
			// 就是一個合法位址。兩種都印，讓人自己判斷。
			fmt.Printf("  #%d 呼叫 %s AX=%04X BX=%04X CX=%04X DX=%04X "+
				"SI=%04X DI=%04X 來自 近=%05X 遠=%05X\n",
				o.Steps(), label, r.AX, r.BX, r.CX, r.DX, r.SI, r.DI,
				o.ToIDA(o.NearCaller()), o.ToIDA(o.Caller()))
		})
	}
	return nil
}

// peek 印出 `ds:OFF` 起 N 個 byte。
//
// **畫面只回答「長什麼樣」，行為要問記憶體**——這是不用 DOSBox 的重點。
func peek(o *oracle.Oracle, arg string) error {
	f := strings.Split(arg, ":")
	if len(f) != 2 && len(f) != 3 {
		return fmt.Errorf("格式是 peek:偏移:長度 或 peek:段:偏移:長度")
	}
	var addr oracle.Addr
	var label string
	if len(f) == 3 {
		seg, err := strconv.ParseUint(strings.TrimSpace(f[0]), 16, 16)
		if err != nil {
			return err
		}
		off, err := strconv.ParseUint(strings.TrimSpace(f[1]), 16, 16)
		if err != nil {
			return err
		}
		addr, label = oracle.Far(uint16(seg), uint16(off)),
			fmt.Sprintf("%04X:%04X", seg, off)
		f = f[1:]
	} else {
		off, err := strconv.ParseUint(strings.TrimSpace(f[0]), 16, 16)
		if err != nil {
			return err
		}
		addr, label = o.DS(uint16(off)), fmt.Sprintf("ds:%04X", off)
	}
	n, err := strconv.Atoi(strings.TrimSpace(f[1]))
	if err != nil {
		return err
	}
	buf := o.Bytes(addr, n)
	parts := make([]string, len(buf))
	for i, b := range buf {
		parts[i] = fmt.Sprintf("%02X", b)
	}
	fmt.Printf("   %s = %s\n", label, strings.Join(parts, " "))
	return nil
}

// ipeek 印出一個 **IDA 線性位址**起 N 個 byte。
//
// ⚠ **`cs:word_1989A` 這種變數 `peek` 讀不到**——它不在 DGROUP，
// 而它的段要看程式載到哪裡。RE 筆記寫的是線性位址，這一支就吃線性位址。
func ipeek(o *oracle.Oracle, arg string) error {
	f := strings.Split(arg, ":")
	if len(f) != 2 {
		return fmt.Errorf("格式是 ipeek:線性位址:長度")
	}
	lin, err := strconv.ParseUint(strings.TrimSpace(f[0]), 16, 32)
	if err != nil {
		return err
	}
	n, err := strconv.Atoi(strings.TrimSpace(f[1]))
	if err != nil {
		return err
	}
	buf := o.Bytes(o.IDA(uint32(lin)), n)
	parts := make([]string, len(buf))
	for i, b := range buf {
		parts[i] = fmt.Sprintf("%02X", b)
	}
	fmt.Printf("   %05X = %s\n", lin, strings.Join(parts, " "))
	return nil
}

func point(arg string, dosboxY bool) (int, int, error) {
	xs, ys, ok := strings.Cut(arg, ",")
	if !ok {
		return 0, 0, fmt.Errorf("座標要寫成 X,Y")
	}
	x, err := strconv.Atoi(strings.TrimSpace(xs))
	if err != nil {
		return 0, 0, err
	}
	y, err := strconv.Atoi(strings.TrimSpace(ys))
	if err != nil {
		return 0, 0, err
	}
	if dosboxY {
		y = wolong.FromDOSBoxY(y)
	}
	return x, y, nil
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "shot:", err)
	os.Exit(1)
}
