# dosoracle

**一個只跑得動一個 binary 的 DOS 執行器**：無頭、決定性、可以當 Go 套件 import。

它為《大富翁2》（大宇資訊，1993，DOS）的 remake
[`rich2`](https://github.com/wicanr2/rich2) 而寫，用途只有一個——
**讓「原版怎麼做」這件事可以在 `go test` 裡直接量，而不是隔著 X 看畫面。**

```go
ora := oracle.Load("…/RICH2")            // 載入原版（玩家自備）
ora.RunUntil(oracle.CallOf(0x25BF6))     // 跑到收租常式
rent := ora.Word("ds:1BE")               // 直接讀原版的變數
shot := ora.Indexed()                    // 320×200 色號陣列，不經過 X

parity.Compare(t, shot, remakeFrame)     // 兩邊都在同一個行程的記憶體裡
```

## 為什麼不是 DOSBox

`rich2` 目前的對拍是「docker ＋ Xvfb ＋ DOSBox ＋ xdotool ＋ 截圖」，
54 支腳本 6,116 行。量下來瓶頸不是 IPC（一次 `docker exec` 只要 0.058 秒），
是 **sleep**——每送一個鍵等 0.35 秒、每張截圖等 0.7 秒、每走一步等 2.2 秒。

那些 sleep 存在的唯一理由是：**主機沒辦法問模擬器「你跑完這一幀了嗎」，
只能拿牆上的時鐘猜。** 那同時是慢的原因與不穩的原因，也讓判準只能讀像素。

DOSBox-X 有 943,452 行 C++，重寫它並不能解決這件事。真正需要的東西小得多：
這個 binary 的主程式區 52,892 bytes **只用到 62 個助憶碼、全部是 8086**，
而且**不需要 x87**（浮點走 binary 自帶的 Microsoft 模擬器，
全檔 876 個 `INT 34h–3Dh`）。

完整評估：[`rich2/docs/spec/082`](https://github.com/wicanr2/rich2/blob/master/docs/spec/082-parity-oracle-emulator.md)。

**DOSBox-X 沒有被丟掉**——它是時序的參考實作與交叉 oracle。
本專案畫出來的每一張，都要拿它的索引截圖驗過才算數。

## 現況

| | 里程碑 | 狀態 |
|---|---|---|
| MVP-A | 8086 整數指令核心 | 進行中——SingleStepTests/8088 v2 |
| MVP-B | 跑到防拷畫面，與 DOSBox-X 索引截圖逐點相同 | 未開始 |
| M2 | 輸入與時序（鍵盤／滑鼠／PIT）| 未開始 |
| M3 | 儀器層：breakpoint／watchpoint／call trace／RND 記錄／savestate | 未開始 |
| M4 | Go API 與 `parity` 套件 | 未開始 |

規格在 [`docs/spec/`](docs/spec/)，標 `DRAFT` 或 `READY`；
**只有 READY 的可以動手**。

## 怎麼跑

建置與測試一律走 docker，不裝到系統環境：

```sh
tools/go.sh build ./...
tools/go.sh test ./...

tools/fetch_cputests.sh                  # 抓 SingleStepTests/8088 v2（761 MB）
tools/go.sh test ./internal/cpu -run TestSingleStep
```

測試語料**不進版控**，也不隨本儲存庫散布——它有自己的授權。
沒抓語料時 CPU 測試會 skip，不會假裝通過。

## 不含原版素材

本儲存庫**沒有** `RUN.EXE`、`.PIX`、`.PAK` 或任何《大富翁2》的檔案。
要跑原版得自己有一份合法的。缺檔的測試一律 skip，
**不用自製代用品**——安靜的替代品會讓「還沒做完」看起來像做完了。

## 授權

採 **RRSAL-1.0**（復古重製 source-available 授權條款 1.0），
全文見 [`LICENSE`](LICENSE)。非商業使用、修改與再散布免費且不必事先同意；
商業使用要另洽 `wicanr2@gmail.com`。

⚠ 這一份的定位其實是**工具**，不是 remake 本身，而 RRSAL 是為 remake 寫的
非商業條款。沿用它是為了與 `rich2` 家族一致；
如果要讓別人拿去做自己的模擬器，換成 MIT／Apache-2.0 更合適。
