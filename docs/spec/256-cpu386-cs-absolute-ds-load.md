# 256 — 從 CS 絕對位址載入 DS

狀態：**CONFORMED（限定 CPU 指令形狀）**
日期：2026-10-01
範圍：`66 2E 8E 1D disp32` 及兩個前綴的對調次序；不擴充其他 CS 段載入形狀。

## 證據與契約

- **已證實，自生停點**：dosgolem `438d6cc5971c3e212e0ce949e1ddd61de307f794` 加規格 [255-moo2-protected-mouse-callback.md](255-moo2-protected-mouse-callback.md) 已測實作；Go 1.24.13。官方 1.31 EXE SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`、正版根層 417 檔及固定 `MOX.SET`。從 LE entry 自生，註冊後首次 IF=1 由受控 API 排入 `x=657/y=189/buttons=0/mickey=0/0`，第 1,546,194 步在 **dosgolem 高位 LE 線性** `0x2001E4` 的 `66 2E 8E 1D ED 01 20 00` 拒絕 CS 覆寫，callback started=1／completed=0。私有 `workplace/moo2-probe-255-mouse-event.txt.gz` SHA-256 `da3e269e70e1ec703c1afdfe056805eb92523a1eeb7c17c0e3b21a56dfcefc3f`。
- **已證實，原版同次消費**：規格 255 的 DOSBox-X 2026.07.02 SDL2 heavy debugger 一般移動回呼 LOG，**DOSBox-X CS:EIP** `0180:003341E4` 執行 `mov ds,cs:[003341ED]`，來源 word=`0188h`，下一指令 `0180:003341EC` 的 DS=`00A8h → 0188h`，其他所列暫存器與旗標保持。LOG SHA-256 `e9b59d0464c17c70ae0c9194403ec6f9f30f526d2e99ca49a6521122df0258b2`。兩套位址空間及初態不同，不宣稱完整同狀態。
- **公開 CPU 契約**：[Intel 80386 原廠手冊](https://read.seas.harvard.edu/~kohler/class/aosref/i386.pdf)，PDF 第 345–346 頁的 MOV 段載入契約：來源 word，目的段由 selector 驗證後載入，旗標不變。CS 覆寫決定來源段；`disp32` 仍為 32 位元有效位址。

重用既有 `8Eh` 來源 descriptor 的 word 讀取及 `canLoadSegment`；只解除入口 CS 覆寫攔阻，並在 ModRM 解碼後要求 operand16、無 repeat、ModRM=`1Dh`。只改 DS，不寫來源、不改一般暫存器及旗標。未知 selector、來源越界／不可讀、截短指令與其他 CS 形狀拒絕。通用 CPU 不認識 MOO2 位址或內容。

## 驗收

READY 審查：原版同次 word 來源、DS 前後值與不變旗標已取得；現行解碼器已有該 ModRM 的 descriptor 讀取，只需解除一項過窄前綴拒絕並維持形狀檢查。原始定位、bytes、輸入雜湊及工具位址空間皆可回查，據此由 DRAFT 轉 READY。

兩個前綴次序、CS 與 DS 不同 base、word 後有雜訊、來源邊界、null／有效／無效 selector、截短與錯誤形狀；固定 EXE 全套測試及原檔受控回呼自行越過停點。回呼完成與正常玩家路徑另驗，不因一條指令通過升格整個回呼。

## CONFORMED 收據

上述指令測試與固定 EXE 全套回歸通過，私有 `workplace/full-test-256.txt` SHA-256 `9860d86ce52134078bb35dca6254e433845197a2803109bb1f2a758e61c8f057`。原檔受控回呼自生越過 `0x2001E4` 並完成一次遠返回，隨後第 1,546,522 步停於座標設定 `INT 33h/AX=4`；私有 `workplace/moo2-probe-256-mouse-event.txt.gz` SHA-256 `2cc16391476f390116582129013b28f784cb0a4c48ced8a530399e454fc98510`。只驗窄指令契約，不外推回呼所有 consumer 或玩家路徑。

## ES 目的的後續範圍

本規格仍限定已驗的 DS 形狀；後續 CS 絕對 word 的 ES 形狀由規格 279 擴充，見 [279-cpu386-cs-absolute-es-load.md](279-cpu386-cs-absolute-es-load.md)。原版有限 word 來源與完整保持狀態已審查後 READY 才實作；不把新增 ES 範圍回寫為當時已驗。未核准的 SS／FS／GS／CS 與非絕對形狀保持拒絕，完整保護模式權限模型仍未建立。
