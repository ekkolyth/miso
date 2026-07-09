package commands

import (
	"fmt"
	"os"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/ekkolyth/miso/internal/ui"
)

var misoASCII = []string{
	"                                                                       ",
	"                                                           @@@@@       ",
	"                                                         @@@ @@@       ",
	"                                                       @@@ @@@         ",
	"                    @@@@@@                           @@@ @@@           ",
	"                  @@@    @@@@                      @@@ @@@             ",
	"        @@@@@@   @@         @@@@@@@              @@@  @@      @@@@     ",
	"       @@@@@@@@@@@    @       @@@@@@@@@@@@@@@  @@@  @@     @@@@ @@@    ",
	"       @@@@@@@@@@  @  @@        @@@ @@@@@@ @@@@@ @@@    @@@@ @@@@      ",
	"       @@@@@@@@@ @   @   @@ @   @@@@@@@@@@ @@@  @@@@@@@@@  @@@         ",
	"       @@@@@@@@@@@@@@@@  @@@  @ @@@ @@@@@@@@ @@@   @@   @@@            ",
	"       @@@@@@          @@   @@  @@@@@@@@@  @@@  @@   @@@@@@@           ",
	"       @@@@   @    @     @@    @@@@@@@@  @@@@@@@@@@@@@ @@@  @@@        ",
	"     @@@@@   @      @     @@@@@        @@@@@       @   @  @@ @@@@      ",
	"     @@ @@   @      @    @@                 @@@   @ @@@@@@@@@@@ @@     ",
	"    @@@  @@@  @    @    @@                    @@ @@  @ @@@ @@@ @@@     ",
	"    @@@@@@  @@@@@    @@@                         @@ @@@@@@@  @@@@@     ",
	"     @@   @@@@     @@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@     @@@@   @@     ",
	"     @@        @@@@@@@@                           @@@@@@        @@     ",
	"     @@                   @@@@@@@@@@@@@@@@@@@@                 @@      ",
	"      @@                                                       @@      ",
	"      @@@                                                     @@       ",
	"       @@             @@@@@                 @@@@@            @@@       ",
	"        @@            @@@@@                @@@@@@           @@@        ",
	"         @@@          @@@@@                 @@@@           @@          ",
	"           @@                 @@@@@@@@@@                  @@           ",
	"            @@@       @@@     @@@@@@@@@@     @@@@       @@@            ",
	"              @@@               @@@@@@                @@@              ",
	"                @@@                                @@@@                ",
	"                   @@@@                         @@@@                   ",
	"                      @@@@@@@@@@@@@@@@@@@@@@@@@@                       ",
	"                                                                       ",
	"                                                                       ",
	"                                                                       ",
	"                                                          ",
}

// RenderMisoLogo returns the colored miso ASCII logo.
func RenderMisoLogo() string {
	// color styles matching logo
	purpleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7c3aed"))
	orangeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#f0a343"))
	yellowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#fbb115"))
	whiteStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff"))
	lightGrayStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#e4e6e5"))

	// render with colors
	var rendered strings.Builder
	for _, line := range misoASCII {
		if strings.TrimSpace(line) == "" {
			rendered.WriteString("\n")
			continue
		}

		var renderedLine strings.Builder
		for i, char := range line {
			switch char {
			case '@':
				renderedLine.WriteString(purpleStyle.Render("@"))
			case '#':
				if i%4 < 2 {
					renderedLine.WriteString(purpleStyle.Render("#"))
				} else {
					renderedLine.WriteString(orangeStyle.Render("#"))
				}
			case '%':
				if i%2 == 0 {
					renderedLine.WriteString(orangeStyle.Render("%"))
				} else {
					renderedLine.WriteString(yellowStyle.Render("%"))
				}
			case '*':
				renderedLine.WriteString(purpleStyle.Render("*"))
			case '+':
				renderedLine.WriteString(purpleStyle.Render("+"))
			case '=':
				if i%3 == 0 {
					renderedLine.WriteString(orangeStyle.Render("="))
				} else {
					renderedLine.WriteString(lightGrayStyle.Render("="))
				}
			case '-':
				renderedLine.WriteString(lightGrayStyle.Render("-"))
			case ':':
				renderedLine.WriteString(whiteStyle.Render(":"))
			case '.':
				renderedLine.WriteString(lightGrayStyle.Render("."))
			case ' ':
				renderedLine.WriteString(" ")
			default:
				renderedLine.WriteString(purpleStyle.Render(string(char)))
			}
		}
		rendered.WriteString(renderedLine.String())
		rendered.WriteString("\n")
	}

	return rendered.String()
}

// display miso ASCII art with welcome message
func printMisoWelcome(_ ui.Styles) {
	_, _ = fmt.Fprint(os.Stdout, RenderMisoLogo())

	welcomeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7c3aed")).Bold(true)
	welcomeMsg := welcomeStyle.Render("Welcome to Miso! Let's get started:")
	_, _ = fmt.Fprint(os.Stdout, welcomeMsg)
	_, _ = fmt.Fprint(os.Stdout, "\n\n")
}
