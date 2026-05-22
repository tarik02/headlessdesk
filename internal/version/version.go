package version

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
)

var (
	Version = "0.0.0-dev"
	Commit  = ""
	Date    = ""
)

type Info struct {
	Version   string
	Commit    string
	Date      string
	GoOS      string
	GoArch    string
	GoVersion string
}

func Get() Info {
	info := Info{
		Version: Version,
		Commit:  Commit,
		Date:    Date,
		GoOS:    runtime.GOOS,
		GoArch:  runtime.GOARCH,
	}

	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		info.GoVersion = buildInfo.GoVersion
		for _, setting := range buildInfo.Settings {
			switch setting.Key {
			case "vcs.revision":
				if info.Commit == "" {
					info.Commit = setting.Value
				}
			case "vcs.time":
				if info.Date == "" {
					info.Date = setting.Value
				}
			}
		}
	}

	return info
}

func Short() string {
	info := Get()
	if info.Commit == "" {
		return info.Version
	}
	return fmt.Sprintf("%s (%s)", info.Version, shortCommit(info.Commit))
}

func Details() string {
	info := Get()
	lines := []string{
		"version: " + info.Version,
		"go: " + info.GoVersion,
		"os/arch: " + info.GoOS + "/" + info.GoArch,
	}
	if info.Commit != "" {
		lines = append(lines, "commit: "+info.Commit)
	}
	if info.Date != "" {
		lines = append(lines, "date: "+info.Date)
	}
	return strings.Join(lines, "\n")
}

func shortCommit(commit string) string {
	if len(commit) < 7 {
		return commit
	}
	return commit[:7]
}
