# 規格索引

`docs/spec/` 是 dosgolem 的規格目錄。SDD 的規則沒變：**只有標 `READY`
的規格可以動手**，反組譯／量測 → 規格 → 才寫程式。

## 引用規格時要連號碼帶檔名

目前有 249 份規格，而編號**不是唯一鍵**：十條分支各自從 007 開始編，
合併之後同一個號碼底下有好幾份不同主題的文件。

| 號碼 | 底下有幾份 | 檔名 |
|---:|---:|---|
| `004` | 2 | `004-dos-bios-services.md`、`004-machine.md` |
| `007` | 6 | `007-com-loader-and-keyboard.md`、`007-dos-exec-overlay.md`、`007-ega-mode-0dh-planar-vram.md`、`007-exec-ems.md`、`007-linear-executable-intake.md`、`007-vga-planar.md` |
| `008` | 7 | `008-bios-keyboard-injection.md`、`008-blocking-console-input.md`、`008-ems-paging.md`、`008-eob1-adapter.md`、`008-flat-386-entry.md`、`008-tsr-resident.md`、`008-wolong-services.md` |
| `009` | 8 | `009-bios-time-of-day.md`、`009-exec.md`、`009-fd2-dos4g-install-check.md`、`009-keyboard-irq1.md`、`009-memory-allocator.md`、`009-mouse-event-handler.md`、`009-planar-vga.md`、`009-scratch-writes.md` |
| `010` | 6 | `010-dosv.md`、`010-eob1-title-menu.md`、`010-exec-memory-reclaim-and-planar-write-modes.md`、`010-fd2-segment-bootstrap.md`、`010-input-verbs.md`、`010-overlay-loading.md` |
| `011` | 5 | `011-bios-palette-and-vector-stubs.md`、`011-ega-mode10h.md`、`011-eob1-new-party-entry.md`、`011-fd2-es-environment-cell.md`、`011-xms.md` |
| `012` | 5 | `012-cpu-386-subset.md`、`012-eob1-first-character-race.md`、`012-fd2-parity-capture.md`、`012-input-keyboard-irq-and-mouse-scale.md`、`012-mcb-chain.md` |
| `013` | 5 | `013-eob1-first-character-class.md`、`013-fd2-load-es-selector.md`、`013-file-handles.md`、`013-mouse-event-callback.md`、`013-vga-planar.md` |
| `014` | 5 | `014-dos4gw-flat-descriptors.md`、`014-ems.md`、`014-eob1-first-character-alignment.md`、`014-hardware-keyboard.md`、`014-poke-state-not-luck.md` |
| `015` | 4 | `015-bytecode-observation.md`、`015-eob1-first-character-stats.md`、`015-fd2-store-environment-word.md`、`015-outermost-cmdline.md` |
| `016` | 3 | `016-dos4gw-protected-push.md`、`016-eob1-first-character-review.md`、`016-right-mouse-button.md` |
| `017` | 2 | `017-dos4gw-selector-load-validation.md`、`017-mouse-callback.md` |
| `018` | 2 | `018-cpu386-command-tail-prelude.md`、`018-eob1-first-character-name.md` |
| `019` | 2 | `019-dos4gw-psp-command-tail.md`、`019-eob1-first-character-alfa.md` |
| `020` | 2 | `020-cpu386-repe-scasb.md`、`020-eob1-second-character-race.md` |
| `021` | 2 | `021-cpu386-lea-disp8.md`、`021-eob1-second-character-review.md` |
| `022` | 2 | `022-dos4gw-selector-swap-jz.md`、`022-eob1-second-character-beta.md` |
| `023` | 2 | `023-cpu386-buffer-stack-finalize.md`、`023-eob1-third-character-review.md` |
| `024` | 2 | `024-dos4gw-environment-selector-load.md`、`024-eob1-third-character-gamma.md` |
| `025` | 2 | `025-dos4gw-environment-block.md`、`025-eob1-fourth-character-review.md` |
| `026` | 2 | `026-cpu386-environment-prefix-test.md`、`026-eob1-fourth-character-delta.md` |
| `027` | 2 | `027-cpu386-environment-byte-scan.md`、`027-dos-dta-find-first.md` |
| `028` | 2 | `028-cpu386-pop-ds.md`、`028-eob1-play-level1-entry.md` |
| `029` | 2 | `029-cpu386-startup-buffer-clear.md`、`029-eob1-level1-first-move.md` |
| `030` | 2 | `030-cpu386-startup-buffer-tail.md`、`030-eob1-level1-first-pickup.md` |
| `031` | 2 | `031-cpu386-near-call.md`、`031-eob1-level1-first-drop.md` |
| `032` | 2 | `032-cpu386-push-es.md`、`032-eob1-level1-camp-lifecycle.md` |
| `033` | 2 | `033-cpu386-register-cmp-jae.md`、`033-eob1-level1-memorize-spell.md` |

