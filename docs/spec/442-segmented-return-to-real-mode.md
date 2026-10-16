# 442：分段模式回實模式（殘留快取實模式）

狀態：**READY**（第 2 版，依審查修訂）
日期：2026-10-16
前置：[`431-hybrid-cpu-protected-mode`](431-hybrid-cpu-protected-mode.md)（第 2 版）、[`432-linear-memory-and-paging`](432-linear-memory-and-paging.md)、
[`433-real-mode-system-instructions`](433-real-mode-system-instructions.md)、[`434-legacy-core-segmented-mode`](434-legacy-core-segmented-mode.md)、
[`435-v86-hooks-and-idt-delivery`](435-v86-hooks-and-idt-delivery.md)、[`436-tss-task-return-v86-entry`](436-tss-task-return-v86-entry.md)、
[`437-segmented-32bit-far-transfers`](437-segmented-32bit-far-transfers.md)

DOSBox-X 引用一律指 `DOSBox-X-MCP-Debugger/dosbox-src/src/` 下的檔案。JEMM 位址為 img 位移（`strike_commander` `docs/re/003` §1）；
反組譯在 `strike_commander` 的 `workplace/re/jemm-api/img.dis`（本機）。`X` 為 JEMM 的 XMS 區塊線性位址（`[3EA9]`，434 收據實測 `110000h`）。

本規格同時修訂 431 §5 交接表與 §8、432 §2 與 §5.4、433 §2 與 §8 第 3 項、434 §2、§2.2、§3.2、§5.1、§8、§9 第 4 項；各處已加註「由 442 修訂」。

---

## 1. 為什麼要做

431 §5 原本把「保護模式→實模式」列為不支援（S9），`SetCR0`（`internal/machine/linear.go:117-119`）在分段模式下拒絕清 PE。
《Strike Commander》的 SCCD 在 JEMM 卸載時走這條路，觸發案例停在 `0070:519E`：
`strike_commander` 的 `workplace/logs/probe451a.log` 第 6 行「分段模式下不能把 CR0.PE 清成 0（保護模式→實模式不支援，S9；docs/spec/434 §3.2）」，
停點 `DS=ES=SS=0078h`、`SP=13BCh`、`AX=FFF0h`、`IF=0`。

### 1.1 JEMM 的回實模式序列

| img | 指令 | 說明 | 等級 |
|---|---|---|---|
| `583C`–`584A` | `add sp,2`；`mov ax,80h`；`mov ds,ax`；`xor eax,eax`；`mov dr7,eax`；`jmp 5180` | 經 `int 15h` 分派（`5822 jmp [di]`）進入；哪個子功能未知 | 已證實（靜態） |
| `5180` | `jmp 70:5187` | 遠跳到碼段 `70h` | 已證實（靜態） |
| `5187`–`5192` | DS、ES、FS、GS、SS 都載入 `78h` | SP 不變 | 已證實（靜態） |
| `5194`–`519E` | `cli`；`mov eax,cr0`；`and eax,7FFFFFFEh`；`mov cr0,eax` | 同時清 PE 與 PG。`mov eax,cr0` 讀回值帶 `|7FFFFFF0h`（`internal/cpu/sys.go:253`），MP、EM、TS 保留原值 | 已證實（靜態、probe） |
| `51A1` | `jmp short 51A3` | 不重載 CS | 已證實（靜態） |
| `51A3` | `lidtw ds:[3E18h]` | 還原 `41ED sidtw ds:[3E18h]` 在進保護模式前存的 IDTR；16 位元形式只取 24 位元基底（`sys.go:175-177`） | 已證實（靜態） |
| `51A8`–`51BE` | `mov bx,sp`，以 `[bx+34h]`、`[bx+3Ch]`、`[bx+40h]`、`[bx+30h]` 載入 ES、FS、GS、SS，`[bx+2Ch]` 載入 SP | 讀取經 DS（仍是 `78h` 的快取） | 已證實（靜態） |
| `51C1`–`51CE` | 推 `[bx+28h]`、`[bx+24h]`、`[bx+20h]`、dword `[bx+10h]`、`[bx+38h]` | 在新的實模式 SS:SP 疊 iret 框架與 EBX、DS | 已證實（靜態） |
| `51D1`–`51E7` | `EAX=0`，ECX、EDX、EBP、ESI、EDI 由 `[bx+18h]`、`[bx+14h]`、`[bx+8]`、`[bx+4]`、`[bx]` 載入 | | 已證實（靜態） |
| `51EA`–`51ED` | `pop ds`；`pop ebx`；`iret` | 16 位元 `iret` 在實模式重載 CS | 已證實（靜態） |

`[bx+0]`–`[bx+1Fh]` 是 `pushad` 區，`[bx+20h]` 起是 CPU 推入的 V86 框架（EIP、CS、EFLAGS、ESP、SS、ES、DS、FS、GS），與
`docs/re/003` §2 的分派框架相同，所以 JEMM 是回到發出 `int 15h` 的 V86 呼叫者，改以實模式繼續（強推論）。

### 1.2 段基底在 1 MB 以上

GDT 範本（`gdt.py` 解析 `jemm.img` 偏移 `1400h`）：`70h` 碼段 access `9Ah`、界限 `FFFFh`、位元組粒度、16 位元；`78h` 資料段 access `92h`、
界限 `FFFFh`、位元組粒度、16 位元（已證實，靜態）。基底在執行時填入：`4059`／`4067` 以 `EBX` 寫第一份 GDT，`42FF`／`430E` 以 `[3EA9]`
寫 XMS 副本（`img.dis` 第 1112–1118、1289–1295 行）。434 收據的 GDTR 是副本（`X+1400h`），所以卸載時 `70h`、`78h` 的基底是 `X`＝`110000h`
（強推論；靜態上沒有其他寫 `[1472h]`、`[147Ah]` 的位置，執行期值由觸發案例確認）。

