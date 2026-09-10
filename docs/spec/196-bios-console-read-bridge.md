# 196 — 預設主控台的 BIOS 讀鍵轉接

狀態：**READY**；日期：2026-09-08。

## 原版證據

KOLBOOK.EXE SHA-256 `2cada02856d3cf5c3bec948ef3736be6f4bd47e3abce94ddf835c1678e0e5832`，
IDA Pro 9.4，IDA linear：`sub_146CE` 的 `0x14B60..0x14BAF` 直接讀實體
`0000:041A/041C` 的 BIOS 佇列指標，非空才呼叫 `_getch`。
`_getch` 在 `0x187D2` 設 DH=08、`0x187F4` 交換 AX/DX、`0x187F5` 執行 INT21。
因此同一個鍵必須同時能被 BDA 非空檢查看見、由 DOS AH08 取走。這是已證實的讀取鏈。
原始 bytes／xref 在 KOL 的 `workplace/kolbook-input-20260908/kolbook-input-private.json`。

## 契約與審查

現有 conIn 的 AH01/07/08 共用預設主控台輸入，只有 Stdin，因此原版會卡在混用的兩端。
保留明確 Stdin 字元的優先序；當 Stdin 空時，從 BIOS BDA 環形佇列 PopKey 取一 word。
普通鍵返回低 byte；擴充鍵低 byte=0 時先返回 0，再把掃描碼保存在既有 Stdin 供下一次取走。
一次 PopKey 只增加一次 KeysConsumed，字元回傳仍由 noteKey 逐次記錄。
沒有任一輸入時維持既有阻塞／NonBlockingKeys 契約，不生成假字元。
本切片不變更檔案重導向模型或額外合併其他 BIOS／DOS 介面。

Microsoft DOS 4.0 Programmer’s Reference 的 Direct Console Input／Unfiltered Character Input
記載擴充鍵需讀兩次： https://www.pcjs.org/documents/books/mspl13/msdos/dosref40/
驗收：BDA 非空→AH08 返回一般鍵→BDA 清空；方向鍵兩次返回；Stdin 優先且沒有雙讀；原版正常書本重跑。
