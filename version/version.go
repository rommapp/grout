package version

import "os"

var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

type BuildInfo struct {
	Version   string
	GitCommit string
	BuildDate string
}

func Get() BuildInfo {
	v := Version
	if override := os.Getenv("GROUT_VERSION"); override != "" {
		v = override
	}
	return BuildInfo{
		Version:   v,
		GitCommit: GitCommit,
		BuildDate: BuildDate,
	}
}
