//go:build unix

package grout

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

const lockFile = ".grout.lock"

func lockDataDir(dir string) (*os.File, error) {
	path := filepath.Join(dir, lockFile)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, newError(CodeStorage, fmt.Errorf("opening %s: %w", path, err))
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, newError(CodeDataDirLocked, fmt.Errorf("data directory %s is in use by another Grout", dir))
		}
		return nil, newError(CodeStorage, fmt.Errorf("locking %s: %w", path, err))
	}
	return f, nil
}

func unlockDataDir(f *os.File) error {
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_UN); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
