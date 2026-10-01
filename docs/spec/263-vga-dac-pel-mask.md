# 263 — VGA DAC 像素遮罩

狀態：**CONFORMED（標準遮罩的硬體規格近似）**
日期：2026-10-01
範圍：標準 VGA 的 `03C6h` 位元組埠、查色消費端與機器狀態保存；不修改 remake 玩法。

## 證據與定位

- **已證實，自生停點**：dosgolem `f9e043fb41de8fff6e2ec3b49b0fd0695712af12`、Go 1.24.13，官方 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`。固定 ZIP 根層 417 檔與 MOX.SET，無事件第 6,728,298 步、受控事件第 6,728,333 步在 **dosgolem 高位 LE 線性** `0x222C9E` 的 `EE E8 9D FE FF FF 66 BA C8 03 8B 35 80 38 2A 00` 拒絕；`OUT DX,AL`，DX=`03C6h`、AL=`FFh`、EFLAGS=`246h`。原始輸入、映像與診斷雜湊見 [規格 262](262-cpu386-test-word-register.md)。
- **公開平台契約**：[IBM VGA／XGA 技術參考，1992-05，2-104 至 2-106](https://bitsavers.trailing-edge.com/pdf/ibm/pc/cards/IBM_VGA_XGA_Technical_Reference_Manual_May92.pdf) 列 `03C6h` 像素遮罩、6 位元 RGB 資料與模式設定初始化 `FFh`。表格將遮罩列為讀取，並警告寫入可能改變調色盤；不把該表格單獨解讀成完整可寫語意。
- [DOSBox-X 公開 VGA DAC 模型](https://dosbox-x.com/doxygen/html/vga__dac_8cpp_source.html) 的埠說明及 `read_p3c6`／`write_p3c6`／查色消費端交叉確認：遮罩可讀寫，輸入 DAC 索引與遮罩做 AND 後選色。只沿用公開介面契約，不複製實作控制流。

## 擬議契約

`Machine` 新增像素遮罩，建立與支援的模式設定時為 `FFh`。`In8(03C6h)` 回目前值；`Out8(03C6h,value)` 保存全部八位，不改原始 DAC、RGB 寫入索引／相位或 CPU。`LEOPLPorts` 明確接通這個埠的兩個方向並記錄存取；未知埠維持拒絕。

`Palette()` 對每個輸入 DAC 色號，以 `index & mask` 查原始 DAC，再沿既有 6→8 位展開。平面模式先經屬性控制器的 DACIndex，再消費這份查色表；LE 的既有 Palette 路徑也必須生效。既有 `03C8h`／`03C9h` RGB 寫入次序及索引自增不變。

記憶體快照與 gob 機器存檔都保存遮罩。gob 版本 2 加入明示存在欄位：新檔可區分合法遮罩 0 與未保存的舊檔；舊檔缺欄位時使用 `FFh`，保留舊模型的查色行為，不從舊 Ports 猜測未建模的遮罩。

本契約標為 **hardware-spec approximation（硬體規格近似）**：不建模 PEL 時鐘、類比輸出、回掃雪點、SVGA 隱藏 RAMDAC 暫存器或未觀測的 DAC 讀取週期。它只接通已觀測的標準平台依賴，不代表 MOO2 正常畫面或 remake 同狀態完成。

## 驗收計畫

先審查來源與既有消費端，再轉 READY 後實作。核對全部色號與遮罩 0／FF／非連續位、原始 DAC 不變、RGB 中途改遮罩不打斷、模式重設、兩種保存／還原及舊版本 2 缺欄位相容。沿正常 Palette／平面 RGB／LE Palette 消費端抽測，不以單純接受 OUT 代替色彩效果。

DOSBox-X 原版有界輔助探針核對候選 `0180:00356C9E` 的實際 bytes、DX／AL 與執行後下一指令；候選位址在實際命中前只是假說。正式下一停點由 dosgolem 的無事件與受控事件兩條自然路徑自行重生。固定 EXE 全套測試、解析回填正常／負向護欄與擁有權／容器清理一併驗證。

READY 審查：IBM 的初始化與 RGB 契約、成熟模擬器的標準遮罩介面已互相核對；既有 `Palette()`、`PlanarRGB()` 及 `LEVideo.Palette()` 有實際消費端。新狀態的快照／存檔與舊檔預設值已明定，未使用 SVGA 特例或遊戲 driver 假說。原檔自然停點足以證明標準埠依賴，輔助探針僅核對該使用點；由 DRAFT 轉 READY 後實作。模式重設包含 `Machine.SetVideoMode` 與既有 LE mode13 入口，MOO2 的其他 BIOS／VESA 子集仍依其既有契約。

## 原版輔助樣本

**已證實，同次使用點**：DOSBox-X 2026.07.02 SDL2 heavy debugger，映像 `fd2-dosbox-x:debug-0d7b272b` ID=`sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`。`startup_probe_131.py --vga-pel-mask` 以相同原檔與 MOX.SET 命中 **DOSBox-X CS:EIP** `0180:00356C9E`，16 bytes=`EE E8 9D FE FF FF 66 BA C8 03 8B 35 80 18 3D 00`；前十 bytes 與高位 LE 一致，後續絕對位址因重定位不同，不宣稱整個窗口相等。

EAX=`FFh`、EBX=`A0h`、ECX=0、EDX=`3C6h`、ESI=0、EDI=`3A2090h`、EBP=`3EBC06h`、ESP=`3EBBA0h`；DS／ES／SS=`188h`、FS=0、GS=`20h`、EFLAGS=`246h`。同次下一指令 `0180:00356C9F` 的一般暫存器、段與旗標保持。探針沒有從 debugger 額外讀埠，不引入 SVGA RAMDAC 計數器副作用；非 FF 遮罩以公開平台契約驗證，不冒稱 MOO2 動態使用過。

私有 JSON SHA-256 `959fa5f9e9cc867d470a937469bd7669bd10bb8de81496cd1131241f5faef267`、後續 LOG `933bff7d2cc559e17f97dfc64f24028cfcd17f0414785a596c9ee47235891216`、終端 `6d2db41eaa5b77c30389809495c1aad11c278e98487f7be05086876ade2ed8fd`。DOSBox-X 只作輔助樣本；正式自然路徑由 dosgolem 重生。

解析回填不可變鍵：固定 EXE 雜湊＋dosgolem 高位 LE `0x222C9E`／DOSBox-X `0180:00356C9E`＋`EE`＋DX=`03C6h`、AL=`FFh`。規格 262 必須保留「VGA 像素遮罩停點已由規格 263 接通」與本檔連結；`startup_probe_131.py --check-vga-pel-mask-spec-backlinks` 核對來源與狀態，缺舊標記必拒絕。

## 實作與驗收

依 READY 契約接入 `machine.go`／`le_opl_ports.go`，`bda.go` 與 `le_video.go` 支援的模式初始化恢復 `FFh`；`snapshot.go`／`state.go` 保存合法 0 遮罩並相容缺欄位的 v2 舊檔。`vga_pel_mask_test.go` 驗證所有 256 色號與五組遮罩、原始 DAC 保持、RGB 中途讀寫遮罩、255→0 索引環繞、平面屬性索引後的 RGB、LE Palette、音訊隔離及兩種還原／舊格式。未知 `03C7h` 仍拒絕；新狀態沒有擴大原版 BIOS／VESA 契約。

Go 1.24.13，映像 `golang:1.24-bookworm` ID=`sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac`。`go test -p 2 -buildvcs=false ./internal/machine -run 'Test(VGA|LEVGAPELMask)' -count=1` 通過，輸出 SHA-256 `ba6cdc65bff8c8fc6aa2474276d6eb7159f74a8cbc7140e81e8797f2a7254d7f`。固定原檔 `DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE GOMAXPROCS=2 go test -p 2 -buildvcs=false ./... -count=1` 全套通過，`workplace/full-test-263.txt` SHA-256 `3255ab2d5d3bb47cc53fd0739430536048704cc1fcf602342ac88c8bedfec559`。

**已證實，自生下一停點**：同樣分離 DOS arena、完整原檔與 MOX.SET，`DOSGOLEM_MOO2_SEPARATE_DOS=1 go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game` 在無事件第 6,728,365 步、設定後受控事件第 6,728,400 步，均停於 **dosgolem 高位 LE 線性** `0x222CCB` 的 `F6 F3 EE AC F6 E7 B3 64 F6 F3 EE AC F6 E7 B3 64`。當前 byte 暫存器 DIV 形式未接通；通用錯誤訊息寫「F6 記憶體形狀未支援」，ModRM=`F3h` 實際為暫存器，不誤分類為記憶體存取。拒絕後 EIP=`0x222CCD`、EAX=0、EBX=`64h`、ECX=`80h`、EDX=`3C9h`、DS／ES／SS=`188h`、EFLAGS=`6h`、DOS 呼叫 6；受控回呼 started=1／completed=1。

兩份診斷 `workplace/moo2-probe-263-full-game.txt.gz`／`workplace/moo2-probe-263-mouse-event.txt.gz` 的 SHA-256 分別 `a5836830e9c15f632e22f7a110de950604f7cea87b8d3076c693d3d892ef86ba`／`9adbbef63c5fa11f133f1f16c9785a0d5a4efc5d755b4cc94b8e2b3da4cb8df3`。Python 語法、索引、既有回填與新護欄正常／刪除舊標記／刪除原始定位必拒絕通過。本規格只在標準遮罩的硬體規格近似範圍 CONFORMED，255 仍 READY；MOO2 正常畫面、音效、亂數與 Go remake 玩法同狀態仍未驗證。

下一最小行動：核對公開 CPU byte DIV 契約、既有 F6／暫存器解碼與固定原版使用點，審查窄規格後接通並重跑。停止於足以通過啟動的平台證據，不反組譯顯示 driver 內部。
