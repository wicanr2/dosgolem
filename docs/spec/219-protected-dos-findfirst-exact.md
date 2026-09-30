# 219 — 受保護模式 DOS 的精確檔名首次搜尋

狀態：**CONFORMED**（僅精確 8.3／CX=0 的合成環境切片）
日期：2026-10-01

## 原版問題與可重播輸入

固定 MOO2 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`。dosgolem 以合成 PSP／環境、無遊戲資料提供者診斷至第 4066 步，在**重定位 LE 線性位址** `0x139A59` 的 `CD 21` 因 `INT 21h/AH=4Eh` 未接線而停止。原版 DOSBox-X 2026.07.02 SDL2 heavy debugger，映像 `fd2-dosbox-x:debug-0d7b272b` ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`，於 **CS:EIP** `0180:0035DA59 → 0180:0035DA5B` 搜尋 `DS:0038E099` 的 ASCIIZ `MOX.SET`，CX=`0`；已設定的 DTA 是 `0188:003C3828`。兩工具位址基準、PSP／環境及 DTA 偏移不同，不能宣稱同狀態。

**原版缺檔樣本**：僅在容器 `/tmp/game` 放固定原版 EXE，`MOX.SET` 不存在。進入 EAX=`00384E99h`、EFLAGS=`0246h`；返回 EAX=`00000012h`、EFLAGS=`0247h`，其餘已擷取暫存器不變。DTA 的 43 bytes 由零變成 `02`、`MOX` 零填滿的 8 bytes、`SET` 3 bytes，其餘仍零。其後有界 24 指令先依 CF 進入錯誤處理，使用 `AX=0012h`；這段沒有直接讀 DTA，不代表整段程序永遠不讀。版控 `apps/moo2/tools/startup_probe_131.py --dta-find` 重生私有 `dta-find-registers.json` SHA-256 `1416ebb092555aa7c40a64ef7972caff19b311934797d8792e9fe7b5b18c529e`、`dta-find-after.bin` SHA-256 `3a702e4d5eedf367556099638460764faa3d587607db9c313319c3ceb1c8f1a7`、`dta-find-next-logcpu.txt` SHA-256 `a120eceae2a33819ceb3a6fe2d2c8b2b50e82e2e5604314337745488a55c1060`。

**受控檔案存在樣本，非正常安裝**：同一原版 EXE 外，僅在一次性容器新增零位元組 `MOX.SET`，SHA-256 `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`，修改時間固定 1996-01-01 00:00:00 UTC。版控 `--dta-find-present` 重生後，原版返回 EAX=`00380000h`、EFLAGS=`0246h`；DTA `+15h` 屬性 `20h`、`+16h` 時間 `0000h`、`+18h` 日期 `2021h`、`+1Ah` 大小 0、`+1Eh` 名稱 `MOX.SET\0`，搜尋標頭與缺檔樣本相同。後續有界指令沿清 CF 路徑，從 DTA 名稱區消費字元。私有 `dta-find-present-registers.json` SHA-256 `ca68fb5618b84b667b749e5fe6874fc2c42a26e3b460c13d6c00cd71a1898e2b`、`dta-find-present-after.bin` SHA-256 `a36f4b577def2164c9b875189aaf1519863de0b7d115f0c7ecaac4f389effe4e`、`dta-find-present-next-logcpu.txt` SHA-256 `2a7c41a13ce549af44afff0e9da2943d434a15141f76902652f13342a80b4b7a`。原始 EXE 與完整擷取均留在未版控工作區。

## 擬議契約與限制

證據審查：原版兩種結果為「已證實」的單一路徑收據；[DOSBox-X 目前的 `DOS_DTA::SetupSearch` 與 `SetResult`](https://github.com/joncampbell123/dosbox-x/blob/master/src/dos/dos_classes.cpp) 顯示前者只重設 11-byte 搜尋樣式並寫 drive／attribute，後者只在成功時填結果欄位。因此「缺檔保留既有結果欄」屬來源支持的**強推論**，回放時仍須以非零舊欄位測試。8.3 精確檔名以外、其他屬性與時區均為**未知**，不進此切片。

只處理 `AH=4Eh`、CX=`0`、無路徑分隔／萬用字元的單一 8.3 精確檔名。從 DS:EDX 有界讀 ASCIIZ，存取前由已設定的 DTA selector／32 位偏移讀取 43 bytes；描述子／匯流排越界、名稱非法、提供者錯誤及檔案存在但無法取得 metadata 時失敗即關閉。唯讀提供者為 `nil` 是診斷用空目錄；有提供者時以既有安全的大小寫不敏感 `OpenRead` 解析檔名。

缺檔時，依已觀測形狀在 DTA 的 12-byte 搜尋標頭寫目前 C: drive `02h` 與 8.3 名稱，其他 31 bytes 保留；EAX=`00000012h`、只設 CF。存在時，在同一 DTA 填 DOS 屬性 `20h`、檔案時間／日期、32 位大小與大寫 ASCIIZ 名稱；EAX 低 16 位清零但高 16 位保留、清 CF。時間採輸入檔修改時間的 UTC 打包作**明示環境近似**，不宣稱其他時區逐位一致。檔案已開啟但 metadata 不可得時拒絕；不加入寫檔、萬用字元、列舉順序或 Find Next。兩種返回都不改 DS、DX、BX、CX、ESP。

合成測試須核對上述兩個收據形狀、DTA 指標早於 DS 改動、缺檔時舊結果區保留、檔案存在時大小／日期／名稱、無效樣式與越界失敗即關閉。固定原檔診斷需越過第 4066 步並明列下一停點；CPU／machine 與全套 Go 測試須通過。這僅是受保護模式檔案服務工具切片，沒有改 remake 玩法、UI、資料或存檔；合成 PSP／環境與完整原版資料仍待對齊。

## 實作與有限驗收

`internal/machine/le_startup.go` 接入受限 `AH=4Eh`；`le_startup_test.go` 以非零舊結果欄驗證缺檔時保留，並以修改時間固定的空 `MOX.SET` 核對成功 DTA。`go test -buildvcs=false ./internal/machine -count=1`、固定原檔輸入的 `go test -buildvcs=false ./... -count=1` 均通過。已綁定 DPMI 的合成 PSP／環境診斷從第 4066 步進至第 4168 步，在 **dosgolem 重定位 LE 線性位址** `0x148224`、bytes `38 10 A8 03` 的 `CMP r/m8,r8` 不支援處停下。這個步數只證明工具跨過目前服務邊界，沒有原版與 remake 的同狀態玩法收據。`CONFORMED` 僅適用上述受限路徑，不涵蓋實際遊戲資料目錄、任意 DOS 檔名或時區。
