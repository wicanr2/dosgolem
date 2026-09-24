package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// receiptOutputs holds every file until the replay and all display checks have
// passed. An in-process commit error restores previous files before reporting
// failure. Multiple paths cannot be crash- or power-loss-atomic.
// The rename operation is replaceable only to exercise the rollback path.
type receiptOutputs struct {
	files  []receiptOutput
	rename func(string, string) error
}

type receiptOutput struct {
	path  string
	data  []byte
	write func(string) error
}

func (o *receiptOutputs) add(path string, data []byte) {
	if path != "" {
		o.files = append(o.files, receiptOutput{path: path, data: data})
	}
}

func (o *receiptOutputs) addGenerated(path string, write func(string) error) {
	if path != "" {
		o.files = append(o.files, receiptOutput{path: path, write: write})
	}
}

func (o *receiptOutputs) commitReceipt(stdout io.Writer, receiptPath string, value any) error {
	b, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("編碼 JSON 收據：%w", err)
	}
	b = append(b, '\n')
	o.add(receiptPath, b)
	return o.commit(stdout, b)
}

type stagedReceiptOutput struct {
	path, stage, backup string
	existed             bool
	published           bool
}

func (o *receiptOutputs) commit(stdout io.Writer, receipt []byte) error {
	rename := o.rename
	if rename == nil {
		rename = os.Rename
	}
	staged := make([]stagedReceiptOutput, 0, len(o.files))
	defer func() {
		for _, item := range staged {
			if item.stage != "" {
				_ = os.Remove(item.stage)
			}
		}
	}()
	seen := make(map[string]bool, len(o.files))
	for _, output := range o.files {
		path, absErr := filepath.Abs(output.path)
		if absErr != nil {
			return absErr
		}
		path = filepath.Clean(path)
		if seen[path] {
			return fmt.Errorf("重複的收據輸出路徑：%s", path)
		}
		seen[path] = true
		parent := filepath.Dir(path)
		if info, statErr := os.Stat(parent); statErr != nil || !info.IsDir() {
			return fmt.Errorf("收據輸出目錄無效：%s", parent)
		}
		item := stagedReceiptOutput{path: path}
		info, statErr := os.Lstat(path)
		if statErr == nil {
			if !info.Mode().IsRegular() {
				return fmt.Errorf("收據輸出不是一般檔案：%s", path)
			}
			item.existed = true
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return fmt.Errorf("檢查收據輸出：%w", statErr)
		}
		file, createErr := os.CreateTemp(parent, ".buckrogers-receipt-stage-*")
		if createErr != nil {
			return fmt.Errorf("建立收據暫存檔：%w", createErr)
		}
		item.stage = file.Name()
		staged = append(staged, item)
		if closeErr := file.Close(); closeErr != nil {
			return fmt.Errorf("關閉收據暫存檔：%w", closeErr)
		}
		if output.write != nil {
			if writeErr := output.write(item.stage); writeErr != nil {
				return fmt.Errorf("準備收據輸出 %s：%w", path, writeErr)
			}
		} else if writeErr := os.WriteFile(item.stage, output.data, 0o644); writeErr != nil {
			return fmt.Errorf("準備收據輸出 %s：%w", path, writeErr)
		}
		if item.existed {
			if chmodErr := os.Chmod(item.stage, info.Mode().Perm()); chmodErr != nil {
				return fmt.Errorf("設定收據暫存檔權限：%w", chmodErr)
			}
		} else if chmodErr := os.Chmod(item.stage, 0o644); chmodErr != nil {
			return fmt.Errorf("設定收據暫存檔權限：%w", chmodErr)
		}
		if syncErr := syncReceiptFile(item.stage); syncErr != nil {
			return fmt.Errorf("同步收據暫存檔：%w", syncErr)
		}
	}
	for i := range staged {
		item := &staged[i]
		if item.existed {
			file, createErr := os.CreateTemp(filepath.Dir(item.path), ".buckrogers-receipt-backup-*")
			if createErr != nil {
				return rollbackReceiptOutputs(staged, fmt.Errorf("建立收據備份：%w", createErr))
			}
			backup := file.Name()
			if closeErr := file.Close(); closeErr != nil {
				_ = os.Remove(backup)
				return rollbackReceiptOutputs(staged, fmt.Errorf("關閉收據備份：%w", closeErr))
			}
			if removeErr := os.Remove(backup); removeErr != nil {
				return rollbackReceiptOutputs(staged, fmt.Errorf("準備收據備份：%w", removeErr))
			}
			// Keep the old target visible until its replacement is ready.
			if linkErr := os.Link(item.path, backup); linkErr != nil {
				return rollbackReceiptOutputs(staged, fmt.Errorf("備份收據輸出：%w", linkErr))
			}
			item.backup = backup
		}
		if moveErr := rename(item.stage, item.path); moveErr != nil {
			return rollbackReceiptOutputs(staged, fmt.Errorf("提交收據輸出：%w", moveErr))
		}
		item.stage = ""
		item.published = true
	}
	n, writeErr := stdout.Write(receipt)
	if writeErr == nil && n != len(receipt) {
		writeErr = io.ErrShortWrite
	}
	if writeErr != nil {
		return rollbackReceiptOutputs(staged, fmt.Errorf("寫出 stdout 收據：%w", writeErr))
	}
	for _, item := range staged {
		if item.backup != "" {
			_ = os.Remove(item.backup)
		}
	}
	return nil
}

func rollbackReceiptOutputs(items []stagedReceiptOutput, cause error) error {
	for i := len(items) - 1; i >= 0; i-- {
		item := &items[i]
		if item.published {
			if removeErr := os.Remove(item.path); removeErr != nil {
				cause = errors.Join(cause, fmt.Errorf("撤銷新收據 %s：%w", item.path, removeErr))
			}
		}
		if item.backup != "" {
			if item.published {
				if restoreErr := os.Rename(item.backup, item.path); restoreErr != nil {
					cause = errors.Join(cause, fmt.Errorf("還原舊收據 %s（備份保留於 %s）：%w", item.path, item.backup, restoreErr))
				} else {
					item.backup = ""
				}
			} else if removeErr := os.Remove(item.backup); removeErr != nil {
				cause = errors.Join(cause, fmt.Errorf("清除未使用備份 %s：%w", item.backup, removeErr))
			} else {
				item.backup = ""
			}
		}
	}
	return cause
}

func syncReceiptFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	syncErr := file.Sync()
	return errors.Join(syncErr, file.Close())
}
