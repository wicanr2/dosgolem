# 436 — TSS、`ltr` 與任務返回進入 V86

狀態：**READY**（第 3 版，依第 2 輪複審修正）
日期：2026-10-16
前置：[`431-hybrid-cpu-protected-mode`](431-hybrid-cpu-protected-mode.md)（第 2 版，§8 的 S4）、[`432-linear-memory-and-paging`](432-linear-memory-and-paging.md)、
[`433-real-mode-system-instructions`](433-real-mode-system-instructions.md)、[`434-legacy-core-segmented-mode`](434-legacy-core-segmented-mode.md)

編號說明：本規格是 431 §8 的 S4。觸發案例先碰到 S4，才碰到 `435-v86-hooks-and-idt-delivery`，所以先做本規格；435 改以本規格為前置。

DOSBox-X 引用一律指 `DOSBox-X-MCP-Debugger/dosbox-src/src/` 下的檔案。JEMM 位址為 img 位移；`X` 為 XMS 區塊線性位址（本案 `110000h`，434 §1.1）。

---

## 1. 為什麼要做

434 的觸發案例停在 JEMM img `0x4586` 的 `ltr cx`（`CX=F0h`）。之後 JEMM 進 V86：

```text
4586  0f 00 d9       ltr  cx            ; TR ← F0h
4589  66 9c          pushfd
458b  66 58          pop  eax
458d  80 cc 40       or   ah,40h        ; NT
4590  66 50          push eax
4592  66 9d          popfd              ; EFLAGS.NT=1
4594  66 cf          iretd              ; NT=1：任務返回到 TSS F0h 的 back link
4596  fc             cld                ; 以下在 V86（CS=2000h）
4597  0f b2 26 97 3e lss  sp,[3E97h]
459c  33 c0          xor  ax,ax
459e  fb             sti
459f  cb             retf               ; 回 SCCD 映像 0x1D1F
```

`4589`–`4594` 的 `66 9C`、`66 58`、`80 CC 40`、`66 50`、`66 9D`、`66 CF` 在 434 的路徑上沒有執行過，本規格的驗收要涵蓋。

第 104585 步（`ltr` 前一步）的兩個 TSS（已證實，probe 傾印；GDT 副本中 `F0h` 為 `89h`、基底 `X+1D30h`、界限 `67h`；`10h` 為 `8Bh`、基底 `X+1D98h`、界限 `2068h`）：

| 欄位（386 TSS 偏移） | TSS `F0h` | TSS `10h` |
|---|---|---|
| back link（`00h`） | `10h` | 0 |
| `SS0:ESP0`（`08h`、`04h`） | `50h:1400h` | `50h:1400h` |
| `CR3`（`1Ch`） | 0 | `0011C000h` |
| `EIP`（`20h`） | 0 | `4596h` |
| `EFLAGS`（`24h`） | `2` | `00023202h`（VM、IOPL=3、IF） |
| `EAX`…`EDI`（`28h`–`44h`，順序 AX、CX、DX、BX、SP、BP、SI、DI） | 0 | 0，`ESP=200h` |
| `ES`、`CS`、`SS`、`DS`、`FS`、`GS`（`48h`–`5Ch`，各 4 bytes） | `20h`、`20h`、`20h`、`20h`、0、0 | 全部 `2000h` |
| `LDT`（`60h`） | `08h` | `BBh` |
| I/O 點陣圖位移（`66h`） | `68h` | `68h`（點陣圖靜態全 0，`docs/re/004` §1） |

`iretd` 讀 TSS `F0h` 的 back link `10h`，切到 V86 任務：`CS:IP=2000:4596`、`EFLAGS=00023202h`、`SS:SP=2000:0200`。`2000:3E97`（線性 `23E97h`）、
`2000:0200`、`2000:4596` 所在頁的 PTE 為 `x007`（恆等、P、RW、US），已證實。

### 1.1 預期停點（強推論，未實測）