清 PE 之後到 `51ED iret` 之前，CPU 在實模式下以各段暫存器的描述元快取（基底 `X`、界限 `FFFFh`）執行；`mov es/fs/gs/ss`、`pop ds`、`iret`
以實模式語意重載。清 PG 之後線性即實體，`X+51A1h` 能取到下一道指令，表示 JEMM 對自身映像做恆等映射（強推論，觸發案例記錄 PTE 確認）。

### 1.3 dosgolem 現況的缺口（已證實，讀碼）

| 位置 | 現況 | 問題 |
|---|---|---|
| `linear.go:32-42` `recalcPM` | 「轉換開啟」`xlat ＝ pmProfile && PE` | PE 一清 `xlat=false`，`Read8`／`Write8`（`machine.go:873-903`）在 HMA 之外遮成 20 位元。`segFetch8` 送出的 `X+51A1h＝1151A1h` 會讀成 `151A1h` |
| `linear.go:117-119` `SetCR0` | 分段模式下新值 PE=0 一律回錯誤 | S9 本身；條件也擋下子狀態內「PE 維持 0」的寫入 |
| `ops.go:383-403` `iret` | 分段模式先看 NT（任務返回）否則失敗；實模式分支直接寫 `c.Seg[CS]` | 子狀態要走實模式 iret，且 CS 要經快取載入。DOSBox-X `CPU_IRET` 先判 `!cpu.pmode`（`cpu/cpu.cpp:1528-1540`），實模式不看 NT |
| `ops386.go:352-364` `66 CF` | 分段模式 CPL 0 時當成回 V86（435 §2.5） | 子狀態下會誤走 `v86ReturnRead` |
| `ops.go:920-927` `farRet` | 分段模式檢查彈出 CS 的 RPL＝CPL | 實模式沒有這條檢查 |
| `segmented.go:147-153` `loadSeg` | 分段模式一律查描述元表 | 子狀態要用實模式語意 |
| `sys.go:97-109` `execSys0F` | 分段模式的 `0F 00` 群組走 `seg0F00`（`lldt`、`ltr` 等） | 實模式下這組是 `#UD` |
| `pmstate.go:293-296` `decodePMV3` | `segmented=1` 要求 PE=1 | 子狀態存不了檔 |

## 2. 規則

### 2.1 名詞與狀態

- **殘留快取實模式**（下稱子狀態）：`segmented=true` 且 `CR0.PE=0`。只能由 §2.2 進入，由 §2.7 離開。
- 不新增持久狀態。子狀態由既有欄位推得：CPU 的 `segmented` 與 Machine 的 `CR0`。
- CPU 側以 `c.realCache()`（`c.segmented && c.Sys != nil && CR0.PE==0`，經 `c.Sys.CR(0)` 讀取）判斷。只在 `segmented=true` 的分支內呼叫，
  實模式熱路徑不經過。理由：單一來源，不必在 `Restore`、`LoadState`、profile 切換時同步第二個欄位；段載入不在 431 §7 的效能門檻量測範圍內。
  cpu 套件既有替身因此要改（§2.10）。
- 434 §2 的不變式（已由 442 修訂）：`segmented=true` ⇒ profile 開啟、`VM=0`、`pmCPL=0`，且（`PE=1`）或（`PE=0`、`PG=0`）⇒ `xlat=true`
  ⇒ `CPU.Code==nil`。

### 2.2 進入：分段模式下 `mov cr0` 清 PE

`Machine.SetCR0`（`linear.go:109-126`）的模式檢查只看「PE 由 1 變 0」與「子狀態內設 PE 或 PG」；PE 維持 0 的寫入（子狀態內的 `clts`、
只改 MP、EM、TS 的 `mov cr0`）不受 117 行原條件影響，該條件刪除。規則：

| 條件 | 結果 |
|---|---|
| `VM=1` 而 PE 由 1 變 0 | 回錯誤，`CR0` 不改（現況 114–116 行，保留） |
| 分段模式、`PE=1`，新值 `PE=0` 且 `PG=0` | 接受：寫入 `CR0`，PE、PG 改變時作廢 TLB（現況 120–122 行），`recalcPM`。進入子狀態 |
| 分段模式、`PE=1`，新值 `PE=0` 而 `PG=1` | 回錯誤，`CR0` 不改。Intel 為 `#GP(0)`；DOSBox-X 只記 log「Paging requested without PE=1」並關分頁（`cpu/cpu.cpp:2629-2632`），刻意從嚴 |
| 子狀態，新值 `PE=1` | 回錯誤，`CR0` 不改：由子狀態再進保護模式不在範圍（§2.11）。`enterSegmented` 會以段值重設快取，與真機保留快取的行為不同 |
| 子狀態，新值 `PE=0` 而 `PG=1` | 回錯誤，`CR0` 不改 |
| 子狀態，新值 `PE=0`、`PG=0`（只改 MP、EM、TS 或不改） | 接受 |

- 只有 `mov cr0` 能進入子狀態：`lmsw` 在舊值 PE＝1 時強制保留 PE（`sys.go:205-208`）。進入時 CPU 側不回哨兵，下一道指令仍以分段模式的
  位址計算取指令（CS 快取）。`lmsw` 在子狀態設 PE 時經 `SetCR(0)` 被上表擋下。
- CPL、GDTR、LDTR、TR 不動（DOSBox-X `CPU_SET_CRX` 清 PE 只設 `cpu.pmode=false` 並關分頁，`cpu/cpu.cpp:2578-2634`）。`pmCPL` 依 §2.1
  的不變式已是 0；實模式下 `cplUser` 恆為 false（`recalcPM`）。
- IDTR 不動。子狀態期間 IDTR 通常仍是保護模式 IDT（JEMM 為 `X+1530h`、`7FFh`），由程式自己 `lidt` 還原。
- 進入的那道指令完成後標記一次 §2.7 的離開檢查（快取恰好全是實模式推得值時，同一步就離開）。

