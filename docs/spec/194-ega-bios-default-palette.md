# 194 — EGA 相容模式的 BIOS 預設色盤

狀態：**READY**；日期：2026-09-08。

## 證據與審查

KOL 正常入口收據 `kol/docs/verification/dosgolem-integration-20260908.md`：
mode 0Dh 的索引像素非零，DAC 256 色全黑，RGB 擷取全黑。`SetVideoMode` 只呼叫
`VGA.resetMode()`，沒有載入預設 DAC。以上為已證實的程式與動態證據。

採用 SeaBIOS `vgasrc/stdvgamodes.c` 的 `vga_modes`、`pal_cga`、`actl_0d`、
`stdvga_set_mode` 作為既有 BIOS 行為契約，存取日期 2026-09-08：
https://github.com/coreboot/seabios/blob/master/vgasrc/stdvgamodes.c
這是 BIOS 相容行為，不宣稱原版硬體逐週期一致。不移植上游程式；按色彩位元公式獨立實作。

## 契約

1. `SetVideoMode(0Dh/0Eh)` 重設後載入 64 個 CGA 相容 DAC 色，其餘清為黑。
2. DAC 索引 bit0/1/2 分別為藍／綠／紅的 42 強度；bit4 為各分量 21 強度。
   bit3、bit5 不影響顏色；暗黃（bit0..2=6，bit4=0）修正為棕色 (42,21,0)。
3. 屬性色號 0..7 映射 DAC 0..7；8..15 映射 DAC 16..23。
   屬性模式控制為 01h、plane enable 為 0Fh，色彩選擇為 0。
4. `ScreenRGB` 仍只讀實際 DAC／屬性控制器；後續遊戲改寫 DAC 應立即生效。
   重新設模式恢復預設。其他模式暫不由本切片改變。
5. 驗收：棕色、深灰與白色、DAC 可改寫、模式重設、非目標模式不改色盤；
   KOL 原版正常入口的全黑失敗探針由 dosgolem 自行重跑並逐張檢視。

## 歷史訂正

`vga.go resetMode` 原註解以單一程式自行寫 identity 為由沿用 identity，不能證明
BIOS 預設就是 identity。保留一般 reset 的既有行為，在有明確來源的 0Dh/0Eh
模式設定階段覆寫正確映射。其他模式的預設色盤仍須各依證據驗收。

來源快照 SHA-256：`8ae58eb660e0b002b19a64c9c98992408a65d55d1379e7383d376f4868cb8411`。
驗證：機器層／DOS／oracle 套件通過；KOL 六項規則對照通過（123.786 秒）。
相同 1.2 億指令路徑的六張 RGB 擷取已逐張確認可見，索引雜湊保持不變；
收據位於 KOL `workplace/dosgolem-palette-20260908/`，原全黑證據保留於 integration 目錄。
