// Package reference renders per-command documentation for `miso help <cmd>`
// from content embedded at build time.
package reference

import (
	"embed"
	"fmt"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/glamour"

	"github.com/ekkolyth/miso/internal/cli"
)

//go:generate go run ../refgen/gen

//go:embed content/*.mdx
var contentFS embed.FS

// raw markdown for a command; false if none embedded
func Body(name string) (string, bool) {
	data, err := contentFS.ReadFile("content/" + name + ".mdx")
	if err != nil {
		return "", false
	}
	return string(data), true
}

func renderMarkdown(body string) string {
	out, err := glamour.Render(body, "auto")
	if err != nil {
		return body
	}
	return out
}

func header(cmd cli.Command) string {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7c3aed"))
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280"))

	usage := "miso " + cmd.Name
	if cmd.Usage != "" {
		usage += " " + cmd.Usage
	}

	var out strings.Builder
	out.WriteString(title.Render("miso "+cmd.Name) + "\n")
	out.WriteString(muted.Render(cmd.Summary) + "\n")
	out.WriteString(muted.Render("Usage: "+usage) + "\n")
	if len(cmd.Aliases) > 0 {
		out.WriteString(muted.Render("Aliases: "+strings.Join(cmd.Aliases, ", ")) + "\n")
	}
	return out.String()
}

// full help screen for a command; false if the name is not a builtin
func Render(name string) (string, bool) {
	cmd, ok := cli.LookupBuiltin(name)
	if !ok {
		return "", false
	}
	var out strings.Builder
	out.WriteString(header(cmd))
	if body, ok := Body(cmd.Name); ok {
		out.WriteString("\n" + renderMarkdown(body))
	} else {
		out.WriteString("\n(no extended docs for this command)\n")
	}
	return out.String(), true
}

// prints per-command help; errors on an unknown command
func RunCommandHelp(name string) error {
	out, ok := Render(name)
	if !ok {
		fmt.Fprintf(os.Stderr, "no help for %q — run `miso help` to see all commands\n", name)
		return fmt.Errorf("unknown command %q", name)
	}
	_, _ = fmt.Fprint(os.Stdout, out)
	return nil
}
