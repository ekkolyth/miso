package env

import (
	"strings"
	"testing"

	"github.com/ekkolyth/miso/internal/config"
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
