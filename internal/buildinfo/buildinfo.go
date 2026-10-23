// Package buildinfo exposes reproducible tool metadata independently of Cobra.
package buildinfo

import (
	"fmt"
	"strings"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildTime = "unknown"
	dirty     = "false"
)

// Info identifies the exact Sloth build that produced output or a report.
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`
	Dirty     bool   `json:"dirty"`
}

// Current returns metadata injected by release linker flags with safe fallbacks.
func Current() Info {
	return resolve(version, commit, buildTime, dirty)
}

func resolve(rawVersion, rawCommit, rawBuildTime, rawDirty string) Info {
	return Info{
		Version:   fallback(rawVersion, "dev"),
		Commit:    fallback(rawCommit, "unknown"),
		BuildTime: fallback(rawBuildTime, "unknown"),
		Dirty:     parseDirty(rawDirty),
	}
}

func fallback(value, defaultValue string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultValue
	}
	return value
}

func parseDirty(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "dirty":
		return true
	default:
		return false
	}
}

// Display returns concise human-readable version metadata.
func (i Info) Display() string {
	versionValue := fallback(i.Version, "dev")
	details := make([]string, 0, 3)
	if commitValue := fallback(i.Commit, "unknown"); commitValue != "unknown" {
		details = append(details, "commit "+commitValue)
	}
	if buildTimeValue := fallback(i.BuildTime, "unknown"); buildTimeValue != "unknown" {
		details = append(details, "built "+buildTimeValue)
	}
	if i.Dirty {
		details = append(details, "dirty")
	}
	if len(details) == 0 {
		return versionValue
	}
	return fmt.Sprintf("%s (%s)", versionValue, strings.Join(details, ", "))
}
