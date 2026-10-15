# 433 — 既有核心的實模式系統指令與實模式→保護模式交棒

狀態：**DRAFT**（第 4 版，依第 3 輪複審修訂）
日期：2026-10-15
前置：[`431-hybrid-cpu-protected-mode`](431-hybrid-cpu-protected-mode.md)（契約）、
[`432-linear-memory-and-paging`](432-linear-memory-and-paging.md)（已實作）、
[`197-cpu386-real-mode-subset`](197-cpu386-real-mode-subset.md)

DOSBox-X 引用一律指 `DOSBox-X-MCP-Debugger/dosbox-src/src/` 下的檔案。

---

## 1. 為什麼要做

觸發案例在實模式就碰到系統指令：SCCD 的 `smsw ax`（映像 `0x1FE9`，dosgolem 目前停在這裡，`430-physical-memory-and-xms-unification`
§6 收據）。JEMM 在實模式依序執行 `sgdt`（img `0x41E8`）、`sidt`（`0x41ED`）、`lgdt`（`0x41F2`，16 位元形式）、`lidt`（`0x41F7`）、
`mov eax,cr0`（`0x41FC`），再以 `mov cr0,eax`（`0x4203`，`0F 22 C0`）設 PE；`lldt`（`0x420C`）在 PE=1 之後（`strike_commander`
的 JEMM 解壓映像 objdump，第 1 輪審查核對）。`431-hybrid-cpu-protected-mode` 定案由既有核心在實模式補這幾道，PE 設起時交棒給 cpu386。

本規格補：(1) Machine 的完整 `SysRegs`；(2) 既有核心在實模式的系統指令；(3) 交棒介面與交棒點；(4) `-pm-profile` 旗標。
cpu386 的分段模式與作用中核心的觀測視圖在 `434-cpu386-segmented-mode`；本規格的交棒在 434 之前停在明確的失敗即關閉。

## 2. `SysRegs`

在 432 已有的 `CR0`、`CR3`、`EFlagsHi`、`pmCPL` 之外，Machine 增加：

| 欄位 | 規則 | 初值 | 依據 |
|---|---|---|---|
| `CR2` | 讀寫 | 0 | |
| `DR0`–`DR3` | 讀寫 | 0 | `cpu/cpu.cpp:3644-3653` |
| `DR6` | 寫入為 `(v\|FFFF0FF0h)&FFFFEFFFh`；`DR4` 是別名 | `FFFF1FF0h` | `cpu.cpp:2708`、`3651` |
| `DR7` | 寫入為 `(v\|400h)&FFFF2FFFh`；`DR5` 是別名 | `00000400h` | `cpu.cpp:2713`、`3653` |
| `GDTR` | 基底 32 位元、界限 16 位元 | 基底 0、界限 0 | |
| `IDTR` | 同上 | 基底 0、界限 `03FFh` | `cpu.cpp:3641-3642` |
| `LDTR`、`TR` | 選擇子＋快取（基底 32、界限 32、屬性 16）；本規格只保存，載入在 434 與後續 S4 | 全 0 | |

`DR7` 寫入只保存，不觸發 `#DB`（後續 S6）。所有寫入經 Machine 的方法（431 §4）。

實模式的 `INT` 與 `tick()` 一律讀 `0000:0000` 的 IVT，不看 `IDTR`。這是刻意差異：DOSBox-X 與真機在實模式以 `IDTR` 基底取向量
（`cpu/cpu.cpp:1294-1296`）。dosgolem 的服務層向量檢查與 `tick()` 都假設 IVT 在 0，所以不改。為了不讓差異悄悄影響結果：profile 開啟、
`IDTR` 基底不為 0、仍在實模式時，若發生軟體 `INT` 或要遞送 IRQ，失敗即關閉並寫明原因。JEMM 的 `lidt`（`0x41F7`）與 `mov cr0`（`0x4203`）之間
有兩道指令（`0x41FC`、`0x41FF`，不含 `mov cr0`），觸發案例收據記錄這段期間 `IF` 的值。

守門機制（「實模式」判定為 `Sys != nil && Sys.CR(0)&1 == 0`；V86 的 IDT 遞送屬 435，不在此範圍）：

