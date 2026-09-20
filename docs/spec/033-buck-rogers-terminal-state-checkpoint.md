# Buck Rogers 終態 savestate 診斷 checkpoint

狀態：**CONFORMED**

完整技能配置後的 `SAVE A? YES` 與加入角色輸入已由畫面、事件及 BIOS 排程對齊；spec 032
也排除了已記錄的未實作 DOS／BIOS 服務。然而 FileOps 與 framebuffer 無法回答原版是否在
記憶體建立暫存角色、在哪個狀態拒絕列入名冊。此規格核准 receipt 命令在終點保存本機
savestate，供既有 probe 從相同狀態做記憶體差分。

## 工具契約

- `buckrogers-text-receipt` 新增可選 `-state-out <path>`；空值維持既有行為。
- 命令完成全部鍵盤排程、事件完整性與 `until` 執行後，才以既有 `internal/state.Save` 保存
  Machine 與 DOS 終態。失敗時整個命令失敗即關閉，不得留下成功收據。
- savestate 是含原版執行期內容的本機研究輸入，只能寫入被 Git 忽略的 `workplace/`；不得
  加入 Git、GitHub、Release 或任何可散布包。JSON 不嵌入 state 內容。
- 旗標不得改變 CPU、DOS、畫面、鍵盤或檔案語意；只在終態序列化既有狀態。

## 驗收

- helper 的空路徑為 no-op；有效路徑能由 `state.Load` 回讀相同步數與記憶體；目錄或不可寫
  路徑失敗。
- 以正常玩家路徑分別在儲存詢問前、`YES` 接受後及加入角色後保存 checkpoint；後續差分
  必須保留位址空間、step 與證據等級。
- receipt command、Buck Rogers packages、全部正式 package tests、`go vet` 與相關 race
  detector 通過後，方可升為 CONFORMED。

## 停止線

- 記憶體差分只用來定位候選狀態，不得把任意變動 byte 自動命名為名冊、角色資格或存檔欄位。
- 候選語意仍須由原版讀寫端、控制流或相鄰玩家操作證實；否則維持假說或未知。

## 符合性紀錄（2026-09-21）

- `-state-out` 已實作；空路徑 no-op、有效 state 回讀相同步數／記憶體及目錄失敗的單元測試
  通過。
- 已由正常玩家路徑保存 `SAVE A?` 前、接受後與加入角色後 checkpoint；兩次正式終態 state
  的 gzip／gob 檔案本身不相同，但各自回讀的完整 1 MiB memory 逐 byte 相同，SHA-256 為
  `665fca1fa5f9bbfc38724b75d0779197665b4a066de1481d04ee17e07fb90a6f`。這只證明機器記憶體
  狀態決定性，不把封裝 bytes 差異誤報為狀態漂移。
- 正式 framebuffer 逐 byte 相同，SHA-256 為
  `a9171abc8464207e24377891d75097b290d74a455fad29dd92fa12cfe283ca00`；scratch `CHARS.DAX`
  亦逐 byte 相同且等於 pristine。
- 全部正式 packages test／vet 與 Buck Rogers／receipt race detector 通過。
