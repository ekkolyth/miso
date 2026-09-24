package env

import (
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"

	"github.com/ekkolyth/miso/internal/config"
	"github.com/ekkolyth/miso/internal/testutil"
)

func TestValidateVariables_CollectsAllErrors(t *testing.T) {
	vars := map[string]config.VarConfigOrString{
		"APP_SECRET": {IsShorthand: true, Type: "string"},
		"DB_PORT":    {IsShorthand: true, Type: "port"},
		"API_URL":    {IsShorthand: true, Type: "url"},
	}
	required := config.EnvRequired{Mode: "all"}

	// envMap missing APP_SECRET, has invalid port and invalid url
	envMap := map[string]string{
		"DB_PORT": "not-a-number",
		"API_URL": "not-a-url",
	}

	errs := validateVariables(envMap, vars, required)
	if len(errs) != 3 {
		t.Fatalf("got %d errors, want 3: %v", len(errs), errs)
	}
}

func TestValidateVariables_NoErrors(t *testing.T) {
	vars := map[string]config.VarConfigOrString{
		"PORT": {IsShorthand: true, Type: "port"},
	}
	required := config.EnvRequired{Mode: "all"}
	envMap := map[string]string{"PORT": "8080"}

	errs := validateVariables(envMap, vars, required)
	if len(errs) != 0 {
		t.Fatalf("got %d errors, want 0: %v", len(errs), errs)
	}
}

func TestValidateVariables_MixedMissingAndInvalid(t *testing.T) {
	vars := map[string]config.VarConfigOrString{
		"REQUIRED_VAR": {IsShorthand: true, Type: "string"},
		"BAD_PORT":     {IsShorthand: true, Type: "port"},
	}
	required := config.EnvRequired{Mode: "all"}
	envMap := map[string]string{
		"BAD_PORT": "99999",
	}

	errs := validateVariables(envMap, vars, required)
	if len(errs) != 2 {
		t.Fatalf("got %d errors, want 2: %v", len(errs), errs)
	}
}

func TestValidateVariables_VarErrorType(t *testing.T) {
	vars := map[string]config.VarConfigOrString{
		"MY_VAR": {IsShorthand: true, Type: "port"},
	}
	required := config.EnvRequired{Mode: "all"}
	envMap := map[string]string{"MY_VAR": "abc"}

	errs := validateVariables(envMap, vars, required)
	if len(errs) != 1 {
		t.Fatalf("got %d errors, want 1: %v", len(errs), errs)
	}
	ve, ok := errs[0].(*varError)
	if !ok {
		t.Fatalf("expected *varError, got %T", errs[0])
	}
	if ve.name != "MY_VAR" {
		t.Errorf("varError.name = %q, want %q", ve.name, "MY_VAR")
	}
	if ve.msg != "expected port 1-65535, got string" {
		t.Errorf("varError.msg = %q, want %q", ve.msg, "expected port 1-65535, got string")
	}
}

func TestFriendlyValidationMsg_String(t *testing.T) {
	vars := map[string]config.VarConfigOrString{
		"MY_STRING": {IsShorthand: true, Type: "string"},
	}
	required := config.EnvRequired{Mode: "all"}
	envMap := map[string]string{"MY_STRING": ""}

	errs := validateVariables(envMap, vars, required)
	if len(errs) != 1 {
		t.Fatalf("got %d errors, want 1: %v", len(errs), errs)
	}
	ve, ok := errs[0].(*varError)
	if !ok {
		t.Fatalf("expected *varError, got %T", errs[0])
	}
	if ve.msg != "expected string, got empty" {
		t.Errorf("varError.msg = %q, want %q", ve.msg, "expected string, got empty")
	}
}

func floatPtr(f float64) *float64 { return &f }

