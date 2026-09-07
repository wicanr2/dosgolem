# 146 — FD2 LE DOS IOCTL get-device-information

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 102`](../re/102-fd2-isatty-ioctl-device-query.md)、
[`spec 140`](140-fd2-le-dos-open-readonly.md)

## 平台契約與範圍

- `FD2StartupDOS` 支援 `INT 21h AX=4400h`，以 BX 的低 16 bits 查詢已登錄
  handle 的裝置資訊。
- 由本服務 `AH=3Dh` 開啟的唯讀 regular file 是磁碟檔案，成功時將 DX 低
  16 bits 設為 0、保留 EDX 高 16 bits並清 CF；bit 7 因而為 0。
- handle 不存在時設 CF 並回傳 DOS error 6（invalid handle）。
- `AH=44h` 的其他 AL 子功能設 CF 並回傳 DOS error 1（invalid function）。

這些回傳值採 DOS 公開介面契約；FD2 專屬證據只證實固定原版在
`0x3FB1D` 選用 `AX=4400h`、BX=已開啟 handle，並以 DX bit 7 判斷
`isatty`。

## 失敗即關閉與邊界

- 不偽造 character device、console 或其他裝置位元。
- 不接受未由目前 LE 服務登錄的標準／外部 handle。
- 不實作其他 IOCTL 子功能。

## 驗收

- 合成服務測試覆蓋 regular file、invalid handle 與 unsupported subfunction。
- 固定原版由 LE entry 自然執行 `0x3FB1D`，抵達 `0x3FB1F`，CF=0、
  DX bit 7=0，且 BX 仍指向已登錄 handle。

本規格不宣告 DOS read／seek／close 或 character-device handle 已完成。

驗收收據（2026-09-06）：`TestFD2StartupDOSDeviceInformation` 覆蓋 regular
file、invalid handle 與 unsupported subfunction；
`TestFD2QueriesOpenedFileDeviceInformation` 由固定原版 LE entry 自然執行
`0x3FB1D`，抵達 `0x3FB1F`，CF=0、DX bit 7=0 且 BX 仍是已登錄 handle。
後續有界探針越過既有 RCL／ROR，下一阻塞移至 `0x3FB23` 的
`F6 C2 80`。
