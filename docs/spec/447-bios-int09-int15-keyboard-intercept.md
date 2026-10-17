# 447：BIOS int 09h 預設處理改為客體碼，呼叫 `int 15h AH=4Fh` 鍵盤攔截

狀態：**READY**（第 2 版，依審查修訂）
日期：2026-10-17
前置：[`014-hardware-keyboard`](014-hardware-keyboard.md)、[`197-key-hold-and-typematic`](197-key-hold-and-typematic.md)、
[`202-bios-int09-default-handler`](202-bios-int09-default-handler.md)、[`435-v86-hooks-and-idt-delivery`](435-v86-hooks-and-idt-delivery.md)
修訂：202 §2「向量 9 指向 stub 時由 `handle` 的 `0x09` 分支翻譯」在本規格選項開啟時不適用（§2.3）。

DOSBox-X 引用一律指 `DOSBox-X-MCP-Debugger/dosbox-src/` 下的檔案，行號以 DOSBox-X commit `e0b4287` 為準。
dosgolem 的程式位置以 commit `ee695e9` 為準。推論等級：已證實、強推論、假說、未知。

---

## 1. 為什麼要做

### 1.1 觸發案例

《Strike Commander》的 `STRIKE.EXE` 在飛行中對 IRQ1 的按鍵沒有反應。以下 log 都在 `strike_commander` 的 `workplace/logs/`。

| 事實 | 來源 | 等級 |
|---|---|---|
| 飛行中 `IVT[09h]` 為 `0080:0024`（dosgolem 的 per-vector stub），STRIKE 沒有掛 int 09h | `probe475.log` 第 10 行、`probe475p.log` | 已證實 |
| `-hold left@6600000000+300000000`：定時事件 279 個，`KeyIRQs`＝0，畫面與不送鍵的對照逐張相同 | `probe474a.log`、`probe474b.log` 與兩組截圖（`workplace/scratch/p474a`、`p474b`，各自的目錄） | 已證實 |
| 擋下的原因：`timedKeyTick`（`internal/machine/keyhold.go` 103 行）與 `keyTick`（`keyboard.go` 113 行）在 `IVT[09h]` 的段為 `StubSeg` 時不送，只累加 `keyStalls` | 程式碼 | 已證實 |
| 飛行中 1 億步內服務呼叫只有 `int 1Ch AH=00` ×157 與 `int 16h AH=01` ×149 | `probe475.log` 第 40–42 行（`-log-calls`） | 已證實 |
| 飛行中 `IVT[15h]`＝`69EE:000F`；`69EE:000A` 存的舊向量是 `0080:0408`（dosgolem 的 int 15h trampoline `CD F4 CF`） | `probe475p.log`（`-peek`） | 已證實 |

### 1.2 STRIKE 的鍵盤攔截

`69EE:000F` 起的位元組與 `STRIKE.EXE` 檔案位移 `0x6FCDF` 相同（`9C 80 FC 4F 74 06 9D 2E FF 2E 0A 00 3C E0 75 0B …`）。主代理以
`objdump -D -b binary -m i8086` 反組譯檔案 `0x6FC40`–`0x6FD80`（本機，不進版控）：

| 檔案位移 | 內容 | 等級 |
|---|---|---|
| `6FCDF` | `pushf; cmp ah,4Fh; je 6FCEB`；不是 `4Fh` 就 `popf; jmp far cs:[000Ah]` | 強推論 |
| `6FCEB` | `AL＝E0h` 時存進 `cs:[000Eh]`，`popf; stc; jmp far cs:[000Ah]` | 強推論 |
| `6FCFA` | 其餘：`cli`，`BL＝AL`，換到 `6C4C` 段自己的堆疊，`[42BAh]` 的遠指標非 0 時 `push bx; lcall [42BAh]`，之後 `popf; stc; jmp far cs:[000Ah]` | 強推論 |
| `6FD5E` | `int 16h AH=01h`，有鍵就 `AH=00h` 讀掉，直到緩衝空 | 強推論 |

STRIKE 不掛 int 09h，改在 `int 15h AH=4Fh`（BIOS 鍵盤處理對每個掃描碼呼叫的攔截點）取得每一個按下與放開碼，交給遊戲的按鍵常式，
再以 `stc` chain 回舊向量，讓 BIOS 照常把鍵放進緩衝；遊戲另以 `6FD5E` 清緩衝。