`retf` 回 SCCD 映像 `0x1D1F` 後，依 SCCD 反組譯約 18 道 V86 指令到映像 `0x2BE` 的 `INT 2Fh`（`AX=4300h`），這是第一個需要 435 的事件。
預設 `IRQ0Every=165000`、首次 `nextIRQ0=165000`（`machine.go` 約 194、722 行），第 104587 步時 IRQ0 還沒到期，所以預期不會先停在 IRQ0。若 PIT 被重新設定，`recalcIRQ0` 可能把 `nextIRQ0` 提前（`machine.go` 約 1353 行），預測失效。

## 2. 任務暫存器與介面

433 §2 已有 `TR` 的選擇子與快取（存於 `PM`）。本規格不改 `PM` 格式。

### 2.1 `SysBus` 增補

`internal/cpu.SysBus` 增加：`VM() bool`（`EFLAGS.VM`）、`TR() (sel uint16, c SegCache)`、`SetTR(sel uint16, c SegCache)`、`ReadSup8`、`ReadSup16`、
`WriteSup8`（Machine 已有，`internal/machine/linear.go`）。`cpu/sys_test.go` 的 `fakeSys`、`cpu/segmented_test.go` 的 `cplSys` 與 machine 測試中包住
`SysBus` 的替身跟著補。

核心判定 V86 一律用 `c.Sys != nil && c.Sys.VM()`；profile 關閉時只多一次 nil 比較。

描述元解析：434 的 `descriptor()`（`internal/cpu/segmented.go`）匯出為 `cpu.DecodeDescriptor(lo, hi uint32) SegCache`，核心的 `ltr` 與 Machine 的
任務返回共用，確保 `TR` 快取一致。`cpu.go` 的旗標常數加 `NT = 1 << 14`。

### 2.2 `ltr r/m16`（分段模式）

照 DOSBox-X `CPU_LTR`（`cpu/cpu.cpp:2452-2477`）與 `prefix_0f.h:42-47`；違反時失敗即關閉，`TR` 與 GDT 都不改：

1. CPL ≠ 0（DOSBox-X `#GP`）。
2. 空選擇子（`(sel & FFFCh)==0`）：失敗（DOSBox-X 設為無效 TR 且不報錯；刻意從嚴，JEMM 不會這樣用）。
3. `TI=1` 或超出 GDT 界限（真機 `#GP`）。
4. 描述元須為可用的 386 TSS（S=0、type 9）；286 TSS、忙碌 TSS、其他型別失敗（DOSBox-X 接受 286 可用 TSS；刻意從嚴）。
5. not present（真機 `#NP`）。
6. 先以 `WriteSup8` 把 GDT 中該描述元的 access byte `|= 02h`（忙碌），再 `SetTR`（選擇子、`DecodeDescriptor` 的快取；快取 `Attr` 含忙碌位元，例如 `8Bh`）。

434 §6 表中 `ltr` 一列改為本節行為；`internal/machine/seginsn_test.go` 的 `TestSegInsnLTRStops` 改寫為成功例與上列失敗例。

## 3. 任務返回

### 3.1 觸發

分段模式下 `CF`（`ops.go` 的 `iret`）或 `66 CF`，且 `EFLAGS.NT=1`：核心把 `IP` 設回本道指令起點（`opIP`），記下下一道的位址（`opIP`＋指令長度，
以 `TaskReturnNextIP() uint16` 讀取），回傳哨兵 `cpu.ErrTaskReturn`（`errors.Is` 可判）。其他狀態都不改。DOSBox-X 的任務返回不看運算元大小
（`cpu.cpp:1586-1597`），存入舊 TSS 的 `EIP` 是下一道（`prefix_66.h:567` 以 `GETIP` 呼叫，`core_normal.cpp:114`）。

`66 CF` 目前落在 `execute32` 的 default（`ops386.go` 的 `sizeSensitive` 含 `0xCF`）：`execute32` 加 `0xCF`，分段模式且 `NT=1` 回哨兵，否則維持
434 的失敗訊息（含 "IRET"，`segmode_test.go` 現有斷言不變）；V86 的 `iretd` 在這裡實作（§4）。

