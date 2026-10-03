# 193：CPU386 短奇偶條件跳躍

狀態：**READY**
日期：2026-10-04
追蹤：https://github.com/wicanr2/fd2_re/issues/148

## 原版證據

FD2.EXE，357074 bytes，MD5 b97caf2239a27a896069d03549d96e1e，
SHA-256 222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f。
fresh-full-original-r4，runner9ad073f7591a783cfa4a71638b1ea24226321f15，
6958650868步在dosgolem relocated LE線性0x3C868拒絕opcode7B。
原bytes 7B 18，JNP rel8，目的0x3C882。
同段0x3C87D原bytes 7A F4，JP rel8，目的0x3C873；這條尚未實測到達。
Capstone 5.0.3經fd2_re/tools/disasm_le.py核對0x3C859..0x3C885，
工具位址為該工具LE線性，分別保留，未合併工具位址基準。
純屬已知停止點與鄰近原bytes機械核對，不新解遊戲函式，故不另開IDA。

同段FNSTSW word ptr [EBP-2]、MOV AH byte ptr [EBP-1]、SAHF的
直接指令，是PF旗標的消費線索。語意依Intel架構契約，不附加遊戲欄位名。
來源：fd2_re/work/parity-slot-ch24/fresh-full-original-r4/oracle.log；
三角函數修正前的2115點仍零差異。

## 契約

只新增無前綴7A JP／7B JNP，消費一個signed rel8。
JP在PF為1時取分支，JNP在PF為0時取分支。
目標由指令末尾EIP加signed rel8，沿既有32位EIP政策。
不取分支時維持指令末尾EIP；整數旗標、暫存器、FPU與記憶體不改寫。
前綴與截斷拒絕；解碼錯誤沿既有逐byte消費，不宣稱EIP原子回復。

來源：[Intel SDM Volume 2A，Jcc 3-483..487](https://www.intel.com/content/dam/www/public/us/en/documents/manuals/64-ia-32-architectures-software-developer-vol-2a-manual.pdf#page=585)。
實作入口internal/cpu386/cpu.go既有短Jcc相鄰case；不更動既有其他Jcc。

## 驗收

- 兩opcode的taken／not-taken，其他旗標不能影響判斷。
- 原7B18、7AF4、signed rel8正負邊界與EIP32位環繞。
- FCOS → FNSTSW AX → SAHF → JNP，以正常CPU指令消費C2。
- 前綴／截斷拒絕且不發布資料或旗標。
- 完整cpu386／machine／FD2 oracle回歸與parity建置。
- 同SAV／seed／計畫重跑，舊2115點一致並越過3C868。
  CPU切片與完整章#142分開，JP原版可達性未驗則保留限制。

## 2026-10-04 相關回歸

針對性測試先重現7A／7B拒絕，修正後全部通過。完整cpu386／machine／FD2 oracle回歸通過，parity無測試但建置通過；固定原版EXE／ROOT唯讀掛載，沒有缺來源略過。收據workplace/fd2-parity-branch-regression-20261004.jsonl。原版同槽重跑尚待，狀態保持READY。