dosgolem 沒有這個呼叫：`int 09h` 的預設處理是 Go 端的 `biosInt9`（202），它直接翻譯掃描碼放進 BDA 緩衝，不經過 `int 15h`。
即使 IRQ1 送得出去，STRIKE 的按鍵常式也不會被呼叫。

### 1.3 真機與 DOSBox-X

IBM PC/AT BIOS 的 int 09h 在讀到掃描碼後以 `AH=4Fh`、`CF=1` 呼叫 `int 15h`，回來 `CF=0` 就丟掉這個碼，`CF=1` 則用回傳的 `AL` 繼續處理。
DOSBox-X 的預設 IRQ1 處理是一段客體碼（`src/cpu/callback.cpp` 413–475 行，`CB_IRQ1`）：

```text
push ax / in al,60h / mov ah,4Fh / stc / int 15h / jnc skip / <callback IRQ1_Handler> /
skip: cli / mov al,20h / out 20h,al / pop ax / iret
```

`src/ints/bios_keyboard.cpp` 1726–1748 行的註解寫「`jc skip`」，實際寫入的位元組是 `73 04`，即 `jnc`。`IRQ1_Handler` 以 `reg_al` 為掃描碼
（同檔 601–605 行）。`int 15h AH=4Fh` 的預設服務設 `CF=1`（`src/ints/bios.cpp` 6963–6966 行）。

## 2. 規則

### 2.1 選項

- `internal/machine` 的 `Machine` 新增欄位 `KbdIntercept bool`（不進狀態檔）與方法 `EnableKbdIntercept() error`。方法寫入 §2.2 的客體碼，
  逐 byte 讀回驗證，全部相符才設欄位為 true 並回 nil；不符回錯誤、欄位維持 false。可重複呼叫，結果相同。
- 寫入與讀回走 `WriteSup8`／`ReadSup8`（客體線性視角、監督者身分；轉換關閉時等同 `Write8`／`Read8`）。理由：V86 客體執行的是線性位址上的
  位元組，IVT 檢查也用 `ReadSup16`（435 §4）；`WriteBytes` 經 `Write8` 在轉換開啟且 `cplUser` 時可能因頁面權限被靜默丟棄
  （`internal/machine/linear.go` 230–235 行）。
- `cmd/probe` 加 `flag.Bool` 旗標 `-kbd-int15`（說明文字引用本規格），在 `-load-state` 載入之後、送鍵排程之前呼叫 `m.EnableKbdIntercept()`，錯誤即結束。
- 不一致偵測：probe 在 `-load-state` 之後、選項關閉時讀 `StubOff(9)` 起 3 bytes；為 `E9 91 04` 就以錯誤結束（狀態檔是在選項開啟時存的）。
- `-kbd-int15` 與任何呼叫 `feedKeys` 的旗標（`-keys`、`-keys-at`）同時給時 probe 以錯誤結束：`feedKeys` 同時排進 `d.Stdin` 與硬體佇列，
  開啟本選項後硬體佇列的鍵也會進 BDA 緩衝，程式可能讀到兩次。`-bios-keys`、`-bios-key-name(s)` 直接寫 BDA，不經硬體路徑，不擋。
- 預設關閉，關閉時行為與現行完全相同。`session`、`oracle` 與遊戲前端的開關另立規格。

### 2.2 客體碼

`StubSeg` 佈局新增具名常數 `KbdInterceptOff = specialStubBase + 0xB8`（`0x4B8`），`machine.go` 的佈局註解表加上 `0x4B8–0x4C5`，
`internal/dos` 只引用這個名字。寫入 14 bytes：

| 位移 | 位元組 | 指令 |
|---|---|---|
| `4B8` | `50` | `push ax` |
| `4B9` | `E4 60` | `in al,60h` |
| `4BB` | `B4 4F` | `mov ah,4Fh` |
| `4BD` | `F9` | `stc` |
| `4BE` | `CD 15` | `int 15h` |
| `4C0` | `73 02` | `jnc 4C4` |
| `4C2` | `CD FA` | `int FAh` |
| `4C4` | `58` | `pop ax` |
| `4C5` | `CF` | `iret` |

- 向量 9 的 per-vector stub 改寫：`StubOff(9)`（`0x24`）起 3 bytes 為 `E9 91 04`（`jmp near 04B8h`，下一道在 `0x27`，`0x4B8−0x27＝0x491`），
  蓋掉原本的 `CD 09 CF`；`0x27` 是填充，不寫。`IVT[09h]` 不改：沒掛 int 09h 的程式與掛了再 chain 回 `0080:0024` 的程式都走到這段碼。
