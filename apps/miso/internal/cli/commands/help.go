package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/ekkolyth/miso/internal/cli"
)

const docsURL = "https://misojs.dev"

// RunHelp prints the miso logo and built-in command reference to stdout.
func RunHelp() {
	fmt.Fprint(os.Stdout, RenderHelp())
}

// RenderHelp builds the help screen: logo, tagline, grouped commands, docs link.
func RenderHelp() string {
	var out strings.Builder
	out.WriteString(RenderMisoLogo())
	out.WriteString("\nMiso – the agnostic package manager\n\n")
	out.WriteString(renderCommandGroup("Commands", false))
	out.WriteString("\n")
	out.WriteString(renderCommandGroup("Miso", true))
	out.WriteString("\nDocs: " + docsURL + "\n")
	return out.String()
}

func renderCommandGroup(title string, meta bool) string {
	var out strings.Builder
	out.WriteString(title + "\n")
	for _, cmd := range cli.Builtins {
		if cmd.Meta != meta {
			continue
		}
		left := strings.Join(append([]string{cmd.Name}, cmd.Aliases...), ", ")
		if cmd.Usage != "" {
			left += " " + cmd.Usage
		}
		out.WriteString(fmt.Sprintf("  %-28s %s\n", left, cmd.Summary))
	}
	return out.String()
}
