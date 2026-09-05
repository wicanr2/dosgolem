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

// ToFirstCharacterClass 由第一角色種族頁進入職業選擇頁。
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

// ToFirstCharacterAlignment 由第一角色職業頁進入陣營選擇頁。
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

// ToFirstCharacterStats 由第一角色陣營頁進入屬性／肖像頁。
func ToFirstCharacterStats(o *oracle.Oracle) error {
	if err := ToFirstCharacterAlignment(o); err != nil {
		return err
	}
	o.PressKey(oracle.KeyEnter)
	if err := o.RunUntil(screenDigest("第一角色屬性／肖像頁", 130, 55, 180, 140,
		"d57c5b9740bf8708984ab7da91f950755a1255d06f0bcc4555500d294f95d42e"), oracle.Budget(10_000_000)); err != nil {
		return fmt.Errorf("EOB1等待第一角色屬性頁：%w", err)
	}
	return nil
}

// ToFirstCharacterReview 由屬性／肖像頁進入含Reroll／Modify／Faces／Keep的檢視頁。
func ToFirstCharacterReview(o *oracle.Oracle) error {
	if err := ToFirstCharacterStats(o); err != nil {
		return err
	}
	o.PressKey(oracle.KeyEnter)
	if err := o.RunUntil(screenDigest("第一角色檢視操作頁", 130, 55, 180, 140,
		"1f1bbba36dc25a84f6ce2ebb4f01a60dc64d2b7ddc63bb9ffa3d25d8e47c164f"), oracle.Budget(10_000_000)); err != nil {
		return fmt.Errorf("EOB1等待第一角色檢視頁：%w", err)
	}
	return nil
}

// ToFirstCharacterName 以原版KEEP按鈕接受目前角色，進入姓名輸入頁。
func ToFirstCharacterName(o *oracle.Oracle) error {
	if err := ToFirstCharacterReview(o); err != nil {
		return err
	}
	if err := o.Click(282, 180); err != nil {
		return fmt.Errorf("EOB1點擊第一角色KEEP：%w", err)
	}
	if err := o.RunUntil(screenDigest("第一角色Name輸入頁", 0, 0, oracle.Width, oracle.Height,
		"ae05e3fff97f306ec714c79603509ec45a4778f152d8ccca2943faef23c47a89"), oracle.Budget(10_000_000)); err != nil {
		return fmt.Errorf("EOB1等待第一角色姓名輸入：%w", err)
	}
	return nil
}

// ToFirstCharacterALFA 在原版姓名頁鍵入ALFA並確認，返回四槽建角總覽。
func ToFirstCharacterALFA(o *oracle.Oracle) error {
	if err := ToFirstCharacterName(o); err != nil {
		return err
	}
	for _, key := range []oracle.Key{oracle.KeyA, oracle.KeyL, oracle.KeyF, oracle.KeyA} {
		o.PressKey(key)
		if err := o.Run(250_000); err != nil {
			return fmt.Errorf("EOB1輸入第一角色姓名：%w", err)
		}
	}
	o.PressKey(oracle.KeyEnter)
	if err := o.RunUntil(screenDigest("第一角色ALFA姓名列", 5, 100, 70, 16,
		"cbc568f33b26cf8dd7809e3f5081b8dc9c63049de4ab7f7caf6c406443109b7c"), oracle.Budget(10_000_000)); err != nil {
		return fmt.Errorf("EOB1等待第一角色ALFA完成：%w", err)
	}
	return nil
}

// ToSecondCharacterRace 選取第二角色槽，進入其種族／性別頁。
func ToSecondCharacterRace(o *oracle.Oracle) error {
	if err := ToFirstCharacterALFA(o); err != nil {
		return err
	}
	if err := o.Click(98, 82); err != nil {
		return fmt.Errorf("EOB1點擊第二角色槽：%w", err)
	}
	if err := o.RunUntil(screenDigest("第二角色SELECT RACE頁", 138, 60, 170, 130,
		"0ac2799d93a1c51adcd884eeefa770b5dc01fbfba36311000a8b5f9d586fc2b3"), oracle.Budget(10_000_000)); err != nil {
		return fmt.Errorf("EOB1等待第二角色種族選擇：%w", err)
	}
	return nil
}

// ToSecondCharacterReview 由第二角色種族頁走到Human Male Fighter檢視頁。
func ToSecondCharacterReview(o *oracle.Oracle) error {
	if err := ToSecondCharacterRace(o); err != nil {
		return err
	}
	steps := []struct {
		name   string
		digest string
	}{
		{"第二角色SELECT CLASS頁", "31954c4eabf3fe236734a89b17447907ac3c54448a962b4ffd0432a29e2777eb"},
		{"第二角色SELECT ALIGNMENT頁", "b701f7d8e0738aaf110f702f34a53d3ab9f0c01b60fd0f7e514bdd773a7bafbc"},
		{"第二角色屬性／肖像頁", "b0794062e92812ef3321b3e3184427a3b2440d6671f6ffd3dffe0fb71e0aa49d"},
		{"第二角色檢視操作頁", "46c7b7647dd6ae8102a8b970315b5597c422a0690308b05668d548d6bdc40759"},
	}
	for _, step := range steps {
		o.PressKey(oracle.KeyEnter)
		if err := o.RunUntil(screenDigest(step.name, 130, 55, 180, 140, step.digest), oracle.Budget(10_000_000)); err != nil {
			return fmt.Errorf("EOB1等待%s：%w", step.name, err)
		}
	}
	return nil
}

// ToSecondCharacterName 以KEEP接受第二角色，進入姓名頁。
func ToSecondCharacterName(o *oracle.Oracle) error {
	if err := ToSecondCharacterReview(o); err != nil {
		return err
	}
	if err := o.Click(282, 180); err != nil {
		return fmt.Errorf("EOB1點擊第二角色KEEP：%w", err)
	}
	if err := o.RunUntil(screenDigest("第二角色Name輸入頁", 0, 0, oracle.Width, oracle.Height,
		"837508573e890228667986bb5932f32dd62732d36c152831984ec504c94bba8b"), oracle.Budget(10_000_000)); err != nil {
		return fmt.Errorf("EOB1等待第二角色姓名輸入：%w", err)
	}
	return nil
}

// ToSecondCharacterBETA 在第二角色姓名頁鍵入BETA並確認。
func ToSecondCharacterBETA(o *oracle.Oracle) error {
	if err := ToSecondCharacterName(o); err != nil {
		return err
	}
	for _, key := range []oracle.Key{oracle.KeyB, oracle.KeyE, oracle.KeyT, oracle.KeyA} {
		o.PressKey(key)
		if err := o.Run(250_000); err != nil {
			return fmt.Errorf("EOB1輸入第二角色姓名：%w", err)
		}
	}
	o.PressKey(oracle.KeyEnter)
	if err := o.RunUntil(screenDigest("第二角色BETA姓名列", 75, 100, 70, 16,
		"26e91595e8209faf10fa52e34ea9d00477ee0fd6c823d3aa1432e623d2a2258c"), oracle.Budget(10_000_000)); err != nil {
		return fmt.Errorf("EOB1等待第二角色BETA完成：%w", err)
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