### 3.2 Machine 的處置（`finishStep` 收到 `ErrTaskReturn`）

步驟照 DOSBox-X `CPU_SwitchTask` 的 `TSwitch_IRET` 路徑（`cpu.cpp:905-1091`）。**第 1–3 步只讀不寫，任何失敗都在第 4 步之前，失敗時狀態與
`CPU.Step` 返回時相同**（`IP` 已在本道指令起點，A 位元除外，見第 3 步）：

1. 目前 `TR`：有效、`TI=0`、在 GDT 界限內、快取界限 ≥ `67h`（DOSBox-X 以 `CPU_CHECK_COND` 檢查有效，真機 `#TS`）。目前 TSS 不忙碌時 DOSBox-X
   只記 log，本規格照做，不失敗。
2. 讀 back link：`ReadSup16(TR.base)`。空選擇子失敗。
3. 新 TSS：`TI=0`、在 GDT 界限內、386 TSS、**忙碌**（type `B`）、present、界限 ≥ `67h`；否則失敗（DOSBox-X `E_Exit`）。讀新 TSS 的全部欄位（§1 的偏移）。
   接著檢查：
   - 新選擇子 ≠ 目前 `TR`（DOSBox-X 有同選擇子特例；本案不會發生，刻意從嚴）；
   - 新 `EFLAGS` 的 `VM=1`（保護模式任務不在範圍）；
   - 新 `EFLAGS` 的 bit 18–31 為 0（386 沒有 `AC`、`VIF`、`VIP`、`ID`；`SetEFlagsHi` 只接受 `RF`、`VM`）；
   - 新 `EIP` 高 16 位元為 0（DOSBox-X 不檢查；刻意從嚴）。
   - 分頁：第 2、3 步的讀取若留下分頁紀錄（`m.pfOK`），失敗。`m.paging` 為真時，第 4、5 步的寫入目標（舊描述元的 access byte；舊 TSS 的 `20h`–`5Fh`；IRET 路徑不寫新描述元）以
     `peekXlatPage`（`internal/machine/paging.go`，無副作用、只判 P）逐一檢查每個寫入範圍的首尾 byte，不存在就失敗；`m.paging` 為假時不探測（只有 A20 遮罩，沒有頁面失敗）。讀取時 `walk` 會照常設 A 位元與填 TLB，
     這是讀取本身的效果，不算「狀態改變」。
   失敗時：`m.pfOK` 為真才以 `m.stepFault(err)` 包成 `*PageFaultError`（沿用 `finishStep` 現行的 `if m.pfOK` 分流）；其餘回一般 `error`。哨兵在取指之後
   立刻返回，進入本處理時 `m.pfOK` 為假，第 2、3 步的讀取若設起它，就是這次讀取的紀錄。
4. 清目前 TSS 描述元的忙碌位元：讀 access byte、`&^= 02h`、`WriteSup8` 寫回（DOSBox-X 以快取描述元整份寫回，`cpu.cpp:960-963`；讀改寫較接近真機，刻意）。
5. 把目前狀態寫入目前 TSS（`WriteSup8`，32 位元欄位以 4 次寫入）：
   - `EFLAGS` ＝ `uint32(CPU.Flags) | uint32(eflagsHi)<<16`，清 `NT`；
   - `EIP` ＝ `TaskReturnNextIP()`；
   - 通用暫存器 ＝ `R[i] | Hi[i]<<16`（索引順序同 TSS）；
   - 段值 `Seg[ES]`、`Seg[CS]`、`Seg[SS]`、`Seg[DS]`、`FSeg`、`GSeg`，各寫 32 位元、高半 0（DOSBox-X `mem_writed`）。
   不寫 `CR3`、`LDT`、back link。
