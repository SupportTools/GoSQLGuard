package version

import "fmt"

const (
    Version      = "0.1.0"
    GitCommit    = "unknown"
    BuildTime    = "unknown"
)

type VersionInfo struct {
    Version      string
    GitCommit    string
    BuildTime    string
}

func (v VersionInfo) String() string {
    return fmt.Sprintf("Version: %s\nGitCommit: %s\nBuildTime: %s",
        v.Version, v.GitCommit, v.BuildTime)
}
