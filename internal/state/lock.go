package state

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

type Lock struct {
	file *os.File
	path string
}

func NewLock(path string) *Lock {
	return &Lock{path: path}
}

func (l *Lock) Acquire() (bool, error) {
	file, err := os.OpenFile(l.path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return false, fmt.Errorf("open lock file: %w", err)
	}

	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		return false, nil
	}

	l.file = file

	content := fmt.Sprintf("%d\n", os.Getpid())
	file.Truncate(0)
	file.Seek(0, 0)
	file.WriteString(content)

	return true, nil
}

func (l *Lock) Release() error {
	if l.file == nil {
		return nil
	}
	syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	l.file.Close()
	l.file = nil
	return nil
}

func (l *Lock) ReadOwner() (int, error) {
	data, err := os.ReadFile(l.path)
	if err != nil {
		return 0, fmt.Errorf("read lock: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("parse lock: %w", err)
	}
	return pid, nil
}
