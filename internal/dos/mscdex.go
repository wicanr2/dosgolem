package dos

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// MSCDEX（`int 2Fh AH=15h`）與它掛的一台光碟機。規格：`docs/spec/434-mscdex-minimal`。
//
// 依據是 DOSBox-X `src/dos/dos_mscdex.cpp`。這裡只做程式「問有沒有光碟、問曲目、
// 叫它播放」那一層：曲目表由呼叫端給（`ParseCDTOC`），播放是依指令數推算位置的
// 狀態機，不出聲音，也不讀光碟的資料扇區。
//
// ⚠ 沒有設 `DOS.CD` 時行為與以前相同：`AH=15h` 記為未實作、BX 不動。
// 三國志V 的 `MAIN.EXE` 因此判定沒有光碟，畫面停在「請將光碟片放入光碟機」。

// CDTrack 是曲目表的一軌。
type CDTrack struct {
	Number uint8
	Data   bool   // 資料軌（attr 40h）；否則是音軌（attr 00h）
	Start  uint32 // 起點 LBA（不含 150 frames 的前導）
	// Offset 是這一軌第一個扇區在光碟映像檔裡的位元組位移（每扇區 2352 bytes），
	// −1 ＝ 曲目表沒給，這一軌不能倒成音訊（`docs/spec/438-mscdex-cd-audio-dump`）。
	Offset int64
}

// CDPlay 是一段實際播放：從 FromStep 起播 Start 開始的 Length frames，
// 在 ToStep（0 ＝ 沒被停）或播滿長度時結束。SPS 是當時的每秒指令數。
type CDPlay struct {
	FromStep, ToStep uint64
	Start, Length    uint32
	SPS              float64
}

// CDROM 是 MSCDEX 掛的光碟機與它的播放狀態。
type CDROM struct {
	Drive   uint8 // 0 ＝ A:，3 ＝ D:
	Tracks  []CDTrack
	LeadOut uint32 // 光碟結尾的 LBA

	// 播放狀態（DOSBox-X 的 dinfo：audioPlay、audioPaused、audioStart、audioEnd）。
	playing, paused bool
	start, length   uint32 // 起點 LBA 與長度（frames）
	startStep       uint64 // 開始播放時的指令數

	// Plays 是播放紀錄，給 WriteWAV 倒音訊用。
	Plays []CDPlay
}

// cdFramePad 是 Red Book 的 2 秒前導（150 frames）：MSF ＝ LBA ＋ 150。
const cdFramePad = 150

// mscdexVersion 是 `AX=150Ch` 回的版本（DOSBox-X 的 2.23）。
const mscdexVersion = 2<<8 + 23

// ParseCDTOC 讀曲目表文字：每行 `<軌號> audio|data <LBA>`，加一行 `leadout <LBA>`，
// `#` 之後是註解。軌號要從 1 起連續遞增。
func ParseCDTOC(drive uint8, text string) (*CDROM, error) {
	cd := &CDROM{Drive: drive}
	sc := bufio.NewScanner(strings.NewReader(text))
	for line := 1; sc.Scan(); line++ {
		s := sc.Text()
		if i := strings.IndexByte(s, '#'); i >= 0 {
			s = s[:i]
		}
		f := strings.Fields(s)
		if len(f) == 0 {
			continue
		}
		if f[0] == "leadout" && len(f) == 2 {
			v, err := strconv.ParseUint(f[1], 10, 32)
			if err != nil {
				return nil, fmt.Errorf("曲目表第 %d 行：%v", line, err)
			}
			cd.LeadOut = uint32(v)
			continue
		}
		if (len(f) != 3 && len(f) != 4) || (f[1] != "audio" && f[1] != "data") {
			return nil, fmt.Errorf("曲目表第 %d 行要是 `<軌號> audio|data <LBA> [映像位移]`：%q", line, sc.Text())
		}
		n, err1 := strconv.ParseUint(f[0], 10, 8)
		lba, err2 := strconv.ParseUint(f[2], 10, 32)
		off := int64(-1)
		var err3 error
		if len(f) == 4 {
			off, err3 = strconv.ParseInt(f[3], 10, 64)
		}
		if err1 != nil || err2 != nil || err3 != nil {
			return nil, fmt.Errorf("曲目表第 %d 行的數字不對：%q", line, sc.Text())
		}
		if int(n) != len(cd.Tracks)+1 {
			return nil, fmt.Errorf("曲目表第 %d 行：軌號 %d 不連續", line, n)
		}
		cd.Tracks = append(cd.Tracks, CDTrack{Number: uint8(n), Data: f[1] == "data", Start: uint32(lba), Offset: off})
	}
	if len(cd.Tracks) == 0 || cd.LeadOut == 0 {
		return nil, fmt.Errorf("曲目表至少要一軌與 leadout")
	}
	return cd, nil
}

