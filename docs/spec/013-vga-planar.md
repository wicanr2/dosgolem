# 013 — VGA planar（mode 12h：四個 plane 的寫入路徑）

狀態：**READY**（§2 是實測的埠序列；§5 是邊界宣告）
日期：2026-09-06
前置：[`003`](003-machine-and-loader.md)（記憶體與畫面）、[`005`](005-oracle-api.md) §3.3（畫面回索引）

---

## 1. 定位

mode 13h 的畫面是線性的：一個位元組一個像素，寫 `A0000+n` 就是點第 n 個點。
EGA／VGA 的 16 色模式不是——**一個位元組管八個像素的同一個 bit plane**，
四個 plane 疊起來才是 0–15 的色號。程式對 `A0000` 寫一次，實際上要先用
Sequencer 的 Map Mask 選「這一次寫哪幾個 plane」，再由 Graphics Controller
決定資料怎麼跟 latch 合成。

沒有這層模型，四個 plane 的寫入會落在同一段線性記憶體互相覆蓋。
**症狀不是黑畫面，是一張看起來有東西、但內容是最後一次寫入的圖**——
而記憶體、色盤、指令流全部正常，沒有任何地方會報錯。

換一支 binary 還成立 → 機器層（`internal/machine`）。

## 2. 量測證據（2026-09-06，源平合戰 `OPEN.EXE`）

收據 `yuan/workplace/boot-20260906-04/`，繞過 DOSJP 的殼鏈跑 2,000 萬道，
`oracle.PortWrites()` 取得的顯示埠序列：

| 暫存器 | 次數 | 寫過的值 | 意思 |
|---|---:|---|---|
| `SEQ[02]` Map Mask | 4 | `0F 0F 0F 01` | 先四個 plane 一起寫（清畫面），再切成只寫 plane 0 |
| `GC[05]` Graphics Mode | 3 | `0B 0B 00` | write mode 3 ＋ read mode 1，最後回 write mode 0 |
| `GC[08]` Bit Mask | 3 | `FF FF FF` | 整個位元組都寫 |
| `GC[00]` Set/Reset | 2 | `00 00` | |
| `GC[01]` Enable Set/Reset | 1 | `00` | |
| `GC[03]` Data Rotate／Function | 3 | `00 00 00` | 不旋轉、replace |
| `GC[04]` Read Map Select | 1 | `00` | 讀 plane 0 |
| `GC[07]` Color Don't Care | 2 | `00 00` | |
| `3C8`／`3C9` DAC | 288／864 | — | 288 組 RGB |

BDA 的視訊模式是 `12h`（640×480、16 色、四個 plane 各 38,400 bytes）。
`3C0`（Attribute Controller）與 `3D4`（CRTC）**沒有任何寫入**——模式是
`int 10h AH=00h AL=12h` 設的，程式沒有自己改時序或 palette 對映。

## 3. 模型

### 3.1 記憶體

`A0000`–`AFFFF` 的 64 KB 視窗背後是四個各 64 KB 的 plane。**這段位址不再
落在 1 MB 的線性記憶體裡**——planar 模式打開後 `Read8`／`Write8` 對這段
改走 §3.3／§3.4，`Mem[]` 的對應位元組不再被讀寫。

打開的條件是**目前視訊模式**（BDA `0040:0049`，由 `int 10h AH=00h` 寫）：
`0Dh`、`0Eh`、`10h`、`12h` 是 planar，其餘（含 `13h`、文字模式）不是。
設模式時清空四個 plane 與 latch——真機的 BIOS 設模式也清畫面。

### 3.2 暫存器

| 埠 | 內容 | 實作 |
|---|---|---|
| `3C4`／`3C5` | Sequencer 索引／資料 | 只有 index 2（Map Mask，低 4 bit）有作用 |
| `3CE`／`3CF` | Graphics Controller 索引／資料 | index 0–8 全存，語意見 §3.3／§3.4 |
| `3C8`／`3C9` | DAC | 既有，不動 |

`3C5`／`3CF` 可以讀回（`In8`）——有些程式先讀再改。

### 3.3 讀：先載 latch，再看 read mode

**讀一次 `A0000+off` 會把四個 plane 的那個位元組載進四個 latch。**
這是副作用，不是最佳化的餘地：`write mode 1`（見下）整個機制就靠它。

- read mode 0（`GC[05]` bit 3 ＝ 0）：回 `plane[GC[04] & 3][off]`。
- read mode 1：color compare。回傳的每個 bit 表示「這八個像素裡，
  參與比較的 plane 是否都等於 `GC[02]`」。`GC[07]`（Color Don't Care）
  的 bit ＝ 1 才參與比較。

