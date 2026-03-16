package env

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

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
	validate.RegisterValidation("matches_regex", func(fl validator.FieldLevel) bool {
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
	// Types that need custom validation (validator expects specific Go types)
	switch cfg.Type {
	case "port":
		p, err := strconv.Atoi(strings.TrimSpace(val))
		if err != nil {
			return &varError{name: name, msg: fmt.Sprintf("expected port, found '%s'", val)}
		}
		if p < 1 || p > 65535 {
			return &varError{name: name, msg: fmt.Sprintf("port must be 1-65535, got %d", p)}
		}
		return nil
	case "int", "int+":
		n, err := strconv.Atoi(strings.TrimSpace(val))
		if err != nil {
			return &varError{name: name, msg: fmt.Sprintf("expected integer, found '%s'", val)}
		}
		if cfg.Type == "int+" && n <= 0 {
			return &varError{name: name, msg: "must be positive integer"}
		}
		if cfg.Min != nil && float64(n) < *cfg.Min {
			return &varError{name: name, msg: fmt.Sprintf("must be >= %v", *cfg.Min)}
		}
		if cfg.Max != nil && float64(n) > *cfg.Max {
			return &varError{name: name, msg: fmt.Sprintf("must be <= %v", *cfg.Max)}
		}
		return nil
	case "float":
		f, err := strconv.ParseFloat(strings.TrimSpace(val), 64)
		if err != nil {
			return &varError{name: name, msg: fmt.Sprintf("expected float, found '%s'", val)}
		}
		if cfg.Min != nil && f < *cfg.Min {
			return &varError{name: name, msg: fmt.Sprintf("must be >= %v", *cfg.Min)}
		}
		if cfg.Max != nil && f > *cfg.Max {
			return &varError{name: name, msg: fmt.Sprintf("must be <= %v", *cfg.Max)}
		}
		return nil
	case "url":
		u, err := url.Parse(val)
		if err != nil {
			return &varError{name: name, msg: fmt.Sprintf("expected url, found '%s'", val)}
		}
		if u.Scheme == "" || u.Host == "" {
			return &varError{name: name, msg: fmt.Sprintf("expected url, found '%s'", val)}
		}
		schemes := cfg.Schemes
		if len(schemes) == 0 {
			schemes = []string{"http", "https"}
		}
		allowed := false
		for _, s := range schemes {
			if u.Scheme == s {
				allowed = true
				break
			}
		}
		if !allowed {
			return &varError{name: name, msg: fmt.Sprintf("url scheme must be one of %v", schemes)}
		}
		return nil
	case "enum":
		val = strings.TrimSpace(val)
		for _, v := range cfg.Values {
			if val == v {
				return nil
			}
		}
		return &varError{name: name, msg: fmt.Sprintf("must be one of %v", cfg.Values)}
	case "pattern":
		if cfg.Pattern == "" {
			return &varError{name: name, msg: "pattern type requires pattern"}
		}
		re, err := regexp.Compile(cfg.Pattern)
		if err != nil {
			return &varError{name: name, msg: fmt.Sprintf("invalid pattern: %s", err)}
		}
		if !re.MatchString(val) {
			return &varError{name: name, msg: "value does not match pattern"}
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
		val = strings.TrimSpace(strings.ToLower(val))
		for _, t := range trueVals {
			if val == strings.ToLower(t) {
				return nil
			}
		}
		for _, f := range falseVals {
			if val == strings.ToLower(f) {
				return nil
			}
		}
		return &varError{name: name, msg: fmt.Sprintf("invalid bool (use %v or %v)", trueVals, falseVals)}
	}

	// String with pattern: validate directly (matches_regex tag breaks when pattern contains commas)
	if cfg.Type == "string" && cfg.Pattern != "" {
		re, err := regexp.Compile(cfg.Pattern)
		if err != nil {
			return &varError{name: name, msg: fmt.Sprintf("invalid pattern: %s", err)}
		}
		if !re.MatchString(val) {
			return &varError{name: name, msg: "value does not match pattern"}
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

// friendlyValidationMsg converts go-playground/validator errors into human-readable messages.
func friendlyValidationMsg(typeName, val string, ve validator.ValidationErrors) string {
	if len(ve) == 0 {
		return "validation failed"
	}
	fe := ve[0]
	switch fe.Tag() {
	case "required":
		return fmt.Sprintf("expected %s, found ''", typeName)
	case "min":
		return fmt.Sprintf("must be at least %s characters, found '%s'", fe.Param(), val)
	case "max":
		return fmt.Sprintf("must be at most %s characters, found '%s'", fe.Param(), val)
	case "email":
		return fmt.Sprintf("expected email, found '%s'", val)
	case "json":
		return fmt.Sprintf("expected json, found '%s'", val)
	case "uuid":
		return fmt.Sprintf("expected uuid, found '%s'", val)
	default:
		return fmt.Sprintf("expected %s, found '%s'", typeName, val)
	}
}

func buildValidatorTag(cfg config.VarConfig) string {
	var parts []string

	if !cfg.Optional {
		parts = append(parts, "required")
	}

	switch cfg.Type {
	case "string":
		min := 1
		if cfg.Min != nil {
			min = int(*cfg.Min)
		}
		parts = append(parts, fmt.Sprintf("min=%d", min))
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
