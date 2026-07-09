package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/ekkolyth/miso/internal/cli"
	"github.com/ekkolyth/miso/internal/ui"
)

const docsURL = "https://misojs.dev"

// RunHelp prints the miso logo and built-in command reference to stdout.
func RunHelp() {
	_, _ = fmt.Fprint(os.Stdout, RenderHelp())
}

// RenderHelp builds the help screen: logo, tagline, grouped commands, docs link.
func RenderHelp() string {
	s := ui.Default()
	var out strings.Builder
	out.WriteString(RenderMisoLogo())
	out.WriteString("\n" + s.Heading.Render("Miso – the agnostic package manager") + "\n\n")
	out.WriteString(renderCommandGroup(s, "Commands", false))
	out.WriteString("\n")
	out.WriteString(renderCommandGroup(s, "Miso", true))
	out.WriteString("\n" + s.Muted.Render("Docs:") + " " + s.Flavor.Render(docsURL) + "\n")
	return out.String()
}

func renderCommandGroup(s ui.Styles, title string, meta bool) string {
	var out strings.Builder
	out.WriteString(s.Label.Render(title) + "\n")
	for _, cmd := range cli.Builtins {
		if cmd.Meta != meta {
			continue
		}
		left := strings.Join(append([]string{cmd.Name}, cmd.Aliases...), ", ")
		if cmd.Usage != "" {
			left += " " + cmd.Usage
		}
		// pad the plain string before styling so ANSI codes don't skew the column
		_, _ = fmt.Fprintf(&out, "  %s %s\n", s.Accent.Render(fmt.Sprintf("%-28s", left)), s.Muted.Render(cmd.Summary))
	}
	return out.String()
}
