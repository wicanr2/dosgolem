# 027 — 386 environment byte scan 指令

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`025`](025-dos4gw-environment-block.md)、[`RE 019`](../re/019-fd2-environment-double-null-scan.md)

- `40`–`47`：32-bit INC r32，更新算術旗標但保留 CF；operand-size 16-bit 失敗即關閉。
- `80 /7`：只支援固定路徑所需的 DS:`[ESI]` 與 DS:`[ESI+disp8]` byte CMP；以
  `sub8` 更新旗標，不寫回記憶體。其他 memory addressing 仍失敗即關閉。
- `AC`：LODSB 由 DS:`[ESI]` 讀入 AL，依 DF 調整 ESI。
- `A4`：非 REP MOVSB 由 DS:`[ESI]` 搬至 ES:`[EDI]`，依 DF 調整兩個索引。
  `F3 A4` 尚未由本切片授權，維持失敗即關閉。
- 固定雜湊 FD2 應由 `0x3CB27` 抵達 `0x3CB3B`；最小 environment 結果為
  ESI=`12`、EDI=`0x546B9`、EAX=`0x20212000`、ZF=`1`，目的緩衝區包含
  `FD2.EXE\0`，且尚未執行 `POP DS`。

驗收包含零／非零 byte CMP、不寫回、INC 保留 CF、分離 descriptor 的 LODSB／MOVSB
實際搬移，以及固定雜湊 FD2 整合執行。

2026-09-06：上述單元測試與固定雜湊 FD2 整合測試通過，抵達 `0x3CB3B`。
