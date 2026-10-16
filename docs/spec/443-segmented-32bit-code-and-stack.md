# 443：分段模式的 32 位元碼段（D=1）

狀態：**READY**（第 2 版，依審查修訂）
日期：2026-10-16
前置：[`431-hybrid-cpu-protected-mode`](431-hybrid-cpu-protected-mode.md)（第 2 版）、[`432-linear-memory-and-paging`](432-linear-memory-and-paging.md)、
[`434-legacy-core-segmented-mode`](434-legacy-core-segmented-mode.md)、[`435-v86-hooks-and-idt-delivery`](435-v86-hooks-and-idt-delivery.md)、
[`436-tss-task-return-v86-entry`](436-tss-task-return-v86-entry.md)、[`437-segmented-32bit-far-transfers`](437-segmented-32bit-far-transfers.md)、
[`442-segmented-return-to-real-mode`](442-segmented-return-to-real-mode.md)（第 2 版）

DOSBox-X 引用一律指 `DOSBox-X-MCP-Debugger/dosbox-src/src/` 下的檔案。JEMM 位址為 img 位移（`strike_commander` `docs/re/003` §1）；
反組譯在 `strike_commander` 的 `workplace/re/jemm-api/img.dis`，`98h` 段的清單在 `workplace/re/cpu-inventory/r2.lst` 的 `P32:` 列，GDT 範本解析在
`workplace/re/cpu-inventory/gdt_decoded.txt`（皆為本機）。`X` 為 JEMM 的 XMS 區塊線性位址（434 收據實測 `110000h`）。
程式位置以 commit `968c736` 的內容為準（`git show HEAD:`）；工作樹中 442 的未提交實作與本規格的合併點見 §2.9。

使用者定案（2026-10-16）：JEMM 的 32 位元碼段由既有 `internal/cpu` 在分段模式執行，不交給 cpu386。第 1 版草稿涵蓋的 B=1 堆疊依審查移出本規格（§4.2 第 6 項）。

---

## 1. 為什麼要做

`strike_commander` 的 `workplace/logs/probe452.log`（從 `s1900.state` 起跑，共 19016571292 道）第 9–12 行：

```text
停止原因：cpu: 0018:5B90 op EA：分段模式載入 CS=0098：D=1 的 32 位元碼段屬後續規格（docs/spec/434 §5.1）
CS:IP ＝ 0018:5B95  AX=0028 BX=13BC CX=0200 DX=0000
計時器：送出 196970 次  IF=true  int08 向量 0080:0420  int1C 向量 0080:0070
DS=0028 ES=0028 SS:SP=0110:0FF0  視訊模式 13h
```

錯誤回報的是本道起點 `5B90`；`CS:IP` 已是 `5B95`，因為取指令照常前進 `IP`（`internal/cpu/cpu.go:333-340`），434 §4.1 規定失敗後的狀態只供診斷。
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
觸發當下 `CX=0200h`、`DX=0`，是 `5B6E`、`5B73` 載入之後的值，即 CL=0、CH=2、DX=0（已證實，probe）。

### 1.2 `98h` 段的內容與目前的派送

`98h` 段共 `90h` bytes（`0087h`–`008Bh` 是 `0086h` 的 `jmp 8C` 跳過的空隙）。下表依 D=1 的預設（運算元、定址皆 32 位元）解讀，最右欄是
現行既有核心在「讓 CS 載入成功、其餘不改」時的結果（讀碼，已證實）：

| 偏移 | 指令 | 目前的派送 |
|---|---|---|
| `0000`–`0005` | `mov edx,ecx`；`and edx,3`；`shr ecx,2` | 需要 `operand32=true` 才走 `execute32`；預設不是 32 時以 16 位元執行，高半遺失 |
| `0008`、`000E`、`0034`、`0039`、`0062`、`0077`、`0084` | `je`／`jne` rel8 | `70`–`7F` 在 `sizeSensitive`（`ops386.go:46` 的註解寫明不實作），`operand32` 時落到 `execute32` 的 default 失敗 |
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

CU 的分支（`r2.lst` `P32:0031`–`0039`，已證實，靜態）：

| 條件 | 路徑 |
|---|---|
| `DX=0` | `0034 je 3B`：整段複製，`rep movsd` 搬 `3E80h`（16000）個 dword |
| `DX≠0`、`CH≠0` | `0039` 不跳，落到 `003B`：整段複製 |
| `DX≠0`、`CH=0` | `0039 je 44`：差分複製 |

資料流（強推論，靜態）：`ESI＝[GS:12Ch]＋CL×10000h`（來源），`EDI＝[GS:12Ch]＋CH×10000h`（整段複製的目的）。差分複製時 `EDI＝[GS:12Ch]＋
(CL xor 3)×10000h`（比較緩衝區），`EBX＝[GS:12Ch]＋CH×10000h`（目的，CH=0 即 `[GS:12Ch]` 本身），比較 `FA00h`（320×200）bytes，只把來源與比較緩衝區
不同的 dword 經 FS 寫到目的的對應位置。觸發當下 `DX=0`，走整段複製。

### 1.3 描述元（以快取為準）

| 選擇子 | 範本（`gdt_decoded.txt`） | 執行時改寫 | 等級 |
|---|---|---|---|
| `98h` | 碼段 `9Ah`、D=1、G=0、界限 `FFFFFh` | 基底由 `433E`、`4345` 寫成 `X+7780h`（`0778h<<4` 加 `[3EA9]`）；`img.dis` 沒有以直接位址寫 `[149Eh]`（旗標 byte）的指令 | 基底強推論（靜態）；D=1 強推論 |
| `110h` | 資料 `96h`（向下展開、可寫）、B=1、界限 0 | 基底由 `431D`、`4324` 寫成 `X`；B 位元由 `int15` SP 子功能改寫：`5CE9` 設、`5CFA` 在 `CX=0` 時清（437 §1） | 已證實（靜態） |
| `28h` | 資料 `92h`、G=1、基底 0、4 GB | | 已證實（靜態、434 收據） |
| `80h` | 資料 `92h`、G=0、界限 `FFFFFh` | 基底由 `62FE`、`6305` 改寫（434 §1.1） | 已證實（靜態） |