// msf 把 frames（已含前導）拆成分、秒、frame。
func msf(frames uint32) (m, s, f uint8) {
	return uint8(frames / 75 / 60), uint8(frames / 75 % 60), uint8(frames % 75)
}

// pos 回目前的播放位置（LBA）與是否還在播。播完就停下（DOSBox-X 的
// GetAudioStatus 回 playing=false 之後 audioPlay 清掉，起點不動）。
func (cd *CDROM) pos(d *DOS) (uint32, bool) {
	if !cd.playing {
		return cd.start, false
	}
	el := uint32(float64(d.M.Steps-cd.startStep) / d.M.StepsPerSecondNow() * 75)
	if el >= cd.length {
		cd.playing = false
		return cd.start, false
	}
	return cd.start + el, true
}

// trackAt 回 LBA 所在的軌（最後一個起點不大於它的軌）。
func (cd *CDROM) trackAt(lba uint32) CDTrack {
	t := cd.Tracks[0]
	for _, x := range cd.Tracks {
		if x.Start <= lba {
			t = x
		}
	}
	return t
}

func (t CDTrack) attr() uint8 {
	if t.Data {
		return 0x40
	}
	return 0
}

// writeCDHeader 在 StubSeg 寫光碟機的 device header（DOSBox-X 的 sDeviceHeader）。
// strategy／interrupt 指向一個 RETF：程式走 `AX=1510h` 送請求，不直接呼叫它們。
func (d *DOS) writeCDHeader() {
	base := uint32(machine.StubSeg)*16 + machine.MSCDEXHeaderOff
	d.M.Write16(base, 0xFFFF) // 下一個 header：沒有
	d.M.Write16(base+2, 0xFFFF)
	d.M.Write16(base+4, 0xC800) // 字元裝置、支援 IOCTL、open／close
	d.M.Write16(base+6, machine.MSCDEXHeaderOff+0x16)
	d.M.Write16(base+8, machine.MSCDEXHeaderOff+0x16)
	d.M.WriteBytes(base+0x0A, []byte("MSCD001 "))
	d.M.Write16(base+0x12, 0)
	d.M.Write8(base+0x14, d.CD.Drive+1) // 磁碟機字母（1 ＝ A:）
	d.M.Write8(base+0x15, 1)            // 子單位數
	d.M.Write8(base+0x16, 0xCB)         // RETF
}

// mscdex 是 `int 2Fh AH=15h`（DOSBox-X `MSCDEX_Handler`）。CF 先清，出錯才設。
func (d *DOS) mscdex(c *cpu.CPU) {
	cd := d.CD
	clearCarry(c)
	data := cpu.Addr(c.Seg[cpu.ES], c.R[cpu.BX])
	switch c.R[cpu.AX] {
	case 0x1500: // 安裝檢查：BX ＝ 光碟機數、CX ＝ 第一台
		c.R[cpu.BX] = 1
		c.R[cpu.CX] = uint16(cd.Drive)
		setAL(c, 0xFF)
	case 0x1501: // 每台一筆：子單位 ＋ device header 的 far 指標
		d.writeCDHeader()
		d.M.Write8(data, 0)
		d.M.Write16(data+1, machine.MSCDEXHeaderOff)
		d.M.Write16(data+3, machine.StubSeg)
	case 0x150B: // 是不是光碟機
		c.R[cpu.AX] = 0
		if c.R[cpu.CX] == uint16(cd.Drive) {
			c.R[cpu.AX] = 0x5AD8
		}
		c.R[cpu.BX] = 0xADAD
	case 0x150C:
		c.R[cpu.BX] = mscdexVersion
	case 0x150D: // 光碟機字母清單（0 ＝ A:）
		d.M.Write8(data, cd.Drive)
	case 0x1510: // 送 device request
		if c.R[cpu.CX] != uint16(cd.Drive) {
			c.R[cpu.AX] = 0x0F // invalid drive
			setCarry(c)
			return
		}
		d.cdRequest(data)
	default:
		d.note(0x2F, 0x15, al(c))
		c.R[cpu.AX] = 0x01 // function number invalid
		setCarry(c)
	}
}

