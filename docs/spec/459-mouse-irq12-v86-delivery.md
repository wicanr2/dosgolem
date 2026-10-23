# 459：保護模式剖面的 int 33h 使用者回呼經 IRQ12 遞送

狀態：**READY**（第 4 版，2026-10-23 轉 READY；三輪獨立審查與前置、補充探針的處置見附錄 A）
日期：2026-10-23
前置：[`006-layering`](006-layering.md)、[`017-mouse-callback`](017-mouse-callback.md)（本規格修訂其停止線）、[`432-linear-memory-and-paging`](432-linear-memory-and-paging.md) §7、
[`434-legacy-core-segmented-mode`](434-legacy-core-segmented-mode.md) §5.3、[`435-v86-hooks-and-idt-delivery`](435-v86-hooks-and-idt-delivery.md)（本規格擴充 §2.1 的外部中斷列）、
[`444-legacy-sb16-dma8-irq7`](444-legacy-sb16-dma8-irq7.md) §2.6、§2.7（本規格修訂 PIC 埠表）、[`448-session-config-legacy-pm-profile`](448-session-config-legacy-pm-profile.md)、
[`451-session-pointer-move`](451-session-pointer-move.md)、[`454-session-view-block-read-and-digest`](454-session-view-block-read-and-digest.md) §2.1（本規格新增一個送達來源）
使用端：strike_commander RE 030（`docs/re/030-cursor-trail-mouse-callback-delivery.md`）、規格 014 分期 (b) 的游標殘跡類

程式位置以 dosgolem-sc `strike-commander` 分支 `8687800` 為準（第 1 版寫 `9cd1458`，審查抽查行號一致）。前置探針：`workplace/mouse459/probe/report.md`（下稱探針報告）。DOSBox-X 引用一律指 `DOSBox-X-MCP-Debugger/dosbox-src/src/` 下的檔案。
推論等級：已證實、強推論、假說、未知。

用語：

| 用語 | 意義 |
|---|---|
| 保護模式剖面 | `Machine.ProtectedModeProfile()` 為真（`internal/machine/linear.go:106`），由 `SetProtectedModeProfile`、session `Config.ProtectedModeProfile`、probe `-pm-profile` 設定 |
| 實模式剖面 | 保護模式剖面以外的一切機器，含 LE／DOS4GW 路徑（`LEMachine` 另有自己的 PIC 模型，不在本規格範圍） |
| 直接遠呼叫 | 現行路徑：`fireMouseEventMickeys` 排 `cbQueue`，`tick` 在指令邊界以 `startCallback` 推遠呼叫到目前堆疊（`internal/machine/callback.go:65-85`） |
| 使用者回呼 | 遊戲以 `int 33h AX=000Ch`／`0014h` 登記的事件常式（`internal/dos/bios.go:363-388`） |
| IRQ12 樁 | 本規格在 StubSeg 新增、由 IVT[74h] 指向的 BIOS 客體碼，對應 DOSBox-X 的 `CB_IRQ12`、`CB_IRQ12_RET` 與 `uir_callback` |
| ISR0、ISR1、ISR2、ISR4、ISR7 | 主片 ISR bit 0（IRQ0）、bit 1（IRQ1）、bit 2（級聯）、從片 ISR bit 4（IRQ12）、主片 ISR bit 7（IRQ7，444 既有）。保護模式剖面建模這五個位元（§3.3a），其餘 ISR 位元不建模 |
| UIR | 使用者回呼執行中（DOSBox-X `mouse.in_UIR`，`ints/mouse.cpp:253`） |

---

## 1. 為什麼要做

| 事實 | 來源 | 等級 |
|---|---|---|
| 陸空戰將 OPTTEST 的游標程式每次移動由兩條路各處理一次：回呼路徑經描述元 `DB2E/DCBE` 寫線性 `A0000`，主迴圈路徑經 `E7E8` 寫線性 `100000h` | RE 030 §2 | 已證實（靜態） |
| JEMM 平時把線性 `A0000` 映到後台頁，只在中斷反射期間換成實體 VGA（dosgolem 量到 `0018:4782` 換入、`0018:47C8` 換回） | RE 030 §2；`workplace/cursortrail/runs/flip1/windows.tsv` 的 `flip` 列 | 已證實（dosgolem 動態） |
| DOSBox-X 上回呼路徑命中時在中斷堆疊上、線性 `A0000` 是畫面，且與上一次主迴圈路徑寫的 `FFFF:0010` 雜湊相同，6/6 | `workplace/cursortrail/dbx/bp1/driver.log` 第 7–47 列 | 已證實（DOSBox-X 動態） |
| DOSBox-X 的回呼由 IRQ12 經 PIC、`CPU_Interrupt`、IDT 進 JEMM，JEMM 反射 IVT[74h] 後才呼叫使用者回呼 | `ints/mouse.cpp:469-493`、`hardware/pic.cpp:500-534`、`cpu/cpu.cpp:1200`、`ints/mouse.cpp:2254-2296` | 已證實（原始碼） |
| dosgolem 以直接遠呼叫插入使用者回呼，不經 PIC、IDT、JEMM，沒有換頁，回呼寫到後台頁，螢幕留下殘跡 | `internal/dos/bios.go:752-764`、`internal/machine/machine.go:1743`、`callback.go:80`；探針 `runs/open1` 256 個擦除窗中 15 個 `mis_pix>0` | 已證實 |
| JEMM 對外部向量 `74h` 的碼路徑：IDT[74h]＝`0018:49AE`（`E8 2D 02 CF`）→ `4BDE`（無錯誤碼）→ `4D6E` 記向量 → `4F6E` 建 V86 框架、`CS:IP` 改 `0110:05F2` → `4752` 換入、`4782` 重載 CR3 → `iretd` → V86 `0110:05F2` 的 `retf` 進 IVT[74h] → 處理常式 `iret` 回 `0110:05FA` → `0110:0663` `int 15h AX=1209h BX=4549h` → `5998` → `4798` 換回、`47C8` 重載 CR3 → `popad; iretd` 回被打斷的程式 | 探針報告 §2（`img.dis` 靜態，並以 s449 內選擇子 18h 共 2326 bytes 與 `jemm.img` 逐位元組相同驗證） | 已證實（碼路徑） |
| 陸空戰將移動滑鼠時走上列的換頁分支 | s449 的 `[3ED3]=0`、`[3F38]=1`，加 DOSBox-X 6/6；`[3F38]=0` 走 `4F1F` 直接反射、`[3ED3]≠0` 走 `5120` 未追 | 強推論（單點） |

017 §停止線寫「不模擬 PS/2 IRQ12」。陸空戰將的正常玩家路徑需要這項能力，依 017 的條文另開本規格。

## 2. 範圍與剖面判定

### 2.1 判準

| 剖面 | int 33h 事件的遞送 |
|---|---|
| 保護模式剖面 | 本規格的 IRQ12 路徑（§3） |
| 實模式剖面 | 現行直接遠呼叫，程式路徑與位元組不變 |

判準只看 `pmProfile`，不偵測 JEMM 是否常駐、不看事件當下的 CPU 模式。理由：

1. 決定性：剖面在載入程式前就固定，同一份狀態檔、同一組輸入只會走一條路。依 CPU 模式動態切換，會讓同一個佇列裡的事件因遞送時點不同而走不同路，收據難以重現。
2. IRQ12 路徑在實模式也正確：`raiseIRQ`（`internal/machine/v86deliver.go:256-264`）在 V86 提交 IDT 遞送，否則走 `CPU.Interrupt` 經 IVT。SCCD 載入 `JEMM.OVL` 之前、JEMM 離開之後的實模式段落，事件照樣經 IVT[74h] 樁送到使用者回呼，與 DOSBox-X 一致（DOSBox-X 不分模式一律 IRQ12）。
3. 保護模式剖面的狀態本來就有自己的不透明欄位 `PM`（§5），IRQ12 的狀態放得進去而不動 gob 型別。

### 2.2 剖面切換的時點

- 剖面只能在載入程式前設定（448 §2.2 的順序：`SetExtendedMemoryKB`、`SetEMSPresent`、`SetProtectedModeProfile`、…、`SetSBDMA8`）。載入後不切換。
- 本規格在剖面開啟的機器上做三件初始化（§3.7）：IVT[74h] 指向 IRQ12 樁、寫入樁的位元組、從片 IMR bit 4 清 0。實作只放在 `LoadEXE`、`LoadCOM`（`internal/machine/loader.go:161、181`）設 `programLoaded` 的那一點，所以與 setter 的呼叫次序無關。`programLoaded` 由假轉真也發生在 `LoadState`（`state.go:410`），但 `LoadState` 不做這三件事：版本 4 的狀態檔記憶體裡已有樁與 IVT，版本 1–3 由 §5.3 的升級處理。
- `LoadState`：狀態檔的 `PM` 有無必須與剖面一致（`internal/machine/state.go:297-299` 既有檢查）。PM 版本 4 帶 IRQ12 狀態；版本 1–3 的舊檔依 §5.3 升級。

### 2.3 其他已收錄程式不受影響的論證

- 產品程式中開剖面的只有 `apps/strikecommander/receipt.go:157`（session `Config`）與 probe 的 `-pm-profile`。已證實（容器內 grep）。
- rich2、EOB1、EOB3、wolong、buckrogers、phantasie、dm、yuan、logh3 等都在實模式剖面。moo2、fd2 走 LE／DOS4GW（`le_mouse_callback.go`、`le_opl_ports.go` 的 `picMasks`），不經 `startCallback`，也不在保護模式剖面。
- 實模式剖面的改動只有新增的 `pmProfile` 判斷：`tick` 的回呼分支與 IRQ12 分支、`QueueCallback` 的上限、`In8`／`Out8` 的 `20h`、`A0h`、`A1h` 分支內（判斷放在埠分支內，不放在 `In8`／`Out8` 開頭；`20h` 一次執行讀約 190 萬次，444 檔內第 453 行的收據）、`handle` 的 `case 0xFB`／`0xFC`。都不在 `Step` 本體（§3.4）。`fireMouseEventMickeys` 不改。

測試要分兩份清單。字面 grep `ProtectedModeProfile` 會漏掉間接開剖面的夾具，第 1 版就漏了。

**盤點方法**（實作者在動工前做，結果寫進實作收據）：在本機工作樹於 `SetProtectedModeProfile` 的 `on=true` 分支暫時加一行，把 `runtime.Caller` 往上找到的 `Test…` 函式名寫到 `os.Getenv("P459_INV")` 指定的檔案；以 `go test ./... -count=1` 跑一次（需原版的測試照 SKIP），收集檔名去重，再把這行改動丟掉，不進 commit。所得清單與下表「剖面開」比對，多出的逐項補期望值。審查甲已知的間接夾具：`newPM`（`internal/machine/linear_test.go:20`）、`newSysProg`（`sysinsn_test.go:16`）、`deliverProg`（`v86deliver_test.go:70`）、`deliverProgSB8`（`sb_dma8_test.go:458`）。

**剖面關，逐位元組相同**（結果須與動工前 commit 逐項相同）：

| 類別 | 項目 |
|---|---|
| CPU | SingleStepTests 315 PASS、0 SKIP，`knownGaps` 不變 |
| 錨點 | 機器與 DOS 狀態檔的 gob 位元組錨點測試、`StateDigest` 語意錨點、`internal/machine/state_new_fields_zero_test.go`、`internal/dos/state_new_fields_zero_test.go` |
| 滑鼠與回呼 | `internal/dos/mouse_contract_test.go`（`TestMouseCallbackFiresAtAnInstructionBoundary`、`TestMouseCallbackRestoresCallerRegisters`、`TestMouseEventMaskFiltersWithoutQueueing`、`TestMouseResetReportsInstalledAndClearsHandler`、`TestInt33AX14ExchangesHandler` 等）、`internal/dos/state_mouse_formats_test.go`、`internal/machine/deliveries454_test.go` 中不開剖面的案例（含以 `new454` 建機者，審查甲第 3 輪核對其不開剖面）、`le_mouse_callback_test.go`、`le_mouse_callback_exchange_test.go`、444–446 中不經 `deliverProgSB8` 開剖面的 sb8 測試、`sb_dma8_test.go:717-724`（審查甲第 3 輪核對其不開剖面） |
| 上層 | `host/mouse_bridge_test.go`、`host/mouse_move_test.go`、`host/mouse_route_plan_test.go`、`session/pointer_move_test.go`、`session/owner_test.go`、`session/view454_test.go`、`session/view458_test.go`、`oracle/oracle_test.go`、`frontend/ebiten/game_test.go`、`cmd/buckrogers-play/input_test.go` |
| 程式層 | `go test ./apps/...` 對動工前 commit 逐行相同（含 `apps/rich2/frame_test.go`、`apps/wolong/wolong_test.go`、EOB1 的 017 驗收第 3、4 項）。需原版的測試照實記錄 SKIP 數 |
| 效能 | `BenchmarkStepRealMode` 與動工前 commit 交錯比較（431 §7、435 §7 第 7 項的做法），照實記錄 |
| 靜態 | 454 §3.2 對 `Step` 的機器碼比對：`Step` 本體不變 |

上表只收不經間接夾具開剖面的測試；盤點結果中開了剖面的，移到下表。

**剖面開，預期改變**（每項寫新期望值，舊期望值不得保留）：

| 測試 | 現況期望 | 新期望 |
|---|---|---|
| `TestV86CallbacksDeferred`（`v86gate_test.go:50-58`） | V86 下 `QueueCallback` 的遠呼叫照常執行、分段模式延後 | 分段模式延後不消耗（不變）；V86 下這個夾具沒有服務層，`Step` 回 §3.4 條件 1a 的「IRQ12 樁需要服務層」錯誤，不直接遠呼叫 |
| `TestV86CallbacksRun`（`v86deliver_test.go:491-520`） | V86 下回呼照常執行 | 同上，期望條件 1a 的錯誤。IDT `74h` 送達與回呼執行改在有服務層的夾具驗（§6.2 第 2 項） |
| `session` 的 `noKbdRewrite` 負對照（450 §3.4） | `BootFromState` 跳過第 6 步時鍵盤攔截碼沒被寫 | 剖面開時跳過第 6 步，記憶體裡是狀態檔帶來的客體碼；版本 3 升級來的狀態是 447 的 14 bytes 版本，IRQ1 交給它時不送 EOI，ISR1 卡住，§3.3a 的卡住上限在 IRQ1 再次待處理後觸發失敗即關閉。新期望是該錯誤（訊息含 ISR1） |
| `linear_test.go:674`、`692` | `encodePM` 長度 142、版本 3 | 版本 4，長度 `148+16n`（佇列空時 148），142 起的欄位照 §5.1 |
| `realcache442_test.go:909`、`seg443_test.go:911` | PM 版本 3 的位元組 | 同上，版本 4 |
| `deliveries454_test.go` 中以 `newPM`／`deliverProg` 建機的案例 | 現值 | 不經滑鼠者不變；有滑鼠事件者每事件 `Deliveries` 加 1（不變），送達點由 `startCallback` 改為 `raiseIRQ(74h)` |
| `session/legacy_pm_profile_test.go` 的 448 §3.1.1 字面錨點 | `9544c12` 的錨點 | 錨點若含 `PM` 或記憶體雜湊（StubSeg 新樁、IVT[74h]）就改變；新值由 (a)、(b) 各自的收據給出，附改變原因 |
| `session/boot_from_state_test.go`、`legacy_pm_profile_long_test.go`、`sc_perf_long_test.go`、`irq1_scan_long_test.go` | 版本 3 狀態檔 | 走 §5.3 升級；記憶體雜湊因樁與 IVT 改變；`LegacyMouseUpgrade` 為真 |
| 盤點新增者 | — | 逐項補 |

