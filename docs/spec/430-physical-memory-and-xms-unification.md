# 430 — 延伸實體記憶體與 XMS 統一

狀態：**READY**（四輪審查；最後一輪只剩文字修正，已套用）
修訂：§2.3 的 StubSeg 佈局依 [`459`](459-mouse-irq12-v86-delivery.md) 補列（註「由 459 修訂」，2026-10-23）。
日期：2026-10-15
前置：[`011-xms`](011-xms.md)（已部分過期，現行行為以 `internal/dos/xms.go` 為準）、[`014-ems`](014-ems.md)、
[`189-a20-gate-and-hma-addressing`](189-a20-gate-and-hma-addressing.md)、
[`429-dos-sft-chain`](429-dos-sft-chain.md)（`programLoaded` 旗標由 429 定義，本規格在 429 實作之後實作）

---

## 1. 觸發案例與目標

《Strike Commander》光碟版的啟動器 `SCCD.EXE` 載入 Origin 的 V86 監控程式 JEMM。證據在
`strike_commander_cht_dosgolem` 的 `docs/re/002-sccd-launch-flow.md`、`004-cpu-requirements.md`，以及下列
SCCD 映像位移（檔案位移減 `0x200`）：

| SCCD／JEMM 的需求 | 證據 | dosgolem 現況 |
|---|---|---|
| 取得 XMS 入口後，入口第一個 byte 是 `EBh` 就以 `入口 + 2 + rel8` 為原處理常式；否則把 `[入口+1]` 的 dword 當遠指標。接著存下入口前 5 bytes，改寫成 `EA` 遠跳到自己的掛鉤 | 映像 `0x2D2`–`0x31B`：`26 80 3f eb` `74 0c` … `26 c6 07 ea`（已證實） | 入口 `CD F5 CB`，第一個 byte 不是 `EBh`，SCCD 會把 `F5 CB ?? ??` 當成遠指標 |
| XMS `AH=09h` 配置約 5,000 KB、`AH=0Ch` Lock 取實體位址，位址須 ≤ `0xFE0000` | 映像 `0x37E`：`66 81 FA 00 00 FE 00`（比較 Lock 回傳的基底；已證實） | 區塊在 Go map，Lock 回合成位址 `0x01000000` 起 |
| JEMM 以平坦選擇子把 `0xA000` bytes 映像複製到該實體位址，之後在那裡建頁表、EMS 頁 | JEMM img `0x4222`（`004-cpu-requirements.md` §1、§4 的 S0 列） | 1 MB 以上只有 64 KB HMA |
| SCCD 用 8042 開 A20 | 映像 `0x1B80`–`0x1B97` | 8042 寫入只記錄 |
| 有 EMM 時 SCCD 走 VCPI 路徑並失敗 | `002` §1 | `int 67h` 一律由內建 EMM 服務 |

本規格只處理記憶體與 DOS 服務層。CPU 的保護模式與分頁另立規格；本規格提供它們要用的實體位址存取入口。

## 2. 規則

### 2.1 延伸實體記憶體（選項，預設關閉）

`Machine.SetExtendedMemoryKB(kb int) error`。合法值：0（預設），或 4,096 到 64,512 之間、4 的倍數
（以 4 KB 頁為單位，與 §2.2 的配置粒度一致）。其他值、`programLoaded` 為真、或 `len(m.Mem) != MemSize`
（LE 路徑會增長 `Mem`）時回錯誤。

- 檢查順序固定：(1) `programLoaded` 為真回錯誤；(2) `len(m.Mem) != MemSize` 回錯誤；(3) 範圍與倍數不合回錯誤；
  (4) 目前值：`kb` 仍為 0 時 `SetExtendedMemoryKB(0)` 是無害空呼叫；設成 `kb > 0` 之後再呼叫（含改回 0、改成別的值）回錯誤。
