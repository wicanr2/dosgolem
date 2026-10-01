# 265 — MOO2 VBE 顯存視窗控制

狀態：**CONFORMED（有限視窗與消費端，硬體規格近似）**
日期：2026-10-01
範圍：固定 MOO2 啟動設定、模式 `0101h` 的視窗 A、CPU／DPMI 顯存讀寫與零起點畫面消費；不修改 remake 玩法。

## 證據

- **已證實，自生停點**：dosgolem `39ac121bb2f2343809a9e3ecc67836a4d510faa1`、Go 1.24.13，官方 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`。完整 ZIP 根層 417 檔與固定 MOX.SET，無事件第 6,738,645 步、受控事件第 6,738,680 步，在 **dosgolem 高位 LE 線性** `0x228C54` 的 `CD 10 61 C3 60 25 FF FF 00 00 33 D2 BB 00 00 00` 拒絕；AX=`4F05h`、BX=CX=0、DX=5、EFLAGS=`246h`。輸入與診斷雜湊見 [規格 264](264-cpu386-div-byte-register.md)。
- **已證實，既有平台設定**：[238-moo2-vbe-mode-0101-info.md](238-moo2-vbe-mode-0101-info.md) 的同次模式資訊已反映在 `moo2VBEMode0101`：視窗 A 屬性 7、B 屬性 0、粒度／大小各 64 KiB、段 `A000h`、stride 640、640×480、8 位元 packed pixel、額外影像頁 5。控制器資訊的 TotalMemory=32 個 64 KiB 區塊，即 2 MiB。這是固定輔助環境設定，不冒稱真機通用數值。
- **公開平台契約**：[VESA VBE 2.0 Rev 1.1，Function 01h／02h／05h](https://www.phatcode.net/res/221/files/vbe20.pdf)，頁 20–21、25、27–28：模式資訊決定視窗位置／大小／粒度；`BH=0` 設定、`BH=1` 讀回，`BL=0` 視窗 A，DX 以粒度為單位。`0101h` 未設保存位，重設時清已報告影像頁；已報告頁之外記憶體保留。成功 AX=`004Fh`。

## 擬議契約

MOO2 `AttachMachine` 建立與埠調色盤共用的有限顯存裝置；附掛機器後的 `4F02h/0101h` 啟用視窗、bank=0，清已報告的六頁。2 MiB 容量與視窗／影像參數取自同一固定模式／控制器設定，避免第二套魔術數字。模式 03h 停用視窗映射；未附掛的歷史規格 239 仍只記受限模式狀態，不聲稱有顯存。

支援直接 `INT 10h` 精確 EAX=`4F05h`、EBX=0 的設定與 EBX=`100h` 的視窗 A 讀回。設定只接受完整 EDX 在 0–31，更新偏移 `bank * 64 KiB`，不改資料；讀回保留 EDX 高半部、返回目前 bank 到 DX。CX 不參與操作且保持。兩者成功 EAX=`4Fh`，其餘資料／段／旗標保持；未設定模式、未綁定、視窗 B、未知高位／子功能與越界 bank 明確拒絕且不改狀態。這是有限工具介面，未知狀態不偽造 BIOS 失敗返回。

`LEMachine.Read8`／`Write8` 對活躍 `A0000h–AFFFFh` 轉入顯存；word／dword 讀取及 CPU 存取也消費相同映射。DPMI 實模式橋接必須經相同 byte 接口，不直接繞到 Mem。視窗之外維持原記憶體；切換保留各區段內容，不以複製窗口覆蓋遊戲碼。`Mem` 是一般 RAM，不能當活躍顯存收據。

提供零起點 640×480 索引快照及經既有 DAC／像素遮罩的 RGB 消費端，快照複製以避免外部改內部資料。正式診斷保存 bank／索引雜湊／bank 呼叫，不以畫面檔存在宣稱正常玩家畫面完成。LE 尚無完整機器序列化入口，本規格不新增存檔格式或宣稱持久化恢復。

本模型標 **hardware-spec approximation（硬體規格近似）**：不建模 S3 暫存器、硬體回掃、線性顯存模式、非零顯示起點或通用 VBE BIOS。原版正常畫面／音效／亂數與 remake 同狀態、規格 255，仍是獨立閘門。

## 驗收計畫

先有界擷取候選 **DOSBox-X CS:EIP** `0180:0035CC54` 的實際 bytes、輸入、同次返回及第一個 caller；未命中前只是假說。另依公開契約驗證 0／5／31 切換、內容保留、跨窗口邊界、byte／word／dword 與 CPU／DPMI 兩條路徑、模式重設清頁與頁外保留、索引／RGB 消費與快照隔離、未知形狀拒絕。審查 READY 後實作，固定 EXE 全套測試與兩條自然路徑自行重生；舊規格定位回填正負護欄、擁有權與容器清理一起驗證。

READY 審查：VBE 標準定義單位、視窗參數、返回及清頁契約；固定模式資訊與控制器收據已提供全部容量／布局，不必追 S3 driver。`LEMachine` 及 DPMI 橋接尚未經顯存映射，word／dword helper 也需補同一消費端；修改邊界已明定。模式 03h 停用映射並清 `vbeModeSet`，模式重設只清六個已報告頁，頁外保持；初次新裝置零初始化是明示近似。未列 BIOS 形狀仍拒絕、未附掛的舊返回契約保持。由 DRAFT 轉 READY 後實作，輔助樣本只核對固定使用點。

## 原版輔助使用點

**已證實，同次服務返回**：DOSBox-X 2026.07.02 SDL2 heavy debugger、映像 ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`，`startup_probe_131.py --vbe-window-control` 命中 **CS:EIP** `0180:0035CC54 → 0180:0035CC56`，原始 bytes 與上述 dosgolem `0x228C54` 窗口相同。EAX=`4F05h`、EBX=ECX=ESI=0、EDX=5、EDI=`3A2090h`、EBP=`3EBB88h`、ESP=`3EBB5Ch`；DS／ES／SS=`188h`、FS=0、GS=`20h`、EFLAGS=`206h`。返回 EAX=`4Fh`，其餘擷取欄位保持。第一個 caller 的 POPAD／RET 位於 `0180:0035CC56／0035CC57`；僅核對固定使用點。dosgolem 自然呼叫 EFLAGS=`246h`，不宣稱兩側完整初始狀態一致。

