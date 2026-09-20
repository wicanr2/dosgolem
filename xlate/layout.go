package xlate

import "errors"

// ErrTooLong 表示譯文放不下，多的字被截掉（spec 202 §2.2）。
var ErrTooLong = errors.New("xlate: 譯文過長")

// Layout 把譯文排進各行的格數：一個字元一格，依序填滿各行；`\n` 強制換行。
// widths 是每一行的格數上限。放不下時回 ErrTooLong，並附上截掉後的結果。
func Layout(text string, widths []int) ([][]rune, error) {
	out := make([][]rune, len(widths))
	line := 0
	var err error
	for _, r := range text {
		if line >= len(widths) {
			err = ErrTooLong
			break
		}
		if r == '\n' {
			line++
			continue
		}
		if len(out[line]) == widths[line] {
			line++
			if line >= len(widths) {
				err = ErrTooLong
				break
			}
		}
		out[line] = append(out[line], r)
	}
	return out, err
}
