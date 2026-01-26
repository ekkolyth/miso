package core

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"

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

// display miso ASCII art with welcome message
func printMisoWelcome(styles ui.Styles) {
	// create color styles matching the original logo
	// purple for main structure
	purpleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7c3aed")) // purple from accent (main)
	// orange/yellow for highlights
	orangeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#f0a343"))
	yellowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#fbb115"))
	// white/light gray for spacing and details
	whiteStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff"))
	lightGrayStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#e4e6e5"))

	// render ASCII art with colors matching original logo
	// use lipgloss transform to scale down (though font size is terminal-dependent)
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
				// main structure - purple
				renderedLine.WriteString(purpleStyle.Render("@"))
			case '#':
				// can be purple (main) or orange (highlight) - alternate based on position
				if i%4 < 2 {
					renderedLine.WriteString(purpleStyle.Render("#"))
				} else {
					renderedLine.WriteString(orangeStyle.Render("#"))
				}
			case '%':
				// highlights - orange/yellow
				if i%2 == 0 {
					renderedLine.WriteString(orangeStyle.Render("%"))
				} else {
					renderedLine.WriteString(yellowStyle.Render("%"))
				}
			case '*':
				// main structure - purple
				renderedLine.WriteString(purpleStyle.Render("*"))
			case '+':
				// main structure - purple
				renderedLine.WriteString(purpleStyle.Render("+"))
			case '=':
				// can be orange (highlight) or white (detail)
				if i%3 == 0 {
					renderedLine.WriteString(orangeStyle.Render("="))
				} else {
					renderedLine.WriteString(lightGrayStyle.Render("="))
				}
			case '-':
				// white/light gray for details
				renderedLine.WriteString(lightGrayStyle.Render("-"))
			case ':':
				// white/light gray for details
				renderedLine.WriteString(whiteStyle.Render(":"))
			case '.':
				// white/light gray for details
				renderedLine.WriteString(lightGrayStyle.Render("."))
			case ' ':
				renderedLine.WriteString(" ")
			default:
				// default to purple for other chars
				renderedLine.WriteString(purpleStyle.Render(string(char)))
			}
		}
		rendered.WriteString(renderedLine.String())
		rendered.WriteString("\n")
	}
	
	// make more compact by removing extra blank lines
	asciiArt := rendered.String()

	// print ASCII art with minimal spacing
	fmt.Fprint(os.Stdout, asciiArt)

	// print welcome message in purple
	welcomeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7c3aed")).Bold(true)
	welcomeMsg := welcomeStyle.Render("Welcome to Miso! Let's get started:")
	fmt.Fprint(os.Stdout, welcomeMsg)
	fmt.Fprint(os.Stdout, "\n\n")
}
