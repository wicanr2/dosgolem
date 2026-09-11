# 199 — BIOS ROM 的 8×8 字型放在 F000:FA6E

狀態：**READY**（位置是 IBM PC 相容 BIOS 的固定慣例；觸發案例有讀取監看的直接證據，見 §2）
日期：2026-09-11
前置：[`198-mode-set-default-dac`](198-mode-set-default-dac.md)

---

## 1. 規則

機器開機時，線性位址 `FFA6Eh`（`F000:FA6E`）起放 128 個字元的 8×8 字型，一個字元 8 bytes、
由上而下一列一 byte、**最高位元是最左邊的點**。`F000:FA6E` 是 IBM PC／XT 以來 BIOS 放 CGA 字型前半
（字元 0–127）的固定位址，程式可以不經任何中斷服務直接讀它；DOSBox-X 也在同一個位址放字型
（`src/ints/int10_memory.cpp` 把 8×8 字型寫到 `0xfa6e`）。

字形資料取自 `dhepper/font8x8` 的 `font8x8_basic.h`（**Public Domain**，SHA-256
`49d8df366296b203ca3211bc0672cf2a762135bf12710735b6292756b19dffd5`），每個 byte 反轉位元順序後產生
`internal/machine/romfont8x8.go`。

**與真機的差異**：字形不是 IBM 原廠 ROM 的點陣（授權不明，不收），逐像素比對會不同；
字元 `00h`–`1Fh`（原機是笑臉、心形等符號）在這份字型裡是空白。字元 128–255（`int 1Fh` 指的後半）與
`int 10h AX=1130h` 回傳的字型指標都不在本規格範圍，維持現狀。

## 2. 觸發案例

Borland C++ 2.0 的 BGI 以 `DEFAULT_FONT` 輸出文字時，字母與 `0` 用它自己內建的點陣，
其餘字元（標點、`1`–`9`、`@`、`{|}~`，共 37 個）去讀 `F000:FA6E`。
dosgolem 那塊記憶體全是 0，這 37 個字元畫成空白：`"Borland C++ 2.0"` 只剩 `"Borland C     0"`（末尾是走內建點陣的數字 0），
分數與等級的數字消失。`cmd/run -watch-read F0000-FFFFF` 記到 BGI 讀了 297 個位址（37 × 8＋1），
範圍 `FFB6E`–`FFFEA`，正好是那 37 個字元的字形。

## 3. 驗收

1. 測試：`machine.New()` 之後 `F000:FA6E` 起的內容逐字元等於 `romFont8x8`；抽驗 `'A'` 第一列是 `30h`；
   反面對照：修改前讀到 0。
2. `tools/go.sh test ./internal/machine ./internal/dos ./apps/...` 全綠。
3. BGI 印出 ASCII 32–127 的測試程式，所有可列印字元都畫得出來。