- `kb == 0`：行為與本規格之前完全相同。
- `kb > 0` 時 `SetExtendedMemoryKB` 本身完成三件事：建立 `Ext`、把現有 `hma` 陣列內容複製到 `Ext[0:HMASize]`、
  在 `StubSeg:XMSHookEntryOff` 寫入 §2.3 的 8 bytes 入口。入口不由 `Install()` 寫，因為 `initVectors` 在
  `New()` 內、早於本設定。
- `kb > 0`：新增 `Ext []byte`，大小 `kb*1024`，代表實體位址 `0x100000` 起的延伸記憶體；建立時全為 0
  （不套 `MemPoison`）。
  - HMA 改為 `Ext[0:HMASize]`：`hmaAddr` 命中時讀寫 `Ext`，不再使用獨立的 `hma` 陣列。`Read8`／`Write8`、
    `Peek8`（`internal/machine/peek.go`）三處都改。實模式可達範圍不變：A20 開時只有
    `0x100000`–`0x10FFEF`，`0x10FFF0` 以上照舊折回低記憶體。
  - 新增實體位址存取 `ReadPhys8／16／32`、`WritePhys8／16／32(a uint32)`：
    （2026-10-15 修訂：`a < 0x100000` 時改走 `432-linear-memory-and-paging` 的實體層，不再經 `Read8`／`Write8`。）
    - `a < 0x100000`：走 `Read8`／`Write8`（含 VGA、ROM、監看）。
    - `0x100000 ≤ a < 0x100000+len(Ext)`：讀寫 `Ext`。
    - 其他：讀回 `0xFF`；寫入丟棄並累加 `UnmappedPhysWrites`（診斷計數，不進狀態，與 `ROMWrites` 同類）。
    - 不套 A20 遮罩，是純記憶體視圖。分頁匯流排（S3）自行處理 A20。
    - 多 byte 存取逐 byte 組成，跨越上述區段邊界時各 byte 各自歸類。
  - 已知限制：HMA 與 `Ext` 的寫入不進 `WatchWrites` 與寫入計數（與現行 HMA 相同）。

### 2.2 XMS 區塊實體化（`kb > 0` 時）

行為以 DOSBox-X `src/ints/xms.cpp` 為準（引用行號為本 repo 審查時的 `DOSBox-X-MCP-Debugger/dosbox-src`）。

- 配置區：`Ext` 中 HMA 之後，實體 `0x110000` 起到 `Ext` 結尾。粒度 4 KB 頁，`kb` 向上取整頁數
  （`xms.cpp:239`）。
- 選洞照 DOSBox-X 的 `BestMatch`（`src/hardware/memory.cpp:1029-1064`）：從低位址掃描，遇到頁數剛好相等的空洞
  立即採用；否則取能容納的最小空洞，同大小取位址較低者。結果與 map 走訪順序無關。
- Handle：1 到 64（含），沿用 dosgolem 現值 `xmsMaxHandles`（DOSBox-X 預設是 `1..49`，`xms.cpp:38-42`、`235`）；
  取最小的空 handle；用完回 `BL=A1h`。
- 錯誤碼一律用 DOSBox-X 常數（`xms.cpp:70-86`）：`80h` 未實作、`A0h` 空間不足、`A1h` handle 用完、
  `A2h` handle 無效、`A3h`／`A4h` 來源 handle／位移無效、`A5h`／`A6h` 目的 handle／位移無效、`A7h` 長度無效、
  `AAh` 未鎖定、`ABh` 已鎖定。失敗時 `AX=0`、`BL=碼`；成功 `AX=1`。

