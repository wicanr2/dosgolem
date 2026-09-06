# 159 — CPU386 從基址間接載入 dword

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 109`](../re/109-fd2-filbuf-consumes-first-byte.md)

- 擴充無前綴 opcode `8B /r`，接受 `mod=0`、非 ESP／EBP 的
  `MOV r32,dword ptr [r32]`；資料 segment 使用 DS。
- 目的暫存器由 ModRM reg 欄位選擇，來源基址由 r/m 欄位選擇；讀取失敗時
  目的暫存器保持不變。
- EBP 的此編碼代表絕對 disp32、ESP 代表 SIB，沿用既有獨立分支；
  operand-size、segment override 與其他形狀維持失敗即關閉（fail-closed）。
- MOV 不修改旗標。

驗收：CPU 測試覆蓋非 ESI 基址、任意目的暫存器、旗標不變與越界失敗；固定
雜湊 FD2 自然執行 `0x3DA5A`，確認 EAX 載入 `[EBX]` 並抵達 `0x3DA5C`。

驗收收據（2026-09-06）：`TestMoveDwordFromDSBaseMemory` 覆蓋 EBX 基址、
ECX 目的、旗標不變與越界失敗，既有 ESI 路徑亦通過；
`TestFD2LoadsAdvancedFilbufPointer` 由固定原版 LE entry 自然執行
`0x3DA5A`，確認 EAX 等於 `[EBX]` 並抵達 `0x3DA5C`。後續一次性有界探針
已刪除，原版已返回設定解析器，下一阻塞移至 `0x46C84` 的 `88 03`。