**觸發當下 `110h` 的 B 是 0（已證實）**：434 讓分段模式的每個 `push` 先經 `stackCheck`，B=1 時失敗即關閉（`segmented.go:137-142`）。
`5B6A mov ss,110h` 之後的六個 push（`5B6C`–`5B8D`）都通過，停點在其後的 `5B90`，所以當時 `cache[SS]` 的 B=0。這與 437 收據第 1971307 步的傾印
（`110h` B=0）及 SP 子功能的 `CX=0` 清除一致。擋住這次觸發的是 D=1。B=1 只在沒有呼叫過 `CX=0` 的 SP 子功能時成立，本遊戲目前沒有這種證據，
所以 B=1 堆疊留給後續規格（§4.2 第 6 項）。本規格的 D、基底一律以執行時載入的快取為準，不以範本推定。

### 1.4 中斷

`5B86 sti` 之後才進 `98h`，所以 CU 期間 IF=1。434 §5.3 與 435 §3 規定分段模式（ring 0）的 IRQ 延後，不經 IDT 遞送；435 的 IDT 遞送只處理
V86 → ring 0。JEMM 為 ring 0 期間的中斷寫了專用路徑（已證實，靜態）：`4C2C` 比較中斷前的 `SS` 是否為 `110h`，`4C3F` 起在 `[3F54]≠0`
（CU、SP 進行中）時走 `4D24`–`4D66`，把 V86 框架改成 `AX=1209h`、`[bx+10h]=5243h`、`IP−2`，讓呼叫者在中斷處理完後重新發出 API。
本規格維持延後（§2.7），這條路徑不會被執行。

## 2. 規則

### 2.1 名詞

- **D=1 碼段**：`cache[CS].Attr` 的 bit 14 為 1。
- **本道的預設大小**：`Step` 開始時，`segmented=true` 且 `cache[CS]` 的 D=1 則為 32，否則為 16。指令執行途中 CS 改變（遠轉移）不影響本道，
  只影響下一道。核心不另存衍生欄位，每道指令開頭由快取推導。
- 匯出唯讀方法 `CPU.Code32() bool`，回 `segmented && cache[CS].Attr&attrDB != 0`，供 machine 層測試與觀測使用。
- D=1 只可能在 `Model80386`、profile 開啟、`PE=1` 時出現（442 子狀態的進入條件見 §2.9）。

### 2.2 前綴與預設大小

照 DOSBox-X `core_normal.cpp:167-170`（每道指令開頭 `opcode_index＝code.big×200h`、`prefixes＝code.big`、`ea_table` 依 `code.big`）：

- `Step` 開頭照舊把 `operand32`、`addr32`、`ext386` 設為 false，接著在 `if c.segmented` 分支內，`cache[CS]` 的 D=1 時三者設為 true。
  實模式熱路徑只多一個布林判斷（`segmented`），依 431 §7 量 `BenchmarkStepRealMode`（§3 第 15 項）。
- 前綴 `66` 設 `operand32 = !預設`，`67` 設 `addr32 = !預設`，`預設` 在前綴處以同一條件重算（前綴解碼期間 CS 不會改變）。照 DOSBox-X
  `prefix_none.h:347` 的 `opcode_index=(code.big^1)×200h` 與 `core_normal.cpp:84-86` 的 `DO_PREFIX_ADDR`：是「設成相反」，不是翻轉，
  所以 `66 66` 與單一 `66` 相同，與 Intel 一致。`ext386` 照舊設 true。
- D=1 下 `ext386` 恆為 true，每道指令先經 `execute386`：§2.4 的相對轉移與計數迴圈先處理；`addr32` 時 A0–A3 走 `moffs32`（disp32），字串指令走 §2.5；
  `operand32` 且在 `sizeSensitive` 內走 `execute32`；其餘落到 `executeCore`。`66` 使 `operand32=false` 時 `sizeSensitive` 的指令回到 16 位元的
  `executeCore`；`67` 使 `addr32=false` 時 ModRM 走 16 位元 `decodeModRM`。

### 2.3 EIP

- 核心維持 16 位元 `IP`，D=1 碼段內 EIP 的高 16 位元恆為 0（不變式）。
- 守住不變式的位置（違反時失敗即關閉，診斷含「EIP 超過 FFFFh，docs/spec/443 §2.3」）：
  1. 取指令：D=1 時 `segFetch8` 在 `IP=FFFFh` 取 byte 失敗（下一道的 EIP 會是 `10000h`）。刻意從嚴：最後一個 byte 在 `FFFFh` 的跳躍也被擋下。
     D=0 碼段不做這項檢查（16 位元環繞照舊）。
  2. 相對轉移：運算元大小為 32 時目標以 `uint32(IP)＋符號延伸的位移` 計算（DOSBox-X `JumpCond32_b`、`JumpCond32_d`，`core_normal/support.h:76-88`；
     `EB` 見 `prefix_66.h:652-657`），大於 `FFFFh` 失敗。
  3. 遠轉移與返回：目標偏移高 16 位元非 0 失敗，沿用 437 的 `farOffOK`（`ops386.go:422-428`）。
