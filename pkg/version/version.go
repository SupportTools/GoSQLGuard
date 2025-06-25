package version

import "fmt"

// Version constants for the application
const (
	Version   = "0.1.0"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

// Info contains version information for the application
type Info struct {
	Version   string
	GitCommit string
	BuildTime string
}

func (v Info) String() string {
	return fmt.Sprintf("Version: %s\nGitCommit: %s\nBuildTime: %s",
		v.Version, v.GitCommit, v.BuildTime)
}