### 2.3 轉換與位址計算

- 432 §2 的「轉換開啟」（已由 442 修訂）為 `xlat ＝ pmProfile && (CR0.PE=1 || segmented)`。`paging` 仍為 `xlat && CR0.PG`，子狀態下為 false；
  `cplUser`、`v86` 為 false。效果：子狀態的存取經 `readLin`／`writeLin`（`linear.go:202-226`），只套 A20 遮罩，`a ≥ MemSize` 落到 `Ext`，
  不遮 20 位元。`syncCodeFastPath`（`machine.go:847-858`）因 `xlat` 為真而保持 `Code==nil`，A20 關閉時也一樣，不另加條件。
- 任何改變 `segmented` 的路徑之後都要呼叫 `recalcPM`：`applyPM`、`SetProtectedModeProfile`、`enterSegmented`（經 `SetPMCPL`）現況已符合，
  §2.7 的離開路徑新增。
- 位址計算、界限與型別檢查照 434 §4 不變（`segRead`、`segWrite`、`segFetch8`、`stackCheck`）：線性位址＝快取基底＋偏移，取指令
  ＝ `cache[CS].Base + IP`，每個 byte 檢查界限，寫入唯讀段等型別違反失敗即關閉。DOSBox-X 不檢查型別、預設也不檢查界限
  （`cpu/cpu.cpp:78` 的 `do_seg_limits=false`）。
- **已知不一致**：子狀態沿用從嚴的型別檢查，但 §2.7 的離開條件不比較 RW 與可執行位元，離開後的實模式核心也不檢查型別。所以同一段指令流
  （例如經 CS 前綴寫入，或寫入唯讀資料段）在子狀態內失敗即關閉，在離開後照常執行。JEMM 路徑在子狀態內只經 `78h`（可寫資料段）讀寫、
  經 `70h` 取指令，不觸發。
- `67` 定址照 434 §4.2（分段模式不截斷，界限由快取檢查）；界限大於 `FFFFh` 的快取因此可在子狀態以 32 位元偏移存取（unreal mode）。
- A20 關閉時，子狀態的線性位址照樣遮 bit 20（`readLin` 先套 A20），與真機相同。

### 2.4 子狀態的段載入：`loadSeg` 的實模式分支

`loadSeg`（`segmented.go:147-153`）在 `segmented=true` 時先判 `c.realCache()`，成立就走實模式語意，不呼叫 `loadSegPM`：

1. `c.fault != nil` 時回 false，段值與快取不寫，不標記離開檢查。理由：`pop ds`、`mov ds,[m]` 等在 `pop()`、`get16()` 失敗時回 0 後仍呼叫
   `loadSeg`（`ops.go:129`、`142`、`146`、`263`），不能把 0 當成載入值。
2. 寫入段值（`setSegVal`）。
3. 目標快取不可用（`Attr` 的 P 為 0，例如分段模式下載入過空選擇子）時，改設實模式預設快取：`Base ＝ 值×16`、`Limit ＝ FFFFh`，
   CS 屬性 `009Bh`、其餘 `0093h`（與 `enterSegmented` 相同，`internal/machine/handoff.go:68-83`）。依據：DOSBox-X 沒有 P 的概念，實模式載入後照常存取；
   真機行為待查（§4.1）。到此結束，跳過第 4、5 步。
4. 否則 `Base ＝ 值×16`，`Limit` 不變（DOSBox-X `CPU_SetSegGeneral` 實模式分支只改 `val`、`phys`，註解「real mode: loads do not change the limit」，
   `cpu/cpu.cpp:2976-2991`；`SegSet16` 同，`include/regs.h:115-119`）。
5. `Attr` 不變，但 CS 與 SS 清 D/B（bit 14）。依據：DOSBox-X 實模式載入 SS 時 `cpu.stack.big=false`（`cpu/cpu.cpp:2980-2985`），實模式的
   `iret`、遠跳、遠呼叫、遠返回載入 CS 後 `cpu.code.big=false`（`cpu_allow_big16` 預設 false，`cpu/cpu.cpp:202`；`1538`、`1793`、`1942`、`2235` 行）。
6. 不做空選擇子、型別、DPL、RPL、present 的描述元檢查；不讀描述元表；不寫 accessed 位元；CS 的值不併入 CPL。
7. 寫入完成後設離開檢查旗標（§2.7），回 true。

`segmented=false` 時 `loadSeg` 不變（只寫段值）；分段模式（`PE=1`）時照 434 §5.1。

### 2.5 子狀態下的指令

