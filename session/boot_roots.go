package session

import "github.com/wicanr2/dosgolem/internal/fsaccess"

func prepareBootRoots(original, save string) (string, string, error) {
	if original == "" {
		return "", save, checkSaveRoot(save)
	}
	original, save, err := fsaccess.RootPair(original, save)
	if err != nil {
		return "", "", err
	}
	if err = checkSaveRoot(save); err != nil {
		return "", "", err
	}
	return original, save, nil
}
