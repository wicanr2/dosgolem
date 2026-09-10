package machine

// frameDots 是 mode 13h 一整幀的 dot 數，**含消隱**：800 × 449。
// 25.175 MHz 的 dot clock 除以它得 70.09 Hz，就是這個模式的更新率。
const frameDots uint64 = 800 * 449

// mode13InputStatus 由指令數推算 `3DA` 的消隱旗標，是標準 mode 13h 的
// hardware-spec approximation。來源與限制：Civ1 `docs/spec/i561-vga-time.md`；
// 不代表實機逐週期一致。
//
// ⚠ **幀長綁呼叫端給的 frameEvery，這裡不自帶第二份標定。** 這個函式在
// Civ1 那份 patch 裡寫死「一幀 42,800 道指令」（＝ 3 M 指令／秒）。
// dosgolem 的標定是 17,000 分頻下的 165,000 道一幀（`DefaultVGAFrameEvery`，
// 考證見 `docs/spec/190-irq0-calibration.md`），兩者差 3.855 倍。留著自己
// 那一份的話，**畫面內容、順序與因果全對，只有時間軸整體縮了**——正是
// 190 裁決掉的那個錯誤，而且不會報錯。
//
// frameEvery ＝ 0 是「不產生幀」，沒有幀就沒有回掃相位可言，回 0。
func mode13InputStatus(steps, frameEvery uint64) uint8 {
	if frameEvery == 0 {
		return 0
	}
	dots := (steps % frameEvery) * frameDots / frameEvery
	line, x := dots/800, dots%800
	var status uint8
	// bit0 ＝ 消隱中（水平或垂直），bit3 ＝ 垂直回掃脈衝。
	if line >= 400 || (x >= 640 && x < 784) {
		status |= 1
	}
	if line >= 412 && line < 414 {
		status |= 8
	}
	return status
}

// stepsAtDot 是 mode13InputStatus 的反向換算：一幀之內走到 dot 位置
// 至少要幾道指令。給測試與對拍腳本用，省得在別處重推一次比例。
func stepsAtDot(dots, frameEvery uint64) uint64 {
	return (dots*frameEvery + frameDots - 1) / frameDots
}
