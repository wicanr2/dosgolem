# 434 — MSCDEX 最小集：光碟機偵測、曲目資訊與播放狀態

狀態：**READY**
日期：2026-10-18
前置：[`004-dos-bios-services`](004-dos-bios-services.md)、[`011-xms`](011-xms.md)（`int 2Fh` 的分派）
案例：三國志V（san5）`MAIN.EXE`、`OPEN.EXE`

---

## 1. 為什麼要做

`int 2Fh AH=15h` 沒有實作時，`AX=1500h` 的 BX 不動，程式判定沒有光碟機。三國志V 的 CD 模組
（`main:12F5F`）因此初始化失敗。標題選單之前的 `ov12:1021E` 要求放入光碟、再以 `main:20178`
找光碟機，找不到就結束（量測：`wicanr2/san5-remake` 的 `docs/re/012-dosgolem-san5-branch.md`）。

`main:20012` 另外核對光碟身分：末軌要是第 21 軌，第 2 到 21 軌的長度（分:秒）與內建表相差不到
6 秒。所以光碟機要回答真實光碟的曲目表，不能是空的或假的。

## 2. 範圍

依據 DOSBox-X `src/dos/dos_mscdex.cpp`（commit `262a304f768d5d8e93ef4abcc479ea7175aef183`）的
`MSCDEX_Handler`（`:1460`）、`MSCDEX_Interrupt_Handler`（`:1306`）、`MSCDEX_IOCTL_Input`（`:1106`）。

- 只在呼叫端設了 `DOS.CD` 時生效。沒設時與以前相同。
- 一台光碟機、一個子單位。曲目表由呼叫端給（`dos.ParseCDTOC`，`probe -cdrom D:<檔>`）：
  每行 `<軌號> audio|data <LBA>`（可加第四欄映像位移，見 `438`），加一行 `leadout <LBA>`，軌號從 1 連續。
- 不讀資料扇區、不出聲音。播放只維護位置與 BUSY。

## 3. 語意

### 3.1 `int 2Fh`

CF 先清，出錯才設（DOSBox-X 同）。

| AX | 回傳 |
|---|---|
| `1500h` | BX = 1，CX = 光碟機代號（0 = A:），AL = FFh |
| `1501h` | ES:BX 寫一筆：子單位 0、device header 的 far 指標 |
| `150Bh` | CX 是光碟機：AX = 5AD8h；否則 0。BX = ADADh |
| `150Ch` | BX = 0217h（2.23，DOSBox-X 的版本） |
| `150Dh` | ES:BX 寫光碟機代號（1 byte） |
| `1510h` | CX 不是光碟機：AX = 0Fh、CF。否則處理 ES:BX 的 device request |
| 其他 `15xxh` | 記為未實作，AX = 01h、CF |

device header 在 `StubSeg:04B0`（`machine.MSCDEXHeaderOff`）：next = FFFF:FFFF，屬性 C800h，
strategy 與 interrupt 都指向 header 後一個 `RETF`，名稱 `MSCD001 `，+14h 磁碟機字母（1 = A:），
+15h 子單位數 1。

### 3.2 device request

request +2 是命令，+3 寫回狀態字。IOCTL 的緩衝區 far 指標在 +0Eh（位移）、+10h（段）。

狀態字 = 0100h（DONE）| 錯誤碼；播放中再 | 0200h（BUSY）。照參考實作，IOCTL 的錯誤碼
不加 8000h，沒實作的命令回 DONE。

| 命令 | 動作 |
|---|---|
| `03h` IOCTL INPUT | 見 3.3 |
| `0Ch` IOCTL OUTPUT | 子功能 0..5 成功，其餘錯誤碼 03h |
| `83h` SEEK | 停止播放，起點設成 +14h（+0Dh 非 0 時是打包的 MSF） |
| `84h` PLAY AUDIO | 起點 +0Eh（同上）、長度 +12h（frames），開始播放 |
| `85h` STOP AUDIO | 播放中：暫停，起點移到目前位置、長度扣掉已播的。已停：重設起點與長度為 0 |
| `88h` RESUME AUDIO | 暫停中才恢復 |
| `80h`、`82h` READ LONG | 不讀資料：錯誤碼 03h，記為未實作 |

