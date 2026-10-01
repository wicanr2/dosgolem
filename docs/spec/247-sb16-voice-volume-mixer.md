# 247 — SB16 左右數位語音音量混音器暫存器

狀態：**CONFORMED（受限平台規格近似）**
日期：2026-10-01
範圍：dosgolem 固定 MOO2 1.31 音效初始化的 SB16 `32h/33h` 混音器索引讀寫；只保存原始暫存器狀態，無聲音輸出、音量聽感或逐週期硬體聲稱。

## 原始輸入與證據分級

- **已證實，合成原檔路徑**：官方 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`、正版 ZIP SHA-256 `3a28a52f5953ff6d8fc251548940500236752ee19b52b51581e71ec1a3373c2f`、`MOX.SET` SHA-256 `bfd6855a41760b31156b96114b5b33c88f442ab8f8aae020c1740b3b486a3a80`。隔離 dosgolem `642abfe`、`golang:1.24-bookworm` 的高位 LE／低位 DOS 合成初態，自 LE entry 自行執行至 **dosgolem 實模式** `1201:073F` 的 `IN 0225h`；前一筆 `OUT 0224h,32h`，混音器索引 `32h`。私有正式收據 `workplace/moo2-probe-246-full-game.txt.gz` SHA-256 `0b3d80dc9aba63f4973089d855384e24080d8e0f529f18f4caca31e0b508667c`；可丟棄埠狀態探針 `workplace/moo2-probe-247-mixer-diagnostic.txt.gz` SHA-256 `0b8ea234f8b901976818c4acf1836958ca993ee5bb12a3733dbd329e3c1b11b0`。原版實機讀值與後續寫入尚未另行量測。
- **已證實，公開硬體契約**：[Creative《Sound Blaster Series Hardware Programming Guide》第 4 章 CT1745 混音器](https://www.ardent-tool.com/sound/Sound_Blaster_HW_Programming_Guide_1st.pdf)標 `32h/33h` 為左／右數位語音音量，D7–D3 為五位音量，D2–D0 保留，預設量級為 24/31，對應暫存器高位 `C0h`；寫 `00h` 混音器重設暫存器才恢復各預設。本切片不補混音器重設指令。
- **強推論**：MOO2 於此讀取目的在保存／調整左聲道音量；尚未追後續消費端。這一解釋不作實作前提。**未知**：原版物理卡的當次使用者混音器設定、正式遊戲讀值、聲波與聽感；合成初態僅按原廠預設。

## 受限契約

既有 `224h` 索引選擇不變。新增 `225h` 在索引 `32h`／`33h` 的讀寫：初始原始高五位為 `C0h`，讀取回目前保存值；寫入只保留 D7–D3，低三位視為保留位不留狀態。兩聲道互相獨立。DSP `226h` 重設只重設 DSP，不偷改混音器；其他未知索引／寫入維持失敗即關閉。這只改 dosgolem 平台狀態，不碰 MOO2 Go remake 玩法或存檔。

## READY 證據審查

固定原檔的實際索引 `32h` 與讀埠 `225h` 已在合成初態自行產生；索引 `32h/33h` 的左右欄位、有效位元與預設值可由 Creative 原廠指南直接讀得。原版物理卡音量未知，故只採原廠重設預設作合成初態，不宣稱原版當次值。狀態轉移、未知索引拒絕及自然後續停點均可測，DRAFT 所需的行為邊界已足夠明確，轉 READY。後續若原檔寫其他索引或依賴 mixer reset，另立規格。

## 驗收與停止線

合成測試覆蓋預設讀值、左右獨立寫入、保留位遮罩、DSP 重設不影響混音器、未知索引拒絕。固定官方 EXE 全套 Go 測試通過；正版資料由 dosgolem 原檔入口自然越過 `32h` 讀取並記錄下一個停點。若後續需要其他混音器功能，另核對其實際索引與公開契約，不以通用預設值放行。這是 **hardware-spec approximation**，不是原版音量或玩家路徑同狀態對拍。

## CONFORMED 收據與限制

固定 1.31 EXE 全套 `go test -buildvcs=false ./... -count=1` 通過，私有 `workplace/full-test-247.txt` SHA-256 `fe95d9119cde6fb624038c04a132ff9df0694f0031b3e5928c3d78c329d23147`。正版資料自 LE entry 自然越過混音器 `32h` 讀取，`DPMI 0300h → INT 66h` 於實模式 135 步後返回，下一停點為 **dosgolem 高位 LE 線性位址** `0x25221E` 的 `83 C8 10`，第 1,170,054 步；私有 `workplace/moo2-probe-247-full-game.txt.gz` SHA-256 `ad3568f83e7a85f70e682484bf392bac3bcd66dce42fd474e1d061b6af3083ca`。這只驗收受限混音器平台契約和執行器自然前進；正式原版卡當次音量、聲波、畫面及玩法同狀態仍未知。
