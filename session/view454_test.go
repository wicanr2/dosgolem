package session

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// docs/spec/454-session-view-block-read-and-digest §3.1 第 3 項回呼案、第 7、9、11 項（session 側）。

// prog454Session 同 internal/machine 的 prog454：0100 call 0110／jmp 0100；0110 inc word [0800h]／ret；0181 retf。
func prog454Session() []byte {
	code := make([]byte, 0x82)
	copy(code, []byte{0xE8, 0x0D, 0x00, 0xEB, 0xFB})
	copy(code[0x10:], []byte{0xFF, 0x06, 0x00, 0x08, 0xC3})
	code[0x80], code[0x81] = 0xCF, 0xCB
	return code
}

type obs454s struct {
	cs, ip uint16
	d      uint64
}

// cbObserver 記下每個觀察點的 PC 與送達計數；第 injectAt 次看到 S 入口時排一次回呼（目標 0181h 的 RETF）。
type cbObserver struct {
	o        *Owner
	tr       []obs454s
	seenS    int
	injectAt int
}

func (c *cbObserver) BeforeStep(v StepView) error {
	vp := &v
	c.tr = append(c.tr, obs454s{vp.CS(), vp.IP(), vp.Deliveries()})
	if vp.CS() == machine.PSPSeg && vp.IP() == 0x0110 {
		c.seenS++
		if c.seenS == c.injectAt {
			c.o.machine.QueueCallback(machine.QueuedCall{Seg: machine.PSPSeg, Off: 0x0181})
		}
	}
	return nil
}
func (c *cbObserver) VideoWrite(machine.VideoWrite) {}
func (c *cbObserver) Frame([]byte, [256][3]uint8)   {}

// §3.1 第 3 項（回呼案）：排入的回呼落在 S 入口的觀察點；去重後 S 命中數等於程式自己的計數。
func TestDeliveries454Callback(t *testing.T) {
	o := startSyntheticOwner(t, prog454Session())
	obs := &cbObserver{o: o, injectAt: 5}
	o.observer = obs
	if _, err := runTurn(t, o, 3000); err != nil {
		t.Fatal(err)
	}
	if o.machine.CallbacksMade() != 1 {
		t.Fatalf("前提：回呼應送出一次，實際 %d", o.machine.CallbacksMade())
	}
	tr := append(obs.tr, obs454s{0xFFFF, 0xFFFF, o.machine.Deliveries()})
	hits, atS := 0, 0
	for i := 0; i+1 < len(tr); i++ {
		p, n := tr[i], tr[i+1]
		if n.d < p.d || n.d-p.d > 1 {
			t.Fatalf("第 %d 個觀察點：計數 %d → %d", i, p.d, n.d)
		}
		if p.cs != machine.PSPSeg || p.ip != 0x0110 {
			continue
		}
		if n.d == p.d {
			hits++
			continue
		}
		atS++
		again := false
		for _, x := range tr[i+1:] {
			if x.cs == p.cs && x.ip == p.ip {
				again = true
				break
			}
		}
		if !again {
			t.Errorf("第 %d 個觀察點在 S 有送達，之後沒有再觀察到 S", i)
		}
	}
	a := uint32(machine.PSPSeg)*16 + 0x0800
	calls := int(o.machine.Mem[a]) | int(o.machine.Mem[a+1])<<8
	if hits != calls || atS == 0 {
		t.Errorf("去重後 S 命中 %d 次，程式實際 %d 次；落在 S 的送達 %d 次", hits, calls, atS)
	}
	t.Logf("回呼案：Deliveries %d（含 DOS 的 IRQ0），落在 S 入口 %d 次", o.machine.Deliveries(), atS)
}

// readerObserver 每個觀察點讀（或不讀）送達計數與一段記憶體。
type readerObserver struct {
	read bool
	buf  [256]byte
}