所以：**引用寫 `docs/spec/009-memory-allocator`，不要只寫 `docs/spec/009`**。
程式碼註解裡既有的「`docs/spec/NNN`」是各分支寫的，指的是那條分支自己的
那一份——照上表對回去（用寫那一行的子系統判斷），不要照號碼硬套。

> 沒有重編號是刻意的取捨：註解裡幾百處「`docs/spec/NNN`」在合併之後每一處
> 都有 2–7 個候選，重編號要逐處判斷它原本指哪一份。判錯的結果是一個
> **看起來對、其實指到別份規格**的指標，比號碼重複更難發現。

## 依案例分組

### 共同基礎（master）（13 份）

- `001-scope-and-mvp.md`
- `002-cpu-8086.md`
- `003-machine-and-loader.md`
- `004-dos-bios-services.md`
- `005-oracle-api.md`
- `006-layering.md`
- `187-frame-clock.md`
- `188-pit-divisor-decoding.md`
- `189-a20-gate-and-hma-addressing.md`
- `190-irq0-calibration.md`
- `191-two-clocks-relationship.md`
- `192-crtc-effects.md`
- `193-crtc-timing.md`

### 銀河英雄傳說 3（logh3）（10 份）

- `007-exec-ems.md`
- `008-ems-paging.md`
- `009-planar-vga.md`
- `010-exec-memory-reclaim-and-planar-write-modes.md`
- `011-bios-palette-and-vector-stubs.md`
- `012-input-keyboard-irq-and-mouse-scale.md`
- `013-mouse-event-callback.md`
- `014-poke-state-not-luck.md`
- `015-outermost-cmdline.md`
- `016-right-mouse-button.md`

### 源平合戰（yuan-genpei）（9 份）

- `004-machine.md`
- `008-tsr-resident.md`
- `009-exec.md`
- `010-dosv.md`
- `011-xms.md`
- `012-cpu-386-subset.md`
- `013-vga-planar.md`
- `014-ems.md`
- `015-bytecode-observation.md`

### 三國演義（san1）（7 份）

- `008-blocking-console-input.md`
- `009-memory-allocator.md`
- `010-overlay-loading.md`
- `011-ega-mode10h.md`
- `012-mcb-chain.md`
- `013-file-handles.md`
- `014-hardware-keyboard.md`

### 臥龍傳（wolong）（4 份）

- `007-vga-planar.md`
- `008-wolong-services.md`
- `009-mouse-event-handler.md`
- `010-input-verbs.md`

### Eye of the Beholder（eob1）（28 份）

- `007-dos-exec-overlay.md`
- `008-eob1-adapter.md`
- `009-bios-time-of-day.md`
- `009-keyboard-irq1.md`
- `010-eob1-title-menu.md`
- `011-eob1-new-party-entry.md`
- `012-eob1-first-character-race.md`
- `013-eob1-first-character-class.md`
- `014-eob1-first-character-alignment.md`
- `015-eob1-first-character-stats.md`
- `016-eob1-first-character-review.md`
- `017-mouse-callback.md`
- `018-eob1-first-character-name.md`
- `019-eob1-first-character-alfa.md`
- `020-eob1-second-character-race.md`
- `021-eob1-second-character-review.md`
- `022-eob1-second-character-beta.md`
- `023-eob1-third-character-review.md`
- `024-eob1-third-character-gamma.md`
- `025-eob1-fourth-character-review.md`
- `026-eob1-fourth-character-delta.md`
- `027-dos-dta-find-first.md`
- `028-eob1-play-level1-entry.md`
- `029-eob1-level1-first-move.md`
- `030-eob1-level1-first-pickup.md`
- `031-eob1-level1-first-drop.md`
- `032-eob1-level1-camp-lifecycle.md`
- `033-eob1-level1-memorize-spell.md`

