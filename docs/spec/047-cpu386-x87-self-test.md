# 047 — 386／x87 啟動自我測試最小契約

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 039`](../re/039-fd2-x87-self-test-helper.md)

只實作固定 helper 實際使用的標準形式：

- `66 PUSH r16`、`66 POP r16`，以 32 位元 ESP 配合兩位元組堆疊項目；
- `FLD1`、`FLD ST(0)`、`FCHS`、`FDIVP ST(1),ST(0)`、`FCOMPP`；
- `FNSTSW AX`，保存本切片需要的例外與 C0/C2/C3 狀態；
- `FLDCW word ptr [ESP]` 的精確 ModRM/SIB `2C 24` 形式，經 SS 描述子讀取；
- `SAHF`，由 AH 更新 SF、ZF、AF、PF、CF；
- `66 XCHG AX,word ptr [ESP]`，只授權 ModRM/SIB `04 24`。

所有其他新操作碼、ModRM、SIB、前綴及 x87 形式維持失敗即關閉。`FNINIT` 必須
清除 x87 status，重設控制字與 stack；除零在預設 masked control 下不得中止 CPU。

固定 FD2 驗收由 LE 入口自然執行整個 helper、返回 `0x460FB`，預期 AX=`0x0103`、
ESP=`0x55694`、FPUControl=`0x127F`、FPUDepth=0，且 DOS service calls 仍為 2。

2026-09-06：獨立 CPU 序列測試、固定雜湊整合測試、命令列探針及全套 Go
回歸通過；探針於 367 步返回 `0x460FB`，AX=`0x0103`、ESP=`0x55694`。