func (r *readerObserver) BeforeStep(v StepView) error {
	if r.read {
		vp := &v
		_ = vp.Deliveries()
		vp.ReadBlock(uint32(machine.PSPSeg)*16, r.buf[:])
	}
	return nil
}
func (r *readerObserver) VideoWrite(machine.VideoWrite) {}
func (r *readerObserver) Frame([]byte, [256][3]uint8)   {}

// §3.1 第 7 項：同一程式、同一輸入，觀察器讀與不讀計數，停止點的 Digest 與 MachineDigest 相同。
func TestDeliveries454DoesNotAffectExecution(t *testing.T) {
	var ds []StateDigest
	var fs []FullDigest
	for _, read := range []bool{false, true} {
		o := startSyntheticOwner(t, prog454Session())
		o.observer = &readerObserver{read: read}
		for i := 0; i < 4; i++ {
			if _, err := runTurn(t, o, 50000); err != nil {
				t.Fatal(err)
			}
		}
		if o.machine.Deliveries() == 0 {
			t.Fatal("前提：要有送達（DOS 的 IRQ0）")
		}
		d, err := o.Digest()
		if err != nil {
			t.Fatal(err)
		}
		f, err := o.MachineDigest()
		if err != nil {
			t.Fatal(err)
		}
		ds, fs = append(ds, d), append(fs, f)
	}
	if ds[0] != ds[1] || fs[0] != fs[1] {
		t.Errorf("讀與不讀計數，摘要應相同：Digest %v，MachineDigest %v", ds[0] == ds[1], fs[0] == fs[1])
	}
}

// staleObserver 第 1 步保留視圖，第 2 步以舊視圖呼叫 ReadBlock 或 Deliveries。
type staleObserver struct {
	keep     *StepView
	n        int
	useBlock bool
	gotOK    bool
	gotBuf   []byte
	gotD     uint64
}

func (s *staleObserver) BeforeStep(v StepView) error {
	s.n++
	if s.keep == nil {
		kept := v
		s.keep = &kept
		return nil
	}
	if s.n == 2 {
		if s.useBlock {
			s.gotBuf = bytes.Repeat([]byte{0xAA}, 64)
			s.gotOK = s.keep.ReadBlock(uint32(machine.PSPSeg)*16+0x100, s.gotBuf)
		} else {
			s.gotD = s.keep.Deliveries()
		}
	}
	return nil
}
func (s *staleObserver) VideoWrite(machine.VideoWrite) {}
func (s *staleObserver) Frame([]byte, [256][3]uint8)   {}

// §3.1 第 9 項：保留的視圖呼叫 ReadBlock 得全 0、ok 為假；呼叫 Deliveries 得 0；Advance 回觀測器誤用。
func TestReadBlock454Misuse(t *testing.T) {
	for _, useBlock := range []bool{true, false} {
		o := startSyntheticOwner(t, prog454Session())
		o.machine.QueueCallback(machine.QueuedCall{Seg: machine.PSPSeg, Off: 0x0181}) // 讓計數非 0
		s := &staleObserver{useBlock: useBlock}
		o.observer = s
		r, err := runTurn(t, o, 20)
		if err == nil || !strings.Contains(err.Error(), "觀測器在回呼外使用了 StepView") {
			t.Fatalf("useBlock=%v：應回觀測器誤用：%v", useBlock, err)
		}
		assertObserverFault(t, o, r, err, 2)
		if useBlock {
			if s.gotOK || !bytes.Equal(s.gotBuf, make([]byte, 64)) {
				t.Errorf("保留的視圖 ReadBlock 應全 0、ok 為假：ok=%v % X", s.gotOK, s.gotBuf[:8])
			}
		} else if s.gotD != 0 || o.machine.Deliveries() == 0 {
			t.Errorf("保留的視圖 Deliveries 應回 0：%d（機器 %d）", s.gotD, o.machine.Deliveries())
		}
	}
}

// ReadBlock 在開著的視圖內與 Read8 逐位元組相同。
type blockObserver struct {
	t    *testing.T
	bad  int
	seen int
}

