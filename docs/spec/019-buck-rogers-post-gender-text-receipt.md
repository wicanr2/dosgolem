# Buck Rogers 確認性別後的文字事件收據

狀態：**CONFORMED**

本規格核准沿用既有 `cmd/buckrogers-text-receipt`，從固定 state 排入三次正常 BIOS Enter，
建立接受預設種族及預設性別後，職業選擇畫面的 content-safe 文字事件與 indexed framebuffer
收據。不新增命令能力、不翻譯、不繪中文，也不修改原版角色建立規則或狀態。

## 固定輸入與排程

- state SHA-256：`cfe15d3c66c9fe3c2e684815740a0cc0165e59d08ab5866370608d49f8a8e164`，
  起點 #99,999,999。
- `START.EXE`／`GAME.OVR` SHA-256：
  `58a34a38b1db455202d2d30daa82915982d7d905932b46bdc7371cb466226cf1`／
  `3a4ad4856c08fe5973179f1d907feed1d870af99d08abd1cb884b316324f3cc0`。
- dosgolem 基線 commit：`24f0dd55cdce8a929ab513e2a2a1f78afb6fb56b`；receipt command
  SHA-256：`ed6e7b6537a9be3fe332f49821adee0d91c51a1f4510ca32648ed1b0f8fad5c9`。
- Enter 排程：#100,010,000、#100,240,000、#100,400,000；停止於 #101,000,000。
- caller 採 dosgolem runtime `segment:offset`，不是檔案偏移或 IDA 線性位址。

## 事件契約

收據必須恰有 22 個完成事件、零 pending、零 drop。前 14 筆完全等於既有性別畫面初始
收據；第 15 筆是確認前把 selected 男性列恢復 normal；後七筆固定如下：

| entry → post-call | caller | len／SHA-256 | bg/fg | row,col | 已證實內容角色 |
| --- | --- | --- | --- | --- | --- |
| 100410301 → 100418147 | `37F1:158C` | 10／`ba7ebb6c…11cf2` | 0/13 | 2,1 | 職業選擇提示 |
| 100437195 → 100447349 | `37F1:15BD` | 13／`0daed6fa…28df7` | 0/10 | 3,1 | normal 第一職業 |
| 100464371 → 100469932 | `37F1:15BD` | 7／`5f06e4b5…afa06` | 0/10 | 4,1 | normal 第二職業 |
| 100491004 → 100498085 | `37F1:15BD` | 9／`703e3358…c1ed6` | 0/10 | 5,1 | normal 第三職業 |
| 100517807 → 100525650 | `37F1:15BD` | 10／`c5d861df…9d682` | 0/10 | 6,1 | normal 第四職業 |
| 100544697 → 100550258 | `37F1:15BD` | 7／`01889f0c…aa4bd` | 0/10 | 7,1 | normal 第五職業 |
| 100571639 → 100580245 | `37F1:175D` | 11／`86e1e1a3…555f1` | 15/0 | 3,3 | selected 第一職業 |

一次性 probe 已以原版 bytes 對雜湊核對提示與五個選項的語意；正式 spec、清冊與 JSON 不保存
原版英文全文。上述 identity 與單一預設種族／性別路徑是已證實；不同分支、方向鍵、Escape、
確認職業後畫面與職業規則仍未知。

## 驗收與停止線

- 固定三鍵排程完整重播兩次；JSON 與 64,000-byte indexed framebuffer 各自逐 byte 相同。
- 專案 verifier 固定 state／原版／command hash、三鍵排程、22 筆 exact identity、嚴格 step、
  新七筆 inventory 與終點 framebuffer hash。
- inventory 與 verifier 正反向測試拒絕 schema、次序、identity、step、排程與畫面漂移。
- 全部正式 packages test／vet 及相關 race detector 通過後才能標為 CONFORMED。
- 本規格不授權繁中 catalog、text-safe rectangle、renderer、倍率、角色規則或下一畫面外推。

## 符合性紀錄（2026-09-21）

- 固定三鍵排程完整重播兩次，兩份 22-event JSON 逐 byte 相同，SHA-256 均為
  `a3e195ebe6f43ffa2d9fdcfaeb92c8f68323da9bb487dc9c468095c3b012570b`。
- 兩份終點 64,000-byte indexed framebuffer 逐 byte 相同，SHA-256 均為
  `3a61cb542625bdb0fd353f1d337e87c68091bd2dd2185e9b2ba34dad7558012c`。
- 第 15 筆 normal male 與後七筆職業畫面事件逐欄符合本規格；最後事件於 #100,580,245
  完成，到 #101,000,000 沒有新增 dispatcher 完成事件。
- 專案 58 項測試與真實 verifier 通過；dosgolem spec 索引實際為 256 份，全部正式 packages
  test／vet、`apps/buckrogers` 與 receipt command race detector 通過。