## 3. 遞送鏈

### 3.1 總覽

```text
dos.MoveMouse／PressMouse／ReleaseMouse
  → fireMouseEventMickeys：遮罩相符才排進 cbQueue（不變）
  → tick：本步沒送 IRQ0、IRQ0 守門未失敗時，IRQ12 條件成立就 ISR4=ISR2=1、raiseIRQ(74h)（§3.4 的固定結構）
      ├ V86：prepareV86Delivery／commitV86Delivery 經 IDT 向量 74h 進 JEMM ring 0（0018:49AE）
      │        → 4BDE → 4D6E → 4F6E 建 V86 框架 → 4752 換入、4782 重載 CR3 → iretd
      │        → V86 0110:05F2 retf 進 IVT[74h]
      └ 實模式：CPU.Interrupt(74h) 經 IVT（不經 IntHook，cpu.go:463-483）
  → IRQ12 樁（StubSeg:Mouse74Off）：sti、push ds、push es、pushad、int FBh
  → int FBh（服務層）：取出佇列頭；登記仍有效就推三組返回位址並設暫存器，否則跳到返回樁
  → 樁尾：EOI（out A0h、out 20h）、cld、retf → 使用者回呼
  → 使用者回呼 retf → UIR 樁：cli、int FCh（清 UIR）、retf
  → 返回樁 +7：popad、pop es、pop ds、iret
      ├ V86：回 JEMM 的 V86 跳板 0110:05FA → 0110:0663 int 15h AX=1209h BX=4549h
      │        → ring 0 5998 → 4798 換回、47C8 重載 CR3 → popad; iretd 回被打斷的程式
      └ 實模式：回被打斷的程式
```

換回發生在 JEMM 跳板的 `int 15h` 之後，不在返回樁的 `iret`（探針報告 §2、§3）。樁的形狀與 JEMM 相容：`retf` 進 IVT[74h]、`iret` 框架、進樁時 `IF` 已清。

### 3.2 事件排入

- `fireMouseEventMickeys`（`internal/dos/bios.go:752-764`）的遮罩判斷、`Mouse.Events` 紀錄、`QueuedCall` 的六個暫存器值都不變。
- 保護模式剖面下 `cbQueue` 專供本路徑使用。現行唯一的 `QueueCallback` 呼叫端就是這一支（已證實，容器內 grep）。
- 上限：保護模式剖面下佇列筆數上限 4096，與 §5.1 的編碼上限相同。`QueueCallback` 在已滿時不排入，設黏著的溢位旗標並記下被拒的筆數；`tick`（佇列非空必進）每次進入都把溢位錯誤放進 `irqErr`，由 `Step` 既有的 `takeIRQErr` 出口回傳（`machine.go:1609-1611`），失敗即關閉。溢位旗標進 §5.1 的旗標 bit 6。實模式剖面不設上限（現況）。
- 佇列只由 `int FBh` 消耗（§3.6），另外只有 `int 33h AX=0000h` 清空（§3.7）。IVT[74h] 被遊戲改掛時照 DOSBox-X 仍遞送 IRQ12 到 IVT 所指；遊戲的處理常式要鏈回 `Mouse74Off` 才會消耗事件。沒鏈回時佇列只增不減，到上限即失敗即關閉，錯誤訊息記 IVT[74h] 的值。
- `tick` 的直接遠呼叫分支（`machine.go:1740-1747`）在保護模式剖面下不執行 `startCallback`；週期遠呼叫分支（`1751-1757`）不變。
- `Step` 的進入條件（`machine.go:1604-1607`）已含 `len(m.cbQueue) > 0`，事件待送期間每步都會進 `tick`，所以 `Step` 不必新增判斷。代價是 ISR4 未清的那幾步也進 `tick`，可接受。

### 3.3 PIC 模型

保護模式剖面開啟 444 §2.6 的 PIC 埠模型（`20h`、`A0h`、`A1h`），與 `-sb-dma8` 是否開啟無關。DMA 埠、`22Ch`、`22Eh` 仍只在 `-sb-dma8` 開啟時接管。
PIC 的狀態從 `sbDMA8` 移到 `Machine`，兩個選項共用一份：

| 狀態 | 初值 | 現況位置 |
|---|---|---|
| 主片 IMR | `picMask`，0 | `machine.go:571`（不變） |
| 從片 IMR | 剖面開啟：`EFh`；只開 `-sb-dma8`：`FFh` | `sbDMA8.slaveIMR`（`sb_dma8.go:42、130`） |
| OCW3 讀取選擇 | 主、從都選 IRR | `sbDMA8.readISR`（`sb_dma8.go:45`） |
| IRR7、ISR7 | 444 不變 | `sbDMA8`（不搬） |
| ISR0、ISR1（主片，只在保護模式剖面） | 0 | 新增（§3.3a） |
| ISR4（從片）、ISR2（主片級聯） | 0 | 新增 |
| UIR | 0 | 新增 |

上表全部進狀態檔（§5.1），與分期無關（§10）。

IRQ12 待處理（「從片 IRR4」）不另存，以 `len(cbQueue) > 0 && !UIR` 推得。理由：佇列非空等於 DOSBox-X 的 `mouse.events > 0`，DOSBox-X 在 UIR 期間也不再呼叫使用者回呼（`ints/mouse.cpp:2255`）。

埠的行為（修訂 444 §2.6 表；只列與 444 不同或新增的列）：

| 埠 | 行為 | DOSBox-X |
|---|---|---|
| `A1h` 寫 | 存從片 IMR。剖面開啟時另影響 IRQ12 遞送（bit 4） | `pic.cpp` `write_data`→`set_imr` |
| `A1h` 讀 | 回從片 IMR | `read_data` |
| `A0h` 寫，OCW2 | `20h`（非特定 EOI）：ISR4 為 1 時清除；`64h`（特定 EOI，IRQ4）：清 ISR4；`60h`–`63h`、`65h`–`67h`、`00h`、`40h` 不動作。旋轉與設定優先序失敗即關閉（444 不變） | `pic.cpp:267-281` |
| `A0h` 寫，OCW3 | 存讀取選擇（444 不變）。poll、特殊遮罩失敗即關閉（444 只對 `20h` 擋，本規格對 `A0h` 一併擋） | `pic.cpp:254-265` |
| `A0h` 讀 | 依 OCW3：IRR 選擇回 bit 4＝IRQ12 待處理；ISR 選擇回 bit 4＝ISR4。其餘位元 0 | `read_command` |
| `20h` 寫，OCW2 | 保護模式剖面：非特定 EOI 清「優先權最高的在服務位元」，即 ISR0、ISR1、ISR2、ISR7 中編號最小且為 1 的那一位；都為 0 時不動作。特定 EOI：`60h`、`61h`、`62h`、`67h` 分別清 ISR0、ISR1、ISR2、ISR7，其餘不動作。只開 `-sb-dma8`：照 444 不變 | `pic.cpp:274-281`（非特定 EOI 清 `active_irq`）、`65-80`（`update_active_irq` 取編號最小的在服務位元） |
| `20h` 讀 | 依 OCW3：IRR bit 2＝IRQ12 待處理且從片 bit 4 未遮、ISR4 為 0；ISR 選擇在保護模式剖面回 bit 0＝ISR0、bit 1＝ISR1、bit 2＝ISR2、bit 7＝ISR7。IRR bit 0、bit 7 照 444，IRR bit 1 維持 0 | `read_command`（`pic.cpp:337-350`）；級聯見 `pic.cpp:141-185` |

`20h` 的讀取者（補充探針，探針報告 §3a）：從 s449 跑 259 M 步（主選單、存讀檔、滑鼠移動），`in 20h`、`21h`、`A0h`、`A1h` 都是 0 次，已證實（抽樣）。靜態唯一的讀取者是 OPTTEST `12A81h`、MENU 約 `14BC0h`、STRIKE `2AF0Dh`、TOPTTST1 `EFE3h` 的 PIT 精密計時常式，以 OCW3=`0Ah` 讀 IRR 且只測 bit 0，IRR bit 2 對它沒有影響；「沒有其他讀取者」為強推論（靜態搜尋未涵蓋計算出的 `DX` 與 `ins`／`outs`）。444 檔內第 453 行收據的 1,926,463 次讀取，在本次抽樣範圍內沒有出現。

V86 下 `out 20h`、`out A0h`、`in`／`out A1h` 經 TSS 的 I/O 點陣圖（`internal/cpu/v86.go:52-82` 的 `v86IOPermit`；`ops.go:509-558`）。s449 的 `TR=10h`（基底 `00111D98h`、界限 `2068h`），點陣圖位移 `0068h` 涵蓋全部埠，`20h`、`21h`、`A0h`、`A1h` 的位元皆 0，V86 的 `out A0h` 照點陣圖放行到埠模型，已證實（探針報告 §3a）。

IRQ7 的遞送條件（`sb8IRQ7Ready`，`sb_dma8.go:802-805`）與 `segWait` 中 IRQ7 的待處理判斷加 `!ISR2`；保護模式剖面另加 §3.3a 的在服務閘控。沒有這條，`Mouse74Off+0` 的 `sti` 到 `+0Ch` 的 `out 20h` 之間 IRQ7 可插入（DOSBox-X `PIC_runIRQs` 以 `isrr` 遮掉，`pic.cpp:549-556`）。

### 3.3a 主片在服務模型（保護模式剖面）

444 的最小模型沒有 ISR0、ISR1，IRQ0 處理常式在 IRQ12 樁內插入時，它的非特定 EOI 會清掉 ISR2。本規格在保護模式剖面把主片改成 8259 的在服務語意（主代理裁決 2026-10-23），並修訂 444 的這一點。只開 `-sb-dma8`（不開剖面）時維持 444 原狀，該設定目前沒有正式使用者。

| 項目 | 規則 | DOSBox-X |
|---|---|---|
| 設 ISR | IRQ0 送達（`tick` 的 `raiseIRQ(0x08)`）設 ISR0；IRQ1 送達（`keyTick`、`timedKeyTick`）設 ISR1；IRQ12 設 ISR2 與 ISR4；IRQ7 設 ISR7（444） | `start_irq`（`pic.cpp:211-226`） |
| 非特定 EOI | 清編號最小且為 1 的主片 ISR 位元（見上表） | `pic.cpp:274-281`、`65-80` |
| 閘控 | IRQ n 只在沒有編號 ≤ n 的主片 ISR 位元為 1 時遞送（級聯 IRQ12 以 n=2 計，另要 ISR4 為 0）：IRQ0 要 `!ISR0`；IRQ1 要 `!ISR0 && !ISR1`；IRQ12 要 `!ISR0 && !ISR1 && !ISR2 && !ISR4`；IRQ7 要 ISR0、ISR1、ISR2、ISR7 都為 0 | `PIC_runIRQs` 以 `active_irq` 為上限（`pic.cpp:541-586`） |
| BIOS 預設處理送 EOI | IVT[08h] 的 BIOS 計時器樁（`machine.go:2014-2036`）在 `int 1Ch` 之後、`pop dx` 之前加 `FA B0 20 E6 20`（`cli; mov al,20h; out 20h,al`），長度 35→40 bytes，佔 `0x420`–`0x447`，正好到 `SysConfigOff`（`0x448`）之前，沒有餘裕。鍵盤攔截客體碼（`kbdintercept.go:16-26`，447 §2.2）在 `pop ax` 之前加同樣 5 bytes，14→19 bytes，佔 `0x4B8`–`0x4CA`，`jnc +2` 的目標變成這段 EOI，兩條路都送。不開鍵盤攔截時，`internal/dos` 的 `biosInt9`（`dos.go` 的 `case 0x09`）結尾以等同 `out 20h,20h` 的埠寫入送 EOI（記入 `PortLog`）。只在保護模式剖面；實模式剖面的樁與 447 的位元組不變 | `CB_IRQ0`（`callback.cpp:393-412` 的 `cli; mov al,20h; out 20h,al`）、`CB_IRQ1`（`462-466`） |

理由：沒有 ISR0、ISR1，非特定 EOI 無從歸屬；只追蹤不閘控，ISR 位元與遞送脫節；BIOS 預設處理不送 EOI，ISR0／ISR1 會卡住，之後每一次非特定 EOI 都清錯位元。三件事要一起做（DOSBox-X 也是三件一起）。447 當初「不送 EOI」的理由是 dosgolem 沒有 ISR 模型（`kbdintercept.go:13`），本節之後在保護模式剖面不成立。

已知的雙重 EOI：遊戲的 IRQ0 處理常式若自己送 EOI 又鏈回 BIOS 樁，會送兩次；第二次清掉下一個在服務位元。DOSBox-X 行為相同（`CB_IRQ0` 也送），照真機語意處理，不另防。陸空戰將 OPTTEST 的 IRQ0 處理常式 `341D:050D` 送非特定 EOI 2685 次（探針報告 §3a），是否鏈回 BIOS 樁由 (a) 收據記錄。另一個 EOI 點 `1243:0039`（447 次，探針報告 §3a；主代理訊息寫作 `1243:0030`，以報告為準）服務哪一條 IRQ 待辨認，不擋 READY，(a) 收據一併記錄。

EOI 計數（診斷，不進狀態檔，probe 報告與收據列出）：主片、從片各自的非特定 EOI 次數，按「清掉的位元」分列（ISR0、ISR1、ISR2、ISR7、ISR4），另列「空 EOI」（送出時沒有任何建模的在服務位元）次數。雙重 EOI 表現為空 EOI 或清掉非預期位元，由這組計數看出。

**ISR 卡住偵測**。處理常式不送 EOI 時，閘控會讓較低優先序的 IRQ 永遠不送，而機器照常執行，不報錯。所以：

- 計數：對每個 IRQ（IRQ0、IRQ1、IRQ12、IRQ7）記「待處理但被 ISR 擋住」的連續步數，被擋的原因按擋住它的位元（ISR0、ISR1、ISR2、ISR4、ISR7）分列；每項另記全程最長值。條件不成立（沒有待處理、或已不被擋）時連續步數歸零。`IF=0`、IMR 遮住不算被 ISR 擋住。
- 上限：L＝10 × `IRQ0Every` 步；`IRQ0Every` 為 0（週期時鐘，`CycleClock`）時 L＝10 × `DefaultIRQ0Every`。上限只以步數計，不換算成秒（dosgolem `CLAUDE.md` 的兩個時間基準規則；這裡用不到秒），收據記 L 的值與來源。
- 超過 L：失敗即關閉，錯誤放進 `irqErr`，由 `tick`（IRQ0、IRQ12 待處理時必進）、`keyTick`／`timedKeyTick`（IRQ1）、`sb8IRQ7`（IRQ7）既有的錯誤出口回傳，`Step` 本體不改。訊息列 ISR0、ISR1、ISR2、ISR4、ISR7、各 IRQ 的待處理狀態、被擋的連續步數與最後一次設起該 ISR 的步數。
- 分段模式下的步數照樣計入（JEMM ring 0 期間 ISR 不會被清，但 ring 0 期間也不會持續超過一百萬步，434 §5.3 的 `segWait` 另管）。
- probe 報告與 §6.3 收據列出各項最長值。

