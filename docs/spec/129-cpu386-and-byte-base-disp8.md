# 129 — 386 AND byte base+disp8

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 089`](../re/089-fd2-open-clears-file-mode-bits.md)

- 支援無 prefix 的 `80 /4`、`mod=1`、非 SIB：
  `AND byte ptr [base+sign-extended disp8],imm8`。
- EBP base 使用 SS；其他 base 使用 DS。讀取 byte、AND immediate、寫回同一位置，
  並依 byte 結果更新邏輯旗標。
- 暫存器與 effective address 不變。
- operand16、segment override、repeat、SIB 與其他新 `80` 形狀維持失敗即關閉。

驗收：合成測試覆蓋正／負位移、DS／SS 選擇、寫回與旗標；固定雜湊 FD2
必須從 LE entry 自然執行 `0x36EC9` 至 `0x36ECD`，確認 `[DS:EBX+0x0C]`
由原值變為 `原值 & 0xFC`，EBX 不變。

本規格不授權新的 C runtime FILE 結構或 host 檔案服務語意。

驗收收據（2026-09-06）：`TestAndByteBaseDisp8` 覆蓋正／負位移、DS／SS、
寫回與旗標；`TestFD2ClearsOpenFileModeBits` 從固定原版 LE entry 自然執行
至 `0x36ECD`，確認 `[DS:EBX+0x0C]` 清除低兩位且 EBX 不變。
