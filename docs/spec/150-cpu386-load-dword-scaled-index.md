# 150 — CPU386 base＋scaled-index dword load

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 101`](../re/101-fd2-iomode-tests-file-state.md)

## 範圍

- 擴充 opcode `8B` 的 ModRM `04h`、SIB `B0h`：
  `MOV EAX,[EAX+ESI*4]`。
- 在覆寫目的 EAX 前，先用原 EAX base 與 ESI index 計算有效位址；由 DS
  selector 讀取一個 little-endian dword，寫入 EAX，不修改旗標。

## 失敗即關閉

- 只接受固定原版證實的 SIB `B0h`；其他既有 `8B` shape 維持原契約。
- DS descriptor 超界時不覆寫 EAX。

## 驗收

- CPU 單元測試驗證 scale 4、目的與 base 重疊、來源不變、旗標不變與越界失敗。
- 固定原版由 LE entry 自然執行 `__IOMode` 的 `0x4639D`，讀回
  `table_base + handle*4` 的 dword 並抵達 `0x463A0`。

本規格只處理 Watcom FILE table 的必要 CPU addressing shape，不宣告該表格
所有欄位語意或其他一般 SIB 組合已完成。

驗收收據（2026-09-06）：`TestLoadDwordFromBaseScaledIndex` 驗證 scale 4、
目的／base 重疊、來源與旗標不變及越界失敗；`TestFD2LoadsOpenedFileRecord`
由固定原版 LE entry 自然執行 `0x4639D`，讀回 `table_base + handle*4`
dword 並抵達 `0x463A0`。後續有界探針的下一阻塞移至 `0x463B6` 的
`89 04 98`。
