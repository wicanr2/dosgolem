# 135 — 386 MOVZX byte ptr [EAX]

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 095`](../re/095-fd2-open-reloads-mode-byte.md)

- 在既有 `0F B6` 分支增加無 prefix、`mod=0`、`r/m=EAX`：
  `MOVZX r32,byte ptr [EAX]`。
- effective address 使用指令前 EAX；從 DS 讀 byte，零擴展寫入 ModRM reg，
  旗標不變。
- 其他 base 與 prefix 維持失敗即關閉。

驗收：合成測試確認來源位址在寫回前取值；固定雜湊 FD2 必須自然執行
`0x36EDB` 至 `0x36EDE`，確認 EAX 等於原 `[DS:EAX]` byte。

驗收收據（2026-09-06）：`TestMOVZXByteFromEAX` 確認先取址後覆寫與旗標
不變；`TestFD2ReloadsOpenModeByte` 從固定原版 LE entry 自然執行至
`0x36EDE`，確認 EAX 等於原 `[DS:EAX]` byte。
