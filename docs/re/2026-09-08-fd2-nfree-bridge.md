# FD2 近堆配置與釋放轉接缺口

輸入FD2.EXE 357074 bytes；MD5 b97caf2239a27a896069d03549d96e1e；
SHA-256 222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f。
工具IDA Pro 9.4，locked-v1；本頁位址均為IDA LE載入器線性位址，
執行軌跡另標dosgolem重定位LE線性。原始名稱不改名。

**已證實**：IDA函式_nfree為37426..3744B，37436從[EBP+0C]取得
入參（前面兩次push後即入口ESP+4）；3743C呼叫__MemFree，
37441以C6 05 9C 41 05 00 00清byte_5419C，還原EBP/EBX後RETN。
__MemFree在3D32C先ptr-4、3D32F讀dword，再依bit0處理，
3D339把去bit0後的值加到ptr-4；3D33B讀取新指標。

**已證實的執行器缺口**：既有_nmalloc轉接只提供物件bytes，沒有原生header。
自然第79182步進__MemFree，ptr=88564；ptr-4內容00767879，
因此形成7EFDD8（8322520），超過實際Mem長度565248。
該header讀到的是相鄰資料，不能靠放寬CPU界限或補新opcode修正。
較早的釋放可能因前方資料bit0為0而提早返回，不能當轉接完整證據。

修正沿既有RE049的runtime服務邊界，成對實作配置／釋放，
不逐行還原Watcom內部管理器。正式規格見186批次79。
[原始定位與機械證據](../evidence/fd2-nfree-bridge-20260908.json)。

首次IDA匯出因預設ASCII寫入中文而留下截斷JSON，列為無效工具收據；
明示UTF-8後從同一原檔乾淨重跑成功，不把exit code單獨視為成功。
