# 010 — 子程式結束時回收記憶體；planar write mode 3 與 set/reset

日期：2026-09-06
狀態：**READY**（§1 記憶體回收、§2 write mode 3、§3 mode 0 的 set/reset）
動機：`GIN3PS.EXE` 在第 ~2576 萬道指令把 Big5 選單文字寫進 IVT 而停機
（`~/cht/logh3/docs/re/05`）。根因不在圖形也不在中斷，而在 EXEC 之後
配置游標沒有退回，本體被載到太高的段，heap 要不到空間。
前置：[`007`](007-exec-ems.md) §2（EXEC 的 bump 配置器）、
[`009`](009-planar-vga.md) §4（write mode 3 當時列為「未見使用」）。

---

## 1. 子程式結束時回收記憶體（READY）

### 證據

| 事實 | 證據 | 等級 |
|---|---|---|
| `GIN3PS.EXE` 的 PSP 落在段 `0x3922` | probe 記憶體配置報告 `#23289375 AH=4A ES=3922` | confirmed（軌跡） |
| 它向 DOS 要 `0x6A07` 段被拒，退而拿 `0x66DD` | `#23305663 AH=4A BX=6A07 → 失敗（可用 66DD）` | confirmed |
| `0x3922 + 0x6A07 = 0xA329` 超過 `MemTop = 0x9FFF` | 算術 | confirmed |
| heap 拿不到空間後，malloc 從一個假區塊切出 `0000:0002` | 反組譯 `72C2:0047` ＋ 軌跡 | confirmed |
| `OPEN.EXE` 的 PSP 是 `0x0339`，結束後那塊沒有還回去 | `#120 AH=4A ES=0339`；`childExit` 不動 `freeSeg` | confirmed（程式碼） |

### 語意

真 DOS 的 `AH=4Ch` 會釋放終止中的 PSP 名下所有記憶體區塊。dosgolem 的
bump 配置器只有 `freeSeg` 一個游標，`childExit` 原本只還原 PSP、handle、
IVT 22h–24h 與暫存器，**游標留在高點**——等於子程式的記憶體永遠不還。

改為：`execFrame` 記下 EXEC 之前的 `freeSeg`，`childExit` 一併還原。

**回收的是所有權，不是內容。** 視訊記憶體、IVT、子程式寫過的資料都保留
（畫面不會因為程式結束而消失，符合真 DOS）；只有「下一次配置從哪裡開始」
退回去。

### 已知邊界

- 子程式若替父程式配置了要活過自己的記憶體，會被一起回收。真 DOS 也是
  這樣（那塊 MCB 的擁有者是子程式的 PSP），所以先照做；有反例再說。
- 這是單一游標的近似：真 DOS 能回收中間的洞，我們只能退回連續尾端。
  EXEC 鏈是嚴格巢狀時兩者等價，本作就是這種形狀。

### 1.1 arena 時代的補充：MCB 也要跟著消失（READY，2026-09-10）

上面那一節是 bump 配置器時代寫的，處理的只有 `freeSeg` 這一個游標。
換成 arena（`009-memory-allocator`）之後多了一件事：**arena 的每一塊都會把
一個假 MCB 標頭寫進客體記憶體**（`syncMCB`）。子程式結束時若只還游標、
不還它的區塊，那些標頭就留在原地。

留下來不是「多佔一點記憶體」而已。接手的程式向 DOS 把自己的區塊撐大蓋過
那一段之後，標頭落在它的**程式碼或資料**中間——`'M'`（`0x4D`）、擁有者、
大小、8 個空白的名稱欄，一共 16 個位元組，長得很像一般資料。

| 事實 | 證據 | 等級 |
|---|---|---|
| `OPEN.EXE` 用 `AH=48h` 拿走四塊後結束 | probe 配置器逐筆帳 | confirmed |
| `GIN3PS.EXE` 在同一個 PSP 載入，`AH=4Ah` 撐到 `0x7950` | 同上 | confirmed |
| `375CE` 的 `83 C4 24` 變成 `83 C4 4D` | 兩版記憶體傾印對照 | confirmed |
| `add sp,24h` → `add sp,4Dh`，每次收尾堆疊歪 41 個位元組 | 指令軌跡 | confirmed |

