// Command webplay 在瀏覽器裡即時玩一支 DOS 程式（`docs/spec/200-webplay`）。
//
// 載入程式、以接近真實時間的速度執行，畫面經 HTTP 送給瀏覽器、瀏覽器的按鍵送回來。
// **這是給人玩的互動外殼，不是對拍工具**：按鍵時機來自人、速度跟著主機負載走，
// 重跑不會得到相同結果。決定性的觀測照舊用 cmd/run、cmd/shots 與 oracle。
//
//	webplay -prog /orig/TETRIS.EXE -root /orig -listen 0.0.0.0:8086
//
// 在容器裡跑時只把埠發佈到本機：docker run -p 127.0.0.1:8086:8086 …
package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"hash/fnv"
	"image"
	"image/color"
	"image/png"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
)

//go:embed index.html
var indexHTML []byte

// pitHz 是 PIT 的輸入頻率（Hz）；分頻 65536 時每秒約 18.2 個 tick。
const pitHz = 1193182

// session 是一台正在跑的機器。m 與 d 不是 goroutine-safe，一律持有 mu 才碰。
type session struct {
	mu       sync.Mutex
	m        *machine.Machine
	d        *dos.DOS
	input    string
	realtime bool
	ips      float64 // 最近量到的每秒指令數
	runErr   error
}

func main() {
	prog := flag.String("prog", "", "要跑的 .COM 或 .EXE")
	root := flag.String("root", "", "DOS 看得到的目錄（唯讀）")
	dir := flag.String("dir", "", "DOS 的目前目錄（root 底下的子目錄）")
	scratch := flag.String("scratch", "", "可寫的暫存層（`docs/spec/009-scratch-writes`）")
	args := flag.String("args", "", "命令列參數，寫進 PSP 的 80h")
	cpuModel := flag.String("cpu", "386", "CPU 機型：8086、186 或 386")
	listen := flag.String("listen", "0.0.0.0:8086", "HTTP 位址；在容器裡跑時只把埠發佈到 127.0.0.1")
	input := flag.String("input", "bios", "按鍵送哪條路：bios（int 16h 佇列）、hw（IRQ1 掃描碼）、both")
	realtime := flag.Bool("realtime", true, "跟不上真實時間時調降 IRQ0Base，讓 BIOS 時鐘照常走（`docs/spec/200-webplay` §4）")
	flag.Parse()
	if *prog == "" || *root == "" {
		flag.Usage()
		os.Exit(2)
	}
	if *input != "bios" && *input != "hw" && *input != "both" {
		log.Fatalf("-input 只接受 bios、hw、both：%q", *input)
	}

	s, err := newSession(*prog, *root, *dir, *scratch, *args, *cpuModel)
	if err != nil {
		log.Fatal(err)
	}
	s.input, s.realtime = *input, *realtime
	go s.run()
	log.Printf("webplay：%s，在 %s 提供畫面", *prog, *listen)
	log.Fatal(http.ListenAndServe(*listen, s.routes()))
}

func newSession(prog, root, dir, scratch, args, model string) (*session, error) {
	data, err := os.ReadFile(prog)
	if err != nil {
		return nil, err
	}
	m := machine.New()
	if len(data) >= 2 && data[0] == 'M' && data[1] == 'Z' {
		err = m.LoadEXE(data)
	} else {
		err = m.LoadCOM(data)
	}
	if err != nil {
		return nil, err
	}
	d := dos.New(m, root)
	d.Dir = dir
	d.Scratch = scratch
	d.Install()
	switch model {
	case "386":
	case "186":
		m.CPU.Model = cpu.Model80186
		m.CPU.SetFlags(m.CPU.Flags)
	case "8086":
		m.CPU.Model = cpu.Model8086
		m.CPU.SetFlags(m.CPU.Flags)
	default:
		return nil, fmt.Errorf("-cpu 只接受 8086、186 或 386：%q", model)
	}
	if args != "" {
		setCmdline(m, args)
	}
	return &session{m: m, d: d, input: "bios", realtime: true}, nil
}

