# 規格019：EOB1第一角色ALFA完成checkpoint

狀態：**CONFORMED**

## 證據與契約

第一角色姓名頁直接接受硬體Set 1字母掃描碼：`A=1Eh`、`L=26h`、`F=21h`。依序送
`A/L/F/A`，每字之間讓原版主迴圈消費250,000道指令，再送Enter；原版將`ALFA`顯示於左上角色
槽並返回四槽建角總覽。此路徑不使用DOS stdin，也不直接改寫角色記憶體。

總覽左側仍有循環效果，全畫面SHA-256隨相位改變；姓名列安全矩形`(5,100) 70×16`固定為
`cbc568f33b26cf8dd7809e3f5081b8dc9c63049de4ab7f7caf6c406443109b7c`。

`apps/eob1.ToFirstCharacterALFA`由冷啟動走既有姓名頁，逐鍵輸入ALFA、確認並以有界預算等待
姓名列。真實資料測試核對雜湊，以及先前八鍵、四字母與確認Enter共十三鍵／二十六次port
`60h`讀取。

## 限制與停止線

本checkpoint證實ASCII fixture `ALFA`與第一角色完成；不外推原版姓名最大長度、其他字元或
系統IME。remake的Unicode／系統IME是獨立產品契約。真實資料、二十六次port `60h`讀取與人眼
檢視均已通過，本規格升為CONFORMED。
