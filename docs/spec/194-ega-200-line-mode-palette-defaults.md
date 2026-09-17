# 194 — 200 線 EGA 模式（0Dh／0Eh）設模式時的屬性暫存器與 DAC 預設值

狀態：**READY**
日期：2026-09-17
前置：[`011-bios-palette-and-vector-stubs.md`](011-bios-palette-and-vector-stubs.md)（色彩鏈與 `AH=10h`）、
[`007-ega-mode-0dh-planar-vram.md`](007-ega-mode-0dh-planar-vram.md)（mode 0Dh 平面 VRAM）

---

## 1. 問題

16 色平面模式的色彩鏈是「4 位元色號 → 屬性暫存器 → 6 位元 DAC 索引 → DAC」（`011` §2）。
dosgolem 設模式時：

- 屬性暫存器 0–15 設成 identity（`ac[i] = i`）；
- **DAC 完全不動**，開機後一直是 0。

只用 `INT 10h AH=10h AL=00/02` 設屬性暫存器、不寫 DAC 的程式（EGA 時代的遊戲都是這樣：EGA 卡根本沒有 DAC），
畫面輸出經過 DAC 之後**全部是黑色**。`PlanarRGB`、`-dump-screen-png`、`.pal` 都受影響。

症狀不是報錯：平面上的色號完全正確，只有顏色是黑的。

量測案例：DOS 版《銀河超能力戰記》（`PW.EXE`，mode 0Dh）在 `psychic_war_cht` 的 `docs/re/003`：
dosgolem 色號與 DOSBox-X 版面逐像素一致，但 DAC 全部 ≤ 1，PNG 輸出全黑。

## 2. 真機行為

200 線 EGA 模式接的是 CGA 相容的 RGBI 螢幕：**6 位元色值只有 bit 0（B）、bit 1（G）、bit 2（R）、bit 4（I）有意義**，
bit 3、bit 5 不接。VGA BIOS 設 mode 0Dh／0Eh 時照這個規則載入 DAC 0–63，讓 EGA 程式在 VGA 上顏色不變。

### 2.1 屬性暫存器

| 暫存器 | 值 |
|---|---|
| 0–7 | `00h`–`07h` |
| 8–15 | `10h`–`17h`（bit 4 ＝ 高亮） |
| `10h` mode control | `01h`（圖形） |
| `12h` color plane enable | `0Fh` |
| 其餘 | `00h` |

### 2.2 DAC 0–63

對索引 `i`：`b = i&1`、`g = i>>1&1`、`r = i>>2&1`、`hi = i>>4&1`（bit 3、bit 5 忽略）。

| 條件 | 每個分量（6 位元） |
|---|---|
| 分量位元 0、`hi = 0` | `00h` |
| 分量位元 1、`hi = 0` | `2Ah` |
| 分量位元 0、`hi = 1` | `15h` |
| 分量位元 1、`hi = 1` | `3Fh` |
| **例外：`hi = 0` 且 `r = g = 1、b = 0`（棕色）** | G 分量為 `15h`（不是 `2Ah`） |

所以 `i=06h` → `(2Ah,15h,00h)` 棕色，`i=16h` → `(3Fh,3Fh,15h)` 黃色，`i=17h` → 白色。
DAC 64–255 不動。

依據：DOSBox-X `src/ints/int10_modes.cpp` 設模式流程——`M_EGA` 且 mode ≤ 0Eh 時寫入的 64 色表，
與 `att_data` 的 `ct`／`ct+0x10` 設定。這裡只取其規則，以公式實作，不照抄表格。

## 3. 範圍

- **只改 mode 0Dh、0Eh。** 0Fh／10h（350 線，6 位元 rgbRGB）與 12h（VGA）DAC 預設不同，
  而既有對拍（例如 mode 12h 的案例）可能依賴目前的行為，本規格不動它們。
- 屬性暫存器預設也只在 0Dh／0Eh 改成 §2.1；其他平面模式維持 identity。
- 快照（`state`）本來就存 DAC 與屬性暫存器，不必改。

## 4. 驗收

1. `SetVideoMode(0x0D)` 後：屬性暫存器符合 §2.1；DAC 0–63 符合 §2.2 全部 64 筆；DAC 64 起不變。
2. mode 0Eh 同上；mode 12h、10h 的 DAC 在設模式前後不變（回歸）。
3. 色彩鏈端到端：mode 0Dh 設模式後，`SetPal(15, 0x77&0x3F)` 的色號 15 經 `PlanarRGB` 為白色；
   `SetPal(10, 0x76&0x3F)` 為黃色；未設的色號 8 為深灰 `(55,55,55)`（8 位元）。
4. 既有測試全部通過（CPU 語料缺檔的會 skip）。
5. 使用案例：`psychic_war_cht` 的四個檢查點，dosgolem RGB 與 DOSBox-X 逐像素一致（在該專案驗收）。

## 5. 不做

- 350 線 EGA 與 VGA 的 DAC 預設。
- `AH=10h AL=03h`（閃爍／高亮切換）等其他子功能。
- 過掃描色（overscan）的顯示。
