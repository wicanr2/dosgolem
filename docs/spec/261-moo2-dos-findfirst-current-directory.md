# 261 — MOO2 DOS 搜尋的目前目錄前綴

狀態：**CONFORMED（限定前綴、返回與公開結果欄）**
日期：2026-10-01
範圍：只補明示 MOO2 DOS 設定下 `AH=4Eh`／CX=0／單一 `.\` 目前目錄前綴的精確 8.3 查詢；無 remake 玩法變更。

## 證據與原始定位

- **已證實，自生輸入**：隔離 dosgolem `8f4c579beda504a66aefd74186fa6d1fa8d3d072`、Go 1.24.13，以固定官方 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`、正版根層 417 檔及固定 MOX.SET 重播。**dosgolem 高位 LE 線性** `0x229A59`、原始 `CD 21 E8 AC 65 01 00 89 DA E8 21 00 00 00 59 C3`，無事件第 6,725,897 步因當前輸入拒絕；不是所有 `4Eh` 未支援。
- 停點輸入 EAX=`002B4E38h`、ECX=0、DS:EDX=`0188:002BDB38`；ASCIIZ bytes=`2E 5C 73 69 6D 74 65 78 2E 6C 62 78 00`，即 `.\simtex.lbx`。已設定 DTA=`0188:00295828`；之前 `simtex.lbx` 查詢缺檔，結果欄仍留有 DIPLOMAT.LBX 的舊內容。私有診斷 `workplace/moo2-probe-find-input.txt.gz` SHA-256 `f1865f4101ce2b6e5a3054e2d4d7875a3503a24105c45898e0c717489e0321c1`；輸入、工具映像與前一階段收據見 [260-cpu386-sar-stack-dword-immediate.md](260-cpu386-sar-stack-dword-immediate.md)。
- **平台來源**：[DOSBox-X 原始碼的 DOS_MakeName 與 DOS_FindFirst](https://github.com/joncampbell123/dosbox-x/blob/master/src/dos/dos_files.cpp) 將單一 `.` 目錄元件消去；目前目錄中的精確檔名查詢可沿用不帶前綴的搜尋。DTA 搜尋標頭／成功結果欄沿用 [219-protected-dos-findfirst-exact.md](219-protected-dos-findfirst-exact.md) 的既有契約，不追遊戲檔案 helper 內部。
- **已證實，原版同次輔助樣本**：`apps/moo2/tools/startup_probe_131.py --find-current-directory` 捕獲 **DOSBox-X CS:EIP** `0180:0035DA59 → 0180:0035DA5B`，實際 16 bytes 與上述序列相同。原版 DS:EDX=`0188:003EBB18` 的搜尋字串相同，DTA=`0188:003C3828`；EAX=`003E4E18h → 00000012h`、EFLAGS=`246h → 247h`，其餘已擷取一般暫存器、段與堆疊不變。先前無前綴 `simtex.lbx` 也缺檔；ZIP 中確無此檔，未加入合成遊戲素材。
- DTA 的搜尋名保持 SIMTEX／LBX，公開結果區 `+15h..+2Ah` 保留之前 DIPLOMAT.LBX 的內容；原版保留區 `+0Ch` 由 `04h → 05h`。執行器既有策略保留該區，不能宣稱完整 DTA 逐位元相等，保留區內部仍未知。最小 caller `0180:0037400C` 依 CF 走錯誤路徑並使用 AX=`12h`，不追該 helper 內部。工具版本／映像與 ZIP／MOX.SET 雜湊沿用規格 260／255；私有 JSON SHA-256 `d3d2ee93dc1f9f812d81d2ac37ee55e312d427e530b664825c71388389ad987d`、caller LOG `6b05980307ee4e21811c58da5f8832c0e9a594c224b6ea4bb35892402ec793c6`、終端 `bc81ece31155571252f243c1d1ad0027f94164fc42b476eebf5f9447cf148fcb`。

## 擬議契約

只在 `moo2Profile` 下，容許精確 8.3 檔名開頭的一次 `.\`；去除後仍由既有 `exactDOSName` 驗證。有界讀取最大 14 字元加 NUL，其餘設定維持 12 字元加 NUL。唯讀提供者只收到去除前綴的單一檔名，保留 `os.Root` 與原有 slash／drive／symlink 安全邊界。

缺檔及成功的 EAX、CF、DTA 和其他暫存器保持規格 219 的既有策略；不加入任意子目錄、父目錄、絕對路徑、磁碟代號、萬用字元、第二次 `.\` 或 Find Next。未知前綴、截短 ASCIIZ、描述子／DTA 越界與提供者錯誤仍拒絕，不能因試跑向前而猜補玩法。

## 驗收計畫

READY 審查：公開目錄前綴契約與原版固定字串／返回已足以支援此平台形狀；不需要猜測玩法或 DOS 保留區內部。由 DRAFT 轉 READY；原版保留區差異明示，不新增私有欄位或完整 DTA 對齊聲明。

用既有成功／缺檔資料比較 plain／prefix 的相同執行器 DTA 與暫存器，另測最大 8.3 長度、舊結果欄保留、前綴未移交提供者、未知路徑／前綴、非 MOO2 設定、名稱／DTA 越界仍拒絕。固定 EXE 全套回歸、無事件與設定後受控滑鼠事件兩條自然路徑重生；正常玩家路徑與玩法同狀態仍是獨立未完成閘門。

## 實作與限定驗收

`internal/machine/le_startup.go` 只在 MOO2 設定擴大有界讀取長度並消去一次目前目錄前綴，然後沿用精確檔名驗證及唯讀提供者；沒有放寬提供者路徑安全。`le_find_current_directory_test.go` 驗證成功／缺檔、最長 8.3、暫存器及鄰接哨兵、完整執行器 DTA 與無前綴結果相同；拒絕第二前綴、父／子目錄、磁碟、其他分隔、萬用字元、過長 ASCIIZ、非 MOO2 設定、CX 非零與 DTA 越界，且不改旗標／暫存器／記憶體。原版保留區 `+0Ch` 的差異仍未建模，不宣稱完整 DTA 對齊。

固定 EXE 的 `DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE GOMAXPROCS=2 go test -p 2 -buildvcs=false ./... -count=1` 全套通過，私有輸出 SHA-256 `c498ec94eab675212c95c76d394bacada8ca6d51f8c88112d9488139b68d6fd9`。無事件第 6,725,985 步、設定後受控滑鼠事件第 6,726,020 步自行越過搜尋，均停於 **dosgolem 高位 LE 線性** `0x114F43`、原始 `66 85 C0 0F 85 F5 00 00 00 B8 17 0C 26 00 31 D2` 的 TEST 的運算元大小前綴 拒絕。EAX=0，事件路徑回呼 started=1／completed=1。兩份診斷 SHA-256 分別 `872b6e8979491ade99b79c5d2aa3e1734bf257b22fc0d9a9dc4293ec288e1f49`、`c27af6189cc321d44b7455abb6f7fa016e575d1ffe97fe40eeef9aa0ecd770c0`。

解析回填：不可變鍵為固定 EXE 雜湊＋dosgolem 高位 LE `0x229A59`／DOSBox-X `0180:0035DA59`＋搜尋原始 bytes。規格 260 保存「目前目錄前綴已由規格 261 接通」及本檔連結，`startup_probe_131.py --check-find-current-directory-spec-backlinks` 核對定位與狀態，缺少舊標記必拒絕。規格 219 保留舊限定範圍，另連到本延伸規格。

仍無正常玩家畫面、音效、受控亂數及 Go remake 玩法同狀態收據；規格 255 維持 READY。下一最小行動是依公開 CPU 契約核對 `66 85 C0` 的位元寬度與定義旗標，不展開 runtime helper 內部。
