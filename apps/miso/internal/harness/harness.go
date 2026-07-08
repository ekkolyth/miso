package harness

import (
	"errors"
	"os/exec"
)

// Harness is a supported agent target for the skills tool.
type Harness struct {
	Agent string   // skills-tool -a identifier
	Label string   // display name in the selector
	Bins  []string // PATH binaries indicating it is installed (any match)
}

var table = []Harness{
	{Agent: "claude", Label: "Claude Code", Bins: []string{"claude"}},
	{Agent: "opencode", Label: "OpenCode", Bins: []string{"opencode"}},
	{Agent: "codex", Label: "Codex", Bins: []string{"codex"}},
	{Agent: "gemini", Label: "Gemini CLI", Bins: []string{"gemini"}},
	{Agent: "cursor", Label: "Cursor", Bins: []string{"cursor", "cursor-agent"}},
}

var errNotFound = errors.New("not found")

// seam for tests
var lookPath = exec.LookPath

// Detect returns the installed harnesses, in table order.
func Detect() []Harness {
	var found []Harness
	for _, entry := range table {
		for _, bin := range entry.Bins {
			if _, err := lookPath(bin); err == nil {
				found = append(found, entry)
				break
			}
		}
	}
	return found
}
