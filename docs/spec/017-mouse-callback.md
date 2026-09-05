# 規格017：INT 33h AX=0Ch滑鼠事件callback

狀態：**CONFORMED**

## 平台契約與EOB1證據

Microsoft Mouse Programmer's Reference（2nd edition, 1991）規定：`AX=000Ch`以`CX`保存事件遮罩、
`ES:DX`保存使用者far routine；符合遮罩的事件發生時，以far call進入，`AX`為事件旗標、`BX`
為按鍵狀態、`CX/DX`為游標座標、`SI/DI`為mickey計數。

EOB1正常建角鏈至角色檢視頁共呼叫`AX=000Ch`三次，且不再以`AX=0003h`輪詢。從同一快照測試
Enter、Right、Down、Right+Down、Down+Right及三次Tab後Enter，畫面均留在檢視頁；因此本頁
`KEEP`不能以現有鍵盤或輪詢滑鼠路徑觸發。

## typed行為

- `Mouse`保存callback mask、segment及offset；`AX=0000h`重設或`AX=000Ch,CX=0`停用。
- 移動事件旗標bit 0、左鍵按下bit 1、左鍵放開bit 2；只有與註冊mask交集非零才far call。
- callback入口：`AX=event`、`BX=buttons`、`CX=X*XScale`、`DX=Y`、`SI`為垂直mickey、`DI`為
  水平mickey，依Microsoft原始參考手冊，不採後來部分列表的對調版本。
- far call推入原CS:IP後跳到callback，後續由原版routine的`RETF`回復；不偽造軟體中斷框架。
- `MoveMouse`及`Click`必須由同一事件入口更新狀態；Click順序仍為move／hover／press／hold／
  release／settle，讓每次callback有界執行，不允許未返回時巢狀覆蓋。
- Save／Restore沿既有Mouse深拷貝保存callback註冊資料。

## 驗收與失敗邊界

1. 合成callback以`RETF`返回，驗證原CS:IP／SP及六個入口暫存器。
2. mask不符或mask為0不得呼叫；reset停用callback。
3. EOB1檢視頁點擊`KEEP`後必須離開該頁並抵達可見姓名輸入checkpoint。
4. 真實資料測試與既有Rich2全庫回歸通過後升CONFORMED。

以上均已通過：EOB1檢視頁原版註冊`AX=0Ch`三次，`Click(282,180)`經move／press／release
callback抵達姓名頁；全庫測試與vet通過。

## 停止線

不模擬PS/2 IRQ12、串列滑鼠封包、加速度、真實mickey時序、右／中鍵或巢狀callback；若EOB1正常
玩家路徑需要其中任一能力，另開窄規格。原版檔案與畫面不提交版本庫。