| AH | 規則 | 依據 |
|---|---|---|
| `08h` | `AX` = 最大連續空間 KB、`DX` = 總空間 KB（HMA 不計）；總空間為 0 時 `AX=0`、`BL=A0h`。DOSBox-X 會截到 65535（`xms.cpp:578-583`），`kb ≤ 64512` 時不觸發 | `xms.cpp:202-208` |
| `09h` | `DX` = KB。允許 0 KB：不佔頁，基底為 `BestMatch(1)` 的結果，沒有空頁時為 0。不清零（沿用該處舊內容） | `xms.cpp:231-258`；`memory.cpp:1190-1191` |
| `0Ah` | handle 無效 `A2h`；已鎖定 `ABh`；否則釋放頁 | `xms.cpp:260-269` |
| `0Bh` | 描述子由 `DS:SI` 經 `Linear` 讀取。長度奇數 `A7h`；先檢查來源三項，再檢查目的三項：handle 非 0 時依序為 handle 無效（`A3h`／`A5h`）、位移 ≥ 區塊大小（`A4h`／`A6h`）、長度超過區塊剩餘（`A7h`），區塊大小以請求的 KB×1024 計，不以取整後的頁數；handle 為 0 時位移是實模式 `段:偏移`，換算位址加長度超過 `0x10FFF0` 回 `A7h`。檢查通過後逐 byte 由低位址往高位址複製（重疊時照此順序，不做 memmove）；複製期間 A20 視為開啟，結束還原 | `xms.cpp:272-353`；逐 byte 方向見 `src/hardware/memory.cpp:910-912` |
| `0Ch` | handle 無效 `A2h`；鎖定計數小於 255 才加一；`DX:BX` = 區塊實體基底 | `xms.cpp:356-361` |
| `0Dh` | handle 無效 `A2h`；計數為 0 回 `AAh`；否則減一 | `xms.cpp:363-370` |
| `0Eh` | handle 無效 `A2h`；`BH` = 鎖定計數、`BL` = 空 handle 數、`DX` = KB | `xms.cpp:372-382` |
| `0Fh` | handle 無效 `A2h`；已鎖定 `ABh`；新舊頁數相等時只更新 KB（`memory.cpp:1225`）；新大小能原地容納（縮小或後方有空頁）就原地，否則依 `BestMatch` 另找空洞，找洞時舊頁仍算占用，配到後複製原內容再釋放舊頁（`memory.cpp:1263-1267`），都不行回 `A0h`。占 0 頁的區塊（`09h` 配的 0 KB、或縮到 0 的區塊）擴大時一律視為新配置，以 `BestMatch(pages)` 找洞，不論原基底值。這點刻意偏離 DOSBox-X：DOSBox-X 的 0 KB 區塊持有一個未標記占用的頁（`xms.cpp:244` 的 `MEM_GetNextFreePage`），擴大到 1 頁時直接成功，會與其他區塊共用同一頁。新大小為 0 時釋放全部頁，handle 保留、大小 0、基底 0（Lock 回 0）；DOSBox-X 此時把底層 handle 設成 -1（`memory.cpp:1207-1220`），dosgolem 改用基底 0 以免位址溢位 | `xms.cpp:384-394`；`memory.cpp:1263` |
| `10h`、`11h` | 維持現行 UMB 行為（`C000`–`CFFF`） | 不在範圍 |
| `03h`–`07h` | 維持現行 A20 簿記。8042 直接寫入（§2.4）不更新這套簿記，與真機一致 | |

`kb == 0` 時 XMS 行為完全不變（含既有的奇數長度照搬、合成 Lock 位址），本節只在 `kb > 0` 生效。

`int 15h AH=88h` 在 `kb > 0` 時回 `AX=0`：XMS 存在時 DOSBox-X 回 0（`src/ints/bios.cpp:7098-7102`，`other_memsystems ? 0 : size_extended`；`other_memsystems` 由 XMS 初始化經 `BIOS_ZeroExtendedSize(true)` 遞增，`xms.cpp:839`、`bios.cpp:7978`）。`kb == 0` 維持回 `xmsTotalKB`（8192），兩者差異是刻意的：`kb == 0` 保留既有收據的行為，`kb > 0` 照 DOSBox-X。`internal/dos/font.go` 的 `AH=88h` 註解（「回 0 會被讀成沒有延伸記憶體」）要同步改寫成這兩種語意。觸發案例不受影響：SCCD 只在沒有 XMS 時才呼叫 `AH=88h`（`002-sccd-launch-flow.md` §1 步驟 12a）；`AH=87h` 在 `kb > 0` 時以 `ReadPhys`／`WritePhys` 存取
24 位元實體位址。`kb == 0` 時兩者不變。