// setCmdline 把命令列參數寫進 PSP 的 80h（長度＋前導空白＋內容＋CR），與 cmd/run 相同。
func setCmdline(m *machine.Machine, s string) {
	if len(s) > 125 {
		s = s[:125]
	}
	psp := uint32(machine.PSPSeg) * 16
	line := " " + s + "\r"
	m.Write8(psp+0x80, uint8(len(line)-1))
	m.WriteBytes(psp+0x81, []byte(line))
}

// run 以約 60 Hz 分段執行，並讓 BIOS 時鐘跟上真實時間（`docs/spec/200-webplay` §4）。
func (s *session) run() {
	const slice = time.Second / 60
	start := time.Now()
	s.mu.Lock()
	ticks0 := s.m.Ticks
	s.mu.Unlock()
	winStart, winSteps := time.Now(), uint64(0)

	for {
		t0 := time.Now()
		s.mu.Lock()
		if s.d.Exited || s.runErr != nil {
			s.mu.Unlock()
			return
		}
		before := s.m.Steps
		// 執行到這一段的時間用完，或 BIOS 時鐘已經趕上真實時間為止。
		for time.Since(t0) < slice {
			for i := 0; i < 20000 && !s.d.Exited; i++ {
				if err := s.m.Step(); err != nil {
					s.runErr = err
					break
				}
			}
			if s.runErr != nil || s.d.Exited || s.ahead(start, ticks0) {
				break
			}
		}
		winSteps += s.m.Steps - before
		if w := time.Since(winStart); w >= 500*time.Millisecond {
			s.ips = float64(winSteps) / w.Seconds()
			s.adjustClock(start, ticks0)
			winStart, winSteps = time.Now(), 0
		}
		s.mu.Unlock()
		if rest := slice - time.Since(t0); rest > 0 {
			time.Sleep(rest)
		}
	}
}

// tickHz 是目前分頻下每秒的計時器中斷次數。
func (s *session) tickHz() float64 {
	div := float64(s.m.PITDiv)
	if div == 0 {
		div = 65536
	}
	return pitHz / div
}

// ahead 回 BIOS 時鐘是不是已經比真實時間快（快了就休息，不空轉）。
func (s *session) ahead(start time.Time, ticks0 uint64) bool {
	if !s.realtime {
		return false
	}
	want := time.Since(start).Seconds() * s.tickHz()
	return float64(s.m.Ticks-ticks0) > want+1
}

// adjustClock 在跟不上真實時間時調降 IRQ0Base：模擬的 CPU 變慢、時間照常走。
func (s *session) adjustClock(start time.Time, ticks0 uint64) {
	if !s.realtime || s.ips <= 0 {
		return
	}
	want := time.Since(start).Seconds() * s.tickHz()
	if float64(s.m.Ticks-ticks0) >= want-2 {
		return // 跟得上
	}
	base := uint64(s.ips * 17000 / pitHz * 0.95)
	if base < 1000 {
		base = 1000
	}
	if base < s.m.IRQ0Base || s.m.IRQ0Base == 0 {
		s.m.IRQ0Base = base
		s.m.RecalcIRQ0()
	}
}

func (s *session) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexHTML)
	})
	mux.HandleFunc("/frame", s.handleFrame)
	mux.HandleFunc("/key", s.handleKey)
	mux.HandleFunc("/status", s.handleStatus)
	return mux
}

