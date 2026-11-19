package commands

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ekkolyth/miso/internal/scriptlog"
)

// under a parent miso (MISO=1) the line is left for it to render
func RunLog(args []string, stdout, stderr io.Writer) error {
	var msg scriptlog.Message
	if len(args) > 0 {
		msg.Level = scriptlog.Level(args[0])
	}
	if len(args) > 1 {
		msg.Text = args[1]
	}
	if len(args) > 2 {
		for _, arg := range args[2:] {
			key, value, ok := strings.Cut(arg, "=")
			if !ok {
				return fmt.Errorf("field %q is not key=value", arg)
			}
			msg.Fields = append(msg.Fields, scriptlog.Field{Key: key, Value: value})
		}
	}
	if err := msg.Validate(); err != nil {
		return err
	}
	if os.Getenv("MISO") == "1" {
		_, _ = fmt.Fprintln(stdout, scriptlog.Encode(msg))
		return nil
	}
	_, _ = fmt.Fprintln(stderr, scriptlog.Render(msg))
	return nil
}
