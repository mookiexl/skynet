package tools

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// ApplyReplace performs an exact-match replacement with an ambiguity check.
// If replaceAll is false and the search text occurs more than once, it returns an error
// instructing the caller to set replaceAll to true.
func ApplyReplace(content, search, replacement string, replaceAll bool) (count int, newContent string, err error) {
	count = strings.Count(content, search)
	if !replaceAll && count > 1 {
		return count, "", fmt.Errorf("multiple occurrences (%d) of search text found - set 'replace_all' to true to replace all", count)
	}
	if replaceAll {
		return count, strings.ReplaceAll(content, search, replacement), nil
	}
	return count, strings.Replace(content, search, replacement, 1), nil
}

// TruncateRuneSafe truncates s to at most limit bytes without splitting a
// multi-byte UTF-8 rune. The result is valid UTF-8 whenever s is.
func TruncateRuneSafe(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	i := limit
	for i > 0 && !utf8.RuneStart(s[i]) {
		i--
	}
	return s[:i]
}

// TruncateOutput truncates a string to MaxTextLimit characters.
// Returns the (possibly truncated) string and whether truncation occurred.
func TruncateOutput(s string) (string, bool) {
	limit := int(MaxTextLimit.Load())
	if len(s) <= limit {
		return s, false
	}
	return TruncateRuneSafe(s, limit), true
}

// ErrorResult creates a standardized error response map.
func ErrorResult(msg string) map[string]any {
	return map[string]any{"error": msg}
}

// ArgString extracts a string argument from the tool args map.
func ArgString(args map[string]any, key string) (string, bool) {
	s, ok := args[key].(string)
	return s, ok
}

// ArgStringSlice extracts a []string argument from the tool args map.
// It returns false if the argument is missing, not a JSON array, or
// contains a non-string element.
func ArgStringSlice(args map[string]any, key string) ([]string, bool) {
	raw, ok := args[key].([]any)
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		s, ok := v.(string)
		if !ok {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}

// ArgBool extracts a bool argument from the tool args map.
func ArgBool(args map[string]any, key string) (bool, bool) {
	b, ok := args[key].(bool)
	return b, ok
}

// ArgFloat extracts a float64 argument from the tool args map.
func ArgFloat(args map[string]any, key string) (float64, bool) {
	f, ok := args[key].(float64)
	return f, ok
}