- 失敗時 `IP` 不被設為越界的跳躍目標；`IP` 的其他值（例如已越過本道指令的位元組）依 434 §4.1 只供診斷，錯誤回報 `opCS:opIP`。
- 運算元大小為 16 的相對轉移（D=1 下帶 `66`）照 Intel 截成 16 位元，即 16 位元環繞；在不變式下 EIP 高半仍為 0。

### 2.4 D=1 下的指令

**相對轉移與計數迴圈**：D=1 時，`70`–`7F`、`EB`、`E0`–`E3` 不論 `operand32`、`addr32` 為何，一律在 `execute386` 開頭進入本規格的處理，
不依賴 `addr32` 的 `implicitAddr` 分派：

- 目標寬度看 `operand32`：32 依 §2.3 第 2 點；16 則 16 位元環繞。
- `E0`–`E3` 的計數暫存器看 `addr32`：ECX（32 位元遞減、判斷含高半）或 CX（只動低 16 位元）。`E3` 判 `ECX==0` 或 `CX==0`。
- 條件判斷沿用 `c.cond`。DOSBox-X：`prefix_66.h:221-252`（`70`–`7F`）、`584-605`（`E0`–`E3`）、`prefix_none.h:1218-1239`（16 位元運算元形式），
  `AddrMaskTable`＝`core_normal.cpp:101`。
- `0F 80`–`8F`：既有 `jccNear` 在 `operand32` 時已做 rel32 與 `> FFFFh` 檢查（`ops0f.go:202-214`），只改診斷文字。

**字串指令**：`addr32` 時 `A4`–`A7`、`AA`–`AF` 走 §2.5；`addr32=false`（D=1 帶 `67`）走現行 `stringOp`／`stringOp32`。

**其餘指令**：照現有派送。`execute32` 的 `default` 一律失敗即關閉，所以 D=1 下（運算元大小 32）下列形狀失敗即關閉，`98h` 段都沒有：

| 類別 | opcode |
|---|---|
| 近呼叫、近跳躍、近返回 | `E8`、`E9`、`C2`、`C3`、`FF /2`、`FF /4` |
| 堆疊框 | `C8`（`enter`）、`C9`（`leave`） |
| 32 位元 I/O | `E5`、`E7`、`ED`、`EF`、`6D`、`6F` |
| 遠指標載入 | `C4`、`C5`（`les`／`lds` r32） |
| 其他 | `62`（`bound` r32） |
| `addr32` 下的隱含位址 | `6C`–`6F`（`ins`／`outs`）、`D7`（`xlat`） |

已實作、D=1 下可用的 `execute32` 形狀（HEAD `ops386.go:140-415`）：`00`–`3D` 的 ALU、段暫存器 push／pop、`40`–`61`、`68`、`69`、`6A`、`6B`、`81`、
`83`、`85`、`87`、`89`、`8B`、`8C`、`8D`、`8F`、`90`–`99`、`9C`、`9D`、`A1`、`A3`、`A5`、`A7`、`A9`、`AB`、`AD`、`AF`、`B8`–`BF`、`C1`、`C7`、`CF`、`D1`、`D3`、
`F7`、`FF /0 /1 /3 /5 /6`，以及分段模式的 `9A`、`EA`、`CA`、`CB`（437）。

**D=1 下運算元大小 16 的形式**：`66 E8`、`66 E9`、`66 C2`、`66 C3`、`66 EB`、`66 70`–`7F` 等 `operand32=false` 的形式以 16 位元 `IP` 執行
（`E8`／`E9`／`C2`／`C3` 走 `executeCore`，推入與彈出 16 位元返回位址、目標 16 位元環繞；`EB`、`70`–`7F` 經上面的處理，結果相同）。這是刻意的結果：
運算元大小 16 時 Intel 把 EIP 截成 16 位元，在不變式下與 `executeCore` 相同。`66 E0`–`E3` 的計數暫存器看 `addr32`，D=1 下為 ECX，`executeCore`
只看 CX，所以必須經上面的處理，不得落到 `executeCore`。

**不支援的形狀**：D=1 下 `66 8F /0`（`pop r/m16`）的 ModRM32 以 ESP 為基底時失敗即關閉。Intel 以彈出後的 ESP 計算位址，`executeCore` 的 `8F`
先解位址後彈出（`ops.go:264-266`）；`execute32` 的 `8F` 已依 Intel 順序（`ops386.go:280-284`）。

**`CF`**：D=1 下不帶 `66` 是 `iretd`，照 435 §2.5、436 §3.1 現行語意（NT=1 任務返回，否則 CPL 0 回 V86）；帶 `66` 的 16 位元 `iret` 照 434 失敗即關閉。

D=0 碼段（含實模式、V86）帶 `66` 的 `70`–`7F`、帶 `67` 的隱含位址指令維持現況（失敗即關閉），本規格只在 D=1 時開放。

### 2.5 32 位元定址的字串指令

`addr32` 且 D=1 時：

- 位址暫存器為 ESI、EDI，計數為 ECX，三者都以 32 位元遞增或遞減（DF），不截成 16 位元。偏移以 32 位元交給 `segRead`／`segWrite`
  （`w32=true`，各 byte 32 位元相加）。來源段可被前綴覆寫，目的段恆為 ES（`string.go:5-13` 的規則不變）。
- 運算元寬度：byte（`A4`、`A6`、`AA`、`AC`、`AE`）、word（D=1 帶 `66`）、dword。
- `rep`：先看 ECX，為 0 時不執行；`repe`／`repne` 只對 `cmps`、`scas` 判 ZF，每次迭代先減 ECX 再判 ZF，與 `stringOp` 相同。
- 週期：每次迭代 `charge(cycString)`；`DOSBoxCost` 時每次迭代另 `Cycles++`，與 16 位元 `stringOnce`（`string.go:51-53`）一致。既有的
  `stringOnce32`（`ops386.go:533-537`）沒有這一項，本規格不修，列為已知不一致（§4.1）。
