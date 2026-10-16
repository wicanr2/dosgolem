# 443：分段模式的 32 位元碼段（D=1）與 32 位元堆疊（B=1）

狀態：**DRAFT**（第 1 版）
日期：2026-10-16
前置：[`431-hybrid-cpu-protected-mode`](431-hybrid-cpu-protected-mode.md)（第 2 版）、[`432-linear-memory-and-paging`](432-linear-memory-and-paging.md)、
[`434-legacy-core-segmented-mode`](434-legacy-core-segmented-mode.md)、[`435-v86-hooks-and-idt-delivery`](435-v86-hooks-and-idt-delivery.md)、
[`436-tss-task-return-v86-entry`](436-tss-task-return-v86-entry.md)、[`437-segmented-32bit-far-transfers`](437-segmented-32bit-far-transfers.md)、
[`442-segmented-return-to-real-mode`](442-segmented-return-to-real-mode.md)（第 2 版）

DOSBox-X 引用一律指 `DOSBox-X-MCP-Debugger/dosbox-src/src/` 下的檔案。JEMM 位址為 img 位移（`strike_commander` `docs/re/003` §1）；
反組譯在 `strike_commander` 的 `workplace/re/jemm-api/img.dis`，`98h` 段的清單在 `workplace/re/cpu-inventory/r2.lst` 的 `P32:` 列，GDT 範本解析在
`workplace/re/cpu-inventory/gdt_decoded.txt`（皆為本機）。`X` 為 JEMM 的 XMS 區塊線性位址（434 收據實測 `110000h`）。

使用者定案（2026-10-16）：JEMM 的 32 位元碼段由既有 `internal/cpu` 在分段模式加 D=1 碼段與 B=1 堆疊執行，不交給 cpu386。

---

## 1. 為什麼要做

`strike_commander` 的 `workplace/logs/probe452.log`（從 `s1900.state` 起跑，共 19016571292 道）停止原因：

```text
cpu: 0018:5B90 op EA：分段模式載入 CS=0098：D=1 的 32 位元碼段屬後續規格（docs/spec/434 §5.1）
CS:IP ＝ 0018:5B95  AX=0028 BX=13BC CX=0200 DX=0000
DS=0028 ES=0028 SS:SP=0110:0FF0  IF=true  視訊模式 13h
```

開過的檔最後一個是 `strike.exe`，所以發出這次 `int 15h` 的是 STRIKE.EXE（強推論，依開檔順序）。434 §5.1 的 `loadSegPM` 在 CS 的 D=1 時失敗即關閉
（`internal/cpu/segmented.go:249-251`）。

### 1.1 JEMM 的 `int15` CU 進入端

| img | 指令 | 說明 | 等級 |
|---|---|---|---|
| `5B5C`–`5B65` | `mov ecx,esp`；`mov esp,ss:[4]`；`push ecx` | 在 `SS=50h` 上換成 `ss:[4]` 存的 ESP，推入舊 ESP | 已證實（靜態） |
| `5B67`–`5B6A` | `mov cx,110h`；`mov ss,cx` | 換到 `110h` 堆疊，ESP 不變 | 已證實（靜態） |
| `5B6C`–`5B73` | `push es`；`push bx`；`mov ecx,es:[bx+18h]`；`mov edx,es:[bx+14h]` | ES:BX 是分派框架，ECX、EDX 取自呼叫者 | 已證實（靜態） |
| `5B78`–`5B84` | `GS=80h`；`DS=ES=FS=28h` | | 已證實（靜態） |
| `5B86` | `sti` | CU 期間 IF=1 | 已證實（靜態、probe） |
| `5B87`–`5B8D` | `push 0`；`push 18h`；`push 0`；`push 5B95h` | 四個 16 位元 push，記憶體由低到高為 `5B95 0000 0018 0000`，即 32 位元 `retf` 的 `EIP=00005B95h`、`CS=00000018h` | 已證實（靜態） |
| `5B90` | `jmp 98:15` | 16 位元遠跳到 D=1 碼段 | 已證實（靜態、probe） |
| `5B95`–`5B9D` | `cli`；`pop bx`；`pop es`；`mov ss,50h`；`pop ecx` | 返回點 | 已證實（靜態） |

`5E60`–`5E6A`（`sti` 後同樣推四個 word，`jmp 98:0`）與 `7107`–`7117`（無 `sti`，`jmp 98:0`）是另外兩個進入點，框架相同（已證實，靜態）。

### 1.2 `98h` 段的內容與目前的派送

`98h` 段共 `90h` bytes（`0087h`–`008Bh` 是 `0086h` 的 `jmp 8C` 跳過的空隙）。下表依 D=1 的預設（運算元、定址皆 32 位元）解讀，最右欄是
現行既有核心在「讓 CS 載入成功、其餘不改」時的結果（讀碼，已證實）：

