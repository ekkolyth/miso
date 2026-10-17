package env

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/go-playground/validator/v10"

	"github.com/ekkolyth/miso/internal/config"
)

// varError is a structured validation error that separates the variable name from the message,
// allowing the formatter to style them independently.
type varError struct {
	name string
	msg  string
}

func (e *varError) Error() string {
	return fmt.Sprintf("%s: %s", e.name, e.msg)
}

// shorthand-only types (pattern and enum need extra config)
var shorthandTypes = map[string]bool{
	"string": true, "port": true, "int": true, "int+": true,
	"float": true, "bool": true, "url": true, "email": true,
	"json": true, "uuid": true,
}

func validateVariables(envMap map[string]string, vars map[string]config.VarConfigOrString, required config.EnvRequired) []error {
	validate := validator.New()

	// Register custom pattern validator for dynamic regex
	_ = validate.RegisterValidation("matches_regex", func(fl validator.FieldLevel) bool {
		pattern := fl.Param()
		if pattern == "" {
			return false
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return false
		}
		return re.MatchString(fl.Field().String())
	})

	names := make([]string, 0, len(vars))
	for name := range vars {
		names = append(names, name)
	}
	sort.Strings(names)

	var errs []error

	for _, name := range names {
		v := vars[name]
		var cfg config.VarConfig
		if v.IsShorthand {
			if v.Type == "pattern" || v.Type == "enum" {
				errs = append(errs, &varError{name: name, msg: fmt.Sprintf("type %s cannot use shorthand (requires pattern/values)", v.Type)})
				continue
			}
			if !shorthandTypes[v.Type] {
				errs = append(errs, &varError{name: name, msg: fmt.Sprintf("unknown type %s", v.Type)})
				continue
			}
			cfg = config.VarConfig{Type: v.Type, Optional: false}
		} else {
			cfg = v.Config
		}

		val, ok := envMap[name]
		if !ok {
			if isRequired(name, required, vars) {
				errs = append(errs, &varError{name: name, msg: "missing required variable"})
			}
			continue
		}

		// Optional variables with empty values are allowed — skip validation.
		if cfg.Optional && val == "" {
			continue
		}

		if err := validateVar(validate, name, val, cfg); err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}

// isRequired returns true if this variable must be present (required config says so AND var is not optional)
func isRequired(name string, required config.EnvRequired, vars map[string]config.VarConfigOrString) bool {
	var inRequiredSet bool
	mode := required.Mode
	if mode == "" && len(required.Keys) == 0 {
		mode = "all" // default when required omitted
	}
	switch mode {
	case "all":
		inRequiredSet = true
	case "none":
		inRequiredSet = false
	default:
		for _, k := range required.Keys {
			if k == name {
				inRequiredSet = true
				break
			}
		}
	}
	if !inRequiredSet {
		return false
	}
	// Check if this var has optional:true
	v, ok := vars[name]
	if !ok {
		return true
	}
	if v.IsShorthand {
		return true // shorthand = required
	}
	return !v.Config.Optional
}

func validateVar(validate *validator.Validate, name, val string, cfg config.VarConfig) error {
	// every message is "expected <constraint>, got <kind>"; the value itself may
	// be a secret, so only its kind and non-revealing detail are reported
	fail := func(expected, got string) error {
		return &varError{name: name, msg: fmt.Sprintf("expected %s, got %s", expected, got)}
	}

	// Types that need custom validation (validator expects specific Go types)
	switch cfg.Type {
	case "port":
		const expected = "port 1-65535"
		p, err := strconv.Atoi(strings.TrimSpace(val))
		if err != nil {
			return fail(expected, valueKind(val))
		}
		if p < 1 {
			return fail(expected, "integer below minimum")
		}
		if p > 65535 {
			return fail(expected, "integer above maximum")
		}
		return nil
	case "int", "int+":
		expected := "integer"
		if cfg.Type == "int+" {
			expected = "positive integer"
		}
		expected += boundsPhrase(cfg)
		n, err := strconv.Atoi(strings.TrimSpace(val))
		if err != nil {
			return fail(expected, valueKind(val))
		}
		if cfg.Type == "int+" && n <= 0 {
			return fail(expected, "integer below minimum")
		}
		if cfg.Min != nil && float64(n) < *cfg.Min {
			return fail(expected, "integer below minimum")
		}
		if cfg.Max != nil && float64(n) > *cfg.Max {
			return fail(expected, "integer above maximum")
		}
		return nil
	case "float":
		expected := "number" + boundsPhrase(cfg)
		f, err := strconv.ParseFloat(strings.TrimSpace(val), 64)
		if err != nil {
			return fail(expected, valueKind(val))
		}
		if cfg.Min != nil && f < *cfg.Min {
			return fail(expected, valueKind(val)+" below minimum")
		}
		if cfg.Max != nil && f > *cfg.Max {
			return fail(expected, valueKind(val)+" above maximum")
		}
		return nil
	case "url":
		u, err := url.Parse(val)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return fail("url", valueKind(val))
		}
		schemes := cfg.Schemes
		if len(schemes) == 0 {
			schemes = []string{"http", "https"}
		}
		for _, s := range schemes {
			if u.Scheme == s {
				return nil
			}
		}
		return fail(fmt.Sprintf("url with scheme one of %v", schemes), "url with a different scheme")
	case "enum":
		trimmed := strings.TrimSpace(val)
		for _, v := range cfg.Values {
			if trimmed == v {
				return nil
			}
		}
		return fail(fmt.Sprintf("one of %v", cfg.Values), valueKind(val))
	case "pattern":
		if cfg.Pattern == "" {
			return &varError{name: name, msg: "pattern type requires pattern"}
		}
		re, err := regexp.Compile(cfg.Pattern)
		if err != nil {
			return &varError{name: name, msg: fmt.Sprintf("invalid pattern: %s", err)}
		}
		if !re.MatchString(val) {
			return fail("match for "+cfg.Pattern, valueKind(val))
		}
		return nil
	case "bool":
		trueVals := cfg.TrueValues
		if len(trueVals) == 0 {
			trueVals = []string{"true", "1", "yes", "on"}
		}
		falseVals := cfg.FalseValues
		if len(falseVals) == 0 {
			falseVals = []string{"false", "0", "no", "off"}
		}
		lowered := strings.TrimSpace(strings.ToLower(val))
		for _, t := range trueVals {
			if lowered == strings.ToLower(t) {
				return nil
			}
		}
		for _, f := range falseVals {
			if lowered == strings.ToLower(f) {
				return nil
			}
		}
		return fail(fmt.Sprintf("boolean (%v or %v)", trueVals, falseVals), valueKind(val))
	}

	// String with pattern: validate directly (matches_regex tag breaks when pattern contains commas)
	if cfg.Type == "string" && cfg.Pattern != "" {
		re, err := regexp.Compile(cfg.Pattern)
		if err != nil {
			return &varError{name: name, msg: fmt.Sprintf("invalid pattern: %s", err)}
		}
		if !re.MatchString(val) {
			return fail("match for "+cfg.Pattern, valueKind(val))
		}
	}

	// Use validator for string, email, json, uuid
	tag := buildValidatorTag(cfg)
	if tag == "" {
		return nil
	}

	err := validate.Var(val, tag)
	if err != nil {
		if ve, ok := err.(validator.ValidationErrors); ok {
			return &varError{name: name, msg: friendlyValidationMsg(cfg.Type, val, ve)}
		}
		return &varError{name: name, msg: err.Error()}
	}
	return nil
}

