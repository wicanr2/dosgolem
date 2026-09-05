// Package eob1 保存《Eye of the Beholder》第一代原版的導航知識。
// 原版執行檔與資料由使用者提供，套件本身不含任何遊戲內容。
package eob1

import (
	"fmt"

	"github.com/wicanr2/dosgolem/oracle"
)

// LauncherInput 是START1.EXE的正常選項：VGA、無音效、使用滑鼠。
const LauncherInput = "44Y"

// ToWestwoodLogo 從START1.EXE冷啟動走到Westwood標誌穩定幀。
func ToWestwoodLogo(o *oracle.Oracle) error {
	o.Type(LauncherInput)
	if err := o.RunUntil(oracle.Opened("EOBDATA4.PAK"), oracle.Budget(20_000_000)); err != nil {
		return fmt.Errorf("EOB1等待標誌資產：%w", err)
	}
	if err := o.RunUntil(oracle.ScreenIdle(1_000_000), oracle.Budget(20_000_000)); err != nil {
		return fmt.Errorf("EOB1等待Westwood標誌穩定：%w", err)
	}
	return nil
}
