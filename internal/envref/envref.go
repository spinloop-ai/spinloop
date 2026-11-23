// Package envref resolves environment references in a value: `$NAME`,
// `${NAME}` and `${NAME:-default}`, with `$$` for a literal `$`. It is the
// one grammar shared by every place a value is read — Spinloop PARAM lines
// and preset entry values — so the two spell the same language.
//
// The grammar follows the shell and Docker Compose conventions:
//
//   - `$NAME` and `${NAME}` expand to the variable's value; unset or empty
//     expands to the empty string.
//   - `${NAME:-default}` expands to the variable's value unless it is unset
//     or empty, in which case it expands to default (which may itself be
//     empty).
//   - `$$` renders a literal `$`.
//   - A `$` that begins none of these forms — followed by a character that
//     cannot start a name, or ending the value — stays literal, so values
//     that merely contain a dollar sign are untouched.
//
// A default is literal: a `$` inside `${NAME:-...}` is text, not a nested
// reference, and is never rescanned; the first `}` closes the reference.
//
// A `$` beginning a malformed reference (an unterminated `${`, an invalid or
// empty name, or any other `${...}` form such as `${NAME:+x}`) is an error:
// the syntax does not define it, and quietly passing it to an engine that
// rejects it cryptically is worse than naming it here.
package envref

import (
	"fmt"
	"os"
	"strings"
)

// Expand resolves every reference in value against the process environment.
// A value with no references is returned unchanged.
func Expand(value string) (string, error) {
	if !strings.Contains(value, "$") {
		return value, nil
	}
	var b strings.Builder
	b.Grow(len(value))
	for i := 0; i < len(value); {
		if value[i] != '$' {
			b.WriteByte(value[i])
			i++
			continue
		}
		// At a `$`: `$$` is an escaped dollar and resolves to one.
		if i+1 < len(value) && value[i+1] == '$' {
			b.WriteByte('$')
			i += 2
			continue
		}
		// `${...}` is a braced reference; everything it does not define is
		// an error, so a typo fails here rather than reaching the engine.
		if i+1 < len(value) && value[i+1] == '{' {
			end := strings.IndexByte(value[i:], '}')
			if end < 0 {
				return "", fmt.Errorf("unterminated environment reference %q: a `${` reference must end with `}` (write $$ for a literal `$`)",
					value[i:])
			}
			text, err := braced(value[i : i+end+1])
			if err != nil {
				return "", err
			}
			b.WriteString(text)
			i += end + 1
			continue
		}
		// `$NAME` is a bare reference. A `$` before anything that cannot
		// start a name is literal.
		if i+1 < len(value) && isNameStart(value[i+1]) {
			j := i + 1
			for j < len(value) && isNameChar(value[j]) {
				j++
			}
			b.WriteString(os.Getenv(value[i+1 : j]))
			i = j
			continue
		}
		b.WriteByte('$')
		i++
	}
	return b.String(), nil
}

// braced resolves one complete `${...}` reference, spelled exactly as it
// appears in the value so the error can quote it.
func braced(ref string) (string, error) {
	inner := ref[2 : len(ref)-1]
	name, def := inner, ""
	if colon := strings.IndexByte(inner, ':'); colon >= 0 {
		// Only `:-default` is defined; every other colon form (`:+`, `#`,
		// `%`, …) belongs to shells this grammar does not speak.
		if colon+1 >= len(inner) || inner[colon+1] != '-' {
			return "", fmt.Errorf("unsupported environment reference %q: only ${NAME} and ${NAME:-default} are resolved (write $$ for a literal `$`)", ref)
		}
		name, def = inner[:colon], inner[colon+2:]
	}
	if !validName(name) {
		return "", fmt.Errorf("invalid environment reference %q: a variable name starts with a letter or `_` and holds letters, digits or `_` (write $$ for a literal `$`)", ref)
	}
	if v, ok := os.LookupEnv(name); ok && v != "" {
		return v, nil
	}
	return def, nil
}

// validName reports whether s is a shell-style variable name.
func validName(s string) bool {
	if s == "" || !isNameStart(s[0]) {
		return false
	}
	for i := 1; i < len(s); i++ {
		if !isNameChar(s[i]) {
			return false
		}
	}
	return true
}

func isNameStart(c byte) bool {
	return c == '_' || ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z')
}

func isNameChar(c byte) bool {
	return isNameStart(c) || ('0' <= c && c <= '9')
}