### 2.3 可掛鉤的 XMS 入口（`kb > 0` 時）

- StubSeg 佈局的唯一權威是 `internal/machine/machine.go` 的特殊 stub 常數區塊（main `81c85be`）：
  `0x40C` XMS 入口、`0x410` 回呼返回哨兵、`0x414`／`0x418` 字型 entry、`0x420`–`0x442` BIOS 計時器、
  `0x448` 系統設定表、`0x450` InDOS、`0x460` DBCS 表、`0x480`–`0x4A1` 國別資訊表，上界 `0x4F0`
  （`EnvMCBSeg`）。
- 本規格之後的佈局（由 459 修訂，2026-10-23；以 `5b88abd` 的 `machine.go:121-133` 註解與 `140-198` 行常數為準）：
  - BIOS 計時器在保護模式剖面為 40 bytes，佔 `0x420`–`0x447`（459 §3.3a）。
  - `0x4B8`–`0x4C5` 鍵盤攔截客體碼（447 §2.2）；保護模式剖面為 19 bytes，佔 `0x4B8`–`0x4CA`（459 §3.3a）。
  - `0x4CC`–`0x4EC` 保護模式剖面的 IRQ12 三段樁 `Mouse74Off`、`Mouse74RetOff`、`MouseUIROff`（459 §3.6）。
- 在該區塊新增常數 `XMSHookEntryOff = specialStubBase + 0xB0`，並在常數區塊上方的佈局註解表加一行。新入口佔 `0x4B0`–`0x4B7`，內容
  `EB 03 90 90 90 CD F5 CB`，落在國別表之後、上界之前的空段。
- 舊位置 `XMSTrapOff`（`0x40C`）的 `CD F5 CB` 保留不動，常數意義不變（`TestXMSEntryIsFarCallNotInterrupt`
  釘住它）。`0x40C` 之後 1 byte 就是回呼哨兵（`0x410`），放不下 8 bytes。
- `int 2Fh AX=4310h` 回 `ES=StubSeg`、`BX` = Machine 回報的目前入口位移（新增 `XMSEntryOff()`：`kb == 0` 回
  `XMSTrapOff`，`kb > 0` 回 `XMSHookEntryOff`）。
- 依 §1 的 SCCD 證據：第一個 byte 是 `EBh`，SCCD 以 `0x4B0+2+3 = 0x4B5` 為原處理常式，即 `CD F5 CB`；
  改寫的 5 bytes 是 `0x4B0`–`0x4B4`，不碰 trampoline。`int F5h` 的分派只看中斷號，與入口位置無關。

### 2.4 8042 A20（`kb > 0` 時）

- 任何 `out 64h,v` 先清除「等待輸出埠資料」旗標；`v == D1h` 時再設起。
- 旗標為真時的下一個 `out 60h,v`：以 `v` 的位元 1 呼叫 `SetA20`；位元 0 為 0（要求重設）只記錄，不重設；
  清除旗標。
- 依據：DOSBox-X `src/hardware/keyboard.cpp` 第 825–827 行（`0xd1` → `CMD_SETOUTPORT`）、第 666–678 行
  （`MEM_A20_Enable((val & 2)>0)`，位元 0 為 0 時預設只記錄警告）。
- 狀態埠 `64h` 的位元 1 現行已恆為 0（回 `0x14`／`0x15`），不改。
- 不支援：`D0h`（讀輸出埠）、port `92h`（fast A20）、`int 15h AX=2400h`–`2403h`。讀改寫型開法
  （`D0h` 讀出再 `D1h` 寫回）會讀到鍵盤資料埠的值，列為已知限制。
- 旗標是 Machine 欄位，進 `Snapshot` 與狀態檔（§3）。

### 2.5 不安裝內建 EMM（選項）

