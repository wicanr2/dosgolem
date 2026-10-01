# 235 — LE 映像與 DOS 低位記憶體隔離

狀態：**READY**（映射已實作；完整原檔第二筆配置尚待驗收）
日期：2026-10-01
範圍：隔離 dosgolem 的 DOS/4GW LE 載入與 DPMI `INT 31h/AX=0100h` 可用記憶體配置。這是執行器平台映射，不改 MOO2 玩法與資料。

## RE／平台證據

- **已證實，原版輔助執行**：固定官方 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`、`MOX.SET` SHA-256 `bfd6855a41760b31156b96114b5b33c88f442ab8f8aae020c1740b3b486a3a80` 與正版 ZIP 根層 417 檔。DOSBox-X 2026.07.02 SDL2 heavy debugger 映像 ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`；其 **CS:EIP** `0180:00380315 → 00380317` 首次 `AX=0100h` 請求 BX=`0201h`，返回 AX=`0FE3h`、DX 低 word=`01D8h`、CF=0；第二次請求 `00B0h` 也成功。私有 `workplace/dosbox-moo2/dos-memory-0100-registers.json` SHA-256 `1d96749e2d0811ffc506f228c553f8cd0976e24c4a0e8341b6bf2525b1b30600`。這是 DOSBox-X 輔助基準，非 dosgolem 正式對拍。
- **已證實，合成執行器**：同一輸入的 dosgolem 在**重定位 LE 線性位址** `0x15C317` 對首筆 513 段落請求回 `8013h`、CF=1、BX=0；第 3,947,961 步退出時線性仍餘 39,150,168 bytes，DOS 空間為零。私有 `workplace/moo2-probe-234-dos-alloc.txt` SHA-256 `efe498bae3d15aab2b87bfcd1b8a5982341e3c94b56e142f8632b0d5c26a6942`，完整探針 SHA-256 `843006eb6fc957cc8d8326fd9c6a1c825a534220ae9e4da663adc480f109611b`。`LoadLEInMZ` 將 LE 物件映射在 `0x10000` 起，映像長 1,891,536 bytes；`DPMIHost.setLimits` 的 DOS 游標從映像尾起算，超過 640 KiB。
- **已證實，平台契約**：[DPMI 1.0 `0100h`，PDF 第 72 頁](https://docs.pcjs.org/specs/dpmi/1991_03_12-DPMI_Spec_v10.pdf)要求返回可由實模式段與保護模式 selector 存取的同一個 DOS 記憶體區塊，配置失敗須回報最大可用段數。此規格不要求與 DOSBox-X 的具體段號／selector 值相同。
- **強推論，工程阻塞**：若只將現行 DOS 游標改小，第一次配置會寫到 LE 映像；必須同時移動 LE 映像並修正 LE fixup，或另設雙空間位址轉譯。現行 `LEMachine.Mem`、保護模式 CPU bus 與實模式 bus 都以同一份線性 bytes 存取，因此將 LE 物件重定位到 1 MiB 以上，可保留一致的物理記憶體視圖。

## 擬議契約

新增**明示選用**的內嵌 MZ/LE 載入入口：解析後計算最小 LE 物件基址；若低於 `0x100000`，以相同位移將所有 LE 物件基址移至不低於 `0x100000`，再執行既有 object fixup、載入 entry／stack。原始 EXE bytes 與原有 `LoadLEInMZ` 入口不變。無物件、位址加法溢位、移動後物件重疊或映射超界，失敗即關閉。明示入口在機器上設定 DOS arena 起點 `0x10000`；`DPMIHost.setLimits` 只在此模式使用該起點，止於 `0xA0000`。低位配置與高位 LE/線性配置共用 `Mem`，既有 `0100h` 返回實模式段與 selector 同指一份 bytes。`0501h/0502h` 規格 234 不變。`0x10000` 及 LE 高位映射是**平台規格近似**，不是原版精確物理地址或 DOSBox-X 段號。

## 驗收與邊界

1. 合成 LE fixture 驗證相同原檔的明示載入，entry、stack、含物件目標的 fixup 同位移，原 bytes 不變；既有載入入口行為不變，溢位拒絕。
2. 在明示載入機器上用 `0100h` 配出 513 段落；CF=0，實模式段與 selector 雙向讀寫同 bytes，且配置前後高位 LE entry／fixup bytes 相同；後續再配 176 段落仍成功。舊載入模式的測試與 DOS arena 行為不變。
3. 固定 1.31 原檔、真實 `MOX.SET` 與 ZIP 根層 417 檔案從 LE entry 自然跑，記錄第一次兩筆 `0100h` 的輸入／返回、下一個自然結果、未實作服務；以 dosgolem 自生收據作為執行器進展。沒有正常玩家畫面、同狀態或相同輸入之前不得宣稱玩法對拍。

## READY 證據審查

已核對 `LEHeader.RelocatedObjectImages`：每筆支援的 object fixup 目標均由目標物件 `RelocationBase + TargetOffset` 計算；所有物件同移位，內部相對偏移不變。`loadLEHeader` 也由相同 object 基址建置映像、entry 與 stack。`DPMIHost.Attach` 只呼叫 `setLimits` 設定游標，能依明示機器模式選擇 DOS 起點；既有 `dpmiRealBus` 與保護模式 bus 共用 `LEMachine.Mem`，可用同一映像維持段／selector 一致。原版檔案是否含未標 fixup 的絕對常數**未知**，列為固定原檔自然執行的驗收／停止條件，不把重定位通過合成測試直接寫成原版啟動成功。原檔 `InspectLEInMZ` 與現有 MOO2 入口測試已證實最小物件基址 `0x10000`、真入口 bytes `EB 76 WATCOM`；規格限定明示入口，原有 loader／FD2 預設路徑保持。這些證據足以決定輸入、映射、拒絕與驗收，故可進入受限實作。

## 已驗收部分與下一停止點

明示 `LoadLEInMZWithDOSArena` 將固定 1.31 LE 映像與所有支援的 object fixup 一致平移 `0xF0000`，entry 從 `0x10FF18` 到**dosgolem 高位重定位 LE 線性位址** `0x1FFF18`。合成測試證明 `0100h` 先後配置 513、176 段落成功，selector 與實模式 bus 雙向讀寫同一資料，LE entry bytes 未被覆蓋，原載入入口不變，零物件與平移溢位拒絕。固定原檔測試遍歷既有 LE fixup 並核對平移後目標。`DOSGOLEM_MOO2_EXE=/tmp/ORION2.EXE go test -buildvcs=false ./... -count=1` 全通過；私有 `workplace/full-test-235.txt` SHA-256 `6ed111b7c3a51af6330c947b8a447ce2a065fcc5f7c91da5137040c8f2582de1`。

以同一正版根層 417 檔與 1.31 EXE 的明示高位診斷，dosgolem 於第 189,580 步首筆 `0100h` 成功：請求 513 段落，返回 AX=`1000h`、DX=`0100h`、CF=0；此數值為合成配置，**不冒充原版段號**。第 189,582 步在 `INT 31h/AX=0300h` 停下，`RealModeLast` 顯示 `Interrupt:16`、`Entry:0000:0000`、`Error:DPMI0300中斷10未註冊`；私有 `workplace/moo2-probe-235-real-mode.txt` SHA-256 `5dbf8b22cda56a2f77fd0ff498daa3dd5d1a14f7b194f074fe748013708023e8`。因此低位配置與 LE 不覆蓋已驗收，後續 `0300h/INT 10h` 仍是獨立平台服務缺口；第二筆 `0100h` 尚未在完整原檔自然抵達。沒有畫面、完整同狀態或玩法原版對拍。

驗收第 3 項的第二筆原檔配置尚未抵達，故本規格維持 READY；不能以合成雙筆測試代替原檔自然執行收據。