### 3.4 寫：四種 write mode

共同的最後一步（write mode 1 除外）——ALU 與 bit mask 都是**逐 plane**
對 latch 做的：

```
x = alu(val[p], latch[p])           # GC[03] bit 4–3：0 replace／1 AND／2 OR／3 XOR
x = x & bitmask | latch[p] & ^bitmask
plane[p][off] = x                   # 只有 Map Mask 選中的 plane 才寫
```

四種 mode 的差別在 `val[p]` 與 `bitmask` 怎麼來：

| write mode | `val[p]` | `bitmask` |
|---:|---|---|
| 0 | `GC[01]` 該 plane 是 1 → `GC[00]` 該 bit 展開成 `00`／`FF`；否則是旋轉後的資料 | `GC[08]` |
| 1 | `latch[p]`，**不套 ALU 也不套 bit mask**（latch 原封不動寫回選中的 plane） | — |
| 2 | 資料的 bit p 展開成 `00`／`FF` | `GC[08]` |
| 3 | `GC[00]` 該 bit 展開成 `00`／`FF` | `GC[08] & 旋轉後的資料` |

「旋轉後的資料」＝ 寫入值向右旋轉 `GC[03] & 7` 位。

### 3.5 畫面讀出

`Indexed()` 在 planar 模式回 `w×h` 的色號（0–15），`w`／`h` 由模式決定：
`0Dh` 320×200、`0Eh` 640×200、`10h` 640×350、`12h` 640×480。
色號是**四個 plane 的 bit 疊出來的值**，不經 Attribute Controller 的
palette 對映（§5）。`VideoSize()` 回目前的尺寸；`oracle.Width`／`Height`
仍是 mode 13h 的 320×200，planar 下要改用 `ScreenSize()`。

## 4. 驗收

1. `tools/go.sh test ./internal/machine ./oracle` 全綠，含新增的
   write mode 0／1／2／3、read mode 0／1、Map Mask、bit mask、latch 與
   `Indexed()` 的釘死測試；
2. mode 13h 的行為不變（既有測試 ＋ 一個明寫「13h 不走 planar」的測試）；
3. 源平合戰 `OPEN.EXE` 在 mode 12h 下的 `Indexed()` 不再是「一片 FF」。

## 5. 這份規格**不做**什麼

- **Attribute Controller（`3C0`）**：16 個 palette register 到 DAC 的對映。
  量測到的程式沒寫它，色號直接用 plane 值比對更接近原始資料。
  要做顏色比對時再補。
- **CRTC（`3D4`／`3D5`）**：起始位址、pel panning、split screen、
  可程式化的解析度。量測到的程式沒寫它，尺寸從模式號決定。
- **chain-4／mode X**、奇偶模式（文字模式的 plane 0/1 交錯）、
  256 KB 以上的分頁。
- **latch 的讀取延遲與 CPU 匯流排時序**：這是行為模型，不是週期模型
  （同 `004` §5）。

## 觀測：planar 的寫入看得見

`Write8` 把 `A0000–AFFFF` 的寫入轉給 planar 模型，那些位元組不在 `Mem[]`
裡，所以 `WatchWrites` 看不到。兩個補洞的工具：

- `Machine.RowWritesFrom` ＋ `VideoRowWrites[]`：從某一道指令起，統計每一列
  被寫過幾個位元組。`cmd/probe -row-writes-from N`。
- `Machine.VGATraceRow0/Row1` ＋ `VGATrace`：記下寫進指定幾列的**最後 40 筆**
  寫入，連同 CS:IP、write mode、Map Mask、Bit Mask、Set/Reset 與 latch。
  `cmd/probe -vga-trace-rows 起-迄`。

**「畫面不對」要先分成三種**：程式根本沒寫、寫了但被後面蓋掉、寫了但
顯示卡狀態讓它沒生效。只看最後的畫面分不出來，這三種的修法完全不同。
留最後 40 筆而不是前 40 筆，是因為要知道**最後是誰寫的**。

> 用法實例（源平合戰的環境設定畫面）：對話框下半部看起來沒畫，
> 但每列寫入量顯示那些列有三千個位元組寫進去，與上半部相當——
> 所以不是「程式沒畫」。再看 CS:IP，填色的是 GRPDRV 的
> `035F:3B29`（一個逐列的遮罩搬移常式），而那段期間**一次 VGA 埠寫入
> 都沒有**，整個對話框是用同一組顯示卡設定畫的。