`Machine.SetEMSPresent(present bool) error`，預設 true。檢查順序：先看 `programLoaded`（為真一律回錯誤，不論引數），再看目前值：目前 true 而 `present == true` 不做事；目前 true 而 `present == false` 執行下列撤銷；目前 false 而 `present == false` 不做事；目前 false 而 `present == true` 回錯誤。設為 false 時：

- 撤銷 `initVectors` 寫入的三項：`EMSSeg*16` 的 `CD F6 CF`、`EMSSeg*16+0Ah` 的 `EMMXXXX0`（還原為
  `MemPoison` 值）、`int 67h` 向量改指 `StubSeg:StubOff(0x67)`。
- 閘放在 `emsCall` 的入口，`int 67h`（`dos.go` 的 `case 0x67`）與 EMM trampoline 的 `int F6h`（`case 0xF6`）都經過它：不呈現 EMM 時不服務，暫存器原樣返回（等同無驅動時的 `iret`），並以 `d.noteCPU(c, 0x67, ah, al)` 記一筆（附暫存器），讓呼叫在診斷中看得見。
- `EMMXXXX0` 的兩個 Go 側判斷點（`files.go` 的 `open`、`fileAttr`）不再視為裝置：`open` 回「找不到檔案」、
  `fileAttr` 回錯誤 2。`isEMMDevice` 改為能讀 Machine 旗標。

## 3. 介面與狀態

- `machine`：`SetExtendedMemoryKB`、`ExtendedMemoryKB`、`ReadPhys*`、`WritePhys*`、`UnmappedPhysWrites`、
  `SetEMSPresent`、`EMSPresent`、`XMSEntryOff`。
- `cmd/probe`：`-ext-mem-kb N`、`-no-ems`。兩者在 `machine.New()` 之後、`if *loadState == ""` 區塊之外呼叫，
  所以同時涵蓋「載入程式」與「載入狀態檔」兩條路徑，且早於 `LoadEXE`／`LoadCOM`／`state.Load` 與 `d.Install()`。
  抽出 `configureMem(m *machine.Machine, kb int, noEMS bool) error`（依序呼叫 `SetExtendedMemoryKB`、`SetEMSPresent`），
  由 `main` 在上述位置呼叫；測試直接呼叫它再 `state.Load`。`kb` 範圍由 `SetExtendedMemoryKB` 自己檢查，不另設驗證函式。可與 `-load-state` 同給：probe 先依旗標設定機器，再 `LoadState`；
  旗標值必須與狀態檔一致，不一致由 `LoadState` 回錯誤（與 `429` 的 `-dos-files` 不同：`Ext` 與 EMS 旗標在載入後
  仍影響執行，必須在載入前就設好）。載入 `kb > 0` 的狀態檔時必須給相同的 `-ext-mem-kb`。
- 狀態檔（`internal/machine/state.go`）：**不升版號**。新增欄位 `Ext`、`EMSAbsent`、`KbdAwaitOutport`，
  JSON 標 `omitempty`。`LoadState` 的 `Ext` 長度與 `EMSAbsent` 比對放在任何寫入之前（現有 `len(s.Mem)` 檢查旁），
  失敗時機器維持原狀。舊狀態檔照常讀得進來（先例見 `state.go` 對 `Hi` 的註解）。`kb > 0` 時不另存 `HMA` 欄位
  （`Ext` 已含 HMA）。
- gob 在串流開頭寫出 struct 的型別描述，含全部欄位名；即使新欄位都是零值，`SaveState`（機器與 DOS 兩側）的位元組
  也會改變。既有收據用的是 `session` 的摘要與記憶體雜湊，不是 gob 原始位元組；`machine.StateDigest`、
  `dos.StateDigest` 是解碼後的 JSON，新欄位加 `omitempty` 後 `kb == 0` 不變。實作分兩個 commit：先提交「只加欄位、
  不改行為」的 commit，在其中更新錨點測試的兩個 gob 位元組常數並註明原因；之後的行為 commit 必須讓全部錨點維持不變。
  machine 與 DOS 兩側所有新欄位與新型別（含 slice 的元素型別）都要在這一個 commit 加完，之後再加或改欄位、型別會讓
  gob 位元組第二次改變。這個 commit 的 `SaveState` 不寫入任何新欄位，由「解碼後新欄位皆零值」測試把關。錨點測試檔頭
  註解同步改寫，區分「語意錨點（記憶體雜湊、兩個 `StateDigest`）永不變」與「gob 位元組可因型別描述改變一次」。