優先序總表（保護模式剖面）：IRQ0 > IRQ1 > IRQ12（級聯 IRQ2）> IRQ7，同時套用上表的在服務閘控。

失敗即關閉的載體：`-sb-dma8` 開啟時沿用 `sb8Err`（444 §2.7）。只開剖面時，PIC 埠的失敗即關閉（ICW1、旋轉、設定優先序、poll、特殊遮罩）同樣寫入 `sb8Err` 欄位（改名不在本規格；欄位語意擴大為「PIC 與 SB 的黏著錯誤」）：`Step` 在 `CPU.Step` 之後的判斷 `err != nil || m.pfOK || m.sb8Err != nil`（`machine.go:1649`、`1669`、`1690`）不看 `sb8` 是否存在，所以發生的那一步經 `endStep` 回傳，`Step` 本體不改。`Step` 開頭不執行指令的黏著檢查只在 `sb8 != nil` 時有（`1586-1588`）。只開剖面時的實際行為是：錯誤一旦設起，之後每一次 `Step` 都回同一個錯誤，但那一步的指令照樣先執行（判斷在 `CPU.Step` 之後）。所以呼叫端必須在第一個錯誤就停；session 與 probe 既有的行為就是如此，收據註明這一點。

### 3.4 遞送點

位置：`tick`（`machine.go:1735-1816`）。現行 IRQ0 段有三個提早返回（`1799-1808`：`IF=0` 或主片 bit 0 遮、`!irq0Pending`、`ivtGuardIRQ(0x08)` 失敗），所以「放在 IRQ0 之後」不夠精確。結構寫死如下（`pm` 為 `pmProfile`）：

```text
tick():
  segmented := CPU.Segmented()
  if !segmented:
      if !pm && startCallback(): eventThisStep=true; deliveries++; return   // 實模式剖面不變
      if periodic 到期: …; return                                            // 不變
  幀計數、IRQ0 時鐘                                                          // 不變
  if segmented: segWait(); return                                            // 不變（segWait 納入 IRQ12）
  if pm && 溢位旗標: irqErr = 溢位錯誤; return                               // §3.2
  if pm: 更新 IRQ0、IRQ12 的「被 ISR 擋住」連續步數；超過 L 時 irqErr = 卡住錯誤; return   // §3.3a
  if IF && picMask&1==0 && irq0Pending && !(pm && ISR0):
      if ivtGuardIRQ(0x08): return            // IRQ0 守門失敗：本步不判 IRQ12，irqErr 由 Step 回傳
      irq0Pending=false; Ticks++; onTick; eventThisStep=true; if pm: ISR0=1; raiseIRQ(0x08); return
                                              // 現行次序不變，只在 raiseIRQ 前插入 ISR0=1；IRQ0 送達：本步不判 IRQ12
  if pm: mouseIRQ12()                         // IF=0、主片 bit 0 遮、無 IRQ0 待處理、ISR0 擋住 IRQ0 時都會到這裡
```

`mouseIRQ12` 自己檢查 `IF` 與 §3.3a 的閘控（ISR0 為 1 時它也不送）。週期遠呼叫送出的那一步不判 IRQ12（現行提早返回），卡住計數也不在那一步更新（下一步照常；連續步數以 `Steps` 差值計，不以呼叫次數計）。`keyTick`（`keyboard.go:105-129`）與 `timedKeyTick`（`keyhold.go:218` 起）在保護模式剖面的條件加 `!ISR0 && !ISR1`，送達時設 ISR1，並更新 IRQ1 的卡住計數；`sb8IRQ7` 更新 IRQ7 的卡住計數。這些都不在 `Step` 本體內。`segWait` 的「有待處理 IRQ」在保護模式剖面只計通過閘控的 IRQ（ISR 卡住的 IRQ 不算待處理，避免把「等 EOI」誤判成分段模式卡死）。

優先序 IRQ0 > IRQ1 > IRQ12（級聯 IRQ2）> IRQ7。IRQ1 在 `tick` 之後的 `keyTick` 處理，IRQ12 先於 IRQ1 送出的情形見 §8 第 6 項。

條件（全部成立才遞送）：

1. `len(cbQueue) > 0`、`!UIR`、`!ISR0`、`!ISR1`、`!ISR2`、`!ISR4`（§3.3a）。
1a. `CPU.IntHook != nil`。沒有服務層的機器（只有 `internal/machine` 的合成夾具）送 IRQ12 後 `int FBh` 無人服務，會在 per-vector stub 內遞迴；所以這裡失敗即關閉，錯誤放進 `irqErr`，訊息寫明「IRQ12 樁需要服務層」。
2. 從片 IMR bit 4 為 0、主片 `picMask` bit 2 為 0。
3. `IF=1`。
4. `ivtGuardIRQ(0x74)` 回假（`internal/machine/sysregs.go:108-129`）：V86 時準備 IDT 遞送計畫，實模式時做 433 的 IDTR 檢查。失敗時錯誤存 `irqErr`，由 `Step` 回傳（`machine.go:1609-1611` 既有路徑）。

遞送：`ISR4=1`、`ISR2=1`、`eventThisStep=true`、`raiseIRQ(0x74)`。`raiseIRQ` 開頭的 `deliveries++`（454 §2.1）計入這一次。事件本身不在這裡取出，留到 §3.6 的 `int FBh`。

分段模式（ring 0）：照 434 §5.3 延後，不消耗狀態。`segWait`（`machine.go:1824-1844`）的「有待處理 IRQ」加上 IRQ12 的條件 1、2，並把 IRQ7 那一項改為 `sb8IRQ7Ready()`（已含 `!ISR2`，§3.3）；錯誤訊息加上 ISR0、ISR1、ISR2、ISR4、ISR7、UIR 與佇列筆數。

`eventThisStep` 只在 `-sb-dma8` 開啟時於 `Step` 開頭清除（`machine.go:1590`），所以 IRQ12 的條件不看它；同一步內 IRQ0 與 IRQ12 互斥靠 `tick` 的控制流程。IRQ12 送達後設 `eventThisStep`，讓同一步的 IRQ7 分支（`sb_dma8.go:820`）讓開，這是 444 §2.7 的第七個設置點。

### 3.5 IDT 與 JEMM 反射

- V86：`prepareV86Delivery(V86Event{Kind: V86External, Vector: 0x74, FrameEIP: IP})` 與 `commitV86Delivery`（435 §2.3）照 IRQ0、IRQ1 的方式處理，不改 435 的程式，只在 435 §2.1 表的「外部中斷」列加一項 IRQ12（向量 `74h`）。
- JEMM：IDT 第 `74h` 項是 386 中斷閘，`0018:49AE`（`E8 2D 02 CF`），碼路徑見 §1 表與探針報告 §2，已證實。進入時 `4BE1` 的 `cmp sp,13D6h` 走無錯誤碼分支，§6.3 第 1 項以動態序列確認。換頁分支取決於 `[3ED3]`、`[3F38]`，強推論，§6.3 第 1 項逐次記這兩格。
- 實模式：`CPU.Interrupt(0x74)` 照 IVT。

DOSBox-X 對照：`slave_startIRQ`（`hardware/pic.cpp:500-534`）設從片與主片 ISR 後呼叫 `CPU_HW_Interrupt(slave.vector_base + 4)`（`include/cpu.h:186`），進 `CPU_Interrupt`（`cpu/cpu.cpp:1200`）的保護模式分支（`1300-1475`）。

### 3.6 IRQ12 樁與服務

StubSeg 佈局（`machine.go:104-187` 的 const 區塊；現況 0x4C6–0x4EF 未用，上界 0x4F0；審查甲以 `machine.go:126-130`、`xms_hook_test.go:205-207` 核對）新增三段，只在剖面開啟時寫入。第 3 版因鍵盤攔截客體碼在保護模式剖面加長到 `0x4CA`（§3.3a），三段整體後移 4 bytes。佈局註解的位移表要同步加這三段；既有的重疊檢查測試（`xms_hook_test.go` 一類）擴充為涵蓋全部特殊樁，任兩段重疊或超過 0x4F0 即失敗。

| 名稱 | 位移 | 位元組 | 對應 DOSBox-X |
|---|---|---|---|
| `Mouse74Off` | `0x4CC`（17 bytes，到 0x4DC） | `FB 1E 06 66 60 CD FB 50 B0 20 E6 A0 E6 20 58 FC CB` | `CB_IRQ12`（`cpu/callback.cpp:506-542`）：`sti; push ds; push es; pushad; <callback>; push ax; mov al,20h; out A0h,al; out 20h,al; pop ax; cld; retf` |
| `Mouse74RetOff` | `0x4DD`（12 bytes，到 0x4E8） | `FA B0 20 E6 A0 E6 20 66 61 07 1F CF` | `CB_IRQ12_RET`（`callback.cpp:543-577`）：`cli; mov al,20h; out A0h,al; out 20h,al; <callback>; popad; pop es; pop ds; iret` |
| `MouseUIROff` | `0x4E9`（4 bytes，到 0x4EC） | `FA CD FC CB` | `uir_callback`，`CB_RETF_CLI`（`callback.cpp:339-348`）：`cli; <callback>; retf` |

返回樁省略 DOSBox-X 的 `<callback>`（`INT74_Ret_Handler`，`ints/mouse.cpp:2298-2306`）：它只在佇列非空時重排 `MOUSE_Limit_Events` 計時，本規格的待處理由佇列推得（§3.3），不需要。返回樁有兩個入口：`+0` 從 `cli` 起，送 EOI 後收尾，給沒有使用者回呼的路徑；`+7` 是 `popad` 那一道，跳過 EOI，給使用者回呼返回的路徑（使用者路徑的 EOI 已在 `Mouse74Off` 的樁尾送過）。DOSBox-X 的 `+7`（`mouse.cpp:2277`）跳過的是同樣的 7 bytes（`cli` 加兩道 `out` 與 `mov`）。兩條路徑每個事件都恰好送一次 `A0h` EOI、一次 `20h` EOI（§6.2 第 15 項斷言）。

陷入號碼：`FBh`、`FCh`。`F2h`–`FAh` 已用（`callback.go:14-18`、`internal/dos/font.go:67-68`、`internal/dos/int9.go:81`），`FBh`、`FCh` 目前沒有使用者（審查乙核對）。兩者都不是 per-vector stub 位置，`writeBackFlags`（`internal/dos/dos.go:872-886`）不寫旗標。

接手條件（`handle` 開頭，在 435 §4 的 V86 條件與 IVT 段值判斷之前）：

- `n == 0xFB`：`pmProfile` 為真且本道指令 `c.Op()` 為 `StubSeg:Mouse74Off+5` 時服務；否則回 false，照一般軟體中斷經 IVT／IDT。
- `n == 0xFC`：`pmProfile` 為真且 `c.Op()` 為 `StubSeg:MouseUIROff+1` 時服務；否則回 false。
- 不看 IVT[FBh]、IVT[FCh] 的值。理由：第 1 版沿用 `handle` 的 IVT 段值判斷（`dos.go:770`），任何程式在任何位置執行 `CD FB` 都會被接手；改成位址判斷後只有樁本身的那一道算數。

`int FBh`（對應 `INT74_Handler`，`ints/mouse.cpp:2254-2296`），由 `internal/dos` 的 `handle` 分派：

1. `!UIR` 且佇列非空時取出佇列頭 `q`；否則（不應發生，見下）把 `CS:IP` 設為 `StubSeg:Mouse74RetOff`，結束。
2. 登記仍有效（`Mouse.Handler.Set` 且 `Handler.Mask & q.AX != 0`；DOSBox-X 在遞送當下看 `sub_mask`，`mouse.cpp:2267`）：
   - 以目前堆疊推入（與 `FarCallWithReturn` 相同的推入方式，V86 與實模式都以 `SS:SP` 段值×16 經線性層）：`StubSeg`、`Mouse74RetOff+7`、`StubSeg`、`MouseUIROff`、`Handler.Seg`、`Handler.Off`，共 6 個 word（`mouse.cpp:2276-2281`）。
   - `AX=q.AX`、`BX=q.BX`、`CX=q.CX`、`DX=q.DX`、`SI=q.SI`、`DI=q.DI`（`mouse.cpp:2268-2275`；值的差異見 §8 第 3、4 項）。
   - `UIR=true`、`CallbacksMade` 加 1。
   - `CS:IP` 不動，繼續執行樁尾：EOI、`cld`、`retf` 彈出 `Handler.Seg:Off` 進使用者回呼。
3. 登記已失效（遊戲在排入與遞送之間以 `AX=000Ch`／`0014h` 改了遮罩或停用；`AX=0000h` 已在 §3.7 清空佇列）：事件丟棄，丟棄計數加 1，`CS:IP` 設為 `StubSeg:Mouse74RetOff`（`mouse.cpp:2287-2293`），在返回樁送 EOI。

`int FCh`（對應 `UIR_Handler`，`mouse.cpp:2308-2311`）：`UIR=false`。UIR 本來為假時記一筆 `Unimplemented`（與 `IntCallbackReturn` 的處置相同，`dos.go:837-843`），不吞掉。

EOI 的次序照 DOSBox-X：有使用者回呼時 EOI 在進回呼之前（樁尾），回呼執行期間 `IF=1`（樁首 `sti`），IRQ0、IRQ1、IRQ7 可以插入；第二個 IRQ12 由 UIR 擋住（§3.4 條件 1）。沒有使用者回呼時 EOI 在返回樁。

EOI 以客體碼的 `out` 執行，不在 Go 內直接清 ISR，埠紀錄（`PortLog`）才與真機一致。V86 下的前提，均已證實（探針報告 §3a）：

- I/O 點陣圖放行 `20h`、`A0h`、`A1h`（§3.3，`TR=10h`）。
- V86 的 `IOPL=3`（s449 `iretd` 框架 `EFLAGS=00023246h`，單點），樁內的 `sti`、`cli`、`pushf` 類指令直接執行，不經 `#GP(0)`（435 §2.1）。

註記：JEMM 另有 TSS `F0h`（界限 `67h`，沒有 I/O 點陣圖）。若 `TR=F0h`，V86 的所有 I/O 都是 `#GP(0)`；這只發生在 JEMM `4742` 的 `ltr` 路徑。JEMM 的 `#GP` 處理只模擬 `CF`／`CD`／`CC`，`OUT` 會交給 `[3ED5]` 或暖開機（強推論，靜態），所以樁的 EOI 在那條路徑下不成立。(c) 探針在每次 IRQ12 遞送時記 `TR` 與框架的 `IOPL`，任一次不是 `TR=10h`、`IOPL=3` 即不通過（§6.3 第 1 項）。

