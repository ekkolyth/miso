package env

import (
	"testing"

	"github.com/go-playground/validator/v10"

	"github.com/ekkolyth/miso/internal/config"
)

// fakeValue must produce values that pass the real validators — that's the whole
// point of --example (fill an env that satisfies `miso env`).
func TestFakeValue_PassesValidation(t *testing.T) {
	validate := validator.New()
	cases := []config.VarConfig{
		{Type: "port"},
		{Type: "int"},
		{Type: "int+"},
		{Type: "float"},
		{Type: "bool"},
		{Type: "email"},
		{Type: "uuid"},
		{Type: "json"},
		{Type: "string"},
		{Type: "url"},
		{Type: "url", Schemes: []string{"redis", "rediss"}},
		{Type: "enum", Values: []string{"dev", "prod"}},
		{Type: "pattern", Pattern: "^postgres(s)?(ql)?://"},
		{Type: "string", Pattern: "^redis"},
	}
	for _, cfg := range cases {
		val := fakeValue(cfg)
		if err := validateVar(validate, "VAR", val, cfg); err != nil {
			t.Errorf("fakeValue(%+v) = %q, fails validation: %v", cfg, val, err)
		}
	}
}

func TestFakeValue_Scalars(t *testing.T) {
	cases := map[string]string{
		"port":  "3000",
		"email": "user@example.com",
		"uuid":  "00000000-0000-0000-0000-000000000000",
		"json":  "{}",
	}
	for typ, want := range cases {
		if got := fakeValue(config.VarConfig{Type: typ}); got != want {
			t.Errorf("fakeValue(%s) = %q, want %q", typ, got, want)
		}
	}
}

func TestFakeValue_EnumFirstValue(t *testing.T) {
	got := fakeValue(config.VarConfig{Type: "enum", Values: []string{"debug", "info"}})
	if got != "debug" {
		t.Errorf("enum fake = %q, want first value debug", got)
	}
}

func TestResolveVarConfig(t *testing.T) {
	if got := resolveVarConfig(config.VarConfigOrString{IsShorthand: true, Type: "port"}); got.Type != "port" {
		t.Errorf("shorthand resolve = %+v, want Type port", got)
	}
	full := config.VarConfig{Type: "url", Schemes: []string{"redis"}}
	if got := resolveVarConfig(config.VarConfigOrString{Config: full}); got.Type != "url" || len(got.Schemes) != 1 {
		t.Errorf("object resolve = %+v, want the VarConfig", got)
	}
}
