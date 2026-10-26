package player

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/wicanr2/dosgolem/session"
	"io"
)

type Appearance struct {
	Scale         int
	Language      string
	HD, Subtitles bool
	Version       uint64
}
type RecordingHeader struct {
	Format                                     string
	Version                                    int
	StepsPerSecond                             uint64
	StateSHA256, EXESHA256, TreeManifestSHA256 string
}
type RecordedTurn struct {
	N          uint64
	Input      session.CapturedUpdate
	Appearance Appearance
}
type RecordingWriter struct{ e *json.Encoder }

func NewRecording(w io.Writer, h RecordingHeader) (*RecordingWriter, error) {
	if h.Format != "strikecommander-player-turns" || h.Version != 1 || h.StepsPerSecond != StepsPerSecond {
		return nil, errors.New("錄製header格式錯誤")
	}
	e := json.NewEncoder(w)
	if err := e.Encode(h); err != nil {
		return nil, err
	}
	return &RecordingWriter{e}, nil
}
func validTurn(t RecordedTurn) error {
	if t.N == 0 {
		return errors.New("錄製回合不得0步")
	}
	if len(t.Input.BIOSKeys) != 0 {
		return errors.New("016錄製不得有BIOSKeys")
	}
	if t.Appearance.Scale != 2 && t.Appearance.Scale != 3 {
		return errors.New("錄製倍率不符")
	}
	if t.Appearance.Language != "zh-TW" && t.Appearance.Language != "en" {
		return errors.New("錄製語言不符")
	}
	for _, s := range t.Input.ScanEvents {
		if s.Scan == 0 || s.Scan > 127 || s.Scan == 0x57 || s.Scan == 0x58 {
			return fmt.Errorf("錄製scan不合法：%d", s.Scan)
		}
	}
	return nil
}
func (w *RecordingWriter) Append(t RecordedTurn) error {
	if e := validTurn(t); e != nil {
		return e
	}
	return w.e.Encode(t)
}
func ReplayRecording(r io.Reader, run func(RecordedTurn) error) (RecordingHeader, error) {
	return ReplayRecordingChecked(r, nil, run)
}
func ReplayRecordingChecked(r io.Reader, check func(RecordingHeader) error, run func(RecordedTurn) error) (RecordingHeader, error) {
	d := json.NewDecoder(r)
	d.DisallowUnknownFields()
	var h RecordingHeader
	if e := d.Decode(&h); e != nil {
		return h, e
	}
	if h.Format != "strikecommander-player-turns" || h.Version != 1 || h.StepsPerSecond != StepsPerSecond {
		return h, errors.New("錄製header格式錯誤")
	}
	if check != nil {
		if e := check(h); e != nil {
			return h, e
		}
	}
	for {
		var t RecordedTurn
		e := d.Decode(&t)
		if e == io.EOF {
			return h, nil
		}
		if e != nil {
			return h, e
		}
		if e = validTurn(t); e != nil {
			return h, e
		}
		if e = run(t); e != nil {
			return h, e
		}
	}
}
