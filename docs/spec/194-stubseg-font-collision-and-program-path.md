# 194 — StubSeg 字型 stub 撞到 INT 9 預設 stub，與可指定的程式路徑

狀態：**READY**
日期：2026-09-30
前置：[`004-dos-bios-services.md`](004-dos-bios-services.md)（服務層與 stub）、
[`008-wolong-services.md`](008-wolong-services.md)（DOS/V 字型服務）、
[`003-machine-and-loader.md`](003-machine-and-loader.md)（環境區塊與程式路徑）
修訂：§1.2 佈局表依 447、430 與 [`459`](459-mouse-irq12-v86-delivery.md) 補列（各註「由 459 修訂」，2026-10-23）。

---

## 1. 問題一：`int 15h AX=5000h` 的字型 stub 蓋掉向量 8、9 的預設 stub

`machine.initVectors` 為每個向量在 `StubSeg:StubOff(n)`（`n × 4`）種一段
`CD n / CF`，向量表指向它。程式「存舊向量 → 裝自己的 → 做完 jmp far 舊向量」
時，舊向量就是這段 stub，服務層用 `fromOwnStub` 在這裡做預設動作後 `iret`。

`internal/dos/font.go` 把兩段字型 stub 種在 `0x20`、`0x24`：

| 位移 | 原本是 | 被蓋成 |
|---|---|---|
| `0x20` | 向量 8 的 `CD 08 CF` | `CD F7 CB`（`int F7h; retf`） |
| `0x24` | 向量 9 的 `CD 09 CF` | `CD F8 CB`（`int F8h; retf`） |

向量 8 另指 `biosTimerOff`，所以 `0x20` 目前沒人跳過去。**向量 9 沒有**：
程式的鍵盤中斷處理常式 chain 回舊 INT 9 時跑到的是 `int F8h; retf`。
`retf` 只彈 CS:IP，不彈 FLAGS，**每按一個鍵堆疊就少 2 byte**。

### 1.1 量到的證據（已證實）

《魔眼殺機二》中文版 `START.EXE`（智冠，SHA-256 見 eob_123_remake 的研究紀錄）：

- 鍵盤處理常式 `2534:0860` 結尾 `pop ×8; jmp far cs:[020B]`，舊向量 `0080:0024`。
- 執行當下 `0080:0020` 的內容是 `CD F7 CB 00 CD F8 CB 00`。
- 中斷前 `SP=0FA2`；處理常式彈完後 `SP=0F9C`；經 `0080:0024`／`0026` 回到被中斷處
  `SP=0FA0`，少 2。
- 被中斷的是 `43BC:117C` 的圖形編碼函式（入口 7 個 push、結尾 7 個 pop），
  結尾 `retf` 因此讀錯位置，回到 `0009:0A95`，之後走進中斷向量表執行垃圾。
- 把字型 stub 移出 `0x000–0x3FF` 之後，同一條路徑跑滿 6,000 萬道指令不再當掉。

### 1.2 修法

字型 stub 移到 `specialStubBase` 之後的空位。StubSeg 的佈局後來收歸 `internal/machine`
的一個 const 區塊統一分配（eob1 分支 `defb9c2`），`internal/dos` 只引用名字：

| 位移 | 內容 |
|---|---|
| `0x400–0x40F` | 四個 trampoline |
| `0x410` | 回呼返回哨兵 `CD F9` |
| `0x414`／`0x418` | DOS/V 字型 entry（`CD F7 CB`、`CD F8 CB`） |
| `0x420–0x442` | BIOS `int 08h`（35 bytes，含 40:40 馬達倒數）；保護模式剖面 `0x420–0x447`（40 bytes，加 EOI，459 §3.3a；由 459 修訂，2026-10-23） |
| `0x448` | `int 15h AH=C0h` 系統設定表 |
| `0x450` | InDOS 旗標 |
| `0x460` | DBCS 前導位元組表 |
| `0x480` | 國別資訊表 |
| `0x4B0–0x4B7` | 可掛鉤的 XMS 入口（430 §2.3） |
| `0x4B8–0x4C5` | BIOS `int 09h` 鍵盤攔截客體碼（447 §2.2，14 bytes）；保護模式剖面 `0x4B8–0x4CA`（19 bytes，加 EOI，459 §3.3a；由 459 修訂，2026-10-23） |
| `0x4CC–0x4DC`、`0x4DD–0x4E8`、`0x4E9–0x4EC` | IRQ12 樁 `Mouse74Off`、返回樁 `Mouse74RetOff`、使用者回呼返回樁 `MouseUIROff`（只在保護模式剖面寫入，459 §3.6；由 459 修訂，2026-10-23） |

現行佈局以 `internal/machine/machine.go` 的 const 區塊為準（`5b88abd` 的 121–133 行註解與 140–198 行常數）。上界是 `0x4F0`。（由 459 修訂，2026-10-23）

系統設定表一度放在 `0x440`。計時器 stub 從 24 bytes 長到 35 bytes 之後，設定表會蓋到
stub 結尾，所以挪到 `0x448`。

`int 15h AX=5000h` 回的 `ES:BX` 跟著改；呼叫端拿的是指標，不寫死位移，行為不變。

### 1.3 同類的重疊（已隨 §1.2 的佈局一起搬走）

| 原位移 | 用途 | 壓到的向量 stub | 現在 |
|---|---|---|---|
| `0x30` | `CallbackRetOff` | 向量 `0Ch` | `0x410` |
| `0x40` | DBCS 前導位元組表 | 向量 `10h` | `0x460` |
| `0x80` | 國別資訊表 | 向量 `20h` | `0x480` |

eob1 分支踩到過同一類事故（字型 stub 壓到向量 09h，`docs/re/eob1-dosgolem-handoff-255.md`）
之後一起搬走。這會改變 StubSeg 裡的記憶體內容，以記憶體雜湊為準的舊收據要重錄。
`TestPerVectorStubsAreNeverOverwritten` 釘住 256 格向量 stub 不被任何服務蓋到。

### 1.4 驗收

- `StubSeg:StubOff(9)` 在 `Install` 之後仍是 `CD 09 CF`。
- 字型 stub 不落在 `0x000–0x3FF`，且 `int 15h AX=5000h` 回的指標指到 `CD F7／F8 CB`。
- 既有測試全綠（字型服務的臥龍傳路徑）。

## 2. 問題二：程式路徑只能用中性預設值

`machine.Machine.ProgramPath` 是環境區塊尾端的程式全路徑（argv[0]）。
`oracle` 與 `cmd/probe` 都沒有設定入口，所以每支程式拿到 `C:\PROG.EXE`。

Borland overlay 管理員（EOB2 的 `START.EXE`）用 argv[0] 重開自己讀 overlay；
名字不對就開檔失敗（`找不到的檔：C:\PROG.EXE`）。

### 2.1 修法

- `oracle.Options.ProgramPath`：非空時在 `LoadEXE` 之前寫進 `Machine.ProgramPath`。
- `cmd/probe -program-path`：同上。
- 預設值不變（仍是中性的 `C:\PROG.EXE`），既有 app 與收據不受影響。
  **不從執行檔檔名自動推導**：推導會改變所有既有程式的環境區塊內容。

### 2.2 驗收

- 設定 `ProgramPath` 後，環境區塊尾端是指定字串；不設時維持 `C:\PROG.EXE`。
