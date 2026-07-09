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
	if !strings.Contains(ve.msg, "expected port") {
		t.Errorf("varError.msg = %q, want to contain 'expected port'", ve.msg)
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
	if !strings.Contains(ve.msg, "expected string") {
		t.Errorf("expected friendly message with 'expected string', got: %q", ve.msg)
	}
}

func floatPtr(f float64) *float64 { return &f }

func TestValidateVar_Types(t *testing.T) {
	validate := validator.New()
	tests := []struct {
		name    string
		val     string
		cfg     config.VarConfig
		wantErr string // "" = expect pass
	}{
		{"int ok", "42", config.VarConfig{Type: "int"}, ""},
		{"int rejects float", "3.14", config.VarConfig{Type: "int"}, "expected integer"},
		{"int+ rejects negative", "-5", config.VarConfig{Type: "int+"}, "must be positive integer"},
		{"float above max", "1.5", config.VarConfig{Type: "float", Max: floatPtr(1)}, "must be <="},
		{"float below min", "-1", config.VarConfig{Type: "float", Min: floatPtr(0)}, "must be >="},
		{"url disallowed scheme", "https://x.com", config.VarConfig{Type: "url", Schemes: []string{"redis", "rediss"}}, "url scheme must be one of"},
		{"url ok scheme", "redis://localhost:6379", config.VarConfig{Type: "url", Schemes: []string{"redis", "rediss"}}, ""},
		{"enum rejects", "d", config.VarConfig{Type: "enum", Values: []string{"a", "b", "c"}}, "must be one of"},
		{"enum ok", "b", config.VarConfig{Type: "enum", Values: []string{"a", "b", "c"}}, ""},
		{"pattern rejects", "not-semver", config.VarConfig{Type: "pattern", Pattern: `^v?\d+\.\d+\.\d+$`}, "does not match pattern"},
		{"pattern ok", "v1.2.3", config.VarConfig{Type: "pattern", Pattern: `^v?\d+\.\d+\.\d+$`}, ""},
		{"bool rejects", "maybe", config.VarConfig{Type: "bool"}, "invalid bool"},
		{"bool ok", "yes", config.VarConfig{Type: "bool"}, ""},
		{"email rejects", "not-an-email", config.VarConfig{Type: "email"}, "expected email"},
		{"email ok", "a@b.com", config.VarConfig{Type: "email"}, ""},
		{"json rejects", "{bad", config.VarConfig{Type: "json"}, "expected json"},
		{"json ok", `{"a":1}`, config.VarConfig{Type: "json"}, ""},
		{"uuid rejects", "not-a-uuid", config.VarConfig{Type: "uuid"}, "expected uuid"},
		{"string min", "hi", config.VarConfig{Type: "string", Min: floatPtr(5)}, "at least 5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateVar(validate, "VAR", tt.val, tt.cfg)
			if tt.wantErr == "" {
				testutil.NoError(t, err)
			} else {
				testutil.ErrorContains(t, err, tt.wantErr)
			}
		})
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