| 偏移 | 指令 | 目前的派送 |
|---|---|---|
| `0000`–`0005` | `mov edx,ecx`；`and edx,3`；`shr ecx,2` | 需要 `operand32=true` 才走 `execute32`；預設值不是 32 時以 16 位元執行，高半遺失 |
| `0008`、`000E`、`0034`、`0039`、`0062`、`0077`、`0084` | `je`／`jne` rel8 | `70`–`7F` 在 `sizeSensitive`，`operand32` 時落到 `execute32` 的 default 失敗（`ops386.go:46` 的註解寫明不實作） |
| `000A`、`0012`、`0040`、`0060` | `rep movsd`、`rep movsb`、`rep movsd`、`repz cmpsd` | `addr32` 時 `implicitAddr` 失敗（`ops386.go:71-85`、`96`） |
| `0014`、`008F` | `retf` | `operand32` 時走 437 的 `farRet32`（`ops386.go:470-481`），可用 |
| `0015` | `65 67 8B 36 2C 01`：`mov esi,gs:[012Ch]` | D=1 下 `67` 使定址變 16 位元（disp16），運算元仍 32 位元 |
| `0031`、`008C` | `66 0B D2`、`66 33 C0`：`or dx,dx`、`xor ax,ax` | D=1 下 `66` 使運算元變 16 位元，走 `executeCore` |
| `0021`、`002A`、`0036`、`0046`、`0048` | `mov al,cl`、`mov al,ch`、`cmp ch,0`、`mov al,cl`、`xor al,3` | 位元組運算，不看運算元大小；帶 ModRM 者在 `addr32` 下走 `decodeModRM32`，`mod=3` 無差異 |
| `0069`、`0072` | `mov eax,[esi-4]`、`mov eax,[esi]` | ModRM32，預設段 DS（16 位元定址的 `rm=6` 是 BP／SS，32 位元是 ESI／DS） |
| `006C`、`007D` | `mov fs:[ebx-4],eax`、`mov fs:[ebx],eax` | ModRM32 加 FS 前綴 |
| `0070` | `jecxz` | `addr32` 時 `implicitAddr` 失敗 |
| `0074` | `cmp eax,es:[edi]` | ModRM32 加 ES 前綴 |
| `0042`、`0086` | `jmp short` | `EB` 不在 `sizeSensitive`，走 `executeCore`（`ops.go:515-518`），IP 16 位元環繞 |

CU 的資料流（強推論，靜態）：`ESI＝[GS:12Ch]＋CL×10000h`（來源），`EDI＝[GS:12Ch]＋CH×10000h`。`DX=0` 或 `CH=0` 時以 `rep movsd` 複製 `3E80h`
（16000）個 dword。否則比較來源與 `[GS:12Ch]＋(CL xor 3)×10000h`，只把不相同的 dword 經 FS 寫到 `[GS:12Ch]＋CH×10000h` 的對應位置（差分複製，
`FA00h`＝320×200 bytes）。

### 1.3 描述元（以快取為準）

| 選擇子 | 範本（`gdt_decoded.txt`） | 執行時改寫 | 等級 |
|---|---|---|---|
| `98h` | 碼段 `9Ah`、D=1、G=0、界限 `FFFFFh` | 基底由 `433E`、`4345` 寫成 `X+7780h`（`0778h<<4` 加 `[3EA9]`）；`img.dis` 沒有以直接位址寫 `[149Eh]`（旗標 byte）的指令 | 基底強推論（靜態）；D=1 強推論 |
| `110h` | 資料 `96h`（向下展開、可寫）、B=1、界限 0 | 基底由 `431D`、`4324` 寫成 `X`；B 位元由 `int15` SP 子功能改寫：`5CE9` 設、`5CFA` 在 `CX=0` 時清（437 §1） | 已證實（靜態） |
| `128h` | 碼段 | D 位元由 `5CE4` 設、`5CF5` 在 `CX=0` 時清 | 已證實（靜態） |
| `28h` | 資料 `92h`、G=1、基底 0、4 GB | | 已證實（靜態、434 收據） |
| `80h` | 資料 `92h`、G=0、界限 `FFFFFh` | 基底由 `62FE`、`6305` 改寫（434 §1.1） | 已證實（靜態） |

**觸發當下 `110h` 的 B 是 0（已證實）**：434 讓分段模式的每個 `push` 先經 `stackCheck`，B=1 時失敗即關閉（`segmented.go:137-142`）。
`5B6A mov ss,110h` 之後的六個 push（`5B6C`–`5B8D`）都通過，停點在其後的 `5B90`，所以當時 `cache[SS]` 的 B=0。這與 437 收據第 1971307 步的傾印
（`110h` B=0）及 SP 子功能的 `CX=0` 清除一致。範本的 B=1 只在沒有呼叫過 `CX=0` 的 SP 子功能時成立，所以 D=1 碼段是這次觸發的必要條件，
B=1 堆疊是同一條路徑在另一種執行歷史下的必要條件（強推論）。本規格的 D、B、基底一律以執行時載入的快取為準，不以範本推定。

### 1.4 中斷

`5B86 sti` 之後才進 `98h`，所以 CU 期間 IF=1。434 §5.3 與 435 §3 規定分段模式（ring 0）的 IRQ 延後，不經 IDT 遞送；435 的 IDT 遞送只處理
V86 → ring 0。JEMM 為 ring 0 期間的中斷寫了專用路徑（已證實，靜態）：`4C2C` 比較中斷前的 `SS` 是否為 `110h`，`4C3F` 起在 `[3F54]≠0`
（CU、SP 進行中）時走 `4D24`–`4D66`，把 V86 框架改成 `AX=1209h`、`[bx+10h]=5243h`、`IP−2`，讓呼叫者在中斷處理完後重新發出 API。
本規格不改延後規則（§2.7），這條路徑不會被執行。

## 2. 規則

### 2.1 名詞與衍生旗標

- **D=1 碼段**：`cache[CS].Attr` 的 bit 14 為 1。**B=1 堆疊**：`cache[SS].Attr` 的 bit 14 為 1。
- CPU 新增未匯出欄位 `code32 bool`，不變式 `code32 ＝ segmented && cache[CS].Attr&attrDB != 0`。不持久化，由下列寫入點維護：
  `loadSegPM` 寫 CS 快取、`SetSegCache(CS, …)`、`SetSegmented`（`false` 時清為 false，`true` 時由 `cache[CS]` 重算）、442 §2.4 的實模式載入
  （CS 清 D 後為 false）。所有寫 `cache[CS]` 的位置經同一支內部方法，與 442 §2.7 的 `leavePending` 清除點共用（§2.9）。
