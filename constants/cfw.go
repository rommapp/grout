package constants

type CFW string

const (
	NextUI CFW = "NEXTUI"
	MuOS   CFW = "MUOS"
	Knulli CFW = "KNULLI"
)

const MuOSSD1 = "/mnt/mmc"
const MuOSSD2 = "/mnt/sdcard"
const MuOSRomsFolderUnion = "/mnt/union/ROMS"

var NextUIPlatforms = mustLoadJSONMap[string, []string]("cfw/nextui/platforms.json")

var NextUISaveDirectories = mustLoadJSONMap[string, []string]("cfw/nextui/save_directories.json")

var MuOSPlatforms = mustLoadJSONMap[string, []string]("cfw/muos/platforms.json")

var MuOSSaveDirectories = mustLoadJSONMap[string, []string]("cfw/muos/save_directories.json")

var MuOSArtDirectory = mustLoadJSONMap[string, string]("cfw/muos/art_directories.json")

var KnulliPlatforms = mustLoadJSONMap[string, []string]("cfw/knulli/platforms.json")

var KnulliSaveDirectories = KnulliPlatforms
