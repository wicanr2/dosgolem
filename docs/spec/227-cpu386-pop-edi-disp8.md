# 227 — POP dword 至 DS:[EDI+disp8]

狀態：**CONFORMED**  
日期：2026-10-01  
用途：固定 MOO2 1.31 原檔的啟動 record 尾端。

## 證據與位址

輸入固定 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`。DOSBox-X 2026.07.02 SDL2 heavy debugger 映像 ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582` 的 `apps/moo2/tools/startup_probe_131.py --mouse-query` 連續返回 LOG，於其 **CS:EIP** `0180:003801C6 → 003801C9` 顯示 `pop dword [edi+0014] → sbb eax,eax`；該列 EDI=`003D18E0h`、ESP=`003EB96Ch`、EFLAGS=`0006h`，來源堆疊值未以獨立記憶體擷取直接確認。私有 `mouse-query-return-logcpu.txt` SHA-256 `f78b6b183041b58bdebbfdcd7c0a9495126b3a8eb8b756c5582ac80528cbb7bd`，`mouse-query-registers.json` SHA-256 `a3d2148d16441bc034176c6c09e5b52031b754b0d08b48c8e79a1ca10b98abcc`。

dosgolem 固定原檔在合成 PSP／環境下第 5830 步、**重定位 LE 線性位址** `0x15C1C6` 遇 `8F 47 14` 失敗即關閉；私有 `workplace/moo2-probe-226-after.txt` SHA-256 `7190527fa2c8dfc170172864e4dd46ef31edeca94d62aac78a53299aa611b1b6`。同一原檔整合測試已在此停點前驗證滑鼠回傳寫入 record `+0/+4/+8/+0Ch`。

## 擬議 CPU 契約

只新增無前綴 `8F /0`、`mod=01`、`rm=EDI` 的 32 位 POP 記憶體目的形狀。`disp8` 以有號擴展加到 EDI，目的段為 DS；先從 SS:[ESP] 讀 32 位值，再寫至 DS:[EDI+disp8]，成功後 ESP 加 4。EFLAGS、其他暫存器與來源記憶體不變。堆疊來源或目的範圍失敗時拒絕且 ESP 不增加；`ESP+4` 溢位、其他 `/r` 編碼、重複／段／operand-size 前綴仍拒絕。這是通用 CPU 指令形狀；MOO2 原版僅命中位移 `14h`，其他位移以合成測試與既有 CPU 契約檢查。

本切片僅供啟動原檔前進；不修改 remake 玩法、UI 或存檔，不把編譯器包裝常式列為玩法 RE 分母。兩側 PSP、記憶體與堆疊位置仍不同。

## 驗收

合成測試須以非零來源值檢查非零 DS／SS base、正負 disp8、ESP／旗標／其他暫存器、來源與目的越界及未列形狀拒絕。固定 1.31 原檔整合測試從 LE entry 自然走到第 5830 步，單步檢查目的 record `+14h`、ESP 與下一 EIP；完整 `go test -buildvcs=false ./... -count=1` 通過。有界探針記錄下一個真正停點；DOSBox-X 收據只作輔助，不宣稱玩家路徑同狀態。

READY 審查：原版同次 LOG 證實 `8F 47 14` 的位址、解碼、EIP 與 ESP 由 `003EB96Ch → 003EB970h`，旗標不變；未從該樣本直接證實非零堆疊值或目的寫入。因此這兩項只依 [Intel `POP` 指令契約](https://cdrdv2-public.intel.com/835752/253667-sdm-vol-2b.pdf)與非零合成測試建立通用 CPU 形狀，不能冒稱原版非零值實測。目的來源都走既有段描述子讀寫，拒絕未列形狀；證據足以實作此有限 CPU 能力。

## 實作與有限驗收

`internal/cpu386/cpu.go` 只接無前綴 `8F 47 disp8`，先由 SS:[ESP] 讀 dword，再向 DS:[EDI+有號 disp8] 寫入，成功後 ESP 加 4。合成測試以非零 `DEADBEEFh` 驗證不同 DS／SS base、正負位移、來源不變、界限拒絕與未列形狀拒絕。固定原檔整合測試自 LE entry 與合成 PSP／環境自然至第 5830 步、**dosgolem 重定位 LE 線性位址** `0x15C1C6`，讀取當次 SS:[ESP]，單步後在 record `EDI+14h` 讀回同值，ESP 加 4、EIP=`0x15C1C9`，通用暫存器與旗標不變。`DOSGOLEM_MOO2_EXE=/tmp/ORION2.EXE go test -buildvcs=false ./... -count=1` 全通過；私有 `workplace/full-test-228.txt` SHA-256 `47dc1f8bd19688d9f2bf5d38637770bba02cdc74cd7f380016b76ffb6c3ea109`。原版樣本的來源值未獨立抓取，故相等收據只由 dosgolem 自身重生，非原版非零值對拍。

有界原檔診斷前進到第 5838 步、同位址基準 `0x15C1D6` 的 `66 8C 03`（失敗時 EIP=`0x15C1D9`），私有 `workplace/moo2-probe-227-after.txt` SHA-256 `3becc0caf1cd574f0818da4e265a4358c163caa08f86c1f4b6b91a2ae468db50`。原版 **CS:EIP** `0180:003801D6` 的同次 `--mouse-query` 返回 LOG 顯示 `mov [ebx],es`，其 16 位來源與記憶體效果尚待下一受限規格；本規格不據此宣稱正常玩家路徑或畫面同狀態。