- CPU 例外與軟體中斷：檢查放在 `doInt`（`ops.go`，在 `IntHook` 之前，所以 DOS 服務類 `INT` 也會被擋）、`divideError`（`ops.go`、`wide.go` 兩處
  `INT 0`）與 186 `BOUND`（`ops186.go` 的 `INT 5`）的最前面，以 `failf` 回報，不呼叫 `Interrupt`。
- IRQ：`CPU.Interrupt` 沒有回傳值，且呼叫點在 `CPU.Step()` 之外（`machine.go` 的 `tick`、`keyboard.go`、`keyhold.go`、`le_dma_clock.go`）。
  守門放在 Machine 層，送出前檢查；條件成立時不呼叫 `Interrupt`，把錯誤存進 `Machine.Step` 這一步要回傳的欄位，該步回錯誤。

### 2.1 `PM` 版本 2

`internal/machine/pmstate.go` 的 `encodePM` 改輸出版本 2（80 bytes，小端序），`decodePM` 支援版本 1（缺的欄位取上表初值）與版本 2：

| 位移 | 大小 | 欄位 |
|---|---|---|
| 0–15 | 16 | 版本 1 的內容，版本號改為 `2` |
| 16 | 4 | `CR2` |
| 20、24、28、32 | 各 4 | `DR0`–`DR3` |
| 36 | 4 | `DR6` |
| 40 | 4 | `DR7` |
| 44 | 4 | `GDTR` 基底 |
| 48 | 2 | `GDTR` 界限 |
| 50 | 4 | `IDTR` 基底 |
| 54 | 2 | `IDTR` 界限 |
| 56 | 2 | `LDTR` 選擇子 |
| 58 | 4 | `LDTR` 基底 |
| 62 | 4 | `LDTR` 界限 |
| 66 | 2 | `LDTR` 屬性 |
| 68 | 2 | `TR` 選擇子 |
| 70 | 4 | `TR` 基底 |
| 74 | 4 | `TR` 界限 |
| 78 | 2 | `TR` 屬性 |

新增 `sysRegs` 結構，收齊上表 `CR2`、`DR`、`GDTR`、`IDTR`、`LDTR`、`TR` 欄位。Machine 持有一份，`pmFields` 內嵌它，`Snapshot()` 與 `applyPM`
整份複製，不逐欄賦值。profile 關閉時 `applyPM` 套用的 `sysRegs` 用上表初值，不用零值（不影響行為，避免 `IDTR` 界限、`DR6` 出現 0）。

`LoadState` 的驗證比照 432 §10 第 6 項，另加：版本 2 長度至少 80；`DR6 & FFFF0FF0h == FFFF0FF0h`（bit 12 不驗）；`DR7` 的 bit 10 為 1 且 bit 12、14、15 為 0；`LDTR`、`TR` 屬性的 bit 8–11 與 bit 13 為 0。
版本 1 的狀態檔載入後再存檔會變成版本 2，屬預期。432 的 `PM` 測試要改：`TestPMV1Layout` 改以手造的版本 1 位元組驗解碼相容；往返測試的長度斷言
由 16 改 80；「未知版本號」的例子改用 3；「長度不足」「尾端多餘 bytes」依版本 2 重新檢查。`StateDigest` 以 JSON 編碼整個 `machineState`，
`PM` 在 profile 開啟時會進摘要，版本 2 會改變 profile 開啟時的摘要；profile 關閉時 `PM` 為 nil，摘要不變。目前沒有 profile 開啟的摘要收據。

## 3. 介面

### 3.1 既有核心的 `SysBus`

既有核心在結構尾端新增可選欄位 `Sys SysBus`（nil 時行為完全不變）。Machine 只在 profile 開啟時裝上（`SetProtectedModeProfile(false)` 時拿掉）。方法：

- `CR(n int) (uint32, error)`（回原始值；`mov r32,cr0` 的 `\|7FFFFFF0h` 與 `CR3` 的遮罩在指令處理函式做）、`SetCR(n int, v uint32) error`
- `DR(n int) uint32`、`SetDR(n int, v uint32)`
- `GDTR() (base uint32, limit uint16)`、`SetGDTR(base uint32, limit uint16)`、`IDTR()`、`SetIDTR`
- `User() bool`：目前是否為使用者模式（432 的 `cplUser`）。實模式恆為 false。