| 指令 | 子狀態行為 | 依據 |
|---|---|---|
| 434 §5.1 表內所有經 `loadSeg` 的位置（`pop sreg`、`mov sreg`、`les`／`lds`／`lss`／`lfs`／`lgs`、16 位元 `EA`、`9A`、`FF /3`、`FF /5`、`CA`、`CB`） | §2.4 的實模式語意。遠呼叫、遠返回的推入與彈出經 SS 快取 | DOSBox-X `CPU_JMP`（`1785-1795`）、`CPU_CALL`（`1920-1945`）、`CPU_RET`（`2210-2236`）的實模式分支 |
| `CA`、`CB`（`farRet`） | 不做 `retRPLOK`（`ops.go:923` 的條件改為「分段模式且非子狀態」） | 實模式沒有 RPL |
| `CF`（16 位元 `iret`） | 實模式 iret，**不看 NT**：先 `stackCheck`，以 `read16` 讀 `SP`、`SP+2`、`SP+4`（各自 16 位元環繞）而不動 `SP`，看 `c.fault`；再 `loadSeg(CS)`（§2.4）；成功後設 `IP`、`SetFlags`（386 遮罩，同現況 `ops.go:402`）、`SP += 6`。任一步失敗時 `Seg[CS]`、快取、`IP`（照既有失敗路徑回報 `opCS:opIP`）、`SP`、旗標都不改 | DOSBox-X `CPU_IRET` 實模式分支（`cpu/cpu.cpp:1528-1540`） |
| `66 CF` | 失敗即關閉（不得走 `taskReturn` 或 `v86ReturnRead`） | 不在 JEMM 路徑 |
| 437 的 `66 EA`、`66 9A`、`66 CA`、`66 CB`、`66 FF /3`、`/5` | 失敗即關閉 | 不在 JEMM 路徑；實模式的 `66` 遠轉移在 dosgolem 本來就失敗即關閉 |
| `pop cs`、`mov cs` | 失敗即關閉（現況 `ops.go:135`、`257`） | |
| `0F 00` 群組、`LAR`、`LSL` | 失敗即關閉，走實模式的訊息（`sys.go:107-109`），不進 `seg0F00` | 真機實模式 `#UD`；DOSBox-X 送 IVT 第 6 項（433 §4） |
| `0F 01` 群組（`lgdt`、`lidt`、`sgdt`、`sidt`、`smsw`、`lmsw`）、`mov CRn`／`DRn`、`clts` | 照 433 的處理函式；`mov cr0`、`lmsw`、`clts` 受 §2.2 約束（`clts` 只改 TS，接受） | |
| `hlt` | 失敗即關閉（現況 `ops.go:539`） | |
| `in`／`out`／字串 I/O | 照分段模式現況（`cycles.go:72`、`80`，`ops186.go:95`）；CPL 0 所以一律放行 | |
| x87 ESC | 照分段模式現況（`ops.go:447`）：`FNINIT`、`FNSTCW`、`FLDCW`、`FWAIT` 以外失敗即關閉 | 從嚴，JEMM 路徑沒有 x87 |
| `pushf`、`popf` 與 `66` 形式 | 不改（CPL 0，與實模式相同） | |

### 2.6 中斷、例外、IRQ 與回呼

- `INT n`、`INT3`、`INTO`、除以零、`BOUND`：照 434 §5.3 經 `ivtGuard`（`sys.go:70-75`）的分段分支失敗即關閉，診斷改為註明「殘留快取實模式，
  docs/spec/442 §2.6」。`IntHook` 不會被呼叫（守門在它之前，`ops.go:954-960`）。
- `CPU.Interrupt`、`FarCallWithReturn` 的第二道防線（`cpu.go:412-416`、`481-484`）照舊。
- Machine 側：`tick` 的回呼閘門（`machine.go:1560`）、`keyTick`／`timedKeyTick` 的延後（`machine.go:1453`）、IRQ0 只累積不遞送、`hlt` 與
  「`IF=1` 且有待處理 IRQ 連續一百萬步」的失敗條件，都以 `CPU.Segmented()` 判斷，子狀態下自動生效，不改程式。
- JEMM 路徑在 `5194 cli` 之後才清 PE，`51ED iret` 同一步離開子狀態，期間 `IF=0`、沒有 `int`，所以不觸發上列任何一條（強推論，觸發案例記錄）。
  延後的 IRQ0 與按鍵在離開後的下一步經實模式 IVT 遞送，受 §2.7 的 IDTR 守門約束。

### 2.7 離開

**旗標**：CPU 的未匯出欄位 `leavePending bool`，不持久化。

- 只在 `segmented=true` 的分支設定：§2.4 第 7 步，與 §2.2 進入子狀態的那道 `mov cr0`（`setCR0` 在 `SetCR(0)` 成功、讀回 PE 由 1 變 0 之後）。
- 消費時清除：`Step` 在指令結束、`c.fault==nil` 且 `execute` 回 nil 時看到旗標，清除後做下面的條件檢查。
- 其他清除點：指令失敗（`c.fault != nil`，在 `Step` 的失敗分支清除，不回哨兵）、`SetSegmented`、`SetSegCache`（`Restore`、`LoadState`、
  `SetProtectedModeProfile`、`enterSegmented` 都經這兩個方法，因此一併涵蓋）。
- 不在每道指令開頭清除，實模式熱路徑只多一次布林判斷（`BenchmarkStepRealMode`，431 §7）。

**條件**（六個段暫存器都成立，段值以 `segVal(i)` 取，含 FS、GS）：

1. `Base ＝ 段值×16`；
2. `Limit ＝ 0000FFFFh`；
3. `Attr` 的 P＝1、S＝1；
4. ES、SS、DS、FS、GS 為資料段時須向上展開（EC＝0）；
5. CS 與 SS 的 D/B＝0。

不比較：RW、可執行、DPL、accessed、AVL、G，與資料段的 B（§2.3 的已知不一致）。理由：離開後既有實模式核心以「段值×16＋16 位元偏移」定址、
不檢查界限與型別。條件 1、2、4 保證每個 16 位元偏移在子狀態與實模式算出同一個線性位址且都在界限內；`67` 定址超過 `FFFFh` 時兩邊都失敗即關閉
（子狀態超出界限、實模式 `modrm.go:165-167` 的 64 KB 檢查）。

條件不成立時清除旗標、繼續在子狀態執行。

**動作**：條件成立時 `Step` 回新哨兵 `cpu.ErrLeaveSegmented`（指令已完成，`IP` 指向下一道，與 `ErrTaskReturn` 不同）。`finishStep`
（`internal/machine/handoff.go:23-61`）的 `ErrLeaveSegmented` 分支放在通用的 `if m.pfOK { stepFault }` 之前，與 `ErrModeSwitch` 相同：先離開，再看分頁紀錄。
`m.leaveSegmented()`：