// " >= min", " <= max", or both joined by "and"
func boundsPhrase(cfg config.VarConfig) string {
	var bounds []string
	if cfg.Min != nil {
		bounds = append(bounds, fmt.Sprintf(">= %v", *cfg.Min))
	}
	if cfg.Max != nil {
		bounds = append(bounds, fmt.Sprintf("<= %v", *cfg.Max))
	}
	if len(bounds) == 0 {
		return ""
	}
	return " " + strings.Join(bounds, " and ")
}

// describes a value without revealing it
func valueKind(val string) string {
	trimmed := strings.TrimSpace(val)
	if trimmed == "" {
		return "empty"
	}
	if _, err := strconv.Atoi(trimmed); err == nil {
		return "integer"
	}
	if _, err := strconv.ParseFloat(trimmed, 64); err == nil {
		return "number"
	}
	if lowered := strings.ToLower(trimmed); lowered == "true" || lowered == "false" {
		return "boolean"
	}
	if u, err := url.Parse(trimmed); err == nil && u.Scheme != "" && u.Host != "" {
		return "url"
	}
	if (trimmed[0] == '{' || trimmed[0] == '[') && json.Valid([]byte(trimmed)) {
		return "json"
	}
	return "string"
}

// converts go-playground/validator errors into "expected X, got Y"
func friendlyValidationMsg(typeName, val string, ve validator.ValidationErrors) string {
	if len(ve) == 0 {
		return "validation failed"
	}
	fe := ve[0]
	switch fe.Tag() {
	case "min":
		return fmt.Sprintf("expected string of at least %s characters, got string of %d characters", fe.Param(), utf8.RuneCountInString(val))
	case "max":
		return fmt.Sprintf("expected string of at most %s characters, got string of %d characters", fe.Param(), utf8.RuneCountInString(val))
	default:
		return fmt.Sprintf("expected %s, got %s", typeName, valueKind(val))
	}
}

func buildValidatorTag(cfg config.VarConfig) string {
	var parts []string

	if !cfg.Optional {
		parts = append(parts, "required")
	}

	switch cfg.Type {
	case "string":
		minLen := 1
		if cfg.Min != nil {
			minLen = int(*cfg.Min)
		}
		parts = append(parts, fmt.Sprintf("min=%d", minLen))
		if cfg.Max != nil {
			parts = append(parts, fmt.Sprintf("max=%d", int(*cfg.Max)))
		}
		// Pattern for string is validated directly in validateVar (avoids comma-in-pattern breakage)
	case "email":
		parts = append(parts, "email")
	case "json":
		parts = append(parts, "json")
	case "uuid":
		parts = append(parts, "uuid")
	}

	return strings.Join(parts, ",")
}