### 3.2 交棒介面

在 `internal/machine` 定義：

```go
type Handoff struct {
    R      [8]uint32  // EAX…EDI（cpu386 的順序）
    EIP    uint32
    EFlags uint32
    Seg    [6]uint16  // cpu386 順序：CS, DS, ES, FS, GS, SS
    Cache  [6]SegCache
}
type SegCache struct{ Base, Limit uint32; Attr uint16 }
// Attr：bits 0–7 為 386 描述元的 access byte；bits 8–11 恆 0；bit 12 AVL；bit 13 恆 0；
// bit 14 D/B；bit 15 G。Limit 是已依 G 展開的有效界限。Cache 的索引與 Seg 相同（cpu386 順序）。
// LDTR、TR 的屬性用同一編碼。
type PMCore interface {
    Enter(h Handoff) error // 接手執行
    Step() error
}
```

Machine 有未匯出欄位 `pm PMCore`（434 填入 cpu386 的轉接；測試以替身注入）。`SetProtectedModeProfile` 切換成功時，§2 的所有 `SysRegs`
（含 `CR2`、`DR`、`GDTR`、`IDTR`、`LDTR`、`TR`）一律回初值，與 432 的處置一致。段索引映射函式放 `internal/machine`，以具名欄位逐一對應：
既有核心 `Seg[ES]`、`Seg[CS]`、`Seg[SS]`、`Seg[DS]`、`FSeg`、`GSeg` 對 cpu386 的 `SegES`、`SegCS`、`SegSS`、`SegDS`、`SegFS`、`SegGS`。不用迴圈，
`cpu.FS`、`cpu.GS` 常數不是 `Seg` 的索引。

## 4. 指令

只在 `Sys` 非 nil 時生效；實作位置在 `internal/cpu/ops0f.go` 的 `default` 分支內，先判 `c.Sys != nil` 再交給新檔的處理函式，未處理的照現況
失敗即關閉，`Sys == nil` 時錯誤訊息與現況逐字相同。只在 `Model80386` 下可達（`Model8086` 時 `0F` 是 `POP CS`，SingleStepTests 不受影響）。

「特權」指令在 `Sys.User()` 為真時失敗即關閉，診斷寫明「V86 路徑在 435」；實模式恆可執行。

| 指令 | 行為 | 特權 | 依據 |
|---|---|---|---|
| `smsw r/m16`（`0F 01 /4`） | 讀原始 `CR0`。16 位元暫存器形式寫低 16 位元、保留高半；`66` 前綴的暫存器形式寫整個 `CR0`；記憶體形式一律寫 2 bytes | 否 | `core_normal/prefix_0f.h:113-115`、`prefix_66_0f.h:108-110`；`cpu.cpp:2784` |
| `lmsw r/m16`（`0F 01 /6`） | 只改 `CR0` 低 4 位元，不能清除 PE；PE 由 0 變 1 時交棒（§5） | 是 | `cpu.cpp:2788-2796`（`lmsw_allow_clear_pe_bit` 預設 false，`cpu.cpp:70`、`4263-4270`） |
| `sgdt`／`sidt m`（`0F 01 /0`、`/1`） | 存 16 位元界限與完整 32 位元基底（386 不遮成 24 位元） | 否 | `prefix_0f.h:74-81` |
| `lgdt`／`lidt m`（`0F 01 /2`、`/3`） | 16 位元運算元只取 24 位元基底，`66` 前綴取整個 dword | 是 | `prefix_0f.h:85`、`89`；`prefix_66_0f.h:80`、`84` |
| `0F 01` 的其他形式 | 暫存器形式 `/0`、`/1`、`/2`、`/3`、`/5`、`/7` 與記憶體形式 `/5`、`/7`：失敗即關閉（DOSBox-X 對記憶體 `/5` 靜默略過、`/7 invlpg` 在 386 為 `#UD`；刻意從嚴） | — | `prefix_0f.h:72-124`（暫存器分支的 `default: goto illegal_opcode`） |
| `mov r32,crN`（`0F 20`） | `CR0` 讀回為 `CR0 \| 7FFFFFF0h`（386，同 DOSBox-X；與 `smsw` 讀原始值不同）；`CR2` 照存；`CR3` 讀回遮 `FFFFF000h`；`CR1`、`CR4` 起失敗即關閉 | 是 | `cpu.cpp:2653-2693`（`2669` 為 `<486` 的讀回） |
| `mov crN,r32`（`0F 22`） | 經 Machine 的 setter；寫 `CR0` 使 PE 由 0 變 1 時交棒（§5） | 是 | `cpu.cpp:2580-2636` |
| `mov r32,drN`、`mov drN,r32`（`0F 21`、`0F 23`） | 讀寫 §2 | 是 | `cpu.cpp:2695-2749` |
| `clts`（`0F 06`） | 只清 `CR0.TS` | 是 | `prefix_0f.h:151-155` |
| `0F 00` 整組（`/0`–`/7`）、`lar`、`lsl`（`0F 02`、`0F 03`） | 實模式失敗即關閉（DOSBox-X 送 IVT 第 6 項；刻意從嚴，寫入診斷） | — | `prefix_0f.h:22`、`128`、`141` |
| `0F 24`、`0F 26`（TRx） | 失敗即關閉（刻意從嚴：DOSBox-X 對 TR6、TR7 在實模式照常執行，其餘 `#UD`） | — | `prefix_0f.h:222-250`；`cpu.cpp:2751-2782` |

