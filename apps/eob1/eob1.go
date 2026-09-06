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

// ToTitleInvalidSaveError 在素材目錄含無效EOBDATA.SAV時，選取LOAD並等待原版錯誤訊息。
func ToTitleInvalidSaveError(o *oracle.Oracle) error {
	if err := ToTitleMenu(o); err != nil {
		return err
	}
	o.PressKey(oracle.KeyEnter)
	if err := o.RunUntil(screenDigest("標題無效存檔錯誤", 0, 0, oracle.Width, oracle.Height,
		"aaa57e4d66a41bc0eaeb464a9318c5aa5bd1c92e369c5a6df633be21e44f3c43"), oracle.Budget(5_000_000)); err != nil {
		return fmt.Errorf("EOB1等待無效存檔錯誤：%w", err)
	}
	return nil
}

// ToTitleInvalidSaveReturn 由無效存檔錯誤按Enter，返回原標題選單。
func ToTitleInvalidSaveReturn(o *oracle.Oracle) error {
	if err := ToTitleInvalidSaveError(o); err != nil {
		return err
	}
	o.PressKey(oracle.KeyEnter)
	if err := o.RunUntil(screenDigest("無效存檔錯誤返回標題", 0, 0, oracle.Width, oracle.Height,
		"caa3082b3e8cb5ee15547555669eb82e954982fe919674d5481271e06a253dc0"), oracle.Budget(5_000_000)); err != nil {
		return fmt.Errorf("EOB1等待無效存檔錯誤返回標題：%w", err)
	}
	return nil
}

// ToSavedGameEntrance 從標題第一列載入有效EOBDATA.SAV，等待原版存檔的地城畫面穩定。
func ToSavedGameEntrance(o *oracle.Oracle) error {
	if err := ToTitleMenu(o); err != nil {
		return err
	}
	o.PressKey(oracle.KeyEnter)
	if err := o.RunUntil(screenDigest("有效原版存檔地城入口", 0, 0, oracle.Width, oracle.Height,
		"823a0224b25517894968eb0cd1ba95bc5e8e3533084b31a75964f13c93639e1b"), oracle.Budget(40_000_000)); err != nil {
		return fmt.Errorf("EOB1等待有效原版存檔載入：%w", err)
	}
	return nil
}

// ToSavedGameProtectionPrepared 從有效原版存檔進CAMP，替TENMIYANA排入Protection From Evil。
func ToSavedGameProtectionPrepared(o *oracle.Oracle) error {
	if err := ToSavedGameEntrance(o); err != nil {
		return err
	}
	if err := o.Click(304, 187, oracle.Hover(0), oracle.Hold(200_000), oracle.Settle(1_000_000)); err != nil {
		return fmt.Errorf("EOB1有效存檔開啟CAMP：%w", err)
	}
	for i := 0; i < 2; i++ {
		o.PressKey(oracle.KeyDown)
		if err := o.Run(1_000_000); err != nil {
			return fmt.Errorf("EOB1 CAMP選取Pray Spells：%w", err)
		}
	}
	o.PressKey(oracle.KeyEnter)
	if err := o.RunUntil(screenDigest("有效存檔CAMP祈禱選角", 0, 0, oracle.Width, oracle.Height,
		"421302378922e208ca941c1b2e17050debe55a77f60b33b980df042e70333c5a"), oracle.Budget(5_000_000)); err != nil {
		return fmt.Errorf("EOB1等待CAMP祈禱選角：%w", err)
	}
	// TENMIYANA是第四格；點其肖像安全點。
	if err := o.Click(272, 78, oracle.Hover(0), oracle.Hold(200_000), oracle.Settle(1_000_000)); err != nil {
		return fmt.Errorf("EOB1選取TENMIYANA祈禱：%w", err)
	}
	// 一級第五列是Protection From Evil。
	for i := 0; i < 4; i++ {
		o.PressKey(oracle.KeyDown)
		if err := o.Run(250_000); err != nil {
			return fmt.Errorf("EOB1選取Protection From Evil：%w", err)
		}
	}
	o.PressKey(oracle.KeyEnter)
	if err := o.RunUntil(screenDigest("TENMIYANA排入Protection From Evil", 0, 0, 176, 168,
		"a02ba3f50182043bc84b840f381c1a963ef46044bd1df527cb857720d61cf259"), oracle.Budget(5_000_000)); err != nil {
		return fmt.Errorf("EOB1等待Protection From Evil排入：%w", err)
	}
	return nil
}

