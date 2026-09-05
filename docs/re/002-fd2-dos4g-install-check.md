# FD2 DOS/4G 安裝檢查返回狀態

日期：2026-09-05  
證據等級：**已證實**（固定雜湊原版、IDA 直接指令、DOSBox-X 同狀態執行）

## 輸入與工具

- 檔案：`FD2.EXE`，357,074 bytes
- MD5：`b97caf2239a27a896069d03549d96e1e`
- SHA-256：`222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`
- 靜態工具：授權 Docker 內 IDA Pro 9.4，LE loader 線性位址空間
- 執行工具：Docker image `fd2-dosbox-x:debug-0d7b272b`
  （image ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`）
- DOSBox-X 執行期自報：`2026.07.02 Commit 6fb8c07`，heavy debugger
- 外部契約：Ralf Brown Interrupt List，
  <https://delorie.com/djgpp/doc/rbinter/id/75/40.html>

## 定位方法

DOSBox-X 的 `BPLM` 是「線性記憶體內容變更」斷點，不是執行斷點；直接把 IDA
位址 `3CA76` 傳給它不會證明程式執行至該處。實驗改採下列可重播定位鏈：

1. `BPINT 21 30` 攔截 DOS 版本查詢；
2. 略過載入器呼叫，直到 `EBX=50484152h`，與 IDA `0x3CA03` 的 FD2 caller 一致；
3. 原版當下為 `CS:EIP=0158:001F2A03`；
4. IDA 直接控制流證實 `0x3CA74-0x3CA03=0x71`，因此在同一 selector 設
   `BP 0158:001F2A74`；
5. 呼叫返回點設為 `BP 0158:001F2A76`。兩個執行斷點均命中，且位元組分別是
   `CD 21` 與 `3C 00`。

早先以 `BPINT 21 FF 00` 取得的 `CS:EIP=3E58:463F, DX=0D00h` 等狀態屬
DOS/4GW 載入器內部檢查，不是 FD2 caller，已排除。

最終實驗在一次性、無網路 Docker 容器執行，原版目錄唯讀掛載、遊戲複本只寫入
容器 `/tmp`；以 Xvfb、tmux 真正終端與 xdotool 對 SDL 視窗送鍵。斷點定位與結果
解析皆有 20–60 秒上限，命中條件同時核對 caller signature、selector、EIP 與
目標指令，不以輸出中曾出現位址字串判定成功。

## 已證實暫存器

`INT 21h` 前（IDA 線性位址 `0x3CA74`）：

```text
EAX=0000FF00 ESI=00000000 DS=0160 ES=0028 FS=0000 GS=0020 SS=0160
EBX=5048FF00 EDI=00000081 CS=0158 EIP=001F2A74
ECX=00000005 EBP=00000000 EDX=00000078 ESP=0019E690
```

返回後（IDA 線性位址 `0x3CA76`）：

```text
EAX=4734FFFF ESI=00000000 DS=0160 ES=0028 FS=0000 GS=0020 SS=0160
EBX=5048FF00 EDI=00000081 CS=0158 EIP=001F2A76
ECX=00000005 EBP=00000000 EDX=00000078 ESP=0019E690
next: 3C 00  cmp al,00
```

RBIL 的通用契約只保證 `AL != 0` 表示已安裝，並在成功時由 `GS` 提供 kernel
segment。本固定原版環境進一步證實完整 `EAX=4734FFFFh` 與 `GS=0020h`；這兩值
只能用於本次固定 DOS/4GW 執行環境，不可宣稱所有版本皆相同。

## 實作邊界

本證據足以讓 dosgolem 的 FD2 LE 啟動服務回傳固定 oracle 值，並執行
`cmp al,0`／`jz` 的成功分支。不足以證明 descriptor base、`ES:[2Ch]`、DOS 指標
轉換或一般化 DOS/4GW API；這些仍失敗即關閉。