`mov crN`／`mov drN`（`0F 20`–`0F 23`）只讀一個 ModRM 位元組，`mod` 欄位一律視為暫存器形式，不解有效位址、不取位移或 SIB（`prefix_0f.h:196-208`）；
運算元一律 32 位元，與 `66` 前綴無關（`core_normal/helpers.h:201`）。

`sgdt`、`sidt` 的寫入與 `lgdt`、`lidt` 的讀取都以 16 位元偏移各自環繞（既有核心 `read16`／`read32`／`write16`／`write32`），與 DOSBox-X 以線性
位址相加在 `FFFA`–`FFFF` 附近不同，屬已知差異。

## 5. 交棒

- 判定：`SetCR(0, v)` 或 `lmsw` 寫入前後比較 PE，只在 0 變 1 時回哨兵 `ErrModeSwitch`（`errors.Is` 可判）。只改 TS、MP 或寫入不改 PE 時不回。
  這道指令照常完成（`IP` 前進、`CR0` 更新、衍生欄位重算）。
- `Machine.Step`：只在 `CPU.Step()` 回錯誤時才進哨兵判斷（正常步沒有額外成本）。三個回傳點抽成一個輔助函式，順序為：先依 432 §10 第 1 項處理
  分頁紀錄（同一步有紀錄時回 `*PageFaultError`，哨兵不交棒，但依下方規則設停擺旗標）；否則若是 `ErrModeSwitch`，組出 `Handoff`（段快取設為實模式推得的值：基底＝段值×16、
  界限 `FFFFh`、CS 屬性 `009Bh`（present、DPL0、可執行可讀、accessed、16 位元）、其餘段 `0093h`（present、DPL0、可讀寫、accessed、16 位元））：
  - `m.pm` 為 nil：回 `ErrPMCoreNotAvailable`（`errors.Is` 可判），診斷寫明交棒指令的起點位址（JEMM 為 img `0x4203`）、下一道位址（`0x4206`）與 `CR0`；
    並設停擺旗標，之後每次 `Step` 都回同一個錯誤，不讓既有核心在 `PE=1` 下繼續執行。`Restore`、`LoadState` 的處置見本節最後一項。
  - `m.pm` 非 nil：呼叫 `Enter`，`Enter` 回錯誤時同上設停擺並回該錯誤。成功後的最小分派：`PE=1` 且 `VM=0` 時，`Machine.Step` 改呼叫
    `m.pm.Step()`（`tick`、`keyTick` 依 431 §3 在保護模式延後），`m.Steps++` 與 `stepFault` 包裝照舊；`Enter` 成功的那一步回 nil。其餘照舊。
    434 再擴充（觀測視圖、回 V86 的交接）。
