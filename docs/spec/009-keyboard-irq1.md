# 規格009：鍵盤IRQ1與EOB1 Escape輸入

狀態：**CONFORMED**

## 範圍

為自行掛接`int 09h`的DOS遊戲提供最小、決定性的IBM PC/AT鍵盤輸入：外部排入
Set 1 make／break掃描碼，machine在IF允許時逐碼觸發IRQ1，遊戲處理程式由port
`60h`讀取當前掃描碼。本規格公開目前正常路徑所需的Escape、Enter與向下鍵；不在本輪模擬8042命令、PIC遮罩、
typematic、鍵盤LED或真實硬體wall-clock。

## 證據與分級

- **已證實**：EOB1 `START1.EXE`輸入`44Y`後，在2,240,174道指令的Westwood
  穩定幀，IVT `0000:0024`為`0B29:07B4`，不是dosgolem預設stub。
- **已證實**：由同狀態把Escape餵入既有DOS stdin，再跑50,000,000道指令，
  字元仍待讀；畫面只沿計時器路徑繼續片頭動畫。
- **已證實**：同一實驗的I/O讀取統計只有`03DAh`，沒有`0060h`；dosgolem未送
  IRQ1，因此EOB1的`int 09h`處理程式沒有執行。
- **平台契約**：IBM PC/AT鍵盤IRQ由`int 09h`處理，掃描碼由port `60h`取得；
  此部分採IBM PC/AT Technical Reference，不把標準平台行為重複列為遊戲逆向成果。
- **已證實**：向該處理程式送Escape Set 1 make `01h`與break `81h`後，port
  `60h`恰讀取兩次；原版開啟`FONT6.FNT`、`EOBDATA2.PAK`、`EOBDATA1.PAK`、
  `EOBDATA6.PAK`與`EOBDATA3.PAK`，抵達主選單穩定幀。

## 型別行為與邊界

- `machine.QueueScanCodes(...uint8)`保存有序byte佇列。
- `oracle.PressKey(KeyEscape)`排入`01h,81h`，不改動`Type`的DOS stdin佇列。
- `KeyEnter=1Ch`、`KeyDown=50h`；`PressKey`同樣各排入其make與最高位為1的break碼。
- IF清除時保留掃描碼；IF設定後，每個`Machine.Step`最多送一個IRQ1。
- IRQ1優先於同一步待送的IRQ0；其後IRQ0仍保留pending，不得遺失。
- port `60h`在處理期間回傳本次掃描碼；未送鍵時不建立玩家可見契約。
- Snapshot／Restore必須保存掃描碼佇列與當前port `60h`資料。

## 驗收

1. 合成`int 09h`處理程式依序讀到`01h,81h`。
2. IF清除時不送IRQ1，重新開啟後送出。
3. Snapshot／Restore重播得到相同掃描碼序列。
4. EOB1真實資料從具名Westwood checkpoint呼叫`PressKey(KeyEscape)`後，抵達
   可由檔案開啟、穩定畫面與SHA-256固定的下一個具名checkpoint；通過後本規格
   已通過；色號SHA-256為`caa3082b3e8cb5ee15547555669eb82e954982fe919674d5481271e06a253dc0`，
   palette SHA-256為`bf08a3424f429a6cb5400a8ddef50f8ee35ed03deb0de51a307799bbc6b9687f`。

## 權利與停止線

測試只保存雜湊、狀態與自造處理程式，不提交原版執行檔、資料或畫面。到達下一個
玩家可見穩定檢查點後停止本切片，不追求8042／PIC逐週期精確度。