- 載入規則：狀態檔 `Ext` 長度與機器不同（含一邊為 0）時回錯誤；`EMSAbsent` 與機器設定不同時回錯誤。
- DOS 側狀態（`internal/dos/state.go`）：現有 `EMB`（只讀舊檔）與 `EMBList []embState`（`kb == 0` 的 Go 端內容）不變。
  新增欄位 `XMSBlocks []xmsBlockState`（元素：handle、基底頁、頁數、KB、鎖定計數），依 handle 排序（比照 main `ca3696c`），
  JSON 標 `omitempty`（`dos.StateDigest` 對整個 struct 做 `json.Marshal`）。`kb > 0` 時存檔只寫 `XMSBlocks`；載入時
  `EMB` 或 `EMBList` 非空而機器 `kb > 0`、或 `XMSBlocks` 非空而機器 `kb == 0`，回錯誤。
- `Snapshot`／`Restore`（`snapshot.go`）同步涵蓋 `Ext`、8042 旗標與 EMS 旗標（`programLoaded` 已由 429 涵蓋）。`kb > 0` 時快照要複製
  整個 `Ext`，成本隨 `kb` 線性增加，寫進該函式註解。`Restore` 沒有錯誤回傳：`Ext` 長度與機器不同時 panic，
  訊息寫明兩邊長度。
- `StateDigest` 對 `Ext` 先算 SHA-256 再放入摘要，不把整個 `Ext` 做 JSON base64。
- 已知限制：
  - `oracle.State` 只快照 Machine，不含 DOS 側的 XMS 配置表；`kb > 0` 時倒回 `Ext` 而配置表不倒回，兩邊會不一致
    （現行 EMB 已有同樣缺口，`Ext` 讓影響變大）。
  - 舊版執行器讀到 `kb > 0` 的新狀態檔時，gob 會安靜丟掉 `Ext`，HMA 變成零。
- `session/digest.go` 新增 `ExtSHA256`，`kb == 0` 時為空字串，不影響既有收據。

## 4. 不在範圍

- 分頁、保護模式、cpu386 對 `Ext` 的存取（S1–S3）。
- `int 2Fh AX=0803h`、SFT（`429`）。
- UMB（`AH=10h`／`11h`）。
- `oracle.Options`、`cmd/run`、`session` 的選項接線（只做 `cmd/probe`）。
- EMS 頁框位置：JEMM 自己提供頁框 `E000h`。`429` 的 SFT 第二張表在 `F000` 段，不與頁框重疊。

## 5. 驗收

測試放置：`internal/machine` 放 `ReadPhys`、`Ext` 邊界、8042、機器側存讀與快照；`internal/dos` 放 XMS 服務、
掛鉤、`SetEMSPresent` 的 DOS 側（用 `newTest`）；同時涉及兩者的存讀往返放 `internal/state`。

1. 預設不變（`kb == 0`、`SetEMSPresent` 未呼叫）：
   (a) 既有測試全綠；`0x40C` 仍是 `CD F5 CB`，`0x4B0`–`0x4B7` 未寫入，`int 2Fh AX=4310h` 回 `BX=0x40C`。
   (b) 錨點測試 `internal/state/baseline_anchor_test.go`（前置 commit `cbce2d9`）：`m.Mem` SHA-256、
       `machine.StateDigest` 全程不變；新增 `dos.StateDigest` 的錨點（在加欄位 commit 之前先補，值由未修改的程式算出），
       全程不變；`SaveState`、`dos.SaveState` 的 gob 位元組錨點只在「只加欄位」的 commit 中更新一次，之後不變。
       該 commit 另加一個測試：解碼後新欄位皆為零值。算 `dos.SaveState` 前把 `d.Root` 設成固定字串
       （`newTest` 用 `t.TempDir()`，路徑會寫進狀態檔）。與 `429` 驗收 2(b) 共用夾具與前置 commit。