- 堆疊寬度不另存欄位：分段模式的堆疊原語直接讀 `cache[SS].Attr` 的 B（只在 `segmented=true` 的分支）。
- `code32=true` 只可能在 `Model80386`、profile 開啟、`PE=1` 時出現（442 子狀態的進入條件見 §2.9）。

### 2.2 前綴與預設大小

照 DOSBox-X `core_normal.cpp:167-170`（每道指令開頭 `opcode_index＝code.big×200h`、`prefixes＝code.big`、`ea_table` 依 `code.big`）：

- `Step` 開頭由 `c.operand32 = false` 等常數改為 `c.operand32 = c.code32`、`c.addr32 = c.code32`、`c.ext386 = c.code32`。實模式與 V86 的
  `code32` 恆為 false，結果與現況相同。
- 前綴 `66` 設 `operand32 = !code32`，`67` 設 `addr32 = !code32`（照 DOSBox-X `prefix_none.h:347` 的 `opcode_index=(code.big^1)×200h` 與
  `core_normal.cpp:84-86` 的 `DO_PREFIX_ADDR`：是「設成相反」，不是翻轉，所以 `66 66` 與單一 `66` 相同，與 Intel 一致）。`ext386` 照舊設 true。
- `ext386` 在 D=1 下恆為 true，所以每道指令先經 `execute386`：`addr32` 時 A0–A3 走 `moffs32`（disp32），隱含位址指令走 §2.4；`operand32` 且在
  `sizeSensitive` 內走 `execute32`；其餘落到 `executeCore`。`66` 使 `operand32=false` 時 `sizeSensitive` 的指令回到 16 位元的 `executeCore`，
  `67` 使 `addr32=false` 時 ModRM 走 16 位元 `decodeModRM`、隱含位址指令用 SI／DI／CX。
- 熱路徑的改動是三個常數改成欄位讀取，依 431 §7 量 `BenchmarkStepRealMode`（§3 第 13 項）。

### 2.3 EIP

- 核心維持 16 位元 `IP`，D=1 碼段內 EIP 的高 16 位元恆為 0（不變式）。理由：`98h` 段的偏移都小於 `90h`，JEMM 的三個進入點偏移為 `0` 與 `15h`；
  加 EIP 高半要改狀態檔、診斷格式、`Op()`／`Rewind`／事件框架與觀測，換不到 JEMM 路徑上的行為。
- 守住不變式的位置（違反時失敗即關閉，診斷含「EIP 超過 FFFFh，docs/spec/443 §2.3」）：
  1. 取指令：`code32` 時 `segFetch8` 在 `IP=FFFFh` 取 byte 失敗（下一道的 EIP 會是 `10000h`）。刻意從嚴：最後一個 byte 在 `FFFFh` 的跳躍也被擋下。
  2. 相對轉移：目標以 `uint32(IP)＋符號延伸的位移` 計算（DOSBox-X `JumpCond32_b`、`JumpCond32_d`，`core_normal/support.h:76-88`；`EB` 見
     `prefix_66.h:652-657`），大於 `FFFFh` 失敗，`IP` 不改。適用 §2.4 表中的 `70`–`7F`、`EB`、`E0`–`E3`，以及既有的 `0F 80`–`8F`（`jccNear`
     在 `operand32` 時已做同樣檢查，`ops0f.go:202-214`，只改診斷文字）。
  3. 遠轉移與返回：目標偏移高 16 位元非 0 失敗，沿用 437 的 `farOffOK`（`ops386.go:422-428`）。
- 運算元大小為 16 的相對轉移（D=1 下帶 `66`）照 Intel 截成 16 位元，即現行 `executeCore` 的 16 位元環繞；在不變式下結果相同。

### 2.4 D=1 下的指令

| 指令 | `code32` 時的行為 | DOSBox-X |
|---|---|---|
| `70`–`7F` rel8（`operand32`） | §2.3 第 2 點的 32 位元目標；條件判斷沿用 `c.cond` | `prefix_66.h:221-252` |
| `EB` rel8（`operand32`） | 同上 | `prefix_66.h:652-657` |
| `E0`–`E2` `loopnz`／`loopz`／`loop` | 計數暫存器依 `addr32`：ECX（32 位元遞減，高半參與判斷）或 CX（只動低 16 位元）；目標依 §2.3 | `prefix_66.h:584-604` |
| `E3` `jecxz`／`jcxz` | `addr32` 時判 `ECX==0`，否則 `CX==0`；目標依 §2.3 | `prefix_66.h:605`，`AddrMaskTable`＝`core_normal.cpp:101` |
| 字串 `A4`–`A7`、`AA`–`AF`（`addr32`） | §2.5 | `core_normal/string.h:48-52`、`535-560` |
| 字串（`67` 使 `addr32=false`） | 現行 `stringOp`／`stringOp32`（SI／DI／CX） | |
| 其餘 `sizeSensitive` 已實作者（ALU、`mov`、`xchg`、移位、`imul`、`push`／`pop`、`pushad`／`popad`、`pushfd`／`popfd`、`in`／`out` 32 位元、`F7`、`FF /0 /1 /6`、`9A`／`EA`／`CA`／`CB`） | 照 `execute32` 現行語意；堆疊依 §2.6 | |
| `CF`（`operand32`，即 `iretd`） | 照 435 §2.5、436 §3.1 現行語意（NT=1 任務返回，否則 CPL 0 回 V86）；B=1 時見 §2.6 | |
| `0F` 雙位元組 | 照 `execute0F` 現行語意（依 `operand32`、`addr32`） | |

