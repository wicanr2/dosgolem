# 260 — 堆疊 dword 的立即數算術右移

狀態：**CONFORMED（窄 CPU 形狀與定義旗標）**
日期：2026-10-01
範圍：32 位 `C1 7D disp8 imm8`，即 `SAR dword SS:[EBP+disp8],imm8`。只補 CPU，無 remake 玩法變更。

## 證據

- **已證實，自生停點**：隔離 dosgolem `a6a7b79a60c9656f93b216b539707ff9b52b5671`、Go 1.24.13。官方 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`、正版根層 417 檔與固定 MOX.SET。無事件第 6,713,034 步、受控事件第 6,713,069 步在 **dosgolem 高位 LE 線性** `0x200E5A` 的 `C1 7D F4 04` 拒絕；EFLAGS=`246h`，完整輸入／工具／收據雜湊見 [259-dpmi-free-memory-information.md](259-dpmi-free-memory-information.md)。
- **公開 CPU 契約**：[Intel 80386 原始手冊](https://read.seas.harvard.edu/~kohler/class/aosref/i386.pdf) 的 SAR／SHR 指令頁與 [相同手冊 HTML 指令頁](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/SAL.htm)：`C1 /7 ib` 算術右移 dword，保留符號，count 只取低五位。非零 count 的 CF 取最後移出位元，ZF／SF／PF 由結果決定；count=1 時 OF=0，多位 OF 未定義，AF 未定義。有效位址及可寫段檢查遵守既有 CPU 描述子模型。
- 原版同次資料／旗標樣本由 `apps/moo2/tools/startup_probe_131.py --sar-stack-memory` 輔助擷取，DOSBox-X 2026.07.02 SDL2 heavy debugger；只核對這四個 bytes 的使用，不追所在 helper 的內部。
- **已證實，原版同次樣本**：**DOSBox-X CS:EIP** `0180:00334E5A` 原始 `C1 7D F4 04 EB 0A 8B 45 F4 C1 F8 04 40 89 45 F4`；原始運算元 `SS:[EBP-0Ch]`，EBP=`003EBB84h`，SS=`0188h`，目的 `0188:003EBB78` 的 little-endian dword=`32 → 2`。同次下一指令 `0180:00334E5E` 的一般暫存器、段與堆疊保持；EFLAGS=`0246h → 0212h`。CF／PF／ZF／SF 符合結果；多位 OF 與 AF 未定義。原版 AF=1，執行器策略清 AF，因此不能宣稱完整旗標逐位元相等。
- 私有 JSON SHA-256 `c6030ecb0371d9a5521f4581934d459423a0f2832e9591ad750247eb04e3136b`、caller LOG `7ef34efe2fa2add0e082fd7e4f2b4b9aa19726ffcd68c059d2e23bb02d0af093`、終端 `0be4a8d736a44d0699d876865c96497ce48cbc32d7af218b570ead7f3528bbaa`；工具映像、ZIP／MOX.SET 雜湊沿用 [255-moo2-protected-mouse-callback.md](255-moo2-protected-mouse-callback.md)。最初錯把工具間位址差算成 `133000h` 而未命中，按既有已確認定位修正為候選 `00334E5A` 後，實際原始 bytes 及同次返回確認；保留第一次私有終端，不把換算當新位址證據。

## 擬議契約

只接無前綴的 `C1 /7`、ModRM=`7Dh`；位移為有號 disp8，基底 EBP，預設段 SS。保留既有所有暫存器及段狀態，只改四 bytes 與定義旗標；EIP 正常增四。重用既有完整 dword 段存取，不硬編遊戲地址或移位四位。

完整取得位移與立即數後，再驗可寫範圍並讀取操作數。有效 count=0 不改值與任何旗標；非零 count 用有號 32 位右移。多位 OF 沿既有 D3 算術右移保留，AF 沿既有 logic flags 清除策略；兩者不當作原版精確旗標證據。寫入成功後才更新旗標，未知形狀、前綴、截短、段或 backing 越界仍明確拒絕。

READY 審查：公開 CPU 規格足以描述此固定 ModRM 的行為，原檔 bytes 證明目前玩家路徑確實使用；既有完整 dword 段讀寫與 D3 暫存器 SAR 可重用。不需要猜測 helper 語意或先重證 CPU 標準。DRAFT 轉 READY，原版同次樣本作實作驗收交叉檢查；未定義旗標及任意自訂 Bus 的部分寫入錯誤不外推為完整硬體例外模型。

## 驗收

解析回填台帳：不可變鍵為固定 EXE 雜湊＋dosgolem 高位 LE 線性 `0x200E5A`／DOSBox-X `0180:00334E5A`，語意為本 CPU 指令形狀。舊規格 259 須含「堆疊 SAR 停點已由規格 260 接通」與本規格連結；`startup_probe_131.py --check-sar-stack-spec-backlinks` 核對，缺定位、狀態或舊標記即拒絕。保留舊停點收據，未新增玩法語意。

固定的正／負／零／端點值及 count=`0/1/4/31/32/33/255`；使用非零 SS base 及不同 DS base，正負位移與鄰接哨兵。比較暫存器、段、EIP、定義旗標及明示未定義旗標策略。截短指令、唯讀、缺描述子、descriptor limit、backing 越界、讀寫失敗及未列形狀必須拒絕。固定 EXE 全套回歸、兩條原檔自然啟動路徑自行重生；沒有正常玩家收據時不宣稱遊戲完成。

## CONFORMED 收據

上述形狀／值／段／旗標與錯誤邊界通過；另以原版同次 `32 → 2`、原始 CS:EIP／SS／EBP 輸入執行 CPU 樣本，定義旗標及暫存器保持相符。原版 AF=1、執行器 AF=0 的差異明確保留；後續第一個無條件跳躍至 `0180:00334E6A` 的 CMP 重算旗標，這個樣本不消費移位後的 AF，但不外推其他使用。

原檔全套 `DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE GOMAXPROCS=2 go test -p 2 -buildvcs=false ./... -count=1` 最終通過，私有 `workplace/full-test-260-final.txt` SHA-256 `12e3a3f78ea25fa3c1c4745b78faaf2b008b0466837bee54e06393b4445c23a4`。加入原版樣本的第一次測試把外部段排列直接放進 CPU 陣列，SS 因此錯成 `20h`，被目的可寫檢查正確拒絕；改具名索引，並修正規格 258 查詢測試中的相同排列後，同一映像與資料乾淨重跑全套。保留 `full-test-260-final-first.txt` 及加入樣本前已通過的 `full-test-260.txt`，不改 production 行為。

無事件原檔第 6,725,897 步、設定後受控事件第 6,725,932 步，均停於 **dosgolem 高位 LE 線性** `0x229A59` 的 `CD 21`，EAX=`002B4E38h`（AH=`4Eh`）、ECX=`0`、DS:EDX=`0188:002BDB38`。這是現有受限檔案查詢在當前輸入拒絕，不宣稱所有 `4Eh` 都未支援。受控路徑回呼 started=1／completed=1。無事件診斷 SHA-256 `66b324e1fce7d3e260d550eeb7e8c16b7058528074ec3bb7130ed7fdab94eac7`、事件診斷 SHA-256 `dda9b0d9cc2b663a8bef3c4cf6735f19e7c8327c187df6864175d7eb97593732`。

Python 語法、規格索引、正常解析回填與刪除舊標記必拒絕通過。仍無正常玩家畫面、音效、受控亂數或 Go remake 玩法同狀態收據，規格 255 維持 READY。下一最小行動為擷取 `4Eh` 搜尋輸入、DTA 狀態與原版返回，依公開 DOS 契約補受限平台支援，不追檔案 helper 內部。

**目前目錄前綴已由規格 261 接通**：[261-moo2-dos-findfirst-current-directory.md](261-moo2-dos-findfirst-current-directory.md) 已核對拒絕輸入為 `.\simtex.lbx`、原版返回 `12h`／CF=1，明示 DTA 保留區差異。原有停點收據保留；兩條新自生停點與限定驗收見後續規格，不把舊搜尋停點當作現況。