語意：**`AH=4Ch`（與 `int 20h`）要釋放終止中的 PSP 名下所有 MCB**，
就是真 DOS 的行為。`memBlock` 因此記 `owner`，`terminate` 的非 TSR 兩條路
呼叫 `releaseOwnedBy(curPSP)`，釋放後合併並重新發布 MCB 鏈。
TSR（`AH=31h`）不釋放——常駐程式的區塊本來就要留著。

`curPSP` 為 0 時（開機階段、測試直接呼叫配置器）擁有者退回
`machine.PSPSeg`：0 在 MCB 上代表「自由」，拿它當擁有者會讓
`releaseOwnedBy` 永遠找不到那一塊。

### 驗收

- 單元測試 `TestChildExitReclaimsMemory`。
- 單元測試 `TestChildBlocksAndTheirMCBsVanishOnExit`（§1.1）：子程式
  `AH=48h` 拿一塊後結束，arena 不得留下任何已配置的區塊。
  **反向對照做過**：拿掉 `releaseOwnedBy` 它會指名留下來的那一塊。
- 整合：原版 `GIN3.COM` 鏈的 `GIN3PS.EXE` PSP 從 `0x3922` 降到 `0x0339`，
  `AH=4A BX=6A07` 由失敗轉成功，本體跑過原停點（實測 6000 萬道指令仍在
  互動迴圈，滑鼠輪詢 205,509 次）。

---

## 2. planar write mode 3（READY）

### 證據

| 事實 | 證據 | 等級 |
|---|---|---|
| 鏈內 write mode 使用次數 0=42048、1=0、2=480、**3=1160** | probe `planar write mode 使用次數` | confirmed（軌跡） |
| 當成 mode 0 處理時，中文字形只畫出零星像素 | `workplace/re05/menu-crop.png` | confirmed |
| 實作後選單文字完整（帝國／同盟／對戰／取進度） | `workplace/re05/full-wm3.png` | confirmed |

### 語意（VGA 標準）

write mode 3 的 CPU 位元組**是遮罩不是顏色**：

1. 依 `gc[3]` 低 3 位右旋 CPU 位元組；
2. 旋轉結果與位元遮罩暫存器 `gc[8]` 相 AND，得到這次寫入的有效遮罩；
3. 每個 plane 的資料一律取自 set/reset 暫存器 `gc[0]` 的對應位元
   （展成 `0x00`／`0xFF`），**不看 enable set/reset (`gc[1]`)**；
4. 之後的 ALU（`gc[3]` bit3-4）、遮罩混合與 map mask 與其他模式共通。

漏掉第 2 步會把字形遮罩當成顏色寫進 plane——畫面上就是「字只剩幾個點」，
而且不會有任何錯誤訊息。

## 3. mode 0 的 enable set/reset（READY）

`gc[1]` 每一位對應一個 plane：該位為 1 時，那個 plane 的資料來自 `gc[0]`
而不是 CPU 位元組。原本完全沒實作，等於把 `gc[1]` 當永遠是 0。

實測影響：關閉這條時大面積底色是色號 15，開啟後是色號 3，兩者差
297,587 個像素（同一狀態，`workplace/re05/ab-nosetreset.bin` 對照）。
硬體語意上忽略 `gc[1]` 就是錯的，故實作；**但底色究竟該是哪一個，要等
原版對拍才算定案**——本 spec 只宣稱硬體語意正確，不宣稱畫面 parity。

### 驗收

- 單元測試 `TestPlanarWriteMode3MasksWithCPUByte`、
  `TestPlanarMode0EnableSetReset`。
- probe 報告新增 `planar write mode 使用次數`，讓「用了沒實作的模式」
  這件事可被看見而不是安靜跑過去。
