# 258 — MOO2 在 DOS 環境查詢 Windows 版本

狀態：**CONFORMED（限定未安裝 Windows 的平台查詢）**
日期：2026-10-01
範圍：明示 MOO2 DOS 平台的 `INT 2Fh/AX=160Ah`；不模擬 Windows 內部，不修改 remake 玩法。

## 證據與審查

- **已證實，自生停點**：隔離 dosgolem `7a50ae93495ed70d00e5fb2be1e129a569c5ef5f`、Go 1.24.13，官方 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`。無事件路徑第 6,216,999 步在 **dosgolem 高位 LE 線性** `0x217888` 的 `CD 2F` 拒絕：EAX=`160Ah`、EBX／ECX=`0`、EDX=`1608Ah`、EFLAGS=`246h`。輸入與先前收據見 [257](257-moo2-protected-mouse-position-setting.md)。
- **已證實，原版同次返回**：DOSBox-X 2026.07.02 SDL2 heavy debugger，既有映像 `fd2-dosbox-x:debug-0d7b272b`；`apps/moo2/tools/startup_probe_131.py --windows-version`。**DOSBox-X CS:EIP** `0180:0034B888`，原始 bytes `CD 2F 83 F8 00 75 0D 89 1D 7C A5 39 00 61 A1 7C`。下一指令 `0180:0034B88A` 的全部擷取欄位保持：EAX=`160A`、EBX／ECX=`0`、EDX=`1608A`、ESI=`3E0151`、EDI=`3A2090`、EBP=`3EBC06`、DS／ES／SS=`188`、FS=`0`、GS=`20`、ESP=`3EBBAC`、EFLAGS=`246`。caller 在 `0034B88D` 跳至 `0034B89C`，略過 Windows 版本保存分支；恢復一般暫存器後返回零，繼續初始化。這是 DOSBox-X 輔助基準，不是正式 dosgolem 正常玩家收據。
- 私有 JSON SHA-256 `a35f451af240449b66a8762a05d6798bd4dc49a901fa53b28490e83e92d7904b`、caller 64 指令 LOG SHA-256 `6d787297c2f2aa296b7c4681c6d53fcf0a79bac9749f096dbdb94b8b7ae0a001`、終端 SHA-256 `fc0d6f75a86828ed1c4aabfbf4e7ef04209fe869f51d948c21b5f6f246bf2a77`。工具映像與 ZIP／固定 MOX.SET 雜湊沿用 [255](255-moo2-protected-mouse-callback.md)。兩個工具的位址與資料定位分開記錄，不把重定位後的 bytes 冒稱相同。
- **公開平台契約**：[DOSBox-X `DOS_MultiplexFunctions` 原始碼](https://dosbox-x.com/doxygen/html/dos__misc_8cpp_source.html) 的 `case 0x160A`，僅在 NESTICLE 等特例報告 Windows；一般 DOS 環境未接管，不修改暫存器。MOO2 的實際同次返回與此相符。此窄範圍已足以停止平台追查，不逆向 Windows 驅動或多工鏈內部。

DRAFT 審查結果：原版返回、caller 分支與成熟模擬器公開平台契約相符，接受「未安裝 Windows」的限定環境，轉 READY。不是忽略所有 `INT 2Fh`，不製造 AX=0 的 Windows 成功返回。

## 契約與驗收

只在明示 MOO2 設定且低 AX=`160Ah` 接受此查詢。完整一般暫存器、段、EIP、旗標與記憶體保持；不更動 DOS 版本查詢計數。其他設定及所有未實作多工功能仍拒絕。高 AX 不參與 16 位功能選擇，不能抹掉高位。

測試實際輸入與高 AX 雜訊、重複呼叫、完整架構欄位／記憶體保持、其他設定與未知功能拒絕。固定原檔全套測試後，以相同資料重跑無事件及設定後受控事件兩條自然啟動路徑。沒有正常玩家畫面、受控亂數或 remake 對拍時，不把平台通過寫成遊戲完成。

## CONFORMED 收據

限定查詢與其他設定／未知功能拒絕測試通過。固定 EXE 全套 `go test -buildvcs=false ./... -count=1` 通過，私有 `workplace/full-test-258.txt` SHA-256 `1a7ddbe2c287b573ea39b942b954bf9a69ec5a95e7c1842b060c6edd0241196c`。首次容器命令因巢狀 shell 引號在啟動前被拒絕，修正 heredoc 引號後同一映像乾淨重跑，沒有產品失敗。測試初稿以陣列順序配置暫存器時把 ECX 填成 ESI 的值，收尾改為具名索引以符合捕獲初態，另以 `GOMAXPROCS=2 go test -p 2 -buildvcs=false ./internal/machine -run TestMOO2WindowsVersionAbsentPreservesState -count=1` 通過；首次單項重跑因程序上限不足失敗，限制平行度後解決。不影響已跑的兩條原檔輸入。收尾診斷改逐行串流讀取，避免整份解壓至記憶體造成容器超限；Python 語法與兩筆停點核對通過。

兩條原檔路徑自行越過 Windows 版本查詢，無事件第 6,217,167 步、設定後受控事件第 6,217,202 步停於 **dosgolem 高位 LE 線性** `0x24C315` 的 `CD 31`，AX=`0500h`，屬下一個未支援的平台查詢。無事件診斷 `workplace/moo2-probe-258-full-game.txt.gz` SHA-256 `3522de797c533648583a633a451530536bdc43a6b945edadd5ee76a7341f31d1`，受控事件診斷 SHA-256 `9fd435c4c06ca6ee936bebfdee48efb0a6eeca360ccc328bee44e642e2303743`。事件路徑仍須核對完整座標／游標 consumer；規格 255 不因越過此查詢升格。沒有正常玩家畫面或玩法同狀態完成收據。