2. `SetExtendedMemoryKB`（每個值各用新機器）：0、4096、64512 成功；4095、4097、64516、65536、-4 失敗；載入後呼叫失敗；
   設成 4096 之後再呼叫 0、4096、8192 都失敗；`kb` 為 0 時再呼叫 0 成功且無副作用。
3. 實體位址：`kb = 8192`，`ReadPhys8`／`WritePhys8` 在 `0x0FFFFF`、`0x100000`、`0x10FFEF`、`0x10FFF0`、
   `0x8FFFFF`、`0x900000`（越界，讀 `0xFF`、寫入計數加一）逐點驗；A20 開時實模式 `FFFF:0010` 與
   `WritePhys8(0x100000)` 是同一 byte；寫 HMA 不改 `0x110000`，反之亦然；`Peek8` 與 `Read8` 一致。
4. XMS（`kb = 8192`）：§2.2 表逐列各一個正例與負例，包括：配 5,000 KB 後 Lock 基底 ≥ `0x110000`、
   ≤ `0xFE0000`、4 KB 對齊；`WritePhys` 寫入後 XMS Move 讀回；兩區塊不重疊；釋放中間區塊後再配置落在洞內；
   容量耗盡 `A0h`；handle 用完 `A1h`；Move 的 `A3h`–`A7h` 各一例且鄰近區塊與 HMA 不變；
   重疊搬移依逐 byte 方向的期望值；`0Fh` 原地擴大、搬移、新大小 0 各一例；鎖定計數到 255 停止；`BestMatch` 的
   「剛好相等優先」與「最小可容納」各一例；0 KB 區塊擴大到 1 頁後與其他區塊不重疊；`0Bh` 來源與目的兩側都錯時回來源的錯誤碼。
5. 掛鉤入口：以真正執行指令的方式（`Step` 跑遠呼叫，不直接呼叫 `d.handle`）驗證：合成程式依 §2.3 的 SCCD 邏輯
   改寫入口並鏈回 `+5`，服務照常；負對照：掛鉤不鏈回時服務不執行；在已套用 `EA` 改寫的入口上鏈到 `+3` 時失敗
   （未改寫前 `+2`–`+4` 都是 NOP，會成功）。另加版面守門測試（比照 `internal/dos/stub_layout_test.go`）：
   `0x4B0`–`0x4B7` 不與常數區塊中任何項目重疊，且小於 `EnvMCBSeg` 換算的 `0x4F0`。`Install()` 後入口
   8 bytes 完整，且相鄰的國別資訊表與系統設定表內容不變。
6. 8042：`D1h`＋`DFh` 開、`D1h`＋`DDh` 關；`D1h` 後接另一個 `64h` 命令再 `out 60h` 不影響 A20；位元 0 為 0
   不重設；`kb > 0` 時既有鍵盤測試照常通過。
7. `SetEMSPresent`：§2.5 檢查順序表的四種情形與「載入後呼叫」各一例；`SetEMSPresent(false)` 之後：`int 67h` 向量段的 `:000A` 不是 `EMMXXXX0`；呼叫 `int 67h AH=40h` 後暫存器不變；
   開 `EMMXXXX0` 回找不到檔案；`fileAttr` 回 2；`MemPoison = 0xCC` 時 `EMSSeg` 區還原成 `0xCC`。
8. 狀態往返（`internal/state`）：`kb = 8192` 配兩個區塊、鎖一個、寫內容後存讀，`Ext`、配置表、鎖定計數、
   8042 與 EMS 旗標相同；同一狀態連存兩次位元組相同（比照 `TestDOSSaveStateIsByteIdentical`）；`kb` 不同、`EMSAbsent` 不同、`EMBList` 非空的狀態檔載入到 `kb > 0`、`XMSBlocks` 非空的狀態檔載入到 `kb == 0` 各回錯誤；
   `Snapshot`／`Restore` 後 `Ext` 還原。
