package turbo

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	turboLineRe = regexp.MustCompile(`^([a-zA-Z0-9@/_.-]+:[a-zA-Z0-9:_.-]+): (.*)$`)
	exitCodeRe  = regexp.MustCompile(`^exited with code (\d+)$`)
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
	text := matches[2]
	meta := LineMeta{
		Label: label,
		Text:  text,
	}

	// Check for exit code
	if exitMatches := exitCodeRe.FindStringSubmatch(text); exitMatches != nil {
		code, err := strconv.Atoi(exitMatches[1])
		if err == nil {
			meta.IsExit = true
			meta.ExitCode = code
			return meta
		}
	}

	// Check for cache status
	if strings.HasPrefix(text, "cache hit") {
		hit := true
		meta.CacheHit = &hit
	} else if strings.HasPrefix(text, "cache miss") {
		hit := false
		meta.CacheHit = &hit
	}

	return meta
}