1. `CPU.SetSegmented(false)`；六個快取清為 0（434 §2.2：`segmented=0` 時快取全 0）；
2. `resetSegWait()`；
3. `recalcPM()`：`xlat=false`，`syncCodeFastPath` 重判 `Code`；
4. 該步有分頁紀錄時回 `*PageFaultError`（子狀態 PG=0，正常不會有），否則回 nil。

`IP`、旗標、通用暫存器、段值、`CR0`、`CR3`、GDTR、LDTR、TR、IDTR、`pmCPL`（0）都不動。TLB 已在清 PE、PG 時作廢。

**IDTR**：離開時不檢查。離開點是「最後一個段暫存器重載完成的那一步」，不一定是程式邏輯上回實模式完成的時點：JEMM 先 `lidt` 再 `iret`，
但「清 PE、遠跳重載 CS、`mov ds/es/ss`、`lidt`、`sti`」這種寫法會在 `mov ss` 那一步離開，當時 IDTR 仍是保護模式 IDT，之後才換回來。
所以檢查放在中斷發生的當下：433 §2 的 `IVTMismatch`（已由 442 修訂）改為依向量判斷，`IVTMismatch(vec uint8) bool` ＝
`pmProfile && PE=0 && (IDTR 基底 ≠ 0 || 4×vec+3 > IDTR 界限)`。CPU 側 `ivtGuard(n, …)` 與 Machine 側 `ivtGuardIRQ(vec)` 傳入各自的向量，
條件成立時照 433 失敗即關閉，診斷寫出基底、界限與向量。
理由：既有實模式核心一律讀線性 0 的 IVT、不查界限（`cpu.go:427-430`）；真機以 IDTR 基底取向量並查界限，界限大於 `3FFh` 無害；
DOSBox-X 以 IDTR 基底取向量、不查界限（`cpu/cpu.cpp:1293-1296`），這裡刻意從嚴。JEMM 以 `41ED` 存的值還原，期望為 `sysRegsInit` 的
（0, 3FFh）（`pmstate.go:97-98`，強推論）。

JEMM 路徑：`51EA pop ds` 之後 CS 基底仍是 `X`，條件不成立；`51ED iret` 載入 CS 後六個快取都成立，同一步離開（強推論，由合成程式與觸發案例驗）。
條件始終不成立時（例如程式刻意留在 unreal mode），持續在子狀態執行，第一個 `INT`、`hlt` 或 §2.6 的一百萬步條件即失敗即關閉。

### 2.8 狀態檔

- 不升版本、不加欄位：子狀態由 `PM` 版本 3 既有的 `segmented` byte（位移 80）與 `CR0`（位移 4）表示。本規格之前的程式都不會寫出
  `segmented=1 ∧ PE=0`（`SetCR0` 拒絕、`decodePMV3` 拒絕），所以沒有任何既有合法存檔改變意義；gob 位元組錨點不變，符合 431 §4.2 的定案。
  舊版程式讀到子狀態存檔時以「segmented=1 要求 PE=1」拒絕，屬失敗即關閉。
- `decodePMV3`（`pmstate.go:272-306`）的 `segmented=1` 驗證改為：`VM=0`、`pmCPL=0`，且（`PE=1`）或（`PE=0` 且 `PG=0`）。本規格只放寬這一項；
  `cache[CS]`、`cache[SS]` 的 D/B 檢查與 `Attr` 保留位元檢查不變。另一份規格 443（D=1 碼段與 B=1 堆疊，起草中）可能再放寬 D/B 檢查，以 443 為準。
- `encodePM` 仍輸出版本 3、142 bytes。`Snapshot`／`Restore` 不改：`applyPM`（`pmstate.go:313-327`）先 `SetSegmented`、寫快取，最後
  `recalcPM`，`xlat` 與 CPU 的 `realCache()` 都由還原後的欄位推得；`leavePending` 經 `SetSegmented`／`SetSegCache` 清除。
- `LoadState`、`Restore` 不重新做離開檢查（同 434 §3.2「不重新推導」）。步與步之間的子狀態不會已滿足條件；手造的存檔若已滿足，
  讀回後仍在子狀態，下一次段載入才離開。
- profile 關閉時 `PM` 為 nil，`StateDigest` 與錨點測試常數不變；profile 開啟時只有新出現的子狀態會產生新的位元組。

### 2.9 觀測

434 §7 的 `PCLinear()`、`InsnLinear()`、`Coverage`、`atBreak` 以 `CPU.Segmented()` 選擇快取基底，子狀態下自動正確（`machine.go:1471-1473`、
`1664-1681`）。438 的 `PCPhys()` 在子狀態走 `xlat` 分支、`paging=false`，只套 A20。`Translating()` 在子狀態為 true，`internal/dos` 的
診斷回溯與 `cmd/probe` 的 peek 因此經線性層讀取。`probe.go` 的 word 監看在 `xlat` 改變時重新取樣（`syncWordXlat`），離開子狀態時會觸發一次。

### 2.10 要改的程式與測試

| 位置 | 改動 |
|---|---|
| `internal/machine/linear.go` | `recalcPM` 的 `xlat`（§2.3）；`SetCR0` 的表（§2.2），刪除 117–119 行的原條件；註解第 11 行（「轉換開啟」定義）、107–108 行（S9 不支援） |
| `internal/machine/handoff.go` | `finishStep` 處理 `ErrLeaveSegmented`（位置見 §2.7）；新增 `leaveSegmented` |
| `internal/machine/sysregs.go` | `IVTMismatch(vec)`（§2.7）；`ivtGuardIRQ` 傳向量並在診斷寫出界限與向量；註解 80–82 行 |
| `internal/machine/pmstate.go` | `decodePMV3` 驗證；註解 190–191 行（版本 3 的驗證敘述） |
| `internal/cpu/sys.go` | `ErrLeaveSegmented`；`SysBus.IVTMismatch(vec uint8)` 與註解第 26 行；`ivtGuard` 傳向量，子狀態診斷；`setCR0` 在進入子狀態後設 `leavePending`；`execSys0F` 的 `0F 00`、`LAR`、`LSL` 在子狀態走實模式分支 |
| `internal/cpu/segmented.go` | `realCache()`；`loadSeg` 的實模式分支（§2.4） |
| `internal/cpu/segcache.go` | `SetSegmented`、`SetSegCache` 清 `leavePending`；第 8 行不變式註解（§2.1） |
| `internal/cpu/ops.go` | `CF` 的子狀態分支；`farRet` 的 `retRPLOK` 條件；`Step` 的哨兵與失敗分支清旗標 |
| `internal/cpu/ops386.go` | `66 CF` 與 `66` 遠轉移在子狀態失敗即關閉 |
| `internal/cpu/cpu.go` | `leavePending` 欄位 |

