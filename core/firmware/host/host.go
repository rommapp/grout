package host

import (
	"io/fs"
	"strconv"
	"time"
)

type Host interface {
	Getenv(string) string
	ReadFile(string) ([]byte, error)
	Stat(string) (fs.FileInfo, error)
	ReadDir(string) ([]fs.DirEntry, error)
	Executable() (string, error)
}

type Writable interface {
	Host
	Real(p string) (string, error)
}

type Runner interface {
	Start(argv []string, delay time.Duration) error
}

func delaySeconds(d time.Duration) string {
	if d <= 0 {
		return "0"
	}
	return strconv.FormatInt(int64((d+time.Second-1)/time.Second), 10)
}