// cdRequest 處理一個 device request（DOSBox-X `MSCDEX_Interrupt_Handler`）。
// 狀態字在 +3：DONE（0100h）| 錯誤碼，播放中再加 BUSY（0200h）。
// 照參考實作，IOCTL 的錯誤碼不帶 8000h；沒實作的命令回 DONE。
func (d *DOS) cdRequest(req uint32) {
	cd := d.CD
	d.M.Write8(req+1, 0) // 子單位
	fn := d.M.Read8(req + 2)
	buf := cpu.Addr(d.M.Read16(req+0x10), d.M.Read16(req+0x0E))
	var errcode uint16
	switch fn {
	case 0x03: // IOCTL INPUT
		errcode = d.cdIOCTLIn(buf)
	case 0x0C: // IOCTL OUTPUT：退片、鎖門、重設、聲道、控制字串、關門
		if d.M.Read8(buf) > 0x05 {
			errcode = 0x03
		}
	case 0x83: // SEEK：停止，起點移過去
		start := read32(d, req+0x14)
		if d.M.Read8(req+0x0D) != 0 {
			start = redBookToLBA(start)
		}
		cd.stopLog(d)
		cd.playing, cd.paused = false, false
		cd.start, cd.length = start, 0
	case 0x84: // PLAY AUDIO
		start := read32(d, req+0x0E)
		if d.M.Read8(req+0x0D) != 0 {
			start = redBookToLBA(start)
		}
		cd.stopLog(d)
		cd.playing, cd.paused = true, false
		cd.start, cd.length = start, read32(d, req+0x12)
		cd.startStep = d.M.Steps
		cd.startLog(d)
	case 0x85: // STOP AUDIO：播放中是暫停，已停是重設
		if cur, playing := cd.pos(d); playing {
			cd.stopLog(d)
			cd.playing, cd.paused = false, true
			cd.length -= cur - cd.start
			cd.start = cur
		} else {
			cd.paused = false
			cd.start, cd.length = 0, 0
		}
	case 0x88: // RESUME AUDIO
		if cd.paused {
			cd.playing, cd.paused = true, false
			cd.startStep = d.M.Steps
			cd.startLog(d)
		}
	case 0x80, 0x82: // READ LONG：不讀資料扇區
		d.note(0x2F, 0x15, fn)
		errcode = 0x03
	}
	status := 0x0100 | errcode
	if _, playing := cd.pos(d); playing {
		status |= 0x0200
	}
	d.M.Write16(req+3, status)
	d.CDRequests = append(d.CDRequests, CDRequest{Step: d.M.Steps, Fn: fn, Status: status})
}

// redBookToLBA 把打包的 MSF（frame | 秒<<8 | 分<<16）換成 LBA。
func redBookToLBA(v uint32) uint32 {
	frames := (v>>16&0xFF)*60*75 + (v>>8&0xFF)*75 + v&0xFF
	if frames < cdFramePad {
		return 0
	}
	return frames - cdFramePad
}

