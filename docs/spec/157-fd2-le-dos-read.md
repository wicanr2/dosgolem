# 157 — FD2 LE DOS 唯讀檔案讀取

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 108`](../re/108-fd2-qread-dos-read.md)、
[`spec 139`](139-read-only-dos-file-provider.md)、[`spec 140`](140-fd2-le-dos-open-readonly.md)

- `FD2StartupDOS` 接受 `INT 21h/AH=3Fh`：BX 是已登錄唯讀 handle，CX 是最多
  `65535` bytes 的要求長度，DS:EDX 是目的緩衝。
- 成功時從目前檔案位置讀取最多 CX bytes，寫入完整目的範圍，AX 回傳實際
  byte 數並清 CF；EOF 是成功的零 byte 讀取。
- 未登錄 handle 回 DOS 錯誤 6；來源讀取或目的記憶體不可寫回錯誤 5。錯誤
  只改 EAX 低 16 位並設 CF，不接受任意 host path 或可寫檔案。
- 目的 descriptor 必須在寫入前一次驗證完整範圍。若 host read 已前進但目的
  寫入失敗，必須嘗試回復檔案位置；不可把部分資料冒稱成功。
- CX=0 仍須驗證 handle，但不觸碰 DS:EDX。

驗收：服務測試覆蓋完整讀取、短讀／EOF、零長度、無效 handle 與目的越界；
固定雜湊 FD2 自然執行 `0x3D9A2`，確認 `MDI.INI` 首次讀取內容、AX byte 數、
CF 清除及下一個真實阻塞點。

驗收收據（2026-09-06）：`TestFD2StartupDOSReadFile` 覆蓋完整讀取、短讀、
EOF、零長度與無效 handle；`TestFD2StartupDOSReadRejectsDestinationRange`
確認目的越界失敗並回復檔案位置。`TestFD2ReadsMDIINIIntoFillBuffer` 由固定
原版 LE entry 自然執行 `0x3D9A2`，將雜湊清冊內的 218-byte `MDI.INI`
完整寫入 `DS:0x63518`、AX=`218` 且 CF 清除。後續一次性探針已刪除，下一
阻塞移至 `fgetc` 的 `0x3DA58`（`FF 03`）。
