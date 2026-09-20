# Buck Rogers 性別選擇生命週期收據

狀態：**CONFORMED**

本規格核准沿用既有 `cmd/buckrogers-text-receipt`，從固定 state 以正常 BIOS 輸入進入
性別選擇畫面，分別建立 Down→Up 與 Escape 路徑的 content-safe 事件及 indexed
framebuffer 收據。不新增命令能力、不翻譯、不繪中文，也不改寫原版狀態。

## 固定輸入與位址空間

- state SHA-256：`cfe15d3c66c9fe3c2e684815740a0cc0165e59d08ab5866370608d49f8a8e164`。
- `START.EXE`／`GAME.OVR` SHA-256：
  `58a34a38b1db455202d2d30daa82915982d7d905932b46bdc7371cb466226cf1`／
  `3a4ad4856c08fe5973179f1d907feed1d870af99d08abd1cb884b316324f3cc0`。
- 工具基線 commit：`7009315c5a04981eb1b048e7bba34a0b932fbf6d`；命令來源 SHA-256：
  `8ce3a79a791659f72f4fa4190274155462ce701431bf11c8d7da7ee8ae291506`。
- caller 採 dosgolem runtime `segment:offset`，不是 IDA 線性位址或檔案偏移。

## 排程與驗收契約

兩條路徑都在 #100,010,000 以 `-bios-enter-at` 排入第一個 Enter，並在 #100,240,000
排入第二個 Enter；精確停止於 #101,000,000。

- Down→Up：#100,400,000 排 Down，#100,460,000 排 Up；收據恰有 18 事件。Down 必須依序
  normal 重畫舊男性列、selected 重畫女性列；Up 必須反向完成同一對重畫。
- Escape：#100,400,000 排 Escape；收據恰有 22 事件。先 normal 重畫舊男性列，再重建
  上一層功能選單的七筆文字事件；不得誤稱回到種族選單。
- 同一路徑兩次 JSON 與 framebuffer 必須逐 byte 相同；所有事件須通過完整 identity、絕對
  step、嚴格遞增時序及鍵盤排程驗證。
- 正式資料只保存長度、SHA-256、caller、色號、座標與語意角色，不保存原版英文全文。

## 符合性紀錄（2026-09-21）

- Down→Up JSON SHA-256：`e5d58d61cc74cb464343692355f68d5f4d54f3ecdd0f57f6e832f6d2de8cbd52`；
  framebuffer SHA-256：`dcf947d18c85b051ec85e1bc968f30cec5c315a9158d598055d14ebfe82a715c`。
- Escape JSON SHA-256：`333960a20c5699ece5430e93a1fd3f7b04f0b5bd02a45c39a5e1050a0e96198a`；
  framebuffer SHA-256：`b08623d259a39bb2b3c755b3312e0afe413411bed53649d13e99019c5fbdf3a3`。
- 兩路徑各自重播兩次，JSON 與 64,000-byte framebuffer 皆逐 byte 相同。
- Down→Up 終點 framebuffer 等於未移動的性別畫面，證實選擇回到第一列；Escape 終點則
  與功能選單重建一致。本規格不外推 Enter 確認性別後的路徑。

