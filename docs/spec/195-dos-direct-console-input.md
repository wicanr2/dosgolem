# 195 — DOS 非阻塞直接主控台輸入

狀態：**READY**；日期：2026-09-08。

## 證據與契約

Microsoft MS-DOS 4.0 Programmer’s Reference 的 Direct Console I/O (Function 06H)：
https://www.pcjs.org/documents/books/mspl13/msdos/dosref40/
當 AH=06h、DL=FFh 時，若標準輸入有字元，消耗一個並回 AL、清 ZF；沒有字元回 AL=0、設 ZF。
不可回顯，不作 Ctrl-C 檢查，不得阻塞。其他 DL 值仍為輸出路徑。

目前 `internal/dos/int21.go conOut` 的輸入分支永遠設 ZF，即使 Stdin 有字元也不讀。
此介面不符合契約已由程式確認；它是否造成 KOL 書本選單卡住仍待正常路徑重跑，不先宣稱因果。

實作使用現有 DOS Stdin 佇列，每次取一 byte，透過既有 noteKey 留下來源 AH06 的軌跡。
不順便合併 BIOS 與 DOS 佇列，也不更動重導向或擴充鍵編碼。
驗收涵蓋：有字、空佇列、NUL 字元仍算成功、順序與不回顯；保留既有 AH06 輸出測試。
