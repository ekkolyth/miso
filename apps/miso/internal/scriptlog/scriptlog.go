package scriptlog

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const Marker = "::miso::"

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

// fields are slices, not [2]string, so parse can reject wrong-length pairs
type wire struct {
	Level  string     `json:"level"`
	Text   string     `json:"text"`
	Fields [][]string `json:"fields"`
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

func Encode(m Message) string {
	fields := make([][]string, 0, len(m.Fields))
	for _, field := range m.Fields {
		fields = append(fields, []string{field.Key, field.Value})
	}
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	// keep <, > and & literal so the line stays readable
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(wire{Level: string(m.Level), Text: m.Text, Fields: fields})
	return Marker + strings.TrimSuffix(out.String(), "\n")
}

func Parse(line string) (Message, bool) {
	// a pty ends each line with \r
	payload, found := strings.CutPrefix(strings.TrimSuffix(line, "\r"), Marker)
	if !found {
		return Message{}, false
	}
	var decoded wire
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		return Message{}, false
	}
	msg := Message{Level: Level(decoded.Level), Text: decoded.Text}
	for _, pair := range decoded.Fields {
		if len(pair) != 2 {
			return Message{}, false
		}
		msg.Fields = append(msg.Fields, Field{Key: pair[0], Value: pair[1]})
	}
	if msg.Validate() != nil {
		return Message{}, false
	}
	return msg, true
}