// ToSavedGameProtectionRested 完成八小時休息，令TENMIYANA取得Protection From Evil。
func ToSavedGameProtectionRested(o *oracle.Oracle) error {
	if err := ToSavedGameProtectionPrepared(o); err != nil {
		return err
	}
	o.PressKey(oracle.KeyEscape)
	if err := o.Run(1_000_000); err != nil {
		return fmt.Errorf("EOB1由祈禱頁返回CAMP：%w", err)
	}
	for i := 0; i < 2; i++ {
		o.PressKey(oracle.KeyUp)
		if err := o.Run(500_000); err != nil {
			return fmt.Errorf("EOB1 CAMP選取Rest Party：%w", err)
		}
	}
	o.PressKey(oracle.KeyEnter)
	const completed = "7afb4aaf383b8f987338d9211fb751145248730ac1e09ce65027033077db8fbf"
	leftCamp := false
	var next uint64
	returned := oracle.NewCond("有效存檔Protection From Evil休息完成", func(o *oracle.Oracle) bool {
		if o.Steps() < next {
			return false
		}
		next = o.Steps() + 100
		indexed := o.Indexed()
		region := make([]byte, 0, 176*168)
		for row := 0; row < 168; row++ {
			region = append(region, indexed[row*oracle.Width:row*oracle.Width+176]...)
		}
		current := fmt.Sprintf("%x", sha256.Sum256(region))
		if current != completed {
			leftCamp = true
		}
		return leftCamp && current == completed
	})
	if err := o.RunUntil(returned, oracle.Budget(40_000_000)); err != nil {
		return fmt.Errorf("EOB1等待Protection From Evil休息完成：%w", err)
	}
	// 選角畫面會先於休息callback返回；讓交易收尾後才交給下一個Escape。
	if err := o.Run(2_000_000); err != nil {
		return fmt.Errorf("EOB1等待Protection From Evil休息事件收尾：%w", err)
	}
	return nil
}

// ToSavedGameProtectionBook 退出CAMP，再由TENMIYANA聖徽開啟一級牧師法術書。
func ToSavedGameProtectionBook(o *oracle.Oracle) error {
	if err := ToSavedGameProtectionRested(o); err != nil {
		return err
	}
	o.PressKey(oracle.KeyEscape)
	if err := o.RunUntil(screenDigest("Protection From Evil休息後返回CAMP根選單", 0, 0, 176, 168,
		"518da9a2266e58a2ee431128b788e8031da76c9b350251351c0117120b1e6be8"), oracle.Budget(5_000_000)); err != nil {
		return fmt.Errorf("EOB1等待休息後返回CAMP根選單：%w", err)
	}
	o.PressKey(oracle.KeyEscape)
	if err := o.RunUntil(screenDigest("Protection From Evil休息後返回地城", 0, 0, 176, 100,
		"689c6d86fd088a78820474561dc899094714b396184bd492598e4f36d066100a"), oracle.Budget(5_000_000)); err != nil {
		return fmt.Errorf("EOB1等待休息後離開CAMP：%w", err)
	}
	if err := o.Click(296, 87, oracle.RightButton(), oracle.Hover(0), oracle.Hold(200_000), oracle.Settle(1_000_000)); err != nil {
		return fmt.Errorf("EOB1由TENMIYANA聖徽開書：%w", err)
	}
	if err := o.RunUntil(screenDigest("TENMIYANA Protection From Evil法術書", 0, 0, 176, 168,
		"1f985614d63c648842aea92b3b62cbecca1b5cd5ee11aacc4cd28ae5b893cf92"), oracle.Budget(5_000_000)); err != nil {
		return fmt.Errorf("EOB1等待TENMIYANA法術書：%w", err)
	}
	return nil
}

// ToSavedGameProtectionTargeting 點Protection From Evil，進入原版角色目標選擇。
func ToSavedGameProtectionTargeting(o *oracle.Oracle) error {
	if err := ToSavedGameProtectionBook(o); err != nil {
		return err
	}
	if err := o.Click(100, 133, oracle.Hover(0), oracle.Hold(200_000), oracle.Settle(1_000_000)); err != nil {
		return fmt.Errorf("EOB1選取Protection From Evil施法：%w", err)
	}
	if err := o.RunUntil(screenDigest("Protection From Evil選擇目標", 0, 0, 176, 168,
		"f00ab327ba9c9bcb7975e5fbbc69e2b39712ee9b8b498db9c1da00eb393b7e00"), oracle.Budget(5_000_000)); err != nil {
		return fmt.Errorf("EOB1等待Protection From Evil目標：%w", err)
	}
	return nil
}

// ToSavedGameProtectionBookClosed 由有效存檔正常開啟牧師法術書，再使用原版
// ABORT SPELL 控制中止並回到探索畫面。原版目標模式不接受Escape關閉。
func ToSavedGameProtectionBookClosed(o *oracle.Oracle) error {
	if err := ToSavedGameProtectionBook(o); err != nil {
		return err
	}
	if err := o.Click(102, 172, oracle.Hover(0), oracle.Hold(200_000), oracle.Settle(2_000_000)); err != nil {
		return fmt.Errorf("EOB1點擊ABORT SPELL關閉法術書：%w", err)
	}
	if err := o.RunUntil(screenDigest("Protection From Evil法術書關閉", 0, 0, 176, 168,
		"b1dd7f80261a818d7913f6ce0cae6f04a37ba65f20e205ad0e55f7c0386f645b"), oracle.Budget(5_000_000)); err != nil {
		return fmt.Errorf("EOB1等待法術書關閉並返回探索：%w", err)
	}
	return nil
}