下列在 D=1 下失敗即關閉（`98h` 段沒有，照 197 的範圍規則不實作）：`E8`、`E9`、`C2`、`C3`、`FF /2`、`FF /4`、`C8`／`C9`（`execute32` 的 default），
`addr32` 下的 `6C`–`6F`（`ins`／`outs`）與 `D7`（`xlat`）。D=0 碼段（含實模式、V86）帶 `66` 的 `70`–`7F`、帶 `67` 的隱含位址指令維持現況（失敗即關閉），
本規格只在 `code32` 時開放上表。

### 2.5 32 位元定址的字串指令

`addr32` 時（D=1 預設，或 D=1 以外帶 `67`；`code32=false` 時，即實模式、V86 與 D=0 的分段模式，仍依 §2.4 末段失敗即關閉，所以實際只在 `code32` 成立）：

- 位址暫存器為 ESI、EDI，計數為 ECX，三者都以 32 位元遞增或遞減（DF），不截成 16 位元。偏移以 32 位元交給 `segRead`／`segWrite`
  （`w32=true`，各 byte 32 位元相加）。來源段可被前綴覆寫，目的段恆為 ES（`string.go:5-13` 的規則不變）。
- 運算元寬度依 `operand32`：byte（`A4`、`A6`、`AA`、`AC`、`AE`）、word（D=1 帶 `66`）、dword。
- `rep`：先看 ECX，為 0 時不執行；`repe`／`repne` 只對 `cmps`、`scas` 判 ZF，與 `stringOp` 相同。
- **整個 `rep` 在一次 `Step` 內跑完**，與現行 16 位元 `rep` 相同（`string.go:39-46`、`ops386.go:516-531`）；每次迭代照舊 `charge(cycString)`，
  `DOSBoxCost` 時 `Cycles++`。分段模式的 IRQ 延後（§2.7），所以 `rep` 中途不會有中斷到達，也不需要續跑語意。DOSBox-X 在 `CPU_Cycles≤0`
  或 `cpu_rep_max` 時中斷迴圈、以 `LOADIP` 讓指令重新開始（`string.h:60-67`、`535-560`），是刻意差異：dosgolem 的步數與週期語意以「一道
  `rep` 一步」為準，觸發案例的步數依此計算。
- 失敗：某次迭代的存取違反界限、型別或遇到分頁錯誤紀錄時，該次迭代不寫入，ESI／EDI／ECX 停在該次迭代之前的值，迴圈結束，`Step` 回報錯誤。
  之前的迭代已提交（記憶體與暫存器），與 434 §4.1「錯誤回報後狀態只供診斷」一致。現行 16 位元 `rep` 在失敗後仍跑完剩餘迭代
  （`stringOp` 的迴圈不看 `c.fault`），本規格不改它。
- 實作可把每次迭代的 byte 檢查合併成一次區間檢查（例如 dword 一次查 4 bytes），前提是失敗點與上一條相同。

### 2.6 B=1 堆疊

堆疊指標寬度看 SS 的 B，推入與彈出的寬度看運算元大小，兩者獨立（DOSBox-X `CPU_Push16`／`CPU_Push32`／`CPU_Pop16`／`CPU_Pop32` 以
`stack.mask`、`stack.notmask` 決定，`cpu.cpp:675-697`；`CPU_SetSegGeneral` 載入 SS 時依 `desc.Big()` 設定，`cpu.cpp:3028-3036`）。
JEMM 正是在將來可能為 B=1 的 `110h` 上做 16 位元 push。

- `segmented=true` 且 B=1：指標為 ESP（`Hi[SP]:R[SP]`）。push 先 `ESP −= n` 再寫；pop 先讀再 `ESP += n`；各 byte 偏移以 32 位元相加
  （`segRead`／`segWrite` 的 `w32=true`），不做 16 位元環繞。
- B=0（含實模式、V86）：現況，指標為 SP，`Hi[SP]` 不動，各 byte 16 位元環繞。
- 界限：照 434 §4.1 的 `segCheck`。向下展開段 B=1 時有效範圍是 `Limit+1` 到 `FFFFFFFFh`，`segCheck` 已依 B 選上界（`segmented.go:66-73`），不改。
- `stackCheck`（`segmented.go:137-142`）的「B=1 失敗即關閉」取消，改為堆疊指標的寬度存取器。下列位置改為依 B：

| 位置 | 改動 |
|---|---|
| `cpu.go:389-405` `push`／`pop` | 依 B 選 SP 或 ESP |
| `ops386.go:621-637` `push32`／`pop32` | 同上 |
| `ops386.go:444-462` `farCall32` | 預檢的 8 個 byte 位置依 B 計算（B=1 時 `ESP−8`–`ESP−1` 32 位元相減）；推入依 B |
| `ops386.go:470-481` `farRet32` | 兩個 dword 的位置依 B；`ESP += 8+iw`（B=1）或 `SP += 8+iw`（B=0），DOSBox-X `cpu.cpp:2314-2318` |
| `ops.go:367-371` `retf imm16`、`ops.go:339-343` `ret imm16` | `iw` 加到 ESP（B=1）或 SP（B=0）。DOSBox-X 的 `RETN Iw` 一律加到 `reg_esp`（`prefix_none.h:914-931`），B=0 時與 Intel 不同，這裡照 Intel |
| `ops.go:908-927` `farCall`／`farRet`（16 位元） | 經 `push`／`pop`，自動依 B |
| `ops186.go:11-33` `pusha`／`popa`、`ops386.go:195-209` `pushad`／`popad` | 經 `push`／`pop` 系列，自動依 B；`pusha` 推的是 SP 低 16 位元、`pushad` 推 ESP，與 Intel 相同 |

- 下列在 `segmented=true` 且 B=1 時失敗即關閉（JEMM 路徑沒有）：`enter`、`leave`（`ops186.go:150-170`，以 16 位元 SP／BP 計算）；435 §2.5 的
  `v86ReturnRead`（`v86event.go:76-80` 保留 B=1 檢查）；435 §2.3 第 4 步的 `SS0` B=1（`v86deliver.go:184`，不改）。