既有測試的連動：

| 測試 | 改動與新規則下的預期 |
|---|---|
| `internal/cpu/sys_test.go:14` `fakeSys.CR()` 回 `0x10`（PE=0） | 不改 `fakeSys`。`internal/cpu/segmented_test.go:33-38` 的 `cplSys` 覆寫 `CR(0)` 回 `0x11`（PE=1），使 `newSegCPU`（40-52 行，`SetSegmented(true)`）仍代表分段模式。否則 `TestSegmentedCPLChecks`（54-75 行）、`TestSegmentedLTRRequiresCPL0`（88-100 行）與 `v86return_test.go:17`、`34-50` 會改走子狀態，語意改變 |
| `internal/cpu/sys_test.go:23` `fakeSys.IVTMismatch()` | 簽章改為 `IVTMismatch(uint8) bool`，仍回 false |
| 新增 cpu 層測試 | 替身的 `CR(0)` 回 PE=0、`SetSegmented(true)`：`loadSeg` 走實模式分支（基底＝值×16、不呼叫 `ReadSup32`）；`iret` 不看 NT；`66 CF` 失敗即關閉；`farRet` 不查 RPL |
| `internal/machine/handoff_test.go:135-182` | 「分段模式下 `SetCR0(0)`、`SetCR(0, pmCR0Init)` 清 PE 被拒」改為接受並進入子狀態；「`mov cr0` 清 PE 失敗即關閉」改為 §3 第 3 項的期望；另補「清 PE 而保留 PG」被拒 |
| `internal/machine/segstate_test.go` | `segmented=1` 且 `PE=0` 被拒的案例改為：`PG=0` 接受、`PG=1` 被拒 |
| `internal/machine/sysregs_test.go:398-415` `TestIVTMismatchCondition` | 改呼叫 `IVTMismatch(vec)`。原預期在新規則下不變：profile 關閉不成立；基底 0、界限 `3FFh` 對向量 0 與 `FFh` 都不成立；基底 `1000h` 成立；PE=1 不成立。補：基底 0、界限 `1Fh` 時向量 7 不成立、向量 8 成立；界限 `7FFh` 時向量 `FFh` 不成立 |
| `internal/machine/sysregs_test.go:236` | 只驗手造版本 2 解碼出 `idtrLimit=07FFh`，不呼叫守門，新規則下不變 |
| `internal/machine/sysinsn_test.go:426` `guardProg` | 基底 `12000h`、界限 `7FFh`，改呼叫 `IVTMismatch(vec)`（取該測試使用的向量）；基底非 0，新規則下仍成立，各守門點預期不變 |

`internal/cpu386`、`internal/state`、LE 與 DPMI 路徑、`oracle`、`session` 不改。

### 2.11 不在範圍

- 由子狀態再進保護模式（`PE` 在子狀態設回 1），以及卸載後重新載入 JEMM 的「再進入」（`docs/re/004` §4 的 S9 後半）。前者失敗即關閉；後者走
  433／434 既有的實模式→分段模式路徑，是否成立另行驗證。
- 從 V86 清 PE（`VM=1`，維持拒絕）；清 PE 而保留 PG（失敗即關閉）。
- 子狀態下的中斷、例外遞送與 IRQ 遞送；長時間停留的 unreal mode。
- 子狀態的 `66` 遠轉移、`iretd`；32 位元碼段與 B=1 堆疊（443）。
- 真機在實模式的型別檢查與 accessed 位元行為；DOSBox-X 不檢查界限與型別（刻意差異，§2.3）。
- `#GP`、`#SS`、`#UD` 的遞送（S6）；VCPI 分支；TR 忙碌位元在卸載後的處置。

## 3. 驗收

1. profile 關閉時：錨點測試常數與 gob 位元組不變；`StateDigest` 不變；`internal/cpu386`、`internal/state` 與 LE 路徑 `git diff` 為空。
2. SingleStepTests 以掛載的語料實際執行、315 個子測試全過、0 skip，`knownGaps` 不變（掛載方式同 433 §8.2）。
3. 合成程式重現 `5180`–`51ED`（machine 層測試，不依賴原版檔）：
   - 組態：profile 開啟、延伸記憶體足以容納 `X＝110000h` 起 64 KB、A20 開。GDT 放在 `X+1400h`，`70h`、`78h` 照 §1.2 的範本、基底 `X`
     （**基底必須在 `10FFF0h` 以上**，否則 20 位元遮罩的錯誤不會現形）。程式碼放 `X+5180h` 起，逐位元組照 `img.dis` 第 2600–2638 行。
     `X+3E18h` 放（3FFh, 0）；線性 `3E18h`（低記憶體）另放不同的值，證明 `lidt` 經 DS 快取讀取。堆疊 `X+SP` 放 V86 框架，各欄位取互不相同的值，
     CS 欄取 RPL 非 0 的值（例如 `0123h`），FLAGS 欄的 IF＝1。
   - 兩個變體：PG＝1（合成恆等頁表涵蓋 `X` 起的頁）與 PG＝0。
   - 期望：`519E` 之後 `segmented=true`、`CR0=00000010h`（加上原有 MP、EM、TS）、`Translating()=true`、`CPU.Code==nil`、`PCLinear()＝X+51A1h`，
     下一道正確取到 `51A1` 的 `EB 00`；`51A3` 之後 IDTR＝（0, 3FFh）；`51BC` 之後 SS 快取基底＝新 SS×16、界限 `FFFFh`、D/B＝0；
     `51C1`–`51CE` 的推入寫在新 SS×16＋SP；`51EA` 之後 DS 基底＝新 DS×16、`segmented` 仍為 true；`51ED` 之後同一步 `segmented=false`、六個快取全 0、
     `Translating()=false`、CS:IP、FLAGS、EAX–EDI、段值都等於框架與 `pushad` 區的值。
   - 離開後下一步：預先排入的 IRQ0（子狀態期間 `irq0Pending` 保持、`Ticks` 不增）經 IVT 遞送；另一測試以 `IntHook` 攔截的 `INT 21h` 照常執行。
   - 變體：`51ED` 前 FLAGS 的 NT＝1，仍走實模式 iret（不回 `ErrTaskReturn`）。