// cdIOCTLIn 是 IOCTL INPUT 的子功能（DOSBox-X `MSCDEX_IOCTL_Input`）。回錯誤碼。
func (d *DOS) cdIOCTLIn(buf uint32) uint16 {
	cd := d.CD
	switch d.M.Read8(buf) {
	case 0x00: // device header 位址
		d.writeCDHeader()
		d.M.Write16(buf+1, machine.MSCDEXHeaderOff)
		d.M.Write16(buf+3, machine.StubSeg)
	case 0x01: // 讀頭位置
		cur, _ := cd.pos(d)
		switch d.M.Read8(buf + 1) {
		case 0: // HSG
			write32(d, buf+2, cur)
		case 1: // Red Book
			m, s, f := msf(cur + cdFramePad)
			d.M.WriteBytes(buf+2, []byte{f, s, m, 0})
		default:
			return 0x03
		}
	case 0x04: // 聲道設定：照 DOSBox-X 的預設，輸出 0..3、音量 FFh
		for ch := uint32(0); ch < 4; ch++ {
			d.M.Write8(buf+1+ch*2, uint8(ch))
			d.M.Write8(buf+2+ch*2, 0xFF)
		}
	case 0x06: // 裝置狀態
		st := uint32(1<<1 | 1<<2 | 1<<4 | 1<<8 | 1<<9)
		if _, playing := cd.pos(d); playing {
			st |= 1 << 10
		}
		write32(d, buf+1, st)
	case 0x07: // 扇區大小
		switch d.M.Read8(buf + 1) {
		case 0:
			d.M.Write16(buf+2, 2048)
		case 1:
			d.M.Write16(buf+2, 2352)
		default:
			return 0x03
		}
	case 0x08: // 光碟大小（frames）
		write32(d, buf+1, cd.LeadOut+cdFramePad)
	case 0x09: // 換片狀態：1 ＝ 沒換
		d.M.Write8(buf+1, 1)
	case 0x0A: // 光碟資訊：首軌、末軌、leadout（Red Book）
		m, s, f := msf(cd.LeadOut + cdFramePad)
		last := cd.Tracks[len(cd.Tracks)-1].Number
		d.M.WriteBytes(buf+1, []byte{cd.Tracks[0].Number, last, f, s, m, 0})
	case 0x0B: // 單軌資訊
		n := d.M.Read8(buf + 1)
		var t CDTrack
		found := false
		for _, x := range cd.Tracks {
			if x.Number == n {
				t, found = x, true
			}
		}
		var m, s, f, attr uint8
		if found {
			m, s, f = msf(t.Start + cdFramePad)
			attr = t.attr()
		}
		d.M.WriteBytes(buf+2, []byte{f, s, m, 0, attr})
	case 0x0C: // Q channel：attr、軌號（BCD）、index、相對與絕對 MSF
		cur, _ := cd.pos(d)
		t := cd.trackAt(cur)
		rm, rs, rf := msf(cur - t.Start)
		am, as, af := msf(cur + cdFramePad)
		bcd := t.Number/10<<4 | t.Number%10
		d.M.WriteBytes(buf+1, []byte{t.attr(), bcd, 1, rm, rs, rf, 0, am, as, af})
	case 0x0E: // UPC：沒有
		d.M.WriteBytes(buf+1, make([]byte, 9))
	case 0x0F: // 音訊狀態：暫停旗標、起點、終點（照 DOSBox-X，終點欄放長度 ＋ 150）
		_, playing := cd.pos(d)
		var paused uint16
		if cd.paused {
			paused = 1
		}
		d.M.Write16(buf+1, paused)
		var s, e [3]uint8
		if playing {
			s[0], s[1], s[2] = msf(cd.start + cdFramePad)
			e[0], e[1], e[2] = msf(cd.length + cdFramePad)
		}
		d.M.WriteBytes(buf+3, []byte{s[0], s[1], s[2], 0, e[0], e[1], e[2], 0})
	default:
		return 0x03
	}
	return 0
}

func read32(d *DOS, a uint32) uint32 { return uint32(d.M.Read16(a)) | uint32(d.M.Read16(a+2))<<16 }

func write32(d *DOS, a, v uint32) {
	d.M.Write16(a, uint16(v))
	d.M.Write16(a+2, uint16(v>>16))
}

// removable 是 `AX=4408h`（DOSBox-X `dos_ioctl.cpp` 的 case 0x08）：
// 硬碟（目前磁碟機）回 AX=1；MSCDEX 的光碟機可抽換，參考實作回 CF、AX=1；
// 不存在的磁碟機回 CF、AX=0Fh。回 true 表示已設好 CF，呼叫端直接返回。
//
// ⚠ 以前一律回 AX=0（每個代號都「可抽換」），三國志V 掃 26 個代號時
// 每一個都像有一台軟碟機。
func (d *DOS) removable(c *cpu.CPU) bool {
	drive := uint8(c.R[cpu.BX])
	if drive == 0 {
		drive = d.Drive
	} else {
		drive--
	}
	switch {
	case drive == d.Drive:
		c.R[cpu.AX] = 1
		return false
	case d.CD != nil && drive == d.CD.Drive:
		c.R[cpu.AX] = 0x01 // function number invalid
	default:
		c.R[cpu.AX] = 0x0F // invalid drive
	}
	setCarry(c)
	return true
}

