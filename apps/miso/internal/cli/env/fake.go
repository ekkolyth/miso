package env

import (
	"strconv"
	"strings"

	"github.com/ekkolyth/miso/internal/config"
)

// fakeValue returns a type-valid placeholder for a variable so a file filled
// with these passes `miso env` validation — useful as an example env and to
// rule env out as the cause of a crash. Best-effort for custom regex patterns:
// an exotic pattern may still need a hand-written value.
func fakeValue(cfg config.VarConfig) string {
	switch cfg.Type {
	case "port":
		return "3000"
	case "int", "int+":
		if cfg.Min != nil && *cfg.Min >= 1 {
			return strconv.Itoa(int(*cfg.Min))
		}
		return "1"
	case "float":
		if cfg.Min != nil {
			return strconv.FormatFloat(*cfg.Min, 'f', -1, 64)
		}
		return "1"
	case "bool":
		if len(cfg.TrueValues) > 0 {
			return cfg.TrueValues[0]
		}
		return "true"
	case "email":
		return "user@example.com"
	case "uuid":
		return "00000000-0000-0000-0000-000000000000"
	case "json":
		return "{}"
	case "enum":
		if len(cfg.Values) > 0 {
			return cfg.Values[0]
		}
		return "changeme"
	case "url":
		return fakeURL(cfg)
	case "pattern":
		return fakePattern(cfg.Pattern)
	case "string":
		if cfg.Pattern != "" {
			return fakePattern(cfg.Pattern)
		}
		return "changeme"
	default:
		return "changeme"
	}
}

// resolveVarConfig normalises the string-or-object variable form to a VarConfig.
func resolveVarConfig(vc config.VarConfigOrString) config.VarConfig {
	if vc.IsShorthand {
		return config.VarConfig{Type: vc.Type}
	}
	return vc.Config
}

func fakeURL(cfg config.VarConfig) string {
	scheme := "https"
	if len(cfg.Schemes) > 0 {
		scheme = cfg.Schemes[0]
	}
	switch scheme {
	case "postgres", "postgresql":
		return "postgresql://user:pass@localhost:5432/db"
	case "redis", "rediss":
		return scheme + "://localhost:6379"
	case "http", "https":
		return scheme + "://example.com"
	default:
		return scheme + "://localhost"
	}
}

// fakePattern makes a best-effort value satisfying a regex, handling the common
// connection-string prefixes and anchored literals; falls back to a placeholder.
func fakePattern(pattern string) string {
	lower := strings.ToLower(pattern)
	switch {
	case strings.Contains(lower, "postgres"):
		return "postgresql://user:pass@localhost:5432/db"
	case strings.Contains(lower, "redis"):
		return "redis://localhost:6379"
	}
	if head := patternLiteralPrefix(pattern); head != "" {
		return head + "example"
	}
	return "changeme"
}

// patternLiteralPrefix returns the leading literal characters of a regex (after
// a `^` anchor) up to the first metacharacter.
func patternLiteralPrefix(pattern string) string {
	var out strings.Builder
	for _, char := range strings.TrimPrefix(pattern, "^") {
		if strings.ContainsRune(`.*+?()[]{}|\^$`, char) {
			break
		}
		out.WriteRune(char)
	}
	return out.String()
}