6. 新 `CR3` 與目前不同時 `SetCR(3, newCR3)`（432 清 TLB）；相同時不動（DOSBox-X 同）。
7. 載入新內容，依序：`SetSegmented(false)`、清六個段快取、`SetEFlagsHi(VM)`（`RF` 清 0：DOSBox-X 以 `FMASK_ALL|FLAG_VM` 設旗標，保留舊的 `RF`，
   `cpu.cpp:1028`；本案舊 `RF` 恆為 0，結果相同；真機會載入新 TSS 的 `RF`）、核心 `SetFlags`（低 16 位元，含 `IOPL`、`NT`、`IF`）、`IP`、通用暫存器（低半進 `R`，高半進 `Hi`）、
   `Seg[ES/CS/SS/DS]`、`FSeg`、`GSeg`（直接寫，V86 以段值×16 定址）。`pmCPL` 不改（V86 的 CPL 由 432 從 `VM` 推得為 3）。
8. `LDTR` 不載入（照 DOSBox-X，`cpu.cpp:1043-1050` 只在非 V86 任務呼叫 `CPU_LLDT`；386 真機會載入 `BBh`。本案 GDT 副本中 `08h` 與 `B8h` 兩個 LDT
   描述元基底都是 `X+1400h`、界限 7，只差 DPL（強推論，依 `62B5` 之前的基底改寫碼；觸發案例收據以 `iretd` 前一步的 GDT 傾印證實），所以不載入只影響
   `sldt` 的回傳值。刻意對齊 oracle）。
9. `SetTR(新選擇子, DecodeDescriptor(新描述元))`（仍為忙碌）。434 的一百萬步計數歸零。
10. 這一步回 nil。

434 §3.2 的守門（分段模式下拒絕設 `VM`）由第 7 步先 `SetSegmented(false)` 再設 `VM` 滿足（`linear.go` 的 `SetEFlagsHi` 守門）。

## 4. V86 執行（本規格範圍內）

進入 V86 後由既有核心以 432 的 V86 語意執行（段值×16、經線性層、CPL 3 的使用者分頁權限）。本規格補下列規則，其餘屬 435。「失敗即關閉」的
診斷都註明 435（真機為 `#GP` 等，由 435 遞送）。

| 位置 | 指令 | V86 規則 | DOSBox-X |
|---|---|---|---|
| `ops.go` 的 `FA`、`FB` | `cli`、`sti` | IOPL=3 照常；否則失敗 | `CPU_CLI`／`CPU_STI`，`cpu.cpp:756-773` |
| `ops.go` 的 `9C`；`ops386.go` 的 `66 9C` | `pushf`、`pushfd` | IOPL=3 照常（核心 `Flags` 只有 16 位元，`pushfd` 推入的高半已為 0，即清 `VM`、`RF`）；否則失敗 | `CPU_PUSHF`，`cpu.cpp:790-800` |
| `ops.go` 的 `9D`；`ops386.go` 的 `66 9D` | `popf`、`popfd` | IOPL=3 照常，新值為彈出值的 IOPL 位元（bits 12–13）換成舊值（不改 IOPL）；`popfd` 的高半丟棄（不改 `VM`、`RF`）；否則失敗 | `CPU_POPF`，`cpu.cpp:774-788` |
| `ops.go` 的 `CF`；`execute32` 的 `66 CF` | `iret`、`iretd` | IOPL=3 時從 V86 堆疊彈出 `IP`、`CS`、旗標（`iretd` 各 4 bytes，`EIP` 高半非 0 失敗，刻意從嚴），旗標新值為彈出值的 IOPL 位元換成舊值（DOSBox-X `FMASK_NORMAL` 加 `NT`）；否則失敗 | `cpu.cpp:1542-1584` |
| `ops.go` 的 `E4`–`E7`、`EC`–`EF`；`cycles.go` 的 `in8`／`out8`／`ioAllowed`；`ops186.go` 的 `insOuts` | `in`、`out`、`ins`、`outs` | 檢查上移到指令層（寬度 1 或 2、起始埠），V86 路徑不在 `in8`／`out8` 內重複檢查。`TR` 無效或快取不是 386 TSS 時失敗（DOSBox-X `cpu.cpp:1096`）。V86 一律查 `TR` 的 I/O 點陣圖（不論 IOPL）：讀 `TR.base+66h` 的位移 `ofs`，`ofs > TR.limit` 失敗（照 DOSBox-X 只比位移，不檢查點陣圖位元組本身是否超出界限）；讀 `TR.base+ofs+port/8` 起的 16 位元，與遮罩 `((1<<size)-1) << (port&7)` 相交非 0 則失敗。整筆存取（含 word、字串的每次迭代）在動作前一次檢查完；失敗時不呼叫 `In8`／`Out8`；`rep` 且 `CX=0` 時不檢查。`66` 的 dword I/O 形式目前未實作，維持失敗 | `CPU_IO_Exception`，`cpu.cpp:1093-1111` |
| `ops.go` 的 `hlt` | `hlt` | 失敗（條件改為 `segmented` 或 V86） | |
| `sys.go` 的 `ivtGuard` | `INT n`、`INT3`、`INTO`、`BOUND` 越界、除以零 | 加 V86 分支，失敗；`doInt` 的檢查仍在 `IntHook` 之前 | |
| `cpu.go` 的 `Interrupt`、`FarCallWithReturn` | 第二道防線 | 加 V86 條件 | |
| 433 的特權指令 | `mov crN`、`lgdt` 等 | 433 已依 `Sys.User()` 失敗 | |