9. 觸發案例（需原版，缺檔 skip，不算進第 11 項，另附 probe 收據）：`SCCD.EXE` 搭配
   `-dos-files 30 -ext-mem-kb 8192 -no-ems` 通過 SFT、EMM、XMS 檢查與映像 `0x37E`，`call 0:12` 抵達 JEMM，
   停在第一個 dosgolem 尚未支援的系統指令（預期是 img `0x41E8` 的 `sgdt`）。
10. probe（`cmd/probe` 測試）：呼叫 `configureMem` 後 `state.Load`，以合成狀態檔驗證 `-ext-mem-kb` 與狀態檔不同、漏給、多給，
    以及 `-no-ems` 不一致時，載入各回錯誤，一致時成功。
11. `tools/go.sh test ./internal/machine ./internal/dos ./internal/state ./cmd/probe` 全綠。

## 6. 實作收據（2026-10-15）

實作 commit 見 `git log` 的「feat: 430」；前置 `cbce2d9`（錨點）、`f1e8e1b`（`dos.StateDigest` 錨點）、`5b8f878`（只加欄位）。

### 實作期間的處置

1. 成功時的 `BL` 照 DOSBox-X `SET_RESULT`：`08h`、`09h`、`0Ah`、`0Dh`、`0Fh` 成功時 `BL=0`；`0Bh`、`0Ch` 不動 `BL`；`0Eh` 的 `BL`
   是空 handle 數。`09h` 失敗時 `DX=0`。
2. `kb == 0` 時 `ReadPhys*` 在 1 MB 以上讀 `0xFF`、`WritePhys*` 丟棄並計數，不暴露獨立的 `hma` 陣列。
3. handle 0 的「位址加長度超過 `0x10FFF0`」以 64 位元計算；DOSBox-X 以 32 位元計算，長度接近 4 GB 時會溢位放行，視為其缺陷。
4. 0 頁區塊的 `BasePage` 以 0 表示「沒有」（相對頁號 0 是 HMA，不會是合法配置）；Lock 在基底為 0 時回 0。
   `DOS.LoadState` 另檢查 `XMSBlocks` 一致性（handle 1..64、`Pages == ceil(KB/4)`、頁範圍在 `Ext` 內且不重疊、鎖定計數
   0..255），都在任何寫入之前。
5. `state.Load` 先載機器再載 DOS；DOS 側互斥檢查失敗時機器已載入。整體原子性未做，列為已知限制。
6. 8042 位元 0 為 0 的重設要求只記在 `PortLog`。`D1h` 狀態機只在 `kb > 0` 時生效。
7. `kb > 0` 時 `DOS.SaveState` 不寫 `EMBList`。
8. `kb > 0` 的 `0Bh` 與 `int 15h AH=87h` 沿用 `XMSMoves` 診斷記錄。

### 驗收結果

- 第 1–8、10、11 項：測試通過（容器內 `go test ./internal/... ./cmd/probe ./session ./oracle`）。錨點常數在行為 commit 不變。
- 突變自檢：`BestMatch` 改首次適配、入口第一個 byte 改 `0x90`，對應測試皆失敗，已還原。
- 全 repo 另有 `apps/phantasie/cmd/phantasie-play`、`cmd/buckrogers-play`、`frontend/ebiten` 因無顯示器（GLFW）失敗；
  在 main `81c85be` 上同樣失敗，是既有的 `xvfb-run` 需求，非本規格造成。
- 第 9 項（觸發案例）：`SCCD.EXE -dos-files 30 -ext-mem-kb 8192 -no-ems` 通過 SFT 與 EMM 檢查，走「沒有 EMM」路徑，停在
  `0294:07A9`（映像 `0x1FE9`）的 `smsw ax`：dosgolem 的 80386 子集沒有這道指令。尚未到 XMS 檢查與 JEMM，下一關卡屬 CPU 規格。
  本規格因此維持 READY。
