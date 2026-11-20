package scriptlog

import (
	"errors"
	"fmt"
)

type Level string

const (
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

type Field struct {
	Key   string
	Value string
}

type Message struct {
	Level  Level
	Text   string
	Fields []Field
}

const levelHint = "(use debug, info, warn, or error)"

func (m Message) Validate() error {
	switch m.Level {
	case LevelDebug, LevelInfo, LevelWarn, LevelError:
	case "":
		return errors.New("missing level " + levelHint)
	default:
		return fmt.Errorf("unknown level %q %s", string(m.Level), levelHint)
	}
	if m.Text == "" {
		return errors.New("missing message text")
	}
	for _, field := range m.Fields {
		if field.Key == "" {
			return fmt.Errorf("field %q has an empty key", "="+field.Value)
		}
	}
	return nil
}
