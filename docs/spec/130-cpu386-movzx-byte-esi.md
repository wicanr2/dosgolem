# 130 — 386 MOVZX byte ptr [ESI]

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 090`](../re/090-fd2-open-flags-mode-byte.md)

- 支援無 prefix 的 `0F B6 /r`、`mod=0`、`r/m=ESI`：
  `MOVZX r32,byte ptr [ESI]`。
- 從 DS:ESI 讀取 byte，零擴展後覆寫 ModRM reg 指定的完整 32 位暫存器。
- ESI、來源記憶體與旗標不變。
- operand16、segment override、repeat、其他 base 與其他 ModRM 形狀維持失敗即關閉。

驗收：合成測試確認高位清零、來源與旗標不變；固定雜湊 FD2 必須從 LE
entry 自然執行 `0x36E15` 至 `0x36E18`，確認 EAX 等於 `[DS:ESI]`。

本規格不授權新的 C runtime 模式解析或 host 檔案服務行為。

驗收收據（2026-09-06）：`TestMOVZXByteFromESI` 確認 byte 零擴展、來源、
ESI 與旗標不變；`TestFD2LoadsOpenModeByte` 從固定原版 LE entry 自然執行
至 `0x36E18`，確認 EAX 等於 `[DS:ESI]`。
