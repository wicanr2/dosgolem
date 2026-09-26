// Package bootroot prepares a writable save tree from a read-only original
// tree for the future Linux launcher.  It verifies pinned file hashes,
// enforces original/save separation and copies the tree; it never touches
// machine, DOS, owners, translations or windows.
package bootroot

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// maxRequiredFileSize guards against corrupt trees, not versions.
const maxRequiredFileSize = 64 << 20

// RequiredFile pins one file every boot must reproduce byte-for-byte.
type RequiredFile struct {
	Name   string
	SHA256 [32]byte
}

// BootRootInput carries caller-pinned roots and file hashes.
type BootRootInput struct {
	OriginalRoot string
	SaveRoot     string
	Required     []RequiredFile
}

// BootRootOutput proves one prepared save tree.
type BootRootOutput struct {
	SaveRoot string
	Verified map[string][32]byte
}

func validBaseName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	base := filepath.Base(name)
	return base == name && !strings.ContainsAny(name, `\/:`)
}

// Prepare verifies the original tree and reproduces it under the save root.
// The save root is created (0700) when missing; a non-empty save root is
// rejected.  On mid-copy failure it best-effort removes what it created.
func Prepare(input BootRootInput) (BootRootOutput, error) {
	original, err := filepath.Abs(input.OriginalRoot)
	if err != nil {
		return BootRootOutput{}, fmt.Errorf("bootroot: original 路徑無效: %v", err)
	}
	original = filepath.Clean(original)
	save, err := filepath.Abs(input.SaveRoot)
	if err != nil {
		return BootRootOutput{}, fmt.Errorf("bootroot: save 路徑無效: %v", err)
	}
	save = filepath.Clean(save)
	originalInfo, err := os.Lstat(original)
	if err != nil {
		return BootRootOutput{}, fmt.Errorf("bootroot: original 不可用: %v", err)
	}
	if originalInfo.Mode()&os.ModeSymlink != 0 || !originalInfo.IsDir() {
		return BootRootOutput{}, errors.New("bootroot: original 須為非連結目錄")
	}
	if len(input.Required) == 0 {
		return BootRootOutput{}, errors.New("bootroot: 缺少必要檔清單")
	}
	seen := make(map[string]bool, len(input.Required))
	for _, req := range input.Required {
		if !validBaseName(req.Name) {
			return BootRootOutput{}, fmt.Errorf("bootroot: 檔名不合法: %q", req.Name)
		}
		if seen[req.Name] {
			return BootRootOutput{}, fmt.Errorf("bootroot: 檔名重複: %q", req.Name)
		}
		seen[req.Name] = true
		if err := checkRequiredFile(original, req); err != nil {
			return BootRootOutput{}, err
		}
	}
	createdRoot := false
	for _, pair := range [][2]string{{original, save}, {save, original}} {
		rel, err := filepath.Rel(pair[0], pair[1])
		if err != nil {
			return BootRootOutput{}, fmt.Errorf("bootroot: 路徑比較: %v", err)
		}
		if rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return BootRootOutput{}, errors.New("bootroot: save 與 original 不可互為巢狀")
		}
	}
	saveInfo, err := os.Lstat(save)
	if err != nil {
		if !os.IsNotExist(err) {
			return BootRootOutput{}, fmt.Errorf("bootroot: save 不可用: %v", err)
		}
		if err := os.Mkdir(save, 0o700); err != nil {
			return BootRootOutput{}, fmt.Errorf("bootroot: 建 save 目錄: %v", err)
		}
		createdRoot = true
		if err := os.Chmod(save, 0o700); err != nil {
			removeCreated(save, nil, true)
			return BootRootOutput{}, fmt.Errorf("bootroot: save 權限: %v", err)
		}
		saveInfo, err = os.Lstat(save)
		if err != nil {
			removeCreated(save, nil, true)
			return BootRootOutput{}, fmt.Errorf("bootroot: save 不可用: %v", err)
		}
	}
	if saveInfo.Mode()&os.ModeSymlink != 0 || !saveInfo.IsDir() {
		return BootRootOutput{}, errors.New("bootroot: save 須為非連結目錄")
	}
	originalRootInfo, err := os.Lstat(original)
	if err != nil {
		return BootRootOutput{}, fmt.Errorf("bootroot: original 不可用: %v", err)
	}
	if os.SameFile(originalRootInfo, saveInfo) {
		return BootRootOutput{}, errors.New("bootroot: save 不可等同 original")
	}
	if err := unix.Access(save, unix.W_OK|unix.X_OK); err != nil {
		return BootRootOutput{}, fmt.Errorf("bootroot: save 不可寫: %v", err)
	}
	entries, err := os.ReadDir(save)
	if err != nil {
		return BootRootOutput{}, fmt.Errorf("bootroot: 讀 save: %v", err)
	}
	if len(entries) > 0 {
		return BootRootOutput{}, errors.New("bootroot: save 須為空目錄")
	}
	created, err := copyTree(original, save)
	if err != nil {
		removeCreated(save, created, createdRoot)
		return BootRootOutput{}, err
	}
	verified := make(map[string][32]byte, len(input.Required))
	for _, req := range input.Required {
		if err := checkRequiredFile(save, req); err != nil {
			removeCreated(save, created, createdRoot)
			return BootRootOutput{}, err
		}
		verified[req.Name] = req.SHA256
	}
	return BootRootOutput{SaveRoot: save, Verified: verified}, nil
}