func (s *session) handleFrame(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	wd, ht, rgb := screenRGB(s.m)
	s.mu.Unlock()
	h := fnv.New64a()
	h.Write(rgb)
	fmt.Fprintf(h, "%dx%d", wd, ht)
	tag := strconv.FormatUint(h.Sum64(), 16)
	w.Header().Set("X-Frame", tag)
	w.Header().Set("Cache-Control", "no-store")
	if r.URL.Query().Get("have") == tag {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	img := image.NewRGBA(image.Rect(0, 0, wd, ht))
	for i := 0; i < wd*ht; i++ {
		img.Pix[i*4], img.Pix[i*4+1], img.Pix[i*4+2], img.Pix[i*4+3] = rgb[i*3], rgb[i*3+1], rgb[i*3+2], 255
	}
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(&buf, img); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Write(buf.Bytes())
}

type keyEvent struct {
	Code string `json:"code"`
	Key  string `json:"key"`
	Down bool   `json:"down"`
}

func (s *session) handleKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var ev keyEvent
	if err := json.NewDecoder(r.Body).Decode(&ev); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	k, ok := browserKey(ev.Code, ev.Key)
	if !ok {
		w.WriteHeader(http.StatusNoContent) // 不認得的鍵不送，也不當錯誤
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if ev.Down && (s.input == "bios" || s.input == "both") {
		s.d.PushKey(k)
	}
	if s.input == "hw" || s.input == "both" {
		if ev.Down {
			s.m.QueueScan(k.Scan)
		} else {
			s.m.QueueScan(k.Scan | 0x80)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *session) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	st := map[string]any{
		"steps":     s.m.Steps,
		"ticks":     s.m.Ticks,
		"ips":       int64(s.ips),
		"irq0base":  s.m.IRQ0Base,
		"mode":      fmt.Sprintf("%02Xh", s.m.VideoMode()),
		"exited":    s.d.Exited,
		"exitCode":  s.d.ExitCode,
		"console":   string(s.d.Console),
		"keysQueue": s.d.KeysPending(),
	}
	if s.runErr != nil {
		st["error"] = s.runErr.Error()
	}
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(st)
}

// screenRGB 回目前畫面：平面模式、mode 13h，其餘當文字模式畫（`docs/spec/200-webplay` §3）。
func screenRGB(m *machine.Machine) (int, int, []uint8) {
	if w, h, rgb := m.PlanarRGB(); rgb != nil {
		return w, h, rgb
	}
	if m.VideoMode() == 0x13 {
		w, h := 320, 200
		raw, pal := m.VideoRaw(), m.Palette()
		rgb := make([]uint8, w*h*3)
		for i := 0; i < w*h && i < len(raw); i++ {
			c := pal[raw[i]]
			rgb[i*3], rgb[i*3+1], rgb[i*3+2] = c[0], c[1], c[2]
		}
		return w, h, rgb
	}
	return textRGB(m)
}

// cga16 是文字模式用的固定 16 色（簡化：不走屬性調色盤與 DAC）。
var cga16 = [16]color.RGBA{
	{0, 0, 0, 255}, {0, 0, 170, 255}, {0, 170, 0, 255}, {0, 170, 170, 255},
	{170, 0, 0, 255}, {170, 0, 170, 255}, {170, 85, 0, 255}, {170, 170, 170, 255},
	{85, 85, 85, 255}, {85, 85, 255, 255}, {85, 255, 85, 255}, {85, 255, 255, 255},
	{255, 85, 85, 255}, {255, 85, 255, 255}, {255, 255, 85, 255}, {255, 255, 255, 255},
}

// textRGB 把 B800h 的 80×25 字元以 F000:FA6E 的 8×8 字型畫成 640×200。
func textRGB(m *machine.Machine) (int, int, []uint8) {
	const cols, rows, font = 80, 25, 0xFFA6E
	w, h := cols*8, rows*8
	rgb := make([]uint8, w*h*3)
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			a := uint32(0xB8000 + (r*cols+c)*2)
			ch, attr := m.Read8(a), m.Read8(a+1)
			fg, bg := cga16[attr&15], cga16[attr>>4&7]
			for y := 0; y < 8; y++ {
				bits := uint8(0)
				if ch < 128 {
					bits = m.Read8(uint32(font + int(ch)*8 + y))
				}
				for x := 0; x < 8; x++ {
					p := bg
					if bits&(0x80>>x) != 0 {
						p = fg
					}
					i := ((r*8+y)*w + c*8 + x) * 3
					rgb[i], rgb[i+1], rgb[i+2] = p.R, p.G, p.B
				}
			}
		}
	}
	return w, h, rgb
}
