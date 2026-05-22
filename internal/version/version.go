package version

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
)

var (
	Version = "dev"
	Commit  = ""
	Date    = ""
	BuiltBy = ""
)

type Info struct {
	Version   string
	Commit    string
	Date      string
	BuiltBy   string
	Dirty     bool
	GoOS      string
	GoArch    string
	GoVersion string
}

func Get() Info {
	info := Info{
		Version: Version,
		Commit:  Commit,
		Date:    Date,
		BuiltBy: BuiltBy,
		GoOS:    runtime.GOOS,
		GoArch:  runtime.GOARCH,
	}

	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		info.GoVersion = buildInfo.GoVersion
		if info.Version == "dev" && buildInfo.Main.Version != "" && buildInfo.Main.Version != "(devel)" {
			info.Version = strings.TrimPrefix(buildInfo.Main.Version, "v")
		}
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
			case "vcs.modified":
				info.Dirty = setting.Value == "true"
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
	if info.BuiltBy != "" {
		lines = append(lines, "built_by: "+info.BuiltBy)
	}
	if info.Dirty {
		lines = append(lines, "dirty: true")
	}
	return strings.Join(lines, "\n")
}

func shortCommit(commit string) string {
	if len(commit) < 7 {
		return commit
	}
	return commit[:7]
}