播放位置 = 起點 + (目前指令數 − 開始時指令數) ÷ `StepsPerSecondNow()` × 75。播滿長度就停。

### 3.3 IOCTL INPUT 子功能

| 子功能 | 內容 |
|---|---|
| 00h | device header 的 far 指標 |
| 01h | 讀頭位置：+1 = 0 時 HSG（LBA），= 1 時 Red Book（frame、秒、分、0）；其他錯誤碼 03h |
| 04h | 聲道：輸出 0..3、音量 FFh |
| 06h | 裝置狀態（dword）：位元 1、2、4、8、9 恆為 1，播放中加位元 10 |
| 07h | 扇區大小：+1 = 0 回 2048，= 1 回 2352 |
| 08h | 光碟大小（frames）= leadout + 150 |
| 09h | 換片狀態 1（沒換） |
| 0Ah | 首軌、末軌、leadout 的 frame、秒、分、0 |
| 0Bh | +1 指定軌號：起點的 frame、秒、分、0，attr（資料軌 40h、音軌 00h）。軌號不存在時全 0 |
| 0Ch | Q channel：attr、軌號（BCD）、index 1、相對 MSF、0、絕對 MSF |
| 0Eh | UPC：全 0 |
| 0Fh | 暫停旗標（word）、播放中時起點 MSF 與「長度 + 150」的 MSF（照參考實作的欄位用法） |
| 其他 | 錯誤碼 03h |

MSF 一律以「LBA + 150」換算（Red Book 的 2 秒前導）。

## 4. 已知差異

| 項目 | 本實作 | 來源 | 分類 |
|---|---|---|---|
| 聲音 | 不即時出聲；倒成 WAV 見 [`438-mscdex-cd-audio-dump`](438-mscdex-cd-audio-dump.md) | DOSBox-X 播放影像檔的音軌 | remake-owned。到期條件：有通用的即時前端時 |
| 資料扇區與光碟上的檔案 | 不支援，光碟機代號不能開檔 | DOSBox-X 掛 ISO 或目錄 | remake-owned。三國志V 的資料全在硬碟。到期條件：遇到從光碟讀檔的程式 |
| 播放狀態與狀態檔 | 播放狀態不存進狀態檔；`-load-state` 之後要重給 `-cdrom` | 參考實作沒有狀態檔 | remake-owned。到期條件：要跨狀態檔對拍 CD 音樂時 |
| `AH=11h` 網路重導偵測 | 不處理 | DOSBox-X 對 DADAh 回 ADADh | 沒有觀測到的程式使用 |
| strategy／interrupt 進入點 | 只有 `RETF` | DOSBox-X 是 callback | 程式直接呼叫會什麼都不做。到期條件：遇到直接呼叫的程式 |

## 5. 驗收

`internal/dos/mscdex_contract_test.go`：

1. `1500h`、`150Dh`、`150Ch`（至少 0200h）、`150Bh`、`1501h` 與 header 欄位。
2. 沒設 `DOS.CD` 時 `1500h` 的 BX 仍是 0。
3. IOCTL 0Ah 與 0Bh 的 MSF 換算（含資料軌 attr）。
4. 播放、半秒後暫停、恢復、播完自己停、已停時 STOP 重設。
5. 曲目表跳號或缺 leadout 時 `ParseCDTOC` 回錯誤。
6. 負對照：`1500h` 改回 BX = 0，第 1 條必須失敗（2026-10-18 手動確認後還原）。

上層收據：三國志V 以 `-cdrom D:<曲目表>` 跑，`main:12F5F` 初始化成功、`main:20012` 光碟身分核對回 1，
在放入光碟的提示按確認後進到標題選單。
