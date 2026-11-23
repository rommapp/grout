//go:build unix

package host

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func (h *osHost) Start(argv []string, delay time.Duration) error {
	if len(argv) == 0 {
		return errors.New("host: empty command")
	}
	null, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer null.Close()
	args := append([]string{"-c", `sleep "$0"; exec "$@"`, delaySeconds(delay)}, argv...)
	cmd := exec.Command("/bin/sh", args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = null, null, null
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}
