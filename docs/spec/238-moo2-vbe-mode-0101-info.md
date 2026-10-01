# 238 — MOO2 查詢 VBE 模式 0101h

狀態：**CONFORMED（固定模式資訊，非畫面對拍）**
日期：2026-10-01
範圍：只在隔離 dosgolem 的 MOO2 啟動設定處理 DPMI `0300h → INT 10h/AX=4F01h`，查詢 `CX=0101h`。模式設定、顯示記憶體與畫面輸出不在本規格。

## RE／平台證據與來源

- **已證實，固定原版的 DOSBox-X 輔助執行**：官方 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`，正版 ZIP SHA-256 `3a28a52f5953ff6d8fc251548940500236752ee19b52b51581e71ec1a3373c2f` 的根層 417 檔及 `MOX.SET` SHA-256 `bfd6855a41760b31156b96114b5b33c88f442ab8f8aae020c1740b3b486a3a80`。工具為 DOSBox-X 2026.07.02 SDL2 heavy debugger，映像 ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`。版控 `apps/moo2/tools/startup_probe_131.py --real-video-4f01` 的私有暫存器收據 `real-video-4f01-registers.json` SHA-256 `baafd3f5178954e1d5d24224bf7e98233f62b701cda0088c2554c40f018a1454`。
- **已證實，呼叫序及位址基準**：在 **DOSBox-X CS:EIP** `0180:00380315 → 0180:00380317`，兩次 DPMI `0300h/INT 10h` 封包的 AX 均為 `4F00h`，第三次才是 AX=`4F01h`。第三次輸入 CX=`0101h`、ES=`0FE3h`、DI=`0`、flags=`0`，返回 AX=`004Fh`、flags=`0002h`、CX 與其他封包欄位保持；外層 DPMI EAX=`0300h`、CF=0。第二次 `4F00h` 是原版實際執行，不得把前兩次合併為一筆。
- **已證實，記憶體效果**：**DOSBox-X 實模式物理線性位址** `0xFE30` 的 256 位元組在呼叫前全零；呼叫後前 44 位元組為 `9b0007004000400000a00000970100c080028002e001081001080104000501000000000000000000e0`，第 44–255 位元組保持零。前段包含 640×480 解析度等 VBE 模式欄位；這是 DOSBox-X 虛擬硬體型態的原版輸入環境收據，不是 MOO2 自帶素材或通用真機定值。私有輸入／輸出 SHA-256 分別為 `5341e6b2646979a70e57653007a1f310169421ec9bdd9f1a5648f75ade005af1`、`d0eafd13c290b998a4d0d3be332d9be06eabde2a19764f88ab68d483e64e35c3`；caller LOG SHA-256 `9a541abd20036f3f77f1b17e92d6c5e5290e0b793b9f7ca2ddf788278a85b158`。
- **已證實，平台契約**：[VESA VBE Core Functions 2.0 Rev 1.1，Function 01h](https://www.phatcode.net/res/221/files/vbe20.pdf)定義 `AX=4F01h`、`CX=模式號`、`ES:DI` 指向模式資訊結構，成功回 `AX=004Fh`。本規格的逐位元組回傳僅以本次 DOSBox-X 輔助執行為依據。
- **已證實，dosgolem 自生停點**：規格 237 後，官方 EXE／正版資料在明示高位 LE 載入模式自然跑至第 190,214 步，停在 DPMI `0300h → INT 10h/AX=4F01h`；私有 `workplace/moo2-probe-237-full-game.txt` SHA-256 `a95ad7d1ea617396f8a4276bf1984a0a493f8c756667a20bdbb74c79c167d148`。兩側 PSP／環境、堆疊及完整狀態尚不同。

## 擬議受限契約與 READY 審查

在現有 MOO2 專屬 BIOS 回呼中，僅接受 `INT=10h、AX=4F01h、CX=0101h`，且目標 `ES:DI` 指向至少 256 位元組的活 DOS 區塊，位於 VGA 以下。回寫上述 44 位元組，保留緩衝第 44–255 位元組，封包只改 AX=`004Fh` 與 flags=`0002h`。輸入 CX、其他暫存器與段不改；未配置／太小緩衝、其他模式與未知功能都失敗即關閉。`4F00h` 原有服務及一般 FD2 設定不受影響。

現有 `moo2VBEControllerInfo` 只接受 `4F00h`，且已有活 DOS 區塊與低位地址檢查；可在同一專屬回呼加入 `4F01h` 分支，共用記憶體邊界檢查。現有測試將「未知功能」改用真正未知值，另加模式與緩衝保留測試。此證據足以實作固定模式查詢；通用 VBE BIOS、原版畫面及玩法同狀態仍未知。

## 驗收

1. 合成測試核對完整 256 位元組與封包返回、尾部保留、其他模式／小或未配置緩衝拒絕，以及原 `4F00h` 回歸。
2. 固定官方 EXE 的全套 Go 測試通過；完整正版資料由 dosgolem 自行越過第 190,214 步，記錄下一自然停點。
3. 僅符合上述範圍可改為 CONFORMED，不把 DOSBox-X 收據單獨當成正式原版同狀態對拍。

## 實作與驗收

- `internal/machine/moo2_vbe.go` 在 MOO2 專屬回呼處理模式 `0101h`；`internal/machine/dpmi_real_mode.go` 以回寫前的 AX 標記服務，避免把成功返回 `004Fh` 誤記為功能號。拒絕條件與 `4F00h` 回歸均有測試。
- `TestMOO2VBEMode0101MatchesOriginalBuffer` 對完整 256 位元組做原版 DOSBox-X 緩衝 SHA-256 比對，並以非零尾端樣本驗證第 44–255 位元組保持。含官方 1.31 EXE 的 `go test -buildvcs=false ./... -count=1` 全通過；私有 `workplace/full-test-238.txt` SHA-256 `84406686fb97b0c27f8aa42fe41190c08bc256c6f375a45c7b71b5ec94e4088d`。
- dosgolem 從同一 EXE、正版 ZIP 根層 417 檔及明示高位 LE 載入自行越過第 190,214 步；第 190,517 步的**dosgolem 高位重定位 LE 線性位址** `0x24C2B2` 停於直接 `INT 10h/AX=4F02h`，輸入 EBX=`0101h`。私有 `workplace/moo2-probe-238-full-game.txt` SHA-256 `6f5df2e26b65c5935db63cff4751b103182ff75bab09c18314d8a42ed2c70284`。此為執行器重生的受限平台進展，沒有畫面或玩法同狀態收據；下一筆模式設定須另行擷取原版返回與 consumer。