4. 進入與離開時機：
   - 分段模式的快取恰好全是實模式推得值（自建 GDT，描述元基底＝選擇子×16、界限 `FFFFh`）時，清 PE 的那一步就離開。
   - FS、GS 各一例：其餘五個都重載成實模式推得值，只有 FS（或 GS）沒有重載，不離開；重載後同一步離開。
   - SS 在分段模式載入 B＝1 的描述元，清 PE 後不重載 SS：不離開（條件 5），堆疊操作照 434 失敗即關閉；重載 SS 後 D/B 清除，可以離開。
   - 段載入成功但同一步稍後失敗時不離開、不回哨兵，旗標清除：子狀態 16 位元 `call far` 的推入超出 SS 界限（SS 在分段模式載入界限 `0FFFh` 的描述元、
     子狀態重載、`SP=0002h`）；`les` 讀遠指標的第二個 word 超出 DS 界限（`c.fault` 已設，`loadSeg` 不寫）。之後即使快取符合條件也不離開，直到下一次成功的段載入。
   - `pop ds`、`mov ds,[m]` 讀取失敗時 `loadSeg` 不寫段值與快取（§2.4 第 1 步）。
   - unreal：分段模式下 DS 載入基底 0、界限 `FFFFFh` 的描述元，清 PE 後 `mov ds` 載入 0，`67` 定址讀 `[ebx]`（`EBX=00123456h`）取得線性
     `123456h` 的值；不離開子狀態；接著 `INT 21h` 失敗即關閉，診斷含「442」。
   - 每個 §2.5 表中的段載入位置在子狀態各以合成程式觸發一次，確認是實模式語意（基底＝值×16、界限與屬性保留、CS／SS 的 D/B 清除、不讀描述元表）。
   - 目標快取 P＝0（分段模式下 FS 載入空選擇子）時，子狀態 `mov fs` 設成實模式預設快取（基底＝值×16、界限 `FFFFh`、屬性 `0093h`），經 FS 讀寫成功，可參與離開。
   - `retf` 彈出 RPL 非 0 的 CS 在子狀態成功。
   - 子狀態內 `clts`、只改 MP 或 TS 的 `mov cr0` 成功、不離開子狀態。
5. A20 關閉的子狀態：`readLin` 遮 bit 20（基底 `110000h` 的存取落在 `010000h`），`CPU.Code==nil`；`syncCodeFastPath` 在 A20 關且 `xlat` 開時讓 `Code` 為 nil；
   離開後 `xlat=false`、A20 關，`Code` 恢復為 `m.Mem`。
6. 失敗即關閉的每條路徑（各一例，失敗後相關狀態不改）：分段模式清 PE 而 PG＝1；子狀態設 PE＝1（`mov cr0` 與 `lmsw` 各一）；子狀態設 PG＝1；
   子狀態下 `INT n`、`INT3`、`INTO`、`div 0`、`BOUND`、`hlt`、`66 CF`、`66 EA`、`66 9A`、`66 CA`、`66 CB`、`66 FF /3`、`66 FF /5`、`0F 00 /0`–`/3`、
   `LAR`、`LSL`、`mov cs`、`pop cs`；實模式 iret 的框架超出 SS 界限（SS 界限 `0FFFh`、`SP=0FFEh`；`SP`、CS、`IP`、快取不變）；
   離開後 IDTR 基底 0、界限 `1Fh` 時 `INT 08h` 與 IRQ0 失敗即關閉、`INT 07h` 照常，界限改回 `3FFh` 後照常；離開不受 IDTR 影響（IDTR 仍為保護模式 IDT 時也離開，
   之後 `lidt` 還原再 `sti`，IRQ0 照常遞送）；IRQ0、按鍵、`startCallback`、`periodic` 在子狀態延後且不消耗；`IF=1` 且有待處理 IRQ 一百萬步後失敗
   （`IF=0` 時不失敗）；子狀態下 `SetVM(true)`、`SetPMCPL(3)` 被拒。
7. 狀態檔：子狀態（第 3 項的序列停在 `51A3` 之後）`SaveState`／`LoadState` 與 `Snapshot`／`Restore` 往返，接著跑完到離開後再多 100 步，最終
   `SaveState` 位元組與不中斷的執行相同；`encodePM` 仍為版本 3、142 bytes；手造版本 3 的 `segmented=1`、`PE=0`、`PG=0` 可讀，`PG=1`、`VM=1`、
   `pmCPL≠0` 各被拒；手造「已滿足離開條件」的子狀態存檔讀回後仍在子狀態，下一次段載入才離開；手造版本 1、2 的既有測試不變；離開後的存檔
   `segmented=0` 且快取全 0，可讀回。
