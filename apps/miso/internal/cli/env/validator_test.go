package env

import (
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
