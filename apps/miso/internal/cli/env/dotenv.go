package env

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"unicode"
)

// the grammar mirrors godotenv v1.5.1; expansion follows the shell's $VAR,
// ${VAR} and ${VAR:-default}, and every other shell form is a load error so a
// sourced bash fragment can't be read as literal text

const supportedExpansions = "miso expands $VAR, ${VAR} and ${VAR:-default}"

// never carries a value: the line is the key's, msg names only the form
type dotenvError struct {
	source string
	line   int
	key    string
	msg    string
}

func (e *dotenvError) Error() string {
	if e.key == "" {
		return fmt.Sprintf("%s:%d: %s", e.source, e.line, e.msg)
	}
	return fmt.Sprintf("%s:%d: %s: %s", e.source, e.line, e.key, e.msg)
}

func readDotenvFile(path string) (map[string]string, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseDotenv(src, path, os.LookupEnv)
}

// references resolve against lookup first, then keys defined earlier in src
func parseDotenv(src []byte, source string, lookup func(string) (string, bool)) (map[string]string, error) {
	p := &dotenvParser{
		src:    bytes.ReplaceAll(src, []byte("\r\n"), []byte("\n")),
		source: source,
		lookup: lookup,
		vars:   make(map[string]string),
	}
	for {
		p.skipBlankAndComments()
		if p.pos >= len(p.src) {
			return p.vars, nil
		}
		line := p.lineAt(p.pos)
		key, err := p.readKey(line)
		if err != nil {
			return nil, err
		}
		value, err := p.readValue(key, line)
		if err != nil {
			return nil, err
		}
		p.vars[key] = value
	}
}

type dotenvParser struct {
	src    []byte
	pos    int
	source string
	lookup func(string) (string, bool)
	vars   map[string]string
}

func (p *dotenvParser) lineAt(pos int) int {
	return 1 + bytes.Count(p.src[:pos], []byte("\n"))
}

func (p *dotenvParser) fail(line int, key, msg string) error {
	return &dotenvError{source: p.source, line: line, key: key, msg: msg}
}

func (p *dotenvParser) skipBlankAndComments() {
	for p.pos < len(p.src) {
		rest := p.src[p.pos:]
		offset := bytes.IndexFunc(rest, func(r rune) bool { return !unicode.IsSpace(r) })
		if offset == -1 {
			p.pos = len(p.src)
			return
		}
		p.pos += offset
		if p.src[p.pos] != '#' {
			return
		}
		newline := bytes.IndexByte(p.src[p.pos:], '\n')
		if newline == -1 {
			p.pos = len(p.src)
			return
		}
		p.pos += newline
	}
}

func (p *dotenvParser) readKey(line int) (string, error) {
	rest := p.src[p.pos:]
	if trimmed, ok := bytes.CutPrefix(rest, []byte("export")); ok && len(trimmed) > 0 && isInlineSpace(rune(trimmed[0])) {
		rest = bytes.TrimLeftFunc(trimmed, isInlineSpace)
	}
	start := len(p.src) - len(rest)
	for i, char := range rest {
		switch {
		case isInlineSpace(rune(char)):
		case char == '=' || char == ':':
			p.pos = start + i + 1
			return strings.TrimRightFunc(string(rest[:i]), unicode.IsSpace), nil
		case char == '_' || char == '.' || unicode.IsLetter(rune(char)) || unicode.IsNumber(rune(char)):
		default:
			return "", p.fail(line, "", fmt.Sprintf("unexpected character %q in variable name", string(char)))
		}
	}
	// no key: without an = the line may be a stray value
	return "", p.fail(line, "", "expected = after variable name")
}

func (p *dotenvParser) readValue(key string, line int) (string, error) {
	for p.pos < len(p.src) && isInlineSpace(rune(p.src[p.pos])) {
		p.pos++
	}
	if p.pos >= len(p.src) {
		return "", nil
	}

	quote := p.src[p.pos]
	if quote != '"' && quote != '\'' {
		end := bytes.IndexAny(p.src[p.pos:], "\n\r")
		if end == -1 {
			end = len(p.src) - p.pos
		}
		raw := []rune(string(p.src[p.pos : p.pos+end]))
		p.pos += end
		// the last " #" starts a comment
		for i := len(raw) - 1; i > 0; i-- {
			if raw[i] == '#' && isInlineSpace(raw[i-1]) {
				raw = raw[:i]
				break
			}
		}
		value := strings.TrimFunc(string(raw), isInlineSpace)
		return p.expand(value, false, key, line)
	}

	for i := p.pos + 1; i < len(p.src); i++ {
		if p.src[i] != quote || p.src[i-1] == '\\' {
			continue
		}
		isQuote := func(r rune) bool { return r == rune(quote) }
		// godotenv trims runs of the quote, so "a\"" loses its escaped quote too
		raw := string(bytes.TrimLeftFunc(bytes.TrimRightFunc(p.src[p.pos:i], isQuote), isQuote))
		p.pos = i + 1
		if quote == '\'' {
			return raw, nil
		}
		return p.expand(raw, true, key, line)
	}
	return "", p.fail(line, key, "unterminated quoted value")
}