- **一步的上限**：單道 `rep` 在一次 `Step` 內最多執行 2^20（1048576）次迭代。達到上限而 ECX 仍非 0、且 `repe`／`repne` 的條件仍要求繼續時：
  已完成的迭代已提交，ECX、ESI、EDI 為剩餘值，`IP` 設回 `opIP`（含前綴），`Step` 正常回傳；下一步重新解碼同一道指令續跑。上限以下一道一步，
  與現行 16 位元 `rep` 的步數語意相同（`string.go:39-46`）。DOSBox-X 依 `CPU_Cycles` 與 `cpu_rep_max` 切段並以 `LOADIP` 重新開始
  （`string.h:60-67`、`535-560`），切段點不同屬刻意差異。分段模式的 IRQ 延後（§2.7），續跑之間不會有中斷遞送。
- **失敗**：本道指令的 `c.fault` 非 nil 時（界限、型別違反），該次迭代不寫入、不減 ECX、ESI 與 EDI 不前進，迴圈結束，`Step` 回報錯誤。
  `cmps`、`scas` 的讀取失敗時不更新旗標。之前的迭代已提交，與 434 §4.1「錯誤回報後狀態只供診斷」一致。現行 16 位元 `rep` 在失敗後仍跑完剩餘迭代
  （`stringOp` 的迴圈不看 `c.fault`），本規格不改它。
- 分頁錯誤紀錄（432 §10 第 1 項）由 Machine 在指令結束後回報，迴圈不因此提前結束，與現行 `rep` 相同。
- 實作可把每次迭代的 byte 檢查合併成一次區間檢查，前提是失敗點與上一條相同。不強制。

### 2.6 堆疊

- SS 的 B=1 維持 434 §4.1 的失敗即關閉，`stackCheck`（`segmented.go:137-142`）不改。D=1 碼段只在 B=0 的堆疊上執行。
- 推入與彈出的寬度看運算元大小，堆疊指標仍是 SP（16 位元環繞）。D=1 下不帶 `66` 的 `push`／`pop` 走 `push32`／`pop32`；DOSBox-X 在 B=0 時
  同樣以 `stack.mask=FFFFh` 計算（`cpu.cpp:675-697`）。
- 遠返回的框架寬度看運算元大小，不看目標段的 D：D=1 碼段的 `CB` 彈出 32 位元 EIP 與 32 位元 CS（`farRet32`），帶 `66` 時彈出兩個 word
  （DOSBox-X `CPU_RET(use32,…)`，`cpu.cpp:2290-2318`）。JEMM 以四個 word 疊出的框架即對應 32 位元形式（§1.1）。

### 2.7 進入與離開 D=1、中斷

- `loadSegPM` 的 CS 分支刪除 D=1 的拒絕（`segmented.go:249-251`）；其餘 434 §5.1 的 CS 檢查不變。可以載入 D=1 CS 的轉移：16 位元與 32 位元的遠跳
  （`EA`、`66 EA`、`FF /5`、`66 FF /5`）、遠呼叫（`9A`、`66 9A`、`FF /3`、`66 FF /3`）、遠返回（`CB`、`CA`、`66 CB`、`66 CA`）。反向（D=1 → D=0）同一組。
  `loadSeg(CS)` 失敗時段值與快取不寫（434 §5.1 第 4 步），下一道的預設大小不變。
- 仍拒絕 D=1 的位置：435 §2.3 第 3 步 IDT 閘目標碼段 D=1（`v86deliver.go:149`，不改）；436 的任務返回只進 V86，不載入保護模式 CS，不受影響。
- **中斷維持 434 §5.3、435 §3 的延後**：分段模式下（不論 D）IRQ0 與按鍵只累積不遞送、不消耗；軟體中斷與例外失敗即關閉。CU 的 IF=1 視窗是
  `5B86 sti` 到 `5B95 cli`，`rep` 一步跑完，整段在數十步（整段複製）到十幾萬步（差分複製最壞每個 dword 約 9 道）之間，低於 434 §5.3 的
  一百萬步條件。延後的 IRQ 在回到 V86 後經 435 遞送。風險見 §4.1。
- 435 不改。ring 0 的同特權 IDT 遞送與彈出 `VM=0` 的 `iretd` 不在範圍（§2.11）。

### 2.8 狀態檔

- 不升版本、不加欄位：D=1 由 `PM` 版本 3 既有的 `cache[CS]` 的 `Attr` bit 14 表示（434 §2.2 的位移 82 起）；EIP 高半依 §2.3 恆為 0。
  gob 位元組錨點不變（431 §4.2）。
- `decodePMV3`（HEAD `internal/machine/pmstate.go:302-307`）的「`cache[CS]`、`cache[SS]` 的 D/B 為 0」改為：`cache[SS]` 的 B 恆為 0；`cache[CS]` 的 D
  在 `PE=0`（442 子狀態）時為 0，`PE=1` 時可為 1。其餘驗證（`segmented`、保留 byte、`Attr` 的 bit 8–11、13，442 的 `PE`／`PG` 條件）不變。
- 本規格之前沒有程式會寫出 CS 的 D=1（`loadSegPM` 拒絕），放寬不改變任何既有存檔的意義；舊版程式讀到新存檔時以 D/B 檢查拒絕，屬失敗即關閉。
  442 §2.11 記下的「SS 可載入 B=1 但存檔讀不回」維持原狀，隨 B=1 堆疊留給後續規格。
- `applyPM`（`pmstate.go:313-327`）經 `SetSegCache` 還原，預設大小由快取推導，不另加步驟。
- profile 關閉時 `PM` 為 nil，`StateDigest` 與錨點測試常數不變。

