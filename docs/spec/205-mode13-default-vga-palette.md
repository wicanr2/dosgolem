# 205 — Mode 13h 預設 VGA 前 16 色

狀態：CONFORMED

## 問題與證據

`Machine.New` 的 DAC 為全零；`INT 10h AH=00h` 切入 mode 13h 只改 BDA 與畫面狀態，
沒有載入 BIOS 預設色盤。這使只局部重設 DAC 的程式把未重設色號錯誤地顯示為黑色。

《Buck Rogers: Countdown to Doomsday》由乾淨 `START.EXE` 冷啟動的固定證據為：

- #7,520,426：切入 mode 13h；
- #7,559,527：`INT 10h AH=10h AL=12h`，`BX=0`、`CX=15`，只寫 DAC 0–14；
- #7,673,481：同服務由 `BX=16` 寫 16 格；
- 因此 DAC 15 沒被遊戲覆寫，角色資料數值使用色號 15 時應沿用 BIOS 預設白色。

DOSBox-X `src/ints/int10_modes.cpp` 把 mode 13h 列為 `M_VGA`，其 `vga_palette`
前 16 格為標準 VGA 16 色，index 15 是 `(0x3f,0x3f,0x3f)`。這是成熟模擬器契約的
交叉證據；本規格不宣稱已逐位驗證真實 VGA BIOS ROM。

## 契約

1. `INT 10h AH=00h` 切入 mode 13h 時，載入標準 VGA DAC 0–15。
2. 內部值保留 VGA 6-bit 範圍；index 6 為棕色 `(0x2a,0x15,0)`，index 15 為白色
   `(0x3f,0x3f,0x3f)`。
3. 本次最小充分修正不猜補 DAC 16–255；它們仍可由程式後續的 BIOS 或埠寫入設定。
4. 遊戲後續局部 DAC 寫入照現有語意覆蓋對應格，不得重新載入或鎖住預設值。

## 驗收

- 單元測試先以非零 sentinel 填滿 DAC，切入 mode 13h 後核對 0–15 全表，並證明
  index 16 未被本規格改寫。
- 由原版冷啟動與既有正常玩家路徑重生色盤；遊戲仍只寫原有範圍，而 index 15
  必須保持白色。
- 角色資料頁的原版動態值須在 RGBA／PNG 可見；轉譯疊字的 raw framebuffer、輸入
  與安全矩形 containment 不得因此改變。

## CONFORMED 收據（2026-09-21）

- 單元測試核對 0–15 全表，並以 sentinel 證明 DAC 16 未被改寫；全套 Go 測試通過。
- 修正後由 `START.EXE` 冷啟動重建 70M state，再以正常 BIOS 空白鍵重建 100M state；
  raw framebuffer 維持 `b08623d2…a3`，palette 改為 `39f7fda3…f37d`，DAC 15 為
  `(255,255,255)`。
- 角色資料頁 base／`Y`、2×／3× 各獨立重播兩次，JSON、RGBA、baseline RGBA 與 raw
  framebuffer 均各自逐 byte 相同；原版 HP 數值區在 baseline 出現 72 個純白放大像素。
- base／`Y` raw framebuffer 仍分別為 `1f3b8194…d4dd`／`03d9bf1f…0f97`，事件、request、
  action、miss 與 active-key 計數未變。因此本規格在上述 mode 13h 與正常玩家路徑升為
  CONFORMED；未實作 DAC 16–255 的完整 BIOS 預設表。