### 3.7 初始化與 int 33h

剖面開啟的機器在載入程式時：

1. 寫入 §3.6 的三段樁，並把 BIOS 計時器樁換成 §3.3a 帶 EOI 的 40 bytes 版本。鍵盤攔截客體碼由 `EnableKbdIntercept` 寫入，它在保護模式剖面寫 19 bytes 版本並照 447 §2.1 讀回驗證（剖面在 `EnableKbdIntercept` 之前已設定，448 §2.2 的順序）。
2. IVT[74h] 設為 `StubSeg:Mouse74Off`（`initVectors` 原本寫的是 per-vector stub `StubSeg:StubOff(74h)`，`machine.go` 約 1983–1991 行）。
3. 從片 IMR bit 4 清 0。依據：DOSBox-X 開機時 `MOUSE_Startup` 呼叫 `Mouse_ResetHardware`（`ints/mouse.cpp:2538`），後者 `PIC_SetIRQMask(MOUSE_IRQ,false)`（`1394-1396`）。

`int 33h AX=0000h`（`internal/dos/bios.go:280-284`）在剖面開啟時另做（照 DOSBox-X）：

- 清從片 IMR bit 4（`Mouse_ResetHardware`，`mouse.cpp:1394-1396`、`1623-1626`）。
- 清空 `cbQueue`，被清掉的筆數計入 §3.6 的丟棄計數。DOSBox-X 的 `Mouse_Reset` 把 `sub_mask` 歸零（`mouse.cpp:1598`），之後 `INT74_Handler` 對剩下的事件都不呼叫使用者回呼，效果等於丟棄；本規格直接清空，少掉不呼叫回呼的空轉 IRQ12（§8 第 1 項同理）。
- `UIR=false`（`mouse.cpp:1599`）。若 `AX=0000h` 是在使用者回呼內呼叫的，之後的 `int FCh` 會記一筆 `Unimplemented`（§3.6），與 DOSBox-X 一樣不視為錯誤。
- ISR4、ISR2 不動（DOSBox-X 的重設不碰 PIC 的 ISR）。

遊戲或驅動自己寫 `A1h` 遮掉 IRQ12 時照遮，不覆寫。`AX=000Ch`、`0014h` 只改登記，不動佇列；已排入的事件在 `int FBh` 依當下登記決定呼叫或丟棄（§3.6 第 2、3 步）。

初值的理由：sb8 的從片 IMR 初值是 `FFh`（`sb_dma8.go:130`），444 §1.1 的 SB 驅動只對 `21h`、`A1h` 做讀改寫，不會清 bit 4。沒有第 3 步，IRQ12 永遠被遮，修正靜默失效而且不報錯。

DOSBox-X 開機的從片 IMR 是 `EEh`（`pic.cpp` 約 1001–1010 行：全遮後開 IRQ8；滑鼠再開 IRQ12）。本規格不建模 RTC，取 `EFh`。

### 3.8 同步派送與觀測

- `CallbackActive()` 在剖面開啟時回 `UIR`；`CallbacksMade()` 在 §3.6 第 2 步加 1。返回樁的 `popad`…`iret` 在之後幾步執行，`RunMouseEvent` 的呼叫端若要等到 `iret`，另以 ISR 與佇列判斷，本規格不加 API。
- `dos.RunMouseEvent`（`internal/dos/mouseevent.go:53-71`）在剖面開啟時的回傳：
  - 使用者回呼已返回（`CallbacksMade` 增加且 `!CallbackActive`）：`(true, nil)`，與現行相同。
  - 事件被丟棄（§3.6 第 3 步或 §3.7 的清空使丟棄計數增加）：`(false, nil)`，與「遊戲沒登記」同義。不再跑滿 `callbackBudget`。
  - 事件在 `callbackBudget` 內沒送達（例如 IRQ12 被遮、`IF` 一直為 0）：照現行回逾時錯誤，訊息加上 ISR4、ISR2、UIR、從片 IMR 與佇列筆數。
- 新增唯讀查詢 `MouseIRQ12() (pending bool, isr4, isr2, uir bool, queued int, discards uint64)`，給 probe 報告與測試用，不進狀態摘要。
- probe 報告在剖面開啟時列出：IRQ12 送達次數、使用者回呼次數、丟棄次數、`A0h`／`A1h` 讀寫次數、結束時的 ISR0、ISR1、ISR2、ISR4、ISR7、UIR 與佇列筆數、升級血統（§5.1 旗標 bit 5）、§3.3a 的 EOI 計數與各 IRQ 被各 ISR 擋住的最長連續步數、L。這些計數是診斷，不進狀態檔。
- 診斷開關 `SetMouseIRQ12Direct(bool)`：只在剖面開啟時有作用，為真時滑鼠事件照實模式剖面直接遠呼叫。用途是 §6.4 N1 的負對照，必須能由探針設定（`apps/strikecommander` 的探針經 session 的同名診斷方法，`cmd/probe` 經隱藏旗標），所以不能只是套件內欄位。只能在載入程式前或 `BootFromState` 前設定。不進 `session.Config`、不進狀態檔；開啟時 probe 報告與收據標記「負對照，非正式結果」。

### 3.9 DOSBox-X 逐點對照

| 環節 | DOSBox-X | dosgolem（本規格） | 一致性 |
|---|---|---|---|
| 事件入佇列 | `Mouse_AddEvent`（`mouse.cpp:469-493`） | `fireMouseEventMickeys`（`bios.go:752-764`）排 `cbQueue` | 不合併移動，§4 |
| 拉 IRQ | `PIC_ActivateIRQ(12)`（`mouse.cpp:488-491`、`pic.cpp:403-425`） | 佇列非空推得待處理（§3.3） | 等價 |
| 遮罩 | `MouseInterruptEnabled`（`mouse.cpp:434-447`）、從片 `imr`（`pic.cpp` `set_imr`） | 只有 `int 33h` 登記且遮罩相符才排入（現行）；從片 IMR bit 4、主片 bit 2 | §8 第 1 項 |
| 仲裁與 ISR | `PIC_runIRQs`（`pic.cpp:541-586`）→ `slave_startIRQ`（`500-534`）設從片、主片 ISR | §3.4 條件與 `ISR4`、`ISR2` | 只建模兩個位元 |
| 進 CPU | `CPU_HW_Interrupt`（`cpu.h:186`）→ `CPU_Interrupt`（`cpu.cpp:1200`）保護模式分支 | `raiseIRQ`→`commitV86Delivery`（435 §2.3）或 `CPU.Interrupt` | 已有 |
| 監控程式反射 | JEMM（DOSBox-X 執行原版 `JEMM.OVL`） | 同一份 `JEMM.OVL` | 同一客體碼 |
| IVT 74h 樁 | `CB_IRQ12`（`callback.cpp:506-542`）、`RealSetVec`（`mouse.cpp:2401-2404`） | `Mouse74Off` | 位元組對應，`FE 38 xx xx` 換成 `CD FB` |
| 樁內服務 | `INT74_Handler`（`mouse.cpp:2254-2296`） | `int FBh`（§3.6） | `DrawCursor` 不做（dosgolem 不畫游標，`bios.go:286-288`） |
| 使用者回呼返回 | `uir_callback`（`mouse.cpp:2414-2417`）、`UIR_Handler`（`2308-2311`） | `MouseUIROff`、`int FCh` | 一致 |
| 收尾 | `CB_IRQ12_RET`（`callback.cpp:543-577`）、`INT74_Ret_Handler`（`mouse.cpp:2298-2306`） | `Mouse74RetOff` | 省略重排計時 |
| EOI | 樁尾或返回樁的 `out A0h`、`out 20h` → `write_command`（`pic.cpp:241-292`） | 同位元組 → §3.3 埠模型 | 一致 |

## 4. 事件合併與限速

DOSBox-X 的做法：

- 佇列非空時新的移動事件直接丟掉（`mouse.cpp:471-473`），使用者回呼讀的 `CX`／`DX` 是遞送當下的位置（`POS_X`、`POS_Y`，`2272-2273`），所以丟掉的移動由下一次回呼的位置補上。
- 每 `MOUSE_DELAY`（預設 5 ms，`mouse.cpp:412-423`）最多拉一次 IRQ12（`MOUSE_Limit_Events`，`449-467`；`INT74_Ret_Handler` 重排，`2298-2306`）。

**建議：本規格不合併、不限速。** 每一筆排入的事件各遞送一次 IRQ12，依佇列次序；下一筆在前一筆的 UIR 結束、ISR4 與 ISR2 都被 EOI 清掉之後，於下一個符合條件的指令邊界遞送。

理由：

1. 現有契約：`QueueCallback` 的註解「排隊不丟」（`callback.go:38-42`），017 的「每次 callback 有界執行」，451 定案「同一回合多個移動在同一指令邊界送出」（451 §不在範圍）。合併會讓同一份輸入腳本產生不同的回呼次數。
2. 決定性與時間基準：`MOUSE_DELAY` 是 PIC 時間（毫秒）。dosgolem 的時鐘是指令數，換算要選 `StepsPerSecond()` 或 `StepsPerSecondNow()`（dosgolem `CLAUDE.md` 的時間基準硬規則），又多一個與 `IRQ0Pinned` 互動的參數。目前沒有證據顯示陸空戰將依賴這個節奏。
3. 殘跡的成因是遞送路徑（RE 030 §5），與節奏無關。先只改路徑，驗收時看得出單一變因的效果。
4. 輸入腳本的步數關係：排入的那一步不變；第一筆事件的使用者回呼開始時點晚於現行（多了 JEMM 進出與樁），後續事件依序排開。腳本以絕對步數排程的其他輸入不受影響，但它們落在畫面流程的哪一格會改變，這屬於 §7 的重錄。

保留的選項（不在本版範圍）：若之後證實某程式依賴合併或限速（例如快速拖曳時回呼次數影響遊戲邏輯），另開規格做「指令數換算的限速」，並以選項開啟。

## 5. 狀態檔

### 5.1 位置

陸空戰將 AGENTS §2（2026-10-15）定案：「保護模式相關狀態只加一個不透明欄位（自帶版本與編碼），gob 位元組錨點只再變這一次」，432 §7 與 `state.go:150-157` 的政策註解相同。所以本規格的狀態全部放進 `machineState.PM`，**不增減 gob 欄位**，錨點測試的 gob 位元組常數不變。

`PM` 升到版本 4，在版本 3（142 bytes，`internal/machine/pmstate.go:10-58`）尾端追加：

| 位移 | 大小 | 欄位 |
|---|---|---|
| 142 | 1 | 旗標：bit 0 ISR4、bit 1 ISR2、bit 2 UIR、bit 3 主片讀 ISR、bit 4 從片讀 ISR、bit 5 升級血統（§5.3）、bit 6 佇列溢位（§3.2）；bit 7 為 0 |
| 143 | 1 | 旗標 2：bit 0 ISR0、bit 1 ISR1（§3.3a）；bit 2–7 為 0 |
| 144 | 1 | 從片 IMR |
| 145 | 1 | 主片 IMR（`picMask`） |
| 146 | 2 | 佇列筆數 n（≤ 4096） |
| 148 | 16×n | 每筆 `QueuedCall`：`Seg`、`Off`、`AX`、`BX`、`CX`、`DX`、`SI`、`DI`，各 2 bytes 小端序 |

- 編碼：剖面開啟時一律寫版本 4。
- 升級血統 bit 5 一旦為 1，之後每次 `encodePM` 都沿用，不會因再存一次而消失；`LegacyMouseUpgrade()` 讀這一位。
- 驗證（`decodePM`，全部在任何寫入之前）：長度至少 `148+16n`；旗標 bit 7、旗標 2 的 bit 2–7 為 0；`n ≤ 4096`（與 §3.2 的排入上限相同）。ISR0、ISR1、ISR2、ISR4、UIR 不加組合限制（樁尾 `out A0h` 與 `out 20h` 之間是 ISR4=0、ISR2=1；`int FBh` 之後、樁尾 EOI 之前是 UIR=1、ISR4=1；客體程式送錯 EOI 或少送時可留下任何組合）。
- `cbQueue` 首次進狀態檔。451 §不在範圍記的「存檔後跳 `0000:0000`」假說，在保護模式剖面因此消除；實模式剖面維持現況（`session/digest.go:88-90` 的已知缺漏）。
- `cbActive`、`cbSaved` 在保護模式剖面恆為初值（不走 `startCallback`），不需要存。
- 主片 IMR 進 PM 的理由（審查甲第 3 輪）：只開剖面、未開 `-sb-dma8` 時，`picMask` 目前不在任何狀態欄位內（`sbDMA8State.PICMask` 只在 `-sb-dma8` 開時存），而 IRQ12 的遞送看它的 bit 2。

### 5.2 與 sbDMA8State 的重複欄位

`sbDMA8State` 已有 `PICMask`、`SlaveIMR`、`ReadISR`（`sb_dma8.go:999-1000、1045、1075`），格式由 444 §2.9 定，不改。兩個選項都開啟時同一個暫存器出現在兩處：

- `SaveState` 兩處寫同一個值（§3.3 的單一儲存位置）。
- `LoadState` 讀版本 4 時驗兩處的主片 IMR（`PICMask`）、從片 IMR、讀取選擇相同，不同就回錯誤、機器不變。版本 1–3 沒有 PM 端的值，不做這項比對，改走 §5.3 升級。

### 5.3 舊狀態檔

| 狀態檔 | 機器 | 行為 |
|---|---|---|
| 無 `PM` | 實模式剖面 | 不變 |
| `PM` 版本 4 | 保護模式剖面 | 照 §5.1 還原 |
| `PM` 版本 1–3 | 保護模式剖面 | **升級**（主代理裁決），見下 |
| 任何 | 剖面不一致 | 既有錯誤（`state.go:297-299`） |

升級規則（全部在驗證通過之後、其他寫入之前決定，寫入與 `applyPM` 同時）：

