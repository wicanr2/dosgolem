# 198 — `int 10h AH=00h`：切到 mode 10h／12h 時載入預設 DAC

狀態：**READY**（行為照 DOSBox-X 原始碼；觸發案例見 §2）
日期：2026-09-11
前置：[`011-bios-palette-and-vector-stubs`](011-bios-palette-and-vector-stubs.md)、[`013-vga-planar`](013-vga-planar.md)

---

## 1. 規則

BIOS 設定模式（`int 10h AH=00h`）切到 **mode 10h 或 12h** 時，DAC 的第 0–63 格載入 64 色的
rgbRGB 表：色號的 bit 0／1／2 分別讓 B／G／R 加 `2Ah`，bit 3／4／5 分別讓 b／g／r 加 `15h`。

| DAC 格 | R, G, B（6 位元） |
|---|---|
| `00h` | 00, 00, 00 |
| `06h` | 2A, 2A, 00 |
| `07h` | 2A, 2A, 2A |
| `14h` | 2A, 15, 00 |
| `38h` | 15, 15, 15 |
| `3Fh` | 3F, 3F, 3F |

DAC 第 64 格以後、屬性調色盤（色號 → DAC 格）都不在本規格範圍，維持現狀。

依據：DOSBox-X `src/ints/int10_modes.cpp` 的 `INT10_SetVideoMode`：`M_EGA` 類型且模式大於 `0Fh`
（即 10h、12h）時 `goto dac_text16`，把 `text_palette[64]`（第 651 行）逐格寫進 DAC。
這張表就是上面的 rgbRGB 公式；本實作照公式產生，測試拿表裡的值核對。

## 2. 觸發案例

Borland C++ 2.0 的 BGI 驅動 `EGAVGA.BGI` 在 VGA 上用 mode 12h。它以 `int 10h AX=1002h` 把屬性調色盤設成
EGA 預設值（6 → `14h`、8–15 → `38h`–`3Fh`），顏色本身依賴 BIOS 在設定模式時載入的 DAC。
dosgolem 的 DAC 初值全是 0，所以畫面每一點都是黑的——但平面裡的色號完全正確：
測試程式的紅色實心方塊（色號 4）與淺藍方塊（色號 9）各 20,301 點（201×101），黃框、白字也在。

## 3. 不做什麼，以及理由

- **mode 0Dh／0Eh**（DOSBox-X 載入另一張 `ega_palette`）與 **mode 13h**（248 色預設表）：
  既有案例（EoB 等）的對拍紀錄建立在目前的 DAC 行為上，本規格沒有證據需要動它們。遇到需要時另立規格。
- **屬性調色盤的預設值**：DOSBox-X 在 10h／12h 會把屬性暫存器設成 0–5、14h、7、38h–3Fh；
  dosgolem 目前是恆等對映（`VGA.resetMode`）。改它會影響所有自己寫 DAC 0–15 的程式，同上理由不在本次範圍。
  已知差異：不自己設屬性調色盤、又不自己寫 DAC 的程式，色號 6 與 8–15 會與真機不同。

## 4. 驗收

1. 契約測試：`AX=0012h` 之後 DAC 0–63 符合 §1 的公式，並逐格核對 DOSBox-X `text_palette` 的
   六個代表值；`AX=0010h` 同樣成立；`AX=0013h` 之後 DAC 不被這條規則改寫。反面對照：修改前失敗。
2. `tools/go.sh test ./internal/dos ./internal/machine ./apps/...` 全綠。
3. BGI 測試程式在 dosgolem 的截圖出現紅、淺藍、黃、白四色。
