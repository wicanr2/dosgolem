# 192 — CRTC 的效果：列距與分割畫面

狀態：**READY**
日期：2026-09-10
前置：[`013-vga-planar.md`](013-vga-planar.md)（平面式 VRAM）、
[`007-vga-planar.md`](007-vga-planar.md) §2（模式判定）

---

## 1. 問題

dosgolem 現在只認 CRTC 的**顯示起點**（index `0C`／`0D`，加上 mode
control `17` 的 bit6 決定單位）。畫面解碼的其他兩個參數是寫死的：

- **一列幾個位元組**：`Pixels` 用 `w / 8`。真機由 offset（index `13`）
  決定，而**捲動的遊戲一定會改它**——邏輯畫面比可視畫面寬，
  水平捲動就是把顯示起點往右挪幾個位元組。
- **整個畫面共用一個起點**：真機掃到 line compare（index `18`）那一列
  時位址計數器歸零，下半部從 0 開始重數。**遊戲用它固定狀態列**：
  上半部捲動，下半部不動。

兩者的症狀都不是報錯：

| 少了誰 | 倒出來的畫面 |
|---|---|
| offset | 每一列往左或往右錯開固定的量，整張圖斜成平行四邊形 |
| line compare | 狀態列跟著地圖一起捲走，或地圖被狀態列的內容覆蓋 |

**斜掉的圖看起來像解碼壞了，不像少讀了一個暫存器。**

## 2. 事實

### 2.1 offset（index `13`）

單位是**字組**：一列的位元組數 ＝ `offset × 2`。BIOS 給 mode `12h`
（640×480×16）設 `0x28` ＝ 40 → 80 bytes，正好是 `640/8`。

程式設更大的值就是「邏輯畫面比可視寬」。`0` 是「還沒被設過」，
不是「列距 0」——那時退回用模式的寬度推（`w / 8`）。

⚠ 還有一種 dword 定址（underline location `14` 的 bit5），乘數不同。
**這裡不做**：老遊戲的 16 色 planar 模式走的是 byte／word，
真的遇到 dword 的程式再補，現在做等於憑空猜一個沒有量測支持的乘數。

### 2.2 line compare（index `18` ＋ 兩個溢位位元）

9 位元，散在三個暫存器裡——**這是最容易只讀到低 8 位的地方**，
而只讀低 8 位的症狀是「分割線出現在畫面上方某處」而不是沒有分割：

| 位元 | 位置 |
|---|---|
| 0–7 | index `18` |
| 8 | index `07`（overflow）的 bit4 |
| 9 | index `09`（max scan line）的 bit6 |

掃到那一列時位址計數器歸零。所以分割線**之下**那幾列的位址從 0 起算，
與顯示起點無關——那正是狀態列不跟著捲的原因。

沒有分割時 BIOS 把它設成全 1（`0x3FF`），也就是「永遠掃不到」。

**分割在下一列生效**（`line compare + 1`），證據見 §2.3。

### 2.3 分割在哪一列生效：`line compare + 1`

FreeVGA 的字面描述是「垂直計數器到達這個值時位址計數器歸零」，讀起來
像是**那一列本身**就從 0 起算。實際上差一列，而判準來自硬體的一個
邊界行為。

DOSBox 的 `src/hardware/vga_draw.cpp`：

```c
vga.draw.split_line = (vga.config.line_compare + 1) / vga.draw.render_max;
...
if (vga.draw.split_line == vga.draw.lines_done) VGA_ProcessSplit();
```

同一個檔案的註解說明了為什麼是 `+1`：

> What is supposed to happen is that `line_compare == 0` on normal VGA
> will cause **the first scanline to repeat twice**.

**只有 `+1` 的語意產生得出這個效果**：`line_compare ＝ 0` 時第 0 列用
顯示起點、第 1 列用位址 0，起點也是 0 的話就是同一條掃描線畫兩次。
若是「那一列本身就歸零」，第 0 列本來就用起點，不會重複。

那份註解還附了一個具體案例（Issue #40，Flash productions "monstra"
的畫面頂端多一條白線），是這個邊界真的被遊戲踩到過的證據。

推論等級：**強證據**。一手來源是 DOSBox 的實作與它解釋的硬體行為，
而那份實作被幾千款遊戲驗證過；還不到 confirmed 是因為沒有真機或原版
素材可以直接對拍。

### 2.4 畫面多大：問 CRTC，不要問模式編號

`horizontal display end`（index `01`）＋ 1 是寬度（單位是字元，圖形模式
一個字元 8 像素）；`vertical display end`（index `12`）＋ 1 是高度，
而它是 10 位元——第 8 位在 overflow（`07`）的 bit1、第 9 位在 bit6。
（對照 DOSBox `vga_draw.cpp`：
`vdend = vertical_display_end | ((overflow & 2) << 7) | ((overflow & 0x40) << 3)`。）

**模式編號只是 BIOS 的一個代號。** 直接設暫存器換解析度的程式從來不改
它——那種程式在模式表上會被讀成「還在上一個模式」，而畫面早就不是那個
大小了。所以尺寸的一手答案在 CRTC，模式表只是 CRTC 還沒被設過時的退路。

「還沒被設過」要**回 0×0**，不要回一個猜的尺寸：回猜的值，呼叫端會拿它
當事實而不是退回模式表。

## 3. 設計

- `VGA.Pitch(w int) int`：回一列的位元組數。offset 非零就用
  `offset × 2`，否則退回 `w / 8`。
- `VGA.LineCompare() int`：回三個暫存器湊出來的 9 位元值。
- `VGA.Pixels(w, h)` 改成逐列算基底位址：分割線之上是
  `起點 + y × pitch`，之下是 `(y − 分割線) × pitch`。
- `VGA.Size()`：從 CRTC 的 display end 算畫面大小，沒被設過回 0×0。
  `PlanarSize`／`VideoSize` 優先問它，退回模式表。
- `resetMode` 給該模式的標準值：offset ＝ `寬度 / 16`（word 單位）、
  display end ＝ 該模式的寬高、line compare ＝ `0x3FF`（不分割）。

  ⚠ **offset 只在知道模式的時候給。** 真機開機是文字模式，CRTC 的值
  由 BIOS 依模式設；`newVGA` 那次初始化還不知道模式，猜一個寬度的話
  320 寬的模式會拿到 640 的列距，每一列錯開一倍。不知道就留 0，
  `Pitch` 退回用呼叫端給的寬度推——那個路徑本來就要有，
  因為「還沒被設過」與「列距 0」是兩件事。

不做的部分（以及為什麼）：

| 暫存器 | 為什麼不做 |
|---|---|
| `00`–`0B`、`10`–`16` 的時序 | 它們決定掃描的節奏，而 dosgolem 的畫面是「跑完之後解一張圖」，不是逐掃描線輸出 |
| `0E`／`0F` 游標位置 | 圖形模式沒有硬體游標；文字模式的游標不影響對拍 |
| `14` bit5 的 dword 定址 | 見 §2.1 |

## 4. 驗收

1. offset 改變時，`Pixels` 的列距跟著變；offset ＝ 0 時退回 `w / 8`。
2. line compare 之下的列從位址 0 起算，不受顯示起點影響。
3. line compare 的 9 位元三個來源都算進去。
4. `New()` 與 `SetVideoMode` 之後，offset 與 line compare 都是該模式的
   標準值（不是零值）。
5. 既有測試全過。
