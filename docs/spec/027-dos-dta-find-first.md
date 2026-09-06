# 規格027：DOS DTA、檔案屬性與Find First

狀態：**CONFORMED**

## 範圍與證據

平台契約採MS-DOS `int 21h`：AH=1Ah設定DTA、AH=2Fh取得DTA、AH=43h AL=00h取得檔案屬性、
AH=4Eh Find First寫入DTA。EOB1正常`START1.EXE → INTRO.EXE → EOB.EXE`覆疊鏈實際呼叫：

- `0110:0FE3`：AH=2Fh；隨後`0110:0FF6`以CX=`0037h`、`DS:DX=02BA:05C3`
  Find First `C:\RICH2\INTRO.EXE`。
- `0110:6777`：AH=43h AL=00h查`INTRO.EXE`。
- `0110:6870`：AH=2Fh；隨後`0110:6883`以CX=`0037h`、`DS:DX=0FAF:2EFF`
  Find First `C:\RICH2\EOB.EXE`。

現況default分支只清CF卻不填DTA／CX，等於以垃圾宣告成功。正常PLAY後熱點固定在
`133E:1E21–1E58`腳本解譯迴圈，持續讀取資料但結束旗標`DS:A9EC`不成立；本規格只修正其上游
已證實的DOS資料契約，不把熱點語意先猜成特定遊戲規則。

## typed行為與失敗模式

- DOS建立時DTA預設為PSP:`0080h`；AH=1Ah保存`DS:DX`，AH=2Fh回`ES:BX`。
- AH=43h AL=00h以既有唯讀、大小寫不敏感basename resolver找檔；成功回CX archive bit `20h`，
  缺檔回CF及AX=2。其他子功能失敗即關閉並保留未實作診斷。
- AH=4Eh對已觀察的精確檔名以同一resolver找檔；成功清空並填DTA標準欄位：attribute `15h`、
  time `16h`、date `18h`、size `1Ah`、8.3名稱`1Eh`。時間／日期固定0以維持決定性；size來自唯讀
  `stat`。缺檔回CF及AX=18，不寫部分結果。
- 本切片不實作萬用字元、Find Next、volume label、目錄列舉、FCB parser或寫檔。

## 驗收與停止線

合成測試固定DTA round-trip、屬性成功／缺檔／不支援子功能、Find First欄位與缺檔不改DTA；
再以真實EOB1從冷啟動走四人建角及PLAY。正常路徑已進入LEVEL1第一格，且合成測試、全庫test／
vet均通過；本規格升CONFORMED。PLAY停點的直接原因另由規格028證實為callback未收尾，不把兩者
錯誤合併成單一因果宣稱。