### 2.9 與 442 的合併點

442 第 2 版（READY）的實作在工作樹中未提交，與本規格改同一組程式：

| 位置 | 442（工作樹） | 443 | 合併規則 |
|---|---|---|---|
| `Step`（`ops.go`） | 失敗分支清 `leavePending`；成功時看旗標回 `ErrLeaveSegmented` | 開頭在 `segmented` 分支設 32 位元預設；前綴 `66`／`67` 改為「設成相反」 | 兩者都保留，互不影響 |
| `farJmp32`、`farCall32`、`farRet32`（`ops386.go`） | 頂端呼叫 `far32RealCache`，子狀態失敗即關閉 | 函式本體不改；D=1 下不帶 `66` 的 `EA`／`9A`／`CB`／`CA` 也會進來 | 保留 `far32RealCache` 在頂端 |
| `iretRealCache`（`ops.go`） | 呼叫 `stackCheck` | `stackCheck` 不改（B=1 移出範圍） | 無衝突 |
| `loadSeg`、`loadSegReal`（`segmented.go`） | 子狀態走實模式分支，CS／SS 清 D/B | `loadSegPM` 不再拒絕 CS 的 D=1 | 互不影響；預設大小由快取推導，實模式載入清 D 後自然回到 16 |
| `SetSegmented`、`SetSegCache`（`segcache.go`） | 清 `leavePending` | 不改（不維護衍生欄位） | 無衝突 |
| `Machine.SetCR0`（442 §2.2 的表） | 分段模式清 PE 且清 PG 進入子狀態 | 新增一列：清 PE 時 `cache[CS]` 的 D=1，回錯誤、`CR0` 不改（失敗即關閉） | 後實作者補上這一列。理由：子狀態第一道指令仍依快取解碼，D=1 會以 32 位元解讀實模式碼；442 §2.7 的離開條件也要求 CS 的 D=0 |
| `decodePMV3`（`pmstate.go`） | `segmented=1` 允許 `PE=0 ∧ PG=0` | CS 的 D=1 只在 `PE=1` 時接受 | 取交集；後實作者保留另一份的放寬 |
| `66 CF`、`66` 遠轉移（`ops386.go`） | 子狀態失敗即關閉 | 子狀態下 CS 的 D=0（上列 `SetCR0` 保證），D=1 的預設不會在子狀態出現 | 無衝突 |

第 1 版草稿要取消 `stackCheck` 的 B=1 檢查，會與 442 的 `iretRealCache` 衝突；B=1 移出範圍後衝突消失。

### 2.10 要改的程式與測試

| 位置 | 改動 |
|---|---|
| `internal/cpu/ops.go` | `Step`（`ops.go:14-102`）的預設與 `66`／`67`；`executeCore` 的 `8F` 在 D=1、`66`、ModRM32 以 ESP 為基底時失敗即關閉 |
| `internal/cpu/ops386.go` | `execute386` 開頭的 D=1 相對轉移與 `E0`–`E3` 分派；隱含位址分派（D=1 時進 §2.5，否則照舊失敗） |
| `internal/cpu/string.go` 或新檔 | §2.5 的 32 位元定址字串指令與一步上限 |
| `internal/cpu/ops0f.go` | `jccNear` 在 D=1 的診斷文字 |
| `internal/cpu/segmented.go` | `segFetch8` 的 D=1 `IP=FFFFh` 檢查；`loadSegPM` 刪 D=1 拒絕 |
| `internal/cpu/segcache.go` | `Code32()`；檔頭註解 |
| `internal/machine/pmstate.go` | `decodePMV3` 的 D 條件；檔頭註解 |

既有測試的連動（行號依 HEAD）：

| 測試 | 改動 |
|---|---|
| `internal/machine/segmode_test.go:298-309` `TestSegStack` | SS 的 B=1 失敗即關閉維持，不改 |
| `internal/machine/segmode_test.go:394` 的「CS D=1」列 | 期望由失敗改為成功（D=1 可載入） |
| `internal/machine/seginsn437_test.go:217-226` | 「jmp D=1 碼段」「call D=1 碼段」「ret D=1 碼段」三列改為成功例或移到 443 的測試；「call SS B=1」「ret SS B=1」維持失敗 |
| `internal/machine/segstate_test.go` | CS 的 D=1：`PE=1` 接受、`PE=0` 被拒；SS 的 B=1 照舊被拒 |
| `internal/cpu` 中期待「CS 的 D=1 失敗」的測試 | 以 grep `D=1`、`attrDB` 找出後改為本規格的期望 |

`internal/cpu386`、`internal/state`、LE 與 DPMI 路徑、`oracle`、`session`、435 的 `v86deliver.go` 不改。

### 2.11 不在範圍

- SS 的 B=1 堆疊（含 B=1 下的 push／pop、遠轉移、`ret imm16`、`enter`／`leave`、`iretd`、`SS0` 的 B=1）。
- ring 0 的同特權中斷與例外遞送、彈出 `VM=0` 的 `iretd`、JEMM `4C3F`／`4D24` 的 CU 重新發出路徑。CPU 例外的遞送屬 S6。
- EIP 高 16 位元非 0；§2.4 表列的失敗即關閉形狀。
- D=0 碼段的 `66` 相對轉移與 `67` 隱含位址指令。
- CPL 3 的 D=1 碼段、一致碼段、呼叫閘、任務閘；V86 與 D=1 交錯（V86 由 435 從 D=0 的 `18h` 進出）；442 子狀態下的 D=1（§2.9 擋下）。
- cpu386 與 DOS4GW／LE 路徑。

## 3. 驗收

