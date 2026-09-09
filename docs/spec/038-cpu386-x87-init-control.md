# 038 — x87 FNINIT 與 FNSTCW stack form

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 030`](../re/030-fd2-x87-callback-entry.md)

- CPU 保存可延伸的 x87 control word、八層數值 stack 與 depth；本切片不假裝完成
  x87 全指令集。
- `DB E3`（FNINIT）設定 control word=`0x037F`，清空 stack 與 depth。
- `D9 3C 24`（FNSTCW `[ESP]`）經 SS descriptor 寫入 16-bit control word，不改 ESP。
- 其他 `DB`／`D9` forms 維持失敗即關閉，不當 NOP。
- 固定雜湊 FD2 從 `0x45E36` 抵達 `0x45E40`，ESP=`0x55684`，stack dword
  `0x0003037F`，FPUControl=`0x037F`、FPUDepth=0。

驗收包含非預設初值重設、實際 stack word 寫入及固定 callback 入口。

2026-09-06：上述單元測試與固定雜湊 x87 callback 測試通過，抵達 `0x45E40`。