// startLog 記下一段播放的開頭。
func (cd *CDROM) startLog(d *DOS) {
	cd.Plays = append(cd.Plays, CDPlay{FromStep: d.M.Steps, Start: cd.start, Length: cd.length, SPS: d.M.StepsPerSecondNow()})
}

// stopLog 在還在播的那一段記下停止的指令數。
func (cd *CDROM) stopLog(d *DOS) {
	if n := len(cd.Plays); n > 0 && cd.Plays[n-1].ToStep == 0 {
		cd.Plays[n-1].ToStep = d.M.Steps
	}
}

// cdSampleRate 與 cdSectorBytes：CD-DA 是 44.1 kHz、16 位元立體聲，一個扇區 2352 bytes ＝ 588 個樣本。
const (
	cdSampleRate  = 44100
	cdSectorBytes = 2352
)

// sectorOffset 回 LBA 在映像檔裡的位移；那一軌沒有位移或是資料軌回 −1。
func (cd *CDROM) sectorOffset(lba uint32) int64 {
	t := cd.trackAt(lba)
	if t.Data || t.Offset < 0 {
		return -1
	}
	return t.Offset + int64(lba-t.Start)*cdSectorBytes
}

// WriteWAV 把播放紀錄倒成 44.1 kHz 16 位元立體聲 WAV（`docs/spec/438-mscdex-cd-audio-dump`）。
// 時間軸從指令 0 到 endStep：每段播放從 FromStep 換算的樣本位置起，複製映像裡的音軌資料，
// 到 ToStep 或播滿長度為止；其餘是靜音。CD-DA 的取樣格式就是 WAV 的格式，不重取樣。
func (cd *CDROM) WriteWAV(path string, image io.ReaderAt, endStep uint64, sps float64) (played int, err error) {
	total := int64(float64(endStep) / sps * cdSampleRate)
	f, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}()
	data := total * 4
	hdr := make([]byte, 44)
	copy(hdr[0:], "RIFF")
	binary.LittleEndian.PutUint32(hdr[4:], uint32(36+data))
	copy(hdr[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(hdr[16:], 16)
	binary.LittleEndian.PutUint16(hdr[20:], 1) // PCM
	binary.LittleEndian.PutUint16(hdr[22:], 2)
	binary.LittleEndian.PutUint32(hdr[24:], cdSampleRate)
	binary.LittleEndian.PutUint32(hdr[28:], cdSampleRate*4)
	binary.LittleEndian.PutUint16(hdr[32:], 4)
	binary.LittleEndian.PutUint16(hdr[34:], 16)
	copy(hdr[36:], "data")
	binary.LittleEndian.PutUint32(hdr[40:], uint32(data))
	if _, err := f.Write(hdr); err != nil {
		return 0, err
	}
	if err := f.Truncate(44 + data); err != nil { // 靜音：未寫到的部分是 0
		return 0, err
	}
	buf := make([]byte, cdSectorBytes)
	for _, p := range cd.Plays {
		from := int64(float64(p.FromStep) / sps * cdSampleRate)
		n := int64(p.Length) * (cdSectorBytes / 4) // 樣本數
		if p.ToStep != 0 {
			if stop := int64(float64(p.ToStep-p.FromStep) / p.SPS * cdSampleRate); stop < n {
				n = stop
			}
		}
		if from+n > total {
			n = total - from
		}
		for i := int64(0); i < n; i += cdSectorBytes / 4 {
			off := cd.sectorOffset(p.Start + uint32(i/(cdSectorBytes/4)))
			if off < 0 {
				break
			}
			k := min(int64(cdSectorBytes/4), n-i)
			if _, err := image.ReadAt(buf[:k*4], off); err != nil {
				return played, fmt.Errorf("讀映像 %d：%w", off, err)
			}
			if _, err := f.WriteAt(buf[:k*4], 44+(from+i)*4); err != nil {
				return played, err
			}
		}
		played++
	}
	return played, nil
}