1. ISR0、ISR1、ISR2、ISR4、UIR 為 0，佇列清空（舊檔本來就沒存佇列）。舊檔若是在 IRQ0／IRQ1 處理常式送 EOI 之前存的，升級後該 EOI 會落在空的 ISR 上（不動作或清到 ISR7）；s449 存在 JEMM ring 0 出口，不屬這種情形。
2. 從片 IMR：有 `sbDMA8State` 時取它的 `SlaveIMR`，否則 `FFh`；然後清 bit 4（§3.7 第 3 步）。結果寫進單一儲存位置，所以之後存檔時 `sbDMA8State.SlaveIMR` 與 PM 兩處同步是清過的值，下一次載入的一致性檢查（§5.2）不會互擋。s449 實測 `SlaveIMR=FFh`（探針報告 §1），這一步是必要的。
2a. 主片 IMR：有 `sbDMA8State` 時取它的 `PICMask`，否則 0（`machine.go:571` 的初值；只開剖面時舊檔沒有存它）。寫進單一儲存位置。s449 實測 `00h`（探針報告 §1）。
3. BIOS 計時器樁：35 bytes 與舊版逐位元組相同才換成 §3.3a 的 40 bytes 版本；不同就失敗即關閉（`LoadState` 回錯誤、機器不變），訊息列出實際位元組。另檢查返回位址：插入點之後的舊位元組 `0x43F`–`0x442`（`pop dx`、`pop ax`、`pop ds`、`iret`）換版後會移位，`int 1Ch` 推的返回位址就是 `0x43F`。所以目前 `CS:IP`，以及分段模式下 ring 0 堆疊頂 `iretd` 框架的 V86 `CS:IP`，任一落在 `StubSeg:043F`–`0442` 時也失敗即關閉、不換。主代理訊息寫 `0x440`–`0x442`，本規格依 `int 1Ch` 的返回位址擴到 `0x43F`。更深的巢狀返回位址（例如 `int 1Ch` 處理常式內再被中斷）無法逐一偵測，列為限制；s449 的 `iretd` 框架目標由 (a) 收據記錄。
3a. 鍵盤攔截客體碼：`LoadState` 不寫。實際寫入點是 session `BootFromState` 的第 6 步（450 §2.3），它呼叫 `EnableKbdIntercept`，在保護模式剖面寫 19 bytes 版本並讀回驗證。`LoadState` 只在 `KbdIntercept` 為真時比對狀態檔內的位元組是 447 的 14 bytes 版本或本規格的 19 bytes 版本，兩者皆非就失敗即關閉。`KbdIntercept` 為假時不比對（向量 9 的 stub 由 447 §2.1 的 `CheckKbdInterceptAbsent` 另管）。
3b. 寫入 §3.6 的三段樁；IVT[74h] 若仍是 `StubSeg:StubOff(74h)` 就改成 `StubSeg:Mouse74Off`（s449 實測為 `0080:01D0`＝`StubOff(74h)`，探針報告 §1），被程式改過就不動並在升級紀錄記下。
4. 旗標 bit 5 設 1，`Machine.LegacyMouseUpgrade()` 回真，probe 報告與收據要列出。

升級的限制：

- 版本 3 若是在直接遠呼叫的回呼中途存的，`cbActive`、`cbSaved` 不在檔內，升級偵測不到；載入後回呼跑到 `CD F9` 時 `FinishCallback` 回假，記一筆 `Unimplemented`（`dos.go:837-843`）。用升級狀態跑的收據要檢查這一筆計數為 0。s449 存檔時 `cbActive` 為假、在 JEMM ring 0 出口內（探針報告 §1），不受影響。
- 遊戲記憶體帶著舊路徑的結果（存底緩衝 `9FD0:0004`、畫面上已有的殘跡），§6.3 以起算點與基線處理。

升級的理由：陸空戰將所有狀態檔（§7.1）都是版本 3，全部要求從冷開機重錄，會讓驗收（§6.3）與重錄本身都沒有起點。升級的限制要明講：升級後的狀態，遊戲記憶體與畫面帶著舊路徑留下的結果（例如已存在的殘跡），從它跑出的收據不等於從冷開機以 IRQ12 跑出的收據。

主代理裁決採升級加標記（§12 第 2 項）。

### 5.4 Snapshot

`Snapshot`／`Restore`（`internal/machine/snapshot.go`）保存同一組欄位（`state.go:30-32` 的警告）。`cbQueue` 已在 `Snapshot` 內（`snapshot.go:107、180、277`），新增 ISR0、ISR1、ISR2、ISR4、UIR、主片 IMR（已有，`snapshot.go:117`）、從片 IMR、讀取選擇、升級血統與溢位旗標，以及 §3.3a 卡住偵測的各項「起始步數」（連續步數的起點；不存的話 `Restore` 後會從錯的起點續算）。丟棄計數、EOI 計數、卡住最長值與 §3.8 的診斷計數不進。第 3 版漏列 ISR0、ISR1（審查乙第 3 輪阻擋），本版補上並以 §6.2 第 31 項驗證。

狀態檔同理：卡住偵測的起始步數不進 PM；`LoadState` 後全部歸零重新起算（載入點視為新的觀察起點），收據註明。

## 6. 不變式與驗收

### 6.1 實模式剖面

1. §2.3 回歸清單全部與動工前 commit 相同。
2. `machineState` 的 gob 位元組錨點、`StateDigest` 語意錨點不變；`PM` 為 nil。
3. 剖面關閉時 `In8`／`Out8` 對 `20h`、`A0h`、`A1h` 的行為與現況相同：`-sb-dma8` 關時 `A1h` 讀 `FFh`、`20h` 讀 `FFh`、寫入只進 `PortLog`；`-sb-dma8` 開時照 444。
4. `Step` 機器碼不變（454 §3.2 的靜態比對）。

### 6.2 保護模式剖面，合成測試

以 `internal/machine`、`internal/dos` 的合成程式（實模式與 435 的 V86 建法）驗：

1. 實模式：登記回呼後 `MoveMouse`。期望：一步內 `raiseIRQ(74h)`、`Deliveries` 加 1；經 IVT[74h] 進 `Mouse74Off`；回呼入口 `AX`…`DI` 等於 `QueuedCall`；回呼內 `IF=1`；回呼期間 ISR4、ISR2 為 0（已 EOI）、UIR 為 1；回呼 `retf` 後經 UIR 樁、返回樁回到被打斷處，通用暫存器（含高半）、段暫存器、`FLAGS` 與被打斷前相同；`CallbacksMade` 加 1、`Deliveries` 只加 1。
2. V86：435 的合成 GDT／IDT／TSS，IDT 第 `74h` 項指向模擬 JEMM 反射的 ring 0 常式（改 V86 堆疊、讀 IVT、`iretd`）。期望：送向量 `74h`、無錯誤碼、框架 `EIP` 為遞送當下的 `IP`；反射後在 V86 執行 `Mouse74Off`；`int FBh` 由服務層處理（`CS`=StubSeg）；回呼結束後返回樁的 `iret` 回到模擬 JEMM 推的框架。
3. 連續三筆事件：三次 IRQ12、三次回呼，次序與排入相同；第二筆在第一筆 UIR 結束、EOI 之前不遞送。
4. 遮罩：從片 IMR bit 4 為 1 時不遞送、事件留在佇列；清 0 後下一個邊界遞送。主片 bit 2 同。`IF=0` 時不遞送。
5. 分段模式：IRQ12 待處理時延後不消耗；`segWait` 一百萬步失敗即關閉的訊息含 IRQ12 欄位。
6. 登記失效：排入後以 `int 33h AX=000Ch, CX=0` 停用，遞送時走返回樁 `+0`、事件丟棄、丟棄計數加 1、不呼叫回呼、ISR4 與 ISR2 由返回樁清掉。另一例以 `AX=0000h` 重設：佇列當場清空、UIR 清 0、丟棄計數加上清掉的筆數、之後不再遞送 IRQ12。
7. `int 33h AX=0000h` 清從片 IMR bit 4；載入程式後從片 IMR 為 `EFh`（不論 `-sb-dma8` 開關，也不論 setter 次序），IVT[74h] 指向 `Mouse74Off`，三段樁的位元組正確。
8. PIC 埠：`A0h` 讀依 OCW3 回 IRR4／ISR4；`20h` 讀 IRR bit 2，ISR 選擇時 bit 0＝ISR0、bit 1＝ISR1、bit 2＝ISR2、bit 7＝ISR7（各位元單獨立起與同時立起各一例）；`A0h` 的 `20h`、`64h` 清 ISR4；`20h` 的非特定 EOI 清編號最小的在服務位元（細節見第 26 項）；`A0h` 的 poll、特殊遮罩失敗即關閉。
9. 同一步：IRQ0 與 IRQ12 同時待處理時先送 IRQ0；IRQ12 送達的那一步 IRQ7 不送（`-sb-dma8` 開）。
10. `RunMouseEvent` 在剖面開啟時：回呼返回回 `(true, nil)` 且 UIR 為 0；事件被丟棄回 `(false, nil)`，所用步數遠小於 `callbackBudget`；IRQ12 被遮回逾時錯誤，訊息含 §3.8 的欄位。
11. 狀態檔：版本 4 往返；佇列 0、1、多筆；ISR4／ISR2／UIR 各組合；版本 3 升級（§5.3 四條各一例，含 IVT[74h] 已被改的情形）；兩處從片 IMR 不一致回錯誤且機器不變；長度不足、保留位元非 0、`n` 超限各一例。
12. `Deliveries`：每筆事件恰加 1；`int FBh`、`int FCh`、使用者回呼入口不計（454 §2.1 的定義：只計步驟開始時的交出控制）。
13. SingleStepTests 315 PASS、0 SKIP，`knownGaps` 不變。
14. `BenchmarkStepV86Paged` 不低於 435 §7 第 7 項的門檻 11.58 M 道／秒；與動工前 commit 交錯比較，照實記錄（435 收據為 21.08 M 道／秒）。
15. EOI 次數：每筆事件在使用者路徑與丟棄路徑各恰好有一次 `A0h` 非特定 EOI、一次 `20h` 非特定 EOI（以 `PortLog` 計），區分 `+0` 與 `+7` 入口。
16. `tick` 結構（§3.4）：主片 bit 0 遮住 IRQ0、`IF=1`、IRQ12 待處理時，IRQ12 照送；`IF=1`、IRQ0 待處理但 `ivtGuardIRQ(0x08)` 失敗的那一步不判 IRQ12（`irqErr` 回傳，佇列與 ISR4 不變）。
17. IRQ7 與 ISR2：ISR2 為 1、IRR7 為 1、`IF=1` 時 IRQ7 不送；`segWait` 在此組合下不把 IRQ7 算作待處理。在 `Mouse74Off` 的 `sti` 之後、樁尾 `out 20h` 之前立起 IRR7，IRQ7 延到 ISR2 清除後才送。
18. 接手條件：在 `Mouse74Off+5` 以外的位置執行 `CD FB`（例如合成程式自己的碼）不被服務層接手，照 IVT[FBh] 走；剖面關閉時 `Mouse74Off+5` 的 `CD FB` 也不接手。`CD FC` 同理。
19. 沒有服務層：`IntHook` 為 nil 的合成機器，IRQ12 待處理時 `Step` 回「IRQ12 樁需要服務層」的錯誤，不遞送。
20. 佇列上限：剖面開啟時排入第 4097 筆，溢位旗標設起，下一個 `Step` 回溢位錯誤，之後每步都回；剖面關閉時排入 5000 筆不出錯。
21. IVT[74h] 被改掛：合成程式把 IVT[74h] 指向自己的常式並鏈回 `Mouse74Off`，事件照常消耗；不鏈回、只送 EOI 時佇列不減，排滿後回溢位錯誤且訊息含 IVT[74h] 的值。
22. `int 33h AX=0014h` 交換常式後，已排入的事件以新常式呼叫（遞送當下的登記）；新常式遮罩不含該事件時丟棄。
23. 只開剖面、不開 `-sb-dma8`：`20h` 寫 ICW1 時，同一步 `Step` 回錯誤。
24. 升級血統：版本 3 升級後存成版本 4，再載入時 bit 5 仍為 1、`LegacyMouseUpgrade()` 為真；冷開機的機器存檔 bit 5 為 0。
25. 升級時兩處從片 IMR：有 `sbDMA8State` 的版本 3 升級後存檔，`sbDMA8State.SlaveIMR` 與 PM 的從片 IMR 都是 `EFh`，再載入不觸發一致性錯誤。
26. 非特定 EOI 的歸屬：ISR0 與 ISR2 同時為 1 時（IRQ12 樁內 `sti` 之後插入 IRQ0），一次 `out 20h,20h` 只清 ISR0，ISR2 不變；再一次才清 ISR2。ISR1 與 ISR7 同時為 1 時只清 ISR1。全部為 0 時不動作。
27. 特定 EOI：`60h`、`61h`、`62h`、`67h` 各清對應位元，不動其他位元。
28. 在服務閘控：ISR0 為 1 時 IRQ0、IRQ1、IRQ12、IRQ7 都不送；ISR1 為 1 時 IRQ1、IRQ12、IRQ7 不送而 IRQ0 照送；EOI 之後下一個邊界依優先序遞送。`segWait` 在只有被閘控擋住的 IRQ 時計數歸零。
29. BIOS 預設處理送 EOI：保護模式剖面下，IRQ0 交給 BIOS 計時器樁時樁內送一次 `out 20h`，ISR0 清 0；IRQ1 經鍵盤攔截客體碼（`CF=1` 與 `CF=0` 兩條路）與 Go 端 `biosInt9` 各送一次。實模式剖面下兩段客體碼與 447 的位元組相同、不送 EOI。
30. 佈局：保護模式剖面下 BIOS 計時器樁佔 `0x420`–`0x447`、鍵盤攔截佔 `0x4B8`–`0x4CA`、滑鼠三段佔 `0x4CC`–`0x4EC`，與其他特殊樁不重疊、不超過 `0x4F0`。
31. Snapshot 往返：在 ISR0、ISR1、ISR2、ISR4、UIR 為 1、佇列 3 筆、主片與從片 IMR 與讀取選擇為非初值、卡住偵測有起始步數時 `Snapshot`；之後把上述每一項都改成不同的值（佇列清空、ISR 全清、IMR 改值）；`Restore` 後逐項等於 `Snapshot` 當時的值，接著跑同一段程式，`Deliveries`、EOI 計數與不 `Restore` 直接跑的結果相同。
32. ISR 卡住偵測：合成 IRQ0 處理常式不送 EOI（`iret` 前不 `out 20h`），IRQ0 再次待處理後，第 L+1 步回卡住錯誤，訊息含 ISR0=1 與連續步數；IRQ1 處理常式不送 EOI 時同樣以 ISR1 回錯誤；IRQ12 在 ISR2 卡住時（樁尾 EOI 被拿掉）以 ISR2 回錯誤。送 EOI 的正常處理常式跑 100 × L 步不出錯，最長值小於 L。`IF=0` 或 IMR 遮住的步數不計入。
33. 主片 IMR 進 PM：只開剖面時寫 `21h`=`FBh`（遮 IRQ2），存檔、載入後 IRQ12 仍被遮；兩個選項都開時，把 `sbDMA8State.PICMask` 與 PM 位移 145 改成不同值，`LoadState` 回錯誤且機器不變。升級：有 `sbDMA8State` 時取其 `PICMask`，否則為 0。
34. 升級的計時器樁：版本 3 狀態檔中計時器樁被改一個 byte，`LoadState` 回錯誤且機器不變；`CS:IP` 為 `StubSeg:043F`、`0441` 各一例回錯誤；分段模式下 ring 0 堆疊頂 `iretd` 框架的 V86 `CS:IP` 落在範圍內時回錯誤。
35. 升級的鍵盤客體碼：`KbdIntercept` 為真時，狀態檔內是 14 bytes 版本或 19 bytes 版本都接受，其他內容回錯誤；`KbdIntercept` 為假時不比對。
36. EOI 計數：第 26 項的序列中，按位元分列的計數與空 EOI 計數符合預期；IRQ0 處理常式自己送 EOI 又鏈回 BIOS 樁時，第二次 EOI 記為空 EOI 或清掉下一個在服務位元，計數照實反映。
### 6.3 保護模式剖面，陸空戰將（需原版，探針收據）

