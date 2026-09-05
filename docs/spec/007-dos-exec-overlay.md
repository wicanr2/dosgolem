# 007 — DOS EXEC 覆疊載入

狀態：**READY**（僅 `int 21h AX=4B03h` 的 MZ 覆疊載入）  
日期：2026-09-07

## 1. 範圍

只實作 `AX=4B03h`。`AL=00h`／`01h` 的建立行程、PSP、環境、父子行程
切換與離開碼不在本規格內，遇到時仍須列為未實作，不可假裝成功。

## 2. 玩家路徑證據

EOB1 DOS `START1.EXE` 經正常文字選單輸入 `44Y` 後，首次 EXEC 呼叫為：

```text
呼叫後位址 9FE5:011E  AX=4B03  DS:DX=9FE5:0091  ES:BX=9FE5:00FB
路徑 C:\RICH2\INTRO.EXE
參數區前四 bytes 10 01 10 01
```

兩個 little-endian word 都是 `0110h`：第一個是映像載入段，第二個是
重定位基準。輸入雜湊：

- `START1.EXE`：`3274f986770203b891f0c4f2ae18c457e602f54be7955d983427fa7ce548fa96`
- `INTRO.EXE`：`ef1fa246c8bf05aedecbd93390846cb37fdb67bd2c42bcc080c14326039a7963`

證據等級：**已證實（proven）**，來自 dosgolem 指令級重播與呼叫前暫存器／
記憶體快照。這只證明上述 EOB1 版本的呼叫形狀，不把其餘 EXEC 子功能升格。

## 3. 契約

參照 Microsoft 公開的 MS-DOS 4.0 `v4.0/src/DOS/EXEC.ASM`：覆疊模式不配置
記憶體、不建立環境或 PSP；參數區提供載入位址，重定位項以參數區提供的
重定位基準調整。dosgolem 必須：

1. 由 `DS:DX` 有界讀取 NUL 結尾路徑，沿用唯讀、大小寫不敏感的素材解析。
2. 由 `ES:BX` 讀取載入段與重定位基準。
3. 驗證完整 MZ 檔頭、宣告檔案長度、header、重定位表與目的記憶體界線；
   任一越界皆失敗即關閉，不得部分覆寫。
4. 把 header 後的映像載到指定段，逐筆將目標 word 加上重定位基準。
5. 成功時清 CF；失敗時設 CF，找不到檔案回 `AX=2`，格式或界線錯誤回
   `AX=11`。不得改動 CPU 的 CS:IP、SS:SP、DS、ES 或 PSP。
6. 這是載入器，不負責跳入覆疊；呼叫端在返回後自行轉移控制。

## 4. 驗證

- 合成 MZ：指定載入段與不同重定位基準，確認 bytes 與 relocation 正確。
- 截斷 header、越界 relocation、超出模擬記憶體：失敗且目的區不變。
- DOS 服務層：真實檔名大小寫解析、成功 CF、找不到與壞格式錯誤碼。
- EOB1：`START1.EXE` 的 `44Y` 正常路徑不再印出 `Abnormal program termination`，
  並繼續執行 `INTRO.EXE`；後續新阻塞另立窄規格，不擴張本契約。

