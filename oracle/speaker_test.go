package oracle

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/wicanr2/dosgolem/internal/machine"
)

// TestSpeakerWAVHeaderAndLength 釘住 WAV 的檔頭與長度。
//
// **格式錯了播放器只會說「無法開啟」**，看起來像沒有聲音而不像檔頭寫錯，
// 所以要在這裡檢查每一個欄位。
func TestSpeakerWAVHeaderAndLength(t *testing.T) {
	o := newBareOracle(t)
	// 一秒鐘的方波：每 0.1 秒切一次。
	//
	// **第一筆要是高位準**：低位準是埠 0x61 的重置值，記錄端會把它
	// 當成「還沒動過」跳掉，波形就少了開頭那 0.1 秒。
	sps := machine.StepsPerSecond()
	for i := 0; i <= 10; i++ {
		o.m.Steps = uint64(float64(i) * sps / 10)
		o.m.Out8(0x61, uint8((i+1)%2)<<1)
	}
	var buf bytes.Buffer
	if err := o.SpeakerWAV(&buf, 11025); err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()
	if len(b) < 44 {
		t.Fatalf("只有 %d 個位元組，連檔頭都不夠", len(b))
	}
	if string(b[0:4]) != "RIFF" || string(b[8:12]) != "WAVE" {
		t.Fatalf("檔頭不是 RIFF/WAVE：%q", b[0:12])
	}
	if got := binary.LittleEndian.Uint16(b[22:]); got != 1 {
		t.Errorf("聲道數是 %d", got)
	}
	if got := binary.LittleEndian.Uint32(b[24:]); got != 11025 {
		t.Errorf("取樣率是 %d", got)
	}
	if got := binary.LittleEndian.Uint16(b[34:]); got != 8 {
		t.Errorf("位元深度是 %d", got)
	}
	data := binary.LittleEndian.Uint32(b[40:])
	if int(data) != len(b)-44 {
		t.Errorf("data 區塊寫 %d，實際有 %d", data, len(b)-44)
	}
	// 一秒的訊號 ±5%。
	if data < 10_000 || data > 11_600 {
		t.Errorf("一秒的訊號算出 %d 個取樣，應該在 11,025 附近", data)
	}
	// 兩個位準都要出現，否則等於沒解出方波。
	var lo, hi int
	for _, v := range b[44:] {
		switch v {
		case 0x20:
			lo++
		case 0xE0:
			hi++
		default:
			t.Fatalf("出現預期外的取樣值 %02X", v)
		}
	}
	if lo == 0 || hi == 0 {
		t.Errorf("只有一種位準（低 %d 高 %d）——方波沒解出來", lo, hi)
	}
}

// TestSpeakerWAVSaysNothingWhenSilent 是負對照。
//
// **「沒有聲音」要是錯誤不是空檔**：寫出一個 0 秒的 WAV 之後，
// 「功能沒接上」與「這一段本來就沒出聲」長得一模一樣。
func TestSpeakerWAVSaysNothingWhenSilent(t *testing.T) {
	o := newBareOracle(t)
	var buf bytes.Buffer
	if err := o.SpeakerWAV(&buf, 0); err == nil {
		t.Fatal("沒出聲卻沒有回錯誤")
	}
}

func newBareOracle(t *testing.T) *Oracle {
	t.Helper()
	m := machine.New()
	return &Oracle{m: m, onCall: map[uint32][]func(*Oracle){}}
}
