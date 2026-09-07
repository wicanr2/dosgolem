# 140 — FD2 LE DOS AH=3Dh 唯讀開檔

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 099`](../re/099-fd2-dos-open-mdi-ini.md)、[`spec 139`](139-read-only-dos-file-provider.md)

- `FD2StartupDOS` 只在 `INT 21h AH=3Dh` 接手開檔；目前只接受 `AL=0`。
- 由 DS:EDX 有界讀取最多 260 bytes 的 NUL 字串；selector／範圍錯誤、缺少
  NUL、路徑被 provider 拒絕或檔案不存在時，設定 CF 並回傳 DOS 錯誤碼。
- 成功才配置從 5 開始的 handle，清 CF，將 AX 設為 handle；已開檔物件
  留在表內，供後續 `AH=3Eh／3Fh／42h` 規格使用。
- 沒有 provider 時失敗即關閉；不偽造成功。

驗收：合成測試覆蓋成功、非唯讀模式、非法路徑與不存在檔案；固定原版從
LE entry 自然執行 `0x3CD73`，實際開啟固定雜湊 `MDI.INI`，抵達 `0x3CD75`，
CF=0 且 AX 為已登錄 handle。

驗收收據（2026-09-06）：`TestFD2StartupDOSOpenReadOnly` 驗證成功 handle、
非唯讀模式、非法及不存在路徑不配置 handle；`TestFD2OpensMDIINI` 從固定
原版 LE entry 自然執行 `0x3CD73`，以唯讀掛載的固定 `MDI.INI` 開啟
handle 5，抵達 `0x3CD75` 且 CF=0。