- 實模式、V86 與 `segmented=false` 的堆疊行為逐位元組不變。

### 2.7 進入與離開 D=1、中斷

- `loadSegPM` 的 CS 分支刪除 D=1 的拒絕（`segmented.go:249-251`）；其餘 434 §5.1 的 CS 檢查不變。可以載入 D=1 CS 的轉移：16 位元與 32 位元的遠跳
  （`EA`、`66 EA`、`FF /5`、`66 FF /5`）、遠呼叫（`9A`、`66 9A`、`FF /3`、`66 FF /3`）、遠返回（`CB`、`CA`、`66 CB`、`66 CA`）。反向（D=1 → D=0）同一組。
- 遠返回的框架寬度看運算元大小，不看目標段：D=1 碼段的 `CB` 彈出 32 位元 EIP 與 32 位元 CS（`farRet32`），帶 `66` 時彈出兩個 word
  （DOSBox-X `CPU_RET(use32,…)`，`cpu.cpp:2290-2318`）。JEMM 以四個 word 疊出的框架即對應 32 位元形式（§1.1）。
- 仍拒絕 D=1 的位置：435 §2.3 第 3 步 IDT 閘目標碼段 D=1（`v86deliver.go:149`，不改）；436 的任務返回只進 V86，不載入保護模式 CS，不受影響。
- **中斷維持 434 §5.3、435 §3 的延後**：分段模式下（不論 `code32`、B）IRQ0 與按鍵只累積不遞送、不消耗；軟體中斷與例外失敗即關閉。CU 的 IF=1
  視窗是 `5B86 sti` 到 `5B95 cli`，`rep` 一步跑完，整段在數十到十幾萬步之間（差分複製最壞每個 dword 約 9 道），遠低於 434 §5.3 的一百萬步條件。
  延後的 IRQ 在回到 V86 後經 435 遞送。真機會在 CU 中途送出 IRQ0 並走 JEMM `4C3F`／`4D24` 的重新發出路徑（§1.4），dosgolem 跳過該路徑，
  差別只在時間點（強推論：重新發出後 CU 從頭再做一次，結果相同；未驗證，§4.1）。
- 435 不改。ring 0 的同特權 IDT 遞送、`iretd` 彈出 `VM=0` 返回 D=1 碼段，列為不在範圍（§2.10）與待決定事項（§4.2 第 1 項）。

### 2.8 狀態檔

- 不升版本、不加欄位：D=1 與 B=1 由 `PM` 版本 3 既有的 `cache[CS]`、`cache[SS]` 的 `Attr` bit 14 表示（434 §2.2 的位移 82 起）；ESP 高半已在
  CPU 狀態的 `Hi`（`internal/machine/state.go:66`、`snapshot.go:128`）；EIP 高半依 §2.3 恆為 0，不需欄位。gob 位元組錨點不變（431 §4.2）。
- `decodePMV3`（`internal/machine/pmstate.go:302-307`）的「`cache[CS]`、`cache[SS]` 的 D/B 為 0」改為「`PE=0`（442 子狀態）時 `cache[CS]`、`cache[SS]`
  的 D/B 為 0」；`PE=1` 時接受 D/B=1。其餘驗證（`segmented`、保留 byte、`Attr` 的 bit 8–11、13）不變。
- 理由：本規格之前沒有程式會寫出 CS 的 D=1（`loadSegPM` 拒絕）；SS 的 B=1 可以載入（`loadSegPM` 不擋，`segmented.go:232-238`）但 `decodePMV3`
  拒絕，屬 442 §2.11 記下的既有問題，本規格一併解決。舊版程式讀到新存檔時以 D/B 檢查拒絕，屬失敗即關閉。
- `applyPM`（`pmstate.go:313-327`）經 `SetSegmented`、`SetSegCache` 還原，`code32` 隨之重算；不另加步驟。
- profile 關閉時 `PM` 為 nil，`StateDigest` 與錨點測試常數不變。

### 2.9 與 442 的交互

442 第 2 版（READY）與本規格改同一組程式，可能同時實作：

| 位置 | 442 | 443 | 合併規則 |
|---|---|---|---|
| `Step` 開頭與失敗分支 | 失敗分支清 `leavePending`；成功時看旗標回 `ErrLeaveSegmented` | 開頭三個欄位改為 `code32` | 兩者都保留，互不影響 |
| `SetSegmented`、`SetSegCache` | 清 `leavePending` | 重算 `code32` | 同一支方法內兩件事都做；所有寫 `cache[CS]` 的位置經它 |
| `loadSeg` | 先判 `realCache()` 走實模式分支，CS／SS 清 D/B | `loadSegPM` 不再拒絕 CS 的 D=1 | 實模式分支寫 CS 快取後 `code32=false` |
| `Machine.SetCR0`（442 §2.2 的表） | 分段模式清 PE 且清 PG 進入子狀態 | 新增一列：清 PE 時 `cache[CS]` 的 D=1 或 `cache[SS]` 的 B=1，回錯誤、`CR0` 不改（失敗即關閉） | 後實作者補上這一列。理由：子狀態第一道指令仍以快取解碼，D=1 會以 32 位元解讀實模式碼，442 §2.7 的離開條件也要求 CS、SS 的 D/B=0 |
| `decodePMV3` | `segmented=1` 允許 `PE=0 ∧ PG=0` | D/B=1 只在 `PE=1` 時接受 | 條件取交集：`PE=0` 時 D/B 必為 0（442 §2.8 已寫明以 443 為準） |
| `ops386.go` 的 `66 CF`、`66` 遠轉移 | 子狀態失敗即關閉 | D=1 的預設運算元大小也會走到這些分支 | 子狀態下 `code32` 恆為 false（上一列保證），不衝突 |
| `farRet` | 子狀態不做 `retRPLOK` | 經 `pop`，依 B | 互不影響 |

