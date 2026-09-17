package machine

// 系統 BIOS ROM（`F0000`–`FFFFF`）的最後 16 bytes 與唯讀語意
// （`docs/spec/191-bios-rom-tail`）。
//
// **這台機器沒有真的 BIOS 程式碼**——服務都在 Go 這一端，向量指到
// `StubSeg` 的 stub。但 ROM 尾巴那 16 bytes 是程式會**當資料讀**的：
// 機型位元組 `FFFFE`、BIOS 日期 `FFFF5`。更麻煩的是讀到那裡的意外：
// 把 `FFFF:FFFF` 當物件指標的程式，16 位元位移 1–16 落在這一段。
// 真機那裡不是 0，我們以前是 0，於是《銀河英雄傳說III SP》的戰略全滅
// 讀到 `AH = 0`、一路把資料段寫壞（`~/cht/logh3/docs/re/334`）。

// ROMBase 是系統 BIOS ROM 的起點。寫進 `[ROMBase, MemSize)` 的位元組一律忽略。
const ROMBase = 0xF0000

// romTail 是 `FFFF0`–`FFFFF` 開機時的內容：
//
//	FFFF0  EA 5B E0 00 F0   重置進入點 `jmp far F000:E05B`
//	FFFF5  "01/01/92"       BIOS 日期（MM/DD/YY）
//	FFFFD  00
//	FFFFE  FC               機型位元組（PC/AT）
//	FFFFF  55               DOSBox 自己的簽章位元組
//
// 取值依原版執行環境（js-dos 的 DOSBox，`machine=svga_s3`）；
// 參考 DOSBox-X `src/ints/bios.cpp` 的 `write_FFFF_signature`，只讀、不照抄。
var romTail = [16]byte{
	0xEA, 0x5B, 0xE0, 0x00, 0xF0,
	'0', '1', '/', '0', '1', '/', '9', '2',
	0x00, 0xFC, 0x55,
}

// initROM 把 ROM 尾巴填好。**讀狀態檔之後也要再填一次**：ROM 不是機器
// 狀態，舊版執行器存下來的記憶體那一段是 0。
func (m *Machine) initROM() {
	copy(m.Mem[MemSize-len(romTail):], romTail[:])
}

// ROMWrite 記一次被忽略的 ROM 寫入。
type ROMWrite struct {
	Step   uint64
	Addr   uint32
	Val    uint8
	CS, IP uint16
}

// noteROMWrite 記下被忽略的寫入：總數，加上最前面幾筆的位置。
//
// **忽略不等於沒發生**：真機上寫 ROM 是無聲的，但一支程式寫 ROM 通常
// 代表它的指標已經飛了。只算次數看不出是誰，只留位置又會被灌滿，
// 所以兩個都留。
func (m *Machine) noteROMWrite(a uint32, v uint8) {
	m.ROMWrites++
	if len(m.ROMWriteLog) < maxROMWriteLog {
		cs, ip := m.CPU.OpAddr()
		m.ROMWriteLog = append(m.ROMWriteLog, ROMWrite{m.Steps, a, v, cs, ip})
	}
}

const maxROMWriteLog = 20