私有 JSON／caller LOG／完整終端 SHA-256 分別 `b2a8d36a2951d0105908e34769213ef9e3f7094fab135f88d874e676ab036e7b`／`e503634c31c70a0a903a3d5317001ffafcfa70ca2adfaa68595b1ac044331f9d`／`ee76924805774425f43edb214f88865dbbe1312ab388ebb2bfcee7e8106c18ff`，全部留本機。具名最小架構輸入加入測試。

解析回填不可變鍵：固定 EXE 雜湊＋DOSBox-X `0180:0035CC54`／dosgolem 高位 LE `0x228C54`＋`CD 10`／`4F05h`。規格 264 保留「VBE 視窗停點已由規格 265 接通」及本檔連結；239 的歷史返回範圍連到本延伸。`--check-vbe-window-spec-backlinks` 自動核對，不以純文字語意名稱配對不同位址。

## 實作與驗收

`moo2_vbe_video.go` 從固定模式資訊建立 2 MiB 顯存，與既有 DAC 埠共用調色盤；`le_machine.go` 的 byte／word／dword 及 CPU、`dpmi_real_mode.go` 的 byte 橋接消費同一映射。`moo2_vbe_video_test.go` 已驗實際 CPU MOV 與 DPMI 讀寫、切換 0／5／31 後內容保存、首尾跨窗口讀取、一般 RAM 隔離、原版返回樣本與非目的欄位保持、未知形狀原樣拒絕、RGB／索引快照隔離、模式 03h 停用，以及重新設定時只清六頁、頁外保存和遮罩恢復。

Go 1.24.13、映像 ID `sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac`；`go test -buildvcs=false ./internal/machine -run 'TestMOO2VBE(Window|Consumers)' -count=1` 通過。加入原版樣本後的 `DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE GOMAXPROCS=2 go test -p 2 -buildvcs=false ./... -count=1` 全套通過；`workplace/full-test-265.txt` SHA-256 `e253930379399d7b649170f5e72fb40f3bf1eac298f44fdeb2466d883fd0b307`。

**已證實，自生顯存消費與下一停點**：同一完整資料／MOX.SET／分離 DOS arena，`DOSGOLEM_MOO2_SEPARATE_DOS=1 go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game` 及增加 `DOSGOLEM_MOO2_MOUSE_EVENT_AFTER_POSITION=1`，兩條路徑均自行接受 `4F05h` 的 5／6／7／8／9 區段並累計 307,200 byte 寫入。無事件第 6,738,834 步、設定後受控事件第 6,738,869 步，在 **dosgolem 高位 LE 線性** `0x228CA7` 的 `CD 10 61 FC C3 00 00 00 00 60 66 8B 1D 5E 3A 2A` 拒絕，EAX=`4F07h`、EBX=ECX=0、EDX=`200h`（Y=512）、DS／ES／SS=`188h`、EFLAGS=`246h`、DOS 呼叫 6。拒絕後 EIP=`0x228CA9`，受控回呼 started=1／completed=1。

兩份 gzip SHA-256 `d3fd50d6a07c913cacca03a827ef2d4558abd33a0743e9234f89cda46f4d32fd`／`13a24e91b05a4e7addbff28701ca9b3079acf9e80c7800a719eebea8ea9be558`；診斷零起點 307,200 byte 索引 SHA-256 均為 `7818f5542a0404157573be6cffc0e0c8e68ce3c0f5d17d07ccdd9313fb700baf`。這是尚未切到已寫影像的零頁，不能當作正常玩家畫面或跨執行器逐像素收據。沒有新增完整 LE 序列化。

限定視窗、共享映射與消費端 CONFORMED；hardware-spec approximation、原版整段平台差異、SAR 的 AF、DIV 的 ZF、DTA 保留區差異仍明示。規格 255 仍 READY，正常玩家畫面、音效、受控亂數與 Go remake 玩法同狀態未完成。

**非零顯示停點已由規格 266 接通**：[266-moo2-vbe-display-start.md](266-moo2-vbe-display-start.md) 保留此處歷史 `0x228CA7`／`4F07h` 定位，延伸有效起點及顯存圖像消費；未把本規格的零頁快照重登錄成新畫面。
