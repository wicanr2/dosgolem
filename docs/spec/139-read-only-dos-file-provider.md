# 139 — 唯讀 DOS 檔案提供器

狀態：**CONFORMED**
日期：2026-09-06

- 提供 `OpenRead(name)`，回傳可供後續 read／seek／close 使用的唯讀檔案。
- 只接受單一 DOS 檔名；空字串、`.`、`..`、磁碟字首、任何 `/` 或 `\`
  一律拒絕，不採 basename fallback。
- 在明確 root 的第一層進行大小寫不分比對；使用 `os.Root` 開檔，路徑不能
  逃出 root。目標必須是 regular file。
- 不提供 create、truncate 或 write。

驗收：測試大小寫相容，並拒絕絕對路徑、磁碟字首、目錄與 traversal；
成功開啟後可讀取內容且能關閉。

驗收收據（2026-09-06）：`TestDirectoryReadOnlyFiles` 驗證大小寫相容、
內容讀取、關閉，以及空值、`.`、`..`、絕對路徑、磁碟字首、子目錄與
root 外絕對 symlink 均被拒絕。