前置檢查（已完成，探針報告 §1）：`workplace/scratch/p485/s449.state`（SHA-256 `7dff6dd2…1e47a0`，第 4490000000 步）的 IVT[74h]＝`0080:01D0`＝`StubSeg:StubOff(74h)`（`CD 74 CF`），IVT[70h]–[77h] 都是 per-vector stub；`picMask=00h`；`SlaveIMR=FFh`；`readISR`、IRR7、ISR7、`cbQueue`、`cbActive` 皆空或假；存檔時在 JEMM ring 0 出口 `0018:5DE8`。升級規則第 2、3 步成立且必要。

驗收序列：RE 030 的探針 `ctprobe_test.go` 加上本規格的觀察點，以 `ct_open.script`（`runs/open1` 的輸入，停在第 4749100000 步）從升級後的 `p485/s449.state` 執行。

基線（載入後、第一筆輸入前，記入收據）：存底緩衝 `9FD0:0004` 起的內容與 SHA-256、實體 VGA 的 `IndexedInto` SHA-256、線性 `A0000` 與線性 `100000h` 各自對實體 VGA 的相同點數。這些是舊路徑留下的狀態，用來判讀之後的差異來源。

擦除窗的起算點：該畫面第一次全螢幕重繪完成之後。存讀檔對話框以 k0（對話框底圖畫完的那一格）為準，主選單以底板（`MAINMENU.PAK` 底圖）畫完為準；起算步數由探針以該次全畫面寫入結束的步數記錄。起算點之前的窗照記但不計入判準，因為它們可能擦回繼承的殘跡或讀到舊的存底緩衝，造成假失敗或假通過。

1. 第一次移動事件，逐環節列出步數，缺任何一環即不通過：IRQ12 遞送步 → ring 0 `0018:49AE` → `4BDE`（`4BE1` 的 `cmp sp,13D6h` 走無錯誤碼分支）→ `4D6E` → `4F6E` → `4752`、`0018:4782` 換入（線性 `A0000` 讀值變成實體 VGA）→ V86 `0110:05F2` `retf` → `StubSeg:Mouse74Off` → `int FBh` → 使用者回呼入口（`Handler.Seg:Off`）→ UIR 樁 → 返回樁 `+7` → `iret` 回 `0110:05FA` → `0110:0663` `int 15h AX=1209h BX=4549h` → `5998` → `4798`、`0018:47C8` 換回 → `iretd` 回被打斷的程式。每次 IRQ12 遞送時記 JEMM 的 `[3ED3]`、`[3F38]`，以及 `TR` 與 V86 框架 `EFLAGS` 的 `IOPL`；期望 `[3ED3]=0`、`[3F38]=1`（換頁分支）、`TR=10h`、`IOPL=3`（§3.6），後兩項任一次不符即不通過。IRQ12 處理期間（遞送到返回樁 `iret`）有無 IRQ0、IRQ7 插入，以及插入時 ISR0／ISR2／ISR7 的變化，照實記錄。`[3ED3]` 在 OPTTEST 期間是否恆 0 是新增的待量項。
2. 起算點之後、回呼期間的擦除窗與存底窗（進入時 UIR 為 1 者）：`row_mis=0`；整頁比對線性 `A0000` 與實體 VGA（`IndexedInto`）64000/64000 相同。這對應 DOSBox-X `driver.log` 的「`A000` 路徑命中時線性 `A0000` 為畫面」。
3. 起算點之後的擦除窗 `mis_pix>0` 的數量為 0（`runs/open1` 全段現況 15/256，`runs/flip1` 7/234；第 1 版沒有起算點，這兩個數字只當現況參考）。
4. 停止點截圖（存讀檔對話框開啟後移動）與 `workplace/cursortrail/dbx-m2-sheet.png` 的 DOSBox-X 結果目視一致：移開後原位置乾淨。目視只當輔助，判準是第 2、3 項。
5. `Deliveries`：每個移動事件的 IRQ12 計 1 次；與事件數一致。丟棄計數為 0；`Unimplemented` 中沒有 `F9`、`FC` 的紀錄（§5.3 的升級限制）。收據另列 §3.3a 的 EOI 計數（按位元與空 EOI）、各 IRQ 被各 ISR 擋住的最長連續步數與 L 的值；全程沒有觸發卡住錯誤。
6. 主選單（`runs/flip1`、`flip2` 的輸入）重複第 2、3 項。
7. 負對照 N1：同一序列以 `SetMouseIRQ12Direct(true)` 再跑一次，第 2 或第 3 項必須失敗（殘跡重現）。

存讀檔對話框在 DOSBox-X 上尚未重現（RE 030 §6），第 4 項的對照只有主選單有 DOSBox-X 證據；存讀檔以第 2、3 項為準。

### 6.4 負對照

每項都要讓至少一個測試失敗，記在實作收據：

| 編號 | 突變 | 期望失敗 |
|---|---|---|
| N1 | 剖面開啟仍走直接遠呼叫（`SetMouseIRQ12Direct(true)`，§3.8；探針可設） | §6.3 第 7 項（殘跡重現）、§6.2 第 1 項 |
| N2 | `Mouse74Off` 樁尾不送 `out A0h` | §6.2 第 3 項（第二筆事件不遞送）、第 15 項 |
| N3 | 返回樁 `+0` 不送 EOI | §6.2 第 6 項、第 15 項 |
| N4 | 遞送不看從片 IMR bit 4 | §6.2 第 4 項 |
| N5 | `LoadEXE`／`LoadCOM` 時不清從片 IMR bit 4 | §6.2 第 7 項（冷開機路徑；升級路徑由升級規則第 2 步另清，§6.3 不會因此失敗，所以不列） |
| N6 | `int FBh` 也加 `deliveries` | §6.2 第 12 項 |
| N7 | 遞送不看 UIR | §6.2 第 3 項 |
| N8 | `int FCh` 不清 UIR | §6.2 第 3、10 項 |
| N9 | `PM` 版本 4 不存佇列 | §6.2 第 11 項 |
| N10 | 版本 3 升級不清 bit 4 | §6.2 第 11 項 |
| N11 | 主片非特定 EOI 先清 ISR7 | §6.2 第 8 項 |
| N12 | 遞送不設 `eventThisStep` | §6.2 第 9 項 |
| N13 | `segWait` 不納入 IRQ12 | §6.2 第 5 項 |
| N14 | `tick` 的 IRQ12 判斷放在 IRQ0 段的提早返回之後（主片 bit 0 遮或 `IF=0` 時就不判） | §6.2 第 16 項 |
| N15 | IRQ0 守門失敗那一步仍判 IRQ12 | §6.2 第 16 項 |
| N16 | IRQ7 條件不看 ISR2 | §6.2 第 17 項 |
| N17 | 使用者路徑的返回位址用 `Mouse74RetOff+0`（多送一次 EOI） | §6.2 第 15 項 |
| N18 | `int FBh` 接手照 IVT 段值判斷（第 1 版做法） | §6.2 第 18 項 |
| N19 | `AX=0000h` 不清佇列與 UIR | §6.2 第 6 項 |
| N20 | 剖面開啟時佇列無上限 | §6.2 第 20 項 |
| N21 | 升級血統不沿用（再存一次就清掉 bit 5） | §6.2 第 24 項 |
| N22 | 存檔時 `sbDMA8State.SlaveIMR` 取載入當時的舊值，不取單一儲存位置 | §6.2 第 25 項 |
| N23 | 丟棄路徑不計丟棄 | §6.2 第 10 項（跑滿 `callbackBudget`） |
| N24 | 非特定 EOI 照第 2 版：先清 ISR2 | §6.2 第 26 項 |
| N25 | IRQ0 送達不設 ISR0 | §6.2 第 26、28 項 |
| N26 | 保護模式剖面的 BIOS 計時器樁不送 EOI | §6.2 第 29 項（ISR0 卡住，之後 IRQ0 不再遞送） |
| N27 | 閘控不看 ISR0 | §6.2 第 28 項 |
| N28 | `Snapshot` 不存 ISR0 | §6.2 第 31 項 |
| N29 | 拿掉 ISR 卡住偵測（處理常式不送 EOI 時不報錯） | §6.2 第 32 項（必須偵測到；拿掉後測試失敗） |
| N30 | 主片 IMR 不進 PM，或兩處 `PICMask` 不驗一致 | §6.2 第 33 項 |
| N31 | 升級時計時器樁位元組不符仍照換（或不換也不報錯） | §6.2 第 34 項 |
| N32 | 返回位址落在 `0x43F`–`0x442` 時仍換樁 | §6.2 第 34 項 |
| N33 | `KbdIntercept` 為假時也比對鍵盤客體碼 | §6.2 第 35 項 |

## 7. 影響

### 7.1 陸空戰將的收據與狀態檔

本規格只定 dosgolem 行為。重錄流程由陸空戰將專案另行處理。下列在陸空戰將 `SessionConfig`（`apps/strikecommander/receipt.go:146-166`，`ProtectedModeProfile: true`）下執行，含滑鼠事件者的回呼時點與之後畫格都會改變：

| 長測試 | 狀態檔（`workplace/scratch` 下） | 是否含滑鼠事件 |
|---|---|---|
| `trigger_long_test.go` A、B、C | `p485/s449`、`p530/s4495b`、`p539/s500b` | 含（腳本步數） |
| `saveload_long_test.go` F、G、I、J、K | `p485/s449`、`p539/s500b` | 含 |
| `saveload_confirm_long_test.go` | 同上（由該檔讀） | 含 |
| `mainmenu_long_test.go` L、M1、M2、N | `i018-st/m242`、`i018-cold/c800` | 含 |
| `labelframe_long_test.go` | `p486/s600` | 含 |
| `killboard_long_test.go` | `p485/s449` | 待查 |
| `composed_long_test.go` | `p539/s500b` | 待查 |
| `pointer_move_long_test.go` 甲、乙 | `p485/s449`、`p530/s4495b` | 含（本身就是移動） |
| `subtitle_long_test.go` | `i019-a/s500`、`s510`、`i022-t/s500b`、`i022-h/s390` | 待查 |
| `flight_long_test.go` | `p476/s800`、`p487/s760`、`i018-st/m242`、`i018-cold/c800` | 鍵盤為主，可能不受影響，逐一確認（RE 030 §5） |
| `exposure_long_test.go`、`perf_long_test.go` | 依環境變數 | 待查 |
| `session/sc_perf_long_test.go`、`legacy_pm_profile_long_test.go`、`irq1_scan_long_test.go`、`boot_from_state_test.go` | 依各檔 | 待查 |

狀態檔的產生方式：都是 dosgolem probe 或 session 以 `-pm-profile` 從冷開機或前一個狀態檔跑到指定步數存下，途中多半經過滑鼠操作。即使「存檔那一刻」佇列為空，存檔前的滑鼠操作已經以舊路徑改過遊戲記憶體（例如存底緩衝 `9FD0:0004` 與畫面）。§5.3 的升級讓它們能載入，但從它們跑出的結果只能當過渡比較；正式收據要以 IRQ12 從冷開機重錄的鏈產生。存檔腳本在 `docs/re/` 與 `workplace/` 各收據的產生紀錄裡，本規格不重述。

另需注意：

- 只用鍵盤、不碰滑鼠的段落，若遊戲在該段之前曾登記回呼且佇列空，行為不變；但若狀態檔是升級來的，IVT[74h] 與 StubSeg 的位元組改變，會改變記憶體雜湊，`Digest` 的 `MemorySHA256` 與 `MachineDigest` 不再等於舊收據。
- `PM` 版本 4 讓 `MachineDigest` 的機器部分改變（`PM` 有 `omitempty`，剖面開啟時進 JSON）。

### 7.1b 主片在服務模型對 444 收據的影響

§3.3a 只作用於保護模式剖面。陸空戰將的 SB 路徑（`-sb-dma8`）都在保護模式剖面下執行，所以 444 系列的收據會受影響；只開 `-sb-dma8` 的機器沒有正式使用者，維持原狀。

| 情形 | 舊模型 | 新模型 | 是否曾出現 |
|---|---|---|---|
| IRQ7 在 IRQ0 處理常式送 EOI 之前成為待處理 | 立即遞送 | 延到 ISR0 清除後 | 未知：舊模型沒有 ISR0，現有收據無從得知 |
| IRQ0 在 IRQ7 處理常式內插入（IRQ7 處理常式 `sti`） | IRQ0 的 EOI 清掉 ISR7 | 只清 ISR0 | 未知，同上 |
| IRQ0 交給 BIOS 計時器樁 | 樁不送 EOI | 樁送 EOI，步數多 3 道 | 在遊戲掛上自己的 IRQ0 之前（SCCD、MENU 初期）必定出現 |

第三列讓所有保護模式剖面的收據從開機起步數改變。前兩列是否出現，由 (a) 收據在陸空戰將的 SB 收據重跑時計數：IRQ7 被 ISR0 延後的次數與延後步數、IRQ7 處理期間 IRQ0 插入的次數。會改變的收據是以 SB DMA 紀錄比對的長測試：`subtitle_long_test.go` 的 C11、T10、H12、K1、K2（各自的 `sbdma.tsv`）、`session/sbdma_observer_test.go` 中以保護模式剖面執行者、`cmd/probe/sbdma8_test.go` 中開剖面者。這些本來就在 §7.1 的重錄範圍內。

### 7.1a 對陸空戰將規格 014 呈現層的影響

規格 014 的觀察器以 `Deliveries` 差值判斷「同一個 PC 是否會再被觀察到」，並以 `SS:SP` 窗配對函式的進入與返回（`apps/strikecommander/present.go:654`、`721`，審查乙指出）。本規格之後：

- 每個滑鼠事件仍只加 1 次 `Deliveries`，但送達點從回呼入口變成 IRQ12 的 IDT 遞送；回呼入口之前多了 JEMM ring 0、跳板 `0110:05F2` 與樁，這些位置的觀察點不再緊接在送達之後。
- 使用者回呼改在 JEMM 反射建立的 V86 框架與樁的 `pushad` 之後執行，`SS:SP` 不同於第 1 版的直接遠呼叫（DOSBox-X 實測回呼路徑 `SS:SP=514A:03BA`，`driver.log` 第 7 列）。014 依 `SS:SP` 配對的窗要以新收據確認。
- 回呼路徑寫的是實體 VGA，014 分期 (b) 基線中歸為「其他」類、由殘跡引起的露出應消失，基線要重錄。

這些由陸空戰將專案在 014 的下一版處理，本規格不改 014。

### 7.2 其他規格的修訂