- `ErrModeSwitch` 由 `CPU.Step()` 回傳，定義在 `internal/cpu`；`ErrPMCoreNotAvailable` 定義在 `internal/machine`。
- 同一步有分頁紀錄而回 `*PageFaultError` 時，若該步的 CPU 錯誤是 `ErrModeSwitch`（PE 已設起），同樣設停擺旗標；這一步回 `*PageFaultError`，
  之後的 `Step` 回停擺錯誤（`ErrPMCoreNotAvailable`）。
- 停擺與 PM 分派合成一個預先算好的模式值（實模式／V86 照舊、停擺、PM 分派），在每次 PE、VM、`m.pm` 或停擺狀態改變時更新。檢查放在 `Machine.Step`
  的最前面（432 移走殘留分頁紀錄之後、`tick`／`keyTick` 之前），熱路徑只讀它一次；停擺判斷先於分派。
- `Restore`、`LoadState` 不單純清除停擺旗標，而是重新判定：`Sys` 非 nil、`PE=1`、`VM=0` 時設起，否則清除。`m.pm` 非 nil 也設起，因為 cpu386 的
  狀態不在 `Snapshot` 與狀態檔裡，存取屬 434；434 完成前不得以舊的 cpu386 狀態繼續執行。重新判定而設起時沒有交棒當下的位址，診斷用通用訊息，
  仍滿足 `errors.Is(err, ErrPMCoreNotAvailable)`。432 的合成測試（例如 `paging_test.go` 在 `PE=1`、`VM=0` 下 `Restore`）之後不 `Step`，不受影響。

432 既有測試在 `PE=1、VM=0` 下以既有核心執行（例如 `paging_test.go` 的權限驗收）不經交棒路徑，不受停擺旗標影響。

## 6. `-pm-profile` 旗標

`cmd/probe` 加 `-pm-profile`，由新函式 `configurePM(m, on bool) error` 在 `configureMem`（430）之後呼叫 `SetProtectedModeProfile`（`configureMem`
的簽章與既有測試不動）。與 `-load-state` 同給時，值必須與狀態檔一致（`LoadState` 已檢查 profile 與 `PM` 的一致性，旗標只轉述錯誤）。
`-ext-mem-kb 0` 與 `-pm-profile` 同給時由 `SetProtectedModeProfile` 回錯誤。

## 7. 不在範圍

cpu386 的執行與作用中核心的觀測視圖（434）；V86 路徑與 `#GP` 遞送（435）；`ltr`／`lldt` 的保護模式語意與任務切換（S4）；`#UD`／`#GP` 送 IVT；
`DR7` 斷點觸發（S6）。

## 8. 驗收

1. profile 關閉不變：錨點語意常數不變；`Sys == nil` 時 §4 的指令錯誤訊息與現況逐字相同；`Model8086` 與 `Model80186` 下即使 `Sys` 非 nil，
   `0F` 仍照現況（`POP CS` 或失敗）。
2. SingleStepTests 實際執行，步驟：主機先 `mkdir -p dosgolem-sc/testdata/8088`（`.gitignore` 已有 `/testdata/`）；容器以絕對路徑唯讀掛
   `/home/anr2/cht/dosgolem/testdata/8088:/src/testdata/8088:ro`；不帶 `-short`、加 `-v`；確認輸出沒有 SKIP；收據記錄跑過的子測試數與
   `knownGaps` 那一行不變；收尾 `find . -user root` 為空。掛載以 `tools/go.sh` 的
   `DOSGOLEM_EXTRA_MOUNTS="/home/anr2/cht/dosgolem/testdata/8088:/src/testdata/8088:ro"` 傳入（`go.sh` 會檢查來源存在）。
3. `SysRegs`：每個欄位的寫入規則與初值各一例（含 `DR4`→`DR6`、`DR5`→`DR7` 別名）；`PM` 版本 2 逐欄位位移；手造版本 1 可讀且缺欄位取初值；
   §2.1 的驗證條件各一例。`LDTR`、`TR` 在本規格沒有指令寫入路徑，只驗初值、內部欄位賦值後的 `PM` 往返與 `SetProtectedModeProfile`
   切換時回初值。`Snapshot`／`Restore` 與 `SaveState`／`LoadState` 對所有新欄位往返各一例。profile 開啟、`IDTR` 基底不為 0 的實模式失敗即關閉：
   一般 `INT`、被 `IntHook` 攔截的 `INT 21h`、`div 0`、IRQ（`tick`）各一例；IRQ 一例要驗不呼叫 `Interrupt`、錯誤在同一步回傳。