- 這是 `internal/dos/stub_layout_test.go` 的 `TestFontStubsLeaveVectorStubsIntact`、`TestPerVectorStubsAreNeverOverwritten` 所守不變量
  （舊向量 stub 不被蓋）的唯一例外：被蓋的仍是向量 9 自己的預設行為，chain 回來的程式得到的是更完整的 BIOS 處理。兩個測試只在選項關閉時成立，
  測試內註明。
- 不送 EOI。dosgolem 一般機器不追蹤 IRQ1 的 ISR，BIOS int 08h 的客體碼（`biosTimerOff`）也不送 EOI；444 的非特定 EOI 會清 ISR7
  （`sb_dma8.go` 775–797 行），在這裡送會清錯位元。與 DOSBox-X 的差異記在 §4。
- 不 `sti`，與 DOSBox-X 相同。

### 2.3 `int 09h` 與 `int FAh`

- `internal/dos` 新增常數 `intKbdTranslate = 0xFA`，`handle` 的 switch 加一支：以 `AL` 為掃描碼做 202 §2 的翻譯並放進緩衝。
- 202 的 `biosInt9` 改為讀埠 `60h` 之後呼叫共用的 `biosInt9Code(code uint8)`；`int FAh` 也呼叫它。翻譯規則不變。
- `KbdIntercept` 為 true 時，`handle` 對 `n＝09h` 回 false，讓 CPU 照 `IVT[09h]` 進入 §2.2 的客體碼。否則實模式的軟體 `int 09h` 會被 Go 直接翻譯、
  跳過 `int 15h`，與 V86 的路徑不同。選項關閉時 `0x09` 分支照舊。
- `int FAh` 不在 `StubOff(0xFA)`，`writeBackFlags` 不寫旗標。`IVT[FAh]` 被程式改走時 `handle` 回 false，CPU 跳到程式的向量，實作註解記一筆。
- `int FAh` 不受 `KbdIntercept` 限制：選項關閉時沒有客體碼會執行它。

### 2.4 `int 15h AH=4Fh`

`KbdIntercept` 為 true 時，`int15`（`internal/dos/font.go`）在落到「記未實作並清 CF」之前，對 `AH=4Fh` 設 `CF=1`、`AL` 不變、不記未實作，
`KbdIntercepts` 加 1。經 `F4h` trampoline 進來時由既有的 `fixStackedCF` 寫回堆疊框；Go 直接服務時 CF 留在 `c.Flags`。選項關閉時維持現行。

### 2.5 IRQ1 遞送

`KbdIntercept` 為 true 時，`keyTick` 與 `timedKeyTick` 不做「`IVT[09h]` 段為 `StubSeg` 就不送」的檢查。IF、8259 遮罩與 433 的 IDTR 守門照舊，
相對次序不變。V86 下 `ivtGuardIRQ` 會先呼叫 `prepareV86Delivery`（`sysregs.go` 108–118 行）；過去被 stall 擋下時走不到這一步，現在會走到。

### 2.6 可觀測性

`DOS` 新增診斷計數（不進狀態檔）：`KbdIntercepts`（`int 15h AH=4Fh` 由 Go 服務的次數）、`KbdTranslates`（`int FAh` 次數）。
`cmd/probe` 在 `-kbd-int15` 開啟時另印一行，不論 `KeyIRQs`、`KeyEvery`、佇列是否為 0：`KeyIRQs`、`KeyStalls()`、`TimedKeysPending()`、
`KbdIntercepts`、`KbdTranslates`。

### 2.7 範圍外

- 延伸前綴 `E0`、`E1`、Pause、PrintScreen、Ctrl-Break（`int 1Bh`）、Num Lock 與 LED，維持 202 §3。
- `int 15h AH=4Fh` 之外的 BIOS 鍵盤回呼（`AH=85h` SysReq、`AH=91h`）。
- LE 機器（`le_hardware_keyboard_irq1.go`）不改。
- `session`、`oracle`、遊戲前端的開關。

## 3. 驗收