| 規格 | 修訂 |
|---|---|
| 017 | 停止線改為「實模式剖面不模擬 PS/2 IRQ12；保護模式剖面見 459」。§typed 行為的「`SI` 為垂直、`DI` 為水平」與程式（`bios.go:746`）、DOSBox-X（`mouse.cpp:2274-2275`）相反，以程式為準修正文字（`SI` 水平、`DI` 垂直） |
| 435 | §2.1 外部中斷列加 IRQ12（向量 `74h`）；§4 表「`startCallback` 的 `FarCall`」列註明保護模式剖面的滑鼠不走這條 |
| 447 | §2.2 的客體碼在保護模式剖面加 EOI（14→19 bytes）；「不送 EOI」的理由限定於實模式剖面 |
| BIOS 計時器樁（`machine.go:2014-2036` 的註解；所屬規格由實作者查明後同步） | 保護模式剖面加 EOI（35→40 bytes） |
| 444 | §2.6 PIC 埠表依 §3.3 修訂；保護模式剖面下主片改為 §3.3a 的在服務模型（ISR0、ISR1、非特定 EOI 清優先權最高的位元、閘控）；§2.7 的 IRQ7 條件加 `!ISR2` 與在服務閘控、`eventThisStep` 設置點加 IRQ12；PIC 狀態移到 `Machine`；`sb8Err` 語意擴大為 PIC 與 SB 的黏著錯誤；`414` 行「只寫不讀」的審查結論與 `453` 行的 `20h` 讀取收據並列，讀取者身分待補 |
| 454 | §2.1 送達來源表的「硬體 IRQ」列呼叫端加 IRQ12（`tick`）；完整性驗收的設置點從六處變七處 |
| 451 | §不在範圍「待送的滑鼠回呼不在狀態檔裡」在保護模式剖面由本規格處理 |

## 8. 與 DOSBox-X 的刻意差異

| # | 差異 | 理由 |
|---|---|---|
| 1 | 遮罩不符的事件不排入、不拉 IRQ12（DOSBox-X 只要 `MouseInterruptEnabled` 就拉，`INT74_Handler` 再決定呼不呼叫） | 沿用現行排入規則（017），避免多出不呼叫回呼的 IRQ12 改變步數；這些 IRQ 對遊戲不可見，只多 JEMM 進出 |
| 2 | 不合併、不限速 | §4 |
| 3 | `CX`／`DX` 用排入時的位置（DOSBox-X 用遞送當下的位置） | 不合併時兩者只在「排入後、遞送前又移動」才不同；用排入時的值，每筆事件自洽 |
| 4 | `SI`／`DI` 用該次事件的位移（DOSBox-X 用累計的 `mickey_x`、`mickey_y`，`AX=0Bh` 才歸零） | 沿用現行；改成累計值是獨立的行為改變，需要另外的證據 |
| 5 | 不帶 `MOUSE_ABSOLUTE` 位元、`BH` 輪子值為 0 | dosgolem 沒有絕對定位與滾輪 |
| 6 | IRQ12 在 `tick` 內判斷，`keyTick` 在其後；兩者同一步都待處理時 IRQ12 先送 | IRQ12 送達後 V86 進分段模式、實模式 `IF=0`，同一步的 `keyTick` 自然延後；真機 IRQ1 優先。要嚴格照優先序需在 `Step` 加分支，違反 §6.1 第 4 項 |
| 7 | 從片 IMR 開機值 `EFh`（DOSBox-X `EEh`） | 不建模 RTC IRQ8 |
| 8 | 返回樁不重排計時 | 待處理由佇列推得 |
| 9 | 使用者回呼位址取遞送當下的登記，暫存器值取排入時 | 位址與遮罩跟 DOSBox-X；值見第 3、4 項 |
| 10 | UIR 期間不遞送 IRQ12（DOSBox-X 在 UIR 期間仍會因 `MOUSE_Limit_Events` 拉 IRQ12，`INT74_Handler` 看到 `in_UIR` 就直接跳返回樁，`mouse.cpp:2255`、`2292-2294`） | 那些 IRQ 不呼叫回呼、不消耗事件，只多 JEMM 進出；dosgolem 不限速，照做會在 UIR 期間每個可遞送的邊界都送一次，步數膨脹 |
| 11 | `int 33h AX=0000h` 直接清空佇列（DOSBox-X 留著事件、靠 `sub_mask=0` 讓之後的 IRQ12 空轉） | 對遊戲可見的結果相同（回呼都不呼叫），少掉空轉的 IRQ12 |

第 1–3 版的第 7 項「IRQ0 的 EOI 誤清 ISR2」已由 §3.3a 修正，第 4 版刪除並依序重編（舊第 8–12 項成為第 7–11 項）。只開 `-sb-dma8`、不開剖面時仍是 444 的舊行為，該設定沒有正式使用者。

## 9. 改動點

| 位置 | 改動 |
|---|---|
| `internal/machine/machine.go` 的 StubSeg const 區塊 | `Mouse74Off`、`Mouse74RetOff`、`MouseUIROff` 與佈局註解；特殊樁重疊檢查測試 |
| `internal/machine/machine.go` 的 `tick`、`segWait`、`In8`、`Out8` | §3.4 的固定結構；§3.3（剖面判斷放在 `20h`、`A0h`、`A1h` 分支內） |
| `internal/machine/loader.go` 的 `LoadEXE`、`LoadCOM` | §3.7 的初始化（`LoadState` 不做） |
| `internal/machine/callback.go` 的 `QueueCallback` | §3.2 的上限與溢位旗標 |
| `session`、`cmd/probe` | `SetMouseIRQ12Direct` 診斷方法與隱藏旗標（§3.8） |
| `internal/machine/sb_dma8.go` | PIC 狀態移到 `Machine`；`sb8In`／`sb8Out` 的 `20h`、`A0h`、`A1h` 改呼叫共用模型 |
| `internal/machine/callback.go` | `CallbackActive`、`CallbacksMade` 在剖面開啟時的語意（§3.8）；`MouseIRQ12` 查詢 |
| 新檔 `internal/machine/mouse_irq12.go` | 樁的安裝、IVT[74h]、遞送條件、`int FBh`／`int FCh` 用的機器端操作（取佇列、推入、UIR） |
| `internal/machine/pmstate.go`、`state.go`、`snapshot.go` | PM 版本 4、升級、兩處從片 IMR 一致性 |
| `internal/dos/bios.go` | `int 33h AX=0000h` 清 bit 4、佇列與 UIR |
| `internal/dos/dos.go` 的 `handle` | `0xFB`、`0xFC` 的位址接手條件（§3.6），放在 V86 條件與 IVT 判斷之前 |
| `internal/dos/mouseevent.go` | `RunMouseEvent` 的丟棄回傳（§3.8） |
| `internal/machine/machine.go` 的 `initVectors` 一帶、`kbdintercept.go`、`keyboard.go`、`keyhold.go`、`sb_dma8.go` 的 EOI 與 IRQ7 條件 | §3.3a：BIOS 樁帶 EOI 的版本、ISR0／ISR1 的設定與閘控、非特定 EOI 歸屬 |
| `internal/dos/dos.go` 的 `case 0x09` | 保護模式剖面下 `biosInt9` 結尾送 EOI |
| `internal/machine/snapshot.go` | §5.4 的新增欄位（含 ISR0、ISR1、卡住偵測起始步數） |
| `internal/machine` 的 IRQ 路徑 | §3.3a 的卡住偵測與 EOI 計數 |
| `session/boot_from_state.go`（450 §2.3 第 6 步） | 保護模式剖面下寫 19 bytes 鍵盤客體碼（經 `EnableKbdIntercept`） |
| `cmd/probe` | §3.8 的報告欄位與升級標記 |

## 10. 分期

| 分期 | 內容 | 驗收 |
|---|---|---|
| (a) | PIC 狀態搬到 `Machine`，剖面開啟時啟用 444 的埠模型與 §3.3a 的主片在服務模型（含 BIOS 樁送 EOI）；PM 版本 4 的編碼、解碼與升級（佇列欄位先固定為 0 筆，IRQ12 相關旗標恆 0） | §6.1 全部；只開 `-sb-dma8` 的 444、445、446 測試不變；§6.2 第 7（從片 IMR 部分）、8、11（佇列 0 筆）、23–36 項（第 31 項的佇列與 UIR 部分在 (b)）；§2.3「剖面開」表中 PM 版本相關的新期望值；§7.1b 的計數 |
| (b) | IRQ12 樁、遞送、`int FBh`／`FCh`、`int 33h`、佇列進 PM、`RunMouseEvent` | §6.2 其餘、§6.4 |
| (c) | 陸空戰將探針收據 | §6.3 |
| (d) | 017、435、444、454、451 的文字修訂 | 審查 |

PM 版本 4 併進 (a)：(a) 讓從片 IMR 與 OCW3 讀取選擇在只開剖面時也成為機器狀態，它們必須進狀態檔，否則 (a) 單獨提交時存讀檔會丟掉它們。所以 (a) 不能不帶版本 4 單獨提交。

(a) 之後剖面開啟的行為改變：讀 `20h` 的程式看到 IRR／ISR 而不是 `FFh`（只開剖面、未開 `-sb-dma8` 時）；`20h`、`A0h` 的 ICW1 等變成失敗即關閉；§3.3a 的在服務閘控與 BIOS 樁的 EOI 改變 IRQ 的時點與步數（§7.1b）。`20h` 的讀取者只有 PIT 精密計時常式，只測 IRR bit 0（§3.3），不受影響。(a) 的收據以 `session/legacy_pm_profile_long_test.go` 與一次陸空戰將冷開機到主選單的 probe 確認：沒有觸發新的失敗即關閉、沒有 ISR 卡住（各 IRQ 被擋的最長連續步數與 L 一併列出）、§7.1b 的兩項計數、EOI 計數、`341D:050D` 的 IRQ0 處理常式是否鏈回 BIOS 樁、`1243:0039` 服務哪一條 IRQ、每一次 `in 20h`／`in A0h` 的 `CS:IP`（冷開機到主選單全程，補 §3.3 抽樣範圍之外的段落）、s449 存檔時 ring 0 堆疊頂 `iretd` 框架的目標 `CS:IP`（§5.3 第 3 步）。

## 11. READY 條件

| # | 條件 | 狀態 |
|---|---|---|
| 1 | 兩位獨立審查（互不看對方報告）複審無阻擋項 | 完成：第 3 版複審甲無阻擋、乙一條阻擋（Snapshot 漏 ISR0／ISR1），第 4 版依主代理裁決處理（附錄 A.6） |
| 2 | §12 的四項裁決寫回本文 | 完成（第 2 版） |
| 3 | §6.3 前置檢查：s449 的 IVT[74h]、`picMask`、`SlaveIMR` 讀值 | 完成（探針報告 §1，寫回 §6.3、§5.3） |
| 4 | JEMM 對向量 `74h` 的碼路徑 | 完成，已證實（探針報告 §2）；換頁分支仍是強推論，由 §6.3 第 1 項在 (c) 收據閘門確認 |
| 5 | `in 20h` 的讀取者身分，以及它是否會因 IRR bit 2 改變行為 | 完成（探針報告 §3a）：259 M 步抽樣內 0 次讀取，已證實（抽樣）；靜態唯一讀取者是 PIT 精密計時常式，只測 bit 0，強推論。寫回 §3.3、§10、§13 |
| 6 | s449 的 TSS I/O 點陣圖中 `20h`、`A0h`、`A1h` 三個位元 | 完成（探針報告 §3a）：`TR=10h`，皆 0，V86 `out A0h` 經 `v86IOPermit` 到埠模型，已證實；V86 `IOPL=3`，已證實（單點）。寫回 §3.3、§3.6；`TR=F0h` 的註記與 (c) 的 `TR`、`IOPL` 記錄見 §3.6 |
| 7 | OPTTEST 期間 `[3ED3]` 是否恆 0 | 不擋 READY；列為 (c) 收據的量測項（§6.3 第 1 項） |
| 8 | EOI 點 `1243:0039` 服務哪一條 IRQ | 不擋 READY；(a) 收據辨認（§3.3a、§10） |

## 12. 裁決紀錄

主代理 2026-10-23 裁決，照第 1 版的建議：

| # | 問題 | 裁決 | 未採用的替代 |
|---|---|---|---|
| 1 | 剖面判準 | 只看 `pmProfile` | 另設 `-mouse-irq12` 選項 |
| 2 | 版本 1–3 的 `PM` | 升級加標記（§5.3），血統寫進旗標 bit 5 並沿用 | 拒絕舊檔 |
| 3 | 樁的形式 | 客體碼樁，暫存器存在客體堆疊（對應 DOSBox-X） | 沿用 `startCallback` 與 Go 端 `cbSaved` |
| 4 | `CX`／`DX`、`SI`／`DI` 的取值 | 維持現行（排入時的位置、該次位移） | DOSBox-X 的遞送當下位置與累計 mickey |

## 13. 推論等級摘要

| 項目 | 等級 |
|---|---|
| 殘跡成因是遞送路徑 | 已證實（RE 030） |
| DOSBox-X 的遞送鏈、樁位元組、EOI 次序 | 已證實（原始碼） |
| JEMM 對向量 `74h` 的碼路徑（反射 IVT[74h]、`int 15h` 後換回） | 已證實（探針報告 §2，靜態加位元組比對） |
| 陸空戰將移動滑鼠時走換頁分支 | 強推論（s449 單點 `[3ED3]=0`、`[3F38]=1`，加 DOSBox-X 6/6） |
| s449 的 IVT[74h] 是 per-vector stub、`SlaveIMR=FFh` | 已證實（探針報告 §1） |
| 遊戲在 s449 之後不改掛 int 74h | 未知（§6.3 收據記錄；改掛時見 §3.2） |
| 陸空戰將主選單、存讀檔、滑鼠移動期間沒有程式讀 `20h`、`21h`、`A0h`、`A1h` | 已證實（抽樣，s449 起 259 M 步） |
| 唯一的 `20h` 讀取者是 PIT 精密計時常式，只測 IRR bit 0 | 強推論（靜態；計算出的 `DX` 與 `ins`／`outs` 未涵蓋） |
| V86 下 `20h`、`21h`、`A0h`、`A1h` 經 I/O 點陣圖放行（`TR=10h`） | 已證實（s449） |
| V86 `IOPL=3` | 已證實（單點）；(c) 每次 IRQ12 記錄 |
| `TR=F0h` 時 V86 的 `OUT` 由 JEMM 交給 `[3ED5]` 或暖開機 | 強推論（靜態） |
| `1243:0039` 的 EOI 服務哪一條 IRQ | 未知（(a) 收據） |
| 保護模式剖面下 IRQ7 曾被 IRQ0 處理延後、或 IRQ0 曾插入 IRQ7 處理 | 未知（§7.1b，(a) 收據計數） |
| 不合併、不限速不影響陸空戰將的游標邏輯 | 假說（§6.3 驗收） |
| 只用鍵盤的飛行收據不受影響 | 假說（§7.1 逐一確認） |

## 附錄 A：版本差異與審查處置

### A.1 第 2 版差異摘要

