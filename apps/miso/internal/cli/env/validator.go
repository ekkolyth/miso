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
				errs = append(errs, fmt.Errorf("variable %s: type %s cannot use shorthand (requires pattern/values)", name, v.Type))
				continue
			}
			if !shorthandTypes[v.Type] {
				errs = append(errs, fmt.Errorf("variable %s: unknown type %s", name, v.Type))
				continue
			}
			cfg = config.VarConfig{Type: v.Type, Optional: false}
		} else {
			cfg = v.Config
		}

		val, ok := envMap[name]
		if !ok {
			if isRequired(name, required, vars) {
				errs = append(errs, fmt.Errorf("missing required variable: %s", name))
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
			return fmt.Errorf("variable %s: invalid port: %w", name, err)
		}
		if p < 1 || p > 65535 {
			return fmt.Errorf("variable %s: port must be 1-65535, got %d", name, p)
		}
		return nil
	case "int", "int+":
		n, err := strconv.Atoi(strings.TrimSpace(val))
		if err != nil {
			return fmt.Errorf("variable %s: invalid integer: %w", name, err)
		}
		if cfg.Type == "int+" && n <= 0 {
			return fmt.Errorf("variable %s: must be positive integer", name)
		}
		if cfg.Min != nil && float64(n) < *cfg.Min {
			return fmt.Errorf("variable %s: must be >= %v", name, *cfg.Min)
		}
		if cfg.Max != nil && float64(n) > *cfg.Max {
			return fmt.Errorf("variable %s: must be <= %v", name, *cfg.Max)
		}
		return nil
	case "float":
		f, err := strconv.ParseFloat(strings.TrimSpace(val), 64)
		if err != nil {
			return fmt.Errorf("variable %s: invalid float: %w", name, err)
		}
		if cfg.Min != nil && f < *cfg.Min {
			return fmt.Errorf("variable %s: must be >= %v", name, *cfg.Min)
		}
		if cfg.Max != nil && f > *cfg.Max {
			return fmt.Errorf("variable %s: must be <= %v", name, *cfg.Max)
		}
		return nil
	case "url":
		u, err := url.Parse(val)
		if err != nil {
			return fmt.Errorf("variable %s: invalid url: %w", name, err)
		}
		if u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("variable %s: invalid url", name)
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
			return fmt.Errorf("variable %s: url scheme must be one of %v", name, schemes)
		}
		return nil
	case "enum":
		val = strings.TrimSpace(val)
		for _, v := range cfg.Values {
			if val == v {
				return nil
			}
		}
		return fmt.Errorf("variable %s: must be one of %v", name, cfg.Values)
	case "pattern":
		if cfg.Pattern == "" {
			return fmt.Errorf("variable %s: pattern type requires pattern", name)
		}
		re, err := regexp.Compile(cfg.Pattern)
		if err != nil {
			return fmt.Errorf("variable %s: invalid pattern: %w", name, err)
		}
		if !re.MatchString(val) {
			return fmt.Errorf("variable %s: value does not match pattern", name)
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
		return fmt.Errorf("variable %s: invalid bool (use %v or %v)", name, trueVals, falseVals)
	}

	// String with pattern: validate directly (matches_regex tag breaks when pattern contains commas)
	if cfg.Type == "string" && cfg.Pattern != "" {
		re, err := regexp.Compile(cfg.Pattern)
		if err != nil {
			return fmt.Errorf("variable %s: invalid pattern: %w", name, err)
		}
		if !re.MatchString(val) {
			return fmt.Errorf("variable %s: value does not match pattern", name)
		}
	}

	// Use validator for string, email, json, uuid
	tag := buildValidatorTag(cfg)
	if tag == "" {
		return nil
	}

	err := validate.Var(val, tag)
	if err != nil {
		return fmt.Errorf("variable %s: %w", name, err)
	}
	return nil
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