1. 選項關閉時：`go test ./internal/... ./cmd/probe ./session ./oracle` 與 `go test ./apps/...` 結果與改動前相同；錨點常數不變；SingleStepTests 不得 skip。
2. 單元測試，實模式，選項開啟：
   - 程式不掛 int 09h，`QueueKey`、`HoldKey` 各一組：IRQ1 送出（`KeyIRQs` 增加、`KeyStalls` 不增），BDA 緩衝得到 202 的翻譯結果；放開碼不進緩衝。
     中斷前後 AX、SP、FLAGS（含 IF）與被中斷的 CS:IP 不變。
   - 程式掛 `int 15h`，處理常式記下每次入口的 `AH`、`AL`、`CF` 後 chain 回舊向量：按下與放開碼都依序看到，`AH=4Fh`、`AL` 為掃描碼、`CF=1`。
   - 處理常式回 `CF=0`（不 chain，改堆疊旗標後 `iret`）：緩衝沒有鍵，`KbdTranslates` 不增。
   - 處理常式把堆疊框的 CF 清成 0 之後 chain 回舊向量：服務設回 `CF=1`，緩衝有鍵（抓「拿掉 `AH=4Fh` 分支或 `fixStackedCF`」）。
   - 處理常式把 `AL` 改成另一個掃描碼並回 `CF=1`：緩衝得到新碼的翻譯。
   - 程式掛 int 09h、讀 `60h` 後 `pushf; call far` 舊向量：照樣經 `int 15h` 並放進緩衝。
   - 軟體 `int 09h`（`IVT[09h]` 在 stub）：經客體碼與 `int 15h`，不由 `0x09` 分支直接翻譯。
   - `int 15h` 未掛時直接由 Go 服務：`KbdIntercepts` 增加，`CF=1`。
   - `EnableKbdIntercept` 兩次：記憶體與一次相同；`StubOff(9)` 起 3 bytes 為 `E9 91 04`，`KbdInterceptOff` 起 14 bytes 照 §2.2。
   - 選項關閉：`StubOff(9)` 仍為 `CD 09 CF`，不掛 int 09h 時 IRQ1 不送、`KeyStalls` 增加（現行行為）。
   - 202 的三個測試照舊通過。
3. 單元測試，V86（沿用 `internal/dos/v86_test.go` 的工具），選項開啟：
   - `IVT[15h]` 被 V86 程式掛走時，客體碼的 `int 15h` 交給 CPU 經 IDT 遞送（`handle` 回 false），chain 回 `0080:0408` 時 `fixStackedCF` 寫回 `CF=1`。
   - `int FAh` 在 `cs＝StubSeg` 時被 `handle` 攔下並翻譯。
   - 分頁開啟、線性 0 頁為監督者限定時，`EnableKbdIntercept` 仍寫入成功（讀回相符）；頁面不存在時回錯誤、欄位為 false。
4. probe：`-kbd-int15` 旗標預設 false，開啟後欄位為 true；與 `-keys`、`-keys-at` 並用回錯誤；選項關閉讀到帶 `E9 91 04` 的狀態檔回錯誤。
5. 突變自檢，各至少一個測試失敗：拿掉 stall 檢查的選項條件、`jnc` 改 `jc`、拿掉 `push ax`／`pop ax`、拿掉 `stc`、拿掉 `in al,60h`、
   `int FAh` 改讀埠 `60h`、`AH=4Fh` 改回清 CF、不改寫 `StubOff(9)`、jmp 位移錯、`handle` 的 `0x09` 不讓位、V86 不檢查 `cs＝StubSeg`、
   `EnableKbdIntercept` 改用 `WriteBytes`。
6. `go build ./...`、`go vet ./internal/... ./cmd/probe`、`gofmt -l` 無輸出。
7. 觸發案例（需原版，另附 probe 收據）：從 `strike_commander` 的 `workplace/scratch/p474a/s660.state`（不送鍵的那次存下的飛行狀態）以
   `-kbd-int15 -hold left@6600000000+300000000` 跑到第 6900000000 步。記錄載入時的 `TimedKeysPending()` 基準與載入後新增的
   `KeyIRQs`、`KeyStalls`、`KbdIntercepts`、`KbdTranslates`、`int 16h` 的呼叫數，以及與不送鍵的對照在同步數的截圖差異。
   V86 下 `in al,60h`、`int 15h`、`int FAh` 的實際路徑照實記錄。

## 4. 已知限制與差異

- 不送 EOI（§2.2）。程式自己的 int 09h chain 回 BIOS、期待 BIOS 送 EOI 的情形，在 dosgolem 不需要 EOI 也能繼續收 IRQ1。
- 狀態檔存整份記憶體：選項開啟時存下的狀態帶著 §2.2 的客體碼。probe 會擋下選項關閉時讀入（§2.1）；其他前端讀入時按鍵會因 `AH=4Fh` 清 CF 而
  不進緩衝。
- `-hold` 的 typematic 重複送按下碼（197），BIOS 與 `int 15h` 攔截照真機各收一次。