1. profile 關閉時：錨點測試常數與 gob 位元組不變；`StateDigest` 不變；`internal/cpu386`、`internal/state` 與 LE 路徑 `git diff` 為空。
2. SingleStepTests 以掛載的語料實際執行、315 個子測試全過、0 skip，`knownGaps` 不變（掛載方式同 433 §8.2）。
3. 合成程式重現 JEMM CU（machine 層測試，不依賴原版檔）：
   - 組態：profile 開啟、分段模式、A20 開，延伸記憶體容納 `X＝110000h` 起的映像與四個 64 KB 緩衝區。GDT 依 §1.3：`18h`（D=0，基底 `X`）、
     `98h`（D=1，基底 `X+7780h`）、`110h`（向下展開、B=0、界限 0、基底 `X`）、`50h`（向下展開、B=0、界限 0、基底 `X`）、`28h`（G=1 平坦）、
     `80h`（G=0、界限 `FFFFFh`、基底取非 0 的值 `G`）。`18h` 段放 `5B5C`–`5B9D`、`98h` 段放 `0000`–`008F`，逐位元組照 `img.dis` 第 3408–3434 行與
     `r2.lst` 第 3921–3977 行。`G+12Ch` 放緩衝區基底 `B`；ES:BX 的框架放 ECX（CL、CH）與 EDX。
   - 四個資料變體：
     - `DX=0`（整段複製，同觸發案例的 `CX=0200h`）；
     - `DX≠0`、`CH≠0`（整段複製）；
     - `DX≠0`、`CH=0`，來源與比較緩衝區在第一個、最後一個、連續一串、單一個 dword 不同，目的預填哨兵（差分複製）；
     - `DX≠0`、`CH=0`，只有最後一個 dword 不同（`repz cmpsd` 以 ECX=0、ZF=0 結束）。

     期望值以測試內獨立的參考模型計算（整段複製：目的＝來源；差分：來源 ≠ 比較時目的＝來源，否則保持哨兵），不用被測核心的輸出。
   - 期望：`5B90` 之後 `CS=0098h`、`Code32()=true`、`IP=0015h`；`98:0015` 的 `65 67` 讀 `G+12Ch`；`98:008F retf` 之後 `CS=0018h`、`IP=5B95h`、
     `Code32()=false`；返回後 SP 等於四個 push 之前的值；`AX=0`、EAX 高半保留；`5B9D pop ecx` 之後 ECX 等於 `5B5C` 的舊 ESP。
   - 另一例 `110h` 的 B=1：`5B6C push es` 失敗即關閉（B=1 移出範圍的確認）。
   - `5E60`–`5E6A` 與 `98:0000`–`0014` 的複製常式：`ECX` 為 0、1、3、4、5、`10001h` 各一例，比對目的內容與 ESI、EDI、ECX 的終值。
4. 前綴與解碼：D=1 下無前綴、`66`、`67`、`66 67` 各以 `mov`、ModRM 記憶體運算元驗運算元與定址寬度；`66 66` 與單一 `66` 相同；`67 E3` 判 CX；
   `E3` 判 ECX（ECX=`10000h` 時不跳）；`E2` 以 ECX 遞減；`66 E3` 判 ECX、目標 16 位元環繞；`67 F3 A5` 用 SI／DI／CX 搬 dword；`0F 84` 取 rel32。
   D=0 碼段與實模式的同一批位元組行為不變（`66 74` 與 `67 A4` 仍失敗即關閉）。指令中途遠跳到 D=1 不改變本道的預設。
5. 鏡像等價：同一段指令位元組，在（D=0，前綴集合 P）與（D=1，`66`／`67` 與 P 相反）兩種條件下執行，暫存器、旗標與記憶體逐位元組相同。
   `66` 重用 `ops386_wide_test.go` 的語料；`67` 以 ModRM32、`moffs32` 與 SIB 形式做同樣的語料。兩邊都限於本規格開放的形狀。
6. EIP：`EB`、`74`、`E2`、`67 E3`、`67 E2`、`0F 84` 的目標大於 `FFFFh`（正向越界與負向環繞各一）失敗，且 `IP` 不等於越界目標；D=1 下在
   `IP=FFFFh` 取指令失敗；D=0 分段模式在 `IP=FFFFh` 取指令照常環繞；`66 EB` 的 16 位元環繞照舊。
7. 運算元大小 16 的形式：D=1 下 `66 E8` 推入 16 位元返回位址並跳到 16 位元環繞的目標，`66 C3` 彈出 16 位元 `IP`，各一例。D=1 下 `66 8F /0` 以
   ESP 為基底的 ModRM32 失敗即關閉。
8. 字串：`A4`–`A7`、`AA`–`AF` 在 32 位元定址下各一例（DF=0、DF=1；ECX 高半非 0；來源段前綴；目的段不可覆寫）；`rep` 的 ECX=0；`repe`／`repne`
   的提前結束；第 k 次迭代超出 ES 界限、與第 k 次迭代來源超出 DS 界限各一例：前 k−1 次已寫，第 k 次未寫，ESI、EDI、ECX 停在第 k 次之前；
   `cmps` 讀失敗時旗標不變；`DOSBoxCost` 開啟時 `Cycles` 每次迭代加 1。
9. 一步上限：`rep movsb` 以 ECX=`100001h` 執行，第一步後 ECX=1、ESI／EDI 前進 2^20、`IP=opIP`（含 `F3` 前綴）、`Steps` 加 1；第二步完成，ECX=0、
   `IP` 為下一道；`repe cmpsb` 在上限處條件已不成立時不續跑；ECX ≤ 2^20 時一道一步。
