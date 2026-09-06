# 148 — CPU386 register byte SETNZ

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 102`](../re/102-fd2-isatty-ioctl-device-query.md)

- 擴充既有 register-direct SETcc，支援 `0F 95 /r` 的 `SETNZ r8`。
- ZF=0 時寫入 1，ZF=1 時寫入 0；只覆寫指定 byte register，不改其他
  register bits，也不修改任何旗標。
- 記憶體 ModRM 與 prefix 仍失敗即關閉。

驗收：CPU 測試覆蓋 ZF 兩值、高／低 byte 與旗標不變；固定原版自然執行
`0x3FB26` 的 `setnz al`，regular file 路徑抵達 `0x3FB29` 且 AL=0。

本規格不宣告後續 `MOVZX eax,al` 已完成。

驗收收據（2026-09-06）：`TestSETNZRegisterByte` 覆蓋 ZF 兩值、目的 byte
與旗標不變契約，並拒絕記憶體形狀；
`TestFD2NormalizesOpenedFileDeviceBit` 由固定原版 LE entry 自然執行
`0x3FB26`，regular file 路徑抵達 `0x3FB29` 且 AL=0。後續有界探針的
下一阻塞為 `0x3FB29` 的 `0F B6 C0`。
