package spruce

import (
	"runtime"

	"grout/core/firmware/host"
)

var archCores = map[string]string{
	"PS":     "PCSX-ReARMed",
	"FAKE08": "fake-08",
}

func ArchSaves(_ host.Host, romFolder string) ([]string, bool) {
	arch := "64"
	if runtime.GOARCH == "arm" {
		arch = "32"
	}
	folders := archFolders(romFolder, arch)
	return folders, folders != nil
}

func archFolders(romFolder, arch string) []string {
	core, ok := archCores[romFolder]
	if !ok {
		return nil
	}
	return []string{core + "-" + arch}
}