### Pool of Radiance（por）（3 份）

- `007-ega-mode-0dh-planar-vram.md`
- `008-bios-keyboard-injection.md`
- `009-scratch-writes.md`

### UCSD p-System（psys）（1 份）

- `007-com-loader-and-keyboard.md`

### 風之谷 2／DOS4GW（fd2）（177 份）

- `007-linear-executable-intake.md`
- `008-flat-386-entry.md`
- `009-fd2-dos4g-install-check.md`
- `010-fd2-segment-bootstrap.md`
- `011-fd2-es-environment-cell.md`
- `012-fd2-parity-capture.md`
- `013-fd2-load-es-selector.md`
- `014-dos4gw-flat-descriptors.md`
- `015-fd2-store-environment-word.md`
- `016-dos4gw-protected-push.md`
- `017-dos4gw-selector-load-validation.md`
- `018-cpu386-command-tail-prelude.md`
- `019-dos4gw-psp-command-tail.md`
- `020-cpu386-repe-scasb.md`
- `021-cpu386-lea-disp8.md`
- `022-dos4gw-selector-swap-jz.md`
- `023-cpu386-buffer-stack-finalize.md`
- `024-dos4gw-environment-selector-load.md`
- `025-dos4gw-environment-block.md`
- `026-cpu386-environment-prefix-test.md`
- `027-cpu386-environment-byte-scan.md`
- `028-cpu386-pop-ds.md`
- `029-cpu386-startup-buffer-clear.md`
- `030-cpu386-startup-buffer-tail.md`
- `031-cpu386-near-call.md`
- `032-cpu386-push-es.md`
- `033-cpu386-register-cmp-jae.md`
- `034-cpu386-byte-record-scan.md`
- `035-cpu386-callback-pointer-gate.md`
- `036-cpu386-segment-transfer-indirect-call.md`
- `037-cpu386-near-jump.md`
- `038-cpu386-x87-init-control.md`
- `039-cpu386-x87-control-return.md`
- `040-cpu386-record-status-write.md`
- `041-cpu386-absolute-byte-cmp.md`
- `042-cpu386-register-byte-xor.md`
- `043-fd2-second-callback-x87-store.md`
- `044-fd2-second-callback-x87-class-gate.md`
- `045-cpu386-movzx-absolute-word.md`
- `046-fd2-control-baseline-dispatch.md`
- `047-cpu386-x87-self-test.md`
- `048-fd2-second-callback-class-result.md`
- `049-fd2-second-callback-record-state.md`
- `050-fd2-third-callback-selection.md`
- `051-cpu386-push-pop-fs.md`
- `052-cpu386-group83-sub-cmp.md`
- `053-cpu386-lfs-absolute.md`
- `054-cpu386-stack-mov-xor.md`
- `055-cpu386-environment-es-byte-gate.md`
- `056-cpu386-sub-stack-memory.md`
- `057-watcom-near-heap-runtime.md`
- `058-cpu386-register-test.md`
- `059-cpu386-register-shl.md`
- `060-cpu386-register-add.md`
- `061-fd2-environment-copy.md`
- `062-cpu386-mov-eax-moffs.md`
- `063-cpu386-sib-immediate-store.md`
- `064-cpu386-push-sign-extended-byte.md`
- `065-watcom-memset-runtime.md`
- `066-cpu386-leave.md`
- `067-cpu386-null-data-selector.md`
- `068-cpu386-absolute-byte-and-or.md`
- `069-cpu386-cmp-base-disp8.md`
- `070-cpu386-store-base-disp8.md`
- `071-cpu386-load-register-absolute.md`
- `072-cpu386-store-register-indirect.md`
- `073-cpu386-store-immediate-absolute.md`
- `074-watcom-init-argv-runtime.md`
- `075-dos-current-time-deterministic.md`
- `076-cpu386-register-cmp.md`
- `077-cpu386-register-byte-cmp.md`
- `078-cpu386-sub-register-absolute.md`
- `079-cpu386-sub-register.md`
- `080-cpu386-push-absolute-dword.md`
- `081-dos4gw-capability-matrix.md`
- `082-cpu386-push-immediate-dword.md`
- `083-cpu386-xchg-register-stack-disp8.md`
- `084-cpu386-neg-register.md`
- `085-cpu386-cmp-register-absolute.md`
- `086-cpu386-jbe-short.md`
- `087-cpu386-load-register-stack-disp8.md`
- `088-cpu386-ret-immediate.md`
- `089-cpu386-store-immediate-stack-sib.md`
- `090-cpu386-store-register-stack-disp8.md`
- `091-cpu386-and-eax-immediate.md`
- `092-cpu386-and-register-immediate.md`
- `093-cpu386-lea-stack-disp8.md`
- `094-watcom-int386-dpmi-lock.md`
- `095-cpu386-cmp-stack-disp8-immediate.md`
- `096-cpu386-setz-register-byte.md`
- `097-cpu386-add-register-stack-disp8.md`
- `098-cpu386-push-base-disp8-dword.md`
- `099-cpu386-repne-scasb.md`
- `100-cpu386-not-register.md`
- `101-cpu386-mov-absolute-indexed-sib.md`
- `102-cpu386-mov-to-absolute-indexed-sib.md`
- `103-cpu386-dec-absolute-dword.md`
- `104-cpu386-cmp-register-imm8.md`
- `105-cpu386-jl-short.md`
- `106-cpu386-pushfd.md`
- `107-cpu386-cli.md`
- `108-cpu386-store-segment-absolute-word.md`
- `109-cpu386-load-segment-absolute-word.md`
- `110-dpmi-get-real-mode-interrupt-vector.md`
- `111-cpu386-register-mov16.md`
- `112-dos386-interrupt-vectors.md`
- `113-cpu386-store-segment-register16.md`
- `114-cpu386-load-segment-register16.md`
- `115-cpu386-inc-absolute-dword.md`
- `116-cpu386-store-register-base-disp32.md`
- `117-cpu386-store-immediate-base-disp32.md`
- `118-cpu386-cmp-base-disp32-immediate.md`
- `119-cpu386-short-jb.md`
- `120-cpu386-load-register-base-disp32.md`
- `121-cpu386-cmp-base-disp8-immediate32.md`
- `122-cpu386-port-output.md`
- `123-cpu386-test-byte-base-disp8.md`
- `124-cpu386-popfd.md`
- `125-cpu386-sub-register-immediate32.md`
- `126-cpu386-lea-stack-disp32.md`
- `127-cpu386-mov-stack-disp32-read.md`
- `128-cpu386-cmp-register-immediate32.md`
- `129-cpu386-and-byte-base-disp8.md`
- `130-cpu386-movzx-byte-esi.md`
- `131-cpu386-jg-short.md`
- `132-cpu386-or-al-immediate8.md`
- `133-cpu386-or-register8-immediate8.md`
- `134-cpu386-or-base-disp8-register.md`
- `135-cpu386-movzx-byte-eax.md`
- `136-cpu386-mov-byte-base-disp8-register.md`
- `137-cpu386-or-register8-base-disp8.md`
- `138-cpu386-mov-immediate-base-disp8.md`
- `139-read-only-dos-file-provider.md`
- `140-fd2-le-dos-open-readonly.md`
- `141-cpu386-rcl-ror-one.md`
- `142-cpu386-movzx-register-word.md`
- `143-cpu386-test-byte-sib-disp8.md`
- `144-cpu386-or-byte-sib-disp8.md`
- `145-cpu386-mov-register-word-store.md`
- `146-fd2-le-dos-ioctl-device-info.md`
- `147-cpu386-test-register-byte.md`
- `148-cpu386-setnz-register-byte.md`
- `149-cpu386-movzx-register-byte.md`
- `150-cpu386-load-dword-scaled-index.md`
- `151-cpu386-store-dword-scaled-index.md`
- `152-cpu386-short-jle.md`
- `153-cpu386-dec-dword-base-disp8.md`
- `154-cpu386-short-jge.md`
- `155-cpu386-or-byte-base-disp8.md`
- `156-cpu386-push-base-dword.md`
- `157-fd2-le-dos-read.md`
- `158-cpu386-inc-dword-base.md`
- `159-cpu386-load-register-base.md`
- `160-cpu386-store-register-byte-base.md`
- `161-cpu386-load-register-byte-base.md`
- `162-cpu386-load-register-byte-sib-disp32.md`
- `163-cpu386-inc-register-byte.md`
- `164-cpu386-test-byte-base-disp32.md`
- `165-cpu386-store-register-byte-sib-disp32.md`
- `166-cpu386-store-register-stack-disp32.md`
- `167-cpu386-store-immediate-byte-base.md`
- `168-cpu386-cmp-byte-base.md`
- `169-cpu386-add-byte-register.md`
- `170-cpu386-test-byte-register.md`
- `171-cpu386-store-dword-stack-base.md`
- `172-cpu386-store-immediate-stack-disp8.md`
- `173-cpu386-lea-edi-ebp.md`
- `174-cpu386-cmp-register-stack-disp8.md`
- `175-cpu386-movzx-byte-ebx-disp32.md`
- `176-cpu386-load-al-edi-ebp.md`
- `177-cpu386-load-eax-stack-base.md`
- `178-cpu386-imul-eax-stack-disp8.md`
- `179-cpu386-store-ax-stack-disp32.md`
- `180-cpu386-neg-stack-disp8.md`
- `181-cpu386-compare-ebx-ss-ebp-disp8.md`
- `182-cpu386-compare-edx-ds-eax-disp8.md`
- `183-fd2-le-dos-seek.md`
- `184-mvp-scope-review.md`
- `185-keyboard-trace-and-keypad-names.md`


