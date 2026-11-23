package host

import "io/fs"

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