`sti` 與 `mov ss` 之後的中斷遮蔽不模擬（與現行實模式一致；DOSBox-X `prefix_none.h:1458-1495` 會多執行一道才處理 IRQ。刻意差異，435 遞送 IRQ 時重審）。
IRQ 的停點因此在「下一道指令取指之前」。

Machine 側（衍生欄位 `m.v86 = m.xlat && eflagsHi&VM`，在 `recalcPM` 算出，與 `cplUser` 同法，`Step` 不逐步重算）：

- IRQ：把 433 的 `ivtGuardIRQ`（`internal/machine/sysregs.go`）擴充為「`IVTMismatch()` 或 `m.v86`」，診斷註明 V86 與 435。三個遞送點（`tick` 的 IRQ0、
  `keyTick`、`timedKeyTick`）都已把它放在 IF、PIC 遮罩、待處理、`KeyEvery`／`nextKey`、定時鍵到期與 `StubSeg` 判斷之後、消耗狀態之前，所以只在原本會遞送的
  那一步失敗，不重列條件。`tick` 中 `startCallback`、`periodic` 的閘門由 `!segmented` 改為 `!segmented && !m.v86`。
- `startCallback`、`periodic` 的 `FarCall`：延後，不執行、不消耗（同 434 §5.3）。
- `VGAFrameEvery` 的幀計數與 IRQ0 時鐘照常。

## 5. 不在範圍

V86 事件經 IDT 遞送、`#GP` 反射、從保護模式 `iretd` 回 V86（435）；保護模式任務、`jmp`／`call` 到 TSS 或任務閘、286 TSS；`#TS`／`#NP` 的遞送；
V86 下的 x87；中斷遮蔽；回實模式（S9）。

## 6. 驗收

1. profile 關閉時：錨點測試常數與 gob 位元組不變；`internal/cpu386` 無改動；SingleStepTests 315 PASS、0 SKIP，`knownGaps` 不變。
2. `ltr`：§2.2 每條各一例（CPL≠0 以 `cplSys` 類替身測、空選擇子、`TI=1`、超出界限、資料段、286 TSS、忙碌 TSS、not present 失敗；可用 386 TSS 成功），成功後 GDT 描述元變忙碌、
   `TR` 快取正確；失敗時 `TR` 與 GDT 不變。
