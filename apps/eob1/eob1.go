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
	if err := o.RunUntil(screenDigest("第四角色Name輸入頁", 0, 0, oracle.Width, oracle.Height,
		"868b6ff3dec8ba456efd3ccb71b5d26188deef7e608af9f5313cfc2696b2085a"), oracle.Budget(10_000_000)); err != nil {
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
	o.MoveMouse(300, 190)
	if err := o.RunUntil(screenDigest("LEVEL1起始石塊已放回", 0, 0, 176, 120,
		"4cf790c278d6a9115094c61eebacf894d13ad12a118403ed25056c42523352d2"), oracle.Budget(5_000_000)); err != nil {
		return fmt.Errorf("EOB1等待起始石塊放回畫面：%w", err)
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