### 2.10 要改的程式與測試

| 位置 | 改動 |
|---|---|
| `internal/cpu/cpu.go` | `code32` 欄位；`push`／`pop` 依 B |
| `internal/cpu/ops.go` | `Step`（`ops.go:14-102`）的預設與 `66`／`67`；`ret imm16`、`retf imm16` 依 B；`EB`、`70`–`7F`、`E0`–`E3` 在 `code32` 的處理可放 `execute386` 的 `code32` 分支，`executeCore` 不加判斷 |
| `internal/cpu/ops386.go` | `execute386` 的隱含位址分派（`code32` 時進 §2.5，否則照舊失敗）；`execute32` 的 `70`–`7F`；`push32`／`pop32`、`farCall32`、`farRet32` 依 B |
| `internal/cpu/string.go` 或新檔 | §2.5 的 32 位元定址字串指令 |
| `internal/cpu/ops186.go` | `enter`、`leave` 在分段模式 B=1 時失敗即關閉 |
| `internal/cpu/ops0f.go` | `jccNear` 在 `code32` 的診斷文字 |
| `internal/cpu/segmented.go` | `segFetch8` 的 `IP=FFFFh` 檢查；`stackCheck` 改為寬度存取器；`loadSegPM` 刪 D=1 拒絕並經 §2.1 的寫入方法 |
| `internal/cpu/segcache.go` | `SetSegmented`、`SetSegCache` 重算 `code32`；檔頭不變式註解 |
| `internal/cpu/v86event.go` | `v86ReturnRead` 的 B=1 檢查改為明示（原經 `stackCheck`） |
| `internal/machine/pmstate.go` | `decodePMV3` 的 D/B 條件；檔頭註解 |
| 測試 | `internal/machine/segstate_test.go` 中「CS 或 SS 的 D/B=1 被拒」的案例改為 `PE=1` 接受、`PE=0` 被拒；`internal/cpu` 中期待「SS 的 B=1 失敗」「CS 的 D=1 失敗」的測試（以 grep `B=1`、`D=1`、`attrDB` 找出）改為本規格的期望 |

`internal/cpu386`、`internal/state`、LE 與 DPMI 路徑、`oracle`、`session`、435 的 `v86deliver.go`（除註解外）不改。

### 2.11 不在範圍

- ring 0 的同特權中斷與例外遞送（含 286／386 閘的框架寬度、B=1 堆疊上的推入）、`iretd` 彈出 `VM=0` 的同特權返回、JEMM `4C3F`／`4D24` 的
  CU 重新發出路徑。CPU 例外（`#GP`、`#SS`、`#PF`、`#DE`）的遞送屬 S6。
- `rep` 的可中斷與續跑。
- EIP 高 16 位元非 0；D=1 下的 `E8`、`E9`、`C2`、`C3`、`FF /2`、`FF /4`、`enter`、`leave`，`addr32` 下的 `ins`／`outs`／`xlat`。
- B=1 下的 `enter`、`leave`，B=1 的 `SS0`，B=1 堆疊上的 `iretd` 回 V86。
- D=0 碼段的 `66` 相對轉移與 `67` 隱含位址指令（實模式、V86、分段模式 D=0 都維持失敗即關閉）。
- CPL 3 的 D=1 碼段、一致碼段、呼叫閘、任務閘；V86 與 D=1 交錯（V86 由 435 從 D=0 的 `18h` 進出，D=1 不直接進出 V86）；442 子狀態下的 D=1、B=1（§2.9 擋下）。
- cpu386 與 DOS4GW／LE 路徑。

## 3. 驗收

1. profile 關閉時：錨點測試常數與 gob 位元組不變；`StateDigest` 不變；`internal/cpu386`、`internal/state` 與 LE 路徑 `git diff` 為空。
2. SingleStepTests 以掛載的語料實際執行、315 個子測試全過、0 skip，`knownGaps` 不變（掛載方式同 433 §8.2）。
3. 合成程式重現 JEMM CU（machine 層測試，不依賴原版檔）：
   - 組態：profile 開啟、分段模式、A20 開，延伸記憶體容納 `X＝110000h` 起的映像與三個 64 KB 緩衝區。GDT 依 §1.3：`18h`（D=0，基底 `X`）、
     `98h`（D=1，基底 `X+7780h`）、`110h`（向下展開、界限 0、基底 `X`）、`50h`（向下展開、B=0、界限 0、基底 `X`）、`28h`（G=1 平坦）、
     `80h`（G=0、界限 `FFFFFh`、基底取非 0 的值 `G`）。`18h` 段放 `5B5C`–`5B9D`、`98h` 段放 `0000`–`008F`，逐位元組照 `img.dis` 第 3408–3434 行與
     `r2.lst` 第 3921–3977 行。`G+12Ch` 放緩衝區基底 `B`；ES:BX 的框架放 ECX（CL、CH）與 EDX。
   - 兩個堆疊變體：`110h` 的 B=0 與 B=1。B=1 變體的 `ss:[4]` 放 `ESP=00011000h`（高半非 0），證明 `110h` 上的推入寫在 `X+10FFxh`、`50h` 上的
     `push ecx`／`pop ecx` 寫在 `X+0FFCh`，最後 ECX 正確還原。
   - 三個資料變體：`DX=0`（整段複製 16000 個 dword）、`CH=0`（整段複製）、差分複製。差分變體的來源與比較緩衝區在第一個、最後一個、連續一串、
     單一個 dword 不同，目的緩衝區預填哨兵；期望值以測試內獨立的參考迴圈計算（來源 ≠ 比較時目的＝來源，否則保持哨兵），不用被測核心的輸出。
     另一例只有最後一個 dword 不同（`repz cmpsd` 以 ECX=0、ZF=0 結束）。
   - 期望：`5B90` 之後 `CS=0098h`、`code32=true`、`IP=0015h`；`98:0015` 的 `65 67` 讀 `G+12Ch`；`98:008F retf` 之後 `CS=0018h`、`IP=5B95h`、
     `code32=false`；返回後 SP（或 ESP）等於四個 push 之前的值；`AX=0`、EAX 高半保留；`5B9D pop ecx` 之後 ECX 等於 `5B5C` 的舊 ESP。
   - `5E60`–`5E6A` 與 `98:0000`–`0014` 的複製常式：`ECX` 為 0、1、3、4、5、`10001h` 各一例，比對目的內容與 ESI、EDI、ECX 的終值。