10. 進出 D=1：`EA`、`66 EA`、`9A`、`66 9A`、`FF /3`、`FF /5`（16 與 32 位元）載入 D=1 CS；D=1 的 `CB`（32 位元框架）、`66 CB`（16 位元框架）回 D=0；
    D=0 的 `CB` 返回 D=1；每次轉移後 `Code32()` 正確；`loadSeg(CS)` 失敗時 CS、快取與下一道的預設大小不變。
11. 中斷：D=1 碼段、IF=1、有待處理 IRQ0 與按鍵時不遞送、不消耗（`Ticks`、`irq0Pending`、鍵盤佇列不變），`segWait` 照常計數；回到 D=0 並
    `cli` 後仍保留；IF=1 且待處理一百萬步後失敗即關閉。D=1 下 `INT n`、`div 0`、`hlt` 失敗即關閉。
12. 失敗即關閉（各一例）：§2.4 表內每個 opcode；IDT 閘目標為 D=1（435 不變）；442 實作後：分段模式清 PE 時 `cache[CS]` 的 D=1。
13. 狀態檔：第 3 項的差分變體在 `98h` 碼內（`repz cmpsd` 之前）存一次，`SaveState`／`LoadState` 與 `Snapshot`／`Restore` 往返後跑到 `5B9D`，再多 100 步，
    最終 `SaveState` 位元組與不中斷的執行相同；`encodePM` 仍為版本 3、142 bytes；手造版本 3 的 `PE=1` 且 CS D=1 可讀；`PE=0` 且 CS D=1 被拒
    （442 實作後）；SS 的 B=1 照舊被拒；手造版本 1、2 的既有測試不變。
14. 突變自檢（各至少一個測試失敗）：
    - `Step` 開頭的 D=1 預設：`ext386`、`addr32`、`operand32` 各自單獨不設（三個突變）；`66` 翻轉而非設定（`66 66` 測試）；`67` 不改 `addr32`；
    - `loadSeg(CS)` 失敗後仍改變下一道的預設大小；`retf` 框架寬度依目標段的 D 而非運算元大小；
    - 32 位元定址字串用 SI／DI；`rep` 用 CX；`repz cmpsd` 判 ZF 之前未減 ECX；來源讀失敗仍寫目的；字串失敗後仍繼續迭代；一步上限不設 `IP=opIP`；
    - `jecxz` 判 CX；`loop` 遞減 CX；`67 E3` 的目標以 16 位元環繞；32 位元相對轉移以 16 位元環繞；`66 E3` 判 CX；
    - 略過 D=1 的 `IP=FFFFh` 取指令檢查；`IP=FFFFh` 檢查在 D=0 也生效；
    - `decodePMV3` 仍拒絕 `PE=1` 的 CS D=1；`decodePMV3` 接受 `PE=0` 的 CS D=1；`decodePMV3` 接受 SS 的 B=1。
15. 效能：
    - profile 關閉的 `BenchmarkStepRealMode` 與本規格動工前的 commit 交錯比較（431 §7，不低於 97%），照實記錄 load average；主機不安靜時與
      432–442 同列開放。
    - 新增 `BenchmarkSegmentedRepMovsd32`（分段模式、分頁開、D=1、`rep movsd` 16000 個 dword）。驗收門檻：單次 CU 整段複製在容器內 5 ms 以下。
      收據記錄每次迭代的 ns。§2.5 的區間檢查快速路徑可選。
16. `go build ./...`、`go vet ./internal/... ./cmd/probe`、`go test ./internal/... ./cmd/probe ./session ./oracle` 全綠，SingleStepTests 不得 skip；
    `go test ./apps/...` 與改動前逐行相同（437 收據記錄的唯一既有 FAIL 為 GLFW）。
17. 觸發案例（需原版，主代理另附 probe 收據）：從 `strike_commander/workplace/scratch/p452/s1900.state` 接著跑，第 19010000000 步點 (160,90)，
    帶 `-exec-env -exec-reclaim`，收據寫明完整旗標。記錄：
    - 到達 `0018:5B90` 的步數；當下 `cache[SS]` 的 `Attr`、`cache[CS]` 於 `jmp 98:15` 之後的基底、界限、`Attr`；
    - `[GS:12Ch]` 的值、框架內的 ECX（CL、CH）與 EDX、走哪一條分支（§1.2）；
    - 返回 `0018:5B95` 的步數、CU 期間的步數、期間 `irq0Pending` 與 `segWait` 計數；
    - tick 損失量：CU 期間跨過的 IRQ0 週期數（依時鐘模式由 `Cycles` 或步數換算）減去回 V86 後實際遞送的 IRQ0 數；
    - 之後 CU 是否重複發生（次數、間隔步數、每次的分支）；STRIKE.EXE 的後續停點、原因、總步數、視訊模式變化；
    - 停在別處時照實記錄停點與原因，不改程式湊結果。
18. DOSBox-X 交叉對照（主代理收據項）：以除錯器橋接在 `98:008C` 下中斷點，比對 CU 四個變體（第 3 項的分類）的 ESI、EDI、EBX、ECX、EDX、EFLAGS
    與 dosgolem 相同。遊戲自然呼叫中沒有出現的變體，或橋接無法在 D=1 碼段下中斷點時，寫明原因與缺的變體。

## 4. 未知與已決定事項

### 4.1 未知與風險

