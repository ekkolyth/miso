package env

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestParseDotenv(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		lookup   map[string]string
		expected map[string]string
	}{
		// syntax cases: expected values are godotenv v1.5.1's output
		{name: "blank and comment lines", src: "\n# c\n\nA=1\n", expected: map[string]string{"A": "1"}},
		{name: "export prefix", src: "export A=v", expected: map[string]string{"A": "v"}},
		{name: "yaml style", src: "A: v", expected: map[string]string{"A": "v"}},
		{name: "whitespace around equals", src: "A  =  v  ", expected: map[string]string{"A": "v"}},
		{name: "inline comment", src: "A=v # comment", expected: map[string]string{"A": "v"}},
		{name: "tab inline comment", src: "A=v\t# c", expected: map[string]string{"A": "v"}},
		{name: "last inline comment wins", src: "A=v # c # d", expected: map[string]string{"A": "v # c"}},
		{name: "hash without space", src: "A=a#b", expected: map[string]string{"A": "a#b"}},
		{name: "single quoted hash", src: "A='a # b'", expected: map[string]string{"A": "a # b"}},
		{name: "double quoted newline escape", src: `A="a\nb"`, expected: map[string]string{"A": "a\nb"}},
		{name: "double quoted other escape", src: `A="a\tb"`, expected: map[string]string{"A": "atb"}},
		{name: "double quoted escaped quotes", src: `A="say \"hi\""`, expected: map[string]string{"A": `say "hi\`}},
		{name: "double quoted multi-line", src: "A=\"line1\nline2\"\nB=2", expected: map[string]string{"A": "line1\nline2", "B": "2"}},
		{name: "single quoted multi-line", src: "A='x\ny'", expected: map[string]string{"A": "x\ny"}},
		{name: "single quoted escaped quote", src: `A='it\'s'`, expected: map[string]string{"A": `it\'s`}},
		{name: "unquoted backslash literal", src: `A=a\nb`, expected: map[string]string{"A": `a\nb`}},
		{name: "quoted value then comment", src: `A="v" # c`, expected: map[string]string{"A": "v"}},
		{name: "crlf", src: "A=1\r\nB=2\r\n", expected: map[string]string{"A": "1", "B": "2"}},
		{name: "empty values", src: "A=\nB=''\nC=\"\"\nD=  ", expected: map[string]string{"A": "", "B": "", "C": "", "D": ""}},
		{name: "dotted key", src: "a.b=1", expected: map[string]string{"a.b": "1"}},

		// expansion
		{name: "default when unset", src: "A=${UNSET:-postgres://x}", expected: map[string]string{"A": "postgres://x"}},
		{name: "process env wins over default", src: `A="${A:-d}"`, lookup: map[string]string{"A": "ci"}, expected: map[string]string{"A": "ci"}},
		{name: "empty counts as unset for default", src: `A="${A:-d}"`, lookup: map[string]string{"A": ""}, expected: map[string]string{"A": "d"}},
		{name: "earlier key in file", src: "A=x\nB=${A}/0", expected: map[string]string{"A": "x", "B": "x/0"}},
		{name: "process env wins over file", src: "A=file\nB=$A", lookup: map[string]string{"A": "env"}, expected: map[string]string{"A": "file", "B": "env"}},
		{name: "home in double quotes", src: `H="$HOME/x"`, lookup: map[string]string{"HOME": "/h"}, expected: map[string]string{"H": "/h/x"}},
		{name: "unset reference is empty", src: "N=$NOPE", expected: map[string]string{"N": ""}},
		{name: "single quotes stay literal", src: "S='${S:-lit}'", expected: map[string]string{"S": "${S:-lit}"}},
		{name: "escaped dollar in double quotes", src: `E="\$A"`, lookup: map[string]string{"A": "x"}, expected: map[string]string{"E": "$A"}},
		{name: "escaped dollar unquoted", src: `E=\$A`, lookup: map[string]string{"A": "x"}, expected: map[string]string{"E": "$A"}},
		{name: "lower-case name", src: "my_var=v\nB=${my_var}", expected: map[string]string{"my_var": "v", "B": "v"}},
		{name: "nested default reference", src: "B=b\nA=${A:-$B}", expected: map[string]string{"B": "b", "A": "b"}},
		{name: "nested default braces", src: "B=b\nA=\"${A:-${B}/x}\"", expected: map[string]string{"B": "b", "A": "b/x"}},
		{name: "nested default of default", src: "A=${A:-${B:-deep}}", expected: map[string]string{"A": "deep"}},
		{name: "double dollar literal", src: "P=pa$$word", expected: map[string]string{"P": "pa$$word"}},
		{name: "trailing dollar literal", src: "P=cost$", expected: map[string]string{"P": "cost$"}},
		{name: "positional literal", src: "P=$1", expected: map[string]string{"P": "$1"}},
		{name: "dash literal", src: "P=a$-b", expected: map[string]string{"P": "a$-b"}},
		{name: "dollar before space literal", src: "P=a$ b", expected: map[string]string{"P": "a$ b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDotenv([]byte(tt.src), "test.env", mapLookup(tt.lookup))
			if err != nil {
				t.Fatalf("parseDotenv: %v", err)
			}
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestParseDotenv_Errors(t *testing.T) {
	tests := []struct {
		name         string
		src          string
		expectedLine int
		expectedKey  string
		contains     string
	}{
		{name: "unterminated double quote", src: "A=1\nB=\"s3cr3t", expectedLine: 2, expectedKey: "B", contains: "unterminated quoted value"},
		{name: "unterminated single quote", src: "B='s3cr3t", expectedLine: 1, expectedKey: "B", contains: "unterminated quoted value"},
		{name: "invalid key character", src: "\nBAD-KEY=s3cr3t", expectedLine: 2, contains: `unexpected character "-"`},
		{name: "error form", src: "X=${X:?s3cr3t}", expectedLine: 1, expectedKey: "X", contains: "${X:?"},
		{name: "assign form", src: "X=${X:=s3cr3t}", expectedLine: 1, expectedKey: "X", contains: "${X:="},
		{name: "alternate form", src: "X=${X:+s3cr3t}", expectedLine: 1, expectedKey: "X", contains: "${X:+"},
		{name: "unset-only default", src: "X=${X-s3cr3t}", expectedLine: 1, expectedKey: "X", contains: "${X-"},
		{name: "unset-only error", src: "X=${X?s3cr3t}", expectedLine: 1, expectedKey: "X", contains: "${X?"},
		{name: "unset-only alternate", src: "X=${X+s3cr3t}", expectedLine: 1, expectedKey: "X", contains: "${X+"},
		{name: "length", src: "X=${#s3cr3t}", expectedLine: 1, expectedKey: "X", contains: "${#"},
		{name: "substitution", src: "X=${X/s3cr3t/y}", expectedLine: 1, expectedKey: "X", contains: "${X/"},
		{name: "suffix removal", src: "X=${X%s3cr3t}", expectedLine: 1, expectedKey: "X", contains: "${X%"},
		{name: "empty braces", src: "X=${}", expectedLine: 1, expectedKey: "X", contains: "${}"},
		{name: "unterminated brace", src: "X=\"${s3cr3t\"", expectedLine: 1, expectedKey: "X", contains: "unterminated"},
		{name: "unterminated default", src: "X=${X:-s3cr3t", expectedLine: 1, expectedKey: "X", contains: "unterminated"},
		{name: "command substitution", src: "A=1\n\nX=$(s3cr3t)", expectedLine: 3, expectedKey: "X", contains: "$("},
		{name: "unsupported inside default", src: "X=${X:-$(s3cr3t)}", expectedLine: 1, expectedKey: "X", contains: "$("},
		{name: "key without equals at end of file", src: "A=1\ns3cr3t", expectedLine: 2, contains: "expected = after variable name"},
		{name: "after multi-line value", src: "A=\"1\n2\"\nX=${X:?s3cr3t}", expectedLine: 3, expectedKey: "X", contains: "${X:?"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseDotenv([]byte(tt.src), "test.env", mapLookup(nil))
			var parseErr *dotenvError
			if !errors.As(err, &parseErr) {
				t.Fatalf("got %v, want *dotenvError", err)
			}
			if parseErr.line != tt.expectedLine {
				t.Errorf("line = %d, want %d", parseErr.line, tt.expectedLine)
			}
			if parseErr.key != tt.expectedKey {
				t.Errorf("key = %q, want %q", parseErr.key, tt.expectedKey)
			}
			text := err.Error()
			if !strings.Contains(text, tt.contains) {
				t.Errorf("%q does not contain %q", text, tt.contains)
			}
			if strings.Contains(text, "s3cr3t") {
				t.Errorf("%q leaks the value", text)
			}
		})
	}
}

func TestParseDotenv_UnsupportedExpansionWording(t *testing.T) {
	_, err := parseDotenv([]byte("X=${X:?boom}"), "scripts/test.env", mapLookup(nil))
	expected := "scripts/test.env:1: X: unsupported expansion ${X:?…} — miso expands $VAR, ${VAR} and ${VAR:-default}"
	if err == nil || err.Error() != expected {
		t.Errorf("got %v, want %q", err, expected)
	}
}

func mapLookup(values map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}
}