// ToSavedGameProtectionCastOnAriel 點ARIEL角色面板，完成Protection From Evil施法。
func ToSavedGameProtectionCastOnAriel(o *oracle.Oracle) error {
	if err := ToSavedGameProtectionTargeting(o); err != nil {
		return err
	}
	if err := o.Click(272, 26, oracle.Hover(0), oracle.Hold(200_000), oracle.Settle(2_000_000)); err != nil {
		return fmt.Errorf("EOB1對ARIEL施放Protection From Evil：%w", err)
	}
	if err := o.RunUntil(screenDigest("Protection From Evil施放於ARIEL", 0, 168, 320, 32,
		"fd53da2992f12eda5ce14e4256a3e5f08aeea342aeac3f5ef5f4eb1b5ffbb934"), oracle.Budget(5_000_000)); err != nil {
		return fmt.Errorf("EOB1等待Protection From Evil施放完成：%w", err)
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

// ToThirdCharacterRace 選取第三角色槽，進入其種族／性別頁。
func ToThirdCharacterRace(o *oracle.Oracle) error {
	if err := ToSecondCharacterBETA(o); err != nil {
		return err
	}
	if err := o.Click(49, 142); err != nil {
		return fmt.Errorf("EOB1點擊第三角色槽：%w", err)
	}
	if err := o.RunUntil(screenDigest("第三角色SELECT RACE頁", 130, 55, 180, 140,
		"b566dd781770ae6af1cd569cc84d82e37a38fe82b81405da0c979722f6e0041c"), oracle.Budget(10_000_000)); err != nil {
		return fmt.Errorf("EOB1等待第三角色種族選擇：%w", err)
	}
	return nil
}

// ToThirdCharacterReview 由第三角色種族頁走到Human Male Fighter檢視頁。
func ToThirdCharacterReview(o *oracle.Oracle) error {
	if err := ToThirdCharacterRace(o); err != nil {
		return err
	}
	steps := []struct {
		name   string
		digest string
	}{
		{"第三角色SELECT CLASS頁", "31954c4eabf3fe236734a89b17447907ac3c54448a962b4ffd0432a29e2777eb"},
		{"第三角色SELECT ALIGNMENT頁", "b701f7d8e0738aaf110f702f34a53d3ab9f0c01b60fd0f7e514bdd773a7bafbc"},
		{"第三角色屬性／肖像頁", "9e2cbd7c72e68d21971726b39df27ec139e1550373f76abc90f366f879fa8793"},
		{"第三角色檢視操作頁", "832af588ffc848efb145567ac80698b71481fee9da43b2e63892139712998948"},
	}
	for _, step := range steps {
		o.PressKey(oracle.KeyEnter)
		if err := o.RunUntil(screenDigest(step.name, 130, 55, 180, 140, step.digest), oracle.Budget(10_000_000)); err != nil {
			return fmt.Errorf("EOB1等待%s：%w", step.name, err)
		}
	}
	return nil
}

// ToThirdCharacterName 以KEEP接受第三角色，進入姓名頁。
func ToThirdCharacterName(o *oracle.Oracle) error {
	if err := ToThirdCharacterReview(o); err != nil {
		return err
	}
	if err := o.Click(282, 180); err != nil {
		return fmt.Errorf("EOB1點擊第三角色KEEP：%w", err)
	}
	if err := o.RunUntil(screenDigest("第三角色Name輸入頁", 0, 0, oracle.Width, oracle.Height,
		"95118f4657f2096ec0a4fa0ac8ddfafd0ae162c3351b2d7d82ecb5856190b06f"), oracle.Budget(10_000_000)); err != nil {
		return fmt.Errorf("EOB1等待第三角色姓名輸入：%w", err)
	}
	return nil
}

// ToThirdCharacterGAMMA 在第三角色姓名頁鍵入GAMMA並確認。
func ToThirdCharacterGAMMA(o *oracle.Oracle) error {
	if err := ToThirdCharacterName(o); err != nil {
		return err
	}
	for _, key := range []oracle.Key{oracle.KeyG, oracle.KeyA, oracle.KeyM, oracle.KeyM, oracle.KeyA} {
		o.PressKey(key)
		if err := o.Run(250_000); err != nil {
			return fmt.Errorf("EOB1輸入第三角色姓名：%w", err)
		}
	}
	o.PressKey(oracle.KeyEnter)
	if err := o.RunUntil(screenDigest("第三角色GAMMA姓名列", 5, 160, 70, 16,
		"84db489bbd3ecd5cc1050df46382eb3468083941f2fcf2584907c85af15bdef1"), oracle.Budget(10_000_000)); err != nil {
		return fmt.Errorf("EOB1等待第三角色GAMMA完成：%w", err)
	}
	return nil
}

// ToFourthCharacterRace 選取第四角色槽，進入其種族／性別頁。
func ToFourthCharacterRace(o *oracle.Oracle) error {
	if err := ToThirdCharacterGAMMA(o); err != nil {
		return err
	}
	if err := o.Click(98, 142); err != nil {
		return fmt.Errorf("EOB1點擊第四角色槽：%w", err)
	}
	if err := o.RunUntil(screenDigest("第四角色SELECT RACE頁", 130, 55, 180, 140,
		"b566dd781770ae6af1cd569cc84d82e37a38fe82b81405da0c979722f6e0041c"), oracle.Budget(10_000_000)); err != nil {
		return fmt.Errorf("EOB1等待第四角色種族選擇：%w", err)
	}
	return nil
}

// ToFourthCharacterReview 由第四角色種族頁走到Human Male Fighter檢視頁。
func ToFourthCharacterReview(o *oracle.Oracle) error {
	if err := ToFourthCharacterRace(o); err != nil {
		return err
	}
	steps := []struct {
		name   string
		digest string
	}{
		{"第四角色SELECT CLASS頁", "31954c4eabf3fe236734a89b17447907ac3c54448a962b4ffd0432a29e2777eb"},
		{"第四角色SELECT ALIGNMENT頁", "b701f7d8e0738aaf110f702f34a53d3ab9f0c01b60fd0f7e514bdd773a7bafbc"},
		{"第四角色屬性／肖像頁", "70bcf17584387de8d09f85b2f02ca06ba3d1d324b400cf37caa99153aac79461"},
		{"第四角色檢視操作頁", "43c8dc52f9f80fad0b54067648f574b297f8dd194a0b40dcd302c1abe3cc6ab9"},
	}
	for _, step := range steps {
		o.PressKey(oracle.KeyEnter)
		if err := o.RunUntil(screenDigest(step.name, 130, 55, 180, 140, step.digest), oracle.Budget(10_000_000)); err != nil {
			return fmt.Errorf("EOB1等待%s：%w", step.name, err)
		}
	}
	return nil
}

// ToFourthCharacterName 以不跨越下一畫面的短按接受第四角色，進入姓名頁。
func ToFourthCharacterName(o *oracle.Oracle) error {
	if err := ToFourthCharacterReview(o); err != nil {
		return err
	}
	if err := o.Click(282, 180, oracle.Hover(1_000_000), oracle.Hold(200_000), oracle.Settle(1_000_000)); err != nil {
		return fmt.Errorf("EOB1點擊第四角色KEEP：%w", err)
	}
	// 姓名游標會閃爍；只釘住不含游標的Name標籤與左側固定內容，
	// 避免把剛好錯過某個閃爍相位誤判成沒有轉場。
	if err := o.RunUntil(screenDigest("第四角色Name輸入頁", 130, 90, 45, 25,
		"a85053db2ad20dea6b6b6d87b061ddbb9d45aee73eba79d50fb17c5d9dc0bb88"), oracle.Budget(10_000_000)); err != nil {
		return fmt.Errorf("EOB1等待第四角色姓名輸入：%w", err)
	}
	return nil
}

// ToFourthCharacterDELTA 在第四角色姓名頁鍵入DELTA並確認，返回PLAY已啟用的總覽。
func ToFourthCharacterDELTA(o *oracle.Oracle) error {
	if err := ToFourthCharacterName(o); err != nil {
		return err
	}
	for _, key := range []oracle.Key{oracle.KeyD, oracle.KeyE, oracle.KeyL, oracle.KeyT, oracle.KeyA} {
		o.PressKey(key)
		if err := o.Run(250_000); err != nil {
			return fmt.Errorf("EOB1輸入第四角色姓名：%w", err)
		}
	}
	o.PressKey(oracle.KeyEnter)
	if err := o.RunUntil(screenDigest("第四角色DELTA姓名列", 75, 160, 70, 16,
		"5565425afaedff49bad39cd17eac469ec74ee8459698501d1623834ef15c2584"), oracle.Budget(10_000_000)); err != nil {
		return fmt.Errorf("EOB1等待第四角色DELTA完成：%w", err)
	}
	return nil
}

// ToLevel1Entrance 由完成的四人隊伍正常按PLAY，等待第一格地城完成繪製。
func ToLevel1Entrance(o *oracle.Oracle) error {
	if err := ToFourthCharacterDELTA(o); err != nil {
		return err
	}
	// DELTA姓名列會先於觸發它的鍵盤callback返回完成；先讓前一事件收尾，
	// 避免新的滑鼠callback巢狀進入而破壞原版segment context。
	if err := o.Run(1_000_000); err != nil {
		return fmt.Errorf("EOB1等待建角輸入事件收尾：%w", err)
	}
	if err := o.Click(65, 190, oracle.Hover(1_000_000), oracle.Hold(200_000), oracle.Settle(1_000_000)); err != nil {
		return fmt.Errorf("EOB1點擊PLAY：%w", err)
	}
	for i := 0; i < 20_000; i++ {
		if err := o.Run(1_000); err != nil {
			return fmt.Errorf("EOB1執行LEVEL1初始化批次%d：%w", i, err)
		}
	}
	if err := o.RunUntil(screenDigest("LEVEL1第一格視窗", 0, 0, 176, 120,
		"2ef2c0240070bce02b59735c5266fc6163eee170ea8c135982a469f04bb2abbc"), oracle.Budget(30_000_000)); err != nil {
		return fmt.Errorf("EOB1等待LEVEL1第一格：%w", err)
	}
	return nil
}

// ToLevel1FirstInventory 從LEVEL1入口點擊第一名角色肖像，開啟ALFA的原版物品欄。
func ToLevel1FirstInventory(o *oracle.Oracle) error {
	if err := ToLevel1Entrance(o); err != nil {
		return err
	}
	// 原版DOS buttonDefs第9項：第一名角色肖像是(184,10) 33×33；使用內部安全點。
	if err := o.Click(200, 26, oracle.Hover(0), oracle.Hold(200_000), oracle.Settle(1_000_000)); err != nil {
		return fmt.Errorf("EOB1開啟ALFA物品欄：%w", err)
	}
	if err := o.MoveMouse(300, 190); err != nil {
		return err
	}
	if err := o.RunUntil(screenDigest("LEVEL1 ALFA物品欄", 0, 0, oracle.Width, oracle.Height,
		"2d77f8c80423d94e0d28ba9a562a6df86abbe0a587f10ef46e4d19e1b2e95809"), oracle.Budget(5_000_000)); err != nil {
		return fmt.Errorf("EOB1等待ALFA物品欄：%w", err)
	}
	return nil
}

// ToLevel1InventoryNextCharacter 從ALFA物品欄點擊右箭頭，切到BETA。
func ToLevel1InventoryNextCharacter(o *oracle.Oracle) error {
	if err := ToLevel1FirstInventory(o); err != nil {
		return err
	}
	// 原版DOS buttonDefs第60項：(297,35) 20×15。
	if err := o.Click(307, 42, oracle.Hover(0), oracle.Hold(200_000), oracle.Settle(1_000_000)); err != nil {
		return fmt.Errorf("EOB1物品欄切到下一位角色：%w", err)
	}
	if err := o.MoveMouse(300, 190); err != nil {
		return err
	}
	if err := o.RunUntil(screenDigest("LEVEL1 BETA物品欄", 0, 0, oracle.Width, oracle.Height,
		"21d28f3144af98f890f13139469364a9334b78443858d709bba3c81c231350df"), oracle.Budget(5_000_000)); err != nil {
		return fmt.Errorf("EOB1等待BETA物品欄：%w", err)
	}
	return nil
}

// ToLevel1InventoryPreviousCharacter 從BETA物品欄點擊左箭頭，切回ALFA。
func ToLevel1InventoryPreviousCharacter(o *oracle.Oracle) error {
	if err := ToLevel1InventoryNextCharacter(o); err != nil {
		return err
	}
	// 原版DOS buttonDefs第59項：(274,35) 20×15。
	if err := o.Click(284, 42, oracle.Hover(0), oracle.Hold(200_000), oracle.Settle(1_000_000)); err != nil {
		return fmt.Errorf("EOB1物品欄切到上一位角色：%w", err)
	}
	if err := o.MoveMouse(300, 190); err != nil {
		return err
	}
	if err := o.RunUntil(screenDigest("LEVEL1物品欄切回ALFA", 0, 0, oracle.Width, oracle.Height,
		"2d77f8c80423d94e0d28ba9a562a6df86abbe0a587f10ef46e4d19e1b2e95809"), oracle.Budget(5_000_000)); err != nil {
		return fmt.Errorf("EOB1等待物品欄切回ALFA：%w", err)
	}
	return nil
}

// ToLevel1InventoryReturn 再點ALFA肖像，關閉物品欄並返回LEVEL1入口探索面板。
func ToLevel1InventoryReturn(o *oracle.Oracle) error {
	if err := ToLevel1InventoryPreviousCharacter(o); err != nil {
		return err
	}
	if err := o.Click(200, 26, oracle.Hover(0), oracle.Hold(200_000), oracle.Settle(1_000_000)); err != nil {
		return fmt.Errorf("EOB1關閉ALFA物品欄：%w", err)
	}
	if err := o.MoveMouse(300, 190); err != nil {
		return err
	}
	if err := o.RunUntil(screenDigest("LEVEL1由物品欄返回探索", 0, 0, oracle.Width, oracle.Height,
		"b4fd5e6d1a6b2740cc9f390f50d28c65649a4a7c99832e7f27766340a0ebcf26"), oracle.Budget(5_000_000)); err != nil {
		return fmt.Errorf("EOB1等待由物品欄返回探索：%w", err)
	}
	return nil
}

// ToLevel1CharacterExchangeSelected 從LEVEL1入口以右鍵點ALFA姓名列，等待交換標記相位。
func ToLevel1CharacterExchangeSelected(o *oracle.Oracle) error {
	if err := ToLevel1Entrance(o); err != nil {
		return err
	}
	// 原版DOS buttonDefs第17項：第一名角色姓名列是(184,2) 63×8。
	if err := o.Click(200, 6, oracle.RightButton(), oracle.Hover(0), oracle.Hold(200_000), oracle.Settle(200_000)); err != nil {
		return fmt.Errorf("EOB1選取ALFA進入角色交換：%w", err)
	}
	if err := o.MoveMouse(300, 190); err != nil {
		return err
	}
	if err := o.RunUntil(screenDigest("LEVEL1 ALFA角色交換標記", 0, 0, oracle.Width, oracle.Height,
		"7106257fa9aafd131af18e4623ddb4a094f922f32fcdee95a3d0c887ac6752df"), oracle.Budget(5_000_000)); err != nil {
		return fmt.Errorf("EOB1等待ALFA角色交換標記：%w", err)
	}
	return nil
}

// ToLevel1CharacterExchangeCancel 在交換標記相位點面板非姓名區，取消並恢復原隊伍畫面。
func ToLevel1CharacterExchangeCancel(o *oracle.Oracle) error {
	if err := ToLevel1CharacterExchangeSelected(o); err != nil {
		return err
	}
	// 原版button 55涵蓋(184,0) 136×120；此點避開六個姓名列。
	if err := o.Click(230, 45, oracle.Hover(0), oracle.Hold(200_000), oracle.Settle(1_000_000)); err != nil {
		return fmt.Errorf("EOB1取消角色交換：%w", err)
	}
	if err := o.MoveMouse(300, 190); err != nil {
		return err
	}
	if err := o.RunUntil(screenDigest("LEVEL1取消角色交換", 0, 0, oracle.Width, oracle.Height,
		"3064a9c9eea5e6f1be2f18915868cd076cce96778e0e25bf9fb0e12995bd325f"), oracle.Budget(5_000_000)); err != nil {
		return fmt.Errorf("EOB1等待角色交換取消：%w", err)
	}
	return nil
}

// ToLevel1FirstForwardStep 從LEVEL1入口以原版鍵盤右轉，再向前走入相鄰格。
func ToLevel1FirstForwardStep(o *oracle.Oracle) error {
	if err := ToLevel1Entrance(o); err != nil {
		return err
	}
	o.PressKey(oracle.KeyPageUp)
	if err := o.Run(2_000_000); err != nil {
		return fmt.Errorf("EOB1 LEVEL1右轉：%w", err)
	}
	if err := o.RunUntil(screenDigest("LEVEL1入口右轉", 0, 0, 176, 120,
		"a35855823bdecf7b0a150b2ca9d35e650234a644871c24a97ebb8d279cb45270"), oracle.Budget(5_000_000)); err != nil {
		return fmt.Errorf("EOB1等待LEVEL1右轉畫面：%w", err)
	}
	o.PressKey(oracle.KeyUp)
	if err := o.Run(2_000_000); err != nil {
		return fmt.Errorf("EOB1 LEVEL1向前：%w", err)
	}
	if err := o.RunUntil(screenDigest("LEVEL1第一步", 0, 0, 176, 120,
		"7fc8b674d93e02578a2dfe79e54232899c4870bb2122e1b89eaf040477e72f08"), oracle.Budget(5_000_000)); err != nil {
		return fmt.Errorf("EOB1等待LEVEL1第一步畫面：%w", err)
	}
	return nil
}

// ToLevel1FirstPickup 從LEVEL1入口點擊原版左近場景區，拾起起始地面的石塊。
func ToLevel1FirstPickup(o *oracle.Oracle) error {
	if err := ToLevel1Entrance(o); err != nil {
		return err
	}
	// 原版DOS buttonDefs：current-left是(0,102) 88×18；使用中心安全點。
	if err := o.Click(44, 110, oracle.Hover(0), oracle.Hold(200_000), oracle.Settle(1_000_000)); err != nil {
		return fmt.Errorf("EOB1拾取LEVEL1起始石塊：%w", err)
	}
	if err := o.RunUntil(screenDigest("LEVEL1起始石塊已拾取", 0, 0, 176, 120,
		"1d123d4fe9a5a1001446f2d180601e8713fa2dd02f00caac4c9f406f31b59375"), oracle.Budget(5_000_000)); err != nil {
		return fmt.Errorf("EOB1等待起始石塊拾取畫面：%w", err)
	}
	return nil
}

// ToLevel1FirstDrop 將首次拾起的石塊放回原版左近場景區，再把游標移出視窗供觀測。
func ToLevel1FirstDrop(o *oracle.Oracle) error {
	if err := ToLevel1FirstPickup(o); err != nil {
		return err
	}
	if err := o.Click(44, 110, oracle.Hover(0), oracle.Hold(200_000), oracle.Settle(1_000_000)); err != nil {
		return fmt.Errorf("EOB1放回LEVEL1起始石塊：%w", err)
	}
	if err := o.Run(2_000_000); err != nil {
		return fmt.Errorf("EOB1等待起始石塊放回：%w", err)
	}
	if err := o.MoveMouse(300, 190); err != nil {
		return err
	}
	if err := o.RunUntil(screenDigest("LEVEL1起始石塊已放回", 0, 0, 176, 120,
		"4cf790c278d6a9115094c61eebacf894d13ad12a118403ed25056c42523352d2"), oracle.Budget(5_000_000)); err != nil {
		return fmt.Errorf("EOB1等待起始石塊放回畫面：%w", err)
	}
	return nil
}

// ToLevel1Camp 從LEVEL1入口點擊原版CAMP按鈕，等待營地根選單穩定。
func ToLevel1Camp(o *oracle.Oracle) error {
	if err := ToLevel1Entrance(o); err != nil {
		return err
	}
	// 原版DOS buttonDefs：CAMP是(289,177) 31×21；使用中心安全點。
	if err := o.Click(304, 187, oracle.Hover(0), oracle.Hold(200_000), oracle.Settle(1_000_000)); err != nil {
		return fmt.Errorf("EOB1點擊LEVEL1 CAMP：%w", err)
	}
	if err := o.RunUntil(screenDigest("LEVEL1 CAMP根選單", 0, 0, oracle.Width, oracle.Height,
		"79e080c1a738e5ecd7d26ff379d7a126cb9ba4cee7ea998af94555b7a3d4f8ae"), oracle.Budget(5_000_000)); err != nil {
		return fmt.Errorf("EOB1等待CAMP根選單：%w", err)
	}
	return nil
}

// ToLevel1CampMemorizeSelected 從CAMP根選單向下選到Memorize Spells。
func ToLevel1CampMemorizeSelected(o *oracle.Oracle) error {
	if err := ToLevel1Camp(o); err != nil {
		return err
	}
	o.PressKey(oracle.KeyDown)
	if err := o.RunUntil(screenDigest("LEVEL1 CAMP選中Memorize Spells", 0, 0, oracle.Width, oracle.Height,
		"8bd3e9866787810347ed38d1929b8dbb1a12683359037056f5fc369df3b2962c"), oracle.Budget(5_000_000)); err != nil {
		return fmt.Errorf("EOB1等待CAMP選中Memorize Spells：%w", err)
	}
	return nil
}

// ToLevel1CampReturn 從第二列按Escape返回原本LEVEL1入口場景。
func ToLevel1CampReturn(o *oracle.Oracle) error {
	if err := ToLevel1CampMemorizeSelected(o); err != nil {
		return err
	}
	o.PressKey(oracle.KeyEscape)
	if err := o.RunUntil(screenDigest("LEVEL1 CAMP返回地城", 0, 0, 176, 120,
		"2ef2c0240070bce02b59735c5266fc6163eee170ea8c135982a469f04bb2abbc"), oracle.Budget(5_000_000)); err != nil {
		return fmt.Errorf("EOB1等待CAMP返回LEVEL1：%w", err)
	}
	return nil
}

// ToLevel1CampExitSelected 由CAMP第一列按Up，證實選取會環繞到最後一列Exit。
func ToLevel1CampExitSelected(o *oracle.Oracle) error {
	if err := ToLevel1Camp(o); err != nil {
		return err
	}
	o.PressKey(oracle.KeyUp)
	if err := o.RunUntil(screenDigest("LEVEL1 CAMP選中Exit", 0, 0, oracle.Width, oracle.Height,
		"36a8f388e9a480cdf041124127c2e4d55cec59859aaafed32ed15807a6f0565e"), oracle.Budget(5_000_000)); err != nil {
		return fmt.Errorf("EOB1等待CAMP選中Exit：%w", err)
	}
	return nil
}

// ToLevel1CampExitConfirmed 確認最後一列Exit，返回原本LEVEL1入口場景。
func ToLevel1CampExitConfirmed(o *oracle.Oracle) error {
	if err := ToLevel1CampExitSelected(o); err != nil {
		return err
	}
	o.PressKey(oracle.KeyEnter)
	if err := o.RunUntil(screenDigest("LEVEL1 CAMP Exit返回探索", 0, 0, 176, 120,
		"2ef2c0240070bce02b59735c5266fc6163eee170ea8c135982a469f04bb2abbc"), oracle.Budget(5_000_000)); err != nil {
		return fmt.Errorf("EOB1等待CAMP Exit返回LEVEL1：%w", err)
	}
	return nil
}

// ToLevel1CampGameOptions 從正常LEVEL1 CAMP根選單，以鍵盤選取Game Options。
// 原版畫面文字列與滑鼠命中區在此狀態並不一致，故不猜測滑鼠列座標。
func ToLevel1CampGameOptions(o *oracle.Oracle) error {
	if err := ToLevel1Camp(o); err != nil {
		return err
	}
	for i := 0; i < 5; i++ {
		o.PressKey(oracle.KeyDown)
		if err := o.Run(300_000); err != nil {
			return fmt.Errorf("EOB1 CAMP選取Game Options：%w", err)
		}
	}
	o.PressKey(oracle.KeyEnter)
	if err := o.RunUntil(screenDigest("LEVEL1 CAMP Game Options", 0, 0, oracle.Width, oracle.Height,
		"2207602e9d4e144c6fadea7d30cd4a189b9c09cb5fd0269d2ed8ed6f0a86d912"), oracle.Budget(5_000_000)); err != nil {
		return fmt.Errorf("EOB1等待CAMP Game Options：%w", err)
	}
	return nil
}

// ToLevel1CampGameOptionsExit 由Game Options按Escape回CAMP，再按Escape回探索。
func ToLevel1CampGameOptionsExit(o *oracle.Oracle) error {
	if err := ToLevel1CampGameOptions(o); err != nil {
		return err
	}
	o.PressKey(oracle.KeyEscape)
	if err := o.RunUntil(screenDigest("LEVEL1 Game Options返回CAMP", 0, 0, oracle.Width, oracle.Height,
		"79e080c1a738e5ecd7d26ff379d7a126cb9ba4cee7ea998af94555b7a3d4f8ae"), oracle.Budget(5_000_000)); err != nil {
		return fmt.Errorf("EOB1等待Game Options返回CAMP：%w", err)
	}
	o.PressKey(oracle.KeyEscape)
	if err := o.RunUntil(screenDigest("LEVEL1 Game Options返回探索", 0, 0, 176, 120,
		"2ef2c0240070bce02b59735c5266fc6163eee170ea8c135982a469f04bb2abbc"), oracle.Budget(5_000_000)); err != nil {
		return fmt.Errorf("EOB1等待Game Options返回LEVEL1：%w", err)
	}
	return nil
}

// ToLevel1CampSaveConfirmation 從Game Options選取Save Game，停在原版確認視窗。
// 此路徑不確認寫檔，僅供觀察原版儲存交易的入口。
func ToLevel1CampSaveConfirmation(o *oracle.Oracle) error {
	if err := ToLevel1CampGameOptions(o); err != nil {
		return err
	}
	o.PressKey(oracle.KeyDown)
	if err := o.Run(300_000); err != nil {
		return fmt.Errorf("EOB1 Game Options選取Save Game：%w", err)
	}
	o.PressKey(oracle.KeyEnter)
	if err := o.RunUntil(screenDigest("LEVEL1 Save Game確認", 0, 0, oracle.Width, oracle.Height,
		"40002452b7022088be0995c5dcc3a98c394786e3481774db36ab50c2c978e768"), oracle.Budget(5_000_000)); err != nil {
		return fmt.Errorf("EOB1等待Save Game確認：%w", err)
	}
	return nil
}

// ToLevel1CampSaveCancel 在原版Save Game確認視窗點No，返回Game Options。
func ToLevel1CampSaveCancel(o *oracle.Oracle) error {
	if err := ToLevel1CampSaveConfirmation(o); err != nil {
		return err
	}
	if err := o.Click(145, 64, oracle.Hover(0), oracle.Hold(200_000), oracle.Settle(2_000_000)); err != nil {
		return fmt.Errorf("EOB1取消Save Game：%w", err)
	}
	if err := o.MoveMouse(300, 190); err != nil {
		return err
	}
	if err := o.Run(1_000_000); err != nil {
		return fmt.Errorf("EOB1等待Save Game取消返回Game Options：%w", err)
	}
	return nil
}

// ToLevel1CampSaveWritten 以確認視窗預設的Yes完成原版單檔存檔。
// 呼叫端必須先對可丟棄覆蓋層逐檔啟用EOBDATA.SAV與LEVELS.TMP寫入。
func ToLevel1CampSaveWritten(o *oracle.Oracle) error {
	if err := ToLevel1CampSaveConfirmation(o); err != nil {
		return err
	}
	o.PressKey(oracle.KeyEnter)
	if err := o.RunUntil(screenDigest("LEVEL1 Save Game完成", 0, 0, 176, 96,
		"a2ca1d151d25aa8df20113ae78f130df05a1daed3c102173bbb02f557ee13d0c"), oracle.Budget(5_000_000)); err != nil {
		return fmt.Errorf("EOB1等待Save Game完成返回Game Options：%w", err)
	}
	return nil
}

// ToLevel1MemorizeSpells 從CAMP第二列確認，進入ALFA的一級法術記憶頁。
func ToLevel1MemorizeSpells(o *oracle.Oracle) error {
	if err := ToLevel1CampMemorizeSelected(o); err != nil {
		return err
	}
	o.PressKey(oracle.KeyEnter)
	if err := o.RunUntil(screenDigest("LEVEL1 CAMP記憶法術頁", 0, 0, oracle.Width, oracle.Height,
		"1c430348d1f4d21309d1b553fb3f21597baa97c6682563c9d599ad9b9bf6c00a"), oracle.Budget(5_000_000)); err != nil {
		return fmt.Errorf("EOB1等待CAMP記憶法術頁：%w", err)
	}
	return nil
}

// ToLevel1MemorizeFirstSpell 在ALFA的一級法術頁確認第一項，使剩餘槽位由二減為一。
func ToLevel1MemorizeFirstSpell(o *oracle.Oracle) error {
	if err := ToLevel1MemorizeSpells(o); err != nil {
		return err
	}
	o.PressKey(oracle.KeyEnter)
	if err := o.RunUntil(screenDigest("LEVEL1 CAMP已選第一個待記憶法術", 0, 0, oracle.Width, oracle.Height,
		"2d78cd6c8a1886cb8d06b14a708d70ffc6fd9529422c0ade2eff36b2df386da8"), oracle.Budget(5_000_000)); err != nil {
		return fmt.Errorf("EOB1等待第一個待記憶法術：%w", err)
	}
	return nil
}

// ToLevel1MemorizeReturn 從已選法術頁按Escape返回CAMP，保留Memorize Spells列選中。
func ToLevel1MemorizeReturn(o *oracle.Oracle) error {
	if err := ToLevel1MemorizeFirstSpell(o); err != nil {
		return err
	}
	o.PressKey(oracle.KeyEscape)
	if err := o.RunUntil(screenDigest("LEVEL1 CAMP由記憶法術返回", 0, 0, oracle.Width, oracle.Height,
		"8bd3e9866787810347ed38d1929b8dbb1a12683359037056f5fc369df3b2962c"), oracle.Budget(5_000_000)); err != nil {
		return fmt.Errorf("EOB1等待由記憶法術返回CAMP：%w", err)
	}
	return nil
}

// ToLevel1RestAfterMemorize 先替ALFA選一個待記憶法術，再由CAMP第一列完成一次正常休息。
// 健康新隊伍的休息很短，因此用「確實離開、再回到同一CAMP畫面」驗證交易，不以固定等待冒充。
func ToLevel1RestAfterMemorize(o *oracle.Oracle) error {
	if err := ToLevel1MemorizeReturn(o); err != nil {
		return err
	}
	o.PressKey(oracle.KeyUp)
	const restSelected = "79e080c1a738e5ecd7d26ff379d7a126cb9ba4cee7ea998af94555b7a3d4f8ae"
	if err := o.RunUntil(screenDigest("LEVEL1 CAMP選中Rest Party", 0, 0, oracle.Width, oracle.Height,
		restSelected), oracle.Budget(5_000_000)); err != nil {
		return fmt.Errorf("EOB1等待CAMP選中Rest Party：%w", err)
	}
	o.PressKey(oracle.KeyEnter)
	leftCamp := false
	var next uint64
	returned := oracle.NewCond("LEVEL1正常休息離開畫面後返回CAMP", func(o *oracle.Oracle) bool {
		if o.Steps() < next {
			return false
		}
		next = o.Steps() + 100
		current := fmt.Sprintf("%x", sha256.Sum256(o.Indexed()))
		if current != restSelected {
			leftCamp = true
		}
		return leftCamp && current == restSelected
	})
	if err := o.RunUntil(returned, oracle.Budget(5_000_000)); err != nil {
		return fmt.Errorf("EOB1等待正常休息完成：%w", err)
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