func (b *blockObserver) BeforeStep(v StepView) error {
	vp := &v
	a := uint32(vp.SS())*16 + uint32(vp.SP()) - 0x40
	buf := make([]byte, 0x80)
	if !vp.ReadBlock(a, buf) {
		b.bad++
	}
	for i := range buf {
		if buf[i] != vp.Read8(a+uint32(i)) {
			b.bad++
			break
		}
	}
	b.seen++
	return nil
}
func (b *blockObserver) VideoWrite(machine.VideoWrite) {}
func (b *blockObserver) Frame([]byte, [256][3]uint8)   {}

func TestReadBlock454MatchesRead8(t *testing.T) {
	o := startSyntheticOwner(t, prog454Session())
	b := &blockObserver{t: t}
	o.observer = b
	if _, err := runTurn(t, o, 2000); err != nil {
		t.Fatal(err)
	}
	if b.bad != 0 || b.seen != 2000 {
		t.Errorf("ReadBlock 與 Read8 不同 %d 次（觀察 %d 次）", b.bad, b.seen)
	}
}

// ---- §3.1 第 11 項：MachineDigest ---------------------------------------------------

// regTrace 記每個觀察點的暫存器。
type regTrace struct{ rows []string }

func (r *regTrace) BeforeStep(v StepView) error {
	vp := &v
	r.rows = append(r.rows, fmt.Sprintf("%04X:%04X %04X %04X %04X %04X %04X %04X", vp.CS(), vp.IP(), vp.AX(), vp.BX(),
		vp.SP(), vp.Flags(), vp.DS(), vp.SS()))
	return nil
}
func (r *regTrace) VideoWrite(machine.VideoWrite) {}
func (r *regTrace) Frame([]byte, [256][3]uint8)   {}

func TestMachineDigest454(t *testing.T) {
	// 只差 Hi[AX]：Digest 相同，MachineDigest.Machine 不同、DOS 相同。
	a := startSyntheticOwner(t, prog454Session())
	b := startSyntheticOwner(t, prog454Session())
	b.machine.CPU.Hi[cpu.AX] = 0x1234
	da, _ := a.Digest()
	db, _ := b.Digest()
	fa, err := a.MachineDigest()
	if err != nil {
		t.Fatal(err)
	}
	fb, err := b.MachineDigest()
	if err != nil {
		t.Fatal(err)
	}
	if da != db || fa.Machine == fb.Machine || fa.DOS != fb.DOS {
		t.Errorf("只差 Hi[AX]：Digest 相同=%v（應真）、Machine 相同=%v（應假）、DOS 相同=%v（應真）",
			da == db, fa.Machine == fb.Machine, fa.DOS == fb.DOS)
	}

	// 呼叫前後 Digest 相同；接著 1000 步與不呼叫者逐步相同。
	c := startSyntheticOwner(t, prog454Session())
	d := startSyntheticOwner(t, prog454Session())
	for _, o := range []*Owner{c, d} {
		if _, err := runTurn(t, o, 3000); err != nil {
			t.Fatal(err)
		}
	}
	before, _ := c.Digest()
	if _, err := c.MachineDigest(); err != nil {
		t.Fatal(err)
	}
	after, _ := c.Digest()
	if before != after {
		t.Error("MachineDigest 前後 Digest 應相同")
	}
	tc, td := &regTrace{}, &regTrace{}
	c.observer, d.observer = tc, td
	for _, x := range []struct {
		o *Owner
		r *regTrace
	}{{c, tc}, {d, td}} {
		if _, err := runTurn(t, x.o, 1000); err != nil {
			t.Fatal(err)
		}
	}
	if len(tc.rows) != 1000 || strings.Join(tc.rows, "\n") != strings.Join(td.rows, "\n") {
		t.Errorf("呼叫 MachineDigest 之後 1000 步應與不呼叫者逐步相同（%d／%d 步）", len(tc.rows), len(td.rows))
	}
	dc, _ := c.Digest()
	dd, _ := d.Digest()
	fc, _ := c.MachineDigest()
	fd, _ := d.MachineDigest()
	if dc != dd || fc != fd {
		t.Error("1000 步後兩者摘要應相同")
	}

	// 回合進行中與關閉後回錯誤。
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := c.MachineDigest(); err == nil {
		t.Error("關閉後應回錯誤")
	}
}

