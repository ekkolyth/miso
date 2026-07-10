package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/ekkolyth/miso/internal/cli"
	"github.com/ekkolyth/miso/internal/ui"
)

const docsURL = "https://misojs.dev"

func RunHelp() {
	_, _ = fmt.Fprint(os.Stdout, RenderHelp())
}

// logo, tagline, grouped commands, docs link
func RenderHelp() string {
	styles := ui.Default()
	var out strings.Builder
	out.WriteString(RenderMisoLogo())
	out.WriteString("\n" + styles.Heading.Render("Miso") + " – the agnostic package manager\n\n")
	out.WriteString(renderCommandGroup(styles, "Commands", false))
	out.WriteString("\n")
	out.WriteString(renderCommandGroup(styles, "Miso", true))
	out.WriteString("\n" + styles.Muted.Render("Docs:") + " " + styles.Flavor.Render(docsURL) + "\n")
	return out.String()
}

func renderCommandGroup(styles ui.Styles, title string, meta bool) string {
	var out strings.Builder
	out.WriteString(styles.Label.Render(title) + "\n")
	for _, cmd := range cli.Builtins {
		if cmd.Meta != meta {
			continue
		}
		left := strings.Join(append([]string{cmd.Name}, cmd.Aliases...), ", ")
		if cmd.Usage != "" {
			left += " " + cmd.Usage
		}
		// pad the plain string before styling so ANSI codes don't skew the column
		_, _ = fmt.Fprintf(&out, "  %s %s\n", styles.Accent.Render(fmt.Sprintf("%-28s", left)), styles.Muted.Render(cmd.Summary))
	}
	return out.String()
}