func checkRequiredFile(root string, req RequiredFile) error {
	info, err := os.Lstat(filepath.Join(root, req.Name))
	if err != nil {
		return fmt.Errorf("bootroot: 缺少 %q: %v", req.Name, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("bootroot: %q 非常規檔", req.Name)
	}
	if info.Size() > maxRequiredFileSize {
		return fmt.Errorf("bootroot: %q 過大", req.Name)
	}
	data, err := os.ReadFile(filepath.Join(root, req.Name))
	if err != nil {
		return fmt.Errorf("bootroot: 讀 %q: %v", req.Name, err)
	}
	if sha256.Sum256(data) != req.SHA256 {
		return fmt.Errorf("bootroot: %q 雜湊不符", req.Name)
	}
	return nil
}

// copyTree reproduces directories (0700) and regular files (0600) and
// returns created paths leaf-first for best-effort cleanup.
func copyTree(original, save string) ([]string, error) {
	var created []string
	// DOS 經 FindFirst／AH=57h 把檔案時間交給程式；複製品必須帶來源的 mtime，
	// 否則程式記憶體隨複製當下的主機時間變動（docs/spec/237 §2.5）。
	// 目錄的 mtime 會被之後建立的子項改掉，所以整棵樹複製完再回填。
	type dirTime struct {
		path string
		mod  time.Time
	}
	var dirs []dirTime
	err := filepath.WalkDir(original, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(original, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(save, rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || (!info.IsDir() && !info.Mode().IsRegular()) {
			return fmt.Errorf("bootroot: 樹內非常規節點: %q", rel)
		}
		if info.IsDir() {
			if err := os.Mkdir(target, 0o700); err != nil {
				return err
			}
			created = append(created, target)
			if err := os.Chmod(target, 0o700); err != nil {
				return err
			}
			dirs = append(dirs, dirTime{target, info.ModTime()})
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		dst, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return err
		}
		created = append(created, target)
		if _, err := dst.Write(data); err != nil {
			dst.Close()
			return err
		}
		if err := dst.Close(); err != nil {
			return err
		}
		if err := os.Chmod(target, 0o600); err != nil {
			return err
		}
		if err := os.Chtimes(target, info.ModTime(), info.ModTime()); err != nil {
			return err
		}
		back, err := os.ReadFile(target)
		if err != nil {
			return err
		}
		if sha256.Sum256(back) != sha256.Sum256(data) {
			return fmt.Errorf("bootroot: 複製 %q 回讀不符", rel)
		}
		return nil
	})
	if err != nil {
		return created, err
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		if err := os.Chtimes(dirs[i].path, dirs[i].mod, dirs[i].mod); err != nil {
			return created, err
		}
	}
	return created, nil
}

func removeCreated(save string, created []string, createdRoot bool) {
	for i := len(created) - 1; i >= 0; i-- {
		os.Remove(created[i])
	}
	if createdRoot {
		os.Remove(save)
	}
}