// 只差 DOS 的一個檔案 handle 狀態：同一個根目錄、同一支程式，X 檔的長度不同（前兩個 byte 相同，
// 程式讀到的內容相同），機器摘要相同、DOS 摘要不同。
func TestMachineDigest454DOSHandle(t *testing.T) {
	root := synthTree(t, synthFiles())
	scratch := t.TempDir()
	s1 := saveSynth(t, root, scratch, nil, nil)
	f1, err := fullDigestOf(s1.m, s1.d)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "X"), []byte("abcdefghij"), 0o644); err != nil {
		t.Fatal(err)
	}
	s2 := saveSynth(t, root, scratch, nil, nil)
	f2, err := fullDigestOf(s2.m, s2.d)
	if err != nil {
		t.Fatal(err)
	}
	if f1.Machine != f2.Machine || f1.DOS == f2.DOS {
		t.Errorf("只差 X 的 handle 長度：Machine 相同=%v（應真）、DOS 相同=%v（應假）", f1.Machine == f2.Machine, f1.DOS == f2.DOS)
	}
	// 同一份狀態兩次摘要相同。
	if g1, _ := fullDigestOf(s1.m, s1.d); g1 != f1 {
		t.Error("同一份狀態兩次摘要不同")
	}
}

// 兩行程案（N13）：以 helper process 在子行程先以 gob 編一個無關型別、再取同一份狀態的摘要；
// 另一個子行程不先編。兩個子行程與父行程的摘要必須相同（gob 的型別編號依編碼順序配發，直接雜湊 gob 會不同）。
const helper454Env = "DOSGOLEM_SESSION_HELPER_454"

type unrelated454 struct {
	A []struct{ X, Y int }
	B map[string][]byte
	C *unrelated454
}

func digest454Fixed(t *testing.T) FullDigest {
	t.Helper()
	m := machine.New()
	if err := m.LoadCOM(prog454Session()); err != nil {
		t.Fatal(err)
	}
	d := dos.New(m, "/dosgolem-454-root")
	d.Install()
	t.Cleanup(d.Close)
	f, err := fullDigestOf(m, d)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestMachineDigest454Helper(t *testing.T) {
	mode := os.Getenv(helper454Env)
	if mode == "" {
		t.Skip("只在 helper process 內執行")
	}
	if mode == "pre" {
		if err := gob.NewEncoder(&bytes.Buffer{}).Encode(unrelated454{B: map[string][]byte{"a": {1}}}); err != nil {
			t.Fatal(err)
		}
	}
	f := digest454Fixed(t)
	fmt.Printf("DIGEST454 %s %s\n", hex.EncodeToString(f.Machine[:]), hex.EncodeToString(f.DOS[:]))
}

func TestMachineDigest454TwoProcesses(t *testing.T) {
	if os.Getenv(helper454Env) != "" {
		t.Skip("helper process")
	}
	run := func(mode string) string {
		cmd := exec.Command(os.Args[0], "-test.run=^TestMachineDigest454Helper$", "-test.count=1")
		cmd.Env = append(os.Environ(), helper454Env+"="+mode)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("helper %s：%v\n%s", mode, err, out)
		}
		for _, l := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(l, "DIGEST454 ") {
				return strings.TrimPrefix(l, "DIGEST454 ")
			}
		}
		t.Fatalf("helper %s 沒有輸出摘要：\n%s", mode, out)
		return ""
	}
	plain, pre := run("plain"), run("pre")
	f := digest454Fixed(t)
	self := hex.EncodeToString(f.Machine[:]) + " " + hex.EncodeToString(f.DOS[:])
	if plain != pre || plain != self {
		t.Errorf("兩行程摘要應相同：\n plain %s\n pre   %s\n 父    %s", plain, pre, self)
	}
}
