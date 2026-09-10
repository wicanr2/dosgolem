# FD2 標題選單與單次方向鍵驗證

日期：2026-09-08。原版 SHA-256：
222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f。
目前平台狀態以[規格186](../spec/186-fd2-platform-gap-continuation.md)為準。

dosgolem 從原版入口自行執行到標題選單，加入 BIOS AH10 增強鍵盤佇列後，
單次 Down 使選項由 START 移至 LOAD；Down、Up、Enter 三次輸入後，
原版自行進入王宮開場。沒有改寫原版 EXE、EIP、遊戲狀態或導入直接進場。

## 對應畫面的輔助比較

DOSBox-X 2026.07.02 SDL2 heavydebug，Docker 映像
fd2-dosbox-x:debug-0d7b272b，使用原版複本與真正 X 鍵盤事件。
dosgolem 原生320×200；DOSBox 視窗640×417，其中上方17列是外部功能列。
完整遊戲內容按2倍整數顯示比對，逐一驗證所有2×2區塊同色；
不遮蔽任何遊戲像素，不插值、不生成替代原圖。

| 畫面 | 比較像素 | 不同像素 | 最大色差 | 不均勻放大像素 |
|---|---:|---:|---:|---:|
| START選中 | 64000 | 0 | 0 | 0 |
| Down後LOAD選中 | 64000 | 0 | 0 | 0 |

這是**對應選單畫面與單次方向鍵結果**，不是完整機器同狀態、長按／釋放、
重複速率、音畫同步、第一關或 remake AppImage 驗收。
正式原版圖片由 dosgolem 自行產生，DOSBox 圖片只作輔助，兩者檔案分別保留。
逐檔路徑、雜湊與比較結果見[證據清單](../evidence/fd2-title-input-parity-20260908.json)。

## 重跑

所有命令只在已掛載儲存庫及唯讀原版的 Docker 容器內執行，
容器使用 UID/GID 1000、停用網路、資源限制與外層120秒逾時。
Go映像為 golang:1.24-bookworm，使用專案既有快取。

    go run ./apps/fd2/cmd/bootprobe -steps 100000000 -frame /output/start.png
    go run ./apps/fd2/cmd/bootprobe -steps 100000000 -keys down -frame /output/down.png
    go run ./apps/fd2/cmd/framecompare /shots/start-window.png /output/start.png
    go run ./apps/fd2/cmd/framecompare /shots/down-window.png /output/down.png

輸出PNG要求新檔，不能覆蓋舊收據。進場測試另以 -state 指定全新覆蓋目錄，
FD2.TMP首次開寫才由唯讀來源複製，後續寫入與截斷只作用覆蓋層。
讀寫權限、路徑拒絕及來源不變另有平台測試；不等於存讀檔全路徑驗收。

DOSBox擷取腳本及設定保存在證據清單列出的本機研究目錄，原版PNG不加入公開庫。
第一次腳本未等待到斷點，第二次原生抓圖沒有輸出，兩次均不得作畫面驗收；
第三次改用視窗擷取後才成功，保留前次失敗的來源索引。