8. 突變自檢（各至少一個測試失敗）：`xlat` 不含 `segmented`；`SetCR0` 不要求 PG＝0；保留 `linear.go:117` 的原條件（子狀態內 `clts` 被拒）；
   子狀態允許 PE 設回 1；子狀態 `loadSeg` 仍查描述元表；`loadSeg` 實模式分支不看 `c.fault`；P＝0 時不設預設快取；實模式載入把 `Limit` 重設為 `FFFFh`；
   實模式載入不清 CS／SS 的 D/B；實模式載入改寫 `Attr`；離開條件略過基底比較、界限比較、P、EC、D/B 任一項；離開條件只檢查 ES、CS、SS、DS
   （略過 FS 或 GS 各一）；進入那一步不設旗標；指令失敗時不清旗標；`SetSegCache` 不清旗標；離開時不清快取；離開時不呼叫 `recalcPM`；
   離開時不 `resetSegWait`；`IVTMismatch` 不看界限、或以固定 `3FFh` 比較（CPU 側與 Machine 側各自突變）；子狀態 `iret` 先看 NT；子狀態 `iret` 先動 `SP` 再驗；
   `66 CF` 在子狀態走 `v86ReturnRead`；`farRet` 在子狀態仍做 `retRPLOK`；`0F 00` 在子狀態進 `seg0F00`；`decodePMV3` 仍拒絕子狀態；
   `decodePMV3` 不驗 PG。
9. 效能：profile 關閉的 `BenchmarkStepRealMode` 與本規格動工前的 commit 交錯比較（431 §7），照實記錄 load average；主機不安靜時與 432–437 同列開放。
10. `go build ./...`、`go vet ./internal/... ./cmd/probe`、`go test ./internal/... ./cmd/probe ./session ./oracle` 全綠；`go test ./apps/...` 與改動前相同
    （437 收據記錄的唯一既有 FAIL 為 GLFW）。
11. 獨立對照（主代理另附收據）：把第 3 項的合成程式在 DOSBox-X 下跑一次（`~/cht/DOSBox-X-MCP-Debugger` 的除錯器橋接，只下中斷點、讀暫存器與記憶體，
    不寫模擬狀態），比對 `51ED` 之後的 CS:IP、SS:SP、FLAGS、EAX–EDI 與 dosgolem 相同。做不到時收據寫明原因。
12. 觸發案例（需原版，主代理另附 probe 收據）：沿用 `probe451a.log` 的旗標與起點 state（`/st/s1900.state`），跑到 SCCD 結束或下一個停點。
    `probe451a.log` 的停點在 `strike.exe` 的 EXEC 失敗之後；若改用 441 的 `-exec-reclaim`，到達 `519E` 的路徑會不同，收據寫明實際旗標。
    - 清 PG 之前一步記錄：`CR3` 所指頁表中線性 `X+5000h` 所在頁的 PTE，確認對應實體 `X+5000h`（§1.2 的恆等映射，強推論）；GDT 副本 `70h`、`78h` 的基底與屬性。
    - `0070:519E` 之後進入子狀態；`51A3` 之後的 IDTR 值；`51ED` 的步數與離開後的 CS:IP、FLAGS（含 NT、IF）、段值、`EAX=0`；子狀態期間的步數。
    - 離開後：照實記錄最後停點、原因、總步數、`EXEC` 紀錄、主控台輸出、視訊模式變化，以及 SCCD 是否以 `AH=4Ch` 結束。
    - 若在 `51ED` 之前停在別處，或離開後第一個中斷被 IDTR 守門擋下，照實記錄停點與原因，不改程式湊結果。

## 4. 未知與已決定事項

### 4.1 未知

| 項目 | 現況 |
|---|---|
| Intel 386 在實模式載入段暫存器時是否保留 Attr（含 D/B）、是否檢查型別、P＝0 快取的行為、實模式遠轉移後 CS 的屬性 | 本規格未讀 SDM；§2.3、§2.4 以 DOSBox-X 為準，型別檢查從嚴 |
| 觸發 `583C` 的 `int 15h` 子功能 | `5822 jmp [di]` 的表在 `7704h`，未解讀 |
| `51ED` 當下的 NT、IF，框架內容 | 由觸發案例記錄 |
| `[3E18h]` 副本的值是否為（0, 3FFh） | 強推論（`41ED` 在 `41F7 lidt` 之前、`4245` 複製之前），由觸發案例記錄 |
| 卸載時 `70h`、`78h` 的基底是否為 `X`、`X` 起的頁是否恆等映射 | 強推論，由觸發案例記錄 |
| SCCD 在 JEMM 卸載後的流程、是否再次載入 JEMM | 未知，觸發案例照實記錄 |

### 4.2 已決定（2026-10-16，主代理依審查定案）

1. **狀態檔不升版本**。子狀態由版本 3 既有的 `segmented` 與 `CR0` 表示；本規格之前沒有程式會寫出 `segmented=1 ∧ PE=0`，放寬驗證不改變任何既有存檔的意義，
   舊版程式讀到時失敗即關閉。升版本只增加測試改動，沒有換來資訊。
2. **IDTR 在中斷當下檢查**，`IVTMismatch` 改為依向量（§2.7）。離開點不等於程式邏輯上回實模式完成的時點，離開時檢查會擋下「最後才 `lidt`」的正常寫法；
   依向量比較界限只擋真機也會出錯的情形，界限大於 `3FFh` 不受影響。
3. **實模式載入清 CS／SS 的 D/B**，照 DOSBox-X（§2.4 第 5 步）。DOSBox-X 是本專案的交叉對照基準；本案 `70h`、`78h` 的 D/B 都是 0，結果與保留相同。
4. **子狀態沿用從嚴型別檢查**（§2.3）。與 434 的失敗即關閉一致，偏離時停下而不是安靜執行；造成的視窗內外不一致已在 §2.3 註明，JEMM 路徑不觸發。
