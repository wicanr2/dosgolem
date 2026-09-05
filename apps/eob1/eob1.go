// Package eob1 保存《Eye of the Beholder》第一代原版的導航知識。
// 原版執行檔與資料由使用者提供，套件本身不含任何遊戲內容。
package eob1

import (
	"crypto/sha256"
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

// ToTitleMenu 從START1.EXE冷啟動走正常VGA路徑，略過片頭並等待主選單穩定。
func ToTitleMenu(o *oracle.Oracle) error {
	if err := ToWestwoodLogo(o); err != nil {
		return err
	}
	o.PressKey(oracle.KeyEscape)
	if err := o.RunUntil(oracle.Opened("EOBDATA3.PAK"), oracle.Budget(20_000_000)); err != nil {
		return fmt.Errorf("EOB1等待主選單資產：%w", err)
	}
	if err := o.RunUntil(oracle.ScreenIdle(1_000_000), oracle.Budget(20_000_000)); err != nil {
		return fmt.Errorf("EOB1等待主選單穩定：%w", err)
	}
	return nil
}

// ToNewPartyCreation 從冷啟動正常選取START A NEW PARTY，走到建角入口完成繪製。
func ToNewPartyCreation(o *oracle.Oracle) error {
	if err := ToTitleMenu(o); err != nil {
		return err
	}
	o.PressKey(oracle.KeyDown)
	if err := o.RunUntil(screenDigest("主選單選中START A NEW PARTY", 0, 0, oracle.Width, oracle.Height,
		"c99dc9bf3aabbe1823fdf0a91620344e33a3a8f3827ad8dedda931f24f25e79f"), oracle.Budget(5_000_000)); err != nil {
		return fmt.Errorf("EOB1選取新隊伍：%w", err)
	}
	o.PressKey(oracle.KeyEnter)
	header := screenDigest("Character Generation標題", 32, 8, 256, 40,
		"2496edf386e334b90af5de1a670f171e21dd1591b0b574be84847d69177d85f0")
	instructions := screenDigest("建角操作說明", 130, 55, 180, 140,
		"39cb98087f15b0604262c4767b6a7df50593a4d29e604b4a778db58a2df3e3f6")
	both := oracle.NewCond("建角標題與操作說明完成", func(o *oracle.Oracle) bool {
		return header.Ready(o) && instructions.Ready(o)
	})
	if err := o.RunUntil(both, oracle.Budget(20_000_000)); err != nil {
		return fmt.Errorf("EOB1等待建角入口：%w", err)
	}
	return nil
}

// ToFirstCharacterRace 從冷啟動走正常建角入口，開啟第一位角色的種族／性別選擇頁。
func ToFirstCharacterRace(o *oracle.Oracle) error {
	if err := ToNewPartyCreation(o); err != nil {
		return err
	}
	o.PressKey(oracle.KeyEnter)
	if err := o.RunUntil(screenDigest("第一角色SELECT RACE頁", 138, 60, 170, 130,
		"34cd6ac1502742323eb299439fe6066154336a9f42f372457eec4d8b5e3b13f6"), oracle.Budget(10_000_000)); err != nil {
		return fmt.Errorf("EOB1等待第一角色種族選擇：%w", err)
	}
	return nil
}

// ToFirstCharacterClass 選定第一角色預設Human Male，進入職業選擇頁。
func ToFirstCharacterClass(o *oracle.Oracle) error {
	if err := ToFirstCharacterRace(o); err != nil {
		return err
	}
	o.PressKey(oracle.KeyEnter)
	if err := o.RunUntil(screenDigest("第一角色SELECT CLASS頁", 138, 60, 170, 130,
		"731c3a60b6eec89515314cf85aa352bcd4efad241d3aea7bb5106454aee7b890"), oracle.Budget(10_000_000)); err != nil {
		return fmt.Errorf("EOB1等待第一角色職業選擇：%w", err)
	}
	return nil
}

// ToFirstCharacterAlignment 選定第一角色預設Fighter，進入陣營選擇頁。
func ToFirstCharacterAlignment(o *oracle.Oracle) error {
	if err := ToFirstCharacterClass(o); err != nil {
		return err
	}
	o.PressKey(oracle.KeyEnter)
	if err := o.RunUntil(screenDigest("第一角色SELECT ALIGNMENT頁", 138, 60, 170, 130,
		"aade7402dce32dc8a71b9ca7d1ed057836487dc67b27e0fc482cef512da62b40"), oracle.Budget(10_000_000)); err != nil {
		return fmt.Errorf("EOB1等待第一角色陣營選擇：%w", err)
	}
	return nil
}

func screenDigest(name string, x, y, width, height int, want string) oracle.Cond {
	var next uint64
	return oracle.NewCond(name, func(o *oracle.Oracle) bool {
		if o.Steps() < next {
			return false
		}
		next = o.Steps() + 100_000
		indexed := o.Indexed()
		region := make([]byte, 0, width*height)
		for row := y; row < y+height; row++ {
			start := row*oracle.Width + x
			region = append(region, indexed[start:start+width]...)
		}
		return fmt.Sprintf("%x", sha256.Sum256(region)) == want
	})
}
