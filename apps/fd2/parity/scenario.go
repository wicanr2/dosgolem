package parity

import (
	"encoding/json"
	"fmt"
	"os"
)

type Scenario struct {
	Schema          int      `json:"schema"`
	Game            string   `json:"game"`
	Name            string   `json:"scenario"`
	State           string   `json:"state"`
	Cycles          string   `json:"cycles"`
	Timeline        string   `json:"timeline"`
	ExpectedCapture string   `json:"expected_capture"`
	Regions         []Region `json:"regions,omitempty"`
}

type Region struct {
	Name   string `json:"name"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

func LoadScenario(path string) (Scenario, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Scenario{}, err
	}
	var s Scenario
	if err := json.Unmarshal(b, &s); err != nil {
		return Scenario{}, err
	}
	if err := s.Validate(); err != nil {
		return Scenario{}, fmt.Errorf("%s：%w", path, err)
	}
	return s, nil
}

func (s Scenario) Validate() error {
	if s.Schema != 1 {
		return fmt.Errorf("不支援的場景 schema：%d", s.Schema)
	}
	if s.Game != "fd2" || s.Name == "" || s.Cycles == "" || s.Timeline == "" || s.ExpectedCapture == "" {
		return fmt.Errorf("場景必要欄位不完整")
	}
	if s.State != "same-state" && s.State != "near-state" && s.State != "layout-only" {
		return fmt.Errorf("不支援的狀態等級：%q", s.State)
	}
	seen := make(map[string]bool, len(s.Regions))
	for _, region := range s.Regions {
		if region.Name == "" || seen[region.Name] {
			return fmt.Errorf("區域名稱空白或重複：%q", region.Name)
		}
		seen[region.Name] = true
		if region.X < 0 || region.Y < 0 || region.Width <= 0 || region.Height <= 0 ||
			region.X+region.Width > Width || region.Y+region.Height > Height {
			return fmt.Errorf("區域 %q 超出 %dx%d 畫布", region.Name, Width, Height)
		}
	}
	return nil
}
