package scriptlog

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var contractMessage = Message{
	Level: LevelError,
	Text:  "Building the Debian VM image requires Lima 2.x, but limactl was not found.",
	Fields: []Field{
		{Key: "install", Value: "brew install lima"},
		{Key: "then", Value: "miso dev"},
	},
}

func readSample(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "contracts", "log-line.txt"))
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSuffix(string(data), "\n")
}

func TestEncodeMatchesContract(t *testing.T) {
	if got := Encode(contractMessage); got != readSample(t) {
		t.Errorf("Encode() = %q, want %q", got, readSample(t))
	}
}

func TestParseReadsContract(t *testing.T) {
	got, ok := Parse(readSample(t))
	if !ok {
		t.Fatal("Parse() ok = false")
	}
	if !reflect.DeepEqual(got, contractMessage) {
		t.Errorf("Parse() = %+v, want %+v", got, contractMessage)
	}
}

func TestParseTrimsCarriageReturn(t *testing.T) {
	got, ok := Parse(readSample(t) + "\r")
	if !ok {
		t.Fatal("Parse() ok = false")
	}
	if !reflect.DeepEqual(got, contractMessage) {
		t.Errorf("Parse() = %+v, want %+v", got, contractMessage)
	}
}

func TestEncodeEmptyFields(t *testing.T) {
	want := `::miso::{"level":"info","text":"hi","fields":[]}`
	if got := Encode(Message{Level: LevelInfo, Text: "hi"}); got != want {
		t.Errorf("Encode() = %q, want %q", got, want)
	}
}

func TestParseRejects(t *testing.T) {
	tests := []struct {
		name string
		line string
	}{
		{"plain text", "plain text"},
		{"no marker", `{"level":"error","text":"x","fields":[]}`},
		{"not json", "::miso::not json"},
		{"unknown level", `::miso::{"level":"fatal","text":"x","fields":[]}`},
		{"empty text", `::miso::{"level":"error","text":"","fields":[]}`},
		{"short pair", `::miso::{"level":"error","text":"x","fields":[["k"]]}`},
		{"long pair", `::miso::{"level":"error","text":"x","fields":[["k","v","z"]]}`},
		{"empty key", `::miso::{"level":"error","text":"x","fields":[["","v"]]}`},
		{"leading space", `  ::miso::{"level":"error","text":"x","fields":[]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, ok := Parse(tt.line); ok {
				t.Errorf("Parse(%q) = %+v, true; want false", tt.line, got)
			}
		})
	}
}

func TestValidateErrors(t *testing.T) {
	tests := []struct {
		name string
		msg  Message
		want string
	}{
		{"missing level", Message{}, "missing level (use debug, info, warn, or error)"},
		{"unknown level", Message{Level: "fatal"}, `unknown level "fatal" (use debug, info, warn, or error)`},
		{"missing text", Message{Level: LevelInfo}, "missing message text"},
		{"empty key", Message{Level: LevelInfo, Text: "x", Fields: []Field{{Key: "", Value: "v"}}}, `field "=v" has an empty key`},
		{"valid", Message{Level: LevelInfo, Text: "x"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.msg.Validate()
			if tt.want == "" {
				if err != nil {
					t.Errorf("Validate() = %v, want nil", err)
				}
				return
			}
			if err == nil || err.Error() != tt.want {
				t.Errorf("Validate() = %v, want %q", err, tt.want)
			}
		})
	}
}