- 基準改 `8687800`；§1 的 JEMM 碼路徑改為已證實，加入換頁分支的強推論列。
- §2.2：初始化只在 `LoadEXE`、`LoadCOM`。
- §2.3：測試拆成「剖面關，逐位元組相同」與「剖面開，預期改變」兩份，附盤點方法與新期望值表。
- §3.1：期望序列補 JEMM 跳板 `0110:05F2`、`05FA`、`0663` 的 `int 15h` 與 `47C8` 換回。
- §3.2：剖面開啟時佇列上限 4096、溢位黏著失敗即關閉；IVT[74h] 被改掛的處理。
- §3.3：IRQ7 與 `segWait` 加 `!ISR2`，優先序總表；失敗即關閉的載體；`20h` 讀取者與 I/O 點陣圖兩項 READY 條件。
- §3.4：`tick` 結構寫死；沒有服務層時失敗即關閉。
- §3.6：`int FBh`、`FCh` 改以指令位址與 `pmProfile` 接手，不看 IVT；`+0`／`+7` 措辭；佈局註解與重疊檢查。
- §3.7：`AX=0000h` 清佇列與 UIR。
- §3.8：`RunMouseEvent` 三種回傳；丟棄計數；可由探針設定的 N1 開關。
- §5：旗標 bit 5 升級血統、bit 6 溢位；佇列上限對稱；升級同步改 `sbDMA8State.SlaveIMR` 與 PM；版本 3 回呼中途存檔的升級限制。
- §6.2 新增第 15–25 項；§6.3 加基線、起算點、修正後的期望序列、`[3ED3]` 量測、N1 收據；§6.4 修 N1、N5，新增 N14–N23。
- §7.1a 對規格 014 呈現層的影響；§8 補第 11、12 項與第 7 項的連鎖風險。
- §10：PM 版本 4 併入 (a)；刪去引用 444 第 414 行「JEMM 與 SB 驅動不讀」的論證。
- §11 改為條件狀態表；§12 改為裁決紀錄；§13 依探針結果修正等級。

### A.2 審查甲

| 編號 | 類別 | 處置 |
|---|---|---|
| B1 回歸清單漏間接夾具、多項測試會改變 | 阻擋 | 接受。§2.3 拆兩份清單、附盤點方法，列出甲指出的四個夾具與七處測試的新期望值 |
| 應改 1 IRQ7 未被 ISR2 擋 | 應改 | 接受。§3.3 加 `!ISR2`，§6.2 第 17 項，N16 |
| 應改 2 `case 0xFB`、`0xFC` 以 `pmProfile` 為前提 | 應改 | 接受，併入位址接手條件（§3.6），§6.2 第 18 項，N18 |
| 應改 3 只開剖面時失敗即關閉無載體；(a) 狀態不進檔 | 應改 | 接受。§3.3 定載體為 `sb8Err` 欄位經 `endStep`；§10 把 PM 版本 4 併入 (a) |
| 應改 4 `programLoaded` 涵蓋 `LoadState` | 應改 | 接受。§2.2 只在 `LoadEXE`、`LoadCOM` |
| 應改 5 `tick` 提早返回 | 應改 | 接受。§3.4 寫死結構，§6.2 第 16 項，N14、N15 |
| 應改 6 N5 期望失敗寫錯 | 應改 | 接受，§6.4 N5 改為只看冷開機路徑 |
| 應改 7 N1 開關驅動不了探針 | 應改 | 接受。§3.8 `SetMouseIRQ12Direct` 可由探針設定，§6.3 第 7 項 |
| 應改 8 V86 下 `out A0h` 能否到埠模型 | 應改 | 接受，列 READY 條件 6，等補充探針 |
| 應改 9 佇列上限不對稱 | 應改 | 接受。§3.2 排入上限 4096 與 §5.1 相同，§6.2 第 20 項，N20 |
| 建議：AX=0 差異 | 建議 | 依主代理裁決照 DOSBox-X 清佇列與 UIR（§3.7），清空與空轉的差異列 §8 第 12 項（第 4 版重編為第 11 項） |
| 建議：`+7` 措辭 | 建議 | 接受（§3.6） |
| 建議：`+0`／`+7` 區分負對照 | 建議 | 接受。§6.2 第 15 項，N17 |
| 建議：`AX=0014h` 測試 | 建議 | 接受，§6.2 第 22 項 |
| 建議：佈局註解與重疊檢查 | 建議 | 接受（§3.6、§9） |
| 建議：基準 `8687800` | 建議 | 接受 |
| 建議：版本 3 回呼中途存檔的升級限制 | 建議 | 接受（§5.3），§6.3 第 5 項檢查 `F9` 紀錄 |
| 建議：沒有服務層的夾具 | 建議 | 接受，失敗即關閉（§3.4 條件 1a），§6.2 第 19 項 |
| 建議：確認 `CPU.Interrupt(0x74)` 不呼叫 `IntHook` | 建議 | 乙已核對（`cpu.go:463-483`），寫進 §3.1 |
| 建議：READY 條件 4 若用動態序列寫明為 (c) 閘門 | 建議 | 碼路徑已靜態證實；換頁分支寫明為 (c) 收據閘門（§11 第 4 項） |

### A.3 審查乙

| 編號 | 類別 | 處置 |
|---|---|---|
| 阻擋 1 升級狀態的繼承殘跡與存底緩衝；血統遺失 | 阻擋 | 接受。§6.3 加基線與起算點；血統寫進旗標 bit 5 並沿用（§5.1），§6.2 第 24 項，N21 |
| 應改 2 `20h` 有讀取者 | 應改 | 接受。§13 改「已證實有讀取者，身分未知」；READY 條件 5；刪改 §10 引 444 第 414 行的論證 |
| 應改 3 `tick` 控制流程 | 應改 | 接受，同甲應改 5 |
| 應改 4 IRQ7 與 `segWait` 加 `!ISR2` | 應改 | 接受，同甲應改 1；`segWait` 改用 `sb8IRQ7Ready()`（§3.4） |
| 應改 5 `int FBh`／`FCh` 接手條件太寬 | 應改 | 接受，位址判斷（§3.6） |
| 應改 6 `AX=0` 不清佇列與 UIR | 應改 | 接受（§3.7），§6.2 第 6 項，N19 |
| 應改 7 佇列無界、IVT 被改 | 應改 | 接受（§3.2），§6.2 第 20、21 項 |
| 應改 8 I/O 點陣圖 | 應改 | 接受，READY 條件 6 |
| 應改 9 升級與一致性檢查互擋 | 應改 | 接受。升級寫進單一儲存位置，存檔兩處同步（§5.3 第 2 步、§5.2），§6.2 第 25 項，N22 |
| 應改 10 `RunMouseEvent` 丟棄路徑逾時 | 應改 | 接受，三種回傳寫明（§3.8），§6.2 第 10 項，N23 |
| 建議：§2.3 與 §3.2 對 `fireMouseEventMickeys` 前後不一 | 建議 | 接受，§2.3 改為 `fireMouseEventMickeys` 不改 |
| 建議：§8 補 UIR 期間 IRQ12 | 建議 | 接受，§8 第 11 項（第 4 版重編為第 10 項） |
| 建議：§8 第 7 項連鎖風險 | 建議 | 第 2 版接受並登記；第 3 版依主代理裁決以 §3.3a 修正，§8 第 7 項刪除 |
| 建議：`In8`／`Out8` 判斷放埠分支內 | 建議 | 接受（§2.3、§9） |
| 建議：`4BE1` 分支以動態序列確認 | 建議 | 接受，§6.3 第 1 項 |
| 建議：§7 補規格 014 呈現層影響 | 建議 | 接受，§7.1a |

### A.4 前置探針（探針報告）

| 建議 | 處置 |
|---|---|
| §6.3 第 1 項期望序列補 `0110:05FA–0663` 的 `int 15h` 再 `47C8` 換回 | 接受 |
| §3.1 註明換回在 `int 15h` 之後 | 接受 |
| 碼路徑等級改已證實，換頁分支維持強推論 | 接受（§1、§13） |
| 樁設計與 JEMM 相容 | 寫進 §3.1 |
| §6.3、§11 第 3 項寫回讀值 | 接受 |
| 新增待量 `[3ED3]` | 接受（§6.3 第 1 項、§11 第 7 項） |

### A.5 第 3 版差異

依補充探針（探針報告 §3a）與主代理裁決：

| 項目 | 改動 |
|---|---|
| READY 條件 5、6 | 標完成。依據寫進 §3.3（`20h` 讀取者、I/O 點陣圖）、§3.6（V86 前提）、§10、§13，等級照報告：抽樣已證實、靜態強推論 |
| §3.6 | 補 `IOPL=3` 前提與 `TR=F0h` 時 V86 I/O 全為 `#GP(0)` 的註記；(c) 探針每次 IRQ12 記 `TR` 與框架 `IOPL`（§6.3 第 1 項） |
| §3.3a（新節） | 第 2 版 §8 第 7 項不接受為限制。保護模式剖面的主片改為 8259 在服務語意：ISR0、ISR1 建模，非特定 EOI 清編號最小的在服務位元，特定 EOI `60h`–`62h`、`67h`，依 ISR 閘控 IRQ0／IRQ1／IRQ12／IRQ7，BIOS 計時器樁與鍵盤攔截客體碼、`biosInt9` 送 EOI（照 DOSBox-X `CB_IRQ0`、`CB_IRQ1`）。理由：沒有 ISR0 就無從歸屬 EOI；BIOS 預設處理不送 EOI 會讓 ISR0／ISR1 卡住 |
| §3.4 | `tick` 偽碼加 ISR0 閘控與設定；`keyTick`、`timedKeyTick` 加閘控與 ISR1；`segWait` 只計通過閘控的 IRQ |
| §3.6 佈局 | 鍵盤攔截客體碼在保護模式剖面長到 `0x4CA`，滑鼠三段後移到 `0x4CC`、`0x4DD`、`0x4E9` |
| §5.1 | PM 版本 4 加旗標 2（ISR0、ISR1）與保留 byte，佇列起點改 148 |
| §5.3 | 升級時 ISR0／ISR1 清 0；BIOS 計時器樁與鍵盤攔截客體碼逐位元組相符才換新版 |
| §6.2 第 26–30 項、§6.4 N24–N27 | 非特定 EOI 歸屬（ISR0 與 ISR2 同時在服務只清 ISR0）、特定 EOI、閘控、BIOS 樁 EOI、佈局 |
| §7.1b（新節） | 對 444 收據的影響：保護模式剖面下的 SB 收據會變；兩種「同時在服務」情形在舊收據中是否出現未知，由 (a) 收據計數；列出受影響的長測試 |
| §7.2 | 444 修訂加在服務模型；新增 447 與 BIOS 計時器樁的修訂列 |
| §8 第 7 項 | 刪除（已修正） |
| §10 (a) | 併入在服務模型；收據加 §7.1b 計數、`341D:050D` 是否鏈回 BIOS 樁、`1243:0039` 的辨認 |
| §11 第 8 項、§13 | `1243:0039` 的 EOI 點待辨認，不擋 READY；等級表依補充探針更新 |

### A.6 第 4 版差異與第 3 版複審處置

第 3 版複審由主代理轉述（審查報告未存於 `workplace/mouse459/`）：甲無阻擋、應改 1–5；乙阻擋 1、應改 2、3；兩位的建議。處置照主代理裁決：

| 編號 | 類別 | 處置 |
|---|---|---|
| 乙阻擋 1 `Snapshot` 漏 ISR0、ISR1 | 阻擋 | 接受。§5.4 補 ISR0、ISR1 與卡住偵測起始步數；§6.2 第 31 項往返測試（`Restore` 前故意改髒 ISR0／1／2／4、UIR、佇列、IMR）；N28 |
| 甲 4、乙 2 ISR 卡住無偵測 | 應改 | 接受。§3.3a 加卡住偵測：各 IRQ 被各 ISR 擋住的連續步數與最長值，上限 L＝10 × `IRQ0Every` 步，超過失敗即關閉，訊息列 ISR0／1／2／4／7；probe 與 §6.3 收據列出；§6.2 第 32 項、N29 |
| 甲 4、乙 2 升級的計時器樁 | 應改 | 接受。位元組不符改為失敗即關閉；返回位址落在舊樁 `0x43F`–`0x442` 時不換並失敗即關閉（主代理訊息寫 `0x440`，依 `int 1Ch` 返回位址擴到 `0x43F`）；s449 的 `iretd` 框架目標在 (a) 記錄；§6.2 第 34 項、N31、N32 |
| 乙 3 升級的鍵盤客體碼 | 應改 | 接受。只在 `KbdIntercept` 為真時比對；實際寫入點是 `BootFromState` 第 6 步；`noKbdRewrite` 在剖面開的新期望列入 §2.3；§6.2 第 35 項、N33 |
| 甲 1 `sb_dma8_test.go:717-724`、`new454` | 應改 | 接受，改列「剖面關」 |
| 甲 2 `TestV86CallbacksDeferred`／`Run` | 應改 | 接受。新期望為條件 1a 的「IRQ12 樁需要服務層」錯誤；IDT `74h` 送達改在有服務層的夾具驗（§6.2 第 2 項） |
| 甲 3 主片 IMR 不在狀態檔 | 應改 | 接受。`picMask` 進 PM 位移 145；兩選項都開時驗兩處相同；升級取 `sbDMA8State.PICMask`，否則 0；§6.2 第 33 項、N30 |
| 甲 5 錯誤載體描述不實 | 應改 | 接受。§3.3 改寫為「之後每步都回錯誤、指令照樣執行，呼叫端第一個錯誤即停」 |
| 建議：`Ticks` 合併 | 建議 | §3.4 偽碼的 IRQ0 送達改為一行，照現行次序，只在 `raiseIRQ` 前插入 `ISR0=1` |
| 建議：§6.2 第 8 項措辭 | 建議 | 改為「清編號最小的在服務位元」，並加 `20h` 讀 ISR bit 0／1 的測試 |
| 建議：§8、§11 依序編號 | 建議 | §8 刪舊第 7 項後重編；§11 第 7、8 項依序 |
| 建議：舊措辭殘留 | 建議 | 用語表 ISR 列、§3.3a 開頭改寫為現況 |
| 建議：雙重 EOI 計數 | 建議 | §3.3a 加 EOI 計數（按位元與空 EOI），§6.2 第 36 項 |
| 建議：步數換算基準 | 建議 | 卡住上限只以步數計，`IRQ0Every` 為 0 時取 `DefaultIRQ0Every`，不換算成秒，收據記 L 與來源 |
| 建議：冷開機 probe 記 `in 20h` 的 `CS:IP` | 建議 | 接受，列入 §10 (a) 收據 |

### A.7 三輪審查紀錄

| 輪次 | 對象 | 審查者 | 結論 | 主要處置 |
|---|---|---|---|---|
| 第 1 輪 | 第 1 版 | 甲（`review459v1-A.md`）、乙（`review459v1-B.md`），互不看對方報告 | 皆不轉 READY：甲阻擋 1（回歸清單漏間接夾具）、乙阻擋 1（升級狀態的殘跡與血統） | 第 2 版（A.1–A.3）；同輪另有 READY 前置探針（A.4） |
| 第 2 輪 | 第 2 版 | 補充探針（I/O 點陣圖、`20h` 讀取者）與主代理裁決 | READY 條件 5、6 完成；§8 第 7 項不接受為限制 | 第 3 版（A.5）：主片在服務模型 |
| 第 3 輪 | 第 3 版 | 甲、乙複審（主代理轉述） | 甲無阻擋（應改 1–5）；乙阻擋 1（`Snapshot` 漏 ISR0／ISR1）、應改 2、3 | 第 4 版（A.6），轉 READY |

第 2 輪沒有獨立審查報告檔，是探針結果與主代理裁決驅動的修訂；表中如實列出。
