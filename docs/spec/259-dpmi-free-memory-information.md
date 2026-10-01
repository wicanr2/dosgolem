# 259 — DPMI 可用記憶體資訊

狀態：**CONFORMED（配置器一致的平台資訊）**
日期：2026-10-01
範圍：通用 DPMI `INT 31h/AX=0500h`；不逆向 DOS extender 內部，不改 remake 玩法。

## 證據與待審項

- **已證實，自生停點**：隔離 dosgolem `68e0ebca82cdcb436c7910bc676a818167215343`、Go 1.24.13，固定官方 1.31 EXE SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`、正版根層 417 檔與固定 MOX.SET，無事件第 6,217,167 步在 **dosgolem 高位 LE 線性** `0x24C315` 的 `CD 31`、AX=`0500h` 拒絕。ES=`188h`、EDI 指向呼叫端緩衝區，完整收據與工具／輸入雜湊見 [258](258-moo2-windows-version-absence.md)。
- **公開平台契約**：[DPMI 1.0 原始規格](https://docs.pcjs.org/specs/dpmi/1991_03_12-DPMI_Spec_v10.pdf) 的 `Int 31H Function 0500H`：32 位客戶端以 ES:EDI 傳入 48-byte 輸出區塊；成功清 CF。第一個 dword 為可配置的最大連續區塊，未支援欄位與保留 12 bytes 填 `FFFFFFFFh`。此服務為建議資訊，不承諾另一套平台的實體容量與交換檔。亦見 [DJGPP 的 DPMI 介面文件](https://www.delorie.com/djgpp/doc/dpmi/api/310500.html)。
- **已證實，原版同次返回**：`apps/moo2/tools/startup_probe_131.py --free-memory`，DOSBox-X 2026.07.02 SDL2 heavy debugger，僅是輔助基準。**DOSBox-X CS:EIP** `0180:00380315` 原始 `CD 31 C3 CD 32 C3 CD 33 C3 CD 34 C3 CD 35 C3 CD`，ES:EDI=`0188:003EBB38` 的 48 bytes 起初全零。同次 `00380317` 的所有擷取暫存器／旗標保持；12 個返回 dword 為 `01913000 / 1913 / 1913 / 1E90 / 1913 / 1913 / 1E90 / 1913 / FFFFFFFF / FFFFFFFF / FFFFFFFF / FFFFFFFF`（十六進位）。這些是該環境的容量，不寫成通用常數。
- **已證實，玩家初始化邊界**：外層 `0180:00334FD2` 的原始相對運算元 `[ebp-007C]` 指向 `003EBB38`，將第一欄位加到 EAX=`E810h`，右移十位後返回；`0180:00234D5C..00234D68` 乘 `1000`、比較 `001B5418h` 並通過容量檢查。不深入其前一個記憶體 helper 的內部，不宣稱所有分頁欄位在整個遊戲未被使用。
- 私有 JSON SHA-256 `0e5b4b0d324a843ce4f4aa06b70d47e7b8d0bfd7951806acb75b3e9a8e7e14ed`、caller LOG `929e2c6aa680b6e3932bc4692131c3b08e37c00c56ae09fd4f59b1d0dd378f80`、外層 consumer LOG `af617b57c1baaf6c28c785774f476f48fb05dec1b6dfec63ae76644a3df286bc`、終端 `83dceb963801edfa98e9e8c014f712627309d4135d09add5ff51f9271660926c`。ZIP／MOX.SET 與映像 SHA-256 沿用 [255](255-moo2-protected-mouse-callback.md)。

## 擬議契約

重用既有 `DPMIHost`、64 MiB `dpmiAddressLimit` 與 `0501h／0502h` 配置器，第一欄位應與該配置器實際可分配的最大區塊一致：比較現有釋放區間與尚未配置的尾段；尾段起點取 brk 與 backing memory 尾端對齊值的較大者。不硬編原版 extender 的返回容量。

當前容量檢查只取第一欄位；其餘未模型化的分頁欄位依規格填未知，後續玩家流程的使用仍未知。有效 48-byte buffer 寫入後只清 CF；保留一般暫存器、段、EIP、其他旗標與配置帳本。描述子／backing 範圍、唯讀、offset 或線性位址溢位均先驗證，錯誤明確拒絕且不能留下部分寫入，不把未知位置當成功。

READY 審查：原版同次返回符合公開 48-byte 契約，當前容量檢查只取第一欄位；現有配置器足以算出可兌現的連續容量。接受其餘未模型化欄位為規格允許的未知，但保留後續遊戲消費未知邊界。描述子和 backing 必須屬同一台機器；不支援的 buffer 形狀直接拒絕，不猜造此「總是成功」介面的錯誤碼。此為 **platform-spec approximation**，不承諾原版 extender 容量逐值相等。DRAFT 轉 READY 後才實作。

## 驗收規劃

解析回填台帳：不可變鍵為固定 1.31 EXE 雜湊＋dosgolem 高位 LE 線性 `0x24C315`／DOSBox-X `0180:00380315`；語意是標準 `0500h`。舊規格 258 須含「0500h 平台契約已由規格 259 補齊」及本規格連結，由 `startup_probe_131.py --check-free-memory-spec-backlinks` 檢查；缺原始定位、狀態或舊標記即拒絕。不把既有歷史收據的停點改寫。

測試非零段基址、EDI 超過 64 KiB、高 AX、全部 48 bytes／鄰接哨兵、CF 以外狀態保持、配置／釋放／重用前後最大區塊、碎片與容量耗盡。拒絕未知 selector、唯讀、descriptor limit、backing 邊界與溢位，記憶體不變；原本未知 DPMI 功能繼續拒絕。固定 EXE 全套回歸及兩條原檔自然路徑由 dosgolem 重生，不用 DOSBox-X 圖片替代正式收據。

## CONFORMED 收據

後續回填：堆疊 SAR 停點已由規格 260 接通，最新 CPU 驗收見 [260-cpu386-sar-stack-dword-immediate.md](260-cpu386-sar-stack-dword-immediate.md)。下列原始拒絕／步數保留為歷史收據；DPMI 本規格的容量近似及玩家未知邊界不受此回填影響。

上述 buffer 與配置／釋放／重用、耗盡測試通過。一次性 `golang:1.24-bookworm`、Go 1.24.13，`DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE GOMAXPROCS=2 go test -p 2 -buildvcs=false ./... -count=1` 全套通過。最終私有 `workplace/full-test-259.txt` SHA-256 `4f556544b654a81516bd019606d95945855e71aa6f1eae55d44c7563b421fc6d`。首次測試初稿用了既有 `Descriptor` 沒有的 `Default32` 欄位而編譯失敗；刪除多餘欄位後同映像、同輸入乾淨重跑，保留 `full-test-259-first.txt`。

原檔無事件路徑第 6,713,034 步、設定後受控事件路徑第 6,713,069 步，均在 **dosgolem 高位 LE 線性** `0x200E5A` 的 `C1 7D F4 04` 明確拒絕（`ModRM 7D 尚未支援`）。受控路徑回呼 started=1／completed=1。無事件私有診斷 SHA-256 `e6ce6f569119d6ef82e0b319c6ff98c65acc80c8b17ae2a25f9ded0c5f2596a9`、事件診斷 SHA-256 `7b6bedae196683e1f67dc0af1f2356cadde7e593ea54e92f95bfdce75ae645b7`。兩條路徑自行越過 `0500h`，沒有以原版容量常數造假通過。

Python 語法、規格索引、正常回填及刪除舊標記必拒絕已驗。首次負向護欄命令的 shell 引號移除 Python 搜尋字串中的單引號而未命中，改正確引用後重跑；不是平台功能故障。仍無正常玩家畫面、音效、受控亂數或與 Go remake 的玩法同狀態收據，規格 255 維持 READY。下一步只核對此窄 CPU 記憶體移位形狀的公開指令契約與原版使用，不追平台內部。