4. 指令：§4 每列至少一例，並含：`smsw` 讀到 `0010h`；16 位元暫存器形式保留高半；`66 smsw eax` 讀整個 `CR0`；`66 smsw [mem]` 只寫 2 bytes；
   `mov eax,cr0` 讀到 `CR0 | 7FFFFFF0h`；`lgdt` 後 `sgdt` 的 24／32 位元往返；暫存器運算元 `lgdt` 失敗即關閉；`mov cr4` 失敗即關閉；`mov eax,cr3`
   遮低 12 位元；`0F 22 40 xx` 之後 `IP` 恰好前進 3；`clts` 只清 TS；`0F 00 /4`–`/7`、`0F 01` 記憶體 `/5`、`/7`、`0F 24`、`0F 26` 失敗即關閉；
   `Sys.User()` 為真時特權指令失敗即關閉（以 432 式的合成 `PE=1`、`VM=1` 狀態）；`lmsw` 不能清 PE（合成 `PE=1` 狀態）。
5. 交棒：合成程式設 PE 後，`m.pm` 為 nil 時 `Step` 回 `ErrPMCoreNotAvailable`，診斷含起點與下一道位址；`IP` 已越過 `mov cr0`；再 `Step` 回同一錯誤；
   `Restore` 到實模式快照後清除、到 `PE=1`、`VM=0` 的快照時重新設起（`m.pm` 為 nil 與非 nil 各一例）。注入替身時，`Enter` 成功的那一步回 nil、`Steps` 加 1，
   之後的 `Step` 呼叫替身的 `Step`；`Enter` 回錯誤時停擺。注入 `PMCore` 替身時，`Enter` 收到的 `Handoff`（暫存器含高半、旗標、段值、段快取屬性）正確；段索引映射雙向往返，CS 與 SS
   對調的負對照會失敗。`mov cr0` 不改 PE、`lmsw 0`、只動 TS 都不回哨兵。同一步分頁紀錄與哨兵並存的情形以包住 `SysBus` 的替身在 `SetCR` 內
   注入分頁紀錄，`Step` 回 `*PageFaultError`，再 `Step` 回 `ErrPMCoreNotAvailable`。
6. probe：`-pm-profile` 與 `-ext-mem-kb 0` 同給回錯誤；與 `-load-state` 的一致性規則各一例；`SetProtectedModeProfile(false)` 後 `Sys` 為 nil。
7. 突變自檢：`lmsw` 改成可清 PE、`sgdt` 改成遮 24 位元、段索引映射 CS 與 SS 對調、`mov crN` 改走 `decodeModRM`、`CR3` 讀回不遮罩、`66 smsw`
   寬度、哨兵邊緣判定改成「PE 為 1 就回」、移除 `IDTR` 守門（`doInt` 與 IRQ 兩處）、`Restore` 不重新判定、`Enter` 失敗不停擺、分頁紀錄加哨兵不停擺，
   各至少一個測試失敗。
8. 效能：profile 關閉時 `BenchmarkStepRealMode` 與 432 完成的版本（`82fe8b4`）交錯比較（`431-hybrid-cpu-protected-mode` §7），照實記錄。
9. 觸發案例（需原版，另附 probe 收據）：`SCCD.EXE -dos-files 30 -ext-mem-kb 8192 -no-ems -pm-profile` 通過 `smsw`、XMS 與 A20 檢查，進 JEMM；
   `sgdt` 存到 `3E12` 的值為 GDT（界限 0、基底 0）、`sidt` 存到 `3E18` 的值為 IDT（界限 `03FFh`、基底 0）；`lgdt`／`lidt` 以 16 位元形式載入
   （GDT 界限 `012Fh`、IDT 界限 `07FFh`）；在 img `0x4203` 交棒，停在 `ErrPMCoreNotAvailable`。若在此之前停在別的指令，照實記錄。
10. `go build ./...`、`go vet`、`tools/go.sh test ./internal/... ./cmd/probe ./session ./oracle` 全綠。
