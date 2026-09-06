# 011 — int 10h AH=10h 的子功能語意；每個向量一段可執行的 stub

日期：2026-09-06
狀態：**READY**（§1 調色盤子功能、§2 向量 stub）
動機：`GIN3PS.EXE` 畫得出主選單，但整份調色盤是黑的
（`~/cht/logh3/docs/re/06`）。兩個獨立的缺口疊在一起：AH=10h 的子功能
接錯，以及走「取向量再跳過去」的 BIOS 呼叫完全沒有執行。
前置：[`009`](009-planar-vga.md) §1（AH=10h 的第一版）、
[`003`](003-machine-and-loader.md) §2（向量 stub）。

---

## 1. int 10h AH=10h 的子功能（READY）

### 證據

| 事實 | 證據 | 等級 |
|---|---|---|
| 鏈內用到 AL=02/09/10/12/17 五種 | probe `int 10h AH=10h` 記錄（41 次）| confirmed（軌跡） |
| `3C5B:0085` 是 `setDAC(index, blue, red, green)`：BX=索引、CL=藍、DH=紅、CH=綠，`AX=1010h` | objdump | confirmed（bytes） |
| AL=02 傳 `CX=0010`、AL=17 傳 `BX=0000 CX=0010` | probe 記錄 | confirmed |
| 接錯的症狀：DAC 前幾筆是 `(0,1,2)(3,4,5)(6,7,8)…` 的遞增值 | `workplace/re04` 的 `pal.bin` | confirmed |

### 語意

**屬性調色盤與 DAC 是兩層不同的東西。** 16 色模式的色彩鏈是
「4 位元色號 → 屬性暫存器（AttrPal）→ 6 位元 DAC」。AH=10h 的 AL 分成
兩組，接錯不會報錯，只會讓顏色安靜地變成別的：

| AL | 動的是 | 介面 |
|---|---|---|
| 00h | 屬性暫存器 | BL ＝ 索引、BH ＝ 值 |
| 01h | overscan | BH ＝ 值 |
| 02h | 屬性暫存器全份 | ES:DX → **17 bytes**（16 個 ＋ overscan）|
| 07h | 屬性暫存器 | BL ＝ 索引 → BH |
| 08h | overscan | → BH |
| 09h | 屬性暫存器全份 | ES:DX ← 17 bytes |
| 10h | **DAC 單筆** | BX ＝ 索引、DH ＝ R、CH ＝ G、CL ＝ B |
| 12h | **DAC 一段** | BX ＝ 起始、CX ＝ 個數、ES:DX → 3×CX bytes |
| 15h | DAC 單筆 | BX ＝ 索引 → DH/CH/CL |
| 17h | **DAC 一段** | BX ＝ 起始、CX ＝ 個數、ES:DX ← 3×CX bytes |

DAC 值一律 6 位元（`& 0x3F`）。其餘 AL（03h/13h/1Ah/1Bh…）仍記
unimplemented，有證據再補。

**讀回的那半（07h/08h/09h/15h/17h）不能省。** 遊戲的淡出淡入是
「讀回現有調色盤 → 存起來 → 塗黑 → 畫 → 用存起來的值淡回去」；讀回沒
實作的話它存到的是未初始化的緩衝區，淡回來的就是垃圾或全黑，而寫入路徑
本身完全正常，查不出是誰的責任。

### 驗收

`TestPaletteSetSingleDACRegister`、`TestPaletteBlockRoundTrip`、
`TestPaletteAttributeRegistersRoundTrip`；probe 報告新增
`int 10h AH=10h` 逐筆記錄（AL 與暫存器），讓接錯的參數看得見。

---

## 2. 每個向量一段可執行的 stub（READY）

### 證據

| 事實 | 證據 | 等級 |
|---|---|---|
| `3C89:0022` 是 `int86x`：`AH=35h` 取向量 → 疊中斷框架 → `iret` 跳進 handler，**不發 `int nn`** | objdump | confirmed（bytes） |
| 遊戲的調色盤套用走這條（`AX=1012h`、`AX=1001h`）| 軌跡 ＋ 反組譯 | confirmed |
| 這些呼叫在修正前完全沒有出現在 AH=10h 記錄裡 | probe 記錄 | confirmed |

### 語意

服務層掛在 CPU 執行 `INT` 指令的 hook 上（`dos.Install` → `CPU.IntHook`）。
向量本身指到一個裸 `iret` 的話，**只有真的執行 `int nn` 的呼叫會被服務**；
自己取向量跳過去的呼叫落在 `iret` 上，暫存器原樣回去——沒有錯誤、沒有
unimplemented 記錄，只有畫面或資料悄悄不對。

改為每個向量各有一段 stub，內容是 `int n` ＋ `iret`：

- 布局：`StubSeg`（0x0080）內，向量 n 的 stub 在位移 `n×4`，
  共 1 KB（線性 0x800–0xBFF）。`EnvSeg` 因此從 0x0090 移到 0x00C0。
- 跳進 stub → 執行 `int n` → hook 判定向量仍是我們的 → 服務執行 →
  接著的 `iret` 用呼叫端疊好的框架回去。
- 程式若自己裝了 handler（向量段 ≠ `StubSeg`），hook 照舊放行，
  stub 裡的 `int n` 會真的跳進那個 handler，回來時再 `iret` 回呼叫端——
  也就是自動具備鏈接行為。
- 每個 stub 的第一個位元組是 `CDh`，所以「handler 第一個位元組是 `CFh`
  ＝ 沒有滑鼠驅動」那條偵測自然成立（原本靠 `int 33h` 的專屬位移解決，
  現在不需要特例）。

### 驗收

`TestVectorStubRunsService`（含兩項負對照實測：stub 換成裸 `iret`
會被第一個斷言擋下，換成 `nop; iret` 會被「服務沒有跑到」擋下）。
整合：原版 `GIN3.COM` 鏈的主選單從全黑變成有顏色，AH=10h 呼叫從 37 次
增為 41 次。