- [185 — SS 段覆寫的16位元 MOV 記憶體寫入](185-cpu386-ss-word-store.md)：CONFORMED。

- [186 — FD2平台缺口持續驗證](186-fd2-platform-gap-continuation.md)：原版標題與BIOS單次方向鍵已驗證，正常START進王宮；目前CPU缺口、音訊與輸入限制統一見186檔首。

- [189 — A20 閘門與 HMA 的定址](189-a20-gate-and-hma-addressing.md)：READY。位址遮罩從 CPU 移到匯流排——`段:偏移` 到得了 1 MB 之上，A20 關著時才環繞。

- [190 — 計時器間隔的標定](190-irq0-calibration.md)：READY。165,000 是 17,000 分頻的一刻，不是 65,536 的基準；更正 `004` §5 的公式基準。

- [191 — 兩個時鐘的關係](191-two-clocks-relationship.md)：READY。比值隨指令混合走（3.25～7.40），不是常數；不強制對齊，改成把關係做成看得到的觀測。

- [192 — CRTC 的效果](192-crtc-effects.md)：READY。列距（offset）與分割畫面（line compare）；少了它們畫面會斜成平行四邊形或狀態列跟著捲走。

- [193 — 時序暫存器](193-crtc-timing.md)：READY。從 CRTC 算掃描位置，`3DA` 回報真的回掃狀態；**預設不啟用**，既有對拍收據建立在行為模型上。

- [194 — 交換使用者中斷向量（int 33h AX=0014h）](194-mouse-swap-user-interrupt-vectors.md)：READY。`000Ch` 的「設新的、回舊的」版本；**用這一支的程式一次都不叫 `000Ch`**。

- [195 — 子目錄的路徑解析](195-subdirectory-path-resolution.md)：READY。先試完整路徑再退回 basename；資料真的放在 `data\` 底下的遊戲只有這條路走得通。

- [196 — 取磁碟裝置參數（int 21h AH=44h AL=0Dh CL=60h）](196-ioctl-get-device-parameters.md)：READY。回報成固定硬碟，程式才不會提示換片；BPB 與 `AH=1Bh` 同一組數字。

- [197 — 滑鼠事件常式要進狀態檔](197-mouse-handler-in-saved-state.md)：READY。漏掉它的症狀是「展開之後點擊全部沒反應」，而畫面逐像素相同。

- [198 — DM 巡檢 adapter（`apps/dm`）](198-dm-sweep-adapter.md)：DRAFT。原版側的全層巡檢介入——刻的邊界寫回生命力／食物／水、負重上限換回傳值；四個攔截點的位址由 remake 專案反組譯取得。
