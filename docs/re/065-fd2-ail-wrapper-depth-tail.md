# 065 — FD2 AIL 包裝器共用計數尾端

日期：2026-09-06
證據等級：原始指令、writer、函式尾端與 caller 為**已證實**；
`dword_54178` 是包裝器巢狀深度為**強推論**
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`
工具：IDA Pro 9.4.0.260610，映像 `ida-pro-9.4-idapython:locked-v1`
位址空間：IDA 載入 LE 映像後的 32 位線性位址

IDA 保留 `sub_38D3B`（`0x38D3B..0x38E26`），由 FD2 `main` 的
`0x25C2B` 直接呼叫。函式入口 `0x38D3E..0x38D45` 讀取
`dword_54178`、加一並寫回；所有正常與略過記錄分支匯合至
`0x38E1A` 的 `dec dword ptr [54178h]`，隨後在 `0x38E20` 恢復結果並返回。

AIL preference 包裝器 `sub_37C20` 也在入口對同一欄位加一，並跳往
`0x38E1A` 共用尾端。這足以證實 `0x38E1A` 是成對遞減 writer；欄位代表
AIL 包裝器巢狀深度是強推論，不作為遊戲規則或玩家狀態接入。

一次性 IDA 報告 SHA-256：
`1592d6af942c9496ac86f19b6fc94ea55a3c4f7c564c327d0c755226ab98334e`；
一次性 `.i64` SHA-256：
`d2cff8cef5e58f6725b1b8d548d7aee033bde21b51e188ddaedb1280251d723d`。
