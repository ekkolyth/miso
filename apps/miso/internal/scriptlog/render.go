package scriptlog

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/ekkolyth/miso/internal/ui"
)

func Render(m Message) string {
	styles := ui.Default()
	badge := map[Level]lipgloss.Style{
		LevelDebug: styles.Muted,
		LevelInfo:  styles.Accent,
		LevelWarn:  lipgloss.NewStyle().Bold(true).Foreground(ui.WarningColor),
		LevelError: styles.Error,
	}[m.Level]

	var out strings.Builder
	out.WriteString(badge.Render(strings.ToUpper(string(m.Level))))
	out.WriteString(" " + m.Text)
	for _, field := range m.Fields {
		value := field.Value
		if strings.ContainsAny(value, " \"") {
			value = strconv.Quote(value)
		}
		out.WriteString(" " + styles.Muted.Render(field.Key+"=") + value)
	}
	return out.String()
}