4. 前綴與解碼：D=1 下無前綴、`66`、`67`、`66 67` 各以 `mov`、ModRM 記憶體運算元驗運算元與定址寬度；`66 66` 與單一 `66` 相同；`67 E3` 判 CX；
   `E3` 判 ECX（ECX=`10000h` 時不跳）；`E2` 以 ECX 遞減；`67 F3 A5` 用 SI／DI／CX 搬 dword；`0F 84` 取 rel32。D=0 碼段與實模式的同一批位元組
   行為不變（`66 74` 與 `67 A4` 仍失敗即關閉）。
5. EIP：`EB`、`74`、`E2`、`0F 84` 的目標大於 `FFFFh`（正向越界與負向環繞各一）失敗且 `IP` 不變；`code32` 下在 `IP=FFFFh` 取指令失敗；`66 EB`
   的 16 位元環繞照舊。
6. 字串：`A4`–`A7`、`AA`–`AF` 在 32 位元定址下各一例（DF=0、DF=1；ECX 高半非 0；來源段前綴；目的段不可覆寫）；`rep` 的 ECX=0；`repe`／`repne`
   的提前結束；中途在第 k 次迭代超出 ES 界限時，前 k−1 次已寫、第 k 次未寫，ESI、EDI、ECX 停在第 k 次之前。
7. B=1 堆疊：16 位元與 32 位元 push／pop 在 `ESP=00010000h` 時不做 16 位元環繞；向下展開界限的邊界（`ESP−2` 剛好等於 `Limit+1` 與 `Limit`）；
   `ret imm16`、`retf imm16`、`66 CA` 加到 ESP；`farCall32`／`farRet32` 在 B=1 下的位置與預檢；`pusha`／`pushad` 推入的指標值；B=0 的既有測試不變。
8. 進出 D=1：`EA`、`66 EA`、`9A`、`66 9A`、`FF /3`、`FF /5`（16 與 32 位元）載入 D=1 CS；D=1 的 `CB`（32 位元框架）、`66 CB`（16 位元框架）回 D=0；
   D=0 的 `CB` 返回 D=1；每次轉移後 `code32` 正確。
9. 中斷：D=1 碼段、IF=1、有待處理 IRQ0 與按鍵時不遞送、不消耗（`Ticks`、`irq0Pending`、鍵盤佇列不變），`segWait` 照常計數；回到 D=0 並
   `cli` 後仍保留；IF=1 且待處理一百萬步後失敗即關閉。D=1 下 `INT n`、`div 0`、`hlt` 失敗即關閉。
10. 失敗即關閉（各一例，失敗後 `Seg[CS]`、快取、`IP`、SP／ESP 不變）：D=1 下 `E8`、`E9`、`C2`、`C3`、`FF /2`、`FF /4`、`C8`、`C9`、`addr32` 下的 `6C`–`6F`
    與 `D7`；B=1 下 `enter`、`leave`、`iretd` 回 V86；IDT 閘目標為 D=1（435 不變）；`SS0` 為 B=1（435 不變）；442 實作後：分段模式清 PE 時
    `cache[CS]` 的 D=1 或 `cache[SS]` 的 B=1。
11. `code32` 不變式：每個合成測試在每步之後檢查 `code32 ＝ segmented && cache[CS].D`；`SetSegCache`、`SetSegmented`、`applyPM`、435 的 commit、436
    的任務返回、442 的離開與實模式載入各一例。
12. 狀態檔：第 3 項的 B=1 變體在 `98h` 碼內（`rep movsd` 之前）與 `110h` 堆疊上各存一次，`SaveState`／`LoadState` 與 `Snapshot`／`Restore` 往返後跑到
    `5B9D`，再多 100 步，最終 `SaveState` 位元組與不中斷的執行相同；`encodePM` 仍為版本 3、142 bytes；手造版本 3 的 `PE=1` 且 CS D=1 或 SS B=1
    可讀；`PE=0` 且 D/B=1 被拒（442 實作後）；手造版本 1、2 的既有測試不變。
13. 效能：profile 關閉的 `BenchmarkStepRealMode` 與本規格動工前的 commit 交錯比較（431 §7，不低於 97%），照實記錄 load average；主機不安靜時與
    432–442 同列開放。新增 `BenchmarkSegmentedRepMovsd32`（分段模式、分頁開、`rep movsd` 16000 個 dword），記錄每次迭代的 ns 與一次 CU 整段複製的時間，
    不設門檻；收據附上「每畫格一次 CU」佔 1/70 秒的比例。
