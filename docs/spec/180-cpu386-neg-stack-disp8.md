# 180 — CPU386 對 SS:ESP+disp8 執行 32 位 NEG

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 122`](../re/122-fd2-ini-sign-multiplier-negate.md)

- 擴充無 prefix 的 `F7 5C 24 disp8`：`neg dword ptr [esp+disp8]`。
- disp8 有號加入 ESP；來源與目的固定透過 SS 描述子讀寫 32 位小端序值。
- 成功時依 32 位 `0-value` 更新算術旗標；暫存器與 ESP 保持不變。
- 來源越界、目的越界或唯讀描述子時失敗，且不得部分寫入或修改 CPU 狀態。
- operand-size／segment／repeat prefix與未審查F7群組維持失敗即關閉。其他無前綴32位記憶體ModRM與合法SIB由[325](325-cpu386-neg-dword-memory.md)擴充。

## 驗收條件

- CPU 測試覆蓋正／負 disp8、一般值、零與 `0x80000000` 的結果及旗標，並覆蓋
  唯讀、邊界與未審查LOCK前綴拒絕。原錯誤SIB負例是合法EBP形狀，325已明示納入。
- 固定雜湊 FD2 自然執行 `0x3F289`，確認 `SS:[ESP+4]` 由 1 變成
  `0xFFFFFFFF` 並抵達 `0x3F28D`。
- 有界自然執行記錄下一個失敗即關閉位址，不順帶放寬後續指令。

## 驗證收據

- 固定雜湊 FD2 自然執行抵達 `0x3F289`；執行後抵達 `0x3F28D`，
  `SS:[ESP+4]` 由 1 變成 `0xFFFFFFFF`，暫存器保持不變。
- CPU 聚焦測試覆蓋正／負 disp8、一般值、零、`0x80000000`、算術旗標，
  以及唯讀／越界／錯誤 SIB 拒絕。
- 強制重建後的有界自然探針抵達下一個失敗即關閉位址 `0x46C91`：
  opcode `3B`、ModRM `5D`。本規格未順帶解釋或放寬該指令。

## 2026-10-03 勘誤與擴充

原始FD2收據保留。原測試「wrong SIB」F7 5C 25 disp8實為合法SS:EBP+disp8，原規格當時僅支援ESP。325 READY擴充一般32位記憶體NEG，原負例改成真正未審查LOCK；來源／界限／唯讀拒絕仍不部分寫，晚期Bus錯誤的逐byte部分寫近似明示於325，不能稱完整CPU rollback。
