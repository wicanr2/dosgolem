# 144 — CPU386 SIB＋disp8 byte OR

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 101`](../re/101-fd2-iomode-tests-file-state.md)

## 範圍

- 擴充 opcode `80 /1`，支援固定原版的 ModRM `4Ch` 與 SIB `03h`：
  `OR byte ptr [ebx+eax+disp8],imm8`。
- disp8 以有號值加入 EBX 與 EAX；由 DS selector 讀取、OR 後寫回同一 byte，
  再以既有 `setLogicFlags8` 更新旗標。

## 失敗即關閉

- 只接受這個已證實的 SIB shape；其他 SIB、scale、index、base、prefix 與
  記憶體形狀維持拒絕。
- DS descriptor 不可讀或不可寫時不得部分成功；來源 byte 保持不變。

## 驗收

- CPU 單元測試驗證有號 disp8、寫回、旗標、唯讀失敗與未授權 SIB 拒絕。
- 固定原版由 LE entry 自然執行 `0x4637D`，確認目標 FILE 狀態 byte 的
  bit `0x40` 由未設變為已設並抵達 `0x46382`。

本規格不宣告 `isatty` 或 DOS IOCTL 已完成。

驗收收據（2026-09-06）：`TestByteORAtSIBDisp8` 驗證有號 disp8、寫回、
旗標、唯讀失敗及未授權 SIB 拒絕；`TestFD2MarksIOModeFlag` 由固定原版
LE entry 自然執行 `0x4637D`，確認 FILE 狀態 byte 的 bit `0x40` 由未設
變為已設並抵達 `0x46382`。後續有界探針進入 `isatty`，下一阻塞移至
`0x3FB16` 的 operand-size `89 C3`。
