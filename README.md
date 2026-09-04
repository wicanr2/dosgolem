# dosgolem

**只跑得動一個 binary 的 DOS 執行器**：無頭、決定性、可以當 Go 套件 import。
它為《大富翁2》（大宇資訊，1993，DOS）的 remake
[`rich2`](https://github.com/wicanr2/rich2) 而寫。

## 起源：DOSBox-X 不好跟 AI agent 配合

DOSBox-X 是為「人坐在前面玩」設計的——畫面出在 X 上、輸入靠鍵盤事件、
要觀察就得截圖。人玩的時候這樣剛好。

AI agent（Claude Code、Codex）在 remake 專案裡做的是另一件事：**對拍**，
把 remake 的畫面與狀態逐項比對原版。對拍需要的是**問得到、量得到、可重播**，
不是看得到。「這一格的租金算成多少」隔著截圖只能從像素反推；
「跑到收租常式再停下來」隔著 X 只能睡兩秒再猜。

dosgolem 是為這個場景寫的 DOS 執行器：讓「原版怎麼做」可以在 `go test` 裡
直接量，而不是隔著 X 看畫面。

## 名字

dosgolem ＝ DOS 魔像。golem 是被造出來替你做事的自動人偶，
正好是這個工具在 AI 工作流裡的位置；而 "go" 就寫在名字裡。

## 現在這條路的成本

`rich2` 目前的對拍是 docker ＋ Xvfb ＋ DOSBox ＋ xdotool ＋ 截圖，
累積了 54 支腳本、6,116 行，每一支各自重寫一次
「送鍵 → sleep → 截圖 → 判像素」。

| 量 | 值 |
|---|---|
| 一次 `docker exec` 往返 | 0.058 秒 |
| 送一個鍵 | 1 次 exec ＋ 0.35 秒 sleep |
| 取一張截圖 | 2 次 exec ＋ 0.7 秒 sleep |
| 自走腳本每前進一步 | 2.2 秒 sleep |

瓶頸不是 IPC。0.058 秒的往返旁邊掛著 0.35 到 2.2 秒的等待，比例 6 到 38 倍。
那些 sleep 存在的唯一理由是：**主機沒辦法問模擬器「你跑完這一幀了嗎」，
只能拿牆上的時鐘猜。** 那同時是慢的原因與不穩的原因。

派生出來的四個成本：

- 判準只能讀像素。「選單框裡有青字」會在棋盤的青綠色建築上誤中。
- 走到罕見畫面很貴。150 回合擲骰一次都沒踩到法院，最後改存檔碰運氣。
- 相位對不齊。調色盤 240–249／250–254 在輪轉，單張截圖比對會得出假結論。
- 亂數序列對不上。原版 BASIC 的 `RANDOMIZE` 與 remake 的 RNG 沒有可重播對應。

重寫 DOSBox-X 不能解決這件事。它有 943,452 行 C++、4,501 個原始檔、479 MB，
而問題出在「只能從外面看畫面」，不在模擬器本身。它的除錯器又是 ncurses
互動介面，不是可程式化的 API，改它也還是得自己再包一層。

完整評估：[`rich2/docs/spec/082`](https://github.com/wicanr2/rich2/blob/master/docs/spec/082-parity-oracle-emulator.md)。

### DOSBox-X 留著

它是時序的參考實作，也是交叉 oracle。dosgolem 畫出來的每一張，
都要拿 DOSBox-X 的索引截圖驗過才算數——否則就是拿自己驗自己。

## 為什麼做得起來

三個量出來的事實：

1. `RUN_full.EXE` 主程式區 52,892 bytes 只用到 **62 個助憶碼，全部是 8086**。
2. **不需要 x87**：全檔 876 個 `INT 34h–3Dh`，浮點走 Microsoft 浮點模擬器，
   而那個模擬器連結在 binary 裡面，執行期只跑整數指令。
3. 系統服務面很窄，而且 `rich2` 那邊用 unicorn 已經走過一遍、跑到防拷畫面。

## 它長什麼樣

```go
ora := dosgolem.Load("…/RICH2")          // 原版由玩家自備
ora.RunUntil(dosgolem.CallOf(0x25BF6))   // 跑到收租常式
rent := ora.Word("ds:1BE")               // 直接讀原版的變數
shot := ora.Indexed()                    // 320×200 色號，不經過 X

parity.Compare(t, shot, remakeFrame)     // 兩邊都在同一個行程的記憶體裡
```

判準因此從「像素」變成**原版自己的呼叫參數**——攔住繪製常式，
就等於讓原版說出「我在 (154,54) 用色 60 印了第 229 則」。
對拍也變成 CI 跑得動的 `go test`，不再是要人跑三分鐘、還要回頭看截圖的腳本。

⚠ 上面這段 API 是 **DRAFT**（[`docs/spec/005`](docs/spec/005-oracle-api.md)），
**還沒實作**。

## 現況

| | 里程碑 | 狀態 |
|---|---|---|
| MVP-A | 8086 整數指令核心，SingleStepTests/8088 v2 全綠 | **323／323 檔綠**，一項已知差距見下 |
| MVP-B | 跑到防拷畫面，與 DOSBox-X 索引截圖逐點相同 | 未開始 |
| M2 | 輸入與時序（鍵盤／滑鼠／PIT）| 未開始 |
| M3 | 儀器層：breakpoint／watchpoint／call trace／RND 記錄／savestate | 未開始 |
| M4 | Go API 與 `parity` 套件 | 未開始 |

規格在 [`docs/spec/`](docs/spec/)，標 `DRAFT` 或 `READY`；
只有 READY 的可以動手。

CPU 用 [SingleStepTests/8088](https://github.com/SingleStepTests/8088) v2 驗收：
**323 個 opcode 檔、每檔 10,000 筆、共 323 萬筆**，
含每一個匯流排週期的暫存器與記憶體前後狀態。目前唯一沒解的是
**`IDIV` 溢位時推上堆疊的旗標**（[`docs/spec/002`](docs/spec/002-cpu-8086.md) §3.4）。
它以「上限計數」盯著：每一輪照跑照數，超過上限就紅，所以擋得住退步。

### 四條與 Intel 手冊衝突的行為

SingleStepTests 是硬體產生的，與手冊衝突時以它為準。這一路反推出四條：

1. `AAA`／`AAS` 的 `AL + 6` **進位不會跑進 `AH`**——手冊寫的 `AX += 106h`
   在 `AL ≥ FAh` 時會讓 `AH` 多加一次。
2. `DAA`／`DAS` 第二段的條件**不是** `old_AL > 99h OR old_CF`：
   實機在 `old_AL` 落在 9Ah–9Fh、而且進來時 `AF` 已經是 1 的那六個值上不做調整。
3. `D0`–`D3` 的 `/6` 在 8086 **不是 `SHL` 的別名**，是未公開的 `SETMO`
   （把目的地整個設成 1）；186 以上才變成別名。
4. 移位／旋轉的 `OF` **每一圈都重算**，不是只有「移 1 位」時才有定義。
   `D2.3`（`RCR` by `CL`）的 `flags-mask` 是 `FFFF`——一個位元都沒遮。

前兩條都用整份語料（各 10,000 筆）驗過，各 0 筆不合。
照手冊寫這幾處，程式跑得動，但會安靜地跑錯——這就是拿硬體語料當判準、
不照抄手冊的理由。

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