14. 突變自檢（各至少一個測試失敗）：`Step` 開頭不依 `code32` 設預設；`66` 翻轉而非設定（`66 66` 測試）；`67` 不改 `addr32`；`loadSegPM` 寫 CS 快取
    不重算 `code32`；`SetSegCache` 不重算；32 位元定址字串用 SI／DI；`rep` 用 CX；`jecxz` 判 CX；`loop` 遞減 CX；相對轉移以 16 位元環繞；略過
    `IP=FFFFh` 取指令檢查；B=1 的 push 只動 SP；B=1 的 pop 以 16 位元環繞；`retf imm16` 在 B=1 只加 SP；`farRet32` 在 B=1 讀 SP；`farCall32` 預檢
    用 16 位元位置；字串失敗後仍繼續迭代；`decodePMV3` 仍拒絕 `PE=1` 的 D/B=1；`decodePMV3` 接受 `PE=0` 的 D/B=1。
15. `go build ./...`、`go vet ./internal/... ./cmd/probe`、`go test ./internal/... ./cmd/probe ./session ./oracle` 全綠，SingleStepTests 不得 skip；
    `go test ./apps/...` 與改動前逐行相同（437 收據記錄的唯一既有 FAIL 為 GLFW）。
16. 觸發案例（需原版，主代理另附 probe 收據）：從 `strike_commander/workplace/scratch/p452/s1900.state` 接著跑，第 19010000000 步點 (160,90)，
    帶 `-exec-env -exec-reclaim`，收據寫明完整旗標。記錄：
    - 到達 `0018:5B90` 的步數；當下 `cache[SS]` 的 `Attr`（B 的實際值）、`cache[CS]` 於 `jmp 98:15` 之後的基底、界限、`Attr`；
    - `[GS:12Ch]` 的值、框架內的 ECX（CL、CH）與 EDX、走整段複製或差分複製；
    - 返回 `0018:5B95` 的步數、CU 期間的步數、期間 `irq0Pending` 與 `segWait` 計數、回 V86 後 IRQ0 的遞送步數；
    - 之後 CU 是否重複發生（次數、間隔步數）；STRIKE.EXE 的後續停點、原因、總步數、視訊模式變化。
    - 停在別處時照實記錄停點與原因，不改程式湊結果。

## 4. 未知與待決定

### 4.1 未知

| 項目 | 現況 |
|---|---|
| 執行時 `98h` 的基底與 D | 基底 `X+7780h` 為強推論（`433E`、`4345`）；沒有找到改寫旗標的指令，D=1 為強推論。觸發案例記錄快取 |
| CU 被呼叫時 `110h` 的 B | 觸發當下為 0（已證實，§1.3）；其他執行歷史（沒有先呼叫 `CX=0` 的 SP）為 1，本遊戲是否發生未知 |
| CU 的用途與參數語意（CL、CH、DX、`[GS:12Ch]`） | §1.2 的資料流為強推論；`FA00h` 對應 320×200，推測為畫面緩衝的整段或差分上傳，未驗證 |
| 真機在 CU 中途收到 IRQ0 的結果 | JEMM `4D24`–`4D66` 讓呼叫者以 `AX=1209h`、`[bx+10h]=5243h` 重新發出，重新發出後的流程未解讀；延後後的結果是否與真機相同未驗證 |
| 長 `rep` 期間 IRQ0 週期的合併 | `irq0Pending` 是 bool（`internal/machine/machine.go:1595-1607`）：一道 `rep movsd` 跨過多個週期時只留一個待處理 IRQ0。週期時鐘加 `DOSBoxCost` 時 `nextIRQ0Cyc` 以絕對排程追趕，指令數時鐘則丟掉多餘週期。這是現行所有 `rep` 的共同行為，CU 的影響量由觸發案例記錄 |
| `5E6A`（XC）與 `7117` 的呼叫者 | 未知 |
| DOSBox-X 在同一點的交叉對照（CU 前後的緩衝區內容、ESP） | 未做，可用除錯器橋接下中斷點讀值（`strike_commander` `AGENTS.md` §2 定案允許） |
| Intel 386 在 D=1 下 `IP` 由 `FFFFh` 前進、`RETN Iw` 在 B=0 的語意 | 本規格未讀 SDM；§2.3 從嚴，§2.6 照 Intel 慣例（ESP 或 SP 依 B），待對照 |

### 4.2 待使用者決定

1. **CU 期間的 IRQ**。建議維持延後（§2.7）：與 434 §5.3、435 §3 一致，不需要 ring 0 同特權遞送、B=1 堆疊上的中斷框架、`iretd` 回 D=1，也不會執行
   JEMM 的重新發出路徑。替代：本規格加入 ring 0 的 IDT 遞送（同特權、依閘型別 286／386 推 3 個 word 或 dword，堆疊依 B），並支援 `iretd` 彈出
   `VM=0` 返回；JEMM 會因此走 `4C3F`／`4D24`，需要另外解讀並驗收該路徑。
2. **EIP 的表示**。建議維持 16 位元 `IP` 加不變式（§2.3）。替代：CPU 加 `IPHi`，狀態檔、`Op()`、`Rewind`、事件框架、觀測一併改，JEMM 路徑用不到。
3. **D=1 的指令範圍**。建議只開放 `98h` 段用到的形狀，加上同一實作自然涵蓋的字串指令與 `loop` 系列（§2.4）。替代：一次補齊 32 位元的近呼叫、
   近返回、`enter`／`leave`，代價是更多沒有原版證據的測試。
4. **`rep` 在一步內跑完**。建議維持（§2.5），與現行步數語意一致。替代：照 DOSBox-X 依週期切段並重新開始，要先定義步數與 `Cycles` 的對應，
   並改動既有 16 位元 `rep`。
5. **狀態檔不升版本**。建議不升（§2.8），理由同 442 §4.2 第 1 項。替代：升為版本 4、版面相同，只讓「可含 D=1 與 B=1」在版本號上可見。