| 項目 | 現況 |
|---|---|
| 執行時 `98h` 的基底與 D | 基底 `X+7780h` 為強推論（`433E`、`4345`）；沒有找到以直接位址改寫旗標的指令，D=1 為強推論。觸發案例記錄快取 |
| B=1 堆疊 | 觸發當下 `110h` 為 B=0（已證實，§1.3）；本遊戲目前沒有 B=1 的證據。沒有先呼叫 `CX=0` 的 SP 就呼叫 CU 的執行歷史會遇到 B=1，屆時失敗即關閉並另立規格 |
| CU 的用途與參數語意（CL、CH、DX、`[GS:12Ch]`） | §1.2 的資料流為強推論；`FA00h` 對應 320×200，推測為畫面緩衝的整段或差分上傳，未驗證 |
| 真機在 CU 中途收到 IRQ0 的結果 | JEMM `4D24`–`4D66` 讓呼叫者以 `AX=1209h`、`[bx+10h]=5243h` 重新發出，重新發出後的流程未解讀；延後後的結果是否與真機相同未驗證 |
| IRQ 延後的風險 | 分段模式下 IF=1 且有待處理 IRQ 連續一百萬步即失敗即關閉（434 §5.3）；CU 估計最多十幾萬步，但重複呼叫或其他長 ring 0 路徑可能逼近。`irq0Pending` 是 bool（`internal/machine/machine.go:1595-1607`），CU 期間跨過多個週期只留一個：指令數時鐘丟掉多餘週期，週期時鐘加 `DOSBoxCost` 以絕對排程追趕。觸發案例收據記錄 tick 損失量 |
| `stringOnce32` 的 `DOSBoxCost` 計數 | 既有 `66` 字串（`ops386.go:533-537`）沒有每次迭代 `Cycles++`，與 16 位元 `stringOnce` 及本規格的新路徑不一致；本規格不修 |
| `5E6A`（XC）與 `7117` 的呼叫者 | 未知 |
| Intel 386 在 D=1 下 `IP` 由 `FFFFh` 前進的行為 | 本規格未讀 SDM；§2.3 從嚴 |

### 4.2 已決定（2026-10-16，主代理依審查定案）

1. **CU 期間的 IRQ 維持延後**（§2.7）。與 434 §5.3、435 §3 一致，不需要 ring 0 同特權遞送、`iretd` 回 D=1，也不會執行 JEMM 的重新發出路徑。
   CU 的 IF=1 視窗遠小於一百萬步上限；時間點的差異與 tick 損失由觸發案例收據量化。
2. **16 位元 `IP` 加不變式**（§2.3）。`98h` 段的偏移都小於 `90h`；加 EIP 高半要改狀態檔、診斷格式、`Op()`、`Rewind`、事件框架與觀測，換不到
   JEMM 路徑上的行為。
3. **只開放盤點到的指令形狀**（§2.4），加上同一實作自然涵蓋的字串指令與 `loop` 系列。其餘照 197 的範圍規則失敗即關閉，用到時再補，避免沒有
   原版證據的語意。
4. **`rep` 一步跑完，上限 2^20 次迭代**（§2.5）。上限以下與現行步數語意相同；上限防止 ECX 高半造成單步執行數十億次迭代，超過時以 `opIP`
   續跑，暫存器保持一致。
5. **狀態檔不升版本**（§2.8）。D=1 由版本 3 既有欄位表示；放寬驗證不改變任何既有存檔的意義，舊版程式讀到時失敗即關閉。理由同 442 §4.2 第 1 項。
6. **B=1 堆疊移出本規格**。觸發當下 `110h` 為 B=0（已證實），本遊戲目前沒有 B=1 的證據；維持 434 的失敗即關閉，`stackCheck` 不動，也避開與 442
   實作（`iretRealCache`、`far32RealCache`）的衝突。出現證據時另立規格。
7. **不維護衍生欄位**（§2.1）。預設大小每道指令由 `cache[CS]` 推導，實模式熱路徑只多一個布林判斷，也省去所有快取寫入點的同步與不變式測試。

## 5. 實作收據（2026-10-17）

commit：`a726ec2`。

- §3 單元測試：容器內 `go test -count=1 ./internal/... ./cmd/probe ./session ./oracle` 13 個套件 ok；SingleStepTests 316 PASS、0 SKIP；`gofmt`、`go vet` 無輸出；
  新增 `seg443_test.go`、`seg443_string_test.go`、`seg443_mirror_test.go`（鏡像等價約 95 組）、`seg443_bench_test.go`。突變 23 條各至少一個測試失敗。
  `internal/cpu386`、`internal/state` 無改動，錨點不變。`go test -v ./apps/...` 改動前後排序後逐行相同。
- 既有測試連動：442 的 `TestRC442LoadSites` 原以注入 CS D=1 驗「子狀態清 CS 的 D」，該狀態在 443 下不可達（§2.9），CS 六列改為 D=0；清 CS D 改由
  `internal/cpu` 的 `TestRealCacheLoadSegClearsCSD` 直接呼叫 `loadSegReal` 驗證（負對照：拿掉清 CS D 時該測試失敗）。
- 效能（load average 約 9）：`BenchmarkStepRealMode` 改動後為改動前的 101.7%；`BenchmarkSegmentedRepMovsd32` 單次 CU 1.69–3.93 ms（含區間檢查快速路徑）。
- 觸發案例（`strike_commander` 的 `workplace/logs/probe453.log`）：從 `p452/s1900.state` 接著跑，`-exec-env -exec-reclaim`，第 19010000000 步點 (160,90)。
  STRIKE.EXE 經 JEMM `CU` 進出 `98h` 碼段，跑滿 21000000001 步仍在執行，沒有未實作的停點；第 193 億步與第 210 億步的截圖是 F-16 駕駛艙飛行畫面（HUD、MFD 正確）。
  本段平均 4.4 M 道／秒，遊戲內時間比真機慢約 2.13 倍。開檔：`optdflt.cfg`、`op2misn.iff`、`instant.iff`（建立）、`ARENA.PAK`。
- 未做：DOSBox-X 交叉對照 CU 四變體；IRQ 延後造成的 tick 損失量（probe 尚無計數）；`110h` 的 SS B 值未記錄（停點前沒有失敗，可推得為 B=0）。
