# Buck Rogers scratch-backed 儲存與角色名冊生命週期

狀態：**CONFORMED**

本規格修正 spec 030 的檔案副作用證據缺口，核准 `buckrogers-text-receipt` 接受明示的可寫
scratch 目錄，再由同一固定 state 重跑儲存詢問 `NO`／`YES` 及後續加入角色功能。這是診斷
能力，不授權寫入 pristine original、散布原版素材或推導完整存檔格式。

## 工具契約

- 新增 `-scratch <dir>`；非空時必須是已存在的目錄，命令在 `state.Load` 後把它設為
  `DOS.Scratch`。缺目錄或非目錄失敗即關閉。
- `Root` 仍由 savestate 還原為唯讀 `/orig`；所有 create、write、unlink 與 write-open 都依
  dosgolem 既有 scratch 契約落到 scratch，讀取時由 scratch shadow root。
- 收據新增 scratch 路徑只作診斷 metadata；正式 verifier 以寫前／寫後 manifest、事件、
  framebuffer 與鍵盤排程為準，不保存原版內容。
- 新增可選 `-file-ops`；啟用時收據輸出 dosgolem 已有的 `Wrote` 與 `FileOps` metadata，包含
  step、操作、功能號、handle、basename、位置、長度與失敗旗標，不輸出檔案內容。

## 固定輸入與生命週期

- 共用前綴沿用 spec 030；`NO` 為 #107,000,000 `N`，`YES` 為 #107,000,000 Left、
  #107,200,000 Enter。
- 每條接受分支使用空白且相互隔離的 scratch，從相同 state 獨立重跑兩次。
- 回到功能選單後以 Down→Enter 進入加入角色功能；比較 `NO`／`YES` 的角色名冊可見結果。
- 若 `YES` 產生 scratch 檔案，固定檔名、大小與 SHA-256 差異；不得把格式語意外推到未解析
  欄位。若沒有寫入，必須由已啟用 scratch 的收據證明。
- 若名冊可操作，只追一條最小選取／加入或返回路徑到第一個穩定玩家可見邊界。

## 驗收

- `-scratch` 的正反例單元測試通過；既有未指定 scratch 的命令行為不變。
- `NO`／`YES` 與名冊分支各兩次 JSON、64,000-byte framebuffer 及 scratch manifest 可重播。
- verifier 固定 state、原版程式、命令、完整 BIOS 排程、事件 identity、畫面與檔案差異；
  任一漂移皆失敗即關閉。
- 專案測試、全部正式 dosgolem packages test／vet 及相關 race detector 通過後，本規格才能
  升為 CONFORMED。

## 符合性紀錄（2026-09-21）

- `-scratch` 與 `-file-ops` 已實作；空值相容、有效目錄、一般檔案及不存在路徑的正反例測試
  通過。
- `NO`／`YES` 後進入加入角色功能各重播兩次；每對 316-event JSON、framebuffer 與 scratch
  manifest 逐 byte 相同。兩分支沒有角色列，並回到相同功能選單。
- FileOps 證實兩分支都以讀寫模式 shadow `CHARS.DAX`，但沒有 DOS write 呼叫；四份 scratch
  只含內容 SHA-256 等於 pristine 的 `CHARS.DAX`。
- 專案 94 項測試與真實 verifier 通過；dosgolem 正式 packages test／vet 與 Buck Rogers
  相關 race detector 全數通過。