func TestValidateVar_Types(t *testing.T) {
	validate := validator.New()
	tests := []struct {
		name     string
		val      string
		cfg      config.VarConfig
		expected string // "" = expect pass
	}{
		{"port ok", "8080", config.VarConfig{Type: "port"}, ""},
		{"port above range", "99999", config.VarConfig{Type: "port"}, "expected port 1-65535, got integer above maximum"},
		{"port below range", "0", config.VarConfig{Type: "port"}, "expected port 1-65535, got integer below minimum"},
		{"port not numeric", "abc", config.VarConfig{Type: "port"}, "expected port 1-65535, got string"},
		{"int ok", "42", config.VarConfig{Type: "int"}, ""},
		{"int rejects float", "3.14", config.VarConfig{Type: "int"}, "expected integer, got number"},
		{"int+ rejects negative", "-5", config.VarConfig{Type: "int+"}, "expected positive integer, got integer below minimum"},
		{"int+ rejects zero", "0", config.VarConfig{Type: "int+"}, "expected positive integer, got integer below minimum"},
		{"int below min", "3", config.VarConfig{Type: "int", Min: floatPtr(5)}, "expected integer >= 5, got integer below minimum"},
		{"int above max", "30", config.VarConfig{Type: "int", Min: floatPtr(5), Max: floatPtr(10)}, "expected integer >= 5 and <= 10, got integer above maximum"},
		{"float rejects word", "fast", config.VarConfig{Type: "float"}, "expected number, got string"},
		{"float above max", "1.5", config.VarConfig{Type: "float", Max: floatPtr(1)}, "expected number <= 1, got number above maximum"},
		{"float below min", "-1", config.VarConfig{Type: "float", Min: floatPtr(0)}, "expected number >= 0, got integer below minimum"},
		{"url parse failure", "::nope", config.VarConfig{Type: "url"}, "expected url, got string"},
		{"url without host", "localhost", config.VarConfig{Type: "url"}, "expected url, got string"},
		{"url disallowed scheme", "https://x.com", config.VarConfig{Type: "url", Schemes: []string{"redis", "rediss"}}, "expected url with scheme one of [redis rediss], got url with a different scheme"},
		{"url ok scheme", "redis://localhost:6379", config.VarConfig{Type: "url", Schemes: []string{"redis", "rediss"}}, ""},
		{"enum rejects", "staging", config.VarConfig{Type: "enum", Values: []string{"a", "b", "c"}}, "expected one of [a b c], got string"},
		{"enum ok", "b", config.VarConfig{Type: "enum", Values: []string{"a", "b", "c"}}, ""},
		{"pattern rejects", "not-semver", config.VarConfig{Type: "pattern", Pattern: `^v?\d+\.\d+\.\d+$`}, `expected match for ^v?\d+\.\d+\.\d+$, got string`},
		{"pattern rejects shell default", "${TEST_DATABASE_URL:-postgres://x}", config.VarConfig{Type: "pattern", Pattern: "^postgres"}, "expected match for ^postgres, got string"},
		{"pattern ok", "v1.2.3", config.VarConfig{Type: "pattern", Pattern: `^v?\d+\.\d+\.\d+$`}, ""},
		{"string pattern rejects", "https://x", config.VarConfig{Type: "string", Pattern: "^redis"}, "expected match for ^redis, got url"},
		{"bool rejects", "maybe", config.VarConfig{Type: "bool"}, "expected boolean ([true 1 yes on] or [false 0 no off]), got string"},
		{"bool ok", "yes", config.VarConfig{Type: "bool"}, ""},
		{"email rejects", "not-an-email", config.VarConfig{Type: "email"}, "expected email, got string"},
		{"email ok", "a@b.com", config.VarConfig{Type: "email"}, ""},
		{"json rejects", "{bad", config.VarConfig{Type: "json"}, "expected json, got string"},
		{"json ok", `{"a":1}`, config.VarConfig{Type: "json"}, ""},
		{"uuid rejects", "not-a-uuid", config.VarConfig{Type: "uuid"}, "expected uuid, got string"},
		{"string min", "hi", config.VarConfig{Type: "string", Min: floatPtr(5)}, "expected string of at least 5 characters, got string of 2 characters"},
		{"string max", "toolong", config.VarConfig{Type: "string", Max: floatPtr(3)}, "expected string of at most 3 characters, got string of 7 characters"},
		{"required empty", "", config.VarConfig{Type: "string"}, "expected string, got empty"},
		{"required empty email", "", config.VarConfig{Type: "email"}, "expected email, got empty"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateVar(validate, "VAR", tt.val, tt.cfg)
			if tt.expected == "" {
				testutil.NoError(t, err)
				return
			}
			ve, ok := err.(*varError)
			if !ok {
				t.Fatalf("got %v (%T), want *varError", err, err)
			}
			if ve.msg != tt.expected {
				t.Errorf("msg = %q, want %q", ve.msg, tt.expected)
			}
			if tt.val != "" && strings.Contains(ve.msg, tt.val) {
				t.Errorf("msg %q leaks the value %q", ve.msg, tt.val)
			}
		})
	}
}

func TestValueKind(t *testing.T) {
	tests := []struct {
		val      string
		expected string
	}{
		{"", "empty"},
		{"  ", "empty"},
		{" 42 ", "integer"},
		{"3.14", "number"},
		{"TRUE", "boolean"},
		{"https://x.com/a", "url"},
		{`{"a":1}`, "json"},
		{"[1,2]", "json"},
		{"hello", "string"},
		{"localhost:5432", "string"},
	}
	for _, tt := range tests {
		if got := valueKind(tt.val); got != tt.expected {
			t.Errorf("valueKind(%q) = %q, want %q", tt.val, got, tt.expected)
		}
	}
}

func TestValidateVariables_RequiredModes(t *testing.T) {
	vars := map[string]config.VarConfigOrString{
		"NEED": {IsShorthand: true, Type: "string"},
	}
	// mode "none": absent var passes
	if errs := validateVariables(map[string]string{}, vars, config.EnvRequired{Mode: "none"}); len(errs) != 0 {
		t.Fatalf("mode none: want 0 errors, got %v", errs)
	}
	// mode "" + Keys: listed key missing is an error
	errs := validateVariables(map[string]string{}, vars, config.EnvRequired{Keys: []string{"NEED"}})
	if len(errs) != 1 {
		t.Fatalf("keys: want 1 error, got %v", errs)
	}
	testutil.ErrorContains(t, errs[0], "missing required variable")
}
