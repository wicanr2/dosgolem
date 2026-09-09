# 185 — SS 段覆寫的 16 位元 MOV 記憶體寫入
狀態：**CONFORMED**
日期：2026-09-08
證據：[DOSBox-X 交叉驗證](../findings/2026-09-08-fd2-dosbox-seek-diagnosis.md)、
[RE 125](../re/125-fd2-dos-seek-mdi-ini.md)。證據審查：固定 FD2 bytes、caller、
DOS 回傳及兩次原版實際寫入一致；本規格不推導新的遊戲規則。

- 辨識 36h 為 SS 段覆寫，接受與 66h 的兩種順序；仍拒絕重複段／operand prefix。
- 本切片僅放行 66+36+89 的無 SIB 基底暫存器定址：
  mod=00（不含 EBP 的絕對位址例外）或 mod=01（有號 disp8）。
  來源取 ModRM 指定暫存器低16位，目的透過 SS descriptor 寫入小端序 word。
- SS 不得偷換成 DS；檢查完整2 bytes 的 descriptor 權限與界限後才寫入。
  暫存器、段暫存器與旗標不變；成功 EIP 前進實際指令長度。
- 其他 SS 覆寫指令、32位元store、register-only、SIB、disp32 與 repeat prefix
  仍失敗即關閉。不擴大原有 ES 支援集合。
- 驗收：分離 DS/SS base、兩種 prefix 順序、正負 disp8、低字組及鄰接位元組、
  唯讀／未登錄／越界拒絕、unsupported forms；真實 FD2 自然抵達 3CC20，
  逐次核對 AX/DX store、旗標及 EIP=3CC29；最後重跑相關套件及有界自然探針。

## 實跑驗收

2026-09-08：CPU 段分離、寫入寬度與拒絕案例，以及固定 FD2 自然路徑通過。
原版 SS=0160、EDI=00055338，兩次寫入後 value=000000DA、EIP=0003CC29，
暫存器與旗標不變；與前次 DOSBox-X 的操作結果相符。
完整 cpu386／machine／dosfile／FD2 parity 套件通過。
有界自然探針從原入口走到第19,064步，在 0x37381 的 09 C6 停止，
比原阻塞多前進95步。下一個未支援形狀不包含在本規格內。
收據見 [SS store 修正](../evidence/fd2-ss-store-fix-20260908.json)。
