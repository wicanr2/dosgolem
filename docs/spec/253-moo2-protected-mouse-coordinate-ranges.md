# 253 — MOO2 保護模式滑鼠座標範圍

狀態：**CONFORMED（受控座標範圍平台模型）**
日期：2026-10-01
範圍：MOO2 啟動的 `INT 33h/AX=0007h／0008h`；不改 Go remake 玩法與正式操作。

## 證據

- **已證實，自生停點**：隔離 dosgolem `792b8d077d1d4611ae8246ddc9d61129e316b67a`，Go 1.24.13／`golang:1.24-bookworm`；官方 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`，正版 ZIP SHA-256 `3a28a52f5953ff6d8fc251548940500236752ee19b52b51581e71ec1a3373c2f` 根層 417 檔及 `MOX.SET` SHA-256 `bfd6855a41760b31156b96114b5b33c88f442ab8f8aae020c1740b3b486a3a80`。高位 LE／低位 DOS arena、合成 PSP／環境；第 1,545,719 步在 **dosgolem 高位 LE 線性** `0x24C31B` 的 `CD 33`，AX=`0007h`、CX=`0`、DX=`04FEh`（1278）、EFLAGS=`0012h` 拒絕。私有輸出 `workplace/moo2-probe-252-full-game.txt.gz` SHA-256 `d630b6db5fc796dc91164b3e15219083ada2f8aed8bf0a7e263eb6b64708e666`。
- **公開平台契約**：[DOSBox-X 滑鼠來源](https://dosbox-x.com/doxygen/html/mouse_8cpp_source.html) 的 `INT33_Handler case 07h／08h` 以有號 16 位 CX／DX 取較小、較大值為包含端點的範圍，保存並限制目前座標；不改暫存器。VESA 主機游標整合與粒度另有處理，不納入本受控模型。
- **已證實，原版水平樣本**：DOSBox-X 2026.07.02 SDL2 heavy debugger，映像 `fd2-dosbox-x:debug-0d7b272b` ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`，版控探針 `apps/moo2/tools/startup_probe_131.py --mouse-horizontal-range`。**DOSBox-X CS:EIP** `0180:0038031B` 原始 `CD 33 C3`；輸入 AX=`7`、BX／CX=`0`、DX=`04FEh`、ESI／EDI=`0`、DS／ES／SS=`0188h`、ESP=`003EBB58h`、EFLAGS=`0016h`。同次下一指令 `0180:0038031D` 擷取欄位完全保持。`0180:003801B8..003801C0` 將 EAX／EBX／ECX／EDX 存到 record `003D18E0h +0/+4/+8/+0Ch`，`0180:003475BB..003475DD` 接著準備 `AX=8`、CX=`0`、DX=`01DFh`（由高度 480 減一）。私有 JSON SHA-256 `4fb53b8b5813d60729960cb46cc8dd7cae6feeb62105467a62430de98ea9c998`、caller LOG SHA-256 `f9f230d1c5b1a75a89917dda9b02f4eb27bcb5d4d1951046c8f3dd75c26774ba`、終端 SHA-256 `4b8df77b6b79d7b4f10f00bc867f4892a33d4ee37cba420de614fc74f013537c`。
- **已證實，原版垂直樣本**：同工具與固定輸入，版控 `--mouse-vertical-range` 於同一 **DOSBox-X CS:EIP** 入口直接取得 `CD 33 C3`；AX=`8`、CX=`0`、DX=`01DFh`，其餘入口欄位同水平樣本，下一指令擷取欄位完全保持。相同 record 寫入後，`0180:003475F7..00347619` 準備 `AX=001Ah`、BX／CX=`0064h`（100）、DX=`01DFh`（479）。私有 JSON SHA-256 `bf3df1ba3de00614f1754f5b8fb9f73cf68b0a1cc4e909e34b89fd7f25676853`、caller LOG SHA-256 `fb71a9140aa4ecc56bdd9f15428c5d7be4c5eddc3f80da32ad7b3457d3f60d58`、終端 SHA-256 `8f1783e1c319338ecf1d5404a469735bd691bdde16e462b7f1786b405872ef95`。
- **未知**：PSP／環境、堆疊、完整旗標與粒度模型不同，不能宣稱完整狀態、畫面或玩法對拍。

## 擬議契約

只在明示 MOO2 設定接受功能 `7／8`，低 CX／DX 按有號 16 位正規化，保存該軸範圍並限制目前位置；兩軸互不影響。所有暫存器、高位、段、旗標、按鍵與敏感度保持。`SetMouseState` 的受控輸入也遵守已設定的範圍，`AX=3` 查詢可觀測狀態；`00h／21h` 重設清除自訂範圍並回既有模式中心。尚未設定範圍的受控注入沿既有行為。

範圍語意依平台來源，標為 **platform-spec approximation**；未有原版邊界座標實驗，不冒稱所有驅動的精確語意。主機游標比例、粒度、回呼、實體裝置與 IRQ 不猜補；其他功能及一般 FD2 設定仍拒絕。資料模型只在執行器內，Go remake UI／存檔不受影響；正常玩家路徑仍待完成。

## 驗收與停止線

兩個原版呼叫的同次入口／返回／record 消費端、合成兩軸端點／反序／負值／零寬範圍、欄位保持、受控輸入與查詢、重設清自訂範圍、一般 FD2 拒絕、固定 EXE 全套測試，以及正版資料從 LE entry 自然越過服務。範圍足以驗收後停止，不展開滑鼠驅動內部。原版 EXE、素材、記憶體與完整終端只留本機私有工作區。

## READY 審查

原版兩軸參數、同次欄位保持與 record 消費端已取得；平台來源足以界定有號排序、端點限制與重設移除自訂範圍。非零高位、反序、負值與邊界座標只依平台契約及合成測試驗收，明示為近似。受控注入未設定範圍時仍沿既有測試契約，不代表實體滑鼠預設範圍。據此 DRAFT 轉 READY；後續非零 `1Ah` 不在本規格猜補。

## CONFORMED 收據

兩軸原版參數、反序、負值、跨零、零寬及有號極值、端點／外側限制、受控輸入與 `AX=3` 查詢、兩軸互不改寫、兩種重設清自訂範圍、欄位保持與一般 FD2 拒絕均通過。固定 EXE 全套 `go test -buildvcs=false ./... -count=1` 私有 `workplace/full-test-253.txt` SHA-256 `261da4ea09ba0cbd7753a49d07ed0fabae28f1c470748a1e4f707edfa13a28d3`。正版資料從 LE entry 自然越過 `7／8`，第 1,545,911 步在 **dosgolem 高位 LE 線性** `0x24C31B` 的 `INT 33h/AX=001Ah` 拒絕；私有 `workplace/moo2-probe-253-full-game.txt.gz` SHA-256 `76521df20677fe92c685fe909a1fd13fdd41b25ffc2955051d4c9359d8ddd04d`。這是平台範圍服務驗收，不是正常玩家畫面、粒度或實體滑鼠對拍。
