package security

import (
	"os"
	"fmt"
)

func SafeCreate(sp *SecurePath) (*os.File, error) {
	if sp == nil {
		return nil, fmt.Errorf("cannot create file with nil SecurePath")
	}
	return os.Create(sp.path)
}

func SafeRemove(sp *SecurePath) error {
	if sp == nil {
		return fmt.Errorf("cannot remove file with nil SecurePath")
	}
	return os.Remove(sp.path)
}

func SafeMkdirAll(sp *SecurePath, perm os.FileMode) error {
	if sp == nil {
		return fmt.Errorf("cannot create directory with nil SecurePath")
	}
	return os.MkdirAll(sp.path, perm)
}

func SafeOpen(sp *SecurePath) (*os.File, error) {
	if sp == nil {
		return nil, fmt.Errorf("cannot open file with nil SecurePath")
	}
	return os.Open(sp.path)
}

func SafeStat(sp *SecurePath) (os.FileInfo, error) {
	if sp == nil {
		return nil, fmt.Errorf("cannot stat file with nil SecurePath")
	}
	return os.Stat(sp.path)
}

func SafeRename(oldpath, newpath *SecurePath) error {
	if oldpath == nil || newpath == nil {
		return fmt.Errorf("cannot rename with nil SecurePath")
	}
	return os.Rename(oldpath.path, newpath.path)
}

func SafeRemoveIfExists(sp *SecurePath) error {
	if sp == nil {
		return nil
	}
	err := os.Remove(sp.path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func SafeStatExists(sp *SecurePath) bool {
	if sp == nil {
		return false
	}
	_, err := os.Stat(sp.path)
	return err == nil
}