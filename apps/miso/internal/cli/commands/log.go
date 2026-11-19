package commands

import (
	"fmt"
	"io"
	"strings"

	"github.com/ekkolyth/miso/internal/scriptlog"
)

func RunLog(args []string, stdout io.Writer) error {
	var msg scriptlog.Message
	if len(args) > 0 {
		msg.Level = scriptlog.Level(args[0])
	}
	if len(args) > 1 {
		msg.Text = args[1]
	}
	for _, arg := range args[min(len(args), 2):] {
		key, value, ok := strings.Cut(arg, "=")
		if !ok {
			return fmt.Errorf("field %q is not key=value", arg)
		}
		msg.Fields = append(msg.Fields, scriptlog.Field{Key: key, Value: value})
	}
	if err := msg.Validate(); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(stdout, scriptlog.Render(msg))
	return nil
}
