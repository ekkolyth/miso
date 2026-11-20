package scriptlog

import "testing"

var exampleMessage = Message{
	Level: LevelError,
	Text:  "Building the Debian VM image requires Lima 2.x, but limactl was not found.",
	Fields: []Field{
		{Key: "install", Value: "brew install lima"},
		{Key: "then", Value: "miso dev"},
	},
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