// double-quoted values also unescape \n, \r and \X; unquoted ones only \$
func (p *dotenvParser) expand(s string, doubleQuoted bool, key string, line int) (string, error) {
	var out strings.Builder
	for i := 0; i < len(s); {
		switch s[i] {
		case '\\':
			if i+1 >= len(s) {
				out.WriteByte('\\')
				i++
				continue
			}
			next := s[i+1]
			switch {
			case doubleQuoted && next == 'n':
				out.WriteByte('\n')
			case doubleQuoted && next == 'r':
				out.WriteByte('\r')
			case doubleQuoted || next == '$':
				out.WriteByte(next)
			default:
				out.WriteByte('\\')
				i++
				continue
			}
			i += 2
		case '$':
			value, consumed, err := p.reference(s[i:], doubleQuoted, key, line)
			if err != nil {
				return "", err
			}
			out.WriteString(value)
			i += consumed
		default:
			out.WriteByte(s[i])
			i++
		}
	}
	return out.String(), nil
}

// s starts at a '$'; a '$' that doesn't start a reference stays literal
func (p *dotenvParser) reference(s string, doubleQuoted bool, key string, line int) (string, int, error) {
	if len(s) < 2 {
		return "$", 1, nil
	}
	switch {
	case s[1] == '$':
		return "$$", 2, nil
	case s[1] == '(':
		return "", 0, p.unsupported(line, key, "$(…)")
	case isNameStart(s[1]):
		end := nameEnd(s, 1)
		return p.resolve(s[1:end]), end, nil
	case s[1] != '{':
		return "$", 1, nil
	}

	end := nameEnd(s, 2)
	name := s[2:end]
	if end >= len(s) {
		return "", 0, p.fail(line, key, "unterminated ${ expansion")
	}
	if name == "" {
		if s[end] == '}' {
			return "", 0, p.unsupported(line, key, "${}")
		}
		return "", 0, p.unsupported(line, key, "${"+string(s[end])+"…}")
	}
	if s[end] == '}' {
		return p.resolve(name), end + 1, nil
	}

	operator := s[end : end+1]
	if operator == ":" && end+1 < len(s) && strings.IndexByte("-=?+", s[end+1]) != -1 {
		operator = s[end : end+2]
	}
	if operator != ":-" {
		return "", 0, p.unsupported(line, key, "${"+name+operator+"…}")
	}

	closing := matchingBrace(s, end+2)
	if closing == -1 {
		return "", 0, p.fail(line, key, "unterminated ${ expansion")
	}
	// expanded even when unused, so an unsupported form fails regardless of env
	fallback, err := p.expand(s[end+2:closing], doubleQuoted, key, line)
	if err != nil {
		return "", 0, err
	}
	if value := p.resolve(name); value != "" {
		return value, closing + 1, nil
	}
	return fallback, closing + 1, nil
}

func (p *dotenvParser) unsupported(line int, key, form string) error {
	return p.fail(line, key, "unsupported expansion "+form+" — "+supportedExpansions)
}

func (p *dotenvParser) resolve(name string) string {
	if value, ok := p.lookup(name); ok {
		return value
	}
	return p.vars[name]
}

// index of the '}' closing a ${ opened before from, or -1
func matchingBrace(s string, from int) int {
	depth := 1
	for i := from; i < len(s); i++ {
		switch {
		case s[i] == '\\':
			i++
		case s[i] == '$' && i+1 < len(s) && s[i+1] == '{':
			depth++
			i++
		case s[i] == '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func nameEnd(s string, from int) int {
	if from >= len(s) || !isNameStart(s[from]) {
		return from
	}
	i := from + 1
	for i < len(s) && (isNameStart(s[i]) || (s[i] >= '0' && s[i] <= '9')) {
		i++
	}
	return i
}

func isNameStart(char byte) bool {
	return char == '_' || (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z')
}

// unlike unicode.IsSpace, a line break is not inline space
func isInlineSpace(r rune) bool {
	switch r {
	case '\t', '\v', '\f', '\r', ' ', 0x85, 0xA0:
		return true
	}
	return false
}