3. 任務返回：合成 GDT 與兩個 TSS（仿 §1 的表）：
   - 執行後 `VM=1`、`segmented=false`、`CS:IP`、`SS:SP`、`EFLAGS`（`RF` 為 0）、通用暫存器（含高半）、六個段值與新 TSS 相同；`TR` 為新選擇子；`LDTR` 不變；
   - 舊 TSS 描述元不再忙碌，新 TSS 仍忙碌；
   - 舊 TSS 的 `EFLAGS`（無 `NT`）、`EIP`（下一道）、通用暫存器與段值（32 位元寫入）正確，`CR3`、`LDT`、back link 未被寫；
   - `CR3` 相同時不清 TLB、不同時清；
   - 目前 TSS 不忙碌時照常切換；
   - §3.2 第 1–3 步每個失敗條件各一例（`TR` 無效、`TR` 的 `TI=1`、`TR` 界限 < `67h`、back link 為空、指向非忙碌 TSS、`TI=1`、超出界限、
     新 TSS 界限 < `67h`、同選擇子、`VM=0`、`EFLAGS` 高位元、新 `EIP` 高半非 0、讀取遇到不存在頁、寫入目標頁不存在），失敗時狀態與 `CPU.Step` 返回時相同
     （`IP` 在本道指令起點）；
   - `16` 位元 `CF` 帶 `NT=1` 同樣任務返回；`NT=0` 的 `iret`／`66 iret` 在分段模式仍失敗（訊息含 "IRET"）。
4. V86：§4 表每列至少一例（IOPL=3 與 IOPL<3 各一）；`popf`／`iret` 不改 IOPL；`pushfd` 高半為 0；I/O 點陣圖：位元為 0 照常、為 1 失敗（IOPL=3 時也失敗）、
   位移超過界限失敗、word 存取第二個埠被擋時第一個埠也不存取；IRQ0 在遞送條件成立時失敗、`IF=0` 或 `picMask` 遮蔽時延後；回呼延後不消耗。
   `BenchmarkStepV86`、`BenchmarkStepV86Paged` 與 432 的 V86 測試維持 `IRQ0Every=0`，不受閘門影響。
5. `PM` 往返：任務返回後 `SaveState`／`LoadState` 與 `Snapshot`／`Restore` 還原 `VM=1`、`TR`、`segmented=false`，`Step` 照 V86 繼續。
6. 效能：profile 關閉的 `BenchmarkStepRealMode` 與本規格動工前的 commit 交錯比較（431 §7），照實記錄；主機不安靜時與 432–434 同列開放。
7. 突變自檢：`ltr` 不設忙碌、任務返回不清舊 TSS 忙碌、舊 TSS 的 `EFLAGS` 不清 `NT`、存入的 `EIP` 用指令起點、第 3 步的檢查移到第 4 步之後、`popf` 在 V86 改 IOPL、
   略過 I/O 點陣圖、V86 的 `INT` 在 `IntHook` 之後才檢查、IRQ 閘門不看 `picMask`，各至少一個測試失敗。
8. 觸發案例（需原版，另附 probe 收據）：`SCCD.EXE -dos-files 30 -ext-mem-kb 8192 -no-ems -pm-profile`：
   - 在 `ltr` 之後、`iretd` 之前的步數傾印：`TR=F0h`，GDT 副本中 `F0h` 的 access byte 為 `8Bh`；
   - `iretd` 之後：`VM=1`、`CS:IP=2000:4596`、`SS:SP=2000:0200`、`EFLAGS=00023202h`、`TR=10h`、`LDTR=08h`；GDT 中 `F0h` 回到 `89h`、`10h` 仍為 `8Bh`；
     TSS `F0h` 的 `EIP=4596h`、`EFLAGS` 無 `NT`、`CS=18h`，其餘段值與通用暫存器等於 434 收據第 104585 步存檔中的值；
   - 停點：照實記錄停點、原因、步數、`CS:IP`，以及 `irq0Pending`、`picMask`、`nextIRQ0`、`IF`。§1.1 預測停在 SCCD 映像 `0x2BE` 的 `INT 2Fh`。
9. `go build ./...`、`go vet ./internal/... ./cmd/probe`、`go test ./internal/... ./cmd/probe ./session ./oracle` 全綠。
