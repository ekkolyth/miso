package turbo

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	// turboLineRe matches "<pkg>:<task>: <text>". It tolerates the SGR color that
	// turbo wraps around the prefix when color is forced (FORCE_COLOR / a color
	// terminal): "\x1b[35m<label>: \x1b[0m<text>". Without this, colored task
	// lines fail to parse and every workspace's output is silently dropped.
	turboLineRe = regexp.MustCompile(`^(?:\x1b\[[0-9;]*m)*([a-zA-Z0-9@/_.-]+:[a-zA-Z0-9:_.-]+): (?:\x1b\[[0-9;]*m)*(.*)$`)
	exitCodeRe  = regexp.MustCompile(`^exited with code (\d+)$`)
	sgrRe       = regexp.MustCompile(`\x1b\[[0-9;]*m`)
)

// LineMeta holds parsed metadata for a single line of turbo output.
type LineMeta struct {
	Label    string
	Text     string
	Skip     bool
	IsExit   bool
	ExitCode int
	CacheHit *bool
}

// non-matching lines returned with Skip=true
func ParseLine(line string) LineMeta {
	matches := turboLineRe.FindStringSubmatch(line)
	if matches == nil {
		return LineMeta{Skip: true}
	}

	label := matches[1]
	text := matches[2] // keep color for display
	meta := LineMeta{
		Label: label,
		Text:  text,
	}

	// Structural checks run against a color-stripped copy — turbo colors the exit
	// and cache status text too when color is forced.
	plain := sgrRe.ReplaceAllString(text, "")

	// Check for exit code
	if exitMatches := exitCodeRe.FindStringSubmatch(plain); exitMatches != nil {
		code, err := strconv.Atoi(exitMatches[1])
		if err == nil {
			meta.IsExit = true
			meta.ExitCode = code
			return meta
		}
	}

	// Check for cache status
	if strings.HasPrefix(plain, "cache hit") {
		hit := true
		meta.CacheHit = &hit
	} else if strings.HasPrefix(plain, "cache miss") {
		hit := false
		meta.CacheHit = &hit
	}

	return meta
}
